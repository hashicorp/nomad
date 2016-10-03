package client

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/client/stats"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/structs"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/mitchellh/hashstructure"
)

const (
	// clientRPCCache controls how long we keep an idle connection
	// open to a server
	clientRPCCache = 5 * time.Minute

	// clientMaxStreams controsl how many idle streams we keep
	// open to a server
	clientMaxStreams = 2

	// datacenterQueryLimit searches through up to this many adjacent
	// datacenters looking for the Nomad server service.
	datacenterQueryLimit = 9

	// consulReaperIntv is the interval at which the Consul reaper will
	// run.
	consulReaperIntv = 5 * time.Second

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
)

// ClientStatsReporter exposes all the APIs related to resource usage of a Nomad
// Client
type ClientStatsReporter interface {
	// GetAllocStats returns the AllocStatsReporter for the passed allocation.
	// If it does not exist an error is reported.
	GetAllocStats(allocID string) (AllocStatsReporter, error)

	// LatestHostStats returns the latest resource usage stats for the host
	LatestHostStats() *stats.HostStats
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

	connPool *nomad.ConnPool

	// servers is the (optionally prioritized) list of nomad servers
	servers *serverlist

	// heartbeat related times for tracking how often to heartbeat
	lastHeartbeat time.Time
	heartbeatTTL  time.Duration
	heartbeatLock sync.Mutex

	// triggerDiscoveryCh triggers Consul discovery; see triggerDiscovery
	triggerDiscoveryCh chan struct{}

	// discovered will be ticked whenever Consul discovery completes
	// succesfully
	serversDiscoveredCh chan struct{}

	// allocs is the current set of allocations
	allocs    map[string]*AllocRunner
	allocLock sync.RWMutex

	// blockedAllocations are allocations which are blocked because their
	// chained allocations haven't finished running
	blockedAllocations map[string]*structs.Allocation
	blockedAllocsLock  sync.RWMutex

	// allocUpdates stores allocations that need to be synced to the server.
	allocUpdates chan *structs.Allocation

	// consulSyncer advertises this Nomad Agent with Consul
	consulSyncer *consul.Syncer

	// HostStatsCollector collects host resource usage stats
	hostStatsCollector *stats.HostStatsCollector
	resourceUsage      *stats.HostStats
	resourceUsageLock  sync.RWMutex

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex

	// vaultClient is used to interact with Vault for token and secret renewals
	vaultClient vaultclient.VaultClient

	// migratingAllocs is the set of allocs whose data migration is in flight
	migratingAllocs     map[string]chan struct{}
	migratingAllocsLock sync.Mutex
}

var (
	// noServersErr is returned by the RPC method when the client has no
	// configured servers. This is used to trigger Consul discovery if
	// enabled.
	noServersErr = errors.New("no servers")
)

// NewClient is used to create a new client from the given configuration
func NewClient(cfg *config.Config, consulSyncer *consul.Syncer, logger *log.Logger) (*Client, error) {
	// Create the client
	c := &Client{
		config:             cfg,
		consulSyncer:       consulSyncer,
		start:              time.Now(),
		connPool:           nomad.NewPool(cfg.LogOutput, clientRPCCache, clientMaxStreams, nil),
		logger:             logger,
		hostStatsCollector: stats.NewHostStatsCollector(),
		allocs:             make(map[string]*AllocRunner),
		blockedAllocations: make(map[string]*structs.Allocation),
		allocUpdates:       make(chan *structs.Allocation, 64),
		shutdownCh:         make(chan struct{}),
		migratingAllocs:    make(map[string]chan struct{}),
		servers:            newServerList(),
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

	// Store the config copy before restoring state but after it has been
	// initialized.
	c.configLock.Lock()
	c.configCopy = c.config.Copy()
	c.configLock.Unlock()

	// Set the preconfigured list of static servers
	c.configLock.RLock()
	if len(c.configCopy.Servers) > 0 {
		if err := c.SetServers(c.configCopy.Servers); err != nil {
			logger.Printf("[WARN] client: None of the configured servers are valid: %v", err)
		}
	}
	c.configLock.RUnlock()

	// Setup Consul discovery if enabled
	if c.configCopy.ConsulConfig.ClientAutoJoin {
		go c.consulDiscovery()
		if len(c.servers.all()) == 0 {
			// No configured servers; trigger discovery manually
			c.triggerDiscoveryCh <- struct{}{}
		}
	}

	// Start Consul reaper
	go c.consulReaper()

	// Setup the vault client for token and secret renewals
	if err := c.setupVaultClient(); err != nil {
		return nil, fmt.Errorf("failed to setup vault client: %v", err)
	}

	// Restore the state
	if err := c.restoreState(); err != nil {
		return nil, fmt.Errorf("failed to restore state: %v", err)
	}

	// Register and then start heartbeating to the servers.
	go c.registerAndHeartbeat()

	// Begin periodic snapshotting of state.
	go c.periodicSnapshot()

	// Begin syncing allocations to the server
	go c.allocSync()

	// Start the client!
	go c.run()

	// Start collecting stats
	go c.collectHostStats()

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

// Datacenter returns the datacenter for the given client
func (c *Client) Datacenter() string {
	c.configLock.RLock()
	dc := c.configCopy.Node.Datacenter
	c.configLock.RUnlock()
	return dc
}

// Region returns the region for the given client
func (c *Client) Region() string {
	return c.config.Region
}

// RPCMajorVersion returns the structs.ApiMajorVersion supported by the
// client.
func (c *Client) RPCMajorVersion() int {
	return structs.ApiMajorVersion
}

// RPCMinorVersion returns the structs.ApiMinorVersion supported by the
// client.
func (c *Client) RPCMinorVersion() int {
	return structs.ApiMinorVersion
}

// Shutdown is used to tear down the client
func (c *Client) Shutdown() error {
	c.logger.Printf("[INFO] client: shutting down")
	c.shutdownLock.Lock()
	defer c.shutdownLock.Unlock()

	if c.shutdown {
		return nil
	}

	// Stop renewing tokens and secrets
	if c.vaultClient != nil {
		c.vaultClient.Stop()
	}

	// Destroy all the running allocations.
	if c.config.DevMode {
		c.allocLock.Lock()
		for _, ar := range c.allocs {
			ar.Destroy()
			<-ar.WaitCh()
		}
		c.allocLock.Unlock()
	}

	c.shutdown = true
	close(c.shutdownCh)
	c.connPool.Shutdown()
	return c.saveState()
}

// RPC is used to forward an RPC call to a nomad server, or fail if no servers.
func (c *Client) RPC(method string, args interface{}, reply interface{}) error {
	// Invoke the RPCHandler if it exists
	if c.config.RPCHandler != nil {
		return c.config.RPCHandler.RPC(method, args, reply)
	}

	servers := c.servers.all()
	if len(servers) == 0 {
		return noServersErr
	}

	var mErr multierror.Error
	for _, s := range servers {
		// Make the RPC request
		if err := c.connPool.RPC(c.Region(), s.addr, c.RPCMajorVersion(), method, args, reply); err != nil {
			errmsg := fmt.Errorf("RPC failed to server %s: %v", s.addr, err)
			mErr.Errors = append(mErr.Errors, errmsg)
			c.logger.Printf("[DEBUG] client: %v", errmsg)
			c.servers.failed(s)
			continue
		}
		c.servers.good(s)
		return nil
	}

	return mErr.ErrorOrNil()
}

// Stats is used to return statistics for debugging and insight
// for various sub-systems
func (c *Client) Stats() map[string]map[string]string {
	c.allocLock.RLock()
	numAllocs := len(c.allocs)
	c.allocLock.RUnlock()

	c.heartbeatLock.Lock()
	defer c.heartbeatLock.Unlock()
	stats := map[string]map[string]string{
		"client": map[string]string{
			"node_id":         c.Node().ID,
			"known_servers":   c.servers.all().String(),
			"num_allocations": strconv.Itoa(numAllocs),
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

// StatsReporter exposes the various APIs related resource usage of a Nomad
// client
func (c *Client) StatsReporter() ClientStatsReporter {
	return c
}

func (c *Client) GetAllocStats(allocID string) (AllocStatsReporter, error) {
	c.allocLock.RLock()
	defer c.allocLock.RUnlock()
	ar, ok := c.allocs[allocID]
	if !ok {
		return nil, fmt.Errorf("unknown allocation ID %q", allocID)
	}
	return ar.StatsReporter(), nil
}

// HostStats returns all the stats related to a Nomad client
func (c *Client) LatestHostStats() *stats.HostStats {
	c.resourceUsageLock.RLock()
	defer c.resourceUsageLock.RUnlock()
	return c.resourceUsage
}

// GetAllocFS returns the AllocFS interface for the alloc dir of an allocation
func (c *Client) GetAllocFS(allocID string) (allocdir.AllocDirFS, error) {
	c.allocLock.RLock()
	defer c.allocLock.RUnlock()

	ar, ok := c.allocs[allocID]
	if !ok {
		return nil, fmt.Errorf("alloc not found")
	}
	return ar.GetAllocDir(), nil
}

// GetServers returns the list of nomad servers this client is aware of.
func (c *Client) GetServers() []string {
	endpoints := c.servers.all()
	res := make([]string, len(endpoints))
	for i := range endpoints {
		res[i] = endpoints[i].addr.String()
	}
	return res
}

// SetServers sets a new list of nomad servers to connect to. As long as one
// server is resolvable no error is returned.
func (c *Client) SetServers(servers []string) error {
	endpoints := make([]*endpoint, 0, len(servers))
	var merr multierror.Error
	for _, s := range servers {
		addr, err := resolveServer(s)
		if err != nil {
			c.logger.Printf("[DEBUG] client: ignoring server %s due to resolution error: %v", s, err)
			merr.Errors = append(merr.Errors, err)
			continue
		}

		// Valid endpoint, append it without a priority as this API
		// doesn't support different priorities for different servers
		endpoints = append(endpoints, &endpoint{name: s, addr: addr})
	}

	// Only return errors if no servers are valid
	if len(endpoints) == 0 {
		if len(merr.Errors) > 0 {
			return merr.ErrorOrNil()
		}
		return noServersErr
	}

	c.servers.set(endpoints)
	return nil
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
		ar := NewAllocRunner(c.logger, c.configCopy, c.updateAllocStatus, alloc, c.vaultClient)
		c.configLock.RUnlock()
		c.allocLock.Lock()
		c.allocs[id] = ar
		c.allocLock.Unlock()
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

// nodeIDs restores the nodes persistent unique ID and SecretID or generates new
// ones
func (c *Client) nodeID() (id string, secret string, err error) {
	// Do not persist in dev mode
	if c.config.DevMode {
		return structs.GenerateUUID(), structs.GenerateUUID(), nil
	}

	// Attempt to read existing ID
	idPath := filepath.Join(c.config.StateDir, "client-id")
	idBuf, err := ioutil.ReadFile(idPath)
	if err != nil && !os.IsNotExist(err) {
		return "", "", err
	}

	// Attempt to read existing secret ID
	secretPath := filepath.Join(c.config.StateDir, "secret-id")
	secretBuf, err := ioutil.ReadFile(secretPath)
	if err != nil && !os.IsNotExist(err) {
		return "", "", err
	}

	// Use existing ID if any
	if len(idBuf) != 0 {
		id = string(idBuf)
	} else {
		// Generate new ID
		id = structs.GenerateUUID()

		// Persist the ID
		if err := ioutil.WriteFile(idPath, []byte(id), 0700); err != nil {
			return "", "", err
		}
	}

	if len(secretBuf) != 0 {
		secret = string(secretBuf)
	} else {
		// Generate new ID
		secret = structs.GenerateUUID()

		// Persist the ID
		if err := ioutil.WriteFile(secretPath, []byte(secret), 0700); err != nil {
			return "", "", err
		}
	}

	return id, secret, nil
}

// setupNode is used to setup the initial node
func (c *Client) setupNode() error {
	node := c.config.Node
	if node == nil {
		node = &structs.Node{}
		c.config.Node = node
	}
	// Generate an iD for the node
	id, secretID, err := c.nodeID()
	if err != nil {
		return fmt.Errorf("node ID setup failed: %v", err)
	}

	node.ID = id
	node.SecretID = secretID
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
			res.MBits = 0
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
	c.logger.Printf("[DEBUG] client: built-in fingerprints: %v", fingerprint.BuiltinFingerprints())

	var applied []string
	var skipped []string
	for _, name := range fingerprint.BuiltinFingerprints() {
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
	c.logger.Printf("[DEBUG] client: fingerprinting %v every %v", name, d)
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
	return base + lib.RandomStagger(base)
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
		heartbeat = time.After(lib.RandomStagger(initialHeartbeatStagger))
	}

	for {
		select {
		case <-c.serversDiscoveredCh:
		case <-heartbeat:
		case <-c.shutdownCh:
			return
		}

		if err := c.updateNodeStatus(); err != nil {
			// The servers have changed such that this node has not been
			// registered before
			if strings.Contains(err.Error(), "node not found") {
				// Re-register the node
				c.logger.Printf("[INFO] client: re-registering node")
				c.retryRegisterNode()
				heartbeat = time.After(lib.RandomStagger(initialHeartbeatStagger))
			} else {
				intv := c.retryIntv(registerRetryIntv)
				c.logger.Printf("[ERR] client: heartbeating failed. Retrying in %v: %v", intv, err)
				heartbeat = time.After(intv)

				// if heartbeating fails, trigger Consul discovery
				c.triggerDiscovery()
			}
		} else {
			c.heartbeatLock.Lock()
			heartbeat = time.After(c.heartbeatTTL)
			c.heartbeatLock.Unlock()
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
	for {
		err := c.registerNode()
		if err == nil {
			// Registered!
			return
		}

		if err == noServersErr {
			c.logger.Print("[DEBUG] client: registration waiting on servers")
			c.triggerDiscovery()
		} else {
			c.logger.Printf("[ERR] client: registration failure: %v", err)
		}
		select {
		case <-c.serversDiscoveredCh:
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
		WriteRequest: structs.WriteRequest{Region: c.Region()},
	}
	var resp structs.NodeUpdateResponse
	if err := c.RPC("Node.Register", &req, &resp); err != nil {
		return err
	}

	// Update the node status to ready after we register.
	c.configLock.Lock()
	node.Status = structs.NodeStatusReady
	c.configLock.Unlock()

	c.logger.Printf("[INFO] client: node registration complete")
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
	c.heartbeatLock.Lock()
	defer c.heartbeatLock.Unlock()

	node := c.Node()
	req := structs.NodeUpdateStatusRequest{
		NodeID:       node.ID,
		Status:       structs.NodeStatusReady,
		WriteRequest: structs.WriteRequest{Region: c.Region()},
	}
	var resp structs.NodeUpdateResponse
	if err := c.RPC("Node.UpdateStatus", &req, &resp); err != nil {
		c.triggerDiscovery()
		return fmt.Errorf("failed to update status: %v", err)
	}
	if len(resp.EvalIDs) != 0 {
		c.logger.Printf("[DEBUG] client: %d evaluations triggered by node update", len(resp.EvalIDs))
	}
	if resp.Index != 0 {
		c.logger.Printf("[DEBUG] client: state updated to %s", req.Status)
	}

	// Update heartbeat time and ttl
	c.lastHeartbeat = time.Now()
	c.heartbeatTTL = resp.HeartbeatTTL

	// Convert []*NodeServerInfo to []*endpoints
	localdc := c.Datacenter()
	servers := make(endpoints, 0, len(resp.Servers))
	for _, s := range resp.Servers {
		addr, err := resolveServer(s.RPCAdvertiseAddr)
		if err != nil {
			continue
		}
		e := endpoint{name: s.RPCAdvertiseAddr, addr: addr}
		if s.Datacenter != localdc {
			// server is non-local; de-prioritize
			e.priority = 1
		}
		servers = append(servers, &e)
	}
	if len(servers) == 0 {
		return fmt.Errorf("server returned no valid servers")
	}
	c.servers.set(servers)

	// Begin polling Consul if there is no Nomad leader.  We could be
	// heartbeating to a Nomad server that is in the minority of a
	// partition of the Nomad server quorum, but this Nomad Agent still
	// has connectivity to the existing majority of Nomad Servers, but
	// only if it queries Consul.
	if resp.LeaderRPCAddr == "" {
		c.triggerDiscovery()
	}

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

			// If this alloc was blocking another alloc and transitioned to a
			// terminal state then start the blocked allocation
			c.blockedAllocsLock.Lock()
			if blockedAlloc, ok := c.blockedAllocations[alloc.ID]; ok && alloc.Terminated() {
				var prevAllocDir *allocdir.AllocDir
				if ar, ok := c.getAllocRunners()[alloc.ID]; ok {
					prevAllocDir = ar.GetAllocDir()
				}
				if err := c.addAlloc(blockedAlloc, prevAllocDir); err != nil {
					c.logger.Printf("[ERR] client: failed to add alloc which was previously blocked %q: %v",
						blockedAlloc.ID, err)
				}
				delete(c.blockedAllocations, blockedAlloc.PreviousAllocation)
			}
			c.blockedAllocsLock.Unlock()
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
				WriteRequest: structs.WriteRequest{Region: c.Region()},
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
	n := c.Node()
	req := structs.NodeSpecificRequest{
		NodeID:   n.ID,
		SecretID: n.SecretID,
		QueryOptions: structs.QueryOptions{
			Region:     c.Region(),
			AllowStale: true,
		},
	}
	var resp structs.NodeClientAllocsResponse

	// The request and response for pulling down the set of allocations that are
	// new, or updated server side.
	allocsReq := structs.AllocsGetRequest{
		QueryOptions: structs.QueryOptions{
			Region:     c.Region(),
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
			// Shutdown often causes EOF errors, so check for shutdown first
			select {
			case <-c.shutdownCh:
				return
			default:
			}

			if err != noServersErr {
				c.logger.Printf("[ERR] client: failed to query for node allocations: %v", err)
			}
			retry := c.retryIntv(getAllocRetryIntv)
			select {
			case <-c.serversDiscoveredCh:
				continue
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
				case <-c.serversDiscoveredCh:
					continue
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

	// Initialize the hashes
	_, attrHash, metaHash := c.hasNodeChanged(0, 0)
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

		// See if the updated alloc is getting migrated
		c.migratingAllocsLock.Lock()
		ch, ok := c.migratingAllocs[update.updated.ID]
		c.migratingAllocsLock.Unlock()
		if ok {
			// Stopping the migration if the allocation doesn't need any
			// migration
			if !update.updated.ShouldMigrate() {
				close(ch)
			}
		}
	}

	// Start the new allocations
	for _, add := range diff.added {
		// If the allocation is chained and the previous allocation hasn't
		// terminated yet, then add the alloc to the blocked queue.
		ar, ok := c.getAllocRunners()[add.PreviousAllocation]
		if ok && !ar.Alloc().Terminated() {
			c.logger.Printf("[DEBUG] client: added alloc %q to blocked queue", add.ID)
			c.blockedAllocsLock.Lock()
			c.blockedAllocations[add.PreviousAllocation] = add
			c.blockedAllocsLock.Unlock()
			continue
		}

		// This means the allocation has a previous allocation on another node
		// so we will block for the previous allocation to complete
		if add.PreviousAllocation != "" && !ok {
			c.migratingAllocsLock.Lock()
			c.migratingAllocs[add.ID] = make(chan struct{})
			c.migratingAllocsLock.Unlock()
			go c.blockForRemoteAlloc(add)
			continue
		}

		// Setting the previous allocdir if the allocation had a terminal
		// previous allocation
		var prevAllocDir *allocdir.AllocDir
		tg := add.Job.LookupTaskGroup(add.TaskGroup)
		if tg != nil && tg.EphemeralDisk.Sticky == true && ar != nil {
			prevAllocDir = ar.GetAllocDir()
		}

		if err := c.addAlloc(add, prevAllocDir); err != nil {
			c.logger.Printf("[ERR] client: failed to add alloc '%s': %v",
				add.ID, err)
		}
	}

	// Persist our state
	if err := c.saveState(); err != nil {
		c.logger.Printf("[ERR] client: failed to save state: %v", err)
	}
}

// blockForRemoteAlloc blocks until the previous allocation of an allocation has
// been terminated and migrates the snapshot data
func (c *Client) blockForRemoteAlloc(alloc *structs.Allocation) {
	c.logger.Printf("[DEBUG] client: blocking alloc %q for previous allocation %q", alloc.ID, alloc.PreviousAllocation)

	// Removing the allocation from the set of allocs which are currently
	// undergoing migration
	defer func() {
		c.migratingAllocsLock.Lock()
		delete(c.migratingAllocs, alloc.ID)
		c.migratingAllocsLock.Unlock()
	}()

	// Block until the previous allocation migrates to terminal state
	prevAlloc, err := c.waitForAllocTerminal(alloc.PreviousAllocation)
	if err != nil {
		c.logger.Printf("[ERR] client: error waiting for allocation %q: %v", alloc.PreviousAllocation, err)
	}

	// Migrate the data from the remote node
	prevAllocDir, err := c.migrateRemoteAllocDir(prevAlloc, alloc.ID)
	if err != nil {
		c.logger.Printf("[ERR] client: error migrating data from remote alloc %q: %v", alloc.PreviousAllocation, err)
	}

	// Add the allocation
	if err := c.addAlloc(alloc, prevAllocDir); err != nil {
		c.logger.Printf("[ERR] client: error adding alloc: %v", err)
	}
}

// waitForAllocTerminal waits for an allocation with the given alloc id to
// transition to terminal state and blocks the caller until then.
func (c *Client) waitForAllocTerminal(allocID string) (*structs.Allocation, error) {
	req := structs.AllocSpecificRequest{
		AllocID: allocID,
		QueryOptions: structs.QueryOptions{
			Region:     c.Region(),
			AllowStale: true,
		},
	}

	for {
		resp := structs.SingleAllocResponse{}
		err := c.RPC("Alloc.GetAlloc", &req, &resp)
		if err != nil {
			c.logger.Printf("[ERR] client: failed to query allocation %q: %v", allocID, err)
			retry := c.retryIntv(getAllocRetryIntv)
			select {
			case <-time.After(retry):
				continue
			case <-c.shutdownCh:
				return nil, fmt.Errorf("aborting because client is shutting down")
			}
		}
		if resp.Alloc == nil {
			return nil, nil
		}
		if resp.Alloc.Terminated() {
			return resp.Alloc, nil
		}

		// Update the query index.
		if resp.Index > req.MinQueryIndex {
			req.MinQueryIndex = resp.Index
		}

	}
}

// migrateRemoteAllocDir migrates the allocation directory from a remote node to
// the current node
func (c *Client) migrateRemoteAllocDir(alloc *structs.Allocation, allocID string) (*allocdir.AllocDir, error) {
	if alloc == nil {
		return nil, nil
	}

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		return nil, fmt.Errorf("Task Group %q not found in job %q", tg.Name, alloc.Job.ID)
	}

	// Skip migration of data if the ephemeral disk is not sticky or
	// migration is turned off.
	if !tg.EphemeralDisk.Sticky || !tg.EphemeralDisk.Migrate {
		return nil, nil
	}

	node, err := c.getNode(alloc.NodeID)

	// If the node is down then skip migrating the data
	if err != nil {
		return nil, fmt.Errorf("error retreiving node %v: %v", alloc.NodeID, err)
	}

	// Check if node is nil
	if node == nil {
		return nil, fmt.Errorf("node %q doesn't exist", alloc.NodeID)
	}

	// skip migration if the remote node is down
	if node.Status == structs.NodeStatusDown {
		c.logger.Printf("[INFO] client: not migrating data from alloc %q since node %q is down", alloc.ID, alloc.NodeID)
		return nil, nil
	}

	// Create the previous alloc dir
	pathToAllocDir := filepath.Join(c.config.AllocDir, alloc.ID)
	if err := os.MkdirAll(pathToAllocDir, 0777); err != nil {
		c.logger.Printf("[ERR] client: error creating previous allocation dir: %v", err)
	}

	// Get the snapshot
	url := fmt.Sprintf("http://%v/v1/client/allocation/%v/snapshot", node.HTTPAddr, alloc.ID)
	resp, err := http.Get(url)
	if err != nil {
		os.RemoveAll(pathToAllocDir)
		c.logger.Printf("[ERR] client: error getting snapshot: %v", err)
		return nil, fmt.Errorf("error getting snapshot for alloc %v: %v", alloc.ID, err)
	}
	tr := tar.NewReader(resp.Body)
	defer resp.Body.Close()

	buf := make([]byte, 1024)

	stopMigrating, ok := c.migratingAllocs[allocID]
	if !ok {
		os.RemoveAll(pathToAllocDir)
		return nil, fmt.Errorf("couldn't find a migration validity notifier for alloc: %v", alloc.ID)
	}
	for {
		// See if the alloc still needs migration
		select {
		case <-stopMigrating:
			os.RemoveAll(pathToAllocDir)
			c.logger.Printf("[INFO] client: stopping migration of allocdir for alloc: %v", alloc.ID)
			return nil, nil
		case <-c.shutdownCh:
			os.RemoveAll(pathToAllocDir)
			c.logger.Printf("[INFO] client: stopping migration of alloc %q since client is shutting down", alloc.ID)
			return nil, nil
		default:
		}

		// Get the next header
		hdr, err := tr.Next()

		// If the snapshot has ended then we create the previous
		// allocdir
		if err == io.EOF {
			prevAllocDir := allocdir.NewAllocDir(pathToAllocDir, 0)
			return prevAllocDir, nil
		}
		// If there is an error then we avoid creating the alloc dir
		if err != nil {
			os.RemoveAll(pathToAllocDir)
			return nil, fmt.Errorf("error creating alloc dir for alloc %q: %v", alloc.ID, err)
		}

		// If the header is for a directory we create the directory
		if hdr.Typeflag == tar.TypeDir {
			os.MkdirAll(filepath.Join(pathToAllocDir, hdr.Name), 0777)
			continue
		}
		// If the header is a file, we write to a file
		if hdr.Typeflag == tar.TypeReg {
			f, err := os.Create(filepath.Join(pathToAllocDir, hdr.Name))
			if err != nil {
				c.logger.Printf("[ERR] client: error creating file: %v", err)
				continue
			}

			// We write in chunks of 32 bytes so that we can test if
			// the client is still alive
			for {
				if c.shutdown {
					f.Close()
					os.RemoveAll(pathToAllocDir)
					c.logger.Printf("[INFO] client: stopping migration of alloc %q because client is shutting down", alloc.ID)
					return nil, nil
				}

				n, err := tr.Read(buf)
				if err != nil {
					f.Close()
					if err != io.EOF {
						return nil, fmt.Errorf("error reading snapshot: %v", err)
					}
					break
				}
				if _, err := f.Write(buf[:n]); err != nil {
					f.Close()
					os.RemoveAll(pathToAllocDir)
					return nil, fmt.Errorf("error writing to file %q: %v", f.Name(), err)
				}
			}

		}
	}
}

// getNode gets the node from the server with the given Node ID
func (c *Client) getNode(nodeID string) (*structs.Node, error) {
	req := structs.NodeSpecificRequest{
		NodeID: nodeID,
		QueryOptions: structs.QueryOptions{
			Region:     c.Region(),
			AllowStale: true,
		},
	}

	resp := structs.SingleNodeResponse{}
	for {
		err := c.RPC("Node.GetNode", &req, &resp)
		if err != nil {
			c.logger.Printf("[ERR] client: failed to query node info %q: %v", nodeID, err)
			retry := c.retryIntv(getAllocRetryIntv)
			select {
			case <-time.After(retry):
				continue
			case <-c.shutdownCh:
				return nil, fmt.Errorf("aborting because client is shutting down")
			}
		}
		break
	}

	return resp.Node, nil
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
func (c *Client) addAlloc(alloc *structs.Allocation, prevAllocDir *allocdir.AllocDir) error {
	c.configLock.RLock()
	ar := NewAllocRunner(c.logger, c.configCopy, c.updateAllocStatus, alloc, c.vaultClient)
	ar.SetPreviousAllocDir(prevAllocDir)
	c.configLock.RUnlock()
	go ar.Run()

	// Store the alloc runner.
	c.allocLock.Lock()
	c.allocs[alloc.ID] = ar
	c.allocLock.Unlock()
	return nil
}

// setupVaultClient creates an object to periodically renew tokens and secrets
// with vault.
func (c *Client) setupVaultClient() error {
	var err error
	if c.vaultClient, err =
		vaultclient.NewVaultClient(c.config.VaultConfig, c.logger, c.deriveToken); err != nil {
		return err
	}

	if c.vaultClient == nil {
		c.logger.Printf("[ERR] client: failed to create vault client")
		return fmt.Errorf("failed to create vault client")
	}

	// Start renewing tokens and secrets
	c.vaultClient.Start()

	return nil
}

// deriveToken takes in an allocation and a set of tasks and derives vault
// tokens for each of the tasks, unwraps all of them using the supplied vault
// client and returns a map of unwrapped tokens, indexed by the task name.
func (c *Client) deriveToken(alloc *structs.Allocation, taskNames []string, vclient *vaultapi.Client) (map[string]string, error) {
	if alloc == nil {
		return nil, fmt.Errorf("nil allocation")
	}

	if taskNames == nil || len(taskNames) == 0 {
		return nil, fmt.Errorf("missing task names")
	}

	group := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if group == nil {
		return nil, fmt.Errorf("group name in allocation is not present in job")
	}

	verifiedTasks := []string{}
	found := false
	// Check if the given task names actually exist in the allocation
	for _, taskName := range taskNames {
		found = false
		for _, task := range group.Tasks {
			if task.Name == taskName {
				found = true
			}
		}
		if !found {
			c.logger.Printf("[ERR] task %q not found in the allocation", taskName)
			return nil, fmt.Errorf("task %q not found in the allocaition", taskName)
		}
		verifiedTasks = append(verifiedTasks, taskName)
	}

	// DeriveVaultToken of nomad server can take in a set of tasks and
	// creates tokens for all the tasks.
	req := &structs.DeriveVaultTokenRequest{
		NodeID:   c.Node().ID,
		SecretID: c.Node().SecretID,
		AllocID:  alloc.ID,
		Tasks:    verifiedTasks,
		QueryOptions: structs.QueryOptions{
			Region:     c.Region(),
			AllowStale: true,
		},
	}

	// Derive the tokens
	var resp structs.DeriveVaultTokenResponse
	if err := c.RPC("Node.DeriveVaultToken", &req, &resp); err != nil {
		c.logger.Printf("[ERR] client.vault: failed to derive vault tokens: %v", err)
		return nil, fmt.Errorf("failed to derive vault tokens: %v", err)
	}
	if resp.Tasks == nil {
		c.logger.Printf("[ERR] client.vault: failed to derive vault token: invalid response")
		return nil, fmt.Errorf("failed to derive vault tokens: invalid response")
	}

	unwrappedTokens := make(map[string]string)

	// Retrieve the wrapped tokens from the response and unwrap it
	for _, taskName := range verifiedTasks {
		// Get the wrapped token
		wrappedToken, ok := resp.Tasks[taskName]
		if !ok {
			c.logger.Printf("[ERR] client.vault: wrapped token missing for task %q", taskName)
			return nil, fmt.Errorf("wrapped token missing for task %q", taskName)
		}

		// Unwrap the vault token
		unwrapResp, err := vclient.Logical().Unwrap(wrappedToken)
		if err != nil {
			return nil, fmt.Errorf("failed to unwrap the token for task %q: %v", taskName, err)
		}
		if unwrapResp == nil || unwrapResp.Auth == nil || unwrapResp.Auth.ClientToken == "" {
			return nil, fmt.Errorf("failed to unwrap the token for task %q", taskName)
		}

		// Append the unwrapped token to the return value
		unwrappedTokens[taskName] = unwrapResp.Auth.ClientToken
	}

	return unwrappedTokens, nil
}

// triggerDiscovery causes a Consul discovery to begin (if one hasn't alread)
func (c *Client) triggerDiscovery() {
	select {
	case c.triggerDiscoveryCh <- struct{}{}:
		// Discovery goroutine was released to execute
	default:
		// Discovery goroutine was already running
	}
}

// consulDiscovery waits for the signal to attempt server discovery via Consul.
// It's intended to be started in a goroutine. See triggerDiscovery() for
// causing consul discovery from other code locations.
func (c *Client) consulDiscovery() {
	for {
		select {
		case <-c.triggerDiscoveryCh:
			if err := c.consulDiscoveryImpl(); err != nil {
				c.logger.Printf("[ERR] client.consul: error discovering nomad servers: %v", err)
			}
		case <-c.shutdownCh:
			return
		}
	}
}

func (c *Client) consulDiscoveryImpl() error {
	// Acquire heartbeat lock to prevent heartbeat from running
	// concurrently with discovery. Concurrent execution is safe, however
	// discovery is usually triggered when heartbeating has failed so
	// there's no point in allowing it.
	c.heartbeatLock.Lock()
	defer c.heartbeatLock.Unlock()

	consulCatalog := c.consulSyncer.ConsulClient().Catalog()
	dcs, err := consulCatalog.Datacenters()
	if err != nil {
		return fmt.Errorf("client.consul: unable to query Consul datacenters: %v", err)
	}
	if len(dcs) > 2 {
		// Query the local DC first, then shuffle the
		// remaining DCs.  Future heartbeats will cause Nomad
		// Clients to fixate on their local datacenter so
		// it's okay to talk with remote DCs.  If the no
		// Nomad servers are available within
		// datacenterQueryLimit, the next heartbeat will pick
		// a new set of servers so it's okay.
		shuffleStrings(dcs[1:])
		dcs = dcs[0:lib.MinInt(len(dcs), datacenterQueryLimit)]
	}

	// Query for servers in this client's region only
	region := c.Region()
	rpcargs := structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: region,
		},
	}

	serviceName := c.configCopy.ConsulConfig.ServerServiceName
	var mErr multierror.Error
	var servers endpoints
	c.logger.Printf("[DEBUG] client.consul: bootstrap contacting following Consul DCs: %+q", dcs)
DISCOLOOP:
	for _, dc := range dcs {
		consulOpts := &consulapi.QueryOptions{
			AllowStale: true,
			Datacenter: dc,
			Near:       "_agent",
			WaitTime:   consul.DefaultQueryWaitDuration,
		}
		consulServices, _, err := consulCatalog.Service(serviceName, consul.ServiceTagRPC, consulOpts)
		if err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("unable to query service %+q from Consul datacenter %+q: %v", serviceName, dc, err))
			continue
		}

		for _, s := range consulServices {
			port := strconv.Itoa(s.ServicePort)
			addrstr := s.ServiceAddress
			if addrstr == "" {
				addrstr = s.Address
			}
			addr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(addrstr, port))
			if err != nil {
				mErr.Errors = append(mErr.Errors, err)
				continue
			}
			var peers []string
			if err := c.connPool.RPC(region, addr, c.RPCMajorVersion(), "Status.Peers", rpcargs, &peers); err != nil {
				mErr.Errors = append(mErr.Errors, err)
				continue
			}

			// Successfully received the Server peers list of the correct
			// region
			for _, p := range peers {
				addr, err := net.ResolveTCPAddr("tcp", p)
				if err != nil {
					mErr.Errors = append(mErr.Errors, err)
				}
				servers = append(servers, &endpoint{name: p, addr: addr})
			}
			if len(servers) > 0 {
				break DISCOLOOP
			}
		}
	}
	if len(servers) == 0 {
		if len(mErr.Errors) > 0 {
			return mErr.ErrorOrNil()
		}
		return fmt.Errorf("no Nomad Servers advertising service %q in Consul datacenters: %+q", serviceName, dcs)
	}

	c.logger.Printf("[INFO] client.consul: discovered following Servers: %s", servers)
	c.servers.set(servers)

	// Notify waiting rpc calls. If a goroutine just failed an RPC call and
	// isn't receiving on this chan yet they'll still retry eventually.
	// This is a shortcircuit for the longer retry intervals.
	for {
		select {
		case c.serversDiscoveredCh <- struct{}{}:
		default:
			return nil
		}
	}
}

// consulReaper periodically reaps unmatched domains from Consul. Intended to
// be called in its own goroutine. See consulReaperIntv for interval.
func (c *Client) consulReaper() {
	ticker := time.NewTicker(consulReaperIntv)
	defer ticker.Stop()
	lastok := true
	for {
		select {
		case <-ticker.C:
			if err := c.consulReaperImpl(); err != nil {
				if lastok {
					c.logger.Printf("[ERR] consul.client: error reaping services in consul: %v", err)
					lastok = false
				}
			} else {
				lastok = true
			}
		case <-c.shutdownCh:
			return
		}
	}
}

// consulReaperImpl reaps unmatched domains from Consul.
func (c *Client) consulReaperImpl() error {
	const estInitialExecutorDomains = 8

	// Create the domains to keep and add the server and client
	domains := make([]consul.ServiceDomain, 2, estInitialExecutorDomains)
	domains[0] = consul.ServerDomain
	domains[1] = consul.ClientDomain

	for allocID, ar := range c.getAllocRunners() {
		ar.taskStatusLock.RLock()
		taskStates := copyTaskStates(ar.taskStates)
		ar.taskStatusLock.RUnlock()
		for taskName, taskState := range taskStates {
			// Only keep running tasks
			if taskState.State == structs.TaskStateRunning {
				d := consul.NewExecutorDomain(allocID, taskName)
				domains = append(domains, d)
			}
		}
	}

	return c.consulSyncer.ReapUnmatched(domains)
}

// collectHostStats collects host resource usage stats periodically
func (c *Client) collectHostStats() {
	// Start collecting host stats right away and then keep collecting every
	// collection interval
	next := time.NewTimer(0)
	defer next.Stop()
	for {
		select {
		case <-next.C:
			ru, err := c.hostStatsCollector.Collect()
			next.Reset(c.config.StatsCollectionInterval)
			if err != nil {
				c.logger.Printf("[WARN] client: error fetching host resource usage stats: %v", err)
				continue
			}

			c.resourceUsageLock.Lock()
			c.resourceUsage = ru
			c.resourceUsageLock.Unlock()

			// Publish Node metrics if operator has opted in
			if c.config.PublishNodeMetrics {
				c.emitStats(ru)
			}
		case <-c.shutdownCh:
			return
		}
	}
}

// emitStats pushes host resource usage stats to remote metrics collection sinks
func (c *Client) emitStats(hStats *stats.HostStats) {
	nodeID := c.Node().ID
	metrics.SetGauge([]string{"client", "host", "memory", nodeID, "total"}, float32(hStats.Memory.Total))
	metrics.SetGauge([]string{"client", "host", "memory", nodeID, "available"}, float32(hStats.Memory.Available))
	metrics.SetGauge([]string{"client", "host", "memory", nodeID, "used"}, float32(hStats.Memory.Used))
	metrics.SetGauge([]string{"client", "host", "memory", nodeID, "free"}, float32(hStats.Memory.Free))

	metrics.SetGauge([]string{"uptime"}, float32(hStats.Uptime))

	for _, cpu := range hStats.CPU {
		metrics.SetGauge([]string{"client", "host", "cpu", nodeID, cpu.CPU, "total"}, float32(cpu.Total))
		metrics.SetGauge([]string{"client", "host", "cpu", nodeID, cpu.CPU, "user"}, float32(cpu.User))
		metrics.SetGauge([]string{"client", "host", "cpu", nodeID, cpu.CPU, "idle"}, float32(cpu.Idle))
		metrics.SetGauge([]string{"client", "host", "cpu", nodeID, cpu.CPU, "system"}, float32(cpu.System))
	}

	for _, disk := range hStats.DiskStats {
		metrics.SetGauge([]string{"client", "host", "disk", nodeID, disk.Device, "size"}, float32(disk.Size))
		metrics.SetGauge([]string{"client", "host", "disk", nodeID, disk.Device, "used"}, float32(disk.Used))
		metrics.SetGauge([]string{"client", "host", "disk", nodeID, disk.Device, "available"}, float32(disk.Available))
		metrics.SetGauge([]string{"client", "host", "disk", nodeID, disk.Device, "used_percent"}, float32(disk.UsedPercent))
		metrics.SetGauge([]string{"client", "host", "disk", nodeID, disk.Device, "inodes_percent"}, float32(disk.InodesUsedPercent))
	}
}

// resolveServer given a sever's address as a string, return it's resolved
// net.Addr or an error.
func resolveServer(s string) (net.Addr, error) {
	const defaultClientPort = "4647" // default client RPC port
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		if strings.Contains(err.Error(), "missing port") {
			host = s
			port = defaultClientPort
		} else {
			return nil, err
		}
	}
	return net.ResolveTCPAddr("tcp", net.JoinHostPort(host, port))
}
