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

func (cp *fauxConnPool) Ping(net.Addr) (bool, error) {
	var success bool
	successProb := rand.Float64()
	if successProb > cp.failPct {
		success = true
	}
	return success, nil
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

// func (m *Manager) AddServer(server *metadata.Server) {
func TestServers_AddServer(t *testing.T) {
	m := testManager()
	var num int
	num = m.NumServers()
	if num != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	s1 := &servers.Server{Addr: &fauxAddr{"server1"}}
	m.AddServer(s1)
	num = m.NumServers()
	if num != 1 {
		t.Fatalf("Expected one server")
	}

	m.AddServer(s1)
	num = m.NumServers()
	if num != 1 {
		t.Fatalf("Expected one server (still)")
	}

	s2 := &servers.Server{Addr: &fauxAddr{"server2"}}
	m.AddServer(s2)
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

// func (m *Manager) IsOffline() bool {
func TestServers_IsOffline(t *testing.T) {
	m := testManager()
	if !m.IsOffline() {
		t.Fatalf("bad")
	}

	s1 := &servers.Server{Addr: &fauxAddr{"server1"}}
	m.AddServer(s1)
	if m.IsOffline() {
		t.Fatalf("bad")
	}
	m.RebalanceServers()
	if m.IsOffline() {
		t.Fatalf("bad")
	}
	m.RemoveServer(s1)
	m.RebalanceServers()
	if !m.IsOffline() {
		t.Fatalf("bad")
	}

	const failPct = 0.5
	m = testManagerFailProb(failPct)
	m.AddServer(s1)
	var on, off int
	for i := 0; i < 100; i++ {
		m.RebalanceServers()
		if m.IsOffline() {
			off++
		} else {
			on++
		}
	}
	if on == 0 || off == 0 {
		t.Fatalf("bad: %d %d", on, off)
	}
}

func TestServers_FindServer(t *testing.T) {
	m := testManager()

	if m.FindServer() != nil {
		t.Fatalf("Expected nil return")
	}

	m.AddServer(&servers.Server{Addr: &fauxAddr{"s1"}})
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

	m.AddServer(&servers.Server{Addr: &fauxAddr{"s2"}})
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
	m.AddServer(s1)

	// Test again w/ a server not in the list
	m.NotifyFailedServer(s2)
	if m.NumServers() != 1 {
		t.Fatalf("Expected one server")
	}

	m.AddServer(s2)
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
	m.AddServer(s)
	num = m.NumServers()
	if num != 1 {
		t.Fatalf("Expected one server after AddServer")
	}
}

func TestServers_RebalanceServers(t *testing.T) {
	const failPct = 0.5
	m := testManagerFailProb(failPct)
	const maxServers = 100
	const numShuffleTests = 100
	const uniquePassRate = 0.5

	// Make a huge list of nodes.
	for i := 0; i < maxServers; i++ {
		nodeName := fmt.Sprintf("s%02d", i)
		m.AddServer(&servers.Server{Addr: &fauxAddr{nodeName}})

	}

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

func TestManager_RemoveServer(t *testing.T) {
	const nodeNameFmt = "s%02d"
	m := testManager()

	if m.NumServers() != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	// Test removing server before its added
	nodeName := fmt.Sprintf(nodeNameFmt, 1)
	s1 := &servers.Server{Addr: &fauxAddr{nodeName}}
	m.RemoveServer(s1)
	m.AddServer(s1)

	nodeName = fmt.Sprintf(nodeNameFmt, 2)
	s2 := &servers.Server{Addr: &fauxAddr{nodeName}}
	m.RemoveServer(s2)
	m.AddServer(s2)

	const maxServers = 19
	servs := make([]*servers.Server, maxServers)
	// Already added two servers above
	for i := maxServers; i > 2; i-- {
		nodeName := fmt.Sprintf(nodeNameFmt, i)
		server := &servers.Server{Addr: &fauxAddr{nodeName}}
		servs = append(servs, server)
		m.AddServer(server)
	}
	if m.NumServers() != maxServers {
		t.Fatalf("Expected %d servers, received %d", maxServers, m.NumServers())
	}

	m.RebalanceServers()

	if m.NumServers() != maxServers {
		t.Fatalf("Expected %d servers, received %d", maxServers, m.NumServers())
	}

	findServer := func(server *servers.Server) bool {
		for i := m.NumServers(); i > 0; i-- {
			s := m.FindServer()
			if s == server {
				return true
			}
		}
		return false
	}

	expectedNumServers := maxServers
	removedServers := make([]*servers.Server, 0, maxServers)

	// Remove servers from the front of the list
	for i := 3; i > 0; i-- {
		server := m.FindServer()
		if server == nil {
			t.Fatalf("FindServer returned nil")
		}
		m.RemoveServer(server)
		expectedNumServers--
		if m.NumServers() != expectedNumServers {
			t.Fatalf("Expected %d servers (got %d)", expectedNumServers, m.NumServers())
		}
		if findServer(server) {
			t.Fatalf("Did not expect to find server %s after removal from the front", server.String())
		}
		removedServers = append(removedServers, server)
	}

	// Remove server from the end of the list
	for i := 3; i > 0; i-- {
		server := m.FindServer()
		m.NotifyFailedServer(server)
		m.RemoveServer(server)
		expectedNumServers--
		if m.NumServers() != expectedNumServers {
			t.Fatalf("Expected %d servers (got %d)", expectedNumServers, m.NumServers())
		}
		if findServer(server) == true {
			t.Fatalf("Did not expect to find server %s", server.String())
		}
		removedServers = append(removedServers, server)
	}

	// Remove server from the middle of the list
	for i := 3; i > 0; i-- {
		server := m.FindServer()
		m.NotifyFailedServer(server)
		server2 := m.FindServer()
		m.NotifyFailedServer(server2) // server2 now at end of the list

		m.RemoveServer(server)
		expectedNumServers--
		if m.NumServers() != expectedNumServers {
			t.Fatalf("Expected %d servers (got %d)", expectedNumServers, m.NumServers())
		}
		if findServer(server) == true {
			t.Fatalf("Did not expect to find server %s", server.String())
		}
		removedServers = append(removedServers, server)
	}

	if m.NumServers()+len(removedServers) != maxServers {
		t.Fatalf("Expected %d+%d=%d servers", m.NumServers(), len(removedServers), maxServers)
	}

	// Drain the remaining servers from the middle
	for i := m.NumServers(); i > 0; i-- {
		server := m.FindServer()
		m.NotifyFailedServer(server)
		server2 := m.FindServer()
		m.NotifyFailedServer(server2) // server2 now at end of the list
		m.RemoveServer(server)
		removedServers = append(removedServers, server)
	}

	if m.NumServers() != 0 {
		t.Fatalf("Expected an empty server list")
	}
	if len(removedServers) != maxServers {
		t.Fatalf("Expected all servers to be in removed server list")
	}
}
