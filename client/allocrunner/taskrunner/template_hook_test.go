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

	// Create some alloc hook resources, one with tokens and an empty one.
	defaultToken := uuid.Generate()
	hrTokens := cstructs.NewAllocHookResources()
	hrTokens.SetConsulTokens(
		map[string]map[string]*consulapi.ACLToken{
			structs.ConsulDefaultCluster: {
				fmt.Sprintf("consul_%s", structs.ConsulDefaultCluster): &consulapi.ACLToken{
					SecretID: defaultToken,
				},
			},
		},
	)
	hrEmpty := cstructs.NewAllocHookResources()

	tests := []struct {
		name            string
		taskConsul      *structs.Consul
		groupConsul     *structs.Consul
		hr              *cstructs.AllocHookResources
		wantErrMsg      string
		wantConsulToken string
		legacyFlow      bool
	}{
		{
			// COMPAT remove in 1.9+
			name:            "legecy flow",
			hr:              hrEmpty,
			legacyFlow:      true,
			wantConsulToken: "",
		},
		{
			name:       "task missing Consul token",
			hr:         hrEmpty,
			wantErrMsg: "not found",
		},
		{
			name:            "task without consul blocks uses default cluster",
			hr:              hrTokens,
			wantConsulToken: defaultToken,
		},
		{
			name: "task with consul block at task level",
			hr:   hrTokens,
			taskConsul: &structs.Consul{
				Cluster: structs.ConsulDefaultCluster,
			},
			wantConsulToken: defaultToken,
		},
		{
			name: "task with consul block at group level",
			hr:   hrTokens,
			groupConsul: &structs.Consul{
				Cluster: structs.ConsulDefaultCluster,
			},
			wantConsulToken: defaultToken,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := mock.Alloc()

			task := a.Job.TaskGroups[0].Tasks[0]
			if !tt.legacyFlow {
				task.Identities = []*structs.WorkloadIdentity{
					{Name: fmt.Sprintf("%s_%s",
						structs.ConsulTaskIdentityNamePrefix,
						structs.ConsulDefaultCluster,
					)},
				}
			}

			clientConfig := &config.Config{Region: "global"}
			envBuilder := taskenv.NewBuilder(mock.Node(), a, task, clientConfig.Region)
			taskHooks := trtesting.NewMockTaskHooks()

			conf := &templateHookConfig{
				alloc:         a,
				logger:        logger,
				lifecycle:     taskHooks,
				events:        &trtesting.MockEmitter{},
				clientConfig:  clientConfig,
				envBuilder:    envBuilder,
				hookResources: tt.hr,
			}
			h := &templateHook{
				config:       conf,
				logger:       logger,
				managerLock:  sync.Mutex{},
				driverHandle: nil,
			}
			req := &interfaces.TaskPrestartRequest{
				Task:    a.Job.TaskGroups[0].Tasks[0],
				TaskDir: &allocdir.TaskDir{Dir: "foo"},
			}

			err := h.Prestart(context.Background(), req, nil)
			if tt.wantErrMsg != "" {
				must.Error(t, err)
				must.ErrorContains(t, err, tt.wantErrMsg)
			} else {
				must.NoError(t, err)
			}

			must.Eq(t, tt.wantConsulToken, h.consulToken)
		})
	}
}
