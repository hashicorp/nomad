// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package users

import "net"

// ListenTaskSocket creates a unix socket that tasks connect to. SELinux labels
// only exist on Linux, so selinuxLabel has no effect here.
func ListenTaskSocket(path string, _ bool) (net.Listener, error) {
	return net.Listen("unix", path)
}

// TaskSocketLabel returns an empty label, as SELinux only exists on Linux.
func TaskSocketLabel() (string, error) {
	return "", nil
}
