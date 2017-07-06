package structs

import (
	"reflect"
	"testing"
	"time"
)

func TestJobDiff(t *testing.T) {
	cases := []struct {
		Old, New   *Job
		Expected   *JobDiff
		Error      bool
		Contextual bool
	}{
		{
			Old: nil,
			New: nil,
			Expected: &JobDiff{
				Type: DiffTypeNone,
			},
		},
		{
			// Different IDs
			Old: &Job{
				ID: "foo",
			},
			New: &Job{
				ID: "bar",
			},
			Error: true,
		},
		{
			// Primitive only that is the same
			Old: &Job{
				Region:    "foo",
				ID:        "foo",
				Name:      "foo",
				Type:      "batch",
				Priority:  10,
				AllAtOnce: true,
				Meta: map[string]string{
					"foo": "bar",
				},
			},
			New: &Job{
				Region:    "foo",
				ID:        "foo",
				Name:      "foo",
				Type:      "batch",
				Priority:  10,
				AllAtOnce: true,
				Meta: map[string]string{
					"foo": "bar",
				},
			},
			Expected: &JobDiff{
				Type: DiffTypeNone,
				ID:   "foo",
			},
		},
		{
			// Primitive only that is has diffs
			Old: &Job{
				Region:    "foo",
				ID:        "foo",
				Name:      "foo",
				Type:      "batch",
				Priority:  10,
				AllAtOnce: true,
				Meta: map[string]string{
					"foo": "bar",
				},
			},
			New: &Job{
				Region:    "bar",
				ID:        "foo",
				Name:      "bar",
				Type:      "system",
				Priority:  100,
				AllAtOnce: false,
				Meta: map[string]string{
					"foo": "baz",
				},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				ID:   "foo",
				Fields: []*FieldDiff{
					{
						Type: DiffTypeEdited,
						Name: "AllAtOnce",
						Old:  "true",
						New:  "false",
					},
					{
						Type: DiffTypeEdited,
						Name: "Meta[foo]",
						Old:  "bar",
						New:  "baz",
					},
					{
						Type: DiffTypeEdited,
						Name: "Name",
						Old:  "foo",
						New:  "bar",
					},
					{
						Type: DiffTypeEdited,
						Name: "Priority",
						Old:  "10",
						New:  "100",
					},
					{
						Type: DiffTypeEdited,
						Name: "Region",
						Old:  "foo",
						New:  "bar",
					},
					{
						Type: DiffTypeEdited,
						Name: "Type",
						Old:  "batch",
						New:  "system",
					},
				},
			},
		},
		{
			// Primitive only deleted job
			Old: &Job{
				Region:    "foo",
				ID:        "foo",
				Name:      "foo",
				Type:      "batch",
				Priority:  10,
				AllAtOnce: true,
				Meta: map[string]string{
					"foo": "bar",
				},
			},
			New: nil,
			Expected: &JobDiff{
				Type: DiffTypeDeleted,
				ID:   "foo",
				Fields: []*FieldDiff{
					{
						Type: DiffTypeDeleted,
						Name: "AllAtOnce",
						Old:  "true",
						New:  "",
					},
					{
						Type: DiffTypeDeleted,
						Name: "Meta[foo]",
						Old:  "bar",
						New:  "",
					},
					{
						Type: DiffTypeDeleted,
						Name: "Name",
						Old:  "foo",
						New:  "",
					},
					{
						Type: DiffTypeDeleted,
						Name: "Priority",
						Old:  "10",
						New:  "",
					},
					{
						Type: DiffTypeDeleted,
						Name: "Region",
						Old:  "foo",
						New:  "",
					},
					{
						Type: DiffTypeDeleted,
						Name: "Stop",
						Old:  "false",
						New:  "",
					},
					{
						Type: DiffTypeDeleted,
						Name: "Type",
						Old:  "batch",
						New:  "",
					},
				},
			},
		},
		{
			// Primitive only added job
			Old: nil,
			New: &Job{
				Region:    "foo",
				ID:        "foo",
				Name:      "foo",
				Type:      "batch",
				Priority:  10,
				AllAtOnce: true,
				Meta: map[string]string{
					"foo": "bar",
				},
			},
			Expected: &JobDiff{
				Type: DiffTypeAdded,
				ID:   "foo",
				Fields: []*FieldDiff{
					{
						Type: DiffTypeAdded,
						Name: "AllAtOnce",
						Old:  "",
						New:  "true",
					},
					{
						Type: DiffTypeAdded,
						Name: "Meta[foo]",
						Old:  "",
						New:  "bar",
					},
					{
						Type: DiffTypeAdded,
						Name: "Name",
						Old:  "",
						New:  "foo",
					},
					{
						Type: DiffTypeAdded,
						Name: "Priority",
						Old:  "",
						New:  "10",
					},
					{
						Type: DiffTypeAdded,
						Name: "Region",
						Old:  "",
						New:  "foo",
					},
					{
						Type: DiffTypeAdded,
						Name: "Stop",
						Old:  "",
						New:  "false",
					},
					{
						Type: DiffTypeAdded,
						Name: "Type",
						Old:  "",
						New:  "batch",
					},
				},
			},
		},
		{
			// Map diff
			Old: &Job{
				Meta: map[string]string{
					"foo": "foo",
					"bar": "bar",
				},
			},
			New: &Job{
				Meta: map[string]string{
					"bar": "bar",
					"baz": "baz",
				},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Fields: []*FieldDiff{
					{
						Type: DiffTypeAdded,
						Name: "Meta[baz]",
						Old:  "",
						New:  "baz",
					},
					{
						Type: DiffTypeDeleted,
						Name: "Meta[foo]",
						Old:  "foo",
						New:  "",
					},
				},
			},
		},
		{
			// Datacenter diff both added and removed
			Old: &Job{
				Datacenters: []string{"foo", "bar"},
			},
			New: &Job{
				Datacenters: []string{"baz", "bar"},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Datacenters",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Datacenters",
								Old:  "",
								New:  "baz",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Datacenters",
								Old:  "foo",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			// Datacenter diff just added
			Old: &Job{
				Datacenters: []string{"foo", "bar"},
			},
			New: &Job{
				Datacenters: []string{"foo", "bar", "baz"},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Datacenters",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Datacenters",
								Old:  "",
								New:  "baz",
							},
						},
					},
				},
			},
		},
		{
			// Datacenter diff just deleted
			Old: &Job{
				Datacenters: []string{"foo", "bar"},
			},
			New: &Job{
				Datacenters: []string{"foo"},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeDeleted,
						Name: "Datacenters",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "Datacenters",
								Old:  "bar",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			// Datacenter contextual no change
			Contextual: true,
			Old: &Job{
				Datacenters: []string{"foo", "bar"},
			},
			New: &Job{
				Datacenters: []string{"foo", "bar"},
			},
			Expected: &JobDiff{
				Type: DiffTypeNone,
			},
		},
		{
			// Datacenter contextual
			Contextual: true,
			Old: &Job{
				Datacenters: []string{"foo", "bar"},
			},
			New: &Job{
				Datacenters: []string{"foo", "bar", "baz"},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Datacenters",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Datacenters",
								Old:  "",
								New:  "baz",
							},
							{
								Type: DiffTypeNone,
								Name: "Datacenters",
								Old:  "bar",
								New:  "bar",
							},
							{
								Type: DiffTypeNone,
								Name: "Datacenters",
								Old:  "foo",
								New:  "foo",
							},
						},
					},
				},
			},
		},
		{
			// Periodic added
			Old: &Job{},
			New: &Job{
				Periodic: &PeriodicConfig{
					Enabled:         false,
					Spec:            "*/15 * * * * *",
					SpecType:        "foo",
					ProhibitOverlap: false,
					TimeZone:        "Europe/Minsk",
				},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Periodic",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Enabled",
								Old:  "",
								New:  "false",
							},
							{
								Type: DiffTypeAdded,
								Name: "ProhibitOverlap",
								Old:  "",
								New:  "false",
							},
							{
								Type: DiffTypeAdded,
								Name: "Spec",
								Old:  "",
								New:  "*/15 * * * * *",
							},
							{
								Type: DiffTypeAdded,
								Name: "SpecType",
								Old:  "",
								New:  "foo",
							},
							{
								Type: DiffTypeAdded,
								Name: "TimeZone",
								Old:  "",
								New:  "Europe/Minsk",
							},
						},
					},
				},
			},
		},
		{
			// Periodic deleted
			Old: &Job{
				Periodic: &PeriodicConfig{
					Enabled:         false,
					Spec:            "*/15 * * * * *",
					SpecType:        "foo",
					ProhibitOverlap: false,
					TimeZone:        "Europe/Minsk",
				},
			},
			New: &Job{},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeDeleted,
						Name: "Periodic",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "Enabled",
								Old:  "false",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "ProhibitOverlap",
								Old:  "false",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Spec",
								Old:  "*/15 * * * * *",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "SpecType",
								Old:  "foo",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "TimeZone",
								Old:  "Europe/Minsk",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			// Periodic edited
			Old: &Job{
				Periodic: &PeriodicConfig{
					Enabled:         false,
					Spec:            "*/15 * * * * *",
					SpecType:        "foo",
					ProhibitOverlap: false,
					TimeZone:        "Europe/Minsk",
				},
			},
			New: &Job{
				Periodic: &PeriodicConfig{
					Enabled:         true,
					Spec:            "* * * * * *",
					SpecType:        "cron",
					ProhibitOverlap: true,
					TimeZone:        "America/Los_Angeles",
				},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Periodic",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "Enabled",
								Old:  "false",
								New:  "true",
							},
							{
								Type: DiffTypeEdited,
								Name: "ProhibitOverlap",
								Old:  "false",
								New:  "true",
							},
							{
								Type: DiffTypeEdited,
								Name: "Spec",
								Old:  "*/15 * * * * *",
								New:  "* * * * * *",
							},
							{
								Type: DiffTypeEdited,
								Name: "SpecType",
								Old:  "foo",
								New:  "cron",
							},
							{
								Type: DiffTypeEdited,
								Name: "TimeZone",
								Old:  "Europe/Minsk",
								New:  "America/Los_Angeles",
							},
						},
					},
				},
			},
		},
		{
			// Periodic edited with context
			Contextual: true,
			Old: &Job{
				Periodic: &PeriodicConfig{
					Enabled:         false,
					Spec:            "*/15 * * * * *",
					SpecType:        "foo",
					ProhibitOverlap: false,
					TimeZone:        "Europe/Minsk",
				},
			},
			New: &Job{
				Periodic: &PeriodicConfig{
					Enabled:         true,
					Spec:            "* * * * * *",
					SpecType:        "foo",
					ProhibitOverlap: false,
					TimeZone:        "Europe/Minsk",
				},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Periodic",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "Enabled",
								Old:  "false",
								New:  "true",
							},
							{
								Type: DiffTypeNone,
								Name: "ProhibitOverlap",
								Old:  "false",
								New:  "false",
							},
							{
								Type: DiffTypeEdited,
								Name: "Spec",
								Old:  "*/15 * * * * *",
								New:  "* * * * * *",
							},
							{
								Type: DiffTypeNone,
								Name: "SpecType",
								Old:  "foo",
								New:  "foo",
							},
							{
								Type: DiffTypeNone,
								Name: "TimeZone",
								Old:  "Europe/Minsk",
								New:  "Europe/Minsk",
							},
						},
					},
				},
			},
		},
		{
			// Constraints edited
			Old: &Job{
				Constraints: []*Constraint{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
						str:     "foo",
					},
					{
						LTarget: "bar",
						RTarget: "bar",
						Operand: "bar",
						str:     "bar",
					},
				},
			},
			New: &Job{
				Constraints: []*Constraint{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
						str:     "foo",
					},
					{
						LTarget: "baz",
						RTarget: "baz",
						Operand: "baz",
						str:     "baz",
					},
				},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Constraint",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "LTarget",
								Old:  "",
								New:  "baz",
							},
							{
								Type: DiffTypeAdded,
								Name: "Operand",
								Old:  "",
								New:  "baz",
							},
							{
								Type: DiffTypeAdded,
								Name: "RTarget",
								Old:  "",
								New:  "baz",
							},
						},
					},
					{
						Type: DiffTypeDeleted,
						Name: "Constraint",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "LTarget",
								Old:  "bar",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Operand",
								Old:  "bar",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "RTarget",
								Old:  "bar",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			// Task groups edited
			Old: &Job{
				TaskGroups: []*TaskGroup{
					{
						Name:  "foo",
						Count: 1,
					},
					{
						Name:  "bar",
						Count: 1,
					},
					{
						Name:  "baz",
						Count: 1,
					},
				},
			},
			New: &Job{
				TaskGroups: []*TaskGroup{
					{
						Name:  "bar",
						Count: 1,
					},
					{
						Name:  "baz",
						Count: 2,
					},
					{
						Name:  "bam",
						Count: 1,
					},
				},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				TaskGroups: []*TaskGroupDiff{
					{
						Type: DiffTypeAdded,
						Name: "bam",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Count",
								Old:  "",
								New:  "1",
							},
						},
					},
					{
						Type: DiffTypeNone,
						Name: "bar",
					},
					{
						Type: DiffTypeEdited,
						Name: "baz",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "Count",
								Old:  "1",
								New:  "2",
							},
						},
					},
					{
						Type: DiffTypeDeleted,
						Name: "foo",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "Count",
								Old:  "1",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			// Parameterized Job added
			Old: &Job{},
			New: &Job{
				ParameterizedJob: &ParameterizedJobConfig{
					Payload:      DispatchPayloadRequired,
					MetaOptional: []string{"foo"},
					MetaRequired: []string{"bar"},
				},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "ParameterizedJob",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Payload",
								Old:  "",
								New:  DispatchPayloadRequired,
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeAdded,
								Name: "MetaOptional",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "MetaOptional",
										Old:  "",
										New:  "foo",
									},
								},
							},
							{
								Type: DiffTypeAdded,
								Name: "MetaRequired",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "MetaRequired",
										Old:  "",
										New:  "bar",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			// Parameterized Job deleted
			Old: &Job{
				ParameterizedJob: &ParameterizedJobConfig{
					Payload:      DispatchPayloadRequired,
					MetaOptional: []string{"foo"},
					MetaRequired: []string{"bar"},
				},
			},
			New: &Job{},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeDeleted,
						Name: "ParameterizedJob",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "Payload",
								Old:  DispatchPayloadRequired,
								New:  "",
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeDeleted,
								Name: "MetaOptional",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeDeleted,
										Name: "MetaOptional",
										Old:  "foo",
										New:  "",
									},
								},
							},
							{
								Type: DiffTypeDeleted,
								Name: "MetaRequired",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeDeleted,
										Name: "MetaRequired",
										Old:  "bar",
										New:  "",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			// Parameterized Job edited
			Old: &Job{
				ParameterizedJob: &ParameterizedJobConfig{
					Payload:      DispatchPayloadRequired,
					MetaOptional: []string{"foo"},
					MetaRequired: []string{"bar"},
				},
			},
			New: &Job{
				ParameterizedJob: &ParameterizedJobConfig{
					Payload:      DispatchPayloadOptional,
					MetaOptional: []string{"bam"},
					MetaRequired: []string{"bang"},
				},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "ParameterizedJob",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "Payload",
								Old:  DispatchPayloadRequired,
								New:  DispatchPayloadOptional,
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeEdited,
								Name: "MetaOptional",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "MetaOptional",
										Old:  "",
										New:  "bam",
									},
									{
										Type: DiffTypeDeleted,
										Name: "MetaOptional",
										Old:  "foo",
										New:  "",
									},
								},
							},
							{
								Type: DiffTypeEdited,
								Name: "MetaRequired",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "MetaRequired",
										Old:  "",
										New:  "bang",
									},
									{
										Type: DiffTypeDeleted,
										Name: "MetaRequired",
										Old:  "bar",
										New:  "",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			// Parameterized Job edited with context
			Contextual: true,
			Old: &Job{
				ParameterizedJob: &ParameterizedJobConfig{
					Payload:      DispatchPayloadRequired,
					MetaOptional: []string{"foo"},
					MetaRequired: []string{"bar"},
				},
			},
			New: &Job{
				ParameterizedJob: &ParameterizedJobConfig{
					Payload:      DispatchPayloadOptional,
					MetaOptional: []string{"foo"},
					MetaRequired: []string{"bar"},
				},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "ParameterizedJob",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "Payload",
								Old:  DispatchPayloadRequired,
								New:  DispatchPayloadOptional,
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeNone,
								Name: "MetaOptional",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeNone,
										Name: "MetaOptional",
										Old:  "foo",
										New:  "foo",
									},
								},
							},
							{
								Type: DiffTypeNone,
								Name: "MetaRequired",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeNone,
										Name: "MetaRequired",
										Old:  "bar",
										New:  "bar",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for i, c := range cases {
		actual, err := c.Old.Diff(c.New, c.Contextual)
		if c.Error && err == nil {
			t.Fatalf("case %d: expected errored", i+1)
		} else if err != nil {
			if !c.Error {
				t.Fatalf("case %d: errored %#v", i+1, err)
			} else {
				continue
			}
		}

		if !reflect.DeepEqual(actual, c.Expected) {
			t.Fatalf("case %d: got:\n%#v\n want:\n%#v\n",
				i+1, actual, c.Expected)
		}
	}
}

func TestTaskGroupDiff(t *testing.T) {
	cases := []struct {
		Old, New   *TaskGroup
		Expected   *TaskGroupDiff
		Error      bool
		Contextual bool
	}{
		{
			Old: nil,
			New: nil,
			Expected: &TaskGroupDiff{
				Type: DiffTypeNone,
			},
		},
		{
			// Primitive only that has different names
			Old: &TaskGroup{
				Name:  "foo",
				Count: 10,
				Meta: map[string]string{
					"foo": "bar",
				},
			},
			New: &TaskGroup{
				Name:  "bar",
				Count: 10,
				Meta: map[string]string{
					"foo": "bar",
				},
			},
			Error: true,
		},
		{
			// Primitive only that is the same
			Old: &TaskGroup{
				Name:  "foo",
				Count: 10,
				Meta: map[string]string{
					"foo": "bar",
				},
			},
			New: &TaskGroup{
				Name:  "foo",
				Count: 10,
				Meta: map[string]string{
					"foo": "bar",
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeNone,
				Name: "foo",
			},
		},
		{
			// Primitive only that has diffs
			Old: &TaskGroup{
				Name:  "foo",
				Count: 10,
				Meta: map[string]string{
					"foo": "bar",
				},
			},
			New: &TaskGroup{
				Name:  "foo",
				Count: 100,
				Meta: map[string]string{
					"foo": "baz",
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Name: "foo",
				Fields: []*FieldDiff{
					{
						Type: DiffTypeEdited,
						Name: "Count",
						Old:  "10",
						New:  "100",
					},
					{
						Type: DiffTypeEdited,
						Name: "Meta[foo]",
						Old:  "bar",
						New:  "baz",
					},
				},
			},
		},
		{
			// Map diff
			Old: &TaskGroup{
				Meta: map[string]string{
					"foo": "foo",
					"bar": "bar",
				},
			},
			New: &TaskGroup{
				Meta: map[string]string{
					"bar": "bar",
					"baz": "baz",
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Fields: []*FieldDiff{
					{
						Type: DiffTypeAdded,
						Name: "Meta[baz]",
						Old:  "",
						New:  "baz",
					},
					{
						Type: DiffTypeDeleted,
						Name: "Meta[foo]",
						Old:  "foo",
						New:  "",
					},
				},
			},
		},
		{
			// Constraints edited
			Old: &TaskGroup{
				Constraints: []*Constraint{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
						str:     "foo",
					},
					{
						LTarget: "bar",
						RTarget: "bar",
						Operand: "bar",
						str:     "bar",
					},
				},
			},
			New: &TaskGroup{
				Constraints: []*Constraint{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
						str:     "foo",
					},
					{
						LTarget: "baz",
						RTarget: "baz",
						Operand: "baz",
						str:     "baz",
					},
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Constraint",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "LTarget",
								Old:  "",
								New:  "baz",
							},
							{
								Type: DiffTypeAdded,
								Name: "Operand",
								Old:  "",
								New:  "baz",
							},
							{
								Type: DiffTypeAdded,
								Name: "RTarget",
								Old:  "",
								New:  "baz",
							},
						},
					},
					{
						Type: DiffTypeDeleted,
						Name: "Constraint",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "LTarget",
								Old:  "bar",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Operand",
								Old:  "bar",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "RTarget",
								Old:  "bar",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			// RestartPolicy added
			Old: &TaskGroup{},
			New: &TaskGroup{
				RestartPolicy: &RestartPolicy{
					Attempts: 1,
					Interval: 1 * time.Second,
					Delay:    1 * time.Second,
					Mode:     "fail",
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "RestartPolicy",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Attempts",
								Old:  "",
								New:  "1",
							},
							{
								Type: DiffTypeAdded,
								Name: "Delay",
								Old:  "",
								New:  "1000000000",
							},
							{
								Type: DiffTypeAdded,
								Name: "Interval",
								Old:  "",
								New:  "1000000000",
							},
							{
								Type: DiffTypeAdded,
								Name: "Mode",
								Old:  "",
								New:  "fail",
							},
						},
					},
				},
			},
		},
		{
			// RestartPolicy deleted
			Old: &TaskGroup{
				RestartPolicy: &RestartPolicy{
					Attempts: 1,
					Interval: 1 * time.Second,
					Delay:    1 * time.Second,
					Mode:     "fail",
				},
			},
			New: &TaskGroup{},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeDeleted,
						Name: "RestartPolicy",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "Attempts",
								Old:  "1",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Delay",
								Old:  "1000000000",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Interval",
								Old:  "1000000000",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Mode",
								Old:  "fail",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			// RestartPolicy edited
			Old: &TaskGroup{
				RestartPolicy: &RestartPolicy{
					Attempts: 1,
					Interval: 1 * time.Second,
					Delay:    1 * time.Second,
					Mode:     "fail",
				},
			},
			New: &TaskGroup{
				RestartPolicy: &RestartPolicy{
					Attempts: 2,
					Interval: 2 * time.Second,
					Delay:    2 * time.Second,
					Mode:     "delay",
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "RestartPolicy",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "Attempts",
								Old:  "1",
								New:  "2",
							},
							{
								Type: DiffTypeEdited,
								Name: "Delay",
								Old:  "1000000000",
								New:  "2000000000",
							},
							{
								Type: DiffTypeEdited,
								Name: "Interval",
								Old:  "1000000000",
								New:  "2000000000",
							},
							{
								Type: DiffTypeEdited,
								Name: "Mode",
								Old:  "fail",
								New:  "delay",
							},
						},
					},
				},
			},
		},
		{
			// RestartPolicy edited with context
			Contextual: true,
			Old: &TaskGroup{
				RestartPolicy: &RestartPolicy{
					Attempts: 1,
					Interval: 1 * time.Second,
					Delay:    1 * time.Second,
					Mode:     "fail",
				},
			},
			New: &TaskGroup{
				RestartPolicy: &RestartPolicy{
					Attempts: 2,
					Interval: 2 * time.Second,
					Delay:    1 * time.Second,
					Mode:     "fail",
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "RestartPolicy",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "Attempts",
								Old:  "1",
								New:  "2",
							},
							{
								Type: DiffTypeNone,
								Name: "Delay",
								Old:  "1000000000",
								New:  "1000000000",
							},
							{
								Type: DiffTypeEdited,
								Name: "Interval",
								Old:  "1000000000",
								New:  "2000000000",
							},
							{
								Type: DiffTypeNone,
								Name: "Mode",
								Old:  "fail",
								New:  "fail",
							},
						},
					},
				},
			},
		},
		{
			// Update strategy deleted
			Old: &TaskGroup{
				Update: &UpdateStrategy{
					AutoRevert: true,
				},
			},
			New: &TaskGroup{},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeDeleted,
						Name: "Update",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "AutoRevert",
								Old:  "true",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Canary",
								Old:  "0",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "HealthyDeadline",
								Old:  "0",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "MaxParallel",
								Old:  "0",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "MinHealthyTime",
								Old:  "0",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			// Update strategy added
			Old: &TaskGroup{},
			New: &TaskGroup{
				Update: &UpdateStrategy{
					AutoRevert: true,
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Update",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "AutoRevert",
								Old:  "",
								New:  "true",
							},
							{
								Type: DiffTypeAdded,
								Name: "Canary",
								Old:  "",
								New:  "0",
							},
							{
								Type: DiffTypeAdded,
								Name: "HealthyDeadline",
								Old:  "",
								New:  "0",
							},
							{
								Type: DiffTypeAdded,
								Name: "MaxParallel",
								Old:  "",
								New:  "0",
							},
							{
								Type: DiffTypeAdded,
								Name: "MinHealthyTime",
								Old:  "",
								New:  "0",
							},
						},
					},
				},
			},
		},
		{
			// Update strategy edited
			Old: &TaskGroup{
				Update: &UpdateStrategy{
					MaxParallel:     5,
					HealthCheck:     "foo",
					MinHealthyTime:  1 * time.Second,
					HealthyDeadline: 30 * time.Second,
					AutoRevert:      true,
					Canary:          2,
				},
			},
			New: &TaskGroup{
				Update: &UpdateStrategy{
					MaxParallel:     7,
					HealthCheck:     "bar",
					MinHealthyTime:  2 * time.Second,
					HealthyDeadline: 31 * time.Second,
					AutoRevert:      false,
					Canary:          1,
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Update",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "AutoRevert",
								Old:  "true",
								New:  "false",
							},
							{
								Type: DiffTypeEdited,
								Name: "Canary",
								Old:  "2",
								New:  "1",
							},
							{
								Type: DiffTypeEdited,
								Name: "HealthCheck",
								Old:  "foo",
								New:  "bar",
							},
							{
								Type: DiffTypeEdited,
								Name: "HealthyDeadline",
								Old:  "30000000000",
								New:  "31000000000",
							},
							{
								Type: DiffTypeEdited,
								Name: "MaxParallel",
								Old:  "5",
								New:  "7",
							},
							{
								Type: DiffTypeEdited,
								Name: "MinHealthyTime",
								Old:  "1000000000",
								New:  "2000000000",
							},
						},
					},
				},
			},
		},
		{
			// Update strategy edited with context
			Contextual: true,
			Old: &TaskGroup{
				Update: &UpdateStrategy{
					MaxParallel:     5,
					HealthCheck:     "foo",
					MinHealthyTime:  1 * time.Second,
					HealthyDeadline: 30 * time.Second,
					AutoRevert:      true,
					Canary:          2,
				},
			},
			New: &TaskGroup{
				Update: &UpdateStrategy{
					MaxParallel:     7,
					HealthCheck:     "foo",
					MinHealthyTime:  1 * time.Second,
					HealthyDeadline: 30 * time.Second,
					AutoRevert:      true,
					Canary:          2,
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Update",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeNone,
								Name: "AutoRevert",
								Old:  "true",
								New:  "true",
							},
							{
								Type: DiffTypeNone,
								Name: "Canary",
								Old:  "2",
								New:  "2",
							},
							{
								Type: DiffTypeNone,
								Name: "HealthCheck",
								Old:  "foo",
								New:  "foo",
							},
							{
								Type: DiffTypeNone,
								Name: "HealthyDeadline",
								Old:  "30000000000",
								New:  "30000000000",
							},
							{
								Type: DiffTypeEdited,
								Name: "MaxParallel",
								Old:  "5",
								New:  "7",
							},
							{
								Type: DiffTypeNone,
								Name: "MinHealthyTime",
								Old:  "1000000000",
								New:  "1000000000",
							},
						},
					},
				},
			},
		},
		{
			// EphemeralDisk added
			Old: &TaskGroup{},
			New: &TaskGroup{
				EphemeralDisk: &EphemeralDisk{
					Migrate: true,
					Sticky:  true,
					SizeMB:  100,
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "EphemeralDisk",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Migrate",
								Old:  "",
								New:  "true",
							},
							{
								Type: DiffTypeAdded,
								Name: "SizeMB",
								Old:  "",
								New:  "100",
							},
							{
								Type: DiffTypeAdded,
								Name: "Sticky",
								Old:  "",
								New:  "true",
							},
						},
					},
				},
			},
		},
		{
			// EphemeralDisk deleted
			Old: &TaskGroup{
				EphemeralDisk: &EphemeralDisk{
					Migrate: true,
					Sticky:  true,
					SizeMB:  100,
				},
			},
			New: &TaskGroup{},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeDeleted,
						Name: "EphemeralDisk",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "Migrate",
								Old:  "true",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "SizeMB",
								Old:  "100",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Sticky",
								Old:  "true",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			// EphemeralDisk edited
			Old: &TaskGroup{
				EphemeralDisk: &EphemeralDisk{
					Migrate: true,
					Sticky:  true,
					SizeMB:  150,
				},
			},
			New: &TaskGroup{
				EphemeralDisk: &EphemeralDisk{
					Migrate: false,
					Sticky:  false,
					SizeMB:  90,
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "EphemeralDisk",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "Migrate",
								Old:  "true",
								New:  "false",
							},
							{
								Type: DiffTypeEdited,
								Name: "SizeMB",
								Old:  "150",
								New:  "90",
							},

							{
								Type: DiffTypeEdited,
								Name: "Sticky",
								Old:  "true",
								New:  "false",
							},
						},
					},
				},
			},
		},
		{
			// EphemeralDisk edited with context
			Contextual: true,
			Old: &TaskGroup{
				EphemeralDisk: &EphemeralDisk{
					Migrate: false,
					Sticky:  false,
					SizeMB:  100,
				},
			},
			New: &TaskGroup{
				EphemeralDisk: &EphemeralDisk{
					Migrate: true,
					Sticky:  true,
					SizeMB:  90,
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "EphemeralDisk",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "Migrate",
								Old:  "false",
								New:  "true",
							},
							{
								Type: DiffTypeEdited,
								Name: "SizeMB",
								Old:  "100",
								New:  "90",
							},
							{
								Type: DiffTypeEdited,
								Name: "Sticky",
								Old:  "false",
								New:  "true",
							},
						},
					},
				},
			},
		},
		{
			// Tasks edited
			Old: &TaskGroup{
				Tasks: []*Task{
					{
						Name:   "foo",
						Driver: "docker",
					},
					{
						Name:   "bar",
						Driver: "docker",
					},
					{
						Name:   "baz",
						Driver: "docker",
					},
				},
			},
			New: &TaskGroup{
				Tasks: []*Task{
					{
						Name:   "bar",
						Driver: "docker",
					},
					{
						Name:   "baz",
						Driver: "exec",
					},
					{
						Name:   "bam",
						Driver: "docker",
					},
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Tasks: []*TaskDiff{
					{
						Type: DiffTypeAdded,
						Name: "bam",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Driver",
								Old:  "",
								New:  "docker",
							},
							{
								Type: DiffTypeAdded,
								Name: "KillTimeout",
								Old:  "",
								New:  "0",
							},
							{
								Type: DiffTypeAdded,
								Name: "Leader",
								Old:  "",
								New:  "false",
							},
						},
					},
					{
						Type: DiffTypeNone,
						Name: "bar",
					},
					{
						Type: DiffTypeEdited,
						Name: "baz",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "Driver",
								Old:  "docker",
								New:  "exec",
							},
						},
					},
					{
						Type: DiffTypeDeleted,
						Name: "foo",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "Driver",
								Old:  "docker",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "KillTimeout",
								Old:  "0",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Leader",
								Old:  "false",
								New:  "",
							},
						},
					},
				},
			},
		},
	}

	for i, c := range cases {
		actual, err := c.Old.Diff(c.New, c.Contextual)
		if c.Error && err == nil {
			t.Fatalf("case %d: expected errored", i+1)
		} else if err != nil {
			if !c.Error {
				t.Fatalf("case %d: errored %#v", i+1, err)
			} else {
				continue
			}
		}

		if !reflect.DeepEqual(actual, c.Expected) {
			t.Fatalf("case %d: got:\n%#v\n want:\n%#v\n",
				i+1, actual, c.Expected)
		}
	}
}

func TestTaskDiff(t *testing.T) {
	cases := []struct {
		Name       string
		Old, New   *Task
		Expected   *TaskDiff
		Error      bool
		Contextual bool
	}{
		{
			Name: "Empty",
			Old:  nil,
			New:  nil,
			Expected: &TaskDiff{
				Type: DiffTypeNone,
			},
		},
		{
			Name: "Primitive only that has different names",
			Old: &Task{
				Name: "foo",
				Meta: map[string]string{
					"foo": "bar",
				},
			},
			New: &Task{
				Name: "bar",
				Meta: map[string]string{
					"foo": "bar",
				},
			},
			Error: true,
		},
		{
			Name: "Primitive only that is the same",
			Old: &Task{
				Name:   "foo",
				Driver: "exec",
				User:   "foo",
				Env: map[string]string{
					"FOO": "bar",
				},
				Meta: map[string]string{
					"foo": "bar",
				},
				KillTimeout: 1 * time.Second,
				Leader:      true,
			},
			New: &Task{
				Name:   "foo",
				Driver: "exec",
				User:   "foo",
				Env: map[string]string{
					"FOO": "bar",
				},
				Meta: map[string]string{
					"foo": "bar",
				},
				KillTimeout: 1 * time.Second,
				Leader:      true,
			},
			Expected: &TaskDiff{
				Type: DiffTypeNone,
				Name: "foo",
			},
		},
		{
			Name: "Primitive only that has diffs",
			Old: &Task{
				Name:   "foo",
				Driver: "exec",
				User:   "foo",
				Env: map[string]string{
					"FOO": "bar",
				},
				Meta: map[string]string{
					"foo": "bar",
				},
				KillTimeout: 1 * time.Second,
				Leader:      true,
			},
			New: &Task{
				Name:   "foo",
				Driver: "docker",
				User:   "bar",
				Env: map[string]string{
					"FOO": "baz",
				},
				Meta: map[string]string{
					"foo": "baz",
				},
				KillTimeout: 2 * time.Second,
				Leader:      false,
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Name: "foo",
				Fields: []*FieldDiff{
					{
						Type: DiffTypeEdited,
						Name: "Driver",
						Old:  "exec",
						New:  "docker",
					},
					{
						Type: DiffTypeEdited,
						Name: "Env[FOO]",
						Old:  "bar",
						New:  "baz",
					},
					{
						Type: DiffTypeEdited,
						Name: "KillTimeout",
						Old:  "1000000000",
						New:  "2000000000",
					},
					{
						Type: DiffTypeEdited,
						Name: "Leader",
						Old:  "true",
						New:  "false",
					},
					{
						Type: DiffTypeEdited,
						Name: "Meta[foo]",
						Old:  "bar",
						New:  "baz",
					},
					{
						Type: DiffTypeEdited,
						Name: "User",
						Old:  "foo",
						New:  "bar",
					},
				},
			},
		},
		{
			Name: "Map diff",
			Old: &Task{
				Meta: map[string]string{
					"foo": "foo",
					"bar": "bar",
				},
				Env: map[string]string{
					"foo": "foo",
					"bar": "bar",
				},
			},
			New: &Task{
				Meta: map[string]string{
					"bar": "bar",
					"baz": "baz",
				},
				Env: map[string]string{
					"bar": "bar",
					"baz": "baz",
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Fields: []*FieldDiff{
					{
						Type: DiffTypeAdded,
						Name: "Env[baz]",
						Old:  "",
						New:  "baz",
					},
					{
						Type: DiffTypeDeleted,
						Name: "Env[foo]",
						Old:  "foo",
						New:  "",
					},
					{
						Type: DiffTypeAdded,
						Name: "Meta[baz]",
						Old:  "",
						New:  "baz",
					},
					{
						Type: DiffTypeDeleted,
						Name: "Meta[foo]",
						Old:  "foo",
						New:  "",
					},
				},
			},
		},
		{
			Name: "Constraints edited",
			Old: &Task{
				Constraints: []*Constraint{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
						str:     "foo",
					},
					{
						LTarget: "bar",
						RTarget: "bar",
						Operand: "bar",
						str:     "bar",
					},
				},
			},
			New: &Task{
				Constraints: []*Constraint{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
						str:     "foo",
					},
					{
						LTarget: "baz",
						RTarget: "baz",
						Operand: "baz",
						str:     "baz",
					},
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Constraint",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "LTarget",
								Old:  "",
								New:  "baz",
							},
							{
								Type: DiffTypeAdded,
								Name: "Operand",
								Old:  "",
								New:  "baz",
							},
							{
								Type: DiffTypeAdded,
								Name: "RTarget",
								Old:  "",
								New:  "baz",
							},
						},
					},
					{
						Type: DiffTypeDeleted,
						Name: "Constraint",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "LTarget",
								Old:  "bar",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Operand",
								Old:  "bar",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "RTarget",
								Old:  "bar",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			Name: "LogConfig added",
			Old:  &Task{},
			New: &Task{
				LogConfig: &LogConfig{
					MaxFiles:      1,
					MaxFileSizeMB: 10,
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "LogConfig",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "MaxFileSizeMB",
								Old:  "",
								New:  "10",
							},
							{
								Type: DiffTypeAdded,
								Name: "MaxFiles",
								Old:  "",
								New:  "1",
							},
						},
					},
				},
			},
		},
		{
			Name: "LogConfig deleted",
			Old: &Task{
				LogConfig: &LogConfig{
					MaxFiles:      1,
					MaxFileSizeMB: 10,
				},
			},
			New: &Task{},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeDeleted,
						Name: "LogConfig",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "MaxFileSizeMB",
								Old:  "10",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "MaxFiles",
								Old:  "1",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			Name: "LogConfig edited",
			Old: &Task{
				LogConfig: &LogConfig{
					MaxFiles:      1,
					MaxFileSizeMB: 10,
				},
			},
			New: &Task{
				LogConfig: &LogConfig{
					MaxFiles:      2,
					MaxFileSizeMB: 20,
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "LogConfig",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "MaxFileSizeMB",
								Old:  "10",
								New:  "20",
							},
							{
								Type: DiffTypeEdited,
								Name: "MaxFiles",
								Old:  "1",
								New:  "2",
							},
						},
					},
				},
			},
		},
		{
			Name:       "LogConfig edited with context",
			Contextual: true,
			Old: &Task{
				LogConfig: &LogConfig{
					MaxFiles:      1,
					MaxFileSizeMB: 10,
				},
			},
			New: &Task{
				LogConfig: &LogConfig{
					MaxFiles:      1,
					MaxFileSizeMB: 20,
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "LogConfig",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "MaxFileSizeMB",
								Old:  "10",
								New:  "20",
							},
							{
								Type: DiffTypeNone,
								Name: "MaxFiles",
								Old:  "1",
								New:  "1",
							},
						},
					},
				},
			},
		},
		{
			Name: "Artifacts edited",
			Old: &Task{
				Artifacts: []*TaskArtifact{
					{
						GetterSource: "foo",
						GetterOptions: map[string]string{
							"foo": "bar",
						},
						RelativeDest: "foo",
					},
					{
						GetterSource: "bar",
						GetterOptions: map[string]string{
							"bar": "baz",
						},
						GetterMode:   "dir",
						RelativeDest: "bar",
					},
				},
			},
			New: &Task{
				Artifacts: []*TaskArtifact{
					{
						GetterSource: "foo",
						GetterOptions: map[string]string{
							"foo": "bar",
						},
						RelativeDest: "foo",
					},
					{
						GetterSource: "bam",
						GetterOptions: map[string]string{
							"bam": "baz",
						},
						GetterMode:   "file",
						RelativeDest: "bam",
					},
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Artifact",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "GetterMode",
								Old:  "",
								New:  "file",
							},
							{
								Type: DiffTypeAdded,
								Name: "GetterOptions[bam]",
								Old:  "",
								New:  "baz",
							},
							{
								Type: DiffTypeAdded,
								Name: "GetterSource",
								Old:  "",
								New:  "bam",
							},
							{
								Type: DiffTypeAdded,
								Name: "RelativeDest",
								Old:  "",
								New:  "bam",
							},
						},
					},
					{
						Type: DiffTypeDeleted,
						Name: "Artifact",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "GetterMode",
								Old:  "dir",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "GetterOptions[bar]",
								Old:  "baz",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "GetterSource",
								Old:  "bar",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "RelativeDest",
								Old:  "bar",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			Name: "Resources edited (no networks)",
			Old: &Task{
				Resources: &Resources{
					CPU:      100,
					MemoryMB: 100,
					DiskMB:   100,
					IOPS:     100,
				},
			},
			New: &Task{
				Resources: &Resources{
					CPU:      200,
					MemoryMB: 200,
					DiskMB:   200,
					IOPS:     200,
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Resources",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "CPU",
								Old:  "100",
								New:  "200",
							},
							{
								Type: DiffTypeEdited,
								Name: "DiskMB",
								Old:  "100",
								New:  "200",
							},
							{
								Type: DiffTypeEdited,
								Name: "IOPS",
								Old:  "100",
								New:  "200",
							},
							{
								Type: DiffTypeEdited,
								Name: "MemoryMB",
								Old:  "100",
								New:  "200",
							},
						},
					},
				},
			},
		},
		{
			Name:       "Resources edited (no networks) with context",
			Contextual: true,
			Old: &Task{
				Resources: &Resources{
					CPU:      100,
					MemoryMB: 100,
					DiskMB:   100,
					IOPS:     100,
				},
			},
			New: &Task{
				Resources: &Resources{
					CPU:      200,
					MemoryMB: 100,
					DiskMB:   200,
					IOPS:     100,
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Resources",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "CPU",
								Old:  "100",
								New:  "200",
							},
							{
								Type: DiffTypeEdited,
								Name: "DiskMB",
								Old:  "100",
								New:  "200",
							},
							{
								Type: DiffTypeNone,
								Name: "IOPS",
								Old:  "100",
								New:  "100",
							},
							{
								Type: DiffTypeNone,
								Name: "MemoryMB",
								Old:  "100",
								New:  "100",
							},
						},
					},
				},
			},
		},
		{
			Name: "Network Resources edited",
			Old: &Task{
				Resources: &Resources{
					Networks: []*NetworkResource{
						{
							Device: "foo",
							CIDR:   "foo",
							IP:     "foo",
							MBits:  100,
							ReservedPorts: []Port{
								{
									Label: "foo",
									Value: 80,
								},
							},
							DynamicPorts: []Port{
								{
									Label: "bar",
								},
							},
						},
					},
				},
			},
			New: &Task{
				Resources: &Resources{
					Networks: []*NetworkResource{
						{
							Device: "bar",
							CIDR:   "bar",
							IP:     "bar",
							MBits:  200,
							ReservedPorts: []Port{
								{
									Label: "foo",
									Value: 81,
								},
							},
							DynamicPorts: []Port{
								{
									Label: "baz",
								},
							},
						},
					},
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Resources",
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeAdded,
								Name: "Network",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "MBits",
										Old:  "",
										New:  "200",
									},
								},
								Objects: []*ObjectDiff{
									{
										Type: DiffTypeAdded,
										Name: "Static Port",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeAdded,
												Name: "Label",
												Old:  "",
												New:  "foo",
											},
											{
												Type: DiffTypeAdded,
												Name: "Value",
												Old:  "",
												New:  "81",
											},
										},
									},
									{
										Type: DiffTypeAdded,
										Name: "Dynamic Port",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeAdded,
												Name: "Label",
												Old:  "",
												New:  "baz",
											},
										},
									},
								},
							},
							{
								Type: DiffTypeDeleted,
								Name: "Network",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeDeleted,
										Name: "MBits",
										Old:  "100",
										New:  "",
									},
								},
								Objects: []*ObjectDiff{
									{
										Type: DiffTypeDeleted,
										Name: "Static Port",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeDeleted,
												Name: "Label",
												Old:  "foo",
												New:  "",
											},
											{
												Type: DiffTypeDeleted,
												Name: "Value",
												Old:  "80",
												New:  "",
											},
										},
									},
									{
										Type: DiffTypeDeleted,
										Name: "Dynamic Port",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeDeleted,
												Name: "Label",
												Old:  "bar",
												New:  "",
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
		{
			Name: "Config same",
			Old: &Task{
				Config: map[string]interface{}{
					"foo": 1,
					"bar": "bar",
					"bam": []string{"a", "b"},
					"baz": map[string]int{
						"a": 1,
						"b": 2,
					},
					"boom": &Port{
						Label: "boom_port",
					},
				},
			},
			New: &Task{
				Config: map[string]interface{}{
					"foo": 1,
					"bar": "bar",
					"bam": []string{"a", "b"},
					"baz": map[string]int{
						"a": 1,
						"b": 2,
					},
					"boom": &Port{
						Label: "boom_port",
					},
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeNone,
			},
		},
		{
			Name: "Config edited",
			Old: &Task{
				Config: map[string]interface{}{
					"foo": 1,
					"bar": "baz",
					"bam": []string{"a", "b"},
					"baz": map[string]int{
						"a": 1,
						"b": 2,
					},
					"boom": &Port{
						Label: "boom_port",
					},
				},
			},
			New: &Task{
				Config: map[string]interface{}{
					"foo": 2,
					"bar": "baz",
					"bam": []string{"a", "c", "d"},
					"baz": map[string]int{
						"b": 3,
						"c": 4,
					},
					"boom": &Port{
						Label: "boom_port2",
					},
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Config",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "bam[1]",
								Old:  "b",
								New:  "c",
							},
							{
								Type: DiffTypeAdded,
								Name: "bam[2]",
								Old:  "",
								New:  "d",
							},
							{
								Type: DiffTypeDeleted,
								Name: "baz[a]",
								Old:  "1",
								New:  "",
							},
							{
								Type: DiffTypeEdited,
								Name: "baz[b]",
								Old:  "2",
								New:  "3",
							},
							{
								Type: DiffTypeAdded,
								Name: "baz[c]",
								Old:  "",
								New:  "4",
							},
							{
								Type: DiffTypeEdited,
								Name: "boom.Label",
								Old:  "boom_port",
								New:  "boom_port2",
							},
							{
								Type: DiffTypeEdited,
								Name: "foo",
								Old:  "1",
								New:  "2",
							},
						},
					},
				},
			},
		},
		{
			Name:       "Config edited with context",
			Contextual: true,
			Old: &Task{
				Config: map[string]interface{}{
					"foo": 1,
					"bar": "baz",
					"bam": []string{"a", "b"},
					"baz": map[string]int{
						"a": 1,
						"b": 2,
					},
					"boom": &Port{
						Label: "boom_port",
					},
				},
			},
			New: &Task{
				Config: map[string]interface{}{
					"foo": 2,
					"bar": "baz",
					"bam": []string{"a", "c", "d"},
					"baz": map[string]int{
						"a": 1,
						"b": 2,
					},
					"boom": &Port{
						Label: "boom_port",
					},
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Config",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeNone,
								Name: "bam[0]",
								Old:  "a",
								New:  "a",
							},
							{
								Type: DiffTypeEdited,
								Name: "bam[1]",
								Old:  "b",
								New:  "c",
							},
							{
								Type: DiffTypeAdded,
								Name: "bam[2]",
								Old:  "",
								New:  "d",
							},
							{
								Type: DiffTypeNone,
								Name: "bar",
								Old:  "baz",
								New:  "baz",
							},
							{
								Type: DiffTypeNone,
								Name: "baz[a]",
								Old:  "1",
								New:  "1",
							},
							{
								Type: DiffTypeNone,
								Name: "baz[b]",
								Old:  "2",
								New:  "2",
							},
							{
								Type: DiffTypeNone,
								Name: "boom.Label",
								Old:  "boom_port",
								New:  "boom_port",
							},
							{
								Type: DiffTypeNone,
								Name: "boom.Value",
								Old:  "0",
								New:  "0",
							},
							{
								Type: DiffTypeEdited,
								Name: "foo",
								Old:  "1",
								New:  "2",
							},
						},
					},
				},
			},
		},
		{
			Name: "Services edited (no checks)",
			Old: &Task{
				Services: []*Service{
					{
						Name:      "foo",
						PortLabel: "foo",
					},
					{
						Name:      "bar",
						PortLabel: "bar",
					},
					{
						Name:      "baz",
						PortLabel: "baz",
					},
				},
			},
			New: &Task{
				Services: []*Service{
					{
						Name:      "bar",
						PortLabel: "bar",
					},
					{
						Name:      "baz",
						PortLabel: "baz2",
					},
					{
						Name:      "bam",
						PortLabel: "bam",
					},
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Service",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "PortLabel",
								Old:  "baz",
								New:  "baz2",
							},
						},
					},
					{
						Type: DiffTypeAdded,
						Name: "Service",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Name",
								Old:  "",
								New:  "bam",
							},
							{
								Type: DiffTypeAdded,
								Name: "PortLabel",
								Old:  "",
								New:  "bam",
							},
						},
					},
					{
						Type: DiffTypeDeleted,
						Name: "Service",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "Name",
								Old:  "foo",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "PortLabel",
								Old:  "foo",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			Name:       "Services edited (no checks) with context",
			Contextual: true,
			Old: &Task{
				Services: []*Service{
					{
						Name:      "foo",
						PortLabel: "foo",
					},
				},
			},
			New: &Task{
				Services: []*Service{
					{
						Name:        "foo",
						PortLabel:   "bar",
						AddressMode: "driver",
					},
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Service",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "AddressMode",
								New:  "driver",
							},
							{
								Type: DiffTypeNone,
								Name: "Name",
								Old:  "foo",
								New:  "foo",
							},
							{
								Type: DiffTypeEdited,
								Name: "PortLabel",
								Old:  "foo",
								New:  "bar",
							},
						},
					},
				},
			},
		},
		{
			Name: "Service Checks edited",
			Old: &Task{
				Services: []*Service{
					{
						Name: "foo",
						Checks: []*ServiceCheck{
							{
								Name:     "foo",
								Type:     "http",
								Command:  "foo",
								Args:     []string{"foo"},
								Path:     "foo",
								Protocol: "http",
								Interval: 1 * time.Second,
								Timeout:  1 * time.Second,
							},
							{
								Name:     "bar",
								Type:     "http",
								Command:  "foo",
								Args:     []string{"foo"},
								Path:     "foo",
								Protocol: "http",
								Interval: 1 * time.Second,
								Timeout:  1 * time.Second,
							},
							{
								Name:     "baz",
								Type:     "http",
								Command:  "foo",
								Args:     []string{"foo"},
								Path:     "foo",
								Protocol: "http",
								Interval: 1 * time.Second,
								Timeout:  1 * time.Second,
							},
						},
					},
				},
			},
			New: &Task{
				Services: []*Service{
					{
						Name: "foo",
						Checks: []*ServiceCheck{
							{
								Name:     "bar",
								Type:     "http",
								Command:  "foo",
								Args:     []string{"foo"},
								Path:     "foo",
								Protocol: "http",
								Interval: 1 * time.Second,
								Timeout:  1 * time.Second,
							},
							{
								Name:     "baz",
								Type:     "tcp",
								Command:  "foo",
								Args:     []string{"foo"},
								Path:     "foo",
								Protocol: "http",
								Interval: 1 * time.Second,
								Timeout:  1 * time.Second,
							},
							{
								Name:     "bam",
								Type:     "http",
								Command:  "foo",
								Args:     []string{"foo"},
								Path:     "foo",
								Protocol: "http",
								Interval: 1 * time.Second,
								Timeout:  1 * time.Second,
							},
						},
					},
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Service",
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeEdited,
								Name: "Check",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeEdited,
										Name: "Type",
										Old:  "http",
										New:  "tcp",
									},
								},
							},
							{
								Type: DiffTypeAdded,
								Name: "Check",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Command",
										Old:  "",
										New:  "foo",
									},
									{
										Type: DiffTypeAdded,
										Name: "Interval",
										Old:  "",
										New:  "1000000000",
									},
									{
										Type: DiffTypeAdded,
										Name: "Name",
										Old:  "",
										New:  "bam",
									},
									{
										Type: DiffTypeAdded,
										Name: "Path",
										Old:  "",
										New:  "foo",
									},
									{
										Type: DiffTypeAdded,
										Name: "Protocol",
										Old:  "",
										New:  "http",
									},
									{
										Type: DiffTypeAdded,
										Name: "TLSSkipVerify",
										Old:  "",
										New:  "false",
									},
									{
										Type: DiffTypeAdded,
										Name: "Timeout",
										Old:  "",
										New:  "1000000000",
									},
									{
										Type: DiffTypeAdded,
										Name: "Type",
										Old:  "",
										New:  "http",
									},
								},
							},
							{
								Type: DiffTypeDeleted,
								Name: "Check",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeDeleted,
										Name: "Command",
										Old:  "foo",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Interval",
										Old:  "1000000000",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Name",
										Old:  "foo",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Path",
										Old:  "foo",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Protocol",
										Old:  "http",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "TLSSkipVerify",
										Old:  "false",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Timeout",
										Old:  "1000000000",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Type",
										Old:  "http",
										New:  "",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name:       "Service Checks edited with context",
			Contextual: true,
			Old: &Task{
				Services: []*Service{
					{
						Name: "foo",
						Checks: []*ServiceCheck{
							{
								Name:          "foo",
								Type:          "http",
								Command:       "foo",
								Args:          []string{"foo"},
								Path:          "foo",
								Protocol:      "http",
								Interval:      1 * time.Second,
								Timeout:       1 * time.Second,
								InitialStatus: "critical",
							},
						},
					},
				},
			},
			New: &Task{
				Services: []*Service{
					{
						Name: "foo",
						Checks: []*ServiceCheck{
							{
								Name:          "foo",
								Type:          "tcp",
								Command:       "foo",
								Args:          []string{"foo"},
								Path:          "foo",
								Protocol:      "http",
								Interval:      1 * time.Second,
								Timeout:       1 * time.Second,
								InitialStatus: "passing",
							},
						},
					},
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Service",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeNone,
								Name: "AddressMode",
								Old:  "",
								New:  "",
							},
							{
								Type: DiffTypeNone,
								Name: "Name",
								Old:  "foo",
								New:  "foo",
							},
							{
								Type: DiffTypeNone,
								Name: "PortLabel",
								Old:  "",
								New:  "",
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeEdited,
								Name: "Check",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeNone,
										Name: "Command",
										Old:  "foo",
										New:  "foo",
									},
									{
										Type: DiffTypeEdited,
										Name: "InitialStatus",
										Old:  "critical",
										New:  "passing",
									},
									{
										Type: DiffTypeNone,
										Name: "Interval",
										Old:  "1000000000",
										New:  "1000000000",
									},
									{
										Type: DiffTypeNone,
										Name: "Name",
										Old:  "foo",
										New:  "foo",
									},
									{
										Type: DiffTypeNone,
										Name: "Path",
										Old:  "foo",
										New:  "foo",
									},
									{
										Type: DiffTypeNone,
										Name: "PortLabel",
										Old:  "",
										New:  "",
									},
									{
										Type: DiffTypeNone,
										Name: "Protocol",
										Old:  "http",
										New:  "http",
									},
									{
										Type: DiffTypeNone,
										Name: "TLSSkipVerify",
										Old:  "false",
										New:  "false",
									},
									{
										Type: DiffTypeNone,
										Name: "Timeout",
										Old:  "1000000000",
										New:  "1000000000",
									},
									{
										Type: DiffTypeEdited,
										Name: "Type",
										Old:  "http",
										New:  "tcp",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "Vault added",
			Old:  &Task{},
			New: &Task{
				Vault: &Vault{
					Policies:     []string{"foo", "bar"},
					Env:          true,
					ChangeMode:   "signal",
					ChangeSignal: "SIGUSR1",
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Vault",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "ChangeMode",
								Old:  "",
								New:  "signal",
							},
							{
								Type: DiffTypeAdded,
								Name: "ChangeSignal",
								Old:  "",
								New:  "SIGUSR1",
							},
							{
								Type: DiffTypeAdded,
								Name: "Env",
								Old:  "",
								New:  "true",
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeAdded,
								Name: "Policies",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Policies",
										Old:  "",
										New:  "bar",
									},
									{
										Type: DiffTypeAdded,
										Name: "Policies",
										Old:  "",
										New:  "foo",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "Vault deleted",
			Old: &Task{
				Vault: &Vault{
					Policies:     []string{"foo", "bar"},
					Env:          true,
					ChangeMode:   "signal",
					ChangeSignal: "SIGUSR1",
				},
			},
			New: &Task{},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeDeleted,
						Name: "Vault",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "ChangeMode",
								Old:  "signal",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "ChangeSignal",
								Old:  "SIGUSR1",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Env",
								Old:  "true",
								New:  "",
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeDeleted,
								Name: "Policies",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeDeleted,
										Name: "Policies",
										Old:  "bar",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Policies",
										Old:  "foo",
										New:  "",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "Vault edited",
			Old: &Task{
				Vault: &Vault{
					Policies:     []string{"foo", "bar"},
					Env:          true,
					ChangeMode:   "signal",
					ChangeSignal: "SIGUSR1",
				},
			},
			New: &Task{
				Vault: &Vault{
					Policies:     []string{"bar", "baz"},
					Env:          false,
					ChangeMode:   "restart",
					ChangeSignal: "foo",
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Vault",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "ChangeMode",
								Old:  "signal",
								New:  "restart",
							},
							{
								Type: DiffTypeEdited,
								Name: "ChangeSignal",
								Old:  "SIGUSR1",
								New:  "foo",
							},
							{
								Type: DiffTypeEdited,
								Name: "Env",
								Old:  "true",
								New:  "false",
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeEdited,
								Name: "Policies",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Policies",
										Old:  "",
										New:  "baz",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Policies",
										Old:  "foo",
										New:  "",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name:       "Vault edited with context",
			Contextual: true,
			Old: &Task{
				Vault: &Vault{
					Policies:     []string{"foo", "bar"},
					Env:          true,
					ChangeMode:   "signal",
					ChangeSignal: "SIGUSR1",
				},
			},
			New: &Task{
				Vault: &Vault{
					Policies:     []string{"bar", "baz"},
					Env:          true,
					ChangeMode:   "signal",
					ChangeSignal: "SIGUSR1",
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Vault",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeNone,
								Name: "ChangeMode",
								Old:  "signal",
								New:  "signal",
							},
							{
								Type: DiffTypeNone,
								Name: "ChangeSignal",
								Old:  "SIGUSR1",
								New:  "SIGUSR1",
							},
							{
								Type: DiffTypeNone,
								Name: "Env",
								Old:  "true",
								New:  "true",
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeEdited,
								Name: "Policies",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Policies",
										Old:  "",
										New:  "baz",
									},
									{
										Type: DiffTypeNone,
										Name: "Policies",
										Old:  "bar",
										New:  "bar",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Policies",
										Old:  "foo",
										New:  "",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "Template edited",
			Old: &Task{
				Templates: []*Template{
					{
						SourcePath:   "foo",
						DestPath:     "bar",
						EmbeddedTmpl: "baz",
						ChangeMode:   "bam",
						ChangeSignal: "SIGHUP",
						Splay:        1,
						Perms:        "0644",
					},
					{
						SourcePath:   "foo2",
						DestPath:     "bar2",
						EmbeddedTmpl: "baz2",
						ChangeMode:   "bam2",
						ChangeSignal: "SIGHUP2",
						Splay:        2,
						Perms:        "0666",
						Envvars:      true,
					},
				},
			},
			New: &Task{
				Templates: []*Template{
					{
						SourcePath:   "foo",
						DestPath:     "bar",
						EmbeddedTmpl: "baz",
						ChangeMode:   "bam",
						ChangeSignal: "SIGHUP",
						Splay:        1,
						Perms:        "0644",
					},
					{
						SourcePath:   "foo3",
						DestPath:     "bar3",
						EmbeddedTmpl: "baz3",
						ChangeMode:   "bam3",
						ChangeSignal: "SIGHUP3",
						Splay:        3,
						Perms:        "0776",
					},
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Template",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "ChangeMode",
								Old:  "",
								New:  "bam3",
							},
							{
								Type: DiffTypeAdded,
								Name: "ChangeSignal",
								Old:  "",
								New:  "SIGHUP3",
							},
							{
								Type: DiffTypeAdded,
								Name: "DestPath",
								Old:  "",
								New:  "bar3",
							},
							{
								Type: DiffTypeAdded,
								Name: "EmbeddedTmpl",
								Old:  "",
								New:  "baz3",
							},
							{
								Type: DiffTypeAdded,
								Name: "Envvars",
								Old:  "",
								New:  "false",
							},
							{
								Type: DiffTypeAdded,
								Name: "Perms",
								Old:  "",
								New:  "0776",
							},
							{
								Type: DiffTypeAdded,
								Name: "SourcePath",
								Old:  "",
								New:  "foo3",
							},
							{
								Type: DiffTypeAdded,
								Name: "Splay",
								Old:  "",
								New:  "3",
							},
						},
					},
					{
						Type: DiffTypeDeleted,
						Name: "Template",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "ChangeMode",
								Old:  "bam2",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "ChangeSignal",
								Old:  "SIGHUP2",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "DestPath",
								Old:  "bar2",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "EmbeddedTmpl",
								Old:  "baz2",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Envvars",
								Old:  "true",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Perms",
								Old:  "0666",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "SourcePath",
								Old:  "foo2",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Splay",
								Old:  "2",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			Name: "DispatchPayload added",
			Old:  &Task{},
			New: &Task{
				DispatchPayload: &DispatchPayloadConfig{
					File: "foo",
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "DispatchPayload",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "File",
								Old:  "",
								New:  "foo",
							},
						},
					},
				},
			},
		},
		{
			Name: "DispatchPayload deleted",
			Old: &Task{
				DispatchPayload: &DispatchPayloadConfig{
					File: "foo",
				},
			},
			New: &Task{},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeDeleted,
						Name: "DispatchPayload",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "File",
								Old:  "foo",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			Name: "Dispatch payload edited",
			Old: &Task{
				DispatchPayload: &DispatchPayloadConfig{
					File: "foo",
				},
			},
			New: &Task{
				DispatchPayload: &DispatchPayloadConfig{
					File: "bar",
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "DispatchPayload",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "File",
								Old:  "foo",
								New:  "bar",
							},
						},
					},
				},
			},
		},
		{
			// Place holder for if more fields are added
			Name:       "DispatchPayload edited with context",
			Contextual: true,
			Old: &Task{
				DispatchPayload: &DispatchPayloadConfig{
					File: "foo",
				},
			},
			New: &Task{
				DispatchPayload: &DispatchPayloadConfig{
					File: "bar",
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "DispatchPayload",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "File",
								Old:  "foo",
								New:  "bar",
							},
						},
					},
				},
			},
		},
	}

	for i, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			actual, err := c.Old.Diff(c.New, c.Contextual)
			if c.Error && err == nil {
				t.Fatalf("case %d: expected errored", i+1)
			} else if err != nil {
				if !c.Error {
					t.Fatalf("case %d: errored %#v", i+1, err)
				} else {
					return
				}
			}

			if !reflect.DeepEqual(actual, c.Expected) {
				t.Errorf("case %d: got:\n%#v\n want:\n%#v\n",
					i+1, actual, c.Expected)
			}
		})
	}
}
