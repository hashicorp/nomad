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

func TestConsul_MakeChecks(t *testing.T) {
	service := &structs.Service{
		Id:   "Foo",
		Name: "Bar",
		Checks: []structs.ServiceCheck{
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

	checks := c.makeChecks(service, "10.10.0.1", 8090)

	if checks[0].HTTP != "http://10.10.0.1:8090/foo/bar" {
		t.Fatalf("Invalid http url for check: %v", checks[0].HTTP)
	}

	if checks[1].HTTP != "https://10.10.0.1:8090/foo/bar" {
		t.Fatalf("Invalid http url for check: %v", checks[0].HTTP)
	}

	if checks[2].TCP != "10.10.0.1:8090" {
		t.Fatalf("Invalid tcp check: %v", checks[0].TCP)
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
		Checks:    make([]structs.ServiceCheck, 0),
	}

	c := newConsulService()
	if err := c.registerService(service, task, "allocid"); err == nil {
		t.Fatalf("Service should be invalid")
	}
}

func TestSyncWithConsul_Services_Deleted_From_Task(t *testing.T) {

}
