package lang

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInferredWordReturnTypes(t *testing.T) {
	source := `word double(n: Int)
    n + n
word say(value: Int)
    print(value)
word forward(n: Int)
    backward(n)
word backward(n: Int)
    if n == 0
        1
    else
        forward(n - 1)
say(double(3))
print(forward(2))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "6\n1\n" {
		t.Fatal(out.String())
	}
}

func TestInferredReturnAmbiguityRequiresAnnotation(t *testing.T) {
	for _, source := range []string{
		"word loop(n: Int)\n    loop(n)\n",
		"word empty()\n    []\n",
		"word none()\n    Option.None\n",
	} {
		_, err := Compile("test.zhex", source)
		if err == nil || !strings.Contains(err.Error(), "annotation") {
			t.Errorf("missing ambiguity diagnostic for %q: %v", source, err)
		}
	}
}

func TestInferredReturnRetainsEscapingEffects(t *testing.T) {
	source := `word parse(raw: String)
    text.to-int(raw) try
let n = parse("bad") catch problem
    7
print(n)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}

func TestInferredReturnThroughForwardFallibleAndCatch(t *testing.T) {
	source := `word outer(raw: String)
    inner(raw) try
word inner(raw: String)
    text.to-int(raw) try
word recover(raw: String)
    outer(raw) catch problem
        8
print(recover("bad"))
print(outer("7") try)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "8\n7\n" {
		t.Fatal(out.String())
	}
}

func TestInferredReturnWithMapWrites(t *testing.T) {
	source := `word update(scores: Map<String, Int>)
    scores["a"] = 1
word update-parenthesized(scores: Map<String, Int>)
    (scores["b"] = 2)
let scores = {"a": 0}
update(scores)
update-parenthesized(scores)
print(map.length(scores))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n" {
		t.Fatal(out.String())
	}
}

func TestInferredReturnWithChoicesFlowsAndExplicitResults(t *testing.T) {
	source := `type Status = choice
    Ready(value: Int)
    Empty
word fallback(n: Int) -> Result<Int, Error>
    if n > 0
        ok(n)
    else
        err(error.new("empty", "empty"))
word select(status: Status)
    match status
        case Status.Ready(value)
            flow value
                int.to-string
        case Status.Empty
            "none"
word apply(n: Int)
    fallback(n) try
print(select(Status.Ready(7)))
print(apply(4) try)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n4\n" {
		t.Fatal(out.String())
	}
}

func TestInferredReturnREPLRedefinitionIsTransactional(t *testing.T) {
	var out bytes.Buffer
	s := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := s.Enter("word number()\n    4\nword caller()\n    number()\n"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(s.Words(), "\n"), "number() -> Int") {
		t.Fatal(s.Words())
	}
	if _, err := s.Enter("word number()\n    \"changed\"\nword helper()\n    1\n"); err == nil {
		t.Fatal("accepted incompatible redefinition")
	}
	if strings.Contains(strings.Join(s.Words(), "\n"), "helper(") {
		t.Fatal("failed entry adopted helper")
	}
	for _, expr := range []string{"number()", "caller()"} {
		if echo, err := s.Enter(expr); err != nil || echo != "4" {
			t.Fatalf("%s: echo %q, error %v", expr, echo, err)
		}
	}
}

func TestInferredReturnAcrossModules(t *testing.T) {
	var out bytes.Buffer
	s := NewSession(strings.NewReader(""), &out, nil, mapResolver(map[string]string{
		"math.zhex": "word twice(n: Int)\n    n + n\n",
	}))
	if _, err := s.Enter("use math\nword four()\n    math.twice(2)\n"); err != nil {
		t.Fatal(err)
	}
	if echo, err := s.Enter("four()"); err != nil || echo != "4" {
		t.Fatalf("echo %q, error %v", echo, err)
	}
}

func TestInferredReturnAlongsideExistingExamples(t *testing.T) {
	paths, err := filepath.Glob("../../examples/*.zhex")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			_, err := Compile(path, string(source)+"\nword inference-probe()\n    1\n")
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInferredReturnDoesNotExposePrivateModuleType(t *testing.T) {
	files := mapResolver(map[string]string{
		"data.zhex": "type _Secret = record\n    value: Int\nword expose()\n    _Secret(1)\n",
	})
	if _, err := CompileModules("main.zhex", "use data\n", files); err == nil {
		t.Fatal("accepted public word with a private inferred return type")
	} else if d, ok := err.(*diagnostic); !ok || d.code != CodePrivateUse {
		t.Fatalf("expected private-use diagnostic, got %v", err)
	}
}

func TestInferredReturnsPreserveUnknownNameDiagnostics(t *testing.T) {
	for _, expression := range []string{"unknown", "unknown()", "true + 1", "text.length(1)"} {
		source := "word broken() -> Int\n    " + expression + "\n"
		_, original := Compile("test.zhex", source)
		_, combined := Compile("test.zhex", source+"word inferred()\n    1\n")
		first, ok1 := original.(*diagnostic)
		second, ok2 := combined.(*diagnostic)
		if !ok1 || !ok2 || first.code != second.code || first.message != second.message || first.hint != second.hint || first.spanStart.line != second.spanStart.line || first.spanStart.column != second.spanStart.column || first.spanEnd.line != second.spanEnd.line || first.spanEnd.column != second.spanEnd.column {
			t.Fatalf("%s: original %v, combined %v", expression, original, combined)
		}
	}
}

func TestInferredReturnsPreserveConditionDiagnostics(t *testing.T) {
	source := "word broken() -> Int\n    if 1\n        1\n    else\n        2\n"
	_, original := Compile("test.zhex", source)
	_, combined := Compile("test.zhex", source+"word inferred()\n    1\n")
	first, ok1 := original.(*diagnostic)
	second, ok2 := combined.(*diagnostic)
	if !ok1 || !ok2 || first.code != second.code || first.message != second.message || first.hint != second.hint || first.spanStart.line != second.spanStart.line || first.spanStart.column != second.spanStart.column || first.spanEnd.line != second.spanEnd.line || first.spanEnd.column != second.spanEnd.column {
		t.Fatalf("original %v, combined %v", original, combined)
	}
}
