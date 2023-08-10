// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package goruntime contains helper functions related to the Go runtime.
package goruntime

import (
	"runtime"
	"strconv"
)

// RuntimeStats is used to return various runtime information
func RuntimeStats() map[string]string {
	return map[string]string{
		"kernel.name": runtime.GOOS,
		"arch":        runtime.GOARCH,
		"version":     runtime.Version(),
		"max_procs":   strconv.FormatInt(int64(runtime.GOMAXPROCS(0)), 10),
		"goroutines":  strconv.FormatInt(int64(runtime.NumGoroutine()), 10),
		"cpu_count":   strconv.FormatInt(int64(runtime.NumCPU()), 10),
	}
}
