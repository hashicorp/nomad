package nomad

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestJobEndpointConnect_isSidecarForService(t *testing.T) {
	t.Parallel()

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

func TestJobEndpointConnect_groupConnectHook(t *testing.T) {
	t.Parallel()

	// Test that connect-proxy task is inserted for backend service
	job := mock.Job()
	job.TaskGroups[0] = &structs.TaskGroup{
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

	// Expected tasks
	tgOut := job.TaskGroups[0].Copy()
	tgOut.Tasks = []*structs.Task{
		newConnectTask(tgOut.Services[0].Name),
		newConnectTask(tgOut.Services[1].Name),
	}

	// Expect sidecar tasks to be properly canonicalized
	tgOut.Tasks[0].Canonicalize(job, tgOut)
	tgOut.Tasks[1].Canonicalize(job, tgOut)
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
	tgOut.Networks[0].Canonicalize()

	require.NoError(t, groupConnectHook(job, job.TaskGroups[0]))
	require.Exactly(t, tgOut, job.TaskGroups[0])

	// Test that hook is idempotent
	require.NoError(t, groupConnectHook(job, job.TaskGroups[0]))
	require.Exactly(t, tgOut, job.TaskGroups[0])
}

// TestJobEndpoint_ConnectInterpolation asserts that when a Connect sidecar
// proxy task is being created for a group service with an interpolated name,
// the service name is interpolated *before the task is created.
//
// See https://github.com/hashicorp/nomad/issues/6853
func TestJobEndpointConnect_ConnectInterpolation(t *testing.T) {
	t.Parallel()

	server := &Server{logger: testlog.HCLogger(t)}
	jobEndpoint := NewJobEndpoints(server)

	j := mock.ConnectJob()
	j.TaskGroups[0].Services[0].Name = "${JOB}-api"
	j, warnings, err := jobEndpoint.admissionMutators(j)
	require.NoError(t, err)
	require.Nil(t, warnings)

	require.Len(t, j.TaskGroups[0].Tasks, 2)
	require.Equal(t, "connect-proxy-my-job-api", j.TaskGroups[0].Tasks[1].Name)
}

func TestJobEndpointConnect_groupConnectSidecarValidate(t *testing.T) {
	t.Run("sidecar 0 networks", func(t *testing.T) {
		require.EqualError(t, groupConnectSidecarValidate(&structs.TaskGroup{
			Name:     "g1",
			Networks: nil,
		}), `Consul Connect sidecars require exactly 1 network, found 0 in group "g1"`)
	})

	t.Run("sidecar non bridge", func(t *testing.T) {
		require.EqualError(t, groupConnectSidecarValidate(&structs.TaskGroup{
			Name: "g2",
			Networks: structs.Networks{{
				Mode: "host",
			}},
		}), `Consul Connect sidecar requires bridge network, found "host" in group "g2"`)
	})

	t.Run("sidecar okay", func(t *testing.T) {
		require.NoError(t, groupConnectSidecarValidate(&structs.TaskGroup{
			Name: "g3",
			Networks: structs.Networks{{
				Mode: "bridge",
			}},
		}))
	})
}

func TestJobEndpointConnect_getNamedTaskForNativeService(t *testing.T) {
	t.Run("named exists", func(t *testing.T) {
		task, err := getNamedTaskForNativeService(&structs.TaskGroup{
			Name:  "g1",
			Tasks: []*structs.Task{{Name: "t1"}, {Name: "t2"}},
		}, "s1", "t2")
		require.NoError(t, err)
		require.Equal(t, "t2", task.Name)
	})

	t.Run("infer exists", func(t *testing.T) {
		task, err := getNamedTaskForNativeService(&structs.TaskGroup{
			Name:  "g1",
			Tasks: []*structs.Task{{Name: "t2"}},
		}, "s1", "")
		require.NoError(t, err)
		require.Equal(t, "t2", task.Name)
	})

	t.Run("infer ambiguous", func(t *testing.T) {
		task, err := getNamedTaskForNativeService(&structs.TaskGroup{
			Name:  "g1",
			Tasks: []*structs.Task{{Name: "t1"}, {Name: "t2"}},
		}, "s1", "")
		require.EqualError(t, err, "task for Consul Connect Native service g1->s1 is ambiguous and must be set")
		require.Nil(t, task)
	})

	t.Run("named absent", func(t *testing.T) {
		task, err := getNamedTaskForNativeService(&structs.TaskGroup{
			Name:  "g1",
			Tasks: []*structs.Task{{Name: "t1"}, {Name: "t2"}},
		}, "s1", "t3")
		require.EqualError(t, err, "task t3 named by Consul Connect Native service g1->s1 does not exist")
		require.Nil(t, task)
	})
}
