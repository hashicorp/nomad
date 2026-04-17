// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package apitests

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
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
				Attempts:      new(structs.DefaultBatchJobReschedulePolicy.Attempts),
				Interval:      new(structs.DefaultBatchJobReschedulePolicy.Interval),
				Delay:         new(structs.DefaultBatchJobReschedulePolicy.Delay),
				DelayFunction: new(structs.DefaultBatchJobReschedulePolicy.DelayFunction),
				MaxDelay:      new(structs.DefaultBatchJobReschedulePolicy.MaxDelay),
				Unlimited:     new(structs.DefaultBatchJobReschedulePolicy.Unlimited),
			},
		},
		{
			desc: "Empty job reschedule policy",
			jobReschedulePolicy: &api.ReschedulePolicy{
				Attempts:      new(0),
				Interval:      new(time.Duration(0)),
				Delay:         new(time.Duration(0)),
				MaxDelay:      new(time.Duration(0)),
				DelayFunction: new(""),
				Unlimited:     new(false),
			},
			taskReschedulePolicy: nil,
			expected: &api.ReschedulePolicy{
				Attempts:      new(0),
				Interval:      new(time.Duration(0)),
				Delay:         new(time.Duration(0)),
				MaxDelay:      new(time.Duration(0)),
				DelayFunction: new(""),
				Unlimited:     new(false),
			},
		},
		{
			desc: "Inherit from job",
			jobReschedulePolicy: &api.ReschedulePolicy{
				Attempts:      new(1),
				Interval:      new(20 * time.Second),
				Delay:         new(20 * time.Second),
				MaxDelay:      new(10 * time.Minute),
				DelayFunction: new("constant"),
				Unlimited:     new(false),
			},
			taskReschedulePolicy: nil,
			expected: &api.ReschedulePolicy{
				Attempts:      new(1),
				Interval:      new(20 * time.Second),
				Delay:         new(20 * time.Second),
				MaxDelay:      new(10 * time.Minute),
				DelayFunction: new("constant"),
				Unlimited:     new(false),
			},
		},
		{
			desc:                "Set in task",
			jobReschedulePolicy: nil,
			taskReschedulePolicy: &api.ReschedulePolicy{
				Attempts:      new(5),
				Interval:      new(2 * time.Minute),
				Delay:         new(20 * time.Second),
				MaxDelay:      new(10 * time.Minute),
				DelayFunction: new("constant"),
				Unlimited:     new(false),
			},
			expected: &api.ReschedulePolicy{
				Attempts:      new(5),
				Interval:      new(2 * time.Minute),
				Delay:         new(20 * time.Second),
				MaxDelay:      new(10 * time.Minute),
				DelayFunction: new("constant"),
				Unlimited:     new(false),
			},
		},
		{
			desc: "Merge from job",
			jobReschedulePolicy: &api.ReschedulePolicy{
				Attempts: new(1),
				Delay:    new(20 * time.Second),
				MaxDelay: new(10 * time.Minute),
			},
			taskReschedulePolicy: &api.ReschedulePolicy{
				Interval:      new(5 * time.Minute),
				DelayFunction: new("constant"),
				Unlimited:     new(false),
			},
			expected: &api.ReschedulePolicy{
				Attempts:      new(1),
				Interval:      new(5 * time.Minute),
				Delay:         new(20 * time.Second),
				MaxDelay:      new(10 * time.Minute),
				DelayFunction: new("constant"),
				Unlimited:     new(false),
			},
		},
		{
			desc: "Override from group",
			jobReschedulePolicy: &api.ReschedulePolicy{
				Attempts: new(1),
				MaxDelay: new(10 * time.Second),
			},
			taskReschedulePolicy: &api.ReschedulePolicy{
				Attempts:      new(5),
				Delay:         new(20 * time.Second),
				MaxDelay:      new(20 * time.Minute),
				DelayFunction: new("constant"),
				Unlimited:     new(false),
			},
			expected: &api.ReschedulePolicy{
				Attempts:      new(5),
				Interval:      new(structs.DefaultBatchJobReschedulePolicy.Interval),
				Delay:         new(20 * time.Second),
				MaxDelay:      new(20 * time.Minute),
				DelayFunction: new("constant"),
				Unlimited:     new(false),
			},
		},
		{
			desc: "Attempts from job, default interval",
			jobReschedulePolicy: &api.ReschedulePolicy{
				Attempts: new(1),
			},
			taskReschedulePolicy: nil,
			expected: &api.ReschedulePolicy{
				Attempts:      new(1),
				Interval:      new(structs.DefaultBatchJobReschedulePolicy.Interval),
				Delay:         new(structs.DefaultBatchJobReschedulePolicy.Delay),
				DelayFunction: new(structs.DefaultBatchJobReschedulePolicy.DelayFunction),
				MaxDelay:      new(structs.DefaultBatchJobReschedulePolicy.MaxDelay),
				Unlimited:     new(structs.DefaultBatchJobReschedulePolicy.Unlimited),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			job := &api.Job{
				ID:         new("test"),
				Reschedule: tc.jobReschedulePolicy,
				Type:       new(api.JobTypeBatch),
			}
			job.Canonicalize()
			tg := &api.TaskGroup{
				Name:             new("foo"),
				ReschedulePolicy: tc.taskReschedulePolicy,
			}
			tg.Canonicalize(job)
			assert.Equal(t, tc.expected, tg.ReschedulePolicy)
		})
	}
}
