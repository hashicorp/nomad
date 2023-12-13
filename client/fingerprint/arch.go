// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"runtime"

	log "github.com/hashicorp/go-hclog"
)

// ArchFingerprint is used to fingerprint the architecture
type ArchFingerprint struct {
	StaticFingerprinter
	logger log.Logger
}

// NewArchFingerprint is used to create an OS fingerprint
func NewArchFingerprint(logger log.Logger) Fingerprint {
	f := &ArchFingerprint{logger: logger.Named("arch")}
	return f
}

func (f *ArchFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	resp.AddAttribute("cpu.arch", runtime.GOARCH)
	resp.Detected = true
	return nil
}
