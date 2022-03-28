package nomad

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// TestCluster is a cluster of test serves and clients suitable for integration testing.
type TestCluster struct {
	cfg     *TestClusterConfig
	State   interface{}
	Leader  *Server
	Servers map[string]*Server
	Clients map[string]*client.Client
	Cleanup func() error
}

type TestClusterConfig struct {
	T                      *testing.T
	ValidateAllocs         bool
	ValidateEvals          bool
	DisconnectedClientName string
	ServerFns              map[string]func(*Config)
	ClientFns              map[string]func(*config.Config)
	TestMainFn             func(*testing.T, interface{})
	PipelineFns            map[string]func(interface{})
	ExpectedAllocStates    []*ExpectedAllocState
	ExpectedEvalStates     []*ExpectedEvalState
}

func NewTestCluster(cfg *TestClusterConfig) (*TestCluster, error) {
	if len(cfg.ServerFns) == 0 || len(cfg.ClientFns) == 0 {
		return nil,
			fmt.Errorf("invalid test cluster: requires both servers and client - server count %d client count %d", len(cfg.ServerFns), len(cfg.ClientFns))
	}

	cluster := &TestCluster{
		Servers: make(map[string]*Server, len(cfg.ServerFns)),
		Clients: make(map[string]*client.Client, len(cfg.ClientFns)),
		cfg:     cfg,
	}

	serverCleanups := make(map[string]func(), len(cfg.ServerFns))

	for name, fn := range cfg.ServerFns {
		testServer, cleanup := TestServer(cfg.T, fn)
		if cluster.Leader == nil {
			cluster.Leader = testServer
		}
		cluster.Servers[name] = testServer
		serverCleanups[name] = cleanup
	}

	testutil.WaitForLeader(cfg.T, cluster.Leader.RPC)

	for _, testServer := range cluster.Servers {
		if testServer.IsLeader() {
			cluster.Leader = testServer
			break
		}
	}

	clientCleanups := make(map[string]func() error, len(cfg.ClientFns))
	for name, fn := range cfg.ClientFns {
		configFn := func(c *config.Config) {
			c.RPCHandler = cluster.Leader
			fn(c)
		}
		testClient, cleanup := client.TestClient(cfg.T, configFn)

		cluster.Clients[name] = testClient
		clientCleanups[name] = cleanup
	}

	cluster.Cleanup = func() error {
		for _, serverCleanup := range serverCleanups {
			serverCleanup()
		}

		var mErr *multierror.Error

		for _, clientCleanup := range clientCleanups {
			err := clientCleanup()
			if err != nil {
				mErr = multierror.Append(mErr, err)
			}
		}

		return mErr.ErrorOrNil()
	}

	return cluster, nil
}

func (tc *TestCluster) RegisterJob(job *structs.Job) (uint64, error) {
	regReq := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	var regResp structs.JobRegisterResponse
	err := tc.Leader.RPC("Job.Register", regReq, &regResp)
	if err != nil {
		return 0, err
	}

	if regResp.Index == 0 {
		return 0, fmt.Errorf("error registering job: index is 0")
	}

	return regResp.JobModifyIndex, nil
}

func (tc *TestCluster) NodeID(clientName string) (string, bool) {
	testClient, ok := tc.Clients[clientName]
	if !ok {
		return "", ok
	}
	return testClient.NodeID(), ok
}

func (tc *TestCluster) FailHeartbeat(clientName string) error {
	testClient, ok := tc.Clients[clientName]
	if !ok {
		return fmt.Errorf("error failing hearbeat: client %s not found", clientName)
	}
	err := client.FailHeartbeat(testClient)
	if err != nil {
		return err
	}
	return nil
}

func (tc *TestCluster) ResumeHeartbeat(clientName string) error {
	testClient, ok := tc.Clients[clientName]
	if !ok {
		return fmt.Errorf("error resuming hearbeat: client %s not found", clientName)
	}
	return client.ResumeHeartbeat(testClient)
}

func (tc *TestCluster) FailTask(clientName, allocID, taskName, taskEvent string) error {
	testClient, ok := tc.Clients[clientName]
	if !ok {
		return fmt.Errorf("error failing hearbeat: client %s not found", clientName)
	}
	return client.FailTask(testClient, allocID, taskName, taskEvent)
}

func (tc *TestCluster) WaitForJobAllocsRunning(job *structs.Job, expectedAllocCount int) ([]*structs.Allocation, error) {
	var outAllocs []*structs.Allocation
	testutil.WaitForResult(func() (bool, error) {
		outAllocs = nil
		allocs, err := tc.Leader.State().AllocsByJob(nil, job.Namespace, job.ID, true)
		if err != nil {
			return false, err
		}

		for _, alloc := range allocs {
			if alloc.ClientStatus == structs.AllocClientStatusRunning {
				outAllocs = append(outAllocs, alloc)
			}
		}

		if len(outAllocs) == expectedAllocCount {
			return true, nil
		}
		return false, nil
	}, func(err error) {
		require.NoError(tc.cfg.T, err, "error retrieving allocs %s", err)
	})

	if len(outAllocs) != expectedAllocCount {
		return nil, fmt.Errorf("error waiting for job allocs: wanted %d have %d", expectedAllocCount, len(outAllocs))
	}

	//var mErr *multierror.Error
	//for _, alloc := range allocs {
	//	if alloc.ClientStatus != structs.AllocClientStatusRunning {
	//		mErr = multierror.Append(mErr, fmt.Errorf("error retrieving allocs: expected status running but alloc %s - %s has status %s", alloc.Name, alloc.ID, alloc.ClientStatus))
	//	}
	//}

	return outAllocs, nil
}

func (tc *TestCluster) WaitForNodeStatus(clientName, nodeStatus string) (err error) {
	testClient, ok := tc.Clients[clientName]
	if !ok || testClient == nil {
		return fmt.Errorf("error: client %s not found", clientName)
	}

	clientReady := false
	var nodeErr error
	var outNode *structs.Node
	testutil.WaitForResult(func() (bool, error) {
		outNode, nodeErr = tc.Leader.State().NodeByID(nil, testClient.Node().ID)
		if nodeErr != nil {
			return false, nodeErr
		}
		if outNode != nil {
			clientReady = outNode.Status == nodeStatus
		}
		return clientReady, nil
	}, func(waitErr error) {
		err = waitErr
	})

	if !clientReady {
		return fmt.Errorf("client %s failed to enter %s state", clientName, nodeStatus)
	}

	return
}

func (tc *TestCluster) WaitForAllocClientStatusOnClient(alloc *structs.Allocation, clientName, clientStatus string) error {
	testClient, ok := tc.Clients[clientName]
	if !ok || testClient == nil {
		return fmt.Errorf("error: client %s not found", clientName)
	}

	var err error
	var outAlloc *structs.Allocation
	testutil.WaitForResult(func() (bool, error) {
		outAlloc, err = testClient.GetAlloc(alloc.ID)
		if err != nil {
			return false, err
		}
		if outAlloc != nil && outAlloc.ClientStatus == clientStatus {
			return true, nil
		}
		return false, nil
	}, func(err error) {
		require.NoError(tc.cfg.T, err, "error retrieving alloc %s", err)
	})

	if outAlloc == nil {
		return fmt.Errorf("expected alloc on client %s with id %s to not be nil", clientName, alloc.ID)
	}

	if outAlloc.ClientStatus != clientStatus {
		return fmt.Errorf("expected alloc on client %s with id %s to have status %s but had %s", clientName, alloc.ID, clientStatus, outAlloc.ClientStatus)
	}

	return nil
}

func (tc *TestCluster) WaitForAllocClientStatusOnServer(alloc *structs.Allocation, clientStatus string) error {
	var err error
	var outAlloc *structs.Allocation

	testutil.WaitForResult(func() (bool, error) {
		outAlloc, err = tc.Leader.State().AllocByID(nil, alloc.ID)
		if err != nil {
			return false, err
		}
		if outAlloc != nil && outAlloc.ClientStatus == clientStatus {
			return true, nil
		}
		return false, nil
	}, func(err error) {
		require.NoError(tc.cfg.T, err, "error retrieving alloc %s", err)
	})

	if outAlloc == nil {
		return fmt.Errorf("expected alloc at server with id %s to not be nil", alloc.ID)
	}

	if outAlloc.ClientStatus != clientStatus {
		return fmt.Errorf("expected alloc at server with id %s to have status %s but had %s", alloc.ID, clientStatus, outAlloc.ClientStatus)
	}

	return nil
}

func (tc *TestCluster) WaitForAllocClientStatusOnServerByNode(alloc *structs.Allocation, clientStatus string) error {
	var err error
	var outAlloc *structs.Allocation

	testutil.WaitForResult(func() (bool, error) {
		var allocs []*structs.Allocation
		allocs, err = tc.Leader.State().AllocsByNode(nil, alloc.NodeID)
		if err != nil {
			return false, err
		}

		for _, nodeAlloc := range allocs {
			if nodeAlloc.ID == alloc.ID && nodeAlloc.ClientStatus == clientStatus {
				outAlloc = nodeAlloc
				return true, nil
			}
		}
		return false, nil
	}, func(err error) {
		require.NoError(tc.cfg.T, err, "error retrieving alloc %s", err)
	})

	if outAlloc == nil {
		return fmt.Errorf("expected alloc at server with id %s to not be nil", alloc.ID)
	}

	if outAlloc.ClientStatus != clientStatus {
		return fmt.Errorf("expected alloc at server with id %s to have status %s but had %s", alloc.ID, clientStatus, outAlloc.ClientStatus)
	}

	return nil
}

func (tc *TestCluster) WaitForJobEvalByTrigger(job *structs.Job, triggerBy string) (*structs.Evaluation, error) {
	var outEval *structs.Evaluation
	testutil.WaitForResult(func() (bool, error) {
		evals, err := tc.Leader.State().EvalsByJob(nil, job.Namespace, job.ID)
		if err != nil {
			return false, err
		}
		for _, eval := range evals {
			if eval.TriggeredBy == triggerBy {
				outEval = eval
				break
			}
		}
		if outEval == nil {
			return false, nil
		}
		return true, nil
	}, func(err error) {
		require.NoError(tc.cfg.T, err, "error retrieving evaluations %s", err)
	})

	if outEval == nil {
		return nil, fmt.Errorf("failed to find eval triggered by %s", triggerBy)
	}

	return outEval, nil
}

func (tc *TestCluster) PrintState(job *structs.Job) {
	var err error

	fmt.Printf("state for job %s\n", job.ID)

	var evals []*structs.Evaluation
	evals, err = tc.Leader.State().EvalsByJob(nil, job.Name, job.ID)
	if err != nil {
		fmt.Printf("\terror getting evals: %s\n", err)
	} else {
		fmt.Println("\tevals:")
		for _, eval := range evals {
			fmt.Printf("\t\t%#v\n", eval)
		}
	}

	deployments, err := tc.Leader.State().DeploymentsByJobID(nil, job.Namespace, job.ID, true)
	if err != nil {
		fmt.Printf("\terror getting deployments: %s\n", err)
	} else {
		fmt.Println("\tdeployments:")
		for _, deployment := range deployments {
			fmt.Printf("\t\t%#v\n", deployment)
		}
	}

	var allocs []*structs.Allocation
	allocs, err = tc.Leader.State().AllocsByJob(nil, job.Namespace, job.ID, true)
	if err != nil {
		fmt.Printf("\terror getting allocs: %s\n", err)
		return
	}

	for name, testClient := range tc.Clients {
		fmt.Printf("\tclient allocs: %s\n", name)
		for _, alloc := range allocs {
			if alloc.NodeID != testClient.NodeID() {
				continue
			}
			fmt.Printf("\t\t%s: %s - %d - %s\n", alloc.Name, alloc.ClientStatus, alloc.AllocModifyIndex, alloc.ID)
		}
		fmt.Println("")
	}
}

func (tc *TestCluster) WaitForAsExpected() error {
	var mErr *multierror.Error

	mErr = nil
	if tc.cfg.ValidateAllocs {
		for _, clientState := range tc.cfg.ExpectedAllocStates {
			if err := clientState.AsExpected(tc); err != nil {
				mErr = multierror.Append(mErr, fmt.Errorf("error validating client state for %s: \n\t%s", clientState.ClientName, err))
			}
		}
	}

	if tc.cfg.ValidateEvals {
		for _, evalState := range tc.cfg.ExpectedEvalStates {
			if err := evalState.AsExpected(tc); err != nil {
				mErr = multierror.Append(mErr, err)
			}
		}
	}

	return mErr.ErrorOrNil()
}

type ExpectedEvalState struct {
	JobID                 string
	JobNamespace          string
	TriggeredByClientName string
	TriggeredBy           string
	Count                 int
}

func (e *ExpectedEvalState) AsExpected(tc *TestCluster) error {
	nodeID := ""
	ok := false
	outCount := 0
	if e.TriggeredByClientName != "" {
		nodeID, ok = tc.NodeID(e.TriggeredByClientName)
		if !ok {
			return fmt.Errorf("error validating eval state: invalid client name %s", tc.cfg.DisconnectedClientName)
		}
	}

	testutil.WaitForResult(func() (bool, error) {
		outCount = 0
		evals, err := e.getEvals(tc)
		if err != nil {
			return false, err
		}

		for _, eval := range evals {
			if e.passesFilter(eval, nodeID) {
				outCount++
			}
		}

		if outCount == e.Count {
			return true, nil
		}

		return false, nil

	}, func(err error) {
		require.NoError(tc.cfg.T, err, "error validating evals: %s", err)
	})

	if outCount != e.Count {
		return fmt.Errorf("error validating eval state: trigger %s expected %d have %d", e.TriggeredBy, e.Count, outCount)
	}

	return nil
}

func (e *ExpectedEvalState) getEvals(tc *TestCluster) ([]*structs.Evaluation, error) {
	if e.JobID == "" || e.JobNamespace == "" {
		return nil, fmt.Errorf("error getting expected evals: invalid query constrains JobID: %s JobNamespace: %s", e.JobID, e.JobNamespace)
	}

	return tc.Leader.State().EvalsByJob(nil, e.JobNamespace, e.JobID)
}

func (e *ExpectedEvalState) passesFilter(eval *structs.Evaluation, nodeID string) bool {
	return eval.TriggeredBy == e.TriggeredBy && eval.NodeID == nodeID && eval.JobID == e.JobID
}

type ExpectedAllocState struct {
	ClientName string
	JobVersion uint64
	Failed     int
	Running    int
	Pending    int
	Stop       int
	Lost       int
}

func (ecs *ExpectedAllocState) AsExpected(tc *TestCluster) error {
	var err error
	var allocs []*structs.Allocation
	var mErr *multierror.Error

	nodeID, ok := tc.NodeID(ecs.ClientName)
	if !ok {
		return fmt.Errorf("error validating alloc state: invalid client name %s", ecs.ClientName)
	}
	testutil.WaitForResult(func() (bool, error) {
		mErr = nil
		allocs, err = tc.Leader.State().AllocsByNode(nil, nodeID)
		if err != nil {
			return false, err
		}

		failed := 0
		running := 0
		pending := 0
		stop := 0
		jobVersion := uint64(0)

		for _, alloc := range allocs {
			switch alloc.ClientStatus {
			case structs.AllocClientStatusFailed:
				failed++
			case structs.AllocClientStatusRunning:
				running++
			case structs.AllocClientStatusPending:
				pending++
			case structs.AllocClientStatusComplete:
				stop++
			}

			jobVersion += alloc.Job.Version
		}

		if failed != ecs.Failed {
			mErr = multierror.Append(mErr, fmt.Errorf("expected %d failed on %s found %d", ecs.Failed, ecs.ClientName, failed))
		}
		if running != ecs.Running {
			mErr = multierror.Append(mErr, fmt.Errorf("expected %d running on %s found %d", ecs.Running, ecs.ClientName, running))
		}
		if pending != ecs.Pending {
			mErr = multierror.Append(mErr, fmt.Errorf("expected %d pending on %s found %d", ecs.Pending, ecs.ClientName, pending))
		}
		if stop != ecs.Stop {
			mErr = multierror.Append(mErr, fmt.Errorf("expected %d stop on %s found %d", ecs.Stop, ecs.ClientName, stop))
		}
		if jobVersion != ecs.JobVersion {
			mErr = multierror.Append(mErr, fmt.Errorf("expected %d job version on %s found %d", ecs.JobVersion, ecs.ClientName, jobVersion))
		}

		if mErr == nil {
			return true, nil
		}

		return false, nil
	}, func(err error) {
		require.NoError(tc.cfg.T, err, "error validating allocs for %s: %s", ecs.ClientName, err)
	})

	return mErr.ErrorOrNil()
}
