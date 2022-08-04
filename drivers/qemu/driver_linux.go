//go:build linux
// +build linux

package qemu

const (
	// https://man7.org/linux/man-pages/man7/unix.7.html
	maxSocketPathLen = 108
)
