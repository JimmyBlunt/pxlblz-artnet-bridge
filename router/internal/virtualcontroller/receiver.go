package virtualcontroller

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	cfgpkg "pxlblz-router/internal/config"
)

const (
	DefaultPartialTimeout = 100 * time.Millisecond
	DefaultSequenceReset  = 1000 * time.Millisecond
	DefaultLossTimeout    = 1000 * time.Millisecond
	DefaultPixelWireTime  = 30 * time.Microsecond
	DefaultLatchTime      = 300 * time.Microsecond
)

type runState byte

const (
	stateWaitBlackout runState = iota
	stateWaitBlackoutCompletion
	stateRunningArtNet
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
	ArtNetWaiting     bool         `json:"artnet_waiting"`
	Link              bool         `json:"link"`
	DisplayGeneration uint64       `json:"display_generation"`
	PublishedGeneration uint64     `json:"published_generation"`
	PublishedReady      bool       `json:"published_ready"`
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

	state runState
	policyEnabled bool
	policyWaiting bool
	link bool
	lastSubmit time.Time
	dmaBusy bool
	dmaReadyAt time.Time
	blackLatched bool

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
		state: stateWaitBlackout,
		policyEnabled: true,
		policyWaiting: true,
		link: true,
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

func (c *Controller) clearReceiverLocked() {
	c.candidateMask = 0
	c.candidateStarted = time.Time{}
	c.publishedReady = false
	c.seqMode = 0
	c.seqValid = false
	c.sealed = false
}

func (c *Controller) expireLocked(now time.Time) {
	if c.candidateMask != 0 && !c.candidateStarted.IsZero() && now.Sub(c.candidateStarted) > c.opts.PartialTimeout {
		c.abandonLocked()
	}
	if !c.lastAccepted.IsZero() && now.Sub(c.lastAccepted) > c.opts.SequenceReset {
		c.seqMode = 0
		c.seqValid = false
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

	// runtime_receiver.h increments packets before every validation branch.
	c.counters.Packets++

	// Exact active Teensy receiver header checks:
	// Art-Net\0, OpDmx 0x5000 (little-endian bytes 00 50), protocol >= 14,
	// even ArtDmx length 2..512, exact UDP payload size, 15-bit port-address.
	if len(pkt) < 18 ||
		string(pkt[:8]) != "Art-Net\x00" ||
		pkt[8] != 0 || pkt[9] != 0x50 ||
		(int(pkt[10])*256+int(pkt[11])) < 14 {
		c.counters.Rejected++
		return
	}
	length := int(pkt[16])*256 + int(pkt[17])
	if length < 2 || length > 512 || length&1 != 0 || len(pkt) != 18+length || pkt[15]&0x80 != 0 {
		c.counters.Rejected++
		return
	}
	universe := uint16(pkt[14]) + uint16(pkt[15])*256
	e, ok := c.expected[universe]
	if !ok {
		c.counters.Ignored++
		return
	}
	// Firmware compares against the useful route length. Since wire length must
	// already be even, an odd useful tail such as 99 bytes naturally requires 100.
	if length < e.dataBytes {
		c.counters.Rejected++
		return
	}

	// The firmware calls expire() only after the packet passed header, universe
	// lookup and route-length validation. Rejected/ignored traffic cannot keep or
	// advance the candidate/sequence timeout state.
	c.expireLocked(now)

	seq := pkt[12]
	if seq != 0 {
		if !c.seqValid && c.candidateMask != 0 {
			c.abandonLocked()
		}
		if c.seqValid && seq != c.seq {
			delta := (int(seq) + 255 - int(c.seq)) % 255
			if delta > 127 {
				c.counters.Stale++
				return
			}
			c.abandonLocked()
		} else if c.seqValid && seq == c.seq && c.sealed {
			c.counters.Duplicates++
			return
		}
		if !c.seqValid || seq != c.seq {
			c.seq = seq
			c.seqValid = true
			c.sealed = false
		}
		c.seqMode = 2
	} else {
		if c.seqValid {
			c.abandonLocked()
		}
		c.seqValid = false
		c.sealed = false
		c.seqMode = 1
	}

	if c.candidateMask&e.bit != 0 {
		c.counters.Duplicates++
		if seq != 0 {
			return
		}
		// Sequence zero carries no ordering information. The active firmware
		// abandons the current partial candidate and starts again with the
		// duplicate packet.
		c.abandonLocked()
	}

	if c.candidateMask == 0 {
		c.candidateStarted = now
	}
	copy(c.candidate[e.frameOff:e.frameOff+e.dataBytes], pkt[18:18+e.dataBytes])
	c.candidateMask |= e.bit
	c.counters.Accepted++
	c.universePackets[universe]++
	c.universeLastSeen[universe] = now
	c.lastAccepted = now

	if c.candidateMask != c.expectedMask {
		return
	}
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
	c.sealed = seq != 0
}

func (c *Controller) wireGuardLocked() time.Duration {
	longest := 0
	for _, r := range c.routes {
		if r.cfg.PixelCount > longest {
			longest = r.cfg.PixelCount
		}
	}
	return time.Duration(longest)*c.opts.PixelWireTime + c.opts.LatchTime
}

func (c *Controller) framePeriodLocked() time.Duration {
	// web_main.cpp uses ceil(1,000,000 / target_fps) microseconds and then
	// clamps the period to the longest physical wire guard.
	us := (1_000_000 + c.opts.OutputFPS - 1) / c.opts.OutputFPS
	period := time.Duration(us) * time.Microsecond
	if guard := c.wireGuardLocked(); guard > period {
		period = guard
	}
	return period
}

func (c *Controller) transferReadyLocked(now time.Time) bool {
	if !c.dmaBusy {
		return true
	}
	if now.Before(c.dmaReadyAt) {
		return false
	}
	c.dmaBusy = false
	c.counters.DMACompleted++
	return true
}

func (c *Controller) dueLocked(now time.Time) bool {
	return c.counters.FramesSubmitted == 0 || c.lastSubmit.IsZero() || now.Sub(c.lastSubmit) >= c.framePeriodLocked()
}

func (c *Controller) submitLocked(now time.Time, black bool) {
	if black {
		clear(c.display)
		c.displayGeneration++
		c.counters.Blackouts++
	}
	c.counters.FramesSubmitted++
	c.lastSubmit = now
	c.dmaBusy = true
	c.dmaReadyAt = now.Add(c.wireGuardLocked())
	// Firmware submit() clears blackLatched for every DMA submission. It is set
	// true only after a blackout DMA has fully completed.
	c.blackLatched = false
}

func (c *Controller) SetLink(link bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.link = link
}

func (c *Controller) Tick(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Main firmware loop calls receiver.expire(now) every iteration.
	c.expireLocked(now)

	ready := c.transferReadyLocked(now)

	// artnet_run::Policy::observe() runs only in RunningArtNet.
	action := byte(0) // 0 wait, 1 render, 2 blackout, 3 discard
	if c.state == stateRunningArtNet && c.policyEnabled {
		fresh := !c.lastComplete.IsZero() && now.Sub(c.lastComplete) <= c.opts.LossTimeout
		if !c.link || !fresh {
			wasPlaying := !c.policyWaiting
			c.policyWaiting = true
			if wasPlaying {
				action = 2
			} else if !c.link || c.publishedReady {
				action = 3
			}
		} else if c.publishedReady {
			c.policyWaiting = false
			action = 1
		}

		if action == 2 || action == 3 {
			c.clearReceiverLocked()
		}
		if action == 2 {
			// requestStop(RunningArtNet) enters the blackout handoff path while
			// preserving the ARTNET-ON user intent.
			c.state = stateWaitBlackout
		}
	}

	due := c.dueLocked(now)

	if c.state == stateWaitBlackout && ready && due {
		// controller::start() and signal-loss recovery both establish a physical
		// black frame before Art-Net owns the LEDs.
		c.submitLocked(now, true)
		c.state = stateWaitBlackoutCompletion
		return
	}

	if c.state == stateWaitBlackoutCompletion && ready {
		c.blackLatched = true
		if c.policyEnabled {
			// Firmware clears any frames that arrived during blackout ownership.
			c.clearReceiverLocked()
			c.state = stateRunningArtNet
		}
		return
	}

	if c.state == stateRunningArtNet && ready && due && action == 1 && c.publishedReady {
		copy(c.display, c.published)
		c.publishedReady = false
		c.displayGeneration = c.publishedGeneration
		c.submitLocked(now, false)
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
	state := "WAIT_BLACKOUT"
	switch c.state {
	case stateWaitBlackoutCompletion:
		state = "WAIT_BLACKOUT_COMPLETION"
	case stateRunningArtNet:
		state = "ARTNET_RUNNING"
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
		ArtNetWaiting: c.policyEnabled && c.policyWaiting,
		Link: c.link,
		DisplayGeneration: c.displayGeneration,
		PublishedGeneration: c.publishedGeneration,
		PublishedReady: c.publishedReady,
		Counters: c.counters,
		Timing: TimingStats{
			IngestAvgUS: ia, IngestP95US: ip95, IngestP99US: ip99, IngestMaxUS: imax,
			AssemblyAvgUS: aa, AssemblyP95US: ap95, AssemblyP99US: ap99, AssemblyMaxUS: amax,
		},
		Routes: append([]RouteInfo(nil), c.routeInfo...),
		Universes: universeStatus,
	}
}

func (c *Controller) FrameRGB(stage string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var frame []byte
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "", "display":
		frame = c.display
	case "published":
		frame = c.published
	case "candidate":
		frame = c.candidate
	default:
		return nil, fmt.Errorf("unknown frame stage %q", stage)
	}
	out := make([]byte, len(frame))
	for _, r := range c.routes {
		src := frame[r.frameOff:r.frameOff+r.byteCount]
		dst := out[r.frameOff:r.frameOff+r.byteCount]
		wireToRGB(dst, src, r.cfg.ColorOrder)
	}
	return out, nil
}

func (c *Controller) DisplayRGB() []byte {
	out, _ := c.FrameRGB("display")
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
