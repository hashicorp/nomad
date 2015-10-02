package discovery

import (
	"github.com/hashicorp/consul/api"
)

type ConsulDiscovery struct {
	ctx    *Context
	client *api.Client
}

func NewConsulDiscovery(ctx *Context) (Discovery, error) {
	// Build the config
	conf := api.DefaultConfig()
	conf.Datacenter = ctx.node.Datacenter
	if addr, ok := ctx.config.Options["discovery.consul.address"]; ok {
		conf.Address = addr
	}
	if scheme, ok := ctx.config.Options["discovery.consul.scheme"]; ok {
		conf.Scheme = scheme
	}
	if token, ok := ctx.config.Options["discovery.consul.token"]; ok {
		conf.Token = token
	}

	// Create the client
	client, err := api.NewClient(conf)
	if err != nil {
		return nil, err
	}

	// Create and return the discovery provider
	return &ConsulDiscovery{ctx, client}, nil
}

func (c *ConsulDiscovery) Enabled() bool {
	return true
	_, ok := c.ctx.config.Options["discovery.consul.enable"]
	return ok
}

func (c *ConsulDiscovery) Register(name string, port int) error {
	// Build the service definition
	svc := &api.AgentServiceRegistration{
		ID:   name,
		Name: name,
		Port: port,
	}

	// Attempt to register
	return c.client.Agent().ServiceRegister(svc)
}

func (c *ConsulDiscovery) Deregister(name string) error {
	// Send the deregister request
	return c.client.Agent().ServiceDeregister(name)
}
