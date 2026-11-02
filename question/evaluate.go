package question

import (
	"fmt"
	"github.com/mxmilu666/nyancaptcha/scene"
)

// evaluation owns per-operation measurements, avoiding repeated raster passes
// without trusting mutable cached metrics stored on the caller's scene.
type evaluation struct {
	s            *scene.Scene
	measured     bool
	measurements []scene.Measurement
	err          error
}

func (e *evaluation) eval(p Predicate, sh scene.Shape) (bool, error) {
	switch v := p.(type) {
	case OcclusionStatus:
		if !e.measured {
			e.measurements, e.err = scene.Measure(e.s)
			e.measured = true
		}
		if e.err != nil {
			return false, e.err
		}
		for i, t := range e.s.Shapes {
			if t.ID == sh.ID {
				return v.matches(e.measurements[i])
			}
		}
		return false, fmt.Errorf("question: candidate absent from scene")
	case And:
		return e.logical(v.Terms, sh, true)
	case Or:
		return e.logical(v.Terms, sh, false)
	case Not:
		if v.Term == nil {
			return false, fmt.Errorf("question: nil negation")
		}
		b, err := e.eval(v.Term, sh)
		return !b, err
	default:
		return p.Evaluate(e.s, sh)
	}
}
func (e *evaluation) logical(terms []Predicate, sh scene.Shape, and bool) (bool, error) {
	if len(terms) < 2 {
		return false, fmt.Errorf("question: logical expression needs two terms")
	}
	result := and
	for _, p := range terms {
		if p == nil {
			return false, fmt.Errorf("question: nil logical term")
		}
		v, err := e.eval(p, sh)
		if err != nil {
			return false, err
		}
		if and {
			result = result && v
		} else {
			result = result || v
		}
	}
	return result, nil
}
func (e *evaluation) count(p Predicate) (int, error) {
	if e.s == nil || p == nil {
		return 0, fmt.Errorf("question: nil scene or predicate")
	}
	n := 0
	for _, sh := range e.s.Shapes {
		v, err := e.eval(p, sh)
		if err != nil {
			return 0, err
		}
		if v {
			n++
		}
	}
	return n, nil
}
