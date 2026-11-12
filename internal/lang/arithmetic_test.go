package lang

import (
	"math"
	"math/big"
	"math/rand"
	"testing"
)

func TestCheckedArithmeticAgainstIntegerMath(t *testing.T) {
	values := []int64{math.MinInt64, math.MinInt64 + 1, -3037000500, -2, -1, 0, 1, 2, 3037000500, math.MaxInt64 - 1, math.MaxInt64}
	random := rand.New(rand.NewSource(1))
	for i := 0; i < 30; i++ {
		values = append(values, int64(random.Uint64()))
	}
	for _, a := range values {
		for _, b := range values {
			for _, operator := range []string{"+", "-", "*", "/", "%"} {
				if b == 0 && (operator == "/" || operator == "%") {
					continue
				}
				left, right := big.NewInt(a), big.NewInt(b)
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
				if want.IsInt64() {
					if err != nil || got.number != want.Int64() {
						t.Fatalf("%d %s %d: got %d (%v), want %s", a, operator, b, got.number, err, want)
					}
				} else if err == nil {
					t.Fatalf("%d %s %d: overflow was not detected", a, operator, b)
				}
			}
		}
	}
}
