package nomad

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-boltdb"
)

const (
	raftState         = "raft/"
	tmpStatePath      = "tmp/"
	snapshotsRetained = 2

	// raftLogCacheSize is the maximum number of logs to cache in-memory.
	// This is used to reduce disk I/O for the recently commited entries.
	raftLogCacheSize = 512
)

// Server is Nomad server which manages the job queues,
// schedulers, and notification bus for agents.
type Server struct {
	config *Config
	logger *log.Logger

	// The raft instance is used among Consul nodes within the
	// DC to protect operations that require strong consistency
	raft          *raft.Raft
	raftLayer     *RaftLayer
	raftPeers     raft.PeerStore
	raftStore     *raftboltdb.BoltStore
	raftTransport *raft.NetworkTransport

	// fsm is the state machine used with Raft
	fsm *nomadFSM

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
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

	// Create the server
	s := &Server{
		config:     config,
		logger:     logger,
		shutdownCh: make(chan struct{}),
	}

	// Initialize the Raft server
	if err := s.setupRaft(); err != nil {
		s.Shutdown()
		return nil, fmt.Errorf("Failed to start Raft: %v", err)
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

	if s.raft != nil {
		s.raftTransport.Close()
		s.raftLayer.Close()
		future := s.raft.Shutdown()
		if err := future.Error(); err != nil {
			s.logger.Printf("[WARN] nomad: Error shutting down raft: %s", err)
		}
		s.raftStore.Close()
	}

	// Close the fsm
	if s.fsm != nil {
		s.fsm.Close()
	}

	return nil
}

// setupRaft is used to setup and initialize Raft
func (s *Server) setupRaft() error {
	// If we are in bootstrap mode, enable a single node cluster
	if s.config.Bootstrap {
		s.config.RaftConfig.EnableSingleNode = true
	}

	// Create the base state path
	statePath := filepath.Join(s.config.DataDir, tmpStatePath)
	if err := os.RemoveAll(statePath); err != nil {
		return err
	}
	if err := ensurePath(statePath, true); err != nil {
		return err
	}

	// Create the FSM
	var err error
	s.fsm, err = NewFSM(statePath, s.config.LogOutput)
	if err != nil {
		return err
	}

	// Create the base raft path
	path := filepath.Join(s.config.DataDir, raftState)
	if err := ensurePath(path, true); err != nil {
		return err
	}

	// Create the backend raft store for logs and stable storage
	store, err := raftboltdb.NewBoltStore(filepath.Join(path, "raft.db"))
	if err != nil {
		return err
	}
	s.raftStore = store

	// Wrap the store in a LogCache to improve performance
	cacheStore, err := raft.NewLogCache(raftLogCacheSize, store)
	if err != nil {
		store.Close()
		return err
	}

	// Create the snapshot store
	snapshots, err := raft.NewFileSnapshotStore(path, snapshotsRetained, s.config.LogOutput)
	if err != nil {
		store.Close()
		return err
	}

	// Create a transport layer
	trans := raft.NewNetworkTransport(s.raftLayer, 3, 10*time.Second, s.config.LogOutput)
	s.raftTransport = trans

	// Setup the peer store
	s.raftPeers = raft.NewJSONPeers(path, trans)

	// Ensure local host is always included if we are in bootstrap mode
	if s.config.Bootstrap {
		peers, err := s.raftPeers.Peers()
		if err != nil {
			store.Close()
			return err
		}
		if !raft.PeerContained(peers, trans.LocalAddr()) {
			s.raftPeers.SetPeers(raft.AddUniquePeer(peers, trans.LocalAddr()))
		}
	}

	// Make sure we set the LogOutput
	s.config.RaftConfig.LogOutput = s.config.LogOutput

	// Setup the Raft store
	s.raft, err = raft.NewRaft(s.config.RaftConfig, s.fsm, cacheStore, store,
		snapshots, s.raftPeers, trans)
	if err != nil {
		store.Close()
		trans.Close()
		return err
	}

	// Start monitoring leadership
	go s.monitorLeadership()
	return nil
}
