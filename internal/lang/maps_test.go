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
