package lang

import (
	"encoding/json"
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
		"(type $zhex_err (struct (field $family i32) (field $message (ref null $zhex_string))))",
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

// TestWatCaseTablesMatchGo re-derives the simple-case spans from Go's
// unicode tables and diffs them against the constants committed in
// watCaseTables — the tool-less half of the case-mapping differential (the
// node half in TestWatPreludeM2 exercises the binary search at every span
// boundary). Also asserts the sorted, non-overlapping invariant the binary
// search needs.
func TestWatCaseTablesMatchGo(t *testing.T) {
	for name, dir := range map[string]func(rune) rune{
		"upper": unicode.ToUpper, "lower": unicode.ToLower,
	} {
		want := scanCaseSpans(dir)
		got := parseCaseTable(t, name)
		if len(got) != len(want) {
			t.Fatalf("%s table: committed %d spans, Go has %d", name, len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s span %d: committed [%d %d %d], Go says [%d %d %d]",
					name, i, got[i][0], got[i][1], got[i][2], want[i][0], want[i][1], want[i][2])
			}
			if i > 0 && got[i][0] <= got[i-1][1] {
				t.Errorf("%s span %d overlaps the previous span (binary search precondition)", name, i)
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

func parseCaseTable(t *testing.T, name string) [][3]int32 {
	t.Helper()
	rest := watCaseTables
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
  (call $zhex_map_remove_str_i64 (local.get $m) (call $t_mapA))
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
  (call $zhex_map_remove_str_i64 (local.get $m) (call $t_mapZZ))
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
  (call $zhex_map_remove_str_i64 (local.get $m) (call $t_mapZZ)))
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
      (call $zhex_list_append_i64 (local.get $l) (i64.const 99))
      (return (i32.const 1))))
  (i32.const 0))
(func $t_append_src (param $clo (ref $zhex_closure)) (param $x i64) (result i64)
  (call $zhex_list_append_i64
    (ref.cast (ref $zhex_list_i64)
      (array.get $zhex_arr_ref (struct.get $zhex_closure $env (local.get $clo)) (i32.const 0)))
    (i64.const 99))
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
  (call $zhex_list_append_i64 (ref.cast (ref $zhex_list_i64) (local.get $row)) (i64.const 99))
  (i32.const 1))
(func $t_clo_row_keep (result (ref $zhex_closure))
  (struct.new $zhex_closure (ref.func $t_row_keep_append) (array.new_fixed $zhex_arr_ref 0)))
(func $t_clo_row_append_all (result (ref $zhex_closure))
  (struct.new $zhex_closure (ref.func $t_row_append_all) (array.new_fixed $zhex_arr_ref 0)))

(func $t_l123 (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $zhex_list_new_i64))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 1))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 2))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 3))
  (local.get $l))
(func $t_l142 (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $zhex_list_new_i64))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 1))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 4))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 2))
  (local.get $l))
(func $t_l312 (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $zhex_list_new_i64))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 3))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 1))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 2))
  (local.get $l))
(func $t_l_stable (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $zhex_list_new_i64))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 100))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 101))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 102))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 103))
  (local.get $l))
(func $t_l_l16 (result (ref $zhex_list_i64))
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $zhex_list_new_i64))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 1))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 2))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 3))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 4))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 5))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 6))
  (local.get $l))
(func $t_l1str (param $s (ref $zhex_string)) (result (ref $zhex_list_ref))
  (local $l (ref $zhex_list_ref))
  (local.set $l (call $zhex_list_new_ref))
  (call $zhex_list_append_ref (local.get $l) (local.get $s))
  (local.get $l))
(func $t_lstr_sort (result (ref $zhex_list_ref))
  (local $l (ref $zhex_list_ref))
  (local.set $l (call $zhex_list_new_ref))
  (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 6 (i32.const 233) (i32.const 99) (i32.const 108) (i32.const 97) (i32.const 105) (i32.const 114)))
  (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 5 (i32.const 380) (i32.const 101) (i32.const 98) (i32.const 114) (i32.const 97)))
  (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 1 (i32.const 128512)))
  (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 6 (i32.const 201) (i32.const 99) (i32.const 108) (i32.const 97) (i32.const 105) (i32.const 114)))
  (local.get $l))
(func $t_lstr_len (result (ref $zhex_list_ref))
  (local $l (ref $zhex_list_ref))
  (local.set $l (call $zhex_list_new_ref))
  (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 3 (i32.const 99) (i32.const 99) (i32.const 99)))
  (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 1 (i32.const 97)))
  (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 2 (i32.const 98) (i32.const 98)))
  (local.get $l))
(func $t_lstr_case (result (ref $zhex_list_ref))
  (local $l (ref $zhex_list_ref))
  (local.set $l (call $zhex_list_new_ref))
  (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 1 (i32.const 66)))
  (call $zhex_list_append_ref (local.get $l) (array.new_fixed $zhex_string 1 (i32.const 97)))
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
  (call $zhex_list_append_i64 (local.get $l) (i64.const 9))
  (call $zhex_list_length_i64 (local.get $l)))
(func (export "t_any_then_append_len") (result i64)
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $t_l142))
  (drop (call $zhex_list_any_i64 (local.get $l) (call $t_clo_gt3)))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 9))
  (call $zhex_list_length_i64 (local.get $l)))
(func (export "t_all_then_append_len") (result i64)
  (local $l (ref $zhex_list_i64))
  (local.set $l (call $t_l142))
  (drop (call $zhex_list_all_i64 (local.get $l) (call $t_clo_gt3)))
  (call $zhex_list_append_i64 (local.get $l) (i64.const 9))
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
  (call $zhex_list_append_i64 (local.get $src) (i64.const 9))
  (call $zhex_list_append_i64 (local.get $res) (i64.const 9))
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
  (call $zhex_list_append_i64 (local.get $l) (i64.const 10))
  (call $zhex_list_get_i64 (call $zhex_list_map_i64_i64 (local.get $l) (ref.as_non_null (local.get $c))) (i64.const 0)))
(func $t_arr23 (result (ref $zhex_array_i64))
  (local $sh (ref $zhex_list_i64))
  (local $a (ref $zhex_array_i64))
  (local.set $sh (call $zhex_list_new_i64))
  (call $zhex_list_append_i64 (local.get $sh) (i64.const 2))
  (call $zhex_list_append_i64 (local.get $sh) (i64.const 3))
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
  (call $zhex_list_append_i64 (local.get $sh) (i64.const 2))
  (call $zhex_list_append_i64 (local.get $sh) (i64.const 2))
  (call $zhex_list_append_i64 (local.get $sh) (i64.const 2))
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
  (call $zhex_list_append_i64 (local.get $sh) (i64.const 3))
  (call $zhex_list_append_i64 (local.get $sh) (i64.const 3))
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
// CASE_BOUNDARIES array (span-boundary code points with Go's expected
// mappings) is generated offline from unicode.ToUpper/ToLower.
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

const watM2CaseBoundaries = `const CASE_BOUNDARIES = [[96,96,96],[97,65,97],[109,77,109],[122,90,122],[123,123,123],[180,180,180],[181,924,181],[182,182,182],[223,223,223],[224,192,224],[235,203,235],[246,214,246],[247,247,247],[248,216,248],[251,219,251],[255,376,255],[256,256,257],[257,256,257],[258,258,259],[259,258,259],[260,260,261],[261,260,261],[262,262,263],[263,262,263],[264,264,265],[265,264,265],[266,266,267],[267,266,267],[268,268,269],[269,268,269],[270,270,271],[271,270,271],[272,272,273],[273,272,273],[274,274,275],[275,274,275],[276,276,277],[277,276,277],[278,278,279],[279,278,279],[280,280,281],[281,280,281],[282,282,283],[283,282,283],[284,284,285],[285,284,285],[286,286,287],[287,286,287],[288,288,289],[289,288,289],[290,290,291],[291,290,291],[292,292,293],[293,292,293],[294,294,295],[295,294,295],[296,296,297],[297,296,297],[298,298,299],[299,298,299],[300,300,301],[301,300,301],[302,302,303],[303,302,303],[304,304,105],[305,73,305],[306,306,307],[307,306,307],[308,308,309],[309,308,309],[310,310,311],[311,310,311],[312,312,312],[313,313,314],[314,313,314],[315,315,316],[316,315,316],[317,317,318],[318,317,318],[319,319,320],[320,319,320],[321,321,322],[322,321,322],[323,323,324],[324,323,324],[325,325,326],[326,325,326],[327,327,328],[328,327,328],[329,329,329],[330,330,331],[331,330,331],[332,332,333],[333,332,333],[334,334,335],[335,334,335],[336,336,337],[337,336,337],[338,338,339],[339,338,339],[340,340,341],[341,340,341],[342,342,343],[343,342,343],[344,344,345],[345,344,345],[346,346,347],[347,346,347],[348,348,349],[349,348,349],[350,350,351],[351,350,351],[352,352,353],[353,352,353],[354,354,355],[355,354,355],[356,356,357],[357,356,357],[358,358,359],[359,358,359],[360,360,361],[361,360,361],[362,362,363],[363,362,363],[364,364,365],[365,364,365],[366,366,367],[367,366,367],[368,368,369],[369,368,369],[370,370,371],[371,370,371],[372,372,373],[373,372,373],[374,374,375],[375,374,375],[376,376,255],[377,377,378],[378,377,378],[379,379,380],[380,379,380],[381,381,382],[382,381,382],[384,579,384],[385,385,595],[386,386,387],[387,386,387],[388,388,389],[389,388,389],[390,390,596],[391,391,392],[392,391,392],[393,393,598],[395,395,396],[396,395,396],[397,397,397],[401,401,402],[402,401,402],[403,403,608],[404,404,611],[405,502,405],[406,406,617],[408,408,409],[409,408,409],[411,42972,411],[412,412,623],[413,413,626],[414,544,414],[415,415,629],[416,416,417],[417,416,417],[418,418,419],[419,418,419],[420,420,421],[421,420,421],[422,422,640],[423,423,424],[424,423,424],[425,425,643],[428,428,429],[429,428,429],[430,430,648],[431,431,432],[432,431,432],[433,433,650],[435,435,436],[436,435,436],[437,437,438],[438,437,438],[439,439,658],[440,440,441],[441,440,441],[442,442,442],[444,444,445],[445,444,445],[446,446,446],[447,503,447],[448,448,448],[452,452,454],[453,452,454],[454,452,454],[455,455,457],[456,455,457],[457,455,457],[458,458,460],[459,458,460],[460,458,460],[461,461,462],[462,461,462],[463,463,464],[464,463,464],[465,465,466],[466,465,466],[467,467,468],[468,467,468],[469,469,470],[470,469,470],[471,471,472],[472,471,472],[473,473,474],[474,473,474],[475,475,476],[476,475,476],[477,398,477],[478,478,479],[479,478,479],[480,480,481],[481,480,481],[482,482,483],[483,482,483],[484,484,485],[485,484,485],[486,486,487],[487,486,487],[488,488,489],[489,488,489],[490,490,491],[491,490,491],[492,492,493],[493,492,493],[494,494,495],[495,494,495],[496,496,496],[497,497,499],[498,497,499],[499,497,499],[500,500,501],[501,500,501],[502,502,405],[504,504,505],[505,504,505],[506,506,507],[507,506,507],[508,508,509],[509,508,509],[510,510,511],[511,510,511],[512,512,513],[513,512,513],[514,514,515],[515,514,515],[516,516,517],[517,516,517],[518,518,519],[519,518,519],[520,520,521],[521,520,521],[522,522,523],[523,522,523],[524,524,525],[525,524,525],[526,526,527],[527,526,527],[528,528,529],[529,528,529],[530,530,531],[531,530,531],[532,532,533],[533,532,533],[534,534,535],[535,534,535],[536,536,537],[537,536,537],[538,538,539],[539,538,539],[540,540,541],[541,540,541],[542,542,543],[543,542,543],[544,544,414],[546,546,547],[547,546,547],[548,548,549],[549,548,549],[550,550,551],[551,550,551],[552,552,553],[553,552,553],[554,554,555],[555,554,555],[556,556,557],[557,556,557],[558,558,559],[559,558,559],[560,560,561],[561,560,561],[562,562,563],[563,562,563],[564,564,564],[571,571,572],[572,571,572],[573,573,410],[574,574,11366],[575,11390,575],[576,11391,576],[577,577,578],[578,577,578],[579,579,384],[582,582,583],[583,582,583],[584,584,585],[585,584,585],[586,586,587],[587,586,587],[588,588,589],[589,588,589],[590,590,591],[591,590,591],[593,11373,593],[596,390,596],[597,597,597],[598,393,598],[599,394,599],[600,600,600],[601,399,601],[602,602,602],[603,400,603],[604,42923,604],[605,605,605],[607,607,607],[608,403,608],[609,42924,609],[610,610,610],[611,404,611],[612,42955,612],[614,42922,614],[615,615,615],[616,407,616],[618,42926,618],[620,42925,620],[621,621,621],[622,622,622],[623,412,623],[624,624,624],[625,11374,625],[626,413,626],[627,627,627],[628,628,628],[629,415,629],[630,630,630],[636,636,636],[637,11364,637],[638,638,638],[639,639,639],[640,422,640],[641,641,641],[642,42949,642],[643,425,643],[644,644,644],[646,646,646],[647,42929,647],[649,580,649],[652,581,652],[653,653,653],[657,657,657],[658,439,658],[659,659,659],[668,668,668],[669,42930,669],[670,42928,670],[671,671,671],[836,836,836],[837,921,837],[838,838,838],[880,880,881],[881,880,881],[882,882,883],[883,882,883],[884,884,884],[886,886,887],[887,886,887],[888,888,888],[890,890,890],[891,1021,891],[893,1023,893],[894,894,894],[939,939,971],[940,902,940],[941,904,941],[943,906,943],[944,944,944],[945,913,945],[959,927,959],[974,911,974],[975,975,983],[976,914,976],[977,920,977],[978,978,978],[980,980,980],[981,934,981],[983,975,983],[984,984,985],[985,984,985],[986,986,987],[987,986,987],[988,988,989],[989,988,989],[990,990,991],[991,990,991],[992,992,993],[993,992,993],[994,994,995],[995,994,995],[996,996,997],[997,996,997],[998,998,999],[999,998,999],[1000,1000,1001],[1001,1000,1001],[1002,1002,1003],[1003,1002,1003],[1004,1004,1005],[1005,1004,1005],[1006,1006,1007],[1007,1006,1007],[1009,929,1009],[1011,895,1011],[1012,1012,952],[1013,917,1013],[1014,1014,1014],[1015,1015,1016],[1016,1015,1016],[1017,1017,1010],[1018,1018,1019],[1019,1018,1019],[1020,1020,1020],[1071,1071,1103],[1072,1040,1072],[1095,1063,1095],[1119,1039,1119],[1120,1120,1121],[1121,1120,1121],[1122,1122,1123],[1123,1122,1123],[1124,1124,1125],[1125,1124,1125],[1126,1126,1127],[1127,1126,1127],[1128,1128,1129],[1129,1128,1129],[1130,1130,1131],[1131,1130,1131],[1132,1132,1133],[1133,1132,1133],[1134,1134,1135],[1135,1134,1135],[1136,1136,1137],[1137,1136,1137],[1138,1138,1139],[1139,1138,1139],[1140,1140,1141],[1141,1140,1141],[1142,1142,1143],[1143,1142,1143],[1144,1144,1145],[1145,1144,1145],[1146,1146,1147],[1147,1146,1147],[1148,1148,1149],[1149,1148,1149],[1150,1150,1151],[1151,1150,1151],[1152,1152,1153],[1153,1152,1153],[1154,1154,1154],[1162,1162,1163],[1163,1162,1163],[1164,1164,1165],[1165,1164,1165],[1166,1166,1167],[1167,1166,1167],[1168,1168,1169],[1169,1168,1169],[1170,1170,1171],[1171,1170,1171],[1172,1172,1173],[1173,1172,1173],[1174,1174,1175],[1175,1174,1175],[1176,1176,1177],[1177,1176,1177],[1178,1178,1179],[1179,1178,1179],[1180,1180,1181],[1181,1180,1181],[1182,1182,1183],[1183,1182,1183],[1184,1184,1185],[1185,1184,1185],[1186,1186,1187],[1187,1186,1187],[1188,1188,1189],[1189,1188,1189],[1190,1190,1191],[1191,1190,1191],[1192,1192,1193],[1193,1192,1193],[1194,1194,1195],[1195,1194,1195],[1196,1196,1197],[1197,1196,1197],[1198,1198,1199],[1199,1198,1199],[1200,1200,1201],[1201,1200,1201],[1202,1202,1203],[1203,1202,1203],[1204,1204,1205],[1205,1204,1205],[1206,1206,1207],[1207,1206,1207],[1208,1208,1209],[1209,1208,1209],[1210,1210,1211],[1211,1210,1211],[1212,1212,1213],[1213,1212,1213],[1214,1214,1215],[1215,1214,1215],[1216,1216,1231],[1217,1217,1218],[1218,1217,1218],[1219,1219,1220],[1220,1219,1220],[1221,1221,1222],[1222,1221,1222],[1223,1223,1224],[1224,1223,1224],[1225,1225,1226],[1226,1225,1226],[1227,1227,1228],[1228,1227,1228],[1229,1229,1230],[1230,1229,1230],[1231,1216,1231],[1232,1232,1233],[1233,1232,1233],[1234,1234,1235],[1235,1234,1235],[1236,1236,1237],[1237,1236,1237],[1238,1238,1239],[1239,1238,1239],[1240,1240,1241],[1241,1240,1241],[1242,1242,1243],[1243,1242,1243],[1244,1244,1245],[1245,1244,1245],[1246,1246,1247],[1247,1246,1247],[1248,1248,1249],[1249,1248,1249],[1250,1250,1251],[1251,1250,1251],[1252,1252,1253],[1253,1252,1253],[1254,1254,1255],[1255,1254,1255],[1256,1256,1257],[1257,1256,1257],[1258,1258,1259],[1259,1258,1259],[1260,1260,1261],[1261,1260,1261],[1262,1262,1263],[1263,1262,1263],[1264,1264,1265],[1265,1264,1265],[1266,1266,1267],[1267,1266,1267],[1268,1268,1269],[1269,1268,1269],[1270,1270,1271],[1271,1270,1271],[1272,1272,1273],[1273,1272,1273],[1274,1274,1275],[1275,1274,1275],[1276,1276,1277],[1277,1276,1277],[1278,1278,1279],[1279,1278,1279],[1280,1280,1281],[1281,1280,1281],[1282,1282,1283],[1283,1282,1283],[1284,1284,1285],[1285,1284,1285],[1286,1286,1287],[1287,1286,1287],[1288,1288,1289],[1289,1288,1289],[1290,1290,1291],[1291,1290,1291],[1292,1292,1293],[1293,1292,1293],[1294,1294,1295],[1295,1294,1295],[1296,1296,1297],[1297,1296,1297],[1298,1298,1299],[1299,1298,1299],[1300,1300,1301],[1301,1300,1301],[1302,1302,1303],[1303,1302,1303],[1304,1304,1305],[1305,1304,1305],[1306,1306,1307],[1307,1306,1307],[1308,1308,1309],[1309,1308,1309],[1310,1310,1311],[1311,1310,1311],[1312,1312,1313],[1313,1312,1313],[1314,1314,1315],[1315,1314,1315],[1316,1316,1317],[1317,1316,1317],[1318,1318,1319],[1319,1318,1319],[1320,1320,1321],[1321,1320,1321],[1322,1322,1323],[1323,1322,1323],[1324,1324,1325],[1325,1324,1325],[1326,1326,1327],[1327,1326,1327],[1328,1328,1328],[1376,1376,1376],[1377,1329,1377],[1395,1347,1395],[1414,1366,1414],[1415,1415,1415],[4303,4303,4303],[4304,7312,4304],[4325,7333,4325],[4346,7354,4346],[4347,4347,4347],[4348,4348,4348],[4349,7357,4349],[4351,7359,4351],[4352,4352,4352],[5111,5111,5111],[5112,5104,5112],[5114,5106,5114],[5117,5109,5117],[5118,5118,5118],[7295,7295,7295],[7296,1042,7296],[7300,1058,7300],[7304,42570,7304],[7305,7305,7306],[7306,7305,7306],[7307,7307,7307],[7544,7544,7544],[7545,42877,7545],[7546,7546,7546],[7548,7548,7548],[7549,11363,7549],[7550,7550,7550],[7565,7565,7565],[7566,42950,7566],[7567,7567,7567],[7680,7680,7681],[7681,7680,7681],[7682,7682,7683],[7683,7682,7683],[7684,7684,7685],[7685,7684,7685],[7686,7686,7687],[7687,7686,7687],[7688,7688,7689],[7689,7688,7689],[7690,7690,7691],[7691,7690,7691],[7692,7692,7693],[7693,7692,7693],[7694,7694,7695],[7695,7694,7695],[7696,7696,7697],[7697,7696,7697],[7698,7698,7699],[7699,7698,7699],[7700,7700,7701],[7701,7700,7701],[7702,7702,7703],[7703,7702,7703],[7704,7704,7705],[7705,7704,7705],[7706,7706,7707],[7707,7706,7707],[7708,7708,7709],[7709,7708,7709],[7710,7710,7711],[7711,7710,7711],[7712,7712,7713],[7713,7712,7713],[7714,7714,7715],[7715,7714,7715],[7716,7716,7717],[7717,7716,7717],[7718,7718,7719],[7719,7718,7719],[7720,7720,7721],[7721,7720,7721],[7722,7722,7723],[7723,7722,7723],[7724,7724,7725],[7725,7724,7725],[7726,7726,7727],[7727,7726,7727],[7728,7728,7729],[7729,7728,7729],[7730,7730,7731],[7731,7730,7731],[7732,7732,7733],[7733,7732,7733],[7734,7734,7735],[7735,7734,7735],[7736,7736,7737],[7737,7736,7737],[7738,7738,7739],[7739,7738,7739],[7740,7740,7741],[7741,7740,7741],[7742,7742,7743],[7743,7742,7743],[7744,7744,7745],[7745,7744,7745],[7746,7746,7747],[7747,7746,7747],[7748,7748,7749],[7749,7748,7749],[7750,7750,7751],[7751,7750,7751],[7752,7752,7753],[7753,7752,7753],[7754,7754,7755],[7755,7754,7755],[7756,7756,7757],[7757,7756,7757],[7758,7758,7759],[7759,7758,7759],[7760,7760,7761],[7761,7760,7761],[7762,7762,7763],[7763,7762,7763],[7764,7764,7765],[7765,7764,7765],[7766,7766,7767],[7767,7766,7767],[7768,7768,7769],[7769,7768,7769],[7770,7770,7771],[7771,7770,7771],[7772,7772,7773],[7773,7772,7773],[7774,7774,7775],[7775,7774,7775],[7776,7776,7777],[7777,7776,7777],[7778,7778,7779],[7779,7778,7779],[7780,7780,7781],[7781,7780,7781],[7782,7782,7783],[7783,7782,7783],[7784,7784,7785],[7785,7784,7785],[7786,7786,7787],[7787,7786,7787],[7788,7788,7789],[7789,7788,7789],[7790,7790,7791],[7791,7790,7791],[7792,7792,7793],[7793,7792,7793],[7794,7794,7795],[7795,7794,7795],[7796,7796,7797],[7797,7796,7797],[7798,7798,7799],[7799,7798,7799],[7800,7800,7801],[7801,7800,7801],[7802,7802,7803],[7803,7802,7803],[7804,7804,7805],[7805,7804,7805],[7806,7806,7807],[7807,7806,7807],[7808,7808,7809],[7809,7808,7809],[7810,7810,7811],[7811,7810,7811],[7812,7812,7813],[7813,7812,7813],[7814,7814,7815],[7815,7814,7815],[7816,7816,7817],[7817,7816,7817],[7818,7818,7819],[7819,7818,7819],[7820,7820,7821],[7821,7820,7821],[7822,7822,7823],[7823,7822,7823],[7824,7824,7825],[7825,7824,7825],[7826,7826,7827],[7827,7826,7827],[7828,7828,7829],[7829,7828,7829],[7830,7830,7830],[7834,7834,7834],[7835,7776,7835],[7836,7836,7836],[7840,7840,7841],[7841,7840,7841],[7842,7842,7843],[7843,7842,7843],[7844,7844,7845],[7845,7844,7845],[7846,7846,7847],[7847,7846,7847],[7848,7848,7849],[7849,7848,7849],[7850,7850,7851],[7851,7850,7851],[7852,7852,7853],[7853,7852,7853],[7854,7854,7855],[7855,7854,7855],[7856,7856,7857],[7857,7856,7857],[7858,7858,7859],[7859,7858,7859],[7860,7860,7861],[7861,7860,7861],[7862,7862,7863],[7863,7862,7863],[7864,7864,7865],[7865,7864,7865],[7866,7866,7867],[7867,7866,7867],[7868,7868,7869],[7869,7868,7869],[7870,7870,7871],[7871,7870,7871],[7872,7872,7873],[7873,7872,7873],[7874,7874,7875],[7875,7874,7875],[7876,7876,7877],[7877,7876,7877],[7878,7878,7879],[7879,7878,7879],[7880,7880,7881],[7881,7880,7881],[7882,7882,7883],[7883,7882,7883],[7884,7884,7885],[7885,7884,7885],[7886,7886,7887],[7887,7886,7887],[7888,7888,7889],[7889,7888,7889],[7890,7890,7891],[7891,7890,7891],[7892,7892,7893],[7893,7892,7893],[7894,7894,7895],[7895,7894,7895],[7896,7896,7897],[7897,7896,7897],[7898,7898,7899],[7899,7898,7899],[7900,7900,7901],[7901,7900,7901],[7902,7902,7903],[7903,7902,7903],[7904,7904,7905],[7905,7904,7905],[7906,7906,7907],[7907,7906,7907],[7908,7908,7909],[7909,7908,7909],[7910,7910,7911],[7911,7910,7911],[7912,7912,7913],[7913,7912,7913],[7914,7914,7915],[7915,7914,7915],[7916,7916,7917],[7917,7916,7917],[7918,7918,7919],[7919,7918,7919],[7920,7920,7921],[7921,7920,7921],[7922,7922,7923],[7923,7922,7923],[7924,7924,7925],[7925,7924,7925],[7926,7926,7927],[7927,7926,7927],[7928,7928,7929],[7929,7928,7929],[7930,7930,7931],[7931,7930,7931],[7932,7932,7933],[7933,7932,7933],[7934,7934,7935],[7935,7934,7935],[7939,7947,7939],[7943,7951,7943],[7944,7944,7936],[7951,7951,7943],[7952,7960,7952],[7954,7962,7954],[7957,7965,7957],[7958,7958,7958],[7967,7967,7967],[7968,7976,7968],[7971,7979,7971],[7975,7983,7975],[7976,7976,7968],[7983,7983,7975],[7984,7992,7984],[7987,7995,7987],[7991,7999,7991],[7992,7992,7984],[7999,7999,7991],[8000,8008,8000],[8002,8010,8002],[8005,8013,8005],[8006,8006,8006],[8016,8016,8016],[8017,8025,8017],[8018,8018,8018],[8019,8027,8019],[8020,8020,8020],[8021,8029,8021],[8022,8022,8022],[8023,8031,8023],[8024,8024,8024],[8031,8031,8023],[8032,8040,8032],[8035,8043,8035],[8039,8047,8039],[8040,8040,8032],[8047,8047,8039],[8048,8122,8048],[8054,8154,8054],[8061,8187,8061],[8062,8062,8062],[8063,8063,8063],[8064,8072,8064],[8067,8075,8067],[8071,8079,8071],[8072,8072,8064],[8079,8079,8071],[8080,8088,8080],[8083,8091,8083],[8087,8095,8087],[8088,8088,8080],[8095,8095,8087],[8096,8104,8096],[8099,8107,8099],[8103,8111,8103],[8104,8104,8096],[8111,8111,8103],[8112,8120,8112],[8113,8121,8113],[8114,8114,8114],[8115,8124,8115],[8116,8116,8116],[8125,8125,8125],[8126,921,8126],[8127,8127,8127],[8130,8130,8130],[8131,8140,8131],[8132,8132,8132],[8143,8143,8143],[8144,8152,8144],[8145,8153,8145],[8146,8146,8146],[8159,8159,8159],[8160,8168,8160],[8161,8169,8161],[8162,8162,8162],[8164,8164,8164],[8165,8172,8165],[8166,8166,8166],[8178,8178,8178],[8179,8188,8179],[8180,8180,8180],[8525,8525,8525],[8526,8498,8526],[8527,8527,8527],[8559,8559,8575],[8560,8544,8560],[8567,8551,8567],[8575,8559,8575],[8576,8576,8576],[8579,8579,8580],[8580,8579,8580],[8581,8581,8581],[9423,9423,9449],[9424,9398,9424],[9436,9410,9436],[9449,9423,9449],[9450,9450,9450],[11311,11311,11359],[11312,11264,11312],[11335,11287,11335],[11359,11311,11359],[11360,11360,11361],[11361,11360,11361],[11362,11362,619],[11364,11364,637],[11365,570,11365],[11366,574,11366],[11367,11367,11368],[11368,11367,11368],[11369,11369,11370],[11370,11369,11370],[11371,11371,11372],[11372,11371,11372],[11373,11373,593],[11378,11378,11379],[11379,11378,11379],[11380,11380,11380],[11381,11381,11382],[11382,11381,11382],[11383,11383,11383],[11392,11392,11393],[11393,11392,11393],[11394,11394,11395],[11395,11394,11395],[11396,11396,11397],[11397,11396,11397],[11398,11398,11399],[11399,11398,11399],[11400,11400,11401],[11401,11400,11401],[11402,11402,11403],[11403,11402,11403],[11404,11404,11405],[11405,11404,11405],[11406,11406,11407],[11407,11406,11407],[11408,11408,11409],[11409,11408,11409],[11410,11410,11411],[11411,11410,11411],[11412,11412,11413],[11413,11412,11413],[11414,11414,11415],[11415,11414,11415],[11416,11416,11417],[11417,11416,11417],[11418,11418,11419],[11419,11418,11419],[11420,11420,11421],[11421,11420,11421],[11422,11422,11423],[11423,11422,11423],[11424,11424,11425],[11425,11424,11425],[11426,11426,11427],[11427,11426,11427],[11428,11428,11429],[11429,11428,11429],[11430,11430,11431],[11431,11430,11431],[11432,11432,11433],[11433,11432,11433],[11434,11434,11435],[11435,11434,11435],[11436,11436,11437],[11437,11436,11437],[11438,11438,11439],[11439,11438,11439],[11440,11440,11441],[11441,11440,11441],[11442,11442,11443],[11443,11442,11443],[11444,11444,11445],[11445,11444,11445],[11446,11446,11447],[11447,11446,11447],[11448,11448,11449],[11449,11448,11449],[11450,11450,11451],[11451,11450,11451],[11452,11452,11453],[11453,11452,11453],[11454,11454,11455],[11455,11454,11455],[11456,11456,11457],[11457,11456,11457],[11458,11458,11459],[11459,11458,11459],[11460,11460,11461],[11461,11460,11461],[11462,11462,11463],[11463,11462,11463],[11464,11464,11465],[11465,11464,11465],[11466,11466,11467],[11467,11466,11467],[11468,11468,11469],[11469,11468,11469],[11470,11470,11471],[11471,11470,11471],[11472,11472,11473],[11473,11472,11473],[11474,11474,11475],[11475,11474,11475],[11476,11476,11477],[11477,11476,11477],[11478,11478,11479],[11479,11478,11479],[11480,11480,11481],[11481,11480,11481],[11482,11482,11483],[11483,11482,11483],[11484,11484,11485],[11485,11484,11485],[11486,11486,11487],[11487,11486,11487],[11488,11488,11489],[11489,11488,11489],[11490,11490,11491],[11491,11490,11491],[11492,11492,11492],[11499,11499,11500],[11500,11499,11500],[11501,11501,11502],[11502,11501,11502],[11503,11503,11503],[11506,11506,11507],[11507,11506,11507],[11508,11508,11508],[11519,11519,11519],[11520,4256,11520],[11538,4274,11538],[11557,4293,11557],[11558,11558,11558],[11559,4295,11559],[11560,11560,11560],[11564,11564,11564],[11565,4301,11565],[11566,11566,11566],[42560,42560,42561],[42561,42560,42561],[42562,42562,42563],[42563,42562,42563],[42564,42564,42565],[42565,42564,42565],[42566,42566,42567],[42567,42566,42567],[42568,42568,42569],[42569,42568,42569],[42570,42570,42571],[42571,42570,42571],[42572,42572,42573],[42573,42572,42573],[42574,42574,42575],[42575,42574,42575],[42576,42576,42577],[42577,42576,42577],[42578,42578,42579],[42579,42578,42579],[42580,42580,42581],[42581,42580,42581],[42582,42582,42583],[42583,42582,42583],[42584,42584,42585],[42585,42584,42585],[42586,42586,42587],[42587,42586,42587],[42588,42588,42589],[42589,42588,42589],[42590,42590,42591],[42591,42590,42591],[42592,42592,42593],[42593,42592,42593],[42594,42594,42595],[42595,42594,42595],[42596,42596,42597],[42597,42596,42597],[42598,42598,42599],[42599,42598,42599],[42600,42600,42601],[42601,42600,42601],[42602,42602,42603],[42603,42602,42603],[42604,42604,42605],[42605,42604,42605],[42606,42606,42606],[42624,42624,42625],[42625,42624,42625],[42626,42626,42627],[42627,42626,42627],[42628,42628,42629],[42629,42628,42629],[42630,42630,42631],[42631,42630,42631],[42632,42632,42633],[42633,42632,42633],[42634,42634,42635],[42635,42634,42635],[42636,42636,42637],[42637,42636,42637],[42638,42638,42639],[42639,42638,42639],[42640,42640,42641],[42641,42640,42641],[42642,42642,42643],[42643,42642,42643],[42644,42644,42645],[42645,42644,42645],[42646,42646,42647],[42647,42646,42647],[42648,42648,42649],[42649,42648,42649],[42650,42650,42651],[42651,42650,42651],[42652,42652,42652],[42786,42786,42787],[42787,42786,42787],[42788,42788,42789],[42789,42788,42789],[42790,42790,42791],[42791,42790,42791],[42792,42792,42793],[42793,42792,42793],[42794,42794,42795],[42795,42794,42795],[42796,42796,42797],[42797,42796,42797],[42798,42798,42799],[42799,42798,42799],[42800,42800,42800],[42802,42802,42803],[42803,42802,42803],[42804,42804,42805],[42805,42804,42805],[42806,42806,42807],[42807,42806,42807],[42808,42808,42809],[42809,42808,42809],[42810,42810,42811],[42811,42810,42811],[42812,42812,42813],[42813,42812,42813],[42814,42814,42815],[42815,42814,42815],[42816,42816,42817],[42817,42816,42817],[42818,42818,42819],[42819,42818,42819],[42820,42820,42821],[42821,42820,42821],[42822,42822,42823],[42823,42822,42823],[42824,42824,42825],[42825,42824,42825],[42826,42826,42827],[42827,42826,42827],[42828,42828,42829],[42829,42828,42829],[42830,42830,42831],[42831,42830,42831],[42832,42832,42833],[42833,42832,42833],[42834,42834,42835],[42835,42834,42835],[42836,42836,42837],[42837,42836,42837],[42838,42838,42839],[42839,42838,42839],[42840,42840,42841],[42841,42840,42841],[42842,42842,42843],[42843,42842,42843],[42844,42844,42845],[42845,42844,42845],[42846,42846,42847],[42847,42846,42847],[42848,42848,42849],[42849,42848,42849],[42850,42850,42851],[42851,42850,42851],[42852,42852,42853],[42853,42852,42853],[42854,42854,42855],[42855,42854,42855],[42856,42856,42857],[42857,42856,42857],[42858,42858,42859],[42859,42858,42859],[42860,42860,42861],[42861,42860,42861],[42862,42862,42863],[42863,42862,42863],[42864,42864,42864],[42873,42873,42874],[42874,42873,42874],[42875,42875,42876],[42876,42875,42876],[42877,42877,7545],[42878,42878,42879],[42879,42878,42879],[42880,42880,42881],[42881,42880,42881],[42882,42882,42883],[42883,42882,42883],[42884,42884,42885],[42885,42884,42885],[42886,42886,42887],[42887,42886,42887],[42888,42888,42888],[42891,42891,42892],[42892,42891,42892],[42893,42893,613],[42896,42896,42897],[42897,42896,42897],[42898,42898,42899],[42899,42898,42899],[42900,42948,42900],[42901,42901,42901],[42902,42902,42903],[42903,42902,42903],[42904,42904,42905],[42905,42904,42905],[42906,42906,42907],[42907,42906,42907],[42908,42908,42909],[42909,42908,42909],[42910,42910,42911],[42911,42910,42911],[42912,42912,42913],[42913,42912,42913],[42914,42914,42915],[42915,42914,42915],[42916,42916,42917],[42917,42916,42917],[42918,42918,42919],[42919,42918,42919],[42920,42920,42921],[42921,42920,42921],[42922,42922,614],[42932,42932,42933],[42933,42932,42933],[42934,42934,42935],[42935,42934,42935],[42936,42936,42937],[42937,42936,42937],[42938,42938,42939],[42939,42938,42939],[42940,42940,42941],[42941,42940,42941],[42942,42942,42943],[42943,42942,42943],[42944,42944,42945],[42945,42944,42945],[42946,42946,42947],[42947,42946,42947],[42948,42948,42900],[42951,42951,42952],[42952,42951,42952],[42953,42953,42954],[42954,42953,42954],[42955,42955,612],[42956,42956,42957],[42957,42956,42957],[42958,42958,42959],[42959,42958,42959],[42960,42960,42961],[42961,42960,42961],[42962,42962,42963],[42963,42962,42963],[42964,42964,42965],[42965,42964,42965],[42966,42966,42967],[42967,42966,42967],[42968,42968,42969],[42969,42968,42969],[42970,42970,42971],[42971,42970,42971],[42972,42972,411],[42997,42997,42998],[42998,42997,42998],[42999,42999,42999],[43858,43858,43858],[43859,42931,43859],[43860,43860,43860],[43887,43887,43887],[43888,5024,43888],[43927,5063,43927],[43967,5103,43967],[43968,43968,43968],[65344,65344,65344],[65345,65313,65345],[65357,65325,65357],[65370,65338,65370],[65371,65371,65371],[66599,66599,66639],[66600,66560,66600],[66619,66579,66619],[66639,66599,66639],[66640,66640,66640],[66775,66775,66775],[66776,66736,66776],[66793,66753,66793],[66811,66771,66811],[66812,66812,66812],[66966,66966,66966],[66967,66928,66967],[66972,66933,66972],[66977,66938,66977],[66978,66978,66978],[66979,66940,66979],[66986,66947,66986],[66993,66954,66993],[66994,66994,66994],[66995,66956,66995],[66998,66959,66998],[67001,66962,67001],[67002,67002,67002],[67003,66964,67003],[67004,66965,67004],[67005,67005,67005],[68799,68799,68799],[68800,68736,68800],[68825,68761,68825],[68850,68786,68850],[68851,68851,68851],[68975,68975,68975],[68976,68944,68976],[68986,68954,68986],[68997,68965,68997],[68998,68998,68998],[71871,71871,71903],[71872,71840,71872],[71887,71855,71887],[71903,71871,71903],[71904,71904,71904],[93791,93791,93823],[93792,93760,93792],[93807,93775,93807],[93823,93791,93823],[93824,93824,93824],[93882,93882,93882],[93883,93856,93883],[93895,93868,93895],[93907,93880,93907],[93908,93908,93908],[125217,125217,125251],[125218,125184,125218],[125234,125200,125234],[125251,125217,125251],[125252,125252,125252],[64,64,64],[65,65,97],[77,77,109],[90,90,122],[91,91,91],[191,191,191],[192,192,224],[203,203,235],[214,214,246],[215,215,215],[216,216,248],[219,219,251],[222,222,254],[398,398,477],[399,399,601],[426,426,426],[427,427,427],[443,443,443],[451,451,451],[545,545,545],[569,569,569],[570,570,11365],[580,580,649],[879,879,879],[885,885,885],[895,895,1011],[896,896,896],[901,901,901],[902,902,940],[903,903,903],[904,904,941],[906,906,943],[907,907,907],[908,908,972],[909,909,909],[910,910,973],[911,911,974],[912,912,912],[913,913,945],[921,921,953],[929,929,961],[930,930,930],[931,931,963],[935,935,967],[1021,1021,891],[1046,1046,1078],[1161,1161,1161],[1329,1329,1377],[1347,1347,1395],[1366,1366,1414],[1367,1367,1367],[4255,4255,4255],[4256,4256,11520],[4274,4274,11538],[4293,4293,11557],[4294,4294,4294],[4295,4295,11559],[4296,4296,4296],[4300,4300,4300],[4301,4301,11565],[4302,4302,4302],[5023,5023,5023],[5024,5024,43888],[5066,5066,43930],[5109,5109,5117],[5110,5110,5110],[7311,7311,7311],[7312,7312,4304],[7333,7333,4325],[7354,7354,4346],[7355,7355,7355],[7356,7356,7356],[7357,7357,4349],[7359,7359,4351],[7360,7360,7360],[7679,7679,7679],[7837,7837,7837],[7838,7838,223],[7839,7839,7839],[7947,7947,7939],[7959,7959,7959],[7960,7960,7952],[7962,7962,7954],[7965,7965,7957],[7966,7966,7966],[7979,7979,7971],[7995,7995,7987],[8007,8007,8007],[8008,8008,8000],[8010,8010,8002],[8013,8013,8005],[8014,8014,8014],[8025,8025,8017],[8026,8026,8026],[8027,8027,8019],[8028,8028,8028],[8029,8029,8021],[8030,8030,8030],[8043,8043,8035],[8075,8075,8067],[8091,8091,8083],[8107,8107,8099],[8119,8119,8119],[8120,8120,8112],[8122,8122,8048],[8124,8124,8115],[8135,8135,8135],[8136,8136,8050],[8138,8138,8052],[8140,8140,8131],[8141,8141,8141],[8151,8151,8151],[8152,8152,8144],[8153,8153,8145],[8155,8155,8055],[8156,8156,8156],[8167,8167,8167],[8168,8168,8160],[8170,8170,8058],[8172,8172,8165],[8173,8173,8173],[8183,8183,8183],[8184,8184,8056],[8186,8186,8060],[8188,8188,8179],[8189,8189,8189],[8485,8485,8485],[8486,8486,969],[8487,8487,8487],[8489,8489,8489],[8490,8490,107],[8491,8491,229],[8492,8492,8492],[8497,8497,8497],[8498,8498,8526],[8499,8499,8499],[8543,8543,8543],[8544,8544,8560],[8551,8551,8567],[8578,8578,8578],[9397,9397,9397],[9398,9398,9424],[9410,9410,9436],[11263,11263,11263],[11264,11264,11312],[11287,11287,11335],[11374,11374,625],[11376,11376,594],[11377,11377,11377],[11389,11389,11389],[11390,11390,575],[11498,11498,11498],[11505,11505,11505],[42559,42559,42559],[42623,42623,42623],[42785,42785,42785],[42801,42801,42801],[42872,42872,42872],[42890,42890,42890],[42894,42894,42894],[42895,42895,42895],[42924,42924,609],[42926,42926,618],[42927,42927,42927],[42928,42928,670],[42930,42930,669],[42949,42949,642],[42973,42973,42973],[42996,42996,42996],[65312,65312,65312],[65313,65313,65345],[65325,65325,65357],[65338,65338,65370],[65339,65339,65339],[66559,66559,66559],[66560,66560,66600],[66579,66579,66619],[66735,66735,66735],[66736,66736,66776],[66753,66753,66793],[66771,66771,66811],[66772,66772,66772],[66927,66927,66927],[66928,66928,66967],[66933,66933,66972],[66938,66938,66977],[66939,66939,66939],[66940,66940,66979],[66947,66947,66986],[66954,66954,66993],[66955,66955,66955],[66956,66956,66995],[66959,66959,66998],[66962,66962,67001],[66963,66963,66963],[66964,66964,67003],[66965,66965,67004],[68735,68735,68735],[68736,68736,68800],[68761,68761,68825],[68786,68786,68850],[68787,68787,68787],[68943,68943,68943],[68944,68944,68976],[68954,68954,68986],[68965,68965,68997],[68966,68966,68966],[71839,71839,71839],[71840,71840,71872],[71855,71855,71887],[93759,93759,93759],[93760,93760,93792],[93775,93775,93807],[93855,93855,93855],[93856,93856,93883],[93868,93868,93895],[93880,93880,93907],[93881,93881,93881],[125183,125183,125183],[125184,125184,125218],[125200,125200,125234]];
`

var watPreludeM2Runner = strings.Replace(watM2RunnerSrc, "@@CASES@@", watM2CaseBoundaries, 1)
