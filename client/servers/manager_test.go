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
	"github.com/shoenig/test/must"
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

	m := testManager(t)
	must.Zero(t, m.NumServers())

	s1 := &servers.Server{Addr: &fauxAddr{"server1"}}
	s2 := &servers.Server{Addr: &fauxAddr{"server2"}}
	must.True(t, m.SetServers([]*servers.Server{s1, s2}))
	must.False(t, m.SetServers([]*servers.Server{s1, s2}))
	must.False(t, m.SetServers([]*servers.Server{s2, s1}))
	must.Eq(t, 2, m.NumServers())
	must.Len(t, 2, m.GetServers())

	must.True(t, m.SetServers([]*servers.Server{s1}))
	must.Eq(t, 1, m.NumServers())
	must.Len(t, 1, m.GetServers())

	// Test that the list of servers does not get shuffled
	// as a side effect when incoming list is equal
	must.True(t, m.SetServers([]*servers.Server{s1, s2}))
	before := m.GetServers()
	must.False(t, m.SetServers([]*servers.Server{s1, s2}))
	after := m.GetServers()
	must.Equal(t, before, after)

	// Send a shuffled list, verify original order doesn't change
	must.False(t, m.SetServers([]*servers.Server{s2, s1}))
	must.Equal(t, after, m.GetServers())
}

func TestServers_FindServer(t *testing.T) {
	ci.Parallel(t)

	m := testManager(t)

	must.Nil(t, m.FindServer())

	var srvs []*servers.Server
	srvs = append(srvs, &servers.Server{Addr: &fauxAddr{"s1"}})
	m.SetServers(srvs)
	must.Eq(t, 1, m.NumServers())

	s1 := m.FindServer()
	must.NotNil(t, s1)
	must.StrEqFold(t, "s1", s1.String())

	s1 = m.FindServer()
	must.NotNil(t, s1)
	must.StrEqFold(t, "s1", s1.String())

	srvs = append(srvs, &servers.Server{Addr: &fauxAddr{"s2"}})
	m.SetServers(srvs)
	must.Eq(t, 2, m.NumServers())

	s1 = m.FindServer()

	for _, srv := range srvs {
		m.NotifyFailedServer(srv)
	}

	s2 := m.FindServer()
	must.NotEq(t, s1, s2)
}

func TestServers_New(t *testing.T) {
	ci.Parallel(t)

	must.NotNil(t, servers.New(testlog.HCLogger(t), make(chan struct{}), &fauxConnPool{}))
}

func TestServers_NotifyFailedServer(t *testing.T) {
	ci.Parallel(t)

	m := testManager(t)

	must.Zero(t, m.NumServers())

	s1 := &servers.Server{Addr: &fauxAddr{"s1"}}
	s2 := &servers.Server{Addr: &fauxAddr{"s2"}}

	// Try notifying for a server that is not managed by Manager
	m.NotifyFailedServer(s1)
	must.Zero(t, m.NumServers())
	m.SetServers([]*servers.Server{s1})

	// Test again w/ a server not in the list
	m.NotifyFailedServer(s2)
	must.Eq(t, 1, m.NumServers())

	m.SetServers([]*servers.Server{s1, s2})
	must.Eq(t, 2, m.NumServers())

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
	must.Equal(t, first, next)

	// Fail the first
	m.NotifyFailedServer(first)
	next = m.FindServer()
	must.True(t, next.Equal(second))

	// Fail the second
	m.NotifyFailedServer(second)
	next = m.FindServer()
	must.True(t, next.Equal(first))
}

func TestServers_NumServers(t *testing.T) {
	ci.Parallel(t)

	m := testManager(t)
	must.Zero(t, m.NumServers())

	s := &servers.Server{Addr: &fauxAddr{"server1"}}
	m.SetServers([]*servers.Server{s})
	must.Eq(t, 1, m.NumServers())
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
	must.Greater(t, int(maxServers*uniquePassRate), len(uniques), must.Sprintf(
		"unique shuffle ratio too low: %d/%d", len(uniques), maxServers),
	)
}
