package client

import (
	"fmt"
	"log"
	"sync"

	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/mitchellh/go-testing-interface"
)

// mockConsulOp represents the register/deregister operations.
type mockConsulOp struct {
	op      string // add, remove, or update
	allocID string
	task    string
}

func newMockConsulOp(op, allocID, task string) mockConsulOp {
	if op != "add" && op != "remove" && op != "update" && op != "alloc_registrations" {
		panic(fmt.Errorf("invalid consul op: %s", op))
	}
	return mockConsulOp{
		op:      op,
		allocID: allocID,
		task:    task,
	}
}

// mockConsulServiceClient implements the ConsulServiceAPI interface to record
// and log task registration/deregistration.
type mockConsulServiceClient struct {
	ops []mockConsulOp
	mu  sync.Mutex

	logger *log.Logger

	// allocRegistrationsFn allows injecting return values for the
	// AllocRegistrations function.
	allocRegistrationsFn func(allocID string) (*consul.AllocRegistration, error)
}

func newMockConsulServiceClient(t testing.T) *mockConsulServiceClient {
	m := mockConsulServiceClient{
		ops:    make([]mockConsulOp, 0, 20),
		logger: testlog.Logger(t),
	}
	return &m
}

func (m *mockConsulServiceClient) UpdateTask(old, new *consul.TaskServices) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Printf("[TEST] mock_consul: UpdateTask(alloc: %s, task: %s)", new.AllocID[:6], new.Name)
	m.ops = append(m.ops, newMockConsulOp("update", new.AllocID, new.Name))
	return nil
}

func (m *mockConsulServiceClient) RegisterTask(task *consul.TaskServices) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Printf("[TEST] mock_consul: RegisterTask(alloc: %s, task: %s)", task.AllocID, task.Name)
	m.ops = append(m.ops, newMockConsulOp("add", task.AllocID, task.Name))
	return nil
}

func (m *mockConsulServiceClient) RemoveTask(task *consul.TaskServices) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Printf("[TEST] mock_consul: RemoveTask(%q, %q)", task.AllocID, task.Name)
	m.ops = append(m.ops, newMockConsulOp("remove", task.AllocID, task.Name))
}

func (m *mockConsulServiceClient) AllocRegistrations(allocID string) (*consul.AllocRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Printf("[TEST] mock_consul: AllocRegistrations(%q)", allocID)
	m.ops = append(m.ops, newMockConsulOp("alloc_registrations", allocID, ""))

	if m.allocRegistrationsFn != nil {
		return m.allocRegistrationsFn(allocID)
	}

	return nil, nil
}
