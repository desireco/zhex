package lang

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestWatPreludePinnedNames pins the shared interface with the WAT emitter
// (DESIGN.md §2/§5/§6): every type, helper, import, global, and memory the
// emitter or the Node runner addresses by name must appear byte-for-byte in
// the prelude, on tool-less CI included.
func TestWatPreludePinnedNames(t *testing.T) {
	pinned := []string{
		// §2 value representation types (all element specializations).
		"(type $zhex_string      (array (mut i32)))",
		"(type $zhex_arr_i64     (array (mut i64)))",
		"(type $zhex_arr_i32     (array (mut i32)))",
		"(type $zhex_arr_ref     (array (mut (ref null eq))))",
		"(type $zhex_list_i64    (struct (field $items (mut (ref null $zhex_arr_i64))) (field $len (mut i32)) (field $active (mut i32))))",
		"(type $zhex_list_i32    (struct (field $items (mut (ref null $zhex_arr_i32))) (field $len (mut i32)) (field $active (mut i32))))",
		"(type $zhex_list_ref    (struct (field $items (mut (ref null $zhex_arr_ref))) (field $len (mut i32)) (field $active (mut i32))))",
		"(type $zhex_array_i64   (struct (field $shape (ref $zhex_arr_i64)) (field $items (mut (ref null $zhex_arr_i64))) (field $active (mut i32))))",
		"(type $zhex_array_i32   (struct (field $shape (ref $zhex_arr_i64)) (field $items (mut (ref null $zhex_arr_i32))) (field $active (mut i32))))",
		"(type $zhex_array_ref   (struct (field $shape (ref $zhex_arr_i64)) (field $items (mut (ref null $zhex_arr_ref))) (field $active (mut i32))))",
		"(type $zhex_some_i64    (struct (field $value i64)))",
		"(type $zhex_some_i32    (struct (field $value i32)))",
		"(type $zhex_some_ref    (struct (field $value (ref null eq))))",
		"(type $zhex_none        (struct))",
		"(type $zhex_result      (struct (field $tag i32) (field $payload (ref null eq))))",
		// §4 import, prelude-owned memory, §3/§6a globals and exports.
		`(import "env" "print" (func $env.print (param i32 i32)))`,
		"(memory $zhex_mem 1)",
		"(global $zhex_depth (mut i32) (i32.const 0))",
		"(global $zhex_err (mut i32) (i32.const 0))",
		"(func $zhex_last_error (result i32)",
		// §5 helper inventory, signatures included.
		"(func $zhex_fail (param $code i32)",
		"(func $zhex_enter",
		"(func $zhex_leave",
		"(func $zhex_int_to_string (param $v i64) (result (ref $zhex_string))",
		"(func $zhex_str_eq (param $a (ref $zhex_string)) (param $b (ref $zhex_string)) (result i32)",
		"(func $zhex_str_concat (param $a (ref $zhex_string)) (param $b (ref $zhex_string)) (result (ref $zhex_string))",
		"(func $zhex_print_str (param $s (ref $zhex_string))",
		"(func $zhex_print_int (param $v i64)",
		"(func $zhex_box_i32 (param $v i32) (result (ref eq))",
		"(func $zhex_unbox_i32 (param $p (ref null eq)) (result i32)",
		"(func $zhex_add (param $a i64) (param $b i64) (result i64)",
		"(func $zhex_sub (param $a i64) (param $b i64) (result i64)",
		"(func $zhex_mul (param $a i64) (param $b i64) (result i64)",
		"(func $zhex_neg (param $a i64) (result i64)",
		"(func $zhex_div (param $a i64) (param $b i64) (result i64)",
		"(func $zhex_rem (param $a i64) (param $b i64) (result i64)",
		"(func $zhex_shl (param $a i64) (param $n i64) (result i64)",
		"(func $zhex_shr (param $a i64) (param $n i64) (result i64)",
	}
	for _, x := range []string{"i64", "i32", "ref"} {
		pinned = append(pinned,
			"(func $zhex_list_new_"+x+" (result (ref $zhex_list_"+x+"))",
			"(func $zhex_list_length_"+x+" (param $list (ref $zhex_list_"+x+")) (result i64)",
			"(func $zhex_list_append_"+x,
			"(func $zhex_list_get_"+x,
			"(func $zhex_list_set_"+x,
			"(func $zhex_array_new_"+x,
			"(func $zhex_array_length_"+x+" (param $arr (ref $zhex_array_"+x+")) (result i64)",
			"(func $zhex_array_get_"+x,
			"(func $zhex_array_set_"+x,
		)
	}
	for _, want := range pinned {
		if !strings.Contains(watPrelude, want) {
			t.Errorf("prelude is missing pinned name: %s", want)
		}
	}
}

// TestWatPrelude assembles the prelude inside a test module (wasm-tools parse
// — the only assembler with WasmGC support, DESIGN.md Surprises 1), then
// behavior-tests every helper family under node: integer formatting edges,
// string concat/equality/print staging, the depth trap with its pinned code,
// overflow helpers, collection failure codes, and i31 boxing. Skipped when
// either tool is missing so tool-less CI stays green.
func TestWatPrelude(t *testing.T) {
	wasmTools, err := exec.LookPath("wasm-tools")
	if err != nil {
		t.Skipf("wasm-tools not installed: %v", err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node not installed: %v", err)
	}
	dir := t.TempDir()
	watPath := filepath.Join(dir, "prelude.wat")
	if err := os.WriteFile(watPath, []byte("(module\n"+watPrelude+"\n"+watPreludeTestExports+"\n)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wasmPath := filepath.Join(dir, "prelude.wasm")
	if out, err := exec.Command(wasmTools, "parse", watPath, "-o", wasmPath).CombinedOutput(); err != nil {
		t.Fatalf("wasm-tools parse: %v\n%s", err, out)
	}
	mjsPath := filepath.Join(dir, "run.mjs")
	if err := os.WriteFile(mjsPath, []byte(watPreludeRunner), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, mjsPath, wasmPath).CombinedOutput()
	if err != nil {
		t.Fatalf("node runner failed: %v\n%s", err, out)
	}
	var report struct {
		Total  int              `json:"total"`
		Failed []map[string]any `json:"failed"`
	}
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("runner output is not the JSON report: %v\n%s", err, out)
	}
	if report.Total < 55 {
		t.Errorf("runner executed only %d cases; the suite shrank", report.Total)
	}
	for _, f := range report.Failed {
		t.Errorf("case %v failed: %v", f["name"], f["error"])
	}
}

// watPreludeTestExports wraps the prelude helpers in exported functions with
// plain numeric signatures (GC references never cross to JS), mirroring what
// the emitter's module tail exports: memory and zhex_last_error.
const watPreludeTestExports = `
(func (export "print_int") (param $v i64) (call $zhex_print_int (local.get $v)))
(func (export "print_str_hello") (call $zhex_print_str (array.new_fixed $zhex_string 6 (i32.const 104) (i32.const 233) (i32.const 108) (i32.const 108) (i32.const 111) (i32.const 9731))))
(func (export "print_str_emoji") (call $zhex_print_str (array.new_fixed $zhex_string 1 (i32.const 128512))))
(func (export "print_str_empty") (call $zhex_print_str (array.new_fixed $zhex_string 0)))

(func (export "eq_same") (result i32)
  (call $zhex_str_eq (array.new_fixed $zhex_string 2 (i32.const 97) (i32.const 98))
                     (array.new_fixed $zhex_string 2 (i32.const 97) (i32.const 98))))
(func (export "eq_tail") (result i32)
  (call $zhex_str_eq (array.new_fixed $zhex_string 2 (i32.const 97) (i32.const 98))
                     (array.new_fixed $zhex_string 2 (i32.const 97) (i32.const 99))))
(func (export "eq_len") (result i32)
  (call $zhex_str_eq (array.new_fixed $zhex_string 2 (i32.const 97) (i32.const 98))
                     (array.new_fixed $zhex_string 3 (i32.const 97) (i32.const 98) (i32.const 99))))
(func (export "eq_empty") (result i32)
  (call $zhex_str_eq (array.new_fixed $zhex_string 0) (array.new_fixed $zhex_string 0)))
(func (export "eq_empty_a") (result i32)
  (call $zhex_str_eq (array.new_fixed $zhex_string 0) (array.new_fixed $zhex_string 1 (i32.const 97))))
(func (export "eq_unicode") (result i32)
  (call $zhex_str_eq (array.new_fixed $zhex_string 3 (i32.const 104) (i32.const 233) (i32.const 9731))
                     (array.new_fixed $zhex_string 3 (i32.const 104) (i32.const 233) (i32.const 9731))))

(func (export "cat_plain") (call $zhex_print_str
  (call $zhex_str_concat (array.new_fixed $zhex_string 3 (i32.const 102) (i32.const 111) (i32.const 111))
                         (array.new_fixed $zhex_string 3 (i32.const 98) (i32.const 97) (i32.const 114)))))
(func (export "cat_empty") (call $zhex_print_str
  (call $zhex_str_concat (array.new_fixed $zhex_string 0) (array.new_fixed $zhex_string 0))))
(func (export "cat_mixed") (call $zhex_print_str
  (call $zhex_str_concat (array.new_fixed $zhex_string 3 (i32.const 104) (i32.const 233) (i32.const 108))
                         (array.new_fixed $zhex_string 2 (i32.const 111) (i32.const 9731)))))

(func $rec (export "rec") (param $n i32)
  (if (i32.eqz (local.get $n))
    (then (return)))
  (call $zhex_enter)
  (call $rec (i32.sub (local.get $n) (i32.const 1)))
  (call $zhex_leave))

(func (export "add") (param $a i64) (param $b i64) (result i64) (call $zhex_add (local.get $a) (local.get $b)))
(func (export "sub") (param $a i64) (param $b i64) (result i64) (call $zhex_sub (local.get $a) (local.get $b)))
(func (export "mul") (param $a i64) (param $b i64) (result i64) (call $zhex_mul (local.get $a) (local.get $b)))
(func (export "neg") (param $a i64) (result i64) (call $zhex_neg (local.get $a)))
(func (export "div") (param $a i64) (param $b i64) (result i64) (call $zhex_div (local.get $a) (local.get $b)))
(func (export "rem_native") (param $a i64) (param $b i64) (result i64) (i64.rem_s (local.get $a) (local.get $b)))

(func $l64 (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $zhex_list_new_i64))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 10)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 20)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 30)))
  (local.get $l))
(func (export "l64_len") (result i64) (call $zhex_list_length_i64 (call $l64)))
(func (export "l64_get") (param $i i64) (result i64) (call $zhex_list_get_i64 (call $l64) (local.get $i)))
(func (export "l64_set_get") (param $i i64) (param $v i64) (result i64)
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $l64))
  (call $zhex_list_set_i64 (local.get $l) (local.get $i) (local.get $v))
  (call $zhex_list_get_i64 (local.get $l) (local.get $i)))
(func (export "l64_alias") (result i64)
  (local $l (ref $zhex_list_i64))
  (local $alias (ref null $zhex_list_i64))
  (local.set $l (call $l64))
  (local.set $alias (local.get $l))
  (drop (call $zhex_list_append_i64 (ref.as_non_null (local.get $alias)) (i64.const 40)))
  (call $zhex_list_length_i64 (local.get $l)))
(func (export "l64_append_active")
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $l64))
  (struct.set $zhex_list_i64 $active (local.get $l) (i32.const 1))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1))))
(func (export "l64_set_active")
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $l64))
  (struct.set $zhex_list_i64 $active (local.get $l) (i32.const 1))
  (call $zhex_list_set_i64 (local.get $l) (i64.const 0) (i64.const 1)))
(func (export "l64_get_active") (result i64)
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $l64))
  (struct.set $zhex_list_i64 $active (local.get $l) (i32.const 1))
  (call $zhex_list_get_i64 (local.get $l) (i64.const 0)))
(func (export "l64_limit")
  (drop (call $zhex_list_append_i64
    (struct.new $zhex_list_i64 (array.new_default $zhex_arr_i64 (i32.const 1000000)) (i32.const 1000000) (i32.const 0))
    (i64.const 1))))

(func $l64_grow (param $n i32) (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local $i i32)
  (local.set $l (call $zhex_list_new_i64))
  (block $done
    (loop $again
      (br_if $done (i32.ge_u (local.get $i) (local.get $n)))
      (drop (call $zhex_list_append_i64 (local.get $l) (i64.extend_i32_u (local.get $i))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $again)))
  (local.get $l))
(func (export "l64_grow_len") (param $n i32) (result i64) (call $zhex_list_length_i64 (call $l64_grow (local.get $n))))
(func (export "l64_grow_ends") (param $n i32) (result i64)
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $l64_grow (local.get $n)))
  (i64.add (call $zhex_list_get_i64 (local.get $l) (i64.const 0))
    (call $zhex_list_get_i64 (local.get $l) (i64.sub (i64.extend_i32_u (local.get $n)) (i64.const 1)))))
(func (export "l64_grow_oob") (param $n i32) (result i64)
  (call $zhex_list_get_i64 (call $l64_grow (local.get $n)) (i64.extend_i32_u (local.get $n))))

(func $l32 (result (ref $zhex_list_i32))
  (local $l (ref $zhex_list_i32))
  (local.set $l (call $zhex_list_new_i32))
  (drop (call $zhex_list_append_i32 (local.get $l) (i32.const 0)))
  (drop (call $zhex_list_append_i32 (local.get $l) (i32.const 1)))
  (local.get $l))
(func (export "l32_len") (result i64) (call $zhex_list_length_i32 (call $l32)))
(func (export "l32_get1") (result i32) (call $zhex_list_get_i32 (call $l32) (i64.const 1)))
(func (export "l32_set_get") (result i32)
  (local $l (ref $zhex_list_i32))
  (local.set $l (call $l32))
  (call $zhex_list_set_i32 (local.get $l) (i64.const 1) (i32.const 1))
  (call $zhex_list_get_i32 (local.get $l) (i64.const 1)))
(func (export "l32_oob") (drop (call $zhex_list_get_i32 (call $l32) (i64.const 2))))

(func $lref (result (ref $zhex_list_ref))
  (local $l (ref $zhex_list_ref))
  (local.set $l (call $zhex_list_new_ref))
  (drop (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 1 (i32.const 97))))
  (drop (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 1 (i32.const 98))))
  (local.get $l))
(func (export "lref_len") (result i64) (call $zhex_list_length_ref (call $lref)))
(func (export "lref_get1_print") (call $zhex_print_str
  (ref.cast (ref $zhex_string) (call $zhex_list_get_ref (call $lref) (i64.const 1)))))

(func $shape2 (param $d0 i64) (param $d1 i64) (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $zhex_list_new_i64))
  (drop (call $zhex_list_append_i64 (local.get $l) (local.get $d0)))
  (drop (call $zhex_list_append_i64 (local.get $l) (local.get $d1)))
  (local.get $l))
(func $shape1 (param $d i64) (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $zhex_list_new_i64))
  (drop (call $zhex_list_append_i64 (local.get $l) (local.get $d)))
  (local.get $l))
(func $shape33 (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $zhex_list_new_i64))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (local.get $l))

(func $a64 (result (ref $zhex_array_i64)) (call $zhex_array_new_i64 (call $shape2 (i64.const 2) (i64.const 3)) (i64.const 7)))
(func (export "a64_len") (result i64) (call $zhex_array_length_i64 (call $a64)))
(func (export "a64_get") (param $c0 i64) (param $c1 i64) (result i64)
  (call $zhex_array_get_i64 (call $a64) (array.new_fixed $zhex_arr_i64 2 (local.get $c0) (local.get $c1))))
(func (export "a64_set_get") (result i64)
  (local $a (ref $zhex_array_i64))
  (local.set $a (call $a64))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 0) (i64.const 1)) (i64.const 5))
  (call $zhex_array_get_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 0) (i64.const 2))))
(func (export "a64_oob") (drop (call $zhex_array_get_i64 (call $a64) (array.new_fixed $zhex_arr_i64 2 (i64.const 1) (i64.const 3)))))
(func (export "a64_neg") (drop (call $zhex_array_get_i64 (call $a64) (array.new_fixed $zhex_arr_i64 2 (i64.const 0) (i64.const -1)))))
(func (export "a64_wrongrank") (drop (call $zhex_array_get_i64 (call $a64) (array.new_fixed $zhex_arr_i64 1 (i64.const 0)))))
(func (export "a64_set_oob") (call $zhex_array_set_i64 (call $a64) (array.new_fixed $zhex_arr_i64 2 (i64.const 2) (i64.const 0)) (i64.const 1)))
(func (export "a64_set_active")
  (local $a (ref $zhex_array_i64))
  (local.set $a (call $a64))
  (struct.set $zhex_array_i64 $active (local.get $a) (i32.const 1))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 0) (i64.const 0)) (i64.const 1)))
(func (export "a64_get_active") (result i64)
  (local $a (ref $zhex_array_i64))
  (local.set $a (call $a64))
  (struct.set $zhex_array_i64 $active (local.get $a) (i32.const 1))
  (call $zhex_array_get_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 1) (i64.const 2))))
(func (export "a64_rank1_len") (result i64) (call $zhex_array_length_i64 (call $zhex_array_new_i64 (call $shape1 (i64.const 5)) (i64.const 0))))
(func (export "a64_rank1_set_get") (result i64)
  (local $a (ref $zhex_array_i64))
  (local.set $a (call $zhex_array_new_i64 (call $shape1 (i64.const 5)) (i64.const 0)))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 1 (i64.const 3)) (i64.const 9))
  (call $zhex_array_get_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 1 (i64.const 3))))
(func (export "a64_rank0") (drop (call $zhex_array_new_i64 (call $zhex_list_new_i64) (i64.const 0))))
(func (export "a64_rank33") (drop (call $zhex_array_new_i64 (call $shape33) (i64.const 0))))
(func (export "a64_negext") (drop (call $zhex_array_new_i64 (call $shape2 (i64.const 2) (i64.const -1)) (i64.const 0))))
(func (export "a64_limit") (drop (call $zhex_array_new_i64 (call $shape2 (i64.const 1001) (i64.const 1001)) (i64.const 0))))
(func (export "a64_zero") (result i64) (call $zhex_array_length_i64 (call $zhex_array_new_i64 (call $shape2 (i64.const 2) (i64.const 0)) (i64.const 0))))
(func (export "a64_exact") (result i64) (call $zhex_array_length_i64 (call $zhex_array_new_i64 (call $shape2 (i64.const 1000) (i64.const 1000)) (i64.const 0))))

(func (export "a32_len") (result i64)
  (call $zhex_array_length_i32 (call $zhex_array_new_i32 (call $shape2 (i64.const 2) (i64.const 3)) (i32.const 1))))
(func (export "a32_set_get") (result i32)
  (local $a (ref $zhex_array_i32))
  (local.set $a (call $zhex_array_new_i32 (call $shape2 (i64.const 2) (i64.const 3)) (i32.const 1)))
  (call $zhex_array_set_i32 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 1) (i64.const 1)) (i32.const 0))
  (call $zhex_array_get_i32 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 1) (i64.const 1))))

(func (export "aref_get_print") (call $zhex_print_str
  (ref.cast (ref $zhex_string)
    (call $zhex_array_get_ref
      (call $zhex_array_new_ref (call $shape2 (i64.const 2) (i64.const 2)) (array.new_fixed $zhex_string 1 (i32.const 120)))
      (array.new_fixed $zhex_arr_i64 2 (i64.const 1) (i64.const 1))))))

(func (export "box_i32_rt") (param $v i32) (result i32) (call $zhex_unbox_i32 (call $zhex_box_i32 (local.get $v))))

(export "memory" (memory $zhex_mem))
(export "zhex_last_error" (func $zhex_last_error))
`

// watPreludeRunner drives the assembled prelude under node. Every case runs
// on a fresh instance (a trap must not poison later cases), env.print decodes
// the staged UTF-8 exactly as the production runner does, and trap cases
// assert the pinned §6a fail code via zhex_last_error (0 = native trap).
const watPreludeRunner = `
import { readFileSync } from 'node:fs';

const bytes = readFileSync(process.argv[2]);
const mod = new WebAssembly.Module(bytes);
let exports = null;
let printed = null;

function fresh() {
  printed = [];
  const env = {
    print(ptr, len) {
      printed.push(new TextDecoder().decode(new Uint8Array(exports.memory.buffer, ptr, len)));
    },
  };
  exports = new WebAssembly.Instance(mod, { env }).exports;
}

const cases = [];
const t = (name, fn) => cases.push({ name, fn });
const eq = (got, want, what) => {
  if (got !== want) throw new Error(what + ': got ' + got + ', want ' + want);
};
const printedEq = (want) => {
  if (printed.length !== want.length || printed.some((s, i) => s !== want[i])) {
    throw new Error('printed ' + JSON.stringify(printed) + ', want ' + JSON.stringify(want));
  }
};
function trap(fn, code) {
  try { fn(); } catch (e) {
    if (!(e instanceof WebAssembly.RuntimeError)) throw new Error('expected RuntimeError, got: ' + e);
    const got = Number(exports.zhex_last_error());
    if (got !== code) throw new Error('trap code ' + got + ', want ' + code + ' (' + e.message + ')');
    return;
  }
  throw new Error('expected a trap, none happened');
}

const MIN = -9223372036854775808n;
const MAX = 9223372036854775807n;

t('last_error fresh is 0', () => { fresh(); eq(Number(exports.zhex_last_error()), 0, 'code'); });

for (const [v, s] of [
  [0n, '0'], [1n, '1'], [-1n, '-1'], [42n, '42'], [-7n, '-7'],
  [MAX, '9223372036854775807'], [MIN, '-9223372036854775808'],
  [1000000007n, '1000000007'], [-1000000007n, '-1000000007'],
]) {
  t('int formatting ' + s, () => { fresh(); exports.print_int(v); printedEq([s]); });
}

t('print unicode text', () => { fresh(); exports.print_str_hello(); printedEq(['h\u00e9llo\u2603']); });
t('print 4-byte emoji', () => { fresh(); exports.print_str_emoji(); printedEq(['\ud83d\ude00']); });
t('print empty string', () => { fresh(); exports.print_str_empty(); printedEq(['']); });

t('str eq same', () => { fresh(); eq(exports.eq_same(), 1, 'eq_same'); });
t('str eq differing tail', () => { fresh(); eq(exports.eq_tail(), 0, 'eq_tail'); });
t('str eq differing length', () => { fresh(); eq(exports.eq_len(), 0, 'eq_len'); });
t('str eq empty empty', () => { fresh(); eq(exports.eq_empty(), 1, 'eq_empty'); });
t('str eq empty a', () => { fresh(); eq(exports.eq_empty_a(), 0, 'eq_empty_a'); });
t('str eq unicode', () => { fresh(); eq(exports.eq_unicode(), 1, 'eq_unicode'); });

t('concat plain', () => { fresh(); exports.cat_plain(); printedEq(['foobar']); });
t('concat empty empty', () => { fresh(); exports.cat_empty(); printedEq(['']); });
t('concat mixed scripts', () => { fresh(); exports.cat_mixed(); printedEq(['h\u00e9lo\u2603']); });

t('depth 10 ok', () => { fresh(); exports.rec(10); });
t('depth 256 ok, leave resets', () => { fresh(); exports.rec(256); exports.rec(200); });
t('depth 257 traps code 2', () => { fresh(); trap(() => exports.rec(257), 2); });

t('add within range', () => { fresh(); eq(exports.add(1n, 2n), 3n, 'add'); });
t('add max+1 traps 1', () => { fresh(); trap(() => exports.add(MAX, 1n), 1); });
t('add min-1 traps 1', () => { fresh(); trap(() => exports.add(MIN, -1n), 1); });
t('sub within range', () => { fresh(); eq(exports.sub(1n, 2n), -1n, 'sub'); });
t('sub min-1 traps 1', () => { fresh(); trap(() => exports.sub(MIN, 1n), 1); });
t('sub max+1 traps 1', () => { fresh(); trap(() => exports.sub(MAX, -1n), 1); });
t('sub 0-min traps 1', () => { fresh(); trap(() => exports.sub(0n, MIN), 1); });
t('mul within range', () => { fresh(); eq(exports.mul(3n, 4n), 12n, 'mul'); });
t('mul -1*min traps 1', () => { fresh(); trap(() => exports.mul(-1n, MIN), 1); });
t('mul min*-1 traps 1', () => { fresh(); trap(() => exports.mul(MIN, -1n), 1); });
t('mul -1*max negates', () => { fresh(); eq(exports.mul(-1n, MAX), -9223372036854775807n, 'mul'); });
t('mul 0*min is 0', () => { fresh(); eq(exports.mul(0n, MIN), 0n, 'mul'); });
t('mul max*2 traps 1', () => { fresh(); trap(() => exports.mul(MAX, 2n), 1); });
t('neg within range', () => { fresh(); eq(exports.neg(5n), -5n, 'neg'); });
t('neg zero', () => { fresh(); eq(exports.neg(0n), 0n, 'neg'); });
t('neg min traps 1', () => { fresh(); trap(() => exports.neg(MIN), 1); });
t('div truncates toward zero', () => { fresh(); eq(exports.div(-7n, 2n), -3n, 'div'); });
t('div truncates toward zero neg', () => { fresh(); eq(exports.div(7n, -2n), -3n, 'div'); });
t('div min/-1 traps 1', () => { fresh(); trap(() => exports.div(MIN, -1n), 1); });
t('div by zero deliberate failure', () => { fresh(); trap(() => exports.div(1n, 0n), 19); });
t('rem min%-1 is 0 natively', () => { fresh(); eq(exports.rem_native(MIN, -1n), 0n, 'rem'); });
t('rem negative', () => { fresh(); eq(exports.rem_native(-7n, 2n), -1n, 'rem'); });

t('list i64 length after appends', () => { fresh(); eq(exports.l64_len(), 3n, 'len'); });
t('list i64 get each', () => {
  fresh();
  eq(exports.l64_get(0n), 10n, 'get 0');
  eq(exports.l64_get(1n), 20n, 'get 1');
  eq(exports.l64_get(2n), 30n, 'get 2');
});
t('list i64 get out of range traps 3', () => { fresh(); trap(() => exports.l64_get(3n), 3); });
t('list i64 get negative traps 3', () => { fresh(); trap(() => exports.l64_get(-1n), 3); });
t('list i64 set then get', () => { fresh(); eq(exports.l64_set_get(1n, 99n), 99n, 'set_get'); });
t('list i64 set out of range traps 3', () => { fresh(); trap(() => exports.l64_set_get(5n, 1n), 3); });
t('list i64 shared alias growth', () => { fresh(); eq(exports.l64_alias(), 4n, 'alias'); });
t('list i64 append while iterating traps 10', () => { fresh(); trap(() => exports.l64_append_active(), 10); });
t('list i64 set while iterating traps 10', () => { fresh(); trap(() => exports.l64_set_active(), 10); });
t('list i64 get while iterating is fine', () => { fresh(); eq(exports.l64_get_active(), 10n, 'get_active'); });
t('list i64 append at limit traps 8', () => { fresh(); trap(() => exports.l64_limit(), 8); });
t('list i64 growth crosses doublings', () => {
  fresh();
  for (const n of [1, 3, 4, 5, 8, 9, 33, 1000]) {
    eq(exports.l64_grow_len(n), BigInt(n), 'len ' + n);
    eq(exports.l64_grow_ends(n), BigInt(n - 1), 'ends ' + n);
  }
});
t('list i64 bounds follow logical length past slack', () => {
  fresh();
  trap(() => exports.l64_grow_oob(5), 3);
});

t('list i32 length', () => { fresh(); eq(exports.l32_len(), 2n, 'len'); });
t('list i32 get', () => { fresh(); eq(exports.l32_get1(), 1, 'get'); });
t('list i32 set then get', () => { fresh(); eq(exports.l32_set_get(), 1, 'set_get'); });
t('list i32 out of range traps 3', () => { fresh(); trap(() => exports.l32_oob(), 3); });

t('list ref length', () => { fresh(); eq(exports.lref_len(), 2n, 'len'); });
t('list ref get prints element', () => { fresh(); exports.lref_get1_print(); printedEq(['b']); });

t('array i64 length is flat count', () => { fresh(); eq(exports.a64_len(), 6n, 'len'); });
t('array i64 get fill', () => {
  fresh();
  eq(exports.a64_get(0n, 0n), 7n, 'get 0,0');
  eq(exports.a64_get(1n, 2n), 7n, 'get 1,2');
});
t('array i64 set offsets do not alias', () => { fresh(); eq(exports.a64_set_get(), 7n, 'set_get'); });
t('array i64 get out of range traps 4', () => { fresh(); trap(() => exports.a64_oob(), 4); });
t('array i64 negative coord traps 4', () => { fresh(); trap(() => exports.a64_neg(), 4); });
t('array i64 wrong rank traps 7', () => { fresh(); trap(() => exports.a64_wrongrank(), 7); });
t('array i64 set out of range traps 4', () => { fresh(); trap(() => exports.a64_set_oob(), 4); });
t('array i64 set while iterating traps 10', () => { fresh(); trap(() => exports.a64_set_active(), 10); });
t('array i64 get while iterating is fine', () => { fresh(); eq(exports.a64_get_active(), 7n, 'get_active'); });
t('array rank 1 length', () => { fresh(); eq(exports.a64_rank1_len(), 5n, 'len'); });
t('array rank 1 set then get', () => { fresh(); eq(exports.a64_rank1_set_get(), 9n, 'set_get'); });
t('array rank 0 traps 5', () => { fresh(); trap(() => exports.a64_rank0(), 5); });
t('array rank 33 traps 5', () => { fresh(); trap(() => exports.a64_rank33(), 5); });
t('array negative extent traps 6', () => { fresh(); trap(() => exports.a64_negext(), 6); });
t('array over limit traps 9', () => { fresh(); trap(() => exports.a64_limit(), 9); });
t('array zero extent is empty', () => { fresh(); eq(exports.a64_zero(), 0n, 'len'); });
t('array exactly at limit is allowed', () => { fresh(); eq(exports.a64_exact(), 1000000n, 'len'); });

t('array i32 length', () => { fresh(); eq(exports.a32_len(), 6n, 'len'); });
t('array i32 set then get', () => { fresh(); eq(exports.a32_set_get(), 0, 'set_get'); });
t('array ref get prints fill', () => { fresh(); exports.aref_get_print(); printedEq(['x']); });

t('box i32 round trip', () => { fresh(); eq(exports.box_i32_rt(1), 1, '1'); });

const failures = [];
for (const c of cases) {
  try { c.fn(); } catch (e) { failures.push({ name: c.name, error: String(e && e.message || e) }); }
}
console.log(JSON.stringify({ total: cases.length, failed: failures }));
if (failures.length) process.exit(1);
`
