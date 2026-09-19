package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	cfgpkg "pxlblz-fadecandy/internal/config"
	"pxlblz-fadecandy/internal/frameinput"
	"pxlblz-fadecandy/internal/mapping"
	"pxlblz-fadecandy/internal/output"
	"pxlblz-fadecandy/internal/pattern"
	"pxlblz-fadecandy/internal/wsinput"
)

const version = "0.1.0-dev"

func main() {
	configPath := flag.String("config", "config/l3d-8x8x8.json", "configuration JSON")
	duration := flag.Duration("duration", 0, "optional run duration (0 = until Ctrl-C)")
	inputMode := flag.String("input", "ws", "frame input: ws|pattern")
	patternName := flag.String("pattern", "axes", "test pattern for --input pattern: xyz|axes|voxel|layers|corners")
	dryRun := flag.Bool("dry-run", false, "map frames but do not connect to fcserver")
	flag.Parse()
	cfg, err := cfgpkg.Load(*configPath)
	fatal(err)
	mapper, err := mapping.New(cfg.Mapping)
	fatal(err)
	frameBytes := cfg.Input.PixelCount * 3
	latest, err := frameinput.New(frameBytes)
	fatal(err)
	mode := strings.ToLower(strings.TrimSpace(*inputMode))
	if mode != "ws" && mode != "pattern" {
		fatal(fmt.Errorf("input must be ws or pattern"))
	}
	var server *wsinput.Server
	actual := "disabled"
	if mode == "ws" {
		server, err = wsinput.New(cfg.Input.WSListen, cfg.Input.WSPath, frameBytes, func(p []byte) { _ = latest.Submit(p) })
		fatal(err)
		actual, err = server.Start()
		fatal(err)
		defer server.Close()
	}
	fc := output.New(cfg.Fadecandy.Address, byte(cfg.Fadecandy.Channel))
	defer fc.Close()

	fmt.Printf("PXLBLZ Fadecandy/L3D Router v%s\n", version)
	if mode == "ws" {
		fmt.Printf("Input: ws://%s%s | %d pixels / %d RGB bytes | %d fps target\n", actual, cfg.Input.WSPath, cfg.Input.PixelCount, frameBytes, cfg.Input.FPSTarget)
	} else {
		fmt.Printf("Input: built-in pattern %s | %d pixels | %d fps target\n", *patternName, cfg.Input.PixelCount, cfg.Input.FPSTarget)
	}
	fmt.Printf("Output: fcserver %s OPC channel %d | dry-run=%v\n", cfg.Fadecandy.Address, cfg.Fadecandy.Channel, *dryRun)
	fmt.Printf("Mapping: input %v -> output %v | flip x=%v y=%v z=%v\n", cfg.Mapping.InputOrder, cfg.Mapping.OutputOrder, cfg.Mapping.Flip.X, cfg.Mapping.Flip.Y, cfg.Mapping.Flip.Z)
	if mode == "ws" {
		fmt.Println("Waiting for first valid RGB frame...")
	}

	logical := make([]byte, frameBytes)
	physical := make([]byte, frameBytes)
	ticker := time.NewTicker(time.Second / time.Duration(cfg.Input.FPSTarget))
	defer ticker.Stop()
	statsTicker := time.NewTicker(time.Second)
	defer statsTicker.Stop()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	var stop <-chan time.Time
	if *duration > 0 {
		stop = time.After(*duration)
	}
	var sent, lastSent uint64
	var lastRX, lastReplaced, lastInvalid uint64
	haveExternal := mode == "pattern"
	var patternFrame uint64
	for {
		select {
		case <-ticker.C:
			if mode == "pattern" {
				fatal(pattern.Fill(logical, *patternName, patternFrame))
				patternFrame++
			} else {
				have, _, readErr := latest.ReadInto(logical)
				fatal(readErr)
				if have {
					haveExternal = true
				}
				if !haveExternal {
					continue
				}
			}
			fatal(mapper.MapRGB(logical, physical))
			if !*dryRun {
				if err := fc.SendRGB(physical); err != nil {
					fmt.Fprintln(os.Stderr, "WARN:", strings.TrimSpace(err.Error()))
					continue
				}
			}
			sent++
		case <-statsTicker.C:
			if mode == "ws" {
				st := latest.Stats()
				ws := server.Stats()
				fmt.Printf("RX %5.1f fps replaced %d/s invalid %d/s clients %d | OPC TX %5.1f fps total %d\n", float64(st.Submitted-lastRX), st.Replaced-lastReplaced, st.Invalid-lastInvalid, ws.Active, float64(sent-lastSent), sent)
				lastRX, lastReplaced, lastInvalid = st.Submitted, st.Replaced, st.Invalid
			} else {
				fmt.Printf("Pattern TX %5.1f fps total %d\n", float64(sent-lastSent), sent)
			}
			lastSent = sent
		case <-sig:
			return
		case <-stop:
			return
		}
	}
}
func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", strings.TrimSpace(err.Error()))
		os.Exit(1)
	}
}
