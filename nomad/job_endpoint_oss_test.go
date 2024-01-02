// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"testing"

	"github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

// TestJobEndpoint_Register_Connect_AllowUnauthenticatedFalse asserts that a job
// submission fails allow_unauthenticated is false, and either an invalid or no
// operator Consul token is provided.
func TestJobEndpoint_Register_Connect_AllowUnauthenticatedFalse_oss(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
		c.ConsulConfig.AllowUnauthenticated = pointer.Of(false)
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	newJob := func(namespace string) *structs.Job {
		// Create the register request
		job := mock.Job()
		job.TaskGroups[0].Networks[0].Mode = "bridge"
		job.TaskGroups[0].Services = []*structs.Service{
			{
				Name:      "service1", // matches consul.ExamplePolicyID1
				PortLabel: "8080",
				Connect: &structs.ConsulConnect{
					SidecarService: &structs.ConsulSidecarService{},
				},
			},
		}
		// For this test we only care about authorizing the connect service
		job.TaskGroups[0].Tasks[0].Services = nil

		// If testing with a Consul namespace, set it on the group
		if namespace != "" {
			job.TaskGroups[0].Consul = &structs.Consul{
				Namespace: namespace,
			}
		}
		return job
	}

	newRequest := func(job *structs.Job) *structs.JobRegisterRequest {
		return &structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: job.Namespace,
			},
		}
	}

	noTokenOnJob := func(t *testing.T, job *structs.Job) {
		fsmState := s1.State()
		ws := memdb.NewWatchSet()
		storedJob, err := fsmState.JobByID(ws, job.Namespace, job.ID)
		require.NoError(t, err)
		require.NotNil(t, storedJob)
		require.Empty(t, storedJob.ConsulToken)
	}

	// Non-sense Consul ACL tokens that should be rejected
	missingToken := ""
	fakeToken := uuid.Generate()

	// Consul ACL tokens in no Consul namespace
	ossTokenNoPolicyNoNS := consul.ExampleOperatorTokenID3
	ossTokenNoNS := consul.ExampleOperatorTokenID1

	// Consul ACL tokens in "default" Consul namespace
	entTokenNoPolicyDefaultNS := consul.ExampleOperatorTokenID20
	entTokenDefaultNS := consul.ExampleOperatorTokenID21

	// Consul ACL tokens in "banana" Consul namespace
	entTokenNoPolicyBananaNS := consul.ExampleOperatorTokenID10
	entTokenBananaNS := consul.ExampleOperatorTokenID11

	t.Run("group consul namespace unset", func(t *testing.T) {
		// When the group namespace is unset (which is always the case with
		// Nomad OSS), Consul tokens with no namespace or are in the "default"
		// namespace should be accepted (assuming a sufficient service policy).
		namespace := ""

		t.Run("no token provided", func(t *testing.T) {
			request := newRequest(newJob(namespace))
			request.Job.ConsulToken = missingToken
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, "job-submitter consul token denied: missing consul token")
		})

		t.Run("unknown token provided", func(t *testing.T) {
			request := newRequest(newJob(namespace))
			request.Job.ConsulToken = fakeToken
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, "job-submitter consul token denied: unable to read consul token: no such token")
		})

		t.Run("unauthorized oss token provided", func(t *testing.T) {
			request := newRequest(newJob(namespace))
			request.Job.ConsulToken = ossTokenNoPolicyNoNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, `job-submitter consul token denied: insufficient Consul ACL permissions to write service "service1"`)
		})

		t.Run("authorized oss token provided", func(t *testing.T) {
			job := newJob(namespace)
			request := newRequest(job)
			request.Job.ConsulToken = ossTokenNoNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.NoError(t, err)
			noTokenOnJob(t, job)
		})

		t.Run("unauthorized token in default namespace", func(t *testing.T) {
			job := newJob(namespace)
			request := newRequest(job)
			request.Job.ConsulToken = entTokenNoPolicyDefaultNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, `job-submitter consul token denied: insufficient Consul ACL permissions to write service "service1"`)
		})

		t.Run("authorized token in default namespace", func(t *testing.T) {
			job := newJob(namespace)
			request := newRequest(job)
			request.Job.ConsulToken = entTokenDefaultNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.NoError(t, err)
			noTokenOnJob(t, job)
		})

		t.Run("unauthorized token in banana namespace", func(t *testing.T) {
			job := newJob(namespace)
			request := newRequest(job)
			request.Job.ConsulToken = entTokenNoPolicyBananaNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, `job-submitter consul token denied: consul ACL token requires using namespace "banana"`)
		})

		t.Run("authorized token in banana namespace", func(t *testing.T) {
			job := newJob(namespace)
			request := newRequest(job)
			request.Job.ConsulToken = entTokenBananaNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, `job-submitter consul token denied: consul ACL token requires using namespace "banana"`)
		})
	})

	t.Run("group consul namespace banana", func(t *testing.T) {
		// Nomad OSS does not respect setting the consul namespace field on the group,
		// and for backwards compatibility accepts tokens in the "default" namespace
		// for groups with no namespace set. The net result is setting the group namespace
		// to something like "banana" and using a token in "default" namespace will
		// be accepted in Nomad OSS (assuming sufficient service write policy).
		//
		// Using a Consul token in the non-"default" namespace will always fail in
		// Nomad OSS, again because the group namespace is ignored.
		namespace := "banana"

		t.Run("no token provided", func(t *testing.T) {
			request := newRequest(newJob(namespace))
			request.Job.ConsulToken = missingToken
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, "job-submitter consul token denied: missing consul token")
		})

		t.Run("unknown token provided", func(t *testing.T) {
			request := newRequest(newJob(namespace))
			request.Job.ConsulToken = fakeToken
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, "job-submitter consul token denied: unable to read consul token: no such token")
		})

		t.Run("unauthorized oss token provided", func(t *testing.T) {
			request := newRequest(newJob(namespace))
			request.Job.ConsulToken = ossTokenNoPolicyNoNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, `job-submitter consul token denied: insufficient Consul ACL permissions to write service "service1"`)
		})

		t.Run("authorized oss token provided", func(t *testing.T) {
			job := newJob(namespace)
			request := newRequest(job)
			request.Job.ConsulToken = ossTokenNoNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.NoError(t, err)
			noTokenOnJob(t, job)
		})

		t.Run("unauthorized token in default namespace", func(t *testing.T) {
			job := newJob(namespace)
			request := newRequest(job)
			request.Job.ConsulToken = entTokenNoPolicyDefaultNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, `job-submitter consul token denied: insufficient Consul ACL permissions to write service "service1"`)
		})

		t.Run("authorized token in default namespace", func(t *testing.T) {
			job := newJob(namespace)
			request := newRequest(job)
			request.Job.ConsulToken = entTokenDefaultNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.NoError(t, err)
			noTokenOnJob(t, job)
		})

		// Consul token in custom namespace will always fail in nomad oss

		t.Run("unauthorized token in banana namespace", func(t *testing.T) {
			job := newJob(namespace)
			request := newRequest(job)
			request.Job.ConsulToken = entTokenNoPolicyBananaNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, `job-submitter consul token denied: consul ACL token requires using namespace "banana"`)
		})

		t.Run("authorized token in banana namespace", func(t *testing.T) {
			job := newJob(namespace)
			request := newRequest(job)
			request.Job.ConsulToken = entTokenBananaNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, `job-submitter consul token denied: consul ACL token requires using namespace "banana"`)
		})
	})

	t.Run("group consul namespace default", func(t *testing.T) {
		// Nomad OSS ignores the group consul namespace, and setting it as default
		// should effectively be the same as leaving it unset.
		namespace := "default"

		t.Run("no token provided", func(t *testing.T) {
			request := newRequest(newJob(namespace))
			request.Job.ConsulToken = missingToken
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, "job-submitter consul token denied: missing consul token")
		})

		t.Run("unknown token provided", func(t *testing.T) {
			request := newRequest(newJob(namespace))
			request.Job.ConsulToken = fakeToken
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, "job-submitter consul token denied: unable to read consul token: no such token")
		})

		t.Run("unauthorized oss token provided", func(t *testing.T) {
			request := newRequest(newJob(namespace))
			request.Job.ConsulToken = ossTokenNoPolicyNoNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, `job-submitter consul token denied: insufficient Consul ACL permissions to write service "service1"`)
		})

		t.Run("authorized oss token provided", func(t *testing.T) {
			job := newJob(namespace)
			request := newRequest(job)
			request.Job.ConsulToken = ossTokenNoNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.NoError(t, err)
			noTokenOnJob(t, job)
		})

		t.Run("unauthorized token in default namespace", func(t *testing.T) {
			job := newJob(namespace)
			request := newRequest(job)
			request.Job.ConsulToken = entTokenNoPolicyDefaultNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, `job-submitter consul token denied: insufficient Consul ACL permissions to write service "service1"`)
		})

		t.Run("authorized token in default namespace", func(t *testing.T) {
			job := newJob(namespace)
			request := newRequest(job)
			request.Job.ConsulToken = entTokenDefaultNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.NoError(t, err)
			noTokenOnJob(t, job)
		})

		t.Run("unauthorized token in banana namespace", func(t *testing.T) {
			job := newJob(namespace)
			request := newRequest(job)
			request.Job.ConsulToken = entTokenNoPolicyBananaNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, `job-submitter consul token denied: consul ACL token requires using namespace "banana"`)
		})

		t.Run("authorized token in banana namespace", func(t *testing.T) {
			job := newJob(namespace)
			request := newRequest(job)
			request.Job.ConsulToken = entTokenBananaNS
			var response structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", request, &response)
			require.EqualError(t, err, `job-submitter consul token denied: consul ACL token requires using namespace "banana"`)
		})
	})
}

func TestJobEndpoint_Register_NodePool(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Create test namespace.
	ns := mock.Namespace()
	nsReq := &structs.NamespaceUpsertRequest{
		Namespaces:   []*structs.Namespace{ns},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nsResp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "Namespace.UpsertNamespaces", nsReq, &nsResp)
	must.NoError(t, err)

	// Create test node pool.
	pool := mock.NodePool()
	poolReq := &structs.NodePoolUpsertRequest{
		NodePools:    []*structs.NodePool{pool},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var poolResp structs.GenericResponse
	err = msgpackrpc.CallWithCodec(codec, "NodePool.UpsertNodePools", poolReq, &poolResp)
	must.NoError(t, err)

	testCases := []struct {
		name         string
		namespace    string
		nodePool     string
		expectedPool string
		expectedErr  string
	}{
		{
			name:         "job in default namespace uses default node pool",
			namespace:    structs.DefaultNamespace,
			nodePool:     "",
			expectedPool: structs.NodePoolDefault,
		},
		{
			name:         "job without node pool uses default node pool",
			namespace:    ns.Name,
			nodePool:     "",
			expectedPool: structs.NodePoolDefault,
		},
		{
			name:         "job can set node pool",
			namespace:    ns.Name,
			nodePool:     pool.Name,
			expectedPool: pool.Name,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			job := mock.Job()
			job.Namespace = tc.namespace
			job.NodePool = tc.nodePool

			req := &structs.JobRegisterRequest{
				Job: job,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					Namespace: job.Namespace,
				},
			}
			var resp structs.JobRegisterResponse
			err = msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)

			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
			} else {
				must.NoError(t, err)

				got, err := s.State().JobByID(nil, job.Namespace, job.ID)
				must.NoError(t, err)
				must.Eq(t, tc.expectedPool, got.NodePool)
			}
		})
	}
}
