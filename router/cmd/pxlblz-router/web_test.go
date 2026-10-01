package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cfgpkg "pxlblz-router/internal/config"
	"pxlblz-router/internal/engine"
)

func startTestWeb(t *testing.T) (base string, cfgPath string, eng *engine.Engine) {
	t.Helper()
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, "routes.json")
	cfg := cfgpkg.Config{
		Version: 2, Comment: "keep me",
		Input:       cfgpkg.InputConfig{PixelCount: 10, FPSTarget: 30},
		ArtNet:      cfgpkg.ArtNet{UDPPort: 6454, Unicast: true},
		Controllers: []cfgpkg.Controller{{Name: "A", TargetIP: "10.0.0.1", FPSTarget: 30}},
		Routes:      []cfgpkg.Route{{Name: "r", Enabled: true, TargetIP: "10.0.0.1", PixelCount: 10, UniverseStart: 1, ColorOrder: "RGB"}},
	}
	if _, err := saveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	eng, err := engine.New(ctx, cfg, engine.Options{DryRun: true, Mode: "ws", StaleMSOverride: -1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(eng.Close)
	stop, addr, err := startWeb("127.0.0.1:0", &webServer{eng: eng, configPath: cfgPath, start: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	return "http://" + addr, cfgPath, eng
}

func do(t *testing.T, method, url string, body any, hdr map[string]string) (*http.Response, map[string]any) {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, url, rd)
	for k, v := range hdr {
		if k == "Host" {
			req.Host = v
		} else {
			req.Header.Set(k, v)
		}
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res, out
}

func TestWebGuardRejectsForeignHostAndOrigin(t *testing.T) {
	base, _, _ := startTestWeb(t)
	if res, _ := do(t, "GET", base+"/api/config", nil, map[string]string{"Host": "evil.example:80"}); res.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign Host: %d", res.StatusCode)
	}
	if res, _ := do(t, "POST", base+"/api/test/stop", nil, map[string]string{"Origin": "http://evil.example"}); res.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign Origin: %d", res.StatusCode)
	}
	if res, _ := do(t, "POST", base+"/api/test/stop", nil, map[string]string{"Origin": base}); res.StatusCode != http.StatusOK {
		t.Fatalf("own Origin: %d", res.StatusCode)
	}
	res, err := http.Get(base + "/")
	if err != nil || res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("page: %v %v", res, err)
	}
	res.Body.Close()
}

func TestWebPutSavesWithBackupAndApplies(t *testing.T) {
	base, cfgPath, eng := startTestWeb(t)
	_, doc := do(t, "GET", base+"/api/config", nil, nil)
	cfg := doc["config"].(map[string]any)
	ctrl := cfg["controllers"].([]any)[0].(map[string]any)
	ctrl["target_ip"] = "10.0.0.248"
	cfg["routes"].([]any)[0].(map[string]any)["target_ip"] = "10.0.0.248"

	// check does not apply
	if _, chk := do(t, "POST", base+"/api/config/check", cfg, nil); chk["ok"] != true {
		t.Fatalf("check: %v", chk)
	}
	if eng.Config().Controllers[0].TargetIP != "10.0.0.1" {
		t.Fatal("check applied the config")
	}

	res, put := do(t, "PUT", base+"/api/config", cfg, nil)
	if res.StatusCode != 200 || put["ok"] != true {
		t.Fatalf("put: %d %v", res.StatusCode, put)
	}
	if eng.Config().Controllers[0].TargetIP != "10.0.0.248" {
		t.Fatal("not applied")
	}
	saved, _ := cfgpkg.Load(cfgPath)
	if saved.Controllers[0].TargetIP != "10.0.0.248" || saved.Comment != "keep me" {
		t.Fatalf("saved %+v", saved)
	}
	backup, _ := put["backup"].(string)
	old, err := os.ReadFile(backup)
	if err != nil || !strings.Contains(string(old), "10.0.0.1") {
		t.Fatalf("backup %q: %v", backup, err)
	}

	// invalid config: 400, nothing applied or saved
	cfg["routes"].([]any)[0].(map[string]any)["target_ip"] = "10.0.0.9"
	if res, bad := do(t, "PUT", base+"/api/config", cfg, nil); res.StatusCode != 400 || bad["ok"] != false {
		t.Fatalf("invalid put: %d %v", res.StatusCode, bad)
	}
	if again, _ := cfgpkg.Load(cfgPath); again.Routes[0].TargetIP != "10.0.0.248" {
		t.Fatal("invalid config was saved")
	}
	// unknown fields are rejected instead of silently dropped
	cfg["routes"].([]any)[0].(map[string]any)["target_ip"] = "10.0.0.248"
	cfg["typo_field"] = 1
	if _, bad := do(t, "POST", base+"/api/config/check", cfg, nil); bad["ok"] != false {
		t.Fatal("unknown field accepted")
	}
}

func TestPruneBackupsKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < maxBackups+5; i++ {
		name := filepath.Join(dir, "routes.20261001-0000"+string(rune('a'+i))+".json")
		_ = os.WriteFile(name, []byte("{}"), 0o644)
	}
	pruneBackups(dir, "routes")
	m, _ := filepath.Glob(filepath.Join(dir, "routes.*.json"))
	if len(m) != maxBackups {
		t.Fatalf("%d backups kept", len(m))
	}
}

func TestSelfCheckDetectsForeignProgram(t *testing.T) {
	base, _, _ := startTestWeb(t)
	if err := selfCheck(strings.TrimPrefix(base, "http://")); err != nil {
		t.Fatalf("own router not recognised: %v", err)
	}
	foreign := http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{}")) })}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = foreign.Serve(ln) }()
	defer foreign.Close()
	if err := selfCheck(ln.Addr().String()); err == nil || !strings.Contains(err.Error(), "ANOTHER program") {
		t.Fatalf("foreign program not detected: %v", err)
	}
}
