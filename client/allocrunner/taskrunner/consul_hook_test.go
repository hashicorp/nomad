// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

// TestConsulHook ensures we're only writing Consul tokens for the appropriate
// task's identities
func TestConsulHook(t *testing.T) {

	alloc := mock.Alloc()
	task := alloc.LookupTask("web")
	task.Consul = &structs.Consul{
		Cluster: "default",
	}
	task.Identities = []*structs.WorkloadIdentity{{Name: "consul_default"}}

	resources := cstructs.NewAllocHookResources()
	resources.SetConsulTokens(map[string]map[string]*api.ACLToken{
		"default": map[string]*api.ACLToken{
			"consul_default/web":   &api.ACLToken{SecretID: "foo"},
			"consul_default/extra": &api.ACLToken{SecretID: "bar"}, // for different task
			"consul_infra/web":     &api.ACLToken{SecretID: "baz"}, // for different cluster
			"service_foo":          &api.ACLToken{SecretID: "qux"}, // for service
		},
	})
	taskDir := t.TempDir()

	hook := &consulHook{
		task:          task,
		tokenDir:      taskDir,
		hookResources: resources,
		logger:        testlog.HCLogger(t),
	}

	resp := &interfaces.TaskPrestartResponse{}
	hook.Prestart(context.TODO(), &interfaces.TaskPrestartRequest{}, resp)

	must.FileContains(t, filepath.Join(taskDir, "consul_token"), "foo")
	must.Eq(t, "foo", resp.Env["CONSUL_TOKEN"])
}
