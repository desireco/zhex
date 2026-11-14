package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestBuiltinOptionChoice(t *testing.T) {
	source := "let present = Option.Some(7)\nlet absent: Option<Int> = Option.None\nmatch present\n    case Option.Some(value)\n        print(value)\n    case Option.None\n        print(0)\nmatch absent\n    case Option.Some(value)\n        print(value)\n    case Option.None\n        print(0)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n0\n" {
		t.Fatal(out.String())
	}
	if _, err := Compile("test.zhex", "type Option = record\n    value: Int\n"); err == nil {
		t.Fatal("accepted Option redeclaration")
	}
}

func TestBuiltinOptionConstructorCollisions(t *testing.T) {
	if _, err := Compile("test.zhex", "word Option.Some(n: Int) -> Int\n    n\n"); err == nil {
		t.Fatal("accepted word colliding with Option.Some")
	}
	resolve := func(name string) (ModuleSource, bool) {
		if name == "Option" {
			return ModuleSource{Path: "Option.zhex", Source: "word Some(n: Int) -> Int\n    n\n"}, true
		}
		return ModuleSource{}, false
	}
	if _, err := CompileModules("main.zhex", "use Option\n", resolve); err == nil {
		t.Fatal("accepted imported word colliding with Option.Some")
	}
}

func TestMapLiteralLookupAndPlainAssignment(t *testing.T) {
	source := "let scores = {\"Ada\": 10}\nscores[\"Grace\"] = 11\nmatch scores[\"Grace\"]\n    case Option.Some(score)\n        print(score)\n    case Option.None\n        print(0)\nprint(map.length(scores))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "11\n2\n" {
		t.Fatal(out.String())
	}
}

func TestMapLiteralRejectsBadKeysAndDuplicateConstants(t *testing.T) {
	for _, src := range []string{"let x = {0: 1, -0: 2}\n", "let x = {\"a\": 1, \"\\x61\": 2}\n", "let x = {1: 1, true: 2}\n", "let x = {}\n", "let n = 1\nlet x = {n: 2}\n"} {
		if _, err := Compile("test.zhex", src); err == nil {
			t.Errorf("accepted %q", src)
		}
	}
}

func TestMapIterationOrderRemoveAndMerge(t *testing.T) {
	source := "let scores = {\"Ada\": 1, \"Bob\": 2}\nscores[\"Ada\"] = 4\nmap.remove(scores, \"Bob\")\nscores[\"Bob\"] = 5\nlet incoming = {\"Ada\": 8, \"Cy\": 9}\nflow incoming\n    map.into(scores)\n    map.length\n    print\nfor name, score in scores\n    print(name + \": \" + int.to-string(score))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\nAda: 8\nBob: 5\nCy: 9\n" {
		t.Fatal(out.String())
	}
}

func TestMapIterationRejectsAliasedMutationAndUnwinds(t *testing.T) {
	var out bytes.Buffer
	s := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := s.Enter("let scores = {\"Ada\": 1}\nlet alias = scores\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enter("for name, score in scores\n    alias[\"Ada\"] = 9\n"); err == nil {
		t.Fatal("accepted mutation during iteration")
	} else if d, ok := err.(*diagnostic); !ok || d.code != CodeCollectionIterating {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := s.Enter("alias[\"Bob\"] = 2\n"); err != nil {
		t.Fatalf("guard was not released: %v", err)
	}
	if echo, err := s.Enter("map.length(scores)\n"); err != nil || echo != "2" {
		t.Fatalf("echo %q error %v", echo, err)
	}
}

func TestMapKeyTypesAndContextualEmptyMaps(t *testing.T) {
	source := "let flags = {true: \"yes\", false: \"no\"}\nlet numbers = {-9223372036854775808: 7, 0: 8}\nlet empty: Map<String, Int> = {}\nmap.put(empty, \"n\", 3)\nprint(map.contains(flags, false))\nmatch numbers[-9223372036854775808]\n    case Option.Some(value)\n        print(value)\n    case Option.None\n        print(0)\nprint(map.length(empty))\n"
	// map.put returns the same map; bind it to avoid an unused pass-through.
	source = strings.Replace(source, "map.put(empty, \"n\", 3)\n", "let updated = map.put(empty, \"n\", 3)\n", 1)
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "true\n7\n1\n" {
		t.Fatal(out.String())
	}
}

func TestMapFlowAndContextualOption(t *testing.T) {
	source := "let target: Map<String, Int> = {}\nflow {\"Nancy\": 23}\n    map.into(target)\n    map.put(\"Ada\", 11)\n    map.length\n    print\nlet settings: Map<String, Option<Int>> = {}\nlet updated = map.put(settings, \"x\", Option.None)\nprint(map.length(updated))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n1\n" {
		t.Fatal(out.String())
	}
}

func TestMapOperationsArePureUnlessExplicitlyFallible(t *testing.T) {
	source := "word size() -> Int\n    let scores = {\"Ada\": 1}\n    scores[\"Ada\"] = 2\n    map.remove(scores, \"missing\")\n    map.length(scores)\nprint(size())\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n" {
		t.Fatal(out.String())
	}
	if _, err := Compile("test.zhex", "let x = {\"a\": 1}\nx[\"a\"] try\n"); err == nil {
		t.Fatal("accepted try on Option lookup")
	}
}

func TestMapIntoInfersEitherEmptyOperandFromContext(t *testing.T) {
	source := "let scores = {\"Ada\": 1}\nlet first = map.into({}, scores)\nlet second = map.into({\"Grace\": 2}, {})\nprint(map.length(first))\nprint(map.length(second))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n1\n" {
		t.Fatal(out.String())
	}
}

func TestMapCannotStoreConcreteErrorsOrResults(t *testing.T) {
	for _, src := range []string{"let n = text.to-int(\"bad\") catch problem\n    let x = {\"problem\": problem}\n    0\n", "let x = {\"n\": text.to-int(\"bad\")}\n", "let values: Map<String, Error> = {}\nlet n = text.to-int(\"bad\") catch problem\n    values[\"e\"] = problem\n    0\n"} {
		if _, err := Compile("test.zhex", src); err == nil {
			t.Errorf("accepted unsafe map:\n%s", src)
		}
	}
}

func TestMapMutationGuardCoversNoOpsAndAliases(t *testing.T) {
	for _, stmt := range []string{"map.remove(alias, \"missing\")", "alias[\"Ada\"] = 9", "map.put(alias, \"Ada\", 9)", "map.into(alias, alias)"} {
		t.Run(stmt, func(t *testing.T) {
			var out bytes.Buffer
			s := NewSession(strings.NewReader(""), &out, nil, nil)
			if _, err := s.Enter("let scores = {\"Ada\": 1}\nlet alias = scores\n"); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Enter("for item in scores\n    " + stmt + "\n"); err == nil {
				t.Fatal("mutation accepted while iterating")
			} else if d, ok := err.(*diagnostic); !ok || d.code != CodeCollectionIterating {
				t.Fatalf("wrong error %v", err)
			}
			if _, err := s.Enter("alias[\"Bob\"] = 2\n"); err != nil {
				t.Fatalf("guard remained active: %v", err)
			}
			if echo, err := s.Enter("map.length(scores)\n"); err != nil || echo != "2" {
				t.Fatalf("echo %q error %v", echo, err)
			}
		})
	}
}

func TestMapMergeAllowsIteratedSourceButGuardsTarget(t *testing.T) {
	source := "let source = {\"a\": 1, \"b\": 2}\nlet target: Map<String, Int> = {}\nfor value in source\n    map.into(source, target)\nprint(map.length(target))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n" {
		t.Fatal(out.String())
	}
}

func TestMapGuardUnwindsOnFaultAndDeliberateExit(t *testing.T) {
	for _, stmt := range []string{"print(1 / 0)", "console.exit(3)", "console.fail(\"done\")"} {
		t.Run(stmt, func(t *testing.T) {
			var out bytes.Buffer
			s := NewSession(strings.NewReader(""), &out, nil, nil)
			if _, err := s.Enter("let scores = {\"Ada\": 1}\n"); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Enter("for value in scores\n    " + stmt + "\n"); err == nil {
				t.Fatal("expected unwinding")
			}
			if _, err := s.Enter("scores[\"Bob\"] = 2\n"); err != nil {
				t.Fatalf("guard remained active: %v", err)
			}
		})
	}
}

func TestNestedMapIterationKeepsOuterGuard(t *testing.T) {
	var out bytes.Buffer
	s := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := s.Enter("let scores = {\"Ada\": 1}\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enter("for outer in scores\n    for inner in scores\n        print(inner)\n    map.remove(scores, \"missing\")\n"); err == nil {
		t.Fatal("outer guard was released by inner loop")
	}
}

func TestInferredFallibleWordCanReturnMap(t *testing.T) {
	source := "word parse(raw: String) -> Map<String, Int>\n    {\"value\": text.to-int(raw) try}\nlet values = parse(\"4\") try\nmatch values[\"value\"]\n    case Option.Some(value)\n        print(value)\n    case Option.None\n        print(0)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "4\n" {
		t.Fatal(out.String())
	}
}

func TestMapGuardUnwindsOnReturnAndPropagation(t *testing.T) {
	source := "word first(scores: Map<String, Int>) -> Result<Int, Error>\n    for value in scores\n        return ok(value)\n    ok(0)\nword failing(scores: Map<String, Int>) -> Int\n    for value in scores\n        text.to-int(\"bad\") try\n    0\nlet scores = {\"Ada\": 1}\nprint(first(scores) try)\nlet n = failing(scores) catch problem\n    3\nscores[\"Bob\"] = n\nprint(map.length(scores))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n2\n" {
		t.Fatal(out.String())
	}
}

func TestMapAndOptionTypeNamesAreReserved(t *testing.T) {
	for _, src := range []string{"type Map = record\n    value: Int\n", "type Option = choice\n    None\n", "type Maybe<Map> = choice\n    Some(value: Map)\n"} {
		if _, err := Compile("test.zhex", src); err == nil {
			t.Errorf("accepted reserved type name:\n%s", src)
		}
	}
}
