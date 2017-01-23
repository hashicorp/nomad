package dependency

import (
	"log"
	"net/url"
	"sort"
	"time"

	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ Dependency = (*CatalogDatacentersQuery)(nil)

	// CatalogDatacentersQuerySleepTime is the amount of time to sleep between
	// queries, since the endpoint does not support blocking queries.
	CatalogDatacentersQuerySleepTime = 15 * time.Second
)

// CatalogDatacentersQuery is the dependency to query all datacenters
type CatalogDatacentersQuery struct {
	stopCh chan struct{}
}

// NewCatalogDatacentersQuery creates a new datacenter dependency.
func NewCatalogDatacentersQuery() (*CatalogDatacentersQuery, error) {
	return &CatalogDatacentersQuery{
		stopCh: make(chan struct{}, 1),
	}, nil
}

// Fetch queries the Consul API defined by the given client and returns a slice
// of strings representing the datacenters
func (d *CatalogDatacentersQuery) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
	opts = opts.Merge(&QueryOptions{})

	log.Printf("[TRACE] %s: GET %s", d, &url.URL{
		Path:     "/v1/catalog/datacenters",
		RawQuery: opts.String(),
	})

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
		log.Printf("[TRACE] %s: long polling for %s", d, CatalogDatacentersQuerySleepTime)

		select {
		case <-d.stopCh:
			return nil, nil, ErrStopped
		case <-time.After(CatalogDatacentersQuerySleepTime):
		}
	}

	result, err := clients.Consul().Catalog().Datacenters()
	if err != nil {
		return nil, nil, errors.Wrapf(err, d.String())
	}

	log.Printf("[TRACE] %s: returned %d results", d, len(result))

	sort.Strings(result)

	return respWithMetadata(result)
}

// CanShare returns if this dependency is shareable.
func (d *CatalogDatacentersQuery) CanShare() bool {
	return true
}

// String returns the human-friendly version of this dependency.
func (d *CatalogDatacentersQuery) String() string {
	return "catalog.datacenters"
}

// Stop terminates this dependency's fetch.
func (d *CatalogDatacentersQuery) Stop() {
	close(d.stopCh)
}

// Type returns the type of this dependency.
func (d *CatalogDatacentersQuery) Type() Type {
	return TypeConsul
}
