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
	"sync"
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
	successProb := rand.Float64()
	if successProb > cp.failPct {
		return true, nil
	}
	return false, fmt.Errorf("fake error")
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

// func (p *RPCProxy) AddServer("", server *ServerEndpoint) {
func TestRPCProxy_AddServer(t *testing.T) {
	p := testRPCProxy()
	var num int
	num = p.NumServers()
	if num != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	s1Endpoint := makeServerEndpointName()
	s1, ok := p.AddServer("", s1Endpoint)
	num = p.NumServers()
	if num != 1 {
		t.Fatalf("Expected one server")
	}
	if s1 == nil || !ok {
		t.Fatalf("bad")
	}
	if s1.Name != s1Endpoint {
		t.Fatalf("bad")
	}

	s1, ok = p.AddServer("", s1Endpoint)
	num = p.NumServers()
	if num != 1 {
		t.Fatalf("Expected one server (still)")
	}
	if s1 == nil || ok {
		t.Fatalf("bad")
	}
	if s1.Name != s1Endpoint {
		t.Fatalf("bad")
	}

	s2Endpoint := makeServerEndpointName()
	s2, ok := p.AddServer("", s2Endpoint)
	num = p.NumServers()
	if num != 2 {
		t.Fatalf("Expected two servers")
	}
	if s2 == nil || !ok {
		t.Fatalf("bad")
	}
	if s2.Name != s2Endpoint {
		t.Fatalf("bad")
	}
}

// func (p *RPCProxy) FindServer(1) (server *ServerEndpoint) {
func TestRPCProxy_FindServer(t *testing.T) {
	p := testRPCProxy()

	if p.FindServer(1) != nil {
		t.Fatalf("Expected nil return")
	}

	s1Endpoint := makeServerEndpointName()
	p.AddServer("", s1Endpoint)
	if p.NumServers() != 1 {
		t.Fatalf("Expected one server")
	}

	s1 := p.FindServer(1)
	if s1 == nil {
		t.Fatalf("Expected non-nil server")
	}
	if s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server")
	}

	s1 = p.FindServer(1)
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server (still)")
	}

	s2Endpoint := makeServerEndpointName()
	p.AddServer("", s2Endpoint)
	if p.NumServers() != 2 {
		t.Fatalf("Expected two servers")
	}
	s1 = p.FindServer(1)
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server (still)")
	}

	p.NotifyFailedServer(s1)
	s2 := p.FindServer(1)
	if s2 == nil || s2.Name != s2Endpoint {
		t.Fatalf("Expected s2 server")
	}

	p.NotifyFailedServer(s2)
	s1 = p.FindServer(1)
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
	s1, ok := p.AddServer("", s1Endpoint)
	if s1 == nil || !ok {
		t.Fatalf("bad")
	}
	if p.NumServers() != 1 {
		t.Fatalf("bad")
	}
	p.removeServer(s1)
	if p.NumServers() != 0 {
		t.Fatalf("bad")
	}
	p.NotifyFailedServer(s1)
	s1, _ = p.AddServer("", s1Endpoint)

	// Test again w/ a server not in the list
	s2Endpoint := makeServerEndpointName()
	s2, ok := p.AddServer("", s2Endpoint)
	if s2 == nil || !ok {
		t.Fatalf("bad")
	}
	if p.NumServers() != 2 {
		t.Fatalf("bad")
	}
	p.removeServer(s2)
	if p.NumServers() != 1 {
		t.Fatalf("bad")
	}
	p.NotifyFailedServer(s2)
	if p.NumServers() != 1 {
		t.Fatalf("Expected one server")
	}

	// Re-add s2 so there are two servers in the RPCProxy server list
	s2, _ = p.AddServer("", s2Endpoint)
	if p.NumServers() != 2 {
		t.Fatalf("Expected two servers")
	}

	// Find the first server, it should be s1
	s1 = p.FindServer(1)
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server")
	}

	// Notify s2 as failed, s1 should still be first
	p.NotifyFailedServer(s2)
	s1 = p.FindServer(1)
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server (still)")
	}

	// Fail s1, s2 should be first
	p.NotifyFailedServer(s1)
	s2 = p.FindServer(1)
	if s2 == nil || s2.Name != s2Endpoint {
		t.Fatalf("Expected s2 server")
	}

	// Fail s2, s1 should be first
	p.NotifyFailedServer(s2)
	s1 = p.FindServer(1)
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
		s, ok := p.AddServer("", serverName)
		if s == nil || !ok {
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
		p.removeServer(serverList[i-1])
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
		p.AddServer("", makeServerEndpointName())
	}

	// Keep track of how many unique shuffles we get.
	uniques := make(map[string]struct{}, maxServers)
	for i := 0; i < numShuffleTests; i++ {
		p.RebalanceServers()

		var names []string
		for j := 0; j < maxServers; j++ {
			server := p.FindServer(1)
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

// func (p *RPCProxy) removeServer(server *ServerEndpoint) {
func TestRPCProxy_removeServer(t *testing.T) {
	p := testRPCProxy()
	if p.NumServers() != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	// Test removing server before its added
	s1Endpoint := makeServerEndpointName()
	s1, ok := p.AddServer("", s1Endpoint)
	if p.NumServers() != 1 {
		t.Fatalf("bad")
	}
	if s1 == nil || s1.Name != s1Endpoint || !ok {
		t.Fatalf("Expected s1 server: %+q", s1.Name)
	}
	s1 = p.FindServer(1)
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 server: %+q", s1.Name)
	}
	p.removeServer(s1)
	if p.NumServers() != 0 {
		t.Fatalf("bad")
	}
	// Remove it a second time now that it doesn't exist
	p.removeServer(s1)
	if p.NumServers() != 0 {
		t.Fatalf("bad")
	}
	p.AddServer("", s1Endpoint)
	if p.NumServers() != 1 {
		t.Fatalf("bad")
	}

	s2Endpoint := makeServerEndpointName()
	s2, ok := p.AddServer("", s2Endpoint)
	if p.NumServers() != 2 {
		t.Fatalf("bad")
	}
	if s2 == nil || s2.Name != s2Endpoint || !ok {
		t.Fatalf("Expected s2 server: %+q", s2.Name)
	}
	s1 = p.FindServer(1)
	if s1 == nil || s1.Name != s1Endpoint {
		t.Fatalf("Expected s1 to be the front of the list: %+q==%+q", s1.Name, s1Endpoint)
	}
	// Move s1 to the back of the server list
	p.NotifyFailedServer(s1)
	s2 = p.FindServer(1)
	if s2 == nil || s2.Name != s2Endpoint {
		t.Fatalf("Expected s2 server: %+q", s2Endpoint)
	}
	p.removeServer(s2)
	if p.NumServers() != 1 {
		t.Fatalf("bad")
	}
	p.removeServer(s2)
	if p.NumServers() != 1 {
		t.Fatalf("bad")
	}
	p.AddServer("", s2Endpoint)

	const maxServers = 19
	servers := make([]*ServerEndpoint, 0, maxServers)
	servers = append(servers, s1)
	servers = append(servers, s2)
	// Already added two servers above
	for i := maxServers; i > 2; i-- {
		server, _ := p.AddServer("", makeServerEndpointName())
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
			s := p.FindServer(1)
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
		server := p.FindServer(1)
		if server == nil {
			t.Fatalf("FindServer returned nil")
		}
		p.removeServer(server)
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
		server := p.FindServer(1)
		p.NotifyFailedServer(server)
		p.removeServer(server)
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
		server := p.FindServer(1)
		p.NotifyFailedServer(server)
		server2 := p.FindServer(1)
		p.NotifyFailedServer(server2) // server2 now at end of the list

		p.removeServer(server)
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
		server := p.FindServer(1)
		p.NotifyFailedServer(server)
		server2 := p.FindServer(1)
		p.NotifyFailedServer(server2) // server2 now at end of the list
		p.removeServer(server)
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

	server0 := &ServerEndpoint{Name: "server1"}
	server1 := &ServerEndpoint{Name: "server2"}
	server2 := &ServerEndpoint{Name: "server3"}
	p.activatedList.L = append(p.activatedList.L, server0, server1, server2)

	p.activatedList.cycleServer()
	if len(p.activatedList.L) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(p.activatedList.L))
	}
	if p.activatedList.L[0] != server1 &&
		p.activatedList.L[1] != server2 &&
		p.activatedList.L[2] != server0 {
		t.Fatalf("server ordering after one cycle not correct")
	}

	p.activatedList.cycleServer()
	if len(p.activatedList.L) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(p.activatedList.L))
	}
	if p.activatedList.L[0] != server2 &&
		p.activatedList.L[1] != server0 &&
		p.activatedList.L[2] != server1 {
		t.Fatalf("server ordering after two cycles not correct")
	}

	p.activatedList.cycleServer()
	if len(p.activatedList.L) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(p.activatedList.L))
	}
	if p.activatedList.L[0] != server0 &&
		p.activatedList.L[1] != server1 &&
		p.activatedList.L[2] != server2 {
		t.Fatalf("server ordering after three cycles not correct")
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

	for _, l := range []*serverList{p.activatedList, p.primaryServers, p.backupServers} {
		if l == nil {
			t.Fatalf("serverList nil")
		}

		if len(l.L) != 0 {
			t.Fatalf("serverList.servers length not zero")
		}
	}
}

// TestRPCProxy_Race is meant to be run with -race to find races in rpcproxy
func TestRPCProxy_Race(t *testing.T) {
	p := testRPCProxy()

	randsleep := func() {
		time.Sleep(time.Duration(rand.Int63n(100)) * time.Microsecond)
	}

	errs := make(chan string, 100)

	wg := sync.WaitGroup{}
	m1 := func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			randsleep()
			p.AddServer("", fmt.Sprintf("127.0.0.%d", i))
		}
	}

	m2 := func() {
		defer wg.Done()
		for i := 0; i < 150; i += 2 {
			randsleep()
			p.AddServer("x", fmt.Sprintf("127.0.0.%d", i))
		}
	}

	m3 := func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			randsleep()
			p.RebalanceServers()
			randsleep()
		}
	}

	r1 := func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			randsleep()
			if n := p.NumServers(); n > 125 {
				errs <- fmt.Sprintf("NumServers should never exceed 125; found %d", n)
			}
		}
	}

	r2 := func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			randsleep()
			p.ServerRPCAddrs()
		}
	}

	wg.Add(5)
	go m1()
	go m2()
	go m3()
	go r1()
	go r2()
	wg.Wait()

	select {
	case err := <-errs:
		t.Fatalf(err)
	default:
	}
}
