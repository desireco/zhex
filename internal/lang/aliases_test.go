package lang

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestAliasIsTransparentForAnnotationsAndWords(t *testing.T) {
	source := "type Names = alias List<String>\ntype Count = alias Int\nword size(names: Names) -> Count\n    list.length(names)\nlet names: Names = [\"Ada\"]\nprint(size(names))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n" {
		t.Fatal(out.String())
	}
}
func TestAliasForwardReferencesAndCycles(t *testing.T) {
	source := "type Names = alias Later\ntype Later = alias List<String>\nlet names: Names = []\nprint(list.length(names))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"type A = alias B\ntype B = alias A\n", "type A = alias List<A>\n", "type Count = alias Int\nlet n = Count(2)\n", "type A<T> = alias List<T>\n"} {
		if _, err := Compile("test.zhex", bad); err == nil {
			t.Errorf("accepted invalid alias:\n%s", bad)
		}
	}
}
func TestAliasCannotHideResultObligation(t *testing.T) {
	source := "type Pending = alias Result<Int, ParseError>\nword discard(p: Pending) -> Unit\n    print(1)\ndiscard(text.to-int(\"bad\"))\n"
	if _, err := Compile("test.zhex", source); err == nil {
		t.Fatal("alias hid Result parameter")
	}
}
func TestAliasModulePrivacyAndImports(t *testing.T) {
	resolve := func(name string) (ModuleSource, bool) {
		if name == "model" {
			return ModuleSource{Path: "model.zhex", Source: "type Names = alias List<String>\nword size(names: Names) -> Int\n    list.length(names)\n"}, true
		}
		return ModuleSource{}, false
	}
	source := "use model\nlet names: model.Names = [\"Ada\"]\nprint(model.size(names))\n"
	var out bytes.Buffer
	if err := RunWithModules("main.zhex", source, strings.NewReader(""), &out, nil, nil, resolve); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n" {
		t.Fatal(out.String())
	}
	for _, body := range []string{"type _Hidden = record\n    n: Int\ntype Public = alias _Hidden\n", "type _Names = alias List<String>\nword leak(names: _Names) -> Int\n    list.length(names)\n"} {
		res := func(name string) (ModuleSource, bool) {
			return ModuleSource{Path: "model.zhex", Source: body}, name == "model"
		}
		if _, err := CompileModules("main.zhex", "use model\n", res); err == nil {
			t.Errorf("accepted private alias leak:\n%s", body)
		}
	}
}

func TestPrivateAliasCannotLeakThroughPublicChoice(t *testing.T) {
	resolve := func(name string) (ModuleSource, bool) {
		return ModuleSource{Path: "model.zhex", Source: "type _Names = alias List<String>\ntype Public = choice\n    Value(names: _Names)\n"}, name == "model"
	}
	if _, err := CompileModules("main.zhex", "use model\n", resolve); err == nil {
		t.Fatal("public choice leaked a private alias")
	}
}
func TestAliasResultFamilyMustBeKnown(t *testing.T) {
	if _, err := Compile("test.zhex", "type Bad = alias Result<Int, Bogus>\n"); err == nil {
		t.Fatal("accepted unknown Result family through alias")
	}
}

func TestForwardPublicAliasCannotErasePrivateAlias(t *testing.T) {
	body := "type Public = alias Later\ntype Later = alias _Hidden\ntype _Hidden = alias Int\n"
	resolve := func(name string) (ModuleSource, bool) {
		return ModuleSource{Path: "model.zhex", Source: body}, name == "model"
	}
	if _, err := CompileModules("main.zhex", "use model\n", resolve); err == nil {
		t.Fatal("forward public alias erased private alias name")
	}
}

func TestAliasResultReturnAndREPL(t *testing.T) {
	source := "type Parsed = alias Result<Int, ParseError>\nword parse(raw: String) -> Parsed\n    ok(text.to-int(raw) try)\nprint(parse(\"7\") try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
	s := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := s.Enter("type Names = alias List<String>\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enter("let names: Names = [\"Ada\"]\n"); err != nil {
		t.Fatal(err)
	}
	if echo, err := s.Enter("list.length(names)\n"); err != nil || echo != "1" {
		t.Fatalf("echo %q error %v", echo, err)
	}
}
func TestAliasDepthLimit(t *testing.T) {
	var source strings.Builder
	for i := 0; i < 258; i++ {
		fmt.Fprintf(&source, "type A%d = alias A%d\n", i, i+1)
	}
	source.WriteString("type A258 = alias Int\n")
	if _, err := Compile("test.zhex", source.String()); err == nil {
		t.Fatal("accepted excessive alias chain")
	} else if d, ok := err.(*diagnostic); !ok || d.code != CodeNestingLimit {
		t.Fatalf("wanted nesting-limit, got %v", err)
	}
}
