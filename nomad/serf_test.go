package nomad

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/serf/serf"
)

func TestNomad_JoinPeer(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	s2 := testServer(t, func(c *Config) {
		c.Region = "region2"
	})
	defer s2.Shutdown()
	testJoin(t, s1, s2)

	testutil.WaitForResult(func() (bool, error) {
		if members := s1.Members(); len(members) != 2 {
			return false, fmt.Errorf("bad: %#v", members)
		}
		if members := s2.Members(); len(members) != 2 {
			return false, fmt.Errorf("bad: %#v", members)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	testutil.WaitForResult(func() (bool, error) {
		if len(s1.peers) != 2 {
			return false, fmt.Errorf("bad: %#v", s1.peers)
		}
		if len(s2.peers) != 2 {
			return false, fmt.Errorf("bad: %#v", s2.peers)
		}
		if len(s1.localPeers) != 1 {
			return false, fmt.Errorf("bad: %#v", s1.localPeers)
		}
		if len(s2.localPeers) != 1 {
			return false, fmt.Errorf("bad: %#v", s2.localPeers)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestNomad_RemovePeer(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	s2 := testServer(t, func(c *Config) {
		c.Region = "region2"
	})
	defer s2.Shutdown()
	testJoin(t, s1, s2)

	testutil.WaitForResult(func() (bool, error) {
		if members := s1.Members(); len(members) != 2 {
			return false, fmt.Errorf("bad: %#v", members)
		}
		if members := s2.Members(); len(members) != 2 {
			return false, fmt.Errorf("bad: %#v", members)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Leave immediately
	s2.Leave()
	s2.Shutdown()

	testutil.WaitForResult(func() (bool, error) {
		if len(s1.peers) != 1 {
			return false, fmt.Errorf("bad: %#v", s1.peers)
		}
		if len(s2.peers) != 1 {
			return false, fmt.Errorf("bad: %#v", s2.peers)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestNomad_ReapPeer(t *testing.T) {
	t.Parallel()
	dir := tmpDir(t)
	defer os.RemoveAll(dir)
	s1 := testServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node1")
	})
	defer s1.Shutdown()
	s2 := testServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node2")
	})
	defer s2.Shutdown()
	s3 := testServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node3")
	})
	defer s3.Shutdown()
	testJoin(t, s1, s2, s3)

	testutil.WaitForResult(func() (bool, error) {
		if members := s1.Members(); len(members) != 3 {
			return false, fmt.Errorf("bad: %#v", members)
		}
		if members := s2.Members(); len(members) != 3 {
			return false, fmt.Errorf("bad: %#v", members)
		}
		if members := s3.Members(); len(members) != 3 {
			return false, fmt.Errorf("bad: %#v", members)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	testutil.WaitForLeader(t, s1.RPC)

	// Simulate a reap
	mems := s1.Members()
	var s2mem serf.Member
	for _, m := range mems {
		if strings.Contains(m.Name, s2.config.NodeName) {
			s2mem = m
			s2mem.Status = StatusReap
			break
		}
	}

	// Shutdown and then send the reap
	s2.Shutdown()
	s1.reconcileCh <- s2mem
	s2.reconcileCh <- s2mem
	s3.reconcileCh <- s2mem

	testutil.WaitForResult(func() (bool, error) {
		if len(s1.peers["global"]) != 2 {
			return false, fmt.Errorf("bad: %#v", s1.peers["global"])
		}
		peers, err := s1.numPeers()
		if err != nil {
			return false, fmt.Errorf("numPeers() failed: %v", err)
		}
		if peers != 2 {
			return false, fmt.Errorf("bad: %#v", peers)
		}

		if len(s3.peers["global"]) != 2 {
			return false, fmt.Errorf("bad: %#v", s1.peers["global"])
		}
		peers, err = s3.numPeers()
		if err != nil {
			return false, fmt.Errorf("numPeers() failed: %v", err)
		}
		if peers != 2 {
			return false, fmt.Errorf("bad: %#v", peers)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestNomad_BootstrapExpect(t *testing.T) {
	t.Parallel()
	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	s1 := testServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node1")
	})
	defer s1.Shutdown()
	s2 := testServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node2")
	})
	defer s2.Shutdown()
	s3 := testServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node3")
	})
	defer s3.Shutdown()
	testJoin(t, s1, s2, s3)

	testutil.WaitForResult(func() (bool, error) {
		peers, err := s1.numPeers()
		if err != nil {
			return false, err
		}
		if peers != 3 {
			return false, fmt.Errorf("bad: %#v", peers)
		}
		peers, err = s2.numPeers()
		if err != nil {
			return false, err
		}
		if peers != 3 {
			return false, fmt.Errorf("bad: %#v", peers)
		}
		peers, err = s3.numPeers()
		if err != nil {
			return false, err
		}
		if peers != 3 {
			return false, fmt.Errorf("bad: %#v", peers)
		}
		if len(s1.localPeers) != 3 {
			return false, fmt.Errorf("bad: %#v", s1.localPeers)
		}
		if len(s2.localPeers) != 3 {
			return false, fmt.Errorf("bad: %#v", s2.localPeers)
		}
		if len(s3.localPeers) != 3 {
			return false, fmt.Errorf("bad: %#v", s3.localPeers)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Join a fourth server after quorum has already been formed and ensure
	// there is no election
	s4 := testServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.DevMode = false
		c.DevDisableBootstrap = true
		c.DataDir = path.Join(dir, "node4")
	})
	defer s4.Shutdown()

	// Make sure a leader is elected, grab the current term and then add in
	// the fourth server.
	testutil.WaitForLeader(t, s1.RPC)
	termBefore := s1.raft.Stats()["last_log_term"]
	addr := fmt.Sprintf("127.0.0.1:%d", s1.config.SerfConfig.MemberlistConfig.BindPort)
	if _, err := s4.Join([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for the new server to see itself added to the cluster.
	var p4 int
	testutil.WaitForResult(func() (bool, error) {
		p4, _ = s4.numPeers()
		return p4 == 4, errors.New(fmt.Sprintf("%d", p4))
	}, func(err error) {
		t.Fatalf("should have 4 peers: %v", err)
	})

	// Make sure there's still a leader and that the term didn't change,
	// so we know an election didn't occur.
	testutil.WaitForLeader(t, s1.RPC)
	termAfter := s1.raft.Stats()["last_log_term"]
	if termAfter != termBefore {
		t.Fatalf("looks like an election took place")
	}
}

func TestNomad_BadExpect(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.DevDisableBootstrap = true
	})
	defer s1.Shutdown()
	s2 := testServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.DevDisableBootstrap = true
	})
	defer s2.Shutdown()
	servers := []*Server{s1, s2}
	testJoin(t, s1, s2)

	// Serf members should update
	testutil.WaitForResult(func() (bool, error) {
		for _, s := range servers {
			members := s.Members()
			if len(members) != 2 {
				return false, fmt.Errorf("%d", len(members))
			}
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("should have 2 peers: %v", err)
	})

	// should still have no peers (because s2 is in expect=2 mode)
	testutil.WaitForResult(func() (bool, error) {
		for _, s := range servers {
			p, _ := s.numPeers()
			if p != 1 {
				return false, fmt.Errorf("%d", p)
			}
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("should have 0 peers: %v", err)
	})
}
