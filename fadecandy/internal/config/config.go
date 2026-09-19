package config

import (
	"encoding/json"
	"fmt"
	"os"

	"pxlblz-fadecandy/internal/mapping"
)

type Config struct {
	Input struct {
		PixelCount int    `json:"pixel_count"`
		FPSTarget  int    `json:"fps_target"`
		WSListen   string `json:"ws_listen"`
		WSPath     string `json:"ws_path"`
	} `json:"input"`
	Fadecandy struct {
		Address string `json:"address"`
		Channel int    `json:"channel"`
	} `json:"fadecandy"`
	Mapping mapping.Spec `json:"mapping"`
}

func Load(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if c.Input.PixelCount <= 0 {
		return c, fmt.Errorf("input.pixel_count must be > 0")
	}
	if c.Input.FPSTarget == 0 {
		c.Input.FPSTarget = 60
	}
	if c.Input.FPSTarget < 1 || c.Input.FPSTarget > 240 {
		return c, fmt.Errorf("input.fps_target must be 1..240")
	}
	if c.Fadecandy.Channel < 0 || c.Fadecandy.Channel > 255 {
		return c, fmt.Errorf("fadecandy.channel must be 0..255")
	}
	if err = c.Mapping.Validate(); err != nil {
		return c, fmt.Errorf("mapping: %w", err)
	}
	if c.Mapping.PixelCount() != c.Input.PixelCount {
		return c, fmt.Errorf("mapping pixel count %d != input pixel count %d", c.Mapping.PixelCount(), c.Input.PixelCount)
	}
	return c, nil
}
