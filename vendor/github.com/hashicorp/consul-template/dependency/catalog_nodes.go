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
	gob.Register([]*Node{})
}

// Node is a node entry in Consul
type Node struct {
	Node            string
	Address         string
	TaggedAddresses map[string]string
}

// CatalogNodes is the representation of all registered nodes in Consul.
type CatalogNodes struct {
	sync.Mutex

	rawKey     string
	DataCenter string
	stopped    bool
	stopCh     chan struct{}
}

// Fetch queries the Consul API defined by the given client and returns a slice
// of Node objects
func (d *CatalogNodes) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
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
		return nil, nil, fmt.Errorf("catalog nodes: error getting client: %s", err)
	}

	var n []*api.Node
	var qm *api.QueryMeta
	dataCh := make(chan struct{})
	go func() {
		log.Printf("[DEBUG] (%s) querying Consul with %+v", d.Display(), consulOpts)
		n, qm, err = consul.Catalog().Nodes(consulOpts)
		close(dataCh)
	}()

	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	case <-dataCh:
	}

	if err != nil {
		return nil, nil, fmt.Errorf("catalog nodes: error fetching: %s", err)
	}

	log.Printf("[DEBUG] (%s) Consul returned %d nodes", d.Display(), len(n))

	nodes := make([]*Node, 0, len(n))
	for _, node := range n {
		nodes = append(nodes, &Node{
			Node:            node.Node,
			Address:         node.Address,
			TaggedAddresses: node.TaggedAddresses,
		})
	}
	sort.Stable(NodeList(nodes))

	rm := &ResponseMetadata{
		LastIndex:   qm.LastIndex,
		LastContact: qm.LastContact,
	}

	return nodes, rm, nil
}

// CanShare returns a boolean if this dependency is shareable.
func (d *CatalogNodes) CanShare() bool {
	return true
}

// HashCode returns a unique identifier.
func (d *CatalogNodes) HashCode() string {
	return fmt.Sprintf("CatalogNodes|%s", d.rawKey)
}

// Display prints the human-friendly output.
func (d *CatalogNodes) Display() string {
	if d.rawKey == "" {
		return fmt.Sprintf(`"nodes"`)
	}

	return fmt.Sprintf(`"nodes(%s)"`, d.rawKey)
}

// Stop halts the dependency's fetch function.
func (d *CatalogNodes) Stop() {
	d.Lock()
	defer d.Unlock()

	if !d.stopped {
		close(d.stopCh)
		d.stopped = true
	}
}

// ParseCatalogNodes parses a string of the format @dc.
func ParseCatalogNodes(s ...string) (*CatalogNodes, error) {
	switch len(s) {
	case 0:
		cn := &CatalogNodes{
			rawKey: "",
			stopCh: make(chan struct{}),
		}
		return cn, nil
	case 1:
		dc := s[0]

		re := regexp.MustCompile(`\A` +
			`(@(?P<datacenter>[[:word:]\.\-]+))?` +
			`\z`)
		names := re.SubexpNames()
		match := re.FindAllStringSubmatch(dc, -1)

		if len(match) == 0 {
			return nil, errors.New("invalid node dependency format")
		}

		r := match[0]

		m := map[string]string{}
		for i, n := range r {
			if names[i] != "" {
				m[names[i]] = n
			}
		}

		cn := &CatalogNodes{
			rawKey:     dc,
			DataCenter: m["datacenter"],
			stopCh:     make(chan struct{}),
		}

		return cn, nil
	default:
		return nil, fmt.Errorf("expected 0 or 1 arguments, got %d", len(s))
	}
}

// NodeList is a sortable list of node objects by name and then IP address.
type NodeList []*Node

func (s NodeList) Len() int      { return len(s) }
func (s NodeList) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s NodeList) Less(i, j int) bool {
	if s[i].Node == s[j].Node {
		return s[i].Address <= s[j].Address
	}
	return s[i].Node <= s[j].Node
}
