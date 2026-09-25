package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAllShippedConfigsLoad(t *testing.T) {
	files, err := filepath.Glob("../../config/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no configs found: %v", err)
	}
	for _, f := range files {
		if _, err := Load(f); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

func TestV1ConfigStaysValidAndDefaults(t *testing.T) {
	c, err := Parse([]byte(`{"version":1,"input":{"pixel_count":10},"routes":[
		{"name":"a","enabled":true,"target_ip":"10.0.0.1","pixel_start":0,"pixel_count":10,"universe_start":0}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.ArtNet.UDPPort != 6454 || c.Input.FPSTarget != 60 || c.Input.OnStale != OnStaleHold || c.Routes[0].ColorOrder != "RGB" {
		t.Fatalf("defaults not applied: %+v", c)
	}
	ct := c.ControllerFor("10.0.0.1")
	if ct.Name != "10.0.0.1" || ct.FPSTarget != 60 || !ct.IsEnabled() {
		t.Fatalf("synth controller wrong: %+v", ct)
	}
}

func TestControllerValidation(t *testing.T) {
	base := `{"version":2,"input":{"pixel_count":10,"fps_target":30},"controllers":[%s],"routes":[
		{"name":"a","enabled":true,"target_ip":"10.0.0.1","pixel_start":0,"pixel_count":10,"universe_start":0}]}`
	cases := map[string]string{
		"missing controller for route": `{"name":"X","target_ip":"10.0.0.2"}`,
		"bad fps":                      `{"name":"X","target_ip":"10.0.0.1","fps_target":500}`,
		"duplicate ip":                 `{"name":"X","target_ip":"10.0.0.1"},{"name":"Y","target_ip":"10.0.0.1"}`,
		"empty name":                   `{"name":"","target_ip":"10.0.0.1"}`,
	}
	for name, ctrls := range cases {
		if _, err := Parse([]byte(strings.Replace(base, "%s", ctrls, 1))); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	c, err := Parse([]byte(strings.Replace(base, "%s", `{"name":"X","target_ip":"10.0.0.1"}`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if c.Controllers[0].FPSTarget != 30 {
		t.Fatalf("controller fps should default to input fps, got %d", c.Controllers[0].FPSTarget)
	}
}

func TestOnStaleValidation(t *testing.T) {
	_, err := Parse([]byte(`{"version":1,"input":{"pixel_count":10,"on_stale":"explode"},"routes":[]}`))
	if err == nil {
		t.Fatal("expected on_stale error")
	}
}

func TestWarningsDetectPixelOverlapAndIdleController(t *testing.T) {
	c, err := Parse([]byte(`{"version":2,"input":{"pixel_count":100},
		"controllers":[{"name":"A","target_ip":"10.0.0.1"},{"name":"B","target_ip":"10.0.0.2"}],
		"routes":[
		{"name":"a","enabled":true,"target_ip":"10.0.0.1","pixel_start":0,"pixel_count":50,"universe_start":0},
		{"name":"b","enabled":true,"target_ip":"10.0.0.1","pixel_start":40,"pixel_count":10,"universe_start":5}]}`))
	if err != nil {
		t.Fatal(err)
	}
	w := strings.Join(c.Warnings(), "\n")
	if !strings.Contains(w, "overlapping logical pixels 40..49") || !strings.Contains(w, `controller "B"`) {
		t.Fatalf("warnings missing: %s", w)
	}
}

func TestInstallationConfigTopology(t *testing.T) {
	c, err := Load("../../config/routes.installation-full.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Warnings()) != 0 {
		t.Fatalf("installation config should be warning-free: %v", c.Warnings())
	}
	uni := map[string][]int{}
	for _, r := range c.Routes {
		for u := 0; u < r.Universes(); u++ {
			uni[r.TargetIP] = append(uni[r.TargetIP], r.UniverseStart+u)
		}
	}
	want := map[string]int{"10.0.0.244": 9, "10.0.0.251": 6, "10.0.0.253": 29}
	for ip, n := range want {
		if len(uni[ip]) != n {
			t.Errorf("%s: %d universes, want %d (%v)", ip, len(uni[ip]), n, uni[ip])
		}
	}
	for _, u := range uni["10.0.0.253"] {
		if u == 138 {
			t.Error("U138 must not be sent to BACK_PANEL")
		}
	}
}
