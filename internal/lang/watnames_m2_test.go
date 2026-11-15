package lang

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The M2 fail-code and message-length constants are emitted as literals in
// the prelude and mirrored by the runner's own table (run.mjs). This test
// pins them together so a renumber or rename cannot drift one surface
// silently (DESIGN-M2 §2c; codes are never renumbered).
func TestWatM2CodeNamesPinned(t *testing.T) {
	preludeBytes, err := os.ReadFile("watprelude_m2.go")
	if err != nil {
		t.Fatal(err)
	}
	prelude := string(preludeBytes)

	// Fail codes 13/14 are fixed-string fails; 15 is staged (array.select).
	for _, want := range []string{
		"(call $zhex_fail (i32.const " + strconv.Itoa(watFailReplaceEmpty) + "))",
		"(call $zhex_fail (i32.const " + strconv.Itoa(watFailSplitLimit) + "))",
		"(call $zhex_fail_msg (i32.const " + strconv.Itoa(watFailArraySelect) + ")",
		// The err value mints the ParseError family.
		"(struct.new " + watTypeErr + " (i32.const " + strconv.Itoa(watErrFamilyParse) + ")",
		// The message-length accessor uses the prelude name and its export.
		"(func " + watMsgLenGet + " (export \"" + watMsgLenExport + "\")",
	} {
		if !strings.Contains(prelude, want) {
			t.Errorf("prelude is missing pinned literal %q", want)
		}
	}

	// run.mjs mirrors the staged-message codes and the export name. Codes are
	// additive (never renumbered), so later milestones extend the set: the M2
	// codes must be present as members, not as the whole set.
	runBytes, err := os.ReadFile(filepath.Join("..", "wasm", "testdata", "run.mjs"))
	if err != nil {
		t.Skipf("run.mjs not found from this module layout: %v", err)
	}
	run := string(runBytes)
	setStart := strings.Index(run, "stagedFailCodes = new Set([")
	if setStart < 0 {
		t.Fatal("run.mjs lost its stagedFailCodes set")
	}
	setEnd := strings.Index(run[setStart:], "])")
	if setEnd < 0 {
		t.Fatal("run.mjs stagedFailCodes set is unterminated")
	}
	setBody := run[setStart : setStart+setEnd]
	for _, code := range []int{watFailTextToInt, watFailArraySelect} {
		if !strings.Contains(setBody, strconv.Itoa(code)) {
			t.Errorf("run.mjs staged fail codes are missing %d", code)
		}
	}
	if !strings.Contains(run, watMsgLenExport+"()") {
		t.Errorf("run.mjs is missing the %s() accessor call", watMsgLenExport)
	}
}
