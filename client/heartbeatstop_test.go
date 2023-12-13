// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestHeartbeatStop_allocHook(t *testing.T) {
	ci.Parallel(t)

	server, _, cleanupS1 := testServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, server.RPC)

	client, cleanupC1 := TestClient(t, func(c *config.Config) {
		c.RPCHandler = server
	})
	defer cleanupC1()

	// an allocation, with a tiny lease
	d := 1 * time.Microsecond
	alloc := &structs.Allocation{
		ID:        uuid.Generate(),
		TaskGroup: "foo",
		Job: &structs.Job{
			TaskGroups: []*structs.TaskGroup{
				{
					Name:                      "foo",
					StopAfterClientDisconnect: &d,
				},
			},
		},
		Resources: &structs.Resources{
			CPU:      100,
			MemoryMB: 100,
			DiskMB:   0,
		},
	}

	// alloc added to heartbeatStop.allocs
	err := client.addAlloc(alloc, "")
	require.NoError(t, err)
	testutil.WaitForResult(func() (bool, error) {
		_, ok := client.heartbeatStop.allocInterval[alloc.ID]
		return ok, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// the tiny lease causes the watch loop to destroy it
	testutil.WaitForResult(func() (bool, error) {
		_, ok := client.heartbeatStop.allocInterval[alloc.ID]
		return !ok, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	require.Empty(t, client.allocs[alloc.ID])
}
