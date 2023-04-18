//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd
// +build !linux,!darwin,!freebsd,!netbsd,!openbsd

package qemu

const (
	// Don't enforce any path limit.
	maxSocketPathLen = 0
)
