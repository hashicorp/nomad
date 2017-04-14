package client

import (
	"io/ioutil"
	"log"
	"os"
	"sync"
	"testing"

	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/nomad/structs"
)

// mockConsulOp represents the register/deregister operations.
type mockConsulOp struct {
	allocID string
	task    *structs.Task
	exec    driver.ScriptExecutor
}

// mockConsulServiceClient implements the ConsulServiceAPI interface to record
// and log task registration/deregistration.
type mockConsulServiceClient struct {
	registers []mockConsulOp
	removes   []mockConsulOp
	mu        sync.Mutex

	logger *log.Logger
}

func newMockConsulServiceClient() *mockConsulServiceClient {
	m := mockConsulServiceClient{
		registers: make([]mockConsulOp, 0, 10),
		removes:   make([]mockConsulOp, 0, 10),
		logger:    log.New(ioutil.Discard, "", 0),
	}
	if testing.Verbose() {
		m.logger = log.New(os.Stderr, "", log.LstdFlags)
	}
	return &m
}

func (m *mockConsulServiceClient) UpdateTask(allocID string, old, new *structs.Task, exec driver.ScriptExecutor) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Printf("[TEST] mock_consul: UpdateTask(%q, %q, %q, %T)", allocID, old, new, exec)
	m.removes = append(m.removes, mockConsulOp{allocID, old, exec})
	m.registers = append(m.registers, mockConsulOp{allocID, new, exec})
	return nil
}

func (m *mockConsulServiceClient) RegisterTask(allocID string, task *structs.Task, exec driver.ScriptExecutor) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Printf("[TEST] mock_consul: RegisterTask(%q, %q, %T)", allocID, task.Name, exec)
	m.registers = append(m.registers, mockConsulOp{allocID, task, exec})
	return nil
}

func (m *mockConsulServiceClient) RemoveTask(allocID string, task *structs.Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger.Printf("[TEST] mock_consul: RemoveTask(%q, %q)", allocID, task.Name)
	m.removes = append(m.removes, mockConsulOp{allocID, task, nil})
}
