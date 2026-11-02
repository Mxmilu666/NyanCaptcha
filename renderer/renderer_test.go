package renderer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"reflect"
	"testing"

	"github.com/mxmilu666/nyancaptcha/scene"
)

func fixture() *scene.Scene {
	return &scene.Scene{Width: 128, Height: 96, Shapes: []scene.Shape{
		{ID: 1, Kind: scene.Ellipse, Center: scene.Point{X: 48, Y: 48}, Width: 66, Height: 40, Rotation: .4, Fill: scene.Palette()[0], Border: color.RGBA{45, 45, 45, 255}, Z: 0},
		{ID: 2, Kind: scene.Star, Center: scene.Point{X: 80, Y: 48}, Width: 50, Height: 50, Rotation: -.2, Fill: scene.Palette()[1], Border: color.RGBA{45, 45, 45, 255}, Z: 1},
	}}
}

func TestPNGDecodingAndDeterminism(t *testing.T) {
	s := fixture()
	original := append([]scene.Shape(nil), s.Shapes...)
	first, err := PNG(s, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	second, err := PNG(s, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("same inputs produced different PNG bytes")
	}
	if !reflect.DeepEqual(original, s.Shapes) {
		t.Fatal("rendering modified the input scene")
	}
	decoded, err := png.Decode(bytes.NewReader(first))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != image.Rect(0, 0, s.Width, s.Height) {
		t.Fatalf("unexpected bounds: %v", decoded.Bounds())
	}
	direct, err := Render(s, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	for y := range s.Height {
		for x := range s.Width {
			got := color.RGBAModel.Convert(decoded.At(x, y)).(color.RGBA)
			want := color.RGBAModel.Convert(direct.At(x, y)).(color.RGBA)
			if got != want || got.A != 255 {
				t.Fatalf("pixel (%d,%d): %v, want opaque %v", x, y, got, want)
			}
		}
	}
	for offset := 8; offset < len(first); {
		if len(first)-offset < 12 {
			t.Fatal("truncated PNG chunk")
		}
		size := int(binary.BigEndian.Uint32(first[offset : offset+4]))
		kind := string(first[offset+4 : offset+8])
		switch kind {
		case "IHDR", "IDAT", "IEND":
		default:
			t.Fatalf("unexpected metadata chunk %q", kind)
		}
		offset += 12 + size
	}
	var out bytes.Buffer
	if err := Write(&out, s, DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), first) {
		t.Fatal("Write differs from PNG")
	}
}

func TestGeometryAndAntialiasing(t *testing.T) {
	config := Config{Background: color.RGBA{255, 255, 255, 255}}
	for _, kind := range scene.Kinds() {
		t.Run(string(kind), func(t *testing.T) {
			shape := scene.Shape{ID: 1, Kind: kind, Center: scene.Point{X: 48, Y: 48}, Width: 58, Height: 58, Rotation: .37, Fill: scene.Palette()[0]}
			if kind == scene.Ellipse {
				shape.Height = 34
			}
			s := &scene.Scene{Width: 96, Height: 96, Shapes: []scene.Shape{shape}}
			img, err := Render(s, config)
			if err != nil {
				t.Fatal(err)
			}
			blended := false
			for y := range s.Height {
				for x := range s.Width {
					got := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
					if got != config.Background && got != shape.Fill.RGBA {
						blended = true
					}
					// Ignore the boundary's antialiasing footprint; interior and exterior
					// pixels must agree with the analytic geometry in every direction.
					center := scene.Contains(shape, scene.Point{X: float64(x) + .5, Y: float64(y) + .5})
					stable := true
					for _, dx := range []float64{-2, 0, 2} {
						for _, dy := range []float64{-2, 0, 2} {
							if scene.Contains(shape, scene.Point{X: float64(x) + .5 + dx, Y: float64(y) + .5 + dy}) != center {
								stable = false
							}
						}
					}
					if stable {
						want := config.Background
						if center {
							want = shape.Fill.RGBA
						}
						if got != want {
							t.Fatalf("geometry mismatch at %d,%d: got %v, want %v", x, y, got, want)
						}
					}
				}
			}
			if !blended {
				t.Fatal("shape has no antialiased boundary pixels")
			}
		})
	}
}

func TestZOrderAndBorderClipping(t *testing.T) {
	config := Config{Background: color.RGBA{255, 255, 255, 255}}
	back := scene.Shape{ID: 1, Kind: scene.Rectangle, Center: scene.Point{X: 30, Y: 30}, Width: 40, Height: 40, Fill: scene.Palette()[0], Border: color.RGBA{0, 0, 0, 255}, Z: 0}
	front := scene.Shape{ID: 2, Kind: scene.Rectangle, Center: scene.Point{X: 48, Y: 30}, Width: 40, Height: 40, Fill: scene.Palette()[1], Border: color.RGBA{0, 0, 0, 255}, Z: 1}
	s := &scene.Scene{Width: 80, Height: 64, Shapes: []scene.Shape{front, back}}
	img, err := Render(s, config)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		x, y int
		want color.RGBA
	}{
		{40, 30, front.Fill.RGBA}, {60, 30, front.Fill.RGBA}, {20, 30, back.Fill.RGBA},
		{8, 30, config.Background}, {70, 30, config.Background}, {10, 30, back.Border},
	} {
		got := color.RGBAModel.Convert(img.At(item.x, item.y)).(color.RGBA)
		if got != item.want {
			t.Errorf("pixel %d,%d = %v, want %v", item.x, item.y, got, item.want)
		}
	}
	first, err := PNG(s, config)
	if err != nil {
		t.Fatal(err)
	}
	s.Shapes[0], s.Shapes[1] = s.Shapes[1], s.Shapes[0]
	second, err := PNG(s, config)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("slice order affected Z sorting")
	}
}

func TestBackgroundTexture(t *testing.T) {
	s := &scene.Scene{Width: 32, Height: 24}
	config := Config{Background: color.RGBA{100, 120, 140, 255}}
	plain, err := Render(s, config)
	if err != nil {
		t.Fatal(err)
	}
	config.Texture = true
	textured, err := Render(s, config)
	if err != nil {
		t.Fatal(err)
	}
	changed := false
	for y := range s.Height {
		for x := range s.Width {
			p := color.RGBAModel.Convert(plain.At(x, y)).(color.RGBA)
			q := color.RGBAModel.Convert(textured.At(x, y)).(color.RGBA)
			if p != config.Background {
				t.Fatal("plain background is not uniform")
			}
			if p != q {
				changed = true
			}
			if math.Abs(float64(q.R)-float64(p.R)) > 2 || math.Abs(float64(q.G)-float64(p.G)) > 2 || math.Abs(float64(q.B)-float64(p.B)) > 2 || q.A != 255 {
				t.Fatalf("texture is not subtle and opaque: %v", q)
			}
		}
	}
	if !changed {
		t.Fatal("texture had no effect")
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestErrors(t *testing.T) {
	writerError := errors.New("writer failed")
	if err := Write(failingWriter{writerError}, fixture(), DefaultConfig()); !errors.Is(err, writerError) {
		t.Fatalf("lost writer error: %v", err)
	}
	if err := Write(nil, fixture(), DefaultConfig()); err == nil {
		t.Fatal("nil writer accepted")
	}
	tests := []struct {
		name   string
		change func(*scene.Scene, *Config)
	}{
		{"zero width", func(s *scene.Scene, c *Config) { s.Width = 0 }},
		{"negative height", func(s *scene.Scene, c *Config) { s.Height = -1 }},
		{"giant width", func(s *scene.Scene, c *Config) { s.Width = int(^uint(0) >> 1) }},
		{"giant area", func(s *scene.Scene, c *Config) { s.Width = 4096; s.Height = 4096 }},
		{"transparent background", func(s *scene.Scene, c *Config) { c.Background.A = 254 }},
		{"invalid kind", func(s *scene.Scene, c *Config) { s.Shapes[0].Kind = "unknown" }},
		{"nan center", func(s *scene.Scene, c *Config) { s.Shapes[0].Center.X = math.NaN() }},
		{"infinite dimension", func(s *scene.Scene, c *Config) { s.Shapes[0].Height = math.Inf(1) }},
		{"negative dimension", func(s *scene.Scene, c *Config) { s.Shapes[0].Width = -1 }},
		{"zero dimension", func(s *scene.Scene, c *Config) { s.Shapes[0].Height = 0 }},
		{"infinite rotation", func(s *scene.Scene, c *Config) { s.Shapes[0].Rotation = math.Inf(-1) }},
		{"nan rotation", func(s *scene.Scene, c *Config) { s.Shapes[0].Rotation = math.NaN() }},
		{"giant coordinate", func(s *scene.Scene, c *Config) { s.Shapes[0].Center.Y = math.MaxFloat64 }},
		{"transparent fill", func(s *scene.Scene, c *Config) { s.Shapes[0].Fill.RGBA.A = 0 }},
		{"duplicate Z", func(s *scene.Scene, c *Config) { s.Shapes[0].Z = s.Shapes[1].Z }},
		{"too many shapes", func(s *scene.Scene, c *Config) { s.Shapes = make([]scene.Shape, maxShapes+1) }},
	}
	if _, err := Render(nil, DefaultConfig()); err == nil {
		t.Fatal("nil scene accepted")
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, c := fixture(), DefaultConfig()
			tt.change(s, &c)
			if _, err := Render(s, c); err == nil {
				t.Fatal("invalid input accepted")
			}
			if data, err := PNG(s, c); err == nil || data != nil {
				t.Fatal("invalid PNG input accepted")
			}
			var dst bytes.Buffer
			if err := Write(&dst, s, c); err == nil || dst.Len() != 0 {
				t.Fatal("invalid Write input produced output")
			}
		})
	}
}

func FuzzRenderDimensions(f *testing.F) {
	f.Add(16, 16)
	f.Add(-1, 32)
	f.Add(4097, 1)
	f.Fuzz(func(t *testing.T, width, height int) {
		// Keep successful renders small while retaining arbitrary invalid values.
		if width > 0 && width <= maxDimension && height > 0 && height <= maxDimension && width <= maxPixels/height && (width > 32 || height > 32) {
			return
		}
		s := &scene.Scene{Width: width, Height: height}
		img, err := Render(s, DefaultConfig())
		if err == nil && img.Bounds() != image.Rect(0, 0, width, height) {
			t.Fatal("incorrect output dimensions")
		}
	})
}

func BenchmarkRender(b *testing.B) {
	s := fixture()
	config := DefaultConfig()
	b.ReportAllocs()
	for range b.N {
		if _, err := Render(s, config); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPNG(b *testing.B) {
	s := fixture()
	s.Width, s.Height = 360, 240
	config := DefaultConfig()
	b.ReportAllocs()
	for range b.N {
		if _, err := PNG(s, config); err != nil {
			b.Fatal(err)
		}
	}
}
