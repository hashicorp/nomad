// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"github.com/hashicorp/nomad/client/lib/cgutil"
)

func (f *CPUFingerprint) deriveReservableCores(cgroupParent string) []uint16 {
	// The cpuset cgroup manager is initialized (on linux), but not accessible
	// from the finger-printer. So we reach in and grab the information manually.
	// We may assume the hierarchy is already setup.
	cpuset, err := cgutil.GetCPUsFromCgroup(cgroupParent)
	if err != nil {
		f.logger.Warn("failed to detect set of reservable cores", "error", err)
		return nil
	}
	return cpuset
}
