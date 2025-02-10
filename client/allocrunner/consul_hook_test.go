// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"testing"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	cstate "github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/taskenv"
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

	task1 := alloc.LookupTask("web")

	task1.Consul = &structs.Consul{
		Cluster: "default",
	}
	task1.Identities = []*structs.WorkloadIdentity{
		{Name: fmt.Sprintf("%s_default", structs.ConsulTaskIdentityNamePrefix)},
	}

	task2 := task1.Copy()
	task2.Name = "extra"
	task2.Services = nil
	alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, task2)

	task1.Services = []*structs.Service{
		{
			Provider: structs.ServiceProviderConsul,
			Identity: &structs.WorkloadIdentity{Name: "consul-service_webservice", Audience: []string{"consul.io"}},
			Cluster:  "default",
			Name:     "${NOMAD_TASK_NAME}service",
			TaskName: "web", // note: this doesn't interpolate
		},
	}

	// setup mock signer but don't sign identities, as we're going to want them
	// interpolated by the WIDMgr
	mockSigner := widmgr.NewMockWIDSigner(nil)
	db := cstate.NewMemDB(logger)

	// the WIDMgr env builder never has the task available
	envBuilder := taskenv.NewBuilder(mock.Node(), alloc, nil, "global")

	mockWIDMgr := widmgr.NewWIDMgr(mockSigner, alloc, db, logger, envBuilder)
	mockWIDMgr.SignForTesting()

	consulConfigs := map[string]*structsc.ConsulConfig{
		"default": structsc.DefaultConsulConfig(),
	}

	hookResources := cstructs.NewAllocHookResources()
	envBuilderFn := func() *taskenv.Builder {
		return taskenv.NewBuilder(mock.Node(), alloc, nil, "global")
	}

	consulHookCfg := consulHookConfig{
		alloc:                   alloc,
		allocdir:                nil,
		widmgr:                  mockWIDMgr,
		consulConfigs:           consulConfigs,
		consulClientConstructor: consul.NewMockConsulClient,
		hookResources:           hookResources,
		envBuilder:              envBuilderFn,
		logger:                  logger,
	}
	return newConsulHook(consulHookCfg)
}

func Test_consulHook_prepareConsulTokensForTask(t *testing.T) {
	ci.Parallel(t)

	hook := consulHookTestHarness(t)
	task := hook.alloc.LookupTask("web")

	wid := task.GetIdentity("consul_default")
	ti := *task.IdentityHandle(wid)
	jwt, err := hook.widmgr.Get(ti)
	must.NoError(t, err)
	hashJWT1 := md5.Sum([]byte(jwt.JWT))

	task2 := hook.alloc.LookupTask("extra")
	ti2 := *task2.IdentityHandle(wid)
	jwt2, err := hook.widmgr.Get(ti2)
	must.NoError(t, err)
	hashJWT2 := md5.Sum([]byte(jwt2.JWT))

	tests := []struct {
		name           string
		tasks          []*structs.Task
		wantErr        bool
		errMsg         string
		expectedTokens map[string]map[string]*consulapi.ACLToken
	}{
		{
			name:           "empty task",
			tasks:          nil,
			wantErr:        true,
			errMsg:         "no task specified",
			expectedTokens: map[string]map[string]*consulapi.ACLToken{},
		},
		{
			name:    "task with signed identity",
			tasks:   []*structs.Task{task},
			wantErr: false,
			errMsg:  "",
			expectedTokens: map[string]map[string]*consulapi.ACLToken{
				"default": {
					"consul_default/web": &consulapi.ACLToken{
						AccessorID: hex.EncodeToString(hashJWT1[:]),
						SecretID:   hex.EncodeToString(hashJWT1[:]),
					},
				},
			},
		},
		{
			name:    "multiple tasks with signed identities",
			tasks:   []*structs.Task{task, task2},
			wantErr: false,
			errMsg:  "",
			expectedTokens: map[string]map[string]*consulapi.ACLToken{
				"default": {
					"consul_default/web": &consulapi.ACLToken{
						AccessorID: hex.EncodeToString(hashJWT1[:]),
						SecretID:   hex.EncodeToString(hashJWT1[:]),
					},
					"consul_default/extra": &consulapi.ACLToken{
						AccessorID: hex.EncodeToString(hashJWT2[:]),
						SecretID:   hex.EncodeToString(hashJWT2[:]),
					},
				},
			},
		},
		{
			name: "task with unknown identity",
			tasks: []*structs.Task{{
				Identities: []*structs.WorkloadIdentity{
					{Name: structs.ConsulTaskIdentityNamePrefix + "_default"}},
				Name: "foo",
			}},
			wantErr:        true,
			errMsg:         "unable to find token for workload",
			expectedTokens: map[string]map[string]*consulapi.ACLToken{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := map[string]map[string]*consulapi.ACLToken{}
			for _, task := range tt.tasks {
				err := hook.prepareConsulTokensForTask(task, nil, tokens)
				if tt.wantErr {
					must.Error(t, err)
					must.ErrorContains(t, err, tt.errMsg)
				} else {
					must.NoError(t, err)
				}
			}
			must.Eq(t, tt.expectedTokens, tokens)
		})
	}
}

func Test_consulHook_prepareConsulTokensForServices(t *testing.T) {
	ci.Parallel(t)

	hook := consulHookTestHarness(t)
	task := hook.alloc.LookupTask("web")
	services := task.Services
	hook.envBuilder.UpdateTask(hook.alloc, task)
	env := hook.envBuilder.Build()
	hashedJWT := make(map[string]string)

	for _, s := range services {
		widHandle := *s.IdentityHandle(env.ReplaceEnv)
		jwt, err := hook.widmgr.Get(widHandle)
		must.NoError(t, err)

		hash := md5.Sum([]byte(jwt.JWT))
		hashedJWT[widHandle.InterpolatedWorkloadIdentifier] = hex.EncodeToString(hash[:])
	}

	tests := []struct {
		name           string
		services       []*structs.Service
		wantErr        bool
		errMsg         string
		expectedTokens map[string]map[string]*consulapi.ACLToken
	}{
		{
			name:           "empty services",
			services:       nil,
			wantErr:        false,
			errMsg:         "",
			expectedTokens: map[string]map[string]*consulapi.ACLToken{},
		},
		{
			name:     "services with signed identity",
			services: services,
			wantErr:  false,
			errMsg:   "",
			expectedTokens: map[string]map[string]*consulapi.ACLToken{
				"default": {
					"consul-service_webservice": {
						AccessorID: hashedJWT["webservice"],
						SecretID:   hashedJWT["webservice"],
					},
				},
			},
		},
		{
			name: "services with unknown identity",
			services: []*structs.Service{
				{
					Provider: structs.ServiceProviderConsul,
					Identity: &structs.WorkloadIdentity{
						Name: "consul-service_webservice", Audience: []string{"consul.io"}},
					Cluster:  "default",
					Name:     "foo",
					TaskName: "web",
				},
			},
			wantErr:        true,
			errMsg:         "unable to find token for workload",
			expectedTokens: map[string]map[string]*consulapi.ACLToken{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := map[string]map[string]*consulapi.ACLToken{}
			err := hook.prepareConsulTokensForServices(tt.services, nil, tokens, env)
			if tt.wantErr {
				must.Error(t, err)
				must.ErrorContains(t, err, tt.errMsg)
			} else {
				must.NoError(t, err)
				must.Eq(t, tt.expectedTokens, tokens)
			}
		})
	}
}

func Test_consulHook_Postrun(t *testing.T) {
	ci.Parallel(t)

	// no-op must be safe
	hook := consulHookTestHarness(t)
	must.NoError(t, hook.Postrun())

	task := hook.alloc.LookupTask("web")
	tokens := map[string]map[string]*consulapi.ACLToken{}
	must.NoError(t, hook.prepareConsulTokensForTask(task, nil, tokens))
	hook.hookResources.SetConsulTokens(tokens)
	must.MapLen(t, 1, tokens)

	// gracefully handle wrong tokens
	otherTokens := map[string]map[string]*consulapi.ACLToken{
		"default": {"foo": &consulapi.ACLToken{AccessorID: "foo", SecretID: "foo"}}}
	must.NoError(t, hook.revokeTokens(otherTokens))

	// hook resources should be cleared
	must.NoError(t, hook.Postrun())
	tokens = hook.hookResources.GetConsulTokens()
	must.MapEmpty(t, tokens["default"])
}
