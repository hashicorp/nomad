package consul

import (
	"fmt"
	"log"
	"sync"

	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/mitchellh/go-testing-interface"
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
	Ops []MockConsulOp
	mu  sync.Mutex

	Logger *log.Logger

	// AllocRegistrationsFn allows injecting return values for the
	// AllocRegistrations function.
	AllocRegistrationsFn func(allocID string) (*consul.AllocRegistration, error)
}

func NewMockConsulServiceClient(t testing.T) *MockConsulServiceClient {
	m := MockConsulServiceClient{
		Ops:    make([]MockConsulOp, 0, 20),
		Logger: testlog.Logger(t),
	}
	return &m
}

func (m *MockConsulServiceClient) UpdateTask(old, new *consul.TaskServices) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Logger.Printf("[TEST] mock_consul: UpdateTask(alloc: %s, task: %s)", new.AllocID[:6], new.Name)
	m.Ops = append(m.Ops, NewMockConsulOp("update", new.AllocID, new.Name))
	return nil
}

func (m *MockConsulServiceClient) RegisterTask(task *consul.TaskServices) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Logger.Printf("[TEST] mock_consul: RegisterTask(alloc: %s, task: %s)", task.AllocID, task.Name)
	m.Ops = append(m.Ops, NewMockConsulOp("add", task.AllocID, task.Name))
	return nil
}

func (m *MockConsulServiceClient) RemoveTask(task *consul.TaskServices) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Logger.Printf("[TEST] mock_consul: RemoveTask(%q, %q)", task.AllocID, task.Name)
	m.Ops = append(m.Ops, NewMockConsulOp("remove", task.AllocID, task.Name))
}

func (m *MockConsulServiceClient) AllocRegistrations(allocID string) (*consul.AllocRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Logger.Printf("[TEST] mock_consul: AllocRegistrations(%q)", allocID)
	m.Ops = append(m.Ops, NewMockConsulOp("alloc_registrations", allocID, ""))

	if m.AllocRegistrationsFn != nil {
		return m.AllocRegistrationsFn(allocID)
	}

	return nil, nil
}
