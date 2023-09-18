// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package config

import (
	"github.com/hashicorp/go-hclog"
	structsc "github.com/hashicorp/nomad/nomad/structs/config"
)

// GetVaultConfigs returns the set of Vault configurations available for this
// client. In Nomad CE we only use the default Vault.
func (c *Config) GetVaultConfigs(logger hclog.Logger) map[string]*structsc.VaultConfig {
	if c.VaultConfig == nil || !c.VaultConfig.IsEnabled() {
		return nil
	}

	if len(c.VaultConfigs) > 1 {
		logger.Warn("multiple Vault configurations are only supported in Nomad Enterprise")
	}

	return map[string]*structsc.VaultConfig{"default": c.VaultConfig}
}
