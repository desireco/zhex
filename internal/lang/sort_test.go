package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestListSortOrdersIntsAndStrings(t *testing.T) {
	source := `let numbers = [3, -1, 10, -7, 0]
let sorted-numbers = list.sort(numbers)
print(sorted-numbers[0])
print(sorted-numbers[1])
print(sorted-numbers[2])
print(sorted-numbers[3])
print(sorted-numbers[4])
print(numbers[0])
let words = ["pear", "Apple", "fig"]
let sorted-words = list.sort(words)
print(sorted-words[0])
print(sorted-words[1])
print(sorted-words[2])
print(words[0])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "-7\n-1\n0\n3\n10\n3\nApple\nfig\npear\npear\n" {
		t.Fatal(out.String())
	}
}

func TestListSortUsesCodePointOrder(t *testing.T) {
	source := `let values = ["żebra", "apple", "Éclair", "zebra", "😀", "éclair"]
let sorted = list.sort(values)
print(sorted[0])
print(sorted[1])
print(sorted[2])
print(sorted[3])
print(sorted[4])
print(sorted[5])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "apple\nzebra\nÉclair\néclair\nżebra\n😀\n" {
		t.Fatal(out.String())
	}
}

func TestListSortEmptyAndSingleElement(t *testing.T) {
	source := `let empty: List<Int> = list.new()
print(list.length(list.sort(empty)))
let single = list.sort(["only"])
print(single[0])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "0\nonly\n" {
		t.Fatal(out.String())
	}
}

func TestListSortByRanksRecordsWithStableTies(t *testing.T) {
	source := `type Entry = record
    name: String
    score: Int
word entry-score(e: Entry) -> Int
    e.score
word entry-name(e: Entry) -> String
    e.name
let entries = [Entry("a", 2), Entry("b", 1), Entry("c", 2), Entry("d", 1), Entry("e", 3)]
let ranked = list.sort-by(entries, entry-score(_))
for e in ranked
    print(entry-name(e))
let original = entries[0]
print(entry-name(original))
let by-name = list.sort-by(entries, entry-name(_))
let first-named = by-name[0]
print(entry-name(first-named))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "b\nd\na\nc\ne\na\na\n" {
		t.Fatal(out.String())
	}
}

func TestListSortByKeysRunOnceInOriginalOrder(t *testing.T) {
	source := `word noisy(n: Int) -> Int
    print(n)
    n
let ranked = list.sort-by([3, 1, 2], noisy(_))
print(ranked[0])
print(ranked[1])
print(ranked[2])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n1\n2\n1\n2\n3\n" {
		t.Fatal(out.String())
	}
}

func TestListSortReturnsFreshListWithSharedElements(t *testing.T) {
	source := `word head(values: List<Int>) -> Int
    values[0]
let inner = [2, 1]
let outer = [inner]
let ranked = list.sort-by(outer, head(_))
print(list.length(ranked))
list.append(inner, 3)
print(list.length(ranked[0]))
list.append(ranked, [9])
print(list.length(outer))
let values = [3, 1, 2]
let sorted = list.sort(values)
list.append(sorted, 4)
print(list.length(values))
print(values[0])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n3\n1\n3\n3\n" {
		t.Fatal(out.String())
	}
}

func TestListSortByAcceptsBuiltinPartialKeys(t *testing.T) {
	source := `let names = ["Cherry", "fig", "banana"]
let by-length = list.sort-by(names, text.length(_))
print(by-length[0])
print(by-length[1])
print(by-length[2])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "fig\nCherry\nbanana\n" {
		t.Fatal(out.String())
	}
}

func TestListSortByKeyHoldsTraversalGuard(t *testing.T) {
	source := `word push-and-report(values: List<Int>, n: Int) -> Int
    list.append(values, n)
    n
let values = [1, 2]
let key = push-and-report(values, _)
let ranked = list.sort-by(values, key)
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

func TestListSortByRecursiveKeyHitsDepthLimit(t *testing.T) {
	source := `word descend(n: Int) -> Int
    let deeper = list.sort-by([n], descend(_))
    deeper[0]
print(descend(1))
`
	err := Run("test.zhex", source, strings.NewReader(""), &bytes.Buffer{})
	if d, ok := err.(*diagnostic); !ok || d.code != CodeDepthLimit {
		t.Fatalf("expected depth limit, got %v", err)
	}
}

func TestListSortByKeyFailureKeepsEarlierEffects(t *testing.T) {
	source := `word fail-second(n: Int) -> Int
    print(n)
    n / (n - 1)
let ranked = list.sort-by([1, 2], fail-second(_))
`
	var out bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &out)
	if d, ok := err.(*diagnostic); !ok || d.code != CodeDivisionZero {
		t.Fatalf("expected division by zero, got %v", err)
	}
	if out.String() != "1\n" {
		t.Fatal(out.String())
	}
}

func TestListSortWordsComposeInFlows(t *testing.T) {
	source := `type Entry = record
    name: String
    score: Int
word entry-score(e: Entry) -> Int
    e.score
word entry-name(e: Entry) -> String
    e.name
let ranked = flow [3, 1, 2]
    list.sort
print(ranked[0])
print(ranked[2])
let by-score = flow [Entry("a", 2), Entry("b", 1)]
    list.sort-by(entry-score(_))
let first = by-score[0]
print(entry-name(first))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n3\nb\n" {
		t.Fatal(out.String())
	}
}

func TestListSortByInferOmittedSignatures(t *testing.T) {
	source := `type Entry = record
    name: String
    score: Int
word entry-score(e: Entry) -> Int
    e.score
word entry-name(e: Entry) -> String
    e.name
word rank(values)
    list.sort-by(values, entry-score(_))
let ranked = rank([Entry("a", 2), Entry("b", 1)])
let first = ranked[0]
print(entry-name(first))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "b\n" {
		t.Fatal(out.String())
	}
}

func TestListSortRejectsWrongTypes(t *testing.T) {
	flag := "word flag(n: Int) -> Bool\n    true\n"
	cases := []struct {
		source  string
		message string
	}{
		{"let x = list.sort([true])\n", "list.sort requires a List<Int> or List<String>, found List<Bool>"},
		{"let x = list.sort(5)\n", "list.sort requires a List<Int> or List<String>, found Int"},
		{"let x = list.sort()\n", "list.sort requires 1 argument, found 0"},
		{"let x = list.sort([1], [2])\n", "list.sort requires 1 argument, found 2"},
		{"let x = list.sort-by(5, 2)\n", "list.sort-by requires a List"},
		{"let x = list.sort-by([1])\n", "list.sort-by requires 2 arguments, found 1"},
		{"let x = list.sort-by([1], 2)\n", "list.sort-by requires a key function, found Int"},
		{flag + "let x = list.sort-by([1], flag(_))\n", "list.sort-by key must return Int or String, found Bool"},
		{flag + "word ranked(values: List<Int>) -> List<Int>\n    list.sort-by(values, flag(_))\n", "list.sort-by key must return Int or String, found Bool"},
		{flag + "word ranked(values: List<Int>)\n    list.sort-by(values, flag(_))\n", "list.sort-by key must return Int or String, found Bool"},
	}
	for _, tc := range cases {
		var out bytes.Buffer
		err := Run("test.zhex", tc.source, strings.NewReader(""), &out)
		if err == nil {
			t.Errorf("accepted wrong types: %s", tc.source)
			continue
		}
		if !strings.Contains(err.Error(), tc.message) {
			t.Errorf("%s: wrong diagnostic %q, want %q", tc.source, err.Error(), tc.message)
		}
		if out.Len() != 0 {
			t.Errorf("%s: wrong types must fail before effects, got %q", tc.source, out.String())
		}
	}
	if got := runError(t, "let x = list.sort([true])\n"); !strings.Contains(got, "hint: sort Int or String values, or rank other values with list.sort-by and a key") {
		t.Fatalf("missing sort hint: %q", got)
	}
}

func TestListSortByRejectsResultAndMismatchedKeys(t *testing.T) {
	parse := "word parse(raw: String) -> Result<Int, ParseError>\n    ok(text.to-int(raw) try)\n"
	cases := []struct {
		source  string
		message string
	}{
		{parse + "let x = list.sort-by([\"1\"], parse(_))\n", "a Result cannot be returned from a callback"},
		{parse + "word ranked(values: List<String>) -> List<String>\n    list.sort-by(values, parse(_))\n", "a Result cannot be returned from a callback"},
		{"let x = list.sort-by([1], text.trim(_))\n", "incompatible inferred types"},
		{"word add(a: Int, b: Int) -> Int\n    a + b\nlet x = list.sort-by([1], add(_, _))\n", "incompatible inferred types"},
		{"let x = list.sort-by([\"1\"], text.to-int(_))\n", "a Result cannot be returned from a callback"},
	}
	for _, tc := range cases {
		if got := runError(t, tc.source); !strings.Contains(got, tc.message) {
			t.Errorf("%s: wrong diagnostic %q, want %q", tc.source, got, tc.message)
		}
	}
}
