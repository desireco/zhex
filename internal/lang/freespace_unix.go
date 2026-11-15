//go:build unix

package lang

import "syscall"

// freeSpaceBytes reports the bytes available to this process on the volume
// containing path: f_bavail (unprivileged) blocks of f_bsize bytes.
func freeSpaceBytes(path string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}
