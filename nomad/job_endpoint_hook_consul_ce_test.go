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
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestJobEndpointHook_ConsulCE(t *testing.T) {
	ci.Parallel(t)

	srv, cleanup := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, srv.RPC)

	job := mock.Job()

	// create two group-level services and assign to clusters
	taskSvc := job.TaskGroups[0].Tasks[0].Services[0]
	taskSvc.Provider = structs.ServiceProviderConsul
	taskSvc.Cluster = "nondefault"
	job.TaskGroups[0].Tasks[0].Services = []*structs.Service{taskSvc}

	job.TaskGroups[0].Services = append(job.TaskGroups[0].Services, taskSvc.Copy())
	job.TaskGroups[0].Services = append(job.TaskGroups[0].Services, taskSvc.Copy())
	job.TaskGroups[0].Services[0].Cluster = ""
	job.TaskGroups[0].Services[1].Cluster = "infra"

	hook := jobConsulHook{srv}

	_, _, err := hook.Mutate(job)

	must.NoError(t, err)
	test.Eq(t, "default", job.TaskGroups[0].Services[0].Cluster)
	test.Eq(t, "infra", job.TaskGroups[0].Services[1].Cluster)
	test.Eq(t, "nondefault", job.TaskGroups[0].Tasks[0].Services[0].Cluster)

	_, err = hook.Validate(job)
	must.EqError(t, err, "non-default Consul cluster requires Nomad Enterprise")
}
