package controllerapi

import (
	"context"
	"net/netip"
	"testing"
	"time"

	cfgpkg "pxlblz-router/internal/config"
)

// The ESP test rig as reported by its /api/config (2026-10-01).
var espRig = RemoteConfig{
	HardwareProfile: "esp32-wroom-flex-8ws-2apa",
	TargetFps:       60,
	Outputs: []Output{
		{ID: 0, Type: "WS2812B", Enabled: false, PixelCount: 203, StartUniverse: 120, ColorOrder: "GRB"},
		{ID: 6, Type: "APA102", Enabled: true, PixelCount: 136, StartUniverse: 149, ColorOrder: "BGR"},
	},
}

func base() cfgpkg.Config {
	return cfgpkg.Config{
		Version: 2,
		Input:   cfgpkg.InputConfig{PixelCount: 100, FPSTarget: 30},
		ArtNet:  cfgpkg.ArtNet{UDPPort: 6454},
		Controllers: []cfgpkg.Controller{
			{Name: "OTHER", TargetIP: "10.0.0.1", FPSTarget: 30},
		},
		Routes: []cfgpkg.Route{
			{Name: "o", Enabled: true, TargetIP: "10.0.0.1", PixelStart: 0, PixelCount: 100, UniverseStart: 0, ColorOrder: "RGB"},
		},
	}
}

func TestImportNewControllerAppendsAfterUsedPixels(t *testing.T) {
	cfg, err := ImportOutputs(base(), "10.0.0.248", espRig, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Controllers) != 2 || cfg.Controllers[1].Name != "ESP32_248" || cfg.Controllers[1].FPSTarget != 60 {
		t.Fatalf("controller %+v", cfg.Controllers)
	}
	var got []cfgpkg.Route
	for _, r := range cfg.Routes {
		if r.TargetIP == "10.0.0.248" {
			got = append(got, r)
		}
	}
	if len(got) != 1 { // disabled WS2812B output skipped
		t.Fatalf("routes %+v", got)
	}
	r := got[0]
	if r.PixelStart != 100 || r.PixelCount != 136 || r.UniverseStart != 149 || r.ColorOrder != "RGB" || r.PhysicalPort != 7 {
		t.Fatalf("route %+v", r)
	}
	if cfg.Input.PixelCount != 236 {
		t.Fatalf("pixel_count %d", cfg.Input.PixelCount)
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestImportReplacesRoutesInPlace(t *testing.T) {
	cfg := base()
	cfg.Controllers = append(cfg.Controllers, cfgpkg.Controller{Name: "ESP_TEST", TargetIP: "10.0.0.248", FPSTarget: 30})
	cfg.Routes = append([]cfgpkg.Route{
		{Name: "old", Enabled: true, TargetIP: "10.0.0.248", PixelStart: 0, PixelCount: 127, UniverseStart: 149, ColorOrder: "RGB"},
	}, cfg.Routes[0])
	cfg.Routes[1].PixelStart = 127
	cfg.Input.PixelCount = 227

	out, err := ImportOutputs(cfg, "10.0.0.248", espRig, "ignored-for-existing")
	if err != nil {
		t.Fatal(err)
	}
	if out.Controllers[1].Name != "ESP_TEST" {
		t.Fatal("existing controller name must be kept")
	}
	for _, r := range out.Routes {
		if r.TargetIP == "10.0.0.248" && (r.PixelStart != 0 || r.PixelCount != 136) {
			t.Fatalf("route not replaced in place: %+v", r)
		}
		if r.Name == "old" {
			t.Fatal("old route kept")
		}
	}
	if out.Input.PixelCount != 227 {
		t.Fatalf("pixel_count %d", out.Input.PixelCount)
	}
	if len(cfg.Routes) != 2 || cfg.Routes[0].Name != "old" {
		t.Fatal("input config was modified")
	}
}

func TestImportRejectsNoOutputsAndBadIP(t *testing.T) {
	if _, err := ImportOutputs(base(), "10.0.0.248", RemoteConfig{Outputs: []Output{{ID: 0, Enabled: false, PixelCount: 10}}}, ""); err == nil {
		t.Fatal("no active outputs accepted")
	}
	if _, err := ImportOutputs(base(), "esp.local", espRig, ""); err == nil {
		t.Fatal("host name accepted as IP")
	}
}

func TestDiscoverRejectsHugeNetworksAndFindsNothingQuickly(t *testing.T) {
	if _, err := Discover(context.Background(), []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, time.Second); err == nil {
		t.Fatal("/8 accepted")
	}
	// TEST-NET-1 is never routed: every probe fails, the call must still return
	start := time.Now()
	found, _ := Discover(context.Background(), []netip.Prefix{netip.MustParsePrefix("192.0.2.0/28")}, 300*time.Millisecond)
	if len(found) != 0 || time.Since(start) > 5*time.Second {
		t.Fatalf("found %v in %v", found, time.Since(start))
	}
}
