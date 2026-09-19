package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	cfgpkg "pxlblz-router/internal/config"
	"pxlblz-router/internal/pattern"
	"pxlblz-router/internal/wsmini"
)

const version = "0.2.0"

func main() {
	configPath := flag.String("config", "config/routes.loopback.json", "configuration used for pixel_count and port-id route ranges")
	target := flag.String("ws", "127.0.0.1:9980", "router websocket host:port")
	path := flag.String("path", "/pixels", "router websocket path")
	patternName := flag.String("pattern", "rainbow", "port-id|rainbow|chase|index|red|green|blue|white|black")
	fps := flag.Int("fps", 60, "input frame rate")
	duration := flag.Duration("duration", 10*time.Second, "run duration (0 = until Ctrl-C)")
	flag.Parse()
	cfg, err := cfgpkg.Load(*configPath)
	fatal(err)
	if *fps < 1 || *fps > 240 {
		fatal(fmt.Errorf("fps must be 1..240"))
	}
	c, err := wsmini.Dial(*target, *path)
	fatal(err)
	defer c.Close()
	fmt.Printf("PXLBLZ Frame Sender v%s -> ws://%s%s
", version, *target, *path)
	fmt.Printf("Pixels: %d (%d bytes/frame) | input FPS: %d | pattern: %s
", cfg.Input.PixelCount, cfg.Input.PixelCount*3, *fps, *patternName)
	frame := make([]byte, cfg.Input.PixelCount*3)
	ticker := time.NewTicker(time.Second / time.Duration(*fps))
	defer ticker.Stop()
	statsTicker := time.NewTicker(time.Second)
	defer statsTicker.Stop()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	var stop <-chan time.Time
	if *duration > 0 {
		stop = time.After(*duration)
	}
	var n, last uint64
	started := time.Now()
	for {
		select {
		case <-ticker.C:
			if strings.EqualFold(*patternName, "port-id") {
				err = pattern.FillPortID(frame, cfg.Input.PixelCount, cfg.Routes, n)
			} else {
				err = pattern.Fill(frame, cfg.Input.PixelCount, *patternName, n)
			}
			fatal(err)
			fatal(c.SendBinary(frame))
			n++
		case <-statsTicker.C:
			fmt.Printf("WS TX %5.1f fps | frames %d
", float64(n-last), n)
			last = n
		case <-sig:
			printFinal(n, started)
			return
		case <-stop:
			printFinal(n, started)
			return
		}
	}
}
func printFinal(n uint64, start time.Time) {
	sec := time.Since(start).Seconds()
	if sec < .001 {
		sec = .001
	}
	fmt.Printf("Final: %d frames, %.2f avg fps
", n, float64(n)/sec)
}
func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", strings.TrimSpace(err.Error()))
		os.Exit(1)
	}
}
