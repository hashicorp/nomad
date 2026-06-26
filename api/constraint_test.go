// Copyright IBM Corp. 2015, 2026
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

func TestCompose_Dependencies(t *testing.T) {
	testutil.Parallel(t)

	d := NewDependency("10m", "reject", JobDepdendency{Name: "service-123", Status: "completed"})
	d.Canonicalize()

	must.Eq(t, "10m", d.Timeout)
	must.Eq(t, "reject", d.ActionOnTimeout)
	must.Len(t, 1, d.Jobs)
	must.Eq(t, "service-123", d.Jobs[0].Name)
	must.Eq(t, "completed", d.Jobs[0].Status)
	must.NoError(t, d.Validate())

	copy := d.Copy()
	must.Eq(t, d, copy)
	must.True(t, d.Jobs[0] != copy.Jobs[0])
}

func TestCompose_Dependencies_DefaultsAndValidation(t *testing.T) {
	testutil.Parallel(t)

	d := &Dependency{
		Timeout: "10m",
		Jobs: []*JobDependency{{
			Name: "service-123",
		}},
	}
	d.Canonicalize()

	must.Eq(t, "reject", d.ActionOnTimeout)
	must.Eq(t, "completed", d.Jobs[0].Status)
	must.NoError(t, d.Validate())

	bad := &Dependency{
		Timeout:         "10m",
		ActionOnTimeout: "continue",
		Jobs:            []*JobDependency{{Name: "service-123", Status: "completed"}},
	}
	must.Error(t, bad.Validate())
}
