package client

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"sort"
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
	"github.com/hashicorp/nomad/client/servers"
	"github.com/hashicorp/nomad/client/stats"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pool"
	hstats "github.com/hashicorp/nomad/helper/stats"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	nconfig "github.com/hashicorp/nomad/nomad/structs/config"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/shirou/gopsutil/host"
)

const (
	// clientRPCCache controls how long we keep an idle connection
	// open to a server
	clientRPCCache = 5 * time.Minute

	// clientMaxStreams controls how many idle streams we keep
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

	connPool *pool.ConnPool

	// tlsWrap is used to wrap outbound connections using TLS. It should be
	// accessed using the lock.
	tlsWrap     tlsutil.RegionWrapper
	tlsWrapLock sync.RWMutex

	// servers is the list of nomad servers
	servers *servers.Manager

	// heartbeat related times for tracking how often to heartbeat
	lastHeartbeat   time.Time
	heartbeatTTL    time.Duration
	haveHeartbeated bool
	heartbeatLock   sync.Mutex

	// triggerDiscoveryCh triggers Consul discovery; see triggerDiscovery
	triggerDiscoveryCh chan struct{}

	// triggerNodeUpdate triggers the client to mark the Node as changed and
	// update it.
	triggerNodeUpdate chan struct{}

	// triggerEmitNodeEvent sends an event and triggers the client to update the
	// server for the node event
	triggerEmitNodeEvent chan *structs.NodeEvent

	// rpcRetryCh is closed when there an event such as server discovery or a
	// successful RPC occurring happens such that a retry should happen. Access
	// should only occur via the getter method
	rpcRetryCh   chan struct{}
	rpcRetryLock sync.Mutex

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

	// rpcServer is used to serve RPCs by the local agent.
	rpcServer     *rpc.Server
	endpoints     rpcEndpoints
	streamingRpcs *structs.StreamingRpcRegistry

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
		config:               cfg,
		consulCatalog:        consulCatalog,
		consulService:        consulService,
		start:                time.Now(),
		connPool:             pool.NewPool(cfg.LogOutput, clientRPCCache, clientMaxStreams, tlsWrap),
		tlsWrap:              tlsWrap,
		streamingRpcs:        structs.NewStreamingRpcRegistry(),
		logger:               logger,
		allocs:               make(map[string]*AllocRunner),
		allocUpdates:         make(chan *structs.Allocation, 64),
		shutdownCh:           make(chan struct{}),
		triggerDiscoveryCh:   make(chan struct{}),
		triggerNodeUpdate:    make(chan struct{}, 8),
		triggerEmitNodeEvent: make(chan *structs.NodeEvent, 8),
	}

	// Initialize the server manager
	c.servers = servers.New(c.logger, c.shutdownCh, c)

	// Initialize the client
	if err := c.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize client: %v", err)
	}

	// Setup the clients RPC server
	c.setupClientRpc()

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

	// Store the config copy before restoring state but after it has been
	// initialized.
	c.configLock.Lock()
	c.configCopy = c.config.Copy()
	c.configLock.Unlock()

	fingerprintManager := NewFingerprintManager(c.GetConfig, c.configCopy.Node,
		c.shutdownCh, c.updateNodeFromFingerprint, c.updateNodeFromDriver,
		c.logger)

	// Fingerprint the node and scan for drivers
	if err := fingerprintManager.Run(); err != nil {
		return nil, fmt.Errorf("fingerprinting failed: %v", err)
	}

	// Setup the reserved resources
	c.reservePorts()

	// Set the preconfigured list of static servers
	c.configLock.RLock()
	if len(c.configCopy.Servers) > 0 {
		if err := c.setServersImpl(c.configCopy.Servers, true); err != nil {
			logger.Printf("[WARN] client: None of the configured servers are valid: %v", err)
		}
	}
	c.configLock.RUnlock()

	// Setup Consul discovery if enabled
	if c.configCopy.ConsulConfig.ClientAutoJoin != nil && *c.configCopy.ConsulConfig.ClientAutoJoin {
		go c.consulDiscovery()
		if c.servers.NumServers() == 0 {
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
		// Otherwise make a temp directory to use.
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
		// Otherwise make a temp directory to use.
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

// reloadTLSConnections allows a client to reload its TLS configuration on the
// fly
func (c *Client) reloadTLSConnections(newConfig *nconfig.TLSConfig) error {
	var tlsWrap tlsutil.RegionWrapper
	if newConfig != nil && newConfig.EnableRPC {
		tw, err := tlsutil.NewTLSConfiguration(newConfig)
		if err != nil {
			return err
		}

		twWrap, err := tw.OutgoingTLSWrapper()
		if err != nil {
			return err
		}
		tlsWrap = twWrap
	}

	// Store the new tls wrapper.
	c.tlsWrapLock.Lock()
	c.tlsWrap = tlsWrap
	c.tlsWrapLock.Unlock()

	// Keep the client configuration up to date as we use configuration values to
	// decide on what type of connections to accept
	c.configLock.Lock()
	c.config.TLSConfig = newConfig
	c.configLock.Unlock()

	c.connPool.ReloadTLS(tlsWrap)

	return nil
}

// Reload allows a client to reload its configuration on the fly
func (c *Client) Reload(newConfig *config.Config) error {
	return c.reloadTLSConnections(newConfig.TLSConfig)
}

// Leave is used to prepare the client to leave the cluster
func (c *Client) Leave() error {
	// TODO
	return nil
}

// GetConfig returns the config of the client
func (c *Client) GetConfig() *config.Config {
	c.configLock.Lock()
	defer c.configLock.Unlock()
	return c.configCopy
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

// Stats is used to return statistics for debugging and insight
// for various sub-systems
func (c *Client) Stats() map[string]map[string]string {
	c.heartbeatLock.Lock()
	defer c.heartbeatLock.Unlock()
	stats := map[string]map[string]string{
		"client": {
			"node_id":         c.NodeID(),
			"known_servers":   strings.Join(c.GetServers(), ","),
			"num_allocations": strconv.Itoa(c.NumAllocs()),
			"last_heartbeat":  fmt.Sprintf("%v", time.Since(c.lastHeartbeat)),
			"heartbeat_ttl":   fmt.Sprintf("%v", c.heartbeatTTL),
		},
		"runtime": hstats.RuntimeStats(),
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
		return nil, structs.NewErrUnknownAllocation(allocID)
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

	return structs.CompareMigrateToken(allocID, c.secretNodeID(), migrateToken)
}

// GetAllocFS returns the AllocFS interface for the alloc dir of an allocation
func (c *Client) GetAllocFS(allocID string) (allocdir.AllocDirFS, error) {
	c.allocLock.RLock()
	defer c.allocLock.RUnlock()

	ar, ok := c.allocs[allocID]
	if !ok {
		return nil, structs.NewErrUnknownAllocation(allocID)
	}
	return ar.GetAllocDir(), nil
}

// GetClientAlloc returns the allocation from the client
func (c *Client) GetClientAlloc(allocID string) (*structs.Allocation, error) {
	all := c.allAllocs()
	alloc, ok := all[allocID]
	if !ok {
		return nil, structs.NewErrUnknownAllocation(allocID)
	}
	return alloc, nil
}

// GetServers returns the list of nomad servers this client is aware of.
func (c *Client) GetServers() []string {
	endpoints := c.servers.GetServers()
	res := make([]string, len(endpoints))
	for i := range endpoints {
		res[i] = endpoints[i].String()
	}
	sort.Strings(res)
	return res
}

// SetServers sets a new list of nomad servers to connect to. As long as one
// server is resolvable no error is returned.
func (c *Client) SetServers(in []string) error {
	return c.setServersImpl(in, false)
}

// setServersImpl sets a new list of nomad servers to connect to. If force is
// set, we add the server to the internal serverlist even if the server could not
// be pinged. An error is returned if no endpoints were valid when non-forcing.
//
// Force should be used when setting the servers from the initial configuration
// since the server may be starting up in parallel and initial pings may fail.
func (c *Client) setServersImpl(in []string, force bool) error {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var merr multierror.Error

	endpoints := make([]*servers.Server, 0, len(in))
	wg.Add(len(in))

	for _, s := range in {
		go func(srv string) {
			defer wg.Done()
			addr, err := resolveServer(srv)
			if err != nil {
				c.logger.Printf("[DEBUG] client: ignoring server %s due to resolution error: %v", srv, err)
				merr.Errors = append(merr.Errors, err)
				return
			}

			// Try to ping to check if it is a real server
			if err := c.Ping(addr); err != nil {
				merr.Errors = append(merr.Errors, fmt.Errorf("Server at address %s failed ping: %v", addr, err))

				// If we are forcing the setting of the servers, inject it to
				// the serverlist even if we can't ping immediately.
				if !force {
					return
				}
			}

			mu.Lock()
			endpoints = append(endpoints, &servers.Server{Addr: addr})
			mu.Unlock()
		}(s)
	}

	wg.Wait()

	// Only return errors if no servers are valid
	if len(endpoints) == 0 {
		if len(merr.Errors) > 0 {
			return merr.ErrorOrNil()
		}
		return noServersErr
	}

	c.servers.SetServers(endpoints)
	return nil
}

// restoreState is used to restore our state from the data dir
func (c *Client) restoreState() error {
	if c.config.DevMode {
		return nil
	}

	// COMPAT: Remove in 0.7.0
	// 0.6.0 transitioned from individual state files to a single bolt-db.
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
		ar := NewAllocRunner(c.logger, c.configCopy.Copy(), c.stateDB, c.updateAllocStatus, alloc, c.vaultClient, c.consulService, watcher)
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
	if node.Drivers == nil {
		node.Drivers = make(map[string]*structs.DriverInfo)
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

	// Make the changes available to the config copy.
	c.configCopy = c.config.Copy()
}

// updateNodeFromFingerprint updates the node with the result of
// fingerprinting the node from the diff that was created
func (c *Client) updateNodeFromFingerprint(response *cstructs.FingerprintResponse) *structs.Node {
	c.configLock.Lock()
	defer c.configLock.Unlock()

	nodeHasChanged := false

	for name, newVal := range response.Attributes {
		oldVal := c.config.Node.Attributes[name]
		if oldVal == newVal {
			continue
		}

		nodeHasChanged = true
		if newVal == "" {
			delete(c.config.Node.Attributes, name)
		} else {
			c.config.Node.Attributes[name] = newVal
		}
	}

	// update node links and resources from the diff created from
	// fingerprinting
	for name, newVal := range response.Links {
		oldVal := c.config.Node.Links[name]
		if oldVal == newVal {
			continue
		}

		nodeHasChanged = true
		if newVal == "" {
			delete(c.config.Node.Links, name)
		} else {
			c.config.Node.Links[name] = newVal
		}
	}

	if response.Resources != nil && !resourcesAreEqual(c.config.Node.Resources, response.Resources) {
		nodeHasChanged = true
		c.config.Node.Resources.Merge(response.Resources)
	}

	if nodeHasChanged {
		c.updateNodeLocked()
	}

	return c.configCopy.Node
}

// updateNodeFromDriver receives either a fingerprint of the driver or its
// health and merges this into a single DriverInfo object
func (c *Client) updateNodeFromDriver(name string, fingerprint, health *structs.DriverInfo) *structs.Node {
	c.configLock.Lock()
	defer c.configLock.Unlock()

	var hasChanged bool

	hadDriver := c.config.Node.Drivers[name] != nil
	if fingerprint != nil {
		if !hadDriver {
			// If the driver info has not yet been set, do that here
			hasChanged = true
			c.config.Node.Drivers[name] = fingerprint
			for attrName, newVal := range fingerprint.Attributes {
				c.config.Node.Attributes[attrName] = newVal
			}
		} else {
			// The driver info has already been set, fix it up
			if c.config.Node.Drivers[name].Detected != fingerprint.Detected {
				hasChanged = true
				c.config.Node.Drivers[name].Detected = fingerprint.Detected
			}

			for attrName, newVal := range fingerprint.Attributes {
				oldVal := c.config.Node.Drivers[name].Attributes[attrName]
				if oldVal == newVal {
					continue
				}

				hasChanged = true
				if newVal == "" {
					delete(c.config.Node.Attributes, attrName)
				} else {
					c.config.Node.Attributes[attrName] = newVal
				}
			}
		}

		// COMPAT Remove in Nomad 0.10
		// We maintain the driver enabled attribute until all drivers expose
		// their attributes as DriverInfo
		driverName := fmt.Sprintf("driver.%s", name)
		if fingerprint.Detected {
			c.config.Node.Attributes[driverName] = "1"
		} else {
			delete(c.config.Node.Attributes, driverName)
		}
	}

	if health != nil {
		if !hadDriver {
			hasChanged = true
			if info, ok := c.config.Node.Drivers[name]; !ok {
				c.config.Node.Drivers[name] = health
			} else {
				info.MergeHealthCheck(health)
			}
		} else {
			oldVal := c.config.Node.Drivers[name]
			if health.HealthCheckEquals(oldVal) {
				// Make sure we accurately reflect the last time a health check has been
				// performed for the driver.
				oldVal.UpdateTime = health.UpdateTime
			} else {
				hasChanged = true

				// Only emit an event if the health status has changed after node
				// initial startup (the health description will not get populated until
				// a health check has run; the initial status is equal to whether the
				// node is detected or not).
				if health.Healthy != oldVal.Healthy && health.HealthDescription != "" {
					event := &structs.NodeEvent{
						Subsystem: "Driver",
						Message:   health.HealthDescription,
						Timestamp: time.Now(),
						Details:   map[string]string{"driver": name},
					}
					c.triggerNodeEvent(event)
				}

				// Update the node with the latest information
				c.config.Node.Drivers[name].MergeHealthCheck(health)
			}
		}
	}

	if hasChanged {
		c.config.Node.Drivers[name].UpdateTime = time.Now()
		c.updateNodeLocked()
	}

	return c.configCopy.Node
}

// resourcesAreEqual is a temporary function to compare whether resources are
// equal. We can use this until we change fingerprinters to set pointers on a
// return type.
func resourcesAreEqual(first, second *structs.Resources) bool {
	if first.CPU != second.CPU {
		return false
	}
	if first.MemoryMB != second.MemoryMB {
		return false
	}
	if first.DiskMB != second.DiskMB {
		return false
	}
	if first.IOPS != second.IOPS {
		return false
	}
	if len(first.Networks) != len(second.Networks) {
		return false
	}
	for i, e := range first.Networks {
		if len(second.Networks) < i {
			return false
		}
		f := second.Networks[i]
		if !e.Equals(f) {
			return false
		}
	}
	return true
}

// retryIntv calculates a retry interval value given the base
func (c *Client) retryIntv(base time.Duration) time.Duration {
	if c.config.DevMode {
		return devModeRetryIntv
	}
	return base + lib.RandomStagger(base)
}

// registerAndHeartbeat is a long lived goroutine used to register the client
// and then start heartbeating to the server.
func (c *Client) registerAndHeartbeat() {
	// Register the node
	c.retryRegisterNode()

	// Start watching changes for node changes
	go c.watchNodeUpdates()

	// Start watching for emitting node events
	go c.watchNodeEvents()

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
		case <-c.rpcRetryWatcher():
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
				intv := c.getHeartbeatRetryIntv(err)
				c.logger.Printf("[ERR] client: heartbeating failed. Retrying in %v: %v", intv, err)
				heartbeat = time.After(intv)

				// If heartbeating fails, trigger Consul discovery
				c.triggerDiscovery()
			}
		} else {
			c.heartbeatLock.Lock()
			heartbeat = time.After(c.heartbeatTTL)
			c.heartbeatLock.Unlock()
		}
	}
}

// getHeartbeatRetryIntv is used to retrieve the time to wait before attempting
// another heartbeat.
func (c *Client) getHeartbeatRetryIntv(err error) time.Duration {
	if c.config.DevMode {
		return devModeRetryIntv
	}

	// Collect the useful heartbeat info
	c.heartbeatLock.Lock()
	haveHeartbeated := c.haveHeartbeated
	last := c.lastHeartbeat
	ttl := c.heartbeatTTL
	c.heartbeatLock.Unlock()

	// If we haven't even successfully heartbeated once or there is no leader
	// treat it as a registration. In the case that there is a leadership loss,
	// we will have our heartbeat timer reset to a much larger threshold, so
	// do not put unnecessary pressure on the new leader.
	if !haveHeartbeated || err == structs.ErrNoLeader {
		return c.retryIntv(registerRetryIntv)
	}

	// Determine how much time we have left to heartbeat
	left := last.Add(ttl).Sub(time.Now())

	// Logic for retrying is:
	// * Do not retry faster than once a second
	// * Do not retry less that once every 30 seconds
	// * If we have missed the heartbeat by more than 30 seconds, start to use
	// the absolute time since we do not want to retry indefinitely
	switch {
	case left < -30*time.Second:
		// Make left the absolute value so we delay and jitter properly.
		left *= -1
	case left < 0:
		return time.Second + lib.RandomStagger(time.Second)
	default:
	}

	stagger := lib.RandomStagger(left)
	switch {
	case stagger < time.Second:
		return time.Second + lib.RandomStagger(time.Second)
	case stagger > 30*time.Second:
		return 25*time.Second + lib.RandomStagger(5*time.Second)
	default:
		return stagger
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

// submitNodeEvents is used to submit a client-side node event. Examples of
// these kinds of events include when a driver moves from healthy to unhealthy
// (and vice versa)
func (c *Client) submitNodeEvents(events []*structs.NodeEvent) error {
	nodeID := c.NodeID()
	nodeEvents := map[string][]*structs.NodeEvent{
		nodeID: events,
	}
	req := structs.EmitNodeEventsRequest{
		NodeEvents:   nodeEvents,
		WriteRequest: structs.WriteRequest{Region: c.Region()},
	}
	var resp structs.EmitNodeEventsResponse
	if err := c.RPC("Node.EmitEvents", &req, &resp); err != nil {
		return fmt.Errorf("Emitting node events failed: %v", err)
	}
	return nil
}

// watchNodeEvents is a handler which receives node events and on a interval
// and submits them in batch format to the server
func (c *Client) watchNodeEvents() {
	// batchEvents stores events that have yet to be published
	var batchEvents []*structs.NodeEvent

	// Create and drain the timer
	timer := time.NewTimer(0)
	timer.Stop()
	select {
	case <-timer.C:
	default:
	}
	defer timer.Stop()

	for {
		select {
		case event := <-c.triggerEmitNodeEvent:
			if l := len(batchEvents); l <= structs.MaxRetainedNodeEvents {
				batchEvents = append(batchEvents, event)
			} else {
				// Drop the oldest event
				c.logger.Printf("[WARN] client: dropping node event: %v", batchEvents[0])
				batchEvents = append(batchEvents[1:], event)
			}
			timer.Reset(c.retryIntv(nodeUpdateRetryIntv))
		case <-timer.C:
			if err := c.submitNodeEvents(batchEvents); err != nil {
				c.logger.Printf("[ERR] client: submitting node events failed: %v", err)
				timer.Reset(c.retryIntv(nodeUpdateRetryIntv))
			} else {
				// Reset the events since we successfully sent them.
				batchEvents = []*structs.NodeEvent{}
			}
		case <-c.shutdownCh:
			return
		}
	}
}

// triggerNodeEvent triggers a emit node event
func (c *Client) triggerNodeEvent(nodeEvent *structs.NodeEvent) {
	select {
	case c.triggerEmitNodeEvent <- nodeEvent:
		// emit node event goroutine was released to execute
	default:
		// emit node event goroutine was already running
	}
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
		case <-c.rpcRetryWatcher():
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
	c.config.Node.Status = structs.NodeStatusReady
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
	start := time.Now()
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
	end := time.Now()

	if len(resp.EvalIDs) != 0 {
		c.logger.Printf("[DEBUG] client: %d evaluations triggered by node update", len(resp.EvalIDs))
	}

	// Update the last heartbeat and the new TTL, capturing the old values
	c.heartbeatLock.Lock()
	last := c.lastHeartbeat
	oldTTL := c.heartbeatTTL
	haveHeartbeated := c.haveHeartbeated
	c.lastHeartbeat = time.Now()
	c.heartbeatTTL = resp.HeartbeatTTL
	c.haveHeartbeated = true
	c.heartbeatLock.Unlock()
	c.logger.Printf("[TRACE] client: next heartbeat in %v", resp.HeartbeatTTL)

	if resp.Index != 0 {
		c.logger.Printf("[DEBUG] client: state updated to %s", req.Status)

		// We have potentially missed our TTL log how delayed we were
		if haveHeartbeated {
			c.logger.Printf("[WARN] client: heartbeat missed (request took %v). Heartbeat TTL was %v and heartbeated after %v",
				end.Sub(start), oldTTL, time.Since(last))
		}
	}

	// Update the number of nodes in the cluster so we can adjust our server
	// rebalance rate.
	c.servers.SetNumNodes(resp.NumNodes)

	// Convert []*NodeServerInfo to []*servers.Server
	nomadServers := make([]*servers.Server, 0, len(resp.Servers))
	for _, s := range resp.Servers {
		addr, err := resolveServer(s.RPCAdvertiseAddr)
		if err != nil {
			c.logger.Printf("[WARN] client: ignoring invalid server %q: %v", s.RPCAdvertiseAddr, err)
			continue
		}
		e := &servers.Server{DC: s.Datacenter, Addr: addr}
		nomadServers = append(nomadServers, e)
	}
	if len(nomadServers) == 0 {
		return fmt.Errorf("heartbeat response returned no valid servers")
	}
	c.servers.SetServers(nomadServers)

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
			case <-c.rpcRetryWatcher():
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
				case <-c.rpcRetryWatcher():
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

// updateNode updates the Node copy and triggers the client to send the updated
// Node to the server. This should be done while the caller holds the
// configLock lock.
func (c *Client) updateNodeLocked() {
	// Update the config copy.
	node := c.config.Node.Copy()
	c.configCopy.Node = node

	select {
	case c.triggerNodeUpdate <- struct{}{}:
		// Node update goroutine was released to execute
	default:
		// Node update goroutine was already running
	}
}

// watchNodeUpdates blocks until it is edge triggered. Once triggered,
// it will update the client node copy and re-register the node.
func (c *Client) watchNodeUpdates() {
	var hasChanged bool
	timer := time.NewTimer(c.retryIntv(nodeUpdateRetryIntv))
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			c.logger.Printf("[DEBUG] client: state changed, updating node and re-registering.")
			c.retryRegisterNode()
			hasChanged = false
		case <-c.triggerNodeUpdate:
			if hasChanged {
				continue
			}
			hasChanged = true
			timer.Reset(c.retryIntv(nodeUpdateRetryIntv))
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
	// have caused thresholds to be exceeded
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

	// Copy the config since the node can be swapped out as it is being updated.
	// The long term fix is to pass in the config and node separately and then
	// we don't have to do a copy.
	ar := NewAllocRunner(c.logger, c.configCopy.Copy(), c.stateDB, c.updateAllocStatus, alloc, c.vaultClient, c.consulService, prevAlloc)
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
			return nil, fmt.Errorf("task %q not found in the allocation", taskName)
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
		return nil, structs.NewWrappedServerError(resp.Error)
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
			if structs.VaultUnrecoverableError.MatchString(err.Error()) {
				return nil, err
			}

			// The error is recoverable
			return nil, structs.NewRecoverableError(
				fmt.Errorf("failed to unwrap the token for task %q: %v", taskName, err), true)
		}

		// Validate the response
		var validationErr error
		if unwrapResp == nil {
			validationErr = fmt.Errorf("Vault returned nil secret when unwrapping")
		} else if unwrapResp.Auth == nil {
			validationErr = fmt.Errorf("Vault returned unwrap secret with nil Auth. Secret warnings: %v", unwrapResp.Warnings)
		} else if unwrapResp.Auth.ClientToken == "" {
			validationErr = fmt.Errorf("Vault returned unwrap secret with empty Auth.ClientToken. Secret warnings: %v", unwrapResp.Warnings)
		}
		if validationErr != nil {
			c.logger.Printf("[WARN] client.vault: failed to unwrap token: %v", err)
			return nil, structs.NewRecoverableError(validationErr, true)
		}

		// Append the unwrapped token to the return value
		unwrappedTokens[taskName] = unwrapResp.Auth.ClientToken
	}

	return unwrappedTokens, nil
}

// triggerDiscovery causes a Consul discovery to begin (if one hasn't already)
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
	var nomadServers servers.Servers
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
				srv := &servers.Server{Addr: addr}
				nomadServers = append(nomadServers, srv)
			}
			if len(nomadServers) > 0 {
				break DISCOLOOP
			}
		}
	}
	if len(nomadServers) == 0 {
		if len(mErr.Errors) > 0 {
			return mErr.ErrorOrNil()
		}
		return fmt.Errorf("no Nomad Servers advertising service %q in Consul datacenters: %+q", serviceName, dcs)
	}

	c.logger.Printf("[INFO] client.consul: discovered following Servers: %s", nomadServers)

	// Fire the retry trigger if we have updated the set of servers.
	if c.servers.SetServers(nomadServers) {
		// Start rebalancing
		c.servers.RebalanceServers()

		// Notify waiting rpc calls. If a goroutine just failed an RPC call and
		// isn't receiving on this chan yet they'll still retry eventually.
		// This is a shortcircuit for the longer retry intervals.
		c.fireRpcRetryWatcher()
	}

	return nil
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

// No labels are required so we emit with only a key/value syntax
func (c *Client) setGaugeForUptime(hStats *stats.HostStats) {
	if !c.config.DisableTaggedMetrics {
		metrics.SetGaugeWithLabels([]string{"client", "uptime"}, float32(hStats.Uptime), c.baseLabels)
	}
	if c.config.BackwardsCompatibleMetrics {
		metrics.SetGauge([]string{"client", "uptime"}, float32(hStats.Uptime))
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
