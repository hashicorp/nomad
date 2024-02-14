// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestJobConfig_Validate_LostAfter_Disconnect(t *testing.T) {
	// Set up a job with an invalid Disconnect.LostAfter value
	job := testJob()
	timeout := -1 * time.Minute
	job.TaskGroups[0].Disconnect = &DisconnectStrategy{
		LostAfter:         timeout,
		StopOnClientAfter: &timeout,
	}

	err := job.Validate()
	must.Error(t, err)
	err = errors.Unwrap(err)

	require.Contains(t, err.Error(), errNegativeLostAfter.Error())
	require.Contains(t, err.Error(), errNegativeStopAfter.Error())
	require.Contains(t, err.Error(), errStopAndLost.Error())

	// Modify the job with a valid Disconnect.LostAfter value
	timeout = 1 * time.Minute
	job.TaskGroups[0].Disconnect = &DisconnectStrategy{
		LostAfter:         timeout,
		StopOnClientAfter: nil,
	}
	err = job.Validate()
	require.NoError(t, err)
}

func TestDisconnectStategy_Validate(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		strategy *DisconnectStrategy
		jobType  string
		err      error
	}{
		{
			name: "negative-stop-after",
			strategy: &DisconnectStrategy{
				StopOnClientAfter: pointer.Of(-1 * time.Second),
			},
			jobType: JobTypeService,
			err:     errNegativeStopAfter,
		},
		{
			name: "stop-after-on-system",
			strategy: &DisconnectStrategy{
				StopOnClientAfter: pointer.Of(1 * time.Second),
			},
			jobType: JobTypeSystem,
			err:     errStopAfterNonService,
		},
		{
			name: "negative-lost-after",
			strategy: &DisconnectStrategy{
				LostAfter: -1 * time.Second,
			},
			jobType: JobTypeService,
			err:     errNegativeLostAfter,
		},
		{
			name: "lost-after-and-stop-after-enabled",
			strategy: &DisconnectStrategy{
				LostAfter:         1 * time.Second,
				StopOnClientAfter: pointer.Of(1 * time.Second),
			},
			jobType: JobTypeService,
			err:     errStopAndLost,
		},
		{
			name: "invalid-reconcile",
			strategy: &DisconnectStrategy{
				LostAfter: 1 * time.Second,
				Reconcile: "invalid",
			},
			jobType: JobTypeService,
			err:     errInvalidReconcile,
		},
		{
			name: "valid-configuration",
			strategy: &DisconnectStrategy{
				LostAfter:         1 * time.Second,
				Reconcile:         ReconcileOptionKeepOriginal,
				Replace:           pointer.Of(true),
				StopOnClientAfter: nil,
			},
			jobType: JobTypeService,
			err:     nil,
		},
	}



	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			job := testJob()
			job.Type = c.jobType
			err := c.strategy.Validate(job)
			if !errors.Is(err, c.err) {
				t.Errorf("expected error %v, got %v", c.err, err)
			}
		})
	}
}

func TestJobConfig_Validate_StopAferClient_Disconnect(t *testing.T) {
	ci.Parallel(t)
	// Setup a system Job with Disconnect.StopOnClientAfter set, which is invalid
	job := testJob()
	job.Type = JobTypeSystem
	stop := 1 * time.Minute
	job.TaskGroups[0].Disconnect = &DisconnectStrategy{
		StopOnClientAfter: &stop,
	}

	err := job.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), errStopAfterNonService.Error())

	// Modify the job to a batch job with an invalid Disconnect.StopOnClientAfter value
	job.Type = JobTypeBatch
	invalid := -1 * time.Minute
	job.TaskGroups[0].Disconnect = &DisconnectStrategy{
		StopOnClientAfter: &invalid,
	}

	err = job.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), errNegativeStopAfter.Error())

	// Modify the job to a batch job with a valid Disconnect.StopOnClientAfter value
	job.Type = JobTypeBatch
	job.TaskGroups[0].Disconnect = &DisconnectStrategy{
		StopOnClientAfter: &stop,
	}
	err = job.Validate()
	require.NoError(t, err)
}

// Test using stop_after_client_disconnect, remove after its deprecated  in favor
// of Disconnect.StopOnClientAfter introduced in 1.8.0.
func TestJobConfig_Validate_StopAferClientDisconnect(t *testing.T) {
	ci.Parallel(t)
	// Setup a system Job with stop_after_client_disconnect set, which is invalid
	job := testJob()
	job.Type = JobTypeSystem
	stop := 1 * time.Minute
	job.TaskGroups[0].StopAfterClientDisconnect = &stop

	err := job.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "stop_after_client_disconnect can only be set in batch and service jobs")

	// Modify the job to a batch job with an invalid stop_after_client_disconnect value
	job.Type = JobTypeBatch
	invalid := -1 * time.Minute
	job.TaskGroups[0].StopAfterClientDisconnect = &invalid

	err = job.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "stop_after_client_disconnect must be a positive value")

	// Modify the job to a batch job with a valid stop_after_client_disconnect value
	job.Type = JobTypeBatch
	job.TaskGroups[0].StopAfterClientDisconnect = &stop
	err = job.Validate()
	require.NoError(t, err)
}

func TestJob_Validate_DisconnectRescheduleLost(t *testing.T) {
	ci.Parallel(t)

	// Craft our speciality jobspec to test this particular use-case.
	testDisconnectRescheduleLostJob := &Job{
		ID:     "gh19644",
		Name:   "gh19644",
		Region: "global",
		Type:   JobTypeSystem,
		TaskGroups: []*TaskGroup{
			{
				Name: "cache",
				Disconnect: &DisconnectStrategy{
					LostAfter: 1 * time.Hour,
					Replace:   pointer.Of(false),
				},
				Tasks: []*Task{
					{
						Name:   "redis",
						Driver: "docker",
						Config: map[string]interface{}{
							"image": "redis:7",
						},
						LogConfig: DefaultLogConfig(),
					},
				},
			},
		},
	}

	testDisconnectRescheduleLostJob.Canonicalize()

	must.NoError(t, testDisconnectRescheduleLostJob.Validate())
}

// Test using max_client_disconnect, remove after its deprecated  in favor
// of Disconnect.LostAfter introduced in 1.8.0.
func TestJobConfig_Validate_MaxClientDisconnect(t *testing.T) {
	// Set up a job with an invalid max_client_disconnect value
	job := testJob()
	timeout := -1 * time.Minute
	job.TaskGroups[0].MaxClientDisconnect = &timeout
	job.TaskGroups[0].StopAfterClientDisconnect = &timeout

	err := job.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "max_client_disconnect cannot be negative")
	require.Contains(t, err.Error(), "Task group cannot be configured with both max_client_disconnect and stop_after_client_disconnect")

	// Modify the job with a valid max_client_disconnect value
	timeout = 1 * time.Minute
	job.TaskGroups[0].MaxClientDisconnect = &timeout
	job.TaskGroups[0].StopAfterClientDisconnect = nil
	err = job.Validate()
	require.NoError(t, err)
}
