// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package servers_test

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/servers"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/stretchr/testify/require"
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

func testManager(t *testing.T) (m *servers.Manager) {
	logger := testlog.HCLogger(t)
	shutdownCh := make(chan struct{})
	m = servers.New(logger, shutdownCh, &fauxConnPool{})
	return m
}

func testManagerFailProb(t *testing.T, failPct float64) (m *servers.Manager) {
	logger := testlog.HCLogger(t)
	shutdownCh := make(chan struct{})
	m = servers.New(logger, shutdownCh, &fauxConnPool{failPct: failPct})
	return m
}

func TestServers_SetServers(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	m := testManager(t)
	var num int
	num = m.NumServers()
	if num != 0 {
		t.Fatalf("Expected zero servers to start")
	}

	s1 := &servers.Server{Addr: &fauxAddr{"server1"}}
	s2 := &servers.Server{Addr: &fauxAddr{"server2"}}
	require.True(m.SetServers([]*servers.Server{s1, s2}))
	require.False(m.SetServers([]*servers.Server{s1, s2}))
	require.False(m.SetServers([]*servers.Server{s2, s1}))
	require.Equal(2, m.NumServers())
	require.Len(m.GetServers(), 2)

	require.True(m.SetServers([]*servers.Server{s1}))
	require.Equal(1, m.NumServers())
	require.Len(m.GetServers(), 1)

	// Test that the list of servers does not get shuffled
	// as a side effect when incoming list is equal
	require.True(m.SetServers([]*servers.Server{s1, s2}))
	before := m.GetServers()
	require.False(m.SetServers([]*servers.Server{s1, s2}))
	after := m.GetServers()
	require.Equal(before, after)

	// Send a shuffled list, verify original order doesn't change
	require.False(m.SetServers([]*servers.Server{s2, s1}))
	afterShuffledInput := m.GetServers()
	require.Equal(after, afterShuffledInput)
}

func TestServers_FindServer(t *testing.T) {
	ci.Parallel(t)

	m := testManager(t)

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

	for _, srv := range srvs {
		m.NotifyFailedServer(srv)
	}

	s2 := m.FindServer()
	if s1.Equal(s2) {
		t.Fatalf("Expected different server")
	}
}

func TestServers_New(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	shutdownCh := make(chan struct{})
	m := servers.New(logger, shutdownCh, &fauxConnPool{})
	if m == nil {
		t.Fatalf("Manager nil")
	}
}

func TestServers_NotifyFailedServer(t *testing.T) {
	ci.Parallel(t)

	m := testManager(t)

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

	// Grab a server
	first := m.FindServer()

	// Find the other server
	second := s1
	if first.Equal(s1) {
		second = s2
	}

	// Fail the other server
	m.NotifyFailedServer(second)
	next := m.FindServer()
	if !next.Equal(first) {
		t.Fatalf("Expected first server (still)")
	}

	// Fail the first
	m.NotifyFailedServer(first)
	next = m.FindServer()
	if !next.Equal(second) {
		t.Fatalf("Expected second server")
	}

	// Fail the second
	m.NotifyFailedServer(second)
	next = m.FindServer()
	if !next.Equal(first) {
		t.Fatalf("Expected first server")
	}
}

func TestServers_NumServers(t *testing.T) {
	ci.Parallel(t)

	m := testManager(t)
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
	ci.Parallel(t)

	const failPct = 0.5
	m := testManagerFailProb(t, failPct)
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
