// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

// Nomad OSS ignores Consul Namespace configuration in jobs, these e2e tests
// verify everything still works and is registered into the "default" namespace,
// since e2e always uses Consul Enterprise. With Consul OSS, there  are no namespaces.
// and these tests will not work.

package consul

import (
	"os"
	"sort"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/stretchr/testify/require"
)

func (tc *ConsulNamespacesE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	// cleanup jobs
	for _, id := range tc.jobIDs {
		_, _, err := tc.Nomad().Jobs().Deregister(id, true, nil)
		f.NoError(err)
	}

	// do garbage collection
	err := tc.Nomad().System().GarbageCollect()
	f.NoError(err)

	// reset accumulators
	tc.tokenIDs = make(map[string][]string)
	tc.policyIDs = make(map[string][]string)
}

func (tc *ConsulNamespacesE2ETest) TestConsulRegisterGroupServices(f *framework.F) {
	nomadClient := tc.Nomad()
	jobID := "cns-group-services"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs
	allocations := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, cnsJobGroupServices, jobID, "")
	require.Len(f.T(), allocations, 3)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(f.T(), tc.Nomad(), allocIDs)

	r := f.Assertions
	c := tc.Consul()
	namespace := consulNamespace

	// Verify our services were registered into "default"
	e2eutil.RequireConsulRegistered(r, c, namespace, "b1", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "b2", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "c1", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "c2", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "z1", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "z2", 1)

	// Verify our services are all healthy
	e2eutil.RequireConsulStatus(r, c, namespace, "b1", "passing")
	e2eutil.RequireConsulStatus(r, c, namespace, "b2", "passing")
	e2eutil.RequireConsulStatus(r, c, namespace, "c1", "passing")
	e2eutil.RequireConsulStatus(r, c, namespace, "c2", "passing")
	e2eutil.RequireConsulStatus(r, c, namespace, "z1", "passing")
	e2eutil.RequireConsulStatus(r, c, namespace, "z2", "passing")

	// Verify our services were NOT registered into specified consul namespaces
	e2eutil.RequireConsulRegistered(r, c, "banana", "b1", 0)
	e2eutil.RequireConsulRegistered(r, c, "banana", "b2", 0)
	e2eutil.RequireConsulRegistered(r, c, "cherry", "c1", 0)
	e2eutil.RequireConsulRegistered(r, c, "cherry", "c2", 0)

	// Stop the job
	e2eutil.WaitForJobStopped(f.T(), nomadClient, jobID)

	// Verify that services were de-registered in Consul
	e2eutil.RequireConsulDeregistered(r, c, namespace, "b1")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "b2")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "c1")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "c2")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "z1")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "z2")
}

func (tc *ConsulNamespacesE2ETest) TestConsulRegisterTaskServices(f *framework.F) {
	nomadClient := tc.Nomad()
	jobID := "cns-task-services"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs
	allocations := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, cnsJobTaskServices, jobID, "")
	require.Len(f.T(), allocations, 3)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(f.T(), tc.Nomad(), allocIDs)

	r := f.Assertions
	c := tc.Consul()
	namespace := consulNamespace

	// Verify our services were registered into "default"
	e2eutil.RequireConsulRegistered(r, c, namespace, "b1", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "b2", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "c1", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "c2", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "z1", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "z2", 1)

	// Verify our services are all healthy
	e2eutil.RequireConsulStatus(r, c, namespace, "b1", "passing")
	e2eutil.RequireConsulStatus(r, c, namespace, "b2", "passing")
	e2eutil.RequireConsulStatus(r, c, namespace, "c1", "passing")
	e2eutil.RequireConsulStatus(r, c, namespace, "c2", "passing")
	e2eutil.RequireConsulStatus(r, c, namespace, "z1", "passing")
	e2eutil.RequireConsulStatus(r, c, namespace, "z2", "passing")

	// Verify our services were NOT registered into specified consul namespaces
	e2eutil.RequireConsulRegistered(r, c, "banana", "b1", 0)
	e2eutil.RequireConsulRegistered(r, c, "banana", "b2", 0)
	e2eutil.RequireConsulRegistered(r, c, "cherry", "c1", 0)
	e2eutil.RequireConsulRegistered(r, c, "cherry", "c2", 0)
	e2eutil.RequireConsulRegistered(r, c, "cherry", "z1", 0)
	e2eutil.RequireConsulRegistered(r, c, "cherry", "z2", 0)

	// Stop the job
	e2eutil.WaitForJobStopped(f.T(), nomadClient, jobID)

	// Verify that services were de-registered from Consul
	e2eutil.RequireConsulDeregistered(r, c, namespace, "b1")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "b2")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "c1")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "b2")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "z1")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "z2")
}

func (tc *ConsulNamespacesE2ETest) TestConsulTemplateKV(f *framework.F) {
	t := f.T()
	nomadClient := tc.Nomad()
	jobID := "cns-template-kv"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs to complete
	allocations := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, cnsJobTemplateKV, jobID, "")
	require.Len(t, allocations, 2)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsStopped(f.T(), tc.Nomad(), allocIDs)

	// Sort allocs by name
	sort.Sort(e2eutil.AllocsByName(allocations))

	// Check template read from default namespace even if namespace set
	textB, err := e2eutil.AllocTaskLogs(allocations[0].ID, "task-b", e2eutil.LogsStdOut)
	require.NoError(t, err)
	require.Equal(t, "value: ns_default", textB)

	// Check template read from default namespace if no namespace set
	textZ, err := e2eutil.AllocTaskLogs(allocations[1].ID, "task-z", e2eutil.LogsStdOut)
	require.NoError(t, err)
	require.Equal(t, "value: ns_default", textZ)

	//  Stop the job
	e2eutil.WaitForJobStopped(t, nomadClient, jobID)
}

func (tc *ConsulNamespacesE2ETest) TestConsulConnectSidecars(f *framework.F) {
	nomadClient := tc.Nomad()
	jobID := "cns-connect-sidecars"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs
	allocations := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, cnsJobConnectSidecars, jobID, "")
	require.Len(f.T(), allocations, 4)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(f.T(), tc.Nomad(), allocIDs)

	r := f.Assertions
	c := tc.Consul()
	namespace := consulNamespace

	// Verify services with cns set were registered into "default"
	e2eutil.RequireConsulRegistered(r, c, namespace, "count-api", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "count-api-sidecar-proxy", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "count-dashboard", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "count-dashboard-sidecar-proxy", 1)

	// Verify services without cns set were registered into "default"
	e2eutil.RequireConsulRegistered(r, c, namespace, "count-api-z", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "count-api-z-sidecar-proxy", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "count-dashboard-z", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "count-dashboard-z-sidecar-proxy", 1)

	// Verify our services were NOT registered into specified consul namespaces
	e2eutil.RequireConsulRegistered(r, c, "apple", "count-api", 0)
	e2eutil.RequireConsulRegistered(r, c, "apple", "count-api-sidecar-proxy", 0)
	e2eutil.RequireConsulRegistered(r, c, "apple", "count-dashboard", 0)
	e2eutil.RequireConsulRegistered(r, c, "apple", "count-dashb0ard-sidecar-proxy", 0)

	// Stop the job
	e2eutil.WaitForJobStopped(f.T(), nomadClient, jobID)

	// Verify that services were de-registered from Consul
	e2eutil.RequireConsulDeregistered(r, c, namespace, "count-api")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "count-api-sidecar-proxy")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "count-dashboard")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "count-dashboard-sidecar-proxy")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "count-api-z")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "count-api-z-sidecar-proxy")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "count-dashboard-z")
	e2eutil.RequireConsulDeregistered(r, c, namespace, "count-dashboard-z-sidecar-proxy")
}

func (tc *ConsulNamespacesE2ETest) TestConsulConnectIngressGateway(f *framework.F) {
	nomadClient := tc.Nomad()
	jobID := "cns-connect-ingress"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs
	allocations := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, cnsJobConnectIngress, jobID, "")
	require.Len(f.T(), allocations, 4) // 2 x (1 service + 1 gateway)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(f.T(), tc.Nomad(), allocIDs)

	r := f.Assertions
	c := tc.Consul()
	namespace := consulNamespace

	// Verify services with cns set were registered into "default"
	e2eutil.RequireConsulRegistered(r, c, namespace, "my-ingress-service", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "uuid-api", 1)

	// Verify services without cns set were registered into "default"
	e2eutil.RequireConsulRegistered(r, c, namespace, "my-ingress-service-z", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "uuid-api-z", 1)

	// Verify services with cns set were NOT registered into specified consul namespaces
	e2eutil.RequireConsulRegistered(r, c, "apple", "my-ingress-service", 0)
	e2eutil.RequireConsulRegistered(r, c, "apple", "uuid-api", 0)

	// Read the config entry of gateway with cns set, checking it exists in "default' namespace
	ce := e2eutil.ReadConsulConfigEntry(f.T(), c, namespace, "ingress-gateway", "my-ingress-service")
	require.Equal(f.T(), namespace, ce.GetNamespace())

	// Read the config entry of gateway without cns set, checking it exists in "default' namespace
	ceZ := e2eutil.ReadConsulConfigEntry(f.T(), c, namespace, "ingress-gateway", "my-ingress-service-z")
	require.Equal(f.T(), namespace, ceZ.GetNamespace())

	// Stop the job
	e2eutil.WaitForJobStopped(f.T(), nomadClient, jobID)

	// Remove the config entries
	e2eutil.DeleteConsulConfigEntry(f.T(), c, namespace, "ingress-gateway", "my-ingress-service")
	e2eutil.DeleteConsulConfigEntry(f.T(), c, namespace, "ingress-gateway", "my-ingress-service-z")
}

func (tc *ConsulNamespacesE2ETest) TestConsulConnectTerminatingGateway(f *framework.F) {
	nomadClient := tc.Nomad()
	jobID := "cns-connect-terminating"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs
	allocations := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, cnsJobConnectTerminating, jobID, "")
	require.Len(f.T(), allocations, 6) // 2 x (2 services + 1 gateway)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(f.T(), tc.Nomad(), allocIDs)

	r := f.Assertions
	c := tc.Consul()
	namespace := consulNamespace

	// Verify services with cns set were registered into "default" Consul namespace
	e2eutil.RequireConsulRegistered(r, c, namespace, "api-gateway", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "count-api", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "count-dashboard", 1)

	// Verify services without cns set were registered into "default" Consul namespace
	e2eutil.RequireConsulRegistered(r, c, namespace, "api-gateway-z", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "count-api-z", 1)
	e2eutil.RequireConsulRegistered(r, c, namespace, "count-dashboard-z", 1)

	// Verify services with cns set were NOT registered into specified consul namespaces
	e2eutil.RequireConsulRegistered(r, c, "apple", "api-gateway", 0)
	e2eutil.RequireConsulRegistered(r, c, "apple", "count-api", 0)
	e2eutil.RequireConsulRegistered(r, c, "apple", "count-dashboard", 0)

	// Read the config entry of gateway with cns set, checking it exists in "default' namespace
	ce := e2eutil.ReadConsulConfigEntry(f.T(), c, namespace, "terminating-gateway", "api-gateway")
	require.Equal(f.T(), namespace, ce.GetNamespace())

	// Read the config entry of gateway without cns set, checking it exists in "default' namespace
	ceZ := e2eutil.ReadConsulConfigEntry(f.T(), c, namespace, "terminating-gateway", "api-gateway-z")
	require.Equal(f.T(), namespace, ceZ.GetNamespace())

	// Stop the job
	e2eutil.WaitForJobStopped(f.T(), nomadClient, jobID)

	// Remove the config entries
	e2eutil.DeleteConsulConfigEntry(f.T(), c, namespace, "terminating-gateway", "api-gateway")
	e2eutil.DeleteConsulConfigEntry(f.T(), c, namespace, "terminating-gateway", "api-gateway-z")
}

func (tc *ConsulNamespacesE2ETest) TestConsulScriptChecksTask(f *framework.F) {
	nomadClient := tc.Nomad()
	jobID := "cns-script-checks-task"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs
	allocations := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, cnsJobScriptChecksTask, jobID, "")
	require.Len(f.T(), allocations, 2)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(f.T(), tc.Nomad(), allocIDs)

	r := f.Assertions
	c := tc.Consul()
	namespace := consulNamespace

	sort.Sort(e2eutil.AllocsByName(allocations))
	allocsWithSetNamespace := allocations[0:1]
	allocsWithNoNamespace := allocations[1:2]

	// Verify checks were registered into "default" Consul namespace
	e2eutil.RequireConsulStatus(r, c, namespace, "service-1a", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, c, namespace, "service-2a", capi.HealthWarning)
	e2eutil.RequireConsulStatus(r, c, namespace, "service-3a", capi.HealthCritical)

	// Check in warning state becomes healthy after check passes for the service
	// with specified Consul namespace
	//
	// (ensures UpdateTTL is respecting namespace)
	_, _, err := exec(nomadClient, allocsWithSetNamespace,
		[]string{"/bin/sh", "-c", "touch ${NOMAD_TASK_DIR}/alive-2ab"})
	r.NoError(err)
	e2eutil.RequireConsulStatus(r, c, namespace, "service-2a", capi.HealthPassing)

	// Verify checks were registered into "default" Consul namespace when no
	// namespace was specified.
	e2eutil.RequireConsulStatus(r, c, namespace, "service-1z", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, c, namespace, "service-2z", capi.HealthWarning)
	e2eutil.RequireConsulStatus(r, c, namespace, "service-3z", capi.HealthCritical)

	// Check in warning state becomes healthy after check passes for the service
	// with specified Consul namespace
	//
	// (ensures UpdateTTL is respecting namespace)
	_, _, errZ := exec(nomadClient, allocsWithNoNamespace,
		[]string{"/bin/sh", "-c", "touch ${NOMAD_TASK_DIR}/alive-2zb"})
	r.NoError(errZ)
	e2eutil.RequireConsulStatus(r, c, namespace, "service-2z", capi.HealthPassing)

	// Stop the job
	e2eutil.WaitForJobStopped(f.T(), nomadClient, jobID)
}

func (tc *ConsulNamespacesE2ETest) TestConsulScriptChecksGroup(f *framework.F) {
	nomadClient := tc.Nomad()
	jobID := "cns-script-checks-group"
	tc.jobIDs = append(tc.jobIDs, jobID)

	// Run job and wait for allocs
	allocations := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, cnsJobScriptChecksGroup, jobID, "")
	require.Len(f.T(), allocations, 2)
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocations)
	e2eutil.WaitForAllocsRunning(f.T(), tc.Nomad(), allocIDs)

	r := f.Assertions
	c := tc.Consul()
	namespace := consulNamespace

	sort.Sort(e2eutil.AllocsByName(allocations))
	allocsWithSetNamespace := allocations[0:1]
	allocsWithNoNamespace := allocations[1:2]

	// Verify checks were registered into "default" Consul namespace
	e2eutil.RequireConsulStatus(r, c, namespace, "service-1a", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, c, namespace, "service-2a", capi.HealthWarning)
	e2eutil.RequireConsulStatus(r, c, namespace, "service-3a", capi.HealthCritical)

	// Check in warning state becomes healthy after check passes for the service
	// with specified Consul namespace
	//
	// (ensures UpdateTTL is respecting namespace)
	_, _, err := exec(nomadClient, allocsWithSetNamespace,
		[]string{"/bin/sh", "-c", "touch /tmp/${NOMAD_ALLOC_ID}-alive-2ab"})
	r.NoError(err)
	e2eutil.RequireConsulStatus(r, c, namespace, "service-2a", capi.HealthPassing)

	// Verify checks were registered into "default" Consul namespace when no
	// namespace was specified.
	e2eutil.RequireConsulStatus(r, c, namespace, "service-1z", capi.HealthPassing)
	e2eutil.RequireConsulStatus(r, c, namespace, "service-2z", capi.HealthWarning)
	e2eutil.RequireConsulStatus(r, c, namespace, "service-3z", capi.HealthCritical)

	// Check in warning state becomes healthy after check passes for the service
	// with specified Consul namespace
	//
	// (ensures UpdateTTL is respecting namespace)
	_, _, errZ := exec(nomadClient, allocsWithNoNamespace,
		[]string{"/bin/sh", "-c", "touch /tmp/${NOMAD_ALLOC_ID}-alive-2zb"})
	r.NoError(errZ)
	e2eutil.RequireConsulStatus(r, c, namespace, "service-2z", capi.HealthPassing)

	// Stop the job
	e2eutil.WaitForJobStopped(f.T(), nomadClient, jobID)
}
