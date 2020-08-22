package fileperms

import (
	"fmt"
	"os"
)

// mode represents a simplified octal representation of a Unix file permissions
// mode interface provided by os.FileMode.
//
// This package is intended to prevent accidental incorrect file permissions caused
// by erroneous octal representations of permissions bits in Go's decimal base. A
// linter can be configured to assert calls to functions such as os.OpenFile,
// os.Chmod, os.File.Chmod, etc. are passing a parameter of type fileperms.mode.
// Anything else will be flagged and must be changed. By not exporting the mode
// type, getting past the linter means funneling through one of the exported const
// values in this package.
//
// For example, the code `os.Chmod(755)` produces file permissions "-wxrw--wt",
// whereas the intended octal form `os.Chmod(0755)` produces "rwxr-xr-x".
type mode = os.FileMode

// 0) no permission
// 1) execute
// 2) write
// 3) execute + write
// 4) read
// 5) read + execute
// 6) read + write
// 7) read + write + execute
const (
	Oct555 mode = 0555
	Oct600 mode = 0600
	Oct610 mode = 0610
	Oct655 mode = 0655
	Oct660 mode = 0660
	Oct666 mode = 0666
	Oct755 mode = 0755
	Oct777 mode = 0777
)

// Reduce returns the lowered permissions of m by the mask.
func Reduce(m, mask mode) mode {
	return m & mask
}

// Escalate returns the raised permissions of m by the mask.
func Escalate(m, mask mode) mode {
	return m | mask
}

// Sufficient returns true if m is at least as permissive as the mask.
func Sufficient(m, mask mode) bool {
	return Reduce(m, mask) == mask
}

// Check returns true if the os.FileMode permission bits of f match the
// expected permissions in exp.
func Check(f *os.File, exp mode) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}

	perm := info.Mode().Perm()
	if perm != exp {
		return fmt.Errorf("file mode expected %o, got %o", exp, perm)
	}

	return nil
}
