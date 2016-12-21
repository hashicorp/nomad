package dependency

import (
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"sync"

	"github.com/hashicorp/consul/api"
)

func init() {
	gob.Register([]*CatalogService{})
}

// CatalogService is a catalog entry in Consul.
type CatalogService struct {
	Name string
	Tags ServiceTags
}

// CatalogServices is the representation of a requested catalog service
// dependency from inside a template.
type CatalogServices struct {
	sync.Mutex

	rawKey     string
	Name       string
	Tags       []string
	DataCenter string
	stopped    bool
	stopCh     chan struct{}
}

// Fetch queries the Consul API defined by the given client and returns a slice
// of CatalogService objects.
func (d *CatalogServices) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
	d.Lock()
	if d.stopped {
		defer d.Unlock()
		return nil, nil, ErrStopped
	}
	d.Unlock()

	if opts == nil {
		opts = &QueryOptions{}
	}

	consulOpts := opts.consulQueryOptions()
	if d.DataCenter != "" {
		consulOpts.Datacenter = d.DataCenter
	}

	consul, err := clients.Consul()
	if err != nil {
		return nil, nil, fmt.Errorf("catalog services: error getting client: %s", err)
	}

	var entries map[string][]string
	var qm *api.QueryMeta
	dataCh := make(chan struct{})
	go func() {
		log.Printf("[DEBUG] (%s) querying Consul with %+v", d.Display(), consulOpts)
		entries, qm, err = consul.Catalog().Services(consulOpts)
		close(dataCh)
	}()

	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	case <-dataCh:
	}

	if err != nil {
		return nil, nil, fmt.Errorf("catalog services: error fetching: %s", err)
	}

	log.Printf("[DEBUG] (%s) Consul returned %d catalog services", d.Display(), len(entries))

	var catalogServices []*CatalogService
	for name, tags := range entries {
		tags = deepCopyAndSortTags(tags)
		catalogServices = append(catalogServices, &CatalogService{
			Name: name,
			Tags: ServiceTags(tags),
		})
	}

	sort.Stable(CatalogServicesList(catalogServices))

	rm := &ResponseMetadata{
		LastIndex:   qm.LastIndex,
		LastContact: qm.LastContact,
	}

	return catalogServices, rm, nil
}

// CanShare returns a boolean if this dependency is shareable.
func (d *CatalogServices) CanShare() bool {
	return true
}

// HashCode returns a unique identifier.
func (d *CatalogServices) HashCode() string {
	return fmt.Sprintf("CatalogServices|%s", d.rawKey)
}

// Display prints the human-friendly output.
func (d *CatalogServices) Display() string {
	if d.rawKey == "" {
		return fmt.Sprintf(`"services"`)
	}

	return fmt.Sprintf(`"services(%s)"`, d.rawKey)
}

// Stop halts the dependency's fetch function.
func (d *CatalogServices) Stop() {
	d.Lock()
	defer d.Unlock()

	if !d.stopped {
		close(d.stopCh)
		d.stopped = true
	}
}

// ParseCatalogServices parses a string of the format @dc.
func ParseCatalogServices(s ...string) (*CatalogServices, error) {
	switch len(s) {
	case 0:
		cs := &CatalogServices{
			rawKey: "",
			stopCh: make(chan struct{}),
		}
		return cs, nil
	case 1:
		dc := s[0]

		re := regexp.MustCompile(`\A` +
			`(@(?P<datacenter>[[:word:]\.\-]+))?` +
			`\z`)
		names := re.SubexpNames()
		match := re.FindAllStringSubmatch(dc, -1)

		if len(match) == 0 {
			return nil, errors.New("invalid catalog service dependency format")
		}

		r := match[0]

		m := map[string]string{}
		for i, n := range r {
			if names[i] != "" {
				m[names[i]] = n
			}
		}

		nd := &CatalogServices{
			rawKey:     dc,
			DataCenter: m["datacenter"],
			stopCh:     make(chan struct{}),
		}

		return nd, nil
	default:
		return nil, fmt.Errorf("expected 0 or 1 arguments, got %d", len(s))
	}
}

/// --- Sorting

// CatalogServicesList is a sortable slice of CatalogService structs.
type CatalogServicesList []*CatalogService

func (s CatalogServicesList) Len() int      { return len(s) }
func (s CatalogServicesList) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s CatalogServicesList) Less(i, j int) bool {
	if s[i].Name <= s[j].Name {
		return true
	}
	return false
}
