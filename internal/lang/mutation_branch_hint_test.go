package lang

import (
	"strings"
	"testing"
)

// The branch-type hints after #229: where a branch's value is read — an
// `if` or `match` in a binding, a word tail, any reader — the message's
// first line is unchanged and the hint (branchMismatchHint) names both
// actions: give both arms the same type, or put the block on an ordinary
// line to drop what the arms produce (#226). The old receiver-specific
// bind advice is gone: the case it worked around is legal now.

func TestBranchMismatchIfNamesBothActions(t *testing.T) {
	got := runError(t, "let v = if 1 == 1\n    1\nelse\n    \"one\"\n")
	if !strings.HasPrefix(got, "bad.zhex:1:9: branch result types must match: Int and String") {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "hint: "+branchMismatchHint) {
		t.Fatalf("missing two-action hint: %q", got)
	}
}

func TestBranchMismatchMatchNamesBothActions(t *testing.T) {
	got := runError(t, "type Shape = choice\n    Dot\n    Ring(radius: Int)\n\nlet v = match Shape.Dot\n    case Shape.Dot\n        1\n    case Shape.Ring(radius)\n        \"ring\"\n")
	if !strings.HasPrefix(got, "bad.zhex:8:5: match case result types must match: Int and String") {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "hint: "+branchMismatchHint) {
		t.Fatalf("missing two-action hint: %q", got)
	}
}

// #156 made every mutation word return its receiver, so a mutation call as
// a branch tail stops being Unit wherever the branch's value is read. The
// hint is the shared two-action one — no receiver wording, no binding
// advice: the bind work-around served the ordinary-line case #226 made
// legal.
func TestBranchReceiverMismatchCoversEachMutationWord(t *testing.T) {
	cases := []struct {
		source string
		word   string
	}{
		{"let xs = [1]\nlet stored = if xs[0] == 1\n    list.append(xs, 2)\nelse\n    print(\"kept\")\n", "list.append"},
		{"let xs = [1]\nlet stored = if xs[0] == 1\n    list.insert(xs, 0, 2)\nelse\n    print(\"kept\")\n", "list.insert"},
		{"let grid = array.new([2, 2], 0)\nlet stored = if grid[0, 0] == 0\n    array.resize(grid, [2, 3], 1)\nelse\n    print(\"kept\")\n", "array.resize"},
		{"let m = {\"a\": 1}\nlet stored = if map.contains(m, \"a\")\n    map.put(m, \"b\", 2)\nelse\n    print(\"kept\")\n", "map.put"},
		{"let m = {\"a\": 1}\nlet stored = if map.contains(m, \"a\")\n    map.remove(m, \"a\")\nelse\n    print(\"kept\")\n", "map.remove"},
		{"let m = {\"a\": 1}\nlet stored = if map.contains(m, \"a\")\n    map.into(m, {})\nelse\n    print(\"kept\")\n", "map.into"},
		{"let b = buffer.new()\nlet m = {\"a\": 1}\nlet stored = if map.contains(m, \"a\")\n    buffer.add-line(b, \"x\")\nelse\n    print(\"kept\")\n", "buffer.add-line"},
		{"let b = buffer.new()\nbuffer.add-line(b, \"x\")\nlet m = {\"a\": 1}\nlet stored = if map.contains(m, \"a\")\n    buffer.set-line(b, 0, \"y\") try\nelse\n    print(\"kept\")\n", "buffer.set-line"},
	}
	for _, tc := range cases {
		got := runError(t, tc.source)
		if !strings.Contains(got, "branch result types must match") {
			t.Fatalf("%s: missing branch mismatch: %q", tc.word, got)
		}
		if !strings.Contains(got, "hint: "+branchMismatchHint) {
			t.Fatalf("%s: missing two-action hint: %q", tc.word, got)
		}
		if strings.Contains(got, "returns the") || strings.Contains(got, "bind it as") {
			t.Fatalf("%s: obsolete receiver wording: %q", tc.word, got)
		}
	}
}

// A word with no declared result whose last line is an `if` or `match`
// with arms of different types still fails — the owner's wording says
// what to do about it (#229). Nested reads inside the word keep the plain
// annotation hint: only the word's own answer carries the Unit question.
func TestInferredWordTailBranchHintSaysUnit(t *testing.T) {
	got := runError(t, "word pick(c: Bool)\n    if c\n        1\n    else\n        \"one\"\n")
	if !strings.HasPrefix(got, "bad.zhex:2:5: incompatible inferred types") {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "hint: if this word returns nothing, declare `-> Unit`; otherwise give both arms the same type") {
		t.Fatalf("missing Unit hint: %q", got)
	}
	got = runError(t, "type Shape = choice\n    Dot\n    Ring(radius: Int)\n\nword describe(s: Shape)\n    match s\n        case Shape.Dot\n            1\n        case Shape.Ring(radius)\n            \"ring\"\n")
	if !strings.Contains(got, "incompatible inferred types") {
		t.Fatalf("match tail: missing mismatch: %q", got)
	}
	if !strings.Contains(got, "hint: if this word returns nothing, declare `-> Unit`; otherwise give both arms the same type") {
		t.Fatalf("match tail: missing Unit hint: %q", got)
	}
	// A read that is not the word's answer keeps the annotation hint.
	got = runError(t, "word nested(c: Bool)\n    if c\n        if c\n            1\n        else\n            \"x\"\n    else\n        print(2)\n")
	if !strings.Contains(got, "hint: add an explicit return annotation or correct the conflicting values") {
		t.Fatalf("nested read lost the annotation hint: %q", got)
	}
}

// #226: a branch standing as an ordinary line is no longer an error, so
// the same shapes run and produce their effects.
func TestBranchReceiverOrdinaryLineRuns(t *testing.T) {
	cases := []struct {
		source string
		word   string
	}{
		{"let xs = [1]\nif xs[0] == 1\n    list.append(xs, 2)\nprint(int.to-string(list.length(xs)))\n", "list.append"},
		{"let xs = [1]\nif xs[0] == 1\n    list.insert(xs, 0, 2)\nprint(int.to-string(list.length(xs)))\n", "list.insert"},
		{"let grid = array.new([2, 2], 0)\nif grid[0, 0] == 0\n    array.resize(grid, [2, 3], 1)\nprint(int.to-string(array.shape(grid)[0]))\n", "array.resize"},
		{"let m = {\"a\": 1}\nif map.contains(m, \"a\")\n    map.put(m, \"b\", 2)\nprint(map.contains(m, \"b\"))\n", "map.put"},
		{"let m = {\"a\": 1}\nif map.contains(m, \"a\")\n    map.remove(m, \"a\")\nprint(map.contains(m, \"a\"))\n", "map.remove"},
		{"let m = {\"a\": 1}\nif map.contains(m, \"a\")\n    map.into(m, {})\nprint(int.to-string(map.length(m)))\n", "map.into"},
		{"let b = buffer.new()\nlet m = {\"a\": 1}\nif map.contains(m, \"a\")\n    buffer.add-line(b, \"x\")\nprint(int.to-string(buffer.line-count(b)))\n", "buffer.add-line"},
		{"let b = buffer.new()\nbuffer.add-line(b, \"x\")\nlet m = {\"a\": 1}\nif map.contains(m, \"a\")\n    buffer.set-line(b, 0, \"y\") try\nprint(buffer.to-text(b))\n", "buffer.set-line"},
	}
	for _, tc := range cases {
		runOK(t, tc.source)
	}
}
