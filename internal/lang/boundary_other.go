//go:build !js

package lang

// The interpreter's own builds never fence a word; only the js engine build
// carries the capability boundary (boundary_js.go).
func wordBoundary(*expression) error { return nil }

func streamBoundary(position) error { return nil }
