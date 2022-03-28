package nomad

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	// nomadconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/require"
)

func TestClient_Disconnect_Reconnect(t *testing.T) {
	t.Parallel()

	testCases := []disconnectedClientTestCase{
		{
			clusterConfigFn:          reconnectRunningAllocTestConfig,
			jobFn:                    disconnectJob,
			testName:                 "reconnect-running-no-restart",
			expectedFinalAllocStatus: structs.AllocClientStatusRunning,
		},
		{
			clusterConfigFn:          reconnectFailedAllocTestConfig,
			jobFn:                    disconnectJob,
			testName:                 "reconnect-failed-alloc",
			expectedFinalAllocStatus: structs.AllocClientStatusFailed,
			includeTaskEvents:        []string{structs.TaskClientReconnected, structs.AllocClientStatusFailed},
		},
		{
			clusterConfigFn:          reconnectUpdatedJobTestConfig,
			jobFn:                    disconnectJob,
			testName:                 "reconnect-new-job-version",
			expectedFinalAllocStatus: structs.AllocClientStatusComplete,
			// TODO (derek) - Will it have both killed and terminated?
			includeTaskEvents: []string{structs.TaskClientReconnected, structs.TaskTerminated, structs.TaskKilled},
			excludeTaskEvents: []string{structs.TaskRestarting},
		},
		{
			clusterConfigFn:          reconnectRunningAllocTestConfig,
			jobFn:                    disconnectJob,
			testName:                 "reconnect-new-job-version-follow-up-eval",
			expectedFinalAllocStatus: structs.AllocClientStatusComplete,
			includeTaskEvents:        []string{structs.TaskClientReconnected},
			excludeTaskEvents:        []string{structs.TaskRestarting, structs.TaskTerminated, structs.TaskKilled},
		},
		{
			clusterConfigFn:          reconnectPendingAllocTestConfig,
			jobFn:                    disconnectJob,
			testName:                 "reconnect-pending",
			expectedFinalAllocStatus: structs.AllocClientStatusComplete,
			includeTaskEvents:        []string{structs.TaskClientReconnected, structs.TaskTerminated, structs.TaskKilled},
			excludeTaskEvents:        []string{structs.TaskRestarting, structs.TaskStateRunning},
		},
		{
			clusterConfigFn:          reconnectFollowupEvalMarksLost,
			jobFn:                    shortMaxClientDisconnectJob,
			testName:                 "reconnect-follow-up-eval-marks-lost",
			expectedFinalAllocStatus: structs.AllocClientStatusComplete,
			// TODO (derek) - Will it have both killed and terminated?
			includeTaskEvents: []string{structs.TaskClientReconnected, structs.TaskTerminated, structs.TaskKilled},
		},
		{
			clusterConfigFn:                 noMaxClientDisconnectLostConfig,
			jobFn:                           noMaxClientDisconnectJob,
			testName:                        "reconnect-no-max-client-disconnect-lost",
			expectedDisconnectedNodeStatus:  structs.NodeStatusDown,
			expectedDisconnectedAllocStatus: structs.AllocClientStatusLost,
			expectedFinalAllocStatus:        structs.AllocClientStatusLost,
			// TODO (derek) - Will it have both killed and terminated?
			includeTaskEvents: []string{structs.TaskTerminated, structs.TaskKilled},
			excludeTaskEvents: []string{structs.TaskClientReconnected},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			disconnectMain(t, tc)
		})
	}
}

const (
	whileDisconnected = "whileDisconnected"
	validateState     = "validateState"
)

type MockDisconnectedClient struct {
}

func (m *MockDisconnectedClient) UpdateStatus(_ *structs.NodeUpdateStatusRequest, _ *structs.NodeUpdateResponse) error {
	return fmt.Errorf("failing heartbeat per test config")
}

func disconnectJob(jobID string) *structs.Job {
	noRestart := &structs.RestartPolicy{
		Attempts: 0,
		Interval: 5 * time.Second,
		Mode:     structs.RestartPolicyModeFail,
	}
	// Inject mock job
	job := mock.Job()
	job.ID = jobID
	job.Constraints = []*structs.Constraint{}
	job.TaskGroups[0].MaxClientDisconnect = helper.TimeToPtr(time.Second * 30)
	job.TaskGroups[0].Count = 2
	job.TaskGroups[0].Spreads = []*structs.Spread{
		{
			Attribute:    "${node.unique.id}",
			Weight:       50,
			SpreadTarget: []*structs.SpreadTarget{},
		},
	}
	job.TaskGroups[0].RestartPolicy = noRestart
	job.TaskGroups[0].Tasks[0].RestartPolicy = noRestart
	job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "60s",
	}
	job.TaskGroups[0].Constraints = []*structs.Constraint{}
	job.TaskGroups[0].Tasks[0].Constraints = []*structs.Constraint{}

	return job
}

func shortMaxClientDisconnectJob(jobID string) *structs.Job {
	job := disconnectJob(jobID)
	job.TaskGroups[0].MaxClientDisconnect = helper.TimeToPtr(2 * time.Second)
	return job
}

func noMaxClientDisconnectJob(jobID string) *structs.Job {
	job := disconnectJob(jobID)
	job.TaskGroups[0].MaxClientDisconnect = nil
	return job
}

func defaultsDisconnectedClusterConfig(t *testing.T) *TestClusterConfig {
	return &TestClusterConfig{
		T:                      t,
		DisconnectedClientName: "client1",
		ValidateAllocs:         true,
		TestMainFn:             disconnectMain,
		ServerFns: map[string]func(*Config){
			"server1": func(c *Config) {
				c.HeartbeatGrace = 500 * time.Millisecond
				c.MaxHeartbeatsPerSecond = 1
				c.MinHeartbeatTTL = 1
				// c.ConsulConfig = &nomadconfig.ConsulConfig{}
			},
		},
		ClientFns: map[string]func(*config.Config){
			"client1": func(c *config.Config) {
				c.DevMode = true
				c.Options = make(map[string]string)
				c.Options["test.heartbeat_failer.enabled"] = "true"
				// c.ConsulConfig = &nomadconfig.ConsulConfig{}
			},
			"client2": func(c *config.Config) {
				c.DevMode = true
				// c.ConsulConfig = &nomadconfig.ConsulConfig{}
			},
		},
		PipelineFns: map[string]func(interface{}){},
	}
}

type disconnectedClientTestCase struct {
	clusterConfigFn func(t *testing.T) *TestClusterConfig
	jobFn           func(string) *structs.Job
	testName        string

	// Test case/pipeline expected values should be set on the test case.
	// Generic cluster expected values should be set up in the TestClusterConfig.
	expectedDisconnectedNodeStatus  string
	expectedDisconnectedAllocStatus string
	expectedFinalAllocStatus        string
	includeTaskEvents               []string
	excludeTaskEvents               []string
}

type disconnectedClusterState struct {
	Job   *structs.Job
	Alloc *structs.Allocation
}

func disconnectMain(t *testing.T, tci interface{}) {
	tc, ok := tci.(disconnectedClientTestCase)
	require.True(t, ok, "invalid test case: must be assignable to disconnectedClientTestCase")

	// Do common pipeline cluster function in the main TestFn.
	clusterConfig := tc.clusterConfigFn(t)
	cluster, err := NewTestCluster(clusterConfig)

	defer func() {
		cluster.Cleanup()
	}()

	err = cluster.WaitForNodeStatus("client1", structs.NodeStatusReady)
	require.NoError(t, err)
	err = cluster.WaitForNodeStatus("client2", structs.NodeStatusReady)
	require.NoError(t, err)

	state := disconnectedClusterState{
		Job: tc.jobFn(tc.testName),
	}

	var index uint64
	index, err = cluster.RegisterJob(state.Job)
	require.NoError(t, err)
	require.NotEqual(t, 0, index)

	// Check that allocs are running
	var allocs []*structs.Allocation
	allocs, err = cluster.WaitForJobAllocsRunning(state.Job, state.Job.TaskGroups[0].Count)
	require.NoError(t, err)

	// Get the alloc on the node we are going to disconnect
	nodeID, ok := cluster.NodeID("client1")
	require.True(t, ok)
	var alloc *structs.Allocation
	for _, a := range allocs {
		if a.NodeID == nodeID {
			alloc = a
			break
		}
	}

	require.NotNil(t, alloc)
	require.Equal(t, structs.AllocClientStatusRunning, alloc.ClientStatus)

	state.Alloc = alloc
	cluster.State = state

	// Execute the Pipeline function to validate the state configuration.
	validateStateFn, ok := cluster.cfg.PipelineFns[validateState]
	if ok {
		validateStateFn(cluster)
	}

	// Disconnect
	err = cluster.FailHeartbeat("client1")
	require.NoError(t, err)

	// Set default expected values if not overridden in negative tests.
	if tc.expectedDisconnectedNodeStatus == "" {
		tc.expectedDisconnectedNodeStatus = structs.NodeStatusDisconnected
	}

	if tc.expectedDisconnectedAllocStatus == "" {
		tc.expectedDisconnectedAllocStatus = structs.AllocClientStatusUnknown
	}

	// Check that node and alloc have expected status at the server
	err = cluster.WaitForNodeStatus("client1", tc.expectedDisconnectedNodeStatus)
	require.NoError(t, err)

	err = cluster.WaitForAllocClientStatusOnServer(alloc, tc.expectedDisconnectedAllocStatus)
	require.NoError(t, err)

	// Execute the Pipeline function for when the client is disconnected.
	whileDisconnectedFn, ok := cluster.cfg.PipelineFns[whileDisconnected]
	if ok {
		whileDisconnectedFn(cluster)
	}

	// Reconnect
	err = cluster.ResumeHeartbeat("client1")
	require.NoError(t, err)

	// Check that the node is reconnected
	err = cluster.WaitForNodeStatus("client1", structs.NodeStatusReady)
	require.NoError(t, err, "client1 failed to reconnect")

	// @@@@ This Becomes a new lifecycle Function
	// Check that the alloc has the expected status at the server
	// err = cluster.WaitForAllocClientStatusOnClient(alloc, "client1", tc.expectedFinalAllocStatus)
	//err = cluster.WaitForAllocClientStatusOnServer(alloc, tc.expectedFinalAllocStatus)
	//require.NoError(t, err)
	///@@@@@

	// Populate the expected eval states with dynamic values.
	for _, expected := range cluster.cfg.ExpectedEvalStates {
		expected.JobID = state.Job.ID
		expected.JobNamespace = state.Job.Namespace
	}

	err = cluster.WaitForAsExpected()
	if err != nil {
		cluster.PrintState(state.Job)
	}
	require.NoError(t, err) //, cluster.PrintState(state.Job))
}

func getDisconnectedClusterAndState(c interface{}, t *testing.T) (*TestCluster, disconnectedClusterState) {
	cluster, ok := c.(*TestCluster)
	require.True(t, ok, "invalid pipeline function: argument must be assignable to *TestCluster")
	state, ok := cluster.State.(disconnectedClusterState)
	require.True(t, ok, "invalid cluster state: must be assignable to disconnectedClusterState")
	return cluster, state
}

func reconnectRunningAllocTestConfig(t *testing.T) *TestClusterConfig {
	cfg := defaultsDisconnectedClusterConfig(t)
	cfg.ExpectedAllocStates = []*ExpectedAllocState{
		{
			ClientName: "client1",
			Failed:     0,
			Pending:    0,
			Running:    1,
			Stop:       0,
		},
		{
			ClientName: "client2",
			Failed:     0,
			Pending:    0,
			Running:    1,
			Stop:       1,
		},
	}
	cfg.ExpectedEvalStates = []*ExpectedEvalState{
		{
			TriggeredByClientName: "client1",
			TriggeredBy:           structs.EvalTriggerNodeUpdate,
			Count:                 2,
		},
		{
			TriggeredByClientName: "client1",
			TriggeredBy:           structs.EvalTriggerReconnect,
			Count:                 0,
		},
		{
			TriggeredBy: structs.EvalTriggerMaxDisconnectTimeout,
			Count:       1,
		},
	}

	return cfg
}

func reconnectFailedAllocTestConfig(t *testing.T) *TestClusterConfig {
	cfg := defaultsDisconnectedClusterConfig(t)
	cfg.ClientFns = map[string]func(*config.Config){
		"client1": func(c *config.Config) {
			c.DevMode = true
			c.Options = make(map[string]string)
			c.Options["test.alloc_failer.enabled"] = "true"
			c.Options["test.heartbeat_failer.enabled"] = "true"
			// c.ConsulConfig = &nomadconfig.ConsulConfig{}
		},
		"client2": func(c *config.Config) {
			c.DevMode = true
			// c.ConsulConfig = &nomadconfig.ConsulConfig{}
		},
	}

	cfg.ExpectedAllocStates = []*ExpectedAllocState{
		{
			ClientName: "client1",
			Failed:     1,
			Pending:    0,
			Running:    0,
			Stop:       0,
		},
		{
			ClientName: "client2",
			Failed:     0,
			Pending:    0,
			Running:    2,
			Stop:       0,
		},
	}

	cfg.ExpectedEvalStates = []*ExpectedEvalState{
		{
			TriggeredByClientName: "client1",
			TriggeredBy:           structs.EvalTriggerReconnect,
			Count:                 1,
		},
		{
			TriggeredByClientName: "client1",
			TriggeredBy:           structs.EvalTriggerMaxDisconnectTimeout,
			Count:                 1,
		},
	}

	cfg.PipelineFns[whileDisconnected] = func(c interface{}) {
		cluster, state := getDisconnectedClusterAndState(c, t)

		_, err := cluster.WaitForJobEvalByTrigger(state.Job, structs.EvalTriggerMaxDisconnectTimeout)
		require.NoError(cluster.cfg.T, err, "expected eval with trigger %s", structs.EvalTriggerMaxDisconnectTimeout)

		// Fail the task on the client so that we can test how the reconciler responds
		// after the client reconnects and the task failed during the disconnect period.
		err = cluster.FailTask("client1", state.Alloc.ID, "", "")
		require.NoError(t, err)

		// Ensure the client status is failed.
		err = cluster.WaitForAllocClientStatusOnClient(state.Alloc, "client1", structs.AllocClientStatusFailed)
		require.NoError(t, err)
	}

	return cfg
}

func reconnectUpdatedJobTestConfig(t *testing.T) *TestClusterConfig {
	cfg := defaultsDisconnectedClusterConfig(t)
	cfg.ExpectedAllocStates = []*ExpectedAllocState{
		{
			ClientName: "client1",
			Stop:       1,
		},
		{
			ClientName: "client2",
			JobVersion: 2,
			Running:    2,
			Stop:       2,
		},
	}
	cfg.ExpectedEvalStates = []*ExpectedEvalState{
		{
			TriggeredByClientName: "client1",
			TriggeredBy:           structs.EvalTriggerNodeUpdate,
			Count:                 2,
		},
		{
			TriggeredByClientName: "client1",
			TriggeredBy:           structs.EvalTriggerReconnect,
			Count:                 0,
		},
		{
			TriggeredBy: structs.EvalTriggerMaxDisconnectTimeout,
			Count:       1,
		},
	}

	cfg.PipelineFns[whileDisconnected] = func(c interface{}) {
		cluster, state := getDisconnectedClusterAndState(c, t)

		_, err := cluster.WaitForJobEvalByTrigger(state.Job, structs.EvalTriggerMaxDisconnectTimeout)
		require.NoError(cluster.cfg.T, err, "expected eval with trigger %s", structs.EvalTriggerMaxDisconnectTimeout)

		job := *state.Job
		updatedJob := job.Copy()
		// Make a change that can't be done in-place
		// updatedJob.TaskGroups[0].Spreads[0].Weight = job.TaskGroups[0].Spreads[0].Weight + 10
		updatedJob.TaskGroups[0].Networks = []*structs.NetworkResource{
			{
				Mode: "host",
				DynamicPorts: []structs.Port{
					{Label: "grpc"},
					{Label: "http"},
					{Label: "admin"},
				},
			},
		}

		newIndex, err := cluster.RegisterJob(updatedJob)
		require.NoError(cluster.cfg.T, err)
		require.NotEqual(cluster.cfg.T, job.JobModifyIndex, newIndex)

		// Ensure the new job is running
		_, err = cluster.WaitForJobAllocsRunning(updatedJob, updatedJob.TaskGroups[0].Count)
		require.NoError(t, err)

		state.Job = updatedJob
		cluster.State = state

		cluster.PrintState(updatedJob)
	}

	return cfg
}

func reconnectPendingAllocTestConfig(t *testing.T) *TestClusterConfig {
	return &TestClusterConfig{
		T:                      t,
		DisconnectedClientName: "client1",
		ServerFns: map[string]func(*Config){
			"server1": func(c *Config) {
				c.HeartbeatGrace = 500 * time.Millisecond
				c.MaxHeartbeatsPerSecond = 1
				c.MinHeartbeatTTL = 1
				// c.ConsulConfig = &nomadconfig.ConsulConfig{}
			},
		},
		ClientFns: map[string]func(*config.Config){
			"client1": func(c *config.Config) {
				c.DevMode = true
				c.Options = make(map[string]string)
				c.Options["test.heartbeat_failer.enabled"] = "true"
				// c.ConsulConfig = &nomadconfig.ConsulConfig{}
			},
			"client2": func(c *config.Config) {
				c.DevMode = true
				// c.ConsulConfig = &nomadconfig.ConsulConfig{}
			},
		},
		ExpectedAllocStates: []*ExpectedAllocState{
			{
				ClientName: "client1",
				Failed:     0,
				Pending:    0,
				Running:    0,
				Stop:       1,
			},
			{
				ClientName: "client2",
				Failed:     0,
				Pending:    0,
				Running:    2,
				Stop:       2,
			},
		},
		ExpectedEvalStates: []*ExpectedEvalState{
			{
				TriggeredByClientName: "client1",
				TriggeredBy:           structs.EvalTriggerReconnect,
				Count:                 1,
			},
			{
				TriggeredByClientName: "client1",
				TriggeredBy:           structs.EvalTriggerMaxDisconnectTimeout,
				Count:                 1,
			},
		},
	}
}

func reconnectFollowupEvalMarksLost(t *testing.T) *TestClusterConfig {
	cfg := defaultsDisconnectedClusterConfig(t)
	cfg.ExpectedAllocStates = []*ExpectedAllocState{
		{
			ClientName: "client1",
			Stop:       1,
		},
		{
			ClientName: "client2",
			Running:    2,
		},
	}
	cfg.ExpectedEvalStates = []*ExpectedEvalState{
		{
			TriggeredByClientName: "client1",
			TriggeredBy:           structs.EvalTriggerNodeUpdate,
			Count:                 2,
		},
		{
			TriggeredByClientName: "client1",
			TriggeredBy:           structs.EvalTriggerReconnect,
			Count:                 0,
		},
		{
			TriggeredBy: structs.EvalTriggerMaxDisconnectTimeout,
			Count:       1,
		},
	}

	cfg.PipelineFns[whileDisconnected] = func(c interface{}) {
		time.Sleep(3 * time.Second)
	}

	return cfg
}

func noMaxClientDisconnectLostConfig(t *testing.T) *TestClusterConfig {
	cfg := defaultsDisconnectedClusterConfig(t)

	cfg.ExpectedAllocStates = []*ExpectedAllocState{
		{
			ClientName: "client1",
			Stop:       1,
		},
		{
			ClientName: "client2",
			Running:    2,
		},
	}

	cfg.ExpectedEvalStates = []*ExpectedEvalState{
		{
			TriggeredByClientName: "client1",
			TriggeredBy:           structs.EvalTriggerNodeUpdate,
			Count:                 1,
		},
	}

	cfg.PipelineFns[validateState] = func(c interface{}) {
		cluster, state := getDisconnectedClusterAndState(c, t)
		require.Nil(cluster.cfg.T, state.Job.TaskGroups[0].MaxClientDisconnect, "error validating state: expected MaxClientDisconnect to be nil")
	}

	return cfg
}
