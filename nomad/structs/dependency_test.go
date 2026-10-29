// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestDependency_CanonicalizeAndValidate(t *testing.T) {
	ci.Parallel(t)

	timeout := 10 * time.Minute
	d := &JobDependencies{
		Timeout: &timeout,
		Jobs: []*JobDependency{{
			Name: "service-123",
		}},
	}
	d.Canonicalize()

	must.Eq(t, JobDependencyComplete, d.Jobs[0].Status)
	must.NoError(t, d.Validate())
}

func TestDependency_CopyDeep(t *testing.T) {
	ci.Parallel(t)

	timeout := 10 * time.Minute
	d := &JobDependencies{
		Timeout: &timeout,

		Jobs: []*JobDependency{{
			Name:   "service-123",
			Status: JobDependencyComplete,
		}},
	}

	copy := d.Copy()
	must.True(t, d.Equal(copy))
	must.True(t, d.Jobs[0] != copy.Jobs[0])

	copy.Jobs[0].Status = JobDependencyRunning
	must.Eq(t, JobDependencyComplete, d.Jobs[0].Status)
}

func TestJob_CopyIncludesDependencies(t *testing.T) {
	ci.Parallel(t)

	timeout := 10 * time.Minute
	j := &Job{
		ID:        "job-id",
		Name:      "job-name",
		Namespace: DefaultNamespace,
		Type:      JobTypeService,
		TaskGroups: []*TaskGroup{{
			Name:  "group",
			Tasks: []*Task{{Name: "task", Driver: "raw_exec", Config: map[string]any{"command": "/bin/date"}}},
		}},
		Dependencies: &JobDependencies{
			Timeout: &timeout,

			Jobs: []*JobDependency{{
				Name:   "service-123",
				Status: JobDependencyComplete,
			}},
		},
	}

	copy := j.Copy()
	must.Eq(t, j.Dependencies, copy.Dependencies)
	must.True(t, j.Dependencies != copy.Dependencies)
	must.True(t, j.Dependencies.Jobs[0] != copy.Dependencies.Jobs[0])
}
