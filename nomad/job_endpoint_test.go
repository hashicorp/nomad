// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
	"github.com/kr/pretty"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobEndpoint_Register(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.CreateIndex != resp.JobModifyIndex {
		t.Fatalf("index mis-match")
	}
	serviceName := out.TaskGroups[0].Tasks[0].Services[0].Name
	expectedServiceName := "web-frontend"
	if serviceName != expectedServiceName {
		t.Fatalf("Expected Service Name: %s, Actual: %s", expectedServiceName, serviceName)
	}

	// Lookup the evaluation
	eval, err := state.EvalByID(ws, resp.EvalID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eval == nil {
		t.Fatalf("expected eval")
	}
	if eval.CreateIndex != resp.EvalCreateIndex {
		t.Fatalf("index mis-match")
	}

	if eval.Priority != job.Priority {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Type != job.Type {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.TriggeredBy != structs.EvalTriggerJobRegister {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobID != job.ID {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobModifyIndex != resp.JobModifyIndex {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Status != structs.EvalStatusPending {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.CreateTime == 0 {
		t.Fatalf("eval CreateTime is unset: %#v", eval)
	}
	if eval.ModifyTime == 0 {
		t.Fatalf("eval ModifyTime is unset: %#v", eval)
	}
}

// TestJobEndpoint_Register_NonOverlapping asserts that ClientStatus must be
// terminal, not just DesiredStatus, for the resources used by a job to be
// considered free for subsequent placements to use.
//
// See: https://github.com/hashicorp/nomad/issues/10440
func TestJobEndpoint_Register_NonOverlapping(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
	})
	defer cleanupS1()
	state := s1.fsm.State()

	// Create a mock node with easy to check resources
	node := mock.Node()
	node.NodeResources.Processors = structs.NodeProcessorResources{
		Topology: &numalib.Topology{
			Distances: numalib.SLIT{[]numalib.Cost{10}},
			Cores: []numalib.Core{{
				ID:        0,
				Grade:     numalib.Performance,
				BaseSpeed: 700,
			}},
		},
	}
	node.NodeResources.Processors.Topology.SetNodes(idset.From[hw.NodeID]([]hw.NodeID{0}))
	node.NodeResources.Compatibility()
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1, node))

	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.TaskGroups[0].Count = 1
	req := &structs.JobRegisterRequest{
		Job: job.Copy(),
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
			AuthToken: node.SecretID,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	must.NonZero(t, resp.Index)

	// Assert placement
	jobReq := &structs.JobSpecificRequest{
		JobID: job.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}
	var alloc *structs.AllocListStub
	testutil.Wait(t, func() (bool, error) {
		resp := structs.JobAllocationsResponse{}
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Allocations", jobReq, &resp))
		if n := len(resp.Allocations); n != 1 {
			return false, fmt.Errorf("expected 1 allocation but found %d:\n%v", n, resp.Allocations)
		}

		alloc = resp.Allocations[0]
		return true, nil
	})
	must.Eq(t, alloc.NodeID, node.ID)
	must.Eq(t, alloc.DesiredStatus, structs.AllocDesiredStatusRun)
	must.Eq(t, alloc.ClientStatus, structs.AllocClientStatusPending)

	// Stop
	stopReq := &structs.JobDeregisterRequest{
		JobID:        job.ID,
		Purge:        false,
		WriteRequest: req.WriteRequest,
	}
	var stopResp structs.JobDeregisterResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Deregister", stopReq, &stopResp))

	// Wait until the Stop is complete
	testutil.Wait(t, func() (bool, error) {
		eval, err := state.EvalByID(nil, stopResp.EvalID)
		must.NoError(t, err)
		if eval == nil {
			return false, fmt.Errorf("eval not applied: %s", resp.EvalID)
		}
		return eval.Status == structs.EvalStatusComplete, fmt.Errorf("expected eval to be complete but found: %s", eval.Status)
	})

	// Assert new register blocked
	req.Job = job.Copy()
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	must.NonZero(t, resp.Index)

	blockedEval := ""
	testutil.Wait(t, func() (bool, error) {
		// Assert no new allocs
		allocResp := structs.JobAllocationsResponse{}
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Allocations", jobReq, &allocResp))
		if n := len(allocResp.Allocations); n != 1 {
			return false, fmt.Errorf("expected 1 allocation but found %d:\n%v", n, allocResp.Allocations)
		}

		if alloc.ID != allocResp.Allocations[0].ID {
			return false, fmt.Errorf("unexpected change in alloc: %#v", *allocResp.Allocations[0])
		}

		eval, err := state.EvalByID(nil, resp.EvalID)
		must.NoError(t, err)
		if eval == nil {
			return false, fmt.Errorf("eval not applied: %s", resp.EvalID)
		}
		if eval.Status != structs.EvalStatusComplete {
			return false, fmt.Errorf("expected eval to be complete but found: %s", eval.Status)
		}
		if eval.BlockedEval == "" {
			return false, fmt.Errorf("expected a blocked eval to be created")
		}
		blockedEval = eval.BlockedEval
		return true, nil
	})

	// Set ClientStatus=complete like a client would
	stoppedAlloc := &structs.Allocation{
		ID:     alloc.ID,
		NodeID: alloc.NodeID,
		TaskStates: map[string]*structs.TaskState{
			"web": &structs.TaskState{
				State: structs.TaskStateDead,
			},
		},
		ClientStatus:     structs.AllocClientStatusComplete,
		DeploymentStatus: nil, // should not have an impact
		NetworkStatus:    nil, // should not have an impact
	}
	upReq := &structs.AllocUpdateRequest{
		Alloc:        []*structs.Allocation{stoppedAlloc},
		WriteRequest: req.WriteRequest,
	}
	var upResp structs.GenericResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.UpdateAlloc", upReq, &upResp))

	// Assert newer register's eval unblocked
	testutil.Wait(t, func() (bool, error) {
		eval, err := state.EvalByID(nil, blockedEval)
		must.NoError(t, err)
		must.NotNil(t, eval)
		if eval.Status != structs.EvalStatusComplete {
			return false, fmt.Errorf("expected blocked eval to be complete but found: %s", eval.Status)
		}
		return true, nil
	})

	// Assert new alloc placed
	testutil.Wait(t, func() (bool, error) {
		// Assert no new allocs
		allocResp := structs.JobAllocationsResponse{}
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Allocations", jobReq, &allocResp))
		if n := len(allocResp.Allocations); n != 2 {
			return false, fmt.Errorf("expected 2 allocs but found %d:\n%v", n, allocResp.Allocations)
		}

		for _, a := range allocResp.Allocations {
			if a.ID == alloc.ID {
				if cs := a.ClientStatus; cs != structs.AllocClientStatusComplete {
					return false, fmt.Errorf("expected old alloc to be complete but found: %s", cs)
				}
			} else {
				if cs := a.ClientStatus; cs != structs.AllocClientStatusPending {
					return false, fmt.Errorf("expected new alloc to be pending but found: %s", cs)
				}
			}
		}

		return true, nil
	})
}

func TestJobEndpoint_Register_PreserveCounts(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.TaskGroups[0].Name = "group1"
	job.TaskGroups[0].Count = 10
	job.Canonicalize()

	// Register the job
	require.NoError(msgpackrpc.CallWithCodec(codec, "Job.Register", &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}, &structs.JobRegisterResponse{}))

	// Check the job in the FSM state
	state := s1.fsm.State()
	out, err := state.JobByID(nil, job.Namespace, job.ID)
	require.NoError(err)
	require.NotNil(out)
	require.Equal(10, out.TaskGroups[0].Count)

	// New version:
	// new "group2" with 2 instances
	// "group1" goes from 10 -> 0 in the spec
	job = job.Copy()
	job.TaskGroups[0].Count = 0 // 10 -> 0 in the job spec
	job.TaskGroups = append(job.TaskGroups, job.TaskGroups[0].Copy())
	job.TaskGroups[1].Name = "group2"
	job.TaskGroups[1].Count = 2

	// Perform the update
	require.NoError(msgpackrpc.CallWithCodec(codec, "Job.Register", &structs.JobRegisterRequest{
		Job:            job,
		PreserveCounts: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}, &structs.JobRegisterResponse{}))

	// Check the job in the FSM state
	out, err = state.JobByID(nil, job.Namespace, job.ID)
	require.NoError(err)
	require.NotNil(out)
	require.Equal(10, out.TaskGroups[0].Count) // should not change
	require.Equal(2, out.TaskGroups[1].Count)  // should be as in job spec
}

func TestJobEndpoint_Register_EvalPriority(t *testing.T) {
	ci.Parallel(t)
	requireAssert := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) { c.NumSchedulers = 0 })
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.TaskGroups[0].Name = "group1"
	job.Canonicalize()

	// Register the job.
	requireAssert.NoError(msgpackrpc.CallWithCodec(codec, "Job.Register", &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
		EvalPriority: 99,
	}, &structs.JobRegisterResponse{}))

	// Grab the eval from the state, and check its priority is as expected.
	state := s1.fsm.State()
	out, err := state.EvalsByJob(nil, job.Namespace, job.ID)
	requireAssert.NoError(err)
	requireAssert.Len(out, 1)
	requireAssert.Equal(99, out[0].Priority)
}

func TestJobEndpoint_Register_Connect(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Services = nil
	job.TaskGroups[0].Networks = structs.Networks{{
		Mode: "bridge",
	}}
	job.TaskGroups[0].Services = []*structs.Service{{
		Name:      "backend",
		PortLabel: "8080",
		Connect: &structs.ConsulConnect{
			SidecarService: &structs.ConsulSidecarService{},
		}},
	}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	require.NoError(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	require.NotZero(resp.Index)

	// Check for the node in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	require.NoError(err)
	require.NotNil(out)
	require.Equal(resp.JobModifyIndex, out.CreateIndex)

	// Check that the sidecar task was injected
	require.Len(out.TaskGroups[0].Tasks, 2)
	sidecarTask := out.TaskGroups[0].Tasks[1]
	require.Equal("connect-proxy-backend", sidecarTask.Name)
	require.Equal("connect-proxy:backend", string(sidecarTask.Kind))
	require.Equal("connect-proxy-backend", out.TaskGroups[0].Networks[0].DynamicPorts[0].Label)

	// Check that round tripping the job doesn't change the sidecarTask
	out.Meta["test"] = "abc"
	req.Job = out
	require.NoError(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	require.NotZero(resp.Index)
	// Check for the new node in the FSM
	state = s1.fsm.State()
	ws = memdb.NewWatchSet()
	out, err = state.JobByID(ws, job.Namespace, job.ID)
	require.NoError(err)
	require.NotNil(out)
	require.Equal(resp.JobModifyIndex, out.CreateIndex)

	require.Len(out.TaskGroups[0].Tasks, 2)
	require.Exactly(sidecarTask, out.TaskGroups[0].Tasks[1])
}

func TestJobEndpoint_Register_ConnectIngressGateway_minimum(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// job contains the minimalist possible gateway service definition
	job := mock.ConnectIngressGatewayJob("host", false)

	// Create the register request
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	r.NoError(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	r.NotZero(resp.Index)

	// Check for the node in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	r.NoError(err)
	r.NotNil(out)
	r.Equal(resp.JobModifyIndex, out.CreateIndex)

	// Check that the gateway task got injected
	r.Len(out.TaskGroups[0].Tasks, 1)
	task := out.TaskGroups[0].Tasks[0]
	r.Equal("connect-ingress-my-ingress-service", task.Name)
	r.Equal("connect-ingress:my-ingress-service", string(task.Kind))
	r.Equal("docker", task.Driver)
	r.NotNil(task.Config)

	// Check the CE fields got set
	service := out.TaskGroups[0].Services[0]
	r.Equal(&structs.ConsulIngressConfigEntry{
		TLS: nil,
		Listeners: []*structs.ConsulIngressListener{{
			Port:     2000,
			Protocol: "tcp",
			Services: []*structs.ConsulIngressService{{
				Name: "service1",
			}},
		}},
	}, service.Connect.Gateway.Ingress)

	// Check that round-tripping does not inject a duplicate task
	out.Meta["test"] = "abc"
	req.Job = out
	r.NoError(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	r.NotZero(resp.Index)

	// Check for the new node in the fsm
	state = s1.fsm.State()
	ws = memdb.NewWatchSet()
	out, err = state.JobByID(ws, job.Namespace, job.ID)
	r.NoError(err)
	r.NotNil(out)
	r.Equal(resp.JobModifyIndex, out.CreateIndex)

	// Check we did not re-add the task that was added the first time
	r.Len(out.TaskGroups[0].Tasks, 1)
}

func TestJobEndpoint_Register_ConnectIngressGateway_full(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// reconfigure job to fill in all the possible fields
	job := mock.ConnectIngressGatewayJob("bridge", false)
	job.TaskGroups[0].Services[0].Connect = &structs.ConsulConnect{
		Gateway: &structs.ConsulGateway{
			Proxy: &structs.ConsulGatewayProxy{
				ConnectTimeout:                  pointer.Of(1 * time.Second),
				EnvoyGatewayBindTaggedAddresses: true,
				EnvoyGatewayBindAddresses: map[string]*structs.ConsulGatewayBindAddress{
					"service1": {
						Address: "10.0.0.1",
						Port:    2001,
					},
					"service2": {
						Address: "10.0.0.2",
						Port:    2002,
					},
				},
				EnvoyGatewayNoDefaultBind: true,
				Config: map[string]interface{}{
					"foo": 1,
					"bar": "baz",
				},
			},
			Ingress: &structs.ConsulIngressConfigEntry{
				TLS: &structs.ConsulGatewayTLSConfig{
					Enabled: true,
				},
				Listeners: []*structs.ConsulIngressListener{{
					Port:     3000,
					Protocol: "tcp",
					Services: []*structs.ConsulIngressService{{
						Name: "db",
					}},
				}, {
					Port:     3001,
					Protocol: "http",
					Services: []*structs.ConsulIngressService{{
						Name:  "website",
						Hosts: []string{"10.0.1.0", "10.0.1.0:3001"},
					}},
				}},
			},
		},
	}

	// Create the register request
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	r.NoError(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	r.NotZero(resp.Index)

	// Check for the node in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	r.NoError(err)
	r.NotNil(out)
	r.Equal(resp.JobModifyIndex, out.CreateIndex)

	// Check that the gateway task got injected
	r.Len(out.TaskGroups[0].Tasks, 1)
	task := out.TaskGroups[0].Tasks[0]
	r.Equal("connect-ingress-my-ingress-service", task.Name)
	r.Equal("connect-ingress:my-ingress-service", string(task.Kind))
	r.Equal("docker", task.Driver)
	r.NotNil(task.Config)

	// Check that the ingress service is all set
	service := out.TaskGroups[0].Services[0]
	r.Equal("my-ingress-service", service.Name)
	r.Equal(&structs.ConsulIngressConfigEntry{
		TLS: &structs.ConsulGatewayTLSConfig{
			Enabled: true,
		},
		Listeners: []*structs.ConsulIngressListener{{
			Port:     3000,
			Protocol: "tcp",
			Services: []*structs.ConsulIngressService{{
				Name: "db",
			}},
		}, {
			Port:     3001,
			Protocol: "http",
			Services: []*structs.ConsulIngressService{{
				Name:  "website",
				Hosts: []string{"10.0.1.0", "10.0.1.0:3001"},
			}},
		}},
	}, service.Connect.Gateway.Ingress)

	// Check that round-tripping does not inject a duplicate task
	out.Meta["test"] = "abc"
	req.Job = out
	r.NoError(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	r.NotZero(resp.Index)

	// Check for the new node in the fsm
	state = s1.fsm.State()
	ws = memdb.NewWatchSet()
	out, err = state.JobByID(ws, job.Namespace, job.ID)
	r.NoError(err)
	r.NotNil(out)
	r.Equal(resp.JobModifyIndex, out.CreateIndex)

	// Check we did not re-add the task that was added the first time
	r.Len(out.TaskGroups[0].Tasks, 1)
}

func TestJobEndpoint_Register_ConnectExposeCheck(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Setup the job we are going to register
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Services = nil
	job.TaskGroups[0].Networks = structs.Networks{{
		Mode: "bridge",
		DynamicPorts: []structs.Port{{
			Label: "hcPort",
			To:    -1,
		}, {
			Label: "v2Port",
			To:    -1,
		}},
	}}
	job.TaskGroups[0].Services = []*structs.Service{{
		Name:      "backend",
		PortLabel: "8080",
		Checks: []*structs.ServiceCheck{{
			Name:      "check1",
			Type:      "http",
			Protocol:  "http",
			Path:      "/health",
			Expose:    true,
			PortLabel: "hcPort",
			Interval:  1 * time.Second,
			Timeout:   1 * time.Second,
		}, {
			Name:     "check2",
			Type:     "script",
			Command:  "/bin/true",
			TaskName: "web",
			Interval: 1 * time.Second,
			Timeout:  1 * time.Second,
		}, {
			Name:      "check3",
			Type:      "grpc",
			Protocol:  "grpc",
			Path:      "/v2/health",
			Expose:    true,
			PortLabel: "v2Port",
			Interval:  1 * time.Second,
			Timeout:   1 * time.Second,
		}},
		Connect: &structs.ConsulConnect{
			SidecarService: &structs.ConsulSidecarService{}},
	}}

	// Create the register request
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	r.NoError(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	r.NotZero(resp.Index)

	// Check for the node in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	r.NoError(err)
	r.NotNil(out)
	r.Equal(resp.JobModifyIndex, out.CreateIndex)

	// Check that the new expose paths got created
	r.Len(out.TaskGroups[0].Services[0].Connect.SidecarService.Proxy.Expose.Paths, 2)
	httpPath := out.TaskGroups[0].Services[0].Connect.SidecarService.Proxy.Expose.Paths[0]
	r.Equal(structs.ConsulExposePath{
		Path:          "/health",
		Protocol:      "http",
		LocalPathPort: 8080,
		ListenerPort:  "hcPort",
	}, httpPath)
	grpcPath := out.TaskGroups[0].Services[0].Connect.SidecarService.Proxy.Expose.Paths[1]
	r.Equal(structs.ConsulExposePath{
		Path:          "/v2/health",
		Protocol:      "grpc",
		LocalPathPort: 8080,
		ListenerPort:  "v2Port",
	}, grpcPath)

	// make sure round tripping does not create duplicate expose paths
	out.Meta["test"] = "abc"
	req.Job = out
	r.NoError(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	r.NotZero(resp.Index)

	// Check for the new node in the FSM
	state = s1.fsm.State()
	ws = memdb.NewWatchSet()
	out, err = state.JobByID(ws, job.Namespace, job.ID)
	r.NoError(err)
	r.NotNil(out)
	r.Equal(resp.JobModifyIndex, out.CreateIndex)

	// make sure we are not re-adding what has already been added
	r.Len(out.TaskGroups[0].Services[0].Connect.SidecarService.Proxy.Expose.Paths, 2)
}

func TestJobEndpoint_Register_ConnectWithSidecarTask(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.TaskGroups[0].Networks = structs.Networks{
		{
			Mode: "bridge",
		},
	}
	job.TaskGroups[0].Tasks[0].Services = nil
	job.TaskGroups[0].Services = []*structs.Service{
		{
			Name:      "backend",
			PortLabel: "8080",
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{},
				SidecarTask: &structs.SidecarTask{
					Meta: map[string]string{
						"source": "test",
					},
					Resources: &structs.Resources{
						CPU: 500,
					},
					Config: map[string]interface{}{
						"labels": map[string]string{
							"foo": "bar",
						},
					},
				},
			},
		},
	}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	require.NoError(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	require.NotZero(resp.Index)

	// Check for the node in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	require.NoError(err)
	require.NotNil(out)
	require.Equal(resp.JobModifyIndex, out.CreateIndex)

	// Check that the sidecar task was injected
	require.Len(out.TaskGroups[0].Tasks, 2)
	sidecarTask := out.TaskGroups[0].Tasks[1]
	require.Equal("connect-proxy-backend", sidecarTask.Name)
	require.Equal("connect-proxy:backend", string(sidecarTask.Kind))
	require.Equal("connect-proxy-backend", out.TaskGroups[0].Networks[0].DynamicPorts[0].Label)

	// Check that the correct fields were overridden from the sidecar_task block
	require.Equal("test", sidecarTask.Meta["source"])
	require.Equal(500, sidecarTask.Resources.CPU)
	require.Equal(connectSidecarResources().MemoryMB, sidecarTask.Resources.MemoryMB)
	cfg := connectSidecarDriverConfig()
	cfg["labels"] = map[string]interface{}{
		"foo": "bar",
	}
	require.Equal(cfg, sidecarTask.Config)

	// Check that round tripping the job doesn't change the sidecarTask
	out.Meta["test"] = "abc"
	req.Job = out
	require.NoError(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	require.NotZero(resp.Index)
	// Check for the new node in the FSM
	state = s1.fsm.State()
	ws = memdb.NewWatchSet()
	out, err = state.JobByID(ws, job.Namespace, job.ID)
	require.NoError(err)
	require.NotNil(out)
	require.Equal(resp.JobModifyIndex, out.CreateIndex)

	require.Len(out.TaskGroups[0].Tasks, 2)
	require.Exactly(sidecarTask, out.TaskGroups[0].Tasks[1])

}

func TestJobEndpoint_Register_Connect_ValidatesWithoutSidecarTask(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.TaskGroups[0].Networks = structs.Networks{
		{
			Mode: "bridge",
		},
	}
	job.TaskGroups[0].Tasks[0].Services = nil
	job.TaskGroups[0].Services = []*structs.Service{
		{
			Name:      "backend",
			PortLabel: "8080",
			Connect: &structs.ConsulConnect{
				SidecarService: nil,
			},
			Checks: []*structs.ServiceCheck{{
				Name:     "exposed_no_sidecar",
				Type:     "http",
				Expose:   true,
				Path:     "/health",
				Interval: 10 * time.Second,
				Timeout:  2 * time.Second,
			}},
		},
	}

	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exposed_no_sidecar requires use of sidecar_proxy")
}

func TestJobEndpoint_Register_ACL(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	newVolumeJob := func(readonlyVolume bool) *structs.Job {
		j := mock.Job()
		tg := j.TaskGroups[0]
		tg.Volumes = map[string]*structs.VolumeRequest{
			"ca-certs": {
				Type:     structs.VolumeTypeHost,
				Source:   "prod-ca-certs",
				ReadOnly: readonlyVolume,
			},
			"csi": {
				Type:           structs.VolumeTypeCSI,
				Source:         "prod-db",
				AttachmentMode: structs.CSIVolumeAttachmentModeBlockDevice,
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeMultiWriter,
			},
		}

		tg.Tasks[0].VolumeMounts = []*structs.VolumeMount{
			{
				Volume:      "ca-certs",
				Destination: "/etc/ca-certificates",
				// Task readonly does not effect acls
				ReadOnly: true,
			},
		}

		return j
	}

	newCSIPluginJob := func() *structs.Job {
		j := mock.Job()
		t := j.TaskGroups[0].Tasks[0]
		t.CSIPluginConfig = &structs.TaskCSIPluginConfig{
			ID:   "foo",
			Type: "node",
		}
		return j
	}

	submitJobPolicy := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob, acl.NamespaceCapabilitySubmitJob})

	submitJobToken := mock.CreatePolicyAndToken(t, s1.State(), 1001, "test-submit-job", submitJobPolicy)

	volumesPolicyReadWrite := mock.HostVolumePolicy("prod-*", "", []string{acl.HostVolumeCapabilityMountReadWrite})

	volumesPolicyCSIMount := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityCSIMountVolume}) +
		mock.PluginPolicy("read")

	submitJobWithVolumesReadWriteToken := mock.CreatePolicyAndToken(t, s1.State(), 1002, "test-submit-volumes", submitJobPolicy+
		volumesPolicyReadWrite+
		volumesPolicyCSIMount)

	volumesPolicyReadOnly := mock.HostVolumePolicy("prod-*", "", []string{acl.HostVolumeCapabilityMountReadOnly})

	submitJobWithVolumesReadOnlyToken := mock.CreatePolicyAndToken(t, s1.State(), 1003, "test-submit-volumes-readonly", submitJobPolicy+
		volumesPolicyReadOnly+
		volumesPolicyCSIMount)

	pluginPolicy := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityCSIRegisterPlugin})
	pluginToken := mock.CreatePolicyAndToken(t, s1.State(), 1005, "test-csi-register-plugin", submitJobPolicy+pluginPolicy)

	cases := []struct {
		Name        string
		Job         *structs.Job
		Token       string
		ErrExpected bool
	}{
		{
			Name:        "without a token",
			Job:         mock.Job(),
			Token:       "",
			ErrExpected: true,
		},
		{
			Name:        "with a token",
			Job:         mock.Job(),
			Token:       root.SecretID,
			ErrExpected: false,
		},
		{
			Name:        "with a token that can submit a job, but not use a required volume",
			Job:         newVolumeJob(false),
			Token:       submitJobToken.SecretID,
			ErrExpected: true,
		},
		{
			Name:        "with a token that can submit a job, and use all required volumes",
			Job:         newVolumeJob(false),
			Token:       submitJobWithVolumesReadWriteToken.SecretID,
			ErrExpected: false,
		},
		{
			Name:        "with a token that can submit a job, but only has readonly access",
			Job:         newVolumeJob(false),
			Token:       submitJobWithVolumesReadOnlyToken.SecretID,
			ErrExpected: true,
		},
		{
			Name:        "with a token that can submit a job, and readonly volume access is enough",
			Job:         newVolumeJob(true),
			Token:       submitJobWithVolumesReadOnlyToken.SecretID,
			ErrExpected: false,
		},
		{
			Name:        "with a token that can submit a job, plugin rejected",
			Job:         newCSIPluginJob(),
			Token:       submitJobToken.SecretID,
			ErrExpected: true,
		},
		{
			Name:        "with a token that also has csi-register-plugin, accepted",
			Job:         newCSIPluginJob(),
			Token:       pluginToken.SecretID,
			ErrExpected: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			codec := rpcClient(t, s1)
			req := &structs.JobRegisterRequest{
				Job: tt.Job,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					Namespace: tt.Job.Namespace,
				},
			}
			req.AuthToken = tt.Token

			// Try without a token, expect failure
			var resp structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)

			// If we expected an error, then the job should _not_ be registered.
			if tt.ErrExpected {
				require.Error(t, err, "expected error")
				return
			}

			if !tt.ErrExpected {
				require.NoError(t, err, "unexpected error")
			}

			require.NotEqual(t, 0, resp.Index)

			state := s1.fsm.State()
			ws := memdb.NewWatchSet()
			out, err := state.JobByID(ws, tt.Job.Namespace, tt.Job.ID)
			require.NoError(t, err)
			require.NotNil(t, out)
			require.Equal(t, tt.Job.TaskGroups, out.TaskGroups)
		})
	}
}

func TestJobEndpoint_Register_InvalidNamespace(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.Namespace = "foo"
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Try without a token, expect failure
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), "nonexistent namespace") {
		t.Fatalf("expected namespace error: %v", err)
	}

	// Check for the job in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("expected no job")
	}
}

func TestJobEndpoint_Register_Payload(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request with a job containing an invalid driver
	// config
	job := mock.Job()
	job.Payload = []byte{0x1}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil {
		t.Fatalf("expected a validation error")
	}

	if !strings.Contains(err.Error(), "payload") {
		t.Fatalf("expected a payload error but got: %v", err)
	}
}

func TestJobEndpoint_Register_Existing(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Update the job definition
	job2 := mock.Job()
	job2.Priority = 100
	job2.ID = job.ID
	req.Job = job2

	// Attempt update
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.ModifyIndex != resp.JobModifyIndex {
		t.Fatalf("index mis-match")
	}
	if out.Priority != 100 {
		t.Fatalf("expected update")
	}
	if out.Version != 1 {
		t.Fatalf("expected update")
	}

	// Lookup the evaluation
	eval, err := state.EvalByID(ws, resp.EvalID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eval == nil {
		t.Fatalf("expected eval")
	}
	if eval.CreateIndex != resp.EvalCreateIndex {
		t.Fatalf("index mis-match")
	}

	if eval.Priority != job2.Priority {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Type != job2.Type {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.TriggeredBy != structs.EvalTriggerJobRegister {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobID != job2.ID {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobModifyIndex != resp.JobModifyIndex {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Status != structs.EvalStatusPending {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.CreateTime == 0 {
		t.Fatalf("eval CreateTime is unset: %#v", eval)
	}
	if eval.ModifyTime == 0 {
		t.Fatalf("eval ModifyTime is unset: %#v", eval)
	}

	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Check to ensure the job version didn't get bumped because we submitted
	// the same job
	state = s1.fsm.State()
	ws = memdb.NewWatchSet()
	out, err = state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.Version != 1 {
		t.Fatalf("expected no update; got %v; diff %v", out.Version, pretty.Diff(job2, out))
	}
}

func TestJobEndpoint_Register_Periodic(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request for a periodic job.
	job := mock.PeriodicJob()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.JobModifyIndex == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.CreateIndex != resp.JobModifyIndex {
		t.Fatalf("index mis-match")
	}
	serviceName := out.TaskGroups[0].Tasks[0].Services[0].Name
	expectedServiceName := "web-frontend"
	if serviceName != expectedServiceName {
		t.Fatalf("Expected Service Name: %s, Actual: %s", expectedServiceName, serviceName)
	}

	if resp.EvalID != "" {
		t.Fatalf("Register created an eval for a periodic job")
	}
}

func TestJobEndpoint_Register_ParameterizedJob(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request for a parameterized job.
	job := mock.BatchJob()
	job.ParameterizedJob = &structs.ParameterizedJobConfig{}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.JobModifyIndex == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Check for the job in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.CreateIndex != resp.JobModifyIndex {
		t.Fatalf("index mis-match")
	}
	if resp.EvalID != "" {
		t.Fatalf("Register created an eval for a parameterized job")
	}
}

func TestJobEndpoint_Register_Dispatched(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request with a job with 'Dispatch' set to true
	job := mock.Job()
	job.Dispatched = true
	job.ParameterizedJob = &structs.ParameterizedJobConfig{}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "job can't be submitted with 'Dispatched'")
}

func TestJobEndpoint_Register_EnforceIndex(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request and enforcing an incorrect index
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job:            job,
		EnforceIndex:   true,
		JobModifyIndex: 100, // Not registered yet so not possible
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), RegisterEnforceIndexErrPrefix) {
		t.Fatalf("expected enforcement error")
	}

	// Create the register request and enforcing it is new
	req = &structs.JobRegisterRequest{
		Job:            job,
		EnforceIndex:   true,
		JobModifyIndex: 0,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	curIndex := resp.JobModifyIndex

	// Check for the node in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.CreateIndex != resp.JobModifyIndex {
		t.Fatalf("index mis-match")
	}

	// Reregister request and enforcing it be a new job
	req = &structs.JobRegisterRequest{
		Job:            job,
		EnforceIndex:   true,
		JobModifyIndex: 0,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	err = msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), RegisterEnforceIndexErrPrefix) {
		t.Fatalf("expected enforcement error")
	}

	// Reregister request and enforcing it be at an incorrect index
	req = &structs.JobRegisterRequest{
		Job:            job,
		EnforceIndex:   true,
		JobModifyIndex: curIndex - 1,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	err = msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), RegisterEnforceIndexErrPrefix) {
		t.Fatalf("expected enforcement error")
	}

	// Reregister request and enforcing it be at the correct index
	job.Priority = job.Priority + 1
	req = &structs.JobRegisterRequest{
		Job:            job,
		EnforceIndex:   true,
		JobModifyIndex: curIndex,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	out, err = state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.Priority != job.Priority {
		t.Fatalf("priority mis-match")
	}
}

// TestJobEndpoint_Register_Vault_Disabled asserts that submitting a job that
// uses Vault when Vault is *disabled* results in an error.
func TestJobEndpoint_Register_Vault_Disabled(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
		f := false
		c.GetDefaultVault().Enabled = &f
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request with a job asking for a vault policy
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:   []string{"foo"},
		ChangeMode: structs.VaultChangeModeRestart,
	}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), `Vault "default" not enabled`) {
		t.Fatalf("expected Vault not enabled error: %v", err)
	}
}

// TestJobEndpoint_Register_Vault_AllowUnauthenticated asserts submitting a job
// with a Vault policy but without a Vault token is *succeeds* if
// allow_unauthenticated=true.
func TestJobEndpoint_Register_Vault_AllowUnauthenticated(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Enable vault and allow authenticated
	tr := true
	s1.config.GetDefaultVault().Enabled = &tr
	s1.config.GetDefaultVault().AllowUnauthenticated = &tr

	// Replace the Vault Client on the server
	s1.vault = &TestVaultClient{}

	// Create the register request with a job asking for a vault policy
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:   []string{"foo"},
		ChangeMode: structs.VaultChangeModeRestart,
	}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check for the job in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.CreateIndex != resp.JobModifyIndex {
		t.Fatalf("index mis-match")
	}
}

// TestJobEndpoint_Register_Vault_OverrideConstraint asserts that job
// submitters can specify their own Vault constraint to override the
// automatically injected one.
func TestJobEndpoint_Register_Vault_OverrideConstraint(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Enable vault and allow authenticated
	tr := true
	s1.config.GetDefaultVault().Enabled = &tr
	s1.config.GetDefaultVault().AllowUnauthenticated = &tr

	// Replace the Vault Client on the server
	s1.vault = &TestVaultClient{}

	// Create the register request with a job asking for a vault policy
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:   []string{"foo"},
		ChangeMode: structs.VaultChangeModeRestart,
	}

	vaultConstraint := &structs.Constraint{
		LTarget: "${attr.vault.version}",
		Operand: "is_set",
	}
	job.TaskGroups[0].Tasks[0].Constraints = []*structs.Constraint{vaultConstraint}

	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	must.NoError(t, err)

	// Check for the job in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.NotNil(t, out)
	must.Eq(t, resp.JobModifyIndex, out.CreateIndex)

	// Assert constraint was not overridden by the server
	outConstraints := out.TaskGroups[0].Tasks[0].Constraints
	for _, constraint := range outConstraints {
		if constraint.LTarget == "${attr.vault.version}" {
			must.Eq(t, constraint, vaultConstraint)
		}
	}
	must.True(t, job.TaskGroups[0].Tasks[0].Constraints[0].Equal(outConstraints[0]))
}

func TestJobEndpoint_Register_Vault_NoToken(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Enable vault
	tr, f := true, false
	s1.config.GetDefaultVault().Enabled = &tr
	s1.config.GetDefaultVault().AllowUnauthenticated = &f

	// Replace the Vault Client on the server
	s1.vault = &TestVaultClient{}

	// Create the register request with a job asking for a vault policy but
	// don't send a Vault token
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:   []string{"foo"},
		ChangeMode: structs.VaultChangeModeRestart,
	}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), "missing Vault token") {
		t.Fatalf("expected Vault not enabled error: %v", err)
	}
}

func TestJobEndpoint_Register_Vault_Policies(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Enable vault
	tr, f := true, false
	s1.config.GetDefaultVault().Enabled = &tr
	s1.config.GetDefaultVault().AllowUnauthenticated = &f

	// Replace the Vault Client on the server
	tvc := &TestVaultClient{}
	s1.vault = tvc

	// Add three tokens: one that allows the requesting policy, one that does
	// not and one that returns an error
	policy := "foo"

	badToken := uuid.Generate()
	badPolicies := []string{"a", "b", "c"}
	tvc.SetLookupTokenAllowedPolicies(badToken, badPolicies)

	goodToken := uuid.Generate()
	goodPolicies := []string{"foo", "bar", "baz"}
	tvc.SetLookupTokenAllowedPolicies(goodToken, goodPolicies)

	rootToken := uuid.Generate()
	rootPolicies := []string{"root"}
	tvc.SetLookupTokenAllowedPolicies(rootToken, rootPolicies)

	errToken := uuid.Generate()
	expectedErr := fmt.Errorf("return errors from vault")
	tvc.SetLookupTokenError(errToken, expectedErr)

	// Create the register request with a job asking for a vault policy but
	// send the bad Vault token
	job := mock.Job()
	job.VaultToken = badToken
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:   []string{policy},
		ChangeMode: structs.VaultChangeModeRestart,
	}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(),
		"doesn't allow access to the following policies: "+policy) {
		t.Fatalf("expected permission denied error: %v", err)
	}

	// Use the err token
	job.VaultToken = errToken
	err = msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Fatalf("expected permission denied error: %v", err)
	}

	// Use the good token
	job.VaultToken = goodToken

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check for the job in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.CreateIndex != resp.JobModifyIndex {
		t.Fatalf("index mis-match")
	}
	if out.VaultToken != "" {
		t.Fatalf("vault token not cleared")
	}

	// Check that an implicit constraints were created for Vault and Consul.
	constraints := out.TaskGroups[0].Constraints
	if l := len(constraints); l != 2 {
		t.Fatalf("Unexpected number of tests: %v", l)
	}

	require.ElementsMatch(t, constraints, []*structs.Constraint{consulServiceDiscoveryConstraint, vaultConstraint})

	// Create the register request with another job asking for a vault policy but
	// send the root Vault token
	job2 := mock.Job()
	job2.VaultToken = rootToken
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:   []string{policy},
		ChangeMode: structs.VaultChangeModeRestart,
	}
	req = &structs.JobRegisterRequest{
		Job: job2,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check for the job in the FSM
	out, err = state.JobByID(ws, job2.Namespace, job2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.CreateIndex != resp.JobModifyIndex {
		t.Fatalf("index mis-match")
	}
	if out.VaultToken != "" {
		t.Fatalf("vault token not cleared")
	}
}

func TestJobEndpoint_Register_Vault_MultiNamespaces(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Enable vault
	tr, f := true, false
	s1.config.GetDefaultVault().Enabled = &tr
	s1.config.GetDefaultVault().AllowUnauthenticated = &f

	// Replace the Vault Client on the server
	tvc := &TestVaultClient{}
	s1.vault = tvc

	goodToken := uuid.Generate()
	goodPolicies := []string{"foo", "bar", "baz"}
	tvc.SetLookupTokenAllowedPolicies(goodToken, goodPolicies)

	// Create the register request with a job asking for a vault policy but
	// don't send a Vault token
	job := mock.Job()
	job.VaultToken = goodToken
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Namespace:  "ns1",
		Policies:   []string{"foo"},
		ChangeMode: structs.VaultChangeModeRestart,
	}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	// OSS or Ent check
	if err != nil && s1.EnterpriseState.Features() == 0 {
		// errors.Is cannot be used because the RPC call break error wrapping.
		require.Contains(t, err.Error(), ErrMultipleNamespaces.Error())
	} else {
		require.NoError(t, err)
	}
}

// TestJobEndpoint_Register_SemverConstraint asserts that semver ordering is
// used when evaluating semver constraints.
func TestJobEndpoint_Register_SemverConstraint(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.State()

	// Create a job with a semver constraint
	job := mock.Job()
	job.Constraints = []*structs.Constraint{
		{
			LTarget: "${attr.vault.version}",
			RTarget: ">= 0.6.1",
			Operand: structs.ConstraintSemver,
		},
	}
	job.TaskGroups[0].Count = 1

	// Insert 2 Nodes, 1 matching the constraint, 1 not
	node1 := mock.Node()
	node1.Attributes["vault.version"] = "1.3.0-beta1+ent"
	node1.ComputeClass()
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1, node1))

	node2 := mock.Node()
	delete(node2.Attributes, "vault.version")
	node2.ComputeClass()
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 2, node2))

	// Create the register request
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	require.NotZero(t, resp.Index)

	// Wait for placements
	allocReq := &structs.JobSpecificRequest{
		JobID: job.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}

	testutil.WaitForResult(func() (bool, error) {
		resp := structs.JobAllocationsResponse{}
		err := msgpackrpc.CallWithCodec(codec, "Job.Allocations", allocReq, &resp)
		if err != nil {
			return false, err
		}
		if n := len(resp.Allocations); n != 1 {
			return false, fmt.Errorf("expected 1 alloc, found %d", n)
		}
		alloc := resp.Allocations[0]
		if alloc.NodeID != node1.ID {
			return false, fmt.Errorf("expected alloc to be one node=%q but found node=%q",
				node1.ID, alloc.NodeID)
		}
		return true, nil
	}, func(waitErr error) {
		evals, err := state.EvalsByJob(nil, structs.DefaultNamespace, job.ID)
		require.NoError(t, err)
		for i, e := range evals {
			t.Logf("%d Eval: %s", i, pretty.Sprint(e))
		}

		require.NoError(t, waitErr)
	})
}

// TestJobEndpoint_Register_EvalCreation asserts that job register creates an
// eval atomically with the registration
func TestJobEndpoint_Register_EvalCreation_Modern(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()

	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	t.Run("job registration always create evals", func(t *testing.T) {
		job := mock.Job()
		req := &structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: job.Namespace,
			},
		}

		//// initial registration should create the job and a new eval
		var resp structs.JobRegisterResponse
		err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
		require.NoError(t, err)
		require.NotZero(t, resp.Index)
		require.NotEmpty(t, resp.EvalID)

		// Check for the job in the FSM
		state := s1.fsm.State()
		out, err := state.JobByID(nil, job.Namespace, job.ID)
		require.NoError(t, err)
		require.NotNil(t, out)
		require.Equal(t, resp.JobModifyIndex, out.CreateIndex)

		// Lookup the evaluation
		eval, err := state.EvalByID(nil, resp.EvalID)
		require.NoError(t, err)
		require.NotNil(t, eval)
		require.Equal(t, resp.EvalCreateIndex, eval.CreateIndex)
		require.Nil(t, evalUpdateFromRaft(t, s1, eval.ID))

		//// re-registration should create a new eval, but leave the job untouched
		var resp2 structs.JobRegisterResponse
		err = msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp2)
		require.NoError(t, err)
		require.NotZero(t, resp2.Index)
		require.NotEmpty(t, resp2.EvalID)
		require.NotEqual(t, resp.EvalID, resp2.EvalID)

		// Check for the job in the FSM
		state = s1.fsm.State()
		out, err = state.JobByID(nil, job.Namespace, job.ID)
		require.NoError(t, err)
		require.NotNil(t, out)
		require.Equal(t, resp2.JobModifyIndex, out.CreateIndex)
		require.Equal(t, out.CreateIndex, out.JobModifyIndex)

		// Lookup the evaluation
		eval, err = state.EvalByID(nil, resp2.EvalID)
		require.NoError(t, err)
		require.NotNil(t, eval)
		require.Equal(t, resp2.EvalCreateIndex, eval.CreateIndex)

		raftEval := evalUpdateFromRaft(t, s1, eval.ID)
		require.Equal(t, raftEval, eval)

		//// an update should update the job and create a new eval
		req.Job.TaskGroups[0].Name += "a"
		var resp3 structs.JobRegisterResponse
		err = msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp3)
		require.NoError(t, err)
		require.NotZero(t, resp3.Index)
		require.NotEmpty(t, resp3.EvalID)
		require.NotEqual(t, resp.EvalID, resp3.EvalID)

		// Check for the job in the FSM
		state = s1.fsm.State()
		out, err = state.JobByID(nil, job.Namespace, job.ID)
		require.NoError(t, err)
		require.NotNil(t, out)
		require.Equal(t, resp3.JobModifyIndex, out.JobModifyIndex)

		// Lookup the evaluation
		eval, err = state.EvalByID(nil, resp3.EvalID)
		require.NoError(t, err)
		require.NotNil(t, eval)
		require.Equal(t, resp3.EvalCreateIndex, eval.CreateIndex)

		require.Nil(t, evalUpdateFromRaft(t, s1, eval.ID))
	})

	// Registering a parameterized job shouldn't create an eval
	t.Run("periodic jobs shouldn't create an eval", func(t *testing.T) {
		job := mock.PeriodicJob()
		req := &structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: job.Namespace,
			},
		}

		var resp structs.JobRegisterResponse
		err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
		require.NoError(t, err)
		require.NotZero(t, resp.Index)
		require.Empty(t, resp.EvalID)

		// Check for the job in the FSM
		state := s1.fsm.State()
		out, err := state.JobByID(nil, job.Namespace, job.ID)
		require.NoError(t, err)
		require.NotNil(t, out)
		require.Equal(t, resp.JobModifyIndex, out.CreateIndex)
	})
}

func TestJobEndpoint_Register_ValidateMemoryMax(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	submitNewJob := func() *structs.JobRegisterResponse {
		job := mock.Job()
		job.TaskGroups[0].Tasks[0].Resources.MemoryMaxMB = 2000

		req := &structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: job.Namespace,
			},
		}

		// Fetch the response
		var resp structs.JobRegisterResponse
		err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
		require.NoError(t, err)
		return &resp
	}

	// try default case: Memory oversubscription is disabled
	resp := submitNewJob()
	require.Contains(t, resp.Warnings, "Memory oversubscription is not enabled")

	// enable now and try again
	s1.State().SchedulerSetConfig(100, &structs.SchedulerConfiguration{
		MemoryOversubscriptionEnabled: true,
	})
	resp = submitNewJob()
	require.Empty(t, resp.Warnings)
}

func TestJobEndpoint_Register_ValidateMemoryMax_NodePool(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Store default scheduler configuration to reset between test cases.
	_, defaultSchedConfig, err := s.State().SchedulerConfig()
	must.NoError(t, err)

	// Create test node pools.
	noSchedConfig := mock.NodePool()
	noSchedConfig.SchedulerConfiguration = nil

	withMemOversub := mock.NodePool()
	withMemOversub.SchedulerConfiguration = &structs.NodePoolSchedulerConfiguration{
		MemoryOversubscriptionEnabled: pointer.Of(true),
	}

	noMemOversub := mock.NodePool()
	noMemOversub.SchedulerConfiguration = &structs.NodePoolSchedulerConfiguration{
		MemoryOversubscriptionEnabled: pointer.Of(false),
	}

	s.State().UpsertNodePools(structs.MsgTypeTestSetup, 100, []*structs.NodePool{
		noSchedConfig,
		withMemOversub,
		noMemOversub,
	})

	testCases := []struct {
		name            string
		pool            string
		globalConfig    *structs.SchedulerConfiguration
		expectedWarning string
	}{
		{
			name: "no scheduler config uses global config",
			pool: noSchedConfig.Name,
			globalConfig: &structs.SchedulerConfiguration{
				MemoryOversubscriptionEnabled: true,
			},
			expectedWarning: "",
		},
		{
			name: "enabled via node pool",
			pool: withMemOversub.Name,
			globalConfig: &structs.SchedulerConfiguration{
				MemoryOversubscriptionEnabled: false,
			},
			expectedWarning: "",
		},
		{
			name: "disabled via node pool",
			pool: noMemOversub.Name,
			globalConfig: &structs.SchedulerConfiguration{
				MemoryOversubscriptionEnabled: true,
			},
			expectedWarning: "Memory oversubscription is not enabled",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set global scheduler config if provided.
			if tc.globalConfig != nil {
				idx, err := s.State().LatestIndex()
				must.NoError(t, err)

				err = s.State().SchedulerSetConfig(idx, tc.globalConfig)
				must.NoError(t, err)
			}

			// Create job with node_pool and memory_max.
			job := mock.Job()
			job.TaskGroups[0].Tasks[0].Resources.MemoryMaxMB = 2000
			job.NodePool = tc.pool

			req := &structs.JobRegisterRequest{
				Job: job,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					Namespace: job.Namespace,
				},
			}
			var resp structs.JobRegisterResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)

			// Validate respose.
			must.NoError(t, err)
			if tc.expectedWarning != "" {
				must.StrContains(t, resp.Warnings, tc.expectedWarning)
			} else {
				must.Eq(t, "", resp.Warnings)
			}

			// Reset to default global scheduler config.
			err = s.State().SchedulerSetConfig(resp.Index+1, defaultSchedConfig)
			must.NoError(t, err)
		})
	}
}

// evalUpdateFromRaft searches the raft logs for the eval update pertaining to the eval
func evalUpdateFromRaft(t *testing.T, s *Server, evalID string) *structs.Evaluation {
	var store raft.LogStore = s.raftInmem
	if store == nil {
		store = s.raftStore
	}
	require.NotNil(t, store)

	li, _ := store.LastIndex()
	for i, _ := store.FirstIndex(); i <= li; i++ {
		var log raft.Log
		err := store.GetLog(i, &log)
		require.NoError(t, err)

		if log.Type != raft.LogCommand {
			continue
		}

		if structs.MessageType(log.Data[0]) != structs.EvalUpdateRequestType {
			continue
		}

		var req structs.EvalUpdateRequest
		structs.Decode(log.Data[1:], &req)
		require.NoError(t, err)

		for _, eval := range req.Evals {
			if eval.ID == evalID {
				eval.CreateIndex = i
				eval.ModifyIndex = i
				return eval
			}
		}
	}

	return nil
}

func TestJobEndpoint_Register_ACL_Namespace(t *testing.T) {
	ci.Parallel(t)
	s1, _, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Policy with read on default namespace and write on non default
	policy := &structs.ACLPolicy{
		Name:        fmt.Sprintf("policy-%s", uuid.Generate()),
		Description: "Super cool policy!",
		Rules: `
		namespace "default" {
			policy = "read"
		}
		namespace "test" {
			policy = "write"
		}
		node {
			policy = "read"
		}
		agent {
			policy = "read"
		}
		`,
		CreateIndex: 10,
		ModifyIndex: 20,
	}
	policy.SetHash()

	assert := assert.New(t)

	// Upsert policy and token
	token := mock.ACLToken()
	token.Policies = []string{policy.Name}
	err := s1.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 100, []*structs.ACLPolicy{policy})
	assert.Nil(err)

	err = s1.State().UpsertACLTokens(structs.MsgTypeTestSetup, 110, []*structs.ACLToken{token})
	assert.Nil(err)

	// Upsert namespace
	ns := mock.Namespace()
	ns.Name = "test"
	err = s1.fsm.State().UpsertNamespaces(1000, []*structs.Namespace{ns})
	assert.Nil(err)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	req.AuthToken = token.SecretID
	// Use token without write access to default namespace, expect failure
	var resp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	assert.NotNil(err, "expected permission denied")

	req.Namespace = "test"
	job.Namespace = "test"

	// Use token with write access to default namespace, expect success
	err = msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	assert.Nil(err, "unexpected err: %v", err)
	assert.NotEqual(resp.Index, 0, "bad index: %d", resp.Index)

	// Check for the node in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	assert.Nil(err)
	assert.NotNil(out, "expected job")
}

func TestJobRegister_ACL_RejectedBySchedulerConfig(t *testing.T) {
	ci.Parallel(t)
	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	submitJobToken := mock.CreatePolicyAndToken(t, s1.State(), 1001, "test-valid-write",
		mock.NamespacePolicy(structs.DefaultNamespace, "write", nil)).
		SecretID

	cases := []struct {
		name          string
		token         string
		rejectEnabled bool
		errExpected   string
	}{
		{
			name:          "reject disabled, with a submit token",
			token:         submitJobToken,
			rejectEnabled: false,
		},
		{
			name:          "reject enabled, with a submit token",
			token:         submitJobToken,
			rejectEnabled: true,
			errExpected:   structs.ErrJobRegistrationDisabled.Error(),
		},
		{
			name:          "reject enabled, without a token",
			token:         "",
			rejectEnabled: true,
			errExpected:   structs.ErrJobRegistrationDisabled.Error(),
		},
		{
			name:          "reject enabled, with a management token",
			token:         root.SecretID,
			rejectEnabled: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			job := mock.Job()
			req := &structs.JobRegisterRequest{
				Job: job,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					Namespace: job.Namespace,
				},
			}
			req.AuthToken = tc.token

			cfgReq := &structs.SchedulerSetConfigRequest{
				Config: structs.SchedulerConfiguration{
					RejectJobRegistration: tc.rejectEnabled,
				},
				WriteRequest: structs.WriteRequest{
					Region: "global",
				},
			}
			cfgReq.AuthToken = root.SecretID
			err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerSetConfiguration",
				cfgReq, &structs.SchedulerSetConfigurationResponse{},
			)
			require.NoError(t, err)

			var resp structs.JobRegisterResponse
			err = msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)

			if tc.errExpected != "" {
				require.Error(t, err, "expected error")
				require.EqualError(t, err, tc.errExpected)
				state := s1.fsm.State()
				ws := memdb.NewWatchSet()
				out, err := state.JobByID(ws, job.Namespace, job.ID)
				require.NoError(t, err)
				require.Nil(t, out)
			} else {
				require.NoError(t, err, "unexpected error")
				require.NotEqual(t, 0, resp.Index)
				state := s1.fsm.State()
				ws := memdb.NewWatchSet()
				out, err := state.JobByID(ws, job.Namespace, job.ID)
				require.NoError(t, err)
				require.NotNil(t, out)
				require.Equal(t, job.TaskGroups, out.TaskGroups)
			}
		})
	}
}

func TestJobEndpoint_Register_PortCollistion(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		configFn func(c *Config)
	}{
		{
			name: "no preemption",
			configFn: func(c *Config) {
				c.DefaultSchedulerConfig = structs.SchedulerConfiguration{
					PreemptionConfig: structs.PreemptionConfig{
						ServiceSchedulerEnabled: false,
					},
				}
			},
		},
		{
			name: "with preemption",
			configFn: func(c *Config) {
				c.DefaultSchedulerConfig = structs.SchedulerConfiguration{
					PreemptionConfig: structs.PreemptionConfig{
						ServiceSchedulerEnabled: true,
					},
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s1, cleanupS1 := TestServer(t, tc.configFn)
			defer cleanupS1()
			state := s1.fsm.State()

			// Create test node.
			node := mock.Node()
			must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1000, node))

			// Create test job with a static port.
			job := mock.Job()
			job.TaskGroups[0].Count = 1
			job.TaskGroups[0].Networks[0].DynamicPorts = nil
			job.TaskGroups[0].Networks[0].ReservedPorts = []structs.Port{
				{Label: "http", Value: 80},
			}
			job.TaskGroups[0].Tasks[0].Services = nil

			testutil.RegisterJob(t, s1.RPC, job)
			testutil.WaitForJobAllocStatus(t, s1.RPC, job, map[string]int{
				structs.AllocClientStatusPending: 1,
			})

			// Register second job with port conflict.
			job2 := job.Copy()
			job2.ID = fmt.Sprintf("conflict-%s", uuid.Generate())
			job2.Name = job2.ID

			testutil.RegisterJob(t, s1.RPC, job2)

			// Wait for job registration eval to complete.
			evals := testutil.WaitForJobEvalStatus(t, s1.RPC, job2, map[string]int{
				structs.EvalStatusComplete: 1,
				structs.EvalStatusBlocked:  1,
			})

			var blockedEval *structs.Evaluation
			for _, e := range evals {
				if e.Status == structs.EvalStatusBlocked {
					blockedEval = e
					break
				}
			}

			// Ensure blocked eval is properly annotated.
			must.MapLen(t, 1, blockedEval.FailedTGAllocs)
			must.NotNil(t, blockedEval.FailedTGAllocs["web"])
			must.Eq(t, map[string]int{
				"network: reserved port collision http=80": 1,
			}, blockedEval.FailedTGAllocs["web"].DimensionExhausted)
		})
	}
}

func TestJobEndpoint_Revert(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the initial register request
	job := mock.Job()
	job.Priority = 100
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Reregister again to get another version
	job2 := job.Copy()
	job2.Priority = 1
	req = &structs.JobRegisterRequest{
		Job: job2,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Create revert request and enforcing it be at an incorrect version
	revertReq := &structs.JobRevertRequest{
		JobID:               job.ID,
		JobVersion:          0,
		EnforcePriorVersion: pointer.Of(uint64(10)),
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	err := msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &resp)
	if err == nil || !strings.Contains(err.Error(), "enforcing version 10") {
		t.Fatalf("expected enforcement error")
	}

	// Create revert request and enforcing it be at the current version
	revertReq = &structs.JobRevertRequest{
		JobID:      job.ID,
		JobVersion: 1,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	err = msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &resp)
	if err == nil || !strings.Contains(err.Error(), "current version") {
		t.Fatalf("expected current version err: %v", err)
	}

	// Create revert request and enforcing it be at version 1
	revertReq = &structs.JobRevertRequest{
		JobID:               job.ID,
		JobVersion:          0,
		EnforcePriorVersion: pointer.Of(uint64(1)),
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}
	if resp.EvalID == "" || resp.EvalCreateIndex == 0 {
		t.Fatalf("bad created eval: %+v", resp)
	}
	if resp.JobModifyIndex == 0 {
		t.Fatalf("bad job modify index: %d", resp.JobModifyIndex)
	}

	// Create revert request and don't enforce. We are at version 2 but it is
	// the same as version 0
	revertReq = &structs.JobRevertRequest{
		JobID:      job.ID,
		JobVersion: 0,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}
	if resp.EvalID == "" || resp.EvalCreateIndex == 0 {
		t.Fatalf("bad created eval: %+v", resp)
	}
	if resp.JobModifyIndex == 0 {
		t.Fatalf("bad job modify index: %d", resp.JobModifyIndex)
	}

	// Check that the job is at the correct version and that the eval was
	// created
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.Priority != job.Priority {
		t.Fatalf("priority mis-match")
	}
	if out.Version != 2 {
		t.Fatalf("got version %d; want %d", out.Version, 2)
	}

	eout, err := state.EvalByID(ws, resp.EvalID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eout == nil {
		t.Fatalf("expected eval")
	}
	if eout.JobID != job.ID {
		t.Fatalf("job id mis-match")
	}

	versions, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("got %d versions; want %d", len(versions), 3)
	}
}

func TestJobEndpoint_Revert_Vault_NoToken(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Enable vault
	tr, f := true, false
	s1.config.GetDefaultVault().Enabled = &tr
	s1.config.GetDefaultVault().AllowUnauthenticated = &f

	// Replace the Vault Client on the server
	tvc := &TestVaultClient{}
	s1.vault = tvc

	// Add three tokens: one that allows the requesting policy, one that does
	// not and one that returns an error
	policy := "foo"

	goodToken := uuid.Generate()
	goodPolicies := []string{"foo", "bar", "baz"}
	tvc.SetLookupTokenAllowedPolicies(goodToken, goodPolicies)

	// Create the initial register request
	job := mock.Job()
	job.VaultToken = goodToken
	job.Priority = 100
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:   []string{policy},
		ChangeMode: structs.VaultChangeModeRestart,
	}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Reregister again to get another version
	job2 := job.Copy()
	job2.Priority = 1
	req = &structs.JobRegisterRequest{
		Job: job2,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	revertReq := &structs.JobRevertRequest{
		JobID:      job.ID,
		JobVersion: 1,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	err := msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &resp)
	if err == nil || !strings.Contains(err.Error(), "current version") {
		t.Fatalf("expected current version err: %v", err)
	}

	// Create revert request and enforcing it be at version 1
	revertReq = &structs.JobRevertRequest{
		JobID:               job.ID,
		JobVersion:          0,
		EnforcePriorVersion: pointer.Of(uint64(1)),
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	err = msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &resp)
	if err == nil || !strings.Contains(err.Error(), "missing Vault token") {
		t.Fatalf("expected Vault not enabled error: %v", err)
	}
}

// TestJobEndpoint_Revert_Vault_Policies asserts that job revert uses the
// revert request's Vault token when authorizing policies.
func TestJobEndpoint_Revert_Vault_Policies(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Enable vault
	tr, f := true, false
	s1.config.GetDefaultVault().Enabled = &tr
	s1.config.GetDefaultVault().AllowUnauthenticated = &f

	// Replace the Vault Client on the server
	tvc := &TestVaultClient{}
	s1.vault = tvc

	// Add three tokens: one that allows the requesting policy, one that does
	// not and one that returns an error
	policy := "foo"

	badToken := uuid.Generate()
	badPolicies := []string{"a", "b", "c"}
	tvc.SetLookupTokenAllowedPolicies(badToken, badPolicies)

	registerGoodToken := uuid.Generate()
	goodPolicies := []string{"foo", "bar", "baz"}
	tvc.SetLookupTokenAllowedPolicies(registerGoodToken, goodPolicies)

	revertGoodToken := uuid.Generate()
	revertGoodPolicies := []string{"foo", "bar_revert", "baz_revert"}
	tvc.SetLookupTokenAllowedPolicies(revertGoodToken, revertGoodPolicies)

	rootToken := uuid.Generate()
	rootPolicies := []string{"root"}
	tvc.SetLookupTokenAllowedPolicies(rootToken, rootPolicies)

	errToken := uuid.Generate()
	expectedErr := fmt.Errorf("return errors from vault")
	tvc.SetLookupTokenError(errToken, expectedErr)

	// Create the initial register request
	job := mock.Job()
	job.VaultToken = registerGoodToken
	job.Priority = 100
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:   []string{policy},
		ChangeMode: structs.VaultChangeModeRestart,
	}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Reregister again to get another version
	job2 := job.Copy()
	job2.Priority = 1
	req = &structs.JobRegisterRequest{
		Job: job2,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create the revert request with the bad Vault token
	revertReq := &structs.JobRevertRequest{
		JobID:      job.ID,
		JobVersion: 0,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
		VaultToken: badToken,
	}

	// Fetch the response
	err := msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &resp)
	if err == nil || !strings.Contains(err.Error(),
		"doesn't allow access to the following policies: "+policy) {
		t.Fatalf("expected permission denied error: %v", err)
	}

	// Use the err token
	revertReq.VaultToken = errToken
	err = msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &resp)
	if err == nil || !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Fatalf("expected permission denied error: %v", err)
	}

	// Use a good token
	revertReq.VaultToken = revertGoodToken
	if err := msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}
}

func TestJobEndpoint_Revert_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})

	defer cleanupS1()
	codec := rpcClient(t, s1)
	state := s1.fsm.State()
	testutil.WaitForLeader(t, s1.RPC)

	// Create the jobs
	job := mock.Job()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 300, nil, job)
	require.Nil(err)

	job2 := job.Copy()
	job2.Priority = 1
	err = state.UpsertJob(structs.MsgTypeTestSetup, 400, nil, job2)
	require.Nil(err)

	// Create revert request and enforcing it be at the current version
	revertReq := &structs.JobRevertRequest{
		JobID:      job.ID,
		JobVersion: 0,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Attempt to fetch the response without a valid token
	var resp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	revertReq.AuthToken = invalidToken.SecretID
	var invalidResp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Fetch the response with a valid management token
	revertReq.AuthToken = root.SecretID
	var validResp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &validResp)
	require.Nil(err)

	// Try with a valid non-management token
	validToken := mock.CreatePolicyAndToken(t, state, 1003, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob}))

	revertReq.AuthToken = validToken.SecretID
	var validResp2 structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &validResp2)
	require.Nil(err)
}

func TestJobEndpoint_Stable(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the initial register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Create stability request
	stableReq := &structs.JobStabilityRequest{
		JobID:      job.ID,
		JobVersion: 0,
		Stable:     true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var stableResp structs.JobStabilityResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Stable", stableReq, &stableResp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if stableResp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Check that the job is marked stable
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if !out.Stable {
		t.Fatalf("Job is not marked stable")
	}
}

func TestJobEndpoint_Stable_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	state := s1.fsm.State()
	testutil.WaitForLeader(t, s1.RPC)

	// Register the job
	job := mock.Job()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	require.Nil(err)

	// Create stability request
	stableReq := &structs.JobStabilityRequest{
		JobID:      job.ID,
		JobVersion: 0,
		Stable:     true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Attempt to fetch the token without a token
	var stableResp structs.JobStabilityResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Stable", stableReq, &stableResp)
	require.NotNil(err)
	require.Contains("Permission denied", err.Error())

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	stableReq.AuthToken = invalidToken.SecretID
	var invalidStableResp structs.JobStabilityResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Stable", stableReq, &invalidStableResp)
	require.NotNil(err)
	require.Contains("Permission denied", err.Error())

	// Attempt to fetch with a management token
	stableReq.AuthToken = root.SecretID
	var validStableResp structs.JobStabilityResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Stable", stableReq, &validStableResp)
	require.Nil(err)

	// Attempt to fetch with a valid token
	validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob}))

	stableReq.AuthToken = validToken.SecretID
	var validStableResp2 structs.JobStabilityResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Stable", stableReq, &validStableResp2)
	require.Nil(err)

	// Check that the job is marked stable
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	require.Nil(err)
	require.NotNil(job)
	require.Equal(true, out.Stable)
}

func TestJobEndpoint_Evaluate(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Force a re-evaluation
	reEval := &structs.JobEvaluateRequest{
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Lookup the evaluation
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, resp.EvalID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eval == nil {
		t.Fatalf("expected eval")
	}
	if eval.CreateIndex != resp.EvalCreateIndex {
		t.Fatalf("index mis-match")
	}

	if eval.Priority != job.Priority {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Type != job.Type {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.TriggeredBy != structs.EvalTriggerJobRegister {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobID != job.ID {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobModifyIndex != resp.JobModifyIndex {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Status != structs.EvalStatusPending {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.CreateTime == 0 {
		t.Fatalf("eval CreateTime is unset: %#v", eval)
	}
	if eval.ModifyTime == 0 {
		t.Fatalf("eval ModifyTime is unset: %#v", eval)
	}
}

func TestJobEndpoint_ForceRescheduleEvaluate(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	require.Nil(err)
	require.NotEqual(0, resp.Index)

	state := s1.fsm.State()
	job, err = state.JobByID(nil, structs.DefaultNamespace, job.ID)
	require.Nil(err)

	// Create a failed alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.TaskGroup = job.TaskGroups[0].Name
	alloc.Namespace = job.Namespace
	alloc.ClientStatus = structs.AllocClientStatusFailed
	err = s1.State().UpsertAllocs(structs.MsgTypeTestSetup, resp.Index+1, []*structs.Allocation{alloc})
	require.Nil(err)

	// Force a re-evaluation
	reEval := &structs.JobEvaluateRequest{
		JobID:       job.ID,
		EvalOptions: structs.EvalOptions{ForceReschedule: true},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &resp)
	require.Nil(err)
	require.NotEqual(0, resp.Index)

	// Lookup the evaluation
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, resp.EvalID)
	require.Nil(err)
	require.NotNil(eval)
	require.Equal(eval.CreateIndex, resp.EvalCreateIndex)
	require.Equal(eval.Priority, job.Priority)
	require.Equal(eval.Type, job.Type)
	require.Equal(eval.TriggeredBy, structs.EvalTriggerJobRegister)
	require.Equal(eval.JobID, job.ID)
	require.Equal(eval.JobModifyIndex, resp.JobModifyIndex)
	require.Equal(eval.Status, structs.EvalStatusPending)
	require.NotZero(eval.CreateTime)
	require.NotZero(eval.ModifyTime)

	// Lookup the alloc, verify DesiredTransition ForceReschedule
	alloc, err = state.AllocByID(ws, alloc.ID)
	require.NotNil(alloc)
	require.Nil(err)
	require.True(*alloc.DesiredTransition.ForceReschedule)
}

func TestJobEndpoint_Evaluate_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create the job
	job := mock.Job()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 300, nil, job)
	require.Nil(err)

	// Force a re-evaluation
	reEval := &structs.JobEvaluateRequest{
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Attempt to fetch the response without a token
	var resp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	reEval.AuthToken = invalidToken.SecretID
	var invalidResp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Fetch the response with a valid management token
	reEval.AuthToken = root.SecretID
	var validResp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &validResp)
	require.Nil(err)

	// Fetch the response with a valid token
	validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob}))

	reEval.AuthToken = validToken.SecretID
	var validResp2 structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &validResp2)
	require.Nil(err)

	// Lookup the evaluation
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, validResp2.EvalID)
	require.Nil(err)
	require.NotNil(eval)

	require.Equal(eval.CreateIndex, validResp2.EvalCreateIndex)
	require.Equal(eval.Priority, job.Priority)
	require.Equal(eval.Type, job.Type)
	require.Equal(eval.TriggeredBy, structs.EvalTriggerJobRegister)
	require.Equal(eval.JobID, job.ID)
	require.Equal(eval.JobModifyIndex, validResp2.JobModifyIndex)
	require.Equal(eval.Status, structs.EvalStatusPending)
	require.NotZero(eval.CreateTime)
	require.NotZero(eval.ModifyTime)
}

func TestJobEndpoint_Evaluate_Periodic(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.PeriodicJob()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.JobModifyIndex == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Force a re-evaluation
	reEval := &structs.JobEvaluateRequest{
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &resp); err == nil {
		t.Fatal("expect an err")
	}
}

func TestJobEndpoint_Evaluate_ParameterizedJob(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.BatchJob()
	job.ParameterizedJob = &structs.ParameterizedJobConfig{}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.JobModifyIndex == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Force a re-evaluation
	reEval := &structs.JobEvaluateRequest{
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &resp); err == nil {
		t.Fatal("expect an err")
	}
}

func TestJobEndpoint_Deregister(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register requests
	job := mock.Job()
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp))

	// Deregister but don't purge
	dereg := &structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: false,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp2 structs.JobDeregisterResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Deregister", dereg, &resp2))
	require.NotZero(resp2.Index)

	// Check for the job in the FSM
	state := s1.fsm.State()
	out, err := state.JobByID(nil, job.Namespace, job.ID)
	require.Nil(err)
	require.NotNil(out)
	require.True(out.Stop)

	// Lookup the evaluation
	eval, err := state.EvalByID(nil, resp2.EvalID)
	require.Nil(err)
	require.NotNil(eval)
	require.EqualValues(resp2.EvalCreateIndex, eval.CreateIndex)
	require.Equal(job.Priority, eval.Priority)
	require.Equal(job.Type, eval.Type)
	require.Equal(structs.EvalTriggerJobDeregister, eval.TriggeredBy)
	require.Equal(job.ID, eval.JobID)
	require.Equal(structs.EvalStatusPending, eval.Status)
	require.NotZero(eval.CreateTime)
	require.NotZero(eval.ModifyTime)

	// Deregister and purge
	dereg2 := &structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp3 structs.JobDeregisterResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Deregister", dereg2, &resp3))
	require.NotZero(resp3.Index)

	// Check for the job in the FSM
	out, err = state.JobByID(nil, job.Namespace, job.ID)
	require.Nil(err)
	require.Nil(out)

	// Lookup the evaluation
	eval, err = state.EvalByID(nil, resp3.EvalID)
	require.Nil(err)
	require.NotNil(eval)

	require.EqualValues(resp3.EvalCreateIndex, eval.CreateIndex)
	require.Equal(job.Priority, eval.Priority)
	require.Equal(job.Type, eval.Type)
	require.Equal(structs.EvalTriggerJobDeregister, eval.TriggeredBy)
	require.Equal(job.ID, eval.JobID)
	require.Equal(structs.EvalStatusPending, eval.Status)
	require.NotZero(eval.CreateTime)
	require.NotZero(eval.ModifyTime)
}

func TestJobEndpoint_Deregister_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create and register a job
	job := mock.Job()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job)
	require.Nil(err)

	// Deregister and purge
	req := &structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Expect failure for request without a token
	var resp structs.JobDeregisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Deregister", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
	req.AuthToken = invalidToken.SecretID

	var invalidResp structs.JobDeregisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Deregister", req, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Expect success with a valid management token
	req.AuthToken = root.SecretID

	var validResp structs.JobDeregisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Deregister", req, &validResp)
	require.Nil(err)
	require.NotEqual(validResp.Index, 0)

	// Expect success with a valid token
	validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob}))
	req.AuthToken = validToken.SecretID

	// Check for the job in the FSM
	out, err := state.JobByID(nil, job.Namespace, job.ID)
	require.Nil(err)
	require.Nil(out)

	// Lookup the evaluation
	eval, err := state.EvalByID(nil, validResp.EvalID)
	require.Nil(err)
	require.NotNil(eval, nil)

	require.Equal(eval.CreateIndex, validResp.EvalCreateIndex)
	require.Equal(eval.Priority, structs.JobDefaultPriority)
	require.Equal(eval.Type, structs.JobTypeService)
	require.Equal(eval.TriggeredBy, structs.EvalTriggerJobDeregister)
	require.Equal(eval.JobID, job.ID)
	require.Equal(eval.JobModifyIndex, validResp.JobModifyIndex)
	require.Equal(eval.Status, structs.EvalStatusPending)
	require.NotZero(eval.CreateTime)
	require.NotZero(eval.ModifyTime)

	// Deregistration is idempotent
	var validResp2 structs.JobDeregisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Deregister", req, &validResp2)
	must.NoError(t, err)
	must.Eq(t, "", validResp2.EvalID)
}

func TestJobEndpoint_Deregister_Nonexistent(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Deregister
	jobID := "foo"
	dereg := &structs.JobDeregisterRequest{
		JobID: jobID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}
	var resp2 structs.JobDeregisterResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Deregister", dereg, &resp2))
	must.Eq(t, 0, resp2.JobModifyIndex, must.Sprint("expected no modify index"))

	// Lookup the evaluation
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	eval, err := state.EvalsByJob(ws, structs.DefaultNamespace, jobID)
	must.NoError(t, err)
	must.Nil(t, eval)
}

func TestJobEndpoint_Deregister_EvalPriority(t *testing.T) {
	ci.Parallel(t)
	requireAssert := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.Canonicalize()

	// Register the job.
	requireAssert.NoError(msgpackrpc.CallWithCodec(codec, "Job.Register", &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}, &structs.JobRegisterResponse{}))

	// Create the deregister request.
	deregReq := &structs.JobDeregisterRequest{
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
		EvalPriority: 99,
	}
	var deregResp structs.JobDeregisterResponse
	requireAssert.NoError(msgpackrpc.CallWithCodec(codec, "Job.Deregister", deregReq, &deregResp))

	// Grab the eval from the state, and check its priority is as expected.
	out, err := s1.fsm.State().EvalByID(nil, deregResp.EvalID)
	requireAssert.NoError(err)
	requireAssert.Equal(99, out.Priority)
}

func TestJobEndpoint_Deregister_Periodic(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.PeriodicJob()
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Deregister
	dereg := &structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp2 structs.JobDeregisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Deregister", dereg, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.JobModifyIndex == 0 {
		t.Fatalf("bad index: %d", resp2.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("unexpected job")
	}

	if resp.EvalID != "" {
		t.Fatalf("Deregister created an eval for a periodic job")
	}
}

func TestJobEndpoint_Deregister_ParameterizedJob(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.BatchJob()
	job.ParameterizedJob = &structs.ParameterizedJobConfig{}
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Deregister
	dereg := &structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp2 structs.JobDeregisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Deregister", dereg, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.JobModifyIndex == 0 {
		t.Fatalf("bad index: %d", resp2.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("unexpected job")
	}

	if resp.EvalID != "" {
		t.Fatalf("Deregister created an eval for a parameterized job")
	}
}

// TestJobEndpoint_Deregister_EvalCreation asserts that job deregister creates
// an eval atomically with the registration
func TestJobEndpoint_Deregister_EvalCreation(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()

	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	t.Run("job de-registration always create evals", func(t *testing.T) {
		job := mock.Job()
		req := &structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: job.Namespace,
			},
		}

		var resp structs.JobRegisterResponse
		err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
		require.NoError(t, err)

		dereg := &structs.JobDeregisterRequest{
			JobID: job.ID,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: job.Namespace,
			},
		}
		var resp2 structs.JobDeregisterResponse
		err = msgpackrpc.CallWithCodec(codec, "Job.Deregister", dereg, &resp2)
		require.NoError(t, err)
		require.NotEmpty(t, resp2.EvalID)

		state := s1.fsm.State()
		eval, err := state.EvalByID(nil, resp2.EvalID)
		require.Nil(t, err)
		require.NotNil(t, eval)
		require.EqualValues(t, resp2.EvalCreateIndex, eval.CreateIndex)

		require.Nil(t, evalUpdateFromRaft(t, s1, eval.ID))

	})

	// Registering a parameterized job shouldn't create an eval
	t.Run("periodic jobs shouldn't create an eval", func(t *testing.T) {
		job := mock.PeriodicJob()
		req := &structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: job.Namespace,
			},
		}

		var resp structs.JobRegisterResponse
		err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
		require.NoError(t, err)
		require.NotZero(t, resp.Index)

		dereg := &structs.JobDeregisterRequest{
			JobID: job.ID,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: job.Namespace,
			},
		}
		var resp2 structs.JobDeregisterResponse
		err = msgpackrpc.CallWithCodec(codec, "Job.Deregister", dereg, &resp2)
		require.NoError(t, err)
		require.Empty(t, resp2.EvalID)
	})
}

func TestJobEndpoint_Deregister_NoShutdownDelay(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register requests
	job := mock.Job()
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp0 structs.JobRegisterResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp0))

	// Deregister but don't purge
	dereg1 := &structs.JobDeregisterRequest{
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp1 structs.JobDeregisterResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Deregister", dereg1, &resp1))
	require.NotZero(resp1.Index)

	// Check for the job in the FSM
	state := s1.fsm.State()
	out, err := state.JobByID(nil, job.Namespace, job.ID)
	require.NoError(err)
	require.NotNil(out)
	require.True(out.Stop)

	// Lookup the evaluation
	eval, err := state.EvalByID(nil, resp1.EvalID)
	require.NoError(err)
	require.NotNil(eval)
	require.EqualValues(resp1.EvalCreateIndex, eval.CreateIndex)
	require.Equal(structs.EvalTriggerJobDeregister, eval.TriggeredBy)

	// Lookup allocation transitions
	var ws memdb.WatchSet
	allocs, err := state.AllocsByJob(ws, job.Namespace, job.ID, true)
	require.NoError(err)

	for _, alloc := range allocs {
		require.Nil(alloc.DesiredTransition)
	}

	// Deregister with no shutdown delay
	dereg2 := &structs.JobDeregisterRequest{
		JobID:           job.ID,
		NoShutdownDelay: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp2 structs.JobDeregisterResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Deregister", dereg2, &resp2))
	require.NotZero(resp2.Index)

	// Lookup the evaluation
	eval, err = state.EvalByID(nil, resp2.EvalID)
	require.NoError(err)
	require.NotNil(eval)
	require.EqualValues(resp2.EvalCreateIndex, eval.CreateIndex)
	require.Equal(structs.EvalTriggerJobDeregister, eval.TriggeredBy)

	// Lookup allocation transitions
	allocs, err = state.AllocsByJob(ws, job.Namespace, job.ID, true)
	require.NoError(err)

	for _, alloc := range allocs {
		require.NotNil(alloc.DesiredTransition)
		require.True(*(alloc.DesiredTransition.NoShutdownDelay))
	}

}

func TestJobEndpoint_BatchDeregister(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register requests
	job := mock.Job()
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp))

	job2 := mock.Job()
	job2.Priority = 1
	reg2 := &structs.JobRegisterRequest{
		Job: job2,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job2.Namespace,
		},
	}

	// Fetch the response
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", reg2, &resp))

	// Deregister
	dereg := &structs.JobBatchDeregisterRequest{
		Jobs: map[structs.NamespacedID]*structs.JobDeregisterOptions{
			{
				ID:        job.ID,
				Namespace: job.Namespace,
			}: {},
			{
				ID:        job2.ID,
				Namespace: job2.Namespace,
			}: {
				Purge: true,
			},
		},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp2 structs.JobBatchDeregisterResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, structs.JobBatchDeregisterRPCMethod, dereg, &resp2))
	require.NotZero(resp2.Index)

	// Check for the job in the FSM
	state := s1.fsm.State()
	out, err := state.JobByID(nil, job.Namespace, job.ID)
	require.Nil(err)
	require.NotNil(out)
	require.True(out.Stop)

	out, err = state.JobByID(nil, job2.Namespace, job2.ID)
	require.Nil(err)
	require.Nil(out)
}

func TestJobEndpoint_BatchDeregister_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create and register a job
	job, job2 := mock.Job(), mock.Job()
	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job))
	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 101, nil, job2))

	// Deregister
	req := &structs.JobBatchDeregisterRequest{
		Jobs: map[structs.NamespacedID]*structs.JobDeregisterOptions{
			{
				ID:        job.ID,
				Namespace: job.Namespace,
			}: {},
			{
				ID:        job2.ID,
				Namespace: job2.Namespace,
			}: {},
		},
		WriteRequest: structs.WriteRequest{
			Region: "global",
		},
	}

	// Expect failure for request without a token
	var resp structs.JobBatchDeregisterResponse
	err := msgpackrpc.CallWithCodec(codec, structs.JobBatchDeregisterRPCMethod, req, &resp)
	require.NotNil(err)
	require.True(structs.IsErrPermissionDenied(err))

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
	req.AuthToken = invalidToken.SecretID

	var invalidResp structs.JobDeregisterResponse
	err = msgpackrpc.CallWithCodec(codec, structs.JobBatchDeregisterRPCMethod, req, &invalidResp)
	require.NotNil(err)
	require.True(structs.IsErrPermissionDenied(err))

	// Expect success with a valid management token
	req.AuthToken = root.SecretID

	var validResp structs.JobDeregisterResponse
	err = msgpackrpc.CallWithCodec(codec, structs.JobBatchDeregisterRPCMethod, req, &validResp)
	require.Nil(err)
	require.NotEqual(validResp.Index, 0)
}

func TestJobEndpoint_Deregister_Priority(t *testing.T) {
	ci.Parallel(t)
	requireAssertion := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	fsmState := s1.fsm.State()

	// Create a job which a custom priority and register this.
	job := mock.Job()
	job.Priority = 90
	err := fsmState.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job)
	requireAssertion.Nil(err)

	// Deregister.
	dereg := &structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp structs.JobDeregisterResponse
	requireAssertion.Nil(msgpackrpc.CallWithCodec(codec, "Job.Deregister", dereg, &resp))
	requireAssertion.NotZero(resp.Index)

	// Check for the job in the FSM which should not be there as it was purged.
	out, err := fsmState.JobByID(nil, job.Namespace, job.ID)
	requireAssertion.Nil(err)
	requireAssertion.Nil(out)

	// Lookup the evaluation
	eval, err := fsmState.EvalByID(nil, resp.EvalID)
	requireAssertion.Nil(err)
	requireAssertion.NotNil(eval)
	requireAssertion.EqualValues(resp.EvalCreateIndex, eval.CreateIndex)
	requireAssertion.Equal(job.Priority, eval.Priority)
	requireAssertion.Equal(job.Type, eval.Type)
	requireAssertion.Equal(structs.EvalTriggerJobDeregister, eval.TriggeredBy)
	requireAssertion.Equal(job.ID, eval.JobID)
	requireAssertion.Equal(structs.EvalStatusPending, eval.Status)
	requireAssertion.NotZero(eval.CreateTime)
	requireAssertion.NotZero(eval.ModifyTime)
}

func TestJobEndpoint_GetJob(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	job.CreateIndex = resp.JobModifyIndex
	job.ModifyIndex = resp.JobModifyIndex
	job.JobModifyIndex = resp.JobModifyIndex

	// Lookup the job
	get := &structs.JobSpecificRequest{
		JobID: job.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp2 structs.SingleJobResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJob", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != resp.JobModifyIndex {
		t.Fatalf("Bad index: %d %d", resp2.Index, resp.Index)
	}

	// Make a copy of the origin job and change the service name so that we can
	// do a deep equal with the response from the GET JOB Api
	j := job
	j.TaskGroups[0].Tasks[0].Services[0].Name = "web-frontend"
	for tgix, tg := range j.TaskGroups {
		for tidx, t := range tg.Tasks {
			for sidx, service := range t.Services {
				for cidx, check := range service.Checks {
					check.Name = resp2.Job.TaskGroups[tgix].Tasks[tidx].Services[sidx].Checks[cidx].Name
				}
			}
		}
	}

	// Clear the submit times
	j.SubmitTime = 0
	resp2.Job.SubmitTime = 0

	if !reflect.DeepEqual(j, resp2.Job) {
		t.Fatalf("bad: %#v %#v", job, resp2.Job)
	}

	// Lookup non-existing job
	get.JobID = "foobarbaz"
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJob", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != resp.JobModifyIndex {
		t.Fatalf("Bad index: %d %d", resp2.Index, resp.Index)
	}
	if resp2.Job != nil {
		t.Fatalf("unexpected job")
	}
}

func TestJobEndpoint_GetJob_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create the job
	job := mock.Job()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	require.Nil(err)

	// Lookup the job
	get := &structs.JobSpecificRequest{
		JobID: job.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Looking up the job without a token should fail
	var resp structs.SingleJobResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJob", get, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get.AuthToken = invalidToken.SecretID
	var invalidResp structs.SingleJobResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJob", get, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Looking up the job with a management token should succeed
	get.AuthToken = root.SecretID
	var validResp structs.SingleJobResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJob", get, &validResp)
	require.Nil(err)
	require.Equal(job.ID, validResp.Job.ID)

	// Looking up the job with a valid token should succeed
	validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	get.AuthToken = validToken.SecretID
	var validResp2 structs.SingleJobResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJob", get, &validResp2)
	require.Nil(err)
	require.Equal(job.ID, validResp2.Job.ID)
}

func TestJobEndpoint_GetJob_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the jobs
	job1 := mock.Job()
	job2 := mock.Job()

	// Upsert a job we are not interested in first.
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job1); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert another job later which should trigger the watch.
	time.AfterFunc(200*time.Millisecond, func() {
		if err := state.UpsertJob(structs.MsgTypeTestSetup, 200, nil, job2); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.JobSpecificRequest{
		JobID: job2.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     job2.Namespace,
			MinQueryIndex: 150,
		},
	}
	start := time.Now()
	var resp structs.SingleJobResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJob", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if resp.Job == nil || resp.Job.ID != job2.ID {
		t.Fatalf("bad: %#v", resp.Job)
	}

	// Job delete fires watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.DeleteJob(300, job2.Namespace, job2.ID); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 250
	start = time.Now()

	var resp2 structs.SingleJobResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJob", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 300 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 300)
	}
	if resp2.Job != nil {
		t.Fatalf("bad: %#v", resp2.Job)
	}
}

func TestJobEndpoint_GetJobVersions(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.Priority = 88
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register the job again to create another version
	job.Priority = 100
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the job
	get := &structs.JobVersionsRequest{
		JobID: job.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var versionsResp structs.JobVersionsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", get, &versionsResp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if versionsResp.Index != resp.JobModifyIndex {
		t.Fatalf("Bad index: %d %d", versionsResp.Index, resp.Index)
	}

	// Make sure there are two job versions
	versions := versionsResp.Versions
	if l := len(versions); l != 2 {
		t.Fatalf("Got %d versions; want 2", l)
	}

	if v := versions[0]; v.Priority != 100 || v.ID != job.ID || v.Version != 1 {
		t.Fatalf("bad: %+v", v)
	}
	if v := versions[1]; v.Priority != 88 || v.ID != job.ID || v.Version != 0 {
		t.Fatalf("bad: %+v", v)
	}

	// Lookup non-existing job
	get.JobID = "foobarbaz"
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", get, &versionsResp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if versionsResp.Index != resp.JobModifyIndex {
		t.Fatalf("Bad index: %d %d", versionsResp.Index, resp.Index)
	}
	if l := len(versionsResp.Versions); l != 0 {
		t.Fatalf("unexpected versions: %d", l)
	}
}

func TestJobEndpoint_GetJobSubmission(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	t.Cleanup(cleanupS1)

	codec := rpcClient(t, s1)
	testutil.WaitForLeaders(t, s1.RPC)

	// create a job to register and make queries about
	job := mock.Job()
	registerRequest := &structs.JobRegisterRequest{
		Job: job,
		Submission: &structs.JobSubmission{
			Source:        "job \"my-job\" { group \"g1\" {} }",
			Format:        "hcl2",
			VariableFlags: map[string]string{"one": "1"},
			Variables:     "two = 2",
		},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// register the job first ime
	var registerResponse structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", registerRequest, &registerResponse)
	must.NoError(t, err)
	indexV0 := registerResponse.Index

	// register the job a second time, creating another version (v0, v1)
	// with a new job source file and variables
	job.Priority++ // trigger new version
	registerRequest.Submission = &structs.JobSubmission{
		Source:        "job \"my-job\" { group \"g2\" {} }",
		Format:        "hcl2",
		VariableFlags: map[string]string{"three": "3"},
		Variables:     "four = 4",
	}
	err = msgpackrpc.CallWithCodec(codec, "Job.Register", registerRequest, &registerResponse)
	must.NoError(t, err)
	indexV1 := registerResponse.Index

	// lookup the submission for v0
	submissionRequestV0 := &structs.JobSubmissionRequest{
		JobID:   job.ID,
		Version: 0,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var submissionResponse structs.JobSubmissionResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJobSubmission", submissionRequestV0, &submissionResponse)
	must.NoError(t, err)

	sub := submissionResponse.Submission
	must.StrContains(t, sub.Source, "g1")
	must.Eq(t, "hcl2", sub.Format)
	must.Eq(t, map[string]string{"one": "1"}, sub.VariableFlags)
	must.Eq(t, "two = 2", sub.Variables)
	must.Eq(t, job.Namespace, sub.Namespace)
	must.Eq(t, indexV0, sub.JobModifyIndex)

	// lookup the submission for v1
	submissionRequestV1 := &structs.JobSubmissionRequest{
		JobID:   job.ID,
		Version: 1,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var submissionResponseV1 structs.JobSubmissionResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJobSubmission", submissionRequestV1, &submissionResponseV1)
	must.NoError(t, err)

	sub = submissionResponseV1.Submission
	must.StrContains(t, sub.Source, "g2")
	must.Eq(t, "hcl2", sub.Format)
	must.Eq(t, map[string]string{"three": "3"}, sub.VariableFlags)
	must.Eq(t, "four = 4", sub.Variables)
	must.Eq(t, job.Namespace, sub.Namespace)
	must.Eq(t, indexV1, sub.JobModifyIndex)

	// lookup non-existent submission v2
	submissionRequestV2 := &structs.JobSubmissionRequest{
		JobID:   job.ID,
		Version: 2,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var submissionResponseV2 structs.JobSubmissionResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJobSubmission", submissionRequestV2, &submissionResponseV2)
	must.NoError(t, err)
	must.Nil(t, submissionResponseV2.Submission)

	// create a deregister request
	deRegisterRequest := &structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: true, // force gc
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var deRegisterResponse structs.JobDeregisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Deregister", deRegisterRequest, &deRegisterResponse)
	must.NoError(t, err)

	// lookup the submission for v0 again
	submissionRequestV0 = &structs.JobSubmissionRequest{
		JobID:   job.ID,
		Version: 0,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// should no longer exist
	var submissionResponseGone structs.JobSubmissionResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJobSubmission", submissionRequestV0, &submissionResponseGone)
	must.NoError(t, err)
	must.Nil(t, submissionResponseGone.Submission, must.Sprintf("got sub: %#v", submissionResponseGone.Submission))
}

func TestJobEndpoint_GetJobSubmission_ACL(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	t.Cleanup(cleanupS1)

	codec := rpcClient(t, s1)
	testutil.WaitForLeaders(t, s1.RPC)

	// create a namespace to upsert
	namespaceUpsertRequest := &structs.NamespaceUpsertRequest{
		Namespaces: []*structs.Namespace{{
			Name:        "hashicorp",
			Description: "My Namespace",
		}},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: root.SecretID,
		},
	}

	var namespaceUpsertResponse structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "Namespace.UpsertNamespaces", namespaceUpsertRequest, &namespaceUpsertResponse)
	must.NoError(t, err)

	// create a job to register and make queries about
	job := mock.Job()
	job.Namespace = "hashicorp"
	registerRequest := &structs.JobRegisterRequest{
		Job: job,
		Submission: &structs.JobSubmission{
			Source:        "job \"my-job\" { group \"g1\" {} }",
			Format:        "hcl2",
			VariableFlags: map[string]string{"one": "1"},
			Variables:     "two = 2",
		},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
			AuthToken: root.SecretID,
		},
	}

	// register the job
	var registerResponse structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Register", registerRequest, &registerResponse)
	must.NoError(t, err)

	// make a request with no token set
	submissionRequest := &structs.JobSubmissionRequest{
		JobID:   job.ID,
		Version: 0,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var submissionResponse structs.JobSubmissionResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJobSubmission", submissionRequest, &submissionResponse)
	must.ErrorContains(t, err, "Permission denied")

	// make request with token set
	submissionRequest.QueryOptions.AuthToken = root.SecretID
	var submissionResponse2 structs.JobSubmissionResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJobSubmission", submissionRequest, &submissionResponse2)
	must.NoError(t, err)
}

func TestJobEndpoint_GetJobVersions_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create two versions of a job with different priorities
	job := mock.Job()
	job.Priority = 88
	err := state.UpsertJob(structs.MsgTypeTestSetup, 10, nil, job)
	require.Nil(err)

	job.Priority = 100
	err = state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job)
	require.Nil(err)

	// Lookup the job
	get := &structs.JobVersionsRequest{
		JobID: job.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Attempt to fetch without a token should fail
	var resp structs.JobVersionsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", get, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get.AuthToken = invalidToken.SecretID
	var invalidResp structs.JobVersionsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", get, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Expect success for request with a valid management token
	get.AuthToken = root.SecretID
	var validResp structs.JobVersionsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", get, &validResp)
	require.Nil(err)

	// Expect success for request with a valid token
	validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	get.AuthToken = validToken.SecretID
	var validResp2 structs.JobVersionsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", get, &validResp2)
	require.Nil(err)

	// Make sure there are two job versions
	versions := validResp2.Versions
	require.Equal(2, len(versions))
	require.Equal(versions[0].ID, job.ID)
	require.Equal(versions[1].ID, job.ID)
}

func TestJobEndpoint_GetJobVersions_Diff(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.Priority = 88
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register the job again to create another version
	job.Priority = 90
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register the job again to create another version
	job.Priority = 100
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the job
	get := &structs.JobVersionsRequest{
		JobID: job.ID,
		Diffs: true,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var versionsResp structs.JobVersionsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", get, &versionsResp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if versionsResp.Index != resp.JobModifyIndex {
		t.Fatalf("Bad index: %d %d", versionsResp.Index, resp.Index)
	}

	// Make sure there are two job versions
	versions := versionsResp.Versions
	if l := len(versions); l != 3 {
		t.Fatalf("Got %d versions; want 3", l)
	}

	if v := versions[0]; v.Priority != 100 || v.ID != job.ID || v.Version != 2 {
		t.Fatalf("bad: %+v", v)
	}
	if v := versions[1]; v.Priority != 90 || v.ID != job.ID || v.Version != 1 {
		t.Fatalf("bad: %+v", v)
	}
	if v := versions[2]; v.Priority != 88 || v.ID != job.ID || v.Version != 0 {
		t.Fatalf("bad: %+v", v)
	}

	// Ensure we got diffs
	diffs := versionsResp.Diffs
	if l := len(diffs); l != 2 {
		t.Fatalf("Got %d diffs; want 2", l)
	}
	d1 := diffs[0]
	if len(d1.Fields) != 1 {
		t.Fatalf("Got too many diffs: %#v", d1)
	}
	if d1.Fields[0].Name != "Priority" {
		t.Fatalf("Got wrong field: %#v", d1)
	}
	if d1.Fields[0].Old != "90" && d1.Fields[0].New != "100" {
		t.Fatalf("Got wrong field values: %#v", d1)
	}
	d2 := diffs[1]
	if len(d2.Fields) != 1 {
		t.Fatalf("Got too many diffs: %#v", d2)
	}
	if d2.Fields[0].Name != "Priority" {
		t.Fatalf("Got wrong field: %#v", d2)
	}
	if d2.Fields[0].Old != "88" && d1.Fields[0].New != "90" {
		t.Fatalf("Got wrong field values: %#v", d2)
	}
}

func TestJobEndpoint_GetJobVersions_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the jobs
	job1 := mock.Job()
	job2 := mock.Job()
	job3 := mock.Job()
	job3.ID = job2.ID
	job3.Priority = 1

	// Upsert a job we are not interested in first.
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job1); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert another job later which should trigger the watch.
	time.AfterFunc(200*time.Millisecond, func() {
		if err := state.UpsertJob(structs.MsgTypeTestSetup, 200, nil, job2); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.JobVersionsRequest{
		JobID: job2.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     job2.Namespace,
			MinQueryIndex: 150,
		},
	}
	start := time.Now()
	var resp structs.JobVersionsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if len(resp.Versions) != 1 || resp.Versions[0].ID != job2.ID {
		t.Fatalf("bad: %#v", resp.Versions)
	}

	// Upsert the job again which should trigger the watch.
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertJob(structs.MsgTypeTestSetup, 300, nil, job3); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req2 := &structs.JobVersionsRequest{
		JobID: job3.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     job3.Namespace,
			MinQueryIndex: 250,
		},
	}
	var resp2 structs.JobVersionsResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", req2, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp2.Index != 300 {
		t.Fatalf("Bad index: %d %d", resp.Index, 300)
	}
	if len(resp2.Versions) != 2 {
		t.Fatalf("bad: %#v", resp2.Versions)
	}
}

func TestJobEndpoint_GetJobSummary(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})

	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	job.CreateIndex = resp.JobModifyIndex
	job.ModifyIndex = resp.JobModifyIndex
	job.JobModifyIndex = resp.JobModifyIndex

	// Lookup the job summary
	get := &structs.JobSummaryRequest{
		JobID: job.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp2 structs.JobSummaryResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Summary", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != resp.JobModifyIndex {
		t.Fatalf("Bad index: %d %d", resp2.Index, resp.Index)
	}

	expectedJobSummary := structs.JobSummary{
		JobID:     job.ID,
		Namespace: job.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {},
		},
		Children:    new(structs.JobChildrenSummary),
		CreateIndex: job.CreateIndex,
		ModifyIndex: job.CreateIndex,
	}

	if !reflect.DeepEqual(resp2.JobSummary, &expectedJobSummary) {
		t.Fatalf("expected: %v, actual: %v", expectedJobSummary, resp2.JobSummary)
	}
}

func TestJobEndpoint_Summary_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the job
	job := mock.Job()
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	reg.AuthToken = root.SecretID

	var err error

	// Register the job with a valid token
	var regResp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &regResp)
	require.Nil(err)

	job.CreateIndex = regResp.JobModifyIndex
	job.ModifyIndex = regResp.JobModifyIndex
	job.JobModifyIndex = regResp.JobModifyIndex

	req := &structs.JobSummaryRequest{
		JobID: job.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Expect failure for request without a token
	var resp structs.JobSummaryResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Summary", req, &resp)
	require.NotNil(err)

	expectedJobSummary := &structs.JobSummary{
		JobID:     job.ID,
		Namespace: job.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {},
		},
		Children:    new(structs.JobChildrenSummary),
		CreateIndex: job.CreateIndex,
		ModifyIndex: job.ModifyIndex,
	}

	// Expect success when using a management token
	req.AuthToken = root.SecretID
	var mgmtResp structs.JobSummaryResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Summary", req, &mgmtResp)
	require.Nil(err)
	require.Equal(expectedJobSummary, mgmtResp.JobSummary)

	// Create the namespace policy and tokens
	state := s1.fsm.State()

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	req.AuthToken = invalidToken.SecretID
	var invalidResp structs.JobSummaryResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Summary", req, &invalidResp)
	require.NotNil(err)

	// Try with a valid token
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	req.AuthToken = validToken.SecretID
	var authResp structs.JobSummaryResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Summary", req, &authResp)
	require.Nil(err)
	require.Equal(expectedJobSummary, authResp.JobSummary)
}

func TestJobEndpoint_GetJobSummary_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create a job and insert it
	job1 := mock.Job()
	time.AfterFunc(200*time.Millisecond, func() {
		if err := state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job1); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Ensure the job summary request gets fired
	req := &structs.JobSummaryRequest{
		JobID: job1.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     job1.Namespace,
			MinQueryIndex: 50,
		},
	}
	var resp structs.JobSummaryResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Job.Summary", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}

	// Upsert an allocation for the job which should trigger the watch.
	time.AfterFunc(200*time.Millisecond, func() {
		alloc := mock.Alloc()
		alloc.JobID = job1.ID
		alloc.Job = job1
		if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 200, []*structs.Allocation{alloc}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
	req = &structs.JobSummaryRequest{
		JobID: job1.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     job1.Namespace,
			MinQueryIndex: 199,
		},
	}
	start = time.Now()
	var resp1 structs.JobSummaryResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Summary", req, &resp1); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp1.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if resp1.JobSummary == nil {
		t.Fatalf("bad: %#v", resp)
	}

	// Job delete fires watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.DeleteJob(300, job1.Namespace, job1.ID); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 250
	start = time.Now()

	var resp2 structs.SingleJobResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Summary", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 300 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 300)
	}
	if resp2.Job != nil {
		t.Fatalf("bad: %#v", resp2.Job)
	}
}

func TestJobEndpoint_ListJobs(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	state := s1.fsm.State()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	require.NoError(t, err)

	// Lookup the jobs
	get := &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp2 structs.JobListResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.List", get, &resp2)
	require.NoError(t, err)
	require.Equal(t, uint64(1000), resp2.Index)
	require.Len(t, resp2.Jobs, 1)
	require.Equal(t, job.ID, resp2.Jobs[0].ID)
	require.Equal(t, job.Namespace, resp2.Jobs[0].Namespace)
	require.Nil(t, resp2.Jobs[0].Meta)

	// Lookup the jobs by prefix
	get = &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
			Prefix:    resp2.Jobs[0].ID[:4],
		},
	}
	var resp3 structs.JobListResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.List", get, &resp3)
	require.NoError(t, err)
	require.Equal(t, uint64(1000), resp3.Index)
	require.Len(t, resp3.Jobs, 1)
	require.Equal(t, job.ID, resp3.Jobs[0].ID)
	require.Equal(t, job.Namespace, resp3.Jobs[0].Namespace)

	// Lookup jobs with a meta parameter
	get = &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
			Prefix:    resp2.Jobs[0].ID[:4],
		},
		Fields: &structs.JobStubFields{
			Meta: true,
		},
	}
	var resp4 structs.JobListResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.List", get, &resp4)
	require.NoError(t, err)
	require.Equal(t, job.Meta["owner"], resp4.Jobs[0].Meta["owner"])
}

// TestJobEndpoint_ListJobs_AllNamespaces_OSS asserts that server
// returns all jobs across namespace.
func TestJobEndpoint_ListJobs_AllNamespaces_OSS(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	state := s1.fsm.State()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the jobs
	get := &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: "*",
		},
	}
	var resp2 structs.JobListResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.List", get, &resp2)
	require.NoError(t, err)
	require.Equal(t, uint64(1000), resp2.Index)
	require.Len(t, resp2.Jobs, 1)
	require.Equal(t, job.ID, resp2.Jobs[0].ID)
	require.Equal(t, structs.DefaultNamespace, resp2.Jobs[0].Namespace)

	// Lookup the jobs by prefix
	get = &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: "*",
			Prefix:    resp2.Jobs[0].ID[:4],
		},
	}
	var resp3 structs.JobListResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.List", get, &resp3)
	require.NoError(t, err)
	require.Equal(t, uint64(1000), resp3.Index)
	require.Len(t, resp3.Jobs, 1)
	require.Equal(t, job.ID, resp3.Jobs[0].ID)
	require.Equal(t, structs.DefaultNamespace, resp2.Jobs[0].Namespace)

	// Lookup the jobs by prefix
	get = &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: "*",
			Prefix:    "z" + resp2.Jobs[0].ID[:4],
		},
	}
	var resp4 structs.JobListResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.List", get, &resp4)
	require.NoError(t, err)
	require.Equal(t, uint64(1000), resp4.Index)
	require.Empty(t, resp4.Jobs)
}

func TestJobEndpoint_ListJobs_WithACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	var err error

	// Create the register request
	job := mock.Job()
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	require.Nil(err)

	req := &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Expect failure for request without a token
	var resp structs.JobListResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.List", req, &resp)
	require.NotNil(err)

	// Expect success for request with a management token
	var mgmtResp structs.JobListResponse
	req.AuthToken = root.SecretID
	err = msgpackrpc.CallWithCodec(codec, "Job.List", req, &mgmtResp)
	require.Nil(err)
	require.Equal(1, len(mgmtResp.Jobs))
	require.Equal(job.ID, mgmtResp.Jobs[0].ID)

	// Expect failure for request with a token that has incorrect permissions
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	req.AuthToken = invalidToken.SecretID
	var invalidResp structs.JobListResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.List", req, &invalidResp)
	require.NotNil(err)

	// Try with a valid token with correct permissions
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
	var validResp structs.JobListResponse
	req.AuthToken = validToken.SecretID

	err = msgpackrpc.CallWithCodec(codec, "Job.List", req, &validResp)
	require.Nil(err)
	require.Equal(1, len(validResp.Jobs))
	require.Equal(job.ID, validResp.Jobs[0].ID)
}

func TestJobEndpoint_ListJobs_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the job
	job := mock.Job()

	// Upsert job triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     job.Namespace,
			MinQueryIndex: 50,
		},
	}
	start := time.Now()
	var resp structs.JobListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 100 {
		t.Fatalf("Bad index: %d %d", resp.Index, 100)
	}
	if len(resp.Jobs) != 1 || resp.Jobs[0].ID != job.ID {
		t.Fatalf("bad: %#v", resp)
	}

	// Job deletion triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.DeleteJob(200, job.Namespace, job.ID); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.MinQueryIndex = 150
	start = time.Now()
	var resp2 structs.JobListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.List", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 200)
	}
	if len(resp2.Jobs) != 0 {
		t.Fatalf("bad: %#v", resp2)
	}
}

func TestJobEndpoint_ListJobs_PaginationFiltering(t *testing.T) {
	ci.Parallel(t)
	s1, _, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// create a set of jobs. these are in the order that the state store will
	// return them from the iterator (sorted by key) for ease of writing tests
	mocks := []struct {
		name      string
		namespace string
		status    string
	}{
		{name: "job-01"}, // 0
		{name: "job-02"}, // 1
		{name: "job-03", namespace: "non-default"}, // 2
		{name: "job-04"}, // 3
		{name: "job-05", status: structs.JobStatusRunning}, // 4
		{name: "job-06", status: structs.JobStatusRunning}, // 5
		{},                                   // 6, missing job
		{name: "job-08"},                     // 7
		{name: "job-03", namespace: "other"}, // 8, same name but in another namespace
	}

	state := s1.fsm.State()
	require.NoError(t, state.UpsertNamespaces(999, []*structs.Namespace{{Name: "non-default"}, {Name: "other"}}))

	for i, m := range mocks {
		if m.name == "" {
			continue
		}

		index := 1000 + uint64(i)
		job := mock.Job()
		job.ID = m.name
		job.Name = m.name
		job.Status = m.status
		if m.namespace != "" { // defaults to "default"
			job.Namespace = m.namespace
		}
		job.CreateIndex = index
		require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, index, nil, job))
	}

	aclToken := mock.CreatePolicyAndToken(t, state, 1100, "test-valid-read",
		mock.NamespacePolicy("*", "read", nil)).
		SecretID

	cases := []struct {
		name              string
		namespace         string
		prefix            string
		filter            string
		nextToken         string
		pageSize          int32
		expectedNextToken string
		expectedIDs       []string
		expectedError     string
	}{
		{
			name:              "test01 size-2 page-1 default NS",
			pageSize:          2,
			expectedNextToken: "default.job-04",
			expectedIDs:       []string{"job-01", "job-02"},
		},
		{
			name:              "test02 size-2 page-1 default NS with prefix",
			prefix:            "job",
			pageSize:          2,
			expectedNextToken: "default.job-04",
			expectedIDs:       []string{"job-01", "job-02"},
		},
		{
			name:              "test03 size-2 page-2 default NS",
			pageSize:          2,
			nextToken:         "default.job-04",
			expectedNextToken: "default.job-06",
			expectedIDs:       []string{"job-04", "job-05"},
		},
		{
			name:              "test04 size-2 page-2 default NS with prefix",
			prefix:            "job",
			pageSize:          2,
			nextToken:         "default.job-04",
			expectedNextToken: "default.job-06",
			expectedIDs:       []string{"job-04", "job-05"},
		},
		{
			name:        "test05 no valid results with filters and prefix",
			prefix:      "not-job",
			pageSize:    2,
			nextToken:   "",
			expectedIDs: []string{},
		},
		{
			name:        "test06 go-bexpr filter",
			namespace:   "*",
			filter:      `Name matches "job-0[123]"`,
			expectedIDs: []string{"job-01", "job-02", "job-03", "job-03"},
		},
		{
			name:              "test07 go-bexpr filter with pagination",
			namespace:         "*",
			filter:            `Name matches "job-0[123]"`,
			pageSize:          2,
			expectedNextToken: "non-default.job-03",
			expectedIDs:       []string{"job-01", "job-02"},
		},
		{
			name:        "test08 go-bexpr filter in namespace",
			namespace:   "non-default",
			filter:      `Status == "pending"`,
			expectedIDs: []string{"job-03"},
		},
		{
			name:          "test09 go-bexpr invalid expression",
			filter:        `NotValid`,
			expectedError: "failed to read filter expression",
		},
		{
			name:          "test10 go-bexpr invalid field",
			filter:        `InvalidField == "value"`,
			expectedError: "error finding value in datum",
		},
		{
			name:      "test11 missing index",
			pageSize:  1,
			nextToken: "default.job-07",
			expectedIDs: []string{
				"job-08",
			},
		},
		{
			name:              "test12 same name but different NS",
			namespace:         "*",
			pageSize:          1,
			filter:            `Name == "job-03"`,
			expectedNextToken: "other.job-03",
			expectedIDs: []string{
				"job-03",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.JobListRequest{
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					Namespace: tc.namespace,
					Prefix:    tc.prefix,
					Filter:    tc.filter,
					PerPage:   tc.pageSize,
					NextToken: tc.nextToken,
				},
			}
			req.AuthToken = aclToken
			var resp structs.JobListResponse
			err := msgpackrpc.CallWithCodec(codec, "Job.List", req, &resp)
			if tc.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
				return
			}

			gotIDs := []string{}
			for _, job := range resp.Jobs {
				gotIDs = append(gotIDs, job.ID)
			}
			require.Equal(t, tc.expectedIDs, gotIDs, "unexpected page of jobs")
			require.Equal(t, tc.expectedNextToken, resp.QueryMeta.NextToken, "unexpected NextToken")
		})
	}
}

func TestJobEndpoint_Allocations(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc2.JobID = alloc1.JobID
	state := s1.fsm.State()
	state.UpsertJobSummary(998, mock.JobSummary(alloc1.JobID))
	state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID))
	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: alloc1.JobID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: alloc1.Job.Namespace,
		},
	}
	var resp2 structs.JobAllocationsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Allocations", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 1000)
	}

	if len(resp2.Allocations) != 2 {
		t.Fatalf("bad: %#v", resp2.Allocations)
	}
}

func TestJobEndpoint_Allocations_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create allocations for a job
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc2.JobID = alloc1.JobID
	state.UpsertJobSummary(998, mock.JobSummary(alloc1.JobID))
	state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID))
	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1, alloc2})
	require.Nil(err)

	// Look up allocations for that job
	get := &structs.JobSpecificRequest{
		JobID: alloc1.JobID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: alloc1.Job.Namespace,
		},
	}

	// Attempt to fetch the response without a token should fail
	var resp structs.JobAllocationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Allocations", get, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with an invalid token should fail
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get.AuthToken = invalidToken.SecretID
	var invalidResp structs.JobAllocationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Allocations", get, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with valid management token should succeed
	get.AuthToken = root.SecretID
	var validResp structs.JobAllocationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Allocations", get, &validResp)
	require.Nil(err)

	// Attempt to fetch the response with valid management token should succeed
	validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	get.AuthToken = validToken.SecretID
	var validResp2 structs.JobAllocationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Allocations", get, &validResp2)
	require.Nil(err)

	require.Equal(2, len(validResp2.Allocations))
}

func TestJobEndpoint_Allocations_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc2.JobID = "job1"
	state := s1.fsm.State()

	// First upsert an unrelated alloc
	time.AfterFunc(100*time.Millisecond, func() {
		state.UpsertJobSummary(99, mock.JobSummary(alloc1.JobID))
		err := state.UpsertAllocs(structs.MsgTypeTestSetup, 100, []*structs.Allocation{alloc1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert an alloc for the job we are interested in later
	time.AfterFunc(200*time.Millisecond, func() {
		state.UpsertJobSummary(199, mock.JobSummary(alloc2.JobID))
		err := state.UpsertAllocs(structs.MsgTypeTestSetup, 200, []*structs.Allocation{alloc2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: "job1",
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     alloc1.Job.Namespace,
			MinQueryIndex: 150,
		},
	}
	var resp structs.JobAllocationsResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Job.Allocations", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if len(resp.Allocations) != 1 || resp.Allocations[0].JobID != "job1" {
		t.Fatalf("bad: %#v", resp.Allocations)
	}
}

// TestJobEndpoint_Allocations_NoJobID asserts not setting a JobID in the
// request returns an error.
func TestJobEndpoint_Allocations_NoJobID(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	get := &structs.JobSpecificRequest{
		JobID: "",
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}

	var resp structs.JobAllocationsResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Allocations", get, &resp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing job ID")
}

func TestJobEndpoint_Evaluations(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	eval1 := mock.Eval()
	eval2 := mock.Eval()
	eval2.JobID = eval1.JobID
	state := s1.fsm.State()
	err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1, eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: eval1.JobID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: eval1.Namespace,
		},
	}
	var resp2 structs.JobEvaluationsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Evaluations", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 1000)
	}

	if len(resp2.Evaluations) != 2 {
		t.Fatalf("bad: %#v", resp2.Evaluations)
	}
}

func TestJobEndpoint_Evaluations_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create evaluations for the same job
	eval1 := mock.Eval()
	eval2 := mock.Eval()
	eval2.JobID = eval1.JobID
	err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1, eval2})
	require.Nil(err)

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: eval1.JobID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: eval1.Namespace,
		},
	}

	// Attempt to fetch without providing a token
	var resp structs.JobEvaluationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluations", get, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get.AuthToken = invalidToken.SecretID
	var invalidResp structs.JobEvaluationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluations", get, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch with valid management token should succeed
	get.AuthToken = root.SecretID
	var validResp structs.JobEvaluationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluations", get, &validResp)
	require.Nil(err)
	require.Equal(2, len(validResp.Evaluations))

	// Attempt to fetch with valid token should succeed
	validToken := mock.CreatePolicyAndToken(t, state, 1003, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	get.AuthToken = validToken.SecretID
	var validResp2 structs.JobEvaluationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluations", get, &validResp2)
	require.Nil(err)
	require.Equal(2, len(validResp2.Evaluations))
}

func TestJobEndpoint_Evaluations_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	eval1 := mock.Eval()
	eval2 := mock.Eval()
	eval2.JobID = "job1"
	state := s1.fsm.State()

	// First upsert an unrelated eval
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertEvals(structs.MsgTypeTestSetup, 100, []*structs.Evaluation{eval1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert an eval for the job we are interested in later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertEvals(structs.MsgTypeTestSetup, 200, []*structs.Evaluation{eval2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: "job1",
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     eval1.Namespace,
			MinQueryIndex: 150,
		},
	}
	var resp structs.JobEvaluationsResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Job.Evaluations", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if len(resp.Evaluations) != 1 || resp.Evaluations[0].JobID != "job1" {
		t.Fatalf("bad: %#v", resp.Evaluations)
	}
}

func TestJobEndpoint_Deployments(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()
	require := require.New(t)

	// Create the register request
	j := mock.Job()
	d1 := mock.Deployment()
	d2 := mock.Deployment()
	d1.JobID = j.ID
	d2.JobID = j.ID
	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, j), "UpsertJob")
	d1.JobCreateIndex = j.CreateIndex
	d2.JobCreateIndex = j.CreateIndex

	require.Nil(state.UpsertDeployment(1001, d1), "UpsertDeployment")
	require.Nil(state.UpsertDeployment(1002, d2), "UpsertDeployment")

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: j.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: j.Namespace,
		},
	}
	var resp structs.DeploymentListResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Deployments", get, &resp), "RPC")
	require.EqualValues(1002, resp.Index, "response index")
	require.Len(resp.Deployments, 2, "deployments for job")
}

func TestJobEndpoint_Deployments_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create a job and corresponding deployments
	j := mock.Job()
	d1 := mock.Deployment()
	d2 := mock.Deployment()
	d1.JobID = j.ID
	d2.JobID = j.ID
	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, j), "UpsertJob")
	d1.JobCreateIndex = j.CreateIndex
	d2.JobCreateIndex = j.CreateIndex
	require.Nil(state.UpsertDeployment(1001, d1), "UpsertDeployment")
	require.Nil(state.UpsertDeployment(1002, d2), "UpsertDeployment")

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: j.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: j.Namespace,
		},
	}
	// Lookup with no token should fail
	var resp structs.DeploymentListResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Deployments", get, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get.AuthToken = invalidToken.SecretID
	var invalidResp structs.DeploymentListResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Deployments", get, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Lookup with valid management token should succeed
	get.AuthToken = root.SecretID
	var validResp structs.DeploymentListResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Deployments", get, &validResp), "RPC")
	require.EqualValues(1002, validResp.Index, "response index")
	require.Len(validResp.Deployments, 2, "deployments for job")

	// Lookup with valid token should succeed
	validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	get.AuthToken = validToken.SecretID
	var validResp2 structs.DeploymentListResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Deployments", get, &validResp2), "RPC")
	require.EqualValues(1002, validResp2.Index, "response index")
	require.Len(validResp2.Deployments, 2, "deployments for job")
}

func TestJobEndpoint_Deployments_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()
	require := require.New(t)

	// Create the register request
	j := mock.Job()
	d1 := mock.Deployment()
	d2 := mock.Deployment()
	d2.JobID = j.ID
	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 50, nil, j), "UpsertJob")
	d2.JobCreateIndex = j.CreateIndex
	// First upsert an unrelated eval
	time.AfterFunc(100*time.Millisecond, func() {
		require.Nil(state.UpsertDeployment(100, d1), "UpsertDeployment")
	})

	// Upsert an eval for the job we are interested in later
	time.AfterFunc(200*time.Millisecond, func() {
		require.Nil(state.UpsertDeployment(200, d2), "UpsertDeployment")
	})

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: d2.JobID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     d2.Namespace,
			MinQueryIndex: 150,
		},
	}
	var resp structs.DeploymentListResponse
	start := time.Now()
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Deployments", get, &resp), "RPC")
	require.EqualValues(200, resp.Index, "response index")
	require.Len(resp.Deployments, 1, "deployments for job")
	require.Equal(d2.ID, resp.Deployments[0].ID, "returned deployment")
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
}

func TestJobEndpoint_LatestDeployment(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()
	require := require.New(t)

	// Create the register request
	j := mock.Job()
	d1 := mock.Deployment()
	d2 := mock.Deployment()
	d1.JobID = j.ID
	d2.JobID = j.ID
	d2.CreateIndex = d1.CreateIndex + 100
	d2.ModifyIndex = d2.CreateIndex + 100
	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, j), "UpsertJob")
	d1.JobCreateIndex = j.CreateIndex
	d2.JobCreateIndex = j.CreateIndex
	require.Nil(state.UpsertDeployment(1001, d1), "UpsertDeployment")
	require.Nil(state.UpsertDeployment(1002, d2), "UpsertDeployment")

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: j.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: j.Namespace,
		},
	}
	var resp structs.SingleDeploymentResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.LatestDeployment", get, &resp), "RPC")
	require.EqualValues(1002, resp.Index, "response index")
	require.NotNil(resp.Deployment, "want a deployment")
	require.Equal(d2.ID, resp.Deployment.ID, "latest deployment for job")
}

func TestJobEndpoint_LatestDeployment_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create a job and deployments
	j := mock.Job()
	d1 := mock.Deployment()
	d2 := mock.Deployment()
	d1.JobID = j.ID
	d2.JobID = j.ID
	d2.CreateIndex = d1.CreateIndex + 100
	d2.ModifyIndex = d2.CreateIndex + 100
	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, j), "UpsertJob")
	d1.JobCreateIndex = j.CreateIndex
	d2.JobCreateIndex = j.CreateIndex
	require.Nil(state.UpsertDeployment(1001, d1), "UpsertDeployment")
	require.Nil(state.UpsertDeployment(1002, d2), "UpsertDeployment")

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: j.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: j.Namespace,
		},
	}

	// Attempt to fetch the response without a token should fail
	var resp structs.SingleDeploymentResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.LatestDeployment", get, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with an invalid token should fail
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get.AuthToken = invalidToken.SecretID
	var invalidResp structs.SingleDeploymentResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.LatestDeployment", get, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Fetching latest deployment with a valid management token should succeed
	get.AuthToken = root.SecretID
	var validResp structs.SingleDeploymentResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.LatestDeployment", get, &validResp), "RPC")
	require.EqualValues(1002, validResp.Index, "response index")
	require.NotNil(validResp.Deployment, "want a deployment")
	require.Equal(d2.ID, validResp.Deployment.ID, "latest deployment for job")

	// Fetching latest deployment with a valid token should succeed
	validToken := mock.CreatePolicyAndToken(t, state, 1004, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	get.AuthToken = validToken.SecretID
	var validResp2 structs.SingleDeploymentResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.LatestDeployment", get, &validResp2), "RPC")
	require.EqualValues(1002, validResp2.Index, "response index")
	require.NotNil(validResp2.Deployment, "want a deployment")
	require.Equal(d2.ID, validResp2.Deployment.ID, "latest deployment for job")
}

func TestJobEndpoint_LatestDeployment_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()
	require := require.New(t)

	// Create the register request
	j := mock.Job()
	d1 := mock.Deployment()
	d2 := mock.Deployment()
	d2.JobID = j.ID
	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 50, nil, j), "UpsertJob")
	d2.JobCreateIndex = j.CreateIndex

	// First upsert an unrelated eval
	time.AfterFunc(100*time.Millisecond, func() {
		require.Nil(state.UpsertDeployment(100, d1), "UpsertDeployment")
	})

	// Upsert an eval for the job we are interested in later
	time.AfterFunc(200*time.Millisecond, func() {
		require.Nil(state.UpsertDeployment(200, d2), "UpsertDeployment")
	})

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: d2.JobID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     d2.Namespace,
			MinQueryIndex: 150,
		},
	}
	var resp structs.SingleDeploymentResponse
	start := time.Now()
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.LatestDeployment", get, &resp), "RPC")
	require.EqualValues(200, resp.Index, "response index")
	require.NotNil(resp.Deployment, "deployment for job")
	require.Equal(d2.ID, resp.Deployment.ID, "returned deployment")
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
}

func TestJob_GetActions(t *testing.T) {
	ci.Parallel(t)

	testServer, testServerCleanup := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer testServerCleanup()
	codec := rpcClient(t, testServer)
	testutil.WaitForLeader(t, testServer.RPC)

	// Perform a request which does not include the jobID.
	jobActionsReq1 := structs.JobActionListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			Namespace: structs.DefaultNamespace,
		},
	}

	var jobActionsResp1 structs.JobActionListResponse
	must.ErrorContains(t,
		msgpackrpc.CallWithCodec(codec, structs.JobGetActionsRPCMethod, &jobActionsReq1, &jobActionsResp1),
		"JobID required for actions")

	// Perform a request which specifies a jobID which is not present within
	// Nomad's state.
	jobActionsReq2 := structs.JobActionListRequest{
		JobID: "not-found",
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			Namespace: structs.DefaultNamespace,
		},
	}

	var jobActionsResp2 structs.JobActionListResponse
	must.ErrorContains(t,
		msgpackrpc.CallWithCodec(codec, structs.JobGetActionsRPCMethod, &jobActionsReq2, &jobActionsResp2),
		"Unknown job")

	// Upsert a job which contains some actions.
	mockJob := mock.Job()
	must.NoError(t, testServer.fsm.State().UpsertJob(structs.MsgTypeTestSetup, 4, nil, mockJob))

	// Read the job actions.
	jobActionsReq3 := structs.JobActionListRequest{
		JobID: mockJob.ID,
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			Namespace: mockJob.Namespace,
		},
	}

	var jobActionsResp3 structs.JobActionListResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, structs.JobGetActionsRPCMethod, &jobActionsReq3, &jobActionsResp3))
	must.Len(t, 2, jobActionsResp3.Actions)
	must.Eq(t, 4, jobActionsResp3.Index)

	// Create a namespace, place a job within this, and read out the job
	// actions.
	mockNamespace := mock.Namespace()
	must.NoError(t, testServer.fsm.State().UpsertNamespaces(22, []*structs.Namespace{mockNamespace}))

	mockJobNonDefault := mock.Job()
	mockJobNonDefault.Namespace = mockNamespace.Name
	must.NoError(t, testServer.fsm.State().UpsertJob(structs.MsgTypeTestSetup, 30, nil, mockJobNonDefault))

	jobActionsReq4 := structs.JobActionListRequest{
		JobID: mockJobNonDefault.ID,
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			Namespace: mockNamespace.Name,
		},
	}

	var jobActionsResp4 structs.JobActionListResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, structs.JobGetActionsRPCMethod, &jobActionsReq4, &jobActionsResp4))
	must.Len(t, 2, jobActionsResp4.Actions)
	must.Eq(t, 30, jobActionsResp4.Index)
}

func TestJob_GetActions_ACL(t *testing.T) {
	ci.Parallel(t)

	testServer, rootACLToken, testServerCleanup := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer testServerCleanup()
	codec := rpcClient(t, testServer)
	testutil.WaitForLeader(t, testServer.RPC)

	// Perform a request which does not include the jobID.
	jobActionsReq1 := structs.JobActionListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			Namespace: structs.DefaultNamespace,
			AuthToken: rootACLToken.SecretID,
		},
	}

	var jobActionsResp1 structs.JobActionListResponse
	must.ErrorContains(t,
		msgpackrpc.CallWithCodec(codec, structs.JobGetActionsRPCMethod, &jobActionsReq1, &jobActionsResp1),
		"JobID required for actions")

	// Perform a request which does not include an authentication token.
	jobActionsReq2 := structs.JobActionListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			Namespace: structs.DefaultNamespace,
		},
	}

	var jobActionsResp2 structs.JobActionListResponse
	must.ErrorContains(t,
		msgpackrpc.CallWithCodec(codec, structs.JobGetActionsRPCMethod, &jobActionsReq2, &jobActionsResp2),
		structs.ErrPermissionDenied.Error())

	// Perform a request which specifies a jobID which is not present within
	// Nomad's state.
	jobActionsReq3 := structs.JobActionListRequest{
		JobID: "not-found",
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			Namespace: structs.DefaultNamespace,
			AuthToken: rootACLToken.SecretID,
		},
	}

	var jobActionsResp3 structs.JobActionListResponse
	must.ErrorContains(t,
		msgpackrpc.CallWithCodec(codec, structs.JobGetActionsRPCMethod, &jobActionsReq3, &jobActionsResp3),
		structs.ErrUnknownJobPrefix)

	// Upsert a job which contains some actions.
	mockJob := mock.Job()
	must.NoError(t, testServer.fsm.State().UpsertJob(structs.MsgTypeTestSetup, 4, nil, mockJob))

	// Read the job actions using the root ACL token.
	jobActionsReq4 := structs.JobActionListRequest{
		JobID: mockJob.ID,
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			Namespace: mockJob.Namespace,
			AuthToken: rootACLToken.SecretID,
		},
	}

	var jobActionsResp4 structs.JobActionListResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, structs.JobGetActionsRPCMethod, &jobActionsReq4, &jobActionsResp4))
	must.Len(t, 2, jobActionsResp4.Actions)
	must.Eq(t, 4, jobActionsResp4.Index)

	// Create a namespace and place a job within this.
	mockNamespace := mock.Namespace()
	must.NoError(t, testServer.fsm.State().UpsertNamespaces(22, []*structs.Namespace{mockNamespace}))

	mockJobNonDefault := mock.Job()
	mockJobNonDefault.Namespace = mockNamespace.Name
	must.NoError(t, testServer.fsm.State().UpsertJob(structs.MsgTypeTestSetup, 30, nil, mockJobNonDefault))

	// Create a scoped policy and use this to look-up the job action within the
	// non-default namespace.
	namespaceScopedToken := mock.CreatePolicyAndToken(
		t, testServer.State(), 40, "test-job-actions-get",
		mock.NamespacePolicy(mockJobNonDefault.Namespace, "", []string{acl.NamespaceCapabilityReadJob}))

	jobActionsReq5 := structs.JobActionListRequest{
		JobID: mockJobNonDefault.ID,
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			Namespace: mockNamespace.Name,
			AuthToken: namespaceScopedToken.SecretID,
		},
	}

	var jobActionsResp5 structs.JobActionListResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, structs.JobGetActionsRPCMethod, &jobActionsReq5, &jobActionsResp5))
	must.Len(t, 2, jobActionsResp5.Actions)
	must.Eq(t, 30, jobActionsResp5.Index)

	// Attempt to use the namespace scoped token, to read across into the
	// default namespace.
	jobActionsReq6 := structs.JobActionListRequest{
		JobID: mockJob.ID,
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			Namespace: mockJob.Name,
			AuthToken: namespaceScopedToken.SecretID,
		},
	}

	var jobActionsResp6 structs.JobActionListResponse
	must.ErrorContains(t,
		msgpackrpc.CallWithCodec(codec, structs.JobGetActionsRPCMethod, &jobActionsReq6, &jobActionsResp6),
		structs.ErrPermissionDenied.Error())
}

func TestJobEndpoint_Plan_ACL(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create a plan request
	job := mock.Job()
	planReq := &structs.JobPlanRequest{
		Job:  job,
		Diff: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Try without a token, expect failure
	var planResp structs.JobPlanResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Plan", planReq, &planResp); err == nil {
		t.Fatalf("expected error")
	}

	// Try with a token
	planReq.AuthToken = root.SecretID
	if err := msgpackrpc.CallWithCodec(codec, "Job.Plan", planReq, &planResp); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestJobEndpoint_Plan_WithDiff(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Create a plan request
	planReq := &structs.JobPlanRequest{
		Job:  job,
		Diff: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var planResp structs.JobPlanResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Plan", planReq, &planResp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the response
	if planResp.JobModifyIndex == 0 {
		t.Fatalf("bad cas: %d", planResp.JobModifyIndex)
	}
	if planResp.Annotations == nil {
		t.Fatalf("no annotations")
	}
	if planResp.Diff == nil {
		t.Fatalf("no diff")
	}
	if len(planResp.FailedTGAllocs) == 0 {
		t.Fatalf("no failed task group alloc metrics")
	}
}

func TestJobEndpoint_Plan_NoDiff(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Create a plan request
	planReq := &structs.JobPlanRequest{
		Job:  job,
		Diff: false,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var planResp structs.JobPlanResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Plan", planReq, &planResp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the response
	if planResp.JobModifyIndex == 0 {
		t.Fatalf("bad cas: %d", planResp.JobModifyIndex)
	}
	if planResp.Annotations == nil {
		t.Fatalf("no annotations")
	}
	if planResp.Diff != nil {
		t.Fatalf("got diff")
	}
	if len(planResp.FailedTGAllocs) == 0 {
		t.Fatalf("no failed task group alloc metrics")
	}
}

// TestJobEndpoint_Plan_Scaling asserts that the plan endpoint handles
// jobs with scaling block
func TestJobEndpoint_Plan_Scaling(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create a plan request
	job := mock.Job()
	tg := job.TaskGroups[0]
	tg.Tasks[0].Resources.MemoryMB = 999999999
	scaling := &structs.ScalingPolicy{Min: 1, Max: 100, Type: structs.ScalingPolicyTypeHorizontal}
	scaling.Canonicalize(job, tg, nil)
	tg.Scaling = scaling
	planReq := &structs.JobPlanRequest{
		Job:  job,
		Diff: false,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Try without a token, expect failure
	var planResp structs.JobPlanResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Plan", planReq, &planResp)
	require.NoError(t, err)

	require.NotEmpty(t, planResp.FailedTGAllocs)
	require.Contains(t, planResp.FailedTGAllocs, tg.Name)
}

func TestJobEndpoint_ImplicitConstraints_Vault(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Enable vault
	tr, f := true, false
	s1.config.GetDefaultVault().Enabled = &tr
	s1.config.GetDefaultVault().AllowUnauthenticated = &f

	// Replace the Vault Client on the server
	tvc := &TestVaultClient{}
	s1.vault = tvc

	policy := "foo"
	goodToken := uuid.Generate()
	goodPolicies := []string{"foo", "bar", "baz"}
	tvc.SetLookupTokenAllowedPolicies(goodToken, goodPolicies)

	// Create the register request with a job asking for a vault policy
	job := mock.Job()
	job.VaultToken = goodToken
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:   []string{policy},
		ChangeMode: structs.VaultChangeModeRestart,
	}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check for the job in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.CreateIndex != resp.JobModifyIndex {
		t.Fatalf("index mis-match")
	}

	// Check that there is an implicit Vault and Consul constraint.
	require.Len(t, out.TaskGroups[0].Constraints, 2)
	require.ElementsMatch(t, out.TaskGroups[0].Constraints, []*structs.Constraint{
		consulServiceDiscoveryConstraint, vaultConstraint,
	})
}

func TestJobEndpoint_ValidateJob_ConsulConnect(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	validateJob := func(j *structs.Job) error {
		req := &structs.JobRegisterRequest{
			Job: j,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: j.Namespace,
			},
		}
		var resp structs.JobValidateResponse
		if err := msgpackrpc.CallWithCodec(codec, "Job.Validate", req, &resp); err != nil {
			return err
		}

		if resp.Error != "" {
			return errors.New(resp.Error)
		}

		if len(resp.ValidationErrors) != 0 {
			return errors.New(strings.Join(resp.ValidationErrors, ","))
		}

		if resp.Warnings != "" {
			return errors.New(resp.Warnings)
		}

		return nil
	}

	tgServices := []*structs.Service{
		{
			Name:      "count-api",
			PortLabel: "9001",
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{},
			},
		},
	}

	t.Run("plain job", func(t *testing.T) {
		j := mock.Job()
		require.NoError(t, validateJob(j))
	})
	t.Run("valid consul connect", func(t *testing.T) {
		j := mock.Job()

		tg := j.TaskGroups[0]
		tg.Services = tgServices
		tg.Networks[0].Mode = "bridge"

		err := validateJob(j)
		require.NoError(t, err)
	})

	t.Run("consul connect but missing network", func(t *testing.T) {
		j := mock.Job()

		tg := j.TaskGroups[0]
		tg.Services = tgServices
		tg.Networks = nil

		err := validateJob(j)
		require.Error(t, err)
		require.Contains(t, err.Error(), `Consul Connect sidecars require exactly 1 network`)
	})

	t.Run("consul connect but non bridge network", func(t *testing.T) {
		j := mock.Job()

		tg := j.TaskGroups[0]
		tg.Services = tgServices

		tg.Networks = structs.Networks{
			{Mode: "host"},
		}

		err := validateJob(j)
		require.Error(t, err)
		require.Contains(t, err.Error(), `Consul Connect sidecar requires bridge network, found "host" in group "web"`)
	})

}

func TestJobEndpoint_ImplicitConstraints_Signals(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request with a job asking for a template that sends a
	// signal
	job := mock.Job()
	signal1 := "SIGUSR1"
	signal2 := "SIGHUP"
	job.TaskGroups[0].Tasks[0].Templates = []*structs.Template{
		{
			SourcePath:   "foo",
			DestPath:     "bar",
			ChangeMode:   structs.TemplateChangeModeSignal,
			ChangeSignal: signal1,
		},
		{
			SourcePath:   "foo",
			DestPath:     "baz",
			ChangeMode:   structs.TemplateChangeModeSignal,
			ChangeSignal: signal2,
		},
	}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check for the job in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.CreateIndex != resp.JobModifyIndex {
		t.Fatalf("index mis-match")
	}

	// Check that there is an implicit signal and Consul constraint.
	require.Len(t, out.TaskGroups[0].Constraints, 2)
	require.ElementsMatch(t, out.TaskGroups[0].Constraints, []*structs.Constraint{
		getSignalConstraint([]string{signal1, signal2}), consulServiceDiscoveryConstraint},
	)
}

func TestJobEndpoint_ValidateJobUpdate(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	old := mock.Job()
	new := mock.Job()

	if err := validateJobUpdate(old, new); err != nil {
		t.Errorf("expected update to be valid but got: %v", err)
	}

	new.Type = "batch"
	if err := validateJobUpdate(old, new); err == nil {
		t.Errorf("expected err when setting new job to a different type")
	} else {
		t.Log(err)
	}

	new = mock.Job()
	new.Periodic = &structs.PeriodicConfig{Enabled: true}
	if err := validateJobUpdate(old, new); err == nil {
		t.Errorf("expected err when setting new job to periodic")
	} else {
		t.Log(err)
	}

	new = mock.Job()
	new.ParameterizedJob = &structs.ParameterizedJobConfig{}
	if err := validateJobUpdate(old, new); err == nil {
		t.Errorf("expected err when setting new job to parameterized")
	} else {
		t.Log(err)
	}

	new = mock.Job()
	new.Dispatched = true
	require.Error(validateJobUpdate(old, new),
		"expected err when setting new job to dispatched")
	require.Error(validateJobUpdate(nil, new),
		"expected err when setting new job to dispatched")
	require.Error(validateJobUpdate(new, old),
		"expected err when setting dispatched to false")
	require.NoError(validateJobUpdate(nil, old))
}

func TestJobEndpoint_ValidateJobUpdate_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	job := mock.Job()

	req := &structs.JobValidateRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Attempt to update without providing a valid token
	var resp structs.JobValidateResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Validate", req, &resp)
	require.NotNil(err)

	// Update with a valid token
	req.AuthToken = root.SecretID
	var validResp structs.JobValidateResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Validate", req, &validResp)
	require.Nil(err)

	require.Equal("", validResp.Error)
	require.Equal("", validResp.Warnings)
}

func TestJobEndpoint_ValidateJob_PriorityNotOk(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	validateJob := func(j *structs.Job) error {
		req := &structs.JobRegisterRequest{
			Job: j,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: j.Namespace,
			},
		}
		var resp structs.JobValidateResponse
		if err := msgpackrpc.CallWithCodec(codec, "Job.Validate", req, &resp); err != nil {
			return err
		}

		if resp.Error != "" {
			return errors.New(resp.Error)
		}

		if len(resp.ValidationErrors) != 0 {
			return errors.New(strings.Join(resp.ValidationErrors, ","))
		}

		if resp.Warnings != "" {
			return errors.New(resp.Warnings)
		}

		return nil
	}

	t.Run("job with invalid min priority", func(t *testing.T) {
		j := mock.Job()
		j.Priority = -1

		err := validateJob(j)
		must.Error(t, err)
		must.ErrorContains(t, err, "job priority must be between")
	})

	t.Run("job with invalid max priority", func(t *testing.T) {
		j := mock.Job()
		j.Priority = 101

		err := validateJob(j)
		must.Error(t, err)
		must.ErrorContains(t, err, "job priority must be between")
	})
}

func TestJobEndpoint_Dispatch_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create a parameterized job
	job := mock.BatchJob()
	job.ParameterizedJob = &structs.ParameterizedJobConfig{}
	err := state.UpsertJob(structs.MsgTypeTestSetup, 400, nil, job)
	require.Nil(err)

	req := &structs.JobDispatchRequest{
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Attempt to fetch the response without a token should fail
	var resp structs.JobDispatchResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Dispatch", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with an invalid token should fail
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
	req.AuthToken = invalidToken.SecretID

	var invalidResp structs.JobDispatchResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Dispatch", req, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Dispatch with a valid management token should succeed
	req.AuthToken = root.SecretID

	var validResp structs.JobDispatchResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Dispatch", req, &validResp)
	require.Nil(err)
	require.NotNil(validResp.EvalID)
	require.NotNil(validResp.DispatchedJobID)
	require.NotEqual(validResp.DispatchedJobID, "")

	// Dispatch with a valid token should succeed
	validToken := mock.CreatePolicyAndToken(t, state, 1003, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityDispatchJob}))
	req.AuthToken = validToken.SecretID

	var validResp2 structs.JobDispatchResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Dispatch", req, &validResp2)
	require.Nil(err)
	require.NotNil(validResp2.EvalID)
	require.NotNil(validResp2.DispatchedJobID)
	require.NotEqual(validResp2.DispatchedJobID, "")

	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, validResp2.DispatchedJobID)
	require.Nil(err)
	require.NotNil(out)
	require.Equal(out.ParentID, job.ID)

	// Look up the evaluation
	eval, err := state.EvalByID(ws, validResp2.EvalID)
	require.Nil(err)
	require.NotNil(eval)
	require.Equal(eval.CreateIndex, validResp2.EvalCreateIndex)
}

func TestJobEndpoint_Dispatch(t *testing.T) {
	ci.Parallel(t)

	// No requirements
	d1 := mock.BatchJob()
	d1.ParameterizedJob = &structs.ParameterizedJobConfig{}

	// Require input data
	d2 := mock.BatchJob()
	d2.ParameterizedJob = &structs.ParameterizedJobConfig{
		Payload: structs.DispatchPayloadRequired,
	}

	// Disallow input data
	d3 := mock.BatchJob()
	d3.ParameterizedJob = &structs.ParameterizedJobConfig{
		Payload: structs.DispatchPayloadForbidden,
	}

	// Require meta
	d4 := mock.BatchJob()
	d4.ParameterizedJob = &structs.ParameterizedJobConfig{
		MetaRequired: []string{"foo", "bar"},
	}

	// Optional meta
	d5 := mock.BatchJob()
	d5.ParameterizedJob = &structs.ParameterizedJobConfig{
		MetaOptional: []string{"foo", "bar"},
	}

	// Periodic dispatch job
	d6 := mock.PeriodicJob()
	d6.ParameterizedJob = &structs.ParameterizedJobConfig{}

	d7 := mock.BatchJob()
	d7.ParameterizedJob = &structs.ParameterizedJobConfig{}
	d7.Stop = true

	reqNoInputNoMeta := &structs.JobDispatchRequest{}
	reqInputDataNoMeta := &structs.JobDispatchRequest{
		Payload: []byte("hello world"),
	}
	reqNoInputDataMeta := &structs.JobDispatchRequest{
		Meta: map[string]string{
			"foo": "f1",
			"bar": "f2",
		},
	}
	reqInputDataMeta := &structs.JobDispatchRequest{
		Payload: []byte("hello world"),
		Meta: map[string]string{
			"foo": "f1",
			"bar": "f2",
		},
	}
	reqBadMeta := &structs.JobDispatchRequest{
		Payload: []byte("hello world"),
		Meta: map[string]string{
			"foo": "f1",
			"bar": "f2",
			"baz": "f3",
		},
	}
	reqInputDataTooLarge := &structs.JobDispatchRequest{
		Payload: make([]byte, DispatchPayloadSizeLimit+100),
	}

	type existingIdempotentChildJob struct {
		isTerminal bool
	}

	type testCase struct {
		name                  string
		parameterizedJob      *structs.Job
		dispatchReq           *structs.JobDispatchRequest
		noEval                bool
		err                   bool
		errStr                string
		idempotencyToken      string
		existingIdempotentJob *existingIdempotentChildJob
	}
	cases := []testCase{
		{
			name:             "optional input data w/ data",
			parameterizedJob: d1,
			dispatchReq:      reqInputDataNoMeta,
			err:              false,
		},
		{
			name:             "optional input data w/o data",
			parameterizedJob: d1,
			dispatchReq:      reqNoInputNoMeta,
			err:              false,
		},
		{
			name:             "require input data w/ data",
			parameterizedJob: d2,
			dispatchReq:      reqInputDataNoMeta,
			err:              false,
		},
		{
			name:             "require input data w/o data",
			parameterizedJob: d2,
			dispatchReq:      reqNoInputNoMeta,
			err:              true,
			errStr:           "not provided but required",
		},
		{
			name:             "disallow input data w/o data",
			parameterizedJob: d3,
			dispatchReq:      reqNoInputNoMeta,
			err:              false,
		},
		{
			name:             "disallow input data w/ data",
			parameterizedJob: d3,
			dispatchReq:      reqInputDataNoMeta,
			err:              true,
			errStr:           "provided but forbidden",
		},
		{
			name:             "require meta w/ meta",
			parameterizedJob: d4,
			dispatchReq:      reqInputDataMeta,
			err:              false,
		},
		{
			name:             "require meta w/o meta",
			parameterizedJob: d4,
			dispatchReq:      reqNoInputNoMeta,
			err:              true,
			errStr:           "did not provide required meta keys",
		},
		{
			name:             "optional meta w/ meta",
			parameterizedJob: d5,
			dispatchReq:      reqNoInputDataMeta,
			err:              false,
		},
		{
			name:             "optional meta w/o meta",
			parameterizedJob: d5,
			dispatchReq:      reqNoInputNoMeta,
			err:              false,
		},
		{
			name:             "optional meta w/ bad meta",
			parameterizedJob: d5,
			dispatchReq:      reqBadMeta,
			err:              true,
			errStr:           "unpermitted metadata keys",
		},
		{
			name:             "optional input w/ too big of input",
			parameterizedJob: d1,
			dispatchReq:      reqInputDataTooLarge,
			err:              true,
			errStr:           "Payload exceeds maximum size",
		},
		{
			name:             "periodic job dispatched, ensure no eval",
			parameterizedJob: d6,
			dispatchReq:      reqNoInputNoMeta,
			noEval:           true,
		},
		{
			name:             "periodic job stopped, ensure error",
			parameterizedJob: d7,
			dispatchReq:      reqNoInputNoMeta,
			err:              true,
			errStr:           "stopped",
		},
		{
			name:                  "idempotency token, no existing child job",
			parameterizedJob:      d1,
			dispatchReq:           reqInputDataNoMeta,
			err:                   false,
			idempotencyToken:      "foo",
			existingIdempotentJob: nil,
		},
		{
			name:             "idempotency token, w/ existing non-terminal child job",
			parameterizedJob: d1,
			dispatchReq:      reqInputDataNoMeta,
			err:              false,
			idempotencyToken: "foo",
			existingIdempotentJob: &existingIdempotentChildJob{
				isTerminal: false,
			},
			noEval: true,
		},
		{
			name:             "idempotency token, w/ existing terminal job",
			parameterizedJob: d1,
			dispatchReq:      reqInputDataNoMeta,
			err:              false,
			idempotencyToken: "foo",
			existingIdempotentJob: &existingIdempotentChildJob{
				isTerminal: true,
			},
			noEval: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s1, cleanupS1 := TestServer(t, func(c *Config) {
				c.NumSchedulers = 0 // Prevent automatic dequeue
			})
			defer cleanupS1()
			codec := rpcClient(t, s1)
			testutil.WaitForLeader(t, s1.RPC)

			// Create the register request
			regReq := &structs.JobRegisterRequest{
				Job: tc.parameterizedJob,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					Namespace: tc.parameterizedJob.Namespace,
				},
			}

			// Fetch the response
			var regResp structs.JobRegisterResponse
			if err := msgpackrpc.CallWithCodec(codec, "Job.Register", regReq, &regResp); err != nil {
				t.Fatalf("err: %v", err)
			}

			// Now try to dispatch
			tc.dispatchReq.JobID = tc.parameterizedJob.ID
			tc.dispatchReq.WriteRequest = structs.WriteRequest{
				Region:           "global",
				Namespace:        tc.parameterizedJob.Namespace,
				IdempotencyToken: tc.idempotencyToken,
			}

			// Dispatch with the same request so a child job w/ the idempotency key exists
			var initialIdempotentDispatchResp structs.JobDispatchResponse
			if tc.existingIdempotentJob != nil {
				if err := msgpackrpc.CallWithCodec(codec, "Job.Dispatch", tc.dispatchReq, &initialIdempotentDispatchResp); err != nil {
					t.Fatalf("Unexpected error dispatching initial idempotent job: %v", err)
				}

				if tc.existingIdempotentJob.isTerminal {
					eval, err := s1.State().EvalByID(nil, initialIdempotentDispatchResp.EvalID)
					if err != nil {
						t.Fatalf("Unexpected error fetching eval %v", err)
					}
					eval = eval.Copy()
					eval.Status = structs.EvalStatusComplete
					err = s1.State().UpsertEvals(structs.MsgTypeTestSetup, initialIdempotentDispatchResp.Index+1, []*structs.Evaluation{eval})
					if err != nil {
						t.Fatalf("Unexpected error completing eval %v", err)
					}
				}
			}

			var dispatchResp structs.JobDispatchResponse
			dispatchErr := msgpackrpc.CallWithCodec(codec, "Job.Dispatch", tc.dispatchReq, &dispatchResp)

			if dispatchErr == nil {
				if tc.err {
					t.Fatalf("Expected error: %v", dispatchErr)
				}

				// Check that we got an eval and job id back
				switch dispatchResp.EvalID {
				case "":
					if !tc.noEval {
						t.Fatalf("Bad response")
					}
				default:
					if tc.noEval {
						t.Fatalf("Got eval %q", dispatchResp.EvalID)
					}
				}

				if dispatchResp.DispatchedJobID == "" {
					t.Fatalf("Bad response")
				}

				state := s1.fsm.State()
				ws := memdb.NewWatchSet()
				out, err := state.JobByID(ws, tc.parameterizedJob.Namespace, dispatchResp.DispatchedJobID)
				if err != nil {
					t.Fatalf("err: %v", err)
				}
				if out == nil {
					t.Fatalf("expected job")
				}
				if out.CreateIndex != dispatchResp.JobCreateIndex {
					t.Fatalf("index mis-match")
				}
				if out.ParentID != tc.parameterizedJob.ID {
					t.Fatalf("bad parent ID")
				}
				if !out.Dispatched {
					t.Fatal("expected dispatched job")
				}
				if out.IsParameterized() {
					t.Fatal("dispatched job should not be parameterized")
				}
				if out.ParameterizedJob == nil {
					t.Fatal("parameter job config should exist")
				}

				// Check that the existing job is returned in the case of a supplied idempotency token
				if tc.idempotencyToken != "" && tc.existingIdempotentJob != nil {
					if dispatchResp.DispatchedJobID != initialIdempotentDispatchResp.DispatchedJobID {
						t.Fatal("dispatched job id should match initial dispatch")
					}

					if dispatchResp.JobCreateIndex != initialIdempotentDispatchResp.JobCreateIndex {
						t.Fatal("dispatched job create index should match initial dispatch")
					}
				}

				if tc.noEval {
					return
				}

				// Lookup the evaluation
				eval, err := state.EvalByID(ws, dispatchResp.EvalID)
				if err != nil {
					t.Fatalf("err: %v", err)
				}

				if eval == nil {
					t.Fatalf("expected eval")
				}
				if eval.CreateIndex != dispatchResp.EvalCreateIndex {
					t.Fatalf("index mis-match")
				}
			} else {
				if !tc.err {
					t.Fatalf("Got unexpected error: %v", dispatchErr)
				} else if !strings.Contains(dispatchErr.Error(), tc.errStr) {
					t.Fatalf("Expected err to include %q; got %v", tc.errStr, dispatchErr)
				}
			}
		})
	}
}

// TestJobEndpoint_Dispatch_JobChildrenSummary asserts that the job summary is updated
// appropriately as its dispatched/children jobs status are updated.
func TestJobEndpoint_Dispatch_JobChildrenSummary(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()

	state := s1.fsm.State()

	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	node := mock.Node()
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1, node))

	parameterizedJob := mock.BatchJob()
	parameterizedJob.ParameterizedJob = &structs.ParameterizedJobConfig{}

	// Create the register request
	regReq := &structs.JobRegisterRequest{
		Job: parameterizedJob,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: parameterizedJob.Namespace,
		},
	}
	var regResp structs.JobRegisterResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", regReq, &regResp))

	jobChildren := func() *structs.JobChildrenSummary {
		summary, err := state.JobSummaryByID(nil, parameterizedJob.Namespace, parameterizedJob.ID)
		require.NoError(t, err)

		return summary.Children
	}
	require.Equal(t, &structs.JobChildrenSummary{}, jobChildren())

	// dispatch a child job
	dispatchReq := &structs.JobDispatchRequest{
		JobID: parameterizedJob.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: parameterizedJob.Namespace,
		},
	}
	var dispatchResp structs.JobDispatchResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Dispatch", dispatchReq, &dispatchResp)
	require.NoError(t, err)

	nextIdx := dispatchResp.Index + 1

	require.Equal(t, &structs.JobChildrenSummary{Pending: 1}, jobChildren())

	dispatchedJob, err := state.JobByID(nil, parameterizedJob.Namespace, dispatchResp.DispatchedJobID)
	require.NoError(t, err)
	require.NotNil(t, dispatchedJob)

	dispatchedStatus := func() string {
		job, err := state.JobByID(nil, dispatchedJob.Namespace, dispatchedJob.ID)
		require.NoError(t, err)
		require.NotNil(t, job)

		return job.Status
	}

	// Let's start an alloc for the dispatch job and walk through states
	// Note that job summary reports 1 running even when alloc is pending!
	nextIdx++
	alloc := mock.Alloc()
	alloc.Job = dispatchedJob
	alloc.JobID = dispatchedJob.ID
	alloc.TaskGroup = dispatchedJob.TaskGroups[0].Name
	alloc.Namespace = dispatchedJob.Namespace
	alloc.ClientStatus = structs.AllocClientStatusPending
	err = s1.State().UpsertAllocs(structs.MsgTypeTestSetup, nextIdx, []*structs.Allocation{alloc})
	require.NoError(t, err)
	require.Equal(t, &structs.JobChildrenSummary{Running: 1}, jobChildren())
	require.Equal(t, structs.JobStatusRunning, dispatchedStatus())

	// mark the creation eval as completed
	nextIdx++
	eval, err := state.EvalByID(nil, dispatchResp.EvalID)
	require.NoError(t, err)
	eval = eval.Copy()
	eval.Status = structs.EvalStatusComplete
	require.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, nextIdx, []*structs.Evaluation{eval}))

	updateAllocStatus := func(status string) {
		nextIdx++
		nalloc, err := state.AllocByID(nil, alloc.ID)
		require.NoError(t, err)
		nalloc = nalloc.Copy()
		nalloc.ClientStatus = status
		err = s1.State().UpdateAllocsFromClient(structs.MsgTypeTestSetup, nextIdx, []*structs.Allocation{nalloc})
		require.NoError(t, err)
	}

	// job should remain remaining when alloc runs
	updateAllocStatus(structs.AllocClientStatusRunning)
	require.Equal(t, &structs.JobChildrenSummary{Running: 1}, jobChildren())
	require.Equal(t, structs.JobStatusRunning, dispatchedStatus())

	// job should be dead after alloc completes
	updateAllocStatus(structs.AllocClientStatusComplete)
	require.Equal(t, &structs.JobChildrenSummary{Dead: 1}, jobChildren())
	require.Equal(t, structs.JobStatusDead, dispatchedStatus())
}

func TestJobEndpoint_Dispatch_ACL_RejectedBySchedulerConfig(t *testing.T) {
	ci.Parallel(t)
	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	job := mock.BatchJob()
	job.ParameterizedJob = &structs.ParameterizedJobConfig{}

	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	require.NoError(t, err)

	dispatch := &structs.JobDispatchRequest{
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	submitJobToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid-write",
		mock.NamespacePolicy(structs.DefaultNamespace, "write", nil)).
		SecretID

	cases := []struct {
		name          string
		token         string
		rejectEnabled bool
		errExpected   string
	}{
		{
			name:          "reject disabled, with a submit token",
			token:         submitJobToken,
			rejectEnabled: false,
		},
		{
			name:          "reject enabled, with a submit token",
			token:         submitJobToken,
			rejectEnabled: true,
			errExpected:   structs.ErrJobRegistrationDisabled.Error(),
		},
		{
			name:          "reject enabled, without a token",
			token:         "",
			rejectEnabled: true,
			errExpected:   structs.ErrPermissionDenied.Error(),
		},
		{
			name:          "reject enabled, with a management token",
			token:         root.SecretID,
			rejectEnabled: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			cfgReq := &structs.SchedulerSetConfigRequest{
				Config: structs.SchedulerConfiguration{
					RejectJobRegistration: tc.rejectEnabled,
				},
				WriteRequest: structs.WriteRequest{
					Region: "global",
				},
			}
			cfgReq.AuthToken = root.SecretID
			err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerSetConfiguration",
				cfgReq, &structs.SchedulerSetConfigurationResponse{},
			)
			require.NoError(t, err)

			dispatch.AuthToken = tc.token
			var resp structs.JobDispatchResponse
			err = msgpackrpc.CallWithCodec(codec, "Job.Dispatch", dispatch, &resp)

			if tc.errExpected != "" {
				require.Error(t, err, "expected error")
				require.EqualError(t, err, tc.errExpected)
			} else {
				require.NoError(t, err, "unexpected error")
				require.NotEqual(t, 0, resp.Index)
			}
		})
	}

}

func TestJobEndpoint_Scale(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	job := mock.Job()
	originalCount := job.TaskGroups[0].Count
	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	require.Nil(err)

	groupName := job.TaskGroups[0].Name
	scale := &structs.JobScaleRequest{
		JobID: job.ID,
		Target: map[string]string{
			structs.ScalingTargetGroup: groupName,
		},
		Count:   pointer.Of(int64(originalCount + 1)),
		Message: "because of the load",
		Meta: map[string]interface{}{
			"metrics": map[string]string{
				"1": "a",
				"2": "b",
			},
			"other": "value",
		},
		PolicyOverride: false,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Scale", scale, &resp)
	require.NoError(err)
	require.NotEmpty(resp.EvalID)
	require.Greater(resp.EvalCreateIndex, resp.JobModifyIndex)

	events, _, _ := state.ScalingEventsByJob(nil, job.Namespace, job.ID)
	require.Equal(1, len(events[groupName]))
	require.Equal(int64(originalCount), events[groupName][0].PreviousCount)
}

func TestJobEndpoint_Scale_DeploymentBlocking(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	type testCase struct {
		latestDeploymentStatus string
	}
	cases := []string{
		structs.DeploymentStatusSuccessful,
		structs.DeploymentStatusPaused,
		structs.DeploymentStatusRunning,
	}

	for _, tc := range cases {
		// create a job with a deployment history
		job := mock.Job()
		require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job), "UpsertJob")
		d1 := mock.Deployment()
		d1.Status = structs.DeploymentStatusCancelled
		d1.StatusDescription = structs.DeploymentStatusDescriptionNewerJob
		d1.JobID = job.ID
		d1.JobCreateIndex = job.CreateIndex
		require.Nil(state.UpsertDeployment(1001, d1), "UpsertDeployment")
		d2 := mock.Deployment()
		d2.Status = structs.DeploymentStatusSuccessful
		d2.StatusDescription = structs.DeploymentStatusDescriptionSuccessful
		d2.JobID = job.ID
		d2.JobCreateIndex = job.CreateIndex
		require.Nil(state.UpsertDeployment(1002, d2), "UpsertDeployment")

		// add the latest deployment for the test case
		dLatest := mock.Deployment()
		dLatest.Status = tc
		dLatest.StatusDescription = "description does not matter for this test"
		dLatest.JobID = job.ID
		dLatest.JobCreateIndex = job.CreateIndex
		require.Nil(state.UpsertDeployment(1003, dLatest), "UpsertDeployment")

		// attempt to scale
		originalCount := job.TaskGroups[0].Count
		newCount := int64(originalCount + 1)
		groupName := job.TaskGroups[0].Name
		scalingMetadata := map[string]interface{}{
			"meta": "data",
		}
		scalingMessage := "original reason for scaling"
		scale := &structs.JobScaleRequest{
			JobID: job.ID,
			Target: map[string]string{
				structs.ScalingTargetGroup: groupName,
			},
			Meta:    scalingMetadata,
			Message: scalingMessage,
			Count:   pointer.Of(newCount),
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: job.Namespace,
			},
		}
		var resp structs.JobRegisterResponse
		err := msgpackrpc.CallWithCodec(codec, "Job.Scale", scale, &resp)
		if dLatest.Active() {
			// should fail
			require.Error(err, "test case %q", tc)
			require.Contains(err.Error(), "active deployment")
		} else {
			require.NoError(err, "test case %q", tc)
			require.NotEmpty(resp.EvalID)
			require.Greater(resp.EvalCreateIndex, resp.JobModifyIndex)
		}
	}
}

func TestJobEndpoint_ScaleEnforceIndex(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	store := s1.fsm.State()

	job := mock.Job()
	originalCount := job.TaskGroups[0].Count
	err := store.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	must.NoError(t, err)

	groupName := job.TaskGroups[0].Name
	scale := &structs.JobScaleRequest{
		JobID: job.ID,
		Target: map[string]string{
			structs.ScalingTargetGroup: groupName,
		},
		Count:   pointer.Of(int64(originalCount + 1)),
		Message: "because of the load",
		Meta: map[string]interface{}{
			"metrics": map[string]string{
				"1": "a",
				"2": "b",
			},
			"other": "value",
		},
		PolicyOverride: false,
		EnforceIndex:   true,
		JobModifyIndex: 1000000,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Scale", scale, &resp)
	must.EqError(t, err,
		"Enforcing job modify index 1000000: job exists with conflicting job modify index: 1000")

	events, _, _ := store.ScalingEventsByJob(nil, job.Namespace, job.ID)
	must.Len(t, 0, events[groupName])
}

func TestJobEndpoint_Scale_InformationalEventsShouldNotBeBlocked(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	type testCase struct {
		latestDeploymentStatus string
	}
	cases := []string{
		structs.DeploymentStatusSuccessful,
		structs.DeploymentStatusPaused,
		structs.DeploymentStatusRunning,
	}

	for _, tc := range cases {
		// create a job with a deployment history
		job := mock.Job()
		require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job), "UpsertJob")
		d1 := mock.Deployment()
		d1.Status = structs.DeploymentStatusCancelled
		d1.StatusDescription = structs.DeploymentStatusDescriptionNewerJob
		d1.JobID = job.ID
		d1.JobCreateIndex = job.CreateIndex
		require.Nil(state.UpsertDeployment(1001, d1), "UpsertDeployment")
		d2 := mock.Deployment()
		d2.Status = structs.DeploymentStatusSuccessful
		d2.StatusDescription = structs.DeploymentStatusDescriptionSuccessful
		d2.JobID = job.ID
		d2.JobCreateIndex = job.CreateIndex
		require.Nil(state.UpsertDeployment(1002, d2), "UpsertDeployment")

		// add the latest deployment for the test case
		dLatest := mock.Deployment()
		dLatest.Status = tc
		dLatest.StatusDescription = "description does not matter for this test"
		dLatest.JobID = job.ID
		dLatest.JobCreateIndex = job.CreateIndex
		require.Nil(state.UpsertDeployment(1003, dLatest), "UpsertDeployment")

		// register informational scaling event
		groupName := job.TaskGroups[0].Name
		scalingMetadata := map[string]interface{}{
			"meta": "data",
		}
		scalingMessage := "original reason for scaling"
		scale := &structs.JobScaleRequest{
			JobID: job.ID,
			Target: map[string]string{
				structs.ScalingTargetGroup: groupName,
			},
			Meta:    scalingMetadata,
			Message: scalingMessage,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: job.Namespace,
			},
		}
		var resp structs.JobRegisterResponse
		err := msgpackrpc.CallWithCodec(codec, "Job.Scale", scale, &resp)
		require.NoError(err, "test case %q", tc)
		require.Empty(resp.EvalID)

		events, _, _ := state.ScalingEventsByJob(nil, job.Namespace, job.ID)
		require.Equal(1, len(events[groupName]))
		latestEvent := events[groupName][0]
		require.False(latestEvent.Error)
		require.Nil(latestEvent.Count)
		require.Equal(scalingMessage, latestEvent.Message)
		require.Equal(scalingMetadata, latestEvent.Meta)
	}
}

func TestJobEndpoint_Scale_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	job := mock.Job()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	require.Nil(err)

	scale := &structs.JobScaleRequest{
		JobID: job.ID,
		Target: map[string]string{
			structs.ScalingTargetGroup: job.TaskGroups[0].Name,
		},
		Message: "because of the load",
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Scale without a token should fail
	var resp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Scale", scale, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
	scale.AuthToken = invalidToken.SecretID
	var invalidResp structs.JobRegisterResponse
	require.NotNil(err)
	err = msgpackrpc.CallWithCodec(codec, "Job.Scale", scale, &invalidResp)
	require.Contains(err.Error(), "Permission denied")

	type testCase struct {
		authToken string
		name      string
	}
	cases := []testCase{
		{
			name:      "mgmt token should succeed",
			authToken: root.SecretID,
		},
		{
			name: "write disposition should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-write",
				mock.NamespacePolicy(structs.DefaultNamespace, "write", nil)).
				SecretID,
		},
		{
			name: "autoscaler disposition should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-autoscaler",
				mock.NamespacePolicy(structs.DefaultNamespace, "scale", nil)).
				SecretID,
		},
		{
			name: "submit-job capability should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-submit-job",
				mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob})).SecretID,
		},
		{
			name: "scale-job capability should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-scale-job",
				mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityScaleJob})).
				SecretID,
		},
	}

	for _, tc := range cases {
		scale.AuthToken = tc.authToken
		var resp structs.JobRegisterResponse
		err = msgpackrpc.CallWithCodec(codec, "Job.Scale", scale, &resp)
		require.NoError(err, tc.name)
		require.NotNil(resp.EvalID)
	}

}

func TestJobEndpoint_Scale_ACL_RejectedBySchedulerConfig(t *testing.T) {
	ci.Parallel(t)
	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	job := mock.Job()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	require.NoError(t, err)

	scale := &structs.JobScaleRequest{
		JobID: job.ID,
		Target: map[string]string{
			structs.ScalingTargetGroup: job.TaskGroups[0].Name,
		},
		Message: "because of the load",
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	submitJobToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid-write",
		mock.NamespacePolicy(structs.DefaultNamespace, "write", nil)).
		SecretID

	cases := []struct {
		name          string
		token         string
		rejectEnabled bool
		errExpected   string
	}{
		{
			name:          "reject disabled, with a submit token",
			token:         submitJobToken,
			rejectEnabled: false,
		},
		{
			name:          "reject enabled, with a submit token",
			token:         submitJobToken,
			rejectEnabled: true,
			errExpected:   structs.ErrJobRegistrationDisabled.Error(),
		},
		{
			name:          "reject enabled, without a token",
			token:         "",
			rejectEnabled: true,
			errExpected:   structs.ErrPermissionDenied.Error(),
		},
		{
			name:          "reject enabled, with a management token",
			token:         root.SecretID,
			rejectEnabled: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			cfgReq := &structs.SchedulerSetConfigRequest{
				Config: structs.SchedulerConfiguration{
					RejectJobRegistration: tc.rejectEnabled,
				},
				WriteRequest: structs.WriteRequest{
					Region: "global",
				},
			}
			cfgReq.AuthToken = root.SecretID
			err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerSetConfiguration",
				cfgReq, &structs.SchedulerSetConfigurationResponse{},
			)
			require.NoError(t, err)

			var resp structs.JobRegisterResponse
			scale.AuthToken = tc.token
			err = msgpackrpc.CallWithCodec(codec, "Job.Scale", scale, &resp)

			if tc.errExpected != "" {
				require.Error(t, err, "expected error")
				require.EqualError(t, err, tc.errExpected)
			} else {
				require.NoError(t, err, "unexpected error")
				require.NotEqual(t, 0, resp.Index)
			}
		})
	}

}

func TestJobEndpoint_Scale_Invalid(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	job := mock.Job()
	count := job.TaskGroups[0].Count

	// check before job registration
	scale := &structs.JobScaleRequest{
		JobID: job.ID,
		Target: map[string]string{
			structs.ScalingTargetGroup: job.TaskGroups[0].Name,
		},
		Count:   pointer.Of(int64(count) + 1),
		Message: "this should fail",
		Meta: map[string]interface{}{
			"metrics": map[string]string{
				"1": "a",
				"2": "b",
			},
			"other": "value",
		},
		PolicyOverride: false,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Scale", scale, &resp)
	require.Error(err)
	require.Contains(err.Error(), "not found")

	// register the job
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	require.Nil(err)

	scale.Count = pointer.Of(int64(10))
	scale.Message = "error message"
	scale.Error = true
	err = msgpackrpc.CallWithCodec(codec, "Job.Scale", scale, &resp)
	require.Error(err)
	require.Contains(err.Error(), "should not contain count if error is true")
}

func TestJobEndpoint_Scale_OutOfBounds(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	job, pol := mock.JobWithScalingPolicy()
	pol.Min = 3
	pol.Max = 10
	job.TaskGroups[0].Count = 5

	// register the job
	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	require.Nil(err)

	var resp structs.JobRegisterResponse
	scale := &structs.JobScaleRequest{
		JobID: job.ID,
		Target: map[string]string{
			structs.ScalingTargetGroup: job.TaskGroups[0].Name,
		},
		Count:          pointer.Of(pol.Max + 1),
		Message:        "out of bounds",
		PolicyOverride: false,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	err = msgpackrpc.CallWithCodec(codec, "Job.Scale", scale, &resp)
	require.Error(err)
	require.Contains(err.Error(), "group count was greater than scaling policy maximum: 11 > 10")

	scale.Count = pointer.Of(int64(2))
	err = msgpackrpc.CallWithCodec(codec, "Job.Scale", scale, &resp)
	require.Error(err)
	require.Contains(err.Error(), "group count was less than scaling policy minimum: 2 < 3")
}

func TestJobEndpoint_Scale_NoEval(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	job := mock.Job()
	groupName := job.TaskGroups[0].Name
	originalCount := job.TaskGroups[0].Count
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}, &resp)
	jobCreateIndex := resp.Index
	require.NoError(err)

	scale := &structs.JobScaleRequest{
		JobID: job.ID,
		Target: map[string]string{
			structs.ScalingTargetGroup: groupName,
		},
		Count:   nil, // no count => no eval
		Message: "something informative",
		Meta: map[string]interface{}{
			"metrics": map[string]string{
				"1": "a",
				"2": "b",
			},
			"other": "value",
		},
		PolicyOverride: false,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	err = msgpackrpc.CallWithCodec(codec, "Job.Scale", scale, &resp)
	require.NoError(err)
	require.Empty(resp.EvalID)
	require.Empty(resp.EvalCreateIndex)

	jobEvents, eventsIndex, err := state.ScalingEventsByJob(nil, job.Namespace, job.ID)
	require.NoError(err)
	require.NotNil(jobEvents)
	require.Len(jobEvents, 1)
	require.Contains(jobEvents, groupName)
	groupEvents := jobEvents[groupName]
	require.Len(groupEvents, 1)
	event := groupEvents[0]
	require.Nil(event.EvalID)
	require.Greater(eventsIndex, jobCreateIndex)

	events, _, _ := state.ScalingEventsByJob(nil, job.Namespace, job.ID)
	require.Equal(1, len(events[groupName]))
	require.Equal(int64(originalCount), events[groupName][0].PreviousCount)
}

func TestJobEndpoint_Scale_Priority(t *testing.T) {
	ci.Parallel(t)
	requireAssertion := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	fsmState := s1.fsm.State()

	// Create a job and alter the priority.
	job := mock.Job()
	job.Priority = 90
	originalCount := job.TaskGroups[0].Count
	err := fsmState.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	requireAssertion.Nil(err)

	groupName := job.TaskGroups[0].Name
	scale := &structs.JobScaleRequest{
		JobID: job.ID,
		Target: map[string]string{
			structs.ScalingTargetGroup: groupName,
		},
		Count:          pointer.Of(int64(originalCount + 1)),
		Message:        "scotty, we need more power",
		PolicyOverride: false,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Scale", scale, &resp)
	requireAssertion.NoError(err)
	requireAssertion.NotEmpty(resp.EvalID)
	requireAssertion.Greater(resp.EvalCreateIndex, resp.JobModifyIndex)

	// Check the evaluation priority matches the job priority.
	eval, err := fsmState.EvalByID(nil, resp.EvalID)
	requireAssertion.Nil(err)
	requireAssertion.NotNil(eval)
	requireAssertion.EqualValues(resp.EvalCreateIndex, eval.CreateIndex)
	requireAssertion.Equal(job.Priority, eval.Priority)
	requireAssertion.Equal(job.Type, eval.Type)
	requireAssertion.Equal(structs.EvalTriggerScaling, eval.TriggeredBy)
	requireAssertion.Equal(job.ID, eval.JobID)
	requireAssertion.NotZero(eval.CreateTime)
	requireAssertion.NotZero(eval.ModifyTime)
}

func TestJobEndpoint_Scale_SystemJob(t *testing.T) {
	ci.Parallel(t)

	testServer, testServerCleanup := TestServer(t, nil)
	defer testServerCleanup()
	codec := rpcClient(t, testServer)
	testutil.WaitForLeader(t, testServer.RPC)
	state := testServer.fsm.State()

	mockSystemJob := mock.SystemJob()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 10, nil, mockSystemJob))

	scaleReq := &structs.JobScaleRequest{
		JobID: mockSystemJob.ID,
		Target: map[string]string{
			structs.ScalingTargetGroup: mockSystemJob.TaskGroups[0].Name,
		},
		Count: pointer.Of(int64(13)),
		WriteRequest: structs.WriteRequest{
			Region:    DefaultRegion,
			Namespace: mockSystemJob.Namespace,
		},
	}
	var resp structs.JobRegisterResponse
	must.ErrorContains(t, msgpackrpc.CallWithCodec(codec, "Job.Scale", scaleReq, &resp),
		`400,cannot scale jobs of type "system"`)
}

func TestJobEndpoint_Scale_BatchJob(t *testing.T) {
	ci.Parallel(t)

	testServer, testServerCleanup := TestServer(t, nil)
	defer testServerCleanup()
	codec := rpcClient(t, testServer)
	testutil.WaitForLeader(t, testServer.RPC)
	state := testServer.fsm.State()

	mockBatchJob := mock.BatchJob()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 10, nil, mockBatchJob))

	scaleReq := &structs.JobScaleRequest{
		JobID: mockBatchJob.ID,
		Target: map[string]string{
			structs.ScalingTargetGroup: mockBatchJob.TaskGroups[0].Name,
		},
		Count: pointer.Of(int64(13)),
		WriteRequest: structs.WriteRequest{
			Region:    DefaultRegion,
			Namespace: mockBatchJob.Namespace,
		},
	}
	var resp structs.JobRegisterResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Scale", scaleReq, &resp))

	// Pull the generated evaluation from state and ensure the detailed type
	// matches the jobspec.
	testFMSSnapshot, err := testServer.fsm.State().Snapshot()
	must.NoError(t, err)

	scaleEval, err := testFMSSnapshot.EvalByID(nil, resp.EvalID)
	must.NoError(t, err)
	must.Eq(t, resp.EvalID, scaleEval.ID)
	must.Eq(t, mockBatchJob.ID, scaleEval.JobID)
	must.Eq(t, mockBatchJob.Type, scaleEval.Type)
}

func TestJobEndpoint_InvalidCount(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	job := mock.Job()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	require.Nil(err)

	scale := &structs.JobScaleRequest{
		JobID: job.ID,
		Target: map[string]string{
			structs.ScalingTargetGroup: job.TaskGroups[0].Name,
		},
		Count: pointer.Of(int64(-1)),
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Scale", scale, &resp)
	require.Error(err)
}

func TestJobEndpoint_GetScaleStatus(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	jobV1 := mock.Job()

	// check before registration
	// Fetch the scaling status
	get := &structs.JobScaleStatusRequest{
		JobID: jobV1.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: jobV1.Namespace,
		},
	}
	var resp2 structs.JobScaleStatusResponse
	require.NoError(msgpackrpc.CallWithCodec(codec, "Job.ScaleStatus", get, &resp2))
	require.Nil(resp2.JobScaleStatus)

	// stopped (previous version)
	require.NoError(state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, jobV1), "UpsertJob")
	a0 := mock.Alloc()
	a0.Job = jobV1
	a0.Namespace = jobV1.Namespace
	a0.JobID = jobV1.ID
	a0.ClientStatus = structs.AllocClientStatusComplete
	require.NoError(state.UpsertAllocs(structs.MsgTypeTestSetup, 1010, []*structs.Allocation{a0}), "UpsertAllocs")

	jobV2 := jobV1.Copy()
	require.NoError(state.UpsertJob(structs.MsgTypeTestSetup, 1100, nil, jobV2), "UpsertJob")
	a1 := mock.Alloc()
	a1.Job = jobV2
	a1.Namespace = jobV2.Namespace
	a1.JobID = jobV2.ID
	a1.ClientStatus = structs.AllocClientStatusRunning
	// healthy
	a1.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(true),
	}
	a2 := mock.Alloc()
	a2.Job = jobV2
	a2.Namespace = jobV2.Namespace
	a2.JobID = jobV2.ID
	a2.ClientStatus = structs.AllocClientStatusPending
	// unhealthy
	a2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(false),
	}
	a3 := mock.Alloc()
	a3.Job = jobV2
	a3.Namespace = jobV2.Namespace
	a3.JobID = jobV2.ID
	a3.ClientStatus = structs.AllocClientStatusRunning
	// canary
	a3.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(true),
		Canary:  true,
	}
	// no health
	a4 := mock.Alloc()
	a4.Job = jobV2
	a4.Namespace = jobV2.Namespace
	a4.JobID = jobV2.ID
	a4.ClientStatus = structs.AllocClientStatusRunning
	// upsert allocations
	require.NoError(state.UpsertAllocs(structs.MsgTypeTestSetup, 1110, []*structs.Allocation{a1, a2, a3, a4}), "UpsertAllocs")

	event := &structs.ScalingEvent{
		Time:    time.Now().Unix(),
		Count:   pointer.Of(int64(5)),
		Message: "message",
		Error:   false,
		Meta: map[string]interface{}{
			"a": "b",
		},
		EvalID: nil,
	}

	require.NoError(state.UpsertScalingEvent(1003, &structs.ScalingEventRequest{
		Namespace:    jobV2.Namespace,
		JobID:        jobV2.ID,
		TaskGroup:    jobV2.TaskGroups[0].Name,
		ScalingEvent: event,
	}), "UpsertScalingEvent")

	// check after job registration
	require.NoError(msgpackrpc.CallWithCodec(codec, "Job.ScaleStatus", get, &resp2))
	require.NotNil(resp2.JobScaleStatus)

	expectedStatus := structs.JobScaleStatus{
		JobID:          jobV2.ID,
		Namespace:      jobV2.Namespace,
		JobCreateIndex: jobV2.CreateIndex,
		JobModifyIndex: a1.CreateIndex,
		JobStopped:     jobV2.Stop,
		TaskGroups: map[string]*structs.TaskGroupScaleStatus{
			jobV2.TaskGroups[0].Name: {
				Desired:   jobV2.TaskGroups[0].Count,
				Placed:    3,
				Running:   2,
				Healthy:   1,
				Unhealthy: 1,
				Events:    []*structs.ScalingEvent{event},
			},
		},
	}

	require.True(reflect.DeepEqual(*resp2.JobScaleStatus, expectedStatus))
}

func TestJobEndpoint_GetScaleStatus_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create the job
	job := mock.Job()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	require.Nil(err)

	// Get the job scale status
	get := &structs.JobScaleStatusRequest{
		JobID: job.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Get without a token should fail
	var resp structs.JobScaleStatusResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.ScaleStatus", get, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
	get.AuthToken = invalidToken.SecretID
	require.NotNil(err)
	err = msgpackrpc.CallWithCodec(codec, "Job.ScaleStatus", get, &resp)
	require.Contains(err.Error(), "Permission denied")

	type testCase struct {
		authToken string
		name      string
	}
	cases := []testCase{
		{
			name:      "mgmt token should succeed",
			authToken: root.SecretID,
		},
		{
			name: "read disposition should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-read",
				mock.NamespacePolicy(structs.DefaultNamespace, "read", nil)).
				SecretID,
		},
		{
			name: "write disposition should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-write",
				mock.NamespacePolicy(structs.DefaultNamespace, "write", nil)).
				SecretID,
		},
		{
			name: "autoscaler disposition should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-autoscaler",
				mock.NamespacePolicy(structs.DefaultNamespace, "scale", nil)).
				SecretID,
		},
		{
			name: "read-job capability should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-read-job",
				mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob})).SecretID,
		},
		{
			name: "read-job-scaling capability should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-read-job-scaling",
				mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJobScaling})).
				SecretID,
		},
	}

	for _, tc := range cases {
		get.AuthToken = tc.authToken
		var validResp structs.JobScaleStatusResponse
		err = msgpackrpc.CallWithCodec(codec, "Job.ScaleStatus", get, &validResp)
		require.NoError(err, tc.name)
		require.NotNil(validResp.JobScaleStatus)
	}
}

func TestJob_GetServiceRegistrations(t *testing.T) {
	ci.Parallel(t)

	// This function is a helper function to set up job and service which can
	// be queried.
	correctSetupFn := func(s *Server) (error, string, *structs.ServiceRegistration) {
		// Generate an upsert a job.
		job := mock.Job()
		err := s.State().UpsertJob(structs.MsgTypeTestSetup, 10, nil, job)
		if err != nil {
			return nil, "", nil
		}

		// Generate services. Set the jobID on the first service so this
		// matches the job now held in state.
		services := mock.ServiceRegistrations()
		services[0].JobID = job.ID
		err = s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 20, services)

		return err, job.ID, services[0]
	}

	testCases := []struct {
		serverFn func(t *testing.T) (*Server, *structs.ACLToken, func())
		testFn   func(t *testing.T, s *Server, token *structs.ACLToken)
		name     string
	}{
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				server, cleanup := TestServer(t, nil)
				return server, nil, cleanup
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				err, jobID, service := correctSetupFn(s)
				require.NoError(t, err)

				// Perform a lookup and test the response.
				serviceRegReq := &structs.JobServiceRegistrationsRequest{
					JobID: jobID,
					QueryOptions: structs.QueryOptions{
						Namespace: service.Namespace,
						Region:    s.Region(),
					},
				}
				var serviceRegResp structs.JobServiceRegistrationsResponse
				err = msgpackrpc.CallWithCodec(codec, structs.JobServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.EqualValues(t, uint64(20), serviceRegResp.Index)
				require.ElementsMatch(t, serviceRegResp.Services, []*structs.ServiceRegistration{service})
			},
			name: "ACLs disabled job found with regs",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				server, cleanup := TestServer(t, nil)
				return server, nil, cleanup
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert our services.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 20, services))

				// Perform a lookup on the first service using the job ID. This
				// job does not exist within the Nomad state meaning the
				// service is orphaned or the caller used an incorrect job ID.
				serviceRegReq := &structs.JobServiceRegistrationsRequest{
					JobID: services[0].JobID,
					QueryOptions: structs.QueryOptions{
						Namespace: services[0].Namespace,
						Region:    s.Region(),
					},
				}
				var serviceRegResp structs.JobServiceRegistrationsResponse
				err := msgpackrpc.CallWithCodec(codec, structs.JobServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.Nil(t, serviceRegResp.Services)
			},
			name: "ACLs disabled job not found",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				server, cleanup := TestServer(t, nil)
				return server, nil, cleanup
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate an upsert a job.
				job := mock.Job()
				require.NoError(t, s.State().UpsertJob(structs.MsgTypeTestSetup, 10, nil, job))

				// Perform a lookup and test the response.
				serviceRegReq := &structs.JobServiceRegistrationsRequest{
					JobID: job.ID,
					QueryOptions: structs.QueryOptions{
						Namespace: job.Namespace,
						Region:    s.Region(),
					},
				}
				var serviceRegResp structs.JobServiceRegistrationsResponse
				err := msgpackrpc.CallWithCodec(codec, structs.JobServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, serviceRegResp.Services, []*structs.ServiceRegistration{})
			},
			name: "ACLs disabled job found without regs",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				err, jobID, service := correctSetupFn(s)
				require.NoError(t, err)

				// Perform a lookup and test the response.
				serviceRegReq := &structs.JobServiceRegistrationsRequest{
					JobID: jobID,
					QueryOptions: structs.QueryOptions{
						Namespace: service.Namespace,
						Region:    s.Region(),
						AuthToken: token.SecretID,
					},
				}
				var serviceRegResp structs.JobServiceRegistrationsResponse
				err = msgpackrpc.CallWithCodec(codec, structs.JobServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, serviceRegResp.Services, []*structs.ServiceRegistration{service})
			},
			name: "ACLs enabled use management token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				err, jobID, service := correctSetupFn(s)
				require.NoError(t, err)

				// Create and policy and grab the auth token.
				authToken := mock.CreatePolicyAndToken(t, s.State(), 30, "test-node-get-service-reg",
					mock.NamespacePolicy(service.Namespace, "", []string{acl.NamespaceCapabilityReadJob})).SecretID

				// Perform a lookup and test the response.
				serviceRegReq := &structs.JobServiceRegistrationsRequest{
					JobID: jobID,
					QueryOptions: structs.QueryOptions{
						Namespace: service.Namespace,
						Region:    s.Region(),
						AuthToken: authToken,
					},
				}
				var serviceRegResp structs.JobServiceRegistrationsResponse
				err = msgpackrpc.CallWithCodec(codec, structs.JobServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, serviceRegResp.Services, []*structs.ServiceRegistration{service})
			},
			name: "ACLs enabled use read-job namespace capability token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				err, jobID, service := correctSetupFn(s)
				require.NoError(t, err)

				// Create and policy and grab the auth token.
				authToken := mock.CreatePolicyAndToken(t, s.State(), 30, "test-node-get-service-reg",
					mock.NamespacePolicy(service.Namespace, "read", nil)).SecretID

				// Perform a lookup and test the response.
				serviceRegReq := &structs.JobServiceRegistrationsRequest{
					JobID: jobID,
					QueryOptions: structs.QueryOptions{
						Namespace: service.Namespace,
						Region:    s.Region(),
						AuthToken: authToken,
					},
				}
				var serviceRegResp structs.JobServiceRegistrationsResponse
				err = msgpackrpc.CallWithCodec(codec, structs.JobServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, serviceRegResp.Services, []*structs.ServiceRegistration{service})
			},
			name: "ACLs enabled use read namespace policy token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				err, jobID, service := correctSetupFn(s)
				require.NoError(t, err)

				// Create and policy and grab the auth token.
				authToken := mock.CreatePolicyAndToken(t, s.State(), 30, "test-node-get-service-reg",
					mock.NamespacePolicy("ohno", "read", nil)).SecretID

				// Perform a lookup and test the response.
				serviceRegReq := &structs.JobServiceRegistrationsRequest{
					JobID: jobID,
					QueryOptions: structs.QueryOptions{
						Namespace: service.Namespace,
						Region:    s.Region(),
						AuthToken: authToken,
					},
				}
				var serviceRegResp structs.JobServiceRegistrationsResponse
				err = msgpackrpc.CallWithCodec(codec, structs.JobServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.Error(t, err)
				require.Contains(t, err.Error(), "Permission denied")
				require.Empty(t, serviceRegResp.Services)
			},
			name: "ACLs enabled use read incorrect namespace policy token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				err, jobID, service := correctSetupFn(s)
				require.NoError(t, err)

				// Create and policy and grab the auth token.
				authToken := mock.CreatePolicyAndToken(t, s.State(), 30, "test-node-get-service-reg",
					mock.NamespacePolicy(service.Namespace, "", []string{acl.NamespaceCapabilityReadScalingPolicy})).SecretID

				// Perform a lookup and test the response.
				serviceRegReq := &structs.JobServiceRegistrationsRequest{
					JobID: jobID,
					QueryOptions: structs.QueryOptions{
						Namespace: service.Namespace,
						Region:    s.Region(),
						AuthToken: authToken,
					},
				}
				var serviceRegResp structs.JobServiceRegistrationsResponse
				err = msgpackrpc.CallWithCodec(codec, structs.JobServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.Error(t, err)
				require.Contains(t, err.Error(), "Permission denied")
				require.Empty(t, serviceRegResp.Services)
			},
			name: "ACLs enabled use incorrect capability",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server, aclToken, cleanup := tc.serverFn(t)
			defer cleanup()
			tc.testFn(t, server, aclToken)
		})
	}
}
