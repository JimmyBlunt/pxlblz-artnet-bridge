// Package controllerapi talks to Art-Net controllers that expose the project's
// web firmware API (GET /api/config, GET /api/status; ESP32 and Teensy builds),
// finds them on the local network and turns their output table into routes.
package controllerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	cfgpkg "pxlblz-router/internal/config"
)

// Output is one physical output as reported by the controller firmware.
type Output struct {
	ID            int    `json:"id"`
	Type          string `json:"type"`
	Enabled       bool   `json:"enabled"`
	PixelCount    int    `json:"pixelCount"`
	StartUniverse int    `json:"startUniverse"`
	ColorOrder    string `json:"colorOrder"`
}

// RemoteConfig is the subset of GET /api/config the router needs.
type RemoteConfig struct {
	HardwareProfile string   `json:"hardwareProfile"`
	TargetFps       int      `json:"targetFps"`
	PixelCount      int      `json:"pixelCount"`
	UniverseCount   int      `json:"universeCount"`
	Outputs         []Output `json:"outputs"`
}

// Found is one controller discovered on the network.
type Found struct {
	IP     string       `json:"ip"`
	Config RemoteConfig `json:"config"`
}

var client = &http.Client{Timeout: 3 * time.Second}

func getJSON(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", url, res.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(v)
}

// ValidIP accepts dotted IPv4 only (no host names, no ports).
func ValidIP(ip string) bool {
	a, err := netip.ParseAddr(ip)
	return err == nil && a.Is4()
}

// FetchConfig reads GET http://ip/api/config.
func FetchConfig(ctx context.Context, ip string) (RemoteConfig, error) {
	if !ValidIP(ip) {
		return RemoteConfig{}, fmt.Errorf("ungueltige IP %q", ip)
	}
	var rc RemoteConfig
	if err := getJSON(ctx, "http://"+ip+"/api/config", &rc); err != nil {
		return RemoteConfig{}, err
	}
	if rc.Outputs == nil {
		return RemoteConfig{}, fmt.Errorf("%s: /api/config enthaelt keine Ausgaenge (unbekannte Firmware)", ip)
	}
	return rc, nil
}

// FetchStatus reads GET http://ip/api/status as a generic JSON object.
func FetchStatus(ctx context.Context, ip string) (map[string]any, error) {
	if !ValidIP(ip) {
		return nil, fmt.Errorf("ungueltige IP %q", ip)
	}
	out := map[string]any{}
	return out, getJSON(ctx, "http://"+ip+"/api/status", &out)
}

// LocalSubnets returns the /24 networks of the PC's private IPv4 addresses
// (the usual LED network sizes). Loopback and link-local are skipped.
func LocalSubnets() []netip.Prefix {
	seen := map[netip.Prefix]bool{}
	var out []netip.Prefix
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip, ok := netip.AddrFromSlice(ipn.IP.To4())
			if !ok || !ip.Is4() || !ip.IsPrivate() {
				continue
			}
			p, _ := ip.Prefix(24)
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Addr().Less(out[j].Addr()) })
	return out
}

// Discover probes every host of the given prefixes (each at most /22) for the
// controller API. Results are sorted by IP.
func Discover(ctx context.Context, prefixes []netip.Prefix, perHost time.Duration) ([]Found, error) {
	var hosts []netip.Addr
	for _, p := range prefixes {
		if !p.Addr().Is4() || p.Bits() < 22 {
			return nil, fmt.Errorf("Netz %s ist zu gross (maximal /22)", p)
		}
		p = p.Masked()
		for a := p.Addr().Next(); p.Contains(a); a = a.Next() {
			if a.As4()[3] == 255 {
				continue
			}
			hosts = append(hosts, a)
		}
	}
	var (
		mu    sync.Mutex
		found []Found
		wg    sync.WaitGroup
		sem   = make(chan struct{}, 96)
	)
	for _, h := range hosts {
		wg.Add(1)
		sem <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()
			hctx, cancel := context.WithTimeout(ctx, perHost)
			defer cancel()
			rc, err := FetchConfig(hctx, ip)
			if err != nil {
				return
			}
			mu.Lock()
			found = append(found, Found{IP: ip, Config: rc})
			mu.Unlock()
		}(h.String())
	}
	wg.Wait()
	sort.Slice(found, func(i, j int) bool {
		a, _ := netip.ParseAddr(found[i].IP)
		b, _ := netip.ParseAddr(found[j].IP)
		return a.Less(b)
	})
	return found, ctx.Err()
}

// ImportOutputs returns a copy of cfg in which the controller at ip gets one
// route per enabled firmware output (pixel count and universe exactly as the
// controller expects them). Existing routes of that IP are replaced and keep
// their place in the input frame; a new controller is appended after the
// last used pixel. The firmware reorders colors itself, so routes send RGB.
func ImportOutputs(cfg cfgpkg.Config, ip string, rc RemoteConfig, name string) (cfgpkg.Config, error) {
	if !ValidIP(ip) {
		return cfg, fmt.Errorf("ungueltige IP %q", ip)
	}
	var outs []Output
	for _, o := range rc.Outputs {
		if o.Enabled && o.PixelCount > 0 {
			outs = append(outs, o)
		}
	}
	if len(outs) == 0 {
		return cfg, fmt.Errorf("%s hat keine aktiven Ausgaenge", ip)
	}
	sort.Slice(outs, func(i, j int) bool { return outs[i].StartUniverse < outs[j].StartUniverse })

	out := cfg
	out.Controllers = append([]cfgpkg.Controller(nil), cfg.Controllers...)
	out.Routes = nil

	// pixel position: where this controller's routes were, else after the end
	start, haveOld, end := 0, false, 0
	for _, r := range cfg.Routes {
		if r.TargetIP == ip {
			if !haveOld || r.PixelStart < start {
				start = r.PixelStart
			}
			haveOld = true
			continue
		}
		out.Routes = append(out.Routes, r)
		end = max(end, r.PixelStart+r.PixelCount)
	}
	if !haveOld {
		start = end
	}

	ctrlIdx := -1
	for i, c := range out.Controllers {
		if c.TargetIP == ip {
			ctrlIdx = i
		}
	}
	if ctrlIdx < 0 {
		if name == "" {
			name = controllerName(rc, ip)
		}
		out.Controllers = append(out.Controllers, cfgpkg.Controller{Name: name, TargetIP: ip})
		ctrlIdx = len(out.Controllers) - 1
	}
	ctrl := &out.Controllers[ctrlIdx]
	if rc.TargetFps > 0 && rc.TargetFps <= 240 {
		ctrl.FPSTarget = rc.TargetFps
	}
	on := true
	ctrl.Enabled = &on

	pos := start
	for _, o := range outs {
		out.Routes = append(out.Routes, cfgpkg.Route{
			Name:          fmt.Sprintf("%s_OUT%d_%s", ctrl.Name, o.ID, sanitize(o.Type)),
			Enabled:       true,
			TargetIP:      ip,
			PhysicalPort:  o.ID + 1,
			PixelStart:    pos,
			PixelCount:    o.PixelCount,
			UniverseStart: o.StartUniverse,
			ColorOrder:    "RGB",
		})
		pos += o.PixelCount
	}
	// make room in the input frame, also for routes that now overlap later ones
	need := 0
	for _, r := range out.Routes {
		need = max(need, r.PixelStart+r.PixelCount)
	}
	if out.Input.PixelCount < need {
		out.Input.PixelCount = need
	}
	if out.Version < 2 {
		out.Version = 2
	}
	return out, nil
}

func controllerName(rc RemoteConfig, ip string) string {
	a, _ := netip.ParseAddr(ip)
	base := "CTRL"
	if p := strings.ToUpper(rc.HardwareProfile); strings.HasPrefix(p, "ESP32") {
		base = "ESP32"
	} else if strings.Contains(p, "TEENSY") {
		base = "TEENSY"
	}
	return fmt.Sprintf("%s_%d", base, a.As4()[3])
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "OUT"
	}
	return b.String()
}
