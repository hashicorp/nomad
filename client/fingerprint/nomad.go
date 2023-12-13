// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"strconv"

	log "github.com/hashicorp/go-hclog"
)

// NomadFingerprint is used to fingerprint the Nomad version
type NomadFingerprint struct {
	StaticFingerprinter
	logger log.Logger
}

// NewNomadFingerprint is used to create a Nomad fingerprint
func NewNomadFingerprint(logger log.Logger) Fingerprint {
	f := &NomadFingerprint{logger: logger.Named("nomad")}
	return f
}

func (f *NomadFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	resp.AddAttribute("nomad.advertise.address", req.Node.HTTPAddr)
	resp.AddAttribute("nomad.version", req.Config.Version.VersionNumber())
	resp.AddAttribute("nomad.revision", req.Config.Version.Revision)
	resp.AddAttribute("nomad.service_discovery", strconv.FormatBool(req.Config.NomadServiceDiscovery))
	resp.Detected = true
	return nil
}
