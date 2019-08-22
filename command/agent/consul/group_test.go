package consul

import (
	"io/ioutil"
	"testing"
	"time"

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
	go serviceClient.Run()

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
				SidecarService: &structs.ConsulSidecarService{},
			},
		},
	}

	require.NoError(t, serviceClient.RegisterGroup(alloc))

	require.Eventually(t, func() bool {
		services, err := consulClient.Agent().Services()
		require.NoError(t, err)
		return len(services) == 2
	}, 3*time.Second, 100*time.Millisecond)

	services, err := consulClient.Agent().Services()
	require.NoError(t, err)
	require.Len(t, services, 2)

	serviceID := MakeTaskServiceID(alloc.ID, "group-"+alloc.TaskGroup, tg.Services[0], false)
	connectID := serviceID + "-sidecar-proxy"

	require.Contains(t, services, serviceID)
	agentService := services[serviceID]
	require.Equal(t, agentService.Service, "testconnect")
	require.Equal(t, agentService.Address, "10.0.0.1")
	require.Equal(t, agentService.Port, 9999)
	require.Nil(t, agentService.Connect)
	require.Nil(t, agentService.Proxy)

	require.Contains(t, services, connectID)
	connectService := services[connectID]
	require.Equal(t, connectService.Service, "testconnect-sidecar-proxy")
	require.Equal(t, connectService.Address, "10.0.0.1")
	require.Equal(t, connectService.Port, 9999)
	require.Nil(t, connectService.Connect)
	require.Equal(t, connectService.Proxy.DestinationServiceName, "testconnect")
	require.Equal(t, connectService.Proxy.DestinationServiceID, serviceID)
	require.Equal(t, connectService.Proxy.LocalServiceAddress, "127.0.0.1")
	require.Equal(t, connectService.Proxy.LocalServicePort, 9999)
	require.Equal(t, connectService.Proxy.Config, map[string]interface{}{
		"bind_address": "0.0.0.0",
		"bind_port":    float64(9998),
	})
}
