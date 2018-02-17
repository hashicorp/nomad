package nomad

import (
	"sort"
	"strings"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/nomad/testutil/rpcapi"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

// TestNodeDrainer_SimpleDrain asserts that draining when there are two nodes
// moves allocs from the draining node to the other node.
func TestNodeDrainer_SimpleDrain(t *testing.T) {
	require := require.New(t)
	logger := testlog.Logger(t)
	server := TestServer(t, nil)
	defer server.Shutdown()

	testutil.WaitForLeader(t, server.RPC)

	// Setup 2 Nodes: A & B; A has allocs and is draining

	node1 := mock.Node()
	node1.Name = "node-1"
	node2 := mock.Node()
	node2.Name = "node-2"

	// Create mock jobs
	state := server.fsm.State()

	serviceJob := mock.Job()
	serviceJob.Name = "service-job"
	serviceJob.Type = structs.JobTypeService
	serviceJob.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	serviceJob.TaskGroups[0].Tasks[0].Resources = structs.MinResources()
	serviceJob.TaskGroups[0].Tasks[0].Services = nil

	systemJob := mock.SystemJob()
	systemJob.Name = "system-job"
	systemJob.Type = structs.JobTypeSystem
	//FIXME hack until system job reschedule policy validation is fixed
	systemJob.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{Attempts: 1, Interval: time.Minute}
	systemJob.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	systemJob.TaskGroups[0].Tasks[0].Resources = structs.MinResources()
	systemJob.TaskGroups[0].Tasks[0].Services = nil

	batchJob := mock.Job()
	batchJob.Name = "batch-job"
	batchJob.Type = structs.JobTypeBatch
	batchJob.TaskGroups[0].Name = "batch-group"
	batchJob.TaskGroups[0].Migrate = nil
	batchJob.TaskGroups[0].Tasks[0].Name = "batch-task"
	batchJob.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	batchJob.TaskGroups[0].Tasks[0].Resources = structs.MinResources()
	batchJob.TaskGroups[0].Tasks[0].Services = nil

	// Start node-1
	c1 := client.TestClient(t, func(conf *config.Config) {
		conf.Node = node1
		conf.NetworkSpeed = 10000
		conf.LogOutput = testlog.NewWriter(t)
		conf.Servers = []string{server.config.RPCAddr.String()}
	})
	defer c1.Shutdown()

	// Start jobs so they all get placed on node-1
	codec := rpcClient(t, server)
	for _, job := range []*structs.Job{systemJob, serviceJob, batchJob} {
		req := &structs.JobRegisterRequest{
			Job: job.Copy(),
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: job.Namespace,
			},
		}

		// Fetch the response
		var resp structs.JobRegisterResponse
		require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
		require.NotZero(resp.Index)
		logger.Printf("%s modifyindex: %d warnings: %s", job.Name, resp.JobModifyIndex, resp.Warnings)
	}

	//FIXME replace with a WaitForResult
	logger.Printf("...waiting for jobs to start...")
	time.Sleep(5 * time.Second)

	// Start node-2
	c2 := client.TestClient(t, func(conf *config.Config) {
		conf.Node = node2
		conf.Servers = []string{server.config.RPCAddr.String()}
	})
	defer c2.Shutdown()

	// Wait for all service allocs to be replaced
	rpc := rpcapi.NewRPC(codec)
	jobs, err := rpc.JobList()
	require.Nil(err)
	t.Logf("%d jobs", len(jobs.Jobs))
	for _, job := range jobs.Jobs {
		t.Logf("job: %s status: %s %s", job.Name, job.Status, job.StatusDescription)
	}

	allocs, err := rpc.AllocAll()
	require.Nil(err)

	sort.Slice(allocs, func(i, j int) bool {
		r := strings.Compare(allocs[i].Job.Name, allocs[j].Job.Name)
		switch {
		case r < 0:
			return true
		case r == 0:
			return allocs[i].ModifyIndex < allocs[j].ModifyIndex
		case r > 0:
			return false
		}
		panic("unreachable")
	})

	t.Logf("%d allocs", len(allocs))
	for _, alloc := range allocs {
		t.Logf("job: %s alloc: %s desired: %s actual: %s replaces: %s", alloc.Job.Name, alloc.ID, alloc.DesiredStatus, alloc.ClientStatus, alloc.PreviousAllocation)
	}

	iter, err := state.Evals(nil)
	require.Nil(err)

	evals := map[string]*structs.Evaluation{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		eval := raw.(*structs.Evaluation)
		evals[eval.ID] = eval
	}

	for _, eval := range evals {
		if eval.Status == structs.EvalStatusBlocked {
			blocked := evals[eval.PreviousEval]
			t.Logf("Blocked evaluation: %q - %v\n%s\n--blocked %q - %v\n%s", eval.ID, eval.StatusDescription, pretty.Sprint(eval), blocked.ID, blocked.StatusDescription, pretty.Sprint(blocked))
		}
	}
}
