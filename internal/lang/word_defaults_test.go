package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestWordDefaultsInferTypesAndEvaluateOnlyWhenOmitted(t *testing.T) {
	source := `word fallback()
    print("default")
    "world"
word greet(prefix: String = "hello", name = fallback())
    prefix + " " + name
print(greet("hi", "Ada"))
print(greet())
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "hi Ada\ndefault\nhello world\n" {
		t.Fatal(out.String())
	}
}

func TestWordDefaultsRequireTrailingAndCorrectArity(t *testing.T) {
	for _, source := range []string{
		"word bad(first: Int = 1, second: Int)\n    second\n",
		"word add(a: Int, b: Int = 1)\n    a + b\nadd()\n",
		"word add(a: Int, b: Int = 1)\n    a + b\nadd(1, 2, 3)\n",
		"word bad(a: Int = 1, b: Int = a)\n    b\n",
	} {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
}

func TestWordDefaultsFreshMutableValue(t *testing.T) {
	source := `word add(values: List<Int> = [])
    list.append(values, 1)
    list.length(values)
print(add())
print(add())
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n1\n" {
		t.Fatal(out.String())
	}
}

func TestWordDefaultsREPLSignatureAndRollback(t *testing.T) {
	var out bytes.Buffer
	s := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := s.Enter("word greet(name = \"Ada\")\n    name\n"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(s.Words(), "\n"), "greet(name: String = ...) -> String") {
		t.Fatal(s.Words())
	}
	if _, err := s.Enter("word greet(name: Int)\n    name\n"); err == nil {
		t.Fatal("accepted incompatible parameter type")
	}
	if echo, err := s.Enter("greet()"); err != nil || echo != "Ada" {
		t.Fatalf("echo %q error %v", echo, err)
	}
}

func TestWordDefaultRedefinitionKeepsExistingCallers(t *testing.T) {
	var out bytes.Buffer
	s := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := s.Enter("word greet(name: String = \"Ada\") -> String\n    name\nword caller() -> String\n    greet()\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enter("word greet(name: String) -> String\n    name\n"); err != nil {
		t.Fatal(err)
	}
	if echo, err := s.Enter("caller()"); err != nil || echo != "Ada" {
		t.Fatalf("echo %q error %v", echo, err)
	}
	if _, err := s.Enter("greet()"); err == nil {
		t.Fatal("new signature accepted missing argument")
	}
}

func TestWordDefaultsEvaluateInCallOrder(t *testing.T) {
	source := `word mark(n: Int) -> Int
    print(n)
    n
word sum(a: Int = mark(1), b: Int = mark(2)) -> Int
    a + b
print(sum(mark(3)))
flow 4
    sum
    print
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n2\n5\n2\n6\n" {
		t.Fatal(out.String())
	}
}

func TestWordDefaultsUseDeclaringModule(t *testing.T) {
	files := mapResolver(map[string]string{
		"math.zhex": "word fallback() -> Int\n    7\nword choose(n: Int = fallback()) -> Int\n    n\n",
	})
	source := "use math\nword fallback() -> Int\n    99\nprint(math.choose())\n"
	program, err := CompileModules("main.zhex", source, files)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := program.Run(strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}

func TestWordDefaultsAllowHandledFailureAndRejectEscapingFailure(t *testing.T) {
	source := `word fallback() -> Int
    text.to-int("bad") catch problem
        9
word choose(n: Int = fallback()) -> Result<Int, Error>
    ok(n)
print(choose() try)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "9\n" {
		t.Fatal(out.String())
	}
	if _, err := Compile("test.zhex", "word bad(n: Int = text.to-int(\"bad\") try) -> Int\n    n\n"); err == nil {
		t.Fatal("accepted escaping default failure")
	}
}

func TestRecursiveWordDefaultHitsDepthLimit(t *testing.T) {
	source := "word recur(n: Int = recur()) -> Int\n    n\nprint(recur())\n"
	err := Run("test.zhex", source, strings.NewReader(""), &bytes.Buffer{})
	if d, ok := err.(*diagnostic); !ok || d.code != CodeDepthLimit {
		t.Fatalf("expected depth limit, got %v", err)
	}
}

func TestWordDefaultsConstrainImportedParameters(t *testing.T) {
	files := mapResolver(map[string]string{
		"helper.zhex": "word identity(value)\n    value\n",
		"util.zhex":   "use helper\nword choose(value = helper.identity(7))\n    value\n",
	})
	program, err := CompileModules("main.zhex", "use util\nprint(util.choose())\n", files)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := program.Run(strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}
