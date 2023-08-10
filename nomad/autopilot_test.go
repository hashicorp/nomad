// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	autopilot "github.com/hashicorp/raft-autopilot"
	"github.com/hashicorp/serf/serf"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/testutil"
)

var _ autopilot.ApplicationIntegration = (*AutopilotDelegate)(nil)

// wantPeers determines whether the server has the given
// number of voting raft peers.
func wantPeers(s *Server, peers int) error {
	future := s.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return err
	}

	var n int
	for _, server := range future.Configuration().Servers {
		if server.Suffrage == raft.Voter {
			n++
		}
	}

	if got, want := n, peers; got != want {
		return fmt.Errorf("server %v: got %d peers want %d\n\tservers: %#+v", s.config.NodeName, got, want, future.Configuration().Servers)
	}
	return nil
}

func TestAutopilot_CleanupDeadServer(t *testing.T) {
	ci.Parallel(t)

	conf := func(c *Config) {
		c.NumSchedulers = 0 // reduces test log noise
		c.BootstrapExpect = 3
		c.RaftConfig.ProtocolVersion = raft.ProtocolVersion(3)
	}

	s1, cleanupS1 := TestServer(t, conf)
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, conf)
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, conf)
	defer cleanupS3()

	servers := []*Server{s1, s2, s3}
	TestJoin(t, servers...)

	t.Logf("waiting for initial stable cluster")
	waitForStableLeadership(t, servers)

	s4, cleanupS4 := TestServer(t, conf)
	defer cleanupS4()

	// Kill a non-leader server
	killedIdx := 0
	for i, s := range servers {
		if !s.IsLeader() {
			killedIdx = i
			t.Logf("killing a server (index %d)", killedIdx)
			s.Shutdown()
			break
		}
	}

	t.Logf("waiting for server loss to be detected")
	testutil.WaitForResultUntil(10*time.Second, func() (bool, error) {
		for i, s := range servers {
			alive := 0
			if i == killedIdx {
				// Skip shutdown server
				continue
			}
			for _, m := range s.Members() {
				if m.Status == serf.StatusAlive {
					alive++
				}
			}

			if alive != 2 {
				return false, fmt.Errorf("expected 2 alive servers but found %v", alive)
			}
		}
		return true, nil
	}, func(err error) { must.NoError(t, err) })

	// Join the new server
	servers[killedIdx] = s4
	t.Logf("adding server s4")
	TestJoin(t, servers...)

	t.Logf("waiting for dead server to be removed")
	waitForStableLeadership(t, servers)
}

func TestAutopilot_CleanupDeadServerPeriodic(t *testing.T) {
	ci.Parallel(t)

	conf := func(c *Config) {
		c.NumSchedulers = 0 // reduces test log noise
		c.BootstrapExpect = 5
	}

	s1, cleanupS1 := TestServer(t, conf)
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, conf)
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, conf)
	defer cleanupS3()

	s4, cleanupS4 := TestServer(t, conf)
	defer cleanupS4()

	s5, cleanupS5 := TestServer(t, conf)
	defer cleanupS5()

	servers := []*Server{s1, s2, s3, s4, s5}
	TestJoin(t, servers...)

	t.Logf("waiting for initial stable cluster")
	waitForStableLeadership(t, servers)

	t.Logf("killing a non-leader server")
	if leader := waitForStableLeadership(t, servers); leader == s4 {
		s1, s4 = s4, s1
	}
	s4.Shutdown()

	t.Logf("waiting for dead peer to be removed")
	servers = []*Server{s1, s2, s3, s5}
	waitForStableLeadership(t, servers)
}

func TestAutopilot_RollingUpdate(t *testing.T) {
	ci.Parallel(t)

	conf := func(c *Config) {
		c.NumSchedulers = 0 // reduces test log noise
		c.BootstrapExpect = 3
		c.RaftConfig.ProtocolVersion = 3
	}

	s1, cleanupS1 := TestServer(t, conf)
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, conf)
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, conf)
	defer cleanupS3()

	servers := []*Server{s1, s2, s3}
	TestJoin(t, s1, s2, s3)

	t.Logf("waiting for initial stable cluster")
	waitForStableLeadership(t, servers)

	// Add one more server like we are doing a rolling update.
	t.Logf("adding server s4")
	s4, cleanupS4 := TestServer(t, conf)
	defer cleanupS4()
	TestJoin(t, s1, s4)

	// Wait for s4 to stabilize and get promoted to a voter
	t.Logf("waiting for s4 to stabilize and be promoted")
	servers = append(servers, s4)
	waitForStableLeadership(t, servers)

	// Now kill one of the "old" nodes like we are doing a rolling update.
	t.Logf("shutting down server s3")
	s3.Shutdown()

	// Wait for s3 to be removed and the cluster to stablize.
	t.Logf("waiting for cluster to stabilize")
	servers = []*Server{s1, s2, s4}
	waitForStableLeadership(t, servers)
}

func TestAutopilot_MultiRegion(t *testing.T) {
	ci.Parallel(t)

	conf := func(c *Config) {
		c.NumSchedulers = 0 // reduces test log noise
		c.BootstrapExpect = 3
	}
	s1, cleanupS1 := TestServer(t, conf)
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, conf)
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, conf)
	defer cleanupS3()

	// federated regions should not be considered raft peers or show up in the
	// known servers list
	s4, cleanupS4 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 0
		c.Region = "other"
	})
	defer cleanupS4()

	servers := []*Server{s1, s2, s3}
	TestJoin(t, s1, s2, s3, s4)

	t.Logf("waiting for initial stable cluster")
	waitForStableLeadership(t, servers)

	apDelegate := &AutopilotDelegate{s3}
	known := apDelegate.KnownServers()
	must.Eq(t, 3, len(known))

}

func TestAutopilot_CleanupStaleRaftServer(t *testing.T) {
	ci.Parallel(t)

	conf := func(c *Config) {
		c.NumSchedulers = 0 // reduces test log noise
		c.BootstrapExpect = 3
	}
	s1, cleanupS1 := TestServer(t, conf)
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, conf)
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, conf)
	defer cleanupS3()

	s4, cleanupS4 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 0
	})
	defer cleanupS4()

	servers := []*Server{s1, s2, s3}
	TestJoin(t, s1, s2, s3)

	t.Logf("waiting for initial stable cluster")
	leader := waitForStableLeadership(t, servers)

	t.Logf("adding server s4 to peers directly")
	addr := fmt.Sprintf("127.0.0.1:%d", s4.config.RPCAddr.Port)
	future := leader.raft.AddVoter(raft.ServerID(s4.config.NodeID), raft.ServerAddress(addr), 0, 0)
	if err := future.Error(); err != nil {
		t.Fatal(err)
	}

	t.Logf("waiting for 4th server to be removed")
	waitForStableLeadership(t, servers)
}

func TestAutopilot_PromoteNonVoter(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // reduces test log noise
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	defer codec.Close()
	testutil.WaitForLeader(t, s1.RPC)

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // reduces test log noise
		c.BootstrapExpect = 0
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS2()
	TestJoin(t, s1, s2)

	// Note: we can't reliably detect that the server is initially a non-voter,
	// because it can transition too quickly for the test setup to detect,
	// especially in low-resource environments like CI. We'll assume that
	// happens correctly here and only test that it transitions to become a
	// voter.
	testutil.WaitForResultUntil(10*time.Second, func() (bool, error) {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			return false, err
		}
		servers := future.Configuration().Servers
		if len(servers) != 2 {
			return false, fmt.Errorf("expected 2 servers, got: %v", servers)
		}
		if servers[1].Suffrage != raft.Voter {
			return false, fmt.Errorf("expected server to be voter: %v", servers)
		}
		return true, nil
	}, func(err error) { must.NoError(t, err) })

}
