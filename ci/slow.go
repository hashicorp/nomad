package ci

import (
	"os"
	"strconv"
	"testing"
)

// SkipSlow skips a slow test unless NOMAD_SLOW_TEST is set to a true value.
func SkipSlow(t *testing.T, reason string) {
	value := os.Getenv("NOMAD_SLOW_TEST")
	run, err := strconv.ParseBool(value)
	if !run || err != nil {
		t.Skipf("Skipping slow test: %s", reason)
	}
}

// Parallel runs t in parallel, unless CI is set to a true value.
//
// In CI (CircleCI / GitHub Actions) we get better performance by running tests
// in serial while not restricting GOMAXPROCS.
func Parallel(t *testing.T) {
	value := os.Getenv("CI")
	isCI, err := strconv.ParseBool(value)
	if !isCI || err != nil {
		t.Parallel()
	}
}
