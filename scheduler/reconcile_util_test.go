// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestAllocSet_filterByTainted(t *testing.T) {
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
		all                         allocSet
		taintedNodes                map[string]*structs.Node
		supportsDisconnectedClients bool
		skipNilNodeTest             bool
		now                         time.Time
		// expected results
		untainted     allocSet
		migrate       allocSet
		lost          allocSet
		disconnecting allocSet
		reconnecting  allocSet
		ignore        allocSet
	}

	testCases := []testCase{
		// These two cases test that we maintain parity with pre-disconnected-clients behavior.
		{
			name:                        "lost-client",
			supportsDisconnectedClients: false,
			now:                         time.Now(),
			taintedNodes:                nodes,
			skipNilNodeTest:             false,
			all: allocSet{
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
			untainted: allocSet{
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
			migrate: allocSet{
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
			disconnecting: allocSet{},
			reconnecting:  allocSet{},
			ignore:        allocSet{},
			lost:          allocSet{},
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
			all: allocSet{
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
			untainted:     allocSet{},
			migrate:       allocSet{},
			disconnecting: allocSet{},
			reconnecting:  allocSet{},
			ignore:        allocSet{},
			lost: allocSet{
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
			all: allocSet{
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
			untainted:     allocSet{},
			migrate:       allocSet{},
			disconnecting: allocSet{},
			reconnecting:  allocSet{},
			ignore:        allocSet{},
			lost: allocSet{
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
			all: allocSet{
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
			untainted: allocSet{
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
			migrate:       allocSet{},
			disconnecting: allocSet{},
			reconnecting: allocSet{
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
			ignore: allocSet{},
			lost:   allocSet{},
		},
		{
			name:                        "disco-client-reconnecting-running-no-replacement",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
			skipNilNodeTest:             false,
			all: allocSet{
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
			untainted:     allocSet{},
			migrate:       allocSet{},
			disconnecting: allocSet{},
			reconnecting: allocSet{
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
			ignore: allocSet{},
			lost:   allocSet{},
		},
		{
			name:                        "disco-client-terminal",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
			skipNilNodeTest:             false,
			all: allocSet{
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
			untainted:     allocSet{},
			migrate:       allocSet{},
			disconnecting: allocSet{},
			reconnecting: allocSet{
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
			ignore: allocSet{

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
			lost: allocSet{},
		},
		{
			name:                        "disco-client-disconnect",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
			skipNilNodeTest:             true,
			all: allocSet{
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
			untainted: allocSet{
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
			migrate: allocSet{},
			disconnecting: allocSet{
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
			reconnecting: allocSet{},
			ignore: allocSet{
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
			lost: allocSet{
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
			all: allocSet{
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
			untainted:     allocSet{},
			migrate:       allocSet{},
			disconnecting: allocSet{},
			reconnecting:  allocSet{},
			ignore:        allocSet{},
			lost: allocSet{
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
			all: allocSet{
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
			untainted: allocSet{
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
			migrate:       allocSet{},
			disconnecting: allocSet{},
			reconnecting: allocSet{
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
			ignore: allocSet{},
			lost:   allocSet{},
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
			all: allocSet{
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
			untainted: allocSet{
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
			migrate:       allocSet{},
			disconnecting: allocSet{},
			reconnecting:  allocSet{},
			ignore:        allocSet{},
			lost:          allocSet{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// With tainted nodes
			untainted, migrate, lost, disconnecting, reconnecting, ignore := tc.all.filterByTainted(tc.taintedNodes, tc.supportsDisconnectedClients, tc.now)
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
			untainted, migrate, lost, disconnecting, reconnecting, ignore = tc.all.filterByTainted(nil, tc.supportsDisconnectedClients, tc.now)
			must.Eq(t, tc.untainted, untainted, must.Sprintf("with-nodes: untainted"))
			must.Eq(t, tc.migrate, migrate, must.Sprintf("with-nodes: migrate"))
			must.Eq(t, tc.lost, lost, must.Sprintf("with-nodes: lost"))
			must.Eq(t, tc.disconnecting, disconnecting, must.Sprintf("with-nodes: disconnecting"))
			must.Eq(t, tc.reconnecting, reconnecting, must.Sprintf("with-nodes: reconnecting"))
			must.Eq(t, tc.ignore, ignore, must.Sprintf("with-nodes: ignore"))
		})
	}
}

func TestReconcile_shouldFilter(t *testing.T) {
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
			description:   "batch lost",
			batch:         true,
			desiredStatus: structs.AllocDesiredStatusStop,
			clientStatus:  structs.AllocClientStatusLost,
			untainted:     true,
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
		{
			description:   "service lost",
			batch:         false,
			desiredStatus: structs.AllocDesiredStatusStop,
			clientStatus:  structs.AllocClientStatusLost,
			untainted:     true,
			ignore:        false,
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

// Test that we properly create the bitmap even when the alloc set includes an
// allocation with a higher count than the current min count and it is byte
// aligned.
// Ensure no regression from: https://github.com/hashicorp/nomad/issues/3008
func TestBitmapFrom(t *testing.T) {
	ci.Parallel(t)

	input := map[string]*structs.Allocation{
		"8": {
			JobID:     "foo",
			TaskGroup: "bar",
			Name:      "foo.bar[8]",
		},
	}
	b, dups := bitmapFrom(input, 1)
	must.Eq(t, 16, b.Size())
	must.MapEmpty(t, dups)

	b, dups = bitmapFrom(input, 8)
	must.Eq(t, 16, b.Size())
	must.MapEmpty(t, dups)
}

func Test_allocNameIndex_Highest(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                string
		inputAllocNameIndex *allocNameIndex
		inputN              uint
		expectedOutput      map[string]struct{}
	}{
		{
			name: "select 1",
			inputAllocNameIndex: newAllocNameIndex(
				"example", "cache", 3, map[string]*structs.Allocation{
					"6b255fa3-c2cb-94de-5ddd-41aac25a6851": {
						Name:      "example.cache[0]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"e24771e6-8900-5d2d-ec93-e7076284774a": {
						Name:      "example.cache[1]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"d7842822-32c4-1a1c-bac8-66c3f20dfb0f": {
						Name:      "example.cache[2]",
						JobID:     "example",
						TaskGroup: "cache",
					},
				}),
			inputN: 1,
			expectedOutput: map[string]struct{}{
				"example.cache[2]": {},
			},
		},
		{
			name: "select all",
			inputAllocNameIndex: newAllocNameIndex(
				"example", "cache", 3, map[string]*structs.Allocation{
					"6b255fa3-c2cb-94de-5ddd-41aac25a6851": {
						Name:      "example.cache[0]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"e24771e6-8900-5d2d-ec93-e7076284774a": {
						Name:      "example.cache[1]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"d7842822-32c4-1a1c-bac8-66c3f20dfb0f": {
						Name:      "example.cache[2]",
						JobID:     "example",
						TaskGroup: "cache",
					},
				}),
			inputN: 3,
			expectedOutput: map[string]struct{}{
				"example.cache[2]": {},
				"example.cache[1]": {},
				"example.cache[0]": {},
			},
		},
		{
			name: "select too many",
			inputAllocNameIndex: newAllocNameIndex(
				"example", "cache", 3, map[string]*structs.Allocation{
					"6b255fa3-c2cb-94de-5ddd-41aac25a6851": {
						Name:      "example.cache[0]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"e24771e6-8900-5d2d-ec93-e7076284774a": {
						Name:      "example.cache[1]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"d7842822-32c4-1a1c-bac8-66c3f20dfb0f": {
						Name:      "example.cache[2]",
						JobID:     "example",
						TaskGroup: "cache",
					},
				}),
			inputN: 13,
			expectedOutput: map[string]struct{}{
				"example.cache[2]": {},
				"example.cache[1]": {},
				"example.cache[0]": {},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expectedOutput, tc.inputAllocNameIndex.Highest(tc.inputN))
		})
	}
}

func Test_allocNameIndex_NextCanaries(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                string
		inputAllocNameIndex *allocNameIndex
		inputN              uint
		inputExisting       allocSet
		inputDestructive    allocSet
		expectedOutput      []string
	}{
		{
			name: "single canary",
			inputAllocNameIndex: newAllocNameIndex(
				"example", "cache", 3, map[string]*structs.Allocation{
					"6b255fa3-c2cb-94de-5ddd-41aac25a6851": {
						Name:      "example.cache[0]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"e24771e6-8900-5d2d-ec93-e7076284774a": {
						Name:      "example.cache[1]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"d7842822-32c4-1a1c-bac8-66c3f20dfb0f": {
						Name:      "example.cache[2]",
						JobID:     "example",
						TaskGroup: "cache",
					},
				}),
			inputN:        1,
			inputExisting: nil,
			inputDestructive: map[string]*structs.Allocation{
				"6b255fa3-c2cb-94de-5ddd-41aac25a6851": {
					Name:      "example.cache[0]",
					JobID:     "example",
					TaskGroup: "cache",
				},
				"e24771e6-8900-5d2d-ec93-e7076284774a": {
					Name:      "example.cache[1]",
					JobID:     "example",
					TaskGroup: "cache",
				},
				"d7842822-32c4-1a1c-bac8-66c3f20dfb0f": {
					Name:      "example.cache[2]",
					JobID:     "example",
					TaskGroup: "cache",
				},
			},
			expectedOutput: []string{
				"example.cache[0]",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.SliceContainsAll(
				t, tc.expectedOutput,
				tc.inputAllocNameIndex.NextCanaries(tc.inputN, tc.inputExisting, tc.inputDestructive))
		})
	}
}

func Test_allocNameIndex_Next(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                string
		inputAllocNameIndex *allocNameIndex
		inputN              uint
		expectedOutput      []string
	}{
		{
			name:                "empty existing bitmap",
			inputAllocNameIndex: newAllocNameIndex("example", "cache", 3, nil),
			inputN:              3,
			expectedOutput: []string{
				"example.cache[0]", "example.cache[1]", "example.cache[2]",
			},
		},
		{
			name: "non-empty existing bitmap simple",
			inputAllocNameIndex: newAllocNameIndex(
				"example", "cache", 3, map[string]*structs.Allocation{
					"6b255fa3-c2cb-94de-5ddd-41aac25a6851": {
						Name:      "example.cache[0]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"e24771e6-8900-5d2d-ec93-e7076284774a": {
						Name:      "example.cache[1]",
						JobID:     "example",
						TaskGroup: "cache",
					},
					"d7842822-32c4-1a1c-bac8-66c3f20dfb0f": {
						Name:      "example.cache[2]",
						JobID:     "example",
						TaskGroup: "cache",
					},
				}),
			inputN: 3,
			expectedOutput: []string{
				"example.cache[0]", "example.cache[1]", "example.cache[2]",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.SliceContainsAll(t, tc.expectedOutput, tc.inputAllocNameIndex.Next(tc.inputN))
		})
	}
}

func Test_allocNameIndex_Duplicates(t *testing.T) {
	ci.Parallel(t)

	inputAllocSet := map[string]*structs.Allocation{
		"6b255fa3-c2cb-94de-5ddd-41aac25a6851": {
			Name:      "example.cache[0]",
			JobID:     "example",
			TaskGroup: "cache",
		},
		"e24771e6-8900-5d2d-ec93-e7076284774a": {
			Name:      "example.cache[1]",
			JobID:     "example",
			TaskGroup: "cache",
		},
		"d7842822-32c4-1a1c-bac8-66c3f20dfb0f": {
			Name:      "example.cache[2]",
			JobID:     "example",
			TaskGroup: "cache",
		},
		"76a6a487-016b-2fc2-8295-d811473ca93d": {
			Name:      "example.cache[0]",
			JobID:     "example",
			TaskGroup: "cache",
		},
	}

	// Build the tracker, and check some key information.
	allocNameIndexTracker := newAllocNameIndex("example", "cache", 4, inputAllocSet)
	must.Eq(t, 8, allocNameIndexTracker.b.Size())
	must.MapLen(t, 1, allocNameIndexTracker.duplicates)
	must.True(t, allocNameIndexTracker.IsDuplicate(0))

	// Unsetting the index should remove the duplicate entry, but not the entry
	// from the underlying bitmap.
	allocNameIndexTracker.UnsetIndex(0)
	must.MapLen(t, 0, allocNameIndexTracker.duplicates)
	must.True(t, allocNameIndexTracker.b.Check(0))

	// If we now select a new index, having previously checked for a duplicate,
	// we should get a non-duplicate.
	nextAllocNames := allocNameIndexTracker.Next(1)
	must.Len(t, 1, nextAllocNames)
	must.Eq(t, "example.cache[3]", nextAllocNames[0])
}

func TestAllocSet_filterByRescheduleable(t *testing.T) {
	ci.Parallel(t)

	noRescheduleJob := mock.Job()
	noRescheduleTG := &structs.TaskGroup{
		Name: "noRescheduleTG",
		ReschedulePolicy: &structs.ReschedulePolicy{
			Attempts:  0,
			Unlimited: false,
		},
	}

	noRescheduleJob.TaskGroups[0] = noRescheduleTG

	testJob := mock.Job()
	rescheduleTG := &structs.TaskGroup{
		Name: "rescheduleTG",
		ReschedulePolicy: &structs.ReschedulePolicy{
			Attempts:  1,
			Unlimited: false,
		},
	}
	testJob.TaskGroups[0] = rescheduleTG

	now := time.Now()

	type testCase struct {
		name                        string
		all                         allocSet
		isBatch                     bool
		supportsDisconnectedClients bool
		isDisconnecting             bool
		deployment                  *structs.Deployment

		// expected results
		untainted allocSet
		resNow    allocSet
		resLater  []*delayedRescheduleInfo
	}

	testCases := []testCase{
		{
			name:            "batch disconnecting allocation no reschedule",
			isDisconnecting: true,
			isBatch:         true,
			all: allocSet{
				"untainted1": {
					ID:           "untainted1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          noRescheduleJob,
					TaskGroup:    "noRescheduleTG",
				},
			},
			untainted: allocSet{
				"untainted1": {
					ID:           "untainted1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          noRescheduleJob,
					TaskGroup:    "noRescheduleTG",
				},
			},
			resNow:   allocSet{},
			resLater: []*delayedRescheduleInfo{},
		},
		{
			name:            "batch ignore unknown disconnecting allocs",
			isDisconnecting: true,
			isBatch:         true,
			all: allocSet{
				"disconnecting1": {
					ID:           "disconnection1",
					ClientStatus: structs.AllocClientStatusUnknown,
					Job:          testJob,
				},
			},
			untainted: allocSet{},
			resNow:    allocSet{},
			resLater:  []*delayedRescheduleInfo{},
		},
		{
			name:            "batch disconnecting allocation reschedule",
			isDisconnecting: true,
			isBatch:         true,
			all: allocSet{
				"rescheduleNow1": {
					ID:           "rescheduleNow1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					TaskGroup:    "rescheduleTG",
				},
			},
			untainted: allocSet{},
			resNow: allocSet{
				"rescheduleNow1": {
					ID:           "rescheduleNow1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					TaskGroup:    "rescheduleTG",
				},
			},
			resLater: []*delayedRescheduleInfo{},
		},
		{
			name:            "service disconnecting allocation no reschedule",
			isDisconnecting: true,
			isBatch:         false,
			all: allocSet{
				"untainted1": {
					ID:           "untainted1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          noRescheduleJob,
					TaskGroup:    "noRescheduleTG",
				},
			},
			untainted: allocSet{
				"untainted1": {
					ID:           "untainted1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          noRescheduleJob,
					TaskGroup:    "noRescheduleTG",
				},
			},
			resNow:   allocSet{},
			resLater: []*delayedRescheduleInfo{},
		},
		{
			name:            "service disconnecting allocation reschedule",
			isDisconnecting: true,
			isBatch:         false,
			all: allocSet{
				"rescheduleNow1": {
					ID:           "rescheduleNow1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					TaskGroup:    "rescheduleTG",
				},
			},
			untainted: allocSet{},
			resNow: allocSet{
				"rescheduleNow1": {
					ID:           "rescheduleNow1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					TaskGroup:    "rescheduleTG",
				},
			},
			resLater: []*delayedRescheduleInfo{},
		},
		{
			name:            "service ignore unknown disconnecting allocs",
			isDisconnecting: true,
			isBatch:         false,
			all: allocSet{
				"disconnecting1": {
					ID:           "disconnection1",
					ClientStatus: structs.AllocClientStatusUnknown,
					Job:          testJob,
				},
			},
			untainted: allocSet{},
			resNow:    allocSet{},
			resLater:  []*delayedRescheduleInfo{},
		},
		{
			name:            "service running allocation no reschedule",
			isDisconnecting: false,
			isBatch:         true,
			all: allocSet{
				"untainted1": {
					ID:           "untainted1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          noRescheduleJob,
					TaskGroup:    "noRescheduleTG",
				},
			},
			untainted: allocSet{
				"untainted1": {
					ID:           "untainted1",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          noRescheduleJob,
					TaskGroup:    "noRescheduleTG",
				},
			},
			resNow:   allocSet{},
			resLater: []*delayedRescheduleInfo{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			untainted, resNow, resLater := tc.all.filterByRescheduleable(tc.isBatch,
				tc.isDisconnecting, now, "evailID", tc.deployment)
			must.Eq(t, tc.untainted, untainted, must.Sprintf("with-nodes: untainted"))
			must.Eq(t, tc.resNow, resNow, must.Sprintf("with-nodes: reschedule-now"))
			must.Eq(t, tc.resLater, resLater, must.Sprintf("with-nodes: rescheduleLater"))
		})
	}
}
