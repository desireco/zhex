//go:build js

package lang

import "zhex/internal/zhexrt"

// shiftCountWide reports the browser engine build's 64-bit shift ruling
// (#231 rule 3): the engine is the 64-bit Int surface for shifts, so a
// nonnegative count of 64 or more fails int-shift-width at runtime instead
// of using the interpreter's full 128-bit width. The count is negative or
// fits a limb here, so only its low bits decide.
func shiftCountWide(n zhexrt.Int) bool {
	return n.Hi > 0 || n.Hi == 0 && n.Lo >= 64
}
