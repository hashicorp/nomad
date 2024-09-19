// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"path"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestAuthenticate_mTLS(t *testing.T) {
	ci.Parallel(t)

	// Set up a cluster with mTLS and ACLs

	dir := t.TempDir()

	tlsCfg := &config.TLSConfig{
		EnableHTTP:           true,
		EnableRPC:            true,
		VerifyServerHostname: true,
		CAFile:               "../helper/tlsutil/testdata/nomad-agent-ca.pem",
		CertFile:             "../helper/tlsutil/testdata/regionFoo-server-nomad.pem",
		KeyFile:              "../helper/tlsutil/testdata/regionFoo-server-nomad-key.pem",
	}
	clientTLSCfg := tlsCfg.Copy()
	clientTLSCfg.CertFile = "../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
	clientTLSCfg.KeyFile = "../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"

	setCfg := func(name string, bootstrapExpect int) func(*Config) {
		return func(c *Config) {
			c.Region = "regionFoo"
			c.AuthoritativeRegion = "regionFoo"
			c.ACLEnabled = true
			c.BootstrapExpect = bootstrapExpect
			c.NumSchedulers = 0
			c.DevMode = false
			c.DataDir = path.Join(dir, name)
			c.TLSConfig = tlsCfg
		}
	}

	leader, cleanupLeader := TestServer(t, setCfg("node1", 1))
	defer cleanupLeader()
	testutil.WaitForLeader(t, leader.RPC)

	follower, cleanupFollower := TestServer(t, setCfg("node2", 0))
	defer cleanupFollower()

	TestJoin(t, leader, follower)
	testutil.WaitForLeader(t, leader.RPC)

	testutil.Wait(t, func() (bool, error) {
		cs, err := follower.encrypter.activeCipherSet()
		return cs != nil, err
	})

	rootToken := uuid.Generate()
	var bootstrapResp *structs.ACLTokenUpsertResponse

	codec := rpcClientWithTLS(t, follower, tlsCfg)
	must.NoError(t, msgpackrpc.CallWithCodec(codec,
		"ACL.Bootstrap", &structs.ACLTokenBootstrapRequest{
			BootstrapSecret: rootToken,
			WriteRequest:    structs.WriteRequest{Region: "regionFoo"},
		}, &bootstrapResp))
	must.NotNil(t, bootstrapResp)
	must.Len(t, 1, bootstrapResp.Tokens)
	rootAccessor := bootstrapResp.Tokens[0].AccessorID

	// create some ACL tokens directly into raft so we can bypass RPC validation
	// around expiration times

	token1 := mock.ACLToken()
	token2 := mock.ACLToken()
	expireTime := time.Now().Add(time.Second * -10)
	token2.ExpirationTime = &expireTime

	_, _, err := leader.raftApply(structs.ACLTokenUpsertRequestType,
		&structs.ACLTokenUpsertRequest{Tokens: []*structs.ACLToken{token1, token2}})
	must.NoError(t, err)

	// create a node so we can test client RPCs

	node := mock.Node()
	nodeRegisterReq := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "regionFoo"},
	}
	var nodeRegisterResp structs.NodeUpdateResponse

	must.NoError(t, msgpackrpc.CallWithCodec(codec,
		"Node.Register", nodeRegisterReq, &nodeRegisterResp))
	must.NotNil(t, bootstrapResp)

	// create some allocations so we can test WorkloadIdentity claims. we'll
	// create directly into raft so we can bypass RPC validation and the whole
	// eval, plan, etc. workflow.
	job := mock.Job()

	_, _, err = leader.raftApply(structs.JobRegisterRequestType,
		&structs.JobRegisterRequest{Job: job})
	must.NoError(t, err)

	alloc1 := mock.Alloc()
	alloc1.NodeID = node.ID
	alloc1.ClientStatus = structs.AllocClientStatusFailed
	alloc1.Job = job
	alloc1.JobID = job.ID

	alloc2 := mock.Alloc()
	alloc2.NodeID = node.ID
	alloc2.Job = job
	alloc2.JobID = job.ID
	alloc2.ClientStatus = structs.AllocClientStatusRunning

	wiHandle := &structs.WIHandle{
		WorkloadIdentifier: "web",
		WorkloadType:       structs.WorkloadTypeTask,
	}

	task1 := alloc1.LookupTask("web")
	claims1 := structs.NewIdentityClaimsBuilder(job, alloc1,
		wiHandle,
		task1.Identity).
		WithTask(task1).
		Build(time.Now())

	claims1Token, _, err := leader.encrypter.SignClaims(claims1)
	must.NoError(t, err, must.Sprint("could not sign claims"))

	task2 := alloc2.LookupTask("web")
	claims2 := structs.NewIdentityClaimsBuilder(job, alloc2,
		wiHandle,
		task2.Identity).
		WithTask(task1).
		Build(time.Now())

	claims2Token, _, err := leader.encrypter.SignClaims(claims2)
	must.NoError(t, err, must.Sprint("could not sign claims"))

	planReq := &structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Alloc: []*structs.Allocation{alloc1, alloc2},
			Job:   job,
		},
	}
	_, _, err = leader.raftApply(structs.ApplyPlanResultsRequestType, planReq)
	must.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		store := follower.fsm.State()
		alloc, err := store.AllocByID(nil, alloc1.ID)
		return alloc != nil, err
	}, func(err error) {
		t.Fatalf("alloc was not replicated via raft: %v", err) // should never happen
	})

	testCases := []struct {
		name           string
		tlsCfg         *config.TLSConfig
		stale          bool
		testToken      string
		expectAccessor string
		expectClientID string
		expectAllocID  string
		expectTLSName  string
		expectIP       string
		expectErr      string
		expectIDKey    string
		sendFromPeer   *Server
	}{
		{
			name:           "root token",
			tlsCfg:         clientTLSCfg, // TODO: this is a mixed use cert
			testToken:      rootToken,
			expectAccessor: rootAccessor,
			expectIDKey:    fmt.Sprintf("token:%s", rootAccessor),
		},
		{
			name:           "from peer to leader without token", // ex. Eval.Dequeue
			tlsCfg:         tlsCfg,
			expectTLSName:  "server.regionFoo.nomad",
			expectAccessor: "anonymous",
			expectIP:       follower.GetConfig().RPCAddr.IP.String(),
			sendFromPeer:   follower,
			expectIDKey:    "token:anonymous",
		},
		{
			// note: this test is somewhat bogus because under test all the
			// servers share the same IP address with the RPC client
			name:           "anonymous forwarded from peer to leader",
			tlsCfg:         tlsCfg,
			expectAccessor: "anonymous",
			expectTLSName:  "server.regionFoo.nomad",
			expectIP:       "127.0.0.1",
			expectIDKey:    "token:anonymous",
		},
		{
			name:          "invalid token",
			tlsCfg:        clientTLSCfg,
			testToken:     uuid.Generate(),
			expectTLSName: "server.regionFoo.nomad",
			expectIP:      follower.GetConfig().RPCAddr.IP.String(),
			expectIDKey:   "server.regionFoo.nomad:127.0.0.1",
			expectErr:     "rpc error: Permission denied",
		},
		{
			name:           "from peer to leader with leader ACL", // ex. core job GC
			tlsCfg:         tlsCfg,
			testToken:      leader.getLeaderAcl(),
			expectTLSName:  "server.regionFoo.nomad",
			expectAccessor: "leader",
			expectIP:       follower.GetConfig().RPCAddr.IP.String(),
			sendFromPeer:   follower,
			expectIDKey:    "token:leader",
		},
		{
			name:           "from client", // ex. Node.GetAllocs
			tlsCfg:         clientTLSCfg,
			testToken:      node.SecretID,
			expectClientID: node.ID,
			expectIDKey:    fmt.Sprintf("client:%s", node.ID),
		},
		{
			name:           "from client missing secret", // ex. Node.Register
			tlsCfg:         clientTLSCfg,
			expectAccessor: "anonymous",
			expectTLSName:  "server.regionFoo.nomad",
			expectIP:       follower.GetConfig().RPCAddr.IP.String(),
		},
		{
			name:      "from failed workload", // ex. Variables.List
			tlsCfg:    clientTLSCfg,
			testToken: claims1Token,
			expectErr: "rpc error: allocation is terminal",
		},
		{
			name:          "from running workload", // ex. Variables.List
			tlsCfg:        clientTLSCfg,
			testToken:     claims2Token,
			expectAllocID: alloc2.ID,
			expectIDKey:   fmt.Sprintf("alloc:%s", alloc2.ID),
		},
		{
			name:           "valid user token",
			tlsCfg:         clientTLSCfg,
			testToken:      token1.SecretID,
			expectAccessor: token1.AccessorID,
			expectIDKey:    fmt.Sprintf("token:%s", token1.AccessorID),
		},
		{
			name:      "expired user token",
			tlsCfg:    clientTLSCfg,
			testToken: token2.SecretID,
			expectErr: "rpc error: ACL token expired",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			req := &structs.GenericRequest{
				QueryOptions: structs.QueryOptions{
					Region:     "regionFoo",
					AllowStale: tc.stale,
					AuthToken:  tc.testToken,
				},
			}
			var resp structs.ACLWhoAmIResponse
			var err error

			if tc.sendFromPeer != nil {
				aclEndpoint := NewACLEndpoint(tc.sendFromPeer, nil)
				err = aclEndpoint.WhoAmI(req, &resp)
			} else {
				err = msgpackrpc.CallWithCodec(codec, "ACL.WhoAmI", req, &resp)
			}

			if tc.expectErr != "" {
				must.EqError(t, err, tc.expectErr)
				return
			}

			must.NoError(t, err)
			must.NotNil(t, resp)
			must.NotNil(t, resp.Identity)

			if tc.expectIDKey != "" {
				must.Eq(t, tc.expectIDKey, resp.Identity.String(),
					must.Sprintf("expected identity key for metrics to match"))
			}

			if tc.expectAccessor != "" {
				must.NotNil(t, resp.Identity.ACLToken, must.Sprint("expected ACL token"))
				test.Eq(t, tc.expectAccessor, resp.Identity.ACLToken.AccessorID,
					test.Sprint("expected ACL token accessor ID"))
			}

			test.Eq(t, tc.expectClientID, resp.Identity.ClientID,
				test.Sprint("expected client ID"))

			if tc.expectAllocID != "" {
				must.NotNil(t, resp.Identity.Claims, must.Sprint("expected claims"))
				test.Eq(t, tc.expectAllocID, resp.Identity.Claims.AllocationID,
					test.Sprint("expected workload identity"))
			}

			test.Eq(t, tc.expectTLSName, resp.Identity.TLSName, test.Sprint("expected TLS name"))

			if tc.expectIP == "" {
				test.Nil(t, resp.Identity.RemoteIP, test.Sprint("expected no remote IP"))
			} else {
				test.Eq(t, tc.expectIP, resp.Identity.RemoteIP.String())
			}

		})
	}
}
