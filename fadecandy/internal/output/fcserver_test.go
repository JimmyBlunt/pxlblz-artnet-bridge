package output

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestSendRGBWritesOPCPacket(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	got := make(chan []byte, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		b := make([]byte, 4+6)
		_, err = io.ReadFull(c, b)
		if err == nil {
			got <- b
		}
	}()

	c := New(ln.Addr().String(), 3)
	defer c.Close()
	if err := c.SendRGB([]byte{255, 0, 0, 0, 255, 0}); err != nil {
		t.Fatal(err)
	}

	select {
	case p := <-got:
		want := []byte{3, 0, 0, 6, 255, 0, 0, 0, 255, 0}
		if len(p) != len(want) {
			t.Fatalf("len=%d want=%d", len(p), len(want))
		}
		for i := range want {
			if p[i] != want[i] {
				t.Fatalf("byte %d=%d want=%d", i, p[i], want[i])
			}
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for OPC packet")
	}
}
