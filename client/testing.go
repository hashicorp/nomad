package client

import (
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/go-testing-interface"
)

// TestClient creates an in-memory client for testing purposes.
func TestClient(t testing.T, cb func(c *config.Config)) *Client {
	conf := config.DefaultConfig()
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

	// Tighten the fingerprinter timeouts
	if conf.Options == nil {
		conf.Options = make(map[string]string)
	}
	conf.Options[fingerprint.TightenNetworkTimeoutsConfig] = "true"

	if cb != nil {
		cb(conf)
	}

	logger := testlog.Logger(t)
	catalog := consul.NewMockCatalog(logger)
	mockService := newMockConsulServiceClient(t)
	mockService.logger = logger
	client, err := NewClient(conf, catalog, mockService, logger)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return client
}
