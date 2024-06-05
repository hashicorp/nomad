// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"testing"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestConnect_ClientRestart(t *testing.T) {
	t.Skip("skipping test that does nomad agent restart")

	nomadClient := e2eutil.NomadClient(t)
	e2eutil.WaitForLeader(t, nomadClient)
	e2eutil.WaitForNodesReady(t, nomadClient, 2)

	sub, cleanup := jobs3.Submit(t, "./input/demo.nomad")
	t.Cleanup(cleanup)

	cc := e2eutil.ConsulClient(t)

	ixn := &capi.Intention{
		SourceName:      "count-dashboard",
		DestinationName: "count-api",
		Action:          "allow",
	}
	_, err := cc.Connect().IntentionUpsert(ixn, nil)
	must.NoError(t, err, must.Sprint("could not create intention"))

	t.Cleanup(func() {
		_, err := cc.Connect().IntentionDeleteExact("count-dashboard", "count-api", nil)
		test.NoError(t, err)
	})

	assertServiceOk(t, cc, "count-api-sidecar-proxy")
	assertServiceOk(t, cc, "count-dashboard-sidecar-proxy")

	nodeID := sub.Allocs()[0].NodeID
	_, err = e2eutil.AgentRestart(nomadClient, nodeID)
	must.Error(t, err, must.Sprint("node cannot be restarted"))

	assertServiceOk(t, cc, "count-api-sidecar-proxy")
	assertServiceOk(t, cc, "count-dashboard-sidecar-proxy")
}
