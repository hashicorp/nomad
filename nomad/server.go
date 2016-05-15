package nomad

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-boltdb"
	"github.com/hashicorp/serf/serf"
)

const (
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

	// apiMajorVersion is returned as part of the Status.Version request.
	// It should be incremented anytime the APIs are changed in a way that
	// would break clients for sane client versioning.
	apiMajorVersion = 1

	// apiMinorVersion is returned as part of the Status.Version request.
	// It should be incremented anytime the APIs are changed to allow
	// for sane client versioning. Minor changes should be compatible
	// within the major version.
	apiMinorVersion = 1
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
	raftPeers     raft.PeerStore
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
	localPeers map[string]*serverParts
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

	// evalBroker is used to manage the in-progress evaluations
	// that are waiting to be brokered to a sub-scheduler
	evalBroker *EvalBroker

	// BlockedEvals is used to manage evaluations that are blocked on node
	// capacity changes.
	blockedEvals *BlockedEvals

	// planQueue is used to manage the submitted allocation
	// plans that are waiting to be assessed by the leader
	planQueue *PlanQueue

	// periodicDispatcher is used to track and create evaluations for periodic jobs.
	periodicDispatcher *PeriodicDispatch

	// heartbeatTimers track the expiration time of each heartbeat that has
	// a TTL. On expiration, the node status is updated to be 'down'.
	heartbeatTimers     map[string]*time.Timer
	heartbeatTimersLock sync.Mutex

	// Worker used for processing
	workers []*Worker

	left         bool
	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
}

// Holds the RPC endpoints
type endpoints struct {
	Status   *Status
	Node     *Node
	Job      *Job
	Eval     *Eval
	Plan     *Plan
	Alloc    *Alloc
	Region   *Region
	Periodic *Periodic
	System   *System
}

// NewServer is used to construct a new Nomad server from the
// configuration, potentially returning an error
func NewServer(config *Config) (*Server, error) {
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

	// Create an eval broker
	evalBroker, err := NewEvalBroker(config.EvalNackTimeout, config.EvalDeliveryLimit)
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

	// Create the server
	s := &Server{
		config:       config,
		connPool:     NewPool(config.LogOutput, serverRPCCache, serverMaxStreams, nil),
		logger:       logger,
		rpcServer:    rpc.NewServer(),
		peers:        make(map[string][]*serverParts),
		localPeers:   make(map[string]*serverParts),
		reconcileCh:  make(chan serf.Member, 32),
		eventCh:      make(chan serf.Event, 256),
		evalBroker:   evalBroker,
		blockedEvals: blockedEvals,
		planQueue:    planQueue,
		shutdownCh:   make(chan struct{}),
	}

	// Create the periodic dispatcher for launching periodic jobs.
	s.periodicDispatcher = NewPeriodicDispatch(s.logger, s)

	// Initialize the RPC layer
	// TODO: TLS...
	if err := s.setupRPC(nil); err != nil {
		s.Shutdown()
		logger.Printf("[ERR] nomad: failed to start RPC layer: %s", err)
		return nil, fmt.Errorf("Failed to start RPC layer: %v", err)
	}

	// Initialize the Raft server
	if err := s.setupRaft(); err != nil {
		s.Shutdown()
		logger.Printf("[ERR] nomad: failed to start Raft: %s", err)
		return nil, fmt.Errorf("Failed to start Raft: %v", err)
	}

	// Initialize the wan Serf
	s.serf, err = s.setupSerf(config.SerfConfig, s.eventCh, serfSnapshot)
	if err != nil {
		s.Shutdown()
		logger.Printf("[ERR] nomad: failed to start serf WAN: %s", err)
		return nil, fmt.Errorf("Failed to start serf: %v", err)
	}

	// Initialize the scheduling workers
	if err := s.setupWorkers(); err != nil {
		s.Shutdown()
		logger.Printf("[ERR] nomad: failed to start workers: %s", err)
		return nil, fmt.Errorf("Failed to start workers: %v", err)
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

	// Emit metrics
	go s.heartbeatStats()

	// Seed the global random.
	if err := seedRandom(); err != nil {
		return nil, err
	}

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
	numPeers, err := s.numOtherPeers()
	if err != nil {
		s.logger.Printf("[ERR] nomad: failed to check raft peers: %v", err)
		return err
	}

	// If we are the current leader, and we have any other peers (cluster has multiple
	// servers), we should do a RemovePeer to safely reduce the quorum size. If we are
	// not the leader, then we should issue our leave intention and wait to be removed
	// for some sane period of time.
	isLeader := s.IsLeader()
	if isLeader && numPeers > 0 {
		future := s.raft.RemovePeer(s.raftTransport.LocalAddr())
		if err := future.Error(); err != nil && err != raft.ErrUnknownPeer {
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
		limit := time.Now().Add(raftRemoveGracePeriod)
		for numPeers > 0 && time.Now().Before(limit) {
			// Update the number of peers
			numPeers, err = s.numOtherPeers()
			if err != nil {
				s.logger.Printf("[ERR] nomad: failed to check raft peers: %v", err)
				break
			}

			// Avoid the sleep if we are done
			if numPeers == 0 {
				break
			}

			// Sleep a while and check again
			time.Sleep(50 * time.Millisecond)
		}
		if numPeers != 0 {
			s.logger.Printf("[WARN] nomad: failed to leave raft peer set gracefully, timeout")
		}
	}
	return nil
}

// setupRPC is used to setup the RPC listener
func (s *Server) setupRPC(tlsWrap tlsutil.DCWrapper) error {
	// Create endpoints
	s.endpoints.Status = &Status{s}
	s.endpoints.Node = &Node{srv: s}
	s.endpoints.Job = &Job{s}
	s.endpoints.Eval = &Eval{s}
	s.endpoints.Plan = &Plan{s}
	s.endpoints.Alloc = &Alloc{s}
	s.endpoints.Region = &Region{s}
	s.endpoints.Periodic = &Periodic{s}
	s.endpoints.System = &System{s}

	// Register the handlers
	s.rpcServer.Register(s.endpoints.Status)
	s.rpcServer.Register(s.endpoints.Node)
	s.rpcServer.Register(s.endpoints.Job)
	s.rpcServer.Register(s.endpoints.Eval)
	s.rpcServer.Register(s.endpoints.Plan)
	s.rpcServer.Register(s.endpoints.Alloc)
	s.rpcServer.Register(s.endpoints.Region)
	s.rpcServer.Register(s.endpoints.Periodic)
	s.rpcServer.Register(s.endpoints.System)

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

	// Provide a DC specific wrapper. Raft replication is only
	// ever done in the same datacenter, so we can provide it as a constant.
	// wrapper := tlsutil.SpecificDC(s.config.Datacenter, tlsWrap)
	// TODO: TLS...
	s.raftLayer = NewRaftLayer(s.rpcAdvertise, nil)
	return nil
}

// setupRaft is used to setup and initialize Raft
func (s *Server) setupRaft() error {
	// If we are in bootstrap mode, enable a single node cluster
	if s.config.Bootstrap || (s.config.DevMode && !s.config.DevDisableBootstrap) {
		s.config.RaftConfig.EnableSingleNode = true
	}

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

	// Create the backend raft store for logs and stable storage
	var log raft.LogStore
	var stable raft.StableStore
	var snap raft.SnapshotStore
	var peers raft.PeerStore
	if s.config.DevMode {
		store := raft.NewInmemStore()
		s.raftInmem = store
		stable = store
		log = store
		snap = raft.NewDiscardSnapshotStore()
		peers = &raft.StaticPeers{}
		s.raftPeers = peers

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

		// Setup the peer store
		s.raftPeers = raft.NewJSONPeers(path, trans)
		peers = s.raftPeers
	}

	// Ensure local host is always included if we are in bootstrap mode
	if s.config.RaftConfig.EnableSingleNode {
		p, err := peers.Peers()
		if err != nil {
			if s.raftStore != nil {
				s.raftStore.Close()
			}
			return err
		}
		if !raft.PeerContained(p, trans.LocalAddr()) {
			peers.SetPeers(raft.AddUniquePeer(p, trans.LocalAddr()))
		}
	}

	// Make sure we set the LogOutput
	s.config.RaftConfig.LogOutput = s.config.LogOutput

	// Setup the leader channel
	leaderCh := make(chan bool, 1)
	s.config.RaftConfig.NotifyCh = leaderCh
	s.leaderCh = leaderCh

	// Setup the Raft store
	s.raft, err = raft.NewRaft(s.config.RaftConfig, s.fsm, log, stable,
		snap, peers, trans)
	if err != nil {
		if s.raftStore != nil {
			s.raftStore.Close()
		}
		trans.Close()
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
	conf.Tags["vsn"] = fmt.Sprintf("%d", s.config.ProtocolVersion)
	conf.Tags["vsn_min"] = fmt.Sprintf("%d", ProtocolVersionMin)
	conf.Tags["vsn_max"] = fmt.Sprintf("%d", ProtocolVersionMax)
	conf.Tags["build"] = s.config.Build
	conf.Tags["port"] = fmt.Sprintf("%d", s.rpcAdvertise.(*net.TCPAddr).Port)
	if s.config.Bootstrap || (s.config.DevMode && !s.config.DevDisableBootstrap) {
		conf.Tags["bootstrap"] = "1"
	}
	if s.config.BootstrapExpect != 0 {
		conf.Tags["expect"] = fmt.Sprintf("%d", s.config.BootstrapExpect)
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

// numOtherPeers is used to check on the number of known peers
// excluding the local node
func (s *Server) numOtherPeers() (int, error) {
	peers, err := s.raftPeers.Peers()
	if err != nil {
		return 0, err
	}
	otherPeers := raft.ExcludePeer(peers, s.raftTransport.LocalAddr())
	return len(otherPeers), nil
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
			"leader_addr":   s.raft.Leader(),
			"bootstrap":     fmt.Sprintf("%v", s.config.Bootstrap),
			"known_regions": toString(uint64(len(s.peers))),
		},
		"raft":    s.raft.Stats(),
		"serf":    s.serf.Stats(),
		"runtime": RuntimeStats(),
	}
	if peers, err := s.raftPeers.Peers(); err == nil {
		stats["raft"]["raft_peers"] = strings.Join(peers, ",")
	} else {
		s.logger.Printf("[DEBUG] server: error getting raft peers: %v", err)
	}
	return stats
}
