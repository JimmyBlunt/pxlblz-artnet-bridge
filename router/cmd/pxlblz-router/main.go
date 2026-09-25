package main

import (
	"flag"
	"fmt"
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
	"pxlblz-router/internal/wsmini"
)

const version = "0.2.1"

func main() {
	configPath := flag.String("config", "config/routes.loopback.json", "route configuration JSON")
	inputMode := flag.String("input", "pattern", "frame input: pattern|ws")
	patternName := flag.String("pattern", "rainbow", "test pattern for --input pattern: port-id|rainbow|chase|index|red|green|blue|white|black")
	fpsOverride := flag.Int("fps", 0, "override Art-Net output FPS")
	duration := flag.Duration("duration", 0, "optional run duration, e.g. 10s (0 = until Ctrl-C)")
	dryRun := flag.Bool("dry-run", false, "build/route packets but do not transmit UDP")
	listOnly := flag.Bool("list-routes", false, "validate config and print planned routes, then exit")
	wsListen := flag.String("ws-listen", "127.0.0.1:9980", "websocket listen address for --input ws")
	wsPath := flag.String("ws-path", "/pixels", "websocket path for --input ws")
	flag.Parse()

	cfg, err := cfgpkg.Load(*configPath)
	fatalIf(err)
	fps := cfg.Input.FPSTarget
	if *fpsOverride > 0 {
		fps = *fpsOverride
	}
	if fps < 1 || fps > 240 {
		fatalIf(fmt.Errorf("fps must be 1..240"))
	}
	mode := strings.ToLower(strings.TrimSpace(*inputMode))
	if mode != "pattern" && mode != "ws" {
		fatalIf(fmt.Errorf("input must be pattern or ws"))
	}

	r, err := router.New(cfg, *dryRun)
	fatalIf(err)
	defer r.Close()

	fmt.Printf("PXLBLZ Art-Net Router v%s\n", version)
	fmt.Printf("Pixels: %d (%d bytes/frame) | Art-Net FPS target: %d | UDP: %d | input: %s | dry-run: %v\n",
		cfg.Input.PixelCount, cfg.Input.PixelCount*3, fps, cfg.ArtNet.UDPPort, mode, *dryRun)
	fmt.Println("Routes:")
	for _, s := range r.RouteSummary() {
		fmt.Println("  " + s)
	}
	if *listOnly {
		return
	}

	frame := make([]byte, cfg.Input.PixelCount*3)
	period := time.Second / time.Duration(fps)
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	statsTicker := time.NewTicker(time.Second)
	defer statsTicker.Stop()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	var stop <-chan time.Time
	if *duration > 0 {
		stop = time.After(*duration)
	}

	var wsServer *wsmini.Server
	var latest *frameinput.LatestFrame
	if mode == "ws" {
		latest, err = frameinput.NewLatestFrame(len(frame))
		fatalIf(err)
		wsServer, err = wsmini.NewServer(*wsListen, *wsPath, len(frame), func(p []byte) {
			_ = latest.Submit(p)
		})
		fatalIf(err)
		actual, err := wsServer.Start()
		fatalIf(err)
		defer wsServer.Close()
		fmt.Printf("Pixel input listening: ws://%s%s (binary frame = exactly %d RGB bytes)\n", actual, *wsPath, len(frame))
		fmt.Println("Waiting for first valid input frame before Art-Net transmission starts...")
	}

	var patternFrameNo uint64
	var haveExternal bool
	start := time.Now()
	lastFrames, lastPackets := uint64(0), uint64(0)
	var lastTotalFrameTime time.Duration
	var lastRX, lastReplaced, lastInvalid uint64

	for {
		select {
		case <-ticker.C:
			switch mode {
			case "pattern":
				var fillErr error
				if strings.EqualFold(*patternName, "port-id") {
					fillErr = pattern.FillPortID(frame, cfg.Input.PixelCount, cfg.Routes, patternFrameNo)
				} else {
					fillErr = pattern.Fill(frame, cfg.Input.PixelCount, *patternName, patternFrameNo)
				}
				fatalIf(fillErr)
				fatalIf(r.SendFrame(frame))
				patternFrameNo++
			case "ws":
				have, _, _, readErr := latest.ReadInto(frame)
				fatalIf(readErr)
				if have {
					haveExternal = true
				}
				if haveExternal {
					fatalIf(r.SendFrame(frame))
				}
			}
		case <-statsTicker.C:
			st := r.Stats()
			intervalFrames := st.Frames - lastFrames
			txFPS := float64(intervalFrames)
			txPPS := st.Packets - lastPackets
			intervalSend := st.TotalFrameTime - lastTotalFrameTime
			avgSendMs := 0.0
			if intervalFrames > 0 {
				avgSendMs = float64(intervalSend.Microseconds()) / 1000.0 / float64(intervalFrames)
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
			lastFrames, lastPackets, lastTotalFrameTime = st.Frames, st.Packets, st.TotalFrameTime
		case <-sig:
			printFinal(r, latest, start)
			return
		case <-stop:
			printFinal(r, latest, start)
			return
		}
	}
}

func printFinal(r *router.Router, latest *frameinput.LatestFrame, start time.Time) {
	st := r.Stats()
	sec := time.Since(start).Seconds()
	if sec < 0.001 {
		sec = 0.001
	}
	avgSendMs := 0.0
	if st.Frames > 0 {
		avgSendMs = float64(st.TotalFrameTime.Microseconds()) / 1000.0 / float64(st.Frames)
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	fmt.Printf("\nFinal TX: %d frames, %d packets, %.2f avg fps, %.2f packets/s, %.3f avg send ms, %.3f max send ms, %.1f MB heap, %d send errors\n",
		st.Frames, st.Packets, float64(st.Frames)/sec, float64(st.Packets)/sec,
		avgSendMs, float64(st.MaxFrameTime.Microseconds())/1000.0,
		float64(mem.HeapAlloc)/1024.0/1024.0, st.SendErrors)
	if latest != nil {
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
