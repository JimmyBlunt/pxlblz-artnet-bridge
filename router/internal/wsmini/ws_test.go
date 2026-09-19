package wsmini

import (
	"bytes"
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
