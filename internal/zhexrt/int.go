// Package zhexrt is the runtime behind zhex compile --target=native: the
// value model and builtin implementations generated programs call. The
// code is extracted from internal/lang (the interpreter's own
// implementations), so native and interpreted runs agree by construction;
// the differential gates enforce it.
package zhexrt

import (
	"math"
	"math/big"
	"math/bits"
	"strconv"
	"strings"
)

// Int is Zhex Int's width (#167): a signed 128-bit two's-complement value
// carried as an inline lo/hi limb pair so ordinary values stay
// allocation-free on the hot paths. Division and decimal rendering of wide
// values fall back to math/big; add, subtract, multiply, compare, and the
// int64 fast paths never allocate. Moved unchanged from the interpreter's
// i128 (internal/lang/i128.go).
type Int struct {
	Lo uint64
	Hi int64
}

// Max and Min are the signed 128-bit bounds.
var (
	Max = Int{Lo: math.MaxUint64, Hi: math.MaxInt64}
	Min = Int{Lo: 0, Hi: math.MinInt64}
)

// I64 widens a signed 64-bit value.
func I64(n int64) Int { return Int{Lo: uint64(n), Hi: n >> 63} }

// Fits64 reports whether the value shares int64 range; AsInt64 extracts
// it. Construction sites that measure lengths or indexes check this before
// narrowing.
func (v Int) Fits64() bool {
	return v.Hi == 0 && v.Lo <= math.MaxInt64 || v.Hi == -1 && v.Lo >= 1<<63
}
func (v Int) AsInt64() int64 { return int64(v.Lo) }
func (v Int) Negative() bool { return v.Hi < 0 }
func (v Int) IsZero() bool   { return v.Lo == 0 && v.Hi == 0 }

func Cmp(a, b Int) int {
	if a.Hi != b.Hi {
		if a.Hi < b.Hi {
			return -1
		}
		return 1
	}
	if a.Lo != b.Lo {
		if a.Lo < b.Lo {
			return -1
		}
		return 1
	}
	return 0
}

func AddOvf(a, b Int) (Int, bool) {
	lo, carry := bits.Add64(a.Lo, b.Lo, 0)
	hi, _ := bits.Add64(uint64(a.Hi), uint64(b.Hi), carry)
	sum := Int{Lo: lo, Hi: int64(hi)}
	// Same-sign operands are the only overflow case: the result's sign bit
	// must keep their sign.
	overflow := a.Negative() == b.Negative() && sum.Negative() != a.Negative()
	return sum, overflow
}

func SubOvf(a, b Int) (Int, bool) {
	lo, borrow := bits.Sub64(a.Lo, b.Lo, 0)
	hi, _ := bits.Sub64(uint64(a.Hi), uint64(b.Hi), borrow)
	diff := Int{Lo: lo, Hi: int64(hi)}
	overflow := a.Negative() != b.Negative() && diff.Negative() != a.Negative()
	return diff, overflow
}

// twos flips the sign of any value except Min.
func twos(v Int) Int {
	lo := ^v.Lo + 1
	var carry uint64
	if v.Lo == 0 {
		carry = 1
	}
	return Int{Lo: lo, Hi: int64(^uint64(v.Hi) + carry)}
}

func NegOvf(a Int) (Int, bool) {
	if a == Min {
		return Int{}, false
	}
	return twos(a), true
}

// mag returns the magnitude of v; callers negate first, so v is never Min.
func mag(v Int) Int {
	if v.Negative() {
		return twos(v)
	}
	return v
}

// Mul multiplies through a 2×2 limb schoolbook on magnitudes and, like Add
// and Sub, reports overflow when any bit above 127 survives or the signed
// range edge is crossed.
func MulOvf(a, b Int) (Int, bool) {
	neg := a.Negative() != b.Negative()
	am, bm := mag(a), mag(b)
	h0, l0 := bits.Mul64(am.Lo, bm.Lo)
	h1, l1 := bits.Mul64(am.Lo, uint64(bm.Hi))
	h2, l2 := bits.Mul64(uint64(am.Hi), bm.Lo)
	h3, l3 := bits.Mul64(uint64(am.Hi), uint64(bm.Hi))
	mid, c1 := bits.Add64(h0, l1, 0)
	mid, c2 := bits.Add64(mid, l2, 0)
	// Bits above 127 are h1 + h2 + carries + a1b1 in full. bits.Add64's
	// carry-in must be 0 or 1, so c1+c2 joins the sum as an operand.
	top, c3 := bits.Add64(h1, h2, 0)
	top, c4 := bits.Add64(top, h3, 0)
	top, c5 := bits.Add64(top, c1+c2, 0)
	if top != 0 || c3 != 0 || c4 != 0 || c5 != 0 || l3 != 0 {
		return Int{}, true
	}
	magnitude := Int{Lo: l0, Hi: int64(mid)}
	if neg {
		// −2^127 is representable; every other magnitude ≥ 2^127 is not.
		if uint64(magnitude.Hi) > 1<<63 || uint64(magnitude.Hi) == 1<<63 && magnitude.Lo != 0 {
			return Int{}, true
		}
		return twos(magnitude), false
	}
	if uint64(magnitude.Hi) >= 1<<63 {
		return Int{}, true
	}
	return magnitude, false
}

// And, Or, and Xor are the bitwise words: two's complement over the full
// width, limbwise — no failure modes exist for bit operations.
func And(a, b Int) Int { return Int{Lo: a.Lo & b.Lo, Hi: a.Hi & b.Hi} }
func Or(a, b Int) Int  { return Int{Lo: a.Lo | b.Lo, Hi: a.Hi | b.Hi} }
func Xor(a, b Int) Int { return Int{Lo: a.Lo ^ b.Lo, Hi: a.Hi ^ b.Hi} }

// shiftCount splits a nonnegative count for the limb shift helpers:
// ok is false when the count is the width or more, where the clamped
// mathematical result applies instead.
func shiftCount(n Int) (uint, bool) {
	if n.Hi != 0 || n.Lo >= 128 {
		return 0, false
	}
	return uint(n.Lo), true
}

// shl truncates a left shift over the two's-complement width; the caller
// checks exactness (shift-left) — shifts by themselves never wrap silently
// in the language.
func shl(a Int, n uint) Int {
	if n >= 128 {
		return Int{}
	}
	if n >= 64 {
		return Int{Lo: 0, Hi: int64(a.Lo << (n - 64))}
	}
	return Int{Lo: a.Lo << n, Hi: int64(uint64(a.Hi)<<n | a.Lo>>(64-n))}
}

// shr is the arithmetic (sign-keeping) right shift over the full width.
func shr(a Int, n uint) Int {
	if n >= 128 {
		if a.Negative() {
			return Int{Lo: ^uint64(0), Hi: -1}
		}
		return Int{}
	}
	if n >= 64 {
		return Int{Lo: uint64(a.Hi >> (n - 64)), Hi: a.Hi >> 63}
	}
	return Int{Lo: a.Lo>>n | uint64(a.Hi)<<(64-n), Hi: a.Hi >> n}
}

// DivMod truncates toward zero, matching / and %. The divisor is nonzero
// and the Min/-1 overflow case is the caller's to reject.
func DivMod(a, b Int) (Int, Int) {
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(ToBig(a), ToBig(b), r)
	return FromBig(q), FromBig(r)
}

func ToBig(v Int) *big.Int {
	lo := new(big.Int).SetUint64(v.Lo)
	hi := new(big.Int).SetUint64(uint64(v.Hi))
	n := lo.Or(lo, hi.Lsh(hi, 64))
	if v.Negative() {
		return n.Sub(n, new(big.Int).Lsh(big.NewInt(1), 128))
	}
	return n
}

func FromBig(n *big.Int) Int {
	mask := new(big.Int).SetUint64(math.MaxUint64)
	lo := new(big.Int).And(n, mask)
	hi := new(big.Int).And(new(big.Int).Rsh(n, 64), mask)
	return Int{Lo: lo.Uint64(), Hi: int64(hi.Uint64())}
}

// Str renders decimal text; values that fit int64 take the strconv fast
// path. String satisfies fmt for the rare %v site.
func Str(v Int) string {
	if v.Fits64() {
		return strconv.FormatInt(v.AsInt64(), 10)
	}
	return ToBig(v).String()
}

func (v Int) String() string { return Str(v) }

// Parse reads a signed decimal with optional surrounding whitespace,
// reporting success only inside [−2^127, 2^127−1].
func Parse(text string) (Int, bool) {
	trimmed := strings.TrimSpace(text)
	if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return I64(n), true
	}
	n, ok := new(big.Int).SetString(trimmed, 10)
	if !ok {
		return Int{}, false
	}
	limit := new(big.Int).Lsh(big.NewInt(1), 127)
	if n.Cmp(limit) >= 0 || n.Cmp(new(big.Int).Neg(limit)) < 0 {
		return Int{}, false
	}
	return FromBig(n), true
}
