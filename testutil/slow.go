package testutil

import (
	"os"
	"strconv"
	"testing"
)

// SkipSlow skips a slow test unless the NOMAD_SLOW_TEST environment variable is set.
func SkipSlow(t *testing.T, reason string) {
	value := os.Getenv("NOMAD_SLOW_TEST")
	run, err := strconv.ParseBool(value)
	if !run || err != nil {
		t.Skipf("Skipping slow test: %s", reason)
	}
}

func Parallel(t *testing.T) {
	value := os.Getenv("CI")
	isCI, err := strconv.ParseBool(value)
	if !isCI || err != nil {
		t.Parallel()
	}
}
