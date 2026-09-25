package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
)

// Config is the router configuration.
//
// Version 1 files (routes only) remain valid unchanged. Version 2 adds an
// optional controller table (per-controller name / FPS / enable) and input
// staleness handling. A version 1 file may also use the new optional fields;
// the version number only gates what is required, not what is allowed.
type Config struct {
	Version     int          `json:"version"`
	Input       InputConfig  `json:"input"`
	ArtNet      ArtNet       `json:"artnet"`
	Controllers []Controller `json:"controllers,omitempty"`
	Routes      []Route      `json:"routes"`
}

type InputConfig struct {
	PixelCount int `json:"pixel_count"`
	FPSTarget  int `json:"fps_target"`
	// StaleTimeoutMS: when > 0 and no new live frame has arrived for this long,
	// OnStale decides what the controllers transmit. 0 disables the check.
	StaleTimeoutMS int `json:"stale_timeout_ms,omitempty"`
	// OnStale: "hold" (default, keep sending the last frame), "blackout"
	// (send all-black frames) or "stop" (stop transmitting until input resumes).
	OnStale string `json:"on_stale,omitempty"`
}

type ArtNet struct {
	UDPPort int  `json:"udp_port"`
	Unicast bool `json:"unicast"`
}

// Controller groups all routes that share one target IP. Frame completeness,
// the Art-Net sequence number and the output FPS are owned per controller.
type Controller struct {
	Name      string `json:"name"`
	TargetIP  string `json:"target_ip"`
	Enabled   *bool  `json:"enabled,omitempty"`
	FPSTarget int    `json:"fps_target,omitempty"`
}

func (c Controller) IsEnabled() bool { return c.Enabled == nil || *c.Enabled }

type Route struct {
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	TargetIP      string `json:"target_ip"`
	PhysicalPort  int    `json:"physical_port,omitempty"`
	PixelStart    int    `json:"pixel_start"`
	PixelCount    int    `json:"pixel_count"`
	UniverseStart int    `json:"universe_start"`
	ColorOrder    string `json:"color_order"`
}

// Universes returns the number of 510-channel universes the route occupies.
func (r Route) Universes() int { return (r.PixelCount + 169) / 170 }

const (
	OnStaleHold     = "hold"
	OnStaleBlackout = "blackout"
	OnStaleStop     = "stop"
)

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Parse(b)
}

func Parse(b []byte) (Config, error) {
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	c.ApplyDefaults()
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c *Config) ApplyDefaults() {
	if c.ArtNet.UDPPort == 0 {
		c.ArtNet.UDPPort = 6454
	}
	if c.Input.FPSTarget == 0 {
		c.Input.FPSTarget = 60
	}
	c.Input.OnStale = strings.ToLower(strings.TrimSpace(c.Input.OnStale))
	if c.Input.OnStale == "" {
		c.Input.OnStale = OnStaleHold
	}
	for i := range c.Routes {
		if c.Routes[i].ColorOrder == "" {
			c.Routes[i].ColorOrder = "RGB"
		}
		c.Routes[i].ColorOrder = strings.ToUpper(c.Routes[i].ColorOrder)
	}
	for i := range c.Controllers {
		if c.Controllers[i].FPSTarget == 0 {
			c.Controllers[i].FPSTarget = c.Input.FPSTarget
		}
	}
}

func (c Config) Validate() error {
	if c.Version != 1 && c.Version != 2 {
		return fmt.Errorf("unsupported config version %d (expected 1 or 2)", c.Version)
	}
	if c.Input.PixelCount <= 0 {
		return fmt.Errorf("input.pixel_count must be > 0")
	}
	if c.Input.FPSTarget < 1 || c.Input.FPSTarget > 240 {
		return fmt.Errorf("input.fps_target must be 1..240")
	}
	if c.Input.StaleTimeoutMS < 0 {
		return fmt.Errorf("input.stale_timeout_ms must be >= 0")
	}
	switch c.Input.OnStale {
	case "", OnStaleHold, OnStaleBlackout, OnStaleStop:
	default:
		return fmt.Errorf("input.on_stale must be hold|blackout|stop, got %q", c.Input.OnStale)
	}
	if c.ArtNet.UDPPort < 1 || c.ArtNet.UDPPort > 65535 {
		return fmt.Errorf("artnet.udp_port must be 1..65535")
	}

	ctrlByIP := map[string]Controller{}
	names := map[string]bool{}
	for i, ct := range c.Controllers {
		where := fmt.Sprintf("controllers[%d] (%q)", i, ct.Name)
		if strings.TrimSpace(ct.Name) == "" {
			return fmt.Errorf("%s: name must not be empty", where)
		}
		if names[ct.Name] {
			return fmt.Errorf("%s: duplicate controller name", where)
		}
		names[ct.Name] = true
		if ip := net.ParseIP(ct.TargetIP); ip == nil || ip.To4() == nil {
			return fmt.Errorf("%s: target_ip must be an IPv4 address", where)
		}
		if _, dup := ctrlByIP[ct.TargetIP]; dup {
			return fmt.Errorf("%s: duplicate controller target_ip %s", where, ct.TargetIP)
		}
		fps := ct.FPSTarget
		if fps == 0 {
			fps = c.Input.FPSTarget
		}
		if fps < 1 || fps > 240 {
			return fmt.Errorf("%s: fps_target must be 1..240", where)
		}
		ctrlByIP[ct.TargetIP] = ct
	}

	type span struct {
		start, end int
		name       string
	}
	spansByIP := map[string][]span{}
	routeNames := map[string]bool{}

	for i, r := range c.Routes {
		if !r.Enabled {
			continue
		}
		where := fmt.Sprintf("routes[%d] (%q)", i, r.Name)
		if strings.TrimSpace(r.Name) == "" {
			return fmt.Errorf("%s: name must not be empty", where)
		}
		if routeNames[r.Name] {
			return fmt.Errorf("%s: duplicate route name", where)
		}
		routeNames[r.Name] = true
		ip := net.ParseIP(r.TargetIP)
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("%s: target_ip must be an IPv4 address", where)
		}
		if len(c.Controllers) > 0 {
			if _, ok := ctrlByIP[r.TargetIP]; !ok {
				return fmt.Errorf("%s: target_ip %s has no entry in controllers[]", where, r.TargetIP)
			}
		}
		if r.PixelStart < 0 || r.PixelCount <= 0 {
			return fmt.Errorf("%s: invalid pixel_start/pixel_count", where)
		}
		if r.PixelStart+r.PixelCount > c.Input.PixelCount {
			return fmt.Errorf("%s: pixel range %d..%d exceeds input pixel_count %d", where, r.PixelStart, r.PixelStart+r.PixelCount-1, c.Input.PixelCount)
		}
		if r.UniverseStart < 0 || r.UniverseStart > 32767 {
			return fmt.Errorf("%s: universe_start must be 0..32767", where)
		}
		universes := r.Universes()
		if r.UniverseStart+universes-1 > 32767 {
			return fmt.Errorf("%s: universe range exceeds Art-Net 15-bit port-address range", where)
		}
		if !validOrder(r.ColorOrder) {
			return fmt.Errorf("%s: unsupported color_order %q", where, r.ColorOrder)
		}
		spansByIP[r.TargetIP] = append(spansByIP[r.TargetIP], span{r.UniverseStart, r.UniverseStart + universes - 1, r.Name})
	}

	for ip, spans := range spansByIP {
		sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
		for i := 1; i < len(spans); i++ {
			if spans[i].start <= spans[i-1].end {
				return fmt.Errorf("routes %q and %q overlap on %s universes %d..%d", spans[i-1].name, spans[i].name, ip, spans[i].start, min(spans[i-1].end, spans[i].end))
			}
		}
	}
	return nil
}

// Warnings reports configurations that are legal but usually unintended,
// e.g. two enabled routes reading the same logical pixels (mirroring) or a
// declared controller without any enabled route.
func (c Config) Warnings() []string {
	var out []string
	type span struct {
		start, end int
		name       string
	}
	var spans []span
	used := map[string]bool{}
	for _, r := range c.Routes {
		if !r.Enabled {
			continue
		}
		used[r.TargetIP] = true
		spans = append(spans, span{r.PixelStart, r.PixelStart + r.PixelCount - 1, r.Name})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	for i := 1; i < len(spans); i++ {
		for j := 0; j < i; j++ {
			if spans[i].start <= spans[j].end {
				out = append(out, fmt.Sprintf("routes %q and %q read overlapping logical pixels %d..%d", spans[j].name, spans[i].name, spans[i].start, min(spans[i].end, spans[j].end)))
			}
		}
	}
	for _, ct := range c.Controllers {
		if ct.IsEnabled() && !used[ct.TargetIP] {
			out = append(out, fmt.Sprintf("controller %q (%s) has no enabled routes", ct.Name, ct.TargetIP))
		}
	}
	return out
}

// ControllerFor returns the controller entry for an IP, or a synthesized
// default (name = IP, fps = input.fps_target) when none is declared.
func (c Config) ControllerFor(ip string) Controller {
	for _, ct := range c.Controllers {
		if ct.TargetIP == ip {
			if ct.FPSTarget == 0 {
				ct.FPSTarget = c.Input.FPSTarget
			}
			return ct
		}
	}
	return Controller{Name: ip, TargetIP: ip, FPSTarget: c.Input.FPSTarget}
}

func validOrder(s string) bool {
	switch s {
	case "RGB", "RBG", "GRB", "GBR", "BRG", "BGR":
		return true
	default:
		return false
	}
}
