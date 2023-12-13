// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_serviceOfSIToken(t *testing.T) {
	try := func(description, exp string) {
		tc := new(ConnectACLsE2ETest)
		result := tc.serviceofSIToken(description)
		require.Equal(t, exp, result)
	}

	try("", "")
	try("foobarbaz", "")
	try("_nomad_si [8b1a5d3f-7e61-4a5a-8a57-7e7ad91e63b6] [8b1a5d3f-7e61-4a5a-8a57-7e7ad91e63b6] [foo-service]", "foo-service")
}
