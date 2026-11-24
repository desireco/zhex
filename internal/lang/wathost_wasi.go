package lang

import (
	"fmt"
	"strings"
)

// The WASI host skin (DESIGN-M3 §1c, Surprises 8): thirteen
// wasi_snapshot_preview1 imports whose adapter functions implement the same
// fifteen $zhex_host_* names the browser skin imports, plus the stderr fail
// path (fd_write to fd 2 + proc_exit(1), because traps cannot carry staged
// messages under the wasmtime CLI) and the _start export. Guest paths
// resolve against the first preopen (fd 3) with any leading "/" stripped;
// shortcut: the single-preopen assumption is fixed (upgrade: a preopen
// table) per DESIGN-M3 §1c. Errno values cross unchanged in WASI preview 1
// numbering; every rights/oflags combination below was verified against
// wasmtime 50 on Linux (read/dir opens with 0x40006/0x40800, write opens
// with 0x40084, EINVAL only for malformed paths/flags).
//
// Linear-memory layout the adapters own (the module's staging slot at
// offset 0 is reserved for the §1a scratch protocol):
//
//	0..4095        caller scratch + module-staged strings
//	4096..8191     adapter path copies (two slots, 2KB each)
//	8192..8255     fd_filestat_get buffer
//	8256..8271     iovec {ptr,len} + nwritten/nread
//	8448..8479     misc state (fd out, rd_len, dirent bytes, readdir cookie)
//	65536..0x100000  transient bulk (stdout flush, read_file relay, environ, readdir)
//
// Persistent file handles, pending input, and stdout live in GC objects;
// linear memory is reusable staging for synchronous host calls only.

// watWasiImports: the thirteen wasi_snapshot_preview1 imports (§1c).
const watWasiImports = `
;; wasi host imports (DESIGN-M3 1c)
(import "wasi_snapshot_preview1" "fd_write" (func $wasi.fd_write (param i32 i32 i32 i32) (result i32)))
(import "wasi_snapshot_preview1" "fd_read" (func $wasi.fd_read (param i32 i32 i32 i32) (result i32)))
(import "wasi_snapshot_preview1" "path_open" (func $wasi.path_open (param i32 i32 i32 i32 i32 i64 i64 i32 i32) (result i32)))
(import "wasi_snapshot_preview1" "fd_close" (func $wasi.fd_close (param i32) (result i32)))
(import "wasi_snapshot_preview1" "fd_filestat_get" (func $wasi.fd_filestat_get (param i32 i32) (result i32)))
(import "wasi_snapshot_preview1" "fd_readdir" (func $wasi.fd_readdir (param i32 i32 i32 i64 i32) (result i32)))
(import "wasi_snapshot_preview1" "path_create_directory" (func $wasi.path_create_directory (param i32 i32 i32) (result i32)))
(import "wasi_snapshot_preview1" "path_unlink_file" (func $wasi.path_unlink_file (param i32 i32 i32) (result i32)))
(import "wasi_snapshot_preview1" "path_remove_directory" (func $wasi.path_remove_directory (param i32 i32 i32) (result i32)))
(import "wasi_snapshot_preview1" "path_rename" (func $wasi.path_rename (param i32 i32 i32 i32 i32 i32) (result i32)))
(import "wasi_snapshot_preview1" "environ_sizes_get" (func $wasi.environ_sizes_get (param i32 i32) (result i32)))
(import "wasi_snapshot_preview1" "environ_get" (func $wasi.environ_get (param i32 i32) (result i32)))
(import "wasi_snapshot_preview1" "proc_exit" (func $wasi.proc_exit (param i32)))
`

// watWasiFuncs assembles the adapter layer: shared helpers, the fifteen
// $zhex_host_* implementations, the fail pair, and _start. The fixed fail
// message funcs are generated from committed literals by watWasiFailTexts.
var watWasiFuncs = strings.Join([]string{
	watWasiCore,
	watWasiFailTexts(),
}, "")

// watWasiCore is the hand-written adapter body.
const watWasiCore = `
;; ---- shared helpers ----

(func $wasi_ensure (param $end i32)
  (local $pages i32)
  (local.set $pages (i32.div_u (i32.add (local.get $end) (i32.const 65535)) (i32.const 65536)))
  (if (i32.gt_u (local.get $pages) (memory.size))
    (then (drop (memory.grow (i32.sub (local.get $pages) (memory.size)))))))

(func $wasi_write (param $fd i32) (param $ptr i32) (param $len i32) (result i32)
  (i32.store (i32.const 8256) (local.get $ptr))
  (i32.store (i32.const 8260) (local.get $len))
  (call $wasi.fd_write (local.get $fd) (i32.const 8256) (i32.const 1) (i32.const 8264)))

(type $wasi_file_slot (struct (field $fd i32)
  (field $buf (mut (ref null $zhex_arr_i32)))
  (field $len (mut i32)) (field $pos (mut i32))))
(type $wasi_file_slots (array (mut (ref null $wasi_file_slot))))
(global $wasi_files (mut (ref null $wasi_file_slots)) (ref.null $wasi_file_slots))

(func $wasi_file_emit (param $slot (ref $wasi_file_slot)) (param $start i32) (param $len i32)
  (local $i i32)
  (call $wasi_ensure (i32.add (i32.const 4) (local.get $len)))
  (i32.store (i32.const 0) (local.get $len))
  (block $staged
    (loop $stage
      (br_if $staged (i32.ge_u (local.get $i) (local.get $len)))
      (i32.store8 (i32.add (i32.const 4) (local.get $i))
        (array.get $zhex_arr_i32 (ref.as_non_null (struct.get $wasi_file_slot $buf (local.get $slot)))
          (i32.add (local.get $start) (local.get $i))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $stage))))

;; $wasi_bytes_eq compares len bytes; $wasi_name_lt is byte-wise unsigned
;; lexicographic with the shorter prefix first (fs_list's sorted order).
(func $wasi_bytes_eq (param $a i32) (param $b i32) (param $len i32) (result i32)
  (local $i i32)
  (block $diff
    (loop $cmp
      (br_if $diff (i32.ne (i32.load8_u (i32.add (local.get $a) (local.get $i)))
                           (i32.load8_u (i32.add (local.get $b) (local.get $i)))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br_if $cmp (i32.lt_u (local.get $i) (local.get $len)))))
  (i32.eq (local.get $i) (local.get $len)))

(func $wasi_name_lt (param $a i32) (param $alen i32) (param $b i32) (param $blen i32) (result i32)
  (local $i i32) (local $n i32) (local $ca i32) (local $cb i32)
  (local.set $n (select (local.get $alen) (local.get $blen) (i32.lt_u (local.get $alen) (local.get $blen))))
  (block $decided
    (loop $cmp
      (br_if $decided (i32.ge_u (local.get $i) (local.get $n)))
      (local.set $ca (i32.load8_u (i32.add (local.get $a) (local.get $i))))
      (local.set $cb (i32.load8_u (i32.add (local.get $b) (local.get $i))))
      (br_if $decided (i32.ne (local.get $ca) (local.get $cb)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $cmp)))
  (if (i32.lt_u (local.get $i) (local.get $n))
    (then (return (i32.lt_u (local.get $ca) (local.get $cb)))))
  (return (i32.lt_u (local.get $alen) (local.get $blen))))

;; $wasi_is_dir stats a path by opening it (oflags 0 works on files and
;; directories alike) and reading the filestat type; any failure is "not a
;; directory".
(func $wasi_is_dir (param $ptr i32) (param $len i32) (result i32)
  (local $e i32)
  ;; oflags DIRECTORY: only a directory opens; files fail ENOTDIR
  (local.set $e (call $wasi.path_open (i32.const 3) (i32.const 0) (local.get $ptr) (local.get $len)
    (i32.const 4) (i64.const 0x40800) (i64.const 0x40800) (i32.const 0) (i32.const 8448)))
  (if (i32.ne (local.get $e) (i32.const 0)) (then (return (i32.const 0))))
  (local.set $e (call $wasi.fd_filestat_get (i32.load (i32.const 8448)) (i32.const 8192)))
  (drop (call $wasi.fd_close (i32.load (i32.const 8448))))
  (if (i32.ne (local.get $e) (i32.const 0)) (then (return (i32.const 0))))
  (i32.eq (i32.load8_u (i32.const 8208)) (i32.const 3)))

;; $wasi_load_environ gathers the environment once: the pointer array lands
;; at 65536, the string bytes at 65536 + count*4. Returns the pair count or
;; a negative errno.
(func $wasi_load_environ (result i32)
  (local $e i32) (local $count i32) (local $size i32)
  (local.set $e (call $wasi.environ_sizes_get (i32.const 8448) (i32.const 8452)))
  (if (i32.ne (local.get $e) (i32.const 0)) (then (return (i32.sub (i32.const 0) (local.get $e)))))
  (local.set $count (i32.load (i32.const 8448)))
  (local.set $size (i32.load (i32.const 8452)))
  (call $wasi_ensure (i32.add (i32.const 65536) (i32.add (i32.mul (local.get $count) (i32.const 4)) (local.get $size))))
  (local.set $e (call $wasi.environ_get (i32.const 65536) (i32.add (i32.const 65536) (i32.mul (local.get $count) (i32.const 4)))))
  (if (i32.ne (local.get $e) (i32.const 0)) (then (return (i32.sub (i32.const 0) (local.get $e)))))
  (local.get $count))

;; $wasi_read_dirents drains a directory fd into 65536.., retrying with a
;; doubled buffer on EINVAL (buffer too small) and resuming from the last
;; complete dirent's d_next cookie. Complete dirents accumulate; the total
;; byte length lands at 8456. Returns errno.
(func $wasi_read_dirents (param $fd i32) (result i32)
  (local $e i32) (local $cap i32) (local $space i32) (local $used i32)
  (local $off i32) (local $last i32) (local $nl i32)
  (i64.store (i32.const 8460) (i64.const 0))
  (i32.store (i32.const 8456) (i32.const 0))
  (local.set $cap (i32.const 65536))
  (block $done
    (loop $again
      (call $wasi_ensure (i32.add (i32.const 65536) (local.get $cap)))
      (local.set $space (i32.sub (local.get $cap) (i32.load (i32.const 8456))))
      (local.set $e (call $wasi.fd_readdir (local.get $fd) (i32.add (i32.const 65536) (i32.load (i32.const 8456)))
        (local.get $space) (i64.load (i32.const 8460)) (i32.const 8452)))
      (if (i32.eq (local.get $e) (i32.const 28))
        (then (local.set $cap (i32.mul (local.get $cap) (i32.const 2))) (br $again)))
      (if (i32.ne (local.get $e) (i32.const 0)) (then (return (local.get $e))))
      (local.set $used (i32.load (i32.const 8452)))
      ;; a short read means the listing is complete; account for its bytes
      (if (i32.lt_u (local.get $used) (local.get $space))
        (then
          (i32.store (i32.const 8456) (i32.add (i32.load (i32.const 8456)) (local.get $used)))
          (return (i32.const 0))))
      ;; walk the complete dirents; drop any partial tail
      (local.set $off (i32.const 0))
      (local.set $last (i32.const -1))
      (block $walked
        (loop $walk
          (br_if $walked (i32.ge_u (local.get $off) (local.get $used)))
          (local.set $nl (i32.load (i32.add (i32.const 65552) (local.get $off))))
          (if (i32.gt_u (i32.add (local.get $off) (i32.add (i32.const 24) (local.get $nl))) (local.get $used))
            (then (br $walked)))
          (local.set $last (local.get $off))
          (local.set $off (i32.add (local.get $off) (i32.add (i32.const 24) (local.get $nl))))
          (br $walk)))
      (if (i32.eq (local.get $last) (i32.const -1))
        (then ;; not even one entry fit; grow and retry the same cookie
          (local.set $cap (i32.mul (local.get $cap) (i32.const 2))) (br $again)))
      ;; resume after the last complete entry
      (i64.store (i32.const 8460) (i64.load (i32.add (i32.const 65536) (local.get $last))))
      (i32.store (i32.const 8456) (i32.add (i32.load (i32.const 8456)) (local.get $off)))
      (local.set $cap (i32.mul (local.get $cap) (i32.const 2)))
      (br $again)))
  (i32.const 0))

;; $wasi_rmrf removes a tree best-effort, first failure reported (§2): list
;; the folder, recurse into directories, unlink files, then remove the
;; folder itself. Child paths are built in the second work slot; shortcut:
;; 2KB path cap per level (upgrade: dynamic buffers).
(func $wasi_rmrf (param $ptr i32) (param $len i32) (result i32)
  (local $e i32) (local $fd i32) (local $off i32) (local $nl i32)
  (local $child i32) (local $type i32)
  (local.set $e (call $wasi.path_open (i32.const 3) (i32.const 0) (local.get $ptr) (local.get $len)
    (i32.const 4) (i64.const 0x40800) (i64.const 0x40800) (i32.const 0) (i32.const 8448)))
  (if (i32.ne (local.get $e) (i32.const 0)) (then (return (local.get $e))))
  (local.set $fd (i32.load (i32.const 8448)))
  (local.set $e (call $wasi_read_dirents (local.get $fd)))
  (drop (call $wasi.fd_close (local.get $fd)))
  (if (i32.ne (local.get $e) (i32.const 0)) (then (return (local.get $e))))
  (block $listed
    (loop $entry
      (br_if $listed (i32.ge_u (local.get $off) (i32.load (i32.const 8456))))
      (local.set $nl (i32.load (i32.add (i32.const 65552) (local.get $off))))
      (local.set $type (i32.load (i32.add (i32.const 65556) (local.get $off))))
      ;; skip "." and ".."
      (if (i32.and (i32.eq (local.get $nl) (i32.const 1))
                   (i32.eq (i32.load8_u (i32.add (i32.const 65560) (local.get $off))) (i32.const 46)))
        (then (local.set $off (i32.add (local.get $off) (i32.add (i32.const 24) (local.get $nl)))) (br $entry)))
      (if (i32.and (i32.and (i32.eq (local.get $nl) (i32.const 2))
                            (i32.eq (i32.load8_u (i32.add (i32.const 65560) (local.get $off))) (i32.const 46)))
                   (i32.eq (i32.load8_u (i32.add (i32.const 65561) (local.get $off))) (i32.const 46)))
        (then (local.set $off (i32.add (local.get $off) (i32.add (i32.const 24) (local.get $nl)))) (br $entry)))
      ;; child = <ptr>/<name> in work slot 2
      (memory.copy (i32.const 6144) (local.get $ptr) (local.get $len))
      (i32.store8 (i32.add (i32.const 6144) (local.get $len)) (i32.const 47))
      (memory.copy (i32.add (i32.const 6145) (local.get $len))
        (i32.add (i32.const 65560) (local.get $off)) (local.get $nl))
      (local.set $child (i32.add (local.get $len) (i32.add (i32.const 1) (local.get $nl))))
      (if (i32.eq (local.get $type) (i32.const 2))
        (then (local.set $e (call $wasi_rmrf (i32.const 6144) (local.get $child))))
        (else (local.set $e (call $wasi.path_unlink_file (i32.const 3) (i32.const 6144) (local.get $child)))))
      (if (i32.ne (local.get $e) (i32.const 0)) (then (return (local.get $e))))
      (local.set $off (i32.add (local.get $off) (i32.add (i32.const 24) (local.get $nl))))
      (br $entry)))
  (call $wasi.path_remove_directory (i32.const 3) (local.get $ptr) (local.get $len)))

;; ---- batched stdout (#187) ----
;; Buffered bytes and their fill count live outside linear scratch memory:
;; print/file staging can overwrite any prefix of that memory. Bytes cross
;; to fd_write when the 32KB buffer fills, before stderr diagnostics, before a stdin read
;; (prompt visibility), and at _start's end — stdout ordering is
;; unchanged. shortcut: an unexpected native trap (failed cast) loses
;; at most the pending 32KB; the staged fail codes all flush (upgrade:
;; wasm EH to flush on trap).

(global $wasi_out_buf (mut (ref null $zhex_arr_i32)) (ref.null $zhex_arr_i32))
(global $wasi_out_len (mut i32) (i32.const 0))

(func $wasi_out_flush
  (local $i i32)
  (if (global.get $wasi_out_len)
    (then
      (call $wasi_ensure (i32.add (i32.const 65536) (global.get $wasi_out_len)))
      (block $staged
        (loop $stage
          (br_if $staged (i32.ge_u (local.get $i) (global.get $wasi_out_len)))
          (i32.store8 (i32.add (i32.const 65536) (local.get $i))
            (array.get $zhex_arr_i32 (ref.as_non_null (global.get $wasi_out_buf)) (local.get $i)))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $stage)))
      (drop (call $wasi_write (i32.const 1) (i32.const 65536) (global.get $wasi_out_len)))
      (global.set $wasi_out_len (i32.const 0)))))

(func $wasi_out_write (param $ptr i32) (param $len i32)
  (local $space i32)
  (local $i i32)
  (local $offset i32)
  (local $snapshot (ref null $zhex_arr_i32))
  (if (ref.is_null (global.get $wasi_out_buf))
    (then (global.set $wasi_out_buf (array.new_default $zhex_arr_i32 (i32.const 32768)))))
  ;; A flush reuses linear staging, so preserve the current payload first
  ;; whenever this call will cross a buffer boundary. Small writes that
  ;; fit need no allocation.
  (if (i32.gt_u (local.get $len) (i32.sub (i32.const 32768) (global.get $wasi_out_len)))
    (then
      (local.set $snapshot (array.new_default $zhex_arr_i32 (local.get $len)))
      (block $saved
        (loop $save
          (br_if $saved (i32.ge_u (local.get $i) (local.get $len)))
          (array.set $zhex_arr_i32 (ref.as_non_null (local.get $snapshot)) (local.get $i)
            (i32.load8_u (i32.add (local.get $ptr) (local.get $i))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $save)))))
  (block $done
    (loop $again
      (br_if $done (i32.eqz (local.get $len)))
      (local.set $space (i32.sub (i32.const 32768) (global.get $wasi_out_len)))
      (if (i32.ge_u (local.get $space) (local.get $len))
        (then (local.set $space (local.get $len))))
      (local.set $i (i32.const 0))
      (block $copied
        (loop $copy
          (br_if $copied (i32.ge_u (local.get $i) (local.get $space)))
          (array.set $zhex_arr_i32 (ref.as_non_null (global.get $wasi_out_buf))
            (i32.add (global.get $wasi_out_len) (local.get $i))
            (if (result i32) (ref.is_null (local.get $snapshot))
              (then (i32.load8_u (i32.add (local.get $ptr) (i32.add (local.get $offset) (local.get $i)))))
              (else (array.get $zhex_arr_i32 (ref.as_non_null (local.get $snapshot))
                (i32.add (local.get $offset) (local.get $i))))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $copy)))
      (global.set $wasi_out_len (i32.add (global.get $wasi_out_len) (local.get $space)))
      (local.set $offset (i32.add (local.get $offset) (local.get $space)))
      (local.set $len (i32.sub (local.get $len) (local.get $space)))
      (br_if $done (i32.eqz (local.get $len)))
      (call $wasi_out_flush)
      (br $again))))

;; ---- the fifteen $zhex_host_* adapters ----

(func $zhex_host_print (param $ptr i32) (param $len i32)
  (call $wasi_out_write (local.get $ptr) (local.get $len)))

(func $env.print (param $p i32) (param $l i32)
  (call $wasi_out_write (local.get $p) (local.get $l)))

(func $zhex_host_stdin_read (param $ptr i32) (param $cap i32) (result i32)
  (local $e i32)
  (call $wasi_out_flush)
  (i32.store (i32.const 8256) (local.get $ptr))
  (i32.store (i32.const 8260) (local.get $cap))
  (local.set $e (call $wasi.fd_read (i32.const 0) (i32.const 8256) (i32.const 1) (i32.const 8264)))
  (if (i32.ne (local.get $e) (i32.const 0)) (then (return (i32.const -1))))
  (i32.load (i32.const 8264)))

(func $zhex_host_fs_read_file (param $pptr i32) (param $plen i32) (result i32)
  (local $e i32) (local $fd i32) (local $total i32) (local $n i32)
  (memory.copy (i32.const 4096) (local.get $pptr) (local.get $plen))
  ;; wasmtime grants the open on files and directories alike (oflags and
  ;; rights are enforced at use), so: open dir-capable, let the filestat
  ;; decide — a directory fails at Go's read stage with EISDIR (Surprises 3)
  ;; — and a regular file reopens with read rights below
  (local.set $e (call $wasi.path_open (i32.const 3) (i32.const 0) (i32.const 4096) (local.get $plen)
    (i32.const 4) (i64.const 0x40800) (i64.const 0x40800) (i32.const 0) (i32.const 8448)))
  (if (i32.ne (local.get $e) (i32.const 0)) (then (return (local.get $e))))
  (local.set $fd (i32.load (i32.const 8448)))
  (local.set $e (call $wasi.fd_filestat_get (local.get $fd) (i32.const 8192)))
  (if (i32.ne (local.get $e) (i32.const 0))
    (then (drop (call $wasi.fd_close (local.get $fd))) (return (local.get $e))))
  (if (i32.eq (i32.load8_u (i32.const 8208)) (i32.const 3))
    (then (drop (call $wasi.fd_close (local.get $fd))) (return (i32.const 31))))
  (drop (call $wasi.fd_close (local.get $fd)))
  (local.set $e (call $wasi.path_open (i32.const 3) (i32.const 0) (i32.const 4096) (local.get $plen)
    (i32.const 0) (i64.const 0x40006) (i64.const 0x40006) (i32.const 0) (i32.const 8448)))
  (if (i32.ne (local.get $e) (i32.const 0)) (then (return (local.get $e))))
  (local.set $fd (i32.load (i32.const 8448)))
  (block $read_done
    (loop $read
      (call $wasi_ensure (i32.add (i32.const 131072) (local.get $total)))
      (i32.store (i32.const 8256) (i32.add (i32.const 65536) (local.get $total)))
      (i32.store (i32.const 8260) (i32.const 65536))
      (local.set $e (call $wasi.fd_read (local.get $fd) (i32.const 8256) (i32.const 1) (i32.const 8264)))
      (if (i32.ne (local.get $e) (i32.const 0))
        (then (drop (call $wasi.fd_close (local.get $fd))) (return (local.get $e))))
      (local.set $n (i32.load (i32.const 8264)))
      (local.set $total (i32.add (local.get $total) (local.get $n)))
      (br_if $read (i32.gt_u (local.get $n) (i32.const 0)))))
  (i32.store (i32.const 0) (local.get $total))
  (memory.copy (i32.const 4) (i32.const 65536) (local.get $total))
  (drop (call $wasi.fd_close (local.get $fd)))
  (i32.const 0))

(func $zhex_host_fs_write_file (param $pptr i32) (param $plen i32) (param $dptr i32) (param $dlen i32) (param $append i32) (result i32)
  (local $e i32) (local $fd i32)
  (memory.copy (i32.const 4096) (local.get $pptr) (local.get $plen))
  (local.set $e (call $wasi.path_open (i32.const 3) (i32.const 0) (i32.const 4096) (local.get $plen)
    (select (i32.const 9) (i32.const 1) (i32.eqz (local.get $append)))
    (i64.const 0x40084) (i64.const 0x40084)
    (select (i32.const 1) (i32.const 0) (local.get $append))
    (i32.const 8448)))
  (if (i32.ne (local.get $e) (i32.const 0)) (then (return (local.get $e))))
  (local.set $fd (i32.load (i32.const 8448)))
  (i32.store (i32.const 8256) (local.get $dptr))
  (i32.store (i32.const 8260) (local.get $dlen))
  (local.set $e (call $wasi.fd_write (local.get $fd) (i32.const 8256) (i32.const 1) (i32.const 8264)))
  (if (i32.ne (local.get $e) (i32.const 0))
    (then (drop (call $wasi.fd_close (local.get $fd))) (return (local.get $e))))
  (call $wasi.fd_close (local.get $fd)))

(func $zhex_host_fs_open_read (param $pptr i32) (param $plen i32) (result i32)
  (local $e i32) (local $slot i32)
  (if (ref.is_null (global.get $wasi_files))
    (then (global.set $wasi_files (array.new_default $wasi_file_slots (i32.const 8)))))
  (block $found
    (loop $scan
      (br_if $found (ref.is_null (array.get $wasi_file_slots
        (ref.as_non_null (global.get $wasi_files)) (local.get $slot))))
      (local.set $slot (i32.add (local.get $slot) (i32.const 1)))
      (if (i32.lt_u (local.get $slot) (i32.const 8)) (then (br $scan)))
      (return (i32.const -33))))
  (memory.copy (i32.const 4096) (local.get $pptr) (local.get $plen))
  (local.set $e (call $wasi.path_open (i32.const 3) (i32.const 0) (i32.const 4096) (local.get $plen)
    (i32.const 0) (i64.const 0x40006) (i64.const 0x40006) (i32.const 0) (i32.const 8448)))
  (if (i32.ne (local.get $e) (i32.const 0)) (then (return (i32.sub (i32.const 0) (local.get $e)))))
  (array.set $wasi_file_slots (ref.as_non_null (global.get $wasi_files)) (local.get $slot)
    (struct.new $wasi_file_slot (i32.load (i32.const 8448))
      (array.new_default $zhex_arr_i32 (i32.const 65536)) (i32.const 0) (i32.const 0)))
  (local.get $slot))

;; Pending bytes stay in a GC buffer per handle. Linear memory holds only
;; one synchronous fd_read payload or the returned scratch line.
(func $zhex_host_fs_read_line (param $handle i32) (result i32)
  (local $slot (ref null $wasi_file_slot))
  (local $buf (ref null $zhex_arr_i32)) (local $fresh (ref null $zhex_arr_i32))
  (local $fd i32) (local $buflen i32) (local $bufpos i32)
  (local $k i32) (local $e i32) (local $n i32) (local $cap i32) (local $i i32)
  (if (i32.ge_u (local.get $handle) (i32.const 8)) (then (return (i32.const -9))))
  (if (ref.is_null (global.get $wasi_files)) (then (return (i32.const -9))))
  (local.set $slot (array.get $wasi_file_slots (ref.as_non_null (global.get $wasi_files)) (local.get $handle)))
  (if (ref.is_null (local.get $slot)) (then (return (i32.const -9))))
  (local.set $fd (struct.get $wasi_file_slot $fd (ref.as_non_null (local.get $slot))))
  (local.set $buf (struct.get $wasi_file_slot $buf (ref.as_non_null (local.get $slot))))
  (local.set $buflen (struct.get $wasi_file_slot $len (ref.as_non_null (local.get $slot))))
  (local.set $bufpos (struct.get $wasi_file_slot $pos (ref.as_non_null (local.get $slot))))
  (block $newline
    (loop $scan
      (local.set $k (local.get $bufpos))
      (block $miss
        (loop $cmp
          (br_if $miss (i32.ge_u (local.get $k) (local.get $buflen)))
          (br_if $newline (i32.eq (array.get $zhex_arr_i32 (ref.as_non_null (local.get $buf)) (local.get $k)) (i32.const 10)))
          (local.set $k (i32.add (local.get $k) (i32.const 1)))
          (br $cmp)))
      (if (i32.gt_u (local.get $bufpos) (i32.const 0))
        (then
          (array.copy $zhex_arr_i32 $zhex_arr_i32
            (ref.as_non_null (local.get $buf)) (i32.const 0)
            (ref.as_non_null (local.get $buf)) (local.get $bufpos)
            (i32.sub (local.get $buflen) (local.get $bufpos)))
          (local.set $buflen (i32.sub (local.get $buflen) (local.get $bufpos)))
          (local.set $bufpos (i32.const 0))))
      (local.set $cap (array.len (ref.as_non_null (local.get $buf))))
      (if (i32.ge_u (local.get $buflen) (local.get $cap))
        (then
          (local.set $cap (i32.add (local.get $cap) (local.get $cap)))
          (local.set $fresh (array.new_default $zhex_arr_i32 (local.get $cap)))
          (array.copy $zhex_arr_i32 $zhex_arr_i32 (ref.as_non_null (local.get $fresh)) (i32.const 0)
            (ref.as_non_null (local.get $buf)) (i32.const 0) (local.get $buflen))
          (local.set $buf (local.get $fresh))
          (struct.set $wasi_file_slot $buf (ref.as_non_null (local.get $slot)) (local.get $buf))))
      (local.set $cap (i32.sub (local.get $cap) (local.get $buflen)))
      (if (i32.gt_u (local.get $cap) (i32.const 65536)) (then (local.set $cap (i32.const 65536))))
      (call $wasi_ensure (i32.add (i32.const 65536) (local.get $cap)))
      (i32.store (i32.const 8256) (i32.const 65536))
      (i32.store (i32.const 8260) (local.get $cap))
      (local.set $e (call $wasi.fd_read (local.get $fd) (i32.const 8256) (i32.const 1) (i32.const 8264)))
      (if (i32.ne (local.get $e) (i32.const 0))
        (then
          (struct.set $wasi_file_slot $len (ref.as_non_null (local.get $slot)) (local.get $buflen))
          (struct.set $wasi_file_slot $pos (ref.as_non_null (local.get $slot)) (local.get $bufpos))
          (return (i32.sub (i32.const 0) (local.get $e)))))
      (local.set $n (i32.load (i32.const 8264)))
      (if (i32.eqz (local.get $n))
        (then
          (struct.set $wasi_file_slot $len (ref.as_non_null (local.get $slot)) (i32.const 0))
          (struct.set $wasi_file_slot $pos (ref.as_non_null (local.get $slot)) (i32.const 0))
          (if (i32.gt_u (local.get $buflen) (local.get $bufpos))
            (then
              (call $wasi_file_emit (ref.as_non_null (local.get $slot)) (local.get $bufpos)
                (i32.sub (local.get $buflen) (local.get $bufpos)))
              (return (i32.const 1)))
            (else (return (i32.const 0))))))
      (local.set $i (i32.const 0))
      (block $copied
        (loop $copy
          (br_if $copied (i32.ge_u (local.get $i) (local.get $n)))
          (array.set $zhex_arr_i32 (ref.as_non_null (local.get $buf)) (i32.add (local.get $buflen) (local.get $i))
            (i32.load8_u (i32.add (i32.const 65536) (local.get $i))))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $copy)))
      (local.set $buflen (i32.add (local.get $buflen) (local.get $n)))
      (br $scan)))
  (call $wasi_file_emit (ref.as_non_null (local.get $slot)) (local.get $bufpos)
    (i32.add (i32.sub (local.get $k) (local.get $bufpos)) (i32.const 1)))
  (struct.set $wasi_file_slot $len (ref.as_non_null (local.get $slot)) (local.get $buflen))
  (struct.set $wasi_file_slot $pos (ref.as_non_null (local.get $slot)) (i32.add (local.get $k) (i32.const 1)))
  (i32.const 1))

(func $zhex_host_fs_close (param $handle i32) (result i32)
  (local $slot (ref null $wasi_file_slot))
  (if (i32.ge_u (local.get $handle) (i32.const 8)) (then (return (i32.const 0))))
  (if (ref.is_null (global.get $wasi_files)) (then (return (i32.const 0))))
  (local.set $slot (array.get $wasi_file_slots (ref.as_non_null (global.get $wasi_files)) (local.get $handle)))
  (if (ref.is_null (local.get $slot)) (then (return (i32.const 0))))
  (array.set $wasi_file_slots (ref.as_non_null (global.get $wasi_files)) (local.get $handle) (ref.null $wasi_file_slot))
  (drop (call $wasi.fd_close (struct.get $wasi_file_slot $fd (ref.as_non_null (local.get $slot)))))
  (i32.const 0))

(func $zhex_host_fs_stat (param $pptr i32) (param $plen i32) (result i32)
  (local $e i32) (local $fd i32)
  (memory.copy (i32.const 4096) (local.get $pptr) (local.get $plen))
  (local.set $e (call $wasi.path_open (i32.const 3) (i32.const 0) (i32.const 4096) (local.get $plen)
    (i32.const 0) (i64.const 0x40006) (i64.const 0x40006) (i32.const 0) (i32.const 8448)))
  (if (i32.ne (local.get $e) (i32.const 0)) (then (return (local.get $e))))
  (local.set $fd (i32.load (i32.const 8448)))
  (local.set $e (call $wasi.fd_filestat_get (local.get $fd) (i32.const 8192)))
  (drop (call $wasi.fd_close (local.get $fd)))
  (if (i32.ne (local.get $e) (i32.const 0)) (then (return (local.get $e))))
  (if (i32.eq (i32.load8_u (i32.const 8208)) (i32.const 3))
    (then
      (i32.store (i32.const 0) (i32.const 12))
      (i32.store (i32.const 4) (i32.const 1))
      (i64.store (i32.const 8) (i64.const 0)))
    (else
      (i32.store (i32.const 0) (i32.const 12))
      (i32.store (i32.const 4) (i32.const 0))
      (i64.store (i32.const 8) (i64.load (i32.const 8224)))))
  (i32.const 0))

(func $zhex_host_fs_list (param $pptr i32) (param $plen i32) (result i32)
  (local $e i32) (local $fd i32) (local $off i32) (local $nl i32)
  (local $count i32) (local $tbase i32) (local $cursor i32) (local $i i32) (local $j i32)
  (local $total i32) (local $tmp i32) (local $src i32)
  (memory.copy (i32.const 4096) (local.get $pptr) (local.get $plen))
  (local.set $e (call $wasi.path_open (i32.const 3) (i32.const 0) (i32.const 4096) (local.get $plen)
    (i32.const 4) (i64.const 0x40800) (i64.const 0x40800) (i32.const 0) (i32.const 8448)))
  (if (i32.ne (local.get $e) (i32.const 0)) (then (return (local.get $e))))
  (local.set $fd (i32.load (i32.const 8448)))
  (local.set $e (call $wasi_read_dirents (local.get $fd)))
  (drop (call $wasi.fd_close (local.get $fd)))
  ;; reading dirents of a plain file reports EBADF; Go's os.ReadDir fails
  ;; the same call with ENOTDIR at the open stage
  (if (i32.eq (local.get $e) (i32.const 8)) (then (local.set $e (i32.const 54))))
  (if (i32.ne (local.get $e) (i32.const 0)) (then (return (local.get $e))))
  ;; pass 1: a table of {u32 namlen, u32 name source offset} slots past the
  ;; dirent bytes; "." and ".." are skipped. Names are compared in place and
  ;; only copied after sorting, so table and names never overlap.
  (local.set $tbase (i32.and (i32.add (i32.add (i32.const 65536) (i32.load (i32.const 8456))) (i32.const 7)) (i32.const -8)))
  (local.set $off (i32.const 0))
  (block $p1done
    (loop $p1
      (br_if $p1done (i32.ge_u (local.get $off) (i32.load (i32.const 8456))))
      (local.set $nl (i32.load (i32.add (i32.const 65552) (local.get $off))))
      (if (i32.and (i32.eq (local.get $nl) (i32.const 1))
                   (i32.eq (i32.load8_u (i32.add (i32.const 65560) (local.get $off))) (i32.const 46)))
        (then (local.set $off (i32.add (local.get $off) (i32.add (i32.const 24) (local.get $nl)))) (br $p1)))
      (if (i32.and (i32.and (i32.eq (local.get $nl) (i32.const 2))
                            (i32.eq (i32.load8_u (i32.add (i32.const 65560) (local.get $off))) (i32.const 46)))
                   (i32.eq (i32.load8_u (i32.add (i32.const 65561) (local.get $off))) (i32.const 46)))
        (then (local.set $off (i32.add (local.get $off) (i32.add (i32.const 24) (local.get $nl)))) (br $p1)))
      (i32.store (i32.add (local.get $tbase) (i32.mul (local.get $count) (i32.const 8))) (local.get $nl))
      (i32.store (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $count) (i32.const 8)) (i32.const 4)))
        (i32.add (i32.const 65560) (local.get $off)))
      (local.set $count (i32.add (local.get $count) (i32.const 1)))
      (local.set $off (i32.add (local.get $off) (i32.add (i32.const 24) (local.get $nl))))
      (br $p1)))
  ;; pass 2: insertion sort the table by name bytes (byte-sorted order)
  (local.set $i (i32.const 1))
  (block $sorted
    (loop $sort
      (br_if $sorted (i32.ge_u (local.get $i) (local.get $count)))
      (local.set $j (local.get $i))
      (block $placed
        (loop $shift
          (br_if $placed (i32.eqz (local.get $j)))
          (if (i32.eqz (call $wasi_name_lt
                (i32.load (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $j) (i32.const 8)) (i32.const 4))))
                (i32.load (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $j) (i32.const 8)) (i32.const 0))))
                (i32.load (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $j) (i32.const 8)) (i32.const -4))))
                (i32.load (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $j) (i32.const 8)) (i32.const -8))))))
            (then (br $placed)))
          (local.set $tmp (i32.load (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $j) (i32.const 8)) (i32.const 0)))))
          (i32.store (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $j) (i32.const 8)) (i32.const 0)))
            (i32.load (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $j) (i32.const 8)) (i32.const -8)))))
          (i32.store (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $j) (i32.const 8)) (i32.const -8))) (local.get $tmp))
          (local.set $tmp (i32.load (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $j) (i32.const 8)) (i32.const 4)))))
          (i32.store (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $j) (i32.const 8)) (i32.const 4)))
            (i32.load (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $j) (i32.const 8)) (i32.const -4)))))
          (i32.store (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $j) (i32.const 8)) (i32.const -4))) (local.get $tmp))
          (local.set $j (i32.sub (local.get $j) (i32.const 1)))
          (br $shift)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $sort)))
  ;; pass 3: copy the sorted names after the table, slots now {len, dst}
  (local.set $cursor (i32.add (local.get $tbase) (i32.mul (local.get $count) (i32.const 8))))
  (local.set $i (i32.const 0))
  (block $copied
    (loop $cp
      (br_if $copied (i32.ge_u (local.get $i) (local.get $count)))
      (local.set $nl (i32.load (i32.add (local.get $tbase) (i32.mul (local.get $i) (i32.const 8)))))
      (local.set $src (i32.load (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $i) (i32.const 8)) (i32.const 4)))))
      (memory.copy (local.get $cursor) (local.get $src) (local.get $nl))
      (i32.store (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $i) (i32.const 8)) (i32.const 4))) (local.get $cursor))
      (local.set $cursor (i32.add (local.get $cursor) (local.get $nl)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $cp)))
  ;; emit [u32 len][u32 count]{[u32 len][bytes]} at offset 0; the payload
  ;; starts at the count word, each entry adds its own length word
  (local.set $total (i32.const 4))
  (local.set $i (i32.const 0))
  (block $summed
    (loop $sum
      (br_if $summed (i32.ge_u (local.get $i) (local.get $count)))
      (local.set $total (i32.add (local.get $total)
        (i32.add (i32.const 4) (i32.load (i32.add (local.get $tbase) (i32.mul (local.get $i) (i32.const 8)))))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $sum)))
  (call $wasi_ensure (i32.add (i32.const 8) (local.get $total)))
  (i32.store (i32.const 0) (local.get $total))
  (i32.store (i32.const 4) (local.get $count))
  (local.set $cursor (i32.const 8))
  (local.set $i (i32.const 0))
  (block $emitted
    (loop $emit
      (br_if $emitted (i32.ge_u (local.get $i) (local.get $count)))
      (local.set $nl (i32.load (i32.add (local.get $tbase) (i32.mul (local.get $i) (i32.const 8)))))
      (local.set $src (i32.load (i32.add (local.get $tbase) (i32.add (i32.mul (local.get $i) (i32.const 8)) (i32.const 4)))))
      (i32.store (local.get $cursor) (local.get $nl))
      (memory.copy (i32.add (local.get $cursor) (i32.const 4)) (local.get $src) (local.get $nl))
      (local.set $cursor (i32.add (local.get $cursor) (i32.add (i32.const 4) (local.get $nl))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $emit)))
  (i32.const 0))

(func $zhex_host_fs_make_folder (param $pptr i32) (param $plen i32) (param $parents i32) (result i32)
  (local $e i32) (local $i i32)
  (memory.copy (i32.const 4096) (local.get $pptr) (local.get $plen))
  (if (i32.eqz (local.get $parents))
    (then (return (call $wasi.path_create_directory (i32.const 3) (i32.const 4096) (local.get $plen)))))
  ;; parents mode: create every prefix, tolerating existing folders
  ;; (os.MkdirAll shape: an existing file reports ENOTDIR, not EEXIST)
  (block $walked
    (loop $walk
      (br_if $walked (i32.ge_u (local.get $i) (local.get $plen)))
      (if (i32.and (i32.gt_u (local.get $i) (i32.const 0))
                   (i32.eq (i32.load8_u (i32.add (i32.const 4096) (local.get $i))) (i32.const 47)))
        (then
          (local.set $e (call $wasi.path_create_directory (i32.const 3) (i32.const 4096) (local.get $i)))
          (if (i32.eq (local.get $e) (i32.const 20))
            (then
              (if (i32.eqz (call $wasi_is_dir (i32.const 4096) (local.get $i)))
                (then (return (i32.const 54))))))
          (if (i32.eq (local.get $e) (i32.const 54))
            (then
              (if (i32.eqz (call $wasi_is_dir (i32.const 4096) (local.get $i)))
                (then (return (i32.const 54)))))
            (else
              (if (i32.ne (local.get $e) (i32.const 0)) (then (return (local.get $e))))))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $walk)))
  (local.set $e (call $wasi.path_create_directory (i32.const 3) (i32.const 4096) (local.get $plen)))
  (if (i32.eq (local.get $e) (i32.const 20))
    (then
      (if (i32.eqz (call $wasi_is_dir (i32.const 4096) (local.get $plen)))
        (then (return (i32.const 54))))
      (return (i32.const 0))))
  (if (i32.eq (local.get $e) (i32.const 54))
    (then
      (if (i32.eqz (call $wasi_is_dir (i32.const 4096) (local.get $plen)))
        (then (return (i32.const 54))))
      (return (i32.const 0))))
  (local.get $e))

(func $zhex_host_fs_delete (param $pptr i32) (param $plen i32) (result i32)
  (local $e i32)
  (memory.copy (i32.const 4096) (local.get $pptr) (local.get $plen))
  (local.set $e (call $wasi.path_unlink_file (i32.const 3) (i32.const 4096) (local.get $plen)))
  (if (i32.eq (local.get $e) (i32.const 31))
    (then (return (call $wasi.path_remove_directory (i32.const 3) (i32.const 4096) (local.get $plen)))))
  (local.get $e))

(func $zhex_host_fs_delete_folder (param $pptr i32) (param $plen i32) (param $recursive i32) (result i32)
  (memory.copy (i32.const 4096) (local.get $pptr) (local.get $plen))
  (if (i32.eqz (local.get $recursive))
    (then (return (call $wasi.path_remove_directory (i32.const 3) (i32.const 4096) (local.get $plen)))))
  (call $wasi_rmrf (i32.const 4096) (local.get $plen)))

;; shortcut: overwrite is unchecked at the rename layer (POSIX rename
;; replaces files); the prelude's stat pre-checks carry the contract, and
;; the documented check-then-act window stays open exactly as the
;; interpreter's does (files.go movePath).
(func $zhex_host_fs_move (param $sptr i32) (param $slen i32) (param $dptr i32) (param $dlen i32) (param $overwrite i32) (result i32)
  (memory.copy (i32.const 4096) (local.get $sptr) (local.get $slen))
  (memory.copy (i32.const 6144) (local.get $dptr) (local.get $dlen))
  (call $wasi.path_rename (i32.const 3) (i32.const 4096) (local.get $slen) (i32.const 3) (i32.const 6144) (local.get $dlen)))

(func $zhex_host_env_get (param $kptr i32) (param $klen i32) (result i32)
  (local $n i32) (local $i i32) (local $p i32) (local $q i32)
  (memory.copy (i32.const 4096) (local.get $kptr) (local.get $klen))
  (local.set $n (call $wasi_load_environ))
  (if (i32.lt_s (local.get $n) (i32.const 0)) (then (return (local.get $n))))
  (block $miss
    (loop $entry
      (br_if $miss (i32.ge_u (local.get $i) (local.get $n)))
      (local.set $p (i32.load (i32.add (i32.const 65536) (i32.mul (local.get $i) (i32.const 4)))))
      ;; shortcut: prelude never calls env_get (module-side env); unreachable as shipped, delete when the host env surface ever goes live
      (if (i32.eq (i32.load8_u (i32.add (local.get $p) (local.get $klen))) (i32.const 61))
        (then
          (if (call $wasi_bytes_eq (local.get $p) (i32.const 4096) (local.get $klen))
            (then
              (local.set $q (i32.add (local.get $p) (i32.add (local.get $klen) (i32.const 1))))
              (block $end
                (loop $scan
                  (br_if $end (i32.eqz (i32.load8_u (local.get $q))))
                  (local.set $q (i32.add (local.get $q) (i32.const 1)))
                  (br $scan)))
              (i32.store (i32.const 0) (i32.sub (local.get $q) (i32.add (local.get $p) (i32.add (local.get $klen) (i32.const 1)))))
              (memory.copy (i32.const 4) (i32.add (local.get $p) (i32.add (local.get $klen) (i32.const 1)))
                (i32.sub (local.get $q) (i32.add (local.get $p) (i32.add (local.get $klen) (i32.const 1)))))
              (return (i32.const 1))))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $entry)))
  (i32.const 0))

(func $zhex_host_env_environ (result i32)
  (local $n i32) (local $i i32) (local $p i32) (local $eq i32) (local $end i32)
  (local $total i32) (local $cursor i32) (local $count i32) (local $klen i32)
  (local.set $n (call $wasi_load_environ))
  (if (i32.lt_s (local.get $n) (i32.const 0)) (then (return (i32.sub (i32.const 0) (local.get $n)))))
  ;; pass 1: measure (entries without '=' are skipped entirely)
  (block $m1
    (loop $measure
      (br_if $m1 (i32.ge_u (local.get $i) (local.get $n)))
      (local.set $p (i32.load (i32.add (i32.const 65536) (i32.mul (local.get $i) (i32.const 4)))))
      (local.set $eq (local.get $p))
      (block $ke
        (loop $ks
          (br_if $ke (i32.eqz (i32.load8_u (local.get $eq))))
          (br_if $ke (i32.eq (i32.load8_u (local.get $eq)) (i32.const 61)))
          (local.set $eq (i32.add (local.get $eq) (i32.const 1)))
          (br $ks)))
      (if (i32.and (i32.ne (i32.load8_u (local.get $eq)) (i32.const 61)) (i32.eqz (i32.load8_u (local.get $eq))))
        (then (local.set $i (i32.add (local.get $i) (i32.const 1))) (br $measure)))
      (local.set $end (local.get $eq))
      (block $ve
        (loop $vs
          (br_if $ve (i32.eqz (i32.load8_u (local.get $end))))
          (local.set $end (i32.add (local.get $end) (i32.const 1)))
          (br $vs)))
      (local.set $total (i32.add (local.get $total)
        (i32.add (i32.const 8) (i32.add (i32.sub (local.get $eq) (local.get $p)) (i32.sub (local.get $end) (i32.add (local.get $eq) (i32.const 1)))))))
      (local.set $count (i32.add (local.get $count) (i32.const 1)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $measure)))
  (call $wasi_ensure (i32.add (i32.const 8) (local.get $total)))
  (i32.store (i32.const 0) (local.get $total))
  (i32.store (i32.const 4) (local.get $count))
  (local.set $i (i32.const 0))
  (local.set $cursor (i32.const 8))
  (block $done
    (loop $emit
      (br_if $done (i32.ge_u (local.get $i) (local.get $n)))
      (local.set $p (i32.load (i32.add (i32.const 65536) (i32.mul (local.get $i) (i32.const 4)))))
      (local.set $eq (local.get $p))
      (block $ke
        (loop $ks
          (br_if $ke (i32.eqz (i32.load8_u (local.get $eq))))
          (br_if $ke (i32.eq (i32.load8_u (local.get $eq)) (i32.const 61)))
          (local.set $eq (i32.add (local.get $eq) (i32.const 1)))
          (br $ks)))
      (if (i32.and (i32.ne (i32.load8_u (local.get $eq)) (i32.const 61)) (i32.eqz (i32.load8_u (local.get $eq))))
        (then (local.set $i (i32.add (local.get $i) (i32.const 1))) (br $emit)))
      (local.set $klen (i32.sub (local.get $eq) (local.get $p)))
      (local.set $end (local.get $eq))
      (block $ve
        (loop $vs
          (br_if $ve (i32.eqz (i32.load8_u (local.get $end))))
          (local.set $end (i32.add (local.get $end) (i32.const 1)))
          (br $vs)))
      (i32.store (local.get $cursor) (local.get $klen))
      (memory.copy (i32.add (local.get $cursor) (i32.const 4)) (local.get $p) (local.get $klen))
      (local.set $cursor (i32.add (local.get $cursor) (i32.add (i32.const 4) (local.get $klen))))
      (i32.store (local.get $cursor) (i32.sub (local.get $end) (i32.add (local.get $eq) (i32.const 1))))
      (memory.copy (i32.add (local.get $cursor) (i32.const 4)) (i32.add (local.get $eq) (i32.const 1))
        (i32.sub (local.get $end) (i32.add (local.get $eq) (i32.const 1))))
      (local.set $cursor (i32.add (local.get $cursor)
        (i32.add (i32.const 4) (i32.sub (local.get $end) (i32.add (local.get $eq) (i32.const 1))))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $emit)))
  (i32.const 0))

;; ---- fail path (Surprises 8) and _start ----

(func $wasi_fail_line (param $msg (ref $zhex_string)) (result i32)
  ;; the interpreter's diagnostics end with a newline; stage prefix + msg + \n
  (call $zhex_stage_utf8
    (call $zhex_str_concat (call $zhex_str_concat (call $wasi_fail_prefix) (local.get $msg))
      (array.new_fixed $zhex_string 1 (i32.const 10)))))

(func $zhex_fail (param $code i32)
  (call $wasi_out_flush)
  (drop (call $wasi_write (i32.const 2) (i32.const 0)
    (call $wasi_fail_line (call $wasi_fail_text (local.get $code)))))
  (call $wasi.proc_exit (i32.const 1)))

(func $zhex_fail_msg (param $code i32) (param $msg (ref $zhex_string))
  (call $wasi_out_flush)
  (drop (call $wasi_write (i32.const 2) (i32.const 0)
    (call $wasi_fail_line (local.get $msg))))
  (call $wasi.proc_exit (i32.const 1)))

(func $zhex_wasi_start (export "_start")
  (call $zhex_main)
  (call $wasi_out_flush))


`

// watWasiFailTexts renders the runner's fixed fail strings (M1 §6a + M2 §2c)
// as prelude string funcs plus the "runtime error: " prefix and the code
// dispatcher. Committed literals, not stdlib calls at test time.
func watWasiFailTexts() string {
	messages := map[int]string{
		1:  "integer overflow",
		2:  "word call depth limit of 256 exceeded",
		3:  "list index is out of range",
		4:  "array coordinate is out of range",
		5:  "array rank must be between 1 and 32",
		6:  "array extents cannot be negative",
		7:  "array index has the wrong rank",
		8:  "list exceeds element limit",
		9:  "array exceeds element limit",
		10: "collection is being iterated",
		11: "unhandled failure",
		13: "text.replace old text is empty",
		14: "text.split exceeds element limit",
		19: "division or remainder by zero",
		20: "shift count is negative",
		21: "shift count of 64 or more exceeds this build's 64-bit Int width",
	}
	var b strings.Builder
	b.WriteString("\n;; fixed fail messages (M1 6a + M2 2c), committed literals\n")
	b.WriteString(watStrFn("$wasi_fail_prefix", "runtime error: "))
	for code := 1; code <= 21; code++ {
		msg, ok := messages[code]
		if !ok {
			continue // 12 and 15 stage dynamic messages via $zhex_fail_msg
		}
		b.WriteString(watStrFn("$wasi_msg_"+itoa(code), msg))
	}
	b.WriteString("(func $wasi_fail_text (param $code i32) (result (ref $zhex_string))\n")
	for code := 1; code <= 21; code++ {
		if _, ok := messages[code]; !ok {
			continue
		}
		fmt.Fprintf(&b, "  (if (i32.eq (local.get $code) (i32.const %d)) (then (return (call $wasi_msg_%d))))\n", code, code)
	}
	b.WriteString("  (call $wasi_msg_11))\n")
	return b.String()
}

// watStrFn renders a zero-arg WAT function returning a fresh code-point
// array for text.
func watStrFn(name, text string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "(func %s (result (ref $zhex_string)) (array.new_fixed $zhex_string %d", name, len([]rune(text)))
	for _, r := range text {
		fmt.Fprintf(&b, " (i32.const %d)", r)
	}
	b.WriteString("))\n")
	return b.String()
}
