package opc

import (
	"encoding/binary"
	"fmt"
)

const (
	CommandSetPixelColors byte = 0x00
	HeaderSize                 = 4
)

// EncodeSetPixelColors builds a standard TCP Open Pixel Control packet.
// The payload must be RGB bytes (R,G,B repeated). Fadecandy's default fcserver
// accepts this on TCP port 7890.
func EncodeSetPixelColors(channel byte, rgb []byte) ([]byte, error) {
	if len(rgb)%3 != 0 {
		return nil, fmt.Errorf("RGB payload length %d is not divisible by 3", len(rgb))
	}
	if len(rgb) > 0xffff {
		return nil, fmt.Errorf("OPC payload too large: %d bytes", len(rgb))
	}
	out := make([]byte, HeaderSize+len(rgb))
	out[0] = channel
	out[1] = CommandSetPixelColors
	binary.BigEndian.PutUint16(out[2:4], uint16(len(rgb)))
	copy(out[4:], rgb)
	return out, nil
}
