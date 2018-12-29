package client

import (
	"github.com/hashicorp/nomad/client/config"
	consulApi "github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/plugins/shared/catalog"
	"github.com/hashicorp/nomad/plugins/shared/singleton"
	"github.com/mitchellh/go-testing-interface"
)

// TestClient creates an in-memory client for testing purposes and returns a
// cleanup func to shutdown the client and remove the alloc and state dirs.
//
// There is no need to override the AllocDir or StateDir as they are randomized
// and removed in the returned cleanup function. If they are overridden in the
// callback then the caller still must run the returned cleanup func.
func TestClient(t testing.T, cb func(c *config.Config)) (*Client, func()) {
	conf, cleanup := config.TestClientConfig(t)

	// Tighten the fingerprinter timeouts (must be done in client package
	// to avoid circular dependencies)
	if conf.Options == nil {
		conf.Options = make(map[string]string)
	}
	conf.Options[fingerprint.TightenNetworkTimeoutsConfig] = "true"

	logger := testlog.HCLogger(t)
	conf.Logger = logger

	if cb != nil {
		cb(conf)
	}

	// Set the plugin loaders
	if conf.PluginLoader == nil {
		conf.PluginLoader = catalog.TestPluginLoaderWithOptions(t, "", conf.Options, nil)
		conf.PluginSingletonLoader = singleton.NewSingletonLoader(logger, conf.PluginLoader)
	}
	catalog := consul.NewMockCatalog(logger)
	mockService := consulApi.NewMockConsulServiceClient(t, logger)
	client, err := NewClient(conf, catalog, mockService)
	if err != nil {
		cleanup()
		t.Fatalf("err: %v", err)
	}
	return client, func() {
		// Shutdown client
		client.Shutdown()

		// Call TestClientConfig cleanup
		cleanup()
	}
}
