// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package allocrunner

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/consul"
	structsc "github.com/hashicorp/nomad/nomad/structs/config"
)

func getConsulClients(configs map[string]*structsc.ConsulConfig, logger hclog.Logger) (map[string]consul.Client, error) {
	conf := configs["default"] // Nomad CE only supports a single Consul client

	client, err := consul.NewConsulClient(conf, logger)

	return map[string]consul.Client{"default": client}, err
}
