package wsmini

import (
	"bytes"
	"encoding/binary"
	"sync"
	"testing"
	"time"
)

func TestBinaryRoundTrip(t *testing.T) {
	var mu sync.Mutex
	var got []byte
	done := make(chan struct{}, 1)
	s, err := NewServer("127.0.0.1:0", "/pixels", 1024, func(p []byte) {
		mu.Lock()
		got = append([]byte(nil), p...)
		mu.Unlock()
		select {
		case done <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	addr, err := s.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c, err := Dial(addr, "/pixels")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	want := []byte{1, 2, 3, 4, 5}
	if err := c.SendBinary(want); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
	mu.Lock()
	defer mu.Unlock()
	if !bytes.Equal(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
	if s.Stats().Binary != 1 {
		t.Fatalf("stats=%+v", s.Stats())
	}
}

func TestReadFrameIntoReusesPayloadBuffer(t *testing.T) {
	first := []byte{1, 2, 3, 4, 5, 6}
	second := []byte{6, 5, 4, 3, 2, 1}
	var wire bytes.Buffer
	if err := writeFrame(&wire, 0x2, first, true); err != nil {
		t.Fatal(err)
	}
	if err := writeFrame(&wire, 0x2, second, true); err != nil {
		t.Fatal(err)
	}

	op1, p1, scratch, err := readFrameInto(&wire, true, 1024, nil)
	if err != nil {
		t.Fatal(err)
	}
	if op1 != 0x2 || !bytes.Equal(p1, first) {
		t.Fatalf("first frame opcode=%d payload=%v", op1, p1)
	}
	if len(p1) == 0 {
		t.Fatal("first payload unexpectedly empty")
	}
	firstPtr := &p1[0]

	op2, p2, scratch2, err := readFrameInto(&wire, true, 1024, scratch)
	if err != nil {
		t.Fatal(err)
	}
	if op2 != 0x2 || !bytes.Equal(p2, second) {
		t.Fatalf("second frame opcode=%d payload=%v", op2, p2)
	}
	if firstPtr != &p2[0] {
		t.Fatal("same-sized second frame did not reuse payload backing buffer")
	}
	if cap(scratch2) != cap(scratch) {
		t.Fatalf("scratch capacity changed %d -> %d", cap(scratch), cap(scratch2))
	}
}

func BenchmarkReadFrameIntoReuse98K(b *testing.B) {
	payload := make([]byte, 32768*3)
	for i := range payload {
		payload[i] = byte(i)
	}
	wire := maskedFrameForBenchmark(payload)
	reader := bytes.NewReader(wire)
	scratch := make([]byte, 0, len(payload))

	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset(wire)
		_, got, next, err := readFrameInto(reader, true, len(payload), scratch)
		if err != nil {
			b.Fatal(err)
		}
		scratch = next
		if len(got) != len(payload) {
			b.Fatalf("got payload=%d want=%d", len(got), len(payload))
		}
	}
}

func maskedFrameForBenchmark(payload []byte) []byte {
	const opcode = byte(0x2)
	mask := [4]byte{0x12, 0x34, 0x56, 0x78}
	n := len(payload)
	header := make([]byte, 0, 14)
	header = append(header, 0x80|opcode)
	switch {
	case n <= 125:
		header = append(header, 0x80|byte(n))
	case n <= 65535:
		header = append(header, 0x80|126, byte(n>>8), byte(n))
	default:
		header = append(header, 0x80|127)
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(n))
		header = append(header, ext[:]...)
	}
	header = append(header, mask[:]...)
	out := make([]byte, len(header)+n)
	copy(out, header)
	for i := range payload {
		out[len(header)+i] = payload[i] ^ mask[i&3]
	}
	return out
}
