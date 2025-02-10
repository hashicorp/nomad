// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package nomad

import (
	"errors"

	"github.com/hashicorp/nomad/nomad/structs"
)

// Validate ensures job does not contain any task making use of the
// resources.numa block, which is only supported in Nomad Enterprise.
func (jobNumaHook) Validate(job *structs.Job) ([]error, error) {
	for _, tg := range job.TaskGroups {
		for _, task := range tg.Tasks {
			if task.Resources.NUMA.Requested() {
				return nil, errors.New("numa scheduling requires Nomad Enterprise")
			}
		}
	}
	return nil, nil
}

// Mutate does nothing.
func (jobNumaHook) Mutate(job *structs.Job) (*structs.Job, []error, error) {
	return job, nil, nil
}
