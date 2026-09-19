package pattern

import "testing"

func TestAxes(t *testing.T) {
	f := make([]byte, 512*3)
	if err := Fill(f, "axes", 0); err != nil {
		t.Fatal(err)
	}
	p := func(x, y, z int) [3]byte { i := logicalIndex(x, y, z) * 3; return [3]byte{f[i], f[i+1], f[i+2]} }
	if got := p(7, 0, 0); got != [3]byte{255, 0, 0} {
		t.Fatalf("x axis=%v", got)
	}
	if got := p(0, 7, 0); got != [3]byte{0, 255, 0} {
		t.Fatalf("y axis=%v", got)
	}
	if got := p(0, 0, 7); got != [3]byte{0, 0, 255} {
		t.Fatalf("z axis=%v", got)
	}
}
