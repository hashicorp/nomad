package client

import (
	"errors"
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

	metrics "github.com/armon/go-metrics"
	"github.com/boltdb/bolt"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/client/stats"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/structs"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/mitchellh/hashstructure"
	"github.com/shirou/gopsutil/host"
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
	// starting and the initial heartbeat. After the initial heartbeat,
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

	// stateDB is used to efficiently store client state.
	stateDB *bolt.DB

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
	// successfully
	serversDiscoveredCh chan struct{}

	// allocs maps alloc IDs to their AllocRunner. This map includes all
	// AllocRunners - running and GC'd - until the server GCs them.
	allocs    map[string]*AllocRunner
	allocLock sync.RWMutex

	// allocUpdates stores allocations that need to be synced to the server.
	allocUpdates chan *structs.Allocation

	// consulService is Nomad's custom Consul client for managing services
	// and checks.
	consulService ConsulServiceAPI

	// consulCatalog is the subset of Consul's Catalog API Nomad uses.
	consulCatalog consul.CatalogAPI

	// HostStatsCollector collects host resource usage stats
	hostStatsCollector *stats.HostStatsCollector

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex

	// vaultClient is used to interact with Vault for token and secret renewals
	vaultClient vaultclient.VaultClient

	// garbageCollector is used to garbage collect terminal allocations present
	// in the node automatically
	garbageCollector *AllocGarbageCollector

	// clientACLResolver holds the ACL resolution state
	clientACLResolver

	// baseLabels are used when emitting tagged metrics. All client metrics will
	// have these tags, and optionally more.
	baseLabels []metrics.Label
}

var (
	// noServersErr is returned by the RPC method when the client has no
	// configured servers. This is used to trigger Consul discovery if
	// enabled.
	noServersErr = errors.New("no servers")
)

// NewClient is used to create a new client from the given configuration
func NewClient(cfg *config.Config, consulCatalog consul.CatalogAPI, consulService ConsulServiceAPI, logger *log.Logger) (*Client, error) {
	// Create the tls wrapper
	var tlsWrap tlsutil.RegionWrapper
	if cfg.TLSConfig.EnableRPC {
		tw, err := cfg.TLSConfiguration().OutgoingTLSWrapper()
		if err != nil {
			return nil, err
		}
		tlsWrap = tw
	}

	// Create the client
	c := &Client{
		config:              cfg,
		consulCatalog:       consulCatalog,
		consulService:       consulService,
		start:               time.Now(),
		connPool:            nomad.NewPool(cfg.LogOutput, clientRPCCache, clientMaxStreams, tlsWrap),
		logger:              logger,
		allocs:              make(map[string]*AllocRunner),
		allocUpdates:        make(chan *structs.Allocation, 64),
		shutdownCh:          make(chan struct{}),
		servers:             newServerList(),
		triggerDiscoveryCh:  make(chan struct{}),
		serversDiscoveredCh: make(chan struct{}),
	}

	// Initialize the client
	if err := c.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize client: %v", err)
	}

	// Initialize the ACL state
	if err := c.clientACLResolver.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize ACL state: %v", err)
	}

	// Add the stats collector
	statsCollector := stats.NewHostStatsCollector(logger, c.config.AllocDir)
	c.hostStatsCollector = statsCollector

	// Add the garbage collector
	gcConfig := &GCConfig{
		MaxAllocs:           cfg.GCMaxAllocs,
		DiskUsageThreshold:  cfg.GCDiskUsageThreshold,
		InodeUsageThreshold: cfg.GCInodeUsageThreshold,
		Interval:            cfg.GCInterval,
		ParallelDestroys:    cfg.GCParallelDestroys,
		ReservedDiskMB:      cfg.Node.Reserved.DiskMB,
	}
	c.garbageCollector = NewAllocGarbageCollector(logger, statsCollector, c, gcConfig)
	go c.garbageCollector.Run()

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
	if c.configCopy.ConsulConfig.ClientAutoJoin != nil && *c.configCopy.ConsulConfig.ClientAutoJoin {
		go c.consulDiscovery()
		if len(c.servers.all()) == 0 {
			// No configured servers; trigger discovery manually
			c.triggerDiscoveryCh <- struct{}{}
		}
	}

	// Setup the vault client for token and secret renewals
	if err := c.setupVaultClient(); err != nil {
		return nil, fmt.Errorf("failed to setup vault client: %v", err)
	}

	// Restore the state
	if err := c.restoreState(); err != nil {
		logger.Printf("[ERR] client: failed to restore state: %v", err)
		logger.Printf("[ERR] client: Nomad is unable to start due to corrupt state. "+
			"The safest way to proceed is to manually stop running task processes "+
			"and remove Nomad's state (%q) and alloc (%q) directories before "+
			"restarting. Lost allocations will be rescheduled.",
			c.config.StateDir, c.config.AllocDir)
		logger.Printf("[ERR] client: Corrupt state is often caused by a bug. Please " +
			"report as much information as possible to " +
			"https://github.com/hashicorp/nomad/issues")
		return nil, fmt.Errorf("failed to restore state")
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
	go c.emitStats()

	c.logger.Printf("[INFO] client: Node ID %q", c.NodeID())
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

		p, err = filepath.EvalSymlinks(p)
		if err != nil {
			return fmt.Errorf("failed to find temporary directory for the StateDir: %v", err)
		}

		c.config.StateDir = p
	}
	c.logger.Printf("[INFO] client: using state directory %v", c.config.StateDir)

	// Create or open the state database
	db, err := bolt.Open(filepath.Join(c.config.StateDir, "state.db"), 0600, nil)
	if err != nil {
		return fmt.Errorf("failed to create state database: %v", err)
	}
	c.stateDB = db

	// Ensure the alloc dir exists if we have one
	if c.config.AllocDir != "" {
		if err := os.MkdirAll(c.config.AllocDir, 0711); err != nil {
			return fmt.Errorf("failed creating alloc dir: %s", err)
		}
	} else {
		// Othewise make a temp directory to use.
		p, err := ioutil.TempDir("", "NomadClient")
		if err != nil {
			return fmt.Errorf("failed creating temporary directory for the AllocDir: %v", err)
		}

		p, err = filepath.EvalSymlinks(p)
		if err != nil {
			return fmt.Errorf("failed to find temporary directory for the AllocDir: %v", err)
		}

		// Change the permissions to have the execute bit
		if err := os.Chmod(p, 0711); err != nil {
			return fmt.Errorf("failed to change directory permissions for the AllocDir: %v", err)
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
	return c.config.Node.Datacenter
}

// Region returns the region for the given client
func (c *Client) Region() string {
	return c.config.Region
}

// NodeID returns the node ID for the given client
func (c *Client) NodeID() string {
	return c.config.Node.ID
}

// secretNodeID returns the secret node ID for the given client
func (c *Client) secretNodeID() string {
	return c.config.Node.SecretID
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

	// Defer closing the database
	defer func() {
		if err := c.stateDB.Close(); err != nil {
			c.logger.Printf("[ERR] client: failed to close state database on shutdown: %v", err)
		}
	}()

	// Stop renewing tokens and secrets
	if c.vaultClient != nil {
		c.vaultClient.Stop()
	}

	// Stop Garbage collector
	c.garbageCollector.Stop()

	// Destroy all the running allocations.
	if c.config.DevMode {
		for _, ar := range c.getAllocRunners() {
			ar.Destroy()
			<-ar.WaitCh()
		}
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
	c.heartbeatLock.Lock()
	defer c.heartbeatLock.Unlock()
	stats := map[string]map[string]string{
		"client": {
			"node_id":         c.NodeID(),
			"known_servers":   c.servers.all().String(),
			"num_allocations": strconv.Itoa(c.NumAllocs()),
			"last_heartbeat":  fmt.Sprintf("%v", time.Since(c.lastHeartbeat)),
			"heartbeat_ttl":   fmt.Sprintf("%v", c.heartbeatTTL),
		},
		"runtime": nomad.RuntimeStats(),
	}
	return stats
}

// CollectAllocation garbage collects a single allocation on a node. Returns
// true if alloc was found and garbage collected; otherwise false.
func (c *Client) CollectAllocation(allocID string) bool {
	return c.garbageCollector.Collect(allocID)
}

// CollectAllAllocs garbage collects all allocations on a node in the terminal
// state
func (c *Client) CollectAllAllocs() {
	c.garbageCollector.CollectAll()
}

// Node returns the locally registered node
func (c *Client) Node() *structs.Node {
	c.configLock.RLock()
	defer c.configLock.RUnlock()
	return c.configCopy.Node
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
	return c.hostStatsCollector.Stats()
}

// ValidateMigrateToken verifies that a token is for a specific client and
// allocation, and has been created by a trusted party that has privileged
// knowledge of the client's secret identifier
func (c *Client) ValidateMigrateToken(allocID, migrateToken string) bool {
	if !c.config.ACLEnabled {
		return true
	}

	return nomad.CompareMigrateToken(allocID, c.secretNodeID(), migrateToken)
}

// GetAllocFS returns the AllocFS interface for the alloc dir of an allocation
func (c *Client) GetAllocFS(allocID string) (allocdir.AllocDirFS, error) {
	c.allocLock.RLock()
	defer c.allocLock.RUnlock()

	ar, ok := c.allocs[allocID]
	if !ok {
		return nil, fmt.Errorf("unknown allocation ID %q", allocID)
	}
	return ar.GetAllocDir(), nil
}

// GetClientAlloc returns the allocation from the client
func (c *Client) GetClientAlloc(allocID string) (*structs.Allocation, error) {
	all := c.allAllocs()
	alloc, ok := all[allocID]
	if !ok {
		return nil, fmt.Errorf("unknown allocation ID %q", allocID)
	}
	return alloc, nil
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

	// COMPAT: Remove in 0.7.0
	// 0.6.0 transistioned from individual state files to a single bolt-db.
	// The upgrade path is to:
	// Check if old state exists
	//   If so, restore from that and delete old state
	// Restore using state database

	// Allocs holds the IDs of the allocations being restored
	var allocs []string

	// Upgrading tracks whether this is a pre 0.6.0 upgrade path
	var upgrading bool

	// Scan the directory
	allocDir := filepath.Join(c.config.StateDir, "alloc")
	list, err := ioutil.ReadDir(allocDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to list alloc state: %v", err)
	} else if err == nil && len(list) != 0 {
		upgrading = true
		for _, entry := range list {
			allocs = append(allocs, entry.Name())
		}
	} else {
		// Normal path
		err := c.stateDB.View(func(tx *bolt.Tx) error {
			allocs, err = getAllAllocationIDs(tx)
			if err != nil {
				return fmt.Errorf("failed to list allocations: %v", err)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	// Load each alloc back
	var mErr multierror.Error
	for _, id := range allocs {
		alloc := &structs.Allocation{ID: id}

		// don't worry about blocking/migrating when restoring
		watcher := noopPrevAlloc{}

		c.configLock.RLock()
		ar := NewAllocRunner(c.logger, c.configCopy, c.stateDB, c.updateAllocStatus, alloc, c.vaultClient, c.consulService, watcher)
		c.configLock.RUnlock()

		c.allocLock.Lock()
		c.allocs[id] = ar
		c.allocLock.Unlock()

		if err := ar.RestoreState(); err != nil {
			c.logger.Printf("[ERR] client: failed to restore state for alloc %q: %v", id, err)
			mErr.Errors = append(mErr.Errors, err)
		} else {
			go ar.Run()

			if upgrading {
				if err := ar.SaveState(); err != nil {
					c.logger.Printf("[WARN] client: initial save state for alloc %q failed: %v", id, err)
				}
			}
		}
	}

	// Delete all the entries
	if upgrading {
		if err := os.RemoveAll(allocDir); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	return mErr.ErrorOrNil()
}

// saveState is used to snapshot our state into the data dir.
func (c *Client) saveState() error {
	if c.config.DevMode {
		return nil
	}

	var wg sync.WaitGroup
	var l sync.Mutex
	var mErr multierror.Error
	runners := c.getAllocRunners()
	wg.Add(len(runners))

	for id, ar := range runners {
		go func(id string, ar *AllocRunner) {
			err := ar.SaveState()
			if err != nil {
				c.logger.Printf("[ERR] client: failed to save state for alloc %q: %v", id, err)
				l.Lock()
				multierror.Append(&mErr, err)
				l.Unlock()
			}
			wg.Done()
		}(id, ar)
	}

	wg.Wait()
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

// NumAllocs returns the number of un-GC'd allocs this client has. Used to
// fulfill the AllocCounter interface for the GC.
func (c *Client) NumAllocs() int {
	n := 0
	c.allocLock.RLock()
	for _, a := range c.allocs {
		if !a.IsDestroyed() {
			n++
		}
	}
	c.allocLock.RUnlock()
	return n
}

// nodeID restores, or generates if necessary, a unique node ID and SecretID.
// The node ID is, if available, a persistent unique ID.  The secret ID is a
// high-entropy random UUID.
func (c *Client) nodeID() (id, secret string, err error) {
	var hostID string
	hostInfo, err := host.Info()
	if !c.config.NoHostUUID && err == nil {
		if hashed, ok := helper.HashUUID(hostInfo.HostID); ok {
			hostID = hashed
		}
	}

	if hostID == "" {
		// Generate a random hostID if no constant ID is available on
		// this platform.
		hostID = uuid.Generate()
	}

	// Do not persist in dev mode
	if c.config.DevMode {
		return hostID, uuid.Generate(), nil
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
		id = strings.ToLower(string(idBuf))
	} else {
		id = hostID

		// Persist the ID
		if err := ioutil.WriteFile(idPath, []byte(id), 0700); err != nil {
			return "", "", err
		}
	}

	if len(secretBuf) != 0 {
		secret = string(secretBuf)
	} else {
		// Generate new ID
		secret = uuid.Generate()

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
	// Generate an ID and secret for the node
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
	blacklist := c.config.ReadStringListToMap("fingerprint.blacklist")

	c.logger.Printf("[DEBUG] client: built-in fingerprints: %v", fingerprint.BuiltinFingerprints())

	var applied []string
	var skipped []string
	for _, name := range fingerprint.BuiltinFingerprints() {
		// Skip modules that are not in the whitelist if it is enabled.
		if _, ok := whitelist[name]; whitelistEnabled && !ok {
			skipped = append(skipped, name)
			continue
		}
		// Skip modules that are in the blacklist
		if _, ok := blacklist[name]; ok {
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
		c.logger.Printf("[DEBUG] client: fingerprint modules skipped due to white/blacklist: %v", skipped)
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
	// Build the white/blacklists of drivers.
	whitelist := c.config.ReadStringListToMap("driver.whitelist")
	whitelistEnabled := len(whitelist) > 0
	blacklist := c.config.ReadStringListToMap("driver.blacklist")

	var avail []string
	var skipped []string
	driverCtx := driver.NewDriverContext("", "", c.config, c.config.Node, c.logger, nil)
	for name := range driver.BuiltinDrivers {
		// Skip fingerprinting drivers that are not in the whitelist if it is
		// enabled.
		if _, ok := whitelist[name]; whitelistEnabled && !ok {
			skipped = append(skipped, name)
			continue
		}
		// Skip fingerprinting drivers that are in the blacklist
		if _, ok := blacklist[name]; ok {
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
		c.logger.Printf("[DEBUG] client: drivers skipped due to white/blacklist: %v", skipped)
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

	req := structs.NodeUpdateStatusRequest{
		NodeID:       c.NodeID(),
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
			c.logger.Printf("[WARN] client: ignoring invalid server %q: %v", s.RPCAdvertiseAddr, err)
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
	if alloc.Terminated() {
		// Terminated, mark for GC if we're still tracking this alloc
		// runner. If it's not being tracked that means the server has
		// already GC'd it (see removeAlloc).
		c.allocLock.RLock()
		ar, ok := c.allocs[alloc.ID]
		c.allocLock.RUnlock()

		if ok {
			c.garbageCollector.MarkForCollection(ar)

			// Trigger a GC in case we're over thresholds and just
			// waiting for eligible allocs.
			c.garbageCollector.Trigger()
		}
	}

	// Strip all the information that can be reconstructed at the server.  Only
	// send the fields that are updatable by the client.
	stripped := new(structs.Allocation)
	stripped.ID = alloc.ID
	stripped.NodeID = c.NodeID()
	stripped.TaskStates = alloc.TaskStates
	stripped.ClientStatus = alloc.ClientStatus
	stripped.ClientDescription = alloc.ClientDescription
	stripped.DeploymentStatus = alloc.DeploymentStatus

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

	// migrateTokens are a list of tokens necessary for when clients pull data
	// from authorized volumes
	migrateTokens map[string]string
}

// watchAllocations is used to scan for updates to allocations
func (c *Client) watchAllocations(updates chan *allocUpdates) {
	// The request and response for getting the map of allocations that should
	// be running on the Node to their AllocModifyIndex which is incremented
	// when the allocation is updated by the servers.
	req := structs.NodeSpecificRequest{
		NodeID:   c.NodeID(),
		SecretID: c.secretNodeID(),
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

OUTER:
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

			// COMPAT: Remove in 0.6. This is to allow the case in which the
			// servers are not fully upgraded before the clients register. This
			// can cause the SecretID to be lost
			if strings.Contains(err.Error(), "node secret ID does not match") {
				c.logger.Printf("[DEBUG] client: re-registering node as there was a secret ID mismatch: %v", err)
				c.retryRegisterNode()
			} else if err != noServersErr {
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
		var pullIndex uint64
		for allocID, modifyIndex := range resp.Allocs {
			// Pull the allocation if we don't have an alloc runner for the
			// allocation or if the alloc runner requires an updated allocation.
			runner, ok := runners[allocID]

			if !ok || runner.shouldUpdate(modifyIndex) {
				// Only pull allocs that are required. Filtered
				// allocs might be at a higher index, so ignore
				// it.
				if modifyIndex > pullIndex {
					pullIndex = modifyIndex
				}
				pull = append(pull, allocID)
			} else {
				filtered[allocID] = struct{}{}
			}
		}

		// Pull the allocations that passed filtering.
		allocsResp.Allocs = nil
		var pulledAllocs map[string]*structs.Allocation
		if len(pull) != 0 {
			// Pull the allocations that need to be updated.
			allocsReq.AllocIDs = pull
			allocsReq.MinQueryIndex = pullIndex - 1
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

			// Ensure that we received all the allocations we wanted
			pulledAllocs = make(map[string]*structs.Allocation, len(allocsResp.Allocs))
			for _, alloc := range allocsResp.Allocs {
				pulledAllocs[alloc.ID] = alloc
			}

			for _, desiredID := range pull {
				if _, ok := pulledAllocs[desiredID]; !ok {
					// We didn't get everything we wanted. Do not update the
					// MinQueryIndex, sleep and then retry.
					wait := c.retryIntv(2 * time.Second)
					select {
					case <-time.After(wait):
						// Wait for the server we contact to receive the
						// allocations
						continue OUTER
					case <-c.shutdownCh:
						return
					}
				}
			}

			// Check for shutdown
			select {
			case <-c.shutdownCh:
				return
			default:
			}
		}

		c.logger.Printf("[DEBUG] client: updated allocations at index %d (total %d) (pulled %d) (filtered %d)",
			resp.Index, len(resp.Allocs), len(allocsResp.Allocs), len(filtered))

		// Update the query index.
		if resp.Index > req.MinQueryIndex {
			req.MinQueryIndex = resp.Index
		}

		// Push the updates.
		update := &allocUpdates{
			filtered:      filtered,
			pulled:        pulledAllocs,
			migrateTokens: resp.MigrateTokens,
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
		c.removeAlloc(remove)
	}

	// Update the existing allocations
	for _, update := range diff.updated {
		if err := c.updateAlloc(update.exist, update.updated); err != nil {
			c.logger.Printf("[ERR] client: failed to update alloc %q: %v",
				update.exist.ID, err)
		}
	}

	// Make room for new allocations before running
	if err := c.garbageCollector.MakeRoomFor(diff.added); err != nil {
		c.logger.Printf("[ERR] client: error making room for new allocations: %v", err)
	}

	// Start the new allocations
	for _, add := range diff.added {
		migrateToken := update.migrateTokens[add.ID]
		if err := c.addAlloc(add, migrateToken); err != nil {
			c.logger.Printf("[ERR] client: failed to add alloc '%s': %v",
				add.ID, err)
		}
	}

	// Trigger the GC once more now that new allocs are started that could
	// have caused thesholds to be exceeded
	c.garbageCollector.Trigger()
}

// removeAlloc is invoked when we should remove an allocation because it has
// been removed by the server.
func (c *Client) removeAlloc(alloc *structs.Allocation) {
	c.allocLock.Lock()
	ar, ok := c.allocs[alloc.ID]
	if !ok {
		c.allocLock.Unlock()
		c.logger.Printf("[WARN] client: missing context for alloc '%s'", alloc.ID)
		return
	}

	// Stop tracking alloc runner as it's been GC'd by the server
	delete(c.allocs, alloc.ID)
	c.allocLock.Unlock()

	// Ensure the GC has a reference and then collect. Collecting through the GC
	// applies rate limiting
	c.garbageCollector.MarkForCollection(ar)

	// GC immediately since the server has GC'd it
	go c.garbageCollector.Collect(alloc.ID)
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
func (c *Client) addAlloc(alloc *structs.Allocation, migrateToken string) error {
	// Check if we already have an alloc runner
	c.allocLock.Lock()
	defer c.allocLock.Unlock()
	if _, ok := c.allocs[alloc.ID]; ok {
		c.logger.Printf("[DEBUG]: client: dropping duplicate add allocation request: %q", alloc.ID)
		return nil
	}

	// get the previous alloc runner - if one exists - for the
	// blocking/migrating watcher
	var prevAR *AllocRunner
	if alloc.PreviousAllocation != "" {
		prevAR = c.allocs[alloc.PreviousAllocation]
	}

	c.configLock.RLock()
	prevAlloc := newAllocWatcher(alloc, prevAR, c, c.configCopy, c.logger, migrateToken)

	ar := NewAllocRunner(c.logger, c.configCopy, c.stateDB, c.updateAllocStatus, alloc, c.vaultClient, c.consulService, prevAlloc)
	c.configLock.RUnlock()

	// Store the alloc runner.
	c.allocs[alloc.ID] = ar

	if err := ar.SaveState(); err != nil {
		c.logger.Printf("[WARN] client: initial save state for alloc %q failed: %v", alloc.ID, err)
	}

	go ar.Run()
	return nil
}

// setupVaultClient creates an object to periodically renew tokens and secrets
// with vault.
func (c *Client) setupVaultClient() error {
	var err error
	c.vaultClient, err = vaultclient.NewVaultClient(c.config.VaultConfig, c.logger, c.deriveToken)
	if err != nil {
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
	// Check if the given task names actually exist in the allocation
	for _, taskName := range taskNames {
		found := false
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
		NodeID:   c.NodeID(),
		SecretID: c.secretNodeID(),
		AllocID:  alloc.ID,
		Tasks:    verifiedTasks,
		QueryOptions: structs.QueryOptions{
			Region:     c.Region(),
			AllowStale: false,
		},
	}

	// Derive the tokens
	var resp structs.DeriveVaultTokenResponse
	if err := c.RPC("Node.DeriveVaultToken", &req, &resp); err != nil {
		c.logger.Printf("[ERR] client.vault: DeriveVaultToken RPC failed: %v", err)
		return nil, fmt.Errorf("DeriveVaultToken RPC failed: %v", err)
	}
	if resp.Error != nil {
		c.logger.Printf("[ERR] client.vault: failed to derive vault tokens: %v", resp.Error)
		return nil, resp.Error
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

	dcs, err := c.consulCatalog.Datacenters()
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
		consulServices, _, err := c.consulCatalog.Service(serviceName, consul.ServiceTagRPC, consulOpts)
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

// emitStats collects host resource usage stats periodically
func (c *Client) emitStats() {
	// Assign labels directly before emitting stats so the information expected
	// is ready
	c.baseLabels = []metrics.Label{{Name: "node_id", Value: c.NodeID()}, {Name: "datacenter", Value: c.Datacenter()}}

	// Start collecting host stats right away and then keep collecting every
	// collection interval
	next := time.NewTimer(0)
	defer next.Stop()
	for {
		select {
		case <-next.C:
			err := c.hostStatsCollector.Collect()
			next.Reset(c.config.StatsCollectionInterval)
			if err != nil {
				c.logger.Printf("[WARN] client: error fetching host resource usage stats: %v", err)
				continue
			}

			// Publish Node metrics if operator has opted in
			if c.config.PublishNodeMetrics {
				c.emitHostStats()
			}

			c.emitClientMetrics()
		case <-c.shutdownCh:
			return
		}
	}
}

// setGaugeForMemoryStats proxies metrics for memory specific statistics
func (c *Client) setGaugeForMemoryStats(nodeID string, hStats *stats.HostStats) {
	if !c.config.DisableTaggedMetrics {
		metrics.SetGaugeWithLabels([]string{"client", "host", "memory", "total"}, float32(hStats.Memory.Total), c.baseLabels)
		metrics.SetGaugeWithLabels([]string{"client", "host", "memory", "available"}, float32(hStats.Memory.Available), c.baseLabels)
		metrics.SetGaugeWithLabels([]string{"client", "host", "memory", "used"}, float32(hStats.Memory.Used), c.baseLabels)
		metrics.SetGaugeWithLabels([]string{"client", "host", "memory", "free"}, float32(hStats.Memory.Free), c.baseLabels)
	}

	if c.config.BackwardsCompatibleMetrics {
		metrics.SetGauge([]string{"client", "host", "memory", nodeID, "total"}, float32(hStats.Memory.Total))
		metrics.SetGauge([]string{"client", "host", "memory", nodeID, "available"}, float32(hStats.Memory.Available))
		metrics.SetGauge([]string{"client", "host", "memory", nodeID, "used"}, float32(hStats.Memory.Used))
		metrics.SetGauge([]string{"client", "host", "memory", nodeID, "free"}, float32(hStats.Memory.Free))
	}
}

// setGaugeForCPUStats proxies metrics for CPU specific statistics
func (c *Client) setGaugeForCPUStats(nodeID string, hStats *stats.HostStats) {
	for _, cpu := range hStats.CPU {
		if !c.config.DisableTaggedMetrics {
			labels := append(c.baseLabels, metrics.Label{
				Name:  "cpu",
				Value: cpu.CPU,
			})

			metrics.SetGaugeWithLabels([]string{"client", "host", "cpu", "total"}, float32(cpu.Total), labels)
			metrics.SetGaugeWithLabels([]string{"client", "host", "cpu", "user"}, float32(cpu.User), labels)
			metrics.SetGaugeWithLabels([]string{"client", "host", "cpu", "idle"}, float32(cpu.Idle), labels)
			metrics.SetGaugeWithLabels([]string{"client", "host", "cpu", "system"}, float32(cpu.System), labels)
		}

		if c.config.BackwardsCompatibleMetrics {
			metrics.SetGauge([]string{"client", "host", "cpu", nodeID, cpu.CPU, "total"}, float32(cpu.Total))
			metrics.SetGauge([]string{"client", "host", "cpu", nodeID, cpu.CPU, "user"}, float32(cpu.User))
			metrics.SetGauge([]string{"client", "host", "cpu", nodeID, cpu.CPU, "idle"}, float32(cpu.Idle))
			metrics.SetGauge([]string{"client", "host", "cpu", nodeID, cpu.CPU, "system"}, float32(cpu.System))
		}
	}
}

// setGaugeForDiskStats proxies metrics for disk specific statistics
func (c *Client) setGaugeForDiskStats(nodeID string, hStats *stats.HostStats) {
	for _, disk := range hStats.DiskStats {
		if !c.config.DisableTaggedMetrics {
			labels := append(c.baseLabels, metrics.Label{
				Name:  "disk",
				Value: disk.Device,
			})

			metrics.SetGaugeWithLabels([]string{"client", "host", "disk", "size"}, float32(disk.Size), labels)
			metrics.SetGaugeWithLabels([]string{"client", "host", "disk", "used"}, float32(disk.Used), labels)
			metrics.SetGaugeWithLabels([]string{"client", "host", "disk", "available"}, float32(disk.Available), labels)
			metrics.SetGaugeWithLabels([]string{"client", "host", "disk", "used_percent"}, float32(disk.UsedPercent), labels)
			metrics.SetGaugeWithLabels([]string{"client", "host", "disk", "inodes_percent"}, float32(disk.InodesUsedPercent), labels)
		}

		if c.config.BackwardsCompatibleMetrics {
			metrics.SetGauge([]string{"client", "host", "disk", nodeID, disk.Device, "size"}, float32(disk.Size))
			metrics.SetGauge([]string{"client", "host", "disk", nodeID, disk.Device, "used"}, float32(disk.Used))
			metrics.SetGauge([]string{"client", "host", "disk", nodeID, disk.Device, "available"}, float32(disk.Available))
			metrics.SetGauge([]string{"client", "host", "disk", nodeID, disk.Device, "used_percent"}, float32(disk.UsedPercent))
			metrics.SetGauge([]string{"client", "host", "disk", nodeID, disk.Device, "inodes_percent"}, float32(disk.InodesUsedPercent))
		}
	}
}

// setGaugeForAllocationStats proxies metrics for allocation specific statistics
func (c *Client) setGaugeForAllocationStats(nodeID string) {
	c.configLock.RLock()
	node := c.configCopy.Node
	c.configLock.RUnlock()
	total := node.Resources
	res := node.Reserved
	allocated := c.getAllocatedResources(node)

	// Emit allocated
	if !c.config.DisableTaggedMetrics {
		metrics.SetGaugeWithLabels([]string{"client", "allocated", "memory"}, float32(allocated.MemoryMB), c.baseLabels)
		metrics.SetGaugeWithLabels([]string{"client", "allocated", "disk"}, float32(allocated.DiskMB), c.baseLabels)
		metrics.SetGaugeWithLabels([]string{"client", "allocated", "cpu"}, float32(allocated.CPU), c.baseLabels)
		metrics.SetGaugeWithLabels([]string{"client", "allocated", "iops"}, float32(allocated.IOPS), c.baseLabels)
	}

	if c.config.BackwardsCompatibleMetrics {
		metrics.SetGauge([]string{"client", "allocated", "memory", nodeID}, float32(allocated.MemoryMB))
		metrics.SetGauge([]string{"client", "allocated", "disk", nodeID}, float32(allocated.DiskMB))
		metrics.SetGauge([]string{"client", "allocated", "cpu", nodeID}, float32(allocated.CPU))
		metrics.SetGauge([]string{"client", "allocated", "iops", nodeID}, float32(allocated.IOPS))
	}

	for _, n := range allocated.Networks {
		if !c.config.DisableTaggedMetrics {
			labels := append(c.baseLabels, metrics.Label{
				Name:  "device",
				Value: n.Device,
			})
			metrics.SetGaugeWithLabels([]string{"client", "allocated", "network"}, float32(n.MBits), labels)
		}

		if c.config.BackwardsCompatibleMetrics {
			metrics.SetGauge([]string{"client", "allocated", "network", n.Device, nodeID}, float32(n.MBits))
		}
	}

	// Emit unallocated
	unallocatedMem := total.MemoryMB - res.MemoryMB - allocated.MemoryMB
	unallocatedDisk := total.DiskMB - res.DiskMB - allocated.DiskMB
	unallocatedCpu := total.CPU - res.CPU - allocated.CPU
	unallocatedIops := total.IOPS - res.IOPS - allocated.IOPS

	if !c.config.DisableTaggedMetrics {
		metrics.SetGaugeWithLabels([]string{"client", "unallocated", "memory"}, float32(unallocatedMem), c.baseLabels)
		metrics.SetGaugeWithLabels([]string{"client", "unallocated", "disk"}, float32(unallocatedDisk), c.baseLabels)
		metrics.SetGaugeWithLabels([]string{"client", "unallocated", "cpu"}, float32(unallocatedCpu), c.baseLabels)
		metrics.SetGaugeWithLabels([]string{"client", "unallocated", "iops"}, float32(unallocatedIops), c.baseLabels)
	}

	if c.config.BackwardsCompatibleMetrics {
		metrics.SetGauge([]string{"client", "unallocated", "memory", nodeID}, float32(unallocatedMem))
		metrics.SetGauge([]string{"client", "unallocated", "disk", nodeID}, float32(unallocatedDisk))
		metrics.SetGauge([]string{"client", "unallocated", "cpu", nodeID}, float32(unallocatedCpu))
		metrics.SetGauge([]string{"client", "unallocated", "iops", nodeID}, float32(unallocatedIops))
	}

	for _, n := range allocated.Networks {
		totalIdx := total.NetIndex(n)
		if totalIdx != -1 {
			continue
		}

		totalMbits := total.Networks[totalIdx].MBits
		unallocatedMbits := totalMbits - n.MBits

		if !c.config.DisableTaggedMetrics {
			labels := append(c.baseLabels, metrics.Label{
				Name:  "device",
				Value: n.Device,
			})
			metrics.SetGaugeWithLabels([]string{"client", "unallocated", "network"}, float32(unallocatedMbits), labels)
		}

		if c.config.BackwardsCompatibleMetrics {
			metrics.SetGauge([]string{"client", "unallocated", "network", n.Device, nodeID}, float32(unallocatedMbits))
		}
	}
}

// No lables are required so we emit with only a key/value syntax
func (c *Client) setGaugeForUptime(hStats *stats.HostStats) {
	if !c.config.DisableTaggedMetrics {
		metrics.SetGaugeWithLabels([]string{"uptime"}, float32(hStats.Uptime), c.baseLabels)
	}
	if c.config.BackwardsCompatibleMetrics {
		metrics.SetGauge([]string{"uptime"}, float32(hStats.Uptime))
	}
}

// emitHostStats pushes host resource usage stats to remote metrics collection sinks
func (c *Client) emitHostStats() {
	nodeID := c.NodeID()
	hStats := c.hostStatsCollector.Stats()

	c.setGaugeForMemoryStats(nodeID, hStats)
	c.setGaugeForUptime(hStats)
	c.setGaugeForCPUStats(nodeID, hStats)
	c.setGaugeForDiskStats(nodeID, hStats)
}

// emitClientMetrics emits lower volume client metrics
func (c *Client) emitClientMetrics() {
	nodeID := c.NodeID()

	c.setGaugeForAllocationStats(nodeID)

	// Emit allocation metrics
	blocked, migrating, pending, running, terminal := 0, 0, 0, 0, 0
	for _, ar := range c.getAllocRunners() {
		switch ar.Alloc().ClientStatus {
		case structs.AllocClientStatusPending:
			switch {
			case ar.IsWaiting():
				blocked++
			case ar.IsMigrating():
				migrating++
			default:
				pending++
			}
		case structs.AllocClientStatusRunning:
			running++
		case structs.AllocClientStatusComplete, structs.AllocClientStatusFailed:
			terminal++
		}
	}

	if !c.config.DisableTaggedMetrics {
		metrics.SetGaugeWithLabels([]string{"client", "allocations", "migrating"}, float32(migrating), c.baseLabels)
		metrics.SetGaugeWithLabels([]string{"client", "allocations", "blocked"}, float32(blocked), c.baseLabels)
		metrics.SetGaugeWithLabels([]string{"client", "allocations", "pending"}, float32(pending), c.baseLabels)
		metrics.SetGaugeWithLabels([]string{"client", "allocations", "running"}, float32(running), c.baseLabels)
		metrics.SetGaugeWithLabels([]string{"client", "allocations", "terminal"}, float32(terminal), c.baseLabels)
	}

	if c.config.BackwardsCompatibleMetrics {
		metrics.SetGauge([]string{"client", "allocations", "migrating", nodeID}, float32(migrating))
		metrics.SetGauge([]string{"client", "allocations", "blocked", nodeID}, float32(blocked))
		metrics.SetGauge([]string{"client", "allocations", "pending", nodeID}, float32(pending))
		metrics.SetGauge([]string{"client", "allocations", "running", nodeID}, float32(running))
		metrics.SetGauge([]string{"client", "allocations", "terminal", nodeID}, float32(terminal))
	}
}

func (c *Client) getAllocatedResources(selfNode *structs.Node) *structs.Resources {
	// Unfortunately the allocs only have IP so we need to match them to the
	// device
	cidrToDevice := make(map[*net.IPNet]string, len(selfNode.Resources.Networks))
	for _, n := range selfNode.Resources.Networks {
		_, ipnet, err := net.ParseCIDR(n.CIDR)
		if err != nil {
			continue
		}
		cidrToDevice[ipnet] = n.Device
	}

	// Sum the allocated resources
	allocs := c.allAllocs()
	var allocated structs.Resources
	allocatedDeviceMbits := make(map[string]int)
	for _, alloc := range allocs {
		if !alloc.TerminalStatus() {
			allocated.Add(alloc.Resources)
			for _, allocatedNetwork := range alloc.Resources.Networks {
				for cidr, dev := range cidrToDevice {
					ip := net.ParseIP(allocatedNetwork.IP)
					if cidr.Contains(ip) {
						allocatedDeviceMbits[dev] += allocatedNetwork.MBits
						break
					}
				}
			}
		}
	}

	// Clear the networks
	allocated.Networks = nil
	for dev, speed := range allocatedDeviceMbits {
		net := &structs.NetworkResource{
			Device: dev,
			MBits:  speed,
		}
		allocated.Networks = append(allocated.Networks, net)
	}

	return &allocated
}

// allAllocs returns all the allocations managed by the client
func (c *Client) allAllocs() map[string]*structs.Allocation {
	ars := c.getAllocRunners()
	allocs := make(map[string]*structs.Allocation, len(ars))
	for _, ar := range c.getAllocRunners() {
		a := ar.Alloc()
		allocs[a.ID] = a
	}
	return allocs
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
