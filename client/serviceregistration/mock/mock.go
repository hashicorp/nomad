// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/serviceregistration"
)

// Ensure that the mock handler implements the service registration handler
// interface.
var _ serviceregistration.Handler = (*ServiceRegistrationHandler)(nil)

// ServiceRegistrationHandler is the mock implementation of the
// serviceregistration.Handler interface and can be used for testing.
type ServiceRegistrationHandler struct {
	log hclog.Logger

	// ops tracks the requested operations by the caller during the entire
	// lifecycle of the ServiceRegistrationHandler. The mutex should be used
	// whenever interacting with this.
	mu  sync.Mutex
	ops []Operation

	// AllocRegistrationsFn allows injecting return values for the
	// AllocRegistrations function.
	AllocRegistrationsFn func(allocID string) (*serviceregistration.AllocRegistration, error)
}

// NewServiceRegistrationHandler returns a ready to use
// ServiceRegistrationHandler for testing.
func NewServiceRegistrationHandler(log hclog.Logger) *ServiceRegistrationHandler {
	return &ServiceRegistrationHandler{
		ops: make([]Operation, 0, 20),
		log: log.Named("mock_service_registration"),
	}
}

func (h *ServiceRegistrationHandler) RegisterWorkload(services *serviceregistration.WorkloadServices) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.log.Trace("RegisterWorkload", "alloc_id", services.AllocInfo.AllocID,
		"name", services.Name(), "services", len(services.Services))

	h.ops = append(h.ops, newOperation("add", services.AllocInfo.AllocID, services.Name()))
	return nil
}

func (h *ServiceRegistrationHandler) RemoveWorkload(services *serviceregistration.WorkloadServices) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.log.Trace("RemoveWorkload", "alloc_id", services.AllocInfo.AllocID,
		"name", services.Name(), "services", len(services.Services))

	h.ops = append(h.ops, newOperation("remove", services.AllocInfo.AllocID, services.Name()))
}

func (h *ServiceRegistrationHandler) UpdateWorkload(old, newServices *serviceregistration.WorkloadServices) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.log.Trace("UpdateWorkload", "alloc_id", newServices.AllocInfo.AllocID, "name", newServices.Name(),
		"old_services", len(old.Services), "new_services", len(newServices.Services))

	h.ops = append(h.ops, newOperation("update", newServices.AllocInfo.AllocID, newServices.Name()))
	return nil
}

func (h *ServiceRegistrationHandler) AllocRegistrations(allocID string) (*serviceregistration.AllocRegistration, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.log.Trace("AllocRegistrations", "alloc_id", allocID)
	h.ops = append(h.ops, newOperation("alloc_registrations", allocID, ""))

	if h.AllocRegistrationsFn != nil {
		return h.AllocRegistrationsFn(allocID)
	}
	return nil, nil
}

func (h *ServiceRegistrationHandler) UpdateTTL(checkID, namespace, output, status string) error {
	// TODO(tgross): this method is here so we can implement the
	// interface but the locking we need for testing creates a lot
	// of opportunities for deadlocks in testing that will never
	// appear in live code.
	h.log.Trace("UpdateTTL", "check_id", checkID, "namespace", namespace, "status", status)
	return nil
}

// GetOps returns all stored operations within the handler.
func (h *ServiceRegistrationHandler) GetOps() []Operation {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.ops
}

// Operation represents the register/deregister operations.
type Operation struct {
	Op         string // add, remove, or update
	AllocID    string
	Name       string // task or group name
	OccurredAt time.Time
}

// newOperation generates a new Operation for the given parameters.
func newOperation(op, allocID, name string) Operation {
	switch op {
	case "add", "remove", "update", "alloc_registrations",
		"add_group", "remove_group", "update_group", "update_ttl":
	default:
		panic(fmt.Errorf("invalid consul op: %s", op))
	}
	return Operation{
		Op:         op,
		AllocID:    allocID,
		Name:       name,
		OccurredAt: time.Now(),
	}
}
