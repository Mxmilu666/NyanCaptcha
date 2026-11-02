package nyancaptcha

import (
	"bytes"
	"errors"
	"image/color"
	"image/png"
	"strconv"
	"sync"
	"testing"

	"github.com/mxmilu666/nyancaptcha/question"
	"github.com/mxmilu666/nyancaptcha/scene"
)

func TestGenerateDifficultiesAndLanguages(t *testing.T) {
	for _, d := range []Difficulty{Easy, Medium, Hard} {
		for _, l := range []Language{English, Chinese} {
			t.Run(string(d)+"/"+string(l), func(t *testing.T) {
				g, err := New(WithDifficulty(d), WithLanguage(l))
				if err != nil {
					t.Fatal(err)
				}
				for range 5 {
					c, err := g.Generate()
					if err != nil {
						t.Fatal(err)
					}
					img, err := png.Decode(bytes.NewReader(c.Image))
					if err != nil {
						t.Fatal(err)
					}
					if img.Bounds().Dx() != 360 || img.Bounds().Dy() != 240 {
						t.Fatal(img.Bounds())
					}
					n, err := strconv.Atoi(c.Answer)
					if err != nil || n < 1 || n >= g.cfg.max {
						t.Fatalf("invalid count %q", c.Answer)
					}
					if c.Question == "" || c.Difficulty != d {
						t.Fatalf("invalid challenge: %+v", c)
					}
				}
			})
		}
	}
}

func TestInvalidOptions(t *testing.T) {
	cases := [][]Option{
		{nil}, {WithSize(1, 240)}, {WithSize(1200, 900)}, {WithDifficulty("unknown")}, {WithLanguage("xx")},
		{WithShapeCount(4, 8)}, {WithShapeCount(10, 5)}, {WithShapeCount(5, 31)},
		{WithMaxRetries(0)}, {WithShapes()}, {WithShapes(scene.Kind("bad"))}, {WithShapes(scene.Circle, scene.Circle)},
		{WithQuestionTypes()}, {WithQuestionTypes(question.Type("bad"))}, {WithColors()},
		{WithBackground(color.RGBA{})},
		{WithColors(scene.Color{Name: "white", Chinese: "白色", RGBA: color.RGBA{250, 250, 250, 255}})},
	}
	for i, opts := range cases {
		if _, err := New(opts...); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
}

func TestConcurrentGeneration(t *testing.T) {
	g, err := New(WithDifficulty(Hard))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 3 {
				c, e := g.Generate()
				if e != nil {
					t.Error(e)
					return
				}
				if _, e = png.Decode(bytes.NewReader(c.Image)); e != nil {
					t.Error(e)
				}
			}
		}()
	}
	wg.Wait()
}

func TestConfigurationCopiesInputs(t *testing.T) {
	kinds := []scene.Kind{scene.Circle, scene.Rectangle}
	colors := scene.Palette()
	types := []question.Type{question.Basic}
	shapeOpt, colorOpt, typeOpt := WithShapes(kinds...), WithColors(colors...), WithQuestionTypes(types...)
	kinds[0] = "bad"
	colors[0].RGBA.A = 0
	types[0] = "bad"
	g, err := New(WithDifficulty(Easy), shapeOpt, colorOpt, typeOpt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.Generate(); err != nil {
		t.Fatal(err)
	}
}

func TestCustomCanvas(t *testing.T) {
	g, err := New(WithSize(480, 320), WithShapeCount(8, 10))
	if err != nil {
		t.Fatal(err)
	}
	c, err := g.Generate()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(c.Image))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 480 || cfg.Height != 320 {
		t.Fatal(cfg)
	}
}

func TestExhaustedImpossibleSubset(t *testing.T) {
	g, err := New(WithDifficulty(Easy), WithShapes(scene.Circle), WithColors(scene.Palette()[0]), WithQuestionTypes(question.Basic), WithMaxRetries(2))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.Generate(); !errors.Is(err, ErrExhausted) {
		t.Fatalf("expected exhaustion, got %v", err)
	}
}

func BenchmarkGenerate(b *testing.B) {
	for _, d := range []Difficulty{Easy, Medium, Hard} {
		b.Run(string(d), func(b *testing.B) {
			g, err := New(WithDifficulty(d))
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := g.Generate(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkGenerateParallel(b *testing.B) {
	g, err := New()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := g.Generate(); err != nil {
				b.Error(err)
			}
		}
	})
}
