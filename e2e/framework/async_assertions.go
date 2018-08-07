package framework

import (
	"fmt"
	"time"

	"github.com/stretchr/testify/assert"
)

var defaultEventuallyTimeout = time.Second
var defaultEventuallyPollingInterval = 10 * time.Millisecond
var defaultConsistentlyTimeout = 100 * time.Millisecond
var defaultConsistentlyPollingInterval = 10 * time.Millisecond

type asyncAssert struct {
	f               func() error
	timeoutInterval time.Duration
	pollingInterval time.Duration
}

func newAsyncAssert(f func() error, timeout, polling time.Duration) *asyncAssert {
	return &asyncAssert{
		f:               f,
		timeoutInterval: timeout,
		pollingInterval: polling,
	}
}

func (a *asyncAssert) runEventually(t *assert.Assertions) {
	timer := time.Now()
	timeout := time.After(a.timeoutInterval)
	err := a.f()
	for {
		if err == nil {
			return
		}

		select {
		case <-time.After(a.pollingInterval):
			err = a.f()
		case <-timeout:
			t.FailNow(fmt.Sprintf("Eventually failed with error after %.3fs:\n%+v",
				time.Since(timer).Seconds(), err))
			return
		}
	}
}

func (a *asyncAssert) runConsistently(t *assert.Assertions) {
	timer := time.Now()
	timeout := time.After(a.timeoutInterval)
	err := a.f()
	for {
		if err != nil {
			t.FailNow(fmt.Sprintf("Consistently failed with error after %.3fs:\n%+v",
				time.Since(timer).Seconds(), err))
			return
		}

		select {
		case <-time.After(a.pollingInterval):
			err = a.f()
		case <-timeout:
			return
		}
	}
}

type AsyncAssertions struct {
	a *assert.Assertions
}

func (a *AsyncAssertions) Eventually(f func() error, intervals ...time.Duration) {
	timeoutInterval := defaultEventuallyTimeout
	pollingInterval := defaultEventuallyPollingInterval
	if len(intervals) > 0 {
		timeoutInterval = intervals[0]
	}
	if len(intervals) > 1 {
		pollingInterval = intervals[1]
	}

	newAsyncAssert(f, timeoutInterval, pollingInterval).runEventually(a.a)
}

func (a *AsyncAssertions) Consistently(f func() error, intervals ...time.Duration) {
	timeoutInterval := defaultConsistentlyTimeout
	pollingInterval := defaultConsistentlyPollingInterval
	if len(intervals) > 0 {
		timeoutInterval = intervals[0]
	}
	if len(intervals) > 1 {
		pollingInterval = intervals[1]
	}

	newAsyncAssert(f, timeoutInterval, pollingInterval).runConsistently(a.a)

}
