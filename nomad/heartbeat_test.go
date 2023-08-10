// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestHeartbeat_InitializeHeartbeatTimers(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	node := mock.Node()
	state := s1.fsm.State()
	err := state.UpsertNode(structs.MsgTypeTestSetup, 1, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Reset the heartbeat timers
	err = s1.initializeHeartbeatTimers()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check that we have a timer
	_, ok := s1.heartbeatTimers[node.ID]
	if !ok {
		t.Fatalf("missing heartbeat timer")
	}
}

func TestHeartbeat_ResetHeartbeatTimer(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a new timer
	ttl, err := s1.resetHeartbeatTimer("test")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ttl < s1.config.MinHeartbeatTTL || ttl > 2*s1.config.MinHeartbeatTTL {
		t.Fatalf("bad: %#v", ttl)
	}

	// Check that we have a timer
	_, ok := s1.heartbeatTimers["test"]
	if !ok {
		t.Fatalf("missing heartbeat timer")
	}
}

func TestHeartbeat_ResetHeartbeatTimer_Nonleader(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3 // Won't become leader
	})
	defer cleanupS1()

	require.False(s1.IsLeader())

	// Create a new timer
	_, err := s1.resetHeartbeatTimer("test")
	require.NotNil(err)
	require.EqualError(err, heartbeatNotLeader)
}

func TestHeartbeat_ResetHeartbeatTimerLocked(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	s1.heartbeatTimersLock.Lock()
	s1.resetHeartbeatTimerLocked("foo", 5*time.Millisecond)
	s1.heartbeatTimersLock.Unlock()

	if _, ok := s1.heartbeatTimers["foo"]; !ok {
		t.Fatalf("missing timer")
	}

	time.Sleep(time.Duration(testutil.TestMultiplier()*10) * time.Millisecond)

	if _, ok := s1.heartbeatTimers["foo"]; ok {
		t.Fatalf("timer should be gone")
	}
}

func TestHeartbeat_ResetHeartbeatTimerLocked_Renew(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	s1.heartbeatTimersLock.Lock()
	s1.resetHeartbeatTimerLocked("foo", 30*time.Millisecond)
	s1.heartbeatTimersLock.Unlock()

	if _, ok := s1.heartbeatTimers["foo"]; !ok {
		t.Fatalf("missing timer")
	}

	time.Sleep(2 * time.Millisecond)

	// Renew the heartbeat
	s1.heartbeatTimersLock.Lock()
	s1.resetHeartbeatTimerLocked("foo", 30*time.Millisecond)
	s1.heartbeatTimersLock.Unlock()
	renew := time.Now()

	// Watch for invalidation
	for time.Now().Sub(renew) < time.Duration(testutil.TestMultiplier()*100)*time.Millisecond {
		s1.heartbeatTimersLock.Lock()
		_, ok := s1.heartbeatTimers["foo"]
		s1.heartbeatTimersLock.Unlock()
		if !ok {
			end := time.Now()
			if diff := end.Sub(renew); diff < 30*time.Millisecond {
				t.Fatalf("early invalidate %v", diff)
			}
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("should have expired")
}

func TestHeartbeat_InvalidateHeartbeat(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a node
	node := mock.Node()
	state := s1.fsm.State()
	require.NoError(state.UpsertNode(structs.MsgTypeTestSetup, 1, node))

	// This should cause a status update
	s1.invalidateHeartbeat(node.ID)

	// Check it is updated
	ws := memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	require.NoError(err)
	require.True(out.TerminalStatus())
	require.Len(out.Events, 2)
	require.Equal(NodeHeartbeatEventMissed, out.Events[1].Message)
}

func TestHeartbeat_ClearHeartbeatTimer(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	s1.heartbeatTimersLock.Lock()
	s1.resetHeartbeatTimerLocked("foo", 5*time.Millisecond)
	s1.heartbeatTimersLock.Unlock()

	err := s1.clearHeartbeatTimer("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, ok := s1.heartbeatTimers["foo"]; ok {
		t.Fatalf("timer should be gone")
	}
}

func TestHeartbeat_ClearAllHeartbeatTimers(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	s1.heartbeatTimersLock.Lock()
	s1.resetHeartbeatTimerLocked("foo", 10*time.Millisecond)
	s1.resetHeartbeatTimerLocked("bar", 10*time.Millisecond)
	s1.resetHeartbeatTimerLocked("baz", 10*time.Millisecond)
	s1.heartbeatTimersLock.Unlock()

	err := s1.clearAllHeartbeatTimers()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(s1.heartbeatTimers) != 0 {
		t.Fatalf("timers should be gone")
	}
}

func TestHeartbeat_Server_HeartbeatTTL_Failover(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
	})
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
	})
	defer cleanupS3()
	servers := []*Server{s1, s2, s3}
	TestJoin(t, s1, s2, s3)

	leader := waitForStableLeadership(t, servers)
	codec := rpcClient(t, leader)

	// Create the register request
	node := mock.Node()
	req := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check that heartbeatTimers has the heartbeat ID
	if _, ok := leader.heartbeatTimers[node.ID]; !ok {
		t.Fatalf("missing heartbeat timer")
	}

	// Shutdown the leader!
	leader.Shutdown()

	// heartbeatTimers should be cleared on leader shutdown
	testutil.WaitForResult(func() (bool, error) {
		return len(leader.heartbeatTimers) == 0, nil
	}, func(err error) {
		t.Fatalf("heartbeat timers should be empty on the shutdown leader")
	})

	// Find the new leader
	testutil.WaitForResult(func() (bool, error) {
		leader = nil
		for _, s := range servers {
			if s.IsLeader() {
				leader = s
			}
		}
		if leader == nil {
			return false, fmt.Errorf("Should have a new leader")
		}

		// Ensure heartbeat timer is restored
		if _, ok := leader.heartbeatTimers[node.ID]; !ok {
			return false, fmt.Errorf("missing heartbeat timer")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestHeartbeat_InvalidateHeartbeat_DisconnectedClient(t *testing.T) {
	ci.Parallel(t)

	type testCase struct {
		name                string
		now                 time.Time
		maxClientDisconnect *time.Duration
		expectedNodeStatus  string
	}

	testCases := []testCase{
		{
			name:                "has-pending-reconnects",
			now:                 time.Now().UTC(),
			maxClientDisconnect: pointer.Of(5 * time.Second),
			expectedNodeStatus:  structs.NodeStatusDisconnected,
		},
		{
			name:                "has-expired-reconnects",
			maxClientDisconnect: pointer.Of(5 * time.Second),
			now:                 time.Now().UTC().Add(-10 * time.Second),
			expectedNodeStatus:  structs.NodeStatusDown,
		},
		{
			name:                "has-expired-reconnects-equal-timestamp",
			maxClientDisconnect: pointer.Of(5 * time.Second),
			now:                 time.Now().UTC().Add(-5 * time.Second),
			expectedNodeStatus:  structs.NodeStatusDown,
		},
		{
			name:                "has-no-reconnects",
			now:                 time.Now().UTC(),
			maxClientDisconnect: nil,
			expectedNodeStatus:  structs.NodeStatusDown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s1, cleanupS1 := TestServer(t, nil)
			defer cleanupS1()
			testutil.WaitForLeader(t, s1.RPC)

			// Create a node
			node := mock.Node()
			state := s1.fsm.State()
			require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1, node))

			alloc := mock.Alloc()
			alloc.NodeID = node.ID
			alloc.Job.TaskGroups[0].MaxClientDisconnect = tc.maxClientDisconnect
			alloc.ClientStatus = structs.AllocClientStatusUnknown
			alloc.AllocStates = []*structs.AllocState{{
				Field: structs.AllocStateFieldClientStatus,
				Value: structs.AllocClientStatusUnknown,
				Time:  tc.now,
			}}
			require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 2, []*structs.Allocation{alloc}))

			// Trigger status update
			s1.invalidateHeartbeat(node.ID)
			out, err := state.NodeByID(nil, node.ID)
			require.NoError(t, err)
			require.Equal(t, tc.expectedNodeStatus, out.Status)
		})
	}
}
