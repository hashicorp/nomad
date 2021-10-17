package nomad

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
)

func TestNomad_JoinPeer(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.Region = "region2"
	})
	defer cleanupS2()
	TestJoin(t, s1, s2)

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

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.Region = "global"
	})
	defer cleanupS2()
	TestJoin(t, s1, s2)

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

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NodeName = "node1"
		c.BootstrapExpect = 3
		c.DevMode = false
		c.RaftConfig = raft.DefaultConfig()
		c.DataDir = path.Join(dir, "node1")
	})
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.NodeName = "node2"
		c.BootstrapExpect = 3
		c.DevMode = false
		c.RaftConfig = raft.DefaultConfig()
		c.DataDir = path.Join(dir, "node2")
	})
	defer cleanupS2()
	s3, cleanupS3 := TestServer(t, func(c *Config) {
		c.NodeName = "node3"
		c.BootstrapExpect = 3
		c.DevMode = false
		c.RaftConfig = raft.DefaultConfig()
		c.DataDir = path.Join(dir, "node3")
	})
	defer cleanupS3()
	TestJoin(t, s1, s2, s3)

	testutil.WaitForResult(func() (bool, error) {
		// Retry the join to decrease flakiness
		TestJoin(t, s1, s2, s3)
		if members := s1.Members(); len(members) != 3 {
			return false, fmt.Errorf("bad s1: %#v", members)
		}
		if members := s2.Members(); len(members) != 3 {
			return false, fmt.Errorf("bad s2: %#v", members)
		}
		if members := s3.Members(); len(members) != 3 {
			return false, fmt.Errorf("bad s3: %#v", members)
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

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.DevMode = false
		c.RaftConfig = raft.DefaultConfig()
		c.DataDir = path.Join(dir, "node1")
	})
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.DevMode = false
		c.RaftConfig = raft.DefaultConfig()
		c.DataDir = path.Join(dir, "node2")
	})
	defer cleanupS2()
	s3, cleanupS3 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.DevMode = false
		c.RaftConfig = raft.DefaultConfig()
		c.DataDir = path.Join(dir, "node3")
	})
	defer cleanupS3()
	TestJoin(t, s1, s2, s3)

	// Join a fourth server after quorum has already been formed and ensure
	// there is no election
	s4, cleanupS4 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.DevMode = false
		c.RaftConfig = raft.DefaultConfig()
		c.DataDir = path.Join(dir, "node4")
	})
	defer cleanupS4()

	// Make sure a leader is elected, grab the current term and then add in
	// the fourth server.
	t.Logf("waiting for stable leadership and up to date leadership")
	leader := waitForStableLeadership(t, []*Server{s1, s2, s3})
	require.NoError(t, leader.raft.Barrier(10*time.Second).Error())

	termBefore := leader.raft.Stats()["last_log_term"]
	t.Logf("got term: %v\n%#+v", termBefore, leader.raft.Stats())

	var addresses []string
	for _, s := range []*Server{s1, s2, s3} {
		addr := fmt.Sprintf("127.0.0.1:%d", s.config.SerfConfig.MemberlistConfig.BindPort)
		addresses = append(addresses, addr)
	}

	// Wait for the new server to see itself added to the cluster.
	testutil.WaitForResult(func() (bool, error) {
		// Retry join to reduce flakiness
		if _, err := s4.Join(addresses); err != nil {
			return false, fmt.Errorf("failed to to join addresses: %v", err)
		}
		p4, _ := s4.numPeers()
		if p4 != 4 {
			return false, fmt.Errorf("expected %d peers found %d", 4, p4)
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// Make sure there's still a leader and that the term didn't change,
	// so we know an election didn't occur.
	leader = waitForStableLeadership(t, []*Server{s1, s2, s3, s4})
	require.NoError(t, leader.raft.Barrier(10*time.Second).Error())
	termAfter := leader.raft.Stats()["last_log_term"]
	require.Equal(t, termBefore, termAfter, "expected no election")
}

func TestNomad_BootstrapExpect_NonVoter(t *testing.T) {
	t.Parallel()

	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DataDir = path.Join(dir, "node1")
		c.NonVoter = true
	})
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DataDir = path.Join(dir, "node2")
		c.NonVoter = true
	})
	defer cleanupS2()
	s3, cleanupS3 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DataDir = path.Join(dir, "node3")
	})
	defer cleanupS3()
	TestJoin(t, s1, s2, s3)

	// Assert that we do not bootstrap
	testutil.AssertUntil(testutil.Timeout(time.Second), func() (bool, error) {
		_, p := s1.getLeader()
		if p != nil {
			return false, fmt.Errorf("leader %v", p)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("should not have leader: %v", err)
	})

	// Add the fourth server that is a voter
	s4, cleanupS4 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DataDir = path.Join(dir, "node4")
	})
	defer cleanupS4()
	TestJoin(t, s1, s2, s3, s4)

	testutil.WaitForResult(func() (bool, error) {
		// Retry the join to decrease flakiness
		TestJoin(t, s1, s2, s3, s4)
		peers, err := s1.numPeers()
		if err != nil {
			return false, err
		}
		if peers != 4 {
			return false, fmt.Errorf("bad: %#v", peers)
		}
		peers, err = s2.numPeers()
		if err != nil {
			return false, err
		}
		if peers != 4 {
			return false, fmt.Errorf("bad: %#v", peers)
		}
		peers, err = s3.numPeers()
		if err != nil {
			return false, err
		}
		if peers != 4 {
			return false, fmt.Errorf("bad: %#v", peers)
		}
		peers, err = s4.numPeers()
		if err != nil {
			return false, err
		}
		if peers != 4 {
			return false, fmt.Errorf("bad: %#v", peers)
		}

		if len(s1.localPeers) != 4 {
			return false, fmt.Errorf("bad: %#v", s1.localPeers)
		}
		if len(s2.localPeers) != 4 {
			return false, fmt.Errorf("bad: %#v", s2.localPeers)
		}
		if len(s3.localPeers) != 4 {
			return false, fmt.Errorf("bad: %#v", s3.localPeers)
		}
		if len(s4.localPeers) != 4 {
			return false, fmt.Errorf("bad: %#v", s3.localPeers)
		}

		_, p := s1.getLeader()
		if p == nil {
			return false, fmt.Errorf("no leader")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

}

func TestNomad_BadExpect(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
	})
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
	})
	defer cleanupS2()
	servers := []*Server{s1, s2}
	TestJoin(t, s1, s2)

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
			if p != 0 {
				return false, fmt.Errorf("%d", p)
			}
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("should have 0 peers: %v", err)
	})
}

// TestNomad_NonBootstraping_ShouldntBootstap asserts that if BootstrapExpect is zero,
// the server shouldn't bootstrap
func TestNomad_NonBootstraping_ShouldntBootstap(t *testing.T) {
	t.Parallel()

	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 0
		c.DevMode = false
		c.DataDir = path.Join(dir, "node")
	})
	defer cleanupS1()

	testutil.WaitForResult(func() (bool, error) {
		s1.peerLock.Lock()
		p := len(s1.localPeers)
		s1.peerLock.Unlock()
		if p != 1 {
			return false, fmt.Errorf("%d", p)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("expected 1 local peer: %v", err)
	})

	// as non-bootstrap mode is the initial state, we must wait long enough to assert that
	// we don't bootstrap even if enough time has elapsed.  Also, explicitly attempt bootstrap.
	s1.maybeBootstrap()
	time.Sleep(100 * time.Millisecond)

	bootstrapped := atomic.LoadInt32(&s1.config.Bootstrapped)
	require.Zero(t, bootstrapped, "expecting non-bootstrapped servers")

	p, _ := s1.numPeers()
	require.Zero(t, p, "number of peers in Raft")

}
