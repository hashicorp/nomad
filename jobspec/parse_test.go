package jobspec

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/kr/pretty"
)

func TestParse(t *testing.T) {
	cases := []struct {
		File   string
		Result *api.Job
		Err    bool
	}{
		{
			"basic.hcl",
			&api.Job{
				ID:          helper.StringToPtr("binstore-storagelocker"),
				Name:        helper.StringToPtr("binstore-storagelocker"),
				Type:        helper.StringToPtr("batch"),
				Priority:    helper.IntToPtr(52),
				AllAtOnce:   helper.BoolToPtr(true),
				Datacenters: []string{"us2", "eu1"},
				Region:      helper.StringToPtr("fooregion"),
				Namespace:   helper.StringToPtr("foonamespace"),
				ConsulToken: helper.StringToPtr("abc"),
				VaultToken:  helper.StringToPtr("foo"),

				Meta: map[string]string{
					"foo": "bar",
				},

				Constraints: []*api.Constraint{
					{
						LTarget: "kernel.os",
						RTarget: "windows",
						Operand: "=",
					},
					{
						LTarget: "${attr.vault.version}",
						RTarget: ">= 0.6.1",
						Operand: "semver",
					},
				},

				Affinities: []*api.Affinity{
					{
						LTarget: "${meta.team}",
						RTarget: "mobile",
						Operand: "=",
						Weight:  helper.Int8ToPtr(50),
					},
				},

				Spreads: []*api.Spread{
					{
						Attribute: "${meta.rack}",
						Weight:    helper.Int8ToPtr(100),
						SpreadTarget: []*api.SpreadTarget{
							{
								Value:   "r1",
								Percent: 40,
							},
							{
								Value:   "r2",
								Percent: 60,
							},
						},
					},
				},

				Update: &api.UpdateStrategy{
					Stagger:          helper.TimeToPtr(60 * time.Second),
					MaxParallel:      helper.IntToPtr(2),
					HealthCheck:      helper.StringToPtr("manual"),
					MinHealthyTime:   helper.TimeToPtr(10 * time.Second),
					HealthyDeadline:  helper.TimeToPtr(10 * time.Minute),
					ProgressDeadline: helper.TimeToPtr(10 * time.Minute),
					AutoRevert:       helper.BoolToPtr(true),
					AutoPromote:      helper.BoolToPtr(true),
					Canary:           helper.IntToPtr(1),
				},

				TaskGroups: []*api.TaskGroup{
					{
						Name: helper.StringToPtr("outside"),

						Tasks: []*api.Task{
							{
								Name:   "outside",
								Driver: "java",
								Config: map[string]interface{}{
									"jar_path": "s3://my-cool-store/foo.jar",
								},
								Meta: map[string]string{
									"my-cool-key": "foobar",
								},
							},
						},
					},

					{
						Name:  helper.StringToPtr("binsl"),
						Count: helper.IntToPtr(5),
						Constraints: []*api.Constraint{
							{
								LTarget: "kernel.os",
								RTarget: "linux",
								Operand: "=",
							},
						},
						Volumes: map[string]*api.VolumeRequest{
							"foo": {
								Name:         "foo",
								Type:         "host",
								Source:       "/path",
								ExtraKeysHCL: nil,
							},
							"bar": {
								Name:   "bar",
								Type:   "csi",
								Source: "bar-vol",
								MountOptions: &api.CSIMountOptions{
									FSType: "ext4",
								},
								ExtraKeysHCL: nil,
							},
							"baz": {
								Name:   "baz",
								Type:   "csi",
								Source: "bar-vol",
								MountOptions: &api.CSIMountOptions{
									MountFlags: []string{
										"ro",
									},
								},
								ExtraKeysHCL: nil,
							},
						},
						Affinities: []*api.Affinity{
							{
								LTarget: "${node.datacenter}",
								RTarget: "dc2",
								Operand: "=",
								Weight:  helper.Int8ToPtr(100),
							},
						},
						Meta: map[string]string{
							"elb_mode":     "tcp",
							"elb_interval": "10",
							"elb_checks":   "3",
						},
						RestartPolicy: &api.RestartPolicy{
							Interval: helper.TimeToPtr(10 * time.Minute),
							Attempts: helper.IntToPtr(5),
							Delay:    helper.TimeToPtr(15 * time.Second),
							Mode:     helper.StringToPtr("delay"),
						},
						Spreads: []*api.Spread{
							{
								Attribute: "${node.datacenter}",
								Weight:    helper.Int8ToPtr(50),
								SpreadTarget: []*api.SpreadTarget{
									{
										Value:   "dc1",
										Percent: 50,
									},
									{
										Value:   "dc2",
										Percent: 25,
									},
									{
										Value:   "dc3",
										Percent: 25,
									},
								},
							},
						},
						StopAfterClientDisconnect: helper.TimeToPtr(120 * time.Second),
						ReschedulePolicy: &api.ReschedulePolicy{
							Interval: helper.TimeToPtr(12 * time.Hour),
							Attempts: helper.IntToPtr(5),
						},
						EphemeralDisk: &api.EphemeralDisk{
							Sticky: helper.BoolToPtr(true),
							SizeMB: helper.IntToPtr(150),
						},
						Update: &api.UpdateStrategy{
							MaxParallel:      helper.IntToPtr(3),
							HealthCheck:      helper.StringToPtr("checks"),
							MinHealthyTime:   helper.TimeToPtr(1 * time.Second),
							HealthyDeadline:  helper.TimeToPtr(1 * time.Minute),
							ProgressDeadline: helper.TimeToPtr(1 * time.Minute),
							AutoRevert:       helper.BoolToPtr(false),
							AutoPromote:      helper.BoolToPtr(false),
							Canary:           helper.IntToPtr(2),
						},
						Migrate: &api.MigrateStrategy{
							MaxParallel:     helper.IntToPtr(2),
							HealthCheck:     helper.StringToPtr("task_states"),
							MinHealthyTime:  helper.TimeToPtr(11 * time.Second),
							HealthyDeadline: helper.TimeToPtr(11 * time.Minute),
						},
						Tasks: []*api.Task{
							{
								Name:   "binstore",
								Driver: "docker",
								User:   "bob",
								Kind:   "connect-proxy:test",
								Config: map[string]interface{}{
									"image": "hashicorp/binstore",
									"labels": []map[string]interface{}{
										{
											"FOO": "bar",
										},
									},
								},
								VolumeMounts: []*api.VolumeMount{
									{
										Volume:      helper.StringToPtr("foo"),
										Destination: helper.StringToPtr("/mnt/foo"),
									},
								},
								Affinities: []*api.Affinity{
									{
										LTarget: "${meta.foo}",
										RTarget: "a,b,c",
										Operand: "set_contains",
										Weight:  helper.Int8ToPtr(25),
									},
								},
								RestartPolicy: &api.RestartPolicy{
									Attempts: helper.IntToPtr(10),
								},
								Services: []*api.Service{
									{
										Tags:       []string{"foo", "bar"},
										CanaryTags: []string{"canary", "bam"},
										Meta: map[string]string{
											"abc": "123",
										},
										CanaryMeta: map[string]string{
											"canary": "boom",
										},
										PortLabel: "http",
										Checks: []api.ServiceCheck{
											{
												Name:        "check-name",
												Type:        "tcp",
												PortLabel:   "admin",
												Interval:    10 * time.Second,
												Timeout:     2 * time.Second,
												GRPCService: "foo.Bar",
												GRPCUseTLS:  true,
												CheckRestart: &api.CheckRestart{
													Limit:          3,
													Grace:          helper.TimeToPtr(10 * time.Second),
													IgnoreWarnings: true,
												},
											},
										},
									},
								},
								Env: map[string]string{
									"HELLO": "world",
									"LOREM": "ipsum",
								},
								Resources: &api.Resources{
									CPU:      helper.IntToPtr(500),
									MemoryMB: helper.IntToPtr(128),
									Networks: []*api.NetworkResource{
										{
											MBits:         helper.IntToPtr(100),
											ReservedPorts: []api.Port{{Label: "one", Value: 1}, {Label: "two", Value: 2}, {Label: "three", Value: 3}},
											DynamicPorts:  []api.Port{{Label: "http", Value: 0}, {Label: "https", Value: 0}, {Label: "admin", Value: 0}},
										},
									},
									Devices: []*api.RequestedDevice{
										{
											Name:  "nvidia/gpu",
											Count: helper.Uint64ToPtr(10),
											Constraints: []*api.Constraint{
												{
													LTarget: "${device.attr.memory}",
													RTarget: "2GB",
													Operand: ">",
												},
											},
											Affinities: []*api.Affinity{
												{
													LTarget: "${device.model}",
													RTarget: "1080ti",
													Operand: "=",
													Weight:  helper.Int8ToPtr(50),
												},
											},
										},
										{
											Name:  "intel/gpu",
											Count: nil,
										},
									},
								},
								KillTimeout:   helper.TimeToPtr(22 * time.Second),
								ShutdownDelay: 11 * time.Second,
								LogConfig: &api.LogConfig{
									MaxFiles:      helper.IntToPtr(14),
									MaxFileSizeMB: helper.IntToPtr(101),
								},
								Artifacts: []*api.TaskArtifact{
									{
										GetterSource: helper.StringToPtr("http://foo.com/artifact"),
										GetterOptions: map[string]string{
											"checksum": "md5:b8a4f3f72ecab0510a6a31e997461c5f",
										},
									},
									{
										GetterSource: helper.StringToPtr("http://bar.com/artifact"),
										RelativeDest: helper.StringToPtr("test/foo/"),
										GetterOptions: map[string]string{
											"checksum": "md5:ff1cc0d3432dad54d607c1505fb7245c",
										},
										GetterMode: helper.StringToPtr("file"),
									},
								},
								Vault: &api.Vault{
									Namespace:  helper.StringToPtr("ns1"),
									Policies:   []string{"foo", "bar"},
									Env:        helper.BoolToPtr(true),
									ChangeMode: helper.StringToPtr(structs.VaultChangeModeRestart),
								},
								Templates: []*api.Template{
									{
										SourcePath:   helper.StringToPtr("foo"),
										DestPath:     helper.StringToPtr("foo"),
										ChangeMode:   helper.StringToPtr("foo"),
										ChangeSignal: helper.StringToPtr("foo"),
										Splay:        helper.TimeToPtr(10 * time.Second),
										Perms:        helper.StringToPtr("0644"),
										Envvars:      helper.BoolToPtr(true),
										VaultGrace:   helper.TimeToPtr(33 * time.Second),
									},
									{
										SourcePath: helper.StringToPtr("bar"),
										DestPath:   helper.StringToPtr("bar"),
										ChangeMode: helper.StringToPtr(structs.TemplateChangeModeRestart),
										Splay:      helper.TimeToPtr(5 * time.Second),
										Perms:      helper.StringToPtr("777"),
										LeftDelim:  helper.StringToPtr("--"),
										RightDelim: helper.StringToPtr("__"),
									},
								},
								Leader:     true,
								KillSignal: "",
							},
							{
								Name:   "storagelocker",
								Driver: "docker",
								User:   "",
								Lifecycle: &api.TaskLifecycle{
									Hook:    "prestart",
									Sidecar: true,
								},
								Config: map[string]interface{}{
									"image": "hashicorp/storagelocker",
								},
								Resources: &api.Resources{
									CPU:      helper.IntToPtr(500),
									MemoryMB: helper.IntToPtr(128),
								},
								Constraints: []*api.Constraint{
									{
										LTarget: "kernel.arch",
										RTarget: "amd64",
										Operand: "=",
									},
								},
								Vault: &api.Vault{
									Policies:     []string{"foo", "bar"},
									Env:          helper.BoolToPtr(false),
									ChangeMode:   helper.StringToPtr(structs.VaultChangeModeSignal),
									ChangeSignal: helper.StringToPtr("SIGUSR1"),
								},
							},
						},
					},
				},
			},
			false,
		},

		{
			"multi-network.hcl",
			nil,
			true,
		},

		{
			"multi-resource.hcl",
			nil,
			true,
		},

		{
			"multi-vault.hcl",
			nil,
			true,
		},

		{
			"default-job.hcl",
			&api.Job{
				ID:   helper.StringToPtr("foo"),
				Name: helper.StringToPtr("foo"),
			},
			false,
		},

		{
			"version-constraint.hcl",
			&api.Job{
				ID:   helper.StringToPtr("foo"),
				Name: helper.StringToPtr("foo"),
				Constraints: []*api.Constraint{
					{
						LTarget: "$attr.kernel.version",
						RTarget: "~> 3.2",
						Operand: structs.ConstraintVersion,
					},
				},
			},
			false,
		},

		{
			"regexp-constraint.hcl",
			&api.Job{
				ID:   helper.StringToPtr("foo"),
				Name: helper.StringToPtr("foo"),
				Constraints: []*api.Constraint{
					{
						LTarget: "$attr.kernel.version",
						RTarget: "[0-9.]+",
						Operand: structs.ConstraintRegex,
					},
				},
			},
			false,
		},

		{
			"set-contains-constraint.hcl",
			&api.Job{
				ID:   helper.StringToPtr("foo"),
				Name: helper.StringToPtr("foo"),
				Constraints: []*api.Constraint{
					{
						LTarget: "$meta.data",
						RTarget: "foo,bar,baz",
						Operand: structs.ConstraintSetContains,
					},
				},
			},
			false,
		},

		{
			"distinctHosts-constraint.hcl",
			&api.Job{
				ID:   helper.StringToPtr("foo"),
				Name: helper.StringToPtr("foo"),
				Constraints: []*api.Constraint{
					{
						Operand: structs.ConstraintDistinctHosts,
					},
				},
			},
			false,
		},

		{
			"distinctProperty-constraint.hcl",
			&api.Job{
				ID:   helper.StringToPtr("foo"),
				Name: helper.StringToPtr("foo"),
				Constraints: []*api.Constraint{
					{
						Operand: structs.ConstraintDistinctProperty,
						LTarget: "${meta.rack}",
					},
				},
			},
			false,
		},

		{
			"periodic-cron.hcl",
			&api.Job{
				ID:   helper.StringToPtr("foo"),
				Name: helper.StringToPtr("foo"),
				Periodic: &api.PeriodicConfig{
					SpecType:        helper.StringToPtr(api.PeriodicSpecCron),
					Spec:            helper.StringToPtr("*/5 * * *"),
					ProhibitOverlap: helper.BoolToPtr(true),
					TimeZone:        helper.StringToPtr("Europe/Minsk"),
				},
			},
			false,
		},

		{
			"specify-job.hcl",
			&api.Job{
				ID:   helper.StringToPtr("job1"),
				Name: helper.StringToPtr("My Job"),
			},
			false,
		},

		{
			"task-nested-config.hcl",
			&api.Job{
				ID:   helper.StringToPtr("foo"),
				Name: helper.StringToPtr("foo"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: helper.StringToPtr("bar"),
						Tasks: []*api.Task{
							{
								Name:   "bar",
								Driver: "docker",
								Config: map[string]interface{}{
									"image": "hashicorp/image",
									"port_map": []map[string]interface{}{
										{
											"db": 1234,
										},
									},
								},
							},
						},
					},
				},
			},
			false,
		},

		{
			"bad-artifact.hcl",
			nil,
			true,
		},

		{
			"artifacts.hcl",
			&api.Job{
				ID:   helper.StringToPtr("binstore-storagelocker"),
				Name: helper.StringToPtr("binstore-storagelocker"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: helper.StringToPtr("binsl"),
						Tasks: []*api.Task{
							{
								Name:   "binstore",
								Driver: "docker",
								Artifacts: []*api.TaskArtifact{
									{
										GetterSource:  helper.StringToPtr("http://foo.com/bar"),
										GetterOptions: map[string]string{"foo": "bar"},
										RelativeDest:  helper.StringToPtr(""),
									},
									{
										GetterSource:  helper.StringToPtr("http://foo.com/baz"),
										GetterOptions: nil,
										RelativeDest:  nil,
									},
									{
										GetterSource:  helper.StringToPtr("http://foo.com/bam"),
										GetterOptions: nil,
										RelativeDest:  helper.StringToPtr("var/foo"),
									},
								},
							},
						},
					},
				},
			},
			false,
		},
		{
			"csi-plugin.hcl",
			&api.Job{
				ID:   helper.StringToPtr("binstore-storagelocker"),
				Name: helper.StringToPtr("binstore-storagelocker"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: helper.StringToPtr("binsl"),
						Tasks: []*api.Task{
							{
								Name:   "binstore",
								Driver: "docker",
								CSIPluginConfig: &api.TaskCSIPluginConfig{
									ID:       "org.hashicorp.csi",
									Type:     api.CSIPluginTypeMonolith,
									MountDir: "/csi/test",
								},
							},
						},
					},
				},
			},
			false,
		},
		{
			"service-check-initial-status.hcl",
			&api.Job{
				ID:   helper.StringToPtr("check_initial_status"),
				Name: helper.StringToPtr("check_initial_status"),
				Type: helper.StringToPtr("service"),
				TaskGroups: []*api.TaskGroup{
					{
						Name:  helper.StringToPtr("group"),
						Count: helper.IntToPtr(1),
						Tasks: []*api.Task{
							{
								Name: "task",
								Services: []*api.Service{
									{
										Tags:      []string{"foo", "bar"},
										PortLabel: "http",
										Checks: []api.ServiceCheck{
											{
												Name:          "check-name",
												Type:          "http",
												Path:          "/",
												Interval:      10 * time.Second,
												Timeout:       2 * time.Second,
												InitialStatus: capi.HealthPassing,
												Method:        "POST",
												Header: map[string][]string{
													"Authorization": {"Basic ZWxhc3RpYzpjaGFuZ2VtZQ=="},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			false,
		},
		{
			"service-check-pass-fail.hcl",
			&api.Job{
				ID:   helper.StringToPtr("check_pass_fail"),
				Name: helper.StringToPtr("check_pass_fail"),
				Type: helper.StringToPtr("service"),
				TaskGroups: []*api.TaskGroup{{
					Name:  helper.StringToPtr("group"),
					Count: helper.IntToPtr(1),
					Tasks: []*api.Task{{
						Name: "task",
						Services: []*api.Service{{
							Name:      "service",
							PortLabel: "http",
							Checks: []api.ServiceCheck{{
								Name:                   "check-name",
								Type:                   "http",
								Path:                   "/",
								Interval:               10 * time.Second,
								Timeout:                2 * time.Second,
								InitialStatus:          capi.HealthPassing,
								Method:                 "POST",
								SuccessBeforePassing:   3,
								FailuresBeforeCritical: 4,
							}},
						}},
					}},
				}},
			},
			false,
		},
		{
			"service-check-pass-fail.hcl",
			&api.Job{
				ID:   helper.StringToPtr("check_pass_fail"),
				Name: helper.StringToPtr("check_pass_fail"),
				Type: helper.StringToPtr("service"),
				TaskGroups: []*api.TaskGroup{{
					Name:  helper.StringToPtr("group"),
					Count: helper.IntToPtr(1),
					Tasks: []*api.Task{{
						Name: "task",
						Services: []*api.Service{{
							Name:      "service",
							PortLabel: "http",
							Checks: []api.ServiceCheck{{
								Name:                   "check-name",
								Type:                   "http",
								Path:                   "/",
								Interval:               10 * time.Second,
								Timeout:                2 * time.Second,
								InitialStatus:          capi.HealthPassing,
								Method:                 "POST",
								SuccessBeforePassing:   3,
								FailuresBeforeCritical: 4,
							}},
						}},
					}},
				}},
			},
			false,
		},
		{
			"service-check-bad-header.hcl",
			nil,
			true,
		},
		{
			"service-check-bad-header-2.hcl",
			nil,
			true,
		},
		{
			// TODO This should be pushed into the API
			"vault_inheritance.hcl",
			&api.Job{
				ID:   helper.StringToPtr("example"),
				Name: helper.StringToPtr("example"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: helper.StringToPtr("cache"),
						Tasks: []*api.Task{
							{
								Name: "redis",
								Vault: &api.Vault{
									Policies:   []string{"group"},
									Env:        helper.BoolToPtr(true),
									ChangeMode: helper.StringToPtr(structs.VaultChangeModeRestart),
								},
							},
							{
								Name: "redis2",
								Vault: &api.Vault{
									Policies:   []string{"task"},
									Env:        helper.BoolToPtr(false),
									ChangeMode: helper.StringToPtr(structs.VaultChangeModeRestart),
								},
							},
						},
					},
					{
						Name: helper.StringToPtr("cache2"),
						Tasks: []*api.Task{
							{
								Name: "redis",
								Vault: &api.Vault{
									Policies:   []string{"job"},
									Env:        helper.BoolToPtr(true),
									ChangeMode: helper.StringToPtr(structs.VaultChangeModeRestart),
								},
							},
						},
					},
				},
			},
			false,
		},
		{
			"parameterized_job.hcl",
			&api.Job{
				ID:   helper.StringToPtr("parameterized_job"),
				Name: helper.StringToPtr("parameterized_job"),

				ParameterizedJob: &api.ParameterizedJobConfig{
					Payload:      "required",
					MetaRequired: []string{"foo", "bar"},
					MetaOptional: []string{"baz", "bam"},
				},

				TaskGroups: []*api.TaskGroup{
					{
						Name: helper.StringToPtr("foo"),
						Tasks: []*api.Task{
							{
								Name:   "bar",
								Driver: "docker",
								DispatchPayload: &api.DispatchPayloadConfig{
									File: "foo/bar",
								},
							},
						},
					},
				},
			},
			false,
		},
		{
			"job-with-kill-signal.hcl",
			&api.Job{
				ID:   helper.StringToPtr("foo"),
				Name: helper.StringToPtr("foo"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: helper.StringToPtr("bar"),
						Tasks: []*api.Task{
							{
								Name:       "bar",
								Driver:     "docker",
								KillSignal: "SIGQUIT",
								Config: map[string]interface{}{
									"image": "hashicorp/image",
								},
							},
						},
					},
				},
			},
			false,
		},
		{
			"service-check-driver-address.hcl",
			&api.Job{
				ID:   helper.StringToPtr("address_mode_driver"),
				Name: helper.StringToPtr("address_mode_driver"),
				Type: helper.StringToPtr("service"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: helper.StringToPtr("group"),
						Tasks: []*api.Task{
							{
								Name: "task",
								Services: []*api.Service{
									{
										Name:        "http-service",
										PortLabel:   "http",
										AddressMode: "auto",
										Checks: []api.ServiceCheck{
											{
												Name:        "http-check",
												Type:        "http",
												Path:        "/",
												PortLabel:   "http",
												AddressMode: "driver",
											},
										},
									},
									{
										Name:        "random-service",
										PortLabel:   "9000",
										AddressMode: "driver",
										Checks: []api.ServiceCheck{
											{
												Name:        "random-check",
												Type:        "tcp",
												PortLabel:   "9001",
												AddressMode: "driver",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			false,
		},
		{
			"service-check-restart.hcl",
			&api.Job{
				ID:   helper.StringToPtr("service_check_restart"),
				Name: helper.StringToPtr("service_check_restart"),
				Type: helper.StringToPtr("service"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: helper.StringToPtr("group"),
						Tasks: []*api.Task{
							{
								Name: "task",
								Services: []*api.Service{
									{
										Name: "http-service",
										CheckRestart: &api.CheckRestart{
											Limit:          3,
											Grace:          helper.TimeToPtr(10 * time.Second),
											IgnoreWarnings: true,
										},
										Checks: []api.ServiceCheck{
											{
												Name:      "random-check",
												Type:      "tcp",
												PortLabel: "9001",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			false,
		},
		{
			"service-meta.hcl",
			&api.Job{
				ID:   helper.StringToPtr("service_meta"),
				Name: helper.StringToPtr("service_meta"),
				Type: helper.StringToPtr("service"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: helper.StringToPtr("group"),
						Tasks: []*api.Task{
							{
								Name: "task",
								Services: []*api.Service{
									{
										Name: "http-service",
										Meta: map[string]string{
											"foo": "bar",
										},
									},
								},
							},
						},
					},
				},
			},
			false,
		},
		{
			"service-enable-tag-override.hcl",
			&api.Job{
				ID:   helper.StringToPtr("service_eto"),
				Name: helper.StringToPtr("service_eto"),
				Type: helper.StringToPtr("service"),
				TaskGroups: []*api.TaskGroup{{
					Name: helper.StringToPtr("group"),
					Tasks: []*api.Task{{
						Name: "task",
						Services: []*api.Service{{
							Name:              "example",
							EnableTagOverride: true,
						}},
					}},
				}},
			},
			false,
		},
		{
			"reschedule-job.hcl",
			&api.Job{
				ID:          helper.StringToPtr("foo"),
				Name:        helper.StringToPtr("foo"),
				Type:        helper.StringToPtr("batch"),
				Datacenters: []string{"dc1"},
				Reschedule: &api.ReschedulePolicy{
					Attempts:      helper.IntToPtr(15),
					Interval:      helper.TimeToPtr(30 * time.Minute),
					DelayFunction: helper.StringToPtr("constant"),
					Delay:         helper.TimeToPtr(10 * time.Second),
				},
				TaskGroups: []*api.TaskGroup{
					{
						Name:  helper.StringToPtr("bar"),
						Count: helper.IntToPtr(3),
						Tasks: []*api.Task{
							{
								Name:   "bar",
								Driver: "raw_exec",
								Config: map[string]interface{}{
									"command": "bash",
									"args":    []interface{}{"-c", "echo hi"},
								},
							},
						},
					},
				},
			},
			false,
		},
		{
			"reschedule-job-unlimited.hcl",
			&api.Job{
				ID:          helper.StringToPtr("foo"),
				Name:        helper.StringToPtr("foo"),
				Type:        helper.StringToPtr("batch"),
				Datacenters: []string{"dc1"},
				Reschedule: &api.ReschedulePolicy{
					DelayFunction: helper.StringToPtr("exponential"),
					Delay:         helper.TimeToPtr(10 * time.Second),
					MaxDelay:      helper.TimeToPtr(120 * time.Second),
					Unlimited:     helper.BoolToPtr(true),
				},
				TaskGroups: []*api.TaskGroup{
					{
						Name:  helper.StringToPtr("bar"),
						Count: helper.IntToPtr(3),
						Tasks: []*api.Task{
							{
								Name:   "bar",
								Driver: "raw_exec",
								Config: map[string]interface{}{
									"command": "bash",
									"args":    []interface{}{"-c", "echo hi"},
								},
							},
						},
					},
				},
			},
			false,
		},
		{
			"migrate-job.hcl",
			&api.Job{
				ID:          helper.StringToPtr("foo"),
				Name:        helper.StringToPtr("foo"),
				Type:        helper.StringToPtr("batch"),
				Datacenters: []string{"dc1"},
				Migrate: &api.MigrateStrategy{
					MaxParallel:     helper.IntToPtr(2),
					HealthCheck:     helper.StringToPtr("task_states"),
					MinHealthyTime:  helper.TimeToPtr(11 * time.Second),
					HealthyDeadline: helper.TimeToPtr(11 * time.Minute),
				},
				TaskGroups: []*api.TaskGroup{
					{
						Name:  helper.StringToPtr("bar"),
						Count: helper.IntToPtr(3),
						Migrate: &api.MigrateStrategy{
							MaxParallel:     helper.IntToPtr(3),
							HealthCheck:     helper.StringToPtr("checks"),
							MinHealthyTime:  helper.TimeToPtr(1 * time.Second),
							HealthyDeadline: helper.TimeToPtr(1 * time.Minute),
						},
						Tasks: []*api.Task{
							{
								Name:   "bar",
								Driver: "raw_exec",
								Config: map[string]interface{}{
									"command": "bash",
									"args":    []interface{}{"-c", "echo hi"},
								},
							},
						},
					},
				},
			},
			false,
		},
		{
			"tg-network.hcl",
			&api.Job{
				ID:          helper.StringToPtr("foo"),
				Name:        helper.StringToPtr("foo"),
				Datacenters: []string{"dc1"},
				TaskGroups: []*api.TaskGroup{
					{
						Name:          helper.StringToPtr("bar"),
						ShutdownDelay: helper.TimeToPtr(14 * time.Second),
						Count:         helper.IntToPtr(3),
						Networks: []*api.NetworkResource{
							{
								Mode: "bridge",
								ReservedPorts: []api.Port{
									{
										Label:       "http",
										Value:       80,
										To:          8080,
										HostNetwork: "public",
									},
								},
								DNS: &api.DNSConfig{
									Servers: []string{"8.8.8.8"},
									Options: []string{"ndots:2", "edns0"},
								},
							},
						},
						Services: []*api.Service{
							{
								Name:       "connect-service",
								Tags:       []string{"foo", "bar"},
								CanaryTags: []string{"canary", "bam"},
								PortLabel:  "1234",
								Connect: &api.ConsulConnect{
									SidecarService: &api.ConsulSidecarService{
										Tags: []string{"side1", "side2"},
										Proxy: &api.ConsulProxy{
											LocalServicePort: 8080,
											Upstreams: []*api.ConsulUpstream{
												{
													DestinationName: "other-service",
													LocalBindPort:   4567,
												},
											},
										},
									},
									SidecarTask: &api.SidecarTask{
										Resources: &api.Resources{
											CPU:      helper.IntToPtr(500),
											MemoryMB: helper.IntToPtr(1024),
										},
										Env: map[string]string{
											"FOO": "abc",
										},
										ShutdownDelay: helper.TimeToPtr(5 * time.Second),
									},
								},
							},
						},
						Tasks: []*api.Task{
							{
								Name:   "bar",
								Driver: "raw_exec",
								Config: map[string]interface{}{
									"command": "bash",
									"args":    []interface{}{"-c", "echo hi"},
								},
								Resources: &api.Resources{
									Networks: []*api.NetworkResource{
										{
											MBits: helper.IntToPtr(10),
										},
									},
								},
							},
						},
					},
				},
			},
			false,
		},

		{
			"tg-service-check.hcl",
			&api.Job{
				ID:   helper.StringToPtr("group_service_check_script"),
				Name: helper.StringToPtr("group_service_check_script"),
				TaskGroups: []*api.TaskGroup{
					{
						Name:  helper.StringToPtr("group"),
						Count: helper.IntToPtr(1),
						Networks: []*api.NetworkResource{
							{
								Mode: "bridge",
								ReservedPorts: []api.Port{
									{
										Label: "http",
										Value: 80,
										To:    8080,
									},
								},
							},
						},
						Services: []*api.Service{
							{
								Name:      "foo-service",
								PortLabel: "http",
								Checks: []api.ServiceCheck{
									{
										Name:          "check-name",
										Type:          "script",
										Command:       "/bin/true",
										Interval:      time.Duration(10 * time.Second),
										Timeout:       time.Duration(2 * time.Second),
										InitialStatus: "passing",
										TaskName:      "foo",
									},
								},
							},
						},
						Tasks: []*api.Task{{Name: "foo"}},
					},
				},
			},
			false,
		},
		{
			"tg-service-proxy-expose.hcl",
			&api.Job{
				ID:   helper.StringToPtr("group_service_proxy_expose"),
				Name: helper.StringToPtr("group_service_proxy_expose"),
				TaskGroups: []*api.TaskGroup{{
					Name: helper.StringToPtr("group"),
					Services: []*api.Service{{
						Name: "example",
						Connect: &api.ConsulConnect{
							SidecarService: &api.ConsulSidecarService{
								Proxy: &api.ConsulProxy{
									ExposeConfig: &api.ConsulExposeConfig{
										Path: []*api.ConsulExposePath{{
											Path:          "/health",
											Protocol:      "http",
											LocalPathPort: 2222,
											ListenerPort:  "healthcheck",
										}, {
											Path:          "/metrics",
											Protocol:      "grpc",
											LocalPathPort: 3000,
											ListenerPort:  "metrics",
										}},
									},
								},
							},
						},
					}},
				}},
			},
			false,
		},
		{
			"tg-service-connect-sidecar_task-name.hcl",
			&api.Job{
				ID:   helper.StringToPtr("sidecar_task_name"),
				Name: helper.StringToPtr("sidecar_task_name"),
				Type: helper.StringToPtr("service"),
				TaskGroups: []*api.TaskGroup{{
					Name: helper.StringToPtr("group"),
					Services: []*api.Service{{
						Name: "example",
						Connect: &api.ConsulConnect{
							Native:         false,
							SidecarService: &api.ConsulSidecarService{},
							SidecarTask: &api.SidecarTask{
								Name: "my-sidecar",
							},
						},
					}},
				}},
			},
			false,
		},
		{
			"tg-service-connect-proxy.hcl",
			&api.Job{
				ID:   helper.StringToPtr("service-connect-proxy"),
				Name: helper.StringToPtr("service-connect-proxy"),
				Type: helper.StringToPtr("service"),
				TaskGroups: []*api.TaskGroup{{
					Name: helper.StringToPtr("group"),
					Services: []*api.Service{{
						Name: "example",
						Connect: &api.ConsulConnect{
							Native: false,
							SidecarService: &api.ConsulSidecarService{
								Proxy: &api.ConsulProxy{
									LocalServiceAddress: "10.0.1.2",
									LocalServicePort:    8080,
									ExposeConfig: &api.ConsulExposeConfig{
										Path: []*api.ConsulExposePath{{
											Path:          "/metrics",
											Protocol:      "http",
											LocalPathPort: 9001,
											ListenerPort:  "metrics",
										}, {
											Path:          "/health",
											Protocol:      "http",
											LocalPathPort: 9002,
											ListenerPort:  "health",
										}},
									},
									Upstreams: []*api.ConsulUpstream{{
										DestinationName: "upstream1",
										LocalBindPort:   2001,
									}, {
										DestinationName: "upstream2",
										LocalBindPort:   2002,
									}},
									Config: map[string]interface{}{
										"foo": "bar",
									},
								},
							},
						},
					}},
				}},
			},
			false,
		},
		{
			"tg-service-connect-local-service.hcl",
			&api.Job{
				ID:   helper.StringToPtr("connect-proxy-local-service"),
				Name: helper.StringToPtr("connect-proxy-local-service"),
				Type: helper.StringToPtr("service"),
				TaskGroups: []*api.TaskGroup{{
					Name: helper.StringToPtr("group"),
					Services: []*api.Service{{
						Name: "example",
						Connect: &api.ConsulConnect{
							Native: false,
							SidecarService: &api.ConsulSidecarService{
								Proxy: &api.ConsulProxy{
									LocalServiceAddress: "10.0.1.2",
									LocalServicePort:    9876,
								},
							},
						},
					}},
				}},
			},
			false,
		},
		{
			"tg-service-check-expose.hcl",
			&api.Job{
				ID:   helper.StringToPtr("group_service_proxy_expose"),
				Name: helper.StringToPtr("group_service_proxy_expose"),
				TaskGroups: []*api.TaskGroup{{
					Name: helper.StringToPtr("group"),
					Services: []*api.Service{{
						Name: "example",
						Connect: &api.ConsulConnect{
							SidecarService: &api.ConsulSidecarService{
								Proxy: &api.ConsulProxy{},
							},
						},
						Checks: []api.ServiceCheck{{
							Name:   "example-check1",
							Expose: true,
						}, {
							Name:   "example-check2",
							Expose: false,
						}},
					}},
				}},
			},
			false,
		},
		{
			"tg-service-connect-native.hcl",
			&api.Job{
				ID:   helper.StringToPtr("connect_native_service"),
				Name: helper.StringToPtr("connect_native_service"),
				TaskGroups: []*api.TaskGroup{{
					Name: helper.StringToPtr("group"),
					Services: []*api.Service{{
						Name:     "example",
						TaskName: "task1",
						Connect: &api.ConsulConnect{
							Native: true,
						},
					}},
				}},
			},
			false,
		},
		{
			"tg-service-enable-tag-override.hcl",
			&api.Job{
				ID:   helper.StringToPtr("group_service_eto"),
				Name: helper.StringToPtr("group_service_eto"),
				TaskGroups: []*api.TaskGroup{{
					Name: helper.StringToPtr("group"),
					Services: []*api.Service{{
						Name:              "example",
						EnableTagOverride: true,
					}},
				}},
			},
			false,
		},

		{
			"tg-scaling-policy.hcl",
			&api.Job{
				ID:   helper.StringToPtr("elastic"),
				Name: helper.StringToPtr("elastic"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: helper.StringToPtr("group"),
						Scaling: &api.ScalingPolicy{
							Min: helper.Int64ToPtr(5),
							Max: helper.Int64ToPtr(100),
							Policy: map[string]interface{}{
								"foo": "bar",
								"b":   true,
								"val": 5,
								"f":   .1,
							},
							Enabled: helper.BoolToPtr(false),
						},
					},
				},
			},
			false,
		},
		{
			"tg-service-connect-gateway-ingress.hcl",
			&api.Job{
				ID:   helper.StringToPtr("connect_gateway_ingress"),
				Name: helper.StringToPtr("connect_gateway_ingress"),
				TaskGroups: []*api.TaskGroup{{
					Name: helper.StringToPtr("group"),
					Services: []*api.Service{{
						Name: "ingress-gateway-service",
						Connect: &api.ConsulConnect{
							Gateway: &api.ConsulGateway{
								Proxy: &api.ConsulGatewayProxy{
									ConnectTimeout:                  helper.TimeToPtr(3 * time.Second),
									EnvoyGatewayBindTaggedAddresses: true,
									EnvoyGatewayBindAddresses: map[string]*api.ConsulGatewayBindAddress{
										"listener1": {Address: "10.0.0.1", Port: 8888},
										"listener2": {Address: "10.0.0.2", Port: 8889},
									},
									EnvoyGatewayNoDefaultBind: true,
									Config:                    map[string]interface{}{"foo": "bar"},
								},
								Ingress: &api.ConsulIngressConfigEntry{
									TLS: &api.ConsulGatewayTLSConfig{
										Enabled: true,
									},
									Listeners: []*api.ConsulIngressListener{{
										Port:     8001,
										Protocol: "tcp",
										Services: []*api.ConsulIngressService{{
											Name: "service1",
											Hosts: []string{
												"127.0.0.1:8001",
												"[::1]:8001",
											}}, {
											Name: "service2",
											Hosts: []string{
												"10.0.0.1:8001",
											}},
										}}, {
										Port:     8080,
										Protocol: "http",
										Services: []*api.ConsulIngressService{{
											Name: "nginx",
											Hosts: []string{
												"2.2.2.2:8080",
											},
										}},
									},
									},
								},
							},
						},
					}},
				}},
			},
			false,
		},
		{
			"tg-scaling-policy-minimal.hcl",
			&api.Job{
				ID:   helper.StringToPtr("elastic"),
				Name: helper.StringToPtr("elastic"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: helper.StringToPtr("group"),
						Scaling: &api.ScalingPolicy{
							Min:     nil,
							Max:     helper.Int64ToPtr(10),
							Policy:  nil,
							Enabled: nil,
						},
					},
				},
			},
			false,
		},

		{
			"tg-scaling-policy-missing-max.hcl",
			nil,
			true,
		},

		{
			"tg-scaling-policy-multi-policy.hcl",
			nil,
			true,
		},

		{
			"multiregion.hcl",
			&api.Job{
				ID:   helper.StringToPtr("multiregion_job"),
				Name: helper.StringToPtr("multiregion_job"),
				Multiregion: &api.Multiregion{
					Strategy: &api.MultiregionStrategy{
						MaxParallel: helper.IntToPtr(1),
						OnFailure:   helper.StringToPtr("fail_all"),
					},
					Regions: []*api.MultiregionRegion{
						{
							Name:        "west",
							Count:       helper.IntToPtr(2),
							Datacenters: []string{"west-1"},
							Meta:        map[string]string{"region_code": "W"},
						},
						{
							Name:        "east",
							Count:       helper.IntToPtr(1),
							Datacenters: []string{"east-1", "east-2"},
							Meta:        map[string]string{"region_code": "E"},
						},
					},
				},
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Logf("Testing parse: %s", tc.File)

		path, err := filepath.Abs(filepath.Join("./test-fixtures", tc.File))
		if err != nil {
			t.Fatalf("file: %s\n\n%s", tc.File, err)
			continue
		}

		actual, err := ParseFile(path)
		if (err != nil) != tc.Err {
			t.Fatalf("file: %s\n\n%s", tc.File, err)
			continue
		}

		if !reflect.DeepEqual(actual, tc.Result) {
			for _, d := range pretty.Diff(actual, tc.Result) {
				t.Logf(d)
			}
			t.Fatalf("file: %s", tc.File)
		}
	}
}

func TestBadPorts(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("./test-fixtures", "bad-ports.hcl"))
	if err != nil {
		t.Fatalf("Can't get absolute path for file: %s", err)
	}

	_, err = ParseFile(path)

	if !strings.Contains(err.Error(), errPortLabel.Error()) {
		t.Fatalf("\nExpected error\n  %s\ngot\n  %v", errPortLabel, err)
	}
}

func TestOverlappingPorts(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("./test-fixtures", "overlapping-ports.hcl"))
	if err != nil {
		t.Fatalf("Can't get absolute path for file: %s", err)
	}

	_, err = ParseFile(path)

	if err == nil {
		t.Fatalf("Expected an error")
	}

	if !strings.Contains(err.Error(), "found a port label collision") {
		t.Fatalf("Expected collision error; got %v", err)
	}
}

func TestIncorrectKey(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("./test-fixtures", "basic_wrong_key.hcl"))
	if err != nil {
		t.Fatalf("Can't get absolute path for file: %s", err)
	}

	_, err = ParseFile(path)

	if err == nil {
		t.Fatalf("Expected an error")
	}

	if !strings.Contains(err.Error(), "* group: 'binsl', task: 'binstore', service (0): 'foo', check -> invalid key: nterval") {
		t.Fatalf("Expected key error; got %v", err)
	}
}
