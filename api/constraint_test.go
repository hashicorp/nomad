// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestCompose_Constraints(t *testing.T) {
	testutil.Parallel(t)

	c := NewConstraint("kernel.name", "=", "darwin")
	expect := &Constraint{
		LTarget: "kernel.name",
		RTarget: "darwin",
		Operand: "=",
	}
	must.Eq(t, expect, c)
}
