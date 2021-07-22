package nomad

import (
	"testing"
	"time"

	"fmt"

	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

// wantPeers determines whether the server has the given
// number of voting raft peers.
func wantPeers(s *Server, peers int) error {
	future := s.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return err
	}

	n := autopilot.NumPeers(future.Configuration())
	if got, want := n, peers; got != want {
		return fmt.Errorf("got %d peers want %d", got, want)
	}
	return nil
}

// wantRaft determines if the servers have all of each other in their
// Raft configurations,
func wantRaft(servers []*Server) error {
	// Make sure all the servers are represented in the Raft config,
	// and that there are no extras.
	verifyRaft := func(c raft.Configuration) error {
		want := make(map[raft.ServerID]bool)
		for _, s := range servers {
			want[s.config.RaftConfig.LocalID] = true
		}

		found := make([]raft.ServerID, 0, len(c.Servers))
		for _, s := range c.Servers {
			found = append(found, s.ID)
			if !want[s.ID] {
				return fmt.Errorf("don't want %q", s.ID)
			}
			delete(want, s.ID)
		}

		if len(want) > 0 {
			return fmt.Errorf("didn't find %v in %#+v", want, found)
		}
		return nil
	}

	for _, s := range servers {
		future := s.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			return err
		}
		if err := verifyRaft(future.Configuration()); err != nil {
			return err
		}
	}
	return nil
}

func TestAutopilot_CleanupDeadServer(t *testing.T) {
	t.Parallel()
	for i := 1; i <= 3; i++ {
		testCleanupDeadServer(t, i)
	}
}

func testCleanupDeadServer(t *testing.T, raftVersion int) {
	conf := func(c *Config) {
		c.BootstrapExpect = 3
		c.RaftConfig.ProtocolVersion = raft.ProtocolVersion(raftVersion)
	}

	s1, cleanupS1 := TestServer(t, conf)
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, conf)
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, conf)
	defer cleanupS3()

	servers := []*Server{s1, s2, s3}

	// Try to join
	TestJoin(t, s1, s2, s3)

	for _, s := range servers {
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 3)) })
	}

	// Bring up a new server
	s4, cleanupS4 := TestServer(t, conf)
	defer cleanupS4()

	// Kill a non-leader server
	s3.Shutdown()
	retry.Run(t, func(r *retry.R) {
		alive := 0
		for _, m := range s1.Members() {
			if m.Status == serf.StatusAlive {
				alive++
			}
		}
		if alive != 2 {
			r.Fatal(nil)
		}
	})

	// Join the new server
	TestJoin(t, s1, s4)
	servers[2] = s4

	// Make sure the dead server is removed and we're back to 3 total peers
	for _, s := range servers {
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 3)) })
	}
}

func TestAutopilot_CleanupDeadServerPeriodic(t *testing.T) {
	t.Parallel()

	conf := func(c *Config) {
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

	// Join the servers to s1, and wait until they are all promoted to
	// voters.
	TestJoin(t, servers...)
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 5))
		}
	})

	// Kill a non-leader server
	s4.Shutdown()

	// Should be removed from the peers automatically
	servers = []*Server{s1, s2, s3, s5}
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 4))
		}
	})
}

func TestAutopilot_RollingUpdate(t *testing.T) {
	t.Parallel()

	conf := func(c *Config) {
		c.BootstrapExpect = 3
		c.RaftConfig.ProtocolVersion = 3
	}

	s1, cleanupS1 := TestServer(t, conf)
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, conf)
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, conf)
	defer cleanupS3()

	// Join the servers to s1, and wait until they are all promoted to
	// voters.
	servers := []*Server{s1, s2, s3}
	TestJoin(t, s1, s2, s3)
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 3))
		}
	})

	// Add one more server like we are doing a rolling update.
	t.Logf("adding server s4")
	s4, cleanupS4 := TestServer(t, conf)
	defer cleanupS4()
	TestJoin(t, s1, s4)

	servers = append(servers, s4)
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 4))
		}
	})

	// Now kill one of the "old" nodes like we are doing a rolling update.
	t.Logf("shutting down server s3")
	s3.Shutdown()

	isVoter := func() bool {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
		for _, s := range future.Configuration().Servers {
			if string(s.ID) == string(s4.config.NodeID) {
				return s.Suffrage == raft.Voter
			}
		}
		t.Fatalf("didn't find s4")
		return false
	}

	t.Logf("waiting for s4 to stabalize and be promoted")

	// Wait for s4 to stabilize, get promoted to a voter, and for s3 to be
	// removed.
	servers = []*Server{s1, s2, s4}
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 3))
		}
		if !isVoter() {
			r.Fatalf("should be a voter")
		}
	})
}

func TestAutopilot_CleanupStaleRaftServer(t *testing.T) {
	t.Skip("TestAutopilot_CleanupDeadServer is very flaky, removing it for now")
	t.Parallel()

	conf := func(c *Config) {
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

	// Join the servers to s1
	TestJoin(t, s1, s2, s3)

	leader := waitForStableLeadership(t, servers)

	// Add s4 to peers directly
	addr := fmt.Sprintf("127.0.0.1:%d", s4.config.RPCAddr.Port)
	future := leader.raft.AddVoter(raft.ServerID(s4.config.NodeID), raft.ServerAddress(addr), 0, 0)
	if err := future.Error(); err != nil {
		t.Fatal(err)
	}

	// Verify we have 4 peers
	peers, err := s1.numPeers()
	if err != nil {
		t.Fatal(err)
	}
	if peers != 4 {
		t.Fatalf("bad: %v", peers)
	}

	// Wait for s4 to be removed
	for _, s := range []*Server{s1, s2, s3} {
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 3)) })
	}
}

func TestAutopilot_PromoteNonVoter(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	defer codec.Close()
	testutil.WaitForLeader(t, s1.RPC)

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 0
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS2()
	TestJoin(t, s1, s2)

	// Make sure we see it as a nonvoter initially. We wait until half
	// the stabilization period has passed.
	retry.Run(t, func(r *retry.R) {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			r.Fatal(err)
		}

		servers := future.Configuration().Servers
		if len(servers) != 2 {
			r.Fatalf("bad: %v", servers)
		}
		if servers[1].Suffrage != raft.Nonvoter {
			r.Fatalf("bad: %v", servers)
		}
		health := s1.autopilot.GetServerHealth(string(servers[1].ID))
		if health == nil {
			r.Fatalf("nil health, %v", s1.autopilot.GetClusterHealth())
		}
		if !health.Healthy {
			r.Fatalf("bad: %v", health)
		}
		if time.Since(health.StableSince) < s1.config.AutopilotConfig.ServerStabilizationTime/2 {
			r.Fatal("stable period not elapsed")
		}
	})

	// Make sure it ends up as a voter.
	retry.Run(t, func(r *retry.R) {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			r.Fatal(err)
		}

		servers := future.Configuration().Servers
		if len(servers) != 2 {
			r.Fatalf("bad: %v", servers)
		}
		if servers[1].Suffrage != raft.Voter {
			r.Fatalf("bad: %v", servers)
		}
	})
}
