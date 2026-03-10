// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import "golang.org/x/sys/windows"

// IsPrivilegedProcess checks if current process is a privileged windows process
func IsPrivilegedProcess() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
