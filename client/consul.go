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

func (c *ConsulClient) Register(task *structs.Task, allocID string) error {
	var mErr multierror.Error
	var serviceDefns []*api.AgentServiceRegistration
	for idx, service := range task.Services {
		service.Id = fmt.Sprintf("%s-%s", allocID, task.Name)
		host, port := c.findPortAndHostForLabel(service.PortLabel, task)
		if host == "" || port == 0 {
			continue
		}
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

func (c *ConsulClient) findPortAndHostForLabel(portLabel string, task *structs.Task) (string, int) {
	for _, network := range task.Resources.Networks {
		if p, ok := network.MapLabelToValues()[portLabel]; ok {
			return network.IP, p
		}
	}
	return "", 0
}
