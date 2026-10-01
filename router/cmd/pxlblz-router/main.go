package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	cfgpkg "pxlblz-router/internal/config"
	"pxlblz-router/internal/engine"
	"pxlblz-router/internal/frameinput"
	"pxlblz-router/internal/pattern"
	"pxlblz-router/internal/router"
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
	statusListen := flag.String("status-listen", "127.0.0.1:9988", "status + configuration web page (http://.../ and GET /status); empty disables")
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

	// planning / validation output (also used by --list-routes)
	plan, err := router.New(cfg, true)
	fatalIf(err)
	if len(plan.Controllers()) == 0 {
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
	for _, s := range plan.RouteSummary() {
		fmt.Println("  " + s)
	}
	for _, w := range cfg.Warnings() {
		fmt.Println("WARNING:", w)
	}
	plan.Close()
	if *listOnly {
		return
	}
	if mode == "pattern" {
		// validate the pattern name once before starting
		probe := make([]byte, cfg.Input.PixelCount*3)
		fatalIf(fillPattern(probe, cfg.Input.PixelCount, *patternName, cfg.Routes, 0, *walkSpeed))
	}

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

	eng, err := engine.New(ctx, cfg, engine.Options{
		DryRun:          *dryRun,
		FPSOverride:     *fpsOverride,
		Mode:            mode,
		Pattern:         *patternName,
		WalkSpeed:       *walkSpeed,
		OnStaleOverride: *onStale,
		StaleMSOverride: *staleMS,
	})
	fatalIf(err)
	defer eng.Close()

	var wsServer *wsmini.Server
	switch mode {
	case "ws":
		frameSize := cfg.Input.PixelCount * 3
		maxPayload := max(frameSize, variableSizeMaxPayload) // the config may enable variable_size later
		wsServer, err = wsmini.NewServer(*wsListen, *wsPath, maxPayload, eng.Submit)
		fatalIf(err)
		actual, err := wsServer.Start()
		fatalIf(err)
		defer wsServer.Close()
		if cfg.Input.VariableSize {
			fmt.Printf("Pixel input listening: ws://%s%s (binary frame = any whole RGB pixel count; first %d pixels used, missing pixels black)\n", actual, *wsPath, cfg.Input.PixelCount)
		} else {
			fmt.Printf("Pixel input listening: ws://%s%s (binary frame = exactly %d RGB bytes)\n", actual, *wsPath, frameSize)
		}
		fmt.Println("Waiting for first valid input frame before Art-Net transmission starts...")
	case "pattern":
		genFPS := cfg.Input.FPSTarget
		if *fpsOverride > 0 {
			genFPS = *fpsOverride
		}
		fmt.Printf("Pattern %q generated at %d fps\n", *patternName, genFPS)
	}

	start := time.Now()
	if *statusListen != "" {
		absConfig, _ := filepath.Abs(*configPath)
		stop, addr, err := startWeb(*statusListen, &webServer{eng: eng, ws: wsServer, configPath: absConfig, start: start})
		if err != nil {
			fmt.Println("WARNING: status endpoint disabled:", err)
		} else {
			defer stop()
			fmt.Printf("Status: http://%s/status\n", addr)
			fmt.Printf("Configuration page: http://%s/\n", addr)
		}
	}

	statsTicker := time.NewTicker(time.Second)
	defer statsTicker.Stop()
	var (
		lastRouter                    *router.Router
		lastLatest                    *frameinput.LatestFrame
		lastTotal                     router.Stats
		lastSendFrames                uint64
		lastSendTime                  time.Duration
		lastRX, lastReplaced, lastInv uint64
		prev                          = eng.Scheduler().Status()
	)
	for {
		select {
		case <-ctx.Done():
			printFinal(eng.Router(), eng.Latest(), mode, start)
			return
		case err := <-eng.Errors():
			printFinal(eng.Router(), eng.Latest(), mode, start)
			fatalIf(err)
			return
		case <-statsTicker.C:
			r, latest := eng.Router(), eng.Latest()
			if r != lastRouter { // a new config was applied: counters restart
				lastRouter, lastTotal = r, router.Stats{}
				lastSendFrames, lastSendTime = 0, 0
				prev = eng.Scheduler().Status()
			}
			if latest != lastLatest {
				lastLatest, lastRX, lastReplaced, lastInv = latest, 0, 0, 0
			}
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
					float64(ist.Submitted-lastRX), ist.Replaced-lastReplaced, ist.Invalid-lastInv, wst.Active,
					txFPS, txPPS,
					avgSendMs,
					float64(st.LastFrameTime.Microseconds())/1000.0,
					float64(st.MaxFrameTime.Microseconds())/1000.0,
					heapMB, mem.NumGC, runtime.NumGoroutine(),
					st.SendErrors)
				lastRX, lastReplaced, lastInv = ist.Submitted, ist.Replaced, ist.Invalid
			} else {
				fmt.Printf("TX %5.1f fps | %5d pkt/s | send avg %7.3f ms last %7.3f ms max %7.3f ms | heap %5.1f MB gc %d goroutines %d | errors %d\n",
					txFPS, txPPS,
					avgSendMs,
					float64(st.LastFrameTime.Microseconds())/1000.0,
					float64(st.MaxFrameTime.Microseconds())/1000.0,
					heapMB, mem.NumGC, runtime.NumGoroutine(),
					st.SendErrors)
			}
			cur := eng.Scheduler().Status()
			if *perController && len(cur) > 1 && len(cur) == len(prev) {
				onStaleMode := eng.Config().Input.OnStale
				for i, c := range cur {
					state := "live"
					if c.Stale {
						state = "STALE/" + onStaleMode
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

// variableSizeMaxPayload bounds WebSocket frames with input.variable_size
// (~350k RGB pixels), so a larger PXLBLZ map than the router config still fits.
const variableSizeMaxPayload = 1 << 20

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
