// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/peers"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
	"github.com/shoenig/test/must"
)

func TestStatsFetcher(t *testing.T) {
	ci.Parallel(t)

	conf := func(c *Config) {
		c.Region = "region-a"
		c.BootstrapExpect = 3
	}

	s1, cleanupS1 := TestServer(t, conf)
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, conf)
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, conf)
	defer cleanupS3()

	TestJoin(t, s1, s2, s3)
	testutil.WaitForLeader(t, s1.RPC)

	members := s1.serf.Members()
	if len(members) != 3 {
		t.Fatalf("bad len: %d", len(members))
	}

	var servers []*peers.Parts
	for _, member := range members {
		ok, server := peers.IsNomadServer(member)
		if !ok {
			t.Fatalf("bad: %#v", member)
		}
		servers = append(servers, server)
	}

	// Do a normal fetch and make sure we get three responses.
	func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		stats := s1.statsFetcher.Fetch(ctx, s1.autopilotServers())
		if len(stats) != 3 {
			t.Fatalf("bad: %#v", stats)
		}
		for id, stat := range stats {
			switch id {
			case raft.ServerID(s1.config.NodeID), raft.ServerID(s2.config.NodeID), raft.ServerID(s3.config.NodeID):
				// OK
			default:
				t.Fatalf("bad: %s", id)
			}

			if stat == nil || stat.LastTerm == 0 {
				t.Fatalf("bad: %#v", stat)
			}
		}
	}()

	// Fake an in-flight request to server 3 and make sure we don't fetch
	// from it.
	func() {
		s1.statsFetcher.inflightLock.Lock()
		s1.statsFetcher.inflight[raft.ServerID(s3.config.NodeID)] = struct{}{}
		s1.statsFetcher.inflightLock.Unlock()
		defer func() {
			s1.statsFetcher.inflightLock.Lock()
			delete(s1.statsFetcher.inflight, raft.ServerID(s3.config.NodeID))
			s1.statsFetcher.inflightLock.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		stats := s1.statsFetcher.Fetch(ctx, s1.autopilotServers())
		if len(stats) != 2 {
			t.Fatalf("bad: %#v", stats)
		}
		for id, stat := range stats {
			switch id {
			case raft.ServerID(s1.config.NodeID), raft.ServerID(s2.config.NodeID):
				// OK
			case raft.ServerID(s3.config.NodeID):
				t.Fatalf("bad")
			default:
				t.Fatalf("bad: %s", id)
			}

			if stat == nil || stat.LastTerm == 0 {
				t.Fatalf("bad: %#v", stat)
			}
		}
	}()
}

// TestStatsFetcher_LocalServerOptimization verifies that the StatsFetcher uses
// the in-memory codec when fetching stats from the local server instead of
// going through the network pool.
func TestStatsFetcher_LocalServerOptimization(t *testing.T) {
	ci.Parallel(t)

	conf := func(c *Config) {
		c.Region = "region-a"
		c.BootstrapExpect = 1
	}

	s1, cleanupS1 := TestServer(t, conf)
	defer cleanupS1()

	testutil.WaitForLeader(t, s1.RPC)

	must.NotNil(t, s1.statsFetcher.localServer)
	must.Eq(t, raft.ServerID(s1.config.NodeID), s1.statsFetcher.localID)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stats := s1.statsFetcher.Fetch(ctx, s1.autopilotServers())
	must.Eq(t, 1, len(stats))

	// verify we got valid stats for the local server
	for id, stat := range stats {
		must.Eq(t, raft.ServerID(s1.config.NodeID), id)
		must.NotNil(t, stat)
		must.NotEq(t, 0, stat.LastTerm)
	}
}

// TestStatsFetcher_LocalAndRemote verifies that the StatsFetcher correctly
// handles a mix of local and remote servers, using local calls for local server
// and the network pool for remote servers.
func TestStatsFetcher_LocalAndRemote(t *testing.T) {
	ci.Parallel(t)

	conf := func(c *Config) {
		c.Region = "region-a"
		c.BootstrapExpect = 2
	}

	s1, cleanupS1 := TestServer(t, conf)
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, conf)
	defer cleanupS2()

	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stats := s1.statsFetcher.Fetch(ctx, s1.autopilotServers())
	must.Eq(t, 2, len(stats))

	// Verify we got valid stats for both servers
	s1ID := raft.ServerID(s1.config.NodeID)
	s2ID := raft.ServerID(s2.config.NodeID)
	foundS1 := false
	foundS2 := false

	for id, stat := range stats {
		if id == s1ID {
			foundS1 = true
		} else if id == s2ID {
			foundS2 = true
		}

		must.NotNil(t, stat)
		must.NotEq(t, 0, stat.LastTerm)
	}

	must.True(t, foundS1 && foundS2)
}
