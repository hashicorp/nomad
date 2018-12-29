package nomad

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/nomad/testutil"
)

func TestStatsFetcher(t *testing.T) {
	t.Parallel()

	conf := func(c *Config) {
		c.Region = "region-a"
		c.DevDisableBootstrap = true
		c.BootstrapExpect = 3
	}

	s1 := TestServer(t, conf)
	defer s1.Shutdown()

	s2 := TestServer(t, conf)
	defer s2.Shutdown()

	s3 := TestServer(t, conf)
	defer s3.Shutdown()

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
		stats := s1.statsFetcher.Fetch(ctx, s1.Members())
		if len(stats) != 3 {
			t.Fatalf("bad: %#v", stats)
		}
		for id, stat := range stats {
			switch id {
			case s1.config.NodeID, s2.config.NodeID, s3.config.NodeID:
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
		s1.statsFetcher.inflight[string(s3.config.NodeID)] = struct{}{}
		defer delete(s1.statsFetcher.inflight, string(s3.config.NodeID))

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		stats := s1.statsFetcher.Fetch(ctx, s1.Members())
		if len(stats) != 2 {
			t.Fatalf("bad: %#v", stats)
		}
		for id, stat := range stats {
			switch id {
			case s1.config.NodeID, s2.config.NodeID:
				// OK
			case s3.config.NodeID:
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
