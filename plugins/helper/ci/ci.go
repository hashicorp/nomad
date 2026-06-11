package ci

import (
	"os"
	"strconv"
)

type TestingT interface {
	Parallel()
}

// Parallel runs t in parallel, unless CI is set to a true value.
//
// In CI (CircleCI / GitHub Actions) we get better performance by running tests
// in serial while not restricting GOMAXPROCS.
func Parallel(t TestingT) {
	value := os.Getenv("CI")
	isCI, err := strconv.ParseBool(value)
	if !isCI || err != nil {
		t.Parallel()
	}
}
