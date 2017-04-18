package consul

import (
	"log"

	"github.com/hashicorp/consul/api"
)

// MockCatalog can be used for testing where the CatalogAPI is needed.
type MockCatalog struct {
	logger *log.Logger
}

func NewMockCatalog(l *log.Logger) *MockCatalog {
	return &MockCatalog{logger: l}
}

func (m *MockCatalog) Datacenters() ([]string, error) {
	dcs := []string{"dc1"}
	m.logger.Printf("[DEBUG] mock_consul: Datacenters() -> (%q, nil)", dcs)
	return dcs, nil
}

func (m *MockCatalog) Service(service, tag string, q *api.QueryOptions) ([]*api.CatalogService, *api.QueryMeta, error) {
	m.logger.Printf("[DEBUG] mock_consul: Service(%q, %q, %#v) -> (nil, nil, nil)", service, tag, q)
	return nil, nil, nil
}
