package pattern

import (
	"fmt"
	"strings"
)

const side = 8

// Fill writes a logical x-fastest 8x8x8 RGB frame.
func Fill(dst []byte, name string, frame uint64) error {
	if len(dst) != side*side*side*3 {
		return fmt.Errorf("frame length %d, expected %d", len(dst), side*side*side*3)
	}
	clear(dst)
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "xyz", "gradient":
		fillXYZ(dst)
	case "axes":
		fillAxes(dst)
	case "voxel", "chase":
		fillVoxel(dst, int(frame%512), 255, 255, 255)
	case "layers":
		fillLayers(dst, int(frame%8))
	case "corners":
		fillCorners(dst)
	default:
		return fmt.Errorf("unknown pattern %q (xyz|axes|voxel|layers|corners)", name)
	}
	return nil
}

func logicalIndex(x, y, z int) int { return x + y*side + z*side*side }
func set(dst []byte, x, y, z int, r, g, b byte) {
	i := logicalIndex(x, y, z) * 3
	dst[i], dst[i+1], dst[i+2] = r, g, b
}
func fillXYZ(dst []byte) {
	for z := 0; z < side; z++ {
		for y := 0; y < side; y++ {
			for x := 0; x < side; x++ {
				set(dst, x, y, z, byte(x*255/7), byte(y*255/7), byte(z*255/7))
			}
		}
	}
}
func fillAxes(dst []byte) {
	for x := 0; x < side; x++ {
		set(dst, x, 0, 0, 255, 0, 0)
	}
	for y := 0; y < side; y++ {
		set(dst, 0, y, 0, 0, 255, 0)
	}
	for z := 0; z < side; z++ {
		set(dst, 0, 0, z, 0, 0, 255)
	}
	set(dst, 0, 0, 0, 255, 255, 255)
}
func fillVoxel(dst []byte, index int, r, g, b byte) {
	i := index * 3
	dst[i], dst[i+1], dst[i+2] = r, g, b
}
func fillLayers(dst []byte, active int) {
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			set(dst, x, y, active, 255, 255, 255)
		}
	}
}
func fillCorners(dst []byte) {
	set(dst, 0, 0, 0, 255, 255, 255)
	set(dst, 7, 0, 0, 255, 0, 0)
	set(dst, 0, 7, 0, 0, 255, 0)
	set(dst, 0, 0, 7, 0, 0, 255)
	set(dst, 7, 7, 0, 255, 255, 0)
	set(dst, 7, 0, 7, 255, 0, 255)
	set(dst, 0, 7, 7, 0, 255, 255)
	set(dst, 7, 7, 7, 255, 255, 255)
}
