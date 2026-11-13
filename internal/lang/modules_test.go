package lang

import (
	"strings"
	"testing"
)

// mapResolver hands out fixed modules by name, the in-memory shape of a
// directory of sibling <name>.zhex files.
func mapResolver(files map[string]string) func(string) (ModuleSource, bool) {
	return func(name string) (ModuleSource, bool) {
		source, ok := files[name+".zhex"]
		if !ok {
			return ModuleSource{}, false
		}
		return ModuleSource{Path: name + ".zhex", Source: source}, true
	}
}

const mathModule = `word gcd(a: Int, b: Int) -> Int
    if b == 0
        a
    else
        gcd(b, a % b)
`

// compileModulesError returns the error text of a failed CompileModules run.
func compileModulesError(t *testing.T, entry string, resolve func(string) (ModuleSource, bool)) string {
	t.Helper()
	_, err := CompileModules("main.zhex", entry, resolve)
	if err == nil {
		t.Fatal("expected an error")
	}
	return err.Error()
}

func TestUseParsesIntoProgramUses(t *testing.T) {
	p, err := parse("main.zhex", "use math\nprint(1)\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.uses) != 1 || p.uses[0].name != "math" {
		t.Fatalf("uses = %+v, want one math", p.uses)
	}
	if len(p.body) != 1 {
		t.Fatalf("use must not become a statement, body = %+v", p.body)
	}
}

func TestUseInsideWordBodyIsUseLocation(t *testing.T) {
	got := compileModulesError(t, "word f() -> Unit\n    use math\n    print(1)\nprint(f())\n", nil)
	if !strings.HasPrefix(got, "main.zhex:2:5: use is only allowed at the top level") {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "\n        ^^^^^^^^\nhint:") {
		t.Fatalf("caret must span the use statement: %q", got)
	}
	if !strings.Contains(got, "hint: move the use statement to the top level of the file") {
		t.Fatalf("missing hint: %q", got)
	}
}

func TestUseDottedTargetIsExpectedName(t *testing.T) {
	got := compileModulesError(t, "use math.extra\n", nil)
	if !strings.HasPrefix(got, `main.zhex:1:5: expected a module name, found "math.extra"`) {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "hint: module names cannot contain dots; use the module's name, as in use math") {
		t.Fatalf("missing hint: %q", got)
	}
}

func TestCompileModulesResolvesSiblingModules(t *testing.T) {
	calls := []string{}
	resolve := func(name string) (ModuleSource, bool) {
		calls = append(calls, name)
		return mapResolver(map[string]string{"math.zhex": mathModule})(name)
	}
	if _, err := CompileModules("main.zhex", "use math\nprint(1)\n", resolve); err != nil {
		t.Fatalf("resolution failed: %v", err)
	}
	if len(calls) != 1 || calls[0] != "math" {
		t.Fatalf("resolver saw %v, want [math]", calls)
	}
}

func TestDuplicateUseIsIdempotent(t *testing.T) {
	calls := []string{}
	resolve := func(name string) (ModuleSource, bool) {
		calls = append(calls, name)
		return mapResolver(map[string]string{"math.zhex": mathModule})(name)
	}
	if _, err := CompileModules("main.zhex", "use math\nuse math\nprint(1)\n", resolve); err != nil {
		t.Fatalf("duplicate use must be a no-op: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("math resolved %d times, want 1", len(calls))
	}
}

func TestSessionDuplicateUseInEntryIsIdempotent(t *testing.T) {
	var out strings.Builder
	s := NewSession(strings.NewReader(""), &out, nil, mapResolver(map[string]string{"math.zhex": mathModule}))
	if _, err := s.Enter("use math\nuse math\n"); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("duplicate use produced output: %q", out.String())
	}
	if echo, err := s.Enter("math.gcd(48, 18)\n"); err != nil || echo != "6" {
		t.Fatalf("adopted word: echo %q, error %v", echo, err)
	}
}

func TestUseMissingNamesTheSearchedPath(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entry   string
		resolve func(string) (ModuleSource, bool)
	}{
		{"resolver without the file", "use math\nprint(1)\n", mapResolver(nil)},
		{"no resolver at all", "use math\nprint(1)\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := compileModulesError(t, tc.entry, tc.resolve)
			if !strings.HasPrefix(got, `main.zhex:1:1: module "math" not found; searched math.zhex`) {
				t.Fatalf("first line changed: %q", got)
			}
			if !strings.Contains(got, "\n    ^^^^^^^^\nhint: create math.zhex next to main.zhex, or fix the module name") {
				t.Fatalf("missing caret or hint: %q", got)
			}
		})
	}
}

func TestUseCycleSpellsThePath(t *testing.T) {
	got := compileModulesError(t, "use math\n", mapResolver(map[string]string{
		"math.zhex": "use math\n" + mathModule,
	}))
	if !strings.HasPrefix(got, "math.zhex:1:1: module cycle: math.zhex -> math.zhex") {
		t.Fatalf("self-import diagnostic changed: %q", got)
	}
	got = compileModulesError(t, "use a\n", mapResolver(map[string]string{
		"a.zhex": "use b\nword f(n: Int) -> Int\n    n\n",
		"b.zhex": "use a\nword g(n: Int) -> Int\n    n\n",
	}))
	if !strings.HasPrefix(got, "b.zhex:1:1: module cycle: a.zhex -> b.zhex -> a.zhex") {
		t.Fatalf("two-module cycle diagnostic changed: %q", got)
	}
}

func TestUseCycleBackToEntry(t *testing.T) {
	for _, entryPath := range []string{"main.zhex", "./main.zhex"} {
		for _, indirect := range []bool{false, true} {
			source := "use main\nprint(1)\n"
			if indirect {
				source = "use math\nprint(1)\n"
			}
			_, err := CompileModules(entryPath, source, mapResolver(map[string]string{
				"main.zhex": source,
				"math.zhex": "use main\n" + mathModule,
			}))
			if code, _ := codeOf(t, err); code != "use-cycle" {
				t.Fatalf("entry %s, indirect %v: wanted use-cycle, got %v", entryPath, indirect, err)
			}
		}
	}
}

func TestUseReservedModuleNames(t *testing.T) {
	for _, name := range []string{"text", "int", "buffer", "file", "console"} {
		got := compileModulesError(t, "use "+name+"\n", mapResolver(map[string]string{
			name + ".zhex": mathModule,
		}))
		want := `main.zhex:1:1: module name "` + name + `" is reserved`
		if !strings.HasPrefix(got, want) {
			t.Fatalf("first line changed: %q", got)
		}
		if !strings.Contains(got, "\n    "+strings.Repeat("^", len("use ")+len(name))+"\nhint:") {
			t.Fatalf("caret must span the use statement: %q", got)
		}
		if !strings.Contains(got, "hint: rename the module file — a module cannot be named text, int, buffer, file, or console") {
			t.Fatalf("missing hint: %q", got)
		}
	}
}

func TestModuleBodyRejectsTopLevelStatements(t *testing.T) {
	got := compileModulesError(t, "use math\n", mapResolver(map[string]string{
		"math.zhex": "print(1)\n" + mathModule,
	}))
	if !strings.HasPrefix(got, "math.zhex:1:1: imported files declare words only; move the statements into the file you run") {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "\n    print(1)\n") {
		t.Fatalf("excerpt must come from the module file: %q", got)
	}
}

// privateMathModule pins underscore privacy: gcd calls _wrap bare in its own
// file; from outside, _wrap is private-use.
const privateMathModule = `word gcd(a: Int, b: Int) -> Int
    if b == 0
        _wrap(a)
    else
        gcd(b, a % b)
word _wrap(n: Int) -> Int
    n
`

func mathFiles(source string) func(string) (ModuleSource, bool) {
	return mapResolver(map[string]string{"math.zhex": source})
}

func TestModuleWordsRunQualified(t *testing.T) {
	p, err := CompileModules("main.zhex", "use math\nprint(math.gcd(48, 18))\n", mathFiles(privateMathModule))
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := p.Run(strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "6\n" {
		t.Fatalf("got %q, want gcd(48, 18) = 6", out.String())
	}
}

func TestBareModuleWordFromEntryIsUnknownWord(t *testing.T) {
	got := compileModulesError(t, "use math\nprint(gcd(48, 18))\n", mathFiles(mathModule))
	if !strings.HasPrefix(got, `main.zhex:2:7: unknown word "gcd"`) {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, `did you mean "math.gcd"?`) {
		t.Fatalf("bare name must suggest the qualified word: %q", got)
	}
}

func TestQualifiedCallWithoutUseIsUnknownWord(t *testing.T) {
	got := compileModulesError(t, "print(math.gcd(48, 18))\n", mathFiles(mathModule))
	if !strings.HasPrefix(got, `main.zhex:1:7: unknown word "math.gcd"`) {
		t.Fatalf("first line changed: %q", got)
	}
}

func TestModuleWordsCallEachOtherBare(t *testing.T) {
	if _, err := CompileModules("main.zhex", "use math\nprint(math.gcd(48, 18))\n", mathFiles(privateMathModule)); err != nil {
		t.Fatalf("module's own words must call each other bare: %v", err)
	}
}

func TestModuleCannotCallEntryWords(t *testing.T) {
	got := compileModulesError(t, "use math\nword helper(n: Int) -> Int\n    n\nprint(math.gcd(4, 2))\n", mathFiles(
		"word gcd(a: Int, b: Int) -> Int\n    helper(a)\n"))
	if !strings.HasPrefix(got, `math.zhex:2:5: unknown word "helper"`) {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "\n        helper(a)\n") {
		t.Fatalf("excerpt must come from the module file: %q", got)
	}
}

func TestUnderscoreWordFromOutsideIsPrivateUse(t *testing.T) {
	got := compileModulesError(t, "use math\nprint(math._wrap(1))\n", mathFiles(privateMathModule))
	if !strings.HasPrefix(got, `main.zhex:2:7: "_wrap" is private to module "math"`) {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "\n          ^^^^^^^^^^^^^\n") {
		t.Fatalf("caret must span the qualified call: %q", got)
	}
	if !strings.Contains(got, "hint: remove the leading underscore in math.zhex to export it") {
		t.Fatalf("missing hint: %q", got)
	}
}

func TestModuleSuggestionsComeFromPublicsOnly(t *testing.T) {
	source := "word double(n: Int) -> Int\n    n * 2\nword _secret(n: Int) -> Int\n    n\n"
	if got := compileModulesError(t, "use math\nprint(math.doble(2))\n", mathFiles(source)); !strings.Contains(got, `did you mean "math.double"?`) {
		t.Fatalf("missing public suggestion: %q", got)
	}
	if got := compileModulesError(t, "use math\nprint(math._secrt(2))\n", mathFiles(source)); strings.Contains(got, "did you mean") {
		t.Fatalf("underscore words must never be suggested cross-module: %q", got)
	}
}

func TestPerFileDuplicateWordInModule(t *testing.T) {
	got := compileModulesError(t, "use math\n", mathFiles(
		"word f() -> Unit\n    print(1)\nword f() -> Unit\n    print(2)\n"))
	if !strings.HasPrefix(got, `math.zhex:3:6: word "f" is already defined`) {
		t.Fatalf("first line changed: %q", got)
	}
}

func TestEntryWordCollidingWithAdoptedKey(t *testing.T) {
	got := compileModulesError(t, "use math\nword math.gcd(a: Int, b: Int) -> Int\n    a\nprint(math.gcd(48, 18))\n", mathFiles(mathModule))
	if !strings.HasPrefix(got, `main.zhex:2:6: word "math.gcd" is already defined`) {
		t.Fatalf("first line changed: %q", got)
	}
}

func TestTwoModulesNeverCollide(t *testing.T) {
	files := map[string]string{
		"math.zhex":  mathModule,
		"stats.zhex": "word gcd(a: Int, b: Int) -> Int\n    a - b\n",
	}
	p, err := CompileModules("main.zhex", "use math\nuse stats\nprint(math.gcd(48, 18))\nprint(stats.gcd(7, 3))\n", mapResolver(files))
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := p.Run(strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "6\n4\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestModuleUsesModulePerFile(t *testing.T) {
	files := map[string]string{
		"a.zhex": "use b\nword f(n: Int) -> Int\n    b.g(n)\n",
		"b.zhex": "word g(n: Int) -> Int\n    n\n",
	}
	p, err := CompileModules("main.zhex", "use a\nprint(a.f(2))\n", mapResolver(files))
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
	// A used module's own imports stay private to it.
	if got := compileModulesError(t, "use a\nprint(b.g(2))\n", mapResolver(files)); !strings.Contains(got, `unknown word "b.g"`) {
		t.Fatalf("transitive use must not leak: %q", got)
	}
}

func TestModuleBodiesCheckBeforeEntry(t *testing.T) {
	got := compileModulesError(t, "use math\nif 1\n    print(1)\n", mathFiles(
		"word bad() -> Int\n    true\n"))
	if !strings.HasPrefix(got, `math.zhex:1:6: word "bad" return type is Bool; declared Int`) {
		t.Fatalf("module error must surface first: %q", got)
	}
}

const runtimeMathModule = `word boom() -> Int
    1 / 0
word halve(n: Int) -> Int
    n / 2
word double(n: Int) -> Int
    n * 2
`

func runtimeFiles() func(string) (ModuleSource, bool) {
	return mapResolver(map[string]string{"math.zhex": runtimeMathModule})
}

func TestRuntimeErrorInModuleWordAnnotatesModuleSource(t *testing.T) {
	var out strings.Builder
	err := RunWithModules("main.zhex", "use math\nprint(math.boom())\n", strings.NewReader(""), &out, nil, nil, runtimeFiles())
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.HasPrefix(err.Error(), "math.zhex:2:7: division or remainder by zero") {
		t.Fatalf("first line changed: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "\n        1 / 0\n") || !strings.Contains(err.Error(), "\n          ^") {
		t.Fatalf("excerpt and caret must come from math.zhex: %q", err.Error())
	}
}

func TestRunWithModulesTracesQualifiedNames(t *testing.T) {
	var out, trace strings.Builder
	err := RunWithModules("main.zhex", "use math\nprint(math.double(21))\nflow 84\n    math.halve\n    print\n", strings.NewReader(""), &out, &trace, nil, runtimeFiles())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`{"event":"word-call","word":"math.double","file":"main.zhex","line":2,"args":["21"]}`,
		`{"event":"word-return","word":"math.double","value":"42","file":"math.zhex","line":5}`,
		`{"event":"flow-step","word":"math.halve","value":"84","file":"main.zhex","line":4}`,
		`{"event":"word-call","word":"math.halve","file":"main.zhex","line":4,"args":["84"]}`,
	} {
		if !strings.Contains(trace.String(), want) {
			t.Fatalf("trace misses %s:\n%s", want, trace.String())
		}
	}
}

func TestRunWithModulesReportCarriesModulePath(t *testing.T) {
	var out strings.Builder
	err := RunWithModules("main.zhex", "use math\nprint(math.boom())\n", strings.NewReader(""), &out, nil, nil, runtimeFiles())
	if err == nil {
		t.Fatal("expected an error")
	}
	report, ok := Report(err)
	if !ok {
		t.Fatal("error is not a diagnostic")
	}
	for _, want := range []string{`"code":"division-zero"`, `"file":"math.zhex"`} {
		if !strings.Contains(report, want) {
			t.Fatalf("report %q misses %s", report, want)
		}
	}
	if strings.Contains(report, "span_") {
		t.Fatalf("runtime report must stay span-free: %s", report)
	}
}

func TestMultiFileOutputMatchesSingleFile(t *testing.T) {
	entry := "use math\nprint(math.double(21))\nflow 84\n    math.halve\n    print\n"
	var multi, single strings.Builder
	if err := RunWithModules("main.zhex", entry, strings.NewReader(""), &multi, nil, nil, runtimeFiles()); err != nil {
		t.Fatal(err)
	}
	if err := Run("main.zhex", runtimeMathModule+"\nprint(double(21))\nflow 84\n    halve\n    print\n", strings.NewReader(""), &single); err != nil {
		t.Fatal(err)
	}
	if multi.String() != single.String() || multi.String() != "42\n42\n" {
		t.Fatalf("multi %q single %q", multi.String(), single.String())
	}
}

func TestSelfImportingEntryIsUseCycle(t *testing.T) {
	entry := "use main\nprint(1)\n"
	got := compileModulesError(t, entry, mapResolver(map[string]string{"main.zhex": entry}))
	if !strings.HasPrefix(got, "main.zhex:1:1: module cycle: main.zhex -> main.zhex") {
		t.Fatalf("first line changed: %q", got)
	}
}

// F6: the trace reports what actually ran — a module-private word keeps its
// qualified name in the event stream.
func TestTraceKeepsQualifiedNamesForModulePrivateWords(t *testing.T) {
	files := mapResolver(map[string]string{"math2.zhex": "word pub(n: Int) -> Int\n    _inner(n)\nword _inner(n: Int) -> Int\n    n + 1\n"})
	var out, trace strings.Builder
	if err := RunWithModules("main.zhex", "use math2\nprint(math2.pub(21))\n", strings.NewReader(""), &out, &trace, nil, files); err != nil {
		t.Fatal(err)
	}
	if out.String() != "22\n" {
		t.Fatalf("got %q", out.String())
	}
	for _, want := range []string{
		`{"event":"word-call","word":"math2._inner"`,
		`{"event":"word-return","word":"math2._inner","value":"22"`,
	} {
		if !strings.Contains(trace.String(), want) {
			t.Fatalf("trace misses %s:\n%s", want, trace.String())
		}
	}
}

func TestUseHoistsBelowUseSite(t *testing.T) {
	p, err := CompileModules("main.zhex", "print(f())\nword f() -> Int\n    math.gcd(12, 18)\nuse math\n", mathFiles(mathModule))
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := p.Run(strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "6\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestModuleSyntaxErrorAnnotatesModuleSource(t *testing.T) {
	got := compileModulesError(t, "use math\n", mapResolver(map[string]string{
		"math.zhex": "word f() -> Int\n    1 +\n",
	}))
	if !strings.HasPrefix(got, "math.zhex:2:") {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "\n        1 +\n") {
		t.Fatalf("excerpt must come from the module file: %q", got)
	}
}
