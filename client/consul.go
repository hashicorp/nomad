package client

import (
	"fmt"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	consulPort = 8080
)

type ConsulClient struct {
	client *api.Client
}

func NewConsulClient() (*ConsulClient, error) {
	var err error
	var c *api.Client
	if c, err = api.NewClient(api.DefaultConfig()); err != nil {
		return nil, err
	}

	consulClient := ConsulClient{
		client: c,
	}

	return &consulClient, nil
}

func (c *ConsulClient) Register(task *structs.Task, allocID string, port int, host string) error {
	var mErr multierror.Error
	serviceDefns := make([]*api.AgentServiceRegistration, len(task.Services))
	for idx, service := range task.Services {
		service.Id = fmt.Sprintf("%s-%s", allocID, task.Name)
		asr := &api.AgentServiceRegistration{
			ID:      service.Id,
			Name:    service.Name,
			Tags:    service.Tags,
			Port:    port,
			Address: host,
		}
		serviceDefns[idx] = asr
	}

	for _, serviceDefn := range serviceDefns {
		if err := c.client.Agent().ServiceRegister(serviceDefn); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	return mErr.ErrorOrNil()
}

func (c *ConsulClient) DeRegister(task *structs.Task) error {
	var mErr multierror.Error
	for _, service := range task.Services {
		if err := c.client.Agent().ServiceDeregister(service.Id); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}
	return mErr.ErrorOrNil()
}
