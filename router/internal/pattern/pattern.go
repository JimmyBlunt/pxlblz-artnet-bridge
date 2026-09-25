package pattern

import (
	"fmt"
	"math"
	"strings"

	cfgpkg "pxlblz-router/internal/config"
)

func Fill(frame []byte, pixelCount int, name string, frameNo uint64) error {
	if len(frame) != pixelCount*3 {
		return fmt.Errorf("frame has %d bytes, expected %d", len(frame), pixelCount*3)
	}
	name = strings.ToLower(name)
	switch name {
	case "red", "solid-red":
		solid(frame, 255, 0, 0)
	case "green", "solid-green":
		solid(frame, 0, 255, 0)
	case "blue", "solid-blue":
		solid(frame, 0, 0, 255)
	case "white", "solid-white":
		solid(frame, 255, 255, 255)
	case "black", "off":
		solid(frame, 0, 0, 0)
	case "chase":
		solid(frame, 0, 0, 0)
		if pixelCount > 0 {
			p := int(frameNo % uint64(pixelCount))
			frame[p*3], frame[p*3+1], frame[p*3+2] = 255, 255, 255
		}
	case "gradient", "rainbow":
		phase := float64(frameNo%360) / 360.0
		for i := 0; i < pixelCount; i++ {
			h := math.Mod(float64(i)/math.Max(1, float64(pixelCount))+phase, 1)
			r, g, b := hsv(h, 1, 1)
			frame[i*3], frame[i*3+1], frame[i*3+2] = r, g, b
		}
	case "index":
		for i := 0; i < pixelCount; i++ {
			frame[i*3] = byte(i & 0xff)
			frame[i*3+1] = byte((i >> 8) & 0xff)
			frame[i*3+2] = byte((i * 37) & 0xff)
		}
	case "port-id":
		return fmt.Errorf("pattern %q requires route information; use FillPortID", name)
	default:
		return fmt.Errorf("unknown pattern %q", name)
	}
	return nil
}

func FillPortID(frame []byte, pixelCount int, routes []cfgpkg.Route, frameNo uint64) error {
	if len(frame) != pixelCount*3 {
		return fmt.Errorf("frame has %d bytes, expected %d", len(frame), pixelCount*3)
	}
	solid(frame, 0, 0, 0)

	const idLevel byte = 56
	const markerLevel byte = 192
	const idPixels = 8
	const diagnosticSpan = 64
	const framesPerStep = 2

	palette := [][3]byte{
		{1, 0, 0},
		{0, 1, 0},
		{0, 0, 1},
		{0, 1, 1},
		{1, 0, 1},
		{1, 1, 0},
		{1, 1, 1},
	}

	for routeIndex, route := range routes {
		if !route.Enabled || route.PixelCount <= 0 {
			continue
		}
		if route.PixelStart < 0 || route.PixelStart+route.PixelCount > pixelCount {
			return fmt.Errorf("route %q pixel range %d..%d outside frame 0..%d",
				route.Name, route.PixelStart, route.PixelStart+route.PixelCount-1, pixelCount-1)
		}

		portIndex := route.PhysicalPort - 1
		if portIndex < 0 {
			portIndex = routeIndex
		}
		base := palette[portIndex%len(palette)]

		barCount := idPixels
		if route.PixelCount < barCount {
			barCount = route.PixelCount
		}
		for i := 0; i < barCount; i++ {
			setScaledPixel(frame, route.PixelStart+i, base, idLevel)
		}

		span := diagnosticSpan
		if route.PixelCount < span {
			span = route.PixelCount
		}
		if span <= 0 {
			continue
		}

		markerStart := idPixels
		if markerStart >= span {
			markerStart = 0
		}
		markerRange := span - markerStart
		if markerRange <= 0 {
			markerRange = 1
		}
		markerOffset := markerStart + int((frameNo/framesPerStep)%uint64(markerRange))
		setScaledPixel(frame, route.PixelStart+markerOffset, base, markerLevel)
	}
	return nil
}

func setScaledPixel(frame []byte, pixel int, rgb [3]byte, level byte) {
	i := pixel * 3
	frame[i] = rgb[0] * level
	frame[i+1] = rgb[1] * level
	frame[i+2] = rgb[2] * level
}

func solid(frame []byte, r, g, b byte) {
	for i := 0; i < len(frame); i += 3 {
		frame[i], frame[i+1], frame[i+2] = r, g, b
	}
}

func hsv(h, s, v float64) (byte, byte, byte) {
	h = math.Mod(h, 1)
	if h < 0 {
		h += 1
	}
	i := int(h * 6)
	f := h*6 - float64(i)
	p := v * (1 - s)
	q := v * (1 - f*s)
	t := v * (1 - (1-f)*s)
	var r, g, b float64
	switch i % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	case 5:
		r, g, b = v, p, q
	}
	return byte(r*255 + 0.5), byte(g*255 + 0.5), byte(b*255 + 0.5)
}

// FillPanelWalk is the G9 visual-verification pattern. It needs no PXLBLZ.
//
//   - Every enabled route is dimly lit in its port colour (P1 red, P2 green,
//     P3 blue, P4 cyan, P5 magenta, P6 yellow, P7 white) so all panels are
//     visibly alive and identifiable.
//   - The first 3 pixels of every route are brighter in the port colour:
//     this marks the electrical START of the lane, i.e. its direction.
//   - One white "head" with a fading port-coloured tail walks through all
//     enabled routes in config order (P1 → P2 → … → P7), advancing
//     `speed` logical pixels per output frame. Watching the head reveals
//     panel order, lane direction and the P6 → P7 hand-over on Panel 6.
//
// Brightness is kept low on purpose (background 6/255, head 160/255) so the
// pattern is safe to run on the full installation power budget.
func FillPanelWalk(frame []byte, pixelCount int, routes []cfgpkg.Route, frameNo uint64, speed int) error {
	if len(frame) != pixelCount*3 {
		return fmt.Errorf("frame has %d bytes, expected %d", len(frame), pixelCount*3)
	}
	if speed < 1 {
		speed = 1
	}
	solid(frame, 0, 0, 0)

	const bgLevel byte = 6
	const startLevel byte = 56
	const startPixels = 3
	const headLevel byte = 160
	const tail = 24

	palette := portPalette()
	total := 0
	for _, r := range routes {
		if !r.Enabled || r.PixelCount <= 0 {
			continue
		}
		if r.PixelStart < 0 || r.PixelStart+r.PixelCount > pixelCount {
			return fmt.Errorf("route %q pixel range outside frame", r.Name)
		}
		total += r.PixelCount
	}
	if total == 0 {
		return nil
	}
	head := int((frameNo * uint64(speed)) % uint64(total))

	offset := 0
	for idx, r := range routes {
		if !r.Enabled || r.PixelCount <= 0 {
			continue
		}
		pi := r.PhysicalPort - 1
		if pi < 0 {
			pi = idx
		}
		base := palette[pi%len(palette)]
		for i := 0; i < r.PixelCount; i++ {
			lvl := bgLevel
			if i < startPixels {
				lvl = startLevel
			}
			setScaledPixel(frame, r.PixelStart+i, base, lvl)

			// distance behind the head along the concatenated walk
			d := head - (offset + i)
			if d < 0 {
				d += total
			}
			switch {
			case d == 0:
				p := (r.PixelStart + i) * 3
				frame[p], frame[p+1], frame[p+2] = headLevel, headLevel, headLevel
			case d < tail:
				l := byte(int(headLevel) * (tail - d) / tail)
				if l > lvl {
					setScaledPixel(frame, r.PixelStart+i, base, l)
				}
			}
		}
		offset += r.PixelCount
	}
	return nil
}

func portPalette() [][3]byte {
	return [][3]byte{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {0, 1, 1}, {1, 0, 1}, {1, 1, 0}, {1, 1, 1}}
}
