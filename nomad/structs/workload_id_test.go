// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestWorkloadIdentity_Equal(t *testing.T) {
	ci.Parallel(t)

	var orig *WorkloadIdentity

	newWI := orig.Copy()
	must.Equal(t, orig, newWI)

	orig = &WorkloadIdentity{}
	must.NotEqual(t, orig, newWI)

	newWI = &WorkloadIdentity{}
	must.Equal(t, orig, newWI)

	orig.Env = true
	must.NotEqual(t, orig, newWI)

	newWI.Env = true
	must.Equal(t, orig, newWI)

	newWI.File = true
	must.NotEqual(t, orig, newWI)
}
