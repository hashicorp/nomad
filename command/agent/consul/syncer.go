package consul

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	consul "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/nomad/types"
)

const (
	// initialSyncBuffer is the max time an initial sync will sleep
	// before syncing.
	initialSyncBuffer = 30 * time.Second

	// initialSyncDelay is the delay before an initial sync.
	initialSyncDelay = 5 * time.Second

	// The periodic time interval for syncing services and checks with Consul
	syncInterval = 5 * time.Second

	// syncJitter provides a little variance in the frequency at which
	// Syncer polls Consul.
	syncJitter = 8

	// ttlCheckBuffer is the time interval that Nomad can take to report Consul
	// the check result
	ttlCheckBuffer = 31 * time.Second

	// ServiceTagHttp is the tag assigned to HTTP services
	ServiceTagHttp = "http"

	// ServiceTagRpc is the tag assigned to RPC services
	ServiceTagRpc = "rpc"

	// ServiceTagSerf is the tag assigned to Serf services
	ServiceTagSerf = "serf"
)

// Syncer allows syncing of services and checks with Consul
type Syncer struct {
	client    *consul.Client
	runChecks bool

	// servicesGroups is a named group of services that will be flattened
	// and reconciled with Consul when SyncServices() is called.  The key
	// to the servicesGroups map is unique per handler and is used to
	// allow the Agent's services to be maintained independently of the
	// Client or Server's services.
	servicesGroups     map[string][]*consul.AgentServiceRegistration
	servicesGroupsLock sync.RWMutex

	// The "Consul Registry" is a collection of Consul Services and
	// Checks all guarded by the registryLock.
	registryLock sync.RWMutex

	checkRunners    map[string]*CheckRunner
	delegateChecks  map[string]struct{} // delegateChecks are the checks that the Nomad client runs and reports to Consul

	// serviceRegPrefix is used to namespace the domain of registered
	// Consul Services and Checks belonging to a single Syncer.  A given
	// Nomad Agent may spawn multiple Syncer tasks between the Agent
	// Agent and its Executors, all syncing to a single Consul Agent.
	// The serviceRegPrefix allows multiple Syncers to coexist without
	// each Syncer clobbering each others Services.  The Syncer namespace
	// protocol is fmt.Sprintf("nomad-%s-%s", serviceRegPrefix, miscID).
	// serviceRegPrefix is guarded by the registryLock.
	serviceRegPrefix string

	addrFinder           func(portLabel string) (string, int)
	createDelegatedCheck func(*structs.ServiceCheck, string) (Check, error)
	// End registryLock guarded attributes.

	logger *log.Logger

	shutdownCh   chan struct{}
	shutdown     bool
	shutdownLock sync.Mutex

	// notifyShutdownCh is used to notify a Syncer it needs to shutdown.
	// This can happen because there was an explicit call to the Syncer's
	// Shutdown() method, or because the calling task signaled the
	// program is going to exit by closing its shutdownCh.
	notifyShutdownCh chan struct{}

	// periodicCallbacks is walked sequentially when the timer in Run
	// fires.
	periodicCallbacks map[string]types.PeriodicCallback
	notifySyncCh      chan struct{}
	periodicLock      sync.RWMutex
}

// NewSyncer returns a new consul.Syncer
func NewSyncer(config *config.ConsulConfig, shutdownCh chan struct{}, logger *log.Logger) (*Syncer, error) {
	var err error
	var c *consul.Client
	cfg := consul.DefaultConfig()
	if config.Addr != "" {
		cfg.Address = config.Addr
	}
	if config.Token != "" {
		cfg.Token = config.Token
	}
	if config.Auth != "" {
		var username, password string
		if strings.Contains(config.Auth, ":") {
			split := strings.SplitN(config.Auth, ":", 2)
			username = split[0]
			password = split[1]
		} else {
			username = config.Auth
		}

		cfg.HttpAuth = &consul.HttpBasicAuth{
			Username: username,
			Password: password,
		}
	}
	if config.EnableSSL {
		cfg.Scheme = "https"
		tlsCfg := consul.TLSConfig{
			Address:            cfg.Address,
			CAFile:             config.CAFile,
			CertFile:           config.CertFile,
			KeyFile:            config.KeyFile,
			InsecureSkipVerify: !config.VerifySSL,
		}
		tlsClientCfg, err := consul.SetupTLSConfig(&tlsCfg)
		if err != nil {
			return nil, fmt.Errorf("error creating tls client config for consul: %v", err)
		}
		cfg.HttpClient.Transport = &http.Transport{
			TLSClientConfig: tlsClientCfg,
		}
	}
	if config.EnableSSL && !config.VerifySSL {
		cfg.HttpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}
	if c, err = consul.NewClient(cfg); err != nil {
		return nil, err
	}
	consulSyncer := Syncer{
		client:            c,
		logger:            logger,
		shutdownCh:        shutdownCh,
		trackedServices:   make(map[string]*consul.AgentService),
		servicesGroups:    make(map[string][]*consul.AgentServiceRegistration),
		trackedChecks:     make(map[string]*consul.AgentCheckRegistration),
		checkRunners:      make(map[string]*CheckRunner),
		periodicCallbacks: make(map[string]types.PeriodicCallback),
	}
	return &consulSyncer, nil
}

// SetDelegatedChecks sets the checks that nomad is going to run and report the
// result back to consul
func (c *Syncer) SetDelegatedChecks(delegateChecks map[string]struct{}, createDelegatedCheckFn func(*structs.ServiceCheck, string) (Check, error)) *Syncer {
	c.delegateChecks = delegateChecks
	c.createDelegatedCheck = createDelegatedCheckFn
	return c
}

// SetAddrFinder sets a function to find the host and port for a Service given its port label
func (c *Syncer) SetAddrFinder(addrFinder func(string) (string, int)) *Syncer {
	c.addrFinder = addrFinder
	return c
}

// SetServiceRegPrefix sets the registration prefix used by the Syncer when
// registering Services with Consul.
func (c *Syncer) SetServiceRegPrefix(servicePrefix string) *Syncer {
	c.registryLock.Lock()
	defer c.registryLock.Unlock()
	c.serviceRegPrefix = servicePrefix
	return c
}

// SyncNow expires the current timer forcing the list of periodic callbacks
// to be synced immediately.
func (c *Syncer) SyncNow() {
	select {
	case c.notifySyncCh <- struct{}{}:
	default:
	}
}

// SetServices assigns the slice of Nomad Services to the provided services
// group name.
func (c *Syncer) SetServices(groupName string, services []*structs.ConsulService) error {
	var mErr multierror.Error
	registeredServices := make([]*consul.AgentServiceRegistration, 0, len(services))
	for _, service := range services {
		if service.ServiceID == "" {
			service.ServiceID = c.GenerateServiceID(groupName, service)
		}
		var serviceReg *consul.AgentServiceRegistration
		var err error
		if serviceReg, err = c.createService(service); err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}
		registeredServices = append(registeredServices, serviceReg)

		// Register the check(s) for this service
		for _, chk := range service.Checks {
			// Create a Consul check registration
			chkReg, err := c.createDelegatedCheckReg(chk, serviceReg)
			if err != nil {
				mErr.Errors = append(mErr.Errors, err)
				continue
			}
			// creating a nomad check if we have to handle this particular check type
			if _, ok := c.delegateChecks[chk.Type]; ok {
				if _, ok := c.checkRunners[chkReg.ID]; ok {
					continue
				}
				nc, err := c.createDelegatedCheck(chk, chkReg.ID)
				if err != nil {
					mErr.Errors = append(mErr.Errors, err)
					continue
				}
				cr := NewCheckRunner(nc, c.runCheck, c.logger)
				c.checkRunners[nc.ID()] = cr
			}
		}
	}

	if len(mErr.Errors) > 0 {
		return mErr.ErrorOrNil()
	}

	c.servicesGroupsLock.Lock()
	c.servicesGroups[groupName] = registeredServices
	c.servicesGroupsLock.Unlock()

	return nil
}

// SyncNow expires the current timer forcing the list of periodic callbacks
// to be synced immediately.
func (c *Syncer) SyncNow() {
	select {
	case c.notifySyncCh <- struct{}{}:
	default:
	}
}

// flattenedServices returns a flattened list of services
func (c *Syncer) flattenedServices() []*consul.AgentServiceRegistration {
	const initialNumServices = 8
	services := make([]*consul.AgentServiceRegistration, 0, initialNumServices)
	c.servicesGroupsLock.RLock()
	for _, servicesGroup := range c.servicesGroups {
		for _, service := range servicesGroup {
			services = append(services, service)
		}
	}
	c.servicesGroupsLock.RUnlock()

	return services
}

func (c *Syncer) signalShutdown() {
	select {
	case c.notifyShutdownCh <- struct{}{}:
	default:
	}
}

// Shutdown de-registers the services and checks and shuts down periodic syncing
func (c *Syncer) Shutdown() error {
	var mErr multierror.Error

	c.shutdownLock.Lock()
	if !c.shutdown {
		c.shutdown = true
	}
	c.shutdownLock.Unlock()

	c.signalShutdown()

	// Stop all the checks that nomad is running
	for _, cr := range c.checkRunners {
		cr.Stop()
	}

	// De-register all the services from consul
	for _, service := range c.trackedServices {
		if err := c.client.Agent().ServiceDeregister(service.ID); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}
	return mErr.ErrorOrNil()
}

// KeepServices removes services from consul which are not present in the list
// of tasks passed to it
func (c *Syncer) KeepServices(services map[string]struct{}) error {
	var mErr multierror.Error

	// Get the services from Consul
	cServices, err := c.client.Agent().Services()
	if err != nil {
		return err
	}
	cServices = c.filterConsulServices(cServices)

	// Remove the services from consul which are not in any of the tasks
	for _, service := range cServices {
		if _, validService := services[service.ID]; !validService {
			if err := c.deregisterService(service.ID); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
		}
	}
	return mErr.ErrorOrNil()
}

// registerCheck registers a check definition with Consul
func (c *Syncer) registerCheck(chkReg *consul.AgentCheckRegistration) error {
	if cr, ok := c.checkRunners[chkReg.ID]; ok {
		cr.Start()
	}
	return c.client.Agent().CheckRegister(chkReg)
}

// createDelegatedCheckReg creates a Check that can be registered with
// Nomad. It also creates a Nomad check for the check types that it can
// handle.
func (c *Syncer) createDelegatedCheckReg(check *structs.ServiceCheck, service *consul.AgentService) (*consul.AgentCheckRegistration, error) {
	chkReg := consul.AgentCheckRegistration{
		ID:        check.Hash(service.ID),
		Name:      check.Name,
		ServiceID: service.ID,
	}
	chkReg.Timeout = check.Timeout.String()
	chkReg.Interval = check.Interval.String()
	switch check.Type {
	case structs.ServiceCheckHTTP:
		if check.Protocol == "" {
			check.Protocol = "http"
		}
		url := url.URL{
			Scheme: check.Protocol,
			Host:   fmt.Sprintf("%s:%d", service.Address, service.Port),
			Path:   check.Path,
		}
		chkReg.HTTP = url.String()
	case structs.ServiceCheckTCP:
		chkReg.TCP = fmt.Sprintf("%s:%d", service.Address, service.Port)
	case structs.ServiceCheckScript:
		chkReg.TTL = (check.Interval + ttlCheckBuffer).String()
	default:
		return nil, fmt.Errorf("check type %q not valid", check.Type)
	}
	return &chkReg, nil
}

// createService creates a Consul AgentService from a Nomad Service
func (c *Syncer) createService(service *structs.ConsulService) (*consul.AgentServiceRegistration, error) {
	c.registryLock.RLock()
	defer c.registryLock.RUnlock()

	srv := consul.AgentServiceRegistration{
		ID:   service.ID(c.serviceRegPrefix),
		Name: service.Name,
		Tags: service.Tags,
	}
	host, port := c.addrFinder(service.PortLabel)
	if host != "" {
		srv.Address = host
	}

	if port != 0 {
		srv.Port = port
	}

	return &srv, nil
}

// deregisterService de-registers a service with the given ID from consul
func (c *Syncer) deregisterService(ID string) error {
	return c.client.Agent().ServiceDeregister(ID)
}

// deregisterCheck de-registers a check with a given ID from Consul.
func (c *Syncer) deregisterCheck(ID string) error {
	// Deleting the nomad check
	if cr, ok := c.checkRunners[ID]; ok {
		cr.Stop()
		delete(c.checkRunners, ID)
	}

	// Deleting from consul
	return c.client.Agent().CheckDeregister(ID)
}

// Run triggers periodic syncing of services and checks with Consul.  This is
// a long lived go-routine which is stopped during shutdown.
func (c *Syncer) Run() {
	d := initialSyncDelay + lib.RandomStagger(initialSyncBuffer-initialSyncDelay)
	sync := time.NewTimer(d)
	c.logger.Printf("[DEBUG] consul.sync: sleeping %v before first sync", d)

	for {
		select {
		case <-sync.C:
			d = syncInterval - lib.RandomStagger(syncInterval/syncJitter)
			sync.Reset(d)

			if err := c.performSync(); err != nil {
				if c.runChecks {
					c.logger.Printf("[DEBUG] consul.sync: disabling checks until Consul sync completes for %q: %v", c.serviceRegPrefix, err)
				}
				c.runChecks = false
			} else {
				c.runChecks = true
			}
		case <-c.notifySyncCh:
			sync.Reset(syncInterval)
		case <-c.shutdownCh:
			c.Shutdown()
		case <-c.notifyShutdownCh:
			sync.Stop()
			c.logger.Printf("[INFO] consul.syncer: shutting down sync for %q", c.serviceRegPrefix)
			return
		}
	}
}

// RunHandlers executes each handler (randomly)
func (c *Syncer) RunHandlers() error {
	c.periodicLock.RLock()
	handlers := make(map[string]types.PeriodicCallback, len(c.periodicCallbacks))
	for name, fn := range c.periodicCallbacks {
		handlers[name] = fn
	}
	c.periodicLock.RUnlock()

	var mErr multierror.Error
	for _, fn := range handlers {
		if err := fn(); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}
	return mErr.ErrorOrNil()
}

// performSync sync the services and checks we are tracking with Consul.
func (c *Syncer) performSync() error {
	var mErr multierror.Error
	if err := c.RunHandlers(); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}
	if err := c.syncServices(); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}
	if err := c.syncChecks(); err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}

	return mErr.ErrorOrNil()
}

// filterConsulServices prunes out all the service whose ids are not prefixed
// with nomad-
func (c *Syncer) filterConsulServices(consulServices map[string]*consul.AgentService) map[string]*consul.AgentService {
	localServices := make(map[string]*consul.AgentService, len(consulServices))
	c.registryLock.RLock()
	defer c.registryLock.RUnlock()
	for serviceID, service := range consulServices {
		if strings.HasPrefix(service.ID, c.serviceRegPrefix) {
			localServices[serviceID] = service
		}
	}
	return localServices
}

// filterConsulChecks prunes out all the consul checks which do not have
// services with id prefixed with noamd-
func (c *Syncer) filterConsulChecks(chks map[string]*consul.AgentCheck) map[string]*consul.AgentCheck {
	nomadChecks := make(map[string]*consul.AgentCheck)
	for _, chk := range chks {
		if strings.HasPrefix(chk.ServiceID, structs.NomadConsulPrefix) {
			nomadChecks[chk.CheckID] = chk
		}
	}
	return nomadChecks
}

// consulPresent indicates whether the consul agent is responding
func (c *Syncer) consulPresent() bool {
	_, err := c.client.Agent().Self()
	return err == nil
}

// runCheck runs a check and updates the corresponding ttl check in consul
func (c *Syncer) runCheck(check Check) {
	res := check.Run()
	if res.Duration >= check.Timeout() {
		c.logger.Printf("[DEBUG] consul.syncer: check took time: %v, timeout: %v", res.Duration, check.Timeout())
	}
	state := consul.HealthCritical
	output := res.Output
	switch res.ExitCode {
	case 0:
		state = consul.HealthPassing
	case 1:
		state = consul.HealthWarning
	default:
		state = consul.HealthCritical
	}
	if res.Err != nil {
		state = consul.HealthCritical
		output = res.Err.Error()
	}
	if err := c.client.Agent().UpdateTTL(check.ID(), output, state); err != nil {
		if c.runChecks {
			c.logger.Printf("[DEBUG] consul.syncer: check %q failed, disabling Consul checks until until next successful sync: %v", check.ID(), err)
			c.runChecks = false
		} else {
			c.runChecks = true
		}
	}
}

// GenerateServicePrefix returns a service prefix based on an allocation id
// and task name.
func GenerateServicePrefix(allocID string, taskName string) string {
	return fmt.Sprintf("%s-%s", taskName, allocID)
}

// AddPeriodicHandler adds a uniquely named callback.  Returns true if
// successful, false if a handler with the same name already exists.
func (c *Syncer) AddPeriodicHandler(name string, fn types.PeriodicCallback) bool {
	c.periodicLock.Lock()
	defer c.periodicLock.Unlock()
	if _, found := c.periodicCallbacks[name]; found {
		c.logger.Printf("[ERROR] consul.syncer: failed adding handler %q", name)
		return false
	}
	c.periodicCallbacks[name] = fn
	return true
}

func (c *Syncer) NumHandlers() int {
	c.periodicLock.RLock()
	defer c.periodicLock.RUnlock()
	return len(c.periodicCallbacks)
}

// RemovePeriodicHandler removes a handler with a given name.
func (c *Syncer) RemovePeriodicHandler(name string) {
	c.periodicLock.Lock()
	defer c.periodicLock.Unlock()
	delete(c.periodicCallbacks, name)
}

func (c *Syncer) ConsulClient() *consul.Client {
	return c.client
}
