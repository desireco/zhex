package zhexrt

import "testing"

// TestAndOrXorAreLimbwiseTwosComplement pins the three bitwise words on the
// limb edges: two's complement over the full width, no failure modes.
func TestAndOrXorAreLimbwiseTwosComplement(t *testing.T) {
	wide := Int{Lo: 0, Hi: 1} // 2^64
	for _, tc := range []struct {
		name    string
		a, b    Int
		and, or Int
		xor     Int
	}{
		{"small", I64(12), I64(10), I64(8), I64(14), I64(6)},
		{"all ones masks", I64(-1), I64(255), I64(255), I64(-1), I64(-256)},
		{"min and minus one", Min, I64(-1), Min, I64(-1), Max},
		{"wide limbs", wide, I64(-1), wide, I64(-1), Int{Lo: ^uint64(0), Hi: -2}}, // xor = -(2^64+1)
	} {
		t.Run(tc.name, func(t *testing.T) {
			if And(tc.a, tc.b) != tc.and || Or(tc.a, tc.b) != tc.or || Xor(tc.a, tc.b) != tc.xor {
				t.Fatalf("And/Or/Xor(%s, %s) = (%s, %s, %s), want (%s, %s, %s)",
					Str(tc.a), Str(tc.b), Str(And(tc.a, tc.b)), Str(Or(tc.a, tc.b)), Str(Xor(tc.a, tc.b)),
					Str(tc.and), Str(tc.or), Str(tc.xor))
			}
		})
	}
}

// TestShiftLeftChecked pins int.shift-left's shared body: the mathematical
// a·2^n, integer-overflow when it leaves the signed range, zero for zero at
// any count, int-shift-count for negative counts.
func TestShiftLeftChecked(t *testing.T) {
	pos := At("t.zhex", 1, 1)
	for _, tc := range []struct {
		name string
		a, n Int
		want Int
		fail string
	}{
		{"count zero", I64(21), I64(0), I64(21), ""},
		{"count one", I64(3), I64(1), I64(6), ""},
		{"count sixty-three", I64(1), I64(63), Int{Lo: 1 << 63, Hi: 0}, ""},
		{"count sixty-four crosses limbs", I64(1), I64(64), Int{Lo: 0, Hi: 1}, ""},
		{"count one twenty-six fits", I64(1), I64(126), Int{Lo: 0, Hi: 1 << 62}, ""},
		{"count one twenty-seven overflows", I64(1), I64(127), Int{}, "integer-overflow"},
		{"count past width overflows nonzero", I64(1), I64(200), Int{}, "integer-overflow"},
		{"zero at any count stays zero", I64(0), I64(200), I64(0), ""},
		{"minus one at one twenty-seven is min", I64(-1), I64(127), Min, ""},
		{"minus one past width overflows", I64(-1), I64(128), Int{}, "integer-overflow"},
		{"negative count fails", I64(1), I64(-1), Int{}, "int-shift-count"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got Int
			fail := func() (code string) {
				defer func() {
					if r := recover(); r != nil {
						if e, ok := r.(*Error); ok {
							code = e.Code
							return
						}
						panic(r)
					}
				}()
				got = ShiftLeft(pos, tc.a, tc.n)
				return ""
			}()
			if fail != tc.fail {
				t.Fatalf("ShiftLeft(%s, %s) failed %q, want %q", Str(tc.a), Str(tc.n), fail, tc.fail)
			}
			if fail == "" && got != tc.want {
				t.Fatalf("ShiftLeft(%s, %s) = %s, want %s", Str(tc.a), Str(tc.n), Str(got), Str(tc.want))
			}
		})
	}
}

// TestShiftRightSignKeeping pins int.shift-right's shared body: arithmetic
// shift that keeps the sign, counts past the width clamped to the
// mathematical result (0, or −1 for negative values), int-shift-count for
// negative counts.
func TestShiftRightSignKeeping(t *testing.T) {
	pos := At("t.zhex", 1, 1)
	for _, tc := range []struct {
		name string
		a, n Int
		want Int
		fail string
	}{
		{"count zero", I64(21), I64(0), I64(21), ""},
		{"halves toward minus infinity", I64(-9), I64(1), I64(-5), ""},
		{"count sixty-three crosses limbs", Int{Lo: 0, Hi: 1}, I64(63), I64(2), ""},
		{"count sixty-four drops the low limb", Int{Lo: 0, Hi: 1}, I64(64), I64(1), ""},
		{"count past limb leaves zero", Int{Lo: 0, Hi: 1}, I64(65), I64(0), ""},
		{"minus one keeps every bit at one twenty-seven", I64(-1), I64(127), I64(-1), ""},
		{"minus one clamps past width", I64(-1), I64(128), I64(-1), ""},
		{"min clamps past width", Min, I64(200), I64(-1), ""},
		{"positive clamps past width", I64(5), I64(128), I64(0), ""},
		{"negative count fails", I64(8), I64(-3), Int{}, "int-shift-count"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got Int
			fail := func() (code string) {
				defer func() {
					if r := recover(); r != nil {
						if e, ok := r.(*Error); ok {
							code = e.Code
							return
						}
						panic(r)
					}
				}()
				got = ShiftRight(pos, tc.a, tc.n)
				return ""
			}()
			if fail != tc.fail {
				t.Fatalf("ShiftRight(%s, %s) failed %q, want %q", Str(tc.a), Str(tc.n), fail, tc.fail)
			}
			if fail == "" && got != tc.want {
				t.Fatalf("ShiftRight(%s, %s) = %s, want %s", Str(tc.a), Str(tc.n), Str(got), Str(tc.want))
			}
		})
	}
}
