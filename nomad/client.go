package nomad

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	// clientRPCCache controls how long we keep an idle connection
	// open to a server
	clientRPCCache = 30 * time.Second

	// clientMaxStreams controsl how many idle streams we keep
	// open to a server
	clientMaxStreams = 2
)

// Interface is used to provide either a Client or Server,
// both of which can be used to perform certain common methods
type Interface interface {
	RPC(method string, args interface{}, reply interface{}) error
}

// Client is used to interact with a Nomad server. The client is
// effectively stateless, and just forwards requests.
type Client struct {
	config   *Config
	connPool *ConnPool

	// lastServer is the last server we made an RPC call to,
	// this is used to re-use the last connection
	lastServer  net.Addr
	lastRPCTime time.Time

	logger *log.Logger

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
}

// NewClient is used to construct a new Nomad client from the
// configuration, potentially returning an error. The client
// is only used to forward requests to the servers.
func NewClient(config *Config) (*Client, error) {
	// Check the protocol version
	if err := config.CheckVersion(); err != nil {
		return nil, err
	}

	// Ensure we have a log output
	if config.LogOutput == nil {
		config.LogOutput = os.Stderr
	}

	// Create a logger
	logger := log.New(config.LogOutput, "", log.LstdFlags)

	// Create the client
	c := &Client{
		config:     config,
		connPool:   NewPool(config.LogOutput, clientRPCCache, clientMaxStreams, nil),
		logger:     logger,
		shutdownCh: make(chan struct{}),
	}
	return c, nil
}

// Shutdown is used to shutdown the client
func (c *Client) Shutdown() error {
	c.logger.Printf("[INFO] nomad: shutting down client")
	c.shutdownLock.Lock()
	defer c.shutdownLock.Unlock()

	if c.shutdown {
		return nil
	}

	c.shutdown = true
	close(c.shutdownCh)

	// Close the connection pool
	c.connPool.Shutdown()
	return nil
}

// RPC is used to forward an RPC call to a nomad server, or fail if no servers
func (c *Client) RPC(method string, args interface{}, reply interface{}) error {
	// Check the last rpc time
	var addr net.Addr
	var servers []string
	var err error
	if time.Now().Sub(c.lastRPCTime) < clientRPCCache {
		addr = c.lastServer
		if addr != nil {
			goto TRY_RPC
		}
	}

	// Bail if we can't find any servers
	if len(c.config.ServerAddress) == 0 {
		return fmt.Errorf("no known servers")
	}

	// Copy the list of servers and shuffle
	servers = make([]string, len(c.config.ServerAddress))
	copy(servers, c.config.ServerAddress)
	shuffleStrings(servers)

	// Try to resolve each server
	for i := 0; i < len(servers); i++ {
		addr, err = net.ResolveTCPAddr("tcp", servers[i])
		if err != nil {
			return fmt.Errorf("failed to resolve '%s': %v", err)
		}
	}

	// Forward to remote Nomad
TRY_RPC:
	if err := c.connPool.RPC(c.config.Region, addr, 1, method, args, reply); err != nil {
		c.lastServer = nil
		c.lastRPCTime = time.Time{}
		return err
	}

	// Cache the last server
	c.lastServer = addr
	c.lastRPCTime = time.Now()
	return nil
}

// Stats is used to return statistics for debugging and insight
// for various sub-systems
func (c *Client) Stats() map[string]map[string]string {
	toString := func(v uint64) string {
		return strconv.FormatUint(v, 10)
	}
	stats := map[string]map[string]string{
		"nomad": map[string]string{
			"server":        "false",
			"known_servers": toString(uint64(len(c.config.ServerAddress))),
		},
		"runtime": RuntimeStats(),
	}
	return stats
}
