package consul

import (
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/sdk"
	"github.com/stretchr/testify/require"
)

func TestConsul_Connect(t *testing.T) {
	ci.Parallel(t)

	consul, ready, stop := sdk.NewConsul(t, nil)
	t.Cleanup(stop)

	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = consul.HTTP()
	consulClient, err := consulapi.NewClient(consulConfig)
	require.NoError(t, err)

	namespacesClient := NewNamespacesClient(consulClient.Namespaces(), consulClient.Agent())
	serviceClient := NewServiceClient(consulClient.Agent(), namespacesClient, testlog.HCLogger(t), true)

	// Lower periodicInterval to ensure periodic syncing doesn't improperly
	// remove Connect services.
	const interval = 50 * time.Millisecond
	serviceClient.periodicInterval = interval

	// Disable de-registration probation to test syncing
	serviceClient.deregisterProbationExpiry = time.Time{}

	go serviceClient.Run()
	t.Cleanup(func() {
		_ = serviceClient.Shutdown()
	})

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
			Meta: map[string]string{
				"alloc_id": "${NOMAD_ALLOC_ID}",
			},
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Proxy: &structs.ConsulProxy{
						LocalServicePort: 9000,
					},
				},
			},
		},
	}

	// wait for consul agent
	ready()

	require.NoError(t, serviceClient.RegisterWorkload(BuildAllocServices(mock.Node(), alloc, NoopRestarter())))

	require.Eventually(t, func() bool {
		services, err := consulClient.Agent().Services()
		require.NoError(t, err)
		return len(services) == 2
	}, 3*time.Second, 100*time.Millisecond)

	// Test a few times to ensure Nomad doesn't improperly de-register
	// Connect services.
	for i := 10; i > 0; i-- {
		services, err := consulClient.Agent().Services()
		require.NoError(t, err)
		require.Len(t, services, 2)

		serviceID := serviceregistration.MakeAllocServiceID(alloc.ID, "group-"+alloc.TaskGroup, tg.Services[0])
		connectID := serviceID + "-sidecar-proxy"

		require.Contains(t, services, serviceID)
		require.True(t, isNomadService(serviceID))
		require.False(t, maybeConnectSidecar(serviceID))
		agentService := services[serviceID]
		require.Equal(t, agentService.Service, "testconnect")
		require.Equal(t, agentService.Address, "10.0.0.1")
		require.Equal(t, agentService.Port, 9999)
		require.Nil(t, agentService.Connect)
		require.Nil(t, agentService.Proxy)

		require.Contains(t, services, connectID)
		require.True(t, isNomadService(connectID))
		require.True(t, maybeConnectSidecar(connectID))
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
			"bind_address":     "0.0.0.0",
			"bind_port":        float64(9998),
			"envoy_stats_tags": []interface{}{"nomad.alloc_id=" + alloc.ID},
		})
		require.Equal(t, alloc.ID, agentService.Meta["alloc_id"])

		time.Sleep(interval >> 2)
	}
}
