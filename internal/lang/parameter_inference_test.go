package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestInferredParametersFromBodyAndEntryCalls(t *testing.T) {
	source := `word length(value)
    text.length(value)
word identity(value)
    value
print(length("hello"))
print(identity(7))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "5\n7\n" {
		t.Fatal(out.String())
	}
}

func TestInferredParameterConflictsBeforeEffects(t *testing.T) {
	source := `word identity(value)
    value
print("not run")
print(identity(1))
print(identity("two"))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err == nil {
		t.Fatal("accepted conflicting argument types")
	}
	if out.Len() != 0 {
		t.Fatalf("ran partial program: %q", out.String())
	}
}

func TestInferredParametersAcrossModules(t *testing.T) {
	files := mapResolver(map[string]string{"math.zhex": "word identity(value)\n    value\n"})
	if _, err := CompileModules("main.zhex", "use math\nprint(math.identity(7))\n", files); err != nil {
		t.Fatal(err)
	}
}

func TestInferredParameterREPLRedefinitionRollback(t *testing.T) {
	var out bytes.Buffer
	s := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := s.Enter("word length(value)\n    text.length(value)\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enter("word length(value)\n    value + 1\n"); err == nil {
		t.Fatal("accepted incompatible redefinition")
	}
	if echo, err := s.Enter("length(\"abc\")"); err != nil || echo != "3" {
		t.Fatalf("echo %q error %v", echo, err)
	}
}

func TestInferredParameterInImportedREPLModule(t *testing.T) {
	var out bytes.Buffer
	s := NewSession(strings.NewReader(""), &out, nil, mapResolver(map[string]string{"math.zhex": "word identity(value)\n    value\n"}))
	if echo, err := s.Enter("use math\nmath.identity(7)\n"); err != nil || echo != "7" {
		t.Fatalf("echo %q error %v", echo, err)
	}
	if echo, err := s.Enter("math.identity(8)"); err != nil || echo != "8" {
		t.Fatalf("echo %q error %v", echo, err)
	}
}

func TestInferredParameterRecursionAndMixedAnnotations(t *testing.T) {
	source := `word count-down(n, prefix: String)
    if n == 0
        prefix
    else
        count-down(n - 1, prefix)
print(count-down(2, "done"))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "done\n" {
		t.Fatal(out.String())
	}
}

func TestUnconstrainedParametersRequireAnnotation(t *testing.T) {
	for _, source := range []string{
		"word identity(value)\n    value\n",
		"word combine(a, b)\n    a + b\n",
	} {
		if _, err := Compile("test.zhex", source); err == nil || !strings.Contains(err.Error(), "annotation") {
			t.Errorf("%q: %v", source, err)
		}
	}
}

func TestInferredParametersAcrossModuleCallGraph(t *testing.T) {
	files := mapResolver(map[string]string{
		"first.zhex":  "use second\nword forward(value)\n    second.identity(value)\n",
		"second.zhex": "word identity(value)\n    value\n",
	})
	if _, err := CompileModules("main.zhex", "use first\nprint(first.forward(7))\n", files); err != nil {
		t.Fatal(err)
	}
}

func TestInferredParametersRejectStoredResults(t *testing.T) {
	source := "word identity(value)\n    value\nidentity(text.to-int(\"bad\"))\n"
	if _, err := Compile("test.zhex", source); err == nil {
		t.Fatal("accepted a Result parameter")
	}
}

func TestInferredParametersWithImportedRecordAndAliases(t *testing.T) {
	files := mapResolver(map[string]string{
		"model.zhex": "type Age = alias Int\ntype Person = record\n    age: Age = 3\nword make(value)\n    Person(age: value)\n",
	})
	program, err := CompileModules("main.zhex", "use model\nlet person = model.make(7)\nprint(person.age)\n", files)
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

func TestInferredParametersAcrossFallibleModules(t *testing.T) {
	files := mapResolver(map[string]string{
		"first.zhex":  "use second\nword parse(value)\n    second.parse(value) try\n",
		"second.zhex": "word parse(value)\n    text.to-int(value) try\n",
	})
	program, err := CompileModules("main.zhex", "use first\nprint(first.parse(\"7\") try)\n", files)
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

func TestInferredParametersWithReporterRegistration(t *testing.T) {
	source := `word report(problem)
    print(error.code(problem))
error.set-reporter(report)
let n = text.to-int("bad") catch problem
    error.report(problem)
    7
print(n)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "text-to-int\n7\n" {
		t.Fatal(out.String())
	}
}
