package scene

import "math"

// Geometry uses closed filled sets: points on an edge belong to a shape.
// Width and Height are the diameters of the unrotated circumscribed ellipse;
// rectangles alone fill that ellipse's bounding rectangle. Regular polygons
// have a vertex pointing upward before rotation, with vertices equally spaced
// on the ellipse. Stars have five outer vertices and five alternating inner
// vertices at 0.45 of the outer radii. Rotation is clockwise in screen space.
const starInnerRadius = 0.45

type bounds struct{ minX, minY, maxX, maxY float64 }
type geometry struct {
	shape    Shape
	cos, sin float64
	vertices [10]Point
	count    int
	box      bounds
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func shapeGeometry(s Shape) (geometry, bool) {
	g := geometry{shape: s}
	if !finite(s.Center.X) || !finite(s.Center.Y) || !finite(s.Width) || !finite(s.Height) || !finite(s.Rotation) || s.Width <= 0 || s.Height <= 0 || s.Width > 1e6 || s.Height > 1e6 || math.Abs(s.Center.X) > 1e9 || math.Abs(s.Center.Y) > 1e9 {
		return g, false
	}
	g.sin, g.cos = math.Sincos(s.Rotation)
	switch s.Kind {
	case Circle:
		if math.Abs(s.Width-s.Height) > 1e-9*math.Max(s.Width, s.Height) {
			return g, false
		}
		fallthrough
	case Ellipse:
		rx, ry := s.Width/2, s.Height/2
		ex := math.Hypot(rx*g.cos, ry*g.sin)
		ey := math.Hypot(rx*g.sin, ry*g.cos)
		g.box = bounds{s.Center.X - ex, s.Center.Y - ey, s.Center.X + ex, s.Center.Y + ey}
		return g, true
	case Rectangle:
		g.count = 4
	case Triangle:
		g.count = 3
	case Pentagon:
		g.count = 5
	case Hexagon:
		g.count = 6
	case Star:
		g.count = 10
	default:
		return g, false
	}
	g.box = bounds{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for i := range g.count {
		var x, y float64
		if s.Kind == Rectangle {
			x = s.Width / 2
			y = s.Height / 2
			if i == 0 || i == 3 {
				x = -x
			}
			if i < 2 {
				y = -y
			}
		} else {
			angle := -math.Pi/2 + 2*math.Pi*float64(i)/float64(g.count)
			r := 1.0
			if s.Kind == Star && i%2 == 1 {
				r = starInnerRadius
			}
			x = r * s.Width / 2 * math.Cos(angle)
			y = r * s.Height / 2 * math.Sin(angle)
		}
		p := Point{s.Center.X + x*g.cos - y*g.sin, s.Center.Y + x*g.sin + y*g.cos}
		g.vertices[i] = p
		g.box.minX = math.Min(g.box.minX, p.X)
		g.box.maxX = math.Max(g.box.maxX, p.X)
		g.box.minY = math.Min(g.box.minY, p.Y)
		g.box.maxY = math.Max(g.box.maxY, p.Y)
	}
	return g, true
}

func (b bounds) contains(p Point) bool {
	return p.X >= b.minX && p.X <= b.maxX && p.Y >= b.minY && p.Y <= b.maxY
}
func (b bounds) intersects(c bounds) bool {
	return b.minX <= c.maxX && c.minX <= b.maxX && b.minY <= c.maxY && c.minY <= b.maxY
}

func (g *geometry) contains(p Point) bool {
	if !g.box.contains(p) {
		return false
	}
	if g.count == 0 {
		dx, dy := p.X-g.shape.Center.X, p.Y-g.shape.Center.Y
		x := (dx*g.cos + dy*g.sin) / (g.shape.Width / 2)
		y := (-dx*g.sin + dy*g.cos) / (g.shape.Height / 2)
		return x*x+y*y <= 1+1e-12
	}
	inside := false
	for i, j := 0, g.count-1; i < g.count; j, i = i, i+1 {
		a, b := g.vertices[j], g.vertices[i]
		cross := (p.X-a.X)*(b.Y-a.Y) - (p.Y-a.Y)*(b.X-a.X)
		tolerance := 1e-10 * math.Max(1, math.Hypot(b.X-a.X, b.Y-a.Y))
		if math.Abs(cross) <= tolerance && p.X >= math.Min(a.X, b.X)-1e-10 && p.X <= math.Max(a.X, b.X)+1e-10 && p.Y >= math.Min(a.Y, b.Y)-1e-10 && p.Y <= math.Max(a.Y, b.Y)+1e-10 {
			return true
		}
		if (a.Y > p.Y) != (b.Y > p.Y) && p.X < (b.X-a.X)*(p.Y-a.Y)/(b.Y-a.Y)+a.X {
			inside = !inside
		}
	}
	return inside
}

// Contains reports membership in the actual closed, rotated shape, not its
// bounding box. Invalid shapes and nonfinite points return false. Coordinates
// beyond 1e9 or dimensions beyond 1e6 are treated as invalid for numerical safety.
func Contains(s Shape, p Point) bool {
	if !finite(p.X) || !finite(p.Y) {
		return false
	}
	g, ok := shapeGeometry(s)
	return ok && g.contains(p)
}

// Vertices returns a fresh clockwise polygon boundary in canvas coordinates.
// Circles, ellipses and invalid shapes return nil; their boundary is analytic.
func Vertices(s Shape) []Point {
	g, ok := shapeGeometry(s)
	if !ok || g.count == 0 {
		return nil
	}
	result := make([]Point, g.count)
	copy(result, g.vertices[:g.count])
	return result
}

// Area returns exact analytic area in square pixels, or zero for invalid shapes.
// Polygon area is n*rx*ry*sin(2*pi/n)/2; stars use the alternating inner radius.
func Area(s Shape) float64 {
	g, ok := shapeGeometry(s)
	if !ok {
		return 0
	}
	switch s.Kind {
	case Circle, Ellipse:
		return math.Pi * s.Width * s.Height / 4
	case Rectangle:
		return s.Width * s.Height
	case Star:
		return 5 * starInnerRadius * s.Width * s.Height / 4 * math.Sin(math.Pi/5)
	default:
		return float64(g.count) * s.Width * s.Height / 8 * math.Sin(2*math.Pi/float64(g.count))
	}
}
