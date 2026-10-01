package probe

import "testing"

func TestParseColor(t *testing.T) {
	c, err := ParseColor("#123456")
	if err != nil {
		t.Fatal(err)
	}
	if c != (Color{R: 0x12, G: 0x34, B: 0x56}) {
		t.Fatalf("got %#v", c)
	}
}

func TestSetAndStrand(t *testing.T) {
	f := BlankFrame()
	if err := SetPixel(f, 64, Color{R: 1, G: 2, B: 3}); err != nil {
		t.Fatal(err)
	}
	i := 64 * 3
	if f[i] != 1 || f[i+1] != 2 || f[i+2] != 3 {
		t.Fatal("pixel 64 not set")
	}

	f = BlankFrame()
	if err := FillStrand(f, 2, Color{R: 9, G: 8, B: 7}); err != nil {
		t.Fatal(err)
	}
	for p := 128; p < 192; p++ {
		i := p * 3
		if f[i] != 9 || f[i+1] != 8 || f[i+2] != 7 {
			t.Fatalf("pixel %d not set", p)
		}
	}
}

func TestBuildPacket(t *testing.T) {
	f := BlankFrame()
	_ = SetPixel(f, 0, Color{R: 255})
	p, err := BuildPacket(0, f)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 4+1536 {
		t.Fatalf("len=%d want=%d", len(p), 4+1536)
	}
	if p[2] != 0x06 || p[3] != 0x00 {
		t.Fatalf("bad length header %02x %02x", p[2], p[3])
	}
}
