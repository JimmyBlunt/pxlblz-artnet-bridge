package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	cfgpkg "pxlblz-router/internal/config"
	"pxlblz-router/internal/controllerapi"
	"pxlblz-router/internal/engine"
	"pxlblz-router/internal/router"
	"pxlblz-router/internal/scheduler"
	"pxlblz-router/internal/wsmini"
)

//go:embed ui/index.html
var uiHTML []byte

const maxBackups = 20

type webServer struct {
	eng        *engine.Engine
	ws         *wsmini.Server
	configPath string
	start      time.Time
	port       string

	remoteMu sync.Mutex
	remote   map[string]*remoteCache
}

// remoteCache keeps the last good answers of a controller. Busy controllers
// (60 fps output + Wi-Fi) sometimes answer in seconds; the page then shows the
// last values instead of "unreachable".
type remoteCache struct {
	config     *controllerapi.RemoteConfig
	configAt   time.Time
	status     map[string]any
	statusAt   time.Time
	lastErr    string
	inProgress bool
}

type statusDoc struct {
	Version    string  `json:"version"`
	UptimeSec  float64 `json:"uptime_s"`
	Input      string  `json:"input"`
	RXFrames   uint64  `json:"rx_frames"`
	RXReplaced uint64  `json:"rx_replaced"`
	RXInvalid  uint64  `json:"rx_invalid"`
	// PixelCount / VariableSize: what input frames must look like; with
	// variable_size false only frames of exactly PixelCount pixels are taken.
	PixelCount   int  `json:"pixel_count"`
	VariableSize bool `json:"variable_size"`
	// LastInvalidPixels: pixel count of the last rejected frame (0 = none).
	LastInvalidPixels int                          `json:"last_invalid_pixels,omitempty"`
	LastFrameMS       *float64                     `json:"last_frame_age_ms"`
	WSClients         int64                        `json:"ws_clients"`
	Controllers       []scheduler.ControllerStatus `json:"controllers"`
	ConfigPath        string                       `json:"config_path"`
	AppliedAt         string                       `json:"config_applied_at"`
	Test              string                       `json:"test_pattern,omitempty"`
	TestLeftSec       float64                      `json:"test_left_s,omitempty"`
}

// startWeb serves the status endpoint (as before), the configuration page and
// its JSON API. Only loopback clients are accepted.
func startWeb(listen string, s *webServer) (func(), string, error) {
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, "", err
	}
	_, s.port, _ = net.SplitHostPort(ln.Addr().String())
	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(uiHTML)
	})
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("POST /api/config/check", s.handleCheckConfig)
	mux.HandleFunc("PUT /api/config", s.handlePutConfig)
	mux.HandleFunc("GET /api/subnets", s.handleSubnets)
	mux.HandleFunc("GET /api/discover", s.handleDiscover)
	mux.HandleFunc("GET /api/controller", s.handleController)
	mux.HandleFunc("POST /api/import", s.handleImport)
	mux.HandleFunc("POST /api/test", s.handleTest)
	mux.HandleFunc("POST /api/test/stop", s.handleTestStop)
	srv := &http.Server{Handler: s.guard(mux), ReadHeaderTimeout: 2 * time.Second}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Println("status endpoint stopped:", err)
		}
	}()
	if err := selfCheck(ln.Addr().String()); err != nil {
		fmt.Println("WARNING:", err)
	}
	return func() {
		ctx, c := context.WithTimeout(context.Background(), 500*time.Millisecond)
		_ = srv.Shutdown(ctx)
		c()
	}, ln.Addr().String(), nil
}

// selfCheck asks the page once and verifies that this router answered.
// Windows lets another program (e.g. TouchDesigner on 0.0.0.0:9981) share a
// port; requests may then reach the other program.
func selfCheck(addr string) error {
	c := &http.Client{Timeout: 2 * time.Second}
	res, err := c.Get("http://" + addr + "/status")
	if err != nil {
		return fmt.Errorf("status page %s not reachable: %v", addr, err)
	}
	res.Body.Close()
	if res.Header.Get("X-PXLBLZ-Router") == "" {
		return fmt.Errorf("port %s answers for ANOTHER program - status/configuration page not reachable; start with --status-listen 127.0.0.1:<free port>", addr)
	}
	return nil
}

// guard rejects non-loopback Host headers (DNS rebinding) and cross-origin
// writes. /status stays readable cross-origin for tools and PXLBLZ.
func (s *webServer) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-PXLBLZ-Router", version)
		host := r.Host
		allowed := host == "127.0.0.1:"+s.port || host == "localhost:"+s.port || host == "[::1]:"+s.port
		if !allowed {
			http.Error(w, "nur lokal erreichbar (127.0.0.1)", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" && o != "http://"+host {
				http.Error(w, "fremder Ursprung", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]any{"ok": false, "error": err.Error()})
}

func (s *webServer) handleStatus(w http.ResponseWriter, _ *http.Request) {
	latest := s.eng.Latest()
	ist := latest.Stats()
	doc := statusDoc{
		Version:           version,
		UptimeSec:         time.Since(s.start).Seconds(),
		Input:             s.eng.Mode(),
		RXFrames:          ist.Submitted,
		RXReplaced:        ist.Replaced,
		RXInvalid:         ist.Invalid,
		PixelCount:        latest.Size() / 3,
		VariableSize:      s.eng.Config().Input.VariableSize,
		LastInvalidPixels: ist.LastInvalidLen / 3,
		Controllers:       s.eng.Scheduler().Status(),
		ConfigPath:        s.configPath,
		AppliedAt:         s.eng.AppliedAt().Format(time.RFC3339),
	}
	if !ist.LastAt.IsZero() {
		age := float64(time.Since(ist.LastAt).Microseconds()) / 1000
		doc.LastFrameMS = &age
	}
	if s.ws != nil {
		doc.WSClients = s.ws.Stats().Active
	}
	if name, left := s.eng.TestStatus(); name != "" {
		doc.Test, doc.TestLeftSec = name, left.Seconds()
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	writeJSON(w, http.StatusOK, doc)
}

type configDoc struct {
	OK       bool          `json:"ok"`
	Path     string        `json:"path,omitempty"`
	Backup   string        `json:"backup,omitempty"`
	Config   cfgpkg.Config `json:"config"`
	Summary  []string      `json:"summary,omitempty"`
	Warnings []string      `json:"warnings,omitempty"`
	Error    string        `json:"error,omitempty"`
}

func summarize(cfg cfgpkg.Config) []string {
	r, err := router.New(cfg, true)
	if err != nil {
		return nil
	}
	defer r.Close()
	return r.RouteSummary()
}

func (s *webServer) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := s.eng.Config()
	writeJSON(w, http.StatusOK, configDoc{OK: true, Path: s.configPath, Config: cfg, Summary: summarize(cfg), Warnings: cfg.Warnings()})
}

func readConfigBody(r *http.Request) (cfgpkg.Config, error) {
	var cfg cfgpkg.Config
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("Konfiguration ist kein gueltiges JSON: %v", err)
	}
	return cfg, nil
}

func (s *webServer) handleCheckConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := readConfigBody(r)
	if err == nil {
		cfg, err = s.eng.Prepare(cfg)
	}
	if err != nil {
		writeJSON(w, http.StatusOK, configDoc{OK: false, Error: err.Error(), Config: cfg})
		return
	}
	writeJSON(w, http.StatusOK, configDoc{OK: true, Config: cfg, Summary: summarize(cfg), Warnings: cfg.Warnings()})
}

func (s *webServer) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := readConfigBody(r)
	if err == nil {
		cfg, err = s.eng.Prepare(cfg)
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, configDoc{OK: false, Error: err.Error(), Config: cfg})
		return
	}
	backup, err := saveConfig(s.configPath, cfg)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("Speichern fehlgeschlagen: %v", err))
		return
	}
	if err := s.eng.Apply(cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, configDoc{OK: false, Error: "gespeichert, aber nicht angewendet: " + err.Error(), Path: s.configPath, Backup: backup, Config: cfg})
		return
	}
	fmt.Printf("Config applied from web UI (%s, backup %s)\n", s.configPath, backup)
	for _, line := range summarize(cfg) {
		fmt.Println("  " + line)
	}
	writeJSON(w, http.StatusOK, configDoc{OK: true, Path: s.configPath, Backup: backup, Config: cfg, Summary: summarize(cfg), Warnings: cfg.Warnings()})
}

// saveConfig writes cfg atomically and keeps the previous file in
// <dir>/backups/<name>.<timestamp>.json (newest maxBackups per config).
func saveConfig(path string, cfg cfgpkg.Config) (string, error) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	backup := ""
	if old, err := os.ReadFile(path); err == nil {
		bdir := filepath.Join(dir, "backups")
		if err := os.MkdirAll(bdir, 0o755); err != nil {
			return "", err
		}
		backup = filepath.Join(bdir, base+"."+time.Now().Format("20060102-150405")+".json")
		if err := os.WriteFile(backup, old, 0o644); err != nil {
			return "", err
		}
		pruneBackups(bdir, base)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	return backup, os.Rename(tmp, path)
}

func pruneBackups(dir, base string) {
	matches, _ := filepath.Glob(filepath.Join(dir, base+".*.json"))
	sort.Strings(matches) // timestamps sort chronologically
	for len(matches) > maxBackups {
		_ = os.Remove(matches[0])
		matches = matches[1:]
	}
}

func (s *webServer) handleSubnets(w http.ResponseWriter, _ *http.Request) {
	var out []string
	for _, p := range controllerapi.LocalSubnets() {
		out = append(out, p.String())
	}
	writeJSON(w, http.StatusOK, map[string]any{"subnets": out})
}

func (s *webServer) handleDiscover(w http.ResponseWriter, r *http.Request) {
	var prefixes []netip.Prefix
	for _, n := range r.URL.Query()["net"] {
		p, err := netip.ParsePrefix(strings.TrimSpace(n))
		if err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("ungueltiges Netz %q (Beispiel 10.0.0.0/24)", n))
			return
		}
		prefixes = append(prefixes, p)
	}
	if len(prefixes) == 0 {
		prefixes = controllerapi.LocalSubnets()
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	found, err := controllerapi.Discover(ctx, prefixes, 2500*time.Millisecond)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var nets []string
	for _, p := range prefixes {
		nets = append(nets, p.String())
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "nets": nets, "found": found})
}

// handleController returns the controller's outputs (refreshed every 30 s)
// and counters. One request per controller at a time; a slow answer returns
// the cached values with their age.
func (s *webServer) handleController(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	if !controllerapi.ValidIP(ip) {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("ungueltige IP %q", ip))
		return
	}
	s.remoteMu.Lock()
	if s.remote == nil {
		s.remote = map[string]*remoteCache{}
	}
	c := s.remote[ip]
	if c == nil {
		c = &remoteCache{}
		s.remote[ip] = c
	}
	refresh := !c.inProgress
	needConfig := c.config == nil || time.Since(c.configAt) > 30*time.Second
	if refresh {
		c.inProgress = true
	}
	s.remoteMu.Unlock()

	if refresh {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		var rc controllerapi.RemoteConfig
		var cfgErr, stErr error
		if needConfig {
			rc, cfgErr = controllerapi.FetchConfig(ctx, ip)
		}
		st, stErr := controllerapi.FetchStatus(ctx, ip)
		cancel()
		s.remoteMu.Lock()
		now := time.Now()
		if needConfig && cfgErr == nil {
			c.config, c.configAt = &rc, now
		}
		if stErr == nil {
			c.status, c.statusAt = st, now
			c.lastErr = ""
		} else {
			c.lastErr = stErr.Error()
		}
		if cfgErr != nil && c.config == nil {
			c.lastErr = cfgErr.Error()
		}
		c.inProgress = false
		s.remoteMu.Unlock()
	}

	s.remoteMu.Lock()
	out := map[string]any{"ip": ip}
	if c.config != nil {
		out["config"] = c.config
	}
	if c.status != nil {
		out["status"] = c.status
		out["status_age_s"] = time.Since(c.statusAt).Seconds()
	}
	// "unreachable" only when nothing usable arrived for 10 s
	if c.lastErr != "" && (c.status == nil || time.Since(c.statusAt) > 10*time.Second) {
		out["error"] = c.lastErr
	}
	s.remoteMu.Unlock()
	writeJSON(w, http.StatusOK, out)
}

// handleImport reads the controller's outputs and returns the draft config
// (request body) with matching routes. Nothing is applied or saved.
func (s *webServer) handleImport(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	draft, err := readConfigBody(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	rc, err := controllerapi.FetchConfig(ctx, ip)
	if err != nil {
		writeErr(w, http.StatusBadGateway, fmt.Errorf("Controller %s nicht lesbar: %v", ip, err))
		return
	}
	cfg, err := controllerapi.ImportOutputs(draft, ip, rc, r.URL.Query().Get("name"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	doc := configDoc{OK: true, Config: cfg}
	if prepared, err := s.eng.Prepare(cfg); err != nil {
		doc.OK, doc.Error = false, err.Error()
	} else {
		doc.Summary, doc.Warnings = summarize(prepared), prepared.Warnings()
	}
	writeJSON(w, http.StatusOK, doc)
}

func (s *webServer) handleTest(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sec, _ := strconv.Atoi(q.Get("seconds"))
	if sec == 0 {
		sec = 30
	}
	if err := s.eng.Test(q.Get("pattern"), time.Duration(sec)*time.Second); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "pattern": q.Get("pattern"), "seconds": sec})
}

func (s *webServer) handleTestStop(w http.ResponseWriter, _ *http.Request) {
	s.eng.StopTest()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
