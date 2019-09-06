package allocrunner

import (
	"sync"

	hclog "github.com/hashicorp/go-hclog"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/nomad/structs"
)

// groupServiceHook manages task group Consul service registration and
// deregistration.
type groupServiceHook struct {
	alloc        *structs.Allocation
	consulClient consul.ConsulServiceAPI
	prerun       bool
	mu           sync.Mutex

	logger log.Logger
}

func newGroupServiceHook(logger hclog.Logger, alloc *structs.Allocation, consulClient consul.ConsulServiceAPI) *groupServiceHook {
	h := &groupServiceHook{
		alloc:        alloc,
		consulClient: consulClient,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*groupServiceHook) Name() string {
	return "group_services"
}

func (h *groupServiceHook) Prerun() error {
	h.mu.Lock()
	defer func() {
		// Mark prerun as true to unblock Updates
		h.prerun = true
		h.mu.Unlock()
	}()
	return h.consulClient.RegisterGroup(h.alloc)
}

func (h *groupServiceHook) Update(req *interfaces.RunnerUpdateRequest) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	oldAlloc := h.alloc
	h.alloc = req.Alloc

	if !h.prerun {
		// Update called before Prerun. Update alloc and exit to allow
		// Prerun to do initial registration.
		return nil
	}

	return h.consulClient.UpdateGroup(oldAlloc, h.alloc)
}

func (h *groupServiceHook) Postrun() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.consulClient.RemoveGroup(h.alloc)
}
