// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestJobEndpointHook_VaultCE(t *testing.T) {
	ci.Parallel(t)

	srv, cleanup := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.VaultConfigs[structs.VaultDefaultCluster].Enabled = pointer.Of(true)
		c.VaultConfigs[structs.VaultDefaultCluster].DefaultIdentity = &config.WorkloadIdentityConfig{
			Name:     "vault_default",
			Audience: []string{"vault.io"},
		}
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

	job = mock.Job()
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{Cluster: structs.VaultDefaultCluster}
	warnings, err := hook.Validate(job)
	must.Len(t, 0, warnings)
	must.NoError(t, err)

	// Attempt to validate a job which details a Vault cluster name which has
	// no configuration mapping within the server config.
	mockJob2 := mock.Job()
	mockJob2.TaskGroups[0].Tasks[0].Vault = &structs.Vault{Cluster: "does-not-exist"}

	warnings, err = hook.Validate(mockJob2)
	must.Nil(t, warnings)
	must.EqError(t, err, `Vault "does-not-exist" not enabled but used in the job`)
}
