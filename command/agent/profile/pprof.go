package profile

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime/pprof"
	"runtime/trace"
	"strings"
	"time"
)

// goroutine
// threadcreate
// heap
// allocs
// block
// mutex
type ReqType string

const (
	CmdReq    ReqType = "cmdline"
	CPUReq    ReqType = "cpu"
	TraceReq  ReqType = "trace"
	LookupReq ReqType = "profile"

	ErrProfileNotFoundPrefix = "Pprof profile not found"
)

// NewErrProfileNotFound returns a new error caused by a pprof.Lookup
// profile not being found
func NewErrProfileNotFound(profile string) error {
	return fmt.Errorf("%s %s", ErrProfileNotFoundPrefix, profile)
}

// IsErrProfileNotFound returns whether the error is due to a pprof profile
// being invalid
func IsErrProfileNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrProfileNotFoundPrefix)
}

// Cmdline responds with the running program's
// command line, with arguments separated by NUL bytes.
func Cmdline() ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, strings.Join(os.Args, "\x00"))
	return buf.Bytes(), nil
}

// Profile generates a pprof.Profile report for the given profile name
// see runtime/pprof/pprof.go for available profiles.
func Profile(profile string, debug int) ([]byte, error) {
	p := pprof.Lookup(profile)
	if p == nil {
		return nil, NewErrProfileNotFound(profile)
	}

	var buf bytes.Buffer
	if err := p.WriteTo(&buf, debug); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// CPUProfile generates a CPU Profile for a given duration
func CPUProfile(ctx context.Context, sec int) ([]byte, error) {
	if sec <= 0 {
		sec = 1
	}

	var buf bytes.Buffer
	if err := pprof.StartCPUProfile(&buf); err != nil {
		// trace.Start failed, no writes yet
		return nil, err
	}

	sleep(ctx, time.Duration(sec)*time.Second)

	pprof.StopCPUProfile()

	return buf.Bytes(), nil
}

// Trace runs a trace profile for a given duration
func Trace(ctx context.Context, sec int) ([]byte, error) {
	if sec <= 0 {
		sec = 1
	}

	var buf bytes.Buffer
	if err := trace.Start(&buf); err != nil {
		// trace.Start failed, no writes yet
		return nil, err
	}

	sleep(context.TODO(), time.Duration(sec)*time.Second)

	trace.Stop()

	return buf.Bytes(), nil
}

func sleep(ctx context.Context, d time.Duration) {
	// Sleep until duration is met or ctx is cancelled
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}
