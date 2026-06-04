package dependency

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/structs"
	sstructs "github.com/hashicorp/nomad/scheduler/structs"
)

type dependency struct {
	cancelFunc context.CancelFunc
	job        *structs.Job
}

type Coordinator struct {
	logger hclog.Logger
	state  sstructs.State
	l      sync.RWMutex

	dependencies map[string]*dependency
	evalBroker   *nomad.EvalBroker
}

func (c *Coordinator) AddDependecy(ctx context.Context, eval *structs.Evaluation) {

	job, err := c.state.JobByID(nil, eval.Namespace, eval.ID)
	if err != nil {
		c.logger.Error("failed to get job by ID", "error", err)
		return
	}

	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(10*time.Minute))
	c.dependencies[eval.JobID] = &dependency{
		cancelFunc: cancel,
		job:        job,
	}

	go c.waitForDependency(ctx, eval)
}

func (c *Coordinator) waitForDependency(ctx context.Context, eval *structs.Evaluation) {

	for {
		ws := memdb.NewWatchSet()
		dj := []*structs.Job{}

		for _, dep := range c.dependencies[eval.JobID].job.Dependencies {
			j, err := c.state.JobByID(ws, eval.Namespace, dep.Job)
			if err != nil {
				c.logger.Error("failed to get job by ID", "error", err)
			}
			dj = append(dj, j)
		}

		select {
		case <-ws.WatchCh(ctx):
			ready, err := c.verifyDependency(eval.JobID, dj...)
			if err != nil {
				c.logger.Error("failed to verify dependency", "error", err)
				continue
			}

			if ready {

				c.l.Lock()
				defer c.l.Unlock()

				c.evalBroker.Enqueue(eval)
				delete(c.dependencies, eval.ID)
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func (c *Coordinator) verifyDependency(dependantJob string, dependeeJob ...*structs.Job) (bool, error) {
	return true, nil
}
