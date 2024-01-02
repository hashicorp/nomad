// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"os"
	"sort"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper"
	"github.com/stretchr/testify/require"
)

// Job files used to test Consul Namespaces. Each job should run on Nomad OSS
// and Nomad ENT with expectations set accordingly.
//
// All tests require Consul Enterprise.
const (
	cnsJobGroupServices      = "consul/input/namespaces/services_group.nomad"
	cnsJobTaskServices       = "consul/input/namespaces/services_task.nomad"
	cnsJobTemplateKV         = "consul/input/namespaces/template_kv.nomad"
	cnsJobConnectSidecars    = "consul/input/namespaces/connect_sidecars.nomad"
	cnsJobConnectIngress     = "consul/input/namespaces/connect_ingress.nomad"
	cnsJobConnectTerminating = "consul/input/namespaces/connect_terminating.nomad"
	cnsJobScriptChecksTask   = "consul/input/namespaces/script_checks_task.nomad"
	cnsJobScriptChecksGroup  = "consul/input/namespaces/script_checks_group.nomad"
)

var (
	// consulNamespaces represents the custom consul namespaces we create and
	// can make use of in tests, but usefully so only in Nomad Enterprise
	consulNamespaces = []string{"apple", "banana", "cherry"}

	// allConsulNamespaces represents all namespaces we expect in consul after
	// creating consulNamespaces, which then includes "default", which is the
	// only namespace accessed by Nomad OSS (outside of agent configuration)
	allConsulNamespaces = append(consulNamespaces, "default")
)

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "ConsulNamespaces",
		CanRunLocal: true,
		Consul:      true,
		Cases: []framework.TestCase{
			new(ConsulNamespacesE2ETest),
		},
	})
}

type ConsulNamespacesE2ETest struct {
	framework.TC

	jobIDs []string

	// cToken contains the Consul global-management token
	cToken string

	// created policy and token IDs should be set here so they can be cleaned
	// up after each test case, organized by namespace
	policyIDs map[string][]string
	tokenIDs  map[string][]string
}

func (tc *ConsulNamespacesE2ETest) BeforeAll(f *framework.F) {
	tc.policyIDs = make(map[string][]string)
	tc.tokenIDs = make(map[string][]string)

	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)

	tc.cToken = os.Getenv("CONSUL_HTTP_TOKEN")

	// create a set of consul namespaces in which to register services
	e2eutil.CreateConsulNamespaces(f.T(), tc.Consul(), consulNamespaces)

	// insert a key of the same name into KV for each namespace, where the value
	// contains the namespace name making it easy to determine which namespace
	// consul template actually accessed
	for _, namespace := range allConsulNamespaces {
		value := fmt.Sprintf("ns_%s", namespace)
		e2eutil.PutConsulKey(f.T(), tc.Consul(), namespace, "ns-kv-example", value)
	}
}

func (tc *ConsulNamespacesE2ETest) AfterAll(f *framework.F) {
	e2eutil.DeleteConsulNamespaces(f.T(), tc.Consul(), consulNamespaces)
}

func (tc *ConsulNamespacesE2ETest) TestNamespacesExist(f *framework.F) {
	// make sure our namespaces exist + default
	namespaces := e2eutil.ListConsulNamespaces(f.T(), tc.Consul())
	require.True(f.T(), helper.SliceSetEq(namespaces, append(consulNamespaces, "default")))
}

func (tc *ConsulNamespacesE2ETest) testConsulRegisterGroupServices(f *framework.F, token, nsA, nsB, nsC, nsZ string) {
	nomadClient := tc.Nomad()
	jobID := "cns-group-services"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs
	allocations := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, cnsJobGroupServices, jobID, token)
	require.Len(f.T(), allocations, 3)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(f.T(), tc.Nomad(), allocIDs)

	r := f.Assertions
	c := tc.Consul()

	// Verify services with namespace set are registered into expected namespaces
	e2eutil.RequireConsulRegistered(r, c, nsB, "b1", 1)
	e2eutil.RequireConsulRegistered(r, c, nsB, "b2", 1)
	e2eutil.RequireConsulRegistered(r, c, nsC, "c1", 1)
	e2eutil.RequireConsulRegistered(r, c, nsC, "c2", 1)

	// Verify services without namespace set are registered into default
	e2eutil.RequireConsulRegistered(r, c, nsZ, "z1", 1)
	e2eutil.RequireConsulRegistered(r, c, nsZ, "z2", 1)

	// Verify our services are all healthy
	e2eutil.RequireConsulStatus(r, c, nsB, "b1", "passing")
	e2eutil.RequireConsulStatus(r, c, nsB, "b2", "passing")
	e2eutil.RequireConsulStatus(r, c, nsC, "c1", "passing")
	e2eutil.RequireConsulStatus(r, c, nsC, "c2", "passing")
	e2eutil.RequireConsulStatus(r, c, nsZ, "z1", "passing")
	e2eutil.RequireConsulStatus(r, c, nsZ, "z2", "passing")

	// Stop the job
	e2eutil.WaitForJobStopped(f.T(), nomadClient, jobID)

	// Verify that services were de-registered from Consul
	e2eutil.RequireConsulDeregistered(r, c, nsB, "b1")
	e2eutil.RequireConsulDeregistered(r, c, nsB, "b2")
	e2eutil.RequireConsulDeregistered(r, c, nsC, "c1")
	e2eutil.RequireConsulDeregistered(r, c, nsC, "c2")
	e2eutil.RequireConsulDeregistered(r, c, nsZ, "z1")
	e2eutil.RequireConsulDeregistered(r, c, nsZ, "z2")
}

func (tc *ConsulNamespacesE2ETest) testConsulRegisterTaskServices(f *framework.F, token, nsA, nsB, nsC, nsZ string) {
	nomadClient := tc.Nomad()
	jobID := "cns-task-services"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs
	allocations := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, cnsJobTaskServices, jobID, token)
	require.Len(f.T(), allocations, 3)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(f.T(), tc.Nomad(), allocIDs)

	r := f.Assertions
	c := tc.Consul()

	// Verify our services were registered into expected namespaces
	e2eutil.RequireConsulRegistered(r, c, nsB, "b1", 1)
	e2eutil.RequireConsulRegistered(r, c, nsB, "b2", 1)
	e2eutil.RequireConsulRegistered(r, c, nsC, "c1", 1)
	e2eutil.RequireConsulRegistered(r, c, nsC, "c2", 1)
	e2eutil.RequireConsulRegistered(r, c, nsZ, "z1", 1)
	e2eutil.RequireConsulRegistered(r, c, nsZ, "z2", 1)

	// Verify our services are all healthy
	e2eutil.RequireConsulStatus(r, c, nsB, "b1", "passing")
	e2eutil.RequireConsulStatus(r, c, nsB, "b2", "passing")
	e2eutil.RequireConsulStatus(r, c, nsC, "c1", "passing")
	e2eutil.RequireConsulStatus(r, c, nsC, "c2", "passing")
	e2eutil.RequireConsulStatus(r, c, nsZ, "z1", "passing")
	e2eutil.RequireConsulStatus(r, c, nsZ, "z2", "passing")

	// Stop the job
	e2eutil.WaitForJobStopped(f.T(), nomadClient, jobID)

	// Verify that services were de-registered from Consul
	e2eutil.RequireConsulDeregistered(r, c, nsB, "b1")
	e2eutil.RequireConsulDeregistered(r, c, nsB, "b2")
	e2eutil.RequireConsulDeregistered(r, c, nsC, "c1")
	e2eutil.RequireConsulDeregistered(r, c, nsC, "c2")
	e2eutil.RequireConsulDeregistered(r, c, nsZ, "z1")
	e2eutil.RequireConsulDeregistered(r, c, nsZ, "z2")
}

func (tc *ConsulNamespacesE2ETest) testConsulTemplateKV(f *framework.F, token, expB, expZ string) {
	t := f.T()
	nomadClient := tc.Nomad()
	jobID := "cns-template-kv"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs to complete
	allocations := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, cnsJobTemplateKV, jobID, token)
	require.Len(t, allocations, 2)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsStopped(f.T(), tc.Nomad(), allocIDs)

	// Sort allocs by name
	sort.Sort(e2eutil.AllocsByName(allocations))

	// Check template read from expected namespace when namespace set
	textB, err := e2eutil.AllocTaskLogs(allocations[0].ID, "task-b", e2eutil.LogsStdOut)
	require.NoError(t, err)
	require.Equal(t, expB, textB)

	// Check template read from default namespace if no namespace set
	textZ, err := e2eutil.AllocTaskLogs(allocations[1].ID, "task-z", e2eutil.LogsStdOut)
	require.NoError(t, err)
	require.Equal(t, expZ, textZ)

	//  Stop the job
	e2eutil.WaitForJobStopped(t, nomadClient, jobID)
}

func (tc *ConsulNamespacesE2ETest) testConsulConnectSidecars(f *framework.F, token, nsA, nsZ string) {
	nomadClient := tc.Nomad()
	jobID := "cns-connect-sidecars"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs
	allocations := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, cnsJobConnectSidecars, jobID, token)
	require.Len(f.T(), allocations, 4)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(f.T(), tc.Nomad(), allocIDs)

	r := f.Assertions
	c := tc.Consul()

	// Verify services with cns set were registered into expected namespace
	e2eutil.RequireConsulRegistered(r, c, nsA, "count-api", 1)
	e2eutil.RequireConsulRegistered(r, c, nsA, "count-api-sidecar-proxy", 1)
	e2eutil.RequireConsulRegistered(r, c, nsA, "count-dashboard", 1)
	e2eutil.RequireConsulRegistered(r, c, nsA, "count-dashboard-sidecar-proxy", 1)

	// Verify services without cns set were registered into default
	e2eutil.RequireConsulRegistered(r, c, nsZ, "count-api-z", 1)
	e2eutil.RequireConsulRegistered(r, c, nsZ, "count-api-z-sidecar-proxy", 1)
	e2eutil.RequireConsulRegistered(r, c, nsZ, "count-dashboard-z", 1)
	e2eutil.RequireConsulRegistered(r, c, nsZ, "count-dashboard-z-sidecar-proxy", 1)

	// Stop the job
	e2eutil.WaitForJobStopped(f.T(), nomadClient, jobID)

	// Verify that services were de-registered from Consul
	e2eutil.RequireConsulDeregistered(r, c, nsA, "count-api")
	e2eutil.RequireConsulDeregistered(r, c, nsA, "count-api-sidecar-proxy")
	e2eutil.RequireConsulDeregistered(r, c, nsA, "count-dashboard")
	e2eutil.RequireConsulDeregistered(r, c, nsA, "count-dashboard-sidecar-proxy")
	e2eutil.RequireConsulDeregistered(r, c, nsZ, "count-api-z")
	e2eutil.RequireConsulDeregistered(r, c, nsZ, "count-api-z-sidecar-proxy")
	e2eutil.RequireConsulDeregistered(r, c, nsZ, "count-dashboard-z")
	e2eutil.RequireConsulDeregistered(r, c, nsZ, "count-dashboard-z-sidecar-proxy")
}

func (tc *ConsulNamespacesE2ETest) testConsulConnectIngressGateway(f *framework.F, token, nsA, nsZ string) {
	nomadClient := tc.Nomad()
	jobID := "cns-connect-ingress"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs
	allocations := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, cnsJobConnectIngress, jobID, token)
	require.Len(f.T(), allocations, 4) // 2 x (1 service + 1 gateway)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(f.T(), tc.Nomad(), allocIDs)

	r := f.Assertions
	c := tc.Consul()

	// Verify services with cns set were registered into expected namespace
	e2eutil.RequireConsulRegistered(r, c, nsA, "my-ingress-service", 1)
	e2eutil.RequireConsulRegistered(r, c, nsA, "uuid-api", 1)

	// Verify services without cns set were registered into default
	e2eutil.RequireConsulRegistered(r, c, nsZ, "my-ingress-service-z", 1)
	e2eutil.RequireConsulRegistered(r, c, nsZ, "uuid-api-z", 1)

	// Read the config entry of gateway with cns set, checking it exists in expected namespace
	ce := e2eutil.ReadConsulConfigEntry(f.T(), c, nsA, "ingress-gateway", "my-ingress-service")
	require.Equal(f.T(), nsA, ce.GetNamespace())

	// Read the config entry of gateway without cns set, checking it exists in default namespace
	ceZ := e2eutil.ReadConsulConfigEntry(f.T(), c, nsZ, "ingress-gateway", "my-ingress-service-z")
	require.Equal(f.T(), nsZ, ceZ.GetNamespace())

	// Stop the job
	e2eutil.WaitForJobStopped(f.T(), nomadClient, jobID)

	// Remove the config entries
	e2eutil.DeleteConsulConfigEntry(f.T(), c, nsA, "ingress-gateway", "my-ingress-service")
	e2eutil.DeleteConsulConfigEntry(f.T(), c, nsZ, "ingress-gateway", "my-ingress-service-z")
}

func (tc *ConsulNamespacesE2ETest) testConsulConnectTerminatingGateway(f *framework.F, token, nsA, nsZ string) {
	nomadClient := tc.Nomad()
	jobID := "cns-connect-terminating"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs
	allocations := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, cnsJobConnectTerminating, jobID, token)
	require.Len(f.T(), allocations, 6) // 2 x (2 services + 1 gateway)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(f.T(), tc.Nomad(), allocIDs)

	r := f.Assertions
	c := tc.Consul()

	// Verify services with cns set were registered into "default" Consul namespace
	e2eutil.RequireConsulRegistered(r, c, nsA, "api-gateway", 1)
	e2eutil.RequireConsulRegistered(r, c, nsA, "count-api", 1)
	e2eutil.RequireConsulRegistered(r, c, nsA, "count-dashboard", 1)

	// Verify services without cns set were registered into "default" Consul namespace
	e2eutil.RequireConsulRegistered(r, c, nsZ, "api-gateway-z", 1)
	e2eutil.RequireConsulRegistered(r, c, nsZ, "count-api-z", 1)
	e2eutil.RequireConsulRegistered(r, c, nsZ, "count-dashboard-z", 1)

	// Read the config entry of gateway with cns set, checking it exists in "default' namespace
	ce := e2eutil.ReadConsulConfigEntry(f.T(), c, nsA, "terminating-gateway", "api-gateway")
	require.Equal(f.T(), nsA, ce.GetNamespace())

	// Read the config entry of gateway without cns set, checking it exists in "default' namespace
	ceZ := e2eutil.ReadConsulConfigEntry(f.T(), c, nsZ, "terminating-gateway", "api-gateway-z")
	require.Equal(f.T(), nsZ, ceZ.GetNamespace())

	// Stop the job
	e2eutil.WaitForJobStopped(f.T(), nomadClient, jobID)

	// Remove the config entries
	e2eutil.DeleteConsulConfigEntry(f.T(), c, nsA, "terminating-gateway", "api-gateway")
	e2eutil.DeleteConsulConfigEntry(f.T(), c, nsZ, "terminating-gateway", "api-gateway-z")
}

func (tc *ConsulNamespacesE2ETest) testConsulScriptChecksTask(f *framework.F, token, nsA, nsZ string) {
	nomadClient := tc.Nomad()
	jobID := "cns-script-checks-task"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs
	allocations := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, cnsJobScriptChecksTask, jobID, token)
	require.Len(f.T(), allocations, 2)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(f.T(), tc.Nomad(), allocIDs)

	r := f.Assertions
	c := tc.Consul()

	sort.Sort(e2eutil.AllocsByName(allocations))
	allocsWithSetNamespace := allocations[0:1]
	allocsWithNoNamespace := allocations[1:2]

	// Verify checks with namespace set are set into expected namespace
	e2eutil.RequireConsulStatus(r, c, nsA, "service-1a", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, c, nsA, "service-2a", capi.HealthWarning)
	e2eutil.RequireConsulStatus(r, c, nsA, "service-3a", capi.HealthCritical)

	// Check in warning state becomes healthy after check passes for the service
	// with specified Consul namespace
	//
	// (ensures UpdateTTL is respecting namespace)
	_, _, err := exec(nomadClient, allocsWithSetNamespace,
		[]string{"/bin/sh", "-c", "touch ${NOMAD_TASK_DIR}/alive-2ab"})
	r.NoError(err)
	e2eutil.RequireConsulStatus(r, c, nsA, "service-2a", capi.HealthPassing)

	// Verify checks without namespace are set in default namespace
	e2eutil.RequireConsulStatus(r, c, nsZ, "service-1z", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, c, nsZ, "service-2z", capi.HealthWarning)
	e2eutil.RequireConsulStatus(r, c, nsZ, "service-3z", capi.HealthCritical)

	// Check in warning state becomes healthy after check passes for the service
	// with specified Consul namespace
	//
	// (ensures UpdateTTL is respecting namespace)
	_, _, errZ := exec(nomadClient, allocsWithNoNamespace,
		[]string{"/bin/sh", "-c", "touch ${NOMAD_TASK_DIR}/alive-2zb"})
	r.NoError(errZ)
	e2eutil.RequireConsulStatus(r, c, nsZ, "service-2z", capi.HealthPassing)

	// Stop the job
	e2eutil.WaitForJobStopped(f.T(), nomadClient, jobID)
}

func (tc *ConsulNamespacesE2ETest) testConsulScriptChecksGroup(f *framework.F, token, nsA, nsZ string) {
	nomadClient := tc.Nomad()
	jobID := "cns-script-checks-group"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs
	allocations := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, cnsJobScriptChecksGroup, jobID, token)
	require.Len(f.T(), allocations, 2)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(f.T(), tc.Nomad(), allocIDs)

	r := f.Assertions
	c := tc.Consul()

	sort.Sort(e2eutil.AllocsByName(allocations))
	allocsWithSetNamespace := allocations[0:1]
	allocsWithNoNamespace := allocations[1:2]

	// Verify checks were registered into "default" Consul namespace
	e2eutil.RequireConsulStatus(r, c, nsA, "service-1a", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, c, nsA, "service-2a", capi.HealthWarning)
	e2eutil.RequireConsulStatus(r, c, nsA, "service-3a", capi.HealthCritical)

	// Check in warning state becomes healthy after check passes for the service
	// with specified Consul namespace
	//
	// (ensures UpdateTTL is respecting namespace)
	_, _, err := exec(nomadClient, allocsWithSetNamespace,
		[]string{"/bin/sh", "-c", "touch /tmp/${NOMAD_ALLOC_ID}-alive-2ab"})
	r.NoError(err)
	e2eutil.RequireConsulStatus(r, c, nsA, "service-2a", capi.HealthPassing)

	// Verify checks were registered into "default" Consul namespace when no
	// namespace was specified.
	e2eutil.RequireConsulStatus(r, c, nsZ, "service-1z", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, c, nsZ, "service-2z", capi.HealthWarning)
	e2eutil.RequireConsulStatus(r, c, nsZ, "service-3z", capi.HealthCritical)

	// Check in warning state becomes healthy after check passes for the service
	// with specified Consul namespace
	//
	// (ensures UpdateTTL is respecting namespace)
	_, _, errZ := exec(nomadClient, allocsWithNoNamespace,
		[]string{"/bin/sh", "-c", "touch /tmp/${NOMAD_ALLOC_ID}-alive-2zb"})
	r.NoError(errZ)
	e2eutil.RequireConsulStatus(r, c, nsZ, "service-2z", capi.HealthPassing)

	// Stop the job
	e2eutil.WaitForJobStopped(f.T(), nomadClient, jobID)
}
