//go:build js

package lang

// wordBoundary fences the engine boundary words before their real
// implementations run, so the browser engine never reaches Go's js-port
// syscall errors; the failure lands inside the word's own Result.
func wordBoundary(x *expression) error {
	if engineBoundaryWords[x.text] {
		return engineBoundary(x.pos, x.text)
	}
	return nil
}

// streamBoundary fences the loop-only file.stream the same way; its failures
// are hard runtime diagnostics, like every file.stream failure.
func streamBoundary(pos position) error {
	return engineBoundary(pos, "file.stream")
}
