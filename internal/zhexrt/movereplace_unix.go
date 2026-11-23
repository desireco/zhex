//go:build unix

package zhexrt

import "os"

// replaceRename renames oldpath to newpath replacing an existing file:
// POSIX rename(2) already replaces, so os.Rename is the whole story (the
// interpreter's movereplace_unix.go).
func replaceRename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}
