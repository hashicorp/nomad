package consul

import (
	"context"
	"log"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/nomad/structs"
)

// heartbeater is the subset of consul agent functionality needed by script
// checks to heartbeat
type heartbeater interface {
	UpdateTTL(id, output, status string) error
}

// scriptHandle is returned by scriptCheck.run by cancelling a scriptCheck and
// waiting for it to shutdown.
type scriptHandle struct {
	// cancel the script
	cancel func()
	exitCh chan struct{}
}

// wait returns a chan that's closed when the script exits
func (s *scriptHandle) wait() <-chan struct{} {
	return s.exitCh
}

// scriptCheck runs script checks via a ScriptExecutor and updates the
// appropriate check's TTL when the script succeeds.
type scriptCheck struct {
	allocID  string
	taskName string

	id    string
	check *structs.ServiceCheck
	exec  driver.ScriptExecutor
	agent heartbeater

	// lastCheckOk is true if the last check was ok; otherwise false
	lastCheckOk bool

	logger     *log.Logger
	shutdownCh <-chan struct{}
}

// newScriptCheck creates a new scriptCheck. run() should be called once the
// initial check is registered with Consul.
func newScriptCheck(allocID, taskName, checkID string, check *structs.ServiceCheck,
	exec driver.ScriptExecutor, agent heartbeater, logger *log.Logger,
	shutdownCh <-chan struct{}) *scriptCheck {

	return &scriptCheck{
		allocID:     allocID,
		taskName:    taskName,
		id:          checkID,
		check:       check,
		exec:        exec,
		agent:       agent,
		lastCheckOk: true, // start logging on first failure
		logger:      logger,
		shutdownCh:  shutdownCh,
	}
}

// run this script check and return its cancel func. If the shutdownCh is
// closed the check will be run once more before exiting.
func (s *scriptCheck) run() *scriptHandle {
	ctx, cancel := context.WithCancel(context.Background())
	exitCh := make(chan struct{})
	go func() {
		defer close(exitCh)
		timer := time.NewTimer(0)
		defer timer.Stop()
		for {
			// Block until check is removed, Nomad is shutting
			// down, or the check interval is up
			select {
			case <-ctx.Done():
				// check has been removed
				return
			case <-s.shutdownCh:
				// unblock but don't exit until after we heartbeat once more
			case <-timer.C:
				timer.Reset(s.check.Interval)
			}
			metrics.IncrCounter([]string{"client", "consul", "script_runs"}, 1)

			// Execute check script with timeout
			execctx, cancel := context.WithTimeout(ctx, s.check.Timeout)
			output, code, err := s.exec.Exec(execctx, s.check.Command, s.check.Args)
			switch execctx.Err() {
			case context.Canceled:
				// check removed during execution; exit
				cancel()
				return
			case context.DeadlineExceeded:
				metrics.IncrCounter([]string{"client", "consul", "script_timeouts"}, 1)
				// If no error was returned, set one to make sure the task goes critical
				if err == nil {
					err = context.DeadlineExceeded
				}

				// Log deadline exceeded every time as it's a
				// distinct issue from checks returning
				// failures
				s.logger.Printf("[WARN] consul.checks: check %q for task %q alloc %q timed out (%s)",
					s.check.Name, s.taskName, s.allocID, s.check.Timeout)
			}

			// cleanup context
			cancel()

			state := api.HealthCritical
			switch code {
			case 0:
				state = api.HealthPassing
			case 1:
				state = api.HealthWarning
			}

			var outputMsg string
			if err != nil {
				state = api.HealthCritical
				outputMsg = err.Error()
			} else {
				outputMsg = string(output)
			}

			// Actually heartbeat the check
			err = s.agent.UpdateTTL(s.id, outputMsg, state)
			select {
			case <-ctx.Done():
				// check has been removed; don't report errors
				return
			default:
			}

			if err != nil {
				if s.lastCheckOk {
					s.lastCheckOk = false
					s.logger.Printf("[WARN] consul.checks: update for task %q alloc %q check %q failed: %v",
						s.taskName, s.allocID, s.check.Name, err)
				} else {
					s.logger.Printf("[DEBUG] consul.checks: update for task %q alloc %q check %q still failing: %v",
						s.taskName, s.allocID, s.check.Name, err)
				}

			} else if !s.lastCheckOk {
				// Succeeded for the first time or after failing; log
				s.lastCheckOk = true
				s.logger.Printf("[INFO] consul.checks: update for task %q alloc %q check %q succeeded",
					s.taskName, s.allocID, s.check.Name)
			}

			select {
			case <-s.shutdownCh:
				// We've been told to exit and just heartbeated so exit
				return
			default:
			}
		}
	}()
	return &scriptHandle{cancel: cancel, exitCh: exitCh}
}
