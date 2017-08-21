package consul

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/kr/pretty"
)

const (
	// Ports used in testTask
	xPort = 1234
	yPort = 1235
)

func testLogger() *log.Logger {
	if testing.Verbose() {
		return log.New(os.Stderr, "", log.LstdFlags)
	}
	return log.New(ioutil.Discard, "", 0)
}

func testTask() *structs.Task {
	return &structs.Task{
		Name: "taskname",
		Resources: &structs.Resources{
			Networks: []*structs.NetworkResource{
				{
					DynamicPorts: []structs.Port{
						{Label: "x", Value: xPort},
						{Label: "y", Value: yPort},
					},
				},
			},
		},
		Services: []*structs.Service{
			{
				Name:      "taskname-service",
				PortLabel: "x",
				Tags:      []string{"tag1", "tag2"},
			},
		},
	}
}

// testFakeCtx contains a fake Consul AgentAPI and implements the Exec
// interface to allow testing without running Consul.
type testFakeCtx struct {
	ServiceClient *ServiceClient
	FakeConsul    *fakeConsul
	Task          *structs.Task

	// Ticked whenever a script is called
	execs chan int

	// If non-nil will be called by script checks
	ExecFunc func(ctx context.Context, cmd string, args []string) ([]byte, int, error)
}

// Exec implements the ScriptExecutor interface and will use an alternate
// implementation t.ExecFunc if non-nil.
func (t *testFakeCtx) Exec(ctx context.Context, cmd string, args []string) ([]byte, int, error) {
	select {
	case t.execs <- 1:
	default:
	}
	if t.ExecFunc == nil {
		// Default impl is just "ok"
		return []byte("ok"), 0, nil
	}
	return t.ExecFunc(ctx, cmd, args)
}

var errNoOps = fmt.Errorf("testing error: no pending operations")

// syncOps simulates one iteration of the ServiceClient.Run loop and returns
// any errors returned by sync() or errNoOps if no pending operations.
func (t *testFakeCtx) syncOnce() error {
	select {
	case ops := <-t.ServiceClient.opCh:
		t.ServiceClient.merge(ops)
		return t.ServiceClient.sync()
	default:
		return errNoOps
	}
}

// setupFake creates a testFakeCtx with a ServiceClient backed by a fakeConsul.
// A test Task is also provided.
func setupFake() *testFakeCtx {
	fc := newFakeConsul()
	return &testFakeCtx{
		ServiceClient: NewServiceClient(fc, true, testLogger()),
		FakeConsul:    fc,
		Task:          testTask(),
		execs:         make(chan int, 100),
	}
}

// fakeConsul is a fake in-memory Consul backend for ServiceClient.
type fakeConsul struct {
	// maps of what services and checks have been registered
	services map[string]*api.AgentServiceRegistration
	checks   map[string]*api.AgentCheckRegistration
	mu       sync.Mutex

	// when UpdateTTL is called the check ID will have its counter inc'd
	checkTTLs map[string]int

	// What check status to return from Checks()
	checkStatus string
}

func newFakeConsul() *fakeConsul {
	return &fakeConsul{
		services:    make(map[string]*api.AgentServiceRegistration),
		checks:      make(map[string]*api.AgentCheckRegistration),
		checkTTLs:   make(map[string]int),
		checkStatus: api.HealthPassing,
	}
}

func (c *fakeConsul) Services() (map[string]*api.AgentService, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	r := make(map[string]*api.AgentService, len(c.services))
	for k, v := range c.services {
		r[k] = &api.AgentService{
			ID:                v.ID,
			Service:           v.Name,
			Tags:              make([]string, len(v.Tags)),
			Port:              v.Port,
			Address:           v.Address,
			EnableTagOverride: v.EnableTagOverride,
		}
		copy(r[k].Tags, v.Tags)
	}
	return r, nil
}

func (c *fakeConsul) Checks() (map[string]*api.AgentCheck, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	r := make(map[string]*api.AgentCheck, len(c.checks))
	for k, v := range c.checks {
		r[k] = &api.AgentCheck{
			CheckID:     v.ID,
			Name:        v.Name,
			Status:      c.checkStatus,
			Notes:       v.Notes,
			ServiceID:   v.ServiceID,
			ServiceName: c.services[v.ServiceID].Name,
		}
	}
	return r, nil
}

func (c *fakeConsul) CheckRegister(check *api.AgentCheckRegistration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[check.ID] = check

	// Be nice and make checks reachable-by-service
	scheck := check.AgentServiceCheck
	c.services[check.ServiceID].Checks = append(c.services[check.ServiceID].Checks, &scheck)
	return nil
}

func (c *fakeConsul) CheckDeregister(checkID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.checks, checkID)
	delete(c.checkTTLs, checkID)
	return nil
}

func (c *fakeConsul) ServiceRegister(service *api.AgentServiceRegistration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services[service.ID] = service
	return nil
}

func (c *fakeConsul) ServiceDeregister(serviceID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.services, serviceID)
	return nil
}

func (c *fakeConsul) UpdateTTL(id string, output string, status string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	check, ok := c.checks[id]
	if !ok {
		return fmt.Errorf("unknown check id: %q", id)
	}
	// Flip initial status to passing
	check.Status = "passing"
	c.checkTTLs[id]++
	return nil
}

func TestConsul_ChangeTags(t *testing.T) {
	ctx := setupFake()

	allocID := "allocid"
	if err := ctx.ServiceClient.RegisterTask(allocID, ctx.Task, nil, nil); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if n := len(ctx.FakeConsul.services); n != 1 {
		t.Fatalf("expected 1 service but found %d:\n%#v", n, ctx.FakeConsul.services)
	}

	// Query the allocs registrations and then again when we update. The IDs
	// should change
	reg1, err := ctx.ServiceClient.AllocRegistrations(allocID)
	if err != nil {
		t.Fatalf("Looking up alloc registration failed: %v", err)
	}
	if reg1 == nil {
		t.Fatalf("Nil alloc registrations: %v", err)
	}
	if num := reg1.NumServices(); num != 1 {
		t.Fatalf("Wrong number of servies: got %d; want 1", num)
	}
	if num := reg1.NumChecks(); num != 0 {
		t.Fatalf("Wrong number of checks: got %d; want 0", num)
	}

	origKey := ""
	for k, v := range ctx.FakeConsul.services {
		origKey = k
		if v.Name != ctx.Task.Services[0].Name {
			t.Errorf("expected Name=%q != %q", ctx.Task.Services[0].Name, v.Name)
		}
		if !reflect.DeepEqual(v.Tags, ctx.Task.Services[0].Tags) {
			t.Errorf("expected Tags=%v != %v", ctx.Task.Services[0].Tags, v.Tags)
		}
	}

	origTask := ctx.Task
	ctx.Task = testTask()
	ctx.Task.Services[0].Tags[0] = "newtag"
	if err := ctx.ServiceClient.UpdateTask("allocid", origTask, ctx.Task, nil, nil); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}
	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if n := len(ctx.FakeConsul.services); n != 1 {
		t.Fatalf("expected 1 service but found %d:\n%#v", n, ctx.FakeConsul.services)
	}

	for k, v := range ctx.FakeConsul.services {
		if k == origKey {
			t.Errorf("expected key to change but found %q", k)
		}
		if v.Name != ctx.Task.Services[0].Name {
			t.Errorf("expected Name=%q != %q", ctx.Task.Services[0].Name, v.Name)
		}
		if !reflect.DeepEqual(v.Tags, ctx.Task.Services[0].Tags) {
			t.Errorf("expected Tags=%v != %v", ctx.Task.Services[0].Tags, v.Tags)
		}
	}

	// Check again and ensure the IDs changed
	reg2, err := ctx.ServiceClient.AllocRegistrations(allocID)
	if err != nil {
		t.Fatalf("Looking up alloc registration failed: %v", err)
	}
	if reg2 == nil {
		t.Fatalf("Nil alloc registrations: %v", err)
	}
	if num := reg2.NumServices(); num != 1 {
		t.Fatalf("Wrong number of servies: got %d; want 1", num)
	}
	if num := reg2.NumChecks(); num != 0 {
		t.Fatalf("Wrong number of checks: got %d; want 0", num)
	}

	for task, treg := range reg1.Tasks {
		otherTaskReg, ok := reg2.Tasks[task]
		if !ok {
			t.Fatalf("Task %q not in second reg", task)
		}

		for sID := range treg.Services {
			if _, ok := otherTaskReg.Services[sID]; ok {
				t.Fatalf("service ID didn't change")
			}
		}
	}
}

// TestConsul_ChangePorts asserts that changing the ports on a service updates
// it in Consul. Since ports are not part of the service ID this is a slightly
// different code path than changing tags.
func TestConsul_ChangePorts(t *testing.T) {
	ctx := setupFake()
	ctx.Task.Services[0].Checks = []*structs.ServiceCheck{
		{
			Name:      "c1",
			Type:      "tcp",
			Interval:  time.Second,
			Timeout:   time.Second,
			PortLabel: "x",
		},
		{
			Name:     "c2",
			Type:     "script",
			Interval: 9000 * time.Hour,
			Timeout:  time.Second,
		},
		{
			Name:      "c3",
			Type:      "http",
			Protocol:  "http",
			Path:      "/",
			Interval:  time.Second,
			Timeout:   time.Second,
			PortLabel: "y",
		},
	}

	if err := ctx.ServiceClient.RegisterTask("allocid", ctx.Task, ctx, nil); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if n := len(ctx.FakeConsul.services); n != 1 {
		t.Fatalf("expected 1 service but found %d:\n%#v", n, ctx.FakeConsul.services)
	}

	origServiceKey := ""
	for k, v := range ctx.FakeConsul.services {
		origServiceKey = k
		if v.Name != ctx.Task.Services[0].Name {
			t.Errorf("expected Name=%q != %q", ctx.Task.Services[0].Name, v.Name)
		}
		if !reflect.DeepEqual(v.Tags, ctx.Task.Services[0].Tags) {
			t.Errorf("expected Tags=%v != %v", ctx.Task.Services[0].Tags, v.Tags)
		}
		if v.Port != xPort {
			t.Errorf("expected Port x=%v but found: %v", xPort, v.Port)
		}
	}

	if n := len(ctx.FakeConsul.checks); n != 3 {
		t.Fatalf("expected 3 checks but found %d:\n%#v", n, ctx.FakeConsul.checks)
	}

	origTCPKey := ""
	origScriptKey := ""
	origHTTPKey := ""
	for k, v := range ctx.FakeConsul.checks {
		switch v.Name {
		case "c1":
			origTCPKey = k
			if expected := fmt.Sprintf(":%d", xPort); v.TCP != expected {
				t.Errorf("expected Port x=%v but found: %v", expected, v.TCP)
			}
		case "c2":
			origScriptKey = k
			select {
			case <-ctx.execs:
				if n := len(ctx.execs); n > 0 {
					t.Errorf("expected 1 exec but found: %d", n+1)
				}
			case <-time.After(3 * time.Second):
				t.Errorf("script not called in time")
			}
		case "c3":
			origHTTPKey = k
			if expected := fmt.Sprintf("http://:%d/", yPort); v.HTTP != expected {
				t.Errorf("expected Port y=%v but found: %v", expected, v.HTTP)
			}
		default:
			t.Fatalf("unexpected check: %q", v.Name)
		}
	}

	// Now update the PortLabel on the Service and Check c3
	origTask := ctx.Task
	ctx.Task = testTask()
	ctx.Task.Services[0].PortLabel = "y"
	ctx.Task.Services[0].Checks = []*structs.ServiceCheck{
		{
			Name:      "c1",
			Type:      "tcp",
			Interval:  time.Second,
			Timeout:   time.Second,
			PortLabel: "x",
		},
		{
			Name:     "c2",
			Type:     "script",
			Interval: 9000 * time.Hour,
			Timeout:  time.Second,
		},
		{
			Name:     "c3",
			Type:     "http",
			Protocol: "http",
			Path:     "/",
			Interval: time.Second,
			Timeout:  time.Second,
			// Removed PortLabel; should default to service's (y)
		},
	}
	if err := ctx.ServiceClient.UpdateTask("allocid", origTask, ctx.Task, ctx, nil); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}
	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if n := len(ctx.FakeConsul.services); n != 1 {
		t.Fatalf("expected 1 service but found %d:\n%#v", n, ctx.FakeConsul.services)
	}

	for k, v := range ctx.FakeConsul.services {
		if k != origServiceKey {
			t.Errorf("unexpected key change; was: %q -- but found %q", origServiceKey, k)
		}
		if v.Name != ctx.Task.Services[0].Name {
			t.Errorf("expected Name=%q != %q", ctx.Task.Services[0].Name, v.Name)
		}
		if !reflect.DeepEqual(v.Tags, ctx.Task.Services[0].Tags) {
			t.Errorf("expected Tags=%v != %v", ctx.Task.Services[0].Tags, v.Tags)
		}
		if v.Port != yPort {
			t.Errorf("expected Port y=%v but found: %v", yPort, v.Port)
		}
	}

	if n := len(ctx.FakeConsul.checks); n != 3 {
		t.Fatalf("expected 3 check but found %d:\n%#v", n, ctx.FakeConsul.checks)
	}

	for k, v := range ctx.FakeConsul.checks {
		switch v.Name {
		case "c1":
			if k != origTCPKey {
				t.Errorf("unexpected key change for %s from %q to %q", v.Name, origTCPKey, k)
			}
			if expected := fmt.Sprintf(":%d", xPort); v.TCP != expected {
				t.Errorf("expected Port x=%v but found: %v", expected, v.TCP)
			}
		case "c2":
			if k != origScriptKey {
				t.Errorf("unexpected key change for %s from %q to %q", v.Name, origScriptKey, k)
			}
			select {
			case <-ctx.execs:
				if n := len(ctx.execs); n > 0 {
					t.Errorf("expected 1 exec but found: %d", n+1)
				}
			case <-time.After(3 * time.Second):
				t.Errorf("script not called in time")
			}
		case "c3":
			if k == origHTTPKey {
				t.Errorf("expected %s key to change from %q", v.Name, k)
			}
			if expected := fmt.Sprintf("http://:%d/", yPort); v.HTTP != expected {
				t.Errorf("expected Port y=%v but found: %v", expected, v.HTTP)
			}
		default:
			t.Errorf("Unknown check: %q", k)
		}
	}
}

// TestConsul_ChangeChecks asserts that updating only the checks on a service
// properly syncs with Consul.
func TestConsul_ChangeChecks(t *testing.T) {
	ctx := setupFake()
	ctx.Task.Services[0].Checks = []*structs.ServiceCheck{
		{
			Name:      "c1",
			Type:      "tcp",
			Interval:  time.Second,
			Timeout:   time.Second,
			PortLabel: "x",
		},
	}

	allocID := "allocid"
	if err := ctx.ServiceClient.RegisterTask(allocID, ctx.Task, ctx, nil); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if n := len(ctx.FakeConsul.services); n != 1 {
		t.Fatalf("expected 1 service but found %d:\n%#v", n, ctx.FakeConsul.services)
	}

	// Query the allocs registrations and then again when we update. The IDs
	// should change
	reg1, err := ctx.ServiceClient.AllocRegistrations(allocID)
	if err != nil {
		t.Fatalf("Looking up alloc registration failed: %v", err)
	}
	if reg1 == nil {
		t.Fatalf("Nil alloc registrations: %v", err)
	}
	if num := reg1.NumServices(); num != 1 {
		t.Fatalf("Wrong number of servies: got %d; want 1", num)
	}
	if num := reg1.NumChecks(); num != 1 {
		t.Fatalf("Wrong number of checks: got %d; want 1", num)
	}

	origServiceKey := ""
	for k, v := range ctx.FakeConsul.services {
		origServiceKey = k
		if v.Name != ctx.Task.Services[0].Name {
			t.Errorf("expected Name=%q != %q", ctx.Task.Services[0].Name, v.Name)
		}
		if v.Port != xPort {
			t.Errorf("expected Port x=%v but found: %v", xPort, v.Port)
		}
	}

	if n := len(ctx.FakeConsul.checks); n != 1 {
		t.Fatalf("expected 1 check but found %d:\n%#v", n, ctx.FakeConsul.checks)
	}
	for _, v := range ctx.FakeConsul.checks {
		if v.Name != "c1" {
			t.Fatalf("expected check c1 but found %q", v.Name)
		}
	}

	// Now add a check and modify the original
	origTask := ctx.Task.Copy()
	ctx.Task.Services[0].Checks = []*structs.ServiceCheck{
		{
			Name:      "c1",
			Type:      "tcp",
			Interval:  2 * time.Second,
			Timeout:   time.Second,
			PortLabel: "x",
		},
		{
			Name:      "c2",
			Type:      "http",
			Path:      "/",
			Interval:  time.Second,
			Timeout:   time.Second,
			PortLabel: "x",
		},
	}
	if err := ctx.ServiceClient.UpdateTask("allocid", origTask, ctx.Task, ctx, nil); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}
	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if n := len(ctx.FakeConsul.services); n != 1 {
		t.Fatalf("expected 1 service but found %d:\n%#v", n, ctx.FakeConsul.services)
	}

	if _, ok := ctx.FakeConsul.services[origServiceKey]; !ok {
		t.Errorf("unexpected key change; was: %q -- but found %#v", origServiceKey, ctx.FakeConsul.services)
	}

	if n := len(ctx.FakeConsul.checks); n != 2 {
		t.Fatalf("expected 2 check but found %d:\n%#v", n, ctx.FakeConsul.checks)
	}

	for k, v := range ctx.FakeConsul.checks {
		switch v.Name {
		case "c1":
			if expected := fmt.Sprintf(":%d", xPort); v.TCP != expected {
				t.Errorf("expected Port x=%v but found: %v", expected, v.TCP)
			}
		case "c2":
			if expected := fmt.Sprintf("http://:%d/", xPort); v.HTTP != expected {
				t.Errorf("expected Port x=%v but found: %v", expected, v.HTTP)
			}
		default:
			t.Errorf("Unknown check: %q", k)
		}
	}

	// Check again and ensure the IDs changed
	reg2, err := ctx.ServiceClient.AllocRegistrations(allocID)
	if err != nil {
		t.Fatalf("Looking up alloc registration failed: %v", err)
	}
	if reg2 == nil {
		t.Fatalf("Nil alloc registrations: %v", err)
	}
	if num := reg2.NumServices(); num != 1 {
		t.Fatalf("Wrong number of servies: got %d; want 1", num)
	}
	if num := reg2.NumChecks(); num != 2 {
		t.Fatalf("Wrong number of checks: got %d; want 2", num)
	}

	for task, treg := range reg1.Tasks {
		otherTaskReg, ok := reg2.Tasks[task]
		if !ok {
			t.Fatalf("Task %q not in second reg", task)
		}

		for sID, sreg := range treg.Services {
			otherServiceReg, ok := otherTaskReg.Services[sID]
			if !ok {
				t.Fatalf("service ID changed")
			}

			for newID := range sreg.checkIDs {
				if _, ok := otherServiceReg.checkIDs[newID]; ok {
					t.Fatalf("check IDs should change")
				}
			}
		}
	}
}

// TestConsul_RegServices tests basic service registration.
func TestConsul_RegServices(t *testing.T) {
	ctx := setupFake()

	if err := ctx.ServiceClient.RegisterTask("allocid", ctx.Task, nil, nil); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if n := len(ctx.FakeConsul.services); n != 1 {
		t.Fatalf("expected 1 service but found %d:\n%#v", n, ctx.FakeConsul.services)
	}
	for _, v := range ctx.FakeConsul.services {
		if v.Name != ctx.Task.Services[0].Name {
			t.Errorf("expected Name=%q != %q", ctx.Task.Services[0].Name, v.Name)
		}
		if !reflect.DeepEqual(v.Tags, ctx.Task.Services[0].Tags) {
			t.Errorf("expected Tags=%v != %v", ctx.Task.Services[0].Tags, v.Tags)
		}
		if v.Port != xPort {
			t.Errorf("expected Port=%d != %d", xPort, v.Port)
		}
	}

	// Make a change which will register a new service
	ctx.Task.Services[0].Name = "taskname-service2"
	ctx.Task.Services[0].Tags[0] = "tag3"
	if err := ctx.ServiceClient.RegisterTask("allocid", ctx.Task, nil, nil); err != nil {
		t.Fatalf("unpexpected error registering task: %v", err)
	}

	// Make sure changes don't take affect until sync() is called (since
	// Run() isn't running)
	if n := len(ctx.FakeConsul.services); n != 1 {
		t.Fatalf("expected 1 service but found %d:\n%#v", n, ctx.FakeConsul.services)
	}
	for _, v := range ctx.FakeConsul.services {
		if reflect.DeepEqual(v.Tags, ctx.Task.Services[0].Tags) {
			t.Errorf("expected Tags to differ, changes applied before sync()")
		}
	}

	// Now sync() and re-check for the applied updates
	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}
	if n := len(ctx.FakeConsul.services); n != 2 {
		t.Fatalf("expected 2 services but found %d:\n%#v", n, ctx.FakeConsul.services)
	}
	found := false
	for _, v := range ctx.FakeConsul.services {
		if v.Name == ctx.Task.Services[0].Name {
			if found {
				t.Fatalf("found new service name %q twice", v.Name)
			}
			found = true
			if !reflect.DeepEqual(v.Tags, ctx.Task.Services[0].Tags) {
				t.Errorf("expected Tags=%v != %v", ctx.Task.Services[0].Tags, v.Tags)
			}
		}
	}
	if !found {
		t.Fatalf("did not find new service %q", ctx.Task.Services[0].Name)
	}

	// Remove the new task
	ctx.ServiceClient.RemoveTask("allocid", ctx.Task)
	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}
	if n := len(ctx.FakeConsul.services); n != 1 {
		t.Fatalf("expected 1 service but found %d:\n%#v", n, ctx.FakeConsul.services)
	}
	for _, v := range ctx.FakeConsul.services {
		if v.Name != "taskname-service" {
			t.Errorf("expected original task to survive not %q", v.Name)
		}
	}
}

// TestConsul_ShutdownOK tests the ok path for the shutdown logic in
// ServiceClient.
func TestConsul_ShutdownOK(t *testing.T) {
	ctx := setupFake()

	// Add a script check to make sure its TTL gets updated
	ctx.Task.Services[0].Checks = []*structs.ServiceCheck{
		{
			Name:    "scriptcheck",
			Type:    "script",
			Command: "true",
			// Make check block until shutdown
			Interval:      9000 * time.Hour,
			Timeout:       10 * time.Second,
			InitialStatus: "warning",
		},
	}

	go ctx.ServiceClient.Run()

	// Register a task and agent
	if err := ctx.ServiceClient.RegisterTask("allocid", ctx.Task, ctx, nil); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	agentServices := []*structs.Service{
		{
			Name:      "http",
			Tags:      []string{"nomad"},
			PortLabel: "localhost:2345",
		},
	}
	if err := ctx.ServiceClient.RegisterAgent("client", agentServices); err != nil {
		t.Fatalf("unexpected error registering agent: %v", err)
	}

	// Shutdown should block until scripts finish
	if err := ctx.ServiceClient.Shutdown(); err != nil {
		t.Errorf("unexpected error shutting down client: %v", err)
	}

	// UpdateTTL should have been called once for the script check
	if n := len(ctx.FakeConsul.checkTTLs); n != 1 {
		t.Fatalf("expected 1 checkTTL entry but found: %d", n)
	}
	for _, v := range ctx.FakeConsul.checkTTLs {
		if v != 1 {
			t.Fatalf("expected script check to be updated once but found %d", v)
		}
	}
	for _, v := range ctx.FakeConsul.checks {
		if v.Status != "passing" {
			t.Fatalf("expected check to be passing but found %q", v.Status)
		}
	}
}

// TestConsul_ShutdownSlow tests the slow but ok path for the shutdown logic in
// ServiceClient.
func TestConsul_ShutdownSlow(t *testing.T) {
	t.Parallel() // run the slow tests in parallel
	ctx := setupFake()

	// Add a script check to make sure its TTL gets updated
	ctx.Task.Services[0].Checks = []*structs.ServiceCheck{
		{
			Name:    "scriptcheck",
			Type:    "script",
			Command: "true",
			// Make check block until shutdown
			Interval:      9000 * time.Hour,
			Timeout:       5 * time.Second,
			InitialStatus: "warning",
		},
	}

	// Make Exec slow, but not too slow
	waiter := make(chan struct{})
	ctx.ExecFunc = func(ctx context.Context, cmd string, args []string) ([]byte, int, error) {
		select {
		case <-waiter:
		default:
			close(waiter)
		}
		time.Sleep(time.Second)
		return []byte{}, 0, nil
	}

	// Make shutdown wait time just a bit longer than ctx.Exec takes
	ctx.ServiceClient.shutdownWait = 3 * time.Second

	go ctx.ServiceClient.Run()

	// Register a task and agent
	if err := ctx.ServiceClient.RegisterTask("allocid", ctx.Task, ctx, nil); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	// wait for Exec to get called before shutting down
	<-waiter

	// Shutdown should block until all enqueued operations finish.
	preShutdown := time.Now()
	if err := ctx.ServiceClient.Shutdown(); err != nil {
		t.Errorf("unexpected error shutting down client: %v", err)
	}

	// Shutdown time should have taken: 1s <= shutdown <= 3s
	shutdownTime := time.Now().Sub(preShutdown)
	if shutdownTime < time.Second || shutdownTime > ctx.ServiceClient.shutdownWait {
		t.Errorf("expected shutdown to take >1s and <%s but took: %s", ctx.ServiceClient.shutdownWait, shutdownTime)
	}

	// UpdateTTL should have been called once for the script check
	if n := len(ctx.FakeConsul.checkTTLs); n != 1 {
		t.Fatalf("expected 1 checkTTL entry but found: %d", n)
	}
	for _, v := range ctx.FakeConsul.checkTTLs {
		if v != 1 {
			t.Fatalf("expected script check to be updated once but found %d", v)
		}
	}
	for _, v := range ctx.FakeConsul.checks {
		if v.Status != "passing" {
			t.Fatalf("expected check to be passing but found %q", v.Status)
		}
	}
}

// TestConsul_ShutdownBlocked tests the blocked past deadline path for the
// shutdown logic in ServiceClient.
func TestConsul_ShutdownBlocked(t *testing.T) {
	t.Parallel() // run the slow tests in parallel
	ctx := setupFake()

	// Add a script check to make sure its TTL gets updated
	ctx.Task.Services[0].Checks = []*structs.ServiceCheck{
		{
			Name:    "scriptcheck",
			Type:    "script",
			Command: "true",
			// Make check block until shutdown
			Interval:      9000 * time.Hour,
			Timeout:       9000 * time.Hour,
			InitialStatus: "warning",
		},
	}

	block := make(chan struct{})
	defer close(block) // cleanup after test

	// Make Exec block forever
	waiter := make(chan struct{})
	ctx.ExecFunc = func(ctx context.Context, cmd string, args []string) ([]byte, int, error) {
		close(waiter)
		<-block
		return []byte{}, 0, nil
	}

	// Use a short shutdown deadline since we're intentionally blocking forever
	ctx.ServiceClient.shutdownWait = time.Second

	go ctx.ServiceClient.Run()

	// Register a task and agent
	if err := ctx.ServiceClient.RegisterTask("allocid", ctx.Task, ctx, nil); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	// Wait for exec to be called
	<-waiter

	// Shutdown should block until all enqueued operations finish.
	preShutdown := time.Now()
	err := ctx.ServiceClient.Shutdown()
	if err == nil {
		t.Errorf("expected a timed out error from shutdown")
	}

	// Shutdown time should have taken shutdownWait; to avoid timing
	// related errors simply test for wait <= shutdown <= wait+3s
	shutdownTime := time.Now().Sub(preShutdown)
	maxWait := ctx.ServiceClient.shutdownWait + (3 * time.Second)
	if shutdownTime < ctx.ServiceClient.shutdownWait || shutdownTime > maxWait {
		t.Errorf("expected shutdown to take >%s and <%s but took: %s", ctx.ServiceClient.shutdownWait, maxWait, shutdownTime)
	}

	// UpdateTTL should not have been called for the script check
	if n := len(ctx.FakeConsul.checkTTLs); n != 0 {
		t.Fatalf("expected 0 checkTTL entry but found: %d", n)
	}
	for _, v := range ctx.FakeConsul.checks {
		if expected := "warning"; v.Status != expected {
			t.Fatalf("expected check to be %q but found %q", expected, v.Status)
		}
	}
}

// TestConsul_NoTLSSkipVerifySupport asserts that checks with
// TLSSkipVerify=true are skipped when Consul doesn't support TLSSkipVerify.
func TestConsul_NoTLSSkipVerifySupport(t *testing.T) {
	ctx := setupFake()
	ctx.ServiceClient = NewServiceClient(ctx.FakeConsul, false, testLogger())
	ctx.Task.Services[0].Checks = []*structs.ServiceCheck{
		// This check sets TLSSkipVerify so it should get dropped
		{
			Name:          "tls-check-skip",
			Type:          "http",
			Protocol:      "https",
			Path:          "/",
			TLSSkipVerify: true,
		},
		// This check doesn't set TLSSkipVerify so it should work fine
		{
			Name:          "tls-check-noskip",
			Type:          "http",
			Protocol:      "https",
			Path:          "/",
			TLSSkipVerify: false,
		},
	}

	if err := ctx.ServiceClient.RegisterTask("allocid", ctx.Task, nil, nil); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if len(ctx.FakeConsul.checks) != 1 {
		t.Errorf("expected 1 check but found %d", len(ctx.FakeConsul.checks))
	}
	for _, v := range ctx.FakeConsul.checks {
		if expected := "tls-check-noskip"; v.Name != expected {
			t.Errorf("only expected %q but found: %q", expected, v.Name)
		}
		if v.TLSSkipVerify {
			t.Errorf("TLSSkipVerify=true when TLSSkipVerify not supported!")
		}
	}
}

// TestConsul_RemoveScript assert removing a script check removes all objects
// related to that check.
func TestConsul_CancelScript(t *testing.T) {
	ctx := setupFake()
	ctx.Task.Services[0].Checks = []*structs.ServiceCheck{
		{
			Name:     "scriptcheckDel",
			Type:     "script",
			Interval: 9000 * time.Hour,
			Timeout:  9000 * time.Hour,
		},
		{
			Name:     "scriptcheckKeep",
			Type:     "script",
			Interval: 9000 * time.Hour,
			Timeout:  9000 * time.Hour,
		},
	}

	if err := ctx.ServiceClient.RegisterTask("allocid", ctx.Task, ctx, nil); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if len(ctx.FakeConsul.checks) != 2 {
		t.Errorf("expected 2 checks but found %d", len(ctx.FakeConsul.checks))
	}

	if len(ctx.ServiceClient.scripts) != 2 && len(ctx.ServiceClient.runningScripts) != 2 {
		t.Errorf("expected 2 running script but found scripts=%d runningScripts=%d",
			len(ctx.ServiceClient.scripts), len(ctx.ServiceClient.runningScripts))
	}

	for i := 0; i < 2; i++ {
		select {
		case <-ctx.execs:
			// Script ran as expected!
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for script check to run")
		}
	}

	// Remove a check and update the task
	origTask := ctx.Task.Copy()
	ctx.Task.Services[0].Checks = []*structs.ServiceCheck{
		{
			Name:     "scriptcheckKeep",
			Type:     "script",
			Interval: 9000 * time.Hour,
			Timeout:  9000 * time.Hour,
		},
	}

	if err := ctx.ServiceClient.UpdateTask("allocid", origTask, ctx.Task, ctx, nil); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if len(ctx.FakeConsul.checks) != 1 {
		t.Errorf("expected 1 check but found %d", len(ctx.FakeConsul.checks))
	}

	if len(ctx.ServiceClient.scripts) != 1 && len(ctx.ServiceClient.runningScripts) != 1 {
		t.Errorf("expected 1 running script but found scripts=%d runningScripts=%d",
			len(ctx.ServiceClient.scripts), len(ctx.ServiceClient.runningScripts))
	}

	// Make sure exec wasn't called again
	select {
	case <-ctx.execs:
		t.Errorf("unexpected execution of script; was goroutine not cancelled?")
	case <-time.After(100 * time.Millisecond):
		// No unexpected script execs
	}

	// Don't leak goroutines
	for _, scriptHandle := range ctx.ServiceClient.runningScripts {
		scriptHandle.cancel()
	}
}

// TestConsul_DriverNetwork_AutoUse asserts that if a driver network has
// auto-use set then services should advertise it unless explicitly set to
// host. Checks should always use host.
func TestConsul_DriverNetwork_AutoUse(t *testing.T) {
	ctx := setupFake()

	ctx.Task.Services = []*structs.Service{
		{
			Name:        "auto-advertise-x",
			PortLabel:   "x",
			AddressMode: structs.AddressModeAuto,
			Checks: []*structs.ServiceCheck{
				{
					Name:     "default-check-x",
					Type:     "tcp",
					Interval: time.Second,
					Timeout:  time.Second,
				},
				{
					Name:      "weird-y-check",
					Type:      "http",
					Interval:  time.Second,
					Timeout:   time.Second,
					PortLabel: "y",
				},
			},
		},
		{
			Name:        "driver-advertise-y",
			PortLabel:   "y",
			AddressMode: structs.AddressModeDriver,
			Checks: []*structs.ServiceCheck{
				{
					Name:     "default-check-y",
					Type:     "tcp",
					Interval: time.Second,
					Timeout:  time.Second,
				},
			},
		},
		{
			Name:        "host-advertise-y",
			PortLabel:   "y",
			AddressMode: structs.AddressModeHost,
		},
	}

	net := &cstructs.DriverNetwork{
		PortMap: map[string]int{
			"x": 8888,
			"y": 9999,
		},
		IP:            "172.18.0.2",
		AutoAdvertise: true,
	}

	if err := ctx.ServiceClient.RegisterTask("allocid", ctx.Task, ctx, net); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if n := len(ctx.FakeConsul.services); n != 3 {
		t.Fatalf("expected 2 services but found: %d", n)
	}

	for _, v := range ctx.FakeConsul.services {
		switch v.Name {
		case ctx.Task.Services[0].Name: // x
			// Since DriverNetwork.AutoAdvertise=true, driver ports should be used
			if v.Port != net.PortMap["x"] {
				t.Errorf("expected service %s's port to be %d but found %d",
					v.Name, net.PortMap["x"], v.Port)
			}
			// The order of checks in Consul is not guaranteed to
			// be the same as their order in the Task definition,
			// so check in a loop
			if expected := 2; len(v.Checks) != expected {
				t.Errorf("expected %d checks but found %d", expected, len(v.Checks))
			}
			for _, c := range v.Checks {
				// No name on AgentServiceChecks, use type
				switch {
				case c.TCP != "":
					// Checks should always use host port though
					if c.TCP != ":1234" { // xPort
						t.Errorf("exepcted service %s check 1's port to be %d but found %q",
							v.Name, xPort, c.TCP)
					}
				case c.HTTP != "":
					if c.HTTP != "http://:1235" { // yPort
						t.Errorf("exepcted service %s check 2's port to be %d but found %q",
							v.Name, yPort, c.HTTP)
					}
				default:
					t.Errorf("unexpected check %#v on service %q", c, v.Name)
				}
			}
		case ctx.Task.Services[1].Name: // y
			// Service should be container ip:port
			if v.Address != net.IP {
				t.Errorf("expected service %s's address to be %s but found %s",
					v.Name, net.IP, v.Address)
			}
			if v.Port != net.PortMap["y"] {
				t.Errorf("expected service %s's port to be %d but found %d",
					v.Name, net.PortMap["x"], v.Port)
			}
			// Check should be host ip:port
			if v.Checks[0].TCP != ":1235" { // yPort
				t.Errorf("expected service %s check's port to be %d but found %s",
					v.Name, yPort, v.Checks[0].TCP)
			}
		case ctx.Task.Services[2].Name: // y + host mode
			if v.Port != yPort {
				t.Errorf("expected service %s's port to be %d but found %d",
					v.Name, yPort, v.Port)
			}
		default:
			t.Errorf("unexpected service name: %q", v.Name)
		}
	}
}

// TestConsul_DriverNetwork_NoAutoUse asserts that if a driver network doesn't
// set auto-use only services which request the driver's network should
// advertise it.
func TestConsul_DriverNetwork_NoAutoUse(t *testing.T) {
	ctx := setupFake()

	ctx.Task.Services = []*structs.Service{
		{
			Name:        "auto-advertise-x",
			PortLabel:   "x",
			AddressMode: structs.AddressModeAuto,
		},
		{
			Name:        "driver-advertise-y",
			PortLabel:   "y",
			AddressMode: structs.AddressModeDriver,
		},
		{
			Name:        "host-advertise-y",
			PortLabel:   "y",
			AddressMode: structs.AddressModeHost,
		},
	}

	net := &cstructs.DriverNetwork{
		PortMap: map[string]int{
			"x": 8888,
			"y": 9999,
		},
		IP:            "172.18.0.2",
		AutoAdvertise: false,
	}

	if err := ctx.ServiceClient.RegisterTask("allocid", ctx.Task, ctx, net); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if n := len(ctx.FakeConsul.services); n != 3 {
		t.Fatalf("expected 3 services but found: %d", n)
	}

	for _, v := range ctx.FakeConsul.services {
		switch v.Name {
		case ctx.Task.Services[0].Name: // x + auto
			// Since DriverNetwork.AutoAdvertise=false, host ports should be used
			if v.Port != xPort {
				t.Errorf("expected service %s's port to be %d but found %d",
					v.Name, xPort, v.Port)
			}
		case ctx.Task.Services[1].Name: // y + driver mode
			// Service should be container ip:port
			if v.Address != net.IP {
				t.Errorf("expected service %s's address to be %s but found %s",
					v.Name, net.IP, v.Address)
			}
			if v.Port != net.PortMap["y"] {
				t.Errorf("expected service %s's port to be %d but found %d",
					v.Name, net.PortMap["x"], v.Port)
			}
		case ctx.Task.Services[2].Name: // y + host mode
			if v.Port != yPort {
				t.Errorf("expected service %s's port to be %d but found %d",
					v.Name, yPort, v.Port)
			}
		default:
			t.Errorf("unexpected service name: %q", v.Name)
		}
	}
}

// TestConsul_DriverNetwork_Change asserts that if a driver network is
// specified and a service updates its use its properly updated in Consul.
func TestConsul_DriverNetwork_Change(t *testing.T) {
	ctx := setupFake()

	ctx.Task.Services = []*structs.Service{
		{
			Name:        "service-foo",
			PortLabel:   "x",
			AddressMode: structs.AddressModeAuto,
		},
	}

	net := &cstructs.DriverNetwork{
		PortMap: map[string]int{
			"x": 8888,
			"y": 9999,
		},
		IP:            "172.18.0.2",
		AutoAdvertise: false,
	}

	syncAndAssertPort := func(port int) {
		if err := ctx.syncOnce(); err != nil {
			t.Fatalf("unexpected error syncing task: %v", err)
		}

		if n := len(ctx.FakeConsul.services); n != 1 {
			t.Fatalf("expected 1 service but found: %d", n)
		}

		for _, v := range ctx.FakeConsul.services {
			switch v.Name {
			case ctx.Task.Services[0].Name:
				if v.Port != port {
					t.Errorf("expected service %s's port to be %d but found %d",
						v.Name, port, v.Port)
				}
			default:
				t.Errorf("unexpected service name: %q", v.Name)
			}
		}
	}

	// Initial service should advertise host port x
	if err := ctx.ServiceClient.RegisterTask("allocid", ctx.Task, ctx, net); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	syncAndAssertPort(xPort)

	// UpdateTask to use Host (shouldn't change anything)
	orig := ctx.Task.Copy()
	ctx.Task.Services[0].AddressMode = structs.AddressModeHost

	if err := ctx.ServiceClient.UpdateTask("allocid", orig, ctx.Task, ctx, net); err != nil {
		t.Fatalf("unexpected error updating task: %v", err)
	}

	syncAndAssertPort(xPort)

	// UpdateTask to use Driver (*should* change IP and port)
	orig = ctx.Task.Copy()
	ctx.Task.Services[0].AddressMode = structs.AddressModeDriver

	if err := ctx.ServiceClient.UpdateTask("allocid", orig, ctx.Task, ctx, net); err != nil {
		t.Fatalf("unexpected error updating task: %v", err)
	}

	syncAndAssertPort(net.PortMap["x"])
}

// TestIsNomadService asserts the isNomadService helper returns true for Nomad
// task IDs and false for unknown IDs and Nomad agent IDs (see #2827).
func TestIsNomadService(t *testing.T) {
	tests := []struct {
		id     string
		result bool
	}{
		{"_nomad-client-nomad-client-http", false},
		{"_nomad-server-nomad-serf", false},
		{"_nomad-executor-abc", true},
		{"_nomad-executor", true},
		{"not-nomad", false},
	}

	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			actual := isNomadService(test.id)
			if actual != test.result {
				t.Errorf("%q should be %t but found %t", test.id, test.result, actual)
			}
		})
	}
}

// TestCreateCheckReg asserts Nomad ServiceCheck structs are properly converted
// to Consul API AgentCheckRegistrations.
func TestCreateCheckReg(t *testing.T) {
	check := &structs.ServiceCheck{
		Name:      "name",
		Type:      "http",
		Path:      "/path",
		PortLabel: "label",
		Method:    "POST",
		Header: map[string][]string{
			"Foo": {"bar"},
		},
	}

	serviceID := "testService"
	checkID := check.Hash(serviceID)
	host := "localhost"
	port := 41111

	expected := &api.AgentCheckRegistration{
		ID:        checkID,
		Name:      "name",
		ServiceID: serviceID,
		AgentServiceCheck: api.AgentServiceCheck{
			Timeout:  "0s",
			Interval: "0s",
			HTTP:     fmt.Sprintf("http://%s:%d/path", host, port),
			Method:   "POST",
			Header: map[string][]string{
				"Foo": {"bar"},
			},
		},
	}

	actual, err := createCheckReg(serviceID, checkID, check, host, port)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if diff := pretty.Diff(actual, expected); len(diff) > 0 {
		t.Fatalf("diff:\n%s\n", strings.Join(diff, "\n"))
	}
}
