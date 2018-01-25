package servers

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"testing"
	"time"
)

func init() {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())
}

type fauxAddr struct {
	Addr string
}

func (fa *fauxAddr) String() string  { return fa.Addr }
func (fa *fauxAddr) Network() string { return fa.Addr }

type fauxConnPool struct {
	// failPct between 0.0 and 1.0 == pct of time a Ping should fail
	failPct float64
}

func (cp *fauxConnPool) Ping(net.Addr) error {
	successProb := rand.Float64()
	if successProb > cp.failPct {
		return nil
	}
	return fmt.Errorf("bad server")
}

func testManager(t *testing.T) (m *Manager) {
	logger := log.New(os.Stderr, "", 0)
	shutdownCh := make(chan struct{})
	m = New(logger, shutdownCh, &fauxConnPool{})
	return m
}

func testManagerFailProb(failPct float64) (m *Manager) {
	logger := log.New(os.Stderr, "", 0)
	logger = log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})
	m = New(logger, shutdownCh, &fauxConnPool{failPct: failPct})
	return m
}

// func (l *serverList) cycleServer() (servers []*metadata.Server) {
func TestManagerInternal_cycleServer(t *testing.T) {
	m := testManager(t)
	l := m.getServerList()

	server0 := &Server{Addr: &fauxAddr{"server1"}}
	server1 := &Server{Addr: &fauxAddr{"server2"}}
	server2 := &Server{Addr: &fauxAddr{"server3"}}
	l.servers = append(l.servers, server0, server1, server2)
	m.saveServerList(l)

	l = m.getServerList()
	if len(l.servers) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(l.servers))
	}
	if l.servers[0] != server0 &&
		l.servers[1] != server1 &&
		l.servers[2] != server2 {
		t.Fatalf("initial server ordering not correct")
	}

	l.servers = l.cycleServer()
	if len(l.servers) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(l.servers))
	}
	if l.servers[0] != server1 &&
		l.servers[1] != server2 &&
		l.servers[2] != server0 {
		t.Fatalf("server ordering after one cycle not correct")
	}

	l.servers = l.cycleServer()
	if len(l.servers) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(l.servers))
	}
	if l.servers[0] != server2 &&
		l.servers[1] != server0 &&
		l.servers[2] != server1 {
		t.Fatalf("server ordering after two cycles not correct")
	}

	l.servers = l.cycleServer()
	if len(l.servers) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(l.servers))
	}
	if l.servers[0] != server0 &&
		l.servers[1] != server1 &&
		l.servers[2] != server2 {
		t.Fatalf("server ordering after three cycles not correct")
	}
}

// func (m *Manager) getServerList() serverList {
func TestManagerInternal_getServerList(t *testing.T) {
	m := testManager(t)
	l := m.getServerList()
	if l.servers == nil {
		t.Fatalf("serverList.servers nil")
	}

	if len(l.servers) != 0 {
		t.Fatalf("serverList.servers length not zero")
	}
}

func TestManagerInternal_New(t *testing.T) {
	m := testManager(t)
	if m == nil {
		t.Fatalf("Manager nil")
	}

	if m.logger == nil {
		t.Fatalf("Manager.logger nil")
	}

	if m.shutdownCh == nil {
		t.Fatalf("Manager.shutdownCh nil")
	}
}

// func (l *serverList) refreshServerRebalanceTimer() {
func TestManagerInternal_refreshServerRebalanceTimer(t *testing.T) {
	type clusterSizes struct {
		numNodes     int32
		numServers   int
		minRebalance time.Duration
	}
	clusters := []clusterSizes{
		{1, 0, 5 * time.Minute}, // partitioned cluster
		{1, 3, 5 * time.Minute},
		{2, 3, 5 * time.Minute},
		{100, 0, 5 * time.Minute}, // partitioned
		{100, 1, 5 * time.Minute}, // partitioned
		{100, 3, 5 * time.Minute},
		{1024, 1, 5 * time.Minute}, // partitioned
		{1024, 3, 5 * time.Minute}, // partitioned
		{1024, 5, 5 * time.Minute},
		{16384, 1, 4 * time.Minute}, // partitioned
		{16384, 2, 5 * time.Minute}, // partitioned
		{16384, 3, 5 * time.Minute}, // partitioned
		{16384, 5, 5 * time.Minute},
		{32768, 0, 5 * time.Minute}, // partitioned
		{32768, 1, 8 * time.Minute}, // partitioned
		{32768, 2, 3 * time.Minute}, // partitioned
		{32768, 3, 5 * time.Minute}, // partitioned
		{32768, 5, 3 * time.Minute}, // partitioned
		{65535, 7, 5 * time.Minute},
		{65535, 0, 5 * time.Minute}, // partitioned
		{65535, 1, 8 * time.Minute}, // partitioned
		{65535, 2, 3 * time.Minute}, // partitioned
		{65535, 3, 5 * time.Minute}, // partitioned
		{65535, 5, 3 * time.Minute}, // partitioned
		{65535, 7, 5 * time.Minute},
		{1000000, 1, 4 * time.Hour},     // partitioned
		{1000000, 2, 2 * time.Hour},     // partitioned
		{1000000, 3, 80 * time.Minute},  // partitioned
		{1000000, 5, 50 * time.Minute},  // partitioned
		{1000000, 11, 20 * time.Minute}, // partitioned
		{1000000, 19, 10 * time.Minute},
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})

	for _, s := range clusters {
		m := New(logger, shutdownCh, &fauxConnPool{})
		m.SetNumNodes(s.numNodes)
		servers := make([]*Server, 0, s.numServers)
		for i := 0; i < s.numServers; i++ {
			nodeName := fmt.Sprintf("s%02d", i)
			servers = append(servers, &Server{Addr: &fauxAddr{nodeName}})
		}
		m.SetServers(servers)

		d := m.refreshServerRebalanceTimer()
		t.Logf("Nodes: %d; Servers: %d; Refresh: %v; Min: %v", s.numNodes, s.numServers, d, s.minRebalance)
		if d < s.minRebalance {
			t.Errorf("duration too short for cluster of size %d and %d servers (%s < %s)", s.numNodes, s.numServers, d, s.minRebalance)
		}
	}
}

// func (m *Manager) saveServerList(l serverList) {
func TestManagerInternal_saveServerList(t *testing.T) {
	m := testManager(t)

	// Initial condition
	func() {
		l := m.getServerList()
		if len(l.servers) != 0 {
			t.Fatalf("Manager.saveServerList failed to load init config")
		}

		newServer := new(Server)
		l.servers = append(l.servers, newServer)
		m.saveServerList(l)
	}()

	// Test that save works
	func() {
		l1 := m.getServerList()
		t1NumServers := len(l1.servers)
		if t1NumServers != 1 {
			t.Fatalf("Manager.saveServerList failed to save mutated config")
		}
	}()

	// Verify mutation w/o a save doesn't alter the original
	func() {
		newServer := new(Server)
		l := m.getServerList()
		l.servers = append(l.servers, newServer)

		l_orig := m.getServerList()
		origNumServers := len(l_orig.servers)
		if origNumServers >= len(l.servers) {
			t.Fatalf("Manager.saveServerList unsaved config overwrote original")
		}
	}()
}
