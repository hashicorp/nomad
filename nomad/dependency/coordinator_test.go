// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dependency

import (
	"slices"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/dependency/loop_detection"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

// mockLoopDetector implements loopDetector for testing
type mockLoopDetector struct {
	nodes                map[string][]string
	circularDeps         map[string]bool
	shouldReturnError    bool
	shouldReturnNotFound bool
}

func newMockLoopDetector() *mockLoopDetector {
	return &mockLoopDetector{
		nodes:        make(map[string][]string),
		circularDeps: make(map[string]bool),
	}
}

func (m *mockLoopDetector) AddNodes(dependantJob string, dependeeJob ...string) error {
	if m.shouldReturnError {
		return loop_detection.ErrCircularDependency
	}
	m.nodes[dependantJob] = dependeeJob
	return nil
}

func (m *mockLoopDetector) RemoveNode(dependantJob string) error {
	if m.shouldReturnNotFound {
		return loop_detection.ErrNodeNotFound
	}
	// Check if this job is a dependency of any other job BEFORE attempting to remove
	for _, deps := range m.nodes {
		if slices.Contains(deps, dependantJob) {
			return loop_detection.ErrNodeIsDependency
		}
	}
	// Try to remove the node if it exists
	if _, hasDeps := m.nodes[dependantJob]; hasDeps {
		delete(m.nodes, dependantJob)
	}
	return nil
}

func (m *mockLoopDetector) CreatesCircularDependency(dependantJob string, dependeeJob ...string) bool {
	return m.circularDeps[dependantJob]
}

// Table-driven tests for Coordinator public methods

func TestCoordinator_AddDependency(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		name         string
		setupState   func(*state.StateStore, *mockLoopDetector) *structs.Job
		expectError  bool
		expectStored bool
	}{
		{
			name:         "nil inputs",
			setupState:   nil,
			expectError:  false,
			expectStored: false,
		},
		{
			name: "no dependencies",
			setupState: func(s *state.StateStore, _ *mockLoopDetector) *structs.Job {
				job := mock.Job()
				job.Namespace = structs.DefaultNamespace
				must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 1, nil, job))
				return job
			},
			expectError:  false,
			expectStored: false,
		},
		{
			name: "dependencies are tracked",
			setupState: func(s *state.StateStore, _ *mockLoopDetector) *structs.Job {
				job := mock.Job()
				job.Namespace = structs.DefaultNamespace
				job.Dependencies = &structs.JobDependencies{
					Jobs: []*structs.JobDependency{
						{
							Name:   "job-dependency",
							Status: structs.JobDependencyRunning,
						},
					},
				}
				must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 1, nil, job))
				return job
			},
			expectError:  false,
			expectStored: true,
		},
		{
			name: "loop detector error",
			setupState: func(s *state.StateStore, loopDetector *mockLoopDetector) *structs.Job {
				loopDetector.shouldReturnError = true
				job := mock.Job()
				job.Namespace = structs.DefaultNamespace
				job.Dependencies = &structs.JobDependencies{
					Jobs: []*structs.JobDependency{
						{
							Name:   "job-dependency",
							Status: structs.JobDependencyRunning,
						},
					},
				}
				must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 1, nil, job))
				return job
			},
			expectError:  true,
			expectStored: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loopDetector := newMockLoopDetector()
			coord := NewCoordinator(hclog.NewNullLogger(), loopDetector)

			var stateStore *state.StateStore
			var job *structs.Job
			if tt.setupState != nil {
				stateStore = state.TestStateStore(t)
				job = tt.setupState(stateStore, loopDetector)
			}

			err := coord.AddDependency(stateStore, job)

			if tt.expectError {
				must.Error(t, err)
			} else {
				must.NoError(t, err)
			}

			if tt.expectStored {
				depName := job.Dependencies.Jobs[0].Name
				stored, ok := coord.dependencies[depName][job.ID]
				must.True(t, ok)
				must.Eq(t, job.ID, stored.ID)
				must.Eq(t, []string{depName}, loopDetector.nodes[job.ID])
			}
		})
	}
}

func TestCoordinator_CreatesCircularDependency(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		name              string
		job               *structs.Job
		setupLoopDetector func(*mockLoopDetector)
		expectCircular    bool
	}{
		{
			name: "job with no dependencies",
			job: &structs.Job{
				Name: "job-1",
			},
			setupLoopDetector: func(m *mockLoopDetector) {},
			expectCircular:    false,
		},
		{
			name: "job with non-circular dependencies",
			job: &structs.Job{
				Name: "job-1",
				Dependencies: &structs.JobDependencies{
					Jobs: []*structs.JobDependency{
						{Name: "job-2"},
					},
				},
			},
			setupLoopDetector: func(m *mockLoopDetector) {
				m.circularDeps["job-1"] = false
			},
			expectCircular: false,
		},
		{
			name: "job with circular dependencies",
			job: &structs.Job{
				Name: "job-1",
				Dependencies: &structs.JobDependencies{
					Jobs: []*structs.JobDependency{
						{Name: "job-2"},
					},
				},
			},
			setupLoopDetector: func(m *mockLoopDetector) {
				m.circularDeps["job-1"] = true
			},
			expectCircular: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loopDetector := newMockLoopDetector()
			tt.setupLoopDetector(loopDetector)

			coord := NewCoordinator(hclog.NewNullLogger(), loopDetector)
			result := coord.CreatesCircularDependency(tt.job)

			must.Eq(t, tt.expectCircular, result)
		})
	}
}

func TestCoordinator_UnblockedDependencies(t *testing.T) {
	ci.Parallel(t)

	stateStore := state.TestStateStore(t)
	loopDetector := newMockLoopDetector()
	coord := NewCoordinator(hclog.NewNullLogger(), loopDetector)

	depJob := mock.Job()
	depJob.Namespace = structs.DefaultNamespace
	must.NoError(t, stateStore.UpsertJob(structs.MsgTypeTestSetup, 1, nil, depJob))

	blockedJob := mock.Job()
	blockedJob.Namespace = structs.DefaultNamespace
	blockedJob.Dependencies = &structs.JobDependencies{
		Jobs: []*structs.JobDependency{{
			Name:   depJob.ID,
			Status: structs.JobDependencyRunning,
		}},
	}
	must.NoError(t, stateStore.UpsertJob(structs.MsgTypeTestSetup, 2, nil, blockedJob))
	must.NoError(t, coord.AddDependency(stateStore, blockedJob))

	blockedEval := mock.Eval()
	blockedEval.JobID = blockedJob.ID
	blockedEval.Namespace = blockedJob.Namespace
	blockedEval.Status = structs.EvalStatusBlocked
	blockedEval.TriggeredBy = structs.EvalTriggeredDeps
	must.NoError(t, stateStore.UpsertEvals(structs.MsgTypeTestSetup, 3, []*structs.Evaluation{blockedEval}))

	evals, err := coord.UnblockedDependencies(stateStore, depJob.ID)
	must.NoError(t, err)
	must.Len(t, 0, evals)

	alloc := mock.Alloc()
	alloc.JobID = depJob.ID
	alloc.Job = depJob
	alloc.Namespace = depJob.Namespace
	alloc.DesiredStatus = structs.AllocDesiredStatusRun
	alloc.ClientStatus = structs.AllocClientStatusPending
	must.NoError(t, stateStore.UpsertAllocs(structs.MsgTypeTestSetup, 4, []*structs.Allocation{alloc}))

	alloc.ClientStatus = structs.AllocClientStatusRunning
	must.NoError(t, stateStore.UpsertAllocs(structs.MsgTypeTestSetup, 5, []*structs.Allocation{alloc}))

	evals, err = coord.UnblockedDependencies(stateStore, depJob.ID)
	must.NoError(t, err)
	must.Len(t, 1, evals)
	must.Eq(t, blockedEval.ID, evals[0].ID)

	_, stillTracked := coord.dependencies[depJob.ID]
	must.False(t, stillTracked)
}

func TestCoordinator_Reload(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		name       string
		setupState func(*state.StateStore)
	}{
		{
			name:       "empty_evaluations",
			setupState: func(s *state.StateStore) {},
		},
		{
			name: "evaluations_without_dependencies",
			setupState: func(s *state.StateStore) {
				job := mock.Job()
				must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 1, nil, job))

				eval := mock.Eval()
				eval.JobID = job.ID
				must.NoError(t, s.UpsertEvals(structs.MsgTypeTestSetup, 2, []*structs.Evaluation{eval}))
			},
		},
		{
			name: "evaluations_with_dependencies",
			setupState: func(s *state.StateStore) {
				depJob := mock.Job()
				depJob.Namespace = structs.DefaultNamespace
				must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 1, nil, depJob))

				job := mock.Job()
				job.Namespace = structs.DefaultNamespace
				job.Dependencies = &structs.JobDependencies{
					Jobs: []*structs.JobDependency{
						{
							Name:   depJob.ID,
							Status: structs.JobDependencyRunning,
						},
					},
				}
				must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 2, nil, job))

				eval := mock.Eval()
				eval.JobID = job.ID
				eval.Namespace = job.Namespace
				must.NoError(t, s.UpsertEvals(structs.MsgTypeTestSetup, 3, []*structs.Evaluation{eval}))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateStore := state.TestStateStore(t)
			tt.setupState(stateStore)

			iter, _ := stateStore.Evals(memdb.NewWatchSet(), state.SortDefault)

			coord := NewCoordinator(hclog.NewNullLogger(), newMockLoopDetector())
			coord.Reload(stateStore, iter)

			must.NotNil(t, coord)
		})
	}
}

// Table-driven tests for condition matching functions

func TestConditionsMatch(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		name          string
		allocations   []*structs.Allocation
		expectedState string
		expectMatch   bool
	}{
		{
			name:          "nil allocations",
			allocations:   nil,
			expectedState: structs.JobDependencyRunning,
			expectMatch:   false,
		},
		{
			name:          "empty allocations",
			allocations:   []*structs.Allocation{},
			expectedState: structs.JobDependencyRunning,
			expectMatch:   false,
		},
		{
			name: "single allocation running",
			allocations: []*structs.Allocation{
				{
					ClientStatus: structs.AllocClientStatusRunning,
				},
			},
			expectedState: structs.JobDependencyRunning,
			expectMatch:   true,
		},
		{
			name: "single allocation complete",
			allocations: []*structs.Allocation{
				{
					ClientStatus: structs.AllocClientStatusComplete,
				},
			},
			expectedState: structs.JobDependencyComplete,
			expectMatch:   true,
		},
		{
			name: "single allocation failed",
			allocations: []*structs.Allocation{
				{
					ClientStatus: structs.AllocClientStatusFailed,
				},
			},
			expectedState: structs.JobDependencyFailed,
			expectMatch:   true,
		},
		{
			name: "multiple allocations all running",
			allocations: []*structs.Allocation{
				{ClientStatus: structs.AllocClientStatusRunning},
				{ClientStatus: structs.AllocClientStatusRunning},
			},
			expectedState: structs.JobDependencyRunning,
			expectMatch:   true,
		},
		{
			name: "multiple allocations mixed running and pending",
			allocations: []*structs.Allocation{
				{ClientStatus: structs.AllocClientStatusRunning},
				{ClientStatus: structs.AllocClientStatusPending},
			},
			expectedState: structs.JobDependencyRecovering,
			expectMatch:   true,
		},
		{
			name: "allocation lost",
			allocations: []*structs.Allocation{
				{
					ClientStatus: structs.AllocClientStatusUnknown,
				},
			},
			expectedState: structs.JobDependencyLost,
			expectMatch:   true,
		},
		{
			name: "allocation wrong state",
			allocations: []*structs.Allocation{
				{
					ClientStatus: structs.AllocClientStatusPending,
				},
			},
			expectedState: structs.JobDependencyRunning,
			expectMatch:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := conditionsMatch(tt.allocations, tt.expectedState)
			must.Eq(t, tt.expectMatch, result)
		})
	}
}

func TestDependencyTimeout(t *testing.T) {
	ci.Parallel(t)

	thirtySeconds := 30 * time.Second

	tests := []struct {
		name          string
		job           *structs.Job
		expectDefault bool
	}{
		{
			name: "job with no dependencies",
			job: &structs.Job{
				Name: "test",
			},
			expectDefault: true,
		},
		{
			name: "job with dependencies no timeout",
			job: &structs.Job{
				Name: "test",
				Dependencies: &structs.JobDependencies{
					Timeout: nil,
				},
			},
			expectDefault: true,
		},
		{
			name: "job with custom timeout",
			job: &structs.Job{
				Name: "test",
				Dependencies: &structs.JobDependencies{
					Timeout: &thirtySeconds,
				},
			},
			expectDefault: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dependencyTimeout(tt.job)
			if result.Nanoseconds() <= 0 {
				t.Fatalf("expected result to be positive, got %d", result.Nanoseconds())
			}

			if tt.expectDefault {
				must.Eq(t, structs.JobDependencyTimeoutDefault, result)
			} else if tt.job != nil && tt.job.Dependencies != nil && tt.job.Dependencies.Timeout != nil && *tt.job.Dependencies.Timeout > 0 {
				must.Eq(t, *tt.job.Dependencies.Timeout, result)
			}
		})
	}
}
