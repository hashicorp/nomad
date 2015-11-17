package structs

import "fmt"

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
