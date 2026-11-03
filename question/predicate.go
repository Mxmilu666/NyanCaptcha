// Package question builds bilingual counting questions from structured scene predicates.
package question

import (
	"fmt"
	"strings"

	"github.com/mxmilu666/nyancaptcha/scene"
)

// Language selects a structured expression renderer.
type Language string

// Supported languages.
const (
	English Language = "en"
	Chinese Language = "zh"
)

// Predicate is an extensible expression evaluated against one candidate shape.
// Implementations must be deterministic, reject ambiguous inputs, render their exact
// semantics in both languages, and return their number of atomic conditions.
// Evaluate must use scene data only and must not mutate it.
type Predicate interface {
	Evaluate(*scene.Scene, scene.Shape) (bool, error)
	Describe(Language) (string, error)
	Conditions() int
}

// Count evaluates every candidate, propagating ambiguity even for nonmatches.
func Count(s *scene.Scene, p Predicate) (int, error) {
	context := evaluation{s: s}
	return context.count(p)
}

func phrase(lang Language, en, zh string) (string, error) {
	switch lang {
	case English:
		return en, nil
	case Chinese:
		return zh, nil
	}
	return "", fmt.Errorf("question: unsupported language %q", lang)
}
func kindName(k scene.Kind, lang Language) (string, error) {
	names := map[scene.Kind]string{scene.Circle: "圆形", scene.Ellipse: "椭圆形", scene.Rectangle: "矩形", scene.Triangle: "三角形", scene.Pentagon: "五边形", scene.Hexagon: "六边形", scene.Star: "星形"}
	zh, ok := names[k]
	if !ok {
		return "", fmt.Errorf("question: unknown kind %q", k)
	}
	return phrase(lang, string(k), zh)
}

// ColorIs matches a palette color by its unique English name.
type ColorIs struct{ Color scene.Color }

// Evaluate matches the candidate's named color.
func (p ColorIs) Evaluate(_ *scene.Scene, sh scene.Shape) (bool, error) {
	if p.Color.Name == "" {
		return false, fmt.Errorf("question: unnamed color")
	}
	return sh.Fill.Name == p.Color.Name, nil
}

// Describe renders the color condition in the selected language.
func (p ColorIs) Describe(l Language) (string, error) {
	if p.Color.Name == "" || p.Color.Chinese == "" {
		return "", fmt.Errorf("question: color needs bilingual names")
	}
	return phrase(l, "are "+p.Color.Name, "是"+p.Color.Chinese+"的")
}

// Conditions returns one atomic condition.
func (ColorIs) Conditions() int { return 1 }

// KindIs matches a geometric kind.
type KindIs struct{ Kind scene.Kind }

// Evaluate matches the candidate's supported geometric kind.
func (p KindIs) Evaluate(_ *scene.Scene, sh scene.Shape) (bool, error) {
	_, err := kindName(p.Kind, English)
	return sh.Kind == p.Kind, err
}

// Describe renders the kind condition in the selected language.
func (p KindIs) Describe(l Language) (string, error) {
	name, err := kindName(p.Kind, l)
	if err != nil {
		return "", err
	}
	return phrase(l, "have the shape of a "+name, "是"+name)
}

// Conditions returns one atomic condition.
func (KindIs) Conditions() int { return 1 }

// And requires every child; all children are evaluated to detect ambiguities.
type And struct{ Terms []Predicate }

// Evaluate requires all terms and propagates every ambiguity.
func (p And) Evaluate(s *scene.Scene, sh scene.Shape) (bool, error) {
	return junction(s, sh, p.Terms, true)
}

// Describe renders a conjunction with language-appropriate logical grouping.
func (p And) Describe(l Language) (string, error) { return describeTerms(p.Terms, l, true) }

// Conditions sums the atomic conditions of all terms.
func (p And) Conditions() int { return conditions(p.Terms) }

// Or requires at least one child; all children are evaluated to detect ambiguities.
type Or struct{ Terms []Predicate }

// Evaluate requires any term while checking every term for ambiguity.
func (p Or) Evaluate(s *scene.Scene, sh scene.Shape) (bool, error) {
	return junction(s, sh, p.Terms, false)
}

// Describe renders a disjunction with language-appropriate logical grouping.
func (p Or) Describe(l Language) (string, error) { return describeTerms(p.Terms, l, false) }

// Conditions sums the atomic conditions of all terms.
func (p Or) Conditions() int { return conditions(p.Terms) }
func conditions(ps []Predicate) int {
	n := 0
	for _, p := range ps {
		if p != nil {
			n += p.Conditions()
		}
	}
	return n
}
func junction(s *scene.Scene, sh scene.Shape, ps []Predicate, and bool) (bool, error) {
	if len(ps) < 2 {
		return false, fmt.Errorf("question: conjunction/disjunction needs at least two terms")
	}
	result := and
	for _, p := range ps {
		if p == nil {
			return false, fmt.Errorf("question: nil term")
		}
		v, err := p.Evaluate(s, sh)
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
func describeTerms(ps []Predicate, l Language, and bool) (string, error) {
	if l == Chinese {
		return describeChineseTerms(ps, and)
	}
	if len(ps) < 2 {
		return "", fmt.Errorf("question: too few logical terms")
	}
	join := " or "
	if and {
		join = " and "
	}
	if _, err := phrase(l, "", ""); err != nil {
		return "", err
	}
	out := make([]string, len(ps))
	for i, p := range ps {
		if p == nil {
			return "", fmt.Errorf("question: nil term")
		}
		v, e := p.Describe(l)
		if e != nil {
			return "", e
		}
		out[i] = "(" + v + ")"
	}
	return strings.Join(out, join), nil
}

// Not negates a child without suppressing its validation errors.
type Not struct{ Term Predicate }

// Evaluate negates the child result without suppressing errors.
func (p Not) Evaluate(s *scene.Scene, sh scene.Shape) (bool, error) {
	if p.Term == nil {
		return false, fmt.Errorf("question: nil negation")
	}
	v, e := p.Term.Evaluate(s, sh)
	return !v, e
}

// Describe renders negation without changing the child's logical scope.
func (p Not) Describe(l Language) (string, error) {
	if p.Term == nil {
		return "", fmt.Errorf("question: nil negation")
	}
	v, e := p.Term.Describe(l)
	if e != nil {
		return "", e
	}
	if l == Chinese {
		return describeChineseNot(p.Term, v)
	}
	return phrase(l, "do not satisfy ("+v+")", "")
}

// Conditions returns the child's atomic condition count.
func (p Not) Conditions() int {
	if p.Term == nil {
		return 0
	}
	return p.Term.Conditions()
}

// Side identifies a canvas half, using the shape's center rather than its bounds.
type Side string

// Supported canvas halves.
const (
	Left   Side = "left"
	Right  Side = "right"
	Top    Side = "top"
	Bottom Side = "bottom"
)

// Position matches centers in a canvas half; centers within 3% of its midline are ambiguous.
type Position struct{ Side Side }

// Evaluate compares the candidate center to the canvas midline.
func (p Position) Evaluate(s *scene.Scene, sh scene.Shape) (bool, error) {
	if s == nil || s.Width <= 0 || s.Height <= 0 {
		return false, fmt.Errorf("question: invalid canvas")
	}
	var delta, band float64
	switch p.Side {
	case Left, Right:
		delta = sh.Center.X - float64(s.Width)/2
		band = float64(s.Width) * .03
	case Top, Bottom:
		delta = sh.Center.Y - float64(s.Height)/2
		band = float64(s.Height) * .03
	default:
		return false, fmt.Errorf("question: invalid side")
	}
	if delta >= -band && delta <= band {
		return false, fmt.Errorf("question: center near canvas midline")
	}
	if p.Side == Left || p.Side == Top {
		return delta < 0, nil
	}
	return delta > 0, nil
}

// Describe explicitly names the center-based canvas half.
func (p Position) Describe(l Language) (string, error) {
	switch p.Side {
	case Left:
		return phrase(l, "have their centers in the left half of the image", "中心位于图片左半边")
	case Right:
		return phrase(l, "have their centers in the right half of the image", "中心位于图片右半边")
	case Top:
		return phrase(l, "have their centers in the top half of the image", "中心位于图片上半边")
	case Bottom:
		return phrase(l, "have their centers in the bottom half of the image", "中心位于图片下半边")
	}
	return "", fmt.Errorf("question: invalid side")
}

// Conditions returns one atomic condition.
func (Position) Conditions() int { return 1 }

// Reference describes targets without IDs or labels. A singular reference must
// match exactly one shape. All explicitly quantifies every match, never an empty set.
// Color and Kind are optional individually, but at least one is required.
type Reference struct {
	Color *scene.Color
	Kind  scene.Kind
	All   bool
}

func (r Reference) resolve(s *scene.Scene) ([]scene.Shape, error) {
	if s == nil || r.Color == nil && r.Kind == "" {
		return nil, fmt.Errorf("question: empty reference")
	}
	if _, e := r.describe(English); e != nil {
		return nil, e
	}
	var targets []scene.Shape
	for _, sh := range s.Shapes {
		if (r.Color == nil || sh.Fill.Name == r.Color.Name) && (r.Kind == "" || sh.Kind == r.Kind) {
			targets = append(targets, sh)
		}
	}
	if len(targets) == 0 || !r.All && len(targets) != 1 {
		return nil, fmt.Errorf("question: reference matches %d shapes", len(targets))
	}
	return targets, nil
}
func (r Reference) describe(l Language) (string, error) {
	if r.Color == nil && r.Kind == "" {
		return "", fmt.Errorf("question: empty reference")
	}
	en, zh := "shape", "图形"
	if r.Kind != "" {
		var err error
		en, err = kindName(r.Kind, English)
		if err != nil {
			return "", err
		}
		zh, _ = kindName(r.Kind, Chinese)
	}
	if r.Color != nil {
		if r.Color.Name == "" || r.Color.Chinese == "" {
			return "", fmt.Errorf("question: unnamed reference color")
		}
		en = r.Color.Name + " " + en
		zh = r.Color.Chinese + zh
	}
	if r.All {
		return phrase(l, "every "+en, "每一个"+zh)
	}
	return phrase(l, "the "+en, "唯一的"+zh)
}

// Related matches a relation to one unique reference or to every explicit target.
// Self-relations are false, including within universal quantification.
type Related struct {
	Relation scene.Relation
	Target   Reference
}

// Evaluate resolves targets and checks every actual-geometry relation.
func (p Related) Evaluate(s *scene.Scene, sh scene.Shape) (bool, error) {
	if _, _, e := relationNames(p.Relation); e != nil {
		return false, e
	}
	ts, e := p.Target.resolve(s)
	if e != nil {
		return false, e
	}
	result := true
	for _, target := range ts {
		if sh.ID == target.ID {
			result = false
			continue
		}
		v, amb := scene.Relate(s, sh, target, p.Relation)
		if amb {
			return false, fmt.Errorf("question: ambiguous %s relation", p.Relation)
		}
		result = result && v
	}
	return result, nil
}
func relationNames(r scene.Relation) (string, string, error) {
	switch r {
	case scene.LeftOf:
		return "are in the same row as and to the left of", "与%s在同一排，且位于其左侧", nil
	case scene.RightOf:
		return "are in the same row as and to the right of", "与%s在同一排，且位于其右侧", nil
	case scene.Above:
		return "are in the same column as and above", "与%s在同一列，且位于其上方", nil
	case scene.Below:
		return "are in the same column as and below", "与%s在同一列，且位于其下方", nil
	case scene.Near:
		return "are near", "靠近%s", nil
	case scene.Far:
		return "are far from", "远离%s", nil
	case scene.Inside:
		return "are inside", "位于%s的内部", nil
	case scene.Overlapping:
		return "overlap", "与%s重叠", nil
	case scene.Occluded:
		return "are partly hidden behind", "被%s部分遮挡", nil
	case scene.LargerThan:
		return "have the same shape as and are larger than", "与%s形状相同，且比它大", nil
	case scene.SmallerThan:
		return "have the same shape as and are smaller than", "与%s形状相同，且比它小", nil
	}
	return "", "", fmt.Errorf("question: unsupported relation %q", r)
}

// Describe renders the relation and its singular or universal reference.
func (p Related) Describe(l Language) (string, error) {
	en, zh, e := relationNames(p.Relation)
	if e != nil {
		return "", e
	}
	target, e := p.Target.describe(l)
	if e != nil {
		return "", e
	}
	return phrase(l, en+" "+target, fmt.Sprintf(zh, target))
}

// Conditions returns one relational condition, regardless of target count.
func (Related) Conditions() int { return 1 }

// Comparison specifies an integer count comparison.
type Comparison string

// Supported integer comparisons.
const (
	Equal   Comparison = "equal"
	AtLeast Comparison = "at_least"
	AtMost  Comparison = "at_most"
	Greater Comparison = "greater"
	Less    Comparison = "less"
)

// CountComparison matches candidates related to a counted set of other shapes.
// Filter selects targets; empty target sets are rejected, not vacuously satisfied.
// Its relation is oriented from the candidate toward each selected target.
type CountComparison struct {
	Filter   Predicate
	Relation scene.Relation
	Op       Comparison
	Value    int
}

// Evaluate counts related nonself targets and applies the integer comparison.
func (p CountComparison) Evaluate(s *scene.Scene, sh scene.Shape) (bool, error) {
	if s == nil || p.Filter == nil || p.Value < 0 {
		return false, fmt.Errorf("question: invalid count comparison")
	}
	if _, _, e := relationNames(p.Relation); e != nil {
		return false, e
	}
	n, targets := 0, 0
	for _, t := range s.Shapes {
		if t.ID == sh.ID {
			continue
		}
		ok, e := p.Filter.Evaluate(s, t)
		if e != nil {
			return false, e
		}
		if !ok {
			continue
		}
		targets++
		v, a := scene.Relate(s, sh, t, p.Relation)
		if a {
			return false, fmt.Errorf("question: ambiguous count relation")
		}
		if v {
			n++
		}
	}
	if targets == 0 {
		return false, fmt.Errorf("question: empty count target set")
	}
	switch p.Op {
	case Equal:
		return n == p.Value, nil
	case AtLeast:
		return n >= p.Value, nil
	case AtMost:
		return n <= p.Value, nil
	case Greater:
		return n > p.Value, nil
	case Less:
		return n < p.Value, nil
	}
	return false, fmt.Errorf("question: invalid comparison")
}

// Describe renders the comparison, target filter, and relation together.
func (p CountComparison) Describe(l Language) (string, error) {
	if p.Filter == nil || p.Value < 0 {
		return "", fmt.Errorf("question: invalid count comparison")
	}
	en, zh, e := relationNames(p.Relation)
	if e != nil {
		return "", e
	}
	filter, e := p.Filter.Describe(l)
	if e != nil {
		return "", e
	}
	var a, b string
	switch p.Op {
	case Equal:
		a, b = "exactly", "恰好"
	case AtLeast:
		a, b = "at least", "至少"
	case AtMost:
		a, b = "at most", "至多"
	case Greater:
		a, b = "more than", "多于"
	case Less:
		a, b = "fewer than", "少于"
	default:
		return "", fmt.Errorf("question: invalid comparison")
	}
	if l == Chinese {
		quantity := fmt.Sprintf("%s%d个", b, p.Value)
		if p.Value == 1 {
			quantity = b + "一个"
		}
		target, err := chineseCountTarget(p.Filter)
		if err != nil {
			return "", err
		}
		if p.Value == 0 {
			return describeChineseZeroCount(p, target)
		}
		return fmt.Sprintf(zh, quantity+target), nil
	}
	return phrase(l, fmt.Sprintf("%s %s %d other shapes that (%s)", en, a, p.Value, filter), "")
}

// chineseCountTarget uses a noun phrase for attributes and keeps compound
// target conditions explicitly grouped, distinct from the candidate's clauses.
func chineseCountTarget(p Predicate) (string, error) {
	noun, terms := chineseSubject(p)
	clause, err := chineseRemaining(terms)
	if err != nil {
		return "", err
	}
	if clause == "" {
		return "其他" + noun, nil
	}
	return "其他" + noun + "（这些目标图形" + clause + "）", nil
}

// Conditions includes the relation and target filter's atomic conditions.
func (p CountComparison) Conditions() int {
	if p.Filter == nil {
		return 1
	}
	return 1 + p.Filter.Conditions()
}
