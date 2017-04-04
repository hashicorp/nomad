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
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
)

var mark = struct{}{}

const (
	// nomadServicePrefix is the first prefix that scopes all Nomad registered
	// services
	nomadServicePrefix = "_nomad"

	// The periodic time interval for syncing services and checks with Consul
	defaultSyncInterval = 6 * time.Second

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

// ScriptExecutor is the interface the ServiceClient uses to execute script
// checks inside a container.
type ScriptExecutor interface {
	Exec(ctx context.Context, cmd string, args []string) ([]byte, int, error)
}

// CatalogAPI is the consul/api.Catalog API used by Nomad.
type CatalogAPI interface {
	Datacenters() ([]string, error)
	Service(service, tag string, q *api.QueryOptions) ([]*api.CatalogService, *api.QueryMeta, error)
}

// AgentAPI is the consul/api.Agent API used by Nomad.
type AgentAPI interface {
	CheckRegister(check *api.AgentCheckRegistration) error
	CheckDeregister(checkID string) error
	ServiceRegister(service *api.AgentServiceRegistration) error
	ServiceDeregister(serviceID string) error
	UpdateTTL(id, output, status string) error
}

// ServiceClient handles task and agent service registration with Consul.
type ServiceClient struct {
	client        AgentAPI
	logger        *log.Logger
	retryInterval time.Duration

	// runningCh is closed when the main Run loop exits
	runningCh chan struct{}

	// shutdownCh is closed when the client should shutdown
	shutdownCh chan struct{}

	// shutdownWait is how long Shutdown() blocks waiting for the final
	// sync() to finish. Defaults to defaultShutdownWait
	shutdownWait time.Duration

	// syncCh triggers a sync in the main Run loop
	syncCh chan struct{}

	// pending service and check operations
	pending *consulOps
	opsLock sync.Mutex

	// script check cancel funcs to be called before their corresponding
	// check is removed. Only accessed in sync() so not covered by regLock
	runningScripts map[string]*scriptHandle

	// regLock must be held while accessing reg and dereg maps
	regLock sync.Mutex

	// Registered agent services and checks
	agentServices map[string]struct{}
	agentChecks   map[string]struct{}

	// agentLock must be held while accessing agent maps
	agentLock sync.Mutex
}

// NewServiceClient creates a new Consul ServiceClient from an existing Consul API
// Client and logger.
func NewServiceClient(consulClient AgentAPI, logger *log.Logger) *ServiceClient {
	return &ServiceClient{
		client:         consulClient,
		logger:         logger,
		retryInterval:  defaultSyncInterval,
		runningCh:      make(chan struct{}),
		shutdownCh:     make(chan struct{}),
		shutdownWait:   defaultShutdownWait,
		syncCh:         make(chan struct{}, 1),
		pending:        newConsulOps(),
		runningScripts: make(map[string]*scriptHandle),
		agentServices:  make(map[string]struct{}, 8),
		agentChecks:    make(map[string]struct{}, 8),
	}
}

// Run the Consul main loop which retries operations against Consul. It should
// be called exactly once.
func (c *ServiceClient) Run() {
	defer close(c.runningCh)
	timer := time.NewTimer(0)
	defer timer.Stop()

	// Drain the initial tick so we don't sync until instructed
	<-timer.C

	lastOk := true
	for {
		select {
		case <-c.syncCh:
			timer.Reset(0)
		case <-timer.C:
			if err := c.sync(); err != nil {
				if lastOk {
					lastOk = false
					c.logger.Printf("[WARN] consul: failed to update services in Consul: %v", err)
				}
				timer.Reset(c.retryInterval)
			} else {
				if !lastOk {
					c.logger.Printf("[INFO] consul: successfully updated services in Consul")
					lastOk = true
				}
			}
		case <-c.shutdownCh:
			return
		}
	}
}

// forceSync asynchronously causes a sync to happen. Any operations enqueued
// prior to calling forceSync will be synced.
func (c *ServiceClient) forceSync() {
	select {
	case c.syncCh <- mark:
	default:
	}
}

// sync enqueued operations.
func (c *ServiceClient) sync() error {
	c.opsLock.Lock()
	ops := c.pending
	c.pending = newConsulOps()
	c.opsLock.Unlock()

	var err error

	msg := ops.String()

	// Register Services
	for id, service := range ops.regServices {
		if err = c.client.ServiceRegister(service); err != nil {
			goto ERROR
		}
		delete(ops.regServices, id)
	}

	// Register Checks
	for id, check := range ops.regChecks {
		if err = c.client.CheckRegister(check); err != nil {
			goto ERROR
		}
		delete(ops.regChecks, id)

		// Run the script for this check if one exists
		if script, ok := ops.regScripts[id]; ok {
			// This check is a script check; run it
			c.runningScripts[id] = script.run()
		}
	}

	// Deregister Checks
	for id := range ops.deregChecks {
		if h, ok := c.runningScripts[id]; ok {
			// This check is a script check; stop it
			h.cancel()
			delete(c.runningScripts, id)
		}

		if err = c.client.CheckDeregister(id); err != nil {
			goto ERROR
		}
		delete(ops.deregChecks, id)
	}

	// Deregister Services
	for id := range ops.deregServices {
		if err = c.client.ServiceDeregister(id); err != nil {
			goto ERROR
		}
		delete(ops.deregServices, id)
	}

	c.logger.Printf("[DEBUG] consul: %s", msg)
	return nil

	//TODO Labels and gotos are nasty; move to a function?
ERROR:
	// An error occurred, repopulate the operation maps but give
	// precendence to new ops
	c.opsLock.Lock()
	ops.merge(c.pending)
	c.pending = ops
	c.opsLock.Unlock()
	return err
}

// RegisterAgent registers Nomad agents (client or server). Script checks are
// not supported and will return an error. Registration is asynchronous.
//
// Agents will be deregistered when Shutdown is called.
func (c *ServiceClient) RegisterAgent(role string, services []*structs.Service) error {
	ops := newConsulOps()

	for _, service := range services {
		id := makeAgentServiceID(role, service)
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
		ops.regServices[id] = serviceReg

		for _, check := range service.Checks {
			checkID := createCheckID(id, check)
			if check.Type == structs.ServiceCheckScript {
				return fmt.Errorf("service %q contains invalid check: agent checks do not support scripts", service.Name)
			}
			checkHost, checkPort := serviceReg.Address, serviceReg.Port
			if check.PortLabel != "" {
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
			ops.regChecks[checkID] = checkReg
		}
	}

	// Now add them to the registration queue
	c.opsLock.Lock()
	c.pending.merge(ops)
	c.opsLock.Unlock()

	// Record IDs for deregistering on shutdown
	c.agentLock.Lock()
	for id := range ops.regServices {
		c.agentServices[id] = mark
	}
	for id := range ops.regChecks {
		c.agentChecks[id] = mark
	}
	c.agentLock.Unlock()
	c.forceSync()
	return nil
}

type addrParser func(portLabel string) (string, int)

// makeCheckReg adds a check reg to operations.
func (c *ServiceClient) makeCheckReg(ops *consulOps, check *structs.ServiceCheck,
	service *api.AgentServiceRegistration, exec ScriptExecutor, parseAddr addrParser) error {

	checkID := createCheckID(service.ID, check)
	if check.Type == structs.ServiceCheckScript {
		if exec == nil {
			return fmt.Errorf("driver doesn't support script checks")
		}
		ops.regScripts[checkID] = newScriptCheck(
			checkID, check, exec, c.client, c.logger, c.shutdownCh)
	}
	host, port := service.Address, service.Port
	if check.PortLabel != "" {
		host, port = parseAddr(check.PortLabel)
	}
	checkReg, err := createCheckReg(service.ID, checkID, check, host, port)
	if err != nil {
		return fmt.Errorf("failed to add check %q: %v", check.Name, err)
	}
	ops.regChecks[checkID] = checkReg
	return nil
}

// serviceRegs creates service registrations, check registrations, and script
// checks from a service.
func (c *ServiceClient) serviceRegs(ops *consulOps, allocID string, service *structs.Service,
	exec ScriptExecutor, task *structs.Task) error {

	id := makeTaskServiceID(allocID, task.Name, service)
	host, port := task.FindHostAndPortFor(service.PortLabel)
	serviceReg := &api.AgentServiceRegistration{
		ID:      id,
		Name:    service.Name,
		Tags:    make([]string, len(service.Tags)),
		Address: host,
		Port:    port,
	}
	// copy isn't strictly necessary but can avoid bugs especially
	// with tests that may reuse Tasks
	copy(serviceReg.Tags, service.Tags)
	ops.regServices[id] = serviceReg

	for _, check := range service.Checks {
		err := c.makeCheckReg(ops, check, serviceReg, exec, task.FindHostAndPortFor)
		if err != nil {
			return err
		}
	}
	return nil
}

// RegisterTask with Consul. Adds all sevice entries and checks to Consul. If
// exec is nil and a script check exists an error is returned.
//
// Actual communication with Consul is done asynchrously (see Run).
func (c *ServiceClient) RegisterTask(allocID string, task *structs.Task, exec ScriptExecutor) error {
	ops := newConsulOps()
	for _, service := range task.Services {
		if err := c.serviceRegs(ops, allocID, service, exec, task); err != nil {
			return err
		}
	}

	// Now add them to the registration queue
	c.opsLock.Lock()
	c.pending.merge(ops)
	c.opsLock.Unlock()
	c.forceSync()
	return nil
}

// UpdateTask in Consul. Does not alter the service if only checks have
// changed.
func (c *ServiceClient) UpdateTask(allocID string, existing, newTask *structs.Task, exec ScriptExecutor) error {
	ops := newConsulOps()

	existingIDs := make(map[string]*structs.Service, len(existing.Services))
	for _, s := range existing.Services {
		existingIDs[makeTaskServiceID(allocID, existing.Name, s)] = s
		c.logger.Printf("[XXX] EXISTING: %s", makeTaskServiceID(allocID, existing.Name, s))
	}
	newIDs := make(map[string]*structs.Service, len(newTask.Services))
	for _, s := range newTask.Services {
		newIDs[makeTaskServiceID(allocID, newTask.Name, s)] = s
		c.logger.Printf("[XXX] UPDATED : %s", makeTaskServiceID(allocID, newTask.Name, s))
	}

	parseAddr := newTask.FindHostAndPortFor

	// Loop over existing Service IDs to see if they have been removed or
	// updated.
	for existingID, existingSvc := range existingIDs {
		newSvc, ok := newIDs[existingID]
		if !ok {
			// Existing sevice entry removed
			ops.deregServices[existingID] = mark
			for _, check := range existingSvc.Checks {
				ops.deregChecks[createCheckID(existingID, check)] = mark
			}
			continue
		}

		// Manipulating checks is cheap and easy, so just remove old and add new
		for _, check := range existingSvc.Checks {
			ops.deregChecks[createCheckID(existingID, check)] = mark
		}

		// Register new checks
		for _, check := range newSvc.Checks {
			checkID := createCheckID(existingID, check)
			// Don't deregister this check if it hasn't changed
			delete(ops.deregChecks, checkID)
			if check.Type == structs.ServiceCheckScript {
				if exec == nil {
					return fmt.Errorf("driver doesn't support script checks")
				}
				ops.regScripts[checkID] = newScriptCheck(
					checkID, check, exec, c.client, c.logger, c.shutdownCh)
			}
			host, port := parseAddr(existingSvc.PortLabel)
			if check.PortLabel != "" {
				host, port = parseAddr(check.PortLabel)
			}
			checkReg, err := createCheckReg(existingID, checkID, check, host, port)
			if err != nil {
				return err
			}
			ops.regChecks[checkID] = checkReg
		}

		// Service hasn't changed and checks are updated so don't
		// process this service again later
		delete(newIDs, existingID)
	}

	// Any remaining services should just be enqueued directly
	for _, newSvc := range newIDs {
		err := c.serviceRegs(ops, allocID, newSvc, exec, newTask)
		if err != nil {
			return err
		}
	}

	// Finally enqueue the updates and force sync
	c.opsLock.Lock()
	c.pending.merge(ops)
	c.opsLock.Unlock()

	c.forceSync()
	return nil
}

// RemoveTask from Consul. Removes all service entries and checks.
//
// Actual communication with Consul is done asynchrously (see Run).
func (c *ServiceClient) RemoveTask(allocID string, task *structs.Task) {
	ops := newConsulOps()

	for _, service := range task.Services {
		id := makeTaskServiceID(allocID, task.Name, service)
		ops.deregServices[id] = mark

		for _, check := range service.Checks {
			ops.deregChecks[createCheckID(id, check)] = mark
		}
	}

	// Now add them to the deregistration fields; main Run loop will update
	c.regLock.Lock()
	c.pending.merge(ops)
	c.regLock.Unlock()

	c.forceSync()
}

// Shutdown the Consul client. Update running task registations and deregister
// agent from Consul. Blocks up to shutdownWait before giving up on syncing
// operations.
func (c *ServiceClient) Shutdown() error {
	select {
	case <-c.shutdownCh:
		return nil
	default:
		close(c.shutdownCh)
	}

	var mErr multierror.Error

	// Don't let Shutdown block indefinitely
	deadline := time.After(c.shutdownWait)

	// Deregister agent services and checks
	c.agentLock.Lock()
	for id := range c.agentServices {
		if err := c.client.ServiceDeregister(id); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	// Deregister Checks
	for id := range c.agentChecks {
		if err := c.client.CheckDeregister(id); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}
	c.agentLock.Unlock()

	// Wait for Run to finish any outstanding sync() calls and exit
	select {
	case <-c.runningCh:
		// sync one last time to ensure all enqueued operations are applied
		if err := c.sync(); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	case <-deadline:
		// Don't wait forever though
		mErr.Errors = append(mErr.Errors, fmt.Errorf("timed out waiting for Consul operations to complete"))
		return mErr.ErrorOrNil()
	}

	// Give script checks time to exit (no need to lock as Run() has exited)
	for _, h := range c.runningScripts {
		select {
		case <-h.wait():
		case <-deadline:
			mErr.Errors = append(mErr.Errors, fmt.Errorf("timed out waiting for script checks to run"))
			return mErr.ErrorOrNil()
		}
	}
	return mErr.ErrorOrNil()
}

// makeAgentServiceID creates a unique ID for identifying an agent service in
// Consul.
//
// Agent service IDs are of the form:
//
//	{nomadServicePrefix}-{ROLE}-{Service.Name}-{Service.Tags...}
//	Example Server ID: _nomad-server-nomad-serf
//	Example Client ID: _nomad-client-nomad-client-http
//
func makeAgentServiceID(role string, service *structs.Service) string {
	parts := make([]string, len(service.Tags)+3)
	parts[0] = nomadServicePrefix
	parts[1] = role
	parts[2] = service.Name
	copy(parts[3:], service.Tags)
	return strings.Join(parts, "-")
}

// makeTaskServiceID creates a unique ID for identifying a task service in
// Consul.
//
// Task service IDs are of the form:
//
//	{nomadServicePrefix}-executor-{ALLOC_ID}-{Service.Name}-{Service.Tags...}
//	Example Service ID: _nomad-executor-1234-echo-http-tag1-tag2-tag3
//
func makeTaskServiceID(allocID, taskName string, service *structs.Service) string {
	parts := make([]string, len(service.Tags)+5)
	parts[0] = nomadServicePrefix
	parts[1] = "executor"
	parts[2] = allocID
	parts[3] = taskName
	parts[4] = service.Name
	copy(parts[5:], service.Tags)
	return strings.Join(parts, "-")
}

// createCheckID creates a unique ID for a check.
func createCheckID(serviceID string, check *structs.ServiceCheck) string {
	return check.Hash(serviceID)
}

// createCheckReg creates a Check that can be registered with Consul.
//
// Only supports HTTP(S) and TCP checks. Script checks must be handled
// externally.
func createCheckReg(serviceID, checkID string, check *structs.ServiceCheck, host string, port int) (*api.AgentCheckRegistration, error) {
	chkReg := api.AgentCheckRegistration{
		ID:        checkID,
		Name:      check.Name,
		ServiceID: serviceID,
	}
	chkReg.Status = check.InitialStatus
	chkReg.Timeout = check.Timeout.String()
	chkReg.Interval = check.Interval.String()

	switch check.Type {
	case structs.ServiceCheckHTTP:
		if check.Protocol == "" {
			check.Protocol = "http"
		}
		base := url.URL{
			Scheme: check.Protocol,
			Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		}
		relative, err := url.Parse(check.Path)
		if err != nil {
			return nil, err
		}
		url := base.ResolveReference(relative)
		chkReg.HTTP = url.String()
	case structs.ServiceCheckTCP:
		chkReg.TCP = net.JoinHostPort(host, strconv.Itoa(port))
	case structs.ServiceCheckScript:
		chkReg.TTL = (check.Interval + ttlCheckBuffer).String()
	default:
		return nil, fmt.Errorf("check type %+q not valid", check.Type)
	}
	return &chkReg, nil
}
