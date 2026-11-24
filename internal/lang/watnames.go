package lang

import "strings"

// Names shared between the WAT emitter (wat.go) and the runtime prelude
// (watprelude.go), pinned in internal/wasm/DESIGN.md §2 (types), §5
// (helpers), and §6a (fail codes). Builder B's prelude defines these symbols
// in WAT; the emitter references them by these exact spellings.

const (
	watImportPrint = "$env.print" // (import "env" "print" (func $env.print (param i32 i32)))

	watTypeString = "$zhex_string" // (array (mut i32)) of Unicode code points

	watTypeArrI64 = "$zhex_arr_i64"
	watTypeArrI32 = "$zhex_arr_i32"
	watTypeArrRef = "$zhex_arr_ref"

	watTypeListI64 = "$zhex_list_i64"
	watTypeListI32 = "$zhex_list_i32"
	watTypeListRef = "$zhex_list_ref"

	watTypeArrayI64 = "$zhex_array_i64"
	watTypeArrayI32 = "$zhex_array_i32"
	watTypeArrayRef = "$zhex_array_ref"

	watTypeSomeI64 = "$zhex_some_i64"
	watTypeSomeI32 = "$zhex_some_i32"
	watTypeSomeRef = "$zhex_some_ref"
	watTypeNone    = "$zhex_none"

	watTypeResult = "$zhex_result"

	watGlobalDepth = "$zhex_depth"
	watGlobalErr   = "$zhex_err"

	watFail        = "$zhex_fail"
	watEnter       = "$zhex_enter"
	watLeave       = "$zhex_leave"
	watAdd         = "$zhex_add"
	watSub         = "$zhex_sub"
	watMul         = "$zhex_mul"
	watNeg         = "$zhex_neg"
	watDiv         = "$zhex_div"
	watRem         = "$zhex_rem"
	watIntShl      = "$zhex_shl"
	watIntShr      = "$zhex_shr"
	watIntToString = "$zhex_int_to_string"
	watStrEq       = "$zhex_str_eq"
	watStrConcat   = "$zhex_str_concat"
	watPrintStr    = "$zhex_print_str"
	watPrintInt    = "$zhex_print_int"
	watUnboxI32    = "$zhex_unbox_i32"
)

// Fail codes for $zhex_fail, mapping one-to-one onto the runner's stderr
// strings in DESIGN.md §6a. The codes are the artifact's runtime contract;
// never renumber.
const (
	watFailOverflow     = 1  // integer-overflow
	watFailDepth        = 2  // depth-limit
	watFailListRange    = 3  // list-range
	watFailArrayRange   = 4  // array-range
	watFailArrayRank    = 5  // array-rank (rank outside 1..32)
	watFailArrayShape   = 6  // array-shape (negative extent)
	watFailArrayIndex   = 7  // array-rank (index has the wrong rank)
	watFailListLimit    = 8  // collection-limit (list)
	watFailArrayLimit   = 9  // collection-limit (array)
	watFailIterating    = 10 // collection-iterating
	watFailUnhandled    = 11 // unhandled-result
	watFailShiftCount   = 20 // int-shift-count (negative shift count)
	watFailShiftWidth   = 21 // int-shift-width (count of 64 or more in the 64-bit build)
	watFailDivisionZero = 19 // division-zero
	watFailStackRange   = 22 // stack-range
)

// watSpec is the element specialization of a collection or Option payload:
// i64 (Int), i32 (Bool), ref (everything else), per DESIGN.md §2.
type watSpec string

const (
	watSpecI64 watSpec = "i64"
	watSpecI32 watSpec = "i32"
	watSpecRef watSpec = "ref"
)

// Prelude helper names per specialization; X ∈ {i64, i32, ref}.
func watArrType(s watSpec) string     { return "$zhex_arr_" + string(s) }
func watListType(s watSpec) string    { return "$zhex_list_" + string(s) }
func watArrayType(s watSpec) string   { return "$zhex_array_" + string(s) }
func watSomeType(s watSpec) string    { return "$zhex_some_" + string(s) }
func watListNew(s watSpec) string     { return "$zhex_list_new_" + string(s) }
func watListAppend(s watSpec) string  { return "$zhex_list_append_" + string(s) }
func watListLength(s watSpec) string  { return "$zhex_list_length_" + string(s) }
func watListGet(s watSpec) string     { return "$zhex_list_get_" + string(s) }
func watListSet(s watSpec) string     { return "$zhex_list_set_" + string(s) }
func watListPop(s watSpec) string     { return "$zhex_list_pop_" + string(s) }
func watListFirst(s watSpec) string   { return "$zhex_list_first_" + string(s) }
func watListLast(s watSpec) string    { return "$zhex_list_last_" + string(s) }

// The stack words (#236) lower onto the list repr with the top at the end;
// underflow fails code 22, the interpreter's stack-range (the family's
// list-range shape). push reuses the plain append helper.
func watStackPop(s watSpec) string   { return "$zhex_stack_pop_" + string(s) }
func watStackPeek(s watSpec) string  { return "$zhex_stack_peek_" + string(s) }
func watStackDup(s watSpec) string   { return "$zhex_stack_dup_" + string(s) }
func watStackSwap(s watSpec) string  { return "$zhex_stack_swap_" + string(s) }
func watStackDrop(s watSpec) string  { return "$zhex_stack_drop_" + string(s) }
func watArrayNew(s watSpec) string    { return "$zhex_array_new_" + string(s) }
func watArrayGet(s watSpec) string    { return "$zhex_array_get_" + string(s) }
func watArraySet(s watSpec) string    { return "$zhex_array_set_" + string(s) }
func watArrayLength(s watSpec) string { return "$zhex_array_length_" + string(s) }

// watMangle maps a Zhex qualified name injectively onto a WAT identifier:
// each '_' becomes "_u", '-' becomes "_h", '.' becomes "_q"; every other
// character passes through (DESIGN.md §2).
func watMangle(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch r {
		case '_':
			b.WriteString("_u")
		case '-':
			b.WriteString("_h")
		case '.':
			b.WriteString("_q")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// watWordName is a word's wasm function name: $zw_<mangled qualified name>.
func watWordName(w *word) string {
	name := w.name
	if w.module != "" {
		name = w.module + "." + w.name
	}
	return "$zw_" + watMangle(name)
}

// watRecordName is a user record type's struct name: $zt_<mangled id>.
func watRecordName(r *recordType) string { return "$zt_" + watMangle(r.id()) }

// watVariantName is one choice variant's struct name:
// $zt_<mangled type id>_<Variant>.
func watVariantName(r *recordType, variant int) string {
	return watRecordName(r) + "_" + watMangle(r.variants[variant].name)
}
