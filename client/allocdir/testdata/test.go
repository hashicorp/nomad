package allocdir

import (
	"os"
	"syscall"
)

// LinkDir hardlinks src to dst. The src and dst must be on the same filesystem.
func LinkDir(src, dst string) error {
	return syscall.Link(src, dst)
}

// UnlinkDir removes a directory link.
func UnlinkDir(dir string) error {
	return syscall.Unlink(dir)
}

// CreateSecretDir creates the secrets dir folder at the given path
func CreateSecretDir(dir string) error {
	return os.MkdirAll(dir, 0777)
}

// RemoveSecretDir removes the secrets dir folder
func RemoveSecretDir(dir string) error {
	return os.RemoveAll(dir)
}
