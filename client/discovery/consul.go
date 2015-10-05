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

// newConsulDiscovery creates a new Consul discovery provider using the
// configuration provided in the client options.
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

// Name returns the name of the discovery provider.
func (c *consulDiscovery) Name() string {
	return "consul"
}

// Enabled determines if the Consul layer is enabled. This just looks at a
// client option and doesn't do any connection testing, as Consul may or may
// not be available at the time of Nomad's start.
func (c *consulDiscovery) Enabled() bool {
	return c.ctx.config.Read("discovery.consul.enable") == "true"
}

// Register registers a service name into a Consul agent. The agent will then
// sync this definition into the service catalog.
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

// Deregister removes a service from the Consul agent. Anti-entropy will
// then handle deregistering the service from the catalog.
func (c *consulDiscovery) Deregister(name string) error {
	// Send the deregister request
	return c.client.Agent().ServiceDeregister(name)
}

// DiscoverName returns the service name in Consul, given the parts of the
// name. This is a simple hyphen-joined string so that we can easily support
// DNS lookups from Consul.
func (c *consulDiscovery) DiscoverName(parts []string) string {
	return strings.Join(parts, "-")
}
