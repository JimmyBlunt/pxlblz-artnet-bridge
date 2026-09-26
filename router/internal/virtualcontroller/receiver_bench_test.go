package virtualcontroller

import (
	"testing"
	"time"
)

func BenchmarkBackPanelCompleteFrame(b *testing.B) {
	cfg := backPanelConfig()
	c, err := New(cfg, "10.0.0.253", Options{})
	if err != nil { b.Fatal(err) }
	universes := c.ExpectedUniverses()
	packets := make([][]byte, len(universes))
	for i,u := range universes {
		packets[i] = dmxPacket(&testing.T{}, u, 1, wireDataBytes(u))
	}
	now := time.Unix(100,0)
	b.ReportAllocs()
	b.ResetTimer()
	for i:=0;i<b.N;i++ {
		seq := byte(i%255 + 1)
		for j,u := range universes {
			_ = u
			p := append([]byte(nil), packets[j]...)
			p[12] = seq
			c.IngestPacket(p, now)
			now = now.Add(10*time.Microsecond)
		}
		c.Tick(now)
		now = now.Add(33*time.Millisecond)
	}
}

func BenchmarkBackPanelSingleArtDmx(b *testing.B) {
	c, err := New(backPanelConfig(), "10.0.0.253", Options{})
	if err != nil { b.Fatal(err) }
	p := dmxPacket(&testing.T{}, 120, 1, 510)
	now := time.Unix(200,0)
	b.ReportAllocs()
	b.ResetTimer()
	for i:=0;i<b.N;i++ {
		p[12] = byte(i%255 + 1)
		c.IngestPacket(p, now)
		now = now.Add(time.Millisecond)
	}
}
