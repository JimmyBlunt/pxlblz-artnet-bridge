package frameinput

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type LatestFrame struct {
	mu        sync.Mutex
	buf       []byte
	have      bool
	gen       uint64
	consumed  uint64
	submitted atomic.Uint64
	replaced  atomic.Uint64
	invalid   atomic.Uint64
	lastUnix  atomic.Int64
}

type Stats struct {
	Submitted, Replaced, Invalid uint64
	LastAt                       time.Time
}

func New(size int) (*LatestFrame, error) {
	if size <= 0 || size%3 != 0 {
		return nil, fmt.Errorf("frame size must be a positive multiple of 3 RGB bytes")
	}
	return &LatestFrame{buf: make([]byte, size)}, nil
}
func (l *LatestFrame) Submit(frame []byte) error {
	if len(frame) == 0 || len(frame)%3 != 0 {
		l.invalid.Add(1)
		return fmt.Errorf("frame length %d must contain complete RGB pixels", len(frame))
	}
	l.mu.Lock()
	if l.have && l.gen != l.consumed {
		l.replaced.Add(1)
	}
	// Keep the first output-sized set of logical pixels. Clear unused LEDs
	// on shorter frames so they do not retain colors from the previous frame.
	n := copy(l.buf, frame)
	clear(l.buf[n:])
	l.gen++
	l.have = true
	l.mu.Unlock()
	l.submitted.Add(1)
	l.lastUnix.Store(time.Now().UnixNano())
	return nil
}
func (l *LatestFrame) ReadInto(dst []byte) (bool, bool, error) {
	if len(dst) != len(l.buf) {
		return false, false, fmt.Errorf("destination length %d, expected %d", len(dst), len(l.buf))
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.have {
		return false, false, nil
	}
	copy(dst, l.buf)
	fresh := l.gen != l.consumed
	l.consumed = l.gen
	return true, fresh, nil
}
func (l *LatestFrame) Stats() Stats {
	n := l.lastUnix.Load()
	var t time.Time
	if n != 0 {
		t = time.Unix(0, n)
	}
	return Stats{Submitted: l.submitted.Load(), Replaced: l.replaced.Load(), Invalid: l.invalid.Load(), LastAt: t}
}
