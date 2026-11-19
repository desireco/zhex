package lang

import (
	"strings"
	"testing"
)

// compileNative is the emission seam's test helper: check a program and
// return its Go source (or the checker/fence error).
func compileNative(t *testing.T, source string) (string, error) {
	t.Helper()
	return CompileNativeSource("native.zhex", source, nil)
}

// TestNativeEmitsReadableGo pins the artifact's shape: a word becomes a
// Go function with a concrete signature, arithmetic names the zhexrt
// operations, and the entry runs through zhexrt.Run — the readable-output
// acceptance criterion of #185.
func TestNativeEmitsReadableGo(t *testing.T) {
	source := "word double(n: Int) -> Int\n    n * 2\n\nprint(int.to-string(double(21)))\n"
	goSource, err := compileNative(t, source)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"func double(n zhexrt.Int) zhexrt.Int {",
		"zhexrt.Mul(_",
		"zhexrt.Print(_",
		"func main() {\n\tzhexrt.Run(zhexMain)\n}",
		"zhexrt.EnterWord(",
	} {
		if !strings.Contains(goSource, want) {
			t.Fatalf("emitted Go misses %q:\n%s", want, goSource)
		}
	}
}

// TestNativeFenceNamesTheConstruct: out-of-subset constructs fail with
// the pinned not-available-in-build code and a diagnostic naming them.
func TestNativeFenceNamesTheConstruct(t *testing.T) {
	for _, tc := range []struct{ name, source, construct string }{
		{"lists", "let xs: List<Int> = []\n", "list expressions"},
		{"records", "type P = record\n    x: Int\nlet p = P(1)\n", "record constructors"},
		{"choices", "type Light = choice\n    Red\n    Green\nlet l = Light.Red\n", "choices"},
		{"while", "var n = 0\nwhile n < 3\n    n = n + 1\n", "while statements"},
		{"builtin", "let s = text.upper(\"a\")\n", "text.upper"},
		{"partials", "word add(a: Int, b: Int) -> Int\n    a + b\nlet inc = add(_, 1)\n", "partials"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := compileNative(t, tc.source)
			if err == nil {
				t.Fatalf("%s compiled; expected a not-available-in-build fence", tc.name)
			}
			if tc.construct == "" {
				return // merely must compile-check cleanly elsewhere
			}
			if !strings.Contains(err.Error(), "not available in this build") || !strings.Contains(err.Error(), tc.construct) {
				t.Fatalf("fence mismatch: %v", err)
			}
		})
	}
}

// TestNativeCheckerErrorsPassThrough: a type error renders exactly as
// run's, before any Go is produced.
func TestNativeCheckerErrorsPassThrough(t *testing.T) {
	_, err := compileNative(t, "print(a-b)\n")
	if err == nil || !strings.Contains(err.Error(), `native.zhex:1:7: unknown name "a-b"`) {
		t.Fatalf("err = %v", err)
	}
}
