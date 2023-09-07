// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

const (
	consulHookName = "consul_hook"
)

type consulHook struct {
	alloc  *structs.Allocation
	config *config.ConsulConfig

	logger hclog.Logger
}

func newConsulHook(
	logger hclog.Logger,
	alloc *structs.Allocation,
	config *config.ConsulConfig) *consulHook {
	return &consulHook{alloc: alloc, config: config, logger: logger}
}

func (ch *consulHook) Name() string {
	return consulHookName
}

func (ch *consulHook) Prerun() error {
	tokensRequired := map[string]consul.JWTLoginRequest{}

	for _, tg := range ch.alloc.Job.TaskGroups {
		for _, s := range tg.Services {
			if s.Provider == "consul" {
				if s.Identity != nil {
					tokensRequired[s.Identity.ServiceName] = consul.JWTLoginRequest{
						JWT:            "",
						Role:           "",
						AuthMethodName: "",
					}
				}
			}
		}
	}

	return nil
}
