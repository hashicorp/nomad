package consul

import (
	"log"
	"math/rand"
	"sync"
	"time"

	cstructs "github.com/hashicorp/nomad/client/driver/structs"
)

// NomadCheck runs a given check in a specific interval and update a
// corresponding Consul TTL check
type NomadCheck struct {
	check    Check
	runCheck func(Check)
	logger   *log.Logger
	stop     bool
	stopCh   chan struct{}
	stopLock sync.Mutex

	started     bool
	startedLock sync.Mutex
}

// NewNomadCheck configures and returns a NomadCheck
func NewNomadCheck(check Check, runCheck func(Check), logger *log.Logger) *NomadCheck {
	nc := NomadCheck{
		check:    check,
		runCheck: runCheck,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
	return &nc
}

// Start is used to start the check. The check runs until stop is called
func (n *NomadCheck) Start() {
	n.startedLock.Lock()
	if n.started {
		return
	}
	n.started = true
	n.stopLock.Lock()
	defer n.stopLock.Unlock()
	n.stopCh = make(chan struct{})
	go n.run()
}

// Stop is used to stop the check.
func (n *NomadCheck) Stop() {
	n.stopLock.Lock()
	defer n.stopLock.Unlock()
	if !n.stop {
		n.stop = true
		close(n.stopCh)
	}
}

// run is invoked by a goroutine to run until Stop() is called
func (n *NomadCheck) run() {
	// Get the randomized initial pause time
	initialPauseTime := randomStagger(n.check.Interval())
	n.logger.Printf("[DEBUG] agent: pausing %v before first invocation of %s", initialPauseTime, n.check.ID())
	next := time.After(initialPauseTime)
	for {
		select {
		case <-next:
			n.runCheck(n.check)
			next = time.After(n.check.Interval())
		case <-n.stopCh:
			return
		}
	}
}

// Check is an interface which check providers can implement for Nomad to run
type Check interface {
	Run() *cstructs.CheckResult
	ID() string
	Interval() time.Duration
}

// Returns a random stagger interval between 0 and the duration
func randomStagger(intv time.Duration) time.Duration {
	return time.Duration(uint64(rand.Int63()) % uint64(intv))
}
