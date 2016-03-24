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
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/nomad/structs"
)

// ConsulService allows syncing of services and checks with Consul
type ConsulService struct {
	client *consul.Client

	task    *structs.Task
	allocID string

	trackedServices map[string]*consul.AgentService
	trackedChecks   map[string]*structs.ServiceCheck

	logger *log.Logger

	shutdownCh   chan struct{}
	shutdown     bool
	shutdownLock sync.Mutex
}

// ConsulConfig is the configuration used to create a new ConsulService client
type ConsulConfig struct {
	Addr      string
	Token     string
	Auth      string
	EnableSSL bool
	VerifySSL bool
}

const (
	// The periodic time interval for syncing services and checks with Consul
	syncInterval = 5 * time.Second
)

// NewConsulService returns a new ConsulService
func NewConsulService(config *ConsulConfig, logger *log.Logger, allocID string) (*ConsulService, error) {
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
	consulService := ConsulService{
		client:          c,
		logger:          logger,
		trackedServices: make(map[string]*consul.AgentService),
		trackedChecks:   make(map[string]*structs.ServiceCheck),

		shutdownCh: make(chan struct{}),
	}
	return &consulService, nil
}

// SyncTask sync the services and task with consul
func (c *ConsulService) SyncTask(task *structs.Task) error {
	var mErr multierror.Error
	c.task = task
	taskServices := make(map[string]*consul.AgentService)
	taskChecks := make(map[string]*structs.ServiceCheck)

	// Register Services and Checks that we don't know about or has changed
	for _, service := range task.Services {
		srv, err := c.createService(service)
		if err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}
		trackedService, ok := c.trackedServices[srv.ID]
		if (ok && !reflect.DeepEqual(trackedService, srv)) || !ok {
			if err := c.registerService(srv); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
		}
		c.trackedServices[srv.ID] = srv
		taskServices[srv.ID] = srv

		for _, chk := range service.Checks {
			checkID := chk.Hash(srv.ID)
			if _, ok := c.trackedChecks[checkID]; !ok {
				if err := c.registerCheck(chk, srv); err != nil {
					mErr.Errors = append(mErr.Errors, err)
				}
			}
			c.trackedChecks[checkID] = chk
			taskChecks[checkID] = chk
		}
	}

	// Remove services that are not present anymore
	for _, service := range c.trackedServices {
		if _, ok := taskServices[service.ID]; !ok {
			if err := c.deregisterService(service.ID); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
			delete(c.trackedServices, service.ID)
		}
	}

	// Remove the checks that are not present anymore
	for checkID, _ := range c.trackedChecks {
		if _, ok := taskChecks[checkID]; !ok {
			if err := c.deregisterCheck(checkID); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
			delete(c.trackedChecks, checkID)
		}
	}
	return mErr.ErrorOrNil()
}

// Shutdown de-registers the services and checks and shuts down periodic syncing
func (c *ConsulService) Shutdown() error {
	var mErr multierror.Error

	c.shutdownLock.Lock()
	if !c.shutdown {
		close(c.shutdownCh)
		c.shutdown = true
	}
	c.shutdownLock.Unlock()
	for _, service := range c.trackedServices {
		if err := c.client.Agent().ServiceDeregister(service.ID); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}
	return mErr.ErrorOrNil()
}

// KeepServices removes services from consul which are not present in the list
// of tasks passed to it
func (c *ConsulService) KeepServices(tasks []*structs.Task) error {
	var mErr multierror.Error
	var services map[string]struct{}
	for _, task := range tasks {
		for _, service := range task.Services {
			services[service.ID(c.allocID, c.task.Name)] = struct{}{}
		}
	}

	cServices, err := c.client.Agent().Services()
	if err != nil {
		return err
	}
	cServices = c.filterConsulServices(cServices)

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
func (c *ConsulService) registerCheck(check *structs.ServiceCheck, service *consul.AgentService) error {
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
		chkReg.TTL = check.Interval.String()
	}
	return c.client.Agent().CheckRegister(&chkReg)
}

// createService creates a Consul AgentService from a Nomad Service
func (c *ConsulService) createService(service *structs.Service) (*consul.AgentService, error) {
	host, port := c.task.FindHostAndPortFor(service.PortLabel)
	if host == "" {
		return nil, fmt.Errorf("host for the service %q  couldn't be found", service.Name)
	}

	if port == 0 {
		return nil, fmt.Errorf("port for the service %q  couldn't be found", service.Name)
	}
	srv := consul.AgentService{
		ID:      service.ID(c.allocID, c.task.Name),
		Service: service.Name,
		Tags:    service.Tags,
		Address: host,
		Port:    port,
	}
	return &srv, nil
}

// registerService registers a service with Consul
func (c *ConsulService) registerService(service *consul.AgentService) error {
	srvReg := consul.AgentServiceRegistration{
		ID:      service.ID,
		Name:    service.Service,
		Tags:    service.Tags,
		Port:    service.Port,
		Address: service.Address,
	}
	return c.client.Agent().ServiceRegister(&srvReg)
}

// deregisterService de-registers a service with the given ID from consul
func (c *ConsulService) deregisterService(ID string) error {
	return c.client.Agent().ServiceDeregister(ID)
}

// deregisterCheck de-registers a check with a given ID from Consul.
func (c *ConsulService) deregisterCheck(ID string) error {
	return c.client.Agent().CheckDeregister(ID)
}

// PeriodicSync triggers periodic syncing of services and checks with Consul
func (c *ConsulService) PeriodicSync() {
	sync := time.After(syncInterval)
	for {
		select {
		case <-sync:
			if err := c.performSync(); err != nil {
				c.logger.Printf("[DEBUG] consul: error in syncing task %q: %v", c.task.Name, err)
			}
			sync = time.After(syncInterval)
		case <-c.shutdownCh:
			c.logger.Printf("[INFO] consul: shutting down sync for task %q", c.task.Name)
			return
		}
	}
}

// performSync sync the services and checks we are tracking with Consul.
func (c *ConsulService) performSync() error {
	var mErr multierror.Error
	cServices, err := c.client.Agent().Services()
	if err != nil {
		return err
	}

	cChecks, err := c.client.Agent().Checks()
	if err != nil {
		return err
	}

	// Add services and checks that consul doesn't have but we do
	for serviceID, service := range c.trackedServices {
		if _, ok := cServices[serviceID]; !ok {
			if err := c.registerService(service); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
		}
	}
	for checkID, check := range c.trackedChecks {
		if chk, ok := cChecks[checkID]; !ok {
			if err := c.registerCheck(check, c.trackedServices[chk.ServiceID]); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
		}
	}

	return mErr.ErrorOrNil()
}

// filterConsulServices prunes out all the service whose ids are not prefixed
// with nomad-
func (c *ConsulService) filterConsulServices(srvcs map[string]*consul.AgentService) map[string]*consul.AgentService {
	nomadServices := make(map[string]*consul.AgentService)
	for _, srv := range srvcs {
		if strings.HasPrefix(srv.ID, structs.NomadConsulPrefix) {
			nomadServices[srv.ID] = srv
		}
	}
	return nomadServices
}

// filterConsulChecks prunes out all the consul checks which do not have
// services with id prefixed with noamd-
func (c *ConsulService) filterConsulChecks(chks map[string]*consul.AgentCheck) map[string]*consul.AgentCheck {
	nomadChecks := make(map[string]*consul.AgentCheck)
	for _, chk := range chks {
		if strings.HasPrefix(chk.ServiceID, structs.NomadConsulPrefix) {
			nomadChecks[chk.CheckID] = chk
		}
	}
	return nomadChecks
}

func (c *ConsulService) consulPresent() bool {
	_, err := c.client.Agent().Self()
	return err == nil
}
