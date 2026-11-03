package scene

import (
	"math"
	"sync"
	"testing"
)

func testShape(kind Kind, id int, x, y, w, h float64) Shape {
	return Shape{ID: id, Kind: kind, Center: Point{x, y}, Width: w, Height: h, Fill: Palette()[0], Z: id}
}
func testScene(shapes ...Shape) *Scene { return &Scene{Width: 200, Height: 160, Shapes: shapes} }

func TestShapeGeometry(t *testing.T) {
	for _, kind := range Kinds() {
		t.Run(string(kind), func(t *testing.T) {
			s := testShape(kind, 1, 80, 70, 40, 40)
			if !Contains(s, s.Center) || Contains(s, Point{101, 70}) {
				t.Fatal("incorrect center or exterior membership")
			}
			if Area(s) <= 0 {
				t.Fatal("nonpositive area")
			}
			vs := Vertices(s)
			want := map[Kind]int{Circle: 0, Ellipse: 0, Rectangle: 4, Triangle: 3, Pentagon: 5, Hexagon: 6, Star: 10}[kind]
			if len(vs) != want {
				t.Fatalf("vertices=%d want %d", len(vs), want)
			}
			for _, v := range vs {
				if !Contains(s, v) {
					t.Fatalf("vertex outside: %+v", v)
				}
			}
			if len(vs) > 0 {
				twice := 0.0
				for i, a := range vs {
					b := vs[(i+1)%len(vs)]
					twice += a.X*b.Y - b.X*a.Y
				}
				if math.Abs(twice/2-Area(s)) > 1e-8 {
					t.Fatalf("shoelace area=%g analytic=%g", twice/2, Area(s))
				}
				vs[0].X = 0
				if Vertices(s)[0].X == 0 {
					t.Fatal("aliased vertices")
				}
			}
			original := Area(s)
			s.Rotation = .73
			if !Contains(s, s.Center) || Area(s) != original {
				t.Fatal("rotation changes area or center membership")
			}
		})
	}
}

func TestAnalyticBoundaries(t *testing.T) {
	circle := testShape(Circle, 1, 60, 60, 40, 40)
	if !Contains(circle, Point{80, 60}) || Contains(circle, Point{80.0001, 60}) || Contains(circle, Point{79, 79}) {
		t.Fatal("circle boundary is not analytic")
	}
	ellipse := testShape(Ellipse, 2, 60, 60, 60, 20)
	ellipse.Rotation = math.Pi / 2
	if !Contains(ellipse, Point{60, 89}) || Contains(ellipse, Point{71, 60}) {
		t.Fatal("rotated ellipse membership")
	}
	triangle := testShape(Triangle, 3, 60, 60, 40, 40)
	if Contains(triangle, Point{79, 41}) {
		t.Fatal("polygon bounding box used as fill")
	}
	star := testShape(Star, 4, 60, 60, 40, 40)
	if Contains(star, Point{60, 77}) {
		t.Fatal("star concavity ignored")
	}
	rectangle := testShape(Rectangle, 5, 60, 60, 40, 20)
	rectangle.Rotation = math.Pi / 2
	if !Contains(rectangle, Point{69, 79}) || Contains(rectangle, Point{71, 60}) {
		t.Fatal("rotated rectangle membership")
	}
}

func TestRelationsAndAmbiguity(t *testing.T) {
	a := testShape(Rectangle, 1, 50, 50, 40, 40)
	cases := []struct {
		name             string
		b                Shape
		relation         Relation
		match, ambiguous bool
	}{
		{"direction true", testShape(Rectangle, 2, 80, 50, 40, 40), LeftOf, true, false},
		{"direction band", testShape(Rectangle, 2, 72, 50, 40, 40), LeftOf, false, true},
		{"direction false", testShape(Rectangle, 2, 55, 50, 40, 40), LeftOf, false, false},
		{"touching", testShape(Rectangle, 2, 90, 50, 40, 40), Overlapping, false, false},
		{"small overlap", testShape(Rectangle, 2, 89, 50, 40, 40), Overlapping, false, true},
		{"overlap", testShape(Rectangle, 2, 80, 50, 40, 40), Overlapping, true, false},
		{"inside", testShape(Rectangle, 2, 50, 50, 60, 60), Inside, true, false},
		{"outside", testShape(Circle, 2, 50, 50, 40, 40), Inside, false, false},
		{"inside band", testShape(Rectangle, 2, 50.5, 50, 40, 40), Inside, false, true},
		{"occluded", testShape(Rectangle, 2, 80, 50, 40, 40), Occluded, true, false},
		{"above paint", testShape(Rectangle, 0, 80, 50, 40, 40), Occluded, false, false},
		{"larger", testShape(Rectangle, 2, 100, 50, 20, 20), LargerThan, true, false},
		{"size band", testShape(Rectangle, 2, 100, 50, 36, 36), LargerThan, false, true},
		{"near", testShape(Rectangle, 2, 80, 50, 40, 40), Near, true, false},
		{"far", testShape(Rectangle, 2, 160, 50, 40, 40), Far, true, false},
		{"unknown", testShape(Rectangle, 2, 80, 50, 40, 40), Relation("unknown"), false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, amb := Relate(testScene(a, tc.b), a, tc.b, tc.relation)
			if got != tc.match || amb != tc.ambiguous {
				t.Fatalf("got (%v,%v), want (%v,%v)", got, amb, tc.match, tc.ambiguous)
			}
		})
	}
}

func TestSizeRelationsRequireClearSeparation(t *testing.T) {
	a := testShape(Rectangle, 1, 50, 50, 40, 40)
	for _, ratio := range []float64{0.25, 0.5, 0.624, 0.625, 0.626, 0.9, 1, 1.1, 1.31, 1.59, 1.6, 1.61, 2, 4} {
		b := testShape(Rectangle, 2, 130, 50, 40*ratio, 40*ratio)
		for _, relation := range []Relation{LargerThan, SmallerThan} {
			wantMatch := ratio < 0.625
			if relation == SmallerThan {
				wantMatch = ratio > 1.6
			}
			wantAmbiguous := ratio >= 0.625 && ratio <= 1.6
			match, ambiguous := Relate(testScene(a, b), a, b, relation)
			if match != wantMatch || ambiguous != wantAmbiguous {
				t.Fatalf("%s with target dimension ratio %g = (%v,%v), want (%v,%v)", relation, ratio, match, ambiguous, wantMatch, wantAmbiguous)
			}
			if match, ambiguous := Relate(testScene(a), a, a, relation); match || ambiguous {
				t.Fatalf("self %s = (%v,%v)", relation, match, ambiguous)
			}
		}
	}
}

func TestSizeRelationsRejectAreaOnlyComparisons(t *testing.T) {
	a := testShape(Rectangle, 1, 60, 60, 40, 40)
	for _, b := range []Shape{
		testShape(Rectangle, 2, 140, 60, 20, 40),
		testShape(Rectangle, 2, 140, 60, 40, 20),
		testShape(Rectangle, 2, 140, 60, 20, 80),
	} {
		for _, relation := range []Relation{LargerThan, SmallerThan} {
			if match, ambiguous := Relate(testScene(a, b), a, b, relation); match || !ambiguous {
				t.Fatalf("accepted incomparable dimensions: %s %+v", relation, b)
			}
		}
	}
	for _, kind := range []Kind{Circle, Triangle, Star} {
		b := testShape(kind, 2, 140, 60, 12, 12)
		for _, relation := range []Relation{LargerThan, SmallerThan} {
			if match, ambiguous := Relate(testScene(a, b), a, b, relation); match || ambiguous {
				t.Fatalf("cross-kind comparison was not excluded: %s %s", relation, kind)
			}
		}
	}
}

func TestUnionOcclusionAndOrdering(t *testing.T) {
	back := testShape(Rectangle, 1, 50, 50, 40, 40)
	front := testShape(Rectangle, 2, 60, 50, 20, 40)
	duplicate := front
	duplicate.ID = 3
	duplicate.Z = 3
	s := testScene(duplicate, back, front)
	if err := Analyze(s); err != nil {
		t.Fatal(err)
	}
	if s.Shapes[1].VisibleArea != 800 || s.Shapes[1].Occlusion != .5 {
		t.Fatalf("union double-counted: %+v", s.Shapes[1])
	}
	if s.Shapes[0].VisibleArea != 800 || s.Shapes[2].VisibleArea != 0 {
		t.Fatal("Z order not respected")
	}
	if err := Validate(s); err == nil {
		t.Fatal("hidden shapes accepted")
	}
	// A circle's box intersects the triangle's box, but actual membership is used.
	c := testShape(Circle, 4, 100, 80, 40, 40)
	r := testShape(Rectangle, 5, 125, 105, 12, 12)
	s = testScene(c, r)
	if err := Analyze(s); err != nil {
		t.Fatal(err)
	}
	if s.Shapes[0].Occlusion != 0 {
		t.Fatal("bounding-box intersection mistaken for actual overlap")
	}
}

func TestValidateSafetyAndNonmutation(t *testing.T) {
	good := testShape(Circle, 1, 60, 60, 40, 40)
	good.VisibleArea = 123
	good.Occlusion = .123
	s := testScene(good)
	if err := Validate(s); err != nil {
		t.Fatal(err)
	}
	if s.Shapes[0] != good {
		t.Fatal("Validate mutated input")
	}
	cases := []struct {
		name   string
		mutate func(*Scene)
	}{
		{"nan", func(s *Scene) { s.Shapes[0].Center.X = math.NaN() }},
		{"infinity", func(s *Scene) { s.Shapes[0].Rotation = math.Inf(1) }},
		{"huge", func(s *Scene) { s.Shapes[0].Width = 1e300 }},
		{"outside", func(s *Scene) { s.Shapes[0].Center.X = 0 }},
		{"small", func(s *Scene) { s.Shapes[0].Width = 2 }},
		{"unknown", func(s *Scene) { s.Shapes[0].Kind = "blob" }},
		{"duplicate id", func(s *Scene) { b := s.Shapes[0]; b.Z++; s.Shapes = append(s.Shapes, b) }},
		{"duplicate z", func(s *Scene) { b := s.Shapes[0]; b.ID++; s.Shapes = append(s.Shapes, b) }},
		{"transparent", func(s *Scene) { s.Shapes[0].Fill.RGBA.A = 100 }},
		{"canvas", func(s *Scene) { s.Width = int(^uint(0) >> 1) }},
		{"empty", func(s *Scene) { s.Shapes = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := testScene(good)
			tc.mutate(s)
			if err := Validate(s); err == nil {
				t.Fatal("malformed scene accepted")
			}
			if err := Analyze(s); err == nil {
				t.Fatal("malformed scene analyzed")
			}
		})
	}
	if Validate(nil) == nil || Analyze(nil) == nil {
		t.Fatal("nil accepted")
	}
	invalid := good
	invalid.Center.X = math.NaN()
	if Contains(invalid, Point{}) || Area(invalid) != 0 || Vertices(invalid) != nil {
		t.Fatal("invalid shape not rejected")
	}
}

func TestConcurrentIndependentGeometry(t *testing.T) {
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := testScene(testShape(Star, 1, 60, 60, 40, 40))
			if err := Analyze(s); err != nil {
				t.Error(err)
			}
			if err := Validate(s); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

func FuzzGeometry(f *testing.F) {
	f.Add(50.0, 50.0, 30.0, 30.0, 0.0, uint8(0))
	f.Add(math.NaN(), math.Inf(1), 1e300, -1.0, math.Inf(-1), uint8(6))
	f.Fuzz(func(t *testing.T, x, y, w, h, rotation float64, k uint8) {
		s := testShape(Kinds()[int(k)%len(Kinds())], 1, x, y, w, h)
		s.Rotation = rotation
		area := Area(s)
		if math.IsNaN(area) || math.IsInf(area, 0) || area < 0 {
			t.Fatalf("invalid area %g", area)
		}
		Contains(s, Point{x, y})
		Vertices(s)
		sc := testScene(s)
		if err := Analyze(sc); err == nil {
			if sc.Shapes[0].Occlusion != 0 || sc.Shapes[0].VisibleArea <= 0 {
				t.Fatal("single shape coverage inconsistent")
			}
		}
	})
}

func BenchmarkContainsStar(b *testing.B) {
	s := testShape(Star, 1, 60, 60, 40, 40)
	b.ReportAllocs()
	for i := range b.N {
		Contains(s, Point{float64(i % 100), 60})
	}
}
func BenchmarkAnalyze(b *testing.B) {
	s := testScene(testShape(Circle, 1, 50, 50, 40, 40), testShape(Star, 2, 80, 50, 40, 40), testShape(Hexagon, 3, 120, 90, 40, 40))
	b.ReportAllocs()
	for range b.N {
		if err := Analyze(s); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkPairIntersection(b *testing.B) {
	a, c := testShape(Circle, 1, 50, 50, 40, 40), testShape(Star, 2, 65, 50, 40, 40)
	s := testScene(a, c)
	b.ReportAllocs()
	for range b.N {
		Relate(s, a, c, Overlapping)
	}
}

func TestVisibilityThresholdAndMeasurement(t *testing.T) {
	back := testShape(Rectangle, 1, 50, 50, 40, 40)
	front := testShape(Rectangle, 2, 70, 50, 24, 40)
	s := testScene(back, front)
	measurements, err := Measure(s)
	if err != nil {
		t.Fatal(err)
	}
	if measurements[0].Area != 1600 || measurements[0].Visible != 1120 {
		t.Fatalf("unexpected coverage: %+v", measurements[0])
	}
	if s.Shapes[0].VisibleArea != 0 {
		t.Fatal("Measure mutated cached metrics")
	}
	if err := Validate(s); err != nil {
		t.Fatalf("exactly 70 percent must pass: %v", err)
	}
	s.Shapes[1].Center.X -= .5
	if err := Validate(s); err == nil {
		t.Fatal("below 70 percent accepted")
	}
}

func TestActualShapeIntersection(t *testing.T) {
	a := testShape(Circle, 1, 60, 60, 40, 40)
	b := testShape(Circle, 2, 90, 90, 40, 40)
	s := testScene(a, b)
	match, ambiguous := Relate(s, a, b, Overlapping)
	if match || ambiguous {
		t.Fatal("diagonal box overlap is not circle overlap")
	}
	b.Center = Point{75, 60}
	s.Shapes[1] = b
	match, ambiguous = Relate(s, a, b, Overlapping)
	reverse, reverseAmbiguous := Relate(s, b, a, Overlapping)
	if !match || ambiguous || match != reverse || ambiguous != reverseAmbiguous {
		t.Fatal("intersection must be symmetric and meaningful")
	}
	if err := Analyze(s); err != nil {
		t.Fatal(err)
	}
	if s.Shapes[0].Occlusion <= 0 || s.Shapes[1].Occlusion != 0 {
		t.Fatal("actual circular occlusion not measured")
	}
}

func TestSamplingWorkLimit(t *testing.T) {
	s := &Scene{Width: 2000, Height: 2000}
	for i := range 8 {
		s.Shapes = append(s.Shapes, testShape(Rectangle, i, 1000, 1000, 1900, 1900))
	}
	if err := Analyze(s); err == nil {
		t.Fatal("unbounded stacked raster work accepted")
	}
}

func TestStrictDirectionThresholds(t *testing.T) {
	a := testShape(Rectangle, 1, 50, 50, 40, 40)
	for _, offset := range []float64{18, 26} {
		b := testShape(Rectangle, 2, 50+offset, 50, 40, 40)
		if match, ambiguous := Relate(testScene(a, b), a, b, LeftOf); match || !ambiguous {
			t.Fatalf("closed ambiguity endpoint %g accepted", offset)
		}
	}
}

func TestDirectionsRequireAlignedRowsAndColumns(t *testing.T) {
	for _, relation := range []Relation{LeftOf, RightOf, Above, Below} {
		for _, tc := range []struct {
			along, across    float64
			match, ambiguous bool
		}{
			{80, 0, true, false},
			{80, 19, true, false},
			{80, 20, false, true},
			{80, 25, false, true},
			{80, 30, false, true},
			{80, 31, false, false},
			{80, 80, false, false},
			{22, 80, false, false},
			{-80, 25, false, false},
		} {
			for _, sign := range []float64{-1, 1} {
				a := testShape(Rectangle, 1, 100, 100, 40, 40)
				dx, dy := tc.along, sign*tc.across
				if relation == RightOf || relation == Below {
					dx = -dx
				}
				if relation == Above || relation == Below {
					dx, dy = dy, dx
				}
				b := testShape(Rectangle, 2, 100+dx, 100+dy, 40, 40)
				match, ambiguous := Relate(testScene(a, b), a, b, relation)
				if match != tc.match || ambiguous != tc.ambiguous {
					t.Fatalf("%s offset (%g,%g) = (%v,%v), want (%v,%v)", relation, dx, dy, match, ambiguous, tc.match, tc.ambiguous)
				}
			}
		}
	}
}
