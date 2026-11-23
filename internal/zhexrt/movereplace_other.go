//go:build !unix && !windows

package zhexrt

import "os"

// replaceRename answers unsupported Go ports (js, plan9, wasip1) so the
// runtime still builds; the plain rename keeps today's no-replace behavior
// (the interpreter's movereplace_other.go).
// shortcut: no replace-capable rename exists on these ports, drop when one lands
func replaceRename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}
