package lang

// engineBoundaryWords names the host words the browser engine build cannot
// honor: every filesystem word plus console.arg and env.load (itself a file
// read). The pure lexical path words (file.join, file.base, file.extension,
// file.directory-of), read-line against the supplied stdin, console.args-count
// (always 0), and env.get (always Option.None) keep working there. Only the
// js build consults this set — boundary_js.go wires it, boundary_other.go
// keeps every other build unfenced.
var engineBoundaryWords = map[string]bool{
	"file.read":          true,
	"file.open":          true,
	"file.write":         true,
	"file.list":          true,
	"file.exists":        true,
	"file.size":          true,
	"file.free-space":    true,
	"file.make-folder":   true,
	"file.delete":        true,
	"file.delete-folder": true,
	"file.move":          true,
	"console.arg":        true,
	"env.load":           true,
}

// engineBoundary is the coded capability fence shared by the js build's word
// and stream seams: not-available-in-build naming the word, catchable inside
// the word's own Result like any file failure.
func engineBoundary(pos position, name string) error {
	return withHint(errorAt(pos, CodeNotAvailableInBuild, "%s is not available in this build", name),
		"this engine build has no filesystem or console arguments; run the program with zhex run instead")
}
