// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package serviceregistration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

// restartRecord is used by a fakeWorkloadRestarter to record when restarts occur
// due to a watched check.
type restartRecord struct {
	timestamp time.Time
	source    string
	reason    string
	failure   bool
}

// fakeWorkloadRestarter is a test implementation of TaskRestarter.
type fakeWorkloadRestarter struct {
	// restarts is a slice of all of the restarts triggered by the checkWatcher
	restarts []restartRecord

	// need the checkWatcher to re-Watch restarted tasks like TaskRunner
	watcher *UniversalCheckWatcher

	// check to re-Watch on restarts
	check     *structs.ServiceCheck
	allocID   string
	taskName  string
	checkName string

	lock sync.Mutex
}

// newFakeCheckRestart creates a new mock WorkloadRestarter.
func newFakeWorkloadRestarter(w *UniversalCheckWatcher, allocID, taskName, checkName string, c *structs.ServiceCheck) *fakeWorkloadRestarter {
	return &fakeWorkloadRestarter{
		watcher:   w,
		check:     c,
		allocID:   allocID,
		taskName:  taskName,
		checkName: checkName,
	}
}

// Restart implements part of the TaskRestarter interface needed for check watching
// and is normally fulfilled by a TaskRunner.
//
// Restarts are recorded in the []restarts field and re-Watch the check.
func (c *fakeWorkloadRestarter) Restart(_ context.Context, event *structs.TaskEvent, failure bool) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	restart := restartRecord{
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

// String is useful for debugging.
func (c *fakeWorkloadRestarter) String() string {
	c.lock.Lock()
	defer c.lock.Unlock()

	s := fmt.Sprintf("%s %s %s restarts:\n", c.allocID, c.taskName, c.checkName)
	for _, r := range c.restarts {
		s += fmt.Sprintf("%s - %s: %s (failure: %t)\n", r.timestamp, r.source, r.reason, r.failure)
	}
	return s
}

// GetRestarts for testing in a thread-safe way
func (c *fakeWorkloadRestarter) GetRestarts() []restartRecord {
	c.lock.Lock()
	defer c.lock.Unlock()

	o := make([]restartRecord, len(c.restarts))
	copy(o, c.restarts)
	return o
}

// response is a response returned by fakeCheckStatusGetter after a certain time
type response struct {
	at     time.Time
	id     string
	status string
}

// fakeCheckStatusGetter is a mock implementation of CheckStatusGetter
type fakeCheckStatusGetter struct {
	lock      sync.Mutex
	responses map[string][]response
}

func (g *fakeCheckStatusGetter) Get() (map[string]string, error) {
	g.lock.Lock()
	defer g.lock.Unlock()

	now := time.Now()
	result := make(map[string]string)
	// use the newest response after now for the response
	for k, vs := range g.responses {
		for _, v := range vs {
			if v.at.After(now) {
				break
			}
			result[k] = v.status
		}
	}

	return result, nil
}

func (g *fakeCheckStatusGetter) add(checkID, status string, at time.Time) {
	g.lock.Lock()
	defer g.lock.Unlock()
	if g.responses == nil {
		g.responses = make(map[string][]response)
	}
	g.responses[checkID] = append(g.responses[checkID], response{at, checkID, status})
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

// testWatcherSetup sets up a fakeChecksAPI and a real checkWatcher with a test
// logger and faster poll frequency.
func testWatcherSetup(t *testing.T) (*fakeCheckStatusGetter, *UniversalCheckWatcher) {
	logger := testlog.HCLogger(t)
	getter := new(fakeCheckStatusGetter)
	cw := NewCheckWatcher(logger, getter)
	cw.pollFrequency = 10 * time.Millisecond
	return getter, cw
}

func before() time.Time {
	return time.Now().Add(-10 * time.Second)
}

// TestCheckWatcher_SkipUnwatched asserts unwatched checks are ignored.
func TestCheckWatcher_SkipUnwatched(t *testing.T) {
	ci.Parallel(t)

	// Create a check with restarting disabled
	check := testCheck()
	check.CheckRestart = nil

	logger := testlog.HCLogger(t)
	getter := new(fakeCheckStatusGetter)

	cw := NewCheckWatcher(logger, getter)
	restarter1 := newFakeWorkloadRestarter(cw, "testalloc1", "testtask1", "testcheck1", check)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check, restarter1)

	// Check should have been dropped as it's not watched
	enqueued := len(cw.checkUpdateCh)
	must.Zero(t, enqueued, must.Sprintf("expected 0 checks to be enqueued for watching but found %d", enqueued))
}

// TestCheckWatcher_Healthy asserts healthy tasks are not restarted.
func TestCheckWatcher_Healthy(t *testing.T) {
	ci.Parallel(t)

	now := before()
	getter, cw := testWatcherSetup(t)

	// Make both checks healthy from the beginning
	getter.add("testcheck1", "passing", now)
	getter.add("testcheck2", "passing", now)

	check1 := testCheck()
	restarter1 := newFakeWorkloadRestarter(cw, "testalloc1", "testtask1", "testcheck1", check1)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check1, restarter1)

	check2 := testCheck()
	check2.CheckRestart.Limit = 1
	check2.CheckRestart.Grace = 0
	restarter2 := newFakeWorkloadRestarter(cw, "testalloc2", "testtask2", "testcheck2", check2)
	cw.Watch("testalloc2", "testtask2", "testcheck2", check2, restarter2)

	// Run
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cw.Run(ctx)

	// Ensure restart was never called
	must.SliceEmpty(t, restarter1.restarts, must.Sprint("expected check 1 to not be restarted"))
	must.SliceEmpty(t, restarter2.restarts, must.Sprint("expected check 2 to not be restarted"))
}

// TestCheckWatcher_Unhealthy asserts unhealthy tasks are restarted exactly once.
func TestCheckWatcher_Unhealthy(t *testing.T) {
	ci.Parallel(t)

	now := before()
	getter, cw := testWatcherSetup(t)

	// Check has always been failing
	getter.add("testcheck1", "critical", now)

	check1 := testCheck()
	restarter1 := newFakeWorkloadRestarter(cw, "testalloc1", "testtask1", "testcheck1", check1)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check1, restarter1)

	// Run
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cw.Run(ctx)

	// Ensure restart was called exactly once
	must.Len(t, 1, restarter1.restarts, must.Sprint("expected check to be restarted once"))
}

// TestCheckWatcher_HealthyWarning asserts checks in warning with
// ignore_warnings=true do not restart tasks.
func TestCheckWatcher_HealthyWarning(t *testing.T) {
	ci.Parallel(t)

	now := before()
	getter, cw := testWatcherSetup(t)

	// Check is always in warning but that's ok
	getter.add("testcheck1", "warning", now)

	check1 := testCheck()
	check1.CheckRestart.Limit = 1
	check1.CheckRestart.Grace = 0
	check1.CheckRestart.IgnoreWarnings = true
	restarter1 := newFakeWorkloadRestarter(cw, "testalloc1", "testtask1", "testcheck1", check1)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check1, restarter1)

	// Run
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	cw.Run(ctx)

	// Ensure restart was never called on check 1
	must.SliceEmpty(t, restarter1.restarts, must.Sprint("expected check 1 to not be restarted"))
}

// TestCheckWatcher_Flapping asserts checks that flap from healthy to unhealthy
// before the unhealthy limit is reached do not restart tasks.
func TestCheckWatcher_Flapping(t *testing.T) {
	ci.Parallel(t)

	getter, cw := testWatcherSetup(t)

	check1 := testCheck()
	check1.CheckRestart.Grace = 0
	restarter1 := newFakeWorkloadRestarter(cw, "testalloc1", "testtask1", "testcheck1", check1)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check1, restarter1)

	// Check flaps and is never failing for the full 200ms needed to restart
	now := time.Now()
	getter.add("testcheck1", "passing", now)
	getter.add("testcheck1", "critical", now.Add(100*time.Millisecond))
	getter.add("testcheck1", "passing", now.Add(250*time.Millisecond))
	getter.add("testcheck1", "critical", now.Add(300*time.Millisecond))
	getter.add("testcheck1", "passing", now.Add(450*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	cw.Run(ctx)

	// Ensure restart was never called on check 1
	must.SliceEmpty(t, restarter1.restarts, must.Sprint("expected check 1 to not be restarted"))
}

// TestCheckWatcher_Unwatch asserts unwatching checks prevents restarts.
func TestCheckWatcher_Unwatch(t *testing.T) {
	ci.Parallel(t)

	now := before()
	getter, cw := testWatcherSetup(t)

	// Always failing
	getter.add("testcheck1", "critical", now)

	// Unwatch immediately
	check1 := testCheck()
	check1.CheckRestart.Limit = 1
	check1.CheckRestart.Grace = 100 * time.Millisecond
	restarter1 := newFakeWorkloadRestarter(cw, "testalloc1", "testtask1", "testcheck1", check1)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check1, restarter1)
	cw.Unwatch("testcheck1")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	cw.Run(ctx)

	// Ensure restart was never called on check 1
	must.SliceEmpty(t, restarter1.restarts, must.Sprint("expected check 1 to not be restarted"))
}

// TestCheckWatcher_MultipleChecks asserts that when there are multiple checks
// for a single task, all checks should be removed when any of them restart the
// task to avoid multiple restarts.
func TestCheckWatcher_MultipleChecks(t *testing.T) {
	ci.Parallel(t)

	getter, cw := testWatcherSetup(t)

	// check is critical, 3 passing; should only be 1 net restart
	now := time.Now()
	getter.add("testcheck1", "critical", before())
	getter.add("testcheck1", "passing", now.Add(150*time.Millisecond))
	getter.add("testcheck2", "critical", before())
	getter.add("testcheck2", "passing", now.Add(150*time.Millisecond))
	getter.add("testcheck3", "passing", time.Time{})

	check1 := testCheck()
	check1.Name = "testcheck1"
	check1.CheckRestart.Limit = 1
	restarter1 := newFakeWorkloadRestarter(cw, "testalloc1", "testtask1", "testcheck1", check1)
	cw.Watch("testalloc1", "testtask1", "testcheck1", check1, restarter1)

	check2 := testCheck()
	check2.Name = "testcheck2"
	check2.CheckRestart.Limit = 1
	restarter2 := newFakeWorkloadRestarter(cw, "testalloc1", "testtask1", "testcheck2", check2)
	cw.Watch("testalloc1", "testtask1", "testcheck2", check2, restarter2)

	check3 := testCheck()
	check3.Name = "testcheck3"
	check3.CheckRestart.Limit = 1
	restarter3 := newFakeWorkloadRestarter(cw, "testalloc1", "testtask1", "testcheck3", check3)
	cw.Watch("testalloc1", "testtask1", "testcheck3", check3, restarter3)

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
	ci.Parallel(t)

	getter, cw := testWatcherSetup(t)

	// If TR.Restart blocks, restarting len(checkUpdateCh)+1 checks causes
	// a deadlock due to checkWatcher.Run being blocked in
	// checkRestart.apply and unable to process updates from the chan!
	n := cap(cw.checkUpdateCh) + 1
	checks := make([]*structs.ServiceCheck, n)
	restarters := make([]*fakeWorkloadRestarter, n)
	for i := 0; i < n; i++ {
		c := testCheck()
		r := newFakeWorkloadRestarter(cw,
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
		getter.add(r.checkName, "critical", time.Time{})
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
		must.NoError(t, err)
	})
}
