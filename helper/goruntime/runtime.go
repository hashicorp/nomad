// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package goruntime contains helper functions related to the Go runtime.
package goruntime

import (
	"runtime"
	"strconv"
	"time"

	humanize "github.com/dustin/go-humanize"
	// shelpers "github.com/hashicorp/nomad/command/helpers.go"
)

// RuntimeStats is used to return various runtime information
func RuntimeStats() map[string]string {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// LastGC is the time the last garbage collection finished, as nanoseconds since 1970 (the UNIX epoch).
	lastGC := time.Unix(0, int64(memStats.LastGC))
	gcSince := time.Since(lastGC)

	return map[string]string{
		"kernel.name":               runtime.GOOS,
		"arch":                      runtime.GOARCH,
		"version":                   runtime.Version(),
		"max_procs":                 strconv.FormatInt(int64(runtime.GOMAXPROCS(0)), 10),
		"goroutines":                strconv.FormatInt(int64(runtime.NumGoroutine()), 10),
		"cpu_count":                 strconv.FormatInt(int64(runtime.NumCPU()), 10),
		"mem_last_gc":               lastGC.UTC().String(),
		"mem_last_gc_nanos":         strconv.FormatInt(int64(memStats.LastGC), 10),
		"mem_last_gc_formatted":     humanize.Time(lastGC),
		"mem_last_gc_since":         gcSince.String(),
		"mem_last_gc_since_rounded": (gcSince.Round(time.Second)).String(),

		// Alloc is bytes of allocated heap objects.
		//   This is the same as HeapAlloc (see below).
		"mem_alloc":          strconv.FormatInt(int64(memStats.Alloc), 10),
		"mem_alloc_humanize": humanize.IBytes(memStats.Alloc),

		// Sys is the total bytes of memory obtained from the OS.
		//   Sys is the sum of the XSys fields below. Sys measures the virtual
		//   address space reserved by the Go runtime for the heap, stacks, and other
		//   internal data structures. It's likely that not all of the virtual
		//   address space is backed by physical memory at any given moment, though
		//   in general it all was at some point.
		"mem_sys":          strconv.FormatInt(int64(memStats.Sys), 10),
		"mem_sys_humanize": humanize.IBytes(memStats.Sys),

		// NumCgoCall returns the number of cgo calls made by the current process.
		"num_cgo_call": strconv.FormatInt(int64(runtime.NumCgoCall()), 10),
	}
}
