// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test/must"
)

var _ ReloadableFingerprint = &SELinuxFingerprint{}

func TestSELinuxFingerprint_active(t *testing.T) {
	ci.Parallel(t)

	f := NewSELinuxFingerprint(testlog.HCLogger(t))
	f.(*SELinuxFingerprint).state = func() (string, bool) {
		return "system_u:system_r:container_t:s0", true
	}

	var response FingerprintResponse
	must.NoError(t, f.Fingerprint(nil, &response))
	must.True(t, response.Detected)
	must.Eq(t, "true", response.Attributes[selinuxLabelTaskSocketsKey])
}

func TestSELinuxFingerprint_inactive(t *testing.T) {
	ci.Parallel(t)

	f := NewSELinuxFingerprint(testlog.HCLogger(t))
	f.(*SELinuxFingerprint).state = func() (string, bool) {
		return "", false
	}

	var response FingerprintResponse
	must.NoError(t, f.Fingerprint(nil, &response))
	must.False(t, response.Detected)

	// The attribute is named with an empty value, so a reload that turned
	// labeling off removes it from the node.
	value, exists := response.Attributes[selinuxLabelTaskSocketsKey]
	must.True(t, exists)
	must.Eq(t, "", value)
}
