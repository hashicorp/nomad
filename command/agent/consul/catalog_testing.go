// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"sync"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
)

// MockNamespaces is a mock implementation of NamespaceAPI.
type MockNamespaces struct {
	namespaces []*api.Namespace
}

var _ NamespaceAPI = (*MockNamespaces)(nil)

// NewMockNamespaces creates a MockNamespaces with the given namespaces, and
// will automatically add the "default" namespace if not included.
func NewMockNamespaces(namespaces []string) *MockNamespaces {
	list := slices.Clone(namespaces)
	if !slices.Contains(list, "default") {
		list = append(list, "default")
	}
	sort.Strings(list)

	data := make([]*api.Namespace, 0, len(list))
	for _, namespace := range list {
		data = append(data, &api.Namespace{
			Name: namespace,
		})
	}

	return &MockNamespaces{
		namespaces: data,
	}
}

// List implements NamespaceAPI
func (m *MockNamespaces) List(*api.QueryOptions) ([]*api.Namespace, *api.QueryMeta, error) {
	result := make([]*api.Namespace, len(m.namespaces))
	copy(result, m.namespaces)
	return result, new(api.QueryMeta), nil
}

// MockCatalog can be used for testing where the CatalogAPI is needed.
type MockCatalog struct {
	logger hclog.Logger
}

var _ CatalogAPI = (*MockCatalog)(nil)

func NewMockCatalog(l hclog.Logger) *MockCatalog {
	return &MockCatalog{logger: l.Named("mock_consul")}
}

func (m *MockCatalog) Datacenters() ([]string, error) {
	dcs := []string{"dc1"}
	m.logger.Trace("Datacenters()", "dcs", dcs, "error", "nil")
	return dcs, nil
}

func (m *MockCatalog) Service(service, tag string, q *api.QueryOptions) ([]*api.CatalogService, *api.QueryMeta, error) {
	m.logger.Trace("Services()", "service", service, "tag", tag, "query_options", q)
	return nil, nil, nil
}

// MockAgent is a fake in-memory Consul backend for ServiceClient.
type MockAgent struct {
	// services tracks what services have been registered, per namespace
	services map[string]map[string]*api.AgentServiceRegistration

	// checks tracks what checks have been registered, per namespace
	checks map[string]map[string]*api.AgentCheckRegistration

	// hits is the total number of times agent methods have been called
	hits int

	// ent indicates whether the agent is mocking an enterprise consul
	ent bool

	// namespaces indicates whether the agent is mocking consul with namespaces
	// feature enabled
	namespaces bool

	// mu guards above fields
	mu sync.Mutex

	// checkTTLS counts calls to UpdateTTL for each check, per namespace
	checkTTLs map[string]map[string]int

	// What check status to return from Checks()
	checkStatus string
}

var _ AgentAPI = (*MockAgent)(nil)

type Features struct {
	Enterprise bool
	Namespaces bool
}

// NewMockAgent that returns all checks as passing.
func NewMockAgent(f Features) *MockAgent {
	return &MockAgent{
		services:    make(map[string]map[string]*api.AgentServiceRegistration),
		checks:      make(map[string]map[string]*api.AgentCheckRegistration),
		checkTTLs:   make(map[string]map[string]int),
		checkStatus: api.HealthPassing,

		ent:        f.Enterprise,
		namespaces: f.Namespaces,
	}
}

// getHits returns how many Consul Agent API calls have been made.
func (c *MockAgent) getHits() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits
}

// SetStatus that Checks() should return. Returns old status value.
func (c *MockAgent) SetStatus(s string) string {
	c.mu.Lock()
	old := c.checkStatus
	c.checkStatus = s
	c.mu.Unlock()
	return old
}

func (c *MockAgent) Self() (map[string]map[string]interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits++

	version := "1.9.5"
	build := "1.9.5:22ce6c6a"
	if c.ent {
		version = "1.9.5+ent"
		build = "1.9.5+ent:22ce6c6a"
	}

	stats := make(map[string]interface{})
	if c.ent {
		if c.namespaces {
			stats = map[string]interface{}{
				"license": map[string]interface{}{
					"features": "Namespaces,",
				},
			}
		}
	}

	return map[string]map[string]interface{}{
		"Config": {
			"Datacenter": "dc1",
			"NodeName":   "x52",
			"NodeID":     "9e7bf42e-a0b4-61b7-24f9-66dead411f0f",
			"Revision":   "22ce6c6ad",
			"Server":     true,
			"Version":    version,
		},
		"Stats": stats,
		"Member": {
			"Addr":        "127.0.0.1",
			"DelegateCur": 4,
			"DelegateMax": 5,
			"DelegateMin": 2,
			"Name":        "rusty",
			"Port":        8301,
			"ProtocolCur": 2,
			"ProtocolMax": 5,
			"ProtocolMin": 1,
			"Status":      1,
			"Tags": map[string]interface{}{
				"build": build,
			},
		},
		"xDS": {
			"SupportedProxies": map[string]interface{}{
				"envoy": []interface{}{
					"1.14.2",
					"1.13.2",
					"1.12.4",
					"1.11.2",
				},
			},
		},
	}, nil
}

func getNamespace(q *api.QueryOptions) string {
	if q == nil || q.Namespace == "" {
		return "default"
	}
	return q.Namespace
}

// ServicesWithFilterOpts implements AgentAPI
func (c *MockAgent) ServicesWithFilterOpts(_ string, q *api.QueryOptions) (map[string]*api.AgentService, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.hits++
	namespace := getNamespace(q)

	r := make(map[string]*api.AgentService, len(c.services))
	for k, v := range c.services[namespace] {
		r[k] = &api.AgentService{
			ID:                v.ID,
			Service:           v.Name,
			Tags:              make([]string, len(v.Tags)),
			Meta:              maps.Clone(v.Meta),
			Port:              v.Port,
			Address:           v.Address,
			EnableTagOverride: v.EnableTagOverride,
		}
		copy(r[k].Tags, v.Tags)
	}
	return r, nil
}

// ChecksWithFilterOpts implements AgentAPI
func (c *MockAgent) ChecksWithFilterOpts(_ string, q *api.QueryOptions) (map[string]*api.AgentCheck, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.hits++
	namespace := getNamespace(q)

	r := make(map[string]*api.AgentCheck, len(c.checks))
	for k, v := range c.checks[namespace] {
		r[k] = &api.AgentCheck{
			CheckID:     v.ID,
			Name:        v.Name,
			Status:      c.checkStatus,
			Notes:       v.Notes,
			ServiceID:   v.ServiceID,
			ServiceName: c.services[namespace][v.ServiceID].Name,
		}
	}
	return r, nil
}

// CheckRegs returns the raw AgentCheckRegistrations registered with this mock
// agent, across all namespaces.
func (c *MockAgent) CheckRegs() []*api.AgentCheckRegistration {
	c.mu.Lock()
	defer c.mu.Unlock()

	regs := make([]*api.AgentCheckRegistration, 0, len(c.checks))
	for namespace := range c.checks {
		for _, check := range c.checks[namespace] {
			regs = append(regs, check)
		}
	}
	return regs
}

// CheckRegister implements AgentAPI
func (c *MockAgent) CheckRegister(check *api.AgentCheckRegistration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.checkRegister(check)
}

// checkRegister registers a check; c.mu must be held.
func (c *MockAgent) checkRegister(check *api.AgentCheckRegistration) error {
	c.hits++

	// Consul will set empty Namespace to default, do the same here
	if check.Namespace == "" {
		check.Namespace = "default"
	}

	if c.checks[check.Namespace] == nil {
		c.checks[check.Namespace] = make(map[string]*api.AgentCheckRegistration)
	}

	c.checks[check.Namespace][check.ID] = check

	// Be nice and make checks reachable-by-service
	serviceCheck := check.AgentServiceCheck

	if c.services[check.Namespace] == nil {
		c.services[check.Namespace] = make(map[string]*api.AgentServiceRegistration)
	}

	// replace existing check if one with same id already exists
	replace := false
	for i := 0; i < len(c.services[check.Namespace][check.ServiceID].Checks); i++ {
		if c.services[check.Namespace][check.ServiceID].Checks[i].CheckID == check.CheckID {
			c.services[check.Namespace][check.ServiceID].Checks[i] = &check.AgentServiceCheck
			replace = true
			break
		}
	}

	if !replace {
		c.services[check.Namespace][check.ServiceID].Checks = append(c.services[check.Namespace][check.ServiceID].Checks, &serviceCheck)
	}
	return nil
}

// CheckDeregisterOpts implements AgentAPI
func (c *MockAgent) CheckDeregisterOpts(checkID string, q *api.QueryOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.hits++
	namespace := getNamespace(q)

	delete(c.checks[namespace], checkID)
	delete(c.checkTTLs[namespace], checkID)
	return nil
}

// ServiceRegister implements AgentAPI
func (c *MockAgent) ServiceRegister(service *api.AgentServiceRegistration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.hits++

	// Consul will set empty Namespace to default, do the same here
	if service.Namespace == "" {
		service.Namespace = "default"
	}

	if c.services[service.Namespace] == nil {
		c.services[service.Namespace] = make(map[string]*api.AgentServiceRegistration)
	}
	c.services[service.Namespace][service.ID] = service

	// as of Nomad v1.4.x registering service now also registers its checks
	for _, check := range service.Checks {
		if err := c.checkRegister(&api.AgentCheckRegistration{
			ID:                check.CheckID,
			Name:              check.Name,
			ServiceID:         service.ID,
			AgentServiceCheck: *check,
			Namespace:         service.Namespace,
		}); err != nil {
			return err
		}
	}

	return nil
}

// ServiceDeregisterOpts implements AgentAPI
func (c *MockAgent) ServiceDeregisterOpts(serviceID string, q *api.QueryOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.hits++
	namespace := getNamespace(q)

	delete(c.services[namespace], serviceID)

	for k, v := range c.checks[namespace] {
		if v.ServiceID == serviceID {
			delete(c.checks[namespace], k)
			delete(c.checkTTLs[namespace], k)
		}
	}
	return nil
}

// UpdateTTLOpts implements AgentAPI
func (c *MockAgent) UpdateTTLOpts(id string, output string, status string, q *api.QueryOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.hits++
	namespace := getNamespace(q)

	checks, nsExists := c.checks[namespace]
	if !nsExists {
		return fmt.Errorf("unknown checks namespace: %q", namespace)
	}

	check, checkExists := checks[id]
	if !checkExists {
		return fmt.Errorf("unknown check: %s/%s", namespace, id)
	}

	// Flip initial status to passing
	// todo(shoenig) why not just set to the given status?
	check.Status = "passing"
	c.checkTTLs[namespace][id]++

	return nil
}

// a convenience method for looking up a registered service by name
func (c *MockAgent) lookupService(namespace, name string) []*api.AgentServiceRegistration {
	c.mu.Lock()
	defer c.mu.Unlock()

	var services []*api.AgentServiceRegistration
	for _, service := range c.services[namespace] {
		if service.Name == name {
			services = append(services, service)
		}
	}
	return services
}
