package config

import "testing"

func ptr(v float64) *float64 { return &v }

func TestRouteBrightnessDefaultsToOne(t *testing.T) {
	r := Route{}
	if got := r.BrightnessLevel(); got != 1 {
		t.Fatalf("BrightnessLevel()=%v want 1", got)
	}
}

func TestValidateRouteBrightness(t *testing.T) {
	base := Config{
		Version: 1,
		Input: InputConfig{PixelCount: 10, FPSTarget: 30},
		ArtNet: ArtNet{UDPPort: 6454, Unicast: true},
		Routes: []Route{{
			Name: "test", Enabled: true, TargetIP: "127.0.0.1",
			PixelStart: 0, PixelCount: 10, UniverseStart: 0, ColorOrder: "RGB",
		}},
	}
	for _, level := range []float64{0, 0.3, 0.7, 1} {
		cfg := base
		cfg.Routes = append([]Route(nil), base.Routes...)
		cfg.Routes[0].Brightness = ptr(level)
		if err := cfg.Validate(); err != nil {
			t.Fatalf("brightness %v rejected: %v", level, err)
		}
	}
	for _, level := range []float64{-0.01, 1.01, 2} {
		cfg := base
		cfg.Routes = append([]Route(nil), base.Routes...)
		cfg.Routes[0].Brightness = ptr(level)
		if err := cfg.Validate(); err == nil {
			t.Fatalf("brightness %v should be rejected", level)
		}
	}
}
