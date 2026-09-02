// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fifo

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

type fifoWorkload struct {
	id            string
	counter       uint64
	eval          *structs.Evaluation
	waitOnRestore bool
	status        string
	description   string
}

func newFifoWorkload(e *structs.Evaluation) *fifoWorkload {
	return &fifoWorkload{
		id:   e.ID,
		eval: e,
	}
}

func (f *fifoWorkload) GetEval() *structs.Evaluation {
	return f.eval
}

func (f *fifoWorkload) SetEval(e *structs.Evaluation) {
	f.eval = e
}

func (f *fifoWorkload) WaitOnRestore() bool {
	return f.waitOnRestore
}

func (f *fifoWorkload) SetStatus(s, description string) {
	f.status = s
	f.description = description
}
func (f *fifoWorkload) GetStatus() string {
	if f.description != "" {
		return fmt.Sprintf("%s (%s)", f.status, f.description)
	}
	return f.status
}
