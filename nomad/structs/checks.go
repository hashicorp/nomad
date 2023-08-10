// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"crypto/md5"
	"fmt"
)

// The CheckMode of a Nomad check is either Healthiness or Readiness.
type CheckMode string

const (
	// A Healthiness check is useful in the context of ensuring a service
	// is capable of performing its duties. This is an indicator that a check's
	// on_update configuration is set to "check_result", implying that Deployments
	// will not move forward while the check is failing.
	Healthiness CheckMode = "healthiness"

	// A Readiness check is useful in the context of ensuring a service
	// should be performing its duties (regardless of healthiness). This is an
	// indicator that the check's on_update configuration is set to "ignore",
	// implying that Deployments will move forward regardless if the check is
	// failing.
	Readiness CheckMode = "readiness"
)

// GetCheckMode determines whether the check is readiness or healthiness.
func GetCheckMode(c *ServiceCheck) CheckMode {
	if c != nil && c.OnUpdate == OnUpdateIgnore {
		return Readiness
	}
	return Healthiness
}

// An CheckID is unique to a check.
type CheckID string

// A CheckQueryResult represents the outcome of a single execution of a Nomad service
// check. It records the result, the output, and when the execution took place.
// Advanced check math (e.g. success_before_passing) are left to the calling
// context.
type CheckQueryResult struct {
	ID         CheckID
	Mode       CheckMode
	Status     CheckStatus
	StatusCode int `json:",omitempty"`
	Output     string
	Timestamp  int64

	// check coordinates
	Group   string
	Task    string `json:",omitempty"`
	Service string
	Check   string
}

func (r *CheckQueryResult) String() string {
	return fmt.Sprintf("(%s %s %s %v)", r.ID, r.Mode, r.Status, r.Timestamp)
}

// A CheckStatus is the result of executing a check. The status of a query is
// ternary - success, failure, or pending (not yet executed). Deployments treat
// pending and failure as the same - a deployment does not continue until a check
// is passing (unless on_update=ignore).
type CheckStatus string

const (
	CheckSuccess CheckStatus = "success"
	CheckFailure CheckStatus = "failure"
	CheckPending CheckStatus = "pending"
)

// NomadCheckID returns an ID unique to the nomad service check.
//
// Checks of group-level services have no task.
func NomadCheckID(allocID, group string, c *ServiceCheck) CheckID {
	sum := md5.New()
	hashString(sum, allocID)
	hashString(sum, group)
	hashString(sum, c.TaskName)
	hashString(sum, c.Name)
	hashString(sum, c.Type)
	hashString(sum, c.PortLabel)
	hashString(sum, c.OnUpdate)
	hashString(sum, c.AddressMode)
	hashDuration(sum, c.Interval)
	hashDuration(sum, c.Timeout)
	hashString(sum, c.Protocol)
	hashString(sum, c.Path)
	hashString(sum, c.Method)
	h := sum.Sum(nil)
	return CheckID(fmt.Sprintf("%x", h))
}
