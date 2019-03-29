package apitests

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

// Verifies that reschedule policy is merged correctly
func TestTaskGroup_Canonicalize_ReschedulePolicy(t *testing.T) {
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
				Attempts:      intToPtr(structs.DefaultBatchJobReschedulePolicy.Attempts),
				Interval:      timeToPtr(structs.DefaultBatchJobReschedulePolicy.Interval),
				Delay:         timeToPtr(structs.DefaultBatchJobReschedulePolicy.Delay),
				DelayFunction: stringToPtr(structs.DefaultBatchJobReschedulePolicy.DelayFunction),
				MaxDelay:      timeToPtr(structs.DefaultBatchJobReschedulePolicy.MaxDelay),
				Unlimited:     boolToPtr(structs.DefaultBatchJobReschedulePolicy.Unlimited),
			},
		},
		{
			desc: "Empty job reschedule policy",
			jobReschedulePolicy: &api.ReschedulePolicy{
				Attempts:      intToPtr(0),
				Interval:      timeToPtr(0),
				Delay:         timeToPtr(0),
				MaxDelay:      timeToPtr(0),
				DelayFunction: stringToPtr(""),
				Unlimited:     boolToPtr(false),
			},
			taskReschedulePolicy: nil,
			expected: &api.ReschedulePolicy{
				Attempts:      intToPtr(0),
				Interval:      timeToPtr(0),
				Delay:         timeToPtr(0),
				MaxDelay:      timeToPtr(0),
				DelayFunction: stringToPtr(""),
				Unlimited:     boolToPtr(false),
			},
		},
		{
			desc: "Inherit from job",
			jobReschedulePolicy: &api.ReschedulePolicy{
				Attempts:      intToPtr(1),
				Interval:      timeToPtr(20 * time.Second),
				Delay:         timeToPtr(20 * time.Second),
				MaxDelay:      timeToPtr(10 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
			taskReschedulePolicy: nil,
			expected: &api.ReschedulePolicy{
				Attempts:      intToPtr(1),
				Interval:      timeToPtr(20 * time.Second),
				Delay:         timeToPtr(20 * time.Second),
				MaxDelay:      timeToPtr(10 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
		},
		{
			desc:                "Set in task",
			jobReschedulePolicy: nil,
			taskReschedulePolicy: &api.ReschedulePolicy{
				Attempts:      intToPtr(5),
				Interval:      timeToPtr(2 * time.Minute),
				Delay:         timeToPtr(20 * time.Second),
				MaxDelay:      timeToPtr(10 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
			expected: &api.ReschedulePolicy{
				Attempts:      intToPtr(5),
				Interval:      timeToPtr(2 * time.Minute),
				Delay:         timeToPtr(20 * time.Second),
				MaxDelay:      timeToPtr(10 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
		},
		{
			desc: "Merge from job",
			jobReschedulePolicy: &api.ReschedulePolicy{
				Attempts: intToPtr(1),
				Delay:    timeToPtr(20 * time.Second),
				MaxDelay: timeToPtr(10 * time.Minute),
			},
			taskReschedulePolicy: &api.ReschedulePolicy{
				Interval:      timeToPtr(5 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
			expected: &api.ReschedulePolicy{
				Attempts:      intToPtr(1),
				Interval:      timeToPtr(5 * time.Minute),
				Delay:         timeToPtr(20 * time.Second),
				MaxDelay:      timeToPtr(10 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
		},
		{
			desc: "Override from group",
			jobReschedulePolicy: &api.ReschedulePolicy{
				Attempts: intToPtr(1),
				MaxDelay: timeToPtr(10 * time.Second),
			},
			taskReschedulePolicy: &api.ReschedulePolicy{
				Attempts:      intToPtr(5),
				Delay:         timeToPtr(20 * time.Second),
				MaxDelay:      timeToPtr(20 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
			expected: &api.ReschedulePolicy{
				Attempts:      intToPtr(5),
				Interval:      timeToPtr(structs.DefaultBatchJobReschedulePolicy.Interval),
				Delay:         timeToPtr(20 * time.Second),
				MaxDelay:      timeToPtr(20 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
		},
		{
			desc: "Attempts from job, default interval",
			jobReschedulePolicy: &api.ReschedulePolicy{
				Attempts: intToPtr(1),
			},
			taskReschedulePolicy: nil,
			expected: &api.ReschedulePolicy{
				Attempts:      intToPtr(1),
				Interval:      timeToPtr(structs.DefaultBatchJobReschedulePolicy.Interval),
				Delay:         timeToPtr(structs.DefaultBatchJobReschedulePolicy.Delay),
				DelayFunction: stringToPtr(structs.DefaultBatchJobReschedulePolicy.DelayFunction),
				MaxDelay:      timeToPtr(structs.DefaultBatchJobReschedulePolicy.MaxDelay),
				Unlimited:     boolToPtr(structs.DefaultBatchJobReschedulePolicy.Unlimited),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			job := &api.Job{
				ID:         stringToPtr("test"),
				Reschedule: tc.jobReschedulePolicy,
				Type:       stringToPtr(api.JobTypeBatch),
			}
			job.Canonicalize()
			tg := &api.TaskGroup{
				Name:             stringToPtr("foo"),
				ReschedulePolicy: tc.taskReschedulePolicy,
			}
			tg.Canonicalize(job)
			assert.Equal(t, tc.expected, tg.ReschedulePolicy)
		})
	}
}
