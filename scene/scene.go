// Package scene defines the structured geometry that is the sole source of truth.
package scene

import "image/color"

// Kind identifies a geometric shape.
type Kind string

// Supported shape kinds.
const (
	Circle    Kind = "circle"
	Ellipse   Kind = "ellipse"
	Rectangle Kind = "rectangle"
	Triangle  Kind = "triangle"
	Pentagon  Kind = "pentagon"
	Hexagon   Kind = "hexagon"
	Star      Kind = "star"
)

// Point is a position in canvas pixels; X increases right, Y increases down.
type Point struct{ X, Y float64 }

// Color associates an unambiguous bilingual name with an opaque sRGB color.
type Color struct {
	Name, Chinese string
	RGBA          color.RGBA
}

// Palette returns a fresh high-contrast default palette.
func Palette() []Color {
	return []Color{
		{"red", "红色", color.RGBA{210, 45, 55, 255}},
		{"blue", "蓝色", color.RGBA{35, 100, 210, 255}},
		{"yellow", "黄色", color.RGBA{240, 205, 40, 255}},
		{"green", "绿色", color.RGBA{35, 150, 80, 255}},
		{"purple", "紫色", color.RGBA{145, 65, 185, 255}},
		{"orange", "橙色", color.RGBA{240, 125, 25, 255}},
	}
}

// Kinds returns all supported shape kinds in a fresh slice.
func Kinds() []Kind { return []Kind{Circle, Ellipse, Rectangle, Triangle, Pentagon, Hexagon, Star} }

// Shape describes a filled shape. Rotation is clockwise radians. Z must be unique.
// VisibleArea and Occlusion are measured by Analyze, never inferred from a box.
type Shape struct {
	ID                      int
	Kind                    Kind
	Center                  Point
	Width, Height, Rotation float64
	Fill                    Color
	Border                  color.RGBA
	Z                       int
	VisibleArea, Occlusion  float64
}

// Scene stores canvas dimensions and shapes. Consumers must not mutate it concurrently.
// Shapes need not be ordered; Z determines painting order.
type Scene struct {
	Width, Height int
	Shapes        []Shape
}

// Relation identifies a geometric predicate.
type Relation string

// Spatial and area relations supported by Relate.
const (
	LeftOf      Relation = "left_of"
	RightOf     Relation = "right_of"
	Above       Relation = "above"
	Below       Relation = "below"
	Near        Relation = "near"
	Far         Relation = "far"
	Inside      Relation = "inside"
	Overlapping Relation = "overlapping"
	Occluded    Relation = "occluded"
	LargerThan  Relation = "larger_than"
	SmallerThan Relation = "smaller_than"
)

// Measurement contains shape coverage and topmost-visible sample counts.
// Areas use a deterministic half-pixel lattice and have units of square pixels.
type Measurement struct{ Area, Visible float64 }
