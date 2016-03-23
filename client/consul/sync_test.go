package consul

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/go-multierror"
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
				Checks: []*structs.ServiceCheck{
					&structs.ServiceCheck{
						ID:       "100",
						Name:     "check-foo-1",
						Type:     structs.ServiceCheckTCP,
						Interval: 30 * time.Second,
						Timeout:  5 * time.Second,
					},
				},
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
	defer cs.Shutdown()
	if err := servicesPresent(t, []string{"1", "2"}, cs); err != nil {
		t.Fatalf("err : %v", err)
	}
	if err := checksPresent(t, []string{"100"}, cs); err != nil {
		t.Fatalf("err : %v", err)
	}
}

func TestConsulServiceUpdateService(t *testing.T) {
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
				Checks: []*structs.ServiceCheck{
					&structs.ServiceCheck{
						ID:       "100",
						Name:     "check-foo-1",
						Type:     structs.ServiceCheckTCP,
						Interval: 30 * time.Second,
						Timeout:  5 * time.Second,
					},
				},
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
	defer cs.Shutdown()

	//Update Service defn 1
	newTags := []string{"tag3"}
	task.Services[0].Tags = newTags
	if err := cs.SyncTask(&task); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Make sure all the services and checks are still present
	if err := servicesPresent(t, []string{"1", "2"}, cs); err != nil {
		t.Fatalf("err : %v", err)
	}
	if err := checksPresent(t, []string{"100"}, cs); err != nil {
		t.Fatalf("err : %v", err)
	}

	// check if service defn 1 has been updated
	services, err := cs.client.Agent().Services()
	if err != nil {
		t.Fatalf("errL: %v", err)
	}
	srv, _ := services["1"]
	if !reflect.DeepEqual(srv.Tags, newTags) {
		t.Fatalf("expected tags: %v, actual: %v", newTags, srv.Tags)
	}
}

func servicesPresent(t *testing.T, serviceIDs []string, consulService *ConsulService) error {
	var mErr multierror.Error
	services, err := consulService.client.Agent().Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	for _, serviceID := range serviceIDs {
		if _, ok := services[serviceID]; !ok {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("service ID %q not synced", serviceID))
		}
	}
	return mErr.ErrorOrNil()
}

func checksPresent(t *testing.T, checkIDs []string, consulService *ConsulService) error {
	var mErr multierror.Error
	checks, err := consulService.client.Agent().Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	for _, checkID := range checkIDs {
		if _, ok := checks[checkID]; !ok {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("check ID %q not synced", checkID))
		}
	}
	return mErr.ErrorOrNil()
}
