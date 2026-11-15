//go:build !unix && !windows

package lang

import "os"

// replaceRename answers unsupported Go ports (js, plan9, wasip1) so the
// package still builds; the plain rename keeps today's no-replace behavior.
// shortcut: no replace-capable rename exists on these ports, drop when one lands
func replaceRename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}
