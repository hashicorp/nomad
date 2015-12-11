package client

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	consul "github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	syncInterval = 5 * time.Second
)

// consulApi is the interface which wraps the actual consul api client
type consulApi interface {
	CheckRegister(check *consul.AgentCheckRegistration) error
	CheckDeregister(checkID string) error
	ServiceRegister(service *consul.AgentServiceRegistration) error
	ServiceDeregister(ServiceID string) error
	Services() (map[string]*consul.AgentService, error)
	Checks() (map[string]*consul.AgentCheck, error)
}

// consulApiClient is the actual implementation of the consulApi which
// talks to the consul agent
type consulApiClient struct {
	client *consul.Client
}

func (a *consulApiClient) CheckRegister(check *consul.AgentCheckRegistration) error {
	return a.client.Agent().CheckRegister(check)
}

func (a *consulApiClient) CheckDeregister(checkID string) error {
	return a.client.Agent().CheckDeregister(checkID)
}

func (a *consulApiClient) ServiceRegister(service *consul.AgentServiceRegistration) error {
	return a.client.Agent().ServiceRegister(service)
}

func (a *consulApiClient) ServiceDeregister(serviceId string) error {
	return a.client.Agent().ServiceDeregister(serviceId)
}

func (a *consulApiClient) Services() (map[string]*consul.AgentService, error) {
	return a.client.Agent().Services()
}

func (a *consulApiClient) Checks() (map[string]*consul.AgentCheck, error) {
	return a.client.Agent().Checks()
}

// trackedTask is a Task that we are tracking for changes in service and check
// definitions and keep them sycned with Consul Agent
type trackedTask struct {
	allocID string
	task    *structs.Task
}

// ConsulService is the service which tracks tasks and syncs the services and
// checks defined in them with Consul Agent
type ConsulService struct {
	client     consulApi
	logger     *log.Logger
	shutdownCh chan struct{}

	trackedTasks   map[string]*trackedTask
	serviceStates  map[string]string
	trackedTskLock sync.Mutex
}

// A factory method to create new consul service
func NewConsulService(logger *log.Logger, consulAddr string, token string,
	auth string, enableSSL bool, verifySSL bool) (*ConsulService, error) {
	var err error
	var c *consul.Client
	cfg := consul.DefaultConfig()
	cfg.Address = consulAddr
	if token != "" {
		cfg.Token = token
	}

	if auth != "" {
		var username, password string
		if strings.Contains(auth, ":") {
			split := strings.SplitN(auth, ":", 2)
			username = split[0]
			password = split[1]
		} else {
			username = auth
		}

		cfg.HttpAuth = &consul.HttpBasicAuth{
			Username: username,
			Password: password,
		}
	}
	if enableSSL {
		cfg.Scheme = "https"
	}
	if enableSSL && !verifySSL {
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
		client:        &consulApiClient{client: c},
		logger:        logger,
		trackedTasks:  make(map[string]*trackedTask),
		serviceStates: make(map[string]string),
		shutdownCh:    make(chan struct{}),
	}

	return &consulService, nil
}

// Register starts tracking a task for changes to it's services and tasks and
// adds/removes services and checks associated with it.
func (c *ConsulService) Register(task *structs.Task, allocID string) error {
	var mErr multierror.Error
	c.trackedTskLock.Lock()
	tt := &trackedTask{allocID: allocID, task: task}
	c.trackedTasks[fmt.Sprintf("%s-%s", allocID, task.Name)] = tt
	c.trackedTskLock.Unlock()
	for _, service := range task.Services {
		c.logger.Printf("[INFO] consul: registering service %s with consul.", service.Name)
		if err := c.registerService(service, task, allocID); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	return mErr.ErrorOrNil()
}

// Deregister stops tracking a task for changes to it's services and checks and
// removes all the services and checks associated with the Task
func (c *ConsulService) Deregister(task *structs.Task, allocID string) error {
	var mErr multierror.Error
	c.trackedTskLock.Lock()
	delete(c.trackedTasks, fmt.Sprintf("%s-%s", allocID, task.Name))
	c.trackedTskLock.Unlock()
	for _, service := range task.Services {
		if service.Id == "" {
			continue
		}
		c.logger.Printf("[INFO] consul: deregistering service %v from consul", service.Name)
		if err := c.deregisterService(service.Id); err != nil {
			c.logger.Printf("[DEBUG] consul: error in deregistering service %v from consul", service.Name)
			mErr.Errors = append(mErr.Errors, err)
		}
	}
	return mErr.ErrorOrNil()
}

func (c *ConsulService) ShutDown() {
	close(c.shutdownCh)
}

// SyncWithConsul is a long lived function that performs calls to sync
// checks and services periodically with Consul Agent
func (c *ConsulService) SyncWithConsul() {
	sync := time.After(syncInterval)

	for {
		select {
		case <-sync:
			c.performSync()
			sync = time.After(syncInterval)
		case <-c.shutdownCh:
			c.logger.Printf("[INFO] consul: shutting down consul service")
			return
		}
	}
}

// performSync syncs checks and services with Consul and removes tracked
// services which are no longer present in tasks
func (c *ConsulService) performSync() {
	// Get the list of the services and that Consul knows about
	consulServices, _ := c.client.Services()
	consulChecks, _ := c.client.Checks()
	delete(consulServices, "consul")

	knownChecks := make(map[string]struct{})
	knownServices := make(map[string]struct{})

	// Add services and checks which Consul doesn't know about
	for _, trackedTask := range c.trackedTasks {
		for _, service := range trackedTask.task.Services {

			// Add new services which Consul agent isn't aware of
			knownServices[service.Id] = struct{}{}
			if _, ok := consulServices[service.Id]; !ok {
				c.logger.Printf("[INFO] consul: registering service %s with consul.", service.Name)
				c.registerService(service, trackedTask.task, trackedTask.allocID)
				continue
			}

			// If a service has changed, re-register it with Consul agent
			if service.Hash() != c.serviceStates[service.Id] {
				c.logger.Printf("[INFO] consul: reregistering service %s with consul.", service.Name)
				c.registerService(service, trackedTask.task, trackedTask.allocID)
				continue
			}

			// Add new checks that Consul isn't aware of
			for _, check := range service.Checks {
				knownChecks[check.Id] = struct{}{}
				if _, ok := consulChecks[check.Id]; !ok {
					host, port := trackedTask.task.FindHostAndPortFor(service.PortLabel)
					cr := c.makeCheck(service, check, host, port)
					c.registerCheck(cr)
				}
			}
		}
	}

	// Remove services from the service tracker which no longer exists
	for serviceId := range c.serviceStates {
		if _, ok := knownServices[serviceId]; !ok {
			delete(c.serviceStates, serviceId)
		}
	}
}

// registerService registers a Service with Consul
func (c *ConsulService) registerService(service *structs.Service, task *structs.Task, allocID string) error {
	var mErr multierror.Error
	host, port := task.FindHostAndPortFor(service.PortLabel)
	if host == "" || port == 0 {
		return fmt.Errorf("consul: the port:%q marked for registration of service: %q couldn't be found", service.PortLabel, service.Name)
	}
	c.serviceStates[service.Id] = service.Hash()

	asr := &consul.AgentServiceRegistration{
		ID:      service.Id,
		Name:    service.Name,
		Tags:    service.Tags,
		Port:    port,
		Address: host,
	}

	if err := c.client.ServiceRegister(asr); err != nil {
		c.logger.Printf("[DEBUG] consul: error while registering service %v with consul: %v", service.Name, err)
		mErr.Errors = append(mErr.Errors, err)
	}
	for _, check := range service.Checks {
		cr := c.makeCheck(service, check, host, port)
		if err := c.registerCheck(cr); err != nil {
			c.logger.Printf("[DEBUG] consul: error while registerting check %v with consul: %v", check.Name, err)
			mErr.Errors = append(mErr.Errors, err)
		}

	}
	return mErr.ErrorOrNil()
}

// registerCheck registers a check with Consul
func (c *ConsulService) registerCheck(check *consul.AgentCheckRegistration) error {
	c.logger.Printf("[INFO] consul: registering Check with ID: %v for service: %v", check.ID, check.ServiceID)
	return c.client.CheckRegister(check)
}

// deregisterCheck de-registers a check with a specific ID from Consul
func (c *ConsulService) deregisterCheck(checkID string) error {
	c.logger.Printf("[INFO] consul: removing check with ID: %v", checkID)
	return c.client.CheckDeregister(checkID)
}

// deregisterService de-registers a Service with a specific id from Consul
func (c *ConsulService) deregisterService(serviceId string) error {
	delete(c.serviceStates, serviceId)
	if err := c.client.ServiceDeregister(serviceId); err != nil {
		return err
	}
	return nil
}

// makeCheck creates a Consul Check Registration struct
func (c *ConsulService) makeCheck(service *structs.Service, check *structs.ServiceCheck, ip string, port int) *consul.AgentCheckRegistration {
	cr := &consul.AgentCheckRegistration{
		ID:        check.Id,
		Name:      check.Name,
		ServiceID: service.Id,
	}
	cr.Interval = check.Interval.String()
	cr.Timeout = check.Timeout.String()

	switch check.Type {
	case structs.ServiceCheckHTTP:
		if check.Protocol == "" {
			check.Protocol = "http"
		}
		url := url.URL{
			Scheme: check.Protocol,
			Host:   fmt.Sprintf("%s:%d", ip, port),
			Path:   check.Path,
		}
		cr.HTTP = url.String()
	case structs.ServiceCheckTCP:
		cr.TCP = fmt.Sprintf("%s:%d", ip, port)
	case structs.ServiceCheckScript:
		cr.Script = check.Script // TODO This needs to include the path of the alloc dir and based on driver types
	}
	return cr
}
