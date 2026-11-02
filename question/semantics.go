package question

import (
	"fmt"
	"github.com/mxmilu666/nyancaptcha/scene"
)

// OcclusionStatus matches shapes obscured by any other shape (Hidden true), or
// shapes not obscured at all (Hidden false). Tiny overlaps in the geometry
// ambiguity band are rejected rather than silently treated as unobscured.
type OcclusionStatus struct{ Hidden bool }

// Evaluate measures actual union occlusion independently of cached metrics.
func (p OcclusionStatus) Evaluate(s *scene.Scene, sh scene.Shape) (bool, error) {
	measurements, e := scene.Measure(s)
	if e != nil {
		return false, e
	}
	for i, other := range s.Shapes {
		if other.ID != sh.ID {
			continue
		}
		return p.matches(measurements[i])
	}
	return false, fmt.Errorf("question: candidate absent from scene")
}

// Describe renders partial occlusion or complete absence of occlusion.
func (p OcclusionStatus) Describe(l Language) (string, error) {
	if p.Hidden {
		return phrase(l, "are partially hidden by other shapes", "被其他图形部分遮挡")
	}
	return phrase(l, "are not hidden by any other shape", "完全没有被其他图形遮挡")
}

// Conditions returns one atomic condition.
func (OcclusionStatus) Conditions() int { return 1 }
func (p OcclusionStatus) matches(m scene.Measurement) (bool, error) {
	if m.Area <= 0 {
		return false, fmt.Errorf("question: zero shape coverage")
	}
	hidden := (m.Area - m.Visible) / m.Area
	if hidden > 0 && hidden <= .04 {
		return false, fmt.Errorf("question: ambiguous occlusion status")
	}
	return (hidden > .04) == p.Hidden, nil
}

// meaningful requires each branch of AND/OR to change the selected set. This
// rejects duplicate, implied, incompatible, and decorative difficulty filters.
func meaningful(s *scene.Scene, p Predicate) error {
	var terms []Predicate
	and := false
	switch v := p.(type) {
	case And:
		terms = v.Terms
		and = true
	case Or:
		terms = v.Terms
	case Not:
		return meaningful(s, v.Term)
	case CountComparison:
		return meaningful(s, v.Filter)
	default:
		return nil
	}
	if len(terms) < 2 {
		return fmt.Errorf("question: empty logical expression")
	}
	for _, term := range terms {
		if term == nil {
			return fmt.Errorf("question: nil logical term")
		}
		if e := meaningful(s, term); e != nil {
			return e
		}
	}
	context := evaluation{s: s}
	for skip := range terms {
		changes := false
		for _, sh := range s.Shapes {
			full := and
			without := and
			for j, term := range terms {
				v, e := context.eval(term, sh)
				if e != nil {
					return e
				}
				if and {
					full = full && v
					if j != skip {
						without = without && v
					}
				} else {
					full = full || v
					if j != skip {
						without = without || v
					}
				}
			}
			if full != without {
				changes = true
				break
			}
		}
		if !changes {
			return fmt.Errorf("question: redundant logical condition")
		}
	}
	return nil
}
