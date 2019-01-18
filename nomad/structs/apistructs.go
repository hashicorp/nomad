package structs

import (
	"fmt"
	"time"

	"github.com/gorhill/cronexpr"
)

// This file contains objects that are shared between /nomad/structs and /api package.
// The packages need to be independent:
//
//  * /api package is public facing and should be minimal
// .* /nomad/structs is an internal package and may reference some libraries (e.g. raft, codec libraries)
//

var (
	defaultServiceJobRestartPolicyDelay    = 15 * time.Second
	defaultServiceJobRestartPolicyAttempts = 2
	defaultServiceJobRestartPolicyInterval = 30 * time.Minute
	defaultServiceJobRestartPolicyMode     = RestartPolicyModeFail

	defaultBatchJobRestartPolicyDelay    = 15 * time.Second
	defaultBatchJobRestartPolicyAttempts = 3
	defaultBatchJobRestartPolicyInterval = 24 * time.Hour
	defaultBatchJobRestartPolicyMode     = RestartPolicyModeFail
)

const (
	NodeStatusInit  = "initializing"
	NodeStatusReady = "ready"
	NodeStatusDown  = "down"

	// NodeSchedulingEligible and Ineligible marks the node as eligible or not,
	// respectively, for receiving allocations. This is orthoginal to the node
	// status being ready.
	NodeSchedulingEligible   = "eligible"
	NodeSchedulingIneligible = "ineligible"
)

const (
	AllocDesiredStatusRun   = "run"   // Allocation should run
	AllocDesiredStatusStop  = "stop"  // Allocation should stop
	AllocDesiredStatusEvict = "evict" // Allocation should stop, and was evicted
)

const (
	AllocClientStatusPending  = "pending"
	AllocClientStatusRunning  = "running"
	AllocClientStatusComplete = "complete"
	AllocClientStatusFailed   = "failed"
	AllocClientStatusLost     = "lost"
)

var (
	defaultServiceJobReschedulePolicyAttempts      = 0
	defaultServiceJobReschedulePolicyInterval      = time.Duration(0)
	defaultServiceJobReschedulePolicyDelay         = 30 * time.Second
	defaultServiceJobReschedulePolicyDelayFunction = "exponential"
	defaultServiceJobReschedulePolicyMaxDelay      = 1 * time.Hour
	defaultServiceJobReschedulePolicyUnlimited     = true

	defaultBatchJobReschedulePolicyAttempts      = 1
	defaultBatchJobReschedulePolicyInterval      = 24 * time.Hour
	defaultBatchJobReschedulePolicyDelay         = 5 * time.Second
	defaultBatchJobReschedulePolicyDelayFunction = "constant"
	defaultBatchJobReschedulePolicyMaxDelay      = time.Duration(0)
	defaultBatchJobReschedulePolicyUnlimited     = false
)

const (
	// RestartPolicyModeDelay causes an artificial delay till the next interval is
	// reached when the specified attempts have been reached in the interval.
	RestartPolicyModeDelay = "delay"

	// RestartPolicyModeFail causes a job to fail if the specified number of
	// attempts are reached within an interval.
	RestartPolicyModeFail = "fail"
)

// CronParseNext is a helper that parses the next time for the given expression
// but captures any panic that may occur in the underlying library.
func CronParseNext(e *cronexpr.Expression, fromTime time.Time, spec string) (t time.Time, err error) {
	defer func() {
		if recover() != nil {
			t = time.Time{}
			err = fmt.Errorf("failed parsing cron expression: %q", spec)
		}
	}()

	return e.Next(fromTime), nil
}
