package lang

// The fixed WasmGC runtime prelude for the wasm milestone-1 compiler,
// internal/wasm/DESIGN.md §2/§5/§6. watPrelude is emitted verbatim ahead of
// every program's WAT: value-representation types, the env.print import, the
// prelude-owned linear memory, depth/error globals, and every helper the
// emitter calls by pinned name. Semantics mirror eval.go/collections.go
// exactly (check order, limits, and overflow conditions); the fail codes are
// the pinned §6a table.
const watPrelude string = watPreludeBase +
	watPreludeArith +
	watPreludeText +
	watPreludeBoxing +
	watPreludeListI64 +
	watPreludeListI32 +
	watPreludeListRef +
	watPreludeArrayI64 +
	watPreludeArrayI32 +
	watPreludeArrayRef

// watPreludeBase: §2 value types (all three element specializations, always
// emitted), the single §4 import, linear memory, depth and error globals, and
// the trap/depth helpers ($zhex_fail sets the code then traps — the only
// deliberate trap; $zhex_enter mirrors eval's `depth >= 256` check).
const watPreludeBase = `
;; zhex wasm runtime prelude (DESIGN.md 2/5/6) - fixed text, emitted verbatim
(type $zhex_string      (array (mut i32)))
(type $zhex_arr_i64     (array (mut i64)))
(type $zhex_arr_i32     (array (mut i32)))
(type $zhex_arr_ref     (array (mut (ref null eq))))
(type $zhex_list_i64    (struct (field $items (mut (ref null $zhex_arr_i64))) (field $active (mut i32))))
(type $zhex_list_i32    (struct (field $items (mut (ref null $zhex_arr_i32))) (field $active (mut i32))))
(type $zhex_list_ref    (struct (field $items (mut (ref null $zhex_arr_ref))) (field $active (mut i32))))
(type $zhex_array_i64   (struct (field $shape (ref $zhex_arr_i64)) (field $items (mut (ref null $zhex_arr_i64))) (field $active (mut i32))))
(type $zhex_array_i32   (struct (field $shape (ref $zhex_arr_i64)) (field $items (mut (ref null $zhex_arr_i32))) (field $active (mut i32))))
(type $zhex_array_ref   (struct (field $shape (ref $zhex_arr_i64)) (field $items (mut (ref null $zhex_arr_ref))) (field $active (mut i32))))
(type $zhex_some_i64    (struct (field $value i64)))
(type $zhex_some_i32    (struct (field $value i32)))
(type $zhex_some_ref    (struct (field $value (ref null eq))))
(type $zhex_none        (struct))
(type $zhex_result      (struct (field $tag i32) (field $payload (ref null eq))))

(import "env" "print" (func $env.print (param i32 i32)))
(memory $zhex_mem 1)
(global $zhex_depth (mut i32) (i32.const 0))
(global $zhex_err (mut i32) (i32.const 0))

(func $zhex_fail (param $code i32)
  (global.set $zhex_err (local.get $code))
  (unreachable))

(func $zhex_last_error (result i32)
  (global.get $zhex_err))

(func $zhex_enter
  (if (i32.ge_s (global.get $zhex_depth) (i32.const 256))
    (then (call $zhex_fail (i32.const 2))))
  (global.set $zhex_depth (i32.add (global.get $zhex_depth) (i32.const 1))))

(func $zhex_leave
  (global.set $zhex_depth (i32.sub (global.get $zhex_depth) (i32.const 1))))
`

// watPreludeArith: checked integer helpers mirroring eval.binary §6d (fail 1
// on overflow) and §6b ($zhex_div pre-checks MinInt64/-1 because the native
// trap wording varies; division by zero keeps its native trap).
const watPreludeArith = `
(func $zhex_add (param $a i64) (param $b i64) (result i64)
  (if (i64.gt_s (local.get $b) (i64.const 0))
    (then
      (if (i64.gt_s (local.get $a) (i64.sub (i64.const 9223372036854775807) (local.get $b)))
        (then (call $zhex_fail (i32.const 1)))))
    (else
      (if (i64.lt_s (local.get $b) (i64.const 0))
        (then
          (if (i64.lt_s (local.get $a) (i64.sub (i64.const -9223372036854775808) (local.get $b)))
            (then (call $zhex_fail (i32.const 1))))))))
  (i64.add (local.get $a) (local.get $b)))

(func $zhex_sub (param $a i64) (param $b i64) (result i64)
  (if (i64.lt_s (local.get $b) (i64.const 0))
    (then
      (if (i64.gt_s (local.get $a) (i64.add (i64.const 9223372036854775807) (local.get $b)))
        (then (call $zhex_fail (i32.const 1)))))
    (else
      (if (i64.gt_s (local.get $b) (i64.const 0))
        (then
          (if (i64.lt_s (local.get $a) (i64.add (i64.const -9223372036854775808) (local.get $b)))
            (then (call $zhex_fail (i32.const 1))))))))
  (i64.sub (local.get $a) (local.get $b)))

(func $zhex_mul (param $a i64) (param $b i64) (result i64)
  (local $n i64)
  (local.set $n (i64.mul (local.get $a) (local.get $b)))
  (if (i64.ne (local.get $a) (i64.const 0))
    (then
      (if (i64.eq (local.get $a) (i64.const -1))
        (then
          (if (i64.eq (local.get $b) (i64.const -9223372036854775808))
            (then (call $zhex_fail (i32.const 1)))))
        (else
          (if (i64.ne (i64.div_s (local.get $n) (local.get $a)) (local.get $b))
            (then (call $zhex_fail (i32.const 1))))))))
  (local.get $n))

(func $zhex_neg (param $a i64) (result i64)
  (if (i64.eq (local.get $a) (i64.const -9223372036854775808))
    (then (call $zhex_fail (i32.const 1))))
  (i64.sub (i64.const 0) (local.get $a)))

(func $zhex_div (param $a i64) (param $b i64) (result i64)
  (if (i64.eq (local.get $b) (i64.const -1))
    (then
      (if (i64.eq (local.get $a) (i64.const -9223372036854775808))
        (then (call $zhex_fail (i32.const 1))))))
  (i64.div_s (local.get $a) (local.get $b)))
`

// watPreludeText: strings are arrays of i32 code points (§2).
// $zhex_int_to_string formats exactly like strconv.FormatInt (decimal,
// MinInt64 handled through unsigned division on the wrapped magnitude).
// $zhex_print_str stages UTF-8 bytes at offset 0 of the prelude memory,
// growing pages as needed, then calls env.print(0, len).
const watPreludeText = `
(func $zhex_int_to_string (param $v i64) (result (ref $zhex_string))
  (local $neg i32)
  (local $u i64)
  (local $digits i32)
  (local $len i32)
  (local $t i64)
  (local $s (ref null $zhex_string))
  (local $i i32)
  (local.set $neg (i64.lt_s (local.get $v) (i64.const 0)))
  (local.set $u (select (i64.sub (i64.const 0) (local.get $v)) (local.get $v) (local.get $neg)))
  (local.set $t (local.get $u))
  (loop $count
    (local.set $digits (i32.add (local.get $digits) (i32.const 1)))
    (local.set $t (i64.div_u (local.get $t) (i64.const 10)))
    (br_if $count (i64.ne (local.get $t) (i64.const 0))))
  (local.set $len (i32.add (local.get $digits) (local.get $neg)))
  (local.set $s (array.new $zhex_string (i32.const 0) (local.get $len)))
  (local.set $t (local.get $u))
  (local.set $i (local.get $len))
  (block $done
    (loop $fill
      (br_if $done (i32.le_u (local.get $i) (local.get $neg)))
      (local.set $i (i32.sub (local.get $i) (i32.const 1)))
      (array.set $zhex_string (local.get $s) (local.get $i)
        (i32.add (i32.const 48) (i32.wrap_i64 (i64.rem_u (local.get $t) (i64.const 10)))))
      (local.set $t (i64.div_u (local.get $t) (i64.const 10)))
      (br $fill)))
  (if (local.get $neg)
    (then (array.set $zhex_string (local.get $s) (i32.const 0) (i32.const 45))))
  (ref.as_non_null (local.get $s)))

(func $zhex_str_eq (param $a (ref $zhex_string)) (param $b (ref $zhex_string)) (result i32)
  (local $n i32)
  (local $i i32)
  (local.set $n (array.len (local.get $a)))
  (if (i32.ne (local.get $n) (array.len (local.get $b)))
    (then (return (i32.const 0))))
  (block $mismatch
    (loop $cmp
      (if (i32.ge_u (local.get $i) (local.get $n))
        (then (return (i32.const 1))))
      (br_if $mismatch
        (i32.ne (array.get $zhex_string (local.get $a) (local.get $i))
                (array.get $zhex_string (local.get $b) (local.get $i))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $cmp)))
  (i32.const 0))

(func $zhex_str_concat (param $a (ref $zhex_string)) (param $b (ref $zhex_string)) (result (ref $zhex_string))
  (local $r (ref null $zhex_string))
  (local $la i32)
  (local $lb i32)
  (local $i i32)
  (local.set $la (array.len (local.get $a)))
  (local.set $lb (array.len (local.get $b)))
  (local.set $r (array.new_default $zhex_string (i32.add (local.get $la) (local.get $lb))))
  (local.set $i (i32.const 0))
  (block $copied_a
    (loop $copy_a
      (br_if $copied_a (i32.ge_u (local.get $i) (local.get $la)))
      (array.set $zhex_string (local.get $r) (local.get $i)
        (array.get $zhex_string (local.get $a) (local.get $i)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $copy_a)))
  (local.set $i (i32.const 0))
  (block $copied_b
    (loop $copy_b
      (br_if $copied_b (i32.ge_u (local.get $i) (local.get $lb)))
      (array.set $zhex_string (local.get $r) (i32.add (local.get $la) (local.get $i))
        (array.get $zhex_string (local.get $b) (local.get $i)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $copy_b)))
  (ref.as_non_null (local.get $r)))

(func $zhex_print_str (param $s (ref $zhex_string))
  (local $n i32)
  (local $i i32)
  (local $cp i32)
  (local $len i32)
  (local $pages i32)
  (local $w i32)
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
  (local.set $pages (i32.div_u (i32.add (local.get $len) (i32.const 65535)) (i32.const 65536)))
  (if (i32.gt_u (local.get $pages) (memory.size))
    (then (drop (memory.grow (i32.sub (local.get $pages) (memory.size))))))
  (local.set $i (i32.const 0))
  (local.set $w (i32.const 0))
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
  (call $env.print (i32.const 0) (local.get $len)))

(func $zhex_print_int (param $v i64)
  (call $zhex_print_str (call $zhex_int_to_string (local.get $v))))
`

// watPreludeBoxing: Result payloads are eq refs (§2) — i31 boxes Int/Bool,
// everything else is a direct reference, Unit is null.
const watPreludeBoxing = `
(func $zhex_box_i64 (param $v i64) (result (ref eq))
  (ref.i31 (i32.wrap_i64 (local.get $v))))

(func $zhex_unbox_i64 (param $p (ref null eq)) (result i64)
  (i64.extend_i32_s (i31.get_s (ref.cast (ref i31) (local.get $p)))))

(func $zhex_box_i32 (param $v i32) (result (ref eq))
  (ref.i31 (local.get $v)))

(func $zhex_unbox_i32 (param $p (ref null eq)) (result i32)
  (i31.get_s (ref.cast (ref i31) (local.get $p))))
`

// watPreludeListI64: List<Int> helpers. A list is the wrapper struct, so
// names alias one buffer; append grows by storing a fresh array into $items
// (shared-reference growth). Check order mirrors host.go list.append and
// collections.go listRead/listWrite: active (fail 10) before the limit
// (fail 8) on append; active before bounds (fail 3) on writes.
const watPreludeListI64 = `
(func $zhex_list_new_i64 (result (ref $zhex_list_i64))
  (struct.new $zhex_list_i64
    (array.new_default $zhex_arr_i64 (i32.const 0))
    (i32.const 0)))

(func $zhex_list_length_i64 (param $list (ref $zhex_list_i64)) (result i64)
  (i64.extend_i32_u (array.len (struct.get $zhex_list_i64 $items (local.get $list)))))

(func $zhex_list_append_i64 (param $list (ref $zhex_list_i64)) (param $elem i64)
  (local $old (ref null $zhex_arr_i64))
  (local $n i32)
  (local $fresh (ref null $zhex_arr_i64))
  (local $i i32)
  (if (i32.ne (struct.get $zhex_list_i64 $active (local.get $list)) (i32.const 0))
    (then (call $zhex_fail (i32.const 10))))
  (local.set $old (struct.get $zhex_list_i64 $items (local.get $list)))
  (local.set $n (array.len (local.get $old)))
  (if (i32.ge_u (local.get $n) (i32.const 1000000))
    (then (call $zhex_fail (i32.const 8))))
  (local.set $fresh (array.new_default $zhex_arr_i64 (i32.add (local.get $n) (i32.const 1))))
  (block $copied
    (loop $copy
      (br_if $copied (i32.ge_u (local.get $i) (local.get $n)))
      (array.set $zhex_arr_i64 (local.get $fresh) (local.get $i)
        (array.get $zhex_arr_i64 (local.get $old) (local.get $i)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $copy)))
  (array.set $zhex_arr_i64 (local.get $fresh) (local.get $n) (local.get $elem))
  (struct.set $zhex_list_i64 $items (local.get $list) (local.get $fresh)))

(func $zhex_list_get_i64 (param $list (ref $zhex_list_i64)) (param $index i64) (result i64)
  (if (i64.ge_u (local.get $index)
        (i64.extend_i32_u (array.len (struct.get $zhex_list_i64 $items (local.get $list)))))
    (then (call $zhex_fail (i32.const 3))))
  (array.get $zhex_arr_i64 (struct.get $zhex_list_i64 $items (local.get $list))
    (i32.wrap_i64 (local.get $index))))

(func $zhex_list_set_i64 (param $list (ref $zhex_list_i64)) (param $index i64) (param $value i64)
  (if (i32.ne (struct.get $zhex_list_i64 $active (local.get $list)) (i32.const 0))
    (then (call $zhex_fail (i32.const 10))))
  (if (i64.ge_u (local.get $index)
        (i64.extend_i32_u (array.len (struct.get $zhex_list_i64 $items (local.get $list)))))
    (then (call $zhex_fail (i32.const 3))))
  (array.set $zhex_arr_i64 (struct.get $zhex_list_i64 $items (local.get $list))
    (i32.wrap_i64 (local.get $index)) (local.get $value)))
`

// watPreludeListI32: List<Bool> — same shape as the i64 family.
const watPreludeListI32 = `
(func $zhex_list_new_i32 (result (ref $zhex_list_i32))
  (struct.new $zhex_list_i32
    (array.new_default $zhex_arr_i32 (i32.const 0))
    (i32.const 0)))

(func $zhex_list_length_i32 (param $list (ref $zhex_list_i32)) (result i64)
  (i64.extend_i32_u (array.len (struct.get $zhex_list_i32 $items (local.get $list)))))

(func $zhex_list_append_i32 (param $list (ref $zhex_list_i32)) (param $elem i32)
  (local $old (ref null $zhex_arr_i32))
  (local $n i32)
  (local $fresh (ref null $zhex_arr_i32))
  (local $i i32)
  (if (i32.ne (struct.get $zhex_list_i32 $active (local.get $list)) (i32.const 0))
    (then (call $zhex_fail (i32.const 10))))
  (local.set $old (struct.get $zhex_list_i32 $items (local.get $list)))
  (local.set $n (array.len (local.get $old)))
  (if (i32.ge_u (local.get $n) (i32.const 1000000))
    (then (call $zhex_fail (i32.const 8))))
  (local.set $fresh (array.new_default $zhex_arr_i32 (i32.add (local.get $n) (i32.const 1))))
  (block $copied
    (loop $copy
      (br_if $copied (i32.ge_u (local.get $i) (local.get $n)))
      (array.set $zhex_arr_i32 (local.get $fresh) (local.get $i)
        (array.get $zhex_arr_i32 (local.get $old) (local.get $i)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $copy)))
  (array.set $zhex_arr_i32 (local.get $fresh) (local.get $n) (local.get $elem))
  (struct.set $zhex_list_i32 $items (local.get $list) (local.get $fresh)))

(func $zhex_list_get_i32 (param $list (ref $zhex_list_i32)) (param $index i64) (result i32)
  (if (i64.ge_u (local.get $index)
        (i64.extend_i32_u (array.len (struct.get $zhex_list_i32 $items (local.get $list)))))
    (then (call $zhex_fail (i32.const 3))))
  (array.get $zhex_arr_i32 (struct.get $zhex_list_i32 $items (local.get $list))
    (i32.wrap_i64 (local.get $index))))

(func $zhex_list_set_i32 (param $list (ref $zhex_list_i32)) (param $index i64) (param $value i32)
  (if (i32.ne (struct.get $zhex_list_i32 $active (local.get $list)) (i32.const 0))
    (then (call $zhex_fail (i32.const 10))))
  (if (i64.ge_u (local.get $index)
        (i64.extend_i32_u (array.len (struct.get $zhex_list_i32 $items (local.get $list)))))
    (then (call $zhex_fail (i32.const 3))))
  (array.set $zhex_arr_i32 (struct.get $zhex_list_i32 $items (local.get $list))
    (i32.wrap_i64 (local.get $index)) (local.get $value)))
`

// watPreludeListRef: lists of reference values (strings, records, choices).
const watPreludeListRef = `
(func $zhex_list_new_ref (result (ref $zhex_list_ref))
  (struct.new $zhex_list_ref
    (array.new_default $zhex_arr_ref (i32.const 0))
    (i32.const 0)))

(func $zhex_list_length_ref (param $list (ref $zhex_list_ref)) (result i64)
  (i64.extend_i32_u (array.len (struct.get $zhex_list_ref $items (local.get $list)))))

(func $zhex_list_append_ref (param $list (ref $zhex_list_ref)) (param $elem (ref null eq))
  (local $old (ref null $zhex_arr_ref))
  (local $n i32)
  (local $fresh (ref null $zhex_arr_ref))
  (local $i i32)
  (if (i32.ne (struct.get $zhex_list_ref $active (local.get $list)) (i32.const 0))
    (then (call $zhex_fail (i32.const 10))))
  (local.set $old (struct.get $zhex_list_ref $items (local.get $list)))
  (local.set $n (array.len (local.get $old)))
  (if (i32.ge_u (local.get $n) (i32.const 1000000))
    (then (call $zhex_fail (i32.const 8))))
  (local.set $fresh (array.new_default $zhex_arr_ref (i32.add (local.get $n) (i32.const 1))))
  (block $copied
    (loop $copy
      (br_if $copied (i32.ge_u (local.get $i) (local.get $n)))
      (array.set $zhex_arr_ref (local.get $fresh) (local.get $i)
        (array.get $zhex_arr_ref (local.get $old) (local.get $i)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $copy)))
  (array.set $zhex_arr_ref (local.get $fresh) (local.get $n) (local.get $elem))
  (struct.set $zhex_list_ref $items (local.get $list) (local.get $fresh)))

(func $zhex_list_get_ref (param $list (ref $zhex_list_ref)) (param $index i64) (result (ref null eq))
  (if (i64.ge_u (local.get $index)
        (i64.extend_i32_u (array.len (struct.get $zhex_list_ref $items (local.get $list)))))
    (then (call $zhex_fail (i32.const 3))))
  (array.get $zhex_arr_ref (struct.get $zhex_list_ref $items (local.get $list))
    (i32.wrap_i64 (local.get $index))))

(func $zhex_list_set_ref (param $list (ref $zhex_list_ref)) (param $index i64) (param $value (ref null eq))
  (if (i32.ne (struct.get $zhex_list_ref $active (local.get $list)) (i32.const 0))
    (then (call $zhex_fail (i32.const 10))))
  (if (i64.ge_u (local.get $index)
        (i64.extend_i32_u (array.len (struct.get $zhex_list_ref $items (local.get $list)))))
    (then (call $zhex_fail (i32.const 3))))
  (array.set $zhex_arr_ref (struct.get $zhex_list_ref $items (local.get $list))
    (i32.wrap_i64 (local.get $index)) (local.get $value)))
`

// watPreludeArrayI64: Array<Int>. $zhex_array_new mirrors collections.go
// arrayShape/arrayNew: rank outside 2-32 fails 5, a negative extent fails 6,
// a zero extent yields empty items, the per-axis limit check fails 9, and the
// flat items are row-major. The stored shape is a private copy, so later
// mutation of the shape list cannot alias it. get/set check rank (fail 7),
// then per-axis bounds while folding offset = offset*shape[i] + coord[i]
// (fail 4); set checks $active first (fail 10).
const watPreludeArrayI64 = `
(func $zhex_array_new_i64 (param $shape (ref $zhex_list_i64)) (param $fill i64) (result (ref $zhex_array_i64))
  (local $dims (ref null $zhex_arr_i64))
  (local $rank i32)
  (local $i i32)
  (local $n i64)
  (local $zero i32)
  (local $count i64)
  (local $flat (ref null $zhex_arr_i64))
  (local $copy (ref null $zhex_arr_i64))
  (local.set $dims (struct.get $zhex_list_i64 $items (local.get $shape)))
  (local.set $rank (array.len (local.get $dims)))
  (if (i32.lt_u (local.get $rank) (i32.const 2))
    (then (call $zhex_fail (i32.const 5))))
  (if (i32.gt_u (local.get $rank) (i32.const 32))
    (then (call $zhex_fail (i32.const 5))))
  (block $scanned
    (loop $scan
      (br_if $scanned (i32.ge_u (local.get $i) (local.get $rank)))
      (local.set $n (array.get $zhex_arr_i64 (local.get $dims) (local.get $i)))
      (if (i64.lt_s (local.get $n) (i64.const 0))
        (then (call $zhex_fail (i32.const 6))))
      (if (i64.eqz (local.get $n))
        (then (local.set $zero (i32.const 1))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $scan)))
  (if (i32.eqz (local.get $zero))
    (then
      (local.set $count (i64.const 1))
      (local.set $i (i32.const 0))
      (block $counted
        (loop $countup
          (br_if $counted (i32.ge_u (local.get $i) (local.get $rank)))
          (local.set $n (array.get $zhex_arr_i64 (local.get $dims) (local.get $i)))
          (if (i64.gt_u (local.get $n) (i64.div_u (i64.const 1000000) (local.get $count)))
            (then (call $zhex_fail (i32.const 9))))
          (local.set $count (i64.mul (local.get $count) (local.get $n)))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $countup)))
      (local.set $flat (array.new $zhex_arr_i64 (local.get $fill) (i32.wrap_i64 (local.get $count)))))
    (else
      (local.set $flat (array.new_default $zhex_arr_i64 (i32.const 0)))))
  (local.set $copy (array.new_default $zhex_arr_i64 (local.get $rank)))
  (local.set $i (i32.const 0))
  (block $copied
    (loop $copyloop
      (br_if $copied (i32.ge_u (local.get $i) (local.get $rank)))
      (array.set $zhex_arr_i64 (local.get $copy) (local.get $i)
        (array.get $zhex_arr_i64 (local.get $dims) (local.get $i)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $copyloop)))
  (struct.new $zhex_array_i64 (ref.as_non_null (local.get $copy)) (local.get $flat) (i32.const 0)))

(func $zhex_array_length_i64 (param $arr (ref $zhex_array_i64)) (result i64)
  (i64.extend_i32_u (array.len (struct.get $zhex_array_i64 $items (local.get $arr)))))

(func $zhex_array_get_i64 (param $arr (ref $zhex_array_i64)) (param $coords (ref $zhex_arr_i64)) (result i64)
  (local $dims (ref null $zhex_arr_i64))
  (local $rank i32)
  (local $i i32)
  (local $off i64)
  (local $n i64)
  (local $ext i64)
  (local.set $dims (struct.get $zhex_array_i64 $shape (local.get $arr)))
  (local.set $rank (array.len (local.get $dims)))
  (if (i32.ne (array.len (local.get $coords)) (local.get $rank))
    (then (call $zhex_fail (i32.const 7))))
  (block $folded
    (loop $fold
      (br_if $folded (i32.ge_u (local.get $i) (local.get $rank)))
      (local.set $n (array.get $zhex_arr_i64 (local.get $coords) (local.get $i)))
      (local.set $ext (array.get $zhex_arr_i64 (local.get $dims) (local.get $i)))
      (if (i64.ge_u (local.get $n) (local.get $ext))
        (then (call $zhex_fail (i32.const 4))))
      (local.set $off (i64.add (i64.mul (local.get $off) (local.get $ext)) (local.get $n)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $fold)))
  (array.get $zhex_arr_i64 (struct.get $zhex_array_i64 $items (local.get $arr))
    (i32.wrap_i64 (local.get $off))))

(func $zhex_array_set_i64 (param $arr (ref $zhex_array_i64)) (param $coords (ref $zhex_arr_i64)) (param $value i64)
  (local $dims (ref null $zhex_arr_i64))
  (local $rank i32)
  (local $i i32)
  (local $off i64)
  (local $n i64)
  (local $ext i64)
  (if (i32.ne (struct.get $zhex_array_i64 $active (local.get $arr)) (i32.const 0))
    (then (call $zhex_fail (i32.const 10))))
  (local.set $dims (struct.get $zhex_array_i64 $shape (local.get $arr)))
  (local.set $rank (array.len (local.get $dims)))
  (if (i32.ne (array.len (local.get $coords)) (local.get $rank))
    (then (call $zhex_fail (i32.const 7))))
  (block $folded
    (loop $fold
      (br_if $folded (i32.ge_u (local.get $i) (local.get $rank)))
      (local.set $n (array.get $zhex_arr_i64 (local.get $coords) (local.get $i)))
      (local.set $ext (array.get $zhex_arr_i64 (local.get $dims) (local.get $i)))
      (if (i64.ge_u (local.get $n) (local.get $ext))
        (then (call $zhex_fail (i32.const 4))))
      (local.set $off (i64.add (i64.mul (local.get $off) (local.get $ext)) (local.get $n)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $fold)))
  (array.set $zhex_arr_i64 (struct.get $zhex_array_i64 $items (local.get $arr))
    (i32.wrap_i64 (local.get $off)) (local.get $value)))
`

// watPreludeArrayI32: Array<Bool> — same shape as the i64 family.
const watPreludeArrayI32 = `
(func $zhex_array_new_i32 (param $shape (ref $zhex_list_i64)) (param $fill i32) (result (ref $zhex_array_i32))
  (local $dims (ref null $zhex_arr_i64))
  (local $rank i32)
  (local $i i32)
  (local $n i64)
  (local $zero i32)
  (local $count i64)
  (local $flat (ref null $zhex_arr_i32))
  (local $copy (ref null $zhex_arr_i64))
  (local.set $dims (struct.get $zhex_list_i64 $items (local.get $shape)))
  (local.set $rank (array.len (local.get $dims)))
  (if (i32.lt_u (local.get $rank) (i32.const 2))
    (then (call $zhex_fail (i32.const 5))))
  (if (i32.gt_u (local.get $rank) (i32.const 32))
    (then (call $zhex_fail (i32.const 5))))
  (block $scanned
    (loop $scan
      (br_if $scanned (i32.ge_u (local.get $i) (local.get $rank)))
      (local.set $n (array.get $zhex_arr_i64 (local.get $dims) (local.get $i)))
      (if (i64.lt_s (local.get $n) (i64.const 0))
        (then (call $zhex_fail (i32.const 6))))
      (if (i64.eqz (local.get $n))
        (then (local.set $zero (i32.const 1))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $scan)))
  (if (i32.eqz (local.get $zero))
    (then
      (local.set $count (i64.const 1))
      (local.set $i (i32.const 0))
      (block $counted
        (loop $countup
          (br_if $counted (i32.ge_u (local.get $i) (local.get $rank)))
          (local.set $n (array.get $zhex_arr_i64 (local.get $dims) (local.get $i)))
          (if (i64.gt_u (local.get $n) (i64.div_u (i64.const 1000000) (local.get $count)))
            (then (call $zhex_fail (i32.const 9))))
          (local.set $count (i64.mul (local.get $count) (local.get $n)))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $countup)))
      (local.set $flat (array.new $zhex_arr_i32 (local.get $fill) (i32.wrap_i64 (local.get $count)))))
    (else
      (local.set $flat (array.new_default $zhex_arr_i32 (i32.const 0)))))
  (local.set $copy (array.new_default $zhex_arr_i64 (local.get $rank)))
  (local.set $i (i32.const 0))
  (block $copied
    (loop $copyloop
      (br_if $copied (i32.ge_u (local.get $i) (local.get $rank)))
      (array.set $zhex_arr_i64 (local.get $copy) (local.get $i)
        (array.get $zhex_arr_i64 (local.get $dims) (local.get $i)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $copyloop)))
  (struct.new $zhex_array_i32 (ref.as_non_null (local.get $copy)) (local.get $flat) (i32.const 0)))

(func $zhex_array_length_i32 (param $arr (ref $zhex_array_i32)) (result i64)
  (i64.extend_i32_u (array.len (struct.get $zhex_array_i32 $items (local.get $arr)))))

(func $zhex_array_get_i32 (param $arr (ref $zhex_array_i32)) (param $coords (ref $zhex_arr_i64)) (result i32)
  (local $dims (ref null $zhex_arr_i64))
  (local $rank i32)
  (local $i i32)
  (local $off i64)
  (local $n i64)
  (local $ext i64)
  (local.set $dims (struct.get $zhex_array_i32 $shape (local.get $arr)))
  (local.set $rank (array.len (local.get $dims)))
  (if (i32.ne (array.len (local.get $coords)) (local.get $rank))
    (then (call $zhex_fail (i32.const 7))))
  (block $folded
    (loop $fold
      (br_if $folded (i32.ge_u (local.get $i) (local.get $rank)))
      (local.set $n (array.get $zhex_arr_i64 (local.get $coords) (local.get $i)))
      (local.set $ext (array.get $zhex_arr_i64 (local.get $dims) (local.get $i)))
      (if (i64.ge_u (local.get $n) (local.get $ext))
        (then (call $zhex_fail (i32.const 4))))
      (local.set $off (i64.add (i64.mul (local.get $off) (local.get $ext)) (local.get $n)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $fold)))
  (array.get $zhex_arr_i32 (struct.get $zhex_array_i32 $items (local.get $arr))
    (i32.wrap_i64 (local.get $off))))

(func $zhex_array_set_i32 (param $arr (ref $zhex_array_i32)) (param $coords (ref $zhex_arr_i64)) (param $value i32)
  (local $dims (ref null $zhex_arr_i64))
  (local $rank i32)
  (local $i i32)
  (local $off i64)
  (local $n i64)
  (local $ext i64)
  (if (i32.ne (struct.get $zhex_array_i32 $active (local.get $arr)) (i32.const 0))
    (then (call $zhex_fail (i32.const 10))))
  (local.set $dims (struct.get $zhex_array_i32 $shape (local.get $arr)))
  (local.set $rank (array.len (local.get $dims)))
  (if (i32.ne (array.len (local.get $coords)) (local.get $rank))
    (then (call $zhex_fail (i32.const 7))))
  (block $folded
    (loop $fold
      (br_if $folded (i32.ge_u (local.get $i) (local.get $rank)))
      (local.set $n (array.get $zhex_arr_i64 (local.get $coords) (local.get $i)))
      (local.set $ext (array.get $zhex_arr_i64 (local.get $dims) (local.get $i)))
      (if (i64.ge_u (local.get $n) (local.get $ext))
        (then (call $zhex_fail (i32.const 4))))
      (local.set $off (i64.add (i64.mul (local.get $off) (local.get $ext)) (local.get $n)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $fold)))
  (array.set $zhex_arr_i32 (struct.get $zhex_array_i32 $items (local.get $arr))
    (i32.wrap_i64 (local.get $off)) (local.get $value)))
`

// watPreludeArrayRef: arrays of reference values (strings, records, choices).
const watPreludeArrayRef = `
(func $zhex_array_new_ref (param $shape (ref $zhex_list_i64)) (param $fill (ref null eq)) (result (ref $zhex_array_ref))
  (local $dims (ref null $zhex_arr_i64))
  (local $rank i32)
  (local $i i32)
  (local $n i64)
  (local $zero i32)
  (local $count i64)
  (local $flat (ref null $zhex_arr_ref))
  (local $copy (ref null $zhex_arr_i64))
  (local.set $dims (struct.get $zhex_list_i64 $items (local.get $shape)))
  (local.set $rank (array.len (local.get $dims)))
  (if (i32.lt_u (local.get $rank) (i32.const 2))
    (then (call $zhex_fail (i32.const 5))))
  (if (i32.gt_u (local.get $rank) (i32.const 32))
    (then (call $zhex_fail (i32.const 5))))
  (block $scanned
    (loop $scan
      (br_if $scanned (i32.ge_u (local.get $i) (local.get $rank)))
      (local.set $n (array.get $zhex_arr_i64 (local.get $dims) (local.get $i)))
      (if (i64.lt_s (local.get $n) (i64.const 0))
        (then (call $zhex_fail (i32.const 6))))
      (if (i64.eqz (local.get $n))
        (then (local.set $zero (i32.const 1))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $scan)))
  (if (i32.eqz (local.get $zero))
    (then
      (local.set $count (i64.const 1))
      (local.set $i (i32.const 0))
      (block $counted
        (loop $countup
          (br_if $counted (i32.ge_u (local.get $i) (local.get $rank)))
          (local.set $n (array.get $zhex_arr_i64 (local.get $dims) (local.get $i)))
          (if (i64.gt_u (local.get $n) (i64.div_u (i64.const 1000000) (local.get $count)))
            (then (call $zhex_fail (i32.const 9))))
          (local.set $count (i64.mul (local.get $count) (local.get $n)))
          (local.set $i (i32.add (local.get $i) (i32.const 1)))
          (br $countup)))
      (local.set $flat (array.new $zhex_arr_ref (local.get $fill) (i32.wrap_i64 (local.get $count)))))
    (else
      (local.set $flat (array.new_default $zhex_arr_ref (i32.const 0)))))
  (local.set $copy (array.new_default $zhex_arr_i64 (local.get $rank)))
  (local.set $i (i32.const 0))
  (block $copied
    (loop $copyloop
      (br_if $copied (i32.ge_u (local.get $i) (local.get $rank)))
      (array.set $zhex_arr_i64 (local.get $copy) (local.get $i)
        (array.get $zhex_arr_i64 (local.get $dims) (local.get $i)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $copyloop)))
  (struct.new $zhex_array_ref (ref.as_non_null (local.get $copy)) (local.get $flat) (i32.const 0)))

(func $zhex_array_length_ref (param $arr (ref $zhex_array_ref)) (result i64)
  (i64.extend_i32_u (array.len (struct.get $zhex_array_ref $items (local.get $arr)))))

(func $zhex_array_get_ref (param $arr (ref $zhex_array_ref)) (param $coords (ref $zhex_arr_i64)) (result (ref null eq))
  (local $dims (ref null $zhex_arr_i64))
  (local $rank i32)
  (local $i i32)
  (local $off i64)
  (local $n i64)
  (local $ext i64)
  (local.set $dims (struct.get $zhex_array_ref $shape (local.get $arr)))
  (local.set $rank (array.len (local.get $dims)))
  (if (i32.ne (array.len (local.get $coords)) (local.get $rank))
    (then (call $zhex_fail (i32.const 7))))
  (block $folded
    (loop $fold
      (br_if $folded (i32.ge_u (local.get $i) (local.get $rank)))
      (local.set $n (array.get $zhex_arr_i64 (local.get $coords) (local.get $i)))
      (local.set $ext (array.get $zhex_arr_i64 (local.get $dims) (local.get $i)))
      (if (i64.ge_u (local.get $n) (local.get $ext))
        (then (call $zhex_fail (i32.const 4))))
      (local.set $off (i64.add (i64.mul (local.get $off) (local.get $ext)) (local.get $n)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $fold)))
  (array.get $zhex_arr_ref (struct.get $zhex_array_ref $items (local.get $arr))
    (i32.wrap_i64 (local.get $off))))

(func $zhex_array_set_ref (param $arr (ref $zhex_array_ref)) (param $coords (ref $zhex_arr_i64)) (param $value (ref null eq))
  (local $dims (ref null $zhex_arr_i64))
  (local $rank i32)
  (local $i i32)
  (local $off i64)
  (local $n i64)
  (local $ext i64)
  (if (i32.ne (struct.get $zhex_array_ref $active (local.get $arr)) (i32.const 0))
    (then (call $zhex_fail (i32.const 10))))
  (local.set $dims (struct.get $zhex_array_ref $shape (local.get $arr)))
  (local.set $rank (array.len (local.get $dims)))
  (if (i32.ne (array.len (local.get $coords)) (local.get $rank))
    (then (call $zhex_fail (i32.const 7))))
  (block $folded
    (loop $fold
      (br_if $folded (i32.ge_u (local.get $i) (local.get $rank)))
      (local.set $n (array.get $zhex_arr_i64 (local.get $coords) (local.get $i)))
      (local.set $ext (array.get $zhex_arr_i64 (local.get $dims) (local.get $i)))
      (if (i64.ge_u (local.get $n) (local.get $ext))
        (then (call $zhex_fail (i32.const 4))))
      (local.set $off (i64.add (i64.mul (local.get $off) (local.get $ext)) (local.get $n)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $fold)))
  (array.set $zhex_arr_ref (struct.get $zhex_array_ref $items (local.get $arr))
    (i32.wrap_i64 (local.get $off)) (local.get $value)))
`
