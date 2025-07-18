// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test/must"
)

func TestPluginsSecretsFingerprint(t *testing.T) {
	fp := NewPluginsSecretsFingerprint(testlog.HCLogger(t))

	resp := FingerprintResponse{}
	err := fp.Fingerprint(&FingerprintRequest{}, &resp)
	must.NoError(t, err)
	must.True(t, resp.Detected)
	must.MapContainsKeys(t, resp.Attributes, []string{
		"plugins.secrets.nomad.version",
		"plugins.secrets.vault.version",
	})
}
