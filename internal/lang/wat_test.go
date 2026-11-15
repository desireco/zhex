package lang

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
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
		"struct.get $zt_Status_Failed 2", // payload index shifts past the padding + discriminant
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
		{"builtin", `print(text.reverse("ab"))`, "text.reverse is not available in this build"},
		{"while", `
var n = 0
while n < 3
    n = n + 1
`, "while is not available in this build"},
		{"checked-append", `
let xs: List<Int> = list.new()
list.append(xs, 1) try
`, "checked list.append is not available in this build"},
		{"checked-index", `
let xs: List<Int> = [1]
print(xs[9] try)
`, "checked index reads is not available in this build"},
		{"map-literal", `
let m: Map<String, Int> = {}
print(1)
`, "map literals is not available in this build"},
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
		{"partials", `
word add(a: Int, b: Int) -> Int
    a + b

let inc = add(_, 1)
print(inc(2))
`, "partials is not available in this build"},
		{"file-stream", `
for line in file.stream("nope.txt")
    print(line)
`, "file.stream is not available in this build"},
		{"builtin-2", `let t = text.length("ab")`, "text.length is not available in this build"},
		{"generic-choice", `
type Pair<T> = choice
    Left(a: T)
    Right(b: T)

print(1)
`, "generic choice Pair is not available in this build"},
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
			if !strings.Contains(err.Error(), "hint: this construct is outside the wasm milestone-1 subset; run it with zhex run instead") {
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

// ------------------------------------------------- assemble-and-run smoke

// TestWatAssembleAndRunSmoke compiles, assembles with wasm-tools, and
// executes under Node — including the amended Int Result payload round trip
// (ok(2^31) try must print 2147483648, which ref.i31 boxing would corrupt).
func TestWatAssembleAndRunSmoke(t *testing.T) {
	requireTool(t, "wasm-tools")
	requireTool(t, "node")
	wat := watOf(t, `
word big() -> Result<Int, Error>
    return ok(2147483648)

print(1 + 1)
print(big() try)
`)
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
	if got, want := out.String(), "2\n2147483648\n"; got != want {
		t.Fatalf("stdout = %q, want %q (a 31-bit i31 box would corrupt the second line)", got, want)
	}
}
