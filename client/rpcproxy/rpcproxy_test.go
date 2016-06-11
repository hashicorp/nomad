package rpcproxy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	ipv4len          = 4
	nodeNameFmt      = "s%03d"
	defaultNomadPort = "4647"

	// Poached from RFC2544 and RFC3330
	testingNetworkCidr   = "198.18.0.0/15"
	testingNetworkUint32 = 3323068416
)

var (
	localLogger    *log.Logger
	localLogBuffer *bytes.Buffer
	serverCount    uint32
	validIp        uint32
)

func init() {
	localLogBuffer = new(bytes.Buffer)
	localLogger = log.New(localLogBuffer, "", 0)
}

func makeServerEndpointName() string {
	serverNum := atomic.AddUint32(&serverCount, 1)
	validIp := testingNetworkUint32 + serverNum
	ipv4 := make(net.IP, ipv4len)
	binary.BigEndian.PutUint32(ipv4, validIp)
	return net.JoinHostPort(ipv4.String(), defaultNomadPort)
}

func GetBufferedLogger() *log.Logger {
	return localLogger
}

type fauxConnPool struct {
	// failPct between 0.0 and 1.0 == pct of time a Ping should fail
	failPct float64
}

func (cp *fauxConnPool) PingNomadServer(region string, majorVersion int, s *ServerEndpoint) (bool, error) {
	var success bool
	successProb := rand.Float64()
	if successProb > cp.failPct {
		success = true
	}
	return success, nil
}

type fauxSerf struct {
	datacenter      string
	numNodes        int
	region          string
	rpcMinorVersion int
	rpcMajorVersion int
}

func (s *fauxSerf) NumNodes() int {
	return s.numNodes
}

func (s *fauxSerf) Region() string {
	return s.region
}

func (s *fauxSerf) Datacenter() string {
	return s.datacenter
}

func (s *fauxSerf) RPCMajorVersion() int {
	return s.rpcMajorVersion
}

func (s *fauxSerf) RPCMinorVersion() int {
	return s.rpcMinorVersion
}

func testRPCProxy() (p *RPCProxy) {
	logger := GetBufferedLogger()
	logger = log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})
	p = NewRPCProxy(logger, shutdownCh, &fauxSerf{numNodes: 16384}, &fauxConnPool{})
	return p
}

func testRPCProxyFailProb(failPct float64) (p *RPCProxy) {
	logger := GetBufferedLogger()
	logger = log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})
	p = NewRPCProxy(logger, shutdownCh, &fauxSerf{}, &fauxConnPool{failPct: failPct})
	return p
}

// func (p *RPCProxy) AddPrimaryServer(server *ServerEndpoint) {
func TestRPCProxy_AddPrimaryServer(t *testing.T) {
	p := testRPCProxy()
	var num int
	num = p.NumServers()
	if num != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	s1Endpoint := makeServerEndpointName()
	s1 := p.AddPrimaryServer(s1Endpoint)
	num = p.NumServers()
	if num != 1 {
		t.Fatalf("Expected one server")
	}
	if s1 == nil {
		t.Fatalf("bad")
	}
	if s1.Name != s1Endpoint {
		t.Fatalf("bad")
	}

	s1 = p.AddPrimaryServer(s1Endpoint)
	num = p.NumServers()
	if num != 1 {
		t.Fatalf("Expected one server (still)")
	}
	if s1 == nil {
		t.Fatalf("bad")
	}
	if s1.Name != s1Endpoint {
		t.Fatalf("bad")
	}

	s2Endpoint := makeServerEndpointName()
	s2 := p.AddPrimaryServer(s2Endpoint)
	num = p.NumServers()
	if num != 2 {
		t.Fatalf("Expected two servers")
	}
	if s2 == nil {
		t.Fatalf("bad")
	}
	if s2.Name != s2Endpoint {
		t.Fatalf("bad")
	}
}

// func (p *RPCProxy) FindServer() (server *ServerEndpoint) {
func TestRPCProxy_FindServer(t *testing.T) {
	p := testRPCProxy()

	if p.FindServer() != nil {
		t.Fatalf("Expected nil return")
	}

	s1Endpoint := makeServerEndpointName()
	p.AddPrimaryServer(s1Endpoint)
	if p.NumServers() != 1 {
		t.Fatalf("Expected one server")
	}

	s1 := p.FindServer()
	if s1 == nil {
		t.Fatalf("Expected non-nil server")
	}
	if s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server")
	}

	s1 = p.FindServer()
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server (still)")
	}

	s2Endpoint := makeServerEndpointName()
	p.AddPrimaryServer(s2Endpoint)
	if p.NumServers() != 2 {
		t.Fatalf("Expected two servers")
	}
	s1 = p.FindServer()
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server (still)")
	}

	p.NotifyFailedServer(s1)
	s2 := p.FindServer()
	if s2 == nil || s2.Name != s2Endpoint {
		t.Fatalf("Expected s2 server")
	}

	p.NotifyFailedServer(s2)
	s1 = p.FindServer()
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server")
	}
}

// func New(logger *log.Logger, shutdownCh chan struct{}) (p *RPCProxy) {
func TestRPCProxy_New(t *testing.T) {
	logger := GetBufferedLogger()
	logger = log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})
	p := NewRPCProxy(logger, shutdownCh, &fauxSerf{}, &fauxConnPool{})
	if p == nil {
		t.Fatalf("RPCProxy nil")
	}
}

// func (p *RPCProxy) NotifyFailedServer(server *ServerEndpoint) {
func TestRPCProxy_NotifyFailedServer(t *testing.T) {
	p := testRPCProxy()

	if p.NumServers() != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	// Try notifying for a server that is not managed by RPCProxy
	s1Endpoint := makeServerEndpointName()
	s1 := p.AddPrimaryServer(s1Endpoint)
	if s1 == nil {
		t.Fatalf("bad")
	}
	if p.NumServers() != 1 {
		t.Fatalf("bad")
	}
	p.RemoveServer(s1)
	if p.NumServers() != 0 {
		t.Fatalf("bad")
	}
	p.NotifyFailedServer(s1)
	s1 = p.AddPrimaryServer(s1Endpoint)

	// Test again w/ a server not in the list
	s2Endpoint := makeServerEndpointName()
	s2 := p.AddPrimaryServer(s2Endpoint)
	if s2 == nil {
		t.Fatalf("bad")
	}
	if p.NumServers() != 2 {
		t.Fatalf("bad")
	}
	p.RemoveServer(s2)
	if p.NumServers() != 1 {
		t.Fatalf("bad")
	}
	p.NotifyFailedServer(s2)
	if p.NumServers() != 1 {
		t.Fatalf("Expected one server")
	}

	// Re-add s2 so there are two servers in the RPCProxy server list
	s2 = p.AddPrimaryServer(s2Endpoint)
	if p.NumServers() != 2 {
		t.Fatalf("Expected two servers")
	}

	// Find the first server, it should be s1
	s1 = p.FindServer()
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server")
	}

	// Notify s2 as failed, s1 should still be first
	p.NotifyFailedServer(s2)
	s1 = p.FindServer()
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server (still)")
	}

	// Fail s1, s2 should be first
	p.NotifyFailedServer(s1)
	s2 = p.FindServer()
	if s2 == nil || s2.Name != s2Endpoint {
		t.Fatalf("Expected s2 server")
	}

	// Fail s2, s1 should be first
	p.NotifyFailedServer(s2)
	s1 = p.FindServer()
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server")
	}
}

// func (p *RPCProxy) NumServers() (numServers int) {
func TestRPCProxy_NumServers(t *testing.T) {
	p := testRPCProxy()
	const maxNumServers = 100
	serverList := make([]*ServerEndpoint, 0, maxNumServers)

	// Add some servers
	for i := 0; i < maxNumServers; i++ {
		num := p.NumServers()
		if num != i {
			t.Fatalf("%d: Expected %d servers", i, num)
		}
		serverName := makeServerEndpointName()
		s := p.AddPrimaryServer(serverName)
		if s == nil {
			t.Fatalf("Expected server from %+q", serverName)
		}
		serverList = append(serverList, s)

		num = p.NumServers()
		if num != i+1 {
			t.Fatalf("%d: Expected %d servers", i, num+1)
		}
	}

	// Remove some servers
	for i := maxNumServers; i > 0; i-- {
		num := p.NumServers()
		if num != i {
			t.Fatalf("%d: Expected %d servers", i, num)
		}
		p.RemoveServer(serverList[i-1])
		num = p.NumServers()
		if num != i-1 {
			t.Fatalf("%d: Expected %d servers", i, num-1)
		}
	}
}

// func (p *RPCProxy) RebalanceServers() {
func TestRPCProxy_RebalanceServers(t *testing.T) {
	const failPct = 0.5
	p := testRPCProxyFailProb(failPct)
	const maxServers = 100
	const numShuffleTests = 100
	const uniquePassRate = 0.5

	// Make a huge list of nodes.
	for i := 0; i < maxServers; i++ {
		p.AddPrimaryServer(makeServerEndpointName())
	}

	// Keep track of how many unique shuffles we get.
	uniques := make(map[string]struct{}, maxServers)
	for i := 0; i < numShuffleTests; i++ {
		p.RebalanceServers()

		var names []string
		for j := 0; j < maxServers; j++ {
			server := p.FindServer()
			p.NotifyFailedServer(server)
			names = append(names, server.Name)
		}
		key := strings.Join(names, "|")
		uniques[key] = struct{}{}
	}

	// We have to allow for the fact that there won't always be a unique
	// shuffle each pass, so we just look for smell here without the test
	// being flaky.
	if len(uniques) < int(maxServers*uniquePassRate) {
		t.Fatalf("unique shuffle ratio too low: %d/%d", len(uniques), maxServers)
	}
}

// func (p *RPCProxy) RemoveServer(server *ServerEndpoint) {
func TestRPCProxy_RemoveServer(t *testing.T) {
	p := testRPCProxy()
	if p.NumServers() != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	// Test removing server before its added
	s1Endpoint := makeServerEndpointName()
	s1 := p.AddPrimaryServer(s1Endpoint)
	if p.NumServers() != 1 {
		t.Fatalf("bad")
	}
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server: %+q", s1.Name)
	}
	s1 = p.FindServer()
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server: %+q", s1.Name)
	}
	p.RemoveServer(s1)
	if p.NumServers() != 0 {
		t.Fatalf("bad")
	}
	// Remove it a second time now that it doesn't exist
	p.RemoveServer(s1)
	if p.NumServers() != 0 {
		t.Fatalf("bad")
	}
	p.AddPrimaryServer(s1Endpoint)
	if p.NumServers() != 1 {
		t.Fatalf("bad")
	}

	s2Endpoint := makeServerEndpointName()
	s2 := p.AddPrimaryServer(s2Endpoint)
	if p.NumServers() != 2 {
		t.Fatalf("bad")
	}
	if s2 == nil || s2.Name != s2Endpoint {
		t.Fatalf("Expected s2 server: %+q", s2.Name)
	}
	s1 = p.FindServer()
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 to be the front of the list: %+q==%+q", s1.Name, s1Endpoint)
	}
	// Move s1 to the back of the server list
	p.NotifyFailedServer(s1)
	s2 = p.FindServer()
	if s2 == nil || s2.Name != s2Endpoint {
		t.Fatalf("Expected s2 server: %+q", s2Endpoint)
	}
	p.RemoveServer(s2)
	if p.NumServers() != 1 {
		t.Fatalf("bad")
	}
	p.RemoveServer(s2)
	if p.NumServers() != 1 {
		t.Fatalf("bad")
	}
	p.AddPrimaryServer(s2Endpoint)

	const maxServers = 19
	servers := make([]*ServerEndpoint, 0, maxServers)
	servers = append(servers, s1)
	servers = append(servers, s2)
	// Already added two servers above
	for i := maxServers; i > 2; i-- {
		server := p.AddPrimaryServer(makeServerEndpointName())
		servers = append(servers, server)
	}
	if p.NumServers() != maxServers {
		t.Fatalf("Expected %d servers, received %d", maxServers, p.NumServers())
	}

	p.RebalanceServers()

	if p.NumServers() != maxServers {
		t.Fatalf("Expected %d servers, received %d", maxServers, p.NumServers())
	}

	findServer := func(server *ServerEndpoint) bool {
		for i := p.NumServers(); i > 0; i-- {
			s := p.FindServer()
			if s == server {
				return true
			}
		}
		return false
	}

	expectedNumServers := maxServers
	removedServers := make([]*ServerEndpoint, 0, maxServers)

	// Remove servers from the front of the list
	for i := 3; i > 0; i-- {
		server := p.FindServer()
		if server == nil {
			t.Fatalf("FindServer returned nil")
		}
		p.RemoveServer(server)
		expectedNumServers--
		if p.NumServers() != expectedNumServers {
			t.Fatalf("Expected %d servers (got %d)", expectedNumServers, p.NumServers())
		}
		if findServer(server) == true {
			t.Fatalf("Did not expect to find server %s after removal from the front", server.Name)
		}
		removedServers = append(removedServers, server)
	}

	// Remove server from the end of the list
	for i := 3; i > 0; i-- {
		server := p.FindServer()
		p.NotifyFailedServer(server)
		p.RemoveServer(server)
		expectedNumServers--
		if p.NumServers() != expectedNumServers {
			t.Fatalf("Expected %d servers (got %d)", expectedNumServers, p.NumServers())
		}
		if findServer(server) == true {
			t.Fatalf("Did not expect to find server %s", server.Name)
		}
		removedServers = append(removedServers, server)
	}

	// Remove server from the middle of the list
	for i := 3; i > 0; i-- {
		server := p.FindServer()
		p.NotifyFailedServer(server)
		server2 := p.FindServer()
		p.NotifyFailedServer(server2) // server2 now at end of the list

		p.RemoveServer(server)
		expectedNumServers--
		if p.NumServers() != expectedNumServers {
			t.Fatalf("Expected %d servers (got %d)", expectedNumServers, p.NumServers())
		}
		if findServer(server) == true {
			t.Fatalf("Did not expect to find server %s", server.Name)
		}
		removedServers = append(removedServers, server)
	}

	if p.NumServers()+len(removedServers) != maxServers {
		t.Fatalf("Expected %d+%d=%d servers", p.NumServers(), len(removedServers), maxServers)
	}

	// Drain the remaining servers from the middle
	for i := p.NumServers(); i > 0; i-- {
		server := p.FindServer()
		p.NotifyFailedServer(server)
		server2 := p.FindServer()
		p.NotifyFailedServer(server2) // server2 now at end of the list
		p.RemoveServer(server)
		removedServers = append(removedServers, server)
	}

	if p.NumServers() != 0 {
		t.Fatalf("Expected an empty server list")
	}
	if len(removedServers) != maxServers {
		t.Fatalf("Expected all servers to be in removed server list")
	}
}

// func (p *RPCProxy) Start() {

// func (l *serverList) cycleServer() (servers []*Server) {
func TestRPCProxyInternal_cycleServer(t *testing.T) {
	p := testRPCProxy()
	l := p.getServerList()

	server0 := &ServerEndpoint{Name: "server1"}
	server1 := &ServerEndpoint{Name: "server2"}
	server2 := &ServerEndpoint{Name: "server3"}
	l.L = append(l.L, server0, server1, server2)
	p.saveServerList(l)

	l = p.getServerList()
	if len(l.L) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(l.L))
	}
	if l.L[0] != server0 &&
		l.L[1] != server1 &&
		l.L[2] != server2 {
		t.Fatalf("initial server ordering not correct")
	}

	l.L = l.cycleServer()
	if len(l.L) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(l.L))
	}
	if l.L[0] != server1 &&
		l.L[1] != server2 &&
		l.L[2] != server0 {
		t.Fatalf("server ordering after one cycle not correct")
	}

	l.L = l.cycleServer()
	if len(l.L) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(l.L))
	}
	if l.L[0] != server2 &&
		l.L[1] != server0 &&
		l.L[2] != server1 {
		t.Fatalf("server ordering after two cycles not correct")
	}

	l.L = l.cycleServer()
	if len(l.L) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(l.L))
	}
	if l.L[0] != server0 &&
		l.L[1] != server1 &&
		l.L[2] != server2 {
		t.Fatalf("server ordering after three cycles not correct")
	}
}

// func (p *RPCProxy) getServerList() serverList {
func TestRPCProxyInternal_getServerList(t *testing.T) {
	p := testRPCProxy()
	l := p.getServerList()
	if l.L == nil {
		t.Fatalf("serverList.servers nil")
	}

	if len(l.L) != 0 {
		t.Fatalf("serverList.servers length not zero")
	}
}

func TestRPCProxyInternal_New(t *testing.T) {
	p := testRPCProxy()
	if p == nil {
		t.Fatalf("bad")
	}

	if p.logger == nil {
		t.Fatalf("bad")
	}

	if p.shutdownCh == nil {
		t.Fatalf("bad")
	}
}

// func (p *RPCProxy) reconcileServerList(l *serverList) bool {
func TestRPCProxyInternal_reconcileServerList(t *testing.T) {
	tests := []int{0, 1, 2, 3, 4, 5, 10, 100}
	for _, n := range tests {
		ok, err := test_reconcileServerList(n)
		if !ok {
			t.Errorf("Expected %d to pass: %v", n, err)
		}
	}
}

func test_reconcileServerList(maxServers int) (bool, error) {
	// Build a server list, reconcile, verify the missing servers are
	// missing, the added have been added, and the original server is
	// present.
	const failPct = 0.5
	p := testRPCProxyFailProb(failPct)

	var failedServers, healthyServers []*ServerEndpoint
	for i := 0; i < maxServers; i++ {
		nodeName := fmt.Sprintf("s%02d", i)

		node := &ServerEndpoint{Name: nodeName}
		// Add 66% of servers to RPCProxy
		if rand.Float64() > 0.33 {
			p.activateEndpoint(node)

			// Of healthy servers, (ab)use connPoolPinger to
			// failPct of the servers for the reconcile.  This
			// allows for the selected server to no longer be
			// healthy for the reconcile below.
			if ok, _ := p.connPoolPinger.PingNomadServer(p.configInfo.Region(), p.configInfo.RPCMajorVersion(), node); ok {
				// Will still be present
				healthyServers = append(healthyServers, node)
			} else {
				// Will be missing
				failedServers = append(failedServers, node)
			}
		} else {
			// Will be added from the call to reconcile
			healthyServers = append(healthyServers, node)
		}
	}

	// Randomize RPCProxy's server list
	p.RebalanceServers()
	selectedServer := p.FindServer()

	var selectedServerFailed bool
	for _, s := range failedServers {
		if selectedServer.Key().Equal(s.Key()) {
			selectedServerFailed = true
			break
		}
	}

	// Update RPCProxy's server list to be "healthy" based on Serf.
	// Reconcile this with origServers, which is shuffled and has a live
	// connection, but possibly out of date.
	origServers := p.getServerList()
	p.saveServerList(serverList{L: healthyServers})

	// This should always succeed with non-zero server lists
	if !selectedServerFailed && !p.reconcileServerList(&origServers) &&
		len(p.getServerList().L) != 0 &&
		len(origServers.L) != 0 {
		// If the random gods are unfavorable and we end up with zero
		// length lists, expect things to fail and retry the test.
		return false, fmt.Errorf("Expected reconcile to succeed: %v %d %d",
			selectedServerFailed,
			len(p.getServerList().L),
			len(origServers.L))
	}

	// If we have zero-length server lists, test succeeded in degenerate
	// case.
	if len(p.getServerList().L) == 0 &&
		len(origServers.L) == 0 {
		// Failed as expected w/ zero length list
		return true, nil
	}

	resultingServerMap := make(map[EndpointKey]bool)
	for _, s := range p.getServerList().L {
		resultingServerMap[*s.Key()] = true
	}

	// Test to make sure no failed servers are in the RPCProxy's
	// list.  Error if there are any failedServers in l.servers
	for _, s := range failedServers {
		_, ok := resultingServerMap[*s.Key()]
		if ok {
			return false, fmt.Errorf("Found failed server %v in merged list %v", s, resultingServerMap)
		}
	}

	// Test to make sure all healthy servers are in the healthy list.
	if len(healthyServers) != len(p.getServerList().L) {
		return false, fmt.Errorf("Expected healthy map and servers to match: %d/%d", len(healthyServers), len(healthyServers))
	}

	// Test to make sure all healthy servers are in the resultingServerMap list.
	for _, s := range healthyServers {
		_, ok := resultingServerMap[*s.Key()]
		if !ok {
			return false, fmt.Errorf("Server %v missing from healthy map after merged lists", s)
		}
	}
	return true, nil
}

// func (l *serverList) refreshServerRebalanceTimer() {
func TestRPCProxyInternal_refreshServerRebalanceTimer(t *testing.T) {
	type clusterSizes struct {
		numNodes     int
		numServers   int
		minRebalance time.Duration
	}
	clusters := []clusterSizes{
		{0, 3, 10 * time.Minute},
		{1, 0, 10 * time.Minute}, // partitioned cluster
		{1, 3, 10 * time.Minute},
		{2, 3, 10 * time.Minute},
		{100, 0, 10 * time.Minute}, // partitioned
		{100, 1, 10 * time.Minute}, // partitioned
		{100, 3, 10 * time.Minute},
		{1024, 1, 10 * time.Minute}, // partitioned
		{1024, 3, 10 * time.Minute}, // partitioned
		{1024, 5, 10 * time.Minute},
		{16384, 1, 10 * time.Minute}, // partitioned
		{16384, 2, 10 * time.Minute}, // partitioned
		{16384, 3, 10 * time.Minute}, // partitioned
		{16384, 5, 10 * time.Minute},
		{65535, 0, 10 * time.Minute}, // partitioned
		{65535, 1, 10 * time.Minute}, // partitioned
		{65535, 2, 10 * time.Minute}, // partitioned
		{65535, 3, 10 * time.Minute}, // partitioned
		{65535, 5, 10 * time.Minute}, // partitioned
		{65535, 7, 10 * time.Minute},
		{1000000, 1, 10 * time.Minute},  // partitioned
		{1000000, 2, 10 * time.Minute},  // partitioned
		{1000000, 3, 10 * time.Minute},  // partitioned
		{1000000, 5, 10 * time.Minute},  // partitioned
		{1000000, 11, 10 * time.Minute}, // partitioned
		{1000000, 19, 10 * time.Minute},
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})

	for i, s := range clusters {
		p := NewRPCProxy(logger, shutdownCh, &fauxSerf{numNodes: s.numNodes}, &fauxConnPool{})
		for i := 0; i < s.numServers; i++ {
			nodeName := fmt.Sprintf("s%02d", i)
			p.activateEndpoint(&ServerEndpoint{Name: nodeName})
		}

		d := p.refreshServerRebalanceTimer()
		if d < s.minRebalance {
			t.Errorf("[%d] duration too short for cluster of size %d and %d servers (%s < %s)", i, s.numNodes, s.numServers, d, s.minRebalance)
		}
	}
}

// func (p *RPCProxy) saveServerList(l serverList) {
func TestRPCProxyInternal_saveServerList(t *testing.T) {
	p := testRPCProxy()

	// Initial condition
	func() {
		l := p.getServerList()
		if len(l.L) != 0 {
			t.Fatalf("RPCProxy.saveServerList failed to load init config")
		}

		newServer := new(ServerEndpoint)
		l.L = append(l.L, newServer)
		p.saveServerList(l)
	}()

	// Test that save works
	func() {
		l1 := p.getServerList()
		t1NumServers := len(l1.L)
		if t1NumServers != 1 {
			t.Fatalf("RPCProxy.saveServerList failed to save mutated config")
		}
	}()

	// Verify mutation w/o a save doesn't alter the original
	func() {
		newServer := new(ServerEndpoint)
		l := p.getServerList()
		l.L = append(l.L, newServer)

		l_orig := p.getServerList()
		origNumServers := len(l_orig.L)
		if origNumServers >= len(l.L) {
			t.Fatalf("RPCProxy.saveServerList unsaved config overwrote original")
		}
	}()
}
