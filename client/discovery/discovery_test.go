package discovery

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"
)

// mockDiscovery is a basic discovery provider which just logs
// various things about the calls to it.
type mockDiscovery struct {
	registered map[string]int
	enabled    bool
	name       string
}

// newMockDiscovery makes a new MockDiscovery.
func newMockDiscovery(ctx *context) (provider, error) {
	return &mockDiscovery{
		registered: make(map[string]int),
		enabled:    true,
		name:       "mock",
	}, nil
}

func (m *mockDiscovery) Name() string {
	return m.name
}

func (m *mockDiscovery) Enabled() bool {
	return m.enabled
}

func (m *mockDiscovery) Register(name string, port int) error {
	m.registered[name] = port
	return nil
}

func (m *mockDiscovery) Deregister(name string) error {
	delete(m.registered, name)
	return nil
}

// newDisabledMockDiscovery makes a new discovery provider which
// is disabled.
func newDisabledMockDiscovery(ctx *context) (provider, error) {
	return &mockDiscovery{
		registered: make(map[string]int),
		enabled:    false,
		name:       "disabled_mock",
	}, nil
}

// newErrorMockDiscovery makes a new discovery provider which
// returns an error on creation.
func newErrorMockDiscovery(ctx *context) (provider, error) {
	disc := &mockDiscovery{
		registered: make(map[string]int),
		enabled:    true,
		name:       "error_mock",
	}
	err := errors.New("failed")
	return disc, err
}

func TestDiscoveryLayer_Fails(t *testing.T) {
	builtins = []factory{
		newMockDiscovery,
		newDisabledMockDiscovery,
		newErrorMockDiscovery,
	}

	// Returns error if a layer fails to initialize
	dl, err := NewDiscoveryLayer(nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "error_mock") {
		t.Fatalf("expected mock discovery error, got: %#v", err)
	}
	if dl != nil {
		t.Fatalf("discovery layer should be nil")
	}
}

func TestDiscoveryLayer(t *testing.T) {
	builtins = []factory{
		newMockDiscovery,
		newDisabledMockDiscovery,
	}

	// Create a logger
	logBuf := new(bytes.Buffer)
	logger := log.New(logBuf, "", log.LstdFlags)

	// Create the discovery layer
	dl, err := NewDiscoveryLayer(nil, logger, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check the enabled providers
	if p := dl.Providers(); len(p) != 1 || p[0] != "mock" {
		t.Fatalf("expected only mock provider, got: %v", p)
	}
	provider := dl.providers[0].(*mockDiscovery)

	// Register a service
	dl.Register("foobar", 123)
	if port, ok := provider.registered["foobar"]; !ok || port != 123 {
		t.Fatalf("bad registered services: %v", provider.registered)
	}
	logs := logBuf.String()
	if !strings.Contains(logs, `registered "foobar" with mock`) {
		t.Fatalf("should log registration\n\n%s", logs)
	}
	logBuf.Reset()

	// Deregister the service
	dl.Deregister("foobar")
	if _, ok := provider.registered["foobar"]; ok {
		t.Fatalf("should deregister")
	}
	logs = logBuf.String()
	if !strings.Contains(logs, `deregistered "foobar" from mock`) {
		t.Fatalf("should log deregistration\n\n%s", logs)
	}
}
