// Package engine owns the running router core (controllers + scheduler) and
// swaps it for a new configuration without restarting the process.
package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	cfgpkg "pxlblz-router/internal/config"
	"pxlblz-router/internal/frameinput"
	"pxlblz-router/internal/pattern"
	"pxlblz-router/internal/router"
	"pxlblz-router/internal/scheduler"
)

// Options are fixed for the lifetime of the process (command-line flags).
type Options struct {
	DryRun      bool
	FPSOverride int    // > 0 replaces every controller's FPS
	Mode        string // "ws" or "pattern"
	Pattern     string // --input pattern only
	WalkSpeed   int
	// Overrides re-applied on top of every config (empty / -1 = use config).
	OnStaleOverride string
	StaleMSOverride int
}

type state struct {
	cfg     cfgpkg.Config
	r       *router.Router
	latest  *frameinput.LatestFrame
	sched   *scheduler.Scheduler
	cancel  context.CancelFunc
	done    chan struct{}
	applied time.Time
}

// Engine is safe for concurrent use: input goroutines call Submit, the HTTP
// layer calls Apply / Test, the stats loop reads the getters.
type Engine struct {
	parent context.Context
	opts   Options

	applyMu sync.Mutex
	cur     atomic.Pointer[state]
	errs    chan error

	testMu     sync.Mutex
	testCancel context.CancelFunc
	testName   atomic.Value // string
	testUntil  atomic.Int64 // unix nanos, 0 = no test
}

// New builds the router for cfg and starts scheduling. The engine stops when
// ctx is cancelled.
func New(ctx context.Context, cfg cfgpkg.Config, opts Options) (*Engine, error) {
	e := &Engine{parent: ctx, opts: opts, errs: make(chan error, 4)}
	e.testName.Store("")
	st, err := e.build(cfg, nil)
	if err != nil {
		return nil, err
	}
	e.start(st)
	e.cur.Store(st)
	return e, nil
}

// Prepare applies the command-line overrides and validates cfg the same way
// Apply would, without touching the running router.
func (e *Engine) Prepare(cfg cfgpkg.Config) (cfgpkg.Config, error) {
	cfg.ApplyDefaults()
	if e.opts.OnStaleOverride != "" {
		cfg.Input.OnStale = strings.ToLower(e.opts.OnStaleOverride)
	}
	if e.opts.StaleMSOverride >= 0 {
		cfg.Input.StaleTimeoutMS = e.opts.StaleMSOverride
	}
	if err := cfg.Validate(); err != nil {
		return cfgpkg.Config{}, err
	}
	r, err := router.New(cfg, true)
	if err != nil {
		return cfgpkg.Config{}, err
	}
	defer r.Close()
	if len(r.Controllers()) == 0 {
		return cfgpkg.Config{}, fmt.Errorf("keine aktiven Controller/Routen in der Konfiguration")
	}
	return cfg, nil
}

func (e *Engine) build(cfg cfgpkg.Config, prev *state) (*state, error) {
	cfg, err := e.Prepare(cfg)
	if err != nil {
		return nil, err
	}
	r, err := router.New(cfg, e.opts.DryRun)
	if err != nil {
		return nil, err
	}
	size := cfg.Input.PixelCount * 3
	var latest *frameinput.LatestFrame
	if prev != nil && prev.latest.Size() == size {
		latest = prev.latest // same size: keep the last frame, output continues seamlessly
	} else {
		latest, err = frameinput.NewLatestFrame(size)
		if err != nil {
			r.Close()
			return nil, err
		}
	}
	latest.SetVariableSize(cfg.Input.VariableSize)
	sched := scheduler.New(r, latest, scheduler.Options{
		FPSOverride:  e.opts.FPSOverride,
		StaleTimeout: time.Duration(cfg.Input.StaleTimeoutMS) * time.Millisecond,
		OnStale:      cfg.Input.OnStale,
	})
	return &state{cfg: cfg, r: r, latest: latest, sched: sched, applied: time.Now()}, nil
}

func (e *Engine) start(st *state) {
	ctx, cancel := context.WithCancel(e.parent)
	st.cancel = cancel
	st.done = make(chan struct{})
	go func() {
		defer close(st.done)
		if err := st.sched.Run(ctx); err != nil {
			select {
			case e.errs <- err:
			default:
			}
		}
	}()
	if e.opts.Mode == "pattern" {
		go e.runGenerator(ctx, st, e.opts.Pattern, false)
	}
}

func (e *Engine) stop(st *state) {
	st.cancel()
	<-st.done
	st.r.Close()
}

// Apply validates cfg, builds a new router and swaps it in. On error the
// running router is left untouched.
func (e *Engine) Apply(cfg cfgpkg.Config) error {
	e.applyMu.Lock()
	defer e.applyMu.Unlock()
	old := e.cur.Load()
	st, err := e.build(cfg, old)
	if err != nil {
		return err
	}
	// Stop first so the two routers never send to a controller at the same
	// time; the gap is a few milliseconds.
	e.stop(old)
	st.r.RestoreSequences(old.r.Sequences()) // old router is stopped: its sequences are final
	e.start(st)
	e.cur.Store(st)
	return nil
}

// Close stops output.
func (e *Engine) Close() {
	e.StopTest()
	e.applyMu.Lock()
	defer e.applyMu.Unlock()
	if st := e.cur.Load(); st != nil {
		e.stop(st)
	}
}

// Errors reports fatal scheduler errors (not intentional stops).
func (e *Engine) Errors() <-chan error { return e.errs }

func (e *Engine) Config() cfgpkg.Config           { return e.cur.Load().cfg }
func (e *Engine) Router() *router.Router          { return e.cur.Load().r }
func (e *Engine) Scheduler() *scheduler.Scheduler { return e.cur.Load().sched }
func (e *Engine) Latest() *frameinput.LatestFrame { return e.cur.Load().latest }
func (e *Engine) AppliedAt() time.Time            { return e.cur.Load().applied }
func (e *Engine) Mode() string                    { return e.opts.Mode }

// Submit hands one input frame (e.g. from PXLBLZ) to the current router.
// While a test pattern runs, input frames are dropped so the test is visible.
func (e *Engine) Submit(frame []byte) {
	if e.TestActive() {
		return
	}
	_ = e.cur.Load().latest.Submit(frame)
}

// --- test patterns ---------------------------------------------------------

// TestPatterns lists the patterns Test accepts.
var TestPatterns = []string{"chase", "rainbow", "port-id", "panel-walk", "white", "black"}

// Test runs a router-side test pattern for d, replacing the input. A new test
// replaces a running one.
func (e *Engine) Test(name string, d time.Duration) error {
	name = strings.ToLower(strings.TrimSpace(name))
	ok := false
	for _, p := range TestPatterns {
		ok = ok || p == name
	}
	if !ok {
		return fmt.Errorf("unbekanntes Testmuster %q", name)
	}
	if d <= 0 || d > 10*time.Minute {
		return fmt.Errorf("Testdauer muss 1 s bis 10 min sein")
	}
	e.StopTest()
	e.testMu.Lock()
	defer e.testMu.Unlock()
	ctx, cancel := context.WithTimeout(e.parent, d)
	e.testCancel = cancel
	e.testName.Store(name)
	e.testUntil.Store(time.Now().Add(d).UnixNano())
	go func() {
		e.runGenerator(ctx, nil, name, true)
		e.testUntil.Store(0)
		e.testName.Store("")
	}()
	return nil
}

// StopTest ends a running test pattern; input resumes with the next frame.
func (e *Engine) StopTest() {
	e.testMu.Lock()
	defer e.testMu.Unlock()
	if e.testCancel != nil {
		e.testCancel()
		e.testCancel = nil
	}
	e.testUntil.Store(0)
	e.testName.Store("")
}

// TestActive reports whether a test pattern currently replaces the input.
func (e *Engine) TestActive() bool {
	u := e.testUntil.Load()
	return u != 0 && time.Now().UnixNano() < u
}

// TestStatus returns the running test pattern and remaining time.
func (e *Engine) TestStatus() (string, time.Duration) {
	if !e.TestActive() {
		return "", 0
	}
	return e.testName.Load().(string), time.Until(time.Unix(0, e.testUntil.Load()))
}

// runGenerator fills frames for a pattern. With fixed == nil it follows the
// current state (so a config change during a test keeps working).
func (e *Engine) runGenerator(ctx context.Context, fixed *state, name string, test bool) {
	fps := 30
	if name == "chase" {
		fps = 4 // slow enough to follow the wiring order by eye
	}
	if !test {
		fps = e.cur.Load().cfg.Input.FPSTarget
		if fixed != nil {
			fps = fixed.cfg.Input.FPSTarget
		}
		if e.opts.FPSOverride > 0 {
			fps = e.opts.FPSOverride
		}
	}
	t := time.NewTicker(time.Second / time.Duration(fps))
	defer t.Stop()
	var n uint64
	var frame []byte
	for {
		st := fixed
		if st == nil {
			st = e.cur.Load()
		}
		pixels := st.cfg.Input.PixelCount
		if len(frame) != pixels*3 {
			frame = make([]byte, pixels*3)
		}
		// the normal --input pattern generator pauses while a test pattern runs
		if test || !e.TestActive() {
			if err := fill(frame, pixels, name, activeRoutes(st), n, e.opts.WalkSpeed); err == nil {
				_ = st.latest.Submit(frame)
			}
		}
		n++
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func activeRoutes(st *state) []cfgpkg.Route {
	on := map[string]bool{}
	for _, c := range st.r.Controllers() {
		on[c.TargetIP()] = true
	}
	var out []cfgpkg.Route
	for _, rt := range st.cfg.Routes {
		if rt.Enabled && on[rt.TargetIP] {
			out = append(out, rt)
		}
	}
	return out
}

func fill(frame []byte, pixels int, name string, routes []cfgpkg.Route, n uint64, walkSpeed int) error {
	switch strings.ToLower(name) {
	case "port-id":
		return pattern.FillPortID(frame, pixels, routes, n)
	case "panel-walk", "walk":
		if walkSpeed <= 0 {
			walkSpeed = 4
		}
		return pattern.FillPanelWalk(frame, pixels, routes, n, walkSpeed)
	default:
		return pattern.Fill(frame, pixels, name, n)
	}
}
