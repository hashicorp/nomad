// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/clientsockets"
)

const (
	selinuxLabelTaskSocketsKey = "nomad.selinux.label_task_sockets"
)

// SELinuxFingerprint reports whether the client creates the sockets of confined
// tasks with the container SELinux type (client.selinux.label_task_sockets).
type SELinuxFingerprint struct {
	StaticFingerprinter
	logger hclog.Logger

	// state reports the task socket label and whether labeling is active.
	state func() (label string, active bool)
}

func NewSELinuxFingerprint(logger hclog.Logger) Fingerprint {
	return &SELinuxFingerprint{
		logger: logger.Named("selinux"),
		state:  clientsockets.Fingerprint,
	}
}

// Reload is a no-op but implements ReloadableFingerprint so the fingerprinter
// re-runs after a config reload, where Client.Reload may have changed the
// labeling state.
func (f *SELinuxFingerprint) Reload() {}

func (f *SELinuxFingerprint) Fingerprint(_ *FingerprintRequest, resp *FingerprintResponse) error {
	_, active := f.state()
	if !active {
		// The client deletes an attribute only when the response names it with
		// an empty value. This clears it after a reload turns labeling off.
		resp.RemoveAttribute(selinuxLabelTaskSocketsKey)
		return nil
	}
	resp.AddAttribute(selinuxLabelTaskSocketsKey, "true")
	resp.Detected = true
	return nil
}
