package lang

import (
	"bytes"
	"strings"
	"testing"
)

// Widened Int: every surface carries the signed 128-bit width (#167) —
// boundary values, the day-24 product class that overflowed 64 bits,
// parsing and printing round trips, comparisons, and clean failures
// beyond the new bounds.

const int128MaxText = "170141183460469231731687303715884105727"
const int128MinText = "-170141183460469231731687303715884105728"
const twoTo100Text = "1267650600228229401496703205376"

func runInt128(t *testing.T, source string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := Run("int128.zhex", source, strings.NewReader(""), &out)
	return out.String(), err
}

func TestInt128BoundaryValues(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"max literal prints full width", "print(" + int128MaxText + ")\n", int128MaxText + "\n"},
		{"min literal prints full width", "print(" + int128MinText + ")\n", int128MinText + "\n"},
		{"max minus one", "print(" + int128MaxText + " - 1)\n", "170141183460469231731687303715884105726\n"},
		{"max plus min is minus one", "print(" + int128MaxText + " + " + int128MinText + ")\n", "-1\n"},
		{"min remainder minus one", "print(" + int128MinText + " % -1)\n", "0\n"},
		{"64-bit top now adds", "print(9223372036854775807 + 1)\n", "9223372036854775808\n"},
		{"64-bit min squares wide", "print(3037000500 * 3037000500)\n", "9223372037000250000\n"},
		{"negation across 64-bit min", "print(-(-9223372036854775808 - 1))\n", "9223372036854775809\n"},
		{"max halved", "print(" + int128MaxText + " / 2)\n", "85070591730234615865843651857942052863\n"},
	}
	for _, c := range cases {
		got, err := runInt128(t, c.src)
		if err != nil || got != c.want {
			t.Errorf("%s: output %q, error %v; want %q", c.name, got, err, c.want)
		}
	}
}

func TestInt128OverflowStaysError(t *testing.T) {
	for _, src := range []string{
		"print(" + int128MaxText + " + 1)\n",
		"print(" + int128MinText + " - 1)\n",
		"print(-(" + int128MinText + "))\n",
		"print(" + int128MinText + " / -1)\n",
		"print(" + int128MaxText + " * 2)\n",
		"print(" + int128MaxText + " - " + int128MinText + ")\n",
	} {
		got, err := runInt128(t, src)
		if err == nil || !strings.Contains(err.Error(), "integer overflow") || got != "" {
			t.Errorf("%q: output %q, error %v; want an integer overflow diagnostic", src, got, err)
		}
	}
}

// The day-24 gap: hailstone coordinates near 4e14 crossed with velocities
// near 1e15 overflowed every 64-bit product; the same products now compute
// exactly.
func TestInt128Day24Products(t *testing.T) {
	source := "let p = 400000000000000\n" +
		"print(p * p)\n" +
		"print(999999999999999 * 999999999999999)\n" +
		"print(160000000000000000000000000000 / 999999999999999)\n" +
		"print(160000000000000000000000000000 % 999999999999999)\n"
	want := "160000000000000000000000000000\n" +
		"999999999999998000000000000001\n" +
		"160000000000000\n" +
		"160000000000000\n"
	got, err := runInt128(t, source)
	if err != nil || got != want {
		t.Fatalf("output %q, error %v; want %q", got, err, want)
	}
}

func TestInt128TruncatedDivisionAtWidth(t *testing.T) {
	source := "print(" + twoTo100Text + " / 3)\n" +
		"print(" + twoTo100Text + " % 3)\n" +
		"print(-" + twoTo100Text + " / 3)\n" +
		"print(-" + twoTo100Text + " % 3)\n"
	want := "422550200076076467165567735125\n1\n-422550200076076467165567735125\n-1\n"
	got, err := runInt128(t, source)
	if err != nil || got != want {
		t.Fatalf("output %q, error %v; want %q", got, err, want)
	}
}

func TestInt128ParsePrintRoundTrip(t *testing.T) {
	source := "print(int.to-string(" + int128MaxText + "))\n" +
		"print(int.to-string(" + int128MinText + "))\n" +
		"let n = text.to-int(\"" + int128MaxText + "\") try\n" +
		"print(n)\n" +
		"let m = text.to-int(\" " + int128MinText + " \") try\n" +
		"print(int.to-string(m))\n"
	want := int128MaxText + "\n" + int128MinText + "\n" + int128MaxText + "\n" + int128MinText + "\n"
	got, err := runInt128(t, source)
	if err != nil || got != want {
		t.Fatalf("output %q, error %v; want %q", got, err, want)
	}
}

func TestInt128ParseBeyondWidthFails(t *testing.T) {
	source := "let n = text.to-int(\"170141183460469231731687303715884105728\") catch problem\n" +
		"    print(error.code(problem))\n" +
		"    0\n"
	got, err := runInt128(t, source)
	if err != nil || got != "text-to-int\n" {
		t.Fatalf("output %q, error %v; want text-to-int failure code", got, err)
	}
}

func TestInt128LiteralBeyondWidthIsParseError(t *testing.T) {
	_, err := runInt128(t, "print(170141183460469231731687303715884105728)\n")
	if err == nil || !strings.Contains(err.Error(), "integer literal is outside the signed 128-bit range") {
		t.Fatalf("error %v; want the literal-range diagnostic naming the signed 128-bit range", err)
	}
}

func TestInt128Comparisons(t *testing.T) {
	source := "print(9223372036854775807 < 9223372036854775808)\n" +
		"print(" + int128MaxText + " > 85070591730234615865843651857942052864)\n" +
		"print(18446744073709551616 == 18446744073709551616)\n" +
		"print(" + int128MinText + " != " + int128MaxText + ")\n" +
		"print(-" + twoTo100Text + " <= -" + twoTo100Text + ")\n"
	want := "true\ntrue\ntrue\ntrue\ntrue\n"
	got, err := runInt128(t, source)
	if err != nil || got != want {
		t.Fatalf("output %q, error %v; want %q", got, err, want)
	}
}

func TestInt128ModFloorAtWidth(t *testing.T) {
	source := "print(int.mod-floor(-" + twoTo100Text + ", 7))\n" +
		"print(int.mod-floor(" + int128MinText + " + 9, 5))\n"
	want := "5\n1\n"
	got, err := runInt128(t, source)
	if err != nil || got != want {
		t.Fatalf("output %q, error %v; want %q", got, err, want)
	}
}

func TestInt128StringContextException(t *testing.T) {
	source := "print(\"n=\" + " + twoTo100Text + ")\n" +
		"print(" + int128MinText + " + \"ns\")\n"
	want := "n=" + twoTo100Text + "\n" + int128MinText + "ns\n"
	got, err := runInt128(t, source)
	if err != nil || got != want {
		t.Fatalf("output %q, error %v; want %q", got, err, want)
	}
}

func TestInt128JSONRoundTrip(t *testing.T) {
	source := "type Config = record\n" +
		"    total: Int\n\n" +
		"let c = json.decode(\"{\\\"total\\\": " + int128MaxText + "}\", Config(0)) try\n" +
		"print(json.encode(c) try)\n"
	want := "{\"total\":" + int128MaxText + "}\n"
	got, err := runInt128(t, source)
	if err != nil || got != want {
		t.Fatalf("output %q, error %v; want %q", got, err, want)
	}
}

func TestInt128JSONBeyondRangeFails(t *testing.T) {
	source := "type Config = record\n" +
		"    total: Int\n\n" +
		"let c = json.decode(\"{\\\"total\\\": 170141183460469231731687303715884105728}\", Config(0)) catch problem\n" +
		"    print(error.code(problem))\n" +
		"    Config(0)\n"
	got, err := runInt128(t, source)
	if err != nil || got != "json-range\n" {
		t.Fatalf("output %q, error %v; want json-range failure code", got, err)
	}
}

func TestInt128XMLRoundTripAndRange(t *testing.T) {
	source := "type Doc = record\n" +
		"    total: Int\n\n" +
		"let d = xml.decode(\"<doc><total>" + twoTo100Text + "</total></doc>\", Doc(0)) try\n" +
		"print(d.total)\n" +
		"let bad = xml.decode(\"<doc><total>170141183460469231731687303715884105728</total></doc>\", Doc(0)) catch problem\n" +
		"    print(error.code(problem))\n" +
		"    Doc(0)\n"
	want := twoTo100Text + "\nxml-range\n"
	got, err := runInt128(t, source)
	if err != nil || got != want {
		t.Fatalf("output %q, error %v; want %q", got, err, want)
	}
}

func TestInt128SortAndMapKeys(t *testing.T) {
	source := "let xs = [" + int128MaxText + ", 3, " + int128MinText + ", 5]\n" +
		"print(list.sort(xs)[0])\n" +
		"print(list.sort(xs)[3])\n" +
		"let m = {" + int128MaxText + ": \"max\"}\n" +
		"match m[" + int128MaxText + "]\n" +
		"    case Option.Some(tag)\n" +
		"        print(tag)\n" +
		"    case Option.None\n" +
		"        print(\"none\")\n" +
		"match m[" + twoTo100Text + "]\n" +
		"    case Option.Some(tag)\n" +
		"        print(tag)\n" +
		"    case Option.None\n" +
		"        print(\"none\")\n"
	want := int128MinText + "\n" + int128MaxText + "\nmax\nnone\n"
	got, err := runInt128(t, source)
	if err != nil || got != want {
		t.Fatalf("output %q, error %v; want %q", got, err, want)
	}
}

func TestInt128RangeLoopCrosses64BitBound(t *testing.T) {
	source := "for n in range(9223372036854775807, 9223372036854775810)\n    print(n)\n"
	want := "9223372036854775807\n9223372036854775808\n9223372036854775809\n"
	got, err := runInt128(t, source)
	if err != nil || got != want {
		t.Fatalf("output %q, error %v; want %q", got, err, want)
	}
}
