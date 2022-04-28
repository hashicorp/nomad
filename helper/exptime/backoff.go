// Package exptime provides a generalized exponential backoff retry implementation.
//
// This package was copied from oss.indeed.com/go/libtime/decay and modified.
package exptime

import (
	"errors"
	"fmt"
	"math/rand"
	"time"
)

var (
	// ErrMaximumTimeExceeded indicates the maximum wait time has been exceeded.
	ErrMaximumTimeExceeded = errors.New("maximum backoff time exceeded")
)

// A TryFunc is what gets executed between retry wait periods during execution
// of Backoff. The keepRetrying return value is used to control whether a retry
// attempt should be made. This feature is useful in manipulating control flow
// in cases where it is known a retry will not be successful.
type TryFunc func() (keepRetrying bool, err error)

// BackoffOptions allow for fine-tuning backoff behavior.
type BackoffOptions struct {
	// MaxSleepTime represents the maximum amount of time
	// the exponential backoff system will spend sleeping,
	// accumulating the amount of time spent asleep between
	// retries.
	//
	// The algorithm starts at an interval of InitialGapSize
	// and increases exponentially (x2 each iteration) from there.
	// With no jitter, a MaxSleepTime of 10 seconds and InitialGapSize
	// of 1 millisecond would suggest a total of 15 attempts
	// (since the very last retry truncates the sleep time to
	// align exactly with MaxSleepTime).
	MaxSleepTime time.Duration

	// InitialGapSize sets the initial amount of time the algorithm
	// will sleep before the first retry (after the first attempt).
	// The actual amount of sleep time will include a random amount
	// of jitter, if MaxJitterSize is non-zero.
	InitialGapSize time.Duration

	// MaxJitterSize limits how much randomness we may
	// introduce in the duration of each retry interval.
	// The purpose of introducing jitter is to mitigate the
	// effect of thundering herds
	MaxJitterSize time.Duration

	// RandomSeed is used for generating a randomly computed
	// jitter size for each retry.
	RandomSeed int64

	// Sleeper is used to cause the process to sleep for
	// a computed amount of time. If not set, a default
	// implementation based on time.Sleep will be used.
	Sleeper Sleeper
}

// A Sleeper is a useful way for calling time.Sleep
// in a mock-able way for tests.
type Sleeper func(time.Duration)

// Backoff will attempt to execute function using a configurable
// exponential backoff algorithm. function is a TryFunc which requires
// two return parameters - a boolean for optimizing control flow, and
// an error for reporting failure conditions. If the first parameter is
// false, the backoff algorithm will abandon further retry attempts and
// simply return an error. Otherwise, if the returned error is non-nil, the
// backoff algorithm will sleep for an increasing amount of time, and
// then retry again later, until the maximum amount of sleep time has
// been consumed. Once function has executed successfully with no error,
// the backoff algorithm returns a nil error.
func Backoff(function TryFunc, options BackoffOptions) error {
	if options.MaxSleepTime <= 0 {
		panic("max sleep time must be > 0")
	}

	if options.InitialGapSize <= 0 {
		panic("initial gap size must be > 0")
	}

	if options.MaxJitterSize < 0 {
		panic("max jitter size must be >= 0")
	}

	if options.MaxJitterSize > (options.MaxSleepTime / 2) {
		panic("max jitter size is way too large")
	}

	if options.Sleeper == nil {
		options.Sleeper = time.Sleep
	}

	consumed := time.Duration(0)
	gap := options.InitialGapSize
	random := rand.New(rand.NewSource(options.RandomSeed))

	for consumed < options.MaxSleepTime {
		keepRetrying, err := function()
		if err != nil && !keepRetrying {
			return fmt.Errorf("exponential backoff instructed to stop retrying: %w", err)
		}

		// we can ignore keepRetrying at this point, since we know
		// what to do based on err
		if err == nil {
			return nil // success
		}

		// there was an error, and function wants to keep retrying
		// we will sleep, and then let the loop continue
		//
		// (random.Float64 returns a value [0.0, 1.0), which is used to
		// randomly scale the jitter from 0 to MaxJitterSize.
		jitter := nextJitter(random.Float64(), options.MaxJitterSize)
		duration := gap + jitter

		if (duration + consumed) > options.MaxSleepTime {
			// this will be our last try, force the duration
			// to line up with the maximum sleep time
			duration = options.MaxSleepTime - consumed
		}

		// sleep for the configured duration
		options.Sleeper(duration)

		// account for how long we intended to sleep
		consumed += duration

		// exponentially increase the gap
		gap *= 2
	}

	return ErrMaximumTimeExceeded
}

func nextJitter(fraction float64, maxSize time.Duration) time.Duration {
	scaled := fraction * float64(maxSize)
	return time.Duration(scaled)
}
