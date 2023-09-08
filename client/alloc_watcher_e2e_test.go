// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// TestPrevAlloc_StreamAllocDir_TLS asserts ephemeral disk migrations still
// work when TLS is enabled.
func TestPrevAlloc_StreamAllocDir_TLS(t *testing.T) {
	const (
		caFn         = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		serverCertFn = "../helper/tlsutil/testdata/global-server-nomad.pem"
		serverKeyFn  = "../helper/tlsutil/testdata/global-server-nomad-key.pem"
		clientCertFn = "../helper/tlsutil/testdata/global-client-nomad.pem"
		clientKeyFn  = "../helper/tlsutil/testdata/global-client-nomad-key.pem"
	)
	ci.Parallel(t)
	require := require.New(t)

	server, cleanupS := nomad.TestServer(t, func(c *nomad.Config) {
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               caFn,
			CertFile:             serverCertFn,
			KeyFile:              serverKeyFn,
		}
	})
	defer cleanupS()
	testutil.WaitForLeader(t, server.RPC)

	t.Logf("[TEST] Leader started: %s", server.GetConfig().RPCAddr.String())

	agentConfFunc := func(c *agent.Config) {
		c.Region = "global"
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:           true,
			EnableRPC:            true,
			VerifyServerHostname: true,
			CAFile:               caFn,
			CertFile:             clientCertFn,
			KeyFile:              clientKeyFn,
		}
		c.Client.Enabled = true
		c.Client.Servers = []string{server.GetConfig().RPCAddr.String()}
	}
	client1 := agent.NewTestAgent(t, "client1", agentConfFunc)
	defer client1.Shutdown()

	client2 := agent.NewTestAgent(t, "client2", agentConfFunc)
	defer client2.Shutdown()

	job := mock.Job()
	job.Constraints = []*structs.Constraint{
		{
			LTarget: "${node.unique.name}",
			RTarget: "client1",
			Operand: "=",
		},
	}
	job.TaskGroups[0].Constraints = nil
	job.TaskGroups[0].Tasks[0].Services = nil
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].EphemeralDisk.Sticky = true
	job.TaskGroups[0].EphemeralDisk.Migrate = true
	job.TaskGroups[0].Tasks[0] = &structs.Task{
		Name:   "migrate_tls",
		Driver: "mock_driver",
		Config: map[string]interface{}{
			"run_for": "1m",
		},
		LogConfig: structs.DefaultLogConfig(),
		Resources: &structs.Resources{
			CPU:      50,
			MemoryMB: 25,
		},
	}
	testutil.WaitForRunning(t, server.RPC, job.Copy())

	allocArgs := &structs.JobSpecificRequest{}
	allocArgs.JobID = job.ID
	allocArgs.QueryOptions.Region = "global"
	var allocReply structs.JobAllocationsResponse
	require.NoError(server.RPC("Job.Allocations", allocArgs, &allocReply))
	require.Len(allocReply.Allocations, 1)
	origAlloc := allocReply.Allocations[0].ID

	// Save a file into alloc dir
	contents := []byte("123\n456")
	allocFn := filepath.Join(client1.DataDir, "alloc", origAlloc, "alloc", "data", "bar")
	require.NoError(os.WriteFile(allocFn, contents, 0666))
	t.Logf("[TEST] Wrote initial file: %s", allocFn)

	// Migrate alloc to other node
	job.Constraints[0].RTarget = "client2"

	// Only register job - don't wait for running - since previous completed allocs
	// will interfere
	testutil.RegisterJob(t, server.RPC, job.Copy())

	// Wait for new alloc to be running
	var newAlloc *structs.AllocListStub
	testutil.WaitForResult(func() (bool, error) {
		allocArgs := &structs.JobSpecificRequest{}
		allocArgs.JobID = job.ID
		allocArgs.QueryOptions.Region = "global"
		var allocReply structs.JobAllocationsResponse
		require.NoError(server.RPC("Job.Allocations", allocArgs, &allocReply))
		if n := len(allocReply.Allocations); n != 2 {
			return false, fmt.Errorf("expected 2 allocs found %d", n)
		}

		// Pick the one that didn't exist before
		if allocReply.Allocations[0].ID == origAlloc {
			newAlloc = allocReply.Allocations[1]
		} else {
			newAlloc = allocReply.Allocations[0]
		}

		return newAlloc.ClientStatus != structs.AllocClientStatusRunning,
			fmt.Errorf("client status: %v", newAlloc.ClientStatus)
	}, func(err error) {
		t.Fatalf("new alloc not running: %v", err)
	})

	// Wait for file to appear on other client
	allocFn2 := filepath.Join(client2.DataDir, "alloc", newAlloc.ID, "alloc", "data", "bar")
	t.Logf("[TEST] Comparing against file: %s", allocFn2)
	testutil.WaitForResult(func() (bool, error) {
		found, err := os.ReadFile(allocFn2)
		if err != nil {
			return false, err
		}
		return bytes.Equal(contents, found), fmt.Errorf("contents misatch. expected:\n%s\n\nfound:\n%s\n",
			contents, found)
	}, func(err error) {
		t.Fatalf("file didn't migrate: %v", err)
	})
}
