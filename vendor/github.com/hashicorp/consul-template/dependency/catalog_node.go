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
	gob.Register([]*NodeDetail{})
	gob.Register([]*NodeService{})
}

// NodeDetail is a wrapper around the node and its services.
type NodeDetail struct {
	Node     *Node
	Services NodeServiceList
}

// NodeService is a service on a single node.
type NodeService struct {
	ID                string
	Service           string
	Tags              ServiceTags
	Port              int
	Address           string
	EnableTagOverride bool
}

// CatalogNode represents a single node from the Consul catalog.
type CatalogNode struct {
	sync.Mutex

	rawKey     string
	dataCenter string
	stopped    bool
	stopCh     chan struct{}
}

// Fetch queries the Consul API defined by the given client and returns a
// of NodeDetail object.
func (d *CatalogNode) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
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
	if d.dataCenter != "" {
		consulOpts.Datacenter = d.dataCenter
	}

	consul, err := clients.Consul()
	if err != nil {
		return nil, nil, fmt.Errorf("catalog node: error getting client: %s", err)
	}

	nodeName := d.rawKey
	if nodeName == "" {
		log.Printf("[DEBUG] (%s) getting local agent name", d.Display())
		nodeName, err = consul.Agent().NodeName()
		if err != nil {
			return nil, nil, fmt.Errorf("catalog node: error getting local agent: %s", err)
		}
	}

	var n *api.CatalogNode
	var qm *api.QueryMeta
	dataCh := make(chan struct{})
	go func() {
		log.Printf("[DEBUG] (%s) querying consul with %+v", d.Display(), consulOpts)
		n, qm, err = consul.Catalog().Node(nodeName, consulOpts)
		close(dataCh)
	}()

	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	case <-dataCh:
	}

	if err != nil {
		return nil, nil, fmt.Errorf("catalog node: error fetching: %s", err)
	}

	rm := &ResponseMetadata{
		LastIndex:   qm.LastIndex,
		LastContact: qm.LastContact,
	}

	if n == nil {
		log.Printf("[WARN] (%s) could not find node by that name", d.Display())
		var node *NodeDetail
		return node, rm, nil
	}

	services := make(NodeServiceList, 0, len(n.Services))
	for _, v := range n.Services {
		services = append(services, &NodeService{
			ID:                v.ID,
			Service:           v.Service,
			Tags:              ServiceTags(deepCopyAndSortTags(v.Tags)),
			Port:              v.Port,
			Address:           v.Address,
			EnableTagOverride: v.EnableTagOverride,
		})
	}
	sort.Stable(services)

	node := &NodeDetail{
		Node: &Node{
			Node:            n.Node.Node,
			Address:         n.Node.Address,
			TaggedAddresses: n.Node.TaggedAddresses,
		},
		Services: services,
	}

	return node, rm, nil
}

// CanShare returns a boolean if this dependency is shareable.
func (d *CatalogNode) CanShare() bool {
	return false
}

// HashCode returns a unique identifier.
func (d *CatalogNode) HashCode() string {
	if d.dataCenter != "" {
		return fmt.Sprintf("NodeDetail|%s@%s", d.rawKey, d.dataCenter)
	}
	return fmt.Sprintf("NodeDetail|%s", d.rawKey)
}

// Display prints the human-friendly output.
func (d *CatalogNode) Display() string {
	if d.dataCenter != "" {
		return fmt.Sprintf("node(%s@%s)", d.rawKey, d.dataCenter)
	}
	return fmt.Sprintf(`"node(%s)"`, d.rawKey)
}

// Stop halts the dependency's fetch function.
func (d *CatalogNode) Stop() {
	d.Lock()
	defer d.Unlock()

	if !d.stopped {
		close(d.stopCh)
		d.stopped = true
	}
}

// ParseCatalogNode parses a name name and optional datacenter value.
// If the name is empty or not provided then the current agent is used.
func ParseCatalogNode(s ...string) (*CatalogNode, error) {
	switch len(s) {
	case 0:
		cn := &CatalogNode{stopCh: make(chan struct{})}
		return cn, nil
	case 1:
		cn := &CatalogNode{
			rawKey: s[0],
			stopCh: make(chan struct{}),
		}
		return cn, nil
	case 2:
		dc := s[1]

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

		nd := &CatalogNode{
			rawKey:     s[0],
			dataCenter: m["datacenter"],
			stopCh:     make(chan struct{}),
		}

		return nd, nil
	default:
		return nil, fmt.Errorf("expected 0, 1, or 2 arguments, got %d", len(s))
	}
}

// Sorting

// NodeServiceList is a sortable list of node service names.
type NodeServiceList []*NodeService

func (s NodeServiceList) Len() int      { return len(s) }
func (s NodeServiceList) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s NodeServiceList) Less(i, j int) bool {
	if s[i].Service == s[j].Service {
		return s[i].ID <= s[j].ID
	}
	return s[i].Service <= s[j].Service
}
