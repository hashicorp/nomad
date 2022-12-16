package testutil

import (
	"os"
	"strconv"
	"testing"
)

// Copy of ci/slow.go for API.

// SkipSlow skips a slow test unless NOMAD_SLOW_TEST is set to a true value.
func SkipSlow(t *testing.T, reason string) {
	value := os.Getenv("NOMAD_SLOW_TEST")
	run, err := strconv.ParseBool(value)
	if !run || err != nil {
		t.Skipf("Skipping slow test: %s", reason)
	}
}

// Parallel runs t in parallel.
//
// The API package has been vetted to be concurrency safe (ish).
func Parallel(t *testing.T) {
	t.Parallel() // :)
}
