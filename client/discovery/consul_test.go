package discovery

import (
	"bytes"
	"io"
	"log"
	"os"
	"testing"

	consulapi "github.com/hashicorp/consul/api"
	consultest "github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestConsulDiscovery_Register(t *testing.T) {
	srv := consultest.NewTestServerConfig(t, func(c *consultest.TestServerConfig) {
		c.Bootstrap = false
	})
	defer srv.Stop()

	// Make the consul client
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = srv.HTTPAddr
	consulClient, err := consulapi.NewClient(consulConfig)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Build the context
	conf := &config.Config{}
	logBuf := new(bytes.Buffer)
	logger := log.New(io.MultiWriter(logBuf, os.Stdout), "", log.LstdFlags)
	node := &structs.Node{}
	ctx := Context{
		config: conf,
		logger: logger,
		node:   node,
	}

	// Create the discovery layer
	disc, err := NewConsulDiscovery(ctx)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Returns false if not enabled
	if disc.Enabled() {
		t.Fatalf("should not be enabled")
	}

	// Enable the discovery layer
	conf.Options = map[string]string{
		"discovery.consul.enable":  "true",
		"discovery.consul.address": srv.HTTPAddr,
	}
	disc, err = NewConsulDiscovery(ctx)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !disc.Enabled() {
		t.Fatalf("should be enabled")
	}

	// Should register a service
	if err := disc.Register("alloc1", "foobar", 123); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check that the service exists
	services, err := consulClient.Agent().Services()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	for _, svc := range services {
		if svc.Service == "foobar" {
			if svc.ID != "foobar:alloc1" {
				t.Fatalf("bad id: %s", svc.ID)
			}
			if svc.Port != 123 {
				t.Fatalf("bad port: %d", svc.Port)
			}
			goto REGISTERED
		}
	}
	t.Fatalf("missing service")

REGISTERED:
	// Deregister the service
	if err := disc.Deregister("alloc1", "foobar"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check that the service is gone
	services, err = consulClient.Agent().Services()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	for _, svc := range services {
		if svc.Service == "foobar" {
			t.Fatalf("foobar service should be deregistered")
		}
	}
}
