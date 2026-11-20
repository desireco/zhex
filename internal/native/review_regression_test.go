package native

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"zhex/internal/lang"
)

func TestNativeReviewBooleanGrouping(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"not-group", "let a = false\nlet b = true\nif not (a or b)\n    print(\"true\")\nelse\n    print(\"false\")\n"},
		{"left-group", "let a = true\nlet b = false\nlet c = false\nif (a or b) and c\n    print(\"true\")\nelse\n    print(\"false\")\n"},
		{"equality-group", "let a = true\nlet b = false\nlet c = false\nif a == (b == c)\n    print(\"true\")\nelse\n    print(\"false\")\n"},
	} {
		t.Run(tc.name, func(t *testing.T) { diffCase(t, tc.name, tc.source, "") })
	}
}

func TestNativeReviewRangeBoundsOnce(t *testing.T) {
	diffCase(t, "range-bounds", "word bound() -> Int\n    print(\"bound\")\n    2\nfor n in range(0, bound())\n    print(n)\n", "")
	diffCase(t, "range-snapshot", "var end = 2\nfor n in range(0, end)\n    end = 0\n    print(n)\n", "")
}

func TestNativeReviewBlockAssignments(t *testing.T) {
	diffCase(t, "block-assign", "var x = 1\nx = if true\n    2\nelse\n    3\nprint(x)\nx = match Option.Some(4)\n    case Option.Some(value)\n        value\n    case Option.None\n        0\nprint(x)\n", "")
}

func TestNativeReviewUnusedBindings(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"list-neither", "for index, value in [1]\n    print(\"once\")\n"},
		{"list-index", "for index, value in [1]\n    print(index)\n"},
		{"list-value", "for index, value in [1]\n    print(value)\n"},
		{"array-neither", "let a = array.new([1], 7)\nfor indexes, value in a\n    print(\"once\")\n"},
		{"array-index", "let a = array.new([1], 7)\nfor indexes, value in a\n    print(indexes[0])\n"},
		{"array-value", "let a = array.new([1], 7)\nfor indexes, value in a\n    print(value)\n"},
		{"map-neither", "for value in {\"a\": 1}\n    print(\"once\")\n"},
		{"map-dual-neither", "for key, value in {\"a\": 1}\n    print(\"once\")\n"},
		{"match-option", "match Option.Some(1)\n    case Option.Some(value)\n        print(\"some\")\n    case Option.None\n        print(\"none\")\n"},
		{"match-json", "let j = json.parse(\"1\") try\nmatch j\n    case Json.Number(value)\n        print(\"number\")\n    case _\n        print(\"other\")\n"},
	} {
		t.Run(tc.name, func(t *testing.T) { diffCase(t, tc.name, tc.source, "") })
	}
	t.Run("stream", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "input.txt")
		if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		diffCase(t, "stream-unused", fmt.Sprintf("for line in file.stream(%q)\n    print(\"line\")\n", path), "")
	})
}

func TestNativeReviewArrayShape(t *testing.T) {
	diffCase(t, "array-shape-text", "let a = array.new([2], \"a\")\nprint(array.shape(a)[0])\n", "")
}

func TestNativeReviewRelativeOutput(t *testing.T) {
	requireGo(t)
	dir, err := os.MkdirTemp(".", ".native-relative-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	rel := filepath.Join(dir, "program")
	outDir, err := filepath.Abs(rel)
	if err != nil {
		t.Fatal(err)
	}
	if err := Compile("program.zhex", "print(\"hi\")\n", rel, nil); err != nil {
		t.Fatal(err)
	}
	out, diag, code := runBinary(t, filepath.Join(outDir, "program"), "")
	if out != "hi\n" || diag != "" || code != 0 {
		t.Fatalf("stdout=%q stderr=%q status=%d", out, diag, code)
	}
}

func TestNativeReviewEvaluationOrder(t *testing.T) {
	const words = "word first() -> Int\n    print(\"first\")\n    1\nword second() -> String\n    print(\"second\")\n    \"2\"\nword add(a: Int, b: Int) -> Int\n    a + b\n"
	for _, tc := range []struct{ name, source string }{
		{"call-order", "print(add(first(), text.to-int(second()) try))\n"},
		{"binary-order", "print(first() + text.to-int(second()) try)\n"},
		{"list-order", "let xs = [first(), text.to-int(second()) try]\nprint(xs[0] + xs[1])\n"},
		{"map-order", "let m = {\"a\": first(), \"b\": text.to-int(second()) try}\nprint(map.length(m))\n"},
		{"record-order", "type Pair = record\n    a: Int\n    b: Int\nlet p = Pair(first(), text.to-int(second()) try)\nprint(p.a + p.b)\n"},
		{"named-record-order", "type Pair = record\n    a: Int\n    b: Int\nlet p = Pair(b: first(), a: text.to-int(second()) try)\nprint(p.a + p.b)\n"},
		{"named-record-inline-order", "type Pair = record\n    a: String\n    b: Int\nlet p = Pair(b: first(), a: second())\nprint(p.a + p.b)\n"},
		{"index-order", "word values() -> List<Int>\n    print(\"receiver\")\n    [10, 20, 30]\nprint(values()[text.to-int(second()) try])\n"},
		{"write-order", "let xs = [0, 0, 0]\nxs[first()] = text.to-int(second()) try\nprint(xs[1])\n"},
		{"default-order", "word defaulted() -> Int\n    print(\"default\")\n    2\nword finish(a: Int, b: Int = [defaulted()][0]) -> Int\n    a + b\nprint(finish(first()))\n"},
		{"short-circuit-order", "word yes() -> Bool\n    print(\"left\")\n    true\nif yes() or text.to-int(second()) try == 2\n    print(\"true\")\n"},
	} {
		t.Run(tc.name, func(t *testing.T) { diffCase(t, tc.name, words+tc.source, "") })
	}
}

func TestNativeReviewPartialDefaultTiming(t *testing.T) {
	diffCase(t, "partial-default-time", "word defaulted() -> Int\n    print(\"default\")\n    2\nword finish(a: Int, b: Int = [defaulted()][0]) -> Int\n    a + b\nlet f = finish(_)\nprint(\"created\")\nprint(f(1))\nprint(f(2))\n", "")
}

func TestNativeReviewModuleDefaults(t *testing.T) {
	requireGo(t)
	const moduleSource = "word _default() -> Int\n    print(\"default\")\n    2\nword finish(a: Int = _default()) -> Int\n    a\n"
	resolve := func(name string) (lang.ModuleSource, bool) {
		return lang.ModuleSource{Path: "defaults.zhex", Source: moduleSource}, name == "defaults"
	}
	outDir := filepath.Join(t.TempDir(), "defaults")
	if err := Compile("main.zhex", "use defaults\nprint(defaults.finish())\n", outDir, resolve); err != nil {
		t.Fatal(err)
	}
	out, diag, code := runBinary(t, filepath.Join(outDir, "defaults"), "")
	if out != "default\n2\n" || diag != "" || code != 0 {
		t.Fatalf("stdout=%q stderr=%q status=%d", out, diag, code)
	}
}

func TestNativeReviewUnusedBlockValues(t *testing.T) {
	diffCase(t, "unused-block", "let unused = if true\n    print(\"effect\")\n    1\nelse\n    0\nlet other = flow \"flow effect\"\n    print\n", "")
}

func TestNativeReviewIdentifierCollisions(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"runtime-local", "let zhexrt = 1\nprint(zhexrt)\n"},
		{"runtime-word", "word zhexrt() -> Int\n    1\nprint(zhexrt())\n"},
		{"later-word", "word caller(later: Int) -> Int\n    later(2)\nword later(n: Int) -> Int\n    n + 1\nprint(caller(1))\n"},
		{"position-local", "let _p2_1 = 1\nprint(_p2_1 + 1)\n"},
	} {
		t.Run(tc.name, func(t *testing.T) { diffCase(t, tc.name, tc.source, "") })
	}
}

func TestNativeReviewColonSourcePath(t *testing.T) {
	const name, source = "C:/source/program", "print(1 / 0)\n"
	wantOut, wantDiag, wantCode := interpret(t, name, source, "")
	outDir := filepath.Join(t.TempDir(), "colon-path")
	if err := Compile(name+".zhex", source, outDir, nil); err != nil {
		t.Fatal(err)
	}
	out, diag, code := runBinary(t, filepath.Join(outDir, "colon-path"), "")
	if out != wantOut || diag != wantDiag || code != wantCode {
		t.Fatalf("native stdout=%q stderr=%q status=%d; interpreter stdout=%q stderr=%q status=%d", out, diag, code, wantOut, wantDiag, wantCode)
	}
}

func TestNativeReviewMapIntoOrder(t *testing.T) {
	diffCase(t, "map-into-order", "word source() -> Map<String, Int>\n    print(\"source\")\n    {\"a\": 1}\nword target() -> Map<String, Int>\n    print(\"target\")\n    let m: Map<String, Int> = {}\n    m\nlet merged = map.into(source(), target())\nprint(map.length(merged))\n", "")
}

func TestNativeReviewImportedSignatureTypes(t *testing.T) {
	const moduleSource = "type Pair = record\n    x: Int\ntype Nested = record\n    pair: Pair\n"
	resolve := func(name string) (lang.ModuleSource, bool) {
		return lang.ModuleSource{Path: "model.zhex", Source: moduleSource}, name == "model"
	}
	outDir := filepath.Join(t.TempDir(), "types")
	if err := Compile("main.zhex", "use model\nword show(p: model.Nested) -> Int\n    p.pair.x\nprint(\"ready\")\n", outDir, resolve); err != nil {
		t.Fatal(err)
	}
	out, diag, code := runBinary(t, filepath.Join(outDir, "types"), "")
	if out != "ready\n" || diag != "" || code != 0 {
		t.Fatalf("stdout=%q stderr=%q status=%d", out, diag, code)
	}
}

func TestNativeReviewNestedDefaultCalls(t *testing.T) {
	diffCase(t, "nested-default", "word f(a: Int, b: Int = 1) -> Int\n    a + b\nprint(f(f(1)))\n", "")
}
