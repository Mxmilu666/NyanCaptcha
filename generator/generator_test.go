package generator

import (
	"testing"

	"github.com/mxmilu666/nyancaptcha/scene"
)

func TestGenerateConfiguredScene(t *testing.T) {
	c := Config{Width: 360, Height: 240, MinShapes: 9, MaxShapes: 14, Kinds: scene.Kinds(), Colors: scene.Palette(), Overlap: .22}
	successes := 0
	for range 30 {
		s, err := Generate(c)
		if err != nil {
			continue
		}
		successes++
		if len(s.Shapes) < 9 || len(s.Shapes) > 14 {
			t.Fatal(len(s.Shapes))
		}
		if err = scene.Validate(s); err != nil {
			t.Fatal(err)
		}
		for _, a := range s.Shapes {
			if a.Occlusion > .28 || a.VisibleArea <= 0 {
				t.Fatalf("unreadable shape: %+v", a)
			}
		}
	}
	if successes < 15 {
		t.Fatalf("only %d/30 scenes readable", successes)
	}
}

func TestInvalidConfig(t *testing.T) {
	if _, err := Generate(Config{}); err == nil {
		t.Fatal("invalid config accepted")
	}
}
