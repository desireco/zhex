package lang

import (
	"strings"
	"testing"
)

// #156 made every mutation word return its receiver, so a mutation call as
// a branch tail stops being Unit and trips the branch-type check wherever
// the branch's value is read (#226 narrowed the check to those positions:
// a branch standing as an ordinary line drops its value and may end with
// any type). The message stays stable; the hint names the binding fix from
// the ticket.
func TestBranchReceiverMismatchHintShape(t *testing.T) {
	got := runError(t, "let seen = {\"a\": 1}\nlet stored = if map.contains(seen, \"a\")\n    map.put(seen, \"b\", 2)\nelse\n    print(\"missing\")\n")
	if !strings.HasPrefix(got, `bad.zhex:2:14: branch result types must match: Map<String, Int> and Unit`) {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "map.put returns the map it changed") || !strings.Contains(got, "`let stored = map.put(...)`") {
		t.Fatalf("missing binding hint: %q", got)
	}
}

func TestBranchReceiverMismatchHintCoversEachMutationWord(t *testing.T) {
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
		if !strings.Contains(got, tc.word+" returns the") || !strings.Contains(got, "`let stored = "+tc.word+"(...)`") {
			t.Fatalf("%s: missing receiver hint: %q", tc.word, got)
		}
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

func TestBranchMismatchWithoutReceiverStaysHintless(t *testing.T) {
	got := runError(t, "let v = if 1 == 1\n    list.length([1])\nelse\n    print(\"no\")\n")
	if !strings.Contains(got, "branch result types must match: Int and Unit") {
		t.Fatalf("missing mismatch: %q", got)
	}
	if strings.Contains(got, "hint:") {
		t.Fatalf("unexpected hint on a plain value mismatch: %q", got)
	}
	got = runError(t, "let v = if 1 == 1\n    1\nelse\n    \"one\"\n")
	if strings.Contains(got, "hint:") {
		t.Fatalf("unexpected hint on matching-arm mismatch: %q", got)
	}
}
