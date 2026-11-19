package lang

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestWatNamesM3PinnedNames pins the shared M3 surface on tool-less CI:
// every $zhex_host_* name appears in both skin layers (browser as imports,
// WASI as adapters), the seam strip removes exactly the three replaced
// definitions, and watnames_m3.go's constants spell the names the prelude
// defines (DESIGN-M3 §1/§10).
func TestWatNamesM3PinnedNames(t *testing.T) {
	for _, name := range []string{
		watHostPrint, watHostStdinRead, watHostFsReadFile, watHostFsWriteFile,
		watHostFsOpenRead, watHostFsReadLine, watHostFsClose, watHostFsStat,
		watHostFsList, watHostFsMakeFolder, watHostFsDelete, watHostFsDeleteFolder,
		watHostFsMove, watHostEnvGet, watHostEnvEnviron,
	} {
		if !strings.Contains(watBrowserImports, "(func "+name+" ") {
			t.Errorf("browser imports missing %s", name)
		}
		if !strings.Contains(watWasiFuncs, "(func "+name+" ") {
			t.Errorf("wasi adapters missing %s", name)
		}
		// env_get is the one 1a import the prelude never calls: env state
		// is module-side (7), seeded from env_environ, so env.get is a map
		// lookup; the import exists for host-surface parity.
		if name != watHostEnvGet && !strings.Contains(watPreludeM3, name) {
			t.Errorf("prelude never calls %s", name)
		}
	}
	// both skins redefine the swapped seam
	for _, layer := range []string{watBrowserFuncs, watWasiFuncs} {
		for _, def := range []string{"(func $env.print ", "(func $zhex_fail ", "(func $zhex_fail_msg "} {
			if !strings.Contains(layer, def) {
				t.Errorf("host layer missing seam redefinition %q", def)
			}
		}
	}
	if !strings.Contains(watWasiFuncs, `(export "_start")`) {
		t.Error("wasi layer must export _start")
	}
	// the strip removes the M1 import line and both function defs, keeps
	// everything else, and is idempotent
	pre := watPrelude + watPreludeM2 + watPreludeM3
	stripped := watStripSeamDefs(pre)
	if strings.Contains(stripped, watM1EnvPrintImport) {
		t.Error("strip left the env.print import behind")
	}
	if strings.Contains(stripped, "(func $zhex_fail ") || strings.Contains(stripped, "(func $zhex_fail_msg ") {
		t.Error("strip left a replaced function behind")
	}
	for _, keep := range []string{"(func $zhex_enter", "(func $zhex_last_error", "(func $zhex_stage_utf8 ", "$zhex_host_fs_read_file"} {
		if !strings.Contains(stripped, keep) {
			t.Errorf("strip removed %q", keep)
		}
	}
	if again := watStripSeamDefs(stripped); again != stripped {
		t.Error("strip is not idempotent")
	}
}

// TestWatPreludeM3 assembles the production shape — host imports, the
// seam-stripped prelude trio, the browser host funcs, then test exports —
// and behavior-drives the M3 families under node against a JS host that
// implements §1a (in-memory FS, injected env, string stdin). Skipped when
// wasm-tools or node is missing so tool-less CI stays green. Expected
// strings are pinned literals (never derived from Go's stdlib at test
// time); exotic cross-stdlib cases live in T's differential fixtures.
func TestWatPreludeM3(t *testing.T) {
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
		watStripSeamDefs(watPrelude+watPreludeM2+watPreludeM3) +
		watHostFuncs(WatSkinBrowser) + watPreludeM3TestExports + "\n)\n"
	// The WASI skin has no JS host to drive, but its adapter layer must still
	// assemble and validate — the else-shape bugs lived only there because the
	// browser shell used to be the only one this test built.
	wasiWat := "(module\n" + watHostImports(WatSkinWASI) +
		watStripSeamDefs(watPrelude+watPreludeM2+watPreludeM3) +
		watHostFuncs(WatSkinWASI) + "(func $zhex_main)\n)\n"
	wasiWatPath := filepath.Join(dir, "prelude-m3-wasi.wat")
	if err := os.WriteFile(wasiWatPath, []byte(wasiWat), 0o644); err != nil {
		t.Fatal(err)
	}
	wasiWasmPath := filepath.Join(dir, "prelude-m3-wasi.wasm")
	if out, err := exec.Command(wasmTools, "parse", wasiWatPath, "-o", wasiWasmPath).CombinedOutput(); err != nil {
		t.Fatalf("wasm-tools parse (wasi skin): %v\n%s", err, out)
	}
	if out, err := exec.Command(wasmTools, "validate", wasiWasmPath).CombinedOutput(); err != nil {
		t.Fatalf("wasm-tools validate (wasi skin): %v\n%s", err, out)
	}
	watPath := filepath.Join(dir, "prelude-m3.wat")
	if err := os.WriteFile(watPath, []byte(wat), 0o644); err != nil {
		t.Fatal(err)
	}
	wasmPath := filepath.Join(dir, "prelude-m3.wasm")
	if out, err := exec.Command(wasmTools, "parse", watPath, "-o", wasmPath).CombinedOutput(); err != nil {
		t.Fatalf("wasm-tools parse: %v\n%s", err, out)
	}
	if out, err := exec.Command(wasmTools, "validate", wasmPath).CombinedOutput(); err != nil {
		t.Fatalf("wasm-tools validate: %v\n%s", err, out)
	}
	mjsPath := filepath.Join(dir, "run.mjs")
	if err := os.WriteFile(mjsPath, []byte(watPreludeM3Runner), 0o644); err != nil {
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
	if report.Total < 100 {
		t.Errorf("runner executed only %d cases; the suite shrank", report.Total)
	}
	for _, f := range report.Failed {
		t.Errorf("case %v failed: %v", f["name"], f["error"])
	}
}

// watPreludeM3TestExports wraps the M3 helpers in exported functions with
// plain numeric signatures; strings enter as UTF-8 bytes the runner writes
// at fixed memory offsets and leave through the staging slot at offset 0.
const watPreludeM3TestExports = `
(export "memory" (memory $zhex_mem))

(func $t_in (param $off i32) (param $len i32) (result (ref $zhex_string))
  (call $zhex_bytes_to_string (local.get $off) (local.get $len)))

(func (export "t_errno") (param $e i32) (result i32)
  (call $zhex_stage_utf8_at (call $zhex_errno_text (local.get $e)) (i32.const 0)))
(func (export "t_os_err") (param $plen i32) (param $vlen i32) (param $e i32) (result i32)
  (call $zhex_stage_utf8_at
    (call $zhex_os_err (call $t_in (i32.const 4096) (local.get $vlen)) (call $t_in (i32.const 0) (local.get $plen)) (local.get $e))
    (i32.const 0)))
(func (export "t_clean") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at (call $zhex_path_clean (call $t_in (i32.const 0) (local.get $len))) (i32.const 0)))
(func (export "t_join") (param $la i32) (param $lb i32) (result i32)
  (call $zhex_stage_utf8_at
    (call $zhex_path_join (call $t_in (i32.const 0) (local.get $la)) (call $t_in (i32.const 4096) (local.get $lb)))
    (i32.const 0)))
(func (export "t_base") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at (call $zhex_path_base (call $t_in (i32.const 0) (local.get $len))) (i32.const 0)))
(func (export "t_ext") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at (call $zhex_path_extension (call $t_in (i32.const 0) (local.get $len))) (i32.const 0)))
(func (export "t_dir") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at (call $zhex_path_dir_of (call $t_in (i32.const 0) (local.get $len))) (i32.const 0)))

(func (export "t_dur_fmt") (param $v i64) (result i32)
  (call $zhex_stage_utf8_at (call $zhex_duration_to_string (local.get $v)) (i32.const 0)))
(func (export "t_dur_tag") (param $len i32) (result i32)
  (struct.get $zhex_result $tag (call $zhex_duration_parse (call $t_in (i32.const 0) (local.get $len)))))
(func (export "t_dur_val") (param $len i32) (result i64)
  (struct.get $zhex_some_i64 $value
    (ref.cast (ref $zhex_some_i64)
      (struct.get $zhex_result $payload (call $zhex_duration_parse (call $t_in (i32.const 0) (local.get $len)))))))
(func (export "t_dur_msg") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload (call $zhex_duration_parse (call $t_in (i32.const 0) (local.get $len)))))))
    (i32.const 0)))

(func (export "t_buf_count") (param $b (ref $zhex_buffer)) (result i64)
  (call $zhex_buffer_line_count (local.get $b)))
(func (export "t_buf_add") (param $b (ref $zhex_buffer)) (param $len i32) (result (ref $zhex_buffer))
  (drop (call $zhex_buffer_add_line (local.get $b) (call $t_in (i32.const 0) (local.get $len))))
  (local.get $b))
(func (export "t_buf_to_text") (param $b (ref $zhex_buffer)) (result i32)
  (call $zhex_stage_utf8_at (call $zhex_buffer_to_text (local.get $b)) (i32.const 0)))
(func (export "t_buf_line_tag") (param $b (ref $zhex_buffer)) (param $i i64) (result i32)
  (struct.get $zhex_result $tag (call $zhex_buffer_line (local.get $b) (local.get $i))))
(func (export "t_buf_line_val") (param $b (ref $zhex_buffer)) (param $i i64) (result i32)
  (call $zhex_stage_utf8_at
    (ref.cast (ref $zhex_string)
      (struct.get $zhex_result $payload (call $zhex_buffer_line (local.get $b) (local.get $i))))
    (i32.const 0)))
(func (export "t_buf_line_msg") (param $b (ref $zhex_buffer)) (param $i i64) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload (call $zhex_buffer_line (local.get $b) (local.get $i))))))
    (i32.const 0)))
(func (export "t_buf_new") (result (ref $zhex_buffer)) (call $zhex_buffer_new))
(func (export "t_buf_set") (param $b (ref $zhex_buffer)) (param $i i64) (param $len i32) (result i32)
  (struct.get $zhex_result $tag
    (call $zhex_buffer_set_line (local.get $b) (local.get $i) (call $t_in (i32.const 0) (local.get $len)))))

(func (export "t_rl_tag") (param $len i32) (result i32)
  (struct.get $zhex_result $tag (call $zhex_read_line (call $t_in (i32.const 0) (local.get $len)))))
(func (export "t_rl_val") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.cast (ref $zhex_string)
      (struct.get $zhex_result $payload (call $zhex_read_line (call $t_in (i32.const 0) (local.get $len)))))
    (i32.const 0)))
(func (export "t_rl_msg") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload (call $zhex_read_line (call $t_in (i32.const 0) (local.get $len)))))))
    (i32.const 0)))

(func (export "t_env_init") (call $zhex_env_init))
(func (export "t_env_payload_err") (param $len i32) (result i32)
  (ref.test (ref $zhex_err)
    (struct.get $zhex_result $payload (call $zhex_env_load (call $t_in (i32.const 0) (local.get $len))))))
(func (export "t_env_payload_null") (param $len i32) (result i32)
  (ref.is_null
    (struct.get $zhex_result $payload (call $zhex_env_load (call $t_in (i32.const 0) (local.get $len))))))
(func (export "t_env_some") (param $len i32) (result i32)
  (ref.test (ref $zhex_some_ref) (call $zhex_env_get (call $t_in (i32.const 0) (local.get $len)))))
(func (export "t_env_val") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.cast (ref $zhex_string)
      (struct.get $zhex_some_ref $value
        (ref.cast (ref $zhex_some_ref) (call $zhex_env_get (call $t_in (i32.const 0) (local.get $len))))))
    (i32.const 0)))
(func (export "t_env_load_tag") (param $len i32) (result i32)
  (struct.get $zhex_result $tag (call $zhex_env_load (call $t_in (i32.const 0) (local.get $len)))))
(func (export "t_env_load_msg") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload (call $zhex_env_load (call $t_in (i32.const 0) (local.get $len)))))))
    (i32.const 0)))

(func (export "t_file_read_tag") (param $len i32) (result i32)
  (struct.get $zhex_result $tag (call $zhex_file_read (call $t_in (i32.const 0) (local.get $len)))))
(func (export "t_file_read_val") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.cast (ref $zhex_string)
      (struct.get $zhex_result $payload (call $zhex_file_read (call $t_in (i32.const 0) (local.get $len)))))
    (i32.const 0)))
(func (export "t_file_read_msg") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload (call $zhex_file_read (call $t_in (i32.const 0) (local.get $len)))))))
    (i32.const 0)))
(func (export "t_file_write_tag") (param $clen i32) (param $plen i32) (param $append i32) (result i32)
  (struct.get $zhex_result $tag
    (call $zhex_file_write (call $t_in (i32.const 0) (local.get $clen)) (call $t_in (i32.const 4096) (local.get $plen)) (local.get $append))))
(func (export "t_file_write_msg") (param $clen i32) (param $plen i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload
            (call $zhex_file_write (call $t_in (i32.const 0) (local.get $clen)) (call $t_in (i32.const 4096) (local.get $plen)) (i32.const 0))))))
    (i32.const 0)))
(func (export "t_file_exists") (param $len i32) (result i32)
  (call $zhex_file_exists (call $t_in (i32.const 0) (local.get $len))))
(func (export "t_file_size_val") (param $len i32) (result i64)
  (struct.get $zhex_some_i64 $value
    (ref.cast (ref $zhex_some_i64)
      (struct.get $zhex_result $payload (call $zhex_file_size (call $t_in (i32.const 0) (local.get $len)))))))
(func (export "t_file_list_count") (param $len i32) (result i64)
  (call $zhex_list_length_ref
    (ref.cast (ref $zhex_list_ref)
      (struct.get $zhex_result $payload (call $zhex_file_list (call $t_in (i32.const 0) (local.get $len)))))))
(func (export "t_file_list_item") (param $len i32) (param $i i64) (result i32)
  (call $zhex_stage_utf8_at
    (ref.cast (ref $zhex_string)
      (array.get $zhex_arr_ref
        (struct.get $zhex_list_ref $items
          (ref.cast (ref $zhex_list_ref)
            (struct.get $zhex_result $payload (call $zhex_file_list (call $t_in (i32.const 0) (local.get $len))))))
        (i32.wrap_i64 (local.get $i))))
    (i32.const 0)))
(func (export "t_file_list_msg") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload (call $zhex_file_list (call $t_in (i32.const 0) (local.get $len)))))))
    (i32.const 0)))
(func (export "t_folder_tag") (param $len i32) (param $parents i32) (result i32)
  (struct.get $zhex_result $tag
    (call $zhex_file_make_folder (call $t_in (i32.const 0) (local.get $len)) (local.get $parents))))
(func (export "t_folder_msg") (param $len i32) (param $parents i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload
            (call $zhex_file_make_folder (call $t_in (i32.const 0) (local.get $len)) (local.get $parents))))))
    (i32.const 0)))
(func (export "t_del_msg") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload (call $zhex_file_delete (call $t_in (i32.const 0) (local.get $len)))))))
    (i32.const 0)))
(func (export "t_delfolder_tag") (param $len i32) (param $rec i32) (result i32)
  (struct.get $zhex_result $tag
    (call $zhex_file_delete_folder (call $t_in (i32.const 0) (local.get $len)) (local.get $rec))))
(func (export "t_delfolder_msg") (param $len i32) (param $rec i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload
            (call $zhex_file_delete_folder (call $t_in (i32.const 0) (local.get $len)) (local.get $rec))))))
    (i32.const 0)))
(func (export "t_move_msg") (param $slen i32) (param $dlen i32) (param $ow i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload
            (call $zhex_file_move (call $t_in (i32.const 0) (local.get $slen)) (call $t_in (i32.const 4096) (local.get $dlen)) (local.get $ow))))))
    (i32.const 0)))
(func (export "t_move_tag") (param $slen i32) (param $dlen i32) (param $ow i32) (result i32)
  (struct.get $zhex_result $tag
    (call $zhex_file_move (call $t_in (i32.const 0) (local.get $slen)) (call $t_in (i32.const 4096) (local.get $dlen)) (local.get $ow))))
(func (export "t_stream_open") (param $len i32) (result i32)
  (call $zhex_stream_open (call $t_in (i32.const 0) (local.get $len))))
(func (export "t_stream_line") (param $h i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (ref.cast (ref null $zhex_string)
        (struct.get $zhex_result $payload (call $zhex_stream_line (local.get $h)))))
    (i32.const 0)))
(func (export "t_stream_line_null") (param $h i32) (result i32)
  (ref.is_null
    (struct.get $zhex_result $payload (call $zhex_stream_line (local.get $h)))))
(func (export "t_stream_close") (param $h i32) (call $zhex_stream_close (local.get $h)))
(func (export "t_print_line") (param $len i32)
  (call $zhex_print_line (call $t_in (i32.const 0) (local.get $len))))

;; ---- json (DESIGN-M3 6): tree parse/encode and typed decode ---------------
;; The typed path mirrors the json.zhex fixture's Config/Endpoint records
;; through hand-written record structs and make/get thunks, exactly as the
;; emitter generates them per call site.

(func (export "t_json_parse_tag") (param $len i32) (result i32)
  (struct.get $zhex_result $tag (call $zhex_json_parse (call $t_in (i32.const 0) (local.get $len)))))
(func (export "t_json_parse_msg") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload (call $zhex_json_parse (call $t_in (i32.const 0) (local.get $len)))))))
    (i32.const 0)))
(func (export "t_json_variants") (param $len i32) (result i32)
  (local $p (ref null eq))
  (local.set $p (struct.get $zhex_result $payload (call $zhex_json_parse (call $t_in (i32.const 0) (local.get $len)))))
  (if (ref.test (ref $zt_Json_Null) (local.get $p)) (then (return (i32.const 0))))
  (if (ref.test (ref $zt_Json_Bool) (local.get $p)) (then (return (i32.const 1))))
  (if (ref.test (ref $zt_Json_Text) (local.get $p)) (then (return (i32.const 2))))
  (if (ref.test (ref $zt_Json_Number) (local.get $p)) (then (return (i32.const 3))))
  (if (ref.test (ref $zt_Json_List) (local.get $p)) (then (return (i32.const 4))))
  (if (ref.test (ref $zt_Json_Object) (local.get $p)) (then (return (i32.const 5))))
  (i32.const -1))
(func (export "t_json_top_len") (param $len i32) (result i64)
  (local $p (ref null eq))
  (local.set $p (struct.get $zhex_result $payload (call $zhex_json_parse (call $t_in (i32.const 0) (local.get $len)))))
  (if (ref.test (ref $zt_Json_List) (local.get $p))
    (then
      (return (i64.extend_i32_u (array.len (ref.as_non_null
        (struct.get $zhex_list_ref $items (struct.get $zt_Json_List $f0 (ref.cast (ref $zt_Json_List) (local.get $p))))))))))
  (if (ref.test (ref $zt_Json_Object) (local.get $p))
    (then
      (return (i64.extend_i32_u (array.len (struct.get $zhex_map_str_ref $keys
        (struct.get $zt_Json_Object $f0 (ref.cast (ref $zt_Json_Object) (local.get $p)))))))))
  (i64.const -1))

;; recursive node counter and innermost literal: prove deep trees walk
(func $t_count (param $v (ref null eq)) (result i64)
  (local $i i32) (local $n i32) (local $total i64)
  (local $items (ref null $zhex_arr_ref))
  (if (ref.test (ref $zt_Json_List) (local.get $v))
    (then
      (local.set $items (struct.get $zhex_list_ref $items (struct.get $zt_Json_List $f0 (ref.cast (ref $zt_Json_List) (local.get $v)))))
      (local.set $n (array.len (ref.as_non_null (local.get $items))))
      (block $done
        (loop $next
          (br_if $done (i32.ge_u (local.get $i) (local.get $n)))
          (local.set $total (i64.add (local.get $total)
            (call $t_count (array.get $zhex_arr_ref (ref.as_non_null (local.get $items)) (local.get $i)))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $next)))
      (return (i64.add (local.get $total) (i64.const 1)))))
  (if (ref.test (ref $zt_Json_Object) (local.get $v))
    (then
      (local.set $n (array.len (struct.get $zhex_map_str_ref $keys
        (struct.get $zt_Json_Object $f0 (ref.cast (ref $zt_Json_Object) (local.get $v))))))
      (local.set $items (struct.get $zhex_map_str_ref $vals
        (struct.get $zt_Json_Object $f0 (ref.cast (ref $zt_Json_Object) (local.get $v)))))
      (block $done
        (loop $next
          (br_if $done (i32.ge_u (local.get $i) (local.get $n)))
          (local.set $total (i64.add (local.get $total)
            (call $t_count (array.get $zhex_arr_ref (local.get $items) (local.get $i)))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $next)))
      (return (i64.add (local.get $total) (i64.const 1)))))
  (i64.const 1))
(func (export "t_json_count") (param $len i32) (result i64)
  (call $t_count (struct.get $zhex_result $payload (call $zhex_json_parse (call $t_in (i32.const 0) (local.get $len))))))

(func $t_innermost (param $v (ref null eq)) (result i32)
  (if (ref.test (ref $zt_Json_List) (local.get $v))
    (then
      (return (call $t_innermost
        (array.get $zhex_arr_ref
          (ref.as_non_null (struct.get $zhex_list_ref $items (struct.get $zt_Json_List $f0 (ref.cast (ref $zt_Json_List) (local.get $v)))))
          (i32.const 0))))))
  (if (ref.test (ref $zt_Json_Number) (local.get $v))
    (then
      (return (call $zhex_stage_utf8_at
        (struct.get $zt_Json_Number $f0 (ref.cast (ref $zt_Json_Number) (local.get $v))) (i32.const 0)))))
  (i32.const -1))
(func (export "t_json_innermost") (param $len i32) (result i32)
  (call $t_innermost (struct.get $zhex_result $payload (call $zhex_json_parse (call $t_in (i32.const 0) (local.get $len))))))

;; parse + tree encode round trip: stages the encoded text; -1 parse error,
;; -2 encode error (message staged either way)
(func (export "t_json_rt") (param $len i32) (result i32)
  (local $r (ref $zhex_result))
  (local.set $r (call $zhex_json_parse (call $t_in (i32.const 0) (local.get $len))))
  (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
    (then
      (drop (call $zhex_stage_utf8_at
        (ref.as_non_null (struct.get $zhex_err $message (ref.cast (ref $zhex_err) (struct.get $zhex_result $payload (local.get $r)))))
        (i32.const 0)))
      (return (i32.const -1))))
  (local.set $r (call $zhex_json_encode (struct.get $zhex_result $payload (local.get $r)) (ref.null $zjson_schema)))
  (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
    (then
      (drop (call $zhex_stage_utf8_at
        (ref.as_non_null (struct.get $zhex_err $message (ref.cast (ref $zhex_err) (struct.get $zhex_result $payload (local.get $r)))))
        (i32.const 0)))
      (return (i32.const -2))))
  (call $zhex_stage_utf8_at (ref.cast (ref $zhex_string) (struct.get $zhex_result $payload (local.get $r))) (i32.const 0)))

;; a hand-built tree with an invalid Number literal: encode must fail
(func (export "t_json_badnum") (result i32)
  (local $r (ref $zhex_result))
  (local.set $r (call $zhex_json_encode
    (struct.new $zt_Json_Number (i64.const 0) (i64.const 0) (i64.const 0) (i32.const 3)
      (array.new_fixed $zhex_string 2 (i32.const 49) (i32.const 120)))
    (ref.null $zjson_schema)))
  (call $zhex_stage_utf8_at
    (ref.as_non_null (struct.get $zhex_err $message (ref.cast (ref $zhex_err) (struct.get $zhex_result $payload (local.get $r)))))
    (i32.const 0)))

;; the typed fixtures: Endpoint and Config records with make/get thunks
(type $t_ep (struct (field $host (ref $zhex_string)) (field $port i64)))
(type $t_cfg (struct (field $service (ref $zhex_string)) (field $retries i64) (field $verbose i32)
  (field $endpoints (ref null $zhex_list_ref)) (field $limits (ref null $zhex_map_str_i64)) (field $timeout (ref null eq))))

(func $t_ep_make (type $zjson_make_fn) (param $f (ref $zhex_arr_ref)) (result (ref null eq))
  (struct.new $t_ep
    (ref.cast (ref $zhex_string) (array.get $zhex_arr_ref (local.get $f) (i32.const 0)))
    (struct.get $zhex_some_i64 $value (ref.cast (ref $zhex_some_i64) (array.get $zhex_arr_ref (local.get $f) (i32.const 1))))))
(func $t_ep_get (type $zjson_get_fn) (param $v (ref null eq)) (result (ref null $zhex_arr_ref))
  (array.new_fixed $zhex_arr_ref 2
    (struct.get $t_ep $host (ref.cast (ref $t_ep) (local.get $v)))
    (struct.new $zhex_some_i64 (struct.get $t_ep $port (ref.cast (ref $t_ep) (local.get $v))))))
(func $t_cfg_make (type $zjson_make_fn) (param $f (ref $zhex_arr_ref)) (result (ref null eq))
  (struct.new $t_cfg
    (ref.cast (ref $zhex_string) (array.get $zhex_arr_ref (local.get $f) (i32.const 0)))
    (struct.get $zhex_some_i64 $value (ref.cast (ref $zhex_some_i64) (array.get $zhex_arr_ref (local.get $f) (i32.const 1))))
    (i31.get_u (ref.cast (ref i31) (array.get $zhex_arr_ref (local.get $f) (i32.const 2))))
    (ref.cast (ref $zhex_list_ref) (array.get $zhex_arr_ref (local.get $f) (i32.const 3)))
    (ref.cast (ref $zhex_map_str_i64) (array.get $zhex_arr_ref (local.get $f) (i32.const 4)))
    (array.get $zhex_arr_ref (local.get $f) (i32.const 5))))
(func $t_cfg_get (type $zjson_get_fn) (param $v (ref null eq)) (result (ref null $zhex_arr_ref))
  (array.new_fixed $zhex_arr_ref 6
    (struct.get $t_cfg $service (ref.cast (ref $t_cfg) (local.get $v)))
    (struct.new $zhex_some_i64 (struct.get $t_cfg $retries (ref.cast (ref $t_cfg) (local.get $v))))
    (ref.i31 (struct.get $t_cfg $verbose (ref.cast (ref $t_cfg) (local.get $v))))
    (struct.get $t_cfg $endpoints (ref.cast (ref $t_cfg) (local.get $v)))
    (struct.get $t_cfg $limits (ref.cast (ref $t_cfg) (local.get $v)))
    (struct.get $t_cfg $timeout (ref.cast (ref $t_cfg) (local.get $v)))))
(elem declare func $t_ep_make $t_ep_get $t_cfg_make $t_cfg_get)

(func $t_schema (result (ref $zjson_schema))
  (local $str (ref null $zjson_schema)) (local $int (ref null $zjson_schema)) (local $bool (ref null $zjson_schema))
  (local $ep (ref null $zjson_schema)) (local $eps (ref null $zjson_schema))
  (local $lim (ref null $zjson_schema)) (local $opt (ref null $zjson_schema))
  (local.set $str (struct.new $zjson_schema (i32.const 2) (ref.null $zjson_schema) (ref.null $zhex_arr_ref) (ref.null $zjson_make_fn) (ref.null $zjson_get_fn)))
  (local.set $int (struct.new $zjson_schema (i32.const 0) (ref.null $zjson_schema) (ref.null $zhex_arr_ref) (ref.null $zjson_make_fn) (ref.null $zjson_get_fn)))
  (local.set $bool (struct.new $zjson_schema (i32.const 1) (ref.null $zjson_schema) (ref.null $zhex_arr_ref) (ref.null $zjson_make_fn) (ref.null $zjson_get_fn)))
  (local.set $ep (struct.new $zjson_schema (i32.const 6) (ref.null $zjson_schema)
    (array.new_fixed $zhex_arr_ref 2
      (struct.new $zjson_field (array.new_fixed $zhex_string 4 (i32.const 104) (i32.const 111) (i32.const 115) (i32.const 116)) (ref.as_non_null (local.get $str)))
      (struct.new $zjson_field (array.new_fixed $zhex_string 4 (i32.const 112) (i32.const 111) (i32.const 114) (i32.const 116)) (ref.as_non_null (local.get $int))))
    (ref.func $t_ep_make) (ref.func $t_ep_get)))
  (local.set $eps (struct.new $zjson_schema (i32.const 4) (ref.as_non_null (local.get $ep)) (ref.null $zhex_arr_ref) (ref.null $zjson_make_fn) (ref.null $zjson_get_fn)))
  (local.set $lim (struct.new $zjson_schema (i32.const 5) (ref.as_non_null (local.get $int)) (ref.null $zhex_arr_ref) (ref.null $zjson_make_fn) (ref.null $zjson_get_fn)))
  (local.set $opt (struct.new $zjson_schema (i32.const 3) (ref.as_non_null (local.get $int)) (ref.null $zhex_arr_ref) (ref.null $zjson_make_fn) (ref.null $zjson_get_fn)))
  (struct.new $zjson_schema (i32.const 6) (ref.null $zjson_schema)
    (array.new_fixed $zhex_arr_ref 6
      (struct.new $zjson_field (array.new_fixed $zhex_string 7 (i32.const 115) (i32.const 101) (i32.const 114) (i32.const 118) (i32.const 105) (i32.const 99) (i32.const 101)) (ref.as_non_null (local.get $str)))
      (struct.new $zjson_field (array.new_fixed $zhex_string 7 (i32.const 114) (i32.const 101) (i32.const 116) (i32.const 114) (i32.const 105) (i32.const 101) (i32.const 115)) (ref.as_non_null (local.get $int)))
      (struct.new $zjson_field (array.new_fixed $zhex_string 7 (i32.const 118) (i32.const 101) (i32.const 114) (i32.const 98) (i32.const 111) (i32.const 115) (i32.const 101)) (ref.as_non_null (local.get $bool)))
      (struct.new $zjson_field (array.new_fixed $zhex_string 9 (i32.const 101) (i32.const 110) (i32.const 100) (i32.const 112) (i32.const 111) (i32.const 105) (i32.const 110) (i32.const 116) (i32.const 115)) (ref.as_non_null (local.get $eps)))
      (struct.new $zjson_field (array.new_fixed $zhex_string 6 (i32.const 108) (i32.const 105) (i32.const 109) (i32.const 105) (i32.const 116) (i32.const 115)) (ref.as_non_null (local.get $lim)))
      (struct.new $zjson_field (array.new_fixed $zhex_string 7 (i32.const 116) (i32.const 105) (i32.const 109) (i32.const 101) (i32.const 111) (i32.const 117) (i32.const 116)) (ref.as_non_null (local.get $opt))))
    (ref.func $t_cfg_make) (ref.func $t_cfg_get)))

(func $t_defaults (result (ref null eq))
  (struct.new $t_cfg
    (array.new_fixed $zhex_string 6 (i32.const 105) (i32.const 110) (i32.const 103) (i32.const 101) (i32.const 115) (i32.const 116))
    (i64.const 3)
    (i32.const 0)
    (struct.new $zhex_list_ref (array.new_default $zhex_arr_ref (i32.const 0)) (i32.const 0) (i32.const 0))
    (call $zhex_map_new_str_i64)
    (struct.new $zhex_none)))

(func (export "t_dec_tag") (param $len i32) (result i32)
  (struct.get $zhex_result $tag
    (call $zhex_json_decode (call $t_in (i32.const 0) (local.get $len)) (call $t_defaults) (call $t_schema))))
(func (export "t_dec_msg") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload
            (call $zhex_json_decode (call $t_in (i32.const 0) (local.get $len)) (call $t_defaults) (call $t_schema))))))
    (i32.const 0)))
(func (export "t_dec_service") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (struct.get $t_cfg $service
      (ref.cast (ref $t_cfg)
        (struct.get $zhex_result $payload
          (call $zhex_json_decode (call $t_in (i32.const 0) (local.get $len)) (call $t_defaults) (call $t_schema)))))
    (i32.const 0)))
(func (export "t_dec_retries") (param $len i32) (result i64)
  (struct.get $t_cfg $retries
    (ref.cast (ref $t_cfg)
      (struct.get $zhex_result $payload
        (call $zhex_json_decode (call $t_in (i32.const 0) (local.get $len)) (call $t_defaults) (call $t_schema))))))
(func (export "t_dec_ep_host") (param $len i32) (result i32)
  (call $zhex_stage_utf8_at
    (struct.get $t_ep $host
      (ref.cast (ref $t_ep)
        (array.get $zhex_arr_ref
          (ref.as_non_null (struct.get $zhex_list_ref $items
            (ref.cast (ref $zhex_list_ref)
              (struct.get $t_cfg $endpoints
                (ref.cast (ref $t_cfg)
                  (struct.get $zhex_result $payload
                    (call $zhex_json_decode (call $t_in (i32.const 0) (local.get $len)) (call $t_defaults) (call $t_schema))))))))
          (i32.const 0))))
    (i32.const 0)))
;; decode then typed encode: the full round trip, staged
(func (export "t_dec_rt") (param $len i32) (result i32)
  (local $r (ref $zhex_result))
  (local.set $r (call $zhex_json_decode (call $t_in (i32.const 0) (local.get $len)) (call $t_defaults) (call $t_schema)))
  (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
    (then
      (drop (call $zhex_stage_utf8_at
        (ref.as_non_null (struct.get $zhex_err $message (ref.cast (ref $zhex_err) (struct.get $zhex_result $payload (local.get $r)))))
        (i32.const 0)))
      (return (i32.const -1))))
  (local.set $r (call $zhex_json_encode (struct.get $zhex_result $payload (local.get $r)) (call $t_schema)))
  (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
    (then
      (drop (call $zhex_stage_utf8_at
        (ref.as_non_null (struct.get $zhex_err $message (ref.cast (ref $zhex_err) (struct.get $zhex_result $payload (local.get $r)))))
        (i32.const 0)))
      (return (i32.const -2))))
  (call $zhex_stage_utf8_at (ref.cast (ref $zhex_string) (struct.get $zhex_result $payload (local.get $r))) (i32.const 0)))
`

// watPreludeM3Runner drives the assembled module under node, one fresh
// instance per case, against a JS host implementing the §1a import surface:
// an in-memory FS (fresh per case), an injected env table, and a string
// stdin. Expected strings are pinned literals.
const watPreludeM3Runner = `
import { readFileSync } from 'node:fs';

const bytes = readFileSync(process.argv[2]);
const mod = new WebAssembly.Module(bytes);
let exports = null;
let printed = null;
let fsRoot = null;
let envTable = null;
let stdinBytes = null;
let stdinOffset = 0;

const enc = new TextEncoder();
const dec = new TextDecoder();
const mem = () => new Uint8Array(exports.memory.buffer);
function grow(end) {
  while (exports.memory.buffer.byteLength < end) exports.memory.grow(1);
}
function readBytes(ptr, len) { return mem().subarray(ptr, ptr + len); }
function readText(ptr, len) { return dec.decode(readBytes(ptr, len)); }
function scratch(data) {
  const bytes = typeof data === 'string' ? enc.encode(data) : data;
  grow(4 + bytes.length);
  mem()[0] = bytes.length & 0xff; mem()[1] = (bytes.length >>> 8) & 0xff;
  mem()[2] = (bytes.length >>> 16) & 0xff; mem()[3] = (bytes.length >>> 24) & 0xff;
  mem().set(bytes, 4);
}
function segs(path) { return path.split('/').filter((s) => s.length > 0); }
function lookup(path) {
  let node = fsRoot;
  for (const part of segs(path)) {
    if (node.kind !== 'dir') return null;
    node = node.entries.get(part) ?? null;
    if (node === null) return null;
  }
  return node;
}
function parentOf(path) {
  const parts = segs(path);
  const name = parts.pop();
  if (name === undefined) return null;
  let node = fsRoot;
  for (const part of parts) {
    if (node.kind !== 'dir') return null;
    node = node.entries.get(part) ?? null;
    if (node === null) return null;
  }
  return { dir: node, name };
}
const ERRNO = { SUCCESS: 0, ACCES: 2, EXIST: 20, INVAL: 28, ISDIR: 31, NOENT: 44, NOSYS: 52, NOTDIR: 54, NOTEMPTY: 55, XDEV: 75 };
const handles = new Map();
let handleSeq = 0;

function fresh() {
  printed = [];
  fsRoot = { kind: 'dir', entries: new Map() };
  envTable = new Map([['PRE', 'injected']]);
  stdinBytes = new Uint8Array(0);
  stdinOffset = 0;
  handles.clear();
  handleSeq = 0;
  exports = new WebAssembly.Instance(mod, {
    env: {
      print(ptr, len) { printed.push(dec.decode(readBytes(ptr, len))); },
      stdin_read(ptr, cap) {
        if (stdinOffset >= stdinBytes.length) return 0;
        const n = Math.min(cap, stdinBytes.length - stdinOffset);
        grow(ptr + n);
        mem().set(stdinBytes.subarray(stdinOffset, stdinOffset + n), ptr);
        stdinOffset += n;
        return n;
      },
      fs_read_file(pptr, plen) {
        const node = lookup(readText(pptr, plen));
        if (node === null) return ERRNO.NOENT;
        if (node.kind === 'dir') return ERRNO.ISDIR;
        scratch(node.data);
        return ERRNO.SUCCESS;
      },
      fs_write_file(pptr, plen, dptr, dlen, append) {
        const spot = parentOf(readText(pptr, plen));
        if (spot === null) return ERRNO.NOENT;
        const existing = spot.dir.entries.get(spot.name);
        if (existing !== undefined && existing.kind === 'dir') return ERRNO.ISDIR;
        const data = Uint8Array.from(readBytes(dptr, dlen));
        if (existing !== undefined && append) {
          const merged = new Uint8Array(existing.data.length + data.length);
          merged.set(existing.data); merged.set(data, existing.data.length);
          existing.data = merged;
        } else {
          spot.dir.entries.set(spot.name, { kind: 'file', data });
        }
        return ERRNO.SUCCESS;
      },
      fs_open_read(pptr, plen) {
        const node = lookup(readText(pptr, plen));
        if (node === null) return -ERRNO.NOENT;
        handles.set(handleSeq, { node, offset: 0 });
        return handleSeq++;
      },
      fs_read_line(handle) {
        const open = handles.get(handle);
        if (open === undefined) return -28;
        if (open.node.kind === 'dir') return -ERRNO.ISDIR;
        const data = open.node.data;
        if (open.offset >= data.length) return 0;
        const nl = data.indexOf(0x0a, open.offset);
        const end = nl >= 0 ? nl + 1 : data.length;
        scratch(data.subarray(open.offset, end));
        open.offset = end;
        return 1;
      },
      fs_close(handle) { handles.delete(handle); return 0; },
      fs_stat(pptr, plen) {
        const node = lookup(readText(pptr, plen));
        if (node === null) return ERRNO.NOENT;
        const payload = new Uint8Array(12);
        const dv = new DataView(payload.buffer);
        dv.setUint32(0, node.kind === 'dir' ? 1 : 0, true);
        dv.setBigInt64(4, BigInt(node.kind === 'file' ? node.data.length : 0), true);
        scratch(payload);
        return ERRNO.SUCCESS;
      },
      fs_list(pptr, plen) {
        const node = lookup(readText(pptr, plen));
        if (node === null) return ERRNO.NOENT;
        if (node.kind !== 'dir') return ERRNO.NOTDIR;
        const names = [...node.entries.keys()].sort();
        const out = new Uint8Array(4 + names.reduce((s, n) => s + 4 + enc.encode(n).length, 0));
        new DataView(out.buffer).setUint32(0, names.length, true);
        let at = 4;
        for (const name of names) {
          const nb = enc.encode(name);
          new DataView(out.buffer).setUint32(at, nb.length, true);
          out.set(nb, at + 4);
          at += 4 + nb.length;
        }
        scratch(out);
        return ERRNO.SUCCESS;
      },
      fs_make_folder(pptr, plen, parents) {
        const path = readText(pptr, plen);
        if (parents) {
          let node = fsRoot;
          for (const part of segs(path)) {
            if (node.kind !== 'dir') return ERRNO.NOTDIR;
            let child = node.entries.get(part);
            if (child === undefined) { child = { kind: 'dir', entries: new Map() }; node.entries.set(part, child); }
            else if (child.kind !== 'dir') return ERRNO.NOTDIR;
            node = child;
          }
          return ERRNO.SUCCESS;
        }
        const spot = parentOf(path);
        if (spot === null) return ERRNO.NOENT;
        if (spot.dir.entries.has(spot.name)) return ERRNO.EXIST;
        spot.dir.entries.set(spot.name, { kind: 'dir', entries: new Map() });
        return ERRNO.SUCCESS;
      },
      fs_delete(pptr, plen) {
        const path = readText(pptr, plen);
        const node = lookup(path);
        if (node === null) return ERRNO.NOENT;
        if (node.kind === 'dir' && node.entries.size > 0) return ERRNO.NOTEMPTY;
        const spot = parentOf(path);
        spot.dir.entries.delete(spot.name);
        return ERRNO.SUCCESS;
      },
      fs_delete_folder(pptr, plen, recursive) {
        const path = readText(pptr, plen);
        const node = lookup(path);
        if (node === null) return ERRNO.NOENT;
        if (!recursive && node.kind === 'dir' && node.entries.size > 0) return ERRNO.NOTEMPTY;
        const spot = parentOf(path);
        spot.dir.entries.delete(spot.name);
        return ERRNO.SUCCESS;
      },
      fs_move(sptr, slen, dptr, dlen, overwrite) {
        const source = readText(sptr, slen);
        const target = readText(dptr, dlen);
        const srcSpot = parentOf(source);
        const src = srcSpot === null ? null : (srcSpot.dir.entries.get(srcSpot.name) ?? null);
        if (src === null) return ERRNO.NOENT;
        const spot = parentOf(target);
        if (spot === null) return ERRNO.NOENT;
        const existing = spot.dir.entries.get(spot.name);
        if (existing !== undefined && existing.kind === 'dir') return ERRNO.ISDIR;
        if (existing !== undefined && !overwrite) return ERRNO.EXIST;
        spot.dir.entries.set(spot.name, src);
        srcSpot.dir.entries.delete(srcSpot.name);
        return ERRNO.SUCCESS;
      },
      env_get(kptr, klen) {
        const key = readText(kptr, klen);
        if (!envTable.has(key)) return 0;
        scratch(envTable.get(key));
        return 1;
      },
      env_environ() {
        const pairs = [...envTable.entries()];
        const out = new Uint8Array(4 + pairs.reduce((s, [k, v]) => s + 8 + enc.encode(k).length + enc.encode(v).length, 0));
        new DataView(out.buffer).setUint32(0, pairs.length, true);
        let at = 4;
        for (const [k, v] of pairs) {
          const kb = enc.encode(k), vb = enc.encode(v);
          new DataView(out.buffer).setUint32(at, kb.length, true); out.set(kb, at + 4); at += 4 + kb.length;
          new DataView(out.buffer).setUint32(at, vb.length, true); out.set(vb, at + 4); at += 4 + vb.length;
        }
        scratch(out);
        return ERRNO.SUCCESS;
      },
    },
  }).exports;
}

const cases = [];
const t = (name, fn) => cases.push({ name, fn });
const show = (v) => typeof v === 'bigint' ? v.toString() + 'n' : JSON.stringify(v);
const eq = (got, want, what) => { if (got !== want) throw new Error(what + ': got ' + show(got) + ', want ' + show(want)); };
const put = (off, s) => { const b = enc.encode(s); grow(off + b.length); mem().set(b, off); };
const outStr = (len) => dec.decode(mem().subarray(0, len));
const str = (op, s, ...rest) => { put(0, s); return outStr(exports[op](enc.encode(s).length, ...rest)); };
const str2 = (op, a, b, ...rest) => { put(0, a); put(4096, b); return outStr(exports[op](enc.encode(a).length, enc.encode(b).length, ...rest)); };
const call = (op, s, ...rest) => { put(0, s); return exports[op](enc.encode(s).length, ...rest); };
const callMsg = (op, s, ...rest) => { put(0, s); return outStr(exports[op](enc.encode(s).length, ...rest)); };
const call2 = (op, a, b, ...rest) => { put(0, a); put(4096, b); return exports[op](enc.encode(a).length, enc.encode(b).length, ...rest); };
const call2Msg = (op, a, b, ...rest) => { put(0, a); put(4096, b); return outStr(exports[op](enc.encode(a).length, enc.encode(b).length, ...rest)); };
const seedFile = (path, text) => {
  const spot = parentOf(path);
  spot.dir.entries.set(spot.name, { kind: 'file', data: enc.encode(text) });
};
const seedDir = (path) => {
  const spot = parentOf(path);
  spot.dir.entries.set(spot.name, { kind: 'dir', entries: new Map() });
};

// ---- errno rendering (DESIGN-M3 1b) ------------------------------------
for (const [e, want] of [
  [2, 'permission denied'], [20, 'file exists'], [28, 'invalid argument'],
  [31, 'is a directory'], [44, 'no such file or directory'], [52, 'function not implemented'],
  [54, 'not a directory'], [55, 'directory not empty'], [75, 'invalid cross-device link'],
  [200, 'errno 200'], [0, 'errno 0'],
]) {
  t('errno ' + e, () => { fresh(); eq(outStr(exports.t_errno(e)), want, 'text'); });
}
t('os_err open/noent', () => {
  fresh(); put(0, 'nope.txt'); put(4096, 'open');
  eq(outStr(exports.t_os_err(8, 4, 44)), 'open nope.txt: no such file or directory', 'os_err');
});
t('os_err rename verb with errno text', () => {
  fresh(); put(0, 'a'); put(4096, 'rename');
  eq(outStr(exports.t_os_err(1, 6, 55)), 'rename a: directory not empty', 'os_err');
});

// ---- path words (DESIGN-M3 3) ------------------------------------------
for (const [s, want] of [
  ['a/b/../c', 'a/c'], ['/../a', '/a'], ['abc/def/', 'abc/def'], ['//x', '/x'],
  ['a//b', 'a/b'], ['../x', '../x'], ['.', '.'], ['..', '..'], ['/', '/'], ['', '.'],
  ['a/./b', 'a/b'], ['a/b/..', 'a'],
]) {
  t('clean ' + JSON.stringify(s), () => { fresh(); eq(str('t_clean', s), want, 'clean'); });
}
for (const [a, b, want] of [
  ['a', 'b', 'a/b'], ['a', '/b', '/b'], ['', 'x', 'x'], ['x', '', 'x'],
  ['a/b', '../c', 'a/c'], ['/base', 'child', '/base/child'],
]) {
  t('join ' + JSON.stringify([a, b]), () => { fresh(); eq(str2('t_join', a, b), want, 'join'); });
}
for (const [s, want] of [
  ['/x/y.txt', 'y.txt'], ['y.txt', 'y.txt'], ['a/b/', 'b'], ['/', '/'], ['', '.'], ['abc/', 'abc'],
]) {
  t('base ' + JSON.stringify(s), () => { fresh(); eq(str('t_base', s), want, 'base'); });
}
for (const [s, want] of [
  ['a/b.txt', '.txt'], ['.bashrc', '.bashrc'], ['a/b', ''], ['a.b/c', ''], ['a.tar.gz', '.gz'],
]) {
  t('ext ' + JSON.stringify(s), () => { fresh(); eq(str('t_ext', s), want, 'ext'); });
}
for (const [s, want] of [
  ['a/b', 'a'], ['/x/y', '/x'], ['x', '.'], ['/x', '/'], ['a//b', 'a'], ['a/b/c', 'a/b'],
]) {
  t('dir ' + JSON.stringify(s), () => { fresh(); eq(str('t_dir', s), want, 'dir'); });
}

// ---- duration (DESIGN-M3 5) --------------------------------------------
for (const [v, want] of [
  [0n, '0s'], [1n, '1ns'], [1500n, '1.5\u00b5s'], [500000n, '500\u00b5s'], [1500000n, '1.5ms'],
  [500000000n, '500ms'], [1500000000n, '1.5s'], [90000000000n, '1m30s'], [3600000000000n, '1h0m0s'],
  [5400000000000n, '1h30m0s'], [-1500n, '-1.5\u00b5s'], [9900000000000n, '2h45m0s'],
  [9223372036854775807n, '2562047h47m16.854775807s'],
  [-9223372036854775808n, '-2562047h47m16.854775808s'],
]) {
  t('duration fmt ' + v, () => { fresh(); eq(outStr(exports.t_dur_fmt(v)), want, 'fmt'); });
}
for (const [s, want] of [
  ['0', 0n], ['1.5h', 5400000000000n], ['2h45m', 9900000000000n], ['500ms', 500000000n],
  ['1.5\u00b5s', 1500n], ['1.5\u03bcs', 1500n], ['-1.5h', -5400000000000n], ['.5s', 500000000n],
  ['1.s', 1000000000n], ['0.5ns', 0n], ['-0', 0n], ['+7s', 7000000000n], ['300ms', 300000000n],
  ['9223372036854775807ns', 9223372036854775807n], ['-9223372036854775808ns', -9223372036854775808n],
  ['1h1m1s1ms1\u00b5s1ns', 3661001001001n],
]) {
  t('duration parse ' + JSON.stringify(s), () => {
    fresh(); put(0, s);
    eq(exports.t_dur_tag(enc.encode(s).length), 0, 'tag');
    eq(exports.t_dur_val(enc.encode(s).length), want, 'value');
  });
}
for (const s of ['', 'abc', '1', '1x', '1.5', '-', 's', '1.5s1', '9999999999999999999s',
                 '9223372036854775808ns', '1.5hh', '1.5 s']) {
  t('duration invalid ' + JSON.stringify(s), () => {
    fresh(); put(0, s);
    eq(exports.t_dur_tag(enc.encode(s).length), 1, 'tag');
    eq(outStr(exports.t_dur_msg(enc.encode(s).length)), 'invalid duration', 'msg');
  });
}

// ---- buffers (DESIGN-M3 4) ----------------------------------------------
t('buffer add/count/to-text/set', () => {
  fresh();
  let buf = exports.t_buf_new();
  eq(exports.t_buf_count(buf), 0n, 'empty count');
  for (const line of ['alpha', 'beta', 'gamma']) {
    put(0, line);
    buf = exports.t_buf_add(buf, enc.encode(line).length);
  }
  eq(exports.t_buf_count(buf), 3n, 'count');
  put(0, 'ALPHA');
  eq(exports.t_buf_set(buf, 0n, 5), 0, 'set tag');
  eq(outStr(exports.t_buf_to_text(buf)), 'ALPHA\nbeta\ngamma', 'to-text');
  eq(exports.t_buf_line_tag(buf, 1n), 0, 'line tag');
  eq(outStr(exports.t_buf_line_val(buf, 1n)), 'beta', 'line val');
});
t('buffer aliasing: two names observe one buffer', () => {
  fresh();
  let b = exports.t_buf_new();
  put(0, 'one');
  b = exports.t_buf_add(b, 3);
  const alias = b;
  put(0, 'two');
  exports.t_buf_add(alias, 3);
  eq(exports.t_buf_count(b), 2n, 'alias count');
});
t('buffer out-of-range message', () => {
  fresh();
  const b = exports.t_buf_add(exports.t_buf_add(exports.t_buf_add(exports.t_buf_new(), 'a'), 'b'), 'c');
  eq(exports.t_buf_line_tag(b, 5n), 1, 'tag');
  eq(outStr(exports.t_buf_line_msg(b, 5n)), 'buffer index 5 is outside 0..2', 'msg');
});
t('buffer empty range message', () => {
  fresh();
  const b = exports.t_buf_new();
  eq(outStr(exports.t_buf_line_msg(b, 0n)), 'buffer index 0 is outside 0..-1', 'msg');
});

// ---- read-line (DESIGN-M3 4) --------------------------------------------
t('read-line prompt is raw and two reads work', () => {
  fresh();
  stdinBytes = enc.encode('hello\nworld');
  put(0, 'N: ');
  eq(outStr(exports.t_rl_val(3)), 'hello', 'first line');
  eq(printed.join(''), 'N: ', 'prompt has no newline');
});
t('read-line partial final line then end of input', () => {
  fresh();
  stdinBytes = enc.encode('hello\nworld');
  put(0, 'P: ');
  exports.t_rl_val(2); // consume 'hello'
  put(0, 'P: ');
  eq(outStr(exports.t_rl_val(2)), 'world', 'partial final line');
  put(0, 'P: ');
  eq(exports.t_rl_tag(2), 1, 'eof tag');
  put(0, 'P: ');
  eq(outStr(exports.t_rl_msg(2)), 'end of input', 'eof message');
});
t('read-line chomps CRLF once', () => {
  fresh();
  stdinBytes = enc.encode('x\r\ny\n');
  put(0, 'Q: ');
  eq(outStr(exports.t_rl_val(2)), 'x', 'crlf');
  put(0, 'Q: ');
  eq(outStr(exports.t_rl_val(2)), 'y', 'lf');
});

// ---- env (DESIGN-M3 7) ---------------------------------------------------
t('env seeding and get Some/None/set-but-empty', () => {
  fresh();
  envTable.set('A', '1');
  envTable.set('EMPTY', '');
  exports.t_env_init();
  eq(call('t_env_some', 'A'), 1, 'A is Some');
  eq(callMsg('t_env_val', 'A'), '1', 'A value');
  eq(call('t_env_some', 'EMPTY'), 1, 'set-but-empty is Some');
  eq(callMsg('t_env_val', 'EMPTY'), '', 'empty value');
  eq(call('t_env_some', 'MISSING'), 0, 'missing is None');
});
t('env.load: comments, blanks, quotes, first-wins against startup', () => {
  fresh();
  seedFile('ok.env', '# comment\n\nFIRST=one\nFIRST=two\nQUOTED="dq"\nSQ=\'sq\'\nPRE=fromfile\nEMPTY2=\n');
  eq(call('t_env_load_tag', 'ok.env'), 0, 'load ok');
  exports.t_env_init();
  eq(callMsg('t_env_val', 'FIRST'), 'one', 'first line wins');
  eq(callMsg('t_env_val', 'QUOTED'), 'dq', 'double quotes stripped');
  eq(callMsg('t_env_val', 'SQ'), 'sq', 'single quotes stripped');
  eq(callMsg('t_env_val', 'PRE'), 'injected', 'startup environ wins');
  eq(call('t_env_some', 'EMPTY2'), 1, 'empty value is Some');
});
t('env.load: malformed line message', () => {
  fresh();
  seedFile('bad.env', 'GOOD=1\nBAD LINE\nAFTER=x\n');
  eq(call('t_env_load_tag', 'bad.env'), 1, 'tag');
  eq(callMsg('t_env_load_msg', 'bad.env'), 'line 2 of "bad.env" is not a KEY=VALUE assignment: "BAD LINE"', 'msg');
});
t('env.load: missing file message', () => {
  fresh();
  eq(callMsg('t_env_load_msg', 'missing.env'), 'cannot read "missing.env": open missing.env: no such file or directory', 'msg');
});

// ---- files (DESIGN-M3 3) -------------------------------------------------
t('file write/read/append round-trip', () => {
  fresh();
  eq(call('t_folder_tag', 'out', 0), 0, 'make out');
  put(0, 'hello world'); put(4096, 'out/a.txt');
  eq(exports.t_file_write_tag(11, 9, 0), 0, 'write');
  eq(callMsg('t_file_read_val', 'out/a.txt'), 'hello world', 'read value');
  put(0, '!'); put(4096, 'out/a.txt');
  eq(exports.t_file_write_tag(1, 9, 1), 0, 'append');
  eq(callMsg('t_file_read_val', 'out/a.txt'), 'hello world!', 'appended');
});
t('file write into missing folder message', () => {
  fresh();
  put(0, 'data'); put(4096, 'nope/x.txt');
  eq(exports.t_file_write_tag(4, 10, 0), 1, 'tag');
  put(0, 'data'); put(4096, 'nope/x.txt');
  eq(outStr(exports.t_file_write_msg(4, 10)),
     'cannot open "nope/x.txt" for writing: open nope/x.txt: no such file or directory', 'msg');
});
t('file read missing message', () => {
  fresh();
  eq(callMsg('t_file_read_msg', 'nope.txt'), 'cannot open "nope.txt": open nope.txt: no such file or directory', 'msg');
});
t('file exists/size', () => {
  fresh();
  seedFile('a.txt', '0123456789');
  seedDir('d');
  eq(call('t_file_exists', 'a.txt'), 1, 'file exists');
  eq(call('t_file_size_val', 'a.txt'), 10n, 'file size');
  eq(call('t_file_exists', 'd'), 1, 'dir exists');
  eq(call('t_file_size_val', 'd'), 0n, 'dir size is zero');
  eq(call('t_file_exists', 'gone'), 0, 'missing is false');
});
t('file list sorted with dotfiles', () => {
  fresh();
  seedFile('.hidden', 'x');
  seedFile('b.txt', 'y');
  seedFile('a.txt', 'z');
  seedDir('out');
  eq(call('t_file_list_count', ''), 4n, 'count');
  put(0, '');
  eq(outStr(exports.t_file_list_item(0, 0n)), '.hidden', 'dotfile first');
  put(0, '');
  eq(outStr(exports.t_file_list_item(0, 1n)), 'a.txt', 'sorted a');
  put(0, '');
  eq(outStr(exports.t_file_list_item(0, 3n)), 'out', 'dir listed');
});
t('file list a file message', () => {
  fresh();
  seedFile('plain', 'x');
  eq(callMsg('t_file_list_msg', 'plain'), 'cannot list "plain": open plain: not a directory', 'msg');
});
t('file make-folder semantics', () => {
  fresh();
  eq(call('t_folder_tag', 'd', 0), 0, 'create');
  eq(call('t_folder_tag', 'd', 0), 1, 'EEXIST tag');
  eq(callMsg('t_folder_msg', 'd', 0), 'cannot create folder "d": mkdir d: file exists', 'EEXIST msg');
  eq(call('t_folder_tag', 'x/y/z', 1), 0, 'parents create');
  eq(callMsg('t_folder_msg', 'p/q', 0), 'cannot create folder "p/q": mkdir p/q: no such file or directory', 'missing parent');
});
t('file delete missing message', () => {
  fresh();
  eq(callMsg('t_del_msg', 'f.txt'), 'cannot delete "f.txt": remove f.txt: no such file or directory', 'msg');
});
t('file delete non-empty message', () => {
  fresh();
  seedDir('full');
  seedFile('full/inner', 'x');
  eq(callMsg('t_del_msg', 'full'), 'cannot delete "full": remove full: directory not empty', 'msg');
  eq(callMsg('t_delfolder_msg', 'full', 0), 'cannot remove folder "full": remove full: directory not empty', 'folder msg');
});
t('file delete-folder recursive succeeds', () => {
  fresh();
  seedDir('tree');
  seedFile('tree/a', '1');
  seedFile('tree/b', '2');
  eq(call('t_delfolder_tag', 'tree', 1), 0, 'recursive ok');
  eq(call('t_file_exists', 'tree'), 0, 'gone afterwards');
});
t('file move refusal messages', () => {
  fresh();
  seedFile('src.txt', 's');
  seedFile('dst.txt', 'd');
  seedDir('adir');
  eq(call2('t_move_tag', 'src.txt', 'dst.txt', 0), 1, 'target exists tag');
  eq(call2Msg('t_move_msg', 'src.txt', 'dst.txt', 0), 'cannot move "src.txt" to "dst.txt": the target exists', 'msg');
  eq(call2Msg('t_move_msg', 'src.txt', 'adir', 1), 'cannot move "src.txt" to "adir": the target is a folder', 'folder target');
  eq(call2Msg('t_move_msg', 'gone', 'x', 0), 'cannot move "gone" to "x": rename gone x: no such file or directory', 'missing source');
});
t('file move and move overwrite succeed', () => {
  fresh();
  seedFile('a', '1');
  seedFile('b', '2');
  eq(call2('t_move_tag', 'a', 'b', 1), 0, 'overwrite ok');
  eq(call('t_file_exists', 'b'), 1, 'target present');
});
t('stream lines, partial final, EOF, close', () => {
  fresh();
  seedFile('s.txt', 'l1\nl2\nno-nl');
  const h = call('t_stream_open', 's.txt');
  eq(outStr(exports.t_stream_line(h)), 'l1', 'line 1');
  eq(outStr(exports.t_stream_line(h)), 'l2', 'line 2');
  eq(outStr(exports.t_stream_line(h)), 'no-nl', 'partial final');
  eq(exports.t_stream_line_null(h), 1, 'clean EOF is ok-null');
  exports.t_stream_close(h);
});
t('print_line owns the newline', () => {
  fresh();
  put(0, 'hello');
  exports.t_print_line(5);
  eq(printed.join(''), 'hello\n', 'raw plus one newline');
});

// ---- json (DESIGN-M3 6): every message pinned from the interpreter --------
const parseErrs = [
  ['', 'invalid JSON: EOF at ""'],
  ['x', 'invalid JSON: invalid character \'x\' looking for beginning of value at ""'],
  ['tru', 'invalid JSON: unexpected EOF at ""'],
  ['trux', "invalid JSON: invalid character 'x' in literal true (expecting 'e') at \"\""],
  ['truex', 'invalid JSON: invalid character \'x\' looking for beginning of value at ""'],
  ['nul', 'invalid JSON: unexpected EOF at ""'],
  ['falsx', "invalid JSON: invalid character 'x' in literal false (expecting 'e') at \"\""],
  ['nulx', "invalid JSON: invalid character 'x' in literal null (expecting 'l') at \"\""],
  ['nul l', "invalid JSON: invalid character ' ' in literal null (expecting 'l') at \"\""],
  ['1 2', 'multiple JSON values at ""'],
  ['1 x', 'invalid JSON: invalid character \'x\' looking for beginning of value at ""'],
  ['01', 'multiple JSON values at ""'],
  ['1.5x', 'invalid JSON: invalid character \'x\' looking for beginning of value at ""'],
  ['2e', 'invalid JSON: unexpected EOF at ""'],
  ['1.x', 'invalid JSON: invalid character \'x\' in numeric literal at ""'],
  ['-x', 'invalid JSON: invalid character \'x\' in numeric literal at ""'],
  ['.5', 'invalid JSON: invalid character \'.\' looking for beginning of value at ""'],
  ['[', 'invalid JSON: unexpected end of JSON input at "/0"'],
  ['[1', 'invalid JSON: unexpected end of JSON input at "/1"'],
  ['[1,]', 'invalid JSON: invalid character \',\' looking for beginning of value at "/1"'],
  ['[}]', 'invalid JSON: invalid character \'}\' looking for beginning of value at ""'],
  ['[1 2]', 'invalid JSON: invalid character \'2\' after array element at "/1"'],
  [']', 'invalid JSON: invalid character \']\' looking for beginning of value at ""'],
  ['{', 'invalid JSON: unexpected end of JSON input at ""'],
  ['{{', 'invalid JSON: object member name must be a string at ""'],
  ['{"a"}', 'invalid JSON: invalid character \'}\' after object key at "/a"'],
  ['{"a" 1}', 'invalid JSON: invalid character \'1\' after object key at "/a"'],
  ['{"a":1,}', 'invalid JSON: invalid character \',\' looking for beginning of value at ""'],
  ['{1:2}', 'invalid JSON: object member name must be a string at ""'],
  ['{"a":1 "b":2}', 'invalid JSON: invalid character \'"\' after object key:value pair at ""'],
  ['{"a":1,"a":2}', 'duplicate object key at "/a"'],
  ['{"a":{"b":1,"b":2}}', 'duplicate object key at "/a/b"'],
  ['"abc', 'invalid JSON: unexpected EOF at ""'],
  ['"a\\qb"', 'invalid JSON: invalid escape sequence \u0060\\q\u0060 in string at ""'],
  ['"a\\u12"', 'invalid JSON: invalid escape sequence \u0060\\u12"\u0060 in string at ""'],
  ['"a\\u12x"', 'invalid JSON: invalid escape sequence \u0060\\u12x"\u0060 in string at ""'],
  ['"a\\uzzzz"', 'invalid JSON: invalid escape sequence \u0060\\uzzzz\u0060 in string at ""'],
  ['"a\\x"', 'invalid JSON: invalid escape sequence \u0060\\x\u0060 in string at ""'],
  ['"a\\', 'invalid JSON: unexpected EOF at ""'],
  ['"a\nb"', 'invalid JSON: invalid character \'\\n\' in string at ""'],
  ['"a\tb"', 'invalid JSON: invalid character \'\\t\' in string at ""'],
  ['"a\\ud800"', 'unpaired UTF-16 surrogate at ""'],
  ['"a\\ud800A"', 'unpaired UTF-16 surrogate at ""'],
  ['["a\\ud800"]', 'unpaired UTF-16 surrogate at ""'],
  ['{"a"', 'invalid JSON: EOF at "/a"'],
  ['{"a":', 'invalid JSON: EOF at "/a"'],
  ['{"a":1', 'invalid JSON: unexpected end of JSON input at ""'],
  ['{,}', 'invalid JSON: invalid character \',\' looking for beginning of value at ""'],
  ['{"a":1,', 'invalid JSON: unexpected end of JSON input at ""'],
  ['{"a":1,x}', 'invalid JSON: invalid character \'x\' looking for beginning of value at ""'],
  ['{"a":1,,"b":2}', 'invalid JSON: invalid character \',\' looking for beginning of value at ""'],
  ['[tru]', "invalid JSON: invalid character ']' in literal true (expecting 'e') at \"/0\""],
  ['1,2', 'invalid JSON: invalid character \',\' looking for beginning of value at ""'],
  ['{}x', 'invalid JSON: invalid character \'x\' looking for beginning of value at ""'],
];
for (const [raw, want] of parseErrs) {
  t('json parse err ' + JSON.stringify(raw), () => {
    fresh(); put(0, raw);
    eq(exports.t_json_parse_tag(enc.encode(raw).length), 1, 'tag');
    eq(outStr(exports.t_json_parse_msg(enc.encode(raw).length)), want, 'msg');
  });
}
t('json parse variants and shapes', () => {
  const vars = [
    ['null', 0], ['true', 1], ['"hi"', 2], ['1.50', 3],
    ['[1,"two",false]', 4], ['{"deep":[3]}', 5],
  ];
  for (const [raw, want] of vars) {
    fresh(); put(0, raw);
    eq(exports.t_json_parse_tag(enc.encode(raw).length), 0, raw + ' parses');
    eq(exports.t_json_variants(enc.encode(raw).length), want, raw + ' variant');
  }
  fresh(); put(0, '[1,"two",false]');
  eq(exports.t_json_top_len(enc.encode('[1,"two",false]').length), 3n, 'list length');
  fresh(); put(0, '{"a":1,"b":2}');
  eq(exports.t_json_top_len(enc.encode('{"a":1,"b":2}').length), 2n, 'object length');
});
t('json tree encode round trip preserves key order and literals', () => {
  const raw = '{ "ok": true, "n": null, "t": "hi", "num": 1.50, "e": 2e3, "neg": -0.5, "arr": [1, "two", false], "obj": { "deep": [3] } }';
  fresh(); put(0, raw);
  eq(outStr(exports.t_json_rt(enc.encode(raw).length)),
     '{"ok":true,"n":null,"t":"hi","num":1.50,"e":2e3,"neg":-0.5,"arr":[1,"two",false],"obj":{"deep":[3]}}', 'round trip');
});
t('json encode escapes like json.Marshal', () => {
  const roundTrip = (raw, want, what) => {
    fresh(); put(0, raw);
    eq(outStr(exports.t_json_rt(enc.encode(raw).length)), want, what);
  };
  roundTrip('"<>&\u2028"', '"\\u003c\\u003e\\u0026\\u2028"', 'html escapes');
  roundTrip('"a\\nb\\t\\u0001"', '"a\\nb\\t\\u0001"', 'control escapes');
  roundTrip('"\\u00e9\\ud83d\\ude00"', '"\u00e9\ud83d\ude00"', 'code points re-emitted');
});
t('json number literal validation on tree encode', () => {
  fresh();
  eq(outStr(exports.t_json_badnum()), 'number literal "1x" is not valid JSON at ""', 'msg');
});
t('json deep tree at the word-depth edge', () => {
  const brackets = (d) => d === 0 ? '1' : '[' + brackets(d - 1) + ']';
  fresh(); const raw = brackets(255);
  put(0, raw);
  eq(exports.t_json_count(enc.encode(raw).length), 256n, '255 nested arrays count');
  eq(outStr(exports.t_json_innermost(enc.encode(raw).length)), '1', 'innermost literal');
  fresh(); const deep = brackets(300);
  put(0, deep);
  eq(exports.t_json_parse_tag(enc.encode(deep).length), 1, 'tag');
  eq(outStr(exports.t_json_parse_msg(enc.encode(deep).length)), 'JSON nesting limit exceeded at "' + '/0'.repeat(257) + '"', 'depth limit');
});
t('json decode happy paths and default fill', () => {
  // staging exports write to offset 0, so re-put the input before every call
  const dec1 = (op, raw, ...rest) => { put(0, raw); return exports[op](enc.encode(raw).length, ...rest); };
  const good = '{ "service": "batch", "endpoints": [ { "host": "db", "port": 5432 } ], "limits": { "cpu": 4, "mem": 2 } }';
  fresh();
  eq(dec1('t_dec_tag', good), 0, 'tag');
  eq(outStr(dec1('t_dec_service', good)), 'batch', 'service');
  eq(dec1('t_dec_retries', good), 3n, 'missing retries filled from defaults');
  eq(outStr(dec1('t_dec_ep_host', good)), 'db', 'nested record field');
  const missing = '{ "service": "batch", "endpoints": [] }';
  fresh();
  eq(dec1('t_dec_tag', missing), 0, 'tag');
  eq(dec1('t_dec_retries', missing), 3n, 'default fill');
  const out = '{"service":"batch","retries":3,"verbose":false,"endpoints":[],"limits":{},"timeout":null}';
  eq(outStr(dec1('t_dec_rt', missing)), out, 'typed encode round trip');
});
t('json decode failures with paths', () => {
  const errs = [
    ['{"retries": 1.5, "service": "s"}', 'expected an integer-form signed 64-bit number at "/retries"'],
    ['{"service": "s", "extra": 1}', 'unknown field at "/extra"'],
    ['{"service": "s", "retries": 9223372036854775808}', 'expected an integer-form signed 64-bit number at "/retries"'],
    ['{"service": 1}', 'expected String at "/service"'],
    ['[1]', 'expected object at ""'],
    ['{"service": "s", "verbose": "x"}', 'expected Bool at "/verbose"'],
    ['{"service": "s", "endpoints": {}}', 'expected array at "/endpoints"'],
    ['{"service": "s", "limits": []}', 'expected object at "/limits"'],
    ['{"service": "s", "endpoints": [{}]}', 'missing required field at "/endpoints/0/host"'],
    ['{"service": "s", "endpoints": [1]}', 'expected object at "/endpoints/0"'],
  ];
  for (const [raw, want] of errs) {
    fresh(); put(0, raw);
    eq(exports.t_dec_tag(enc.encode(raw).length), 1, raw + ' tag');
    eq(outStr(exports.t_dec_msg(enc.encode(raw).length)), want, raw + ' msg');
  }
});

let total = 0;
const failed = [];
for (const c of cases) {
  total++;
  try { c.fn(); } catch (e) { failed.push({ name: c.name, error: String(e) }); }
}
console.log(JSON.stringify({ total, failed }));
`
