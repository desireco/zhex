package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestPlainIndexedWritesAndCheckedRecovery(t *testing.T) {
	source := "let values = [1, 2]\nvalues[0] = 9\nlet grid = array.new([2, 2], 0) try\ngrid[1, 1] = 7\n(values[9] = 4) catch problem\n    print(error.code(problem))\nprint(values[0] try)\nprint(grid[1, 1] try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "list-range\n9\n7\n" {
		t.Fatal(out.String())
	}
}

func TestPlainIndexedFailureIsRuntimeDiagnostic(t *testing.T) {
	for _, tc := range []struct {
		source string
		code   Code
	}{{"let values = [1]\nvalues[2] = 9\n", CodeListRange}, {"let grid = array.new([2, 2], 0) try\ngrid[0, 5] = 1\n", CodeArrayRange}, {"let grid = array.new([2, 2], 0) try\ngrid[0] = 1\n", CodeArrayRank}} {
		var out bytes.Buffer
		err := Run("test.zhex", tc.source, strings.NewReader(""), &out)
		if d, ok := err.(*diagnostic); !ok || d.code != tc.code {
			t.Errorf("wanted %s, got %v", tc.code, err)
		}
	}
}

func TestPlainIndexedWriteDoesNotInferFallibility(t *testing.T) {
	source := "word update(values: List<Int>) -> Unit\n    values[0] = 4\nlet values = [1]\nupdate(values)\nprint(values[0] try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "4\n" {
		t.Fatal(out.String())
	}
}

func TestPlainIndexedGuardFailureIsAtomicAndReleases(t *testing.T) {
	var out bytes.Buffer
	s := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := s.Enter("let values = [1]\nlet alias = values\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enter("for value in values\n    alias[0] = 9\n"); err == nil {
		t.Fatal("accepted mutation while iterating")
	} else if d, ok := err.(*diagnostic); !ok || d.code != CodeCollectionIterating {
		t.Fatalf("wrong error %v", err)
	}
	if echo, err := s.Enter("values[0] try\n"); err != nil || echo != "1" {
		t.Fatalf("mutated after failure: %q %v", echo, err)
	}
	if _, err := s.Enter("values[0] = 4\n"); err != nil {
		t.Fatalf("guard remained active: %v", err)
	}
}

func TestPlainAndCheckedWritesInferOnlyExplicitEffects(t *testing.T) {
	source := "word index(raw: String) -> Int\n    text.to-int(raw) try\nword update(values: List<Int>) -> Unit\n    values[index(\"0\") try] = 4\nword checked(values: List<Int>) -> Unit\n    (values[9] = 4) try\nlet values = [1]\nupdate(values) try\nchecked(values) catch problem\n    print(error.code(problem))\nprint(values[0] try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "list-range\n4\n" {
		t.Fatal(out.String())
	}
}

func TestPlainIndexedWriteEvaluatesOperandsOnce(t *testing.T) {
	source := "word index() -> Int\n    print(\"index\")\n    0\nword value() -> Int\n    print(\"value\")\n    4\nlet values = [1]\nvalues[index()] = value()\nprint(values[0] try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "index\nvalue\n4\n" {
		t.Fatal(out.String())
	}
}

func TestCheckedIndexedWriteCanRecoverFromIterationGuard(t *testing.T) {
	source := "let values = [1]\nfor value in values\n    (values[0] = 9) catch problem\n        print(error.code(problem))\nprint(values[0] try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "collection-iterating\n1\n" {
		t.Fatal(out.String())
	}
}

func TestPlainWritePropagatesReceiverAndValueEffects(t *testing.T) {
	source := "word values() -> List<Int>\n    let n = text.to-int(\"1\") try\n    [n]\nword value() -> Int\n    text.to-int(\"4\") try\nword update() -> Unit\n    (values() try)[0] = value() try\nupdate() try\nprint(\"done\")\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "done\n" {
		t.Fatal(out.String())
	}
}

func TestPlainWriteDiagnosticKeepsOriginAndDetails(t *testing.T) {
	source := "let values = [1]\nvalues[4] = 9\n"
	var out bytes.Buffer
	err := Run("sample.zhex", source, strings.NewReader(""), &out)
	d, ok := err.(*diagnostic)
	if !ok || d.code != CodeListRange || d.pos.file != "sample.zhex" || d.pos.line != 2 || d.operation != "list.set" || d.details["index"] != "4" || d.details["length"] != "1" {
		t.Fatalf("diagnostic %#v", err)
	}
}
