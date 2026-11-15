package lang

// Milestone-3 names shared between the WAT emitter (wat.go), the M3 runtime
// prelude (watprelude_m3.go), and the two host skin layers
// (wathost_browser.go / wathost_wasi.go), pinned in internal/wasm/DESIGN-M3.md
// §1 (imports, scratch protocol, errno/fail tables), §3-§7 (file/buffer/
// duration/json/env helpers), §8 (fail codes 16-18, families 2-5), and §10.
// The prelude and skin layers define these symbols in WAT; the emitter
// references them by these exact spellings.
//
// Emit recipe (wat.go emit(), DESIGN-M3 §1/§10 — wasm-tools requires every
// (import) ahead of all definitions, verified against 1.263):
//
//	out.WriteString(watHostImports(skin))
//	out.WriteString(watStripSeamDefs(watPrelude + watPreludeM2 + watPreludeM3))
//	out.WriteString(watHostFuncs(skin))
//	// program types, words, $zhex_main, thunks, exports — unchanged
//
// watStripSeamDefs removes the three browser-seam definitions the host layer
// replaces ($env.print's M1 import plus the M1 $zhex_fail and M2
// $zhex_fail_msg functions), identically for both skins, so the emitted
// prelude text stays byte-identical between skins and only the host layer
// differs (DESIGN-M3 §1). E's $zhex_main gains `call $zhex_env_init` as its
// first statement (§7).

// WatSkin selects the host layer a compiled module targets (DESIGN-M3 §1c;
// default browser per D4). Part of CompileWat's exported signature.
type WatSkin uint8

const (
	WatSkinBrowser WatSkin = iota
	WatSkinWASI
)

// watHostImports is the skin's import block: browser = the fifteen env.*
// imports of §1a; WASI = the thirteen wasi_snapshot_preview1 imports of §1c.
// It must be written before every module definition (wasm-tools rule).
func watHostImports(skin WatSkin) string {
	if skin == WatSkinWASI {
		return watWasiImports
	}
	return watBrowserImports
}

// watHostFuncs is the skin's host layer: the fifteen $zhex_host_* helpers
// (imports on browser, adapters over WASI on the WASI skin), the $env.print
// seam, and the skin's $zhex_fail/$zhex_fail_msg (browser keeps the M1/M2
// trap semantics; WASI writes the pinned stderr line via fd_write and exits
// through proc_exit(1) — DESIGN-M3 Surprises 8). The WASI layer also exports
// _start. May be written anywhere after the imports.
func watHostFuncs(skin WatSkin) string {
	if skin == WatSkinWASI {
		return watWasiFuncs
	}
	return watBrowserFuncs
}

// watM1EnvPrintImport is the M1 import line watStripSeamDefs removes; the
// host layers provide $env.print themselves (browser forwards to the env
// import, WASI writes fd 1), so the frozen M1 line must not survive.
const watM1EnvPrintImport = `(import "env" "print" (func $env.print (param i32 i32)))`

// The fifteen $zhex_host_* seam names (DESIGN-M3 §1a). Signatures:
//
//	print          (ptr i32, len i32)                 raw UTF-8 stdout, no newline
//	stdin_read     (ptr i32, cap i32) -> i32          n bytes, 0 = EOF, -1 = failure
//	fs_read_file   (pptr i32, plen i32) -> errno      whole file to scratch
//	fs_write_file  (pptr, plen, dptr, dlen, append) -> errno
//	fs_open_read   (pptr i32, plen i32) -> i32        handle >= 0, < 0 = -errno
//	fs_read_line   (handle i32) -> i32                1 line, 0 EOF, < 0 = -errno
//	fs_close       (handle i32) -> errno              idempotent
//	fs_stat        (pptr i32, plen i32) -> errno      scratch [u32 kind][i64 size]
//	fs_list        (pptr i32, plen i32) -> errno      scratch [u32 count]{[u32 len][bytes]}
//	fs_make_folder (pptr, plen, parents i32) -> errno
//	fs_delete      (pptr i32, plen i32) -> errno
//	fs_delete_folder (pptr, plen, recursive i32) -> errno
//	fs_move        (sptr, slen, dptr, dlen, overwrite i32) -> errno
//	env_get        (kptr i32, klen i32) -> i32        1 set, 0 unset, < 0 = -errno
//	env_environ    () -> errno                        scratch [u32 count]{[u32 klen][key][u32 vlen][value]}
//
// Scratch protocol: host->module data always lands at memory offset 0 as
// [u32 byte length][bytes]; the host grows memory first (§1a).
const (
	watHostPrint          = "$zhex_host_print"
	watHostStdinRead      = "$zhex_host_stdin_read"
	watHostFsReadFile     = "$zhex_host_fs_read_file"
	watHostFsWriteFile    = "$zhex_host_fs_write_file"
	watHostFsOpenRead     = "$zhex_host_fs_open_read"
	watHostFsReadLine     = "$zhex_host_fs_read_line"
	watHostFsClose        = "$zhex_host_fs_close"
	watHostFsStat         = "$zhex_host_fs_stat"
	watHostFsList         = "$zhex_host_fs_list"
	watHostFsMakeFolder   = "$zhex_host_fs_make_folder"
	watHostFsDelete       = "$zhex_host_fs_delete"
	watHostFsDeleteFolder = "$zhex_host_fs_delete_folder"
	watHostFsMove         = "$zhex_host_fs_move"
	watHostEnvGet         = "$zhex_host_env_get"
	watHostEnvEnviron     = "$zhex_host_env_environ"
)

// The WASI skin's import names (DESIGN-M3 §1c): wasi_snapshot_preview1
// functions the adapter layer builds the same $zhex_host_* names over.
// Guest paths resolve against the first preopen (fd 3), leading "/" stripped.
const (
	wasiFnFdWrite         = "fd_write"
	wasiFnFdRead          = "fd_read"
	wasiFnPathOpen        = "path_open"
	wasiFnFdClose         = "fd_close"
	wasiFnFdFilestatGet   = "fd_filestat_get"
	wasiFnFdReaddir       = "fd_readdir"
	wasiFnPathCreateDir   = "path_create_directory"
	wasiFnPathUnlinkFile  = "path_unlink_file"
	wasiFnPathRemoveDir   = "path_remove_directory"
	wasiFnPathRename      = "path_rename"
	wasiFnEnvironSizesGet = "environ_sizes_get"
	wasiFnEnvironGet      = "environ_get"
	wasiFnProcExit        = "proc_exit"
)

// Fail codes 16-18 (DESIGN-M3 §8) and the $zhex_err families 2-5. Codes
// 16-18 stage their message via $zhex_fail_msg; 16 retires M2's code-12
// reuse for top-level unhandled Results. Never renumber.
const (
	watFailUnhandledResult = 16 // unhandled-result, message staged (§8)
	watFailStreamFile      = 17 // file-open/file-read/file-utf8 in file.stream
	watFailFileExists      = 18 // file-exists stat failure other than not-found
)

const (
	watErrFamilyInput  = 2 // InputError  — read-line
	watErrFamilyFile   = 3 // FileError   — file.*
	watErrFamilyBuffer = 4 // BufferError — buffer.line/set-line bounds
	watErrFamilyCommon = 5 // Error       — json.*, env.load, duration.parse
)

// WASI preview 1 errno numbers (DESIGN-M3 §1b). Pinned from the
// wasi-sysroot headers and verified under wasmtime 50 on Linux: EACCES=2,
// EEXIST=20, EINVAL=28, EISDIR=31, ENOENT=44, ENOSYS=52, ENOTDIR=54,
// ENOTEMPTY=55, EXDEV=75. The prelude renders Go's syscall.Errno strings
// for these numbers; unknown errno renders "errno N".
const (
	watErrnoAcces    = 2
	watErrnoExist    = 20
	watErrnoInval    = 28
	watErrnoIsdir    = 31
	watErrnoNoent    = 44
	watErrnoNosys    = 52
	watErrnoNotdir   = 54
	watErrnoNotempty = 55
	watErrnoXdev     = 75
)

// M3 value types (DESIGN-M3 §3/§4/§6). The File and Buffer structs are the
// emitter's representation for fileType/bufferType values (reference
// semantics — two names observe one file/buffer); the Json variants follow
// the M1 padded-variant scheme (variant i gets i i64 pads + trailing i32
// discriminant) with ordinals Null 0 … Object 5.
const (
	watTypeFile   = "$zhex_file"   // {path: (ref $zhex_string), handle: i32, closed: (mut i32)}
	watTypeBuffer = "$zhex_buffer" // {lines: (mut (ref null $zhex_arr_str))}
)

// Json variant ordinals and struct names ($zt_Json_<Variant>).
const (
	watJsonNull   = 0
	watJsonBool   = 1
	watJsonText   = 2
	watJsonNumber = 3
	watJsonList   = 4
	watJsonObject = 5
)

func watJsonType(variant int) string {
	return "$zt_Json_" + [...]string{"Null", "Bool", "Text", "Number", "List", "Object"}[variant]
}

// Internal json types (DESIGN-M3 §6), declared in one rec group by the
// prelude. Parse nodes use kinds 0 null … 5 object; decode schemas use
// kinds 0 int … 6 record.
const (
	watTypeJsonNode   = "$zjson_node"
	watTypeJsonMember = "$zjson_member"
	watTypeJsonSchema = "$zjson_schema"
	watTypeJsonField  = "$zjson_field"
	watTypeJsonMemo   = "$zjson_memo"
	watTypeJsonMakeFn = "$zjson_make_fn" // (func (param (ref $zhex_arr_ref)) (result (ref null eq)))
	watTypeJsonGetFn  = "$zjson_get_fn"  // (func (param (ref null eq)) (result (ref null $zhex_arr_ref)))
)

// Schema kinds for $zjson_schema nodes and the emitter's per-call-site
// builders. CONTRACT DEVIATION from DESIGN-M3 §6 (flagged for E): the
// pinned three-field $zjson_schema cannot build or read user record
// structs from generic WAT, so the prelude's schema type carries two extra
// nullable-func fields — $make (type $zjson_make_fn) and $get (type
// $zjson_get_fn) — null for every kind except record. E's record schema
// builders pass site thunks that convert between the record struct and an
// eqref array of M1-boxed fields (Int as $zhex_some_i64, Bool as i31).
const (
	watJsonSchemaInt    = 0
	watJsonSchemaBool   = 1
	watJsonSchemaString = 2
	watJsonSchemaOption = 3
	watJsonSchemaList   = 4
	watJsonSchemaMap    = 5
	watJsonSchemaRecord = 6
)

func watJsonSchemaBuilder(n int) string { return "$zjson_schema" + itoa(n) }
func watJsonMakeThunk(n int) string     { return "$zjson_make" + itoa(n) }
func watJsonGetThunk(n int) string      { return "$zjson_get" + itoa(n) }

// Errno rendering (§1b): $zhex_errno_text(e) is the Go/Linux errno text for
// a WASI errno number (unknown -> "errno N"); $zhex_os_err renders the
// interpreter's inner PathError/LinkError text "<verb> <path>: <errno text>"
// (fs_move callers pass the two-path verb themselves).
const (
	watErrnoText = "$zhex_errno_text"
	watOsErr     = "$zhex_os_err"
)

// Byte/scratch helpers (§1a scratch protocol; §4 UTF-8 validation). Scratch
// payloads live at offset 4 after the [u32 length] header at offset 0.
const (
	watBytesToString = "$zhex_bytes_to_string" // (ptr, len) -> string, lenient decode
	watBytesUtf8Ok   = "$zhex_bytes_utf8_ok"   // (ptr, len) -> i32, strict UTF-8 check
	watStageUtf8At   = "$zhex_stage_utf8_at"   // (s, off) -> i32 byte length staged
	watScratchLen    = "$zhex_scratch_len"     // () -> i32, the u32 at offset 0
)

// Print amendment (§1 Surprises 2): the module owns the newline.
// $zhex_print_line(s) stages s + "\n" at offset 0 and calls
// $zhex_host_print(0, len); run.mjs stops appending.
const watPrintLine = "$zhex_print_line"

// Pure path words (§3), lexical filepath semantics, host-free.
const (
	watPathJoin       = "$zhex_path_join"
	watPathBase       = "$zhex_path_base"
	watPathExtension  = "$zhex_path_extension"
	watPathDirOf      = "$zhex_path_dir_of"
	watPathCleanInner = "$zhex_path_clean" // shared by join/dir-of
)

// read-line (§4): prompt bytes go out raw via $zhex_host_print, then the
// prelude loops $zhex_host_stdin_read (4096-byte chunks at the prelude's
// read slot) over a module-side pending byte array until "\n" or EOF;
// chomp strips one "\n" then one "\r" only when the newline was present.
const (
	watReadLine = "$zhex_read_line" // (prompt) -> Result<String, InputError>
)

// buffer.* (§4): pure runtime, no imports, no element limit.
const (
	watBufferNew       = "$zhex_buffer_new"
	watBufferAddLine   = "$zhex_buffer_add_line"
	watBufferLineCount = "$zhex_buffer_line_count"
	watBufferLine      = "$zhex_buffer_line"     // (b, i) -> Result<String, BufferError>
	watBufferSetLine   = "$zhex_buffer_set_line" // (b, i, s) -> Result<Unit, BufferError>
	watBufferToText    = "$zhex_buffer_to_text"
)

// file.* over the import surface (§3). Messages quote paths with
// $zhex_quote (M2) and render inner errors via $zhex_os_err; $zhex_file
// handles are the struct above. The stream helpers back
// `for line in file.stream` with hard fail code 17.
const (
	watFileRead         = "$zhex_file_read"          // (path) -> Result<String, FileError>
	watFileOpen         = "$zhex_file_open"          // (path) -> Result<File, FileError>
	watFileReadLine     = "$zhex_file_read_line"     // (file) -> Result<String, FileError>
	watFileClose        = "$zhex_file_close"         // (file) -> Unit
	watFileWrite        = "$zhex_file_write"         // (contents, path, append) -> Result<Unit, FileError>
	watFileList         = "$zhex_file_list"          // (path) -> Result<List<String>, FileError>
	watFileExists       = "$zhex_file_exists"        // (path) -> Bool (hard fail 18 on stat errors)
	watFileSize         = "$zhex_file_size"          // (path) -> Result<Int, FileError>
	watFileMakeFolder   = "$zhex_file_make_folder"   // (path, parents) -> Result<Unit, FileError>
	watFileDelete       = "$zhex_file_delete"        // (path) -> Result<Unit, FileError>
	watFileDeleteFolder = "$zhex_file_delete_folder" // (path, recursive) -> Result<Unit, FileError>
	watFileMove         = "$zhex_file_move"          // (src, dst, overwrite) -> Result<Unit, FileError>
	watStreamOpen       = "$zhex_stream_open"        // (path) -> i32 handle, fail_msg 17 on open
	watStreamLine       = "$zhex_stream_line"        // (handle) -> Result: ok-null = clean EOF, ok-string = chomped line; fail_msg 17 on read/UTF-8
	watStreamClose      = "$zhex_stream_close"       // (handle) -> Unit, always closes
)

// duration.* (§5): Duration is unboxed i64 nanoseconds. duration.parse
// reimplements Go's time.ParseDuration (go1.27.3 unsigned arithmetic read
// from the toolchain source) with every failure collapsed to "invalid
// duration" in the common Error family; to-string reimplements
// time.Duration.format (µ = U+00B5).
const (
	watDurationParse    = "$zhex_duration_parse"     // (s) -> Result<Duration, Error>
	watDurationToString = "$zhex_duration_to_string" // (i64) -> string
)

// json.* (§6). parse returns the Json tree; encode takes the schema ref
// (null schema = Json tree path); decode parses once then schema-walks
// with the defaults ref and a linear ref.eq memo for aliasing-preserving
// default fills. All failures are family 5 with ` at "<path>"` suffixes.
const (
	watJsonParse       = "$zhex_json_parse"         // (raw) -> Result<Json, Error>
	watJsonEncode      = "$zhex_json_encode"        // (v, schema|null) -> Result<String, Error>
	watJsonDecode      = "$zhex_json_decode"        // (raw, defaults, schema) -> Result<T, Error>
	watJsonSurrogateOk = "$zhex_json_surrogates_ok" // (s) -> i32, the strictness preamble
)

// env.* (§7): module-side state, a $zhex_map_str_ref seeded once from
// $zhex_host_env_environ; hosts are never mutated. env.get is a plain
// Option (never a Result); env.load applies first-wins puts.
const (
	watEnvInit = "$zhex_env_init" // () -> Unit, idempotent guard; first stmt of $zhex_main
	watEnvGet  = "$zhex_env_get"  // (name) -> Option<String> as eq (some_ref/none)
	watEnvLoad = "$zhex_env_load" // (path) -> Result<Unit, Error>
)

// itoa is the small decimal helper the schema-thunk name builders share.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
