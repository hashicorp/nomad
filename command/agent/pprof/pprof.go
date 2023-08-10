// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package profile is meant to be a near identical implemenation of
// https://golang.org/src/net/http/pprof/pprof.go
// It's purpose is to provide a way to accommodate the RPC endpoint style
// we use instead of traditional http handlers.

package pprof

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"strings"
	"time"
)

type ReqType string

const (
	CmdReq    ReqType = "cmdline"
	CPUReq    ReqType = "cpu"
	TraceReq  ReqType = "trace"
	LookupReq ReqType = "lookup"

	ErrProfileNotFoundPrefix = "Pprof profile not found profile:"
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
func Cmdline() ([]byte, map[string]string, error) {
	var buf bytes.Buffer
	fmt.Fprint(&buf, strings.Join(os.Args, "\x00"))

	return buf.Bytes(),
		map[string]string{
			"X-Content-Type-Options": "nosniff",
			"Content-Type":           "text/plain; charset=utf-8",
		}, nil
}

// Profile generates a pprof.Profile report for the given profile name
// see runtime/pprof/pprof.go for available profiles.
func Profile(profile string, debug, gc int) ([]byte, map[string]string, error) {
	p := pprof.Lookup(profile)
	if p == nil {
		return nil, nil, NewErrProfileNotFound(profile)
	}

	if profile == "heap" && gc > 0 {
		runtime.GC()
	}

	var buf bytes.Buffer
	if err := p.WriteTo(&buf, debug); err != nil {
		return nil, nil, err
	}

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
	}
	if debug != 0 {
		headers["Content-Type"] = "text/plain; charset=utf-8"
	} else {
		headers["Content-Type"] = "application/octet-stream"
		headers["Content-Disposition"] = fmt.Sprintf(`attachment; filename="%s"`, profile)
	}
	return buf.Bytes(), headers, nil
}

// CPUProfile generates a CPU Profile for a given duration
func CPUProfile(ctx context.Context, sec int) ([]byte, map[string]string, error) {
	if sec <= 0 {
		sec = 1
	}

	var buf bytes.Buffer
	if err := pprof.StartCPUProfile(&buf); err != nil {
		return nil, nil, err
	}

	sleep(ctx, time.Duration(sec)*time.Second)

	pprof.StopCPUProfile()

	return buf.Bytes(),
		map[string]string{
			"X-Content-Type-Options": "nosniff",
			"Content-Type":           "application/octet-stream",
			"Content-Disposition":    `attachment; filename="profile"`,
		}, nil
}

// Trace runs a trace profile for a given duration
func Trace(ctx context.Context, sec int) ([]byte, map[string]string, error) {
	if sec <= 0 {
		sec = 1
	}

	var buf bytes.Buffer
	if err := trace.Start(&buf); err != nil {
		return nil, nil, err
	}

	sleep(ctx, time.Duration(sec)*time.Second)

	trace.Stop()

	return buf.Bytes(),
		map[string]string{
			"X-Content-Type-Options": "nosniff",
			"Content-Type":           "application/octet-stream",
			"Content-Disposition":    `attachment; filename="trace"`,
		}, nil
}

func sleep(ctx context.Context, d time.Duration) {
	// Sleep until duration is met or ctx is cancelled
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}
