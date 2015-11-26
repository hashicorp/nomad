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

type trackedTask struct {
	allocID string
	task    *structs.Task
}

type ConsulService struct {
	client     *consul.Client
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
		client:        c,
		logger:        logger,
		trackedTasks:  make(map[string]*trackedTask),
		serviceStates: make(map[string]string),
		shutdownCh:    make(chan struct{}),
	}

	return &consulService, nil
}

// Starts tracking a task for changes to it's services and tasks
func (c *ConsulService) Register(task *structs.Task, allocID string) error {
	var mErr multierror.Error
	c.trackedTskLock.Lock()
	tt := &trackedTask{allocID: allocID, task: task}
	c.trackedTasks[fmt.Sprintf("%s-%s", allocID, task.Name)] = tt
	c.trackedTskLock.Unlock()
	for _, service := range task.Services {
		c.logger.Printf("[INFO] consul: Registering service %s with Consul.", service.Name)
		if err := c.registerService(service, task, allocID); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	return mErr.ErrorOrNil()
}

// Stops tracking a task for changes to it's services and checks
func (c *ConsulService) Deregister(task *structs.Task, allocID string) error {
	var mErr multierror.Error
	c.trackedTskLock.Lock()
	delete(c.trackedTasks, fmt.Sprintf("%s-%s", allocID, task.Name))
	c.trackedTskLock.Unlock()
	for _, service := range task.Services {
		if service.Id == "" {
			continue
		}
		c.logger.Printf("[INFO] consul: De-Registering service %v with Consul", service.Name)
		if err := c.deregisterService(service.Id); err != nil {
			c.logger.Printf("[DEBUG] consul: Error in de-registering service %v from Consul", service.Name)
			mErr.Errors = append(mErr.Errors, err)
		}
	}
	return mErr.ErrorOrNil()
}

func (c *ConsulService) ShutDown() {
	close(c.shutdownCh)
}

// Performs calls to sync checks and services periodically
func (c *ConsulService) SyncWithConsul() {
	sync := time.After(syncInterval)
	agent := c.client.Agent()

	for {
		select {
		case <-sync:
			c.performSync(agent)
			sync = time.After(syncInterval)
		case <-c.shutdownCh:
			c.logger.Printf("[INFO] Shutting down Consul Client")
			return
		}
	}
}

// Sync checks and services with Consul
func (c *ConsulService) performSync(agent *consul.Agent) (int, int) {
	// Get the list of the services and that Consul knows about
	consulServices, _ := agent.Services()
	consulChecks, _ := agent.Checks()
	delete(consulServices, "consul")

	knownChecks := make(map[string]struct{})
	knownServices := make(map[string]struct{})

	// Add services and checks which Consul doesn't know about
	for _, trackedTask := range c.trackedTasks {
		for _, service := range trackedTask.task.Services {
			knownServices[service.Id] = struct{}{}
			if _, ok := consulServices[service.Id]; !ok {
				c.registerService(service, trackedTask.task, trackedTask.allocID)
				continue
			}

			if service.Hash() != c.serviceStates[service.Id] {
				c.registerService(service, trackedTask.task, trackedTask.allocID)
				continue
			}
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

	// Remove services that are not present anymore
	for _, consulService := range consulServices {
		if _, ok := knownServices[consulService.ID]; !ok {
			delete(c.serviceStates, consulService.ID)
			c.deregisterService(consulService.ID)
		}
	}

	// Remove checks that are not present anymore
	for _, consulCheck := range consulChecks {
		if _, ok := knownChecks[consulCheck.CheckID]; !ok {
			c.deregisterCheck(consulCheck.CheckID)
		}
	}

	return len(c.serviceStates), len(knownChecks)
}

// Registers a Service with Consul
func (c *ConsulService) registerService(service *structs.Service, task *structs.Task, allocID string) error {
	var mErr multierror.Error
	service.Id = fmt.Sprintf("%s-%s", allocID, service.Name)
	host, port := task.FindHostAndPortFor(service.PortLabel)
	if host == "" || port == 0 {
		return fmt.Errorf("consul: The port:%s marked for registration of service: %s couldn't be found", service.PortLabel, service.Name)
	}
	c.serviceStates[service.Id] = service.Hash()

	asr := &consul.AgentServiceRegistration{
		ID:      service.Id,
		Name:    service.Name,
		Tags:    service.Tags,
		Port:    port,
		Address: host,
	}

	if err := c.client.Agent().ServiceRegister(asr); err != nil {
		c.logger.Printf("[DEBUG] consul: Error while registering service %v with Consul: %v", service.Name, err)
		mErr.Errors = append(mErr.Errors, err)
	}
	for _, check := range service.Checks {
		cr := c.makeCheck(service, check, host, port)
		if err := c.registerCheck(cr); err != nil {
			c.logger.Printf("[ERROR] consul: Error while registerting check %v with Consul: %v", check.Name, err)
			mErr.Errors = append(mErr.Errors, err)
		}

	}
	return mErr.ErrorOrNil()
}

// Registers a check with Consul
func (c *ConsulService) registerCheck(check *consul.AgentCheckRegistration) error {
	c.logger.Printf("[DEBUG] Registering Check with ID: %v for Service: %v", check.ID, check.ServiceID)
	return c.client.Agent().CheckRegister(check)
}

// Deregisters a check with a specific ID from Consul
func (c *ConsulService) deregisterCheck(checkID string) error {
	c.logger.Printf("[DEBUG] Removing check with ID: %v", checkID)
	return c.client.Agent().CheckDeregister(checkID)
}

// De-Registers a Service with a specific id from Consul
func (c *ConsulService) deregisterService(serviceId string) error {
	delete(c.serviceStates, serviceId)
	if err := c.client.Agent().ServiceDeregister(serviceId); err != nil {
		return err
	}
	return nil
}

// Creates a Consul Check Registration struct
func (c *ConsulService) makeCheck(service *structs.Service, check *structs.ServiceCheck, ip string, port int) *consul.AgentCheckRegistration {
	if check.Name == "" {
		check.Name = fmt.Sprintf("service: %q%s%q check", service.Name)
	}
	check.Id = check.Hash(service.Id)

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
