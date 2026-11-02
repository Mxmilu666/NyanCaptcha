// Package validator checks scene readability and question semantic validity.
package validator

import (
	"fmt"

	"github.com/mxmilu666/nyancaptcha/question"
	"github.com/mxmilu666/nyancaptcha/scene"
)

// Validate recomputes geometry and the structured answer, rejecting ambiguous
// or unreadable challenges. It does not mutate the scene.
func Validate(s *scene.Scene, q question.Question) error {
	if err := question.Validate(s, q); err != nil {
		return fmt.Errorf("validator: question: %w", err)
	}
	return nil
}
