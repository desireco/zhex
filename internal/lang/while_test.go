package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestWhileLoops(t *testing.T) {
	cases := []struct{ name, source, input, want string }{
		{"counter", "var i = 0\nwhile i < 3\n    i = i + 1\nprint(i)\n", "", "3\n"},
		{"initially false", "var i = 0\nwhile i < 0\n    i = i + 1\nprint(i)\n", "", "0\n"},
		{"each pass", "var i = 0\nwhile i < 3\n    print(i)\n    i = i + 1\n", "", "0\n1\n2\n"},
		{"flag exit", "var go = true\nvar n = 0\nwhile go\n    n = n + 1\n    if n == 2\n        go = false\nprint(n)\n", "", "2\n"},
		{"nested while", "var i = 0\nwhile i < 2\n    var j = 0\n    while j < 2\n        print(i * 10 + j)\n        j = j + 1\n    i = i + 1\n", "", "0\n1\n10\n11\n"},
		{"while inside for", "for n in range(1, 3)\n    var i = 0\n    while i < n\n        print(n)\n        i = i + 1\n", "", "1\n2\n2\n"},
		{"for inside while", "var i = 0\nwhile i < 2\n    for n in range(0, i + 1)\n        print(n)\n    i = i + 1\n", "", "0\n0\n1\n"},
		{"while inside word", "word count-until(limit: Int) -> Int\n    var i = 0\n    while i < limit\n        i = i + 1\n    i\nprint(count-until(4))\n", "", "4\n"},
		{"unit result", "word tick() -> Unit\n    var i = 0\n    while i < 2\n        i = i + 1\nprint(\"before\")\ntick()\nprint(\"done\")\n", "", "before\ndone\n"},
		{"inferred unit", "word tick()\n    var i = 0\n    while i < 2\n        i = i + 1\ntick()\nprint(\"done\")\n", "", "done\n"},
		{
			"driver idiom",
			"var done = false\nvar lines = 0\nwhile not done\n    let line = read-line(\"\") catch\n        done = true\n        \"\"\n    if line != \"\"\n        lines = lines + 1\nprint(lines)\n",
			"a\nb\n\n",
			"2\n",
		},
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

func TestWhileRuntimeErrorPropagates(t *testing.T) {
	var out bytes.Buffer
	err := Run("boom.zhex", "var i = 0\nwhile i < 3\n    i = i + 1\n    print(i)\n    1 / 0\n", strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "division or remainder by zero") || !strings.Contains(err.Error(), "boom.zhex:5:") {
		t.Fatalf("wanted runtime error, got %v", err)
	}
	if out.String() != "1\n" {
		t.Fatalf("earlier passes did not run: %q", out.String())
	}
}

func TestWhileRejected(t *testing.T) {
	cases := []struct{ name, source, message string }{
		{"condition", "while 1\n    print(1)\n", "condition must have type Bool, found Int"},
		{"result condition", "while read-line(\"\")\n    print(1)\n", "condition must have type Bool, found Result<String, InputError>"},
		{"let rhs", "let x = while true\n    print(1)\n", `expected an expression, found "while"`},
		{"var rhs", "var x = while true\n    print(1)\n", `expected an expression, found "while"`},
		{"assign rhs", "var x = 1\nx = while true\n    print(1)\n", `expected an expression, found "while"`},
		{"argument", "print(while true)\n", `expected an expression, found "while"`},
		{"word name", "word while() -> Int\n    1\n", `expected a name, found keyword "while"`},
		{"binding name", "let while = 1\n", `expected a name, found keyword "while"`},
		{"body result", "while true\n    read-line(\"\")\n", "Result must be handled"},
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

func TestWhileSession(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := session.Enter("var i = 0"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Enter("while i < 3\n    i = i + 1\n    print(i)\n"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n2\n3\n" {
		t.Fatalf("session output %q", out.String())
	}
	if echo, err := session.Enter("i"); err != nil || echo != "3" {
		t.Fatalf("echo %q, error %v", echo, err)
	}
}

func TestWhileSessionRuntimeErrorKeepsEffects(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := session.Enter("var i = 0"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Enter("while i < 3\n    i = i + 1\n    print(i)\n    1 / 0\n"); err == nil || err.(*diagnostic).code != CodeDivisionZero {
		t.Fatalf("wanted division-zero, got %v", err)
	}
	if out.String() != "1\n" {
		t.Fatalf("earlier effects lost: %q", out.String())
	}
	if echo, err := session.Enter("i"); err != nil || echo != "1" {
		t.Fatalf("echo %q, error %v", echo, err)
	}
	if echo, err := session.Enter("1 + 1"); err != nil || echo != "2" {
		t.Fatalf("session did not survive: %q, %v", echo, err)
	}
}

func TestReadLineEOFReason(t *testing.T) {
	t.Run("end of file", func(t *testing.T) {
		source := "let first = read-line(\"\") try\nlet second = read-line(\"\") catch e\n    error.detail(e, \"reason\")\nprint(first)\nprint(second)\n"
		var out bytes.Buffer
		if err := Run("eof.zhex", source, strings.NewReader("a\n"), &out); err != nil {
			t.Fatal(err)
		}
		if out.String() != "a\nend-of-file\n" {
			t.Fatalf("output %q", out.String())
		}
	})
	t.Run("empty line is not end of file", func(t *testing.T) {
		source := "let first = read-line(\"\") try\nprint(text.length(first))\nlet second = read-line(\"\") catch e\n    error.detail(e, \"reason\")\nprint(second)\n"
		var out bytes.Buffer
		if err := Run("empty.zhex", source, strings.NewReader("\n"), &out); err != nil {
			t.Fatal(err)
		}
		if out.String() != "0\nend-of-file\n" {
			t.Fatalf("output %q", out.String())
		}
	})
	t.Run("final line without newline", func(t *testing.T) {
		source := "let first = read-line(\"\") try\nprint(first)\nlet second = read-line(\"\") catch e\n    error.detail(e, \"reason\")\nprint(second)\n"
		var out bytes.Buffer
		if err := Run("partial.zhex", source, strings.NewReader("abc"), &out); err != nil {
			t.Fatal(err)
		}
		if out.String() != "abc\nend-of-file\n" {
			t.Fatalf("output %q", out.String())
		}
	})
	t.Run("read failure", func(t *testing.T) {
		source := "let line = read-line(\"\") catch e\n    error.detail(e, \"reason\")\nprint(line)\n"
		var out bytes.Buffer
		if err := Run("fail.zhex", source, failingReader{}, &out); err != nil {
			t.Fatal(err)
		}
		if out.String() != "read-failed\n" {
			t.Fatalf("output %q", out.String())
		}
	})
	t.Run("unhandled eof message", func(t *testing.T) {
		var out bytes.Buffer
		err := Run("unhandled.zhex", "let n = read-line(\"\") try\nprint(n)\n", strings.NewReader(""), &out)
		if err == nil || !strings.Contains(err.Error(), "end of input") {
			t.Fatalf("wanted end-of-input diagnostic, got %v", err)
		}
	})
}
