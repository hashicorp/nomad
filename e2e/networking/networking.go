// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package networking

import (
	"os"
	"strings"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
)

type NetworkingE2ETest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Networking",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			e2eutil.NewE2EJob("networking/inputs/basic.nomad"),
			new(NetworkingE2ETest),
		},
	})
}

func (tc *NetworkingE2ETest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *NetworkingE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, jobID := range tc.jobIDs {
		err := e2eutil.StopJob(jobID, "-purge")
		f.NoError(err)
	}
	tc.jobIDs = []string{}

	_, err := e2eutil.Command("nomad", "system", "gc")
	f.NoError(err)
}

func (tc *NetworkingE2ETest) TestNetworking_DockerBridgedHostname(f *framework.F) {

	jobID := "test-networking-" + uuid.Generate()[0:8]
	f.NoError(e2eutil.Register(jobID, "networking/inputs/docker_bridged_hostname.nomad"))
	tc.jobIDs = append(tc.jobIDs, jobID)
	f.NoError(e2eutil.WaitForAllocStatusExpected(jobID, "default", []string{"running"}),
		"job should be running with 1 alloc")

	// Grab the allocations for the job.
	allocs, _, err := tc.Nomad().Jobs().Allocations(jobID, false, nil)
	f.NoError(err, "failed to get allocs for job")
	f.Len(allocs, 1, "job should have one alloc")

	// Run the hostname command within the allocation.
	hostnameOutput, err := e2eutil.AllocExec(allocs[0].ID, "sleep", "hostname", "default", nil)
	f.NoError(err, "failed to run hostname exec command")
	f.Equal("mylittlepony", strings.TrimSpace(hostnameOutput), "incorrect hostname set within container")

	// Check the /etc/hosts file for the correct IP address and hostname entry.
	hostsOutput, err := e2eutil.AllocExec(allocs[0].ID, "sleep", "cat /etc/hosts", "default", nil)
	f.NoError(err, "failed to run hostname exec command")
	f.Contains(hostsOutput, "mylittlepony", "/etc/hosts doesn't contain hostname entry")
}

func (tc *NetworkingE2ETest) TestNetworking_DockerBridgedHostnameInterpolation(f *framework.F) {

	jobID := "test-networking-" + uuid.Generate()[0:8]
	f.NoError(e2eutil.Register(jobID, "networking/inputs/docker_bridged_hostname_interpolation.nomad"))
	tc.jobIDs = append(tc.jobIDs, jobID)
	f.NoError(e2eutil.WaitForAllocStatusExpected(jobID, "default", []string{"running"}),
		"job should be running with 1 alloc")

	// Grab the allocations for the job.
	allocs, _, err := tc.Nomad().Jobs().Allocations(jobID, false, nil)
	f.NoError(err, "failed to get allocs for job")
	f.Len(allocs, 1, "job should have one alloc")

	// Run the hostname command within the allocation.
	hostnameOutput, err := e2eutil.AllocExec(allocs[0].ID, "sleep", "hostname", "default", nil)
	f.NoError(err, "failed to run hostname exec command")
	f.Equal("mylittlepony-0", strings.TrimSpace(hostnameOutput), "incorrect hostname set within container")

	// Check the /etc/hosts file for the correct IP address and hostname entry.
	hostsOutput, err := e2eutil.AllocExec(allocs[0].ID, "sleep", "cat /etc/hosts", "default", nil)
	f.NoError(err, "failed to run hostname exec command")
	f.Contains(hostsOutput, "mylittlepony-0", "/etc/hosts doesn't contain hostname entry")
}

func (tc *NetworkingE2ETest) TestNetworking_DockerBridgedCNIEnvVars(f *framework.F) {

	jobID := "test-networking-" + uuid.Generate()[0:8]
	f.NoError(e2eutil.Register(jobID, "networking/inputs/docker_bridged_basic.nomad"))
	tc.jobIDs = append(tc.jobIDs, jobID)
	f.NoError(e2eutil.WaitForAllocStatusExpected(jobID, "default", []string{"running"}),
		"job should be running with 1 alloc")

	// Grab the allocations for the job.
	allocs, _, err := tc.Nomad().Jobs().Allocations(jobID, false, nil)
	f.NoError(err, "failed to get allocs for job")
	f.Len(allocs, 1, "job should have one alloc")

	// Run the env command within the allocation.
	envOutput, err := e2eutil.AllocExec(allocs[0].ID, "sleep", "env", "default", nil)
	f.NoError(err, "failed to run env exec command")

	// Check all the network namespace env vars are present.
	f.Contains(envOutput, "NOMAD_ALLOC_INTERFACE_dummy", "namespace interface env var not found")
	f.Contains(envOutput, "NOMAD_ALLOC_IP_dummy", "namespace ip env var not found")
	f.Contains(envOutput, "NOMAD_ALLOC_PORT_dummy", "namespace port env var not found")
	f.Contains(envOutput, "NOMAD_ALLOC_ADDR_dummy", "namespace addr env var not found")
}

func (tc *NetworkingE2ETest) TestNetworking_DockerBridgedDefaultIPv6(f *framework.F) {

	jobID := "test-networking-" + uuid.Generate()[0:8]
	f.NoError(e2eutil.Register(jobID, "networking/inputs/docker_bridged_ipv6.nomad.hcl"))
	tc.jobIDs = append(tc.jobIDs, jobID)
	f.NoError(e2eutil.WaitForAllocStatusExpected(jobID, "default", []string{"running"}),
		"job should be running with 1 alloc")

	// Grab the allocations for the job.
	allocs, _, err := tc.Nomad().Jobs().Allocations(jobID, false, nil)
	f.NoError(err, "failed to get allocs for job")
	f.Len(allocs, 1, "job should have one alloc")

	// check that within bridge network we can curl IPv6 address
	_, err = e2eutil.AllocExec(allocs[0].ID, "t", "chmod +x /local/bridge_ipv6.sh", "default", nil)
	f.NoError(err, "failed to run chmod exec command")
	_, err = e2eutil.AllocExec(allocs[0].ID, "t", "./local/bridge_ipv6.sh", "default", nil)
	f.NoError(err, "failed to run script exec command")
	curlOutput, err := e2eutil.AllocExec(allocs[0].ID, "t", "curl -s [::]:8000", "default", nil)
	f.NoError(err, "failed to run curl IPv6 exec command")
	f.Equal("hellooo-thereee:P", strings.TrimSpace(curlOutput), "incorrect IPv6 response")

}
