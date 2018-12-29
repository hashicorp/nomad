package structs

import (
	"fmt"
	"time"
)

const (
	// The default user that the executor uses to run tasks
	DefaultUnprivilegedUser = "nobody"

	// CheckBufSize is the size of the check output result
	CheckBufSize = 4 * 1024
)

// WaitResult stores the result of a Wait operation.
type WaitResult struct {
	ExitCode int
	Signal   int
	Err      error
}

func NewWaitResult(code, signal int, err error) *WaitResult {
	return &WaitResult{
		ExitCode: code,
		Signal:   signal,
		Err:      err,
	}
}

func (r *WaitResult) Successful() bool {
	return r.ExitCode == 0 && r.Signal == 0 && r.Err == nil
}

func (r *WaitResult) String() string {
	return fmt.Sprintf("Wait returned exit code %v, signal %v, and error %v",
		r.ExitCode, r.Signal, r.Err)
}

// CheckResult encapsulates the result of a check
type CheckResult struct {

	// ExitCode is the exit code of the check
	ExitCode int

	// Output is the output of the check script
	Output string

	// Timestamp is the time at which the check was executed
	Timestamp time.Time

	// Duration is the time it took the check to run
	Duration time.Duration

	// Err is the error that a check returned
	Err error
}

// ExecutorConfig is the config that Nomad passes to the executor
type ExecutorConfig struct {

	// LogFile is the file to which Executor logs
	LogFile string

	// LogLevel is the level of the logs to putout
	LogLevel string
}
