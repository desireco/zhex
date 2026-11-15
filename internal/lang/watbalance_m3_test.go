package lang

import (
	"strings"
	"testing"
)

// TestWatM3ParenBalance guards the bug class that twice broke the milestone-3
// build: every WAT text the compiler concatenates into a module must be
// paren-balanced before wasm-tools ever sees it. Tool-less (a plain counter
// over comment-stripped text, skipping double-quoted string segments), so it
// runs on CI without the wasm toolchain. Skewed (not just unbalanced) text is
// reported with both boundary counts to catch text that opens or closes a
// group it never finishes.
func TestWatM3ParenBalance(t *testing.T) {
	for name, wat := range map[string]string{
		"watPrelude":        watPrelude,
		"watPreludeM2":      watPreludeM2,
		"watPreludeM3":      watPreludeM3,
		"watBrowserImports": watBrowserImports,
		"watBrowserFuncs":   watBrowserFuncs,
		"watWasiImports":    watWasiImports,
		"watWasiFuncs":      watWasiFuncs,
	} {
		opens, closes, minDepth := countWatParens(wat)
		if opens != closes || minDepth < 0 {
			t.Errorf("%s is not paren-balanced: %d opens, %d closes, min depth %d", name, opens, closes, minDepth)
		}
	}
	// The seam strip must keep the prelude balanced too (it cuts whole
	// balanced definitions); idempotence is pinned in the names test.
	if o, c, _ := countWatParens(watStripSeamDefs(watPrelude + watPreludeM2 + watPreludeM3)); o != c {
		t.Errorf("seam-stripped prelude is not paren-balanced: %d opens, %d closes", o, c)
	}
}

// countWatParens counts parens outside ;; comments and double-quoted string
// segments (WAT data/string literals), tracking the minimum depth so a stray
// closer is caught even when totals match.
func countWatParens(wat string) (opens, closes, minDepth int) {
	inComment := false
	inString := false
	depth := 0
	for i := 0; i < len(wat); i++ {
		ch := wat[i]
		if inComment {
			if ch == '\n' {
				inComment = false
			}
			continue
		}
		if inString {
			if ch == '\\' {
				i++ // skip the escaped byte
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case ';':
			if i+1 < len(wat) && wat[i+1] == ';' {
				inComment = true
			}
		case '"':
			inString = true
		case '(':
			opens++
			depth++
		case ')':
			closes++
			depth--
			if depth < minDepth {
				minDepth = depth
			}
		}
	}
	return opens, closes, minDepth
}

// TestWatM3IfThenShape catches the second wasm-tools failure class seen in
// this milestone: a folded (if ...) whose trailing arm is not (else ...) —
// valid parens, invalid grammar. Scans every emitted WAT text for an (if
// whose (then ...) group is followed by anything but an (else group or the
// if's own close.
func TestWatM3IfThenShape(t *testing.T) {
	for name, wat := range map[string]string{
		"watPrelude":      watPrelude,
		"watPreludeM2":    watPreludeM2,
		"watPreludeM3":    watPreludeM3,
		"watBrowserFuncs": watBrowserFuncs,
		"watWasiFuncs":    watWasiFuncs,
	} {
		for _, bad := range ifTrailingArms(wat) {
			t.Errorf("%s: (if without (else after its (then block near offset %d: %.60s", name, bad, wat[bad:bad+60])
		}
	}
}

// ifTrailingArms returns the byte offsets of malformed folded ifs: after the
// condition group and the (then ...) group, the next non-space token must be
// (else or ')'. Comments are not stripped here; WAT texts keep ;; comments on
// their own lines, so the token scan below may land on a comment only when a
// real arm is missing — wasm-tools remains the final authority.
func ifTrailingArms(wat string) []int {
	var bad []int
	rest := wat
	base := 0
	for {
		i := strings.Index(rest, "(if")
		if i < 0 {
			break
		}
		off := base + i
		afterCond, ok := skipIfCondition(wat, off)
		if ok {
			if thenOpen := strings.Index(wat[afterCond:], "(then"); thenOpen >= 0 {
				thenStart := afterCond + thenOpen
				thenEnd := matchGroup(wat, thenStart)
				if thenEnd >= 0 {
					k := thenEnd + 1
					for k < len(wat) && (wat[k] == ' ' || wat[k] == '\n' || wat[k] == '\t' || wat[k] == '\r') {
						k++
					}
					if k < len(wat) && wat[k] != ')' && !strings.HasPrefix(wat[k:], "(else") {
						bad = append(bad, k)
					}
				}
			}
		}
		next := off + 3
		rest = wat[next:]
		base = next
	}
	return bad
}

// skipIfCondition returns the offset just past an (if's condition: either one
// folded group or plain instructions up to the (then. ok is false for
// non-block ifs (a plain (if with no (then), which carry no trailing arm.
func skipIfCondition(wat string, ifOff int) (int, bool) {
	i := ifOff + len("(if")
	for i < len(wat) && (wat[i] == ' ' || wat[i] == '\n') {
		i++
	}
	if i >= len(wat) {
		return 0, false
	}
	if wat[i] == '(' {
		end := matchGroup(wat, i)
		if end < 0 {
			return 0, false
		}
		i = end + 1
	}
	for i < len(wat) && (wat[i] == ' ' || wat[i] == '\n') {
		i++
	}
	if strings.HasPrefix(wat[i:], "(then") {
		return i, true
	}
	return 0, false
}

// matchGroup returns the offset of the ')' closing the group at open, or -1.
func matchGroup(wat string, open int) int {
	depth := 0
	inString := false
	for i := open; i < len(wat); i++ {
		ch := wat[i]
		if inString {
			if ch == '\\' {
				i++
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
