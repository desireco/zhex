package zhexrt

import (
	"math"
	"testing"
)

// TestMulOvfPinsTheSchoolbookEdges pins multiply's overflow verdicts on the
// limb edges, including the mid-accumulation whose carries (c1 and c2 both
// 1) used to reach bits.Add64 as one carry-in of 2 — past its 0-or-1
// contract. The split keeps the same verdicts: (2^65-1)^2 must overflow.
func TestMulOvfPinsTheSchoolbookEdges(t *testing.T) {
	wide := Int{Lo: math.MaxUint64, Hi: 1} // 2^65 - 1, both mid carries set
	half := Int{Lo: 1 << 63}               // 2^63
	for _, tc := range []struct {
		name      string
		a, b, got Int
		overflow  bool
	}{
		{"small", I64(3), I64(4), I64(12), false},
		{"negative", I64(-3), I64(4), I64(-12), false},
		{"max by one", Max, I64(1), Max, false},
		{"min by one", Min, I64(1), Min, false},
		{"square of 2^63 fits", half, half, Int{Lo: 0, Hi: 1 << 62}, false}, // 2^126
		{"square of 2^65-1 overflows", wide, wide, Int{}, true},
		{"square of max overflows", Max, Max, Int{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, overflow := MulOvf(tc.a, tc.b)
			if got != tc.got || overflow != tc.overflow {
				t.Fatalf("MulOvf(%s, %s) = (%s, %v), want (%s, %v)", Str(tc.a), Str(tc.b), Str(got), overflow, Str(tc.got), tc.overflow)
			}
		})
	}
}
