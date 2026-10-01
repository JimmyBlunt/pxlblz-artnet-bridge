package frameinput

import (
	"bytes"
	"testing"
)

func TestVariableRGBFrames(t *testing.T) {
	latest, err := New(512 * 3)
	if err != nil {
		t.Fatal(err)
	}
	dst := make([]byte, 512*3)
	for _, count := range []int{512, 2048, 2197, 1, 128, 512} {
		frame := make([]byte, count*3)
		for i := range frame {
			frame[i] = byte(i%251 + 1)
		}
		if err := latest.Submit(frame); err != nil {
			t.Fatal(err)
		}
		have, fresh, err := latest.ReadInto(dst)
		if err != nil || !have || !fresh {
			t.Fatalf("count %d: have=%v fresh=%v err=%v", count, have, fresh, err)
		}
		want := make([]byte, len(dst))
		copy(want, frame)
		if !bytes.Equal(dst, want) {
			t.Fatalf("count %d: wrong crop or stale tail", count)
		}
		_, fresh, err = latest.ReadInto(dst)
		if err != nil || fresh {
			t.Fatalf("second read: fresh=%v err=%v", fresh, err)
		}
	}
	if st := latest.Stats(); st.Submitted != 6 || st.Invalid != 0 {
		t.Fatalf("stats: %+v", st)
	}
}

func TestInvalidFramePreservesLastValidFrame(t *testing.T) {
	latest, _ := New(6)
	valid := []byte{1, 2, 3, 4, 5, 6}
	if err := latest.Submit(valid); err != nil {
		t.Fatal(err)
	}
	for _, frame := range [][]byte{nil, {1}, {1, 2}, {1, 2, 3, 4}} {
		if err := latest.Submit(frame); err == nil {
			t.Fatalf("accepted malformed frame %v", frame)
		}
	}
	dst := make([]byte, 6)
	if _, _, err := latest.ReadInto(dst); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dst, valid) {
		t.Fatalf("invalid frame changed output: %v", dst)
	}
	if st := latest.Stats(); st.Invalid != 4 || st.Submitted != 1 {
		t.Fatalf("stats: %+v", st)
	}
}

func TestOutputSizeMustContainWholePixels(t *testing.T) {
	for _, size := range []int{-1, 0, 1, 4} {
		if _, err := New(size); err == nil {
			t.Fatalf("accepted size %d", size)
		}
	}
}
