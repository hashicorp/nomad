package consultemplate

import (
	"time"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"

	. "github.com/onsi/gomega"
)

type ConsulTemplateTest struct {
	framework.TC
	jobIds []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Consul Template",
		CanRunLocal: true,
		Consul:      true,
		Cases: []framework.TestCase{
			new(ConsulTemplateTest),
		},
	})
}

func (tc *ConsulTemplateTest) TestUpdatesRestartTasks(f *framework.F) {
	require := require.New(f.T())
	g := NewGomegaWithT(f.T())

	nomadClient := tc.Nomad()
	consulClient := tc.Consul()

	// Ensure consultemplatetest does not exist
	_, err := consulClient.KV().Delete("consultemplatetest", nil)
	require.NoError(err)

	// Parse job
	job, err := jobspec.ParseFile("consultemplate/input/docker.nomad")
	require.Nil(err)
	uuid := uuid.Generate()
	jobId := helper.StringToPtr("cltp" + uuid[:8])
	job.ID = jobId

	tc.jobIds = append(tc.jobIds, *jobId)

	// Register job
	jobs := nomadClient.Jobs()
	resp, _, err := jobs.Register(job, nil)
	require.Nil(err)
	require.NotEmpty(resp.EvalID)

	waitForTaskState := func(taskState string) {
		g.Eventually(func() string {
			allocs, _, _ := jobs.Allocations(*job.ID, false, nil)
			if len(allocs) != 1 {
				return ""
			}
			first := allocs[0]
			taskState := first.TaskStates["test"]
			if taskState == nil {
				return ""
			}

			return taskState.State
		}, 5*time.Second, time.Second).Should(Equal(taskState), "Incorrect task state")
	}

	waitForClientAllocStatus := func(allocState string) {
		g.Eventually(func() string {
			allocSummaries, _, _ := jobs.Allocations(*job.ID, false, nil)
			if len(allocSummaries) != 1 {
				return ""
			}

			alloc, _, _ := nomadClient.Allocations().Info(allocSummaries[0].ID, nil)
			if alloc == nil {
				return ""
			}

			return alloc.ClientStatus
		}, 5*time.Second, time.Second).Should(Equal(allocState), "Incorrect alloc state")
	}

	waitForRestartCount := func(count uint64) {
		g.Eventually(func() uint64 {
			allocs, _, _ := jobs.Allocations(*job.ID, false, nil)
			if len(allocs) != 1 {
				return 0
			}
			first := allocs[0]
			return first.TaskStates["test"].Restarts
		}, 10*time.Second, time.Second).Should(Equal(count), "Incorrect restart count")
	}

	// Wrap in retry to wait until placement
	waitForTaskState(structs.TaskStatePending)

	// Client should be pending
	waitForClientAllocStatus(structs.AllocClientStatusPending)

	// Alloc should have a blocked event
	g.Eventually(func() []string {
		allocSummaries, _, _ := jobs.Allocations(*job.ID, false, nil)
		events := allocSummaries[0].TaskStates["test"].Events
		messages := []string{}
		for _, event := range events {
			messages = append(messages, event.DisplayMessage)
		}

		return messages
	}, 5*time.Second, time.Second).Should(ContainElement(ContainSubstring("kv.block")))

	// Insert consultemplatetest
	_, err = consulClient.KV().Put(&capi.KVPair{Key: "consultemplatetest", Value: []byte("bar")}, nil)
	require.Nil(err)

	// Placement should start running
	waitForClientAllocStatus(structs.AllocClientStatusRunning)

	// Ensure restart count 0 -- we should be going from blocked to running.
	waitForRestartCount(0)

	// Update consultemplatetest
	_, err = consulClient.KV().Put(&capi.KVPair{Key: "consultemplatetest", Value: []byte("baz")}, nil)
	require.Nil(err)

	// Wrap in retry to wait until restart
	waitForRestartCount(1)
}

func (tc *ConsulTemplateTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	consulClient := tc.Consul()

	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.jobIds {
		jobs.Deregister(id, true, nil)
	}
	// Garbage collect
	nomadClient.System().GarbageCollect()

	// Ensure consultemplatetest does not exist
	consulClient.KV().Delete("consultemplatetest", nil)
}
