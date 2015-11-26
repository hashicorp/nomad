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

func NewConsulService(logger *log.Logger, consulAddr string) (*ConsulService, error) {
	var err error
	var c *consul.Client
	cfg := consul.DefaultConfig()
	cfg.Address = consulAddr
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
	// Get the list of the services and that Consul knows about
	consulServices, _ := agent.Services()
	consulChecks, _ := agent.Checks()
	delete(consulServices, "consul")

	knownChecks := make(map[string]struct{})
	for _, trackedTask := range c.trackedTasks {
		for _, service := range trackedTask.task.Services {
			if _, ok := consulServices[service.Id]; !ok {
				c.registerService(service, trackedTask.task, trackedTask.allocID)
				continue
			}

			if service.Hash() != c.serviceStates[service.Id] {
				c.registerService(service, trackedTask.task, trackedTask.allocID)
				continue
			}
			for _, check := range service.Checks {
				hash := check.Hash(service.Id)
				knownChecks[hash] = struct{}{}
				if _, ok := consulChecks[hash]; !ok {
					host, port := trackedTask.task.FindHostAndPortFor(service.PortLabel)
					if host == "" || port == 0 {
						continue
					}
					cr := c.makeCheck(service, &check, host, port)
					c.registerCheck(cr)
				}
			}
		}
	}

	for _, consulService := range consulServices {
		if _, ok := c.serviceStates[consulService.ID]; !ok {
			c.deregisterService(consulService.ID)
		}
	}

	for _, consulCheck := range consulChecks {
		if _, ok := knownChecks[consulCheck.CheckID]; !ok {
			c.deregisterCheck(consulCheck.CheckID)
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
		cr := c.makeCheck(service, &check, host, port)
		if err := c.registerCheck(cr); err != nil {
			c.logger.Printf("[ERROR] consul: Error while registerting check %v with Consul: %v", check.Name, err)
			mErr.Errors = append(mErr.Errors, err)
		}

	}
	return mErr.ErrorOrNil()
}

func (c *ConsulService) registerCheck(check *consul.AgentCheckRegistration) error {
	c.logger.Printf("[DEBUG] Registering Check with ID: %v for Service: %v", check.ID, check.ServiceID)
	return c.client.Agent().CheckRegister(check)
}

func (c *ConsulService) deregisterCheck(checkID string) error {
	c.logger.Printf("[DEBUG] Removing check with ID: %v", checkID)
	return c.client.Agent().CheckDeregister(checkID)
}

func (c *ConsulService) deregisterService(serviceId string) error {
	delete(c.serviceStates, serviceId)
	if err := c.client.Agent().ServiceDeregister(serviceId); err != nil {
		return err
	}
	return nil
}

func (c *ConsulService) makeCheck(service *structs.Service, check *structs.ServiceCheck, ip string, port int) *consul.AgentCheckRegistration {
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
	return cr
}
