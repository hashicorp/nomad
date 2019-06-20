package allocrunner

import (
	hclog "github.com/hashicorp/go-hclog"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/nomad/structs"
)

type groupServiceHook struct {
	alloc        *structs.Allocation
	consulClient consul.ConsulServiceAPI

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

func (groupServiceHook) Name() string {
	return "group_services"
}

// pre-run hook "group_services" failed: unable to get address for service "redis-cache": invalid port "db": port label not found
func (h *groupServiceHook) Prerun() error {
	return h.consulClient.RegisterAlloc(h.alloc)
}
