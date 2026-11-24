package lang

import (
	"bytes"
	"strings"
	"testing"
)

// The #231 bitwise words: int.and, int.or, and int.xor are two's-complement
// bit operations over the interpreter's full 128-bit width; int.shift-left
// is the checked mathematical a·2^n (integer overflow when it leaves the
// range, zero shifted by any count stays zero); int.shift-right is the
// arithmetic, sign-keeping shift with counts past the width clamped to the
// mathematical result (0, or −1 for negative values). A negative count is
// the runtime int-shift-count fault in every build; the js engine build
// adds the owner's 64-bit ruling — a count of 64 or more fails
// int-shift-width (pinned by the engine gate, not here).

func TestIntAndOrXorAreTwosComplementOverTheFullWidth(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"small masks", "print(int.and(12, 10))\nprint(int.or(12, 10))\nprint(int.xor(12, 10))\n", "8\n14\n6\n"},
		{"negative operands follow two's complement", "print(int.and(-1, 255))\nprint(int.or(-1, 0))\nprint(int.xor(-1, 5))\nprint(int.xor(-1, -1))\n", "255\n-1\n-6\n0\n"},
		{"values above 2^64", "let wide = 1267650600228229401496703205379\nprint(int.and(wide, 1267650600228229401496703205377))\nprint(int.or(wide, 1267650600228229401496703205377))\nprint(int.xor(wide, 1267650600228229401496703205377))\n", "1267650600228229401496703205377\n1267650600228229401496703205379\n2\n"},
		{"negative wide values", "print(int.and(-1267650600228229401496703205376, -1))\nprint(int.xor(-1267650600228229401496703205376, -1))\nprint(int.or(-1267650600228229401496703205376, 3))\n", "-1267650600228229401496703205376\n1267650600228229401496703205375\n-1267650600228229401496703205373\n"},
		{"the signed 128-bit edges", "print(int.and(-170141183460469231731687303715884105728, -1))\nprint(int.or(-170141183460469231731687303715884105728, 0))\nprint(int.xor(-170141183460469231731687303715884105728, -1))\n", "-170141183460469231731687303715884105728\n-170141183460469231731687303715884105728\n170141183460469231731687303715884105727\n"},
		{"flow receiver and partial", "let mask = int.and _, 7\nprint(mask(12))\nprint(mask(13))\nlet tagged = flow 40\n    int.or 128\n    print\n", "4\n5\n168\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Run("bits.zhex", tc.source, strings.NewReader(""), &out); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("got %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestIntShiftCountsZeroThroughSixtyThree(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"count zero keeps the value", "print(int.shift-left(21, 0))\nprint(int.shift-right(21, 0))\n", "21\n21\n"},
		{"count one", "print(int.shift-left(3, 1))\nprint(int.shift-right(256, 1))\n", "6\n128\n"},
		{"count sixty-three crosses limbs for wide values", "print(int.shift-left(1, 63))\nprint(int.shift-right(1180591620717411303424, 63))\nprint(int.shift-right(-1329227995784915872903807060280344576, 63))\n", "9223372036854775808\n128\n-144115188075855872\n"},
		{"negative values shift with their sign kept", "print(int.shift-right(-8, 1))\nprint(int.shift-right(-9, 1))\nprint(int.shift-left(-1, 5))\n", "-4\n-5\n-32\n"},
		{"masks compose with shifts and flows", "let code = int.or(int.shift-left(5, 3), int.shift-left(2, 6))\nprint(int.and(int.shift-right(code, 3), 7))\nprint(int.and(int.shift-right(code, 6), 7))\nlet masked = flow code\n    int.shift-right 6\n    int.and 7\nprint(masked)\n", "5\n2\n2\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Run("bits.zhex", tc.source, strings.NewReader(""), &out); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("got %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestIntShiftCountsSixtyFourThroughOneTwentySeven(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"shift-left crosses the 64-bit limb", "print(int.shift-left(1, 64))\nprint(int.shift-left(1, 100))\nprint(int.shift-left(1, 126))\n", "18446744073709551616\n1267650600228229401496703205376\n85070591730234615865843651857942052864\n"},
		{"shift-right crosses the 64-bit limb", "print(int.shift-right(1180591620717411303424, 64))\nprint(int.shift-right(1180591620717411303424, 70))\nprint(int.shift-right(1180591620717411303424, 71))\nprint(int.shift-right(-1329227995784915872903807060280344576, 64))\n", "64\n1\n0\n-72057594037927936\n"},
		{"count one twenty-seven is the last in-width count", "print(int.shift-right(170141183460469231731687303715884105727, 127))\nprint(int.shift-right(-1, 127))\nprint(int.shift-left(-1, 127))\n", "0\n-1\n-170141183460469231731687303715884105728\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Run("bits.zhex", tc.source, strings.NewReader(""), &out); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("got %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestIntShiftCountsPastTheWidthGiveTheMathematicalResult(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"count one twenty-eight", "print(int.shift-right(5, 128))\nprint(int.shift-right(-1, 128))\n", "0\n-1\n"},
		{"count two hundred", "print(int.shift-right(5, 200))\nprint(int.shift-right(-1, 200))\nprint(int.shift-right(-170141183460469231731687303715884105728, 200))\nprint(int.shift-left(0, 200))\n", "0\n-1\n-1\n0\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Run("bits.zhex", tc.source, strings.NewReader(""), &out); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("got %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestIntShiftLeftOverflowFailsIntegerOverflow(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"count one twenty-seven leaves the range", "print(int.shift-left(1, 127))\n"},
		{"magnitude leaves the range inside the width", "print(int.shift-left(3, 126))\n"},
		{"count past the width with a nonzero value", "print(int.shift-left(1, 128))\n"},
		{"count far past the width", "print(int.shift-left(-1, 200))\n"},
		{"negative value leaves the range", "print(int.shift-left(-1329227995784915872903807060280344576, 8))\n"},
		{"max value shifted once", "print(int.shift-left(170141183460469231731687303715884105727, 1))\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Run("bad.zhex", tc.source, strings.NewReader(""), &strings.Builder{})
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.HasPrefix(err.Error(), "bad.zhex:1:7: integer overflow") {
				t.Fatalf("first line changed: %q", err.Error())
			}
			if strings.Contains(err.Error(), "hint:") {
				t.Fatalf("integer-overflow must carry no hint: %q", err.Error())
			}
			report, ok := Report(err)
			if !ok || !strings.Contains(report, `"code":"integer-overflow"`) {
				t.Fatalf("wanted code integer-overflow, got %s", report)
			}
		})
	}
}

func TestIntShiftNegativeCountFailsIntShiftCount(t *testing.T) {
	cases := []struct {
		name   string
		source string
		at     string
	}{
		{"shift-left", "print(int.shift-left(1, -1))\n", "bad.zhex:1:7"},
		{"shift-right", "print(int.shift-right(8, -3))\n", "bad.zhex:1:7"},
		{"computed count", "var n = 0 - 2\nprint(int.shift-right(8, n))\n", "bad.zhex:2:7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Run("bad.zhex", tc.source, strings.NewReader(""), &strings.Builder{})
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.HasPrefix(err.Error(), tc.at+": shift count is negative") {
				t.Fatalf("first line changed: %q", err.Error())
			}
			if strings.Contains(err.Error(), "hint:") {
				t.Fatalf("int-shift-count must carry no hint: %q", err.Error())
			}
			report, ok := Report(err)
			if !ok || !strings.Contains(report, `"code":"int-shift-count"`) {
				t.Fatalf("wanted code int-shift-count, got %s", report)
			}
		})
	}
}

// TestIntBitWordsTyping pins the argument diagnostics: both arguments are
// Int, and try/catch never select a Result — the words are plain Int calls
// whose failures are runtime faults like division's, not Result effects.
func TestIntBitWordsTyping(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"value type", "print(int.and(\"7\", 3))\n", "argument 1 of int.and requires type Int, found String"},
		{"mask type", "print(int.or(7, \"3\"))\n", "argument 2 of int.or requires type Int, found String"},
		{"count type", "print(int.shift-left(7, \"3\"))\n", "argument 2 of int.shift-left requires type Int, found String"},
		{"argument count", "print(int.xor(7))\n", "int.xor requires 2 arguments, found 1"},
		{"argument count shift", "print(int.shift-right(7))\n", "int.shift-right requires 2 arguments, found 1"},
		{"try requires a Result", "let x = int.and(1, 2) try\n", "try requires a Result, found Int"},
		{"catch requires a Result", "let x = int.shift-left(1, 2) catch 0\n", "catch requires a Result, found Int"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			err := Run("bad.zhex", tc.source, strings.NewReader(""), &out)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted %q, got %q (%v)", tc.want, out.String(), err)
			}
		})
	}
}

// TestIntShiftWordsInferPlainIntSignatures pins the checker's word
// signatures: a word ending in a shift call infers (Int, Int) -> Int with
// no Result effect, exactly like one ending in +.
func TestIntShiftWordsInferPlainIntSignatures(t *testing.T) {
	source := `word decode(code: Int, s: Int) -> Int
    int.and(int.shift-right(code, 3 * s), 7)
print(decode(168, 1))
print(decode(168, 2))
`
	var out bytes.Buffer
	if err := Run("bits.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "5\n2\n" {
		t.Fatal(out.String())
	}
}
