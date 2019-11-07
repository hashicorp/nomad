// +build !windows

package loader

import "os"

// executable Checks to see if the file is executable by anyone.
func executable(path string, f os.FileInfo) bool {
	return f.Mode().Perm()&0111 != 0
}
