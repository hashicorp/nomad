// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package alloc

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestSet_filterByTainted(t *testing.T) {
	ci.Parallel(t)

	nodes := map[string]*structs.Node{
		"draining": {
			ID:            "draining",
			DrainStrategy: mock.DrainNode().DrainStrategy,
		},
		"lost": {
			ID:     "lost",
			Status: structs.NodeStatusDown,
		},
		"nil": nil,
		"normal": {
			ID:     "normal",
			Status: structs.NodeStatusReady,
		},
		"disconnected": {
			ID:     "disconnected",
			Status: structs.NodeStatusDisconnected,
		},
	}

	testJob := mock.Job()
	testJob.TaskGroups[0].MaxClientDisconnect = pointer.Of(5 * time.Second)
	now := time.Now()

	testJobNoMaxDisconnect := mock.Job()
	testJobNoMaxDisconnect.TaskGroups[0].MaxClientDisconnect = nil

	unknownAllocState := []*structs.AllocState{{
		Field: structs.AllocStateFieldClientStatus,
		Value: structs.AllocClientStatusUnknown,
		Time:  now,
	}}

	expiredAllocState := []*structs.AllocState{{
		Field: structs.AllocStateFieldClientStatus,
		Value: structs.AllocClientStatusUnknown,
		Time:  now.Add(-60 * time.Second),
	}}

	reconnectedAllocState := []*structs.AllocState{
		{
			Field: structs.AllocStateFieldClientStatus,
			Value: structs.AllocClientStatusUnknown,
			Time:  now.Add(-time.Second),
		},
		{
			Field: structs.AllocStateFieldClientStatus,
			Value: structs.AllocClientStatusRunning,
			Time:  now,
		},
	}

	type testCase struct {
		name                        string
		all                         Set
		taintedNodes                map[string]*structs.Node
		supportsDisconnectedClients bool
		skipNilNodeTest             bool
		now                         time.Time
		// expected results
		untainted     Set
		migrate       Set
		lost          Set
		disconnecting Set
		reconnecting  Set
		ignore        Set
	}

	testCases := []testCase{
		// These two cases test that we maintain parity with pre-disconnected-clients behavior.
		{
			name:                        "lost-client",
			supportsDisconnectedClients: false,
			now:                         time.Now(),
			taintedNodes:                nodes,
			skipNilNodeTest:             false,
			all: Set{
				"untainted1": {
					ID:           "untainted1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					NodeID:       "normal",
				},
				// Terminal allocs are always untainted
				"untainted2": {
					ID:           "untainted2",
					ClientStatus: structs.AllocClientStatusComplete,
					Job:          testJob,
					NodeID:       "normal",
				},
				// Terminal allocs are always untainted, even on draining nodes
				"untainted3": {
					ID:           "untainted3",
					ClientStatus: structs.AllocClientStatusComplete,
					Job:          testJob,
					NodeID:       "draining",
				},
				// Terminal allocs are always untainted, even on lost nodes
				"untainted4": {
					ID:           "untainted4",
					ClientStatus: structs.AllocClientStatusComplete,
					Job:          testJob,
					NodeID:       "lost",
				},
				// Non-terminal alloc with migrate=true should migrate on a draining node
				"migrating1": {
					ID:                "migrating1",
					ClientStatus:      structs.AllocClientStatusRunning,
					DesiredTransition: structs.DesiredTransition{Migrate: pointer.Of(true)},
					Job:               testJob,
					NodeID:            "draining",
				},
				// Non-terminal alloc with migrate=true should migrate on an unknown node
				"migrating2": {
					ID:                "migrating2",
					ClientStatus:      structs.AllocClientStatusRunning,
					DesiredTransition: structs.DesiredTransition{Migrate: pointer.Of(true)},
					Job:               testJob,
					NodeID:            "nil",
				},
			},
			untainted: Set{
				"untainted1": {
					ID:           "untainted1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					NodeID:       "normal",
				},
				// Terminal allocs are always untainted
				"untainted2": {
					ID:           "untainted2",
					ClientStatus: structs.AllocClientStatusComplete,
					Job:          testJob,
					NodeID:       "normal",
				},
				// Terminal allocs are always untainted, even on draining nodes
				"untainted3": {
					ID:           "untainted3",
					ClientStatus: structs.AllocClientStatusComplete,
					Job:          testJob,
					NodeID:       "draining",
				},
				// Terminal allocs are always untainted, even on lost nodes
				"untainted4": {
					ID:           "untainted4",
					ClientStatus: structs.AllocClientStatusComplete,
					Job:          testJob,
					NodeID:       "lost",
				},
			},
			migrate: Set{
				// Non-terminal alloc with migrate=true should migrate on a draining node
				"migrating1": {
					ID:                "migrating1",
					ClientStatus:      structs.AllocClientStatusRunning,
					DesiredTransition: structs.DesiredTransition{Migrate: pointer.Of(true)},
					Job:               testJob,
					NodeID:            "draining",
				},
				// Non-terminal alloc with migrate=true should migrate on an unknown node
				"migrating2": {
					ID:                "migrating2",
					ClientStatus:      structs.AllocClientStatusRunning,
					DesiredTransition: structs.DesiredTransition{Migrate: pointer.Of(true)},
					Job:               testJob,
					NodeID:            "nil",
				},
			},
			disconnecting: Set{},
			reconnecting:  Set{},
			ignore:        Set{},
			lost:          Set{},
		},
		{
			name:                        "lost-client-only-tainted-nodes",
			supportsDisconnectedClients: false,
			now:                         time.Now(),
			taintedNodes:                nodes,
			// The logic associated with this test case can only trigger if there
			// is a tainted node. Therefore, testing with a nil node set produces
			// false failures, so don't perform that test if in this case.
			skipNilNodeTest: true,
			all: Set{
				// Non-terminal allocs on lost nodes are lost
				"lost1": {
					ID:           "lost1",
					ClientStatus: structs.AllocClientStatusPending,
					Job:          testJob,
					NodeID:       "lost",
				},
				// Non-terminal allocs on lost nodes are lost
				"lost2": {
					ID:           "lost2",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					NodeID:       "lost",
				},
			},
			untainted:     Set{},
			migrate:       Set{},
			disconnecting: Set{},
			reconnecting:  Set{},
			ignore:        Set{},
			lost: Set{
				// Non-terminal allocs on lost nodes are lost
				"lost1": {
					ID:           "lost1",
					ClientStatus: structs.AllocClientStatusPending,
					Job:          testJob,
					NodeID:       "lost",
				},
				// Non-terminal allocs on lost nodes are lost
				"lost2": {
					ID:           "lost2",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					NodeID:       "lost",
				},
			},
		},
		{
			name:                        "disco-client-disconnect-unset-max-disconnect",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
			skipNilNodeTest:             true,
			all: Set{
				// Non-terminal allocs on disconnected nodes w/o max-disconnect are lost
				"lost-running": {
					ID:            "lost-running",
					Name:          "lost-running",
					ClientStatus:  structs.AllocClientStatusRunning,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJobNoMaxDisconnect,
					NodeID:        "disconnected",
					TaskGroup:     "web",
				},
			},
			untainted:     Set{},
			migrate:       Set{},
			disconnecting: Set{},
			reconnecting:  Set{},
			ignore:        Set{},
			lost: Set{
				"lost-running": {
					ID:            "lost-running",
					Name:          "lost-running",
					ClientStatus:  structs.AllocClientStatusRunning,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJobNoMaxDisconnect,
					NodeID:        "disconnected",
					TaskGroup:     "web",
				},
			},
		},
		// Everything below this line tests the disconnected client mode.
		{
			name:                        "disco-client-untainted-reconnect-failed-and-replaced",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
			skipNilNodeTest:             false,
			all: Set{
				"running-replacement": {
					ID:                 "running-replacement",
					Name:               "web",
					ClientStatus:       structs.AllocClientStatusRunning,
					DesiredStatus:      structs.AllocDesiredStatusRun,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					PreviousAllocation: "failed-original",
				},
				// Failed and replaced allocs on reconnected nodes
				// that are still desired-running are reconnected so
				// we can stop them
				"failed-original": {
					ID:            "failed-original",
					Name:          "web",
					ClientStatus:  structs.AllocClientStatusFailed,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
			},
			untainted: Set{
				"running-replacement": {
					ID:                 "running-replacement",
					Name:               "web",
					ClientStatus:       structs.AllocClientStatusRunning,
					DesiredStatus:      structs.AllocDesiredStatusRun,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					PreviousAllocation: "failed-original",
				},
			},
			migrate:       Set{},
			disconnecting: Set{},
			reconnecting: Set{
				"failed-original": {
					ID:            "failed-original",
					Name:          "web",
					ClientStatus:  structs.AllocClientStatusFailed,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
			},
			ignore: Set{},
			lost:   Set{},
		},
		{
			name:                        "disco-client-reconnecting-running-no-replacement",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
			skipNilNodeTest:             false,
			all: Set{
				// Running allocs on reconnected nodes with no replacement are reconnecting.
				// Node.UpdateStatus has already handled syncing client state so this
				// should be a noop.
				"reconnecting-running-no-replacement": {
					ID:            "reconnecting-running-no-replacement",
					Name:          "web",
					ClientStatus:  structs.AllocClientStatusRunning,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
			},
			untainted:     Set{},
			migrate:       Set{},
			disconnecting: Set{},
			reconnecting: Set{
				"reconnecting-running-no-replacement": {
					ID:            "reconnecting-running-no-replacement",
					Name:          "web",
					ClientStatus:  structs.AllocClientStatusRunning,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
			},
			ignore: Set{},
			lost:   Set{},
		},
		{
			name:                        "disco-client-terminal",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
			skipNilNodeTest:             false,
			all: Set{
				// Allocs on reconnected nodes that are complete are ignored
				"ignored-reconnect-complete": {
					ID:            "ignored-reconnect-complete",
					Name:          "ignored-reconnect-complete",
					ClientStatus:  structs.AllocClientStatusComplete,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
				// Failed allocs on reconnected nodes are in reconnecting so that
				// they be marked with desired status stop at the server.
				"reconnecting-failed": {
					ID:            "reconnecting-failed",
					Name:          "reconnecting-failed",
					ClientStatus:  structs.AllocClientStatusFailed,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
				// Lost allocs on reconnected nodes don't get restarted
				"ignored-reconnect-lost": {
					ID:            "ignored-reconnect-lost",
					Name:          "ignored-reconnect-lost",
					ClientStatus:  structs.AllocClientStatusLost,
					DesiredStatus: structs.AllocDesiredStatusStop,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
				// Replacement allocs that are complete are ignored
				"ignored-reconnect-complete-replacement": {
					ID:                 "ignored-reconnect-complete-replacement",
					Name:               "ignored-reconnect-complete",
					ClientStatus:       structs.AllocClientStatusComplete,
					DesiredStatus:      structs.AllocDesiredStatusRun,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					AllocStates:        unknownAllocState,
					PreviousAllocation: "untainted-reconnect-complete",
				},
				// Replacement allocs on reconnected nodes that are failed are ignored
				"ignored-reconnect-failed-replacement": {
					ID:                 "ignored-reconnect-failed-replacement",
					Name:               "ignored-reconnect-failed",
					ClientStatus:       structs.AllocClientStatusFailed,
					DesiredStatus:      structs.AllocDesiredStatusStop,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					PreviousAllocation: "reconnecting-failed",
				},
				// Lost replacement allocs on reconnected nodes don't get restarted
				"ignored-reconnect-lost-replacement": {
					ID:                 "ignored-reconnect-lost-replacement",
					Name:               "ignored-reconnect-lost",
					ClientStatus:       structs.AllocClientStatusLost,
					DesiredStatus:      structs.AllocDesiredStatusStop,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					AllocStates:        unknownAllocState,
					PreviousAllocation: "untainted-reconnect-lost",
				},
			},
			untainted:     Set{},
			migrate:       Set{},
			disconnecting: Set{},
			reconnecting: Set{
				"reconnecting-failed": {
					ID:            "reconnecting-failed",
					Name:          "reconnecting-failed",
					ClientStatus:  structs.AllocClientStatusFailed,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
			},
			ignore: Set{

				"ignored-reconnect-complete": {
					ID:            "ignored-reconnect-complete",
					Name:          "ignored-reconnect-complete",
					ClientStatus:  structs.AllocClientStatusComplete,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
				"ignored-reconnect-lost": {
					ID:            "ignored-reconnect-lost",
					Name:          "ignored-reconnect-lost",
					ClientStatus:  structs.AllocClientStatusLost,
					DesiredStatus: structs.AllocDesiredStatusStop,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
				"ignored-reconnect-complete-replacement": {
					ID:                 "ignored-reconnect-complete-replacement",
					Name:               "ignored-reconnect-complete",
					ClientStatus:       structs.AllocClientStatusComplete,
					DesiredStatus:      structs.AllocDesiredStatusRun,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					AllocStates:        unknownAllocState,
					PreviousAllocation: "untainted-reconnect-complete",
				},
				"ignored-reconnect-failed-replacement": {
					ID:                 "ignored-reconnect-failed-replacement",
					Name:               "ignored-reconnect-failed",
					ClientStatus:       structs.AllocClientStatusFailed,
					DesiredStatus:      structs.AllocDesiredStatusStop,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					PreviousAllocation: "reconnecting-failed",
				},
				"ignored-reconnect-lost-replacement": {
					ID:                 "ignored-reconnect-lost-replacement",
					Name:               "ignored-reconnect-lost",
					ClientStatus:       structs.AllocClientStatusLost,
					DesiredStatus:      structs.AllocDesiredStatusStop,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					AllocStates:        unknownAllocState,
					PreviousAllocation: "untainted-reconnect-lost",
				},
			},
			lost: Set{},
		},
		{
			name:                        "disco-client-disconnect",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
			skipNilNodeTest:             true,
			all: Set{
				// Non-terminal allocs on disconnected nodes are disconnecting
				"disconnect-running": {
					ID:            "disconnect-running",
					Name:          "disconnect-running",
					ClientStatus:  structs.AllocClientStatusRunning,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "disconnected",
					TaskGroup:     "web",
				},
				// Unknown allocs on disconnected nodes are acknowledge, so they wont be rescheduled again
				"untainted-unknown": {
					ID:            "untainted-unknown",
					Name:          "untainted-unknown",
					ClientStatus:  structs.AllocClientStatusUnknown,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "disconnected",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
				// Unknown allocs on disconnected nodes are lost when expired
				"lost-unknown": {
					ID:            "lost-unknown",
					Name:          "lost-unknown",
					ClientStatus:  structs.AllocClientStatusUnknown,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "disconnected",
					TaskGroup:     "web",
					AllocStates:   expiredAllocState,
				},
				// Pending allocs on disconnected nodes are lost
				"lost-pending": {
					ID:            "lost-pending",
					Name:          "lost-pending",
					ClientStatus:  structs.AllocClientStatusPending,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "disconnected",
					TaskGroup:     "web",
				},
				// Expired allocs on reconnected clients are lost
				// Pending allocs on disconnected nodes are lost
				"lost-expired": {
					ID:            "lost-expired",
					Name:          "lost-expired",
					ClientStatus:  structs.AllocClientStatusUnknown,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   expiredAllocState,
				},
				// Failed and stopped allocs on disconnected nodes are ignored
				"ignore-reconnected-failed-stopped": {
					ID:            "ignore-reconnected-failed-stopped",
					Name:          "ignore-reconnected-failed-stopped",
					ClientStatus:  structs.AllocClientStatusFailed,
					DesiredStatus: structs.AllocDesiredStatusStop,
					Job:           testJob,
					NodeID:        "disconnected",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
			},
			untainted: Set{
				// Unknown allocs on disconnected nodes are acknowledge, so they wont be rescheduled again
				"untainted-unknown": {
					ID:            "untainted-unknown",
					Name:          "untainted-unknown",
					ClientStatus:  structs.AllocClientStatusUnknown,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "disconnected",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
			},
			migrate: Set{},
			disconnecting: Set{
				"disconnect-running": {
					ID:            "disconnect-running",
					Name:          "disconnect-running",
					ClientStatus:  structs.AllocClientStatusRunning,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "disconnected",
					TaskGroup:     "web",
				},
			},
			reconnecting: Set{},
			ignore: Set{
				"ignore-reconnected-failed-stopped": {
					ID:            "ignore-reconnected-failed-stopped",
					Name:          "ignore-reconnected-failed-stopped",
					ClientStatus:  structs.AllocClientStatusFailed,
					DesiredStatus: structs.AllocDesiredStatusStop,
					Job:           testJob,
					NodeID:        "disconnected",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
			},
			lost: Set{
				"lost-unknown": {
					ID:            "lost-unknown",
					Name:          "lost-unknown",
					ClientStatus:  structs.AllocClientStatusUnknown,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "disconnected",
					TaskGroup:     "web",
					AllocStates:   expiredAllocState,
				},
				"lost-pending": {
					ID:            "lost-pending",
					Name:          "lost-pending",
					ClientStatus:  structs.AllocClientStatusPending,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "disconnected",
					TaskGroup:     "web",
				},
				"lost-expired": {
					ID:            "lost-expired",
					Name:          "lost-expired",
					ClientStatus:  structs.AllocClientStatusUnknown,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   expiredAllocState,
				},
			},
		},
		{
			name:                        "disco-client-reconnect",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
			skipNilNodeTest:             false,
			all: Set{
				// Expired allocs on reconnected clients are lost
				"lost-expired-reconnect": {
					ID:            "lost-expired-reconnect",
					Name:          "lost-expired-reconnect",
					ClientStatus:  structs.AllocClientStatusUnknown,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   expiredAllocState,
				},
			},
			untainted:     Set{},
			migrate:       Set{},
			disconnecting: Set{},
			reconnecting:  Set{},
			ignore:        Set{},
			lost: Set{
				"lost-expired-reconnect": {
					ID:            "lost-expired-reconnect",
					Name:          "lost-expired-reconnect",
					ClientStatus:  structs.AllocClientStatusUnknown,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   expiredAllocState,
				},
			},
		},
		{
			name:                        "disco-client-running-reconnecting-and-replacement-untainted",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
			skipNilNodeTest:             false,
			all: Set{
				"running-replacement": {
					ID:                 "running-replacement",
					Name:               "web",
					ClientStatus:       structs.AllocClientStatusRunning,
					DesiredStatus:      structs.AllocDesiredStatusRun,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					PreviousAllocation: "running-original",
				},
				// Running and replaced allocs on reconnected nodes are reconnecting
				"running-original": {
					ID:            "running-original",
					Name:          "web",
					ClientStatus:  structs.AllocClientStatusRunning,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
			},
			untainted: Set{
				"running-replacement": {
					ID:                 "running-replacement",
					Name:               "web",
					ClientStatus:       structs.AllocClientStatusRunning,
					DesiredStatus:      structs.AllocDesiredStatusRun,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					PreviousAllocation: "running-original",
				},
			},
			migrate:       Set{},
			disconnecting: Set{},
			reconnecting: Set{
				"running-original": {
					ID:            "running-original",
					Name:          "web",
					ClientStatus:  structs.AllocClientStatusRunning,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
			},
			ignore: Set{},
			lost:   Set{},
		},
		{
			// After an alloc is reconnected, it should be considered
			// "untainted" instead of "reconnecting" to allow changes such as
			// job updates to be applied properly.
			name:                        "disco-client-reconnected-alloc-untainted",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
			skipNilNodeTest:             false,
			all: Set{
				"running-reconnected": {
					ID:            "running-reconnected",
					Name:          "web",
					ClientStatus:  structs.AllocClientStatusRunning,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   reconnectedAllocState,
				},
			},
			untainted: Set{
				"running-reconnected": {
					ID:            "running-reconnected",
					Name:          "web",
					ClientStatus:  structs.AllocClientStatusRunning,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "normal",
					TaskGroup:     "web",
					AllocStates:   reconnectedAllocState,
				},
			},
			migrate:       Set{},
			disconnecting: Set{},
			reconnecting:  Set{},
			ignore:        Set{},
			lost:          Set{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// With tainted nodes
			untainted, migrate, lost, disconnecting, reconnecting, ignore := tc.all.FilterByTainted(tc.taintedNodes, tc.supportsDisconnectedClients, tc.now)
			must.Eq(t, tc.untainted, untainted, must.Sprintf("with-nodes: untainted"))
			must.Eq(t, tc.migrate, migrate, must.Sprintf("with-nodes: migrate"))
			must.Eq(t, tc.lost, lost, must.Sprintf("with-nodes: lost"))
			must.Eq(t, tc.disconnecting, disconnecting, must.Sprintf("with-nodes: disconnecting"))
			must.Eq(t, tc.reconnecting, reconnecting, must.Sprintf("with-nodes: reconnecting"))
			must.Eq(t, tc.ignore, ignore, must.Sprintf("with-nodes: ignore"))

			if tc.skipNilNodeTest {
				return
			}

			// Now again with nodes nil
			untainted, migrate, lost, disconnecting, reconnecting, ignore = tc.all.FilterByTainted(nil, tc.supportsDisconnectedClients, tc.now)
			must.Eq(t, tc.untainted, untainted, must.Sprintf("with-nodes: untainted"))
			must.Eq(t, tc.migrate, migrate, must.Sprintf("with-nodes: migrate"))
			must.Eq(t, tc.lost, lost, must.Sprintf("with-nodes: lost"))
			must.Eq(t, tc.disconnecting, disconnecting, must.Sprintf("with-nodes: disconnecting"))
			must.Eq(t, tc.reconnecting, reconnecting, must.Sprintf("with-nodes: reconnecting"))
			must.Eq(t, tc.ignore, ignore, must.Sprintf("with-nodes: ignore"))
		})
	}
}

func Test_shouldFilter(t *testing.T) {
	testCases := []struct {
		description   string
		batch         bool
		failed        bool
		desiredStatus string
		clientStatus  string

		untainted bool
		ignore    bool
	}{
		{
			description:   "batch running",
			batch:         true,
			failed:        false,
			desiredStatus: structs.AllocDesiredStatusRun,
			clientStatus:  structs.AllocClientStatusRunning,
			untainted:     true,
			ignore:        false,
		},
		{
			description:   "batch stopped success",
			batch:         true,
			failed:        false,
			desiredStatus: structs.AllocDesiredStatusStop,
			clientStatus:  structs.AllocClientStatusRunning,
			untainted:     true,
			ignore:        false,
		},
		{
			description:   "batch stopped failed",
			batch:         true,
			failed:        true,
			desiredStatus: structs.AllocDesiredStatusStop,
			clientStatus:  structs.AllocClientStatusComplete,
			untainted:     false,
			ignore:        true,
		},
		{
			description:   "batch evicted",
			batch:         true,
			desiredStatus: structs.AllocDesiredStatusEvict,
			clientStatus:  structs.AllocClientStatusComplete,
			untainted:     false,
			ignore:        true,
		},
		{
			description:   "batch failed",
			batch:         true,
			desiredStatus: structs.AllocDesiredStatusRun,
			clientStatus:  structs.AllocClientStatusFailed,
			untainted:     false,
			ignore:        false,
		},
		{
			description:   "service running",
			batch:         false,
			failed:        false,
			desiredStatus: structs.AllocDesiredStatusRun,
			clientStatus:  structs.AllocClientStatusRunning,
			untainted:     false,
			ignore:        false,
		},
		{
			description:   "service stopped",
			batch:         false,
			failed:        false,
			desiredStatus: structs.AllocDesiredStatusStop,
			clientStatus:  structs.AllocClientStatusComplete,
			untainted:     false,
			ignore:        true,
		},
		{
			description:   "service evicted",
			batch:         false,
			failed:        false,
			desiredStatus: structs.AllocDesiredStatusEvict,
			clientStatus:  structs.AllocClientStatusComplete,
			untainted:     false,
			ignore:        true,
		},
		{
			description:   "service client complete",
			batch:         false,
			failed:        false,
			desiredStatus: structs.AllocDesiredStatusRun,
			clientStatus:  structs.AllocClientStatusComplete,
			untainted:     false,
			ignore:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			alloc := &structs.Allocation{
				DesiredStatus: tc.desiredStatus,
				TaskStates:    map[string]*structs.TaskState{"task": {State: structs.TaskStateDead, Failed: tc.failed}},
				ClientStatus:  tc.clientStatus,
			}

			untainted, ignore := shouldFilter(alloc, tc.batch)
			must.Eq(t, tc.untainted, untainted)
			must.Eq(t, tc.ignore, ignore)
		})
	}
}
