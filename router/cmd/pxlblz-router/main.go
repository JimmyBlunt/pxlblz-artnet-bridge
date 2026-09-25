package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	cfgpkg "pxlblz-router/internal/config"
	"pxlblz-router/internal/frameinput"
	"pxlblz-router/internal/pattern"
	"pxlblz-router/internal/router"
	"pxlblz-router/internal/scheduler"
	"pxlblz-router/internal/wsmini"
)

const version = "0.3.0-dev"

func main() {
	configPath := flag.String("config", "config/routes.loopback.json", "route configuration JSON")
	inputMode := flag.String("input", "pattern", "frame input: pattern|ws")
	patternName := flag.String("pattern", "rainbow", "test pattern for --input pattern: panel-walk|port-id|rainbow|chase|index|red|green|blue|white|black")
	walkSpeed := flag.Int("walk-speed", 4, "panel-walk: logical pixels the head advances per pattern frame")
	fpsOverride := flag.Int("fps", 0, "override Art-Net output FPS for ALL controllers (0 = per-controller config)")
	onStale := flag.String("on-stale", "", "override input.on_stale: hold|blackout|stop")
	staleMS := flag.Int("stale-timeout-ms", -1, "override input.stale_timeout_ms (0 disables, -1 = config)")
	duration := flag.Duration("duration", 0, "optional run duration, e.g. 10s (0 = until Ctrl-C)")
	dryRun := flag.Bool("dry-run", false, "build/route packets but do not transmit UDP")
	listOnly := flag.Bool("list-routes", false, "validate config and print planned routes, then exit")
	wsListen := flag.String("ws-listen", "127.0.0.1:9980", "websocket listen address for --input ws")
	wsPath := flag.String("ws-path", "/pixels", "websocket path for --input ws")
	statusListen := flag.String("status-listen", "127.0.0.1:9981", "HTTP status endpoint (GET /status); empty disables")
	perController := flag.Bool("per-controller-stats", true, "print one stats line per controller when more than one is configured")
	flag.Parse()

	cfg, err := cfgpkg.Load(*configPath)
	fatalIf(err)
	if *onStale != "" {
		cfg.Input.OnStale = strings.ToLower(*onStale)
	}
	if *staleMS >= 0 {
		cfg.Input.StaleTimeoutMS = *staleMS
	}
	fatalIf(cfg.Validate())
	if *fpsOverride < 0 || *fpsOverride > 240 {
		fatalIf(fmt.Errorf("fps must be 1..240"))
	}
	mode := strings.ToLower(strings.TrimSpace(*inputMode))
	if mode != "pattern" && mode != "ws" {
		fatalIf(fmt.Errorf("input must be pattern or ws"))
	}

	r, err := router.New(cfg, *dryRun)
	fatalIf(err)
	defer r.Close()
	if len(r.Controllers()) == 0 {
		fatalIf(fmt.Errorf("no enabled routes/controllers in %s", *configPath))
	}

	fmt.Printf("PXLBLZ Art-Net Router v%s\n", version)
	fmt.Printf("Pixels: %d (%d bytes/frame) | input FPS: %d | UDP: %d | input: %s | dry-run: %v\n",
		cfg.Input.PixelCount, cfg.Input.PixelCount*3, cfg.Input.FPSTarget, cfg.ArtNet.UDPPort, mode, *dryRun)
	if cfg.Input.StaleTimeoutMS > 0 {
		fmt.Printf("Stale input: after %d ms without a new frame -> %s\n", cfg.Input.StaleTimeoutMS, cfg.Input.OnStale)
	} else {
		fmt.Println("Stale input: disabled (last frame is held)")
	}
	if *fpsOverride > 0 {
		fmt.Printf("FPS override: every controller at %d fps\n", *fpsOverride)
	}
	fmt.Println("Controllers / routes:")
	for _, s := range r.RouteSummary() {
		fmt.Println("  " + s)
	}
	for _, w := range cfg.Warnings() {
		fmt.Println("WARNING:", w)
	}
	if *listOnly {
		return
	}

	frameSize := cfg.Input.PixelCount * 3
	latest, err := frameinput.NewLatestFrame(frameSize)
	fatalIf(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sig:
		case <-ctx.Done():
		}
		cancel()
	}()
	if *duration > 0 {
		go func() {
			select {
			case <-time.After(*duration):
			case <-ctx.Done():
			}
			cancel()
		}()
	}

	var wsServer *wsmini.Server
	switch mode {
	case "ws":
		wsServer, err = wsmini.NewServer(*wsListen, *wsPath, frameSize, func(p []byte) {
			_ = latest.Submit(p)
		})
		fatalIf(err)
		actual, err := wsServer.Start()
		fatalIf(err)
		defer wsServer.Close()
		fmt.Printf("Pixel input listening: ws://%s%s (binary frame = exactly %d RGB bytes)\n", actual, *wsPath, frameSize)
		fmt.Println("Waiting for first valid input frame before Art-Net transmission starts...")
	case "pattern":
		genFPS := cfg.Input.FPSTarget
		if *fpsOverride > 0 {
			genFPS = *fpsOverride
		}
		routes := activeRoutes(cfg, r)
		// validate pattern name once before starting
		probe := make([]byte, frameSize)
		fatalIf(fillPattern(probe, cfg.Input.PixelCount, *patternName, routes, 0, *walkSpeed))
		go runPattern(ctx, latest, cfg.Input.PixelCount, *patternName, routes, *walkSpeed, genFPS)
		fmt.Printf("Pattern %q generated at %d fps\n", *patternName, genFPS)
	}

	sched := scheduler.New(r, latest, scheduler.Options{
		FPSOverride:  *fpsOverride,
		StaleTimeout: time.Duration(cfg.Input.StaleTimeoutMS) * time.Millisecond,
		OnStale:      cfg.Input.OnStale,
	})

	start := time.Now()
	if *statusListen != "" {
		stop, addr, err := startStatus(*statusListen, sched, latest, wsServer, start, mode)
		if err != nil {
			fmt.Println("WARNING: status endpoint disabled:", err)
		} else {
			defer stop()
			fmt.Printf("Status: http://%s/status\n", addr)
		}
	}

	schedErr := make(chan error, 1)
	go func() { schedErr <- sched.Run(ctx) }()

	statsTicker := time.NewTicker(time.Second)
	defer statsTicker.Stop()
	prev := sched.Status()
	lastTotal := r.Stats()
	lastSendFrames, lastSendTime := r.SendTotals()
	var lastRX, lastReplaced, lastInvalid uint64

	for {
		select {
		case err := <-schedErr:
			printFinal(r, latest, mode, start)
			fatalIf(err)
			return
		case <-statsTicker.C:
			st := r.Stats()
			txFPS := float64(st.Frames - lastTotal.Frames)
			txPPS := st.Packets - lastTotal.Packets
			sendFrames, sendTime := r.SendTotals()
			avgSendMs := 0.0
			if n := sendFrames - lastSendFrames; n > 0 {
				avgSendMs = float64((sendTime - lastSendTime).Microseconds()) / 1000.0 / float64(n)
			}
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			heapMB := float64(mem.HeapAlloc) / 1024.0 / 1024.0
			if mode == "ws" {
				ist := latest.Stats()
				wst := wsServer.Stats()
				fmt.Printf("RX %5.1f fps | replaced %4d/s invalid %d/s clients %d | TX %5.1f fps %5d pkt/s | send avg %6.3f ms last %6.3f ms max %6.3f ms | heap %5.1f MB gc %d goroutines %d | errors %d\n",
					float64(ist.Submitted-lastRX), ist.Replaced-lastReplaced, ist.Invalid-lastInvalid, wst.Active,
					txFPS, txPPS,
					avgSendMs,
					float64(st.LastFrameTime.Microseconds())/1000.0,
					float64(st.MaxFrameTime.Microseconds())/1000.0,
					heapMB, mem.NumGC, runtime.NumGoroutine(),
					st.SendErrors)
				lastRX, lastReplaced, lastInvalid = ist.Submitted, ist.Replaced, ist.Invalid
			} else {
				fmt.Printf("TX %5.1f fps | %5d pkt/s | send avg %7.3f ms last %7.3f ms max %7.3f ms | heap %5.1f MB gc %d goroutines %d | errors %d\n",
					txFPS, txPPS,
					avgSendMs,
					float64(st.LastFrameTime.Microseconds())/1000.0,
					float64(st.MaxFrameTime.Microseconds())/1000.0,
					heapMB, mem.NumGC, runtime.NumGoroutine(),
					st.SendErrors)
			}
			cur := sched.Status()
			if *perController && len(cur) > 1 {
				for i, c := range cur {
					state := "live"
					if c.Stale {
						state = "STALE/" + cfg.Input.OnStale
					}
					fmt.Printf("   %-18s %-15s TX %5.1f/%3d fps %5d pkt/s repeats %4d/s errors %d %s\n",
						c.Name, c.TargetIP, float64(c.Frames-prev[i].Frames), c.FPS,
						c.Packets-prev[i].Packets, c.Repeats-prev[i].Repeats, c.Errors, state)
				}
			}
			prev = cur
			lastTotal = st
			lastSendFrames, lastSendTime = sendFrames, sendTime
		}
	}
}

func activeRoutes(cfg cfgpkg.Config, r *router.Router) []cfgpkg.Route {
	on := map[string]bool{}
	for _, c := range r.Controllers() {
		on[c.TargetIP()] = true
	}
	var out []cfgpkg.Route
	for _, rt := range cfg.Routes {
		if rt.Enabled && on[rt.TargetIP] {
			out = append(out, rt)
		}
	}
	return out
}

func fillPattern(frame []byte, pixels int, name string, routes []cfgpkg.Route, n uint64, walkSpeed int) error {
	switch strings.ToLower(name) {
	case "port-id":
		return pattern.FillPortID(frame, pixels, routes, n)
	case "panel-walk", "walk":
		return pattern.FillPanelWalk(frame, pixels, routes, n, walkSpeed)
	default:
		return pattern.Fill(frame, pixels, name, n)
	}
}

func runPattern(ctx context.Context, latest *frameinput.LatestFrame, pixels int, name string, routes []cfgpkg.Route, walkSpeed, fps int) {
	frame := make([]byte, pixels*3)
	t := time.NewTicker(time.Second / time.Duration(fps))
	defer t.Stop()
	var n uint64
	for {
		if err := fillPattern(frame, pixels, name, routes, n, walkSpeed); err == nil {
			_ = latest.Submit(frame)
		}
		n++
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

type statusDoc struct {
	Version     string                       `json:"version"`
	UptimeSec   float64                      `json:"uptime_s"`
	Input       string                       `json:"input"`
	RXFrames    uint64                       `json:"rx_frames"`
	RXReplaced  uint64                       `json:"rx_replaced"`
	RXInvalid   uint64                       `json:"rx_invalid"`
	LastFrameMS *float64                     `json:"last_frame_age_ms"`
	WSClients   int64                        `json:"ws_clients"`
	Controllers []scheduler.ControllerStatus `json:"controllers"`
}

func startStatus(listen string, sched *scheduler.Scheduler, latest *frameinput.LatestFrame, ws *wsmini.Server, start time.Time, mode string) (func(), string, error) {
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, "", err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, _ *http.Request) {
		ist := latest.Stats()
		doc := statusDoc{
			Version:     version,
			UptimeSec:   time.Since(start).Seconds(),
			Input:       mode,
			RXFrames:    ist.Submitted,
			RXReplaced:  ist.Replaced,
			RXInvalid:   ist.Invalid,
			Controllers: sched.Status(),
		}
		if !ist.LastAt.IsZero() {
			age := float64(time.Since(ist.LastAt).Microseconds()) / 1000
			doc.LastFrameMS = &age
		}
		if ws != nil {
			doc.WSClients = ws.Stats().Active
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(doc)
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 2 * time.Second}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Println("status endpoint stopped:", err)
		}
	}()
	return func() {
		ctx, c := context.WithTimeout(context.Background(), 500*time.Millisecond)
		_ = srv.Shutdown(ctx)
		c()
	}, ln.Addr().String(), nil
}

func printFinal(r *router.Router, latest *frameinput.LatestFrame, mode string, start time.Time) {
	sec := time.Since(start).Seconds()
	if sec < 0.001 {
		sec = 0.001
	}
	// The aggregate "Final TX:" line is parsed by perf-test/run-performance.mjs.
	st := r.Stats()
	avgSendMs := 0.0
	if frames, total := r.SendTotals(); frames > 0 {
		avgSendMs = float64(total.Microseconds()) / 1000.0 / float64(frames)
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	fmt.Printf("\nFinal TX: %d frames, %d packets, %.2f avg fps, %.2f packets/s, %.3f avg send ms, %.3f max send ms, %.1f MB heap, %d send errors\n",
		st.Frames, st.Packets, float64(st.Frames)/sec, float64(st.Packets)/sec,
		avgSendMs, float64(st.MaxFrameTime.Microseconds())/1000.0,
		float64(mem.HeapAlloc)/1024.0/1024.0, st.SendErrors)
	if len(r.Controllers()) > 1 {
		for _, c := range r.Controllers() {
			cs := c.Stats()
			fmt.Printf("Final TX [%s %s]: %d frames, %d packets, %.2f avg fps, %.2f packets/s, %d send errors\n",
				c.Name(), c.TargetIP(), cs.Frames, cs.Packets, float64(cs.Frames)/sec, float64(cs.Packets)/sec, cs.SendErrors)
		}
	}
	if mode == "ws" {
		s := latest.Stats()
		fmt.Printf("Final RX: %d valid frames, %d replaced before output observation, %d invalid\n", s.Submitted, s.Replaced, s.Invalid)
	}
}

func fatalIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", strings.TrimSpace(err.Error()))
		os.Exit(1)
	}
}
