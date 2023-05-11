// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build ent

package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestOperator_LicenseGet(t *testing.T) {
	testutil.Parallel(t)

	c, s, _ := makeACLClient(t, nil, nil)
	defer s.Stop()

	operator := c.Operator()

	// Make authenticated request.
	_, _, err := operator.LicenseGet(nil)
	must.NoError(t, err)

	// Make unauthenticated request.
	c.SetSecretID("")
	_, _, err = operator.LicenseGet(nil)
	must.ErrorContains(t, err, "403")
}
