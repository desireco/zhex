package lang

import (
	"math"
	"math/big"
	"math/bits"
	"strconv"
	"strings"
)

// i128 is Int's runtime width (#167): a signed 128-bit two's-complement
// value carried as an inline lo/hi limb pair so ordinary values stay
// allocation-free on the interpreter's hot paths. Division and decimal
// rendering of wide values fall back to math/big; add, subtract,
// multiply, compare, and the int64 fast paths never allocate.
type i128 struct {
	lo uint64
	hi int64
}

var (
	i128Max = i128{lo: math.MaxUint64, hi: math.MaxInt64}
	i128Min = i128{lo: 0, hi: math.MinInt64}
)

func i128from64(n int64) i128 { return i128{lo: uint64(n), hi: n >> 63} }

// fits64 reports whether the value shares int64 range; int64 extracts it.
// Construction sites that measure lengths or indexes check this before
// narrowing.
func (v i128) fits64() bool {
	return v.hi == 0 && v.lo <= math.MaxInt64 || v.hi == -1 && v.lo >= 1<<63
}
func (v i128) int64() int64   { return int64(v.lo) }
func (v i128) negative() bool { return v.hi < 0 }
func (v i128) isZero() bool   { return v.lo == 0 && v.hi == 0 }

func i128cmp(a, b i128) int {
	if a.hi != b.hi {
		if a.hi < b.hi {
			return -1
		}
		return 1
	}
	if a.lo != b.lo {
		if a.lo < b.lo {
			return -1
		}
		return 1
	}
	return 0
}

func i128add(a, b i128) (i128, bool) {
	lo, carry := bits.Add64(a.lo, b.lo, 0)
	hi, _ := bits.Add64(uint64(a.hi), uint64(b.hi), carry)
	sum := i128{lo: lo, hi: int64(hi)}
	// Same-sign operands are the only overflow case: the result's sign
	// bit must keep their sign.
	overflow := a.negative() == b.negative() && sum.negative() != a.negative()
	return sum, overflow
}

func i128sub(a, b i128) (i128, bool) {
	lo, borrow := bits.Sub64(a.lo, b.lo, 0)
	hi, _ := bits.Sub64(uint64(a.hi), uint64(b.hi), borrow)
	diff := i128{lo: lo, hi: int64(hi)}
	overflow := a.negative() != b.negative() && diff.negative() != a.negative()
	return diff, overflow
}

// i128twos flips the sign of any value except i128Min.
func i128twos(v i128) i128 {
	lo := ^v.lo + 1
	var carry uint64
	if v.lo == 0 {
		carry = 1
	}
	return i128{lo: lo, hi: int64(^uint64(v.hi) + carry)}
}

func i128neg(a i128) (i128, bool) {
	if a == i128Min {
		return i128{}, false
	}
	return i128twos(a), true
}

// i128mag returns the magnitude of v; callers negate first, so v is
// never i128Min.
func i128mag(v i128) i128 {
	if v.negative() {
		return i128twos(v)
	}
	return v
}

// i128mul multiplies through a 2×2 limb schoolbook on magnitudes and,
// like i128add and i128sub, reports overflow when any bit above 127
// survives or the signed range edge is crossed.
func i128mul(a, b i128) (i128, bool) {
	neg := a.negative() != b.negative()
	am, bm := i128mag(a), i128mag(b)
	h0, l0 := bits.Mul64(am.lo, bm.lo)
	h1, l1 := bits.Mul64(am.lo, uint64(bm.hi))
	h2, l2 := bits.Mul64(uint64(am.hi), bm.lo)
	h3, l3 := bits.Mul64(uint64(am.hi), uint64(bm.hi))
	mid, c1 := bits.Add64(h0, l1, 0)
	mid, c2 := bits.Add64(mid, l2, 0)
	// Bits above 127 are h1 + h2 + carries + a1b1 in full.
	top, c3 := bits.Add64(h1, h2, c1+c2)
	top, c4 := bits.Add64(top, h3, 0)
	if top != 0 || c3 != 0 || c4 != 0 || l3 != 0 {
		return i128{}, true
	}
	magnitude := i128{lo: l0, hi: int64(mid)}
	if neg {
		// −2^127 is representable; every other magnitude ≥ 2^127 is not.
		if uint64(magnitude.hi) > 1<<63 || uint64(magnitude.hi) == 1<<63 && magnitude.lo != 0 {
			return i128{}, true
		}
		return i128twos(magnitude), false
	}
	if uint64(magnitude.hi) >= 1<<63 {
		return i128{}, true
	}
	return magnitude, false
}

// i128divmod truncates toward zero, matching / and %. The divisor is
// nonzero and the min/-1 overflow case is the caller's to reject.
func i128divmod(a, b i128) (i128, i128) {
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(i128toBig(a), i128toBig(b), r)
	return i128fromBig(q), i128fromBig(r)
}

func i128toBig(v i128) *big.Int {
	lo := new(big.Int).SetUint64(v.lo)
	hi := new(big.Int).SetUint64(uint64(v.hi))
	n := lo.Or(lo, hi.Lsh(hi, 64))
	if v.negative() {
		return n.Sub(n, new(big.Int).Lsh(big.NewInt(1), 128))
	}
	return n
}

func i128fromBig(n *big.Int) i128 {
	mask := new(big.Int).SetUint64(math.MaxUint64)
	lo := new(big.Int).And(n, mask)
	hi := new(big.Int).And(new(big.Int).Rsh(n, 64), mask)
	return i128{lo: lo.Uint64(), hi: int64(hi.Uint64())}
}

// i128string renders decimal text; values that fit int64 take the
// strconv fast path.
func i128string(v i128) string {
	if v.fits64() {
		return strconv.FormatInt(v.int64(), 10)
	}
	return i128toBig(v).String()
}

// i128parse reads a signed decimal with optional surrounding whitespace,
// reporting success only inside [−2^127, 2^127−1].
func i128parse(text string) (i128, bool) {
	trimmed := strings.TrimSpace(text)
	if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return i128from64(n), true
	}
	n, ok := new(big.Int).SetString(trimmed, 10)
	if !ok {
		return i128{}, false
	}
	limit := new(big.Int).Lsh(big.NewInt(1), 127)
	if n.Cmp(limit) >= 0 || n.Cmp(new(big.Int).Neg(limit)) < 0 {
		return i128{}, false
	}
	return i128fromBig(n), true
}
