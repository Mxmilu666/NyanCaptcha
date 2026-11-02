package random

import "testing"

func TestBounds(t *testing.T) {
	for _, n := range []int{1, 2, 3, 7, 256, 1000001} {
		for range 100 {
			v, err := Int(n)
			if err != nil || v < 0 || v >= n {
				t.Fatalf("Int(%d)=%d,%v", n, v, err)
			}
		}
	}
	for _, n := range []int{0, -1} {
		if _, err := Int(n); err == nil {
			t.Fatalf("accepted %d", n)
		}
	}
	for range 100 {
		v, err := Float()
		if err != nil || v < 0 || v >= 1 {
			t.Fatalf("Float=%g,%v", v, err)
		}
	}
}
