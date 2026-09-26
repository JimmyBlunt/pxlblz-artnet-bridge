package virtualcontroller

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"pxlblz-router/internal/artnet"
	cfgpkg "pxlblz-router/internal/config"
)

const (
	DefaultPartialTimeout = 100 * time.Millisecond
	DefaultSequenceReset  = 1000 * time.Millisecond
	DefaultLossTimeout    = 1000 * time.Millisecond
	DefaultPixelWireTime  = 30 * time.Microsecond
	DefaultLatchTime      = 300 * time.Microsecond
)

type Options struct {
	OutputFPS       int
	PartialTimeout  time.Duration
	SequenceReset   time.Duration
	LossTimeout     time.Duration
	PixelWireTime   time.Duration
	LatchTime       time.Duration
}

type Counters struct {
	Packets          uint64 `json:"packets"`
	Accepted         uint64 `json:"accepted"`
	Rejected         uint64 `json:"rejected"`
	Ignored          uint64 `json:"ignored"`
	Stale            uint64 `json:"stale"`
	Duplicates       uint64 `json:"duplicates"`
	Complete         uint64 `json:"complete"`
	Incomplete       uint64 `json:"incomplete"`
	CompleteReplaced uint64 `json:"complete_replaced"`
	FramesSubmitted  uint64 `json:"frames_submitted"`
	DMACompleted     uint64 `json:"dma_completed"`
	Blackouts        uint64 `json:"blackouts"`
}

type UniverseInfo struct {
	Universe   uint16 `json:"universe"`
	DataBytes  int    `json:"data_bytes"`
	MinPayload int    `json:"min_payload"`
}

type UniverseStatus struct {
	Universe      uint16  `json:"universe"`
	DataBytes     int     `json:"data_bytes"`
	MinPayload    int     `json:"min_payload"`
	Received      bool    `json:"received"`
	Packets       uint64  `json:"packets"`
	LastSeenAgeMS float64 `json:"last_seen_age_ms"`
}

type RouteInfo struct {
	Name          string  `json:"name"`
	PhysicalPort  int     `json:"physical_port"`
	PixelCount    int     `json:"pixel_count"`
	UniverseStart int     `json:"universe_start"`
	UniverseEnd   int     `json:"universe_end"`
	ColorOrder    string  `json:"color_order"`
	Brightness    float64 `json:"brightness"`
	FrameOffset   int     `json:"frame_offset"`
}

type TimingStats struct {
	IngestAvgUS    float64 `json:"ingest_avg_us"`
	IngestP95US    float64 `json:"ingest_p95_us"`
	IngestP99US    float64 `json:"ingest_p99_us"`
	IngestMaxUS    float64 `json:"ingest_max_us"`
	AssemblyAvgUS  float64 `json:"assembly_avg_us"`
	AssemblyP95US  float64 `json:"assembly_p95_us"`
	AssemblyP99US  float64 `json:"assembly_p99_us"`
	AssemblyMaxUS  float64 `json:"assembly_max_us"`
}

type Snapshot struct {
	TargetIP          string       `json:"target_ip"`
	State             string       `json:"state"`
	SequenceMode      string       `json:"sequence_mode"`
	Sequence          int          `json:"sequence"`
	SequenceSealed    bool         `json:"sequence_sealed"`
	CandidateReceived int          `json:"candidate_received"`
	ExpectedUniverses int          `json:"expected_universes"`
	CandidateAgeMS    float64      `json:"candidate_age_ms"`
	OutputFPS         int          `json:"output_fps"`
	WireGuardUS       int64        `json:"wire_guard_us"`
	DMABusy           bool         `json:"dma_busy"`
	BlackLatched      bool         `json:"black_latched"`
	DisplayGeneration uint64       `json:"display_generation"`
	PublishedGeneration uint64     `json:"published_generation"`
	Counters          Counters     `json:"counters"`
	Timing            TimingStats  `json:"timing"`
	Routes            []RouteInfo      `json:"routes"`
	Universes         []UniverseStatus `json:"universes"`
}

type expectedUniverse struct {
	universe   uint16
	bit        uint64
	frameOff   int
	dataBytes  int
	minPayload int
}

type routeRuntime struct {
	cfg       cfgpkg.Route
	frameOff  int
	byteCount int
	universes int
}

type sampleRing struct {
	values [2048]int64
	count  int
	next   int
	sum    int64
	max    int64
}

func (r *sampleRing) add(v time.Duration) {
	n := v.Nanoseconds()
	if r.count < len(r.values) {
		r.values[r.next] = n
		r.count++
		r.sum += n
	} else {
		r.sum -= r.values[r.next]
		r.values[r.next] = n
		r.sum += n
	}
	r.next = (r.next + 1) % len(r.values)
	if n > r.max {
		r.max = n
	}
}

func (r *sampleRing) stats() (avg, p95, p99, max float64) {
	if r.count == 0 {
		return
	}
	tmp := make([]int64, r.count)
	copy(tmp, r.values[:r.count])
	sort.Slice(tmp, func(i, j int) bool { return tmp[i] < tmp[j] })
	pick := func(q float64) int64 {
		idx := int(float64(len(tmp)-1) * q)
		return tmp[idx]
	}
	return float64(r.sum)/float64(r.count)/1000.0,
		float64(pick(.95))/1000.0,
		float64(pick(.99))/1000.0,
		float64(r.max)/1000.0
}

type Controller struct {
	mu sync.Mutex

	targetIP string
	opts     Options
	routes   []routeRuntime
	routeInfo []RouteInfo
	expected map[uint16]expectedUniverse
	universePackets map[uint16]uint64
	universeLastSeen map[uint16]time.Time
	expectedMask uint64
	expectedCount int

	candidate []byte
	published []byte
	display   []byte
	candidateMask uint64
	candidateStarted time.Time
	publishedReady bool
	publishedGeneration uint64
	displayGeneration uint64

	seqMode byte // 0=unknown, 1=sequence zero, 2=non-zero
	seqValid bool
	seq byte
	sealed bool
	lastAccepted time.Time
	lastComplete time.Time

	nextOutput time.Time
	dmaBusy bool
	dmaReadyAt time.Time
	blackLatched bool
	everRunning bool

	counters Counters
	ingestTiming sampleRing
	assemblyTiming sampleRing
}

func defaultOptions(o Options, cfgFPS int) Options {
	if o.OutputFPS == 0 { o.OutputFPS = cfgFPS }
	if o.PartialTimeout == 0 { o.PartialTimeout = DefaultPartialTimeout }
	if o.SequenceReset == 0 { o.SequenceReset = DefaultSequenceReset }
	if o.LossTimeout == 0 { o.LossTimeout = DefaultLossTimeout }
	if o.PixelWireTime == 0 { o.PixelWireTime = DefaultPixelWireTime }
	if o.LatchTime == 0 { o.LatchTime = DefaultLatchTime }
	return o
}

func New(cfg cfgpkg.Config, targetIP string, opts Options) (*Controller, error) {
	opts = defaultOptions(opts, cfg.Input.FPSTarget)
	if opts.OutputFPS < 1 || opts.OutputFPS > 240 {
		return nil, fmt.Errorf("output FPS must be 1..240")
	}
	if opts.PartialTimeout <= 0 || opts.SequenceReset <= 0 || opts.LossTimeout <= 0 {
		return nil, fmt.Errorf("timeouts must be > 0")
	}
	c := &Controller{
		targetIP: targetIP,
		opts: opts,
		expected: map[uint16]expectedUniverse{},
		universePackets: map[uint16]uint64{},
		universeLastSeen: map[uint16]time.Time{},
	}
	totalBytes := 0
	expectedIndex := 0
	for _, r := range cfg.Routes {
		if !r.Enabled || r.TargetIP != targetIP {
			continue
		}
		rr := routeRuntime{
			cfg: r,
			frameOff: totalBytes,
			byteCount: r.PixelCount * 3,
			universes: (r.PixelCount + 169) / 170,
		}
		c.routes = append(c.routes, rr)
		c.routeInfo = append(c.routeInfo, RouteInfo{
			Name: r.Name,
			PhysicalPort: r.PhysicalPort,
			PixelCount: r.PixelCount,
			UniverseStart: r.UniverseStart,
			UniverseEnd: r.UniverseStart + rr.universes - 1,
			ColorOrder: r.ColorOrder,
			Brightness: r.BrightnessLevel(),
			FrameOffset: totalBytes,
		})
		remain := rr.byteCount
		for u := 0; u < rr.universes; u++ {
			if expectedIndex >= 64 {
				return nil, fmt.Errorf("virtual receiver supports at most 64 expected universes")
			}
			dataBytes := 510
			if remain < dataBytes { dataBytes = remain }
			minPayload := dataBytes
			if minPayload&1 != 0 { minPayload++ }
			universe := uint16(r.UniverseStart + u)
			if _, exists := c.expected[universe]; exists {
				return nil, fmt.Errorf("duplicate expected universe %d for %s", universe, targetIP)
			}
			bit := uint64(1) << expectedIndex
			c.expected[universe] = expectedUniverse{
				universe: universe,
				bit: bit,
				frameOff: totalBytes + u*510,
				dataBytes: dataBytes,
				minPayload: minPayload,
			}
			c.expectedMask |= bit
			expectedIndex++
			remain -= dataBytes
		}
		totalBytes += rr.byteCount
	}
	if len(c.routes) == 0 {
		return nil, fmt.Errorf("no enabled routes for target IP %s", targetIP)
	}
	c.expectedCount = expectedIndex
	c.candidate = make([]byte, totalBytes)
	c.published = make([]byte, totalBytes)
	c.display = make([]byte, totalBytes)
	return c, nil
}

func sequenceDistance(current, next byte) int {
	if current == next { return 0 }
	a := int(current) - 1
	b := int(next) - 1
	d := (b - a + 255) % 255
	return d
}

func bitCount(v uint64) int {
	n := 0
	for v != 0 {
		v &= v - 1
		n++
	}
	return n
}

func (c *Controller) abandonLocked() {
	if c.candidateMask != 0 {
		c.counters.Incomplete++
	}
	c.candidateMask = 0
	c.candidateStarted = time.Time{}
}

func (c *Controller) expireLocked(now time.Time) {
	if c.candidateMask != 0 && !c.candidateStarted.IsZero() && now.Sub(c.candidateStarted) > c.opts.PartialTimeout {
		c.abandonLocked()
	}
	if !c.lastAccepted.IsZero() && now.Sub(c.lastAccepted) > c.opts.SequenceReset {
		c.seqMode = 0
		c.seqValid = false
		c.seq = 0
		c.sealed = false
	}
}

func (c *Controller) IngestPacket(pkt []byte, now time.Time) {
	started := time.Now()
	c.mu.Lock()
	defer func() {
		c.ingestTiming.add(time.Since(started))
		c.mu.Unlock()
	}()

	c.counters.Packets++
	c.expireLocked(now)

	p, err := artnet.ParseDmx(pkt)
	if err != nil {
		c.counters.Rejected++
		return
	}
	e, ok := c.expected[p.Universe]
	if !ok {
		c.counters.Ignored++
		return
	}
	if len(p.Data) < e.minPayload {
		c.counters.Rejected++
		return
	}

	if p.Sequence == 0 {
		if c.seqMode == 2 {
			c.abandonLocked()
			c.seqValid = false
			c.seq = 0
			c.sealed = false
		}
		c.seqMode = 1
		if c.candidateMask&e.bit != 0 {
			// Sequence zero has no ordering. A duplicate universe before
			// completion starts a new candidate with this packet.
			c.counters.Duplicates++
			c.abandonLocked()
		}
	} else {
		if c.seqMode == 1 {
			c.abandonLocked()
			c.seqMode = 2
			c.seqValid = true
			c.seq = p.Sequence
			c.sealed = false
		} else if c.seqMode == 0 || !c.seqValid {
			c.seqMode = 2
			c.seqValid = true
			c.seq = p.Sequence
			c.sealed = false
		} else if p.Sequence == c.seq {
			if c.sealed || c.candidateMask&e.bit != 0 {
				c.counters.Duplicates++
				return
			}
		} else {
			d := sequenceDistance(c.seq, p.Sequence)
			if d >= 1 && d <= 127 {
				c.abandonLocked()
				c.seq = p.Sequence
				c.sealed = false
			} else {
				c.counters.Stale++
				return
			}
		}
	}

	if c.candidateMask == 0 {
		c.candidateStarted = now
	}
	copy(c.candidate[e.frameOff:e.frameOff+e.dataBytes], p.Data[:e.dataBytes])
	c.candidateMask |= e.bit
	c.counters.Accepted++
	c.universePackets[p.Universe]++
	c.universeLastSeen[p.Universe] = now
	c.lastAccepted = now

	if c.candidateMask == c.expectedMask {
		if c.publishedReady {
			c.counters.CompleteReplaced++
		}
		copy(c.published, c.candidate)
		c.publishedReady = true
		c.publishedGeneration++
		c.counters.Complete++
		c.lastComplete = now
		if !c.candidateStarted.IsZero() {
			c.assemblyTiming.add(now.Sub(c.candidateStarted))
		}
		c.candidateMask = 0
		c.candidateStarted = time.Time{}
		if p.Sequence != 0 {
			c.sealed = true
		}
	}
}

func (c *Controller) wireGuardLocked() time.Duration {
	longest := 0
	for _, r := range c.routes {
		if r.cfg.PixelCount > longest { longest = r.cfg.PixelCount }
	}
	return time.Duration(longest)*c.opts.PixelWireTime + c.opts.LatchTime
}

func (c *Controller) Tick(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.expireLocked(now)
	if c.dmaBusy && !now.Before(c.dmaReadyAt) {
		c.dmaBusy = false
		c.counters.DMACompleted++
	}

	period := time.Second / time.Duration(c.opts.OutputFPS)
	if c.nextOutput.IsZero() {
		c.nextOutput = now
	}

	if c.publishedReady && !c.dmaBusy && !now.Before(c.nextOutput) {
		copy(c.display, c.published)
		c.publishedReady = false
		c.displayGeneration = c.publishedGeneration
		c.counters.FramesSubmitted++
		c.everRunning = true
		c.blackLatched = false
		c.dmaBusy = true
		c.dmaReadyAt = now.Add(c.wireGuardLocked())
		c.nextOutput = now.Add(period)
		return
	}

	if c.everRunning && !c.blackLatched && !c.lastComplete.IsZero() && now.Sub(c.lastComplete) > c.opts.LossTimeout && !c.dmaBusy {
		clear(c.display)
		c.displayGeneration++
		c.blackLatched = true
		c.counters.Blackouts++
		c.dmaBusy = true
		c.dmaReadyAt = now.Add(c.wireGuardLocked())
		c.nextOutput = now.Add(period)
	}
}

func (c *Controller) Snapshot(now time.Time) Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expireLocked(now)
	ia, ip95, ip99, imax := c.ingestTiming.stats()
	aa, ap95, ap99, amax := c.assemblyTiming.stats()
	mode := "unknown"
	switch c.seqMode {
	case 1: mode = "sequence-zero"
	case 2: mode = "sequenced"
	}
	state := "waiting"
	if c.blackLatched {
		state = "blackout"
	} else if c.everRunning {
		state = "running-artnet"
	} else if c.publishedReady {
		state = "frame-ready"
	}
	age := 0.0
	if c.candidateMask != 0 && !c.candidateStarted.IsZero() {
		age = float64(now.Sub(c.candidateStarted).Microseconds()) / 1000.0
	}
	seq := -1
	if c.seqValid || c.seqMode == 1 {
		seq = int(c.seq)
	}
	universeStatus := make([]UniverseStatus, 0, len(c.expected))
	for u, e := range c.expected {
		age := -1.0
		if last, ok := c.universeLastSeen[u]; ok && !last.IsZero() {
			age = float64(now.Sub(last).Microseconds()) / 1000.0
		}
		universeStatus = append(universeStatus, UniverseStatus{
			Universe: u,
			DataBytes: e.dataBytes,
			MinPayload: e.minPayload,
			Received: c.candidateMask&e.bit != 0,
			Packets: c.universePackets[u],
			LastSeenAgeMS: age,
		})
	}
	sort.Slice(universeStatus, func(i, j int) bool { return universeStatus[i].Universe < universeStatus[j].Universe })

	return Snapshot{
		TargetIP: c.targetIP,
		State: state,
		SequenceMode: mode,
		Sequence: seq,
		SequenceSealed: c.sealed,
		CandidateReceived: bitCount(c.candidateMask),
		ExpectedUniverses: c.expectedCount,
		CandidateAgeMS: age,
		OutputFPS: c.opts.OutputFPS,
		WireGuardUS: c.wireGuardLocked().Microseconds(),
		DMABusy: c.dmaBusy,
		BlackLatched: c.blackLatched,
		DisplayGeneration: c.displayGeneration,
		PublishedGeneration: c.publishedGeneration,
		Counters: c.counters,
		Timing: TimingStats{
			IngestAvgUS: ia, IngestP95US: ip95, IngestP99US: ip99, IngestMaxUS: imax,
			AssemblyAvgUS: aa, AssemblyP95US: ap95, AssemblyP99US: ap99, AssemblyMaxUS: amax,
		},
		Routes: append([]RouteInfo(nil), c.routeInfo...),
		Universes: universeStatus,
	}
}

func (c *Controller) DisplayRGB() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]byte, len(c.display))
	for _, r := range c.routes {
		src := c.display[r.frameOff:r.frameOff+r.byteCount]
		dst := out[r.frameOff:r.frameOff+r.byteCount]
		wireToRGB(dst, src, r.cfg.ColorOrder)
	}
	return out
}

func wireToRGB(dst, src []byte, order string) {
	order = strings.ToUpper(order)
	for i := 0; i+2 < len(src); i += 3 {
		var rgb [3]byte
		for j, ch := range order {
			switch ch {
			case 'R': rgb[0] = src[i+j]
			case 'G': rgb[1] = src[i+j]
			case 'B': rgb[2] = src[i+j]
			}
		}
		dst[i], dst[i+1], dst[i+2] = rgb[0], rgb[1], rgb[2]
	}
}

func (c *Controller) UniverseInfo() []UniverseInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]UniverseInfo, 0, len(c.expected))
	for _, e := range c.expected {
		out = append(out, UniverseInfo{Universe: e.universe, DataBytes: e.dataBytes, MinPayload: e.minPayload})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Universe < out[j].Universe })
	return out
}

func (c *Controller) ExpectedUniverses() []uint16 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]uint16, 0, len(c.expected))
	for u := range c.expected { out = append(out, u) }
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
