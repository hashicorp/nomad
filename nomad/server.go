package nomad

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-multierror"
	lru "github.com/hashicorp/golang-lru"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/nomad/deploymentwatcher"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-boltdb"
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

	// serverMaxStreams controsl how many idle streams we keep open to a server
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
	logger *log.Logger

	// Connection pool to other Nomad servers
	connPool *ConnPool

	// Endpoints holds our RPC endpoints
	endpoints endpoints

	// The raft instance is used among Nomad nodes within the
	// region to protect operations that require strong consistency
	leaderCh      <-chan bool
	raft          *raft.Raft
	raftLayer     *RaftLayer
	raftStore     *raftboltdb.BoltStore
	raftInmem     *raft.InmemStore
	raftTransport *raft.NetworkTransport

	// fsm is the state machine used with Raft
	fsm *nomadFSM

	// rpcListener is used to listen for incoming connections
	rpcListener  net.Listener
	rpcServer    *rpc.Server
	rpcAdvertise net.Addr

	// rpcTLS is the TLS config for incoming TLS requests
	rpcTLS *tls.Config

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

	// eventCh is used to receive events from the serf cluster
	eventCh chan serf.Event

	// BlockedEvals is used to manage evaluations that are blocked on node
	// capacity changes.
	blockedEvals *BlockedEvals

	// deploymentWatcher is used to watch deployments and their allocations and
	// make the required calls to continue to transition the deployment.
	deploymentWatcher *deploymentwatcher.Watcher

	// evalBroker is used to manage the in-progress evaluations
	// that are waiting to be brokered to a sub-scheduler
	evalBroker *EvalBroker

	// periodicDispatcher is used to track and create evaluations for periodic jobs.
	periodicDispatcher *PeriodicDispatch

	// planQueue is used to manage the submitted allocation
	// plans that are waiting to be assessed by the leader
	planQueue *PlanQueue

	// heartbeatTimers track the expiration time of each heartbeat that has
	// a TTL. On expiration, the node status is updated to be 'down'.
	heartbeatTimers     map[string]*time.Timer
	heartbeatTimersLock sync.Mutex

	// consulCatalog is used for discovering other Nomad Servers via Consul
	consulCatalog consul.CatalogAPI

	// vault is the client for communicating with Vault.
	vault VaultClient

	// Worker used for processing
	workers []*Worker

	// aclCache is used to maintain the parsed ACL objects
	aclCache *lru.TwoQueueCache

	left         bool
	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
}

// Holds the RPC endpoints
type endpoints struct {
	Status     *Status
	Node       *Node
	Job        *Job
	Eval       *Eval
	Plan       *Plan
	Alloc      *Alloc
	Deployment *Deployment
	Region     *Region
	Search     *Search
	Periodic   *Periodic
	System     *System
	Operator   *Operator
	ACL        *ACL
	Enterprise *EnterpriseEndpoints
}

// NewServer is used to construct a new Nomad server from the
// configuration, potentially returning an error
func NewServer(config *Config, consulCatalog consul.CatalogAPI, logger *log.Logger) (*Server, error) {
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

	// Create a new blocked eval tracker.
	blockedEvals := NewBlockedEvals(evalBroker)

	// Create a plan queue
	planQueue, err := NewPlanQueue()
	if err != nil {
		return nil, err
	}

	// Configure TLS
	var tlsWrap tlsutil.RegionWrapper
	var incomingTLS *tls.Config
	if config.TLSConfig.EnableRPC {
		tlsConf := config.tlsConfig()
		tw, err := tlsConf.OutgoingTLSWrapper()
		if err != nil {
			return nil, err
		}
		tlsWrap = tw

		itls, err := tlsConf.IncomingTLSConfig()
		if err != nil {
			return nil, err
		}
		incomingTLS = itls
	}

	// Create the ACL object cache
	aclCache, err := lru.New2Q(aclCacheSize)
	if err != nil {
		return nil, err
	}

	// Create the server
	s := &Server{
		config:        config,
		consulCatalog: consulCatalog,
		connPool:      NewPool(config.LogOutput, serverRPCCache, serverMaxStreams, tlsWrap),
		logger:        logger,
		rpcServer:     rpc.NewServer(),
		peers:         make(map[string][]*serverParts),
		localPeers:    make(map[raft.ServerAddress]*serverParts),
		reconcileCh:   make(chan serf.Member, 32),
		eventCh:       make(chan serf.Event, 256),
		evalBroker:    evalBroker,
		blockedEvals:  blockedEvals,
		planQueue:     planQueue,
		rpcTLS:        incomingTLS,
		aclCache:      aclCache,
		shutdownCh:    make(chan struct{}),
	}

	// Create the periodic dispatcher for launching periodic jobs.
	s.periodicDispatcher = NewPeriodicDispatch(s.logger, s)

	// Setup Vault
	if err := s.setupVaultClient(); err != nil {
		s.Shutdown()
		s.logger.Printf("[ERR] nomad: failed to setup Vault client: %v", err)
		return nil, fmt.Errorf("Failed to setup Vault client: %v", err)
	}

	// Initialize the RPC layer
	if err := s.setupRPC(tlsWrap); err != nil {
		s.Shutdown()
		s.logger.Printf("[ERR] nomad: failed to start RPC layer: %s", err)
		return nil, fmt.Errorf("Failed to start RPC layer: %v", err)
	}

	// Initialize the Raft server
	if err := s.setupRaft(); err != nil {
		s.Shutdown()
		s.logger.Printf("[ERR] nomad: failed to start Raft: %s", err)
		return nil, fmt.Errorf("Failed to start Raft: %v", err)
	}

	// Initialize the wan Serf
	s.serf, err = s.setupSerf(config.SerfConfig, s.eventCh, serfSnapshot)
	if err != nil {
		s.Shutdown()
		s.logger.Printf("[ERR] nomad: failed to start serf WAN: %s", err)
		return nil, fmt.Errorf("Failed to start serf: %v", err)
	}

	// Initialize the scheduling workers
	if err := s.setupWorkers(); err != nil {
		s.Shutdown()
		s.logger.Printf("[ERR] nomad: failed to start workers: %s", err)
		return nil, fmt.Errorf("Failed to start workers: %v", err)
	}

	// Setup the Consul syncer
	if err := s.setupConsulSyncer(); err != nil {
		return nil, fmt.Errorf("failed to create server Consul syncer: %v", err)
	}

	// Setup the deployment watcher.
	if err := s.setupDeploymentWatcher(); err != nil {
		return nil, fmt.Errorf("failed to create deployment watcher: %v", err)
	}

	// Monitor leadership changes
	go s.monitorLeadership()

	// Start ingesting events for Serf
	go s.serfEventHandler()

	// Start the RPC listeners
	go s.listen()

	// Emit metrics for the eval broker
	go evalBroker.EmitStats(time.Second, s.shutdownCh)

	// Emit metrics for the plan queue
	go planQueue.EmitStats(time.Second, s.shutdownCh)

	// Emit metrics for the blocked eval tracker.
	go blockedEvals.EmitStats(time.Second, s.shutdownCh)

	// Emit metrics for the Vault client.
	go s.vault.EmitStats(time.Second, s.shutdownCh)

	// Emit metrics
	go s.heartbeatStats()

	// Done
	return s, nil
}

// Shutdown is used to shutdown the server
func (s *Server) Shutdown() error {
	s.logger.Printf("[INFO] nomad: shutting down server")
	s.shutdownLock.Lock()
	defer s.shutdownLock.Unlock()

	if s.shutdown {
		return nil
	}

	s.shutdown = true
	close(s.shutdownCh)

	if s.serf != nil {
		s.serf.Shutdown()
	}

	if s.raft != nil {
		s.raftTransport.Close()
		s.raftLayer.Close()
		future := s.raft.Shutdown()
		if err := future.Error(); err != nil {
			s.logger.Printf("[WARN] nomad: Error shutting down raft: %s", err)
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

	// Stop Vault token renewal
	if s.vault != nil {
		s.vault.Stop()
	}

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
	s.logger.Printf("[INFO] nomad: server starting leave")
	s.left = true

	// Check the number of known peers
	numPeers, err := s.numPeers()
	if err != nil {
		s.logger.Printf("[ERR] nomad: failed to check raft peers: %v", err)
		return err
	}

	// TODO (alexdadgar) - This will need to be updated once we support node
	// IDs.
	addr := s.raftTransport.LocalAddr()

	// If we are the current leader, and we have any other peers (cluster has multiple
	// servers), we should do a RemovePeer to safely reduce the quorum size. If we are
	// not the leader, then we should issue our leave intention and wait to be removed
	// for some sane period of time.
	isLeader := s.IsLeader()
	if isLeader && numPeers > 1 {
		future := s.raft.RemovePeer(addr)
		if err := future.Error(); err != nil {
			s.logger.Printf("[ERR] nomad: failed to remove ourself as raft peer: %v", err)
		}
	}

	// Leave the gossip pool
	if s.serf != nil {
		if err := s.serf.Leave(); err != nil {
			s.logger.Printf("[ERR] nomad: failed to leave Serf cluster: %v", err)
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
				s.logger.Printf("[ERR] nomad: failed to get raft configuration: %v", err)
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
			s.logger.Printf("[WARN] nomad: failed to leave raft configuration gracefully, timeout")
		}
	}
	return nil
}

// Reload handles a config reload. Not all config fields can handle a reload.
func (s *Server) Reload(config *Config) error {
	if config == nil {
		return fmt.Errorf("Reload given a nil config")
	}

	var mErr multierror.Error

	// Handle the Vault reload. Vault should never be nil but just guard.
	if s.vault != nil {
		if err := s.vault.SetConfig(config.VaultConfig); err != nil {
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
		bootstrapExpect := atomic.LoadInt32(&s.config.BootstrapExpect)
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
			if raftPeers >= int(bootstrapExpect) {
				peersTimeout.Reset(peersPollInterval + lib.RandomStagger(peersPollInterval/peersPollJitterFactor))
				return nil
			}
		}
		consulQueryCount++

		s.logger.Printf("[DEBUG] server.nomad: lost contact with Nomad quorum, falling back to Consul for server list")

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
				s.logger.Printf("[WARN] server.nomad: %v", err)
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
			s.logger.Printf("[TRACE] server.nomad: no Nomad Servers advertising service %+q in Consul datacenters %+q, sleeping for %v", nomadServerServiceName, dcs, pollInterval)
			peersTimeout.Reset(pollInterval)
			return nil
		}

		numServersContacted, err := s.Join(nomadServerServices)
		if err != nil {
			peersTimeout.Reset(peersPollInterval + lib.RandomStagger(peersPollInterval/peersPollJitterFactor))
			return fmt.Errorf("contacted %d Nomad Servers: %v", numServersContacted, err)
		}

		peersTimeout.Reset(maxStaleLeadership)
		s.logger.Printf("[INFO] server.nomad: successfully contacted %d Nomad Servers", numServersContacted)

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
						s.logger.Printf("[ERR] consul: error looking up Nomad servers: %v", err)
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
// endpoints for state information and makes transistions via Raft through a
// shim that provides the appropriate methods.
func (s *Server) setupDeploymentWatcher() error {

	// Create the raft shim type to restrict the set of raft methods that can be
	// made
	raftShim := &deploymentWatcherRaftShim{
		apply: s.raftApply,
	}

	// Create the deployment watcher
	s.deploymentWatcher = deploymentwatcher.NewDeploymentsWatcher(
		s.logger, raftShim,
		deploymentwatcher.LimitStateQueriesPerSecond,
		deploymentwatcher.CrossDeploymentEvalBatchDuration)

	return nil
}

// setupVaultClient is used to set up the Vault API client.
func (s *Server) setupVaultClient() error {
	v, err := NewVaultClient(s.config.VaultConfig, s.logger, s.purgeVaultAccessors)
	if err != nil {
		return err
	}
	s.vault = v
	return nil
}

// setupRPC is used to setup the RPC listener
func (s *Server) setupRPC(tlsWrap tlsutil.RegionWrapper) error {
	// Create endpoints
	s.endpoints.ACL = &ACL{s}
	s.endpoints.Alloc = &Alloc{s}
	s.endpoints.Eval = &Eval{s}
	s.endpoints.Job = &Job{s}
	s.endpoints.Node = &Node{srv: s}
	s.endpoints.Deployment = &Deployment{srv: s}
	s.endpoints.Operator = &Operator{s}
	s.endpoints.Periodic = &Periodic{s}
	s.endpoints.Plan = &Plan{s}
	s.endpoints.Region = &Region{s}
	s.endpoints.Status = &Status{s}
	s.endpoints.System = &System{s}
	s.endpoints.Search = &Search{s}
	s.endpoints.Enterprise = NewEnterpriseEndpoints(s)

	// Register the handlers
	s.rpcServer.Register(s.endpoints.ACL)
	s.rpcServer.Register(s.endpoints.Alloc)
	s.rpcServer.Register(s.endpoints.Eval)
	s.rpcServer.Register(s.endpoints.Job)
	s.rpcServer.Register(s.endpoints.Node)
	s.rpcServer.Register(s.endpoints.Deployment)
	s.rpcServer.Register(s.endpoints.Operator)
	s.rpcServer.Register(s.endpoints.Periodic)
	s.rpcServer.Register(s.endpoints.Plan)
	s.rpcServer.Register(s.endpoints.Region)
	s.rpcServer.Register(s.endpoints.Status)
	s.rpcServer.Register(s.endpoints.System)
	s.rpcServer.Register(s.endpoints.Search)
	s.endpoints.Enterprise.Register(s)

	list, err := net.ListenTCP("tcp", s.config.RPCAddr)
	if err != nil {
		return err
	}
	s.rpcListener = list

	if s.config.RPCAdvertise != nil {
		s.rpcAdvertise = s.config.RPCAdvertise
	} else {
		s.rpcAdvertise = s.rpcListener.Addr()
	}

	// Verify that we have a usable advertise address
	addr, ok := s.rpcAdvertise.(*net.TCPAddr)
	if !ok {
		list.Close()
		return fmt.Errorf("RPC advertise address is not a TCP Address: %v", addr)
	}
	if addr.IP.IsUnspecified() {
		list.Close()
		return fmt.Errorf("RPC advertise address is not advertisable: %v", addr)
	}

	wrapper := tlsutil.RegionSpecificWrapper(s.config.Region, tlsWrap)
	s.raftLayer = NewRaftLayer(s.rpcAdvertise, wrapper)
	return nil
}

// setupRaft is used to setup and initialize Raft
func (s *Server) setupRaft() error {
	// If we have an unclean exit then attempt to close the Raft store.
	defer func() {
		if s.raft == nil && s.raftStore != nil {
			if err := s.raftStore.Close(); err != nil {
				s.logger.Printf("[ERR] nomad: failed to close Raft store: %v", err)
			}
		}
	}()

	// Create the FSM
	var err error
	s.fsm, err = NewFSM(s.evalBroker, s.periodicDispatcher, s.blockedEvals, s.config.LogOutput)
	if err != nil {
		return err
	}

	// Create a transport layer
	trans := raft.NewNetworkTransport(s.raftLayer, 3, s.config.RaftTimeout,
		s.config.LogOutput)
	s.raftTransport = trans

	// Make sure we set the LogOutput.
	s.config.RaftConfig.LogOutput = s.config.LogOutput

	// Our version of Raft protocol requires the LocalID to match the network
	// address of the transport.
	s.config.RaftConfig.LocalID = raft.ServerID(trans.LocalAddr())

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
				s.logger.Printf("[INFO] nomad: deleted peers.json file (see peers.info for details)")
			}
		} else if _, err := os.Stat(peersFile); err == nil {
			s.logger.Printf("[INFO] nomad: found peers.json file, recovering Raft configuration...")
			configuration, err := raft.ReadPeersJSON(peersFile)
			if err != nil {
				return fmt.Errorf("recovery failed to parse peers.json: %v", err)
			}
			tmpFsm, err := NewFSM(s.evalBroker, s.periodicDispatcher, s.blockedEvals, s.config.LogOutput)
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
			s.logger.Printf("[INFO] nomad: deleted peers.json file after successful recovery")
		}
	}

	// If we are in bootstrap or dev mode and the state is clean then we can
	// bootstrap now.
	if s.config.Bootstrap || s.config.DevMode {
		hasState, err := raft.HasExistingState(log, stable, snap)
		if err != nil {
			return err
		}
		if !hasState {
			// TODO (alexdadgar) - This will need to be updated when
			// we add support for node IDs.
			configuration := raft.Configuration{
				Servers: []raft.Server{
					raft.Server{
						ID:      raft.ServerID(trans.LocalAddr()),
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

	// Setup the leader channel
	leaderCh := make(chan bool, 1)
	s.config.RaftConfig.NotifyCh = leaderCh
	s.leaderCh = leaderCh

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
	conf.Tags["port"] = fmt.Sprintf("%d", s.rpcAdvertise.(*net.TCPAddr).Port)
	if s.config.Bootstrap || (s.config.DevMode && !s.config.DevDisableBootstrap) {
		conf.Tags["bootstrap"] = "1"
	}
	bootstrapExpect := atomic.LoadInt32(&s.config.BootstrapExpect)
	if bootstrapExpect != 0 {
		conf.Tags["expect"] = fmt.Sprintf("%d", bootstrapExpect)
	}
	conf.MemberlistConfig.LogOutput = s.config.LogOutput
	conf.LogOutput = s.config.LogOutput
	conf.EventCh = ch
	if !s.config.DevMode {
		conf.SnapshotPath = filepath.Join(s.config.DataDir, path)
		if err := ensurePath(conf.SnapshotPath, false); err != nil {
			return nil, err
		}
	}
	conf.ProtocolVersion = protocolVersionMap[s.config.ProtocolVersion]
	conf.RejoinAfterLeave = true
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
		s.logger.Printf("[WARN] nomad: no enabled schedulers")
		return nil
	}

	// Start the workers
	for i := 0; i < s.config.NumSchedulers; i++ {
		if w, err := NewWorker(s); err != nil {
			return err
		} else {
			s.workers = append(s.workers, w)
		}
	}
	s.logger.Printf("[INFO] nomad: starting %d scheduling worker(s) for %v",
		s.config.NumSchedulers, s.config.EnabledSchedulers)
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
func (c *Server) LocalMember() serf.Member {
	return c.serf.LocalMember()
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

// Regions returns the known regions in the cluster.
func (s *Server) Regions() []string {
	s.peerLock.RLock()
	defer s.peerLock.RUnlock()

	regions := make([]string, 0, len(s.peers))
	for region, _ := range s.peers {
		regions = append(regions, region)
	}
	sort.Strings(regions)
	return regions
}

// inmemCodec is used to do an RPC call without going over a network
type inmemCodec struct {
	method string
	args   interface{}
	reply  interface{}
	err    error
}

func (i *inmemCodec) ReadRequestHeader(req *rpc.Request) error {
	req.ServiceMethod = i.method
	return nil
}

func (i *inmemCodec) ReadRequestBody(args interface{}) error {
	sourceValue := reflect.Indirect(reflect.Indirect(reflect.ValueOf(i.args)))
	dst := reflect.Indirect(reflect.Indirect(reflect.ValueOf(args)))
	dst.Set(sourceValue)
	return nil
}

func (i *inmemCodec) WriteResponse(resp *rpc.Response, reply interface{}) error {
	if resp.Error != "" {
		i.err = errors.New(resp.Error)
		return nil
	}
	sourceValue := reflect.Indirect(reflect.Indirect(reflect.ValueOf(reply)))
	dst := reflect.Indirect(reflect.Indirect(reflect.ValueOf(i.reply)))
	dst.Set(sourceValue)
	return nil
}

func (i *inmemCodec) Close() error {
	return nil
}

// RPC is used to make a local RPC call
func (s *Server) RPC(method string, args interface{}, reply interface{}) error {
	codec := &inmemCodec{
		method: method,
		args:   args,
		reply:  reply,
	}
	if err := s.rpcServer.ServeRequest(codec); err != nil {
		return err
	}
	return codec.err
}

// Stats is used to return statistics for debugging and insight
// for various sub-systems
func (s *Server) Stats() map[string]map[string]string {
	toString := func(v uint64) string {
		return strconv.FormatUint(v, 10)
	}
	stats := map[string]map[string]string{
		"nomad": map[string]string{
			"server":        "true",
			"leader":        fmt.Sprintf("%v", s.IsLeader()),
			"leader_addr":   string(s.raft.Leader()),
			"bootstrap":     fmt.Sprintf("%v", s.config.Bootstrap),
			"known_regions": toString(uint64(len(s.peers))),
		},
		"raft":    s.raft.Stats(),
		"serf":    s.serf.Stats(),
		"runtime": RuntimeStats(),
	}

	return stats
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

// peersInfoContent is used to help operators understand what happened to the
// peers.json file. This is written to a file called peers.info in the same
// location.
const peersInfoContent = `
As of Nomad 0.5.5, the peers.json file is only used for recovery
after an outage. It should be formatted as a JSON array containing the address
and port (RPC) of each Nomad server in the cluster, like this:

["10.1.0.1:4647","10.1.0.2:4647","10.1.0.3:4647"]

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
