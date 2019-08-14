package getter

import (
	"io"
	"os"
)

// copyReader copies from an io.Reader into a file, using umask to create the dst file
func copyReader(dst string, src io.Reader, fmode, umask os.FileMode) (int64, error) {
	dstF, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, fmode)
	if err != nil {
		return 0, err
	}
	defer dstF.Close()

	count, err := io.Copy(dstF, src)
	if err != nil {
		return 0, err
	}

	// Explicitly chmod; the process umask is unconditionally applied otherwise.
	// We'll mask the mode with our own umask, but that may be different than
	// the process umask
	return count, os.Chmod(dst, mode(fmode, umask))
}

// copyFile copies a file in chunks from src path to dst path, using umask to create the dst file
func copyFile(dst, src string, fmode, umask os.FileMode) (int64, error) {
	srcF, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer srcF.Close()

	dstF, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, fmode)
	if err != nil {
		return 0, err
	}
	defer dstF.Close()

	count, err := io.Copy(dstF, srcF)
	if err != nil {
		return 0, err
	}

	// Explicitly chmod; the process umask is unconditionally applied otherwise.
	// We'll mask the mode with our own umask, but that may be different than
	// the process umask
	err = os.Chmod(dst, mode(fmode, umask))
	return count, err
}
