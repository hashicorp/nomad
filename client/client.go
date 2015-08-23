package client

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// clientRPCCache controls how long we keep an idle connection
	// open to a server
	clientRPCCache = 30 * time.Second

	// clientMaxStreams controsl how many idle streams we keep
	// open to a server
	clientMaxStreams = 2

	// registerRetryIntv is minimum interval on which we retry
	// registration. We pick a value between this and 2x this.
	registerRetryIntv = 30 * time.Second

	// getAllocRetryIntv is minimum interval on which we retry
	// to fetch allocations. We pick a value between this and 2x this.
	getAllocRetryIntv = 30 * time.Second
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

	// Servers is a list of known server addresses. These are as "host:port"
	Servers []string

	// RPCHandler can be provided to avoid network traffic if the
	// server is running locally.
	RPCHandler RPCHandler

	// Node provides the base node
	Node *structs.Node
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		LogOutput: os.Stderr,
		Region:    "region1",
	}
}

// Client is used to implement the client interaction with Nomad. Clients
// are expected to register as a schedulable node to the servers, and to
// run allocations as determined by the servers.
type Client struct {
	config *Config

	logger *log.Logger

	lastServer     net.Addr
	lastRPCTime    time.Time
	lastServerLock sync.Mutex

	connPool *nomad.ConnPool

	lastHeartbeat time.Time
	heartbeatTTL  time.Duration

	// allocs is the current set of allocations
	allocs    map[string]*AllocRunner
	allocLock sync.RWMutex

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
}

// NewClient is used to create a new client from the given configuration
func NewClient(config *Config) (*Client, error) {
	// Create a logger
	logger := log.New(config.LogOutput, "", log.LstdFlags)

	// Create the client
	c := &Client{
		config:     config,
		connPool:   nomad.NewPool(config.LogOutput, clientRPCCache, clientMaxStreams, nil),
		logger:     logger,
		allocs:     make(map[string]*AllocRunner),
		shutdownCh: make(chan struct{}),
	}

	// Restore the state
	if err := c.restoreState(); err != nil {
		return nil, fmt.Errorf("failed to restore state: %v", err)
	}

	// Setup the node
	if err := c.setupNode(); err != nil {
		return nil, fmt.Errorf("node setup failed: %v", err)
	}

	// Fingerprint the node
	if err := c.fingerprint(); err != nil {
		return nil, fmt.Errorf("fingerprinting failed: %v", err)
	}

	// Scan for drivers
	if err := c.setupDrivers(); err != nil {
		return nil, fmt.Errorf("driver setup failed: %v", err)
	}

	// Start the client!
	go c.run()
	return c, nil
}

// Leave is used to prepare the client to leave the cluster
func (c *Client) Leave() error {
	// TODO
	return nil
}

// Shutdown is used to tear down the client
func (c *Client) Shutdown() error {
	c.logger.Printf("[INFO] client: shutting down")
	c.shutdownLock.Lock()
	defer c.shutdownLock.Unlock()

	if c.shutdown {
		return nil
	}
	c.shutdown = true
	close(c.shutdownCh)
	c.connPool.Shutdown()
	return nil
}

// RPC is used to forward an RPC call to a nomad server, or fail if no servers
func (c *Client) RPC(method string, args interface{}, reply interface{}) error {
	// Invoke the RPCHandle if it exists
	if c.config.RPCHandler != nil {
		return c.config.RPCHandler.RPC(method, args, reply)
	}

	// Pick a server to request from
	addr, err := c.pickServer()
	if err != nil {
		return err
	}

	// Make the RPC request
	err = c.connPool.RPC(c.config.Region, addr, 1, method, args, reply)

	// Update the last server information
	c.lastServerLock.Lock()
	if err != nil {
		c.lastServer = nil
		c.lastRPCTime = time.Time{}
	} else {
		c.lastServer = addr
		c.lastRPCTime = time.Now()
	}
	c.lastServerLock.Unlock()
	return err
}

// pickServer is used to pick a target RPC server
func (c *Client) pickServer() (net.Addr, error) {
	c.lastServerLock.Lock()
	defer c.lastServerLock.Unlock()

	// Check for a valid last-used server
	if c.lastServer != nil && time.Now().Sub(c.lastRPCTime) < clientRPCCache {
		return c.lastServer, nil
	}

	// Bail if we can't find any servers
	if len(c.config.Servers) == 0 {
		return nil, fmt.Errorf("no known servers")
	}

	// Copy the list of servers and shuffle
	servers := make([]string, len(c.config.Servers))
	copy(servers, c.config.Servers)
	shuffleStrings(servers)

	// Try to resolve each server
	for i := 0; i < len(servers); i++ {
		addr, err := net.ResolveTCPAddr("tcp", servers[i])
		if err == nil {
			c.lastServer = addr
			c.lastRPCTime = time.Now()
			return addr, nil
		}
		c.logger.Printf("[WARN] client: failed to resolve '%s': %v", err)
	}

	// Bail if we reach this point
	return nil, fmt.Errorf("failed to resolve any servers")
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
			"known_servers": toString(uint64(len(c.config.Servers))),
		},
		"runtime": nomad.RuntimeStats(),
	}
	return stats
}

// Node returns the locally registered node
func (c *Client) Node() *structs.Node {
	return c.config.Node
}

// restoreState is used to restore our state from the data dir
func (c *Client) restoreState() error {
	if c.config.DevMode {
		return nil
	}

	// TODO
	return nil
}

// saveState is used to snapshot our state into the data dir
func (c *Client) saveState() error {
	if c.config.DevMode {
		return nil
	}

	// TODO
	return nil
}

// setupNode is used to setup the initial node
func (c *Client) setupNode() error {
	node := c.config.Node
	if node == nil {
		node = &structs.Node{}
		c.config.Node = node
	}
	if node.Attributes == nil {
		node.Attributes = make(map[string]string)
	}
	if node.Links == nil {
		node.Links = make(map[string]string)
	}
	if node.Meta == nil {
		node.Meta = make(map[string]string)
	}
	if node.Resources == nil {
		node.Resources = &structs.Resources{}
	}
	if node.ID == "" {
		node.ID = generateUUID()
	}
	if node.Datacenter == "" {
		node.Datacenter = "dc1"
	}
	if node.Name == "" {
		node.Name, _ = os.Hostname()
	}
	if node.Name == "" {
		node.Name = node.ID
	}
	node.Status = structs.NodeStatusInit
	return nil
}

// fingerprint is used to fingerprint the client and setup the node
func (c *Client) fingerprint() error {
	var applied []string
	for name := range fingerprint.BuiltinFingerprints {
		f, err := fingerprint.NewFingerprint(name, c.logger)
		if err != nil {
			return err
		}
		applies, err := f.Fingerprint(c.config.Node)
		if err != nil {
			return err
		}
		if applies {
			applied = append(applied, name)
		}
	}
	c.logger.Printf("[DEBUG] client: applied fingerprints %v", applied)
	return nil
}

// setupDrivers is used to find the available drivers
func (c *Client) setupDrivers() error {
	var avail []string
	for name := range driver.BuiltinDrivers {
		d, err := driver.NewDriver(name, c.logger)
		if err != nil {
			return err
		}
		applies, err := d.Fingerprint(c.config.Node)
		if err != nil {
			return err
		}
		if applies {
			avail = append(avail, name)
		}
	}
	c.logger.Printf("[DEBUG] client: available drivers %v", avail)
	return nil
}

// run is a long lived goroutine used to run the client
func (c *Client) run() {
	// Register the client
	for {
		if err := c.registerNode(); err == nil {
			break
		}
		select {
		case <-time.After(registerRetryIntv + randomStagger(registerRetryIntv)):
		case <-c.shutdownCh:
			return
		}
	}

	// Setup the heartbeat timer
	heartbeat := time.After(c.heartbeatTTL)

	// Watch for changes in allocations
	allocUpdates := make(chan []*structs.Allocation, 1)
	go c.watchAllocations(allocUpdates)

	// Periodically update our status and wait for termination
	select {
	case allocs := <-allocUpdates:
		c.runAllocs(allocs)

	case <-heartbeat:
		if err := c.updateNodeStatus(); err != nil {
			heartbeat = time.After(registerRetryIntv + randomStagger(registerRetryIntv))
		} else {
			heartbeat = time.After(c.heartbeatTTL)
		}

	case <-c.shutdownCh:
		return
	}
}

// registerNode is used to register the node or update the registration
func (c *Client) registerNode() error {
	node := c.Node()
	req := structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: c.config.Region},
	}
	var resp structs.NodeUpdateResponse
	err := c.RPC("Client.Register", &req, &resp)
	if err != nil {
		c.logger.Printf("[ERR] client: failed to register node: %v", err)
		return err
	}
	c.logger.Printf("[DEBUG] client: node registration complete")
	if len(resp.EvalIDs) != 0 {
		c.logger.Printf("[DEBUG] client: %d evaluations triggered by node registration", len(resp.EvalIDs))
	}
	c.lastHeartbeat = time.Now()
	c.heartbeatTTL = resp.HeartbeatTTL
	return nil
}

// updateNodeStatus is used to heartbeat and update the status of the node
func (c *Client) updateNodeStatus() error {
	node := c.Node()
	req := structs.NodeUpdateStatusRequest{
		NodeID:       node.ID,
		Status:       structs.NodeStatusReady,
		WriteRequest: structs.WriteRequest{Region: c.config.Region},
	}
	var resp structs.NodeUpdateResponse
	err := c.RPC("Client.UpdateStatus", &req, &resp)
	if err != nil {
		c.logger.Printf("[ERR] client: failed to update status: %v", err)
		return err
	}
	if len(resp.EvalIDs) != 0 {
		c.logger.Printf("[DEBUG] client: %d evaluations triggered by node update", len(resp.EvalIDs))
	}
	if resp.Index != 0 {
		c.logger.Printf("[DEBUG] client: client state updated")
	}
	c.lastHeartbeat = time.Now()
	c.heartbeatTTL = resp.HeartbeatTTL
	return nil
}

// watchAllocations is used to scan for updates to allocations
func (c *Client) watchAllocations(allocUpdates chan []*structs.Allocation) {
	var lastIndex uint64
	req := structs.NodeSpecificRequest{
		NodeID: c.Node().ID,
		QueryOptions: structs.QueryOptions{
			Region:        c.config.Region,
			MinQueryIndex: lastIndex,
			AllowStale:    true,
		},
	}
	var resp structs.NodeAllocsResponse

	for {
		// Get the allocations, blocking for updates
		err := c.RPC("Client.GetAllocs", &req, &resp)
		if err != nil {
			c.logger.Printf("[ERR] client: failed to query for node allocations: %v", err)
			retry := getAllocRetryIntv + randomStagger(getAllocRetryIntv)
			select {
			case <-time.After(retry):
				continue
			case <-c.shutdownCh:
				return
			}
		}

		// Check for shutdown
		select {
		case <-c.shutdownCh:
			return
		default:
		}

		// Check for updates
		if resp.Index == lastIndex {
			continue
		}
		lastIndex = resp.Index
		c.logger.Printf("[DEBUG] client: updated allocations at index %d (%d allocs)", lastIndex, len(resp.Allocs))

		// Push the updates
		select {
		case allocUpdates <- resp.Allocs:
		case <-c.shutdownCh:
			return
		}
	}
}

// runAllocs is invoked when we get an updated set of allocations
func (c *Client) runAllocs(updated []*structs.Allocation) {
	// Get the existing allocs
	c.allocLock.RLock()
	exist := make([]*structs.Allocation, len(c.allocs))
	for _, ctx := range c.allocs {
		exist = append(exist, ctx.Alloc())
	}
	c.allocLock.RUnlock()

	// Diff the existing and updated allocations
	diff := diffAllocs(exist, updated)
	c.logger.Printf("[DEBUG] client: %#v", diff)

	// Remove the old allocations
	for _, remove := range diff.removed {
		if err := c.removeAlloc(remove); err != nil {
			c.logger.Printf("[ERR] client: failed to remove alloc '%s': %v",
				remove.ID, err)
		}
	}

	// Update the existing allocations
	for _, update := range diff.updated {
		if err := c.updateAlloc(update.exist, update.updated); err != nil {
			c.logger.Printf("[ERR] client: failed to update alloc '%s': %v",
				update.exist.ID, err)
		}
	}

	// Start the new allocations
	for _, add := range diff.added {
		if err := c.addAlloc(add); err != nil {
			c.logger.Printf("[ERR] client: failed to add alloc '%s': %v",
				add.ID, err)
		}
	}

	// Persist our state
	if err := c.saveState(); err != nil {
		c.logger.Printf("[ERR] client: failed to save state: %v", err)
	}
}

// removeAlloc is invoked when we should remove an allocation
func (c *Client) removeAlloc(alloc *structs.Allocation) error {
	c.allocLock.RLock()
	defer c.allocLock.RUnlock()
	ctx, ok := c.allocs[alloc.ID]
	if !ok {
		c.logger.Printf("[WARN] client: missing context for alloc '%s'", alloc.ID)
		return nil
	}
	ctx.Destroy()
	return nil
}

// updateAlloc is invoked when we should update an allocation
func (c *Client) updateAlloc(exist, update *structs.Allocation) error {
	c.allocLock.RLock()
	defer c.allocLock.RUnlock()
	ctx, ok := c.allocs[exist.ID]
	if !ok {
		c.logger.Printf("[WARN] client: missing context for alloc '%s'", exist.ID)
		return nil
	}
	ctx.Update(update)
	return nil
}

// addAlloc is invoked when we should add an allocation
func (c *Client) addAlloc(alloc *structs.Allocation) error {
	c.allocLock.Lock()
	defer c.allocLock.Unlock()
	ctx := NewAllocRunner(c, alloc)
	c.allocs[alloc.ID] = ctx
	go ctx.Run()
	return nil
}
