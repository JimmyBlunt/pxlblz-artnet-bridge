package virtualcontroller

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestVisualizerEndpoints(t *testing.T) {
	c, err := New(backPanelConfig(), "10.0.0.253", Options{})
	if err != nil { t.Fatal(err) }
	h := c.Handler()

	statusReq := httptest.NewRequest("GET", "/api/status", nil)
	statusRec := httptest.NewRecorder()
	h.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != 200 { t.Fatalf("status code=%d", statusRec.Code) }
	var s Snapshot
	if err := json.Unmarshal(statusRec.Body.Bytes(), &s); err != nil { t.Fatal(err) }
	if s.TargetIP != "10.0.0.253" || s.ExpectedUniverses != 29 || len(s.Routes) != 7 {
		t.Fatalf("status=%+v", s)
	}

	frameReq := httptest.NewRequest("GET", "/api/frame", nil)
	frameRec := httptest.NewRecorder()
	h.ServeHTTP(frameRec, frameReq)
	if frameRec.Code != 200 { t.Fatalf("frame code=%d", frameRec.Code) }
	body, err := io.ReadAll(frameRec.Result().Body)
	if err != nil { t.Fatal(err) }
	if len(body) != 4105*3 {
		t.Fatalf("frame bytes=%d want=%d", len(body), 4105*3)
	}

	rootReq := httptest.NewRequest("GET", "/", nil)
	rootRec := httptest.NewRecorder()
	h.ServeHTTP(rootRec, rootReq)
	if rootRec.Code != 200 { t.Fatalf("root code=%d", rootRec.Code) }
	html := rootRec.Body.String()
	for _, needle := range []string{"PXLBLZ Virtual Art-Net Controller", "/api/status", "/api/frame", "canvas"} {
		if !strings.Contains(html, needle) { t.Fatalf("visualizer missing %q", needle) }
	}
}

func TestDisplayRGBConvertsWireOrder(t *testing.T) {
	cfg := backPanelConfig()
	cfg.Routes = cfg.Routes[:1]
	cfg.Routes[0].PixelStart = 0
	cfg.Routes[0].PixelCount = 1
	cfg.Routes[0].UniverseStart = 1
	cfg.Routes[0].ColorOrder = "GRB"
	cfg.Input.PixelCount = 1
	c, err := New(cfg, "10.0.0.253", Options{})
	if err != nil { t.Fatal(err) }

	// Wire order GRB bytes: G=20, R=10, B=30.
	p := dmxPacket(t, 1, 1, 3)
	// BuildDmx pads to four bytes; overwrite three useful bytes.
	p[18], p[19], p[20] = 20, 10, 30
	now := time.Unix(20,0)
	c.IngestPacket(p, now)
	c.Tick(now)
	got := c.DisplayRGB()
	want := []byte{10,20,30}
	for i := range want {
		if got[i] != want[i] { t.Fatalf("rgb=%v want=%v", got, want) }
	}
}
