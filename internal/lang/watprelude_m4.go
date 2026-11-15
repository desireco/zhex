package lang

import (
	"sort"
	"strings"
)

// The wasm milestone-4 runtime prelude (internal/wasm/DESIGN-M4.md §2-§4):
// the six void-result closure grid rows (§3a), the checked List index read
// (§4), and the xml.decode runtime (§2) — a pure-computation element-tree
// parser plus a schema-directed decode that reuses the M3 json schema,
// clone-memo, and failure-envelope machinery verbatim. Every xml failure is
// family 5 through $zjson_fail's "<message> at \"<path>\"" shape; unhandled
// top-level failures stage via the M3 code-16 path (runners unchanged).
var watPreludeM4 = strings.Join([]string{
	watM4Types,
	watM4CloVoidRows,
	watM4CheckedReads,
	watM4XmlLiterals(),
	watM4XmlHelpers,
	watM4XmlParse,
	watM4XmlDecode,
}, "")

// watM4Types: the xml element tree (§2a) — one rec group; attrs are
// $zxml_attr structs and children are elem refs, both boxed into the
// nullable-eqref array; text accumulates by $zhex_str_concat (O(n²)
// accepted; parity beats speed). The names global pairs schema-node refs
// with their Zhex type names: the emitter's schema builders fill it (wat.go
// appendSchemaFuncs), $zxml_node_name reads it back so `expected <Type>`
// failures render the interpreter's jsonSchema.typ string, aliases included.
const watM4Types = `
;; zhex wasm runtime prelude, milestone 4 (DESIGN-M4 2-4)
(rec
  (type $zxml_elem
    (struct (field $name (ref $zhex_string))
            (field $attrs (ref null $zhex_arr_ref))
            (field $text (mut (ref null $zhex_string)))
            (field $children (mut (ref null $zhex_arr_ref)))))
  (type $zxml_attr (struct (field $name (ref $zhex_string)) (field $value (ref $zhex_string)))))
(global $zxml_i (mut i32) (i32.const 0))
(global $zxml_line (mut i32) (i32.const 1))
(global $zxml_ent (mut (ref null $zhex_string)) (ref.null $zhex_string))
(global $zxml_cp (mut i32) (i32.const 0))
(global $zxml_int_val (mut i64) (i64.const 0))
(global $zxml_names (mut (ref null $zhex_arr_ref)) (ref.null $zhex_arr_ref))
`

// watM4CloVoidRows: the `v` result marker of the closure functype grid
// (§3a) — six void rows for arities 1-2 over the three element reprs. A
// partial needs a hole, so arity 0 is impossible.
const watM4CloVoidRows = `
(type $zhex_clo_i64_r_v (func (param (ref $zhex_closure)) (param i64)))
(type $zhex_clo_i32_r_v (func (param (ref $zhex_closure)) (param i32)))
(type $zhex_clo_ref_r_v (func (param (ref $zhex_closure)) (param (ref null eq))))
(type $zhex_clo_i64_i64_r_v (func (param (ref $zhex_closure)) (param i64) (param i64)))
(type $zhex_clo_i64_i32_r_v (func (param (ref $zhex_closure)) (param i64) (param i32)))
(type $zhex_clo_i64_ref_r_v (func (param (ref $zhex_closure)) (param i64) (param (ref null eq))))
(type $zhex_clo_i32_i64_r_v (func (param (ref $zhex_closure)) (param i32) (param i64)))
(type $zhex_clo_i32_i32_r_v (func (param (ref $zhex_closure)) (param i32) (param i32)))
(type $zhex_clo_i32_ref_r_v (func (param (ref $zhex_closure)) (param i32) (param (ref null eq))))
(type $zhex_clo_ref_i64_r_v (func (param (ref $zhex_closure)) (param (ref null eq)) (param i64)))
(type $zhex_clo_ref_i32_r_v (func (param (ref $zhex_closure)) (param (ref null eq)) (param i32)))
(type $zhex_clo_ref_ref_r_v (func (param (ref $zhex_closure)) (param (ref null eq)) (param (ref null eq))))
`

// watM4CheckedReads: the checked List index read (§4) — in bounds wraps the
// element with M1 boxing, out of bounds is family 5 with the interpreter
// listRead message (details stay interpreter-envelope-only).
const watM4CheckedReads = `
(func $zhex_list_get_checked_i64 (param $list (ref $zhex_list_i64)) (param $index i64) (result (ref $zhex_result))
  (if (i64.ge_u (local.get $index)
        (i64.extend_i32_u (array.len (struct.get $zhex_list_i64 $items (local.get $list)))))
    (then (return (struct.new $zhex_result (i32.const 1)
      (call $zhex_errv (i32.const 5) (call $mx_list_range))))))
  (struct.new $zhex_result (i32.const 0)
    (struct.new $zhex_some_i64
      (array.get $zhex_arr_i64 (struct.get $zhex_list_i64 $items (local.get $list)) (i32.wrap_i64 (local.get $index))))))

(func $zhex_list_get_checked_i32 (param $list (ref $zhex_list_i32)) (param $index i64) (result (ref $zhex_result))
  (if (i64.ge_u (local.get $index)
        (i64.extend_i32_u (array.len (struct.get $zhex_list_i32 $items (local.get $list)))))
    (then (return (struct.new $zhex_result (i32.const 1)
      (call $zhex_errv (i32.const 5) (call $mx_list_range))))))
  (struct.new $zhex_result (i32.const 0)
    (ref.i31
      (array.get $zhex_arr_i32 (struct.get $zhex_list_i32 $items (local.get $list)) (i32.wrap_i64 (local.get $index))))))

(func $zhex_list_get_checked_ref (param $list (ref $zhex_list_ref)) (param $index i64) (result (ref $zhex_result))
  (if (i64.ge_u (local.get $index)
        (i64.extend_i32_u (array.len (struct.get $zhex_list_ref $items (local.get $list)))))
    (then (return (struct.new $zhex_result (i32.const 1)
      (call $zhex_errv (i32.const 5) (call $mx_list_range))))))
  (struct.new $zhex_result (i32.const 0)
    (array.get $zhex_arr_ref (struct.get $zhex_list_ref $items (local.get $list)) (i32.wrap_i64 (local.get $index)))))
`

// watM4XmlLiterals renders the xml message fragments as zero-arg string
// funcs (watStrFn), literals pinned from the interpreter (xml_codec.go) and
// Go 1.27's encoding/xml error phrasing; nothing derives from Go at emit or
// test time.
func watM4XmlLiterals() string {
	literals := map[string]string{
		"$mx_inv_pre":      "invalid XML: XML syntax error on line ",
		"$mx_colon":        ": ",
		"$mx_unexp_eof":    "unexpected EOF",
		"$mx_exp_name":     "expected element name after <",
		"$mx_exp_name_sl":  "expected element name after </",
		"$mx_inv_name":     "invalid XML name: ",
		"$mx_gt":           ">",
		"$mx_unexp_end":    "unexpected end element </",
		"$mx_elem_pre":     "element <",
		"$mx_closed_by":    "> closed by </",
		"$mx_inv_cdata":    "invalid <![ sequence",
		"$mx_exp_sl_gt":    "expected /> in element",
		"$mx_attr_no_eq":   "attribute name without = in element",
		"$mx_attr_unq":     "unquoted or missing attribute value in element",
		"$mx_ent_pre":      "invalid character entity &",
		"$mx_semi":         ";",
		"$mx_nosemi":       " (no semicolon)",
		"$mx_bad_cp":       "illegal character code U+",
		"$mx_ns_mid":       "namespace-qualified name ",
		"$mx_ns_suf":       " is not supported",
		"$mx_pi":           "processing instruction outside the prolog",
		"$mx_dtd":          "DTD and entity declarations are rejected",
		"$mx_multiple":     "multiple root elements",
		"$mx_doc_limit":    "document exceeds element limit",
		"$mx_nest_limit":   "XML nesting limit exceeded",
		"$mx_no_root":      "no root element",
		"$mx_not_utf8":     "XML is not valid UTF-8",
		"$mx_mixed":        "mixed content is not supported",
		"$mx_dup_key":      "duplicate map key",
		"$mx_unknown_attr": "unknown attribute ",
		"$mx_unknown_elem": "unknown element ",
		"$mx_elem_word":    "element ",
		"$mx_attr_word":    "attribute ",
		"$mx_coll_attr":    " collides with a child element",
		"$mx_coll_elem":    " collides with an attribute",
		"$mx_dup_elem":     "duplicate element ",
		"$mx_missing":      "missing required field ",
		"$mx_expected":     "expected ",
		"$mx_true":         "true",
		"$mx_false":        "false",
		"$mx_list_range":   "list index is out of range",
		"$mx_slash":        "/",
	}
	keys := make([]string, 0, len(literals))
	for k := range literals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("\n;; xml message fragments (DESIGN-M4 2)\n")
	for _, k := range keys {
		b.WriteString(watStrFn(k, literals[k]))
	}
	return b.String()
}

// watM4XmlHelpers: cursor primitives over the code-point string (line
// counting is raw-\n and counts every consumed newline, entity-decoded
// ones never), the XML name grammar (ASCII shortcut per §2a: upgrade the
// ranges on divergence), colon handling for the always-fatal namespace
// rule, character legality, hex rendering, entity decoding with Go's
// exact failure texts (surrogate numeric refs decode as U+FFFD, matching
// the probed stdlib), and the type-name lookup over the names table.
const watM4XmlHelpers = `
(func $zxml_peek (param $s (ref $zhex_string)) (result i32)
  (if (i32.ge_u (global.get $zxml_i) (array.len (local.get $s)))
    (then (return (i32.const -1))))
  (array.get $zhex_string (local.get $s) (global.get $zxml_i)))

(func $zxml_peek_at (param $s (ref $zhex_string)) (param $k i32) (result i32)
  (if (i32.ge_u (i32.add (global.get $zxml_i) (local.get $k)) (array.len (local.get $s)))
    (then (return (i32.const -1))))
  (array.get $zhex_string (local.get $s) (i32.add (global.get $zxml_i) (local.get $k))))

(func $zxml_next (param $s (ref $zhex_string)) (result i32)
  (local $c i32)
  (local.set $c (call $zxml_peek (local.get $s)))
  (if (i32.eq (local.get $c) (i32.const -1))
    (then (return (i32.const -1))))
  (global.set $zxml_i (i32.add (global.get $zxml_i) (i32.const 1)))
  (if (i32.eq (local.get $c) (i32.const 10))
    (then (global.set $zxml_line (i32.add (global.get $zxml_line) (i32.const 1)))))
  (local.get $c))

(func $zxml_is_ws (param $c i32) (result i32)
  (i32.or (i32.or (i32.eq (local.get $c) (i32.const 32)) (i32.eq (local.get $c) (i32.const 9)))
          (i32.or (i32.eq (local.get $c) (i32.const 10)) (i32.eq (local.get $c) (i32.const 13)))))

(func $zxml_skip_ws (param $s (ref $zhex_string))
  (block $done
    (loop $ws
      (br_if $done (i32.eqz (call $zxml_is_ws (call $zxml_peek (local.get $s)))))
      (drop (call $zxml_next (local.get $s)))
      (br $ws))))

(func $zxml_name_start (param $c i32) (result i32)
  (i32.or (i32.or (i32.and (i32.ge_u (local.get $c) (i32.const 65)) (i32.le_u (local.get $c) (i32.const 90)))
                  (i32.and (i32.ge_u (local.get $c) (i32.const 97)) (i32.le_u (local.get $c) (i32.const 122))))
          (i32.or (i32.eq (local.get $c) (i32.const 95)) (i32.eq (local.get $c) (i32.const 58)))))

(func $zxml_name_char (param $c i32) (result i32)
  (i32.or (call $zxml_name_start (local.get $c))
          (i32.or (i32.and (i32.ge_u (local.get $c) (i32.const 48)) (i32.le_u (local.get $c) (i32.const 57)))
                  (i32.or (i32.eq (local.get $c) (i32.const 46)) (i32.eq (local.get $c) (i32.const 45))))))

;; scan a run of name chars at the cursor into a fresh string (may be empty);
;; the caller validates the first code point (digit/dot/dash runs spell
;; "invalid XML name: <run>", anything else "expected element name after <")
(func $zxml_scan_name (param $s (ref $zhex_string)) (result (ref $zhex_string))
  (local $b i32)
  (local $n i32)
  (local $i i32)
  (local $r (ref null $zhex_string))
  (local.set $b (global.get $zxml_i))
  (block $done
    (loop $sc
      (br_if $done (i32.eqz (call $zxml_name_char (call $zxml_peek (local.get $s)))))
      (drop (call $zxml_next (local.get $s)))
      (br $sc)))
  (local.set $n (i32.sub (global.get $zxml_i) (local.get $b)))
  (local.set $r (array.new_default $zhex_string (local.get $n)))
  (block $cp
    (loop $cl
      (br_if $cp (i32.ge_u (local.get $i) (local.get $n)))
      (array.set $zhex_string (ref.as_non_null (local.get $r)) (local.get $i)
        (array.get $zhex_string (local.get $s) (i32.add (local.get $b) (local.get $i))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $cl)))
  (ref.as_non_null (local.get $r)))

(func $zxml_has_colon (param $s (ref $zhex_string)) (result i32)
  (local $i i32)
  (block $done
    (loop $l
      (br_if $done (i32.ge_u (local.get $i) (array.len (local.get $s))))
      (if (i32.eq (array.get $zhex_string (local.get $s) (local.get $i)) (i32.const 58))
        (then (return (i32.const 1))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $l)))
  (i32.const 0))

;; the local part after the first ':' (whole string when there is none)
(func $zxml_after_colon (param $s (ref $zhex_string)) (result (ref $zhex_string))
  (local $i i32)
  (block $done
    (loop $l
      (br_if $done (i32.ge_u (local.get $i) (array.len (local.get $s))))
      (if (i32.eq (array.get $zhex_string (local.get $s) (local.get $i)) (i32.const 58))
        (then (return (call $zhex_str_sub (local.get $s) (i32.add (local.get $i) (i32.const 1)) (array.len (local.get $s))))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $l)))
  (local.get $s))

;; Go's isInCharacterRange: 9, 10, 13, 0x20-0xD7FF, 0xE000-0xFFFD, astral
(func $zxml_char_ok (param $c i32) (result i32)
  (i32.or (i32.or (i32.eq (local.get $c) (i32.const 9))
                  (i32.or (i32.eq (local.get $c) (i32.const 10)) (i32.eq (local.get $c) (i32.const 13))))
          (i32.or (i32.and (i32.ge_u (local.get $c) (i32.const 32)) (i32.le_u (local.get $c) (i32.const 55295)))
                  (i32.or (i32.and (i32.ge_u (local.get $c) (i32.const 57344)) (i32.le_u (local.get $c) (i32.const 65533)))
                          (i32.ge_u (local.get $c) (i32.const 65536))))))

(func $zxml_hex_digit (param $v i32) (result i32)
  (select (i32.add (local.get $v) (i32.const 55)) (i32.add (local.get $v) (i32.const 48)) (i32.gt_u (local.get $v) (i32.const 9))))

;; uppercase hex, zero-padded to at least four digits (Go %U)
(func $zxml_hex_min4 (param $c i32) (result (ref $zhex_string))
  (local $w i32)
  (local $r (ref null $zhex_string))
  (local $k i32)
  (local.set $w (i32.const 4))
  (if (i32.gt_u (local.get $c) (i32.const 65535))
    (then (local.set $w (i32.const 6))))
  (local.set $r (array.new_default $zhex_string (local.get $w)))
  (block $d
    (loop $l
      (br_if $d (i32.ge_u (local.get $k) (local.get $w)))
      (array.set $zhex_string (ref.as_non_null (local.get $r)) (local.get $k)
        (call $zxml_hex_digit
          (i32.and (i32.shr_u (local.get $c) (i32.mul (i32.sub (i32.sub (local.get $w) (local.get $k)) (i32.const 1)) (i32.const 4))) (i32.const 15))))
      (local.set $k (i32.add (local.get $k) (i32.const 1)))
      (br $l)))
  (ref.as_non_null (local.get $r)))

(func $zxml_cp_push (param $arr (ref null $zhex_arr_i32)) (param $cp i32) (result (ref $zhex_arr_i32))
  (local $n i32)
  (local $i i32)
  (local $f (ref null $zhex_arr_i32))
  (if (ref.is_null (local.get $arr))
    (then
      (local.set $f (array.new_default $zhex_arr_i32 (i32.const 1)))
      (array.set $zhex_arr_i32 (ref.as_non_null (local.get $f)) (i32.const 0) (local.get $cp))
      (return (ref.as_non_null (local.get $f)))))
  (local.set $n (array.len (ref.as_non_null (local.get $arr))))
  (local.set $f (array.new_default $zhex_arr_i32 (i32.add (local.get $n) (i32.const 1))))
  (block $cp
    (loop $cl
      (br_if $cp (i32.ge_u (local.get $i) (local.get $n)))
      (array.set $zhex_arr_i32 (ref.as_non_null (local.get $f)) (local.get $i)
        (array.get $zhex_arr_i32 (ref.as_non_null (local.get $arr)) (local.get $i)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $cl)))
  (array.set $zhex_arr_i32 (ref.as_non_null (local.get $f)) (local.get $n) (local.get $cp))
  (ref.as_non_null (local.get $f)))

(func $zxml_cps_str (param $a (ref $zhex_arr_i32)) (result (ref $zhex_string))
  (local $n i32)
  (local $i i32)
  (local $r (ref null $zhex_string))
  (local.set $n (array.len (local.get $a)))
  (local.set $r (array.new_default $zhex_string (local.get $n)))
  (block $cp
    (loop $cl
      (br_if $cp (i32.ge_u (local.get $i) (local.get $n)))
      (array.set $zhex_string (ref.as_non_null (local.get $r)) (local.get $i)
        (array.get $zhex_arr_i32 (local.get $a) (local.get $i)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $cl)))
  (ref.as_non_null (local.get $r)))

;; a module string is invalid UTF-8 exactly when it holds a surrogate
(func $zxml_utf8_ok (param $s (ref $zhex_string)) (result i32)
  (local $n i32)
  (local $i i32)
  (local $c i32)
  (local.set $n (array.len (local.get $s)))
  (block $ok
    (loop $l
      (br_if $ok (i32.ge_u (local.get $i) (local.get $n)))
      (local.set $c (array.get $zhex_string (local.get $s) (local.get $i)))
      (if (i32.and (i32.ge_u (local.get $c) (i32.const 55296)) (i32.le_u (local.get $c) (i32.const 57343)))
        (then (return (i32.const 0))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $l)))
  (i32.const 1))

;; $zxml_names holds [schema node, type name] pairs filled by the emitter's
;; schema builders; a miss is unreachable (every node registers when built)
(func $zxml_node_name (param $s (ref $zjson_schema)) (result (ref $zhex_string))
  (local $tab (ref null $zhex_arr_ref))
  (local $n i32)
  (local $i i32)
  (local.set $tab (global.get $zxml_names))
  (if (ref.is_null (local.get $tab))
    (then (return (array.new_fixed $zhex_string 0))))
  (local.set $n (array.len (ref.as_non_null (local.get $tab))))
  (block $done
    (loop $scan
      (br_if $done (i32.ge_u (local.get $i) (local.get $n)))
      (if (ref.eq (array.get $zhex_arr_ref (ref.as_non_null (local.get $tab)) (local.get $i)) (local.get $s))
        (then
          (return (ref.cast (ref $zhex_string)
            (array.get $zhex_arr_ref (ref.as_non_null (local.get $tab)) (i32.add (local.get $i) (i32.const 1)))))))
      (local.set $i (i32.add (local.get $i) (i32.const 2)))
      (br $scan)))
  (array.new_fixed $zhex_string 0))
`

// watM4XmlParse: one code-point-domain scanner mirroring Go's decoder for
// every fixture-reachable behavior (§2a) with the interpreter's own checks
// in its exact order: element-name prefix (or a default xmlns=, which Go
// resolves onto the element) -> multiple roots -> element count -> depth ->
// prefixed attributes. ProcInst outside the prolog and every Directive are
// interpreter-level failures; CDATA is raw CharData; text accumulates with
// \r\n/\r -> \n translation like the stdlib.
const watM4XmlParse = `
(func $zxml_syntax_err (param $detail (ref $zhex_string)) (result (ref $zhex_result))
  (call $zjson_fail
    (call $zhex_cat4 (call $mx_inv_pre)
      (call $zhex_int_to_string (i64.extend_i32_s (global.get $zxml_line)))
      (call $mx_colon) (local.get $detail))
    (array.new_fixed $zhex_string 0)))

;; the entity failure kinds $zxml_entity reports
(func $zxml_entity_err (param $r i32) (result (ref $zhex_result))
  (if (i32.eq (local.get $r) (i32.const -1))
    (then (return (call $zxml_syntax_err
      (call $zhex_cat3 (call $mx_ent_pre) (ref.as_non_null (global.get $zxml_ent)) (call $mx_semi))))))
  (if (i32.eq (local.get $r) (i32.const -2))
    (then (return (call $zxml_syntax_err
      (call $zhex_cat3 (call $mx_ent_pre) (ref.as_non_null (global.get $zxml_ent)) (call $mx_nosemi))))))
  (call $zxml_syntax_err
    (call $zhex_str_concat (call $mx_bad_cp) (call $zxml_hex_min4 (global.get $zxml_cp)))))

;; decode one entity; the cursor sits just past '&'. Result >= 0 is the
;; code point; -1 invalid-entity-with-semicolon, -2 no-semicolon, -3
;; illegal character code (the literal lands in $zxml_ent / $zxml_cp).
;; shortcut: numeric surrogate refs decode as U+FFFD, matching the probed
;; stdlib rather than the letter of the legality table (upgrade on divergence).
(func $zxml_entity (param $s (ref $zhex_string)) (result i32)
  (local $b i32)
  (local $c i32)
  (local $hex i32)
  (local $digs i32)
  (local $v i64)
  (local $ovf i32)
  (local $d i32)
  (local $cp i32)
  (local.set $b (global.get $zxml_i))
  (local.set $c (call $zxml_peek (local.get $s)))
  (if (i32.eq (local.get $c) (i32.const -1))
    (then
      (global.set $zxml_ent (array.new_fixed $zhex_string 0))
      (return (i32.const -2))))
  (if (i32.eq (local.get $c) (i32.const 35))
    (then
      (drop (call $zxml_next (local.get $s)))
      (if (i32.eq (call $zxml_peek (local.get $s)) (i32.const 120))
        (then
          (drop (call $zxml_next (local.get $s)))
          (local.set $hex (i32.const 1))))
      (block $digits
        (loop $dl
          (local.set $c (call $zxml_peek (local.get $s)))
          (if (i32.and (i32.ge_u (local.get $c) (i32.const 48)) (i32.le_u (local.get $c) (i32.const 57)))
            (then
              (local.set $d (i32.sub (local.get $c) (i32.const 48)))
              (drop (call $zxml_next (local.get $s)))
              (local.set $digs (i32.add (local.get $digs) (i32.const 1)))
              (if (i32.eqz (local.get $ovf))
                (then
                  (local.set $v (i64.add (i64.mul (local.get $v) (select (i64.const 16) (i64.const 10) (local.get $hex))) (i64.extend_i32_u (local.get $d))))
                  (if (i64.gt_u (local.get $v) (i64.const 2147483647))
                    (then (local.set $ovf (i32.const 1))))))
              (br $dl)))
          (if (i32.and (i32.eq (local.get $hex) (i32.const 1))
                       (i32.or (i32.and (i32.ge_u (local.get $c) (i32.const 97)) (i32.le_u (local.get $c) (i32.const 102)))
                               (i32.and (i32.ge_u (local.get $c) (i32.const 65)) (i32.le_u (local.get $c) (i32.const 70)))))
            (then
              (local.set $d (i32.sub (i32.or (local.get $c) (i32.const 32)) (i32.const 87)))
              (drop (call $zxml_next (local.get $s)))
              (local.set $digs (i32.add (local.get $digs) (i32.const 1)))
              (if (i32.eqz (local.get $ovf))
                (then
                  (local.set $v (i64.add (i64.mul (local.get $v) (i64.const 16)) (i64.extend_i32_u (local.get $d))))
                  (if (i64.gt_u (local.get $v) (i64.const 2147483647))
                    (then (local.set $ovf (i32.const 1))))))
              (br $dl)))
          (br $digits)))
      (if (i32.eq (call $zxml_peek (local.get $s)) (i32.const 59))
        (then
          (drop (call $zxml_next (local.get $s)))
          (global.set $zxml_ent
            (call $zhex_str_sub (local.get $s) (local.get $b) (i32.sub (global.get $zxml_i) (i32.const 1))))
          (if (i32.eqz (local.get $digs))
            (then (return (i32.const -1))))
          (if (i32.or (i32.eq (local.get $ovf) (i32.const 1)) (i64.gt_u (local.get $v) (i64.const 1114111)))
            (then (return (i32.const -1))))
          (local.set $cp (i32.wrap_i64 (local.get $v)))
          (if (i32.and (i32.ge_u (local.get $cp) (i32.const 55296)) (i32.le_u (local.get $cp) (i32.const 57343)))
            (then (return (i32.const 65533))))
          (if (i32.eqz (call $zxml_char_ok (local.get $cp)))
            (then
              (global.set $zxml_cp (local.get $cp))
              (return (i32.const -3))))
          (return (local.get $cp))))
      (global.set $zxml_ent (call $zhex_str_sub (local.get $s) (local.get $b) (global.get $zxml_i)))
      (return (i32.const -2))))
  (if (i32.eqz (call $zxml_name_start (local.get $c)))
    (then
      (if (i32.eq (local.get $c) (i32.const 59))
        (then
          (drop (call $zxml_next (local.get $s)))
          (global.set $zxml_ent (array.new_fixed $zhex_string 0))
          (return (i32.const -1))))
      (global.set $zxml_ent (array.new_fixed $zhex_string 0))
      (return (i32.const -2))))
  (global.set $zxml_ent (call $zxml_scan_name (local.get $s)))
  (if (i32.eq (call $zxml_peek (local.get $s)) (i32.const 59))
    (then
      (drop (call $zxml_next (local.get $s)))
      (if (call $zhex_str_eq (ref.as_non_null (global.get $zxml_ent)) (call $zxml_lit_lt))
        (then (return (i32.const 60))))
      (if (call $zhex_str_eq (ref.as_non_null (global.get $zxml_ent)) (call $zxml_lit_gt))
        (then (return (i32.const 62))))
      (if (call $zhex_str_eq (ref.as_non_null (global.get $zxml_ent)) (call $zxml_lit_amp))
        (then (return (i32.const 38))))
      (if (call $zhex_str_eq (ref.as_non_null (global.get $zxml_ent)) (call $zxml_lit_apos))
        (then (return (i32.const 39))))
      (if (call $zhex_str_eq (ref.as_non_null (global.get $zxml_ent)) (call $zxml_lit_quot))
        (then (return (i32.const 34))))
      (return (i32.const -1))))
  (return (i32.const -2)))

(func $zxml_lit_lt (result (ref $zhex_string)) (array.new_fixed $zhex_string 2 (i32.const 108) (i32.const 116)))
(func $zxml_lit_gt (result (ref $zhex_string)) (array.new_fixed $zhex_string 2 (i32.const 103) (i32.const 116)))
(func $zxml_lit_amp (result (ref $zhex_string)) (array.new_fixed $zhex_string 3 (i32.const 97) (i32.const 109) (i32.const 112)))
(func $zxml_lit_apos (result (ref $zhex_string)) (array.new_fixed $zhex_string 4 (i32.const 97) (i32.const 112) (i32.const 111) (i32.const 115)))
(func $zxml_lit_quot (result (ref $zhex_string)) (array.new_fixed $zhex_string 4 (i32.const 113) (i32.const 117) (i32.const 111) (i32.const 116)))

;; "/" + join(names, "/") — the interpreter xmlStackPath; empty stack is "/"
(func $zxml_stack_path (param $stack (ref null $zhex_arr_ref)) (param $depth i32) (result (ref $zhex_string))
  (local $r (ref null $zhex_string))
  (local $i i32)
  (local.set $r (array.new_fixed $zhex_string 1 (i32.const 47)))
  (block $done
    (loop $l
      (br_if $done (i32.ge_u (local.get $i) (local.get $depth)))
      (if (i32.gt_u (local.get $i) (i32.const 0))
        (then (local.set $r (call $zhex_str_concat (ref.as_non_null (local.get $r)) (call $mx_slash)))))
      (local.set $r (call $zhex_str_concat (ref.as_non_null (local.get $r))
        (struct.get $zxml_elem $name
          (ref.cast (ref $zxml_elem) (array.get $zhex_arr_ref (ref.as_non_null (local.get $stack)) (local.get $i))))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $l)))
  (ref.as_non_null (local.get $r)))

;; pop must shrink the stack array: the next push appends at the array end,
;; so a stale entry past $depth would take the next element's slot
(func $zxml_stack_trunc (param $arr (ref null $zhex_arr_ref)) (param $n i32) (result (ref null $zhex_arr_ref))
  (local $i i32)
  (local $f (ref null $zhex_arr_ref))
  (if (ref.is_null (local.get $arr))
    (then (return (ref.null $zhex_arr_ref))))
  (local.set $f (array.new_default $zhex_arr_ref (local.get $n)))
  (block $cp
    (loop $cl
      (br_if $cp (i32.ge_u (local.get $i) (local.get $n)))
      (array.set $zhex_arr_ref (ref.as_non_null (local.get $f)) (local.get $i)
        (array.get $zhex_arr_ref (ref.as_non_null (local.get $arr)) (local.get $i)))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $cl)))
  (local.get $f))

(func $zxml_field_path (param $path (ref $zhex_string)) (param $name (ref $zhex_string)) (result (ref $zhex_string))
  (call $zhex_str_concat (local.get $path) (call $zhex_str_concat (call $mx_slash) (local.get $name))))

(func $zxml_path_index (param $path (ref $zhex_string)) (param $i i32) (result (ref $zhex_string))
  (call $zhex_str_concat (local.get $path)
    (call $zhex_str_concat (call $mx_slash) (call $zhex_int_to_string (i64.extend_i32_u (local.get $i))))))

(func $zxml_ns_fail (param $local (ref $zhex_string)) (param $path (ref $zhex_string)) (result (ref $zhex_result))
  (call $zjson_fail
    (call $zhex_cat3 (call $mx_ns_mid) (call $zhex_quote (local.get $local)) (call $mx_ns_suf))
    (local.get $path)))

(func $zxml_attr_name (param $a (ref null eq)) (result (ref $zhex_string))
  (struct.get $zxml_attr $name (ref.cast (ref $zxml_attr) (local.get $a))))

(func $zxml_attr_value (param $a (ref null eq)) (result (ref $zhex_string))
  (struct.get $zxml_attr $value (ref.cast (ref $zxml_attr) (local.get $a))))

(func $zxml_child (param $arr (ref $zhex_arr_ref)) (param $i i32) (result (ref $zxml_elem))
  (ref.cast (ref $zxml_elem) (array.get $zhex_arr_ref (local.get $arr) (local.get $i))))

(func $zxml_text (param $e (ref $zxml_elem)) (result (ref $zhex_string))
  (ref.as_non_null (struct.get $zxml_elem $text (local.get $e))))

;; elem.text += chunk (O(n^2) accepted; parity beats speed)
(func $zxml_add_text (param $e (ref $zxml_elem)) (param $chunk (ref $zhex_string))
  (struct.set $zxml_elem $text (local.get $e)
    (call $zhex_str_concat (call $zxml_text (local.get $e)) (local.get $chunk))))

(func $zxml_parse (param $s (ref $zhex_string)) (result (ref $zhex_result))
  (local $n i32)
  (local $c i32)
  (local $root (ref null eq))
  (local $seen i32)
  (local $count i32)
  (local $stack (ref null $zhex_arr_ref))
  (local $depth i32)
  (local $name (ref null $zhex_string))
  (local $clo (ref null $zhex_string))
  (local $attrs (ref null $zhex_arr_ref))
  (local $an (ref null $zhex_string))
  (local $sc i32)
  (local $top (ref $zxml_elem))
  (local $elm (ref $zxml_elem))
  (local $cps (ref null $zhex_arr_i32))
  (local $r i32)
  (local $j i32)
  (local $k i32)
  (local $q i32)
  (local $ac i32)
  (global.set $zxml_i (i32.const 0))
  (global.set $zxml_line (i32.const 1))
  (local.set $n (array.len (local.get $s)))
  (block $eof
    (loop $main
      ;; ---- one text run until '<' ----
      (local.set $cps (ref.null $zhex_arr_i32))
      (block $trun
        (loop $tl
          (br_if $trun (i32.ge_u (global.get $zxml_i) (local.get $n)))
          (local.set $c (call $zxml_peek (local.get $s)))
          (br_if $trun (i32.eq (local.get $c) (i32.const 60)))
          (drop (call $zxml_next (local.get $s)))
          (if (i32.eqz (call $zxml_char_ok (local.get $c)))
            (then
              (global.set $zxml_cp (local.get $c))
              (return (call $zxml_entity_err (i32.const -3)))))
          (if (i32.eq (local.get $c) (i32.const 38))
            (then
              (local.set $r (call $zxml_entity (local.get $s)))
              (if (i32.lt_s (local.get $r) (i32.const 0))
                (then (return (call $zxml_entity_err (local.get $r)))))
              (local.set $cps (call $zxml_cp_push (local.get $cps) (local.get $r)))
              (br $tl)))
          (if (i32.eq (local.get $c) (i32.const 13))
            (then
              (if (i32.eq (call $zxml_peek (local.get $s)) (i32.const 10))
                (then (drop (call $zxml_next (local.get $s)))))
              (local.set $c (i32.const 10))))
          (local.set $cps (call $zxml_cp_push (local.get $cps) (local.get $c)))
          (br $tl)))
      (if (i32.and (i32.gt_u (local.get $depth) (i32.const 0)) (i32.eqz (ref.is_null (local.get $cps))))
        (then
          (local.set $top (call $zxml_child (ref.as_non_null (local.get $stack)) (i32.sub (local.get $depth) (i32.const 1))))
          (call $zxml_add_text (local.get $top) (call $zxml_cps_str (ref.as_non_null (local.get $cps))))))
      (br_if $eof (i32.ge_u (global.get $zxml_i) (local.get $n)))
      (drop (call $zxml_next (local.get $s)))
      (local.set $c (call $zxml_peek (local.get $s)))
      (if (i32.eq (local.get $c) (i32.const -1))
        (then (return (call $zxml_syntax_err (call $mx_unexp_eof)))))
      ;; ---- close tag ----
      (if (i32.eq (local.get $c) (i32.const 47))
        (then
          (drop (call $zxml_next (local.get $s)))
          (local.set $clo (call $zxml_scan_name (local.get $s)))
          (if (i32.eq (array.len (ref.as_non_null (local.get $clo))) (i32.const 0))
            (then (return (call $zxml_syntax_err (call $mx_exp_name_sl)))))
          (if (i32.eqz (call $zxml_name_start (array.get $zhex_string (ref.as_non_null (local.get $clo)) (i32.const 0))))
            (then (return (call $zxml_syntax_err (call $zhex_str_concat (call $mx_inv_name) (ref.as_non_null (local.get $clo)))))))
          (call $zxml_skip_ws (local.get $s))
          (if (i32.eq (call $zxml_peek (local.get $s)) (i32.const -1))
            (then (return (call $zxml_syntax_err (call $mx_unexp_eof)))))
          (if (i32.ne (call $zxml_peek (local.get $s)) (i32.const 62))
            (then (return (call $zxml_syntax_err (call $mx_exp_sl_gt)))))
          (drop (call $zxml_next (local.get $s)))
          (if (i32.eqz (local.get $depth))
            (then (return (call $zxml_syntax_err
              (call $zhex_cat3 (call $mx_unexp_end) (ref.as_non_null (local.get $clo)) (call $mx_gt))))))
          (local.set $top (call $zxml_child (ref.as_non_null (local.get $stack)) (i32.sub (local.get $depth) (i32.const 1))))
          (if (i32.eqz (call $zhex_str_eq (struct.get $zxml_elem $name (local.get $top)) (ref.as_non_null (local.get $clo))))
            (then (return (call $zxml_syntax_err
              (call $zhex_cat5 (call $mx_elem_pre) (struct.get $zxml_elem $name (local.get $top)) (call $mx_closed_by) (ref.as_non_null (local.get $clo)) (call $mx_gt))))))
          (local.set $depth (i32.sub (local.get $depth) (i32.const 1)))
          (local.set $stack (call $zxml_stack_trunc (local.get $stack) (local.get $depth)))
          (br $main)))
      ;; ---- ProcInst ----
      (if (i32.eq (local.get $c) (i32.const 63))
        (then
          (drop (call $zxml_next (local.get $s)))
          (block $pid
            (loop $pil
              (if (i32.eq (call $zxml_peek (local.get $s)) (i32.const -1))
                (then (return (call $zxml_syntax_err (call $mx_unexp_eof)))))
              (if (i32.and (i32.eq (call $zxml_peek (local.get $s)) (i32.const 63))
                           (i32.eq (call $zxml_peek_at (local.get $s) (i32.const 1)) (i32.const 62)))
                (then (br $pid)))
              (drop (call $zxml_next (local.get $s)))
              (br $pil)))
          (drop (call $zxml_next (local.get $s)))
          (drop (call $zxml_next (local.get $s)))
          (if (i32.or (i32.gt_u (local.get $depth) (i32.const 0)) (local.get $seen))
            (then (return (call $zjson_fail (call $mx_pi) (array.new_fixed $zhex_string 0)))))
          (br $main)))
      ;; ---- comment / CDATA / directive ----
      (if (i32.eq (local.get $c) (i32.const 33))
        (then
          (drop (call $zxml_next (local.get $s)))
          (if (i32.and (i32.eq (call $zxml_peek (local.get $s)) (i32.const 45))
                       (i32.eq (call $zxml_peek_at (local.get $s) (i32.const 1)) (i32.const 45)))
            (then
              (drop (call $zxml_next (local.get $s)))
              (drop (call $zxml_next (local.get $s)))
              (block $cd
                (loop $cl
                  (if (i32.eq (call $zxml_peek (local.get $s)) (i32.const -1))
                    (then (return (call $zxml_syntax_err (call $mx_unexp_eof)))))
                  (if (i32.and (i32.and (i32.eq (call $zxml_peek (local.get $s)) (i32.const 45))
                                        (i32.eq (call $zxml_peek_at (local.get $s) (i32.const 1)) (i32.const 45)))
                               (i32.eq (call $zxml_peek_at (local.get $s) (i32.const 2)) (i32.const 62)))
                    (then (br $cd)))
                  (drop (call $zxml_next (local.get $s)))
                  (br $cl)))
              (drop (call $zxml_next (local.get $s)))
              (drop (call $zxml_next (local.get $s)))
              (drop (call $zxml_next (local.get $s)))
              (br $main)))
          (if (i32.and (i32.and (i32.and (i32.eq (call $zxml_peek (local.get $s)) (i32.const 91))
                                          (i32.eq (call $zxml_peek_at (local.get $s) (i32.const 1)) (i32.const 67)))
                                 (i32.and (i32.eq (call $zxml_peek_at (local.get $s) (i32.const 2)) (i32.const 68))
                                          (i32.eq (call $zxml_peek_at (local.get $s) (i32.const 3)) (i32.const 65))))
                       (i32.and (i32.eq (call $zxml_peek_at (local.get $s) (i32.const 4)) (i32.const 84))
                                (i32.and (i32.eq (call $zxml_peek_at (local.get $s) (i32.const 5)) (i32.const 65))
                                         (i32.eq (call $zxml_peek_at (local.get $s) (i32.const 6)) (i32.const 91)))))
            (then
              (local.set $k (i32.const 0))
              (block $sk (loop $skl (br_if $sk (i32.ge_u (local.get $k) (i32.const 7))) (drop (call $zxml_next (local.get $s))) (local.set $k (i32.add (local.get $k) (i32.const 1))) (br $skl)))
              (local.set $cps (ref.null $zhex_arr_i32))
              (block $cd
                (loop $cl
                  (if (i32.eq (call $zxml_peek (local.get $s)) (i32.const -1))
                    (then (return (call $zxml_syntax_err (call $mx_unexp_eof)))))
                  (if (i32.and (i32.and (i32.eq (call $zxml_peek (local.get $s)) (i32.const 93))
                                        (i32.eq (call $zxml_peek_at (local.get $s) (i32.const 1)) (i32.const 93)))
                               (i32.eq (call $zxml_peek_at (local.get $s) (i32.const 2)) (i32.const 62)))
                    (then (br $cd)))
                  (local.set $c (call $zxml_next (local.get $s)))
                  (if (i32.eqz (call $zxml_char_ok (local.get $c)))
                    (then
                      (global.set $zxml_cp (local.get $c))
                      (return (call $zxml_entity_err (i32.const -3)))))
                  (if (i32.eq (local.get $c) (i32.const 13))
                    (then
                      (if (i32.eq (call $zxml_peek (local.get $s)) (i32.const 10))
                        (then (drop (call $zxml_next (local.get $s)))))
                      (local.set $c (i32.const 10))))
                  (local.set $cps (call $zxml_cp_push (local.get $cps) (local.get $c)))
                  (br $cl)))
              (drop (call $zxml_next (local.get $s)))
              (drop (call $zxml_next (local.get $s)))
              (drop (call $zxml_next (local.get $s)))
              (if (i32.and (i32.gt_u (local.get $depth) (i32.const 0)) (i32.eqz (ref.is_null (local.get $cps))))
                (then
                  (local.set $top (call $zxml_child (ref.as_non_null (local.get $stack)) (i32.sub (local.get $depth) (i32.const 1))))
                  (call $zxml_add_text (local.get $top) (call $zxml_cps_str (ref.as_non_null (local.get $cps))))))
              (br $main)))
          (if (i32.eq (call $zxml_peek (local.get $s)) (i32.const 91))
            (then (return (call $zxml_syntax_err (call $mx_inv_cdata)))))
          (block $dd
            (loop $dl
              (if (i32.eq (call $zxml_peek (local.get $s)) (i32.const -1))
                (then (return (call $zxml_syntax_err (call $mx_unexp_eof)))))
              (br_if $dd (i32.eq (call $zxml_peek (local.get $s)) (i32.const 62)))
              (drop (call $zxml_next (local.get $s)))
              (br $dl)))
          (drop (call $zxml_next (local.get $s)))
          (return (call $zjson_fail (call $mx_dtd) (array.new_fixed $zhex_string 0)))))
      ;; ---- start tag ----
      (local.set $name (call $zxml_scan_name (local.get $s)))
      (if (i32.eq (array.len (ref.as_non_null (local.get $name))) (i32.const 0))
        (then (return (call $zxml_syntax_err (call $mx_exp_name)))))
      (if (i32.eqz (call $zxml_name_start (array.get $zhex_string (ref.as_non_null (local.get $name)) (i32.const 0))))
        (then (return (call $zxml_syntax_err (call $zhex_str_concat (call $mx_inv_name) (ref.as_non_null (local.get $name)))))))
      (local.set $attrs (ref.null $zhex_arr_ref))
      (local.set $sc (i32.const 0))
      (block $attdone
        (loop $al
          (call $zxml_skip_ws (local.get $s))
          (local.set $c (call $zxml_peek (local.get $s)))
          (if (i32.eq (local.get $c) (i32.const -1))
            (then (return (call $zxml_syntax_err (call $mx_unexp_eof)))))
          (if (i32.eq (local.get $c) (i32.const 62))
            (then
              (drop (call $zxml_next (local.get $s)))
              (br $attdone)))
          (if (i32.eq (local.get $c) (i32.const 47))
            (then
              (drop (call $zxml_next (local.get $s)))
              (if (i32.eq (call $zxml_peek (local.get $s)) (i32.const -1))
                (then (return (call $zxml_syntax_err (call $mx_unexp_eof)))))
              (if (i32.ne (call $zxml_next (local.get $s)) (i32.const 62))
                (then (return (call $zxml_syntax_err (call $mx_exp_sl_gt)))))
              (local.set $sc (i32.const 1))
              (br $attdone)))
          (local.set $an (call $zxml_scan_name (local.get $s)))
          (if (i32.eq (array.len (ref.as_non_null (local.get $an))) (i32.const 0))
            (then (return (call $zxml_syntax_err (call $mx_attr_no_eq)))))
          (if (i32.eqz (call $zxml_name_start (array.get $zhex_string (ref.as_non_null (local.get $an)) (i32.const 0))))
            (then (return (call $zxml_syntax_err (call $zhex_str_concat (call $mx_inv_name) (ref.as_non_null (local.get $an)))))))
          (call $zxml_skip_ws (local.get $s))
          (if (i32.ne (call $zxml_peek (local.get $s)) (i32.const 61))
            (then (return (call $zxml_syntax_err (call $mx_attr_no_eq)))))
          (drop (call $zxml_next (local.get $s)))
          (call $zxml_skip_ws (local.get $s))
          (local.set $q (call $zxml_next (local.get $s)))
          (if (i32.and (i32.ne (local.get $q) (i32.const 34)) (i32.ne (local.get $q) (i32.const 39)))
            (then (return (call $zxml_syntax_err (call $mx_attr_unq)))))
          (local.set $cps (ref.null $zhex_arr_i32))
          (block $vd
            (loop $vl
              (local.set $c (call $zxml_peek (local.get $s)))
              (if (i32.eq (local.get $c) (i32.const -1))
                (then (return (call $zxml_syntax_err (call $mx_unexp_eof)))))
              (br_if $vd (i32.eq (local.get $c) (local.get $q)))
              (drop (call $zxml_next (local.get $s)))
              (if (i32.eqz (call $zxml_char_ok (local.get $c)))
                (then
                  (global.set $zxml_cp (local.get $c))
                  (return (call $zxml_entity_err (i32.const -3)))))
              (if (i32.eq (local.get $c) (i32.const 38))
                (then
                  (local.set $r (call $zxml_entity (local.get $s)))
                  (if (i32.lt_s (local.get $r) (i32.const 0))
                    (then (return (call $zxml_entity_err (local.get $r)))))
                  (local.set $cps (call $zxml_cp_push (local.get $cps) (local.get $r)))
                  (br $vl)))
              (if (i32.eq (local.get $c) (i32.const 13))
                (then
                  (if (i32.eq (call $zxml_peek (local.get $s)) (i32.const 10))
                    (then (drop (call $zxml_next (local.get $s)))))
                  (local.set $c (i32.const 10))))
              (local.set $cps (call $zxml_cp_push (local.get $cps) (local.get $c)))
              (br $vl)))
          (drop (call $zxml_next (local.get $s)))
          (local.set $attrs (call $zjson_arr_push (local.get $attrs)
            (struct.new $zxml_attr (ref.as_non_null (local.get $an)) (call $zxml_cps_str (ref.as_non_null (local.get $cps))))))
          (br $al)))
      (local.set $ac (i32.const 0))
      (if (i32.eqz (ref.is_null (local.get $attrs)))
        (then (local.set $ac (array.len (ref.as_non_null (local.get $attrs))))))
      ;; 1. element namespace: a prefixed name, or a default xmlns= (which
      ;;    Go resolves onto the element) — path excludes the element
      (if (call $zxml_has_colon (ref.as_non_null (local.get $name)))
        (then (return (call $zxml_ns_fail (call $zxml_after_colon (ref.as_non_null (local.get $name)))
          (call $zxml_stack_path (local.get $stack) (local.get $depth))))))
      (local.set $j (i32.const 0))
      (block $xd
        (loop $xl
          (br_if $xd (i32.ge_u (local.get $j) (local.get $ac)))
          (if (call $zhex_str_eq (call $zxml_attr_name (array.get $zhex_arr_ref (ref.as_non_null (local.get $attrs)) (local.get $j))) (call $zxml_lit_xmlns))
            (then (return (call $zxml_ns_fail (ref.as_non_null (local.get $name))
              (call $zxml_stack_path (local.get $stack) (local.get $depth))))))
          (local.set $j (i32.add (local.get $j) (i32.const 1)))
          (br $xl)))
      ;; 2. multiple roots  3. element count  4. nesting depth
      (if (i32.and (local.get $seen) (i32.eqz (local.get $depth)))
        (then (return (call $zjson_fail (call $mx_multiple) (call $zxml_stack_path (local.get $stack) (local.get $depth))))))
      (local.set $count (i32.add (local.get $count) (i32.const 1)))
      (if (i32.gt_u (local.get $count) (i32.const 1000000))
        (then (return (call $zjson_fail (call $mx_doc_limit) (call $zxml_stack_path (local.get $stack) (local.get $depth))))))
      (if (i32.ge_u (local.get $depth) (i32.const 256))
        (then (return (call $zjson_fail (call $mx_nest_limit) (call $zxml_stack_path (local.get $stack) (local.get $depth))))))
      ;; 5. prefixed attributes, left to right
      (local.set $j (i32.const 0))
      (block $yd
        (loop $yl
          (br_if $yd (i32.ge_u (local.get $j) (local.get $ac)))
          (if (call $zxml_has_colon (call $zxml_attr_name (array.get $zhex_arr_ref (ref.as_non_null (local.get $attrs)) (local.get $j))))
            (then (return (call $zxml_ns_fail
              (call $zxml_after_colon (call $zxml_attr_name (array.get $zhex_arr_ref (ref.as_non_null (local.get $attrs)) (local.get $j))))
              (call $zxml_field_path (call $zxml_stack_path (local.get $stack) (local.get $depth))
                (call $zxml_after_colon (call $zxml_attr_name (array.get $zhex_arr_ref (ref.as_non_null (local.get $attrs)) (local.get $j)))))))))
          (local.set $j (i32.add (local.get $j) (i32.const 1)))
          (br $yl)))
      (local.set $elm (struct.new $zxml_elem (ref.as_non_null (local.get $name))
        (local.get $attrs)
        (array.new_fixed $zhex_string 0)
        (array.new_fixed $zhex_arr_ref 0)))
      (if (i32.eqz (local.get $depth))
        (then
          (local.set $root (local.get $elm))
          (local.set $seen (i32.const 1)))
        (else
          (local.set $top (call $zxml_child (ref.as_non_null (local.get $stack)) (i32.sub (local.get $depth) (i32.const 1))))
          (struct.set $zxml_elem $children (local.get $top)
            (call $zjson_arr_push (struct.get $zxml_elem $children (local.get $top)) (local.get $elm)))))
      (if (i32.eqz (local.get $sc))
        (then
          (local.set $stack (call $zjson_arr_push (local.get $stack) (local.get $elm)))
          (local.set $depth (i32.add (local.get $depth) (i32.const 1)))))
      (br $main)))
  (if (i32.gt_u (local.get $depth) (i32.const 0))
    (then (return (call $zxml_syntax_err (call $mx_unexp_eof)))))
  (if (i32.eqz (local.get $seen))
    (then (return (call $zjson_fail (call $mx_no_root) (array.new_fixed $zhex_string 0)))))
  (struct.new $zhex_result (i32.const 0) (ref.as_non_null (local.get $root))))

(func $zxml_lit_xmlns (result (ref $zhex_string)) (array.new_fixed $zhex_string 5 (i32.const 120) (i32.const 109) (i32.const 108) (i32.const 110) (i32.const 115)))
`

// watM4XmlDecode: the schema-directed decode (§2b) over the M3 json schema
// machinery — the linear ref.eq clone memo for aliasing-preserving default
// fills, the make/get record thunks, and $zjson_fail's envelope. Option
// values wrap per the inner element's repr (Int keeps its $zhex_some_i64
// payload as the Option repr, Bool re-boxes as i31, refs wrap some_ref) —
// the interpreter's uniform choice value, not the M3 json prelude's
// always-some_ref shortcut.
const watM4XmlDecode = `
;; exactXMLInt with ParseInt's split: all-digit overflow is a range failure,
;; any non-digit or empty digit run a type failure
(func $zxml_exact_int (param $t (ref $zhex_string)) (result i32)
  (local $n i32)
  (local $i i32)
  (local $neg i32)
  (local $d i32)
  (local $acc i64)
  (local $ovf i32)
  (local $start i32)
  (local.set $t (call $zhex_text_trim (local.get $t)))
  (local.set $n (array.len (local.get $t)))
  (if (i32.gt_u (local.get $n) (i32.const 0))
    (then
      ;; nested tests: the leading char read must not run on empty text
      ;; (wasm's i32.and does not short-circuit)
      (if (i32.or (i32.eq (array.get $zhex_string (local.get $t) (i32.const 0)) (i32.const 45))
                  (i32.eq (array.get $zhex_string (local.get $t) (i32.const 0)) (i32.const 43)))
        (then
          (local.set $neg (i32.eq (array.get $zhex_string (local.get $t) (i32.const 0)) (i32.const 45)))
          (local.set $i (i32.const 1))))))
  (local.set $start (local.get $i))
  (block $digits
    (loop $dl
      (br_if $digits (i32.ge_u (local.get $i) (local.get $n)))
      (local.set $d (i32.sub (array.get $zhex_string (local.get $t) (local.get $i)) (i32.const 48)))
      (if (i32.or (i32.lt_s (local.get $d) (i32.const 0)) (i32.gt_s (local.get $d) (i32.const 9)))
        (then (return (i32.const 2))))
      ;; the to-int overflow steps, latched: once ovf is set, skip them all
      (if (i32.eqz (local.get $ovf))
        (then
          (if (i64.lt_s (local.get $acc) (i64.const -922337203685477580))
            (then (local.set $ovf (i32.const 1))))))
      (if (i32.eqz (local.get $ovf))
        (then (local.set $acc (i64.mul (local.get $acc) (i64.const 10)))))
      (if (i32.eqz (local.get $ovf))
        (then
          (if (i64.lt_s (local.get $acc) (i64.add (i64.const -9223372036854775808) (i64.extend_i32_u (local.get $d))))
            (then (local.set $ovf (i32.const 1))))))
      (if (i32.eqz (local.get $ovf))
        (then (local.set $acc (i64.sub (local.get $acc) (i64.extend_i32_u (local.get $d))))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br $dl)))
  (if (i32.eq (local.get $i) (local.get $start))
    (then (return (i32.const 2))))
  (if (local.get $ovf)
    (then (return (i32.const 1))))
  (if (i32.and (i32.eqz (local.get $neg)) (i64.eq (local.get $acc) (i64.const -9223372036854775808)))
    (then (return (i32.const 1))))
  (global.set $zxml_int_val
    (if (result i64) (local.get $neg) (then (local.get $acc)) (else (i64.sub (i64.const 0) (local.get $acc)))))
  (i32.const 0))

(func $zxml_expected_fail (param $s (ref $zjson_schema)) (param $path (ref $zhex_string)) (result (ref $zhex_result))
  (call $zjson_fail
    (call $zhex_str_concat (call $mx_expected) (call $zxml_node_name (local.get $s)))
    (local.get $path)))

;; Option wrap by the inner schema's repr: Int payloads already are the
;; $zhex_some_i64 Option repr, Bool re-boxes the i31, refs wrap some_ref
(func $zxml_wrap_some (param $v (ref null eq)) (param $inner (ref $zjson_schema)) (result (ref null eq))
  (if (i32.eq (struct.get $zjson_schema $kind (local.get $inner)) (i32.const 0))
    (then (return (local.get $v))))
  (if (i32.eq (struct.get $zjson_schema $kind (local.get $inner)) (i32.const 1))
    (then (return (struct.new $zhex_some_i32 (i31.get_u (ref.cast (ref i31) (local.get $v)))))))
  (struct.new $zhex_some_ref (local.get $v)))

;; decodeXMLText: attribute or element text against a scalar or Option
;; schema (structure-bearing schemas fail with "expected <Type>")
(func $zxml_decode_text (param $t (ref $zhex_string)) (param $s (ref $zjson_schema)) (param $def (ref null eq)) (param $m (ref $zjson_memo)) (param $path (ref $zhex_string)) (result (ref $zhex_result))
  (local $kind i32)
  (local $r (ref $zhex_result))
  (local $code i32)
  (local.set $kind (struct.get $zjson_schema $kind (local.get $s)))
  (if (i32.eq (local.get $kind) (i32.const 3))
    (then
      (local.set $r (call $zxml_decode_text (local.get $t)
        (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))
        (if (result (ref null eq))
          (ref.test (ref $zhex_some_ref) (local.get $def))
          (then (struct.get $zhex_some_ref $value (ref.cast (ref $zhex_some_ref) (local.get $def))))
          (else (ref.null eq)))
        (local.get $m) (local.get $path)))
      (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
        (then (return (local.get $r))))
      (return (struct.new $zhex_result (i32.const 0)
        (call $zxml_wrap_some (struct.get $zhex_result $payload (local.get $r))
          (ref.as_non_null (struct.get $zjson_schema $element (local.get $s))))))))
  (if (i32.eq (local.get $kind) (i32.const 0))
    (then
      (local.set $code (call $zxml_exact_int (local.get $t)))
      (if (i32.eq (local.get $code) (i32.const 0))
        (then (return (struct.new $zhex_result (i32.const 0) (struct.new $zhex_some_i64 (global.get $zxml_int_val))))))
      (if (i32.eq (local.get $code) (i32.const 1))
        (then (return (call $zjson_fail (call $mj_exp_intform) (local.get $path)))))
      (return (call $zjson_fail (call $mj_exp_int) (local.get $path)))))
  (if (i32.eq (local.get $kind) (i32.const 1))
    (then
      (local.set $t (call $zhex_text_trim (local.get $t)))
      (if (call $zhex_str_eq (local.get $t) (call $mx_true))
        (then (return (struct.new $zhex_result (i32.const 0) (ref.i31 (i32.const 1))))))
      (if (call $zhex_str_eq (local.get $t) (call $mx_false))
        (then (return (struct.new $zhex_result (i32.const 0) (ref.i31 (i32.const 0))))))
      (return (call $zjson_fail (call $mj_exp_bool) (local.get $path)))))
  (if (i32.eq (local.get $kind) (i32.const 2))
    (then (return (struct.new $zhex_result (i32.const 0) (local.get $t)))))
  (call $zxml_expected_fail (local.get $s) (local.get $path)))

;; decodeXMLValue: the element-tree walk. Records map attributes and child
;; elements onto fields verbatim by name (unknown/collision/duplicate rules
;; pinned from the interpreter, in its scan order), lists and maps decode
;; per element repr, missing fields clone the defaults through the identity
;; memo (aliasing preserved).
(func $zxml_decode_value (param $n (ref $zxml_elem)) (param $s (ref $zjson_schema)) (param $def (ref null eq)) (param $m (ref $zjson_memo)) (param $path (ref $zhex_string)) (result (ref $zhex_result))
  (local $kind i32)
  (local $elkind i32)
  (local $i i32)
  (local $i2 i32)
  (local $j i32)
  (local $k i32)
  (local $found i32)
  (local $ac i32)
  (local $cc i32)
  (local $fc i32)
  (local $attrs (ref null $zhex_arr_ref))
  (local $children (ref null $zhex_arr_ref))
  (local $fields (ref null $zhex_arr_ref))
  (local $deff (ref null $zhex_arr_ref))
  (local $fresh (ref null $zhex_arr_ref))
  (local $items (ref null $zhex_arr_ref))
  (local $i64s (ref null $zhex_arr_i64))
  (local $i32s (ref null $zhex_arr_i32))
  (local $mi64 (ref null $zhex_map_str_i64))
  (local $mi32 (ref null $zhex_map_str_i32))
  (local $map (ref $zhex_map_str_ref))
  (local $r (ref $zhex_result))
  (local $child (ref $zxml_elem))
  (local $fs (ref $zjson_schema))
  (local $fname (ref $zhex_string))
  (local $cname (ref $zhex_string))
  (local $p (ref $zhex_string))
  (local $v (ref null eq))
  (local.set $kind (struct.get $zjson_schema $kind (local.get $s)))
  (local.set $attrs (struct.get $zxml_elem $attrs (local.get $n)))
  (local.set $children (struct.get $zxml_elem $children (local.get $n)))
  (local.set $ac (i32.const 0))
  (if (i32.eqz (ref.is_null (local.get $attrs)))
    (then (local.set $ac (array.len (ref.as_non_null (local.get $attrs))))))
  (local.set $cc (i32.const 0))
  (if (i32.eqz (ref.is_null (local.get $children)))
    (then (local.set $cc (array.len (ref.as_non_null (local.get $children))))))
  ;; Option: an element or attribute present wraps Some of the inner decode
  (if (i32.eq (local.get $kind) (i32.const 3))
    (then
      (local.set $r (call $zxml_decode_value (local.get $n)
        (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))
        (if (result (ref null eq))
          (ref.test (ref $zhex_some_ref) (local.get $def))
          (then (struct.get $zhex_some_ref $value (ref.cast (ref $zhex_some_ref) (local.get $def))))
          (else (ref.null eq)))
        (local.get $m) (local.get $path)))
      (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
        (then (return (local.get $r))))
      (return (struct.new $zhex_result (i32.const 0)
        (call $zxml_wrap_some (struct.get $zhex_result $payload (local.get $r))
          (ref.as_non_null (struct.get $zjson_schema $element (local.get $s))))))))
  ;; scalars: attributes, then children, are shape errors; else the text
  (if (i32.lt_u (local.get $kind) (i32.const 3))
    (then
      (if (i32.gt_u (local.get $ac) (i32.const 0))
        (then (return (call $zxml_expected_fail (local.get $s) (local.get $path)))))
      (if (i32.gt_u (local.get $cc) (i32.const 0))
        (then (return (call $zxml_expected_fail (local.get $s) (local.get $path)))))
      (return (call $zxml_decode_text (call $zxml_text (local.get $n)) (local.get $s) (ref.null eq) (local.get $m) (local.get $path)))))
  ;; list and map share the preamble
  (if (i32.or (i32.eq (local.get $kind) (i32.const 4)) (i32.eq (local.get $kind) (i32.const 5)))
    (then
      (if (i32.gt_u (local.get $ac) (i32.const 0))
        (then (return (call $zxml_expected_fail (local.get $s) (local.get $path)))))
      (if (i32.gt_u (array.len (call $zhex_text_trim (call $zxml_text (local.get $n)))) (i32.const 0))
        (then (return (call $zjson_fail (call $mx_mixed) (local.get $path)))))
      (if (i32.gt_u (local.get $cc) (i32.const 1000000))
        (then (return (call $zjson_fail (call $mj_list_limit) (local.get $path)))))))
  (if (i32.eq (local.get $kind) (i32.const 4))
    (then
      (local.set $elkind (struct.get $zjson_schema $kind (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))))
      (if (i32.eq (local.get $elkind) (i32.const 0))
        (then
          (local.set $i64s (array.new_default $zhex_arr_i64 (local.get $cc)))
          (block $dec (loop $dl
            (br_if $dec (i32.ge_u (local.get $i) (local.get $cc)))
            (local.set $r (call $zxml_decode_value (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $i))
              (ref.as_non_null (struct.get $zjson_schema $element (local.get $s))) (ref.null eq) (local.get $m)
              (call $zxml_path_index (local.get $path) (local.get $i))))
            (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
              (then (return (local.get $r))))
            (array.set $zhex_arr_i64 (ref.as_non_null (local.get $i64s)) (local.get $i)
              (struct.get $zhex_some_i64 $value (ref.cast (ref $zhex_some_i64) (struct.get $zhex_result $payload (local.get $r)))))
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (br $dl)))
          (return (struct.new $zhex_result (i32.const 0)
            (struct.new $zhex_list_i64 (ref.as_non_null (local.get $i64s)) (i32.const 0))))))
      (if (i32.eq (local.get $elkind) (i32.const 1))
        (then
          (local.set $i32s (array.new_default $zhex_arr_i32 (local.get $cc)))
          (block $dec (loop $dl
            (br_if $dec (i32.ge_u (local.get $i) (local.get $cc)))
            (local.set $r (call $zxml_decode_value (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $i))
              (ref.as_non_null (struct.get $zjson_schema $element (local.get $s))) (ref.null eq) (local.get $m)
              (call $zxml_path_index (local.get $path) (local.get $i))))
            (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
              (then (return (local.get $r))))
            (array.set $zhex_arr_i32 (ref.as_non_null (local.get $i32s)) (local.get $i)
              (i31.get_u (ref.cast (ref i31) (struct.get $zhex_result $payload (local.get $r)))))
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (br $dl)))
          (return (struct.new $zhex_result (i32.const 0)
            (struct.new $zhex_list_i32 (ref.as_non_null (local.get $i32s)) (i32.const 0))))))
      (local.set $items (array.new_default $zhex_arr_ref (local.get $cc)))
      (block $dec (loop $dl
        (br_if $dec (i32.ge_u (local.get $i) (local.get $cc)))
        (local.set $r (call $zxml_decode_value (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $i))
          (ref.as_non_null (struct.get $zjson_schema $element (local.get $s))) (ref.null eq) (local.get $m)
          (call $zxml_path_index (local.get $path) (local.get $i))))
        (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
          (then (return (local.get $r))))
        (array.set $zhex_arr_ref (ref.as_non_null (local.get $items)) (local.get $i)
          (struct.get $zhex_result $payload (local.get $r)))
        (local.set $i (i32.add (local.get $i) (i32.const 1)))
        (br $dl)))
      (return (struct.new $zhex_result (i32.const 0)
        (struct.new $zhex_list_ref (ref.as_non_null (local.get $items)) (i32.const 0))))))
  (if (i32.eq (local.get $kind) (i32.const 5))
    (then
      ;; duplicate tag names are duplicate map keys (first offender reported)
      (local.set $j (i32.const 0))
      (block $kd (loop $kl
        (br_if $kd (i32.ge_u (local.get $j) (local.get $cc)))
        (local.set $k (i32.const 0))
        (block $sd (loop $sl
          (br_if $sd (i32.ge_u (local.get $k) (local.get $j)))
          (if (call $zhex_str_eq
                (struct.get $zxml_elem $name (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $j)))
                (struct.get $zxml_elem $name (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $k))))
            (then (return (call $zjson_fail (call $mx_dup_key)
              (call $zxml_field_path (local.get $path)
                (struct.get $zxml_elem $name (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $j))))))))
          (local.set $k (i32.add (local.get $k) (i32.const 1)))
          (br $sl)))
        (local.set $j (i32.add (local.get $j) (i32.const 1)))
        (br $kl)))
      (local.set $elkind (struct.get $zjson_schema $kind (ref.as_non_null (struct.get $zjson_schema $element (local.get $s)))))
      (if (i32.eq (local.get $elkind) (i32.const 0))
        (then
          (local.set $mi64 (call $zhex_map_new_str_i64))
          (block $dec (loop $dl
            (br_if $dec (i32.ge_u (local.get $i) (local.get $cc)))
            (local.set $child (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $i)))
            (local.set $r (call $zxml_decode_value (local.get $child)
              (ref.as_non_null (struct.get $zjson_schema $element (local.get $s))) (ref.null eq) (local.get $m)
              (call $zxml_field_path (local.get $path) (struct.get $zxml_elem $name (local.get $child)))))
            (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
              (then (return (local.get $r))))
            (drop (call $zhex_map_put_str_i64 (ref.as_non_null (local.get $mi64)) (struct.get $zxml_elem $name (local.get $child))
              (struct.get $zhex_some_i64 $value (ref.cast (ref $zhex_some_i64) (struct.get $zhex_result $payload (local.get $r))))))
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (br $dl)))
          (return (struct.new $zhex_result (i32.const 0) (ref.as_non_null (local.get $mi64))))))
      (if (i32.eq (local.get $elkind) (i32.const 1))
        (then
          (local.set $mi32 (call $zhex_map_new_str_i32))
          (block $dec (loop $dl
            (br_if $dec (i32.ge_u (local.get $i) (local.get $cc)))
            (local.set $child (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $i)))
            (local.set $r (call $zxml_decode_value (local.get $child)
              (ref.as_non_null (struct.get $zjson_schema $element (local.get $s))) (ref.null eq) (local.get $m)
              (call $zxml_field_path (local.get $path) (struct.get $zxml_elem $name (local.get $child)))))
            (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
              (then (return (local.get $r))))
            (drop (call $zhex_map_put_str_i32 (ref.as_non_null (local.get $mi32)) (struct.get $zxml_elem $name (local.get $child))
              (i31.get_u (ref.cast (ref i31) (struct.get $zhex_result $payload (local.get $r))))))
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (br $dl)))
          (return (struct.new $zhex_result (i32.const 0) (ref.as_non_null (local.get $mi32))))))
      (local.set $map (call $zhex_map_new_str_ref))
      (block $dec (loop $dl
        (br_if $dec (i32.ge_u (local.get $i) (local.get $cc)))
        (local.set $child (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $i)))
        (local.set $r (call $zxml_decode_value (local.get $child)
          (ref.as_non_null (struct.get $zjson_schema $element (local.get $s))) (ref.null eq) (local.get $m)
          (call $zxml_field_path (local.get $path) (struct.get $zxml_elem $name (local.get $child)))))
        (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
          (then (return (local.get $r))))
        (drop (call $zhex_map_put_str_ref (local.get $map) (struct.get $zxml_elem $name (local.get $child))
          (struct.get $zhex_result $payload (local.get $r))))
        (local.set $i (i32.add (local.get $i) (i32.const 1)))
        (br $dl)))
      (return (struct.new $zhex_result (i32.const 0) (local.get $map)))))
  ;; ---- record ----
  (local.set $fields (ref.as_non_null (struct.get $zjson_schema $fields (local.get $s))))
  (local.set $fc (array.len (ref.as_non_null (local.get $fields))))
  ;; unknown attributes
  (local.set $i (i32.const 0))
  (block $ad (loop $al
    (br_if $ad (i32.ge_u (local.get $i) (local.get $ac)))
    (local.set $cname (call $zxml_attr_name (array.get $zhex_arr_ref (ref.as_non_null (local.get $attrs)) (local.get $i))))
    (local.set $found (i32.const 0))
    (local.set $j (i32.const 0))
    (block $fd (loop $fl
      (br_if $fd (i32.ge_u (local.get $j) (local.get $fc)))
      (if (call $zhex_str_eq (struct.get $zjson_field $name (ref.cast (ref $zjson_field) (array.get $zhex_arr_ref (ref.as_non_null (local.get $fields)) (local.get $j)))) (local.get $cname))
        (then (local.set $found (i32.const 1))))
      (local.set $j (i32.add (local.get $j) (i32.const 1)))
      (br $fl)))
    (if (i32.eqz (local.get $found))
      (then (return (call $zjson_fail
        (call $zhex_str_concat (call $mx_unknown_attr) (call $zhex_quote (local.get $cname)))
        (call $zxml_field_path (local.get $path) (local.get $cname))))))
    (local.set $i (i32.add (local.get $i) (i32.const 1)))
    (br $al)))
  ;; unknown / colliding / duplicate child elements, in scan order
  (local.set $i (i32.const 0))
  (block $cd (loop $cl
    (br_if $cd (i32.ge_u (local.get $i) (local.get $cc)))
    (local.set $child (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $i)))
    (local.set $cname (struct.get $zxml_elem $name (local.get $child)))
    (local.set $found (i32.const -1))
    (local.set $j (i32.const 0))
    (block $fd (loop $fl
      (br_if $fd (i32.ge_u (local.get $j) (local.get $fc)))
      (if (call $zhex_str_eq (struct.get $zjson_field $name (ref.cast (ref $zjson_field) (array.get $zhex_arr_ref (ref.as_non_null (local.get $fields)) (local.get $j)))) (local.get $cname))
        (then (local.set $found (local.get $j))))
      (local.set $j (i32.add (local.get $j) (i32.const 1)))
      (br $fl)))
    (if (i32.lt_s (local.get $found) (i32.const 0))
      (then (return (call $zjson_fail
        (call $zhex_str_concat (call $mx_unknown_elem) (call $zhex_quote (local.get $cname)))
        (call $zxml_field_path (local.get $path) (local.get $cname))))))
    ;; collision: the name is also an attribute of this element
    (local.set $j (i32.const 0))
    (block $xa (loop $xl
      (br_if $xa (i32.ge_u (local.get $j) (local.get $ac)))
      (if (call $zhex_str_eq (call $zxml_attr_name (array.get $zhex_arr_ref (ref.as_non_null (local.get $attrs)) (local.get $j))) (local.get $cname))
        (then (return (call $zjson_fail
          (call $zhex_cat3 (call $mx_elem_word) (call $zhex_quote (local.get $cname)) (call $mx_coll_elem))
          (call $zxml_field_path (local.get $path) (local.get $cname))))))
      (local.set $j (i32.add (local.get $j) (i32.const 1)))
      (br $xl)))
    ;; duplicate: a non-list field seen before
    (if (i32.ne (struct.get $zjson_schema $kind
          (ref.as_non_null (struct.get $zjson_field $schema (ref.cast (ref $zjson_field) (array.get $zhex_arr_ref (ref.as_non_null (local.get $fields)) (local.get $found)))))) (i32.const 4))
      (then
        (local.set $j (i32.const 0))
        (block $dc (loop $dl
          (br_if $dc (i32.ge_u (local.get $j) (local.get $i)))
          (if (call $zhex_str_eq (struct.get $zxml_elem $name (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $j))) (local.get $cname))
            (then (return (call $zjson_fail
              (call $zhex_str_concat (call $mx_dup_elem) (call $zhex_quote (local.get $cname)))
              (call $zxml_field_path (local.get $path) (local.get $cname))))))
          (local.set $j (i32.add (local.get $j) (i32.const 1)))
          (br $dl)))))
    (local.set $i (i32.add (local.get $i) (i32.const 1)))
    (br $cl)))
  ;; non-whitespace text among children
  (if (i32.gt_u (array.len (call $zhex_text_trim (call $zxml_text (local.get $n)))) (i32.const 0))
    (then (return (call $zjson_fail (call $mx_mixed) (local.get $path)))))
  ;; the defaults' boxed fields, when a defaults value was supplied
  (if (i32.eqz (ref.is_null (local.get $def)))
    (then
      (local.set $deff (call_ref $zjson_get_fn (local.get $def)
        (ref.cast (ref $zjson_get_fn) (struct.get $zjson_schema $get (local.get $s)))))))
  ;; fill fields in declaration order
  (local.set $fresh (array.new_default $zhex_arr_ref (local.get $fc)))
  (local.set $i (i32.const 0))
  (block $fld (loop $fl2
    (br_if $fld (i32.ge_u (local.get $i) (local.get $fc)))
    (local.set $fs (ref.as_non_null (struct.get $zjson_field $schema (ref.cast (ref $zjson_field) (array.get $zhex_arr_ref (ref.as_non_null (local.get $fields)) (local.get $i))))))
    (local.set $fname (struct.get $zjson_field $name (ref.cast (ref $zjson_field) (array.get $zhex_arr_ref (ref.as_non_null (local.get $fields)) (local.get $i)))))
    (local.set $v (ref.null eq))
    (if (i32.eqz (ref.is_null (local.get $deff)))
      (then (local.set $v (array.get $zhex_arr_ref (ref.as_non_null (local.get $deff)) (local.get $i)))))
    (local.set $p (call $zxml_field_path (local.get $path) (local.get $fname)))
    ;; an attribute supplies text; else children; else the cloned default
    (local.set $found (i32.const -1))
    (local.set $j (i32.const 0))
    (block $fa (loop $fal
      (br_if $fa (i32.ge_u (local.get $j) (local.get $ac)))
      (if (call $zhex_str_eq (call $zxml_attr_name (array.get $zhex_arr_ref (ref.as_non_null (local.get $attrs)) (local.get $j))) (local.get $fname))
        (then (local.set $found (local.get $j))))
      (local.set $j (i32.add (local.get $j) (i32.const 1)))
      (br $fal)))
    (if (i32.ge_s (local.get $found) (i32.const 0))
      (then
        (local.set $r (call $zxml_decode_text
          (call $zxml_attr_value (array.get $zhex_arr_ref (ref.as_non_null (local.get $attrs)) (local.get $found)))
          (local.get $fs) (local.get $v) (local.get $m) (local.get $p)))
        (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
          (then (return (local.get $r))))
        (array.set $zhex_arr_ref (ref.as_non_null (local.get $fresh)) (local.get $i) (struct.get $zhex_result $payload (local.get $r))))
      (else
        ;; count matching child elements
        (local.set $k (i32.const 0))
        (local.set $j (i32.const 0))
        (block $cc2 (loop $cl2
          (br_if $cc2 (i32.ge_u (local.get $j) (local.get $cc)))
          (if (call $zhex_str_eq (struct.get $zxml_elem $name (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $j))) (local.get $fname))
            (then (local.set $k (i32.add (local.get $k) (i32.const 1)))))
          (local.set $j (i32.add (local.get $j) (i32.const 1)))
          (br $cl2)))
        (if (i32.gt_u (local.get $k) (i32.const 0))
          (then
            (if (i32.eq (struct.get $zjson_schema $kind (local.get $fs)) (i32.const 4))
              (then
                ;; list field: every matching child decodes at <path>/<j>
                (local.set $i2 (i32.const 0))
                (local.set $elkind (struct.get $zjson_schema $kind (ref.as_non_null (struct.get $zjson_schema $element (local.get $fs)))))
                (if (i32.eq (local.get $elkind) (i32.const 0))
                  (then
                    (local.set $i64s (array.new_default $zhex_arr_i64 (local.get $k)))
                    (local.set $j (i32.const 0))
                    (block $li (loop $lil
                      (br_if $li (i32.ge_u (local.get $j) (local.get $cc)))
                      (if (call $zhex_str_eq (struct.get $zxml_elem $name (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $j))) (local.get $fname))
                        (then
                          (local.set $r (call $zxml_decode_value (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $j))
                            (ref.as_non_null (struct.get $zjson_schema $element (local.get $fs))) (ref.null eq) (local.get $m)
                            (call $zxml_path_index (local.get $p) (local.get $i2))))
                          (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
                            (then (return (local.get $r))))
                          (array.set $zhex_arr_i64 (ref.as_non_null (local.get $i64s)) (local.get $i2)
                            (struct.get $zhex_some_i64 $value (ref.cast (ref $zhex_some_i64) (struct.get $zhex_result $payload (local.get $r)))))
                          (local.set $i2 (i32.add (local.get $i2) (i32.const 1)))))
                      (local.set $j (i32.add (local.get $j) (i32.const 1)))
                      (br $lil)))
                    (array.set $zhex_arr_ref (ref.as_non_null (local.get $fresh)) (local.get $i)
                      (struct.new $zhex_list_i64 (ref.as_non_null (local.get $i64s)) (i32.const 0)))))
                (if (i32.eq (local.get $elkind) (i32.const 1))
                  (then
                    (local.set $i32s (array.new_default $zhex_arr_i32 (local.get $k)))
                    (local.set $j (i32.const 0))
                    (block $li (loop $lil
                      (br_if $li (i32.ge_u (local.get $j) (local.get $cc)))
                      (if (call $zhex_str_eq (struct.get $zxml_elem $name (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $j))) (local.get $fname))
                        (then
                          (local.set $r (call $zxml_decode_value (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $j))
                            (ref.as_non_null (struct.get $zjson_schema $element (local.get $fs))) (ref.null eq) (local.get $m)
                            (call $zxml_path_index (local.get $p) (local.get $i2))))
                          (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
                            (then (return (local.get $r))))
                          (array.set $zhex_arr_i32 (ref.as_non_null (local.get $i32s)) (local.get $i2)
                            (i31.get_u (ref.cast (ref i31) (struct.get $zhex_result $payload (local.get $r)))))
                          (local.set $i2 (i32.add (local.get $i2) (i32.const 1)))))
                      (local.set $j (i32.add (local.get $j) (i32.const 1)))
                      (br $lil)))
                    (array.set $zhex_arr_ref (ref.as_non_null (local.get $fresh)) (local.get $i)
                      (struct.new $zhex_list_i32 (ref.as_non_null (local.get $i32s)) (i32.const 0)))))
                (if (i32.and (i32.ne (local.get $elkind) (i32.const 0)) (i32.ne (local.get $elkind) (i32.const 1)))
                  (then
                    (local.set $items (array.new_default $zhex_arr_ref (local.get $k)))
                    (local.set $j (i32.const 0))
                    (block $li (loop $lil
                      (br_if $li (i32.ge_u (local.get $j) (local.get $cc)))
                      (if (call $zhex_str_eq (struct.get $zxml_elem $name (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $j))) (local.get $fname))
                        (then
                          (local.set $r (call $zxml_decode_value (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $j))
                            (ref.as_non_null (struct.get $zjson_schema $element (local.get $fs))) (ref.null eq) (local.get $m)
                            (call $zxml_path_index (local.get $p) (local.get $i2))))
                          (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
                            (then (return (local.get $r))))
                          (array.set $zhex_arr_ref (ref.as_non_null (local.get $items)) (local.get $i2)
                            (struct.get $zhex_result $payload (local.get $r)))
                          (local.set $i2 (i32.add (local.get $i2) (i32.const 1)))))
                      (local.set $j (i32.add (local.get $j) (i32.const 1)))
                      (br $lil)))
                    (array.set $zhex_arr_ref (ref.as_non_null (local.get $fresh)) (local.get $i)
                      (struct.new $zhex_list_ref (ref.as_non_null (local.get $items)) (i32.const 0))))))
              (else
                ;; non-list field: the first matching child element
                (local.set $found (i32.const -1))
                (local.set $j (i32.const 0))
                (block $fc2 (loop $fcl
                  (br_if $fc2 (i32.ge_u (local.get $j) (local.get $cc)))
                  (if (i32.and (i32.lt_s (local.get $found) (i32.const 0))
                               (call $zhex_str_eq (struct.get $zxml_elem $name (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $j))) (local.get $fname)))
                    (then (local.set $found (local.get $j))))
                  (local.set $j (i32.add (local.get $j) (i32.const 1)))
                  (br $fcl)))
                (local.set $r (call $zxml_decode_value (call $zxml_child (ref.as_non_null (local.get $children)) (local.get $found))
                  (local.get $fs) (local.get $v) (local.get $m) (local.get $p)))
                (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
                  (then (return (local.get $r))))
                (array.set $zhex_arr_ref (ref.as_non_null (local.get $fresh)) (local.get $i) (struct.get $zhex_result $payload (local.get $r))))))
          (else
            ;; neither: the cloned default, or the missing-field failure
            (if (ref.is_null (local.get $v))
              (then (return (call $zjson_fail
                (call $zhex_str_concat (call $mx_missing) (call $zhex_quote (local.get $fname)))
                (call $zxml_field_path (local.get $path) (local.get $fname))))))
            (array.set $zhex_arr_ref (ref.as_non_null (local.get $fresh)) (local.get $i)
              (call $zxml_clone (local.get $v) (local.get $fs) (local.get $m)))))))
    (local.set $i (i32.add (local.get $i) (i32.const 1)))
    (br $fl2)))
  (struct.new $zhex_result (i32.const 0)
    (call_ref $zjson_make_fn (ref.as_non_null (local.get $fresh))
      (ref.cast (ref $zjson_make_fn) (struct.get $zjson_schema $make (local.get $s))))))

;; schema-directed deep clone for absent-field fallbacks. The interpreter's
;; jsonClone is value-shaped and covers every list/map spec; the M3
;; $zjson_clone only clones the ref flavors, so an absent List<Int> or
;; Map<String, Int> field would alias the defaults' wrapper (append mutates
;; the shared $items). This clone covers the Int and Bool flavors directly
;; and delegates the ref flavors (and records) to $zjson_clone; Options
;; clone through their payload.
(func $zxml_clone (param $v (ref null eq)) (param $s (ref $zjson_schema)) (param $m (ref $zjson_memo)) (result (ref null eq))
  (local $kind i32)
  (local $elkind i32)
  (local $n i32)
  (local $i i32)
  (local $r (ref null eq))
  (local $f64 (ref null $zhex_arr_i64))
  (local $f32 (ref null $zhex_arr_i32))
  (local $m64 (ref null $zhex_map_str_i64))
  (local $m32 (ref null $zhex_map_str_i32))
  (local.set $kind (struct.get $zjson_schema $kind (local.get $s)))
  (if (i32.eq (local.get $kind) (i32.const 3))
    (then
      ;; Some of a ref payload clones through the element schema; Int/Bool
      ;; payloads and None share nothing mutable
      (if (ref.test (ref $zhex_some_ref) (local.get $v))
        (then
          (return (struct.new $zhex_some_ref
            (call $zxml_clone (struct.get $zhex_some_ref $value (ref.cast (ref $zhex_some_ref) (local.get $v)))
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
          (local.set $n (array.len (ref.as_non_null (struct.get $zhex_list_i64 $items (ref.cast (ref $zhex_list_i64) (local.get $v))))))
          (local.set $f64 (array.new_default $zhex_arr_i64 (local.get $n)))
          (block $cp (loop $cl
            (br_if $cp (i32.ge_u (local.get $i) (local.get $n)))
            (array.set $zhex_arr_i64 (ref.as_non_null (local.get $f64)) (local.get $i)
              (array.get $zhex_arr_i64 (ref.as_non_null (struct.get $zhex_list_i64 $items (ref.cast (ref $zhex_list_i64) (local.get $v)))) (local.get $i)))
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (br $cl)))
          (local.set $r (struct.new $zhex_list_i64 (ref.as_non_null (local.get $f64)) (i32.const 0)))
          (call $zjson_memo_push (local.get $m) (local.get $v) (local.get $r))
          (return (local.get $r))))
      (if (i32.eq (local.get $elkind) (i32.const 1))
        (then
          (if (i32.eqz (ref.test (ref $zhex_list_i32) (local.get $v))) (then (return (local.get $v))))
          (local.set $r (call $zjson_memo_find (local.get $m) (local.get $v)))
          (if (i32.eqz (ref.is_null (local.get $r))) (then (return (local.get $r))))
          (local.set $n (array.len (ref.as_non_null (struct.get $zhex_list_i32 $items (ref.cast (ref $zhex_list_i32) (local.get $v))))))
          (local.set $f32 (array.new_default $zhex_arr_i32 (local.get $n)))
          (block $cp (loop $cl
            (br_if $cp (i32.ge_u (local.get $i) (local.get $n)))
            (array.set $zhex_arr_i32 (ref.as_non_null (local.get $f32)) (local.get $i)
              (array.get $zhex_arr_i32 (ref.as_non_null (struct.get $zhex_list_i32 $items (ref.cast (ref $zhex_list_i32) (local.get $v)))) (local.get $i)))
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (br $cl)))
          (local.set $r (struct.new $zhex_list_i32 (ref.as_non_null (local.get $f32)) (i32.const 0)))
          (call $zjson_memo_push (local.get $m) (local.get $v) (local.get $r))
          (return (local.get $r))))
      (return (call $zjson_clone (local.get $v) (local.get $s) (local.get $m)))))
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
          (local.set $n (array.len (struct.get $zhex_map_str_i64 $keys (ref.cast (ref $zhex_map_str_i64) (local.get $v)))))
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
          (local.set $n (array.len (struct.get $zhex_map_str_i32 $keys (ref.cast (ref $zhex_map_str_i32) (local.get $v)))))
          (block $cp (loop $cl
            (br_if $cp (i32.ge_u (local.get $i) (local.get $n)))
            (drop (call $zhex_map_put_str_i32 (ref.as_non_null (local.get $m32))
              (array.get $zhex_arr_str (struct.get $zhex_map_str_i32 $keys (ref.cast (ref $zhex_map_str_i32) (local.get $v))) (local.get $i))
              (array.get $zhex_arr_i32 (struct.get $zhex_map_str_i32 $vals (ref.cast (ref $zhex_map_str_i32) (local.get $v))) (local.get $i))))
            (local.set $i (i32.add (local.get $i) (i32.const 1)))
            (br $cl)))
          (return (local.get $r))))
      (return (call $zjson_clone (local.get $v) (local.get $s) (local.get $m)))))
  (call $zjson_clone (local.get $v) (local.get $s) (local.get $m)))

;; xml.decode: surrogate scan (module strings hold code points; an unpaired
;; surrogate cannot UTF-8-encode), parse, then the schema walk
(func $zhex_xml_decode (param $raw (ref $zhex_string)) (param $defaults (ref null eq)) (param $s (ref $zjson_schema)) (result (ref $zhex_result))
  (local $r (ref $zhex_result))
  (if (i32.eqz (call $zxml_utf8_ok (local.get $raw)))
    (then (return (call $zjson_fail (call $mx_not_utf8) (array.new_fixed $zhex_string 0)))))
  (local.set $r (call $zxml_parse (local.get $raw)))
  (if (i32.ne (struct.get $zhex_result $tag (local.get $r)) (i32.const 0))
    (then (return (local.get $r))))
  (call $zxml_decode_value (ref.cast (ref $zxml_elem) (struct.get $zhex_result $payload (local.get $r)))
    (local.get $s) (local.get $defaults)
    (struct.new $zjson_memo (ref.null $zhex_arr_ref) (ref.null $zhex_arr_ref))
    (array.new_fixed $zhex_string 0)))
`
