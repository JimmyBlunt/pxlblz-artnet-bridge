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
