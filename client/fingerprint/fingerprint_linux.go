// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package fingerprint

func initPlatformFingerprints(fps map[string]Factory) {
	// SETH setup cgroup fingerprinter
	fps["bridge"] = NewBridgeFingerprint
}
