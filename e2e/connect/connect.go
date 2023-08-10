// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"os"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
)

const (
	// envConsulToken is the consul http token environment variable
	envConsulToken = "CONSUL_HTTP_TOKEN"

	// demoConnectJob is the example connect enabled job useful for testing
	demoConnectJob = "connect/input/demo.nomad"

	// demoConnectCustomProxyExposed is a connect job with custom sidecar_task
	// that also uses the expose check feature.
	demoConnectCustomProxyExposed = "connect/input/expose-custom.nomad"

	// demoConnectNativeJob is the example connect native enabled job useful for testing
	demoConnectNativeJob = "connect/input/native-demo.nomad"

	// demoConnectIngressGateway is the example ingress gateway job useful for testing
	demoConnectIngressGateway = "connect/input/ingress-gateway.nomad"

	// demoConnectMultiIngressGateway is the example multi ingress gateway job useful for testing
	demoConnectMultiIngressGateway = "connect/input/multi-ingress.nomad"

	// demoConnectTerminatingGateway is the example terminating gateway job useful for testing
	demoConnectTerminatingGateway = "connect/input/terminating-gateway.nomad"
)

type ConnectE2ETest struct {
	framework.TC
	jobIds []string
}

func init() {
	// Connect tests without Consul ACLs enabled.
	framework.AddSuites(&framework.TestSuite{
		Component:   "Connect",
		CanRunLocal: true,
		Consul:      true,
		Cases: []framework.TestCase{
			new(ConnectE2ETest),
			new(ConnectClientStateE2ETest),
		},
	})

	framework.AddSuites(&framework.TestSuite{
		Component:   "ConnectACLs",
		CanRunLocal: false,
		Consul:      true,
		Parallel:    false,
		Cases: []framework.TestCase{
			new(ConnectACLsE2ETest),
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

func connectJobID() string {
	return "connect" + uuid.Generate()[0:8]
}

// TestConnectDemo tests the demo job file used in Connect Integration examples.
func (tc *ConnectE2ETest) TestConnectDemo(f *framework.F) {
	t := f.T()

	jobID := connectJobID()
	tc.jobIds = append(tc.jobIds, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), demoConnectJob, jobID, "")
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), allocIDs)
}

// TestConnectCustomSidecarExposed tests that a connect sidecar with custom task
// definition can also make use of the expose service check feature.
func (tc *ConnectE2ETest) TestConnectCustomSidecarExposed(f *framework.F) {
	t := f.T()

	jobID := connectJobID()
	tc.jobIds = append(tc.jobIds, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), demoConnectCustomProxyExposed, jobID, "")
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), allocIDs)
}

// TestConnectNativeDemo tests the demo job file used in Connect Native Integration examples.
func (tc *ConnectE2ETest) TestConnectNativeDemo(f *framework.F) {
	t := f.T()

	jobID := connectJobID()
	tc.jobIds = append(tc.jobIds, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), demoConnectNativeJob, jobID, "")
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), allocIDs)
}

func (tc *ConnectE2ETest) TestConnectIngressGatewayDemo(f *framework.F) {
	t := f.T()

	jobID := connectJobID()
	tc.jobIds = append(tc.jobIds, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), demoConnectIngressGateway, jobID, "")
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), allocIDs)
}

func (tc *ConnectE2ETest) TestConnectMultiIngressGatewayDemo(f *framework.F) {
	t := f.T()

	jobID := connectJobID()
	tc.jobIds = append(tc.jobIds, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), demoConnectMultiIngressGateway, jobID, "")

	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), allocIDs)
}

func (tc *ConnectE2ETest) TestConnectTerminatingGatewayDemo(f *framework.F) {

	t := f.T()

	jobID := connectJobID()
	tc.jobIds = append(tc.jobIds, jobID)

	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), demoConnectTerminatingGateway, jobID, "")

	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsRunning(t, tc.Nomad(), allocIDs)
}
