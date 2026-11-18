package lang

import (
	"strings"
	"testing"
)

// #156 made every mutation word return its receiver, so a mutation call as
// an if-arm tail stops being Unit and trips the branch-type check. The
// message stays stable; the hint names the binding fix from the ticket.
func TestBranchReceiverMismatchHintShape(t *testing.T) {
	got := runError(t, "let seen = {\"a\": 1}\nif map.contains(seen, \"a\")\n    map.put(seen, \"b\", 2)\n")
	if !strings.HasPrefix(got, `bad.zhex:2:1: branch result types must match: Map<String, Int> and Unit`) {
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
		{"let xs = [1]\nif xs[0] == 1\n    list.append(xs, 2)\n", "list.append"},
		{"let xs = [1]\nif xs[0] == 1\n    list.insert(xs, 0, 2)\n", "list.insert"},
		{"let grid = array.new([2, 2], 0)\nif grid[0, 0] == 0\n    array.resize(grid, [2, 3], 1)\n", "array.resize"},
		{"let m = {\"a\": 1}\nif map.contains(m, \"a\")\n    map.put(m, \"b\", 2)\n", "map.put"},
		{"let m = {\"a\": 1}\nif map.contains(m, \"a\")\n    map.remove(m, \"a\")\n", "map.remove"},
		{"let m = {\"a\": 1}\nif map.contains(m, \"a\")\n    map.into(m, {})\n", "map.into"},
		{"let b = buffer.new()\nlet m = {\"a\": 1}\nif map.contains(m, \"a\")\n    buffer.add-line(b, \"x\")\n", "buffer.add-line"},
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

func TestBranchMismatchWithoutReceiverStaysHintless(t *testing.T) {
	got := runError(t, "let xs = [1]\nif xs[0] == 1\n    list.length(xs)\n")
	if !strings.Contains(got, "branch result types must match: Int and Unit") {
		t.Fatalf("missing mismatch: %q", got)
	}
	if strings.Contains(got, "hint:") {
		t.Fatalf("unexpected hint on a plain value mismatch: %q", got)
	}
	got = runError(t, "if 1 == 1\n    1\nelse\n    \"one\"\n")
	if strings.Contains(got, "hint:") {
		t.Fatalf("unexpected hint on matching-arm mismatch: %q", got)
	}
}
