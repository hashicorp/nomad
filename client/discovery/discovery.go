package discovery

import (
	"fmt"
	"log"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// builtins is a set of discovery layer implementations for various
// providers.
var builtins = []factory{
	newConsulDiscovery,
}

// factory is a function interface to create new discovery providers.
type factory func(ctx *context) (provider, error)

// provider is a generic interface which can be used to implement
// service discovery back-ends in Nomad.
type provider interface {
	// Name returns the name of the service discovery subsystem. This
	// is used to identify the system in log messages.
	Name() string

	// Enabled determines if the discovery layer has been enabled.
	Enabled() bool

	// Register is used to register a new entry into a service discovery
	// system. Only the name and port are required. The name is a symbolic
	// name used to refer to the service. The context can be used from
	// this function for internal node information such as IP address.
	Register(name string, port int) error

	// Deregister is used to deregister a service from a discovery system.
	Deregister(name string) error
}

// context is used to initialize a discovery backend.
type context struct {
	config *config.Config
	logger *log.Logger
	node   *structs.Node
}

// DiscoveryLayer wraps a set of discovery providers and provides
// easy calls to register/deregister into all of them.
type DiscoveryLayer struct {
	ctx       *context
	providers []provider
}

// NewDiscoveryLayer is used to initialize the discovery layer. It
// automatically initializes all of the built-in providers and
// checks their configuration.
func NewDiscoveryLayer(
	config *config.Config,
	logger *log.Logger,
	node *structs.Node) (*DiscoveryLayer, error) {

	// Make the context
	ctx := &context{
		config: config,
		logger: logger,
		node:   node,
	}

	// Create the DiscoveryLayer
	dl := &DiscoveryLayer{ctx: ctx}

	// Initialize the providers
	for _, factory := range builtins {
		provider, err := factory(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed initializing %s discovery: %s",
				provider.Name(), err)
		}
		if provider.Enabled() {
			dl.providers = append(dl.providers, provider)
		}
	}
	return dl, nil
}

// Providers returns the names of the enabled discovery providers.
func (d *DiscoveryLayer) Providers() []string {
	avail := make([]string, len(d.providers))
	for i, provider := range d.providers {
		avail[i] = provider.Name()
	}
	return avail
}

// Register iterates all of the providers and registers the given
// service information with them. If an error is encountered, it is
// only logged to prevent a single failure from crippling the entire
// discovery layer.
func (d *DiscoveryLayer) Register(name string, port int) {
	for _, disc := range d.providers {
		if err := disc.Register(name, port); err != nil {
			d.ctx.logger.Printf(
				"[ERR] client.discovery: error registering %q with %s: %s",
				name, disc.Name(), err)
			return
		}
		d.ctx.logger.Printf("[DEBUG] client.discovery: registered %q with %s",
			name, disc.Name())
	}
}

// Deregister is like Register, but removes the named service from
// the discovery subsystems.
func (d *DiscoveryLayer) Deregister(name string) {
	for _, disc := range d.providers {
		if err := disc.Deregister(name); err != nil {
			d.ctx.logger.Printf(
				"[ERR] client.discovery: error deregistering %q from %s: %s",
				name, disc.Name(), err)
			return
		}
		d.ctx.logger.Printf("[DEBUG] client.discovery: deregistered %q from %s",
			name, disc.Name())
	}
}
