//go:build unix

package lang

import (
	"math"
	"syscall"
)

// freeSpaceBytes reports the bytes available to this process on the volume
// containing path: f_bavail (unprivileged) blocks of f_bsize bytes.
func freeSpaceBytes(path string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	avail := int64(st.Bavail) * int64(st.Bsize)
	if avail < 0 {
		// A volume absurd enough to overflow the product clamps instead of
		// wrapping negative.
		return math.MaxInt64, nil
	}
	return avail, nil
}
