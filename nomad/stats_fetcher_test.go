// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
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

	var servers []*serverParts
	for _, member := range members {
		ok, server := isNomadServer(member)
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
		s1.statsFetcher.inflight[raft.ServerID(s3.config.NodeID)] = struct{}{}
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
