package dependency

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

var sleepTime = 15 * time.Second

// Datacenters is the dependency to query all datacenters
type Datacenters struct {
	sync.Mutex

	rawKey string

	stopped bool
	stopCh  chan struct{}
}

// Fetch queries the Consul API defined by the given client and returns a slice
// of strings representing the datacenters
func (d *Datacenters) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
	d.Lock()
	if d.stopped {
		defer d.Unlock()
		return nil, nil, ErrStopped
	}
	d.Unlock()

	if opts == nil {
		opts = &QueryOptions{}
	}

	log.Printf("[DEBUG] (%s) querying Consul with %+v", d.Display(), opts)

	// This is pretty ghetto, but the datacenters endpoint does not support
	// blocking queries, so we are going to "fake it until we make it". When we
	// first query, the LastIndex will be "0", meaning we should immediately
	// return data, but future calls will include a LastIndex. If we have a
	// LastIndex in the query metadata, sleep for 15 seconds before asking Consul
	// again.
	//
	// This is probably okay given the frequency in which datacenters actually
	// change, but is technically not edge-triggering.
	if opts.WaitIndex != 0 {
		log.Printf("[DEBUG] (%s) pretending to long-poll", d.Display())
		select {
		case <-d.stopCh:
			log.Printf("[DEBUG] (%s) received interrupt", d.Display())
			return nil, nil, ErrStopped
		case <-time.After(sleepTime):
		}
	}

	consul, err := clients.Consul()
	if err != nil {
		return nil, nil, fmt.Errorf("datacenters: error getting client: %s", err)
	}

	catalog := consul.Catalog()
	result, err := catalog.Datacenters()
	if err != nil {
		return nil, nil, fmt.Errorf("datacenters: error fetching: %s", err)
	}

	log.Printf("[DEBUG] (%s) Consul returned %d datacenters", d.Display(), len(result))
	sort.Strings(result)

	return respWithMetadata(result)
}

// CanShare returns if this dependency is shareable.
func (d *Datacenters) CanShare() bool {
	return true
}

// HashCode returns the hash code for this dependency.
func (d *Datacenters) HashCode() string {
	return fmt.Sprintf("Datacenters|%s", d.rawKey)
}

// Display returns a string that should be displayed to the user in output (for
// example).
func (d *Datacenters) Display() string {
	if d.rawKey == "" {
		return fmt.Sprintf(`"datacenters"`)
	}

	return fmt.Sprintf(`"datacenters(%s)"`, d.rawKey)
}

// Stop terminates this dependency's execution early.
func (d *Datacenters) Stop() {
	d.Lock()
	defer d.Unlock()

	if !d.stopped {
		close(d.stopCh)
		d.stopped = true
	}
}

// ParseDatacenters creates a new datacenter dependency.
func ParseDatacenters(s ...string) (*Datacenters, error) {
	switch len(s) {
	case 0:
		dcs := &Datacenters{
			rawKey: "",
			stopCh: make(chan struct{}, 0),
		}
		return dcs, nil
	default:
		return nil, fmt.Errorf("expected 0 arguments, got %d", len(s))
	}
}
