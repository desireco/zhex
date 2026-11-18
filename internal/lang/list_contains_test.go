package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestListContainsAnswersMembership(t *testing.T) {
	source := `let numbers = [3, -1, 10, -7, 0]
print(list.contains(numbers, -7))
print(list.contains(numbers, 4))
print(list.contains(numbers, numbers[0]))
let words = ["pear", "Apple", "fig"]
print(list.contains(words, "fig"))
print(list.contains(words, "pear "))
let empty: List<Int> = list.new()
print(list.contains(empty, 0))
let no-words: List<String> = list.new()
print(list.contains(no-words, ""))
print(numbers[0])
print(words[0])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "true\nfalse\ntrue\ntrue\nfalse\nfalse\nfalse\n3\npear\n" {
		t.Fatal(out.String())
	}
}

func TestListContainsInfersWordParameters(t *testing.T) {
	source := `word member(values, item: Int) -> Bool
    list.contains(values, item)
print(member([1, 2], 2))
print(member([1, 2], 5))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "true\nfalse\n" {
		t.Fatal(out.String())
	}
}

func TestListContainsPartiallyAppliesItem(t *testing.T) {
	source := `let has-fig = list.contains _, "fig"
print(has-fig(["a", "fig"]))
print(has-fig(["a"]))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "true\nfalse\n" {
		t.Fatal(out.String())
	}
}

func TestListContainsRejectsOtherElementTypes(t *testing.T) {
	cases := []struct {
		source  string
		message string
	}{
		{"let x = list.contains(5, 2)\n", "list.contains requires a List<Int> or List<String>, found Int"},
		{"let x = list.contains([true, false], true)\n", "list.contains requires a List<Int> or List<String>, found List<Bool>"},
		{"type P = record\n    n: Int\nlet x = list.contains([P(1)], P(1))\n", "list.contains requires a List<Int> or List<String>, found List<P>"},
		{"let x = list.contains([1], \"1\")\n", "argument 2 of list.contains requires type Int, found String"},
		{"let x = list.contains([1])\n", "list.contains requires 2 arguments, found 1"},
		{"let x = list.contains([1], 2, 3)\n", "list.contains requires 2 arguments, found 3"},
	}
	for _, tc := range cases {
		if got := runError(t, tc.source); !strings.Contains(got, tc.message) {
			t.Errorf("%s: wrong diagnostic %q, want %q", tc.source, got, tc.message)
		}
	}
}

func TestListContainsHintNamesAnyWorkaround(t *testing.T) {
	got := runError(t, "let x = list.contains([true], true)\n")
	if !strings.Contains(got, "hint: test other element types with list.any and a predicate") {
		t.Fatalf("missing list.any hint: %q", got)
	}
}
