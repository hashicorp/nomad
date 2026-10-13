// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/go-metrics"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/queues/mocks"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
	tmock "github.com/stretchr/testify/mock"
)

func TestQueueRunner_Enqueue(t *testing.T) {
	t.Run("rejected workloads are enqueued on eval broker", func(t *testing.T) {
		eval := mock.Eval()
		job := mock.BatchJob()

		q := &mocks.MockQueue{}
		q.On("NewWorkload", eval, job).Return(nil, false).Once()

		broker := &mocks.MockBroker{}
		broker.On("Enqueue", eval).Once()

		qr := New(testlog.HCLogger(t), q, "", broker, nil, nil)
		qr.Enqueue(eval, job)

		q.AssertExpectations(t)
		broker.AssertExpectations(t)
	})

	t.Run("accepted workloads are processed correctly", func(t *testing.T) {
		store := state.TestStateStore(t)
		eval, job := pendingEvalWithJob(t, store)
		wl := newWorkload(eval, job)

		q := &mocks.MockQueue{}
		q.On("NewWorkload", eval, job).Return(wl, true).Once()
		q.On("Get", wl.ID()).Return(nil, false).Once()
		q.On("Push", wl).Once()
		q.On("Pop").Return(wl, true).Once()
		q.On("Pop").Return(nil, false).Once()

		enqueued := make(chan struct{})
		broker := &mocks.MockBroker{}
		broker.On("Enqueue", eval).Run(func(args tmock.Arguments) { close(enqueued) })

		qr := New(testlog.HCLogger(t), q, "", broker, store, nil)
		qr.Start(t.Context())
		qr.Enqueue(eval, job)

		must.Wait(t, wait.InitialSuccess(wait.BoolFunc(func() bool {
			select {
			case <-enqueued:
				return true
			default:
				return false
			}
		}), wait.Gap(50*time.Millisecond)))
		qr.Stop()

		q.AssertExpectations(t)
		broker.AssertExpectations(t)
	})
}

func TestQueueRunner_Restore(t *testing.T) {
	store := state.TestStateStore(t)

	job := mock.BatchJob()
	blockedEval := mock.Eval()
	blockedEval.JobID = job.ID
	blockedEval.Status = structs.EvalStatusBlocked

	completeEval := mock.Eval()
	completeEval.Status = structs.EvalStatusComplete
	evals := []*structs.Evaluation{blockedEval, completeEval}

	must.NoError(t, store.UpsertEvals(structs.MsgTypeTestSetup, 100, evals))

	q := &mocks.MockQueue{}
	q.On("NewWorkload", blockedEval, job).Return(newWorkload(blockedEval, job), true).Once()

	qr := New(testlog.HCLogger(t), q, "", &mocks.MockBroker{}, store, nil)
	must.NoError(t, qr.Restore(blockedEval, job))

	must.Eq(t, 1, len(qr.restored))
	q.AssertExpectations(t)
}

// A newer version of a queued job replaces the queued workload and cancels
// its eval.
func TestQueueRunner_CancelRedundant(t *testing.T) {
	t.Run("newer version cancels existing eval", func(t *testing.T) {
		job := mock.BatchJob()
		queued := newWorkload(mock.Eval(), job)

		newJob := job.Copy()
		newJob.Version++
		incoming := newWorkload(mock.Eval(), newJob)

		q := &mocks.MockQueue{}
		q.On("Get", incoming.ID()).Return(queued, true).Once()
		q.On("Swap", incoming).Return(queued, true).Once()

		canceler := &evalCanceler{}
		qr := New(testlog.HCLogger(t), q, "", &mocks.MockBroker{}, nil, canceler.cancel)

		must.True(t, qr.cancelRedundant(incoming))
		must.Eq(t, []*structs.Evaluation{queued.Eval()}, canceler.canceled)
		q.AssertExpectations(t)
	})

	t.Run("same version cancels new eval", func(t *testing.T) {
		job := mock.BatchJob()
		queued := newWorkload(mock.Eval(), job)
		incoming := newWorkload(mock.Eval(), job)

		q := &mocks.MockQueue{}
		q.On("Get", incoming.ID()).Return(queued, true).Once()

		canceler := &evalCanceler{}
		qr := New(testlog.HCLogger(t), q, "", &mocks.MockBroker{}, nil, canceler.cancel)

		must.True(t, qr.cancelRedundant(incoming))
		must.Eq(t, []*structs.Evaluation{incoming.Eval()}, canceler.canceled)
		q.AssertExpectations(t)
	})
}

func TestQueueRunner_cancelEval(t *testing.T) {
	e := mock.Eval()
	mockBroker := &mocks.MockBroker{}
	mockBroker.On("Enqueue", e).Return().Once()

	qr := New(testlog.HCLogger(t), nil, "", mockBroker, nil, func(e *structs.Evaluation) error {
		return errors.New("bad raft")
	})

	qr.cancelEval(e)

	mockBroker.AssertExpectations(t)
}

func TestQueueRunner_SkipQueue(t *testing.T) {
	t.Run("no alloc for job yet", func(t *testing.T) {
		ss := state.TestStateStore(t)

		job := mock.Job()
		job.Type = structs.JobTypeBatch
		eval := mock.Eval()
		eval.JobID = job.ID

		must.NoError(t, ss.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job))

		qr := New(testlog.HCLogger(t), nil, "", nil, ss, nil)

		wl := newWorkload(eval, job)
		must.False(t, qr.skipQueue(wl))
	})

	t.Run("alloc exists in state", func(t *testing.T) {
		ss := state.TestStateStore(t)

		job := mock.Job()
		job.Type = structs.JobTypeBatch
		eval := mock.Eval()
		eval.JobID = job.ID

		must.NoError(t, ss.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job))

		alloc := mock.MinAllocForJob(job)
		must.NoError(t, ss.UpsertAllocs(structs.MsgTypeTestSetup, 101, []*structs.Allocation{alloc}))
		broker := &mocks.MockBroker{}
		broker.On("Enqueue", eval).Return().Once()
		qr := New(testlog.HCLogger(t), nil, "", broker, ss, nil)

		wl := newWorkload(eval, job)
		must.True(t, qr.skipQueue(wl))

		broker.AssertExpectations(t)
	})

	t.Run("job is already being watched", func(t *testing.T) {
		ss := state.TestStateStore(t)

		job := mock.Job()
		job.Type = structs.JobTypeBatch
		eval := mock.Eval()
		eval.JobID = job.ID

		must.NoError(t, ss.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job))

		qr := New(testlog.HCLogger(t), nil, "", nil, ss, nil)

		wl := newWorkload(eval, job)
		qr.watcher.TrackPlacement(wl)
		must.True(t, qr.skipQueue(wl))
	})
}

// Dequeue removes a job's workload from the queue and returns its eval.
func TestQueueRunner_Dequeue(t *testing.T) {
	wl := newWorkload(mock.Eval(), mock.BatchJob())
	missing := structs.NewNamespacedID("missing", structs.DefaultNamespace)

	q := &mocks.MockQueue{}
	q.On("Remove", wl.ID()).Return(wl, true).Once()
	q.On("Remove", missing).Return(nil, false).Once()

	canceler := &evalCanceler{}
	qr := New(testlog.HCLogger(t), q, "", &mocks.MockBroker{}, nil, canceler.cancel)

	qr.Dequeue(wl.ID())
	qr.Dequeue(missing)

	must.Len(t, 1, canceler.canceled)
	must.Eq(t, canceler.canceled[0], wl.Eval())
	q.AssertExpectations(t)
}

func TestQueueRunner_Metrics(t *testing.T) {
	numJobs := 5
	pool := "testpool"

	sink := metrics.NewInmemSink(time.Millisecond*10, time.Millisecond*50)
	conf := metrics.DefaultConfig("metrics_svc")
	conf.EnableHostname = false
	m, err := metrics.NewGlobal(conf, sink)
	must.NoError(t, err)
	t.Cleanup(m.Shutdown)

	q := &mocks.MockQueue{
		Qlen: numJobs,
	}
	q.On("Len").Return(numJobs)

	canceler := &evalCanceler{}
	qr := New(testlog.HCLogger(t), q, pool, &mocks.MockBroker{}, nil, canceler.cancel)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	// this starts the emitMetrics goroutine
	qr.Start(ctx)
	time.Sleep(time.Millisecond * 20)

	data := sink.Data()
	must.Greater(t, 0, len(data), must.Sprint("no data in metrics sink"))
	info := data[0]

	must.MapLen(t, 1, info.Gauges)
	for _, val := range info.Gauges {
		must.Eq(t, "metrics_svc.nomad.queue.jobs", val.Name)
		must.Eq(t, float32(numJobs), val.Value)
		must.SliceLen(t, 1, val.Labels, must.Sprint("unexpected metric labels"))
		must.Eq(t, "node_pool", val.Labels[0].Name)
		must.Eq(t, "testpool", val.Labels[0].Value)
	}
}

// pendingEvalWithJob returns a pending eval for a new batch job, and stores the eval
// so the runner can watch it for placement.
func pendingEvalWithJob(t *testing.T, store *state.StateStore) (*structs.Evaluation, *structs.Job) {
	t.Helper()
	job := mock.BatchJob()
	eval := mock.Eval()
	eval.JobID = job.ID
	must.NoError(t, store.UpsertEvals(structs.MsgTypeTestSetup, 100, []*structs.Evaluation{eval}))
	return eval, job
}

func newWorkload(e *structs.Evaluation, j *structs.Job) queue.Workload {
	wl := queue.NewBaseWorkload(e, j, "")
	return &wl
}

// evalCanceler records the evals canceled by the runner.
type evalCanceler struct {
	canceled []*structs.Evaluation
}

func (c *evalCanceler) cancel(e *structs.Evaluation) error {
	c.canceled = append(c.canceled, e)
	return nil
}
