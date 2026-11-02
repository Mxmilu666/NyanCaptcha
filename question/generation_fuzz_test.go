package question

import "testing"

func FuzzQuestionGeneration(f *testing.F) {
	f.Add(uint8(0), uint8(1), uint8(0), uint8(0))
	f.Add(uint8(5), uint8(3), uint8(1), uint8(12))
	f.Fuzz(func(t *testing.T, typ, depth, language, shift uint8) {
		s := fixture()
		s.Shapes[0].Center.X += float64(shift % 20)
		l := English
		if language%2 == 1 {
			l = Chinese
		}
		q, err := Build(s, []Type{Types()[int(typ)%len(Types())]}, int(depth%3)+1, l)
		if err != nil {
			return
		}
		count, err := Count(s, q.Expr)
		if err != nil || count != q.Answer {
			t.Fatalf("generated wrong answer: %d,%v", count, err)
		}
		if err := Validate(s, q); err != nil {
			t.Fatal(err)
		}
		rendered, err := Render(q.Expr, l)
		if err != nil || rendered != q.Text {
			t.Fatal("language/expression mismatch")
		}
	})
}
