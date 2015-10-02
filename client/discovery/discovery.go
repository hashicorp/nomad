package discovery

import (
	"log"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Builtins is a set of discovery layer implementations for various
// providers.
var Builtins = map[string]Factory{
	"consul": NewConsulDiscovery,
}

// Factory is a function interface which creates new discovery layers.
type Factory func(ctx *Context) (Discovery, error)

// Discovery provides a generic interface which can be used to implement
// service discovery back-ends in Nomad.
type Discovery interface {
	// Enabled determines if the discovery layer has been enabled.
	Enabled() bool

	// Register is used to register a new entry into a service discovery
	// system. The address and port are the location of the service, and
	// the name is the symbolic name used to query it. The node is the
	// unique node ID known to Nomad.
	Register(name string, port int) error

	// Deregister is used to deregister a service from a discovery system.
	Deregister(name string) error
}

// Context is used to initialize a discovery backend.
type Context struct {
	config *config.Config
	logger *log.Logger
	node   *structs.Node
}

// NewContext is used to create a new DiscoveryContext based
// on the current settings of the client.
func NewContext(
	config *config.Config,
	logger *log.Logger,
	node *structs.Node) *Context {

	return &Context{
		config: config,
		logger: logger,
		node:   node,
	}
}
