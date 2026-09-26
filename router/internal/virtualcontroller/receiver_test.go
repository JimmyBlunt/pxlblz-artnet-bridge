package virtualcontroller

import (
	"testing"
	"time"

	"pxlblz-router/internal/artnet"
	cfgpkg "pxlblz-router/internal/config"
)

func backPanelConfig() cfgpkg.Config {
	return cfgpkg.Config{
		Version: 1,
		Input: cfgpkg.InputConfig{PixelCount: 8186, FPSTarget: 30},
		ArtNet: cfgpkg.ArtNet{UDPPort: 6454, Unicast: true},
		Routes: []cfgpkg.Route{
			{Name:"P1",Enabled:true,TargetIP:"10.0.0.253",PhysicalPort:1,PixelStart:1440,PixelCount:203,UniverseStart:120,ColorOrder:"RGB"},
			{Name:"P2",Enabled:true,TargetIP:"10.0.0.253",PhysicalPort:2,PixelStart:3744,PixelCount:738,UniverseStart:122,ColorOrder:"RGB"},
			{Name:"P3",Enabled:true,TargetIP:"10.0.0.253",PhysicalPort:3,PixelStart:4482,PixelCount:880,UniverseStart:127,ColorOrder:"RGB"},
			{Name:"P4",Enabled:true,TargetIP:"10.0.0.253",PhysicalPort:4,PixelStart:5362,PixelCount:810,UniverseStart:133,ColorOrder:"RGB"},
			{Name:"P5",Enabled:true,TargetIP:"10.0.0.253",PhysicalPort:5,PixelStart:6172,PixelCount:352,UniverseStart:139,ColorOrder:"RGB"},
			{Name:"P6",Enabled:true,TargetIP:"10.0.0.253",PhysicalPort:6,PixelStart:6524,PixelCount:610,UniverseStart:142,ColorOrder:"RGB"},
			{Name:"P7",Enabled:true,TargetIP:"10.0.0.253",PhysicalPort:7,PixelStart:7134,PixelCount:512,UniverseStart:146,ColorOrder:"RGB"},
		},
	}
}

func wireDataBytes(u uint16) int {
	switch u {
	case 121: return 99
	case 126: return 174
	case 132: return 90
	case 137: return 390
	case 141: return 36
	case 145: return 300
	case 149: return 6
	default: return 510
	}
}

func dmxPacket(t *testing.T, u uint16, seq byte, dataBytes int) []byte {
	t.Helper()
	data := make([]byte, dataBytes)
	for i := range data { data[i] = byte(int(u)+i) }
	buf := make([]byte, artnet.HeaderSize+artnet.MaxDmxPayload)
	p, err := artnet.BuildDmx(buf, u, seq, data)
	if err != nil { t.Fatal(err) }
	return append([]byte(nil), p...)
}

func sendComplete(t *testing.T, c *Controller, seq byte, start time.Time) time.Time {
	t.Helper()
	now := start
	for _, u := range c.ExpectedUniverses() {
		c.IngestPacket(dmxPacket(t, u, seq, wireDataBytes(u)), now)
		now = now.Add(100 * time.Microsecond)
	}
	return now
}

func TestExactBackPanelCompleteFrame(t *testing.T) {
	c, err := New(backPanelConfig(), "10.0.0.253", Options{})
	if err != nil { t.Fatal(err) }
	now := sendComplete(t, c, 7, time.Unix(1,0))
	s := c.Snapshot(now)
	if s.ExpectedUniverses != 29 || s.Counters.Complete != 1 || s.Counters.Accepted != 29 {
		t.Fatalf("snapshot=%+v", s)
	}
	if s.Counters.Rejected != 0 || s.Counters.Ignored != 0 || s.Counters.Incomplete != 0 {
		t.Fatalf("unexpected counters=%+v", s.Counters)
	}
}

func TestShortU145RejectsAndCandidateExpires(t *testing.T) {
	c, _ := New(backPanelConfig(), "10.0.0.253", Options{})
	start := time.Unix(2,0)
	now := start
	for _, u := range c.ExpectedUniverses() {
		n := wireDataBytes(u)
		if u == 145 { n = 78 }
		c.IngestPacket(dmxPacket(t, u, 8, n), now)
		now = now.Add(100 * time.Microsecond)
	}
	s := c.Snapshot(now)
	if s.Counters.Rejected != 1 || s.Counters.Complete != 0 {
		t.Fatalf("before expiry=%+v", s.Counters)
	}
	c.Tick(start.Add(101 * time.Millisecond))
	s = c.Snapshot(start.Add(101*time.Millisecond))
	if s.Counters.Incomplete != 1 {
		t.Fatalf("incomplete=%d want 1", s.Counters.Incomplete)
	}
}

func TestMissingU149ExpiresIncomplete(t *testing.T) {
	c, _ := New(backPanelConfig(), "10.0.0.253", Options{})
	start := time.Unix(3,0)
	now := start
	for _, u := range c.ExpectedUniverses() {
		if u == 149 { continue }
		c.IngestPacket(dmxPacket(t,u,9,wireDataBytes(u)), now)
		now = now.Add(100*time.Microsecond)
	}
	if c.Snapshot(now).Counters.Complete != 0 { t.Fatal("unexpected complete frame") }
	c.Tick(start.Add(101*time.Millisecond))
	if got := c.Snapshot(start.Add(101*time.Millisecond)).Counters.Incomplete; got != 1 {
		t.Fatalf("incomplete=%d want 1", got)
	}
}

func TestPerPacketSequenceNeverCompletes(t *testing.T) {
	c, _ := New(backPanelConfig(), "10.0.0.253", Options{})
	now := time.Unix(4,0)
	seq := byte(1)
	for _, u := range c.ExpectedUniverses() {
		c.IngestPacket(dmxPacket(t,u,seq,wireDataBytes(u)), now)
		now = now.Add(100*time.Microsecond)
		seq++
	}
	s := c.Snapshot(now)
	if s.Counters.Complete != 0 || s.Counters.Incomplete == 0 {
		t.Fatalf("counters=%+v", s.Counters)
	}
}

func TestExtraUniversesIgnoredButComplete(t *testing.T) {
	c, _ := New(backPanelConfig(), "10.0.0.253", Options{})
	start := time.Unix(5,0)
	now := start
	for _, u := range []uint16{138,150,151,152} {
		c.IngestPacket(dmxPacket(t,u,10,510), now)
		now = now.Add(100*time.Microsecond)
	}
	now = sendComplete(t,c,10,now)
	s := c.Snapshot(now)
	if s.Counters.Ignored != 4 || s.Counters.Complete != 1 {
		t.Fatalf("counters=%+v", s.Counters)
	}
}

func TestSequenceZeroCompletes(t *testing.T) {
	c, _ := New(backPanelConfig(), "10.0.0.253", Options{})
	now := sendComplete(t,c,0,time.Unix(6,0))
	s := c.Snapshot(now)
	if s.Counters.Complete != 1 || s.SequenceMode != "sequence-zero" {
		t.Fatalf("snapshot=%+v", s)
	}
}

func TestSequenceZeroDuplicateRestartsCandidate(t *testing.T) {
	c, _ := New(backPanelConfig(), "10.0.0.253", Options{})
	now := time.Unix(7,0)
	c.IngestPacket(dmxPacket(t,120,0,510), now)
	c.IngestPacket(dmxPacket(t,120,0,510), now.Add(time.Millisecond))
	s := c.Snapshot(now.Add(time.Millisecond))
	if s.Counters.Duplicates != 1 || s.Counters.Incomplete != 1 || s.CandidateReceived != 1 {
		t.Fatalf("snapshot=%+v", s)
	}
}

func TestSequenceWrap255To1Accepted(t *testing.T) {
	c, _ := New(backPanelConfig(), "10.0.0.253", Options{})
	now := sendComplete(t,c,255,time.Unix(8,0))
	now = sendComplete(t,c,1,now.Add(time.Millisecond))
	s := c.Snapshot(now)
	if s.Counters.Complete != 2 || s.Counters.Stale != 0 {
		t.Fatalf("counters=%+v", s.Counters)
	}
}

func TestStaleSequenceIgnored(t *testing.T) {
	c, _ := New(backPanelConfig(), "10.0.0.253", Options{})
	now := time.Unix(9,0)
	c.IngestPacket(dmxPacket(t,120,200,510), now)
	c.IngestPacket(dmxPacket(t,122,100,510), now.Add(time.Millisecond))
	s := c.Snapshot(now.Add(time.Millisecond))
	if s.Counters.Stale != 1 || s.CandidateReceived != 1 {
		t.Fatalf("snapshot=%+v", s)
	}
}

func TestRunPolicyDMAAndBlackout(t *testing.T) {
	c, _ := New(backPanelConfig(), "10.0.0.253", Options{})
	start := time.Unix(10,0)
	now := sendComplete(t,c,20,start)
	c.Tick(now)
	s := c.Snapshot(now)
	if s.Counters.FramesSubmitted != 1 || !s.DMABusy || s.WireGuardUS != 26700 {
		t.Fatalf("after render=%+v", s)
	}
	c.Tick(now.Add(26699*time.Microsecond))
	if c.Snapshot(now.Add(26699*time.Microsecond)).Counters.DMACompleted != 0 {
		t.Fatal("DMA completed before wire guard")
	}
	c.Tick(now.Add(26700*time.Microsecond))
	if c.Snapshot(now.Add(26700*time.Microsecond)).Counters.DMACompleted != 1 {
		t.Fatal("DMA completion not observed at guard")
	}
	c.Tick(start.Add(1100*time.Millisecond))
	s = c.Snapshot(start.Add(1100*time.Millisecond))
	if s.Counters.Blackouts != 1 || !s.BlackLatched {
		t.Fatalf("blackout=%+v", s)
	}
}

func TestSequenceStateResetsAfterOneSecondWithoutAcceptedPacket(t *testing.T) {
	c, _ := New(backPanelConfig(), "10.0.0.253", Options{})
	start := time.Unix(11,0)
	c.IngestPacket(dmxPacket(t,120,77,510), start)
	c.Tick(start.Add(1001*time.Millisecond))
	s := c.Snapshot(start.Add(1001*time.Millisecond))
	if s.SequenceMode != "unknown" || s.Sequence != -1 {
		t.Fatalf("sequence state not reset: %+v", s)
	}
}
