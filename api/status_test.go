// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestStatus_Leader(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	status := c.Status()

	// Query for leader status should return a result
	out, err := status.Leader()
	must.NoError(t, err)
	must.NotEq(t, "", out)
}
