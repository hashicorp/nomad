package nomad

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/testutil"
)

func TestLeader_LeftServer(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()

	s2 := testServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s2.Shutdown()

	s3 := testServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s3.Shutdown()
	servers := []*Server{s1, s2, s3}

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfConfig.MemberlistConfig.BindPort)
	if _, err := s2.Join([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := s3.Join([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.raftPeers.Peers()
			return len(peers) == 3, nil
		}, func(err error) {
			t.Fatalf("should have 3 peers")
		})
	}

	// Kill any server
	servers[0].Shutdown()

	testutil.WaitForResult(func() (bool, error) {
		// Force remove the non-leader (transition to left state)
		name := fmt.Sprintf("%s.%s",
			servers[0].config.NodeName, servers[0].config.Region)
		if err := servers[1].RemoveFailedNode(name); err != nil {
			t.Fatalf("err: %v", err)
		}

		for _, s := range servers[1:] {
			peers, _ := s.raftPeers.Peers()
			return len(peers) == 2, errors.New(fmt.Sprintf("%v", peers))
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestLeader_LeftLeader(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()

	s2 := testServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s2.Shutdown()

	s3 := testServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s3.Shutdown()
	servers := []*Server{s1, s2, s3}

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfConfig.MemberlistConfig.BindPort)
	if _, err := s2.Join([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := s3.Join([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.raftPeers.Peers()
			return len(peers) == 3, nil
		}, func(err error) {
			t.Fatalf("should have 3 peers")
		})
	}

	// Kill the leader!
	var leader *Server
	for _, s := range servers {
		if s.IsLeader() {
			leader = s
			break
		}
	}
	if leader == nil {
		t.Fatalf("Should have a leader")
	}
	leader.Leave()
	leader.Shutdown()

	for _, s := range servers {
		if s == leader {
			continue
		}
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.raftPeers.Peers()
			return len(peers) == 2, errors.New(fmt.Sprintf("%v", peers))
		}, func(err error) {
			t.Fatalf("should have 2 peers: %v", err)
		})
	}
}

func TestLeader_MultiBootstrap(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()

	s2 := testServer(t, nil)
	defer s2.Shutdown()
	servers := []*Server{s1, s2}

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfConfig.MemberlistConfig.BindPort)
	if _, err := s2.Join([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers := s.Members()
			return len(peers) == 2, nil
		}, func(err error) {
			t.Fatalf("should have 2 peers")
		})
	}

	// Ensure we don't have multiple raft peers
	for _, s := range servers {
		peers, _ := s.raftPeers.Peers()
		if len(peers) != 1 {
			t.Fatalf("should only have 1 raft peer!")
		}
	}
}
