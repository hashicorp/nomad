package client

import (
	"fmt"
	"log"
	"sync"

	"github.com/hashicorp/nomad/client/driver"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/go-testing-interface"
)

// mockConsulOp represents the register/deregister operations.
type mockConsulOp struct {
	op      string // add, remove, or update
	allocID string
	task    *structs.Task
	exec    driver.ScriptExecutor
	net     *cstructs.DriverNetwork
}

func newMockConsulOp(op, allocID string, task *structs.Task, exec driver.ScriptExecutor, net *cstructs.DriverNetwork) mockConsulOp {
	if op != "add" && op != "remove" && op != "update" && op != "alloc_registrations" {
		panic(fmt.Errorf("invalid consul op: %s", op))
	}
	return mockConsulOp{
		op:      op,
		allocID: allocID,
		task:    task,
		exec:    exec,
		net:     net,
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

func (m *mockConsulServiceClient) UpdateTask(allocID string, old, new *structs.Task, restarter consul.TaskRestarter, exec driver.ScriptExecutor, net *cstructs.DriverNetwork) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Printf("[TEST] mock_consul: UpdateTask(%q, %v, %v, %T, %x)", allocID, old, new, exec, net.Hash())
	m.ops = append(m.ops, newMockConsulOp("update", allocID, new, exec, net))
	return nil
}

func (m *mockConsulServiceClient) RegisterTask(allocID string, task *structs.Task, restarter consul.TaskRestarter, exec driver.ScriptExecutor, net *cstructs.DriverNetwork) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Printf("[TEST] mock_consul: RegisterTask(%q, %q, %T, %x)", allocID, task.Name, exec, net.Hash())
	m.ops = append(m.ops, newMockConsulOp("add", allocID, task, exec, net))
	return nil
}

func (m *mockConsulServiceClient) RemoveTask(allocID string, task *structs.Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Printf("[TEST] mock_consul: RemoveTask(%q, %q)", allocID, task.Name)
	m.ops = append(m.ops, newMockConsulOp("remove", allocID, task, nil, nil))
}

func (m *mockConsulServiceClient) AllocRegistrations(allocID string) (*consul.AllocRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Printf("[TEST] mock_consul: AllocRegistrations(%q)", allocID)
	m.ops = append(m.ops, newMockConsulOp("alloc_registrations", allocID, nil, nil, nil))

	if m.allocRegistrationsFn != nil {
		return m.allocRegistrationsFn(allocID)
	}

	return nil, nil
}
