// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package envoy

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestEnvoy_PortLabel(t *testing.T) {
	ci.Parallel(t)

	for _, tc := range []struct {
		prefix  string
		service string
		suffix  string
		exp     string
	}{
		{prefix: structs.ConnectProxyPrefix, service: "foo", suffix: "", exp: "connect-proxy-foo"},
		{prefix: structs.ConnectMeshPrefix, service: "bar", exp: "connect-mesh-bar"},
	} {
		test := fmt.Sprintf("%s_%s_%s", tc.prefix, tc.service, tc.suffix)
		t.Run(test, func(t *testing.T) {
			result := PortLabel(tc.prefix, tc.service, tc.suffix)
			require.Equal(t, tc.exp, result)
		})
	}
}
