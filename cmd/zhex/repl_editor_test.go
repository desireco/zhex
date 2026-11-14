package main

import (
	"testing"
)

func TestContinuationIndentTracksNestedBlocksAndDedent(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   int
	}{
		{"word f() -> Int\n", 4},
		{"word f() -> Int\n    if true\n", 8},
		{"word f() -> Int\n    if true\n        1\n", 8},
		{"word f() -> Int\n    if true\n        1\n    else\n", 8},
		{"word f() -> Int\n    if true\n        1\n    else\n        2\n    3\n", 4},
	} {
		if got := continuationIndent(tc.source); got != tc.want {
			t.Errorf("%q: got %d, want %d", tc.source, got, tc.want)
		}
	}
}
