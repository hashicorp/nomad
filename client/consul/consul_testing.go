package consul

import (
	"fmt"
	"sync"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/command/agent/consul"
	testing "github.com/mitchellh/go-testing-interface"
)

// MockConsulOp represents the register/deregister operations.
type MockConsulOp struct {
	Op      string // add, remove, or update
	AllocID string
	Name    string // task or group name
}

func NewMockConsulOp(op, allocID, name string) MockConsulOp {
	switch op {
	case "add", "remove", "update", "alloc_registrations",
		"add_group", "remove_group", "update_group", "update_ttl":
	default:
		panic(fmt.Errorf("invalid consul op: %s", op))
	}
	return MockConsulOp{
		Op:      op,
		AllocID: allocID,
		Name:    name,
	}
}

// MockConsulServiceClient implements the ConsulServiceAPI interface to record
// and log task registration/deregistration.
type MockConsulServiceClient struct {
	ops []MockConsulOp
	mu  sync.Mutex

	logger log.Logger

	// AllocRegistrationsFn allows injecting return values for the
	// AllocRegistrations function.
	AllocRegistrationsFn func(allocID string) (*consul.AllocRegistration, error)
}

func NewMockConsulServiceClient(t testing.T, logger log.Logger) *MockConsulServiceClient {
	logger = logger.Named("mock_consul")
	m := MockConsulServiceClient{
		ops:    make([]MockConsulOp, 0, 20),
		logger: logger,
	}
	return &m
}

func (m *MockConsulServiceClient) UpdateWorkload(old, newSvcs *consul.WorkloadServices) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Trace("UpdateWorkload", "alloc_id", newSvcs.AllocID, "name", newSvcs.Name(),
		"old_services", len(old.Services), "new_services", len(newSvcs.Services),
	)
	m.ops = append(m.ops, NewMockConsulOp("update", newSvcs.AllocID, newSvcs.Name()))
	return nil
}

func (m *MockConsulServiceClient) RegisterWorkload(svcs *consul.WorkloadServices) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Trace("RegisterWorkload", "alloc_id", svcs.AllocID, "name", svcs.Name(),
		"services", len(svcs.Services),
	)
	m.ops = append(m.ops, NewMockConsulOp("add", svcs.AllocID, svcs.Name()))
	return nil
}

func (m *MockConsulServiceClient) RemoveWorkload(svcs *consul.WorkloadServices) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Trace("RemoveWorkload", "alloc_id", svcs.AllocID, "name", svcs.Name(),
		"services", len(svcs.Services),
	)
	m.ops = append(m.ops, NewMockConsulOp("remove", svcs.AllocID, svcs.Name()))
}

func (m *MockConsulServiceClient) AllocRegistrations(allocID string) (*consul.AllocRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Trace("AllocRegistrations", "alloc_id", allocID)
	m.ops = append(m.ops, NewMockConsulOp("alloc_registrations", allocID, ""))

	if m.AllocRegistrationsFn != nil {
		return m.AllocRegistrationsFn(allocID)
	}

	return nil, nil
}

func (m *MockConsulServiceClient) UpdateTTL(checkID, output, status string) error {
	// TODO(tgross): this method is here so we can implement the
	// interface but the locking we need for testing creates a lot
	// of opportunities for deadlocks in testing that will never
	// appear in live code.
	m.logger.Trace("UpdateTTL", "check_id", checkID, "status", status)
	return nil
}

func (m *MockConsulServiceClient) GetOps() []MockConsulOp {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ops
}
