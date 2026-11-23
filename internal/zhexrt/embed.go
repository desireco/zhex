package zhexrt

import "embed"

// rtFiles lists every runtime source copied into a compiled program's
// zhexrt/ directory. zhex compile --target=native emits these verbatim so
// the artifact is a self-contained Go module (no network, no replace
// directives); the accompanying test fails when a file is added without
// being listed here. Build-tagged files embed as raw bytes and their tags
// apply where the copies compile: the movereplace trio's union builds for
// every GOOS/GOARCH (embedding ignores the tags, so all three ship).
//
//go:embed int.go diag.go console.go value.go arith.go coll.go text.go files.go words.go json.go movereplace_unix.go movereplace_windows.go movereplace_other.go
var rtFS embed.FS

// Files returns the runtime sources keyed by file name.
func Files() map[string]string {
	out := map[string]string{}
	entries, err := rtFS.ReadDir(".")
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		if source, err := rtFS.ReadFile(entry.Name()); err == nil {
			out[entry.Name()] = string(source)
		}
	}
	return out
}
