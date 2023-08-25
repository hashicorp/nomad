// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"errors"
	"fmt"
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
	consulapi "github.com/hashicorp/consul/api"
	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	arstate "github.com/hashicorp/nomad/client/allocrunner/state"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/getter"
	"github.com/hashicorp/nomad/client/allocwatcher"
	"github.com/hashicorp/nomad/client/config"
	consulApi "github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/devicemanager"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/client/hoststats"
	cinterfaces "github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/lib/proclib"
	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	"github.com/hashicorp/nomad/client/servers"
	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/client/serviceregistration/checks/checkstore"
	"github.com/hashicorp/nomad/client/serviceregistration/nsd"
	"github.com/hashicorp/nomad/client/serviceregistration/wrapper"
	"github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/envoy"
	"github.com/hashicorp/nomad/helper/goruntime"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/pool"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	nconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/plugins/csi"
	"github.com/hashicorp/nomad/plugins/device"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/shirou/gopsutil/v3/host"
	"golang.org/x/exp/maps"
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

	// noServerRetryIntv is the retry interval used when client has not
	// connected to server yet
	noServerRetryIntv = time.Second

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

	// defaultConnectLogLevel is the log level set in the node meta by default
	// to be used by Consul Connect sidecar tasks.
	defaultConnectLogLevel = "info"

	// defaultConnectProxyConcurrency is the default number of worker threads the
	// connect sidecar should be configured to use.
	//
	// https://www.envoyproxy.io/docs/envoy/latest/operations/cli#cmdoption-concurrency
	defaultConnectProxyConcurrency = "1"
)

var (
	// grace period to allow for batch fingerprint processing
	batchFirstFingerprintsProcessingGrace = batchFirstFingerprintsTimeout + 5*time.Second
)

// ClientStatsReporter exposes all the APIs related to resource usage of a Nomad
// Client
type ClientStatsReporter interface {
	// GetAllocStats returns the AllocStatsReporter for the passed allocation.
	// If it does not exist an error is reported.
	GetAllocStats(allocID string) (interfaces.AllocStatsReporter, error)

	// LatestHostStats returns the latest resource usage stats for the host
	LatestHostStats() *hoststats.HostStats
}

// Client is used to implement the client interaction with Nomad. Clients
// are expected to register as a schedule-able node to the servers, and to
// run allocations as determined by the servers.
type Client struct {
	start time.Time

	// stateDB is used to efficiently store client state.
	stateDB state.StateDB

	// config must only be accessed with lock held. To update the config, use the
	// Client.UpdateConfig() helper. If you need more fine grained control use
	// the following pattern:
	//
	// 	c.configLock.Lock()
	// 	newConfig := c.config.Copy()
	// 	// <mutate newConfig>
	// 	c.config = newConfig
	// 	c.configLock.Unlock()
	configLock  sync.Mutex
	config      *config.Config
	metaDynamic map[string]*string // dynamic node metadata

	// metaStatic are the Node's static metadata set via the agent configuration
	// and defaults during client initialization. Since this map is never updated
	// at runtime it may be accessed outside of locks.
	metaStatic map[string]string

	logger    hclog.InterceptLogger
	rpcLogger hclog.Logger

	connPool *pool.ConnPool

	// tlsWrap is used to wrap outbound connections using TLS. It should be
	// accessed using the lock.
	tlsWrap     tlsutil.RegionWrapper
	tlsWrapLock sync.RWMutex

	// servers is the list of nomad servers
	servers *servers.Manager

	// heartbeat related times for tracking how often to heartbeat
	heartbeatTTL    time.Duration
	haveHeartbeated bool
	heartbeatLock   sync.Mutex
	heartbeatStop   *heartbeatStop

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
	allocs    map[string]interfaces.AllocRunner
	allocLock sync.RWMutex

	// allocrunnerFactory is the function called to create new allocrunners
	allocrunnerFactory config.AllocRunnerFactory

	// invalidAllocs is a map that tracks allocations that failed because
	// the client couldn't initialize alloc or task runners for it. This can
	// happen due to driver errors
	invalidAllocs     map[string]struct{}
	invalidAllocsLock sync.Mutex

	// pendingUpdates stores allocations that need to be synced to the server.
	pendingUpdates *pendingClientUpdates

	// consulService is the Consul handler implementation for managing services
	// and checks.
	consulService serviceregistration.Handler

	// nomadService is the Nomad handler implementation for managing service
	// registrations.
	nomadService serviceregistration.Handler

	// checkStore is used to store group and task checks and their current pass/fail
	// status.
	checkStore checkstore.Shim

	// serviceRegWrapper wraps the consulService and nomadService
	// implementations so that the alloc and task runner service hooks can call
	// this without needing to identify which backend provider should be used.
	serviceRegWrapper *wrapper.HandlerWrapper

	// consulProxies is Nomad's custom Consul client for looking up supported
	// envoy versions
	consulProxies consulApi.SupportedProxiesAPI

	// consulCatalog is the subset of Consul's Catalog API Nomad uses.
	consulCatalog consul.CatalogAPI

	// HostStatsCollector collects host resource usage stats
	hostStatsCollector *hoststats.HostStatsCollector

	// shutdown is true when the Client has been shutdown. Must hold
	// shutdownLock to access.
	shutdown bool

	// shutdownCh is closed to signal the Client is shutting down.
	shutdownCh chan struct{}

	shutdownLock sync.Mutex

	// shutdownGroup are goroutines that exit when shutdownCh is closed.
	// Shutdown() blocks on Wait() after closing shutdownCh.
	shutdownGroup group

	// tokensClient is Nomad Client's custom Consul client for requesting Consul
	// Service Identity tokens through Nomad Server.
	tokensClient consulApi.ServiceIdentityAPI

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

	// fingerprintManager is the FingerprintManager registered by the client
	fingerprintManager *FingerprintManager

	// pluginManagers is the set of PluginManagers registered by the client
	pluginManagers *pluginmanager.PluginGroup

	// csimanager is responsible for managing csi plugins.
	csimanager csimanager.Manager

	// devicemanger is responsible for managing device plugins.
	devicemanager devicemanager.Manager

	// drivermanager is responsible for managing driver plugins
	drivermanager drivermanager.Manager

	// baseLabels are used when emitting tagged metrics. All client metrics will
	// have these tags, and optionally more.
	baseLabels []metrics.Label

	// batchNodeUpdates is used to batch initial updates to the node
	batchNodeUpdates *batchNodeUpdates

	// fpInitialized chan is closed when the first batch of fingerprints are
	// applied to the node
	fpInitialized chan struct{}

	// registeredCh is closed when Node.Register has successfully run once.
	registeredCh   chan struct{}
	registeredOnce sync.Once

	// serversContactedCh is closed when GetClientAllocs and runAllocs have
	// successfully run once.
	serversContactedCh   chan struct{}
	serversContactedOnce sync.Once

	// dynamicRegistry provides access to plugins that are dynamically registered
	// with a nomad client. Currently only used for CSI.
	dynamicRegistry dynamicplugins.Registry

	// EnterpriseClient is used to set and check enterprise features for clients
	EnterpriseClient *EnterpriseClient

	// getter is an interface for retrieving artifacts.
	getter cinterfaces.ArtifactGetter

	// wranglers is used to keep track of processes and manage their interaction
	// with drivers and stuff
	wranglers *proclib.Wranglers

	// topology represents the system memory / cpu topology detected via
	// fingerprinting
	topology *numalib.Topology

	// widmgr retrieves workload identities
	widmgr *widmgr.WIDMgr
}

var (
	// noServersErr is returned by the RPC method when the client has no
	// configured servers. This is used to trigger Consul discovery if
	// enabled.
	noServersErr = errors.New("no servers")
)

// NewClient is used to create a new client from the given configuration.
// `rpcs` is a map of RPC names to RPC structs that, if non-nil, will be
// registered via https://golang.org/pkg/net/rpc/#Server.RegisterName in place
// of the client's normal RPC handlers. This allows server tests to override
// the behavior of the client.
func NewClient(cfg *config.Config, consulCatalog consul.CatalogAPI, consulProxies consulApi.SupportedProxiesAPI, consulService serviceregistration.Handler, rpcs map[string]interface{}) (*Client, error) {
	// Create the tls wrapper
	var tlsWrap tlsutil.RegionWrapper
	if cfg.TLSConfig.EnableRPC {
		tw, err := tlsutil.NewTLSConfiguration(cfg.TLSConfig, true, true)
		if err != nil {
			return nil, err
		}
		tlsWrap, err = tw.OutgoingTLSWrapper()
		if err != nil {
			return nil, err
		}
	}

	if cfg.StateDBFactory == nil {
		cfg.StateDBFactory = state.GetStateDBFactory(cfg.DevMode)
	}

	// Create the logger
	logger := cfg.Logger.ResetNamedIntercept("client")

	// Create the client
	c := &Client{
		config:               cfg,
		consulCatalog:        consulCatalog,
		consulProxies:        consulProxies,
		consulService:        consulService,
		start:                time.Now(),
		connPool:             pool.NewPool(logger, clientRPCCache, clientMaxStreams, tlsWrap),
		tlsWrap:              tlsWrap,
		streamingRpcs:        structs.NewStreamingRpcRegistry(),
		logger:               logger,
		rpcLogger:            logger.Named("rpc"),
		allocs:               make(map[string]interfaces.AllocRunner),
		pendingUpdates:       newPendingClientUpdates(),
		shutdownCh:           make(chan struct{}),
		triggerDiscoveryCh:   make(chan struct{}),
		triggerNodeUpdate:    make(chan struct{}, 8),
		triggerEmitNodeEvent: make(chan *structs.NodeEvent, 8),
		fpInitialized:        make(chan struct{}),
		invalidAllocs:        make(map[string]struct{}),
		serversContactedCh:   make(chan struct{}),
		serversContactedOnce: sync.Once{},
		registeredCh:         make(chan struct{}),
		registeredOnce:       sync.Once{},
		getter:               getter.New(cfg.Artifact, logger),
		EnterpriseClient:     newEnterpriseClient(logger),
		allocrunnerFactory:   cfg.AllocRunnerFactory,
	}

	// we can't have this set in the default Config because of import cycles
	if c.allocrunnerFactory == nil {
		c.allocrunnerFactory = allocrunner.NewAllocRunner
	}

	c.batchNodeUpdates = newBatchNodeUpdates(
		c.updateNodeFromDriver,
		c.updateNodeFromDevices,
		c.updateNodeFromCSI,
	)

	// Initialize the server manager
	c.servers = servers.New(c.logger, c.shutdownCh, c)

	// Start server manager rebalancing go routine
	go c.servers.Start()

	// initialize the client
	if err := c.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize client: %v", err)
	}

	// initialize the dynamic registry (needs to happen after init)
	c.dynamicRegistry =
		dynamicplugins.NewRegistry(c.stateDB, map[string]dynamicplugins.PluginDispenser{
			dynamicplugins.PluginTypeCSIController: func(info *dynamicplugins.PluginInfo) (interface{}, error) {
				return csi.NewClient(info.ConnectionInfo.SocketPath, logger.Named("csi_client").With("plugin.name", info.Name, "plugin.type", "controller")), nil
			},
			dynamicplugins.PluginTypeCSINode: func(info *dynamicplugins.PluginInfo) (interface{}, error) {
				return csi.NewClient(info.ConnectionInfo.SocketPath, logger.Named("csi_client").With("plugin.name", info.Name, "plugin.type", "client")), nil
			},
		})

	// Setup the clients RPC server
	c.setupClientRpc(rpcs)

	// Initialize the ACL state
	c.clientACLResolver.init()

	// Setup the node
	if err := c.setupNode(); err != nil {
		return nil, fmt.Errorf("node setup failed: %v", err)
	}

	// Add workload identity manager after node secret has been generated/loaded
	c.widmgr = widmgr.New(widmgr.Config{
		NodeSecret: c.secretNodeID(),
		Region:     cfg.Region,
		RPC:        c,
	})

	c.fingerprintManager = NewFingerprintManager(
		cfg.PluginSingletonLoader,
		c.GetConfig,
		cfg.Node,
		c.shutdownCh,
		c.updateNodeFromFingerprint,
		c.logger,
	)
	c.pluginManagers = pluginmanager.New(c.logger)

	// Fingerprint the node and scan for drivers
	if ir, err := c.fingerprintManager.Run(); err != nil {
		return nil, fmt.Errorf("fingerprinting failed: %v", err)
	} else {
		c.topology = numalib.NoImpl(ir.Topology)
	}

	// Create the process wranglers
	wranglers := proclib.New(&proclib.Configs{
		Logger: c.logger.Named("proclib"),
	})
	c.wranglers = wranglers

	// Build the allow/denylists of drivers.
	// COMPAT(1.0) uses inclusive language. white/blacklist are there for backward compatible reasons only.
	allowlistDrivers := cfg.ReadStringListToMap("driver.allowlist", "driver.whitelist")
	blocklistDrivers := cfg.ReadStringListToMap("driver.denylist", "driver.blacklist")

	// Setup the csi manager
	csiConfig := &csimanager.Config{
		Logger:                c.logger,
		DynamicRegistry:       c.dynamicRegistry,
		UpdateNodeCSIInfoFunc: c.batchNodeUpdates.updateNodeFromCSI,
		TriggerNodeEvent:      c.triggerNodeEvent,
	}
	csiManager := csimanager.New(csiConfig)
	c.csimanager = csiManager
	c.pluginManagers.RegisterAndRun(csiManager.PluginManager())

	// Setup the driver manager
	driverConfig := &drivermanager.Config{
		Logger:              c.logger,
		Loader:              cfg.PluginSingletonLoader,
		PluginConfig:        cfg.NomadPluginConfig(),
		Updater:             c.batchNodeUpdates.updateNodeFromDriver,
		EventHandlerFactory: c.GetTaskEventHandler,
		State:               c.stateDB,
		AllowedDrivers:      allowlistDrivers,
		BlockedDrivers:      blocklistDrivers,
	}
	drvManager := drivermanager.New(driverConfig)
	c.drivermanager = drvManager
	c.pluginManagers.RegisterAndRun(drvManager)

	// Setup the device manager
	devConfig := &devicemanager.Config{
		Logger:        c.logger,
		Loader:        cfg.PluginSingletonLoader,
		PluginConfig:  cfg.NomadPluginConfig(),
		Updater:       c.batchNodeUpdates.updateNodeFromDevices,
		StatsInterval: cfg.StatsCollectionInterval,
		State:         c.stateDB,
	}
	devManager := devicemanager.New(devConfig)
	c.devicemanager = devManager
	c.pluginManagers.RegisterAndRun(devManager)

	// Set up the service registration wrapper using the Consul and Nomad
	// implementations. The Nomad implementation is only ever used on the
	// client, so we do that here rather than within the agent.
	c.setupNomadServiceRegistrationHandler()
	c.serviceRegWrapper = wrapper.NewHandlerWrapper(c.logger, c.consulService, c.nomadService)

	// Batching of initial fingerprints is done to reduce the number of node
	// updates sent to the server on startup.
	go c.batchFirstFingerprints()

	// create heartbeatStop. We go after the first attempt to connect to the server, so
	// that our grace period for connection goes for the full time
	c.heartbeatStop = newHeartbeatStop(c.getAllocRunner, batchFirstFingerprintsTimeout, logger, c.shutdownCh)

	// Watch for disconnection, and heartbeatStopAllocs configured to have a maximum
	// lifetime when out of touch with the server
	go c.heartbeatStop.watch()

	// Add the stats collector
	statsCollector := hoststats.NewHostStatsCollector(c.logger, c.topology, c.GetConfig().AllocDir, c.devicemanager.AllStats)
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
	c.garbageCollector = NewAllocGarbageCollector(c.logger, statsCollector, c, gcConfig)
	go c.garbageCollector.Run()

	// Set the preconfigured list of static servers
	if len(cfg.Servers) > 0 {
		if _, err := c.setServersImpl(cfg.Servers, true); err != nil {
			logger.Warn("none of the configured servers are valid", "error", err)
		}
	}

	// Setup Consul discovery if enabled
	if cfg.ConsulConfig.ClientAutoJoin != nil && *cfg.ConsulConfig.ClientAutoJoin {
		c.shutdownGroup.Go(c.consulDiscovery)
		if c.servers.NumServers() == 0 {
			// No configured servers; trigger discovery manually
			c.triggerDiscoveryCh <- struct{}{}
		}
	}

	if err := c.setupConsulTokenClient(); err != nil {
		return nil, fmt.Errorf("failed to setup consul tokens client: %w", err)
	}

	// Setup the vault client for token and secret renewals
	if err := c.setupVaultClient(); err != nil {
		return nil, fmt.Errorf("failed to setup vault client: %v", err)
	}

	// wait until drivers are healthy before restoring or registering with servers
	select {
	case <-c.fpInitialized:
	case <-time.After(batchFirstFingerprintsProcessingGrace):
		logger.Warn("batch fingerprint operation timed out; proceeding to register with fingerprinted plugins so far")
	}

	// Register and then start heartbeating to the servers.
	c.shutdownGroup.Go(c.registerAndHeartbeat)

	// Restore the state
	if err := c.restoreState(); err != nil {
		logger.Error("failed to restore state", "error", err)
		logger.Error("Nomad is unable to start due to corrupt state. "+
			"The safest way to proceed is to manually stop running task processes "+
			"and remove Nomad's state and alloc directories before "+
			"restarting. Lost allocations will be rescheduled.",
			"state_dir", cfg.StateDir, "alloc_dir", cfg.AllocDir)
		logger.Error("Corrupt state is often caused by a bug. Please " +
			"report as much information as possible to " +
			"https://github.com/hashicorp/nomad/issues")
		return nil, fmt.Errorf("failed to restore state")
	}

	// Begin periodic snapshotting of state.
	c.shutdownGroup.Go(c.periodicSnapshot)

	// Begin syncing allocations to the server
	c.shutdownGroup.Go(c.allocSync)

	// Start the client! Don't use the shutdownGroup as run handles
	// shutdowns manually to prevent updates from being applied during
	// shutdown.
	go c.run()

	// Start collecting stats
	c.shutdownGroup.Go(c.emitStats)

	c.logger.Info("started client", "node_id", c.NodeID())
	return c, nil
}

// Ready returns a chan that is closed when the client is fully initialized
func (c *Client) Ready() <-chan struct{} {
	return c.serversContactedCh
}

// init is used to initialize the client and perform any setup
// needed before we begin starting its various components.
func (c *Client) init() error {
	// Ensure the state dir exists if we have one
	conf := c.GetConfig()
	if conf.StateDir != "" {
		if err := os.MkdirAll(conf.StateDir, 0700); err != nil {
			return fmt.Errorf("failed creating state dir: %s", err)
		}

	} else {
		// Otherwise make a temp directory to use.
		p, err := os.MkdirTemp("", "NomadClient")
		if err != nil {
			return fmt.Errorf("failed creating temporary directory for the StateDir: %v", err)
		}

		p, err = filepath.EvalSymlinks(p)
		if err != nil {
			return fmt.Errorf("failed to find temporary directory for the StateDir: %v", err)
		}

		conf = c.UpdateConfig(func(c *config.Config) {
			c.StateDir = p
		})
	}
	c.logger.Info("using state directory", "state_dir", conf.StateDir)

	// Open the state database
	db, err := conf.StateDBFactory(c.logger, conf.StateDir)
	if err != nil {
		return fmt.Errorf("failed to open state database: %v", err)
	}

	// Upgrade the state database
	if err := db.Upgrade(); err != nil {
		// Upgrade only returns an error on critical persistence
		// failures in which an operator should intervene before the
		// node is accessible. Upgrade drops and logs corrupt state it
		// encounters, so failing to start the agent should be extremely
		// rare.
		return fmt.Errorf("failed to upgrade state database: %v", err)
	}

	c.stateDB = db

	// Ensure the alloc dir exists if we have one
	if conf.AllocDir != "" {
		if err := os.MkdirAll(conf.AllocDir, 0711); err != nil {
			return fmt.Errorf("failed creating alloc dir: %s", err)
		}
	} else {
		// Otherwise make a temp directory to use.
		p, err := os.MkdirTemp("", "NomadClient")
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

		conf = c.UpdateConfig(func(c *config.Config) {
			c.AllocDir = p
		})
	}

	c.logger.Info("using alloc directory", "alloc_dir", conf.AllocDir)

	reserved := "<none>"
	if conf.Node != nil && conf.Node.ReservedResources != nil {
		// Node should always be non-nil due to initialization in the
		// agent package, but don't risk a panic just for a long line.
		reserved = conf.Node.ReservedResources.Networks.ReservedHostPorts
	}
	c.logger.Info("using dynamic ports",
		"min", conf.MinDynamicPort,
		"max", conf.MaxDynamicPort,
		"reserved", reserved,
	)

	// setup the nsd check store
	c.checkStore = checkstore.NewStore(c.logger, c.stateDB)

	return nil
}

// reloadTLSConnections allows a client to reload its TLS configuration on the fly
func (c *Client) reloadTLSConnections(newConfig *nconfig.TLSConfig) error {
	var tlsWrap tlsutil.RegionWrapper
	if newConfig != nil && newConfig.EnableRPC {
		tw, err := tlsutil.NewTLSConfiguration(newConfig, true, true)
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
	c.UpdateConfig(func(c *config.Config) {
		c.TLSConfig = newConfig
	})

	c.connPool.ReloadTLS(tlsWrap)

	return nil
}

// Reload allows a client to reload parts of its configuration on the fly
func (c *Client) Reload(newConfig *config.Config) error {
	existing := c.GetConfig()
	shouldReloadTLS, err := tlsutil.ShouldReloadRPCConnections(existing.TLSConfig, newConfig.TLSConfig)
	if err != nil {
		c.logger.Error("error parsing TLS configuration", "error", err)
		return err
	}

	if shouldReloadTLS {
		if err := c.reloadTLSConnections(newConfig.TLSConfig); err != nil {
			return err
		}
	}

	c.fingerprintManager.Reload()

	return nil
}

// Leave is used to prepare the client to leave the cluster
func (c *Client) Leave() error {
	if c.GetConfig().DevMode {
		return nil
	}

	// In normal mode optionally drain the node
	return c.DrainSelf()
}

// GetConfig returns the config of the client. Do *not* mutate without first
// calling Copy().
func (c *Client) GetConfig() *config.Config {
	c.configLock.Lock()
	defer c.configLock.Unlock()
	return c.config
}

// UpdateConfig allows mutating the configuration. The updated configuration is
// returned.
func (c *Client) UpdateConfig(cb func(*config.Config)) *config.Config {
	c.configLock.Lock()
	defer c.configLock.Unlock()

	// Create a copy of the active config
	newConfig := c.config.Copy()

	// Pass the copy to the supplied callback for mutation
	cb(newConfig)

	// Set new config struct
	c.config = newConfig

	return newConfig
}

// UpdateNode allows mutating just the Node portion of the client
// configuration. The updated Node is returned.
//
// This is similar to UpdateConfig but avoids deep copying the entier Config
// struct when only the Node is updated.
func (c *Client) UpdateNode(cb func(*structs.Node)) *structs.Node {
	c.configLock.Lock()
	defer c.configLock.Unlock()

	// Create a new copy of Node for updating
	newNode := c.config.Node.Copy()

	// newNode is now a fresh unshared copy, mutate away!
	cb(newNode)

	// Shallow copy config before mutating Node pointer which might have
	// concurrent readers
	newConfig := *c.config
	newConfig.Node = newNode
	c.config = &newConfig

	return newNode
}

// Datacenter returns the datacenter for the given client
func (c *Client) Datacenter() string {
	return c.GetConfig().Node.Datacenter
}

// Region returns the region for the given client
func (c *Client) Region() string {
	return c.GetConfig().Region
}

// NodeID returns the node ID for the given client
func (c *Client) NodeID() string {
	return c.GetConfig().Node.ID
}

// secretNodeID returns the secret node ID for the given client
func (c *Client) secretNodeID() string {
	return c.GetConfig().Node.SecretID
}

// Shutdown is used to tear down the client
func (c *Client) Shutdown() error {
	c.shutdownLock.Lock()
	defer c.shutdownLock.Unlock()

	if c.shutdown {
		c.logger.Info("already shutdown")
		return nil
	}
	c.logger.Info("shutting down")

	// Stop renewing tokens and secrets
	if c.vaultClient != nil {
		c.vaultClient.Stop()
	}

	// Stop Garbage collector
	c.garbageCollector.Stop()

	arGroup := group{}
	if c.GetConfig().DevMode {
		// In DevMode destroy all the running allocations.
		for _, ar := range c.getAllocRunners() {
			ar.Destroy()
			arGroup.AddCh(ar.DestroyCh())
		}
	} else {
		// In normal mode call shutdown
		for _, ar := range c.getAllocRunners() {
			ar.Shutdown()
			arGroup.AddCh(ar.ShutdownCh())
		}
	}
	arGroup.Wait()

	// Assert the implementation, so we can trigger the shutdown call. This is
	// the only place this occurs, so it's OK to store the interface rather
	// than the implementation.
	if h, ok := c.nomadService.(*nsd.ServiceRegistrationHandler); ok {
		h.Shutdown()
	}

	// Shutdown the plugin managers
	c.pluginManagers.Shutdown()

	c.shutdown = true
	close(c.shutdownCh)

	// Must close connection pool to unblock alloc watcher
	c.connPool.Shutdown()

	// Wait for goroutines to stop
	c.shutdownGroup.Wait()

	// One final save state
	c.saveState()
	return c.stateDB.Close()
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
			"last_heartbeat":  fmt.Sprintf("%v", time.Since(c.lastHeartbeat())),
			"heartbeat_ttl":   fmt.Sprintf("%v", c.heartbeatTTL),
		},
		"runtime": goruntime.RuntimeStats(),
	}
	return stats
}

// GetAlloc returns an allocation or an error.
func (c *Client) GetAlloc(allocID string) (*structs.Allocation, error) {
	ar, err := c.getAllocRunner(allocID)
	if err != nil {
		return nil, err
	}

	return ar.Alloc(), nil
}

// SignalAllocation sends a signal to the tasks within an allocation.
// If the provided task is empty, then every allocation will be signalled.
// If a task is provided, then only an exactly matching task will be signalled.
func (c *Client) SignalAllocation(allocID, task, signal string) error {
	ar, err := c.getAllocRunner(allocID)
	if err != nil {
		return err
	}

	return ar.Signal(task, signal)
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

func (c *Client) RestartAllocation(allocID, taskName string, allTasks bool) error {
	if allTasks && taskName != "" {
		return fmt.Errorf("task name cannot be set when restarting all tasks")
	}

	ar, err := c.getAllocRunner(allocID)
	if err != nil {
		return err
	}

	if taskName != "" {
		event := structs.NewTaskEvent(structs.TaskRestartSignal).
			SetRestartReason("User requested task to restart")
		return ar.RestartTask(taskName, event)
	}

	if allTasks {
		event := structs.NewTaskEvent(structs.TaskRestartSignal).
			SetRestartReason("User requested all tasks to restart")
		return ar.RestartAll(event)
	}

	event := structs.NewTaskEvent(structs.TaskRestartSignal).
		SetRestartReason("User requested running tasks to restart")
	return ar.RestartRunning(event)
}

// Node returns the locally registered node
func (c *Client) Node() *structs.Node {
	return c.GetConfig().Node
}

// getAllocRunner returns an AllocRunner or an UnknownAllocation error if the
// client has no runner for the given alloc ID.
func (c *Client) getAllocRunner(allocID string) (interfaces.AllocRunner, error) {
	c.allocLock.RLock()
	defer c.allocLock.RUnlock()

	ar, ok := c.allocs[allocID]
	if !ok {
		return nil, structs.NewErrUnknownAllocation(allocID)
	}

	return ar, nil
}

// StatsReporter exposes the various APIs related resource usage of a Nomad
// client
func (c *Client) StatsReporter() ClientStatsReporter {
	return c
}

func (c *Client) GetAllocStats(allocID string) (interfaces.AllocStatsReporter, error) {
	ar, err := c.getAllocRunner(allocID)
	if err != nil {
		return nil, err
	}
	return ar.StatsReporter(), nil
}

// LatestHostStats returns all the stats related to a Nomad client.
func (c *Client) LatestHostStats() *hoststats.HostStats {
	return c.hostStatsCollector.Stats()
}

func (c *Client) LatestDeviceResourceStats(devices []*structs.AllocatedDeviceResource) []*device.DeviceGroupStats {
	return c.computeAllocatedDeviceGroupStats(devices, c.LatestHostStats().DeviceStats)
}

func (c *Client) computeAllocatedDeviceGroupStats(devices []*structs.AllocatedDeviceResource, hostDeviceGroupStats []*device.DeviceGroupStats) []*device.DeviceGroupStats {
	// basic optimization for the usual case
	if len(devices) == 0 || len(hostDeviceGroupStats) == 0 {
		return nil
	}

	// Build an index of allocated devices
	adIdx := map[structs.DeviceIdTuple][]string{}

	total := 0
	for _, ds := range devices {
		adIdx[*ds.ID()] = ds.DeviceIDs
		total += len(ds.DeviceIDs)
	}

	// Collect allocated device stats from host stats
	result := make([]*device.DeviceGroupStats, 0, len(adIdx))

	for _, dg := range hostDeviceGroupStats {
		k := structs.DeviceIdTuple{
			Vendor: dg.Vendor,
			Type:   dg.Type,
			Name:   dg.Name,
		}

		allocatedDeviceIDs, ok := adIdx[k]
		if !ok {
			continue
		}

		rdgStats := &device.DeviceGroupStats{
			Vendor:        dg.Vendor,
			Type:          dg.Type,
			Name:          dg.Name,
			InstanceStats: map[string]*device.DeviceStats{},
		}

		for _, adID := range allocatedDeviceIDs {
			deviceStats, ok := dg.InstanceStats[adID]
			if !ok || deviceStats == nil {
				c.logger.Warn("device not found in stats", "device_id", adID, "device_group_id", k)
				continue
			}

			rdgStats.InstanceStats[adID] = deviceStats
		}
		result = append(result, rdgStats)
	}

	return result
}

// ValidateMigrateToken verifies that a token is for a specific client and
// allocation, and has been created by a trusted party that has privileged
// knowledge of the client's secret identifier
func (c *Client) ValidateMigrateToken(allocID, migrateToken string) bool {
	conf := c.GetConfig()
	if !conf.ACLEnabled {
		return true
	}

	return structs.CompareMigrateToken(allocID, conf.Node.SecretID, migrateToken)
}

// GetAllocFS returns the AllocFS interface for the alloc dir of an allocation
func (c *Client) GetAllocFS(allocID string) (allocdir.AllocDirFS, error) {
	ar, err := c.getAllocRunner(allocID)
	if err != nil {
		return nil, err
	}
	return ar.GetAllocDir(), nil
}

// GetAllocState returns a copy of an allocation's state on this client. It
// returns either an AllocState or an unknown allocation error.
func (c *Client) GetAllocState(allocID string) (*arstate.State, error) {
	ar, err := c.getAllocRunner(allocID)
	if err != nil {
		return nil, err
	}

	return ar.AllocState(), nil
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
func (c *Client) SetServers(in []string) (int, error) {
	return c.setServersImpl(in, false)
}

// setServersImpl sets a new list of nomad servers to connect to. If force is
// set, we add the server to the internal serverlist even if the server could not
// be pinged. An error is returned if no endpoints were valid when non-forcing.
//
// Force should be used when setting the servers from the initial configuration
// since the server may be starting up in parallel and initial pings may fail.
func (c *Client) setServersImpl(in []string, force bool) (int, error) {
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
				mu.Lock()
				c.logger.Debug("ignoring server due to resolution error", "error", err, "server", srv)
				merr.Errors = append(merr.Errors, err)
				mu.Unlock()
				return
			}

			// Try to ping to check if it is a real server
			if err := c.Ping(addr); err != nil {
				mu.Lock()
				merr.Errors = append(merr.Errors, fmt.Errorf("Server at address %s failed ping: %v", addr, err))
				mu.Unlock()

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
			return 0, merr.ErrorOrNil()
		}
		return 0, noServersErr
	}

	c.servers.SetServers(endpoints)
	return len(endpoints), nil
}

// restoreState is used to restore our state from the data dir
// If there are errors restoring a specific allocation it is marked
// as failed whenever possible.
func (c *Client) restoreState() error {
	conf := c.GetConfig()
	if conf.DevMode {
		return nil
	}

	//XXX REMOVED! make a note in backward compat / upgrading doc
	// COMPAT: Remove in 0.7.0
	// 0.6.0 transitioned from individual state files to a single bolt-db.
	// The upgrade path is to:
	// Check if old state exists
	//   If so, restore from that and delete old state
	// Restore using state database

	// Restore allocations
	allocs, allocErrs, err := c.stateDB.GetAllAllocations()
	if err != nil {
		return err
	}

	for allocID, err := range allocErrs {
		c.logger.Error("error restoring alloc", "error", err, "alloc_id", allocID)
		//TODO Cleanup
		// Try to clean up alloc dir
		// Remove boltdb entries?
		// Send to server with clientstatus=failed
	}

	// Load each alloc back
	for _, alloc := range allocs {

		// COMPAT(0.12): remove once upgrading from 0.9.5 is no longer supported
		// See hasLocalState for details.  Skipping suspicious allocs
		// now.  If allocs should be run, they will be started when the client
		// gets allocs from servers.
		if !c.hasLocalState(alloc) {
			c.logger.Warn("found an alloc without any local state, skipping restore", "alloc_id", alloc.ID)
			continue
		}

		//XXX On Restore we give up on watching previous allocs because
		//    we need the local AllocRunners initialized first. We could
		//    add a second loop to initialize just the alloc watcher.
		prevAllocWatcher := allocwatcher.NoopPrevAlloc{}
		prevAllocMigrator := allocwatcher.NoopPrevAlloc{}

		arConf := &config.AllocRunnerConfig{
			Alloc:               alloc,
			Logger:              c.logger,
			ClientConfig:        conf,
			StateDB:             c.stateDB,
			StateUpdater:        c,
			DeviceStatsReporter: c,
			Consul:              c.consulService,
			ConsulSI:            c.tokensClient,
			ConsulProxies:       c.consulProxies,
			Vault:               c.vaultClient,
			PrevAllocWatcher:    prevAllocWatcher,
			PrevAllocMigrator:   prevAllocMigrator,
			DynamicRegistry:     c.dynamicRegistry,
			CSIManager:          c.csimanager,
			DeviceManager:       c.devicemanager,
			DriverManager:       c.drivermanager,
			ServersContactedCh:  c.serversContactedCh,
			ServiceRegWrapper:   c.serviceRegWrapper,
			CheckStore:          c.checkStore,
			RPCClient:           c,
			Getter:              c.getter,
			Wranglers:           c.wranglers,
		}

		ar, err := c.allocrunnerFactory(arConf)
		if err != nil {
			c.logger.Error("error running alloc", "error", err, "alloc_id", alloc.ID)
			c.handleInvalidAllocs(alloc, err)
			continue
		}

		// Restore state
		if err := ar.Restore(); err != nil {
			c.logger.Error("error restoring alloc", "error", err, "alloc_id", alloc.ID)
			// Override the status of the alloc to failed
			ar.SetClientStatus(structs.AllocClientStatusFailed)
			// Destroy the alloc runner since this is a failed restore
			ar.Destroy()
			continue
		}

		allocState, err := c.stateDB.GetAcknowledgedState(alloc.ID)
		if err != nil {
			c.logger.Error("error restoring last acknowledged alloc state, will update again",
				err, "alloc_id", alloc.ID)
		} else {
			ar.AcknowledgeState(allocState)
		}

		// Maybe mark the alloc for halt on missing server heartbeats
		if c.heartbeatStop.shouldStop(alloc) {
			err = c.heartbeatStop.stopAlloc(alloc.ID)
			if err != nil {
				c.logger.Error("error stopping alloc", "error", err, "alloc_id", alloc.ID)
			}
			continue
		}

		//XXX is this locking necessary?
		c.allocLock.Lock()
		c.allocs[alloc.ID] = ar
		c.allocLock.Unlock()

		c.heartbeatStop.allocHook(alloc)
	}

	// All allocs restored successfully, run them!
	c.allocLock.Lock()
	for _, ar := range c.allocs {
		go ar.Run()
	}
	c.allocLock.Unlock()
	return nil
}

// hasLocalState returns true if we have any other associated state
// with alloc beyond the task itself
//
// Useful for detecting if a potentially completed alloc got resurrected
// after AR was destroyed.  In such cases, re-running the alloc lead to
// unexpected reruns and may lead to process and task exhaustion on node.
//
// The heuristic used here is an alloc is suspect if we see no other information
// and no other task/status info is found.
//
// Also, an alloc without any client state will not be restored correctly; there will
// be no tasks processes to reattach to, etc.  In such cases, client should
// wait until it gets allocs from server to launch them.
//
// See:
//   - https://github.com/hashicorp/nomad/pull/6207
//   - https://github.com/hashicorp/nomad/issues/5984
//
// COMPAT(0.12): remove once upgrading from 0.9.5 is no longer supported
func (c *Client) hasLocalState(alloc *structs.Allocation) bool {
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		// corrupt alloc?!
		return false
	}

	for _, task := range tg.Tasks {
		ls, tr, _ := c.stateDB.GetTaskRunnerState(alloc.ID, task.Name)
		if ls != nil || tr != nil {
			return true
		}
	}

	return false
}

func (c *Client) handleInvalidAllocs(alloc *structs.Allocation, err error) {
	c.invalidAllocsLock.Lock()
	c.invalidAllocs[alloc.ID] = struct{}{}
	c.invalidAllocsLock.Unlock()

	// Mark alloc as failed so server can handle this
	failed := makeFailedAlloc(alloc, err)
	c.pendingUpdates.add(failed)
}

// saveState is used to snapshot our state into the data dir.
func (c *Client) saveState() error {
	var wg sync.WaitGroup
	var l sync.Mutex
	var mErr multierror.Error
	runners := c.getAllocRunners()
	wg.Add(len(runners))

	for id, ar := range runners {
		go func(id string, ar interfaces.AllocRunner) {
			err := ar.PersistState()
			if err != nil {
				c.logger.Error("error saving alloc state", "error", err, "alloc_id", id)
				l.Lock()
				_ = multierror.Append(&mErr, err)
				l.Unlock()
			}
			wg.Done()
		}(id, ar)
	}

	wg.Wait()
	return mErr.ErrorOrNil()
}

// getAllocRunners returns a snapshot of the current set of alloc runners.
func (c *Client) getAllocRunners() map[string]interfaces.AllocRunner {
	c.allocLock.RLock()
	defer c.allocLock.RUnlock()
	runners := make(map[string]interfaces.AllocRunner, len(c.allocs))
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

// ensureNodeID restores, or generates if necessary, a unique node ID and
// SecretID.  The node ID is, if available, a persistent unique ID.  The secret
// ID is a high-entropy random UUID.
func ensureNodeID(conf *config.Config) (id, secret string, err error) {
	var hostID string
	hostInfo, err := host.Info()
	if !conf.NoHostUUID && err == nil {
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
	if conf.DevMode {
		return hostID, uuid.Generate(), nil
	}

	// Attempt to read existing ID
	idPath := filepath.Join(conf.StateDir, "client-id")
	idBuf, err := os.ReadFile(idPath)
	if err != nil && !os.IsNotExist(err) {
		return "", "", err
	}

	// Attempt to read existing secret ID
	secretPath := filepath.Join(conf.StateDir, "secret-id")
	secretBuf, err := os.ReadFile(secretPath)
	if err != nil && !os.IsNotExist(err) {
		return "", "", err
	}

	// Use existing ID if any
	if len(idBuf) != 0 {
		id = strings.ToLower(string(idBuf))
	} else {
		id = hostID

		// Persist the ID
		if err := os.WriteFile(idPath, []byte(id), 0700); err != nil {
			return "", "", err
		}
	}

	if len(secretBuf) != 0 {
		secret = string(secretBuf)
	} else {
		// Generate new ID
		secret = uuid.Generate()

		// Persist the ID
		if err := os.WriteFile(secretPath, []byte(secret), 0700); err != nil {
			return "", "", err
		}
	}

	return id, secret, nil
}

// setupNode is used to setup the initial node
func (c *Client) setupNode() error {
	c.configLock.Lock()
	defer c.configLock.Unlock()

	newConfig := c.config.Copy()
	node := newConfig.Node
	if node == nil {
		node = &structs.Node{}
		newConfig.Node = node
	}

	// Generate an ID and secret for the node
	id, secretID, err := ensureNodeID(newConfig)
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
	if node.CSIControllerPlugins == nil {
		node.CSIControllerPlugins = make(map[string]*structs.CSIInfo)
	}
	if node.CSINodePlugins == nil {
		node.CSINodePlugins = make(map[string]*structs.CSIInfo)
	}
	if node.Meta == nil {
		node.Meta = make(map[string]string)
	}
	if node.NodeResources == nil {
		node.NodeResources = &structs.NodeResources{}
		node.NodeResources.MinDynamicPort = newConfig.MinDynamicPort
		node.NodeResources.MaxDynamicPort = newConfig.MaxDynamicPort
		node.NodeResources.Cpu = newConfig.Node.NodeResources.Cpu
	}
	if node.ReservedResources == nil {
		node.ReservedResources = &structs.NodeReservedResources{}
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
	node.CgroupParent = newConfig.CgroupParent
	if node.HostVolumes == nil {
		if l := len(newConfig.HostVolumes); l != 0 {
			node.HostVolumes = make(map[string]*structs.ClientHostVolumeConfig, l)
			for k, v := range newConfig.HostVolumes {
				if _, err := os.Stat(v.Path); err != nil {
					return fmt.Errorf("failed to validate volume %s, err: %v", v.Name, err)
				}
				node.HostVolumes[k] = v.Copy()
			}
		}
	}
	if node.HostNetworks == nil {
		if l := len(newConfig.HostNetworks); l != 0 {
			node.HostNetworks = make(map[string]*structs.ClientHostNetworkConfig, l)
			for k, v := range newConfig.HostNetworks {
				node.HostNetworks[k] = v.Copy()
			}
		}
	}

	if node.Name == "" {
		node.Name = node.ID
	}
	node.Status = structs.NodeStatusInit

	// Setup default static meta
	if _, ok := node.Meta[envoy.SidecarMetaParam]; !ok {
		node.Meta[envoy.SidecarMetaParam] = envoy.ImageFormat
	}
	if _, ok := node.Meta[envoy.GatewayMetaParam]; !ok {
		node.Meta[envoy.GatewayMetaParam] = envoy.ImageFormat
	}
	if _, ok := node.Meta["connect.log_level"]; !ok {
		node.Meta["connect.log_level"] = defaultConnectLogLevel
	}
	if _, ok := node.Meta["connect.proxy_concurrency"]; !ok {
		node.Meta["connect.proxy_concurrency"] = defaultConnectProxyConcurrency
	}

	// Since node.Meta will get dynamic metadata merged in, save static metadata
	// here.
	c.metaStatic = maps.Clone(node.Meta)

	// Merge dynamic node metadata
	c.metaDynamic, err = c.stateDB.GetNodeMeta()
	if err != nil {
		return fmt.Errorf("error reading dynamic node metadata: %w", err)
	}

	if c.metaDynamic == nil {
		c.metaDynamic = map[string]*string{}
	}

	for dk, dv := range c.metaDynamic {
		if dv == nil {
			_, ok := node.Meta[dk]
			if ok {
				// Unset static node metadata
				delete(node.Meta, dk)
			} else {
				// Forget dynamic node metadata tombstone as there's no
				// static value to erase.
				delete(c.metaDynamic, dk)
			}
			continue
		}

		node.Meta[dk] = *dv
	}

	// Write back dynamic node metadata as tombstones may have been removed
	// above
	if err := c.stateDB.PutNodeMeta(c.metaDynamic); err != nil {
		return fmt.Errorf("error syncing dynamic node metadata: %w", err)
	}

	c.config = newConfig
	return nil
}

// updateNodeFromFingerprint updates the node with the result of
// fingerprinting the node from the diff that was created
func (c *Client) updateNodeFromFingerprint(response *fingerprint.FingerprintResponse) *structs.Node {
	c.configLock.Lock()
	defer c.configLock.Unlock()

	nodeHasChanged := false
	newConfig := c.config.Copy()

	for name, newVal := range response.Attributes {
		oldVal := newConfig.Node.Attributes[name]
		if oldVal == newVal {
			continue
		}

		nodeHasChanged = true
		if newVal == "" {
			delete(newConfig.Node.Attributes, name)
		} else {
			newConfig.Node.Attributes[name] = newVal
		}
	}

	// update node links and resources from the diff created from
	// fingerprinting
	for name, newVal := range response.Links {
		oldVal := newConfig.Node.Links[name]
		if oldVal == newVal {
			continue
		}

		nodeHasChanged = true
		if newVal == "" {
			delete(newConfig.Node.Links, name)
		} else {
			newConfig.Node.Links[name] = newVal
		}
	}

	// COMPAT(0.10): Remove in 0.10
	// update the response networks with the config
	// if we still have node changes, merge them
	if response.Resources != nil {
		response.Resources.Networks = updateNetworks(
			response.Resources.Networks,
			newConfig)
		if !newConfig.Node.Resources.Equal(response.Resources) {
			newConfig.Node.Resources.Merge(response.Resources)
			nodeHasChanged = true
		}
	}

	// update the response networks with the config
	// if we still have node changes, merge them
	if response.NodeResources != nil {
		response.NodeResources.Networks = updateNetworks(response.NodeResources.Networks, newConfig)
		if !newConfig.Node.NodeResources.Equal(response.NodeResources) {
			newConfig.Node.NodeResources.Merge(response.NodeResources)
			nodeHasChanged = true
		}

		response.NodeResources.MinDynamicPort = newConfig.MinDynamicPort
		response.NodeResources.MaxDynamicPort = newConfig.MaxDynamicPort
		if newConfig.Node.NodeResources.MinDynamicPort != response.NodeResources.MinDynamicPort ||
			newConfig.Node.NodeResources.MaxDynamicPort != response.NodeResources.MaxDynamicPort {
			nodeHasChanged = true
		}

		// update config with total cpu compute if it was detected
		if cpu := int(response.NodeResources.Cpu.CpuShares); cpu > 0 {
			newConfig.CpuCompute = cpu
		}
	}

	if nodeHasChanged {
		c.config = newConfig
		c.updateNode()
	}

	return newConfig.Node
}

// updateNetworks filters and overrides network speed of host networks based
// on configured settings
func updateNetworks(up structs.Networks, c *config.Config) structs.Networks {
	if up == nil {
		return nil
	}

	if c.NetworkInterface != "" {
		// For host networks, if a network device is configured filter up to contain details for only
		// that device
		upd := []*structs.NetworkResource{}
		for _, n := range up {
			switch n.Mode {
			case "host":
				if c.NetworkInterface == n.Device {
					upd = append(upd, n)
				}
			default:
				upd = append(upd, n)

			}
		}
		up = upd
	}

	// if set, apply the config NetworkSpeed to networks in host mode
	if c.NetworkSpeed != 0 {
		for _, n := range up {
			if n.Mode == "host" {
				n.MBits = c.NetworkSpeed
			}
		}
	}
	return up
}

// retryIntv calculates a retry interval value given the base
func (c *Client) retryIntv(base time.Duration) time.Duration {
	if c.GetConfig().DevMode {
		return devModeRetryIntv
	}
	return base + helper.RandomStagger(base)
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
	if c.GetConfig().DevMode {
		heartbeat = time.After(0)
	} else {
		heartbeat = time.After(helper.RandomStagger(initialHeartbeatStagger))
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
				c.logger.Info("re-registering node")
				c.retryRegisterNode()
				heartbeat = time.After(helper.RandomStagger(initialHeartbeatStagger))
			} else {
				intv := c.getHeartbeatRetryIntv(err)
				c.logger.Error("error heartbeating. retrying", "error", err, "period", intv)
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

func (c *Client) lastHeartbeat() time.Time {
	return c.heartbeatStop.getLastOk()
}

// getHeartbeatRetryIntv is used to retrieve the time to wait before attempting
// another heartbeat.
func (c *Client) getHeartbeatRetryIntv(err error) time.Duration {
	if c.GetConfig().DevMode {
		return devModeRetryIntv
	}

	// Collect the useful heartbeat info
	c.heartbeatLock.Lock()
	haveHeartbeated := c.haveHeartbeated
	last := c.lastHeartbeat()
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
	left := time.Until(last.Add(ttl))

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
		return time.Second + helper.RandomStagger(time.Second)
	default:
	}

	stagger := helper.RandomStagger(left)
	switch {
	case stagger < time.Second:
		return time.Second + helper.RandomStagger(time.Second)
	case stagger > 30*time.Second:
		return 25*time.Second + helper.RandomStagger(5*time.Second)
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
				c.logger.Error("error saving state", "error", err)
			}

		case <-c.shutdownCh:
			return
		}
	}
}

// run is a long lived goroutine used to run the client. Shutdown() stops it first
func (c *Client) run() {

	// Watch for changes in allocations
	allocUpdates := make(chan *allocUpdates, 8)
	go c.watchAllocations(allocUpdates)

	for {
		select {
		case update := <-allocUpdates:
			// Don't apply updates while shutting down.
			c.shutdownLock.Lock()
			if c.shutdown {
				c.shutdownLock.Unlock()
				return
			}

			// Apply updates inside lock to prevent a concurrent
			// shutdown.
			c.runAllocs(update)
			c.shutdownLock.Unlock()

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
		NodeEvents: nodeEvents,
		WriteRequest: structs.WriteRequest{
			Region:    c.Region(),
			AuthToken: c.secretNodeID(),
		},
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

	timer := stoppedTimer()
	defer timer.Stop()

	for {
		select {
		case event := <-c.triggerEmitNodeEvent:
			if l := len(batchEvents); l <= structs.MaxRetainedNodeEvents {
				batchEvents = append(batchEvents, event)
			} else {
				// Drop the oldest event
				c.logger.Warn("dropping node event", "node_event", batchEvents[0])
				batchEvents = append(batchEvents[1:], event)
			}
			timer.Reset(c.retryIntv(nodeUpdateRetryIntv))
		case <-timer.C:
			if err := c.submitNodeEvents(batchEvents); err != nil {
				c.logger.Error("error submitting node events", "error", err)
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

	authToken := c.getRegistrationToken()

	for {
		err := c.registerNode(authToken)
		if err == nil {
			// Registered!
			return
		}

		retryIntv := registerRetryIntv
		if err == noServersErr || structs.IsErrNoRegionPath(err) {
			c.logger.Debug("registration waiting on servers")
			c.triggerDiscovery()
			retryIntv = noServerRetryIntv
		} else if structs.IsErrPermissionDenied(err) {
			// any previous cluster state we have here is invalid (ex. client
			// has been assigned to a new region), so clear the token and local
			// state for next pass.
			authToken = ""
			c.stateDB.PutNodeRegistration(&cstructs.NodeRegistration{HasRegistered: false})
			c.logger.Error("error registering", "error", err)
		} else {
			c.logger.Error("error registering", "error", err)
		}
		select {
		case <-c.rpcRetryWatcher():
		case <-time.After(c.retryIntv(retryIntv)):
		case <-c.shutdownCh:
			return
		}
	}
}

// getRegistrationToken gets the node secret to use for the Node.Register call.
// Registration is trust-on-first-use so we can't send the auth token with the
// initial request, but we want to add the auth token after that so that we can
// capture metrics.
func (c *Client) getRegistrationToken() string {

	select {
	case <-c.registeredCh:
		return c.secretNodeID()
	default:
		// If we haven't yet closed the registeredCh we're either starting for
		// the 1st time or we've just restarted. Check the local state to see if
		// we've written a successful registration previously so that we don't
		// block allocrunner operations on disconnected clients.
		registration, err := c.stateDB.GetNodeRegistration()
		if err != nil {
			c.logger.Error("could not determine previous node registration", "error", err)
		}
		if registration != nil && registration.HasRegistered {
			c.registeredOnce.Do(func() { close(c.registeredCh) })
			return c.secretNodeID()
		}
	}
	return ""
}

// registerNode is used to register the node or update the registration
func (c *Client) registerNode(authToken string) error {
	req := structs.NodeRegisterRequest{
		Node: c.Node(),
		WriteRequest: structs.WriteRequest{
			Region:    c.Region(),
			AuthToken: authToken,
		},
	}

	var resp structs.NodeUpdateResponse
	if err := c.UnauthenticatedRPC("Node.Register", &req, &resp); err != nil {
		return err
	}

	// Signal that we've registered once so that RPCs sent from the client can
	// send authenticated requests. Persist this information in the state so
	// that we don't block restoring running allocs when restarting while
	// disconnected
	c.registeredOnce.Do(func() {
		err := c.stateDB.PutNodeRegistration(&cstructs.NodeRegistration{
			HasRegistered: true,
		})
		if err != nil {
			c.logger.Error("could not write node registration", "error", err)
		}
		close(c.registeredCh)
	})

	err := c.handleNodeUpdateResponse(resp)
	if err != nil {
		return err
	}

	// Update the node status to ready after we register.
	c.UpdateConfig(func(c *config.Config) {
		c.Node.Status = structs.NodeStatusReady
	})

	c.logger.Info("node registration complete")
	if len(resp.EvalIDs) != 0 {
		c.logger.Debug("evaluations triggered by node registration", "num_evals", len(resp.EvalIDs))
	}

	c.heartbeatLock.Lock()
	defer c.heartbeatLock.Unlock()
	c.heartbeatStop.setLastOk(time.Now())
	c.heartbeatTTL = resp.HeartbeatTTL

	return nil
}

// updateNodeStatus is used to heartbeat and update the status of the node
func (c *Client) updateNodeStatus() error {
	start := time.Now()
	req := structs.NodeUpdateStatusRequest{
		NodeID: c.NodeID(),
		Status: structs.NodeStatusReady,
		WriteRequest: structs.WriteRequest{
			Region:    c.Region(),
			AuthToken: c.secretNodeID(),
		},
	}
	var resp structs.NodeUpdateResponse
	if err := c.RPC("Node.UpdateStatus", &req, &resp); err != nil {
		c.triggerDiscovery()
		return fmt.Errorf("failed to update status: %v", err)
	}
	end := time.Now()

	if len(resp.EvalIDs) != 0 {
		c.logger.Debug("evaluations triggered by node update", "num_evals", len(resp.EvalIDs))
	}

	// Update the last heartbeat and the new TTL, capturing the old values
	c.heartbeatLock.Lock()
	last := c.lastHeartbeat()
	oldTTL := c.heartbeatTTL
	haveHeartbeated := c.haveHeartbeated
	c.heartbeatStop.setLastOk(time.Now())
	c.heartbeatTTL = resp.HeartbeatTTL
	c.haveHeartbeated = true
	c.heartbeatLock.Unlock()
	c.logger.Trace("next heartbeat", "period", resp.HeartbeatTTL)

	if resp.Index != 0 {
		c.logger.Debug("state updated", "node_status", req.Status)

		// We have potentially missed our TTL log how delayed we were
		if haveHeartbeated {
			c.logger.Warn("missed heartbeat",
				"req_latency", end.Sub(start), "heartbeat_ttl", oldTTL, "since_last_heartbeat", time.Since(last))
		}
	}

	// Check heartbeat response for information about the server-side scheduling
	// state of this node. If there are errors on the server side, this will come
	// back as an empty string.
	c.UpdateConfig(func(c *config.Config) {
		if resp.SchedulingEligibility != "" {
			c.Node.SchedulingEligibility = resp.SchedulingEligibility
		}
	})

	err := c.handleNodeUpdateResponse(resp)
	if err != nil {
		return fmt.Errorf("heartbeat response returned no valid servers")
	}

	// If there's no Leader in the response we may be talking to a partitioned
	// server. Redo discovery to ensure our server list is up to date.
	if resp.LeaderRPCAddr == "" {
		c.triggerDiscovery()
	}

	c.EnterpriseClient.SetFeatures(resp.Features)
	return nil
}

func (c *Client) handleNodeUpdateResponse(resp structs.NodeUpdateResponse) error {
	// Update the number of nodes in the cluster so we can adjust our server
	// rebalance rate.
	c.servers.SetNumNodes(resp.NumNodes)

	// Convert []*NodeServerInfo to []*servers.Server
	nomadServers := make([]*servers.Server, 0, len(resp.Servers))
	for _, s := range resp.Servers {
		addr, err := resolveServer(s.RPCAdvertiseAddr)
		if err != nil {
			c.logger.Warn("ignoring invalid server", "error", err, "server", s.RPCAdvertiseAddr)
			continue
		}
		e := &servers.Server{Addr: addr}
		nomadServers = append(nomadServers, e)
	}
	if len(nomadServers) == 0 {
		return noServersErr
	}
	c.servers.SetServers(nomadServers)
	return nil
}

// AllocStateUpdated asynchronously updates the server with the current state
// of an allocations and its tasks.
func (c *Client) AllocStateUpdated(alloc *structs.Allocation) {
	if alloc.Terminated() {
		// Terminated, mark for GC if we're still tracking this alloc
		// runner. If it's not being tracked that means the server has
		// already GC'd it (see removeAlloc).
		ar, err := c.getAllocRunner(alloc.ID)

		if err == nil {
			c.garbageCollector.MarkForCollection(alloc.ID, ar)

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
	stripped.NetworkStatus = alloc.NetworkStatus

	c.pendingUpdates.add(stripped)
}

// PutAllocation stores an allocation or returns an error if it could not be stored.
func (c *Client) PutAllocation(alloc *structs.Allocation) error {
	return c.stateDB.PutAllocation(alloc)
}

// allocSync is a long lived function that batches allocation updates to the
// server.
func (c *Client) allocSync() {
	syncTicker := time.NewTicker(allocSyncIntv)
	updateTicks := 0

	for {
		select {
		case <-c.shutdownCh:
			syncTicker.Stop()
			return

		case <-syncTicker.C:

			updateTicks++
			toSync := c.pendingUpdates.nextBatch(c, updateTicks)

			if len(toSync) == 0 {
				syncTicker.Reset(allocSyncIntv)
				continue
			}

			// Send to server.
			args := structs.AllocUpdateRequest{
				Alloc: toSync,
				WriteRequest: structs.WriteRequest{
					Region:    c.Region(),
					AuthToken: c.secretNodeID(),
				},
			}

			var resp structs.GenericResponse
			err := c.RPC("Node.UpdateAlloc", &args, &resp)
			if err != nil {
				// Error updating allocations, do *not* clear
				// updates and retry after backoff
				c.logger.Error("error updating allocations", "error", err)

				// refill the updates queue with updates that we failed to make
				c.pendingUpdates.restore(toSync)
				syncTicker.Reset(c.retryIntv(allocSyncRetryIntv))
				continue
			}

			// Record that we've successfully synced these updates so that it's
			// written to disk
			c.allocLock.RLock()
			for _, update := range toSync {
				if ar, ok := c.allocs[update.ID]; ok {
					ar.AcknowledgeState(&arstate.State{
						ClientStatus:      update.ClientStatus,
						ClientDescription: update.ClientDescription,
						DeploymentStatus:  update.DeploymentStatus,
						TaskStates:        update.TaskStates,
						NetworkStatus:     update.NetworkStatus,
					})
				}
			}
			c.allocLock.RUnlock()

			// Successfully updated allocs. Reset ticker to give loop time to
			// receive new alloc updates. Otherwise if the RPC took the ticker
			// interval we may call it in a tight loop reading empty updates.
			updateTicks = 0
			syncTicker.Reset(allocSyncIntv)
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
			Region: c.Region(),

			// Make a consistent read query when the client starts
			// to avoid acting on stale data that predates this
			// client state before a client restart.
			//
			// After the first request, only require monotonically
			// increasing state.
			AllowStale: false,
			AuthToken:  c.secretNodeID(),
		},
	}
	var resp structs.NodeClientAllocsResponse

	// The request and response for pulling down the set of allocations that are
	// new, or updated server side.
	allocsReq := structs.AllocsGetRequest{
		QueryOptions: structs.QueryOptions{
			Region:     c.Region(),
			AllowStale: true,
			AuthToken:  c.secretNodeID(),
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
				c.logger.Debug("secret mismatch; re-registering node", "error", err)
				c.retryRegisterNode()
			} else if err != noServersErr {
				c.logger.Error("error querying node allocations", "error", err)
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

		// We have not received any new data, or received stale data. This may happen in
		// an array of situations, the worst of which seems to be a blocking request
		// timeout when the scheduler which we are contacting is newly added or recovering
		// after a prolonged downtime.
		//
		// For full context, please see https://github.com/hashicorp/nomad/issues/18267
		if resp.Index <= req.MinQueryIndex {
			c.logger.Debug("Received stale allocation information. Retrying.",
				"index", resp.Index, "min_index", req.MinQueryIndex)
			continue OUTER
		}

		// Filter all allocations whose AllocModifyIndex was not incremented.
		// These are the allocations who have either not been updated, or whose
		// updates are a result of the client sending an update for the alloc.
		// This lets us reduce the network traffic to the server as we don't
		// need to pull all the allocations.
		var pull []string
		filtered := make(map[string]struct{})
		var pullIndex uint64
		for allocID, modifyIndex := range resp.Allocs {
			// Pull the allocation if we don't have an alloc runner for the
			// allocation or if the alloc runner requires an updated allocation.
			//XXX Part of Client alloc index tracking exp
			c.allocLock.RLock()
			currentAR, ok := c.allocs[allocID]
			c.allocLock.RUnlock()

			// Ignore alloc updates for allocs that are invalid because of initialization errors
			c.invalidAllocsLock.Lock()
			_, isInvalid := c.invalidAllocs[allocID]
			c.invalidAllocsLock.Unlock()

			if (!ok || modifyIndex > currentAR.Alloc().AllocModifyIndex) && !isInvalid {
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
				c.logger.Error("error querying updated allocations", "error", err)
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

				// handle an old Server
				alloc.Canonicalize()

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

		c.logger.Debug("updated allocations", "index", resp.Index,
			"total", len(resp.Allocs), "pulled", len(allocsResp.Allocs), "filtered", len(filtered))

		// After the first request, only require monotonically increasing state.
		req.AllowStale = true
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

// updateNode signals the client to send the updated
// Node to the server.
func (c *Client) updateNode() {
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

	timer := stoppedTimer()
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			c.logger.Debug("state changed, updating node and re-registering")
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
	existing := make(map[string]uint64, len(c.allocs))
	for id, ar := range c.allocs {
		existing[id] = ar.Alloc().AllocModifyIndex
	}
	c.allocLock.RUnlock()

	// Diff the existing and updated allocations
	diff := diffAllocs(existing, update)
	c.logger.Debug("allocation updates", "added", len(diff.added), "removed", len(diff.removed),
		"updated", len(diff.updated), "ignored", len(diff.ignore))

	errs := 0

	// Remove the old allocations
	for _, remove := range diff.removed {
		c.removeAlloc(remove)
	}

	// Update the existing allocations
	for _, update := range diff.updated {
		c.updateAlloc(update)
	}

	// Make room for new allocations before running
	if err := c.garbageCollector.MakeRoomFor(diff.added); err != nil {
		c.logger.Error("error making room for new allocations", "error", err)
		errs++
	}

	// Start the new allocations
	for _, add := range diff.added {
		migrateToken := update.migrateTokens[add.ID]
		if err := c.addAlloc(add, migrateToken); err != nil {
			c.logger.Error("error adding alloc", "error", err, "alloc_id", add.ID)
			errs++
			// We mark the alloc as failed and send an update to the server
			// We track the fact that creating an allocrunner failed so that we don't send updates again
			if add.ClientStatus != structs.AllocClientStatusFailed {
				c.handleInvalidAllocs(add, err)
			}
		}
	}

	// Mark servers as having been contacted so blocked tasks that failed
	// to restore can now restart.
	c.serversContactedOnce.Do(func() {
		close(c.serversContactedCh)
	})

	// Trigger the GC once more now that new allocs are started that could
	// have caused thresholds to be exceeded
	c.garbageCollector.Trigger()
	c.logger.Debug("allocation updates applied", "added", len(diff.added), "removed", len(diff.removed),
		"updated", len(diff.updated), "ignored", len(diff.ignore), "errors", errs)
}

// makeFailedAlloc creates a stripped down version of the allocation passed in
// with its status set to failed and other fields needed for the server to be
// able to examine deployment and task states
func makeFailedAlloc(add *structs.Allocation, err error) *structs.Allocation {
	stripped := new(structs.Allocation)
	stripped.ID = add.ID
	stripped.NodeID = add.NodeID
	stripped.ClientStatus = structs.AllocClientStatusFailed
	stripped.ClientDescription = fmt.Sprintf("Unable to add allocation due to error: %v", err)

	// Copy task states if it exists in the original allocation
	if add.TaskStates != nil {
		stripped.TaskStates = add.TaskStates
	} else {
		stripped.TaskStates = make(map[string]*structs.TaskState)
	}

	failTime := time.Now()
	if add.DeploymentStatus.HasHealth() {
		// Never change deployment health once it has been set
		stripped.DeploymentStatus = add.DeploymentStatus.Copy()
	} else {
		stripped.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy:   pointer.Of(false),
			Timestamp: failTime,
		}
	}

	taskGroup := add.Job.LookupTaskGroup(add.TaskGroup)
	if taskGroup == nil {
		return stripped
	}
	for _, task := range taskGroup.Tasks {
		ts, ok := stripped.TaskStates[task.Name]
		if !ok {
			ts = &structs.TaskState{}
			stripped.TaskStates[task.Name] = ts
		}
		if ts.FinishedAt.IsZero() {
			ts.FinishedAt = failTime
		}
	}
	return stripped
}

// removeAlloc is invoked when we should remove an allocation because it has
// been removed by the server.
func (c *Client) removeAlloc(allocID string) {
	c.allocLock.Lock()
	defer c.allocLock.Unlock()

	ar, ok := c.allocs[allocID]
	if !ok {
		c.invalidAllocsLock.Lock()
		if _, ok := c.invalidAllocs[allocID]; ok {
			// Removing from invalid allocs map if present
			delete(c.invalidAllocs, allocID)
		} else {
			// Alloc is unknown, log a warning.
			c.logger.Warn("cannot remove nonexistent alloc", "alloc_id", allocID, "error", "alloc not found")
		}
		c.invalidAllocsLock.Unlock()
		return
	}

	// Stop tracking alloc runner as it's been GC'd by the server
	delete(c.allocs, allocID)

	// Ensure the GC has a reference and then collect. Collecting through the GC
	// applies rate limiting
	c.garbageCollector.MarkForCollection(allocID, ar)

	// GC immediately since the server has GC'd it
	go c.garbageCollector.Collect(allocID)
}

// updateAlloc is invoked when we should update an allocation
func (c *Client) updateAlloc(update *structs.Allocation) {
	ar, err := c.getAllocRunner(update.ID)
	if err != nil {
		c.logger.Warn("cannot update nonexistent alloc", "alloc_id", update.ID)
		return
	}

	// Reconnect unknown allocations if they were updated and are not terminal.
	reconnect := update.ClientStatus == structs.AllocClientStatusUnknown &&
		update.AllocModifyIndex > ar.Alloc().AllocModifyIndex &&
		!update.ServerTerminalStatus()
	if reconnect {
		err = ar.Reconnect(update)
		if err != nil {
			c.logger.Error("error reconnecting alloc", "alloc_id", update.ID, "alloc_modify_index", update.AllocModifyIndex, "error", err)
		}
		return
	}

	// Update local copy of alloc
	if err := c.stateDB.PutAllocation(update); err != nil {
		c.logger.Error("error persisting updated alloc locally", "error", err, "alloc_id", update.ID)
	}

	// Update alloc runner
	ar.Update(update)
}

// addAlloc is invoked when we should add an allocation
func (c *Client) addAlloc(alloc *structs.Allocation, migrateToken string) error {
	c.allocLock.Lock()
	defer c.allocLock.Unlock()

	// Check if we already have an alloc runner
	if _, ok := c.allocs[alloc.ID]; ok {
		c.logger.Debug("dropping duplicate add allocation request", "alloc_id", alloc.ID)
		return nil
	}

	// Initialize local copy of alloc before creating the alloc runner so
	// we can't end up with an alloc runner that does not have an alloc.
	if err := c.stateDB.PutAllocation(alloc); err != nil {
		return err
	}

	// Collect any preempted allocations to pass into the previous alloc watcher
	var preemptedAllocs map[string]allocwatcher.AllocRunnerMeta
	if len(alloc.PreemptedAllocations) > 0 {
		preemptedAllocs = make(map[string]allocwatcher.AllocRunnerMeta)
		for _, palloc := range alloc.PreemptedAllocations {
			preemptedAllocs[palloc] = c.allocs[palloc]
		}
	}

	// Since only the Client has access to other AllocRunners and the RPC
	// client, create the previous allocation watcher here.
	watcherConfig := allocwatcher.Config{
		Alloc:            alloc,
		PreviousRunner:   c.allocs[alloc.PreviousAllocation],
		PreemptedRunners: preemptedAllocs,
		RPC:              c,
		Config:           c.GetConfig(),
		MigrateToken:     migrateToken,
		Logger:           c.logger,
	}
	prevAllocWatcher, prevAllocMigrator := allocwatcher.NewAllocWatcher(watcherConfig)

	arConf := &config.AllocRunnerConfig{
		Alloc:               alloc,
		Logger:              c.logger,
		ClientConfig:        c.GetConfig(),
		StateDB:             c.stateDB,
		Consul:              c.consulService,
		ConsulProxies:       c.consulProxies,
		ConsulSI:            c.tokensClient,
		Vault:               c.vaultClient,
		StateUpdater:        c,
		DeviceStatsReporter: c,
		PrevAllocWatcher:    prevAllocWatcher,
		PrevAllocMigrator:   prevAllocMigrator,
		DynamicRegistry:     c.dynamicRegistry,
		CSIManager:          c.csimanager,
		DeviceManager:       c.devicemanager,
		DriverManager:       c.drivermanager,
		ServiceRegWrapper:   c.serviceRegWrapper,
		CheckStore:          c.checkStore,
		RPCClient:           c,
		Getter:              c.getter,
		Wranglers:           c.wranglers,
		WIDMgr:              c.widmgr,
	}

	ar, err := c.allocrunnerFactory(arConf)
	if err != nil {
		return err
	}

	// Store the alloc runner.
	c.allocs[alloc.ID] = ar

	// Maybe mark the alloc for halt on missing server heartbeats
	c.heartbeatStop.allocHook(alloc)

	go ar.Run()
	return nil
}

// setupConsulTokenClient configures a tokenClient for managing consul service
// identity tokens.
func (c *Client) setupConsulTokenClient() error {
	tc := consulApi.NewIdentitiesClient(c.logger, c.deriveSIToken)
	c.tokensClient = tc
	return nil
}

// setupVaultClient creates an object to periodically renew tokens and secrets
// with vault.
func (c *Client) setupVaultClient() error {
	var err error
	c.vaultClient, err = vaultclient.NewVaultClient(c.GetConfig().VaultConfig, c.logger, c.deriveToken)
	if err != nil {
		return err
	}

	if c.vaultClient == nil {
		c.logger.Error("failed to create vault client")
		return fmt.Errorf("failed to create vault client")
	}

	// Start renewing tokens and secrets
	c.vaultClient.Start()

	return nil
}

// setupNomadServiceRegistrationHandler sets up the registration handler to use
// for native service discovery.
func (c *Client) setupNomadServiceRegistrationHandler() {
	cfg := nsd.ServiceRegistrationHandlerCfg{
		Datacenter: c.Datacenter(),
		Enabled:    c.GetConfig().NomadServiceDiscovery,
		NodeID:     c.NodeID(),
		NodeSecret: c.secretNodeID(),
		Region:     c.Region(),
		RPCFn:      c.RPC,
		CheckWatcher: serviceregistration.NewCheckWatcher(
			c.logger, nsd.NewStatusGetter(c.checkStore),
		),
	}
	c.nomadService = nsd.NewServiceRegistrationHandler(c.logger, &cfg)
}

// deriveToken takes in an allocation and a set of tasks and derives vault
// tokens for each of the tasks, unwraps all of them using the supplied vault
// client and returns a map of unwrapped tokens, indexed by the task name.
func (c *Client) deriveToken(alloc *structs.Allocation, taskNames []string, vclient *vaultapi.Client) (map[string]string, error) {
	vlogger := c.logger.Named("vault")

	verifiedTasks, err := verifiedTasks(vlogger, alloc, taskNames)
	if err != nil {
		return nil, err
	}

	// DeriveVaultToken of nomad server can take in a set of tasks and
	// creates tokens for all the tasks.
	req := &structs.DeriveVaultTokenRequest{
		NodeID:   c.NodeID(),
		SecretID: c.secretNodeID(),
		AllocID:  alloc.ID,
		Tasks:    verifiedTasks,
		QueryOptions: structs.QueryOptions{
			Region:        c.Region(),
			AllowStale:    false,
			MinQueryIndex: alloc.CreateIndex,
			AuthToken:     c.secretNodeID(),
		},
	}

	// Derive the tokens
	// namespace is handled via nomad/vault
	var resp structs.DeriveVaultTokenResponse
	if err := c.RPC("Node.DeriveVaultToken", &req, &resp); err != nil {
		vlogger.Error("error making derive token RPC", "error", err)
		return nil, fmt.Errorf("DeriveVaultToken RPC failed: %v", err)
	}
	if resp.Error != nil {
		vlogger.Error("error deriving vault tokens", "error", resp.Error)
		return nil, structs.NewWrappedServerError(resp.Error)
	}
	if resp.Tasks == nil {
		vlogger.Error("error derivng vault token", "error", "invalid response")
		return nil, fmt.Errorf("failed to derive vault tokens: invalid response")
	}

	unwrappedTokens := make(map[string]string)

	// Retrieve the wrapped tokens from the response and unwrap it
	for _, taskName := range verifiedTasks {
		// Get the wrapped token
		wrappedToken, ok := resp.Tasks[taskName]
		if !ok {
			vlogger.Error("wrapped token missing for task", "task_name", taskName)
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
			vlogger.Warn("error unwrapping token", "error", err)
			return nil, structs.NewRecoverableError(validationErr, true)
		}

		// Append the unwrapped token to the return value
		unwrappedTokens[taskName] = unwrapResp.Auth.ClientToken
	}

	return unwrappedTokens, nil
}

// deriveSIToken takes an allocation and a set of tasks and derives Consul
// Service Identity tokens for each of the tasks by requesting them from the
// Nomad Server.
func (c *Client) deriveSIToken(alloc *structs.Allocation, taskNames []string) (map[string]string, error) {
	tasks, err := verifiedTasks(c.logger, alloc, taskNames)
	if err != nil {
		return nil, err
	}

	req := &structs.DeriveSITokenRequest{
		NodeID:   c.NodeID(),
		SecretID: c.secretNodeID(),
		AllocID:  alloc.ID,
		Tasks:    tasks,
		QueryOptions: structs.QueryOptions{
			Region:    c.Region(),
			AuthToken: c.secretNodeID(),
		},
	}

	// Nicely ask Nomad Server for the tokens.
	var resp structs.DeriveSITokenResponse
	if err := c.RPC("Node.DeriveSIToken", &req, &resp); err != nil {
		c.logger.Error("error making derive token RPC", "error", err)
		return nil, fmt.Errorf("DeriveSIToken RPC failed: %v", err)
	}
	if err := resp.Error; err != nil {
		c.logger.Error("error deriving SI tokens", "error", err)
		return nil, structs.NewWrappedServerError(err)
	}
	if len(resp.Tokens) == 0 {
		c.logger.Error("error deriving SI tokens", "error", "invalid_response")
		return nil, fmt.Errorf("failed to derive SI tokens: invalid response")
	}

	// NOTE: Unlike with the Vault integration, Nomad Server replies with the
	// actual Consul SI token (.SecretID), because otherwise each Nomad
	// Client would need to be blessed with 'acl:write' permissions to read the
	// secret value given the .AccessorID, which does not fit well in the Consul
	// security model.
	//
	// https://www.consul.io/api/acl/tokens.html#read-a-token
	// https://www.consul.io/docs/internals/security.html

	m := maps.Clone(resp.Tokens)
	return m, nil
}

// verifiedTasks asserts each task in taskNames actually exists in the given alloc,
// otherwise an error is returned.
func verifiedTasks(logger hclog.Logger, alloc *structs.Allocation, taskNames []string) ([]string, error) {
	if alloc == nil {
		return nil, fmt.Errorf("nil allocation")
	}

	if len(taskNames) == 0 {
		return nil, fmt.Errorf("missing task names")
	}

	group := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if group == nil {
		return nil, fmt.Errorf("group name in allocation is not present in job")
	}

	verifiedTasks := make([]string, 0, len(taskNames))

	// confirm the requested task names actually exist in the allocation
	for _, taskName := range taskNames {
		if !taskIsPresent(taskName, group.Tasks) {
			logger.Error("task not found in the allocation", "task_name", taskName)
			return nil, fmt.Errorf("task %q not found in allocation", taskName)
		}
		verifiedTasks = append(verifiedTasks, taskName)
	}

	return verifiedTasks, nil
}

func taskIsPresent(taskName string, tasks []*structs.Task) bool {
	for _, task := range tasks {
		if task.Name == taskName {
			return true
		}
	}
	return false
}

// triggerDiscovery causes a Consul discovery to begin (if one hasn't already)
func (c *Client) triggerDiscovery() {
	config := c.GetConfig()
	if config.ConsulConfig.ClientAutoJoin != nil && *config.ConsulConfig.ClientAutoJoin {
		select {
		case c.triggerDiscoveryCh <- struct{}{}:
			// Discovery goroutine was released to execute
		default:
			// Discovery goroutine was already running
		}
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
				c.logger.Error("error discovering nomad servers", "error", err)
			}
		case <-c.shutdownCh:
			return
		}
	}
}

func (c *Client) consulDiscoveryImpl() error {
	consulLogger := c.logger.Named("consul")

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
		dcs = dcs[0:min(len(dcs), datacenterQueryLimit)]
	}

	serviceName := c.GetConfig().ConsulConfig.ServerServiceName
	var mErr multierror.Error
	var nomadServers servers.Servers
	consulLogger.Debug("bootstrap contacting Consul DCs", "consul_dcs", dcs)
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

			srv := &servers.Server{Addr: addr}
			nomadServers = append(nomadServers, srv)
		}

		if len(nomadServers) > 0 {
			break DISCOLOOP
		}

	}
	if len(nomadServers) == 0 {
		if len(mErr.Errors) > 0 {
			return mErr.ErrorOrNil()
		}
		return fmt.Errorf("no Nomad Servers advertising service %q in Consul datacenters: %+q", serviceName, dcs)
	}

	consulLogger.Info("discovered following servers", "servers", nomadServers)

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
	// Determining NodeClass to be emitted
	var emittedNodeClass string
	if emittedNodeClass = c.Node().NodeClass; emittedNodeClass == "" {
		emittedNodeClass = "none"
	}

	// Assign labels directly before emitting stats so the information expected
	// is ready
	c.baseLabels = []metrics.Label{
		{Name: "node_id", Value: c.NodeID()},
		{Name: "datacenter", Value: c.Datacenter()},
		{Name: "node_class", Value: emittedNodeClass},
		{Name: "node_pool", Value: c.Node().NodePool},
	}

	// Start collecting host stats right away and then keep collecting every
	// collection interval
	next := time.NewTimer(0)
	defer next.Stop()
	for {
		config := c.GetConfig()
		select {
		case <-next.C:
			err := c.hostStatsCollector.Collect()
			next.Reset(config.StatsCollectionInterval)
			if err != nil {
				c.logger.Warn("error fetching host resource usage stats", "error", err)
			} else if config.PublishNodeMetrics {
				// Publish Node metrics if operator has opted in
				c.emitHostStats()
			}

			c.emitClientMetrics()
		case <-c.shutdownCh:
			return
		}
	}
}

// setGaugeForMemoryStats proxies metrics for memory specific statistics
func (c *Client) setGaugeForMemoryStats(nodeID string, hStats *hoststats.HostStats, baseLabels []metrics.Label) {
	metrics.SetGaugeWithLabels([]string{"client", "host", "memory", "total"}, float32(hStats.Memory.Total), baseLabels)
	metrics.SetGaugeWithLabels([]string{"client", "host", "memory", "available"}, float32(hStats.Memory.Available), baseLabels)
	metrics.SetGaugeWithLabels([]string{"client", "host", "memory", "used"}, float32(hStats.Memory.Used), baseLabels)
	metrics.SetGaugeWithLabels([]string{"client", "host", "memory", "free"}, float32(hStats.Memory.Free), baseLabels)
}

// setGaugeForCPUStats proxies metrics for CPU specific statistics
func (c *Client) setGaugeForCPUStats(nodeID string, hStats *hoststats.HostStats, baseLabels []metrics.Label) {

	labels := make([]metrics.Label, len(baseLabels))
	copy(labels, baseLabels)

	for _, cpu := range hStats.CPU {
		labels := append(labels, metrics.Label{
			Name:  "cpu",
			Value: cpu.CPU,
		})

		// Keep "total" around to remain compatible with older consumers of the metrics
		metrics.SetGaugeWithLabels([]string{"client", "host", "cpu", "total"}, float32(cpu.TotalPercent), labels)
		metrics.SetGaugeWithLabels([]string{"client", "host", "cpu", "total_percent"}, float32(cpu.TotalPercent), labels)
		metrics.SetGaugeWithLabels([]string{"client", "host", "cpu", "total_ticks"}, float32(cpu.TotalTicks), labels)
		metrics.IncrCounterWithLabels([]string{"client", "host", "cpu", "total_ticks_count"}, float32(cpu.TotalTicks), labels)
		metrics.SetGaugeWithLabels([]string{"client", "host", "cpu", "user"}, float32(cpu.User), labels)
		metrics.SetGaugeWithLabels([]string{"client", "host", "cpu", "idle"}, float32(cpu.Idle), labels)
		metrics.SetGaugeWithLabels([]string{"client", "host", "cpu", "system"}, float32(cpu.System), labels)
	}
}

// setGaugeForDiskStats proxies metrics for disk specific statistics
func (c *Client) setGaugeForDiskStats(nodeID string, hStats *hoststats.HostStats, baseLabels []metrics.Label) {

	labels := make([]metrics.Label, len(baseLabels))
	copy(labels, baseLabels)

	for _, disk := range hStats.DiskStats {
		labels := append(labels, metrics.Label{
			Name:  "disk",
			Value: disk.Device,
		})

		metrics.SetGaugeWithLabels([]string{"client", "host", "disk", "size"}, float32(disk.Size), labels)
		metrics.SetGaugeWithLabels([]string{"client", "host", "disk", "used"}, float32(disk.Used), labels)
		metrics.SetGaugeWithLabels([]string{"client", "host", "disk", "available"}, float32(disk.Available), labels)
		metrics.SetGaugeWithLabels([]string{"client", "host", "disk", "used_percent"}, float32(disk.UsedPercent), labels)
		metrics.SetGaugeWithLabels([]string{"client", "host", "disk", "inodes_percent"}, float32(disk.InodesUsedPercent), labels)
	}
}

// setGaugeForAllocationStats proxies metrics for allocation specific statistics
func (c *Client) setGaugeForAllocationStats(nodeID string, baseLabels []metrics.Label) {
	node := c.GetConfig().Node
	total := node.NodeResources
	res := node.ReservedResources
	allocated := c.getAllocatedResources(node)

	// Emit allocated
	metrics.SetGaugeWithLabels([]string{"client", "allocated", "memory"}, float32(allocated.Flattened.Memory.MemoryMB), baseLabels)
	metrics.SetGaugeWithLabels([]string{"client", "allocated", "max_memory"}, float32(allocated.Flattened.Memory.MemoryMaxMB), baseLabels)
	metrics.SetGaugeWithLabels([]string{"client", "allocated", "disk"}, float32(allocated.Shared.DiskMB), baseLabels)
	metrics.SetGaugeWithLabels([]string{"client", "allocated", "cpu"}, float32(allocated.Flattened.Cpu.CpuShares), baseLabels)

	for _, n := range allocated.Flattened.Networks {
		labels := append(baseLabels, metrics.Label{ //nolint:gocritic
			Name:  "device",
			Value: n.Device,
		})
		metrics.SetGaugeWithLabels([]string{"client", "allocated", "network"}, float32(n.MBits), labels)
	}

	// Emit unallocated
	unallocatedMem := total.Memory.MemoryMB - res.Memory.MemoryMB - allocated.Flattened.Memory.MemoryMB
	unallocatedDisk := total.Disk.DiskMB - res.Disk.DiskMB - allocated.Shared.DiskMB
	unallocatedCpu := total.Cpu.CpuShares - res.Cpu.CpuShares - allocated.Flattened.Cpu.CpuShares

	metrics.SetGaugeWithLabels([]string{"client", "unallocated", "memory"}, float32(unallocatedMem), baseLabels)
	metrics.SetGaugeWithLabels([]string{"client", "unallocated", "disk"}, float32(unallocatedDisk), baseLabels)
	metrics.SetGaugeWithLabels([]string{"client", "unallocated", "cpu"}, float32(unallocatedCpu), baseLabels)

	totalComparable := total.Comparable()
	for _, n := range totalComparable.Flattened.Networks {
		// Determined the used resources
		var usedMbits int
		totalIdx := allocated.Flattened.Networks.NetIndex(n)
		if totalIdx != -1 {
			usedMbits = allocated.Flattened.Networks[totalIdx].MBits
		}

		unallocatedMbits := n.MBits - usedMbits
		labels := append(baseLabels, metrics.Label{ //nolint:gocritic
			Name:  "device",
			Value: n.Device,
		})
		metrics.SetGaugeWithLabels([]string{"client", "unallocated", "network"}, float32(unallocatedMbits), labels)
	}
}

// No labels are required so we emit with only a key/value syntax
func (c *Client) setGaugeForUptime(hStats *hoststats.HostStats, baseLabels []metrics.Label) {
	metrics.SetGaugeWithLabels([]string{"client", "uptime"}, float32(hStats.Uptime), baseLabels)
}

// emitHostStats pushes host resource usage stats to remote metrics collection sinks
func (c *Client) emitHostStats() {
	nodeID := c.NodeID()
	hStats := c.hostStatsCollector.Stats()
	labels := c.labels()

	c.setGaugeForMemoryStats(nodeID, hStats, labels)
	c.setGaugeForUptime(hStats, labels)
	c.setGaugeForCPUStats(nodeID, hStats, labels)
	c.setGaugeForDiskStats(nodeID, hStats, labels)
}

// emitClientMetrics emits lower volume client metrics
func (c *Client) emitClientMetrics() {
	nodeID := c.NodeID()
	labels := c.labels()

	c.setGaugeForAllocationStats(nodeID, labels)

	// Emit allocation metrics
	blocked, migrating, pending, running, terminal := 0, 0, 0, 0, 0
	for _, ar := range c.getAllocRunners() {
		switch ar.AllocState().ClientStatus {
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

	metrics.SetGaugeWithLabels([]string{"client", "allocations", "migrating"}, float32(migrating), labels)
	metrics.SetGaugeWithLabels([]string{"client", "allocations", "blocked"}, float32(blocked), labels)
	metrics.SetGaugeWithLabels([]string{"client", "allocations", "pending"}, float32(pending), labels)
	metrics.SetGaugeWithLabels([]string{"client", "allocations", "running"}, float32(running), labels)
	metrics.SetGaugeWithLabels([]string{"client", "allocations", "terminal"}, float32(terminal), labels)
}

// labels takes the base labels and appends the node state
func (c *Client) labels() []metrics.Label {
	node := c.Node()

	return append(c.baseLabels,
		metrics.Label{Name: "node_status", Value: node.Status},
		metrics.Label{Name: "node_scheduling_eligibility", Value: node.SchedulingEligibility},
	)
}

func (c *Client) getAllocatedResources(selfNode *structs.Node) *structs.ComparableResources {
	// Unfortunately the allocs only have IP so we need to match them to the
	// device
	cidrToDevice := make(map[*net.IPNet]string, len(selfNode.Resources.Networks))
	for _, n := range selfNode.NodeResources.Networks {
		_, ipnet, err := net.ParseCIDR(n.CIDR)
		if err != nil {
			continue
		}
		cidrToDevice[ipnet] = n.Device
	}

	// Sum the allocated resources
	var allocated structs.ComparableResources
	allocatedDeviceMbits := make(map[string]int)
	for _, ar := range c.getAllocRunners() {
		alloc := ar.Alloc()
		if alloc.ServerTerminalStatus() || ar.AllocState().ClientTerminalStatus() {
			continue
		}

		// Add the resources
		// COMPAT(0.11): Just use the allocated resources
		allocated.Add(alloc.ComparableResources())

		// Add the used network
		if alloc.AllocatedResources != nil {
			for _, tr := range alloc.AllocatedResources.Tasks {
				for _, allocatedNetwork := range tr.Networks {
					for cidr, dev := range cidrToDevice {
						ip := net.ParseIP(allocatedNetwork.IP)
						if cidr.Contains(ip) {
							allocatedDeviceMbits[dev] += allocatedNetwork.MBits
							break
						}
					}
				}
			}
		} else if alloc.Resources != nil {
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
	allocated.Flattened.Networks = nil
	for dev, speed := range allocatedDeviceMbits {
		net := &structs.NetworkResource{
			Device: dev,
			MBits:  speed,
		}
		allocated.Flattened.Networks = append(allocated.Flattened.Networks, net)
	}

	return &allocated
}

// GetTaskEventHandler returns an event handler for the given allocID and task name
func (c *Client) GetTaskEventHandler(allocID, taskName string) drivermanager.EventHandler {
	c.allocLock.RLock()
	defer c.allocLock.RUnlock()
	if ar, ok := c.allocs[allocID]; ok {
		return ar.GetTaskEventHandler(taskName)
	}
	return nil
}

// group wraps a func() in a goroutine and provides a way to block until it
// exits. Inspired by https://godoc.org/golang.org/x/sync/errgroup
type group struct {
	wg sync.WaitGroup
}

// Go starts f in a goroutine and must be called before Wait.
func (g *group) Go(f func()) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		f()
	}()
}

func (g *group) AddCh(ch <-chan struct{}) {
	g.Go(func() {
		<-ch
	})
}

// Wait for all goroutines to exit. Must be called after all calls to Go
// complete.
func (g *group) Wait() {
	g.wg.Wait()
}

// pendingClientUpdates are the set of allocation updates that the client is
// waiting to send
type pendingClientUpdates struct {
	updates map[string]*structs.Allocation
	lock    sync.Mutex
}

func newPendingClientUpdates() *pendingClientUpdates {
	return &pendingClientUpdates{
		updates: make(map[string]*structs.Allocation, 64),
	}
}

// add overwrites a pending update. The updates we get from the allocrunner are
// lightweight copies of its *structs.Allocation (i.e. just the client state),
// serialized with an internal lock. So the latest update is always the
// authoritative one, and the server only cares about that one.
func (p *pendingClientUpdates) add(alloc *structs.Allocation) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.updates[alloc.ID] = alloc
}

// restore refills the pending updates map, but only if a newer update hasn't come in
func (p *pendingClientUpdates) restore(toRestore []*structs.Allocation) {
	p.lock.Lock()
	defer p.lock.Unlock()

	for _, alloc := range toRestore {
		if _, ok := p.updates[alloc.ID]; !ok {
			p.updates[alloc.ID] = alloc
		}
	}
}

// nextBatch returns a list of client allocation updates we need to make in this
// tick of the allocSync. It returns nil if there's no updates to make yet. The
// caller is responsible for calling restore() if it can't successfully send the
// updates.
func (p *pendingClientUpdates) nextBatch(c *Client, updateTicks int) []*structs.Allocation {
	p.lock.Lock()
	defer p.lock.Unlock()

	// Fast path if there are no pending updates
	if len(p.updates) == 0 {
		return nil
	}

	// Ensure we never send an update before we've had at least one sync from
	// the server
	select {
	case <-c.serversContactedCh:
	default:
		return nil
	}

	toSync, urgent := p.filterAcknowledgedUpdatesLocked(c)

	// Only update every 5th tick if there's no priority updates
	if updateTicks%5 != 0 && !urgent {
		return nil
	}

	// Clear here so that allocrunners can queue up the next set of updates
	// while we're waiting to hear from the server
	maps.Clear(p.updates)

	return toSync

}

// filteredAcknowledgedUpdatesLocked returns a list of client alloc updates with
// the already-acknowledged updates removed, and the highest priority of any
// update. note: this method requires that p.lock is held
func (p *pendingClientUpdates) filterAcknowledgedUpdatesLocked(c *Client) ([]*structs.Allocation, bool) {
	var urgent bool
	sync := make([]*structs.Allocation, 0, len(p.updates))
	c.allocLock.RLock()
	defer c.allocLock.RUnlock()

	for allocID, update := range p.updates {
		if ar, ok := c.allocs[allocID]; ok {
			switch ar.GetUpdatePriority(update) {
			case cstructs.AllocUpdatePriorityUrgent:
				sync = append(sync, update)
				urgent = true
			case cstructs.AllocUpdatePriorityTypical:
				sync = append(sync, update)
			case cstructs.AllocUpdatePriorityNone:
				// update is dropped
			}
		} else {
			// no allocrunner (typically a failed placement), so we need
			// to send update
			sync = append(sync, update)
		}
	}
	return sync, urgent
}
