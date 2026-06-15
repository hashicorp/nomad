// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/v2/ci"
	"github.com/hashicorp/nomad/v2/helper/testlog"
	"github.com/hashicorp/nomad/v2/nomad/structs"
)

func TestSignalFingerprint(t *testing.T) {
	ci.Parallel(t)

	fp := NewSignalFingerprint(testlog.HCLogger(t))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	response := assertFingerprintOK(t, fp, node)
	assertNodeAttributeContains(t, response.Attributes, "os.signals")
}
