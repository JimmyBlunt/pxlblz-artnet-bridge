// Package scheduler drives every controller at its own output rate from one
// shared LatestFrame.
//
//	LatestFrame (newest logical RGB frame, written by WS input or pattern gen)
//	     ├── controller A goroutine @ fps A → own snapshot → own seq → burst
//	     ├── controller B goroutine @ fps B → own snapshot → own seq → burst
//	     └── …
//
// Each controller takes ONE snapshot per tick and sends all of its universes
// from that snapshot with one sequence number, so controller-wide frame
// completeness on the receiver is preserved. Controllers never wait for each
// other and never queue frames: a slow controller just samples less often.
package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	cfgpkg "pxlblz-router/internal/config"
	"pxlblz-router/internal/frameinput"
	"pxlblz-router/internal/router"
)

type Options struct {
	// FPSOverride, when > 0, replaces every controller's configured FPS.
	FPSOverride  int
	StaleTimeout time.Duration
	OnStale      string // hold | blackout | stop
	// Now is injectable for tests.
	Now func() time.Time
}

// ControllerStatus is a point-in-time view of one scheduled controller.
type ControllerStatus struct {
	Name     string       `json:"name"`
	TargetIP string       `json:"target_ip"`
	FPS      int          `json:"fps_target"`
	Stats    router.Stats `json:"-"`
	Frames   uint64       `json:"frames"`
	Packets  uint64       `json:"packets"`
	Errors   uint64       `json:"send_errors"`
	Repeats  uint64       `json:"repeated_frames"`
	Skipped  uint64       `json:"skipped_ticks"`
	Stale    bool         `json:"stale"`
	LastMS   float64      `json:"last_send_ms"`
	MaxMS    float64      `json:"max_send_ms"`
	Universe []int        `json:"universes"`
}

type ctrlState struct {
	ctrl    *router.Controller
	fps     int
	buf     []byte
	lastGen uint64
	repeats atomic.Uint64
	skipped atomic.Uint64
	stale   atomic.Bool
}

type Scheduler struct {
	r      *router.Router
	latest *frameinput.LatestFrame
	opts   Options
	black  []byte
	states []*ctrlState
}

func New(r *router.Router, latest *frameinput.LatestFrame, opts Options) *Scheduler {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.OnStale == "" {
		opts.OnStale = cfgpkg.OnStaleHold
	}
	s := &Scheduler{r: r, latest: latest, opts: opts, black: make([]byte, latest.Size())}
	for _, c := range r.Controllers() {
		fps := c.FPS()
		if opts.FPSOverride > 0 {
			fps = opts.FPSOverride
		}
		s.states = append(s.states, &ctrlState{ctrl: c, fps: fps, buf: make([]byte, latest.Size())})
	}
	return s
}

// Run blocks until ctx is cancelled or a controller reports a fatal error.
func (s *Scheduler) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	errCh := make(chan error, len(s.states))
	for _, st := range s.states {
		wg.Add(1)
		go func(st *ctrlState) {
			defer wg.Done()
			if err := s.runController(ctx, st); err != nil {
				errCh <- err
				cancel()
			}
		}(st)
	}
	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (s *Scheduler) runController(ctx context.Context, st *ctrlState) error {
	t := time.NewTicker(time.Second / time.Duration(st.fps))
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if err := s.tick(st); err != nil {
				return err
			}
		}
	}
}

// tick performs one output decision for one controller. Exposed through
// TickAll for deterministic tests.
func (s *Scheduler) tick(st *ctrlState) error {
	have, fresh, gen, at, err := s.latest.Snapshot(st.buf, st.lastGen)
	if err != nil {
		return err
	}
	if !have {
		// Never transmit before the first valid frame: avoids blacking out
		// the installation just because the router started first.
		st.skipped.Add(1)
		return nil
	}
	st.lastGen = gen
	frame := st.buf
	stale := s.opts.StaleTimeout > 0 && !at.IsZero() && s.opts.Now().Sub(at) > s.opts.StaleTimeout
	st.stale.Store(stale)
	if stale {
		switch s.opts.OnStale {
		case cfgpkg.OnStaleStop:
			st.skipped.Add(1)
			return nil
		case cfgpkg.OnStaleBlackout:
			frame = s.black
		}
	}
	if !fresh {
		st.repeats.Add(1)
	}
	return s.r.SendController(st.ctrl, frame)
}

// TickAll runs one Tick on every controller (test / single-step helper).
func (s *Scheduler) TickAll() error {
	for _, st := range s.states {
		if err := s.tick(st); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) Status() []ControllerStatus {
	out := make([]ControllerStatus, 0, len(s.states))
	for _, st := range s.states {
		cs := st.ctrl.Stats()
		out = append(out, ControllerStatus{
			Name:     st.ctrl.Name(),
			TargetIP: st.ctrl.TargetIP(),
			FPS:      st.fps,
			Stats:    cs,
			Frames:   cs.Frames,
			Packets:  cs.Packets,
			Errors:   cs.SendErrors,
			Repeats:  st.repeats.Load(),
			Skipped:  st.skipped.Load(),
			Stale:    st.stale.Load(),
			LastMS:   float64(cs.LastFrameTime.Microseconds()) / 1000,
			MaxMS:    float64(cs.MaxFrameTime.Microseconds()) / 1000,
			Universe: st.ctrl.ExpectedUniverses(),
		})
	}
	return out
}
