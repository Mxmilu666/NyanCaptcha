package question

import (
	"fmt"

	"github.com/mxmilu666/nyancaptcha/internal/random"
	"github.com/mxmilu666/nyancaptcha/scene"
)

// Type identifies the principal semantic category of a question.
type Type string

// Supported question categories.
const (
	Basic       Type = "basic"
	Spatial     Type = "spatial"
	ColorShape  Type = "color_shape"
	Occlusion   Type = "occlusion"
	Size        Type = "size"
	Compound    Type = "compound"
	MultiTarget Type = "multi_target"
	Exclusion   Type = "exclusion"
)

// Types returns all eight categories in a fresh slice.
func Types() []Type {
	return []Type{Basic, Spatial, ColorShape, Occlusion, Size, Compound, MultiTarget, Exclusion}
}

// Question contains a natural-language rendering, a scene-derived answer, and its executable expression.
type Question struct {
	Type   Type
	Text   string
	Answer int
	Expr   Predicate
}

// Suitability documents a category's generation constraints.
type Suitability struct {
	MinConditions, MaxConditions int
	Requirements                 string
}

// Constraints describes suitability; unknown categories return an error.
func Constraints(t Type) (Suitability, error) {
	switch t {
	case Basic:
		return Suitability{1, 1, "At least two distinct colors or kinds; a nonempty proper subset."}, nil
	case Spatial:
		return Suitability{1, 3, "Clear canvas half or unambiguous spatial relation to a unique color/kind reference."}, nil
	case ColorShape:
		return Suitability{2, 3, "A nonempty proper subset selected jointly by color and kind."}, nil
	case Occlusion:
		return Suitability{1, 3, "A uniquely described occluder and clearly occluded, still readable shapes."}, nil
	case Size:
		return Suitability{1, 3, "A unique same-kind reference with both dimensions separated by more than 1.6x."}, nil
	case Compound:
		return Suitability{2, 3, "Two or three distinct atomic conditions joined by explicit AND/OR, or a relational count comparison."}, nil
	case MultiTarget:
		return Suitability{1, 3, "At least two targets sharing a descriptor; the relation holds for every target, excluding self."}, nil
	case Exclusion:
		return Suitability{1, 3, "An explicitly negated condition selecting a nonempty proper subset."}, nil
	}
	return Suitability{}, fmt.Errorf("question: unknown type %q", t)
}

// Render turns an AST into a counting question without text substitution.
func Render(p Predicate, l Language) (string, error) {
	if p == nil {
		return "", fmt.Errorf("question: nil expression")
	}
	d, e := p.Describe(l)
	if e != nil {
		return "", e
	}
	if l == Chinese {
		return renderChinese(p)
	}
	return phrase(l, "How many shapes "+d+"?", "")
}

// Validate checks geometry/readability, every semantic boundary, exact rendering,
// and a nonzero answer smaller than the number of shapes. It never mutates s.
// Custom Predicate implementations participate through their interface methods.
func Validate(s *scene.Scene, q Question) error {
	if e := scene.Validate(s); e != nil {
		return fmt.Errorf("question: invalid scene: %w", e)
	}
	if e := validateColors(s); e != nil {
		return e
	}
	return validateExpression(s, q)
}
func validateExpression(s *scene.Scene, q Question) error {
	limits, e := Constraints(q.Type)
	if e != nil {
		return e
	}
	if q.Expr == nil {
		return fmt.Errorf("question: nil expression")
	}
	n := q.Expr.Conditions()
	if n < limits.MinConditions || n > limits.MaxConditions {
		return fmt.Errorf("question: condition count outside type constraints")
	}
	answer, e := Count(s, q.Expr)
	if e != nil {
		return e
	}
	if answer <= 0 || answer >= len(s.Shapes) {
		return fmt.Errorf("question: trivial answer")
	}
	if answer != q.Answer {
		return fmt.Errorf("question: answer differs from scene")
	}
	if e := meaningful(s, q.Expr); e != nil {
		return e
	}
	en, e := Render(q.Expr, English)
	if e != nil {
		return e
	}
	zh, e := Render(q.Expr, Chinese)
	if e != nil {
		return e
	}
	if q.Text != en && q.Text != zh {
		return fmt.Errorf("question: text differs from expression")
	}
	return nil
}

// Build randomly tries applicable categories with at most 64 candidates per type.
// conditions must be 1 (easy), 2 (medium), or 3 (hard); generated expressions
// have exactly that many atomic conditions. An empty types slice enables all types.
// Failure means no clear, nontrivial candidate was found within the bounded search.
// A desired answer is sampled independently in [1,len(shapes)-1].
// Feasibility can still bias successful outputs; this is not a uniformity guarantee.
func Build(s *scene.Scene, types []Type, conditions int, lang Language) (Question, error) {
	if conditions < 1 || conditions > 3 {
		return Question{}, fmt.Errorf("question: conditions must be 1, 2, or 3")
	}
	if _, e := phrase(lang, "", ""); e != nil {
		return Question{}, e
	}
	if e := scene.Validate(s); e != nil {
		return Question{}, fmt.Errorf("question: invalid scene: %w", e)
	}
	if e := validateColors(s); e != nil {
		return Question{}, e
	}
	if len(types) == 0 {
		types = Types()
	}
	applicable := make([]Type, 0, len(types))
	seen := map[Type]bool{}
	for _, t := range types {
		c, e := Constraints(t)
		if e != nil {
			return Question{}, e
		}
		if conditions >= c.MinConditions && conditions <= c.MaxConditions && !seen[t] {
			applicable = append(applicable, t)
			seen[t] = true
		}
	}
	if len(applicable) == 0 {
		return Question{}, fmt.Errorf("question: no type supports %d conditions", conditions)
	}
	desired, e := pick(len(s.Shapes) - 1)
	if e != nil {
		return Question{}, e
	}
	desired++
	context := evaluation{s: s}
	for len(applicable) > 0 {
		i, e := random.Int(len(applicable))
		if e != nil {
			return Question{}, e
		}
		t := applicable[i]
		applicable[i] = applicable[len(applicable)-1]
		applicable = applicable[:len(applicable)-1]
		for range 64 {
			p, e := candidate(s, t, conditions)
			if e != nil {
				return Question{}, e
			}
			if p == nil {
				continue
			}
			answer, e := context.count(p)
			if e != nil || answer != desired {
				continue
			}
			text, e := Render(p, lang)
			if e != nil {
				continue
			}
			q := Question{t, text, answer, p}
			if e = validateExpression(s, q); e == nil {
				return q, nil
			}
		}
	}
	return Question{}, fmt.Errorf("question: no unambiguous nontrivial candidate for requested types and conditions")
}
func pick(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("question: empty choice")
	}
	return random.Int(n)
}
func sample(s *scene.Scene) (scene.Shape, error) {
	i, e := pick(len(s.Shapes))
	if e != nil {
		return scene.Shape{}, e
	}
	return s.Shapes[i], nil
}
func attribute(s *scene.Scene) (Predicate, error) {
	sh, e := sample(s)
	if e != nil {
		return nil, e
	}
	i, e := pick(2)
	if e != nil {
		return nil, e
	}
	if i == 0 {
		return ColorIs{sh.Fill}, nil
	}
	return KindIs{sh.Kind}, nil
}
func unique(sh scene.Shape) Reference { c := sh.Fill; return Reference{Color: &c, Kind: sh.Kind} }
func relationCandidate(s *scene.Scene, rs []scene.Relation, all bool) (Predicate, error) {
	sh, e := sample(s)
	if e != nil {
		return nil, e
	}
	i, e := pick(len(rs))
	if e != nil {
		return nil, e
	}
	r := unique(sh)
	if all {
		j, e := pick(2)
		if e != nil {
			return nil, e
		}
		r.All = true
		if j == 0 && rs[i] != scene.LargerThan && rs[i] != scene.SmallerThan {
			r.Kind = ""
		} else {
			r.Color = nil
		}
		targets, e := r.resolve(s)
		if e != nil || len(targets) < 2 {
			return nil, nil
		}
	}
	return Related{Relation: rs[i], Target: r}, nil
}

// sizeCandidate samples references clearly separated in both dimensions from
// every other shape of the same kind; unrelated kinds never count as matches.
func sizeCandidate(s *scene.Scene) (Predicate, error) {
	i, err := pick(2)
	if err != nil {
		return nil, err
	}
	relation := scene.LargerThan
	if i == 1 {
		relation = scene.SmallerThan
	}
	var selected Predicate
	eligible := 0
	for _, sh := range s.Shapes {
		p := Related{Relation: relation, Target: unique(sh)}
		n, err := Count(s, p)
		if err != nil || n == 0 || n == len(s.Shapes) {
			continue
		}
		eligible++
		choice, err := pick(eligible)
		if err != nil {
			return nil, err
		}
		if choice == 0 {
			selected = p
		}
	}
	return selected, nil
}
func candidate(s *scene.Scene, t Type, n int) (Predicate, error) {
	var p Predicate
	var e error
	switch t {
	case Basic:
		p, e = attribute(s)
	case ColorShape:
		var sh scene.Shape
		sh, e = sample(s)
		if e == nil {
			p = And{[]Predicate{ColorIs{sh.Fill}, KindIs{sh.Kind}}}
		}
	case Spatial:
		var i int
		i, e = pick(8)
		if e != nil {
			break
		}
		if i < 4 {
			p = Position{[]Side{Left, Right, Top, Bottom}[i]}
		} else {
			// Generated questions use visible relations, never subjective near/far distances.
			p, e = relationCandidate(s, []scene.Relation{scene.LeftOf, scene.RightOf, scene.Above, scene.Below, scene.Overlapping, scene.Inside}, false)
		}
	case Occlusion:
		var i int
		i, e = pick(3)
		if e == nil {
			if i < 2 {
				p = OcclusionStatus{Hidden: i == 0}
			} else {
				p, e = relationCandidate(s, []scene.Relation{scene.Occluded}, false)
			}
		}
	case Size:
		p, e = sizeCandidate(s)
	case MultiTarget:
		p, e = relationCandidate(s, []scene.Relation{scene.LargerThan, scene.SmallerThan, scene.LeftOf, scene.RightOf, scene.Above, scene.Below, scene.Overlapping}, true)
	case Exclusion:
		var child Predicate
		child, e = attribute(s)
		if e == nil {
			p = Not{child}
		}
	case Compound:
		var i int
		i, e = pick(3)
		if e != nil {
			break
		}
		if i == 0 {
			var filter Predicate
			filter, e = attribute(s)
			if e != nil {
				break
			}
			var r int
			r, e = pick(4)
			if e != nil {
				break
			}
			p = CountComparison{Filter: filter, Relation: []scene.Relation{scene.LeftOf, scene.RightOf, scene.LargerThan, scene.SmallerThan}[r], Op: AtLeast, Value: 1}
		}
		if i != 0 {
			var a, b Predicate
			a, e = attribute(s)
			if e != nil {
				break
			}
			b, e = attribute(s)
			if e != nil {
				break
			}
			if same(a, b) {
				return nil, nil
			}
			if i == 1 {
				p = And{[]Predicate{a, b}}
			} else {
				p = Or{[]Predicate{a, b}}
			}
		}
	}
	if e != nil || p == nil {
		return p, e
	}
	for p.Conditions() < n {
		extra, e := additional(s)
		if e != nil {
			return nil, e
		}
		if contains(p, extra) {
			return nil, nil
		}
		p = And{[]Predicate{p, extra}}
	}
	if p.Conditions() != n {
		return nil, nil
	}
	return p, nil
}
func same(a, b Predicate) bool {
	x, e := a.Describe(English)
	if e != nil {
		return false
	}
	y, e := b.Describe(English)
	return e == nil && x == y
}
func contains(p, needle Predicate) bool {
	if same(p, needle) {
		return true
	}
	switch v := p.(type) {
	case And:
		for _, c := range v.Terms {
			if contains(c, needle) {
				return true
			}
		}
	case Or:
		for _, c := range v.Terms {
			if contains(c, needle) {
				return true
			}
		}
	case Not:
		return contains(v.Term, needle)
	}
	return false
}
func additional(s *scene.Scene) (Predicate, error) {
	i, e := pick(3)
	if e != nil {
		return nil, e
	}
	if i < 2 {
		return attribute(s)
	}
	j, e := pick(4)
	if e != nil {
		return nil, e
	}
	return Position{[]Side{Left, Right, Top, Bottom}[j]}, nil
}
