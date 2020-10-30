package nomad

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

var _ SinkDelegate = &Server{}

type testDelegate struct{ s *state.StateStore }

func (t *testDelegate) State() *state.StateStore { return t.s }
func (t *testDelegate) getLeaderAcl() string     { return "" }
func (t *testDelegate) Region() string           { return "" }

func (t *testDelegate) RPC(method string, args interface{}, reply interface{}) error {
	return nil
}

func newTestDelegate(t *testing.T) *testDelegate {
	return &testDelegate{
		s: state.TestStateStoreCfg(t, state.TestStateStorePublisher(t)),
	}
}

func TestManager_Run(t *testing.T) {
	t.Parallel()

	// Start a test server and count the number of requests
	receivedCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var event structs.Events
		dec := json.NewDecoder(r.Body)
		require.NoError(t, dec.Decode(&event))
		require.Equal(t, "Deployment", string(event.Events[0].Topic))

		receivedCount++
	}))
	defer ts.Close()

	// Store two sinks
	s1 := mock.EventSink()
	s1.Address = ts.URL
	s2 := mock.EventSink()
	s2.Address = ts.URL

	td := newTestDelegate(t)

	require.NoError(t, td.State().UpsertEventSink(1000, s1))
	require.NoError(t, td.State().UpsertEventSink(1001, s2))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create the manager
	manager := NewSinkManager(ctx, td, hclog.Default())
	require.NoError(t, manager.establishManagedSinks())

	require.Len(t, manager.sinkSubscriptions, 2)

	// Run the manager in the background
	runErr := make(chan error)
	go func() {
		err := manager.Run()
		runErr <- err
	}()

	// Publish an event
	broker, err := td.State().EventBroker()
	require.NoError(t, err)

	broker.Publish(&structs.Events{Index: 1, Events: []structs.Event{{Topic: "Deployment"}}})

	testutil.WaitForResult(func() (bool, error) {
		return receivedCount == 2, fmt.Errorf("webhook count not equal to expected want %d, got %d", 2, receivedCount)
	}, func(err error) {
		require.Fail(t, err.Error())
	})

	// Stop the manager
	cancel()

	select {
	case err := <-runErr:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timeout waiting for manager to stop")
	}
}

func TestManager_SinkErr(t *testing.T) {
	t.Parallel()

	receivedCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var event structs.Events
		dec := json.NewDecoder(r.Body)
		require.NoError(t, dec.Decode(&event))
		require.Equal(t, "Deployment", string(event.Events[0].Topic))

		receivedCount++
	}))
	defer ts.Close()

	s1 := mock.EventSink()
	s1.Address = ts.URL
	s2 := mock.EventSink()
	s2.Address = ts.URL

	td := newTestDelegate(t)

	require.NoError(t, td.State().UpsertEventSink(1000, s1))
	require.NoError(t, td.State().UpsertEventSink(1001, s2))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := NewSinkManager(ctx, td, hclog.Default())
	require.NoError(t, manager.establishManagedSinks())

	require.Len(t, manager.sinkSubscriptions, 2)

	runErr := make(chan error)
	go func() {
		err := manager.Run()
		runErr <- err
	}()

	// Publish an event
	broker, err := td.State().EventBroker()
	require.NoError(t, err)

	broker.Publish(&structs.Events{Index: 1, Events: []structs.Event{{Topic: "Deployment"}}})

	testutil.WaitForResult(func() (bool, error) {
		return receivedCount == 2, fmt.Errorf("webhook count not equal to expected want %d, got %d", 2, receivedCount)
	}, func(err error) {
		require.Fail(t, err.Error())
	})

	require.NoError(t, td.State().DeleteEventSinks(2000, []string{s1.ID}))

	// Wait for the manager to drop the old managed sink
	testutil.WaitForResult(func() (bool, error) {
		if len(manager.sinkSubscriptions) != 1 {
			return false, fmt.Errorf("expected manager to have 1 managed sink")
		}
		return true, nil
	}, func(err error) {
		require.Fail(t, err.Error())
	})

	// Stop the manager
	cancel()
	select {
	case err := <-runErr:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timeout waiting for manager to stop")
	}
}

// TestManager_Run_AddNew asserts that adding a new managed sink to the state
// store notifies the manager and starts the sink while leaving the existing
// managed sinks running
func TestManager_Run_AddNew(t *testing.T) {
	t.Parallel()

	received := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var event structs.Events
		dec := json.NewDecoder(r.Body)
		require.NoError(t, dec.Decode(&event))
		require.Equal(t, "Deployment", string(event.Events[0].Topic))

		close(received)
	}))
	defer ts.Close()

	received2 := make(chan struct{})
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var event structs.Events
		dec := json.NewDecoder(r.Body)
		require.NoError(t, dec.Decode(&event))
		require.Equal(t, "Deployment", string(event.Events[0].Topic))

		close(received2)
	}))
	defer ts2.Close()

	s1 := mock.EventSink()
	s1.Address = ts.URL

	td := newTestDelegate(t)

	require.NoError(t, td.State().UpsertEventSink(1000, s1))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := NewSinkManager(ctx, td, hclog.Default())
	require.NoError(t, manager.establishManagedSinks())

	require.Len(t, manager.sinkSubscriptions, 1)

	runErr := make(chan error)
	go func() {
		err := manager.Run()
		runErr <- err
	}()

	// Publish an event
	broker, err := td.State().EventBroker()
	require.NoError(t, err)

	broker.Publish(&structs.Events{Index: 1, Events: []structs.Event{{Topic: "Deployment"}}})

	select {
	case <-received:
		// pass
	case <-time.After(2 * time.Second):
		require.Fail(t, "timeout waiting for first sink to send event")
	}

	s2 := mock.EventSink()
	s2.Address = ts2.URL
	require.NoError(t, td.State().UpsertEventSink(1001, s2))

	select {
	case <-received2:
		// pass
	case <-time.After(2 * time.Second):
		require.Fail(t, "timeout waiting for first sink to send event")
	}

	// stop
	cancel()

	select {
	case err := <-runErr:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timeout waiting for manager to stop")
	}
}

func TestManagedSink_Run_Webhook(t *testing.T) {
	t.Parallel()

	// Setup webhook destination
	received := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var event structs.Events
		dec := json.NewDecoder(r.Body)
		require.NoError(t, dec.Decode(&event))
		require.Equal(t, "Deployment", string(event.Events[0].Topic))

		close(received)
	}))
	defer ts.Close()

	td := newTestDelegate(t)

	s1 := mock.EventSink()
	s1.Address = ts.URL
	require.NoError(t, td.State().UpsertEventSink(1000, s1))

	ws := memdb.NewWatchSet()
	_, err := td.State().EventSinkByID(ws, s1.ID)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create sink
	mSink, err := NewManagedSink(ctx, s1.ID, td.State, hclog.NewNullLogger())
	require.NoError(t, err)

	// Run in background
	go func() {
		mSink.run()
	}()

	// Publish an event
	broker, err := td.State().EventBroker()
	require.NoError(t, err)

	broker.Publish(&structs.Events{Index: 1, Events: []structs.Event{{Topic: "Deployment"}}})

	// Ensure the webhook destination receives event
	select {
	case <-received:
		// pass
	case <-time.After(2 * time.Second):
		require.Fail(t, "timeout waiting for webhook received")
	}
}

// TestManagedSink_Run_Webhook_Update tests the behavior when updating a
// managed sink's EventSink address to different values
func TestManagedSink_Run_Webhook_Update(t *testing.T) {
	t.Parallel()

	// Setup webhook destination
	received1 := make(chan int, 3)
	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var event structs.Events
		dec := json.NewDecoder(r.Body)
		require.NoError(t, dec.Decode(&event))
		require.Equal(t, "Deployment", string(event.Events[0].Topic))

		received1 <- int(event.Index)
	}))
	defer ts1.Close()

	// Setup a second webhook destination
	received2 := make(chan int, 3)
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var event structs.Events
		dec := json.NewDecoder(r.Body)
		require.NoError(t, dec.Decode(&event))
		require.Equal(t, "Deployment", string(event.Events[0].Topic))

		received2 <- int(event.Index)
	}))
	defer ts2.Close()

	td := newTestDelegate(t)

	// Create and store a sink
	s1 := mock.EventSink()
	s1.ID = "sink1"
	s1.Address = ts1.URL
	require.NoError(t, td.State().UpsertEventSink(1000, s1))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the managed sink
	mSink, err := NewManagedSink(ctx, s1.ID, td.State, hclog.Default())
	require.NoError(t, err)

	errCh := make(chan error)
	go func() {
		err := mSink.run()
		errCh <- err
	}()

	// Publish and event
	broker, err := td.State().EventBroker()
	require.NoError(t, err)

	broker.Publish(&structs.Events{Index: 1, Events: []structs.Event{{Topic: "Deployment"}}})

	// Ensure webhook received the event
	select {
	case got := <-received1:
		require.Equal(t, 1, got)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timeout waiting for webhook received")
	}

	// Wait and check that the sink reported the succesfully sent index
	testutil.WaitForResult(func() (bool, error) {
		ls := mSink.GetLastSuccess()
		return int(ls) == 1, fmt.Errorf("expected last success to update")
	}, func(err error) {
		require.NoError(t, err)
	})

	// Update sink to point to new address
	updateSink := mock.EventSink()
	updateSink.ID = s1.ID
	updateSink.Address = ts2.URL

	require.NoError(t, td.State().UpsertEventSink(1001, updateSink))

	// Wait for the address to propogate
	testutil.WaitForResult(func() (bool, error) {
		return mSink.Sink.Address == updateSink.Address, fmt.Errorf("expected managed sink address to update")
	}, func(err error) {
		require.Fail(t, err.Error())
	})

	// Publish a new event
	broker.Publish(&structs.Events{Index: 2, Events: []structs.Event{{Topic: "Deployment"}}})

	// Check we got it on the webhook side
	select {
	case got := <-received2:
		require.Equal(t, 1, got)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timeout waiting for webhook received")
	}

	// Wait and check that the sink reported the successfully sent index
	testutil.WaitForResult(func() (bool, error) {
		ls := mSink.GetLastSuccess()
		if int(ls) != 2 {
			return false, fmt.Errorf("expected last success to update")
		}
		return true, nil
	}, func(error) {
		t.Fatalf("expected sink progress")
	})

	// Point back to original
	updateSink = mock.EventSink()
	updateSink.ID = s1.ID
	updateSink.Address = ts1.URL
	require.NoError(t, td.State().UpsertEventSink(1002, updateSink))

	// Wait for the address to propogate
	testutil.WaitForResult(func() (bool, error) {
		return mSink.Sink.Address == s1.Address, fmt.Errorf("expected managed sink address to update")
	}, func(err error) {
		require.FailNow(t, err.Error())
	})

	broker.Publish(&structs.Events{Index: 3, Events: []structs.Event{{Topic: "Deployment"}}})

	select {
	case got := <-received1:
		got2 := <-received1
		require.Equal(t, 2, got)
		require.Equal(t, 3, got2)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timeout waiting for webhook received")
	}

	// Stop
	cancel()

	select {
	case err := <-errCh:
		require.Error(t, err)
		require.Equal(t, context.Canceled, err)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timeout waiting for shutdown")
	}
}

func TestManagedSink_Shutdown(t *testing.T) {
	t.Parallel()

	td := newTestDelegate(t)

	s1 := mock.EventSink()
	require.NoError(t, td.State().UpsertEventSink(1000, s1))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create sink
	mSink, err := NewManagedSink(ctx, s1.ID, td.State, hclog.NewNullLogger())
	require.NoError(t, err)

	// Run in background
	closed := make(chan struct{})
	go func() {
		err := mSink.run()
		require.Error(t, err)
		require.Equal(t, context.Canceled, err)
		close(closed)
	}()

	// Stop the parent context
	cancel()

	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		require.Fail(t, "expected managed sink to stop")
	}
}

func TestManagedSink_DeregisterSink(t *testing.T) {
	t.Parallel()

	td := newTestDelegate(t)

	s1 := mock.EventSink()
	require.NoError(t, td.State().UpsertEventSink(1000, s1))

	ws := memdb.NewWatchSet()
	_, err := td.State().EventSinkByID(ws, s1.ID)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create sink
	mSink, err := NewManagedSink(ctx, s1.ID, td.State, hclog.NewNullLogger())
	require.NoError(t, err)

	// Run in background
	closed := make(chan struct{})
	go func() {
		err := mSink.run()
		close(closed)
		require.Error(t, err)
		require.Equal(t, ErrEventSinkDeregistered, err)
	}()

	// Stop the parent context
	require.NoError(t, td.State().DeleteEventSinks(1001, []string{s1.ID}))

	select {
	case <-closed:
		// success
	case <-time.After(2 * time.Second):
		require.Fail(t, "expected managed sink to stop")
	}
}

// TestManagedSink_Run_UpdateProgress tests that the sink manager updates the
// event sinks progress in the state store
func TestManagedSink_Run_UpdateProgress(t *testing.T) {
	t.Parallel()

	// Start a test server and count the number of requests
	receivedCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var event structs.Events
		dec := json.NewDecoder(r.Body)
		require.NoError(t, dec.Decode(&event))
		require.Equal(t, "Deployment", string(event.Events[0].Topic))

		receivedCount++
	}))
	defer ts.Close()

	// Store two sinks
	s1 := mock.EventSink()
	s1.Address = ts.URL
	s2 := mock.EventSink()
	s2.Address = ts.URL

	srv, cancelSrv := TestServer(t, nil)
	defer cancelSrv()
	testutil.WaitForLeader(t, srv.RPC)

	require.NoError(t, srv.State().UpsertEventSink(1000, s1))
	require.NoError(t, srv.State().UpsertEventSink(1001, s2))

	// Get the manager from the server
	manager := srv.eventSinkManager

	// Wait for manager to be running
	testutil.WaitForResult(func() (bool, error) {
		if manager.Running() {
			return true, nil
		}
		return false, fmt.Errorf("expected manager to be running")
	}, func(error) {
		require.Fail(t, "expected manager to be running")
	})

	// Ensure the manager is running
	require.True(t, manager.Running())

	// Publish an event
	broker, err := srv.State().EventBroker()
	require.NoError(t, err)
	broker.Publish(&structs.Events{Index: 100, Events: []structs.Event{{Topic: "Deployment"}}})

	// Wait for the webhook to receive the event
	testutil.WaitForResult(func() (bool, error) {
		return receivedCount == 2, fmt.Errorf("webhook count not equal to expected want %d, got %d", 2, receivedCount)
	}, func(err error) {
		require.Fail(t, err.Error())
	})

	// force an update
	require.NoError(t, manager.updateSinkProgress())

	// Check that the progress was saved via raft
	testutil.WaitForResult(func() (bool, error) {
		out1, err := srv.State().EventSinkByID(nil, s1.ID)
		require.NoError(t, err)

		out2, err := srv.State().EventSinkByID(nil, s2.ID)
		require.NoError(t, err)

		if out1.LatestIndex == 100 && out2.LatestIndex == 100 {
			return true, nil
		}
		return false, fmt.Errorf("expected sinks to update from index out1: %d out2: %d", out1.LatestIndex, out2.LatestIndex)
	}, func(error) {
		require.Fail(t, "timeout waiting for progress to update via raft")
	})

	// Stop the server ensure manager is shutdown
	cancelSrv()

	testutil.WaitForResult(func() (bool, error) {
		running := manager.Running()
		if !running {
			return true, nil
		}
		return false, fmt.Errorf("expected manager to not be running")
	}, func(error) {
		require.Fail(t, "timeout waiting for manager to report not running")
	})
}
