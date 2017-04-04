package consul

import (
	"fmt"

	"github.com/hashicorp/consul/api"
)

type consulOps struct {
	// services and checks to be registered
	regServices map[string]*api.AgentServiceRegistration
	regChecks   map[string]*api.AgentCheckRegistration

	// services and checks to be unregisterd
	deregServices map[string]struct{}
	deregChecks   map[string]struct{}

	// script checks to be run() after their corresponding check is
	// registered
	regScripts map[string]*scriptCheck
}

func newConsulOps() *consulOps {
	return &consulOps{
		regServices:   make(map[string]*api.AgentServiceRegistration),
		regChecks:     make(map[string]*api.AgentCheckRegistration),
		deregServices: make(map[string]struct{}),
		deregChecks:   make(map[string]struct{}),
		regScripts:    make(map[string]*scriptCheck),
	}
}

// merge newer operations. New operations registrations override existing
// deregistrations.
func (c *consulOps) merge(newer *consulOps) {
	for id, service := range newer.regServices {
		delete(c.deregServices, id)
		c.regServices[id] = service
	}
	for id, check := range newer.regChecks {
		delete(c.deregChecks, id)
		c.regChecks[id] = check
	}
	for id, script := range newer.regScripts {
		c.regScripts[id] = script
	}
	for id, _ := range newer.deregServices {
		delete(c.regServices, id)
		c.deregServices[id] = mark
	}
	for id, _ := range newer.deregChecks {
		delete(c.regChecks, id)
		delete(c.regScripts, id)
		c.deregChecks[id] = mark
	}
}

func (c *consulOps) String() string {
	return fmt.Sprintf("registered %d services / %d checks; deregisterd %d services / %d checks",
		len(c.regServices), len(c.regChecks), len(c.deregServices), len(c.deregChecks))
}
