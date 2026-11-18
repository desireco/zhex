package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestListInsertAtHeadMiddleAndEnd(t *testing.T) {
	source := `let xs = [2, 3, 5]
list.insert(xs, 0, 1)
list.insert(xs, 2, 4)
list.insert(xs, 5, 6)
print(xs[0])
print(xs[1])
print(xs[2])
print(xs[3])
print(xs[4])
print(xs[5])
let words = ["b"]
list.insert(words, 0, "a")
list.insert(words, 2, "c")
print(words[0])
print(words[1])
print(words[2])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n2\n4\n3\n5\n6\na\nb\nc\n" {
		t.Fatal(out.String())
	}
}

func TestListInsertIntoEmptyList(t *testing.T) {
	source := `let xs: List<Int> = list.new()
list.insert(xs, 0, 7)
print(xs[0])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}

func TestListInsertReturnsItsReceiver(t *testing.T) {
	source := `let xs = [1]
let same = list.insert(xs, 0, 0)
list.append(same, 2)
print(list.length(xs))
print(xs[0])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n0\n" {
		t.Fatal(out.String())
	}
}

func TestListInsertThreadsInFlows(t *testing.T) {
	source := `let xs = [3]
flow xs
    list.insert 0, 1
    list.insert 2, 4
    list.length
    print
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n" {
		t.Fatal(out.String())
	}
}

func TestListInsertPartiallyAppliesIndex(t *testing.T) {
	source := `let put: (List<Int>, Int) -> List<Int> = list.insert _, 0, _
let xs = put([2], 1)
print(xs[0])
print(xs[1])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n2\n" {
		t.Fatal(out.String())
	}
}

func TestListInsertInfersWordParameters(t *testing.T) {
	source := `word put-first(xs, item: Int) -> Int
    list.insert(xs, 0, item)
    list.length(xs)
print(put-first([2, 3], 1))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n" {
		t.Fatal(out.String())
	}
}

func TestListInsertOutOfRangeIsARuntimeDiagnostic(t *testing.T) {
	for _, index := range []string{"-1", "2"} {
		source := "let xs = [1]\nlist.insert(xs, " + index + ", 2)\n"
		err := Run("bad.zhex", source, strings.NewReader(""), &strings.Builder{})
		if err == nil || !strings.Contains(err.Error(), "list insert index is out of range") {
			t.Fatalf("index %s: wanted range diagnostic, got %v", index, err)
		}
		if code, _ := codeOf(t, err); code != "list-range" {
			t.Fatalf("index %s: wanted code list-range, got %s", index, code)
		}
	}
}

func TestListInsertCheckedResultSelectsFailure(t *testing.T) {
	source := `let xs = [1]
let kept = list.insert(xs, 5, 2) catch problem
    print(error.code(problem))
    xs
print(list.length(kept))
word put-at(xs: List<Int>, i: Int) -> Int
    let grown = list.insert(xs, i, 0) try
    list.length(grown)
print(put-at([1], 1) try)
let n = put-at([1], 5) catch problem
    print(error.code(problem))
    0
print(n)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "list-range\n1\n2\nlist-range\n0\n" {
		t.Fatal(out.String())
	}
}

func TestListInsertDuringIterationFailsBeforeMutation(t *testing.T) {
	source := `let xs = [1, 2]
for v in xs
    list.insert(xs, 0, 9)
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

func TestListInsertElementLimit(t *testing.T) {
	source := `var big: List<Int> = list.new()
for i in range(0, 1000000)
    big = list.append(big, i)
list.insert(big, 0, 1)
`
	err := Run("big.zhex", source, strings.NewReader(""), &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "list exceeds element limit") {
		t.Fatalf("wanted element limit diagnostic, got %v", err)
	}
	report, ok := Report(err)
	if !ok || !strings.Contains(report, `"code":"collection-limit"`) {
		t.Fatalf("wanted code collection-limit, got %s", report)
	}
}

func TestListInsertRejectsWrongTypes(t *testing.T) {
	cases := []struct {
		source  string
		message string
	}{
		{"let x = list.insert(5, 0, 1)\n", "list.insert requires a List, an Int index, and a matching element"},
		{"let x = list.insert([1], \"0\", 1)\n", "list.insert requires a List, an Int index, and a matching element"},
		{"let x = list.insert([1], 0, \"1\")\n", "list.insert requires a List, an Int index, and a matching element"},
		{"let x = list.insert([1], 0)\n", "list.insert requires 3 arguments, found 2"},
		{"let x = list.insert([1], 0, 2, 3)\n", "list.insert requires 3 arguments, found 4"},
	}
	for _, tc := range cases {
		if got := runError(t, tc.source); !strings.Contains(got, tc.message) {
			t.Errorf("%s: wrong diagnostic %q, want %q", tc.source, got, tc.message)
		}
	}
}
