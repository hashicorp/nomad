package discovery

import (
	"errors"
	"strings"
)

// MockDiscovery is a basic discovery provider which just logs
// various things about the calls to it.
type MockDiscovery struct {
	Registered map[string]int
	enabled    bool
	name       string
}

// NewMockDiscovery makes a new MockDiscovery.
func NewMockDiscovery(ctx Context) (Provider, error) {
	return &MockDiscovery{
		Registered: make(map[string]int),
		enabled:    true,
		name:       "mock",
	}, nil
}

func (m *MockDiscovery) Name() string {
	return m.name
}

func (m *MockDiscovery) Enabled() bool {
	return m.enabled
}

func (m *MockDiscovery) DiscoverName(parts []string) string {
	return strings.Join(parts, ".")
}

func (m *MockDiscovery) Register(name string, port int) error {
	m.Registered[name] = port
	return nil
}

func (m *MockDiscovery) Deregister(name string) error {
	delete(m.Registered, name)
	return nil
}

// NewDisabledMockDiscovery makes a new discovery provider which
// is disabled.
func NewDisabledMockDiscovery(ctx Context) (Provider, error) {
	return &MockDiscovery{
		Registered: make(map[string]int),
		enabled:    false,
		name:       "disabled_mock",
	}, nil
}

// NewErrorMockDiscovery makes a new discovery provider which
// returns an error on creation.
func NewErrorMockDiscovery(ctx Context) (Provider, error) {
	disc := &MockDiscovery{
		Registered: make(map[string]int),
		enabled:    true,
		name:       "error_mock",
	}
	err := errors.New("failed")
	return disc, err
}
