// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package fingerprint

import "github.com/hashicorp/nomad/nomad/structs/config"

// vaultConfigs returns the set of Vault configurations the fingerprint needs to
// check. In Nomad CE we only check the default Vault.
func (f *VaultFingerprint) vaultConfigs(req *FingerprintRequest) map[string]*config.VaultConfig {
	agentCfg := req.Config
	if agentCfg.VaultConfig == nil || !agentCfg.VaultConfig.IsEnabled() {
		return nil
	}

	return map[string]*config.VaultConfig{"default": agentCfg.VaultConfig}
}
