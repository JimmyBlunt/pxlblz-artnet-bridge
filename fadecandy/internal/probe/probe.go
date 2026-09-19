package probe

import (
	"encoding/hex"
	"fmt"
	"strings"

	"pxlblz-fadecandy/internal/opc"
)

const PixelCount = 512

type Color struct {
	R byte
	G byte
	B byte
}

func ParseColor(s string) (Color, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	named := map[string]Color{
		"red":     {255, 0, 0},
		"green":   {0, 255, 0},
		"blue":    {0, 0, 255},
		"white":   {255, 255, 255},
		"black":   {0, 0, 0},
		"yellow":  {255, 255, 0},
		"cyan":    {0, 255, 255},
		"magenta": {255, 0, 255},
	}
	if c, ok := named[s]; ok {
		return c, nil
	}
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return Color{}, fmt.Errorf("color must be named or #RRGGBB")
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return Color{}, err
	}
	return Color{R: b[0], G: b[1], B: b[2]}, nil
}

func BlankFrame() []byte {
	return make([]byte, PixelCount*3)
}

func SetPixel(frame []byte, index int, c Color) error {
	if len(frame) != PixelCount*3 {
		return fmt.Errorf("frame length %d, expected %d", len(frame), PixelCount*3)
	}
	if index < 0 || index >= PixelCount {
		return fmt.Errorf("pixel %d out of range 0..511", index)
	}
	i := index * 3
	frame[i], frame[i+1], frame[i+2] = c.R, c.G, c.B
	return nil
}

func FillAll(frame []byte, c Color) error {
	if len(frame) != PixelCount*3 {
		return fmt.Errorf("frame length %d, expected %d", len(frame), PixelCount*3)
	}
	for i := 0; i < PixelCount; i++ {
		if err := SetPixel(frame, i, c); err != nil {
			return err
		}
	}
	return nil
}

func FillStrand(frame []byte, strand int, c Color) error {
	if strand < 0 || strand > 7 {
		return fmt.Errorf("strand %d out of range 0..7", strand)
	}
	start := strand * 64
	for i := 0; i < 64; i++ {
		if err := SetPixel(frame, start+i, c); err != nil {
			return err
		}
	}
	return nil
}

func BuildPacket(channel byte, frame []byte) ([]byte, error) {
	return opc.EncodeSetPixelColors(channel, frame)
}

func PacketSummary(packet []byte) string {
	if len(packet) < 4 {
		return "invalid packet"
	}
	payloadLen := int(packet[2])<<8 | int(packet[3])
	return fmt.Sprintf(
		"OPC channel=%d command=0x%02X payload=%d bytes pixels=%d total=%d bytes",
		packet[0],
		packet[1],
		payloadLen,
		payloadLen/3,
		len(packet),
	)
}
