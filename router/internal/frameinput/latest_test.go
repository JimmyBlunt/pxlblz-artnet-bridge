package frameinput

import "testing"

func TestLatestFrameReplacesUnconsumedFrames(t *testing.T) {
	l, err := NewLatestFrame(3)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Submit([]byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	if err := l.Submit([]byte{4, 5, 6}); err != nil {
		t.Fatal(err)
	}
	st := l.Stats()
	if st.Submitted != 2 || st.Replaced != 1 {
		t.Fatalf("stats = %+v", st)
	}
	dst := make([]byte, 3)
	have, fresh, gen, err := l.ReadInto(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !have || !fresh || gen != 2 {
		t.Fatalf("have=%v fresh=%v gen=%d", have, fresh, gen)
	}
	if string(dst) != string([]byte{4, 5, 6}) {
		t.Fatalf("dst=%v", dst)
	}
	_, fresh, _, _ = l.ReadInto(dst)
	if fresh {
		t.Fatal("second read should not be fresh")
	}
}

func TestLatestFrameRejectsWrongSize(t *testing.T) {
	l, _ := NewLatestFrame(3)
	if err := l.Submit([]byte{1, 2}); err == nil {
		t.Fatal("expected error")
	}
	if l.Stats().Invalid != 1 {
		t.Fatalf("invalid=%d", l.Stats().Invalid)
	}
}

func TestVariableSizeTruncatesPadsAndRejectsPartialPixels(t *testing.T) {
	l, err := NewLatestFrame(6) // 2 pixels
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Submit([]byte{1, 2, 3}); err == nil {
		t.Fatal("strict mode must reject a short frame")
	}
	l.AcceptVariableSize()

	dst := make([]byte, 6)
	// longer than the output: only the first two pixels are used
	if err := l.Submit([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := l.ReadInto(dst); err != nil || string(dst) != string([]byte{1, 2, 3, 4, 5, 6}) {
		t.Fatalf("long frame: %v %v", dst, err)
	}
	// shorter: the missing pixel must be black, not left over from before
	if err := l.Submit([]byte{9, 9, 9}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := l.ReadInto(dst); err != nil || string(dst) != string([]byte{9, 9, 9, 0, 0, 0}) {
		t.Fatalf("short frame: %v %v", dst, err)
	}
	// partial pixels and empty frames stay invalid
	for _, bad := range [][]byte{{1, 2}, {}} {
		if err := l.Submit(bad); err == nil {
			t.Fatalf("frame of %d bytes accepted", len(bad))
		}
	}
	if s := l.Stats(); s.Invalid != 3 || s.Submitted != 2 {
		t.Fatalf("stats %+v", s)
	}
}
