package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestInferredFallibleWordAndPureWord(t *testing.T) {
	source := "word parse(raw: String) -> Int\n    text.to-int(raw) try\nword double(n: Int) -> Int\n    n * 2\nprint(double(parse(\"21\") try))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "42\n" {
		t.Fatal(out.String())
	}
	if _, err := Compile("test.zhex", strings.Replace(source, "parse(\"21\") try", "parse(\"21\")", 1)); err == nil {
		t.Fatal("accepted unhandled inferred Result")
	}
}

func TestInferredEffectsThroughForwardRecursionAndFlow(t *testing.T) {
	source := "word first(raw: String) -> Int\n    flow raw\n        second try\n        double\nword second(raw: String) -> Int\n    text.to-int(raw) try\nword double(n: Int) -> Int\n    n * 2\nprint(first(\"5\") try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "10\n" {
		t.Fatal(out.String())
	}
}

func TestInferredMixedFamiliesMustBeHandled(t *testing.T) {
	source := "word read() -> Int\n    let line = read-line(\"N: \") try\n    text.to-int(line) try\n"
	if _, err := Compile("test.zhex", source); err == nil || !strings.Contains(err.Error(), "multiple error families") {
		t.Fatalf("wanted mixed family rejection, got %v", err)
	}
}

func TestCatchRemovesOnlyHandledFailure(t *testing.T) {
	source := "word number(raw: String) -> Int\n    let n = text.to-int(raw) catch problem\n        3\n    n\nprint(number(\"bad\"))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n" {
		t.Fatal(out.String())
	}
	source = "word number() -> Int\n    let n = text.to-int(read-line(\"N: \") try) catch problem\n        3\n    n\nprint(number() try)\n"
	out.Reset()
	if err := Run("test.zhex", source, strings.NewReader("bad\n"), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "N: 3\n" {
		t.Fatal(out.String())
	}
}

func TestInferredFailureReturnsOnceAndKeepsOrigin(t *testing.T) {
	source := "word inner(raw: String) -> Int\n    text.to-int(raw) try\nword outer(raw: String) -> Int\n    inner(raw) try\nouter(\"bad\") try\n"
	var out, trace bytes.Buffer
	err := RunWithTrace("test.zhex", source, strings.NewReader(""), &out, &trace)
	if err == nil || !strings.Contains(err.Error(), "not a signed 64-bit decimal integer") {
		t.Fatalf("error %v", err)
	}
	if !strings.Contains(trace.String(), `"word":"outer","value":"err(`) {
		t.Fatal(trace.String())
	}
}

func TestInferredEarlyWrappedError(t *testing.T) {
	source := "word positive(n: Int) -> Int\n    if n < 1\n        return err(error.new(\"bad\", \"positive only\"))\n    n\nprint(positive(3) try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n" {
		t.Fatal(out.String())
	}
}

func TestInferredRecursiveAndImportedEffects(t *testing.T) {
	source := "word a(n: Int) -> Int\n    if n == 0\n        text.to-int(\"7\") try\n    else\n        b(n - 1) try\nword b(n: Int) -> Int\n    a(n) try\nprint(b(2) try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
	resolve := func(name string) (ModuleSource, bool) {
		if name == "model" {
			return ModuleSource{Path: "model.zhex", Source: "word parse(s: String) -> Int\n    text.to-int(s) try\n"}, true
		}
		return ModuleSource{}, false
	}
	out.Reset()
	if err := RunWithModules("main.zhex", "use model\nword read() -> Int\n    model.parse(\"9\") try\nprint(read() try)\n", strings.NewReader(""), &out, nil, nil, resolve); err != nil {
		t.Fatal(err)
	}
	if out.String() != "9\n" {
		t.Fatal(out.String())
	}
}

func TestREPLRejectsChangedInferredContractTransactionally(t *testing.T) {
	var out bytes.Buffer
	s := NewSession(strings.NewReader(""), &out, nil, nil)
	for _, entry := range []string{"word parse(s: String) -> Int\n    7\n", "word caller() -> Int\n    parse(\"bad\")\n"} {
		if _, err := s.Enter(entry); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Enter("word parse(s: String) -> Int\n    text.to-int(s) try\n"); err == nil {
		t.Fatal("accepted changed caller contract")
	}
	if echo, err := s.Enter("parse(\"bad\")\n"); err != nil || echo != "7" {
		t.Fatalf("direct echo %q error %v", echo, err)
	}
	if echo, err := s.Enter("caller()\n"); err != nil || echo != "7" {
		t.Fatalf("caller echo %q error %v", echo, err)
	}
}

func TestInferredConversionAndStoredResult(t *testing.T) {
	source := "word parse(raw: String) -> Int\n    let pending = text.to-int(raw)\n    let n = pending catch problem\n        return err(error.wrap(\"bad-number\", \"invalid number\", problem))\n    n\nlet n = parse(\"bad\") catch problem\n    print(error.code(problem))\n    3\nprint(n)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "bad-number\n3\n" {
		t.Fatal(out.String())
	}
}

func TestInferredHandlerMayPropagateDifferentFamily(t *testing.T) {
	source := "word parse(raw: String) -> String\n    let n = text.to-int(raw) catch problem\n        let replacement = read-line(\"Replacement: \") try\n        text.length(replacement)\n    int.to-string(n)\nprint(parse(\"bad\") try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader("abc\n"), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "Replacement: 3\n" {
		t.Fatal(out.String())
	}
}

func TestInferredChoiceSuccessAndExplicitResultCompatibility(t *testing.T) {
	source := "type Status = choice\n    Ready(value: Int)\nword parse(raw: String) -> Status\n    Status.Ready(text.to-int(raw) try)\nword explicit(raw: String) -> Result<Int, ParseError>\n    ok(text.to-int(raw) try)\nmatch parse(\"4\") try\n    case Status.Ready(value)\n        print(value)\nprint(explicit(\"5\") try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "4\n5\n" {
		t.Fatal(out.String())
	}
}

func TestInferredWordCanFinishWithFallibleFlow(t *testing.T) {
	source := "word parse(raw: String) -> Int\n    flow raw\n        text.to-int try\nprint(parse(\"8\") try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "8\n" {
		t.Fatal(out.String())
	}
}

func TestInferredRethrowKeepsConcreteFamily(t *testing.T) {
	source := "word inner(raw: String) -> Int\n    text.to-int(raw) try\nword outer(raw: String) -> Int\n    let n = inner(raw) catch problem\n        let alias = problem\n        return err(alias)\n    n\nlet value = outer(\"bad\") catch problem\n    print(error.code(problem))\n    3\nprint(value)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "text-to-int\n3\n" {
		t.Fatal(out.String())
	}
}

func TestREPLShowsInferredSignatureAndRejectsFailedEntry(t *testing.T) {
	var out bytes.Buffer
	s := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := s.Enter("word parse(s: String) -> Int\n    text.to-int(s) try\n"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(s.Words(), "\n"), "parse(s: String) -> Result<Int, ParseError>") {
		t.Fatalf("words %v", s.Words())
	}
	if _, err := s.Enter("word parse(s: String) -> Int\n    5\nword helper() -> Int\n    1\n"); err == nil {
		t.Fatal("accepted incompatible redefinition")
	}
	if strings.Contains(strings.Join(s.Words(), "\n"), "helper(") {
		t.Fatal("failed entry adopted helper")
	}
	if echo, err := s.Enter("parse(\"9\") try\n"); err != nil || echo != "9" {
		t.Fatalf("echo %q error %v", echo, err)
	}
}

func TestRuntimeFaultInsideCatchIsNotAnInferredResult(t *testing.T) {
	source := "word parse() -> Int\n    let n = text.to-int(\"bad\") catch problem\n        1 / 0\n    n\nprint(parse())\n"
	var out bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &out)
	if d, ok := err.(*diagnostic); !ok || d.code != CodeDivisionZero {
		t.Fatalf("wanted division-zero runtime fault, got %v", err)
	}
}

func TestInferredWordReturnsGenericChoice(t *testing.T) {
	source := "type Maybe<T> = choice\n    None\n    Some(value: T)\nword parse(raw: String) -> Maybe<Int>\n    Maybe.Some(text.to-int(raw) try)\nmatch parse(\"12\") try\n    case Maybe.Some(value)\n        print(value)\n    case Maybe.None\n        print(0)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "12\n" {
		t.Fatal(out.String())
	}
}
