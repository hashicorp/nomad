package client

import (
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	consul "github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	syncInterval = 5 * time.Second
)

type trackedService struct {
	allocId     string
	task        *structs.Task
	serviceHash string
	service     *structs.Service
	host        string
	port        int
}

type trackedTask struct {
	allocID string
	task    *structs.Task
}

func (t *trackedService) IsServiceValid() bool {
	for _, service := range t.task.Services {
		if service.Id == t.service.Id && service.Hash() == t.serviceHash {
			return true
		}
	}

	return false
}

type ConsulService struct {
	client     *consul.Client
	logger     *log.Logger
	shutdownCh chan struct{}

	trackedServices map[string]*trackedService                // Service ID to Tracked Service Map
	trackedChecks   map[string]*consul.AgentCheckRegistration // List of check ids that is being tracked
	trackedTasks    map[string]*trackedTask
	trackedSrvLock  sync.Mutex
	trackedChkLock  sync.Mutex
	trackedTskLock  sync.Mutex
}

func NewConsulService(logger *log.Logger, consulAddr string) (*ConsulService, error) {
	var err error
	var c *consul.Client
	cfg := consul.DefaultConfig()
	cfg.Address = consulAddr
	if c, err = consul.NewClient(cfg); err != nil {
		return nil, err
	}

	consulService := ConsulService{
		client:          c,
		logger:          logger,
		trackedServices: make(map[string]*trackedService),
		trackedTasks:    make(map[string]*trackedTask),
		trackedChecks:   make(map[string]*consul.AgentCheckRegistration),
		shutdownCh:      make(chan struct{}),
	}

	return &consulService, nil
}

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

func (c *ConsulService) performSync(agent *consul.Agent) {
	var consulServices map[string]*consul.AgentService
	var consulChecks map[string]*consul.AgentCheck

	// Remove the tracked services which tasks no longer references
	for serviceId, ts := range c.trackedServices {
		if !ts.IsServiceValid() {
			c.logger.Printf("[DEBUG] consul: Removing service: %s since the task doesn't have it anymore", ts.service.Name)
			c.deregisterService(serviceId)
		}
	}

	// Add additional services that we might not have added from tasks
	for _, trackedTask := range c.trackedTasks {
		for _, service := range trackedTask.task.Services {
			if _, ok := c.trackedServices[service.Id]; !ok {
				c.registerService(service, trackedTask.task, trackedTask.allocID)
			}
		}
	}

	// Get the list of the services that Consul knows about
	consulServices, _ = agent.Services()

	// See if we have services that Consul doesn't know about yet.
	// Register with Consul the services which are not registered
	for serviceId := range c.trackedServices {
		if _, ok := consulServices[serviceId]; !ok {
			ts := c.trackedServices[serviceId]
			c.registerService(ts.service, ts.task, ts.allocId)
		}
	}

	// See if consul thinks we have some services which are not running
	// anymore on the node. We de-register those services
	for serviceId := range consulServices {
		if serviceId == "consul" {
			continue
		}
		if _, ok := c.trackedServices[serviceId]; !ok {
			if err := c.deregisterService(serviceId); err != nil {
				c.logger.Printf("[DEBUG] consul: Error while de-registering service with ID: %s", serviceId)
			}
		}
	}

	consulChecks, _ = agent.Checks()

	// Remove checks that Consul knows about but we don't
	for checkID := range consulChecks {
		if _, ok := c.trackedChecks[checkID]; !ok {
			c.deregisterCheck(checkID)
		}
	}

	// Add checks that might not be present
	for _, ts := range c.trackedServices {
		checks := c.makeChecks(ts.service, ts.host, ts.port)
		for _, check := range checks {
			if _, ok := consulChecks[check.ID]; !ok {
				c.registerCheck(check)
			}
		}

	}
}

func (c *ConsulService) registerService(service *structs.Service, task *structs.Task, allocID string) error {
	var mErr multierror.Error
	service.Id = fmt.Sprintf("%s-%s", allocID, service.Name)
	host, port := task.FindHostAndPortFor(service.PortLabel)
	if host == "" || port == 0 {
		return fmt.Errorf("consul: The port:%s marked for registration of service: %s couldn't be found", service.PortLabel, service.Name)
	}
	ts := &trackedService{
		allocId:     allocID,
		task:        task,
		serviceHash: service.Hash(),
		service:     service,
		host:        host,
		port:        port,
	}
	c.trackedSrvLock.Lock()
	c.trackedServices[service.Id] = ts
	c.trackedSrvLock.Unlock()

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
	checks := c.makeChecks(service, host, port)
	for _, check := range checks {
		if err := c.registerCheck(check); err != nil {
			c.logger.Printf("[ERROR] consul: Error while registerting check %v with Consul: %v", check.Name, err)
			mErr.Errors = append(mErr.Errors, err)
		}
	}
	return mErr.ErrorOrNil()
}

func (c *ConsulService) registerCheck(check *consul.AgentCheckRegistration) error {
	c.logger.Printf("[DEBUG] Registering Check with ID: %v for Service: %v", check.ID, check.ServiceID)
	c.trackedChkLock.Lock()
	c.trackedChecks[check.ID] = check
	c.trackedChkLock.Unlock()
	return c.client.Agent().CheckRegister(check)
}

func (c *ConsulService) deregisterCheck(checkID string) error {
	c.logger.Printf("[DEBUG] Removing check with ID: %v", checkID)
	c.trackedChkLock.Lock()
	delete(c.trackedChecks, checkID)
	c.trackedChkLock.Unlock()
	return c.client.Agent().CheckDeregister(checkID)
}

func (c *ConsulService) deregisterService(serviceId string) error {
	c.trackedSrvLock.Lock()
	delete(c.trackedServices, serviceId)
	c.trackedSrvLock.Unlock()

	if err := c.client.Agent().ServiceDeregister(serviceId); err != nil {
		return err
	}
	return nil
}

func (c *ConsulService) makeChecks(service *structs.Service, ip string, port int) []*consul.AgentCheckRegistration {
	var checks []*consul.AgentCheckRegistration
	for _, check := range service.Checks {
		if check.Name == "" {
			check.Name = fmt.Sprintf("service: '%s' check", service.Name)
		}
		cr := &consul.AgentCheckRegistration{
			ID:        check.Hash(service.Id),
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

		checks = append(checks, cr)
	}
	return checks
}
