package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
	"time"
)

// Test that we properly create the bitmap even when the alloc set includes an
// allocation with a higher count than the current min count and it is byte
// aligned.
// Ensure no regression from: https://github.com/hashicorp/nomad/issues/3008
func TestBitmapFrom(t *testing.T) {
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

	type testCase struct {
		name                        string
		all                         allocSet
		taintedNodes                map[string]*structs.Node
		supportsDisconnectedClients bool
		now                         time.Time
		// expected results
		untainted     allocSet
		migrate       allocSet
		lost          allocSet
		disconnecting allocSet
		reconnecting  allocSet
		ignore        allocSet
	}

	testCases := []*testCase{
		// This first case tests that we maintain parity with pre-disconnected-clients behavior.
		{
			name:                        "lost-client",
			supportsDisconnectedClients: false,
			now:                         time.Now(),
			taintedNodes:                nodes,
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

		// Everything below this line tests the disconnected client mode.
		{
			name:                        "disco-client-ignore-reconnect-failed-and-replaced",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
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
				// Failed and replaced allocs on reconnected nodes are ignored
				"failed-original": {
					ID:           "failed-original",
					Name:         "web",
					ClientStatus: structs.AllocClientStatusFailed,
					Job:          testJob,
					NodeID:       "normal",
					TaskGroup:    "web",
					AllocStates:  unknownAllocState,
				},
			},
			untainted: allocSet{
				"running-replacement": {
					ID:                 "running-replacement",
					Name:               "web",
					ClientStatus:       structs.AllocClientStatusRunning,
					PreviousAllocation: "failed-original",
				},
			},
			ignore: allocSet{
				"failed-original": {
					ID:           "failed-original",
					Name:         "web",
					ClientStatus: structs.AllocClientStatusFailed,
					AllocStates:  unknownAllocState,
				},
			},
		},
		{
			name:                        "disco-client-untainted-reconnect-running-no-replacement",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
			all: allocSet{
				// Running allocs on reconnected nodes with no replacement are untainted.
				// Node.UpdateStatus has already handled syncing client state so this
				// should be a noop.
				"untainted-running-no-replacement": {
					ID:           "untainted-running-no-replacement",
					Name:         "web",
					ClientStatus: structs.AllocClientStatusRunning,
					Job:          testJob,
					NodeID:       "normal",
					TaskGroup:    "web",
					AllocStates:  unknownAllocState,
				},
			},
			untainted: allocSet{
				"untainted-running-no-replacement": {
					ID:           "untainted-running-no-replacement",
					Name:         "web",
					ClientStatus: structs.AllocClientStatusRunning,
					AllocStates:  unknownAllocState,
				},
			},
		},
		{
			name:                        "disco-client-terminal",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
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
				},
				// Failed allocs on reconnected nodes that are complete are untainted
				"untainted-reconnect-failed": {
					ID:           "untainted-reconnect-failed",
					Name:         "untainted-reconnect-failed",
					ClientStatus: structs.AllocClientStatusFailed,
					Job:          testJob,
					NodeID:       "normal",
					TaskGroup:    "web",
					AllocStates:  unknownAllocState,
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
					PreviousAllocation: "untainted-reconnect-failed",
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
					AllocStates:  unknownAllocState,
				},
				"untainted-reconnect-failed": {
					ID:           "untainted-reconnect-failed",
					Name:         "untainted-reconnect-failed",
					ClientStatus: structs.AllocClientStatusFailed,
					AllocStates:  unknownAllocState,
				},
				"untainted-reconnect-lost": {
					ID:           "untainted-reconnect-lost",
					Name:         "untainted-reconnect-lost",
					ClientStatus: structs.AllocClientStatusLost,
					AllocStates:  unknownAllocState,
				},
				"untainted-reconnect-complete-replacement": {
					ID:                 "untainted-reconnect-complete-replacement",
					Name:               "untainted-reconnect-complete",
					ClientStatus:       structs.AllocClientStatusComplete,
					AllocStates:        unknownAllocState,
					PreviousAllocation: "untainted-reconnect-complete",
				},
				// Replacement allocs on reconnected nodes that are failed are ignored
				"untainted-reconnect-failed-replacement": {
					ID:                 "untainted-reconnect-failed-replacement",
					Name:               "untainted-reconnect-failed",
					ClientStatus:       structs.AllocClientStatusFailed,
					AllocStates:        unknownAllocState,
					PreviousAllocation: "untainted-reconnect-failed",
				},
				"untainted-reconnect-lost-replacement": {
					ID:                 "untainted-reconnect-lost-replacement",
					Name:               "untainted-reconnect-lost",
					ClientStatus:       structs.AllocClientStatusLost,
					AllocStates:        unknownAllocState,
					PreviousAllocation: "untainted-reconnect-lost",
				},
			},
		},
		{
			name:                        "disco-client-disconnect",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
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
					ID:           "ignore-unknown",
					Name:         "ignore-unknown",
					ClientStatus: structs.AllocClientStatusUnknown,
					Job:          testJob,
					NodeID:       "disconnected",
					TaskGroup:    "web",
					AllocStates:  unknownAllocState,
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
			},
			disconnecting: allocSet{
				"disconnect-running": {
					ID:           "disconnect-running",
					Name:         "disconnect-running",
					ClientStatus: structs.AllocClientStatusRunning,
				},
			},
			ignore: allocSet{
				// Unknown allocs on disconnected nodes are ignored
				"ignore-unknown": {
					ID:           "ignore-unknown",
					Name:         "ignore-unknown",
					ClientStatus: structs.AllocClientStatusUnknown,
				},
			},
			lost: allocSet{
				"lost-unknown": {
					ID:           "lost-unknown",
					Name:         "lost-unknown",
					ClientStatus: structs.AllocClientStatusUnknown,
					AllocStates:  expiredAllocState,
				},
			},
		},
		{
			name:                        "disco-client-running-reconnecting-and-replacement-untainted",
			supportsDisconnectedClients: true,
			now:                         time.Now(),
			taintedNodes:                nodes,
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
				},
			},
			reconnecting: allocSet{
				"running-original": {
					ID:           "running-original",
					Name:         "web",
					ClientStatus: structs.AllocClientStatusRunning,
					AllocStates:  unknownAllocState,
				},
			},
			untainted: allocSet{
				"running-replacement": {
					ID:                 "running-replacement",
					Name:               "web",
					ClientStatus:       structs.AllocClientStatusRunning,
					PreviousAllocation: "running-original",
				},
			},
		},
	}

	validateAllocSet := func(t *testing.T, name, setName string, expectedSet, actualSet allocSet) {
		if len(expectedSet) != len(actualSet) {
			for id, _ := range expectedSet {
				_, ok := actualSet[id]
				if !ok {
					t.Logf("\nexpected %s for set %s: is missing %s", id, setName, id)
				}
			}
			require.Equal(t, len(expectedSet), len(actualSet), "len", name, setName)
		}

		for _, actual := range actualSet {
			require.NotNil(t, actual, "actual nil", name, setName)

			expected, ok := expectedSet[actual.ID]
			require.True(t, ok, "expected not found", actual.ID, name, setName)
			require.NotNil(t, expected, "expected not nil", name, setName)

			require.Equal(t, expected.ID, actual.ID, "ID", name, setName)
			require.Equal(t, expected.Name, actual.Name, "Name", name, setName)
			require.Equal(t, expected.ClientStatus, actual.ClientStatus, "ClientStatus", name, setName)

			if expected.PreviousAllocation != "" {
				require.Equal(t, expected.PreviousAllocation, actual.PreviousAllocation, "PreviousAllocation", name, setName)
			}

			if len(expected.AllocStates) > 0 {
				require.Equal(t, expected.AllocStates, actual.AllocStates, "AllocStates", name, setName)
			}
		}
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// With tainted nodes
			untainted, migrate, lost, disconnecting, reconnecting, ignore := tc.all.filterByTainted(tc.taintedNodes, tc.supportsDisconnectedClients, tc.now)
			validateAllocSet(t, tc.name+"-with-nodes", "untainted", tc.untainted, untainted)
			validateAllocSet(t, tc.name+"-with-nodes", "migrate", tc.migrate, migrate)
			validateAllocSet(t, tc.name+"-with-nodes", "lost", tc.lost, lost)
			validateAllocSet(t, tc.name+"-with-nodes", "disconnecting", tc.disconnecting, disconnecting)
			validateAllocSet(t, tc.name+"-with-nodes", "reconnecting", tc.reconnecting, reconnecting)
			validateAllocSet(t, tc.name+"-with-nodes", "ignore", tc.ignore, ignore)

			// Now again with nodes nil
			untainted, migrate, lost, disconnecting, reconnecting, ignore = tc.all.filterByTainted(tc.taintedNodes, tc.supportsDisconnectedClients, tc.now)
			validateAllocSet(t, tc.name+"-nodes-nil", "untainted", tc.untainted, untainted)
			validateAllocSet(t, tc.name+"-nodes-nil", "migrate", tc.migrate, migrate)
			validateAllocSet(t, tc.name+"-nodes-nil", "lost", tc.lost, lost)
			validateAllocSet(t, tc.name+"-nodes-nil", "disconnecting", tc.disconnecting, disconnecting)
			validateAllocSet(t, tc.name+"-nodes-nil", "reconnecting", tc.reconnecting, reconnecting)
			validateAllocSet(t, tc.name+"-nodes-nil", "ignore", tc.ignore, ignore)
		})
	}
}
