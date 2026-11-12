package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestSyntaxDepthLimit(t *testing.T) {
	for name, source := range map[string]string{
		"parentheses":  "print(" + strings.Repeat("(", 300) + "1" + strings.Repeat(")", 300) + ")\n",
		"unary":        "print(" + strings.Repeat("not ", 300) + "true)\n",
		"binary chain": "print(1" + strings.Repeat(" + 1", 300) + ")\n",
		"elif chain":   "if false\n    print(0)\n" + strings.Repeat("elif false\n    print(0)\n", 300),
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			err := Run("deep.zhex", source, strings.NewReader(""), &out)
			if err == nil || !strings.Contains(err.Error(), "nesting limit") {
				t.Fatalf("wanted nesting diagnostic, got %v", err)
			}
		})
	}
}

func TestRecursiveWordLimit(t *testing.T) {
	var out bytes.Buffer
	err := Run("recursive.zhex", "word again(n: Int) -> Int\n    again(n)\nprint(again(1))\n", strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "call depth limit") {
		t.Fatalf("wanted call depth error, got %v", err)
	}
}

func TestCheckedProgramHasFreshState(t *testing.T) {
	p, err := Compile("state.zhex", "var n = 1\nn = n + 1\nprint(n)\n")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		var out bytes.Buffer
		if err := p.Run(strings.NewReader(""), &out); err != nil {
			t.Fatal(err)
		}
		if out.String() != "2\n" {
			t.Fatalf("state leaked between executions: %q", out.String())
		}
	}
}
