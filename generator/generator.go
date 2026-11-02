// Package generator creates randomized structured scenes independently of questions.
package generator

import (
	"errors"
	"fmt"
	"image/color"
	"math"

	"github.com/mxmilu666/nyancaptcha/internal/random"
	"github.com/mxmilu666/nyancaptcha/scene"
)

// Config controls scene dimensions, diversity and intentional overlap probability.
type Config struct {
	Width, Height, MinShapes, MaxShapes int
	Kinds                               []scene.Kind
	Colors                              []scene.Color
	Overlap                             float64
}

// Generate lays out shapes with cryptographic randomness and bounded placement
// attempts. A jittered, randomly assigned grid supplies distractors throughout
// the canvas; some neighbors are pulled together to create controlled overlap.
func Generate(c Config) (*scene.Scene, error) {
	if c.Width < 240 || c.Width > 1200 || c.Height < 180 || c.Height > 900 || c.Width*c.Height > 720000 || c.MinShapes < 5 || c.MaxShapes < c.MinShapes || c.MaxShapes > 30 || len(c.Kinds) == 0 || len(c.Colors) == 0 || math.IsNaN(c.Overlap) || c.Overlap < 0 || c.Overlap > 1 {
		return nil, errors.New("generator: invalid configuration")
	}
	extra, err := random.Int(c.MaxShapes - c.MinShapes + 1)
	if err != nil {
		return nil, err
	}
	n := c.MinShapes + extra
	cols := int(math.Ceil(math.Sqrt(float64(n) * float64(c.Width) / float64(c.Height))))
	rows := (n + cols - 1) / cols
	cw, ch := float64(c.Width)/float64(cols), float64(c.Height)/float64(rows)
	base := math.Min(cw, ch) * 0.66
	cells := make([]int, cols*rows)
	for i := range cells {
		cells[i] = i
	}
	for i := len(cells) - 1; i > 0; i-- {
		j, e := random.Int(i + 1)
		if e != nil {
			return nil, e
		}
		cells[i], cells[j] = cells[j], cells[i]
	}
	s := &scene.Scene{Width: c.Width, Height: c.Height, Shapes: make([]scene.Shape, 0, n)}
	for i := range n {
		// Collect entropy up front and propagate failures rather than substituting defaults.
		var u [8]float64
		for j := range u {
			u[j], err = random.Float()
			if err != nil {
				return nil, err
			}
		}
		ki, e := random.Int(len(c.Kinds))
		if e != nil {
			return nil, e
		}
		ci, e := random.Int(len(c.Colors))
		if e != nil {
			return nil, e
		}
		w := base * (0.72 + 0.48*u[0])
		h := w
		if c.Kinds[ki] == scene.Ellipse || c.Kinds[ki] == scene.Rectangle {
			h = w * (0.65 + 0.2*u[1])
		}
		x := (float64(cells[i]%cols)+0.5)*cw + (u[2]-0.5)*cw*0.22
		y := (float64(cells[i]/cols)+0.5)*ch + (u[3]-0.5)*ch*0.22
		if i > 0 && u[4] < c.Overlap {
			nearest := 0
			distance := math.Inf(1)
			for j, a := range s.Shapes {
				d := math.Hypot(x-a.Center.X, y-a.Center.Y)
				if d < distance {
					nearest, distance = j, d
				}
			}
			a := s.Shapes[nearest]
			angle := math.Atan2(y-a.Center.Y, x-a.Center.X)
			separation := (w + a.Width) * (0.30 + 0.08*u[5])
			x, y = a.Center.X+math.Cos(angle)*separation, a.Center.Y+math.Sin(angle)*separation
		}
		radius := math.Hypot(w, h)/2 + 3
		x = math.Max(radius, math.Min(float64(c.Width)-radius, x))
		y = math.Max(radius, math.Min(float64(c.Height)-radius, y))
		s.Shapes = append(s.Shapes, scene.Shape{ID: i + 1, Kind: c.Kinds[ki], Center: scene.Point{X: x, Y: y}, Width: w, Height: h, Rotation: u[6] * 2 * math.Pi, Fill: c.Colors[ci], Border: color.RGBA{35, 40, 48, 255}, Z: i})
	}
	if err := scene.Analyze(s); err != nil {
		return nil, fmt.Errorf("generator: geometry: %w", err)
	}
	for _, a := range s.Shapes {
		if a.Occlusion > 0.28 {
			return nil, errors.New("generator: excessive occlusion")
		}
	}
	return s, nil
}
