// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"testing"

	"github.com/hashicorp/nomad/v2/ci"
	"github.com/hashicorp/nomad/v2/client/config"
	"github.com/hashicorp/nomad/v2/helper/testlog"
	"github.com/hashicorp/nomad/v2/nomad/structs"
)

func TestArchFingerprint(t *testing.T) {
	ci.Parallel(t)

	f := NewArchFingerprint(testlog.HCLogger(t))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	assertNodeAttributeContains(t, response.Attributes, "cpu.arch")
}
