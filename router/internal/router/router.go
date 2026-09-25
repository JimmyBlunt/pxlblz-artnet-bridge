package router

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"pxlblz-router/internal/artnet"
	cfgpkg "pxlblz-router/internal/config"
)

type PlannedRoute struct {
	cfg       cfgpkg.Route
	byteStart int
	byteCount int
	universes int
	scratch   []byte
}

// Stats are cumulative counters. They are safe to read from any goroutine.
type Stats struct {
	Frames         uint64
	Packets        uint64
	Bytes          uint64
	SendErrors     uint64
	LastFrameTime  time.Duration
	MaxFrameTime   time.Duration
	TotalFrameTime time.Duration
}

type statCounters struct {
	frames, packets, bytes, sendErrors atomic.Uint64
	lastNS, maxNS, totalNS             atomic.Int64
}

func (s *statCounters) snapshot() Stats {
	return Stats{
		Frames:         s.frames.Load(),
		Packets:        s.packets.Load(),
		Bytes:          s.bytes.Load(),
		SendErrors:     s.sendErrors.Load(),
		LastFrameTime:  time.Duration(s.lastNS.Load()),
		MaxFrameTime:   time.Duration(s.maxNS.Load()),
		TotalFrameTime: time.Duration(s.totalNS.Load()),
	}
}

// Controller owns every route that targets one IP. The receiver assembles
// frames controller-wide, so everything that must be consistent within one
// received frame lives here: the Art-Net sequence, the packet burst and the
// output rate. Each Controller may be driven from its own goroutine; a single
// Controller must not be driven from two goroutines at once.
type Controller struct {
	cfg    cfgpkg.Controller
	addr   *net.UDPAddr
	routes []PlannedRoute
	seq    byte
	packet []byte
	stats  statCounters

	mu sync.Mutex // guards SendFrame against accidental concurrent use
}

func (c *Controller) Name() string     { return c.cfg.Name }
func (c *Controller) TargetIP() string { return c.cfg.TargetIP }
func (c *Controller) FPS() int         { return c.cfg.FPSTarget }
func (c *Controller) Stats() Stats     { return c.stats.snapshot() }

// ExpectedUniverses lists every universe this controller receives per frame,
// in transmit order.
func (c *Controller) ExpectedUniverses() []int {
	var out []int
	for _, r := range c.routes {
		for u := 0; u < r.universes; u++ {
			out = append(out, r.cfg.UniverseStart+u)
		}
	}
	return out
}

type Router struct {
	cfg         cfgpkg.Config
	conn        *net.UDPConn
	controllers []*Controller
	dryRun      bool
}

func New(cfg cfgpkg.Config, dryRun bool) (*Router, error) {
	r := &Router{cfg: cfg, dryRun: dryRun}
	if !dryRun {
		conn, err := net.ListenUDP("udp4", nil)
		if err != nil {
			return nil, err
		}
		r.conn = conn
	}
	byIP := map[string]*Controller{}
	for _, rc := range cfg.Routes {
		if !rc.Enabled {
			continue
		}
		ctrl, ok := byIP[rc.TargetIP]
		if !ok {
			cc := cfg.ControllerFor(rc.TargetIP)
			if !cc.IsEnabled() {
				continue
			}
			addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", rc.TargetIP, cfg.ArtNet.UDPPort))
			if err != nil {
				r.Close()
				return nil, fmt.Errorf("route %s: %w", rc.Name, err)
			}
			ctrl = &Controller{
				cfg:    cc,
				addr:   addr,
				seq:    1,
				packet: make([]byte, artnet.HeaderSize+artnet.MaxDmxPayload),
			}
			byIP[rc.TargetIP] = ctrl
			r.controllers = append(r.controllers, ctrl)
		}
		pr := PlannedRoute{
			cfg:       rc,
			byteStart: rc.PixelStart * 3,
			byteCount: rc.PixelCount * 3,
			universes: rc.Universes(),
		}
		if rc.ColorOrder != "RGB" {
			pr.scratch = make([]byte, pr.byteCount)
		}
		ctrl.routes = append(ctrl.routes, pr)
	}
	return r, nil
}

func (r *Router) Close() error {
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

func (r *Router) Controllers() []*Controller { return r.controllers }

// Stats aggregates all controllers. Frames, LastFrameTime and MaxFrameTime
// are the maximum over controllers; packets, bytes, errors and TotalFrameTime
// are summed. Use SendTotals for an average send time per controller frame.
func (r *Router) Stats() Stats {
	var out Stats
	for _, c := range r.controllers {
		s := c.Stats()
		out.Frames = max(out.Frames, s.Frames)
		out.Packets += s.Packets
		out.Bytes += s.Bytes
		out.SendErrors += s.SendErrors
		out.LastFrameTime = max(out.LastFrameTime, s.LastFrameTime)
		out.MaxFrameTime = max(out.MaxFrameTime, s.MaxFrameTime)
		out.TotalFrameTime += s.TotalFrameTime
	}
	return out
}

// SendTotals returns the number of controller frames sent and the time spent
// sending them, summed over all controllers.
func (r *Router) SendTotals() (frames uint64, total time.Duration) {
	for _, c := range r.controllers {
		s := c.Stats()
		frames += s.Frames
		total += s.TotalFrameTime
	}
	return frames, total
}

func (r *Router) RouteSummary() []string {
	var out []string
	for _, c := range r.controllers {
		out = append(out, fmt.Sprintf("[%s] %s  %d fps  %d universes/frame",
			c.cfg.Name, c.cfg.TargetIP, c.cfg.FPSTarget, len(c.ExpectedUniverses())))
		for _, p := range c.routes {
			out = append(out, fmt.Sprintf("  %-18s P%-2d pixels %d..%d  U%d..U%d  %s",
				p.cfg.Name, p.cfg.PhysicalPort,
				p.cfg.PixelStart, p.cfg.PixelStart+p.cfg.PixelCount-1,
				p.cfg.UniverseStart, p.cfg.UniverseStart+p.universes-1,
				p.cfg.ColorOrder))
		}
	}
	return out
}

func (r *Router) checkFrame(frame []byte) error {
	if expected := r.cfg.Input.PixelCount * 3; len(frame) != expected {
		return fmt.Errorf("frame length %d, expected %d", len(frame), expected)
	}
	return nil
}

// SendFrame transmits one complete logical frame to every controller.
func (r *Router) SendFrame(frame []byte) error {
	if err := r.checkFrame(frame); err != nil {
		return err
	}
	for _, c := range r.controllers {
		if err := r.sendController(c, frame); err != nil {
			return err
		}
	}
	return nil
}

// SendController transmits one complete logical frame to a single controller.
func (r *Router) SendController(c *Controller, frame []byte) error {
	if err := r.checkFrame(frame); err != nil {
		return err
	}
	return r.sendController(c, frame)
}

func (r *Router) sendController(c *Controller, frame []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	started := time.Now()
	seq := c.seq
	var packets, bytes, errs uint64

	for i := range c.routes {
		route := &c.routes[i]
		data := frame[route.byteStart : route.byteStart+route.byteCount]
		if route.scratch != nil {
			reorder(route.scratch, data, route.cfg.ColorOrder)
			data = route.scratch
		}

		sent := 0
		universe := route.cfg.UniverseStart
		for sent < len(data) {
			n := artnet.MaxChannelsPerUniverse
			if remain := len(data) - sent; remain < n {
				n = remain
			}
			pkt, err := artnet.BuildDmx(c.packet, uint16(universe), seq, data[sent:sent+n])
			if err != nil {
				return fmt.Errorf("route %s U%d: %w", route.cfg.Name, universe, err)
			}
			if !r.dryRun {
				if _, err := r.conn.WriteToUDP(pkt, c.addr); err != nil {
					errs++
				}
			}
			packets++
			bytes += uint64(len(pkt))
			sent += n
			universe++
		}
	}

	// Sequence advances only after the whole controller frame is on the wire.
	c.seq++
	if c.seq == 0 {
		c.seq = 1
	}

	dt := time.Since(started)
	c.stats.frames.Add(1)
	c.stats.packets.Add(packets)
	c.stats.bytes.Add(bytes)
	c.stats.sendErrors.Add(errs)
	c.stats.lastNS.Store(int64(dt))
	c.stats.totalNS.Add(int64(dt))
	if int64(dt) > c.stats.maxNS.Load() {
		c.stats.maxNS.Store(int64(dt))
	}
	return nil
}

func reorder(dst, src []byte, order string) {
	idx := [3]int{0, 1, 2}
	for i, c := range strings.ToUpper(order) {
		switch c {
		case 'R':
			idx[i] = 0
		case 'G':
			idx[i] = 1
		case 'B':
			idx[i] = 2
		}
	}
	for i := 0; i < len(src); i += 3 {
		dst[i] = src[i+idx[0]]
		dst[i+1] = src[i+idx[1]]
		dst[i+2] = src[i+idx[2]]
	}
}
