// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-version"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

func TestLeader_LeftServer(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
	})
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
	})
	defer cleanupS3()
	servers := []*Server{s1, s2, s3}
	TestJoin(t, s1, s2, s3)

	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.numPeers()
			return peers == 3, nil
		}, func(err error) {
			t.Fatalf("should have 3 peers")
		})
	}

	// Kill any server
	var peer *Server
	for _, s := range servers {
		if !s.IsLeader() {
			peer = s
			break
		}
	}
	if peer == nil {
		t.Fatalf("Should have a non-leader")
	}
	peer.Shutdown()
	name := fmt.Sprintf("%s.%s", peer.config.NodeName, peer.config.Region)

	testutil.WaitForResult(func() (bool, error) {
		for _, s := range servers {
			if s == peer {
				continue
			}

			// Force remove the non-leader (transition to left state)
			if err := s.RemoveFailedNode(name); err != nil {
				return false, err
			}

			peers, _ := s.numPeers()
			return peers == 2, errors.New(fmt.Sprintf("%v", peers))
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestLeader_LeftLeader(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
	})
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
	})
	defer cleanupS3()
	servers := []*Server{s1, s2, s3}
	TestJoin(t, s1, s2, s3)

	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.numPeers()
			return peers == 3, nil
		}, func(err error) {
			t.Fatalf("should have 3 peers")
		})
	}

	// Kill the leader!
	leader := waitForStableLeadership(t, servers)

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
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, nil)
	defer cleanupS2()
	servers := []*Server{s1, s2}
	TestJoin(t, s1, s2)

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
		peers, err := s.numPeers()
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if peers != 1 {
			t.Fatalf("should only have 1 raft peer! %v", peers)
		}
	}
}

func TestLeader_PlanQueue_Reset(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
	})
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
	})
	defer cleanupS3()
	servers := []*Server{s1, s2, s3}
	TestJoin(t, s1, s2, s3)

	leader := waitForStableLeadership(t, servers)

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
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.BootstrapExpect = 3
	})
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.BootstrapExpect = 3
	})
	defer cleanupS3()
	servers := []*Server{s1, s2, s3}
	TestJoin(t, s1, s2, s3)

	leader := waitForStableLeadership(t, servers)

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
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.BootstrapExpect = 3
	})
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.BootstrapExpect = 3
	})
	defer cleanupS3()
	servers := []*Server{s1, s2, s3}
	TestJoin(t, s1, s2, s3)

	leader := waitForStableLeadership(t, servers)

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
		leader.periodicDispatcher.l.Lock()
		defer leader.periodicDispatcher.l.Unlock()
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
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS1()
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
	time.Sleep(5 * time.Second)

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

type mockJobEvalDispatcher struct {
	forceEvalCalled, children bool
	evalToReturn              *structs.Evaluation
	JobEvalDispatcher
}

func (mjed *mockJobEvalDispatcher) DispatchJob(_ *structs.Job) (*structs.Evaluation, error) {
	mjed.forceEvalCalled = true
	return mjed.evalToReturn, nil
}

func (mjed *mockJobEvalDispatcher) RunningChildren(_ *structs.Job) (bool, error) {
	return mjed.children, nil
}

func testPeriodicJob_OverlapEnabled(times ...time.Time) *structs.Job {
	job := testPeriodicJob(times...)
	job.Periodic.ProhibitOverlap = true
	return job
}

func TestLeader_PeriodicDispatcher_Restore_Evals(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS1()

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
	eval, err := s1.periodicDispatcher.createEval(job, past)
	must.NoError(t, err)

	md := &mockJobEvalDispatcher{
		children:          false,
		evalToReturn:      eval,
		JobEvalDispatcher: s1,
	}

	s1.periodicDispatcher.dispatcher = md

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

	must.True(t, md.forceEvalCalled, must.Sprint("failed to force job evaluation"))
}

func TestLeader_PeriodicDispatcher_No_Overlaps_No_Running_Job(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Inject a periodic job that triggered once in the past, should trigger now
	// and once in the future.
	now := time.Now()
	past := now.Add(-1 * time.Second)
	future := now.Add(10 * time.Second)

	job := testPeriodicJob_OverlapEnabled(past, now, future)
	req := structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	_, _, err := s1.raftApply(structs.JobRegisterRequestType, req)
	must.NoError(t, err)

	// Create an eval for the past launch.
	eval, err := s1.periodicDispatcher.createEval(job, past)
	must.NoError(t, err)

	md := &mockJobEvalDispatcher{
		children:     false,
		evalToReturn: eval,
	}

	s1.periodicDispatcher.dispatcher = md

	// Flush the periodic dispatcher, ensuring that no evals will be created.
	s1.periodicDispatcher.SetEnabled(false)

	// Sleep till after the job should have been launched.
	time.Sleep(3 * time.Second)

	// Restore the periodic dispatcher.
	s1.periodicDispatcher.SetEnabled(true)
	must.NoError(t, s1.restorePeriodicDispatcher())

	// Ensure the job is tracked.
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	must.MapContainsKey(t, s1.periodicDispatcher.tracked, tuple, must.Sprint("periodic job not restored"))

	// Check that an eval was made.
	ws := memdb.NewWatchSet()
	last, err := s1.fsm.State().PeriodicLaunchByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.NotNil(t, last)

	must.NotEq(t, last.Launch, past, must.Sprint("restorePeriodicDispatcher did not force launch"))

	must.True(t, md.forceEvalCalled, must.Sprint("failed to force job evaluation"))
}

func TestLeader_PeriodicDispatcher_No_Overlaps_Running_Job(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Inject a periodic job that triggered once in the past, should trigger now
	// and once in the future.
	now := time.Now()
	past := now.Add(-1 * time.Second)
	future := now.Add(10 * time.Second)

	job := testPeriodicJob_OverlapEnabled(past, now, future)
	req := structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Namespace: job.Namespace,
		},
	}
	_, _, err := s1.raftApply(structs.JobRegisterRequestType, req)
	must.NoError(t, err)

	// Create an eval for the past launch.
	eval, err := s1.periodicDispatcher.createEval(job, past)
	must.NoError(t, err)

	md := &mockJobEvalDispatcher{
		children:     true,
		evalToReturn: eval,
	}

	s1.periodicDispatcher.dispatcher = md

	// Flush the periodic dispatcher, ensuring that no evals will be created.
	s1.periodicDispatcher.SetEnabled(false)

	// Sleep till after the job should have been launched.
	time.Sleep(3 * time.Second)

	// Restore the periodic dispatcher.
	s1.periodicDispatcher.SetEnabled(true)
	must.NoError(t, s1.restorePeriodicDispatcher())

	// Ensure the job is tracked.
	tuple := structs.NamespacedID{
		ID:        job.ID,
		Namespace: job.Namespace,
	}
	must.MapContainsKey(t, s1.periodicDispatcher.tracked, tuple, must.Sprint("periodic job not restored"))

	must.False(t, md.forceEvalCalled, must.Sprint("evaluation forced with job already running"))
}

func TestLeader_PeriodicDispatch(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EvalGCInterval = 5 * time.Millisecond
	})
	defer cleanupS1()

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
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EvalDeliveryLimit = 1
	})
	defer cleanupS1()
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
		if out.NextEval == "" {
			return false, fmt.Errorf("got empty NextEval")
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

			if e.ID != out.NextEval {
				return false, fmt.Errorf("follow up eval id is %v; orig eval NextEval %v",
					e.ID, out.NextEval)
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
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a duplicate blocked eval
	eval := mock.Eval()
	eval.CreateIndex = 100
	eval2 := mock.Eval()
	eval2.JobID = eval.JobID
	eval2.CreateIndex = 102
	s1.blockedEvals.Block(eval)
	s1.blockedEvals.Block(eval2)

	// Wait for the evaluation to marked as cancelled
	state := s1.fsm.State()
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		out, err := state.EvalByID(ws, eval.ID)
		if err != nil {
			return false, err
		}
		return out != nil && out.Status == structs.EvalStatusCancelled, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestLeader_revokeVaultAccessorsOnRestore(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Insert a vault accessor that should be revoked
	fsmState := s1.fsm.State()
	va := mock.VaultAccessor()
	if err := fsmState.UpsertVaultAccessor(100, []*structs.VaultAccessor{va}); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Swap the Vault client
	tvc := &TestVaultClient{}
	s1.vault = tvc

	// Do a restore
	if err := s1.revokeVaultAccessorsOnRestore(); err != nil {
		t.Fatalf("Failed to restore: %v", err)
	}

	if len(tvc.RevokedTokens) != 1 && tvc.RevokedTokens[0].Accessor != va.Accessor {
		t.Fatalf("Bad revoked accessors: %v", tvc.RevokedTokens)
	}
}

func TestLeader_revokeSITokenAccessorsOnRestore(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// replace consul ACLs API with a mock for tracking calls in tests
	var consulACLsAPI mockConsulACLsAPI
	s1.consulACLs = &consulACLsAPI

	// Insert a SI token accessor that should be revoked
	fsmState := s1.fsm.State()
	accessor := mock.SITokenAccessor()
	err := fsmState.UpsertSITokenAccessors(100, []*structs.SITokenAccessor{accessor})
	r.NoError(err)

	// Do a restore
	err = s1.revokeSITokenAccessorsOnRestore()
	r.NoError(err)

	// Check the accessor was revoked
	exp := []revokeRequest{{
		accessorID: accessor.AccessorID,
		committed:  true,
	}}
	r.ElementsMatch(exp, consulACLsAPI.revokeRequests)
}

func TestLeader_ClusterID(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.Build = minClusterIDVersion.String()
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	clusterID, err := s1.ClusterID()

	require.NoError(t, err)
	require.True(t, helper.IsUUID(clusterID))
}

func TestLeader_ClusterID_upgradePath(t *testing.T) {
	ci.Parallel(t)

	before := version.Must(version.NewVersion("0.10.1")).String()
	after := minClusterIDVersion.String()

	type server struct {
		s       *Server
		cleanup func()
	}

	outdated := func() server {
		s, cleanup := TestServer(t, func(c *Config) {
			c.NumSchedulers = 0
			c.Build = before
			c.BootstrapExpect = 3
			c.Logger.SetLevel(hclog.Trace)
		})
		return server{s: s, cleanup: cleanup}
	}

	upgraded := func() server {
		s, cleanup := TestServer(t, func(c *Config) {
			c.NumSchedulers = 0
			c.Build = after
			c.BootstrapExpect = 0
			c.Logger.SetLevel(hclog.Trace)
		})
		return server{s: s, cleanup: cleanup}
	}

	servers := []server{outdated(), outdated(), outdated()}
	// fallback shutdown attempt in case testing fails
	defer servers[0].cleanup()
	defer servers[1].cleanup()
	defer servers[2].cleanup()

	upgrade := func(i int) {
		previous := servers[i]

		servers[i] = upgraded()
		TestJoin(t, servers[i].s, servers[(i+1)%3].s, servers[(i+2)%3].s)
		testutil.WaitForLeader(t, servers[i].s.RPC)

		require.NoError(t, previous.s.Leave())
		require.NoError(t, previous.s.Shutdown())
	}

	// Join the servers before doing anything
	TestJoin(t, servers[0].s, servers[1].s, servers[2].s)

	// Wait for servers to settle
	for i := 0; i < len(servers); i++ {
		testutil.WaitForLeader(t, servers[i].s.RPC)
	}

	// A check that ClusterID is not available yet
	noIDYet := func() {
		for _, s := range servers {
			_, err := s.s.ClusterID()
			must.Error(t, err)
		}
	}

	// Replace first old server with new server
	upgrade(0)
	defer servers[0].cleanup()
	noIDYet() // ClusterID should not work yet, servers: [new, old, old]

	// Replace second old server with new server
	upgrade(1)
	defer servers[1].cleanup()
	noIDYet() // ClusterID should not work yet, servers: [new, new, old]

	// Replace third / final old server with new server
	upgrade(2)
	defer servers[2].cleanup()

	// Wait for old servers to really be gone
	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.s.numPeers()
			return peers == 3, nil
		}, func(_ error) {
			t.Fatalf("should have 3 peers")
		})
	}

	// Now we can tickle the leader into making a cluster ID
	leaderID := ""
	for _, s := range servers {
		if s.s.IsLeader() {
			id, err := s.s.ClusterID()
			require.NoError(t, err)
			leaderID = id
			break
		}
	}
	require.True(t, helper.IsUUID(leaderID))

	// Now every participating server has been upgraded, each one should be
	// able to get the cluster ID, having been plumbed all the way through.
	agreeClusterID(t, []*Server{servers[0].s, servers[1].s, servers[2].s})
}

func TestLeader_ClusterID_noUpgrade(t *testing.T) {
	ci.Parallel(t)

	type server struct {
		s       *Server
		cleanup func()
	}

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.Logger.SetLevel(hclog.Trace)
		c.NumSchedulers = 0
		c.Build = minClusterIDVersion.String()
		c.BootstrapExpect = 3
	})
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.Logger.SetLevel(hclog.Trace)
		c.NumSchedulers = 0
		c.Build = minClusterIDVersion.String()
		c.BootstrapExpect = 3
	})
	defer cleanupS2()
	s3, cleanupS3 := TestServer(t, func(c *Config) {
		c.Logger.SetLevel(hclog.Trace)
		c.NumSchedulers = 0
		c.Build = minClusterIDVersion.String()
		c.BootstrapExpect = 3
	})
	defer cleanupS3()

	servers := []*Server{s1, s2, s3}

	// Join the servers before doing anything
	TestJoin(t, servers[0], servers[1], servers[2])

	// Wait for servers to settle
	for i := 0; i < len(servers); i++ {
		testutil.WaitForLeader(t, servers[i].RPC)
	}

	// Each server started at the minimum version, check there should be only 1
	// cluster ID they all agree on.
	agreeClusterID(t, []*Server{servers[0], servers[1], servers[2]})
}

func agreeClusterID(t *testing.T, servers []*Server) {
	must.Len(t, 3, servers)

	f := func() error {
		id1, err1 := servers[0].ClusterID()
		if err1 != nil {
			return err1
		}
		id2, err2 := servers[1].ClusterID()
		if err2 != nil {
			return err2
		}
		id3, err3 := servers[2].ClusterID()
		if err3 != nil {
			return err3
		}
		if id1 != id2 || id2 != id3 {
			return fmt.Errorf("ids do not match, id1: %s, id2: %s, id3: %s", id1, id2, id3)
		}
		return nil
	}

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(f),
		wait.Timeout(60*time.Second),
		wait.Gap(1*time.Second),
	))
}

func TestLeader_ReplicateACLPolicies(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.Region = "region1"
		c.AuthoritativeRegion = "region1"
		c.ACLEnabled = true
	})
	defer cleanupS1()
	s2, _, cleanupS2 := TestACLServer(t, func(c *Config) {
		c.Region = "region2"
		c.AuthoritativeRegion = "region1"
		c.ACLEnabled = true
		c.ReplicationBackoff = 20 * time.Millisecond
		c.ReplicationToken = root.SecretID
	})
	defer cleanupS2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

	// Write a policy to the authoritative region
	p1 := mock.ACLPolicy()
	if err := s1.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 100, []*structs.ACLPolicy{p1}); err != nil {
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
	ci.Parallel(t)

	state := state.TestStateStore(t)

	// Populate the local state
	p1 := mock.ACLPolicy()
	p2 := mock.ACLPolicy()
	p3 := mock.ACLPolicy()
	assert.Nil(t, state.UpsertACLPolicies(structs.MsgTypeTestSetup, 100, []*structs.ACLPolicy{p1, p2, p3}))

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
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.Region = "region1"
		c.AuthoritativeRegion = "region1"
		c.ACLEnabled = true
	})
	defer cleanupS1()
	s2, _, cleanupS2 := TestACLServer(t, func(c *Config) {
		c.Region = "region2"
		c.AuthoritativeRegion = "region1"
		c.ACLEnabled = true
		c.ReplicationBackoff = 20 * time.Millisecond
		c.ReplicationToken = root.SecretID
	})
	defer cleanupS2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

	// Write a token to the authoritative region
	p1 := mock.ACLToken()
	p1.Global = true
	if err := s1.State().UpsertACLTokens(structs.MsgTypeTestSetup, 100, []*structs.ACLToken{p1}); err != nil {
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
	ci.Parallel(t)

	state := state.TestStateStore(t)

	// Populate the local state
	p0 := mock.ACLToken()
	p1 := mock.ACLToken()
	p1.Global = true
	p2 := mock.ACLToken()
	p2.Global = true
	p3 := mock.ACLToken()
	p3.Global = true
	assert.Nil(t, state.UpsertACLTokens(structs.MsgTypeTestSetup, 100, []*structs.ACLToken{p0, p1, p2, p3}))

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

func TestServer_replicationBackoffContinue(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		testFn func()
	}{
		{
			name: "leadership lost",
			testFn: func() {

				// Create a test server with a long enough backoff that we will
				// be able to close the channel before it fires, but not too
				// long that the test having problems means CI will hang
				// forever.
				testServer, testServerCleanup := TestServer(t, func(c *Config) {
					c.ReplicationBackoff = 5 * time.Second
				})
				defer testServerCleanup()

				// Create our stop channel which is used by the server to
				// indicate leadership loss.
				stopCh := make(chan struct{})

				// The resultCh is used to block and collect the output from
				// the test routine.
				resultCh := make(chan bool, 1)

				// Run a routine to collect the result and close the channel
				// straight away.
				go func() {
					output := testServer.replicationBackoffContinue(stopCh)
					resultCh <- output
				}()

				close(stopCh)

				actualResult := <-resultCh
				require.False(t, actualResult)
			},
		},
		{
			name: "backoff continue",
			testFn: func() {

				// Create a test server with a short backoff.
				testServer, testServerCleanup := TestServer(t, func(c *Config) {
					c.ReplicationBackoff = 10 * time.Nanosecond
				})
				defer testServerCleanup()

				// Create our stop channel which is used by the server to
				// indicate leadership loss.
				stopCh := make(chan struct{})

				// The resultCh is used to block and collect the output from
				// the test routine.
				resultCh := make(chan bool, 1)

				// Run a routine to collect the result without closing stopCh.
				go func() {
					output := testServer.replicationBackoffContinue(stopCh)
					resultCh <- output
				}()

				actualResult := <-resultCh
				require.True(t, actualResult)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFn()
		})
	}
}

func Test_diffACLRoles(t *testing.T) {
	ci.Parallel(t)

	stateStore := state.TestStateStore(t)

	// Build an initial baseline of ACL Roles.
	aclRole0 := mock.ACLRole()
	aclRole1 := mock.ACLRole()
	aclRole2 := mock.ACLRole()
	aclRole3 := mock.ACLRole()

	// Upsert these into our local state. Use copies, so we can alter the roles
	// directly and use within the diff func.
	err := stateStore.UpsertACLRoles(structs.MsgTypeTestSetup, 50,
		[]*structs.ACLRole{aclRole0.Copy(), aclRole1.Copy(), aclRole2.Copy(), aclRole3.Copy()}, true)
	require.NoError(t, err)

	// Modify the ACL roles to create a number of differences. These roles
	// represent the state of the authoritative region.
	aclRole2.ModifyIndex = 50
	aclRole3.ModifyIndex = 200
	aclRole3.Hash = []byte{0, 1, 2, 3}
	aclRole4 := mock.ACLRole()

	// Run the diff function and test the output.
	toDelete, toUpdate := diffACLRoles(stateStore, 50, []*structs.ACLRoleListStub{
		aclRole2.Stub(), aclRole3.Stub(), aclRole4.Stub()})
	require.ElementsMatch(t, []string{aclRole0.ID, aclRole1.ID}, toDelete)
	require.ElementsMatch(t, []string{aclRole3.ID, aclRole4.ID}, toUpdate)
}

func Test_diffACLAuthMethods(t *testing.T) {
	ci.Parallel(t)

	stateStore := state.TestStateStore(t)

	// Build an initial baseline of ACL auth-methods.
	aclAuthMethod0 := mock.ACLOIDCAuthMethod()
	aclAuthMethod1 := mock.ACLOIDCAuthMethod()
	aclAuthMethod2 := mock.ACLOIDCAuthMethod()
	aclAuthMethod3 := mock.ACLOIDCAuthMethod()

	// Upsert these into our local state. Use copies, so we can alter the
	// auth-methods directly and use within the diff func.
	err := stateStore.UpsertACLAuthMethods(50,
		[]*structs.ACLAuthMethod{aclAuthMethod0.Copy(), aclAuthMethod1.Copy(),
			aclAuthMethod2.Copy(), aclAuthMethod3.Copy()})
	must.NoError(t, err)

	// Modify the ACL auth-methods to create a number of differences. These
	// methods represent the state of the authoritative region.
	aclAuthMethod2.ModifyIndex = 50
	aclAuthMethod3.ModifyIndex = 200
	aclAuthMethod3.Hash = []byte{0, 1, 2, 3}
	aclAuthMethod4 := mock.ACLOIDCAuthMethod()

	// Run the diff function and test the output.
	toDelete, toUpdate := diffACLAuthMethods(stateStore, 50, []*structs.ACLAuthMethodStub{
		aclAuthMethod2.Stub(), aclAuthMethod3.Stub(), aclAuthMethod4.Stub()})
	require.ElementsMatch(t, []string{aclAuthMethod0.Name, aclAuthMethod1.Name}, toDelete)
	require.ElementsMatch(t, []string{aclAuthMethod3.Name, aclAuthMethod4.Name}, toUpdate)
}

func Test_diffACLBindingRules(t *testing.T) {
	ci.Parallel(t)

	stateStore := state.TestStateStore(t)

	// Build an initial baseline of ACL binding rules.
	aclBindingRule0 := mock.ACLBindingRule()
	aclBindingRule1 := mock.ACLBindingRule()
	aclBindingRule2 := mock.ACLBindingRule()
	aclBindingRule3 := mock.ACLBindingRule()

	// Upsert these into our local state. Use copies, so we can alter the
	// binding rules directly and use within the diff func.
	err := stateStore.UpsertACLBindingRules(50,
		[]*structs.ACLBindingRule{aclBindingRule0.Copy(), aclBindingRule1.Copy(),
			aclBindingRule2.Copy(), aclBindingRule3.Copy()}, true)
	must.NoError(t, err)

	// Modify the ACL auth-methods to create a number of differences. These
	// methods represent the state of the authoritative region.
	aclBindingRule2.ModifyIndex = 50
	aclBindingRule3.ModifyIndex = 200
	aclBindingRule3.Hash = []byte{0, 1, 2, 3}
	aclBindingRule4 := mock.ACLBindingRule()

	// Run the diff function and test the output.
	toDelete, toUpdate := diffACLBindingRules(stateStore, 50, []*structs.ACLBindingRuleListStub{
		aclBindingRule2.Stub(), aclBindingRule3.Stub(), aclBindingRule4.Stub()})
	must.SliceContainsAll(t, []string{aclBindingRule0.ID, aclBindingRule1.ID}, toDelete)
	must.SliceContainsAll(t, []string{aclBindingRule3.ID, aclBindingRule4.ID}, toUpdate)
}

func TestLeader_Reelection(t *testing.T) {
	ci.Parallel(t)

	const raftProtocol = 3

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.RaftConfig.ProtocolVersion = raftProtocol
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.RaftConfig.ProtocolVersion = raftProtocol
	})
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.RaftConfig.ProtocolVersion = raftProtocol
	})
	defer cleanupS3() // todo(shoenig) added this, should be here right??

	servers := []*Server{s1, s2, s3}

	// Try to join
	TestJoin(t, s1, s2, s3)
	testutil.WaitForLeader(t, s1.RPC)

	testutil.WaitForResult(func() (bool, error) {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			return false, err
		}

		for _, server := range future.Configuration().Servers {
			if server.Suffrage == raft.Nonvoter {
				return false, fmt.Errorf("non-voter %v", server)
			}
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})

	var leader, nonLeader *Server
	for _, s := range servers {
		if s.IsLeader() {
			leader = s
		} else {
			nonLeader = s
		}
	}

	// make sure we still have a leader, then shut it down
	must.NotNil(t, leader, must.Sprint("expected there to be a leader"))
	leader.Shutdown()

	// Wait for new leader to elect
	testutil.WaitForLeader(t, nonLeader.RPC)
}

func TestLeader_RollRaftServer(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS2()

	s3, cleanupS3 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS3()

	servers := []*Server{s1, s2, s3}
	TestJoin(t, s1, s2, s3)

	t.Logf("waiting for initial stable cluster")
	waitForStableLeadership(t, servers)

	t.Logf("killing server s1")
	s1.Shutdown()
	for _, s := range []*Server{s2, s3} {
		s.RemoveFailedNode(s1.config.NodeID)
	}

	t.Logf("waiting for server loss to be detected")
	testutil.WaitForResultUntil(time.Second*10,
		func() (bool, error) {
			for _, s := range []*Server{s2, s3} {
				err := wantPeers(s, 2)
				if err != nil {
					return false, err
				}
			}
			return true, nil

		},
		func(err error) { must.NoError(t, err) },
	)

	t.Logf("adding replacement server s4")
	s4, cleanupS4 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS4()
	TestJoin(t, s2, s3, s4)
	servers = []*Server{s4, s2, s3}

	t.Logf("waiting for s4 to stabilize")
	waitForStableLeadership(t, servers)

	t.Logf("killing server s2")
	s2.Shutdown()
	for _, s := range []*Server{s3, s4} {
		s.RemoveFailedNode(s2.config.NodeID)
	}

	t.Logf("waiting for server loss to be detected")
	testutil.WaitForResultUntil(time.Second*10,
		func() (bool, error) {
			for _, s := range []*Server{s3, s4} {
				err := wantPeers(s, 2)
				if err != nil {
					return false, err
				}
			}
			return true, nil
		},
		func(err error) { must.NoError(t, err) },
	)

	t.Logf("adding replacement server s5")
	s5, cleanupS5 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS5()
	TestJoin(t, s3, s4, s5)
	servers = []*Server{s4, s5, s3}

	t.Logf("waiting for s5 to stabilize")
	waitForStableLeadership(t, servers)

	t.Logf("killing server s3")
	s3.Shutdown()

	t.Logf("waiting for server loss to be detected")
	testutil.WaitForResultUntil(time.Second*10,
		func() (bool, error) {
			for _, s := range []*Server{s4, s5} {
				err := wantPeers(s, 2)
				if err != nil {
					return false, err
				}
			}
			return true, nil
		},
		func(err error) { must.NoError(t, err) },
	)

	t.Logf("adding replacement server s6")
	s6, cleanupS6 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS6()
	TestJoin(t, s6, s4)
	servers = []*Server{s4, s5, s6}

	t.Logf("waiting for s6 to stabilize")
	waitForStableLeadership(t, servers)
}

func TestLeader_RevokeLeadership_MultipleTimes(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	testutil.WaitForResult(func() (bool, error) {
		return s1.evalBroker.Enabled(), nil
	}, func(err error) {
		t.Fatalf("should have finished establish leader loop")
	})

	require.Nil(t, s1.revokeLeadership())
	require.Nil(t, s1.revokeLeadership())
	require.Nil(t, s1.revokeLeadership())
}

func TestLeader_TransitionsUpdateConsistencyRead(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	testutil.WaitForResult(func() (bool, error) {
		return s1.isReadyForConsistentReads(), nil
	}, func(err error) {
		require.Fail(t, "should have finished establish leader loop")
	})

	require.Nil(t, s1.revokeLeadership())
	require.False(t, s1.isReadyForConsistentReads())

	ch := make(chan struct{})
	require.Nil(t, s1.establishLeadership(ch))
	require.True(t, s1.isReadyForConsistentReads())
}

// TestLeader_PausingWorkers asserts that scheduling workers are paused
// (and unpaused) upon leader elections (and step downs).
func TestLeader_PausingWorkers(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 12
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)
	require.Len(t, s1.workers, 12)

	// this satisfies the require.Eventually test interface
	checkPaused := func(count int) func() bool {
		return func() bool {
			pausedWorkers := func() int {
				c := 0
				for _, w := range s1.workers {
					if w.IsPaused() {
						c++
					}
				}
				return c
			}

			return pausedWorkers() == count
		}
	}

	// acquiring leadership should have paused 3/4 of the workers
	require.Eventually(t, checkPaused(9), 1*time.Second, 10*time.Millisecond, "scheduler workers did not pause within a second at leadership change")

	err := s1.revokeLeadership()
	require.NoError(t, err)

	// unpausing is a relatively quick activity
	require.Eventually(t, checkPaused(0), 50*time.Millisecond, 10*time.Millisecond, "scheduler workers should have unpaused after losing leadership")
}

// Test doing an inplace upgrade on a server from raft protocol 2 to 3
// This verifies that removing the server and adding it back with a uuid works
// even if the server's address stays the same.
func TestServer_ReconcileMember(t *testing.T) {
	ci.Parallel(t)

	// Create a three node cluster
	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS2()

	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)

	// test relies on s3 not being the leader, so adding it
	// after leadership has been established to reduce
	s3, cleanupS3 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 0
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS3()

	TestJoin(t, s1, s3)

	// Create a memberlist object for s3, with raft protocol upgraded to 3
	upgradedS3Member := serf.Member{
		Name:   s3.config.NodeName,
		Addr:   s3.config.RPCAddr.IP,
		Status: serf.StatusAlive,
		Tags:   make(map[string]string),
	}
	upgradedS3Member.Tags["role"] = "nomad"
	upgradedS3Member.Tags["id"] = s3.config.NodeID
	upgradedS3Member.Tags["region"] = s3.config.Region
	upgradedS3Member.Tags["dc"] = s3.config.Datacenter
	upgradedS3Member.Tags["rpc_addr"] = "127.0.0.1"
	upgradedS3Member.Tags["port"] = strconv.Itoa(s3.config.RPCAddr.Port)
	upgradedS3Member.Tags["build"] = "0.8.0"
	upgradedS3Member.Tags["vsn"] = "2"
	upgradedS3Member.Tags["mvn"] = "1"
	upgradedS3Member.Tags["raft_vsn"] = "3"

	findLeader := func(t *testing.T) *Server {
		t.Helper()
		for _, s := range []*Server{s1, s2, s3} {
			if s.IsLeader() {
				t.Logf("found leader: %v %v", s.config.NodeID, s.config.RPCAddr)
				return s
			}
		}

		t.Fatalf("no leader found")
		return nil
	}

	// Find the leader so that we can call reconcile member on it
	leader := findLeader(t)
	if err := leader.reconcileMember(upgradedS3Member); err != nil {
		t.Fatalf("failed to reconcile member: %v", err)
	}

	// This should remove s3 from the config and potentially cause a leader election
	testutil.WaitForLeader(t, s1.RPC)

	// Figure out the new leader and call reconcile again, this should add s3 with the new ID format
	leader = findLeader(t)
	if err := leader.reconcileMember(upgradedS3Member); err != nil {
		t.Fatalf("failed to reconcile member: %v", err)
	}

	testutil.WaitForLeader(t, s1.RPC)
	future := s2.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		t.Fatal(err)
	}
	addrs := 0
	ids := 0
	for _, server := range future.Configuration().Servers {
		if string(server.ID) == string(server.Address) {
			addrs++
		} else {
			ids++
		}
	}
	// After this, all three servers should have IDs in raft
	if got, want := addrs, 0; got != want {
		t.Fatalf("got %d server addresses want %d", got, want)
	}
	if got, want := ids, 3; got != want {
		t.Fatalf("got %d server ids want %d: %#v", got, want, future.Configuration().Servers)
	}
}

func TestLeader_ReplicateNamespaces(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.Region = "region1"
		c.AuthoritativeRegion = "region1"
		c.ACLEnabled = true
	})
	defer cleanupS1()
	s2, _, cleanupS2 := TestACLServer(t, func(c *Config) {
		c.Region = "region2"
		c.AuthoritativeRegion = "region1"
		c.ACLEnabled = true
		c.ReplicationBackoff = 20 * time.Millisecond
		c.ReplicationToken = root.SecretID
	})
	defer cleanupS2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

	// Write a namespace to the authoritative region
	ns1 := mock.Namespace()
	assert.Nil(s1.State().UpsertNamespaces(100, []*structs.Namespace{ns1}))

	// Wait for the namespace to replicate
	testutil.WaitForResult(func() (bool, error) {
		state := s2.State()
		out, err := state.NamespaceByName(nil, ns1.Name)
		return out != nil, err
	}, func(err error) {
		t.Fatalf("should replicate namespace")
	})

	// Delete the namespace at the authoritative region
	assert.Nil(s1.State().DeleteNamespaces(200, []string{ns1.Name}))

	// Wait for the namespace deletion to replicate
	testutil.WaitForResult(func() (bool, error) {
		state := s2.State()
		out, err := state.NamespaceByName(nil, ns1.Name)
		return out == nil, err
	}, func(err error) {
		t.Fatalf("should replicate namespace deletion")
	})
}

func TestLeader_DiffNamespaces(t *testing.T) {
	ci.Parallel(t)

	state := state.TestStateStore(t)

	// Populate the local state
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	ns3 := mock.Namespace()
	assert.Nil(t, state.UpsertNamespaces(100, []*structs.Namespace{ns1, ns2, ns3}))

	// Simulate a remote list
	rns2 := ns2.Copy()
	rns2.ModifyIndex = 50 // Ignored, same index
	rns3 := ns3.Copy()
	rns3.ModifyIndex = 100 // Updated, higher index
	rns3.Hash = []byte{0, 1, 2, 3}
	ns4 := mock.Namespace()
	remoteList := []*structs.Namespace{
		rns2,
		rns3,
		ns4,
	}
	delete, update := diffNamespaces(state, 50, remoteList)
	sort.Strings(delete)

	// ns1 does not exist on the remote side, should delete
	assert.Equal(t, []string{structs.DefaultNamespace, ns1.Name}, delete)

	// ns2 is un-modified - ignore. ns3 modified, ns4 new.
	assert.Equal(t, []string{ns3.Name, ns4.Name}, update)
}

func TestLeader_ReplicateNodePools(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.Region = "region1"
		c.AuthoritativeRegion = "region1"
		c.ACLEnabled = true
	})
	defer cleanupS1()
	s2, _, cleanupS2 := TestACLServer(t, func(c *Config) {
		c.Region = "region2"
		c.AuthoritativeRegion = "region1"
		c.ACLEnabled = true
		c.ReplicationBackoff = 20 * time.Millisecond
		c.ReplicationToken = root.SecretID
	})
	defer cleanupS2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

	// Write a node pool to the authoritative region
	np1 := mock.NodePool()
	must.NoError(t, s1.State().UpsertNodePools(
		structs.MsgTypeTestSetup, 100, []*structs.NodePool{np1}))

	// Wait for the node pool to replicate
	testutil.WaitForResult(func() (bool, error) {
		store := s2.State()
		out, err := store.NodePoolByName(nil, np1.Name)
		return out != nil, err
	}, func(err error) {
		t.Fatalf("should replicate node pool")
	})

	// Delete the node pool at the authoritative region
	must.NoError(t, s1.State().DeleteNodePools(structs.MsgTypeTestSetup, 200, []string{np1.Name}))

	// Wait for the namespace deletion to replicate
	testutil.WaitForResult(func() (bool, error) {
		store := s2.State()
		out, err := store.NodePoolByName(nil, np1.Name)
		return out == nil, err
	}, func(err error) {
		t.Fatalf("should replicate node pool deletion")
	})
}

func TestLeader_DiffNodePools(t *testing.T) {
	ci.Parallel(t)

	state := state.TestStateStore(t)

	// Populate the local state
	np1, np2, np3 := mock.NodePool(), mock.NodePool(), mock.NodePool()
	must.NoError(t, state.UpsertNodePools(
		structs.MsgTypeTestSetup, 100, []*structs.NodePool{np1, np2, np3}))

	// Simulate a remote list
	rnp2 := np2.Copy()
	rnp2.ModifyIndex = 50 // Ignored, same index
	rnp3 := np3.Copy()
	rnp3.ModifyIndex = 100 // Updated, higher index
	rnp3.Description = "force a hash update"
	rnp3.SetHash()
	rnp4 := mock.NodePool()
	remoteList := []*structs.NodePool{
		rnp2,
		rnp3,
		rnp4,
	}
	delete, update := diffNodePools(state, 50, remoteList)
	sort.Strings(delete)

	// np1 does not exist on the remote side, should delete
	test.Eq(t, []string{structs.NodePoolAll, structs.NodePoolDefault, np1.Name}, delete)

	// np2 is un-modified - ignore. np3 modified, np4 new.
	test.Eq(t, []*structs.NodePool{rnp3, rnp4}, update)
}

// waitForStableLeadership waits until a leader is elected and all servers
// get promoted as voting members, returns the leader
func waitForStableLeadership(t *testing.T, servers []*Server) *Server {
	nPeers := len(servers)

	// wait for all servers to discover each other
	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.numPeers()
			if peers != nPeers {
				return false, fmt.Errorf("should find %d peers but found %d", nPeers, peers)
			}

			return true, nil
		}, func(err error) {
			require.NoError(t, err)
		})
	}

	// wait for leader
	var leader *Server
	testutil.WaitForResult(func() (bool, error) {
		for _, s := range servers {
			if s.IsLeader() {
				leader = s
				return true, nil
			}
		}

		return false, fmt.Errorf("no leader found")
	}, func(err error) {
		require.NoError(t, err)
	})

	// wait for all servers get marked as voters
	testutil.WaitForResult(func() (bool, error) {
		future := leader.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			return false, fmt.Errorf("failed to get raft config: %v", future.Error())
		}
		ss := future.Configuration().Servers
		if len(ss) != len(servers) {
			return false, fmt.Errorf("raft doesn't contain all servers.  Expected %d but found %d", len(servers), len(ss))
		}

		for _, s := range ss {
			if s.Suffrage != raft.Voter {
				return false, fmt.Errorf("configuration has non voting server: %v", s)
			}
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	return leader
}

func TestServer_getLatestIndex(t *testing.T) {
	ci.Parallel(t)

	testServer, testServerCleanup := TestServer(t, nil)
	defer testServerCleanup()

	// Test a new state store value.
	idx, success := testServer.getLatestIndex()
	require.True(t, success)
	must.Eq(t, 1, idx)

	// Upsert something with a high index, and check again.
	err := testServer.State().UpsertACLPolicies(
		structs.MsgTypeTestSetup, 1013, []*structs.ACLPolicy{mock.ACLPolicy()})
	require.NoError(t, err)

	idx, success = testServer.getLatestIndex()
	require.True(t, success)
	must.Eq(t, 1013, idx)
}

func TestServer_handleEvalBrokerStateChange(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		startValue                  bool
		testServerCallBackConfig    func(c *Config)
		inputSchedulerConfiguration *structs.SchedulerConfiguration
		expectedOutput              bool
		name                        string
	}{
		{
			startValue:                  false,
			testServerCallBackConfig:    func(c *Config) { c.DefaultSchedulerConfig.PauseEvalBroker = false },
			inputSchedulerConfiguration: nil,
			expectedOutput:              true,
			name:                        "bootstrap un-paused",
		},
		{
			startValue:                  false,
			testServerCallBackConfig:    func(c *Config) { c.DefaultSchedulerConfig.PauseEvalBroker = true },
			inputSchedulerConfiguration: nil,
			expectedOutput:              false,
			name:                        "bootstrap paused",
		},
		{
			startValue:                  true,
			testServerCallBackConfig:    nil,
			inputSchedulerConfiguration: &structs.SchedulerConfiguration{PauseEvalBroker: true},
			expectedOutput:              false,
			name:                        "state change to paused",
		},
		{
			startValue:                  false,
			testServerCallBackConfig:    nil,
			inputSchedulerConfiguration: &structs.SchedulerConfiguration{PauseEvalBroker: true},
			expectedOutput:              false,
			name:                        "no state change to paused",
		},
		{
			startValue:                  false,
			testServerCallBackConfig:    nil,
			inputSchedulerConfiguration: &structs.SchedulerConfiguration{PauseEvalBroker: false},
			expectedOutput:              true,
			name:                        "state change to un-paused",
		},
		{
			startValue:                  false,
			testServerCallBackConfig:    nil,
			inputSchedulerConfiguration: &structs.SchedulerConfiguration{PauseEvalBroker: true},
			expectedOutput:              false,
			name:                        "no state change to un-paused",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Create a new server and wait for leadership to be established.
			testServer, cleanupFn := TestServer(t, nil)
			_ = waitForStableLeadership(t, []*Server{testServer})
			defer cleanupFn()

			// If we set a callback config, we are just testing the eventual
			// state of the brokers. Otherwise, we set our starting value and
			// then perform our state modification change and check.
			if tc.testServerCallBackConfig == nil {
				testServer.evalBroker.SetEnabled(tc.startValue)
				testServer.blockedEvals.SetEnabled(tc.startValue)
				actualOutput := testServer.handleEvalBrokerStateChange(tc.inputSchedulerConfiguration)
				require.Equal(t, tc.expectedOutput, actualOutput)
			}

			// Check the brokers are in the expected state.
			var expectedEnabledVal bool

			if tc.inputSchedulerConfiguration == nil {
				expectedEnabledVal = !testServer.config.DefaultSchedulerConfig.PauseEvalBroker
			} else {
				expectedEnabledVal = !tc.inputSchedulerConfiguration.PauseEvalBroker
			}
			require.Equal(t, expectedEnabledVal, testServer.evalBroker.Enabled())
			require.Equal(t, expectedEnabledVal, testServer.blockedEvals.Enabled())
		})
	}
}
