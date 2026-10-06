// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dependency

/*
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
		for _, dep := range deps {
			if dep == dependantJob {
				return loop_detection.ErrNodeIsDependency
			}
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

type mockEvalUnblocker struct {
	unblocked map[string]uint64
}

func newMockEvalUnblocker() *mockEvalUnblocker {
	return &mockEvalUnblocker{
		unblocked: make(map[string]uint64),
	}
}

func (m *mockEvalUnblocker) Unblock(computedClass string, index uint64) chan struct{} {
	m.unblocked[computedClass] = index
	ch := make(chan struct{})
	close(ch)
	return ch
}

// Table-driven tests for Coordinator public methods

func TestDebugJobDependency(t *testing.T) {
	// Create a dependency job
	depJob := mock.Job()
	depJob.Namespace = structs.DefaultNamespace
	t.Logf("depJob.ID: %s", depJob.ID)

	// Create the JobDependency
	dep := &structs.JobDependency{
		Name:   depJob.ID,
		Status: structs.JobDependencyRunning,
	}
	t.Logf("JobDependency.Name: %s", dep.Name)
	t.Logf("JobDependency.Status: %s", dep.Status)

	must.Eq(t, depJob.ID, dep.Name)
}

func TestDebugCheckDependency(t *testing.T) {
	stateStore := state.TestStateStore(t)

	// Create a dependency job
	depJob := mock.Job()
	depJob.Namespace = structs.DefaultNamespace
	must.NoError(t, stateStore.UpsertJob(structs.MsgTypeTestSetup, 1, nil, depJob))
	t.Logf("Created depJob with ID: %s", depJob.ID)

	// Don't create any allocations for depJob

	// Create a job with a dependency on depJob
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
	must.NoError(t, stateStore.UpsertJob(structs.MsgTypeTestSetup, 2, nil, job))
	t.Logf("Created job with ID: %s, depends on: %s", job.ID, depJob.ID)

	// Create an evaluation for the job
	eval := &structs.Evaluation{
		ID:        uuid.Generate(),
		JobID:     job.ID,
		Namespace: job.Namespace,
		Status:    structs.EvalStatusPending,
	}
	must.NoError(t, stateStore.UpsertEvals(structs.MsgTypeTestSetup, 3, []*structs.Evaluation{eval}))

	// Create a coordinator with a mock evalUnblocker
	coord := NewCoordinator(hclog.NewNullLogger(), newMockLoopDetector(), newMockEvalUnblocker(), nil)

	// Check the dependency
	blockers, err := coord.CheckDependency(stateStore, job, eval)
	t.Logf("CheckDependency returned: blockers=%v, err=%v", blockers, err)

	must.NoError(t, err)
	must.Greater(t, len(blockers), 0)
}

func TestDebugAllocsByJob(t *testing.T) {
	stateStore := state.TestStateStore(t)

	// Create a dependency job
	depJob := mock.Job()
	depJob.Namespace = structs.DefaultNamespace
	must.NoError(t, stateStore.UpsertJob(structs.MsgTypeTestSetup, 1, nil, depJob))

	// Try to get allocations for this job
	allocs, err := stateStore.AllocsByJob(memdb.NewWatchSet(), depJob.Namespace, depJob.ID, true)
	t.Logf("Job ID: %s", depJob.ID)
	t.Logf("Error: %v", err)
	t.Logf("Allocations: %v", allocs)
	t.Logf("Len(Allocations): %d", len(allocs))

	must.NoError(t, err)
	must.Len(t, 0, allocs)

	// Now create an allocation and try again
	depAlloc := mock.Alloc()
	depAlloc.JobID = depJob.ID
	depAlloc.Job = depJob
	depAlloc.DesiredStatus = structs.AllocDesiredStatusRun
	depAlloc.ClientStatus = structs.AllocClientStatusPending
	must.NoError(t, stateStore.UpsertAllocs(structs.MsgTypeTestSetup, 2, []*structs.Allocation{depAlloc}))

	// Try to get allocations again
	allocs2, err := stateStore.AllocsByJob(memdb.NewWatchSet(), depJob.Namespace, depJob.ID, true)
	t.Logf("After insert - Allocations: %v", allocs2)
	t.Logf("After insert - Len(Allocations): %d", len(allocs2))

	must.NoError(t, err)
	must.Len(t, 1, allocs2)
}

func TestCoordinator_CheckDependency(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		name           string
		setupState     func(*state.StateStore) (string, string, string)
		expectError    bool
		expectBlockers bool
	}{
		{
			name:           "nil inputs",
			setupState:     nil,
			expectError:    false,
			expectBlockers: false,
		},
		{
			name: "no dependencies",
			setupState: func(s *state.StateStore) (string, string, string) {
				job := mock.Job()
				job.Namespace = structs.DefaultNamespace
				must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 1, nil, job))

				eval := &structs.Evaluation{
					ID:        uuid.Generate(),
					JobID:     job.ID,
					Namespace: job.Namespace,
					Status:    structs.EvalStatusPending,
				}
				must.NoError(t, s.UpsertEvals(structs.MsgTypeTestSetup, 2, []*structs.Evaluation{eval}))

				return job.ID, eval.ID, job.Namespace
			},
			expectError:    false,
			expectBlockers: false,
		},
		{
			name: "dependencies without running allocations",
			setupState: func(s *state.StateStore) (string, string, string) {
				// Create dependency job but don't allocate it
				depJob := mock.Job()
				depJob.Namespace = structs.DefaultNamespace
				must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 1, nil, depJob))

				// Create dependent job with dependency using the job ID
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

				eval := &structs.Evaluation{
					ID:        uuid.Generate(),
					JobID:     job.ID,
					Namespace: job.Namespace,
					Status:    structs.EvalStatusPending,
				}
				must.NoError(t, s.UpsertEvals(structs.MsgTypeTestSetup, 3, []*structs.Evaluation{eval}))

				return job.ID, eval.ID, job.Namespace
			},
			expectError:    false,
			expectBlockers: true,
		},
		{
			name: "dependencies with running allocations",
			setupState: func(s *state.StateStore) (string, string, string) {
				// Create dependency job
				depJob := mock.Job()
				depJob.Namespace = structs.DefaultNamespace
				must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 1, nil, depJob))

				// Create running allocation for dependency job
				depAlloc := mock.Alloc()
				depAlloc.JobID = depJob.ID
				depAlloc.Job = depJob
				depAlloc.ClientStatus = structs.AllocClientStatusPending
				depAlloc.DesiredStatus = structs.AllocDesiredStatusRun
				must.NoError(t, s.UpsertAllocs(structs.MsgTypeTestSetup, 2, []*structs.Allocation{depAlloc}))

				// Update allocation to running
				depAlloc.ClientStatus = structs.AllocClientStatusRunning
				must.NoError(t, s.UpsertAllocs(structs.MsgTypeTestSetup, 3, []*structs.Allocation{depAlloc}))

				// Create dependent job using the dependency job's ID
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
				must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 4, nil, job))

				eval := &structs.Evaluation{
					ID:        uuid.Generate(),
					JobID:     job.ID,
					Namespace: job.Namespace,
					Status:    structs.EvalStatusPending,
				}
				must.NoError(t, s.UpsertEvals(structs.MsgTypeTestSetup, 5, []*structs.Evaluation{eval}))

				return job.ID, eval.ID, job.Namespace
			},
			expectError:    false,
			expectBlockers: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stateStore *state.StateStore
			var job *structs.Job
			var eval *structs.Evaluation

			if tt.setupState != nil {
				stateStore = state.TestStateStore(t)
				jobID, evalID, namespace := tt.setupState(stateStore)

				// Retrieve job and eval from state
				jobIt, err := stateStore.JobByID(memdb.NewWatchSet(), namespace, jobID)
				must.NoError(t, err)
				if jobIt != nil {
					job = jobIt
				}

				evalIt, err := stateStore.EvalByID(memdb.NewWatchSet(), evalID)
				must.NoError(t, err)
				if evalIt != nil {
					eval = evalIt
				}
			}

			coord := NewCoordinator(hclog.NewNullLogger(), newMockLoopDetector(), newMockEvalUnblocker(), nil)

			blockers, err := coord.CheckDependency(stateStore, job, eval)
			t.Logf("Test %s: blockers=%v, len=%d", tt.name, blockers, len(blockers))

			if tt.expectError {
				must.Error(t, err)
			} else {
				must.NoError(t, err)
			}

			if tt.expectBlockers {
				t.Logf("Expecting blockers > 0, got %d", len(blockers))
				must.Greater(t, len(blockers), 0, must.Sprint("expected blockers to be non-empty"))
			} else {
				must.Len(t, 0, blockers)
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

			coord := NewCoordinator(hclog.NewNullLogger(), loopDetector, nil, nil)
			result := coord.CreatesCircularDependency(tt.job)

			must.Eq(t, tt.expectCircular, result)
		})
	}
}

func TestCoordinator_HasActiveDependents(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		name              string
		job               *structs.Job
		setupLoopDetector func(*mockLoopDetector)
		expectActiveDeps  bool
		expectError       bool
	}{
		{
			name: "job with no dependents",
			job: &structs.Job{
				Name: "job-1",
				ID:   "job-1-id",
			},
			setupLoopDetector: func(m *mockLoopDetector) {
				m.shouldReturnNotFound = true
			},
			expectActiveDeps: false,
			expectError:      false,
		},
		{
			name: "job with active dependents",
			job: &structs.Job{
				Name: "job-1",
				ID:   "job-1-id",
			},
			setupLoopDetector: func(m *mockLoopDetector) {
				// Setup so RemoveNode returns ErrNodeIsDependency for job-1-id
				m.nodes["job-2"] = []string{"job-1-id"}
			},
			expectActiveDeps: true,
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loopDetector := newMockLoopDetector()
			tt.setupLoopDetector(loopDetector)

			coord := NewCoordinator(hclog.NewNullLogger(), loopDetector, nil, nil)
			result, err := coord.HasActiveDependents(tt.job)

			if tt.expectError {
				must.Error(t, err)
			} else {
				must.NoError(t, err)
			}

			must.Eq(t, tt.expectActiveDeps, result)
		})
	}
}

func TestCoordinator_Stop(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		name string
	}{
		{
			name: "stop_clears_dependencies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coord := NewCoordinator(hclog.NewNullLogger(), newMockLoopDetector(), nil, nil)

			must.NotNil(t, coord.dependencies)
			coord.Stop()
			must.Nil(t, coord.dependencies)
		})
	}
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
				depJob.Name = "dep-job"
				depJob.Namespace = structs.DefaultNamespace
				must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 1, nil, depJob))

				job := mock.Job()
				job.Name = "test-job"
				job.Namespace = structs.DefaultNamespace
				job.Dependencies = &structs.JobDependencies{
					Jobs: []*structs.JobDependency{
						{
							Name:   "dep-job",
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

			coord := NewCoordinator(hclog.NewNullLogger(), newMockLoopDetector(), newMockEvalUnblocker(), nil)
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
			name: "allocation stopped",
			allocations: []*structs.Allocation{
				{
					DesiredStatus: structs.AllocDesiredStatusStop,
					ClientStatus:  structs.AllocClientStatusComplete,
				},
			},
			expectedState: structs.JobDependencyStopped,
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

	tests := []struct {
		name          string
		job           *structs.Job
		expectTimeout bool
		expectDefault bool
	}{
		{
			name:          "nil job",
			job:           nil,
			expectTimeout: true,
			expectDefault: false,
		},
		{
			name: "job with no dependencies",
			job: &structs.Job{
				Name: "test",
			},
			expectTimeout: true,
			expectDefault: true,
		},
		{
			name: "job with dependencies no timeout",
			job: &structs.Job{
				Name: "test",
				Dependencies: &structs.JobDependencies{
					Timeout: 0,
				},
			},
			expectTimeout: true,
			expectDefault: true,
		},
		{
			name: "job with custom timeout",
			job: &structs.Job{
				Name: "test",
				Dependencies: &structs.JobDependencies{
					Timeout: 30000000000, // 30 seconds in nanoseconds
				},
			},
			expectTimeout: true,
			expectDefault: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dependencyTimeout(tt.job)
			must.Greater(t, result.Nanoseconds(), int64(0))

			if tt.expectDefault {
				must.Eq(t, DefaultTimeout, result)
			} else if tt.job != nil && tt.job.Dependencies != nil && tt.job.Dependencies.Timeout > 0 {
				must.Eq(t, tt.job.Dependencies.Timeout, result)
			}
		})
	}
}
*/
