// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
)

type CgroupFingerprint struct {
	StaticFingerprinter
	logger hclog.Logger
}

func NewCgroupFingerprint(logger hclog.Logger) Fingerprint {
	return &CgroupFingerprint{
		logger: logger.Named("cgroup"),
	}
}

func (f *CgroupFingerprint) Fingerprint(request *FingerprintRequest, response *FingerprintResponse) error {
	const versionKey = "os.cgroups.version"
	switch cgroupslib.GetMode() {
	case cgroupslib.CG1:
		response.AddAttribute(versionKey, "1")
		f.logger.Debug("detected cgroups", "version", "1")
	case cgroupslib.CG2:
		response.AddAttribute(versionKey, "2")
		f.logger.Debug("detected cgroups", "version", "2")
	}
	return nil
}
