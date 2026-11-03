package scene

import (
	"fmt"
	"math"
)

// Coverage is sampled at the globally anchored lattice (0.25+0.5*i,
// 0.25+0.5*j), with each sample representing 0.25 square pixels. This keeps
// pair intersections, containment, and occlusion consistent across calls.
// Occlusion is the union of all higher-Z filled sets, never the sum of pair
// intersections. Bounding-box culling does not change shape membership.
const sampleArea = 0.25
const maxSampleWork = 32_000_000

func lattice(b bounds) (x0, y0, x1, y1 int) {
	return int(math.Ceil(2*b.minX - .5)), int(math.Ceil(2*b.minY - .5)), int(math.Floor(2*b.maxX - .5)), int(math.Floor(2*b.maxY - .5))
}
func sampleCount(b bounds) float64 {
	x0, y0, x1, y1 := lattice(b)
	if x1 < x0 || y1 < y0 {
		return 0
	}
	return float64(x1-x0+1) * float64(y1-y0+1)
}

func prepare(s *Scene) ([]geometry, error) {
	if s == nil {
		return nil, fmt.Errorf("scene: nil scene")
	}
	if s.Width < 1 || s.Height < 1 || s.Width > 2048 || s.Height > 2048 || s.Width*s.Height > 4_000_000 {
		return nil, fmt.Errorf("scene: canvas dimensions outside safe limits")
	}
	if len(s.Shapes) == 0 || len(s.Shapes) > 128 {
		return nil, fmt.Errorf("scene: expected 1 to 128 shapes")
	}
	ids, zs := make(map[int]bool, len(s.Shapes)), make(map[int]bool, len(s.Shapes))
	gs := make([]geometry, len(s.Shapes))
	for i, shape := range s.Shapes {
		g, ok := shapeGeometry(shape)
		if !ok {
			return nil, fmt.Errorf("scene: shape %d has invalid geometry", shape.ID)
		}
		if shape.Width < 12 || shape.Height < 12 {
			return nil, fmt.Errorf("scene: shape %d is too small to recognize", shape.ID)
		}
		if math.Max(shape.Width, shape.Height)/math.Min(shape.Width, shape.Height) > 3 {
			return nil, fmt.Errorf("scene: shape %d is too elongated to recognize", shape.ID)
		}
		if g.box.minX < 0 || g.box.minY < 0 || g.box.maxX > float64(s.Width) || g.box.maxY > float64(s.Height) {
			return nil, fmt.Errorf("scene: shape %d is outside canvas", shape.ID)
		}
		if ids[shape.ID] || zs[shape.Z] {
			return nil, fmt.Errorf("scene: duplicate shape ID or Z")
		}
		if shape.Fill.RGBA.A != 255 {
			return nil, fmt.Errorf("scene: shape %d fill must be opaque", shape.ID)
		}
		ids[shape.ID] = true
		zs[shape.Z] = true
		gs[i] = g
	}
	work := 0.0
	for i := range gs {
		candidates := 1
		for j := range gs {
			if gs[j].shape.Z > gs[i].shape.Z && gs[i].box.intersects(gs[j].box) {
				candidates++
			}
		}
		work += sampleCount(gs[i].box) * float64(candidates)
		if work > maxSampleWork {
			return nil, fmt.Errorf("scene: geometry exceeds sampling work limit")
		}
	}
	return gs, nil
}

func measure(gs []geometry) []Measurement {
	result := make([]Measurement, len(gs))
	candidates := make([]int, 0, len(gs))
	for i := range gs {
		g := &gs[i]
		candidates = candidates[:0]
		for j := range gs {
			if gs[j].shape.Z > g.shape.Z && gs[j].box.intersects(g.box) {
				candidates = append(candidates, j)
			}
		}
		x0, y0, x1, y1 := lattice(g.box)
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				p := Point{float64(x)*.5 + .25, float64(y)*.5 + .25}
				if !g.contains(p) {
					continue
				}
				result[i].Area += sampleArea
				hidden := false
				for _, j := range candidates {
					if gs[j].contains(p) {
						hidden = true
						break
					}
				}
				if !hidden {
					result[i].Visible += sampleArea
				}
			}
		}
	}
	return result
}

// Analyze updates VisibleArea and Occlusion using actual-shape lattice coverage.
// Occlusion is hidden sampled area divided by total sampled area, not analytic
// area. Structural validation and work limits happen before any mutation, so
// errors leave the scene unchanged. Analyze permits fully hidden shapes; use
// Validate to require recognizable visibility. Callers must exclusively own s.
func Analyze(s *Scene) error {
	gs, err := prepare(s)
	if err != nil {
		return err
	}
	measurements := measure(gs)
	for i, m := range measurements {
		s.Shapes[i].VisibleArea = m.Visible
		s.Shapes[i].Occlusion = 0
		if m.Area > 0 {
			s.Shapes[i].Occlusion = 1 - m.Visible/m.Area
		}
	}
	return nil
}

// Measure returns total and topmost-visible lattice areas in scene shape order.
// It applies Analyze's structural and work-limit checks without mutating s or
// trusting cached Shape metrics. Results are independent fresh slices.
func Measure(s *Scene) ([]Measurement, error) {
	gs, err := prepare(s)
	if err != nil {
		return nil, err
	}
	return measure(gs), nil
}

// Validate checks finite supported geometry, opaque fills, unique IDs and Z,
// actual rotated bounds, minimum 12-pixel diameters, maximum 3:1 aspect ratio,
// and at least 70 percent visible sampled area for every shape. It ignores
// cached metrics and never mutates s. Canvases are bounded to 2048 per axis and
// four million pixels, scenes to 128 shapes, and sampling work to 32 million
// candidate checks; malformed inputs cannot trigger unbounded raster work.
func Validate(s *Scene) error {
	gs, err := prepare(s)
	if err != nil {
		return err
	}
	for i, m := range measure(gs) {
		if m.Area == 0 || m.Visible/m.Area < .70 {
			return fmt.Errorf("scene: shape %d is insufficiently visible", s.Shapes[i].ID)
		}
	}
	return nil
}

func pairCoverage(a, b *geometry) (aa, bb, intersection float64, ok bool) {
	if sampleCount(a.box)+sampleCount(b.box) > maxSampleWork {
		return 0, 0, 0, false
	}
	x0, y0, x1, y1 := lattice(a.box)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			p := Point{float64(x)*.5 + .25, float64(y)*.5 + .25}
			if a.contains(p) {
				aa += sampleArea
				if b.contains(p) {
					intersection += sampleArea
				}
			}
		}
	}
	x0, y0, x1, y1 = lattice(b.box)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if b.contains(Point{float64(x)*.5 + .25, float64(y)*.5 + .25}) {
				bb += sampleArea
			}
		}
	}
	return aa, bb, intersection, aa > 0 && bb > 0
}

func greaterBand(value, low, high float64) (bool, bool) {
	if value > high {
		return true, false
	}
	if value < low {
		return false, false
	}
	return false, true
}

func alignedDirection(along, across float64) (match, ambiguous bool) {
	direction, directionAmbiguous := greaterBand(along, .45, .65)
	aligned, alignmentAmbiguous := greaterBand(-math.Abs(across), -.75, -.5)
	// A clearly wrong direction or separate row/column rules out the relation,
	// even when the other component is on its boundary.
	if !direction && !directionAmbiguous || !aligned && !alignmentAmbiguous {
		return false, false
	}
	return direction && aligned, directionAmbiguous || alignmentAmbiguous
}

// Relate evaluates a strict relation; ambiguous results must not be presented as
// false facts. Directional relations compare center displacement to mean actual
// axis extent: >0.65 is true, <0.45 false, and the closed interval is ambiguous.
// Left/Right also require the same row, Above/Below the same column: absolute
// perpendicular center displacement / mean actual perpendicular extent must be
// <0.5; >0.75 is a different row/column, and [0.5,0.75] is ambiguous.
// Near compares center distance / mean actual bounding diagonal: <0.85 is true,
// >1.05 false; Far uses >1.8 and <1.6. Larger/Smaller only compare matching kinds.
// Both unrotated dimensions must exceed 1.6x for true, or be below 1/1.6 for
// false; all other same-kind pairs are ambiguous. Different kinds are false.
// Self-relations remain false. Intersections use the half-pixel lattice:
// Overlapping requires >4% of the smaller shape; zero is false, (0,4%] ambiguous.
// Inside requires >=99.5% of the first shape inside the second; <=95% is false.
// Occluded requires the second shape to be above the first and cover >4% of
// the first; zero is false, (0,4%] ambiguous. Invalid/unknown inputs are ambiguous.
// The scene supplies context only; pair relations do not read cached metrics
// and are independent of unrelated shapes. No input is mutated.
func Relate(s *Scene, a, b Shape, r Relation) (match, ambiguous bool) {
	if s == nil {
		return false, true
	}
	ga, oka := shapeGeometry(a)
	gb, okb := shapeGeometry(b)
	if !oka || !okb {
		return false, true
	}
	if a.ID == b.ID {
		return false, false
	}
	switch r {
	case LeftOf, RightOf:
		d := b.Center.X - a.Center.X
		if r == RightOf {
			d = -d
		}
		return alignedDirection(
			d/((ga.box.maxX-ga.box.minX+gb.box.maxX-gb.box.minX)/2),
			(a.Center.Y-b.Center.Y)/((ga.box.maxY-ga.box.minY+gb.box.maxY-gb.box.minY)/2),
		)
	case Above, Below:
		d := b.Center.Y - a.Center.Y
		if r == Below {
			d = -d
		}
		return alignedDirection(
			d/((ga.box.maxY-ga.box.minY+gb.box.maxY-gb.box.minY)/2),
			(a.Center.X-b.Center.X)/((ga.box.maxX-ga.box.minX+gb.box.maxX-gb.box.minX)/2),
		)
	case Near, Far:
		distance := math.Hypot(a.Center.X-b.Center.X, a.Center.Y-b.Center.Y)
		scale := (math.Hypot(ga.box.maxX-ga.box.minX, ga.box.maxY-ga.box.minY) + math.Hypot(gb.box.maxX-gb.box.minX, gb.box.maxY-gb.box.minY)) / 2
		if r == Near {
			return greaterBand(-distance/scale, -1.05, -.85)
		}
		return greaterBand(distance/scale, 1.6, 1.8)
	case LargerThan, SmallerThan:
		if a.Kind != b.Kind {
			return false, false
		}
		if r == SmallerThan {
			a, b = b, a
		}
		width, height := a.Width/b.Width, a.Height/b.Height
		if width > 1.6 && height > 1.6 {
			return true, false
		}
		if width < 1/1.6 && height < 1/1.6 {
			return false, false
		}
		return false, true
	case Inside, Overlapping, Occluded:
		if r == Occluded && b.Z <= a.Z {
			return false, false
		}
		if !ga.box.intersects(gb.box) {
			return false, false
		}
		aa, bb, overlap, ok := pairCoverage(&ga, &gb)
		if !ok {
			return false, true
		}
		if r == Inside {
			fraction := overlap / aa
			if fraction >= .995 {
				return true, false
			}
			if fraction <= .95 {
				return false, false
			}
			return false, true
		}
		denominator := aa
		if r == Overlapping {
			denominator = math.Min(aa, bb)
		}
		if overlap == 0 {
			return false, false
		}
		if overlap/denominator > .04 {
			return true, false
		}
		return false, true
	default:
		return false, true
	}
}
