package mapping

import (
	"fmt"
)

type Axis string

const (
	X Axis = "x"
	Y Axis = "y"
	Z Axis = "z"
)

type Flip struct {
	X bool `json:"x"`
	Y bool `json:"y"`
	Z bool `json:"z"`
}

type Spec struct {
	Dimensions  [3]int  `json:"dimensions"`
	InputOrder  [3]Axis `json:"input_order"`  // fastest -> slowest
	OutputOrder [3]Axis `json:"output_order"` // fastest -> slowest
	Flip        Flip    `json:"flip"`
	LUT         []int   `json:"lut,omitempty"`
}

func (s Spec) PixelCount() int {
	return s.Dimensions[0] * s.Dimensions[1] * s.Dimensions[2]
}

func (s Spec) Validate() error {
	if s.Dimensions[0] <= 0 || s.Dimensions[1] <= 0 || s.Dimensions[2] <= 0 {
		return fmt.Errorf("all dimensions must be > 0")
	}
	if err := validateOrder(s.InputOrder); err != nil {
		return fmt.Errorf("input_order: %w", err)
	}
	if err := validateOrder(s.OutputOrder); err != nil {
		return fmt.Errorf("output_order: %w", err)
	}
	if len(s.LUT) != 0 {
		if len(s.LUT) != s.PixelCount() {
			return fmt.Errorf("lut has %d entries, expected %d", len(s.LUT), s.PixelCount())
		}
		seen := make([]bool, s.PixelCount())
		for i, v := range s.LUT {
			if v < 0 || v >= s.PixelCount() {
				return fmt.Errorf("lut[%d]=%d out of range", i, v)
			}
			if seen[v] {
				return fmt.Errorf("lut contains duplicate physical index %d", v)
			}
			seen[v] = true
		}
	}
	return nil
}

func validateOrder(o [3]Axis) error {
	seen := map[Axis]bool{}
	for _, a := range o {
		if a != X && a != Y && a != Z {
			return fmt.Errorf("invalid axis %q", a)
		}
		if seen[a] {
			return fmt.Errorf("duplicate axis %q", a)
		}
		seen[a] = true
	}
	return nil
}

type Mapper struct {
	spec Spec
	lut  []int // logical pixel -> physical pixel
}

func New(spec Spec) (*Mapper, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	m := &Mapper{spec: spec, lut: make([]int, spec.PixelCount())}
	if len(spec.LUT) != 0 {
		copy(m.lut, spec.LUT)
		return m, nil
	}
	dims := map[Axis]int{X: spec.Dimensions[0], Y: spec.Dimensions[1], Z: spec.Dimensions[2]}
	for logical := 0; logical < spec.PixelCount(); logical++ {
		c := decodeIndex(logical, spec.InputOrder, dims)
		if spec.Flip.X {
			c[X] = dims[X] - 1 - c[X]
		}
		if spec.Flip.Y {
			c[Y] = dims[Y] - 1 - c[Y]
		}
		if spec.Flip.Z {
			c[Z] = dims[Z] - 1 - c[Z]
		}
		m.lut[logical] = encodeIndex(c, spec.OutputOrder, dims)
	}
	return m, nil
}

func decodeIndex(index int, order [3]Axis, dims map[Axis]int) map[Axis]int {
	c := map[Axis]int{X: 0, Y: 0, Z: 0}
	n := index
	for _, a := range order {
		d := dims[a]
		c[a] = n % d
		n /= d
	}
	return c
}

func encodeIndex(c map[Axis]int, order [3]Axis, dims map[Axis]int) int {
	stride := 1
	out := 0
	for _, a := range order {
		out += c[a] * stride
		stride *= dims[a]
	}
	return out
}

// MapRGB converts a logical RGB frame into physical output order.
func (m *Mapper) MapRGB(src []byte, dst []byte) error {
	expected := m.spec.PixelCount() * 3
	if len(src) != expected {
		return fmt.Errorf("source frame length %d, expected %d", len(src), expected)
	}
	if len(dst) != expected {
		return fmt.Errorf("destination frame length %d, expected %d", len(dst), expected)
	}
	for logical, physical := range m.lut {
		si := logical * 3
		di := physical * 3
		dst[di] = src[si]
		dst[di+1] = src[si+1]
		dst[di+2] = src[si+2]
	}
	return nil
}

func (m *Mapper) PhysicalIndex(logical int) (int, error) {
	if logical < 0 || logical >= len(m.lut) {
		return 0, fmt.Errorf("logical index %d out of range", logical)
	}
	return m.lut[logical], nil
}

func (m *Mapper) LUT() []int {
	out := make([]int, len(m.lut))
	copy(out, m.lut)
	return out
}
