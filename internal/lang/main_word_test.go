package lang

import (
	"regexp"
	"strings"
	"testing"
)

// The reserved zero-argument main word (#134, option b): an entry file whose
// top level has no statements may define `word main`; zhex run and zhex
// compile invoke it as the entry point. Imports stay words-only, and a main
// beside top-level statements, in a module, or in the REPL stays ordinary.

func TestMainWordRunsAsEntryPoint(t *testing.T) {
	for _, tc := range []struct {
		name, source, input, want string
	}{
		{
			name:   "unit main",
			source: "word main() -> Unit\n    print(\"entry\")\n",
			want:   "entry\n",
		},
		{
			name:   "int main discards its value",
			source: "word main() -> Int\n    42\n",
			want:   "",
		},
		{
			name:   "words and main in one file",
			source: "word double(n: Int) -> Int\n    n + n\nword main() -> Unit\n    print(double(21))\n",
			want:   "42\n",
		},
		{
			name:   "uses and records still load",
			source: "use math\nword main() -> Unit\n    print(math.gcd(48, 18))\n",
			want:   "6\n",
		},
		{
			name:   "inferred result main succeeding",
			source: "word main() -> Unit\n    let n = text.to-int(\"3\") try\n    print(n)\n",
			input:  "",
			want:   "3\n",
		},
		{
			name:   "explicit result main succeeding",
			source: "word main() -> Result<Unit, Error>\n    return ok(print(\"ran\"))\n",
			want:   "ran\n",
		},
		{
			name:   "no main preserves empty-program behavior",
			source: "word unused() -> Unit\n    print(\"never\")\n",
			want:   "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := CompileModules("main.zhex", tc.source, mathFiles(mathModule))
			if err != nil {
				t.Fatal(err)
			}
			var out strings.Builder
			if err := p.Run(strings.NewReader(tc.input), &out); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("got %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestMainWordWithStatementsStaysOrdinary(t *testing.T) {
	p, err := Compile("main.zhex", "word main() -> Unit\n    print(\"main\")\nprint(\"statements\")\n")
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := p.Run(strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "statements\n" {
		t.Fatalf("got %q, want the statements' output only", out.String())
	}
}

func TestMainWordCallableBesideStatements(t *testing.T) {
	p, err := Compile("main.zhex", "word main() -> Unit\n    print(\"called\")\nmain()\n")
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := p.Run(strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "called\n" {
		t.Fatalf("got %q, want the explicit call to run once", out.String())
	}
}

func TestMainWordWithParametersIsRejected(t *testing.T) {
	for _, tc := range []struct {
		name, source string
	}{
		{"specific", "word main(n: Int) -> Unit\n    print(n)\n"},
		{"plain", "word main(n) -> Unit\n    print(n)\n"},
		{"defaulted", "word main(n: Int = 3) -> Unit\n    print(n)\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile("main.zhex", tc.source)
			if err == nil {
				t.Fatal("expected a main-parameters diagnostic")
			}
			if code, _ := codeOf(t, err); code != "main-parameters" {
				t.Fatalf("code %s, want main-parameters", code)
			}
			if !strings.HasPrefix(err.Error(), "main.zhex:1:6: main takes no parameters") {
				t.Fatalf("first line changed: %q", err.Error())
			}
			if !strings.Contains(err.Error(), "hint: remove the parameter list from word main, or rename the word — zhex run and zhex compile call main with no arguments") {
				t.Fatalf("missing hint: %q", err.Error())
			}
		})
	}
}

func TestMainWordWithParametersBesideStatementsIsOrdinary(t *testing.T) {
	p, err := Compile("main.zhex", "word main(n: Int) -> Int\n    n\nprint(\"ran\")\n")
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := p.Run(strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "ran\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestModuleMainWordIsOrdinary(t *testing.T) {
	t.Run("never auto-invoked from the entry", func(t *testing.T) {
		p, err := CompileModules("main.zhex", "use math\n", mathFiles("word main() -> Unit\n    print(\"module main\")\n"+mathModule))
		if err != nil {
			t.Fatal(err)
		}
		var out strings.Builder
		if err := p.Run(strings.NewReader(""), &out); err != nil {
			t.Fatal(err)
		}
		if out.String() != "" {
			t.Fatalf("got %q, want no output: a module main is never the entry point", out.String())
		}
	})
	t.Run("parameters allowed in a module", func(t *testing.T) {
		p, err := CompileModules("main.zhex", "use math\nprint(math.main(2))\n", mathFiles("word main(n: Int) -> Int\n    n\n"))
		if err != nil {
			t.Fatal(err)
		}
		var out strings.Builder
		if err := p.Run(strings.NewReader(""), &out); err != nil {
			t.Fatal(err)
		}
		if out.String() != "2\n" {
			t.Fatalf("got %q", out.String())
		}
	})
}

// TestMainWordFailingTryReportsLikeTopLevel pins the design note's parity
// rule: a failing try inside main reports exactly like the same statements at
// the top level, modulo the position inside main.
func TestMainWordFailingTryReportsLikeTopLevel(t *testing.T) {
	position := regexp.MustCompile(`^\S+:\d+:\d+: `)
	_, topErr := runFile(t, "let n = text.to-int(\"x\") try\nprint(n)\n")
	if topErr == nil {
		t.Fatal("top-level try did not fail")
	}
	_, mainErr := runFile(t, "word main() -> Unit\n    let n = text.to-int(\"x\") try\n    print(n)\n")
	if mainErr == nil {
		t.Fatal("main-based try did not fail")
	}
	firstLine := func(err error) string { line, _, _ := strings.Cut(err.Error(), "\n"); return line }
	top, mainBased := position.ReplaceAllString(firstLine(topErr), ""), position.ReplaceAllString(firstLine(mainErr), "")
	if top != mainBased {
		t.Fatalf("main-based failure %q differs from top-level %q", mainBased, top)
	}
	if code, _ := codeOf(t, mainErr); code != "text-to-int" {
		t.Fatalf("code %s, want text-to-int", code)
	}
}
