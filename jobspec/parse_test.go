package jobspec

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/kr/pretty"

	capi "github.com/hashicorp/consul/api"
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
				VaultToken:  helper.StringToPtr("foo"),

				Meta: map[string]string{
					"foo": "bar",
				},

				Constraints: []*api.Constraint{
					&api.Constraint{
						LTarget: "kernel.os",
						RTarget: "windows",
						Operand: "=",
					},
				},

				Update: &api.UpdateStrategy{
					Stagger:         helper.TimeToPtr(60 * time.Second),
					MaxParallel:     helper.IntToPtr(2),
					HealthCheck:     helper.StringToPtr("manual"),
					MinHealthyTime:  helper.TimeToPtr(10 * time.Second),
					HealthyDeadline: helper.TimeToPtr(10 * time.Minute),
					AutoRevert:      helper.BoolToPtr(true),
					Canary:          helper.IntToPtr(1),
				},

				TaskGroups: []*api.TaskGroup{
					&api.TaskGroup{
						Name: helper.StringToPtr("outside"),
						Tasks: []*api.Task{
							&api.Task{
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

					&api.TaskGroup{
						Name:  helper.StringToPtr("binsl"),
						Count: helper.IntToPtr(5),
						Constraints: []*api.Constraint{
							&api.Constraint{
								LTarget: "kernel.os",
								RTarget: "linux",
								Operand: "=",
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
						EphemeralDisk: &api.EphemeralDisk{
							Sticky: helper.BoolToPtr(true),
							SizeMB: helper.IntToPtr(150),
						},
						Update: &api.UpdateStrategy{
							MaxParallel:     helper.IntToPtr(3),
							HealthCheck:     helper.StringToPtr("checks"),
							MinHealthyTime:  helper.TimeToPtr(1 * time.Second),
							HealthyDeadline: helper.TimeToPtr(1 * time.Minute),
							AutoRevert:      helper.BoolToPtr(false),
							Canary:          helper.IntToPtr(2),
						},
						Tasks: []*api.Task{
							&api.Task{
								Name:   "binstore",
								Driver: "docker",
								User:   "bob",
								Config: map[string]interface{}{
									"image": "hashicorp/binstore",
									"labels": []map[string]interface{}{
										map[string]interface{}{
											"FOO": "bar",
										},
									},
								},
								Services: []*api.Service{
									{
										Tags:      []string{"foo", "bar"},
										PortLabel: "http",
										Checks: []api.ServiceCheck{
											{
												Name:      "check-name",
												Type:      "tcp",
												PortLabel: "admin",
												Interval:  10 * time.Second,
												Timeout:   2 * time.Second,
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
										&api.NetworkResource{
											MBits:         helper.IntToPtr(100),
											ReservedPorts: []api.Port{{Label: "one", Value: 1}, {Label: "two", Value: 2}, {Label: "three", Value: 3}},
											DynamicPorts:  []api.Port{{Label: "http", Value: 0}, {Label: "https", Value: 0}, {Label: "admin", Value: 0}},
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
								Leader: true,
							},
							&api.Task{
								Name:   "storagelocker",
								Driver: "docker",
								User:   "",
								Config: map[string]interface{}{
									"image": "hashicorp/storagelocker",
								},
								Resources: &api.Resources{
									CPU:      helper.IntToPtr(500),
									MemoryMB: helper.IntToPtr(128),
									IOPS:     helper.IntToPtr(30),
								},
								Constraints: []*api.Constraint{
									&api.Constraint{
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
					&api.Constraint{
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
					&api.Constraint{
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
					&api.Constraint{
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
					&api.Constraint{
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
					&api.Constraint{
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
					&api.TaskGroup{
						Name: helper.StringToPtr("bar"),
						Tasks: []*api.Task{
							&api.Task{
								Name:   "bar",
								Driver: "docker",
								Config: map[string]interface{}{
									"image": "hashicorp/image",
									"port_map": []map[string]interface{}{
										map[string]interface{}{
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
					&api.TaskGroup{
						Name: helper.StringToPtr("binsl"),
						Tasks: []*api.Task{
							&api.Task{
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
			"service-check-initial-status.hcl",
			&api.Job{
				ID:   helper.StringToPtr("check_initial_status"),
				Name: helper.StringToPtr("check_initial_status"),
				Type: helper.StringToPtr("service"),
				TaskGroups: []*api.TaskGroup{
					&api.TaskGroup{
						Name:  helper.StringToPtr("group"),
						Count: helper.IntToPtr(1),
						Tasks: []*api.Task{
							&api.Task{
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
					&api.TaskGroup{
						Name: helper.StringToPtr("cache"),
						Tasks: []*api.Task{
							&api.Task{
								Name: "redis",
								Vault: &api.Vault{
									Policies:   []string{"group"},
									Env:        helper.BoolToPtr(true),
									ChangeMode: helper.StringToPtr(structs.VaultChangeModeRestart),
								},
							},
							&api.Task{
								Name: "redis2",
								Vault: &api.Vault{
									Policies:   []string{"task"},
									Env:        helper.BoolToPtr(false),
									ChangeMode: helper.StringToPtr(structs.VaultChangeModeRestart),
								},
							},
						},
					},
					&api.TaskGroup{
						Name: helper.StringToPtr("cache2"),
						Tasks: []*api.Task{
							&api.Task{
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

	if !strings.Contains(err.Error(), "* group: 'binsl', task: 'binstore', service: 'foo', check -> invalid key: nterval") {
		t.Fatalf("Expected key error; got %v", err)
	}
}
