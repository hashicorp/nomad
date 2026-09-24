// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

type BaseWorkload struct {
	// id is the unique identifier used in a WorkloadQueue
	id structs.NamespacedID

	// eval is the eval that will be submitted to eval broker.
	eval *structs.Evaluation

	jobVersion uint64

	status string

	description string

	waitOnRestore bool
}

func NewBaseWorkload(e *structs.Evaluation, j *structs.Job, status string) BaseWorkload {
	return BaseWorkload{
		id:            j.NamespacedID(),
		eval:          e,
		jobVersion:    j.Version,
		waitOnRestore: false,
		status:        status,
	}
}

func (b *BaseWorkload) ID() structs.NamespacedID {
	return b.id
}

func (b *BaseWorkload) Eval() *structs.Evaluation {
	return b.eval
}

func (b *BaseWorkload) SetEval(e *structs.Evaluation) {
	b.eval = e
}

func (b *BaseWorkload) JobVersion() uint64 {
	return b.jobVersion
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

func (b *BaseWorkload) WaitOnRestore() bool {
	return b.waitOnRestore
}

func (b *BaseWorkload) SetWaitOnRestore(w bool) {
	b.waitOnRestore = w
}
