// Package rpc_proxy provides a proxy interface for Nomad Servers.  The
// RpcProxy periodically shuffles which server a Nomad Client communicates
// with in order to redistribute load across Nomad Servers.  Nomad Servers
// that fail an RPC request are automatically cycled to the end of the list
// until the server list is reshuffled.
//
// The servers package does not provide any external API guarantees and
// should be called only by `hashicorp/nomad`.
package rpc_proxy

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// apiMajorVersion is synchronized with `nomad/server.go` and
	// represents the API version supported by this client.
	//
	// TODO(sean@): This symbol should be exported somewhere.
	apiMajorVersion = 1

	// clientRPCJitterFraction determines the amount of jitter added to
	// clientRPCMinReuseDuration before a connection is expired and a new
	// connection is established in order to rebalance load across Nomad
	// servers.  The cluster-wide number of connections per second from
	// rebalancing is applied after this jitter to ensure the CPU impact
	// is always finite.  See newRebalanceConnsPerSecPerServer's comment
	// for additional commentary.
	//
	// For example, in a 10K Nomad cluster with 5x servers, this default
	// averages out to ~13 new connections from rebalancing per server
	// per second (each connection is reused for 120s to 180s).
	clientRPCJitterFraction = 2

	// clientRPCMinReuseDuration controls the minimum amount of time RPC
	// queries are sent over an established connection to a single server
	clientRPCMinReuseDuration = 120 * time.Second

	// Limit the number of new connections a server receives per second
	// for connection rebalancing.  This limit caps the load caused by
	// continual rebalancing efforts when a cluster is in equilibrium.  A
	// lower value comes at the cost of increased recovery time after a
	// partition.  This parameter begins to take effect when there are
	// more than ~48K clients querying 5x servers or at lower server
	// counts when there is a partition.
	//
	// For example, in a 100K Nomad cluster with 5x servers, it will take
	// ~5min for all servers to rebalance their connections.  If 99,995
	// agents are in the minority talking to only one server, it will
	// take ~26min for all servers to rebalance.  A 10K cluster in the
	// same scenario will take ~2.6min to rebalance.
	newRebalanceConnsPerSecPerServer = 64

	// rpcAPIMismatchLogRate determines the rate at which log entries are
	// emitted when the client and server's API versions are mismatched.
	rpcAPIMismatchLogRate = 3 * time.Hour
)

// NomadConfigInfo is an interface wrapper around this Nomad Agent's
// configuration to prevents a cyclic import dependency.
type NomadConfigInfo interface {
	RPCVersion() int
	Region() string
}

// Pinger is an interface wrapping client.ConnPool to prevent a
// cyclic import dependency
type Pinger interface {
	PingNomadServer(region string, version int, s *ServerEndpoint) (bool, error)
}

// serverList is an array of Nomad Servers.  The first server in the list is
// the active server.
//
// NOTE(sean@): We are explicitly relying on the fact that serverList will be
// copied onto the stack by atomic.Value.  Please keep this structure light.
type serverList struct {
	L []*ServerEndpoint
}

type RpcProxy struct {
	// activatedList manages the list of Nomad Servers that are eligible
	// to be queried by the Agent
	activatedList atomic.Value
	listLock      sync.Mutex

	// primaryServers is a list of servers found in the last heartbeat.
	// primaryServers are periodically reshuffled.  Covered by
	// serverListLock.
	primaryServers serverList

	// backupServers is a list of fallback servers.  These servers are
	// appended to the RpcProxy's serverList, but are never shuffled with
	// the list of servers discovered via the Nomad heartbeat.  Covered
	// by serverListLock.
	backupServers serverList

	// serverListLock covers both backupServers and primaryServers
	serverListLock sync.RWMutex

	leaderAddr string
	numNodes   int

	// rebalanceTimer controls the duration of the rebalance interval
	rebalanceTimer *time.Timer

	// shutdownCh is a copy of the channel in nomad.Client
	shutdownCh chan struct{}

	logger *log.Logger

	configInfo NomadConfigInfo

	// rpcAPIMismatchThrottle regulates the rate at which warning
	// messages are emitted in the event of an API mismatch between the
	// clients and servers.
	rpcAPIMismatchThrottle map[string]time.Time

	// connPoolPinger is used to test the health of a server in the
	// connection pool.  Pinger is an interface that wraps
	// client.ConnPool.
	connPoolPinger Pinger

	// notifyFailedBarrier is acts as a barrier to prevent queuing behind
	// serverListLock and acts as a TryLock().
	notifyFailedBarrier int32
}

// activateEndpoint adds an endpoint to the RpcProxy's active serverList.
// Returns true if the server was added, returns false if the server already
// existed in the RpcProxy's serverList.
func (p *RpcProxy) activateEndpoint(s *ServerEndpoint) bool {
	l := p.getServerList()

	// Check if this server is known
	found := false
	for idx, existing := range l.L {
		if existing.Name == s.Name {
			newServers := make([]*ServerEndpoint, len(l.L))
			copy(newServers, l.L)

			// Overwrite the existing server details in order to
			// possibly update metadata (e.g. server version)
			newServers[idx] = s

			l.L = newServers
			found = true
			break
		}
	}

	// Add to the list if not known
	if !found {
		newServers := make([]*ServerEndpoint, len(l.L), len(l.L)+1)
		copy(newServers, l.L)
		newServers = append(newServers, s)
		l.L = newServers
	}

	p.saveServerList(l)

	return !found
}

// SetBackupServers sets a list of Nomad Servers to be used in the event that
// the Nomad Agent lost contact with the list of Nomad Servers provided via
// the Nomad Agent's heartbeat.  If available, the backup servers are
// populated via Consul.
func (p *RpcProxy) SetBackupServers(addrs []string) error {
	l := make([]*ServerEndpoint, 0, len(addrs))
	for _, s := range addrs {
		s, err := newServer(s)
		if err != nil {
			p.logger.Printf("[WARN] RPC Proxy: unable to create backup server %q: %v", s, err)
			return fmt.Errorf("unable to create new backup server from %q: %v", s, err)
		}
	}

	p.serverListLock.Lock()
	p.backupServers.L = l
	p.serverListLock.Unlock()

	p.listLock.Lock()
	defer p.listLock.Unlock()
	for _, s := range l {
		p.activateEndpoint(s)
	}

	return nil
}

// AddPrimaryServer takes the RPC address of a Nomad server, creates a new
// endpoint, and adds it to both the primaryServers list and the active
// serverList used in the RPC Proxy.  If the endpoint is not known by the
// RpcProxy, appends the endpoint to the list.  The new endpoint will begin
// seeing use after the rebalance timer fires (or enough servers fail
// organically).  Any values in the primary server list are overridden by the
// next successful heartbeat.
func (p *RpcProxy) AddPrimaryServer(rpcAddr string) *ServerEndpoint {
	s, err := newServer(rpcAddr)
	if err != nil {
		p.logger.Printf("[WARN] RPC Proxy: unable to create new primary server from endpoint %q", rpcAddr)
		return nil
	}

	p.serverListLock.Lock()
	p.primaryServers.L = append(p.primaryServers.L, s)
	p.serverListLock.Unlock()

	p.listLock.Lock()
	p.activateEndpoint(s)
	p.listLock.Unlock()

	return s
}

// cycleServers returns a new list of servers that has dequeued the first
// server and enqueued it at the end of the list.  cycleServers assumes the
// caller is holding the listLock.  cycleServer does not test or ping
// the next server inline.  cycleServer may be called when the environment
// has just entered an unhealthy situation and blocking on a server test is
// less desirable than just returning the next server in the firing line.  If
// the next server fails, it will fail fast enough and cycleServer will be
// called again.
func (l *serverList) cycleServer() (servers []*ServerEndpoint) {
	numServers := len(l.L)
	if numServers < 2 {
		return servers // No action required
	}

	newServers := make([]*ServerEndpoint, 0, numServers)
	newServers = append(newServers, l.L[1:]...)
	newServers = append(newServers, l.L[0])

	return newServers
}

// removeServerByKey performs an inline removal of the first matching server
func (l *serverList) removeServerByKey(targetKey *EndpointKey) {
	for i, s := range l.L {
		if targetKey.Equal(s.Key()) {
			copy(l.L[i:], l.L[i+1:])
			l.L[len(l.L)-1] = nil
			l.L = l.L[:len(l.L)-1]
			return
		}
	}
}

// shuffleServers shuffles the server list in place
func (l *serverList) shuffleServers() {
	for i := len(l.L) - 1; i > 0; i-- {
		j := rand.Int31n(int32(i + 1))
		l.L[i], l.L[j] = l.L[j], l.L[i]
	}
}

// FindServer takes out an internal "read lock" and searches through the list
// of servers to find a "healthy" server.  If the server is actually
// unhealthy, we rely on heartbeats to detect this and remove the node from
// the server list.  If the server at the front of the list has failed or
// fails during an RPC call, it is rotated to the end of the list.  If there
// are no servers available, return nil.
func (p *RpcProxy) FindServer() *ServerEndpoint {
	l := p.getServerList()
	numServers := len(l.L)
	if numServers == 0 {
		p.logger.Printf("[WARN] RPC Proxy: No servers available")
		return nil
	} else {
		// Return whatever is at the front of the list because it is
		// assumed to be the oldest in the server list (unless -
		// hypothetically - the server list was rotated right after a
		// server was added).
		return l.L[0]
	}
}

// getServerList is a convenience method which hides the locking semantics
// of atomic.Value from the caller.
func (p *RpcProxy) getServerList() serverList {
	return p.activatedList.Load().(serverList)
}

// saveServerList is a convenience method which hides the locking semantics
// of atomic.Value from the caller.
func (p *RpcProxy) saveServerList(l serverList) {
	p.activatedList.Store(l)
}

func (p *RpcProxy) LeaderAddr() string {
	p.listLock.Lock()
	defer p.listLock.Unlock()
	return p.leaderAddr
}

// NewRpcProxy is the only way to safely create a new RpcProxy.
func NewRpcProxy(logger *log.Logger, shutdownCh chan struct{}, configInfo NomadConfigInfo, connPoolPinger Pinger) (p *RpcProxy) {
	p = new(RpcProxy)
	p.logger = logger
	p.configInfo = configInfo         // can't pass *nomad.Client: import cycle
	p.connPoolPinger = connPoolPinger // can't pass *nomad.ConnPool: import cycle
	p.rebalanceTimer = time.NewTimer(clientRPCMinReuseDuration)
	p.shutdownCh = shutdownCh

	l := serverList{}
	l.L = make([]*ServerEndpoint, 0)
	p.saveServerList(l)
	return p
}

// NotifyFailedServer marks the passed in server as "failed" by rotating it
// to the end of the server list.
func (p *RpcProxy) NotifyFailedServer(s *ServerEndpoint) {
	l := p.getServerList()

	// If the server being failed is not the first server on the list,
	// this is a noop.  If, however, the server is failed and first on
	// the list, acquire the lock, retest, and take the penalty of moving
	// the server to the end of the list.

	// Only rotate the server list when there is more than one server
	if len(l.L) > 1 && l.L[0] == s &&
		// Use atomic.CAS to emulate a TryLock().
		atomic.CompareAndSwapInt32(&p.notifyFailedBarrier, 0, 1) {
		defer atomic.StoreInt32(&p.notifyFailedBarrier, 0)

		// Grab a lock, retest, and take the hit of cycling the first
		// server to the end.
		p.listLock.Lock()
		defer p.listLock.Unlock()
		l = p.getServerList()

		if len(l.L) > 1 && l.L[0] == s {
			l.L = l.cycleServer()
			p.saveServerList(l)
		}
	}
}

func (p *RpcProxy) NumNodes() int {
	return p.numNodes
}

// NumServers takes out an internal "read lock" and returns the number of
// servers.  numServers includes both healthy and unhealthy servers.
func (p *RpcProxy) NumServers() int {
	l := p.getServerList()
	return len(l.L)
}

// RebalanceServers shuffles the list of servers on this agent.  The server
// at the front of the list is selected for the next RPC.  RPC calls that
// fail for a particular server are rotated to the end of the list.  This
// method reshuffles the list periodically in order to redistribute work
// across all known Nomad servers (i.e. guarantee that the order of servers
// in the server list is not positively correlated with the age of a server
// in the Nomad cluster).  Periodically shuffling the server list prevents
// long-lived clients from fixating on long-lived servers.
//
// Unhealthy servers are removed from the server list during the next client
// heartbeat.  Before the newly shuffled server list is saved, the new remote
// endpoint is tested to ensure its responsive.
func (p *RpcProxy) RebalanceServers() {
	var serverListLocked bool
	p.serverListLock.Lock()
	serverListLocked = true
	defer func() {
		if serverListLocked {
			p.serverListLock.Unlock()
		}
	}()

	// Early abort if there is nothing to shuffle
	if (len(p.primaryServers.L) + len(p.backupServers.L)) < 2 {
		return
	}

	// Shuffle server lists independently
	p.primaryServers.shuffleServers()
	p.backupServers.shuffleServers()

	// Create a new merged serverList
	type targetServer struct {
		server *ServerEndpoint
		// 'n' == Nomad Server
		// 'c' == Consul Server
		// 'b' == Both
		state byte
	}
	mergedList := make(map[EndpointKey]*targetServer, len(p.primaryServers.L)+len(p.backupServers.L))
	for _, s := range p.primaryServers.L {
		mergedList[*s.Key()] = &targetServer{server: s, state: 'n'}
	}
	for _, s := range p.backupServers.L {
		k := s.Key()
		_, found := mergedList[*k]
		if found {
			mergedList[*k].state = 'b'
		} else {
			mergedList[*k] = &targetServer{server: s, state: 'c'}
		}
	}

	l := &serverList{L: make([]*ServerEndpoint, 0, len(mergedList))}
	for _, s := range p.primaryServers.L {
		l.L = append(l.L, s)
	}
	for _, v := range mergedList {
		if v.state != 'c' {
			continue
		}
		l.L = append(l.L, v.server)
	}

	// Release the lock before we begin transition to operations on the
	// network timescale and attempt to ping servers.  A copy of the
	// servers has been made at this point.
	p.serverListLock.Unlock()
	serverListLocked = false

	// Iterate through the shuffled server list to find an assumed
	// healthy server.  NOTE: Do not iterate on the list directly because
	// this loop mutates the server list in-place.
	var foundHealthyServer bool
	for i := 0; i < len(l.L); i++ {
		// Always test the first server.  Failed servers are cycled
		// and eventually removed from the list when Nomad heartbeats
		// detect the failed node.
		selectedServer := l.L[0]

		ok, err := p.connPoolPinger.PingNomadServer(p.configInfo.Region(), p.configInfo.RPCVersion(), selectedServer)
		if ok {
			foundHealthyServer = true
			break
		}
		p.logger.Printf(`[DEBUG] RPC Proxy: pinging server "%s" failed: %s`, selectedServer.String(), err)

		l.cycleServer()
	}

	// If no healthy servers were found, sleep and wait for the admin to
	// join this node to a server and begin receiving heartbeats with an
	// updated list of Nomad servers.  Or Consul will begin advertising a
	// new server in the nomad-servers service.
	if !foundHealthyServer {
		p.logger.Printf("[DEBUG] RPC Proxy: No healthy servers during rebalance, aborting")
		return
	}

	// Verify that all servers are present.  Reconcile will save the
	// final serverList.
	if p.reconcileServerList(l) {
		p.logger.Printf("[DEBUG] RPC Proxy: Rebalanced %d servers, next active server is %s", len(l.L), l.L[0].String())
	} else {
		// reconcileServerList failed because Nomad removed the
		// server that was at the front of the list that had
		// successfully been Ping'ed.  Between the Ping and
		// reconcile, a Nomad heartbeat removed the node.
		//
		// Instead of doing any heroics, "freeze in place" and
		// continue to use the existing connection until the next
		// rebalance occurs.
	}

	return
}

// reconcileServerList returns true when the first server in serverList (l)
// exists in the receiver's serverList (m).  If true, the merged serverList
// (l) is stored as the receiver's serverList (m).  Returns false if the
// first server in m does not exist in the passed in list (l) (i.e. was
// removed by Nomad during a PingNomadServer() call.  Newly added servers are
// appended to the list and other missing servers are removed from the list.
func (p *RpcProxy) reconcileServerList(l *serverList) bool {
	p.listLock.Lock()
	defer p.listLock.Unlock()

	// newServerList is a serverList that has been kept up-to-date with
	// join and leave events.
	newServerList := p.getServerList()

	// If a Nomad heartbeat removed all nodes, or there is no selected
	// server (zero nodes in serverList), abort early.
	if len(newServerList.L) == 0 || len(l.L) == 0 {
		return false
	}

	type targetServer struct {
		server *ServerEndpoint

		//   'b' == both
		//   'o' == original
		//   'n' == new
		state byte
	}
	mergedList := make(map[EndpointKey]*targetServer, len(l.L))
	for _, s := range l.L {
		mergedList[*s.Key()] = &targetServer{server: s, state: 'o'}
	}
	for _, s := range newServerList.L {
		k := s.Key()
		_, found := mergedList[*k]
		if found {
			mergedList[*k].state = 'b'
		} else {
			mergedList[*k] = &targetServer{server: s, state: 'n'}
		}
	}

	// Ensure the selected server has not been removed by a heartbeat
	selectedServerKey := l.L[0].Key()
	if v, found := mergedList[*selectedServerKey]; found && v.state == 'o' {
		return false
	}

	// Append any new servers and remove any old servers
	for k, v := range mergedList {
		switch v.state {
		case 'b':
			// Do nothing, server exists in both
		case 'o':
			// Server has been removed
			l.removeServerByKey(&k)
		case 'n':
			// Server added
			l.L = append(l.L, v.server)
		default:
			panic("unknown merge list state")
		}
	}

	p.saveServerList(*l)
	return true
}

// RemoveServer takes out an internal write lock and removes a server from
// the server list.
func (p *RpcProxy) RemoveServer(s *ServerEndpoint) {
	p.listLock.Lock()
	defer p.listLock.Unlock()
	l := p.getServerList()

	// Remove the server if known
	for i, _ := range l.L {
		if l.L[i].Name == s.Name {
			newServers := make([]*ServerEndpoint, 0, len(l.L)-1)
			newServers = append(newServers, l.L[:i]...)
			newServers = append(newServers, l.L[i+1:]...)
			l.L = newServers

			p.saveServerList(l)
			return
		}
	}
}

// refreshServerRebalanceTimer is only called once m.rebalanceTimer expires.
func (p *RpcProxy) refreshServerRebalanceTimer() time.Duration {
	l := p.getServerList()
	numServers := len(l.L)
	// Limit this connection's life based on the size (and health) of the
	// cluster.  Never rebalance a connection more frequently than
	// connReuseLowWatermarkDuration, and make sure we never exceed
	// clusterWideRebalanceConnsPerSec operations/s across numLANMembers.
	clusterWideRebalanceConnsPerSec := float64(numServers * newRebalanceConnsPerSecPerServer)
	connReuseLowWatermarkDuration := clientRPCMinReuseDuration + lib.RandomStagger(clientRPCMinReuseDuration/clientRPCJitterFraction)
	numLANMembers := p.numNodes
	connRebalanceTimeout := lib.RateScaledInterval(clusterWideRebalanceConnsPerSec, connReuseLowWatermarkDuration, numLANMembers)

	p.rebalanceTimer.Reset(connRebalanceTimeout)
	return connRebalanceTimeout
}

// ResetRebalanceTimer resets the rebalance timer.  This method exists for
// testing and should not be used directly.
func (p *RpcProxy) ResetRebalanceTimer() {
	p.listLock.Lock()
	defer p.listLock.Unlock()
	p.rebalanceTimer.Reset(clientRPCMinReuseDuration)
}

// ServerRPCAddrs returns one RPC Address per server
func (p *RpcProxy) ServerRPCAddrs() []string {
	l := p.getServerList()
	serverAddrs := make([]string, 0, len(l.L))
	for _, s := range l.L {
		serverAddrs = append(serverAddrs, s.Addr.String())
	}
	return serverAddrs
}

// Run is used to start and manage the task of automatically shuffling and
// rebalancing the list of Nomad servers.  This maintenance only happens
// periodically based on the expiration of the timer.  Failed servers are
// automatically cycled to the end of the list.  New servers are appended to
// the list.  The order of the server list must be shuffled periodically to
// distribute load across all known and available Nomad servers.
func (p *RpcProxy) Run() {
	for {
		select {
		case <-p.rebalanceTimer.C:
			p.RebalanceServers()

			p.refreshServerRebalanceTimer()
		case <-p.shutdownCh:
			p.logger.Printf("[INFO] RPC Proxy: shutting down")
			return
		}
	}
}

// UpdateFromNodeUpdateResponse handles heartbeat responses from Nomad
// Servers.  Heartbeats contain a list of Nomad Servers that the client
// should talk with for RPC requests.  UpdateFromNodeUpdateResponse does not
// rebalance its serverList, that is handled elsewhere.  New servers learned
// via the heartbeat are appended to the RpcProxy's serverList.  Removed
// servers are removed immediately.  Servers speaking a newer RPC version are
// filtered from the serverList.
func (p *RpcProxy) UpdateFromNodeUpdateResponse(resp *structs.NodeUpdateResponse) error {
	// Merge all servers found in the response.  Servers in the response
	// with newer API versions are filtered from the list.  If the list
	// is missing an address found in the RpcProxy's server list, remove
	// it from the RpcProxy.
	//
	// FIXME(sean@): This is not true.  We rely on an outside pump to set
	// these values.  In order to catch the orphaned clients where all
	// Nomad servers were rolled between the heartbeat interval, the
	// rebalance task queries Consul and adds the servers found in Consul
	// to the server list in order to reattach an orphan to a server.

	p.serverListLock.Lock()
	defer p.serverListLock.Unlock()

	// 1) Create a map to reconcile the difference between
	// m.primaryServers and resp.Servers.
	type targetServer struct {
		server *ServerEndpoint

		//   'b' == both
		//   'o' == original
		//   'n' == new
		state byte
	}
	mergedNomadMap := make(map[EndpointKey]*targetServer, len(p.primaryServers.L)+len(resp.Servers))
	numOldServers := 0
	for _, s := range p.primaryServers.L {
		mergedNomadMap[*s.Key()] = &targetServer{server: s, state: 'o'}
		numOldServers++
	}
	numBothServers := 0
	var newServers bool
	for _, s := range resp.Servers {
		// Filter out servers using a newer API version.  Prevent
		// spamming the logs every heartbeat.
		//
		// TODO(sean@): Move the logging throttle logic into a
		// dedicated logging package so RpcProxy does not have to
		// perform this accounting.
		if int32(p.configInfo.RPCVersion()) < s.RPCVersion {
			now := time.Now()
			t, ok := p.rpcAPIMismatchThrottle[s.RPCAdvertiseAddr]
			if ok && t.After(now) {
				continue
			}

			p.logger.Printf("[WARN] API mismatch between client (v%d) and server (v%d), ignoring server %q", apiMajorVersion, s.RPCVersion, s.RPCAdvertiseAddr)
			p.rpcAPIMismatchThrottle[s.RPCAdvertiseAddr] = now.Add(rpcAPIMismatchLogRate)
			continue
		}

		server, err := newServer(s.RPCAdvertiseAddr)
		if err != nil {
			p.logger.Printf("[WARN] Unable to create a server from %q: %v", s.RPCAdvertiseAddr, err)
			continue
		}

		k := server.Key()
		_, found := mergedNomadMap[*k]
		if found {
			mergedNomadMap[*k].state = 'b'
			numBothServers++
		} else {
			mergedNomadMap[*k] = &targetServer{server: server, state: 'n'}
			newServers = true
		}
	}

	// Short-circuit acquiring a lock if nothing changed
	if !newServers && numOldServers == numBothServers {
		return nil
	}

	p.listLock.Lock()
	defer p.listLock.Unlock()
	newServerCfg := p.getServerList()
	for k, v := range mergedNomadMap {
		switch v.state {
		case 'b':
			// Do nothing, server exists in both
		case 'o':
			// Server has been removed

			// TODO(sean@): Teach Nomad servers how to remove
			// themselves from their heartbeat in order to
			// gracefully drain their clients over the next
			// cluster's max rebalanceTimer duration.  Without
			// this enhancement, if a server being shutdown and
			// it is the first in serverList, the client will
			// fail its next RPC connection.
			p.primaryServers.removeServerByKey(&k)
			newServerCfg.removeServerByKey(&k)
		case 'n':
			// Server added.  Append it to both lists
			// immediately.  The server should only go into
			// active use in the event of a failure or after a
			// rebalance occurs.
			p.primaryServers.L = append(p.primaryServers.L, v.server)
			newServerCfg.L = append(newServerCfg.L, v.server)
		default:
			panic("unknown merge list state")
		}
	}

	p.numNodes = int(resp.NumNodes)
	p.leaderAddr = resp.LeaderRPCAddr
	p.saveServerList(newServerCfg)

	return nil
}
