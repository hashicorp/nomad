package nomad

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
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
			peers, _ := s.numPeers()
			return peers == 3, nil
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
			peers, _ := s.numPeers()
			return peers == 2, errors.New(fmt.Sprintf("%v", peers))
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
			peers, _ := s.numPeers()
			return peers == 3, nil
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
			peers, _ := s.numPeers()
			return peers == 2, errors.New(fmt.Sprintf("%v", peers))
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
		peers, _ := s.numPeers()
		if peers != 1 {
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
			peers, _ := s.numPeers()
			return peers == 3, nil
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
	testutil.WaitForLeader(t, s1.RPC)

	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.numPeers()
			return peers == 3, nil
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

func TestLeader_PeriodicDispatcher_Restore_Adds(t *testing.T) {
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
	testutil.WaitForLeader(t, s1.RPC)

	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.numPeers()
			return peers == 3, nil
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

	// Inject a periodic job, a parameterized periodic job and a non-periodic job
	periodic := mock.PeriodicJob()
	nonPeriodic := mock.Job()
	parameterizedPeriodic := mock.PeriodicJob()
	parameterizedPeriodic.ParameterizedJob = &structs.ParameterizedJobConfig{}
	for _, job := range []*structs.Job{nonPeriodic, periodic, parameterizedPeriodic} {
		req := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Namespace: job.Namespace,
			},
		}
		_, _, err := leader.raftApply(structs.JobRegisterRequestType, req)
		if err != nil {
			t.Fatalf("err: %v", err)
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

	tuplePeriodic := structs.NamespacedID{
		ID:        periodic.ID,
		Namespace: periodic.Namespace,
	}
	tupleNonPeriodic := structs.NamespacedID{
		ID:        nonPeriodic.ID,
		Namespace: nonPeriodic.Namespace,
	}
	tupleParameterized := structs.NamespacedID{
		ID:        parameterizedPeriodic.ID,
		Namespace: parameterizedPeriodic.Namespace,
	}

	// Check that the new leader is tracking the periodic job only
	testutil.WaitForResult(func() (bool, error) {
		if _, tracked := leader.periodicDispatcher.tracked[tuplePeriodic]; !tracked {
			return false, fmt.Errorf("periodic job not tracked")
		}
		if _, tracked := leader.periodicDispatcher.tracked[tupleNonPeriodic]; tracked {
			return false, fmt.Errorf("non periodic job tracked")
		}
		if _, tracked := leader.periodicDispatcher.tracked[tupleParameterized]; tracked {
			return false, fmt.Errorf("parameterized periodic job tracked")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf(err.Error())
	})
}

func TestLeader_PeriodicDispatcher_Restore_NoEvals(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Inject a periodic job that will be triggered soon.
	launch := time.Now().Add(1 * time.Second)
	job := testPeriodicJob(launch)
	req := structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	_, _, err := s1.raftApply(structs.JobRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Flush the periodic dispatcher, ensuring that no evals will be created.
	s1.periodicDispatcher.SetEnabled(false)

	// Get the current time to ensure the launch time is after this once we
	// restore.
	now := time.Now()

	// Sleep till after the job should have been launched.
	time.Sleep(3 * time.Second)

	// Restore the periodic dispatcher.
	s1.periodicDispatcher.SetEnabled(true)
	s1.restorePeriodicDispatcher()

	// Ensure the job is tracked.
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	if _, tracked := s1.periodicDispatcher.tracked[tuple]; !tracked {
		t.Fatalf("periodic job not restored")
	}

	// Check that an eval was made.
	ws := memdb.NewWatchSet()
	last, err := s1.fsm.State().PeriodicLaunchByID(ws, job.Namespace, job.ID)
	if err != nil || last == nil {
		t.Fatalf("failed to get periodic launch time: %v", err)
	}

	if last.Launch.Before(now) {
		t.Fatalf("restorePeriodicDispatcher did not force launch: last %v; want after %v", last.Launch, now)
	}
}

func TestLeader_PeriodicDispatcher_Restore_Evals(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Inject a periodic job that triggered once in the past, should trigger now
	// and once in the future.
	now := time.Now()
	past := now.Add(-1 * time.Second)
	future := now.Add(10 * time.Second)
	job := testPeriodicJob(past, now, future)
	req := structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	_, _, err := s1.raftApply(structs.JobRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create an eval for the past launch.
	s1.periodicDispatcher.createEval(job, past)

	// Flush the periodic dispatcher, ensuring that no evals will be created.
	s1.periodicDispatcher.SetEnabled(false)

	// Sleep till after the job should have been launched.
	time.Sleep(3 * time.Second)

	// Restore the periodic dispatcher.
	s1.periodicDispatcher.SetEnabled(true)
	s1.restorePeriodicDispatcher()

	// Ensure the job is tracked.
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	if _, tracked := s1.periodicDispatcher.tracked[tuple]; !tracked {
		t.Fatalf("periodic job not restored")
	}

	// Check that an eval was made.
	ws := memdb.NewWatchSet()
	last, err := s1.fsm.State().PeriodicLaunchByID(ws, job.Namespace, job.ID)
	if err != nil || last == nil {
		t.Fatalf("failed to get periodic launch time: %v", err)
	}
	if last.Launch == past {
		t.Fatalf("restorePeriodicDispatcher did not force launch")
	}
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
	s1.evalBroker.Enqueue(eval)

	// Dequeue and Nack
	out, token, err := s1.evalBroker.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	s1.evalBroker.Nack(out.ID, token)

	// Wait for an updated and followup evaluation
	state := s1.fsm.State()
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		out, err := state.EvalByID(ws, eval.ID)
		if err != nil {
			return false, err
		}
		if out == nil {
			return false, fmt.Errorf("expect original evaluation to exist")
		}
		if out.Status != structs.EvalStatusFailed {
			return false, fmt.Errorf("got status %v; want %v", out.Status, structs.EvalStatusFailed)
		}

		// See if there is a followup
		evals, err := state.EvalsByJob(ws, eval.Namespace, eval.JobID)
		if err != nil {
			return false, err
		}

		if l := len(evals); l != 2 {
			return false, fmt.Errorf("got %d evals, want 2", l)
		}

		for _, e := range evals {
			if e.ID == eval.ID {
				continue
			}

			if e.Status != structs.EvalStatusPending {
				return false, fmt.Errorf("follow up eval has status %v; want %v",
					e.Status, structs.EvalStatusPending)
			}

			if e.Wait < s1.config.EvalFailedFollowupBaselineDelay ||
				e.Wait > s1.config.EvalFailedFollowupBaselineDelay+s1.config.EvalFailedFollowupDelayRange {
				return false, fmt.Errorf("bad wait: %v", e.Wait)
			}

			if e.TriggeredBy != structs.EvalTriggerFailedFollowUp {
				return false, fmt.Errorf("follow up eval TriggeredBy %v; want %v",
					e.TriggeredBy, structs.EvalTriggerFailedFollowUp)
			}
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestLeader_ReapDuplicateEval(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a duplicate blocked eval
	eval := mock.Eval()
	eval2 := mock.Eval()
	eval2.JobID = eval.JobID
	s1.blockedEvals.Block(eval)
	s1.blockedEvals.Block(eval2)

	// Wait for the evaluation to marked as cancelled
	state := s1.fsm.State()
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		out, err := state.EvalByID(ws, eval2.ID)
		if err != nil {
			return false, err
		}
		return out != nil && out.Status == structs.EvalStatusCancelled, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestLeader_RestoreVaultAccessors(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Insert a vault accessor that should be revoked
	state := s1.fsm.State()
	va := mock.VaultAccessor()
	if err := state.UpsertVaultAccessor(100, []*structs.VaultAccessor{va}); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Swap the Vault client
	tvc := &TestVaultClient{}
	s1.vault = tvc

	// Do a restore
	if err := s1.restoreRevokingAccessors(); err != nil {
		t.Fatalf("Failed to restore: %v", err)
	}

	if len(tvc.RevokedTokens) != 1 && tvc.RevokedTokens[0].Accessor != va.Accessor {
		t.Fatalf("Bad revoked accessors: %v", tvc.RevokedTokens)
	}
}

func TestLeader_ReplicateACLPolicies(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, func(c *Config) {
		c.Region = "region1"
		c.AuthoritativeRegion = "region1"
		c.ACLEnabled = true
	})
	defer s1.Shutdown()
	s2, _ := testACLServer(t, func(c *Config) {
		c.Region = "region2"
		c.AuthoritativeRegion = "region1"
		c.ACLEnabled = true
		c.ReplicationBackoff = 20 * time.Millisecond
		c.ReplicationToken = root.SecretID
	})
	defer s2.Shutdown()
	testJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

	// Write a policy to the authoritative region
	p1 := mock.ACLPolicy()
	if err := s1.State().UpsertACLPolicies(100, []*structs.ACLPolicy{p1}); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Wait for the policy to replicate
	testutil.WaitForResult(func() (bool, error) {
		state := s2.State()
		out, err := state.ACLPolicyByName(nil, p1.Name)
		return out != nil, err
	}, func(err error) {
		t.Fatalf("should replicate policy")
	})
}

func TestLeader_DiffACLPolicies(t *testing.T) {
	t.Parallel()

	state, err := state.NewStateStore(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Populate the local state
	p1 := mock.ACLPolicy()
	p2 := mock.ACLPolicy()
	p3 := mock.ACLPolicy()
	err = state.UpsertACLPolicies(100, []*structs.ACLPolicy{p1, p2, p3})
	assert.Nil(t, err)

	// Simulate a remote list
	p2Stub := p2.Stub()
	p2Stub.ModifyIndex = 50 // Ignored, same index
	p3Stub := p3.Stub()
	p3Stub.ModifyIndex = 100 // Updated, higher index
	p3Stub.Hash = []byte{0, 1, 2, 3}
	p4 := mock.ACLPolicy()
	remoteList := []*structs.ACLPolicyListStub{
		p2Stub,
		p3Stub,
		p4.Stub(),
	}
	delete, update := diffACLPolicies(state, 50, remoteList)

	// P1 does not exist on the remote side, should delete
	assert.Equal(t, []string{p1.Name}, delete)

	// P2 is un-modified - ignore. P3 modified, P4 new.
	assert.Equal(t, []string{p3.Name, p4.Name}, update)
}

func TestLeader_ReplicateACLTokens(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, func(c *Config) {
		c.Region = "region1"
		c.AuthoritativeRegion = "region1"
		c.ACLEnabled = true
	})
	defer s1.Shutdown()
	s2, _ := testACLServer(t, func(c *Config) {
		c.Region = "region2"
		c.AuthoritativeRegion = "region1"
		c.ACLEnabled = true
		c.ReplicationBackoff = 20 * time.Millisecond
		c.ReplicationToken = root.SecretID
	})
	defer s2.Shutdown()
	testJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

	// Write a token to the authoritative region
	p1 := mock.ACLToken()
	p1.Global = true
	if err := s1.State().UpsertACLTokens(100, []*structs.ACLToken{p1}); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Wait for the token to replicate
	testutil.WaitForResult(func() (bool, error) {
		state := s2.State()
		out, err := state.ACLTokenByAccessorID(nil, p1.AccessorID)
		return out != nil, err
	}, func(err error) {
		t.Fatalf("should replicate token")
	})
}

func TestLeader_DiffACLTokens(t *testing.T) {
	t.Parallel()

	state, err := state.NewStateStore(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Populate the local state
	p0 := mock.ACLToken()
	p1 := mock.ACLToken()
	p1.Global = true
	p2 := mock.ACLToken()
	p2.Global = true
	p3 := mock.ACLToken()
	p3.Global = true
	err = state.UpsertACLTokens(100, []*structs.ACLToken{p0, p1, p2, p3})
	assert.Nil(t, err)

	// Simulate a remote list
	p2Stub := p2.Stub()
	p2Stub.ModifyIndex = 50 // Ignored, same index
	p3Stub := p3.Stub()
	p3Stub.ModifyIndex = 100 // Updated, higher index
	p3Stub.Hash = []byte{0, 1, 2, 3}
	p4 := mock.ACLToken()
	p4.Global = true
	remoteList := []*structs.ACLTokenListStub{
		p2Stub,
		p3Stub,
		p4.Stub(),
	}
	delete, update := diffACLTokens(state, 50, remoteList)

	// P0 is local and should be ignored
	// P1 does not exist on the remote side, should delete
	assert.Equal(t, []string{p1.AccessorID}, delete)

	// P2 is un-modified - ignore. P3 modified, P4 new.
	assert.Equal(t, []string{p3.AccessorID, p4.AccessorID}, update)
}
