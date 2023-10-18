// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"crypto/md5"
	"encoding/hex"
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

// statically assert consul hook implements the expected interfaces
var _ interfaces.RunnerPrerunHook = (*consulHook)(nil)

func consulHookTestHarness(t *testing.T) *consulHook {
	logger := testlog.HCLogger(t)

	alloc := mock.Alloc()
	task := alloc.LookupTask("web")
	task.Consul = &structs.Consul{
		Cluster: "default",
	}
	task.Identities = []*structs.WorkloadIdentity{
		{Name: fmt.Sprintf("%s_default", structs.ConsulTaskIdentityNamePrefix)},
	}
	task.Services = []*structs.Service{
		{
			Provider: structs.ServiceProviderConsul,
			Identity: &structs.WorkloadIdentity{Name: "consul-service_webservice", Audience: []string{"consul.io"}},
			Cluster:  "default",
			Name:     "webservice",
			TaskName: "web",
		},
	}

	identitiesToSign := []*structs.WorkloadIdentity{}
	identitiesToSign = append(identitiesToSign, task.Identities...)
	for _, service := range task.Services {
		identitiesToSign = append(identitiesToSign, service.Identity)
	}

	// setup mock signer and sign the identities
	mockSigner := widmgr.NewMockWIDSigner(identitiesToSign)
	signedIDs, err := mockSigner.SignIdentities(1, []*structs.WorkloadIdentityRequest{
		{
			AllocID: alloc.ID,
			WIHandle: structs.WIHandle{
				WorkloadIdentifier: task.Name,
				IdentityName:       task.Identities[0].Name,
			},
		},
		{
			AllocID: alloc.ID,
			WIHandle: structs.WIHandle{
				WorkloadIdentifier: task.Services[0].Name,
				IdentityName:       task.Services[0].Identity.Name,
				WorkloadType:       structs.WorkloadTypeService,
			},
		},
	})
	must.NoError(t, err)

	mockWIDMgr := widmgr.NewMockWIDMgr(signedIDs)

	consulConfigs := map[string]*structsc.ConsulConfig{
		"default": structsc.DefaultConsulConfig(),
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
	return newConsulHook(consulHookCfg)
}

func Test_consulHook_prepareConsulTokensForTask(t *testing.T) {
	ci.Parallel(t)

	hook := consulHookTestHarness(t)
	task := hook.alloc.LookupTask("web")
	hashForDefaultCluster := md5.Sum([]byte("consul_default"))

	tests := []struct {
		name           string
		task           *structs.Task
		tokens         map[string]map[string]string
		wantErr        bool
		errMsg         string
		expectedTokens map[string]map[string]string
	}{
		{
			name:           "empty task",
			task:           nil,
			tokens:         map[string]map[string]string{},
			wantErr:        true,
			errMsg:         "no task specified",
			expectedTokens: map[string]map[string]string{},
		},
		{
			name:    "task with signed identity",
			task:    task,
			tokens:  map[string]map[string]string{},
			wantErr: false,
			errMsg:  "",
			expectedTokens: map[string]map[string]string{
				"default": {
					"consul_default": hex.EncodeToString(hashForDefaultCluster[:]),
				},
			},
		},
		{
			name: "task with unknown identity",
			task: &structs.Task{
				Identities: []*structs.WorkloadIdentity{
					{Name: structs.ConsulTaskIdentityNamePrefix + "_default"}},
				Name: "foo",
			},
			tokens:         map[string]map[string]string{},
			wantErr:        true,
			errMsg:         "unable to find token for workload",
			expectedTokens: map[string]map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hook.prepareConsulTokensForTask(tt.task, nil, tt.tokens)
			if tt.wantErr {
				must.Error(t, err)
				must.ErrorContains(t, err, tt.errMsg)
			} else {
				must.NoError(t, err)
				must.Eq(t, tt.tokens, tt.expectedTokens)
			}
		})
	}
}

func Test_consulHook_prepareConsulTokensForServices(t *testing.T) {
	ci.Parallel(t)

	hook := consulHookTestHarness(t)
	task := hook.alloc.LookupTask("web")
	services := task.Services
	hashForServiceCluster := md5.Sum([]byte("consul-service_webservice"))

	tests := []struct {
		name           string
		services       []*structs.Service
		tokens         map[string]map[string]string
		wantErr        bool
		errMsg         string
		expectedTokens map[string]map[string]string
	}{
		{
			name:           "empty services",
			services:       nil,
			tokens:         map[string]map[string]string{},
			wantErr:        false,
			errMsg:         "",
			expectedTokens: map[string]map[string]string{},
		},
		{
			name:     "services with signed identity",
			services: services,
			tokens:   map[string]map[string]string{},
			wantErr:  false,
			errMsg:   "",
			expectedTokens: map[string]map[string]string{
				"default": {
					"consul-service_webservice": hex.EncodeToString(hashForServiceCluster[:]),
				},
			},
		},
		{
			name: "services with unknown identity",
			services: []*structs.Service{
				{
					Provider: structs.ServiceProviderConsul,
					Identity: &structs.WorkloadIdentity{Name: "consul-service_webservice", Audience: []string{"consul.io"}},
					Cluster:  "default",
					Name:     "foo",
					TaskName: "web",
				},
			},
			tokens:         map[string]map[string]string{},
			wantErr:        true,
			errMsg:         "unable to find token for workload",
			expectedTokens: map[string]map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hook.prepareConsulTokensForServices(tt.services, nil, tt.tokens)
			if tt.wantErr {
				must.Error(t, err)
				must.ErrorContains(t, err, tt.errMsg)
			} else {
				must.NoError(t, err)
				must.Eq(t, tt.tokens, tt.expectedTokens)
			}
		})
	}
}
