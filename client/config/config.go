package config

import (
	"io"

	"github.com/hashicorp/nomad/nomad/structs"
)

// RPCHandler can be provided to the Client if there is a local server
// to avoid going over the network. If not provided, the Client will
// maintain a connection pool to the servers
type RPCHandler interface {
	RPC(method string, args interface{}, reply interface{}) error
}

// Config is used to parameterize and configure the behavior of the client
type Config struct {
	// DevMode controls if we are in a development mode which
	// avoids persistent storage.
	DevMode bool

	// StateDir is where we store our state
	StateDir string

	// AllocDir is where we store data for allocations
	AllocDir string

	// LogOutput is the destination for logs
	LogOutput io.Writer

	// Region is the clients region
	Region string

	// Network interface to be used in network fingerprinting
	Iface string

	// Servers is a list of known server addresses. These are as "host:port"
	Servers []string

	// RPCHandler can be provided to avoid network traffic if the
	// server is running locally.
	RPCHandler RPCHandler

	// Node provides the base node
	Node *structs.Node

	// Options provides arbitrary key-value configuration for nomad internals,
	// like fingerprinters and drivers. The format is:
	//
	//	namespace.option = value
	Options map[string]string
}

// Read returns the specified configuration value or "".
func (c *Config) Read(id string) string {
	val, ok := c.Options[id]
	if !ok {
		return ""
	}
	return val
}

// ReadDefault returns the specified configuration value, or the specified
// default value if none is set.
func (c *Config) ReadDefault(id string, defaultValue string) string {
	val := c.Read(id)
	if val != "" {
		return val
	}
	return defaultValue
}
