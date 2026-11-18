package lang

import (
	"math"
	"math/big"
	"math/rand"
	"testing"
)

func TestCheckedArithmeticAgainstIntegerMath(t *testing.T) {
	values := []i128{
		i128Min,
		i128from64(math.MinInt64),
		i128from64(math.MinInt64 + 1),
		i128from64(-3037000500),
		i128from64(-2), i128from64(-1), i128from64(0), i128from64(1), i128from64(2),
		i128from64(3037000500),
		i128from64(math.MaxInt64 - 1),
		i128from64(math.MaxInt64),
		i128Max,
		i128from64(-math.MaxInt64),
	}
	random := rand.New(rand.NewSource(1))
	for i := 0; i < 30; i++ {
		values = append(values, i128from64(int64(random.Uint64())))
	}
	for i := 0; i < 8; i++ {
		lo, hi := random.Uint64(), random.Uint64()>>1
		values = append(values, i128{lo: lo, hi: int64(hi)}, i128{lo: lo, hi: -int64(hi) - 1})
	}
	limit := new(big.Int).Lsh(big.NewInt(1), 127) // 2^127
	for _, a := range values {
		for _, b := range values {
			for _, operator := range []string{"+", "-", "*", "/", "%"} {
				if b == (i128{}) && (operator == "/" || operator == "%") {
					continue
				}
				left, right := i128toBig(a), i128toBig(b)
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
					if err != nil || got.number != i128fromBig(want) {
						t.Fatalf("%s %s %s: got %s (%v), want %s", i128string(a), operator, i128string(b), i128string(got.number), err, want)
					}
				} else if err == nil {
					t.Fatalf("%s %s %s: overflow was not detected", i128string(a), operator, i128string(b))
				}
			}
		}
	}
}
