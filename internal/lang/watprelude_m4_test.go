package lang

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestWatNamesM4PinnedNames pins the M4 shared surface on tool-less CI:
// the void closure grid rows, the xml element-tree types and entry point,
// the names global, and the checked List read helpers all appear in the
// prelude under the exact spellings watnames_m4.go defines (DESIGN-M4 §10).
func TestWatNamesM4PinnedNames(t *testing.T) {
	pinned := []string{
		"(rec\n  (type " + watTypeXmlElem,
		"(type " + watTypeXmlAttr + " (struct (field $name (ref $zhex_string)) (field $value (ref $zhex_string))))",
		"(global " + watXmlNames + " (mut (ref null $zhex_arr_ref))",
		"(func " + watXmlDecode + " (param $raw (ref $zhex_string))",
	}
	for _, p := range []watSpec{watSpecI64, watSpecI32, watSpecRef} {
		pinned = append(pinned,
			"(type "+watCloType([]watSpec{p}, watCloVoid)+" ",
			"(func "+watListGetChecked(p)+" ",
		)
		for _, p2 := range []watSpec{watSpecI64, watSpecI32, watSpecRef} {
			pinned = append(pinned, "(type "+watCloType([]watSpec{p, p2}, watCloVoid)+" ")
		}
	}
	for _, want := range pinned {
		if !strings.Contains(watPreludeM4, want) {
			t.Errorf("M4 prelude is missing pinned name: %q", want)
		}
	}
	// the M2 quote fix ships beside the M4 prelude; its table must generate
	if !strings.Contains(watPreludeM2, "(global $zhex_print_ranges (ref $zhex_arr_i32)") {
		t.Error("M2 prelude lost the printable table (DESIGN-M4 5)")
	}
	// no new fail codes: the family-5 envelope reuses M3's staged path
	if strings.Contains(watPreludeM4, "call $zhex_fail ") {
		t.Error("M4 failures must be Results through $zjson_fail, never trap codes")
	}
}

// TestWatPreludeM4 assembles the production shape (host imports, the
// seam-stripped prelude quartet, the browser host funcs, then test exports)
// and behavior-drives the M4 families under node: the xml message table
// pinned from the interpreter, the depth edge, the checked List reads, and
// a void closure invoked through the grid's _r_v row. Skipped when
// wasm-tools or node is missing so tool-less CI stays green. Expected
// strings are pinned literals, never derived from Go at test time.
func TestWatPreludeM4(t *testing.T) {
	wasmTools, err := exec.LookPath("wasm-tools")
	if err != nil {
		t.Skipf("wasm-tools not installed: %v", err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node not installed: %v", err)
	}
	dir := t.TempDir()
	wat := "(module\n" + watHostImports(WatSkinBrowser) +
		watStripSeamDefs(watPrelude+watPreludeM2+watPreludeM3+watPreludeM4) +
		watHostFuncs(WatSkinBrowser) + watPreludeM4TestExports + "\n)\n"
	// The WASI skin has no JS host to drive, but the M4 prelude must still
	// assemble and validate there (the else-shape bug precedent, M3).
	wasiWat := "(module\n" + watHostImports(WatSkinWASI) +
		watStripSeamDefs(watPrelude+watPreludeM2+watPreludeM3+watPreludeM4) +
		watHostFuncs(WatSkinWASI) + "(func $zhex_main)\n)\n"
	wasiWatPath := filepath.Join(dir, "prelude-m4-wasi.wat")
	if err := os.WriteFile(wasiWatPath, []byte(wasiWat), 0o644); err != nil {
		t.Fatal(err)
	}
	wasiWasmPath := filepath.Join(dir, "prelude-m4-wasi.wasm")
	if out, err := exec.Command(wasmTools, "parse", wasiWatPath, "-o", wasiWasmPath).CombinedOutput(); err != nil {
		t.Fatalf("wasm-tools parse (wasi skin): %v\n%s", err, out)
	}
	if out, err := exec.Command(wasmTools, "validate", wasiWasmPath).CombinedOutput(); err != nil {
		t.Fatalf("wasm-tools validate (wasi skin): %v\n%s", err, out)
	}
	watPath := filepath.Join(dir, "prelude-m4.wat")
	if err := os.WriteFile(watPath, []byte(wat), 0o644); err != nil {
		t.Fatal(err)
	}
	wasmPath := filepath.Join(dir, "prelude-m4.wasm")
	if out, err := exec.Command(wasmTools, "parse", watPath, "-o", wasmPath).CombinedOutput(); err != nil {
		t.Fatalf("wasm-tools parse: %v\n%s", err, out)
	}
	if out, err := exec.Command(wasmTools, "validate", wasmPath).CombinedOutput(); err != nil {
		t.Fatalf("wasm-tools validate: %v\n%s", err, out)
	}
	mjsPath := filepath.Join(dir, "run.mjs")
	if err := os.WriteFile(mjsPath, []byte(watPreludeM4Runner), 0o644); err != nil {
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
	if report.Total < 50 {
		t.Errorf("runner executed only %d cases; the suite shrank", report.Total)
	}
	for _, f := range report.Failed {
		t.Errorf("case %v failed: %v", f["name"], f["error"])
	}
}

// watPreludeM4TestExports wraps the M4 helpers in exported functions with
// plain numeric signatures; strings enter as UTF-8 bytes the runner writes
// at offset 0 and leave through the staging slot, exactly like the M2/M3
// prelude tests. The Server/Coll/Eps record schemas mirror the interpreter
// tests' fixtures through hand-written structs and make/get thunks, exactly
// as the emitter generates them per call site.
const watPreludeM4TestExports = `
(export "memory" (memory $zhex_mem))

;; ---- record fixtures: Server(name, retries, verbose), Coll(ports, tags), Eps(eps)
(type $t_srv (struct (field $f0 (ref $zhex_string)) (field $f1 i64) (field $f2 i32)))
(type $t_coll (struct (field $f0 (ref null $zhex_list_i64)) (field $f1 (ref null $zhex_map_str_i64))))
(type $t_eps (struct (field $f0 (ref null $zhex_list_ref))))

(func $t_srv_make (type $zjson_make_fn) (param $f (ref $zhex_arr_ref)) (result (ref null eq))
  (struct.new $t_srv
    (ref.cast (ref $zhex_string) (array.get $zhex_arr_ref (local.get $f) (i32.const 0)))
    (struct.get $zhex_some_i64 $value (ref.cast (ref $zhex_some_i64) (array.get $zhex_arr_ref (local.get $f) (i32.const 1))))
    (i31.get_u (ref.cast (ref i31) (array.get $zhex_arr_ref (local.get $f) (i32.const 2))))))
(func $t_srv_get (type $zjson_get_fn) (param $v (ref null eq)) (result (ref null $zhex_arr_ref))
  (array.new_fixed $zhex_arr_ref 3
    (struct.get $t_srv $f0 (ref.cast (ref $t_srv) (local.get $v)))
    (struct.new $zhex_some_i64 (struct.get $t_srv $f1 (ref.cast (ref $t_srv) (local.get $v))))
    (ref.i31 (struct.get $t_srv $f2 (ref.cast (ref $t_srv) (local.get $v))))))
(func $t_coll_make (type $zjson_make_fn) (param $f (ref $zhex_arr_ref)) (result (ref null eq))
  (struct.new $t_coll
    (ref.cast (ref $zhex_list_i64) (array.get $zhex_arr_ref (local.get $f) (i32.const 0)))
    (ref.cast (ref $zhex_map_str_i64) (array.get $zhex_arr_ref (local.get $f) (i32.const 1)))))
(func $t_coll_get (type $zjson_get_fn) (param $v (ref null eq)) (result (ref null $zhex_arr_ref))
  (array.new_fixed $zhex_arr_ref 2
    (struct.get $t_coll $f0 (ref.cast (ref $t_coll) (local.get $v)))
    (struct.get $t_coll $f1 (ref.cast (ref $t_coll) (local.get $v)))))
(func $t_eps_make (type $zjson_make_fn) (param $f (ref $zhex_arr_ref)) (result (ref null eq))
  (struct.new $t_eps (ref.cast (ref $zhex_list_ref) (array.get $zhex_arr_ref (local.get $f) (i32.const 0)))))
(func $t_eps_get (type $zjson_get_fn) (param $v (ref null eq)) (result (ref null $zhex_arr_ref))
  (array.new_fixed $zhex_arr_ref 1 (struct.get $t_eps $f0 (ref.cast (ref $t_eps) (local.get $v)))))
(elem declare func $t_srv_make $t_srv_get $t_coll_make $t_coll_get $t_eps_make $t_eps_get)

(func $t_in (param $off i32) (param $len i32) (result (ref $zhex_string))
  (call $zhex_bytes_to_string (local.get $off) (local.get $len)))

;; register one schema node's type name in the M4 names table, exactly as
;; the emitter's per-call-site builders do (DESIGN-M4 2b)
(func $t_reg (param $s (ref null eq)) (param $name (ref $zhex_string))
  (global.set $zxml_names (call $zjson_arr_push (global.get $zxml_names) (local.get $s)))
  (global.set $zxml_names (call $zjson_arr_push (global.get $zxml_names) (local.get $name))))

(func $t_srv_schema (result (ref $zjson_schema))
  (local $str (ref null $zjson_schema)) (local $int (ref null $zjson_schema)) (local $bool (ref null $zjson_schema))
  (local $rec (ref null $zjson_schema))
  (local.set $str (struct.new $zjson_schema (i32.const 2) (ref.null $zjson_schema) (ref.null $zhex_arr_ref) (ref.null $zjson_make_fn) (ref.null $zjson_get_fn)))
  (local.set $int (struct.new $zjson_schema (i32.const 0) (ref.null $zjson_schema) (ref.null $zhex_arr_ref) (ref.null $zjson_make_fn) (ref.null $zjson_get_fn)))
  (local.set $bool (struct.new $zjson_schema (i32.const 1) (ref.null $zjson_schema) (ref.null $zhex_arr_ref) (ref.null $zjson_make_fn) (ref.null $zjson_get_fn)))
  (call $t_reg (local.get $str) (array.new_fixed $zhex_string 6 (i32.const 83) (i32.const 116) (i32.const 114) (i32.const 105) (i32.const 110) (i32.const 103)))
  (call $t_reg (local.get $int) (array.new_fixed $zhex_string 3 (i32.const 73) (i32.const 110) (i32.const 116)))
  (call $t_reg (local.get $bool) (array.new_fixed $zhex_string 4 (i32.const 66) (i32.const 111) (i32.const 111) (i32.const 108)))
  (local.set $rec (struct.new $zjson_schema (i32.const 6) (ref.null $zjson_schema)
    (array.new_fixed $zhex_arr_ref 3
      (struct.new $zjson_field (array.new_fixed $zhex_string 4 (i32.const 110) (i32.const 97) (i32.const 109) (i32.const 101)) (ref.as_non_null (local.get $str)))
      (struct.new $zjson_field (array.new_fixed $zhex_string 7 (i32.const 114) (i32.const 101) (i32.const 116) (i32.const 114) (i32.const 105) (i32.const 101) (i32.const 115)) (ref.as_non_null (local.get $int)))
      (struct.new $zjson_field (array.new_fixed $zhex_string 7 (i32.const 118) (i32.const 101) (i32.const 114) (i32.const 98) (i32.const 111) (i32.const 115) (i32.const 101)) (ref.as_non_null (local.get $bool))))
    (ref.func $t_srv_make) (ref.func $t_srv_get)))
  (call $t_reg (local.get $rec) (array.new_fixed $zhex_string 6 (i32.const 83) (i32.const 101) (i32.const 114) (i32.const 118) (i32.const 101) (i32.const 114)))
  (ref.as_non_null (local.get $rec)))

(func $t_srv_defaults (result (ref null eq))
  (struct.new $t_srv (array.new_fixed $zhex_string 1 (i32.const 120)) (i64.const 1) (i32.const 0)))

(func $t_coll_schema (result (ref $zjson_schema))
  (local $int (ref null $zjson_schema)) (local $ints (ref null $zjson_schema)) (local $mapi (ref null $zjson_schema)) (local $rec (ref null $zjson_schema))
  (local.set $int (struct.new $zjson_schema (i32.const 0) (ref.null $zjson_schema) (ref.null $zhex_arr_ref) (ref.null $zjson_make_fn) (ref.null $zjson_get_fn)))
  (call $t_reg (local.get $int) (array.new_fixed $zhex_string 3 (i32.const 73) (i32.const 110) (i32.const 116)))
  (local.set $ints (struct.new $zjson_schema (i32.const 4) (ref.as_non_null (local.get $int)) (ref.null $zhex_arr_ref) (ref.null $zjson_make_fn) (ref.null $zjson_get_fn)))
  (local.set $mapi (struct.new $zjson_schema (i32.const 5) (ref.as_non_null (local.get $int)) (ref.null $zhex_arr_ref) (ref.null $zjson_make_fn) (ref.null $zjson_get_fn)))
  (call $t_reg (local.get $ints) (array.new_fixed $zhex_string 9 (i32.const 76) (i32.const 105) (i32.const 115) (i32.const 116) (i32.const 60) (i32.const 73) (i32.const 110) (i32.const 116) (i32.const 62)))
  (local.set $rec (struct.new $zjson_schema (i32.const 6) (ref.null $zjson_schema)
    (array.new_fixed $zhex_arr_ref 2
      (struct.new $zjson_field (array.new_fixed $zhex_string 5 (i32.const 112) (i32.const 111) (i32.const 114) (i32.const 116) (i32.const 115)) (ref.as_non_null (local.get $ints)))
      (struct.new $zjson_field (array.new_fixed $zhex_string 4 (i32.const 116) (i32.const 97) (i32.const 103) (i32.const 115)) (ref.as_non_null (local.get $mapi))))
    (ref.func $t_coll_make) (ref.func $t_coll_get)))
  (call $t_reg (local.get $rec) (array.new_fixed $zhex_string 4 (i32.const 67) (i32.const 111) (i32.const 108) (i32.const 108)))
  (ref.as_non_null (local.get $rec)))

(func $t_coll_defaults (result (ref null eq))
  (struct.new $t_coll
    (struct.new $zhex_list_i64 (array.new_default $zhex_arr_i64 (i32.const 0)) (i32.const 0))
    (call $zhex_map_new_str_i64)))

(func $t_eps_schema (result (ref $zjson_schema))
  (local $srv (ref null $zjson_schema)) (local $list (ref null $zjson_schema))
  (local.set $srv (call $t_srv_schema))
  (local.set $list (struct.new $zjson_schema (i32.const 4) (ref.as_non_null (local.get $srv)) (ref.null $zhex_arr_ref) (ref.null $zjson_make_fn) (ref.null $zjson_get_fn)))
  (struct.new $zjson_schema (i32.const 6) (ref.null $zjson_schema)
    (array.new_fixed $zhex_arr_ref 1
      (struct.new $zjson_field (array.new_fixed $zhex_string 3 (i32.const 101) (i32.const 112) (i32.const 115)) (ref.as_non_null (local.get $list))))
    (ref.func $t_eps_make) (ref.func $t_eps_get)))

(func (export "t_xml_tag") (param $len i32) (result i32)
  (struct.get $zhex_result $tag
    (call $zhex_xml_decode (call $t_in (i32.const 0) (local.get $len)) (call $t_srv_defaults) (call $t_srv_schema))))
(func (export "t_xml_msg") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload
            (call $zhex_xml_decode (call $t_in (i32.const 0) (local.get $len)) (call $t_srv_defaults) (call $t_srv_schema))))))
    (i32.const 0)))
(func (export "t_xml_coll_msg") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload
            (call $zhex_xml_decode (call $t_in (i32.const 0) (local.get $len)) (call $t_coll_defaults) (call $t_coll_schema))))))
    (i32.const 0)))
(func (export "t_xml_eps_msg") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload
            (call $zhex_xml_decode (call $t_in (i32.const 0) (local.get $len))
              (struct.new $t_eps (struct.new $zhex_list_ref (array.new_default $zhex_arr_ref (i32.const 0)) (i32.const 0)))
              (call $t_eps_schema))))))
    (i32.const 0)))
(func (export "t_parse_tag") (param $len i32) (result i32)
  (struct.get $zhex_result $tag (call $zxml_parse (call $t_in (i32.const 0) (local.get $len)))))
(func (export "t_parse_msg") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload (call $zxml_parse (call $t_in (i32.const 0) (local.get $len)))))))
    (i32.const 0)))
(func (export "t_xml_name") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (struct.get $t_srv $f0
      (ref.cast (ref $t_srv)
        (struct.get $zhex_result $payload
          (call $zhex_xml_decode (call $t_in (i32.const 0) (local.get $len)) (call $t_srv_defaults) (call $t_srv_schema)))))
    (i32.const 0)))
(func (export "t_xml_retries") (param $len i32) (result i64)
  (struct.get $t_srv $f1
    (ref.cast (ref $t_srv)
      (struct.get $zhex_result $payload
        (call $zhex_xml_decode (call $t_in (i32.const 0) (local.get $len)) (call $t_srv_defaults) (call $t_srv_schema))))))

;; checked List reads: ok value, family, and the pinned message
(func (export "t_ck_i64_tag") (param $i i64) (result i32)
  (struct.get $zhex_result $tag (call $zhex_list_get_checked_i64 (call $t_l12) (local.get $i))))
(func (export "t_ck_i64_val") (param $i i64) (result i64)
  (struct.get $zhex_some_i64 $value
    (ref.cast (ref $zhex_some_i64)
      (struct.get $zhex_result $payload (call $zhex_list_get_checked_i64 (call $t_l12) (local.get $i))))))
(func (export "t_ck_i64_msg") (param $i i64) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload (call $zhex_list_get_checked_i64 (call $t_l12) (local.get $i))))))
    (i32.const 0)))
(func (export "t_ck_i64_family") (param $i i64) (result i32)
  (struct.get $zhex_err $family
    (ref.cast (ref $zhex_err)
      (struct.get $zhex_result $payload (call $zhex_list_get_checked_i64 (call $t_l12) (local.get $i))))))
(func $t_l12 (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $zhex_list_new_i64))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 1))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 2))
  (local.get $l))

;; a void closure invoked through the grid's _r_v row prints its argument
(func $t_void_thunk (param $clo (ref $zhex_closure)) (param $x i64)
  (call $zhex_print_line (call $zhex_int_to_string (local.get $x))))
(func (export "t_clo_void") (param $x i64)
  (local $c (ref $zhex_closure))
  (local.set $c (struct.new $zhex_closure (ref.func $t_void_thunk) (array.new_fixed $zhex_arr_ref 0)))
  (local.get $c)
  (local.get $x)
  (local.get $c)
  (struct.get $zhex_closure $fn)
  (ref.cast (ref $zhex_clo_i64_r_v))
  (call_ref $zhex_clo_i64_r_v))
(elem declare func $t_void_thunk)
`

// watPreludeM4Runner drives the assembled module under node, one fresh
// instance per case, against the fifteen-stub browser surface. Every
// expected string is a pinned literal from the interpreter on this
// toolchain (xml_codec.go + Go 1.27 encoding/xml).
const watPreludeM4Runner = `
import { readFileSync } from 'node:fs';

const bytes = readFileSync(process.argv[2]);
const mod = new WebAssembly.Module(bytes);
let exports = null;
let printed = null;

const enc = new TextEncoder();

function fresh() {
  printed = [];
  exports = new WebAssembly.Instance(mod, { env: {
    print(ptr, len) { printed.push(new TextDecoder().decode(new Uint8Array(exports.memory.buffer, ptr, len))); },
    stdin_read: () => 0, fs_read_file: () => 44, fs_write_file: () => 0, fs_open_read: () => -44,
    fs_read_line: () => 0, fs_close: () => 0, fs_stat: () => -44, fs_list: () => -44,
    fs_make_folder: () => 28, fs_delete: () => -44, fs_delete_folder: () => -44, fs_move: () => -44,
    env_get: () => 0, env_environ: () => 0,
  } }).exports;
}

const cases = [];
const t = (name, fn) => cases.push({ name, fn });
const eq = (got, want, what) => { if (got !== want) throw new Error(what + ': got ' + JSON.stringify(got) + ', want ' + JSON.stringify(JSON.stringify(want))); };
const put = (s) => { const b = enc.encode(s); new Uint8Array(exports.memory.buffer).set(b, 0); return b.length; };
const outStr = (len) => new TextDecoder().decode(new Uint8Array(exports.memory.buffer, 0, len));
const msg = (op, s, ...rest) => { put(s); return outStr(exports[op](enc.encode(s).length, ...rest)); };

// ---- decode failures: the message table pinned from the interpreter ----
for (const [raw, want] of [
  ['<server><ghost>1</ghost></server>', 'unknown element "ghost" at "/ghost"'],
  ['<server ghost="1"/>', 'unknown attribute "ghost" at "/ghost"'],
  ['<server name="a"><name>b</name></server>', 'element "name" collides with an attribute at "/name"'],
  ['<server><name>a</name><name>b</name></server>', 'duplicate element "name" at "/name"'],
  ['<server>text<name>x</name></server>', 'mixed content is not supported at ""'],
  ['<server><retries>soon</retries></server>', 'expected Int at "/retries"'],
  ['<server retries="99999999999999999999"/>', 'expected an integer-form signed 64-bit number at "/retries"'],
  ['<server retries="9223372036854775808"/>', 'expected an integer-form signed 64-bit number at "/retries"'],
  ['<server verbose="yes"/>', 'expected Bool at "/verbose"'],
  ['<server><name>a</name><retries b="1">7</retries></server>', 'expected Int at "/retries"'],
  ['<server><name>a</name><retries><n>7</n></retries></server>', 'expected Int at "/retries"'],
  ['<d>&x;</d>', 'invalid XML: XML syntax error on line 1: invalid character entity &x; at ""'],
  ['<d>&amp</d>', 'invalid XML: XML syntax error on line 1: invalid character entity &amp (no semicolon) at ""'],
  ['<d>&#65</d>', 'invalid XML: XML syntax error on line 1: invalid character entity &#65 (no semicolon) at ""'],
  ['<d>&#;</d>', 'invalid XML: XML syntax error on line 1: invalid character entity &#; at ""'],
  ['<d>&#x;</d>', 'invalid XML: XML syntax error on line 1: invalid character entity &#x; at ""'],
  ['<d>&#x110000;</d>', 'invalid XML: XML syntax error on line 1: invalid character entity &#x110000; at ""'],
  ['<d>&#1114112;</d>', 'invalid XML: XML syntax error on line 1: invalid character entity &#1114112; at ""'],
  ['<d>&#99999999999999999999;</d>', 'invalid XML: XML syntax error on line 1: invalid character entity &#99999999999999999999; at ""'],
  ['<d>& y</d>', 'invalid XML: XML syntax error on line 1: invalid character entity & (no semicolon) at ""'],
  ['<d>&#0;</d>', 'invalid XML: XML syntax error on line 1: illegal character code U+0000 at ""'],
  ['<d>&#xFFFE;</d>', 'invalid XML: XML syntax error on line 1: illegal character code U+FFFE at ""'],
  ['<d>&#xFFFF;</d>', 'invalid XML: XML syntax error on line 1: illegal character code U+FFFF at ""'],
  ['<d>&#x0;</d>', 'invalid XML: XML syntax error on line 1: illegal character code U+0000 at ""'],
  ['<!DOCTYPE doc><d/>', 'DTD and entity declarations are rejected at ""'],
  ['<!ENTITY x "y"><d/>', 'DTD and entity declarations are rejected at ""'],
  ['<d><?pi x?></d>', 'processing instruction outside the prolog at ""'],
  ['<d/><?pi x?>', 'processing instruction outside the prolog at ""'],
  ['<a/><b/>', 'multiple root elements at "/"'],
  ['', 'no root element at ""'],
  ['<d><b></d>', 'invalid XML: XML syntax error on line 1: element <b> closed by </d> at ""'],
  ['<d>\n<b></d>', 'invalid XML: XML syntax error on line 2: element <b> closed by </d> at ""'],
  ['</d>', 'invalid XML: XML syntax error on line 1: unexpected end element </d> at ""'],
  ['<d>', 'invalid XML: XML syntax error on line 1: unexpected EOF at ""'],
  ['<d><b>x', 'invalid XML: XML syntax error on line 1: unexpected EOF at ""'],
  ['<a:b/>', 'namespace-qualified name "b" is not supported at "/"'],
  ['<d b:c="1"/>', 'namespace-qualified name "c" is not supported at "//c"'],
  ['<d xmlns="http://x"/>', 'namespace-qualified name "d" is not supported at "/"'],
  ['<d xmlns:p="http://x"/>', 'namespace-qualified name "p" is not supported at "//p"'],
  ['<d><![CDAT[x]]></d>', 'invalid XML: XML syntax error on line 1: invalid <![ sequence at ""'],
  ['<d/ >', 'invalid XML: XML syntax error on line 1: expected /> in element at ""'],
  ['<d a=/>', 'invalid XML: XML syntax error on line 1: unquoted or missing attribute value in element at ""'],
  ['<d a b/>', 'invalid XML: XML syntax error on line 1: attribute name without = in element at ""'],
  ['<1d/>', 'invalid XML: XML syntax error on line 1: invalid XML name: 1d at ""'],
  ['< d/>', 'invalid XML: XML syntax error on line 1: expected element name after < at ""'],
  ['<d>&#xD800;</d>\n', null], // surrogate refs decode as U+FFFD: tagged below
]) {
  if (want === null) continue;
  t('xml err ' + JSON.stringify(raw), () => {
    fresh(); put(raw);
    eq(exports.t_xml_tag(enc.encode(raw).length), 1, 'tag');
    eq(msg('t_xml_msg', raw), want, 'msg');
  });
}
for (const [raw, want] of [
  ['<c><tags><a>1</a><a>2</a></tags></c>', 'duplicate map key at "/tags/a"'],
  ['<c><ports>1</ports>x</c>', 'mixed content is not supported at ""'],
  // a list field's items decode as elements: the attr surfaces at /ports/0
  ['<c><ports id="1">1</ports></c>', 'expected Int at "/ports/0"'],
]) {
  t('xml coll err ' + JSON.stringify(raw), () => {
    fresh(); put(raw);
    eq(exports.t_xml_tag(enc.encode(raw).length), 1, 'tag');
    eq(msg('t_xml_coll_msg', raw), want, 'msg');
  });
}
for (const [raw, want] of [
  // list elements decode with no defaults: a missing record field fails
  ['<e><eps><name>a</name></eps></e>', 'missing required field "retries" at "/eps/0/retries"'],
]) {
  t('xml eps err ' + JSON.stringify(raw), () => {
    fresh(); put(raw);
    eq(exports.t_xml_tag(enc.encode(raw).length), 1, 'tag');
    eq(msg('t_xml_eps_msg', raw), want, 'msg');
  });
}

// ---- decode happy paths --------------------------------------------
t('xml record fields, attrs, defaults fill, whitespace-padded Int', () => {
  fresh();
  const raw = '<server retries="  3  "><name>a &amp; b</name></server>';
  put(raw);
  eq(exports.t_xml_tag(enc.encode(raw).length), 0, 'tag');
  eq(msg('t_xml_name', raw), 'a & b', 'name');
  put(raw);
  eq(exports.t_xml_retries(enc.encode(raw).length), 3n, 'retries');
});
t('xml numeric surrogate ref decodes as U+FFFD', () => {
  fresh();
  const raw = '<server><name>&#xD800;x</name></server>';
  put(raw);
  eq(exports.t_xml_tag(enc.encode(raw).length), 0, 'tag');
  eq(msg('t_xml_name', raw), '\ufffdx', 'replacement char');
});
t('xml depth 255 parses, 257 fails with the pinned path', () => {
  fresh();
  const ok = '<a>'.repeat(255) + 'x' + '</a>'.repeat(255);
  put(ok);
  eq(exports.t_parse_tag(enc.encode(ok).length), 0, '255 nesting parses');
  fresh();
  const edge = '<a>'.repeat(256) + 'x' + '</a>'.repeat(256);
  put(edge);
  eq(exports.t_parse_tag(enc.encode(edge).length), 0, '256 nesting parses (the interpreter edge)');
  fresh();
  const deep = '<a>'.repeat(257) + '</a>'.repeat(257);
  put(deep);
  eq(exports.t_parse_tag(enc.encode(deep).length), 1, 'tag');
  eq(outStr(exports.t_parse_msg(enc.encode(deep).length)), 'XML nesting limit exceeded at "' + '/a'.repeat(256) + '"', 'path is the 256 open ancestors');
});

// ---- checked List reads (DESIGN-M4 4) -------------------------------
t('checked read in bounds and out, family 5', () => {
  fresh();
  eq(exports.t_ck_i64_tag(0n), 0, 'in tag');
  eq(exports.t_ck_i64_val(0n), 1n, 'val 0');
  eq(exports.t_ck_i64_val(1n), 2n, 'val 1');
  eq(exports.t_ck_i64_tag(2n), 1, 'out tag');
  eq(exports.t_ck_i64_tag(-1n), 1, 'negative tag');
  eq(outStr(exports.t_ck_i64_msg(2n)), 'list index is out of range', 'msg');
  eq(exports.t_ck_i64_family(2n), 5, 'family');
});

// ---- void closure through the grid's _r_v row (DESIGN-M4 3a) --------
t('void closure call_ref prints and returns nothing', () => {
  fresh();
  exports.t_clo_void(41n);
  eq(printed.join(''), '41\n', 'printed with newline, no result consumed');
});

let total = 0;
const failed = [];
for (const c of cases) {
  total++;
  try { c.fn(); } catch (e) { failed.push({ name: c.name, error: String(e && e.message || e) }); }
}
console.log(JSON.stringify({ total, failed }));
if (failed.length) process.exit(1);
`
