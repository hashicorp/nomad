// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestJobEndpointHook_VaultCE(t *testing.T) {
	ci.Parallel(t)

	srv, cleanup := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, srv.RPC)

	job := mock.Job()

	// create two different Vault blocks and assign to clusters
	job.TaskGroups[0].Tasks = append(job.TaskGroups[0].Tasks, job.TaskGroups[0].Tasks[0].Copy())
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{Cluster: structs.VaultDefaultCluster}
	job.TaskGroups[0].Tasks[1].Name = "web2"
	job.TaskGroups[0].Tasks[1].Vault = &structs.Vault{Cluster: "infra"}

	hook := jobVaultHook{srv}
	_, _, err := hook.Mutate(job)
	must.NoError(t, err)
	must.Eq(t, structs.VaultDefaultCluster, job.TaskGroups[0].Tasks[0].Vault.Cluster)
	must.Eq(t, "infra", job.TaskGroups[0].Tasks[1].Vault.Cluster)

	// skipping over the rest of Validate b/c it requires an actual
	// Vault cluster
	err = hook.validateClustersForNamespace(job, job.Vault())
	must.EqError(t, err, "non-default Vault cluster requires Nomad Enterprise")

}
