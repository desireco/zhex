package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestSetBuildDedupeRemoveContainsLength(t *testing.T) {
	source := "let s: Set<Int> = set.new\nset.add(s, 3)\nset.add(s, 1)\nset.add(s, 3)\nset.add(s, 2)\nprint(set.length(s))\nprint(set.contains(s, 3))\nset.remove(s, 3)\nprint(set.contains(s, 3))\nprint(set.length(s))\nset.remove(s, 9)\nprint(set.length(s))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\ntrue\nfalse\n2\n2\n" {
		t.Fatal(out.String())
	}
}

func TestSetElementDomainsAndInsertionOrder(t *testing.T) {
	source := "let names: Set<String> = set.new\nset.add(names, \"Ada\")\nset.add(names, \"Grace\")\nset.add(names, \"Ada\")\nprint(set.length(names))\nlet flags: Set<Bool> = set.new\nset.add(flags, true)\nset.add(flags, false)\nset.add(flags, true)\nprint(set.length(flags))\nprint(set.contains(flags, false))\nlet ordered: Set<Int> = set.new\nset.add(ordered, 3)\nset.add(ordered, 1)\nset.add(ordered, 2)\nset.add(ordered, 1)\nlet xs = set.values(ordered)\nprint(list.length(xs))\nprint(xs[0])\nprint(xs[1])\nprint(xs[2])\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n2\ntrue\n3\n3\n1\n2\n" {
		t.Fatal(out.String())
	}
}

func TestSetValuesIsAFreshList(t *testing.T) {
	source := "let s: Set<Int> = set.new\nset.add(s, 1)\nset.add(s, 2)\nlet xs = set.values(s)\nset.add(s, 3)\nprint(list.length(xs))\nprint(set.length(s))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n3\n" {
		t.Fatal(out.String())
	}
}

func TestSetPrintBracesAndEcho(t *testing.T) {
	source := "let s: Set<Int> = set.new\nset.add(s, 2)\nset.add(s, 1)\nset.add(s, 2)\nprint(s)\nlet names: Set<String> = set.new\nset.add(names, \"Ada\")\nset.add(names, \"Bo\")\nprint(names)\nlet flags: Set<Bool> = set.new\nset.add(flags, true)\nset.add(flags, false)\nprint(flags)\nlet empty: Set<Int> = set.new\nprint(empty)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{2, 1}\n{\"Ada\", \"Bo\"}\n{true, false}\n{}\n" {
		t.Fatal(out.String())
	}
	session := NewSession(strings.NewReader(""), &bytes.Buffer{}, nil, nil)
	if _, err := session.Enter("let s: Set<Int> = set.new\nset.add(s, 1)\nset.add(s, 2)\n"); err != nil {
		t.Fatal(err)
	}
	echo, err := session.Enter("s\n")
	if err != nil {
		t.Fatal(err)
	}
	if echo != "set(2 members)" {
		t.Fatal(echo)
	}
}

func TestSetJSONEncodeArrayDecodeRejected(t *testing.T) {
	source := "let s: Set<Int> = set.new\nset.add(s, 2)\nset.add(s, 1)\nset.add(s, 2)\nlet js = json.encode(s) try\nprint js\nlet names: Set<String> = set.new\nset.add(names, \"Ada\")\nlet js2 = json.encode(names) try\nprint js2\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "[2,1]\n[\"Ada\"]\n" {
		t.Fatal(out.String())
	}
	if _, err := Compile("test.zhex", "let s: Set<Int> = set.new\nlet back = json.decode(\"[1]\", s) try\n"); err == nil {
		t.Fatal("accepted json.decode into a Set target")
	} else if !strings.Contains(err.Error(), "json.decode does not support type Set<Int>") {
		t.Fatal(err.Error())
	}
}

func TestSetFlowStepsAndAmbiguousEmpty(t *testing.T) {
	source := "let s: Set<Int> = set.new\nlet seen = flow s\n    set.add(7)\n    set.contains(7)\nprint(seen)\nprint(set.length(s))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "true\n1\n" {
		t.Fatal(out.String())
	}
	if _, err := Compile("test.zhex", "let s = set.new\n"); err == nil {
		t.Fatal("accepted an ambiguous empty set")
	} else if !strings.Contains(err.Error(), "set.new requires an expected Set type") {
		t.Fatal(err.Error())
	}
}

func TestSetRejectsDisallowedElementsCompareAndTry(t *testing.T) {
	if _, err := Compile("test.zhex", "let s: Set<List<Int>> = set.new\n"); err == nil {
		t.Fatal("accepted Set<List<Int>>")
	} else if !strings.Contains(err.Error(), "Set elements must be Int, String, or Bool, found Set<List<Int>>") {
		t.Fatal(err.Error())
	}
	if _, err := Compile("test.zhex", "let s: Set<Set<Int>> = set.new\n"); err == nil {
		t.Fatal("accepted Set<Set<Int>>")
	} else if !strings.Contains(err.Error(), "Set elements must be Int, String, or Bool, found Set<Set<Int>>") {
		t.Fatal(err.Error())
	}
	if _, err := Compile("test.zhex", "let a: Set<Int> = set.new\nlet b: Set<Int> = set.new\nprint(a == b)\n"); err == nil {
		t.Fatal("accepted set equality")
	} else if !strings.Contains(err.Error(), "cannot compare type Set<Int>") {
		t.Fatal(err.Error())
	}
	if _, err := Compile("test.zhex", "let s: Set<Int> = set.new\nlet v = set.add(s, 1) try\n"); err == nil {
		t.Fatal("accepted try on a set word")
	} else if !strings.Contains(err.Error(), "try requires a Result, found Set<Int>") {
		t.Fatal(err.Error())
	}
}

func TestSetMutationReturnsReceiverAndAliasesShare(t *testing.T) {
	source := "let s: Set<Int> = set.new\nlet alias = s\nset.add(s, 5)\nprint(set.contains(alias, 5))\nprint(set.length(alias))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "true\n1\n" {
		t.Fatal(out.String())
	}
}

func TestSetWordSignatureMismatches(t *testing.T) {
	for _, src := range []string{
		"let s: Set<Int> = set.new\nset.add(s, \"x\")\n",
		"let s: Set<Int> = set.new\nset.contains(s, 1, 2)\n",
		"set.length(5)\n",
		"set.values(\"nope\")\n",
	} {
		if _, err := Compile("test.zhex", src); err == nil {
			t.Errorf("accepted %q", src)
		}
	}
}
