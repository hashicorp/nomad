package queue

import "github.com/hashicorp/nomad/nomad/structs"

type BaseWorkload struct {
	// id is the unique identifier used in a WorkloadQueue
	id structs.NamespacedID

	// eval is the eval that will be submitted to eval broker.
	eval *structs.Evaluation

	jobVersion uint64

	waitOnRestore bool
}

func NewBaseWorkload(e *structs.Evaluation, j *structs.Job) BaseWorkload {
	return BaseWorkload{
		id:            j.NamespacedID(),
		eval:          e,
		jobVersion:    j.Version,
		waitOnRestore: false,
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

func (b *BaseWorkload) WaitOnRestore() bool {
	return b.waitOnRestore
}

func (b *BaseWorkload) SetWaitOnRestore(w bool) {
	b.waitOnRestore = w
}
