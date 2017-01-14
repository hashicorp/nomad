package dependency

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

var (
	// Ensure implements
	_ Dependency = (*EnvQuery)(nil)

	// EnvQuerySleepTime is the amount of time to sleep between queries. Since
	// it's not supporting to change a running processes' environment, this can
	// be a fairly large value.
	EnvQuerySleepTime = 5 * time.Minute
)

// EnvQuery represents a local file dependency.
type EnvQuery struct {
	stopCh chan struct{}

	key  string
	stat os.FileInfo
}

// NewEnvQuery creates a file dependency from the given key.
func NewEnvQuery(s string) (*EnvQuery, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("env: invalid format: %q", s)
	}

	return &EnvQuery{
		key:    s,
		stopCh: make(chan struct{}, 1),
	}, nil
}

// Fetch retrieves this dependency and returns the result or any errors that
// occur in the process.
func (d *EnvQuery) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
	opts = opts.Merge(&QueryOptions{})

	log.Printf("[TRACE] %s: ENV %s", d, d.key)

	if opts.WaitIndex != 0 {
		log.Printf("[TRACE] %s: long polling for %s", d, EnvQuerySleepTime)

		select {
		case <-d.stopCh:
			return nil, nil, ErrStopped
		case <-time.After(EnvQuerySleepTime):
		}
	}

	result := os.Getenv(d.key)

	log.Printf("[TRACE] %s: returned result", d)

	return respWithMetadata(result)
}

// CanShare returns a boolean if this dependency is shareable.
func (d *EnvQuery) CanShare() bool {
	return false
}

// Stop halts the dependency's fetch function.
func (d *EnvQuery) Stop() {
	close(d.stopCh)
}

// String returns the human-friendly version of this dependency.
func (d *EnvQuery) String() string {
	return fmt.Sprintf("env(%s)", d.key)
}

// Type returns the type of this dependency.
func (d *EnvQuery) Type() Type {
	return TypeLocal
}
