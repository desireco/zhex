package lang

import "strings"

// The browser host skin (DESIGN-M3 §1a/§1c): the fifteen env.* imports under
// their $zhex_host_* names plus the M1-seam redefinitions. run.mjs provides
// env.print as a raw byte sink (the module appends its own newline,
// Surprises 2); everything else — the virtual FS, stdin, env table — is the
// runner's in-memory implementation of §2.

// watBrowserImports: the §1a import surface, module "env", 15 total. errno
// results use WASI preview 1 numbering (0 = success); host->module payloads
// land at memory offset 0 as [u32 byte length][bytes].
const watBrowserImports = `
;; browser host imports (DESIGN-M3 1a) - the $zhex_host_* seam, module "env"
(import "env" "print" (func $zhex_host_print (param i32 i32)))
(import "env" "stdin_read" (func $zhex_host_stdin_read (param i32 i32) (result i32)))
(import "env" "fs_read_file" (func $zhex_host_fs_read_file (param i32 i32) (result i32)))
(import "env" "fs_write_file" (func $zhex_host_fs_write_file (param i32 i32 i32 i32 i32) (result i32)))
(import "env" "fs_open_read" (func $zhex_host_fs_open_read (param i32 i32) (result i32)))
(import "env" "fs_read_line" (func $zhex_host_fs_read_line (param i32) (result i32)))
(import "env" "fs_close" (func $zhex_host_fs_close (param i32) (result i32)))
(import "env" "fs_stat" (func $zhex_host_fs_stat (param i32 i32) (result i32)))
(import "env" "fs_list" (func $zhex_host_fs_list (param i32 i32) (result i32)))
(import "env" "fs_make_folder" (func $zhex_host_fs_make_folder (param i32 i32 i32) (result i32)))
(import "env" "fs_delete" (func $zhex_host_fs_delete (param i32 i32) (result i32)))
(import "env" "fs_delete_folder" (func $zhex_host_fs_delete_folder (param i32 i32 i32) (result i32)))
(import "env" "fs_move" (func $zhex_host_fs_move (param i32 i32 i32 i32 i32) (result i32)))
(import "env" "env_get" (func $zhex_host_env_get (param i32 i32) (result i32)))
(import "env" "env_environ" (func $zhex_host_env_environ (result i32)))
`

// watBrowserFuncs: the M1 seam ($env.print forwards to the raw import so the
// frozen M1 $zhex_print_str keeps working) and the browser fail pair —
// byte-identical semantics to the M1/M2 originals the seam strip removed,
// so run.mjs's trap handling (read zhex_last_error / zhex_msg_len) is
// unchanged.
const watBrowserFuncs = `
;; browser host funcs: the M1/M2 seam, unchanged semantics
(func $env.print (param $p i32) (param $l i32)
  (call $zhex_host_print (local.get $p) (local.get $l)))

(func $zhex_fail (param $code i32)
  (global.set $zhex_err (local.get $code))
  (unreachable))

(func $zhex_fail_msg (param $code i32) (param $msg (ref $zhex_string))
  (global.set $zhex_msg_len (call $zhex_stage_utf8 (local.get $msg)))
  (global.set $zhex_err (local.get $code))
  (unreachable))
`

// watStripSeamDefs removes the three browser-seam definitions the host
// layers replace: the M1 env.print import, the M1 $zhex_fail, and the M2
// $zhex_fail_msg (DESIGN-M3 §1/§10 — the strip is skin-independent, so the
// emitted prelude stays byte-identical between skins; only the host layer
// differs). Function bodies are cut by a balanced-paren scan from their
// pinned signature so concurrent edits to the M2 prelude body cannot break
// the anchor; if an anchor is missing the text is left untouched and the
// module fails to assemble, a loud compiler bug rather than silent drift.
func watStripSeamDefs(wat string) string {
	out := strings.Replace(wat, watM1EnvPrintImport+"\n", "", 1)
	out = watCutSeamFunc(out, "(func $zhex_fail ")
	out = watCutSeamFunc(out, "(func $zhex_fail_msg ")
	return out
}

// watCutSeamFunc removes one top-level (func <anchor> ...) definition.
func watCutSeamFunc(wat, anchor string) string {
	i := strings.Index(wat, anchor)
	if i < 0 {
		return wat
	}
	depth := 0
	for j := i; j < len(wat); j++ {
		switch wat[j] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return wat[:i] + wat[j+1:]
			}
		}
	}
	return wat
}
