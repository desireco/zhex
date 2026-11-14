package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestPlainAndCheckedCollectionReads(t *testing.T) {
	source := `let values = [4, 8]
print(values[0])
print(values[1] try)
let fallback = values[3] catch problem
    print(error.code(problem))
    9
print(fallback)
let grid = array.new([2, 2], 7) try
print(grid[1, 1])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "4\n8\nlist-range\n9\n7\n" {
		t.Fatal(out.String())
	}
}

func TestPlainCollectionReadReportsRuntimeFailure(t *testing.T) {
	for _, source := range []string{"let xs = [1]\nprint(xs[1])\n", "let a = array.new([2,2], 0) try\nprint(a[0,3])\n"} {
		var out bytes.Buffer
		if err := Run("test.zhex", source, strings.NewReader(""), &out); err == nil {
			t.Errorf("accepted %s", source)
		}
	}
}

func TestPlainReadDoesNotRelaxResultObligations(t *testing.T) {
	for _, source := range []string{"let xs = [1]\nlet x = xs[0]\nprint(x try)\n", "let x = text.to-int(\"1\")\nprint(1)\n"} {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted %s", source)
		}
	}
}

func TestCollectionReadEvaluationOrderAndNestedFault(t *testing.T) {
	source := `word items() -> List<Int>
    print("receiver")
    [7]
word index() -> Int
    print("index")
    0
print(items()[index()])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "receiver\nindex\n7\n" {
		t.Fatal(out.String())
	}
	source = "let rows = [[1]]\nlet x = rows[5][0] catch problem\n    0\n"
	if err := Run("test.zhex", source, strings.NewReader(""), &bytes.Buffer{}); err == nil {
		t.Fatal("catch swallowed a nested receiver fault")
	}
}

func TestPlainReadPreservesNestedTryEffect(t *testing.T) {
	source := `word read(xs: List<Int>) -> Int
    xs[text.to-int("bad") try]
let value = read([2]) catch problem
    print(error.code(problem))
    4
print(value)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "text-to-int\n4\n" {
		t.Fatal(out.String())
	}
}
