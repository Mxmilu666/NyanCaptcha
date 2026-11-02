package validator

import (
	"testing"

	"github.com/mxmilu666/nyancaptcha/question"
	"github.com/mxmilu666/nyancaptcha/scene"
)

func TestRejectsConflictingColorSemantics(t *testing.T) {
	palette := scene.Palette()
	s := &scene.Scene{Width: 240, Height: 180, Shapes: []scene.Shape{
		{ID: 1, Z: 1, Kind: scene.Circle, Center: scene.Point{X: 50, Y: 50}, Width: 30, Height: 30, Fill: palette[0]},
		{ID: 2, Z: 2, Kind: scene.Rectangle, Center: scene.Point{X: 150, Y: 50}, Width: 30, Height: 30, Fill: palette[1]},
	}}
	p := question.KindIs{Kind: scene.Circle}
	text, err := question.Render(p, question.English)
	if err != nil {
		t.Fatal(err)
	}
	q := question.Question{Type: question.Basic, Text: text, Answer: 1, Expr: p}
	if err := Validate(s, q); err != nil {
		t.Fatal(err)
	}
	s.Shapes[1].Fill.Name = s.Shapes[0].Fill.Name
	if err := Validate(s, q); err == nil {
		t.Fatal("same name with different visible colors accepted")
	}
	s.Shapes[1].Fill = palette[1]
	s.Shapes[1].Fill.RGBA = palette[0].RGBA
	if err := Validate(s, q); err == nil {
		t.Fatal("different names with identical colors accepted")
	}
}
