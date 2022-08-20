package jobs

import (
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_RegisterService_Register(t *testing.T) {
	ci.Parallel(t)

	srv, cleanup := nomad.TestServer(t, func(c *nomad.Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanup()

	// TODO: Abstract away codec to allow GRPC or other replacement
	//codec := nomad.RPCClient(t, srv)
	//_ = codec
	testutil.WaitForLeader(t, srv.RPC)

	// TODO: Refactor to ServiceRegistry that uses Factory Method.
	svc := RegisterService{
		srv:    srv,
		logger: testlog.HCLogger(t),
	}

	svc.Init()

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
	err := svc.Register(req, &resp)
	require.NoError(t, err)
	require.NotEqual(t, resp.Index, 0, "bad index: %d", resp.Index)

	// Check for the node in the FSM
	state := srv.State()
	out, err := state.JobByID(nil, job.Namespace, job.ID)
	require.NoError(t, err)
	require.NotNil(t, out, "expected job")
	require.Equal(t, out.CreateIndex, resp.JobModifyIndex, "index mis-match")

	serviceName := out.TaskGroups[0].Tasks[0].Services[0].Name
	expectedServiceName := "web-frontend"
	require.Equal(t, expectedServiceName, serviceName, "Expected Service Name: %s, Actual: %s", expectedServiceName, serviceName)

	// Lookup the evaluation
	eval, err := state.EvalByID(nil, resp.EvalID)
	require.NoError(t, err)
	require.NotNil(t, eval, "expected eval")
	require.Equal(t, eval.CreateIndex, resp.EvalCreateIndex, "index mis-match")
	require.Equal(t, eval.Priority, job.Priority, "bad eval priority: %#v", eval)
	require.Equal(t, eval.Type, job.Type, "bad eval type: %#v", eval)
	require.Equal(t, eval.TriggeredBy, structs.EvalTriggerJobRegister, "bad eval triggered by: %#v", eval)
	require.Equal(t, eval.JobID, job.ID, "bad eval job id: %#v", eval)
	require.Equal(t, eval.JobModifyIndex, resp.JobModifyIndex, "bad eval job modify index: %#v", eval)
	require.Equal(t, eval.Status, structs.EvalStatusPending, "bad eval status: %#v", eval)
	require.NotEqual(t, 0, eval.CreateTime, "eval CreateTime is unset: %#v", eval)
	require.NotEqual(t, 0, eval.ModifyTime, "eval ModifyTime is unset: %#v", eval)
}
