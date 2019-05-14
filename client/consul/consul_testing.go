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
	Task    string
}

func NewMockConsulOp(op, allocID, task string) MockConsulOp {
	if op != "add" && op != "remove" && op != "update" && op != "alloc_registrations" {
		panic(fmt.Errorf("invalid consul op: %s", op))
	}
	return MockConsulOp{
		Op:      op,
		AllocID: allocID,
		Task:    task,
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

func (m *MockConsulServiceClient) UpdateTask(old, newSvcs *consul.TaskServices) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Trace("UpdateTask", "alloc_id", newSvcs.AllocID, "task", newSvcs.Name,
		"old_services", len(old.Services), "new_services", len(newSvcs.Services),
	)
	m.ops = append(m.ops, NewMockConsulOp("update", newSvcs.AllocID, newSvcs.Name))
	return nil
}

func (m *MockConsulServiceClient) RegisterTask(task *consul.TaskServices) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Trace("RegisterTask", "alloc_id", task.AllocID, "task", task.Name,
		"services", len(task.Services),
	)
	m.ops = append(m.ops, NewMockConsulOp("add", task.AllocID, task.Name))
	return nil
}

func (m *MockConsulServiceClient) RemoveTask(task *consul.TaskServices) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Trace("RemoveTask", "alloc_id", task.AllocID, "task", task.Name,
		"services", len(task.Services),
	)
	m.ops = append(m.ops, NewMockConsulOp("remove", task.AllocID, task.Name))
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

func (m *MockConsulServiceClient) GetOps() []MockConsulOp {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ops
}
