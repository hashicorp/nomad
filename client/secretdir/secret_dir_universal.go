// +build !dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris

package secretdir

import "os"

// create creates a normal folder at the secret dir path
func (s *SecretDir) create(sizeMB int) error {
	return os.MkdirAll(s.Dir, 0700)
}

// destroy removes the secret dir
func (s *SecretDir) destroy() error {
	return os.RemoveAll(s.Dir)
}
