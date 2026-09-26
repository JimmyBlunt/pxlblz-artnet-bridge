package router

import (
	"fmt"
	"math"
	"net"
	"strings"
	"time"

	"pxlblz-router/internal/artnet"
	cfgpkg "pxlblz-router/internal/config"
)

type PlannedRoute struct {
	cfg       cfgpkg.Route
	addr      *net.UDPAddr
	byteStart int
	byteCount int
	universes int
	scratch       []byte
	brightnessLUT *[256]byte
}

type Stats struct {
	Frames         uint64
	Packets        uint64
	Bytes          uint64
	SendErrors     uint64
	LastFrameTime  time.Duration
	MaxFrameTime   time.Duration
	TotalFrameTime time.Duration
}

type Router struct {
	cfg    cfgpkg.Config
	conn   *net.UDPConn
	routes []PlannedRoute
	seq    byte
	packet []byte
	stats  Stats
	dryRun bool
}

func New(cfg cfgpkg.Config, dryRun bool) (*Router, error) {
	r := &Router{
		cfg:    cfg,
		packet: make([]byte, artnet.HeaderSize+artnet.MaxChannelsPerUniverse),
		dryRun: dryRun,
		seq:    1,
	}
	if !dryRun {
		conn, err := net.ListenUDP("udp4", nil)
		if err != nil {
			return nil, err
		}
		r.conn = conn
	}
	for _, c := range cfg.Routes {
		if !c.Enabled {
			continue
		}
		addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", c.TargetIP, cfg.ArtNet.UDPPort))
		if err != nil {
			r.Close()
			return nil, fmt.Errorf("route %s: %w", c.Name, err)
		}
		pr := PlannedRoute{
			cfg:       c,
			addr:      addr,
			byteStart: c.PixelStart * 3,
			byteCount: c.PixelCount * 3,
			universes: (c.PixelCount + 169) / 170,
		}
		level := c.BrightnessLevel()
		if level != 1 {
			lut := &[256]byte{}
			for i := 0; i < 256; i++ {
				lut[i] = byte(math.Round(float64(i) * level))
			}
			pr.brightnessLUT = lut
		}
		if c.ColorOrder != "RGB" || pr.brightnessLUT != nil {
			pr.scratch = make([]byte, pr.byteCount)
		}
		r.routes = append(r.routes, pr)
	}
	return r, nil
}

func (r *Router) Close() error {
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

func (r *Router) Stats() Stats { return r.stats }

func (r *Router) RouteSummary() []string {
	out := make([]string, 0, len(r.routes))
	for _, p := range r.routes {
		out = append(out, fmt.Sprintf("%-18s %-15s P%-2d pixels %d..%d  U%d..U%d  %s  level %.2f",
			p.cfg.Name, p.cfg.TargetIP, p.cfg.PhysicalPort,
			p.cfg.PixelStart, p.cfg.PixelStart+p.cfg.PixelCount-1,
			p.cfg.UniverseStart, p.cfg.UniverseStart+p.universes-1,
			p.cfg.ColorOrder, p.cfg.BrightnessLevel()))
	}
	return out
}

func (r *Router) SendFrame(frame []byte) error {
	expected := r.cfg.Input.PixelCount * 3
	if len(frame) != expected {
		return fmt.Errorf("frame length %d, expected %d", len(frame), expected)
	}
	started := time.Now()
	seq := r.seq

	for i := range r.routes {
		route := &r.routes[i]
		data := frame[route.byteStart : route.byteStart+route.byteCount]
		if route.scratch != nil {
			reorderScale(route.scratch, data, route.cfg.ColorOrder, route.brightnessLUT)
			data = route.scratch
		}

		sent := 0
		universe := route.cfg.UniverseStart
		for sent < len(data) {
			n := artnet.MaxChannelsPerUniverse
			if remain := len(data) - sent; remain < n {
				n = remain
			}
			pkt, err := artnet.BuildDmx(r.packet, uint16(universe), seq, data[sent:sent+n])
			if err != nil {
				return fmt.Errorf("route %s U%d: %w", route.cfg.Name, universe, err)
			}
			if !r.dryRun {
				if _, err := r.conn.WriteToUDP(pkt, route.addr); err != nil {
					r.stats.SendErrors++
				}
			}
			r.stats.Packets++
			r.stats.Bytes += uint64(len(pkt))
			sent += n
			universe++
		}
	}

	r.stats.Frames++
	dt := time.Since(started)
	r.stats.LastFrameTime = dt
	r.stats.TotalFrameTime += dt
	if dt > r.stats.MaxFrameTime {
		r.stats.MaxFrameTime = dt
	}
	r.seq++
	if r.seq == 0 {
		r.seq = 1
	}
	return nil
}

func reorder(dst, src []byte, order string) {
	reorderScale(dst, src, order, nil)
}

func reorderScale(dst, src []byte, order string, lut *[256]byte) {
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
		a := src[i+idx[0]]
		b := src[i+idx[1]]
		cc := src[i+idx[2]]
		if lut != nil {
			a = lut[a]
			b = lut[b]
			cc = lut[cc]
		}
		dst[i] = a
		dst[i+1] = b
		dst[i+2] = cc
	}
}
