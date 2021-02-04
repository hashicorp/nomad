package jobspec

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/api"
	"github.com/kr/pretty"
)

// consts copied from nomad/structs package to keep jobspec isolated from rest of nomad
const (
	// vaultChangeModeRestart restarts the task when a new token is retrieved.
	vaultChangeModeRestart = "restart"

	// vaultChangeModeSignal signals the task when a new token is retrieved.
	vaultChangeModeSignal = "signal"

	// templateChangeModeRestart marks that the task should be restarted if the
	templateChangeModeRestart = "restart"
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
				ID:          stringToPtr("binstore-storagelocker"),
				Name:        stringToPtr("binstore-storagelocker"),
				Type:        stringToPtr("batch"),
				Priority:    intToPtr(52),
				AllAtOnce:   boolToPtr(true),
				Datacenters: []string{"us2", "eu1"},
				Region:      stringToPtr("fooregion"),
				Namespace:   stringToPtr("foonamespace"),
				ConsulToken: stringToPtr("abc"),
				VaultToken:  stringToPtr("foo"),

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
						Weight:  int8ToPtr(50),
					},
				},

				Spreads: []*api.Spread{
					{
						Attribute: "${meta.rack}",
						Weight:    int8ToPtr(100),
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
					Stagger:          timeToPtr(60 * time.Second),
					MaxParallel:      intToPtr(2),
					HealthCheck:      stringToPtr("manual"),
					MinHealthyTime:   timeToPtr(10 * time.Second),
					HealthyDeadline:  timeToPtr(10 * time.Minute),
					ProgressDeadline: timeToPtr(10 * time.Minute),
					AutoRevert:       boolToPtr(true),
					AutoPromote:      boolToPtr(true),
					Canary:           intToPtr(1),
				},

				TaskGroups: []*api.TaskGroup{
					{
						Name: stringToPtr("outside"),

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
						Name:  stringToPtr("binsl"),
						Count: intToPtr(5),
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
								Name:     "bar",
								Type:     "csi",
								Source:   "bar-vol",
								ReadOnly: true,
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
								Weight:  int8ToPtr(100),
							},
						},
						Meta: map[string]string{
							"elb_mode":     "tcp",
							"elb_interval": "10",
							"elb_checks":   "3",
						},
						RestartPolicy: &api.RestartPolicy{
							Interval: timeToPtr(10 * time.Minute),
							Attempts: intToPtr(5),
							Delay:    timeToPtr(15 * time.Second),
							Mode:     stringToPtr("delay"),
						},
						Spreads: []*api.Spread{
							{
								Attribute: "${node.datacenter}",
								Weight:    int8ToPtr(50),
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
						StopAfterClientDisconnect: timeToPtr(120 * time.Second),
						ReschedulePolicy: &api.ReschedulePolicy{
							Interval: timeToPtr(12 * time.Hour),
							Attempts: intToPtr(5),
						},
						EphemeralDisk: &api.EphemeralDisk{
							Sticky: boolToPtr(true),
							SizeMB: intToPtr(150),
						},
						Update: &api.UpdateStrategy{
							MaxParallel:      intToPtr(3),
							HealthCheck:      stringToPtr("checks"),
							MinHealthyTime:   timeToPtr(1 * time.Second),
							HealthyDeadline:  timeToPtr(1 * time.Minute),
							ProgressDeadline: timeToPtr(1 * time.Minute),
							AutoRevert:       boolToPtr(false),
							AutoPromote:      boolToPtr(false),
							Canary:           intToPtr(2),
						},
						Migrate: &api.MigrateStrategy{
							MaxParallel:     intToPtr(2),
							HealthCheck:     stringToPtr("task_states"),
							MinHealthyTime:  timeToPtr(11 * time.Second),
							HealthyDeadline: timeToPtr(11 * time.Minute),
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
										Volume:      stringToPtr("foo"),
										Destination: stringToPtr("/mnt/foo"),
									},
								},
								Affinities: []*api.Affinity{
									{
										LTarget: "${meta.foo}",
										RTarget: "a,b,c",
										Operand: "set_contains",
										Weight:  int8ToPtr(25),
									},
								},
								RestartPolicy: &api.RestartPolicy{
									Attempts: intToPtr(10),
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
													Grace:          timeToPtr(10 * time.Second),
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
									CPU:      intToPtr(500),
									MemoryMB: intToPtr(128),
									Networks: []*api.NetworkResource{
										{
											MBits:         intToPtr(100),
											ReservedPorts: []api.Port{{Label: "one", Value: 1}, {Label: "two", Value: 2}, {Label: "three", Value: 3}},
											DynamicPorts:  []api.Port{{Label: "http", Value: 0}, {Label: "https", Value: 0}, {Label: "admin", Value: 0}},
										},
									},
									Devices: []*api.RequestedDevice{
										{
											Name:  "nvidia/gpu",
											Count: uint64ToPtr(10),
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
													Weight:  int8ToPtr(50),
												},
											},
										},
										{
											Name:  "intel/gpu",
											Count: nil,
										},
									},
								},
								KillTimeout:   timeToPtr(22 * time.Second),
								ShutdownDelay: 11 * time.Second,
								LogConfig: &api.LogConfig{
									MaxFiles:      intToPtr(14),
									MaxFileSizeMB: intToPtr(101),
								},
								Artifacts: []*api.TaskArtifact{
									{
										GetterSource: stringToPtr("http://foo.com/artifact"),
										GetterOptions: map[string]string{
											"checksum": "md5:b8a4f3f72ecab0510a6a31e997461c5f",
										},
									},
									{
										GetterSource: stringToPtr("http://bar.com/artifact"),
										RelativeDest: stringToPtr("test/foo/"),
										GetterOptions: map[string]string{
											"checksum": "md5:ff1cc0d3432dad54d607c1505fb7245c",
										},
										GetterMode: stringToPtr("file"),
									},
								},
								Vault: &api.Vault{
									Namespace:  stringToPtr("ns1"),
									Policies:   []string{"foo", "bar"},
									Env:        boolToPtr(true),
									ChangeMode: stringToPtr(vaultChangeModeRestart),
								},
								Templates: []*api.Template{
									{
										SourcePath:   stringToPtr("foo"),
										DestPath:     stringToPtr("foo"),
										ChangeMode:   stringToPtr("foo"),
										ChangeSignal: stringToPtr("foo"),
										Splay:        timeToPtr(10 * time.Second),
										Perms:        stringToPtr("0644"),
										Envvars:      boolToPtr(true),
										VaultGrace:   timeToPtr(33 * time.Second),
									},
									{
										SourcePath: stringToPtr("bar"),
										DestPath:   stringToPtr("bar"),
										ChangeMode: stringToPtr(templateChangeModeRestart),
										Splay:      timeToPtr(5 * time.Second),
										Perms:      stringToPtr("777"),
										LeftDelim:  stringToPtr("--"),
										RightDelim: stringToPtr("__"),
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
									CPU:      intToPtr(500),
									MemoryMB: intToPtr(128),
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
									Env:          boolToPtr(false),
									ChangeMode:   stringToPtr(vaultChangeModeSignal),
									ChangeSignal: stringToPtr("SIGUSR1"),
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
				ID:   stringToPtr("foo"),
				Name: stringToPtr("foo"),
			},
			false,
		},

		{
			"version-constraint.hcl",
			&api.Job{
				ID:   stringToPtr("foo"),
				Name: stringToPtr("foo"),
				Constraints: []*api.Constraint{
					{
						LTarget: "$attr.kernel.version",
						RTarget: "~> 3.2",
						Operand: api.ConstraintVersion,
					},
				},
			},
			false,
		},

		{
			"regexp-constraint.hcl",
			&api.Job{
				ID:   stringToPtr("foo"),
				Name: stringToPtr("foo"),
				Constraints: []*api.Constraint{
					{
						LTarget: "$attr.kernel.version",
						RTarget: "[0-9.]+",
						Operand: api.ConstraintRegex,
					},
				},
			},
			false,
		},

		{
			"set-contains-constraint.hcl",
			&api.Job{
				ID:   stringToPtr("foo"),
				Name: stringToPtr("foo"),
				Constraints: []*api.Constraint{
					{
						LTarget: "$meta.data",
						RTarget: "foo,bar,baz",
						Operand: api.ConstraintSetContains,
					},
				},
			},
			false,
		},

		{
			"distinctHosts-constraint.hcl",
			&api.Job{
				ID:   stringToPtr("foo"),
				Name: stringToPtr("foo"),
				Constraints: []*api.Constraint{
					{
						Operand: api.ConstraintDistinctHosts,
					},
				},
			},
			false,
		},

		{
			"distinctProperty-constraint.hcl",
			&api.Job{
				ID:   stringToPtr("foo"),
				Name: stringToPtr("foo"),
				Constraints: []*api.Constraint{
					{
						Operand: api.ConstraintDistinctProperty,
						LTarget: "${meta.rack}",
					},
				},
			},
			false,
		},

		{
			"periodic-cron.hcl",
			&api.Job{
				ID:   stringToPtr("foo"),
				Name: stringToPtr("foo"),
				Periodic: &api.PeriodicConfig{
					SpecType:        stringToPtr(api.PeriodicSpecCron),
					Spec:            stringToPtr("*/5 * * *"),
					ProhibitOverlap: boolToPtr(true),
					TimeZone:        stringToPtr("Europe/Minsk"),
				},
			},
			false,
		},

		{
			"specify-job.hcl",
			&api.Job{
				ID:   stringToPtr("job1"),
				Name: stringToPtr("My Job"),
			},
			false,
		},

		{
			"task-nested-config.hcl",
			&api.Job{
				ID:   stringToPtr("foo"),
				Name: stringToPtr("foo"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: stringToPtr("bar"),
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
				ID:   stringToPtr("binstore-storagelocker"),
				Name: stringToPtr("binstore-storagelocker"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: stringToPtr("binsl"),
						Tasks: []*api.Task{
							{
								Name:   "binstore",
								Driver: "docker",
								Artifacts: []*api.TaskArtifact{
									{
										GetterSource:  stringToPtr("http://foo.com/bar"),
										GetterOptions: map[string]string{"foo": "bar"},
										RelativeDest:  stringToPtr(""),
									},
									{
										GetterSource:  stringToPtr("http://foo.com/baz"),
										GetterOptions: nil,
										RelativeDest:  nil,
									},
									{
										GetterSource:  stringToPtr("http://foo.com/bam"),
										GetterOptions: nil,
										RelativeDest:  stringToPtr("var/foo"),
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
				ID:   stringToPtr("binstore-storagelocker"),
				Name: stringToPtr("binstore-storagelocker"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: stringToPtr("binsl"),
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
				ID:   stringToPtr("check_initial_status"),
				Name: stringToPtr("check_initial_status"),
				Type: stringToPtr("service"),
				TaskGroups: []*api.TaskGroup{
					{
						Name:  stringToPtr("group"),
						Count: intToPtr(1),
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
				ID:   stringToPtr("check_pass_fail"),
				Name: stringToPtr("check_pass_fail"),
				Type: stringToPtr("service"),
				TaskGroups: []*api.TaskGroup{{
					Name:  stringToPtr("group"),
					Count: intToPtr(1),
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
				ID:   stringToPtr("check_pass_fail"),
				Name: stringToPtr("check_pass_fail"),
				Type: stringToPtr("service"),
				TaskGroups: []*api.TaskGroup{{
					Name:  stringToPtr("group"),
					Count: intToPtr(1),
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
				ID:   stringToPtr("example"),
				Name: stringToPtr("example"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: stringToPtr("cache"),
						Tasks: []*api.Task{
							{
								Name: "redis",
								Vault: &api.Vault{
									Policies:   []string{"group"},
									Env:        boolToPtr(true),
									ChangeMode: stringToPtr(vaultChangeModeRestart),
								},
							},
							{
								Name: "redis2",
								Vault: &api.Vault{
									Policies:   []string{"task"},
									Env:        boolToPtr(false),
									ChangeMode: stringToPtr(vaultChangeModeRestart),
								},
							},
						},
					},
					{
						Name: stringToPtr("cache2"),
						Tasks: []*api.Task{
							{
								Name: "redis",
								Vault: &api.Vault{
									Policies:   []string{"job"},
									Env:        boolToPtr(true),
									ChangeMode: stringToPtr(vaultChangeModeRestart),
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
				ID:   stringToPtr("parameterized_job"),
				Name: stringToPtr("parameterized_job"),

				ParameterizedJob: &api.ParameterizedJobConfig{
					Payload:      "required",
					MetaRequired: []string{"foo", "bar"},
					MetaOptional: []string{"baz", "bam"},
				},

				TaskGroups: []*api.TaskGroup{
					{
						Name: stringToPtr("foo"),
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
				ID:   stringToPtr("foo"),
				Name: stringToPtr("foo"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: stringToPtr("bar"),
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
				ID:   stringToPtr("address_mode_driver"),
				Name: stringToPtr("address_mode_driver"),
				Type: stringToPtr("service"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: stringToPtr("group"),
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
				ID:   stringToPtr("service_check_restart"),
				Name: stringToPtr("service_check_restart"),
				Type: stringToPtr("service"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: stringToPtr("group"),
						Tasks: []*api.Task{
							{
								Name: "task",
								Services: []*api.Service{
									{
										Name: "http-service",
										CheckRestart: &api.CheckRestart{
											Limit:          3,
											Grace:          timeToPtr(10 * time.Second),
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
				ID:   stringToPtr("service_meta"),
				Name: stringToPtr("service_meta"),
				Type: stringToPtr("service"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: stringToPtr("group"),
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
				ID:   stringToPtr("service_eto"),
				Name: stringToPtr("service_eto"),
				Type: stringToPtr("service"),
				TaskGroups: []*api.TaskGroup{{
					Name: stringToPtr("group"),
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
				ID:          stringToPtr("foo"),
				Name:        stringToPtr("foo"),
				Type:        stringToPtr("batch"),
				Datacenters: []string{"dc1"},
				Reschedule: &api.ReschedulePolicy{
					Attempts:      intToPtr(15),
					Interval:      timeToPtr(30 * time.Minute),
					DelayFunction: stringToPtr("constant"),
					Delay:         timeToPtr(10 * time.Second),
				},
				TaskGroups: []*api.TaskGroup{
					{
						Name:  stringToPtr("bar"),
						Count: intToPtr(3),
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
				ID:          stringToPtr("foo"),
				Name:        stringToPtr("foo"),
				Type:        stringToPtr("batch"),
				Datacenters: []string{"dc1"},
				Reschedule: &api.ReschedulePolicy{
					DelayFunction: stringToPtr("exponential"),
					Delay:         timeToPtr(10 * time.Second),
					MaxDelay:      timeToPtr(120 * time.Second),
					Unlimited:     boolToPtr(true),
				},
				TaskGroups: []*api.TaskGroup{
					{
						Name:  stringToPtr("bar"),
						Count: intToPtr(3),
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
				ID:          stringToPtr("foo"),
				Name:        stringToPtr("foo"),
				Type:        stringToPtr("batch"),
				Datacenters: []string{"dc1"},
				Migrate: &api.MigrateStrategy{
					MaxParallel:     intToPtr(2),
					HealthCheck:     stringToPtr("task_states"),
					MinHealthyTime:  timeToPtr(11 * time.Second),
					HealthyDeadline: timeToPtr(11 * time.Minute),
				},
				TaskGroups: []*api.TaskGroup{
					{
						Name:  stringToPtr("bar"),
						Count: intToPtr(3),
						Migrate: &api.MigrateStrategy{
							MaxParallel:     intToPtr(3),
							HealthCheck:     stringToPtr("checks"),
							MinHealthyTime:  timeToPtr(1 * time.Second),
							HealthyDeadline: timeToPtr(1 * time.Minute),
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
				ID:          stringToPtr("foo"),
				Name:        stringToPtr("foo"),
				Datacenters: []string{"dc1"},
				TaskGroups: []*api.TaskGroup{
					{
						Name:          stringToPtr("bar"),
						ShutdownDelay: timeToPtr(14 * time.Second),
						Count:         intToPtr(3),
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
											CPU:      intToPtr(500),
											MemoryMB: intToPtr(1024),
										},
										Env: map[string]string{
											"FOO": "abc",
										},
										ShutdownDelay: timeToPtr(5 * time.Second),
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
											MBits: intToPtr(10),
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
				ID:   stringToPtr("group_service_check_script"),
				Name: stringToPtr("group_service_check_script"),
				TaskGroups: []*api.TaskGroup{
					{
						Name:  stringToPtr("group"),
						Count: intToPtr(1),
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
				ID:   stringToPtr("group_service_proxy_expose"),
				Name: stringToPtr("group_service_proxy_expose"),
				TaskGroups: []*api.TaskGroup{{
					Name: stringToPtr("group"),
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
				ID:   stringToPtr("sidecar_task_name"),
				Name: stringToPtr("sidecar_task_name"),
				Type: stringToPtr("service"),
				TaskGroups: []*api.TaskGroup{{
					Name: stringToPtr("group"),
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
				ID:   stringToPtr("service-connect-proxy"),
				Name: stringToPtr("service-connect-proxy"),
				Type: stringToPtr("service"),
				TaskGroups: []*api.TaskGroup{{
					Name: stringToPtr("group"),
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
				ID:   stringToPtr("connect-proxy-local-service"),
				Name: stringToPtr("connect-proxy-local-service"),
				Type: stringToPtr("service"),
				TaskGroups: []*api.TaskGroup{{
					Name: stringToPtr("group"),
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
				ID:   stringToPtr("group_service_proxy_expose"),
				Name: stringToPtr("group_service_proxy_expose"),
				TaskGroups: []*api.TaskGroup{{
					Name: stringToPtr("group"),
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
				ID:   stringToPtr("connect_native_service"),
				Name: stringToPtr("connect_native_service"),
				TaskGroups: []*api.TaskGroup{{
					Name: stringToPtr("group"),
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
				ID:   stringToPtr("group_service_eto"),
				Name: stringToPtr("group_service_eto"),
				TaskGroups: []*api.TaskGroup{{
					Name: stringToPtr("group"),
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
				ID:   stringToPtr("elastic"),
				Name: stringToPtr("elastic"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: stringToPtr("group"),
						Scaling: &api.ScalingPolicy{
							Type: "horizontal",
							Min:  int64ToPtr(5),
							Max:  int64ToPtr(100),
							Policy: map[string]interface{}{
								"foo": "bar",
								"b":   true,
								"val": 5,
								"f":   .1,

								"check": []map[string]interface{}{
									{"foo": []map[string]interface{}{
										{"query": "some_query"},
									}},
								},
							},
							Enabled: boolToPtr(false),
						},
					},
				},
			},
			false,
		},
		{
			"task-scaling-policy.hcl",
			&api.Job{
				ID:   stringToPtr("foo"),
				Name: stringToPtr("foo"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: stringToPtr("bar"),
						Tasks: []*api.Task{
							{
								Name:   "bar",
								Driver: "docker",
								ScalingPolicies: []*api.ScalingPolicy{
									{
										Type:   "vertical_cpu",
										Target: nil,
										Min:    int64ToPtr(50),
										Max:    int64ToPtr(1000),
										Policy: map[string]interface{}{
											"test": "cpu",
										},
										Enabled: boolToPtr(true),
									},
									{
										Type:   "vertical_mem",
										Target: nil,
										Min:    int64ToPtr(128),
										Max:    int64ToPtr(1024),
										Policy: map[string]interface{}{
											"test": "mem",
										},
										Enabled: boolToPtr(false),
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
			"tg-service-connect-gateway-ingress.hcl",
			&api.Job{
				ID:   stringToPtr("connect_gateway_ingress"),
				Name: stringToPtr("connect_gateway_ingress"),
				TaskGroups: []*api.TaskGroup{{
					Name: stringToPtr("group"),
					Services: []*api.Service{{
						Name: "ingress-gateway-service",
						Connect: &api.ConsulConnect{
							Gateway: &api.ConsulGateway{
								Proxy: &api.ConsulGatewayProxy{
									ConnectTimeout:                  timeToPtr(3 * time.Second),
									EnvoyGatewayBindTaggedAddresses: true,
									EnvoyGatewayBindAddresses: map[string]*api.ConsulGatewayBindAddress{
										"listener1": {Name: "listener1", Address: "10.0.0.1", Port: 8888},
										"listener2": {Name: "listener2", Address: "10.0.0.2", Port: 8889},
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
				ID:   stringToPtr("elastic"),
				Name: stringToPtr("elastic"),
				TaskGroups: []*api.TaskGroup{
					{
						Name: stringToPtr("group"),
						Scaling: &api.ScalingPolicy{
							Type:    "horizontal",
							Min:     nil,
							Max:     int64ToPtr(10),
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
			"tg-scaling-policy-with-label.hcl",
			nil,
			true,
		},
		{
			"tg-scaling-policy-invalid-type.hcl",
			nil,
			true,
		},
		{
			"task-scaling-policy-missing-name.hcl",
			nil,
			true,
		},
		{
			"task-scaling-policy-multi-name.hcl",
			nil,
			true,
		},
		{
			"task-scaling-policy-multi-cpu.hcl",
			nil,
			true,
		},
		{
			"task-scaling-policy-invalid-type.hcl",
			nil,
			true,
		},
		{
			"task-scaling-policy-invalid-resource.hcl",
			nil,
			true,
		},
		{
			"multiregion.hcl",
			&api.Job{
				ID:   stringToPtr("multiregion_job"),
				Name: stringToPtr("multiregion_job"),
				Multiregion: &api.Multiregion{
					Strategy: &api.MultiregionStrategy{
						MaxParallel: intToPtr(1),
						OnFailure:   stringToPtr("fail_all"),
					},
					Regions: []*api.MultiregionRegion{
						{
							Name:        "west",
							Count:       intToPtr(2),
							Datacenters: []string{"west-1"},
							Meta:        map[string]string{"region_code": "W"},
						},
						{
							Name:        "east",
							Count:       intToPtr(1),
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
