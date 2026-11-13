package lang

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestLanguageFeatures(t *testing.T) {
	cases := []struct{ name, source, input, want string }{
		{"precedence", "print((2 + 3) * 4)\nprint(9 / 2)\nprint(-9 % 2)\n", "", "20\n4\n-1\n"},
		{"booleans", "print(not false and 2 < 3 or false)\nprint(true != false)\n", "", "true\ntrue\n"},
		{"short circuit", "print(false and 1 / 0 == 0)\nprint(true or 1 / 0 == 0)\n", "", "false\ntrue\n"},
		{"minimum integer", "print(-9223372036854775808)\nprint(-9223372036854775808 % -1)\n", "", "-9223372036854775808\n0\n"},
		{"strings", "print(\"a\\nb\" + \"!\")\nprint(text.reverse(\"abé\"))\n", "", "a\nb!\néba\n"},
		{"mutable loop", "var sum = 0\nfor n in range(1, 5)\n    sum = sum + n\nprint(sum)\n", "", "10\n"},
		{"empty ranges", "for n in range(2, 1)\n    print(n)\nfor n in range(2, 2)\n    print(n)\nprint(7)\n", "", "7\n"},
		{"range edge", "for n in range(9223372036854775806, 9223372036854775807)\n    print(n)\n", "", "9223372036854775806\n"},
		{"shadowing", "let n = 1\nif true\n    let n = 2\n    print(n)\nprint(n)\n", "", "2\n1\n"},
		{"forward calls", "word first(n: Int) -> Int\n    second(n)\nword second(n: Int) -> Int\n    n + 1\nprint(first(4))\n", "", "5\n"},
		{"multiple arguments", "word add(a: Int, b: Int) -> Int\n    a + b\nprint(add(2, 3))\n", "", "5\n"},
		{"calls equal flows", "word square(n: Int) -> Int\n    n * n\nprint(square(4))\nflow 4\n    square\n    print\n", "", "16\n16\n"},
		{"recursive result", "word count(n: Int) -> Int\n    if n == 0\n        0\n    else\n        count(n - 1) + 1\nprint(count(5))\n", "", "5\n"},
		{"signed input", "let a = text.to-int(read-line(\"\") try) try\nlet b = text.to-int(read-line(\"\") try) try\nprint(a + b)\n", " +12 \n-2", "10\n"},
		{"text.to-int", "print(text.to-int(\"  42  \") try)\nprint(text.to-int(\"-7\") try)\n", "", "42\n-7\n"},
		{"parse result binding", "let pending = text.to-int(\"9\")\nprint(pending try)\n", "", "9\n"},
		{"flow seed from read", "flow read-line(\"\") try\n    print\n", "hi\n", "hi\n"},
		{"result binding", "let pending = read-line(\"\")\nlet line = pending try\nprint(line)\n", "hello\n", "hello\n"},
		{"comments and blank lines", "# hello\n\nif true\n\n    # inside\n    print(1) # trailing\n", "", "1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Run(tc.name+".zhex", tc.source, strings.NewReader(tc.input), &out); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("got %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestRejectedPrograms(t *testing.T) {
	cases := []struct{ name, source, message string }{
		{"unknown", "print(1)\nmissing(2)\n", "unknown word"},
		{"arity", "print()\n", "argument"},
		{"parameter type", "word square(n: Int) -> Int\n    n * n\nprint(square(\"wrong\"))\n", "Int"},
		{"duplicate parameter", "word add(n: Int, n: Int) -> Int\n    n + n\n", "already defined"},
		{"operator types", "print(\"x\" + 2)\n", "type"},
		{"immutable", "let x = 1\nx = 2\n", "immutable"},
		{"assignment type", "var x = 1\nx = \"a\"\n", "type"},
		{"scope", "if true\n    let x = 1\nprint(x)\n", "unknown name"},
		{"duplicate", "let x = 1\nlet x = 2\n", "already defined"},
		{"condition", "if 1\n    print(1)\n", "Bool"},
		{"branch results", "word bad() -> Int\n    if true\n        1\n    else\n        \"no\"\n", "branch"},
		{"missing else", "word bad() -> Int\n    if true\n        1\n", "branch"},
		{"return", "word bad() -> Int\n    \"no\"\n", "return"},
		{"unused word checked", "word bad() -> Int\n    missing(1)\nprint(1)\n", "unknown word"},
		{"no global capture", "let x = 1\nword bad() -> Int\n    x\nprint(bad())\n", "unknown name"},
		{"word duplicate", "word x() -> Int\n    1\nword x() -> Int\n    2\n", "already defined"},
		{"unknown type", "word x(n: Float) -> Int\n    1\n", "unknown type"},
		{"flow type", "flow \"wrong\"\n    int.to-string\n    print\n", "Int"},
		{"flow arity", "word add(a: Int, b: Int) -> Int\n    a + b\nflow 1\n    add\n", "argument"},
		{"try required", "let n = text.to-int(\"x\")\nprint(n + 1)\n", "type"},
		{"invalid try", "let n = 1 try\n", "try"},
		{"try on a Buffer", "let b = buffer.new()\nlet x = b try\n", "try requires a Result"},
		{"buffer.new arity", "buffer.new(1)\n", "argument"},
		{"bad range", "for n in range(\"a\", 2)\n    print(n)\n", "Int"},
		{"range as value", "let x = range(0, 2)\n", "range"},
		{"bad indentation", "if true\n  print(1)\n", "indent"},
		{"tab indentation", "if true\n\tprint(1)\n", "tab"},
		{"missing body", "word x() -> Int\nprint(1)\n", "indented"},
		{"literal overflow", "print(9223372036854775808)\n", "range"},
		{"invalid UTF8", "print(\"\xff\")\n", "UTF-8"},
		{"unterminated string", "print(\"hello)\n", "string"},
		{"reserved binding", "let if = 1\n", "name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := Run("bad.zhex", tc.source, strings.NewReader(""), &out)
			if err == nil || !strings.Contains(err.Error(), tc.message) || !strings.Contains(err.Error(), "bad.zhex:") {
				t.Fatalf("wanted %q diagnostic, got %v", tc.message, err)
			}
			if out.Len() != 0 {
				t.Fatalf("invalid program produced output %q", out.String())
			}
		})
	}
}

func TestRuntimeErrors(t *testing.T) {
	for _, expression := range []string{"9223372036854775807 + 1", "-9223372036854775808 - 1", "3037000500 * 3037000500", "-(-9223372036854775808)", "-9223372036854775808 / -1", "1 / 0", "1 % 0"} {
		t.Run(expression, func(t *testing.T) {
			var out bytes.Buffer
			err := Run("runtime.zhex", fmt.Sprintf("print(%s)\n", expression), strings.NewReader(""), &out)
			if err == nil || !strings.Contains(err.Error(), "runtime.zhex:1:") {
				t.Fatalf("wanted runtime error, got %v", err)
			}
		})
	}
}
