//go:build unix

package lang

import "os"

// replaceRename renames oldpath to newpath replacing an existing file:
// POSIX rename(2) already replaces, so os.Rename is the whole story.
func replaceRename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}
