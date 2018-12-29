package consul

import (
	"fmt"
	"log"
	"sync"

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

// MockAgent is a fake in-memory Consul backend for ServiceClient.
type MockAgent struct {
	// maps of what services and checks have been registered
	services map[string]*api.AgentServiceRegistration
	checks   map[string]*api.AgentCheckRegistration
	mu       sync.Mutex

	// when UpdateTTL is called the check ID will have its counter inc'd
	checkTTLs map[string]int

	// What check status to return from Checks()
	checkStatus string
}

// NewMockAgent that returns all checks as passing.
func NewMockAgent() *MockAgent {
	return &MockAgent{
		services:    make(map[string]*api.AgentServiceRegistration),
		checks:      make(map[string]*api.AgentCheckRegistration),
		checkTTLs:   make(map[string]int),
		checkStatus: api.HealthPassing,
	}
}

// SetStatus that Checks() should return. Returns old status value.
func (c *MockAgent) SetStatus(s string) string {
	c.mu.Lock()
	old := c.checkStatus
	c.checkStatus = s
	c.mu.Unlock()
	return old
}

func (c *MockAgent) Services() (map[string]*api.AgentService, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	r := make(map[string]*api.AgentService, len(c.services))
	for k, v := range c.services {
		r[k] = &api.AgentService{
			ID:                v.ID,
			Service:           v.Name,
			Tags:              make([]string, len(v.Tags)),
			Port:              v.Port,
			Address:           v.Address,
			EnableTagOverride: v.EnableTagOverride,
		}
		copy(r[k].Tags, v.Tags)
	}
	return r, nil
}

func (c *MockAgent) Checks() (map[string]*api.AgentCheck, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	r := make(map[string]*api.AgentCheck, len(c.checks))
	for k, v := range c.checks {
		r[k] = &api.AgentCheck{
			CheckID:     v.ID,
			Name:        v.Name,
			Status:      c.checkStatus,
			Notes:       v.Notes,
			ServiceID:   v.ServiceID,
			ServiceName: c.services[v.ServiceID].Name,
		}
	}
	return r, nil
}

func (c *MockAgent) CheckRegister(check *api.AgentCheckRegistration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[check.ID] = check

	// Be nice and make checks reachable-by-service
	scheck := check.AgentServiceCheck
	c.services[check.ServiceID].Checks = append(c.services[check.ServiceID].Checks, &scheck)
	return nil
}

func (c *MockAgent) CheckDeregister(checkID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.checks, checkID)
	delete(c.checkTTLs, checkID)
	return nil
}

func (c *MockAgent) ServiceRegister(service *api.AgentServiceRegistration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services[service.ID] = service
	return nil
}

func (c *MockAgent) ServiceDeregister(serviceID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.services, serviceID)
	return nil
}

func (c *MockAgent) UpdateTTL(id string, output string, status string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	check, ok := c.checks[id]
	if !ok {
		return fmt.Errorf("unknown check id: %q", id)
	}
	// Flip initial status to passing
	check.Status = "passing"
	c.checkTTLs[id]++
	return nil
}
