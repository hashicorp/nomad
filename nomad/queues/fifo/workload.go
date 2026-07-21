// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fifo

import "github.com/hashicorp/nomad/nomad/structs"

type fifoWorkload struct {
	id            string
	counter       uint64
	eval          *structs.Evaluation
	waitOnRestore bool
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
