package lang

import (
	"bytes"
	"strings"
	"testing"
)

// The #159 list-end words: list.pop removes and returns the last element
// (the one sanctioned exception to mutation words returning their
// collection), list.first and list.last read the ends without changing the
// list, and every empty-list failure is the family's list-range diagnostic
// with try/catch selecting the checked Result.

func TestListPopRemovesTheLastElementAndReturnsIt(t *testing.T) {
	source := `let xs = [1, 2, 3]
let top = list.pop(xs)
print(top)
print(list.length(xs))
print(xs[1])
let words = ["a", "b"]
let last-word = list.pop(words)
print(last-word)
print(list.length(words))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n2\n2\nb\n1\n" {
		t.Fatal(out.String())
	}
}

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

func TestListPopOnAnEmptyListFailsListRange(t *testing.T) {
	source := `let xs: List<Int> = []
let top = list.pop(xs)
`
	err := Run("bad.zhex", source, strings.NewReader(""), &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "list index is out of range") {
		t.Fatalf("wanted range diagnostic, got %v", err)
	}
	d, ok := err.(*diagnostic)
	if !ok || d.code != CodeListRange {
		t.Fatalf("wanted code list-range, got %v", err)
	}
	if d.operation != "list.pop" {
		t.Fatalf("wanted operation list.pop, got %q", d.operation)
	}
	if d.details["index"] != "-1" || d.details["length"] != "0" {
		t.Fatalf("wanted the today's-idiom details index -1 length 0, got %v", d.details)
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
let a = list.pop(empty) catch problem
    print(error.code(problem))
    0
let b = list.first(empty) catch problem
    print(error.code(problem))
    0
let c = list.last(empty) catch problem
    print(error.code(problem))
    0
word top-of(xs: List<Int>) -> Int
    list.last(xs) try
print(top-of([5, 8]) try)
word drain-one(xs: List<Int>) -> Int
    list.pop(xs) try
let n = drain-one([]) catch problem
    print(error.code(problem))
    0
print(n)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "list-range\nlist-range\nlist-range\n8\nlist-range\n0\n" {
		t.Fatal(out.String())
	}
}

func TestListPopThreadsAsTheFlowTail(t *testing.T) {
	source := `let stack: List<Int> = list.new()
let top = flow stack
    list.append 4
    list.append 5
    list.pop
print(top)
print(list.length(stack))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "5\n1\n" {
		t.Fatal(out.String())
	}
}

func TestListPopInALoopDrainsAStack(t *testing.T) {
	source := `let stack: List<Int> = []
for n in [1, 2, 3, 4]
    list.append(stack, n)
let total = 0
var sum = 0
while list.length(stack) > 0
    let top = list.pop(stack)
    sum = sum + top
print(sum)
var order = ""
let letters: List<String> = []
for n in ["a", "b", "c"]
    list.append(letters, n)
while list.length(letters) > 0
    order = order + list.pop(letters)
print(order)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "10\ncba\n" {
		t.Fatal(out.String())
	}
}

func TestListPopDuringIterationFailsBeforeMutation(t *testing.T) {
	source := `let xs = [1, 2]
for v in xs
    list.pop(xs)
`
	var out bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &out)
	if err == nil {
		t.Fatal("missing iteration diagnostic")
	}
	if d, ok := err.(*diagnostic); !ok || d.code != CodeCollectionIterating {
		t.Fatalf("expected collection-iterating, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("ran effects before failing: %q", out.String())
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

func TestListPopDropsItsElementOnAnOrdinaryLine(t *testing.T) {
	source := `let xs = [1, 2]
list.pop(xs)
print(list.length(xs))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n" {
		t.Fatal(out.String())
	}
}

func TestListEndsRejectWrongTypes(t *testing.T) {
	cases := []struct {
		source  string
		message string
	}{
		{"let x = list.pop(5)\n", "list.pop requires a List"},
		{"let x = list.pop(\"a\")\n", "list.pop requires a List"},
		{"let x = list.pop([1], 2)\n", "list.pop requires 1 argument, found 2"},
		{"let x = list.pop()\n", "list.pop requires 1 argument, found 0"},
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

func TestListPopPartiallyApplies(t *testing.T) {
	source := `let take: (List<Int>) -> Int = list.pop _
print(take([7, 9]))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "9\n" {
		t.Fatal(out.String())
	}
}

func TestListEndsInferWordParameters(t *testing.T) {
	source := `word top-of(xs)
    list.last(xs)
print(top-of([2, 9]))
word bottom-of(xs)
    list.first(xs)
print(bottom-of(["x", "y"]))
word drain(xs: List<Int>) -> Int
    var total = 0
    while list.length(xs) > 0
        total = total + list.pop(xs)
    total
print(drain([1, 2, 3]))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "9\nx\n6\n" {
		t.Fatal(out.String())
	}
}
