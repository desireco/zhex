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
		{"json.decode", "type C = record\n    n: Int\nlet c = json.decode(\"{\\\"n\\\": 1}\", C(0)) try\n", "json.decode"},
		{"xml.decode", "type C = record\n    n: Int\nlet c = xml.decode(\"<c n=\\\"1\\\"/>\", C(0)) try\n", "xml.decode"},
		{"temporal words", "let t = date.new(2026, 1, 1)\n", "date.new"},
		{"free space", "let free = file.free-space(\".\") try\n", "file.free-space"},
		{"generic choices", "type Maybe<T> = choice\n    None\n    Some(value: T)\nlet m = Maybe.Some(1)\n", "generic choice"},
		{"checked writes", "let xs: List<Int> = []\n(xs[0] = 1) try\n", "checked indexed writes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := compileNative(t, tc.source)
			if err == nil {
				t.Fatalf("%s compiled; expected a not-available-in-build fence", tc.name)
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

func TestNativeRecursiveDefaultsFence(t *testing.T) {
	for _, source := range []string{
		"word recur(n: Int = recur()) -> Int\n    n\nprint(recur())\n",
		"word recur(n: Int, other: Int = recur(1)) -> Int\n    other\nlet f = recur(_)\nprint(f(1))\n",
	} {
		_, err := compileNative(t, source)
		if err == nil || !strings.Contains(err.Error(), "recursive word defaults is not available in this build") {
			t.Fatalf("expected positioned recursive-default fence, got %v", err)
		}
	}
}
