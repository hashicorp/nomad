package consul

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Ports used in testTask
	xPort = 1234
	yPort = 1235
)

func testTask() *TaskServices {
	return &TaskServices{
		AllocID:   uuid.Generate(),
		Name:      "taskname",
		Restarter: &restartRecorder{},
		Services: []*structs.Service{
			{
				Name:      "taskname-service",
				PortLabel: "x",
				Tags:      []string{"tag1", "tag2"},
			},
		},
		Networks: []*structs.NetworkResource{
			{
				DynamicPorts: []structs.Port{
					{Label: "x", Value: xPort},
					{Label: "y", Value: yPort},
				},
			},
		},
		DriverExec: newMockExec(),
	}
}

// mockExec implements the ScriptExecutor interface and will use an alternate
// implementation t.ExecFunc if non-nil.
type mockExec struct {
	// Ticked whenever a script is called
	execs chan int

	// If non-nil will be called by script checks
	ExecFunc func(ctx context.Context, cmd string, args []string) ([]byte, int, error)
}

func newMockExec() *mockExec {
	return &mockExec{
		execs: make(chan int, 100),
	}
}

func (m *mockExec) Exec(dur time.Duration, cmd string, args []string) ([]byte, int, error) {
	select {
	case m.execs <- 1:
	default:
	}
	if m.ExecFunc == nil {
		// Default impl is just "ok"
		return []byte("ok"), 0, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()
	return m.ExecFunc(ctx, cmd, args)
}

// restartRecorder is a minimal TaskRestarter implementation that simply
// counts how many restarts were triggered.
type restartRecorder struct {
	restarts int64
}

func (r *restartRecorder) Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error {
	atomic.AddInt64(&r.restarts, 1)
	return nil
}

// testFakeCtx contains a fake Consul AgentAPI
type testFakeCtx struct {
	ServiceClient *ServiceClient
	FakeConsul    *MockAgent
	Task          *TaskServices
	MockExec      *mockExec
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
func setupFake(t *testing.T) *testFakeCtx {
	fc := NewMockAgent()
	tt := testTask()
	return &testFakeCtx{
		ServiceClient: NewServiceClient(fc, testlog.HCLogger(t), true),
		FakeConsul:    fc,
		Task:          tt,
		MockExec:      tt.DriverExec.(*mockExec),
	}
}

func TestConsul_ChangeTags(t *testing.T) {
	ctx := setupFake(t)

	if err := ctx.ServiceClient.RegisterTask(ctx.Task); err != nil {
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
	reg1, err := ctx.ServiceClient.AllocRegistrations(ctx.Task.AllocID)
	if err != nil {
		t.Fatalf("Looking up alloc registration failed: %v", err)
	}
	if reg1 == nil {
		t.Fatalf("Nil alloc registrations: %v", err)
	}
	if num := reg1.NumServices(); num != 1 {
		t.Fatalf("Wrong number of services: got %d; want 1", num)
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

	origTask := ctx.Task.Copy()
	ctx.Task.Services[0].Tags[0] = "newtag"
	if err := ctx.ServiceClient.UpdateTask(origTask, ctx.Task); err != nil {
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
	reg2, err := ctx.ServiceClient.AllocRegistrations(ctx.Task.AllocID)
	if err != nil {
		t.Fatalf("Looking up alloc registration failed: %v", err)
	}
	if reg2 == nil {
		t.Fatalf("Nil alloc registrations: %v", err)
	}
	if num := reg2.NumServices(); num != 1 {
		t.Fatalf("Wrong number of services: got %d; want 1", num)
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
// it in Consul. Pre-0.7.1 ports were not part of the service ID and this was a
// slightly different code path than changing tags.
func TestConsul_ChangePorts(t *testing.T) {
	ctx := setupFake(t)
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

	if err := ctx.ServiceClient.RegisterTask(ctx.Task); err != nil {
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
			case <-ctx.MockExec.execs:
				if n := len(ctx.MockExec.execs); n > 0 {
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
	origTask := ctx.Task.Copy()
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
	if err := ctx.ServiceClient.UpdateTask(origTask, ctx.Task); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}
	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if n := len(ctx.FakeConsul.services); n != 1 {
		t.Fatalf("expected 1 service but found %d:\n%#v", n, ctx.FakeConsul.services)
	}

	for k, v := range ctx.FakeConsul.services {
		if k == origServiceKey {
			t.Errorf("expected key change; still: %q", k)
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
			if k == origTCPKey {
				t.Errorf("expected key change for %s from %q", v.Name, origTCPKey)
			}
			if expected := fmt.Sprintf(":%d", xPort); v.TCP != expected {
				t.Errorf("expected Port x=%v but found: %v", expected, v.TCP)
			}
		case "c2":
			if k == origScriptKey {
				t.Errorf("expected key change for %s from %q", v.Name, origScriptKey)
			}
			select {
			case <-ctx.MockExec.execs:
				if n := len(ctx.MockExec.execs); n > 0 {
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
	ctx := setupFake(t)
	ctx.Task.Services[0].Checks = []*structs.ServiceCheck{
		{
			Name:      "c1",
			Type:      "tcp",
			Interval:  time.Second,
			Timeout:   time.Second,
			PortLabel: "x",
			CheckRestart: &structs.CheckRestart{
				Limit: 3,
			},
		},
	}

	if err := ctx.ServiceClient.RegisterTask(ctx.Task); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if n := len(ctx.FakeConsul.services); n != 1 {
		t.Fatalf("expected 1 service but found %d:\n%#v", n, ctx.FakeConsul.services)
	}

	// Assert a check restart watch update was enqueued and clear it
	if n := len(ctx.ServiceClient.checkWatcher.checkUpdateCh); n != 1 {
		t.Fatalf("expected 1 check restart update but found %d", n)
	}
	upd := <-ctx.ServiceClient.checkWatcher.checkUpdateCh
	c1ID := upd.checkID

	// Query the allocs registrations and then again when we update. The IDs
	// should change
	reg1, err := ctx.ServiceClient.AllocRegistrations(ctx.Task.AllocID)
	if err != nil {
		t.Fatalf("Looking up alloc registration failed: %v", err)
	}
	if reg1 == nil {
		t.Fatalf("Nil alloc registrations: %v", err)
	}
	if num := reg1.NumServices(); num != 1 {
		t.Fatalf("Wrong number of services: got %d; want 1", num)
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
			CheckRestart: &structs.CheckRestart{
				Limit: 3,
			},
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
	if err := ctx.ServiceClient.UpdateTask(origTask, ctx.Task); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	// Assert 2 check restart watch updates was enqueued
	if n := len(ctx.ServiceClient.checkWatcher.checkUpdateCh); n != 2 {
		t.Fatalf("expected 2 check restart updates but found %d", n)
	}

	// First the new watch
	upd = <-ctx.ServiceClient.checkWatcher.checkUpdateCh
	if upd.checkID == c1ID || upd.remove {
		t.Fatalf("expected check watch update to be an add of checkID=%q but found remove=%t checkID=%q",
			c1ID, upd.remove, upd.checkID)
	}

	// Then remove the old watch
	upd = <-ctx.ServiceClient.checkWatcher.checkUpdateCh
	if upd.checkID != c1ID || !upd.remove {
		t.Fatalf("expected check watch update to be a removal of checkID=%q but found remove=%t checkID=%q",
			c1ID, upd.remove, upd.checkID)
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

			// update id
			c1ID = k
		case "c2":
			if expected := fmt.Sprintf("http://:%d/", xPort); v.HTTP != expected {
				t.Errorf("expected Port x=%v but found: %v", expected, v.HTTP)
			}
		default:
			t.Errorf("Unknown check: %q", k)
		}
	}

	// Check again and ensure the IDs changed
	reg2, err := ctx.ServiceClient.AllocRegistrations(ctx.Task.AllocID)
	if err != nil {
		t.Fatalf("Looking up alloc registration failed: %v", err)
	}
	if reg2 == nil {
		t.Fatalf("Nil alloc registrations: %v", err)
	}
	if num := reg2.NumServices(); num != 1 {
		t.Fatalf("Wrong number of services: got %d; want 1", num)
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

	// Alter a CheckRestart and make sure the watcher is updated but nothing else
	origTask = ctx.Task.Copy()
	ctx.Task.Services[0].Checks = []*structs.ServiceCheck{
		{
			Name:      "c1",
			Type:      "tcp",
			Interval:  2 * time.Second,
			Timeout:   time.Second,
			PortLabel: "x",
			CheckRestart: &structs.CheckRestart{
				Limit: 11,
			},
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
	if err := ctx.ServiceClient.UpdateTask(origTask, ctx.Task); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}
	if err := ctx.syncOnce(); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if n := len(ctx.FakeConsul.checks); n != 2 {
		t.Fatalf("expected 2 check but found %d:\n%#v", n, ctx.FakeConsul.checks)
	}

	for k, v := range ctx.FakeConsul.checks {
		if v.Name == "c1" {
			if k != c1ID {
				t.Errorf("expected c1 to still have id %q but found %q", c1ID, k)
			}
			break
		}
	}

	// Assert a check restart watch update was enqueued for a removal and an add
	if n := len(ctx.ServiceClient.checkWatcher.checkUpdateCh); n != 1 {
		t.Fatalf("expected 1 check restart update but found %d", n)
	}
	<-ctx.ServiceClient.checkWatcher.checkUpdateCh
}

// TestConsul_RegServices tests basic service registration.
func TestConsul_RegServices(t *testing.T) {
	ctx := setupFake(t)

	// Add a check w/restarting
	ctx.Task.Services[0].Checks = []*structs.ServiceCheck{
		{
			Name:     "testcheck",
			Type:     "tcp",
			Interval: 100 * time.Millisecond,
			CheckRestart: &structs.CheckRestart{
				Limit: 3,
			},
		},
	}

	if err := ctx.ServiceClient.RegisterTask(ctx.Task); err != nil {
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

	// Assert the check update is pending
	if n := len(ctx.ServiceClient.checkWatcher.checkUpdateCh); n != 1 {
		t.Fatalf("expected 1 check restart update but found %d", n)
	}

	// Assert the check update is properly formed
	checkUpd := <-ctx.ServiceClient.checkWatcher.checkUpdateCh
	if checkUpd.checkRestart.allocID != ctx.Task.AllocID {
		t.Fatalf("expected check's allocid to be %q but found %q", "allocid", checkUpd.checkRestart.allocID)
	}
	if expected := 200 * time.Millisecond; checkUpd.checkRestart.timeLimit != expected {
		t.Fatalf("expected check's time limit to be %v but found %v", expected, checkUpd.checkRestart.timeLimit)
	}

	// Make a change which will register a new service
	ctx.Task.Services[0].Name = "taskname-service2"
	ctx.Task.Services[0].Tags[0] = "tag3"
	if err := ctx.ServiceClient.RegisterTask(ctx.Task); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	// Assert check update is pending
	if n := len(ctx.ServiceClient.checkWatcher.checkUpdateCh); n != 1 {
		t.Fatalf("expected 1 check restart update but found %d", n)
	}

	// Assert the check update's id has changed
	checkUpd2 := <-ctx.ServiceClient.checkWatcher.checkUpdateCh
	if checkUpd.checkID == checkUpd2.checkID {
		t.Fatalf("expected new check update to have a new ID both both have: %q", checkUpd.checkID)
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
	ctx.ServiceClient.RemoveTask(ctx.Task)
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

	// Assert check update is pending
	if n := len(ctx.ServiceClient.checkWatcher.checkUpdateCh); n != 1 {
		t.Fatalf("expected 1 check restart update but found %d", n)
	}

	// Assert the check update's id is correct and that it's a removal
	checkUpd3 := <-ctx.ServiceClient.checkWatcher.checkUpdateCh
	if checkUpd2.checkID != checkUpd3.checkID {
		t.Fatalf("expected checkid %q but found %q", checkUpd2.checkID, checkUpd3.checkID)
	}
	if !checkUpd3.remove {
		t.Fatalf("expected check watch removal update but found: %#v", checkUpd3)
	}
}

// TestConsul_ShutdownOK tests the ok path for the shutdown logic in
// ServiceClient.
func TestConsul_ShutdownOK(t *testing.T) {
	require := require.New(t)
	ctx := setupFake(t)

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
	if err := ctx.ServiceClient.RegisterTask(ctx.Task); err != nil {
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

	testutil.WaitForResult(func() (bool, error) {
		return ctx.ServiceClient.hasSeen(), fmt.Errorf("error contacting Consul")
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Shutdown should block until scripts finish
	if err := ctx.ServiceClient.Shutdown(); err != nil {
		t.Errorf("unexpected error shutting down client: %v", err)
	}

	// UpdateTTL should have been called once for the script check and once
	// for shutdown
	if n := len(ctx.FakeConsul.checkTTLs); n != 1 {
		t.Fatalf("expected 1 checkTTL entry but found: %d", n)
	}
	for _, v := range ctx.FakeConsul.checkTTLs {
		require.Equalf(2, v, "expected 2 updates but found %d", v)
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
	t.Parallel()
	ctx := setupFake(t)

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
	ctx.MockExec.ExecFunc = func(ctx context.Context, cmd string, args []string) ([]byte, int, error) {
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
	if err := ctx.ServiceClient.RegisterTask(ctx.Task); err != nil {
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
	t.Parallel()
	ctx := setupFake(t)

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
	ctx.MockExec.ExecFunc = func(ctx context.Context, cmd string, args []string) ([]byte, int, error) {
		close(waiter)
		<-block
		return []byte{}, 0, nil
	}

	// Use a short shutdown deadline since we're intentionally blocking forever
	ctx.ServiceClient.shutdownWait = time.Second

	go ctx.ServiceClient.Run()

	// Register a task and agent
	if err := ctx.ServiceClient.RegisterTask(ctx.Task); err != nil {
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

// TestConsul_RemoveScript assert removing a script check removes all objects
// related to that check.
func TestConsul_CancelScript(t *testing.T) {
	ctx := setupFake(t)
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

	if err := ctx.ServiceClient.RegisterTask(ctx.Task); err != nil {
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
		case <-ctx.MockExec.execs:
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

	if err := ctx.ServiceClient.UpdateTask(origTask, ctx.Task); err != nil {
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
	case <-ctx.MockExec.execs:
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
	t.Parallel()
	ctx := setupFake(t)

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

	ctx.Task.DriverNetwork = &drivers.DriverNetwork{
		PortMap: map[string]int{
			"x": 8888,
			"y": 9999,
		},
		IP:            "172.18.0.2",
		AutoAdvertise: true,
	}

	if err := ctx.ServiceClient.RegisterTask(ctx.Task); err != nil {
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
			if v.Port != ctx.Task.DriverNetwork.PortMap["x"] {
				t.Errorf("expected service %s's port to be %d but found %d",
					v.Name, ctx.Task.DriverNetwork.PortMap["x"], v.Port)
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
						t.Errorf("expected service %s check 1's port to be %d but found %q",
							v.Name, xPort, c.TCP)
					}
				case c.HTTP != "":
					if c.HTTP != "http://:1235" { // yPort
						t.Errorf("expected service %s check 2's port to be %d but found %q",
							v.Name, yPort, c.HTTP)
					}
				default:
					t.Errorf("unexpected check %#v on service %q", c, v.Name)
				}
			}
		case ctx.Task.Services[1].Name: // y
			// Service should be container ip:port
			if v.Address != ctx.Task.DriverNetwork.IP {
				t.Errorf("expected service %s's address to be %s but found %s",
					v.Name, ctx.Task.DriverNetwork.IP, v.Address)
			}
			if v.Port != ctx.Task.DriverNetwork.PortMap["y"] {
				t.Errorf("expected service %s's port to be %d but found %d",
					v.Name, ctx.Task.DriverNetwork.PortMap["x"], v.Port)
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
	t.Parallel()
	ctx := setupFake(t)

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

	ctx.Task.DriverNetwork = &drivers.DriverNetwork{
		PortMap: map[string]int{
			"x": 8888,
			"y": 9999,
		},
		IP:            "172.18.0.2",
		AutoAdvertise: false,
	}

	if err := ctx.ServiceClient.RegisterTask(ctx.Task); err != nil {
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
			if v.Address != ctx.Task.DriverNetwork.IP {
				t.Errorf("expected service %s's address to be %s but found %s",
					v.Name, ctx.Task.DriverNetwork.IP, v.Address)
			}
			if v.Port != ctx.Task.DriverNetwork.PortMap["y"] {
				t.Errorf("expected service %s's port to be %d but found %d",
					v.Name, ctx.Task.DriverNetwork.PortMap["x"], v.Port)
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
	t.Parallel()
	ctx := setupFake(t)

	ctx.Task.Services = []*structs.Service{
		{
			Name:        "service-foo",
			PortLabel:   "x",
			AddressMode: structs.AddressModeAuto,
		},
	}

	ctx.Task.DriverNetwork = &drivers.DriverNetwork{
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
	if err := ctx.ServiceClient.RegisterTask(ctx.Task); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	syncAndAssertPort(xPort)

	// UpdateTask to use Host (shouldn't change anything)
	origTask := ctx.Task.Copy()
	ctx.Task.Services[0].AddressMode = structs.AddressModeHost

	if err := ctx.ServiceClient.UpdateTask(origTask, ctx.Task); err != nil {
		t.Fatalf("unexpected error updating task: %v", err)
	}

	syncAndAssertPort(xPort)

	// UpdateTask to use Driver (*should* change IP and port)
	origTask = ctx.Task.Copy()
	ctx.Task.Services[0].AddressMode = structs.AddressModeDriver

	if err := ctx.ServiceClient.UpdateTask(origTask, ctx.Task); err != nil {
		t.Fatalf("unexpected error updating task: %v", err)
	}

	syncAndAssertPort(ctx.Task.DriverNetwork.PortMap["x"])
}

// TestConsul_CanaryTags asserts CanaryTags are used when Canary=true
func TestConsul_CanaryTags(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctx := setupFake(t)

	canaryTags := []string{"tag1", "canary"}
	ctx.Task.Canary = true
	ctx.Task.Services[0].CanaryTags = canaryTags

	require.NoError(ctx.ServiceClient.RegisterTask(ctx.Task))
	require.NoError(ctx.syncOnce())
	require.Len(ctx.FakeConsul.services, 1)
	for _, service := range ctx.FakeConsul.services {
		require.Equal(canaryTags, service.Tags)
	}

	// Disable canary and assert tags are not the canary tags
	origTask := ctx.Task.Copy()
	ctx.Task.Canary = false
	require.NoError(ctx.ServiceClient.UpdateTask(origTask, ctx.Task))
	require.NoError(ctx.syncOnce())
	require.Len(ctx.FakeConsul.services, 1)
	for _, service := range ctx.FakeConsul.services {
		require.NotEqual(canaryTags, service.Tags)
	}

	ctx.ServiceClient.RemoveTask(ctx.Task)
	require.NoError(ctx.syncOnce())
	require.Len(ctx.FakeConsul.services, 0)
}

// TestConsul_CanaryTags_NoTags asserts Tags are used when Canary=true and there
// are no specified canary tags
func TestConsul_CanaryTags_NoTags(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctx := setupFake(t)

	tags := []string{"tag1", "foo"}
	ctx.Task.Canary = true
	ctx.Task.Services[0].Tags = tags

	require.NoError(ctx.ServiceClient.RegisterTask(ctx.Task))
	require.NoError(ctx.syncOnce())
	require.Len(ctx.FakeConsul.services, 1)
	for _, service := range ctx.FakeConsul.services {
		require.Equal(tags, service.Tags)
	}

	// Disable canary and assert tags dont change
	origTask := ctx.Task.Copy()
	ctx.Task.Canary = false
	require.NoError(ctx.ServiceClient.UpdateTask(origTask, ctx.Task))
	require.NoError(ctx.syncOnce())
	require.Len(ctx.FakeConsul.services, 1)
	for _, service := range ctx.FakeConsul.services {
		require.Equal(tags, service.Tags)
	}

	ctx.ServiceClient.RemoveTask(ctx.Task)
	require.NoError(ctx.syncOnce())
	require.Len(ctx.FakeConsul.services, 0)
}

// TestConsul_PeriodicSync asserts that Nomad periodically reconciles with
// Consul.
func TestConsul_PeriodicSync(t *testing.T) {
	t.Parallel()

	ctx := setupFake(t)
	defer ctx.ServiceClient.Shutdown()

	// Lower periodic sync interval to speed up test
	ctx.ServiceClient.periodicInterval = 2 * time.Millisecond

	// Run for 10ms and assert hits >= 5 because each sync() calls multiple
	// Consul APIs
	go ctx.ServiceClient.Run()

	select {
	case <-ctx.ServiceClient.exitCh:
		t.Fatalf("exited unexpectedly")
	case <-time.After(10 * time.Millisecond):
	}

	minHits := 5
	if hits := ctx.FakeConsul.getHits(); hits < minHits {
		t.Fatalf("expected at least %d hits but found %d", minHits, hits)
	}
}

// TestIsNomadService asserts the isNomadService helper returns true for Nomad
// task IDs and false for unknown IDs and Nomad agent IDs (see #2827).
func TestIsNomadService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id     string
		result bool
	}{
		{"_nomad-client-nomad-client-http", false},
		{"_nomad-server-nomad-serf", false},

		// Pre-0.7.1 style IDs still match
		{"_nomad-executor-abc", true},
		{"_nomad-executor", true},

		// Post-0.7.1 style IDs match
		{"_nomad-task-FBBK265QN4TMT25ND4EP42TJVMYJ3HR4", true},

		{"not-nomad", false},
		{"_nomad", false},
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

// TestCreateCheckReg_HTTP asserts Nomad ServiceCheck structs are properly
// converted to Consul API AgentCheckRegistrations for HTTP checks.
func TestCreateCheckReg_HTTP(t *testing.T) {
	t.Parallel()
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

// TestCreateCheckReg_GRPC asserts Nomad ServiceCheck structs are properly
// converted to Consul API AgentCheckRegistrations for GRPC checks.
func TestCreateCheckReg_GRPC(t *testing.T) {
	t.Parallel()
	check := &structs.ServiceCheck{
		Name:          "name",
		Type:          "grpc",
		PortLabel:     "label",
		GRPCService:   "foo.Bar",
		GRPCUseTLS:    true,
		TLSSkipVerify: true,
		Timeout:       time.Second,
		Interval:      time.Minute,
	}

	serviceID := "testService"
	checkID := check.Hash(serviceID)

	expected := &api.AgentCheckRegistration{
		ID:        checkID,
		Name:      "name",
		ServiceID: serviceID,
		AgentServiceCheck: api.AgentServiceCheck{
			Timeout:       "1s",
			Interval:      "1m0s",
			GRPC:          "localhost:8080/foo.Bar",
			GRPCUseTLS:    true,
			TLSSkipVerify: true,
		},
	}

	actual, err := createCheckReg(serviceID, checkID, check, "localhost", 8080)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestGetAddress asserts Nomad uses the correct ip and port for services and
// checks depending on port labels, driver networks, and address mode.
func TestGetAddress(t *testing.T) {
	const HostIP = "127.0.0.1"

	cases := []struct {
		Name string

		// Parameters
		Mode      string
		PortLabel string
		Host      map[string]int // will be converted to structs.Networks
		Driver    *drivers.DriverNetwork

		// Results
		ExpectedIP   string
		ExpectedPort int
		ExpectedErr  string
	}{
		// Valid Configurations
		{
			Name:      "ExampleService",
			Mode:      structs.AddressModeAuto,
			PortLabel: "db",
			Host:      map[string]int{"db": 12435},
			Driver: &drivers.DriverNetwork{
				PortMap: map[string]int{"db": 6379},
				IP:      "10.1.2.3",
			},
			ExpectedIP:   HostIP,
			ExpectedPort: 12435,
		},
		{
			Name:      "Host",
			Mode:      structs.AddressModeHost,
			PortLabel: "db",
			Host:      map[string]int{"db": 12345},
			Driver: &drivers.DriverNetwork{
				PortMap: map[string]int{"db": 6379},
				IP:      "10.1.2.3",
			},
			ExpectedIP:   HostIP,
			ExpectedPort: 12345,
		},
		{
			Name:      "Driver",
			Mode:      structs.AddressModeDriver,
			PortLabel: "db",
			Host:      map[string]int{"db": 12345},
			Driver: &drivers.DriverNetwork{
				PortMap: map[string]int{"db": 6379},
				IP:      "10.1.2.3",
			},
			ExpectedIP:   "10.1.2.3",
			ExpectedPort: 6379,
		},
		{
			Name:      "AutoDriver",
			Mode:      structs.AddressModeAuto,
			PortLabel: "db",
			Host:      map[string]int{"db": 12345},
			Driver: &drivers.DriverNetwork{
				PortMap:       map[string]int{"db": 6379},
				IP:            "10.1.2.3",
				AutoAdvertise: true,
			},
			ExpectedIP:   "10.1.2.3",
			ExpectedPort: 6379,
		},
		{
			Name:      "DriverCustomPort",
			Mode:      structs.AddressModeDriver,
			PortLabel: "7890",
			Host:      map[string]int{"db": 12345},
			Driver: &drivers.DriverNetwork{
				PortMap: map[string]int{"db": 6379},
				IP:      "10.1.2.3",
			},
			ExpectedIP:   "10.1.2.3",
			ExpectedPort: 7890,
		},

		// Invalid Configurations
		{
			Name:        "DriverWithoutNetwork",
			Mode:        structs.AddressModeDriver,
			PortLabel:   "db",
			Host:        map[string]int{"db": 12345},
			Driver:      nil,
			ExpectedErr: "no driver network exists",
		},
		{
			Name:      "DriverBadPort",
			Mode:      structs.AddressModeDriver,
			PortLabel: "bad-port-label",
			Host:      map[string]int{"db": 12345},
			Driver: &drivers.DriverNetwork{
				PortMap: map[string]int{"db": 6379},
				IP:      "10.1.2.3",
			},
			ExpectedErr: "invalid port",
		},
		{
			Name:      "DriverZeroPort",
			Mode:      structs.AddressModeDriver,
			PortLabel: "0",
			Driver: &drivers.DriverNetwork{
				IP: "10.1.2.3",
			},
			ExpectedErr: "invalid port",
		},
		{
			Name:        "HostBadPort",
			Mode:        structs.AddressModeHost,
			PortLabel:   "bad-port-label",
			ExpectedErr: "invalid port",
		},
		{
			Name:        "InvalidMode",
			Mode:        "invalid-mode",
			PortLabel:   "80",
			ExpectedErr: "invalid address mode",
		},
		{
			Name:       "NoPort_AutoMode",
			Mode:       structs.AddressModeAuto,
			ExpectedIP: HostIP,
		},
		{
			Name:       "NoPort_HostMode",
			Mode:       structs.AddressModeHost,
			ExpectedIP: HostIP,
		},
		{
			Name: "NoPort_DriverMode",
			Mode: structs.AddressModeDriver,
			Driver: &drivers.DriverNetwork{
				IP: "10.1.2.3",
			},
			ExpectedIP: "10.1.2.3",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			// convert host port map into a structs.Networks
			networks := []*structs.NetworkResource{
				{
					IP:            HostIP,
					ReservedPorts: make([]structs.Port, len(tc.Host)),
				},
			}

			i := 0
			for label, port := range tc.Host {
				networks[0].ReservedPorts[i].Label = label
				networks[0].ReservedPorts[i].Value = port
				i++
			}

			// Run getAddress
			ip, port, err := getAddress(tc.Mode, tc.PortLabel, networks, tc.Driver)

			// Assert the results
			assert.Equal(t, tc.ExpectedIP, ip, "IP mismatch")
			assert.Equal(t, tc.ExpectedPort, port, "Port mismatch")
			if tc.ExpectedErr == "" {
				assert.Nil(t, err)
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q but err=nil", tc.ExpectedErr)
				} else {
					assert.Contains(t, err.Error(), tc.ExpectedErr)
				}
			}
		})
	}
}
