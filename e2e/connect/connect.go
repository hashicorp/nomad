package connect

import (
	"os"
	"strings"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

type ConnectE2ETest struct {
	framework.TC
	jobIds []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Connect",
		CanRunLocal: true,
		Consul:      true,
		Cases: []framework.TestCase{
			new(ConnectE2ETest),
			new(ConnectClientStateE2ETest),
		},
	})
}

func (tc *ConnectE2ETest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 2)
}

func (tc *ConnectE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, id := range tc.jobIds {
		tc.Nomad().Jobs().Deregister(id, true, nil)
	}
	tc.jobIds = []string{}
	tc.Nomad().System().GarbageCollect()
}

// TestConnectDemo tests the demo job file from the Consul Connect Technology
// Preview.
//
// https://github.com/hashicorp/nomad/blob/v0.9.5/website/source/guides/integrations/consul-connect/index.html.md#run-the-connect-enabled-services
//
func (tc *ConnectE2ETest) TestConnectDemo(f *framework.F) {
	t := f.T()
	uuid := uuid.Generate()
	jobID := "connect" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobID)
	jobapi := tc.Nomad().Jobs()

	job, err := jobspec.ParseFile("connect/input/demo.nomad")
	require.NoError(t, err)
	job.ID = &jobID

	resp, _, err := jobapi.Register(job, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Zero(t, resp.Warnings)

EVAL:
	qopts := &api.QueryOptions{
		WaitIndex: resp.EvalCreateIndex,
	}
	evalapi := tc.Nomad().Evaluations()
	eval, qmeta, err := evalapi.Info(resp.EvalID, qopts)
	require.NoError(t, err)
	qopts.WaitIndex = qmeta.LastIndex

	switch eval.Status {
	case "pending":
		goto EVAL
	case "complete":
		// Ok!
	case "failed", "canceled", "blocked":
		t.Fatalf("eval %s\n%s\n", eval.Status, pretty.Sprint(eval))
	default:
		t.Fatalf("unknown eval status: %s\n%s\n", eval.Status, pretty.Sprint(eval))
	}

	// Assert there were 0 placement failures
	require.Zero(t, eval.FailedTGAllocs, pretty.Sprint(eval.FailedTGAllocs))
	require.Len(t, eval.QueuedAllocations, 2, pretty.Sprint(eval.QueuedAllocations))

	// Assert allocs are running
	for i := 0; i < 20; i++ {
		allocs, qmeta, err := evalapi.Allocations(eval.ID, qopts)
		require.NoError(t, err)
		require.Len(t, allocs, 2)
		qopts.WaitIndex = qmeta.LastIndex

		running := 0
		for _, alloc := range allocs {
			switch alloc.ClientStatus {
			case "running":
				running++
			case "pending":
				// keep trying
			default:
				t.Fatalf("alloc failed: %s", pretty.Sprint(alloc))
			}
		}

		if running == len(allocs) {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	allocs, _, err := evalapi.Allocations(eval.ID, qopts)
	require.NoError(t, err)
	allocIDs := make(map[string]bool, 2)
	for _, a := range allocs {
		if a.ClientStatus != "running" || a.DesiredStatus != "run" {
			t.Fatalf("alloc %s (%s) terminal; client=%s desired=%s", a.TaskGroup, a.ID, a.ClientStatus, a.DesiredStatus)
		}
		allocIDs[a.ID] = true
	}

	// Check Consul service health
	agentapi := tc.Consul().Agent()

	failing := map[string]*consulapi.AgentCheck{}
	for i := 0; i < 60; i++ {
		checks, err := agentapi.Checks()
		require.NoError(t, err)

		// Filter out checks for other services
		for cid, check := range checks {
			found := false
			for allocID := range allocIDs {
				if strings.Contains(check.ServiceID, allocID) {
					found = true
					break
				}
			}

			if !found {
				delete(checks, cid)
			}
		}

		// Ensure checks are all passing
		failing = map[string]*consulapi.AgentCheck{}
		for _, check := range checks {
			if check.Status != "passing" {
				failing[check.CheckID] = check
				break
			}
		}

		if len(failing) == 0 {
			break
		}

		t.Logf("still %d checks not passing", len(failing))

		time.Sleep(time.Second)
	}

	require.Len(t, failing, 0, pretty.Sprint(failing))
}
