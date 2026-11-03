package question

import (
	"fmt"
	"strings"

	"github.com/mxmilu666/nyancaptcha/scene"
)

// chineseLogicalTerms flattens only identical operators. Mixed AND/OR groups
// retain their scope; custom predicates remain opaque clauses.
func chineseLogicalTerms(ps []Predicate, and bool) ([]Predicate, error) {
	if len(ps) < 2 {
		return nil, fmt.Errorf("question: too few logical terms")
	}
	out := make([]Predicate, 0, len(ps))
	for _, p := range ps {
		if p == nil {
			return nil, fmt.Errorf("question: nil term")
		}
		children, childAnd, logical := chineseLogicalNode(p)
		if logical && childAnd == and {
			flat, err := chineseLogicalTerms(children, and)
			if err != nil {
				return nil, err
			}
			out = append(out, flat...)
		} else {
			out = append(out, p)
		}
	}
	return out, nil
}

func chineseLogicalNode(p Predicate) (terms []Predicate, and, logical bool) {
	switch p := p.(type) {
	case And:
		return p.Terms, true, true
	case *And:
		return p.Terms, true, true
	case Or:
		return p.Terms, false, true
	case *Or:
		return p.Terms, false, true
	default:
		return nil, false, false
	}
}

func chineseRenderedLogical(p Predicate) (and, logical bool) {
	switch p := p.(type) {
	case Not:
		and, logical = chineseRenderedLogical(p.Term)
		return !and, logical
	case *Not:
		and, logical = chineseRenderedLogical(p.Term)
		return !and, logical
	default:
		_, and, logical = chineseLogicalNode(p)
		return and, logical
	}
}

func describeChineseTerms(ps []Predicate, and bool) (string, error) {
	terms, err := chineseLogicalTerms(ps, and)
	if err != nil {
		return "", err
	}
	out := make([]string, len(terms))
	for i, p := range terms {
		text, err := p.Describe(Chinese)
		if err != nil {
			return "", err
		}
		// Negated groups may change operators through De Morgan expansion.
		if childAnd, logical := chineseRenderedLogical(p); logical && childAnd != and {
			text = "（" + text + "）"
		}
		out[i] = text
	}
	if and {
		return strings.Join(out, "，且"), nil
	}
	// OR is inclusive: a shape matching both branches is counted only once.
	return strings.Join(out, "，或者"), nil
}

// chineseSubject moves positive attributes into the noun phrase, but only
// across conjunctions. An attribute in one OR branch must never filter another.
// Callers first validate the complete expression through Describe.
func chineseSubject(p Predicate) (noun string, rest []Predicate) {
	terms := []Predicate{p}
	if children, and, logical := chineseLogicalNode(p); logical && and {
		terms, _ = chineseLogicalTerms(children, true)
	}
	color, kind := "", ""
	for _, term := range terms {
		var c, k string
		switch t := term.(type) {
		case ColorIs:
			c = t.Color.Chinese
		case *ColorIs:
			c = t.Color.Chinese
		case KindIs:
			k, _ = kindName(t.Kind, Chinese)
		case *KindIs:
			k, _ = kindName(t.Kind, Chinese)
		}
		if c != "" && color == "" {
			color = c
		} else if k != "" && kind == "" {
			kind = k
		} else {
			rest = append(rest, term)
		}
	}
	if kind == "" {
		kind = "图形"
	}
	return color + kind, rest
}

func chineseRemaining(terms []Predicate) (string, error) {
	switch len(terms) {
	case 0:
		return "", nil
	case 1:
		return terms[0].Describe(Chinese)
	default:
		return describeChineseTerms(terms, true)
	}
}

func renderChinese(p Predicate) (string, error) {
	noun, terms := chineseSubject(p)
	clause, err := chineseRemaining(terms)
	if err != nil {
		return "", err
	}
	if children, and, logical := chineseLogicalNode(p); logical {
		parts, err := chineseLogicalTerms(children, and)
		if err != nil {
			return "", err
		}
		for _, child := range parts {
			if childAnd, childLogical := chineseRenderedLogical(child); childLogical && childAnd != and {
				// A short condition list reads more clearly than an expression
				// embedded between the subject and its verb.
				heading := "同时符合以下条件"
				if !and {
					heading = "符合以下至少一项条件（每个图形只计一次）"
				}
				var text strings.Builder
				text.WriteString("有多少个图形" + heading + "？")
				for i, part := range parts {
					description, err := part.Describe(Chinese)
					if err != nil {
						return "", err
					}
					fmt.Fprintf(&text, "\n%d. %s", i+1, description)
				}
				return text.String(), nil
			}
		}
	}
	if len(terms) > 0 {
		if chineseCenterClause(terms[0]) {
			clause = "的" + clause
		}
	}
	return "有多少个" + noun + clause + "？", nil
}

func chineseCenterClause(p Predicate) bool {
	switch p := p.(type) {
	case Position, *Position:
		return true
	case Not:
		return chineseCenterClause(p.Term)
	case *Not:
		return chineseCenterClause(p.Term)
	default:
		return false
	}
}

// Negating a logical group uses De Morgan's law rather than hiding the scope
// inside a literal "does not satisfy" wrapper. Unknown predicates keep scope.
func describeChineseNot(p Predicate, description string) (string, error) {
	if children, and, logical := chineseLogicalNode(p); logical {
		terms := make([]Predicate, len(children))
		for i, child := range children {
			terms[i] = Not{Term: child}
		}
		return describeChineseTerms(terms, !and)
	}
	switch p := p.(type) {
	case ColorIs, *ColorIs, KindIs, *KindIs:
		return "不" + description, nil
	case Not:
		return p.Term.Describe(Chinese)
	case *Not:
		return p.Term.Describe(Chinese)
	case Position:
		return chinesePositionNot(p)
	case *Position:
		return chinesePositionNot(*p)
	case OcclusionStatus:
		return (OcclusionStatus{Hidden: !p.Hidden}).Describe(Chinese)
	case *OcclusionStatus:
		return (OcclusionStatus{Hidden: !p.Hidden}).Describe(Chinese)
	default:
		return "不符合“" + description + "”这一条件", nil
	}
}

func chinesePositionNot(p Position) (string, error) {
	switch p.Side {
	case Left:
		return "中心不在图片左半边", nil
	case Right:
		return "中心不在图片右半边", nil
	case Top:
		return "中心不在图片上半边", nil
	case Bottom:
		return "中心不在图片下半边", nil
	default:
		return "", fmt.Errorf("question: invalid side")
	}
}

// Zero comparisons describe the targets relative to the candidate, so there
// is no pronoun referring to a nonexistent target. This reverses wording only,
// never the evaluated relation (notably Inside and Occluded are asymmetric).
func describeChineseZeroCount(p CountComparison, target string) (string, error) {
	var modifier string
	switch p.Relation {
	case scene.LeftOf:
		modifier = "与它在同一排且位于它右侧的"
	case scene.RightOf:
		modifier = "与它在同一排且位于它左侧的"
	case scene.Above:
		modifier = "与它在同一列且位于它下方的"
	case scene.Below:
		modifier = "与它在同一列且位于它上方的"
	case scene.Near:
		modifier = "靠近它的"
	case scene.Far:
		modifier = "远离它的"
	case scene.Inside:
		modifier = "内部包含它的"
	case scene.Overlapping:
		modifier = "与它重叠的"
	case scene.Occluded:
		modifier = "遮挡了它一部分的"
	case scene.LargerThan:
		modifier = "面积比它小的"
	case scene.SmallerThan:
		modifier = "面积比它大的"
	default:
		return "", fmt.Errorf("question: unsupported relation %q", p.Relation)
	}
	var comparison string
	switch p.Op {
	case Equal, AtMost:
		comparison = "为零"
	case AtLeast:
		comparison = "不少于零"
	case Greater:
		comparison = "大于零"
	case Less:
		comparison = "小于零"
	default:
		return "", fmt.Errorf("question: invalid comparison")
	}
	return "符合以下条件：" + modifier + target + "数量" + comparison, nil
}
