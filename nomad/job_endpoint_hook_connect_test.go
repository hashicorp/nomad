package nomad

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func Test_isSidecarForService(t *testing.T) {
	cases := []struct {
		t *structs.Task // task
		s string        // service
		r bool          // result
	}{
		{
			&structs.Task{},
			"api",
			false,
		},
		{
			&structs.Task{
				Kind: "connect-proxy:api",
			},
			"api",
			true,
		},
		{
			&structs.Task{
				Kind: "connect-proxy:api",
			},
			"db",
			false,
		},
		{
			&structs.Task{
				Kind: "api",
			},
			"api",
			false,
		},
	}

	for _, c := range cases {
		require.Equal(t, c.r, isSidecarForService(c.t, c.s))
	}
}

func Test_groupConnectHook(t *testing.T) {
	// Test that connect-proxy task is inserted for backend service
	tgIn := &structs.TaskGroup{
		Networks: structs.Networks{
			{
				Mode: "bridge",
			},
		},
		Services: []*structs.Service{
			{
				Name:      "backend",
				PortLabel: "8080",
				Connect: &structs.ConsulConnect{
					SidecarService: &structs.ConsulSidecarService{},
				},
			},
			{
				Name:      "admin",
				PortLabel: "9090",
				Connect: &structs.ConsulConnect{
					SidecarService: &structs.ConsulSidecarService{},
				},
			},
		},
	}

	tgOut := tgIn.Copy()
	tgOut.Tasks = []*structs.Task{
		newConnectTask(tgOut.Services[0]),
		newConnectTask(tgOut.Services[1]),
	}
	tgOut.Networks[0].DynamicPorts = []structs.Port{
		{
			Label: fmt.Sprintf("%s-%s", structs.ConnectProxyPrefix, "backend"),
			To:    -1,
		},
		{
			Label: fmt.Sprintf("%s-%s", structs.ConnectProxyPrefix, "admin"),
			To:    -1,
		},
	}

	require.NoError(t, groupConnectHook(tgIn))
	require.Exactly(t, tgOut, tgIn)

	// Test that hook is idempotent
	require.NoError(t, groupConnectHook(tgIn))
	require.Exactly(t, tgOut, tgIn)
}
