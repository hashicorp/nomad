// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package scheduler

/*
type BatchScheduler struct {
	dependencyChecker DependencyChecker
	GenericScheduler
}

type DependencyChecker interface {
	DependenciesMeet(job *structs.Job) (bool, error)
}

// NewBatchScheduler is a factory function to instantiate a new batch scheduler
func NewBatchScheduler(logger log.Logger, eventsCh chan<- interface{}, state sstructs.State,
	dc DependencyChecker, planner sstructs.Planner) sstructs.Scheduler {
	s := &BatchScheduler{
		GenericScheduler: GenericScheduler{
			logger:   logger.Named("batch_sched"),
			eventsCh: eventsCh,
			state:    state,
			planner:  planner,
			batch:    true,
		},
		dependencyChecker: dc,
	}

	return s
}

func (s *BatchScheduler) evaluateDependencies(ws memdb.WatchSet) (bool, error) {

	var mErr multierror.Error
	ready := true

	for _, dep := range s.job.Dependencies {
		s.logger.Debug("watching dependency job for changes", "dependency_job_id", dep.Job)

		job, err := s.GenericScheduler.state.JobByID(ws, s.job.Namespace, dep.Job)
		if err != nil {
			mErr = *multierror.Append(&mErr, err)
		}

		if job == nil || job.Status != dep.Output {
			ready = false
		}
	}

	return ready, mErr.ErrorOrNil()
}
*/
