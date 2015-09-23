// +build !windows

// Functions shared between linux/darwin.
package allocdir

import (
	"os"
)

func (d *AllocDir) linkOrCopy(src, dst string) error {
	// Attempt to hardlink.
	if err := os.Link(src, dst); err == nil {
		return nil
	}

	return fileCopy(src, dst)
}
