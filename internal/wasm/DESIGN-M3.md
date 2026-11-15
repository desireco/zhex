# Zhex → WebAssembly, Milestone 3: delta contract

Binding inputs: `internal/wasm/DESIGN.md` (M1) and `internal/wasm/DESIGN-M2.md` (M2) — both hold
except where amended here — `docs/wasm-plan.md` D3 (Duration-only) and D4 (browser first, WASI
at M3), ticket #100, and the interpreter at `internal/lang` (`host.go`, `files.go`,
`json_codec.go`, `temporal.go`, `eval.go`). Interpreter semantics were pinned from that code,
not from memory; where wording differs, the code wins. Acceptance stays: differential fixtures
produce byte-identical stdout and exit status vs `zhex run` on the CI OS (Linux); stderr follows
the M1 convention (`runtime error: ` prefix replaces the interpreter's `file:line:column:`).
Only the delta from M2 is specified.

## Surprises (read first)

1. **`text-stats`/`file-work` as shipped use fenced words** (`console.arg`, `file.free-space`).
   The acceptance fixtures are *style-adapted* copies (`text-stats-wasm.zhex`,
   `file-work-wasm.zhex`): inline relative paths, free-space block dropped. Full-corpus parity
   waits for M4.
2. **`env.print` semantics amend (raw bytes).** `read-line`'s prompt has no trailing newline,
   which M1's runner-side `"\n"` append cannot express. M3: `env.print(ptr, len)` sends raw
   UTF-8 stdout bytes, the prelude's print helper appends `"\n"` itself, `run.mjs` stops
   appending. Final bytes identical; M1/M2 fixtures stay green.
3. **OS error text parity is a table, not a string.** The interpreter embeds Go
   `*PathError`/`*LinkError` text (`open x: no such file or directory`). Only an errno crosses
   the boundary (WASI preview 1 numbering); the prelude renders `<verb> <path>: <errno text>`
   with Linux/Go strings (§1b). `fs_read_file` on a directory fails at Go's *read* stage —
   EISDIR there renders verb `read`.
4. **The Json tree needs no rec group.** Its payloads are String, Bool, the ref-spec list, and
   the str→ref map — eqref-element containers — so the Json type graph is acyclic and the six
   variants are plain M1 choice structs (padding distinguishes `Text` from `Number`, which
   collide structurally). The *internal* parse-node and decode-schema types do need one rec
   group.
5. **Go `encoding/json` messages are observable** (unhandled `json.*` failures): the prelude
   embeds Go's `invalid character 'x' …` context phrases and `unexpected end of JSON input`;
   encode must HTML-escape `<`, `>`, `&` (plus U+2028/U+2029) as `json.Marshal` does.
6. **WASI has no setenv** — env state lives module-side (a seeded map), making
   `env.get`/`env.load` semantics identical on both skins by construction.
7. **`duration.parse` fails in the common `Error` family** (checker: `Result<Duration, Error>`),
   not ParseError; the internal unit is **nanoseconds** (`duration.new` is an i64 passthrough).
8. **Traps cannot carry staged messages under the wasmtime CLI** (the process dies before anyone
   reads `zhex_msg_len`). The WASI skin's `$zhex_fail`/`$zhex_fail_msg` write the pinned stderr
   line via `fd_write`(fd 2) and exit through `proc_exit(1)`; the browser skin keeps the M1
   trap. Native-trap diagnostics (division by zero) stay browser-only fixtures; WASI fixtures
   avoid native traps.
9. **M2's top-level unhandled-result path reused fail code 12**; M3 mints 16 for it (staged) so
   12 stays the to-int mint. Staged bytes unchanged.
10. **`buffer.*` has no element limit** in the interpreter — none is compiled. `file.list`'s
    1,000,000-entry limit *is* checked (§2).

## 1. Host-import surface (the design decision)

One interface, two skins. Program WAT, `watPrelude`, `watPreludeM2`, and the skin-independent
`watPreludeM3` are byte-identical for both skins; only the final *host layer* differs, and both
host layers define the **same** `$zhex_host_*` helper names (pinned in `watnames_m3.go`, §10).
Only linear-memory UTF-8 crosses the boundary — **no GC refs** (M1 §4 rule).

**(a) Browser skin imports** (module `"env"`, 15 total; errno results use WASI preview 1
numbering, 0 = success):

| import | signature | semantics |
|---|---|---|
| `print` | `(ptr, len)` | raw UTF-8 stdout bytes, no newline added (Surprises 2) |
| `stdin_read` | `(ptr, cap) -> n` | up to `cap` bytes at `ptr`; `0` = EOF; `-1` = read failure |
| `fs_read_file` | `(pptr, plen) -> errno` | whole file → scratch (below); dir → EISDIR (read stage) |
| `fs_write_file` | `(pptr, plen, dptr, dlen, append) -> errno` | create-or-truncate / append (creates missing) |
| `fs_open_read` | `(pptr, plen) -> i32` | `>=0` handle, `<0` = `-errno` |
| `fs_read_line` | `(handle) -> i32` | `1` line (raw bytes incl. trailing `"\n"`) → scratch; `0` clean EOF; `<0` = `-errno` |
| `fs_close` | `(handle) -> errno` | idempotent |
| `fs_stat` | `(pptr, plen) -> errno` | scratch: `[u32 kind][i64 size]`, kind 0 file / 1 dir |
| `fs_list` | `(pptr, plen) -> errno` | scratch: `[u32 count]` then per entry `[u32 len][bytes]`; sorted, dotfiles included, no `.`/`..` |
| `fs_make_folder` | `(pptr, plen, parents) -> errno` | one level; `parents` creates ancestors, tolerates existing |
| `fs_delete` | `(pptr, plen) -> errno` | file or *empty* folder (os.Remove) |
| `fs_delete_folder` | `(pptr, plen, recursive) -> errno` | empty folder; `recursive` = best-effort tree removal |
| `fs_move` | `(sptr, slen, dptr, dlen, overwrite) -> errno` | rename; overwrite replaces files only |
| `env_get` | `(kptr, klen) -> i32` | `1` set (scratch `[u32 len][bytes]`), `0` unset, `<0` = `-errno` |
| `env_environ` | `() -> errno` | scratch: `[u32 count]` then per pair `[u32 klen][key][u32 vlen][value]` |

**Scratch protocol** (host→module data): always memory offset 0, `[u32 byte length][bytes]`; the
host grows memory first, the module consumes synchronously — M2's staged-message slot
discipline. Module→host strings are arbitrary `(ptr, len)` pairs into module memory. No
multi-value imports; single-value errno / handle-or-`-errno` returns everywhere.

**(b) Errno → text.** Prelude table renders Go/Linux `syscall.Errno` strings; fixture-reachable
set: 2 EACCES `permission denied`, 20 EEXIST `file exists`, 28 EINVAL `invalid argument`, 31
EISDIR `is a directory`, 44 ENOENT `no such file or directory`, 54 ENOTDIR `not a directory`, 55
ENOTEMPTY `directory not empty`, 75 EXDEV `invalid cross-device link`. Full 1–75 table generated
offline like the M2 case tables; unknown errno renders `errno N` (shortcut: unreachable through
the fixture set). Verb table (the prelude renders the interpreter's wrapper plus the inner
`PathError`/`LinkError` text, matching `files.go` exactly):

| word / stage | inner error text |
|---|---|
| `file.read`, `file.open`, `env.load` read, `file.stream` open | `open <path>: <err>`; EISDIR from `fs_read_file` → `read <path>: is a directory` |
| `file.write` phases | `open <path>: <err>` / `write <path>: <err>` / `close <path>: <err>` (hosts never report short writes) |
| `file.exists` (code 18), `file.size` | `stat <path>: <err>` |
| `file.list` | `open <path>: <err>`; ENOTDIR → `open <path>: not a directory` (2026-11-15 amendment: ENOTDIR renders at the open stage as `open <path>: not a directory`, matching the interpreter and the pinned fixture — the earlier `readdirent` wording was wrong) |
| `file.make-folder` | `mkdir <path>: <err>` (both modes) |
| `file.delete`, `file.delete-folder` | `remove <path>: <err>` |
| `file.move` | `rename <src> <dst>: <err>` (LinkError, both paths) |
| handle `file.read-line` read failure | `read <path>: <err>` behind `reading <q> failed: ` |

**(c) Skins and the CLI.** `zhex compile <file> [-o out] [--target=browser|wasi]`; **default =
browser** (D4). Unknown `--target` value is a usage error (exit 2). `lang.CompileWat(entryPath,
entrySource, resolve, skin)` gains the skin; `internal/wasm.Compile` forwards it. The WASI skin
emits a module importing `wasi_snapshot_preview1` (`fd_write`, `fd_read`, `path_open`,
`fd_close`, `fd_filestat_get`, `fd_readdir`, `path_create_directory`, `path_unlink_file`,
`path_remove_directory`, `path_rename`, `environ_sizes_get`, `environ_get`, `proc_exit`) whose
adapter functions (WAT text in the host layer, H's file) implement the same `$zhex_host_*`
names; all guest paths resolve against the first preopen (fd 3), leading `/` stripped (shortcut:
single-preopen assumption). Its `$zhex_fail`/`$zhex_fail_msg` follow Surprises 8; it exports
`_start` (calls `zhex_main`) beside the M1 exports. Runners: browser = `node run.mjs <wasm>
--root <dir> --stdin <file> [--env K=V …]`; WASI = `wasmtime run --dir <dir>::/ [--env K=V]
<wasm> < stdin` (wasmtime 50 flags verified).

## 2. Virtual FS semantics (browser skin)

In-memory tree the runner seeds from `--root`; starts empty otherwise (a real page can back it
with anything later). Paths are used verbatim as given, resolved against the root; `/` is the
separator on **all skins** — parity is with `zhex run` on the CI OS (Linux, where `filepath`
uses `/`); Windows `zhex run` divergence is out of scope. Pinned semantics: `fs_write_file`
truncates/creates (0644, unobservable); append creates missing files; `fs_make_folder` without
`parents` fails EEXIST on any existing target, with `parents` tolerates one; `fs_delete` of a
non-empty folder → ENOTEMPTY; `fs_list` returns bare names in byte-sorted order; `fs_read_line`
returns raw bytes through the next `"\n"` (partial final line included). No permissions are
modeled (EACCES unreachable on this skin; WASI shares the fixture set). Open handles read live
node contents; delete-then-read-into-an-open-handle is excluded by fixture discipline
(documented limit). The prelude checks `file.list`'s 1,000,000-entry limit on `fs_list`'s count
(Result, family 3, `file.list exceeds element limit`). **Portable-path discipline:** every file
fixture uses relative `/`-separated paths only — no `..`, no absolute paths, no drive letters —
so all three runs (Linux interpreter, virtual FS, WASI preopen) see identical trees and
byte-match.

## 3. `file.*` mapping

- **Pure path words, host-free** (prelude `$zhex_path_join/_base/_extension/_dir_of`; lexical
  `filepath` Unix semantics): `join` = `joinPath` — an absolute child (leading `/`) replaces the
  base after `Clean`; else `Clean(base + "/" + child)` (empty → `.`; `.`/`..` resolved
  lexically). `base` = final element (`/` for all slashes; trailing slashes ignored).
  `extension` = suffix from the final dot of the final element, dot included (`.bashrc` →
  `.bashrc`), empty when no dot. `directory-of` = all but the final element, `Clean`ed (`.` for
  bare names, `/` for root).
- **Through imports** (family FileError = 3; messages = the interpreter's first line minus
  position, paths `%q`-quoted via `$zhex_quote`): `read`, `open`, `read-line(handle)`, `close`,
  `write`, `list`, `exists`, `size`, `make-folder`, `delete`, `delete-folder`, `move`. Handles
  are `$zhex_file {path: (ref $zhex_string), handle: i32, closed: i32}` (ref-spec; two names
  observe one file). `file.read-line(handle)`: closed → `file <q> is closed`; clean EOF → `end
  of file`; read failure → `reading <q> failed: read <path>: <err>`; invalid UTF-8 in the line →
  `file <q> is not valid UTF-8`. Chomp (strip one trailing `"\n"` then one `"\r"`, only when the
  newline was present) is prelude-side, once, for console and file reads alike. `file.move`
  pre-checks with `fs_stat` (module-side, the interpreter's check-then-act shape): target dir →
  `cannot move <q> to <q>: the target is a folder`; existing target without overwrite → `… the
  target exists`. Plain `file.exists`: ENOENT yields `false`; any other errno is a **hard**
  staged failure (code 18, `cannot check <q>: stat <path>: <err>`; unreachable in fixtures). The
  `try`/`catch` checked form stays fenced (§8).
- **`for line in file.stream(path)`**: open → `fs_read_line` until `0` → close on every exit
  (early word `return` via the M1 `watCleanup` list). Open, read, and UTF-8 failures are hard
  staged errors (code 17): `cannot open <q>: open <path>: <err>`, `reading <q> failed: read
  <path>: <err>`, `file <q> is not valid UTF-8`.
- **`file.free-space` stays fenced** (ticket default; claim explicitly to add).

## 4. `read-line` and `buffer.*`

- **`read-line(prompt)`** (InputError = 2): the prompt is written to stdout *raw* (no newline)
  before reading (Surprises 2). The prelude keeps a pending byte buffer and loops `stdin_read`
  (4096-byte chunks, module-grown) until `"\n"` or EOF. EOF with an empty buffer → `end of
  input` (reason detail `end-of-file` exists in the interpreter, unread while `error.*` is
  fenced); read failure (`-1`) → `console input failed: read error` (shortcut: unreachable under
  both harnesses; pinned for completeness); invalid UTF-8 → `console input is not valid UTF-8`;
  else the chomped line. Browser stdin is the runner's supplied string; WASI stdin is fd 0.
- **`buffer.*`** (pure runtime, no imports): `$zhex_buffer {lines: (mut (ref null
  $zhex_arr_str))}`. `new` empty; `add-line` appends (no limit — Surprises 10); `line-count` =
  length; `line(i)` / `set-line(i, text)` are bounds-checked Results (BufferError = 4) with
  `buffer index <i> is outside 0..<n-1>` (rendered with `$zhex_int_to_string`; empty buffer
  prints `0..-1`); `to-text` = join with `"\n"` (empty → `""`). Reference semantics are the
  struct ref; `==`/`print` of buffers remain checker-rejected.

## 5. `duration.*` (D3)

Duration = **i64 nanoseconds, unboxed** (distinct checker type, Int's repr). `duration.new(n)`
is a passthrough. `duration.parse(text)` → Result (family 5 — Surprises 7): grammar and
arithmetic exactly Go `time.ParseDuration` at the go.mod toolchain — optional sign;
special-cased `0`; `[digits[.digits]]unit` runs with units `ns`, `us`, `µs` (U+00B5), `μs`
(U+03BC), `ms`, `s`, `m`, `h`; Go's fraction and overflow rules (read Go's source, do not
guess); every failure collapses to `invalid duration`. `duration.to-string(d)` (and `print` of a
Duration) reproduces `time.Duration.String()` exactly: `0s`, ns/µs/ms with fractions below one
second (µ = U+00B5), `1.5s`, `1h30m0s`, leading `-`. No clocks;
`date.`/`time.`/`instant.`/`timezone.`/`zoned.` stay fenced.

## 6. `json.*` and the `Json` tree

- **Representation.** Six variant structs `$zt_Json_<Null|Bool|Text|Number|List|Object>` in the
  M1 choice scheme (variant *i* padded with *i* i64 fields + trailing discriminant —
  `Text`/`Number` collide without it), ordinals Null 0 … Object 5 (pinned from `jsonTreeValue`).
  `List` holds a `$zhex_list_ref`, `Object` a `$zhex_map_str_ref` — **no rec group needed**
  (Surprises 4); constructors are ordinary struct.news; `match` is the M1 `ref.test` chain.
- **Internal types** (one rec group, prelude-owned): parse nodes `$zjson_node {kind i32, bool
  i32, text (ref null $zhex_string), items (ref null $zhex_arr_ref), members (ref null
  $zhex_arr_ref)}` with `$zjson_member {key, val}` (kinds 0 null … 5 object), and decode schemas
  `$zjson_schema {kind i32, element (ref null $zjson_schema), fields (ref null $zhex_arr_ref)}`
  + `$zjson_field {name, schema}` (kinds 0 int … 6 record). One `$zjson_schema<N>` builder func
  per `json.decode`/typed `json.encode` call site (emitted; no caching). (2026-11-15 amendment:
  `$zjson_schema` additionally carries `$make`/`$get` thunk fields beyond the three-field pin
  above; the deviation flag in `watnames_m3.go:191` documents why.)
- **`json.parse(raw)`** → `$zhex_json_parse((ref $zhex_string)) (ref $zhex_result)` (family 5).
  Strictness preamble: module strings are valid UTF-8 by construction (noted, not re-checked);
  the surrogate scan follows `validateJSONSurrogates` verbatim (per-byte, inside string literals
  only, unpaired → `unpaired UTF-16 surrogate`). Parser: whitespace space/`\t`/`\n`/`\r`;
  literals; strings escape `\" \\ \/ \b \f \n \r \t` and `\uXXXX` (pairs combine); numbers
  captured as **raw literal text**; depth > 256 → `JSON nesting limit exceeded`; duplicate
  object key → `duplicate object key`; non-string key → `invalid object key`; trailing data →
  `invalid JSON: multiple JSON values`; syntax errors render Go's phrasing — `invalid JSON:
  invalid character '<c>' <context>` with the context table (`looking for beginning of value`,
  `looking for beginning of object key string`, `after object key`, `after object key:value
  pair`, `after array element`, `after top-level value`, `in literal true/false/null (expecting
  '<c>')`) and `invalid JSON: unexpected end of JSON input` at EOF; character rendering is Go's
  single-rune `%q` for ASCII (shortcut: fixtures stick to ASCII offenders; upgrade the table on
  divergence). All failures carry `at "<path>"` with JSON-pointer-ish paths (`/0`, `/a`,
  `~0`/`~1` escaping), root `""` — e.g. `expected Int at ""`. Tree conversion: object members
  into a `Map<String, Json>` in insertion order; array elements > 1,000,000 → `list exceeds
  element limit`.
- **`json.encode(value)`**: tree path when the arg type is `Json` (schema nil), typed path
  otherwise. Compact separators `{ } [ ] , :`, no spaces. Strings escape exactly as
  `json.Marshal`: `"` `\`, controls (< 0x20) with `\b \t \n \f \r` short forms else `\uXXXX`,
  **plus `<` `>` `&` → `\u003c \u003e \u0026` and U+2028/U+2029** (Surprises 5). Tree `Number`
  literals are re-validated against the JSON number grammar
  (`-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?`, pinned verbatim from `jsonNumberLiteral`) →
  `number literal <q> is not valid JSON`; valid literals re-emit byte-for-byte. Typed encode:
  record fields in declaration order, map entries in insertion order, `Option.None` → `null`,
  `Some` → the value; depth > 256 fails as above. Round-trip parity: key order and number
  literals preserved.
- **`json.decode(raw, defaults)`**: parse once, then schema-walk with the defaults ref. Option:
  `null` → `None`, else decode the inner node (defaults unwrap one `Some`). Records: unknown key
  → `unknown field at "<path>"`; missing field without a default → `missing required field at
  "<path>"`; present defaults fill via deep clone with aliasing preserved — a linear-scan
  identity memo over `ref.eq` (defaults trees are small; shortcut accepted). Int requires an
  integer-form signed 64-bit number (`.`,`e`,`E` and range rejected) → `expected an integer-form
  signed 64-bit number`; kind mismatch → `expected Int` / `expected Bool` / `expected String` /
  `expected array` / `expected object`; lists > 1,000,000 → `list exceeds element limit`.
  Depth-256 trees walked by a recursive word sit exactly at the word-depth edge exactly as in
  the interpreter (both counters already pinned: M1 §3, this section); the fixture proves it.

## 7. `env.get` / `env.load`

Env state is **module-side**: a global `$zhex_env` (`$zhex_map_str_str`), seeded once by
`$zhex_env_init` — the first statement of `$zhex_main` (guard global) — from `env_environ`
(browser: the runner's injected table; WASI: real environ). Hosts are never mutated (Surprises
6). `env.get(name)` is a plain map lookup → `Option.Some/None` (never a Result).
`env.load(path)` (family 5): reads the file via `fs_read_file` — failure → `cannot read <q>:
open <path>: <err>` (interpreter code env-read; EISDIR renders verb `read`); then per line
(split on `"\n"` only): `TrimSpace` (the M2 §1 set), skip empty and whole-line `#` comments, cut
at the first `=`, trim key and value; malformed (no `=`, empty key, key containing space/tab) →
`line <n> of <q> is not a KEY=VALUE assignment: <q(trimmed line)>` (code env-parse; lines before
the failure stay applied); strip one surrounding matching quote pair (`"` or `'`, length ≥ 2);
**first assignment wins** — put only when the key is absent from the module-side map (startup
environ and earlier lines alike). Returns `ok(Unit)`.

## 8. Fence inventory and fail codes after M3

**Unfenced:** all of `file.*` except `free-space`; `read-line`; `for line in file.stream`;
`buffer.*`; `json.parse`/`encode`/`decode`; `duration.*`; `env.get`/`env.load`. (2026-11-15
amendment: `print` of a Duration stays FENCED — the language checker rejects it, and parity with
the interpreter wins; it was wrongly listed as unfenced.) **Still fenced** (code `not-available-in-build`, M2's milestone-neutral hint):
`file.free-space`; `xml.decode`; `date.* time.* instant.* timezone.* zoned.*` (D3); `console.arg
args-count exit fail` (M4); all `error.*` (caught err values stay opaque `$zhex_err` refs);
checked forms (`try`/`catch` on `list.append`, `array.new`, `array.resize`, `file.exists`);
word/record defaults, generic choices, re-partial application, function-valued flow steps;
`print` beyond Int/Bool/String/Unit/Duration. New fail codes (runner prints staged messages for
16–18):

| code | Zhex code | message | fires |
|---|---|---|---|
| 16 | unhandled-result | dynamic | top-level unhandled Result of any family (retires M2's code-12 reuse; bytes unchanged) |
| 17 | file-open / file-read / file-utf8 | dynamic | `file.stream` loop open/read/UTF-8 hard failures |
| 18 | file-exists | dynamic | plain `file.exists` stat failure other than not-found |

`$zhex_err` families (M2's field; metadata while `error.*` is fenced): 1 ParseError (M2), 2
InputError, 3 FileError, 4 BufferError, 5 Error (common — json, env, duration.parse).

## 9. Differential fixtures (`internal/wasm/testdata/`)

Browser fixtures run via `run.mjs` (virtual FS seeded from `--root`, stdin, env injection); WASI
fixtures run the same programs under `wasmtime --dir <tmp>::/`; each compares stdout bytes and
exit status against `zhex run`. Portable-path discipline (§2) applies to every file fixture.

1. `paths.zhex` — every §3 pure path word including edges (absolute child, `..` resolution,
   `.bashrc`, trailing slash/dot, bare name).
2. `files.zhex` — write/read round-trip, append, exists (file/folder/missing), size (folder →
   0), list (sorted, dotfile), make-folder (EEXIST caught, parents), delete, delete-folder
   (empty + recursive), move (file, folder, overwrite, both refusal messages).
3. `files-fail.zhex` — unhandled failures, exit 1: read missing, delete non-empty, list a file,
   read a directory, write into a missing folder (exact §1b-wrapped strings, e.g. `runtime
   error: cannot open "nope.txt": open nope.txt: no such file or directory`).
4. `stream.zhex` — multi-line, empty file, final line without newline; `stream-fail.zhex` → the
   staged open-failure line.
5. `readline.zhex` — prompt echo (no newline), two reads, EOF caught with fallback;
   `readline-eof.zhex` unhandled → `runtime error: end of input`.
6. `buffers.zhex` — add/count/line/set/to-text, two-name aliasing, caught out-of-range;
   `buffer-fail.zhex` → `runtime error: buffer index 5 is outside 0..2`.
7. `duration.zhex` — new/parse/to-string (0, negative, ns, `1.5h`, `2h45m`, `500ms`, µs with
   U+00B5), caught `invalid duration` (2026-11-15 amendment: no `print(duration)` — Duration print
is fenced, §8; the fixture renders with `duration.to-string`).
8. `json.zhex` — parse + match over all six variants, encode round-trip (key order, `1.50`
   preserved, exponents), tree mutation then encode; typed decode with records/Option/list/map,
   missing-field default fill, caught unknown field and duplicate key; a depth-256 tree walked
   by a recursive word at the edge; `json-fail.zhex` → `runtime error: invalid JSON: invalid
   character 'x' looking for beginning of value at ""`.
9. `env.zhex` — write `.env`, `env.load`, `get` Some/None, first-wins against a runner-injected
   key, set-but-empty, quote stripping, comments/blanks; `env-fail.zhex` — malformed line (`line
   3 of "bad.env" is not a KEY=VALUE assignment: "BAD LINE"`) and missing file (`cannot read
   ".env": open .env: no such file or directory`).
10. `text-stats-wasm.zhex`, `file-work-wasm.zhex` — style-adapted acceptance programs (Surprises
    1) under **both** skins.
11. Fence unit tests (emitter, no tools): `file.free-space`, `xml.decode`, `date.new`,
    `console.arg` → `not-available-in-build`.

## 10. Builder split (3-way, disjoint files; names first)

- **H — runtime host + M3 prelude**: `internal/lang/watnames_m3.go` (**checked in first**; E and
  T compile against it): skin enum, `$zhex_host_*` helper names, the §1a import and §1c WASI
  import names, errno constants, fail codes 16–18, families 2–5,
  `$zhex_file`/`$zhex_buffer`/Json/node/schema type and helper names, duration/path/json/env
  helper names. Then `internal/lang/watprelude_m3.go` (skin-independent text) and the two skin
  layers `internal/lang/wathost_browser.go` / `internal/lang/wathost_wasi.go` (same
  `$zhex_host_*` + `$zhex_fail`/`$zhex_fail_msg` names, §1c + Surprises 8).
- **E — emitter**: `internal/lang/wat.go` + `wat_test.go`: skin parameter on `CompileWat`,
  skin-layer concat in `emit()`, the §3–§7 builtin cases, `for lines` emission, the print
  raw-byte amendment (newline moves into the prelude), duration/buffer/file/Json reprs,
  `$zjson_schema<N>` builders, unhandled-path code 16.
- **T — CLI, runners, fixtures**: `internal/wasm/compile.go` (+ target), `cmd/zhex/main.go`
  usage, `compile_test.go`, `testdata/*` (fixtures, `run.mjs`: raw sink, virtual FS, stdin/env
  injection, codes 16–18 staged), the wasmtime runner helper. Skip-safe on missing tools.
- Shared surfaces are exactly: `watnames_m3.go` names (H defines first), the §1 import +
  verb/errno tables, the §8 code table (H and T), and the fixture list (T). Nobody edits another
  builder's files; the only existing files touched are `wat.go`/`wat_test.go` (E) and
  `compile.go`/`compile_test.go`/`testdata`/`cmd/zhex/main.go` (T). No new lang catalog codes.

## 11. Default choices taken (not pinned by #100/D3/D4)

- Errno-only imports with the `[u32 len][bytes]` scratch at offset 0 and host-side memory
  growth; no multi-value imports; `_start` export beside the M1 exports on the WASI skin.
- `env.print` raw-byte amendment (module owns the newline) rather than a second import — one
  seam, bytes identical.
- Env state module-side, hosts never mutated; buffered stdin module-side, chomp implemented once
  in the prelude.
- Json variants = plain padded structs (no rec group); one rec group for the internal
  node/schema types; one schema builder per call site, no caching.
- Linear-scan `ref.eq` memo for decode-defaults aliasing; Go json syntax-context table with an
  upgrade shortcut.
- Duration unboxed i64 ns; Go `ParseDuration`/`String` reimplemented; family Error.
- Fail codes 16–18, families 2–5; M2's code-12 unhandled reuse retired. (2026-11-15 note:
  sanctioned M3 cross-file touch — `watnames_m2_test.go`'s exact-set check became
  membership-based to accept the M3 additions.)
- Virtual FS in `run.mjs` (in-memory tree, no permissions); stdout + exit status is the
  differential assertion; final-tree state is asserted through the program's own output
  (list/exists).
- Acceptance examples style-adapted rather than unfencing `console.*` early.
