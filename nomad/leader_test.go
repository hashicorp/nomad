package nomad

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
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
	testJoin(t, s1, s2, s3)

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
	testJoin(t, s1, s2, s3)

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
	testJoin(t, s1, s2)

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

func TestLeader_PlanQueue_Reset(t *testing.T) {
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
	testJoin(t, s1, s2, s3)

	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.raftPeers.Peers()
			return len(peers) == 3, nil
		}, func(err error) {
			t.Fatalf("should have 3 peers")
		})
	}

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

	if !leader.planQueue.Enabled() {
		t.Fatalf("should enable plan queue")
	}

	for _, s := range servers {
		if !s.IsLeader() && s.planQueue.Enabled() {
			t.Fatalf("plan queue should not be enabled")
		}
	}

	// Kill the leader
	leader.Shutdown()
	time.Sleep(100 * time.Millisecond)

	// Wait for a new leader
	leader = nil
	testutil.WaitForResult(func() (bool, error) {
		for _, s := range servers {
			if s.IsLeader() {
				leader = s
				return true, nil
			}
		}
		return false, nil
	}, func(err error) {
		t.Fatalf("should have leader")
	})

	// Check that the new leader has a pending GC expiration
	testutil.WaitForResult(func() (bool, error) {
		return leader.planQueue.Enabled(), nil
	}, func(err error) {
		t.Fatalf("should enable plan queue")
	})
}

func TestLeader_EvalBroker_Reset(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer s1.Shutdown()

	s2 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.DevDisableBootstrap = true
	})
	defer s2.Shutdown()

	s3 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.DevDisableBootstrap = true
	})
	defer s3.Shutdown()
	servers := []*Server{s1, s2, s3}
	testJoin(t, s1, s2, s3)

	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.raftPeers.Peers()
			return len(peers) == 3, nil
		}, func(err error) {
			t.Fatalf("should have 3 peers")
		})
	}

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

	// Inject a pending eval
	req := structs.EvalUpdateRequest{
		Evals: []*structs.Evaluation{mock.Eval()},
	}
	_, _, err := leader.raftApply(structs.EvalUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Kill the leader
	leader.Shutdown()
	time.Sleep(100 * time.Millisecond)

	// Wait for a new leader
	leader = nil
	testutil.WaitForResult(func() (bool, error) {
		for _, s := range servers {
			if s.IsLeader() {
				leader = s
				return true, nil
			}
		}
		return false, nil
	}, func(err error) {
		t.Fatalf("should have leader")
	})

	// Check that the new leader has a pending evaluation
	testutil.WaitForResult(func() (bool, error) {
		stats := leader.evalBroker.Stats()
		return stats.TotalReady == 1, nil
	}, func(err error) {
		t.Fatalf("should have pending evaluation")
	})
}

func TestLeader_PeriodicDispatch(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EvalGCInterval = 5 * time.Millisecond
	})
	defer s1.Shutdown()

	// Wait for a periodic dispatch
	testutil.WaitForResult(func() (bool, error) {
		stats := s1.evalBroker.Stats()
		bySched, ok := stats.ByScheduler[structs.JobTypeCore]
		if !ok {
			return false, nil
		}
		return bySched.Ready > 0, nil
	}, func(err error) {
		t.Fatalf("should pending job")
	})
}

func TestLeader_ReapFailedEval(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EvalDeliveryLimit = 1
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Wait for a periodic dispatch
	eval := mock.Eval()
	testutil.WaitForResult(func() (bool, error) {
		err := s1.evalBroker.Enqueue(eval)
		return err == nil, err
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Dequeue and Nack
	out, token, err := s1.evalBroker.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	s1.evalBroker.Nack(out.ID, token)

	// Wait updated evaluation
	state := s1.fsm.State()
	testutil.WaitForResult(func() (bool, error) {
		out, err := state.GetEvalByID(eval.ID)
		if err != nil {
			return false, err
		}
		return out != nil && out.Status == structs.EvalStatusFailed, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}
