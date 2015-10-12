package discovery

import (
	"fmt"
	"log"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Builtins is a set of discovery layer implementations for various
// providers. Each provider must implement the Provider interface.
var Builtins = []Factory{
	NewConsulDiscovery,
}

// Factory is a function interface to create new discovery providers.
type Factory func(Context) (Provider, error)

// Provider is a generic interface which can be used to implement
// service discovery back-ends in Nomad.
type Provider interface {
	// Name returns the type of the service discovery subsystem. This
	// is used to identify the system in log messages and usually
	// returns a static string value.
	Name() string

	// Enabled determines if the discovery layer has been enabled.
	Enabled() bool

	// DiscoverName returns the name used to register the task into service
	// discovery. The return value is used in log messages and is passed to
	// the Register and Deregister functions.
	DiscoverName(parts []string) string

	// Register is used to register a new entry into a service discovery
	// system. The name is a symbolic name used to refer to the service. The
	// context can be used from this function for internal node information
	// such as IP address. The allocID can be used to uniquely identify the
	// same task running multiple instances on the same node.
	Register(allocID, name string, port int) error

	// Deregister is used to deregister a service from a discovery system.
	Deregister(allocID, name string) error
}

// Context is used to initialize a discovery backend.
type Context struct {
	config *config.Config
	logger *log.Logger
	node   *structs.Node
}

// DiscoveryLayer wraps a set of discovery providers and provides
// easy calls to register/deregister into all of them.
type DiscoveryLayer struct {
	Providers []Provider
	Context
}

// NewDiscoveryLayer is used to initialize the discovery layer. It
// automatically initializes all of the built-in providers and
// checks their configuration.
func NewDiscoveryLayer(
	factories []Factory,
	config *config.Config,
	logger *log.Logger,
	node *structs.Node) (*DiscoveryLayer, error) {

	// Make the context
	ctx := Context{
		config: config,
		logger: logger,
		node:   node,
	}

	// Initialize the providers
	var providers []Provider
	for _, factory := range factories {
		provider, err := factory(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed initializing %s discovery: %s",
				provider.Name(), err)
		}
		if provider.Enabled() {
			providers = append(providers, provider)
		}
	}
	return &DiscoveryLayer{providers, ctx}, nil
}

// EnabledProviders returns the names of the enabled discovery providers.
func (d *DiscoveryLayer) EnabledProviders() []string {
	avail := make([]string, len(d.Providers))
	for i, provider := range d.Providers {
		avail[i] = provider.Name()
	}
	return avail
}

// Register iterates all of the providers and registers the given service
// information with them. If an error is encountered, it is only logged to
// prevent a single failure from crippling the entire discovery layer.
func (d *DiscoveryLayer) Register(allocID string, parts []string, port int) {
	for _, disc := range d.Providers {
		name := disc.DiscoverName(parts)
		if err := disc.Register(allocID, name, port); err != nil {
			d.logger.Printf(
				"[ERR] client.discovery: error registering %q with %s (alloc %s): %s",
				parts, disc.Name(), allocID, err)
			return
		}
		d.logger.Printf(
			"[DEBUG] client.discovery: registered %q with %s (alloc %s)",
			name, disc.Name(), allocID)
	}
}

// Deregister is like Register, but removes a service from the providers.
func (d *DiscoveryLayer) Deregister(allocID string, parts []string) {
	for _, disc := range d.Providers {
		name := disc.DiscoverName(parts)
		if err := disc.Deregister(allocID, name); err != nil {
			d.logger.Printf(
				"[ERR] client.discovery: error deregistering %q from %s (alloc %s): %s",
				name, disc.Name(), allocID, err)
			return
		}
		d.logger.Printf(
			"[DEBUG] client.discovery: deregistered %q from %s (alloc %s)",
			name, disc.Name(), allocID)
	}
}
