// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package checks

import (
	"net/http"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/exp/maps"
)

// GetCheckQuery extracts the needed info from c to actually execute the check.
func GetCheckQuery(c *structs.ServiceCheck) *Query {
	var protocol = c.Protocol // ensure appropriate default
	if c.Type == "http" && protocol == "" {
		protocol = "http"
	}
	return &Query{
		Mode:        structs.GetCheckMode(c),
		Type:        c.Type,
		Timeout:     c.Timeout,
		AddressMode: c.AddressMode,
		PortLabel:   c.PortLabel,
		Protocol:    protocol,
		Path:        c.Path,
		Method:      c.Method,
		Headers:     maps.Clone(c.Header),
		Body:        c.Body,
	}
}

// A Query is derived from a ServiceCheck and contains the minimal
// amount of information needed to actually execute that check.
type Query struct {
	Mode structs.CheckMode // readiness or healthiness
	Type string            // tcp or http

	Timeout time.Duration // connection / request timeout

	AddressMode string // host, driver, or alloc
	PortLabel   string // label or value

	Protocol string      // http checks only (http or https)
	Path     string      // http checks only
	Method   string      // http checks only
	Headers  http.Header // http checks only
	Body     string      // http checks only
}

// A QueryContext contains allocation and service parameters necessary for
// address resolution.
type QueryContext struct {
	ID               structs.CheckID
	CustomAddress    string
	ServicePortLabel string
	Networks         structs.Networks
	NetworkStatus    structs.NetworkStatus
	Ports            structs.AllocatedPorts

	Group   string
	Task    string
	Service string
	Check   string
}

// Stub creates a temporary QueryResult for the check of ID in the Pending state
// so we can represent the status of not being checked yet.
func Stub(
	id structs.CheckID, kind structs.CheckMode, now int64,
	group, task, service, check string,
) *structs.CheckQueryResult {
	return &structs.CheckQueryResult{
		ID:        id,
		Mode:      kind,
		Status:    structs.CheckPending,
		Output:    "nomad: waiting to run",
		Timestamp: now,
		Group:     group,
		Task:      task,
		Service:   service,
		Check:     check,
	}
}

// AllocationResults is a view of the check_id -> latest result for group and task
// checks in an allocation.
type AllocationResults map[structs.CheckID]*structs.CheckQueryResult

// ClientResults is a holistic view of alloc_id -> check_id -> latest result
// group and task checks across all allocations on a client.
type ClientResults map[string]AllocationResults

func (cr ClientResults) Insert(allocID string, result *structs.CheckQueryResult) {
	if _, exists := cr[allocID]; !exists {
		cr[allocID] = make(AllocationResults)
	}
	cr[allocID][result.ID] = result
}
