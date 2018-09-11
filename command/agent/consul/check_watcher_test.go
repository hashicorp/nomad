package consul

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
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
func (c *fakeCheckRestarter) Restart(source, reason string, failure bool) {
	c.restarts = append(c.restarts, checkRestartRecord{time.Now(), source, reason, failure})

	// Re-Watch the check just like TaskRunner
	c.watcher.Watch(c.allocID, c.taskName, c.checkName, c.check, c)
}

// String for debugging
func (c *fakeCheckRestarter) String() string {
	s := fmt.Sprintf("%s %s %s restarts:\n", c.allocID, c.taskName, c.checkName)
	for _, r := range c.restarts {
		s += fmt.Sprintf("%s - %s: %s (failure: %t)\n", r.timestamp, r.source, r.reason, r.failure)
	}
	return s
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
}

func newFakeChecksAPI() *fakeChecksAPI {
	return &fakeChecksAPI{responses: make(map[string][]checkResponse)}
}

// add a new check status to Consul at the given time.
func (c *fakeChecksAPI) add(id, status string, at time.Time) {
	c.responses[id] = append(c.responses[id], checkResponse{at, id, status})
}

func (c *fakeChecksAPI) Checks() (map[string]*api.AgentCheck, error) {
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
	fakeAPI := newFakeChecksAPI()
	cw := newCheckWatcher(testlog.Logger(t), fakeAPI)
	cw.pollFreq = 10 * time.Millisecond
	return fakeAPI, cw
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

	cw := newCheckWatcher(testlog.Logger(t), newFakeChecksAPI())
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
