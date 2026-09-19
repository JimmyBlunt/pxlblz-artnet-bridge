package pattern

import (
	"testing"

	cfgpkg "pxlblz-router/internal/config"
)

func TestPortIDTouchesOnlyDiagnosticPixels(t *testing.T) {
	const pixels = 300
	frame := make([]byte, pixels*3)
	routes := []cfgpkg.Route{
		{Name: "P1", Enabled: true, PhysicalPort: 1, PixelStart: 10, PixelCount: 100},
		{Name: "P2", Enabled: true, PhysicalPort: 2, PixelStart: 150, PixelCount: 80},
	}
	if err := FillPortID(frame, pixels, routes, 0); err != nil {
		t.Fatal(err)
	}
	for p := 10; p < 18; p++ {
		r, g, b := pixel(frame, p)
		if g != 0 || b != 0 || r == 0 {
			t.Fatalf("P1 id pixel %d = %d,%d,%d", p, r, g, b)
		}
	}
	rID, _, _ := pixel(frame, 10)
	rMarker, _, _ := pixel(frame, 18)
	if rMarker <= rID {
		t.Fatalf("P1 marker=%d should be brighter than id=%d", rMarker, rID)
	}
	r, g, b := pixel(frame, 150)
	if r != 0 || g == 0 || b != 0 {
		t.Fatalf("P2 first pixel = %d,%d,%d", r, g, b)
	}
	for _, p := range []int{0, 9, 74, 100, 149, 214, 250, 299} {
		r, g, b := pixel(frame, p)
		if r != 0 || g != 0 || b != 0 {
			t.Fatalf("pixel %d unexpectedly lit: %d,%d,%d", p, r, g, b)
		}
	}
}

func TestPortIDMarkerMovesWithinFirst64Pixels(t *testing.T) {
	frame := make([]byte, 100*3)
	routes := []cfgpkg.Route{{Name: "P7", Enabled: true, PhysicalPort: 7, PixelStart: 0, PixelCount: 100}}
	if err := FillPortID(frame, 100, routes, 20); err != nil {
		t.Fatal(err)
	}
	r, g, b := pixel(frame, 18)
	if r != 192 || g != 192 || b != 192 {
		t.Fatalf("marker at 18 = %d,%d,%d want 192,192,192", r, g, b)
	}
	for p := 64; p < 100; p++ {
		r, g, b := pixel(frame, p)
		if r != 0 || g != 0 || b != 0 {
			t.Fatalf("pixel %d beyond diagnostic span lit: %d,%d,%d", p, r, g, b)
		}
	}
}

func pixel(frame []byte, p int) (byte, byte, byte) {
	i := p * 3
	return frame[i], frame[i+1], frame[i+2]
}
