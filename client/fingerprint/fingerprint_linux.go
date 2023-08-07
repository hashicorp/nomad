// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package fingerprint

func initPlatformFingerprints(fps map[string]Factory) {
	fps["cgroup"] = NewCgroupFingerprint
	fps["bridge"] = NewBridgeFingerprint
}
