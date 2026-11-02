package nyancaptcha

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/mxmilu666/nyancaptcha/question"
	"github.com/mxmilu666/nyancaptcha/renderer"
	"github.com/mxmilu666/nyancaptcha/scene"
)

// Difficulty controls shape count, condition depth, and overlap probability.
type Difficulty string

// Available difficulty levels.
const (
	Easy   Difficulty = "easy"
	Medium Difficulty = "medium"
	Hard   Difficulty = "hard"
)

// Language selects the question grammar.
type Language = question.Language

// Supported question languages.
const (
	English = question.English
	Chinese = question.Chinese
)

// Option configures a Generator during New. Options are not retained.
type Option func(*config) error

type config struct {
	width, height     int
	difficulty        Difficulty
	language          Language
	min, max, retries int
	kinds             []scene.Kind
	types             []question.Type
	colors            []scene.Color
	render            renderer.Config
}

// WithSize sets canvas dimensions (240..1200 by 180..900, at most 720000 pixels).
func WithSize(width, height int) Option {
	return func(c *config) error { c.width, c.height = width, height; return nil }
}

// WithDifficulty selects semantic complexity and default shape counts.
func WithDifficulty(d Difficulty) Option {
	return func(c *config) error { c.difficulty = d; return nil }
}

// WithLanguage selects English or simplified Chinese.
func WithLanguage(l Language) Option { return func(c *config) error { c.language = l; return nil } }

// WithShapeCount overrides the inclusive shape count range (5..30).
func WithShapeCount(min, max int) Option {
	return func(c *config) error { c.min, c.max = min, max; return nil }
}

// WithShapes restricts the available shape kinds. Input is copied.
func WithShapes(kinds ...scene.Kind) Option {
	snapshot := append([]scene.Kind(nil), kinds...)
	return func(c *config) error { c.kinds = append([]scene.Kind(nil), snapshot...); return nil }
}

// WithQuestionTypes restricts candidate question types. Input is copied.
func WithQuestionTypes(types ...question.Type) Option {
	snapshot := append([]question.Type(nil), types...)
	return func(c *config) error { c.types = append([]question.Type(nil), snapshot...); return nil }
}

// WithColors sets an opaque, visually distinct palette with unique bilingual names.
func WithColors(colors ...scene.Color) Option {
	snapshot := append([]scene.Color(nil), colors...)
	return func(c *config) error { c.colors = append([]scene.Color(nil), snapshot...); return nil }
}

// WithMaxRetries bounds scene generation attempts (1..1000).
func WithMaxRetries(n int) Option { return func(c *config) error { c.retries = n; return nil } }

// WithBackground sets the opaque canvas background.
func WithBackground(bg color.RGBA) Option {
	return func(c *config) error { c.render.Background = bg; return nil }
}

// WithTexture enables subtle deterministic background texture.
func WithTexture(enabled bool) Option {
	return func(c *config) error { c.render.Texture = enabled; return nil }
}

func (c *config) validate() error {
	if c.width < 240 || c.width > 1200 || c.height < 180 || c.height > 900 || c.width*c.height > 720000 {
		return fmt.Errorf("nyancaptcha: invalid canvas size")
	}
	switch c.difficulty {
	case Easy, Medium, Hard:
	default:
		return fmt.Errorf("nyancaptcha: unknown difficulty %q", c.difficulty)
	}
	if c.language != English && c.language != Chinese {
		return fmt.Errorf("nyancaptcha: unsupported language %q", c.language)
	}
	if c.min == 0 && c.max == 0 {
		switch c.difficulty {
		case Easy:
			c.min, c.max = 5, 8
		case Medium:
			c.min, c.max = 9, 14
		case Hard:
			c.min, c.max = 15, 22
		}
	}
	if c.min < 5 || c.max < c.min || c.max > 30 {
		return fmt.Errorf("nyancaptcha: invalid shape count range")
	}
	if c.retries < 1 || c.retries > 1000 {
		return fmt.Errorf("nyancaptcha: retries must be in [1,1000]")
	}
	if len(c.kinds) == 0 || len(c.types) == 0 || len(c.colors) == 0 || len(c.colors) > 12 {
		return fmt.Errorf("nyancaptcha: empty kinds/types/palette or oversized palette")
	}
	knownKinds := map[scene.Kind]bool{}
	for _, k := range scene.Kinds() {
		knownKinds[k] = true
	}
	seenKinds := map[scene.Kind]bool{}
	for _, k := range c.kinds {
		if !knownKinds[k] || seenKinds[k] {
			return fmt.Errorf("nyancaptcha: invalid or duplicate shape %q", k)
		}
		seenKinds[k] = true
	}
	knownTypes := map[question.Type]bool{}
	for _, t := range question.Types() {
		knownTypes[t] = true
	}
	seenTypes := map[question.Type]bool{}
	for _, t := range c.types {
		if !knownTypes[t] || seenTypes[t] {
			return fmt.Errorf("nyancaptcha: invalid or duplicate question type %q", t)
		}
		seenTypes[t] = true
	}
	conditions := 2
	switch c.difficulty {
	case Easy:
		conditions = 1
	case Hard:
		conditions = 3
	}
	compatible := false
	for _, t := range c.types {
		limits, _ := question.Constraints(t)
		if conditions >= limits.MinConditions && conditions <= limits.MaxConditions {
			compatible = true
		}
	}
	if !compatible {
		return fmt.Errorf("nyancaptcha: no requested question type supports %s difficulty", c.difficulty)
	}
	if c.render.Background.A != 255 {
		return fmt.Errorf("nyancaptcha: background must be opaque")
	}
	names, zh := map[string]bool{}, map[string]bool{}
	for i, a := range c.colors {
		if strings.TrimSpace(a.Name) == "" || strings.TrimSpace(a.Chinese) == "" || names[a.Name] || zh[a.Chinese] || a.RGBA.A != 255 {
			return fmt.Errorf("nyancaptcha: colors require unique bilingual names and opaque fills")
		}
		names[a.Name], zh[a.Chinese] = true, true
		if colorDistance(a.RGBA, c.render.Background) < 90*90 {
			return fmt.Errorf("nyancaptcha: color %q too close to background", a.Name)
		}
		for _, b := range c.colors[:i] {
			if colorDistance(a.RGBA, b.RGBA) < 80*80 {
				return fmt.Errorf("nyancaptcha: colors %q and %q too similar", a.Name, b.Name)
			}
		}
	}
	return nil
}
func colorDistance(a, b color.RGBA) int {
	r, g, bl := int(a.R)-int(b.R), int(a.G)-int(b.G), int(a.B)-int(b.B)
	return r*r + g*g + bl*bl
}
