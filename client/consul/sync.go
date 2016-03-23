package consul

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	consul "github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/nomad/structs"
)

// ConsulService allows syncing of services and checks with Consul
type ConsulService struct {
	client *consul.Client

	task *structs.Task

	services map[string]*consul.AgentService
	checks   map[string]*structs.ServiceCheck

	logger     *log.Logger
	shutdownCh chan struct{}
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
func NewConsulService(config *ConsulConfig, logger *log.Logger) (*ConsulService, error) {
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
		client:   c,
		logger:   logger,
		services: make(map[string]*consul.AgentService),
		checks:   make(map[string]*structs.ServiceCheck),

		shutdownCh: make(chan struct{}),
	}
	return &consulService, nil
}

// SyncTask sync the services and task with consul
func (c *ConsulService) SyncTask(task *structs.Task) error {
	var mErr multierror.Error
	c.task = task
	services := make(map[string]*consul.AgentService)
	checks := make(map[string]*structs.ServiceCheck)

	// Register Services and Checks that we don't know about or has changed
	for _, service := range task.Services {
		srv, err := c.createService(service)
		if err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}
		trackedService, ok := c.services[srv.ID]
		if (ok && !reflect.DeepEqual(trackedService, srv)) || !ok {
			if err := c.registerService(srv); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
		}
		c.services[srv.ID] = srv
		services[srv.ID] = srv

		for _, chk := range service.Checks {
			if _, ok := c.checks[chk.ID]; !ok {
				if err := c.registerCheck(chk, srv); err != nil {
					mErr.Errors = append(mErr.Errors, err)
				}
			}
			c.checks[chk.ID] = chk
			checks[chk.ID] = chk
		}
	}

	// Remove services that are not present anymore
	for _, service := range c.services {
		if _, ok := services[service.ID]; !ok {
			if err := c.deregisterService(service.ID); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
			delete(c.services, service.ID)
		}
	}

	// Remove the checks that are not present anymore
	for _, check := range c.checks {
		if _, ok := checks[check.ID]; !ok {
			if err := c.deregisterCheck(check.ID); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
			delete(c.checks, check.ID)
		}
	}
	return mErr.ErrorOrNil()
}

// Shutdown de-registers the services and checks and shuts down periodic syncing
func (c *ConsulService) Shutdown() error {
	var mErr multierror.Error
	select {
	case c.shutdownCh <- struct{}{}:
	default:
	}
	for _, service := range c.services {
		if err := c.client.Agent().ServiceDeregister(service.ID); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}
	return mErr.ErrorOrNil()
}

func (c *ConsulService) RemoveServices(tasks []*structs.Task) error {
	var mErr multierror.Error
	var services map[string]struct{}
	for _, task := range tasks {
		for _, service := range task.Services {
			services[service.ID] = struct{}{}
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
		ID:        check.ID,
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
		ID:      service.ID,
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

// SyncWithConsul triggers periodic syncing of services and checks with Consul
func (c *ConsulService) SyncWithConsul() {
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
	for serviceID, service := range c.services {
		if _, ok := cServices[serviceID]; !ok {
			if err := c.registerService(service); err != nil {
				mErr.Errors = append(mErr.Errors, err)
			}
		}
	}
	for checkID, check := range c.checks {
		if chk, ok := cChecks[checkID]; !ok {
			if err := c.registerCheck(check, c.services[chk.ServiceID]); err != nil {
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
	delete(srvcs, "consul")
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
