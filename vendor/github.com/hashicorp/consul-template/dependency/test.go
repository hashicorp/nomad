package dependency

import (
	"fmt"
	"sync"
	"time"
)

// Test is a special dependency that does not actually speaks to a server.
type Test struct {
	Name string
}

func (d *Test) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
	time.Sleep(10 * time.Millisecond)
	data := "this is some data"
	rm := &ResponseMetadata{LastIndex: 1}
	return data, rm, nil
}

func (d *Test) CanShare() bool {
	return true
}

func (d *Test) HashCode() string {
	return fmt.Sprintf("Test|%s", d.Name)
}

func (d *Test) Display() string { return "fakedep" }

func (d *Test) Stop() {}

// TestStale is a special dependency that can be used to test what happens when
// stale data is permitted.
type TestStale struct {
	Name string
}

// Fetch is used to implement the dependency interface.
func (d *TestStale) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
	time.Sleep(10 * time.Millisecond)

	if opts == nil {
		opts = &QueryOptions{}
	}

	if opts.AllowStale {
		data := "this is some stale data"
		rm := &ResponseMetadata{LastIndex: 1, LastContact: 50 * time.Millisecond}
		return data, rm, nil
	} else {
		data := "this is some fresh data"
		rm := &ResponseMetadata{LastIndex: 1}
		return data, rm, nil
	}
}

func (d *TestStale) CanShare() bool {
	return true
}

func (d *TestStale) HashCode() string {
	return fmt.Sprintf("TestStale|%s", d.Name)
}

func (d *TestStale) Display() string { return "fakedep" }

func (d *TestStale) Stop() {}

// TestFetchError is a special dependency that returns an error while fetching.
type TestFetchError struct {
	Name string
}

func (d *TestFetchError) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
	time.Sleep(10 * time.Millisecond)
	return nil, nil, fmt.Errorf("failed to contact server")
}

func (d *TestFetchError) CanShare() bool {
	return true
}

func (d *TestFetchError) HashCode() string {
	return fmt.Sprintf("TestFetchError|%s", d.Name)
}

func (d *TestFetchError) Display() string { return "fakedep" }

func (d *TestFetchError) Stop() {}

// TestRetry is a special dependency that errors on the first fetch and
// succeeds on subsequent fetches.
type TestRetry struct {
	sync.Mutex
	Name    string
	retried bool
}

func (d *TestRetry) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
	time.Sleep(10 * time.Millisecond)

	d.Lock()
	defer d.Unlock()

	if d.retried {
		data := "this is some data"
		rm := &ResponseMetadata{LastIndex: 1}
		return data, rm, nil
	} else {
		d.retried = true
		return nil, nil, fmt.Errorf("failed to contact server (try again)")
	}
}

func (d *TestRetry) CanShare() bool {
	return true
}

func (d *TestRetry) HashCode() string {
	return fmt.Sprintf("TestRetry|%s", d.Name)
}

func (d *TestRetry) Display() string { return "fakedep" }

func (d *TestRetry) Stop() {}
