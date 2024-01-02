// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/shoenig/go-landlock"
)

const (
	landlockKey = "kernel.landlock"
)

// LandlockFingerprint is used to fingerprint the kernel landlock feature.
type LandlockFingerprint struct {
	StaticFingerprinter
	logger   hclog.Logger
	detector func() (int, error)
}

func NewLandlockFingerprint(logger hclog.Logger) Fingerprint {
	return &LandlockFingerprint{
		logger:   logger.Named("landlock"),
		detector: landlock.Detect,
	}
}

func (f *LandlockFingerprint) Fingerprint(_ *FingerprintRequest, resp *FingerprintResponse) error {
	version, err := f.detector()
	if err != nil {
		f.logger.Warn("failed to fingerprint kernel landlock feature", "error", err)
		version = 0
	}
	switch version {
	case 0:
		// do not set any attribute
	default:
		v := fmt.Sprintf("v%d", version)
		resp.AddAttribute(landlockKey, v)
	}
	return nil
}
