//go:build windows

package lang

import "golang.org/x/sys/windows"

// freeSpaceBytes reports the bytes available to this process on the volume
// containing path via GetDiskFreeSpaceEx, which accepts any existing path.
func freeSpaceBytes(path string) (int64, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var free, total, avail uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &free, &total, &avail); err != nil {
		return 0, err
	}
	return int64(free), nil
}
