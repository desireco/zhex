package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestPartialsCaptureArgumentsAndCallFunctions(t *testing.T) {
	source := `word add left: Int, right: Int -> Int
    left + right
let plus-five = add _, 5
let five-plus: (Int) -> Int = add 5, _
print plus-five(7)
let n = five-plus 7
print n
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "12\n12\n" {
		t.Fatal(out.String())
	}
}

func TestPartialsUseDefaultsOnInvocation(t *testing.T) {
	source := `word suffix() -> String
    print("default")
    "!"
word greet name: String, mark: String = suffix() -> String
    name + mark
let greeter = greet _
print greeter("Ada")
print greeter("Grace")
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "default\nAda!\ndefault\nGrace!\n" {
		t.Fatal(out.String())
	}
}

func TestPartialsInferFromWordsAndWorkInFlow(t *testing.T) {
	source := `word add left, right
    left + right
let plus-five = add _, 5
flow 7
    plus-five
    print
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "12\n" {
		t.Fatal(out.String())
	}
}

func TestCallableParameterAndPartialOfPartial(t *testing.T) {
	source := `word add a: Int, b: Int, c: Int -> Int
    a + b + c
word apply(op: (Int) -> Int, n: Int) -> Int
    op(n)
let two-holes = add _, 2, _
let plus-five = two-holes 3, _
print(apply(plus-five, 7))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "12\n" {
		t.Fatal(out.String())
	}
}

func TestPartialCapturesValuesAndSharedCollections(t *testing.T) {
	source := `word describe(values: List<Int>, n: Int) -> Int
    list.length(values) + n
var n = 2
let values = [1]
let describe-now = describe values, _
let add-n = describe _, n
n = 9
list.append(values, 2)
print(describe-now(3))
print(add-n(values))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "5\n4\n" {
		t.Fatal(out.String())
	}
}

func TestPartialCapturesOnceAndRejectsMissingRequired(t *testing.T) {
	source := `word mark(n: Int) -> Int
    print(n)
    n
word add(a: Int, b: Int) -> Int
    a + b
let p = add mark(5), _
print(p(1))
print(p(2))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "5\n6\n7\n" {
		t.Fatal(out.String())
	}
	if _, err := Compile("test.zhex", "word add(a: Int, b: Int) -> Int\n    a + b\nlet p = add 1\n"); err == nil {
		t.Fatal("accepted implicit currying")
	}
}

func TestFlowCanCreatePartial(t *testing.T) {
	source := `word add(a: Int, b: Int) -> Int
    a + b
let plus-five = flow 5
    add _
print(plus-five(7))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "12\n" {
		t.Fatal(out.String())
	}
}

func TestBuiltinPartialsInferAndUseContext(t *testing.T) {
	source := `let append-one = list.append _, 1
let values: List<Int> = []
append-one(values)
let show: (Int) -> Unit = print _
show(list.length(values))
let change = text.replace _, "A", "a"
print(change("ADA"))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\naDa\n" {
		t.Fatal(out.String())
	}
	if _, err := Compile("test.zhex", "let show = print _\n"); err == nil {
		t.Fatal("guessed overloaded builtin type")
	}
}

func TestFalliblePartialEffectOccursOnInvocation(t *testing.T) {
	source := `word parse(raw: String) -> Result<Int, ParseError>
    ok(text.to-int(raw) try)
word call-later(raw: String)
    let later = parse _
    later(raw) try
print(call-later("7") try)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}

func TestInferredCallableBindingWinsOverBuiltinName(t *testing.T) {
	source := `word add(a: Int, b: Int) -> Int
    a + b
word apply(print, n: Int) -> Int
    print(n)
let plus = add _, 5
print(apply(plus, 2))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}

func TestInferredNonFunctionBindingKeepsBuiltinCall(t *testing.T) {
	source := `word apply(print)
    print(1)
apply(9)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n" {
		t.Fatal(out.String())
	}
}

func TestFalliblePartialThroughInferredFactoryAndCaller(t *testing.T) {
	source := `word parse(raw: String) -> Result<Int, ParseError>
    ok(text.to-int(raw) try)
word factory()
    parse _
word call-later(raw: String)
    let f = factory()
    f(raw) try
word outer(raw: String)
    call-later(raw) try
print(outer("7") try)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}

func TestFallibleCallableParameterInfersEffect(t *testing.T) {
	source := `word parse(raw: String) -> Result<Int, ParseError>
    ok(text.to-int(raw) try)
word invoke(f: (String) -> Result<Int, ParseError>, raw: String)
    f(raw) try
let parse-later = parse _
print(invoke(parse-later, "8") try)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "8\n" {
		t.Fatal(out.String())
	}
}

func TestInferredCallableParameterPropagatesEffect(t *testing.T) {
	source := `word parse(raw: String) -> Result<Int, ParseError>
    ok(text.to-int(raw) try)
word invoke(f, raw: String)
    f(raw) try
word outer(raw: String)
    invoke(parse(_), raw) try
print(outer("8") try)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "8\n" {
		t.Fatal(out.String())
	}
}

func TestFalliblePartialThroughFlowParameter(t *testing.T) {
	source := `word parse(raw: String) -> Result<Int, ParseError>
    ok(text.to-int(raw) try)
word invoke(f, raw: String)
    f(raw) try
word outer(raw: String)
    flow parse _
        invoke(raw) try
print(outer("6") try)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "6\n" {
		t.Fatal(out.String())
	}
}

func TestFalliblePartialThroughConditionalBinding(t *testing.T) {
	source := `word parse(raw: String) -> Result<Int, ParseError>
    ok(text.to-int(raw) try)
word call-later(raw: String)
    let f = if true
        parse _
    else
        parse _
    f(raw) try
word outer(raw: String)
    call-later(raw) try
print(outer("5") try)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "5\n" {
		t.Fatal(out.String())
	}
}

func TestNestedPartialRetainsLatentFailure(t *testing.T) {
	source := `word parse(raw: String) -> Result<Int, ParseError>
    ok(text.to-int(raw) try)
word factory(n: Int)
    parse _
word outer(raw: String)
    let delayed = factory _
    let parser = delayed(1)
    parser(raw) try
word top(raw: String)
    outer(raw) try
print(top("7") try)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}

func TestDefaultedCallableParameterInfersInvocationEffect(t *testing.T) {
	source := `word parse(raw: String) -> Result<Int, ParseError>
    ok(text.to-int(raw) try)
word invoke(raw: String, op = parse(_))
    op(raw) try
word outer(raw: String)
    invoke(raw) try
print(outer("9") try)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "9\n" {
		t.Fatal(out.String())
	}
}

func TestFallibleFunctionsThroughAggregates(t *testing.T) {
	source := `type Handler = record
    action: (String) -> Result<Int, ParseError>
type Holder = choice
    Ready(action: (String) -> Result<Int, ParseError>)
word parse(raw: String) -> Result<Int, ParseError>
    ok(text.to-int(raw) try)
word from-list(raw: String)
    let functions = [parse(_)]
    let f = functions[0]
    f(raw) try
word from-record(raw: String)
    let h = Handler(parse(_))
    let f = h.action
    f(raw) try
word from-choice(raw: String)
    match Holder.Ready(parse(_))
        case Holder.Ready(f)
            f(raw) try
word total(raw: String)
    (from-list(raw) try) + (from-record(raw) try) + (from-choice(raw) try)
print(total("2") try)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "6\n" {
		t.Fatal(out.String())
	}
}

func TestBuiltinPartialFailureAndInvalidPartialTry(t *testing.T) {
	source := `let parse = text.to-int _
let n = parse("bad") catch problem
    8
print n
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "8\n" {
		t.Fatal(out.String())
	}
	for _, source := range []string{
		"let f = text.to-int _ try\n",
		"let f = (array.new([2, 2], _)) try\n",
	} {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted try on a partial: %s", source)
		}
	}
}

func TestPartialCaptureFailureOccursAtCreation(t *testing.T) {
	source := `word add(a: Int, b: Int) -> Int
    a + b
word make(raw: String)
    add(text.to-int(raw) try, _)
let f = make("5") try
print(f(7))
let n = make("bad") catch problem
    add 1, _
print(n(2))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "12\n3\n" {
		t.Fatal(out.String())
	}
}

func TestPartialCannotCaptureConcreteErrorWithoutWrapping(t *testing.T) {
	for _, source := range []string{
		"word inspect(problem: Error, n: Int) -> String\n    error.code(problem)\nlet n = text.to-int(\"bad\") catch problem\n    let f = inspect(problem, _)\n    0\n",
		"let n = text.to-int(\"bad\") catch problem\n    let code: (Error) -> String = error.code _\n    code(problem)\n    0\n",
	} {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted concrete error laundering: %s", source)
		}
	}
}

func TestRecursivePartialFactoryRequiresFiniteType(t *testing.T) {
	source := `word parse(raw: String) -> Result<Int, ParseError>
    ok(text.to-int(raw) try)
word factory(n: Int)
    if n == 0
        parse _
    else
        factory _
`
	if _, err := Compile("test.zhex", source); err == nil {
		t.Fatal("accepted recursive function type")
	}
}

func TestCallableFailureCanBeRethrownWithoutChangingFamily(t *testing.T) {
	source := `word parse(raw: String) -> Result<Int, ParseError>
    ok(text.to-int(raw) try)
word call-later(f: (String) -> Result<Int, ParseError>, raw: String)
    f(raw) catch problem
        let alias = problem
        return err(alias)
word outer(raw: String)
    call-later(parse(_), raw) try
let n = outer("bad") catch problem
    print(error.code(problem))
    3
print n
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "text-to-int\n3\n" {
		t.Fatal(out.String())
	}
}

func TestCallableEffectsRejectMultipleFamilies(t *testing.T) {
	source := `word parse(raw: String) -> Result<Int, ParseError>
    ok(text.to-int(raw) try)
word load(path: String) -> Result<String, FileError>
    ok(file.read-all(path) try)
word run(raw: String)
    let parse-later = parse _
    let load-later = load _
    let n = parse-later(raw) try
    let text = load-later(raw) try
    n
`
	_, err := Compile("test.zhex", source)
	if d, ok := err.(*diagnostic); !ok || d.code != CodeTryInput {
		t.Fatalf("expected incompatible error families, got %v", err)
	}
}

func TestOverloadedBuiltinPartialUsesWholeProgramConstraints(t *testing.T) {
	source := `let show = print _
show(7)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}

func TestOverloadedPartialDoesNotGuessFromFirstCall(t *testing.T) {
	source := "let show = print _\nshow(7)\nshow(\"no\")\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err == nil {
		t.Fatal("accepted conflicting callable uses")
	}
	if out.Len() != 0 {
		t.Fatalf("ran before rejecting conflict: %q", out.String())
	}
}

func TestPartialTraceDoesNotTraverseCapturedCyclesOrCallEarly(t *testing.T) {
	source := `word count(values: List<(Int) -> Int>, n: Int) -> Int
    list.length(values) + n
let values: List<(Int) -> Int> = []
let f = count(values, _)
list.append(values, f)
print(f(7))
`
	var out, trace bytes.Buffer
	if err := RunWithTrace("test.zhex", source, strings.NewReader(""), &out, &trace); err != nil {
		t.Fatal(err)
	}
	if out.String() != "8\n" {
		t.Fatal(out.String())
	}
	if n := strings.Count(trace.String(), `"event":"word-call","word":"count"`); n != 1 {
		t.Fatalf("got %d calls: %s", n, trace.String())
	}
	if !strings.Contains(trace.String(), "function(count, 1 remaining)") {
		t.Fatalf("missing concise function trace: %s", trace.String())
	}
}

func TestTraceFormatsDefaultArgumentsByParameterType(t *testing.T) {
	source := `word greet(n: Int = 1, text: String = "hello") -> String
    text
print(greet())
let later = greet _
print(later(2))
`
	var out, trace bytes.Buffer
	if err := RunWithTrace("test.zhex", source, strings.NewReader(""), &out, &trace); err != nil {
		t.Fatal(err)
	}
	if out.String() != "hello\nhello\n" {
		t.Fatal(out.String())
	}
	if !strings.Contains(trace.String(), `"args":["1","hello"]`) || !strings.Contains(trace.String(), `"args":["2","hello"]`) {
		t.Fatalf("incorrect trace: %s", trace.String())
	}
}

func TestDeferredNonFunctionShadowKeepsResultWordContext(t *testing.T) {
	source := `word wrap(ok, n: Int) -> Result<Int, Error>
    ok(n)
print(wrap(0, 7) try)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}

func TestPartialFailuresPointAtInvocation(t *testing.T) {
	cases := []struct {
		source string
		line   int
	}{
		{"let values = [1]\nlet push = list.append(values, _)\nfor n in values\n    push(2)\n", 4},
		{"let values = [1]\nlet append: (List<Int>, Int) -> Unit = list.append(_, _)\nlet push = append(values, _)\nfor n in values\n    push(2)\n", 5},
	}
	for _, tc := range cases {
		err := Run("test.zhex", tc.source, strings.NewReader(""), &bytes.Buffer{})
		if d, ok := err.(*diagnostic); !ok || d.code != CodeCollectionIterating || d.pos.line != tc.line {
			t.Errorf("expected call site line %d, got %v", tc.line, err)
		}
	}
}

func TestPartialTraceAndBodyFailuresKeepCorrectLocations(t *testing.T) {
	files := mapResolver(map[string]string{"math.zhex": "word divide(n: Int) -> Int\n    1 / n\n"})
	program, err := CompileModules("main.zhex", "use math\nlet f = math.divide(_)\nf(0)\n", files)
	if err != nil {
		t.Fatal(err)
	}
	var out, trace bytes.Buffer
	err = program.RunWithTrace(strings.NewReader(""), &out, &trace)
	if d, ok := err.(*diagnostic); !ok || d.code != CodeDivisionZero || d.pos.file != "math.zhex" || d.pos.line != 2 {
		t.Fatalf("wrong body error: %v", err)
	}
	if !strings.Contains(trace.String(), `"event":"word-call","word":"math.divide","file":"main.zhex","line":3`) {
		t.Fatalf("wrong call trace: %s", trace.String())
	}
}

func TestMapBuiltinPartialsSpecializeFromCapturesAndUses(t *testing.T) {
	source := `let put-ada = map.put(_, "Ada", 5)
let contains-ada = map.contains(_, "Ada")
let scores: Map<String, Int> = {}
let same = put-ada(scores)
print(contains-ada(same))
print(map.length(scores))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "true\n1\n" {
		t.Fatal(out.String())
	}
}

func TestRecursivePartialDispatchRespectsDepthLimit(t *testing.T) {
	source := `word recur(callbacks: List<(Int) -> Int>, n: Int) -> Int
    if n == 0
        0
    else
        let next = callbacks[0]
        next(n - 1)
let callbacks: List<(Int) -> Int> = []
let f = recur(callbacks, _)
list.append(callbacks, f)
print(f(300))
`
	err := Run("test.zhex", source, strings.NewReader(""), &bytes.Buffer{})
	if d, ok := err.(*diagnostic); !ok || d.code != CodeDepthLimit {
		t.Fatalf("expected depth limit, got %v", err)
	}
}
