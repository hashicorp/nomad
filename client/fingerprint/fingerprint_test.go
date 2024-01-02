// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

// This file contains helper methods for testing fingerprinters

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func assertFingerprintOK(t *testing.T, fp Fingerprint, node *structs.Node) *FingerprintResponse {
	request := &FingerprintRequest{Config: new(config.Config), Node: node}
	var response FingerprintResponse
	err := fp.Fingerprint(request, &response)
	require.NoError(t, err)

	require.NotEmpty(t, response.Attributes, "Failed to apply node attributes")

	return &response
}

func assertNodeAttributeContains(t *testing.T, nodeAttributes map[string]string, attribute string) {
	require.NotNil(t, nodeAttributes, "expected an initialized map for node attributes")

	require.Contains(t, nodeAttributes, attribute)
	require.NotEmpty(t, nodeAttributes[attribute])
}

func assertNodeAttributeEquals(t *testing.T, nodeAttributes map[string]string, attribute string, expected string) {
	require.NotNil(t, nodeAttributes, "expected an initialized map for node attributes")

	require.Contains(t, nodeAttributes, attribute)
	require.Equal(t, expected, nodeAttributes[attribute])
}

func assertNodeLinksContains(t *testing.T, nodeLinks map[string]string, link string) {
	require.NotNil(t, nodeLinks, "expected an initialized map for node links")

	require.Contains(t, nodeLinks, link)
	require.NotEmpty(t, nodeLinks[link])
}
