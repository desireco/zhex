package lang

import (
	"bytes"
	"strings"
	"testing"
)

// The #159 list-end words after #236's move: list.first and list.last
// read the ends without changing the list, and every empty-list failure
// is the family's list-range diagnostic with try/catch selecting the
// checked Result. list.pop itself moved to Stack<T> (stack.pop) — lists
// are grow-only again, and the old name fails the unknown-word check.

func TestListFirstAndLastReadTheEndsWithoutChanging(t *testing.T) {
	source := `let xs = [4, 5, 6, 7]
print(list.first(xs))
print(list.last(xs))
print(list.length(xs))
let one = [9]
print(list.first(one))
print(list.last(one))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "4\n7\n4\n9\n9\n" {
		t.Fatal(out.String())
	}
}

func TestListPopIsRemovedForStacks(t *testing.T) {
	source := `let xs = [1, 2]
let top = list.pop(xs)
print(top)
`
	err := Run("bad.zhex", source, strings.NewReader(""), &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), `unknown word "list.pop"`) {
		t.Fatalf("wanted unknown-word diagnostic, got %v", err)
	}
	if d, ok := err.(*diagnostic); !ok || d.code != CodeUnknownWord {
		t.Fatalf("wanted code unknown-word, got %v", err)
	}
	// The flow receiver form and partial application are gone with it.
	for _, tail := range []string{
		"let xs: List<Int> = list.new()\nlet top = flow xs\n    list.pop\n",
		"let take: (List<Int>) -> Int = list.pop _\n",
	} {
		if err := Run("bad.zhex", tail, strings.NewReader(""), &strings.Builder{}); err == nil || !strings.Contains(err.Error(), `unknown word "list.pop"`) {
			t.Fatalf("%s: wanted unknown-word diagnostic, got %v", tail, err)
		}
	}
}

func TestListFirstAndLastOnAnEmptyListFailListRange(t *testing.T) {
	for _, tc := range []struct {
		word    string
		indices string
	}{
		{"list.first", `0`},
		{"list.last", `-1`},
	} {
		source := "let xs: List<Int> = []\nlet end = " + tc.word + "(xs)\n"
		err := Run("bad.zhex", source, strings.NewReader(""), &strings.Builder{})
		if err == nil || !strings.Contains(err.Error(), "list index is out of range") {
			t.Fatalf("%s: wanted range diagnostic, got %v", tc.word, err)
		}
		d, ok := err.(*diagnostic)
		if !ok || d.code != CodeListRange {
			t.Fatalf("%s: wanted code list-range, got %v", tc.word, err)
		}
		if d.operation != tc.word {
			t.Fatalf("%s: wanted operation %s, got %q", tc.word, tc.word, d.operation)
		}
		if d.details["index"] != tc.indices || d.details["length"] != "0" {
			t.Fatalf("%s: wanted details index %s length 0, got %v", tc.word, tc.indices, d.details)
		}
	}
}

func TestListEndsCheckedResultSelectsFailure(t *testing.T) {
	source := `let empty: List<Int> = []
let b = list.first(empty) catch problem
    print(error.code(problem))
    0
let c = list.last(empty) catch problem
    print(error.code(problem))
    0
word top-of(xs: List<Int>) -> Int
    list.last(xs) try
print(top-of([5, 8]) try)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "list-range\nlist-range\n8\n" {
		t.Fatal(out.String())
	}
}

func TestListFirstAndLastReadDuringIteration(t *testing.T) {
	source := `let xs = [1, 2]
for v in xs
    print(list.last(xs))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n2\n" {
		t.Fatal(out.String())
	}
}

func TestListEndsRejectWrongTypes(t *testing.T) {
	cases := []struct {
		source  string
		message string
	}{
		{"let x = list.first(5)\n", "list.first requires a List"},
		{"let x = list.first([1], 2)\n", "list.first requires 1 argument, found 2"},
		{"let x = list.last(5)\n", "list.last requires a List"},
		{"let x = list.last([1], 2)\n", "list.last requires 1 argument, found 2"},
	}
	for _, tc := range cases {
		if got := runError(t, tc.source); !strings.Contains(got, tc.message) {
			t.Errorf("%s: wrong diagnostic %q, want %q", tc.source, got, tc.message)
		}
	}
}

func TestListEndsInferWordParameters(t *testing.T) {
	source := `word top-of(xs)
    list.last(xs)
print(top-of([2, 9]))
word bottom-of(xs)
    list.first(xs)
print(bottom-of(["x", "y"]))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "9\nx\n" {
		t.Fatal(out.String())
	}
}
