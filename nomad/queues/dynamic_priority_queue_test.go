// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func TestWaitForPlacement(t *testing.T) {

	t.Run("returns if eval complete", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))
		testQueue.SetEnabled(true, ss)

		testEval := mock.Eval()
		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval})

		ws := memdb.NewWatchSet()
		doneCh := make(chan error)
		go func() {
			err := testQueue.waitForPlacement(t.Context(), testEval, ws)
			doneCh <- err
		}()

		testEval.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		done := <-doneCh

		must.NoError(t, done)
	})

	t.Run("continues watching blocked evals", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))
		testQueue.SetEnabled(true, ss)

		testEval := mock.Eval()
		blocked := mock.Eval()

		testEval.Status = structs.EvalStatusComplete
		testEval.BlockedEval = blocked.ID

		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval, blocked})

		ws := memdb.NewWatchSet()
		doneCh := make(chan error)
		go func() {
			err := testQueue.waitForPlacement(t.Context(), testEval, ws)
			doneCh <- err
		}()

		// We want to make sure the testQueue has begun a watch on the blocked eval
		// before continuing, which is indicated by the length of the watchset being >0.
		must.Wait(t, wait.InitialSuccess(
			wait.ErrorFunc(func() error {
				if len(ws) == 0 {
					return fmt.Errorf("blocking query not started yet")
				}
				return nil
			}),
			wait.Timeout(5*time.Second),
			wait.Gap(100*time.Millisecond),
		))

		select {
		case <-doneCh:
			t.Fatal("should not have exited")
		default:
		}

		blocked.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{blocked})

		done := <-doneCh
		must.NoError(t, done)
	})

	t.Run("continues watching next evals after eval failure", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewDynamicPriorityQueue(nil, &structs.BatchQueue{}, &structs.DynamicQueueConfig{}, hclog.New(hclog.DefaultOptions))
		testQueue.SetEnabled(true, ss)

		testEval := mock.Eval()
		next := mock.Eval()

		testEval.Status = structs.EvalStatusFailed
		testEval.NextEval = next.ID

		ss.UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{testEval, next})

		ws := memdb.NewWatchSet()
		doneCh := make(chan error)
		go func() {
			err := testQueue.waitForPlacement(t.Context(), testEval, ws)
			doneCh <- err
		}()

		// We want to make sure the testQueue has begun a watch on the blocked eval
		// before continuing, which is indicated by the length of the watchset being >0.
		must.Wait(t, wait.InitialSuccess(
			wait.ErrorFunc(func() error {
				if len(ws) == 0 {
					return fmt.Errorf("blocking query not started yet")
				}
				return nil
			}),
			wait.Timeout(5*time.Second),
			wait.Gap(100*time.Millisecond),
		))

		select {
		case <-doneCh:
			t.Fatal("should not have exited")
		default:
		}

		next.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{next})

		done := <-doneCh
		must.NoError(t, done)
	})
}
