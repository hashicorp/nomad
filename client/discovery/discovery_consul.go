package discovery

import (
	"github.com/hashicorp/consul/api"
)

type ConsulDiscovery struct {
	client *api.Client
}

func NewConsulDiscovery() (*ConsulDiscovery, error) {
	// Build the config
	conf := api.DefaultConfig()

	// Create the client
	client, err := api.NewClient(conf)
	if err != nil {
		return nil, err
	}

	// Create and return the discovery provider
	return &ConsulDiscovery{client}, nil
}

func (c *ConsulDiscovery) Register(
	node, name, addr string,
	meta map[string]string,
	port int) error {

	// Build the service definition
	svc := &api.CatalogRegistration{
		Node:       node,
		Address:    addr,
		Datacenter: c.config.Datacenter,
		Service: &api.AgentService{
			ID:      name,
			Service: name,
			Tags:    meta,
			Port:    port,
			Address: addr,
		},
	}

	// Attempt to register
	return c.client.Catalog().Register(svc)
}

func (c *ConsulDiscovery) Deregister(node, addr, name string) error {
	// Build the dereg request
	dereg := &api.CatalogDeregistration{
		Datacenter: c.config.Datacenter,
		Node:       node,
		ServiceID:  name,
	}

	// Send the deregister request
	_, err := c.client.Catalog().Deregister(dereg)
	return err
}
