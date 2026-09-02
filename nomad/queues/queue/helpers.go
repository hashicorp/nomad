// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

func CmpWaitOnRestore(a, b Workload) int {
	if a.WaitOnRestore() && !b.WaitOnRestore() {
		return -1
	} else if !a.WaitOnRestore() && b.WaitOnRestore() {
		return 1
	}
	return 0
}
