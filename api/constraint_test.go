// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

var timeoutDuration = 10 * time.Minute

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

	d := NewJobDependencies("10m", "reject", &JobDependency{Name: "service-123", Status: JobDependencyComplete})
	d.Canonicalize()

	must.Eq(t, &timeoutDuration, d.Timeout)
	must.Len(t, 1, d.Jobs)
	must.Eq(t, "service-123", d.Jobs[0].Name)
	must.Eq(t, JobDependencyComplete, d.Jobs[0].Status)
	must.NoError(t, d.Validate())

	copy := d.Copy()
	must.Eq(t, d, copy)
	must.True(t, d.Jobs[0] != copy.Jobs[0])
}

func TestCompose_Dependencies_DefaultsAndValidation(t *testing.T) {
	testutil.Parallel(t)

	d := &JobDependencies{
		Timeout: &timeoutDuration,
		Jobs: []*JobDependency{{
			Name: "service-123",
		}},
	}
	d.Canonicalize()

	must.Eq(t, JobDependencyComplete, d.Jobs[0].Status)
	must.NoError(t, d.Validate())

	bad := &JobDependencies{
		Timeout: &timeoutDuration,
		Jobs:    []*JobDependency{{Name: "service-123", Status: "unexpectedState"}},
	}
	must.Error(t, bad.Validate())
}
