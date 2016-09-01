// +build !dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris

package secretdir

import "os"

// create creates a normal folder at the secret dir path
func (s *SecretDir) create() error {
	return os.MkdirAll(s.Dir, 0700)
}

// destroy removes the secret dir
func (s *SecretDir) destroy() error {
	return os.RemoveAll(s.Dir)
}

// MemoryUse returns the memory used by the SecretDir
func (s *SecretDir) MemoryUse() int {
	return 0
}
