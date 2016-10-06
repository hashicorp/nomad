package dependency

import (
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"
)

func init() {
	gob.Register([]*HealthService{})
}

const (
	HealthAny      = "any"
	HealthPassing  = "passing"
	HealthWarning  = "warning"
	HealthUnknown  = "unknown"
	HealthCritical = "critical"
	HealthMaint    = "maintenance"

	NodeMaint    = "_node_maintenance"
	ServiceMaint = "_service_maintenance:"
)

// HealthService is a service entry in Consul.
type HealthService struct {
	Node        string
	NodeAddress string
	Address     string
	ID          string
	Name        string
	Tags        ServiceTags
	Checks      []*api.HealthCheck
	Status      string
	Port        uint64
}

// HealthServices is the struct that is formed from the dependency inside a
// template.
type HealthServices struct {
	sync.Mutex

	rawKey       string
	Name         string
	Tag          string
	DataCenter   string
	StatusFilter ServiceStatusFilter
	stopped      bool
	stopCh       chan struct{}
}

// Fetch queries the Consul API defined by the given client and returns a slice
// of HealthService objects.
func (d *HealthServices) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
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

	onlyHealthy := false
	if d.StatusFilter == nil {
		onlyHealthy = true
	}

	consul, err := clients.Consul()
	if err != nil {
		return nil, nil, fmt.Errorf("health services: error getting client: %s", err)
	}

	var entries []*api.ServiceEntry
	var qm *api.QueryMeta
	dataCh := make(chan struct{})
	go func() {
		log.Printf("[DEBUG] (%s) querying consul with %+v", d.Display(), consulOpts)
		entries, qm, err = consul.Health().Service(d.Name, d.Tag, onlyHealthy, consulOpts)
		close(dataCh)
	}()

	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	case <-dataCh:
	}

	if err != nil {
		return nil, nil, fmt.Errorf("health services: error fetching: %s", err)
	}

	log.Printf("[DEBUG] (%s) Consul returned %d services", d.Display(), len(entries))

	services := make([]*HealthService, 0, len(entries))

	for _, entry := range entries {
		// Get the status of this service from its checks.
		status, err := statusFromChecks(entry.Checks)
		if err != nil {
			return nil, nil, fmt.Errorf("health services: "+
				"error getting status from checks: %s", err)
		}

		// If we are not checking only healthy services, filter out services that do
		// not match the given filter.
		if d.StatusFilter != nil && !d.StatusFilter.Accept(status) {
			continue
		}

		// Sort the tags.
		tags := deepCopyAndSortTags(entry.Service.Tags)

		// Get the address of the service, falling back to the address of the node.
		var address string
		if entry.Service.Address != "" {
			address = entry.Service.Address
		} else {
			address = entry.Node.Address
		}

		services = append(services, &HealthService{
			Node:        entry.Node.Node,
			NodeAddress: entry.Node.Address,
			Address:     address,
			ID:          entry.Service.ID,
			Name:        entry.Service.Service,
			Tags:        tags,
			Status:      status,
			Checks:      entry.Checks,
			Port:        uint64(entry.Service.Port),
		})
	}

	log.Printf("[DEBUG] (%s) %d services after health check status filtering", d.Display(), len(services))

	sort.Stable(HealthServiceList(services))

	rm := &ResponseMetadata{
		LastIndex:   qm.LastIndex,
		LastContact: qm.LastContact,
	}

	return services, rm, nil
}

// CanShare returns a boolean if this dependency is shareable.
func (d *HealthServices) CanShare() bool {
	return true
}

// HashCode returns a unique identifier.
func (d *HealthServices) HashCode() string {
	return fmt.Sprintf("HealthServices|%s", d.rawKey)
}

// Display prints the human-friendly output.
func (d *HealthServices) Display() string {
	return fmt.Sprintf(`"service(%s)"`, d.rawKey)
}

// Stop halts the dependency's fetch function.
func (d *HealthServices) Stop() {
	d.Lock()
	defer d.Unlock()

	if !d.stopped {
		close(d.stopCh)
		d.stopped = true
	}
}

// ParseHealthServices processes the incoming strings to build a service dependency.
//
// Supported arguments
//   ParseHealthServices("service_id")
//   ParseHealthServices("service_id", "health_check")
//
// Where service_id is in the format of service(.tag(@datacenter))
// and health_check is either "any" or "passing".
//
// If no health_check is provided then its the same as "passing".
func ParseHealthServices(s ...string) (*HealthServices, error) {
	var query string
	var filter ServiceStatusFilter
	var err error

	switch len(s) {
	case 1:
		query = s[0]
		filter, err = NewServiceStatusFilter("")
		if err != nil {
			return nil, err
		}
	case 2:
		query = s[0]
		filter, err = NewServiceStatusFilter(s[1])
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("expected 1 or 2 arguments, got %d", len(s))
	}

	if len(query) == 0 {
		return nil, errors.New("cannot specify empty health service dependency")
	}

	re := regexp.MustCompile(`\A` +
		`((?P<tag>[[:word:]\-.]+)\.)?` +
		`((?P<name>[[:word:]\-/_]+))` +
		`(@(?P<datacenter>[[:word:]\.\-]+))?(:(?P<port>[0-9]+))?` +
		`\z`)
	names := re.SubexpNames()
	match := re.FindAllStringSubmatch(query, -1)

	if len(match) == 0 {
		return nil, errors.New("invalid health service dependency format")
	}

	r := match[0]

	m := map[string]string{}
	for i, n := range r {
		if names[i] != "" {
			m[names[i]] = n
		}
	}

	tag, name, datacenter, port := m["tag"], m["name"], m["datacenter"], m["port"]

	if name == "" {
		return nil, errors.New("name part is required")
	}

	if port != "" {
		log.Printf("[WARN] specifying a port in a 'service' query is not "+
			"supported - please remove the port from the query %q", query)
	}

	var key string
	if filter == nil {
		key = query
	} else {
		key = fmt.Sprintf("%s %s", query, filter)
	}

	sd := &HealthServices{
		rawKey:       key,
		Name:         name,
		Tag:          tag,
		DataCenter:   datacenter,
		StatusFilter: filter,
		stopCh:       make(chan struct{}),
	}

	return sd, nil
}

// statusFromChecks accepts a list of checks and returns the most likely status
// given those checks. Any "critical" statuses will automatically mark the
// service as critical. After that, any "unknown" statuses will mark as
// "unknown". If any warning checks exist, the status will be marked as
// "warning", and finally "passing". If there are no checks, the service will be
// marked as "passing".
func statusFromChecks(checks []*api.HealthCheck) (string, error) {
	var passing, warning, unknown, critical, maintenance bool
	for _, check := range checks {
		if check.CheckID == NodeMaint || strings.HasPrefix(check.CheckID, ServiceMaint) {
			maintenance = true
			continue
		}

		switch check.Status {
		case "passing":
			passing = true
		case "warning":
			warning = true
		case "unknown":
			unknown = true
		case "critical":
			critical = true
		default:
			return "", fmt.Errorf("unknown status: %q", check.Status)
		}
	}

	switch {
	case maintenance:
		return HealthMaint, nil
	case critical:
		return HealthCritical, nil
	case unknown:
		return HealthUnknown, nil
	case warning:
		return HealthWarning, nil
	case passing:
		return HealthPassing, nil
	default:
		// No checks?
		return HealthPassing, nil
	}
}

// ServiceStatusFilter is used to specify a list of service statuses that you want filter by.
type ServiceStatusFilter []string

// String returns the string representation of this status filter
func (f ServiceStatusFilter) String() string {
	return fmt.Sprintf("[%s]", strings.Join(f, ","))
}

// NewServiceStatusFilter creates a status filter from the given string in the
// format `[key[,key[,key...]]]`. Each status is split on the comma character
// and must match one of the valid status names.
//
// If the empty string is given, it is assumed only "passing" statuses are to
// be returned.
//
// If the user specifies "any" with other keys, an error will be returned.
func NewServiceStatusFilter(s string) (ServiceStatusFilter, error) {
	// If no statuses were given, use the default status of "all passing".
	if len(s) == 0 || len(strings.TrimSpace(s)) == 0 {
		return nil, nil
	}

	var errs *multierror.Error
	var hasAny bool

	raw := strings.Split(s, ",")
	trimmed := make(ServiceStatusFilter, 0, len(raw))
	for _, r := range raw {
		trim := strings.TrimSpace(r)

		// Ignore the empty string.
		if len(trim) == 0 {
			continue
		}

		// Record the case where we have the "any" status - it will be used later.
		if trim == HealthAny {
			hasAny = true
		}

		// Validate that the service is actually a valid name.
		switch trim {
		case HealthAny, HealthUnknown, HealthPassing, HealthWarning, HealthCritical, HealthMaint:
			trimmed = append(trimmed, trim)
		default:
			errs = multierror.Append(errs, fmt.Errorf("service filter: invalid filter %q", trim))
		}
	}

	// If the user specified "any" with additional keys, that is invalid.
	if hasAny && len(trimmed) != 1 {
		errs = multierror.Append(errs, fmt.Errorf("service filter: cannot specify extra keys when using %q", "any"))
	}

	return trimmed, errs.ErrorOrNil()
}

// Accept allows us to check if a slice of health checks pass this filter.
func (f ServiceStatusFilter) Accept(s string) bool {
	// If the any filter is activated, pass everything.
	if f.any() {
		return true
	}

	// Iterate over each status and see if the given status is any of those
	// statuses.
	for _, status := range f {
		if status == s {
			return true
		}
	}

	return false
}

// any is a helper method to determine if this is an "any" service status
// filter. If "any" was given, it must be the only item in the list.
func (f ServiceStatusFilter) any() bool {
	return len(f) == 1 && f[0] == HealthAny
}

// HealthServiceList is a sortable slice of Service
type HealthServiceList []*HealthService

// Len, Swap, and Less are used to implement the sort.Sort interface.
func (s HealthServiceList) Len() int      { return len(s) }
func (s HealthServiceList) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s HealthServiceList) Less(i, j int) bool {
	if s[i].Node < s[j].Node {
		return true
	} else if s[i].Node == s[j].Node {
		return s[i].ID <= s[j].ID
	}
	return false
}
