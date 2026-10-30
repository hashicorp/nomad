// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

type BaseWorkload struct {
	// job is the job the user cares about. its NamespacedID() is used as the
	// BaseWorkload.ID()
	job *structs.Job

	// eval is the eval that will be submitted to eval broker.
	eval *structs.Evaluation

	status string

	description string
}

func NewBaseWorkload(e *structs.Evaluation, j *structs.Job, status string) BaseWorkload {
	return BaseWorkload{
		job:    j,
		eval:   e,
		status: status,
	}
}

func (b *BaseWorkload) ID() structs.NamespacedID {
	return b.job.NamespacedID()
}

func (b *BaseWorkload) Job() *structs.Job {
	return b.job
}

func (b *BaseWorkload) Eval() *structs.Evaluation {
	return b.eval
}

func (b *BaseWorkload) SetEval(e *structs.Evaluation) {
	b.eval = e
}

func (b *BaseWorkload) JobVersion() uint64 {
	return b.job.Version
}

func (b *BaseWorkload) Status() string {
	if b.description != "" {
		return fmt.Sprintf("%s (%s)", b.status, b.description)
	}
	return b.status
}

func (b *BaseWorkload) SetStatus(status, description string) {
	b.status = status
	b.description = description
}
