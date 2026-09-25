package router

import (
	"net"
	"testing"
	"time"

	"pxlblz-router/internal/artnet"
	cfgpkg "pxlblz-router/internal/config"
)

type rxPkt struct {
	from string
	p    artnet.ParsedDmx
}

func listenAny(t *testing.T) (*net.UDPConn, int) {
	t.Helper()
	l, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	return l, l.LocalAddr().(*net.UDPAddr).Port
}

func drain(t *testing.T, l *net.UDPConn, n int) []rxPkt {
	t.Helper()
	var out []rxPkt
	l.SetReadDeadline(time.Now().Add(2 * time.Second))
	for len(out) < n {
		b := make([]byte, 1024)
		k, from, err := l.ReadFromUDP(b)
		if err != nil {
			t.Fatalf("after %d/%d packets: %v", len(out), n, err)
		}
		p, err := artnet.ParseDmx(b[:k])
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, rxPkt{from.IP.String(), p})
	}
	return out
}

// Hard receiver facts from the BACK_PANEL_249 bring-up must survive the
// controller refactor unchanged.
func TestBackPanelFullControllerFrameUnchanged(t *testing.T) {
	l, port := listenAny(t)
	defer l.Close()
	cfg, err := cfgpkg.Load("../../config/routes.backpanel-all.json")
	if err != nil {
		t.Fatal(err)
	}
	for i := range cfg.Routes {
		cfg.Routes[i].TargetIP = "127.0.0.1"
	}
	cfg.ArtNet.UDPPort = port
	r, err := New(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.SendFrame(make([]byte, cfg.Input.PixelCount*3)); err != nil {
		t.Fatal(err)
	}
	pk := drain(t, l, 29)
	got := map[uint16]int{}
	for _, x := range pk {
		got[x.p.Universe] = len(x.p.Data)
		if x.p.Sequence != 1 {
			t.Fatalf("U%d seq %d, want 1", x.p.Universe, x.p.Sequence)
		}
	}
	if _, bad := got[138]; bad {
		t.Fatal("U138 transmitted")
	}
	tails := map[uint16]int{121: 100, 126: 174, 132: 90, 137: 390, 141: 36, 145: 300, 149: 6}
	for u, want := range tails {
		if got[u] != want {
			t.Errorf("U%d len %d want %d", u, got[u], want)
		}
	}
	if len(got) != 29 {
		t.Fatalf("%d universes, want 29", len(got))
	}
}

func TestControllersOwnIndependentSequences(t *testing.T) {
	l, port := listenAny(t)
	defer l.Close()
	cfg := cfgpkg.Config{
		Version: 2,
		Input:   cfgpkg.InputConfig{PixelCount: 400, FPSTarget: 60},
		ArtNet:  cfgpkg.ArtNet{UDPPort: port},
		Controllers: []cfgpkg.Controller{
			{Name: "A", TargetIP: "127.0.0.1", FPSTarget: 60},
			{Name: "B", TargetIP: "127.0.0.2", FPSTarget: 30},
		},
		Routes: []cfgpkg.Route{
			{Name: "a1", Enabled: true, TargetIP: "127.0.0.1", PixelStart: 0, PixelCount: 200, UniverseStart: 0, ColorOrder: "RGB"},
			{Name: "b1", Enabled: true, TargetIP: "127.0.0.2", PixelStart: 200, PixelCount: 100, UniverseStart: 50, ColorOrder: "BGR"},
			{Name: "a2", Enabled: true, TargetIP: "127.0.0.1", PixelStart: 300, PixelCount: 100, UniverseStart: 5, ColorOrder: "RGB"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	r, err := New(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cs := r.Controllers()
	if len(cs) != 2 || cs[0].Name() != "A" || cs[1].Name() != "B" || cs[1].FPS() != 30 {
		t.Fatalf("grouping wrong")
	}
	if u := cs[0].ExpectedUniverses(); len(u) != 3 || u[0] != 0 || u[1] != 1 || u[2] != 5 {
		t.Fatalf("A universes %v", u)
	}
	frame := make([]byte, 400*3)
	frame[200*3] = 11 // R of first B pixel
	frame[200*3+2] = 33
	for i := 0; i < 3; i++ {
		if err := r.SendController(cs[0], frame); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.SendController(cs[1], frame); err != nil {
		t.Fatal(err)
	}
	pk := drain(t, l, 3*3+1)
	var lastA byte
	for _, x := range pk {
		// loopback source IPs are all 127.0.0.1, so identify B by its universe
		switch x.p.Universe {
		default:
			lastA = x.p.Sequence
		case 50:
			if x.p.Sequence != 1 {
				t.Fatalf("B seq %d, want 1 (independent of A)", x.p.Sequence)
			}
			if x.p.Data[0] != 33 || x.p.Data[2] != 11 {
				t.Fatalf("B BGR reorder wrong: % x", x.p.Data[:3])
			}
		}
	}
	if lastA != 3 {
		t.Fatalf("A last seq %d, want 3", lastA)
	}
	if st := r.Stats(); st.Frames != 3 || st.Packets != 10 {
		t.Fatalf("aggregate stats %+v", st)
	}
}

func TestDisabledControllerIsSkipped(t *testing.T) {
	off := false
	cfg := cfgpkg.Config{
		Version: 2,
		Input:   cfgpkg.InputConfig{PixelCount: 20, FPSTarget: 30},
		ArtNet:  cfgpkg.ArtNet{UDPPort: 6454},
		Controllers: []cfgpkg.Controller{
			{Name: "A", TargetIP: "10.0.0.1"},
			{Name: "B", TargetIP: "10.0.0.2", Enabled: &off},
		},
		Routes: []cfgpkg.Route{
			{Name: "a", Enabled: true, TargetIP: "10.0.0.1", PixelStart: 0, PixelCount: 10, ColorOrder: "RGB"},
			{Name: "b", Enabled: true, TargetIP: "10.0.0.2", PixelStart: 10, PixelCount: 10, ColorOrder: "RGB"},
		},
	}
	r, err := New(cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Controllers()) != 1 || r.Controllers()[0].Name() != "A" {
		t.Fatal("disabled controller should not be planned")
	}
}

func TestSequenceWrapSkipsZero(t *testing.T) {
	cfg := cfgpkg.Config{Version: 1, Input: cfgpkg.InputConfig{PixelCount: 1, FPSTarget: 30}, ArtNet: cfgpkg.ArtNet{UDPPort: 6454},
		Routes: []cfgpkg.Route{{Name: "a", Enabled: true, TargetIP: "10.0.0.1", PixelCount: 1, ColorOrder: "RGB"}}}
	r, _ := New(cfg, true)
	c := r.Controllers()[0]
	for i := 0; i < 255; i++ {
		_ = r.SendController(c, make([]byte, 3))
	}
	if c.seq != 1 {
		t.Fatalf("after 255 frames seq=%d, want wrap to 1", c.seq)
	}
}
