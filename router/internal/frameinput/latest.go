package frameinput

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type LatestFrame struct {
	mu       sync.Mutex
	buf      []byte
	have     bool
	gen      uint64
	consumed uint64

	submitted atomic.Uint64
	replaced  atomic.Uint64
	invalid   atomic.Uint64
	lastUnix  atomic.Int64
}

type Stats struct {
	Submitted uint64
	Replaced  uint64
	Invalid   uint64
	LastAt    time.Time
}

func NewLatestFrame(size int) (*LatestFrame, error) {
	if size <= 0 {
		return nil, fmt.Errorf("frame size must be > 0")
	}
	return &LatestFrame{buf: make([]byte, size)}, nil
}

func (l *LatestFrame) Size() int { return len(l.buf) }

func (l *LatestFrame) Reject() { l.invalid.Add(1) }

func (l *LatestFrame) Submit(frame []byte) error {
	if len(frame) != len(l.buf) {
		l.invalid.Add(1)
		return fmt.Errorf("frame length %d, expected %d", len(frame), len(l.buf))
	}
	l.mu.Lock()
	if l.have && l.gen != l.consumed {
		l.replaced.Add(1)
	}
	copy(l.buf, frame)
	l.gen++
	l.have = true
	l.mu.Unlock()

	l.submitted.Add(1)
	l.lastUnix.Store(time.Now().UnixNano())
	return nil
}

func (l *LatestFrame) ReadInto(dst []byte) (have bool, fresh bool, generation uint64, err error) {
	if len(dst) != len(l.buf) {
		return false, false, 0, fmt.Errorf("destination length %d, expected %d", len(dst), len(l.buf))
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.have {
		return false, false, 0, nil
	}
	copy(dst, l.buf)
	fresh = l.gen != l.consumed
	generation = l.gen
	l.consumed = l.gen
	return true, fresh, generation, nil
}

func (l *LatestFrame) Stats() Stats {
	n := l.lastUnix.Load()
	var t time.Time
	if n != 0 {
		t = time.Unix(0, n)
	}
	return Stats{
		Submitted: l.submitted.Load(),
		Replaced:  l.replaced.Load(),
		Invalid:   l.invalid.Load(),
		LastAt:    t,
	}
}

// Snapshot copies the newest frame into dst for one of several independent
// consumers (one per controller). It does not block the producer beyond a
// memcpy. lastGen is the generation this consumer saw previously; fresh is
// true when a newer frame is available. Frames that were replaced before ANY
// consumer observed them are counted in Stats().Replaced.
func (l *LatestFrame) Snapshot(dst []byte, lastGen uint64) (have bool, fresh bool, generation uint64, at time.Time, err error) {
	if len(dst) != len(l.buf) {
		return false, false, 0, time.Time{}, fmt.Errorf("destination length %d, expected %d", len(dst), len(l.buf))
	}
	l.mu.Lock()
	if !l.have {
		l.mu.Unlock()
		return false, false, 0, time.Time{}, nil
	}
	generation = l.gen
	fresh = generation != lastGen
	if fresh {
		copy(dst, l.buf)
	}
	l.consumed = l.gen
	l.mu.Unlock()
	if n := l.lastUnix.Load(); n != 0 {
		at = time.Unix(0, n)
	}
	return true, fresh, generation, at, nil
}
