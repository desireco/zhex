//go:build windows

package lang

import "golang.org/x/sys/windows"

// replaceRename renames oldpath to newpath replacing an existing file:
// os.Rename calls MoveFileEx without MOVEFILE_REPLACE_EXISTING, so the
// replace flag must be passed explicitly.
func replaceRename(oldpath, newpath string) error {
	from, err := windows.UTF16PtrFromString(oldpath)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(newpath)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING)
}
