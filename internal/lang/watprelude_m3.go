package lang

import (
	"sort"
	"strings"
)

// The wasm milestone-3 runtime prelude (internal/wasm/DESIGN-M3.md §3-§7):
// skin-independent text emitted between watPreludeM2 and the host layer —
// the errno table and OS-error rendering (§1b), byte/scratch helpers, the
// newline-owning print path (Surprises 2), the pure path words, the buffer
// suite, read-line with its pending stdin buffer, the file words over the
// fifteen $zhex_host_* imports, the file.stream helpers, the Duration
// parse/format pair reimplemented from Go 1.27's time package, the env map,
// and the Json runtime (types here; parser/encoders in the json sections).
//
// Module-to-host staging offsets: the primary string slot is offset 0; the
// second simultaneous string (fs_write_file contents, fs_move target,
// stream/stat targets) stages at 65536 — above the WASI adapters' working
// area (8192..8479) and below their bulk/handle regions, which no adapter
// touches during those calls.

var watPreludeM3 = strings.Join([]string{
	watM3Types,
	watM3Literals(),
	watM3Errno(),
	watM3Bytes,
	watM3Path,
	watM3Buffer,
	watM3Readline,
	watM3ErrorInspect,
	watM3File,
	watM3Duration,
	watM3Env,
	watM3JsonTypes,
	watM3JsonParse,
	watM3JsonEncode,
	watM3JsonDecode,
}, "")

// watM3Literals renders the message fragments the M3 helpers share as tiny
// zero-arg string funcs, so WAT text never hand-counts code points.
func watM3Literals() string {
	literals := map[string]string{
		"$m3_cannot_open":   "cannot open ",
		"$m3_cannot_read":   "cannot read ",
		"$m3_cannot_list":   "cannot list ",
		"$m3_cannot_check":  "cannot check ",
		"$m3_cannot_size":   "cannot get the size of ",
		"$m3_cannot_folder": "cannot create folder ",
		"$m3_cannot_del":    "cannot delete ",
		"$m3_cannot_rmdir":  "cannot remove folder ",
		"$m3_cannot_move":   "cannot move ",
		"$m3_to":            " to ",
		"$m3_reading":       "reading ",
		"$m3_failed":        " failed: ",
		"$m3_for_writing":   " for writing",
		"$m3_v_open":        "open",
		"$m3_v_read":        "read",
		"$m3_v_stat":        "stat",
		"$m3_v_mkdir":       "mkdir",
		"$m3_v_remove":      "remove",
		"$m3_v_rename":      "rename",
		"$m3_v_readdirent":  "readdirent",
		"$m3_end_of_file":   "end of file",
		"$m3_is_closed_b":   " is closed",
		"$m3_is_utf8_b":     " is not valid UTF-8",
		"$m3_end_of_input":  "end of input",
		"$m3_input_failed":  "console input failed: read error",
		"$m3_input_utf8":    "console input is not valid UTF-8",
		"$m3_d_reason":      "reason",
		"$m3_d_end_of_file": "end-of-file",
		"$m3_d_read_failed": "read-failed",
		"$m3_bad_duration":  "invalid duration",
		"$m3_list_limit":    "file.list exceeds element limit",
		"$m3_target_folder": ": the target is a folder",
		"$m3_target_exists": ": the target exists",
		"$m3_line_of":       " of ",
		"$m3_not_kv":        " is not a KEY=VALUE assignment: ",
		"$m3_line_word":     "line ",
		"$m3_buffer_index":  "buffer index ",
		"$m3_colon_sp":      ": ",
		"$m3_file_word":     "file ",
		// json (DESIGN-M3 6): Go 1.27 encoding/json phrasing pinned from the
		// interpreter (internal/lang/json_codec.go), literals, never derived
		// from Go at emit or test time.
		"$mj_at":          " at ",
		"$mj_inv_char":    "invalid JSON: invalid character ",
		"$mj_eof":         "invalid JSON: EOF",
		"$mj_unexp_eof":   "invalid JSON: unexpected EOF",
		"$mj_unexp_end":   "invalid JSON: unexpected end of JSON input",
		"$mj_looking_b":   " looking for beginning of value",
		"$mj_after_arr_b": " after array element",
		"$mj_after_key_b": " after object key",
		"$mj_after_pair":  " after object key:value pair",
		"$mj_in_str_b":    " in string",
		"$mj_esc_pre":     "invalid JSON: invalid escape sequence `",
		"$mj_esc_suf":     "` in string",
		"$mj_member_name": "invalid JSON: object member name must be a string",
		"$mj_dup_key":     "duplicate object key",
		"$mj_surrogate":   "unpaired UTF-16 surrogate",
		"$mj_multiple":    "multiple JSON values",
		"$mj_numeric_b":   " in numeric literal",
		"$mj_in_lit":      " in literal ",
		"$mj_expecting":   " (expecting ",
		"$mj_nesting":     "JSON nesting limit exceeded",
		"$mj_list_limit":  "list exceeds element limit",
		"$mj_exp_intform": "expected an integer-form signed 64-bit number",
		"$mj_exp_int":     "expected Int",
		"$mj_exp_bool":    "expected Bool",
		"$mj_exp_str":     "expected String",
		"$mj_exp_arr":     "expected array",
		"$mj_exp_obj":     "expected object",
		"$mj_unknown":     "unknown field",
		"$mj_missing":     "missing required field",
		"$mj_num_lit_pre": "number literal ",
		"$mj_not_valid":   " is not valid JSON",
		"$mj_true":        "true",
		"$mj_false":       "false",
		"$mj_null":        "null",
	}
	keys := make([]string, 0, len(literals))
	for k := range literals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("\n;; shared message fragments\n")
	for _, k := range keys {
		b.WriteString(watStrFn(k, literals[k]))
	}
	return b.String()
}

// watM3Types: the M3 value types and module-private globals.
const watM3Types = `
;; zhex wasm runtime prelude, milestone 3 (DESIGN-M3 3-7)

(type $zhex_file (struct (field $path (ref $zhex_string)) (field $handle i32) (field $closed (mut i32))))
;; buffer lines live in a nullable-eqref array: $zhex_arr_str elements are
;; non-null and cannot be default-allocated (prelude-private repr)
(type $zhex_buffer (struct (field $lines (mut (ref null $zhex_arr_ref))) (field $len (mut i32))))

;; the Json tree (DESIGN-M3 6): six M1-scheme padded variants, ordinals
;; Null 0 .. Object 5 (jsonTreeValue); List holds a List<Json>, Object a
;; Map<String, Json> - the type graph is acyclic, no rec group needed
(type $zt_Json_Null   (struct (field $v i32)))
(type $zt_Json_Bool   (struct (field $d0 i64) (field $v i32) (field $f0 i32)))
(type $zt_Json_Text   (struct (field $d0 i64) (field $d1 i64) (field $v i32) (field $f0 (ref $zhex_string))))
(type $zt_Json_Number (struct (field $d0 i64) (field $d1 i64) (field $d2 i64) (field $v i32) (field $f0 (ref $zhex_string))))
(type $zt_Json_List   (struct (field $d0 i64) (field $d1 i64) (field $d2 i64) (field $d3 i64) (field $v i32) (field $f0 (ref $zhex_list_ref))))
(type $zt_Json_Object (struct (field $d0 i64) (field $d1 i64) (field $d2 i64) (field $d3 i64) (field $d4 i64) (field $v i32) (field $f0 (ref $zhex_map_str_ref))))

;; thunk signatures first: the rec group's schema fields reference them,
;; and the binary encoding needs their indices to exist (wasm-tools)
(type $zjson_make_fn (func (param (ref $zhex_arr_ref)) (result (ref null eq))))
(type $zjson_get_fn (func (param (ref null eq)) (result (ref null $zhex_arr_ref))))
;; internal parse nodes and decode schemas share one rec group (DESIGN-M3 6)
(rec
  (type $zjson_node
    (struct (field $kind i32)              ;; 0 null .. 5 object
            (field $bool i32)
            (field $text (ref null $zhex_string))
            (field $items (ref null $zhex_arr_ref))
            (field $members (ref null $zhex_arr_ref))))
  (type $zjson_member (struct (field $key (ref $zhex_string)) (field $val (ref null $zjson_node))))
  ;; schema kinds 0 int .. 6 record; the make/get func fields are the
  ;; flagged deviation in watnames_m3.go (generic WAT cannot build or read
  ;; user record structs without site thunks)
  (type $zjson_schema
    (struct (field $kind i32)
            (field $element (ref null $zjson_schema))
            (field $fields (ref null $zhex_arr_ref))
            (field $make (ref null $zjson_make_fn))
            (field $get (ref null $zjson_get_fn))))
  (type $zjson_field (struct (field $name (ref $zhex_string)) (field $schema (ref null $zjson_schema))))
  (type $zjson_memo (struct (field $srcs (mut (ref null $zhex_arr_ref))) (field $dsts (mut (ref null $zhex_arr_ref))))))
;; Pending stdin lives in a GC array so unrelated linear-memory staging
;; cannot overwrite unread bytes. Positions advance per line; compaction
;; happens only on refill and capacity doubles for long lines.
(global $zhex_stdin_buf (mut (ref null $zhex_arr_i32)) (ref.null $zhex_arr_i32))
(global $zhex_stdin_buflen (mut i32) (i32.const 0))
(global $zhex_stdin_bufpos (mut i32) (i32.const 0))
(global $zhex_stdin_cap (mut i32) (i32.const 4096))
(global $zhex_env (mut (ref null $zhex_map_str_ref)) (ref.null $zhex_map_str_ref))
(global $zhex_env_done (mut i32) (i32.const 0))
(global $zhex_stream_paths (mut (ref null $zhex_map_i64_ref)) (ref.null $zhex_map_i64_ref))
;; duration formatting scratch (fmtFrac/fmtInt communicate through these)
(global $zhex_dw (mut i32) (i32.const 0))
(global $zhex_dv (mut i64) (i64.const 0))

(func $zhex_errv (param $family i32) (param $msg (ref $zhex_string)) (result (ref null eq))
  (struct.new $zhex_err (local.get $family) (local.get $msg) (ref.null $zhex_arr_str)))

;; #135: an err value whose details carry one reason pair (the key literal
;; rides the value; further keys grow the entry array at the call sites)
(func $zhex_errv_reason (param $family i32) (param $msg (ref $zhex_string)) (param $reason (ref $zhex_string)) (result (ref null eq))
  (struct.new $zhex_err (local.get $family) (local.get $msg)
    (array.new_fixed $zhex_arr_str 2 (call $m3_d_reason) (local.get $reason))))
`

// watM3Errno renders the WASI-errno text table (§1b): the full 1-76 mapping
// committed as Go literals generated offline from Go's syscall.Errno strings
// on Linux (the CI OS), pinned by tests on the fixture-reachable entries;
// unknown errno renders "errno N".
var watErrnoTexts = []string{
	"argument list too long",                            // 1 E2BIG
	"permission denied",                                 // 2 EACCES
	"address already in use",                            // 3 EADDRINUSE
	"cannot assign requested address",                   // 4 EADDRNOTAVAIL
	"address family not supported by protocol",          // 5 EAFNOSUPPORT
	"resource temporarily unavailable",                  // 6 EAGAIN
	"operation already in progress",                     // 7 EALREADY
	"bad file descriptor",                               // 8 EBADF
	"bad message",                                       // 9 EBADMSG
	"device or resource busy",                           // 10 EBUSY
	"operation canceled",                                // 11 ECANCELED
	"no child processes",                                // 12 ECHILD
	"software caused connection abort",                  // 13 ECONNABORTED
	"connection refused",                                // 14 ECONNREFUSED
	"connection reset by peer",                          // 15 ECONNRESET
	"resource deadlock avoided",                         // 16 EDEADLK
	"destination address required",                      // 17 EDESTADDRREQ
	"numerical argument out of domain",                  // 18 EDOM
	"disk quota exceeded",                               // 19 EDQUOT
	"file exists",                                       // 20 EEXIST
	"bad address",                                       // 21 EFAULT
	"file too large",                                    // 22 EFBIG
	"no route to host",                                  // 23 EHOSTUNREACH
	"identifier removed",                                // 24 EIDRM
	"invalid or incomplete multibyte or wide character", // 25 EILSEQ
	"operation now in progress",                         // 26 EINPROGRESS
	"interrupted system call",                           // 27 EINTR
	"invalid argument",                                  // 28 EINVAL
	"input/output error",                                // 29 EIO
	"transport endpoint is already connected",           // 30 EISCONN
	"is a directory",                                    // 31 EISDIR
	"too many levels of symbolic links",                 // 32 ELOOP
	"too many open files",                               // 33 EMFILE
	"too many links",                                    // 34 EMLINK
	"message too long",                                  // 35 EMSGSIZE
	"multihop attempted",                                // 36 EMULTIHOP
	"file name too long",                                // 37 ENAMETOOLONG
	"network is down",                                   // 38 ENETDOWN
	"network dropped connection on reset",               // 39 ENETRESET
	"network is unreachable",                            // 40 ENETUNREACH
	"too many open files in system",                     // 41 ENFILE
	"no buffer space available",                         // 42 ENOBUFS
	"no such device",                                    // 43 ENODEV
	"no such file or directory",                         // 44 ENOENT
	"exec format error",                                 // 45 ENOEXEC
	"no locks available",                                // 46 ENOLCK
	"link has been severed",                             // 47 ENOLINK
	"cannot allocate memory",                            // 48 ENOMEM
	"no message of desired type",                        // 49 ENOMSG
	"protocol not available",                            // 50 ENOPROTOOPT
	"no space left on device",                           // 51 ENOSPC
	"function not implemented",                          // 52 ENOSYS
	"transport endpoint is not connected",               // 53 ENOTCONN
	"not a directory",                                   // 54 ENOTDIR
	"directory not empty",                               // 55 ENOTEMPTY
	"state not recoverable",                             // 56 ENOTRECOVERABLE
	"socket operation on non-socket",                    // 57 ENOTSOCK
	"operation not supported",                           // 58 ENOTSUP
	"inappropriate ioctl for device",                    // 59 ENOTTY
	"no such device or address",                         // 60 ENXIO
	"value too large for defined data type",             // 61 EOVERFLOW
	"owner died",                                        // 62 EOWNERDEAD
	"operation not permitted",                           // 63 EPERM
	"broken pipe",                                       // 64 EPIPE
	"protocol error",                                    // 65 EPROTO
	"protocol not supported",                            // 66 EPROTONOSUPPORT
	"protocol wrong type for socket",                    // 67 EPROTOTYPE
	"numerical result out of range",                     // 68 ERANGE
	"read-only file system",                             // 69 EROFS
	"illegal seek",                                      // 70 ESPIPE
	"no such process",                                   // 71 ESRCH
	"stale file handle",                                 // 72 ESTALE
	"connection timed out",                              // 73 ETIMEDOUT
	"text file busy",                                    // 74 ETXTBSY
	"invalid cross-device link",                         // 75 EXDEV
	"not capable",                                       // 76 ENOTCAPABLE
}

// watM3Errno builds the errno blob (all 76 texts, NUL-separated code
// points) plus the offset table and the two renderers.
func watM3Errno() string {
	var blob []int32
	var offs []int32
	for _, t := range watErrnoTexts {
		offs = append(offs, int32(len(blob)))
		for _, r := range t {
			blob = append(blob, int32(r))
		}
		blob = append(blob, 0)
	}
	var b strings.Builder
	b.WriteString("\n;; errno text table (DESIGN-M3 1b): WASI errno -> Go/Linux text\n(global $zhex_errno_blob (ref $zhex_arr_i32)\n  (array.new_fixed $zhex_arr_i32 " + itoa(len(blob)))
	for _, cp := range blob {
		b.WriteString(" (i32.const " + itoa(int(cp)) + ")")
	}
	b.WriteString("))\n(global $zhex_errno_off (ref $zhex_arr_i32)\n  (array.new_fixed $zhex_arr_i32 " + itoa(len(offs)))
	for _, o := range offs {
		b.WriteString(" (i32.const " + itoa(int(o)) + ")")
	}
	b.WriteString("))\n")
	b.WriteString(watM3ErrnoFuncs)
	return b.String()
}

const watM3ErrnoFuncs = `
(func $zhex_errno_text (param $e i32) (result (ref $zhex_string))
  (local $at i32) (local $end i32) (local $n i32) (local $r (ref null $zhex_string)) (local $i i32)
  (if (i32.or (i32.lt_s (local.get $e) (i32.const 1)) (i32.gt_s (local.get $e) (i32.const 76)))
    (then
      (return (call $zhex_str_concat
        (array.new_fixed $zhex_string 6 (i32.const 101) (i32.const 114) (i32.const 114) (i32.const 110) (i32.const 111) (i32.const 32))
        (call $zhex_int_to_string (i64.extend_i32_s (local.get $e)))))))
  (local.set $at (array.get $zhex_arr_i32 (global.get $zhex_errno_off) (i32.sub (local.get $e) (i32.const 1))))
  (local.set $end (local.get $at))
  (block $nul
    (loop $scan
      (br_if $nul (i32.eqz (array.get $zhex_arr_i32 (global.get $zhex_errno_blob) (local.get $end))))
      (local.set $end (i32.add (local.get $end) (i32.const 1)))
      (br $scan)))
  (local.set $n (i32.sub (local.get $end) (local.get $at)))
  (local.set $r (array.new_default $zhex_string (local.get $n)))
  (block $copied
    (loop $copy
      (br_if $copied (i32.ge_u (local.get $i) (local.get $n)))
      (array.set $zhex_string (local.get $r) (local.get $i)
        (array.get $zhex_arr_i32 (global.get $zhex_errno_blob) (i32.add (local.get $at) (local.get $i))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $copy)))
  (ref.as_non_null (local.get $r)))

;; the interpreter's inner PathError/LinkError text: "<verb> <path>: <text>"
(func $zhex_os_err (param $verb (ref $zhex_string)) (param $path (ref $zhex_string)) (param $e i32) (result (ref $zhex_string))
  (call $zhex_str_concat
    (call $zhex_str_concat
      (call $zhex_str_concat (call $zhex_str_concat (local.get $verb) (call $zhex_space)) (local.get $path))
      (array.new_fixed $zhex_string 2 (i32.const 58) (i32.const 32)))
    (call $zhex_errno_text (local.get $e))))

(func $zhex_space (result (ref $zhex_string)) (array.new_fixed $zhex_string 1 (i32.const 32)))
(func $zhex_cat3 (param $a (ref $zhex_string)) (param $b (ref $zhex_string)) (param $c (ref $zhex_string)) (result (ref $zhex_string))
  (call $zhex_str_concat (call $zhex_str_concat (local.get $a) (local.get $b)) (local.get $c)))
(func $zhex_cat4 (param $a (ref $zhex_string)) (param $b (ref $zhex_string)) (param $c (ref $zhex_string)) (param $d (ref $zhex_string)) (result (ref $zhex_string))
  (call $zhex_str_concat (call $zhex_cat3 (local.get $a) (local.get $b) (local.get $c)) (local.get $d)))
(func $zhex_cat5 (param $a (ref $zhex_string)) (param $b (ref $zhex_string)) (param $c (ref $zhex_string)) (param $d (ref $zhex_string)) (param $e2 (ref $zhex_string)) (result (ref $zhex_string))
  (call $zhex_str_concat (call $zhex_cat4 (local.get $a) (local.get $b) (local.get $c) (local.get $d)) (local.get $e2)))
`

// watM3Bytes: scratch and byte helpers. The scratch payload lives at
// offset 4 behind the [u32 length] header at offset 0; $zhex_stage_utf8_at
// is $zhex_stage_utf8 for an arbitrary destination offset.
const watM3Bytes = `
(func $zhex_scratch_len (result i32) (i32.load (i32.const 0)))

(func $zhex_stage_utf8_at (param $s (ref $zhex_string)) (param $off i32) (result i32)
  (local $n i32) (local $i i32) (local $cp i32) (local $len i32)
  (local $pages i32) (local $w i32)
  (local.set $n (array.len (local.get $s)))
  (local.set $i (i32.const 0))
  (block $scanned
    (loop $scan
      (br_if $scanned (i32.ge_u (local.get $i) (local.get $n)))
      (local.set $cp (array.get $zhex_string (local.get $s) (local.get $i)))
      (local.set $len (i32.add (local.get $len)
        (if (result i32)
          (i32.lt_u (local.get $cp) (i32.const 128))
          (then (i32.const 1))
          (else
            (if (result i32)
              (i32.lt_u (local.get $cp) (i32.const 2048))
              (then (i32.const 2))
              (else
                (if (result i32)
                  (i32.lt_u (local.get $cp) (i32.const 65536))
                  (then (i32.const 3))
                  (else (i32.const 4)))))))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $scan)))
  (local.set $pages (i32.div_u (i32.add (i32.add (local.get $off) (local.get $len)) (i32.const 65535)) (i32.const 65536)))
  (if (i32.gt_u (local.get $pages) (memory.size))
    (then (drop (memory.grow (i32.sub (local.get $pages) (memory.size))))))
  (local.set $i (i32.const 0))
  (local.set $w (local.get $off))
  (block $written
    (loop $write
      (br_if $written (i32.ge_u (local.get $i) (local.get $n)))
      (local.set $cp (array.get $zhex_string (local.get $s) (local.get $i)))
      (if (i32.lt_u (local.get $cp) (i32.const 128))
        (then
          (i32.store8 (local.get $w) (local.get $cp))
          (local.set $w (i32.add (local.get $w) (i32.const 1))))
        (else (if (i32.lt_u (local.get $cp) (i32.const 2048))
          (then
            (i32.store8 (local.get $w) (i32.or (i32.const 192) (i32.shr_u (local.get $cp) (i32.const 6))))
            (i32.store8 (i32.add (local.get $w) (i32.const 1))
              (i32.or (i32.const 128) (i32.and (local.get $cp) (i32.const 63))))
            (local.set $w (i32.add (local.get $w) (i32.const 2))))
          (else (if (i32.lt_u (local.get $cp) (i32.const 65536))
            (then
              (i32.store8 (local.get $w) (i32.or (i32.const 224) (i32.shr_u (local.get $cp) (i32.const 12))))
              (i32.store8 (i32.add (local.get $w) (i32.const 1))
                (i32.or (i32.const 128) (i32.and (i32.shr_u (local.get $cp) (i32.const 6)) (i32.const 63))))
              (i32.store8 (i32.add (local.get $w) (i32.const 2))
                (i32.or (i32.const 128) (i32.and (local.get $cp) (i32.const 63))))
              (local.set $w (i32.add (local.get $w) (i32.const 3))))
            (else
              (i32.store8 (local.get $w) (i32.or (i32.const 240) (i32.shr_u (local.get $cp) (i32.const 18))))
              (i32.store8 (i32.add (local.get $w) (i32.const 1))
                (i32.or (i32.const 128) (i32.and (i32.shr_u (local.get $cp) (i32.const 12)) (i32.const 63))))
              (i32.store8 (i32.add (local.get $w) (i32.const 2))
                (i32.or (i32.const 128) (i32.and (i32.shr_u (local.get $cp) (i32.const 6)) (i32.const 63))))
              (i32.store8 (i32.add (local.get $w) (i32.const 3))
                (i32.or (i32.const 128) (i32.and (local.get $cp) (i32.const 63))))
              (local.set $w (i32.add (local.get $w) (i32.const 4)))))))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $write)))
  (local.get $len))

;; strict UTF-8 validation (RFC 3629 ranges: no surrogates, no overlongs)
(func $zhex_bytes_utf8_ok (param $ptr i32) (param $len i32) (result i32)
  (local $i i32) (local $b i32) (local $need i32) (local $lo i32) (local $hi i32)
  (block $done
    (loop $next
      (br_if $done (i32.ge_u (local.get $i) (local.get $len)))
      (local.set $b (i32.load8_u (i32.add (local.get $ptr) (local.get $i))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (if (i32.lt_u (local.get $b) (i32.const 128)) (then (br $next)))
      (local.set $need (i32.const 0))
      (local.set $lo (i32.const 128))
      (local.set $hi (i32.const 191))
      (if (i32.and (i32.ge_u (local.get $b) (i32.const 194)) (i32.le_u (local.get $b) (i32.const 223)))
        (then (local.set $need (i32.const 1))))
      (if (i32.eq (local.get $b) (i32.const 224))
        (then (local.set $need (i32.const 2)) (local.set $lo (i32.const 160))))
      (if (i32.and (i32.ge_u (local.get $b) (i32.const 225)) (i32.le_u (local.get $b) (i32.const 236)))
        (then (local.set $need (i32.const 2))))
      (if (i32.eq (local.get $b) (i32.const 237))
        (then (local.set $need (i32.const 2)) (local.set $hi (i32.const 159))))
      (if (i32.and (i32.ge_u (local.get $b) (i32.const 238)) (i32.le_u (local.get $b) (i32.const 239)))
        (then (local.set $need (i32.const 2))))
      (if (i32.eq (local.get $b) (i32.const 240))
        (then (local.set $need (i32.const 3)) (local.set $lo (i32.const 144))))
      (if (i32.and (i32.ge_u (local.get $b) (i32.const 241)) (i32.le_u (local.get $b) (i32.const 243)))
        (then (local.set $need (i32.const 3))))
      (if (i32.eq (local.get $b) (i32.const 244))
        (then (local.set $need (i32.const 3)) (local.set $hi (i32.const 143))))
      (if (i32.eqz (local.get $need)) (then (return (i32.const 0))))
      (if (i32.gt_u (i32.add (local.get $i) (local.get $need)) (local.get $len)) (then (return (i32.const 0))))
      (local.set $b (i32.load8_u (i32.add (local.get $ptr) (local.get $i))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (if (i32.or (i32.lt_u (local.get $b) (local.get $lo)) (i32.gt_u (local.get $b) (local.get $hi)))
        (then (return (i32.const 0))))
      (block $conts
        (loop $cont
          (br_if $conts (i32.le_u (local.get $need) (i32.const 1)))
          (local.set $b (i32.load8_u (i32.add (local.get $ptr) (local.get $i))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (if (i32.or (i32.lt_u (local.get $b) (i32.const 128)) (i32.gt_u (local.get $b) (i32.const 191)))
            (then (return (i32.const 0))))
          (local.set $need (i32.sub (local.get $need) (i32.const 1)))
          (br $cont)))
      (br $next)))
  (i32.const 1))

;; lenient byte -> code-point decode (call $zhex_bytes_utf8_ok first when
;; the bytes come from outside the module)
(func $zhex_bytes_to_string (param $ptr i32) (param $len i32) (result (ref $zhex_string))
  (local $i i32) (local $b i32) (local $cp i32) (local $n i32) (local $w i32)
  (local $r (ref null $zhex_string))
  (block $cdone
    (loop $count
      (br_if $cdone (i32.ge_u (local.get $i) (local.get $len)))
      (local.set $b (i32.load8_u (i32.add (local.get $ptr) (local.get $i))))
      (local.set $i (i32.add (local.get $i)
        (if (result i32)
          (i32.lt_u (local.get $b) (i32.const 128))
          (then (i32.const 1))
          (else
            (if (result i32)
              (i32.lt_u (local.get $b) (i32.const 224))
              (then (i32.const 2))
              (else
                (if (result i32)
                  (i32.lt_u (local.get $b) (i32.const 240))
                  (then (i32.const 3))
                  (else (i32.const 4)))))))))
      (local.set $n (i32.add (local.get $n) (i32.const 1)))
      (br $count)))
  (local.set $r (array.new_default $zhex_string (local.get $n)))
  (local.set $i (i32.const 0))
  (block $ddone
    (loop $decode
      (br_if $ddone (i32.ge_u (local.get $w) (local.get $n)))
      (local.set $b (i32.load8_u (i32.add (local.get $ptr) (local.get $i))))
      (local.set $cp
        (if (result i32)
          (i32.lt_u (local.get $b) (i32.const 128))
          (then (local.get $b))
          (else
            (if (result i32)
              (i32.lt_u (local.get $b) (i32.const 224))
              (then
                (i32.or
                  (i32.shl (i32.and (local.get $b) (i32.const 31)) (i32.const 6))
                  (i32.and (i32.load8_u (i32.add (local.get $ptr) (i32.add (local.get $i) (i32.const 1)))) (i32.const 63))))
              (else
                (if (result i32)
                  (i32.lt_u (local.get $b) (i32.const 240))
                  (then
                    (i32.or
                      (i32.or
                        (i32.shl (i32.and (local.get $b) (i32.const 15)) (i32.const 12))
                        (i32.shl (i32.and (i32.load8_u (i32.add (local.get $ptr) (i32.add (local.get $i) (i32.const 1)))) (i32.const 63)) (i32.const 6)))
                      (i32.and (i32.load8_u (i32.add (local.get $ptr) (i32.add (local.get $i) (i32.const 2)))) (i32.const 63))))
                  (else
                    (i32.or
                      (i32.or
                        (i32.shl (i32.and (local.get $b) (i32.const 7)) (i32.const 18))
                        (i32.shl (i32.and (i32.load8_u (i32.add (local.get $ptr) (i32.add (local.get $i) (i32.const 1)))) (i32.const 63)) (i32.const 12)))
                      (i32.or
                        (i32.shl (i32.and (i32.load8_u (i32.add (local.get $ptr) (i32.add (local.get $i) (i32.const 2)))) (i32.const 63)) (i32.const 6))
                        (i32.and (i32.load8_u (i32.add (local.get $ptr) (i32.add (local.get $i) (i32.const 3)))) (i32.const 63)))))))))))
      (local.set $i (i32.add (local.get $i)
        (if (result i32)
          (i32.lt_u (local.get $b) (i32.const 128))
          (then (i32.const 1))
          (else
            (if (result i32)
              (i32.lt_u (local.get $b) (i32.const 224))
              (then (i32.const 2))
              (else
                (if (result i32)
                  (i32.lt_u (local.get $b) (i32.const 240))
                  (then (i32.const 3))
                  (else (i32.const 4)))))))))
      (array.set $zhex_string (local.get $r) (local.get $w) (local.get $cp))
      (local.set $w (i32.add (local.get $w) (i32.const 1)))
      (br $decode)))
  (ref.as_non_null (local.get $r)))

;; print owns its newline since M3 (Surprises 2): raw bytes, no host append
(func $zhex_print_line (param $s (ref $zhex_string))
  (local $len i32)
  (local.set $len (call $zhex_stage_utf8_at
    (call $zhex_str_concat (local.get $s) (array.new_fixed $zhex_string 1 (i32.const 10)))
    (i32.const 0)))
  (call $zhex_host_print (i32.const 0) (local.get $len)))
`

// watM3Path: the pure lexical path words (§3), mirroring Go's Unix
// filepath.Clean/Base/Ext/Dir and the interpreter's joinPath exactly
// (an absolute child replaces the base; Join skips empty elements).
const watM3Path = `
(func $zhex_path_clean (param $s (ref $zhex_string)) (result (ref $zhex_string))
  ;; Go filepath.Clean, code-point domain, separator '/'; dot cases use
  ;; nested guards because WAT and/or do not short-circuit and an unchecked
  ;; s[r+1]/s[r+2] read traps on the array bound
  (local $n i32)
  (local $rooted i32)
  (local $r i32)
  (local $w i32)
  (local $dotdot i32)
  (local $dd i32)
  (local $c i32)
  (local $i i32)
  (local $out (ref null $zhex_arr_i32))
  (local $res (ref null $zhex_string))
  (local.set $n (array.len (local.get $s)))
  (if (i32.eqz (local.get $n))
    (then
      (return (array.new_fixed $zhex_string 1 (i32.const 46)))
    )
  )
  (local.set $out (array.new_default $zhex_arr_i32 (local.get $n)))
  (if (i32.eq (array.get $zhex_string (local.get $s) (i32.const 0)) (i32.const 47))
    (then
      (local.set $rooted (i32.const 1))
      (array.set $zhex_arr_i32 (local.get $out) (i32.const 0) (i32.const 47))
      (local.set $w (i32.const 1))
      (local.set $r (i32.const 1))
      (local.set $dotdot (i32.const 1))
    )
  )
  (block $done
    (loop $next
      (br_if $done (i32.ge_u (local.get $r) (local.get $n)))
      (local.set $c (array.get $zhex_string (local.get $s) (local.get $r)))
      ;; empty element
      (if (i32.eq (local.get $c) (i32.const 47))
        (then
          (local.set $r (i32.add (local.get $r) (i32.const 1)))
          (br $next)
        )
      )
      (if (i32.eq (local.get $c) (i32.const 46))
        (then
          ;; "." alone at the end
          (if (i32.eq (i32.add (local.get $r) (i32.const 1)) (local.get $n))
            (then
              (local.set $r (i32.add (local.get $r) (i32.const 1)))
              (br $next)
            )
          )
          (if (i32.lt_u (i32.add (local.get $r) (i32.const 1)) (local.get $n))
            (then
              ;; "./" - a plain dot element
              (if (i32.eq (array.get $zhex_string (local.get $s) (i32.add (local.get $r) (i32.const 1))) (i32.const 47))
                (then
                  (local.set $r (i32.add (local.get $r) (i32.const 1)))
                  (br $next)
                )
              )
              ;; ".." candidate: both dots, then end-or-slash after them
              (if (i32.eq (array.get $zhex_string (local.get $s) (i32.add (local.get $r) (i32.const 1))) (i32.const 46))
                (then
                  (local.set $dd (i32.const 0))
                  (if (i32.eq (i32.add (local.get $r) (i32.const 2)) (local.get $n))
                    (then
                      (local.set $dd (i32.const 1))
                    )
                  )
                  (if (i32.lt_u (i32.add (local.get $r) (i32.const 2)) (local.get $n))
                    (then
                      (if (i32.eq (array.get $zhex_string (local.get $s) (i32.add (local.get $r) (i32.const 2))) (i32.const 47))
                        (then
                          (local.set $dd (i32.const 1))
                        )
                      )
                    )
                  )
                  (if (local.get $dd)
                    (then
                      (local.set $r (i32.add (local.get $r) (i32.const 2)))
                      (if (i32.gt_u (local.get $w) (local.get $dotdot))
                        (then
                          (local.set $w (i32.sub (local.get $w) (i32.const 1)))
                          (block $back_done
                            (loop $back
                              (br_if $back_done (i32.le_u (local.get $w) (local.get $dotdot)))
                              (br_if $back_done (i32.eq (array.get $zhex_arr_i32 (local.get $out) (local.get $w)) (i32.const 47)))
                              (local.set $w (i32.sub (local.get $w) (i32.const 1)))
                              (br $back)
                            )
                          )
                        )
                        (else
                          (if (i32.eqz (local.get $rooted))
                            (then
                              (if (i32.gt_u (local.get $w) (i32.const 0))
                                (then
                                  (array.set $zhex_arr_i32 (local.get $out) (local.get $w) (i32.const 47))
                                  (local.set $w (i32.add (local.get $w) (i32.const 1)))
                                )
                              )
                              (array.set $zhex_arr_i32 (local.get $out) (local.get $w) (i32.const 46))
                              (local.set $w (i32.add (local.get $w) (i32.const 1)))
                              (array.set $zhex_arr_i32 (local.get $out) (local.get $w) (i32.const 46))
                              (local.set $w (i32.add (local.get $w) (i32.const 1)))
                              (local.set $dotdot (local.get $w))
                            )
                          )
                        )
                      )
                      (br $next)
                    )
                  )
                )
              )
            )
          )
        )
      )
      ;; plain element
      (if
        (i32.or
          (i32.and (local.get $rooted) (i32.ne (local.get $w) (i32.const 1)))
          (i32.and (i32.eqz (local.get $rooted)) (i32.ne (local.get $w) (i32.const 0)))
        )
        (then
          (array.set $zhex_arr_i32 (local.get $out) (local.get $w) (i32.const 47))
          (local.set $w (i32.add (local.get $w) (i32.const 1)))
        )
      )
      (block $elem
        (loop $copy
          (br_if $elem (i32.ge_u (local.get $r) (local.get $n)))
          (local.set $c (array.get $zhex_string (local.get $s) (local.get $r)))
          (br_if $elem (i32.eq (local.get $c) (i32.const 47)))
          (array.set $zhex_arr_i32 (local.get $out) (local.get $w) (local.get $c))
          (local.set $w (i32.add (local.get $w) (i32.const 1)))
          (local.set $r (i32.add (local.get $r) (i32.const 1)))
          (br $copy)
        )
      )
      (br $next)
    )
  )
  (if (i32.eqz (local.get $w))
    (then
      (return (array.new_fixed $zhex_string 1 (i32.const 46)))
    )
  )
  (local.set $res (array.new_default $zhex_string (local.get $w)))
  (block $copied
    (loop $cp
      (br_if $copied (i32.ge_u (local.get $i) (local.get $w)))
      (array.set $zhex_string (local.get $res) (local.get $i) (array.get $zhex_arr_i32 (local.get $out) (local.get $i)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $cp)
    )
  )
  (ref.as_non_null (local.get $res))
)

(func $zhex_path_join (param $base (ref $zhex_string)) (param $child (ref $zhex_string)) (result (ref $zhex_string))
  ;; joinPath (files.go): an absolute child replaces the base; otherwise
  ;; filepath.Join semantics - empty elements skipped, then Clean
  (local $nb i32)
  (local $nc i32)
  (local.set $nb (array.len (local.get $base)))
  (local.set $nc (array.len (local.get $child)))
  (if (i32.gt_u (local.get $nc) (i32.const 0))
    (then
      (if (i32.eq (array.get $zhex_string (local.get $child) (i32.const 0)) (i32.const 47))
        (then
          (return (call $zhex_path_clean (local.get $child)))
        )
      )
    )
  )
  (if (i32.eqz (local.get $nb))
    (then
      (return (local.get $child))
    )
  )
  (if (i32.eqz (local.get $nc))
    (then
      (return (call $zhex_path_clean (local.get $base)))
    )
  )
  (call $zhex_path_clean
    (call $zhex_cat3 (local.get $base) (array.new_fixed $zhex_string 1 (i32.const 47)) (local.get $child))
  )
)

(func $zhex_path_base (param $s (ref $zhex_string)) (result (ref $zhex_string))
  ;; filepath.Base: strip trailing separators; all slashes -> "/"
  (local $n i32)
  (local $i i32)
  (local.set $n (array.len (local.get $s)))
  (if (i32.eqz (local.get $n))
    (then
      (return (array.new_fixed $zhex_string 1 (i32.const 46)))
    )
  )
  (block $trimmed
    (loop $t
      (br_if $trimmed (i32.eqz (local.get $n)))
      (br_if $trimmed (i32.ne (array.get $zhex_string (local.get $s) (i32.sub (local.get $n) (i32.const 1))) (i32.const 47)))
      (local.set $n (i32.sub (local.get $n) (i32.const 1)))
      (br $t)
    )
  )
  (if (i32.eqz (local.get $n))
    (then
      (return (array.new_fixed $zhex_string 1 (i32.const 47)))
    )
  )
  (local.set $i (local.get $n))
  (block $found
    (loop $scan
      (br_if $found (i32.eqz (local.get $i)))
      (local.set $i (i32.sub (local.get $i) (i32.const 1)))
      (br_if $found (i32.eq (array.get $zhex_string (local.get $s) (local.get $i)) (i32.const 47)))
      (br $scan)
    )
  )
  ;; i == 0 here means every index was checked and no slash exists
  (if (i32.eqz (local.get $i))
    (then
      (return (call $zhex_str_sub (local.get $s) (i32.const 0) (local.get $n)))
    )
  )
  (call $zhex_str_sub (local.get $s) (i32.add (local.get $i) (i32.const 1)) (local.get $n))
)

(func $zhex_path_extension (param $s (ref $zhex_string)) (result (ref $zhex_string))
  ;; filepath.Ext: suffix from the final dot of the final element, dot
  ;; included (".bashrc" -> ".bashrc"); none -> empty
  (local $n i32)
  (local $i i32)
  (local.set $n (array.len (local.get $s)))
  (local.set $i (local.get $n))
  (block $found
    (loop $scan
      (br_if $found (i32.eqz (local.get $i)))
      (local.set $i (i32.sub (local.get $i) (i32.const 1)))
      (if (i32.eq (array.get $zhex_string (local.get $s) (local.get $i)) (i32.const 46))
        (then
          (return (call $zhex_str_sub (local.get $s) (local.get $i) (local.get $n)))
        )
      )
      (br_if $found (i32.eq (array.get $zhex_string (local.get $s) (local.get $i)) (i32.const 47)))
      (br $scan)
    )
  )
  (array.new_default $zhex_string (i32.const 0))
)

(func $zhex_path_dir_of (param $s (ref $zhex_string)) (result (ref $zhex_string))
  ;; filepath.Dir = Clean of everything through the final separator; no
  ;; separator anywhere -> "." (the tail distinguishes a leading slash from
  ;; a bare name because both scan exits land at i == 0)
  (local $n i32)
  (local $i i32)
  (local.set $n (array.len (local.get $s)))
  ;; the empty string is "." before any index-0 read: i32.and in the tail
  ;; below evaluates both operands, so s[0] must never run on an empty s
  (if (i32.eqz (local.get $n))
    (then
      (return (array.new_fixed $zhex_string 1 (i32.const 46)))
    )
  )
  (local.set $i (local.get $n))
  (block $found
    (loop $scan
      (br_if $found (i32.eqz (local.get $i)))
      (local.set $i (i32.sub (local.get $i) (i32.const 1)))
      (br_if $found (i32.eq (array.get $zhex_string (local.get $s) (local.get $i)) (i32.const 47)))
      (br $scan)
    )
  )
  (if
    (i32.and
      (i32.eqz (local.get $i))
      (i32.ne (array.get $zhex_string (local.get $s) (i32.const 0)) (i32.const 47))
    )
    (then
      (return (array.new_fixed $zhex_string 1 (i32.const 46)))
    )
  )
  (call $zhex_path_clean (call $zhex_str_sub (local.get $s) (i32.const 0) (i32.add (local.get $i) (i32.const 1))))
)
`

// watM3Buffer: the buffer suite (§4) - pure runtime, no imports, no element
// limit (Surprises 10). line/set-line are bounds-checked Results in the
// BufferError family with "buffer index <i> is outside 0..<n-1>". add-line
// grows amortized like list append (#187): $lines holds capacity slack,
// $len tracks the line count.
const watM3Buffer = `
(func $zhex_buffer_new (result (ref $zhex_buffer))
  (struct.new $zhex_buffer (array.new_default $zhex_arr_ref (i32.const 0)) (i32.const 0)))

(func $zhex_buffer_add_line (param $b (ref $zhex_buffer)) (param $s (ref $zhex_string)) (result (ref $zhex_buffer))
  (local $old (ref null $zhex_arr_ref))
  (local $n i32)
  (local $cap i32)
  (local $fresh (ref null $zhex_arr_ref))
  (local $i i32)
  (local.set $n (struct.get $zhex_buffer $len (local.get $b)))
  (local.set $old (struct.get $zhex_buffer $lines (local.get $b)))
  (local.set $cap (array.len (ref.as_non_null (local.get $old))))
  (if (i32.lt_u (local.get $n) (local.get $cap))
    (then
      (array.set $zhex_arr_ref (ref.as_non_null (local.get $old)) (local.get $n) (local.get $s))
      (struct.set $zhex_buffer $len (local.get $b) (i32.add (local.get $n) (i32.const 1))))
    (else
      (if (i32.lt_u (local.get $cap) (i32.const 4))
        (then (local.set $cap (i32.const 4)))
        (else (local.set $cap (i32.add (local.get $cap) (local.get $cap)))))
      (local.set $fresh (array.new_default $zhex_arr_ref (local.get $cap)))
      (block $copied
        (loop $copy
          (br_if $copied (i32.ge_u (local.get $i) (local.get $n)))
          (array.set $zhex_arr_ref (local.get $fresh) (local.get $i)
            (array.get $zhex_arr_ref (ref.as_non_null (local.get $old)) (local.get $i)))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $copy)))
      (array.set $zhex_arr_ref (local.get $fresh) (local.get $n) (local.get $s))
      (struct.set $zhex_buffer $lines (local.get $b) (local.get $fresh))
      (struct.set $zhex_buffer $len (local.get $b) (i32.add (local.get $n) (i32.const 1)))))
  (local.get $b))

(func $zhex_buffer_line_count (param $b (ref $zhex_buffer)) (result i64)
  (i64.extend_i32_u (struct.get $zhex_buffer $len (local.get $b))))

(func $zhex_buffer_line (param $b (ref $zhex_buffer)) (param $index i64) (result (ref $zhex_result))
  (local $n i32)
  (local.set $n (struct.get $zhex_buffer $len (local.get $b)))
  (if (i64.ge_u (local.get $index) (i64.extend_i32_u (local.get $n)))
    (then
      (return (struct.new $zhex_result (i32.const 1)
        (call $zhex_errv (i32.const 4)
          (call $zhex_str_concat
            (call $zhex_str_concat
              (call $zhex_str_concat (call $m3_buffer_index) (call $zhex_int_to_string (local.get $index)))
              (array.new_fixed $zhex_string 15 (i32.const 32) (i32.const 105) (i32.const 115) (i32.const 32) (i32.const 111) (i32.const 117) (i32.const 116) (i32.const 115) (i32.const 105) (i32.const 100) (i32.const 101) (i32.const 32) (i32.const 48) (i32.const 46) (i32.const 46)))
            (call $zhex_int_to_string (i64.sub (i64.extend_i32_u (local.get $n)) (i64.const 1)))))))))
  (struct.new $zhex_result (i32.const 0)
    (ref.cast (ref $zhex_string)
    (array.get $zhex_arr_ref (struct.get $zhex_buffer $lines (local.get $b)) (i32.wrap_i64 (local.get $index))))))

(func $zhex_buffer_set_line (param $b (ref $zhex_buffer)) (param $index i64) (param $s (ref $zhex_string)) (result (ref $zhex_result))
  (local $n i32)
  (local.set $n (struct.get $zhex_buffer $len (local.get $b)))
  (if (i64.ge_u (local.get $index) (i64.extend_i32_u (local.get $n)))
    (then
      (return (struct.new $zhex_result (i32.const 1)
        (call $zhex_errv (i32.const 4)
          (call $zhex_str_concat
            (call $zhex_str_concat
              (call $zhex_str_concat (call $m3_buffer_index) (call $zhex_int_to_string (local.get $index)))
              (array.new_fixed $zhex_string 15 (i32.const 32) (i32.const 105) (i32.const 115) (i32.const 32) (i32.const 111) (i32.const 117) (i32.const 116) (i32.const 115) (i32.const 105) (i32.const 100) (i32.const 101) (i32.const 32) (i32.const 48) (i32.const 46) (i32.const 46)))
            (call $zhex_int_to_string (i64.sub (i64.extend_i32_u (local.get $n)) (i64.const 1)))))))))
  (array.set $zhex_arr_ref (struct.get $zhex_buffer $lines (local.get $b)) (i32.wrap_i64 (local.get $index)) (local.get $s))
  (struct.new $zhex_result (i32.const 0) (local.get $b)))

(func $zhex_buffer_to_text (param $b (ref $zhex_buffer)) (result (ref $zhex_string))
  (local $lines (ref null $zhex_arr_ref))
  (local $n i32)
  (local $i i32)
  (local $j i32)
  (local $w i32)
  (local $part (ref null $zhex_string))
  (local $plen i32)
  (local $r (ref null $zhex_string))
  (local.set $lines (struct.get $zhex_buffer $lines (local.get $b)))
  (local.set $n (struct.get $zhex_buffer $len (local.get $b)))
  (if (i32.eqz (local.get $n)) (then (return (array.new_fixed $zhex_string 0))))
  ;; one pass to measure, one to fill - repeated str_concat was quadratic
  ;; in the line count (#187); this mirrors the interpreter's strings.Join
  (block $measured
    (loop $measure
      (br_if $measured (i32.ge_u (local.get $i) (local.get $n)))
      (local.set $plen (i32.add (local.get $plen)
        (array.len (ref.cast (ref $zhex_string)
          (array.get $zhex_arr_ref (ref.as_non_null (local.get $lines)) (local.get $i))))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $measure)))
  (local.set $r (array.new_default $zhex_string
    (i32.sub (i32.add (local.get $plen) (local.get $n)) (i32.const 1))))
  (local.set $i (i32.const 0))
  (local.set $w (i32.const 0))
  (block $filled
    (loop $fill
      (br_if $filled (i32.ge_u (local.get $i) (local.get $n)))
      (if (i32.gt_u (local.get $i) (i32.const 0))
        (then
          (array.set $zhex_string (ref.as_non_null (local.get $r)) (local.get $w) (i32.const 10))
          (local.set $w (i32.add (local.get $w) (i32.const 1)))))
      (local.set $part (ref.cast (ref $zhex_string)
        (array.get $zhex_arr_ref (ref.as_non_null (local.get $lines)) (local.get $i))))
      (local.set $plen (array.len (ref.as_non_null (local.get $part))))
      (local.set $j (i32.const 0))
      (block $cpdone
        (loop $cp
          (br_if $cpdone (i32.ge_u (local.get $j) (local.get $plen)))
          (array.set $zhex_string (ref.as_non_null (local.get $r)) (local.get $w)
            (array.get $zhex_string (ref.as_non_null (local.get $part)) (local.get $j)))
          (local.set $w (i32.add (local.get $w) (i32.const 1)))
          (local.set $j (i32.add (local.get $j) (i32.const 1)))
          (br $cp)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $fill)))
  (ref.as_non_null (local.get $r)))
`

// watM3Readline: read-line(prompt) (§4) - pending bytes stay in a GC
// buffer, compacted only on refill with amortized growth for long lines.
// Linear memory at 65536 is transient host-read and UTF-8 decoding scratch.
// Chomp strips one "\n" then one "\r" only when the newline was present;
// a partial final line at EOF is delivered unchomped.
const watM3Readline = `
(func $m3_stdin_ensure (param $end i32)
  (local $pages i32)
  (local.set $pages (i32.div_u (i32.add (local.get $end) (i32.const 65535)) (i32.const 65536)))
  (if (i32.gt_u (local.get $pages) (memory.size))
    (then (drop (memory.grow (i32.sub (local.get $pages) (memory.size)))))))

(func $m3_stdin_deliver (param $base i32) (param $len i32) (result (ref $zhex_result))
  (local $i i32)
  (call $m3_stdin_ensure (i32.add (i32.const 65536) (local.get $len)))
  (block $staged
    (loop $stage
      (br_if $staged (i32.ge_u (local.get $i) (local.get $len)))
      (i32.store8 (i32.add (i32.const 65536) (local.get $i))
        (array.get $zhex_arr_i32 (ref.as_non_null (global.get $zhex_stdin_buf))
          (i32.add (local.get $base) (local.get $i))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $stage)))
  (if (i32.eqz (call $zhex_bytes_utf8_ok (i32.const 65536) (local.get $len)))
    (then
      (return (struct.new $zhex_result (i32.const 1)
        (call $zhex_errv (i32.const 2) (call $m3_input_utf8))))))
  (struct.new $zhex_result (i32.const 0)
    (call $zhex_bytes_to_string (i32.const 65536) (local.get $len))))

(func $zhex_read_line (param $prompt (ref $zhex_string)) (result (ref $zhex_result))
  (local $plen i32)
  (local $base i32)
  (local $k i32)
  (local $m i32)
  (local $got i32)
  (local $i i32)
  (local $fresh (ref null $zhex_arr_i32))
  (local.set $plen (call $zhex_stage_utf8_at (local.get $prompt) (i32.const 0)))
  (call $zhex_host_print (i32.const 0) (local.get $plen))
  (if (ref.is_null (global.get $zhex_stdin_buf))
    (then (global.set $zhex_stdin_buf
      (array.new_default $zhex_arr_i32 (global.get $zhex_stdin_cap)))))
  (block $delivered
    (loop $again
      (local.set $k (global.get $zhex_stdin_bufpos))
      (block $miss
        (loop $cmp
          (br_if $miss (i32.ge_u (local.get $k) (global.get $zhex_stdin_buflen)))
          (br_if $delivered (i32.eq
            (array.get $zhex_arr_i32 (ref.as_non_null (global.get $zhex_stdin_buf)) (local.get $k)) (i32.const 10)))
          (local.set $k (i32.add (local.get $k) (i32.const 1)))
          (br $cmp)))
      ;; Compact only on refill, rather than after every delivered line.
      (if (i32.gt_u (global.get $zhex_stdin_bufpos) (i32.const 0))
        (then
          (array.copy $zhex_arr_i32 $zhex_arr_i32
            (ref.as_non_null (global.get $zhex_stdin_buf)) (i32.const 0)
            (ref.as_non_null (global.get $zhex_stdin_buf)) (global.get $zhex_stdin_bufpos)
            (i32.sub (global.get $zhex_stdin_buflen) (global.get $zhex_stdin_bufpos)))
          (global.set $zhex_stdin_buflen (i32.sub (global.get $zhex_stdin_buflen) (global.get $zhex_stdin_bufpos)))
          (global.set $zhex_stdin_bufpos (i32.const 0))))
      (if (i32.ge_u (global.get $zhex_stdin_buflen) (global.get $zhex_stdin_cap))
        (then
          (global.set $zhex_stdin_cap (i32.add (global.get $zhex_stdin_cap) (global.get $zhex_stdin_cap)))
          (local.set $fresh (array.new_default $zhex_arr_i32 (global.get $zhex_stdin_cap)))
          (array.copy $zhex_arr_i32 $zhex_arr_i32 (ref.as_non_null (local.get $fresh)) (i32.const 0)
            (ref.as_non_null (global.get $zhex_stdin_buf)) (i32.const 0) (global.get $zhex_stdin_buflen))
          (global.set $zhex_stdin_buf (local.get $fresh))))
      (call $m3_stdin_ensure (i32.add (i32.const 65536) (global.get $zhex_stdin_cap)))
      (local.set $got (call $zhex_host_stdin_read (i32.const 65536)
        (i32.sub (global.get $zhex_stdin_cap) (global.get $zhex_stdin_buflen))))
      (if (i32.eq (local.get $got) (i32.const -1))
        (then
          (return (struct.new $zhex_result (i32.const 1)
            (call $zhex_errv_reason (i32.const 2) (call $m3_input_failed) (call $m3_d_read_failed))))))
      (if (i32.eqz (local.get $got))
        (then
          (if (i32.eqz (global.get $zhex_stdin_buflen))
            (then
              (return (struct.new $zhex_result (i32.const 1)
                (call $zhex_errv_reason (i32.const 2) (call $m3_end_of_input) (call $m3_d_end_of_file))))))
          (local.set $m (global.get $zhex_stdin_buflen))
          (global.set $zhex_stdin_buflen (i32.const 0))
          (return (call $m3_stdin_deliver (i32.const 0) (local.get $m)))))
      (local.set $i (i32.const 0))
      (block $copied
        (loop $copy
          (br_if $copied (i32.ge_u (local.get $i) (local.get $got)))
          (array.set $zhex_arr_i32 (ref.as_non_null (global.get $zhex_stdin_buf))
            (i32.add (global.get $zhex_stdin_buflen) (local.get $i))
            (i32.load8_u (i32.add (i32.const 65536) (local.get $i))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $copy)))
      (global.set $zhex_stdin_buflen (i32.add (global.get $zhex_stdin_buflen) (local.get $got)))
      (br $again)))
  (local.set $base (global.get $zhex_stdin_bufpos))
  (local.set $m (i32.sub (local.get $k) (local.get $base)))
  (if (i32.gt_u (local.get $m) (i32.const 0))
    (then
      (if (i32.eq (array.get $zhex_arr_i32 (ref.as_non_null (global.get $zhex_stdin_buf))
            (i32.sub (local.get $k) (i32.const 1))) (i32.const 13))
        (then (local.set $m (i32.sub (local.get $m) (i32.const 1)))))))
  (global.set $zhex_stdin_bufpos (i32.add (local.get $k) (i32.const 1)))
  (call $m3_stdin_deliver (local.get $base) (local.get $m)))
`

// watM3ErrorInspect (#135): the error.* inspection readers over the §2 err
// value — message is its $message field, detail scans the alternating
// key/value $details array and yields "" when the array is null or the key
// is absent (the interpreter's empty map read). Both take the payload as a
// plain eq reference the way catch binds it; in-subset errors are always
// $zhex_err structs, so the direct cast is total.
const watM3ErrorInspect = `
(func $zhex_error_message (param $e (ref null eq)) (result (ref $zhex_string))
  (ref.as_non_null (struct.get $zhex_err $message (ref.cast (ref $zhex_err) (local.get $e)))))

(func $zhex_error_detail (param $e (ref null eq)) (param $key (ref $zhex_string)) (result (ref $zhex_string))
  (local $d (ref null $zhex_arr_str))
  (local $i i32)
  (local.set $d (struct.get $zhex_err $details (ref.cast (ref $zhex_err) (local.get $e))))
  (block $absent
    (if (ref.is_null (local.get $d))
      (then (return (array.new_fixed $zhex_string 0))))
    (loop $scan
      (br_if $absent (i32.ge_u (local.get $i) (array.len (ref.as_non_null (local.get $d)))))
      (if (call $zhex_str_eq (array.get $zhex_arr_str (ref.as_non_null (local.get $d)) (local.get $i)) (local.get $key))
        (then (return (array.get $zhex_arr_str (ref.as_non_null (local.get $d)) (i32.add (local.get $i) (i32.const 1))))))
      (local.set $i (i32.add (local.get $i) (i32.const 2)))
      (br $scan)))
  (array.new_fixed $zhex_string 0))
`

// watM3File: the file words over the fifteen $zhex_host_* imports (§3) and
// the file.stream helpers with hard fail code 17. Paths stage at offset 0;
// the second simultaneous string stages at 65536. Messages quote paths with
// $zhex_quote and render inner errors via $zhex_os_err (§1b verb table).
const watM3File = `
(func $zhex_file_read (param $path (ref $zhex_string)) (result (ref $zhex_result))
  (local $len i32) (local $e i32) (local $n i32)
  (local.set $len (call $zhex_stage_utf8_at (local.get $path) (i32.const 0)))
  (local.set $e (call $zhex_host_fs_read_file (i32.const 0) (local.get $len)))
  (if (i32.eqz (local.get $e))
    (then
      (local.set $n (call $zhex_scratch_len))
      (if (i32.eqz (call $zhex_bytes_utf8_ok (i32.const 4) (local.get $n)))
        (then
          (return (struct.new $zhex_result (i32.const 1)
            (call $zhex_errv (i32.const 3)
              (call $zhex_cat3 (call $m3_file_word) (call $zhex_quote (local.get $path)) (call $m3_is_utf8_b)))))))
      (return (struct.new $zhex_result (i32.const 0)
        (call $zhex_bytes_to_string (i32.const 4) (local.get $n))))))
  (if (i32.eq (local.get $e) (i32.const 31))
    (then
      (return (struct.new $zhex_result (i32.const 1)
        (call $zhex_errv (i32.const 3)
          (call $zhex_cat4 (call $m3_cannot_open) (call $zhex_quote (local.get $path)) (call $m3_colon_sp)
            (call $zhex_os_err (call $m3_v_read) (local.get $path) (local.get $e))))))))
  (struct.new $zhex_result (i32.const 1)
    (call $zhex_errv (i32.const 3)
      (call $zhex_cat4 (call $m3_cannot_open) (call $zhex_quote (local.get $path)) (call $m3_colon_sp)
        (call $zhex_os_err (call $m3_v_open) (local.get $path) (local.get $e))))))

(func $zhex_file_open (param $path (ref $zhex_string)) (result (ref $zhex_result))
  (local $len i32) (local $h i32)
  (local.set $len (call $zhex_stage_utf8_at (local.get $path) (i32.const 0)))
  (local.set $h (call $zhex_host_fs_open_read (i32.const 0) (local.get $len)))
  (if (i32.lt_s (local.get $h) (i32.const 0))
    (then
      (return (struct.new $zhex_result (i32.const 1)
        (call $zhex_errv (i32.const 3)
          (call $zhex_cat4 (call $m3_cannot_open) (call $zhex_quote (local.get $path)) (call $m3_colon_sp)
            (call $zhex_os_err (call $m3_v_open) (local.get $path) (i32.sub (i32.const 0) (local.get $h)))))))))
  (struct.new $zhex_result (i32.const 0)
    (struct.new $zhex_file (local.get $path) (local.get $h) (i32.const 0))))

(func $zhex_file_read_line (param $f (ref $zhex_file)) (result (ref $zhex_result))
  (local $path (ref $zhex_string)) (local $r i32) (local $n i32)
  (local.set $path (struct.get $zhex_file $path (local.get $f)))
  (if (i32.ne (struct.get $zhex_file $closed (local.get $f)) (i32.const 0))
    (then
      (return (struct.new $zhex_result (i32.const 1)
        (call $zhex_errv (i32.const 3)
          (call $zhex_cat3 (call $m3_file_word) (call $zhex_quote (local.get $path)) (call $m3_is_closed_b)))))))
  (local.set $r (call $zhex_host_fs_read_line (struct.get $zhex_file $handle (local.get $f))))
  (if (i32.eqz (local.get $r))
    (then
      (return (struct.new $zhex_result (i32.const 1)
        (call $zhex_errv (i32.const 3) (call $m3_end_of_file))))))
  (if (i32.lt_s (local.get $r) (i32.const 0))
    (then
      (return (struct.new $zhex_result (i32.const 1)
        (call $zhex_errv (i32.const 3)
          (call $zhex_cat4 (call $m3_reading) (call $zhex_quote (local.get $path)) (call $m3_failed)
            (call $zhex_os_err (call $m3_v_read) (local.get $path) (i32.sub (i32.const 0) (local.get $r)))))))))
  (local.set $n (call $zhex_scratch_len))
  (if (i32.and (i32.gt_u (local.get $n) (i32.const 0))
               (i32.eq (i32.load8_u (i32.add (i32.const 3) (local.get $n))) (i32.const 10)))
    (then
      (local.set $n (i32.sub (local.get $n) (i32.const 1)))
      (if (i32.and (i32.gt_u (local.get $n) (i32.const 0))
                   (i32.eq (i32.load8_u (i32.add (i32.const 3) (local.get $n))) (i32.const 13)))
        (then (local.set $n (i32.sub (local.get $n) (i32.const 1)))))))
  (if (i32.eqz (call $zhex_bytes_utf8_ok (i32.const 4) (local.get $n)))
    (then
      (return (struct.new $zhex_result (i32.const 1)
        (call $zhex_errv (i32.const 3)
          (call $zhex_cat3 (call $m3_file_word) (call $zhex_quote (local.get $path)) (call $m3_is_utf8_b)))))))
  (struct.new $zhex_result (i32.const 0) (call $zhex_bytes_to_string (i32.const 4) (local.get $n))))

(func $zhex_file_close (param $f (ref $zhex_file))
  (if (i32.eqz (struct.get $zhex_file $closed (local.get $f)))
    (then
      (drop (call $zhex_host_fs_close (struct.get $zhex_file $handle (local.get $f))))
      (struct.set $zhex_file $closed (local.get $f) (i32.const 1)))))

(func $zhex_file_write (param $contents (ref $zhex_string)) (param $path (ref $zhex_string)) (param $append i32) (result (ref $zhex_result))
  (local $lenp i32) (local $lenc i32) (local $e i32)
  (local.set $lenp (call $zhex_stage_utf8_at (local.get $path) (i32.const 0)))
  (local.set $lenc (call $zhex_stage_utf8_at (local.get $contents) (i32.const 65536)))
  (local.set $e (call $zhex_host_fs_write_file (i32.const 0) (local.get $lenp) (i32.const 65536) (local.get $lenc) (local.get $append)))
  (if (i32.eqz (local.get $e))
    (then (return (struct.new $zhex_result (i32.const 0) (ref.null eq)))))
  (struct.new $zhex_result (i32.const 1)
    (call $zhex_errv (i32.const 3)
      (call $zhex_cat5 (call $m3_cannot_open) (call $zhex_quote (local.get $path)) (call $m3_for_writing) (call $m3_colon_sp)
        (call $zhex_os_err (call $m3_v_open) (local.get $path) (local.get $e))))))

(func $zhex_file_list (param $path (ref $zhex_string)) (result (ref $zhex_result))
  (local $len i32) (local $e i32) (local $n i32) (local $count i32) (local $cursor i32) (local $elen i32)
  (local $list (ref $zhex_list_ref))
  (local.set $len (call $zhex_stage_utf8_at (local.get $path) (i32.const 0)))
  (local.set $e (call $zhex_host_fs_list (i32.const 0) (local.get $len)))
  (if (i32.eqz (local.get $e))
    (then
      (local.set $n (call $zhex_scratch_len))
      (local.set $count (i32.load (i32.const 4)))
      (if (i32.gt_u (local.get $count) (i32.const 1000000))
        (then
          (return (struct.new $zhex_result (i32.const 1)
            (call $zhex_errv (i32.const 3) (call $m3_list_limit))))))
      (local.set $list (call $zhex_list_new_ref))
      (local.set $cursor (i32.const 8))
      (block $listed
        (loop $entry
          (br_if $listed (i32.ge_u (local.get $cursor) (i32.add (i32.const 4) (local.get $n))))
          (local.set $elen (i32.load (local.get $cursor)))
          (drop (call $zhex_list_append_ref (local.get $list)
            (call $zhex_bytes_to_string (i32.add (local.get $cursor) (i32.const 4)) (local.get $elen))))
          (local.set $cursor (i32.add (local.get $cursor) (i32.add (i32.const 4) (local.get $elen))))
          (br $entry)))
      (return (struct.new $zhex_result (i32.const 0) (local.get $list)))))
  (struct.new $zhex_result (i32.const 1)
    (call $zhex_errv (i32.const 3)
      (call $zhex_cat4 (call $m3_cannot_list) (call $zhex_quote (local.get $path)) (call $m3_colon_sp)
        (call $zhex_os_err (call $m3_v_open) (local.get $path) (local.get $e))))))

(func $zhex_file_exists (param $path (ref $zhex_string)) (result i32)
  (local $len i32) (local $e i32)
  (local.set $len (call $zhex_stage_utf8_at (local.get $path) (i32.const 0)))
  (local.set $e (call $zhex_host_fs_stat (i32.const 0) (local.get $len)))
  (if (i32.eq (local.get $e) (i32.const 44)) (then (return (i32.const 0))))
  (if (i32.eqz (local.get $e)) (then (return (i32.const 1))))
  (call $zhex_fail_msg (i32.const 18)
    (call $zhex_cat4 (call $m3_cannot_check) (call $zhex_quote (local.get $path)) (call $m3_colon_sp)
      (call $zhex_os_err (call $m3_v_stat) (local.get $path) (local.get $e))))
  (i32.const 0))

(func $zhex_file_size (param $path (ref $zhex_string)) (result (ref $zhex_result))
  (local $len i32) (local $e i32)
  (local.set $len (call $zhex_stage_utf8_at (local.get $path) (i32.const 0)))
  (local.set $e (call $zhex_host_fs_stat (i32.const 0) (local.get $len)))
  (if (i32.eqz (local.get $e))
    (then
      (if (i32.eq (i32.load (i32.const 4)) (i32.const 1))
        (then (return (struct.new $zhex_result (i32.const 0) (struct.new $zhex_some_i64 (i64.const 0))))))
      (return (struct.new $zhex_result (i32.const 0) (struct.new $zhex_some_i64 (i64.load (i32.const 8)))))))
  (struct.new $zhex_result (i32.const 1)
    (call $zhex_errv (i32.const 3)
      (call $zhex_cat4 (call $m3_cannot_size) (call $zhex_quote (local.get $path)) (call $m3_colon_sp)
        (call $zhex_os_err (call $m3_v_stat) (local.get $path) (local.get $e))))))

(func $zhex_file_make_folder (param $path (ref $zhex_string)) (param $parents i32) (result (ref $zhex_result))
  (local $len i32) (local $e i32)
  (local.set $len (call $zhex_stage_utf8_at (local.get $path) (i32.const 0)))
  (local.set $e (call $zhex_host_fs_make_folder (i32.const 0) (local.get $len) (local.get $parents)))
  (if (i32.eqz (local.get $e))
    (then (return (struct.new $zhex_result (i32.const 0) (ref.null eq)))))
  (struct.new $zhex_result (i32.const 1)
    (call $zhex_errv (i32.const 3)
      (call $zhex_cat4 (call $m3_cannot_folder) (call $zhex_quote (local.get $path)) (call $m3_colon_sp)
        (call $zhex_os_err (call $m3_v_mkdir) (local.get $path) (local.get $e))))))

(func $zhex_file_delete (param $path (ref $zhex_string)) (result (ref $zhex_result))
  (local $len i32) (local $e i32)
  (local.set $len (call $zhex_stage_utf8_at (local.get $path) (i32.const 0)))
  (local.set $e (call $zhex_host_fs_delete (i32.const 0) (local.get $len)))
  (if (i32.eqz (local.get $e))
    (then (return (struct.new $zhex_result (i32.const 0) (ref.null eq)))))
  (struct.new $zhex_result (i32.const 1)
    (call $zhex_errv (i32.const 3)
      (call $zhex_cat4 (call $m3_cannot_del) (call $zhex_quote (local.get $path)) (call $m3_colon_sp)
        (call $zhex_os_err (call $m3_v_remove) (local.get $path) (local.get $e))))))

(func $zhex_file_delete_folder (param $path (ref $zhex_string)) (param $recursive i32) (result (ref $zhex_result))
  (local $len i32) (local $e i32)
  (local.set $len (call $zhex_stage_utf8_at (local.get $path) (i32.const 0)))
  (local.set $e (call $zhex_host_fs_delete_folder (i32.const 0) (local.get $len) (local.get $recursive)))
  (if (i32.eqz (local.get $e))
    (then (return (struct.new $zhex_result (i32.const 0) (ref.null eq)))))
  (struct.new $zhex_result (i32.const 1)
    (call $zhex_errv (i32.const 3)
      (call $zhex_cat4 (call $m3_cannot_rmdir) (call $zhex_quote (local.get $path)) (call $m3_colon_sp)
        (call $zhex_os_err (call $m3_v_remove) (local.get $path) (local.get $e))))))

(func $zhex_file_move (param $src (ref $zhex_string)) (param $dst (ref $zhex_string)) (param $overwrite i32) (result (ref $zhex_result))
  (local $ls i32) (local $ld i32) (local $e i32) (local $e2 i32)
  ;; the interpreter's check-then-act shape: stat the target first
  (local.set $ld (call $zhex_stage_utf8_at (local.get $dst) (i32.const 65536)))
  (local.set $e (call $zhex_host_fs_stat (i32.const 65536) (local.get $ld)))
  (if (i32.eqz (local.get $e))
    (then
      (if (i32.eq (i32.load (i32.const 4)) (i32.const 1))
        (then
          (return (struct.new $zhex_result (i32.const 1)
            (call $zhex_errv (i32.const 3)
              (call $zhex_cat5 (call $m3_cannot_move) (call $zhex_quote (local.get $src)) (call $m3_to)
                (call $zhex_quote (local.get $dst)) (call $m3_target_folder)))))))
      (if (i32.eqz (local.get $overwrite))
        (then
          (return (struct.new $zhex_result (i32.const 1)
            (call $zhex_errv (i32.const 3)
              (call $zhex_cat5 (call $m3_cannot_move) (call $zhex_quote (local.get $src)) (call $m3_to)
                (call $zhex_quote (local.get $dst)) (call $m3_target_exists)))))))))
  (if (i32.and (i32.ne (local.get $e) (i32.const 0)) (i32.ne (local.get $e) (i32.const 44)))
    (then
      (return (struct.new $zhex_result (i32.const 1)
        (call $zhex_errv (i32.const 3)
          (call $zhex_cat5 (call $m3_cannot_move) (call $zhex_quote (local.get $src)) (call $m3_to)
            (call $zhex_quote (local.get $dst))
            (call $zhex_str_concat (call $m3_colon_sp)
              (call $zhex_os_err (call $m3_v_stat) (local.get $dst) (local.get $e)))))))))
  (local.set $ls (call $zhex_stage_utf8_at (local.get $src) (i32.const 0)))
  (local.set $ld (call $zhex_stage_utf8_at (local.get $dst) (i32.const 65536)))
  (local.set $e2 (call $zhex_host_fs_move (i32.const 0) (local.get $ls) (i32.const 65536) (local.get $ld) (local.get $overwrite)))
  (if (i32.eqz (local.get $e2))
    (then (return (struct.new $zhex_result (i32.const 0) (ref.null eq)))))
  (struct.new $zhex_result (i32.const 1)
    (call $zhex_errv (i32.const 3)
      (call $zhex_cat5 (call $m3_cannot_move) (call $zhex_quote (local.get $src)) (call $m3_to)
        (call $zhex_quote (local.get $dst))
        (call $zhex_str_concat (call $m3_colon_sp)
          (call $zhex_str_concat
            (call $zhex_str_concat
              (call $zhex_str_concat (call $zhex_str_concat (call $m3_v_rename) (call $zhex_space)) (local.get $src))
              (call $zhex_str_concat (call $zhex_space) (local.get $dst)))
            (call $zhex_str_concat (call $m3_colon_sp) (call $zhex_errno_text (local.get $e2)))))))))

;; ---- file.stream (§3): open/read/UTF-8 failures are hard code-17 errors --

(func $m3_stream_path (param $handle i32) (result (ref $zhex_string))
  (if (ref.is_null (global.get $zhex_stream_paths))
    (then (return (array.new_fixed $zhex_string 0))))
  (if (ref.test (ref $zhex_none)
        (call $zhex_map_lookup_i64_ref (ref.as_non_null (global.get $zhex_stream_paths)) (i64.extend_i32_s (local.get $handle))))
    (then (return (array.new_fixed $zhex_string 0))))
  (ref.cast (ref $zhex_string)
    (struct.get $zhex_some_ref $value
      (ref.cast (ref $zhex_some_ref)
        (call $zhex_map_lookup_i64_ref (ref.as_non_null (global.get $zhex_stream_paths)) (i64.extend_i32_s (local.get $handle)))))))

(func $zhex_stream_open (param $path (ref $zhex_string)) (result i32)
  (local $len i32) (local $h i32)
  (local.set $len (call $zhex_stage_utf8_at (local.get $path) (i32.const 0)))
  (local.set $h (call $zhex_host_fs_open_read (i32.const 0) (local.get $len)))
  (if (i32.lt_s (local.get $h) (i32.const 0))
    (then
      (call $zhex_fail_msg (i32.const 17)
        (call $zhex_cat4 (call $m3_cannot_open) (call $zhex_quote (local.get $path)) (call $m3_colon_sp)
          (call $zhex_os_err (call $m3_v_open) (local.get $path) (i32.sub (i32.const 0) (local.get $h)))))))
  (if (ref.is_null (global.get $zhex_stream_paths))
    (then (global.set $zhex_stream_paths (call $zhex_map_new_i64_ref))))
  (drop (call $zhex_map_put_i64_ref (ref.as_non_null (global.get $zhex_stream_paths))
    (i64.extend_i32_s (local.get $h)) (local.get $path)))
  (local.get $h))

(func $zhex_stream_line (param $handle i32) (result (ref $zhex_result))
  (local $path (ref $zhex_string)) (local $r i32) (local $n i32)
  (local.set $path (call $m3_stream_path (local.get $handle)))
  (local.set $r (call $zhex_host_fs_read_line (local.get $handle)))
  (if (i32.eqz (local.get $r))
    (then (return (struct.new $zhex_result (i32.const 0) (ref.null eq)))))  ;; clean EOF
  (if (i32.lt_s (local.get $r) (i32.const 0))
    (then
      (call $zhex_fail_msg (i32.const 17)
        (call $zhex_cat4 (call $m3_reading) (call $zhex_quote (local.get $path)) (call $m3_failed)
          (call $zhex_os_err (call $m3_v_read) (local.get $path) (i32.sub (i32.const 0) (local.get $r)))))))
  (local.set $n (call $zhex_scratch_len))
  (if (i32.and (i32.gt_u (local.get $n) (i32.const 0))
               (i32.eq (i32.load8_u (i32.add (i32.const 3) (local.get $n))) (i32.const 10)))
    (then
      (local.set $n (i32.sub (local.get $n) (i32.const 1)))
      (if (i32.and (i32.gt_u (local.get $n) (i32.const 0))
                   (i32.eq (i32.load8_u (i32.add (i32.const 3) (local.get $n))) (i32.const 13)))
        (then (local.set $n (i32.sub (local.get $n) (i32.const 1)))))))
  (if (i32.eqz (call $zhex_bytes_utf8_ok (i32.const 4) (local.get $n)))
    (then
      (call $zhex_fail_msg (i32.const 17)
        (call $zhex_cat3 (call $m3_file_word) (call $zhex_quote (local.get $path)) (call $m3_is_utf8_b)))))
  (struct.new $zhex_result (i32.const 0) (call $zhex_bytes_to_string (i32.const 4) (local.get $n))))

(func $zhex_stream_close (param $handle i32)
  (if (ref.is_null (global.get $zhex_stream_paths))
    (then (return)))
  (drop (call $zhex_map_remove_i64_ref (ref.as_non_null (global.get $zhex_stream_paths)) (i64.extend_i32_s (local.get $handle))))
  (drop (call $zhex_host_fs_close (local.get $handle))))
`

// watM3Duration: Duration is unboxed i64 nanoseconds; duration.parse
// reimplements Go 1.27's time.ParseDuration (unsigned arithmetic, read from
// the toolchain source, including the leadingInt/leadingFraction overflow
// rules and the float64 fraction step) with every failure collapsed to
// "invalid duration" in the common Error family; duration.to-string
// reimplements time.Duration.format (fmtFrac trailing-zero trimming,
// µ = U+00B5).
const watM3Duration = `
(func $zhex_duration_parse (param $s (ref $zhex_string)) (result (ref $zhex_result))
  ;; Go 1.27 time.ParseDuration, unsigned arithmetic read from the
  ;; toolchain source; every failure collapses to "invalid duration"
  ;; (family 5). Guards nest because WAT and/or do not short-circuit.
  (local $n i32)
  (local $i i32)
  (local $neg i32)
  (local $c i32)
  (local $pre i32)
  (local $post i32)
  (local $overflow i32)
  (local $ulen i32)
  (local $u0 i32)
  (local $u1 i32)
  (local $d i64)
  (local $v i64)
  (local $f i64)
  (local $unit i64)
  (local $scale f64)
  (local.set $n (array.len (local.get $s)))
  (if (i32.eqz (local.get $n))
    (then
      (return (struct.new $zhex_result (i32.const 1) (call $zhex_errv (i32.const 5) (call $m3_bad_duration))))
    )
  )
  (local.set $c (array.get $zhex_string (local.get $s) (i32.const 0)))
  (if (i32.or (i32.eq (local.get $c) (i32.const 45)) (i32.eq (local.get $c) (i32.const 43)))
    (then
      (local.set $neg (i32.eq (local.get $c) (i32.const 45)))
      (local.set $i (i32.const 1))
    )
  )
  ;; the "0" special case after the sign strip
  (if (i32.eq (i32.sub (local.get $n) (local.get $i)) (i32.const 1))
    (then
      (if (i32.eq (array.get $zhex_string (local.get $s) (local.get $i)) (i32.const 48))
        (then
          (return (struct.new $zhex_result (i32.const 0) (struct.new $zhex_some_i64 (i64.const 0))))
        )
      )
    )
  )
  (if (i32.ge_u (local.get $i) (local.get $n))
    (then
      (return (struct.new $zhex_result (i32.const 1) (call $zhex_errv (i32.const 5) (call $m3_bad_duration))))
    )
  )
  (block $segments
    (loop $segment
      (br_if $segments (i32.ge_u (local.get $i) (local.get $n)))
      (local.set $c (array.get $zhex_string (local.get $s) (local.get $i)))
      (if (i32.eqz (i32.or (i32.eq (local.get $c) (i32.const 46)) (i32.and (i32.ge_u (local.get $c) (i32.const 48)) (i32.le_u (local.get $c) (i32.const 57)))))
        (then
          (return (struct.new $zhex_result (i32.const 1) (call $zhex_errv (i32.const 5) (call $m3_bad_duration))))
        )
      )
      ;; leadingInt: uint64 accumulate; overflow when x > (1<<63-1)/10 or
      ;; x*10+digit > 1<<63 (Go's errLeadingInt)
      (local.set $v (i64.const 0))
      (local.set $pre (i32.const 0))
      (block $li
        (loop $lid
          (br_if $li (i32.ge_u (local.get $i) (local.get $n)))
          (local.set $c (array.get $zhex_string (local.get $s) (local.get $i)))
          (if (i32.or (i32.lt_u (local.get $c) (i32.const 48)) (i32.gt_u (local.get $c) (i32.const 57)))
            (then
              (br $li)
            )
          )
          (if (i64.gt_u (local.get $v) (i64.const 922337203685477580))
            (then
              (return (struct.new $zhex_result (i32.const 1) (call $zhex_errv (i32.const 5) (call $m3_bad_duration))))
            )
          )
          (local.set $v (i64.add (i64.mul (local.get $v) (i64.const 10)) (i64.extend_i32_u (i32.sub (local.get $c) (i32.const 48)))))
          (if (i64.gt_u (local.get $v) (i64.const 9223372036854775808))
            (then
              (return (struct.new $zhex_result (i32.const 1) (call $zhex_errv (i32.const 5) (call $m3_bad_duration))))
            )
          )
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (local.set $pre (i32.const 1))
          (br $lid)
        )
      )
      ;; optional fraction: on overflow keep consuming digits, x/scale frozen
      (local.set $post (i32.const 0))
      (if (i32.lt_u (local.get $i) (local.get $n))
        (then
          (if (i32.eq (array.get $zhex_string (local.get $s) (local.get $i)) (i32.const 46))
            (then
              (local.set $i (i32.add (local.get $i) (i32.const 1)))
              (local.set $f (i64.const 0))
              (local.set $scale (f64.const 1))
              (local.set $overflow (i32.const 0))
              (block $lf
                (loop $lfd
                  (br_if $lf (i32.ge_u (local.get $i) (local.get $n)))
                  (local.set $c (array.get $zhex_string (local.get $s) (local.get $i)))
                  (if (i32.or (i32.lt_u (local.get $c) (i32.const 48)) (i32.gt_u (local.get $c) (i32.const 57)))
                    (then
                      (br $lf)
                    )
                  )
                  (local.set $post (i32.const 1))
                  (if (i32.eqz (local.get $overflow))
                    (then
                      (if (i64.gt_u (local.get $f) (i64.const 922337203685477580))
                        (then
                          (local.set $overflow (i32.const 1))
                        )
                        (else
                          (local.set $f (i64.add (i64.mul (local.get $f) (i64.const 10)) (i64.extend_i32_u (i32.sub (local.get $c) (i32.const 48)))))
                          (if (i64.gt_u (local.get $f) (i64.const 9223372036854775808))
                            (then
                              (local.set $overflow (i32.const 1))
                            )
                            (else
                              (local.set $scale (f64.mul (local.get $scale) (f64.const 10)))
                            )
                          )
                        )
                      )
                    )
                  )
                  (local.set $i (i32.add (local.get $i) (i32.const 1)))
                  (br $lfd)
                )
              )
            )
          )
        )
      )
      (if (i32.eqz (i32.or (local.get $pre) (local.get $post)))
        (then
          (return (struct.new $zhex_result (i32.const 1) (call $zhex_errv (i32.const 5) (call $m3_bad_duration))))
        )
      )
      ;; the unit run: everything up to the next digit or dot
      (local.set $ulen (i32.const 0))
      (local.set $u0 (i32.const 0))
      (local.set $u1 (i32.const 0))
      (block $ur
        (loop $urd
          (br_if $ur (i32.ge_u (local.get $i) (local.get $n)))
          (local.set $c (array.get $zhex_string (local.get $s) (local.get $i)))
          (if (i32.or (i32.eq (local.get $c) (i32.const 46)) (i32.and (i32.ge_u (local.get $c) (i32.const 48)) (i32.le_u (local.get $c) (i32.const 57))))
            (then
              (br $ur)
            )
          )
          (if (i32.eqz (local.get $ulen))
            (then
              (local.set $u0 (local.get $c))
            )
          )
          (if (i32.eq (local.get $ulen) (i32.const 1))
            (then
              (local.set $u1 (local.get $c))
            )
          )
          (local.set $ulen (i32.add (local.get $ulen) (i32.const 1)))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $urd)
        )
      )
      (if (i32.eqz (local.get $ulen))
        (then
          (return (struct.new $zhex_result (i32.const 1) (call $zhex_errv (i32.const 5) (call $m3_bad_duration))))
        )
      )
      ;; unit table: ns=1, us/u00b5s/u03bcs=1e3, ms=1e6, s=1e9, m=60e9, h=3600e9
      (local.set $unit (i64.const -1))
      (if (i32.eq (local.get $ulen) (i32.const 1))
        (then
          (if (i32.eq (local.get $u0) (i32.const 115))
            (then
              (local.set $unit (i64.const 1000000000))
            )
          )
          (if (i32.eq (local.get $u0) (i32.const 109))
            (then
              (local.set $unit (i64.const 60000000000))
            )
          )
          (if (i32.eq (local.get $u0) (i32.const 104))
            (then
              (local.set $unit (i64.const 3600000000000))
            )
          )
        )
      )
      (if (i32.eq (local.get $ulen) (i32.const 2))
        (then
          (if (i32.and (i32.eq (local.get $u0) (i32.const 110)) (i32.eq (local.get $u1) (i32.const 115)))
            (then
              (local.set $unit (i64.const 1))
            )
          )
          (if (i32.and (i32.eq (local.get $u0) (i32.const 109)) (i32.eq (local.get $u1) (i32.const 115)))
            (then
              (local.set $unit (i64.const 1000000))
            )
          )
          (if (i32.and (i32.eq (local.get $u0) (i32.const 117)) (i32.eq (local.get $u1) (i32.const 115)))
            (then
              (local.set $unit (i64.const 1000))
            )
          )
          (if (i32.and (i32.eq (local.get $u0) (i32.const 181)) (i32.eq (local.get $u1) (i32.const 115)))
            (then
              (local.set $unit (i64.const 1000))
            )
          )
          (if (i32.and (i32.eq (local.get $u0) (i32.const 956)) (i32.eq (local.get $u1) (i32.const 115)))
            (then
              (local.set $unit (i64.const 1000))
            )
          )
        )
      )
      (if (i64.lt_s (local.get $unit) (i64.const 0))
        (then
          (return (struct.new $zhex_result (i32.const 1) (call $zhex_errv (i32.const 5) (call $m3_bad_duration))))
        )
      )
      (if (i64.gt_u (local.get $v) (i64.div_u (i64.const 9223372036854775808) (local.get $unit)))
        (then
          (return (struct.new $zhex_result (i32.const 1) (call $zhex_errv (i32.const 5) (call $m3_bad_duration))))
        )
      )
      (local.set $v (i64.mul (local.get $v) (local.get $unit)))
      (if (i64.gt_u (local.get $f) (i64.const 0))
        (then
          (local.set $v (i64.add (local.get $v) (i64.trunc_f64_u (f64.mul (f64.convert_i64_u (local.get $f)) (f64.div (f64.convert_i64_u (local.get $unit)) (local.get $scale))))))
          (if (i64.gt_u (local.get $v) (i64.const 9223372036854775808))
            (then
              (return (struct.new $zhex_result (i32.const 1) (call $zhex_errv (i32.const 5) (call $m3_bad_duration))))
            )
          )
          (local.set $f (i64.const 0))
        )
      )
      (local.set $d (i64.add (local.get $d) (local.get $v)))
      (if (i64.gt_u (local.get $d) (i64.const 9223372036854775808))
        (then
          (return (struct.new $zhex_result (i32.const 1) (call $zhex_errv (i32.const 5) (call $m3_bad_duration))))
        )
      )
      (br $segment)
    )
  )
  (if (i32.ne (local.get $neg) (i32.const 0))
    (then
      (return (struct.new $zhex_result (i32.const 0) (struct.new $zhex_some_i64 (i64.sub (i64.const 0) (local.get $d)))))
    )
  )
  (if (i64.gt_u (local.get $d) (i64.const 9223372036854775807))
    (then
      (return (struct.new $zhex_result (i32.const 1) (call $zhex_errv (i32.const 5) (call $m3_bad_duration))))
    )
  )
  (struct.new $zhex_result (i32.const 0) (struct.new $zhex_some_i64 (local.get $d)))
)

;; fmtInt into buf[$zhex_dw] (unsigned); fmtFrac writes prec digits with
;; trailing zeros trimmed plus '.', dividing u by 10^prec; both communicate
;; through $zhex_dw/$zhex_dv
(func $m3_dur_fmt_int (param $buf (ref $zhex_arr_i32)) (param $v i64)
  (if (i64.eqz (local.get $v))
    (then
      (global.set $zhex_dw (i32.sub (global.get $zhex_dw) (i32.const 1)))
      (array.set $zhex_arr_i32 (local.get $buf) (global.get $zhex_dw) (i32.const 48))
      (return)
    )
  )
  (block $done
    (loop $digit
      (br_if $done (i64.eqz (local.get $v)))
      (global.set $zhex_dw (i32.sub (global.get $zhex_dw) (i32.const 1)))
      (array.set $zhex_arr_i32 (local.get $buf) (global.get $zhex_dw)
        (i32.add (i32.const 48) (i32.wrap_i64 (i64.rem_u (local.get $v) (i64.const 10)))))
      (local.set $v (i64.div_u (local.get $v) (i64.const 10)))
      (br $digit)
    )
  )
)

(func $m3_dur_fmt_frac (param $buf (ref $zhex_arr_i32)) (param $v i64) (param $prec i32)
  (local $print i32)
  (local $digit i32)
  (local $i i32)
  (block $done
    (loop $d
      (br_if $done (i32.ge_u (local.get $i) (local.get $prec)))
      (local.set $digit (i32.wrap_i64 (i64.rem_u (local.get $v) (i64.const 10))))
      (if (i32.ne (local.get $digit) (i32.const 0))
        (then
          (local.set $print (i32.const 1))
        )
      )
      (if (local.get $print)
        (then
          (global.set $zhex_dw (i32.sub (global.get $zhex_dw) (i32.const 1)))
          (array.set $zhex_arr_i32 (local.get $buf) (global.get $zhex_dw)
            (i32.add (i32.const 48) (local.get $digit)))
        )
      )
      (local.set $v (i64.div_u (local.get $v) (i64.const 10)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $d)
    )
  )
  (if (local.get $print)
    (then
      (global.set $zhex_dw (i32.sub (global.get $zhex_dw) (i32.const 1)))
      (array.set $zhex_arr_i32 (local.get $buf) (global.get $zhex_dw) (i32.const 46))
    )
  )
  (global.set $zhex_dv (local.get $v))
)

(func $zhex_duration_to_string (param $d i64) (result (ref $zhex_string))
  (local $neg i32) (local $u i64) (local $buf (ref $zhex_arr_i32))
  (local $prec i32) (local $i i32) (local $res (ref null $zhex_string)) (local $len i32)
  (local.set $neg (i64.lt_s (local.get $d) (i64.const 0)))
  (local.set $u (select (i64.sub (i64.const 0) (local.get $d)) (local.get $d) (local.get $neg)))
  (local.set $buf (array.new_default $zhex_arr_i32 (i32.const 32)))
  (global.set $zhex_dw (i32.const 32))
  (if (i64.lt_u (local.get $u) (i64.const 1000000000))
    (then
      (global.set $zhex_dw (i32.sub (global.get $zhex_dw) (i32.const 1)))
      (array.set $zhex_arr_i32 (local.get $buf) (global.get $zhex_dw) (i32.const 115))
      (if (i64.eqz (local.get $u))
        (then
          (global.set $zhex_dw (i32.sub (global.get $zhex_dw) (i32.const 1)))
          (array.set $zhex_arr_i32 (local.get $buf) (global.get $zhex_dw) (i32.const 48)))
        (else
          (if (i64.lt_u (local.get $u) (i64.const 1000))
            (then
              (global.set $zhex_dw (i32.sub (global.get $zhex_dw) (i32.const 1)))
              (array.set $zhex_arr_i32 (local.get $buf) (global.get $zhex_dw) (i32.const 110)))
            (else
              (if (i64.lt_u (local.get $u) (i64.const 1000000))
                (then
                  ;; code points: one slot for U+00B5, 's' already at w+1
                  (global.set $zhex_dw (i32.sub (global.get $zhex_dw) (i32.const 1)))
                  (array.set $zhex_arr_i32 (local.get $buf) (global.get $zhex_dw) (i32.const 181))
                  (local.set $prec (i32.const 3)))
                (else
                  (global.set $zhex_dw (i32.sub (global.get $zhex_dw) (i32.const 1)))
                  (array.set $zhex_arr_i32 (local.get $buf) (global.get $zhex_dw) (i32.const 109))
                  (local.set $prec (i32.const 6))))))
          (call $m3_dur_fmt_frac (local.get $buf) (local.get $u) (local.get $prec))
          (call $m3_dur_fmt_int (local.get $buf) (global.get $zhex_dv)))))
    (else
      (global.set $zhex_dw (i32.sub (global.get $zhex_dw) (i32.const 1)))
      (array.set $zhex_arr_i32 (local.get $buf) (global.get $zhex_dw) (i32.const 115))
      (call $m3_dur_fmt_frac (local.get $buf) (local.get $u) (i32.const 9))
      (local.set $u (global.get $zhex_dv))
      (call $m3_dur_fmt_int (local.get $buf) (i64.rem_u (local.get $u) (i64.const 60)))
      (local.set $u (i64.div_u (local.get $u) (i64.const 60)))
      (if (i64.gt_u (local.get $u) (i64.const 0))
        (then
          (global.set $zhex_dw (i32.sub (global.get $zhex_dw) (i32.const 1)))
          (array.set $zhex_arr_i32 (local.get $buf) (global.get $zhex_dw) (i32.const 109))
          (call $m3_dur_fmt_int (local.get $buf) (i64.rem_u (local.get $u) (i64.const 60)))
          (local.set $u (i64.div_u (local.get $u) (i64.const 60)))
          (if (i64.gt_u (local.get $u) (i64.const 0))
            (then
              (global.set $zhex_dw (i32.sub (global.get $zhex_dw) (i32.const 1)))
              (array.set $zhex_arr_i32 (local.get $buf) (global.get $zhex_dw) (i32.const 104))
              (call $m3_dur_fmt_int (local.get $buf) (local.get $u))))))))
  (if (local.get $neg)
    (then
      (global.set $zhex_dw (i32.sub (global.get $zhex_dw) (i32.const 1)))
      (array.set $zhex_arr_i32 (local.get $buf) (global.get $zhex_dw) (i32.const 45))))
  (local.set $len (i32.sub (i32.const 32) (global.get $zhex_dw)))
  (local.set $res (array.new_default $zhex_string (local.get $len)))
  (block $copied
    (loop $copy
      (br_if $copied (i32.ge_u (local.get $i) (local.get $len)))
      (array.set $zhex_string (local.get $res) (local.get $i)
        (array.get $zhex_arr_i32 (local.get $buf) (i32.add (global.get $zhex_dw) (local.get $i))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $copy)))
  (ref.as_non_null (local.get $res)))
`

// watM3Env: module-side env state (§7, Surprises 6) - a $zhex_map_str_ref
// seeded once from env_environ, hosts never mutated. env.get is a plain
// Option lookup; env.load reads a .env file and applies first-wins puts
// with the interpreter's line grammar; byte-level trimming covers the ASCII
// whitespace subset (shortcut: non-ASCII spaces like NBSP around keys
// diverge from Go's rune-domain TrimSpace; fixtures are ASCII).
const watM3Env = `
(func $zhex_env_init
  (local $m (ref $zhex_map_str_ref))
  (local $e i32) (local $count i32) (local $cursor i32) (local $n i32) (local $klen i32) (local $vlen i32)
  (if (i32.ne (global.get $zhex_env_done) (i32.const 0)) (then (return)))
  (global.set $zhex_env_done (i32.const 1))
  (local.set $m (call $zhex_map_new_str_ref))
  (local.set $e (call $zhex_host_env_environ))
  (if (i32.eqz (local.get $e))
    (then
      (local.set $n (call $zhex_scratch_len))
      (local.set $count (i32.load (i32.const 4)))
      (local.set $cursor (i32.const 8))
      (block $seeded
        (loop $pair
          (br_if $seeded (i32.ge_u (local.get $cursor) (i32.add (i32.const 4) (local.get $n))))
          (local.set $klen (i32.load (local.get $cursor)))
          (local.set $vlen (i32.load (i32.add (local.get $cursor) (i32.add (i32.const 4) (local.get $klen)))))
          (drop (call $zhex_map_put_str_ref (local.get $m)
            (call $zhex_bytes_to_string (i32.add (local.get $cursor) (i32.const 4)) (local.get $klen))
            (call $zhex_bytes_to_string
              (i32.add (local.get $cursor) (i32.add (i32.const 8) (local.get $klen))) (local.get $vlen))))
          (local.set $cursor (i32.add (local.get $cursor) (i32.add (i32.add (i32.const 8) (local.get $klen)) (local.get $vlen))))
          (br $pair)))))
  (global.set $zhex_env (local.get $m)))

(func $zhex_env_get (param $name (ref $zhex_string)) (result (ref eq))
  (if (ref.is_null (global.get $zhex_env))
    (then (return (struct.new $zhex_none))))
  (call $zhex_map_lookup_str_ref (ref.as_non_null (global.get $zhex_env)) (local.get $name)))

(func $m3_is_space_byte (param $b i32) (result i32)
  (if (i32.lt_u (local.get $b) (i32.const 32))
    (then (return (i32.and (i32.ge_u (local.get $b) (i32.const 9)) (i32.le_u (local.get $b) (i32.const 13))))))
  (i32.eq (local.get $b) (i32.const 32)))

(func $zhex_env_load (param $path (ref $zhex_string)) (result (ref $zhex_result))
  ;; reads KEY=VALUE lines into the module-side map; blank and whole-line
  ;; '#' comments are skipped, one surrounding quote pair is stripped, and
  ;; the first assignment wins (startup environ and earlier lines alike);
  ;; lines before a malformed one stay applied. Byte-level trimming covers
  ;; the ASCII whitespace subset (shortcut: non-ASCII spaces around keys
  ;; diverge from Go's rune-domain TrimSpace; fixtures are ASCII). A guard
  ;; block (br_if $process) keeps the per-line nesting flat.
  (local $len i32)
  (local $e i32)
  (local $n i32)
  (local $start i32)
  (local $eol i32)
  (local $a i32)
  (local $b i32)
  (local $eq i32)
  (local $ka i32)
  (local $kb i32)
  (local $va i32)
  (local $vb i32)
  (local $bad i32)
  (local $i i32)
  (local $lineno i64)
  (local $m (ref $zhex_map_str_ref))
  ;; seed the env map before reading the file: env_environ lands in the
  ;; scratch slot at offset 0 and would clobber the file bytes otherwise
  (if (ref.is_null (global.get $zhex_env))
    (then
      (call $zhex_env_init)
    )
  )
  (local.set $len (call $zhex_stage_utf8_at (local.get $path) (i32.const 0)))
  (local.set $e (call $zhex_host_fs_read_file (i32.const 0) (local.get $len)))
  (if (i32.eq (local.get $e) (i32.const 31))
    (then
      (return
        (struct.new $zhex_result (i32.const 1)
          (call $zhex_errv (i32.const 5)
            (call $zhex_cat4 (call $m3_cannot_read) (call $zhex_quote (local.get $path)) (call $m3_colon_sp)
              (call $zhex_os_err (call $m3_v_read) (local.get $path) (local.get $e))))))
    )
  )
  (if (i32.ne (local.get $e) (i32.const 0))
    (then
      (return
        (struct.new $zhex_result (i32.const 1)
          (call $zhex_errv (i32.const 5)
            (call $zhex_cat4 (call $m3_cannot_read) (call $zhex_quote (local.get $path)) (call $m3_colon_sp)
              (call $zhex_os_err (call $m3_v_open) (local.get $path) (local.get $e))))))
    )
  )
  (local.set $m (ref.as_non_null (global.get $zhex_env)))
  (local.set $n (call $zhex_scratch_len))
  (block $done
    (loop $line
      (local.set $eol (local.get $start))
      (block $found
        (loop $scan
          (br_if $found (i32.ge_u (local.get $eol) (local.get $n)))
          (br_if $found (i32.eq (i32.load8_u (i32.add (i32.const 4) (local.get $eol))) (i32.const 10)))
          (local.set $eol (i32.add (local.get $eol) (i32.const 1)))
          (br $scan)
        )
      )
      (local.set $lineno (i64.add (local.get $lineno) (i64.const 1)))
      ;; trim both ends (ASCII whitespace)
      (local.set $a (local.get $start))
      (local.set $b (local.get $eol))
      (block $lt
        (loop $l1
          (br_if $lt (i32.ge_u (local.get $a) (local.get $b)))
          (br_if $lt (i32.eqz (call $m3_is_space_byte (i32.load8_u (i32.add (i32.const 4) (local.get $a))))))
          (local.set $a (i32.add (local.get $a) (i32.const 1)))
          (br $l1)
        )
      )
      (block $rt
        (loop $l2
          (br_if $rt (i32.le_u (local.get $b) (local.get $a)))
          (br_if $rt (i32.eqz (call $m3_is_space_byte (i32.load8_u (i32.add (i32.const 3) (local.get $b))))))
          (local.set $b (i32.sub (local.get $b) (i32.const 1)))
          (br $l2)
        )
      )
      ;; guard: skip empty lines and whole-line comments
      (block $process
        (br_if $process (i32.ge_u (local.get $a) (local.get $b)))
        (br_if $process (i32.eq (i32.load8_u (i32.add (i32.const 4) (local.get $a))) (i32.const 35)))
        ;; cut at the first '='
        (local.set $eq (local.get $a))
        (local.set $bad (i32.const 0))
        (block $cut
          (loop $cs
            (br_if $cut (i32.ge_u (local.get $eq) (local.get $b)))
            (br_if $cut (i32.eq (i32.load8_u (i32.add (i32.const 4) (local.get $eq))) (i32.const 61)))
            (local.set $eq (i32.add (local.get $eq) (i32.const 1)))
            (br $cs)
          )
        )
        (if (i32.ge_u (local.get $eq) (local.get $b))
          (then
            (local.set $bad (i32.const 1))
          )
        )
        ;; key: [a, eq) with its right edge trimmed
        (local.set $ka (local.get $a))
        (local.set $kb (local.get $eq))
        (block $kr
          (loop $k1
            (br_if $kr (i32.le_u (local.get $kb) (local.get $ka)))
            (br_if $kr (i32.eqz (call $m3_is_space_byte (i32.load8_u (i32.add (i32.const 3) (local.get $kb))))))
            (local.set $kb (i32.sub (local.get $kb) (i32.const 1)))
            (br $k1)
          )
        )
        (if (i32.eq (local.get $kb) (local.get $ka))
          (then
            (local.set $bad (i32.const 1))
          )
        )
        ;; reject keys containing a space or tab
        (if (i32.eqz (local.get $bad))
          (then
            (local.set $i (local.get $ka))
            (block $ks
              (loop $k2
                (br_if $ks (i32.ge_u (local.get $i) (local.get $kb)))
                (if (i32.or (i32.eq (i32.load8_u (i32.add (i32.const 4) (local.get $i))) (i32.const 32)) (i32.eq (i32.load8_u (i32.add (i32.const 4) (local.get $i))) (i32.const 9)))
                  (then
                    (local.set $bad (i32.const 1))
                    (br $ks)
                  )
                )
                (local.set $i (i32.add (local.get $i) (i32.const 1)))
                (br $k2)
              )
            )
          )
        )
        ;; value: (eq, b) trimmed both edges, then a quote pair stripped
        (if (i32.eqz (local.get $bad))
          (then
            (local.set $va (i32.add (local.get $eq) (i32.const 1)))
            (local.set $vb (local.get $b))
            (block $vl
              (loop $v1
                (br_if $vl (i32.ge_u (local.get $va) (local.get $vb)))
                (br_if $vl (i32.eqz (call $m3_is_space_byte (i32.load8_u (i32.add (i32.const 4) (local.get $va))))))
                (local.set $va (i32.add (local.get $va) (i32.const 1)))
                (br $v1)
              )
            )
            (block $vr
              (loop $v2
                (br_if $vr (i32.le_u (local.get $vb) (local.get $va)))
                (br_if $vr (i32.eqz (call $m3_is_space_byte (i32.load8_u (i32.add (i32.const 3) (local.get $vb))))))
                (local.set $vb (i32.sub (local.get $vb) (i32.const 1)))
                (br $v2)
              )
            )
            (if (i32.ge_u (i32.sub (local.get $vb) (local.get $va)) (i32.const 2))
              (then
                (if (i32.or (i32.eq (i32.load8_u (i32.add (i32.const 4) (local.get $va))) (i32.const 34)) (i32.eq (i32.load8_u (i32.add (i32.const 4) (local.get $va))) (i32.const 39)))
                  (then
                    (if (i32.eq (i32.load8_u (i32.add (i32.const 4) (local.get $va))) (i32.load8_u (i32.add (i32.const 3) (local.get $vb))))
                      (then
                        (local.set $va (i32.add (local.get $va) (i32.const 1)))
                        (local.set $vb (i32.sub (local.get $vb) (i32.const 1)))
                      )
                    )
                  )
                )
              )
            )
            ;; first assignment wins: put only when absent
            (if (i32.eqz (call $zhex_map_contains_str_ref (local.get $m) (call $zhex_bytes_to_string (i32.add (i32.const 4) (local.get $ka)) (i32.sub (local.get $kb) (local.get $ka)))))
              (then
                (drop (call $zhex_map_put_str_ref (local.get $m) (call $zhex_bytes_to_string (i32.add (i32.const 4) (local.get $ka)) (i32.sub (local.get $kb) (local.get $ka))) (call $zhex_bytes_to_string (i32.add (i32.const 4) (local.get $va)) (i32.sub (local.get $vb) (local.get $va))))))
              )
            )
          )
        ;; malformed line: report and stop (earlier lines stay applied)
        (if (i32.ne (local.get $bad) (i32.const 0))
          (then
            (return
              (struct.new $zhex_result (i32.const 1)
                (call $zhex_errv (i32.const 5)
                  (call $zhex_str_concat (call $zhex_cat5 (call $m3_line_word) (call $zhex_int_to_string (local.get $lineno)) (call $m3_line_of) (call $zhex_quote (local.get $path)) (call $m3_not_kv)) (call $zhex_quote (call $zhex_bytes_to_string (i32.add (i32.const 4) (local.get $a)) (i32.sub (local.get $b) (local.get $a))))))))
          )
        )
      )
      ;; advance past the newline
      (if (i32.ge_u (local.get $eol) (local.get $n))
        (then
          (return (struct.new $zhex_result (i32.const 0) (ref.null eq)))
        )
      )
      (local.set $start (i32.add (local.get $eol) (i32.const 1)))
      (br $line)
    )
  )
  ;; every exit returns from inside; the loop never falls through
  (unreachable)
)

`

// watM3JsonTypes carries the Json-related types (declared in watM3Types
// with the rec group); kept as a separate section name for the Join.
const watM3JsonTypes = ``

// watM3JsonParse: the JSON parser (DESIGN-M3 §6). A single global cursor
// walks the code-point array; every step returns (ref $zhex_result) so
// failures carry Go 1.27 encoding/json's phrasing verbatim — the tables in
// watprelude_m3_test.go were pinned from the interpreter
// (internal/lang/json_codec.go over encoding/json), not guessed. "invalid
// JSON: " prefixes nest exactly where the interpreter's jsonSyntaxError puts
// them; semantic failures (duplicate key, surrogates, trailing values) carry
// no prefix. The surrogate preamble runs over the raw text before parsing
// and always reports at "" (validateJSONSurrogates, byte semantics mapped
// one-to-one onto code points — the scanned characters are ASCII).
const watM3JsonParse = `
(global $zjson_i (mut i32) (i32.const 0))
(global $zjson_num_start (mut i32) (i32.const 0))

(func $zjson_peek (param $s (ref $zhex_string)) (result i32)
  (if (i32.ge_u (global.get $zjson_i) (array.len (local.get $s)))
    (then (return (i32.const -1))))
  (array.get $zhex_string (local.get $s) (global.get $zjson_i)))

(func $zjson_step
  (global.set $zjson_i (i32.add (global.get $zjson_i) (i32.const 1))))

(func $zjson_skip_ws (param $s (ref $zhex_string))
  (local $c i32)
  (block $done
    (loop $next
      (local.set $c (call $zjson_peek (local.get $s)))
      (br_if $done (i32.eq (local.get $c) (i32.const -1)))
      (br_if $done (i32.eqz
        (i32.or (i32.or (i32.or (i32.eq (local.get $c) (i32.const 32)) (i32.eq (local.get $c) (i32.const 9)))
                        (i32.eq (local.get $c) (i32.const 10)))
                (i32.eq (local.get $c) (i32.const 13)))))
      (call $zjson_step)
      (br $next))))

(func $zjson_fail (param $msg (ref $zhex_string)) (param $path (ref $zhex_string)) (result (ref $zhex_result))
  (struct.new $zhex_result (i32.const 1)
    (call $zhex_errv (i32.const 5)
      (call $zhex_cat3 (local.get $msg) (call $mj_at) (call $zhex_quote (local.get $path))))))

;; Go's single-rune %q: printable ASCII as-is (with \' \\ \" escapes), the
;; C escapes for \t \n \r, \xHH lowercase for the remaining controls
(func $mj_char_q (param $c i32) (result (ref $zhex_string))
  (local $r (ref null $zhex_string))
  (if (i32.and (i32.ge_u (local.get $c) (i32.const 32)) (i32.le_u (local.get $c) (i32.const 126)))
    (then
      (if (i32.or (i32.eq (local.get $c) (i32.const 39)) (i32.eq (local.get $c) (i32.const 92)))
        (then
          (local.set $r (array.new_fixed $zhex_string 4 (i32.const 39) (i32.const 92) (local.get $c) (i32.const 39)))
          (return (ref.as_non_null (local.get $r))))
        (else
          (return (array.new_fixed $zhex_string 3 (i32.const 39) (local.get $c) (i32.const 39)))))))
  (if (i32.or (i32.or (i32.eq (local.get $c) (i32.const 9)) (i32.eq (local.get $c) (i32.const 10))) (i32.eq (local.get $c) (i32.const 13)))
    (then
      (return (array.new_fixed $zhex_string 4 (i32.const 39) (i32.const 92)
        (select (i32.const 116)
          (select (i32.const 110) (i32.const 114) (i32.eq (local.get $c) (i32.const 10)))
          (i32.eq (local.get $c) (i32.const 9)))
        (i32.const 39)))))
  (call $zhex_str_concat
    (call $zhex_str_concat
      (array.new_fixed $zhex_string 3 (i32.const 39) (i32.const 92) (i32.const 120))
      (array.new_fixed $zhex_string 2
        (call $mj_hex_digit (i32.shr_u (local.get $c) (i32.const 4)))
        (call $mj_hex_digit (i32.and (local.get $c) (i32.const 15)))))
    (array.new_fixed $zhex_string 1 (i32.const 39))))

(func $mj_hex_digit (param $v i32) (result i32)
  (select (i32.add (local.get $v) (i32.const 87)) (i32.add (local.get $v) (i32.const 48)) (i32.gt_u (local.get $v) (i32.const 9))))

(func $mj_hex_val (param $c i32) (result i32)
  (if (i32.and (i32.ge_u (local.get $c) (i32.const 48)) (i32.le_u (local.get $c) (i32.const 57)))
    (then (return (i32.sub (local.get $c) (i32.const 48)))))
  (if (i32.and (i32.ge_u (local.get $c) (i32.const 97)) (i32.le_u (local.get $c) (i32.const 102)))
    (then (return (i32.add (i32.sub (local.get $c) (i32.const 97)) (i32.const 10)))))
  (if (i32.and (i32.ge_u (local.get $c) (i32.const 65)) (i32.le_u (local.get $c) (i32.const 70)))
    (then (return (i32.add (i32.sub (local.get $c) (i32.const 65)) (i32.const 10)))))
  (i32.const -1))

;; four hex code points at $at, or -1
(func $mj_hex4 (param $s (ref $zhex_string)) (param $at i32) (result i32)
  (local $v i32) (local $d i32) (local $k i32)
  (block $done
    (loop $digit
      (br_if $done (i32.ge_u (local.get $k) (i32.const 4)))
      (local.set $d (call $mj_hex_val (array.get $zhex_string (local.get $s) (i32.add (local.get $at) (local.get $k)))))
      (if (i32.lt_s (local.get $d) (i32.const 0))
        (then (return (i32.const -1))))
      (local.set $v (i32.add (i32.mul (local.get $v) (i32.const 16)) (local.get $d)))
      (local.set $k (i32.add (local.get $k) (i32.const 1)))
      (br $digit)))
  (local.get $v))

;; the strictness preamble (validateJSONSurrogates verbatim): scans string
;; literals only; \u with non-hex breaks the scan as "ok" (the parser then
;; reports the invalid escape); an unpaired surrogate fails at ""
(func $zhex_json_surrogates_ok (param $s (ref $zhex_string)) (result i32)
  (local $n i32) (local $i i32) (local $in i32) (local $c i32) (local $h1 i32) (local $h2 i32)
  (local.set $n (array.len (local.get $s)))
  (block $done
    (loop $scan
      (br_if $done (i32.ge_u (local.get $i) (local.get $n)))
      (local.set $c (array.get $zhex_string (local.get $s) (local.get $i)))
      (if (i32.eqz (local.get $in))
        (then
          (if (i32.eq (local.get $c) (i32.const 34)) (then (local.set $in (i32.const 1))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $scan)))
      (if (i32.eq (local.get $c) (i32.const 34))
        (then
          (local.set $in (i32.const 0))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $scan)))
      (if (i32.ne (local.get $c) (i32.const 92))
        (then
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $scan)))
      (if (i32.ge_u (i32.add (local.get $i) (i32.const 1)) (local.get $n)) (then (return (i32.const 1))))
      (if (i32.ne (array.get $zhex_string (local.get $s) (i32.add (local.get $i) (i32.const 1))) (i32.const 117))
        (then
          (local.set $i (i32.add (local.get $i) (i32.const 2)))
          (br $scan)))
      (if (i32.gt_u (i32.add (local.get $i) (i32.const 6)) (local.get $n)) (then (return (i32.const 1))))
      (local.set $h1 (call $mj_hex4 (local.get $s) (i32.add (local.get $i) (i32.const 2))))
      (if (i32.lt_s (local.get $h1) (i32.const 0)) (then (return (i32.const 1))))
      (if (i32.and (i32.ge_u (local.get $h1) (i32.const 56320)) (i32.le_u (local.get $h1) (i32.const 57343)))
        (then (return (i32.const 0))))
      (if (i32.and (i32.ge_u (local.get $h1) (i32.const 55296)) (i32.le_u (local.get $h1) (i32.const 56319)))
        (then
          (if (i32.gt_u (i32.add (local.get $i) (i32.const 12)) (local.get $n)) (then (return (i32.const 0))))
          (if (i32.ne (array.get $zhex_string (local.get $s) (i32.add (local.get $i) (i32.const 6))) (i32.const 92))
            (then (return (i32.const 0))))
          (if (i32.ne (array.get $zhex_string (local.get $s) (i32.add (local.get $i) (i32.const 7))) (i32.const 117))
            (then (return (i32.const 0))))
          (local.set $h2 (call $mj_hex4 (local.get $s) (i32.add (local.get $i) (i32.const 8))))
          (if (i32.lt_s (local.get $h2) (i32.const 0)) (then (return (i32.const 0))))
          (if (i32.or (i32.lt_u (local.get $h2) (i32.const 56320)) (i32.gt_u (local.get $h2) (i32.const 57343)))
            (then (return (i32.const 0))))
          (local.set $i (i32.add (local.get $i) (i32.const 12)))
          (br $scan)))
      (local.set $i (i32.add (local.get $i) (i32.const 6)))
      (br $scan)))
  (i32.const 1))

;; path pieces (jsonPath): /<index> and /<key> with ~0/~1 escaping
(func $zjson_path_index (param $path (ref $zhex_string)) (param $i i32) (result (ref $zhex_string))
  (call $zhex_str_concat
    (call $zhex_str_concat (local.get $path) (array.new_fixed $zhex_string 1 (i32.const 47)))
    (call $zhex_int_to_string (i64.extend_i32_u (local.get $i)))))

(func $zjson_path_key (param $path (ref $zhex_string)) (param $key (ref $zhex_string)) (result (ref $zhex_string))
  (local $n i32) (local $i i32) (local $c i32) (local $w i32)
  (local $out (ref null $zhex_string))
  (local.set $n (array.len (local.get $key)))
  (local.set $out (array.new_default $zhex_string (i32.add (i32.mul (local.get $n) (i32.const 2)) (i32.const 1))))
  (block $kd
    (loop $kl
      (br_if $kd (i32.ge_u (local.get $i) (local.get $n)))
      (local.set $c (array.get $zhex_string (local.get $key) (local.get $i)))
      (if (i32.eq (local.get $c) (i32.const 126))
        (then
          (array.set $zhex_string (ref.as_non_null (local.get $out)) (local.get $w) (i32.const 126))
          (local.set $w (i32.add (local.get $w) (i32.const 1)))
          (array.set $zhex_string (ref.as_non_null (local.get $out)) (local.get $w) (i32.const 48)))
        (else
          (if (i32.eq (local.get $c) (i32.const 47))
            (then
              (array.set $zhex_string (ref.as_non_null (local.get $out)) (local.get $w) (i32.const 126))
              (local.set $w (i32.add (local.get $w) (i32.const 1)))
              (array.set $zhex_string (ref.as_non_null (local.get $out)) (local.get $w) (i32.const 49)))
            (else
              (array.set $zhex_string (ref.as_non_null (local.get $out)) (local.get $w) (local.get $c))))))
      (local.set $w (i32.add (local.get $w) (i32.const 1)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $kl)))
  (call $zhex_str_concat
    (call $zhex_str_concat (local.get $path) (array.new_fixed $zhex_string 1 (i32.const 47)))
    (call $zhex_str_sub (ref.as_non_null (local.get $out)) (i32.const 0) (local.get $w))))

;; grow-by-copy append for the nullable-eqref arrays the parser builds
(func $zjson_arr_push (param $arr (ref null $zhex_arr_ref)) (param $v (ref null eq)) (result (ref $zhex_arr_ref))
  (local $n i32) (local $i i32)
  (local $fresh (ref null $zhex_arr_ref))
  (if (ref.is_null (local.get $arr))
    (then
      (local.set $fresh (array.new_default $zhex_arr_ref (i32.const 1)))
      (array.set $zhex_arr_ref (ref.as_non_null (local.get $fresh)) (i32.const 0) (local.get $v))
      (return (ref.as_non_null (local.get $fresh))))
  )
  (local.set $n (array.len (ref.as_non_null (local.get $arr))))
  (local.set $fresh (array.new_default $zhex_arr_ref (i32.add (local.get $n) (i32.const 1))))
  (block $cp
    (loop $cl
      (br_if $cp (i32.ge_u (local.get $i) (local.get $n)))
      (array.set $zhex_arr_ref (ref.as_non_null (local.get $fresh)) (local.get $i)
        (array.get $zhex_arr_ref (ref.as_non_null (local.get $arr)) (local.get $i)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $cl)))
  (array.set $zhex_arr_ref (ref.as_non_null (local.get $fresh)) (local.get $n) (local.get $v))
  (ref.as_non_null (local.get $fresh)))

;; renders the invalid-escape-sequence message: backslash-u + gathered
;; min(4, remaining) code points taken verbatim from $at
(func $mj_esc_seq (param $s (ref $zhex_string)) (param $at i32) (param $max i32) (result (ref $zhex_string))
  (local $seq (ref null $zhex_string)) (local $i i32)
  (if (i32.gt_u (local.get $max) (i32.const 4)) (then (local.set $max (i32.const 4))))
  (local.set $seq (array.new_default $zhex_string (i32.add (local.get $max) (i32.const 2))))
  (array.set $zhex_string (ref.as_non_null (local.get $seq)) (i32.const 0) (i32.const 92))
  (array.set $zhex_string (ref.as_non_null (local.get $seq)) (i32.const 1) (i32.const 117))
  (block $g
    (loop $gl
      (br_if $g (i32.ge_u (local.get $i) (local.get $max)))
      (array.set $zhex_string (ref.as_non_null (local.get $seq)) (i32.add (local.get $i) (i32.const 2))
        (array.get $zhex_string (local.get $s) (i32.add (local.get $at) (local.get $i))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $gl)))
  (call $zhex_cat3 (call $mj_esc_pre) (ref.as_non_null (local.get $seq)) (call $mj_esc_suf)))

;; the simple escape code points: n t r b f, anything else maps to itself
(func $mj_esc_cp (param $e i32) (result i32)
  (if (i32.eq (local.get $e) (i32.const 110)) (then (return (i32.const 10))))
  (if (i32.eq (local.get $e) (i32.const 116)) (then (return (i32.const 9))))
  (if (i32.eq (local.get $e) (i32.const 114)) (then (return (i32.const 13))))
  (if (i32.eq (local.get $e) (i32.const 98)) (then (return (i32.const 8))))
  (if (i32.eq (local.get $e) (i32.const 102)) (then (return (i32.const 12))))
  (local.get $e))

;; the recognized single-character escapes: " \ / b f n r t
(func $mj_simple_esc (param $e i32) (result i32)
  (if (i32.eq (local.get $e) (i32.const 34)) (then (return (i32.const 1))))
  (if (i32.eq (local.get $e) (i32.const 92)) (then (return (i32.const 1))))
  (if (i32.eq (local.get $e) (i32.const 47)) (then (return (i32.const 1))))
  (if (i32.eq (local.get $e) (i32.const 98)) (then (return (i32.const 1))))
  (if (i32.eq (local.get $e) (i32.const 102)) (then (return (i32.const 1))))
  (if (i32.eq (local.get $e) (i32.const 110)) (then (return (i32.const 1))))
  (if (i32.eq (local.get $e) (i32.const 114)) (then (return (i32.const 1))))
  (if (i32.eq (local.get $e) (i32.const 116)) (then (return (i32.const 1))))
  (i32.const 0))

;; ---- string literal: pass 1 counts and validates, pass 2 fills ------------
;; cursor sits just after the opening quote; leaves it just past the closer
(func $zjson_scan_string (param $s (ref $zhex_string)) (param $path (ref $zhex_string)) (result (ref $zhex_result))
  (local $n i32) (local $i i32) (local $c i32) (local $e i32) (local $h1 i32) (local $h2 i32)
  (local $count i32) (local $r (ref null $zhex_string)) (local $w i32)
  (local.set $n (array.len (local.get $s)))
  (local.set $i (global.get $zjson_i))
  (block $passed
    (loop $p1
      (if (i32.ge_u (local.get $i) (local.get $n))
        (then (return (call $zjson_fail (call $mj_unexp_eof) (local.get $path)))))
      (local.set $c (array.get $zhex_string (local.get $s) (local.get $i)))
      (if (i32.eq (local.get $c) (i32.const 34))
        (then (br $passed)))
      (if (i32.eq (local.get $c) (i32.const 92))
        (then
          (if (i32.ge_u (i32.add (local.get $i) (i32.const 1)) (local.get $n))
            (then (return (call $zjson_fail (call $mj_unexp_eof) (local.get $path)))))
          (local.set $e (array.get $zhex_string (local.get $s) (i32.add (local.get $i) (i32.const 1))))
          (if (i32.eq (local.get $e) (i32.const 117))
            (then
              (if (i32.gt_u (i32.add (local.get $i) (i32.const 6)) (local.get $n))
                (then
                  (return (call $zjson_fail
                    (call $mj_esc_seq (local.get $s) (i32.add (local.get $i) (i32.const 2))
                      (i32.sub (local.get $n) (i32.add (local.get $i) (i32.const 2))))
                    (local.get $path)))))
              (local.set $h1 (call $mj_hex4 (local.get $s) (i32.add (local.get $i) (i32.const 2))))
              (if (i32.lt_s (local.get $h1) (i32.const 0))
                (then
                  (return (call $zjson_fail
                    (call $mj_esc_seq (local.get $s) (i32.add (local.get $i) (i32.const 2)) (i32.const 4))
                    (local.get $path)))))
              (if (i32.and (i32.ge_u (local.get $h1) (i32.const 55296)) (i32.le_u (local.get $h1) (i32.const 56319)))
                (then
                  ;; a high surrogate is paired (the preamble rejected lone
                  ;; ones): combine into one code point
                  (local.set $h2 (call $mj_hex4 (local.get $s) (i32.add (local.get $i) (i32.const 8))))
                  (local.set $i (i32.add (local.get $i) (i32.const 12)))
                  (local.set $count (i32.add (local.get $count) (i32.const 1)))
                  (br $p1)))
              (local.set $i (i32.add (local.get $i) (i32.const 6)))
              (local.set $count (i32.add (local.get $count) (i32.const 1)))
              (br $p1)))
          (if (call $mj_simple_esc (local.get $e))
            (then
              (local.set $i (i32.add (local.get $i) (i32.const 2)))
              (local.set $count (i32.add (local.get $count) (i32.const 1)))
              (br $p1)))
          (return (call $zjson_fail
            (call $zhex_cat3 (call $mj_esc_pre) (array.new_fixed $zhex_string 2 (i32.const 92) (local.get $e)) (call $mj_esc_suf))
            (local.get $path))))
        (else
          (if (i32.lt_u (local.get $c) (i32.const 32))
            (then
              (return (call $zjson_fail
                (call $zhex_cat3 (call $mj_inv_char) (call $mj_char_q (local.get $c)) (call $mj_in_str_b))
                (local.get $path)))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (local.set $count (i32.add (local.get $count) (i32.const 1)))
          (br $p1)))
      (br $p1))
    ;; ran off the end without a closing quote
    (return (call $zjson_fail (call $mj_unexp_eof) (local.get $path))))
  (local.set $r (array.new_default $zhex_string (local.get $count)))
  (local.set $i (global.get $zjson_i))
  (local.set $w (i32.const 0))
  (block $filled
    (loop $p2
      (local.set $c (array.get $zhex_string (local.get $s) (local.get $i)))
      (if (i32.eq (local.get $c) (i32.const 34)) (then (br $filled)))
      (if (i32.eq (local.get $c) (i32.const 92))
        (then
          (local.set $e (array.get $zhex_string (local.get $s) (i32.add (local.get $i) (i32.const 1))))
          (if (i32.eq (local.get $e) (i32.const 117))
            (then
              (local.set $h1 (call $mj_hex4 (local.get $s) (i32.add (local.get $i) (i32.const 2))))
              (if (i32.and (i32.ge_u (local.get $h1) (i32.const 55296)) (i32.le_u (local.get $h1) (i32.const 56319)))
                (then
                  (local.set $h2 (call $mj_hex4 (local.get $s) (i32.add (local.get $i) (i32.const 8))))
                  (array.set $zhex_string (ref.as_non_null (local.get $r)) (local.get $w)
                    (i32.add (i32.const 65536)
                      (i32.add (i32.shl (i32.sub (local.get $h1) (i32.const 55296)) (i32.const 10))
                        (i32.sub (local.get $h2) (i32.const 56320)))))
                  (local.set $i (i32.add (local.get $i) (i32.const 12)))
                  (local.set $w (i32.add (local.get $w) (i32.const 1)))
                  (br $p2)))
              (array.set $zhex_string (ref.as_non_null (local.get $r)) (local.get $w) (local.get $h1))
              (local.set $i (i32.add (local.get $i) (i32.const 6)))
              (local.set $w (i32.add (local.get $w) (i32.const 1)))
              (br $p2)))
          (array.set $zhex_string (ref.as_non_null (local.get $r)) (local.get $w)
            (call $mj_esc_cp (local.get $e)))
          (local.set $i (i32.add (local.get $i) (i32.const 2)))
          (local.set $w (i32.add (local.get $w) (i32.const 1)))
          (br $p2)))
      (array.set $zhex_string (ref.as_non_null (local.get $r)) (local.get $w) (local.get $c))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (local.set $w (i32.add (local.get $w) (i32.const 1)))
      (br $p2)))
  (global.set $zjson_i (i32.add (local.get $i) (i32.const 1)))
  (struct.new $zhex_result (i32.const 0) (ref.as_non_null (local.get $r))))

;; ---- number: the JSON grammar, raw literal captured as text ---------------
(func $zjson_parse_number (param $s (ref $zhex_string)) (param $path (ref $zhex_string)) (result (ref $zhex_result))
  (local $c i32)
  (global.set $zjson_num_start (global.get $zjson_i))
  (if (i32.eq (call $zjson_peek (local.get $s)) (i32.const 45)) (then (call $zjson_step)))
  (local.set $c (call $zjson_peek (local.get $s)))
  (if (i32.eq (local.get $c) (i32.const -1))
    (then (return (call $zjson_fail (call $mj_unexp_eof) (local.get $path)))))
  (if (i32.eq (local.get $c) (i32.const 48))
    (then (call $zjson_step))
    (else
      (if (i32.and (i32.ge_u (local.get $c) (i32.const 49)) (i32.le_u (local.get $c) (i32.const 57)))
        (then
          (block $digits
            (loop $dl
              (call $zjson_step)
              (local.set $c (call $zjson_peek (local.get $s)))
              (br_if $digits (i32.or (i32.lt_u (local.get $c) (i32.const 48)) (i32.gt_u (local.get $c) (i32.const 57))))
              (br $dl))))
        (else
          (return (call $zjson_fail
            (call $zhex_cat3 (call $mj_inv_char) (call $mj_char_q (local.get $c)) (call $mj_numeric_b))
            (local.get $path)))))))
  (if (i32.eq (call $zjson_peek (local.get $s)) (i32.const 46))
    (then
      (call $zjson_step)
      (local.set $c (call $zjson_peek (local.get $s)))
      (if (i32.eq (local.get $c) (i32.const -1))
        (then (return (call $zjson_fail (call $mj_unexp_eof) (local.get $path)))))
      (if (i32.or (i32.lt_u (local.get $c) (i32.const 48)) (i32.gt_u (local.get $c) (i32.const 57)))
        (then
          (return (call $zjson_fail
            (call $zhex_cat3 (call $mj_inv_char) (call $mj_char_q (local.get $c)) (call $mj_numeric_b))
            (local.get $path)))))
      (block $fd
        (loop $fl
          (call $zjson_step)
          (local.set $c (call $zjson_peek (local.get $s)))
          (br_if $fd (i32.or (i32.lt_u (local.get $c) (i32.const 48)) (i32.gt_u (local.get $c) (i32.const 57))))
          (br $fl)))))
  (local.set $c (call $zjson_peek (local.get $s)))
  (if (i32.or (i32.eq (local.get $c) (i32.const 101)) (i32.eq (local.get $c) (i32.const 69)))
    (then
      (call $zjson_step)
      (local.set $c (call $zjson_peek (local.get $s)))
      (if (i32.or (i32.eq (local.get $c) (i32.const 43)) (i32.eq (local.get $c) (i32.const 45)))
        (then (call $zjson_step)))
      (local.set $c (call $zjson_peek (local.get $s)))
      (if (i32.eq (local.get $c) (i32.const -1))
        (then (return (call $zjson_fail (call $mj_unexp_eof) (local.get $path)))))
      (if (i32.or (i32.lt_u (local.get $c) (i32.const 48)) (i32.gt_u (local.get $c) (i32.const 57)))
        (then
          (return (call $zjson_fail
            (call $zhex_cat3 (call $mj_inv_char) (call $mj_char_q (local.get $c)) (call $mj_numeric_b))
            (local.get $path)))))
      (block $ed
        (loop $el
          (call $zjson_step)
          (local.set $c (call $zjson_peek (local.get $s)))
          (br_if $ed (i32.or (i32.lt_u (local.get $c) (i32.const 48)) (i32.gt_u (local.get $c) (i32.const 57))))
          (br $el)))))
  (struct.new $zhex_result (i32.const 0)
    (struct.new $zjson_node (i32.const 3) (i32.const 0)
      (call $zhex_str_sub (local.get $s) (global.get $zjson_num_start) (global.get $zjson_i))
      (ref.null $zhex_arr_ref) (ref.null $zhex_arr_ref))))

;; ---- literals: $e1..$e4 are the expected characters after the consumed ----
;; first one (0 = none); $word is the literal name for the message, $rkind
;; 0 null / 1 true / 2 false
(func $zjson_parse_lit (param $s (ref $zhex_string)) (param $path (ref $zhex_string))
    (param $e1 i32) (param $e2 i32) (param $e3 i32) (param $e4 i32)
    (param $word (ref $zhex_string)) (param $rkind i32) (result (ref $zhex_result))
  (local $c i32) (local $e i32) (local $k i32)
  (block $done
    (loop $next
      (br_if $done (i32.ge_u (local.get $k) (i32.const 4)))
      (local.set $e
        (if (result i32)
          (i32.eq (local.get $k) (i32.const 0))
          (then (local.get $e1))
          (else
            (if (result i32)
              (i32.eq (local.get $k) (i32.const 1))
              (then (local.get $e2))
              (else
                (if (result i32)
                  (i32.eq (local.get $k) (i32.const 2))
                  (then (local.get $e3))
                  (else (local.get $e4))))))))
      (if (i32.eqz (local.get $e))
        (then (br $done)))
      (local.set $c (call $zjson_peek (local.get $s)))
      (if (i32.eq (local.get $c) (i32.const -1))
        (then (return (call $zjson_fail (call $mj_unexp_eof) (local.get $path)))))
      (if (i32.ne (local.get $c) (local.get $e))
        (then
          (return (call $zjson_fail
            (call $zhex_cat3
              (call $zhex_cat3 (call $mj_inv_char) (call $mj_char_q (local.get $c)) (call $mj_in_lit))
              (local.get $word)
              (call $zhex_cat3 (call $mj_expecting) (call $mj_char_q (local.get $e))
                (array.new_fixed $zhex_string 1 (i32.const 41))))
            (local.get $path)))))
      (call $zjson_step)
      (local.set $k (i32.add (local.get $k) (i32.const 1)))
      (br $next)))
  (if (i32.eq (local.get $rkind) (i32.const 0))
    (then
      (return (struct.new $zhex_result (i32.const 0)
        (struct.new $zjson_node (i32.const 0) (i32.const 0) (ref.null $zhex_string) (ref.null $zhex_arr_ref) (ref.null $zhex_arr_ref))))))
  (struct.new $zhex_result (i32.const 0)
    (struct.new $zjson_node (i32.const 1) (i32.eq (local.get $rkind) (i32.const 1))
      (ref.null $zhex_string) (ref.null $zhex_arr_ref) (ref.null $zhex_arr_ref))))

;; ---- the value dispatch: ctx 0 top, 1 array element, 2 object value -------
(func $zjson_parse_value (param $s (ref $zhex_string)) (param $path (ref $zhex_string)) (param $depth i32) (param $ctx i32) (result (ref $zhex_result))
  (local $c i32)
  (local $r (ref $zhex_result))
  (if (i32.gt_u (local.get $depth) (i32.const 256))
    (then (return (call $zjson_fail (call $mj_nesting) (local.get $path)))))
  (call $zjson_skip_ws (local.get $s))
  (local.set $c (call $zjson_peek (local.get $s)))
  (if (i32.eq (local.get $c) (i32.const -1))
    (then
      (if (i32.eq (local.get $ctx) (i32.const 1))
        (then (return (call $zjson_fail (call $mj_unexp_end) (local.get $path)))))
      (return (call $zjson_fail (call $mj_eof) (local.get $path)))))
  (if (i32.eq (local.get $c) (i32.const 123))
    (then
      (call $zjson_step)
      (return (call $zjson_parse_object (local.get $s) (local.get $path) (i32.add (local.get $depth) (i32.const 1))))))
  (if (i32.eq (local.get $c) (i32.const 91))
    (then
      (call $zjson_step)
      (return (call $zjson_parse_array (local.get $s) (local.get $path) (i32.add (local.get $depth) (i32.const 1))))))
  (if (i32.eq (local.get $c) (i32.const 34))
    (then
      (call $zjson_step)
      (local.set $r (call $zjson_scan_string (local.get $s) (local.get $path)))
      (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
        (then (return (local.get $r))))
      (return (struct.new $zhex_result (i32.const 0)
        (struct.new $zjson_node (i32.const 2) (i32.const 0)
          (ref.cast (ref $zhex_string) (struct.get $zhex_result $payload (local.get $r)))
          (ref.null $zhex_arr_ref) (ref.null $zhex_arr_ref))))))
  (if (i32.eq (local.get $c) (i32.const 116))
    (then
      (call $zjson_step)
      (return (call $zjson_parse_lit (local.get $s) (local.get $path)
        (i32.const 114) (i32.const 117) (i32.const 101) (i32.const 0) (call $mj_true) (i32.const 1)))))
  (if (i32.eq (local.get $c) (i32.const 102))
    (then
      (call $zjson_step)
      (return (call $zjson_parse_lit (local.get $s) (local.get $path)
        (i32.const 97) (i32.const 108) (i32.const 115) (i32.const 101) (call $mj_false) (i32.const 2)))))
  (if (i32.eq (local.get $c) (i32.const 110))
    (then
      (call $zjson_step)
      (return (call $zjson_parse_lit (local.get $s) (local.get $path)
        (i32.const 117) (i32.const 108) (i32.const 108) (i32.const 0) (call $mj_null) (i32.const 0)))))
  (if (i32.or (i32.eq (local.get $c) (i32.const 45))
              (i32.and (i32.ge_u (local.get $c) (i32.const 48)) (i32.le_u (local.get $c) (i32.const 57))))
    (then (return (call $zjson_parse_number (local.get $s) (local.get $path)))))
  (if (i32.eq (local.get $ctx) (i32.const 2))
    (then
      (return (call $zjson_fail
        (call $zhex_cat3 (call $mj_inv_char) (call $mj_char_q (local.get $c)) (call $mj_after_key_b))
        (local.get $path)))))
  (call $zjson_fail
    (call $zhex_cat3 (call $mj_inv_char) (call $mj_char_q (local.get $c)) (call $mj_looking_b))
    (local.get $path)))

;; ---- array: cursor sits just after '[' --------------------------------------
(func $zjson_parse_array (param $s (ref $zhex_string)) (param $path (ref $zhex_string)) (param $depth i32) (result (ref $zhex_result))
  (local $c i32) (local $first i32) (local $count i32)
  (local $items (ref null $zhex_arr_ref))
  (local $r (ref $zhex_result))
  (local.set $first (i32.const 1))
  (block $done
    (loop $next
      (call $zjson_skip_ws (local.get $s))
      (local.set $c (call $zjson_peek (local.get $s)))
      (if (local.get $first)
        (then
          (if (i32.eq (local.get $c) (i32.const 93))
            (then
              (call $zjson_step)
              (br $done)))
          (if (i32.eq (local.get $c) (i32.const 125))
            (then
              (return (call $zjson_fail
                (call $zhex_cat3 (call $mj_inv_char) (array.new_fixed $zhex_string 3 (i32.const 39) (i32.const 125) (i32.const 39)) (call $mj_looking_b))
                (local.get $path)))))
          (if (i32.eq (local.get $c) (i32.const -1))
            (then
              (return (call $zjson_fail (call $mj_unexp_end) (call $zjson_path_index (local.get $path) (local.get $count)))))))
        (else
          (if (i32.eq (local.get $c) (i32.const 44))
            (then
              (call $zjson_step)
              (call $zjson_skip_ws (local.get $s))
              (local.set $c (call $zjson_peek (local.get $s)))
              (if (i32.or (i32.eq (local.get $c) (i32.const 93)) (i32.eq (local.get $c) (i32.const 125)))
                (then
                  (return (call $zjson_fail
                    (call $zhex_cat3 (call $mj_inv_char) (array.new_fixed $zhex_string 3 (i32.const 39) (i32.const 44) (i32.const 39)) (call $mj_looking_b))
                    (call $zjson_path_index (local.get $path) (local.get $count))))))
              (if (i32.eq (local.get $c) (i32.const -1))
                (then
                  (return (call $zjson_fail (call $mj_unexp_end) (call $zjson_path_index (local.get $path) (local.get $count)))))))
            (else
              (if (i32.eq (local.get $c) (i32.const 93))
                (then
                  (call $zjson_step)
                  (br $done)))
              (if (i32.eq (local.get $c) (i32.const -1))
                (then
                  (return (call $zjson_fail (call $mj_unexp_end) (call $zjson_path_index (local.get $path) (local.get $count))))))
              (return (call $zjson_fail
                (call $zhex_cat3 (call $mj_inv_char) (call $mj_char_q (local.get $c)) (call $mj_after_arr_b))
                (call $zjson_path_index (local.get $path) (local.get $count))))))))
      (local.set $r (call $zjson_parse_value (local.get $s)
        (call $zjson_path_index (local.get $path) (local.get $count))
        (local.get $depth) (i32.const 1)))
      (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
        (then (return (local.get $r))))
      (local.set $items (call $zjson_arr_push (local.get $items) (struct.get $zhex_result $payload (local.get $r))))
      (local.set $first (i32.const 0))
      (local.set $count (i32.add (local.get $count) (i32.const 1)))
      (br $next)))
  (if (ref.is_null (local.get $items))
    (then (local.set $items (array.new_default $zhex_arr_ref (i32.const 0)))))
  (struct.new $zhex_result (i32.const 0)
    (struct.new $zjson_node (i32.const 4) (i32.const 0) (ref.null $zhex_string)
      (ref.as_non_null (local.get $items)) (ref.null $zhex_arr_ref))))

(func $zjson_parse_object (param $s (ref $zhex_string)) (param $path (ref $zhex_string)) (param $depth i32) (result (ref $zhex_result))
  (local $c i32) (local $first i32) (local $count i32) (local $i i32)
  (local $key (ref null $zhex_string))
  (local $members (ref null $zhex_arr_ref))
  (local $r (ref $zhex_result))
  (local.set $first (i32.const 1))
  (block $done
    (loop $next
      (call $zjson_skip_ws (local.get $s))
      (local.set $c (call $zjson_peek (local.get $s)))
      (if (i32.eq (local.get $c) (i32.const 125))
        (then
          (if (local.get $first)
            (then
              (call $zjson_step)
              (br $done)))
          (return (call $zjson_fail
            (call $zhex_cat3 (call $mj_inv_char) (array.new_fixed $zhex_string 3 (i32.const 39) (i32.const 44) (i32.const 39)) (call $mj_looking_b))
            (local.get $path)))))
      (if (i32.eq (local.get $c) (i32.const -1))
        (then
          (return (call $zjson_fail (call $mj_unexp_end) (local.get $path)))))
      (if (i32.eq (local.get $c) (i32.const 34))
        (then
          (call $zjson_step)
          (local.set $r (call $zjson_scan_string (local.get $s) (local.get $path)))
          (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
            (then (return (local.get $r))))
          (local.set $key (ref.cast (ref $zhex_string) (struct.get $zhex_result $payload (local.get $r)))))
        (else
          (if (i32.eq (local.get $c) (i32.const 44))
            (then
              (return (call $zjson_fail
                (call $zhex_cat3 (call $mj_inv_char) (array.new_fixed $zhex_string 3 (i32.const 39) (i32.const 44) (i32.const 39)) (call $mj_looking_b))
                (local.get $path)))))
          (return (call $zjson_fail (call $mj_member_name) (local.get $path)))))
      ;; duplicate key check: linear scan of the members so far
      (local.set $i (i32.const 0))
      (block $uniq
        (loop $ul
          (br_if $uniq (i32.ge_u (local.get $i) (local.get $count)))
          (if (call $zhex_str_eq
                (struct.get $zjson_member $key
                  (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $members)) (local.get $i))))
                (ref.as_non_null (local.get $key)))
            (then
              (return (call $zjson_fail (call $mj_dup_key)
                (call $zjson_path_key (local.get $path) (ref.as_non_null (local.get $key)))))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $ul)))
      ;; the colon
      (call $zjson_skip_ws (local.get $s))
      (local.set $c (call $zjson_peek (local.get $s)))
      (if (i32.eq (local.get $c) (i32.const -1))
        (then
          (return (call $zjson_fail (call $mj_eof) (call $zjson_path_key (local.get $path) (ref.as_non_null (local.get $key)))))))
      (if (i32.ne (local.get $c) (i32.const 58))
        (then
          (return (call $zjson_fail
            (call $zhex_cat3 (call $mj_inv_char) (call $mj_char_q (local.get $c)) (call $mj_after_key_b))
            (call $zjson_path_key (local.get $path) (ref.as_non_null (local.get $key)))))))
      (call $zjson_step)
      ;; the value
      (local.set $r (call $zjson_parse_value (local.get $s)
        (call $zjson_path_key (local.get $path) (ref.as_non_null (local.get $key)))
        (local.get $depth) (i32.const 2)))
      (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
        (then (return (local.get $r))))
      (local.set $members (call $zjson_arr_push (local.get $members)
        (struct.new $zjson_member (ref.as_non_null (local.get $key))
          (ref.cast (ref null $zjson_node) (struct.get $zhex_result $payload (local.get $r))))))
      (local.set $first (i32.const 0))
      (local.set $count (i32.add (local.get $count) (i32.const 1)))
      ;; ',' or '}'
      (call $zjson_skip_ws (local.get $s))
      (local.set $c (call $zjson_peek (local.get $s)))
      (if (i32.eq (local.get $c) (i32.const 125))
        (then
          (call $zjson_step)
          (br $done)))
      (if (i32.eq (local.get $c) (i32.const -1))
        (then
          (return (call $zjson_fail (call $mj_unexp_end) (local.get $path)))))
      (if (i32.ne (local.get $c) (i32.const 44))
        (then
          (return (call $zjson_fail
            (call $zhex_cat3 (call $mj_inv_char) (call $mj_char_q (local.get $c)) (call $mj_after_pair))
            (local.get $path)))))
      (call $zjson_step)
      ;; key position after a comma
      (call $zjson_skip_ws (local.get $s))
      (local.set $c (call $zjson_peek (local.get $s)))
      (if (i32.eq (local.get $c) (i32.const -1))
        (then
          (return (call $zjson_fail (call $mj_unexp_end) (local.get $path)))))
      (if (i32.ne (local.get $c) (i32.const 34))
        (then
          (if (i32.eq (local.get $c) (i32.const 125))
            (then
              (return (call $zjson_fail
                (call $zhex_cat3 (call $mj_inv_char) (array.new_fixed $zhex_string 3 (i32.const 39) (i32.const 44) (i32.const 39)) (call $mj_looking_b))
                (local.get $path)))))
          (return (call $zjson_fail
            (call $zhex_cat3 (call $mj_inv_char) (call $mj_char_q (local.get $c)) (call $mj_looking_b))
            (local.get $path)))))
      (br $next)))
  (if (ref.is_null (local.get $members))
    (then (local.set $members (array.new_default $zhex_arr_ref (i32.const 0)))))
  (struct.new $zhex_result (i32.const 0)
    (struct.new $zjson_node (i32.const 5) (i32.const 0) (ref.null $zhex_string)
      (ref.null $zhex_arr_ref) (ref.as_non_null (local.get $members)))))

;; ---- node -> Json tree (jsonTreeValue): ordinals Null 0 .. Object 5 -------
(func $zjson_to_tree (param $n (ref $zjson_node)) (param $path (ref $zhex_string)) (result (ref $zhex_result))
  (local $kind i32) (local $n2 i32) (local $i i32)
  (local $items (ref null $zhex_arr_ref))
  (local $m (ref $zhex_map_str_ref))
  (local $r (ref $zhex_result))
  (local.set $kind (struct.get $zjson_node $kind (local.get $n)))
  (if (i32.eq (local.get $kind) (i32.const 0))
    (then (return (struct.new $zhex_result (i32.const 0) (struct.new $zt_Json_Null (i32.const 0))))))
  (if (i32.eq (local.get $kind) (i32.const 1))
    (then (return (struct.new $zhex_result (i32.const 0)
      (struct.new $zt_Json_Bool (i64.const 0) (i32.const 1) (struct.get $zjson_node $bool (local.get $n)))))))
  (if (i32.eq (local.get $kind) (i32.const 2))
    (then (return (struct.new $zhex_result (i32.const 0)
      (struct.new $zt_Json_Text (i64.const 0) (i64.const 0) (i32.const 2)
        (ref.as_non_null (struct.get $zjson_node $text (local.get $n))))))))
  (if (i32.eq (local.get $kind) (i32.const 3))
    (then (return (struct.new $zhex_result (i32.const 0)
      (struct.new $zt_Json_Number (i64.const 0) (i64.const 0) (i64.const 0) (i32.const 3)
        (ref.as_non_null (struct.get $zjson_node $text (local.get $n))))))))
  (if (i32.eq (local.get $kind) (i32.const 4))
    (then
      (local.set $items (ref.as_non_null (struct.get $zjson_node $items (local.get $n))))
      (local.set $n2 (array.len (ref.as_non_null (local.get $items))))
      (if (i32.gt_u (local.get $n2) (i32.const 1000000))
        (then (return (call $zjson_fail (call $mj_list_limit) (local.get $path)))))
      (block $built
        (loop $bl
          (br_if $built (i32.ge_u (local.get $i) (local.get $n2)))
          (local.set $r (call $zjson_to_tree
            (ref.cast (ref $zjson_node) (array.get $zhex_arr_ref (ref.as_non_null (local.get $items)) (local.get $i)))
            (call $zjson_path_index (local.get $path) (local.get $i))))
          (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
            (then (return (local.get $r))))
          (array.set $zhex_arr_ref (ref.as_non_null (local.get $items)) (local.get $i) (struct.get $zhex_result $payload (local.get $r)))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $bl)))
      (return (struct.new $zhex_result (i32.const 0)
        (struct.new $zt_Json_List (i64.const 0) (i64.const 0) (i64.const 0) (i64.const 0) (i32.const 4)
          (struct.new $zhex_list_ref (ref.as_non_null (local.get $items)) (local.get $n2) (i32.const 0)))))))
  (local.set $m (call $zhex_map_new_str_ref))
  (local.set $items (ref.as_non_null (struct.get $zjson_node $members (local.get $n))))
  (local.set $n2 (array.len (ref.as_non_null (local.get $items))))
  (block $built
    (loop $bl
      (br_if $built (i32.ge_u (local.get $i) (local.get $n2)))
      (local.set $r (call $zjson_to_tree
        (ref.as_non_null (struct.get $zjson_member $val
          (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $items)) (local.get $i)))))
        (call $zjson_path_key (local.get $path)
          (struct.get $zjson_member $key (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $items)) (local.get $i)))))))
      (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
        (then (return (local.get $r))))
      (drop (call $zhex_map_put_str_ref (local.get $m)
        (struct.get $zjson_member $key (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $items)) (local.get $i))))
        (struct.get $zhex_result $payload (local.get $r))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $bl)))
  (struct.new $zhex_result (i32.const 0)
    (struct.new $zt_Json_Object (i64.const 0) (i64.const 0) (i64.const 0) (i64.const 0) (i64.const 0) (i32.const 5)
      (local.get $m))))

;; json.parse: preamble, one value, the trailing check, tree conversion
(func $zhex_json_parse (param $raw (ref $zhex_string)) (result (ref $zhex_result))
  (local $c i32)
  (local $r (ref $zhex_result))
  (if (i32.eqz (call $zhex_json_surrogates_ok (local.get $raw)))
    (then (return (call $zjson_fail (call $mj_surrogate) (array.new_fixed $zhex_string 0)))))
  (global.set $zjson_i (i32.const 0))
  (local.set $r (call $zjson_parse_value (local.get $raw) (array.new_fixed $zhex_string 0) (i32.const 0) (i32.const 0)))
  (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
    (then (return (local.get $r))))
  (call $zjson_skip_ws (local.get $raw))
  (local.set $c (call $zjson_peek (local.get $raw)))
  (if (i32.eq (local.get $c) (i32.const -1))
    (then
      (return (call $zjson_to_tree (ref.cast (ref $zjson_node) (struct.get $zhex_result $payload (local.get $r)))
        (array.new_fixed $zhex_string 0)))))
  (if (i32.or (i32.or (i32.or (i32.eq (local.get $c) (i32.const 123)) (i32.eq (local.get $c) (i32.const 91))) (i32.eq (local.get $c) (i32.const 34)))
              (i32.or (i32.or (i32.eq (local.get $c) (i32.const 116)) (i32.eq (local.get $c) (i32.const 102)))
                      (i32.or (i32.or (i32.eq (local.get $c) (i32.const 110)) (i32.eq (local.get $c) (i32.const 45)))
                              (i32.and (i32.ge_u (local.get $c) (i32.const 48)) (i32.le_u (local.get $c) (i32.const 57))))))
    (then (return (call $zjson_fail (call $mj_multiple) (array.new_fixed $zhex_string 0)))))
  (call $zjson_fail
    (call $zhex_cat3 (call $mj_inv_char) (call $mj_char_q (local.get $c)) (call $mj_looking_b))
    (array.new_fixed $zhex_string 0)))
`

// watM3JsonEncode: compact separators, json.Marshal string escaping (the
// HTML escapes <, >, & and U+2028/U+2029 included, Surprises 5), the number
// grammar re-check on tree Number literals, and the depth limit.
const watM3JsonEncode = `
;; one encoded code point's width in the escaped output
(func $zjson_esc_len (param $cp i32) (result i32)
  (if (i32.or (i32.eq (local.get $cp) (i32.const 34)) (i32.eq (local.get $cp) (i32.const 92)))
    (then (return (i32.const 2))))
  (if (i32.lt_u (local.get $cp) (i32.const 32))
    (then
      (if (i32.or (i32.or (i32.or (i32.eq (local.get $cp) (i32.const 8)) (i32.eq (local.get $cp) (i32.const 9)))
                          (i32.or (i32.eq (local.get $cp) (i32.const 10)) (i32.eq (local.get $cp) (i32.const 12))))
                  (i32.eq (local.get $cp) (i32.const 13)))
        (then (return (i32.const 2))))
      (return (i32.const 6))))
  (if (i32.or (i32.or (i32.eq (local.get $cp) (i32.const 60)) (i32.eq (local.get $cp) (i32.const 62))) (i32.eq (local.get $cp) (i32.const 38)))
    (then (return (i32.const 6))))
  (if (i32.or (i32.eq (local.get $cp) (i32.const 8232)) (i32.eq (local.get $cp) (i32.const 8233)))
    (then (return (i32.const 6))))
  (i32.const 1))

(func $zjson_esc_write (param $r (ref $zhex_string)) (param $w i32) (param $cp i32) (result i32)
  ;; returns the next write position
  (if (i32.or (i32.eq (local.get $cp) (i32.const 34)) (i32.eq (local.get $cp) (i32.const 92)))
    (then
      (array.set $zhex_string (local.get $r) (local.get $w) (i32.const 92))
      (array.set $zhex_string (local.get $r) (i32.add (local.get $w) (i32.const 1)) (local.get $cp))
      (return (i32.add (local.get $w) (i32.const 2)))))
  (if (i32.lt_u (local.get $cp) (i32.const 32))
    (then
      (if (i32.or (i32.or (i32.or (i32.eq (local.get $cp) (i32.const 8)) (i32.eq (local.get $cp) (i32.const 9)))
                          (i32.or (i32.eq (local.get $cp) (i32.const 10)) (i32.eq (local.get $cp) (i32.const 12))))
                  (i32.eq (local.get $cp) (i32.const 13)))
        (then
          (array.set $zhex_string (local.get $r) (local.get $w) (i32.const 92))
          (array.set $zhex_string (local.get $r) (i32.add (local.get $w) (i32.const 1))
            (select (i32.const 116)
              (select (i32.const 110)
                (select (i32.const 114)
                  (select (i32.const 102) (i32.const 98) (i32.eq (local.get $cp) (i32.const 12)))
                  (i32.eq (local.get $cp) (i32.const 13)))
                (i32.eq (local.get $cp) (i32.const 10)))
              (i32.eq (local.get $cp) (i32.const 9))))
          (return (i32.add (local.get $w) (i32.const 2)))))))
  (if (i32.or (i32.or (i32.or (i32.eq (local.get $cp) (i32.const 60)) (i32.eq (local.get $cp) (i32.const 62))) (i32.eq (local.get $cp) (i32.const 38)))
              (i32.or (i32.eq (local.get $cp) (i32.const 8232)) (i32.eq (local.get $cp) (i32.const 8233))))
    (then
      (array.set $zhex_string (local.get $r) (local.get $w) (i32.const 92))
      (array.set $zhex_string (local.get $r) (i32.add (local.get $w) (i32.const 1)) (i32.const 117))
      (array.set $zhex_string (local.get $r) (i32.add (local.get $w) (i32.const 2)) (call $mj_hex_digit (i32.and (i32.shr_u (local.get $cp) (i32.const 12)) (i32.const 15))))
      (array.set $zhex_string (local.get $r) (i32.add (local.get $w) (i32.const 3)) (call $mj_hex_digit (i32.and (i32.shr_u (local.get $cp) (i32.const 8)) (i32.const 15))))
      (array.set $zhex_string (local.get $r) (i32.add (local.get $w) (i32.const 4)) (call $mj_hex_digit (i32.and (i32.shr_u (local.get $cp) (i32.const 4)) (i32.const 15))))
      (array.set $zhex_string (local.get $r) (i32.add (local.get $w) (i32.const 5)) (call $mj_hex_digit (i32.and (local.get $cp) (i32.const 15))))
      (return (i32.add (local.get $w) (i32.const 6)))))
  (if (i32.lt_u (local.get $cp) (i32.const 32))
    (then
      (array.set $zhex_string (local.get $r) (local.get $w) (i32.const 92))
      (array.set $zhex_string (local.get $r) (i32.add (local.get $w) (i32.const 1)) (i32.const 117))
      (array.set $zhex_string (local.get $r) (i32.add (local.get $w) (i32.const 2)) (i32.const 48))
      (array.set $zhex_string (local.get $r) (i32.add (local.get $w) (i32.const 3)) (i32.const 48))
      (array.set $zhex_string (local.get $r) (i32.add (local.get $w) (i32.const 4)) (call $mj_hex_digit (i32.shr_u (local.get $cp) (i32.const 4))))
      (array.set $zhex_string (local.get $r) (i32.add (local.get $w) (i32.const 5)) (call $mj_hex_digit (i32.and (local.get $cp) (i32.const 15))))
      (return (i32.add (local.get $w) (i32.const 6)))))
  (array.set $zhex_string (local.get $r) (local.get $w) (local.get $cp))
  (i32.add (local.get $w) (i32.const 1)))

;; json.Marshal string escaping, quotes included
(func $zjson_esc (param $t (ref $zhex_string)) (result (ref $zhex_string))
  (local $n i32) (local $i i32) (local $w i32) (local $len i32)
  (local $r (ref null $zhex_string))
  (local.set $n (array.len (local.get $t)))
  (block $summed
    (loop $sum
      (br_if $summed (i32.ge_u (local.get $i) (local.get $n)))
      (local.set $len (i32.add (local.get $len) (call $zjson_esc_len (array.get $zhex_string (local.get $t) (local.get $i)))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $sum)))
  (local.set $r (array.new_default $zhex_string (i32.add (local.get $len) (i32.const 2))))
  (array.set $zhex_string (ref.as_non_null (local.get $r)) (i32.const 0) (i32.const 34))
  (local.set $w (i32.const 1))
  (local.set $i (i32.const 0))
  (block $done
    (loop $each
      (br_if $done (i32.ge_u (local.get $i) (local.get $n)))
      (local.set $w (call $zjson_esc_write (ref.as_non_null (local.get $r)) (local.get $w)
        (array.get $zhex_string (local.get $t) (local.get $i))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $each)))
  (array.set $zhex_string (ref.as_non_null (local.get $r)) (local.get $w) (i32.const 34))
  (ref.as_non_null (local.get $r)))

;; the JSON number grammar (jsonNumberLiteral): -?(0|[1-9][0-9]*)(\.[0-9]+)?
;; ([eE][+-]?[0-9]+)?, the whole text
(func $zjson_lit_ok (param $t (ref $zhex_string)) (result i32)
  (local $n i32) (local $i i32)
  (local.set $n (array.len (local.get $t)))
  (if (i32.lt_u (local.get $i) (local.get $n))
    (then
      (if (i32.eq (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 45))
        (then (local.set $i (i32.add (local.get $i) (i32.const 1)))))))
  ;; integer part
  (if (i32.ge_u (local.get $i) (local.get $n)) (then (return (i32.const 0))))
  (if (i32.eq (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 48))
    (then (local.set $i (i32.add (local.get $i) (i32.const 1))))
    (else
      (if (i32.or (i32.lt_u (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 49))
                  (i32.gt_u (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 57)))
        (then (return (i32.const 0))))
      (block $id
        (loop $il
          (br_if $id (i32.ge_u (local.get $i) (local.get $n)))
          (br_if $id (i32.or (i32.lt_u (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 48))
                             (i32.gt_u (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 57))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $il)))))
  ;; fraction (guards nest: i32.and does not short-circuit)
  (if (i32.lt_u (local.get $i) (local.get $n))
    (then
      (if (i32.eq (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 46))
        (then
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (if (i32.ge_u (local.get $i) (local.get $n)) (then (return (i32.const 0))))
          (if (i32.or (i32.lt_u (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 48))
                      (i32.gt_u (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 57)))
            (then (return (i32.const 0))))
          (block $fd
        (loop $fl
          (br_if $fd (i32.ge_u (local.get $i) (local.get $n)))
          (br_if $fd (i32.or (i32.lt_u (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 48))
                             (i32.gt_u (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 57))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $fl)))))))
  ;; exponent
  (if (i32.lt_u (local.get $i) (local.get $n))
    (then
      (if (i32.or (i32.eq (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 101))
                  (i32.eq (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 69)))
        (then
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (if (i32.lt_u (local.get $i) (local.get $n))
            (then
              (if (i32.or (i32.eq (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 43))
                          (i32.eq (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 45)))
                (then (local.set $i (i32.add (local.get $i) (i32.const 1)))))))
          (if (i32.ge_u (local.get $i) (local.get $n)) (then (return (i32.const 0))))
      (if (i32.or (i32.lt_u (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 48))
                  (i32.gt_u (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 57)))
        (then (return (i32.const 0))))
      (block $ed
        (loop $el
          (br_if $ed (i32.ge_u (local.get $i) (local.get $n)))
          (br_if $ed (i32.or (i32.lt_u (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 48))
                             (i32.gt_u (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 57))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $el)))))))
  (i32.eq (local.get $i) (local.get $n)))

;; ---- the Json tree encoder ---------------------------------------------------
(func $zjson_enc_tree (param $v (ref null eq)) (param $path (ref $zhex_string)) (param $depth i32) (result (ref $zhex_result))
  (local $n i32) (local $i i32)
  (local $acc (ref null $zhex_string))
  (local $r (ref $zhex_result))
  (if (i32.gt_u (local.get $depth) (i32.const 256))
    (then (return (call $zjson_fail (call $mj_nesting) (local.get $path)))))
  (if (ref.test (ref $zt_Json_Null) (local.get $v))
    (then (return (struct.new $zhex_result (i32.const 0) (call $mj_null)))))
  (if (ref.test (ref $zt_Json_Bool) (local.get $v))
    (then
      (return (struct.new $zhex_result (i32.const 0)
        (if (result (ref $zhex_string))
          (struct.get $zt_Json_Bool $f0 (ref.cast (ref $zt_Json_Bool) (local.get $v)))
          (then (call $mj_true))
          (else (call $mj_false)))))))
  (if (ref.test (ref $zt_Json_Text) (local.get $v))
    (then
      (return (struct.new $zhex_result (i32.const 0)
        (call $zjson_esc (struct.get $zt_Json_Text $f0 (ref.cast (ref $zt_Json_Text) (local.get $v))))))))
  (if (ref.test (ref $zt_Json_Number) (local.get $v))
    (then
      (if (i32.eqz (call $zjson_lit_ok (struct.get $zt_Json_Number $f0 (ref.cast (ref $zt_Json_Number) (local.get $v)))))
        (then
          (return (call $zjson_fail
            (call $zhex_cat3 (call $mj_num_lit_pre)
              (call $zhex_quote (struct.get $zt_Json_Number $f0 (ref.cast (ref $zt_Json_Number) (local.get $v))))
              (call $mj_not_valid))
            (local.get $path)))))
      (return (struct.new $zhex_result (i32.const 0)
        (struct.get $zt_Json_Number $f0 (ref.cast (ref $zt_Json_Number) (local.get $v)))))))
  (if (ref.test (ref $zt_Json_List) (local.get $v))
    (then
      (local.set $acc (array.new_fixed $zhex_string 1 (i32.const 91)))
      (local.set $n (struct.get $zhex_list_ref $len
        (struct.get $zt_Json_List $f0 (ref.cast (ref $zt_Json_List) (local.get $v)))))
      (block $joined
        (loop $jl
          (br_if $joined (i32.ge_u (local.get $i) (local.get $n)))
          (if (i32.gt_u (local.get $i) (i32.const 0))
            (then (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 44))))))
          (local.set $r (call $zjson_enc_tree
            (array.get $zhex_arr_ref
              (ref.as_non_null
                (struct.get $zhex_list_ref $items
                  (struct.get $zt_Json_List $f0 (ref.cast (ref $zt_Json_List) (local.get $v)))))
              (local.get $i))
            (call $zjson_path_index (local.get $path) (local.get $i))
            (i32.add (local.get $depth) (i32.const 1))))
          (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
            (then (return (local.get $r))))
          (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc))
            (ref.cast (ref $zhex_string) (struct.get $zhex_result $payload (local.get $r)))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $jl)))
      (return (struct.new $zhex_result (i32.const 0)
        (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 93)))))))
  ;; Object
  (local.set $acc (array.new_fixed $zhex_string 1 (i32.const 123)))
  (local.set $n (struct.get $zhex_map_str_ref $len
    (struct.get $zt_Json_Object $f0 (ref.cast (ref $zt_Json_Object) (local.get $v)))))
  (block $joined
    (loop $jl
      (br_if $joined (i32.ge_u (local.get $i) (local.get $n)))
      (if (i32.gt_u (local.get $i) (i32.const 0))
        (then (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 44))))))
      (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc))
        (call $zjson_esc (array.get $zhex_arr_str
          (struct.get $zhex_map_str_ref $keys (struct.get $zt_Json_Object $f0 (ref.cast (ref $zt_Json_Object) (local.get $v))))
          (local.get $i)))))
      (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 58))))
      (local.set $r (call $zjson_enc_tree
        (array.get $zhex_arr_ref
          (struct.get $zhex_map_str_ref $vals (struct.get $zt_Json_Object $f0 (ref.cast (ref $zt_Json_Object) (local.get $v))))
          (local.get $i))
        (call $zjson_path_key (local.get $path)
          (array.get $zhex_arr_str
            (struct.get $zhex_map_str_ref $keys (struct.get $zt_Json_Object $f0 (ref.cast (ref $zt_Json_Object) (local.get $v))))
            (local.get $i)))
        (i32.add (local.get $depth) (i32.const 1))))
      (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
        (then (return (local.get $r))))
      (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc))
        (ref.cast (ref $zhex_string) (struct.get $zhex_result $payload (local.get $r)))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $jl)))
  (struct.new $zhex_result (i32.const 0)
    (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 125)))))

;; ---- the typed encoder: $v is an M1-boxed value, $s its schema --------------
(func $zjson_enc_typed (param $v (ref null eq)) (param $s (ref $zjson_schema)) (param $path (ref $zhex_string)) (param $depth i32) (result (ref $zhex_result))
  (local $kind i32) (local $n i32) (local $i i32)
  (local $acc (ref null $zhex_string))
  (local $r (ref $zhex_result))
  (local $elem (ref $zjson_schema))
  (local $fv (ref null $zhex_arr_ref))
  (local $i64s (ref null $zhex_arr_i64)) (local $i32s (ref null $zhex_arr_i32))
  (local $i64v (ref $zhex_map_str_i64)) (local $i32v (ref $zhex_map_str_i32)) (local $map (ref $zhex_map_str_ref))
  (if (i32.gt_u (local.get $depth) (i32.const 256))
    (then (return (call $zjson_fail (call $mj_nesting) (local.get $path)))))
  (local.set $kind (struct.get $zjson_schema $kind (local.get $s)))
  (if (i32.eq (local.get $kind) (i32.const 0))
    (then
      (return (struct.new $zhex_result (i32.const 0)
        (call $zhex_int_to_string (struct.get $zhex_some_i64 $value (ref.cast (ref $zhex_some_i64) (local.get $v))))))))
  (if (i32.eq (local.get $kind) (i32.const 1))
    (then
      (return (struct.new $zhex_result (i32.const 0)
        (if (result (ref $zhex_string))
          (i31.get_u (ref.cast (ref i31) (local.get $v)))
          (then (call $mj_true))
          (else (call $mj_false)))))))
  (if (i32.eq (local.get $kind) (i32.const 2))
    (then
      (return (struct.new $zhex_result (i32.const 0)
        (call $zjson_esc (ref.cast (ref $zhex_string) (local.get $v)))))))
  (if (i32.eq (local.get $kind) (i32.const 3))
    (then
      (if (ref.test (ref $zhex_none) (local.get $v))
        (then (return (struct.new $zhex_result (i32.const 0) (call $mj_null)))))
      (return (call $zjson_enc_typed (struct.get $zhex_some_ref $value (ref.cast (ref $zhex_some_ref) (local.get $v)))
        (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))
        (local.get $path) (i32.add (local.get $depth) (i32.const 1))))))
  ;; lists: the element repr picks the flavor (the emitter's record thunks
  ;; cast the specialized structs); every arm returns
  (if (i32.eq (local.get $kind) (i32.const 4))
    (then
      (local.set $elem (ref.as_non_null (struct.get $zjson_schema $element (local.get $s))))
      (local.set $acc (array.new_fixed $zhex_string 1 (i32.const 91)))
      (if (i32.eq (struct.get $zjson_schema $kind (local.get $elem)) (i32.const 0))
        (then
          (local.set $n (struct.get $zhex_list_i64 $len (ref.cast (ref $zhex_list_i64) (local.get $v))))
          (local.set $i64s (struct.get $zhex_list_i64 $items (ref.cast (ref $zhex_list_i64) (local.get $v))))
          (block $joined
            (loop $jl
              (br_if $joined (i32.ge_u (local.get $i) (local.get $n)))
              (if (i32.gt_u (local.get $i) (i32.const 0))
                (then (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 44))))))
              (local.set $r (call $zjson_enc_typed
                (struct.new $zhex_some_i64 (array.get $zhex_arr_i64 (ref.as_non_null (local.get $i64s)) (local.get $i)))
                (local.get $elem)
                (call $zjson_path_index (local.get $path) (local.get $i))
                (i32.add (local.get $depth) (i32.const 1))))
              (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
                (then (return (local.get $r))))
              (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc))
                (ref.cast (ref $zhex_string) (struct.get $zhex_result $payload (local.get $r)))))
              (local.set $i (i32.add (local.get $i) (i32.const 1)))
              (br $jl)))
          (return (struct.new $zhex_result (i32.const 0)
            (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 93)))))))
      (if (i32.eq (struct.get $zjson_schema $kind (local.get $elem)) (i32.const 1))
        (then
          (local.set $n (struct.get $zhex_list_i32 $len (ref.cast (ref $zhex_list_i32) (local.get $v))))
          (local.set $i32s (struct.get $zhex_list_i32 $items (ref.cast (ref $zhex_list_i32) (local.get $v))))
          (block $joined
            (loop $jl
              (br_if $joined (i32.ge_u (local.get $i) (local.get $n)))
              (if (i32.gt_u (local.get $i) (i32.const 0))
                (then (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 44))))))
              (local.set $r (call $zjson_enc_typed
                (ref.i31 (array.get $zhex_arr_i32 (ref.as_non_null (local.get $i32s)) (local.get $i)))
                (local.get $elem)
                (call $zjson_path_index (local.get $path) (local.get $i))
                (i32.add (local.get $depth) (i32.const 1))))
              (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
                (then (return (local.get $r))))
              (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc))
                (ref.cast (ref $zhex_string) (struct.get $zhex_result $payload (local.get $r)))))
              (local.set $i (i32.add (local.get $i) (i32.const 1)))
              (br $jl)))
          (return (struct.new $zhex_result (i32.const 0)
            (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 93)))))))
      (local.set $n (struct.get $zhex_list_ref $len (ref.cast (ref $zhex_list_ref) (local.get $v))))
      (block $joined
        (loop $jl
          (br_if $joined (i32.ge_u (local.get $i) (local.get $n)))
          (if (i32.gt_u (local.get $i) (i32.const 0))
            (then (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 44))))))
          (local.set $r (call $zjson_enc_typed
            (array.get $zhex_arr_ref (ref.as_non_null (struct.get $zhex_list_ref $items (ref.cast (ref $zhex_list_ref) (local.get $v)))) (local.get $i))
            (local.get $elem)
            (call $zjson_path_index (local.get $path) (local.get $i))
            (i32.add (local.get $depth) (i32.const 1))))
          (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
            (then (return (local.get $r))))
          (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc))
            (ref.cast (ref $zhex_string) (struct.get $zhex_result $payload (local.get $r)))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $jl)))
      (return (struct.new $zhex_result (i32.const 0)
        (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 93)))))))
  ;; maps: the same three flavors, keys escaped, values boxed per repr
  (if (i32.eq (local.get $kind) (i32.const 5))
    (then
      (local.set $elem (ref.as_non_null (struct.get $zjson_schema $element (local.get $s))))
      (local.set $acc (array.new_fixed $zhex_string 1 (i32.const 123)))
      (if (i32.eq (struct.get $zjson_schema $kind (local.get $elem)) (i32.const 0))
        (then
          (local.set $i64v (ref.cast (ref $zhex_map_str_i64) (local.get $v)))
          (local.set $n (struct.get $zhex_map_str_i64 $len (local.get $i64v)))
          (block $joined
            (loop $jl
              (br_if $joined (i32.ge_u (local.get $i) (local.get $n)))
              (if (i32.gt_u (local.get $i) (i32.const 0))
                (then (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 44))))))
              (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc))
                (call $zjson_esc (array.get $zhex_arr_str (struct.get $zhex_map_str_i64 $keys (local.get $i64v)) (local.get $i)))))
              (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 58))))
              (local.set $r (call $zjson_enc_typed
                (struct.new $zhex_some_i64 (array.get $zhex_arr_i64 (struct.get $zhex_map_str_i64 $vals (local.get $i64v)) (local.get $i)))
                (local.get $elem)
                (call $zjson_path_key (local.get $path) (array.get $zhex_arr_str (struct.get $zhex_map_str_i64 $keys (local.get $i64v)) (local.get $i)))
                (i32.add (local.get $depth) (i32.const 1))))
              (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
                (then (return (local.get $r))))
              (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc))
                (ref.cast (ref $zhex_string) (struct.get $zhex_result $payload (local.get $r)))))
              (local.set $i (i32.add (local.get $i) (i32.const 1)))
              (br $jl)))
          (return (struct.new $zhex_result (i32.const 0)
            (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 125)))))))
      (if (i32.eq (struct.get $zjson_schema $kind (local.get $elem)) (i32.const 1))
        (then
          (local.set $i32v (ref.cast (ref $zhex_map_str_i32) (local.get $v)))
          (local.set $n (struct.get $zhex_map_str_i32 $len (local.get $i32v)))
          (block $joined
            (loop $jl
              (br_if $joined (i32.ge_u (local.get $i) (local.get $n)))
              (if (i32.gt_u (local.get $i) (i32.const 0))
                (then (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 44))))))
              (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc))
                (call $zjson_esc (array.get $zhex_arr_str (struct.get $zhex_map_str_i32 $keys (local.get $i32v)) (local.get $i)))))
              (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 58))))
              (local.set $r (call $zjson_enc_typed
                (ref.i31 (array.get $zhex_arr_i32 (struct.get $zhex_map_str_i32 $vals (local.get $i32v)) (local.get $i)))
                (local.get $elem)
                (call $zjson_path_key (local.get $path) (array.get $zhex_arr_str (struct.get $zhex_map_str_i32 $keys (local.get $i32v)) (local.get $i)))
                (i32.add (local.get $depth) (i32.const 1))))
              (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
                (then (return (local.get $r))))
              (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc))
                (ref.cast (ref $zhex_string) (struct.get $zhex_result $payload (local.get $r)))))
              (local.set $i (i32.add (local.get $i) (i32.const 1)))
              (br $jl)))
          (return (struct.new $zhex_result (i32.const 0)
            (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 125)))))))
      (local.set $map (ref.cast (ref $zhex_map_str_ref) (local.get $v)))
      (local.set $n (struct.get $zhex_map_str_ref $len (local.get $map)))
      (block $joined
        (loop $jl
          (br_if $joined (i32.ge_u (local.get $i) (local.get $n)))
          (if (i32.gt_u (local.get $i) (i32.const 0))
            (then (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 44))))))
          (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc))
            (call $zjson_esc (array.get $zhex_arr_str (struct.get $zhex_map_str_ref $keys (local.get $map)) (local.get $i)))))
          (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 58))))
          (local.set $r (call $zjson_enc_typed
            (array.get $zhex_arr_ref (struct.get $zhex_map_str_ref $vals (local.get $map)) (local.get $i))
            (local.get $elem)
            (call $zjson_path_key (local.get $path) (array.get $zhex_arr_str (struct.get $zhex_map_str_ref $keys (local.get $map)) (local.get $i)))
            (i32.add (local.get $depth) (i32.const 1))))
          (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
            (then (return (local.get $r))))
          (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc))
            (ref.cast (ref $zhex_string) (struct.get $zhex_result $payload (local.get $r)))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $jl)))
      (return (struct.new $zhex_result (i32.const 0)
        (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 125)))))))
  ;; record: the get thunk reads the fields array; names come from the schema
  (local.set $acc (array.new_fixed $zhex_string 1 (i32.const 123)))
  (local.set $n (array.len (ref.as_non_null (struct.get $zjson_schema $fields (local.get $s)))))
  (local.set $fv
    (call_ref $zjson_get_fn
      (local.get $v)
      (ref.cast (ref $zjson_get_fn) (struct.get $zjson_schema $get (local.get $s)))))
  (block $joined
    (loop $jl
      (br_if $joined (i32.ge_u (local.get $i) (local.get $n)))
      (if (i32.gt_u (local.get $i) (i32.const 0))
        (then (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 44))))))
      (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc))
        (call $zjson_esc (struct.get $zjson_field $name
          (ref.cast (ref $zjson_field)
            (array.get $zhex_arr_ref (ref.as_non_null (struct.get $zjson_schema $fields (local.get $s))) (local.get $i)))))))
      (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 58))))
      (local.set $r (call $zjson_enc_typed
        (array.get $zhex_arr_ref (ref.as_non_null (local.get $fv)) (local.get $i))
        (ref.as_non_null (struct.get $zjson_field $schema
          (ref.cast (ref $zjson_field)
            (array.get $zhex_arr_ref (ref.as_non_null (struct.get $zjson_schema $fields (local.get $s))) (local.get $i)))))
        (call $zjson_path_key (local.get $path)
          (struct.get $zjson_field $name
            (ref.cast (ref $zjson_field)
              (array.get $zhex_arr_ref (ref.as_non_null (struct.get $zjson_schema $fields (local.get $s))) (local.get $i)))))
        (i32.add (local.get $depth) (i32.const 1))))
      (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
        (then (return (local.get $r))))
      (local.set $acc (call $zhex_str_concat (ref.as_non_null (local.get $acc))
        (ref.cast (ref $zhex_string) (struct.get $zhex_result $payload (local.get $r)))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $jl)))
  (struct.new $zhex_result (i32.const 0)
    (call $zhex_str_concat (ref.as_non_null (local.get $acc)) (array.new_fixed $zhex_string 1 (i32.const 125)))))

;; json.encode: tree path when the schema is null (the Json tree), typed
;; path otherwise (the value arrives M1-boxed)
(func $zhex_json_encode (param $v (ref null eq)) (param $s (ref null $zjson_schema)) (result (ref $zhex_result))
  (if (ref.is_null (local.get $s))
    (then (return (call $zjson_enc_tree (local.get $v) (array.new_fixed $zhex_string 0) (i32.const 0)))))
  (call $zjson_enc_typed (local.get $v) (ref.as_non_null (local.get $s)) (array.new_fixed $zhex_string 0) (i32.const 0)))
`

// watM3JsonDecode: parse once, then schema-walk with the defaults ref and a
// linear ref.eq memo for aliasing-preserving default fills (§6).
const watM3JsonDecode = `
;; exactJSONInt: integer-form signed 64-bit only; null payload = invalid
(func $zjson_exact_int (param $t (ref $zhex_string)) (result (ref null eq))
  (local $n i32) (local $i i32) (local $neg i32) (local $c i32) (local $v i64)
  (local.set $n (array.len (local.get $t)))
  ;; nested ifs, not i32.and: and evaluates both arms, so an empty text
  ;; would read index 0 of a zero-length string and trap
  (if (i32.gt_u (local.get $n) (i32.const 0))
    (then
      (if (i32.eq (array.get $zhex_string (local.get $t) (i32.const 0)) (i32.const 45))
        (then
          (local.set $neg (i32.const 1))
          (local.set $i (i32.const 1))))))
  (block $digits
    (loop $dl
      (br_if $digits (i32.ge_u (local.get $i) (local.get $n)))
      (local.set $c (array.get $zhex_string (local.get $t) (local.get $i)))
      (if (i32.or (i32.lt_u (local.get $c) (i32.const 48)) (i32.gt_u (local.get $c) (i32.const 57)))
        (then (return (ref.null eq))))
      (if (i64.gt_u (local.get $v) (i64.const 922337203685477580))
        (then (return (ref.null eq))))
      (local.set $v (i64.add (i64.mul (local.get $v) (i64.const 10)) (i64.extend_i32_u (i32.sub (local.get $c) (i32.const 48)))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $dl)))
  (if (i32.eq (local.get $i) (i32.const 0)) (then (return (ref.null eq))))
  (if (local.get $neg)
    (then (return (struct.new $zhex_some_i64 (i64.sub (i64.const 0) (local.get $v))))))
  (if (i64.gt_u (local.get $v) (i64.const 9223372036854775807))
    (then (return (ref.null eq))))
  (struct.new $zhex_some_i64 (local.get $v)))

;; memo find: linear scan, ref.eq identity
(func $zjson_memo_find (param $m (ref $zjson_memo)) (param $v (ref null eq)) (result (ref null eq))
  (local $srcs (ref null $zhex_arr_ref)) (local $n i32) (local $i i32)
  (local.set $srcs (struct.get $zjson_memo $srcs (local.get $m)))
  (if (ref.is_null (local.get $srcs))
    (then (return (ref.null eq))))
  (local.set $n (array.len (ref.as_non_null (local.get $srcs))))
  (block $done
    (loop $scan
      (br_if $done (i32.ge_u (local.get $i) (local.get $n)))
      (if (ref.eq (array.get $zhex_arr_ref (ref.as_non_null (local.get $srcs)) (local.get $i)) (local.get $v))
        (then
          (return (array.get $zhex_arr_ref (ref.as_non_null (struct.get $zjson_memo $dsts (local.get $m))) (local.get $i)))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $scan)))
  (ref.null eq))

(func $zjson_memo_push (param $m (ref $zjson_memo)) (param $src (ref null eq)) (param $dst (ref null eq))
  (struct.set $zjson_memo $srcs (local.get $m)
    (call $zjson_arr_push (struct.get $zjson_memo $srcs (local.get $m)) (local.get $src)))
  (struct.set $zjson_memo $dsts (local.get $m)
    (call $zjson_arr_push (struct.get $zjson_memo $dsts (local.get $m)) (local.get $dst))))

;; schema-directed deep clone with the identity memo (the interpreter's
;; jsonClone over defaults; kinds without interior mutability pass through).
;; The value flavors (List<Int>, List<Bool>, Map<String, Int>,
;; Map<String, Bool>, Option of a ref) clone too — an absent field's default
;; must not alias the defaults record's list/map wrapper, which append
;; mutates.
(func $zjson_clone (param $v (ref null eq)) (param $s (ref $zjson_schema)) (param $m (ref $zjson_memo)) (result (ref null eq))
  (local $kind i32) (local $elkind i32) (local $n i32) (local $i i32)
  (local $items (ref null $zhex_arr_ref))
  (local $fresh (ref null $zhex_arr_ref))
  (local $f64 (ref null $zhex_arr_i64))
  (local $f32 (ref null $zhex_arr_i32))
  (local $m64 (ref null $zhex_map_str_i64))
  (local $m32 (ref null $zhex_map_str_i32))
  (local $map (ref $zhex_map_str_ref))
  (local $r (ref null eq))
  (local.set $kind (struct.get $zjson_schema $kind (local.get $s)))
  (if (i32.eq (local.get $kind) (i32.const 3))
    (then
      ;; Some of a ref payload clones through the element schema; Int/Bool
      ;; payloads and None share nothing mutable
      (if (ref.test (ref $zhex_some_ref) (local.get $v))
        (then
          (return (struct.new $zhex_some_ref
            (call $zjson_clone (struct.get $zhex_some_ref $value (ref.cast (ref $zhex_some_ref) (local.get $v)))
              (ref.as_non_null (struct.get $zjson_schema $element (local.get $s))) (local.get $m))))))
      (return (local.get $v))))
  (if (i32.eq (local.get $kind) (i32.const 4))
    (then
      (local.set $elkind (struct.get $zjson_schema $kind (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))))
      (if (i32.eq (local.get $elkind) (i32.const 0))
        (then
          (if (i32.eqz (ref.test (ref $zhex_list_i64) (local.get $v))) (then (return (local.get $v))))
          (local.set $r (call $zjson_memo_find (local.get $m) (local.get $v)))
          (if (i32.eqz (ref.is_null (local.get $r))) (then (return (local.get $r))))
          (local.set $n (struct.get $zhex_list_i64 $len (ref.cast (ref $zhex_list_i64) (local.get $v))))
          (local.set $f64 (array.new_default $zhex_arr_i64 (local.get $n)))
          (block $cp (loop $cl
            (br_if $cp (i32.ge_u (local.get $i) (local.get $n)))
            (array.set $zhex_arr_i64 (ref.as_non_null (local.get $f64)) (local.get $i)
              (array.get $zhex_arr_i64 (ref.as_non_null (struct.get $zhex_list_i64 $items (ref.cast (ref $zhex_list_i64) (local.get $v)))) (local.get $i)))
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (br $cl)))
          (local.set $r (struct.new $zhex_list_i64 (ref.as_non_null (local.get $f64)) (local.get $n) (i32.const 0)))
          (call $zjson_memo_push (local.get $m) (local.get $v) (local.get $r))
          (return (local.get $r))))
      (if (i32.eq (local.get $elkind) (i32.const 1))
        (then
          (if (i32.eqz (ref.test (ref $zhex_list_i32) (local.get $v))) (then (return (local.get $v))))
          (local.set $r (call $zjson_memo_find (local.get $m) (local.get $v)))
          (if (i32.eqz (ref.is_null (local.get $r))) (then (return (local.get $r))))
          (local.set $n (struct.get $zhex_list_i32 $len (ref.cast (ref $zhex_list_i32) (local.get $v))))
          (local.set $f32 (array.new_default $zhex_arr_i32 (local.get $n)))
          (block $cp (loop $cl
            (br_if $cp (i32.ge_u (local.get $i) (local.get $n)))
            (array.set $zhex_arr_i32 (ref.as_non_null (local.get $f32)) (local.get $i)
              (array.get $zhex_arr_i32 (ref.as_non_null (struct.get $zhex_list_i32 $items (ref.cast (ref $zhex_list_i32) (local.get $v)))) (local.get $i)))
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (br $cl)))
          (local.set $r (struct.new $zhex_list_i32 (ref.as_non_null (local.get $f32)) (local.get $n) (i32.const 0)))
          (call $zjson_memo_push (local.get $m) (local.get $v) (local.get $r))
          (return (local.get $r))))))
  (if (i32.eq (local.get $kind) (i32.const 5))
    (then
      (local.set $elkind (struct.get $zjson_schema $kind (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))))
      (if (i32.eq (local.get $elkind) (i32.const 0))
        (then
          (if (i32.eqz (ref.test (ref $zhex_map_str_i64) (local.get $v))) (then (return (local.get $v))))
          (local.set $r (call $zjson_memo_find (local.get $m) (local.get $v)))
          (if (i32.eqz (ref.is_null (local.get $r))) (then (return (local.get $r))))
          (local.set $m64 (call $zhex_map_new_str_i64))
          (local.set $r (local.get $m64))
          (call $zjson_memo_push (local.get $m) (local.get $v) (local.get $r))
          (local.set $n (struct.get $zhex_map_str_i64 $len (ref.cast (ref $zhex_map_str_i64) (local.get $v))))
          (block $cp (loop $cl
            (br_if $cp (i32.ge_u (local.get $i) (local.get $n)))
            (drop (call $zhex_map_put_str_i64 (ref.as_non_null (local.get $m64))
              (array.get $zhex_arr_str (struct.get $zhex_map_str_i64 $keys (ref.cast (ref $zhex_map_str_i64) (local.get $v))) (local.get $i))
              (array.get $zhex_arr_i64 (struct.get $zhex_map_str_i64 $vals (ref.cast (ref $zhex_map_str_i64) (local.get $v))) (local.get $i))))
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (br $cl)))
          (return (local.get $r))))
      (if (i32.eq (local.get $elkind) (i32.const 1))
        (then
          (if (i32.eqz (ref.test (ref $zhex_map_str_i32) (local.get $v))) (then (return (local.get $v))))
          (local.set $r (call $zjson_memo_find (local.get $m) (local.get $v)))
          (if (i32.eqz (ref.is_null (local.get $r))) (then (return (local.get $r))))
          (local.set $m32 (call $zhex_map_new_str_i32))
          (local.set $r (local.get $m32))
          (call $zjson_memo_push (local.get $m) (local.get $v) (local.get $r))
          (local.set $n (struct.get $zhex_map_str_i32 $len (ref.cast (ref $zhex_map_str_i32) (local.get $v))))
          (block $cp (loop $cl
            (br_if $cp (i32.ge_u (local.get $i) (local.get $n)))
            (drop (call $zhex_map_put_str_i32 (ref.as_non_null (local.get $m32))
              (array.get $zhex_arr_str (struct.get $zhex_map_str_i32 $keys (ref.cast (ref $zhex_map_str_i32) (local.get $v))) (local.get $i))
              (array.get $zhex_arr_i32 (struct.get $zhex_map_str_i32 $vals (ref.cast (ref $zhex_map_str_i32) (local.get $v))) (local.get $i))))
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (br $cl)))
          (return (local.get $r))))))
  (if (i32.eq (local.get $kind) (i32.const 4))
    (then
      (if (i32.eqz (ref.test (ref $zhex_list_ref) (local.get $v))) (then (return (local.get $v))))
      (local.set $r (call $zjson_memo_find (local.get $m) (local.get $v)))
      (if (i32.eqz (ref.is_null (local.get $r))) (then (return (local.get $r))))
      (local.set $items (struct.get $zhex_list_ref $items (ref.cast (ref $zhex_list_ref) (local.get $v))))
      (local.set $n (struct.get $zhex_list_ref $len (ref.cast (ref $zhex_list_ref) (local.get $v))))
      (local.set $fresh (array.new_default $zhex_arr_ref (local.get $n)))
      (local.set $r (struct.new $zhex_list_ref (ref.as_non_null (local.get $fresh)) (local.get $n) (i32.const 0)))
      (call $zjson_memo_push (local.get $m) (local.get $v) (local.get $r))
      (block $cl
        (loop $clp
          (br_if $cl (i32.ge_u (local.get $i) (local.get $n)))
          (array.set $zhex_arr_ref (ref.as_non_null (local.get $fresh)) (local.get $i)
            (call $zjson_clone (array.get $zhex_arr_ref (ref.as_non_null (local.get $items)) (local.get $i))
              (ref.as_non_null (struct.get $zjson_schema $element (local.get $s))) (local.get $m)))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $clp)))
      (return (local.get $r))))
  (if (i32.eq (local.get $kind) (i32.const 5))
    (then
      (if (i32.eqz (ref.test (ref $zhex_map_str_ref) (local.get $v))) (then (return (local.get $v))))
      (local.set $r (call $zjson_memo_find (local.get $m) (local.get $v)))
      (if (i32.eqz (ref.is_null (local.get $r))) (then (return (local.get $r))))
      (local.set $map (call $zhex_map_new_str_ref))
      (local.set $r (local.get $map))
      (call $zjson_memo_push (local.get $m) (local.get $v) (local.get $r))
      (local.set $n (struct.get $zhex_map_str_ref $len (ref.cast (ref $zhex_map_str_ref) (local.get $v))))
      (block $cl
        (loop $clp
          (br_if $cl (i32.ge_u (local.get $i) (local.get $n)))
          (drop (call $zhex_map_put_str_ref (ref.cast (ref $zhex_map_str_ref) (local.get $r))
            (array.get $zhex_arr_str (struct.get $zhex_map_str_ref $keys (ref.cast (ref $zhex_map_str_ref) (local.get $v))) (local.get $i))
            (call $zjson_clone
              (array.get $zhex_arr_ref (struct.get $zhex_map_str_ref $vals (ref.cast (ref $zhex_map_str_ref) (local.get $v))) (local.get $i))
              (ref.as_non_null (struct.get $zjson_schema $element (local.get $s))) (local.get $m))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $clp)))
      (return (local.get $r))))
  (if (i32.eq (local.get $kind) (i32.const 6))
    (then
      (local.set $r (call $zjson_memo_find (local.get $m) (local.get $v)))
      (if (i32.eqz (ref.is_null (local.get $r))) (then (return (local.get $r))))
      (local.set $fresh
        (call_ref $zjson_get_fn
          (local.get $v)
          (ref.cast (ref $zjson_get_fn) (struct.get $zjson_schema $get (local.get $s)))))
      (local.set $n (array.len (ref.as_non_null (local.get $fresh))))
      (local.set $items (array.new_default $zhex_arr_ref (local.get $n)))
      (local.set $r (call_ref $zjson_make_fn
        (ref.as_non_null (local.get $items))
        (ref.cast (ref $zjson_make_fn) (struct.get $zjson_schema $make (local.get $s)))))
      (call $zjson_memo_push (local.get $m) (local.get $v) (local.get $r))
      (block $cl
        (loop $clp
          (br_if $cl (i32.ge_u (local.get $i) (local.get $n)))
          (array.set $zhex_arr_ref (ref.as_non_null (local.get $items)) (local.get $i)
            (call $zjson_clone (array.get $zhex_arr_ref (ref.as_non_null (local.get $fresh)) (local.get $i))
              (ref.as_non_null (struct.get $zjson_field $schema
                (ref.cast (ref $zjson_field)
                  (array.get $zhex_arr_ref (ref.as_non_null (struct.get $zjson_schema $fields (local.get $s))) (local.get $i)))))
              (local.get $m)))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $clp)))
      (return (local.get $r))))
  (local.get $v))

;; the schema walk (decodeJSONValue): $def is the boxed defaults value; the
;; element repr picks the list/map flavor the emitter's record thunks cast
(func $zjson_decode_value (param $n (ref $zjson_node)) (param $s (ref $zjson_schema)) (param $def (ref null eq)) (param $m (ref $zjson_memo)) (param $path (ref $zhex_string)) (result (ref $zhex_result))
  (local $kind i32) (local $nkind i32) (local $count i32) (local $mcount i32) (local $i i32) (local $j i32) (local $found i32)
  (local $i64s (ref null $zhex_arr_i64)) (local $i32s (ref null $zhex_arr_i32)) (local $i64v (ref null $zhex_map_str_i64)) (local $i32v (ref null $zhex_map_str_i32))
  (local $items (ref null $zhex_arr_ref))
  (local $fresh (ref null $zhex_arr_ref))
  (local $members (ref null $zhex_arr_ref))
  (local $fields (ref null $zhex_arr_ref))
  (local $deff (ref null $zhex_arr_ref))
  (local $map (ref $zhex_map_str_ref))
  (local $key (ref $zhex_string))
  (local $r (ref $zhex_result))
  (local.set $kind (struct.get $zjson_schema $kind (local.get $s)))
  (local.set $nkind (struct.get $zjson_node $kind (local.get $n)))
  ;; Option: null -> None, else decode the inner node with the unwrapped
  ;; default (defaults arrive as Some)
  (if (i32.eq (local.get $kind) (i32.const 3))
    (then
      (if (i32.eq (local.get $nkind) (i32.const 0))
        (then (return (struct.new $zhex_result (i32.const 0) (struct.new $zhex_none)))))
      (return (struct.new $zhex_result (i32.const 0)
        (struct.new $zhex_some_ref
          (struct.get $zhex_result $payload
            (call $zjson_decode_value (local.get $n)
              (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))
              (if (result (ref null eq))
                (ref.test (ref $zhex_some_ref) (local.get $def))
                (then (struct.get $zhex_some_ref $value (ref.cast (ref $zhex_some_ref) (local.get $def))))
                (else (ref.null eq)))
              (local.get $m) (local.get $path))))))))
  (if (i32.eq (local.get $kind) (i32.const 0))
    (then
      (if (i32.ne (local.get $nkind) (i32.const 3))
        (then (return (call $zjson_fail (call $mj_exp_int) (local.get $path)))))
      (local.set $r (struct.new $zhex_result (i32.const 0)
        (call $zjson_exact_int (ref.as_non_null (struct.get $zjson_node $text (local.get $n))))))
      (if (i32.eqz (ref.is_null (struct.get $zhex_result $payload (local.get $r))))
        (then (return (local.get $r))))
      (return (call $zjson_fail (call $mj_exp_intform) (local.get $path)))))
  (if (i32.eq (local.get $kind) (i32.const 1))
    (then
      (if (i32.ne (local.get $nkind) (i32.const 1))
        (then (return (call $zjson_fail (call $mj_exp_bool) (local.get $path)))))
      (return (struct.new $zhex_result (i32.const 0)
        (ref.i31 (struct.get $zjson_node $bool (local.get $n)))))))
  (if (i32.eq (local.get $kind) (i32.const 2))
    (then
      (if (i32.ne (local.get $nkind) (i32.const 2))
        (then (return (call $zjson_fail (call $mj_exp_str) (local.get $path)))))
      (return (struct.new $zhex_result (i32.const 0)
        (ref.as_non_null (struct.get $zjson_node $text (local.get $n)))))))
  (if (i32.eq (local.get $kind) (i32.const 4))
    (then
      (if (i32.ne (local.get $nkind) (i32.const 4))
        (then (return (call $zjson_fail (call $mj_exp_arr) (local.get $path)))))
      (local.set $items (ref.as_non_null (struct.get $zjson_node $items (local.get $n))))
      (local.set $count (array.len (ref.as_non_null (local.get $items))))
      (if (i32.gt_u (local.get $count) (i32.const 1000000))
        (then (return (call $zjson_fail (call $mj_list_limit) (local.get $path)))))
      (if (i32.eq (struct.get $zjson_schema $kind (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))) (i32.const 0))
        (then
          (local.set $i64s (array.new_default $zhex_arr_i64 (local.get $count)))
          (block $dec
            (loop $dl
              (br_if $dec (i32.ge_u (local.get $i) (local.get $count)))
              (local.set $r (call $zjson_decode_value
                (ref.cast (ref $zjson_node) (array.get $zhex_arr_ref (ref.as_non_null (local.get $items)) (local.get $i)))
                (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))
                (ref.null eq) (local.get $m)
                (call $zjson_path_index (local.get $path) (local.get $i))))
              (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
                (then (return (local.get $r))))
              (array.set $zhex_arr_i64 (ref.as_non_null (local.get $i64s)) (local.get $i)
                (struct.get $zhex_some_i64 $value
                  (ref.cast (ref $zhex_some_i64) (struct.get $zhex_result $payload (local.get $r)))))
              (local.set $i (i32.add (local.get $i) (i32.const 1)))
              (br $dl)))
          (return (struct.new $zhex_result (i32.const 0)
            (struct.new $zhex_list_i64 (ref.as_non_null (local.get $i64s)) (local.get $count) (i32.const 0)))))
        (else
          (if (i32.eq (struct.get $zjson_schema $kind (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))) (i32.const 1))
            (then
              (local.set $i32s (array.new_default $zhex_arr_i32 (local.get $count)))
              (block $dec
                (loop $dl
                  (br_if $dec (i32.ge_u (local.get $i) (local.get $count)))
                  (local.set $r (call $zjson_decode_value
                    (ref.cast (ref $zjson_node) (array.get $zhex_arr_ref (ref.as_non_null (local.get $items)) (local.get $i)))
                    (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))
                    (ref.null eq) (local.get $m)
                    (call $zjson_path_index (local.get $path) (local.get $i))))
                  (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
                    (then (return (local.get $r))))
                  (array.set $zhex_arr_i32 (ref.as_non_null (local.get $i32s)) (local.get $i)
                    (i31.get_u (ref.cast (ref i31) (struct.get $zhex_result $payload (local.get $r)))))
                  (local.set $i (i32.add (local.get $i) (i32.const 1)))
                  (br $dl)))
              (return (struct.new $zhex_result (i32.const 0)
                (struct.new $zhex_list_i32 (ref.as_non_null (local.get $i32s)) (local.get $count) (i32.const 0)))))
            (else
              (local.set $fresh (array.new_default $zhex_arr_ref (local.get $count)))
              (block $dec
                (loop $dl
                  (br_if $dec (i32.ge_u (local.get $i) (local.get $count)))
                  (local.set $r (call $zjson_decode_value
                    (ref.cast (ref $zjson_node) (array.get $zhex_arr_ref (ref.as_non_null (local.get $items)) (local.get $i)))
                    (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))
                    (ref.null eq) (local.get $m)
                    (call $zjson_path_index (local.get $path) (local.get $i))))
                  (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
                    (then (return (local.get $r))))
                  (array.set $zhex_arr_ref (ref.as_non_null (local.get $fresh)) (local.get $i)
                    (struct.get $zhex_result $payload (local.get $r)))
                  (local.set $i (i32.add (local.get $i) (i32.const 1)))
                  (br $dl)))
              (return (struct.new $zhex_result (i32.const 0)
                (struct.new $zhex_list_ref (ref.as_non_null (local.get $fresh)) (local.get $count) (i32.const 0))))))))))
  (if (i32.eq (local.get $kind) (i32.const 5))
    (then
      (if (i32.ne (local.get $nkind) (i32.const 5))
        (then (return (call $zjson_fail (call $mj_exp_obj) (local.get $path)))))
      (local.set $members (ref.as_non_null (struct.get $zjson_node $members (local.get $n))))
      (local.set $count (array.len (ref.as_non_null (local.get $members))))
      (if (i32.eq (struct.get $zjson_schema $kind (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))) (i32.const 0))
        (then
          (local.set $i64v (call $zhex_map_new_str_i64))
          (block $dec
            (loop $dl
              (br_if $dec (i32.ge_u (local.get $i) (local.get $count)))
              (local.set $r (call $zjson_decode_value
                (ref.as_non_null (struct.get $zjson_member $val
                  (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $members)) (local.get $i)))))
                (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))
                (ref.null eq) (local.get $m)
                (call $zjson_path_key (local.get $path)
                  (struct.get $zjson_member $key (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $members)) (local.get $i)))))))
              (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
                (then (return (local.get $r))))
              (drop (call $zhex_map_put_str_i64 (ref.as_non_null (local.get $i64v))
                (struct.get $zjson_member $key (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $members)) (local.get $i))))
                (struct.get $zhex_some_i64 $value
                  (ref.cast (ref $zhex_some_i64) (struct.get $zhex_result $payload (local.get $r))))))
              (local.set $i (i32.add (local.get $i) (i32.const 1)))
              (br $dl)))
          (return (struct.new $zhex_result (i32.const 0) (ref.as_non_null (local.get $i64v)))))
        (else
          (if (i32.eq (struct.get $zjson_schema $kind (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))) (i32.const 1))
            (then
              (local.set $i32v (call $zhex_map_new_str_i32))
              (block $dec
                (loop $dl
                  (br_if $dec (i32.ge_u (local.get $i) (local.get $count)))
                  (local.set $r (call $zjson_decode_value
                    (ref.as_non_null (struct.get $zjson_member $val
                      (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $members)) (local.get $i)))))
                    (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))
                    (ref.null eq) (local.get $m)
                    (call $zjson_path_key (local.get $path)
                      (struct.get $zjson_member $key (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $members)) (local.get $i)))))))
                  (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
                    (then (return (local.get $r))))
                  (drop (call $zhex_map_put_str_i32 (ref.as_non_null (local.get $i32v))
                    (struct.get $zjson_member $key (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $members)) (local.get $i))))
                    (i31.get_u (ref.cast (ref i31) (struct.get $zhex_result $payload (local.get $r))))))
                  (local.set $i (i32.add (local.get $i) (i32.const 1)))
                  (br $dl)))
              (return (struct.new $zhex_result (i32.const 0) (ref.as_non_null (local.get $i32v)))))
            (else
              (local.set $map (call $zhex_map_new_str_ref))
              (block $dec
                (loop $dl
                  (br_if $dec (i32.ge_u (local.get $i) (local.get $count)))
                  (local.set $r (call $zjson_decode_value
                    (ref.as_non_null (struct.get $zjson_member $val
                      (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $members)) (local.get $i)))))
                    (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))
                    (ref.null eq) (local.get $m)
                    (call $zjson_path_key (local.get $path)
                      (struct.get $zjson_member $key (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $members)) (local.get $i)))))))
                  (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
                    (then (return (local.get $r))))
                  (drop (call $zhex_map_put_str_ref (local.get $map)
                    (struct.get $zjson_member $key (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $members)) (local.get $i))))
                    (struct.get $zhex_result $payload (local.get $r))))
                  (local.set $i (i32.add (local.get $i) (i32.const 1)))
                  (br $dl)))
              (return (struct.new $zhex_result (i32.const 0) (local.get $map)))))))))
  ;; record
  (if (i32.ne (local.get $nkind) (i32.const 5))
    (then (return (call $zjson_fail (call $mj_exp_obj) (local.get $path)))))
  (local.set $members (ref.as_non_null (struct.get $zjson_node $members (local.get $n))))
  (local.set $mcount (array.len (ref.as_non_null (local.get $members))))
  (local.set $fields (ref.as_non_null (struct.get $zjson_schema $fields (local.get $s))))
  (if (i32.eqz (ref.is_null (local.get $def)))
    (then
      (local.set $deff
        (call_ref $zjson_get_fn
          (local.get $def)
          (ref.cast (ref $zjson_get_fn) (struct.get $zjson_schema $get (local.get $s)))))))
  ;; unknown members
  (local.set $i (i32.const 0))
  (block $checked
    (loop $ck
      (br_if $checked (i32.ge_u (local.get $i) (local.get $mcount)))
      (local.set $key (struct.get $zjson_member $key
        (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $members)) (local.get $i)))))
      (local.set $found (i32.const 0))
      (local.set $j (i32.const 0))
      (block $scan
        (loop $sl
          (br_if $scan (i32.ge_u (local.get $j) (array.len (ref.as_non_null (local.get $fields)))))
          (if (call $zhex_str_eq
                (struct.get $zjson_field $name (ref.cast (ref $zjson_field) (array.get $zhex_arr_ref (ref.as_non_null (local.get $fields)) (local.get $j))))
                (local.get $key))
            (then (local.set $found (i32.const 1))))
          (local.set $j (i32.add (local.get $j) (i32.const 1)))
          (br $sl)))
      (if (i32.eqz (local.get $found))
        (then
          (return (call $zjson_fail (call $mj_unknown) (call $zjson_path_key (local.get $path) (local.get $key))))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $ck)))
  ;; fields: present member decodes, absent falls back to the cloned default
  (local.set $i (i32.const 0))
  (local.set $fresh (array.new_default $zhex_arr_ref (array.len (ref.as_non_null (local.get $fields)))))
  (block $filled
    (loop $fl
      (br_if $filled (i32.ge_u (local.get $i) (array.len (ref.as_non_null (local.get $fields)))))
      (local.set $key (struct.get $zjson_field $name
        (ref.cast (ref $zjson_field) (array.get $zhex_arr_ref (ref.as_non_null (local.get $fields)) (local.get $i)))))
      (local.set $found (i32.const -1))
      (local.set $j (i32.const 0))
      (block $scan
        (loop $sl
          (br_if $scan (i32.ge_u (local.get $j) (local.get $mcount)))
          (if (call $zhex_str_eq
                (struct.get $zjson_member $key (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $members)) (local.get $j))))
                (local.get $key))
            (then (local.set $found (local.get $j))))
          (local.set $j (i32.add (local.get $j) (i32.const 1)))
          (br $sl)))
      (if (i32.lt_s (local.get $found) (i32.const 0))
        (then
          (if (i32.eqz (ref.is_null (local.get $deff)))
            (then
              (array.set $zhex_arr_ref (ref.as_non_null (local.get $fresh)) (local.get $i)
                (call $zjson_clone (array.get $zhex_arr_ref (ref.as_non_null (local.get $deff)) (local.get $i))
                  (ref.as_non_null (struct.get $zjson_field $schema
                    (ref.cast (ref $zjson_field) (array.get $zhex_arr_ref (ref.as_non_null (local.get $fields)) (local.get $i)))))
                  (local.get $m))))
            (else
              (return (call $zjson_fail (call $mj_missing) (call $zjson_path_key (local.get $path) (local.get $key)))))))
        (else
          (local.set $r (call $zjson_decode_value
            (ref.as_non_null (struct.get $zjson_member $val
              (ref.cast (ref $zjson_member) (array.get $zhex_arr_ref (ref.as_non_null (local.get $members)) (local.get $found)))))
            (ref.as_non_null (struct.get $zjson_field $schema
              (ref.cast (ref $zjson_field) (array.get $zhex_arr_ref (ref.as_non_null (local.get $fields)) (local.get $i)))))
            (if (result (ref null eq))
              (i32.eqz (ref.is_null (local.get $deff)))
              (then (array.get $zhex_arr_ref (ref.as_non_null (local.get $deff)) (local.get $i)))
              (else (ref.null eq)))
            (local.get $m)
            (call $zjson_path_key (local.get $path) (local.get $key))))
          (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
            (then (return (local.get $r))))
          (array.set $zhex_arr_ref (ref.as_non_null (local.get $fresh)) (local.get $i) (struct.get $zhex_result $payload (local.get $r)))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $fl)))
  (struct.new $zhex_result (i32.const 0)
    (call_ref $zjson_make_fn
      (ref.as_non_null (local.get $fresh))
      (ref.cast (ref $zjson_make_fn) (struct.get $zjson_schema $make (local.get $s))))))

;; json.decode: preamble + parse + trailing check, then the schema walk
(func $zhex_json_decode (param $raw (ref $zhex_string)) (param $defaults (ref null eq)) (param $s (ref $zjson_schema)) (result (ref $zhex_result))
  (local $c i32)
  (local $r (ref $zhex_result))
  (if (i32.eqz (call $zhex_json_surrogates_ok (local.get $raw)))
    (then (return (call $zjson_fail (call $mj_surrogate) (array.new_fixed $zhex_string 0)))))
  (global.set $zjson_i (i32.const 0))
  (local.set $r (call $zjson_parse_value (local.get $raw) (array.new_fixed $zhex_string 0) (i32.const 0) (i32.const 0)))
  (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
    (then (return (local.get $r))))
  (call $zjson_skip_ws (local.get $raw))
  (local.set $c (call $zjson_peek (local.get $raw)))
  (if (i32.eq (local.get $c) (i32.const -1))
    (then
      (return (call $zjson_decode_value (ref.cast (ref $zjson_node) (struct.get $zhex_result $payload (local.get $r)))
        (local.get $s) (local.get $defaults)
        (struct.new $zjson_memo (ref.null $zhex_arr_ref) (ref.null $zhex_arr_ref))
        (array.new_fixed $zhex_string 0)))))
  (if (i32.or (i32.or (i32.or (i32.eq (local.get $c) (i32.const 123)) (i32.eq (local.get $c) (i32.const 91))) (i32.eq (local.get $c) (i32.const 34)))
              (i32.or (i32.or (i32.eq (local.get $c) (i32.const 116)) (i32.eq (local.get $c) (i32.const 102)))
                      (i32.or (i32.or (i32.eq (local.get $c) (i32.const 110)) (i32.eq (local.get $c) (i32.const 45)))
                              (i32.and (i32.ge_u (local.get $c) (i32.const 48)) (i32.le_u (local.get $c) (i32.const 57))))))
    (then (return (call $zjson_fail (call $mj_multiple) (array.new_fixed $zhex_string 0)))))
  (call $zjson_fail
    (call $zhex_cat3 (call $mj_inv_char) (call $mj_char_q (local.get $c)) (call $mj_looking_b))
    (array.new_fixed $zhex_string 0)))
`
