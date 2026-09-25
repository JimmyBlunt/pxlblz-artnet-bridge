package router

import (
	"testing"

	cfgpkg "pxlblz-router/internal/config"
)

func BenchmarkSendFrameDryRunBackPanel8186(b *testing.B) {
	cfg := cfgpkg.Config{
		Version: 1,
		Input: cfgpkg.InputConfig{PixelCount: 8186, FPSTarget: 30},
		ArtNet: cfgpkg.ArtNet{UDPPort: 6454, Unicast: true},
		Routes: []cfgpkg.Route{
			{Name: "P1", Enabled: true, TargetIP: "127.0.0.1", PixelStart: 1440, PixelCount: 203, UniverseStart: 120, ColorOrder: "RGB"},
			{Name: "P2", Enabled: true, TargetIP: "127.0.0.1", PixelStart: 3744, PixelCount: 738, UniverseStart: 122, ColorOrder: "RGB"},
			{Name: "P3", Enabled: true, TargetIP: "127.0.0.1", PixelStart: 4482, PixelCount: 880, UniverseStart: 127, ColorOrder: "RGB"},
			{Name: "P4", Enabled: true, TargetIP: "127.0.0.1", PixelStart: 5362, PixelCount: 810, UniverseStart: 133, ColorOrder: "RGB"},
			{Name: "P5", Enabled: true, TargetIP: "127.0.0.1", PixelStart: 6172, PixelCount: 352, UniverseStart: 139, ColorOrder: "RGB"},
			{Name: "P6", Enabled: true, TargetIP: "127.0.0.1", PixelStart: 6524, PixelCount: 610, UniverseStart: 142, ColorOrder: "RGB"},
			{Name: "P7", Enabled: true, TargetIP: "127.0.0.1", PixelStart: 7134, PixelCount: 512, UniverseStart: 146, ColorOrder: "RGB"},
		},
	}
	benchmarkConfig(b, cfg)
}

func BenchmarkSendFrameDryRunRGB32768(b *testing.B) {
	cfg := singleRouteBenchmarkConfig(32768, "RGB")
	benchmarkConfig(b, cfg)
}

func BenchmarkSendFrameDryRunGRB32768(b *testing.B) {
	cfg := singleRouteBenchmarkConfig(32768, "GRB")
	benchmarkConfig(b, cfg)
}

func singleRouteBenchmarkConfig(pixels int, order string) cfgpkg.Config {
	return cfgpkg.Config{
		Version: 1,
		Input: cfgpkg.InputConfig{PixelCount: pixels, FPSTarget: 120},
		ArtNet: cfgpkg.ArtNet{UDPPort: 6454, Unicast: true},
		Routes: []cfgpkg.Route{{
			Name: "bench", Enabled: true, TargetIP: "127.0.0.1",
			PixelStart: 0, PixelCount: pixels, UniverseStart: 0, ColorOrder: order,
		}},
	}
}

func benchmarkConfig(b *testing.B, cfg cfgpkg.Config) {
	if err := cfg.Validate(); err != nil {
		b.Fatal(err)
	}
	r, err := New(cfg, true)
	if err != nil {
		b.Fatal(err)
	}
	defer r.Close()
	frame := make([]byte, cfg.Input.PixelCount*3)
	for i := range frame {
		frame[i] = byte(i)
	}
	b.SetBytes(int64(len(frame)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := r.SendFrame(frame); err != nil {
			b.Fatal(err)
		}
	}
}
