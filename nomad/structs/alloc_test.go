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
			expected:      false,
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
