// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package fingerprint

import "github.com/hashicorp/nomad/nomad/structs/config"

// consulConfigs returns the set of Consul configurations the fingerprint needs
// to check. In Nomad CE we only check the default Consul.
func (f *ConsulFingerprint) consulConfigs(req *FingerprintRequest) map[string]*config.ConsulConfig {
	agentCfg := req.Config
	if agentCfg.ConsulConfig == nil {
		return nil
	}

	if len(req.Config.ConsulConfigs) > 1 {
		f.logger.Warn("multiple Consul configurations are only supported in Nomad Enterprise")
	}

	return map[string]*config.ConsulConfig{"default": agentCfg.ConsulConfig}
}
