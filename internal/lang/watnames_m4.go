package lang

// Milestone-4 names shared between the WAT emitter (wat.go), the M4 runtime
// prelude (watprelude_m4.go), and the corpus/fuzz gate (internal/wasm),
// pinned in internal/wasm/DESIGN-M4.md §2 (xml.decode: pure computation,
// family-5 failures through the existing err struct, no new fail codes), §3
// (the fenced trio: void closure grid rows, re-partial env-slot-N targets,
// closure-valued flow steps), and §4 (the checked List index read). The
// prelude defines these symbols in WAT; the emitter references them by these
// exact spellings. B2 compiles against this file — check names in first.

// watCloVoid is the closure-grid void result marker: watCloType(params,
// watCloVoid) names the six `(func (param (ref $zhex_closure)) (param …)`
// rows with no result (DESIGN-M4 §3a). Arity 0 is impossible — a partial
// needs a hole. Never passed to watReprType; the emitter treats a Unit
// closure result as "no wasm result".
const watCloVoid = watSpec("v")

// The xml element tree (DESIGN-M4 §2a): one rec group, attrs as $zxml_attr
// structs boxed in the eqref array, children as elem refs boxed likewise,
// text accumulating by $zhex_str_concat (O(n²) accepted; parity beats
// speed). The names-table global pairs schema-node refs with their Zhex
// type names (filled by the emitter's schema builders) so failure messages
// render `expected <Type>` exactly as the interpreter's jsonSchema.typ does.
const (
	watTypeXmlElem = "$zxml_elem"
	watTypeXmlAttr = "$zxml_attr"
	watXmlNames    = "$zxml_names" // (mut (ref null $zhex_arr_ref)) of [node, name] pairs
)

// xml.decode (DESIGN-M4 §2): one prelude entry point mirroring json.decode's
// emitter case — raw text, boxed defaults, and the M3 $zjson_schema<N>
// builder value; every failure is $zjson_fail's family-5
// "<message> at \"<path>\"" envelope.
const watXmlDecode = "$zhex_xml_decode"

// The checked List index read (DESIGN-M4 §4): in bounds → tag 0 + M1 boxing;
// out of bounds → tag 1 + $zhex_err{5, "list index is out of range"}
// (interpreter listRead: code list-range, operation list.index; the
// index/length details stay interpreter-envelope-only). Checked Array reads
// and all checked writes stay fenced.
func watListGetChecked(s watSpec) string { return "$zhex_list_get_checked_" + string(s) }
