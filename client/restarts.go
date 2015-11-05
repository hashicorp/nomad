package client

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"time"
)

// The errorCounter keeps track of the number of times a process has exited
// It returns the duration after which a task is restarted
// For Batch jobs, the interval is set to zero value since the takss
// will be restarted only upto maxAttempts times
type restartTracker interface {
	nextRestart() (bool, time.Duration)
	increment()
}

func newRestartTracker(jobType string, restartPolicy *structs.RestartPolicy) restartTracker {
	switch jobType {
	case structs.JobTypeService:
		return &serviceRestartTracker{
			maxAttempts: restartPolicy.Attempts,
			startTime:   time.Now(),
			interval:    restartPolicy.Interval,
			delay:       restartPolicy.Delay,
		}
	default:
		return &batchRestartTracker{
			maxAttempts: restartPolicy.Attempts,
			delay:       restartPolicy.Delay,
		}
	}
}

type batchRestartTracker struct {
	maxAttempts int
	delay       time.Duration

	count int
}

func (b *batchRestartTracker) increment() {
	b.count = b.count + 1
}

func (b *batchRestartTracker) nextRestart() (bool, time.Duration) {
	if b.count < b.maxAttempts {
		return true, b.delay
	}
	return false, 0
}

type serviceRestartTracker struct {
	maxAttempts int
	delay       time.Duration
	interval    time.Duration

	count     int
	startTime time.Time
}

func (c *serviceRestartTracker) increment() {
	if c.count <= c.maxAttempts {
		c.count = c.count + 1
	}
}

func (c *serviceRestartTracker) nextRestart() (bool, time.Duration) {
	windowEndTime := c.startTime.Add(c.interval)
	now := time.Now()
	if now.After(windowEndTime) {
		c.count = 0
		c.startTime = time.Now()
		return true, c.delay
	}

	if c.count < c.maxAttempts {
		return true, c.delay
	}

	return true, windowEndTime.Sub(now)
}
