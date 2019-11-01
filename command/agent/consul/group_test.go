package consul

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestConsul_Connect(t *testing.T) {
	// Create an embedded Consul server
	testconsul, err := testutil.NewTestServerConfig(func(c *testutil.TestServerConfig) {
		// If -v wasn't specified squelch consul logging
		if !testing.Verbose() {
			c.Stdout = ioutil.Discard
			c.Stderr = ioutil.Discard
		}
	})
	if err != nil {
		t.Fatalf("error starting test consul server: %v", err)
	}
	defer testconsul.Stop()

	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = testconsul.HTTPAddr
	consulClient, err := consulapi.NewClient(consulConfig)
	require.NoError(t, err)
	serviceClient := NewServiceClient(consulClient.Agent(), testlog.HCLogger(t), true)

	// Lower periodicInterval to ensure periodic syncing doesn't improperly
	// remove Connect services.
	const interval = 50 * time.Millisecond
	serviceClient.periodicInterval = interval

	// Disable deregistration probation to test syncing
	serviceClient.deregisterProbationExpiry = time.Time{}

	go serviceClient.Run()
	defer serviceClient.Shutdown()

	alloc := mock.Alloc()
	alloc.AllocatedResources.Shared.Networks = []*structs.NetworkResource{
		{
			Mode: "bridge",
			IP:   "10.0.0.1",
			DynamicPorts: []structs.Port{
				{
					Label: "connect-proxy-testconnect",
					Value: 9999,
					To:    9998,
				},
			},
		},
	}
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	tg.Services = []*structs.Service{
		{
			Name:      "testconnect",
			PortLabel: "9999",
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Proxy: &structs.ConsulProxy{
						LocalServicePort: 9000,
					},
				},
			},
		},
	}

	// required by isNomadSidecar assertion below
	serviceRegMap := map[string]*api.AgentServiceRegistration{
		MakeTaskServiceID(alloc.ID, "group-"+alloc.TaskGroup, tg.Services[0]): nil,
	}

	require.NoError(t, serviceClient.RegisterGroup(alloc))

	require.Eventually(t, func() bool {
		services, err := consulClient.Agent().Services()
		require.NoError(t, err)
		return len(services) == 2
	}, 3*time.Second, 100*time.Millisecond)

	// Test a few times to ensure Nomad doesn't improperly deregister
	// Connect services.
	for i := 10; i > 0; i-- {
		services, err := consulClient.Agent().Services()
		require.NoError(t, err)
		require.Len(t, services, 2)

		serviceID := MakeTaskServiceID(alloc.ID, "group-"+alloc.TaskGroup, tg.Services[0])
		connectID := serviceID + "-sidecar-proxy"

		require.Contains(t, services, serviceID)
		require.True(t, isNomadService(serviceID))
		require.False(t, isNomadSidecar(serviceID, serviceRegMap))
		agentService := services[serviceID]
		require.Equal(t, agentService.Service, "testconnect")
		require.Equal(t, agentService.Address, "10.0.0.1")
		require.Equal(t, agentService.Port, 9999)
		require.Nil(t, agentService.Connect)
		require.Nil(t, agentService.Proxy)

		require.Contains(t, services, connectID)
		require.True(t, isNomadService(connectID))
		require.True(t, isNomadSidecar(connectID, serviceRegMap))
		connectService := services[connectID]
		require.Equal(t, connectService.Service, "testconnect-sidecar-proxy")
		require.Equal(t, connectService.Address, "10.0.0.1")
		require.Equal(t, connectService.Port, 9999)
		require.Nil(t, connectService.Connect)
		require.Equal(t, connectService.Proxy.DestinationServiceName, "testconnect")
		require.Equal(t, connectService.Proxy.DestinationServiceID, serviceID)
		require.Equal(t, connectService.Proxy.LocalServiceAddress, "127.0.0.1")
		require.Equal(t, connectService.Proxy.LocalServicePort, 9000)
		require.Equal(t, connectService.Proxy.Config, map[string]interface{}{
			"bind_address": "0.0.0.0",
			"bind_port":    float64(9998),
		})

		time.Sleep(interval >> 2)
	}
}

// TestConsul_Update08Alloc asserts that adding group services to a previously
// 0.8 alloc works.
//
// COMPAT(0.11) Only valid for upgrades from 0.8.
func TestConsul_Update08Alloc(t *testing.T) {
	// Create an embedded Consul server
	testconsul, err := testutil.NewTestServerConfig(func(c *testutil.TestServerConfig) {
		// If -v wasn't specified squelch consul logging
		if !testing.Verbose() {
			c.Stdout = ioutil.Discard
			c.Stderr = ioutil.Discard
		}
	})
	if err != nil {
		t.Fatalf("error starting test consul server: %v", err)
	}
	defer testconsul.Stop()

	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = testconsul.HTTPAddr
	consulClient, err := consulapi.NewClient(consulConfig)
	require.NoError(t, err)
	serviceClient := NewServiceClient(consulClient.Agent(), testlog.HCLogger(t), true)

	// Lower periodicInterval to ensure periodic syncing doesn't improperly
	// remove Connect services.
	const interval = 50 * time.Millisecond
	serviceClient.periodicInterval = interval

	// Disable deregistration probation to test syncing
	serviceClient.deregisterProbationExpiry = time.Time{}

	go serviceClient.Run()
	defer serviceClient.Shutdown()

	// Create new 0.10-style alloc
	alloc := mock.Alloc()
	alloc.AllocatedResources.Shared.Networks = []*structs.NetworkResource{
		{
			Mode: "bridge",
			IP:   "10.0.0.1",
			DynamicPorts: []structs.Port{
				{
					Label: "connect-proxy-testconnect",
					Value: 9999,
					To:    9998,
				},
			},
		},
	}
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	tg.Services = []*structs.Service{
		{
			Name:      "testconnect",
			PortLabel: "9999",
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Proxy: &structs.ConsulProxy{
						LocalServicePort: 9000,
					},
				},
			},
		},
	}

	// Create old 0.8-style alloc from new alloc
	oldAlloc := alloc.Copy()
	oldAlloc.AllocatedResources = nil
	oldAlloc.Job.LookupTaskGroup(alloc.TaskGroup).Services = nil

	// Expect new services to get registered
	require.NoError(t, serviceClient.UpdateGroup(oldAlloc, alloc))

	// Assert the group and sidecar services are registered
	require.Eventually(t, func() bool {
		services, err := consulClient.Agent().Services()
		require.NoError(t, err)
		return len(services) == 2
	}, 3*time.Second, 100*time.Millisecond)
}
