package client

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/discovery"
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
	registerRetryIntv = 15 * time.Second

	// getAllocRetryIntv is minimum interval on which we retry
	// to fetch allocations. We pick a value between this and 2x this.
	getAllocRetryIntv = 30 * time.Second

	// devModeRetryIntv is the retry interval used for development
	devModeRetryIntv = time.Second

	// stateSnapshotIntv is how often the client snapshots state
	stateSnapshotIntv = 60 * time.Second

	// registerErrGrace is the grace period where we don't log about
	// register errors after start. This is to improve the user experience
	// in dev mode where the leader isn't elected for a few seconds.
	registerErrGrace = 10 * time.Second

	// initialHeartbeatStagger is used to stagger the interval between
	// starting and the intial heartbeat. After the intial heartbeat,
	// we switch to using the TTL specified by the servers.
	initialHeartbeatStagger = 10 * time.Second
)

// DefaultConfig returns the default configuration
func DefaultConfig() *config.Config {
	return &config.Config{
		LogOutput: os.Stderr,
		Region:    "global",
	}
}

// Client is used to implement the client interaction with Nomad. Clients
// are expected to register as a schedulable node to the servers, and to
// run allocations as determined by the servers.
type Client struct {
	config *config.Config
	start  time.Time

	logger *log.Logger

	lastServer     net.Addr
	lastRPCTime    time.Time
	lastServerLock sync.Mutex

	servers    []string
	serverLock sync.RWMutex

	connPool *nomad.ConnPool

	lastHeartbeat time.Time
	heartbeatTTL  time.Duration

	// allocs is the current set of allocations
	allocs    map[string]*AllocRunner
	allocLock sync.RWMutex

	// discovery is the set of enabled discovery subsystems
	discovery []discovery.Discovery

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
}

// NewClient is used to create a new client from the given configuration
func NewClient(cfg *config.Config) (*Client, error) {
	// Create a logger
	logger := log.New(cfg.LogOutput, "", log.LstdFlags)

	// Create the client
	c := &Client{
		config:     cfg,
		start:      time.Now(),
		connPool:   nomad.NewPool(cfg.LogOutput, clientRPCCache, clientMaxStreams, nil),
		logger:     logger,
		allocs:     make(map[string]*AllocRunner),
		shutdownCh: make(chan struct{}),
	}

	// Initialize the client
	if err := c.init(); err != nil {
		return nil, fmt.Errorf("failed intializing client: %v", err)
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

	// Set up service discovery
	if err := c.setupDiscovery(); err != nil {
		return nil, fmt.Errorf("discovery setup failed: %v", err)
	}

	// Set up the known servers list
	c.SetServers(c.config.Servers)

	// Start the client!
	go c.run()
	return c, nil
}

// init is used to initialize the client and perform any setup
// needed before we begin starting its various components.
func (c *Client) init() error {
	// Ensure the state dir exists if we have one
	if c.config.StateDir != "" {
		if err := os.MkdirAll(c.config.StateDir, 0700); err != nil {
			return fmt.Errorf("failed creating state dir: %s", err)
		}

		c.logger.Printf("[INFO] client: using state directory %v", c.config.StateDir)
	}

	// Ensure the alloc dir exists if we have one
	if c.config.AllocDir != "" {
		if err := os.MkdirAll(c.config.AllocDir, 0700); err != nil {
			return fmt.Errorf("failed creating alloc dir: %s", err)
		}
	} else {
		// Othewise make a temp directory to use.
		p, err := ioutil.TempDir("", "NomadClient")
		if err != nil {
			return fmt.Errorf("failed creating temporary directory for the AllocDir: %v", err)
		}
		c.config.AllocDir = p
	}

	c.logger.Printf("[INFO] client: using alloc directory %v", c.config.AllocDir)
	return nil
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
	return c.saveState()
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
	servers := c.Servers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no known servers")
	}

	// Shuffle so we don't always use the same server
	shuffleStrings(servers)

	// Try to resolve each server
	for i := 0; i < len(servers); i++ {
		addr, err := net.ResolveTCPAddr("tcp", servers[i])
		if err == nil {
			c.lastServer = addr
			c.lastRPCTime = time.Now()
			return addr, nil
		}
		c.logger.Printf("[WARN] client: failed to resolve '%s': %s", servers[i], err)
	}

	// Bail if we reach this point
	return nil, fmt.Errorf("failed to resolve any servers")
}

// Servers is used to return the current known servers list. When an agent
// is first started, this list comes directly from configuration files.
func (c *Client) Servers() []string {
	c.serverLock.RLock()
	defer c.serverLock.RUnlock()
	return c.servers
}

// SetServers is used to modify the known servers list. This avoids forcing
// a config rollout + rolling restart and enables auto-join features. The
// full set of servers is passed to support adding and/or removing servers.
func (c *Client) SetServers(servers []string) {
	c.serverLock.Lock()
	defer c.serverLock.Unlock()
	if servers == nil {
		servers = make([]string, 0)
	}
	c.servers = servers
}

// Stats is used to return statistics for debugging and insight
// for various sub-systems
func (c *Client) Stats() map[string]map[string]string {
	toString := func(v uint64) string {
		return strconv.FormatUint(v, 10)
	}
	c.allocLock.RLock()
	numAllocs := len(c.allocs)
	c.allocLock.RUnlock()

	stats := map[string]map[string]string{
		"client": map[string]string{
			"known_servers":   toString(uint64(len(c.Servers()))),
			"num_allocations": toString(uint64(numAllocs)),
			"last_heartbeat":  fmt.Sprintf("%v", time.Since(c.lastHeartbeat)),
			"heartbeat_ttl":   fmt.Sprintf("%v", c.heartbeatTTL),
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

	// Scan the directory
	list, err := ioutil.ReadDir(filepath.Join(c.config.StateDir, "alloc"))
	if err != nil && os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to list alloc state: %v", err)
	}

	// Load each alloc back
	var mErr multierror.Error
	for _, entry := range list {
		id := entry.Name()
		alloc := &structs.Allocation{ID: id}
		ar := NewAllocRunner(c.logger, c.config, c.updateAllocStatus, alloc)
		c.allocs[id] = ar
		if err := ar.RestoreState(); err != nil {
			c.logger.Printf("[ERR] client: failed to restore state for alloc %s: %v",
				id, err)
			mErr.Errors = append(mErr.Errors, err)
		} else {
			go ar.Run()
		}
	}
	return mErr.ErrorOrNil()
}

// saveState is used to snapshot our state into the data dir
func (c *Client) saveState() error {
	if c.config.DevMode {
		return nil
	}

	var mErr multierror.Error
	c.allocLock.RLock()
	defer c.allocLock.RUnlock()
	for id, ar := range c.allocs {
		if err := ar.SaveState(); err != nil {
			c.logger.Printf("[ERR] client: failed to save state for alloc %s: %v",
				id, err)
			mErr.Errors = append(mErr.Errors, err)
		}
	}
	return mErr.ErrorOrNil()
}

// nodeID restores a persistent unique ID or generates a new one
func (c *Client) nodeID() (string, error) {
	// Do not persist in dev mode
	if c.config.DevMode {
		return structs.GenerateUUID(), nil
	}

	// Attempt to read existing ID
	path := filepath.Join(c.config.StateDir, "client-id")
	buf, err := ioutil.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	// Use existing ID if any
	if len(buf) != 0 {
		return string(buf), nil
	}

	// Generate new ID
	id := structs.GenerateUUID()

	// Persist the ID
	if err := ioutil.WriteFile(path, []byte(id), 0700); err != nil {
		return "", err
	}
	return id, nil
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
		id, err := c.nodeID()
		if err != nil {
			return fmt.Errorf("node ID setup failed: %v", err)
		}
		node.ID = id
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
	for _, name := range fingerprint.BuiltinFingerprints {
		f, err := fingerprint.NewFingerprint(name, c.logger)
		if err != nil {
			return err
		}
		applies, err := f.Fingerprint(c.config, c.config.Node)
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
	driverCtx := driver.NewDriverContext("", c.config, c.config.Node, c.logger)
	for name := range driver.BuiltinDrivers {
		d, err := driver.NewDriver(name, driverCtx)
		if err != nil {
			return err
		}
		applies, err := d.Fingerprint(c.config, c.config.Node)
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

// setupDiscovery sets up the discovery layers and initializes them.
func (c *Client) setupDiscovery() error {
	var avail []string
	ctx := discovery.NewContext(c.config, c.logger, c.config.Node)
	for name, fn := range discovery.Builtins {
		disc, err := fn(ctx)
		if err != nil {
			return err
		}
		if disc.Enabled() {
			c.discovery = append(c.discovery, disc)
			avail = append(avail, name)
		}
	}
	c.logger.Printf("[DEBUG] client: available discovery layers: %v", avail)
	return nil
}

// retryIntv calculates a retry interval value given the base
func (c *Client) retryIntv(base time.Duration) time.Duration {
	if c.config.DevMode {
		return devModeRetryIntv
	}
	return base + randomStagger(base)
}

// run is a long lived goroutine used to run the client
func (c *Client) run() {
	// Register the client
	for {
		if err := c.registerNode(); err == nil {
			break
		}
		select {
		case <-time.After(c.retryIntv(registerRetryIntv)):
		case <-c.shutdownCh:
			return
		}
	}

	// Setup the heartbeat timer, for the initial registration
	// we want to do this quickly. We want to do it extra quickly
	// in development mode.
	var heartbeat <-chan time.Time
	if c.config.DevMode {
		heartbeat = time.After(0)
	} else {
		heartbeat = time.After(randomStagger(initialHeartbeatStagger))
	}

	// Watch for changes in allocations
	allocUpdates := make(chan []*structs.Allocation, 1)
	go c.watchAllocations(allocUpdates)

	// Create a snapshot timer
	snapshot := time.After(stateSnapshotIntv)

	// Periodically update our status and wait for termination
	for {
		select {
		case <-snapshot:
			snapshot = time.After(stateSnapshotIntv)
			if err := c.saveState(); err != nil {
				c.logger.Printf("[ERR] client: failed to save state: %v", err)
			}

		case allocs := <-allocUpdates:
			c.runAllocs(allocs)

		case <-heartbeat:
			if err := c.updateNodeStatus(); err != nil {
				heartbeat = time.After(c.retryIntv(registerRetryIntv))
			} else {
				heartbeat = time.After(c.heartbeatTTL)
			}

		case <-c.shutdownCh:
			return
		}
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
	err := c.RPC("Node.Register", &req, &resp)
	if err != nil {
		if time.Since(c.start) > registerErrGrace {
			c.logger.Printf("[ERR] client: failed to register node: %v", err)
		}
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
	err := c.RPC("Node.UpdateStatus", &req, &resp)
	if err != nil {
		c.logger.Printf("[ERR] client: failed to update status: %v", err)
		return err
	}
	if len(resp.EvalIDs) != 0 {
		c.logger.Printf("[DEBUG] client: %d evaluations triggered by node update", len(resp.EvalIDs))
	}
	if resp.Index != 0 {
		c.logger.Printf("[DEBUG] client: state updated to %s", req.Status)
	}
	c.lastHeartbeat = time.Now()
	c.heartbeatTTL = resp.HeartbeatTTL
	return nil
}

// updateAllocStatus is used to update the status of an allocation
func (c *Client) updateAllocStatus(alloc *structs.Allocation) error {
	args := structs.AllocUpdateRequest{
		Alloc:        []*structs.Allocation{alloc},
		WriteRequest: structs.WriteRequest{Region: c.config.Region},
	}
	var resp structs.GenericResponse
	err := c.RPC("Node.UpdateAlloc", &args, &resp)
	if err != nil {
		c.logger.Printf("[ERR] client: failed to update allocation: %v", err)
		return err
	}
	return nil
}

// watchAllocations is used to scan for updates to allocations
func (c *Client) watchAllocations(allocUpdates chan []*structs.Allocation) {
	req := structs.NodeSpecificRequest{
		NodeID: c.Node().ID,
		QueryOptions: structs.QueryOptions{
			Region:     c.config.Region,
			AllowStale: true,
		},
	}
	var resp structs.NodeAllocsResponse

	for {
		// Get the allocations, blocking for updates
		resp = structs.NodeAllocsResponse{}
		err := c.RPC("Node.GetAllocs", &req, &resp)
		if err != nil {
			c.logger.Printf("[ERR] client: failed to query for node allocations: %v", err)
			retry := c.retryIntv(getAllocRetryIntv)
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
		if resp.Index <= req.MinQueryIndex {
			continue
		}
		req.MinQueryIndex = resp.Index
		c.logger.Printf("[DEBUG] client: updated allocations at index %d (%d allocs)", resp.Index, len(resp.Allocs))

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
	exist := make([]*structs.Allocation, 0, len(c.allocs))
	for _, ar := range c.allocs {
		exist = append(exist, ar.Alloc())
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
	c.allocLock.Lock()
	defer c.allocLock.Unlock()
	ar, ok := c.allocs[alloc.ID]
	if !ok {
		c.logger.Printf("[WARN] client: missing context for alloc '%s'", alloc.ID)
		return nil
	}
	ar.Destroy()

	// Deregister from service discovery
	for _, disc := range c.discovery {
		if err := disc.Deregister(c.Node().ID, "redis"); err != nil {
			return fmt.Errorf("failed to deregister: %v", err)
		}
	}

	delete(c.allocs, alloc.ID)
	return nil
}

// updateAlloc is invoked when we should update an allocation
func (c *Client) updateAlloc(exist, update *structs.Allocation) error {
	c.allocLock.RLock()
	defer c.allocLock.RUnlock()
	ar, ok := c.allocs[exist.ID]
	if !ok {
		c.logger.Printf("[WARN] client: missing context for alloc '%s'", exist.ID)
		return nil
	}
	ar.Update(update)
	return nil
}

// addAlloc is invoked when we should add an allocation
func (c *Client) addAlloc(alloc *structs.Allocation) error {
	c.allocLock.Lock()
	defer c.allocLock.Unlock()
	ar := NewAllocRunner(c.logger, c.config, c.updateAllocStatus, alloc)
	c.allocs[alloc.ID] = ar
	go ar.Run()

	// Register with service discovery
	ip := c.Node().Attributes["network.ip-address"]
	for _, disc := range c.discovery {
		if err := disc.Register(c.Node().ID, "redis", ip, nil, 1234); err != nil {
			return fmt.Errorf("failed to register: %v", err)
		}
	}

	return nil
}
