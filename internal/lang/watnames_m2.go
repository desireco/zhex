package lang

// Milestone-2 names shared between the WAT emitter (wat.go) and the M2
// runtime prelude (watprelude_m2.go), pinned in internal/wasm/DESIGN-M2.md
// §2 (err value, fail codes 12-15, staged messages), §3 (maps), §4
// (closures and callback helpers), and §1 (text). The prelude defines these
// symbols in WAT; the emitter references them by these exact spellings.

// Fail codes appended to the M1 §6a table (DESIGN-M2 §2c). Codes 12 and 15
// carry a staged dynamic message (see watFailMsg); 13 and 14 are fixed
// strings the runner owns. Never renumber.
const (
	watFailTextToInt    = 12 // text-to-int, message staged via watFailMsg
	watFailReplaceEmpty = 13 // replace-empty, fixed
	watFailSplitLimit   = 14 // collection-limit (text.split), fixed
	watFailArraySelect  = 15 // array-rank (array.select), message staged
)

// The err value (DESIGN-M2 §2b): $zhex_err{family, message}. Family 1 is
// ParseError — the only family M2 mints (constants grow later milestones).
const (
	watTypeErr         = "$zhex_err"
	watErrFamilyParse  = 1
	watErrFieldFamily  = "$family"
	watErrFieldMessage = "$message"
)

// Staged fail messages (DESIGN-M2 §2c): $zhex_fail_msg(code, msg) encodes
// the message as UTF-8 at memory offset 0, records its byte length for the
// exported zhex_msg_len() accessor (0 = none), sets $zhex_err, and traps.
const (
	watFailMsg      = "$zhex_fail_msg"
	watMsgLenGet    = "$zhex_get_msg_len"
	watMsgLenExport = "zhex_msg_len"
)

// text.* prelude helpers (DESIGN-M2 §1); every word is a prelude call except
// text.length, which the emitter inlines (array.len + i64.extend_i32_u).
const (
	watTextReverse    = "$zhex_text_reverse"
	watTextSlice      = "$zhex_text_slice"
	watTextIndexOf    = "$zhex_text_index_of"
	watTextContains   = "$zhex_text_contains"
	watTextUpper      = "$zhex_text_upper"
	watTextLower      = "$zhex_text_lower"
	watTextTrim       = "$zhex_text_trim"
	watTextSplit      = "$zhex_text_split"
	watTextJoin       = "$zhex_text_join"
	watTextReplace    = "$zhex_text_replace"
	watTextStartsWith = "$zhex_text_starts_with"
	watTextEndsWith   = "$zhex_text_ends_with"
	watTextLineCount  = "$zhex_text_line_count"
	watTextWordCount  = "$zhex_text_word_count"
	watTextToInt      = "$zhex_text_to_int"
)

// Code-point string ordering (DESIGN-M2 §9), used by list.sort/sort-by on
// String elements/keys; element-wise i32.lt_u equals Go's UTF-8 byte order.
const watStrLt = "$zhex_str_lt"

// watMapKey is a map key specialization: i64 (Int), i32 (Bool), str (String).
type watMapKey string

const (
	watMapKeyI64 watMapKey = "i64"
	watMapKeyI32 watMapKey = "i32"
	watMapKeyStr watMapKey = "str"
)

// watTypeArrStr is the String-key array of a map's $keys field.
const watTypeArrStr = "$zhex_arr_str"

// Map helpers (DESIGN-M2 §3): one type and helper family per key/value repr
// pair. Lookup is the m[k] index read returning an Option; put/remove/into
// guard before the operation (fail 10); contains/length never guard.
func watMapType(k watMapKey, v watSpec) string { return "$zhex_map_" + string(k) + "_" + string(v) }
func watMapNew(k watMapKey, v watSpec) string  { return "$zhex_map_new_" + string(k) + "_" + string(v) }
func watMapPut(k watMapKey, v watSpec) string  { return "$zhex_map_put_" + string(k) + "_" + string(v) }
func watMapLookup(k watMapKey, v watSpec) string {
	return "$zhex_map_lookup_" + string(k) + "_" + string(v)
}
func watMapRemove(k watMapKey, v watSpec) string {
	return "$zhex_map_remove_" + string(k) + "_" + string(v)
}
func watMapContains(k watMapKey, v watSpec) string {
	return "$zhex_map_contains_" + string(k) + "_" + string(v)
}
func watMapLength(k watMapKey, v watSpec) string {
	return "$zhex_map_length_" + string(k) + "_" + string(v)
}
func watMapInto(k watMapKey, v watSpec) string {
	return "$zhex_map_into_" + string(k) + "_" + string(v)
}

// Closures (DESIGN-M2 §4): one uniform struct {fn, env} plus typed funcrefs
// per callback signature. Params and result are element reprs (watSpec), so
// e.g. (String)->Int and (List<Int>)->Int share $zhex_clo_ref_r_i64.
const watTypeClosure = "$zhex_closure"

// watCloType names a callback signature type: $zhex_clo_<p1>_<p2>_r_<r> for
// arity 2, $zhex_clo_<p>_r_<r> for arity 1 (the only arities M2 needs).
func watCloType(params []watSpec, result watSpec) string {
	name := "$zhex_clo"
	for _, p := range params {
		name += "_" + string(p)
	}
	return name + "_r_" + string(result)
}

// Callback word helpers (DESIGN-M2 §4d). map takes (in)->out; filter and
// select share one body under two names; fold's step is (acc, item)->acc;
// find returns the Option Some/None; sort_by's key is Int (i64) or String
// (ref); array.select takes a rank-2 array and a (List<T>)->Bool predicate.
func watListMap(in, out watSpec) string {
	return "$zhex_list_map_" + string(in) + "_" + string(out)
}
func watListFilter(t watSpec) string { return "$zhex_list_filter_" + string(t) }
func watListSelect(t watSpec) string { return "$zhex_list_select_" + string(t) }
func watListFold(t, acc watSpec) string {
	return "$zhex_list_fold_" + string(t) + "_" + string(acc)
}
func watListFind(t watSpec) string { return "$zhex_list_find_" + string(t) }
func watListAny(t watSpec) string  { return "$zhex_list_any_" + string(t) }
func watListAll(t watSpec) string  { return "$zhex_list_all_" + string(t) }
func watListSort(t watSpec) string { return "$zhex_list_sort_" + string(t) }
func watListSortBy(t watSpec, key watSpec) string {
	return "$zhex_list_sort_by_" + string(t) + "_" + string(key)
}
func watArraySelect(t watSpec) string { return "$zhex_array_select_" + string(t) }
