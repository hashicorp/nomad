package allocstats

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"

	"github.com/stretchr/testify/require"
)

type BasicAllocStatsTest struct {
	framework.TC
	jobIds []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "AllocationStats",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(BasicAllocStatsTest),
		},
	})
}

func (tc *BasicAllocStatsTest) BeforeAll(f *framework.F) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	// Ensure that we have at least one client node in ready state
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

// TestResourceStats is an end to end test for resource utilization
// This runs a raw exec job.
// TODO(preetha) - add more test cases with more realistic resource utilization
func (tc *BasicAllocStatsTest) TestResourceStats(f *framework.F) {
	nomadClient := tc.Nomad()

	clientNodes, err := e2eutil.ListLinuxClientNodes(nomadClient)
	if err != nil {
		f.T().Fatalf("could not list client nodes: %v", err)
	}
	if len(clientNodes) == 0 {
		f.T().Skip("no Linux clients")
	}

	uuid := uuid.Generate()
	jobID := "allocstats" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobID)
	runResourceStatsTest(f, nomadClient, jobID,
		"allocstats/input/raw_exec.nomad")
}

// TestResourceStatsWindows is an end to end test for resource utilization.
// This runs a raw exec job.
func (tc *BasicAllocStatsTest) TestResourceStatsWindows(f *framework.F) {
	nomadClient := tc.Nomad()

	clientNodes, err := e2eutil.ListWindowsClientNodes(nomadClient)
	if err != nil {
		f.T().Fatalf("could not list client nodes: %v", err)
	}
	if len(clientNodes) == 0 {
		f.T().Skip("no Windows clients")
	}

	uuid := uuid.Generate()
	jobID := "allocstats_windows" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobID)
	runResourceStatsTest(f, nomadClient, jobID,
		"allocstats/input/raw_exec_windows.nomad")
}

func runResourceStatsTest(f *framework.F, nomadClient *api.Client, jobID, jobSpec string) {

	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, jobSpec, jobID)

	require := require.New(f.T())
	require.Len(allocs, 1)

	// Wait till alloc is running
	allocID := allocs[0].ID
	e2eutil.WaitForAllocRunning(f.T(), nomadClient, allocID)

	allocsClient := nomadClient.Allocations()

	// Verify allocation resource stats
	// This job file should result in non zero CPU and Memory stats
	testutil.WaitForResultRetries(500, func() (bool, error) {
		time.Sleep(time.Millisecond * 100)
		allocStatsResp, err := allocsClient.Stats(&api.Allocation{ID: allocID}, nil)
		if err != nil {
			return false, fmt.Errorf("unexpected error getting alloc stats: %v", err)
		}
		resourceUsage := allocStatsResp.ResourceUsage
		cpuStatsValid := resourceUsage.CpuStats.TotalTicks > 0 && resourceUsage.CpuStats.Percent > 0
		memStatsValid := resourceUsage.MemoryStats.RSS > 0
		return cpuStatsValid && memStatsValid, fmt.Errorf("expected non zero resource usage, but was: %v", resourceUsage)
	}, func(err error) {
		f.T().Fatalf("invalid resource usage : %v", err)
	})
}

func (tc *BasicAllocStatsTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.jobIds {
		jobs.Deregister(id, true, nil)
	}
	// Garbage collect
	nomadClient.System().GarbageCollect()
}
