package question

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/mxmilu666/nyancaptcha/scene"
)

func validateColors(s *scene.Scene) error {
	names := make(map[string]scene.Color)
	chinese := make(map[string]string)
	pixels := make(map[color.RGBA]string)
	for _, sh := range s.Shapes {
		c := sh.Fill
		if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.Chinese) == "" {
			return fmt.Errorf("question: colors require bilingual names")
		}
		if old, ok := names[c.Name]; ok && old != c {
			return fmt.Errorf("question: ambiguous English color descriptor")
		}
		if old, ok := chinese[c.Chinese]; ok && old != c.Name {
			return fmt.Errorf("question: ambiguous Chinese color descriptor")
		}
		if old, ok := pixels[c.RGBA]; ok && old != c.Name {
			return fmt.Errorf("question: identical visual colors have distinct descriptors")
		}
		for name, old := range names {
			if name == c.Name {
				continue
			}
			r, g, b := int(c.RGBA.R)-int(old.RGBA.R), int(c.RGBA.G)-int(old.RGBA.G), int(c.RGBA.B)-int(old.RGBA.B)
			if r*r+g*g+b*b < 80*80 {
				return fmt.Errorf("question: distinct color descriptors are visually too similar")
			}
		}
		names[c.Name] = c
		chinese[c.Chinese] = c.Name
		pixels[c.RGBA] = c.Name
	}
	return nil
}
