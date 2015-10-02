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
	_, ok := c.ctx.config.Options["discovery.consul.enable"]
	return ok
}

func (c *ConsulDiscovery) Register(
	node, name, addr string,
	meta map[string]string,
	port int) error {

	// Build the service definition
	svc := &api.CatalogRegistration{
		Node:       node,
		Address:    addr,
		Datacenter: c.ctx.node.Datacenter,
		Service: &api.AgentService{
			ID:      name,
			Service: name,
			Port:    port,
			Address: addr,
		},
	}

	// Attempt to register
	_, err := c.client.Catalog().Register(svc, nil)
	return err
}

func (c *ConsulDiscovery) Deregister(node, name string) error {
	// Build the dereg request
	dereg := &api.CatalogDeregistration{
		Datacenter: c.ctx.node.Datacenter,
		Node:       node,
		ServiceID:  name,
	}

	// Send the deregister request
	_, err := c.client.Catalog().Deregister(dereg, nil)
	return err
}
