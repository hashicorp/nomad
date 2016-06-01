package rpcproxy_test

import (
	"bytes"
	"encoding/binary"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/nomad/client/rpcproxy"
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
	failPct    float64
	datacenter string
}

func (cp *fauxConnPool) PingNomadServer(region string, majorVersion int, server *rpcproxy.ServerEndpoint) (bool, error) {
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

func (s *fauxSerf) Datacenter() string {
	return s.datacenter
}

func (s *fauxSerf) NumNodes() int {
	return s.numNodes
}

func (s *fauxSerf) Region() string {
	return s.region
}

func (s *fauxSerf) RpcMajorVersion() int {
	return s.rpcMajorVersion
}

func (s *fauxSerf) RpcMinorVersion() int {
	return s.rpcMinorVersion
}

func testRpcProxy() (p *rpcproxy.RpcProxy) {
	logger := GetBufferedLogger()
	logger = log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})
	p = rpcproxy.New(logger, shutdownCh, &fauxSerf{}, &fauxConnPool{})
	return p
}

func testRpcProxyFailProb(failPct float64) (p *rpcproxy.RpcProxy) {
	logger := GetBufferedLogger()
	logger = log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})
	p = rpcproxy.New(logger, shutdownCh, &fauxSerf{}, &fauxConnPool{failPct: failPct})
	return p
}

// func (p *RpcProxy) AddPrimaryServer(server *rpcproxy.ServerEndpoint) {
func TestServers_AddPrimaryServer(t *testing.T) {
	p := testRpcProxy()
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

// func (p *RpcProxy) FindServer() (server *rpcproxy.ServerEndpoint) {
func TestServers_FindServer(t *testing.T) {
	p := testRpcProxy()

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

// func New(logger *log.Logger, shutdownCh chan struct{}) (p *RpcProxy) {
func TestServers_New(t *testing.T) {
	logger := GetBufferedLogger()
	logger = log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})
	p := rpcproxy.New(logger, shutdownCh, &fauxSerf{}, &fauxConnPool{})
	if p == nil {
		t.Fatalf("RpcProxy nil")
	}
}

// func (p *RpcProxy) NotifyFailedServer(server *rpcproxy.ServerEndpoint) {
func TestServers_NotifyFailedServer(t *testing.T) {
	p := testRpcProxy()

	if p.NumServers() != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	// Try notifying for a server that is not managed by RpcProxy
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

	// Re-add s2 so there are two servers in the RpcProxy server list
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

// func (p *RpcProxy) NumServers() (numServers int) {
func TestServers_NumServers(t *testing.T) {
	p := testRpcProxy()
	const maxNumServers = 100
	serverList := make([]*rpcproxy.ServerEndpoint, 0, maxNumServers)

	// Add some servers
	for i := 0; i < maxNumServers; i++ {
		num := p.NumServers()
		if num != i {
			t.Fatalf("%d: Expected %d servers", i, num)
		}
		serverName := makeServerEndpointName()
		s := p.AddPrimaryServer(serverName)
		if s == nil {
			t.Fatalf("Expected server from %q", serverName)
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

// func (p *RpcProxy) RebalanceServers() {
func TestServers_RebalanceServers(t *testing.T) {
	const failPct = 0.5
	p := testRpcProxyFailProb(failPct)
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

// func (p *RpcProxy) RemoveServer(server *rpcproxy.ServerEndpoint) {
func TestRpcProxy_RemoveServer(t *testing.T) {
	p := testRpcProxy()
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
		t.Fatalf("Expected s1 server: %q", s1.Name)
	}
	s1 = p.FindServer()
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server: %q", s1.Name)
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
		t.Fatalf("Expected s2 server: %q", s2.Name)
	}
	s1 = p.FindServer()
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 to be the front of the list: %q==%q", s1.Name, s1Endpoint)
	}
	// Move s1 to the back of the server list
	p.NotifyFailedServer(s1)
	s2 = p.FindServer()
	if s2 == nil || s2.Name != s2Endpoint {
		t.Fatalf("Expected s2 server: %q", s2Endpoint)
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
	servers := make([]*rpcproxy.ServerEndpoint, 0, maxServers)
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

	findServer := func(server *rpcproxy.ServerEndpoint) bool {
		for i := p.NumServers(); i > 0; i-- {
			s := p.FindServer()
			if s == server {
				return true
			}
		}
		return false
	}

	expectedNumServers := maxServers
	removedServers := make([]*rpcproxy.ServerEndpoint, 0, maxServers)

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

// func (p *RpcProxy) Start() {
