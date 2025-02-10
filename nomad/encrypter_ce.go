// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

func getProviderConfigs(srv *Server) (map[string]*structs.KEKProviderConfig, error) {
	providerConfigs := map[string]*structs.KEKProviderConfig{}
	config := srv.GetConfig()
	var active int
	for _, provider := range config.KEKProviderConfigs {
		if provider.Active {
			active++
		}
		if provider.Provider == structs.KEKProviderVaultTransit {
			fallbackVaultConfig(provider, config.GetDefaultVault())
		}

		providerConfigs[provider.ID()] = provider
	}
	if active > 1 {
		return nil, fmt.Errorf(
			"only one server.keyring can be active in Nomad Community Edition")
	}

	if len(srv.config.KEKProviderConfigs) == 0 {
		providerConfigs[string(structs.KEKProviderAEAD)] = &structs.KEKProviderConfig{
			Provider: string(structs.KEKProviderAEAD),
			Active:   true,
		}
	}

	return providerConfigs, nil
}
