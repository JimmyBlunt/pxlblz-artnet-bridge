package virtualcontroller

import (
	"testing"
	"time"

	"pxlblz-router/internal/artnet"
)

func benchPacket(b *testing.B, u uint16, seq byte, dataBytes int) []byte {
	b.Helper()
	data := make([]byte, dataBytes)
	buf := make([]byte, artnet.HeaderSize+artnet.MaxDmxPayload)
	p, err := artnet.BuildDmx(buf, u, seq, data)
	if err != nil { b.Fatal(err) }
	return append([]byte(nil), p...)
}

func BenchmarkBackPanelCompleteFrame(b *testing.B) {
	c, err := New(backPanelConfig(), "10.0.0.253", Options{})
	if err != nil { b.Fatal(err) }
	universes := c.ExpectedUniverses()
	packets := make([][]byte, len(universes))
	for i,u := range universes {
		packets[i] = benchPacket(b, u, 1, wireDataBytes(u))
	}
	now := time.Unix(100,0)
	b.ReportAllocs()
	b.ResetTimer()
	for i:=0;i<b.N;i++ {
		seq := byte(i%255 + 1)
		for j := range packets {
			packets[j][12] = seq
			c.IngestPacket(packets[j], now)
			now = now.Add(10*time.Microsecond)
		}
		c.Tick(now)
		now = now.Add(33*time.Millisecond)
	}
}

func BenchmarkBackPanelSingleArtDmx(b *testing.B) {
	c, err := New(backPanelConfig(), "10.0.0.253", Options{})
	if err != nil { b.Fatal(err) }
	p := benchPacket(b, 120, 1, 510)
	now := time.Unix(200,0)
	b.ReportAllocs()
	b.ResetTimer()
	for i:=0;i<b.N;i++ {
		p[12] = byte(i%255 + 1)
		c.IngestPacket(p, now)
		now = now.Add(time.Millisecond)
	}
}
