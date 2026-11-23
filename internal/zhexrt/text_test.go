package zhexrt

import "testing"

// TestRepeatAndPadFailBeforeAllocation pins the guard that keeps the Go
// runtime's makeslice law inside the words: a count or width whose result
// the runtime cannot allocate fails with the word's range code as a
// panic-and-recover diagnostic, never as a raw makeslice panic.
func TestRepeatAndPadFailBeforeAllocation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		call   func()
		code   string
		result string
	}{
		{"repeat beyond the heap", func() { TextRepeat(At("f.zhex", 1, 7), "x", I64(1<<62)) },
			"text-repeat-count", "text.repeat count 4611686018427387904 is out of range"},
		{"repeat wider than int64", func() { TextRepeat(At("f.zhex", 1, 7), "ab", Max) },
			"text-repeat-count", "text.repeat count 170141183460469231731687303715884105727 is out of range"},
		{"pad-left beyond the heap", func() { TextPadLeft(At("f.zhex", 1, 7), "x", I64(1<<62), "0") },
			"text-pad-width", "text.pad-left width 4611686018427387904 is out of range"},
		{"pad-right beyond the heap", func() { TextPadRight(At("f.zhex", 1, 7), "x", I64(1<<62), "0") },
			"text-pad-width", "text.pad-right width 4611686018427387904 is out of range"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got *Error
			func() {
				defer func() { r := recover(); got, _ = r.(*Error) }()
				tc.call()
			}()
			if got == nil {
				t.Fatalf("wanted a %s failure, got none (a raw panic was swallowed)", tc.code)
			}
			if got.Code != tc.code || got.Message != tc.result {
				t.Fatalf("got %s: %q, want %s: %q", got.Code, got.Message, tc.code, tc.result)
			}
		})
	}
}

// TestRepeatAndPadHappyPaths pins that the guard leaves ordinary use alone:
// zero counts, empty texts, and exact-length pads keep their #196 results.
func TestRepeatAndPadHappyPaths(t *testing.T) {
	if got := TextRepeat(At("f.zhex", 1, 1), "ab", I64(3)); got != "ababab" {
		t.Fatalf(`TextRepeat("ab", 3) = %q`, got)
	}
	if got := TextRepeat(At("f.zhex", 1, 1), "ab", I64(0)); got != "" {
		t.Fatalf(`TextRepeat("ab", 0) = %q`, got)
	}
	if got := TextRepeat(At("f.zhex", 1, 1), "", I64(1<<62)); got != "" {
		t.Fatalf(`TextRepeat("", 1<<62) = %q`, got)
	}
	if got := TextPadLeft(At("f.zhex", 1, 1), "7", I64(3), "."); got != "..7" {
		t.Fatalf(`TextPadLeft("7", 3, ".") = %q`, got)
	}
	if got := TextPadRight(At("f.zhex", 1, 1), "abc", I64(3), "0"); got != "abc" {
		t.Fatalf(`TextPadRight("abc", 3, "0") = %q`, got)
	}
}
