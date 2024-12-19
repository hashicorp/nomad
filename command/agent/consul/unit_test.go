// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/kr/pretty"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

const (
	// Ports used in testWorkload
	xPort = 1234
	yPort = 1235
)

func testWorkload() *serviceregistration.WorkloadServices {
	return &serviceregistration.WorkloadServices{
		AllocInfo: structs.AllocInfo{
			AllocID: uuid.Generate(),
			Task:    "taskname",
		},
		Restarter: &restartRecorder{},
		Services: []*structs.Service{
			{
				Name:      "taskname-service",
				PortLabel: "x",
				Tags:      []string{"tag1", "tag2"},
				Meta:      map[string]string{"meta1": "foo"},
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
	}
}

// restartRecorder is a minimal WorkloadRestarter implementation that simply
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
	Workload      *serviceregistration.WorkloadServices
}

var errNoOps = fmt.Errorf("testing error: no pending operations")

// syncOps simulates one iteration of the ServiceClient.Run loop and returns
// any errors returned by sync() or errNoOps if no pending operations.
func (t *testFakeCtx) syncOnce(reason syncReason) error {
	switch reason {

	case syncPeriodic:
		err := t.ServiceClient.sync(syncPeriodic)
		if err == nil {
			t.ServiceClient.clearExplicitlyDeregistered()
		}
		return err

	case syncNewOps:
		select {
		case ops := <-t.ServiceClient.opCh:
			t.ServiceClient.merge(ops)
			err := t.ServiceClient.sync(syncNewOps)
			if err == nil {
				t.ServiceClient.clearExplicitlyDeregistered()
			}
			return err
		default:
			return errNoOps
		}

	case syncShutdown:
		return errors.New("no test for sync due to shutdown")
	}

	return errors.New("bad sync reason")
}

// setupFake creates a testFakeCtx with a ServiceClient backed by a fakeConsul.
// A test Workload is also provided.
func setupFake(t *testing.T) *testFakeCtx {
	agentClient := NewMockAgent(ossFeatures)
	nsClient := NewNamespacesClient(NewMockNamespaces(nil), agentClient)
	workload := testWorkload()

	// by default start fake client being out of probation
	serviceClient := NewServiceClient(agentClient, nsClient, testlog.HCLogger(t), true)
	serviceClient.deregisterProbationExpiry = time.Now().Add(-1 * time.Minute)

	return &testFakeCtx{
		ServiceClient: serviceClient,
		FakeConsul:    agentClient,
		Workload:      workload,
	}
}

func TestConsul_ChangeTags(t *testing.T) {
	ci.Parallel(t)

	ctx := setupFake(t)
	r := require.New(t)

	r.NoError(ctx.ServiceClient.RegisterWorkload(ctx.Workload))
	r.NoError(ctx.syncOnce(syncNewOps))
	r.Equal(1, len(ctx.FakeConsul.services), "Expected 1 service to be registered with Consul")

	// Validate the alloc registration
	reg1, err := ctx.ServiceClient.AllocRegistrations(ctx.Workload.AllocInfo.AllocID)
	r.NoError(err)
	r.NotNil(reg1, "Unexpected nil alloc registration")
	r.Equal(1, reg1.NumServices())
	r.Equal(0, reg1.NumChecks())

	serviceBefore := ctx.FakeConsul.lookupService("default", "taskname-service")[0]
	r.Equal(serviceBefore.Name, ctx.Workload.Services[0].Name)
	r.Equal(serviceBefore.Tags, ctx.Workload.Services[0].Tags)

	// Update the task definition
	origWorkload := ctx.Workload.Copy()
	ctx.Workload.Services[0].Tags[0] = "new-tag"

	// Register and sync the update
	r.NoError(ctx.ServiceClient.UpdateWorkload(origWorkload, ctx.Workload))
	r.NoError(ctx.syncOnce(syncNewOps))
	r.Equal(1, len(ctx.FakeConsul.services), "Expected 1 service to be registered with Consul")

	// Validate the consul service definition changed
	serviceAfter := ctx.FakeConsul.lookupService("default", "taskname-service")[0]
	r.Equal(serviceAfter.Name, ctx.Workload.Services[0].Name)
	r.Equal(serviceAfter.Tags, ctx.Workload.Services[0].Tags)
	r.Equal("new-tag", serviceAfter.Tags[0])
}

func TestConsul_EnableTagOverride_Syncs(t *testing.T) {
	ci.Parallel(t)

	ctx := setupFake(t)
	r := require.New(t)

	// Configure our test service to set EnableTagOverride = true
	ctx.Workload.Services[0].EnableTagOverride = true

	r.NoError(ctx.ServiceClient.RegisterWorkload(ctx.Workload))
	r.NoError(ctx.syncOnce(syncNewOps))
	r.Equal(1, len(ctx.FakeConsul.services))

	// Validate the alloc registration
	reg1, err := ctx.ServiceClient.AllocRegistrations(ctx.Workload.AllocInfo.AllocID)
	r.NoError(err)
	r.NotNil(reg1)
	r.Equal(1, reg1.NumServices())
	r.Equal(0, reg1.NumChecks())

	const service = "taskname-service"

	// check things are what we expect
	consulServiceDefBefore := ctx.FakeConsul.lookupService("default", service)[0]
	r.Equal(ctx.Workload.Services[0].Name, consulServiceDefBefore.Name)
	r.Equal([]string{"tag1", "tag2"}, consulServiceDefBefore.Tags)
	r.True(consulServiceDefBefore.EnableTagOverride)

	// manually set the tags in consul
	ctx.FakeConsul.lookupService("default", service)[0].Tags = []string{"new", "tags"}

	// do a periodic sync (which will respect EnableTagOverride)
	r.NoError(ctx.syncOnce(syncPeriodic))
	r.Equal(1, len(ctx.FakeConsul.services))
	consulServiceDefAfter := ctx.FakeConsul.lookupService("default", service)[0]
	r.Equal([]string{"new", "tags"}, consulServiceDefAfter.Tags) // manually set tags should still be there

	// now do a new-ops sync (which will override EnableTagOverride)
	r.NoError(ctx.ServiceClient.RegisterWorkload(ctx.Workload))
	r.NoError(ctx.syncOnce(syncNewOps))
	r.Equal(1, len(ctx.FakeConsul.services))
	consulServiceDefUpdated := ctx.FakeConsul.lookupService("default", service)[0]
	r.Equal([]string{"tag1", "tag2"}, consulServiceDefUpdated.Tags) // jobspec tags should be set now
}

// TestConsul_ChangePorts asserts that changing the ports on a service updates
// it in Consul. Pre-0.7.1 ports were not part of the service ID and this was a
// slightly different code path than changing tags.
func TestConsul_ChangePorts(t *testing.T) {
	ci.Parallel(t)

	ctx := setupFake(t)

	ctx.Workload.Services[0].Checks = []*structs.ServiceCheck{
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

	must.NoError(t, ctx.ServiceClient.RegisterWorkload(ctx.Workload))
	must.NoError(t, ctx.syncOnce(syncNewOps))
	must.MapLen(t, 1, ctx.FakeConsul.services["default"])

	for _, v := range ctx.FakeConsul.services["default"] {
		must.Eq(t, ctx.Workload.Services[0].Name, v.Name)
		must.Eq(t, ctx.Workload.Services[0].Tags, v.Tags)
		must.Eq(t, xPort, v.Port)
	}

	must.MapLen(t, 3, ctx.FakeConsul.checks["default"], must.Sprintf("checks %#v", ctx.FakeConsul.checks))

	origTCPKey := ""
	origScriptKey := ""
	origHTTPKey := ""
	for k, v := range ctx.FakeConsul.checks["default"] {
		switch v.Name {
		case "c1":
			origTCPKey = k
			must.Eq(t, fmt.Sprintf(":%d", xPort), v.TCP)
		case "c2":
			origScriptKey = k
		case "c3":
			origHTTPKey = k
			must.Eq(t, fmt.Sprintf("http://:%d/", yPort), v.HTTP)
		default:
			t.Fatalf("unexpected check: %q", v.Name)
		}
	}

	must.StrHasPrefix(t, "_nomad-check-", origTCPKey)
	must.StrHasPrefix(t, "_nomad-check-", origScriptKey)
	must.StrHasPrefix(t, "_nomad-check-", origHTTPKey)

	// Now update the PortLabel on the Service and Check c3
	origWorkload := ctx.Workload.Copy()
	ctx.Workload.Services[0].PortLabel = "y"
	ctx.Workload.Services[0].Checks = []*structs.ServiceCheck{
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

	must.NoError(t, ctx.ServiceClient.UpdateWorkload(origWorkload, ctx.Workload))
	must.NoError(t, ctx.syncOnce(syncNewOps))
	must.MapLen(t, 1, ctx.FakeConsul.services["default"])

	for _, v := range ctx.FakeConsul.services["default"] {
		must.Eq(t, ctx.Workload.Services[0].Name, v.Name)
		must.Eq(t, ctx.Workload.Services[0].Tags, v.Tags)
		must.Eq(t, yPort, v.Port)
	}
	must.MapLen(t, 3, ctx.FakeConsul.checks["default"])

	for k, v := range ctx.FakeConsul.checks["default"] {
		switch v.Name {
		case "c1":
			// C1 is changed because the service was re-registered
			must.NotEq(t, origTCPKey, k)
			must.Eq(t, fmt.Sprintf(":%d", xPort), v.TCP)
		case "c2":
			// C2 is changed because the service was re-registered
			must.NotEq(t, origScriptKey, k)
		case "c3":
			must.NotEq(t, origHTTPKey, k)
			must.Eq(t, fmt.Sprintf("http://:%d/", yPort), v.HTTP)
		default:
			must.Unreachable(t, must.Sprintf("unknown check: %q", k))
		}
	}
}

// TestConsul_ShutdownOK tests the ok path for the shutdown logic in
// ServiceClient.
func TestConsul_ShutdownOK(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	ctx := setupFake(t)
	go ctx.ServiceClient.Run()

	// register the Nomad agent service and check
	agentServices := []*structs.Service{
		{
			Name:      "http",
			Tags:      []string{"nomad"},
			PortLabel: "localhost:2345",
			Checks: []*structs.ServiceCheck{
				{
					Name:          "nomad-tcp",
					Type:          "tcp",
					Interval:      9000 * time.Hour, // make check block
					Timeout:       10 * time.Second,
					InitialStatus: "warning",
				},
			},
		},
	}
	require.NoError(ctx.ServiceClient.RegisterAgent("client", agentServices))
	require.Eventually(ctx.ServiceClient.hasSeen, time.Second, 10*time.Millisecond)

	// assert successful registration
	require.Len(ctx.FakeConsul.services["default"], 1, "expected agent service to be registered")
	require.Len(ctx.FakeConsul.checks["default"], 1, "expected agent check to be registered")
	require.Contains(ctx.FakeConsul.services["default"], makeAgentServiceID("client", agentServices[0]))

	// Shutdown() should block until Nomad agent service/check is deregistered
	require.NoError(ctx.ServiceClient.Shutdown())
	require.Len(ctx.FakeConsul.services["default"], 0, "expected agent service to be deregistered")
	require.Len(ctx.FakeConsul.checks["default"], 0, "expected agent check to be deregistered")
}

// TestConsul_ShutdownBlocked tests the blocked past deadline path for the
// shutdown logic in ServiceClient.
func TestConsul_ShutdownBlocked(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	ctx := setupFake(t)
	// can be short because we're intentionally blocking, but needs to
	// be longer than the time we'll block Consul so we can be sure
	// we're not delayed either.
	ctx.ServiceClient.shutdownWait = time.Second
	go ctx.ServiceClient.Run()

	// register the Nomad agent service and check
	agentServices := []*structs.Service{
		{
			Name:      "http",
			Tags:      []string{"nomad"},
			PortLabel: "localhost:2345",
			Checks: []*structs.ServiceCheck{
				{
					Name:          "nomad-tcp",
					Type:          "tcp",
					Interval:      9000 * time.Hour, // make check block
					Timeout:       10 * time.Second,
					InitialStatus: "warning",
				},
			},
		},
	}
	require.NoError(ctx.ServiceClient.RegisterAgent("client", agentServices))
	require.Eventually(ctx.ServiceClient.hasSeen, time.Second, 10*time.Millisecond)
	require.Len(ctx.FakeConsul.services["default"], 1, "expected agent service to be registered")
	require.Len(ctx.FakeConsul.checks["default"], 1, "expected agent check to be registered")

	// prevent normal shutdown by blocking Consul. the shutdown should wait
	// until agent deregistration has finished
	waiter := make(chan struct{})
	result := make(chan error)
	go func() {
		ctx.FakeConsul.mu.Lock()
		close(waiter)
		result <- ctx.ServiceClient.Shutdown()
	}()

	<-waiter // wait for lock to be hit

	// Shutdown should block until all enqueued operations finish.
	preShutdown := time.Now()
	select {
	case <-time.After(200 * time.Millisecond):
		ctx.FakeConsul.mu.Unlock()
		require.NoError(<-result)
	case <-result:
		t.Fatal("should not have received result until Consul unblocked")
	}
	shutdownTime := time.Now().Sub(preShutdown).Seconds()
	require.Less(shutdownTime, time.Second.Seconds(),
		"expected shutdown to take >200ms and <1s")
	require.Greater(shutdownTime, 200*time.Millisecond.Seconds(),
		"expected shutdown to take >200ms and <1s")
	require.Len(ctx.FakeConsul.services["default"], 0,
		"expected agent service to be deregistered")
	require.Len(ctx.FakeConsul.checks["default"], 0,
		"expected agent check to be deregistered")
}

// TestConsul_DriverNetwork_AutoUse asserts that if a driver network has
// auto-use set then services should advertise it unless explicitly set to
// host. Checks should always use host.
func TestConsul_DriverNetwork_AutoUse(t *testing.T) {
	ci.Parallel(t)

	ctx := setupFake(t)

	ctx.Workload.Services = []*structs.Service{
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

	ctx.Workload.DriverNetwork = &drivers.DriverNetwork{
		PortMap: map[string]int{
			"x": 8888,
			"y": 9999,
		},
		IP:            "172.18.0.2",
		AutoAdvertise: true,
	}

	if err := ctx.ServiceClient.RegisterWorkload(ctx.Workload); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	if err := ctx.syncOnce(syncNewOps); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if n := len(ctx.FakeConsul.services["default"]); n != 3 {
		t.Fatalf("expected 2 services but found: %d", n)
	}

	for _, v := range ctx.FakeConsul.services["default"] {
		switch v.Name {
		case ctx.Workload.Services[0].Name: // x
			// Since DriverNetwork.AutoAdvertise=true, driver ports should be used
			if v.Port != ctx.Workload.DriverNetwork.PortMap["x"] {
				t.Errorf("expected service %s's port to be %d but found %d",
					v.Name, ctx.Workload.DriverNetwork.PortMap["x"], v.Port)
			}
			// The order of checks in Consul is not guaranteed to
			// be the same as their order in the Workload definition,
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
		case ctx.Workload.Services[1].Name: // y
			// Service should be container ip:port
			if v.Address != ctx.Workload.DriverNetwork.IP {
				t.Errorf("expected service %s's address to be %s but found %s",
					v.Name, ctx.Workload.DriverNetwork.IP, v.Address)
			}
			if v.Port != ctx.Workload.DriverNetwork.PortMap["y"] {
				t.Errorf("expected service %s's port to be %d but found %d",
					v.Name, ctx.Workload.DriverNetwork.PortMap["x"], v.Port)
			}
			// Check should be host ip:port
			if v.Checks[0].TCP != ":1235" { // yPort
				t.Errorf("expected service %s check's port to be %d but found %s",
					v.Name, yPort, v.Checks[0].TCP)
			}
		case ctx.Workload.Services[2].Name: // y + host mode
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
	ci.Parallel(t)

	ctx := setupFake(t)

	ctx.Workload.Services = []*structs.Service{
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

	ctx.Workload.DriverNetwork = &drivers.DriverNetwork{
		PortMap: map[string]int{
			"x": 8888,
			"y": 9999,
		},
		IP:            "172.18.0.2",
		AutoAdvertise: false,
	}

	if err := ctx.ServiceClient.RegisterWorkload(ctx.Workload); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	if err := ctx.syncOnce(syncNewOps); err != nil {
		t.Fatalf("unexpected error syncing task: %v", err)
	}

	if n := len(ctx.FakeConsul.services["default"]); n != 3 {
		t.Fatalf("expected 3 services but found: %d", n)
	}

	for _, v := range ctx.FakeConsul.services["default"] {
		switch v.Name {
		case ctx.Workload.Services[0].Name: // x + auto
			// Since DriverNetwork.AutoAdvertise=false, host ports should be used
			if v.Port != xPort {
				t.Errorf("expected service %s's port to be %d but found %d",
					v.Name, xPort, v.Port)
			}
		case ctx.Workload.Services[1].Name: // y + driver mode
			// Service should be container ip:port
			if v.Address != ctx.Workload.DriverNetwork.IP {
				t.Errorf("expected service %s's address to be %s but found %s",
					v.Name, ctx.Workload.DriverNetwork.IP, v.Address)
			}
			if v.Port != ctx.Workload.DriverNetwork.PortMap["y"] {
				t.Errorf("expected service %s's port to be %d but found %d",
					v.Name, ctx.Workload.DriverNetwork.PortMap["x"], v.Port)
			}
		case ctx.Workload.Services[2].Name: // y + host mode
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
	ci.Parallel(t)

	ctx := setupFake(t)

	ctx.Workload.Services = []*structs.Service{
		{
			Name:        "service-foo",
			PortLabel:   "x",
			AddressMode: structs.AddressModeAuto,
		},
	}

	ctx.Workload.DriverNetwork = &drivers.DriverNetwork{
		PortMap: map[string]int{
			"x": 8888,
			"y": 9999,
		},
		IP:            "172.18.0.2",
		AutoAdvertise: false,
	}

	syncAndAssertPort := func(port int) {
		if err := ctx.syncOnce(syncNewOps); err != nil {
			t.Fatalf("unexpected error syncing task: %v", err)
		}

		if n := len(ctx.FakeConsul.services["default"]); n != 1 {
			t.Fatalf("expected 1 service but found: %d", n)
		}

		for _, v := range ctx.FakeConsul.services["default"] {
			switch v.Name {
			case ctx.Workload.Services[0].Name:
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
	if err := ctx.ServiceClient.RegisterWorkload(ctx.Workload); err != nil {
		t.Fatalf("unexpected error registering task: %v", err)
	}

	syncAndAssertPort(xPort)

	// UpdateWorkload to use Host (shouldn't change anything)
	origWorkload := ctx.Workload.Copy()
	ctx.Workload.Services[0].AddressMode = structs.AddressModeHost

	if err := ctx.ServiceClient.UpdateWorkload(origWorkload, ctx.Workload); err != nil {
		t.Fatalf("unexpected error updating task: %v", err)
	}

	syncAndAssertPort(xPort)

	// UpdateWorkload to use Driver (*should* change IP and port)
	origWorkload = ctx.Workload.Copy()
	ctx.Workload.Services[0].AddressMode = structs.AddressModeDriver

	if err := ctx.ServiceClient.UpdateWorkload(origWorkload, ctx.Workload); err != nil {
		t.Fatalf("unexpected error updating task: %v", err)
	}

	syncAndAssertPort(ctx.Workload.DriverNetwork.PortMap["x"])
}

// TestConsul_CanaryTags asserts CanaryTags are used when Canary=true
func TestConsul_CanaryTags(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	ctx := setupFake(t)

	canaryTags := []string{"tag1", "canary"}
	ctx.Workload.Canary = true
	ctx.Workload.Services[0].CanaryTags = canaryTags

	require.NoError(ctx.ServiceClient.RegisterWorkload(ctx.Workload))
	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 1)
	for _, service := range ctx.FakeConsul.services["default"] {
		require.Equal(canaryTags, service.Tags)
	}

	// Disable canary and assert tags are not the canary tags
	origWorkload := ctx.Workload.Copy()
	ctx.Workload.Canary = false
	require.NoError(ctx.ServiceClient.UpdateWorkload(origWorkload, ctx.Workload))
	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 1)
	for _, service := range ctx.FakeConsul.services["default"] {
		require.NotEqual(canaryTags, service.Tags)
	}

	ctx.ServiceClient.RemoveWorkload(ctx.Workload)
	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 0)
}

// TestConsul_CanaryTags_NoTags asserts Tags are used when Canary=true and there
// are no specified canary tags
func TestConsul_CanaryTags_NoTags(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	ctx := setupFake(t)

	tags := []string{"tag1", "foo"}
	ctx.Workload.Canary = true
	ctx.Workload.Services[0].Tags = tags

	require.NoError(ctx.ServiceClient.RegisterWorkload(ctx.Workload))
	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 1)
	for _, service := range ctx.FakeConsul.services["default"] {
		require.Equal(tags, service.Tags)
	}

	// Disable canary and assert tags dont change
	origWorkload := ctx.Workload.Copy()
	ctx.Workload.Canary = false
	require.NoError(ctx.ServiceClient.UpdateWorkload(origWorkload, ctx.Workload))
	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 1)
	for _, service := range ctx.FakeConsul.services["default"] {
		require.Equal(tags, service.Tags)
	}

	ctx.ServiceClient.RemoveWorkload(ctx.Workload)
	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 0)
}

// TestConsul_CanaryMeta asserts CanaryMeta are used when Canary=true
func TestConsul_CanaryMeta(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	ctx := setupFake(t)

	canaryMeta := map[string]string{"meta1": "canary"}
	canaryMeta["external-source"] = "nomad"
	ctx.Workload.Canary = true
	ctx.Workload.Services[0].CanaryMeta = canaryMeta

	require.NoError(ctx.ServiceClient.RegisterWorkload(ctx.Workload))
	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 1)
	for _, service := range ctx.FakeConsul.services["default"] {
		require.Equal(canaryMeta, service.Meta)
	}

	// Disable canary and assert meta are not the canary meta
	origWorkload := ctx.Workload.Copy()
	ctx.Workload.Canary = false
	require.NoError(ctx.ServiceClient.UpdateWorkload(origWorkload, ctx.Workload))
	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 1)
	for _, service := range ctx.FakeConsul.services["default"] {
		require.NotEqual(canaryMeta, service.Meta)
	}

	ctx.ServiceClient.RemoveWorkload(ctx.Workload)
	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 0)
}

// TestConsul_CanaryMeta_NoMeta asserts Meta are used when Canary=true and there
// are no specified canary meta
func TestConsul_CanaryMeta_NoMeta(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	ctx := setupFake(t)

	meta := map[string]string{"meta1": "foo"}
	meta["external-source"] = "nomad"
	ctx.Workload.Canary = true
	ctx.Workload.Services[0].Meta = meta

	require.NoError(ctx.ServiceClient.RegisterWorkload(ctx.Workload))
	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 1)
	for _, service := range ctx.FakeConsul.services["default"] {
		require.Equal(meta, service.Meta)
	}

	// Disable canary and assert meta dont change
	origWorkload := ctx.Workload.Copy()
	ctx.Workload.Canary = false
	require.NoError(ctx.ServiceClient.UpdateWorkload(origWorkload, ctx.Workload))
	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 1)
	for _, service := range ctx.FakeConsul.services["default"] {
		require.Equal(meta, service.Meta)
	}

	ctx.ServiceClient.RemoveWorkload(ctx.Workload)
	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 0)
}

// TestConsul_PeriodicSync asserts that Nomad periodically reconciles with
// Consul.
func TestConsul_PeriodicSync(t *testing.T) {
	ci.Parallel(t)

	ctx := setupFake(t)
	defer ctx.ServiceClient.Shutdown()

	// Lower periodic sync interval to speed up test
	ctx.ServiceClient.periodicInterval = 1 * time.Millisecond

	// Run for 20ms and assert hits >= 5 because each sync() calls multiple
	// Consul APIs
	go ctx.ServiceClient.Run()

	select {
	case <-ctx.ServiceClient.exitCh:
		t.Fatalf("exited unexpectedly")
	case <-time.After(20 * time.Millisecond):
	}

	minHits := 5
	if hits := ctx.FakeConsul.getHits(); hits < minHits {
		t.Fatalf("expected at least %d hits but found %d", minHits, hits)
	}
}

// TestIsNomadService asserts the isNomadService helper returns true for Nomad
// task IDs and false for unknown IDs and Nomad agent IDs (see #2827).
func TestIsNomadService(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		id     string
		result bool
	}{
		{"_nomad-client-nomad-client-http", false},
		{"_nomad-server-nomad-serf", false},
		{"_nomad-task-FBBK265QN4TMT25ND4EP42TJVMYJ3HR4", true},
		{"not-nomad", false},
		{"_nomad", false},
	}

	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			actual := isNomadService(test.id)
			must.Eq(t, test.result, actual)
		})
	}
}

// TestCreateCheckReg_HTTP asserts Nomad ServiceCheck structs are properly
// converted to Consul API AgentCheckRegistrations for HTTP checks.
func TestCreateCheckReg_HTTP(t *testing.T) {
	ci.Parallel(t)

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
	namespace := ""

	expected := &api.AgentCheckRegistration{
		Namespace: namespace,
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

	actual, err := createCheckReg(serviceID, checkID, check, host, port, namespace)
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
	ci.Parallel(t)

	check := &structs.ServiceCheck{
		Name:          "name",
		Type:          "grpc",
		PortLabel:     "label",
		GRPCService:   "foo.Bar",
		GRPCUseTLS:    true,
		TLSServerName: "localhost",
		TLSSkipVerify: true,
		Timeout:       time.Second,
		Interval:      time.Minute,
	}

	serviceID := "testService"
	checkID := check.Hash(serviceID)

	expected := &api.AgentCheckRegistration{
		Namespace: "",
		ID:        checkID,
		Name:      check.Name,
		ServiceID: serviceID,
		AgentServiceCheck: api.AgentServiceCheck{
			Timeout:       "1s",
			Interval:      "1m0s",
			GRPC:          "127.0.0.1:8080/foo.Bar",
			GRPCUseTLS:    true,
			TLSServerName: "localhost",
			TLSSkipVerify: true,
		},
	}

	actual, err := createCheckReg(serviceID, checkID, check, "127.0.0.1", 8080, "default")
	must.NoError(t, err)
	must.Eq(t, expected, actual)
}

func TestConsul_ServiceName_Duplicates(t *testing.T) {
	ci.Parallel(t)
	ctx := setupFake(t)

	ctx.Workload.Services = []*structs.Service{
		{
			Name:      "best-service",
			PortLabel: "x",
			Tags:      []string{"foo"},
			Checks: []*structs.ServiceCheck{
				{
					Name:     "check-a",
					Type:     "tcp",
					Interval: time.Second,
					Timeout:  time.Second,
				},
			},
		},
		{
			Name:      "best-service",
			PortLabel: "y",
			Tags:      []string{"bar"},
			Checks: []*structs.ServiceCheck{
				{
					Name:     "check-b",
					Type:     "tcp",
					Interval: time.Second,
					Timeout:  time.Second,
				},
			},
		},
		{
			Name:      "worst-service",
			PortLabel: "y",
		},
	}

	must.NoError(t, ctx.ServiceClient.RegisterWorkload(ctx.Workload))
	must.NoError(t, ctx.syncOnce(syncNewOps))
	must.MapLen(t, 3, ctx.FakeConsul.services["default"])

	for _, s := range ctx.FakeConsul.services["default"] {
		switch {
		case s.Name == "best-service" && s.Port == xPort:
			must.SliceContainsAll(t, s.Tags, ctx.Workload.Services[0].Tags)
			must.SliceLen(t, 1, s.Checks)
		case s.Name == "best-service" && s.Port == yPort:
			must.SliceContainsAll(t, s.Tags, ctx.Workload.Services[1].Tags)
			must.SliceLen(t, 1, s.Checks)
		case s.Name == "worst-service":
			must.SliceEmpty(t, s.Checks)
		}
	}
}

// TestConsul_ServiceDeregistration_OutOfProbation asserts that during in steady
// state we remove any services we don't reconize locally
func TestConsul_ServiceDeregistration_OutProbation(t *testing.T) {
	ci.Parallel(t)

	ctx := setupFake(t)
	require := require.New(t)

	ctx.ServiceClient.deregisterProbationExpiry = time.Now().Add(-1 * time.Hour)

	remainingWorkload := testWorkload()
	remainingWorkload.Services = []*structs.Service{
		{
			Name:      "remaining-service",
			PortLabel: "x",
			Checks: []*structs.ServiceCheck{
				{
					Name:     "check",
					Type:     "tcp",
					Interval: time.Second,
					Timeout:  time.Second,
				},
			},
		},
	}
	remainingWorkloadServiceID := serviceregistration.MakeAllocServiceID(remainingWorkload.AllocInfo.AllocID,
		remainingWorkload.Name(), remainingWorkload.Services[0])

	require.NoError(ctx.ServiceClient.RegisterWorkload(remainingWorkload))
	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services, 1)
	require.Len(ctx.FakeConsul.checks, 1)

	explicitlyRemovedWorkload := testWorkload()
	explicitlyRemovedWorkload.Services = []*structs.Service{
		{
			Name:      "explicitly-removed-service",
			PortLabel: "y",
			Checks: []*structs.ServiceCheck{
				{
					Name:     "check",
					Type:     "tcp",
					Interval: time.Second,
					Timeout:  time.Second,
				},
			},
		},
	}
	explicitlyRemovedWorkloadServiceID := serviceregistration.MakeAllocServiceID(explicitlyRemovedWorkload.AllocInfo.AllocID,
		explicitlyRemovedWorkload.Name(), explicitlyRemovedWorkload.Services[0])

	require.NoError(ctx.ServiceClient.RegisterWorkload(explicitlyRemovedWorkload))

	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 2)
	require.Len(ctx.FakeConsul.checks["default"], 2)

	// we register a task through nomad API then remove it out of band
	outofbandWorkload := testWorkload()
	outofbandWorkload.Services = []*structs.Service{
		{
			Name:      "unknown-service",
			PortLabel: "x",
			Checks: []*structs.ServiceCheck{
				{
					Name:     "check",
					Type:     "tcp",
					Interval: time.Second,
					Timeout:  time.Second,
				},
			},
		},
	}
	outofbandWorkloadServiceID := serviceregistration.MakeAllocServiceID(outofbandWorkload.AllocInfo.AllocID,
		outofbandWorkload.Name(), outofbandWorkload.Services[0])

	require.NoError(ctx.ServiceClient.RegisterWorkload(outofbandWorkload))
	require.NoError(ctx.syncOnce(syncNewOps))

	require.Len(ctx.FakeConsul.services["default"], 3)

	// remove outofbandWorkload from local services so it appears unknown to client
	require.Len(ctx.ServiceClient.services, 3)
	require.Len(ctx.ServiceClient.checks, 3)

	delete(ctx.ServiceClient.services, outofbandWorkloadServiceID)
	delete(ctx.ServiceClient.checks, MakeCheckID(outofbandWorkloadServiceID, outofbandWorkload.Services[0].Checks[0]))

	require.Len(ctx.ServiceClient.services, 2)
	require.Len(ctx.ServiceClient.checks, 2)

	// Sync and ensure that explicitly removed service as well as outofbandWorkload were removed

	ctx.ServiceClient.RemoveWorkload(explicitlyRemovedWorkload)
	require.NoError(ctx.syncOnce(syncNewOps))
	require.NoError(ctx.ServiceClient.sync(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 1)
	require.Len(ctx.FakeConsul.checks["default"], 1)

	require.Contains(ctx.FakeConsul.services["default"], remainingWorkloadServiceID)
	require.NotContains(ctx.FakeConsul.services["default"], outofbandWorkloadServiceID)
	require.NotContains(ctx.FakeConsul.services["default"], explicitlyRemovedWorkloadServiceID)

	require.Contains(ctx.FakeConsul.checks["default"], MakeCheckID(remainingWorkloadServiceID, remainingWorkload.Services[0].Checks[0]))
	require.NotContains(ctx.FakeConsul.checks["default"], MakeCheckID(outofbandWorkloadServiceID, outofbandWorkload.Services[0].Checks[0]))
	require.NotContains(ctx.FakeConsul.checks["default"], MakeCheckID(explicitlyRemovedWorkloadServiceID, explicitlyRemovedWorkload.Services[0].Checks[0]))
}

// TestConsul_ServiceDeregistration_InProbation asserts that during initialization
// we only deregister services that were explicitly removed and leave unknown
// services untouched.  This adds a grace period for restoring recovered tasks
// before deregistering them
func TestConsul_ServiceDeregistration_InProbation(t *testing.T) {
	ci.Parallel(t)

	ctx := setupFake(t)
	require := require.New(t)

	ctx.ServiceClient.deregisterProbationExpiry = time.Now().Add(1 * time.Hour)

	remainingWorkload := testWorkload()
	remainingWorkload.Services = []*structs.Service{
		{
			Name:      "remaining-service",
			PortLabel: "x",
			Checks: []*structs.ServiceCheck{
				{
					Name:     "check",
					Type:     "tcp",
					Interval: time.Second,
					Timeout:  time.Second,
				},
			},
		},
	}
	remainingWorkloadServiceID := serviceregistration.MakeAllocServiceID(remainingWorkload.AllocInfo.AllocID,
		remainingWorkload.Name(), remainingWorkload.Services[0])

	require.NoError(ctx.ServiceClient.RegisterWorkload(remainingWorkload))
	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services, 1)
	require.Len(ctx.FakeConsul.checks, 1)

	explicitlyRemovedWorkload := testWorkload()
	explicitlyRemovedWorkload.Services = []*structs.Service{
		{
			Name:      "explicitly-removed-service",
			PortLabel: "y",
			Checks: []*structs.ServiceCheck{
				{
					Name:     "check",
					Type:     "tcp",
					Interval: time.Second,
					Timeout:  time.Second,
				},
			},
		},
	}
	explicitlyRemovedWorkloadServiceID := serviceregistration.MakeAllocServiceID(explicitlyRemovedWorkload.AllocInfo.AllocID,
		explicitlyRemovedWorkload.Name(), explicitlyRemovedWorkload.Services[0])

	require.NoError(ctx.ServiceClient.RegisterWorkload(explicitlyRemovedWorkload))

	require.NoError(ctx.syncOnce(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 2)
	require.Len(ctx.FakeConsul.checks["default"], 2)

	// we register a task through nomad API then remove it out of band
	outofbandWorkload := testWorkload()
	outofbandWorkload.Services = []*structs.Service{
		{
			Name:      "unknown-service",
			PortLabel: "x",
			Checks: []*structs.ServiceCheck{
				{
					Name:     "check",
					Type:     "tcp",
					Interval: time.Second,
					Timeout:  time.Second,
				},
			},
		},
	}
	outofbandWorkloadServiceID := serviceregistration.MakeAllocServiceID(outofbandWorkload.AllocInfo.AllocID,
		outofbandWorkload.Name(), outofbandWorkload.Services[0])

	require.NoError(ctx.ServiceClient.RegisterWorkload(outofbandWorkload))
	require.NoError(ctx.syncOnce(syncNewOps))

	require.Len(ctx.FakeConsul.services["default"], 3)

	// remove outofbandWorkload from local services so it appears unknown to client
	require.Len(ctx.ServiceClient.services, 3)
	require.Len(ctx.ServiceClient.checks, 3)

	delete(ctx.ServiceClient.services, outofbandWorkloadServiceID)
	delete(ctx.ServiceClient.checks, MakeCheckID(outofbandWorkloadServiceID, outofbandWorkload.Services[0].Checks[0]))

	require.Len(ctx.ServiceClient.services, 2)
	require.Len(ctx.ServiceClient.checks, 2)

	// Sync and ensure that explicitly removed service was removed, but outofbandWorkload remains

	ctx.ServiceClient.RemoveWorkload(explicitlyRemovedWorkload)
	require.NoError(ctx.syncOnce(syncNewOps))
	require.NoError(ctx.ServiceClient.sync(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 2)
	require.Len(ctx.FakeConsul.checks["default"], 2)

	require.Contains(ctx.FakeConsul.services["default"], remainingWorkloadServiceID)
	require.Contains(ctx.FakeConsul.services["default"], outofbandWorkloadServiceID)
	require.NotContains(ctx.FakeConsul.services["default"], explicitlyRemovedWorkloadServiceID)

	require.Contains(ctx.FakeConsul.checks["default"], MakeCheckID(remainingWorkloadServiceID, remainingWorkload.Services[0].Checks[0]))
	require.Contains(ctx.FakeConsul.checks["default"], MakeCheckID(outofbandWorkloadServiceID, outofbandWorkload.Services[0].Checks[0]))
	require.NotContains(ctx.FakeConsul.checks["default"], MakeCheckID(explicitlyRemovedWorkloadServiceID, explicitlyRemovedWorkload.Services[0].Checks[0]))

	// after probation, outofband services and checks are removed
	ctx.ServiceClient.deregisterProbationExpiry = time.Now().Add(-1 * time.Hour)

	require.NoError(ctx.ServiceClient.sync(syncNewOps))
	require.Len(ctx.FakeConsul.services["default"], 1)
	require.Len(ctx.FakeConsul.checks["default"], 1)

	require.Contains(ctx.FakeConsul.services["default"], remainingWorkloadServiceID)
	require.NotContains(ctx.FakeConsul.services["default"], outofbandWorkloadServiceID)
	require.NotContains(ctx.FakeConsul.services["default"], explicitlyRemovedWorkloadServiceID)

	require.Contains(ctx.FakeConsul.checks["default"], MakeCheckID(remainingWorkloadServiceID, remainingWorkload.Services[0].Checks[0]))
	require.NotContains(ctx.FakeConsul.checks["default"], MakeCheckID(outofbandWorkloadServiceID, outofbandWorkload.Services[0].Checks[0]))
	require.NotContains(ctx.FakeConsul.checks["default"], MakeCheckID(explicitlyRemovedWorkloadServiceID, explicitlyRemovedWorkload.Services[0].Checks[0]))
}
