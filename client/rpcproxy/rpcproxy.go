// Package rpcproxy provides a proxy interface to Nomad Servers.  The
// RPCProxy periodically shuffles which server a Nomad Client communicates
// with in order to redistribute load across Nomad Servers.  Nomad Servers
// that fail an RPC request are automatically cycled to the end of the list
// until the server list is reshuffled.
//
// The rpcproxy package does not provide any external API guarantees and
// should be called only by `hashicorp/nomad`.
package rpcproxy

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
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
	// per second.
	clientRPCJitterFraction = 2

	// clientRPCMinReuseDuration controls the minimum amount of time RPC
	// queries are sent over an established connection to a single server
	clientRPCMinReuseDuration = 600 * time.Second

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
	Datacenter() string
	RPCMajorVersion() int
	RPCMinorVersion() int
	Region() string
}

// Pinger is an interface wrapping client.ConnPool to prevent a
// cyclic import dependency
type Pinger interface {
	PingNomadServer(region string, apiMajorVersion int, s *ServerEndpoint) (bool, error)
}

// serverList is an array of Nomad Servers
type serverList struct {
	L []*ServerEndpoint
}

// RPCProxy is the manager type responsible for returning and managing Nomad
// addresses.
type RPCProxy struct {
	// activatedList manages the list of Nomad Servers that are eligible
	// to be queried by the Client agent.
	activatedList     *serverList
	activatedListLock sync.RWMutex

	// primaryServers is a list of servers found in the last heartbeat.
	// primaryServers are periodically reshuffled.  Covered by
	// serverListLock.
	primaryServers *serverList

	// backupServers is a list of fallback servers.  These servers are
	// appended to the RPCProxy's serverList, but are never shuffled with
	// the list of servers discovered via the Nomad heartbeat.  Covered
	// by serverListLock.
	backupServers *serverList

	// serverListLock covers both backupServers and primaryServers.  If
	// it is necessary to hold serverListLock and listLock, obtain an
	// exclusive lock on serverListLock before listLock.
	serverListLock sync.RWMutex

	// number of nodes in the cluster as reported by the last server heartbeat.
	//
	// Accessed atomically
	numNodes int32

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

	// setServer is how new primary servers are sent to the notifier goroutine
	setServer chan<- *ServerEndpoint

	// getServer is how the primary server is sent to callers
	getServer <-chan *ServerEndpoint
}

// NewRPCProxy is the only way to safely create a new RPCProxy.
func NewRPCProxy(logger *log.Logger, shutdownCh chan struct{}, configInfo NomadConfigInfo, connPoolPinger Pinger) *RPCProxy {
	setServer := make(chan *ServerEndpoint)
	getServer := make(chan *ServerEndpoint)
	p := &RPCProxy{
		activatedList:  newServerList(),
		backupServers:  newServerList(),
		primaryServers: newServerList(),
		logger:         logger,
		configInfo:     configInfo,     // can't pass *nomad.Client: import cycle
		connPoolPinger: connPoolPinger, // can't pass *nomad.ConnPool: import cycle
		rebalanceTimer: time.NewTimer(clientRPCMinReuseDuration),
		setServer:      setServer,
		getServer:      getServer,
		shutdownCh:     shutdownCh,
	}

	go notifier(setServer, getServer, p.shutdownCh)
	return p
}

// notifier notifies RPC callers of the current active RPC server if there is
// one. If not it blocks until one becomes available.
func notifier(in <-chan *ServerEndpoint, out chan<- *ServerEndpoint, exit <-chan struct{}) {
	var cur *ServerEndpoint

BOOTSTRAP:
	select {
	case cur = <-in:
	case <-exit:
		return
	}

	for {
		if cur == nil {
			// No healthy server, block callers until bootstrapped
			goto BOOTSTRAP
		}
		select {
		case cur = <-in:
		case out <- cur:
		case <-exit:
			return
		}
	}
}

// activateEndpoint adds an endpoint to the RPCProxy's active serverList.
// Returns true if the server was added, returns false if the server already
// existed in the RPCProxy's serverList.
func (p *RPCProxy) activateEndpoint(s *ServerEndpoint) bool {
	p.activatedListLock.Lock()
	defer p.activatedListLock.Unlock()

	// Check if this server is known
	found := false
	for idx, existing := range p.activatedList.L {
		if existing.Name == s.Name {
			// Overwrite the existing server because the name may
			// point to a different IP.
			p.activatedList.L[idx] = s

			if idx == 0 {
				// notify notifier active server changed
				p.setServer <- s
			}

			found = true
			break
		}
	}

	// Add to the list if not known
	if !found {
		p.activatedList.L = append(p.activatedList.L, s)

		if len(p.activatedList.L) == 1 {
			// notify notifier goroutine if active server has changed
			p.setServer <- s
		}
	}

	return !found
}

// AddServer to list of RPC servers. If the server is in a remote DC it will be
// added to the backup list of servers.
func (p *RPCProxy) AddServer(dc string, addr string) (*ServerEndpoint, bool) {
	s, err := NewServerEndpoint(addr)
	if err != nil {
		p.logger.Printf("[WARN] client.rpcproxy: unable to create new server from endpoint %+q: %v", addr, err)
		return nil, false
	}

	local := p.configInfo.Datacenter() == dc

	k := s.Key()

	p.serverListLock.Lock()
	defer p.serverListLock.Unlock()

	inprimary := p.primaryServers.serverExistByKey(k)
	inbackup := p.backupServers.serverExistByKey(k)

	if local {
		// Local server; ensure it's in primary list but not backup
		if inbackup {
			p.backupServers.removeServerByKey(k)
		}
		if inprimary {
			return s, false
		}
		p.primaryServers.L = append(p.primaryServers.L, s)
	} else {
		// Remote server; ensure it's in backup list but not primary
		if inprimary {
			p.primaryServers.removeServerByKey(k)
		}
		if inbackup {
			return s, false
		}
		p.backupServers.L = append(p.backupServers.L, s)
	}

	// Acquire write lock and update active server list
	p.activatedListLock.Lock()
	defer p.activatedListLock.Unlock()

	p.shuffleServers()

	return s, true
}

// shuffleServers shuffles primary and backup server lists and merges them into
// an active server list which may change the primary active server.
//
// Callers must hold the serverListLock and the activatedListLock.
func (p *RPCProxy) shuffleServers() {
	// Shuffle server lists to prevent nodes from overwhelming the first server
	p.primaryServers.shuffleServers()
	p.backupServers.shuffleServers()

	// Build new activate server list
	merged := make([]*ServerEndpoint, len(p.primaryServers.L)+len(p.backupServers.L))
	copy(merged, p.primaryServers.L)
	copy(merged[len(p.primaryServers.L):], p.backupServers.L)

	p.activatedList.L = merged

	// notify the primary server may have changed
	if len(p.activatedList.L) == 0 {
		// No healthy server!
		p.setServer <- nil
	} else {
		// New primary server
		p.setServer <- p.activatedList.L[0]
	}
}

func newServerList(s ...*ServerEndpoint) *serverList {
	return &serverList{L: s}
}

// cycleServers dequeues the first server and enqueues it at the end of the
// list.  cycleServers assumes the caller is holding the appropriate lock.
// cycleServer does not test or ping the next server inline.  cycleServer may
// be called when the environment has just entered an unhealthy situation and
// blocking on a server test is less desirable than just returning the next
// server in the firing line.  If the next server fails, it will fail fast
// enough and cycleServer will be called again.
func (l *serverList) cycleServer() {
	numServers := len(l.L)
	if numServers < 2 {
		return // Nothing to be cycled
	}

	newServers := make([]*ServerEndpoint, 0, numServers)
	newServers = append(newServers, l.L[1:]...)
	newServers = append(newServers, l.L[0])
	l.L = newServers
}

// serverExistByKey performs a search to see if a server exists in the
// serverList.  Assumes the caller is holding at least a read lock.
func (l *serverList) serverExistByKey(targetKey *EndpointKey) bool {
	var found bool
	for _, server := range l.L {
		if targetKey.Equal(server.Key()) {
			found = true
		}
	}
	return found
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

// String returns a string representation of serverList
func (l *serverList) String() string {
	if len(l.L) == 0 {
		return fmt.Sprintf("empty server list")
	}

	serverStrs := make([]string, 0, len(l.L))
	for _, server := range l.L {
		serverStrs = append(serverStrs, server.String())
	}

	return fmt.Sprintf("[%s]", strings.Join(serverStrs, ", "))
}

// FindServer takes out an internal read lock and returns the primary active
// server.  If the server is actually unhealthy, we rely on heartbeats to
// detect this and remove the node from the server list.  If the server at the
// front of the list has failed or fails during an RPC call, it is rotated to
// the end of the list.  If there are no servers available wait for one until
// timeout, then return nil.
func (p *RPCProxy) FindServer(timeout time.Duration) *ServerEndpoint {
	wait := time.NewTimer(timeout)

	p.activatedListLock.RLock()
	if len(p.activatedList.L) > 0 {
		s := p.activatedList.L[0]
		p.activatedListLock.RUnlock()
		return s
	}
	p.activatedListLock.RUnlock()

	// Wait until timeout for a new server to appear
	select {
	case s := <-p.getServer:
		wait.Stop()
		return s
	case <-wait.C:
		// Timed out
		return nil
	case <-p.shutdownCh:
		// Exiting
		wait.Stop()
		return nil
	}
}

// NotifyFailedServer marks the passed in server as "failed" by rotating it
// to the end of the server list.
func (p *RPCProxy) NotifyFailedServer(s *ServerEndpoint) {
	p.activatedListLock.Lock()
	defer p.activatedListLock.Unlock()

	// Only rotate the server list when there is more than one server
	if len(p.activatedList.L) > 1 && p.activatedList.L[0] == s {
		p.activatedList.cycleServer()
		p.setServer <- p.activatedList.L[0]
	}
}

// NumServers takes out an internal "read lock" and returns the number of
// servers.  numServers includes both healthy and unhealthy servers.
func (p *RPCProxy) NumServers() int {
	p.activatedListLock.RLock()
	defer p.activatedListLock.RUnlock()
	return len(p.activatedList.L)
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
func (p *RPCProxy) RebalanceServers() {
	p.serverListLock.Lock()
	defer p.serverListLock.Unlock()

	// Early abort if there is nothing to shuffle
	if (len(p.primaryServers.L) + len(p.backupServers.L)) < 2 {
		return
	}

	p.activatedListLock.Lock()
	defer p.activatedListLock.Unlock()

	// Shuffle servers
	p.shuffleServers()

	// Make sure new active server is healthy
	for i := 0; i < len(p.activatedList.L); i++ {
		pri := p.activatedList.L[0]

		ok, err := p.connPoolPinger.PingNomadServer(p.configInfo.Region(), p.configInfo.RPCMajorVersion(), pri)
		if ok {
			// Notify waiting callers of the new healthy server
			p.setServer <- pri
			return
		}
		p.logger.Printf(`[DEBUG] client.rpcproxy: pinging server "%s" failed: %v`, pri, err)
		p.activatedList.cycleServer()
	}

	p.logger.Printf("[WARN] client.rpcproxy: No healthy servers during rebalance")
	return
}

// removeServer takes out an internal write lock and removes a server from
// the activated server list.
func (p *RPCProxy) removeServer(s *ServerEndpoint) {
	// Lock hierarchy protocol dictates serverListLock is acquired first.
	p.serverListLock.Lock()
	defer p.serverListLock.Unlock()

	p.activatedListLock.Lock()
	defer p.activatedListLock.Unlock()

	k := s.Key()
	p.activatedList.removeServerByKey(k)
	p.primaryServers.removeServerByKey(k)
	p.backupServers.removeServerByKey(k)
}

// refreshServerRebalanceTimer is only called once p.rebalanceTimer expires.
func (p *RPCProxy) refreshServerRebalanceTimer() time.Duration {
	numServers := p.NumServers()
	// Limit this connection's life based on the size (and health) of the
	// cluster.  Never rebalance a connection more frequently than
	// connReuseLowWatermarkDuration, and make sure we never exceed
	// clusterWideRebalanceConnsPerSec operations/s across numLANMembers.
	clusterWideRebalanceConnsPerSec := float64(numServers * newRebalanceConnsPerSecPerServer)
	connReuseLowWatermarkDuration := clientRPCMinReuseDuration + lib.RandomStagger(clientRPCMinReuseDuration/clientRPCJitterFraction)
	numLANMembers := int(atomic.LoadInt32(&p.numNodes))
	connRebalanceTimeout := lib.RateScaledInterval(clusterWideRebalanceConnsPerSec, connReuseLowWatermarkDuration, numLANMembers)

	p.rebalanceTimer.Reset(connRebalanceTimeout)
	return connRebalanceTimeout
}

// resetRebalanceTimer resets the rebalance timer.  This method exists for
// testing and should not be used directly.
func (p *RPCProxy) resetRebalanceTimer() {
	p.activatedListLock.Lock()
	defer p.activatedListLock.Unlock()
	p.rebalanceTimer.Reset(clientRPCMinReuseDuration)
}

// ServerRPCAddrs returns one RPC Address per server
func (p *RPCProxy) ServerRPCAddrs() []string {
	p.activatedListLock.RLock()
	defer p.activatedListLock.RUnlock()
	serverAddrs := make([]string, 0, len(p.activatedList.L))
	for _, s := range p.activatedList.L {
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
func (p *RPCProxy) Run() {
	for {
		select {
		case <-p.rebalanceTimer.C:
			p.RebalanceServers()

			p.refreshServerRebalanceTimer()
		case <-p.shutdownCh:
			p.logger.Printf("[INFO] client.rpcproxy: shutting down")
			return
		}
	}
}

// RefreshServerLists is called when the Client receives an update from a
// Nomad Server.  The response from Nomad Client Heartbeats contain a list of
// Nomad Servers that the Nomad Client should use for RPC requests.
// RefreshServerLists does not rebalance its serverLists (that is handled
// elsewhere via a periodic timer).  New Nomad Servers learned via the
// heartbeat are appended to the RPCProxy's activated serverList.  Servers
// that are no longer present in the Heartbeat are removed immediately from
// all server lists.  Nomad Servers speaking a newer major or minor API
// version are filtered from the serverList.
func (p *RPCProxy) RefreshServerLists(servers []*structs.NodeServerInfo, numNodes int32) error {
	// update number of nodes in cluster
	atomic.StoreInt32(&p.numNodes, numNodes)

	// Create new server lists
	primaries := serverList{L: make([]*ServerEndpoint, 0, len(servers))}
	backups := serverList{L: make([]*ServerEndpoint, 0, len(servers))}

	for _, s := range servers {
		// Filter out servers using a newer API version.  Prevent
		// spamming the logs every heartbeat.
		//
		// TODO(sean@): Move the logging throttle logic into a
		// dedicated logging package so RPCProxy does not have to
		// perform this accounting.
		if int32(p.configInfo.RPCMajorVersion()) < s.RPCMajorVersion ||
			(int32(p.configInfo.RPCMajorVersion()) == s.RPCMajorVersion &&
				int32(p.configInfo.RPCMinorVersion()) < s.RPCMinorVersion) {
			now := time.Now()
			t, ok := p.rpcAPIMismatchThrottle[s.RPCAdvertiseAddr]
			if ok && t.After(now) {
				continue
			}

			p.logger.Printf("[WARN] client.rpcproxy: API mismatch between client version (v%d.%d) and server version (v%d.%d), ignoring server %+q", p.configInfo.RPCMajorVersion(), p.configInfo.RPCMinorVersion(), s.RPCMajorVersion, s.RPCMinorVersion, s.RPCAdvertiseAddr)
			p.rpcAPIMismatchThrottle[s.RPCAdvertiseAddr] = now.Add(rpcAPIMismatchLogRate)
			continue
		}

		server, err := NewServerEndpoint(s.RPCAdvertiseAddr)
		if err != nil {
			p.logger.Printf("[WARN] client.rpcproxy: Unable to create a server from %+q: %v", s.RPCAdvertiseAddr, err)
			continue
		}

		// Nomad servers in different datacenters are added to the
		// backup server list.
		if s.Datacenter != p.configInfo.Datacenter() {
			backups.L = append(backups.L, server)
			continue
		}
		primaries.L = append(primaries.L, server)
	}

	// Replace old server lists with new
	p.serverListLock.Lock()
	defer p.serverListLock.Unlock()

	p.activatedListLock.Lock()
	defer p.activatedListLock.Unlock()

	p.primaryServers = &primaries
	p.backupServers = &backups

	p.shuffleServers()

	return nil
}
