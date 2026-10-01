package mapping

import "testing"

func l3dSpec() Spec {
	return Spec{
		Dimensions:  [3]int{8, 8, 8},
		InputOrder:  [3]Axis{X, Y, Z},
		OutputOrder: [3]Axis{Y, X, Z},
	}
}

func TestOriginalL3DMapping(t *testing.T) {
	m, err := New(l3dSpec())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ logical, physical int }{
		{0, 0},   // x0 y0 z0
		{1, 8},   // x1 y0 z0 -> z*64+x*8+y
		{8, 1},   // x0 y1 z0
		{63, 63}, // x7 y7 z0
		{64, 64}, // z1 starts at 64
		{511, 511},
	}
	for _, c := range cases {
		got, _ := m.PhysicalIndex(c.logical)
		if got != c.physical {
			t.Fatalf("logical %d -> %d want %d", c.logical, got, c.physical)
		}
	}
}

func TestFlipX(t *testing.T) {
	s := l3dSpec()
	s.Flip.X = true
	m, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := m.PhysicalIndex(0)
	if got != 56 {
		t.Fatalf("logical 0 -> %d want 56", got)
	}
}

func TestMapRGB(t *testing.T) {
	m, err := New(l3dSpec())
	if err != nil {
		t.Fatal(err)
	}
	src := make([]byte, 512*3)
	dst := make([]byte, 512*3)
	src[3] = 9 // logical pixel 1 red
	if err := m.MapRGB(src, dst); err != nil {
		t.Fatal(err)
	}
	if dst[8*3] != 9 {
		t.Fatalf("physical pixel 8 red=%d want 9", dst[8*3])
	}
}

func TestCustomLUT(t *testing.T) {
	s := Spec{Dimensions: [3]int{2, 1, 1}, InputOrder: [3]Axis{X, Y, Z}, OutputOrder: [3]Axis{X, Y, Z}, LUT: []int{1, 0}}
	m, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	src := []byte{10, 20, 30, 40, 50, 60}
	dst := make([]byte, 6)
	if err := m.MapRGB(src, dst); err != nil {
		t.Fatal(err)
	}
	want := []byte{40, 50, 60, 10, 20, 30}
	for i := range want {
		if dst[i] != want[i] {
			t.Fatalf("byte %d=%d want=%d", i, dst[i], want[i])
		}
	}
}
