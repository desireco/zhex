package lang

import (
	"bytes"
	"strings"
	"testing"
)

// #182: postfix chains mix indexed reads and field reads freely —
// space.bytes[index].x means exactly what naming the intermediate means.

func TestIndexedReadThenFieldReadEvaluates(t *testing.T) {
	source := "type Cell = record\n    x: Int\n\ntype Grid = record\n    cells: List<Cell>\n\nlet g = Grid(cells: [Cell(x: 1), Cell(x: 5)])\nprint(g.cells[1].x)\nprint(g.cells[0].x)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "5\n1\n" {
		t.Fatalf("output = %q", got)
	}
}

// The day-18 evidence shape: a record field of records, indexed then read.
func TestInstructionListFieldChainEvaluates(t *testing.T) {
	source := "type Instruction = record\n    opcode: Int\n\ntype Space = record\n    bytes: List<Instruction>\n\nlet space = Space(bytes: [Instruction(opcode: 7), Instruction(opcode: 9)])\nprint(space.bytes[1].opcode)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "9\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestMixedPostfixChainsEvaluateLikeNamedIntermediates(t *testing.T) {
	source := "type Inner = record\n    value: Int\n\ntype Row = record\n    cells: List<Inner>\n\ntype Table = record\n    rows: List<Row>\n\nlet t = Table(rows: [Row(cells: [Inner(value: 3)]), Row(cells: [Inner(value: 4), Inner(value: 8)])])\nlet named = t.rows[1]\nlet cell = named.cells[0]\nprint(cell.value)\nprint(t.rows[1].cells[0].value)\nprint(t.rows[0].cells[0].value)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "4\n4\n3\n" {
		t.Fatalf("output = %q", got)
	}
}

// Arrays join the chain too: an indexed array cell followed by a field read.
func TestArrayCellThenFieldReadEvaluates(t *testing.T) {
	source := "type Cell = record\n    x: Int\n\nlet grid = array.new([2, 2], Cell(x: 0)) try\ngrid[1, 1] = Cell(x: 6)\nprint(grid[1, 1].x)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "6\n" {
		t.Fatalf("output = %q", got)
	}
}

// Field-then-index (the symmetric direction) already works via dotted name
// tokens; pin it beside the new direction.
func TestFieldThenIndexedReadStillEvaluates(t *testing.T) {
	source := "type Cell = record\n    x: Int\n\ntype Grid = record\n    cells: List<Cell>\n\nlet g = Grid(cells: [Cell(x: 2)])\nprint(g.cells[0].x)\nlet xs: List<Int> = [10, 20]\nprint(xs[1])\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "2\n20\n" {
		t.Fatalf("output = %q", got)
	}
}

// Invalid chains keep the named-intermediate diagnostics: a missing field is
// the same unknown-field error, and a field after a non-record element names
// the type it found.
func TestChainedMissingFieldReportsLikeNamedIntermediate(t *testing.T) {
	if _, err := Compile("test.zhex", "type Cell = record\n    x: Int\n\nlet g = Cell(x: 1)\nlet xs: List<Cell> = [g]\nprint(xs[0].nope)\n"); err == nil || !strings.Contains(err.Error(), `type Cell has no field "nope"`) {
		t.Fatalf("missing chained field diagnostic: %v", err)
	}
	if _, err := Compile("test.zhex", "let xs: List<Int> = [1]\nprint(xs[0].nope)\n"); err == nil || !strings.Contains(err.Error(), `type Int has no field "nope"`) {
		t.Fatalf("non-record chained field diagnostic: %v", err)
	}
}

// Genuinely malformed chains keep the lexer's unexpected-character error:
// a dot must still start a name to continue a chain.
func TestMalformedChainKeepsUnexpectedCharacter(t *testing.T) {
	for _, source := range []string{
		"let xs: List<Int> = [1]\nprint(xs[0].)\n",
		"let xs: List<Int> = [1]\nprint(xs[0].1)\n",
	} {
		if _, err := Compile("test.zhex", source); err == nil || !strings.Contains(err.Error(), "unexpected character") {
			t.Fatalf("malformed chain must keep the lexer diagnostic, got: %v", err)
		}
	}
}

// A leading dot that cannot start an expression still quotes the source
// text exactly: the lexer strips the dot from field tokens, so the parser
// must put it back — the message names ".x", never "x".
func TestLeadingDotExpressionErrorQuotesTheDot(t *testing.T) {
	for _, source := range []string{
		".x\n",
		"let a = .x\n",
		"print(.x)\n",
	} {
		_, err := Compile("test.zhex", source)
		if err == nil || !strings.Contains(err.Error(), `expected an expression, found ".x"`) {
			t.Fatalf("source %q must quote the dot, got: %v", source, err)
		}
	}
}
