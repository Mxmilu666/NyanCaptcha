package scene

import "testing"

func FuzzOcclusionUnion(f *testing.F) {
	f.Add(uint8(20), uint8(35), uint8(0), uint8(6))
	f.Add(uint8(40), uint8(40), uint8(2), uint8(2))
	f.Fuzz(func(t *testing.T, x, y, ka, kb uint8) {
		a := testShape(Kinds()[int(ka)%7], 1, 60, 60, 32, 32)
		b := testShape(Kinds()[int(kb)%7], 2, 40+float64(x%50), 40+float64(y%50), 32, 32)
		s := testScene(a, b)
		if err := Analyze(s); err != nil {
			t.Fatal(err)
		}
		if s.Shapes[0].Occlusion < 0 || s.Shapes[0].Occlusion > 1 || s.Shapes[1].Occlusion != 0 {
			t.Fatal("invalid Z coverage")
		}
		match, amb := Relate(s, a, b, Overlapping)
		reverse, ra := Relate(s, b, a, Overlapping)
		if match != reverse || amb != ra {
			t.Fatal("asymmetric intersection")
		}
		before := s.Shapes[0].Occlusion
		s.Shapes = append(s.Shapes, b)
		s.Shapes[2].ID = 3
		s.Shapes[2].Z = 3
		if err := Analyze(s); err != nil {
			t.Fatal(err)
		}
		if s.Shapes[0].Occlusion != before {
			t.Fatal("duplicate occluder changed union")
		}
	})
}
