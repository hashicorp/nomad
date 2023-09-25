// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package agent

import (
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

// DefaultEntConfig is an empty config in open source
func DefaultEntConfig() *Config {
	return &Config{
		Reporting: &config.Reporting{
			License: &config.LicenseConfig{
				Enabled: pointer.Of(false),
			},
		},
	}
}
