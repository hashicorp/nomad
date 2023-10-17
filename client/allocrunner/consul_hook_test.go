// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	structsc "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/shoenig/test/must"
)

// statically assert network hook implements the expected interfaces
var _ interfaces.RunnerPrerunHook = (*consulHook)(nil)

func testHarness(t *testing.T) (*consulHook, *structs.Task) {
	logger := testlog.HCLogger(t)

	alloc := mock.Alloc()
	task := alloc.LookupTask("web")

	task.Identities = []*structs.WorkloadIdentity{
		{Name: fmt.Sprintf("%s_default", structs.ConsulTaskIdentityNamePrefix)},
	}

	// setup mock signer and sign the identities
	mockSigner := widmgr.NewMockWIDSigner(task.Identities)
	signedIDs, err := mockSigner.SignIdentities(1, []*structs.WorkloadIdentityRequest{
		{
			AllocID: alloc.ID,
			WIHandle: structs.WIHandle{
				WorkloadIdentifier: task.Name,
				IdentityName:       task.Identities[0].Name,
			},
		},
	})
	must.NoError(t, err)

	mockWIDMgr := widmgr.NewMockWIDMgr(signedIDs)

	consulConfigs := map[string]*structsc.ConsulConfig{
		"default": structsc.DefaultConsulConfig(),
		"foo":     {Name: "foo"},
	}

	hookResources := cstructs.NewAllocHookResources()

	consulHookCfg := consulHookConfig{
		alloc:                   alloc,
		allocdir:                nil,
		widmgr:                  mockWIDMgr,
		consulConfigs:           consulConfigs,
		consulClientConstructor: consul.NewMockConsulClient,
		hookResources:           hookResources,
		logger:                  logger,
	}
	return newConsulHook(consulHookCfg), task
}

func Test_consulHook_prepareConsulTokensForTask(t *testing.T) {
	ci.Parallel(t)

	hook, task := testHarness(t)
	must.Eq(t, hook.Name(), "consul")
	must.NoError(t, hook.Prerun())

	tests := []struct {
		name        string
		task        *structs.Task
		tg          *structs.TaskGroup
		tokens      map[string]map[string]string
		wantErr     bool
		errMsg      string
		emptyTokens bool
	}{
		{
			name:        "empty task and tg",
			task:        nil,
			tg:          nil,
			tokens:      map[string]map[string]string{},
			wantErr:     false,
			errMsg:      "",
			emptyTokens: true,
		},
		{
			name: "task with no corresponding cluster",
			task: &structs.Task{
				Consul: &structs.Consul{Cluster: "bar"},
			},
			tg:          nil,
			tokens:      map[string]map[string]string{},
			wantErr:     true,
			errMsg:      "no such consul cluster: bar",
			emptyTokens: true,
		},
		{
			name:        "task with tg and corresponding cluster",
			task:        task,
			tg:          &structs.TaskGroup{Consul: &structs.Consul{Cluster: "default"}},
			tokens:      map[string]map[string]string{},
			wantErr:     false,
			errMsg:      "",
			emptyTokens: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hook.prepareConsulTokensForTask(tt.task, tt.tg, tt.tokens)
			if tt.wantErr {
				must.Error(t, err)
				must.Eq(t, tt.errMsg, err.Error())
			} else {
				must.NoError(t, err)
			}
			if !tt.emptyTokens {
				must.MapNotEmpty(t, tt.tokens)
			}
		})
	}
}
