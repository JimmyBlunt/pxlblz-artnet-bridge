package engine

import (
	"context"
	"testing"
	"time"

	cfgpkg "pxlblz-router/internal/config"
)

func testCfg(ip string, pixels int) cfgpkg.Config {
	return cfgpkg.Config{
		Version: 2,
		Input:   cfgpkg.InputConfig{PixelCount: pixels, FPSTarget: 100},
		ArtNet:  cfgpkg.ArtNet{UDPPort: 6454, Unicast: true},
		Controllers: []cfgpkg.Controller{
			{Name: "A", TargetIP: ip, FPSTarget: 100},
		},
		Routes: []cfgpkg.Route{
			{Name: "r", Enabled: true, TargetIP: ip, PixelCount: pixels, UniverseStart: 1, ColorOrder: "RGB"},
		},
	}
}

func newEngine(t *testing.T, cfg cfgpkg.Config) *Engine {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e, err := New(ctx, cfg, Options{DryRun: true, Mode: "ws", StaleMSOverride: -1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Close)
	return e
}

func waitFrames(t *testing.T, e *Engine, atLeast uint64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if e.Router().Stats().Frames >= atLeast {
			return
		}
		e.Submit(make([]byte, e.Latest().Size()))
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("router sent only %d frames", e.Router().Stats().Frames)
}

func TestApplySwapsControllerAndKeepsSequence(t *testing.T) {
	e := newEngine(t, testCfg("10.0.0.1", 10))
	waitFrames(t, e, 5)
	oldLatest := e.Latest()
	seqBefore := e.Router().Sequences()["10.0.0.1"]

	// same controller, new IP + same frame size
	cfg := testCfg("10.0.0.1", 10)
	cfg.Controllers[0].FPSTarget = 50
	if err := e.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	if e.Latest() != oldLatest {
		t.Fatal("same frame size must keep the input buffer (no output gap)")
	}
	if got := e.Router().Sequences()["10.0.0.1"]; got < seqBefore {
		t.Fatalf("sequence restarted: %d -> %d", seqBefore, got)
	}
	if e.Scheduler().Status()[0].FPS != 50 {
		t.Fatal("new fps not applied")
	}

	// IP change and different size: new buffer, new target
	if err := e.Apply(testCfg("10.0.0.248", 20)); err != nil {
		t.Fatal(err)
	}
	if e.Latest().Size() != 60 || e.Config().Controllers[0].TargetIP != "10.0.0.248" {
		t.Fatal("new config not active")
	}
	waitFrames(t, e, 3)
}

func TestApplyInvalidKeepsRunningRouter(t *testing.T) {
	e := newEngine(t, testCfg("10.0.0.1", 10))
	before := e.Router()
	bad := testCfg("10.0.0.1", 10)
	bad.Routes[0].TargetIP = "10.0.0.9" // no controller entry
	if err := e.Apply(bad); err == nil {
		t.Fatal("invalid config accepted")
	}
	if e.Router() != before {
		t.Fatal("running router replaced by an invalid config")
	}
	none := testCfg("10.0.0.1", 10)
	none.Routes[0].Enabled = false
	if err := e.Apply(none); err == nil {
		t.Fatal("config without active routes accepted")
	}
}

func TestTestPatternReplacesInputAndEnds(t *testing.T) {
	e := newEngine(t, testCfg("10.0.0.1", 10))
	if err := e.Test("nonsense", time.Second); err == nil {
		t.Fatal("unknown pattern accepted")
	}
	if err := e.Test("white", 300*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if !e.TestActive() {
		t.Fatal("test not active")
	}
	before := e.Latest().Stats().Submitted
	e.Submit(make([]byte, 30)) // input dropped while testing
	time.Sleep(100 * time.Millisecond)
	buf := make([]byte, 30)
	_, _, _, _ = e.Latest().ReadInto(buf)
	if buf[0] != 255 {
		t.Fatalf("test pattern not in the frame: %v", buf[:3])
	}
	if e.Latest().Stats().Submitted <= before {
		t.Fatal("generator did not submit")
	}
	time.Sleep(400 * time.Millisecond)
	if e.TestActive() {
		t.Fatal("test did not end")
	}
	e.Submit(make([]byte, 30))
	_, _, _, _ = e.Latest().ReadInto(buf)
	if buf[0] != 0 {
		t.Fatal("input not resumed after the test")
	}
}
