package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestPlainAndCheckedCollectionOperations(t *testing.T) {
	source := `let xs = [1]
list.append(xs, 2)
let grid = array.new([2,2], 3)
array.resize(grid, [2,3], 4)
print(xs[1])
print(grid[1,2])
let fallback = array.new([-1,2], 0) catch problem
    print(error.code(problem))
    array.new([2,2], 1)
print(fallback[0,0])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n4\narray-shape\n1\n" {
		t.Fatal(out.String())
	}
}

func TestMutationWordsReturnTheirReceiver(t *testing.T) {
	source := `let xs = [1]
let same = list.append(xs, 2)
print(same[1])
list.append(same, 3)
print(list.length(xs))
let grid = array.new([2, 2], 0)
let grown = array.resize(grid, [2, 3], 7)
print(array.length(grown))
grid[1, 2] = 5
print(grown[1, 2])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n3\n6\n5\n" {
		t.Fatal(out.String())
	}
}

func TestCheckedMutationsUnwrapToTheReceiver(t *testing.T) {
	source := `let xs = [1]
let kept = list.append(xs, 2) try
print(list.length(kept))
let grid = array.new([2, 2], 0)
let copied = array.resize(grid, [3, 2], 9) try
print(array.length(copied))
let bad = array.resize(grid, [3], 0) catch problem
    print(error.code(problem))
    array.resize(grid, [2, 2], 0) try
print(array.length(bad))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n6\narray-rank\n4\n" {
		t.Fatal(out.String())
	}
}

func TestChainedMutationFlowThreadsTheCollection(t *testing.T) {
	source := `let items: List<Int> = list.new()
flow items
    list.append(1)
    list.append(2)
    list.length
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

func TestPlainCollectionFailureAndScopedCatch(t *testing.T) {
	var out bytes.Buffer
	if err := Run("test.zhex", "let x = array.new([-1,2], 0)\n", strings.NewReader(""), &out); err == nil {
		t.Fatal("missing shape diagnostic")
	}
	source := "let x = array.new([2,2], 1 / 0) catch problem\n    array.new([2,2], 0)\n"
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err == nil || !strings.Contains(err.Error(), "zero") {
		t.Fatalf("catch swallowed argument fault: %v", err)
	}
}

func TestPlainCollectionCallsDoNotInferEffect(t *testing.T) {
	source := `word make() -> Int
    let a = array.new([2,2], 7)
    a[0,0]
print(make())
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}

func TestCheckedCollectionFlowAndArgumentEffects(t *testing.T) {
	source := `word grow(xs: List<Int>) -> Unit
    let grown = flow xs
        list.append(2) try
let xs = [1]
grow(xs) try
print(xs[1])
word make() -> Array<Int>
    array.new([text.to-int("bad") try, 2], 0)
let grid = make() catch problem
    print(error.code(problem))
    array.new([2,2], 0)
print(grid[0,0])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\ntext-to-int\n0\n" {
		t.Fatal(out.String())
	}
}
