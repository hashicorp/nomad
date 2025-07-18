// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
)

type SecretsPluginFingerprint struct {
	logger hclog.Logger
}

func NewPluginsSecretsFingerprint(logger hclog.Logger) Fingerprint {
	return &SecretsPluginFingerprint{
		logger: logger.Named("secrets_plugins"),
	}
}

func (s *SecretsPluginFingerprint) Fingerprint(request *FingerprintRequest, response *FingerprintResponse) error {
	// Add builtin secrets providers
	response.AddAttribute(fmt.Sprintf("plugins.secrets.%s.version", "nomad"), "1.0.0")
	response.AddAttribute(fmt.Sprintf("plugins.secrets.%s.version", "vault"), "1.0.0")
	response.Detected = true

	return nil
}

func (s *SecretsPluginFingerprint) Periodic() (bool, time.Duration) {
	return false, 0
}
