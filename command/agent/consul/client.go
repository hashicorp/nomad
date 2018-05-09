package consul

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/api"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// nomadServicePrefix is the prefix that scopes all Nomad registered
	// services (both agent and task entries).
	nomadServicePrefix = "_nomad"

	// nomadTaskPrefix is the prefix that scopes Nomad registered services
	// for tasks.
	nomadTaskPrefix = nomadServicePrefix + "-task-"

	// defaultRetryInterval is how quickly to retry syncing services and
	// checks to Consul when an error occurs. Will backoff up to a max.
	defaultRetryInterval = time.Second

	// defaultMaxRetryInterval is the default max retry interval.
	defaultMaxRetryInterval = 30 * time.Second

	// defaultPeriodicalInterval is the interval at which the service
	// client reconciles state between the desired services and checks and
	// what's actually registered in Consul. This is done at an interval,
	// rather than being purely edge triggered, to handle the case that the
	// Consul agent's state may change underneath us
	defaultPeriodicInterval = 30 * time.Second

	// ttlCheckBuffer is the time interval that Nomad can take to report Consul
	// the check result
	ttlCheckBuffer = 31 * time.Second

	// defaultShutdownWait is how long Shutdown() should block waiting for
	// enqueued operations to sync to Consul by default.
	defaultShutdownWait = time.Minute

	// DefaultQueryWaitDuration is the max duration the Consul Agent will
	// spend waiting for a response from a Consul Query.
	DefaultQueryWaitDuration = 2 * time.Second

	// ServiceTagHTTP is the tag assigned to HTTP services
	ServiceTagHTTP = "http"

	// ServiceTagRPC is the tag assigned to RPC services
	ServiceTagRPC = "rpc"

	// ServiceTagSerf is the tag assigned to Serf services
	ServiceTagSerf = "serf"
)

// CatalogAPI is the consul/api.Catalog API used by Nomad.
type CatalogAPI interface {
	Datacenters() ([]string, error)
	Service(service, tag string, q *api.QueryOptions) ([]*api.CatalogService, *api.QueryMeta, error)
}

// AgentAPI is the consul/api.Agent API used by Nomad.
type AgentAPI interface {
	Services() (map[string]*api.AgentService, error)
	Checks() (map[string]*api.AgentCheck, error)
	CheckRegister(check *api.AgentCheckRegistration) error
	CheckDeregister(checkID string) error
	Self() (map[string]map[string]interface{}, error)
	ServiceRegister(service *api.AgentServiceRegistration) error
	ServiceDeregister(serviceID string) error
	UpdateTTL(id, output, status string) error
}

// operations are submitted to the main loop via commit() for synchronizing
// with Consul.
type operations struct {
	regServices []*api.AgentServiceRegistration
	regChecks   []*api.AgentCheckRegistration
	scripts     []*scriptCheck

	deregServices []string
	deregChecks   []string
}

// AllocRegistration holds the status of services registered for a particular
// allocations by task.
type AllocRegistration struct {
	// Tasks maps the name of a task to its registered services and checks
	Tasks map[string]*TaskRegistration
}

func (a *AllocRegistration) copy() *AllocRegistration {
	c := &AllocRegistration{
		Tasks: make(map[string]*TaskRegistration, len(a.Tasks)),
	}

	for k, v := range a.Tasks {
		c.Tasks[k] = v.copy()
	}

	return c
}

// NumServices returns the number of registered services
func (a *AllocRegistration) NumServices() int {
	if a == nil {
		return 0
	}

	total := 0
	for _, treg := range a.Tasks {
		for _, sreg := range treg.Services {
			if sreg.Service != nil {
				total++
			}
		}
	}

	return total
}

// NumChecks returns the number of registered checks
func (a *AllocRegistration) NumChecks() int {
	if a == nil {
		return 0
	}

	total := 0
	for _, treg := range a.Tasks {
		for _, sreg := range treg.Services {
			total += len(sreg.Checks)
		}
	}

	return total
}

// TaskRegistration holds the status of services registered for a particular
// task.
type TaskRegistration struct {
	Services map[string]*ServiceRegistration
}

func (t *TaskRegistration) copy() *TaskRegistration {
	c := &TaskRegistration{
		Services: make(map[string]*ServiceRegistration, len(t.Services)),
	}

	for k, v := range t.Services {
		c.Services[k] = v.copy()
	}

	return c
}

// ServiceRegistration holds the status of a registered Consul Service and its
// Checks.
type ServiceRegistration struct {
	// serviceID and checkIDs are internal fields that track just the IDs of the
	// services/checks registered in Consul. It is used to materialize the other
	// fields when queried.
	serviceID string
	checkIDs  map[string]struct{}

	// Service is the AgentService registered in Consul.
	Service *api.AgentService

	// Checks is the status of the registered checks.
	Checks []*api.AgentCheck
}

func (s *ServiceRegistration) copy() *ServiceRegistration {
	// Copy does not copy the external fields but only the internal fields. This
	// is so that the caller of AllocRegistrations can not access the internal
	// fields and that method uses these fields to populate the external fields.
	return &ServiceRegistration{
		serviceID: s.serviceID,
		checkIDs:  helper.CopyMapStringStruct(s.checkIDs),
	}
}

// ServiceClient handles task and agent service registration with Consul.
type ServiceClient struct {
	client           AgentAPI
	logger           *log.Logger
	retryInterval    time.Duration
	maxRetryInterval time.Duration
	periodicInterval time.Duration

	// exitCh is closed when the main Run loop exits
	exitCh chan struct{}

	// shutdownCh is closed when the client should shutdown
	shutdownCh chan struct{}

	// shutdownWait is how long Shutdown() blocks waiting for the final
	// sync() to finish. Defaults to defaultShutdownWait
	shutdownWait time.Duration

	opCh chan *operations

	services       map[string]*api.AgentServiceRegistration
	checks         map[string]*api.AgentCheckRegistration
	scripts        map[string]*scriptCheck
	runningScripts map[string]*scriptHandle

	// allocRegistrations stores the services and checks that are registered
	// with Consul by allocation ID.
	allocRegistrations     map[string]*AllocRegistration
	allocRegistrationsLock sync.RWMutex

	// agent services and checks record entries for the agent itself which
	// should be removed on shutdown
	agentServices map[string]struct{}
	agentChecks   map[string]struct{}
	agentLock     sync.Mutex

	// seen is 1 if Consul has ever been seen; otherwise 0. Accessed with
	// atomics.
	seen int32

	// checkWatcher restarts checks that are unhealthy.
	checkWatcher *checkWatcher
}

// NewServiceClient creates a new Consul ServiceClient from an existing Consul API
// Client and logger.
func NewServiceClient(consulClient AgentAPI, logger *log.Logger) *ServiceClient {
	return &ServiceClient{
		client:             consulClient,
		logger:             logger,
		retryInterval:      defaultRetryInterval,
		maxRetryInterval:   defaultMaxRetryInterval,
		periodicInterval:   defaultPeriodicInterval,
		exitCh:             make(chan struct{}),
		shutdownCh:         make(chan struct{}),
		shutdownWait:       defaultShutdownWait,
		opCh:               make(chan *operations, 8),
		services:           make(map[string]*api.AgentServiceRegistration),
		checks:             make(map[string]*api.AgentCheckRegistration),
		scripts:            make(map[string]*scriptCheck),
		runningScripts:     make(map[string]*scriptHandle),
		allocRegistrations: make(map[string]*AllocRegistration),
		agentServices:      make(map[string]struct{}),
		agentChecks:        make(map[string]struct{}),
		checkWatcher:       newCheckWatcher(logger, consulClient),
	}
}

// seen is used by markSeen and hasSeen
const seen = 1

// markSeen marks Consul as having been seen (meaning at least one operation
// has succeeded).
func (c *ServiceClient) markSeen() {
	atomic.StoreInt32(&c.seen, seen)
}

// hasSeen returns true if any Consul operation has ever succeeded. Useful to
// squelch errors if Consul isn't running.
func (c *ServiceClient) hasSeen() bool {
	return atomic.LoadInt32(&c.seen) == seen
}

// Run the Consul main loop which retries operations against Consul. It should
// be called exactly once.
func (c *ServiceClient) Run() {
	defer close(c.exitCh)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init will be closed when Consul has been contacted
	init := make(chan struct{})
	go checkConsulTLSSkipVerify(ctx, c.logger, c.client, init)

	// Process operations while waiting for initial contact with Consul but
	// do not sync until contact has been made.
INIT:
	for {
		select {
		case <-init:
			c.markSeen()
			break INIT
		case <-c.shutdownCh:
			return
		case ops := <-c.opCh:
			c.merge(ops)
		}
	}
	c.logger.Printf("[TRACE] consul.sync: able to contact Consul")

	// Block until contact with Consul has been established
	// Start checkWatcher
	go c.checkWatcher.Run(ctx)

	// Always immediately sync to reconcile Nomad and Consul's state
	retryTimer := time.NewTimer(0)

	failures := 0
	for {
		select {
		case <-retryTimer.C:
		case <-c.shutdownCh:
			// Cancel check watcher but sync one last time
			cancel()
		case ops := <-c.opCh:
			c.merge(ops)
		}

		if err := c.sync(); err != nil {
			if failures == 0 {
				// Log on the first failure
				c.logger.Printf("[WARN] consul.sync: failed to update services in Consul: %v", err)
			} else if failures%10 == 0 {
				// Log every 10th consecutive failure
				c.logger.Printf("[ERR] consul.sync: still unable to update services in Consul after %d failures; latest error: %v", failures, err)
			}

			failures++
			if !retryTimer.Stop() {
				// Timer already expired, since the timer may
				// or may not have been read in the select{}
				// above, conditionally receive on it
				select {
				case <-retryTimer.C:
				default:
				}
			}
			backoff := c.retryInterval * time.Duration(failures)
			if backoff > c.maxRetryInterval {
				backoff = c.maxRetryInterval
			}
			retryTimer.Reset(backoff)
		} else {
			if failures > 0 {
				c.logger.Printf("[INFO] consul.sync: successfully updated services in Consul")
				failures = 0
			}

			// Reset timer to periodic interval to periodically
			// reconile with Consul
			if !retryTimer.Stop() {
				select {
				case <-retryTimer.C:
				default:
				}
			}
			retryTimer.Reset(c.periodicInterval)
		}

		select {
		case <-c.shutdownCh:
			// Exit only after sync'ing all outstanding operations
			if len(c.opCh) > 0 {
				for len(c.opCh) > 0 {
					c.merge(<-c.opCh)
				}
				continue
			}
			return
		default:
		}

	}
}

// commit operations unless already shutting down.
func (c *ServiceClient) commit(ops *operations) {
	select {
	case c.opCh <- ops:
	case <-c.shutdownCh:
	}
}

// merge registrations into state map prior to sync'ing with Consul
func (c *ServiceClient) merge(ops *operations) {
	for _, s := range ops.regServices {
		c.services[s.ID] = s
	}
	for _, check := range ops.regChecks {
		c.checks[check.ID] = check
	}
	for _, s := range ops.scripts {
		c.scripts[s.id] = s
	}
	for _, sid := range ops.deregServices {
		delete(c.services, sid)
	}
	for _, cid := range ops.deregChecks {
		if script, ok := c.runningScripts[cid]; ok {
			script.cancel()
			delete(c.scripts, cid)
			delete(c.runningScripts, cid)
		}
		delete(c.checks, cid)
	}
	metrics.SetGauge([]string{"client", "consul", "services"}, float32(len(c.services)))
	metrics.SetGauge([]string{"client", "consul", "checks"}, float32(len(c.checks)))
	metrics.SetGauge([]string{"client", "consul", "script_checks"}, float32(len(c.runningScripts)))
}

// sync enqueued operations.
func (c *ServiceClient) sync() error {
	sreg, creg, sdereg, cdereg := 0, 0, 0, 0

	consulServices, err := c.client.Services()
	if err != nil {
		metrics.IncrCounter([]string{"client", "consul", "sync_failure"}, 1)
		return fmt.Errorf("error querying Consul services: %v", err)
	}

	consulChecks, err := c.client.Checks()
	if err != nil {
		metrics.IncrCounter([]string{"client", "consul", "sync_failure"}, 1)
		return fmt.Errorf("error querying Consul checks: %v", err)
	}

	// Remove Nomad services in Consul but unknown locally
	for id := range consulServices {
		if _, ok := c.services[id]; ok {
			// Known service, skip
			continue
		}
		if !isNomadService(id) {
			// Not managed by Nomad, skip
			continue
		}

		// Unknown Nomad managed service; kill
		if err := c.client.ServiceDeregister(id); err != nil {
			if isOldNomadService(id) {
				// Don't hard-fail on old entries. See #3620
				continue
			}

			metrics.IncrCounter([]string{"client", "consul", "sync_failure"}, 1)
			return err
		}
		sdereg++
		metrics.IncrCounter([]string{"client", "consul", "service_deregistrations"}, 1)
	}

	// Add Nomad services missing from Consul
	for id, locals := range c.services {
		if _, ok := consulServices[id]; !ok {
			if err = c.client.ServiceRegister(locals); err != nil {
				metrics.IncrCounter([]string{"client", "consul", "sync_failure"}, 1)
				return err
			}
			sreg++
			metrics.IncrCounter([]string{"client", "consul", "service_registrations"}, 1)
		}
	}

	// Remove Nomad checks in Consul but unknown locally
	for id, check := range consulChecks {
		if _, ok := c.checks[id]; ok {
			// Known check, leave it
			continue
		}
		if !isNomadService(check.ServiceID) {
			// Service not managed by Nomad, skip
			continue
		}

		// Unknown Nomad managed check; remove
		if err := c.client.CheckDeregister(id); err != nil {
			if isOldNomadService(check.ServiceID) {
				// Don't hard-fail on old entries.
				continue
			}

			metrics.IncrCounter([]string{"client", "consul", "sync_failure"}, 1)
			return err
		}
		cdereg++
		metrics.IncrCounter([]string{"client", "consul", "check_deregistrations"}, 1)
	}

	// Add Nomad checks missing from Consul
	for id, check := range c.checks {
		if _, ok := consulChecks[id]; ok {
			// Already in Consul; skipping
			continue
		}

		if err := c.client.CheckRegister(check); err != nil {
			metrics.IncrCounter([]string{"client", "consul", "sync_failure"}, 1)
			return err
		}
		creg++
		metrics.IncrCounter([]string{"client", "consul", "check_registrations"}, 1)

		// Handle starting scripts
		if script, ok := c.scripts[id]; ok {
			// If it's already running, cancel and replace
			if oldScript, running := c.runningScripts[id]; running {
				oldScript.cancel()
			}
			// Start and store the handle
			c.runningScripts[id] = script.run()
		}
	}

	c.logger.Printf("[DEBUG] consul.sync: registered %d services, %d checks; deregistered %d services, %d checks",
		sreg, creg, sdereg, cdereg)
	return nil
}

// RegisterAgent registers Nomad agents (client or server). The
// Service.PortLabel should be a literal port to be parsed with SplitHostPort.
// Script checks are not supported and will return an error. Registration is
// asynchronous.
//
// Agents will be deregistered when Shutdown is called.
func (c *ServiceClient) RegisterAgent(role string, services []*structs.Service) error {
	ops := operations{}

	for _, service := range services {
		id := makeAgentServiceID(role, service)

		// Unlike tasks, agents don't use port labels. Agent ports are
		// stored directly in the PortLabel.
		host, rawport, err := net.SplitHostPort(service.PortLabel)
		if err != nil {
			return fmt.Errorf("error parsing port label %q from service %q: %v", service.PortLabel, service.Name, err)
		}
		port, err := strconv.Atoi(rawport)
		if err != nil {
			return fmt.Errorf("error parsing port %q from service %q: %v", rawport, service.Name, err)
		}
		serviceReg := &api.AgentServiceRegistration{
			ID:      id,
			Name:    service.Name,
			Tags:    service.Tags,
			Address: host,
			Port:    port,
		}
		ops.regServices = append(ops.regServices, serviceReg)

		for _, check := range service.Checks {
			checkID := makeCheckID(id, check)
			if check.Type == structs.ServiceCheckScript {
				return fmt.Errorf("service %q contains invalid check: agent checks do not support scripts", service.Name)
			}
			checkHost, checkPort := serviceReg.Address, serviceReg.Port
			if check.PortLabel != "" {
				// Unlike tasks, agents don't use port labels. Agent ports are
				// stored directly in the PortLabel.
				host, rawport, err := net.SplitHostPort(check.PortLabel)
				if err != nil {
					return fmt.Errorf("error parsing port label %q from check %q: %v", service.PortLabel, check.Name, err)
				}
				port, err := strconv.Atoi(rawport)
				if err != nil {
					return fmt.Errorf("error parsing port %q from check %q: %v", rawport, check.Name, err)
				}
				checkHost, checkPort = host, port
			}
			checkReg, err := createCheckReg(id, checkID, check, checkHost, checkPort)
			if err != nil {
				return fmt.Errorf("failed to add check %q: %v", check.Name, err)
			}
			ops.regChecks = append(ops.regChecks, checkReg)
		}
	}

	// Don't bother committing agent checks if we're already shutting down
	c.agentLock.Lock()
	defer c.agentLock.Unlock()
	select {
	case <-c.shutdownCh:
		return nil
	default:
	}

	// Now add them to the registration queue
	c.commit(&ops)

	// Record IDs for deregistering on shutdown
	for _, id := range ops.regServices {
		c.agentServices[id.ID] = struct{}{}
	}
	for _, id := range ops.regChecks {
		c.agentChecks[id.ID] = struct{}{}
	}
	return nil
}

// serviceRegs creates service registrations, check registrations, and script
// checks from a service. It returns a service registration object with the
// service and check IDs populated.
func (c *ServiceClient) serviceRegs(ops *operations, service *structs.Service, task *TaskServices) (
	*ServiceRegistration, error) {

	// Get the services ID
	id := makeTaskServiceID(task.AllocID, task.Name, service, task.Canary)
	sreg := &ServiceRegistration{
		serviceID: id,
		checkIDs:  make(map[string]struct{}, len(service.Checks)),
	}

	// Service address modes default to auto
	addrMode := service.AddressMode
	if addrMode == "" {
		addrMode = structs.AddressModeAuto
	}

	// Determine the address to advertise based on the mode
	ip, port, err := getAddress(addrMode, service.PortLabel, task.Networks, task.DriverNetwork)
	if err != nil {
		return nil, fmt.Errorf("unable to get address for service %q: %v", service.Name, err)
	}

	// Determine whether to use tags or canary_tags
	var tags []string
	if task.Canary {
		tags = make([]string, len(service.CanaryTags))
		copy(tags, service.CanaryTags)
	} else {
		tags = make([]string, len(service.Tags))
		copy(tags, service.Tags)
	}

	// Build the Consul Service registration request
	serviceReg := &api.AgentServiceRegistration{
		ID:      id,
		Name:    service.Name,
		Tags:    tags,
		Address: ip,
		Port:    port,
	}
	ops.regServices = append(ops.regServices, serviceReg)

	// Build the check registrations
	checkIDs, err := c.checkRegs(ops, id, service, task)
	if err != nil {
		return nil, err
	}
	for _, cid := range checkIDs {
		sreg.checkIDs[cid] = struct{}{}
	}
	return sreg, nil
}

// checkRegs registers the checks for the given service and returns the
// registered check ids.
func (c *ServiceClient) checkRegs(ops *operations, serviceID string, service *structs.Service,
	task *TaskServices) ([]string, error) {

	// Fast path
	numChecks := len(service.Checks)
	if numChecks == 0 {
		return nil, nil
	}

	checkIDs := make([]string, 0, numChecks)
	for _, check := range service.Checks {
		checkID := makeCheckID(serviceID, check)
		checkIDs = append(checkIDs, checkID)
		if check.Type == structs.ServiceCheckScript {
			if task.DriverExec == nil {
				return nil, fmt.Errorf("driver doesn't support script checks")
			}

			sc := newScriptCheck(task.AllocID, task.Name, checkID, check, task.DriverExec,
				c.client, c.logger, c.shutdownCh)
			ops.scripts = append(ops.scripts, sc)

			// Skip getAddress for script checks
			checkReg, err := createCheckReg(serviceID, checkID, check, "", 0)
			if err != nil {
				return nil, fmt.Errorf("failed to add script check %q: %v", check.Name, err)
			}
			ops.regChecks = append(ops.regChecks, checkReg)
			continue
		}

		// Default to the service's port but allow check to override
		portLabel := check.PortLabel
		if portLabel == "" {
			// Default to the service's port label
			portLabel = service.PortLabel
		}

		// Checks address mode defaults to host for pre-#3380 backward compat
		addrMode := check.AddressMode
		if addrMode == "" {
			addrMode = structs.AddressModeHost
		}

		ip, port, err := getAddress(addrMode, portLabel, task.Networks, task.DriverNetwork)
		if err != nil {
			return nil, fmt.Errorf("error getting address for check %q: %v", check.Name, err)
		}

		checkReg, err := createCheckReg(serviceID, checkID, check, ip, port)
		if err != nil {
			return nil, fmt.Errorf("failed to add check %q: %v", check.Name, err)
		}
		ops.regChecks = append(ops.regChecks, checkReg)
	}
	return checkIDs, nil
}

// RegisterTask with Consul. Adds all service entries and checks to Consul. If
// exec is nil and a script check exists an error is returned.
//
// If the service IP is set it used as the address in the service registration.
// Checks will always use the IP from the Task struct (host's IP).
//
// Actual communication with Consul is done asynchronously (see Run).
func (c *ServiceClient) RegisterTask(task *TaskServices) error {
	// Fast path
	numServices := len(task.Services)
	if numServices == 0 {
		return nil
	}

	t := new(TaskRegistration)
	t.Services = make(map[string]*ServiceRegistration, numServices)

	ops := &operations{}
	for _, service := range task.Services {
		sreg, err := c.serviceRegs(ops, service, task)
		if err != nil {
			return err
		}
		t.Services[sreg.serviceID] = sreg
	}

	// Add the task to the allocation's registration
	c.addTaskRegistration(task.AllocID, task.Name, t)

	c.commit(ops)

	// Start watching checks. Done after service registrations are built
	// since an error building them could leak watches.
	for _, service := range task.Services {
		serviceID := makeTaskServiceID(task.AllocID, task.Name, service, task.Canary)
		for _, check := range service.Checks {
			if check.TriggersRestarts() {
				checkID := makeCheckID(serviceID, check)
				c.checkWatcher.Watch(task.AllocID, task.Name, checkID, check, task.Restarter)
			}
		}
	}
	return nil
}

// UpdateTask in Consul. Does not alter the service if only checks have
// changed.
//
// DriverNetwork must not change between invocations for the same allocation.
func (c *ServiceClient) UpdateTask(old, newTask *TaskServices) error {
	ops := &operations{}

	taskReg := new(TaskRegistration)
	taskReg.Services = make(map[string]*ServiceRegistration, len(newTask.Services))

	existingIDs := make(map[string]*structs.Service, len(old.Services))
	for _, s := range old.Services {
		existingIDs[makeTaskServiceID(old.AllocID, old.Name, s, old.Canary)] = s
	}
	newIDs := make(map[string]*structs.Service, len(newTask.Services))
	for _, s := range newTask.Services {
		newIDs[makeTaskServiceID(newTask.AllocID, newTask.Name, s, newTask.Canary)] = s
	}

	// Loop over existing Service IDs to see if they have been removed or
	// updated.
	for existingID, existingSvc := range existingIDs {
		newSvc, ok := newIDs[existingID]
		if !ok {
			// Existing service entry removed
			ops.deregServices = append(ops.deregServices, existingID)
			for _, check := range existingSvc.Checks {
				cid := makeCheckID(existingID, check)
				ops.deregChecks = append(ops.deregChecks, cid)

				// Unwatch watched checks
				if check.TriggersRestarts() {
					c.checkWatcher.Unwatch(cid)
				}
			}
			continue
		}

		// Service exists and hasn't changed, don't re-add it later
		delete(newIDs, existingID)

		// Service still exists so add it to the task's registration
		sreg := &ServiceRegistration{
			serviceID: existingID,
			checkIDs:  make(map[string]struct{}, len(newSvc.Checks)),
		}
		taskReg.Services[existingID] = sreg

		// See if any checks were updated
		existingChecks := make(map[string]*structs.ServiceCheck, len(existingSvc.Checks))
		for _, check := range existingSvc.Checks {
			existingChecks[makeCheckID(existingID, check)] = check
		}

		// Register new checks
		for _, check := range newSvc.Checks {
			checkID := makeCheckID(existingID, check)
			if _, exists := existingChecks[checkID]; exists {
				// Check exists, so don't remove it
				delete(existingChecks, checkID)
				sreg.checkIDs[checkID] = struct{}{}
			}

			// New check on an unchanged service; add them now
			newCheckIDs, err := c.checkRegs(ops, existingID, newSvc, newTask)
			if err != nil {
				return err
			}

			for _, checkID := range newCheckIDs {
				sreg.checkIDs[checkID] = struct{}{}

			}

			// Update all watched checks as CheckRestart fields aren't part of ID
			if check.TriggersRestarts() {
				c.checkWatcher.Watch(newTask.AllocID, newTask.Name, checkID, check, newTask.Restarter)
			}
		}

		// Remove existing checks not in updated service
		for cid, check := range existingChecks {
			ops.deregChecks = append(ops.deregChecks, cid)

			// Unwatch checks
			if check.TriggersRestarts() {
				c.checkWatcher.Unwatch(cid)
			}
		}
	}

	// Any remaining services should just be enqueued directly
	for _, newSvc := range newIDs {
		sreg, err := c.serviceRegs(ops, newSvc, newTask)
		if err != nil {
			return err
		}

		taskReg.Services[sreg.serviceID] = sreg
	}

	// Add the task to the allocation's registration
	c.addTaskRegistration(newTask.AllocID, newTask.Name, taskReg)

	c.commit(ops)

	// Start watching checks. Done after service registrations are built
	// since an error building them could leak watches.
	for _, service := range newIDs {
		serviceID := makeTaskServiceID(newTask.AllocID, newTask.Name, service, newTask.Canary)
		for _, check := range service.Checks {
			if check.TriggersRestarts() {
				checkID := makeCheckID(serviceID, check)
				c.checkWatcher.Watch(newTask.AllocID, newTask.Name, checkID, check, newTask.Restarter)
			}
		}
	}
	return nil
}

// RemoveTask from Consul. Removes all service entries and checks.
//
// Actual communication with Consul is done asynchronously (see Run).
func (c *ServiceClient) RemoveTask(task *TaskServices) {
	ops := operations{}

	for _, service := range task.Services {
		id := makeTaskServiceID(task.AllocID, task.Name, service, task.Canary)
		ops.deregServices = append(ops.deregServices, id)

		for _, check := range service.Checks {
			cid := makeCheckID(id, check)
			ops.deregChecks = append(ops.deregChecks, cid)

			if check.TriggersRestarts() {
				c.checkWatcher.Unwatch(cid)
			}
		}
	}

	// Remove the task from the alloc's registrations
	c.removeTaskRegistration(task.AllocID, task.Name)

	// Now add them to the deregistration fields; main Run loop will update
	c.commit(&ops)
}

// AllocRegistrations returns the registrations for the given allocation. If the
// allocation has no reservations, the response is a nil object.
func (c *ServiceClient) AllocRegistrations(allocID string) (*AllocRegistration, error) {
	// Get the internal struct using the lock
	c.allocRegistrationsLock.RLock()
	regInternal, ok := c.allocRegistrations[allocID]
	if !ok {
		c.allocRegistrationsLock.RUnlock()
		return nil, nil
	}

	// Copy so we don't expose internal structs
	reg := regInternal.copy()
	c.allocRegistrationsLock.RUnlock()

	// Query the services and checks to populate the allocation registrations.
	services, err := c.client.Services()
	if err != nil {
		return nil, err
	}

	checks, err := c.client.Checks()
	if err != nil {
		return nil, err
	}

	// Populate the object
	for _, treg := range reg.Tasks {
		for serviceID, sreg := range treg.Services {
			sreg.Service = services[serviceID]
			for checkID := range sreg.checkIDs {
				if check, ok := checks[checkID]; ok {
					sreg.Checks = append(sreg.Checks, check)
				}
			}
		}
	}

	return reg, nil
}

// Shutdown the Consul client. Update running task registrations and deregister
// agent from Consul. On first call blocks up to shutdownWait before giving up
// on syncing operations.
func (c *ServiceClient) Shutdown() error {
	// Serialize Shutdown calls with RegisterAgent to prevent leaking agent
	// entries.
	c.agentLock.Lock()
	defer c.agentLock.Unlock()
	select {
	case <-c.shutdownCh:
		return nil
	default:
		close(c.shutdownCh)
	}

	// Give run loop time to sync, but don't block indefinitely
	deadline := time.After(c.shutdownWait)

	// Wait for Run to finish any outstanding operations and exit
	select {
	case <-c.exitCh:
	case <-deadline:
		// Don't wait forever though
	}

	// If Consul was never seen nothing could be written so exit early
	if !c.hasSeen() {
		return nil
	}

	// Always attempt to deregister Nomad agent Consul entries, even if
	// deadline was reached
	for id := range c.agentServices {
		if err := c.client.ServiceDeregister(id); err != nil {
			c.logger.Printf("[ERR] consul.sync: error deregistering agent service (id: %q): %v", id, err)
		}
	}
	for id := range c.agentChecks {
		if err := c.client.CheckDeregister(id); err != nil {
			c.logger.Printf("[ERR] consul.sync: error deregistering agent service (id: %q): %v", id, err)
		}
	}

	// Give script checks time to exit (no need to lock as Run() has exited)
	for _, h := range c.runningScripts {
		select {
		case <-h.wait():
		case <-deadline:
			return fmt.Errorf("timed out waiting for script checks to run")
		}
	}
	return nil
}

// addTaskRegistration adds the task registration for the given allocation.
func (c *ServiceClient) addTaskRegistration(allocID, taskName string, reg *TaskRegistration) {
	c.allocRegistrationsLock.Lock()
	defer c.allocRegistrationsLock.Unlock()

	alloc, ok := c.allocRegistrations[allocID]
	if !ok {
		alloc = &AllocRegistration{
			Tasks: make(map[string]*TaskRegistration),
		}
		c.allocRegistrations[allocID] = alloc
	}
	alloc.Tasks[taskName] = reg
}

// removeTaskRegistration removes the task registration for the given allocation.
func (c *ServiceClient) removeTaskRegistration(allocID, taskName string) {
	c.allocRegistrationsLock.Lock()
	defer c.allocRegistrationsLock.Unlock()

	alloc, ok := c.allocRegistrations[allocID]
	if !ok {
		return
	}

	// Delete the task and if it is the last one also delete the alloc's
	// registration
	delete(alloc.Tasks, taskName)
	if len(alloc.Tasks) == 0 {
		delete(c.allocRegistrations, allocID)
	}
}

// makeAgentServiceID creates a unique ID for identifying an agent service in
// Consul.
//
// Agent service IDs are of the form:
//
//	{nomadServicePrefix}-{ROLE}-b32(sha1({Service.Name}-{Service.Tags...})
//	Example Server ID: _nomad-server-fbbk265qn4tmt25nd4ep42tjvmyj3hr4
//	Example Client ID: _nomad-client-ggnjpgl7yn7rgmvxzilmpvrzzvrszc7l
//
func makeAgentServiceID(role string, service *structs.Service) string {
	return fmt.Sprintf("%s-%s-%s", nomadServicePrefix, role, service.Hash(role, "", false))
}

// makeTaskServiceID creates a unique ID for identifying a task service in
// Consul. All structs.Service fields are included in the ID's hash except
// Checks. This allows updates to merely compare IDs.
//
//	Example Service ID: _nomad-task-TNM333JKJPM5AK4FAS3VXQLXFDWOF4VH
func makeTaskServiceID(allocID, taskName string, service *structs.Service, canary bool) string {
	return nomadTaskPrefix + service.Hash(allocID, taskName, canary)
}

// makeCheckID creates a unique ID for a check.
func makeCheckID(serviceID string, check *structs.ServiceCheck) string {
	return check.Hash(serviceID)
}

// createCheckReg creates a Check that can be registered with Consul.
//
// Script checks simply have a TTL set and the caller is responsible for
// running the script and heartbeating.
func createCheckReg(serviceID, checkID string, check *structs.ServiceCheck, host string, port int) (*api.AgentCheckRegistration, error) {
	chkReg := api.AgentCheckRegistration{
		ID:        checkID,
		Name:      check.Name,
		ServiceID: serviceID,
	}
	chkReg.Status = check.InitialStatus
	chkReg.Timeout = check.Timeout.String()
	chkReg.Interval = check.Interval.String()

	// Require an address for http or tcp checks
	if port == 0 && check.RequiresPort() {
		return nil, fmt.Errorf("%s checks require an address", check.Type)
	}

	switch check.Type {
	case structs.ServiceCheckHTTP:
		proto := check.Protocol
		if proto == "" {
			proto = "http"
		}
		if check.TLSSkipVerify {
			chkReg.TLSSkipVerify = true
		}
		base := url.URL{
			Scheme: proto,
			Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		}
		relative, err := url.Parse(check.Path)
		if err != nil {
			return nil, err
		}
		url := base.ResolveReference(relative)
		chkReg.HTTP = url.String()
		chkReg.Method = check.Method
		chkReg.Header = check.Header

	case structs.ServiceCheckTCP:
		chkReg.TCP = net.JoinHostPort(host, strconv.Itoa(port))

	case structs.ServiceCheckScript:
		chkReg.TTL = (check.Interval + ttlCheckBuffer).String()
		// As of Consul 1.0.0 setting TTL and Interval is a 400
		chkReg.Interval = ""

	case structs.ServiceCheckGRPC:
		chkReg.GRPC = fmt.Sprintf("%s/%s", net.JoinHostPort(host, strconv.Itoa(port)), check.GRPCService)
		chkReg.GRPCUseTLS = check.GRPCUseTLS
		if check.TLSSkipVerify {
			chkReg.TLSSkipVerify = true
		}

	default:
		return nil, fmt.Errorf("check type %+q not valid", check.Type)
	}
	return &chkReg, nil
}

// isNomadService returns true if the ID matches the pattern of a Nomad managed
// service (new or old formats). Agent services return false as independent
// client and server agents may be running on the same machine. #2827
func isNomadService(id string) bool {
	return strings.HasPrefix(id, nomadTaskPrefix) || isOldNomadService(id)
}

// isOldNomadService returns true if the ID matches an old pattern managed by
// Nomad.
//
// Pre-0.7.1 task service IDs are of the form:
//
//	{nomadServicePrefix}-executor-{ALLOC_ID}-{Service.Name}-{Service.Tags...}
//	Example Service ID: _nomad-executor-1234-echo-http-tag1-tag2-tag3
//
func isOldNomadService(id string) bool {
	const prefix = nomadServicePrefix + "-executor"
	return strings.HasPrefix(id, prefix)
}

// getAddress returns the IP and port to use for a service or check. If no port
// label is specified (an empty value), zero values are returned because no
// address could be resolved.
func getAddress(addrMode, portLabel string, networks structs.Networks, driverNet *cstructs.DriverNetwork) (string, int, error) {
	switch addrMode {
	case structs.AddressModeAuto:
		if driverNet.Advertise() {
			addrMode = structs.AddressModeDriver
		} else {
			addrMode = structs.AddressModeHost
		}
		return getAddress(addrMode, portLabel, networks, driverNet)
	case structs.AddressModeHost:
		if portLabel == "" {
			if len(networks) != 1 {
				// If no networks are specified return zero
				// values. Consul will advertise the host IP
				// with no port. This is the pre-0.7.1 behavior
				// some people rely on.
				return "", 0, nil
			}

			return networks[0].IP, 0, nil
		}

		// Default path: use host ip:port
		ip, port := networks.Port(portLabel)
		if ip == "" && port <= 0 {
			return "", 0, fmt.Errorf("invalid port %q: port label not found", portLabel)
		}
		return ip, port, nil

	case structs.AddressModeDriver:
		// Require a driver network if driver address mode is used
		if driverNet == nil {
			return "", 0, fmt.Errorf(`cannot use address_mode="driver": no driver network exists`)
		}

		// If no port label is specified just return the IP
		if portLabel == "" {
			return driverNet.IP, 0, nil
		}

		// If the port is a label, use the driver's port (not the host's)
		if port, ok := driverNet.PortMap[portLabel]; ok {
			return driverNet.IP, port, nil
		}

		// If port isn't a label, try to parse it as a literal port number
		port, err := strconv.Atoi(portLabel)
		if err != nil {
			// Don't include Atoi error message as user likely
			// never intended it to be a numeric and it creates a
			// confusing error message
			return "", 0, fmt.Errorf("invalid port label %q: port labels in driver address_mode must be numeric or in the driver's port map", portLabel)
		}
		if port <= 0 {
			return "", 0, fmt.Errorf("invalid port: %q: port must be >0", portLabel)
		}

		return driverNet.IP, port, nil

	default:
		// Shouldn't happen due to validation, but enforce invariants
		return "", 0, fmt.Errorf("invalid address mode %q", addrMode)
	}
}
