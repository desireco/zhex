//go:build windows

package zhexrt

import (
	"syscall"
	"unsafe"
)

// moveFileExW is kernel32's MoveFileEx; the stdlib syscall surface does not
// export it, and a generated program carries no x/sys dependency.
var moveFileExW = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

// replaceRename renames oldpath to newpath replacing an existing file:
// os.Rename on Windows moves without the replace flag, so
// MOVEFILE_REPLACE_EXISTING must be passed explicitly (the interpreter's
// movereplace_windows.go). Not exercised by this suite — no Windows host.
func replaceRename(oldpath, newpath string) error {
	from, err := syscall.UTF16PtrFromString(oldpath)
	if err != nil {
		return err
	}
	to, err := syscall.UTF16PtrFromString(newpath)
	if err != nil {
		return err
	}
	const movefileReplaceExisting = 0x1
	r1, _, e1 := moveFileExW.Call(uintptr(unsafe.Pointer(from)), uintptr(unsafe.Pointer(to)), movefileReplaceExisting)
	if r1 == 0 {
		return e1
	}
	return nil
}
