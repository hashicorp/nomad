// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"sync"
	"testing"

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
		map[string]map[string]string{
			structs.ConsulDefaultCluster: {
				fmt.Sprintf("consul_%s", structs.ConsulDefaultCluster): uuid.Generate(),
			},
			"test": {
				"consul_test": uuid.Generate(),
			},
		},
	)

	a := mock.Alloc()
	clientConfig := &config.Config{Region: "global"}
	envBuilder := taskenv.NewBuilder(mock.Node(), a, a.Job.TaskGroups[0].Tasks[0], clientConfig.Region)
	taskHooks := trtesting.NewMockTaskHooks()

	conf := &templateHookConfig{
		logger:        logger,
		lifecycle:     taskHooks,
		events:        &mockEmitter{},
		clientConfig:  clientConfig,
		envBuilder:    envBuilder,
		hookResources: hr,
	}

	tests := []struct {
		name        string
		req         *interfaces.TaskPrestartRequest
		wantErr     bool
		wantErrMsg  string
		consulToken string
	}{
		{
			"task with no Consul WI",
			&interfaces.TaskPrestartRequest{
				Task:    &structs.Task{},
				TaskDir: &allocdir.TaskDir{Dir: "foo"},
			},
			false,
			"",
			"",
		},
		{
			"task with Consul WI but no corresponding identity",
			&interfaces.TaskPrestartRequest{
				Task: &structs.Task{
					Name:   "foo",
					Consul: &structs.Consul{Cluster: "bar"},
				},
				TaskDir: &allocdir.TaskDir{Dir: "foo"},
			},
			true,
			"consul task foo uses workload identity, but unable to find a consul token for that task",
			"",
		},
		{
			"task with Consul WI",
			&interfaces.TaskPrestartRequest{
				Task: &structs.Task{
					Name:   "foo",
					Consul: &structs.Consul{Cluster: "default"},
				},
				TaskDir: &allocdir.TaskDir{Dir: "foo"},
			},
			false,
			"",
			hr.GetConsulTokens()[structs.ConsulDefaultCluster]["consul_default"],
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
			if tt.wantErr {
				must.NotNil(t, err)
				must.Eq(t, tt.wantErrMsg, err.Error())
			} else {
				must.Nil(t, err)
			}

			must.Eq(t, tt.consulToken, h.consulToken)
		})
	}
}
