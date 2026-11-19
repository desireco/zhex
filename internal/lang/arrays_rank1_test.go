package lang

import (
	"bytes"
	"strings"
	"testing"
)

// #183: arrays allow rank 1 — one-dimensional tables with indexed writes,
// flowing through the whole surface. array.select stays rank-2-only.

func TestRank1ArrayCreateReadWriteResize(t *testing.T) {
	source := "let a = array.new([4], 0)\nprint(array.rank(a))\nprint(array.length(a))\nprint(array.shape(a)[0])\na[2] = 9\nprint(a[2])\nlet grown = array.resize(a, [6], 0)\nprint(array.length(grown))\nprint(grown[2])\ngrown[5] = 4\nprint(grown[5])\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "1\n4\n4\n9\n6\n9\n4\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestRank1ArrayZeroExtentBoundsAndCheckedRead(t *testing.T) {
	source := "let a = array.new([0], 0)\nprint(array.length(a))\nlet b = array.new([3], 0)\nlet v = b[9] catch -1\nprint(v)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "0\n-1\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestRank1ArrayWrongRankIndexFails(t *testing.T) {
	err := Run("test.zhex", "let a = array.new([3], 0)\na[1, 0] = 5\n", strings.NewReader(""), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "array index has the wrong rank") {
		t.Fatalf("wrong-rank rank-1 index must fail, got: %v", err)
	}
}

func TestRank0ShapeFailsArrayRank(t *testing.T) {
	p, err := Compile("test.zhex", "let a = array.new([], 0)\n")
	if err != nil {
		t.Fatal(err)
	}
	var report ErrorReport
	err = p.RunWithOptions(strings.NewReader(""), &bytes.Buffer{}, RunOptions{IncludeSensitive: true, Reporter: func(e ErrorReport) error { report = e; return nil }})
	if err == nil || !strings.Contains(err.Error(), "array rank must be between 1 and 32") {
		t.Fatalf("rank 0 must fail the rank bound, got: %v", err)
	}
	if report.Error.Details["min_rank"] != "1" || report.Error.Details["max_rank"] != "32" {
		t.Fatalf("rank diagnostic keeps its bound details, got: %+v", report.Error)
	}
}

func TestRank1ArrayIterationDualBinding(t *testing.T) {
	source := "let a = array.new([3], 0)\na[1] = 5\nvar sum = 0\nfor coords, value in a\n    sum = sum + value + coords[0]\nprint(sum)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "8\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestRank1ArraySelectStaysRank2Only(t *testing.T) {
	source := "word nonempty(row: List<Int>) -> Bool\n    list.length(row) > 0\nlet a = array.new([3], 1)\nlet kept = array.select(a, nonempty(_))\nprint(array.length(kept))\n"
	err := Run("test.zhex", source, strings.NewReader(""), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "array.select requires a rank-2 array, found rank 1") {
		t.Fatalf("rank-1 array.select must keep its rank-2 rule, got: %v", err)
	}
}

// The day-19 shape: a backwards dynamic-programming fill over one table,
// natively rank 1 instead of the one-column workaround.
func TestRank1BackwardFillEvaluates(t *testing.T) {
	source := "let n = 6\nlet ways = array.new([n + 1], 0)\nways[n] = 1\nvar i = n\nwhile i > 0\n    i = i - 1\n    var step = ways[i + 1]\n    if i + 2 <= n\n        step = step + ways[i + 2]\n    ways[i] = step\nprint(ways[0])\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "13\n" {
		t.Fatalf("output = %q", got)
	}
}
