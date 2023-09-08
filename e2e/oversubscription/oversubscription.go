// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oversubscription

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
)

type OversubscriptionTest struct {
	framework.TC
	jobIDs                 []string
	initialSchedulerConfig *api.SchedulerConfiguration
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "oversubscription",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(OversubscriptionTest),
		},
	})
}

func (tc *OversubscriptionTest) BeforeAll(f *framework.F) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)

	tc.enableMemoryOversubscription(f)
}

func (tc *OversubscriptionTest) AfterAll(f *framework.F) {
	tc.restoreSchedulerConfig(f)
}

func (tc *OversubscriptionTest) enableMemoryOversubscription(f *framework.F) {
	resp, _, err := tc.Nomad().Operator().SchedulerGetConfiguration(nil)
	f.NoError(err)

	tc.initialSchedulerConfig = resp.SchedulerConfig

	conf := *resp.SchedulerConfig
	conf.MemoryOversubscriptionEnabled = true
	_, _, err = tc.Nomad().Operator().SchedulerSetConfiguration(&conf, nil)
	f.NoError(err)
}

func (tc *OversubscriptionTest) restoreSchedulerConfig(f *framework.F) {
	if tc.initialSchedulerConfig != nil {
		_, _, err := tc.Nomad().Operator().SchedulerSetConfiguration(tc.initialSchedulerConfig, nil)
		f.NoError(err)
	}
}

func (tc *OversubscriptionTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	j := nomadClient.Jobs()

	for _, id := range tc.jobIDs {
		j.Deregister(id, true, nil)
	}
	tc.Nomad().System().GarbageCollect()
}

func (tc *OversubscriptionTest) TestDocker(f *framework.F) {
	alloc := tc.runTest(f, "oversubscription-docker-", "docker.nomad")

	// check that cgroup reports the memoryMaxMB as the limit within he container
	stdout, err := e2eutil.AllocLogs(alloc.ID, "", e2eutil.LogsStdOut)
	f.NoError(err)
	f.Equal(fmt.Sprintf("%d\n", 30*1024*1024), stdout)
}

func (tc *OversubscriptionTest) TestExec(f *framework.F) {
	alloc := tc.runTest(f, "oversubscription-exec-", "exec.nomad")

	// check the the cgroup is configured with the memoryMaxMB
	var err error
	expected := fmt.Sprintf("%d\n", 30*1024*1024)
	e2eutil.WaitForAllocFile(alloc.ID, "/alloc/tmp/memory.limit_in_bytes", func(s string) bool {
		if s != expected {
			err = fmt.Errorf("expected %v got %v", expected, s)
			return false
		}
		err = nil
		return true
	}, nil)
	f.NoError(err)
}

func (tc *OversubscriptionTest) runTest(f *framework.F, jobPrefix, jobfile string) *api.Allocation {
	// register a job
	jobID := jobPrefix + uuid.Generate()[:8]
	tc.jobIDs = append(tc.jobIDs, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), tc.Nomad(), "oversubscription/testdata/"+jobfile, jobID, "")
	f.Len(allocs, 1)

	e2eutil.WaitForAllocRunning(f.T(), tc.Nomad(), allocs[0].ID)

	alloc, _, err := tc.Nomad().Allocations().Info(allocs[0].ID, nil)
	f.NoError(err)

	// assert the resources info
	resources := alloc.AllocatedResources.Tasks["task"]
	f.Equal(int64(20), resources.Memory.MemoryMB)
	f.Equal(int64(30), resources.Memory.MemoryMaxMB)

	// assert the status API reports memory, we need to wait for the
	// for metrics to be written before we can assert the entire
	// command line
	var allocInfo string
	f.Eventually(func() bool {
		allocInfo, err = e2eutil.Command("nomad", "alloc", "status", alloc.ID)
		if err != nil {
			return false
		}
		return strings.Contains(allocInfo, "/20 MiB") && // memory reserve
			strings.Contains(allocInfo, "Max: 30 MiB") // memory max
	}, 10*time.Second, 200*time.Millisecond, "unexpected memory output")

	return alloc
}
