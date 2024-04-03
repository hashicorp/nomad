// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
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

// Test using stop_after_client_disconnect, remove after its deprecated  in favor
// of Disconnect.StopOnClientAfter introduced in 1.8.0.
func TestAllocation_WaitClientStop(t *testing.T) {
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

			j.TaskGroups[0].StopAfterClientDisconnect = &tc.stop
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

// Test using max_client_disconnect, remove after its deprecated  in favor
// of Disconnect.LostAfter introduced in 1.8.0.
func TestAllocation_DisconnectTimeout(t *testing.T) {
	type testCase struct {
		desc          string
		maxDisconnect *time.Duration
	}

	testCases := []testCase{
		{
			desc:          "no max_client_disconnect",
			maxDisconnect: nil,
		},
		{
			desc:          "has max_client_disconnect",
			maxDisconnect: pointer.Of(30 * time.Second),
		},
		{
			desc:          "zero max_client_disconnect",
			maxDisconnect: pointer.Of(0 * time.Second),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			j := testJob()
			a := &Allocation{
				Job: j,
			}

			j.TaskGroups[0].MaxClientDisconnect = tc.maxDisconnect
			a.TaskGroup = j.TaskGroups[0].Name

			now := time.Now()

			reschedTime := a.DisconnectTimeout(now)

			if tc.maxDisconnect == nil {
				must.Equal(t, now, reschedTime, must.Sprint("expected to be now"))
			} else {
				difference := reschedTime.Sub(now)
				must.Eq(t, *tc.maxDisconnect, difference, must.Sprint("expected durations to be equal"))
			}
		})
	}
}

// Test using max_client_disconnect, remove after its deprecated  in favor
// of Disconnect.LostAfter introduced in 1.8.0.
func TestAllocation_Expired(t *testing.T) {
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
				alloc.Job.TaskGroups[0].MaxClientDisconnect = &maxDisconnect
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
