package lang

import (
	"strings"
	"testing"
)

// TestIntModFloor pins the floored modulo word: the result always has the
// sign of the modulus (Python/Ruby floored semantics), unlike % which
// truncates toward zero; zero results have no sign. Wrap-around indexing is
// the motivating shape.
func TestIntModFloor(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"positive operands agree with %", "print(int.mod-floor(7, 3))\n", "1\n"},
		{"negative value wraps positive", "print(int.mod-floor(-7, 3))\n", "2\n"},
		{"positive value wraps negative under a negative modulus", "print(int.mod-floor(7, -3))\n", "-2\n"},
		{"negative operands agree with %", "print(int.mod-floor(-7, -3))\n", "-1\n"},
		{"exact multiples are zero without a sign", "print(int.mod-floor(6, 3))\nprint(int.mod-floor(-6, 3))\nprint(int.mod-floor(6, -3))\n", "0\n0\n0\n"},
		{"zero value is zero", "print(int.mod-floor(0, 5))\n", "0\n"},
		{"wrap-around indexing stays in bounds", "let letters = [\"a\", \"b\", \"c\"]\nprint(letters[int.mod-floor(-1, 3)])\n", "c\n"},
		{"MinInt64 modulus one", "print(int.mod-floor(-9223372036854775808, 1))\n", "0\n"},
		{"MinInt64 modulus minus one", "print(int.mod-floor(-9223372036854775808, -1))\n", "0\n"},
		{"MinInt64 wraps in range", "print(int.mod-floor(-9223372036854775808, 3))\n", "1\n"},
		{"flow step", "flow -7\n    int.mod-floor(3)\n    print\n", "2\n"},
		{"partial captures the modulus", "let wrap = int.mod-floor(_, 3)\nprint wrap(-7)\n", "2\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			if err := Run("mod.zhex", tc.source, strings.NewReader(""), &out); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("got %q, want %q", out.String(), tc.want)
			}
		})
	}
}

// TestIntModFloorModulusZero pins the hard error: modulus zero is a language
// error like %, under the same division-zero code, with no hint because
// there is nothing to suggest but a nonzero modulus.
func TestIntModFloorModulusZero(t *testing.T) {
	err := Run("bad.zhex", "print(int.mod-floor(7, 0))\n", strings.NewReader(""), &strings.Builder{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.HasPrefix(err.Error(), "bad.zhex:1:7: int.mod-floor modulus is zero") {
		t.Fatalf("first line changed: %q", err.Error())
	}
	if strings.Contains(err.Error(), "hint:") {
		t.Fatalf("division-zero must carry no hint: %q", err.Error())
	}
	report, ok := Report(err)
	if !ok || !strings.Contains(report, `"code":"division-zero"`) {
		t.Fatalf("wanted code division-zero, got %s", report)
	}
}

// TestIntModFloorTyping pins the argument diagnostics: both arguments are
// Int and there are exactly two.
func TestIntModFloorTyping(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"value type", "print(int.mod-floor(\"7\", 3))\n", "argument 1 of int.mod-floor requires type Int, found String"},
		{"modulus type", "print(int.mod-floor(7, \"3\"))\n", "argument 2 of int.mod-floor requires type Int, found String"},
		{"argument count", "print(int.mod-floor(7))\n", "int.mod-floor requires 2 arguments, found 1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			err := Run("typing.zhex", tc.source, strings.NewReader(""), &out)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted %q, got %v", tc.want, err)
			}
		})
	}
}
