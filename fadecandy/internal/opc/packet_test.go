package opc

import "testing"

func TestEncodeSetPixelColors(t *testing.T) {
	p, err := EncodeSetPixelColors(7, []byte{255, 0, 0, 0, 255, 0})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{7, 0, 0, 6, 255, 0, 0, 0, 255, 0}
	if len(p) != len(want) {
		t.Fatalf("len=%d want=%d", len(p), len(want))
	}
	for i := range want {
		if p[i] != want[i] {
			t.Fatalf("byte %d=%d want=%d", i, p[i], want[i])
		}
	}
}

func TestEncodeRejectsNonRGB(t *testing.T) {
	if _, err := EncodeSetPixelColors(0, []byte{1, 2}); err == nil {
		t.Fatal("expected error")
	}
}
