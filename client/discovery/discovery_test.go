package discovery

import (
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"testing"
)

func TestDiscoveryLayer_Fails(t *testing.T) {
	providers := []Factory{
		NewMockDiscovery,
		NewDisabledMockDiscovery,
		NewErrorMockDiscovery,
	}

	// Returns error if a layer fails to initialize
	dl, err := NewDiscoveryLayer(providers, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "error_mock") {
		t.Fatalf("expected mock discovery error, got: %#v", err)
	}
	if dl != nil {
		t.Fatalf("discovery layer should be nil")
	}
}

func TestDiscoveryLayer(t *testing.T) {
	providers := []Factory{
		NewMockDiscovery,
		NewDisabledMockDiscovery,
	}

	// Create a logger
	logBuf := new(bytes.Buffer)
	logger := log.New(io.MultiWriter(logBuf, os.Stdout), "", log.LstdFlags)

	// Create the discovery layer
	dl, err := NewDiscoveryLayer(providers, nil, logger, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check the enabled providers
	if p := dl.EnabledProviders(); len(p) != 1 || p[0] != "mock" {
		t.Fatalf("expected only mock provider, got: %v", p)
	}
	provider := dl.Providers[0].(*MockDiscovery)

	// Register a service
	dl.Register([]string{"foo", "bar"}, 123)
	if port, ok := provider.Registered["foo.bar"]; !ok || port != 123 {
		t.Fatalf("bad registered services: %v", provider.Registered)
	}
	logs := logBuf.String()
	if !strings.Contains(logs, `registered "foo.bar" with mock`) {
		t.Fatalf("should log registration\n\n%s", logs)
	}
	logBuf.Reset()

	// Deregister the service
	dl.Deregister([]string{"foo", "bar"})
	if _, ok := provider.Registered["foo.bar"]; ok {
		t.Fatalf("should deregister")
	}
	logs = logBuf.String()
	if !strings.Contains(logs, `deregistered "foo.bar" from mock`) {
		t.Fatalf("should log deregistration\n\n%s", logs)
	}
}
