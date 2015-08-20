package client

import (
	"io"
	"log"
	"os"
	"sync"
)

// Config is used to parameterize and configure the behavior of the client
type Config struct {
	// LogOutput is the destination for logs
	LogOutput io.Writer
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		LogOutput: os.Stderr,
	}
}

// Client is used to implement the client interaction with Nomad. Clients
// are expected to register as a schedulable node to the servers, and to
// run allocations as determined by the servers.
type Client struct {
	logger *log.Logger

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
}

// NewClient is used to create a new client from the given configuration
func NewClient(config *Config) (*Client, error) {
	// Create a logger
	logger := log.New(config.LogOutput, "", log.LstdFlags)

	c := &Client{
		logger:     logger,
		shutdownCh: make(chan struct{}),
	}
	return c, nil
}

// Shutdown is used to tear down the client
func (c *Client) Shutdown() error {
	c.shutdownLock.Lock()
	defer c.shutdownLock.Unlock()

	if c.shutdown {
		return nil
	}
	c.shutdown = true
	close(c.shutdownCh)
	return nil
}
