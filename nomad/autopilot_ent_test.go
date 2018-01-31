package nomad

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

func TestAdvancedAutopilot_DesignateNonVoter(t *testing.T) {
	assert := assert.New(t)
	s1 := testServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = 3
	})
	defer s1.Shutdown()

	s2 := testServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
		c.RaftConfig.ProtocolVersion = 3
	})
	defer s2.Shutdown()

	s3 := testServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
		c.NonVoter = true
		c.RaftConfig.ProtocolVersion = 3
	})
	defer s3.Shutdown()

	testutil.WaitForLeader(t, s1.RPC)

	testJoin(t, s1, s2, s3)
	retry.Run(t, func(r *retry.R) { r.Check(wantRaft([]*Server{s1, s2, s3})) })

	// Wait twice the server stabilization threshold to give the server
	// time to be promoted
	time.Sleep(2 * s1.config.AutopilotConfig.ServerStabilizationTime)

	future := s1.raft.GetConfiguration()
	assert.Nil(future.Error())

	servers := future.Configuration().Servers

	// s2 should be a voter
	if !autopilot.IsPotentialVoter(servers[1].Suffrage) || servers[1].ID != s2.config.RaftConfig.LocalID {
		t.Fatalf("bad: %v", servers)
	}

	// s3 should remain a non-voter
	if autopilot.IsPotentialVoter(servers[2].Suffrage) || servers[2].ID != s3.config.RaftConfig.LocalID {
		t.Fatalf("bad: %v", servers)
	}
}

func TestAdvancedAutopilot_RedundancyZone(t *testing.T) {
	assert := assert.New(t)
	s1 := testServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = 3
		c.AutopilotConfig.EnableRedundancyZones = true
		c.RedundancyZone = "east"
	})
	defer s1.Shutdown()

	s2 := testServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
		c.RaftConfig.ProtocolVersion = 3
		c.RedundancyZone = "west"
	})
	defer s2.Shutdown()

	s3 := testServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
		c.RaftConfig.ProtocolVersion = 3
		c.RedundancyZone = "west-2"
	})
	defer s3.Shutdown()

	testutil.WaitForLeader(t, s1.RPC)

	testJoin(t, s1, s2, s3)
	retry.Run(t, func(r *retry.R) { r.Check(wantRaft([]*Server{s1, s2, s3})) })

	// Wait past the stabilization time to give the servers a chance to be promoted
	time.Sleep(2*s1.config.AutopilotConfig.ServerStabilizationTime + s1.config.AutopilotInterval)

	// Now s2 and s3 should be voters
	{
		future := s1.raft.GetConfiguration()
		assert.Nil(future.Error())
		servers := future.Configuration().Servers
		assert.Equal(3, len(servers))
		// s2 and s3 should be voters now
		assert.Equal(raft.Voter, servers[1].Suffrage)
		assert.Equal(raft.Voter, servers[2].Suffrage)
	}

	// Join s4
	s4 := testServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
		c.RaftConfig.ProtocolVersion = 3
		c.RedundancyZone = "west-2"
	})
	defer s4.Shutdown()
	testJoin(t, s1, s4)
	time.Sleep(2*s1.config.AutopilotConfig.ServerStabilizationTime + s1.config.AutopilotInterval)

	// s4 should not be a voter yet
	{
		future := s1.raft.GetConfiguration()
		assert.Nil(future.Error())
		servers := future.Configuration().Servers
		assert.Equal(raft.Nonvoter, servers[3].Suffrage)
	}

	s3.Shutdown()

	// s4 should be a voter now, s3 should be removed
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft([]*Server{s1, s2, s4}))
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			r.Fatal(err)
		}
		servers := future.Configuration().Servers
		for _, s := range servers {
			if s.Suffrage != raft.Voter {
				r.Fatalf("bad: %v", servers)
			}
		}
	})
}

func TestAdvancedAutopilot_UpgradeMigration(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = 3
		c.Build = "0.8.0"
	})
	defer s1.Shutdown()

	s2 := testServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
		c.RaftConfig.ProtocolVersion = 3
		c.Build = "0.8.1"
	})
	defer s2.Shutdown()

	testutil.WaitForLeader(t, s1.RPC)
	testJoin(t, s1, s2)

	// Wait for the migration to complete
	retry.Run(t, func(r *retry.R) {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			r.Fatal(err)
		}
		for _, s := range future.Configuration().Servers {
			switch s.ID {
			case raft.ServerID(s1.config.NodeID):
				if got, want := s.Suffrage, raft.Nonvoter; got != want {
					r.Fatalf("got %v want %v", got, want)
				}

			case raft.ServerID(s2.config.NodeID):
				if got, want := s.Suffrage, raft.Voter; got != want {
					r.Fatalf("got %v want %v", got, want)
				}

			default:
				r.Fatalf("unexpected server %s", s.ID)
			}
		}

		if !s2.IsLeader() {
			r.Fatal("server 2 should be the leader")
		}
	})
}

func TestAdvancedAutopilot_CustomUpgradeMigration(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = 3
		c.AutopilotConfig.EnableCustomUpgrades = true
		c.UpgradeVersion = "0.0.1"
	})
	defer s1.Shutdown()

	s2 := testServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
		c.RaftConfig.ProtocolVersion = 3
		c.UpgradeVersion = "0.0.2"
	})
	defer s2.Shutdown()

	testutil.WaitForLeader(t, s1.RPC)
	testJoin(t, s1, s2)

	// Wait for the migration to complete
	retry.Run(t, func(r *retry.R) {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			r.Fatal(err)
		}
		for _, s := range future.Configuration().Servers {
			switch s.ID {
			case raft.ServerID(s1.config.NodeID):
				if got, want := s.Suffrage, raft.Nonvoter; got != want {
					r.Fatalf("got %v want %v", got, want)
				}

			case raft.ServerID(s2.config.NodeID):
				if got, want := s.Suffrage, raft.Voter; got != want {
					r.Fatalf("got %v want %v", got, want)
				}

			default:
				r.Fatalf("unexpected server %s", s.ID)
			}
		}

		if !s2.IsLeader() {
			r.Fatal("server 2 should be the leader")
		}
	})
}

func TestAdvancedAutopilot_DisableUpgradeMigration(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = 3
		c.AutopilotConfig.DisableUpgradeMigration = true
		c.Build = "0.8.0"
	})
	defer s1.Shutdown()

	testutil.WaitForLeader(t, s1.RPC)

	s2 := testServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
		c.RaftConfig.ProtocolVersion = 3
		c.Build = "0.8.0"
	})
	defer s2.Shutdown()

	s3 := testServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
		c.RaftConfig.ProtocolVersion = 3
		c.Build = "0.8.1"
	})
	defer s3.Shutdown()

	testJoin(t, s1, s2, s3)

	// Wait for both servers to be added as voters
	retry.Run(t, func(r *retry.R) {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			r.Fatal(err)
		}

		for _, s := range future.Configuration().Servers {
			if got, want := s.Suffrage, raft.Voter; got != want {
				r.Fatalf("got %v want %v", got, want)
			}
		}

		if !s1.IsLeader() {
			r.Fatal("server 1 should be leader")
		}
	})
}
