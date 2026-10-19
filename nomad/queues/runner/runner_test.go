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

		qr := New(testlog.HCLogger(t), q, "", broker, nil, nil, time.Hour)
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

		qr := New(testlog.HCLogger(t), q, "", broker, store, nil, time.Hour)
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

	qr := New(testlog.HCLogger(t), q, "", &mocks.MockBroker{}, store, nil, time.Hour)
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
		qr := New(testlog.HCLogger(t), q, "", &mocks.MockBroker{}, nil, canceler.cancel, time.Hour)

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
		qr := New(testlog.HCLogger(t), q, "", &mocks.MockBroker{}, nil, canceler.cancel, time.Hour)

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
	}, time.Hour)

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

		qr := New(testlog.HCLogger(t), nil, "", nil, ss, nil, time.Hour)

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
		qr := New(testlog.HCLogger(t), nil, "", broker, ss, nil, time.Hour)

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

		qr := New(testlog.HCLogger(t), nil, "", nil, ss, nil, time.Hour)

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
	qr := New(testlog.HCLogger(t), q, "", &mocks.MockBroker{}, nil, canceler.cancel, time.Hour)

	qr.Dequeue(wl.ID())
	qr.Dequeue(missing)

	must.Len(t, 1, canceler.canceled)
	must.Eq(t, canceler.canceled[0], wl.Eval())
	q.AssertExpectations(t)
}

func TestQueueRunner_emitMetrics(t *testing.T) {
	numJobs := 5
	pool := "testpool"

	sink := metrics.NewInmemSink(time.Millisecond*20, time.Millisecond*50)
	conf := metrics.DefaultConfig("nomad")
	conf.EnableHostname = false
	m, err := metrics.NewGlobal(conf, sink)
	must.NoError(t, err)
	t.Cleanup(m.Shutdown)

	// jobs in queue
	q := &mocks.MockQueue{Qlen: numJobs}
	q.On("Len").Return(numJobs)

	// time waiting on placement since job submitted
	job := mock.Job()
	eval := mock.Eval()
	eval.JobID = job.ID
	jobSubmitTimeAgo := time.Minute
	job.SubmitTime = time.Now().Add(-jobSubmitTimeAgo).UnixNano()
	wl := newWorkload(eval, job)

	ss := state.TestStateStore(t)
	must.NoError(t, ss.UpsertJob(structs.MsgTypeTestSetup, 1, nil, job))
	qr := New(testlog.HCLogger(t), q, pool, &mocks.MockBroker{}, ss, nilCanceler, time.Hour)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	go qr.waitForPlacement(ctx, wl)
	go qr.emitMetrics(ctx)

	var gauge metrics.GaugeValue
	var ok bool
	var sample metrics.SampledValue
	must.Wait(t, wait.InitialSuccess(
		wait.Timeout(time.Second),
		wait.Gap(20*time.Millisecond),
		wait.TestFunc(func() (bool, error) {
			datas := sink.Data()
			if len(datas) < 1 {
				return false, errors.New("missing data")
			}
			data := datas[0]
			gauge, ok = data.Gauges["nomad.queue.jobs;node_pool=testpool"]
			if !ok {
				return false, errors.New("missing Gauge")
			}
			sample, ok = data.Samples["nomad.queue.watcher.since_job_submitted;node_pool=testpool"]
			if !ok {
				return false, errors.New("missing MeasureSince sample")
			}
			return true, nil
		}),
	))

	// jobs in queue
	must.Eq(t, "nomad.queue.jobs", gauge.Name)
	must.Eq(t, float32(numJobs), gauge.Value)
	must.SliceLen(t, 1, gauge.Labels, must.Sprint("unexpected metric labels"))
	must.Eq(t, "node_pool", gauge.Labels[0].Name)
	must.Eq(t, "testpool", gauge.Labels[0].Value)

	// waiting for placement
	must.Eq(t, "nomad.queue.watcher.since_job_submitted", sample.Name)
	must.NotNil(t, sample.AggregateSample, must.Sprint("expect AggregateSample from MeasureSince"))
	must.Eq(t, 1, sample.AggregateSample.Count, must.Sprint("expect one AggregateSample"))
	must.Greater(t, jobSubmitTimeAgo.Seconds(), sample.AggregateSample.Min,
		must.Sprint("a minute should have elapsed since job SubmitTime"))
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

func nilCanceler(*structs.Evaluation) error {
	return nil
}
