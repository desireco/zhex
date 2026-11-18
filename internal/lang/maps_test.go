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

func TestMapAssignmentEvaluatesKeyAndValueOnceInOrder(t *testing.T) {
	source := "word key() -> String\n    print(\"key\")\n    \"Ada\"\nword score() -> Int\n    print(\"value\")\n    11\nlet scores: Map<String, Int> = {}\nscores[key()] = score()\nmatch scores[\"Ada\"]\n    case Option.Some(value)\n        print(value)\n    case Option.None\n        print(0)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "key\nvalue\n11\n" {
		t.Fatal(out.String())
	}
}

// TestMapKeysValues pins the extraction pair: fresh lists in documented
// insertion order — the same order `for k, v in map` traverses — so a map's
// data can flow into the composition vocabulary without a hand loop.
func TestMapKeysValues(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"keys in insertion order", "let scores = {\"Ada\": 1, \"Bob\": 2}\nprint(text.join(map.keys(scores), \",\"))\n", "Ada,Bob\n"},
		{"values in insertion order", "let scores = {\"Ada\": 1, \"Bob\": 2}\nlet texts = list.map(map.values(scores), int.to-string(_))\nprint(text.join(texts, \"|\"))\n", "1|2\n"},
		{"overwrite keeps the entry's position, remove and re-add move it last", "let scores = {\"Ada\": 1, \"Bob\": 2}\nscores[\"Ada\"] = 4\nmap.remove(scores, \"Bob\")\nscores[\"Bob\"] = 5\nprint(text.join(map.keys(scores), \",\"))\nlet texts = list.map(map.values(scores), int.to-string(_))\nprint(text.join(texts, \",\"))\n", "Ada,Bob\n4,5\n"},
		{"empty map extracts empty lists", "let empty: Map<String, Int> = {}\nprint(list.length(map.keys(empty)))\nprint(list.length(map.values(empty)))\n", "0\n0\n"},
		{"Int keys", "let numbers = {7: \"seven\", 1: \"one\"}\nlet keys = list.map(map.keys(numbers), int.to-string(_))\nprint(text.join(keys, \",\"))\n", "7,1\n"},
		{"Bool keys", "let flags = {true: \"yes\", false: \"no\"}\nprint(text.join(map.values(flags), \",\"))\n", "yes,no\n"},
		{"extracted lists are fresh", "let scores = {\"Ada\": 1}\nlet first = map.keys(scores)\nlist.append(first, \"Zoe\")\nprint(list.length(first))\nprint(list.length(map.keys(scores)))\n", "2\n1\n"},
		{"values compose into sort without a loop", "let members = {\"a\": [1, 2], \"b\": [3]}\nlet sizes = list.map(map.values(members), list.length(_))\nlet texts = list.map(list.sort(sizes), int.to-string(_))\nprint(text.join(texts, \",\"))\n", "1,2\n"},
		{"keys match dual-binding traversal order", "let scores = {\"Ada\": 1, \"Bob\": 2, \"Cy\": 3}\nlet keys = map.keys(scores)\nlet seen: List<String> = []\nfor name, score in scores\n    list.append(seen, name)\nprint(list.length(keys) == list.length(seen))\nprint(keys[2] == seen[2])\n", "true\ntrue\n"},
		{"extraction reads during active traversal", "let scores = {\"Ada\": 1}\nfor name, score in scores\n    print(list.length(map.keys(scores)))\n", "1\n"},
		{"flow step", "let scores = {\"Ada\": 1, \"Bob\": 2}\nflow scores\n    map.values\n    list.length\n    print\n", "2\n"},
		{"annotated partial", "let valuesof: (Map<String, Int>) -> List<Int> = map.values(_)\nlet scores = {\"Ada\": 1, \"Bob\": 2}\nprint(list.length(valuesof(scores)))\n", "2\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Run("keys.zhex", tc.source, strings.NewReader(""), &out); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("got %q, want %q", out.String(), tc.want)
			}
		})
	}
}

// TestMapKeysValuesElementLimit pins the hard stop before materializing: a
// map longer than the runtime element limit fails with the shared
// collection-limit code, the text.split/file.list precedent.
func TestMapKeysValuesElementLimit(t *testing.T) {
	for name, word := range map[string]string{"keys": "map.keys", "values": "map.values"} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			source := "var big: Map<Int, Int> = {}\nfor i in range(0, 1000001)\n    big = map.put(big, i, i)\nprint(list.length(" + word + "(big)))\n"
			err := Run("big.zhex", source, strings.NewReader(""), &out)
			if err == nil || !strings.Contains(err.Error(), word+" exceeds element limit") {
				t.Fatalf("wanted element limit diagnostic, got %v", err)
			}
			report, ok := Report(err)
			if !ok || !strings.Contains(report, `"code":"collection-limit"`) {
				t.Fatalf("wanted code collection-limit, got %s", report)
			}
			if out.Len() != 0 {
				t.Fatalf("limit must stop the run before output, got %q", out.String())
			}
		})
	}
}

// TestMapKeysValuesTyping pins the argument diagnostics: one Map argument,
// with the key and value types flowing into the extracted lists.
func TestMapKeysValuesTyping(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"keys wants a Map", "print(map.keys(\"Ada\"))\n", "map.keys requires a Map"},
		{"values wants a Map", "print(map.values(1))\n", "map.values requires a Map"},
		{"keys argument count", "let scores = {\"Ada\": 1}\nprint(map.keys(scores, \"x\"))\n", "map.keys requires 1 arguments"},
		{"Int keys do not join as text", "let numbers = {1: 2}\nprint(text.join(map.keys(numbers), \",\"))\n", "argument 1 of text.join requires type List<String>, found List<Int>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := Run("typing.zhex", tc.source, strings.NewReader(""), &out)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted %q, got %v", tc.want, err)
			}
		})
	}
}
