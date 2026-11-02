// Package renderer paints scene geometry into deterministic antialiased images.
package renderer

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"sort"

	"github.com/fogleman/gg"
	"github.com/mxmilu666/nyancaptcha/scene"
	"golang.org/x/image/draw"
)

const (
	supersampling = 2
	maxDimension  = 4096
	maxPixels     = 4 * 1024 * 1024
	maxShapes     = 256
	maxCoordinate = 1 << 20
)

// Config controls the opaque canvas underneath the shapes.
// Texture adds subtle deterministic color variation, without labels or metadata.
// Background must have an alpha value of 255; use DefaultConfig for defaults.
type Config struct {
	Background color.RGBA
	Texture    bool
}

// DefaultConfig returns a light opaque background with subtle texture enabled.
func DefaultConfig() Config {
	return Config{Background: color.RGBA{248, 249, 252, 255}, Texture: true}
}

// Render paints shapes in ascending Z order, using two-times supersampling on
// both axes. Polygon outlines come directly from scene.Vertices. Borders lie
// inside the shape silhouette and are one canvas pixel wide.
//
// The input is not modified. For bounded memory and work, canvases are limited
// to 4096 pixels per axis and 4,194,304 total pixels, with at most 256 shapes.
// Shape coordinates and dimensions must be finite and at most 1,048,576 in
// magnitude; dimensions must be positive. Fills and the background must be
// opaque, and Z values must be unique. Empty scenes are permitted.
func Render(s *scene.Scene, config Config) (image.Image, error) {
	if err := validate(s, config); err != nil {
		return nil, err
	}
	dc := gg.NewContext(s.Width*supersampling, s.Height*supersampling)
	dc.SetColor(config.Background)
	dc.Clear()
	if config.Texture {
		texture(dc.Image().(*image.RGBA), config.Background)
	}
	dc.Scale(supersampling, supersampling)
	order := make([]int, len(s.Shapes))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool { return s.Shapes[order[i]].Z < s.Shapes[order[j]].Z })
	for _, index := range order {
		shape := s.Shapes[index]
		outline(dc, shape)
		dc.SetColor(shape.Fill.RGBA)
		if shape.Border.A == 0 {
			dc.Fill()
			continue
		}
		dc.FillPreserve()
		// Clip a two-pixel stroke to keep its inward half inside the geometry used
		// by scene.Contains. Clip state is explicitly reset between shapes.
		dc.ClipPreserve()
		dc.SetLineWidth(2 * supersampling)
		dc.SetLineJoin(gg.LineJoinRound)
		dc.SetColor(shape.Border)
		dc.Stroke()
		dc.ResetClip()
	}
	result := image.NewRGBA(image.Rect(0, 0, s.Width, s.Height))
	draw.ApproxBiLinear.Scale(result, result.Bounds(), dc.Image(), dc.Image().Bounds(), draw.Src, nil)
	return result, nil
}

// PNG returns a deterministic PNG encoding of the rendered scene. The PNG
// contains image data only: no textual labels or ancillary metadata are added.
func PNG(s *scene.Scene, config Config) ([]byte, error) {
	var out bytes.Buffer
	if err := Write(&out, s, config); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// Write renders and encodes a PNG to w, propagating rendering and writer errors.
// A nil writer is rejected. The writer is not closed.
func Write(w io.Writer, s *scene.Scene, config Config) error {
	if w == nil {
		return errors.New("renderer: nil writer")
	}
	img, err := Render(s, config)
	if err != nil {
		return err
	}
	if err := png.Encode(w, img); err != nil {
		return fmt.Errorf("renderer: encode PNG: %w", err)
	}
	return nil
}

func outline(dc *gg.Context, shape scene.Shape) {
	dc.NewSubPath()
	switch shape.Kind {
	case scene.Circle, scene.Ellipse:
		dc.Push()
		dc.Translate(shape.Center.X, shape.Center.Y)
		dc.Rotate(shape.Rotation)
		dc.DrawEllipse(0, 0, shape.Width/2, shape.Height/2)
		dc.Pop()
	default:
		vertices := scene.Vertices(shape)
		dc.MoveTo(vertices[0].X, vertices[0].Y)
		for _, p := range vertices[1:] {
			dc.LineTo(p.X, p.Y)
		}
		dc.ClosePath()
	}
}

func validate(s *scene.Scene, config Config) error {
	if s == nil {
		return errors.New("renderer: nil scene")
	}
	if s.Width <= 0 || s.Height <= 0 || s.Width > maxDimension || s.Height > maxDimension || s.Width > maxPixels/s.Height {
		return errors.New("renderer: canvas dimensions exceed supported bounds")
	}
	if config.Background.A != 255 {
		return errors.New("renderer: background must be opaque")
	}
	if len(s.Shapes) > maxShapes {
		return errors.New("renderer: too many shapes")
	}
	seenZ := make(map[int]struct{}, len(s.Shapes))
	for i, shape := range s.Shapes {
		switch shape.Kind {
		case scene.Circle, scene.Ellipse, scene.Rectangle, scene.Triangle, scene.Pentagon, scene.Hexagon, scene.Star:
		default:
			return fmt.Errorf("renderer: shape %d has unsupported kind %q", i, shape.Kind)
		}
		for _, value := range []float64{shape.Center.X, shape.Center.Y, shape.Width, shape.Height} {
			if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > maxCoordinate {
				return fmt.Errorf("renderer: shape %d has invalid geometry", i)
			}
		}
		if shape.Width <= 0 || shape.Height <= 0 || math.IsNaN(shape.Rotation) || math.IsInf(shape.Rotation, 0) {
			return fmt.Errorf("renderer: shape %d has invalid dimensions or rotation", i)
		}
		if shape.Kind == scene.Circle && math.Abs(shape.Width-shape.Height) > 1e-9*math.Max(shape.Width, shape.Height) {
			return fmt.Errorf("renderer: circle %d must have equal diameters", i)
		}
		if shape.Fill.RGBA.A != 255 {
			return fmt.Errorf("renderer: shape %d fill must be opaque", i)
		}
		if _, exists := seenZ[shape.Z]; exists {
			return fmt.Errorf("renderer: duplicate Z value %d", shape.Z)
		}
		seenZ[shape.Z] = struct{}{}
	}
	return nil
}

func texture(img *image.RGBA, base color.RGBA) {
	// Hash canvas-pixel coordinates, repeating each sample over its 2x2
	// supersampled block. No random source or mutable global state is involved.
	for y := range img.Rect.Dy() {
		row := img.Pix[y*img.Stride:]
		for x := range img.Rect.Dx() {
			h := uint32(x/supersampling)*0x9e3779b1 ^ uint32(y/supersampling)*0x85ebca77
			h ^= h >> 16
			h *= 0x7feb352d
			h ^= h >> 15
			delta := int(h%5) - 2
			offset := x * 4
			row[offset] = adjust(base.R, delta)
			row[offset+1] = adjust(base.G, delta)
			row[offset+2] = adjust(base.B, delta)
			row[offset+3] = 255
		}
	}
}

func adjust(channel uint8, delta int) uint8 {
	value := int(channel) + delta
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return uint8(value)
}
