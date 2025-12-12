// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

func initPlatformFingerprints(fps map[string]Factory) {
	fps["cgroup"] = NewCgroupFingerprint
	fps["bridge"] = NewBridgeFingerprint
}
