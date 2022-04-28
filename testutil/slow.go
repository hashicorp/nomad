package testutil

import (
	"os"

	testing "github.com/mitchellh/go-testing-interface"
)

// SkipSlow skips a slow test unless the NOMAD_SLOW_TEST environment variable
// is set.
func SkipSlow(t testing.T) {
	if os.Getenv("NOMAD_SLOW_TEST") == "" {
		t.Skip("Skipping slow test. Set NOMAD_SLOW_TEST=1 to run.")
	}
}
