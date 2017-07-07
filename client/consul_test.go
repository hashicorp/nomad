package client

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/client/driver"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
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
	if op != "add" && op != "remove" && op != "update" && op != "checks" {
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

	// checksFn allows injecting return values for the Checks function.
	checksFn func(*structs.Allocation) ([]*api.AgentCheck, error)
}

func newMockConsulServiceClient() *mockConsulServiceClient {
	m := mockConsulServiceClient{
		ops:    make([]mockConsulOp, 0, 20),
		logger: log.New(ioutil.Discard, "", 0),
	}
	if testing.Verbose() {
		m.logger = log.New(os.Stderr, "", log.LstdFlags)
	}
	return &m
}

func (m *mockConsulServiceClient) UpdateTask(allocID string, old, new *structs.Task, exec driver.ScriptExecutor, net *cstructs.DriverNetwork) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Printf("[TEST] mock_consul: UpdateTask(%q, %v, %v, %T, %x)", allocID, old, new, exec, net.Hash())
	m.ops = append(m.ops, newMockConsulOp("update", allocID, new, exec, net))
	return nil
}

func (m *mockConsulServiceClient) RegisterTask(allocID string, task *structs.Task, exec driver.ScriptExecutor, net *cstructs.DriverNetwork) error {
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

func (m *mockConsulServiceClient) Checks(alloc *structs.Allocation) ([]*api.AgentCheck, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Printf("[TEST] mock_consul: Checks(%q)", alloc.ID)
	m.ops = append(m.ops, newMockConsulOp("checks", alloc.ID, nil, nil, nil))

	if m.checksFn != nil {
		return m.checksFn(alloc)
	}

	return nil, nil
}
