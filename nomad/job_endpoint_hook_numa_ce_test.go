// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func Test_jobNumaHook_Validate(t *testing.T) {
	ci.Parallel(t)

	// ce does not allow numa block
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Resources.NUMA = &structs.NUMA{
		Affinity: "require",
	}

	hook := jobNumaHook{}
	warnings, err := hook.Validate(job)
	must.SliceEmpty(t, warnings)
	must.EqError(t, err, "numa scheduling requires Nomad Enterprise")
}

func Test_jobNumaHook_Mutate(t *testing.T) {
	ci.Parallel(t)

	// does not get mutated in CE
	job := mock.Job()

	hook := jobNumaHook{}
	result, warns, err := hook.Mutate(job)
	must.NoError(t, err)
	must.SliceEmpty(t, warns)
	must.Eq(t, job, result)
}
