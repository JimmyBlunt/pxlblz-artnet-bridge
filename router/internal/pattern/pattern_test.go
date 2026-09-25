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

func TestPanelWalkCrossesRouteBoundaryInConfigOrder(t *testing.T) {
	const pixels = 400
	frame := make([]byte, pixels*3)
	routes := []cfgpkg.Route{
		// deliberately out of logical order: walk must follow config order
		{Name: "P6", Enabled: true, PhysicalPort: 6, PixelStart: 200, PixelCount: 50},
		{Name: "P7", Enabled: true, PhysicalPort: 7, PixelStart: 10, PixelCount: 40},
		{Name: "off", Enabled: false, PhysicalPort: 1, PixelStart: 300, PixelCount: 50},
	}
	isHead := func(p int) bool {
		r, g, b := pixel(frame, p)
		return r == 160 && g == 160 && b == 160
	}
	// frame 49 with speed 1: head on last pixel of P6
	if err := FillPanelWalk(frame, pixels, routes, 49, 1); err != nil {
		t.Fatal(err)
	}
	if !isHead(249) {
		t.Fatal("head should be on last pixel of P6")
	}
	// frame 50: head hands over to the first pixel of P7
	if err := FillPanelWalk(frame, pixels, routes, 50, 1); err != nil {
		t.Fatal(err)
	}
	if !isHead(10) {
		t.Fatal("head should be on first pixel of P7 after P6")
	}
	// tail of P6 still visible in yellow behind the head
	r, g, b := pixel(frame, 249)
	if r == 0 || g == 0 || b != 0 {
		t.Fatalf("P6 tail pixel should be yellow, got %d,%d,%d", r, g, b)
	}
	// disabled route stays dark
	for p := 300; p < 350; p++ {
		if r, g, b := pixel(frame, p); r|g|b != 0 {
			t.Fatalf("disabled route pixel %d lit", p)
		}
	}
	// wrap-around
	if err := FillPanelWalk(frame, pixels, routes, 90, 1); err != nil {
		t.Fatal(err)
	}
	if !isHead(200) {
		t.Fatal("head should wrap to first pixel of first route")
	}
}
