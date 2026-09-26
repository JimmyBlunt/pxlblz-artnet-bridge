package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	cfgpkg "pxlblz-router/internal/config"
	"pxlblz-router/internal/virtualcontroller"
)

const version = "0.1.0"

func main() {
	configPath := flag.String("config", "config/routes.installation-known.json", "route configuration JSON used to derive receiver profile")
	targetIP := flag.String("target-ip", "10.0.0.253", "controller target IP whose routes should be emulated")
	listen := flag.String("listen", "127.0.0.1:6454", "UDP Art-Net listen address")
	web := flag.String("web", "127.0.0.1:9982", "visualizer HTTP address (empty disables)")
	fps := flag.Int("fps", 0, "physical output FPS override (0 = config fps)")
	duration := flag.Duration("duration", 0, "optional run duration, e.g. 10s")
	summaryJSON := flag.String("summary-json", "", "write final receiver snapshot JSON")
	partialTimeout := flag.Duration("partial-timeout", virtualcontroller.DefaultPartialTimeout, "partial-frame timeout")
	sequenceReset := flag.Duration("sequence-reset", virtualcontroller.DefaultSequenceReset, "sequence reset after no accepted packet")
	lossTimeout := flag.Duration("loss-timeout", virtualcontroller.DefaultLossTimeout, "complete-frame loss timeout before blackout")
	pixelWire := flag.Duration("pixel-wire-time", virtualcontroller.DefaultPixelWireTime, "simulated serial wire time per LED")
	latch := flag.Duration("latch-time", virtualcontroller.DefaultLatchTime, "simulated post-DMA latch guard")
	flag.Parse()

	cfg, err := cfgpkg.Load(*configPath)
	fatal(err)

	controller, err := virtualcontroller.New(cfg, *targetIP, virtualcontroller.Options{
		OutputFPS: *fps,
		PartialTimeout: *partialTimeout,
		SequenceReset: *sequenceReset,
		LossTimeout: *lossTimeout,
		PixelWireTime: *pixelWire,
		LatchTime: *latch,
	})
	fatal(err)

	addr, err := net.ResolveUDPAddr("udp4", *listen)
	fatal(err)
	conn, err := net.ListenUDP("udp4", addr)
	fatal(err)
	defer conn.Close()

	actualListen := conn.LocalAddr().String()
	snap := controller.Snapshot(time.Now())
	fmt.Printf("PXLBLZ Virtual Art-Net Controller v%s\n", version)
	fmt.Println("Receiver model: Teensy runtime_receiver / artnet_run_policy compatible")
	fmt.Printf("Profile target: %s | UDP: %s | output: %d FPS | expected universes: %d | wire guard: %d us\n",
		*targetIP, actualListen, snap.OutputFPS, snap.ExpectedUniverses, snap.WireGuardUS)
	fmt.Println("Expected routes:")
	for _, r := range snap.Routes {
		fmt.Printf("  P%-2d %-20s %4d px  U%d..U%d  %s  level %.2f\n",
			r.PhysicalPort, r.Name, r.PixelCount, r.UniverseStart, r.UniverseEnd, r.ColorOrder, r.Brightness)
	}

	var httpServer *http.Server
	if strings.TrimSpace(*web) != "" {
		httpServer = &http.Server{Addr: *web, Handler: controller.Handler()}
		go func() {
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Fprintln(os.Stderr, "visualizer:", err)
			}
		}()
		fmt.Printf("Visualizer: http://%s/\n", *web)
	}

	stop := make(chan struct{})
	go receiveLoop(conn, controller, stop)

	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	statsTick := time.NewTicker(time.Second)
	defer statsTick.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	var deadline <-chan time.Time
	if *duration > 0 {
		deadline = time.After(*duration)
	}

	prev := controller.Snapshot(time.Now())
	for {
		select {
		case now := <-tick.C:
			controller.Tick(now)
		case now := <-statsTick.C:
			s := controller.Snapshot(now)
			fmt.Printf("RX %5d pkt/s | accepted %4d/s complete %4d/s incomplete %3d rejected %3d ignored %3d stale %3d dup %3d | submit %3d/s DMA %3d/s | cand %2d/%2d | ingest p99 %6.1f us assembly p99 %7.1f us | %s\n",
				s.Counters.Packets-prev.Counters.Packets,
				s.Counters.Accepted-prev.Counters.Accepted,
				s.Counters.Complete-prev.Counters.Complete,
				s.Counters.Incomplete-prev.Counters.Incomplete,
				s.Counters.Rejected-prev.Counters.Rejected,
				s.Counters.Ignored-prev.Counters.Ignored,
				s.Counters.Stale-prev.Counters.Stale,
				s.Counters.Duplicates-prev.Counters.Duplicates,
				s.Counters.FramesSubmitted-prev.Counters.FramesSubmitted,
				s.Counters.DMACompleted-prev.Counters.DMACompleted,
				s.CandidateReceived, s.ExpectedUniverses,
				s.Timing.IngestP99US, s.Timing.AssemblyP99US, s.State)
			prev = s
		case <-sig:
			close(stop)
			finish(controller, *summaryJSON)
			return
		case <-deadline:
			close(stop)
			finish(controller, *summaryJSON)
			return
		}
	}
}

func receiveLoop(conn *net.UDPConn, controller *virtualcontroller.Controller, stop <-chan struct{}) {
	buf := make([]byte, 2048)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-stop:
					return
				default:
					continue
				}
			}
			select {
			case <-stop:
				return
			default:
				fmt.Fprintln(os.Stderr, "UDP receive:", err)
				continue
			}
		}
		controller.IngestPacket(buf[:n], time.Now())
	}
}

func finish(controller *virtualcontroller.Controller, summaryPath string) {
	now := time.Now()
	controller.Tick(now)
	s := controller.Snapshot(now)
	fmt.Printf("\nFinal: packets=%d accepted=%d rejected=%d ignored=%d stale=%d duplicates=%d complete=%d incomplete=%d submitted=%d dma_completed=%d blackouts=%d state=%s\n",
		s.Counters.Packets, s.Counters.Accepted, s.Counters.Rejected, s.Counters.Ignored,
		s.Counters.Stale, s.Counters.Duplicates, s.Counters.Complete, s.Counters.Incomplete,
		s.Counters.FramesSubmitted, s.Counters.DMACompleted, s.Counters.Blackouts, s.State)
	if strings.TrimSpace(summaryPath) == "" {
		return
	}
	b, err := json.MarshalIndent(s, "", "  ")
	fatal(err)
	fatal(os.MkdirAll(filepath.Dir(summaryPath), 0o755))
	fatal(os.WriteFile(summaryPath, append(b, '\n'), 0o644))
	fmt.Println("Summary:", summaryPath)
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", strings.TrimSpace(err.Error()))
		os.Exit(1)
	}
}
