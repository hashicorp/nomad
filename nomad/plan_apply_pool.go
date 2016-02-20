package nomad

import (
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// workerPoolSize is the size of the worker pool
	workerPoolSize = 2

	// workerPoolBufferSize is the size of the buffers used to push
	// request to the workers and to collect the responses. It should
	// be large enough just to keep things busy
	workerPoolBufferSize = 64
)

// EvaluatePool is used to have a pool of workers that are evaluating
// if a plan is valid. It can be used to parallelize the evaluation
// of a plan.
type EvaluatePool struct {
	workers int
	req     chan evaluateRequest
	res     chan evaluateResult
}

type evaluateRequest struct {
	snap   *state.StateSnapshot
	plan   *structs.Plan
	nodeID string
}

type evaluateResult struct {
	nodeID string
	fit    bool
	err    error
}

// NewEvaluatePool returns a pool of the given size.
func NewEvaluatePool(workers, bufSize int) *EvaluatePool {
	p := &EvaluatePool{
		workers: workers,
		req:     make(chan evaluateRequest, bufSize),
		res:     make(chan evaluateResult, bufSize),
	}
	for i := 0; i < workers; i++ {
		go p.run()
	}
	return p
}

// RequestCh is used to push requests
func (p *EvaluatePool) RequestCh() chan<- evaluateRequest {
	return p.req
}

// ResultCh is used to read the results as they are ready
func (p *EvaluatePool) ResultCh() <-chan evaluateResult {
	return p.res
}

// Shutdown is used to shutdown the pool
func (p *EvaluatePool) Shutdown() {
	close(p.req)
}

// run is a long running go routine per worker
func (p *EvaluatePool) run() {
	for req := range p.req {
		fit, err := evaluateNodePlan(req.snap, req.plan, req.nodeID)
		p.res <- evaluateResult{req.nodeID, fit, err}
	}
}
