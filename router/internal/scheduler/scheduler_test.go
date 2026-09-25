package scheduler

import (
	"context"
	"testing"
	"time"

	cfgpkg "pxlblz-router/internal/config"
	"pxlblz-router/internal/frameinput"
	"pxlblz-router/internal/router"
)

func twoControllerRouter(t *testing.T) (*router.Router, cfgpkg.Config) {
	t.Helper()
	cfg := cfgpkg.Config{
		Version: 2,
		Input:   cfgpkg.InputConfig{PixelCount: 100, FPSTarget: 60},
		ArtNet:  cfgpkg.ArtNet{UDPPort: 6454},
		Controllers: []cfgpkg.Controller{
			{Name: "FAST", TargetIP: "10.0.0.1", FPSTarget: 60},
			{Name: "SLOW", TargetIP: "10.0.0.2", FPSTarget: 20},
		},
		Routes: []cfgpkg.Route{
			{Name: "f", Enabled: true, TargetIP: "10.0.0.1", PixelStart: 0, PixelCount: 50, ColorOrder: "RGB"},
			{Name: "s", Enabled: true, TargetIP: "10.0.0.2", PixelStart: 50, PixelCount: 50, ColorOrder: "RGB"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	r, err := router.New(cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	return r, cfg
}

func TestNoTransmitBeforeFirstFrame(t *testing.T) {
	r, _ := twoControllerRouter(t)
	lf, _ := frameinput.NewLatestFrame(300)
	s := New(r, lf, Options{})
	for i := 0; i < 5; i++ {
		if err := s.TickAll(); err != nil {
			t.Fatal(err)
		}
	}
	if r.Stats().Frames != 0 {
		t.Fatal("transmitted before any input frame")
	}
	_ = lf.Submit(make([]byte, 300))
	_ = s.TickAll()
	if r.Stats().Frames != 1 {
		t.Fatal("did not transmit after first frame")
	}
}

func TestStaleStopAndRepeats(t *testing.T) {
	r, _ := twoControllerRouter(t)
	lf, _ := frameinput.NewLatestFrame(300)
	now := time.Now()
	s := New(r, lf, Options{StaleTimeout: 100 * time.Millisecond, OnStale: cfgpkg.OnStaleStop, Now: func() time.Time { return now }})
	_ = lf.Submit(make([]byte, 300))
	now = time.Now()
	_ = s.TickAll()
	_ = s.TickAll() // same frame again = repeat, still live
	st := s.Status()
	if st[0].Frames != 2 || st[0].Repeats != 1 || st[0].Stale {
		t.Fatalf("live status wrong: %+v", st[0])
	}
	now = now.Add(time.Second)
	_ = s.TickAll()
	st = s.Status()
	if st[0].Frames != 2 || !st[0].Stale || st[0].Skipped != 1 {
		t.Fatalf("stop-on-stale should skip: %+v", st[0])
	}
	// input resumes
	_ = lf.Submit(make([]byte, 300))
	now = time.Now()
	_ = s.TickAll()
	if st = s.Status(); st[0].Frames != 3 || st[0].Stale {
		t.Fatalf("resume failed: %+v", st[0])
	}
}

func TestBlackoutUsesZeroFrameWithoutTouchingHeldFrame(t *testing.T) {
	r, _ := twoControllerRouter(t)
	lf, _ := frameinput.NewLatestFrame(300)
	now := time.Now()
	s := New(r, lf, Options{StaleTimeout: 50 * time.Millisecond, OnStale: cfgpkg.OnStaleBlackout, Now: func() time.Time { return now }})
	f := make([]byte, 300)
	for i := range f {
		f[i] = 200
	}
	_ = lf.Submit(f)
	_ = s.TickAll()
	now = now.Add(time.Second)
	_ = s.TickAll()
	if !s.Status()[0].Stale || s.Status()[0].Frames != 2 {
		t.Fatal("blackout should keep transmitting while stale")
	}
	for _, b := range s.black {
		if b != 0 {
			t.Fatal("black frame polluted")
		}
	}
	if s.states[0].buf[0] != 200 {
		t.Fatal("held frame must be preserved during blackout")
	}
}

func TestControllersRunAtOwnRate(t *testing.T) {
	r, _ := twoControllerRouter(t)
	lf, _ := frameinput.NewLatestFrame(300)
	_ = lf.Submit(make([]byte, 300))
	s := New(r, lf, Options{})
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := s.Run(ctx); err != nil {
		t.Fatal(err)
	}
	st := s.Status()
	fast, slow := st[0].Frames, st[1].Frames
	if fast < 45 || fast > 65 || slow < 15 || slow > 22 {
		t.Fatalf("rates off: fast=%d (60) slow=%d (20)", fast, slow)
	}
}

func TestFPSOverride(t *testing.T) {
	r, _ := twoControllerRouter(t)
	lf, _ := frameinput.NewLatestFrame(300)
	s := New(r, lf, Options{FPSOverride: 10})
	for _, c := range s.Status() {
		if c.FPS != 10 {
			t.Fatalf("override not applied: %+v", c)
		}
	}
}
