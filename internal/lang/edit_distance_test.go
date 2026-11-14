package lang

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestEditDistanceAgainstRuneReference(t *testing.T) {
	source, err := os.ReadFile("../../examples/edit-distance.zhex")
	if err != nil {
		t.Fatal(err)
	}
	pairs := []struct{ left, right string }{
		{"", ""},
		{"", "abc"},
		{"abc", ""},
		{"a", "a"},
		{"abc", "abd"},
		{"kitten", "sitting"},
		{"café", "cafe"},
		{"🙂a", "a🙂"},
		{"same", "same"},
	}
	var expected = "kitten → sitting: 3\nempty → abc: 3\ncafé → cafe: 1\n"
	var cases string
	for _, pair := range pairs {
		cases += fmt.Sprintf("print(int.to-string(distance(%q, %q) try))\n", pair.left, pair.right)
		expected += fmt.Sprintf("%d\n", referenceDistance(pair.left, pair.right))
	}
	var output bytes.Buffer
	if err := Run("edit-distance-oracle.zhex", string(source)+"\n"+cases, bytes.NewBuffer(nil), &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != expected {
		t.Fatalf("distance outputs differ from rune reference:\n got %q\nwant %q", output.String(), expected)
	}
}

func referenceDistance(left, right string) int {
	a, b := []rune(left), []rune(right)
	previous := make([]int, len(b)+1)
	for column := range previous {
		previous[column] = column
	}
	for row, leftRune := range a {
		current := make([]int, len(b)+1)
		current[0] = row + 1
		for column, rightRune := range b {
			cost := 0
			if leftRune != rightRune {
				cost = 1
			}
			deletion, insertion, substitution := previous[column+1]+1, current[column]+1, previous[column]+cost
			current[column+1] = min(deletion, insertion, substitution)
		}
		previous = current
	}
	return previous[len(b)]
}
