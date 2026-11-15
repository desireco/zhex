package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestListSelectKeepsMatchingElementsUnmodified(t *testing.T) {
	source := `word even(n: Int) -> Bool
    n % 2 == 0
let values = [3, 1, 4, 1, 5, 9, 2, 6]
let kept = list.select(values, even(_))
print(list.length(kept))
print(kept[0])
print(kept[1])
print(kept[2])
print(kept[0] == values[2])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n4\n2\n6\ntrue\n" {
		t.Fatal(out.String())
	}
}

func TestListSelectEmptyAllFalseAllTrue(t *testing.T) {
	source := `word always(n: Int) -> Bool
    true
word never(n: Int) -> Bool
    false
let values = [1, 2, 3]
let empty: List<Int> = list.new()
print(list.length(list.select(empty, always(_))))
print(list.length(list.select(values, never(_))))
let copy = list.select(values, always(_))
print(list.length(copy))
list.append(copy, 4)
print(list.length(values))
print(copy[3])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "0\n0\n3\n3\n4\n" {
		t.Fatal(out.String())
	}
}

func TestListSelectFreshOuterListWithSharedElements(t *testing.T) {
	source := `word same(values: List<Int>) -> Bool
    true
let inner = [1, 2]
let outer = [inner]
let kept = list.select(outer, same(_))
list.append(inner, 3)
print(list.length(kept[0]))
list.append(kept, [4])
print(list.length(outer))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n1\n" {
		t.Fatal(out.String())
	}
}

func TestListSelectVisitsEveryElementLeftToRight(t *testing.T) {
	source := `word noisy(n: Int) -> Bool
    print(n)
    false
print(list.length(list.select([3, 1, 2], noisy(_))))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n1\n2\n0\n" {
		t.Fatal(out.String())
	}
}

func TestListSelectHoldsTraversalGuard(t *testing.T) {
	source := `word push-onto(values: List<Int>, n: Int) -> Bool
    list.append(values, n)
    true
let values = [1, 2]
let predicate = push-onto(values, _)
let kept = list.select(values, predicate)
`
	var out bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &out)
	if d, ok := err.(*diagnostic); !ok || d.code != CodeCollectionIterating {
		t.Fatalf("expected collection-iterating, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("ran effects before failing: %q", out.String())
	}
}

func TestListSelectCallbackFailureKeepsEarlierEffects(t *testing.T) {
	source := `word fail-second(n: Int) -> Bool
    print(n)
    n / (n - 1) == 0
let kept = list.select([1, 2], fail-second(_))
`
	var out bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &out)
	if d, ok := err.(*diagnostic); !ok || d.code != CodeDivisionZero {
		t.Fatalf("expected division by zero, got %v", err)
	}
	if out.String() != "1\n" {
		t.Fatalf("earlier effects lost: %q", out.String())
	}
}

func TestListSelectAcceptsBuiltinPartialsAndFlows(t *testing.T) {
	source := `let names = ["Ada", "Grace", "Alan"]
let a-names = list.select(names, text.starts-with(_, "A"))
print(list.length(a-names))
print(a-names[1])
flow names
    list.select(text.starts-with(_, "A"))
    text.join(", ")
    print
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\nAlan\nAda, Alan\n" {
		t.Fatal(out.String())
	}
}

func TestSelectAcceptsModulePredicates(t *testing.T) {
	files := map[string]string{"math.zhex": "word even(n: Int) -> Bool\n    n % 2 == 0\nword first-is-even(row: List<Int>) -> Bool\n    even(row[0])\n"}
	program, err := CompileModules("main.zhex", "use math\nlet g = array.new([2, 2], 0)\nprint(list.length(list.select([1, 2, 3], math.even(_))))\nprint(array.length(array.select(g, math.first-is-even(_))))\n", mapResolver(files))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := program.Run(strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n4\n" {
		t.Fatal(out.String())
	}
}

func TestListSelectInfersOmittedSignatures(t *testing.T) {
	source := `word even(n: Int) -> Bool
    n % 2 == 0
word evens(values)
    list.select(values, even(_))
print(list.length(evens([1, 2, 3, 4])))
print(evens([10])[0])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n10\n" {
		t.Fatal(out.String())
	}
}

func TestArraySelectKeepsMatchingRowsUnmodified(t *testing.T) {
	source := `word first-is-big(row: List<Int>) -> Bool
    row[0] > 1
let grid = array.new([3, 3], 0)
grid[0, 0] = 1
grid[0, 1] = 2
grid[0, 2] = 3
grid[1, 0] = 4
grid[1, 1] = 5
grid[1, 2] = 6
grid[2, 0] = 7
grid[2, 1] = 8
grid[2, 2] = 9
let kept = array.select(grid, first-is-big(_))
print(array.rank(kept))
print(array.length(kept))
print(array.shape(kept)[0])
print(array.shape(kept)[1])
print(kept[0, 0])
print(kept[0, 1])
print(kept[0, 2])
print(kept[1, 0])
print(kept[1, 2])
print(grid[0, 0])
print(array.length(grid))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n6\n2\n3\n4\n5\n6\n7\n9\n1\n9\n" {
		t.Fatal(out.String())
	}
}

func TestArraySelectEmptyAllFalseAllTrue(t *testing.T) {
	source := `word always(row: List<Int>) -> Bool
    true
word never(row: List<Int>) -> Bool
    false
let empty = array.new([0, 3], 0)
let none = array.select(empty, always(_))
print(array.length(none))
print(array.shape(none)[1])
let grid = array.new([2, 3], 5)
let rejected = array.select(grid, never(_))
print(array.length(rejected))
print(array.shape(rejected)[1])
let copy = array.select(grid, always(_))
print(array.length(copy))
array.resize(copy, [1, 2], 0)
print(array.length(grid))
print(copy[0, 1])
let thin = array.new([2, 0], 0)
let thin-kept = array.select(thin, always(_))
print(array.length(thin-kept))
print(array.shape(thin-kept)[0])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "0\n3\n0\n3\n6\n6\n5\n0\n2\n" {
		t.Fatal(out.String())
	}
}

func TestArraySelectSharesElementsWithSource(t *testing.T) {
	source := `word always(values: List<List<Int>>) -> Bool
    true
let seed = [7]
let grid = array.new([2, 2], seed)
let kept = array.select(grid, always(_))
list.append(seed, 8)
print(list.length(kept[0, 0]))
print(list.length(grid[1, 1]))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n2\n" {
		t.Fatal(out.String())
	}
}

func TestArraySelectPredicateReceivesFreshRows(t *testing.T) {
	source := `word extend-and-keep(row: List<Int>) -> Bool
    list.append(row, 99)
    true
let grid = array.new([2, 3], 0)
grid[1, 0] = 4
grid[1, 1] = 5
grid[1, 2] = 6
let kept = array.select(grid, extend-and-keep(_))
print(array.length(kept))
print(grid[0, 2])
print(grid[1, 0])
print(kept[1, 2])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "6\n0\n4\n6\n" {
		t.Fatal(out.String())
	}
}

func TestArraySelectVisitsEveryRowLeftToRight(t *testing.T) {
	source := `word noisy(row: List<Int>) -> Bool
    print(row[0])
    false
let grid = array.new([3, 1], 0)
grid[1, 0] = 1
grid[2, 0] = 2
print(array.length(array.select(grid, noisy(_))))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "0\n1\n2\n0\n" {
		t.Fatal(out.String())
	}
}

func TestArraySelectHoldsTraversalGuard(t *testing.T) {
	source := `word poke(row: List<Int>, grid: Array<Int>) -> Bool
    grid[0, 0] = 9
    true
let grid = array.new([2, 2], 0)
let predicate = poke(_, grid)
let kept = array.select(grid, predicate)
`
	var out bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &out)
	if d, ok := err.(*diagnostic); !ok || d.code != CodeCollectionIterating {
		t.Fatalf("expected collection-iterating, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("ran effects before failing: %q", out.String())
	}
}

func TestArraySelectRejectsRankAboveTwo(t *testing.T) {
	source := `word always(row: List<Int>) -> Bool
    true
let cube = array.new([2, 2, 2], 0)
let kept = array.select(cube, always(_))
`
	var out bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &out)
	if d, ok := err.(*diagnostic); !ok || d.code != CodeArrayRank {
		t.Fatalf("expected array-rank, got %v", err)
	}
	if !strings.Contains(err.Error(), "array.select requires a rank-2 array, found rank 3") {
		t.Fatalf("wrong diagnostic: %q", err.Error())
	}
}

func TestArraySelectComposesInFlows(t *testing.T) {
	source := `word first-is-big(row: List<Int>) -> Bool
    row[0] > 1
let grid = array.new([3, 1], 0)
grid[1, 0] = 4
grid[2, 0] = 7
flow grid
    array.select(first-is-big(_))
    array.length
    print
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n" {
		t.Fatal(out.String())
	}
}

func TestArraySelectInfersOmittedSignatures(t *testing.T) {
	source := `word first-is-big(row: List<Int>) -> Bool
    row[0] > 1
word big-rows(values)
    array.select(values, first-is-big(_))
let grid = array.new([2, 2], 0)
grid[1, 0] = 9
print(array.length(big-rows(grid)))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n" {
		t.Fatal(out.String())
	}
}

func TestSelectRejectsWrongTypesBeforeEffects(t *testing.T) {
	grid := "let g = array.new([1, 2], 0)\n"
	cases := []struct {
		source  string
		message string
	}{
		{"let x = list.select(5, 2)\n", "list.select requires a List"},
		{"let x = list.select([1], 2)\n", "argument 2 of list.select requires type (Int) -> Bool, found Int"},
		{"let x = list.select([1])\n", "list.select requires 2 arguments, found 1"},
		{grid + "let x = array.select(5, 2)\n", "array.select requires an Array"},
		{grid + "let x = array.select(g)\n", "array.select requires 2 arguments, found 1"},
		{grid + "let x = array.select(g, 2)\n", "argument 2 of array.select requires type (List<Int>) -> Bool, found Int"},
		{grid + "let x = array.select(g, even-rows(_))\n" + "word even-rows(n: Int) -> Bool\n    n % 2 == 0\n", "incompatible inferred types"},
		{"word double(n: Int) -> Int\n    n + n\nlet x = list.select([1], double(_))\n", "incompatible inferred types"},
		{"word parse(raw: String) -> Result<Int, ParseError>\n    ok(text.to-int(raw) try)\nlet x = list.select([\"1\"], parse(_))\n", "incompatible inferred types"},
		{"word parse(raw: String) -> Result<Int, ParseError>\n    ok(text.to-int(raw) try)\nlet s = array.new([1, 2], \"x\")\nlet x = array.select(s, parse(_))\n", "incompatible inferred types"},
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
}
