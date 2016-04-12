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

const (
	allocID = "12"
)

var (
	logger = log.New(os.Stdout, "", log.LstdFlags)
	check1 = structs.ServiceCheck{
		Name:     "check-foo-1",
		Type:     structs.ServiceCheckTCP,
		Interval: 30 * time.Second,
		Timeout:  5 * time.Second,
	}
	service1 = structs.Service{
		Name:      "foo-1",
		Tags:      []string{"tag1", "tag2"},
		PortLabel: "port1",
		Checks: []*structs.ServiceCheck{
			&check1,
		},
	}

	service2 = structs.Service{
		Name:      "foo-2",
		Tags:      []string{"tag1", "tag2"},
		PortLabel: "port2",
	}
)

func TestConsulServiceRegisterServices(t *testing.T) {
	cs, err := NewConsulService(&ConsulConfig{}, logger)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	cs.SetAllocID(allocID)
	// Skipping the test if consul isn't present
	if !cs.consulPresent() {
		return
	}
	task := mockTask()
	if err := cs.SyncTask(task); err != nil {
		t.Fatalf("err: %v", err)
	}
	defer cs.Shutdown()

	service1ID := service1.ID(allocID, task.Name)
	service2ID := service2.ID(allocID, task.Name)
	if err := servicesPresent(t, []string{service1ID, service2ID}, cs); err != nil {
		t.Fatalf("err : %v", err)
	}
	if err := checksPresent(t, []string{check1.Hash(service1ID)}, cs); err != nil {
		t.Fatalf("err : %v", err)
	}
}

func TestConsulServiceUpdateService(t *testing.T) {
	cs, err := NewConsulService(&ConsulConfig{}, logger)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	cs.SetAllocID(allocID)
	// Skipping the test if consul isn't present
	if !cs.consulPresent() {
		return
	}

	task := mockTask()
	if err := cs.SyncTask(task); err != nil {
		t.Fatalf("err: %v", err)
	}
	defer cs.Shutdown()

	//Update Service defn 1
	newTags := []string{"tag3"}
	task.Services[0].Tags = newTags
	if err := cs.SyncTask(task); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Make sure all the services and checks are still present
	service1ID := service1.ID(allocID, task.Name)
	service2ID := service2.ID(allocID, task.Name)
	if err := servicesPresent(t, []string{service1ID, service2ID}, cs); err != nil {
		t.Fatalf("err : %v", err)
	}
	if err := checksPresent(t, []string{check1.Hash(service1ID)}, cs); err != nil {
		t.Fatalf("err : %v", err)
	}

	// check if service defn 1 has been updated
	services, err := cs.client.Agent().Services()
	if err != nil {
		t.Fatalf("errL: %v", err)
	}
	srv, _ := services[service1ID]
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

func mockTask() *structs.Task {
	task := structs.Task{
		Name:     "foo",
		Services: []*structs.Service{&service1, &service2},
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
	return &task
}
