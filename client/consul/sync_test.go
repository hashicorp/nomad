package consul

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

var logger = log.New(os.Stdout, "", log.LstdFlags)

func TestConsulServiceRegisterServices(t *testing.T) {
	cs, err := NewConsulService(&ConsulConfig{}, logger)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	// Skipping the test if consul isn't present
	if !cs.consulPresent() {
		return
	}
	task := structs.Task{
		Name: "foo",
		Services: []*structs.Service{
			&structs.Service{
				ID:        "1",
				Name:      "foo-1",
				Tags:      []string{"tag1", "tag2"},
				PortLabel: "port1",
			},
			&structs.Service{
				ID:        "2",
				Name:      "foo-2",
				Tags:      []string{"tag1", "tag2"},
				PortLabel: "port2",
			},
		},
		Resources: &structs.Resources{
			Networks: []*structs.NetworkResource{
				&structs.NetworkResource{
					IP: "10.10.11.5",
					DynamicPorts: []structs.Port{
						structs.Port{
							Label: "port1",
							Value: 20002,
						},
						structs.Port{
							Label: "port2",
							Value: 20003,
						},
					},
				},
			},
		},
	}

	if err := cs.SyncTask(&task); err != nil {
		t.Fatalf("err: %v", err)
	}
	go cs.SyncWithConsul()
	time.Sleep(1 * time.Second)
	services, _ := cs.client.Agent().Services()
	if _, ok := services[task.Services[0].ID]; !ok {
		t.Fatalf("Service with ID 1 not registered")
	}

	task.Services = []*structs.Service{
		&structs.Service{
			ID:        "1",
			Name:      "foo-1",
			Tags:      []string{"tag1"},
			PortLabel: "port1",
		},
	}
	if err := cs.SyncTask(&task); err != nil {
		t.Fatalf("err: %v", err)
	}
	services, _ = cs.client.Agent().Services()
	if _, ok := services["2"]; ok {
		t.Fatalf("Service with ID 2 should not be registered")
	}
	if err := cs.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}
	time.Sleep(1 * time.Second)

	services, _ = cs.client.Agent().Services()
	if _, ok := services["2"]; ok {
		t.Fatalf("Service with ID 2 should not be registered")
	}
	if _, ok := services["1"]; ok {
		t.Fatalf("Service with ID 1 should not be registered")
	}

}
