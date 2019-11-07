package testutil

import (
	"os"
	"time"
)

type testFn func() (bool, error)
type errorFn func(error)

func WaitForResult(test testFn, error errorFn) {
	WaitForResultRetries(500*TestMultiplier(), test, error)
}

func WaitForResultRetries(retries int64, test testFn, error errorFn) {
	for retries > 0 {
		time.Sleep(10 * time.Millisecond)
		retries--

		success, err := test()
		if success {
			return
		}

		if retries == 0 {
			error(err)
		}
	}
}

// AssertUntil asserts the test function passes throughout the given duration.
// Otherwise error is called on failure.
func AssertUntil(until time.Duration, test testFn, error errorFn) {
	deadline := time.Now().Add(until)
	for time.Now().Before(deadline) {
		success, err := test()
		if !success {
			error(err)
			return
		}
		// Sleep some arbitrary fraction of the deadline
		time.Sleep(until / 30)
	}
}

// TestMultiplier returns a multiplier for retries and waits given environment
// the tests are being run under.
func TestMultiplier() int64 {
	if IsCI() {
		return 4
	}

	return 1
}

// Timeout takes the desired timeout and increases it if running in Travis
func Timeout(original time.Duration) time.Duration {
	return original * time.Duration(TestMultiplier())
}

func IsCI() bool {
	_, ok := os.LookupEnv("CI")
	return ok
}

func IsTravis() bool {
	_, ok := os.LookupEnv("TRAVIS")
	return ok
}

func IsAppVeyor() bool {
	_, ok := os.LookupEnv("APPVEYOR")
	return ok
}
