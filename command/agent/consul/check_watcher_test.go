package consul

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// checkRestartRecord is used by a testFakeCtx to record when restarts occur
// due to a watched check.
type checkRestartRecord struct {
	timestamp time.Time
	source    string
	reason    string
	failure   bool
}

// fakeCheckRestarter is a test implementation of TaskRestarter.
type fakeCheckRestarter struct {
	// restarts is a slice of all of the restarts triggered by the checkWatcher
	restarts []checkRestartRecord

	// need the checkWatcher to re-Watch restarted tasks like TaskRunner
	watcher *checkWatcher

	// check to re-Watch on restarts
	check     *structs.ServiceCheck
	allocID   string
	taskName  string
	checkName string

	mu sync.Mutex
}

// newFakeCheckRestart creates a new TaskRestarter. It needs all of the
// parameters checkWatcher.Watch expects.
func newFakeCheckRestarter(w *checkWatcher, allocID, taskName, checkName string, c *structs.ServiceCheck) *fakeCheckRestarter {
	return &fakeCheckRestarter{
		watcher:   w,
		check:     c,
		allocID:   allocID,
		taskName:  taskName,
		checkName: checkName,
	}
}

// Restart implements part of the TaskRestarter interface needed for check
// watching and is normally fulfilled by a TaskRunner.
//
// Restarts are recorded in the []restarts field and re-Watch the check.
//func (c *fakeCheckRestarter) Restart(source, reason string, failure bool) {
func (c *fakeCheckRestarter) Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	restart := checkRestartRecord{
		timestamp: time.Now(),
		source:    event.Type,
		reason:    event.DisplayMessage,
		failure:   failure,
	}
	c.restarts = append(c.restarts, restart)

	// Re-Watch the check just like TaskRunner
	c.watcher.Watch(c.allocID, c.taskName, c.checkName, c.check, c)
	return nil
}

// String for debugging
func (c *fakeCheckRestarter) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	s := fmt.Sprintf("%s %s %s restarts:\n", c.allocID, c.taskName, c.checkName)
	for _, r := range c.restarts {
		s += fmt.Sprintf("%s - %s: %s (failure: %t)\n", r.timestamp, r.source, r.reason, r.failure)
	}
	return s
}

// GetRestarts for testing in a threadsafe way
func (c *fakeCheckRestarter) GetRestarts() []checkRestartRecord {
	c.mu.Lock()
	defer c.mu.Unlock()

	o := make([]checkRestartRecord, len(c.restarts))
	copy(o, c.restarts)
	return o
}

// checkResponse is a response returned by the fakeChecksAPI after the given
// time.
type checkResponse struct {
	at     time.Time
	id     string
	status string
}

// fakeChecksAPI implements the Checks() method for testing Consul.
type fakeChecksAPI struct {
	// responses is a map of check ids to their status at a particular
	// time. checkResponses must be in chronological order.
	responses map[string][]checkResponse

	mu sync.Mutex
}

func newFakeChecksAPI() *fakeChecksAPI {
	return &fakeChecksAPI{responses: make(map[string][]checkResponse)}
}

// add a new check status to Consul at the given time.
func (c *fakeChecksAPI) add(id, status string, at time.Time) {
	c.mu.Lock()
	c.responses[id] = append(c.responses[id], checkResponse{at, id, status})
	c.mu.Unlock()
}

func (c *fakeChecksAPI) ChecksWithFilterOpts(filter string, opts *api.QueryOptions) (map[string]*api.AgentCheck, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	result := make(map[string]*api.AgentCheck, len(c.responses))

	// Use the latest response for each check
	for k, vs := range c.responses {
		for _, v := range vs {
			if v.at.After(now) {
				break
			}
			result[k] = &api.AgentCheck{
				CheckID: k,
				Name:    k,
				Status:  v.status,
			}
		}
	}

	return result, nil
}

// testWatcherSetup sets up a fakeChecksAPI and a real checkWatcher with a test
// logger and faster poll frequency.
func testWatcherSetup(t *testing.T) (*fakeChecksAPI, *checkWatcher) {
	logger := testlog.HCLogger(t)
	checksAPI := newFakeChecksAPI()
	namespacesClient := NewNamespacesClient(NewMockNamespaces(nil), NewMockAgent(ossFeatures))
	cw := newCheckWatcher(logger, checksAPI, namespacesClient)
	cw.pollFreq = 10 * time.Millisecond
	return checksAPI, cw
}

func testCheck() *structs.ServiceCheck {
	return &structs.ServiceCheck{
		Name:     "testcheck",
		Interval: 100 * time.Millisecond,
		Timeout:  100 * time.Millisecond,
		CheckRestart: &structs.CheckRestart{
			Limit:          3,
			Grace:          100 * time.Millisecond,
			IgnoreWarnings: false,
		},
	}
}

// TestCheckWatcher_Skip asserts unwatched checks are ignored.
func TestCheckWatcher_Skip(t *testing.T) {
	t.Parallel()

	// Create a check with restarting disabled
	check := testCheck()
	check.CheckRestart = nil

	logger := testlog.HCLogger(t)
	checksAPI := newFakeChecksAPI()
	namespacesClient := NewNamespacesClient(NewMockNamespaces(nil), NewMockAgent(ossFeatures))

	cw := newCheckWatcher(logger, checksAPI, namespacesClient)
	restarter1 := newFakeCheckRestarter(cw, "testalloc1", "testtask1", "testcheck1", check)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check, restarter1)

	// Check should have been dropped as it's not watched
	if n := len(cw.checkUpdateCh); n != 0 {
		t.Fatalf("expected 0 checks to be enqueued for watching but found %d", n)
	}
}

// TestCheckWatcher_Healthy asserts healthy tasks are not restarted.
func TestCheckWatcher_Healthy(t *testing.T) {
	t.Parallel()

	fakeAPI, cw := testWatcherSetup(t)

	check1 := testCheck()
	restarter1 := newFakeCheckRestarter(cw, "testalloc1", "testtask1", "testcheck1", check1)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check1, restarter1)

	check2 := testCheck()
	check2.CheckRestart.Limit = 1
	check2.CheckRestart.Grace = 0
	restarter2 := newFakeCheckRestarter(cw, "testalloc2", "testtask2", "testcheck2", check2)
	cw.Watch("testalloc2", "testtask2", "testcheck2", check2, restarter2)

	// Make both checks healthy from the beginning
	fakeAPI.add("testcheck1", "passing", time.Time{})
	fakeAPI.add("testcheck2", "passing", time.Time{})

	// Run
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cw.Run(ctx)

	// Ensure restart was never called
	if n := len(restarter1.restarts); n > 0 {
		t.Errorf("expected check 1 to not be restarted but found %d:\n%s", n, restarter1)
	}
	if n := len(restarter2.restarts); n > 0 {
		t.Errorf("expected check 2 to not be restarted but found %d:\n%s", n, restarter2)
	}
}

// TestCheckWatcher_Unhealthy asserts unhealthy tasks are restarted exactly once.
func TestCheckWatcher_Unhealthy(t *testing.T) {
	t.Parallel()

	fakeAPI, cw := testWatcherSetup(t)

	check1 := testCheck()
	restarter1 := newFakeCheckRestarter(cw, "testalloc1", "testtask1", "testcheck1", check1)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check1, restarter1)

	// Check has always been failing
	fakeAPI.add("testcheck1", "critical", time.Time{})

	// Run
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cw.Run(ctx)

	// Ensure restart was called exactly once
	require.Len(t, restarter1.restarts, 1)
}

// TestCheckWatcher_HealthyWarning asserts checks in warning with
// ignore_warnings=true do not restart tasks.
func TestCheckWatcher_HealthyWarning(t *testing.T) {
	t.Parallel()

	fakeAPI, cw := testWatcherSetup(t)

	check1 := testCheck()
	check1.CheckRestart.Limit = 1
	check1.CheckRestart.Grace = 0
	check1.CheckRestart.IgnoreWarnings = true
	restarter1 := newFakeCheckRestarter(cw, "testalloc1", "testtask1", "testcheck1", check1)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check1, restarter1)

	// Check is always in warning but that's ok
	fakeAPI.add("testcheck1", "warning", time.Time{})

	// Run
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	cw.Run(ctx)

	// Ensure restart was never called on check 1
	if n := len(restarter1.restarts); n > 0 {
		t.Errorf("expected check 1 to not be restarted but found %d", n)
	}
}

// TestCheckWatcher_Flapping asserts checks that flap from healthy to unhealthy
// before the unhealthy limit is reached do not restart tasks.
func TestCheckWatcher_Flapping(t *testing.T) {
	t.Parallel()

	fakeAPI, cw := testWatcherSetup(t)

	check1 := testCheck()
	check1.CheckRestart.Grace = 0
	restarter1 := newFakeCheckRestarter(cw, "testalloc1", "testtask1", "testcheck1", check1)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check1, restarter1)

	// Check flaps and is never failing for the full 200ms needed to restart
	now := time.Now()
	fakeAPI.add("testcheck1", "passing", now)
	fakeAPI.add("testcheck1", "critical", now.Add(100*time.Millisecond))
	fakeAPI.add("testcheck1", "passing", now.Add(250*time.Millisecond))
	fakeAPI.add("testcheck1", "critical", now.Add(300*time.Millisecond))
	fakeAPI.add("testcheck1", "passing", now.Add(450*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	cw.Run(ctx)

	// Ensure restart was never called on check 1
	if n := len(restarter1.restarts); n > 0 {
		t.Errorf("expected check 1 to not be restarted but found %d\n%s", n, restarter1)
	}
}

// TestCheckWatcher_Unwatch asserts unwatching checks prevents restarts.
func TestCheckWatcher_Unwatch(t *testing.T) {
	t.Parallel()

	fakeAPI, cw := testWatcherSetup(t)

	// Unwatch immediately
	check1 := testCheck()
	check1.CheckRestart.Limit = 1
	check1.CheckRestart.Grace = 100 * time.Millisecond
	restarter1 := newFakeCheckRestarter(cw, "testalloc1", "testtask1", "testcheck1", check1)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check1, restarter1)
	cw.Unwatch("testcheck1")

	// Always failing
	fakeAPI.add("testcheck1", "critical", time.Time{})

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	cw.Run(ctx)

	// Ensure restart was never called on check 1
	if n := len(restarter1.restarts); n > 0 {
		t.Errorf("expected check 1 to not be restarted but found %d\n%s", n, restarter1)
	}
}

// TestCheckWatcher_MultipleChecks asserts that when there are multiple checks
// for a single task, all checks should be removed when any of them restart the
// task to avoid multiple restarts.
func TestCheckWatcher_MultipleChecks(t *testing.T) {
	t.Parallel()

	fakeAPI, cw := testWatcherSetup(t)

	check1 := testCheck()
	check1.CheckRestart.Limit = 1
	restarter1 := newFakeCheckRestarter(cw, "testalloc1", "testtask1", "testcheck1", check1)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check1, restarter1)

	check2 := testCheck()
	check2.CheckRestart.Limit = 1
	restarter2 := newFakeCheckRestarter(cw, "testalloc1", "testtask1", "testcheck2", check2)
	cw.Watch("testalloc1", "testtask1", "testcheck2", check2, restarter2)

	check3 := testCheck()
	check3.CheckRestart.Limit = 1
	restarter3 := newFakeCheckRestarter(cw, "testalloc1", "testtask1", "testcheck3", check3)
	cw.Watch("testalloc1", "testtask1", "testcheck3", check3, restarter3)

	// check 2 & 3 fail long enough to cause 1 restart, but only 1 should restart
	now := time.Now()
	fakeAPI.add("testcheck1", "critical", now)
	fakeAPI.add("testcheck1", "passing", now.Add(150*time.Millisecond))
	fakeAPI.add("testcheck2", "critical", now)
	fakeAPI.add("testcheck2", "passing", now.Add(150*time.Millisecond))
	fakeAPI.add("testcheck3", "passing", time.Time{})

	// Run
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cw.Run(ctx)

	// Ensure that restart was only called once on check 1 or 2. Since
	// checks are in a map it's random which check triggers the restart
	// first.
	if n := len(restarter1.restarts) + len(restarter2.restarts); n != 1 {
		t.Errorf("expected check 1 & 2 to be restarted 1 time but found %d\ncheck 1:\n%s\ncheck 2:%s",
			n, restarter1, restarter2)
	}

	if n := len(restarter3.restarts); n != 0 {
		t.Errorf("expected check 3 to not be restarted but found %d:\n%s", n, restarter3)
	}
}

// TestCheckWatcher_Deadlock asserts that check watcher will not deadlock when
// attempting to restart a task even if its update queue is full.
// https://github.com/hashicorp/nomad/issues/5395
func TestCheckWatcher_Deadlock(t *testing.T) {
	t.Parallel()

	fakeAPI, cw := testWatcherSetup(t)

	// If TR.Restart blocks, restarting len(checkUpdateCh)+1 checks causes
	// a deadlock due to checkWatcher.Run being blocked in
	// checkRestart.apply and unable to process updates from the chan!
	n := cap(cw.checkUpdateCh) + 1
	checks := make([]*structs.ServiceCheck, n)
	restarters := make([]*fakeCheckRestarter, n)
	for i := 0; i < n; i++ {
		c := testCheck()
		r := newFakeCheckRestarter(cw,
			fmt.Sprintf("alloc%d", i),
			fmt.Sprintf("task%d", i),
			fmt.Sprintf("check%d", i),
			c,
		)
		checks[i] = c
		restarters[i] = r
	}

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cw.Run(ctx)

	// Watch
	for _, r := range restarters {
		cw.Watch(r.allocID, r.taskName, r.checkName, r.check, r)
	}

	// Make them all fail
	for _, r := range restarters {
		fakeAPI.add(r.checkName, "critical", time.Time{})
	}

	// Ensure that restart was called exactly once on all checks
	testutil.WaitForResult(func() (bool, error) {
		for _, r := range restarters {
			if n := len(r.GetRestarts()); n != 1 {
				return false, fmt.Errorf("expected 1 restart but found %d", n)
			}
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}
