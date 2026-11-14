package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestRecordDefaultsAreLazyAndFresh(t *testing.T) {
	source := `word make() -> Int
    print("default")
    3
type Config = record
    name: String
    retries: Int = make()
    tags: List<String> = []
let first = Config("a")
let second = Config("b", 7)
let third = Config("c", 8, ["own"])
list.append(first.tags, "first") try
print(list.length(second.tags))
print(first.retries)
print(third.retries)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "default\n0\n3\n8\n" {
		t.Fatal(out.String())
	}
}

func TestRecordDefaultsRejectInvalidFieldsAndEffects(t *testing.T) {
	for _, source := range []string{
		"type C = record\n    x: Int = 1\n    y: Int\n",
		"type C = record\n    x: Int = \"wrong\"\n",
		"type C = record\n    x: Int = text.to-int(\"1\") try\n",
		"let n = 2\ntype C = record\n    x: Int = n\n",
		"type C = record\n    x: Int = 1\nlet c = C(1, 2)\n",
	} {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted invalid default: %s", source)
		}
	}
}

func TestRecordDefaultsUseDefinitionModuleAndJSON(t *testing.T) {
	resolve := func(name string) (ModuleSource, bool) {
		return ModuleSource{Path: "model.zhex", Source: "word initial() -> Int\n    3\ntype Config = record\n    retries: Int = initial()\n    verbose: Bool = false\n"}, name == "model"
	}
	source := "use model\nword initial() -> Int\n    99\nlet config = json.decode(\"{\\\"verbose\\\":true}\", model.Config()) try\nprint(config.retries)\nprint(config.verbose)\n"
	var out bytes.Buffer
	if err := RunWithModules("main.zhex", source, strings.NewReader(""), &out, nil, nil, resolve); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\ntrue\n" {
		t.Fatal(out.String())
	}
}

func TestRecordDefaultsPersistInREPL(t *testing.T) {
	var out bytes.Buffer
	s := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := s.Enter("type Config = record\n    retries: Int = 3\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enter("let c = Config()\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enter("print(c.retries)\n"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n" {
		t.Fatal(out.String())
	}
}

func TestRecordDefaultArityDiagnosticIncludesOptionalRange(t *testing.T) {
	_, err := Compile("test.zhex", "type Config = record\n    name: String\n    retries: Int = 3\nlet config = Config()\n")
	if err == nil || !strings.Contains(err.Error(), "requires 1 to 2 arguments") {
		t.Fatalf("unexpected diagnostic: %v", err)
	}
}

func TestRecordDefaultEvaluationOrder(t *testing.T) {
	source := `word announce(label: String) -> Int
    print(label)
    1
type Config = record
    required: Int
    first: Int = announce("first")
    second: Int = announce("second")
let config = Config(announce("argument"))
print(config.second)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "argument\nfirst\nsecond\n1\n" {
		t.Fatal(out.String())
	}
}

func TestRecordDefaultCanCallHelperThatHandlesFailure(t *testing.T) {
	source := `word fallback() -> Int
    text.to-int("bad") catch
        3
type Config = record
    retries: Int = fallback()
let c = Config()
print(c.retries)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n" {
		t.Fatal(out.String())
	}
}
