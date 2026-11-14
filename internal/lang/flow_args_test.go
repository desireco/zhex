package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestFlowSuppliesFirstArgumentAndCallsExtraArguments(t *testing.T) {
	source := "flow \"hello\"\n    text.replace(\"e\", \"a\")\n    text.upper\n    print\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "HALLO\n" {
		t.Fatal(out.String())
	}
}

func TestFlowInfersGenericChoiceFromSeed(t *testing.T) {
	source := "type Maybe<T> = choice\n    Some(value: T)\nlet result = 0\nflow 7\n    Maybe.Some\n    unwrap\n    print\nword unwrap(value: Maybe<Int>) -> Int\n    match value\n        case Maybe.Some(n)\n            n\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}

func TestFlowExtraArgumentEffectsAreInferred(t *testing.T) {
	source := "word replace(raw: String) -> String\n    flow raw\n        text.replace(\"x\", int.to-string(text.to-int(\"4\") try))\nprint(replace(\"x\") try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "4\n" {
		t.Fatal(out.String())
	}
}

func TestFlowExtraArgumentsEvaluateOnceLeftToRight(t *testing.T) {
	source := "word mark(n: Int) -> Int\n    print(n)\n    n\nword add(a: Int, b: Int, c: Int) -> Int\n    a + b + c\nflow mark(1)\n    add(mark(2), mark(3))\n    print\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n2\n3\n6\n" {
		t.Fatal(out.String())
	}
}

func TestFlowExtraArgumentsUseParameterContext(t *testing.T) {
	source := "word total(seed: Int, items: List<Int>) -> Int\n    seed + list.length(items)\nflow 7\n    total([])\n    print\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}

func TestFlowCannotLaunderCaughtErrorIntoGenericChoice(t *testing.T) {
	source := "type Maybe<T> = choice\n    Some(value: T)\nlet n = text.to-int(\"bad\") catch problem\n    flow problem\n        Maybe.Some\n    0\n"
	if _, err := Compile("test.zhex", source); err == nil {
		t.Fatal("flow erased concrete error family")
	}
}

func TestNonFinalFlowDoesNotInheritWordReturnContext(t *testing.T) {
	source := "type Maybe<T> = choice\n    Some(value: T)\nword demo() -> Maybe<String>\n    flow 7\n        Maybe.Some\n    Maybe.Some(\"ok\")\nmatch demo()\n    case Maybe.Some(value)\n        print(value)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "ok\n" {
		t.Fatal(out.String())
	}
}
