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

// Cmdline responds with the running program's
// command line, with arguments separated by NUL bytes.
func Cmdline() ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, strings.Join(os.Args, "\x00"))
	return buf.Bytes(), nil
}

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
)

// Profile
func Profile(profile string, debug int) ([]byte, error) {
	p := pprof.Lookup(profile)
	if p == nil {
		return nil, fmt.Errorf("Unknown profile: %s", profile)
	}

	var buf bytes.Buffer
	if err := p.WriteTo(&buf, debug); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func CPUProfile(sec int) ([]byte, error) {
	if sec <= 0 {
		sec = 1
	}

	var buf bytes.Buffer
	if err := pprof.StartCPUProfile(&buf); err != nil {
		// trace.Start failed, no writes yet
		return nil, err
	}

	sleep(context.TODO(), time.Duration(sec)*time.Second)

	pprof.StopCPUProfile()

	return buf.Bytes(), nil
}

func Trace(sec int) ([]byte, error) {
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
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}
