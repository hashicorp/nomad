package watch

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Wait is the Min/Max duration used by the Watcher
type Wait struct {
	// Min and Max are the minimum and maximum time, respectively, to wait for
	// data changes before rendering a new template to disk.
	Min time.Duration `json:"min" mapstructure:"min"`
	Max time.Duration `json:"max" mapstructure:"max"`
}

// ParseWait parses a string of the format `minimum(:maximum)` into a Wait
// struct.
func ParseWait(s string) (*Wait, error) {
	if len(strings.TrimSpace(s)) < 1 {
		return nil, errors.New("cannot specify empty wait interval")
	}

	parts := strings.Split(s, ":")

	var min, max time.Duration
	var err error

	if len(parts) == 1 {
		min, err = time.ParseDuration(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, err
		}

		max = 4 * min
	} else if len(parts) == 2 {
		min, err = time.ParseDuration(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, err
		}

		max, err = time.ParseDuration(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("invalid wait interval format")
	}

	if min < 0 || max < 0 {
		return nil, errors.New("cannot specify a negative wait interval")
	}

	if max < min {
		return nil, errors.New("wait interval max must be larger than min")
	}

	return &Wait{min, max}, nil
}

// IsActive returns true if this wait is active (non-zero).
func (w *Wait) IsActive() bool {
	return w.Min != 0 && w.Max != 0
}

// WaitVar implements the Flag.Value interface and allows the user to specify
// a watch interval using Go's flag parsing library.
type WaitVar Wait

// Set sets the value in the format min[:max] for a wait timer.
func (w *WaitVar) Set(value string) error {
	wait, err := ParseWait(value)
	if err != nil {
		return err
	}

	w.Min = wait.Min
	w.Max = wait.Max

	return nil
}

// String returns the string format for this wait variable
func (w *WaitVar) String() string {
	return fmt.Sprintf("%s:%s", w.Min, w.Max)
}
