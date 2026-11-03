package question

import (
	"strings"
	"testing"

	"github.com/mxmilu666/nyancaptcha/scene"
)

func fixture() *scene.Scene {
	colors := scene.Palette()
	return &scene.Scene{Width: 400, Height: 240, Shapes: []scene.Shape{
		{ID: 1, Z: 1, Kind: scene.Circle, Center: scene.Point{X: 60, Y: 50}, Width: 24, Height: 24, Fill: colors[0]},
		{ID: 2, Z: 2, Kind: scene.Rectangle, Center: scene.Point{X: 150, Y: 50}, Width: 21, Height: 21, Fill: colors[0]},
		{ID: 3, Z: 3, Kind: scene.Circle, Center: scene.Point{X: 260, Y: 50}, Width: 48, Height: 48, Fill: colors[1]},
		{ID: 4, Z: 4, Kind: scene.Rectangle, Center: scene.Point{X: 340, Y: 180}, Width: 42, Height: 42, Fill: colors[1]},
		{ID: 5, Z: 5, Kind: scene.Circle, Center: scene.Point{X: 60, Y: 180}, Width: 24, Height: 24, Fill: colors[2]},
		{ID: 6, Z: 6, Kind: scene.Rectangle, Center: scene.Point{X: 150, Y: 180}, Width: 21, Height: 21, Fill: colors[2]},
	}}
}
func occludedFixture() *scene.Scene {
	s := fixture()
	s.Shapes[1].Center = scene.Point{X: 79, Y: 50}
	return s
}
func TestPredicateAnswers(t *testing.T) {
	s := fixture()
	red := scene.Palette()[0]
	tests := []struct {
		name string
		p    Predicate
		want int
	}{
		{"color", ColorIs{red}, 2}, {"kind", KindIs{scene.Circle}, 3},
		{"and", And{[]Predicate{ColorIs{red}, KindIs{scene.Circle}}}, 1},
		{"or", Or{[]Predicate{ColorIs{red}, KindIs{scene.Circle}}}, 4},
		{"not", Not{ColorIs{red}}, 4}, {"position", Position{Left}, 4},
		{"count same row", CountComparison{Filter: ColorIs{red}, Relation: scene.LeftOf, Op: AtLeast, Value: 1}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, e := Count(s, tt.p)
			if e != nil || n != tt.want {
				t.Fatalf("Count = %d, %v; want %d", n, e, tt.want)
			}
			for _, l := range []Language{English, Chinese} {
				text, e := Render(tt.p, l)
				if e != nil || text == "" {
					t.Fatalf("render: %q %v", text, e)
				}
				if strings.Contains(text, "ID") {
					t.Fatal("identifier leaked")
				}
			}
		})
	}
}

func TestSizePredicateAnswers(t *testing.T) {
	s := fixture()
	// Same-kind targets have clearly separated dimensions. Rectangles are
	// distractors and must never count toward a circle size comparison.
	s.Shapes[1].Width, s.Shapes[1].Height = 12, 12
	s.Shapes[4].Width, s.Shapes[4].Height = 7, 7
	red := scene.Palette()[0]
	p := Related{scene.LargerThan, Reference{Color: &red, Kind: scene.Circle}}
	if n, err := Count(s, p); err != nil || n != 1 {
		t.Fatalf("unique reference: Count = %d, %v; want 1", n, err)
	}
	s.Shapes[1].Kind = scene.Circle
	s.Shapes[5].Width, s.Shapes[5].Height = 6, 6
	for _, p := range []Predicate{
		Related{scene.LargerThan, Reference{Color: &red, All: true}},
		CountComparison{Filter: ColorIs{red}, Relation: scene.LargerThan, Op: Equal, Value: 2},
	} {
		if n, err := Count(s, p); err != nil || n != 1 {
			t.Fatalf("%T: Count = %d, %v; want 1", p, n, err)
		}
	}
}

func TestSizeQuestionsRejectSimilarNonmatches(t *testing.T) {
	s := fixture()
	red := scene.Palette()[0]
	for _, relation := range []scene.Relation{scene.LargerThan, scene.SmallerThan} {
		for _, p := range []Predicate{
			Related{relation, Reference{Color: &red, Kind: scene.Circle}},
			Related{relation, Reference{Color: &red, All: true}},
			CountComparison{Filter: ColorIs{red}, Relation: relation, Op: AtLeast, Value: 1},
		} {
			if _, err := Count(s, p); err == nil {
				t.Fatalf("%T %s accepted similar-sized candidates", p, relation)
			}
		}
	}
}

func TestDirectionalQuestionCountsOnlyAlignedShapes(t *testing.T) {
	colors := scene.Palette()
	// Approximate the reported two-row image: three shapes beside the blue
	// star, with three more diagonally above-left that must not be counted.
	shapes := []scene.Shape{
		{ID: 1, Z: 1, Kind: scene.Rectangle, Center: scene.Point{X: 98, Y: 110}, Width: 50, Height: 64, Rotation: .7, Fill: colors[3]},
		{ID: 2, Z: 2, Kind: scene.Hexagon, Center: scene.Point{X: 256, Y: 88}, Width: 90, Height: 90, Fill: colors[5]},
		{ID: 3, Z: 3, Kind: scene.Circle, Center: scene.Point{X: 394, Y: 102}, Width: 82, Height: 82, Fill: colors[0]},
		{ID: 4, Z: 4, Kind: scene.Pentagon, Center: scene.Point{X: 550, Y: 128}, Width: 52, Height: 52, Fill: colors[4]},
		{ID: 5, Z: 5, Kind: scene.Circle, Center: scene.Point{X: 78, Y: 342}, Width: 48, Height: 48, Fill: colors[4]},
		{ID: 6, Z: 6, Kind: scene.Hexagon, Center: scene.Point{X: 240, Y: 316}, Width: 52, Height: 52, Fill: colors[0]},
		{ID: 7, Z: 7, Kind: scene.Circle, Center: scene.Point{X: 418, Y: 335}, Width: 82, Height: 82, Fill: colors[3]},
		{ID: 8, Z: 8, Kind: scene.Star, Center: scene.Point{X: 556, Y: 308}, Width: 120, Height: 120, Fill: colors[1]},
	}
	s := &scene.Scene{Width: 648, Height: 432, Shapes: shapes}
	p := Related{Relation: scene.LeftOf, Target: Reference{Color: &colors[1], Kind: scene.Star}}
	n, err := Count(s, p)
	if err != nil || n != 3 {
		t.Fatalf("left of star = %d, %v; want 3", n, err)
	}
	text, err := Render(p, Chinese)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(s, Question{Type: Spatial, Text: text, Answer: 3, Expr: p}); err != nil {
		t.Fatal(err)
	}
}
func TestReferenceAndBoundaryRejection(t *testing.T) {
	s := fixture()
	red := scene.Palette()[0]
	missing := scene.Palette()[4]
	for _, r := range []Reference{{Color: &red}, {Color: &missing, All: true}, {}} {
		if _, e := Count(s, Related{scene.LeftOf, r}); e == nil {
			t.Fatalf("accepted invalid reference %+v", r)
		}
	}
	s.Shapes[0].Center.X = 200
	if _, e := Count(s, Position{Left}); e == nil {
		t.Fatal("accepted center on midline")
	}
	s = fixture()
	s.Shapes[2].Width = 26
	s.Shapes[2].Height = 26
	if _, e := Count(s, Related{scene.LargerThan, Reference{Color: &red, Kind: scene.Circle}}); e == nil {
		t.Fatal("accepted ambiguous size")
	}
	p := CountComparison{Filter: ColorIs{missing}, Relation: scene.LeftOf, Op: Equal, Value: 0}
	if _, e := Count(s, p); e == nil {
		t.Fatal("accepted empty quantifier")
	}
}
func TestNoLogicalShortCircuitHidesAmbiguity(t *testing.T) {
	s := fixture()
	s.Shapes[0].Center.X = 200
	p := And{[]Predicate{ColorIs{scene.Palette()[4]}, Position{Left}}}
	if _, e := Count(s, p); e == nil {
		t.Fatal("ambiguity suppressed by false branch")
	}
}
func TestValidateRejectsTamperingAndRedundancy(t *testing.T) {
	s := fixture()
	p := ColorIs{scene.Palette()[0]}
	text, _ := Render(p, English)
	q := Question{Basic, text, 2, p}
	if e := Validate(s, q); e != nil {
		t.Fatal(e)
	}
	q.Answer++
	if Validate(s, q) == nil {
		t.Fatal("accepted false answer")
	}
	q.Answer = 2
	q.Text = "How many circles?"
	if Validate(s, q) == nil {
		t.Fatal("accepted mismatched rendering")
	}
	dup := And{[]Predicate{p, p}}
	text, _ = Render(dup, English)
	if Validate(s, Question{Compound, text, 2, dup}) == nil {
		t.Fatal("accepted duplicate filter")
	}
}
func TestBuildAllTypes(t *testing.T) {
	for _, typ := range Types() {
		t.Run(string(typ), func(t *testing.T) {
			s := fixture()
			n := 1
			if typ == ColorShape || typ == Compound {
				n = 2
			}
			if typ == Occlusion {
				s = occludedFixture()
			}
			if typ == Size {
				s.Shapes[0].Width, s.Shapes[0].Height = 14, 14
			}
			for _, lang := range []Language{English, Chinese} {
				q := mustBuild(t, s, typ, n, lang)
				if q.Type != typ || q.Expr.Conditions() != n {
					t.Fatalf("wrong type/difficulty: %+v", q)
				}
				if e := Validate(s, q); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
}
func TestBuildLimitedConfigurations(t *testing.T) {
	s := fixture()
	for i := range s.Shapes {
		s.Shapes[i].Fill = scene.Palette()[0]
	}
	q := mustBuild(t, s, Basic, 1, English)
	if _, ok := q.Expr.(KindIs); !ok {
		t.Fatal("single-color scene did not use kind")
	}
	for i := range s.Shapes {
		s.Shapes[i].Kind = scene.Circle
	}
	if _, e := Build(s, []Type{Basic}, 1, English); e == nil {
		t.Fatal("accepted trivial single-color single-kind basic question")
	}
	mustBuild(t, s, Spatial, 1, English)
}
func TestOcclusionStatusUsesGeometry(t *testing.T) {
	s := occludedFixture()
	for i := range s.Shapes {
		s.Shapes[i].Occlusion = 0
		s.Shapes[i].VisibleArea = 0
	}
	n, e := Count(s, OcclusionStatus{true})
	if e != nil || n != 1 {
		t.Fatalf("hidden = %d, %v", n, e)
	}
	n, e = Count(s, OcclusionStatus{false})
	if e != nil || n != 5 {
		t.Fatalf("visible = %d, %v", n, e)
	}
}
func TestBuildRejectsInvalidOptions(t *testing.T) {
	s := fixture()
	for _, n := range []int{-1, 0, 4} {
		if _, e := Build(s, nil, n, English); e == nil {
			t.Fatal("invalid condition count accepted")
		}
	}
	if _, e := Build(s, []Type{"unknown"}, 1, English); e == nil {
		t.Fatal("unknown type accepted")
	}
	if _, e := Build(s, nil, 1, "unknown"); e == nil {
		t.Fatal("unknown language accepted")
	}
}
func FuzzBooleanCount(f *testing.F) {
	f.Add(uint8(0), uint8(1))
	f.Add(uint8(2), uint8(0))
	f.Fuzz(func(t *testing.T, colorIndex, kindIndex uint8) {
		s := fixture()
		palette := scene.Palette()
		a := ColorIs{palette[int(colorIndex)%len(palette)]}
		b := KindIs{scene.Kinds()[int(kindIndex)%len(scene.Kinds())]}
		na, e := Count(s, a)
		if e != nil {
			t.Fatal(e)
		}
		nn, e := Count(s, Not{a})
		if e != nil || na+nn != len(s.Shapes) {
			t.Fatal("complement law")
		}
		both, e := Count(s, And{[]Predicate{a, b}})
		if e != nil {
			t.Fatal(e)
		}
		either, e := Count(s, Or{[]Predicate{a, b}})
		if e != nil {
			t.Fatal(e)
		}
		nb, _ := Count(s, b)
		if both+either != na+nb {
			t.Fatal("inclusion exclusion law")
		}
	})
}
func FuzzReferenceNonempty(f *testing.F) {
	f.Add("red", true)
	f.Add("unknown", false)
	f.Fuzz(func(t *testing.T, name string, all bool) {
		s := fixture()
		c := scene.Color{Name: name, Chinese: "测试"}
		p := Related{scene.LargerThan, Reference{Color: &c, All: all}}
		_, e := Count(s, p)
		matches := 0
		for _, sh := range s.Shapes {
			if sh.Fill.Name == name {
				matches++
			}
		}
		if (matches == 0 || !all && matches != 1) && e == nil {
			t.Fatal("invalid reference accepted")
		}
	})
}
func mustBuild(t *testing.T, s *scene.Scene, typ Type, n int, l Language) Question {
	t.Helper()
	var err error
	for range 64 {
		q, e := Build(s, []Type{typ}, n, l)
		if e == nil {
			return q
		}
		err = e
	}
	t.Fatalf("no feasible sampled answer: %v", err)
	return Question{}
}
func TestValidateRejectsInvisibleAndAmbiguousColors(t *testing.T) {
	s := fixture()
	p := ColorIs{scene.Palette()[0]}
	text, _ := Render(p, English)
	q := Question{Basic, text, 2, p}
	s.Shapes[1].Fill.Name = "different"
	s.Shapes[1].Fill.Chinese = "不同"
	if Validate(s, q) == nil {
		t.Fatal("accepted identical colors with different names")
	}
	s = fixture()
	s.Shapes[1].Center = s.Shapes[0].Center
	s.Shapes[1].Width = 30
	s.Shapes[1].Height = 30
	if Validate(s, q) == nil {
		t.Fatal("accepted hidden target")
	}
}
func TestThreeMeaningfulConditions(t *testing.T) {
	s := fixture()
	s.Shapes[1].Fill = scene.Palette()[1]
	s.Shapes[1].Kind = scene.Circle
	s.Shapes[1].Center = scene.Point{X: 150, Y: 90}
	s.Shapes[2].Fill = scene.Palette()[0]
	s.Shapes[4].Fill = scene.Palette()[0]
	s.Shapes[5].Fill = scene.Palette()[0]
	s.Shapes[5].Center = scene.Point{X: 150, Y: 50}
	p := And{[]Predicate{ColorIs{scene.Palette()[0]}, KindIs{scene.Circle}, Position{Top}}}
	text, _ := Render(p, English)
	n, e := Count(s, p)
	if e != nil {
		t.Fatal(e)
	}
	if e = Validate(s, Question{Compound, text, n, p}); e != nil {
		t.Fatal(e)
	}
}
