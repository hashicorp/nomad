// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apitests

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

// Verifies that reschedule policy is merged correctly
func TestTaskGroup_Canonicalize_ReschedulePolicy(t *testing.T) {
	ci.Parallel(t)

	type testCase struct {
		desc                 string
		jobReschedulePolicy  *api.ReschedulePolicy
		taskReschedulePolicy *api.ReschedulePolicy
		expected             *api.ReschedulePolicy
	}

	testCases := []testCase{
		{
			desc:                 "Default",
			jobReschedulePolicy:  nil,
			taskReschedulePolicy: nil,
			expected: &api.ReschedulePolicy{
				Attempts:      pointer.Of(structs.DefaultBatchJobReschedulePolicy.Attempts),
				Interval:      pointer.Of(structs.DefaultBatchJobReschedulePolicy.Interval),
				Delay:         pointer.Of(structs.DefaultBatchJobReschedulePolicy.Delay),
				DelayFunction: pointer.Of(structs.DefaultBatchJobReschedulePolicy.DelayFunction),
				MaxDelay:      pointer.Of(structs.DefaultBatchJobReschedulePolicy.MaxDelay),
				Unlimited:     pointer.Of(structs.DefaultBatchJobReschedulePolicy.Unlimited),
			},
		},
		{
			desc: "Empty job reschedule policy",
			jobReschedulePolicy: &api.ReschedulePolicy{
				Attempts:      pointer.Of(0),
				Interval:      pointer.Of(time.Duration(0)),
				Delay:         pointer.Of(time.Duration(0)),
				MaxDelay:      pointer.Of(time.Duration(0)),
				DelayFunction: pointer.Of(""),
				Unlimited:     pointer.Of(false),
			},
			taskReschedulePolicy: nil,
			expected: &api.ReschedulePolicy{
				Attempts:      pointer.Of(0),
				Interval:      pointer.Of(time.Duration(0)),
				Delay:         pointer.Of(time.Duration(0)),
				MaxDelay:      pointer.Of(time.Duration(0)),
				DelayFunction: pointer.Of(""),
				Unlimited:     pointer.Of(false),
			},
		},
		{
			desc: "Inherit from job",
			jobReschedulePolicy: &api.ReschedulePolicy{
				Attempts:      pointer.Of(1),
				Interval:      pointer.Of(20 * time.Second),
				Delay:         pointer.Of(20 * time.Second),
				MaxDelay:      pointer.Of(10 * time.Minute),
				DelayFunction: pointer.Of("constant"),
				Unlimited:     pointer.Of(false),
			},
			taskReschedulePolicy: nil,
			expected: &api.ReschedulePolicy{
				Attempts:      pointer.Of(1),
				Interval:      pointer.Of(20 * time.Second),
				Delay:         pointer.Of(20 * time.Second),
				MaxDelay:      pointer.Of(10 * time.Minute),
				DelayFunction: pointer.Of("constant"),
				Unlimited:     pointer.Of(false),
			},
		},
		{
			desc:                "Set in task",
			jobReschedulePolicy: nil,
			taskReschedulePolicy: &api.ReschedulePolicy{
				Attempts:      pointer.Of(5),
				Interval:      pointer.Of(2 * time.Minute),
				Delay:         pointer.Of(20 * time.Second),
				MaxDelay:      pointer.Of(10 * time.Minute),
				DelayFunction: pointer.Of("constant"),
				Unlimited:     pointer.Of(false),
			},
			expected: &api.ReschedulePolicy{
				Attempts:      pointer.Of(5),
				Interval:      pointer.Of(2 * time.Minute),
				Delay:         pointer.Of(20 * time.Second),
				MaxDelay:      pointer.Of(10 * time.Minute),
				DelayFunction: pointer.Of("constant"),
				Unlimited:     pointer.Of(false),
			},
		},
		{
			desc: "Merge from job",
			jobReschedulePolicy: &api.ReschedulePolicy{
				Attempts: pointer.Of(1),
				Delay:    pointer.Of(20 * time.Second),
				MaxDelay: pointer.Of(10 * time.Minute),
			},
			taskReschedulePolicy: &api.ReschedulePolicy{
				Interval:      pointer.Of(5 * time.Minute),
				DelayFunction: pointer.Of("constant"),
				Unlimited:     pointer.Of(false),
			},
			expected: &api.ReschedulePolicy{
				Attempts:      pointer.Of(1),
				Interval:      pointer.Of(5 * time.Minute),
				Delay:         pointer.Of(20 * time.Second),
				MaxDelay:      pointer.Of(10 * time.Minute),
				DelayFunction: pointer.Of("constant"),
				Unlimited:     pointer.Of(false),
			},
		},
		{
			desc: "Override from group",
			jobReschedulePolicy: &api.ReschedulePolicy{
				Attempts: pointer.Of(1),
				MaxDelay: pointer.Of(10 * time.Second),
			},
			taskReschedulePolicy: &api.ReschedulePolicy{
				Attempts:      pointer.Of(5),
				Delay:         pointer.Of(20 * time.Second),
				MaxDelay:      pointer.Of(20 * time.Minute),
				DelayFunction: pointer.Of("constant"),
				Unlimited:     pointer.Of(false),
			},
			expected: &api.ReschedulePolicy{
				Attempts:      pointer.Of(5),
				Interval:      pointer.Of(structs.DefaultBatchJobReschedulePolicy.Interval),
				Delay:         pointer.Of(20 * time.Second),
				MaxDelay:      pointer.Of(20 * time.Minute),
				DelayFunction: pointer.Of("constant"),
				Unlimited:     pointer.Of(false),
			},
		},
		{
			desc: "Attempts from job, default interval",
			jobReschedulePolicy: &api.ReschedulePolicy{
				Attempts: pointer.Of(1),
			},
			taskReschedulePolicy: nil,
			expected: &api.ReschedulePolicy{
				Attempts:      pointer.Of(1),
				Interval:      pointer.Of(structs.DefaultBatchJobReschedulePolicy.Interval),
				Delay:         pointer.Of(structs.DefaultBatchJobReschedulePolicy.Delay),
				DelayFunction: pointer.Of(structs.DefaultBatchJobReschedulePolicy.DelayFunction),
				MaxDelay:      pointer.Of(structs.DefaultBatchJobReschedulePolicy.MaxDelay),
				Unlimited:     pointer.Of(structs.DefaultBatchJobReschedulePolicy.Unlimited),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			job := &api.Job{
				ID:         pointer.Of("test"),
				Reschedule: tc.jobReschedulePolicy,
				Type:       pointer.Of(api.JobTypeBatch),
			}
			job.Canonicalize()
			tg := &api.TaskGroup{
				Name:             pointer.Of("foo"),
				ReschedulePolicy: tc.taskReschedulePolicy,
			}
			tg.Canonicalize(job)
			assert.Equal(t, tc.expected, tg.ReschedulePolicy)
		})
	}
}
