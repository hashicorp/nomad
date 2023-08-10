// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"os"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
)

const (
	consulNamespace = "default"
)

type ConnectClientStateE2ETest struct {
	framework.TC
	jobIDs []string
}

func (tc *ConnectClientStateE2ETest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *ConnectClientStateE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, id := range tc.jobIDs {
		tc.Nomad().Jobs().Deregister(id, true, nil)
	}
	tc.jobIDs = []string{}
	tc.Nomad().System().GarbageCollect()
}

func (tc *ConnectClientStateE2ETest) TestClientRestart(f *framework.F) {
	t := f.T()

	jobID := "connect" + uuid.Generate()[0:8]
	tc.jobIDs = append(tc.jobIDs, jobID)
	client := tc.Nomad()
	consulClient := tc.Consul()

	allocs := e2eutil.RegisterAndWaitForAllocs(t, client,
		"connect/input/demo.nomad", jobID, "")
	f.Equal(2, len(allocs))

	e2eutil.RequireConsulStatus(f.Assertions, consulClient, consulNamespace, "count-api-sidecar-proxy", capi.HealthPassing)
	nodeID := allocs[0].NodeID

	restartID, err := e2eutil.AgentRestart(client, nodeID)
	if restartID != "" {
		tc.jobIDs = append(tc.jobIDs, restartID)
	}
	if err != nil {
		t.Skip("node cannot be restarted", err)
	}

	e2eutil.RequireConsulStatus(f.Assertions, consulClient, consulNamespace, "count-api-sidecar-proxy", capi.HealthPassing)
}
