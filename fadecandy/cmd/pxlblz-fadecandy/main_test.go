package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"pxlblz-fadecandy/internal/frameinput"
	"pxlblz-fadecandy/internal/mapping"
	"pxlblz-fadecandy/internal/opc"
	"pxlblz-fadecandy/internal/wsinput"
)

// Exercise real masked WebSocket input and the production size limit, then
// verify the physical OPC frame across a large -> small -> exact transition.
func TestVariableFramesThroughWebSocketAndMapping(t *testing.T) {
	latest, _ := frameinput.New(512 * 3)
	received := make(chan error, 1)
	server, err := wsinput.New("127.0.0.1:0", "/pixels", maxInputFrameBytes, func(p []byte) { received <- latest.Submit(p) })
	if err != nil {
		t.Fatal(err)
	}
	addr, err := server.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprintf(conn, "GET /pixels HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n", addr)
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 101 {
		t.Fatalf("upgrade: %s", response.Status)
	}
	mapper, err := mapping.New(mapping.Spec{Dimensions: [3]int{8, 8, 8}, InputOrder: [3]mapping.Axis{mapping.X, mapping.Y, mapping.Z}, OutputOrder: [3]mapping.Axis{mapping.Y, mapping.X, mapping.Z}})
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{2048, 128, 512, 2197, 1} {
		input := make([]byte, count*3)
		for i := range input {
			input[i] = byte(i%251 + 1)
		}
		wire := []byte{0x82, 0x80 | 126, byte(len(input) >> 8), byte(len(input))}
		if len(input) < 126 {
			wire = []byte{0x82, 0x80 | byte(len(input))}
		}
		mask := [4]byte{7, 11, 13, 17}
		wire = append(wire, mask[:]...)
		for i, value := range input {
			wire = append(wire, value^mask[i%4])
		}
		if _, err := conn.Write(wire); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-received:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatalf("%d pixels not accepted", count)
		}
		logical, physical := make([]byte, 1536), make([]byte, 1536)
		if _, _, err := latest.ReadInto(logical); err != nil {
			t.Fatal(err)
		}
		if err := mapper.MapRGB(logical, physical); err != nil {
			t.Fatal(err)
		}
		packet, err := opc.EncodeSetPixelColors(0, physical)
		if err != nil {
			t.Fatal(err)
		}
		if len(packet) != 1540 || binary.BigEndian.Uint16(packet[2:4]) != 1536 {
			t.Fatalf("bad OPC length: %d", len(packet))
		}
		for index := 0; index < 512; index++ {
			x, y, z := index%8, (index/8)%8, index/64
			physicalIndex := z*64 + x*8 + y
			want := []byte{0, 0, 0}
			if index < count {
				want = input[index*3 : index*3+3]
			}
			got := packet[4+physicalIndex*3 : 4+physicalIndex*3+3]
			if !bytes.Equal(got, want) {
				t.Fatalf("count %d pixel %d: got %v want %v", count, index, got, want)
			}
		}
	}
}
