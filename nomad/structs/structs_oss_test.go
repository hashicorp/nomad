// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestNamespace_Validate_Oss(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name        string
		namespace   *Namespace
		expectedErr string
	}{
		{
			name: "node pool config not allowed",
			namespace: &Namespace{
				Name: "test",
				NodePoolConfiguration: &NamespaceNodePoolConfiguration{
					Default: "dev",
				},
			},
			expectedErr: "unlicensed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.namespace.Validate()
			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
			} else {
				must.NoError(t, err)
			}
		})
	}
}
