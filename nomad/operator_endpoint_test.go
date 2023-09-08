// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/snapshot"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperator_RaftGetConfiguration(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	arg := structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: s1.config.Region,
		},
	}
	var reply structs.RaftConfigurationResponse
	if err := msgpackrpc.CallWithCodec(codec, "Operator.RaftGetConfiguration", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	future := s1.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(future.Configuration().Servers) != 1 {
		t.Fatalf("bad: %v", future.Configuration().Servers)
	}
	me := future.Configuration().Servers[0]
	expected := structs.RaftConfigurationResponse{
		Servers: []*structs.RaftServer{
			{
				ID:           me.ID,
				Node:         fmt.Sprintf("%v.%v", s1.config.NodeName, s1.config.Region),
				Address:      me.Address,
				Leader:       true,
				Voter:        true,
				RaftProtocol: fmt.Sprintf("%d", s1.config.RaftConfig.ProtocolVersion),
			},
		},
		Index: future.Index(),
	}
	if !reflect.DeepEqual(reply, expected) {
		t.Fatalf("bad: got %+v; want %+v", reply, expected)
	}
}

func TestOperator_RaftGetConfiguration_ACL(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)
	state := s1.fsm.State()

	// Create ACL token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid", mock.NodePolicy(acl.PolicyWrite))

	arg := structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: s1.config.Region,
		},
	}

	// Try with no token and expect permission denied
	{
		var reply structs.RaftConfigurationResponse
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftGetConfiguration", &arg, &reply)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect permission denied
	{
		arg.AuthToken = invalidToken.SecretID
		var reply structs.RaftConfigurationResponse
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftGetConfiguration", &arg, &reply)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Use management token
	{
		arg.AuthToken = root.SecretID
		var reply structs.RaftConfigurationResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Operator.RaftGetConfiguration", &arg, &reply))

		future := s1.raft.GetConfiguration()
		assert.Nil(future.Error())
		assert.Len(future.Configuration().Servers, 1)

		me := future.Configuration().Servers[0]
		expected := structs.RaftConfigurationResponse{
			Servers: []*structs.RaftServer{
				{
					ID:           me.ID,
					Node:         fmt.Sprintf("%v.%v", s1.config.NodeName, s1.config.Region),
					Address:      me.Address,
					Leader:       true,
					Voter:        true,
					RaftProtocol: fmt.Sprintf("%d", s1.config.RaftConfig.ProtocolVersion),
				},
			},
			Index: future.Index(),
		}
		assert.Equal(expected, reply)
	}
}

func TestOperator_RaftRemovePeerByAddress(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = raft.ProtocolVersion(2)
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	ports := ci.PortAllocator.Grab(1)

	// Try to remove a peer that's not there.
	arg := structs.RaftPeerByAddressRequest{
		Address: raft.ServerAddress(fmt.Sprintf("127.0.0.1:%d", ports[0])),
	}
	arg.Region = s1.config.Region
	var reply struct{}
	err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByAddress", &arg, &reply)
	if err == nil || !strings.Contains(err.Error(), "not found in the Raft configuration") {
		t.Fatalf("err: %v", err)
	}

	// Add it manually to Raft.
	{
		future := s1.raft.AddPeer(arg.Address)
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Make sure it's there.
	{
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
		configuration := future.Configuration()
		if len(configuration.Servers) != 2 {
			t.Fatalf("bad: %v", configuration)
		}
	}

	// Remove it, now it should go through.
	if err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByAddress", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure it's not there.
	{
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
		configuration := future.Configuration()
		if len(configuration.Servers) != 1 {
			t.Fatalf("bad: %v", configuration)
		}
	}
}

func TestOperator_RaftRemovePeerByAddress_ACL(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = raft.ProtocolVersion(2)
	})

	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)
	state := s1.fsm.State()

	// Create ACL token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid", mock.NodePolicy(acl.PolicyWrite))

	ports := ci.PortAllocator.Grab(1)

	arg := structs.RaftPeerByAddressRequest{
		Address: raft.ServerAddress(fmt.Sprintf("127.0.0.1:%d", ports[0])),
	}
	arg.Region = s1.config.Region

	// Add peer manually to Raft.
	{
		future := s1.raft.AddPeer(arg.Address)
		assert.Nil(future.Error())
	}

	var reply struct{}

	// Try with no token and expect permission denied
	{
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByAddress", &arg, &reply)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect permission denied
	{
		arg.AuthToken = invalidToken.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByAddress", &arg, &reply)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a management token
	{
		arg.AuthToken = root.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByAddress", &arg, &reply)
		assert.Nil(err)
	}
}

func TestOperator_RaftRemovePeerByID(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Try to remove a peer that's not there.
	arg := structs.RaftPeerByIDRequest{
		ID: raft.ServerID("e35bde83-4e9c-434f-a6ef-453f44ee21ea"),
	}
	arg.Region = s1.config.Region
	var reply struct{}
	err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByID", &arg, &reply)
	if err == nil || !strings.Contains(err.Error(), "not found in the Raft configuration") {
		t.Fatalf("err: %v", err)
	}

	ports := ci.PortAllocator.Grab(1)

	// Add it manually to Raft.
	{
		future := s1.raft.AddVoter(arg.ID, raft.ServerAddress(fmt.Sprintf("127.0.0.1:%d", ports[0])), 0, 0)
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Make sure it's there.
	{
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
		configuration := future.Configuration()
		if len(configuration.Servers) != 2 {
			t.Fatalf("bad: %v", configuration)
		}
	}

	// Remove it, now it should go through.
	if err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByID", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure it's not there.
	{
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
		configuration := future.Configuration()
		if len(configuration.Servers) != 1 {
			t.Fatalf("bad: %v", configuration)
		}
	}
}

func TestOperator_RaftRemovePeerByID_ACL(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)
	state := s1.fsm.State()

	// Create ACL token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid", mock.NodePolicy(acl.PolicyWrite))

	arg := structs.RaftPeerByIDRequest{
		ID: raft.ServerID("e35bde83-4e9c-434f-a6ef-453f44ee21ea"),
	}
	arg.Region = s1.config.Region

	ports := ci.PortAllocator.Grab(1)

	// Add peer manually to Raft.
	{
		future := s1.raft.AddVoter(arg.ID, raft.ServerAddress(fmt.Sprintf("127.0.0.1:%d", ports[0])), 0, 0)
		assert.Nil(future.Error())
	}

	var reply struct{}

	// Try with no token and expect permission denied
	{
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByID", &arg, &reply)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect permission denied
	{
		arg.AuthToken = invalidToken.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByID", &arg, &reply)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a management token
	{
		arg.AuthToken = root.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByID", &arg, &reply)
		assert.Nil(err)
	}
}

func TestOperator_SchedulerGetConfiguration(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.Build = "0.9.0+unittest"
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	arg := structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: s1.config.Region,
		},
	}
	var reply structs.SchedulerConfigurationResponse
	if err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerGetConfiguration", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}
	require := require.New(t)
	require.NotZero(reply.Index)
	require.True(reply.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
}

func TestOperator_SchedulerSetConfiguration(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.Build = "0.9.0+unittest"
	})
	defer cleanupS1()
	rpcCodec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Disable preemption and pause the eval broker.
	arg := structs.SchedulerSetConfigRequest{
		Config: structs.SchedulerConfiguration{
			PreemptionConfig: structs.PreemptionConfig{
				SystemSchedulerEnabled: false,
			},
			PauseEvalBroker: true,
		},
	}
	arg.Region = s1.config.Region

	var setResponse structs.SchedulerSetConfigurationResponse
	err := msgpackrpc.CallWithCodec(rpcCodec, "Operator.SchedulerSetConfiguration", &arg, &setResponse)
	require.Nil(t, err)
	require.NotZero(t, setResponse.Index)

	// Read and verify that preemption is disabled and the eval and blocked
	// evals systems are disabled.
	readConfig := structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: s1.config.Region,
		},
	}
	var reply structs.SchedulerConfigurationResponse
	err = msgpackrpc.CallWithCodec(rpcCodec, "Operator.SchedulerGetConfiguration", &readConfig, &reply)
	require.NoError(t, err)

	require.NotZero(t, reply.Index)
	require.False(t, reply.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
	require.True(t, reply.SchedulerConfig.PauseEvalBroker)

	require.False(t, s1.evalBroker.Enabled())
	require.False(t, s1.blockedEvals.Enabled())
}

func TestOperator_SchedulerGetConfiguration_ACL(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = 3
		c.Build = "0.9.0+unittest"
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create ACL token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid", mock.NodePolicy(acl.PolicyWrite))

	arg := structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: s1.config.Region,
		},
	}
	require := require.New(t)
	var reply structs.SchedulerConfigurationResponse

	// Try with no token and expect permission denied
	{
		err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerGetConfiguration", &arg, &reply)
		require.NotNil(err)
		require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect permission denied
	{
		arg.AuthToken = invalidToken.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerGetConfiguration", &arg, &reply)
		require.NotNil(err)
		require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with root token, should succeed
	{
		arg.AuthToken = root.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerGetConfiguration", &arg, &reply)
		require.Nil(err)
	}

}

func TestOperator_SchedulerSetConfiguration_ACL(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = 3
		c.Build = "0.9.0+unittest"
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create ACL token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid", mock.NodePolicy(acl.PolicyWrite))

	arg := structs.SchedulerSetConfigRequest{
		Config: structs.SchedulerConfiguration{
			PreemptionConfig: structs.PreemptionConfig{
				SystemSchedulerEnabled: true,
			},
		},
	}
	arg.Region = s1.config.Region

	require := require.New(t)
	var reply structs.SchedulerSetConfigurationResponse

	// Try with no token and expect permission denied
	{
		err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerSetConfiguration", &arg, &reply)
		require.NotNil(err)
		require.Equal(structs.ErrPermissionDenied.Error(), err.Error())
	}

	// Try with an invalid token and expect permission denied
	{
		arg.AuthToken = invalidToken.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerSetConfiguration", &arg, &reply)
		require.NotNil(err)
		require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with root token, should succeed
	{
		arg.AuthToken = root.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerSetConfiguration", &arg, &reply)
		require.Nil(err)
	}

}

func TestOperator_SnapshotSave(t *testing.T) {
	ci.Parallel(t)

	////// Nomad clusters topology - not specific to test
	dir := t.TempDir()

	server1, cleanupLS := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DataDir = path.Join(dir, "server1")
	})
	defer cleanupLS()

	server2, cleanupRS := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DataDir = path.Join(dir, "server2")
	})
	defer cleanupRS()

	remoteRegionServer, cleanupRRS := TestServer(t, func(c *Config) {
		c.Region = "two"
		c.DevMode = false
		c.DataDir = path.Join(dir, "remote_region_server")
	})
	defer cleanupRRS()

	TestJoin(t, server1, server2)
	TestJoin(t, server1, remoteRegionServer)
	testutil.WaitForLeader(t, server1.RPC)
	testutil.WaitForLeader(t, server2.RPC)
	testutil.WaitForLeader(t, remoteRegionServer.RPC)

	leader, nonLeader := server1, server2
	if server2.IsLeader() {
		leader, nonLeader = server2, server1
	}

	/////////  Actually run query now
	cases := []struct {
		name   string
		server *Server
	}{
		{"leader", leader},
		{"non_leader", nonLeader},
		{"remote_region", remoteRegionServer},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			handler, err := c.server.StreamingRpcHandler("Operator.SnapshotSave")
			require.NoError(t, err)

			p1, p2 := net.Pipe()
			defer p1.Close()
			defer p2.Close()

			// start handler
			go handler(p2)

			var req structs.SnapshotSaveRequest
			var resp structs.SnapshotSaveResponse

			req.Region = "global"

			// send request
			encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
			err = encoder.Encode(&req)
			require.NoError(t, err)

			decoder := codec.NewDecoder(p1, structs.MsgpackHandle)
			err = decoder.Decode(&resp)
			require.NoError(t, err)
			require.Empty(t, resp.ErrorMsg)

			require.NotZero(t, resp.Index)
			require.NotEmpty(t, resp.SnapshotChecksum)
			require.Contains(t, resp.SnapshotChecksum, "sha-256=")

			index := resp.Index

			snap, err := os.CreateTemp("", "nomadtests-snapshot-")
			require.NoError(t, err)
			defer os.Remove(snap.Name())

			hash := sha256.New()
			_, err = io.Copy(io.MultiWriter(snap, hash), p1)
			require.NoError(t, err)

			expectedChecksum := "sha-256=" + base64.StdEncoding.EncodeToString(hash.Sum(nil))

			require.Equal(t, expectedChecksum, resp.SnapshotChecksum)

			_, err = snap.Seek(0, 0)
			require.NoError(t, err)

			meta, err := snapshot.Verify(snap)
			require.NoError(t, err)

			require.NotZerof(t, meta.Term, "snapshot term")
			require.Equal(t, index, meta.Index)
		})
	}
}

func TestOperator_SnapshotSave_ACL(t *testing.T) {
	ci.Parallel(t)

	////// Nomad clusters topology - not specific to test
	dir := t.TempDir()

	s, root, cleanupLS := TestACLServer(t, func(c *Config) {
		c.BootstrapExpect = 1
		c.DevMode = false
		c.DataDir = path.Join(dir, "server1")
	})
	defer cleanupLS()

	testutil.WaitForLeader(t, s.RPC)

	deniedToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1001, "test-invalid", mock.NodePolicy(acl.PolicyWrite))

	/////////  Actually run query now
	cases := []struct {
		name    string
		token   string
		errCode int
		err     error
	}{
		{"root", root.SecretID, 0, nil},
		{"no_permission_token", deniedToken.SecretID, 403, structs.ErrPermissionDenied},
		{"invalid token", uuid.Generate(), 403, structs.ErrPermissionDenied},
		{"unauthenticated", "", 403, structs.ErrPermissionDenied},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			handler, err := s.StreamingRpcHandler("Operator.SnapshotSave")
			require.NoError(t, err)

			p1, p2 := net.Pipe()
			defer p1.Close()
			defer p2.Close()

			// start handler
			go handler(p2)

			var req structs.SnapshotSaveRequest
			var resp structs.SnapshotSaveResponse

			req.Region = "global"
			req.AuthToken = c.token

			// send request
			encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
			err = encoder.Encode(&req)
			require.NoError(t, err)

			decoder := codec.NewDecoder(p1, structs.MsgpackHandle)
			err = decoder.Decode(&resp)
			require.NoError(t, err)

			// streaming errors appear as a response rather than a returned error
			if c.err != nil {
				require.Equal(t, c.err.Error(), resp.ErrorMsg)
				require.Equal(t, c.errCode, resp.ErrorCode)
				return

			}

			require.NotZero(t, resp.Index)
			require.NotEmpty(t, resp.SnapshotChecksum)
			require.Contains(t, resp.SnapshotChecksum, "sha-256=")

			io.Copy(io.Discard, p1)
		})
	}
}

func TestOperator_SnapshotRestore(t *testing.T) {
	ci.Parallel(t)

	targets := []string{"leader", "non_leader", "remote_region"}

	for _, c := range targets {
		t.Run(c, func(t *testing.T) {
			snap, job := generateSnapshot(t)

			checkFn := func(t *testing.T, s *Server) {
				found, err := s.State().JobByID(nil, job.Namespace, job.ID)
				require.NoError(t, err)
				require.Equal(t, job.ID, found.ID)
			}

			var req structs.SnapshotRestoreRequest
			req.Region = "global"
			testRestoreSnapshot(t, &req, snap, c, checkFn)
		})
	}
}

func generateSnapshot(t *testing.T) (*snapshot.Snapshot, *structs.Job) {
	dir := t.TempDir()

	s, cleanup := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 1
		c.DevMode = false
		c.DataDir = path.Join(dir, "server1")
	})
	defer cleanup()

	job := mock.Job()
	jobReq := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var jobResp structs.JobRegisterResponse
	codec := rpcClient(t, s)
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", jobReq, &jobResp)
	require.NoError(t, err)

	err = s.State().UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	require.NoError(t, err)

	snapshot, err := snapshot.New(s.logger, s.raft)
	require.NoError(t, err)

	t.Cleanup(func() { snapshot.Close() })

	return snapshot, job
}

func testRestoreSnapshot(t *testing.T, req *structs.SnapshotRestoreRequest, snapshot io.Reader, target string,
	assertionFn func(t *testing.T, server *Server)) {

	////// Nomad clusters topology - not specific to test
	dir := t.TempDir()

	server1, cleanupLS := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DataDir = path.Join(dir, "server1")

		// increase times outs to account for I/O operations that
		// snapshot restore performs - some of which require sync calls
		c.RaftConfig.LeaderLeaseTimeout = 1 * time.Second
		c.RaftConfig.HeartbeatTimeout = 1 * time.Second
		c.RaftConfig.ElectionTimeout = 1 * time.Second
		c.RaftTimeout = 5 * time.Second
	})
	defer cleanupLS()

	server2, cleanupRS := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DataDir = path.Join(dir, "server2")

		// increase times outs to account for I/O operations that
		// snapshot restore performs - some of which require sync calls
		c.RaftConfig.LeaderLeaseTimeout = 1 * time.Second
		c.RaftConfig.HeartbeatTimeout = 1 * time.Second
		c.RaftConfig.ElectionTimeout = 1 * time.Second
		c.RaftTimeout = 5 * time.Second
	})
	defer cleanupRS()

	remoteRegionServer, cleanupRRS := TestServer(t, func(c *Config) {
		c.Region = "two"
		c.DevMode = false
		c.DataDir = path.Join(dir, "remote_region_server")
	})
	defer cleanupRRS()

	TestJoin(t, server1, server2)
	TestJoin(t, server1, remoteRegionServer)
	testutil.WaitForLeader(t, server1.RPC)
	testutil.WaitForLeader(t, server2.RPC)
	testutil.WaitForLeader(t, remoteRegionServer.RPC)

	leader, nonLeader := server1, server2
	if server2.IsLeader() {
		leader, nonLeader = server2, server1
	}

	/////////  Actually run query now
	mapping := map[string]*Server{
		"leader":        leader,
		"non_leader":    nonLeader,
		"remote_region": remoteRegionServer,
	}

	server := mapping[target]
	require.NotNil(t, server, "target not found")

	handler, err := server.StreamingRpcHandler("Operator.SnapshotRestore")
	require.NoError(t, err)

	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	// start handler
	go handler(p2)

	var resp structs.SnapshotRestoreResponse

	// send request
	encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
	err = encoder.Encode(req)
	require.NoError(t, err)

	buf := make([]byte, 1024)
	for {
		n, err := snapshot.Read(buf)
		if n > 0 {
			require.NoError(t, encoder.Encode(&cstructs.StreamErrWrapper{Payload: buf[:n]}))
		}
		if err != nil {
			require.NoError(t, encoder.Encode(&cstructs.StreamErrWrapper{Error: &cstructs.RpcError{Message: err.Error()}}))
			break
		}
	}

	decoder := codec.NewDecoder(p1, structs.MsgpackHandle)
	err = decoder.Decode(&resp)
	require.NoError(t, err)
	require.Empty(t, resp.ErrorMsg)

	t.Run("checking leader state", func(t *testing.T) {
		assertionFn(t, leader)
	})

	t.Run("checking nonleader state", func(t *testing.T) {
		assertionFn(t, leader)
	})
}

func TestOperator_SnapshotRestore_ACL(t *testing.T) {
	ci.Parallel(t)

	dir := t.TempDir()

	/////////  Actually run query now
	cases := []struct {
		name    string
		errCode int
		err     error
	}{
		{"root", 0, nil},
		{"no_permission_token", 403, structs.ErrPermissionDenied},
		{"invalid token", 403, structs.ErrPermissionDenied},
		{"unauthenticated", 403, structs.ErrPermissionDenied},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			snapshot, _ := generateSnapshot(t)

			s, root, cleanupLS := TestACLServer(t, func(cfg *Config) {
				cfg.BootstrapExpect = 1
				cfg.DevMode = false
				cfg.DataDir = path.Join(dir, "server_"+c.name)
			})
			defer cleanupLS()

			testutil.WaitForLeader(t, s.RPC)

			deniedToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1001, "test-invalid", mock.NodePolicy(acl.PolicyWrite))

			token := ""
			switch c.name {
			case "root":
				token = root.SecretID
			case "no_permission_token":
				token = deniedToken.SecretID
			case "invalid token":
				token = uuid.Generate()
			case "unauthenticated":
				token = ""
			default:
				t.Fatalf("unexpected case: %v", c.name)
			}

			handler, err := s.StreamingRpcHandler("Operator.SnapshotRestore")
			require.NoError(t, err)

			p1, p2 := net.Pipe()
			defer p1.Close()
			defer p2.Close()

			// start handler
			go handler(p2)

			var req structs.SnapshotRestoreRequest
			var resp structs.SnapshotRestoreResponse

			req.Region = "global"
			req.AuthToken = token

			// send request
			encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
			err = encoder.Encode(&req)
			require.NoError(t, err)

			if c.err == nil {
				buf := make([]byte, 1024)
				for {
					n, err := snapshot.Read(buf)
					if n > 0 {
						require.NoError(t, encoder.Encode(&cstructs.StreamErrWrapper{Payload: buf[:n]}))
					}
					if err != nil {
						require.NoError(t, encoder.Encode(&cstructs.StreamErrWrapper{Error: &cstructs.RpcError{Message: err.Error()}}))
						break
					}
				}
			}

			decoder := codec.NewDecoder(p1, structs.MsgpackHandle)
			err = decoder.Decode(&resp)
			require.NoError(t, err)

			// streaming errors appear as a response rather than a returned error
			if c.err != nil {
				require.Equal(t, c.err.Error(), resp.ErrorMsg)
				require.Equal(t, c.errCode, resp.ErrorCode)
				return

			}

			require.NotZero(t, resp.Index)

			io.Copy(io.Discard, p1)
		})
	}
}
