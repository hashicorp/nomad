package scheduler

import (
	"testing"

	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

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
	b := bitmapFrom(input, 1)
	exp := uint(16)
	if act := b.Size(); act != exp {
		t.Fatalf("got %d; want %d", act, exp)
	}

	b = bitmapFrom(input, 8)
	if act := b.Size(); act != exp {
		t.Fatalf("got %d; want %d", act, exp)
	}
}

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
	testJob.TaskGroups[0].MaxClientDisconnect = helper.TimeToPtr(5 * time.Second)
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

	reconnectedEvent := structs.NewTaskEvent(structs.TaskClientReconnected)
	reconnectedEvent.Time = time.Now().UnixNano()
	reconnectTaskState := map[string]*structs.TaskState{
		testJob.TaskGroups[0].Tasks[0].Name: {
			Events: []*structs.TaskEvent{reconnectedEvent},
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
					DesiredTransition: structs.DesiredTransition{Migrate: helper.BoolToPtr(true)},
					Job:               testJob,
					NodeID:            "draining",
				},
				// Non-terminal alloc with migrate=true should migrate on an unknown node
				"migrating2": {
					ID:                "migrating2",
					ClientStatus:      structs.AllocClientStatusRunning,
					DesiredTransition: structs.DesiredTransition{Migrate: helper.BoolToPtr(true)},
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
					DesiredTransition: structs.DesiredTransition{Migrate: helper.BoolToPtr(true)},
					Job:               testJob,
					NodeID:            "draining",
				},
				// Non-terminal alloc with migrate=true should migrate on an unknown node
				"migrating2": {
					ID:                "migrating2",
					ClientStatus:      structs.AllocClientStatusRunning,
					DesiredTransition: structs.DesiredTransition{Migrate: helper.BoolToPtr(true)},
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
					ID:           "lost-running",
					Name:         "lost-running",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJobNoMaxDisconnect,
					NodeID:       "disconnected",
					TaskGroup:    "web",
				},
				// Unknown allocs on disconnected nodes w/o max-disconnect are lost
				"lost-unknown": {
					ID:            "lost-unknown",
					Name:          "lost-unknown",
					ClientStatus:  structs.AllocClientStatusUnknown,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJobNoMaxDisconnect,
					NodeID:        "disconnected",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
			},
			untainted:     allocSet{},
			migrate:       allocSet{},
			disconnecting: allocSet{},
			reconnecting:  allocSet{},
			ignore:        allocSet{},
			lost: allocSet{
				"lost-running": {
					ID:           "lost-running",
					Name:         "lost-running",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJobNoMaxDisconnect,
					NodeID:       "disconnected",
					TaskGroup:    "web",
				},
				"lost-unknown": {
					ID:            "lost-unknown",
					Name:          "lost-unknown",
					ClientStatus:  structs.AllocClientStatusUnknown,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJobNoMaxDisconnect,
					NodeID:        "disconnected",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
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
					TaskStates:    reconnectTaskState,
				},
			},
			untainted: allocSet{
				"running-replacement": {
					ID:                 "running-replacement",
					Name:               "web",
					ClientStatus:       structs.AllocClientStatusRunning,
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
					TaskStates:    reconnectTaskState,
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
					ID:           "reconnecting-running-no-replacement",
					Name:         "web",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					NodeID:       "normal",
					TaskGroup:    "web",
					AllocStates:  unknownAllocState,
					TaskStates:   reconnectTaskState,
				},
			},
			untainted:     allocSet{},
			migrate:       allocSet{},
			disconnecting: allocSet{},
			reconnecting: allocSet{
				"reconnecting-running-no-replacement": {
					ID:           "reconnecting-running-no-replacement",
					Name:         "web",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					NodeID:       "normal",
					TaskGroup:    "web",
					AllocStates:  unknownAllocState,
					TaskStates:   reconnectTaskState,
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
				// Allocs on reconnected nodes that are complete are untainted
				"untainted-reconnect-complete": {
					ID:           "untainted-reconnect-complete",
					Name:         "untainted-reconnect-complete",
					ClientStatus: structs.AllocClientStatusComplete,
					Job:          testJob,
					NodeID:       "normal",
					TaskGroup:    "web",
					AllocStates:  unknownAllocState,
					TaskStates:   reconnectTaskState,
				},
				// Failed allocs on reconnected nodes are in reconnecting so that
				// they be marked with desired status stop at the server.
				"reconnecting-failed": {
					ID:           "reconnecting-failed",
					Name:         "reconnecting-failed",
					ClientStatus: structs.AllocClientStatusFailed,
					Job:          testJob,
					NodeID:       "normal",
					TaskGroup:    "web",
					AllocStates:  unknownAllocState,
					TaskStates:   reconnectTaskState,
				},
				// Lost allocs on reconnected nodes don't get restarted
				"untainted-reconnect-lost": {
					ID:           "untainted-reconnect-lost",
					Name:         "untainted-reconnect-lost",
					ClientStatus: structs.AllocClientStatusLost,
					Job:          testJob,
					NodeID:       "normal",
					TaskGroup:    "web",
					AllocStates:  unknownAllocState,
					TaskStates:   reconnectTaskState,
				},
				// Replacement allocs that are complete are untainted
				"untainted-reconnect-complete-replacement": {
					ID:                 "untainted-reconnect-complete-replacement",
					Name:               "untainted-reconnect-complete",
					ClientStatus:       structs.AllocClientStatusComplete,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					AllocStates:        unknownAllocState,
					PreviousAllocation: "untainted-reconnect-complete",
				},
				// Replacement allocs on reconnected nodes that are failed are untainted
				"untainted-reconnect-failed-replacement": {
					ID:                 "untainted-reconnect-failed-replacement",
					Name:               "untainted-reconnect-failed",
					ClientStatus:       structs.AllocClientStatusFailed,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					AllocStates:        unknownAllocState,
					PreviousAllocation: "reconnecting-failed",
				},
				// Lost replacement allocs on reconnected nodes don't get restarted
				"untainted-reconnect-lost-replacement": {
					ID:                 "untainted-reconnect-lost-replacement",
					Name:               "untainted-reconnect-lost",
					ClientStatus:       structs.AllocClientStatusLost,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					AllocStates:        unknownAllocState,
					PreviousAllocation: "untainted-reconnect-lost",
				},
			},
			untainted: allocSet{
				"untainted-reconnect-complete": {
					ID:           "untainted-reconnect-complete",
					Name:         "untainted-reconnect-complete",
					ClientStatus: structs.AllocClientStatusComplete,
					Job:          testJob,
					NodeID:       "normal",
					TaskGroup:    "web",
					AllocStates:  unknownAllocState,
					TaskStates:   reconnectTaskState,
				},
				"untainted-reconnect-lost": {
					ID:           "untainted-reconnect-lost",
					Name:         "untainted-reconnect-lost",
					ClientStatus: structs.AllocClientStatusLost,
					Job:          testJob,
					NodeID:       "normal",
					TaskGroup:    "web",
					AllocStates:  unknownAllocState,
					TaskStates:   reconnectTaskState,
				},
				"untainted-reconnect-complete-replacement": {
					ID:                 "untainted-reconnect-complete-replacement",
					Name:               "untainted-reconnect-complete",
					ClientStatus:       structs.AllocClientStatusComplete,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					AllocStates:        unknownAllocState,
					PreviousAllocation: "untainted-reconnect-complete",
				},
				"untainted-reconnect-failed-replacement": {
					ID:                 "untainted-reconnect-failed-replacement",
					Name:               "untainted-reconnect-failed",
					ClientStatus:       structs.AllocClientStatusFailed,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					AllocStates:        unknownAllocState,
					PreviousAllocation: "reconnecting-failed",
				},
				"untainted-reconnect-lost-replacement": {
					ID:                 "untainted-reconnect-lost-replacement",
					Name:               "untainted-reconnect-lost",
					ClientStatus:       structs.AllocClientStatusLost,
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					AllocStates:        unknownAllocState,
					PreviousAllocation: "untainted-reconnect-lost",
				},
			},
			migrate:       allocSet{},
			disconnecting: allocSet{},
			reconnecting: allocSet{
				"reconnecting-failed": {
					ID:           "reconnecting-failed",
					Name:         "reconnecting-failed",
					ClientStatus: structs.AllocClientStatusFailed,
					Job:          testJob,
					NodeID:       "normal",
					TaskGroup:    "web",
					AllocStates:  unknownAllocState,
					TaskStates:   reconnectTaskState,
				},
			},
			ignore: allocSet{},
			lost:   allocSet{},
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
					ID:           "disconnect-running",
					Name:         "disconnect-running",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					NodeID:       "disconnected",
					TaskGroup:    "web",
				},
				// Unknown allocs on disconnected nodes are ignored
				"ignore-unknown": {
					ID:            "ignore-unknown",
					Name:          "ignore-unknown",
					ClientStatus:  structs.AllocClientStatusUnknown,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "disconnected",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
				// Unknown allocs on disconnected nodes are lost when expired
				"lost-unknown": {
					ID:           "lost-unknown",
					Name:         "lost-unknown",
					ClientStatus: structs.AllocClientStatusUnknown,
					Job:          testJob,
					NodeID:       "disconnected",
					TaskGroup:    "web",
					AllocStates:  expiredAllocState,
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
					TaskStates:    reconnectTaskState,
					AllocStates:   unknownAllocState, // TODO really?
				},
			},
			untainted: allocSet{},
			migrate:   allocSet{},
			disconnecting: allocSet{
				"disconnect-running": {
					ID:           "disconnect-running",
					Name:         "disconnect-running",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					NodeID:       "disconnected",
					TaskGroup:    "web",
				},
			},
			reconnecting: allocSet{},
			ignore: allocSet{
				// Unknown allocs on disconnected nodes are ignored
				"ignore-unknown": {
					ID:            "ignore-unknown",
					Name:          "ignore-unknown",
					ClientStatus:  structs.AllocClientStatusUnknown,
					DesiredStatus: structs.AllocDesiredStatusRun,
					Job:           testJob,
					NodeID:        "disconnected",
					TaskGroup:     "web",
					AllocStates:   unknownAllocState,
				},
				"ignore-reconnected-failed-stopped": {
					ID:            "ignore-reconnected-failed-stopped",
					Name:          "ignore-reconnected-failed-stopped",
					ClientStatus:  structs.AllocClientStatusFailed,
					DesiredStatus: structs.AllocDesiredStatusStop,
					Job:           testJob,
					NodeID:        "disconnected",
					TaskGroup:     "web",
					TaskStates:    reconnectTaskState,
					AllocStates:   unknownAllocState,
				},
			},
			lost: allocSet{
				"lost-unknown": {
					ID:           "lost-unknown",
					Name:         "lost-unknown",
					ClientStatus: structs.AllocClientStatusUnknown,
					Job:          testJob,
					NodeID:       "disconnected",
					TaskGroup:    "web",
					AllocStates:  expiredAllocState,
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
					Job:                testJob,
					NodeID:             "normal",
					TaskGroup:          "web",
					PreviousAllocation: "running-original",
				},
				// Running and replaced allocs on reconnected nodes are reconnecting
				"running-original": {
					ID:           "running-original",
					Name:         "web",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					NodeID:       "normal",
					TaskGroup:    "web",
					AllocStates:  unknownAllocState,
					TaskStates:   reconnectTaskState,
				},
			},
			untainted: allocSet{
				"running-replacement": {
					ID:                 "running-replacement",
					Name:               "web",
					ClientStatus:       structs.AllocClientStatusRunning,
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
					ID:           "running-original",
					Name:         "web",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					NodeID:       "normal",
					TaskGroup:    "web",
					AllocStates:  unknownAllocState,
					TaskStates:   reconnectTaskState,
				},
			},
			ignore: allocSet{},
			lost:   allocSet{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// With tainted nodes
			untainted, migrate, lost, disconnecting, reconnecting, ignore := tc.all.filterByTainted(tc.taintedNodes, tc.supportsDisconnectedClients, tc.now)
			require.Equal(t, tc.untainted, untainted, "with-nodes: %s", "untainted")
			require.Equal(t, tc.migrate, migrate, "with-nodes: %s", "migrate")
			require.Equal(t, tc.lost, lost, "with-nodes: %s", "lost")
			require.Equal(t, tc.disconnecting, disconnecting, "with-nodes: %s", "disconnecting")
			require.Equal(t, tc.reconnecting, reconnecting, "with-nodes: %s", "reconnecting")
			require.Equal(t, tc.ignore, ignore, "with-nodes: %s", "ignore")

			if tc.skipNilNodeTest {
				return
			}

			// Now again with nodes nil
			untainted, migrate, lost, disconnecting, reconnecting, ignore = tc.all.filterByTainted(nil, tc.supportsDisconnectedClients, tc.now)
			require.Equal(t, tc.untainted, untainted, "nodes-nil: %s", "untainted")
			require.Equal(t, tc.migrate, migrate, "nodes-nil: %s", "migrate")
			require.Equal(t, tc.lost, lost, "nodes-nil: %s", "lost")
			require.Equal(t, tc.disconnecting, disconnecting, "nodes-nil: %s", "disconnecting")
			require.Equal(t, tc.reconnecting, reconnecting, "nodes-nil: %s", "reconnecting")
			require.Equal(t, tc.ignore, ignore, "nodes-nil: %s", "ignore")
		})
	}
}
