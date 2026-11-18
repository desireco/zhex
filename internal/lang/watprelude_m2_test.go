package lang

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// TestWatPreludeM2PinnedNames pins the shared M2 interface (DESIGN-M2 §1/§2/
// §3/§4): every type, helper, and export the emitter or the Node runner
// addresses by name must appear byte-for-byte in the prelude — on tool-less
// CI included. The names come from watnames_m2.go, the single surface both
// builders share.
func TestWatPreludeM2PinnedNames(t *testing.T) {
	pinned := []string{
		// $details amended by #135 (alternating key/value pairs, null = none)
		"(type $zhex_err (struct (field $family i32) (field $message (ref null $zhex_string)) (field $details (ref null $zhex_arr_str))))",
		"(type $zhex_arr_str (array (mut (ref $zhex_string))))",
		"(type $zhex_closure (struct (field $fn (ref func)) (field $env (ref null $zhex_arr_ref))))",
		`(func $zhex_get_msg_len (export "zhex_msg_len") (result i32)`,
		"(func $zhex_fail_msg (param $code i32) (param $msg (ref $zhex_string))",
		"(func $zhex_stage_utf8 (param $s (ref $zhex_string)) (result i32)",
		"(func $zhex_str_lt (param $a (ref $zhex_string)) (param $b (ref $zhex_string)) (result i32)",
		"(func $zhex_text_to_int (param $t (ref $zhex_string)) (result (ref $zhex_result))",
		"(func $zhex_text_split (param $t (ref $zhex_string)) (param $sep (ref $zhex_string)) (result (ref $zhex_list_ref))",
		"(func $zhex_text_join (param $values (ref $zhex_list_ref)) (param $sep (ref $zhex_string)) (result (ref $zhex_string))",
		"(func $zhex_text_replace (param $t (ref $zhex_string)) (param $old (ref $zhex_string)) (param $new (ref $zhex_string)) (result (ref $zhex_string))",
		"(global $zhex_case_upper (ref $zhex_arr_i32)",
		"(global $zhex_case_lower (ref $zhex_arr_i32)",
		"(global $zhex_print_ranges (ref $zhex_arr_i32)", // DESIGN-M4 §5
		"(func $zhex_cp_isprint (param $cp i32) (result i32)",
		"(func $zhex_put_hex (param $r (ref $zhex_string)) (param $w i32) (param $cp i32) (param $digits i32)",
	}
	for _, w := range []string{
		watTextReverse, watTextSlice, watTextIndexOf, watTextContains,
		watTextUpper, watTextLower, watTextTrim, watTextJoin,
		watTextReplace, watTextStartsWith, watTextEndsWith, watTextLineCount,
		watTextWordCount, watStrLt,
	} {
		pinned = append(pinned, "(func "+w+" ")
	}
	for _, k := range []watMapKey{watMapKeyI64, watMapKeyI32, watMapKeyStr} {
		for _, v := range []watSpec{watSpecI64, watSpecI32, watSpecRef} {
			pinned = append(pinned,
				"(type "+watMapType(k, v)+"\n",
				"(func "+watMapNew(k, v)+" ",
				"(func "+watMapPut(k, v)+" ",
				"(func "+watMapLookup(k, v)+" ",
				"(func "+watMapRemove(k, v)+" ",
				"(func "+watMapContains(k, v)+" ",
				"(func "+watMapLength(k, v)+" ",
				"(func "+watMapInto(k, v)+" ",
			)
		}
	}
	for _, p := range []watSpec{watSpecI64, watSpecI32, watSpecRef} {
		for _, r := range []watSpec{watSpecI64, watSpecI32, watSpecRef} {
			pinned = append(pinned,
				"(type "+watCloType([]watSpec{p}, r)+" ",
				"(func "+watListMap(p, r)+" ",
				"(func "+watListFold(p, r)+" ",
			)
			for _, p2 := range []watSpec{watSpecI64, watSpecI32, watSpecRef} {
				pinned = append(pinned, "(type "+watCloType([]watSpec{p, p2}, r)+" ")
			}
		}
		pinned = append(pinned,
			"(func "+watListFilter(p)+" ",
			"(func "+watListSelect(p)+" ",
			"(func "+watListFind(p)+" ",
			"(func "+watListAny(p)+" ",
			"(func "+watListAll(p)+" ",
			"(func "+watListSortBy(p, watSpecI64)+" ",
			"(func "+watListSortBy(p, watSpecRef)+" ",
			"(func "+watArraySelect(p)+" ",
		)
	}
	pinned = append(pinned,
		"(func "+watListSort(watSpecI64)+" ",
		"(func "+watListSort(watSpecRef)+" ",
	)
	for _, want := range pinned {
		if !strings.Contains(watPreludeM2, want) {
			t.Errorf("M2 prelude is missing pinned name: %s", strconv.Quote(want))
		}
	}
}

// TestWatCaseTablesStable pins the runtime-generated case tables on a fixed
// sample of code points whose simple mappings Unicode's stability policy
// freezes (ASCII plus long-standing non-ASCII), and checks generation is
// deterministic within a process. A span-count comparison against committed
// constants was environment-dependent by construction — the tables now
// generate from the running toolchain's unicode package — so the only frozen
// expectations left are mappings that can never change.
func TestWatCaseTablesStable(t *testing.T) {
	tables := watCaseTables()
	if again := watCaseTables(); again != tables {
		t.Fatal("watCaseTables is not deterministic within a process")
	}
	up := parseCaseTable(t, tables, "upper")
	lo := parseCaseTable(t, tables, "lower")
	for _, s := range [][][3]int32{up, lo} {
		for i := 1; i < len(s); i++ {
			if s[i][0] <= s[i-1][1] {
				t.Errorf("span %d overlaps the previous span (binary search precondition)", i)
			}
		}
	}
	for _, c := range []struct{ cp, up, lo rune }{
		{'a', 'A', 'a'}, {'z', 'Z', 'z'}, {'A', 'A', 'a'}, {'0', '0', '0'}, {'\u007f', '\u007f', '\u007f'},
		{'\u00e9', '\u00c9', '\u00e9'}, {'\u00c9', '\u00c9', '\u00e9'}, // é É
		{'\u00df', '\u00df', '\u00df'},                                 // sharp s stays
		{'\u00ff', '\u0178', '\u00ff'},                                 // ÿ → Ÿ
		{'\u017f', 'S', '\u017f'},                                      // ſ → S
		{'\u03c3', '\u03a3', '\u03c3'}, {'\u03a3', '\u03a3', '\u03c3'}, // σ Σ simple
		{'\u03a9', '\u03a9', '\u03c9'}, {'\u03c9', '\u03a9', '\u03c9'}, // Ω ω
		{'\u0100', '\u0100', '\u0101'}, {'\u0101', '\u0100', '\u0101'}, // Ā ā pair
		{'\U0001F600', '\U0001F600', '\U0001F600'}, // emoji unchanged
	} {
		if got := applyCaseSpans(up, c.cp); got != c.up {
			t.Errorf("upper(%U): got %U, want %U", c.cp, got, c.up)
		}
		if got := applyCaseSpans(lo, c.cp); got != c.lo {
			t.Errorf("lower(%U): got %U, want %U", c.cp, got, c.lo)
		}
	}
}

// TestWatCaseTablesMatchRuneScan guards the fast unicode.CaseRanges walk in
// watCaseTables against the authoritative per-rune semantics. Both sides are
// computed in this process from the same unicode package, so unlike the
// committed-constant comparison it replaced, it cannot drift with the
// toolchain.
func TestWatCaseTablesMatchRuneScan(t *testing.T) {
	tables := watCaseTables()
	for name, dir := range map[string]func(rune) rune{
		"upper": unicode.ToUpper, "lower": unicode.ToLower,
	} {
		want := scanCaseSpans(dir)
		got := parseCaseTable(t, tables, name)
		if len(got) != len(want) {
			t.Fatalf("%s table: generated %d spans, per-rune scan has %d", name, len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s span %d: generated [%d %d %d], scan [%d %d %d]",
					name, i, got[i][0], got[i][1], got[i][2], want[i][0], want[i][1], want[i][2])
			}
		}
	}
}

// scanCaseSpans mirrors the offline generator: maximal ranges where the
// simple case delta is constant and nonzero.
func scanCaseSpans(dir func(rune) rune) [][3]int32 {
	var out [][3]int32
	flush := func(lo, hi, delta rune) {
		if lo >= 0 {
			out = append(out, [3]int32{int32(lo), int32(hi), int32(delta)})
		}
	}
	lo, hi, delta := rune(-1), rune(-1), rune(0)
	for r := rune(0); r <= 0x10FFFF; r++ {
		d := dir(r) - r
		if d == 0 {
			flush(lo, hi, delta)
			lo = -1
			continue
		}
		if lo >= 0 && r == hi+1 && d == delta {
			hi = r
			continue
		}
		flush(lo, hi, delta)
		lo, hi, delta = r, r, d
	}
	flush(lo, hi, delta)
	return out
}

func parseCaseTable(t *testing.T, tables, name string) [][3]int32 {
	t.Helper()
	rest := tables
	i := strings.Index(rest, "$zhex_case_"+name)
	if i < 0 {
		t.Fatalf("case table %s missing from watCaseTables", name)
	}
	rest = rest[i:]
	if j := strings.Index(rest, "(global"); j > 0 {
		rest = rest[:j] // stop at the next table's global
	}
	nums := regexp.MustCompile(`\(i32\.const (-?\d+)\)`).FindAllStringSubmatch(rest, -1)
	if len(nums) == 0 || len(nums)%3 != 0 {
		t.Fatalf("%s table: %d constants, want a positive multiple of 3", name, len(nums))
	}
	out := make([][3]int32, len(nums)/3)
	for i := range out {
		for j := 0; j < 3; j++ {
			v, err := strconv.ParseInt(nums[i*3+j][1], 10, 32)
			if err != nil {
				t.Fatalf("%s table: bad constant %q", name, nums[i*3+j][1])
			}
			out[i][j] = int32(v)
		}
	}
	return out
}

// applyCaseSpans applies a parsed span table to one code point the way the
// prelude's $zhex_case_apply does (binary search, delta add, miss = identity).
func applyCaseSpans(spans [][3]int32, cp rune) rune {
	lo, hi := 0, len(spans)-1
	for lo <= hi {
		mid := int(uint(lo+hi) / 2)
		switch s := spans[mid]; {
		case cp < rune(s[0]):
			hi = mid - 1
		case cp > rune(s[1]):
			lo = mid + 1
		default:
			return cp + rune(s[2])
		}
	}
	return cp
}

// TestWatPreludeM2 assembles the M1+M2 prelude inside a test module
// (wasm-tools parse AND validate — parse alone skips type checking,
// DESIGN.md Surprises 1) and behavior-tests every M2 family under node:
// the text suite on multibyte input, the case-table binary search at every
// span boundary, to-int edges with the exact %q message, the staged fail
// messages, Map insertion order incl. removes/re-puts/into and the
// traversal guard, and the closure/callback helpers incl. sort stability
// and the early-exit $active restores. Skipped when either tool is missing
// so tool-less CI stays green.
func TestWatPreludeM2(t *testing.T) {
	wasmTools, err := exec.LookPath("wasm-tools")
	if err != nil {
		t.Skipf("wasm-tools not installed: %v", err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node not installed: %v", err)
	}
	dir := t.TempDir()
	watPath := filepath.Join(dir, "prelude-m2.wat")
	wat := "(module\n" + watPrelude + watPreludeM2 + "\n" + watPreludeM2TestExports + "\n)\n"
	if err := os.WriteFile(watPath, []byte(wat), 0o644); err != nil {
		t.Fatal(err)
	}
	wasmPath := filepath.Join(dir, "prelude-m2.wasm")
	if out, err := exec.Command(wasmTools, "parse", watPath, "-o", wasmPath).CombinedOutput(); err != nil {
		t.Fatalf("wasm-tools parse: %v\n%s", err, out)
	}
	if out, err := exec.Command(wasmTools, "validate", wasmPath).CombinedOutput(); err != nil {
		t.Fatalf("wasm-tools validate: %v\n%s", err, out)
	}
	mjsPath := filepath.Join(dir, "run.mjs")
	if err := os.WriteFile(mjsPath, []byte(watPreludeM2Runner), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, mjsPath, wasmPath).CombinedOutput()
	if err != nil {
		t.Fatalf("node runner failed: %v\n%s", err, out)
	}
	var report struct {
		Total  int              `json:"total"`
		Failed []map[string]any `json:"failed"`
	}
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("runner output is not the JSON report: %v\n%s", err, out)
	}
	if report.Total < 90 {
		t.Errorf("runner executed only %d cases; the suite shrank", report.Total)
	}
	for _, f := range report.Failed {
		t.Errorf("case %v failed: %v", f["name"], f["error"])
	}
}

// watPreludeM2TestExports wraps the M2 helpers in exported functions with
// plain numeric signatures (GC references never cross to JS). Strings enter
// as UTF-8 bytes JS writes at fixed memory offsets and leave through the
// staging slot at offset 0, exactly what the production runner decodes.
const watPreludeM2TestExports = `
(func $t_from_utf8_at (param $off i32) (param $len i32) (result (ref $zhex_string))
  (local $i i32)
  (local $n i32)
  (local $b i32)
  (local $cp i32)
  (local $r (ref null $zhex_string))
  (local $w i32)
  (block $cdone
    (loop $count
      (br_if $cdone (i32.ge_u (local.get $i) (local.get $len)))
      (local.set $b (i32.load8_u (i32.add (local.get $off) (local.get $i))))
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
      (local.set $b (i32.load8_u (i32.add (local.get $off) (local.get $i))))
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
                  (i32.and (i32.load8_u (i32.add (local.get $off) (i32.add (local.get $i) (i32.const 1)))) (i32.const 63))))
              (else
                (if (result i32)
                  (i32.lt_u (local.get $b) (i32.const 240))
                  (then
                    (i32.or
                      (i32.or
                        (i32.shl (i32.and (local.get $b) (i32.const 15)) (i32.const 12))
                        (i32.shl (i32.and (i32.load8_u (i32.add (local.get $off) (i32.add (local.get $i) (i32.const 1)))) (i32.const 63)) (i32.const 6)))
                      (i32.and (i32.load8_u (i32.add (local.get $off) (i32.add (local.get $i) (i32.const 2)))) (i32.const 63))))
                  (else
                    (i32.or
                      (i32.or
                        (i32.shl (i32.and (local.get $b) (i32.const 7)) (i32.const 18))
                        (i32.shl (i32.and (i32.load8_u (i32.add (local.get $off) (i32.add (local.get $i) (i32.const 1)))) (i32.const 63)) (i32.const 12)))
                      (i32.or
                        (i32.shl (i32.and (i32.load8_u (i32.add (local.get $off) (i32.add (local.get $i) (i32.const 2)))) (i32.const 63)) (i32.const 6))
                        (i32.and (i32.load8_u (i32.add (local.get $off) (i32.add (local.get $i) (i32.const 3)))) (i32.const 63)))))))))))
      (local.set $i (i32.add (local.get $i)
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
      (array.set $zhex_string (local.get $r) (local.get $w) (local.get $cp))
      (local.set $w (i32.add (local.get $w) (i32.const 1)))
      (br $decode)))
  (ref.as_non_null (local.get $r)))

(func (export "t_upper") (param $len i32) (result i32)
  (call $zhex_stage_utf8 (call $zhex_text_upper (call $t_from_utf8_at (i32.const 0) (local.get $len)))))
(func (export "t_lower") (param $len i32) (result i32)
  (call $zhex_stage_utf8 (call $zhex_text_lower (call $t_from_utf8_at (i32.const 0) (local.get $len)))))
(func (export "t_reverse") (param $len i32) (result i32)
  (call $zhex_stage_utf8 (call $zhex_text_reverse (call $t_from_utf8_at (i32.const 0) (local.get $len)))))
(func (export "t_trim") (param $len i32) (result i32)
  (call $zhex_stage_utf8 (call $zhex_text_trim (call $t_from_utf8_at (i32.const 0) (local.get $len)))))
(func (export "t_slice") (param $len i32) (param $start i64) (param $end i64) (result i32)
  (call $zhex_stage_utf8
    (call $zhex_text_slice (call $t_from_utf8_at (i32.const 0) (local.get $len)) (local.get $start) (local.get $end))))
(func (export "t_index_of") (param $la i32) (param $lb i32) (result i64)
  (call $zhex_text_index_of (call $t_from_utf8_at (i32.const 0) (local.get $la)) (call $t_from_utf8_at (i32.const 4096) (local.get $lb))))
(func (export "t_contains") (param $la i32) (param $lb i32) (result i32)
  (call $zhex_text_contains (call $t_from_utf8_at (i32.const 0) (local.get $la)) (call $t_from_utf8_at (i32.const 4096) (local.get $lb))))
(func (export "t_starts_with") (param $la i32) (param $lb i32) (result i32)
  (call $zhex_text_starts_with (call $t_from_utf8_at (i32.const 0) (local.get $la)) (call $t_from_utf8_at (i32.const 4096) (local.get $lb))))
(func (export "t_ends_with") (param $la i32) (param $lb i32) (result i32)
  (call $zhex_text_ends_with (call $t_from_utf8_at (i32.const 0) (local.get $la)) (call $t_from_utf8_at (i32.const 4096) (local.get $lb))))
(func (export "t_line_count") (param $len i32) (result i64)
  (call $zhex_text_line_count (call $t_from_utf8_at (i32.const 0) (local.get $len))))
(func (export "t_word_count") (param $len i32) (result i64)
  (call $zhex_text_word_count (call $t_from_utf8_at (i32.const 0) (local.get $len))))
(func (export "t_replace") (param $la i32) (param $lb i32) (param $lc i32) (result i32)
  (call $zhex_stage_utf8
    (call $zhex_text_replace
      (call $t_from_utf8_at (i32.const 0) (local.get $la))
      (call $t_from_utf8_at (i32.const 4096) (local.get $lb))
      (call $t_from_utf8_at (i32.const 8192) (local.get $lc)))))
(func (export "t_split_count") (param $la i32) (param $lb i32) (result i64)
  (call $zhex_list_length_ref
    (call $zhex_text_split (call $t_from_utf8_at (i32.const 0) (local.get $la)) (call $t_from_utf8_at (i32.const 4096) (local.get $lb)))))
(func (export "t_split_join") (param $la i32) (param $lb i32) (result i32)
  (call $zhex_stage_utf8
    (call $zhex_text_join
      (call $zhex_text_split (call $t_from_utf8_at (i32.const 0) (local.get $la)) (call $t_from_utf8_at (i32.const 4096) (local.get $lb)))
      (array.new_fixed $zhex_string 1 (i32.const 44)))))
(func (export "t_to_int_tag") (param $len i32) (result i32)
  (struct.get $zhex_result $tag (call $zhex_text_to_int (call $t_from_utf8_at (i32.const 0) (local.get $len)))))
(func (export "t_to_int_val") (param $len i32) (result i64)
  (struct.get $zhex_some_i64 $value
    (ref.cast (ref $zhex_some_i64)
      (struct.get $zhex_result $payload (call $zhex_text_to_int (call $t_from_utf8_at (i32.const 0) (local.get $len)))))))
(func (export "t_to_int_msg") (param $len i32) (result i32)
  (call $zhex_stage_utf8
    (ref.as_non_null
      (struct.get $zhex_err $message
        (ref.cast (ref $zhex_err)
          (struct.get $zhex_result $payload (call $zhex_text_to_int (call $t_from_utf8_at (i32.const 0) (local.get $len)))))))))
(func (export "t_err_family") (param $len i32) (result i32)
  (struct.get $zhex_err $family
    (ref.cast (ref $zhex_err)
      (struct.get $zhex_result $payload (call $zhex_text_to_int (call $t_from_utf8_at (i32.const 0) (local.get $len)))))))
(func (export "t_upper_cp") (param $cp i32) (result i32)
  (call $zhex_case_apply (local.get $cp) (global.get $zhex_case_upper)))
(func (export "t_lower_cp") (param $cp i32) (result i32)
  (call $zhex_case_apply (local.get $cp) (global.get $zhex_case_lower)))
(func (export "t_str_lt") (param $la i32) (param $lb i32) (result i32)
  (call $zhex_str_lt (call $t_from_utf8_at (i32.const 0) (local.get $la)) (call $t_from_utf8_at (i32.const 4096) (local.get $lb))))
(func (export "t_fail12")
  (call $zhex_fail_msg (i32.const 12) (call $zhex_int_to_string (i64.const 314))))

;; ---- map scenarios (str_i64 flagship + one smoke per other key/value) ----
(func $t_mapA (result (ref $zhex_string)) (array.new_fixed $zhex_string 1 (i32.const 97)))
(func $t_mapB (result (ref $zhex_string)) (array.new_fixed $zhex_string 1 (i32.const 98)))
(func $t_mapC (result (ref $zhex_string)) (array.new_fixed $zhex_string 1 (i32.const 99)))
(func $t_mapD (result (ref $zhex_string)) (array.new_fixed $zhex_string 1 (i32.const 100)))
(func $t_mapE (result (ref $zhex_string)) (array.new_fixed $zhex_string 1 (i32.const 101)))
(func $t_mapX (result (ref $zhex_string)) (array.new_fixed $zhex_string 1 (i32.const 120)))
(func $t_mapY (result (ref $zhex_string)) (array.new_fixed $zhex_string 1 (i32.const 121)))
(func $t_mapZ (result (ref $zhex_string)) (array.new_fixed $zhex_string 1 (i32.const 122)))
(func $t_mapZZ (result (ref $zhex_string)) (array.new_fixed $zhex_string 2 (i32.const 122) (i32.const 122)))

(func $t_map_scenario (result (ref $zhex_map_str_i64))
  (local $m (ref $zhex_map_str_i64))
  (local.set $m (call $zhex_map_new_str_i64))
  (drop (call $zhex_map_put_str_i64 (local.get $m) (call $t_mapA) (i64.const 1)))
  (drop (call $zhex_map_put_str_i64 (local.get $m) (call $t_mapB) (i64.const 2)))
  (drop (call $zhex_map_put_str_i64 (local.get $m) (call $t_mapC) (i64.const 3)))
  (drop (call $zhex_map_put_str_i64 (local.get $m) (call $t_mapB) (i64.const 9)))
  (drop (call $zhex_map_remove_str_i64 (local.get $m) (call $t_mapA)))
  (drop (call $zhex_map_put_str_i64 (local.get $m) (call $t_mapA) (i64.const 7)))
  (drop (call $zhex_map_put_str_i64 (local.get $m) (call $t_mapD) (i64.const 4)))
  (local.get $m))

(func (export "t_map_len") (result i64)
  (call $zhex_map_length_str_i64 (call $t_map_scenario)))
(func (export "t_map_key_at") (param $i i64) (result i32)
  (call $zhex_stage_utf8
    (array.get $zhex_arr_str (struct.get $zhex_map_str_i64 $keys (call $t_map_scenario))
      (i32.wrap_i64 (local.get $i)))))
(func (export "t_map_val_at") (param $i i64) (result i64)
  (array.get $zhex_arr_i64 (struct.get $zhex_map_str_i64 $vals (call $t_map_scenario))
    (i32.wrap_i64 (local.get $i))))
(func (export "t_map_contains_yes") (result i32)
  (call $zhex_map_contains_str_i64 (call $t_map_scenario) (call $t_mapB)))
(func (export "t_map_contains_no") (result i32)
  (call $zhex_map_contains_str_i64 (call $t_map_scenario) (call $t_mapZZ)))
(func (export "t_map_lookup_some_val") (result i64)
  (struct.get $zhex_some_i64 $value
    (ref.cast (ref $zhex_some_i64)
      (call $zhex_map_lookup_str_i64 (call $t_map_scenario) (call $t_mapB)))))
(func (export "t_map_lookup_absent_none") (result i32)
  (ref.test (ref $zhex_none) (call $zhex_map_lookup_str_i64 (call $t_map_scenario) (call $t_mapZZ))))
(func (export "t_map_lookup_some_test") (result i32)
  (ref.test (ref $zhex_some_i64) (call $zhex_map_lookup_str_i64 (call $t_map_scenario) (call $t_mapB))))
(func (export "t_map_remove_missing_len") (result i64)
  (local $m (ref $zhex_map_str_i64))
  (local.set $m (call $t_map_scenario))
  (drop (call $zhex_map_remove_str_i64 (local.get $m) (call $t_mapZZ)))
  (call $zhex_map_length_str_i64 (local.get $m)))
(func (export "t_map_guard_put")
  (local $m (ref $zhex_map_str_i64))
  (local.set $m (call $t_map_scenario))
  (struct.set $zhex_map_str_i64 $active (local.get $m) (i32.const 1))
  (drop (call $zhex_map_put_str_i64 (local.get $m) (call $t_mapA) (i64.const 1))))
(func (export "t_map_guard_remove_noop")
  (local $m (ref $zhex_map_str_i64))
  (local.set $m (call $t_map_scenario))
  (struct.set $zhex_map_str_i64 $active (local.get $m) (i32.const 1))
  (drop (call $zhex_map_remove_str_i64 (local.get $m) (call $t_mapZZ))))
(func (export "t_map_guard_into")
  (local $src (ref $zhex_map_str_i64))
  (local $tgt (ref $zhex_map_str_i64))
  (local.set $src (call $t_map_src2))
  (local.set $tgt (call $t_map_tgt2))
  (struct.set $zhex_map_str_i64 $active (local.get $tgt) (i32.const 1))
  (drop (call $zhex_map_into_str_i64 (local.get $src) (local.get $tgt))))
(func (export "t_map_lookup_active") (result i64)
  (local $m (ref $zhex_map_str_i64))
  (local.set $m (call $t_map_scenario))
  (struct.set $zhex_map_str_i64 $active (local.get $m) (i32.const 1))
  (struct.get $zhex_some_i64 $value
    (ref.cast (ref $zhex_some_i64) (call $zhex_map_lookup_str_i64 (local.get $m) (call $t_mapB)))))
(func (export "t_map_contains_active") (result i32)
  (local $m (ref $zhex_map_str_i64))
  (local.set $m (call $t_map_scenario))
  (struct.set $zhex_map_str_i64 $active (local.get $m) (i32.const 1))
  (call $zhex_map_contains_str_i64 (local.get $m) (call $t_mapB)))
(func (export "t_map_len_active") (result i64)
  (local $m (ref $zhex_map_str_i64))
  (local.set $m (call $t_map_scenario))
  (struct.set $zhex_map_str_i64 $active (local.get $m) (i32.const 1))
  (call $zhex_map_length_str_i64 (local.get $m)))
(func (export "t_map_alias_len") (result i64)
  (local $m (ref $zhex_map_str_i64))
  (local $alias (ref null $zhex_map_str_i64))
  (local.set $m (call $t_map_scenario))
  (local.set $alias (local.get $m))
  (drop (call $zhex_map_put_str_i64 (ref.as_non_null (local.get $alias)) (call $t_mapE) (i64.const 5)))
  (call $zhex_map_length_str_i64 (local.get $m)))
(func (export "t_map_i64_lookup") (result i64)
  (local $m (ref $zhex_map_i64_i64))
  (local.set $m (call $zhex_map_new_i64_i64))
  (drop (call $zhex_map_put_i64_i64 (local.get $m) (i64.const 10) (i64.const 100)))
  (drop (call $zhex_map_put_i64_i64 (local.get $m) (i64.const 20) (i64.const 200)))
  (struct.get $zhex_some_i64 $value
    (ref.cast (ref $zhex_some_i64) (call $zhex_map_lookup_i64_i64 (local.get $m) (i64.const 20)))))
(func (export "t_map_i32_lookup") (result i64)
  (local $m (ref $zhex_map_i32_i64))
  (local.set $m (call $zhex_map_new_i32_i64))
  (drop (call $zhex_map_put_i32_i64 (local.get $m) (i32.const 1) (i64.const 7)))
  (struct.get $zhex_some_i64 $value
    (ref.cast (ref $zhex_some_i64) (call $zhex_map_lookup_i32_i64 (local.get $m) (i32.const 1)))))
(func (export "t_map_strref_lookup") (result i32)
  (local $m (ref $zhex_map_str_ref))
  (local.set $m (call $zhex_map_new_str_ref))
  (drop (call $zhex_map_put_str_ref (local.get $m) (call $t_mapA)
    (array.new_fixed $zhex_string 1 (i32.const 118))))
  (call $zhex_stage_utf8
    (ref.cast (ref $zhex_string)
      (struct.get $zhex_some_ref $value
        (ref.cast (ref $zhex_some_ref) (call $zhex_map_lookup_str_ref (local.get $m) (call $t_mapA)))))))
(func $t_map_src2 (result (ref $zhex_map_str_i64))
  (local $m (ref $zhex_map_str_i64))
  (local.set $m (call $zhex_map_new_str_i64))
  (drop (call $zhex_map_put_str_i64 (local.get $m) (call $t_mapX) (i64.const 1)))
  (drop (call $zhex_map_put_str_i64 (local.get $m) (call $t_mapY) (i64.const 2)))
  (local.get $m))
(func $t_map_tgt2 (result (ref $zhex_map_str_i64))
  (local $m (ref $zhex_map_str_i64))
  (local.set $m (call $zhex_map_new_str_i64))
  (drop (call $zhex_map_put_str_i64 (local.get $m) (call $t_mapY) (i64.const 5)))
  (drop (call $zhex_map_put_str_i64 (local.get $m) (call $t_mapZ) (i64.const 6)))
  (local.get $m))
(func (export "t_map_into_len") (result i64)
  (call $zhex_map_length_str_i64 (call $zhex_map_into_str_i64 (call $t_map_src2) (call $t_map_tgt2))))
(func (export "t_map_into_key_at") (param $i i64) (result i32)
  (call $zhex_stage_utf8
    (array.get $zhex_arr_str
      (struct.get $zhex_map_str_i64 $keys (call $zhex_map_into_str_i64 (call $t_map_src2) (call $t_map_tgt2)))
      (i32.wrap_i64 (local.get $i)))))
(func (export "t_map_into_val_at") (param $i i64) (result i64)
  (array.get $zhex_arr_i64
    (struct.get $zhex_map_str_i64 $vals (call $zhex_map_into_str_i64 (call $t_map_src2) (call $t_map_tgt2)))
    (i32.wrap_i64 (local.get $i))))
(func (export "t_map_into_self_len") (result i64)
  (local $m (ref $zhex_map_str_i64))
  (local.set $m (call $t_map_src2))
  (call $zhex_map_length_str_i64 (call $zhex_map_into_str_i64 (local.get $m) (local.get $m))))
(func (export "t_map_into_src_active_len") (result i64)
  (local $src (ref $zhex_map_str_i64))
  (local.set $src (call $t_map_src2))
  (struct.set $zhex_map_str_i64 $active (local.get $src) (i32.const 1))
  (call $zhex_map_length_str_i64 (call $zhex_map_into_str_i64 (local.get $src) (call $t_map_tgt2))))

;; ---- callbacks: thunks, closures, lists, arrays --------------------------
(func $t_inc (param $clo (ref $zhex_closure)) (param $x i64) (result i64)
  (i64.add (local.get $x) (i64.const 1)))
(func $t_zero (param $clo (ref $zhex_closure)) (param $x i64) (result i64)
  (i64.const 0))
(func $t_add2 (param $clo (ref $zhex_closure)) (param $a i64) (param $b i64) (result i64)
  (i64.add (local.get $a) (local.get $b)))
(func $t_gt_env (param $clo (ref $zhex_closure)) (param $x i64) (result i32)
  (i64.gt_s (local.get $x)
    (struct.get $zhex_some_i64 $value
      (ref.cast (ref $zhex_some_i64)
        (array.get $zhex_arr_ref (struct.get $zhex_closure $env (local.get $clo)) (i32.const 0))))))
(func $t_up_ref (param $clo (ref $zhex_closure)) (param $s (ref null eq)) (result (ref null eq))
  (call $zhex_text_upper (ref.cast (ref $zhex_string) (local.get $s))))
(func $t_id_ref (param $clo (ref $zhex_closure)) (param $s (ref null eq)) (result (ref null eq))
  (local.get $s))
(func $t_len_key (param $clo (ref $zhex_closure)) (param $s (ref null eq)) (result i64)
  (i64.extend_i32_u (array.len (ref.cast (ref $zhex_string) (local.get $s)))))
(func $t_lower_key (param $clo (ref $zhex_closure)) (param $s (ref null eq)) (result (ref null eq))
  (call $zhex_text_lower (ref.cast (ref $zhex_string) (local.get $s))))
(func $t_row_gt5 (param $clo (ref $zhex_closure)) (param $row (ref null eq)) (result i32)
  (i64.gt_s (call $zhex_list_get_i64 (ref.cast (ref $zhex_list_i64) (local.get $row)) (i64.const 0)) (i64.const 5)))
(func $t_row_keep_append (param $clo (ref $zhex_closure)) (param $row (ref null eq)) (result i32)
  (local $l (ref $zhex_list_i64))
  (local.set $l (ref.cast (ref $zhex_list_i64) (local.get $row)))
  (if (i64.gt_s (call $zhex_list_get_i64 (local.get $l) (i64.const 0)) (i64.const 5))
    (then
      (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 99)))
      (return (i32.const 1))))
  (i32.const 0))
(func $t_append_src (param $clo (ref $zhex_closure)) (param $x i64) (result i64)
  (drop (call $zhex_list_append_i64
    (ref.cast (ref $zhex_list_i64)
      (array.get $zhex_arr_ref (struct.get $zhex_closure $env (local.get $clo)) (i32.const 0)))
    (i64.const 99)))
  (local.get $x))
(func $t_clo_inc (result (ref $zhex_closure))
  (struct.new $zhex_closure (ref.func $t_inc) (array.new_fixed $zhex_arr_ref 0)))
(func $t_clo_add2 (result (ref $zhex_closure))
  (struct.new $zhex_closure (ref.func $t_add2) (array.new_fixed $zhex_arr_ref 0)))
(func $t_clo_gt3 (result (ref $zhex_closure))
  (struct.new $zhex_closure (ref.func $t_gt_env)
    (array.new_fixed $zhex_arr_ref 1 (struct.new $zhex_some_i64 (i64.const 3)))))
(func $t_clo_up (result (ref $zhex_closure))
  (struct.new $zhex_closure (ref.func $t_up_ref) (array.new_fixed $zhex_arr_ref 0)))
(func $t_clo_id (result (ref $zhex_closure))
  (struct.new $zhex_closure (ref.func $t_id_ref) (array.new_fixed $zhex_arr_ref 0)))
(func $t_clo_zero (result (ref $zhex_closure))
  (struct.new $zhex_closure (ref.func $t_zero) (array.new_fixed $zhex_arr_ref 0)))
(func $t_clo_len_key (result (ref $zhex_closure))
  (struct.new $zhex_closure (ref.func $t_len_key) (array.new_fixed $zhex_arr_ref 0)))
(func $t_clo_lower_key (result (ref $zhex_closure))
  (struct.new $zhex_closure (ref.func $t_lower_key) (array.new_fixed $zhex_arr_ref 0)))
(func $t_clo_row_gt5 (result (ref $zhex_closure))
  (struct.new $zhex_closure (ref.func $t_row_gt5) (array.new_fixed $zhex_arr_ref 0)))
(func $t_row_append_all (param $clo (ref $zhex_closure)) (param $row (ref null eq)) (result i32)
  (drop (call $zhex_list_append_i64 (ref.cast (ref $zhex_list_i64) (local.get $row)) (i64.const 99)))
  (i32.const 1))
(func $t_clo_row_keep (result (ref $zhex_closure))
  (struct.new $zhex_closure (ref.func $t_row_keep_append) (array.new_fixed $zhex_arr_ref 0)))
(func $t_clo_row_append_all (result (ref $zhex_closure))
  (struct.new $zhex_closure (ref.func $t_row_append_all) (array.new_fixed $zhex_arr_ref 0)))

(func $t_l123 (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $zhex_list_new_i64))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 2)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 3)))
  (local.get $l))
(func $t_l142 (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $zhex_list_new_i64))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 4)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 2)))
  (local.get $l))
(func $t_l312 (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $zhex_list_new_i64))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 3)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 2)))
  (local.get $l))
(func $t_l_stable (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $zhex_list_new_i64))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 100)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 101)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 102)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 103)))
  (local.get $l))
(func $t_l_l16 (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $zhex_list_new_i64))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 1)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 2)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 3)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 4)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 5)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 6)))
  (local.get $l))
(func $t_l1str (param $s (ref $zhex_string)) (result (ref $zhex_list_ref))
  (local $l (ref $zhex_list_ref))
  (local.set $l (call $zhex_list_new_ref))
  (drop (call $zhex_list_append_ref (local.get $l) (local.get $s)))
  (local.get $l))
(func $t_lstr_sort (result (ref $zhex_list_ref))
  (local $l (ref $zhex_list_ref))
  (local.set $l (call $zhex_list_new_ref))
  (drop (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 6 (i32.const 233) (i32.const 99) (i32.const 108) (i32.const 97) (i32.const 105) (i32.const 114))))
  (drop (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 5 (i32.const 380) (i32.const 101) (i32.const 98) (i32.const 114) (i32.const 97))))
  (drop (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 1 (i32.const 128512))))
  (drop (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 6 (i32.const 201) (i32.const 99) (i32.const 108) (i32.const 97) (i32.const 105) (i32.const 114))))
  (local.get $l))
(func $t_lstr_len (result (ref $zhex_list_ref))
  (local $l (ref $zhex_list_ref))
  (local.set $l (call $zhex_list_new_ref))
  (drop (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 3 (i32.const 99) (i32.const 99) (i32.const 99))))
  (drop (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 1 (i32.const 97))))
  (drop (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 2 (i32.const 98) (i32.const 98))))
  (local.get $l))
(func $t_lstr_case (result (ref $zhex_list_ref))
  (local $l (ref $zhex_list_ref))
  (local.set $l (call $zhex_list_new_ref))
  (drop (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 1 (i32.const 66))))
  (drop (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 1 (i32.const 97))))
  (local.get $l))

(func (export "t_map_cb_at") (param $i i64) (result i64)
  (call $zhex_list_get_i64 (call $zhex_list_map_i64_i64 (call $t_l123) (call $t_clo_inc)) (local.get $i)))
(func (export "t_map_cb_str") (param $len i32) (result i32)
  (call $zhex_stage_utf8
    (ref.cast (ref $zhex_string)
      (call $zhex_list_get_ref
        (call $zhex_list_map_ref_ref
          (call $t_l1str (call $t_from_utf8_at (i32.const 0) (local.get $len)))
          (call $t_clo_up))
        (i64.const 0)))))
(func (export "t_map_cb_append_src")
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $t_l123))
  (drop (call $zhex_list_map_i64_i64 (local.get $l)
    (struct.new $zhex_closure (ref.func $t_append_src)
      (array.new_fixed $zhex_arr_ref 1 (local.get $l))))))
(func (export "t_filter_len") (result i64)
  (call $zhex_list_length_i64 (call $zhex_list_filter_i64 (call $t_l_l16) (call $t_clo_gt3))))
(func (export "t_filter_at") (param $i i64) (result i64)
  (call $zhex_list_get_i64 (call $zhex_list_filter_i64 (call $t_l_l16) (call $t_clo_gt3)) (local.get $i)))
(func (export "t_select_len") (result i64)
  (call $zhex_list_length_i64 (call $zhex_list_select_i64 (call $t_l_l16) (call $t_clo_gt3))))
(func (export "t_fold_sum") (result i64)
  (call $zhex_list_fold_i64_i64 (call $t_l123) (i64.const 0) (call $t_clo_add2)))
(func (export "t_fold_empty_init") (result i64)
  (call $zhex_list_fold_i64_i64 (call $zhex_list_new_i64) (i64.const 42) (call $t_clo_add2)))
(func (export "t_find_some_val") (result i64)
  (struct.get $zhex_some_i64 $value
    (ref.cast (ref $zhex_some_i64) (call $zhex_list_find_i64 (call $t_l142) (call $t_clo_gt3)))))
(func (export "t_find_none_isnone") (result i32)
  (ref.test (ref $zhex_none) (call $zhex_list_find_i64 (call $t_l123) (call $t_clo_gt3))))
(func (export "t_any_true") (result i32)
  (call $zhex_list_any_i64 (call $t_l142) (call $t_clo_gt3)))
(func (export "t_any_empty") (result i32)
  (call $zhex_list_any_i64 (call $zhex_list_new_i64) (call $t_clo_gt3)))
(func (export "t_all_empty") (result i32)
  (call $zhex_list_all_i64 (call $zhex_list_new_i64) (call $t_clo_gt3)))
(func (export "t_all_false") (result i32)
  (call $zhex_list_all_i64 (call $t_l123) (call $t_clo_gt3)))
(func (export "t_find_then_append_len") (result i64)
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $t_l142))
  (drop (call $zhex_list_find_i64 (local.get $l) (call $t_clo_gt3)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 9)))
  (call $zhex_list_length_i64 (local.get $l)))
(func (export "t_any_then_append_len") (result i64)
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $t_l142))
  (drop (call $zhex_list_any_i64 (local.get $l) (call $t_clo_gt3)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 9)))
  (call $zhex_list_length_i64 (local.get $l)))
(func (export "t_all_then_append_len") (result i64)
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $t_l142))
  (drop (call $zhex_list_all_i64 (local.get $l) (call $t_clo_gt3)))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 9)))
  (call $zhex_list_length_i64 (local.get $l)))
(func (export "t_sort_at") (param $i i64) (result i64)
  (call $zhex_list_get_i64 (call $zhex_list_sort_i64 (call $t_l312)) (local.get $i)))
(func (export "t_sort_src_first") (result i64)
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $t_l312))
  (drop (call $zhex_list_sort_i64 (local.get $l)))
  (call $zhex_list_get_i64 (local.get $l) (i64.const 0)))
(func (export "t_sort_str_at") (param $i i64) (result i32)
  (call $zhex_stage_utf8
    (ref.cast (ref $zhex_string)
      (call $zhex_list_get_ref (call $zhex_list_sort_ref (call $t_lstr_sort)) (local.get $i)))))
(func (export "t_sort_by_stable_at") (param $i i64) (result i64)
  (call $zhex_list_get_i64 (call $zhex_list_sort_by_i64_i64 (call $t_l_stable) (call $t_clo_zero)) (local.get $i)))
(func (export "t_sort_by_len_at") (param $i i64) (result i32)
  (call $zhex_stage_utf8
    (ref.cast (ref $zhex_string)
      (call $zhex_list_get_ref (call $zhex_list_sort_by_ref_i64 (call $t_lstr_len) (call $t_clo_len_key)) (local.get $i)))))
(func (export "t_sort_by_strkey_at") (param $i i64) (result i32)
  (call $zhex_stage_utf8
    (ref.cast (ref $zhex_string)
      (call $zhex_list_get_ref (call $zhex_list_sort_by_ref_ref (call $t_lstr_case) (call $t_clo_lower_key)) (local.get $i)))))
(func (export "t_sort_by_src_first") (result i64)
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $t_l_stable))
  (drop (call $zhex_list_sort_by_i64_i64 (local.get $l) (call $t_clo_zero)))
  (call $zhex_list_get_i64 (local.get $l) (i64.const 0)))
(func (export "t_map_fresh_outer") (result i64)
  (local $src (ref $zhex_list_i64))
  (local $res (ref $zhex_list_i64))
  (local.set $src (call $t_l123))
  (local.set $res (call $zhex_list_map_i64_i64 (local.get $src) (call $t_clo_inc)))
  (drop (call $zhex_list_append_i64 (local.get $src) (i64.const 9)))
  (drop (call $zhex_list_append_i64 (local.get $res) (i64.const 9)))
  (call $zhex_list_length_i64 (local.get $res)))
(func (export "t_map_src_first_after") (result i64)
  (local $src (ref $zhex_list_i64))
  (local.set $src (call $t_l123))
  (drop (call $zhex_list_map_i64_i64 (local.get $src) (call $t_clo_inc)))
  (call $zhex_list_get_i64 (local.get $src) (i64.const 0)))
(func (export "t_map_shared_elem") (result i32)
  (local $src (ref $zhex_list_ref))
  (local $res (ref $zhex_list_ref))
  (local.set $src (call $t_l1str (array.new_fixed $zhex_string 1 (i32.const 113))))
  (local.set $res (call $zhex_list_map_ref_ref (local.get $src) (call $t_clo_id)))
  (ref.eq (call $zhex_list_get_ref (local.get $src) (i64.const 0))
          (call $zhex_list_get_ref (local.get $res) (i64.const 0))))
(func (export "t_clo_reuse") (result i64)
  (local $c (ref null $zhex_closure))
  (local $l (ref $zhex_list_i64))
  (local.set $c (call $t_clo_inc))
  (drop (call $zhex_list_map_i64_i64 (call $t_l123) (ref.as_non_null (local.get $c))))
  (local.set $l (call $zhex_list_new_i64))
  (drop (call $zhex_list_append_i64 (local.get $l) (i64.const 10)))
  (call $zhex_list_get_i64 (call $zhex_list_map_i64_i64 (local.get $l) (ref.as_non_null (local.get $c))) (i64.const 0)))
(func $t_arr23 (result (ref $zhex_array_i64))
  (local $sh (ref $zhex_list_i64))
  (local $a (ref $zhex_array_i64))
  (local.set $sh (call $zhex_list_new_i64))
  (drop (call $zhex_list_append_i64 (local.get $sh) (i64.const 2)))
  (drop (call $zhex_list_append_i64 (local.get $sh) (i64.const 3)))
  (local.set $a (call $zhex_array_new_i64 (local.get $sh) (i64.const 0)))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 0) (i64.const 0)) (i64.const 1))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 0) (i64.const 1)) (i64.const 2))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 0) (i64.const 2)) (i64.const 3))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 1) (i64.const 0)) (i64.const 7))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 1) (i64.const 1)) (i64.const 8))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 1) (i64.const 2)) (i64.const 9))
  (local.get $a))
(func $t_arr222 (result (ref $zhex_array_i64))
  (local $sh (ref $zhex_list_i64))
  (local.set $sh (call $zhex_list_new_i64))
  (drop (call $zhex_list_append_i64 (local.get $sh) (i64.const 2)))
  (drop (call $zhex_list_append_i64 (local.get $sh) (i64.const 2)))
  (drop (call $zhex_list_append_i64 (local.get $sh) (i64.const 2)))
  (call $zhex_array_new_i64 (local.get $sh) (i64.const 0)))
(func (export "t_arrsel_len") (result i64)
  (call $zhex_array_length_i64 (call $zhex_array_select_i64 (call $t_arr23) (call $t_clo_row_gt5))))
(func (export "t_arrsel_rows") (result i64)
  (array.get $zhex_arr_i64 (struct.get $zhex_array_i64 $shape (call $zhex_array_select_i64 (call $t_arr23) (call $t_clo_row_gt5))) (i32.const 0)))
(func (export "t_arrsel_width") (result i64)
  (array.get $zhex_arr_i64 (struct.get $zhex_array_i64 $shape (call $zhex_array_select_i64 (call $t_arr23) (call $t_clo_row_gt5))) (i32.const 1)))
(func (export "t_arrsel_at") (param $i i64) (result i64)
  (call $zhex_array_get_i64 (call $zhex_array_select_i64 (call $t_arr23) (call $t_clo_row_gt5))
    (array.new_fixed $zhex_arr_i64 2 (i64.const 0) (local.get $i))))
(func (export "t_arrsel_rowcopy_len") (result i64)
  (call $zhex_array_length_i64 (call $zhex_array_select_i64 (call $t_arr23) (call $t_clo_row_keep))))
(func (export "t_arrsel_rowcopy_at") (param $i i64) (result i64)
  (call $zhex_array_get_i64 (call $zhex_array_select_i64 (call $t_arr23) (call $t_clo_row_keep))
    (array.new_fixed $zhex_arr_i64 2 (i64.const 0) (local.get $i))))
(func (export "t_arrsel_src_len") (result i64)
  (local $a (ref $zhex_array_i64))
  (local.set $a (call $t_arr23))
  (drop (call $zhex_array_select_i64 (local.get $a) (call $t_clo_row_keep)))
  (call $zhex_array_length_i64 (local.get $a)))
(func (export "t_arrsel_src_first") (result i64)
  (local $a (ref $zhex_array_i64))
  (local.set $a (call $t_arr23))
  (drop (call $zhex_array_select_i64 (local.get $a) (call $t_clo_row_keep)))
  (call $zhex_array_get_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 0) (i64.const 0))))
(func $t_arr33 (result (ref $zhex_array_i64))
  (local $sh (ref $zhex_list_i64))
  (local $a (ref $zhex_array_i64))
  (local.set $sh (call $zhex_list_new_i64))
  (drop (call $zhex_list_append_i64 (local.get $sh) (i64.const 3)))
  (drop (call $zhex_list_append_i64 (local.get $sh) (i64.const 3)))
  (local.set $a (call $zhex_array_new_i64 (local.get $sh) (i64.const 0)))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 0) (i64.const 0)) (i64.const 6))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 0) (i64.const 1)) (i64.const 7))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 0) (i64.const 2)) (i64.const 8))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 1) (i64.const 0)) (i64.const 9))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 1) (i64.const 1)) (i64.const 10))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 1) (i64.const 2)) (i64.const 11))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 2) (i64.const 0)) (i64.const 12))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 2) (i64.const 1)) (i64.const 13))
  (call $zhex_array_set_i64 (local.get $a) (array.new_fixed $zhex_arr_i64 2 (i64.const 2) (i64.const 2)) (i64.const 14))
  (local.get $a))
(func $t_arrsel33 (result (ref $zhex_array_i64))
  (call $zhex_array_select_i64 (call $t_arr33) (call $t_clo_row_append_all)))
(func (export "t_arrsel33_len") (result i64)
  (call $zhex_array_length_i64 (call $t_arrsel33)))
(func (export "t_arrsel33_rows") (result i64)
  (array.get $zhex_arr_i64 (struct.get $zhex_array_i64 $shape (call $t_arrsel33)) (i32.const 0)))
(func (export "t_arrsel33_width") (result i64)
  (array.get $zhex_arr_i64 (struct.get $zhex_array_i64 $shape (call $t_arrsel33)) (i32.const 1)))
(func (export "t_arrsel33_at") (param $r i64) (param $c i64) (result i64)
  (call $zhex_array_get_i64 (call $t_arrsel33)
    (array.new_fixed $zhex_arr_i64 2 (local.get $r) (local.get $c))))
(func (export "t_arrsel33_src_len") (result i64)
  (local $a (ref $zhex_array_i64))
  (local.set $a (call $t_arr33))
  (drop (call $zhex_array_select_i64 (local.get $a) (call $t_clo_row_append_all)))
  (call $zhex_array_length_i64 (local.get $a)))
(func (export "t_arrsel_rank3")
  (drop (call $zhex_array_select_i64 (call $t_arr222) (call $t_clo_row_gt5))))

(elem declare func $t_inc $t_zero $t_add2 $t_gt_env $t_up_ref $t_id_ref $t_len_key $t_lower_key $t_row_gt5 $t_row_keep_append $t_row_append_all $t_append_src)

(export "memory" (memory $zhex_mem))
(export "zhex_last_error" (func $zhex_last_error))
`

// watPreludeM2Runner drives the assembled M2 prelude under node, one fresh
// instance per case, in the shape of watprelude_test.go's runner. The
// CASE_BOUNDARIES array (span-boundary code points with this process's
// expected mappings) is generated from the live unicode package when the
// runner is written, keeping the walk self-consistent.
const watM2RunnerSrc = `
import { readFileSync } from 'node:fs';

const bytes = readFileSync(process.argv[2]);
const mod = new WebAssembly.Module(bytes);
let exports = null;
let printed = null;

function fresh() {
  printed = [];
  const env = {
    print(ptr, len) {
      printed.push(new TextDecoder().decode(new Uint8Array(exports.memory.buffer, ptr, len)));
    },
  };
  exports = new WebAssembly.Instance(mod, { env }).exports;
}

const cases = [];
const t = (name, fn) => cases.push({ name, fn });
const eq = (got, want, what) => {
  if (got !== want) throw new Error(what + ': got ' + got + ', want ' + want);
};
const enc = new TextEncoder();
const put = (off, s) => {
  const bytes = enc.encode(s);
  while (exports.memory.buffer.byteLength < off + bytes.length) exports.memory.grow(1);
  new Uint8Array(exports.memory.buffer, off).set(bytes);
};
const outStr = (len) => new TextDecoder().decode(new Uint8Array(exports.memory.buffer, 0, len));
const str = (op, s, ...rest) => {
  put(0, s);
  return outStr(exports[op](enc.encode(s).length, ...rest));
};
const str2 = (op, a, b) => {
  put(0, a);
  put(4096, b);
  return outStr(exports[op](enc.encode(a).length, enc.encode(b).length));
};
function trapMsg(fn, code, wantMsg) {
  try { fn(); } catch (e) {
    if (!(e instanceof WebAssembly.RuntimeError)) throw new Error('expected RuntimeError, got: ' + e);
    const got = Number(exports.zhex_last_error());
    if (got !== code) throw new Error('trap code ' + got + ', want ' + code + ' (' + e.message + ')');
    const mlen = Number(exports.zhex_msg_len());
    const msg = mlen ? outStr(mlen) : null;
    if (msg !== wantMsg) throw new Error('staged message ' + JSON.stringify(msg) + ', want ' + JSON.stringify(wantMsg));
    return;
  }
  throw new Error('expected a trap, none happened');
}
const trap = (fn, code) => trapMsg(fn, code, null);

const MIN = -9223372036854775808n;
const MAX = 9223372036854775807n;
@@CASES@@

// ---- text.* ------------------------------------------------------------
t('reverse keeps combining mark as its own code point', () => {
  fresh();
  eq(str('t_reverse', 'e\u0301ab'), 'ba\u0301e', 'reverse');
  eq(str('t_reverse', '\ud83d\ude00x'), 'x\ud83d\ude00', 'emoji reverse');
});
t('slice clamps and multibyte', () => {
  fresh();
  eq(str('t_slice', 'he\u0301llo', 0n, 99n), 'he\u0301llo', 'end clamp');
  eq(str('t_slice', 'he\u0301llo', -2n, 99n), 'he\u0301llo', 'start clamp');
  eq(str('t_slice', 'he\u0301llo', 1n, 3n), 'e\u0301', 'cp slice');
  eq(str('t_slice', 'he\u0301llo', 3n, 3n), '', 'empty');
  eq(str('t_slice', 'he\u0301llo', 4n, 2n), '', 'start>=end');
  eq(str('t_slice', '\ud83d\ude00x', 0n, 1n), '\ud83d\ude00', 'emoji one cp');
});
for (const [a, b, want] of [
  ['he\u0301llo', 'llo', 3n], ['he\u0301llo', 'z', -1n], ['banana', 'na', 2n],
  ['abc', 'abc', 0n], ['abc', 'abcd', -1n], ['abc', '', 0n],
]) {
  t('index-of ' + JSON.stringify([a, b]), () => {
    fresh(); put(0, a); put(4096, b);
    eq(exports.t_index_of(enc.encode(a).length, enc.encode(b).length), want, 'index-of');
  });
}
for (const [a, b, want] of [
  ['he\u0301llo', 'e\u0301', 1], ['abc', '', 1], ['', '', 1], ['abc', 'z', 0], ['abc', 'abcd', 0],
]) {
  t('contains ' + JSON.stringify([a, b]), () => {
    fresh(); put(0, a); put(4096, b);
    eq(exports.t_contains(enc.encode(a).length, enc.encode(b).length), want, 'contains');
  });
}
for (const [a, b, want] of [
  ['he\u0301llo', 'he\u0301', 1], ['abc', '', 1], ['abc', 'abcd', 0], ['a', 'a', 1],
]) {
  t('starts/ends ' + JSON.stringify([a, b]), () => {
    fresh(); put(0, a); put(4096, b);
    eq(exports.t_starts_with(enc.encode(a).length, enc.encode(b).length), want, 'starts');
    const ra = [...a].reverse().join(''), rb = [...b].reverse().join('');
    put(0, ra); put(4096, rb);
    eq(exports.t_ends_with(enc.encode(ra).length, enc.encode(rb).length), want, 'ends');
  });
}
t('trim NBSP and the full space set', () => {
  fresh();
  eq(str('t_trim', '\u00a0\u000b he\u0301llo \t\u3000'), 'he\u0301llo', 'trim');
  eq(str('t_trim', ' \t\r\n'), '', 'all space');
  eq(str('t_trim', 'x'), 'x', 'none');
});
for (const [s, want] of [
  ['', 0n], ['one', 1n], ['a\nb', 2n], ['a\nb\n', 2n], ['\n\n\n', 3n], ['\n\nx', 3n],
]) {
  t('line-count ' + JSON.stringify(s), () => {
    fresh(); put(0, s); eq(exports.t_line_count(enc.encode(s).length), want, 'line-count');
  });
}
for (const [s, want] of [
  ['', 0n], ['one', 1n], ['  one two  ', 2n], ['a\u00a0\u2028b\tc', 3n], ['\t\n ', 0n],
]) {
  t('word-count ' + JSON.stringify(s), () => {
    fresh(); put(0, s); eq(exports.t_word_count(enc.encode(s).length), want, 'word-count');
  });
}
t('upper/lower classics', () => {
  fresh();
  eq(str('t_upper', 'e\u0301a'), 'E\u0301A', 'combining stays separate');
  eq(str('t_upper', '\u00e9\u00e0'), '\u00c9\u00c0', 'latin-1');
  eq(str('t_upper', '\u00df'), '\u00df', 'sharp s stays');
  eq(str('t_upper', '\u017f'), 'S', 'long s maps');
  eq(str('t_upper', '\u03c3'), '\u03a3', 'sigma');
  eq(str('t_lower', '\u03a3'), '\u03c3', 'final sigma always simple');
  eq(str('t_lower', '\u00c9\u00c0'), '\u00e9\u00e0', 'lower latin-1');
  eq(str('t_lower', '\ud83d\ude00'), '\ud83d\ude00', 'emoji unchanged');
});
t('case table boundary walk', () => {
  fresh();
  for (const [cp, up, lo] of CASE_BOUNDARIES) {
    eq(exports.t_upper_cp(cp), up, 'upper cp ' + cp);
    eq(exports.t_lower_cp(cp), lo, 'lower cp ' + cp);
  }
});
t('str_lt code-point order', () => {
  fresh();
  for (const [a, b, want] of [
    ['a', 'b', 1], ['b', 'a', 0], ['a', 'a', 0], ['ab', 'a', 0], ['a', 'ab', 1],
    ['\u00c9', '\u00e9', 1], ['\u017c', '\ud83d\ude00', 1], ['', 'a', 1], ['', '', 0],
  ]) {
    put(0, a); put(4096, b);
    eq(exports.t_str_lt(enc.encode(a).length, enc.encode(b).length), want, 'lt ' + JSON.stringify([a, b]));
  }
});
t('split edges', () => {
  fresh();
  put(0, 'a,b,,c,'); put(4096, ',');
  eq(exports.t_split_count(7, 1), 5n, 'leading/repeated/trailing empties');
  eq(str2('t_split_join', 'a,b,,c,', ','), 'a,b,,c,', 'join round trip');
  put(0, 'he\u0301llo'); put(4096, '');
  eq(exports.t_split_count(7, 0), 6n, 'empty sep is cp split');
  eq(str2('t_split_join', 'he\u0301llo', ''), 'h,e,\u0301,l,l,o', 'cp parts');
  put(0, ''); put(4096, ',');
  eq(exports.t_split_count(0, 1), 1n, 'empty text nonempty sep -> one empty field');
  eq(str2('t_split_join', '', ','), '', 'empty join');
  put(0, ''); put(4096, '');
  eq(exports.t_split_count(0, 0), 0n, 'empty text empty sep -> empty list');
  put(0, 'ae\u0301a'); put(4096, 'e\u0301');
  eq(exports.t_split_count(4, 3), 2n, 'multibyte separator');
  eq(str2('t_split_join', 'ae\u0301a', 'e\u0301'), 'a,a', 'multibyte separator join');
});
t('join keeps empty fields', () => {
  fresh();
  eq(str2('t_split_join', 'a::b', ':'), 'a,,b', 'empty middle field');
});
t('replace literal', () => {
  fresh();
  put(0, 'one two two'); put(4096, 'two'); put(8192, 'X');
  eq(outStr(exports.t_replace(11, 3, 1)), 'one X X', 'replace all');
  put(0, 'aaa'); put(4096, 'aa'); put(8192, 'b');
  eq(outStr(exports.t_replace(3, 2, 1)), 'ba', 'non-overlapping');
  put(0, 'axa'); put(4096, 'a'); put(8192, 'aa');
  eq(outStr(exports.t_replace(3, 1, 2)), 'aaxaa', 'replacement not rescanned');
  put(0, 'ab'); put(4096, ''); put(8192, 'z');
  trapMsg(() => exports.t_replace(2, 0, 1), 13, null);
  eq(Number(exports.zhex_msg_len()), 0, 'fixed-code fail stages no message');
});
t('split over limit fails 14', () => {
  fresh();
  const big = 'a'.repeat(1000001);
  put(0, big);
  trap(() => exports.t_split_count(big.length, 0), 14);
  eq(Number(exports.zhex_msg_len()), 0, 'no staged message for 14');
});
t('split exactly at limit is allowed', () => {
  fresh();
  const big = 'a'.repeat(1000000);
  put(0, big);
  eq(exports.t_split_count(big.length, 0), 1000000n, 'cp count');
});

// ---- text.to-int ------------------------------------------------------
for (const [s, want] of [
  ['  12  ', 12n], ['+7', 7n], ['-7', -7n], ['007', 7n], ['0', 0n], ['-0', 0n],
  ['9223372036854775807', MAX], ['-9223372036854775808', MIN],
  [' \t+42\n', 42n], ['0000000000000000000005', 5n],
]) {
  t('to-int ok ' + JSON.stringify(s), () => {
    fresh();
    const len = enc.encode(s).length;
    put(0, s);
    eq(exports.t_to_int_tag(len), 0, 'tag');
    eq(exports.t_to_int_val(len), want, 'value');
  });
}
for (const [s, msg] of [
  ['', '"" is not a signed 64-bit decimal integer'],
  ['x', '"x" is not a signed 64-bit decimal integer'],
  ['  1 x  ', '"  1 x  " is not a signed 64-bit decimal integer'],
  ['9223372036854775808', '"9223372036854775808" is not a signed 64-bit decimal integer'],
  ['-9223372036854775809', '"-9223372036854775809" is not a signed 64-bit decimal integer'],
  ['1_2', '"1_2" is not a signed 64-bit decimal integer'],
  ['0x1', '"0x1" is not a signed 64-bit decimal integer'],
  ['+', '"+" is not a signed 64-bit decimal integer'],
	['\t"q"\n', '"\\t\\"q\\"\\n" is not a signed 64-bit decimal integer'],
	['a\x01\x7fb', '"a\\x01\\x7fb" is not a signed 64-bit decimal integer'],
	['\u00e9', '"\u00e9" is not a signed 64-bit decimal integer'],
	// DESIGN-M4 §5: quote matches Go %q for non-ASCII — NBSP is in the trim
	// set but the message quotes the UNTRIMMED input, so prefix garbage
	['\u00a0x', '"\\u00a0x" is not a signed 64-bit decimal integer'],
	['x\u00a0', '"x\\u00a0" is not a signed 64-bit decimal integer'],
	['\u2028', '"\\u2028" is not a signed 64-bit decimal integer'],
	['\u2029', '"\\u2029" is not a signed 64-bit decimal integer'],
	['\ufffe', '"\\ufffe" is not a signed 64-bit decimal integer'],
	['\ud83d\ude00', '"\ud83d\ude00" is not a signed 64-bit decimal integer'],
	[String.fromCodePoint(0xE0001), '"\\U000e0001" is not a signed 64-bit decimal integer'],
	['\ufffd', '"\ufffd" is not a signed 64-bit decimal integer'],
]) {
  t('to-int fail ' + JSON.stringify(s), () => {
    fresh();
    const len = enc.encode(s).length;
    put(0, s);
    eq(exports.t_to_int_tag(len), 1, 'tag');
    eq(exports.t_err_family(len), 1, 'ParseError family');
    eq(outStr(exports.t_to_int_msg(len)), msg, 'message');
  });
}

// ---- maps ---------------------------------------------------------------
t('map insertion order: overwrite keeps position, remove shifts, re-put appends', () => {
  fresh();
  eq(exports.t_map_len(), 4n, 'len');
  for (let i = 0; i < 4; i++) {
    eq(outStr(exports.t_map_key_at(BigInt(i))), ['b', 'c', 'a', 'd'][i], 'key ' + i);
    eq(exports.t_map_val_at(BigInt(i)), [9n, 3n, 7n, 4n][i], 'val ' + i);
  }
});
t('map contains', () => {
  fresh();
  eq(exports.t_map_contains_yes(), 1, 'present');
  eq(exports.t_map_contains_no(), 0, 'absent');
});
t('map lookup Option', () => {
  fresh();
  eq(exports.t_map_lookup_some_val(), 9n, 'some payload');
  eq(exports.t_map_lookup_absent_none(), 1, 'absent is None');
  eq(exports.t_map_lookup_some_test(), 1, 'present tests Some');
});
t('map remove of a missing key is a no-op', () => {
  fresh();
  eq(exports.t_map_remove_missing_len(), 4n, 'len');
});
t('map guard: put while iterating traps 10', () => {
  fresh(); trap(() => exports.t_map_guard_put(), 10);
});
t('map guard: removing a missing key while iterating still traps 10 (guard before op)', () => {
  fresh(); trap(() => exports.t_map_guard_remove_noop(), 10);
});
t('map guard: into a guarded target traps 10', () => {
  fresh(); trap(() => exports.t_map_guard_into(), 10);
});
t('map lookup/contains/length while iterating are fine', () => {
  fresh();
  eq(exports.t_map_lookup_active(), 9n, 'lookup');
  eq(exports.t_map_contains_active(), 1, 'contains');
  eq(exports.t_map_len_active(), 4n, 'length');
});
t('map aliases observe one map', () => {
  fresh();
  eq(exports.t_map_alias_len(), 5n, 'len via original');
});
t('map i64 keys lookup', () => {
  fresh();
  eq(exports.t_map_i64_lookup(), 200n, 'val');
});
t('map i32 (Bool) keys lookup', () => {
  fresh();
  eq(exports.t_map_i32_lookup(), 7n, 'val');
});
t('map str->str lookup stages the value', () => {
  fresh();
  eq(outStr(exports.t_map_strref_lookup()), 'v', 'val');
});
t('map into merges in source order (overwrite in place, append new)', () => {
  fresh();
  eq(exports.t_map_into_len(), 3n, 'len');
  eq(outStr(exports.t_map_into_key_at(0n)), 'y', 'key 0');
  eq(outStr(exports.t_map_into_key_at(1n)), 'z', 'key 1');
  eq(outStr(exports.t_map_into_key_at(2n)), 'x', 'key 2');
  eq(exports.t_map_into_val_at(0n), 2n, 'y overwritten with source value');
});
t('map into self is a no-op', () => {
  fresh();
  eq(exports.t_map_into_self_len(), 2n, 'len');
});
t('map into with an iterated source is legal', () => {
  fresh();
  eq(exports.t_map_into_src_active_len(), 3n, 'len');
});

// ---- callbacks ----------------------------------------------------------
t('list.map Int callback', () => {
  fresh();
  eq(exports.t_map_cb_at(0n), 2n, 'at 0');
  eq(exports.t_map_cb_at(1n), 3n, 'at 1');
  eq(exports.t_map_cb_at(2n), 4n, 'at 2');
});
t('list.map String callback upper-cases', () => {
  fresh();
  put(0, 'he\u0301llo');
  eq(outStr(exports.t_map_cb_str(enc.encode('he\u0301llo').length)), 'HE\u0301LLO', 'upper');
});
t('list.map callback appending the source traps 10', () => {
  fresh(); trap(() => exports.t_map_cb_append_src(), 10);
});
t('list.filter keeps matching elements', () => {
  fresh();
  eq(exports.t_filter_len(), 3n, 'len');
  eq(exports.t_filter_at(0n), 4n, 'at 0');
  eq(exports.t_filter_at(1n), 5n, 'at 1');
  eq(exports.t_filter_at(2n), 6n, 'at 2');
});
t('list.select matches list.filter', () => {
  fresh();
  eq(exports.t_select_len(), 3n, 'len');
});
t('list.fold sums; empty returns init', () => {
  fresh();
  eq(exports.t_fold_sum(), 6n, 'sum');
  eq(exports.t_fold_empty_init(), 42n, 'empty');
});
t('list.find Some payload / None variant', () => {
  fresh();
  eq(exports.t_find_some_val(), 4n, 'some');
  eq(exports.t_find_none_isnone(), 1, 'none');
});
t('list.any/list.all short-circuit semantics', () => {
  fresh();
  eq(exports.t_any_true(), 1, 'any true');
  eq(exports.t_any_empty(), 0, 'any empty false');
  eq(exports.t_all_empty(), 1, 'all empty true');
  eq(exports.t_all_false(), 0, 'all false');
});
t('find/any/all early exits restore the traversal counter', () => {
  fresh();
  eq(exports.t_find_then_append_len(), 4n, 'find then append');
  eq(exports.t_any_then_append_len(), 4n, 'any then append');
  eq(exports.t_all_then_append_len(), 4n, 'all then append');
});
t('list.sort Int ascending, source unchanged', () => {
  fresh();
  eq(exports.t_sort_at(0n), 1n, 'at 0');
  eq(exports.t_sort_at(1n), 2n, 'at 1');
  eq(exports.t_sort_at(2n), 3n, 'at 2');
  eq(exports.t_sort_src_first(), 3n, 'source unchanged');
});
t('list.sort String code-point order', () => {
  fresh();
  const want = ['\u00c9clair', '\u00e9clair', '\u017cebra', '\ud83d\ude00'];
  for (let i = 0; i < 4; i++) eq(outStr(exports.t_sort_str_at(BigInt(i))), want[i], 'at ' + i);
});
t('list.sort-by stable on ties', () => {
  fresh();
  for (let i = 0n; i < 4n; i++) eq(exports.t_sort_by_stable_at(i), 100n + i, 'stable at ' + i);
});
t('list.sort-by Int key (length)', () => {
  fresh();
  const want = ['a', 'bb', 'ccc'];
  for (let i = 0; i < 3; i++) eq(outStr(exports.t_sort_by_len_at(BigInt(i))), want[i], 'at ' + i);
});
t('list.sort-by String key ranks by key, source unchanged', () => {
  fresh();
  eq(outStr(exports.t_sort_by_strkey_at(0n)), 'a', 'lower key first');
  eq(outStr(exports.t_sort_by_strkey_at(1n)), 'B', 'original text kept');
  eq(exports.t_sort_by_src_first(), 100n, 'source unchanged');
});
t('list.map returns a fresh outer list, source unmodified, elements shared', () => {
  fresh();
  eq(exports.t_map_fresh_outer(), 4n, 'result grew independently');
  eq(exports.t_map_src_first_after(), 1n, 'source unmodified');
  eq(exports.t_map_shared_elem(), 1, 'elements shared');
});
t('closure value can be reused', () => {
  fresh();
  eq(exports.t_clo_reuse(), 11n, 'second use');
});
t('array.select keeps rows, rectangle preserved', () => {
  fresh();
  eq(exports.t_arrsel_rows(), 1n, 'rows');
  eq(exports.t_arrsel_width(), 3n, 'width');
  eq(exports.t_arrsel_len(), 3n, 'flat length');
  eq(exports.t_arrsel_at(0n), 7n, 'at 0');
  eq(exports.t_arrsel_at(1n), 8n, 'at 1');
  eq(exports.t_arrsel_at(2n), 9n, 'at 2');
});
t('array.select row copies: predicate append never widens the rectangle, source untouched', () => {
  fresh();
  eq(exports.t_arrsel_rowcopy_len(), 3n, 'result stays kept*width');
  eq(exports.t_arrsel_rowcopy_at(0n), 7n, 'at 0');
  eq(exports.t_arrsel_rowcopy_at(1n), 8n, 'at 1');
  eq(exports.t_arrsel_rowcopy_at(2n), 9n, 'at 2');
  eq(exports.t_arrsel_src_len(), 6n, 'source length unchanged');
  eq(exports.t_arrsel_src_first(), 1n, 'source row 0 untouched');
});
t('array.select append-in-predicate regression: 3 kept rows stay 3x3', () => {
  fresh();
  eq(exports.t_arrsel33_len(), 9n, 'flat length is kept*width, not the grown rows');
  eq(exports.t_arrsel33_rows(), 3n, 'rows');
  eq(exports.t_arrsel33_width(), 3n, 'width preserved');
  eq(exports.t_arrsel33_at(0n, 2n), 8n, 'row 0 tail');
  eq(exports.t_arrsel33_at(1n, 1n), 10n, 'middle of row 1');
  eq(exports.t_arrsel33_at(2n, 0n), 12n, 'row 2 head');
  eq(exports.t_arrsel33_src_len(), 9n, 'source unchanged');
});
t('array.select rank 3 traps 15 with staged message', () => {
  fresh();
  trapMsg(() => exports.t_arrsel_rank3(), 15, 'array.select requires a rank-2 array, found rank 3');
});
t('fail 12 stages the message at offset 0', () => {
  fresh();
  trapMsg(() => exports.t_fail12(), 12, '314');
});

const failures = [];
for (const c of cases) {
  try { c.fn(); } catch (e) { failures.push({ name: c.name, error: String(e && e.message || e) }); }
}
console.log(JSON.stringify({ total: cases.length, failed: failures }));
if (failures.length) process.exit(1);
`

// watM2CaseBoundariesJS regenerates the node boundary-walk data from the
// running toolchain's unicode package at test time, so the walk always
// compares the wasm tables against the same process that generated them —
// no frozen expectations left to drift across toolchains.
func watM2CaseBoundariesJS() string {
	seen := map[rune]bool{}
	var b strings.Builder
	b.WriteString("const CASE_BOUNDARIES = [")
	emit := func(r rune) {
		if r < 0 || r > 0x10FFFF || seen[r] {
			return
		}
		seen[r] = true
		fmt.Fprintf(&b, "[%d,%d,%d],", r, unicode.ToUpper(r), unicode.ToLower(r))
	}
	for _, dir := range []func(rune) rune{unicode.ToUpper, unicode.ToLower} {
		for _, s := range scanCaseSpans(dir) {
			emit(rune(s[0] - 1))
			emit(rune(s[0]))
			if s[1]-s[0] > 2 {
				emit(rune((s[0] + s[1]) / 2))
			}
			emit(rune(s[1]))
			emit(rune(s[1] + 1))
		}
	}
	b.WriteString("];\n")
	return b.String()
}

var watPreludeM2Runner = strings.Replace(watM2RunnerSrc, "@@CASES@@", watM2CaseBoundariesJS(), 1)
