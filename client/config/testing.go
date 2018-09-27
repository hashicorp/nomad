package config

import (
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// TestClientConfig returns a default client configuration for test clients.
func TestClientConfig() *Config {
	conf := DefaultConfig()
	conf.VaultConfig.Enabled = helper.BoolToPtr(false)
	conf.DevMode = true
	conf.Node = &structs.Node{
		Reserved: &structs.Resources{
			DiskMB: 0,
		},
	}

	// Loosen GC threshold
	conf.GCDiskUsageThreshold = 98.0
	conf.GCInodeUsageThreshold = 98.0
	return conf
}
