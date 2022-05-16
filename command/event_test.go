package command

import (
	"testing"

	"context"
	"fmt"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestEventCommand_BaseCommand(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &EventCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{"-address=" + url})

	require.Equal(t, -18511, code)
}

func TestHTTP_AllocPort_Parsing(t *testing.T) {
	ci.Parallel(t)

	srv, _, _ := testServer(t, true, func(config *agent.Config) {
		config.DevMode = false
	})
	defer srv.Shutdown()
	testutil.WaitForLeader(t, srv.Agent.RPC)

	// Create a test client so that job gets allocated
	srvRPCAddr := srv.GetConfig().AdvertiseAddrs.RPC
	client, _, _ := testClient(t, "client1", newClientAgentConfigFunc("", "", srvRPCAddr))
	defer client.Shutdown()

	mockJob := mock.Job()

	mockJob.TaskGroups[0].Count = 1
	mockJob.TaskGroups[0].Networks = append(mockJob.TaskGroups[0].Networks, &structs.NetworkResource{
		Mode: "host",
		ReservedPorts: []structs.Port{
			{Label: "static", To: 9000},
		},
	})

	state := srv.Agent.Server().State()
	require.Nil(t, state.UpsertJob(structs.MsgTypeTestSetup, 100, mockJob))

	// Inject an allocation
	alloc := mock.Alloc()
	alloc.Job = mockJob
	alloc.JobID = mockJob.ID
	alloc.TaskGroup = mockJob.TaskGroups[0].Name
	alloc.Metrics = &structs.AllocMetric{}
	alloc.DesiredStatus = structs.AllocDesiredStatusRun
	alloc.ClientStatus = structs.AllocClientStatusPending

	err := state.UpsertJobSummary(101, mock.JobSummary(alloc.JobID))
	require.NoError(t, err)

	require.Nil(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 200, []*structs.Allocation{alloc}))

	alloc.ClientStatus = structs.AllocClientStatusRunning
	require.Nil(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 300, []*structs.Allocation{alloc}))

	//args := structs.JobRegisterRequest{
	//	Job: mockJob,
	//	WriteRequest: structs.WriteRequest{
	//		Region:    "global",
	//		Namespace: structs.DefaultNamespace,
	//	},
	//}
	//
	//var resp structs.JobRegisterResponse
	//err := srv.Agent.RPC("Job.Register", &args, &resp)
	//require.NoError(t, err, "failed to register job")
	//
	//// Wait for allocs to be running
	//allocsRunning := false
	//testutil.WaitForResult(func() (bool, error) {
	//	allocs, _, apiErr := client.Client().Jobs().Allocations(mockJob.ID, true, nil)
	//	if apiErr != nil {
	//		return false, apiErr
	//	}
	//	for _, alloc := range allocs {
	//		if alloc.ClientStatus == structs.AllocClientStatusRunning {
	//			allocsRunning = true
	//		}
	//	}
	//	time.Sleep(250 * time.Millisecond)
	//	return allocsRunning, nil
	//}, func(err error) {
	//	require.NoError(t, err)
	//})
	//
	//require.True(t, allocsRunning)

	// Ensure allocation gets upserted with desired status.
	running := false
	testutil.WaitForResult(func() (bool, error) {
		upsertResult, stateErr := state.AllocByID(nil, alloc.ID)
		if stateErr != nil {
			return false, stateErr
		}
		if upsertResult.ClientStatus == structs.AllocClientStatusRunning {
			running = true
			return true, nil
		}
		return false, nil
	}, func(err error) {
		require.NoError(t, err, "allocation query failed")
	})

	require.True(t, running)

	topics := map[api.Topic][]string{
		api.TopicJob: {mockJob.ID},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := srv.Client().EventStream()
	streamCh, err := events.Stream(ctx, topics, 5, nil)
	require.NoError(t, err)

	var allocEvents []api.Event
	// gather job alloc events
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-streamCh:
				if !ok {
					return
				}
				if event.IsHeartbeat() {
					continue
				}
				t.Logf("event_type: %s", event.Events[0].Type)
				allocEvents = append(allocEvents, event.Events...)
			}
		}
	}()

	var eventAlloc *api.Allocation
	testutil.WaitForResult(func() (bool, error) {
		var got string
		for _, e := range allocEvents {
			t.Logf("event_type: %s", e.Type)
			if e.Type == structs.TypeAllocationCreated || e.Type == structs.TypeAllocationUpdated {
				eventAlloc, err = e.Allocation()
				return true, nil
			}
			got = e.Type
		}
		return false, fmt.Errorf("expected to receive allocation updated event, got: %#v", got)
	}, func(e error) {
		require.NoError(t, err)
	})

	require.NotNil(t, eventAlloc)

	networkResource := eventAlloc.AllocatedResources.Tasks["web"].Networks[0]
	require.Equal(t, 9000, networkResource.ReservedPorts[0].Value)
	require.NotEqual(t, 0, networkResource.DynamicPorts[0].Value)
}
