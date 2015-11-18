package client

import (
	"fmt"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
	"log"
)

const (
	consulPort = 8080
)

type ConsulClient struct {
	client *api.Client

	logger *log.Logger
}

func NewConsulClient(logger *log.Logger) (*ConsulClient, error) {
	var err error
	var c *api.Client
	if c, err = api.NewClient(api.DefaultConfig()); err != nil {
		return nil, err
	}

	consulClient := ConsulClient{
		client: c,
		logger: logger,
	}

	return &consulClient, nil
}

func (c *ConsulClient) Register(task *structs.Task, allocID string) error {
	var mErr multierror.Error
	var serviceDefns []*api.AgentServiceRegistration
	for _, service := range task.Services {
		service.Id = fmt.Sprintf("%s-%s", allocID, task.Name)
		host, port := c.findPortAndHostForLabel(service.PortLabel, task)
		if host == "" || port == 0 {
			continue
		}
		checks := c.makeChecks(service, host, port)
		asr := &api.AgentServiceRegistration{
			ID:      service.Id,
			Name:    service.Name,
			Tags:    service.Tags,
			Port:    port,
			Address: host,
			Checks:  checks,
		}
		serviceDefns = append(serviceDefns, asr)
	}

	for _, serviceDefn := range serviceDefns {
		c.logger.Printf("[INFO] Registering service %v with Consul", serviceDefn.Name)
		if err := c.client.Agent().ServiceRegister(serviceDefn); err != nil {
			c.logger.Printf("[ERROR] Error while registering service %v with Consul: %v", serviceDefn.Name, err)
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	return mErr.ErrorOrNil()
}

func (c *ConsulClient) Deregister(task *structs.Task) error {
	var mErr multierror.Error
	for _, service := range task.Services {
		c.logger.Printf("[INFO] De-Registering service %v with Consul", service.Name)
		if err := c.client.Agent().ServiceDeregister(service.Id); err != nil {
			c.logger.Printf("[ERROR] Error in de-registering service %v from Consul", service.Name)
			mErr.Errors = append(mErr.Errors, err)
		}
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

func (c *ConsulClient) makeChecks(service *structs.Service, ip string, port int) []*api.AgentServiceCheck {
	var checks []*api.AgentServiceCheck
	for _, check := range service.Checks {
		c := &api.AgentServiceCheck{
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
