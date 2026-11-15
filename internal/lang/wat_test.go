package lang

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// watOf compiles src through CompileWat; fence failures fail the test with
// the diagnostic rendered.
func watOf(t *testing.T, src string) string {
	t.Helper()
	wat, err := CompileWat("repl", src, nil)
	if err != nil {
		t.Fatalf("CompileWat(%q): %v", src, err)
	}
	return wat
}

func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s is not installed; skipping", name)
	}
}

// itoa keeps constant fail-code assertions literal-free in the table tests.
func itoa(n int) string { return strconv.Itoa(n) }

// ---------------------------------------------------------------- literals

func TestWatLiteralsAndArithmetic(t *testing.T) {
	wat := watOf(t, `print(1 + 2 * 3)`)
	for _, want := range []string{
		"i64.const 1", "i64.const 2", "i64.const 3",
		"call $zhex_mul", "call $zhex_add", // precedence: (1) then (2*3)
		"call $zhex_print_int",
		`(export "memory" (memory $zhex_mem))`,
		`(export "zhex_main" (func $zhex_main))`,
		`(export "zhex_last_error" (func $zhex_last_error))`,
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT", want)
		}
	}
}

func TestWatDivisionAndRemainderAreNativeOrChecked(t *testing.T) {
	wat := watOf(t, `
let n = 7
print(7 / 2)
print(7 % 3)
print(-n * 3)
`)
	for _, want := range []string{"call $zhex_div", "i64.rem_s", "call $zhex_neg", "call $zhex_mul"} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestWatStringConcatAndComparison(t *testing.T) {
	wat := watOf(t, `
let a = "foo"
let b = a + "bar"
print(b)
print(a == "x")
print(a != "x")
`)
	for _, want := range []string{
		"call $zhex_str_concat", "call $zhex_print_str",
		"call $zhex_str_eq", "i32.eqz", // != negates the str_eq result
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// ------------------------------------------------------------------- words

func TestWatWordFunctionShape(t *testing.T) {
	wat := watOf(t, `
word fib(n: Int) -> Int
    if n < 2
        n
    else
        fib(n - 1) + fib(n - 2)

print(fib(20))
`)
	for _, want := range []string{
		"(func $zw_fib (param $p0_n i64) (result i64)",
		"call $zhex_enter", // first instruction: depth check
		"call $zhex_leave", // single epilogue
		"call $zw_fib",     // recursion is a direct call
		"i64.lt_s",         // n < 2
		"if (result i64)",  // value-form branch
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Index(wat, "call $zhex_enter") > strings.Index(wat, "block $zhex_ret") {
		t.Errorf("enter must precede the return block")
	}
}

func TestWatWordUnitsAndVoidResults(t *testing.T) {
	wat := watOf(t, `
word announce()
    print("hi")

announce()
`)
	if !strings.Contains(wat, "(func $zw_announce\n") {
		t.Errorf("void word should have no result")
	}
	if !strings.Contains(wat, "call $zw_announce") {
		t.Errorf("main must call the word")
	}
}

// ------------------------------------------------------- bindings, branches

func TestWatBindingsBranchesAndShortCircuit(t *testing.T) {
	wat := watOf(t, `
var total = 0
for i in range(1, 11)
    total = total + i
let size = if total > 100
    "huge"
elif total > 50
    "large"
else
    "small"
print(size)
print(total > 50 and total < 60)
print(total == 54 or total == 1)
print(not (total == 55))
`)
	for _, want := range []string{
		"(local $l1_total i64)", "local.set $l1_total", // var binding and assignment share the local
		"local.tee $t_bool", "if (result i32)", // and/or short-circuit
		"i32.eqz",                                           // not
		"if (result (ref $zhex_string))",                    // value-form if/elif/else
		"i64.lt_s", "block $exit", "loop $loop", "br $loop", // for range
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// --------------------------------------------------------------- collections

func TestWatListOperations(t *testing.T) {
	wat := watOf(t, `
let names: List<String> = list.new()
list.append(names, "Ada")
print(list.length(names))
print(names[0])
names[0] = "Augusta"
let direct = ["a", "b"]
print(direct[1])
`)
	for _, want := range []string{
		"call $zhex_list_new_ref",
		"call $zhex_list_append_ref",
		"call $zhex_list_length_ref",
		"call $zhex_list_get_ref",
		"call $zhex_list_set_ref",
		"array.new_fixed $zhex_arr_ref 2", // list literal items
		"struct.new $zhex_list_ref",
		"ref.cast (ref $zhex_string)", // element read casts back from eq
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestWatForCollectionTraversalSemantics(t *testing.T) {
	wat := watOf(t, `
let xs: List<Int> = [1, 2]
for x in xs
    print(x)
let ys: List<String> = ["a"]
for i, y in ys
    print(y)
`)
	for _, want := range []string{
		"struct.set $zhex_list_i64 $active", // increment before, decrement after
		"struct.set $zhex_list_ref $active",
		"i64.extend_i32_u", // dual binding: index as Int
		"array.len",
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
	// each traversal resets the counter it raised
	if count := strings.Count(wat, "struct.set $zhex_list_i64 $active"); count != 2 {
		t.Errorf("list_i64 active writes = %d, want 2 (inc and dec)", count)
	}
}

func TestWatArrayOperations(t *testing.T) {
	wat := watOf(t, `
let grid = array.new([2, 3], 0)
print(array.length(grid))
grid[1, 2] = 7
print(grid[1, 2])
for cell, value in grid
    print(value)
`)
	for _, want := range []string{
		"call $zhex_array_new_i64",
		"call $zhex_array_length_i64",
		"call $zhex_array_set_i64",
		"call $zhex_array_get_i64",
		"array.new_fixed $zhex_arr_i64 2", // coordinate array
		"array.new_default $zhex_arr_i64", // dual-binding coords list
		"struct.new $zhex_list_i64",       // coords wrapped as List<Int>
		"i64.rem_s",                       // coords = n % shape[axis]
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// -------------------------------------------------------- choices and records

func TestWatMatchOnUserChoice(t *testing.T) {
	wat := watOf(t, `
type Status = choice
    Pending
    Failed(message: String)

word describe(status: Status) -> String
    match status
        case Status.Pending
            "pending"
        case Status.Failed(message)
            "failed: " + message

print(describe(Status.Failed("down")))
print(describe(Status.Pending))
`)
	for _, want := range []string{
		"(type $zt_Status_Pending (struct (field $v i32)))",
		"(type $zt_Status_Failed (struct (field $d0 i64) (field $v i32) (field $f0 (ref null $zhex_string))))", // padding keeps same-shape siblings distinct
		"ref.test (ref $zt_Status_Pending)",
		"ref.cast (ref $zt_Status_Failed)",
		"struct.new $zt_Status_Failed",
		"struct.new $zt_Status_Pending",
		"struct.get $zt_Status_Failed $f0", // payload names skip the padding fields
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestWatMatchOnOption(t *testing.T) {
	wat := watOf(t, `
word opt-value(opt: Option<Int>) -> Int
    match opt
        case Option.Some(value)
            value
        case Option.None
            0

print(opt-value(Option.Some(7)))
print(opt-value(Option.None))
`)
	for _, want := range []string{
		"struct.new $zhex_some_i64",     // Some(7)
		"struct.new $zhex_none",         // None
		"ref.test (ref $zhex_some_i64)", // None is the final arm: the plain else
		"struct.get $zhex_some_i64 $value",
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestWatRecords(t *testing.T) {
	wat := watOf(t, `
type Customer = record
    name: String
    age: Int

let ada = Customer("Ada", 36)
let named = Customer(age: 37, name: "Bob")
print(ada.name + named.name)
`)
	for _, want := range []string{
		"(type $zt_Customer (struct (field $f0 (ref null $zhex_string)) (field $f1 i64)))",
		// storage form is nullable (fields, locals, params); reads assert non-null,
		"struct.new $zt_Customer",
		"struct.get $zt_Customer $f0",
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// ------------------------------------------------------------------ results

func TestWatResultWrapUnwrapAndCatch(t *testing.T) {
	wat := watOf(t, `
word big() -> Result<Int, Error>
    return ok(2147483648)

print(big() try)

word fallback() -> Int
    big() catch e
        0

print(fallback())
`)
	for _, want := range []string{
		"struct.new $zhex_some_i64", // amended §2: Int payloads box as Some, not i31
		"i32.const 0",
		"struct.new $zhex_result",
		"struct.get $zhex_result $tag",
		"struct.get $zhex_result $payload",
		"i32.const 11", // top-level try failure exits unhandled
		"call $zhex_fail",
		"local.set $r", // propagating failure exits through the epilogue
		"br $zhex_ret",
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(wat, "ref.i31\n") && strings.Contains(wat, "ok") {
		// Bool payloads use ref.i31; Int payloads must not (31-bit loss)
		if strings.Count(wat, "ref.i31") > 0 && !strings.Contains(wat, "struct.new $zhex_some_i64") {
			t.Errorf("Int ok() payload must box as $zhex_some_i64")
		}
	}
}

func TestWatBoolResultPayloadUsesI31(t *testing.T) {
	wat := watOf(t, `
word flag() -> Result<Bool, Error>
    return ok(true)

print(flag() try)
`)
	if !strings.Contains(wat, "ref.i31") {
		t.Errorf("Bool ok() payload boxes with ref.i31")
	}
	if !strings.Contains(wat, "call $zhex_unbox_i32") {
		t.Errorf("Bool try payload unboxes via $zhex_unbox_i32")
	}
}

// M2 §2d amended M1: eval's catch binds value{fault: v.result.err} — the err
// payload — not the Result struct, so the handler's binding holds the opaque
// payload (an eq reference nothing in this build can read).
func TestWatCatchBindsErrPayload(t *testing.T) {
	wat := watOf(t, `
word give() -> Result<Int, Error>
    ok(4)

word fallback() -> Int
    give() catch e
        0

print(fallback())
`)
	if !regexp.MustCompile(`local\.get \$t_res\n\s*struct\.get ` + regexp.QuoteMeta(watTypeResult) + ` \$payload\n\s*local\.set \$l\d+_e`).MatchString(wat) {
		t.Errorf("catch must bind the payload (struct.get $payload feeding the binding local), got:\n%s", wat)
	}
	if !regexp.MustCompile(`\(local \$l\d+_e \(ref null eq\)\)`).MatchString(wat) {
		t.Errorf("the caught error is an opaque eq reference, got:\n%s", wat)
	}
}

// -------------------------------------------------------------------- flows

func TestWatFlowSteps(t *testing.T) {
	wat := watOf(t, `
word double(n: Int) -> Int
    n * 2

flow double(5)
    double
    print
`)
	for _, want := range []string{
		"call $zw_double",      // seed
		"local.set $t_c1_0",    // cursor local for the seed value
		"local.get $t_c1_0",    // cursor feeds the next step's first argument
		"call $zhex_print_int", // bare print step
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// ------------------------------------------------------------------- fences

func TestWatFencesOutOfSubsetConstructs(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"checked-append", `
let xs: List<Int> = list.new()
list.append(xs, 1) try
`, "checked list.append is not available in this build"},
		{"checked-index", `
let xs: List<Int> = [1]
print(xs[9] try)
`, "checked index reads is not available in this build"},
		{"word-defaults", `
word greet(name: String = "Ada") -> String
    name

print(greet())
`, "word parameter defaults is not available in this build"},
		{"record-defaults", `
type R = record
    a: Int
    b: Int = 2

let r = R(1)
print(r.a)
`, "record field defaults is not available in this build"},
		{"re-partial", `
word add(a: Int, b: Int) -> Int
    a + b

let once = add(_, 1)
let twice = once(_)
print(1)
`, "re-partial application is not available in this build"},
		{"unit-result-closure", `
word side(n: Int)
    print(n)

let run = side(_)
run(1)
`, "closures with Unit results is not available in this build"},
		{"flow-function-step", `
word double(n: Int) -> Int
    n * 2

let f = double(_)
flow 5
    f
    print
`, "function values is not available in this build"},
		{"file-stream", `
for line in file.stream("nope.txt")
    print(line)
`, "file.stream is not available in this build"},
		{"generic-choice", `
type Pair<T> = choice
    Left(a: T)
    Right(b: T)

print(1)
`, "generic choice Pair is not available in this build"},
		{"file-read", `let t = file.read("a.txt") try`, "file.read is not available in this build"},
		{"file-path-words", `print(file.join("a", "b"))`, "file.join is not available in this build"},
		{"read-line", `let line = read-line("> ") try`, "read-line is not available in this build"},
		{"buffer", `let b = buffer.new()`, "buffer.new is not available in this build"},
		{"json-parse", `let j = json.parse("{}") try`, "json.parse is not available in this build"},
		{"env-get", `match env.get("HOME")
    case Option.Some(v)
        print(v)
    case Option.None
        print("")
`, "env.get is not available in this build"},
		{"temporal", `print(duration.to-string(duration.new(1)))`, "duration.to-string is not available in this build"},
		{"console-arg", `let a = console.arg(0) try`, "console.arg is not available in this build"},
		{"error-new", `let e = error.new("x", "boom")`, "error.new is not available in this build"},
		{"array-shape", `
let grid = array.new([2, 2], 0)
print(list.length(array.shape(grid)))
`, "array.shape is not available in this build"},
		{"checked-array-new", `
let grid = (array.new([2, 2], 0)) try
print(1)
`, "checked array.new is not available in this build"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CompileWat("repl", tc.src, nil)
			if err == nil {
				t.Fatalf("expected a fence diagnostic")
			}
			d, ok := err.(*diagnostic)
			if !ok || d.code != CodeNotAvailableInBuild {
				t.Fatalf("got %v, want code %s", err, CodeNotAvailableInBuild)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("message %q does not contain %q", err.Error(), tc.want)
			}
			if !strings.Contains(err.Error(), "hint: this construct is not compiled by zhex compile yet; run it with zhex run instead") {
				t.Fatalf("missing pinned hint: %v", err)
			}
			if d.pos.line == 0 {
				t.Fatalf("fence diagnostic must carry a position")
			}
		})
	}
}

func TestWatCheckerErrorsPassThrough(t *testing.T) {
	_, err := CompileWat("repl", `print(1 + true)`, nil)
	if err == nil {
		t.Fatal("expected checker error")
	}
	if d, ok := err.(*diagnostic); !ok || d.code == CodeNotAvailableInBuild {
		t.Fatalf("checker errors must return unchanged, got %v", err)
	}
}

func TestWatEmissionIsDeterministic(t *testing.T) {
	src := `
word fib(n: Int) -> Int
    if n < 2
        n
    else
        fib(n - 1) + fib(n - 2)

print(fib(10))
`
	first := watOf(t, src)
	for i := 0; i < 2; i++ {
		if again := watOf(t, src); again != first {
			t.Fatalf("emission not deterministic on run %d", i+1)
		}
	}
}

// ------------------------------------------------------------------ modules

func TestWatModuleWordsEmitQualified(t *testing.T) {
	wat, err := CompileWat("main.zhex", "use math\nprint(math.gcd(48, 18))\n", mapResolver(map[string]string{"math.zhex": mathModule}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"(func $zw_math_qgcd (param $p0_a i64) (param $p1_b i64) (result i64)",
		"call $zw_math_qgcd",
		"i64.rem_s", // a % b inside the module word
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestWatModuleRecordsEmitWhenUsed(t *testing.T) {
	mod := `
type Point = record
    x: Int
    y: Int

word origin() -> Point
    Point(0, 0)
`
	wat, err := CompileWat("main.zhex", "use geom\nlet p = geom.origin()\nprint(p.x)\n", mapResolver(map[string]string{"geom.zhex": mod}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"(type $zt_geom_qPoint (struct (field $f0 i64) (field $f1 i64)))",
		"struct.new $zt_geom_qPoint",
		"struct.get $zt_geom_qPoint $f0",
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// ------------------------------------------------------------- edge cases

func TestWatUnitParamsAndResults(t *testing.T) {
	wat := watOf(t, `
word run(u: Unit) -> Unit
    print("u")

run(print("side"))
`)
	if !strings.Contains(wat, "(func $zw_run (param $p0_u i32)") {
		t.Errorf("Unit param needs the i32 placeholder: %s", wat)
	}
	if strings.Contains(wat, "(result") && strings.Contains(wat, "(func $zw_run") {
		start := strings.Index(wat, "(func $zw_run")
		if end := strings.Index(wat[start:], "\n"); end > 0 && strings.Contains(wat[start:start+end], "(result") {
			t.Errorf("Unit-result word must have no wasm result")
		}
	}
	if !strings.Contains(wat, "i32.const 0") {
		t.Errorf("Unit argument pushes the placeholder")
	}
}

func TestWatReturnInsideTraversalResetsActive(t *testing.T) {
	wat := watOf(t, `
word first-positive(xs: List<Int>) -> Result<Int, Error>
    for x in xs
        if x > 0
            return ok(x)
    ok(0)

print(first-positive([-1, 5, 7]) try)
`)
	if !strings.Contains(wat, "struct.get $zhex_list_i64 $active") {
		t.Errorf("return path must reset the traversal counter")
	}
	// the cleanup runs before the br: active read precedes br $zhex_ret after it
	retAt := strings.Index(wat, "br $zhex_ret")
	activeAt := strings.LastIndex(wat[:retAt], "struct.set $zhex_list_i64 $active")
	if activeAt < 0 || activeAt > retAt {
		t.Errorf("return must decrement $active before branching out")
	}
}

func TestWatNestedChoiceAndRecordFieldOfList(t *testing.T) {
	wat := watOf(t, `
type Shape = choice
    Circle(r: Int)
    Rect(w: Int, h: Int)

type Drawing = record
    parts: List<Shape>

word area(s: Shape) -> Int
    match s
        case Shape.Circle(r)
            3 * r * r
        case Shape.Rect(w, h)
            w * h

let d = Drawing([Shape.Circle(2), Shape.Rect(3, 4)])
print(area(d.parts[0]) + area(d.parts[1]))
`)
	for _, want := range []string{
		"(type $zt_Drawing (struct (field $f0 (ref null $zhex_list_ref))))",
		"call $zhex_list_get_ref",
		"ref.cast (ref $zt_Shape_Circle)",
		"struct.get $zt_Shape_Circle $f0",
		"struct.get $zt_Shape_Rect $f0",
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestWatInferredResultWordWrapsSuccess(t *testing.T) {
	wat := watOf(t, `
word give() -> Result<Int, Error>
    ok(4)

word take()
    give() try

print(take() try)
`)
	for _, want := range []string{
		"(func $zw_take (result (ref $zhex_result))", // inferred Result signature
		"struct.new $zhex_some_i64",                  // success wrapped once at the boundary
		"local.set $r",
		"br $zhex_ret", // the propagating try exits through the epilogue on err
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// ---------------------------------------------------------------- text suite

func TestWatTextWordsEmitPreludeCalls(t *testing.T) {
	wat := watOf(t, `
let t = "héllo"
print(text.reverse(t))
print(text.length(t))
print(text.slice(t, 1, 3))
print(text.index-of(t, "l"))
print(text.contains(t, "ell"))
print(text.upper(t))
print(text.lower(t))
print(text.trim(t))
let parts = text.split(t, ",")
print(text.join(parts, "-"))
print(text.replace(t, "l", "L"))
print(text.starts-with(t, "h"))
print(text.ends-with(t, "o"))
print(text.line-count(t))
print(text.word-count(t))
`)
	for _, want := range []string{
		"call " + watTextReverse,
		"call " + watTextSlice,
		"call " + watTextIndexOf,
		"call " + watTextContains,
		"call " + watTextUpper,
		"call " + watTextLower,
		"call " + watTextTrim,
		"call " + watTextSplit,
		"call " + watTextJoin,
		"call " + watTextReplace,
		"call " + watTextStartsWith,
		"call " + watTextEndsWith,
		"call " + watTextLineCount,
		"call " + watTextWordCount,
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
	// text.length is inlined: code-point array.len widened to Int.
	if !strings.Contains(wat, "array.len") || !strings.Contains(wat, "i64.extend_i32_u") {
		t.Errorf("text.length must inline array.len + i64.extend_i32_u")
	}
	if strings.Contains(wat, "call $zhex_text_length") {
		t.Errorf("text.length must not call a prelude helper")
	}
}

// text.to-int is a fallible call returning the Result struct; the emitted
// call feeds the standard try/catch machinery.
func TestWatTextToIntEmitsResultCall(t *testing.T) {
	wat := watOf(t, `
let n = text.to-int("12") try
print(n)
`)
	if !strings.Contains(wat, "call "+watTextToInt) {
		t.Errorf("missing %q", watTextToInt)
	}
	if !strings.Contains(wat, "struct.get "+watTypeResult+" $tag") {
		t.Errorf("try must branch on the result tag")
	}
}

// An unhandled failure at top level stages the err message for the runner
// (fail 12) instead of the generic fail 11 (DESIGN-M2 §2c/§2d).
func TestWatTopLevelTryFailureStagesMessage(t *testing.T) {
	wat := watOf(t, `
print("before")
let n = text.to-int("x") try
print(n)
`)
	for _, want := range []string{
		"ref.test (ref " + watTypeErr + ")",
		"ref.cast (ref " + watTypeErr + ")",
		"struct.get " + watTypeErr + " " + watErrFieldMessage,
		"i32.const " + itoa(watFailTextToInt),
		"call " + watFailMsg,
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// --------------------------------------------------------------------- maps

func TestWatMapLiteralAndWords(t *testing.T) {
	wat := watOf(t, `
let scores: Map<String, Int> = {"Ada": 1, "Bob": 2}
scores["Ada"] = 4
map.remove(scores, "Bob")
let updated = map.put(scores, "Cy", 3)
print(map.length(updated))
print(map.contains(scores, "Cy"))
let base: Map<String, Int> = {}
let merged = map.into(updated, base)
print(map.length(merged))
`)
	for _, want := range []string{
		"call " + watMapNew(watMapKeyStr, watSpecI64),
		"call " + watMapPut(watMapKeyStr, watSpecI64), // literal pairs + explicit put + index write
		"call " + watMapRemove(watMapKeyStr, watSpecI64),
		"call " + watMapLength(watMapKeyStr, watSpecI64),
		"call " + watMapContains(watMapKeyStr, watSpecI64),
		"call " + watMapInto(watMapKeyStr, watSpecI64),
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
	// Two literal pairs, the explicit map.put, and the index write: four put
	// calls; the empty literal emits none.
	if got := strings.Count(wat, "call "+watMapPut(watMapKeyStr, watSpecI64)); got != 4 {
		t.Errorf("put calls = %d, want 4 (two literal pairs + map.put + index write)", got)
	}
}

func TestWatMapIndexReadEmitsOptionLookup(t *testing.T) {
	wat := watOf(t, `
let scores: Map<String, Int> = {"Ada": 1}
match scores["Ada"]
    case Option.Some(score)
        print(score)
    case Option.None
        print(-1)
`)
	if !strings.Contains(wat, "call "+watMapLookup(watMapKeyStr, watSpecI64)) {
		t.Errorf("map index read must call the lookup helper")
	}
	if !strings.Contains(wat, "ref.test (ref $zhex_some_i64)") {
		t.Errorf("the lookup result must flow into the Option match")
	}
}

func TestWatMapIndexWriteDropsPutResult(t *testing.T) {
	wat := watOf(t, `
let scores: Map<String, Int> = {"Ada": 1}
scores["Ada"] = 4
`)
	if !strings.Contains(wat, "call "+watMapPut(watMapKeyStr, watSpecI64)) {
		t.Errorf("map write must call put (guard first, receiver/key/value order)")
	}
	putAt := strings.Index(wat, "call "+watMapPut(watMapKeyStr, watSpecI64))
	if !regexp.MustCompile(`(?m)^\s*drop$`).MatchString(wat[putAt:]) {
		t.Errorf("the write is Unit: put's pass-through map ref must be dropped")
	}
}

// Single binding iterates values; dual binding binds key first, value second
// (DESIGN-M2 §3d) — easy to get backwards.
func TestWatMapIterationBindings(t *testing.T) {
	wat := watOf(t, `
let scores: Map<String, Int> = {"Ada": 1}
for score in scores
    print(score)
for name, score in scores
    print(name + "=" + int.to-string(score))
`)
	for _, want := range []string{
		"struct.get " + watMapType(watMapKeyStr, watSpecI64) + " $keys",
		"struct.get " + watMapType(watMapKeyStr, watSpecI64) + " $vals",
		"array.get " + watTypeArrStr, // dual binding reads the key array
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
	// Two traversals: increment and decrement each, nothing else touches it.
	if got := strings.Count(wat, "struct.set "+watMapType(watMapKeyStr, watSpecI64)+" $active"); got != 4 {
		t.Errorf("map active writes = %d, want 4", got)
	}
}

func TestWatMapIntAndBoolKeySpecializations(t *testing.T) {
	wat := watOf(t, `
let by-id: Map<Int, String> = {1: "a"}
let flags: Map<Bool, Int> = {true: 1}
print(map.length(by-id))
print(map.length(flags))
match by-id[1]
    case Option.Some(name)
        print(name)
    case Option.None
        print("none")
`)
	for _, want := range []string{
		"call " + watMapNew(watMapKeyI64, watSpecRef),
		"call " + watMapNew(watMapKeyI32, watSpecI64),
		"call " + watMapLookup(watMapKeyI64, watSpecRef),
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// ---------------------------------------------------------------- closures

func TestWatPartialCreationEmitsClosure(t *testing.T) {
	wat := watOf(t, `
word double(n: Int) -> Int
    n * 2

let numbers = [1, 2]
let doubled = list.map(numbers, double(_))
print(doubled[0])
`)
	for _, want := range []string{
		"ref.null eq",                          // the hole's env slot
		"array.new_fixed " + watTypeArrRef + " 1", // env of target arity
		"ref.func $zcthunk",                    // site-local thunk
		"struct.new " + watTypeClosure,
		"call " + watListMap(watSpecI64, watSpecI64),
		"(func $zcthunk",                       // the thunk is a plain function
		"(param $clo (ref " + watTypeClosure + "))",
		"(param $a0 i64)",                      // the hole is the callback param
		"(result i64)",
		"call $zw_double",                      // direct call, no enter of its own
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
	// Thunks are not words: no depth check of their own.
	if thunks := strings.Count(wat, "(func $zcthunk"); thunks != 1 {
		t.Errorf("thunk count = %d, want 1", thunks)
	}
}

func TestWatThunkCapturesEnvAndFullyApplies(t *testing.T) {
	wat := watOf(t, `
word add(a: Int, b: Int) -> Int
    a + b

let inc = add(_, 1)
print(inc(5))
`)
	for _, want := range []string{
		// capture: env holds [null, boxed 1]; Int boxes as $zhex_some_i64
		"array.new_fixed " + watTypeArrRef + " 2",
		"struct.new " + watSomeType(watSpecI64),
		// thunk: hole $a0 plus the captured env slot unboxed, then the call
		"array.get " + watTypeArrRef,
		"struct.get " + watSomeType(watSpecI64) + " $value",
		"call $zw_add",
		// full application of the bound closure
		"struct.get " + watTypeClosure + " $fn",
		"ref.cast (ref " + watCloType([]watSpec{watSpecI64}, watSpecI64) + ")",
		"call_ref " + watCloType([]watSpec{watSpecI64}, watSpecI64),
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestWatCallbackWordsEmitHelpers(t *testing.T) {
	wat := watOf(t, `
word big(n: Int) -> Bool
    n > 3

let numbers = [1, 2, 3, 4]
let evens = list.filter(numbers, big(_))
let picked = list.select(numbers, big(_))
match list.find(numbers, big(_))
    case Option.Some(n)
        print(n)
    case Option.None
        print(-1)
print(list.any(numbers, big(_)))
print(list.all(numbers, big(_)))
`)
	for _, want := range []string{
		"call " + watListFilter(watSpecI64),
		"call " + watListSelect(watSpecI64),
		"call " + watListFind(watSpecI64),
		"call " + watListAny(watSpecI64),
		"call " + watListAll(watSpecI64),
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestWatSortAndSortByEmitHelpers(t *testing.T) {
	wat := watOf(t, `
type Entry = record
    name: String
    score: Int

word entry-score(e: Entry) -> Int
    e.score

let sorted = list.sort([3, -1, 10])
let words = list.sort(["żebra", "apple"])
let ranked = list.sort-by([Entry("a", 2), Entry("b", 1)], entry-score(_))
let by-length = list.sort-by(["dddd", "a"], text.length(_))
let top = ranked[0]
print(sorted[0])
print(words[0])
print(top.score)
print(by-length[0])
`)
	for _, want := range []string{
		"call " + watListSort(watSpecI64),
		"call " + watListSort(watSpecRef),
		"call " + watListSortBy(watSpecRef, watSpecI64),
		"call " + watListSortBy(watSpecRef, watSpecI64), // text.length keys are Int
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// Builtin partials emit thunks that call the prelude helper directly (or
// inline text.length); only supported builtins may be partialized.
func TestWatBuiltinPartialThunks(t *testing.T) {
	wat := watOf(t, `
let trimmed = list.map(["  Ada "], text.trim(_))
let guests = list.filter(["Alice"], text.contains(_, "Al"))
print(trimmed[0])
print(guests[0])
`)
	for _, want := range []string{
		"call " + watTextTrim,
		"call " + watTextContains,
		"ref.func $zcthunk",
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
	// the contains thunk reads its captured "Al" from the env array
	if !strings.Contains(wat, "array.get "+watTypeArrRef) {
		t.Errorf("captured builtin arguments unbox from the closure env")
	}
}

func TestWatArraySelectEmitsHelper(t *testing.T) {
	wat := watOf(t, `
word big-row(row: List<Int>) -> Bool
    list.length(row) > 1

let grid = array.new([2, 2], 0)
let kept = array.select(grid, big-row(_))
print(array.length(kept))
`)
	if !strings.Contains(wat, "call "+watArraySelect(watSpecI64)) {
		t.Errorf("missing %q", watArraySelect(watSpecI64))
	}
	// The row predicate closure is created inline and its thunk takes the
	// row as an eq-level ref parameter (hyphen mangles to _h).
	if !strings.Contains(wat, "(func $zcthunk") || !strings.Contains(wat, "call $zw_big_hrow") {
		t.Errorf("the site-local thunk must call the predicate word directly")
	}
}

// A let-bound closure is an ordinary value: stored once, passed by name.
func TestWatLetBoundClosurePassedByName(t *testing.T) {
	wat := watOf(t, `
word short(row: List<Int>) -> Bool
    list.length(row) < 2

let keeper = short(_)
let kept = list.filter([[1], [2, 3]], keeper)
print(list.length(kept))
`)
	if !strings.Contains(wat, "(local $l1_keeper (ref null "+watTypeClosure+"))") {
		t.Errorf("a function-typed binding's local is the closure struct")
	}
	if !strings.Contains(wat, "call "+watListFilter(watSpecRef)) {
		t.Errorf("missing %q", watListFilter(watSpecRef))
	}
}

// -------------------------------------------------------------------- while

func TestWatWhileLoop(t *testing.T) {
	wat := watOf(t, `
var n = 0
while n < 3
    n = n + 1
print(n)
`)
	for _, want := range []string{
		"local.set $t_while", // condition cached in a local
		"block $exit",
		"loop $loop",
		"i32.eqz",
		"br_if $exit",
		"br $loop",
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT", want)
		}
	}
	// The condition is evaluated before the first pass and again after every
	// pass: its result is stored into the loop's condition local exactly
	// twice (the prelude also contains lt_s, so count the store).
	if got := strings.Count(wat, "local.set $t_while"); got != 2 {
		t.Errorf("condition stores = %d, want 2", got)
	}
}

func TestWatWhileInitiallyFalseAndNested(t *testing.T) {
	wat := watOf(t, `
var total = 0
var i = 0
while i < 0
    total = total + 1
while i < 2
    var j = 0
    while j < 2
        total = total + 1
        j = j + 1
    i = i + 1
print(total)
`)
	// Nested loops get distinct labels: loop1 and loop2 must each pair with
	// their own exit block.
	for _, want := range []string{"loop $loop1", "loop $loop2", "br_if $exit2"} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// While must not consume recursion-depth budget: the interpreter's while
// touches neither r.depth nor any counter, so a word containing one loop has
// exactly the single depth check of its prologue.
func TestWatWhileConsumesNoDepthBudget(t *testing.T) {
	wat := watOf(t, `
word count-to(limit: Int) -> Int
    var n = 0
    while n < limit
        n = n + 1
    n

print(count-to(5000))
`)
	if got := strings.Count(wat, "call "+watEnter); got != 1 {
		t.Errorf("enter calls = %d, want 1 (while must not consume depth budget)", got)
	}
	if got := strings.Count(wat, "call "+watLeave); got != 1 {
		t.Errorf("leave calls = %d, want 1", got)
	}
}

// ------------------------------------------------- assemble-and-run smoke

// assembleAndRun compiles WAT to wasm and executes zhex_main under Node,
// returning stdout. It is the emitter's end-to-end witness; differential
// fixtures live with Builder T.
func assembleAndRun(t *testing.T, wat string) string {
	t.Helper()
	requireTool(t, "wasm-tools")
	requireTool(t, "node")
	// The M2 emitter references prelude symbols Builder R ships in
	// watprelude_m2.go; until that prelude lands in emit(), a module that
	// references $zhex_err cannot assemble. The skip heals itself once the
	// prelude defines the type.
	if strings.Contains(wat, watTypeErr) && !strings.Contains(wat, "(type "+watTypeErr+" ") && !strings.Contains(wat, "(type "+watTypeErr+")") {
		t.Skip("watprelude_m2 (Builder R) has not landed; $zhex_err is undefined")
	}
	dir := t.TempDir()
	watPath := filepath.Join(dir, "smoke.wat")
	wasmPath := filepath.Join(dir, "smoke.wasm")
	if err := os.WriteFile(watPath, []byte(wat), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("wasm-tools", "parse", watPath, "-o", wasmPath).CombinedOutput(); err != nil {
		t.Fatalf("wasm-tools parse: %v\n%s", err, out)
	}
	script := `
const fs = require("fs");
(async () => {
  const bytes = fs.readFileSync(process.argv[2]);
  let out = "";
  const { instance } = await WebAssembly.instantiate(bytes, {
    env: { print: (ptr, len) => {
      const mem = new Uint8Array(instance.exports.memory.buffer, ptr, len);
      out += new TextDecoder().decode(mem) + "\n";
    } },
  });
  instance.exports.zhex_main();
  process.stdout.write(out);
})().catch((e) => { console.error(String(e)); process.exit(1); });
`
	scriptPath := filepath.Join(dir, "run.js")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	cmd := exec.Command("node", scriptPath, wasmPath)
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("node run: %v\nstderr: %s", err, stderr.String())
	}
	return out.String()
}

// TestWatWhileRunsManyIterations proves the depth counter stays untouched by
// loop iteration: 5000 passes would trip the 256 limit if while consumed
// budget (the ticket's collatz witness is Builder T's fixture).
func TestWatWhileRunsManyIterations(t *testing.T) {
	wat := watOf(t, `
var n = 0
while n < 5000
    n = n + 1
print(n)
`)
	if got, want := assembleAndRun(t, wat), "5000\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

// TestWatAssembleAndRunSmoke compiles, assembles with wasm-tools, and
// executes under Node — including the amended Int Result payload round trip
// (ok(2^31) try must print 2147483648, which ref.i31 boxing would corrupt).
func TestWatAssembleAndRunSmoke(t *testing.T) {
	wat := watOf(t, `
word big() -> Result<Int, Error>
    return ok(2147483648)

print(1 + 1)
print(big() try)
`)
	if got, want := assembleAndRun(t, wat), "2\n2147483648\n"; got != want {
		t.Fatalf("stdout = %q, want %q (a 31-bit i31 box would corrupt the second line)", got, want)
	}
}

// Two all-named constructions in one word must not share temporaries: the
// ctor key includes a per-construction id, otherwise the second struct.new
// reads the first construction's locals at the wrong types (review S2).
func TestWatNamedConstructorTemporariesAreDistinct(t *testing.T) {
	wat := watOf(t, `
type Point = record
    x: Int
    y: Int

type Label = record
    text: String
    weight: Int

let p = Point(y: 3, x: 1)
let l = Label(weight: 2, text: "hi")
print(p.x + p.y + l.weight)
print(l.text)
`)
	var ctorIDs []string
	for _, m := range regexp.MustCompile(`local\.set \$t_ctor(\d+)_0`).FindAllStringSubmatch(wat, -1) {
		ctorIDs = append(ctorIDs, m[1])
	}
	seen := map[string]bool{}
	for _, id := range ctorIDs {
		if seen[id] {
			t.Fatalf("constructor temporary id %s reused across constructions", id)
		}
		seen[id] = true
	}
	if len(seen) != 2 {
		t.Fatalf("got constructor ids %v, want two distinct", ctorIDs)
	}
	// Each construction stores its field temporaries at the right types.
	if strings.Count(wat, "local.set $t_ctor") != 4 {
		t.Fatalf("expected four ctor local.set stores, got:\n%s", wat)
	}
	for _, typ := range []string{"i64", "(ref null $zhex_string)"} {
		if !regexp.MustCompile(`\$t_ctor\d+_\d ` + regexp.QuoteMeta(typ) + `\)`).MatchString(wat) {
			t.Fatalf("ctor temporaries missing %s local type:\n%s", typ, wat)
		}
	}
}
