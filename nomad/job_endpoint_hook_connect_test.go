package nomad

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper"
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

	job.Meta = map[string]string{
		"backend_name": "backend",
		"admin_name":   "admin",
	}

	job.TaskGroups[0] = &structs.TaskGroup{
		Networks: structs.Networks{{
			Mode: "bridge",
		}},
		Services: []*structs.Service{{
			Name:      "${NOMAD_META_backend_name}",
			PortLabel: "8080",
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{},
			},
		}, {
			Name:      "${NOMAD_META_admin_name}",
			PortLabel: "9090",
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{},
			},
		}},
	}

	// Expected tasks
	tgExp := job.TaskGroups[0].Copy()
	tgExp.Tasks = []*structs.Task{
		newConnectTask("backend"),
		newConnectTask("admin"),
	}
	tgExp.Services[0].Name = "backend"
	tgExp.Services[1].Name = "admin"

	// Expect sidecar tasks to be in canonical form.
	tgExp.Tasks[0].Canonicalize(job, tgExp)
	tgExp.Tasks[1].Canonicalize(job, tgExp)
	tgExp.Networks[0].DynamicPorts = []structs.Port{{
		Label: fmt.Sprintf("%s-%s", structs.ConnectProxyPrefix, "backend"),
		To:    -1,
	}, {
		Label: fmt.Sprintf("%s-%s", structs.ConnectProxyPrefix, "admin"),
		To:    -1,
	}}
	tgExp.Networks[0].Canonicalize()

	require.NoError(t, groupConnectHook(job, job.TaskGroups[0]))
	require.Exactly(t, tgExp, job.TaskGroups[0])

	// Test that hook is idempotent
	require.NoError(t, groupConnectHook(job, job.TaskGroups[0]))
	require.Exactly(t, tgExp, job.TaskGroups[0])
}

func TestJobEndpointConnect_groupConnectHook_IngressGateway(t *testing.T) {
	t.Parallel()

	// Test that the connect gateway task is inserted if a gateway service exists
	// and since this is a bridge network, will rewrite the default gateway proxy
	// block with correct configuration.
	job := mock.ConnectIngressGatewayJob("bridge", false)

	job.Meta = map[string]string{
		"gateway_name": "my-gateway",
	}

	job.TaskGroups[0].Services[0].Name = "${NOMAD_META_gateway_name}"

	expTG := job.TaskGroups[0].Copy()
	expTG.Tasks = []*structs.Task{
		// inject the gateway task
		newConnectGatewayTask("my-gateway", false),
	}
	expTG.Services[0].Name = "my-gateway"
	expTG.Tasks[0].Canonicalize(job, expTG)
	expTG.Networks[0].Canonicalize()

	// rewrite the service gateway proxy configuration
	expTG.Services[0].Connect.Gateway.Proxy = gatewayProxyForBridge(expTG.Services[0].Connect.Gateway)

	require.NoError(t, groupConnectHook(job, job.TaskGroups[0]))
	require.Exactly(t, expTG, job.TaskGroups[0])

	// Test that the hook is idempotent
	require.NoError(t, groupConnectHook(job, job.TaskGroups[0]))
	require.Exactly(t, expTG, job.TaskGroups[0])
}

func TestJobEndpointConnect_groupConnectHook_IngressGateway_CustomTask(t *testing.T) {
	t.Parallel()

	// Test that the connect gateway task is inserted if a gateway service exists
	// and since this is a bridge network, will rewrite the default gateway proxy
	// block with correct configuration.
	job := mock.ConnectIngressGatewayJob("bridge", false)

	job.Meta = map[string]string{
		"gateway_name": "my-gateway",
	}

	job.TaskGroups[0].Services[0].Name = "${NOMAD_META_gateway_name}"
	job.TaskGroups[0].Services[0].Connect.SidecarTask = &structs.SidecarTask{
		Driver: "raw_exec",
		User:   "sidecars",
		Config: map[string]interface{}{
			"command": "/bin/sidecar",
			"args":    []string{"a", "b"},
		},
		Resources: &structs.Resources{
			CPU: 400,
			// Memory: inherit 128
		},
		KillSignal: "SIGHUP",
	}

	expTG := job.TaskGroups[0].Copy()
	expTG.Tasks = []*structs.Task{
		// inject merged gateway task
		{
			Name:   "connect-ingress-my-gateway",
			Kind:   structs.NewTaskKind(structs.ConnectIngressPrefix, "my-gateway"),
			Driver: "raw_exec",
			User:   "sidecars",
			Config: map[string]interface{}{
				"command": "/bin/sidecar",
				"args":    []string{"a", "b"},
			},
			Resources: &structs.Resources{
				CPU:      400,
				MemoryMB: 128,
			},
			LogConfig: &structs.LogConfig{
				MaxFiles:      2,
				MaxFileSizeMB: 2,
			},
			ShutdownDelay: 5 * time.Second,
			KillSignal:    "SIGHUP",
			Constraints: structs.Constraints{
				connectGatewayVersionConstraint(),
			},
		},
	}
	expTG.Services[0].Name = "my-gateway"
	expTG.Tasks[0].Canonicalize(job, expTG)
	expTG.Networks[0].Canonicalize()

	// rewrite the service gateway proxy configuration
	expTG.Services[0].Connect.Gateway.Proxy = gatewayProxyForBridge(expTG.Services[0].Connect.Gateway)

	require.NoError(t, groupConnectHook(job, job.TaskGroups[0]))
	require.Exactly(t, expTG, job.TaskGroups[0])

	// Test that the hook is idempotent
	require.NoError(t, groupConnectHook(job, job.TaskGroups[0]))
	require.Exactly(t, expTG, job.TaskGroups[0])
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
	t.Parallel()

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
	t.Parallel()

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

func TestJobEndpointConnect_groupConnectGatewayValidate(t *testing.T) {
	t.Parallel()

	t.Run("no group network", func(t *testing.T) {
		err := groupConnectGatewayValidate(&structs.TaskGroup{
			Name:     "g1",
			Networks: nil,
		})
		require.EqualError(t, err, `Consul Connect gateways require exactly 1 network, found 0 in group "g1"`)
	})

	t.Run("bad network mode", func(t *testing.T) {
		err := groupConnectGatewayValidate(&structs.TaskGroup{
			Name: "g1",
			Networks: structs.Networks{{
				Mode: "",
			}},
		})
		require.EqualError(t, err, `Consul Connect Gateway service requires Task Group with network mode of type "bridge" or "host"`)
	})
}

func TestJobEndpointConnect_newConnectGatewayTask_host(t *testing.T) {
	task := newConnectGatewayTask("service1", true)
	require.Equal(t, "connect-ingress-service1", task.Name)
	require.Equal(t, "connect-ingress:service1", string(task.Kind))
	require.Equal(t, ">= 1.8.0", task.Constraints[0].RTarget)
	require.Equal(t, "host", task.Config["network_mode"])
	require.Nil(t, task.Lifecycle)
}

func TestJobEndpointConnect_newConnectGatewayTask_bridge(t *testing.T) {
	task := newConnectGatewayTask("service1", false)
	require.NotContains(t, task.Config, "network_mode")
}

func TestJobEndpointConnect_hasGatewayTaskForService(t *testing.T) {
	t.Run("no gateway task", func(t *testing.T) {
		result := hasGatewayTaskForService(&structs.TaskGroup{
			Name: "group",
			Tasks: []*structs.Task{{
				Name: "task1",
				Kind: "",
			}},
		}, "my-service")
		require.False(t, result)
	})

	t.Run("has gateway task", func(t *testing.T) {
		result := hasGatewayTaskForService(&structs.TaskGroup{
			Name: "group",
			Tasks: []*structs.Task{{
				Name: "task1",
				Kind: "",
			}, {
				Name: "ingress-gateway-my-service",
				Kind: structs.NewTaskKind(structs.ConnectIngressPrefix, "my-service"),
			}},
		}, "my-service")
		require.True(t, result)
	})
}

func TestJobEndpointConnect_gatewayProxyIsDefault(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		result := gatewayProxyIsDefault(nil)
		require.True(t, result)
	})

	t.Run("unrelated fields set", func(t *testing.T) {
		result := gatewayProxyIsDefault(&structs.ConsulGatewayProxy{
			ConnectTimeout: helper.TimeToPtr(2 * time.Second),
			Config:         map[string]interface{}{"foo": 1},
		})
		require.True(t, result)
	})

	t.Run("no-bind set", func(t *testing.T) {
		result := gatewayProxyIsDefault(&structs.ConsulGatewayProxy{
			EnvoyGatewayNoDefaultBind: true,
		})
		require.False(t, result)
	})

	t.Run("bind-tagged set", func(t *testing.T) {
		result := gatewayProxyIsDefault(&structs.ConsulGatewayProxy{
			EnvoyGatewayBindTaggedAddresses: true,
		})
		require.False(t, result)
	})

	t.Run("bind-addresses set", func(t *testing.T) {
		result := gatewayProxyIsDefault(&structs.ConsulGatewayProxy{
			EnvoyGatewayBindAddresses: map[string]*structs.ConsulGatewayBindAddress{
				"listener1": {
					Address: "1.1.1.1",
					Port:    9000,
				},
			},
		})
		require.False(t, result)
	})
}

func TestJobEndpointConnect_gatewayBindAddresses(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		result := gatewayBindAddresses(nil)
		require.Nil(t, result)
	})

	t.Run("no listeners", func(t *testing.T) {
		result := gatewayBindAddresses(&structs.ConsulIngressConfigEntry{Listeners: nil})
		require.Nil(t, result)
	})

	t.Run("simple", func(t *testing.T) {
		result := gatewayBindAddresses(&structs.ConsulIngressConfigEntry{
			Listeners: []*structs.ConsulIngressListener{{
				Port:     3000,
				Protocol: "tcp",
				Services: []*structs.ConsulIngressService{{
					Name: "service1",
				}},
			}},
		})
		require.Equal(t, map[string]*structs.ConsulGatewayBindAddress{
			"service1": {
				Address: "0.0.0.0",
				Port:    3000,
			},
		}, result)
	})

	t.Run("complex", func(t *testing.T) {
		result := gatewayBindAddresses(&structs.ConsulIngressConfigEntry{
			Listeners: []*structs.ConsulIngressListener{{
				Port:     3000,
				Protocol: "tcp",
				Services: []*structs.ConsulIngressService{{
					Name: "service1",
				}, {
					Name: "service2",
				}},
			}, {
				Port:     3001,
				Protocol: "http",
				Services: []*structs.ConsulIngressService{{
					Name: "service3",
				}},
			}},
		})
		require.Equal(t, map[string]*structs.ConsulGatewayBindAddress{
			"service1": {
				Address: "0.0.0.0",
				Port:    3000,
			},
			"service2": {
				Address: "0.0.0.0",
				Port:    3000,
			},
			"service3": {
				Address: "0.0.0.0",
				Port:    3001,
			},
		}, result)
	})
}

func TestJobEndpointConnect_gatewayProxyForBridge(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		result := gatewayProxyForBridge(nil)
		require.Nil(t, result)
	})

	t.Run("nil proxy", func(t *testing.T) {
		result := gatewayProxyForBridge(&structs.ConsulGateway{
			Ingress: &structs.ConsulIngressConfigEntry{
				Listeners: []*structs.ConsulIngressListener{{
					Port:     3000,
					Protocol: "tcp",
					Services: []*structs.ConsulIngressService{{
						Name: "service1",
					}},
				}},
			},
		})
		require.Equal(t, &structs.ConsulGatewayProxy{
			EnvoyGatewayNoDefaultBind:       true,
			EnvoyGatewayBindTaggedAddresses: false,
			EnvoyGatewayBindAddresses: map[string]*structs.ConsulGatewayBindAddress{
				"service1": {
					Address: "0.0.0.0",
					Port:    3000,
				}},
		}, result)
	})

	t.Run("fill in defaults", func(t *testing.T) {
		result := gatewayProxyForBridge(&structs.ConsulGateway{
			Proxy: &structs.ConsulGatewayProxy{
				ConnectTimeout: helper.TimeToPtr(2 * time.Second),
				Config:         map[string]interface{}{"foo": 1},
			},
			Ingress: &structs.ConsulIngressConfigEntry{
				Listeners: []*structs.ConsulIngressListener{{
					Port:     3000,
					Protocol: "tcp",
					Services: []*structs.ConsulIngressService{{
						Name: "service1",
					}},
				}},
			},
		})
		require.Equal(t, &structs.ConsulGatewayProxy{
			ConnectTimeout:                  helper.TimeToPtr(2 * time.Second),
			Config:                          map[string]interface{}{"foo": 1},
			EnvoyGatewayNoDefaultBind:       true,
			EnvoyGatewayBindTaggedAddresses: false,
			EnvoyGatewayBindAddresses: map[string]*structs.ConsulGatewayBindAddress{
				"service1": {
					Address: "0.0.0.0",
					Port:    3000,
				}},
		}, result)
	})

	t.Run("leave as-is", func(t *testing.T) {
		result := gatewayProxyForBridge(&structs.ConsulGateway{
			Proxy: &structs.ConsulGatewayProxy{
				Config:                          map[string]interface{}{"foo": 1},
				EnvoyGatewayBindTaggedAddresses: true,
			},
			Ingress: &structs.ConsulIngressConfigEntry{
				Listeners: []*structs.ConsulIngressListener{{
					Port:     3000,
					Protocol: "tcp",
					Services: []*structs.ConsulIngressService{{
						Name: "service1",
					}},
				}},
			},
		})
		require.Equal(t, &structs.ConsulGatewayProxy{
			Config:                          map[string]interface{}{"foo": 1},
			EnvoyGatewayNoDefaultBind:       false,
			EnvoyGatewayBindTaggedAddresses: true,
			EnvoyGatewayBindAddresses:       nil,
		}, result)
	})
}
