// Package nyancaptcha generates spatial-reasoning image CAPTCHAs without services
// or external models. Answers and image bytes are sensitive server-side data;
// applications must not send Answer to an untrusted client.
package nyancaptcha

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/mxmilu666/nyancaptcha/generator"
	"github.com/mxmilu666/nyancaptcha/question"
	"github.com/mxmilu666/nyancaptcha/renderer"
	"github.com/mxmilu666/nyancaptcha/scene"
	"github.com/mxmilu666/nyancaptcha/validator"
)

// ErrExhausted indicates that no unambiguous challenge was found within the retry bound.
var ErrExhausted = errors.New("nyancaptcha: generation attempts exhausted")

// Challenge contains a question, its server-only decimal answer, and PNG bytes.
// Type and Difficulty are evaluation metadata, not necessary client payloads.
type Challenge struct {
	Question   string
	Answer     string
	Image      []byte
	Type       question.Type
	Difficulty Difficulty
}

// Generator is an immutable configuration safe for concurrent Generate calls.
type Generator struct{ cfg config }

// New constructs a concurrency-safe generator after validating all options.
func New(options ...Option) (*Generator, error) {
	c := config{width: 360, height: 240, difficulty: Medium, language: English, retries: 80, kinds: scene.Kinds(), types: question.Types(), colors: scene.Palette(), render: renderer.DefaultConfig()}
	for _, opt := range options {
		if opt == nil {
			return nil, errors.New("nyancaptcha: nil option")
		}
		if err := opt(&c); err != nil {
			return nil, err
		}
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &Generator{cfg: c}, nil
}

// Generate creates one independent challenge. Retries are bounded; errors wrap
// ErrExhausted and the final rejection reason when constraints cannot be met.
func (g *Generator) Generate() (*Challenge, error) {
	if g == nil {
		return nil, errors.New("nyancaptcha: nil generator")
	}
	c := g.cfg
	conditions, overlap := 2, 0.22
	switch c.difficulty {
	case Easy:
		conditions, overlap = 1, 0.08
	case Hard:
		conditions, overlap = 3, 0.38
	}
	var last error
	for range c.retries {
		s, err := generator.Generate(generator.Config{Width: c.width, Height: c.height, MinShapes: c.min, MaxShapes: c.max, Kinds: c.kinds, Colors: c.colors, Overlap: overlap})
		if err != nil {
			last = err
			continue
		}
		q, err := question.Build(s, c.types, conditions, c.language)
		if err != nil {
			last = err
			continue
		}
		if err = validator.Validate(s, q); err != nil {
			last = err
			continue
		}
		png, err := renderer.PNG(s, c.render)
		if err != nil {
			return nil, fmt.Errorf("nyancaptcha: render: %w", err)
		}
		return &Challenge{Question: q.Text, Answer: strconv.Itoa(q.Answer), Image: png, Type: q.Type, Difficulty: c.difficulty}, nil
	}
	return nil, fmt.Errorf("%w after %d attempts: %w", ErrExhausted, c.retries, last)
}
