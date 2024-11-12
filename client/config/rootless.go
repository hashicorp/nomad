// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import sconfig "github.com/hashicorp/nomad/nomad/structs/config"

type RootlessConfig struct {
	MounterSocket string
}

func RootlessConfigFromAgent(r *sconfig.RootlessConfig) *RootlessConfig {
	if r == nil {
		return nil
	}

	return &RootlessConfig{
		MounterSocket: r.MounterSocket,
	}
}
