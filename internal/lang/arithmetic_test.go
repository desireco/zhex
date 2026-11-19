package lang

import (
	"zhex/internal/zhexrt"
	"math"
	"math/big"
	"math/rand"
	"testing"
)

func TestCheckedArithmeticAgainstIntegerMath(t *testing.T) {
	values := []i128{
		i128Min,
		zhexrt.I64(math.MinInt64),
		zhexrt.I64(math.MinInt64 + 1),
		zhexrt.I64(-3037000500),
		zhexrt.I64(-2), zhexrt.I64(-1), zhexrt.I64(0), zhexrt.I64(1), zhexrt.I64(2),
		zhexrt.I64(3037000500),
		zhexrt.I64(math.MaxInt64 - 1),
		zhexrt.I64(math.MaxInt64),
		i128Max,
		zhexrt.I64(-math.MaxInt64),
	}
	random := rand.New(rand.NewSource(1))
	for i := 0; i < 30; i++ {
		values = append(values, zhexrt.I64(int64(random.Uint64())))
	}
	for i := 0; i < 8; i++ {
		lo, hi := random.Uint64(), random.Uint64()>>1
		values = append(values, zhexrt.Int{Lo: lo, Hi: int64(hi)}, zhexrt.Int{Lo: lo, Hi: -int64(hi) - 1})
	}
	limit := new(big.Int).Lsh(big.NewInt(1), 127) // 2^127
	for _, a := range values {
		for _, b := range values {
			for _, operator := range []string{"+", "-", "*", "/", "%"} {
				if b == (zhexrt.Int{}) && (operator == "/" || operator == "%") {
					continue
				}
				left, right := zhexrt.ToBig(a), zhexrt.ToBig(b)
				want := new(big.Int)
				switch operator {
				case "+":
					want.Add(left, right)
				case "-":
					want.Sub(left, right)
				case "*":
					want.Mul(left, right)
				case "/":
					want.Quo(left, right)
				case "%":
					want.Rem(left, right)
				}
				x := &expression{text: operator, typ: intType, left: &expression{typ: intType}}
				got, err := binary(x, value{number: a}, value{number: b})
				// min128 / -1 produces +2^127, outside [−2^127, 2^127−1],
				// so the range test alone catches every overflow case.
				if want.Cmp(limit) < 0 && want.Cmp(new(big.Int).Neg(limit)) >= 0 {
					if err != nil || got.number != zhexrt.FromBig(want) {
						t.Fatalf("%s %s %s: got %s (%v), want %s", zhexrt.Str(a), operator, zhexrt.Str(b), zhexrt.Str(got.number), err, want)
					}
				} else if err == nil {
					t.Fatalf("%s %s %s: overflow was not detected", zhexrt.Str(a), operator, zhexrt.Str(b))
				}
			}
		}
	}
}
