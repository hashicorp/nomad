// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package servers

import (
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
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

func testManager(t *testing.T) (m *Manager) {
	logger := testlog.HCLogger(t)
	shutdownCh := make(chan struct{})
	m = New(logger, shutdownCh, &fauxConnPool{})
	return m
}

func TestManagerInternal_cycleServer(t *testing.T) {
	ci.Parallel(t)

	server0 := &Server{Addr: &fauxAddr{"server1"}}
	server1 := &Server{Addr: &fauxAddr{"server2"}}
	server2 := &Server{Addr: &fauxAddr{"server3"}}
	srvs := Servers([]*Server{server0, server1, server2})

	srvs.cycle()
	must.Eq(t, len(srvs), 3)
	must.SliceEqual(t, []*Server{server1, server2, server0}, srvs, must.Sprint(
		"server ordering after one cycle not correct"),
	)

	srvs.cycle()
	must.SliceEqual(t, []*Server{server2, server0, server1}, srvs, must.Sprint(
		"server ordering after two cycles not correct"),
	)

	srvs.cycle()
	must.SliceEqual(t, []*Server{server0, server1, server2}, srvs, must.Sprint(
		"server ordering after three cycles not correct"),
	)
}

func TestManagerInternal_New(t *testing.T) {
	ci.Parallel(t)

	m := testManager(t)
	must.NotNil(t, m)
	must.NotNil(t, m.logger)
	must.NotNil(t, m.shutdownCh)
}

// func (l *serverList) refreshServerRebalanceTimer() {
func TestManagerInternal_refreshServerRebalanceTimer(t *testing.T) {
	ci.Parallel(t)

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

	logger := testlog.HCLogger(t)
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
		must.Greater(t, s.minRebalance, d, must.Sprintf(
			"duration too short for cluster of size %d and %d servers (%s < %s)", s.numNodes, s.numServers, d, s.minRebalance,
		))
	}
}
