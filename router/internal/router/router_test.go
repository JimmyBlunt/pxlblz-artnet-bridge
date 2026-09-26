package router

import (
	"net"
	"testing"
	"time"

	"pxlblz-router/internal/artnet"
	cfgpkg "pxlblz-router/internal/config"
)

func TestUniverseSplit256RGB(t *testing.T) {
	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.LocalAddr().(*net.UDPAddr).Port

	cfg := cfgpkg.Config{
		Version: 1,
		Input:   cfgpkg.InputConfig{PixelCount: 256, FPSTarget: 60},
		ArtNet:  cfgpkg.ArtNet{UDPPort: port, Unicast: true},
		Routes:  []cfgpkg.Route{{Name: "test", Enabled: true, TargetIP: "127.0.0.1", PixelStart: 0, PixelCount: 256, UniverseStart: 7, ColorOrder: "RGB"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	r, err := New(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	frame := make([]byte, 256*3)
	for i := range frame {
		frame[i] = byte(i)
	}
	if err := r.SendFrame(frame); err != nil {
		t.Fatal(err)
	}

	got := make([][]byte, 0, 2)
	listener.SetReadDeadline(time.Now().Add(time.Second))
	for len(got) < 2 {
		b := make([]byte, 1024)
		n, _, err := listener.ReadFromUDP(b)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, b[:n])
	}

	a, err := artnet.ParseDmx(got[0])
	if err != nil {
		t.Fatal(err)
	}
	b, err := artnet.ParseDmx(got[1])
	if err != nil {
		t.Fatal(err)
	}
	if a.Universe != 7 || len(a.Data) != 510 {
		t.Fatalf("first U/len = %d/%d", a.Universe, len(a.Data))
	}
	if b.Universe != 8 || len(b.Data) != 258 {
		t.Fatalf("second U/len = %d/%d", b.Universe, len(b.Data))
	}
	if a.Sequence != b.Sequence {
		t.Fatalf("sequence differs %d != %d", a.Sequence, b.Sequence)
	}
}

func TestColorOrders(t *testing.T) {
	src := []byte{1, 2, 3, 4, 5, 6}
	tests := map[string][]byte{
		"RGB": {1, 2, 3, 4, 5, 6},
		"RBG": {1, 3, 2, 4, 6, 5},
		"GRB": {2, 1, 3, 5, 4, 6},
		"GBR": {2, 3, 1, 5, 6, 4},
		"BRG": {3, 1, 2, 6, 4, 5},
		"BGR": {3, 2, 1, 6, 5, 4},
	}
	for order, want := range tests {
		t.Run(order, func(t *testing.T) {
			dst := make([]byte, len(src))
			reorder(dst, src, order)
			for i := range want {
				if dst[i] != want[i] {
					t.Fatalf("order=%s dst=%v want=%v", order, dst, want)
				}
			}
		})
	}
}

func TestBackPanelPort1PadsU121To100Bytes(t *testing.T) {
	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.LocalAddr().(*net.UDPAddr).Port

	cfg := cfgpkg.Config{
		Version: 1,
		Input:   cfgpkg.InputConfig{PixelCount: 203, FPSTarget: 30},
		ArtNet:  cfgpkg.ArtNet{UDPPort: port, Unicast: true},
		Routes:  []cfgpkg.Route{{Name: "OUT1", Enabled: true, TargetIP: "127.0.0.1", PhysicalPort: 1, PixelStart: 0, PixelCount: 203, UniverseStart: 120, ColorOrder: "RGB"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	r, err := New(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	frame := make([]byte, 203*3)
	for i := range frame {
		frame[i] = byte(i)
	}
	if err := r.SendFrame(frame); err != nil {
		t.Fatal(err)
	}

	listener.SetReadDeadline(time.Now().Add(time.Second))
	packets := make(map[uint16]artnet.ParsedDmx)
	for len(packets) < 2 {
		b := make([]byte, 1024)
		n, _, err := listener.ReadFromUDP(b)
		if err != nil {
			t.Fatal(err)
		}
		p, err := artnet.ParseDmx(b[:n])
		if err != nil {
			t.Fatal(err)
		}
		packets[p.Universe] = p
	}
	if got := len(packets[120].Data); got != 510 {
		t.Fatalf("U120 len=%d want 510", got)
	}
	if got := len(packets[121].Data); got != 100 {
		t.Fatalf("U121 len=%d want 100", got)
	}
	if packets[120].Sequence != packets[121].Sequence {
		t.Fatalf("sequence differs: U120=%d U121=%d", packets[120].Sequence, packets[121].Sequence)
	}
	if packets[121].Data[99] != 0 {
		t.Fatalf("U121 padding=%d want 0", packets[121].Data[99])
	}
}
