package jobspec

import (
	"path/filepath"
	"reflect"
	"testing"

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
				Name:        "binstore-storagelocker",
				Type:        "service",
				Priority:    50,
				AllAtOnce:   true,
				Datacenters: []string{"us2", "eu1"},

				TaskGroups: []*structs.TaskGroup{
					&structs.TaskGroup{
						Name:  "outside",
						Count: 1,
						Tasks: []*structs.Task{
							&structs.Task{
								Name:   "outside",
								Driver: "java",
								Config: map[string]string{
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
						Tasks: []*structs.Task{
							&structs.Task{
								Name:   "binstore",
								Driver: "docker",
								Config: map[string]string{
									"image": "hashicorp/binstore",
								},
								Resources: &structs.Resources{
									CPU:      500,
									MemoryMB: 128,
								},
							},
							&structs.Task{
								Name:   "storagelocker",
								Driver: "java",
								Config: map[string]string{
									"image": "hashicorp/storagelocker",
								},
								Resources: &structs.Resources{
									CPU:      500,
									MemoryMB: 128,
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
