package client

import (
	consul "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/nomad/structs"
	"log"
	"os"
	"testing"
	"time"
)

type mockConsulApiClient struct {
	serviceRegisterCallCount   int
	checkRegisterCallCount     int
	checkDeregisterCallCount   int
	serviceDeregisterCallCount int
}

func (a *mockConsulApiClient) CheckRegister(check *consul.AgentCheckRegistration) error {
	a.checkRegisterCallCount += 1
	return nil
}

func (a *mockConsulApiClient) CheckDeregister(checkID string) error {
	a.checkDeregisterCallCount += 1
	return nil
}

func (a *mockConsulApiClient) ServiceRegister(service *consul.AgentServiceRegistration) error {
	a.serviceRegisterCallCount += 1
	return nil
}

func (a *mockConsulApiClient) ServiceDeregister(serviceId string) error {
	a.serviceDeregisterCallCount += 1
	return nil
}

func (a *mockConsulApiClient) Services() (map[string]*consul.AgentService, error) {
	return make(map[string]*consul.AgentService), nil
}

func (a *mockConsulApiClient) Checks() (map[string]*consul.AgentCheck, error) {
	return make(map[string]*consul.AgentCheck), nil
}

func newConsulService() *ConsulService {
	logger := log.New(os.Stdout, "logger: ", log.Lshortfile)
	c, _ := NewConsulService(logger, "", "", "", false, false)
	c.client = &mockConsulApiClient{}
	return c
}

func newTask() *structs.Task {
	var services []*structs.Service
	return &structs.Task{
		Name:     "redis",
		Services: services,
		Resources: &structs.Resources{
			Networks: []*structs.NetworkResource{
				{
					IP:           "10.10.0.1",
					DynamicPorts: []structs.Port{{"db", 20413}},
				},
			},
		},
	}
}

func TestConsul_MakeChecks(t *testing.T) {
	service := &structs.Service{
		Name: "Bar",
		Checks: []*structs.ServiceCheck{
			{
				Type:     "http",
				Path:     "/foo/bar",
				Interval: 10 * time.Second,
				Timeout:  2 * time.Second,
			},
			{
				Type:     "http",
				Protocol: "https",
				Path:     "/foo/bar",
				Interval: 10 * time.Second,
				Timeout:  2 * time.Second,
			},
			{
				Type:     "tcp",
				Interval: 10 * time.Second,
				Timeout:  2 * time.Second,
			},
		},
	}

	c := newConsulService()

	check1 := c.makeCheck(service, service.Checks[0], "10.10.0.1", 8090, "1234")
	check2 := c.makeCheck(service, service.Checks[1], "10.10.0.1", 8090, "1234")
	check3 := c.makeCheck(service, service.Checks[2], "10.10.0.1", 8090, "1234")

	if check1.HTTP != "http://10.10.0.1:8090/foo/bar" {
		t.Fatalf("Invalid http url for check: %v", check1.HTTP)
	}

	if check2.HTTP != "https://10.10.0.1:8090/foo/bar" {
		t.Fatalf("Invalid http url for check: %v", check2.HTTP)
	}

	if check3.TCP != "10.10.0.1:8090" {
		t.Fatalf("Invalid tcp check: %v", check3.TCP)
	}
}

func TestConsul_InvalidPortLabelForService(t *testing.T) {
	task := &structs.Task{
		Name:   "foo",
		Driver: "docker",
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 1024,
			DiskMB:   1024,
			IOPS:     10,
			Networks: []*structs.NetworkResource{
				{
					Device: "eth0",
					CIDR:   "255.255.0.0/16",
					MBits:  10,
					ReservedPorts: []structs.Port{
						{
							Label: "http",
							Value: 8080,
						},
						{
							Label: "ssh",
							Value: 2026,
						},
					},
				},
			},
		},
	}
	service := &structs.Service{
		Name:      "foo",
		Tags:      []string{"a", "b"},
		PortLabel: "https",
		Checks:    make([]*structs.ServiceCheck, 0),
	}

	c := newConsulService()
	if err := c.registerService(service, task, "allocid"); err == nil {
		t.Fatalf("Service should be invalid")
	}
}

func TestConsul_Services_Deleted_From_Task(t *testing.T) {
	c := newConsulService()
	task := structs.Task{
		Name: "redis",
		Services: []*structs.Service{
			&structs.Service{
				Name:      "example-cache-redis",
				Tags:      []string{"global"},
				PortLabel: "db",
			},
		},
		Resources: &structs.Resources{
			Networks: []*structs.NetworkResource{
				{
					IP:           "10.10.0.1",
					DynamicPorts: []structs.Port{{"db", 20413}},
				},
			},
		},
	}
	c.Register(&task, "1")
	if len(c.serviceStates) != 1 {
		t.Fatalf("Expected tracked services: %v, Actual: %v", 1, len(c.serviceStates))
	}
	task.Services = []*structs.Service{}

	c.performSync()
	if len(c.serviceStates) != 0 {
		t.Fatalf("Expected tracked services: %v, Actual: %v", 0, len(c.serviceStates))
	}
}

func TestConsul_Service_Should_Be_Re_Reregistered_On_Change(t *testing.T) {
	c := newConsulService()
	task := newTask()
	allocId := "1"
	s1 := structs.Service{
		Name:      "example-cache-redis",
		Tags:      []string{"global"},
		PortLabel: "db",
	}
	serviceId := c.createServiceId(allocId, &s1)
	task.Services = append(task.Services, &s1)
	c.Register(task, allocId)

	s1.Tags = []string{"frontcache"}

	c.performSync()

	if len(c.serviceStates) != 1 {
		t.Fatal("We should be tracking one service")
	}

	if c.serviceStates[serviceId] != s1.Hash() {
		t.Fatalf("Hash is %v, expected %v", c.serviceStates[serviceId], s1.Hash())
	}
}

func TestConsul_AddCheck_To_Service(t *testing.T) {
	apiClient := &mockConsulApiClient{}
	c := newConsulService()
	c.client = apiClient
	task := newTask()
	var checks []*structs.ServiceCheck
	s1 := structs.Service{
		Name:      "example-cache-redis",
		Tags:      []string{"global"},
		PortLabel: "db",
		Checks:    checks,
	}
	task.Services = append(task.Services, &s1)
	c.Register(task, "1")

	check1 := structs.ServiceCheck{
		Name:     "alive",
		Type:     "tcp",
		Interval: 10 * time.Second,
		Timeout:  5 * time.Second,
	}

	s1.Checks = append(s1.Checks, &check1)

	c.performSync()
	if apiClient.checkRegisterCallCount != 1 {
		t.Fatalf("Expected number of check registrations: %v, Actual: %v", 1, apiClient.checkRegisterCallCount)
	}
}

func TestConsul_ModifyCheck(t *testing.T) {
	apiClient := &mockConsulApiClient{}
	c := newConsulService()
	c.client = apiClient
	task := newTask()
	var checks []*structs.ServiceCheck
	s1 := structs.Service{
		Name:      "example-cache-redis",
		Tags:      []string{"global"},
		PortLabel: "db",
		Checks:    checks,
	}
	task.Services = append(task.Services, &s1)
	c.Register(task, "1")

	check1 := structs.ServiceCheck{
		Name:     "alive",
		Type:     "tcp",
		Interval: 10 * time.Second,
		Timeout:  5 * time.Second,
	}

	s1.Checks = append(s1.Checks, &check1)

	c.performSync()
	if apiClient.checkRegisterCallCount != 1 {
		t.Fatalf("Expected number of check registrations: %v, Actual: %v", 1, apiClient.checkRegisterCallCount)
	}

	check1.Timeout = 2 * time.Second
	c.performSync()
	if apiClient.checkRegisterCallCount != 2 {
		t.Fatalf("Expected number of check registrations: %v, Actual: %v", 2, apiClient.checkRegisterCallCount)
	}
}
