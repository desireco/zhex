//go:build !unix && !windows

package lang

import "errors"

// freeSpaceBytes answers unsupported Go ports (js, plan9, wasip1) so the
// package still builds; the failure classifies through the normal taxonomy.
// shortcut: no volume query exists on these ports, drop when one lands
func freeSpaceBytes(string) (int64, error) {
	return 0, errors.New("free space is not supported on this platform")
}
