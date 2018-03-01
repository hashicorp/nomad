package drainer_test

import (
	"fmt"
	"net"
	"net/rpc"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/pool"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/nomad/testutil/rpcapi"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rpcClient is a test helper method to return a ClientCodec to use to make rpc
// calls to the passed server.
func rpcClient(t *testing.T, conf *nomad.Config) rpc.ClientCodec {
	addr := conf.RPCAddr
	conn, err := net.DialTimeout("tcp", addr.String(), time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Write the Nomad RPC byte to set the mode
	conn.Write([]byte{byte(pool.RpcNomad)})
	return pool.NewClientCodec(conn)
}

// TestNodeDrainer_SimpleDrain asserts that draining when there are two nodes
// moves allocs from the draining node to the other node.
func TestNodeDrainer_SimpleDrain(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Capture test servers config
	var serverConfig *nomad.Config
	server := nomad.TestServer(t, func(c *nomad.Config) {
		serverConfig = c
	})
	defer server.Shutdown()

	testutil.WaitForLeader(t, server.RPC)

	// Setup 2 Nodes: A & B; A has allocs and is draining

	// Create mock jobs
	state := server.State()

	serviceJob := mock.Job()
	serviceJob.Name = "service-job"
	serviceJob.Type = structs.JobTypeService
	serviceJob.Constraints = nil
	serviceJob.TaskGroups[0].Migrate = &structs.MigrateStrategy{
		MaxParallel:     1,
		HealthCheck:     structs.MigrateStrategyHealthStates,
		MinHealthyTime:  time.Millisecond,
		HealthyDeadline: 2 * time.Second,
	}
	serviceJob.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	serviceJob.TaskGroups[0].Tasks[0].Resources = structs.MinResources()
	serviceJob.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for":    "10m",
		"kill_after": "1ms",
	}
	serviceJob.TaskGroups[0].Tasks[0].Services = nil

	systemJob := mock.SystemJob()
	systemJob.Name = "system-job"
	systemJob.Type = structs.JobTypeSystem
	systemJob.Constraints = nil
	systemJob.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	systemJob.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for":    "10m",
		"kill_after": "1ms",
	}
	systemJob.TaskGroups[0].Tasks[0].Resources = structs.MinResources()
	systemJob.TaskGroups[0].Tasks[0].Services = nil

	// Batch job will run until the node's drain deadline is reached
	batchJob := mock.Job()
	batchJob.Name = "batch-job"
	batchJob.Type = structs.JobTypeBatch
	batchJob.Constraints = nil
	batchJob.TaskGroups[0].Name = "batch-group"
	batchJob.TaskGroups[0].Migrate = nil
	batchJob.TaskGroups[0].Tasks[0].Name = "batch-task"
	batchJob.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	batchJob.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for":    "10m",
		"kill_after": "1ms",
		"exit_code":  13, // set nonzero exit code to cause rescheduling
	}
	batchJob.TaskGroups[0].Tasks[0].Resources = structs.MinResources()
	batchJob.TaskGroups[0].Tasks[0].Services = nil

	// Start node 1
	c1 := client.TestClient(t, func(conf *config.Config) {
		conf.LogOutput = testlog.NewWriter(t)
		conf.Servers = []string{serverConfig.RPCAddr.String()}
	})
	defer c1.Shutdown()
	node1ID := c1.NodeID()

	// Start jobs so they all get placed on node 1
	codec := rpcClient(t, serverConfig)
	rpc := rpcapi.NewRPC(codec)
	for _, job := range []*structs.Job{systemJob, serviceJob, batchJob} {
		resp, err := rpc.JobRegister(job)
		require.Nil(err)
		require.NotZero(resp.Index)
	}

	// Wait for jobs to start on c1
	testutil.WaitForResult(func() (bool, error) {
		resp, err := rpc.NodeGetAllocs(node1ID)
		if err != nil {
			return false, err
		}

		system, batch, service := 0, 0, 0
		for _, alloc := range resp.Allocs {
			if alloc.ClientStatus != structs.AllocClientStatusRunning {
				return false, fmt.Errorf("alloc %s for job %s not running: %s", alloc.ID, alloc.Job.Name, alloc.ClientStatus)
			}
			switch alloc.JobID {
			case batchJob.ID:
				batch++
			case serviceJob.ID:
				service++
			case systemJob.ID:
				system++
			}
		}
		// 1 system + 10 batch + 10 service = 21
		if system+batch+service != 21 {
			return false, fmt.Errorf("wrong number of allocs: system %d/1, batch %d/10, service %d/10", system, batch, service)
		}
		return true, nil
	}, func(err error) {
		if resp, err := rpc.NodeGetAllocs(node1ID); err == nil {
			for i, alloc := range resp.Allocs {
				t.Logf("%d alloc %s job %s status %s", i, alloc.ID, alloc.Job.Name, alloc.ClientStatus)
			}
		}
		if resp, err := rpc.EvalList(); err == nil {
			for _, eval := range resp.Evaluations {
				t.Logf("% #v\n", pretty.Formatter(eval))
			}
		}
		t.Fatalf("failed waiting for all allocs to start: %v", err)
	})

	// Start draining node 1 with no deadline
	strategy := &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: -1 * time.Second,
		},
	}
	node1Resp, err := rpc.NodeGet(node1ID)
	require.Nil(err)
	node1 := node1Resp.Node
	require.Nil(state.UpdateNodeDrain(node1.ModifyIndex+1, node1ID, strategy))

	// Start node 2
	c2 := client.TestClient(t, func(conf *config.Config) {
		conf.LogOutput = testlog.NewWriter(t)
		conf.Servers = []string{serverConfig.RPCAddr.String()}
	})
	defer c2.Shutdown()
	node2ID := c2.NodeID()

	// Wait for services to be migrated
	testutil.WaitForResult(func() (bool, error) {
		resp, err := rpc.NodeGetAllocs(node2ID)
		if err != nil {
			return false, err
		}

		system, batch, service := 0, 0, 0
		for _, alloc := range resp.Allocs {
			if alloc.ClientStatus != structs.AllocClientStatusRunning {
				return false, fmt.Errorf("alloc %s for job %s not running: %s", alloc.ID, alloc.Job.Name, alloc.ClientStatus)
			}
			switch alloc.JobID {
			case batchJob.ID:
				batch++
			case serviceJob.ID:
				service++
			case systemJob.ID:
				system++
			}
		}
		// 1 system + 10 batch + 10 service = 21
		if system+batch+service != 21 {
			return false, fmt.Errorf("wrong number of allocs: system %d/1, batch %d/10, service %d/10", system, batch, service)
		}
		return true, nil
	}, func(err error) {
		if resp, err := rpc.NodeGetAllocs(node2ID); err == nil {
			for i, alloc := range resp.Allocs {
				t.Logf("%d alloc %s job %s status %s prev %s", i, alloc.ID, alloc.Job.Name, alloc.ClientStatus, alloc.PreviousAllocation)
			}
		}
		t.Errorf("failed waiting for all allocs to migrate: %v", err)
	})

	// Wait for drained services to be dead
	testutil.WaitForResult(func() (bool, error) {
		resp, err := rpc.NodeGetAllocs(c1.NodeID())
		if err != nil {
			return false, err
		}

		running := make([]string, 0, len(resp.Allocs))
		for _, alloc := range resp.Allocs {
			if alloc.ClientStatus == structs.AllocClientStatusRunning {
				running = append(running, alloc.ID[:6])
			}
		}

		if len(running) > 0 {
			return false, fmt.Errorf("%d alloc(s) on draining node %s still running: %s", len(running), c1.NodeID()[:6], running)
		}
		return true, nil
	}, func(err error) {
		t.Errorf("failed waiting for all draining allocs to stop: %v", err)
	})

	node1Resp, err = rpc.NodeGet(node1ID)
	require.Nil(err)
	node1 = node1Resp.Node
	assert.False(node1.Drain)
	assert.Nil(node1.DrainStrategy)
	assert.Equal(structs.NodeSchedulingIneligible, node1.SchedulingEligibility)

	jobs, err := rpc.JobList()
	require.Nil(err)
	t.Logf("--> %d jobs", len(jobs.Jobs))
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

	t.Logf("--> %d allocs", len(allocs))
	for _, alloc := range allocs {
		t.Logf("job: %s  node: %s  alloc: %s  desired_status: %s  desired_transition: %s  actual: %s  replaces: %s",
			alloc.Job.Name, alloc.NodeID[:6], alloc.ID[:6], alloc.DesiredStatus, pretty.Sprint(alloc.DesiredTransition.Migrate), alloc.ClientStatus, alloc.PreviousAllocation)
	}

	t.Logf("==> PASS")
}

// TestNodeDrainer_SystemDrain asserts system jobs are drained
func TestNodeDrainer_SystemDrain(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Capture test servers config
	var serverConfig *nomad.Config
	server := nomad.TestServer(t, func(c *nomad.Config) {
		serverConfig = c
	})
	defer server.Shutdown()

	testutil.WaitForLeader(t, server.RPC)

	// Setup 2 Nodes: A & B; A has allocs and is draining

	// Create mock jobs
	state := server.State()

	serviceJob := mock.Job()
	serviceJob.Name = "service-job"
	serviceJob.Type = structs.JobTypeService
	serviceJob.Constraints = nil
	serviceJob.TaskGroups[0].Count = 2
	serviceJob.TaskGroups[0].Migrate = &structs.MigrateStrategy{
		MaxParallel:     1,
		HealthCheck:     structs.MigrateStrategyHealthStates,
		MinHealthyTime:  time.Millisecond,
		HealthyDeadline: 2 * time.Second,
	}
	serviceJob.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	serviceJob.TaskGroups[0].Tasks[0].Resources = structs.MinResources()
	serviceJob.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for":    "10m",
		"kill_after": "1ms",
	}
	serviceJob.TaskGroups[0].Tasks[0].Services = nil

	systemJob := mock.SystemJob()
	systemJob.Name = "system-job"
	systemJob.Type = structs.JobTypeSystem
	systemJob.Constraints = nil
	systemJob.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	systemJob.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for":    "10m",
		"kill_after": "1ms",
	}
	systemJob.TaskGroups[0].Tasks[0].Resources = structs.MinResources()
	systemJob.TaskGroups[0].Tasks[0].Services = nil

	// Start node 1
	c1 := client.TestClient(t, func(conf *config.Config) {
		conf.LogOutput = testlog.NewWriter(t)
		conf.Servers = []string{serverConfig.RPCAddr.String()}
	})
	defer c1.Shutdown()
	node1ID := c1.NodeID()

	// Start jobs so they all get placed on node 1
	codec := rpcClient(t, serverConfig)
	rpc := rpcapi.NewRPC(codec)
	for _, job := range []*structs.Job{systemJob, serviceJob} {
		resp, err := rpc.JobRegister(job)
		require.Nil(err)
		require.NotZero(resp.Index)
	}

	// Wait for jobs to start on c1
	testutil.WaitForResult(func() (bool, error) {
		resp, err := rpc.NodeGetAllocs(c1.NodeID())
		if err != nil {
			return false, err
		}

		system, service := 0, 0
		for _, alloc := range resp.Allocs {
			if alloc.ClientStatus != structs.AllocClientStatusRunning {
				return false, fmt.Errorf("alloc %s for job %s not running: %s", alloc.ID, alloc.Job.Name, alloc.ClientStatus)
			}
			switch alloc.JobID {
			case serviceJob.ID:
				service++
			case systemJob.ID:
				system++
			default:
				return false, fmt.Errorf("unknown job: %s", alloc.Job.Name)
			}
		}
		// 1 system + 2 service = 3
		if system+service != 3 {
			return false, fmt.Errorf("wrong number of allocs: system %d/1, service %d/2", system, service)
		}
		return true, nil
	}, func(err error) {
		if resp, err := rpc.NodeGetAllocs(c1.NodeID()); err == nil {
			for i, alloc := range resp.Allocs {
				t.Logf("%d alloc %s job %s status %s", i, alloc.ID, alloc.Job.Name, alloc.ClientStatus)
			}
		}
		t.Fatalf("failed waiting for all allocs to start: %v", err)
	})

	// Start draining node 1
	strategy := &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: 1 * time.Hour,
		},
	}
	node1Resp, err := rpc.NodeGet(node1ID)
	require.Nil(err)
	node1 := node1Resp.Node
	require.Nil(state.UpdateNodeDrain(node1.ModifyIndex+1, node1ID, strategy))

	// Start node 2
	c2 := client.TestClient(t, func(conf *config.Config) {
		conf.LogOutput = testlog.NewWriter(t)
		conf.Servers = []string{serverConfig.RPCAddr.String()}
	})
	defer c2.Shutdown()
	node2ID := c2.NodeID()

	// Wait for services to be migrated
	testutil.WaitForResult(func() (bool, error) {
		resp, err := rpc.NodeGetAllocs(node2ID)
		if err != nil {
			return false, err
		}

		system, service := 0, 0
		for _, alloc := range resp.Allocs {
			if alloc.ClientStatus != structs.AllocClientStatusRunning {
				return false, fmt.Errorf("alloc %s for job %s not running: %s", alloc.ID, alloc.Job.Name, alloc.ClientStatus)
			}
			switch alloc.JobID {
			case serviceJob.ID:
				service++
			case systemJob.ID:
				system++
			default:
				return false, fmt.Errorf("unknown job: %s", alloc.Job.Name)
			}
		}
		// 1 system + 2 service = 3
		if system+service != 3 {
			return false, fmt.Errorf("wrong number of allocs: system %d/1, service %d/2", system, service)
		}
		return true, nil
	}, func(err error) {
		if resp, err := rpc.NodeGetAllocs(node2ID); err == nil {
			for i, alloc := range resp.Allocs {
				t.Logf("%d alloc %s job %s status %s prev %s", i, alloc.ID, alloc.Job.Name, alloc.ClientStatus, alloc.PreviousAllocation)
			}
		}
		t.Errorf("failed waiting for all allocs to migrate: %v", err)
	})

	// Wait for drained services to be dead
	testutil.WaitForResult(func() (bool, error) {
		resp, err := rpc.NodeGetAllocs(node1ID)
		if err != nil {
			return false, err
		}

		running := make([]string, 0, len(resp.Allocs))
		for _, alloc := range resp.Allocs {
			if alloc.ClientStatus == structs.AllocClientStatusRunning {
				running = append(running, alloc.ID[:6])
			}
		}

		if len(running) > 0 {
			return false, fmt.Errorf("%d alloc(s) on draining node %s still running: %s", len(running), node1ID[:6], running)
		}
		return true, nil
	}, func(err error) {
		t.Errorf("failed waiting for all draining allocs to stop: %v", err)
	})

	node1Resp, err = rpc.NodeGet(node1ID)
	require.Nil(err)
	node1 = node1Resp.Node
	assert.False(node1.Drain)
	assert.Nil(node1.DrainStrategy)
	assert.Equal(structs.NodeSchedulingIneligible, node1.SchedulingEligibility)

	jobs, err := rpc.JobList()
	require.Nil(err)
	t.Logf("--> %d jobs", len(jobs.Jobs))
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

	t.Logf("--> %d allocs", len(allocs))
	for _, alloc := range allocs {
		t.Logf("job: %s  node: %s  alloc: %s  desired_status: %s  desired_transition: %s  actual: %s  replaces: %s",
			alloc.Job.Name, alloc.NodeID[:6], alloc.ID[:6], alloc.DesiredStatus, pretty.Sprint(alloc.DesiredTransition.Migrate), alloc.ClientStatus, alloc.PreviousAllocation)
	}
}
