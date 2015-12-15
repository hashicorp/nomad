package jobspec

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

func TestParse(t *testing.T) {
	cases := []struct {
		File   string
		Result *structs.Job
		Err    bool
	}{
		{
			"basic.hcl",
			&structs.Job{
				ID:          "binstore-storagelocker",
				Name:        "binstore-storagelocker",
				Type:        "service",
				Priority:    50,
				AllAtOnce:   true,
				Datacenters: []string{"us2", "eu1"},
				Region:      "global",

				Meta: map[string]string{
					"foo": "bar",
				},

				Constraints: []*structs.Constraint{
					&structs.Constraint{
						LTarget: "kernel.os",
						RTarget: "windows",
						Operand: "=",
					},
				},

				Update: structs.UpdateStrategy{
					Stagger:     60 * time.Second,
					MaxParallel: 2,
				},

				TaskGroups: []*structs.TaskGroup{
					&structs.TaskGroup{
						Name:  "outside",
						Count: 1,
						RestartPolicy: &structs.RestartPolicy{
							Attempts: 2,
							Interval: 1 * time.Minute,
							Delay:    15 * time.Second,
						},
						Tasks: []*structs.Task{
							&structs.Task{
								Name:   "outside",
								Driver: "java",
								Config: map[string]interface{}{
									"jar": "s3://my-cool-store/foo.jar",
								},
								Meta: map[string]string{
									"my-cool-key": "foobar",
								},
							},
						},
					},

					&structs.TaskGroup{
						Name:  "binsl",
						Count: 5,
						Constraints: []*structs.Constraint{
							&structs.Constraint{
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
						RestartPolicy: &structs.RestartPolicy{
							Interval: 10 * time.Minute,
							Attempts: 5,
							Delay:    15 * time.Second,
						},
						Tasks: []*structs.Task{
							&structs.Task{
								Name:   "binstore",
								Driver: "docker",
								Config: map[string]interface{}{
									"image": "hashicorp/binstore",
								},
								Services: []*structs.Service{
									{
										Name:      "binstore-storagelocker-binsl-binstore",
										Tags:      []string{"foo", "bar"},
										PortLabel: "http",
										Checks: []*structs.ServiceCheck{
											{
												Name:     "check-name",
												Type:     "tcp",
												Interval: 10 * time.Second,
												Timeout:  2 * time.Second,
											},
										},
									},
								},
								Env: map[string]string{
									"HELLO": "world",
									"LOREM": "ipsum",
								},
								Resources: &structs.Resources{
									CPU:      500,
									MemoryMB: 128,
									Networks: []*structs.NetworkResource{
										&structs.NetworkResource{
											MBits:         100,
											ReservedPorts: []structs.Port{{"one", 1}, {"two", 2}, {"three", 3}},
											DynamicPorts:  []structs.Port{{"http", 0}, {"https", 0}, {"admin", 0}},
										},
									},
								},
							},
							&structs.Task{
								Name:   "storagelocker",
								Driver: "java",
								Config: map[string]interface{}{
									"image": "hashicorp/storagelocker",
								},
								Resources: &structs.Resources{
									CPU:      500,
									MemoryMB: 128,
								},
								Constraints: []*structs.Constraint{
									&structs.Constraint{
										LTarget: "kernel.arch",
										RTarget: "amd64",
										Operand: "=",
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
			"default-job.hcl",
			&structs.Job{
				ID:       "foo",
				Name:     "foo",
				Priority: 50,
				Region:   "global",
				Type:     "service",
			},
			false,
		},

		{
			"version-constraint.hcl",
			&structs.Job{
				ID:       "foo",
				Name:     "foo",
				Priority: 50,
				Region:   "global",
				Type:     "service",
				Constraints: []*structs.Constraint{
					&structs.Constraint{
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
			&structs.Job{
				ID:       "foo",
				Name:     "foo",
				Priority: 50,
				Region:   "global",
				Type:     "service",
				Constraints: []*structs.Constraint{
					&structs.Constraint{
						LTarget: "$attr.kernel.version",
						RTarget: "[0-9.]+",
						Operand: structs.ConstraintRegex,
					},
				},
			},
			false,
		},

		{
			"distinctHosts-constraint.hcl",
			&structs.Job{
				ID:       "foo",
				Name:     "foo",
				Priority: 50,
				Region:   "global",
				Type:     "service",
				Constraints: []*structs.Constraint{
					&structs.Constraint{
						Operand: structs.ConstraintDistinctHosts,
					},
				},
			},
			false,
		},

		{
			"periodic-cron.hcl",
			&structs.Job{
				ID:       "foo",
				Name:     "foo",
				Priority: 50,
				Region:   "global",
				Type:     "service",
				Periodic: &structs.PeriodicConfig{
					Enabled:  true,
					SpecType: structs.PeriodicSpecCron,
					Spec:     "*/5 * * *",
				},
			},
			false,
		},

		{
			"gc.hcl",
			&structs.Job{
				ID:       "foo",
				Name:     "foo",
				Priority: 50,
				Region:   "global",
				Type:     "service",
				GC:       true,
			},
			false,
		},

		{
			"specify-job.hcl",
			&structs.Job{
				ID:       "job1",
				Name:     "My Job",
				Priority: 50,
				Region:   "global",
				Type:     "service",
			},
			false,
		},

		{
			"task-nested-config.hcl",
			&structs.Job{
				Region:   "global",
				ID:       "foo",
				Name:     "foo",
				Type:     "service",
				Priority: 50,

				TaskGroups: []*structs.TaskGroup{
					&structs.TaskGroup{
						Name:  "bar",
						Count: 1,
						RestartPolicy: &structs.RestartPolicy{
							Attempts: 2,
							Interval: 1 * time.Minute,
							Delay:    15 * time.Second,
						},
						Tasks: []*structs.Task{
							&structs.Task{
								Name:   "bar",
								Driver: "docker",
								Config: map[string]interface{}{
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
			t.Fatalf("file: %s\n\n%#v\n\n%#v", tc.File, actual, tc.Result)
		}
	}
}

func TestBadPorts(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("./test-fixtures", "bad-ports.hcl"))
	if err != nil {
		t.Fatalf("Can't get absoluate path for file: %s", err)
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

	if !strings.Contains(err.Error(), "Found a port label collision") {
		t.Fatalf("Expected collision error; got %v", err)
	}
}

func TestIncompleteServiceDefn(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("./test-fixtures", "incorrect-service-def.hcl"))
	if err != nil {
		t.Fatalf("Can't get absolute path for file: %s", err)
	}

	_, err = ParseFile(path)

	if err == nil {
		t.Fatalf("Expected an error")
	}

	if !strings.Contains(err.Error(), "Only one service block may omit the Name field") {
		t.Fatalf("Expected collision error; got %v", err)
	}
}
