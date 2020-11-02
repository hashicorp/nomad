package nomad

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/consul/autopilot"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	log "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	lru "github.com/hashicorp/golang-lru"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/codec"
	"github.com/hashicorp/nomad/helper/pool"
	"github.com/hashicorp/nomad/helper/stats"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/nomad/deploymentwatcher"
	"github.com/hashicorp/nomad/nomad/drainer"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/nomad/volumewatcher"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/hashicorp/serf/serf"
)

const (
	// datacenterQueryLimit sets the max number of DCs that a Nomad
	// Server will query to find bootstrap_expect servers.
	datacenterQueryLimit = 25

	// maxStaleLeadership is the maximum time we will permit this Nomad
	// Server to go without seeing a valid Raft leader.
	maxStaleLeadership = 15 * time.Second

	// peersPollInterval is used as the polling interval between attempts
	// to query Consul for Nomad Servers.
	peersPollInterval = 45 * time.Second

	// peersPollJitter is used to provide a slight amount of variance to
	// the retry interval when querying Consul Servers
	peersPollJitterFactor = 2

	raftState         = "raft/"
	serfSnapshot      = "serf/snapshot"
	snapshotsRetained = 2

	// serverRPCCache controls how long we keep an idle connection open to a server
	serverRPCCache = 2 * time.Minute

	// serverMaxStreams controls how many idle streams we keep open to a server
	serverMaxStreams = 64

	// raftLogCacheSize is the maximum number of logs to cache in-memory.
	// This is used to reduce disk I/O for the recently committed entries.
	raftLogCacheSize = 512

	// raftRemoveGracePeriod is how long we wait to allow a RemovePeer
	// to replicate to gracefully leave the cluster.
	raftRemoveGracePeriod = 5 * time.Second

	// defaultConsulDiscoveryInterval is how often to poll Consul for new
	// servers if there is no leader.
	defaultConsulDiscoveryInterval time.Duration = 3 * time.Second

	// defaultConsulDiscoveryIntervalRetry is how often to poll Consul for
	// new servers if there is no leader and the last Consul query failed.
	defaultConsulDiscoveryIntervalRetry time.Duration = 9 * time.Second

	// aclCacheSize is the number of ACL objects to keep cached. ACLs have a parsing and
	// construction cost, so we keep the hot objects cached to reduce the ACL token resolution time.
	aclCacheSize = 512
)

// Server is Nomad server which manages the job queues,
// schedulers, and notification bus for agents.
type Server struct {
	config *Config

	logger log.InterceptLogger

	// Connection pool to other Nomad servers
	connPool *pool.ConnPool

	// The raft instance is used among Nomad nodes within the
	// region to protect operations that require strong consistency
	raft          *raft.Raft
	raftLayer     *RaftLayer
	raftStore     *raftboltdb.BoltStore
	raftInmem     *raft.InmemStore
	raftTransport *raft.NetworkTransport

	// reassertLeaderCh is used to signal that the leader loop must
	// re-establish leadership.
	//
	// This might be relevant in snapshot restores, where leader in-memory
	// state changed significantly such that leader state (e.g. periodic
	// jobs, eval brokers) need to be recomputed.
	reassertLeaderCh chan chan error

	// autopilot is the Autopilot instance for this server.
	autopilot *autopilot.Autopilot

	// fsm is the state machine used with Raft
	fsm *nomadFSM

	// rpcListener is used to listen for incoming connections
	rpcListener net.Listener
	listenerCh  chan struct{}

	// tlsWrap is used to wrap outbound connections using TLS. It should be
	// accessed using the lock.
	tlsWrap     tlsutil.RegionWrapper
	tlsWrapLock sync.RWMutex

	// TODO(alex,hclog): Can I move more into the handler?
	// rpcHandler is used to serve and handle RPCs
	*rpcHandler

	// rpcServer is the static RPC server that is used by the local agent.
	rpcServer *rpc.Server

	// clientRpcAdvertise is the advertised RPC address for Nomad clients to connect
	// to this server
	clientRpcAdvertise net.Addr

	// serverRpcAdvertise is the advertised RPC address for Nomad servers to connect
	// to this server
	serverRpcAdvertise net.Addr

	// rpcTLS is the TLS config for incoming TLS requests
	rpcTLS    *tls.Config
	rpcCancel context.CancelFunc

	// staticEndpoints is the set of static endpoints that can be reused across
	// all RPC connections
	staticEndpoints endpoints

	// streamingRpcs is the registry holding our streaming RPC handlers.
	streamingRpcs *structs.StreamingRpcRegistry

	// nodeConns is the set of multiplexed node connections we have keyed by
	// NodeID
	nodeConns     map[string][]*nodeConnState
	nodeConnsLock sync.RWMutex

	// peers is used to track the known Nomad servers. This is
	// used for region forwarding and clustering.
	peers      map[string][]*serverParts
	localPeers map[raft.ServerAddress]*serverParts
	peerLock   sync.RWMutex

	// serf is the Serf cluster containing only Nomad
	// servers. This is used for multi-region federation
	// and automatic clustering within regions.
	serf *serf.Serf

	// reconcileCh is used to pass events from the serf handler
	// into the leader manager. Mostly used to handle when servers
	// join/leave from the region.
	reconcileCh chan serf.Member

	// used to track when the server is ready to serve consistent reads, updated atomically
	readyForConsistentReads int32

	// eventCh is used to receive events from the serf cluster
	eventCh chan serf.Event

	// BlockedEvals is used to manage evaluations that are blocked on node
	// capacity changes.
	blockedEvals *BlockedEvals

	// deploymentWatcher is used to watch deployments and their allocations and
	// make the required calls to continue to transition the deployment.
	deploymentWatcher *deploymentwatcher.Watcher

	// nodeDrainer is used to drain allocations from nodes.
	nodeDrainer *drainer.NodeDrainer

	// volumeWatcher is used to release volume claims
	volumeWatcher *volumewatcher.Watcher

	// evalBroker is used to manage the in-progress evaluations
	// that are waiting to be brokered to a sub-scheduler
	evalBroker *EvalBroker

	// periodicDispatcher is used to track and create evaluations for periodic jobs.
	periodicDispatcher *PeriodicDispatch

	// planner is used to mange the submitted allocation plans that are waiting
	// to be accessed by the leader
	*planner

	// nodeHeartbeater is used to track expiration times of node heartbeats. If it
	// detects an expired node, the node status is updated to be 'down'.
	*nodeHeartbeater

	// consulCatalog is used for discovering other Nomad Servers via Consul
	consulCatalog consul.CatalogAPI

	// consulConfigEntries is used for managing Consul Configuration Entries.
	consulConfigEntries ConsulConfigsAPI

	// consulACLs is used for managing Consul Service Identity tokens.
	consulACLs ConsulACLsAPI

	// vault is the client for communicating with Vault.
	vault VaultClient

	// Worker used for processing
	workers []*Worker

	// aclCache is used to maintain the parsed ACL objects
	aclCache *lru.TwoQueueCache

	// leaderAcl is the management ACL token that is valid when resolved by the
	// current leader.
	leaderAcl     string
	leaderAclLock sync.Mutex

	// clusterIDLock ensures the server does not try to concurrently establish
	// a cluster ID, racing against itself in calls of ClusterID
	clusterIDLock sync.Mutex

	// statsFetcher is used by autopilot to check the status of the other
	// Nomad router.
	statsFetcher *StatsFetcher

	// eventSinkManager is used by the leader to send events to configured
	// event sinks
	eventSinkManager *SinkManager

	// EnterpriseState is used to fill in state for Pro/Ent builds
	EnterpriseState

	left         bool
	shutdown     bool
	shutdownLock sync.Mutex

	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	shutdownCh     <-chan struct{}
}

// Holds the RPC endpoints
type endpoints struct {
	Status     *Status
	Node       *Node
	Job        *Job
	Eval       *Eval
	Plan       *Plan
	Alloc      *Alloc
	CSIVolume  *CSIVolume
	CSIPlugin  *CSIPlugin
	Deployment *Deployment
	Region     *Region
	Search     *Search
	Periodic   *Periodic
	System     *System
	Operator   *Operator
	ACL        *ACL
	Scaling    *Scaling
	Enterprise *EnterpriseEndpoints
	Event      *Event
	Namespace  *Namespace

	// Client endpoints
	ClientStats       *ClientStats
	FileSystem        *FileSystem
	Agent             *Agent
	ClientAllocations *ClientAllocations
	ClientCSI         *ClientCSI
}

// NewServer is used to construct a new Nomad server from the
// configuration, potentially returning an error
func NewServer(config *Config, consulCatalog consul.CatalogAPI, consulConfigEntries consul.ConfigAPI, consulACLs consul.ACLsAPI) (*Server, error) {
	// Check the protocol version
	if err := config.CheckVersion(); err != nil {
		return nil, err
	}

	// Create an eval broker
	evalBroker, err := NewEvalBroker(
		config.EvalNackTimeout,
		config.EvalNackInitialReenqueueDelay,
		config.EvalNackSubsequentReenqueueDelay,
		config.EvalDeliveryLimit)
	if err != nil {
		return nil, err
	}

	// Configure TLS
	tlsConf, err := tlsutil.NewTLSConfiguration(config.TLSConfig, true, true)
	if err != nil {
		return nil, err
	}
	incomingTLS, tlsWrap, err := getTLSConf(config.TLSConfig.EnableRPC, tlsConf, config.Region)
	if err != nil {
		return nil, err
	}

	// Create the ACL object cache
	aclCache, err := lru.New2Q(aclCacheSize)
	if err != nil {
		return nil, err
	}

	// Create the logger
	logger := config.Logger.ResetNamedIntercept("nomad")

	// Create the server
	s := &Server{
		config:           config,
		consulCatalog:    consulCatalog,
		connPool:         pool.NewPool(logger, serverRPCCache, serverMaxStreams, tlsWrap),
		logger:           logger,
		tlsWrap:          tlsWrap,
		rpcServer:        rpc.NewServer(),
		streamingRpcs:    structs.NewStreamingRpcRegistry(),
		nodeConns:        make(map[string][]*nodeConnState),
		peers:            make(map[string][]*serverParts),
		localPeers:       make(map[raft.ServerAddress]*serverParts),
		reassertLeaderCh: make(chan chan error),
		reconcileCh:      make(chan serf.Member, 32),
		eventCh:          make(chan serf.Event, 256),
		evalBroker:       evalBroker,
		blockedEvals:     NewBlockedEvals(evalBroker, logger),
		rpcTLS:           incomingTLS,
		aclCache:         aclCache,
	}

	s.shutdownCtx, s.shutdownCancel = context.WithCancel(context.Background())
	s.shutdownCh = s.shutdownCtx.Done()

	// Create the RPC handler
	s.rpcHandler = newRpcHandler(s)

	// Create the planner
	planner, err := newPlanner(s)
	if err != nil {
		return nil, err
	}
	s.planner = planner

	// Create the node heartbeater
	s.nodeHeartbeater = newNodeHeartbeater(s)

	// Create the periodic dispatcher for launching periodic jobs.
	s.periodicDispatcher = NewPeriodicDispatch(s.logger, s)

	// Initialize the stats fetcher that autopilot will use.
	s.statsFetcher = NewStatsFetcher(s.logger, s.connPool, s.config.Region)

	// Initialize the event sink manager the leader will use
	s.eventSinkManager = NewSinkManager(s.shutdownCtx, s, s.logger)

	// Setup Consul (more)
	s.setupConsul(consulConfigEntries, consulACLs)

	// Setup Vault
	if err := s.setupVaultClient(); err != nil {
		s.Shutdown()
		s.logger.Error("failed to setup Vault client", "error", err)
		return nil, fmt.Errorf("Failed to setup Vault client: %v", err)
	}

	// Initialize the RPC layer
	if err := s.setupRPC(tlsWrap); err != nil {
		s.Shutdown()
		s.logger.Error("failed to start RPC layer", "error", err)
		return nil, fmt.Errorf("Failed to start RPC layer: %v", err)
	}

	// Initialize the Raft server
	if err := s.setupRaft(); err != nil {
		s.Shutdown()
		s.logger.Error("failed to start Raft", "error", err)
		return nil, fmt.Errorf("Failed to start Raft: %v", err)
	}

	// Initialize the wan Serf
	s.serf, err = s.setupSerf(config.SerfConfig, s.eventCh, serfSnapshot)
	if err != nil {
		s.Shutdown()
		s.logger.Error("failed to start serf WAN", "error", err)
		return nil, fmt.Errorf("Failed to start serf: %v", err)
	}

	// Initialize the scheduling workers
	if err := s.setupWorkers(); err != nil {
		s.Shutdown()
		s.logger.Error("failed to start workers", "error", err)
		return nil, fmt.Errorf("Failed to start workers: %v", err)
	}

	// Setup the Consul syncer
	if err := s.setupConsulSyncer(); err != nil {
		s.logger.Error("failed to create server consul syncer", "error", err)
		return nil, fmt.Errorf("failed to create server Consul syncer: %v", err)
	}

	// Setup the deployment watcher.
	if err := s.setupDeploymentWatcher(); err != nil {
		s.logger.Error("failed to create deployment watcher", "error", err)
		return nil, fmt.Errorf("failed to create deployment watcher: %v", err)
	}

	// Setup the volume watcher
	if err := s.setupVolumeWatcher(); err != nil {
		s.logger.Error("failed to create volume watcher", "error", err)
		return nil, fmt.Errorf("failed to create volume watcher: %v", err)
	}

	// Setup the node drainer.
	s.setupNodeDrainer()

	// Setup the enterprise state
	if err := s.setupEnterprise(config); err != nil {
		return nil, err
	}

	// Monitor leadership changes
	go s.monitorLeadership()

	// Start ingesting events for Serf
	go s.serfEventHandler()

	// start the RPC listener for the server
	s.startRPCListener()

	// Emit metrics for the eval broker
	go evalBroker.EmitStats(time.Second, s.shutdownCh)

	// Emit metrics for the plan queue
	go s.planQueue.EmitStats(time.Second, s.shutdownCh)

	// Emit metrics for the blocked eval tracker.
	go s.blockedEvals.EmitStats(time.Second, s.shutdownCh)

	// Emit metrics for the Vault client.
	go s.vault.EmitStats(time.Second, s.shutdownCh)

	// Emit metrics
	go s.heartbeatStats()

	// Emit raft and state store metrics
	go s.EmitRaftStats(10*time.Second, s.shutdownCh)

	// Start enterprise background workers
	s.startEnterpriseBackground()

	// Done
	return s, nil
}

// startRPCListener starts the server's the RPC listener
func (s *Server) startRPCListener() {
	ctx, cancel := context.WithCancel(context.Background())
	s.rpcCancel = cancel
	go s.listen(ctx)
}

// createRPCListener creates the server's RPC listener
func (s *Server) createRPCListener() (*net.TCPListener, error) {
	s.listenerCh = make(chan struct{})
	listener, err := net.ListenTCP("tcp", s.config.RPCAddr)
	if err != nil {
		s.logger.Error("failed to initialize TLS listener", "error", err)
		return listener, err
	}

	s.rpcListener = listener
	return listener, nil
}

// getTLSConf gets the server's TLS configuration based on the config supplied
// by the operator
func getTLSConf(enableRPC bool, tlsConf *tlsutil.Config, region string) (*tls.Config, tlsutil.RegionWrapper, error) {
	var tlsWrap tlsutil.RegionWrapper
	var incomingTLS *tls.Config
	if !enableRPC {
		return incomingTLS, tlsWrap, nil
	}

	tlsWrap, err := tlsConf.OutgoingTLSWrapper()
	if err != nil {
		return nil, nil, err
	}

	itls, err := tlsConf.IncomingTLSConfig()
	if err != nil {
		return nil, nil, err
	}

	if tlsConf.VerifyServerHostname {
		incomingTLS = itls.Clone()
		incomingTLS.VerifyPeerCertificate = rpcNameAndRegionValidator(region)
	} else {
		incomingTLS = itls
	}
	return incomingTLS, tlsWrap, nil
}

// implements signature of tls.Config.VerifyPeerCertificate which is called
// after the certs have been verified. We'll ignore the raw certs and only
// check the verified certs.
func rpcNameAndRegionValidator(region string) func([][]byte, [][]*x509.Certificate) error {
	return func(_ [][]byte, certificates [][]*x509.Certificate) error {
		if len(certificates) > 0 && len(certificates[0]) > 0 {
			cert := certificates[0][0]
			for _, dnsName := range cert.DNSNames {
				if validateRPCRegionPeer(dnsName, region) {
					return nil
				}
			}
			if validateRPCRegionPeer(cert.Subject.CommonName, region) {
				return nil
			}
		}
		return errors.New("invalid role or region for certificate")
	}
}

func validateRPCRegionPeer(name, region string) bool {
	parts := strings.Split(name, ".")
	if len(parts) < 3 {
		// Invalid SAN
		return false
	}
	if parts[len(parts)-1] != "nomad" {
		// Incorrect service
		return false
	}
	if parts[0] == "client" {
		// Clients may only connect to servers in their region
		return name == "client."+region+".nomad"
	}
	// Servers may connect to any Nomad RPC service for federation.
	return parts[0] == "server"
}

// reloadTLSConnections updates a server's TLS configuration and reloads RPC
// connections.
func (s *Server) reloadTLSConnections(newTLSConfig *config.TLSConfig) error {
	s.logger.Info("reloading server connections due to configuration changes")

	// Check if we can reload the RPC listener
	if s.rpcListener == nil || s.rpcCancel == nil {
		s.logger.Warn("unable to reload configuration due to uninitialized rpc listner")
		return fmt.Errorf("can't reload uninitialized RPC listener")
	}

	tlsConf, err := tlsutil.NewTLSConfiguration(newTLSConfig, true, true)
	if err != nil {
		s.logger.Error("unable to create TLS configuration", "error", err)
		return err
	}

	incomingTLS, tlsWrap, err := getTLSConf(newTLSConfig.EnableRPC, tlsConf, s.config.Region)
	if err != nil {
		s.logger.Error("unable to reset TLS context", "error", err)
		return err
	}

	// Store the new tls wrapper.
	s.tlsWrapLock.Lock()
	s.tlsWrap = tlsWrap
	s.tlsWrapLock.Unlock()

	// Keeping configuration in sync is important for other places that require
	// access to config information, such as rpc.go, where we decide on what kind
	// of network connections to accept depending on the server configuration
	s.config.TLSConfig = newTLSConfig

	// Kill any old listeners
	s.rpcCancel()

	s.rpcTLS = incomingTLS
	s.connPool.ReloadTLS(tlsWrap)

	if err := s.rpcListener.Close(); err != nil {
		s.logger.Error("unable to close rpc listener", "error", err)
		return err
	}

	// Wait for the old listener to exit
	<-s.listenerCh

	// Create the new listener with the update TLS config
	listener, err := s.createRPCListener()
	if err != nil {
		listener.Close()
		return err
	}

	// Start the new RPC listener
	s.startRPCListener()

	// Close and reload existing Raft connections
	wrapper := tlsutil.RegionSpecificWrapper(s.config.Region, tlsWrap)
	s.raftLayer.ReloadTLS(wrapper)
	s.raftTransport.CloseStreams()

	s.logger.Debug("finished reloading server connections")
	return nil
}

// Shutdown is used to shutdown the server
func (s *Server) Shutdown() error {
	s.logger.Info("shutting down server")
	s.shutdownLock.Lock()
	defer s.shutdownLock.Unlock()

	if s.shutdown {
		return nil
	}

	s.shutdown = true
	s.shutdownCancel()

	if s.serf != nil {
		s.serf.Shutdown()
	}

	if s.raft != nil {
		s.raftTransport.Close()
		s.raftLayer.Close()
		future := s.raft.Shutdown()
		if err := future.Error(); err != nil {
			s.logger.Warn("error shutting down raft", "error", err)
		}
		if s.raftStore != nil {
			s.raftStore.Close()
		}
	}

	// Shutdown the RPC listener
	if s.rpcListener != nil {
		s.rpcListener.Close()
	}

	// Close the connection pool
	s.connPool.Shutdown()

	// Close the fsm
	if s.fsm != nil {
		s.fsm.Close()
	}

	// Stop Vault token renewal and revocations
	if s.vault != nil {
		s.vault.Stop()
	}

	// Stop the Consul ACLs token revocations
	s.consulACLs.Stop()

	// Stop being able to set Configuration Entries
	s.consulConfigEntries.Stop()

	return nil
}

// IsShutdown checks if the server is shutdown
func (s *Server) IsShutdown() bool {
	select {
	case <-s.shutdownCh:
		return true
	default:
		return false
	}
}

// Leave is used to prepare for a graceful shutdown of the server
func (s *Server) Leave() error {
	s.logger.Info("server starting leave")
	s.left = true

	// Check the number of known peers
	numPeers, err := s.numPeers()
	if err != nil {
		s.logger.Error("failed to check raft peers during leave", "error", err)
		return err
	}

	addr := s.raftTransport.LocalAddr()

	// If we are the current leader, and we have any other peers (cluster has multiple
	// servers), we should do a RemovePeer to safely reduce the quorum size. If we are
	// not the leader, then we should issue our leave intention and wait to be removed
	// for some sane period of time.
	isLeader := s.IsLeader()
	if isLeader && numPeers > 1 {
		minRaftProtocol, err := s.autopilot.MinRaftProtocol()
		if err != nil {
			return err
		}

		if minRaftProtocol >= 2 && s.config.RaftConfig.ProtocolVersion >= 3 {
			future := s.raft.RemoveServer(raft.ServerID(s.config.NodeID), 0, 0)
			if err := future.Error(); err != nil {
				s.logger.Error("failed to remove ourself as raft peer", "error", err)
			}
		} else {
			future := s.raft.RemovePeer(addr)
			if err := future.Error(); err != nil {
				s.logger.Error("failed to remove ourself as raft peer", "error", err)
			}
		}
	}

	// Leave the gossip pool
	if s.serf != nil {
		if err := s.serf.Leave(); err != nil {
			s.logger.Error("failed to leave Serf cluster", "error", err)
		}
	}

	// If we were not leader, wait to be safely removed from the cluster.
	// We must wait to allow the raft replication to take place, otherwise
	// an immediate shutdown could cause a loss of quorum.
	if !isLeader {
		left := false
		limit := time.Now().Add(raftRemoveGracePeriod)
		for !left && time.Now().Before(limit) {
			// Sleep a while before we check.
			time.Sleep(50 * time.Millisecond)

			// Get the latest configuration.
			future := s.raft.GetConfiguration()
			if err := future.Error(); err != nil {
				s.logger.Error("failed to get raft configuration", "error", err)
				break
			}

			// See if we are no longer included.
			left = true
			for _, server := range future.Configuration().Servers {
				if server.Address == addr {
					left = false
					break
				}
			}
		}

		// TODO (alexdadgar) With the old Raft library we used to force the
		// peers set to empty when a graceful leave occurred. This would
		// keep voting spam down if the server was restarted, but it was
		// dangerous because the peers was inconsistent with the logs and
		// snapshots, so it wasn't really safe in all cases for the server
		// to become leader. This is now safe, but the log spam is noisy.
		// The next new version of the library will have a "you are not a
		// peer stop it" behavior that should address this. We will have
		// to evaluate during the RC period if this interim situation is
		// not too confusing for operators.

		// TODO (alexdadgar) When we take a later new version of the Raft
		// library it won't try to complete replication, so this peer
		// may not realize that it has been removed. Need to revisit this
		// and the warning here.
		if !left {
			s.logger.Warn("failed to leave raft configuration gracefully, timeout")
		}
	}
	return nil
}

// Reload handles a config reload specific to server-only configuration. Not
// all config fields can handle a reload.
func (s *Server) Reload(newConfig *Config) error {
	if newConfig == nil {
		return fmt.Errorf("Reload given a nil config")
	}

	var mErr multierror.Error

	// Handle the Vault reload. Vault should never be nil but just guard.
	if s.vault != nil {
		if err := s.vault.SetConfig(newConfig.VaultConfig); err != nil {
			multierror.Append(&mErr, err)
		}
	}

	shouldReloadTLS, err := tlsutil.ShouldReloadRPCConnections(s.config.TLSConfig, newConfig.TLSConfig)
	if err != nil {
		s.logger.Error("error checking whether to reload TLS configuration", "error", err)
	}

	if shouldReloadTLS {
		if err := s.reloadTLSConnections(newConfig.TLSConfig); err != nil {
			s.logger.Error("error reloading server TLS configuration", "error", err)
			multierror.Append(&mErr, err)
		}
	}

	return mErr.ErrorOrNil()
}

// setupBootstrapHandler() creates the closure necessary to support a Consul
// fallback handler.
func (s *Server) setupBootstrapHandler() error {
	// peersTimeout is used to indicate to the Consul Syncer that the
	// current Nomad Server has a stale peer set.  peersTimeout will time
	// out if the Consul Syncer bootstrapFn has not observed a Raft
	// leader in maxStaleLeadership.  If peersTimeout has been triggered,
	// the Consul Syncer will begin querying Consul for other Nomad
	// Servers.
	//
	// NOTE: time.Timer is used vs time.Time in order to handle clock
	// drift because time.Timer is implemented as a monotonic clock.
	var peersTimeout *time.Timer = time.NewTimer(0)

	// consulQueryCount is the number of times the bootstrapFn has been
	// called, regardless of success.
	var consulQueryCount uint64

	// leadershipTimedOut is a helper method that returns true if the
	// peersTimeout timer has expired.
	leadershipTimedOut := func() bool {
		select {
		case <-peersTimeout.C:
			return true
		default:
			return false
		}
	}

	// The bootstrapFn callback handler is used to periodically poll
	// Consul to look up the Nomad Servers in Consul.  In the event the
	// server has been brought up without a `retry-join` configuration
	// and this Server is partitioned from the rest of the cluster,
	// periodically poll Consul to reattach this Server to other servers
	// in the same region and automatically reform a quorum (assuming the
	// correct number of servers required for quorum are present).
	bootstrapFn := func() error {
		// If there is a raft leader, do nothing
		if s.raft.Leader() != "" {
			peersTimeout.Reset(maxStaleLeadership)
			return nil
		}

		// (ab)use serf.go's behavior of setting BootstrapExpect to
		// zero if we have bootstrapped.  If we have bootstrapped
		bootstrapExpect := s.config.BootstrapExpect
		if bootstrapExpect == 0 {
			// This Nomad Server has been bootstrapped.  Rely on
			// the peersTimeout firing as a guard to prevent
			// aggressive querying of Consul.
			if !leadershipTimedOut() {
				return nil
			}
		} else {
			if consulQueryCount > 0 && !leadershipTimedOut() {
				return nil
			}

			// This Nomad Server has not been bootstrapped, reach
			// out to Consul if our peer list is less than
			// `bootstrap_expect`.
			raftPeers, err := s.numPeers()
			if err != nil {
				peersTimeout.Reset(peersPollInterval + lib.RandomStagger(peersPollInterval/peersPollJitterFactor))
				return nil
			}

			// The necessary number of Nomad Servers required for
			// quorum has been reached, we do not need to poll
			// Consul.  Let the normal timeout-based strategy
			// take over.
			if raftPeers >= bootstrapExpect {
				peersTimeout.Reset(peersPollInterval + lib.RandomStagger(peersPollInterval/peersPollJitterFactor))
				return nil
			}
		}
		consulQueryCount++

		s.logger.Debug("lost contact with Nomad quorum, falling back to Consul for server list")

		dcs, err := s.consulCatalog.Datacenters()
		if err != nil {
			peersTimeout.Reset(peersPollInterval + lib.RandomStagger(peersPollInterval/peersPollJitterFactor))
			return fmt.Errorf("server.nomad: unable to query Consul datacenters: %v", err)
		}
		if len(dcs) > 2 {
			// Query the local DC first, then shuffle the
			// remaining DCs.  If additional calls to bootstrapFn
			// are necessary, this Nomad Server will eventually
			// walk all datacenter until it finds enough hosts to
			// form a quorum.
			shuffleStrings(dcs[1:])
			dcs = dcs[0:lib.MinInt(len(dcs), datacenterQueryLimit)]
		}

		nomadServerServiceName := s.config.ConsulConfig.ServerServiceName
		var mErr multierror.Error
		const defaultMaxNumNomadServers = 8
		nomadServerServices := make([]string, 0, defaultMaxNumNomadServers)
		localNode := s.serf.Memberlist().LocalNode()
		for _, dc := range dcs {
			consulOpts := &consulapi.QueryOptions{
				AllowStale: true,
				Datacenter: dc,
				Near:       "_agent",
				WaitTime:   consul.DefaultQueryWaitDuration,
			}
			consulServices, _, err := s.consulCatalog.Service(nomadServerServiceName, consul.ServiceTagSerf, consulOpts)
			if err != nil {
				err := fmt.Errorf("failed to query service %q in Consul datacenter %q: %v", nomadServerServiceName, dc, err)
				s.logger.Warn("failed to query Nomad service in Consul datacenter", "service_name", nomadServerServiceName, "dc", dc, "error", err)
				mErr.Errors = append(mErr.Errors, err)
				continue
			}

			for _, cs := range consulServices {
				port := strconv.FormatInt(int64(cs.ServicePort), 10)
				addr := cs.ServiceAddress
				if addr == "" {
					addr = cs.Address
				}
				if localNode.Addr.String() == addr && int(localNode.Port) == cs.ServicePort {
					continue
				}
				serverAddr := net.JoinHostPort(addr, port)
				nomadServerServices = append(nomadServerServices, serverAddr)
			}
		}

		if len(nomadServerServices) == 0 {
			if len(mErr.Errors) > 0 {
				peersTimeout.Reset(peersPollInterval + lib.RandomStagger(peersPollInterval/peersPollJitterFactor))
				return mErr.ErrorOrNil()
			}

			// Log the error and return nil so future handlers
			// can attempt to register the `nomad` service.
			pollInterval := peersPollInterval + lib.RandomStagger(peersPollInterval/peersPollJitterFactor)
			s.logger.Trace("no Nomad Servers advertising Nomad service in Consul datacenters", "service_name", nomadServerServiceName, "datacenters", dcs, "retry", pollInterval)
			peersTimeout.Reset(pollInterval)
			return nil
		}

		numServersContacted, err := s.Join(nomadServerServices)
		if err != nil {
			peersTimeout.Reset(peersPollInterval + lib.RandomStagger(peersPollInterval/peersPollJitterFactor))
			return fmt.Errorf("contacted %d Nomad Servers: %v", numServersContacted, err)
		}

		peersTimeout.Reset(maxStaleLeadership)
		s.logger.Info("successfully contacted Nomad servers", "num_servers", numServersContacted)

		return nil
	}

	// Hacky replacement for old ConsulSyncer Periodic Handler.
	go func() {
		lastOk := true
		sync := time.NewTimer(0)
		for {
			select {
			case <-sync.C:
				d := defaultConsulDiscoveryInterval
				if err := bootstrapFn(); err != nil {
					// Only log if it worked last time
					if lastOk {
						lastOk = false
						s.logger.Error("error looking up Nomad servers in Consul", "error", err)
					}
					d = defaultConsulDiscoveryIntervalRetry
				}
				sync.Reset(d)
			case <-s.shutdownCh:
				return
			}
		}
	}()
	return nil
}

// setupConsulSyncer creates Server-mode consul.Syncer which periodically
// executes callbacks on a fixed interval.
func (s *Server) setupConsulSyncer() error {
	if s.config.ConsulConfig.ServerAutoJoin != nil && *s.config.ConsulConfig.ServerAutoJoin {
		if err := s.setupBootstrapHandler(); err != nil {
			return err
		}
	}

	return nil
}

// setupDeploymentWatcher creates a deployment watcher that consumes the RPC
// endpoints for state information and makes transitions via Raft through a
// shim that provides the appropriate methods.
func (s *Server) setupDeploymentWatcher() error {

	// Create the raft shim type to restrict the set of raft methods that can be
	// made
	raftShim := &deploymentWatcherRaftShim{
		apply: s.raftApply,
	}

	// Create the deployment watcher
	s.deploymentWatcher = deploymentwatcher.NewDeploymentsWatcher(
		s.logger,
		raftShim,
		s.staticEndpoints.Deployment,
		s.staticEndpoints.Job,
		deploymentwatcher.LimitStateQueriesPerSecond,
		deploymentwatcher.CrossDeploymentUpdateBatchDuration,
	)

	return nil
}

// setupVolumeWatcher creates a volume watcher that sends CSI RPCs
func (s *Server) setupVolumeWatcher() error {
	s.volumeWatcher = volumewatcher.NewVolumesWatcher(
		s.logger, s.staticEndpoints.CSIVolume, s.getLeaderAcl())

	return nil
}

// setupNodeDrainer creates a node drainer which will be enabled when a server
// becomes a leader.
func (s *Server) setupNodeDrainer() {
	// Create a shim around Raft requests
	shim := drainerShim{s}
	c := &drainer.NodeDrainerConfig{
		Logger:                s.logger,
		Raft:                  shim,
		JobFactory:            drainer.GetDrainingJobWatcher,
		NodeFactory:           drainer.GetNodeWatcherFactory(),
		DrainDeadlineFactory:  drainer.GetDeadlineNotifier,
		StateQueriesPerSecond: drainer.LimitStateQueriesPerSecond,
		BatchUpdateInterval:   drainer.BatchUpdateInterval,
	}
	s.nodeDrainer = drainer.NewNodeDrainer(c)
}

// setupConsul is used to setup Server specific consul components.
func (s *Server) setupConsul(consulConfigEntries consul.ConfigAPI, consulACLs consul.ACLsAPI) {
	s.consulConfigEntries = NewConsulConfigsAPI(consulConfigEntries, s.logger)
	s.consulACLs = NewConsulACLsAPI(consulACLs, s.logger, s.purgeSITokenAccessors)
}

// setupVaultClient is used to set up the Vault API client.
func (s *Server) setupVaultClient() error {
	delegate := s.entVaultDelegate()
	v, err := NewVaultClient(s.config.VaultConfig, s.logger, s.purgeVaultAccessors, delegate)
	if err != nil {
		return err
	}
	s.vault = v
	return nil
}

// setupRPC is used to setup the RPC listener
func (s *Server) setupRPC(tlsWrap tlsutil.RegionWrapper) error {
	// Populate the static RPC server
	s.setupRpcServer(s.rpcServer, nil)

	listener, err := s.createRPCListener()
	if err != nil {
		listener.Close()
		return err
	}

	if s.config.ClientRPCAdvertise != nil {
		s.clientRpcAdvertise = s.config.ClientRPCAdvertise
	} else {
		s.clientRpcAdvertise = s.rpcListener.Addr()
	}

	// Verify that we have a usable advertise address
	clientAddr, ok := s.clientRpcAdvertise.(*net.TCPAddr)
	if !ok {
		listener.Close()
		return fmt.Errorf("Client RPC advertise address is not a TCP Address: %v", clientAddr)
	}
	if clientAddr.IP.IsUnspecified() {
		listener.Close()
		return fmt.Errorf("Client RPC advertise address is not advertisable: %v", clientAddr)
	}

	if s.config.ServerRPCAdvertise != nil {
		s.serverRpcAdvertise = s.config.ServerRPCAdvertise
	} else {
		// Default to the Serf Advertise + RPC Port
		serfIP := s.config.SerfConfig.MemberlistConfig.AdvertiseAddr
		if serfIP == "" {
			serfIP = s.config.SerfConfig.MemberlistConfig.BindAddr
		}

		addr := net.JoinHostPort(serfIP, fmt.Sprintf("%d", clientAddr.Port))
		resolved, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return fmt.Errorf("Failed to resolve Server RPC advertise address: %v", err)
		}

		s.serverRpcAdvertise = resolved
	}

	// Verify that we have a usable advertise address
	serverAddr, ok := s.serverRpcAdvertise.(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("Server RPC advertise address is not a TCP Address: %v", serverAddr)
	}
	if serverAddr.IP.IsUnspecified() {
		listener.Close()
		return fmt.Errorf("Server RPC advertise address is not advertisable: %v", serverAddr)
	}

	wrapper := tlsutil.RegionSpecificWrapper(s.config.Region, tlsWrap)
	s.raftLayer = NewRaftLayer(s.serverRpcAdvertise, wrapper)
	return nil
}

// setupRpcServer is used to populate an RPC server with endpoints
func (s *Server) setupRpcServer(server *rpc.Server, ctx *RPCContext) {
	// Add the static endpoints to the RPC server.
	if s.staticEndpoints.Status == nil {
		// Initialize the list just once
		s.staticEndpoints.ACL = &ACL{srv: s, logger: s.logger.Named("acl")}
		s.staticEndpoints.Alloc = &Alloc{srv: s, logger: s.logger.Named("alloc")}
		s.staticEndpoints.Eval = &Eval{srv: s, logger: s.logger.Named("eval")}
		s.staticEndpoints.Job = NewJobEndpoints(s)
		s.staticEndpoints.Node = &Node{srv: s, logger: s.logger.Named("client")} // Add but don't register
		s.staticEndpoints.CSIVolume = &CSIVolume{srv: s, logger: s.logger.Named("csi_volume")}
		s.staticEndpoints.CSIPlugin = &CSIPlugin{srv: s, logger: s.logger.Named("csi_plugin")}
		s.staticEndpoints.Deployment = &Deployment{srv: s, logger: s.logger.Named("deployment")}
		s.staticEndpoints.Operator = &Operator{srv: s, logger: s.logger.Named("operator")}
		s.staticEndpoints.Operator.register()

		s.staticEndpoints.Periodic = &Periodic{srv: s, logger: s.logger.Named("periodic")}
		s.staticEndpoints.Plan = &Plan{srv: s, logger: s.logger.Named("plan")}
		s.staticEndpoints.Region = &Region{srv: s, logger: s.logger.Named("region")}
		s.staticEndpoints.Scaling = &Scaling{srv: s, logger: s.logger.Named("scaling")}
		s.staticEndpoints.Status = &Status{srv: s, logger: s.logger.Named("status")}
		s.staticEndpoints.System = &System{srv: s, logger: s.logger.Named("system")}
		s.staticEndpoints.Search = &Search{srv: s, logger: s.logger.Named("search")}
		s.staticEndpoints.Namespace = &Namespace{srv: s}
		s.staticEndpoints.Enterprise = NewEnterpriseEndpoints(s)

		// Client endpoints
		s.staticEndpoints.ClientStats = &ClientStats{srv: s, logger: s.logger.Named("client_stats")}
		s.staticEndpoints.ClientAllocations = &ClientAllocations{srv: s, logger: s.logger.Named("client_allocs")}
		s.staticEndpoints.ClientAllocations.register()
		s.staticEndpoints.ClientCSI = &ClientCSI{srv: s, logger: s.logger.Named("client_csi")}

		// Streaming endpoints
		s.staticEndpoints.FileSystem = &FileSystem{srv: s, logger: s.logger.Named("client_fs")}
		s.staticEndpoints.FileSystem.register()

		s.staticEndpoints.Agent = &Agent{srv: s}
		s.staticEndpoints.Agent.register()

		s.staticEndpoints.Event = &Event{srv: s}
		s.staticEndpoints.Event.register()

	}

	// Register the static handlers
	server.Register(s.staticEndpoints.ACL)
	server.Register(s.staticEndpoints.Alloc)
	server.Register(s.staticEndpoints.Eval)
	server.Register(s.staticEndpoints.Job)
	server.Register(s.staticEndpoints.CSIVolume)
	server.Register(s.staticEndpoints.CSIPlugin)
	server.Register(s.staticEndpoints.Deployment)
	server.Register(s.staticEndpoints.Operator)
	server.Register(s.staticEndpoints.Periodic)
	server.Register(s.staticEndpoints.Plan)
	server.Register(s.staticEndpoints.Region)
	server.Register(s.staticEndpoints.Scaling)
	server.Register(s.staticEndpoints.Status)
	server.Register(s.staticEndpoints.System)
	server.Register(s.staticEndpoints.Search)
	s.staticEndpoints.Enterprise.Register(server)
	server.Register(s.staticEndpoints.ClientStats)
	server.Register(s.staticEndpoints.ClientAllocations)
	server.Register(s.staticEndpoints.ClientCSI)
	server.Register(s.staticEndpoints.FileSystem)
	server.Register(s.staticEndpoints.Agent)
	server.Register(s.staticEndpoints.Namespace)
	server.Register(s.staticEndpoints.Event)

	// Create new dynamic endpoints and add them to the RPC server.
	node := &Node{srv: s, ctx: ctx, logger: s.logger.Named("client")}

	// Register the dynamic endpoints
	server.Register(node)
}

// setupRaft is used to setup and initialize Raft
func (s *Server) setupRaft() error {
	// If we have an unclean exit then attempt to close the Raft store.
	defer func() {
		if s.raft == nil && s.raftStore != nil {
			if err := s.raftStore.Close(); err != nil {
				s.logger.Error("failed to close Raft store", "error", err)
			}
		}
	}()

	// Create the FSM
	fsmConfig := &FSMConfig{
		EvalBroker:        s.evalBroker,
		Periodic:          s.periodicDispatcher,
		Blocked:           s.blockedEvals,
		Logger:            s.logger,
		Region:            s.Region(),
		EnableEventBroker: s.config.EnableEventBroker,
		EventBufferSize:   s.config.EventBufferSize,
	}
	var err error
	s.fsm, err = NewFSM(fsmConfig)
	if err != nil {
		return err
	}

	// Create a transport layer
	trans := raft.NewNetworkTransport(s.raftLayer, 3, s.config.RaftTimeout,
		s.config.LogOutput)
	s.raftTransport = trans

	// Make sure we set the Logger.
	s.config.RaftConfig.Logger = s.logger.Named("raft")
	s.config.RaftConfig.LogOutput = nil

	// Our version of Raft protocol 2 requires the LocalID to match the network
	// address of the transport. Raft protocol 3 uses permanent ids.
	s.config.RaftConfig.LocalID = raft.ServerID(trans.LocalAddr())
	if s.config.RaftConfig.ProtocolVersion >= 3 {
		s.config.RaftConfig.LocalID = raft.ServerID(s.config.NodeID)
	}

	// Build an all in-memory setup for dev mode, otherwise prepare a full
	// disk-based setup.
	var log raft.LogStore
	var stable raft.StableStore
	var snap raft.SnapshotStore
	if s.config.DevMode {
		store := raft.NewInmemStore()
		s.raftInmem = store
		stable = store
		log = store
		snap = raft.NewDiscardSnapshotStore()

	} else {
		// Create the base raft path
		path := filepath.Join(s.config.DataDir, raftState)
		if err := ensurePath(path, true); err != nil {
			return err
		}

		// Create the BoltDB backend
		store, err := raftboltdb.NewBoltStore(filepath.Join(path, "raft.db"))
		if err != nil {
			return err
		}
		s.raftStore = store
		stable = store

		// Wrap the store in a LogCache to improve performance
		cacheStore, err := raft.NewLogCache(raftLogCacheSize, store)
		if err != nil {
			store.Close()
			return err
		}
		log = cacheStore

		// Create the snapshot store
		snapshots, err := raft.NewFileSnapshotStore(path, snapshotsRetained, s.config.LogOutput)
		if err != nil {
			if s.raftStore != nil {
				s.raftStore.Close()
			}
			return err
		}
		snap = snapshots

		// For an existing cluster being upgraded to the new version of
		// Raft, we almost never want to run recovery based on the old
		// peers.json file. We create a peers.info file with a helpful
		// note about where peers.json went, and use that as a sentinel
		// to avoid ingesting the old one that first time (if we have to
		// create the peers.info file because it's not there, we also
		// blow away any existing peers.json file).
		peersFile := filepath.Join(path, "peers.json")
		peersInfoFile := filepath.Join(path, "peers.info")
		if _, err := os.Stat(peersInfoFile); os.IsNotExist(err) {
			if err := ioutil.WriteFile(peersInfoFile, []byte(peersInfoContent), 0755); err != nil {
				return fmt.Errorf("failed to write peers.info file: %v", err)
			}

			// Blow away the peers.json file if present, since the
			// peers.info sentinel wasn't there.
			if _, err := os.Stat(peersFile); err == nil {
				if err := os.Remove(peersFile); err != nil {
					return fmt.Errorf("failed to delete peers.json, please delete manually (see peers.info for details): %v", err)
				}
				s.logger.Info("deleted peers.json file (see peers.info for details)")
			}
		} else if _, err := os.Stat(peersFile); err == nil {
			s.logger.Info("found peers.json file, recovering Raft configuration...")
			var configuration raft.Configuration
			if s.config.RaftConfig.ProtocolVersion < 3 {
				configuration, err = raft.ReadPeersJSON(peersFile)
			} else {
				configuration, err = raft.ReadConfigJSON(peersFile)
			}
			if err != nil {
				return fmt.Errorf("recovery failed to parse peers.json: %v", err)
			}
			tmpFsm, err := NewFSM(fsmConfig)
			if err != nil {
				return fmt.Errorf("recovery failed to make temp FSM: %v", err)
			}
			if err := raft.RecoverCluster(s.config.RaftConfig, tmpFsm,
				log, stable, snap, trans, configuration); err != nil {
				return fmt.Errorf("recovery failed: %v", err)
			}
			if err := os.Remove(peersFile); err != nil {
				return fmt.Errorf("recovery failed to delete peers.json, please delete manually (see peers.info for details): %v", err)
			}
			s.logger.Info("deleted peers.json file after successful recovery")
		}
	}

	// If we are a single server cluster and the state is clean then we can
	// bootstrap now.
	if s.isSingleServerCluster() {
		hasState, err := raft.HasExistingState(log, stable, snap)
		if err != nil {
			return err
		}
		if !hasState {
			configuration := raft.Configuration{
				Servers: []raft.Server{
					{
						ID:      s.config.RaftConfig.LocalID,
						Address: trans.LocalAddr(),
					},
				},
			}
			if err := raft.BootstrapCluster(s.config.RaftConfig,
				log, stable, snap, trans, configuration); err != nil {
				return err
			}
		}
	}

	// Setup the Raft store
	s.raft, err = raft.NewRaft(s.config.RaftConfig, s.fsm, log, stable, snap, trans)
	if err != nil {
		return err
	}
	return nil
}

// setupSerf is used to setup and initialize a Serf
func (s *Server) setupSerf(conf *serf.Config, ch chan serf.Event, path string) (*serf.Serf, error) {
	conf.Init()
	conf.NodeName = fmt.Sprintf("%s.%s", s.config.NodeName, s.config.Region)
	conf.Tags["role"] = "nomad"
	conf.Tags["region"] = s.config.Region
	conf.Tags["dc"] = s.config.Datacenter
	conf.Tags["vsn"] = fmt.Sprintf("%d", structs.ApiMajorVersion)
	conf.Tags["mvn"] = fmt.Sprintf("%d", structs.ApiMinorVersion)
	conf.Tags["build"] = s.config.Build
	conf.Tags["raft_vsn"] = fmt.Sprintf("%d", s.config.RaftConfig.ProtocolVersion)
	conf.Tags["id"] = s.config.NodeID
	conf.Tags["rpc_addr"] = s.clientRpcAdvertise.(*net.TCPAddr).IP.String()         // Address that clients will use to RPC to servers
	conf.Tags["port"] = fmt.Sprintf("%d", s.serverRpcAdvertise.(*net.TCPAddr).Port) // Port servers use to RPC to one and another
	if s.isSingleServerCluster() {
		conf.Tags["bootstrap"] = "1"
	}
	bootstrapExpect := s.config.BootstrapExpect
	if bootstrapExpect != 0 {
		conf.Tags["expect"] = fmt.Sprintf("%d", bootstrapExpect)
	}
	if s.config.NonVoter {
		conf.Tags["nonvoter"] = "1"
	}
	if s.config.RedundancyZone != "" {
		conf.Tags[AutopilotRZTag] = s.config.RedundancyZone
	}
	if s.config.UpgradeVersion != "" {
		conf.Tags[AutopilotVersionTag] = s.config.UpgradeVersion
	}
	logger := s.logger.StandardLoggerIntercept(&log.StandardLoggerOptions{InferLevels: true})
	conf.MemberlistConfig.Logger = logger
	conf.Logger = logger
	conf.MemberlistConfig.LogOutput = nil
	conf.LogOutput = nil
	conf.EventCh = ch
	if !s.config.DevMode {
		conf.SnapshotPath = filepath.Join(s.config.DataDir, path)
		if err := ensurePath(conf.SnapshotPath, false); err != nil {
			return nil, err
		}
	}
	conf.ProtocolVersion = protocolVersionMap[s.config.ProtocolVersion]
	conf.RejoinAfterLeave = true
	// LeavePropagateDelay is used to make sure broadcasted leave intents propagate
	// This value was tuned using https://www.serf.io/docs/internals/simulator.html to
	// allow for convergence in 99.9% of nodes in a 10 node cluster
	conf.LeavePropagateDelay = 1 * time.Second
	conf.Merge = &serfMergeDelegate{}

	// Until Nomad supports this fully, we disable automatic resolution.
	// When enabled, the Serf gossip may just turn off if we are the minority
	// node which is rather unexpected.
	conf.EnableNameConflictResolution = false
	return serf.Create(conf)
}

// setupWorkers is used to start the scheduling workers
func (s *Server) setupWorkers() error {
	// Check if all the schedulers are disabled
	if len(s.config.EnabledSchedulers) == 0 || s.config.NumSchedulers == 0 {
		s.logger.Warn("no enabled schedulers")
		return nil
	}

	// Check if the core scheduler is not enabled
	foundCore := false
	for _, sched := range s.config.EnabledSchedulers {
		if sched == structs.JobTypeCore {
			foundCore = true
			continue
		}

		if _, ok := scheduler.BuiltinSchedulers[sched]; !ok {
			return fmt.Errorf("invalid configuration: unknown scheduler %q in enabled schedulers", sched)
		}
	}
	if !foundCore {
		return fmt.Errorf("invalid configuration: %q scheduler not enabled", structs.JobTypeCore)
	}

	// Start the workers
	for i := 0; i < s.config.NumSchedulers; i++ {
		if w, err := NewWorker(s); err != nil {
			return err
		} else {
			s.workers = append(s.workers, w)
		}
	}
	s.logger.Info("starting scheduling worker(s)", "num_workers", s.config.NumSchedulers, "schedulers", s.config.EnabledSchedulers)
	return nil
}

// numPeers is used to check on the number of known peers, including the local
// node.
func (s *Server) numPeers() (int, error) {
	future := s.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return 0, err
	}
	configuration := future.Configuration()
	return len(configuration.Servers), nil
}

// IsLeader checks if this server is the cluster leader
func (s *Server) IsLeader() bool {
	return s.raft.State() == raft.Leader
}

// Join is used to have Nomad join the gossip ring
// The target address should be another node listening on the
// Serf address
func (s *Server) Join(addrs []string) (int, error) {
	return s.serf.Join(addrs, true)
}

// LocalMember is used to return the local node
func (s *Server) LocalMember() serf.Member {
	return s.serf.LocalMember()
}

// Members is used to return the members of the serf cluster
func (s *Server) Members() []serf.Member {
	return s.serf.Members()
}

// RemoveFailedNode is used to remove a failed node from the cluster
func (s *Server) RemoveFailedNode(node string) error {
	return s.serf.RemoveFailedNode(node)
}

// KeyManager returns the Serf keyring manager
func (s *Server) KeyManager() *serf.KeyManager {
	return s.serf.KeyManager()
}

// Encrypted determines if gossip is encrypted
func (s *Server) Encrypted() bool {
	return s.serf.EncryptionEnabled()
}

// State returns the underlying state store. This should *not*
// be used to modify state directly.
func (s *Server) State() *state.StateStore {
	return s.fsm.State()
}

// setLeaderAcl stores the given ACL token as the current leader's ACL token.
func (s *Server) setLeaderAcl(token string) {
	s.leaderAclLock.Lock()
	s.leaderAcl = token
	s.leaderAclLock.Unlock()
}

// getLeaderAcl retrieves the leader's ACL token
func (s *Server) getLeaderAcl() string {
	s.leaderAclLock.Lock()
	defer s.leaderAclLock.Unlock()
	return s.leaderAcl
}

// Atomically sets a readiness state flag when leadership is obtained, to indicate that server is past its barrier write
func (s *Server) setConsistentReadReady() {
	atomic.StoreInt32(&s.readyForConsistentReads, 1)
}

// Atomically reset readiness state flag on leadership revoke
func (s *Server) resetConsistentReadReady() {
	atomic.StoreInt32(&s.readyForConsistentReads, 0)
}

// Returns true if this server is ready to serve consistent reads
func (s *Server) isReadyForConsistentReads() bool {
	return atomic.LoadInt32(&s.readyForConsistentReads) == 1
}

// Regions returns the known regions in the cluster.
func (s *Server) Regions() []string {
	s.peerLock.RLock()
	defer s.peerLock.RUnlock()

	regions := make([]string, 0, len(s.peers))
	for region := range s.peers {
		regions = append(regions, region)
	}
	sort.Strings(regions)
	return regions
}

// RPC is used to make a local RPC call
func (s *Server) RPC(method string, args interface{}, reply interface{}) error {
	codec := &codec.InmemCodec{
		Method: method,
		Args:   args,
		Reply:  reply,
	}
	if err := s.rpcServer.ServeRequest(codec); err != nil {
		return err
	}
	return codec.Err
}

// StreamingRpcHandler is used to make a streaming RPC call.
func (s *Server) StreamingRpcHandler(method string) (structs.StreamingRpcHandler, error) {
	return s.streamingRpcs.GetHandler(method)
}

// Stats is used to return statistics for debugging and insight
// for various sub-systems
func (s *Server) Stats() map[string]map[string]string {
	toString := func(v uint64) string {
		return strconv.FormatUint(v, 10)
	}
	stats := map[string]map[string]string{
		"nomad": {
			"server":        "true",
			"leader":        fmt.Sprintf("%v", s.IsLeader()),
			"leader_addr":   string(s.raft.Leader()),
			"bootstrap":     fmt.Sprintf("%v", s.isSingleServerCluster()),
			"known_regions": toString(uint64(len(s.peers))),
		},
		"raft":    s.raft.Stats(),
		"serf":    s.serf.Stats(),
		"runtime": stats.RuntimeStats(),
		"vault":   s.vault.Stats(),
	}

	return stats
}

// EmitRaftStats is used to export metrics about raft indexes and state store snapshot index
func (s *Server) EmitRaftStats(period time.Duration, stopCh <-chan struct{}) {
	for {
		select {
		case <-time.After(period):
			lastIndex := s.raft.LastIndex()
			metrics.SetGauge([]string{"raft", "lastIndex"}, float32(lastIndex))
			appliedIndex := s.raft.AppliedIndex()
			metrics.SetGauge([]string{"raft", "appliedIndex"}, float32(appliedIndex))
			stateStoreSnapshotIndex, err := s.State().LatestIndex()
			if err != nil {
				s.logger.Warn("Unable to read snapshot index from statestore, metric will not be emitted", "error", err)
			} else {
				metrics.SetGauge([]string{"state", "snapshotIndex"}, float32(stateStoreSnapshotIndex))
			}
		case <-stopCh:
			return
		}
	}
}

// Region returns the region of the server
func (s *Server) Region() string {
	return s.config.Region
}

// Datacenter returns the data center of the server
func (s *Server) Datacenter() string {
	return s.config.Datacenter
}

// GetConfig returns the config of the server for testing purposes only
func (s *Server) GetConfig() *Config {
	return s.config
}

// ReplicationToken returns the token used for replication. We use a method to support
// dynamic reloading of this value later.
func (s *Server) ReplicationToken() string {
	return s.config.ReplicationToken
}

// ClusterID returns the unique ID for this cluster.
//
// Any Nomad server agent may call this method to get at the ID.
// If we are the leader and the ID has not yet been created, it will
// be created now. Otherwise an error is returned.
//
// The ID will not be created until all participating servers have reached
// a minimum version (0.10.4).
func (s *Server) ClusterID() (string, error) {
	s.clusterIDLock.Lock()
	defer s.clusterIDLock.Unlock()

	// try to load the cluster ID from state store
	fsmState := s.fsm.State()
	existingMeta, err := fsmState.ClusterMetadata(nil)
	if err != nil {
		s.logger.Named("core").Error("failed to get cluster ID", "error", err)
		return "", err
	}

	// got the cluster ID from state store, cache that and return it
	if existingMeta != nil && existingMeta.ClusterID != "" {
		return existingMeta.ClusterID, nil
	}

	// if we are not the leader, nothing more we can do
	if !s.IsLeader() {
		return "", errors.New("cluster ID not ready yet")
	}

	// we are the leader, try to generate the ID now
	generatedID, err := s.generateClusterID()
	if err != nil {
		return "", err
	}

	return generatedID, nil
}

func (s *Server) isSingleServerCluster() bool {
	return s.config.BootstrapExpect == 1
}

// peersInfoContent is used to help operators understand what happened to the
// peers.json file. This is written to a file called peers.info in the same
// location.
const peersInfoContent = `
As of Nomad 0.5.5, the peers.json file is only used for recovery
after an outage. The format of this file depends on what the server has
configured for its Raft protocol version. Please see the agent configuration
page at https://www.consul.io/docs/agent/options.html#_raft_protocol for more
details about this parameter.
For Raft protocol version 2 and earlier, this should be formatted as a JSON
array containing the address and port of each Consul server in the cluster, like
this:
[
  "10.1.0.1:8300",
  "10.1.0.2:8300",
  "10.1.0.3:8300"
]
For Raft protocol version 3 and later, this should be formatted as a JSON
array containing the node ID, address:port, and suffrage information of each
Consul server in the cluster, like this:
[
  {
    "id": "adf4238a-882b-9ddc-4a9d-5b6758e4159e",
    "address": "10.1.0.1:8300",
    "non_voter": false
  },
  {
    "id": "8b6dda82-3103-11e7-93ae-92361f002671",
    "address": "10.1.0.2:8300",
    "non_voter": false
  },
  {
    "id": "97e17742-3103-11e7-93ae-92361f002671",
    "address": "10.1.0.3:8300",
    "non_voter": false
  }
]
The "id" field is the node ID of the server. This can be found in the logs when
the server starts up, or in the "node-id" file inside the server's data
directory.
The "address" field is the address and port of the server.
The "non_voter" field controls whether the server is a non-voter, which is used
in some advanced Autopilot configurations, please see
https://www.nomadproject.io/guides/operations/outage.html for more information. If
"non_voter" is omitted it will default to false, which is typical for most
clusters.

Under normal operation, the peers.json file will not be present.

When Nomad starts for the first time, it will create this peers.info file and
delete any existing peers.json file so that recovery doesn't occur on the first
startup.

Once this peers.info file is present, any peers.json file will be ingested at
startup, and will set the Raft peer configuration manually to recover from an
outage. It's crucial that all servers in the cluster are shut down before
creating the peers.json file, and that all servers receive the same
configuration. Once the peers.json file is successfully ingested and applied, it
will be deleted.

Please see https://www.nomadproject.io/guides/outage.html for more information.
`
