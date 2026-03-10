// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package winsvc

// IsPrivilegedProcess checks if current process is a privileged windows process
func IsPrivilegedProcess() bool {
	return false
}
