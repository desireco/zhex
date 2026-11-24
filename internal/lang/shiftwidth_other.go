//go:build !js

package lang

import "zhex/internal/zhexrt"

// The interpreter's own builds never fence a shift count; only the js
// engine build carries the 64-bit width ruling (shiftwidth_js.go). Here
// the shared 128-bit body runs: counts 64..127 shift the full width and
// counts past it take the clamped mathematical result.
func shiftCountWide(zhexrt.Int) bool { return false }
