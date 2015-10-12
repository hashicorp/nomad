package discovery

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
)

// ConsulDiscovery is a back-end for service discovery which can be used
// to populate a local Consul agent with service information. Because
// Consul already has information about the local node, some shortcuts
// can be taken in this back-end. Specifically, the IP address of the Nomad
// agent does not need to be used, because Consul has this information
// already and may even be configured to expose services on an alternate
// advertise address.
type ConsulDiscovery struct {
	client *api.Client
	Context
}

// NewConsulDiscovery creates a new Consul discovery provider using the
// configuration provided in the client options.
func NewConsulDiscovery(ctx Context) (Provider, error) {
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
	return &ConsulDiscovery{client, ctx}, nil
}

// Name returns the name of the discovery provider.
func (c *ConsulDiscovery) Name() string {
	return "consul"
}

// Enabled determines if the Consul layer is enabled. This just looks at a
// client option and doesn't do any connection testing, as Consul may or may
// not be available at the time of Nomad's start.
func (c *ConsulDiscovery) Enabled() bool {
	return c.config.Read("discovery.consul.enable") == "true"
}

// Register registers a service name into a Consul agent. The agent will then
// sync this definition into the service catalog.
func (c *ConsulDiscovery) Register(allocID, name string, port int) error {
	// Build the service definition
	serviceID := fmt.Sprintf("%s:%s", name, allocID)
	svc := &api.AgentServiceRegistration{
		ID:   serviceID,
		Name: name,
		Port: port,
	}

	// Attempt to register
	return c.client.Agent().ServiceRegister(svc)
}

// Deregister removes a service from the Consul agent. Anti-entropy will
// then handle deregistering the service from the catalog.
func (c *ConsulDiscovery) Deregister(allocID, name string) error {
	// Send the deregister request
	serviceID := fmt.Sprintf("%s:%s", name, allocID)
	return c.client.Agent().ServiceDeregister(serviceID)
}

// DiscoverName returns the service name in Consul, given the parts of the
// name. This is a simple hyphen-joined string so that we can easily support
// DNS lookups from Consul.
func (c *ConsulDiscovery) DiscoverName(parts []string) string {
	return strings.Join(parts, "-")
}
