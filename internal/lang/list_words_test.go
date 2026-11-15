package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestListMapFilterFoldTransformLists(t *testing.T) {
	source := `word double(n: Int) -> Int
    n + n
word even(n: Int) -> Bool
    n % 2 == 0
word add(a: Int, b: Int) -> Int
    a + b
let values = [1, 2, 3, 4]
let doubled = list.map(values, double(_))
let evens = list.filter(values, even(_))
let total = list.fold(values, 0, add(_, _))
print(doubled[0])
print(doubled[3])
print(list.length(evens))
print(evens[1])
print(total)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n8\n2\n4\n10\n" {
		t.Fatal(out.String())
	}
}

func TestListWordsAcceptBuiltinPartialCallbacks(t *testing.T) {
	source := `let names = ["Ada", "Grace", "Alan"]
let a-names = list.filter(names, text.contains(_, "Al"))
print(list.length(a-names))
print(a-names[0])
flow text.split(" Ada , Grace ", ",")
    list.map(text.trim(_))
    text.join(" / ")
    print
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\nAlan\nAda / Grace\n" {
		t.Fatal(out.String())
	}
}

func TestListFoldAccumulatesAnotherType(t *testing.T) {
	source := `word join-count(text: String, n: Int) -> String
    text + int.to-string(n)
print(list.fold([7, 8, 9], "n:", join-count(_, _)))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "n:789\n" {
		t.Fatal(out.String())
	}
}

func TestListWordsEmptyInput(t *testing.T) {
	source := `word double(n: Int) -> Int
    n + n
word always(n: Int) -> Bool
    true
word add(a: Int, b: Int) -> Int
    a + b
let empty: List<Int> = list.new()
print(list.length(list.map(empty, double(_))))
print(list.length(list.filter(empty, always(_))))
print(list.fold(empty, 5, add(_, _)))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "0\n0\n5\n" {
		t.Fatal(out.String())
	}
}

func TestListMapBuildsFreshOuterListWithSharedElements(t *testing.T) {
	source := `word same(values: List<Int>) -> List<Int>
    values
let inner = [1, 2]
let outer = [inner]
let mapped = list.map(outer, same(_))
print(list.length(mapped))
print(list.length(mapped[0]))
list.append(inner, 3)
print(list.length(mapped[0]))
list.append(mapped, [4])
print(list.length(outer))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n2\n3\n1\n" {
		t.Fatal(out.String())
	}
}

func TestListSourceUsableAfterTransformation(t *testing.T) {
	source := `word double(n: Int) -> Int
    n + n
let values = [1, 2]
let doubled = list.map(values, double(_))
print(list.length(values))
list.append(values, 5)
print(list.length(values))
print(doubled[1])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n3\n4\n" {
		t.Fatal(out.String())
	}
}

func TestListCallbackHoldsTraversalGuard(t *testing.T) {
	source := `word push-into(values: List<Int>, n: Int) -> Int
    list.append(values, n)
    n + n
let values = [1, 2]
let callback = push-into(values, _)
let doubled = list.map(values, callback)
`
	var out bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &out)
	if err == nil {
		t.Fatal("missing iteration diagnostic")
	}
	if d, ok := err.(*diagnostic); !ok || d.code != CodeCollectionIterating {
		t.Fatalf("expected collection-iterating, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("ran effects before failing: %q", out.String())
	}
}

func TestListCallbacksRunLeftToRight(t *testing.T) {
	source := `word shout(n: Int) -> Int
    print(n)
    n + n
let doubled = list.map([3, 1, 2], shout(_))
print(doubled[0])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n1\n2\n6\n" {
		t.Fatal(out.String())
	}
}

func TestListCallbackFailureKeepsEarlierEffects(t *testing.T) {
	source := `word fail-second(n: Int) -> Int
    print(n)
    n / (n - 1)
let doubled = list.map([1, 2], fail-second(_))
`
	var out bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &out)
	if d, ok := err.(*diagnostic); !ok || d.code != CodeDivisionZero {
		t.Fatalf("expected division by zero, got %v", err)
	}
	if out.String() != "1\n" {
		t.Fatal(out.String())
	}
}

func TestListWordsComposeInFlows(t *testing.T) {
	source := `word double(n: Int) -> Int
    n + n
word add(a: Int, b: Int) -> Int
    a + b
flow [1, 2, 3]
    list.map(double(_))
    list.fold(0, add(_, _))
    print
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "12\n" {
		t.Fatal(out.String())
	}
}

func TestListWordsEvaluateArgumentsOnce(t *testing.T) {
	source := `word add(a: Int, b: Int) -> Int
    a + b
word make-step()
    print("made")
    add _, _
print(list.fold([1, 2], 0, make-step()))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "made\n3\n" {
		t.Fatal(out.String())
	}
}

func TestListWordsInferOmittedSignatures(t *testing.T) {
	source := `word times(a: Int, b: Int) -> Int
    a * b
word add(a: Int, b: Int) -> Int
    a + b
word scale(values, factor)
    list.map(values, times(_, factor))
word total(values: List<Int>)
    list.fold(values, 0, add(_, _))
print(list.fold(scale([2, 3], 3), 0, add(_, _)))
print(total([4, 5]))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "15\n9\n" {
		t.Fatal(out.String())
	}
}

func TestListWordsRejectAmbiguousEmptyListSource(t *testing.T) {
	if got := runError(t, "word double(n: Int) -> Int\n    n + n\nword empty()\n    list.map([], double(_))\n"); !strings.Contains(got, "empty list requires an expected List type") {
		t.Fatalf("wrong diagnostic: %q", got)
	}
}

func TestListWordsAcceptModuleCallbacks(t *testing.T) {
	files := map[string]string{"math.zhex": "word double(n: Int) -> Int\n    n + n\n"}
	program, err := CompileModules("main.zhex", "use math\nlet d = math.double(_)\nprint(list.map([1, 2], math.double(_))[1])\nprint(list.map([3, 4], d)[0])\n", mapResolver(files))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := program.Run(strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "4\n6\n" {
		t.Fatal(out.String())
	}
}

func TestListMapRecursiveCallbackHitsDepthLimit(t *testing.T) {
	source := `word descend(n: Int) -> Int
    let deeper = list.map([n], descend(_))
    deeper[0]
print(descend(1))
`
	err := Run("test.zhex", source, strings.NewReader(""), &bytes.Buffer{})
	if d, ok := err.(*diagnostic); !ok || d.code != CodeDepthLimit {
		t.Fatalf("expected depth limit, got %v", err)
	}
}

func TestListWordsRejectWrongTypes(t *testing.T) {
	add := "word add(a: Int, b: Int) -> Int\n    a + b\n"
	cases := []struct {
		source  string
		message string
	}{
		{"let x = list.map([1], 2)\n", "argument 2 of list.map requires a transform from Int, found Int"},
		{"let x = list.map(5, 2)\n", "list.map requires a List"},
		{"let x = list.map([1])\n", "list.map requires 2 arguments, found 1"},
		{"let x = list.filter(5, 2)\n", "list.filter requires a List"},
		{"let x = list.filter([1], 2)\n", "argument 2 of list.filter requires type (Int) -> Bool, found Int"},
		{add + "let x = list.fold([1], 0)\n", "list.fold requires 3 arguments, found 2"},
		{"let x = list.fold(5, 0, 2)\n", "list.fold requires a List"},
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
	// A mismatched partial callback is rejected during inference, before the
	// whole-program checking pass renders its own argument diagnostic.
	if got := runError(t, "let x = list.map([1], text.trim(_))\n"); !strings.Contains(got, "incompatible inferred types") {
		t.Fatalf("wrong diagnostic: %q", got)
	}
	if got := runError(t, "word add(a: Int, b: Int) -> Int\n    a + b\nlet x = list.map([1], add(_, _))\n"); !strings.Contains(got, "incompatible inferred types") {
		t.Fatalf("wrong diagnostic: %q", got)
	}
	if got := runError(t, "word double(n: Int) -> Int\n    n + n\nlet x = list.filter([1], double(_))\n"); !strings.Contains(got, "incompatible inferred types") {
		t.Fatalf("wrong diagnostic: %q", got)
	}
	if got := runError(t, "let x = list.fold([1], 0, text.trim(_))\n"); !strings.Contains(got, "incompatible inferred types") {
		t.Fatalf("wrong diagnostic: %q", got)
	}
}

func TestListWordsRejectResultReturningCallbacks(t *testing.T) {
	parse := "word parse(raw: String) -> Result<Int, ParseError>\n    ok(text.to-int(raw) try)\n"
	cases := []struct {
		source  string
		message string
	}{
		{parse + "let x = list.map([\"1\"], parse(_))\n", "a Result cannot be returned from a callback"},
		{parse + "let x = list.filter([\"1\"], parse(_))\n", "incompatible inferred types"},
		{"word add(a: Int, b: Int) -> Int\n    a + b\nlet x = list.fold([1], text.to-int(\"7\"), add(_, _))\n", "a Result cannot be returned from a callback"},
	}
	for _, tc := range cases {
		if got := runError(t, tc.source); !strings.Contains(got, tc.message) {
			t.Errorf("%s: wrong diagnostic %q, want %q", tc.source, got, tc.message)
		}
	}
}
