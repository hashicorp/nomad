// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	structsc "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/assert"
)

func testHarness(t *testing.T) *consulHook {
	alloc := mock.Alloc()
	task := alloc.LookupTask("web")

	widTask := &structs.WorkloadIdentity{
		Name:     task.Consul.IdentityName(),
		Audience: []string{"consul.io"},
	}

	widService := &structs.WorkloadIdentity{
		Name:     task.Services[0].Name,
		Audience: []string{"consul.io"},
	}

	return newConsulHook(
		testlog.HCLogger(t),
		alloc,
		&allocdir.AllocDir{Dir: "foo"},
		widmgr,
		consulConfigs,
		hookResources,
	)
}

func Test_consulHook_prepareConsulTokensForTask(t *testing.T) {
	type fields struct {
		alloc         *structs.Allocation
		allocdir      *allocdir.AllocDir
		widmgr        widmgr.IdentityManager
		consulConfigs map[string]*structsc.ConsulConfig
		hookResources *cstructs.AllocHookResources
		logger        hclog.Logger
	}
	type args struct {
		task   *structs.Task
		tokens map[string]map[string]string
	}
	hook := testHarness(t)
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &consulHook{
				alloc:         tt.fields.alloc,
				allocdir:      tt.fields.allocdir,
				widmgr:        tt.fields.widmgr,
				consulConfigs: tt.fields.consulConfigs,
				hookResources: tt.fields.hookResources,
				logger:        tt.fields.logger,
			}
			tt.wantErr(t, h.prepareConsulTokensForTask(tt.args.task, tt.args.tokens), fmt.Sprintf("prepareConsulTokensForTask(%v, %v)", tt.args.task, tt.args.tokens))
		})
	}
}
