// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/helper/envoy"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ConnectJob adds a Connect proxy sidecar group service to mock.Job.
//
// Note this does *not* include the Job.Register mutation that inserts the
// associated Sidecar Task (nor the hook that configures envoy as the default).
func ConnectJob() *structs.Job {
	job := Job()
	tg := job.TaskGroups[0]
	tg.Services = []*structs.Service{{
		Name:      "testconnect",
		PortLabel: "9999",
		Connect: &structs.ConsulConnect{
			SidecarService: new(structs.ConsulSidecarService),
		},
	}}
	tg.Networks = structs.Networks{{
		Mode: "bridge", // always bridge ... for now?
	}}
	return job
}

func ConnectNativeJob(mode string) *structs.Job {
	job := Job()
	tg := job.TaskGroups[0]
	tg.Networks = []*structs.NetworkResource{{
		Mode: mode,
	}}
	tg.Services = []*structs.Service{{
		Name:      "test_connect_native",
		PortLabel: "9999",
		Connect: &structs.ConsulConnect{
			Native: true,
		},
	}}
	tg.Tasks = []*structs.Task{{
		Name: "native_task",
	}}
	return job
}

// ConnectIngressGatewayJob creates a structs.Job that contains the definition
// of a Consul Ingress Gateway service. The mode is the name of the network
// mode assumed by the task group. If inject is true, a corresponding Task is
// set on the group's Tasks (i.e. what the job would look like after job mutation).
func ConnectIngressGatewayJob(mode string, inject bool) *structs.Job {
	job := Job()
	tg := job.TaskGroups[0]
	tg.Networks = []*structs.NetworkResource{{
		Mode: mode,
	}}
	tg.Services = []*structs.Service{{
		Name:      "my-ingress-service",
		PortLabel: "9999",
		Connect: &structs.ConsulConnect{
			Gateway: &structs.ConsulGateway{
				Proxy: &structs.ConsulGatewayProxy{
					ConnectTimeout:            pointer.Of(3 * time.Second),
					EnvoyGatewayBindAddresses: make(map[string]*structs.ConsulGatewayBindAddress),
				},
				Ingress: &structs.ConsulIngressConfigEntry{
					Listeners: []*structs.ConsulIngressListener{{
						Port:     2000,
						Protocol: "tcp",
						Services: []*structs.ConsulIngressService{{
							Name: "service1",
						}},
					}},
				},
			},
		},
	}}

	tg.Tasks = nil

	// some tests need to assume the gateway proxy task has already been injected
	if inject {
		tg.Tasks = []*structs.Task{{
			Name:          fmt.Sprintf("%s-%s", structs.ConnectIngressPrefix, "my-ingress-service"),
			Kind:          structs.NewTaskKind(structs.ConnectIngressPrefix, "my-ingress-service"),
			Driver:        "docker",
			Config:        make(map[string]interface{}),
			ShutdownDelay: 5 * time.Second,
			LogConfig: &structs.LogConfig{
				MaxFiles:      2,
				MaxFileSizeMB: 2,
			},
		}}
	}
	return job
}

// ConnectTerminatingGatewayJob creates a structs.Job that contains the definition
// of a Consul Terminating Gateway service. The mode is the name of the network mode
// assumed by the task group. If inject is true, a corresponding task is set on the
// group's Tasks (i.e. what the job would look like after mutation).
func ConnectTerminatingGatewayJob(mode string, inject bool) *structs.Job {
	job := Job()
	tg := job.TaskGroups[0]
	tg.Networks = []*structs.NetworkResource{{
		Mode: mode,
	}}
	tg.Services = []*structs.Service{{
		Name:      "my-terminating-service",
		PortLabel: "9999",
		Connect: &structs.ConsulConnect{
			Gateway: &structs.ConsulGateway{
				Proxy: &structs.ConsulGatewayProxy{
					ConnectTimeout:            pointer.Of(3 * time.Second),
					EnvoyGatewayBindAddresses: make(map[string]*structs.ConsulGatewayBindAddress),
				},
				Terminating: &structs.ConsulTerminatingConfigEntry{
					Services: []*structs.ConsulLinkedService{{
						Name:     "service1",
						CAFile:   "/ssl/ca_file",
						CertFile: "/ssl/cert_file",
						KeyFile:  "/ssl/key_file",
						SNI:      "sni-name",
					}},
				},
			},
		},
	}}

	tg.Tasks = nil

	// some tests need to assume the gateway proxy task has already been injected
	if inject {
		tg.Tasks = []*structs.Task{{
			Name:          fmt.Sprintf("%s-%s", structs.ConnectTerminatingPrefix, "my-terminating-service"),
			Kind:          structs.NewTaskKind(structs.ConnectTerminatingPrefix, "my-terminating-service"),
			Driver:        "docker",
			Config:        make(map[string]interface{}),
			ShutdownDelay: 5 * time.Second,
			LogConfig: &structs.LogConfig{
				MaxFiles:      2,
				MaxFileSizeMB: 2,
			},
		}}
	}
	return job
}

// ConnectMeshGatewayJob creates a structs.Job that contains the definition of a
// Consul Mesh Gateway service. The mode is the name of the network mode assumed
// by the task group. If inject is true, a corresponding task is set on the group's
// Tasks (i.e. what the job would look like after job mutation).
func ConnectMeshGatewayJob(mode string, inject bool) *structs.Job {
	job := Job()
	tg := job.TaskGroups[0]
	tg.Networks = []*structs.NetworkResource{{
		Mode: mode,
	}}
	tg.Services = []*structs.Service{{
		Name:      "my-mesh-service",
		PortLabel: "public_port",
		Connect: &structs.ConsulConnect{
			Gateway: &structs.ConsulGateway{
				Proxy: &structs.ConsulGatewayProxy{
					ConnectTimeout:            pointer.Of(3 * time.Second),
					EnvoyGatewayBindAddresses: make(map[string]*structs.ConsulGatewayBindAddress),
				},
				Mesh: &structs.ConsulMeshConfigEntry{
					// nothing to configure
				},
			},
		},
	}}

	tg.Tasks = nil

	// some tests need to assume the gateway task has already been injected
	if inject {
		tg.Tasks = []*structs.Task{{
			Name:          fmt.Sprintf("%s-%s", structs.ConnectMeshPrefix, "my-mesh-service"),
			Kind:          structs.NewTaskKind(structs.ConnectMeshPrefix, "my-mesh-service"),
			Driver:        "docker",
			Config:        make(map[string]interface{}),
			ShutdownDelay: 5 * time.Second,
			LogConfig: &structs.LogConfig{
				MaxFiles:      2,
				MaxFileSizeMB: 2,
			},
		}}
	}
	return job
}

func BatchConnectJob() *structs.Job {
	job := &structs.Job{
		Region:      "global",
		ID:          fmt.Sprintf("mock-connect-batch-job%s", uuid.Generate()),
		Name:        "mock-connect-batch-job",
		Namespace:   structs.DefaultNamespace,
		Type:        structs.JobTypeBatch,
		Priority:    50,
		AllAtOnce:   false,
		Datacenters: []string{"dc1"},
		TaskGroups: []*structs.TaskGroup{{
			Name:          "mock-connect-batch-job",
			Count:         1,
			EphemeralDisk: &structs.EphemeralDisk{SizeMB: 150},
			Networks: []*structs.NetworkResource{{
				Mode: "bridge",
			}},
			Tasks: []*structs.Task{{
				Name:   "connect-proxy-testconnect",
				Kind:   "connect-proxy:testconnect",
				Driver: "mock_driver",
				Config: map[string]interface{}{
					"run_for": "500ms",
				},
				LogConfig: structs.DefaultLogConfig(),
				Resources: &structs.Resources{
					CPU:      500,
					MemoryMB: 256,
					Networks: []*structs.NetworkResource{{
						MBits:        50,
						DynamicPorts: []structs.Port{{Label: "port1"}},
					}},
				},
			}},
			Services: []*structs.Service{{
				Name: "testconnect",
			}},
		}},
		Meta:           map[string]string{"owner": "shoenig"},
		Status:         structs.JobStatusPending,
		Version:        0,
		CreateIndex:    42,
		ModifyIndex:    99,
		JobModifyIndex: 99,
	}
	job.Canonicalize()
	return job
}

func ConnectSidecarTask() *structs.Task {
	return &structs.Task{
		Name:   "mysidecar-sidecar-task",
		Driver: "docker",
		User:   "nobody",
		Config: map[string]interface{}{
			"image": envoy.SidecarConfigVar,
		},
		Env: nil,
		Resources: &structs.Resources{
			CPU:      150,
			MemoryMB: 350,
		},
		Kind: structs.NewTaskKind(structs.ConnectProxyPrefix, "mysidecar"),
	}
}

// ConnectAlloc adds a Connect proxy sidecar group service to mock.Alloc.
func ConnectAlloc() *structs.Allocation {
	alloc := Alloc()
	alloc.Job = ConnectJob()
	alloc.AllocatedResources.Shared.Networks = []*structs.NetworkResource{
		{
			Mode: "bridge",
			IP:   "10.0.0.1",
			DynamicPorts: []structs.Port{
				{
					Label: "connect-proxy-testconnect",
					Value: 9999,
					To:    9999,
				},
			},
		},
	}
	return alloc
}

// ConnectNativeAlloc creates an alloc with a connect native task.
func ConnectNativeAlloc(mode string) *structs.Allocation {
	alloc := Alloc()
	alloc.Job = ConnectNativeJob(mode)
	alloc.AllocatedResources.Shared.Networks = []*structs.NetworkResource{{
		Mode: mode,
		IP:   "10.0.0.1",
	}}
	return alloc
}

func ConnectIngressGatewayAlloc(mode string) *structs.Allocation {
	alloc := Alloc()
	alloc.Job = ConnectIngressGatewayJob(mode, true)
	alloc.AllocatedResources.Shared.Networks = []*structs.NetworkResource{{
		Mode: mode,
		IP:   "10.0.0.1",
	}}
	return alloc
}

// BatchConnectAlloc is useful for testing task runner things.
func BatchConnectAlloc() *structs.Allocation {
	alloc := &structs.Allocation{
		ID:        uuid.Generate(),
		EvalID:    uuid.Generate(),
		NodeID:    "12345678-abcd-efab-cdef-123456789abc",
		Namespace: structs.DefaultNamespace,
		TaskGroup: "mock-connect-batch-job",
		TaskResources: map[string]*structs.Resources{
			"connect-proxy-testconnect": {
				CPU:      500,
				MemoryMB: 256,
			},
		},

		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"connect-proxy-testconnect": {
					Cpu:    structs.AllocatedCpuResources{CpuShares: 500},
					Memory: structs.AllocatedMemoryResources{MemoryMB: 256},
				},
			},
			Shared: structs.AllocatedSharedResources{
				Networks: []*structs.NetworkResource{{
					Mode: "bridge",
					IP:   "10.0.0.1",
					DynamicPorts: []structs.Port{{
						Label: "connect-proxy-testconnect",
						Value: 9999,
						To:    9999,
					}},
				}},
				DiskMB: 0,
			},
		},
		Job:           BatchConnectJob(),
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
	}
	alloc.JobID = alloc.Job.ID
	return alloc
}

func BatchAlloc() *structs.Allocation {
	alloc := &structs.Allocation{
		ID:        uuid.Generate(),
		EvalID:    uuid.Generate(),
		NodeID:    "12345678-abcd-efab-cdef-123456789abc",
		Namespace: structs.DefaultNamespace,
		TaskGroup: "web",

		// TODO Remove once clientv2 gets merged
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 256,
			DiskMB:   150,
			Networks: []*structs.NetworkResource{
				{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []structs.Port{{Label: "admin", Value: 5000}},
					MBits:         50,
					DynamicPorts:  []structs.Port{{Label: "http"}},
				},
			},
		},
		TaskResources: map[string]*structs.Resources{
			"web": {
				CPU:      500,
				MemoryMB: 256,
				Networks: []*structs.NetworkResource{
					{
						Device:        "eth0",
						IP:            "192.168.0.100",
						ReservedPorts: []structs.Port{{Label: "admin", Value: 5000}},
						MBits:         50,
						DynamicPorts:  []structs.Port{{Label: "http", Value: 9876}},
					},
				},
			},
		},
		SharedResources: &structs.Resources{
			DiskMB: 150,
		},

		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 500,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
					Networks: []*structs.NetworkResource{
						{
							Device:        "eth0",
							IP:            "192.168.0.100",
							ReservedPorts: []structs.Port{{Label: "admin", Value: 5000}},
							MBits:         50,
							DynamicPorts:  []structs.Port{{Label: "http", Value: 9876}},
						},
					},
				},
			},
			Shared: structs.AllocatedSharedResources{
				DiskMB: 150,
			},
		},
		Job:           BatchJob(),
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
	}
	alloc.JobID = alloc.Job.ID
	return alloc
}
