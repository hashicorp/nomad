// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fifo

import (
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/structs"
)

type fifoWorkload struct {
	queue.BaseWorkload
}

func newFifoWorkload(e *structs.Evaluation, j *structs.Job) *fifoWorkload {
	return &fifoWorkload{
		BaseWorkload: queue.NewBaseWorkload(e, j, queue.WorkloadStatusQueued),
	}
}
