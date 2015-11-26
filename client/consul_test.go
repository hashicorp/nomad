package client

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"log"
	"os"
	"testing"
	"time"
)

func newConsulService() *ConsulService {
	logger := log.New(os.Stdout, "logger: ", log.Lshortfile)
	c, _ := NewConsulService(logger, "")
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
		Id:   "Foo",
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

	check1 := c.makeCheck(service, service.Checks[0], "10.10.0.1", 8090)
	check2 := c.makeCheck(service, service.Checks[1], "10.10.0.1", 8090)
	check3 := c.makeCheck(service, service.Checks[2], "10.10.0.1", 8090)

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
		Id:        "service-id",
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

	c.performSync(c.client.Agent())
	if len(c.serviceStates) != 0 {
		t.Fatalf("Expected tracked services: %v, Actual: %v", 0, len(c.serviceStates))
	}
}

func TestConsul_Service_Should_Be_Re_Reregistered_On_Change(t *testing.T) {
	c := newConsulService()
	task := newTask()
	s1 := structs.Service{
		Id:        "1-example-cache-redis",
		Name:      "example-cache-redis",
		Tags:      []string{"global"},
		PortLabel: "db",
	}
	task.Services = append(task.Services, &s1)
	c.Register(task, "1")

	s1.Tags = []string{"frontcache"}

	c.performSync(c.client.Agent())

	if len(c.serviceStates) != 1 {
		t.Fatal("We should be tracking one service")
	}

	if c.serviceStates[s1.Id] != s1.Hash() {
		t.Fatalf("Hash is %v, expected %v", c.serviceStates[s1.Id], s1.Hash())
	}
}

func TestConsul_AddCheck_To_Service(t *testing.T) {
	c := newConsulService()
	task := newTask()
	var checks []*structs.ServiceCheck
	s1 := structs.Service{
		Id:        "1-example-cache-redis",
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

	_, totalChecks := c.performSync(c.client.Agent())
	if totalChecks != 1 {
		t.Fatalf("Expected tracked checks: %v, actual: %v", 1, totalChecks)
	}
}
