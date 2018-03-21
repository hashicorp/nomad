package servers_test

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/client/servers"
)

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

func testManager() (m *servers.Manager) {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})
	m = servers.New(logger, shutdownCh, &fauxConnPool{})
	return m
}

func testManagerFailProb(failPct float64) (m *servers.Manager) {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})
	m = servers.New(logger, shutdownCh, &fauxConnPool{failPct: failPct})
	return m
}

func TestServers_SetServers(t *testing.T) {
	m := testManager()
	var num int
	num = m.NumServers()
	if num != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	s1 := &servers.Server{Addr: &fauxAddr{"server1"}}
	s2 := &servers.Server{Addr: &fauxAddr{"server2"}}
	m.SetServers([]*servers.Server{s1, s2})
	num = m.NumServers()
	if num != 2 {
		t.Fatalf("Expected two servers")
	}

	all := m.GetServers()
	if l := len(all); l != 2 {
		t.Fatalf("expected 2 servers got %d", l)
	}

	if all[0] == s1 || all[0] == s2 {
		t.Fatalf("expected a copy, got actual server")
	}
}

func TestServers_FindServer(t *testing.T) {
	m := testManager()

	if m.FindServer() != nil {
		t.Fatalf("Expected nil return")
	}

	var srvs []*servers.Server
	srvs = append(srvs, &servers.Server{Addr: &fauxAddr{"s1"}})
	m.SetServers(srvs)
	if m.NumServers() != 1 {
		t.Fatalf("Expected one server")
	}

	s1 := m.FindServer()
	if s1 == nil {
		t.Fatalf("Expected non-nil server")
	}
	if s1.String() != "s1" {
		t.Fatalf("Expected s1 server")
	}

	s1 = m.FindServer()
	if s1 == nil || s1.String() != "s1" {
		t.Fatalf("Expected s1 server (still)")
	}

	srvs = append(srvs, &servers.Server{Addr: &fauxAddr{"s2"}})
	m.SetServers(srvs)
	if m.NumServers() != 2 {
		t.Fatalf("Expected two servers")
	}
	s1 = m.FindServer()
	if s1 == nil || s1.String() != "s1" {
		t.Fatalf("Expected s1 server (still)")
	}

	m.NotifyFailedServer(s1)
	s2 := m.FindServer()
	if s2 == nil || s2.String() != "s2" {
		t.Fatalf("Expected s2 server")
	}

	m.NotifyFailedServer(s2)
	s1 = m.FindServer()
	if s1 == nil || s1.String() != "s1" {
		t.Fatalf("Expected s1 server")
	}
}

func TestServers_New(t *testing.T) {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	shutdownCh := make(chan struct{})
	m := servers.New(logger, shutdownCh, &fauxConnPool{})
	if m == nil {
		t.Fatalf("Manager nil")
	}
}

func TestServers_NotifyFailedServer(t *testing.T) {
	m := testManager()

	if m.NumServers() != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	s1 := &servers.Server{Addr: &fauxAddr{"s1"}}
	s2 := &servers.Server{Addr: &fauxAddr{"s2"}}

	// Try notifying for a server that is not managed by Manager
	m.NotifyFailedServer(s1)
	if m.NumServers() != 0 {
		t.Fatalf("Expected zero servers to start")
	}
	m.SetServers([]*servers.Server{s1})

	// Test again w/ a server not in the list
	m.NotifyFailedServer(s2)
	if m.NumServers() != 1 {
		t.Fatalf("Expected one server")
	}

	m.SetServers([]*servers.Server{s1, s2})
	if m.NumServers() != 2 {
		t.Fatalf("Expected two servers")
	}

	s1 = m.FindServer()
	if s1 == nil || s1.String() != "s1" {
		t.Fatalf("Expected s1 server")
	}

	m.NotifyFailedServer(s2)
	s1 = m.FindServer()
	if s1 == nil || s1.String() != "s1" {
		t.Fatalf("Expected s1 server (still)")
	}

	m.NotifyFailedServer(s1)
	s2 = m.FindServer()
	if s2 == nil || s2.String() != "s2" {
		t.Fatalf("Expected s2 server")
	}

	m.NotifyFailedServer(s2)
	s1 = m.FindServer()
	if s1 == nil || s1.String() != "s1" {
		t.Fatalf("Expected s1 server")
	}
}

func TestServers_NumServers(t *testing.T) {
	m := testManager()
	var num int
	num = m.NumServers()
	if num != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	s := &servers.Server{Addr: &fauxAddr{"server1"}}
	m.SetServers([]*servers.Server{s})
	num = m.NumServers()
	if num != 1 {
		t.Fatalf("Expected one server after SetServers")
	}
}

func TestServers_RebalanceServers(t *testing.T) {
	const failPct = 0.5
	m := testManagerFailProb(failPct)
	const maxServers = 100
	const numShuffleTests = 100
	const uniquePassRate = 0.5

	// Make a huge list of nodes.
	var srvs []*servers.Server
	for i := 0; i < maxServers; i++ {
		nodeName := fmt.Sprintf("s%02d", i)
		srvs = append(srvs, &servers.Server{Addr: &fauxAddr{nodeName}})
	}
	m.SetServers(srvs)

	// Keep track of how many unique shuffles we get.
	uniques := make(map[string]struct{}, maxServers)
	for i := 0; i < numShuffleTests; i++ {
		m.RebalanceServers()

		var names []string
		for j := 0; j < maxServers; j++ {
			server := m.FindServer()
			m.NotifyFailedServer(server)
			names = append(names, server.String())
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
