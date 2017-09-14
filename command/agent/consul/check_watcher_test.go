package consul

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/nomad/structs"
)

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
func testWatcherSetup() (*fakeChecksAPI, *checkWatcher) {
	fakeAPI := newFakeChecksAPI()
	cw := newCheckWatcher(testLogger(), fakeAPI)
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

	cw := newCheckWatcher(testLogger(), newFakeChecksAPI())
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

	fakeAPI, cw := testWatcherSetup()

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

	// Run for 1 second
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	cw.Run(ctx)

	// Assert Restart was never called
	if n := len(restarter1.restarts); n > 0 {
		t.Errorf("expected check 1 to not be restarted but found %d", n)
	}
	if n := len(restarter2.restarts); n > 0 {
		t.Errorf("expected check 2 to not be restarted but found %d", n)
	}
}

// TestCheckWatcher_Unhealthy asserts unhealthy tasks are not restarted.
func TestCheckWatcher_Unhealthy(t *testing.T) {
	t.Parallel()

	fakeAPI, cw := testWatcherSetup()

	check1 := testCheck()
	restarter1 := newFakeCheckRestarter(cw, "testalloc1", "testtask1", "testcheck1", check1)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check1, restarter1)

	check2 := testCheck()
	check2.CheckRestart.Limit = 1
	check2.CheckRestart.Grace = 200 * time.Millisecond
	restarter2 := newFakeCheckRestarter(cw, "testalloc2", "testtask2", "testcheck2", check2)
	cw.Watch("testalloc2", "testtask2", "testcheck2", check2, restarter2)

	// Check 1 always passes, check 2 always fails
	fakeAPI.add("testcheck1", "passing", time.Time{})
	fakeAPI.add("testcheck2", "critical", time.Time{})

	// Run for 1 second
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cw.Run(ctx)

	// Ensure restart was never called on check 1
	if n := len(restarter1.restarts); n > 0 {
		t.Errorf("expected check 1 to not be restarted but found %d", n)
	}

	// Ensure restart was called twice on check 2
	if n := len(restarter2.restarts); n != 2 {
		t.Errorf("expected check 2 to be restarted 2 times but found %d:\n%s", n, restarter2)
	}
}

// TestCheckWatcher_HealthyWarning asserts checks in warning with
// ignore_warnings=true do not restart tasks.
func TestCheckWatcher_HealthyWarning(t *testing.T) {
	t.Parallel()

	fakeAPI, cw := testWatcherSetup()

	check1 := testCheck()
	check1.CheckRestart.Limit = 1
	check1.CheckRestart.Grace = 0
	check1.CheckRestart.IgnoreWarnings = true
	restarter1 := newFakeCheckRestarter(cw, "testalloc1", "testtask1", "testcheck1", check1)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check1, restarter1)

	// Check is always in warning but that's ok
	fakeAPI.add("testcheck1", "warning", time.Time{})

	// Run for 1 second
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

	fakeAPI, cw := testWatcherSetup()

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

	fakeAPI, cw := testWatcherSetup()

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
