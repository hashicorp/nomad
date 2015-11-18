package client

import (
	"fmt"
	consul "github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
	"log"
	"time"
)

const (
	syncInterval = 5 * time.Second
)

type trackedService struct {
	allocId string
	task    *structs.Task
	service *structs.Service
}

type ConsulClient struct {
	client     *consul.Client
	logger     *log.Logger
	shutdownCh chan struct{}

	trackedServices map[string]*trackedService
}

func NewConsulClient(logger *log.Logger) (*ConsulClient, error) {
	var err error
	var c *consul.Client
	ts := make(map[string]*trackedService)
	if c, err = consul.NewClient(consul.DefaultConfig()); err != nil {
		return nil, err
	}

	consulClient := ConsulClient{
		client:          c,
		logger:          logger,
		trackedServices: ts,
	}

	return &consulClient, nil
}

func (c *ConsulClient) Register(task *structs.Task, allocID string) error {
	var mErr multierror.Error
	for _, service := range task.Services {
		if err := c.registerService(service, task, allocID); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
		ts := &trackedService{
			allocId: allocID,
			task:    task,
		}
		c.trackedServices[service.Id] = ts

	}

	return mErr.ErrorOrNil()
}

func (c *ConsulClient) Deregister(task *structs.Task) error {
	var mErr multierror.Error
	for _, service := range task.Services {
		c.logger.Printf("[INFO] De-Registering service %v with Consul", service.Name)
		if err := c.deregisterService(service.Id); err != nil {
			c.logger.Printf("[ERROR] Error in de-registering service %v from Consul", service.Name)
			mErr.Errors = append(mErr.Errors, err)
		}
		delete(c.trackedServices, service.Id)
	}
	return mErr.ErrorOrNil()
}

func (c *ConsulClient) findPortAndHostForLabel(portLabel string, task *structs.Task) (string, int) {
	for _, network := range task.Resources.Networks {
		if p, ok := network.MapLabelToValues()[portLabel]; ok {
			return network.IP, p
		}
	}
	return "", 0
}

func (c *ConsulClient) SyncWithConsul() {
	sync := time.After(syncInterval)
	agent := c.client.Agent()

	for {
		select {
		case <-sync:
			var consulServices map[string]*consul.AgentService
			var err error
			if consulServices, err = agent.Services(); err != nil {
				c.logger.Printf("[DEBUG] Error while syncing services with Consul: %v", err)
				continue
			}
			for serviceId := range c.trackedServices {
				if _, ok := consulServices[serviceId]; !ok {
					ts := c.trackedServices[serviceId]
					c.registerService(ts.service, ts.task, ts.allocId)
				}
			}

			for serviceId := range consulServices {
				if _, ok := c.trackedServices[serviceId]; !ok {
					if err := c.deregisterService(serviceId); err != nil {
						c.logger.Printf("[DEBUG] Error while de-registering service with ID: %s", serviceId)
					}
				}
			}
		case <-c.shutdownCh:
			return
		}
	}
}

func (c *ConsulClient) registerService(service *structs.Service, task *structs.Task, allocID string) error {
	var mErr multierror.Error
	service.Id = fmt.Sprintf("%s-%s", allocID, task.Name)
	host, port := c.findPortAndHostForLabel(service.PortLabel, task)
	if host == "" || port == 0 {
		return fmt.Errorf("The port:%s marked for registration of service: %s couldn't be found", service.PortLabel, service.Name)
	}
	checks := c.makeChecks(service, host, port)
	asr := &consul.AgentServiceRegistration{
		ID:      service.Id,
		Name:    service.Name,
		Tags:    service.Tags,
		Port:    port,
		Address: host,
		Checks:  checks,
	}
	if err := c.client.Agent().ServiceRegister(asr); err != nil {
		c.logger.Printf("[ERROR] Error while registering service %v with Consul: %v", service.Name, err)
		mErr.Errors = append(mErr.Errors, err)
	}
	return mErr.ErrorOrNil()
}

func (c *ConsulClient) deregisterService(serviceId string) error {
	if err := c.client.Agent().ServiceDeregister(serviceId); err != nil {
		return err
	}
	return nil
}

func (c *ConsulClient) makeChecks(service *structs.Service, ip string, port int) []*consul.AgentServiceCheck {
	var checks []*consul.AgentServiceCheck
	for _, check := range service.Checks {
		c := &consul.AgentServiceCheck{
			Interval: check.Interval.String(),
			Timeout:  check.Timeout.String(),
		}
		switch check.Type {
		case structs.ServiceCheckHTTP:
			c.HTTP = fmt.Sprintf("%s://%s:%d/%s", check.Protocol, ip, port, check.Http)
		case structs.ServiceCheckTCP:
			c.TCP = fmt.Sprintf("%s:%d", ip, port)
		case structs.ServiceCheckScript:
			c.Script = check.Script // TODO This needs to include the path of the alloc dir and based on driver types
		}
		checks = append(checks, c)
	}
	return checks
}
