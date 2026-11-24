package lang

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// watOf compiles src through CompileWat (browser skin, the default); fence
// failures fail the test with the diagnostic rendered.
func watOf(t *testing.T, src string) string {
	t.Helper()
	wat, err := CompileWat("repl", src, nil, WatSkinBrowser)
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

// ---------------------------------------------------------------- literals

func TestWatLiteralsAndArithmetic(t *testing.T) {
	wat := watOf(t, `print(1 + 2 * 3)`)
	for _, want := range []string{
		"i64.const 1", "i64.const 2", "i64.const 3",
		"call $zhex_mul", "call $zhex_add", // precedence: (1) then (2*3)
		"call $zhex_int_to_string",
		"call $zhex_print_line", // the module owns print's newline (M3)
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

// TestWatIntBitWords pins the #231 bitwise words in the emitter: and/or/xor
// are unchecked limbwise i64 ops (inline, like text.length), while both
// shifts call the count-checking prelude helpers ($zhex_shl/$zhex_shr).
func TestWatIntBitWords(t *testing.T) {
	wat := watOf(t, `
let code = 168
print(int.and(code, 7))
print(int.or(code, 128))
print(int.xor(code, 255))
print(int.shift-left(1, 3))
print(int.shift-right(code, 6))
`)
	for _, want := range []string{"i64.and", "i64.or", "i64.xor", "call $zhex_shl", "call $zhex_shr"} {
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
		"call $zhex_str_concat", "call $zhex_print_line",
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

// TestWatEntryInvokesMainWord pins #134: an empty entry body compiles to a
// call of the reserved zero-argument main word, a Result return unwrapping
// like a top-level try; beside statements the word is never auto-called.
func TestWatEntryInvokesMainWord(t *testing.T) {
	entryFunc := func(wat string) string {
		at := strings.Index(wat, "(func $zhex_main")
		if at < 0 {
			t.Fatalf("no $zhex_main in %s", wat)
		}
		return wat[at:]
	}
	mainFn := entryFunc(watOf(t, "word main() -> Unit\n    print(1)\n"))
	if !strings.Contains(mainFn, "call $zw_main") {
		t.Errorf("empty body: main must be called, entry was %s", mainFn)
	}
	if strings.Index(mainFn, "call $zhex_env_init") > strings.Index(mainFn, "call $zw_main") {
		t.Errorf("env init must precede the main call: %s", mainFn)
	}
	fallible := entryFunc(watOf(t, "word main() -> Int\n    let n = text.to-int(\"3\") try\n    n\n"))
	if !strings.Contains(fallible, "call $zw_main") {
		t.Fatalf("fallible main missing: %s", fallible)
	}
	if !strings.Contains(fallible, "struct.get $zhex_result $tag") {
		t.Errorf("a Result main must unwrap at the entry: %s", fallible)
	}
	ordinaryFn := entryFunc(watOf(t, "word main() -> Unit\n    print(1)\nprint(2)\n"))
	if strings.Contains(ordinaryFn, "call $zw_main") {
		t.Errorf("statements present: main must stay ordinary, entry was %s", ordinaryFn)
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
	// each traversal resets the counter it raised (count only the program
	// section — the prelude's helpers also touch $active)
	program := wat[strings.Index(wat, ";; program types"):]
	if count := strings.Count(program, "struct.set $zhex_list_i64 $active"); count != 2 {
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

// #181: catch in assignment and argument positions is the same catch node,
// so it compiles through the same Result machinery instead of fencing.
func TestWatCatchInAssignmentAndArgumentPositions(t *testing.T) {
	wat := watOf(t, `
var n = 0
n = text.to-int("x") catch 7
let xs: List<Int> = [1]
let ys = list.append(xs, text.to-int("x") catch 0)
print(n)
print(ys[1])
`)
	for _, want := range []string{
		"struct.get " + watTypeResult + " $tag",
		"call " + watListAppend(watSpecI64),
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// #182: an indexed read followed by field reads compiles to the list/array
// read helper, the element cast, and the same struct.get the dotted-name
// path emits.
func TestWatPostfixFieldChains(t *testing.T) {
	wat := watOf(t, `
type Cell = record
    x: Int

let xs: List<Cell> = [Cell(x: 3)]
print(xs[0].x)
`)
	for _, want := range []string{
		"call " + watListGet(watSpecRef),
		"struct.get $zt_Cell $f0",
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT:\n%s", want, wat)
		}
	}
}

// TestWatErrorInspectionWords pins #135: error.message and error.detail
// compile to the prelude's err-value readers instead of fencing.
func TestWatErrorInspectionWords(t *testing.T) {
	wat := watOf(t, `
let line = read-line("") catch problem
    if error.detail(problem, "reason") == "end-of-file"
        ""
    else
        error.message(problem)
print(line)
`)
	for _, want := range []string{
		"call " + watErrorDetail,
		"call " + watErrorMessage,
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
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
		"call $zw_double",       // seed
		"local.set $t_c1_0",     // cursor local for the seed value
		"local.get $t_c1_0",     // cursor feeds the next step's first argument
		"call $zhex_print_line", // bare print step (int_to_string + print_line)
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// TestWatWithSteps pins the with-block emission: one cursor local for the
// seed, read by every step and never updated, with non-final step values
// dropped — the last step's value is the block's value.
func TestWatWithSteps(t *testing.T) {
	wat := watOf(t, `
let values = [1, 2]
let kept = flow values
with list
    append(3)
    length
print(kept)
`)
	for _, want := range []string{
		"local.set $t_c1_0",          // the seed cursor, set once
		"call $zhex_list_append_i64", // step 1 reads the seed cursor...
		"drop",                       // ...and its value discards
		"call $zhex_list_length_i64", // step 2 reads the same seed cursor
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(wat, "$t_c1_1") {
		t.Error("a with block must not thread a second cursor local")
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
		{"checked-first", `
let xs: List<Int> = list.new()
let head = list.first(xs) try
`, "checked list.first is not available in this build"},
		{"checked-last", `
let xs: List<Int> = list.new()
let tail = list.last(xs) try
`, "checked list.last is not available in this build"},
		{"checked-index-array", `
let grid = array.new([2, 2], 0)
print(grid[0, 0] try)
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
		{"generic-choice", `
type Pair<T> = choice
    Left(a: T)
    Right(b: T)

print(1)
`, "generic choice Pair is not available in this build"},
		{"file-free-space", `print(file.free-space(".") try)`, "file.free-space is not available in this build"},
		{"temporal", `print(date.to-string(date.new(2020, 1, 1)))`, "date.to-string is not available in this build"},
		{"console-arg", `let a = console.arg(0) try`, "console.arg is not available in this build"},
		{"wide-int-literal", `print(170141183460469231731687303715884105727)`, "integers beyond the signed 64-bit range is not available in this build"},
		{"error-new", `let e = error.new("x", "boom")`, "error.new is not available in this build"},
		// text.sentences fences until the #218 sentence layer joins the wasm host subset
		{"text-sentences", `let sentences = text.sentences("A. B.")`, "text.sentences is not available in this build"},
		{"checked-file-exists", `let yes = file.exists("x") try`, "checked file.exists is not available in this build"},
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
			_, err := CompileWat("repl", tc.src, nil, WatSkinBrowser)
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
	_, err := CompileWat("repl", `print(1 + true)`, nil, WatSkinBrowser)
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
	wat, err := CompileWat("main.zhex", "use math\nprint(math.gcd(48, 18))\n", mapResolver(map[string]string{"math.zhex": mathModule}), WatSkinBrowser)
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
	wat, err := CompileWat("main.zhex", "use geom\nlet p = geom.origin()\nprint(p.x)\n", mapResolver(map[string]string{"geom.zhex": mod}), WatSkinBrowser)
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
// (fail 16 since M3, retiring M2's code-12 reuse) instead of the generic
// fail 11 (DESIGN-M2 §2c/§2d, DESIGN-M3 §8).
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
		"i32.const " + itoa(watFailUnhandledResult),
		"call " + watFailMsg,
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q", want)
		}
	}
	program := wat[strings.Index(wat, ";; program types"):]
	if got := strings.Count(program, "i32.const "+itoa(watFailUnhandledResult)+"\n"); got != 1 {
		t.Errorf("code 16 uses = %d, want 1:\n%s", got, program)
	}
	if strings.Contains(program, "i32.const "+itoa(watFailTextToInt)+"\n") {
		t.Errorf("code 12 must no longer be emitted on the unhandled path:\n%s", program)
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
	// calls; the empty literal emits none. Counted in the program section —
	// the M3 prelude's json decode also calls the i64 map put once.
	programSection := wat[strings.Index(wat, ";; program types"):]
	if got := strings.Count(programSection, "call "+watMapPut(watMapKeyStr, watSpecI64)); got != 4 {
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
	// Two traversals: increment and decrement each; the program section is
	// the only place that may touch the counter here (the prelude's map
	// helpers have their own writes).
	program := wat[strings.Index(wat, ";; program types"):]
	if got := strings.Count(program, "struct.set "+watMapType(watMapKeyStr, watSpecI64)+" $active"); got != 4 {
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
		"ref.null eq", // the hole's env slot
		"array.new_fixed " + watTypeArrRef + " 1", // env of target arity
		"ref.func $zcthunk",                       // site-local thunk
		"struct.new " + watTypeClosure,
		"call " + watListMap(watSpecI64, watSpecI64),
		"(func $zcthunk", // the thunk is a plain function
		"(param $clo (ref " + watTypeClosure + "))",
		"(param $a0 i64)", // the hole is the callback param
		"(result i64)",
		"call $zw_double", // direct call, no enter of its own
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
// fixtures live with Builder T. The host is the fifteen-stub browser surface
// (fs/env stubs fail with ENOENT — the smoke programs only print); since M3
// print sends raw bytes and the module owns the newline, the stub forwards
// bytes verbatim (DESIGN-M3 §1 Surprises 2).
func assembleAndRun(t *testing.T, wat string) string {
	t.Helper()
	requireTool(t, "wasm-tools")
	requireTool(t, "node")
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
    env: {
      print: (ptr, len) => {
        const mem = new Uint8Array(instance.exports.memory.buffer, ptr, len);
        out += new TextDecoder().decode(mem);
      },
      stdin_read: () => 0,
      fs_read_file: () => 44,
      fs_write_file: () => 0,
      fs_open_read: () => -44,
      fs_read_line: () => 0,
      fs_close: () => 0,
      fs_stat: () => -44,
      fs_list: () => -44,
      fs_make_folder: () => 28,
      fs_delete: () => -44,
      fs_delete_folder: () => -44,
      fs_move: () => -44,
      env_get: () => 0,
      env_environ: () => 0,
    },
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

// ----------------------------------------------------------------- duration

// Duration is Int's repr — unboxed i64 nanoseconds, a distinct checker type
// (DESIGN-M3 §5); duration.new is a passthrough, so it emits no call.
func TestWatDurationIsUnboxedAndNewPassthrough(t *testing.T) {
	wat := watOf(t, `
word echo(d: Duration) -> Duration
    duration.new(3)

word give() -> Result<Duration, Error>
    return ok(duration.new(4))

let held = duration.new(5)
let got = give() try
let n = text.to-int("1") try
print(n)
`)
	for _, want := range []string{
		"(func $zw_echo (param $p0_d i64) (result i64)", // Duration params and results are i64
		"i64.const 3",               // passthrough: the argument flows straight through
		"(local $l1_held i64)",      // a Duration binding stores i64
		"struct.new $zhex_some_i64", // a Duration Result payload boxes like Int's
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT:\n%s", want, wat)
		}
	}
	if strings.Contains(wat, "call $zhex_duration") {
		t.Errorf("duration.new must not emit a prelude call:\n%s", wat)
	}
}

// --------------------------------------------------------------- json tree

// The Json tree is the host-predeclared choice: six prelude-owned padded
// variant structs ($zt_Json_<Variant>), constructed with plain struct.news
// and matched with the standard ref.test chain in source arm order
// (DESIGN-M3 §6).
func TestWatJsonTreeConstructorsAndMatch(t *testing.T) {
	wat := watOf(t, `
word kind(doc: Json) -> String
    match doc
        case Json.Text(value)
            value
        case Json.Object(fields)
            "object"
        case _
            "other"

let fields: Map<String, Json> = {}
map.put(fields, "n", Json.Number("1.50"))
let tree = Json.Object(fields)
match tree
    case Json.Object(fields)
        print(kind(Json.Text("x")))
    case _
        print("no")
`)
	for _, want := range []string{
		"struct.new $zt_Json_Text",
		"struct.new $zt_Json_Number",
		"struct.new $zt_Json_Object",
		"ref.test (ref $zt_Json_Text)", // arms test their variants in source order
		"ref.test (ref $zt_Json_Object)",
		"struct.get $zt_Json_Text $f0",                // payload bindings read $f0 like user choices
		"call " + watMapPut(watMapKeyStr, watSpecRef), // Object payload is Map<String, Json> (ref values)
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT:\n%s", want, wat)
		}
	}
	// the prelude declares the Json variant structs; the program section must
	// not redeclare them
	program := wat[strings.Index(wat, ";; program types"):]
	if strings.Contains(program, "(type $zt_Json") {
		t.Errorf("the emitter must not redeclare the prelude's Json structs:\n%s", program)
	}
}

// ------------------------------------------------------------------- skins

// The skin selects the host layer: browser keeps the fifteen env.* imports,
// WASI imports wasi_snapshot_preview1 instead; the program section (types,
// words, main, exports) is byte-identical for both (DESIGN-M3 §1).
func TestWatSkinSelectsHostLayer(t *testing.T) {
	src := `print(1 + 1)`
	browser, err := CompileWat("repl", src, nil, WatSkinBrowser)
	if err != nil {
		t.Fatal(err)
	}
	wasi, err := CompileWat("repl", src, nil, WatSkinWASI)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(browser, `"env" "fs_read_file"`) {
		t.Errorf("browser skin must import the env surface")
	}
	if !strings.Contains(wasi, "wasi_snapshot_preview1") || strings.Contains(wasi, `"env" "fs_read_file"`) {
		t.Errorf("wasi skin must import wasi_snapshot_preview1 instead of env.*")
	}
	if b, w := browser[strings.Index(browser, ";; program types"):], wasi[strings.Index(wasi, ";; program types"):]; b != w {
		t.Errorf("program section must be byte-identical across skins")
	}
	// main seeds the module-side environment before any statement (§7).
	if !strings.Contains(browser, "call "+watEnvInit) {
		t.Errorf("$zhex_main must call %s as its first statement", watEnvInit)
	}
}

// ------------------------------------------------------------- file.* words

// Pure path words are host-free prelude calls (DESIGN-M3 §3).
func TestWatFilePathWordsEmitPreludeCalls(t *testing.T) {
	wat := watOf(t, `
print(file.join("a", "b"))
print(file.base("a/b.txt"))
print(file.extension("a/b.txt"))
print(file.directory-of("a/b.txt"))
`)
	for _, want := range []string{
		"call " + watPathJoin,
		"call " + watPathBase,
		"call " + watPathExtension,
		"call " + watPathDirOf,
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT", want)
		}
	}
}

// The import-backed file words evaluate their arguments left to right and
// call one prelude helper each (DESIGN-M3 §3); File values are $zhex_file
// struct references. file.exists emits the plain Bool form (the helper hard
// fails 18 on stat errors other than not-found).
func TestWatFileWordsEmitHostHelpers(t *testing.T) {
	wat := watOf(t, `
word drain(f: File) -> Unit
    let line = file.read-line(f) try
    print(line)

let content = file.read("in.txt") try
print(content)
file.write("hi", "out.txt") try
file.write("hi", "out.txt", true) try
if file.exists("in.txt")
    print("yes")
print(file.size("in.txt") try)
let names = file.list(".") try
print(text.join(names, ","))
file.make-folder("d") try
file.make-folder("d", true) try
file.delete("old.txt") try
file.delete-folder("d") try
file.delete-folder("d", true) try
file.move("a", "b") try
file.move("a", "b", true) try
let h = file.open("in.txt") try
file.close(h)
`)
	for _, want := range []string{
		"call " + watFileRead,
		"call " + watFileOpen,
		"call " + watFileReadLine,
		"call " + watFileClose,
		"call " + watFileList,
		"call " + watFileExists,
		"call " + watFileSize,
		"(func $zw_drain (param $p0_f (ref null " + watTypeFile + "))",
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT:\n%s", want, wat)
		}
	}
	if !regexp.MustCompile(`\(local \$l\d+_h \(ref null ` + regexp.QuoteMeta(watTypeFile) + `\)\)`).MatchString(wat) {
		t.Errorf("a File binding stores the %s struct reference", watTypeFile)
	}
	// write/make-folder/delete-folder/move each appear twice: one
	// omitted-default call plus one explicit-argument call (the prelude's
	// own calls to the same helpers stay out of the count).
	program := wat[strings.Index(wat, ";; program types"):]
	for _, name := range []string{watFileWrite, watFileMakeFolder, watFileDeleteFolder, watFileMove} {
		if got := strings.Count(program, "call "+name); got != 2 {
			t.Errorf("%s calls = %d, want 2", name, got)
		}
	}
}

// Omitted trailing builtin defaults are pushed as constants at the call site
// (eval appends them from the same table; the values are constants).
func TestWatFileBuiltinDefaultsPushedWhenOmitted(t *testing.T) {
	wat := watOf(t, `
file.write("hi", "out.txt") try
file.make-folder("d") try
file.delete-folder("d") try
file.move("a", "b") try
env.load() try
`)
	// each omitted Bool default lands on the line right before its call
	program := wat[strings.Index(wat, ";; program types"):]
	for _, call := range []string{watFileWrite, watFileMakeFolder, watFileDeleteFolder, watFileMove} {
		at := strings.Index(program, "call "+call)
		if at < 0 {
			t.Fatalf("missing call %s", call)
		}
		nl := strings.LastIndex(program[:at], "\n")
		if prev := program[strings.LastIndex(program[:nl], "\n")+1 : nl]; prev != "  i32.const 0" {
			t.Errorf("line before %s = %q, want i32.const 0", call, prev)
		}
	}
	// env.load()'s omitted path default is the ".env" string constant
	// ('.' is its first staged code point).
	at := strings.Index(program, "call "+watEnvLoad)
	if at < 0 || !strings.Contains(program[:at], "i32.const 46") {
		t.Errorf("env.load() must stage the .env string default before the call:\n%s", program)
	}
}

// ------------------------------------------------- for line in file.stream

// The stream loop opens once, reads until the ok-null EOF payload, and
// closes on every exit — the normal one and an early word return (the
// watCleanup-list treatment, DESIGN-M3 §3).
func TestWatForLinesEmitsStreamLoop(t *testing.T) {
	wat := watOf(t, `
for line in file.stream("in.txt")
    print(line)
`)
	for _, want := range []string{
		"call " + watStreamOpen,
		"call " + watStreamLine,
		"call " + watStreamClose,
		"struct.get " + watTypeResult + " $payload", // ok-null = EOF, ok-string = the chomped line
		"ref.is_null",
		"br_if $exit",
		"ref.cast (ref null $zhex_string)",
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT:\n%s", want, wat)
		}
	}
	program := wat[strings.Index(wat, ";; program types"):]
	if got := strings.Count(program, "call "+watStreamClose); got != 1 {
		t.Errorf("stream close calls = %d, want 1 (loop exit)", got)
	}
}

func TestWatForLinesEarlyReturnClosesStream(t *testing.T) {
	wat := watOf(t, `
word first-line(path: String) -> Result<String, Error>
    for line in file.stream(path)
        return ok(line)
    ok("")

print(first-line("in.txt") try)
`)
	program := wat[strings.Index(wat, ";; program types"):]
	retAt := strings.Index(program, "br $zhex_ret")
	if retAt < 0 || !strings.Contains(program[:retAt], "call "+watStreamClose) {
		t.Fatalf("an early return must close the stream before branching out:\n%s", program)
	}
	if got := strings.Count(program, "call "+watStreamClose); got != 2 {
		t.Errorf("stream close calls = %d, want 2 (return path + loop exit)", got)
	}
}

// --------------------------------------------------------- read-line, env

func TestWatReadLineAndEnvWords(t *testing.T) {
	wat := watOf(t, `
let line = read-line("> ") try
print(line)
match env.get("HOME")
    case Option.Some(home)
        print(home)
    case Option.None
        print("none")
env.load() try
env.load("custom.env") try
let ok-loaded = env.load() try
`)
	for _, want := range []string{
		"call " + watReadLine,
		"call " + watEnvGet,
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT", want)
		}
	}
	program := wat[strings.Index(wat, ";; program types"):]
	if got := strings.Count(program, "call "+watEnvLoad); got != 3 {
		t.Errorf("env.load calls = %d, want 3 (one omitted-default, two explicit)", got)
	}
}

// --------------------------------------------------------------- buffer.*

func TestWatBufferWordsEmitHelpers(t *testing.T) {
	wat := watOf(t, `
let b = buffer.new()
buffer.add-line(b, "one")
buffer.add-line(b, "two")
print(buffer.line-count(b))
print(buffer.line(b, 0) try)
buffer.set-line(b, 0, "1") try
print(buffer.to-text(b))
`)
	for _, want := range []string{
		"call " + watBufferNew,
		"call " + watBufferAddLine,
		"call " + watBufferLineCount,
		"call " + watBufferLine,
		"call " + watBufferSetLine,
		"call " + watBufferToText,
		"(local $l1_b (ref null " + watTypeBuffer + "))",
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT:\n%s", want, wat)
		}
	}
	program := wat[strings.Index(wat, ";; program types"):]
	if got := strings.Count(program, "call "+watBufferAddLine); got != 2 {
		t.Errorf("add-line calls = %d, want 2", got)
	}
}

// ------------------------------------------------- duration words and print

func TestWatDurationWordsAndPrint(t *testing.T) {
	wat := watOf(t, `
let d = duration.parse("1.5h") try
print(duration.to-string(d))
print(duration.to-string(duration.new(3)))
`)
	for _, want := range []string{
		"call " + watDurationParse,
		"call " + watDurationToString,
		"call " + watPrintLine, // the module owns print's newline (Surprises 2)
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT:\n%s", want, wat)
		}
	}
	// print of a Duration value composes to-string then print_line (the
	// emitter's print path; the checker currently gates direct duration
	// prints to to-string composition).
	if !regexp.MustCompile(`call ` + regexp.QuoteMeta(watDurationToString) + `\n\s*call ` + regexp.QuoteMeta(watPrintLine)).MatchString(wat) {
		t.Errorf("print of Duration must render via %s then %s:\n%s", watDurationToString, watPrintLine, wat)
	}
}

// -------------------------------------------------------------- json words

func TestWatJsonWordsEmitHelpers(t *testing.T) {
	wat := watOf(t, `
let doc = json.parse("{\"a\":1}") try
print(json.encode(doc) try)

type Point = record
    x: Int
    y: Int

let encoded = json.encode(Point(1, 2)) try
print(encoded)
let decoded = json.decode("{\"x\":1,\"y\":2}", Point(0, 0)) try
print(decoded.x)
`)
	for _, want := range []string{
		"call " + watJsonParse,
		"call " + watJsonEncode,
		"call " + watJsonDecode,
		"ref.null " + watTypeJsonSchema, // tree encode passes a null schema
		"ref.func $zjson_make",          // record schemas carry site thunks (§6 deviation)
		"ref.func $zjson_get",
		// the thunks carry the shared functype index: wasm-tools rejects a
		// ref.func of a separately declared, structurally identical signature
		"type " + watTypeJsonMakeFn,
		"type " + watTypeJsonGetFn,
		"struct.new " + watTypeJsonField,
		"(elem declare func", // ref.func targets must be declared
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT:\n%s", want, wat)
		}
	}
	// two typed call sites, no caching: two distinct builders, each called once
	program := wat[strings.Index(wat, ";; program types"):]
	if got := strings.Count(program, "(func $zjson_schema"); got != 2 {
		t.Errorf("schema builder functions = %d, want 2", got)
	}
	if got := strings.Count(program, "call $zjson_schema"); got != 2 {
		t.Errorf("schema builder calls = %d, want 2", got)
	}
	// the make thunk unboxes Int fields from $zhex_some_i64; get boxes them
	if !strings.Contains(wat, "struct.get "+watSomeType(watSpecI64)+" $value") {
		t.Errorf("the make thunk must unbox Int fields from the boxed array")
	}
}

// ------------------------------------------------------ milestone-4 (DESIGN-M4)

// xml.decode mirrors json.decode's emitter case: raw text, boxed defaults,
// the M3 schema builder (which now also registers node names), one prelude
// call — and it rides a flow step exactly as json.decode does (§2).
func TestWatXmlDecodeEmitsPreludeCall(t *testing.T) {
	wat := watOf(t, `
type Server = record
    name: String
    retries: Int

let s = xml.decode("<server><name>a</name></server>", Server("b", 1)) try
print(s.name)
`)
	for _, want := range []string{
		"call " + watXmlDecode,
		"call $zjson_schema1", // one builder per call site
		"struct.new " + watTypeJsonField,
		"(global.set " + watXmlNames, // the builder registers node names (§2b)
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT:\n%s", want, wat)
		}
	}
	program := wat[strings.Index(wat, ";; program types"):]
	if got := strings.Count(program, "call "+watXmlDecode); got != 1 {
		t.Errorf("xml decode calls = %d, want 1", got)
	}
}

// The checked List read is a Result helper per element spec (§4); checked
// Array reads and all checked writes stay fenced.
func TestWatCheckedListReadEmitsHelper(t *testing.T) {
	wat := watOf(t, `
let xs: List<Int> = [1, 2]
let a = xs[9] try
print(a)
let words: List<String> = ["a"]
let b = words[0] try
print(b)
`)
	for _, want := range []string{
		"call " + watListGetChecked(watSpecI64),
		"call " + watListGetChecked(watSpecRef),
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT:\n%s", want, wat)
		}
	}
	// The negative pin scopes to the program section: the prelude's own
	// first/last helpers delegate to the plain get by design, so only the
	// program's call sites must avoid it for a checked read.
	program := wat[strings.Index(wat, ";; program types"):]
	if strings.Contains(program, "call "+watListGet(watSpecI64)) {
		t.Errorf("a checked read must not use the trapping plain get")
	}
}

// A Unit-result closure compiles: creation and invocation go through the
// grid's void rows, and the invoke leaves nothing on the stack (§3a).
func TestWatVoidClosureInvokeUsesVoidGridRow(t *testing.T) {
	wat := watOf(t, `
word side(n: Int)
    print(n)

let run = side(_)
run(1)
flow 2
    run
`)
	for _, want := range []string{
		"ref.cast (ref " + watCloType([]watSpec{watSpecI64}, watCloVoid) + ")",
		"call_ref " + watCloType([]watSpec{watSpecI64}, watCloVoid),
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT:\n%s", want, wat)
		}
	}
	if regexp.MustCompile(`call_ref ` + regexp.QuoteMeta(watCloType([]watSpec{watSpecI64}, watCloVoid)) + `\n\s*ref\.cast`).MatchString(wat) {
		t.Errorf("a void call_ref must not be followed by a result cast")
	}
}

// Re-partial application: the callee evaluates first, the env array gains
// the target-closure slot, and the thunk call_refs the target's own
// signature (§3b). Partial-of-partial composes the same way.
func TestWatRePartialRidesEnvSlot(t *testing.T) {
	wat := watOf(t, `
word add(a: Int, b: Int) -> Int
    a + b

let once = add(_, 1)
let twice = once(_)
print(twice(5))
`)
	thunk := regexp.MustCompile(`\(func \$zcthunk2`).FindString(wat[strings.Index(wat, ";; closure thunks"):])
	if thunk == "" {
		t.Fatalf("the re-partial site must emit its own thunk:\n%s", wat)
	}
	for _, want := range []string{
		"array.new_fixed " + watTypeArrRef + " 2", // binding arity 1 + the target slot
		"ref.cast (ref " + watCloType([]watSpec{watSpecI64}, watSpecI64) + ")",
		"call_ref " + watCloType([]watSpec{watSpecI64}, watSpecI64),
	} {
		if !strings.Contains(wat, want) {
			t.Errorf("missing %q in emitted WAT:\n%s", want, wat)
		}
	}
}

// A closure binding is a legal flow step: the cursor is args[0] and the
// invoke rides closureInvoke's existing paths (§3c).
func TestWatClosureFlowStepEmitsCallRef(t *testing.T) {
	wat := watOf(t, `
word add(a: Int, b: Int) -> Int
    a + b

let once = add(_, 1)
flow 5
    once
    print
`)
	if !strings.Contains(wat, "call_ref "+watCloType([]watSpec{watSpecI64}, watSpecI64)) {
		t.Errorf("the closure step must call_ref its own signature:\n%s", wat)
	}
}

// TestWatMilestone4RoundTrip is the emitter's end-to-end witness for the M4
// families: xml record/list/scalar decoding with entities, the trio (void
// closure invoked and as a flow step, re-partial chains, closure flow steps
// with results), and the checked List read's ok/out-of-range paths. Skipped
// without wasm-tools or node; the differential corpus is Builder Y's gate.
func TestWatMilestone4RoundTrip(t *testing.T) {
	wat := watOf(t, `
type Endpoint = record
    host: String
    port: Int
type Server = record
    name: String
    retries: Int
    verbose: Bool
    endpoints: List<Endpoint>

let s = xml.decode("<server retries=\"3\" verbose=\"true\"><name>ingest</name><endpoints><host>db</host><port>5432</port></endpoints></server>", Server("backup", 1, false, [])) try
print(s.name)
print(s.retries)
print(s.verbose)
print(list.length(s.endpoints))
let e0 = s.endpoints[0] try
print(e0.host)
print(e0.port)
let n = xml.decode("<n>7</n>", 0) try
print(n)
let txt = xml.decode("<t>a &amp; b &#65;</t>", "") try
print(txt)

word side(n: Int)
    print(n)

let run = side(_)
run(1)
flow 2
    run

word add(a: Int, b: Int) -> Int
    a + b

let once = add(_, 1)
let twice = once(_)
print(twice(5))
print(once(9))
flow 5
    twice
    print

let bad = s.endpoints[9] catch Endpoint("none", 0)
print(bad.host)
`)
	got := assembleAndRun(t, wat)
	want := "ingest\n3\ntrue\n1\ndb\n5432\n7\na & b A\n1\n2\n6\n10\n6\nnone\n"
	if got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

// TestWatM4CloneDefaultsAndPrintPartial regression-tests the review gaps:
// an absent List field deep-clones the defaults (appending to one decode
// never mutates the shared default — the interpreter's jsonClone is
// value-shaped, so the xml side clones every element spec), and `print _`
// compiles as the Unit-closure witness (DESIGN-M4 §3a).
func TestWatM4CloneDefaultsAndPrintPartial(t *testing.T) {
	wat := watOf(t, `
type Settings = record
    left: List<Int>
    right: List<Int>

let shared = [1]
let defaults = Settings(shared, shared)
let first = xml.decode("<settings><left>7</left></settings>", defaults) try
first.left[0] = 9
print(first.left[0] try)
print(first.right[0] try)
print(defaults.left[0] try)
let second = xml.decode("<settings/>", defaults) try
print(second.left[0] try)
second.left[0] = 5
print(defaults.left[0] try)

let show: (Int) -> Unit = print _
show(41)
flow 42
    show
let greet: (String) -> Unit = print _
greet("hi")
`)
	if !strings.Contains(wat, "call_ref "+watCloType([]watSpec{watSpecI64}, watCloVoid)) {
		t.Errorf("print _ must invoke through the grid's void row:\n%s", wat)
	}
	got := assembleAndRun(t, wat)
	want := "9\n1\n1\n1\n1\n41\n42\nhi\n"
	if got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

// TestWatM4NbspQuotingRegression pins the NBSP quoting fix (DESIGN-M4 §5)
// with a real U+00A0 in the source literal: the staged message renders
// "\u00a0x" exactly as the interpreter's %q does (the fixture byte is a
// true NBSP, not the ASCII space lookalike).
func TestWatM4NbspQuotingRegression(t *testing.T) {
	wat := watOf(t, "let n = text.to-int(\"\u00a0x\") catch 0\nprint(n)\n")
	got := assembleAndRun(t, wat)
	if got != "0\n" {
		t.Fatalf("caught fallback = %q, want 0\n", got)
	}
	wat = watOf(t, "let n = text.to-int(\"\u00a0x\") try\nprint(n)\n")
	dir := t.TempDir()
	watPath := dir + "/nbsp.wat"
	wasmPath := dir + "/nbsp.wasm"
	if err := os.WriteFile(watPath, []byte(wat), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("wasm-tools", "parse", watPath, "-o", wasmPath).CombinedOutput(); err != nil {
		t.Fatalf("wasm-tools parse: %v\n%s", err, out)
	}
	var out, stderr bytes.Buffer
	cmd := exec.Command("node", "internal/wasm/testdata/run.mjs", wasmPath)
	cmd.Dir = "../.."
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err == nil {
		t.Fatal("expected the unhandled try to exit nonzero")
	}
	if !strings.Contains(stderr.String(), "runtime error: \"\\u00a0x\" is not a signed 64-bit decimal integer") {
		t.Fatalf("stderr = %q, want the \\u00a0-escaped message", stderr.String())
	}
}
