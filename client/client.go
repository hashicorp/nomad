package client

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/hashstructure"
)

const (
	// clientRPCCache controls how long we keep an idle connection
	// open to a server
	clientRPCCache = 5 * time.Minute

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

	// nodeUpdateRetryIntv is how often the client checks for updates to the
	// node attributes or meta map.
	nodeUpdateRetryIntv = 5 * time.Second

	// allocSyncIntv is the batching period of allocation updates before they
	// are synced with the server.
	allocSyncIntv = 200 * time.Millisecond

	// allocSyncRetryIntv is the interval on which we retry updating
	// the status of the allocation
	allocSyncRetryIntv = 5 * time.Second

	// consulSyncInterval is the interval at which the client syncs with consul
	// to remove services and checks which are no longer valid
	consulSyncInterval = 15 * time.Second
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

	// configCopy is a copy that should be passed to alloc-runners.
	configCopy *config.Config
	configLock sync.RWMutex

	logger *log.Logger

	lastServer     net.Addr
	lastRPCTime    time.Time
	lastServerLock sync.Mutex

	servers    []string
	serverLock sync.RWMutex

	connPool *nomad.ConnPool

	lastHeartbeat time.Time
	heartbeatTTL  time.Duration
	heartbeatLock sync.Mutex

	// allocs is the current set of allocations
	allocs    map[string]*AllocRunner
	allocLock sync.RWMutex

	// allocUpdates stores allocations that need to be synced to the server.
	allocUpdates chan *structs.Allocation

	consulService *consul.ConsulService

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
		config:       cfg,
		start:        time.Now(),
		connPool:     nomad.NewPool(cfg.LogOutput, clientRPCCache, clientMaxStreams, nil),
		logger:       logger,
		allocs:       make(map[string]*AllocRunner),
		allocUpdates: make(chan *structs.Allocation, 64),
		shutdownCh:   make(chan struct{}),
	}

	// Initialize the client
	if err := c.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize client: %v", err)
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

	// Setup the reserved resources
	c.reservePorts()

	// Set up the known servers list
	c.SetServers(c.config.Servers)

	// Store the config copy before restoring state but after it has been
	// initialized.
	c.configCopy = c.config.Copy()

	// Restore the state
	if err := c.restoreState(); err != nil {
		return nil, fmt.Errorf("failed to restore state: %v", err)
	}

	// Setup the consul client
	if err := c.setupConsulClient(); err != nil {
		return nil, fmt.Errorf("failed to create consul client: %v")
	}

	// Register and then start heartbeating to the servers.
	go c.registerAndHeartbeat()

	// Begin periodic snapshotting of state.
	go c.periodicSnapshot()

	// Begin syncing allocations to the server
	go c.allocSync()

	// Start the client!
	go c.run()

	// Start the consul sync
	go c.syncConsul()

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

	} else {
		// Othewise make a temp directory to use.
		p, err := ioutil.TempDir("", "NomadClient")
		if err != nil {
			return fmt.Errorf("failed creating temporary directory for the StateDir: %v", err)
		}
		c.config.StateDir = p
	}
	c.logger.Printf("[INFO] client: using state directory %v", c.config.StateDir)

	// Ensure the alloc dir exists if we have one
	if c.config.AllocDir != "" {
		if err := os.MkdirAll(c.config.AllocDir, 0755); err != nil {
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

	// Destroy all the running allocations.
	if c.config.DevMode {
		for _, ar := range c.allocs {
			ar.Destroy()
			<-ar.WaitCh()
		}
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
	// net.ResolveTCPAddr requires port to be set, if one is not provided, supply default port
	// Using net.SplitHostPort in the event of IPv6 addresses with multiple colons.
	// IPv6 addresses must be passed in with brackets,
	// i.e: [::1]:4647 or [::1]
	setServers := make([]string, len(servers))
	copy(setServers, servers)
	for i := 0; i < len(setServers); i++ {
		if _, _, err := net.SplitHostPort(setServers[i]); err != nil {
			// multiple errors can be returned here, only searching for missing
			if strings.Contains(err.Error(), "missing port") {
				c.logger.Printf("[WARN] client: port not specified, using default port")
				setServers[i] = net.JoinHostPort(setServers[i], "4647")
			} else {
				c.logger.Printf("[WARN] client: server address %q invalid: %v", setServers[i], err)
			}
		}
	}

	c.logger.Printf("[INFO] client: setting server address list: %s", setServers)
	c.servers = setServers
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
			"node_id":         c.Node().ID,
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
	c.configLock.RLock()
	defer c.configLock.RUnlock()
	return c.config.Node
}

// GetAllocFS returns the AllocFS interface for the alloc dir of an allocation
func (c *Client) GetAllocFS(allocID string) (allocdir.AllocDirFS, error) {
	ar, ok := c.allocs[allocID]
	if !ok {
		return nil, fmt.Errorf("alloc not found")
	}
	return ar.ctx.AllocDir, nil
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
		c.configLock.RLock()
		ar := NewAllocRunner(c.logger, c.configCopy, c.updateAllocStatus, alloc)
		c.configLock.RUnlock()
		c.allocs[id] = ar
		if err := ar.RestoreState(); err != nil {
			c.logger.Printf("[ERR] client: failed to restore state for alloc %s: %v", id, err)
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
	for id, ar := range c.getAllocRunners() {
		if err := ar.SaveState(); err != nil {
			c.logger.Printf("[ERR] client: failed to save state for alloc %s: %v",
				id, err)
			mErr.Errors = append(mErr.Errors, err)
		}
	}
	return mErr.ErrorOrNil()
}

// getAllocRunners returns a snapshot of the current set of alloc runners.
func (c *Client) getAllocRunners() map[string]*AllocRunner {
	c.allocLock.RLock()
	defer c.allocLock.RUnlock()
	runners := make(map[string]*AllocRunner, len(c.allocs))
	for id, ar := range c.allocs {
		runners[id] = ar
	}
	return runners
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
	// Generate an iD for the node
	var err error
	node.ID, err = c.nodeID()
	if err != nil {
		return fmt.Errorf("node ID setup failed: %v", err)
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
	if node.Reserved == nil {
		node.Reserved = &structs.Resources{}
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

// reservePorts is used to reserve ports on the fingerprinted network devices.
func (c *Client) reservePorts() {
	c.configLock.RLock()
	defer c.configLock.RUnlock()
	global := c.config.GloballyReservedPorts
	if len(global) == 0 {
		return
	}

	node := c.config.Node
	networks := node.Resources.Networks
	reservedIndex := make(map[string]*structs.NetworkResource, len(networks))
	for _, resNet := range node.Reserved.Networks {
		reservedIndex[resNet.IP] = resNet
	}

	// Go through each network device and reserve ports on it.
	for _, net := range networks {
		res, ok := reservedIndex[net.IP]
		if !ok {
			res = net.Copy()
			reservedIndex[net.IP] = res
		}

		for _, portVal := range global {
			p := structs.Port{Value: portVal}
			res.ReservedPorts = append(res.ReservedPorts, p)
		}
	}

	// Clear the reserved networks.
	if node.Reserved == nil {
		node.Reserved = new(structs.Resources)
	} else {
		node.Reserved.Networks = nil
	}

	// Restore the reserved networks
	for _, net := range reservedIndex {
		node.Reserved.Networks = append(node.Reserved.Networks, net)
	}
}

// fingerprint is used to fingerprint the client and setup the node
func (c *Client) fingerprint() error {
	whitelist := c.config.ReadStringListToMap("fingerprint.whitelist")
	whitelistEnabled := len(whitelist) > 0

	var applied []string
	var skipped []string
	for _, name := range fingerprint.BuiltinFingerprints {
		// Skip modules that are not in the whitelist if it is enabled.
		if _, ok := whitelist[name]; whitelistEnabled && !ok {
			skipped = append(skipped, name)
			continue
		}
		f, err := fingerprint.NewFingerprint(name, c.logger)
		if err != nil {
			return err
		}

		c.configLock.Lock()
		applies, err := f.Fingerprint(c.config, c.config.Node)
		c.configLock.Unlock()
		if err != nil {
			return err
		}
		if applies {
			applied = append(applied, name)
		}
		p, period := f.Periodic()
		if p {
			// TODO: If more periodic fingerprinters are added, then
			// fingerprintPeriodic should be used to handle all the periodic
			// fingerprinters by using a priority queue.
			go c.fingerprintPeriodic(name, f, period)
		}
	}
	c.logger.Printf("[DEBUG] client: applied fingerprints %v", applied)
	if len(skipped) != 0 {
		c.logger.Printf("[DEBUG] client: fingerprint modules skipped due to whitelist: %v", skipped)
	}
	return nil
}

// fingerprintPeriodic runs a fingerprinter at the specified duration.
func (c *Client) fingerprintPeriodic(name string, f fingerprint.Fingerprint, d time.Duration) {
	c.logger.Printf("[DEBUG] client: periodically fingerprinting %v at duration %v", name, d)
	for {
		select {
		case <-time.After(d):
			c.configLock.Lock()
			if _, err := f.Fingerprint(c.config, c.config.Node); err != nil {
				c.logger.Printf("[DEBUG] client: periodic fingerprinting for %v failed: %v", name, err)
			}
			c.configLock.Unlock()
		case <-c.shutdownCh:
			return
		}
	}
}

// setupDrivers is used to find the available drivers
func (c *Client) setupDrivers() error {
	// Build the whitelist of drivers.
	whitelist := c.config.ReadStringListToMap("driver.whitelist")
	whitelistEnabled := len(whitelist) > 0

	var avail []string
	var skipped []string
	driverCtx := driver.NewDriverContext("", c.config, c.config.Node, c.logger, nil)
	for name := range driver.BuiltinDrivers {
		// Skip fingerprinting drivers that are not in the whitelist if it is
		// enabled.
		if _, ok := whitelist[name]; whitelistEnabled && !ok {
			skipped = append(skipped, name)
			continue
		}

		d, err := driver.NewDriver(name, driverCtx)
		if err != nil {
			return err
		}
		c.configLock.Lock()
		applies, err := d.Fingerprint(c.config, c.config.Node)
		c.configLock.Unlock()
		if err != nil {
			return err
		}
		if applies {
			avail = append(avail, name)
		}

		p, period := d.Periodic()
		if p {
			go c.fingerprintPeriodic(name, d, period)
		}

	}

	c.logger.Printf("[DEBUG] client: available drivers %v", avail)

	if len(skipped) != 0 {
		c.logger.Printf("[DEBUG] client: drivers skipped due to whitelist: %v", skipped)
	}

	return nil
}

// retryIntv calculates a retry interval value given the base
func (c *Client) retryIntv(base time.Duration) time.Duration {
	if c.config.DevMode {
		return devModeRetryIntv
	}
	return base + randomStagger(base)
}

// registerAndHeartbeat is a long lived goroutine used to register the client
// and then start heartbeatng to the server.
func (c *Client) registerAndHeartbeat() {
	// Register the node
	c.retryRegisterNode()

	// Start watching changes for node changes
	go c.watchNodeUpdates()

	// Setup the heartbeat timer, for the initial registration
	// we want to do this quickly. We want to do it extra quickly
	// in development mode.
	var heartbeat <-chan time.Time
	if c.config.DevMode {
		heartbeat = time.After(0)
	} else {
		heartbeat = time.After(randomStagger(initialHeartbeatStagger))
	}

	for {
		select {
		case <-heartbeat:
			if err := c.updateNodeStatus(); err != nil {
				heartbeat = time.After(c.retryIntv(registerRetryIntv))
			} else {
				c.heartbeatLock.Lock()
				heartbeat = time.After(c.heartbeatTTL)
				c.heartbeatLock.Unlock()
			}

		case <-c.shutdownCh:
			return
		}
	}
}

// periodicSnapshot is a long lived goroutine used to periodically snapshot the
// state of the client
func (c *Client) periodicSnapshot() {
	// Create a snapshot timer
	snapshot := time.After(stateSnapshotIntv)

	for {
		select {
		case <-snapshot:
			snapshot = time.After(stateSnapshotIntv)
			if err := c.saveState(); err != nil {
				c.logger.Printf("[ERR] client: failed to save state: %v", err)
			}

		case <-c.shutdownCh:
			return
		}
	}
}

// run is a long lived goroutine used to run the client
func (c *Client) run() {
	// Watch for changes in allocations
	allocUpdates := make(chan *allocUpdates, 8)
	go c.watchAllocations(allocUpdates)

	for {
		select {
		case update := <-allocUpdates:
			c.runAllocs(update)

		case <-c.shutdownCh:
			return
		}
	}
}

// hasNodeChanged calculates a hash for the node attributes- and meta map.
// The new hash values are compared against the old (passed-in) hash values to
// determine if the node properties have changed. It returns the new hash values
// in case they are different from the old hash values.
func (c *Client) hasNodeChanged(oldAttrHash uint64, oldMetaHash uint64) (bool, uint64, uint64) {
	c.configLock.RLock()
	defer c.configLock.RUnlock()
	newAttrHash, err := hashstructure.Hash(c.config.Node.Attributes, nil)
	if err != nil {
		c.logger.Printf("[DEBUG] client: unable to calculate node attributes hash: %v", err)
	}
	// Calculate node meta map hash
	newMetaHash, err := hashstructure.Hash(c.config.Node.Meta, nil)
	if err != nil {
		c.logger.Printf("[DEBUG] client: unable to calculate node meta hash: %v", err)
	}
	if newAttrHash != oldAttrHash || newMetaHash != oldMetaHash {
		return true, newAttrHash, newMetaHash
	}
	return false, oldAttrHash, oldMetaHash
}

// retryRegisterNode is used to register the node or update the registration and
// retry in case of failure.
func (c *Client) retryRegisterNode() {
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

	// Update the node status to ready after we register.
	c.configLock.Lock()
	node.Status = structs.NodeStatusReady
	c.configLock.Unlock()

	c.logger.Printf("[DEBUG] client: node registration complete")
	if len(resp.EvalIDs) != 0 {
		c.logger.Printf("[DEBUG] client: %d evaluations triggered by node registration", len(resp.EvalIDs))
	}

	c.heartbeatLock.Lock()
	defer c.heartbeatLock.Unlock()
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

	c.heartbeatLock.Lock()
	defer c.heartbeatLock.Unlock()
	c.lastHeartbeat = time.Now()
	c.heartbeatTTL = resp.HeartbeatTTL
	return nil
}

// updateAllocStatus is used to update the status of an allocation
func (c *Client) updateAllocStatus(alloc *structs.Allocation) {
	// Only send the fields that are updatable by the client.
	stripped := new(structs.Allocation)
	stripped.ID = alloc.ID
	stripped.NodeID = c.Node().ID
	stripped.TaskStates = alloc.TaskStates
	stripped.ClientStatus = alloc.ClientStatus
	stripped.ClientDescription = alloc.ClientDescription
	select {
	case c.allocUpdates <- stripped:
	case <-c.shutdownCh:
	}
}

// allocSync is a long lived function that batches allocation updates to the
// server.
func (c *Client) allocSync() {
	staggered := false
	syncTicker := time.NewTicker(allocSyncIntv)
	updates := make(map[string]*structs.Allocation)
	for {
		select {
		case <-c.shutdownCh:
			syncTicker.Stop()
			return
		case alloc := <-c.allocUpdates:
			// Batch the allocation updates until the timer triggers.
			updates[alloc.ID] = alloc
		case <-syncTicker.C:
			// Fast path if there are no updates
			if len(updates) == 0 {
				continue
			}

			sync := make([]*structs.Allocation, 0, len(updates))
			for _, alloc := range updates {
				sync = append(sync, alloc)
			}

			// Send to server.
			args := structs.AllocUpdateRequest{
				Alloc:        sync,
				WriteRequest: structs.WriteRequest{Region: c.config.Region},
			}

			var resp structs.GenericResponse
			if err := c.RPC("Node.UpdateAlloc", &args, &resp); err != nil {
				c.logger.Printf("[ERR] client: failed to update allocations: %v", err)
				syncTicker.Stop()
				syncTicker = time.NewTicker(c.retryIntv(allocSyncRetryIntv))
				staggered = true
			} else {
				updates = make(map[string]*structs.Allocation)
				if staggered {
					syncTicker.Stop()
					syncTicker = time.NewTicker(allocSyncIntv)
					staggered = false
				}
			}
		}
	}
}

// allocUpdates holds the results of receiving updated allocations from the
// servers.
type allocUpdates struct {
	// pulled is the set of allocations that were downloaded from the servers.
	pulled map[string]*structs.Allocation

	// filtered is the set of allocations that were not pulled because their
	// AllocModifyIndex didn't change.
	filtered map[string]struct{}
}

// watchAllocations is used to scan for updates to allocations
func (c *Client) watchAllocations(updates chan *allocUpdates) {
	// The request and response for getting the map of allocations that should
	// be running on the Node to their AllocModifyIndex which is incremented
	// when the allocation is updated by the servers.
	req := structs.NodeSpecificRequest{
		NodeID: c.Node().ID,
		QueryOptions: structs.QueryOptions{
			Region:     c.config.Region,
			AllowStale: true,
		},
	}
	var resp structs.NodeClientAllocsResponse

	// The request and response for pulling down the set of allocations that are
	// new, or updated server side.
	allocsReq := structs.AllocsGetRequest{
		QueryOptions: structs.QueryOptions{
			Region:     c.config.Region,
			AllowStale: true,
		},
	}
	var allocsResp structs.AllocsGetResponse

	for {
		// Get the allocation modify index map, blocking for updates. We will
		// use this to determine exactly what allocations need to be downloaded
		// in full.
		resp = structs.NodeClientAllocsResponse{}
		err := c.RPC("Node.GetClientAllocs", &req, &resp)
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

		// Filter all allocations whose AllocModifyIndex was not incremented.
		// These are the allocations who have either not been updated, or whose
		// updates are a result of the client sending an update for the alloc.
		// This lets us reduce the network traffic to the server as we don't
		// need to pull all the allocations.
		var pull []string
		filtered := make(map[string]struct{})
		runners := c.getAllocRunners()
		for allocID, modifyIndex := range resp.Allocs {
			// Pull the allocation if we don't have an alloc runner for the
			// allocation or if the alloc runner requires an updated allocation.
			runner, ok := runners[allocID]
			if !ok || runner.shouldUpdate(modifyIndex) {
				pull = append(pull, allocID)
			} else {
				filtered[allocID] = struct{}{}
			}
		}

		c.logger.Printf("[DEBUG] client: updated allocations at index %d (pulled %d) (filtered %d)",
			resp.Index, len(pull), len(filtered))

		// Pull the allocations that passed filtering.
		allocsResp.Allocs = nil
		if len(pull) != 0 {
			// Pull the allocations that need to be updated.
			allocsReq.AllocIDs = pull
			allocsResp = structs.AllocsGetResponse{}
			if err := c.RPC("Alloc.GetAllocs", &allocsReq, &allocsResp); err != nil {
				c.logger.Printf("[ERR] client: failed to query updated allocations: %v", err)
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
		}

		// Update the query index.
		if resp.Index > req.MinQueryIndex {
			req.MinQueryIndex = resp.Index
		}

		// Push the updates.
		pulled := make(map[string]*structs.Allocation, len(allocsResp.Allocs))
		for _, alloc := range allocsResp.Allocs {
			pulled[alloc.ID] = alloc
		}
		update := &allocUpdates{
			filtered: filtered,
			pulled:   pulled,
		}
		select {
		case updates <- update:
		case <-c.shutdownCh:
			return
		}
	}
}

// watchNodeUpdates periodically checks for changes to the node attributes or meta map
func (c *Client) watchNodeUpdates() {
	c.logger.Printf("[DEBUG] client: periodically checking for node changes at duration %v", nodeUpdateRetryIntv)
	var attrHash, metaHash uint64
	var changed bool
	for {
		select {
		case <-time.After(c.retryIntv(nodeUpdateRetryIntv)):
			changed, attrHash, metaHash = c.hasNodeChanged(attrHash, metaHash)
			if changed {
				c.logger.Printf("[DEBUG] client: state changed, updating node.")

				// Update the config copy.
				c.configLock.Lock()
				node := c.config.Node.Copy()
				c.configCopy.Node = node
				c.configLock.Unlock()

				c.retryRegisterNode()
			}
		case <-c.shutdownCh:
			return
		}
	}
}

// runAllocs is invoked when we get an updated set of allocations
func (c *Client) runAllocs(update *allocUpdates) {
	// Get the existing allocs
	c.allocLock.RLock()
	exist := make([]*structs.Allocation, 0, len(c.allocs))
	for _, ar := range c.allocs {
		exist = append(exist, ar.alloc)
	}
	c.allocLock.RUnlock()

	// Diff the existing and updated allocations
	diff := diffAllocs(exist, update)
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
	ar, ok := c.allocs[alloc.ID]
	if !ok {
		c.allocLock.Unlock()
		c.logger.Printf("[WARN] client: missing context for alloc '%s'", alloc.ID)
		return nil
	}
	delete(c.allocs, alloc.ID)
	c.allocLock.Unlock()

	ar.Destroy()
	return nil
}

// updateAlloc is invoked when we should update an allocation
func (c *Client) updateAlloc(exist, update *structs.Allocation) error {
	c.allocLock.RLock()
	ar, ok := c.allocs[exist.ID]
	c.allocLock.RUnlock()
	if !ok {
		c.logger.Printf("[WARN] client: missing context for alloc '%s'", exist.ID)
		return nil
	}

	ar.Update(update)
	return nil
}

// addAlloc is invoked when we should add an allocation
func (c *Client) addAlloc(alloc *structs.Allocation) error {
	c.configLock.RLock()
	ar := NewAllocRunner(c.logger, c.configCopy, c.updateAllocStatus, alloc)
	c.configLock.RUnlock()
	go ar.Run()

	// Store the alloc runner.
	c.allocLock.Lock()
	c.allocs[alloc.ID] = ar
	c.allocLock.Unlock()
	return nil
}

// setupConsulClient creates a ConsulService
func (c *Client) setupConsulClient() error {
	cfg := consul.ConsulConfig{
		Addr:      c.config.ReadDefault("consul.address", "127.0.0.1:8500"),
		Token:     c.config.Read("consul.token"),
		Auth:      c.config.Read("consul.auth"),
		EnableSSL: c.config.ReadBoolDefault("consul.ssl", false),
		VerifySSL: c.config.ReadBoolDefault("consul.verifyssl", true),
	}

	cs, err := consul.NewConsulService(&cfg, c.logger, "")
	c.consulService = cs
	return err
}

// syncConsul removes services of tasks which are no longer in running state
func (c *Client) syncConsul() {
	sync := time.NewTicker(consulSyncInterval)
	for {
		select {
		case <-sync.C:
			// Give up pruning services if we can't fingerprint Consul
			if _, ok := c.config.Node.Attributes["consul.server"]; !ok {
				continue
			}
			services := make(map[string]struct{})
			// Get the existing allocs
			c.allocLock.RLock()
			allocs := make([]*AllocRunner, 0, len(c.allocs))
			for _, ar := range c.allocs {
				allocs = append(allocs, ar)
			}
			c.allocLock.RUnlock()
			for _, ar := range allocs {
				ar.taskStatusLock.RLock()
				taskStates := copyTaskStates(ar.taskStates)
				ar.taskStatusLock.RUnlock()
				for taskName, taskState := range taskStates {
					if taskState.State == structs.TaskStateRunning {
						if tr, ok := ar.tasks[taskName]; ok {
							for _, service := range tr.task.Services {
								services[service.ID(ar.alloc.ID, tr.task.Name)] = struct{}{}
							}
						}
					}
				}
			}
			if err := c.consulService.KeepServices(services); err != nil {
				c.logger.Printf("[DEBUG] client: error removing services from non-running tasks: %v", err)
			}
		case <-c.shutdownCh:
			sync.Stop()
			c.logger.Printf("[INFO] client: shutting down consul sync")
			return
		}

	}
}
