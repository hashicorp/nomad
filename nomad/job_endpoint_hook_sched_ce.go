// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package nomad

import (
	"errors"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (jobSchedHook) Validate(job *structs.Job) ([]error, error) {
	for _, tg := range job.TaskGroups {
		for _, task := range tg.Tasks {
			if task.Schedule != nil {
				return nil, errors.New("task schedules requires Nomad Enterprise")
			}
		}
	}
	return nil, nil
}
