package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
)

type Config struct {
	Version int         `json:"version"`
	Input   InputConfig `json:"input"`
	ArtNet  ArtNet      `json:"artnet"`
	Routes  []Route     `json:"routes"`
}

type InputConfig struct {
	PixelCount int `json:"pixel_count"`
	FPSTarget  int `json:"fps_target"`
}

type ArtNet struct {
	UDPPort int  `json:"udp_port"`
	Unicast bool `json:"unicast"`
}

type Route struct {
	Name          string   `json:"name"`
	Enabled       bool     `json:"enabled"`
	TargetIP      string   `json:"target_ip"`
	PhysicalPort  int      `json:"physical_port,omitempty"`
	PixelStart    int      `json:"pixel_start"`
	PixelCount    int      `json:"pixel_count"`
	UniverseStart int      `json:"universe_start"`
	ColorOrder    string   `json:"color_order"`
	Brightness    *float64 `json:"brightness,omitempty"`
}

func (r Route) BrightnessLevel() float64 {
	if r.Brightness == nil {
		return 1
	}
	return *r.Brightness
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	if c.ArtNet.UDPPort == 0 {
		c.ArtNet.UDPPort = 6454
	}
	if c.Input.FPSTarget == 0 {
		c.Input.FPSTarget = 60
	}
	for i := range c.Routes {
		if c.Routes[i].ColorOrder == "" {
			c.Routes[i].ColorOrder = "RGB"
		}
		c.Routes[i].ColorOrder = strings.ToUpper(c.Routes[i].ColorOrder)
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version %d (expected 1)", c.Version)
	}
	if c.Input.PixelCount <= 0 {
		return fmt.Errorf("input.pixel_count must be > 0")
	}
	if c.Input.FPSTarget < 1 || c.Input.FPSTarget > 240 {
		return fmt.Errorf("input.fps_target must be 1..240")
	}
	if c.ArtNet.UDPPort < 1 || c.ArtNet.UDPPort > 65535 {
		return fmt.Errorf("artnet.udp_port must be 1..65535")
	}

	type span struct {
		start, end int
		name       string
	}
	spansByIP := map[string][]span{}

	for i, r := range c.Routes {
		if !r.Enabled {
			continue
		}
		where := fmt.Sprintf("routes[%d] (%q)", i, r.Name)
		if strings.TrimSpace(r.Name) == "" {
			return fmt.Errorf("%s: name must not be empty", where)
		}
		ip := net.ParseIP(r.TargetIP)
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("%s: target_ip must be an IPv4 address", where)
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
		universes := (r.PixelCount + 169) / 170
		if r.UniverseStart+universes-1 > 32767 {
			return fmt.Errorf("%s: universe range exceeds Art-Net 15-bit port-address range", where)
		}
		if !validOrder(r.ColorOrder) {
			return fmt.Errorf("%s: unsupported color_order %q", where, r.ColorOrder)
		}
		if level := r.BrightnessLevel(); level < 0 || level > 1 {
			return fmt.Errorf("%s: brightness must be 0..1", where)
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

func validOrder(s string) bool {
	switch s {
	case "RGB", "RBG", "GRB", "GBR", "BRG", "BGR":
		return true
	default:
		return false
	}
}
