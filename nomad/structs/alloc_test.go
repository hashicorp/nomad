// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

// TestAllocation_Canonicalize_New asserts that an alloc with latest
// schema isn't modified with Canonicalize
func TestAllocation_Canonicalize_New(t *testing.T) {
	ci.Parallel(t)

	alloc := MockAlloc()
	copy := alloc.Copy()

	alloc.Canonicalize()
	must.Eq(t, copy, alloc)
}

func TestAllocServiceRegistrationsRequest_StaleReadSupport(t *testing.T) {
	req := &AllocServiceRegistrationsRequest{}
	must.True(t, req.IsRead())
}

func Test_Allocation_ServiceProviderNamespace(t *testing.T) {
	testCases := []struct {
		inputAllocation *Allocation
		expectedOutput  string
		name            string
	}{
		{
			inputAllocation: &Allocation{
				Job: &Job{
					TaskGroups: []*TaskGroup{
						{
							Name: "test-group",
							Services: []*Service{
								{
									Provider: ServiceProviderConsul,
								},
							},
						},
					},
				},
				TaskGroup: "test-group",
			},
			expectedOutput: "",
			name:           "consul task group service",
		},
		{
			inputAllocation: &Allocation{
				Job: &Job{
					TaskGroups: []*TaskGroup{
						{
							Name: "test-group",
							Tasks: []*Task{
								{
									Services: []*Service{
										{
											Provider: ServiceProviderConsul,
										},
									},
								},
							},
						},
					},
				},
				TaskGroup: "test-group",
			},
			expectedOutput: "",
			name:           "consul task service",
		},
		{
			inputAllocation: &Allocation{
				Job: &Job{
					Namespace: "platform",
					TaskGroups: []*TaskGroup{
						{
							Name: "test-group",
							Services: []*Service{
								{
									Provider: ServiceProviderNomad,
								},
							},
						},
					},
				},
				TaskGroup: "test-group",
			},
			expectedOutput: "platform",
			name:           "nomad task group service",
		},
		{
			inputAllocation: &Allocation{
				Job: &Job{
					Namespace: "platform",
					TaskGroups: []*TaskGroup{
						{
							Name: "test-group",
							Tasks: []*Task{
								{
									Services: []*Service{
										{
											Provider: ServiceProviderNomad,
										},
									},
								},
							},
						},
					},
				},
				TaskGroup: "test-group",
			},
			expectedOutput: "platform",
			name:           "nomad task service",
		},
		{
			inputAllocation: &Allocation{
				Job: &Job{
					Namespace: "platform",
					TaskGroups: []*TaskGroup{
						{
							Name: "test-group",
							Tasks: []*Task{
								{
									Name: "task1",
								},
								{
									Name: "task2",
									Services: []*Service{
										{
											Provider: ServiceProviderNomad,
										},
									},
								},
							},
						},
					},
				},
				TaskGroup: "test-group",
			},
			expectedOutput: "platform",
			name:           "multiple tasks with service not in first",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputAllocation.ServiceProviderNamespace()
			must.Eq(t, tc.expectedOutput, actualOutput)
		})
	}
}

func TestAllocation_WaitClientStop_Disconnect(t *testing.T) {
	ci.Parallel(t)
	type testCase struct {
		desc                   string
		stop                   time.Duration
		status                 string
		expectedShould         bool
		expectedRescheduleTime time.Time
	}
	now := time.Now().UTC()
	testCases := []testCase{
		{
			desc:           "running",
			stop:           2 * time.Second,
			status:         AllocClientStatusRunning,
			expectedShould: true,
		},
		{
			desc:           "no stop_after_client_disconnect",
			status:         AllocClientStatusLost,
			expectedShould: false,
		},
		{
			desc:                   "stop",
			status:                 AllocClientStatusLost,
			stop:                   2 * time.Second,
			expectedShould:         true,
			expectedRescheduleTime: now.Add((2 + 5) * time.Second),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			j := testJob()
			a := &Allocation{
				ClientStatus: tc.status,
				Job:          j,
				TaskStates:   map[string]*TaskState{},
			}

			if tc.status == AllocClientStatusLost {
				a.AppendState(AllocStateFieldClientStatus, AllocClientStatusLost)
			}

			j.TaskGroups[0].Disconnect = &DisconnectStrategy{
				StopOnClientAfter: &tc.stop,
			}

			a.TaskGroup = j.TaskGroups[0].Name

			must.Eq(t, tc.expectedShould, a.ShouldClientStop())

			if !tc.expectedShould || tc.status != AllocClientStatusLost {
				return
			}

			// the reschedTime is close to the expectedRescheduleTime
			reschedTime := a.WaitClientStop()
			e := reschedTime.Unix() - tc.expectedRescheduleTime.Unix()
			must.Less(t, int64(2), e)
		})
	}
}

func TestAllocation_Timeout_Disconnect(t *testing.T) {
	type testCase struct {
		desc          string
		maxDisconnect time.Duration
	}

	testCases := []testCase{
		{
			desc:          "has lost_after",
			maxDisconnect: 30 * time.Second,
		},
		{
			desc:          "zero lost_after",
			maxDisconnect: 0 * time.Second,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			j := testJob()
			a := &Allocation{
				Job: j,
			}

			j.TaskGroups[0].Disconnect = &DisconnectStrategy{
				LostAfter: tc.maxDisconnect,
			}

			a.TaskGroup = j.TaskGroups[0].Name

			now := time.Now()

			reschedTime := a.DisconnectTimeout(now)

			if tc.maxDisconnect == 0 {
				must.Equal(t, now, reschedTime, must.Sprint("expected to be now"))
			} else {
				difference := reschedTime.Sub(now)
				must.Eq(t, tc.maxDisconnect, difference, must.Sprint("expected durations to be equal"))
			}

		})
	}
}

func TestAllocation_Expired_Disconnected(t *testing.T) {
	type testCase struct {
		name             string
		maxDisconnect    string
		ellapsed         int
		expected         bool
		nilJob           bool
		badTaskGroup     bool
		mixedUTC         bool
		noReconnectEvent bool
		status           string
	}

	testCases := []testCase{
		{
			name:          "has-expired",
			maxDisconnect: "5s",
			ellapsed:      10,
			expected:      true,
		},
		{
			name:          "has-not-expired",
			maxDisconnect: "5s",
			ellapsed:      3,
			expected:      false,
		},
		{
			name:          "are-equal",
			maxDisconnect: "5s",
			ellapsed:      5,
			expected:      true,
		},
		{
			name:          "nil-job",
			maxDisconnect: "5s",
			ellapsed:      10,
			expected:      false,
			nilJob:        true,
		},
		{
			name:          "wrong-status",
			maxDisconnect: "5s",
			ellapsed:      10,
			expected:      false,
			status:        AllocClientStatusRunning,
		},
		{
			name:          "bad-task-group",
			maxDisconnect: "",
			badTaskGroup:  true,
			ellapsed:      10,
			expected:      false,
		},
		{
			name:          "no-max-disconnect",
			maxDisconnect: "",
			ellapsed:      10,
			expected:      true,
		},
		{
			name:          "mixed-utc-has-expired",
			maxDisconnect: "5s",
			ellapsed:      10,
			mixedUTC:      true,
			expected:      true,
		},
		{
			name:          "mixed-utc-has-not-expired",
			maxDisconnect: "5s",
			ellapsed:      3,
			mixedUTC:      true,
			expected:      false,
		},
		{
			name:             "no-reconnect-event",
			maxDisconnect:    "5s",
			ellapsed:         2,
			expected:         false,
			noReconnectEvent: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alloc := MockAlloc()
			var err error
			var maxDisconnect time.Duration

			if tc.maxDisconnect != "" {
				maxDisconnect, err = time.ParseDuration(tc.maxDisconnect)
				must.NoError(t, err)
				alloc.Job.TaskGroups[0].Disconnect = &DisconnectStrategy{
					LostAfter: maxDisconnect,
				}
			}

			if tc.nilJob {
				alloc.Job = nil
			}

			if tc.badTaskGroup {
				alloc.TaskGroup = "bad"
			}

			alloc.ClientStatus = AllocClientStatusUnknown
			if tc.status != "" {
				alloc.ClientStatus = tc.status
			}

			alloc.AllocStates = []*AllocState{{
				Field: AllocStateFieldClientStatus,
				Value: AllocClientStatusUnknown,
				Time:  time.Now(),
			}}

			must.NoError(t, err)
			now := time.Now().UTC()
			if tc.mixedUTC {
				now = time.Now()
			}

			if !tc.noReconnectEvent {
				event := NewTaskEvent(TaskClientReconnected)
				event.Time = now.UnixNano()

				alloc.TaskStates = map[string]*TaskState{
					"web": {
						Events: []*TaskEvent{event},
					},
				}
			}

			ellapsedDuration := time.Duration(tc.ellapsed) * time.Second
			now = now.Add(ellapsedDuration)

			must.Eq(t, tc.expected, alloc.Expired(now))
		})
	}
}

func TestAllocation_NextRescheduleTime(t *testing.T) {
	now := time.Now()
	makeTestAlloc := func(batch bool) *Allocation {
		j := &Job{
			Region:    "global",
			Name:      "mock-job",
			Namespace: DefaultNamespace,
			Priority:  50,
			Status:    JobStatusPending,
			TaskGroups: []*TaskGroup{
				{
					Name: "MockTaskGroup",
					ReschedulePolicy: &ReschedulePolicy{
						Attempts:      2,
						Interval:      time.Hour,
						Delay:         2 * time.Minute,
						DelayFunction: "constant",
						MaxDelay:      -1,
						Unlimited:     false,
					},
				},
			},
		}
		if batch {
			j.Type = JobTypeBatch
			j.ID = fmt.Sprintf("mock-batch-%s", uuid.Generate())
		} else {
			j.Type = JobTypeService
			j.ID = fmt.Sprintf("mock-service-%s", uuid.Generate())
		}
		j.Canonicalize()

		alloc := &Allocation{
			ID:            uuid.Generate(),
			EvalID:        uuid.Generate(),
			NodeID:        "12345678-abcd-efab-cdef-123456789abc",
			Namespace:     DefaultNamespace,
			TaskGroup:     "MockTaskGroup",
			Job:           j,
			DesiredStatus: AllocDesiredStatusRun,
			ClientStatus:  AllocClientStatusFailed,
			TaskStates: map[string]*TaskState{
				"task": {State: TaskStateDead, Failed: true, FinishedAt: now},
			},
		}
		alloc.Canonicalize()

		return alloc
	}

	testCases := []struct {
		name       string
		allocFn    func(*Allocation)
		isBatch    bool
		isEligible bool
	}{
		{
			name:       "client status is failed",
			isEligible: true,
		},
		{
			name:       "client status is lost",
			allocFn:    func(a *Allocation) { a.ClientStatus = AllocClientStatusLost },
			isEligible: true,
		},
		{
			name:       "client status is pending",
			allocFn:    func(a *Allocation) { a.ClientStatus = AllocClientStatusPending },
			isEligible: false,
		},
		{
			name:       "client status is running",
			allocFn:    func(a *Allocation) { a.ClientStatus = AllocClientStatusRunning },
			isEligible: false,
		},
		{
			name:       "client status is complete",
			allocFn:    func(a *Allocation) { a.ClientStatus = AllocClientStatusComplete },
			isEligible: false,
		},
		{
			name:       "client status is unknown",
			allocFn:    func(a *Allocation) { a.ClientStatus = AllocClientStatusUnknown },
			isEligible: false,
		},
		{
			name: "failed service without reschedule policy",
			allocFn: func(a *Allocation) {
				a.Job.TaskGroups[0].ReschedulePolicy = nil
			},
			isEligible: false,
		},
		{
			name: "failed service with policy not unlimited and no attempts",
			allocFn: func(a *Allocation) {
				a.Job.TaskGroups[0].ReschedulePolicy.Attempts = 0
			},
			isEligible: false,
		},
		{
			name: "failed service with policy unlimited and no attempts",
			allocFn: func(a *Allocation) {
				a.Job.TaskGroups[0].ReschedulePolicy.Attempts = 0
				a.Job.TaskGroups[0].ReschedulePolicy.Unlimited = true
			},
			isEligible: true,
		},
		{
			name: "service with desired stop and last reschedule did not fail",
			allocFn: func(a *Allocation) {
				a.DesiredStatus = AllocDesiredStatusStop
				a.RescheduleTracker = &RescheduleTracker{LastReschedule: LastRescheduleSuccess}
			},
			isEligible: false,
		},
		{
			name: "service with desired stop and last reschedule is failed without events",
			allocFn: func(a *Allocation) {
				a.DesiredStatus = AllocDesiredStatusStop
				a.RescheduleTracker = &RescheduleTracker{LastReschedule: LastRescheduleFailedToPlace}
			},
			isEligible: false,
		},
		{
			name: "service with desired stop and last reschedule is failed with events",
			allocFn: func(a *Allocation) {
				a.DesiredStatus = AllocDesiredStatusStop
				a.RescheduleTracker = &RescheduleTracker{
					LastReschedule: LastRescheduleFailedToPlace,
					Events:         []*RescheduleEvent{},
				}
			},
			isEligible: true,
		},
		{
			name: "service has not exceeded reschedule attempts",
			allocFn: func(a *Allocation) {
				a.RescheduleTracker = &RescheduleTracker{
					LastReschedule: LastRescheduleFailedToPlace,
					Events: []*RescheduleEvent{
						{RescheduleTime: now.Add(-10 * time.Minute).UnixNano()},
					},
				}
			},
			isEligible: true,
		},
		{
			name: "service has exceeded reschedule attempts",
			allocFn: func(a *Allocation) {
				a.RescheduleTracker = &RescheduleTracker{
					LastReschedule: LastRescheduleFailedToPlace,
					Events: []*RescheduleEvent{
						{RescheduleTime: now.Add(-10 * time.Minute).UnixNano()},
						{RescheduleTime: now.Add(-5 * time.Minute).UnixNano()},
					},
				}
			},
			isEligible: false,
		},
		{
			name:       "batch without reschedule",
			isBatch:    true,
			allocFn:    func(a *Allocation) { a.Job.TaskGroups[0].ReschedulePolicy = nil },
			isEligible: false,
		},
		{
			name:    "batch with desired stop and last reschedule did not fail",
			isBatch: true,
			allocFn: func(a *Allocation) {
				a.DesiredStatus = AllocDesiredStatusStop
				a.RescheduleTracker = &RescheduleTracker{LastReschedule: LastRescheduleSuccess}
			},
			isEligible: false,
		},
		{
			name:    "batch with desired stop and last reschedule is failed without events",
			isBatch: true,
			allocFn: func(a *Allocation) {
				a.DesiredStatus = AllocDesiredStatusStop
				a.RescheduleTracker = &RescheduleTracker{LastReschedule: LastRescheduleFailedToPlace}
			},
			isEligible: false,
		},
		{
			name:    "batch with desired stop and last reschedule is failed with events",
			isBatch: true,
			allocFn: func(a *Allocation) {
				a.DesiredStatus = AllocDesiredStatusStop
				a.RescheduleTracker = &RescheduleTracker{
					LastReschedule: LastRescheduleFailedToPlace,
					Events:         []*RescheduleEvent{},
				}
			},
			isEligible: true,
		},
		{
			name:    "batch has not exceeded reschedule attempts",
			isBatch: true,
			allocFn: func(a *Allocation) {
				a.RescheduleTracker = &RescheduleTracker{
					LastReschedule: LastRescheduleFailedToPlace,
					Events: []*RescheduleEvent{
						{RescheduleTime: now.Add(-10 * time.Minute).UnixNano()},
					},
				}
			},
			isEligible: true,
		},
		{
			name:    "batch has exceeded reschedule attempts",
			isBatch: true,
			allocFn: func(a *Allocation) {
				a.RescheduleTracker = &RescheduleTracker{
					LastReschedule: LastRescheduleFailedToPlace,
					Events: []*RescheduleEvent{
						{RescheduleTime: now.Add(-10 * time.Minute).UnixNano()},
						{RescheduleTime: now.Add(-5 * time.Minute).UnixNano()},
					},
				}
			},
			isEligible: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alloc := makeTestAlloc(tc.isBatch)
			if tc.allocFn != nil {
				tc.allocFn(alloc)
			}

			nextTime, eligible := alloc.NextRescheduleTime()
			if tc.isEligible {
				must.True(t, eligible)
				must.Eq(t, now.Add(2*time.Minute), nextTime)
			} else {
				must.False(t, eligible)
			}
		})
	}
}

func TestAllocation_Index(t *testing.T) {
	ci.Parallel(t)

	a1 := Allocation{
		Name:      "example.cache[1]",
		TaskGroup: "cache",
		JobID:     "example",
		Job: &Job{
			ID:         "example",
			TaskGroups: []*TaskGroup{{Name: "cache"}}},
	}
	e1 := uint(1)
	a2 := a1.Copy()
	a2.Name = "example.cache[713127]"
	e2 := uint(713127)

	if a1.Index() != e1 || a2.Index() != e2 {
		t.Fatalf("Got %d and %d", a1.Index(), a2.Index())
	}
}
func TestAllocation_Terminated(t *testing.T) {
	ci.Parallel(t)
	type desiredState struct {
		ClientStatus  string
		DesiredStatus string
		Terminated    bool
	}
	harness := []desiredState{
		{
			ClientStatus:  AllocClientStatusPending,
			DesiredStatus: AllocDesiredStatusStop,
			Terminated:    false,
		},
		{
			ClientStatus:  AllocClientStatusRunning,
			DesiredStatus: AllocDesiredStatusStop,
			Terminated:    false,
		},
		{
			ClientStatus:  AllocClientStatusFailed,
			DesiredStatus: AllocDesiredStatusStop,
			Terminated:    true,
		},
		{
			ClientStatus:  AllocClientStatusFailed,
			DesiredStatus: AllocDesiredStatusRun,
			Terminated:    true,
		},
	}

	for _, state := range harness {
		alloc := Allocation{}
		alloc.DesiredStatus = state.DesiredStatus
		alloc.ClientStatus = state.ClientStatus
		if alloc.Terminated() != state.Terminated {
			t.Fatalf("expected: %v, actual: %v", state.Terminated, alloc.Terminated())
		}
	}
}

func TestAllocation_ShouldReschedule(t *testing.T) {
	ci.Parallel(t)
	type testCase struct {
		Desc               string
		FailTime           time.Time
		ClientStatus       string
		DesiredStatus      string
		ReschedulePolicy   *ReschedulePolicy
		RescheduleTrackers []*RescheduleEvent
		ShouldReschedule   bool
	}
	fail := time.Now()

	harness := []testCase{
		{
			Desc:             "Reschedule when desired state is stop",
			ClientStatus:     AllocClientStatusPending,
			DesiredStatus:    AllocDesiredStatusStop,
			FailTime:         fail,
			ReschedulePolicy: nil,
			ShouldReschedule: false,
		},
		{
			Desc:             "Disabled rescheduling",
			ClientStatus:     AllocClientStatusFailed,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: &ReschedulePolicy{Attempts: 0, Interval: 1 * time.Minute},
			ShouldReschedule: false,
		},
		{
			Desc:             "Reschedule when client status is complete",
			ClientStatus:     AllocClientStatusComplete,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: nil,
			ShouldReschedule: false,
		},
		{
			Desc:             "Reschedule with nil reschedule policy",
			ClientStatus:     AllocClientStatusFailed,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: nil,
			ShouldReschedule: false,
		},
		{
			Desc:             "Reschedule with unlimited and attempts >0",
			ClientStatus:     AllocClientStatusFailed,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: &ReschedulePolicy{Attempts: 1, Unlimited: true},
			ShouldReschedule: true,
		},
		{
			Desc:             "Reschedule when client status is complete",
			ClientStatus:     AllocClientStatusComplete,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: nil,
			ShouldReschedule: false,
		},
		{
			Desc:             "Reschedule with policy when client status complete",
			ClientStatus:     AllocClientStatusComplete,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: &ReschedulePolicy{Attempts: 1, Interval: 1 * time.Minute},
			ShouldReschedule: false,
		},
		{
			Desc:             "Reschedule with no previous attempts",
			ClientStatus:     AllocClientStatusFailed,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: &ReschedulePolicy{Attempts: 1, Interval: 1 * time.Minute},
			ShouldReschedule: true,
		},
		{
			Desc:             "Reschedule with leftover attempts",
			ClientStatus:     AllocClientStatusFailed,
			DesiredStatus:    AllocDesiredStatusRun,
			ReschedulePolicy: &ReschedulePolicy{Attempts: 2, Interval: 5 * time.Minute},
			FailTime:         fail,
			RescheduleTrackers: []*RescheduleEvent{
				{
					RescheduleTime: fail.Add(-1 * time.Minute).UTC().UnixNano(),
				},
			},
			ShouldReschedule: true,
		},
		{
			Desc:             "Reschedule with too old previous attempts",
			ClientStatus:     AllocClientStatusFailed,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: &ReschedulePolicy{Attempts: 1, Interval: 5 * time.Minute},
			RescheduleTrackers: []*RescheduleEvent{
				{
					RescheduleTime: fail.Add(-6 * time.Minute).UTC().UnixNano(),
				},
			},
			ShouldReschedule: true,
		},
		{
			Desc:             "Reschedule with no leftover attempts",
			ClientStatus:     AllocClientStatusFailed,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: &ReschedulePolicy{Attempts: 2, Interval: 5 * time.Minute},
			RescheduleTrackers: []*RescheduleEvent{
				{
					RescheduleTime: fail.Add(-3 * time.Minute).UTC().UnixNano(),
				},
				{
					RescheduleTime: fail.Add(-4 * time.Minute).UTC().UnixNano(),
				},
			},
			ShouldReschedule: false,
		},
	}

	for _, state := range harness {
		alloc := Allocation{}
		alloc.DesiredStatus = state.DesiredStatus
		alloc.ClientStatus = state.ClientStatus
		alloc.RescheduleTracker = &RescheduleTracker{
			Events:         state.RescheduleTrackers,
			LastReschedule: "",
		}

		t.Run(state.Desc, func(t *testing.T) {
			if got := alloc.ShouldReschedule(state.ReschedulePolicy, state.FailTime); got != state.ShouldReschedule {
				t.Fatalf("expected %v but got %v", state.ShouldReschedule, got)
			}
		})

	}
}

func TestAllocation_LastEventTime(t *testing.T) {
	ci.Parallel(t)
	type testCase struct {
		desc                  string
		taskState             map[string]*TaskState
		expectedLastEventTime time.Time
	}
	t1 := time.Now().UTC()

	testCases := []testCase{
		{
			desc:                  "nil task state",
			expectedLastEventTime: t1,
		},
		{
			desc:                  "empty task state",
			taskState:             make(map[string]*TaskState),
			expectedLastEventTime: t1,
		},
		{
			desc: "Finished At not set",
			taskState: map[string]*TaskState{"foo": {State: "start",
				StartedAt: t1.Add(-2 * time.Hour)}},
			expectedLastEventTime: t1,
		},
		{
			desc: "One finished ",
			taskState: map[string]*TaskState{"foo": {State: "start",
				StartedAt:  t1.Add(-2 * time.Hour),
				FinishedAt: t1.Add(-1 * time.Hour)}},
			expectedLastEventTime: t1.Add(-1 * time.Hour),
		},
		{
			desc: "Multiple task groups",
			taskState: map[string]*TaskState{"foo": {State: "start",
				StartedAt:  t1.Add(-2 * time.Hour),
				FinishedAt: t1.Add(-1 * time.Hour)},
				"bar": {State: "start",
					StartedAt:  t1.Add(-2 * time.Hour),
					FinishedAt: t1.Add(-40 * time.Minute)}},
			expectedLastEventTime: t1.Add(-40 * time.Minute),
		},
		{
			desc: "No finishedAt set, one task event, should use modify time",
			taskState: map[string]*TaskState{"foo": {
				State:     "run",
				StartedAt: t1.Add(-2 * time.Hour),
				Events: []*TaskEvent{
					{Type: "start", Time: t1.Add(-20 * time.Minute).UnixNano()},
				}},
			},
			expectedLastEventTime: t1,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			alloc := &Allocation{CreateTime: t1.UnixNano(), ModifyTime: t1.UnixNano()}
			alloc.TaskStates = tc.taskState
			must.Eq(t, tc.expectedLastEventTime, alloc.LastEventTime())
		})
	}
}

func TestAllocation_NextDelay(t *testing.T) {
	ci.Parallel(t)
	type testCase struct {
		desc                       string
		reschedulePolicy           *ReschedulePolicy
		alloc                      *Allocation
		expectedRescheduleTime     time.Time
		expectedRescheduleEligible bool
	}
	now := time.Now()
	testCases := []testCase{
		{
			desc: "Allocation hasn't failed yet",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "constant",
				Delay:         5 * time.Second,
			},
			alloc:                      &Allocation{},
			expectedRescheduleTime:     time.Time{},
			expectedRescheduleEligible: false,
		},
		{
			desc:                       "Allocation has no reschedule policy",
			alloc:                      &Allocation{},
			expectedRescheduleTime:     time.Time{},
			expectedRescheduleEligible: false,
		},
		{
			desc: "Allocation lacks task state",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "constant",
				Delay:         5 * time.Second,
				Unlimited:     true,
			},
			alloc:                      &Allocation{ClientStatus: AllocClientStatusFailed, ModifyTime: now.UnixNano()},
			expectedRescheduleTime:     now.UTC().Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "linear delay, unlimited restarts, no reschedule tracker",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "constant",
				Delay:         5 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "dead",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}},
			},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "linear delay with reschedule tracker",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "constant",
				Delay:         5 * time.Second,
				Interval:      10 * time.Minute,
				Attempts:      2,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{{
						RescheduleTime: now.Add(-2 * time.Minute).UTC().UnixNano(),
						Delay:          5 * time.Second,
					}},
				}},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "linear delay with reschedule tracker, attempts exhausted",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "constant",
				Delay:         5 * time.Second,
				Interval:      10 * time.Minute,
				Attempts:      2,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-3 * time.Minute).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-2 * time.Minute).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(5 * time.Second),
			expectedRescheduleEligible: false,
		},
		{
			desc: "exponential delay - no reschedule tracker",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "exponential",
				Delay:         5 * time.Second,
				MaxDelay:      90 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}},
			},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "exponential delay with reschedule tracker",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "exponential",
				Delay:         5 * time.Second,
				MaxDelay:      90 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          10 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          20 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(40 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "exponential delay with delay ceiling reached",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "exponential",
				Delay:         5 * time.Second,
				MaxDelay:      90 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-15 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          10 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          20 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          40 * time.Second,
						},
						{
							RescheduleTime: now.Add(-40 * time.Second).UTC().UnixNano(),
							Delay:          80 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-15 * time.Second).Add(90 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			// Test case where most recent reschedule ran longer than delay ceiling
			desc: "exponential delay, delay ceiling reset condition met",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "exponential",
				Delay:         5 * time.Second,
				MaxDelay:      90 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-15 * time.Minute)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          10 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          20 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          40 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          80 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          90 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          90 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-15 * time.Minute).Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "fibonacci delay - no reschedule tracker",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "fibonacci",
				Delay:         5 * time.Second,
				MaxDelay:      90 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}}},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "fibonacci delay with reschedule tracker",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "fibonacci",
				Delay:         5 * time.Second,
				MaxDelay:      90 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-5 * time.Second).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(10 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "fibonacci delay with more events",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "fibonacci",
				Delay:         5 * time.Second,
				MaxDelay:      90 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          10 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          15 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          25 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(40 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "fibonacci delay with delay ceiling reached",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "fibonacci",
				Delay:         5 * time.Second,
				MaxDelay:      50 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-15 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          10 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          15 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          25 * time.Second,
						},
						{
							RescheduleTime: now.Add(-40 * time.Second).UTC().UnixNano(),
							Delay:          40 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-15 * time.Second).Add(50 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "fibonacci delay with delay reset condition met",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "fibonacci",
				Delay:         5 * time.Second,
				MaxDelay:      50 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-5 * time.Minute)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          10 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          15 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          25 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          40 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-5 * time.Minute).Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "fibonacci delay with the most recent event that reset delay value",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "fibonacci",
				Delay:         5 * time.Second,
				MaxDelay:      50 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-5 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          10 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          15 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          25 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          40 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          50 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Minute).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-5 * time.Second).Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			j := testJob()
			if tc.reschedulePolicy != nil {
				j.TaskGroups[0].ReschedulePolicy = tc.reschedulePolicy
			}
			tc.alloc.Job = j
			tc.alloc.TaskGroup = j.TaskGroups[0].Name
			reschedTime, allowed := tc.alloc.NextRescheduleTime()
			must.Eq(t, tc.expectedRescheduleEligible, allowed)
			must.Eq(t, tc.expectedRescheduleTime, reschedTime)
		})
	}

}

func TestAllocation_NeedsToReconnect(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		states   []*AllocState
		expected bool
	}{
		{
			name:     "no state",
			expected: false,
		},
		{
			name:     "never disconnected",
			states:   []*AllocState{},
			expected: false,
		},
		{
			name: "disconnected once",
			states: []*AllocState{
				{
					Field: AllocStateFieldClientStatus,
					Value: AllocClientStatusUnknown,
					Time:  time.Now(),
				},
			},
			expected: true,
		},
		{
			name: "disconnect reconnect disconnect",
			states: []*AllocState{
				{
					Field: AllocStateFieldClientStatus,
					Value: AllocClientStatusUnknown,
					Time:  time.Now().Add(-2 * time.Minute),
				},
				{
					Field: AllocStateFieldClientStatus,
					Value: AllocClientStatusRunning,
					Time:  time.Now().Add(-1 * time.Minute),
				},
				{
					Field: AllocStateFieldClientStatus,
					Value: AllocClientStatusUnknown,
					Time:  time.Now(),
				},
			},
			expected: true,
		},
		{
			name: "disconnect multiple times before reconnect",
			states: []*AllocState{
				{
					Field: AllocStateFieldClientStatus,
					Value: AllocClientStatusUnknown,
					Time:  time.Now().Add(-2 * time.Minute),
				},
				{
					Field: AllocStateFieldClientStatus,
					Value: AllocClientStatusUnknown,
					Time:  time.Now().Add(-1 * time.Minute),
				},
				{
					Field: AllocStateFieldClientStatus,
					Value: AllocClientStatusRunning,
					Time:  time.Now(),
				},
			},
			expected: false,
		},
		{
			name: "disconnect after multiple updates",
			states: []*AllocState{
				{
					Field: AllocStateFieldClientStatus,
					Value: AllocClientStatusPending,
					Time:  time.Now().Add(-2 * time.Minute),
				},
				{
					Field: AllocStateFieldClientStatus,
					Value: AllocClientStatusRunning,
					Time:  time.Now().Add(-1 * time.Minute),
				},
				{
					Field: AllocStateFieldClientStatus,
					Value: AllocClientStatusUnknown,
					Time:  time.Now(),
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alloc := MockAlloc()
			alloc.AllocStates = tc.states

			got := alloc.NeedsToReconnect()
			must.Eq(t, tc.expected, got)
		})
	}
}

func TestAllocation_LastStartOfTask(t *testing.T) {
	ci.Parallel(t)
	testNow := time.Now()

	alloc := MockAlloc()
	alloc.TaskStates = map[string]*TaskState{
		"task-with-restarts": {
			StartedAt:   testNow.Add(-30 * time.Minute),
			Restarts:    3,
			LastRestart: testNow.Add(-5 * time.Minute),
		},
		"task-without-restarts": {
			StartedAt: testNow.Add(-30 * time.Minute),
			Restarts:  0,
		},
	}

	testCases := []struct {
		name     string
		taskName string
		expected time.Time
	}{
		{
			name:     "missing_task",
			taskName: "missing-task",
			expected: time.Time{},
		},
		{
			name:     "task_with_restarts",
			taskName: "task-with-restarts",
			expected: testNow.Add(-5 * time.Minute),
		},
		{
			name:     "task_without_restarts",
			taskName: "task-without-restarts",
			expected: testNow.Add(-30 * time.Minute),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alloc.TaskGroup = "web"
			got := alloc.LastStartOfTask(tc.taskName)

			must.Eq(t, tc.expected, got)
		})
	}
}

func TestAllocation_Canonicalize_Old(t *testing.T) {
	ci.Parallel(t)

	alloc := MockAlloc()
	alloc.AllocatedResources = nil
	alloc.TaskResources = map[string]*Resources{
		"web": {
			CPU:      500,
			MemoryMB: 256,
			Networks: []*NetworkResource{
				{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []Port{{Label: "admin", Value: 5000}},
					MBits:         50,
					DynamicPorts:  []Port{{Label: "http", Value: 9876}},
				},
			},
		},
	}
	alloc.SharedResources = &Resources{
		DiskMB: 150,
	}
	alloc.Canonicalize()

	expected := &AllocatedResources{
		Tasks: map[string]*AllocatedTaskResources{
			"web": {
				Cpu: AllocatedCpuResources{
					CpuShares: 500,
				},
				Memory: AllocatedMemoryResources{
					MemoryMB: 256,
				},
				Networks: []*NetworkResource{
					{
						Device:        "eth0",
						IP:            "192.168.0.100",
						ReservedPorts: []Port{{Label: "admin", Value: 5000}},
						MBits:         50,
						DynamicPorts:  []Port{{Label: "http", Value: 9876}},
					},
				},
			},
		},
		Shared: AllocatedSharedResources{
			DiskMB: 150,
		},
	}

	must.Eq(t, expected, alloc.AllocatedResources)
}
