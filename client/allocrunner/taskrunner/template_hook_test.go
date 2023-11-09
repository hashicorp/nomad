// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"sync"
	"testing"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	trtesting "github.com/hashicorp/nomad/client/allocrunner/taskrunner/testing"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func Test_templateHook_Prestart_ConsulWI(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)

	// mock some consul tokens
	hr := cstructs.NewAllocHookResources()
	hr.SetConsulTokens(
		map[string]map[string]*consulapi.ACLToken{
			structs.ConsulDefaultCluster: {
				fmt.Sprintf("consul_%s", structs.ConsulDefaultCluster): &consulapi.ACLToken{
					SecretID: uuid.Generate()},
			},
			"test": {
				"consul_test": &consulapi.ACLToken{SecretID: uuid.Generate()},
			},
		},
	)

	a := mock.Alloc()
	clientConfig := &config.Config{Region: "global"}
	envBuilder := taskenv.NewBuilder(mock.Node(), a, a.Job.TaskGroups[0].Tasks[0], clientConfig.Region)
	taskHooks := trtesting.NewMockTaskHooks()

	conf := &templateHookConfig{
		alloc:         a,
		logger:        logger,
		lifecycle:     taskHooks,
		events:        &trtesting.MockEmitter{},
		clientConfig:  clientConfig,
		envBuilder:    envBuilder,
		hookResources: hr,
	}

	tests := []struct {
		name        string
		req         *interfaces.TaskPrestartRequest
		wantErrMsg  string
		consulToken *consulapi.ACLToken
	}{
		{
			name: "task with no Consul WI",
			req: &interfaces.TaskPrestartRequest{
				Task:    &structs.Task{},
				TaskDir: &allocdir.TaskDir{Dir: "foo"},
			},
		},
		{
			name: "task with Consul WI but no corresponding identity",
			req: &interfaces.TaskPrestartRequest{
				Task: &structs.Task{
					Name:   "foo",
					Consul: &structs.Consul{Cluster: "bar"},
				},
				TaskDir: &allocdir.TaskDir{Dir: "foo"},
			},
			// note: the exact message will vary between CE and ENT because they
			// have different helpers for Consul cluster name lookup
			wantErrMsg: "not found",
		},
		{
			name: "task with Consul WI",
			req: &interfaces.TaskPrestartRequest{
				Task: &structs.Task{
					Name:   "foo",
					Consul: &structs.Consul{Cluster: "default"},
				},
				TaskDir: &allocdir.TaskDir{Dir: "foo"},
			},
			consulToken: hr.GetConsulTokens()[structs.ConsulDefaultCluster]["consul_default"],
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &templateHook{
				config:       conf,
				logger:       logger,
				managerLock:  sync.Mutex{},
				driverHandle: nil,
			}

			err := h.Prestart(context.Background(), tt.req, nil)
			if tt.wantErrMsg != "" {
				must.NotNil(t, err)
				must.ErrorContains(t, err, tt.wantErrMsg)
			} else {
				must.Nil(t, err)
			}

			if tt.consulToken != nil {
				must.Eq(t, tt.consulToken.SecretID, h.consulToken)
			} else {
				must.Eq(t, "", h.consulToken)
			}
		})
	}
}
