package discovery

import (
	"strings"

	"github.com/hashicorp/consul/api"
)

// consulDiscovery is a back-end for service discovery which can be used
// to populate a local Consul agent with service information. Because
// Consul already has information about the local node, some shortcuts
// can be taken in this back-end. Specifically, the IP address of the Nomad
// agent does not need to be used, because Consul has this information
// already and may even be configured to expose services on an alternate
// advertise address.
type consulDiscovery struct {
	ctx    *context
	client *api.Client
}

func newConsulDiscovery(ctx *context) (provider, error) {
	// Build the config
	conf := api.DefaultConfig()
	conf.Datacenter = ctx.node.Datacenter
	conf.Address = ctx.config.Read("discovery.consul.address")
	conf.Scheme = ctx.config.Read("discovery.consul.scheme")
	conf.Token = ctx.config.Read("discovery.consul.token")

	// Create the client
	client, err := api.NewClient(conf)
	if err != nil {
		return nil, err
	}

	// Create and return the discovery provider
	return &consulDiscovery{ctx, client}, nil
}

func (c *consulDiscovery) Name() string {
	return "consul"
}

func (c *consulDiscovery) Enabled() bool {
	return c.ctx.config.Read("discovery.consul.enable") == "true"
}

func (c *consulDiscovery) Register(name string, port int) error {
	// Build the service definition
	svc := &api.AgentServiceRegistration{
		ID:   name,
		Name: name,
		Port: port,
	}

	// Attempt to register
	return c.client.Agent().ServiceRegister(svc)
}

func (c *consulDiscovery) Deregister(name string) error {
	// Send the deregister request
	return c.client.Agent().ServiceDeregister(name)
}

func (c *consulDiscovery) DiscoverName(parts []string) string {
	return strings.Join(parts, "-")
}
