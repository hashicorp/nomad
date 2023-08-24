// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/stretchr/testify/require"
)

func TestJobDiff(t *testing.T) {
	ci.Parallel(t)

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
						Name: "Dispatched",
						Old:  "false",
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
						Name: "Dispatched",
						Old:  "",
						New:  "false",
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
			// NodePool added
			Old: &Job{},
			New: &Job{
				NodePool: "default",
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Fields: []*FieldDiff{
					{
						Type: DiffTypeAdded,
						Name: "NodePool",
						Old:  "",
						New:  "default",
					},
				},
			},
		},
		{
			// NodePool removed
			Old: &Job{
				NodePool: "default",
			},
			New: &Job{},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Fields: []*FieldDiff{
					{
						Type: DiffTypeDeleted,
						Name: "NodePool",
						Old:  "default",
						New:  "",
					},
				},
			},
		},
		{
			// NodePool changed
			Old: &Job{
				NodePool: "default",
			},
			New: &Job{
				NodePool: "foo",
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Fields: []*FieldDiff{
					{
						Type: DiffTypeEdited,
						Name: "NodePool",
						Old:  "default",
						New:  "foo",
					},
				},
			},
		},
		{
			// NodePool unchanged
			Old: &Job{
				NodePool: "foo",
			},
			New: &Job{
				NodePool: "foo",
			},
			Expected: &JobDiff{
				Type: DiffTypeNone,
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
			// Periodic multiple times added
			Old: &Job{},
			New: &Job{
				Periodic: &PeriodicConfig{
					Enabled:         false,
					Specs:           []string{"*/15 * * * * *", "*/16 * * * * *"},
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
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeAdded,
								Name: "Specs",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Specs",
										Old:  "",
										New:  "*/15 * * * * *",
									},
									{
										Type: DiffTypeAdded,
										Name: "Specs",
										Old:  "",
										New:  "*/16 * * * * *",
									},
								},
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
			// Periodic single to multiple times
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
					Specs:           []string{"* * * * * *", "*/5 * * * * *"},
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
								Type: DiffTypeDeleted,
								Name: "Spec",
								Old:  "*/15 * * * * *",
								New:  "",
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
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeAdded,
								Name: "Specs",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Specs",
										Old:  "",
										New:  "* * * * * *",
									},
									{
										Type: DiffTypeAdded,
										Name: "Specs",
										Old:  "",
										New:  "*/5 * * * * *",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			// Periodic multiple times to single
			Old: &Job{
				Periodic: &PeriodicConfig{
					Enabled:         false,
					Specs:           []string{"* * * * * *", "*/5 * * * * *"},
					SpecType:        "foo",
					ProhibitOverlap: false,
					TimeZone:        "Europe/Minsk",
				},
			},
			New: &Job{
				Periodic: &PeriodicConfig{
					Enabled:         true,
					Spec:            "*/15 * * * * *",
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
								Type: DiffTypeAdded,
								Name: "Spec",
								Old:  "",
								New:  "*/15 * * * * *",
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
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeDeleted,
								Name: "Specs",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeDeleted,
										Name: "Specs",
										Old:  "* * * * * *",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Specs",
										Old:  "*/5 * * * * *",
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
			// Periodic edit multiple times
			Old: &Job{
				Periodic: &PeriodicConfig{
					Enabled:         false,
					Specs:           []string{"*/4 * * * * *", "*/6 * * * * *"},
					SpecType:        "foo",
					ProhibitOverlap: false,
					TimeZone:        "Europe/Minsk",
				},
			},
			New: &Job{
				Periodic: &PeriodicConfig{
					Enabled:         true,
					Specs:           []string{"*/5 * * * * *", "*/7 * * * * *"},
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
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeEdited,
								Name: "Specs",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Specs",
										Old:  "",
										New:  "*/5 * * * * *",
									},
									{
										Type: DiffTypeAdded,
										Name: "Specs",
										Old:  "",
										New:  "*/7 * * * * *",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Specs",
										Old:  "*/4 * * * * *",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Specs",
										Old:  "*/6 * * * * *",
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
					},
					{
						LTarget: "bar",
						RTarget: "bar",
						Operand: "bar",
					},
				},
			},
			New: &Job{
				Constraints: []*Constraint{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
					},
					{
						LTarget: "baz",
						RTarget: "baz",
						Operand: "baz",
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
			// Affinities edited
			Old: &Job{
				Affinities: []*Affinity{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
						Weight:  20,
					},
					{
						LTarget: "bar",
						RTarget: "bar",
						Operand: "bar",
						Weight:  20,
					},
				},
			},
			New: &Job{
				Affinities: []*Affinity{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
						Weight:  20,
					},
					{
						LTarget: "baz",
						RTarget: "baz",
						Operand: "baz",
						Weight:  20,
					},
				},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Affinity",
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
							{
								Type: DiffTypeAdded,
								Name: "Weight",
								Old:  "",
								New:  "20",
							},
						},
					},
					{
						Type: DiffTypeDeleted,
						Name: "Affinity",
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
							{
								Type: DiffTypeDeleted,
								Name: "Weight",
								Old:  "20",
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
		{
			// Multiregion: region added
			Old: &Job{
				NomadTokenID: "abcdef",
				Multiregion: &Multiregion{
					Strategy: &MultiregionStrategy{
						MaxParallel: 1,
						OnFailure:   "fail_all",
					},
					Regions: []*MultiregionRegion{
						{
							Name:        "west",
							Count:       1,
							Datacenters: []string{"west-1"},
							Meta:        map[string]string{"region_code": "W"},
						},
					},
				},
			},

			New: &Job{
				NomadTokenID: "12345",
				Multiregion: &Multiregion{
					Strategy: &MultiregionStrategy{
						MaxParallel: 2,
						OnFailure:   "fail_all",
					},
					Regions: []*MultiregionRegion{
						{
							Name:        "west",
							Count:       3,
							Datacenters: []string{"west-2"},
							Meta:        map[string]string{"region_code": "W"},
						},
						{
							Name:        "east",
							Count:       2,
							Datacenters: []string{"east-1", "east-2"},
							Meta:        map[string]string{"region_code": "E"},
						},
					},
				},
			},
			Expected: &JobDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Multiregion",
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeEdited,
								Name: "Region",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeEdited,
										Name: "Count",
										Old:  "1",
										New:  "3",
									},
								},
								Objects: []*ObjectDiff{
									{
										Type: DiffTypeEdited,
										Name: "Datacenters",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeAdded,
												Name: "Datacenters",
												Old:  "",
												New:  "west-2",
											},
											{
												Type: DiffTypeDeleted,
												Name: "Datacenters",
												Old:  "west-1",
												New:  "",
											},
										},
									},
								},
							},
							{
								Type: DiffTypeAdded,
								Name: "Region",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Count",
										Old:  "",
										New:  "2",
									},
									{
										Type: DiffTypeAdded,
										Name: "Meta[region_code]",
										Old:  "",
										New:  "E",
									},
									{
										Type: DiffTypeAdded,
										Name: "Name",
										Old:  "",
										New:  "east",
									},
								},

								Objects: []*ObjectDiff{
									{
										Type: DiffTypeAdded,
										Name: "Datacenters",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeAdded,
												Name: "Datacenters",
												Old:  "",
												New:  "east-1",
											},
											{
												Type: DiffTypeAdded,
												Name: "Datacenters",
												Old:  "",
												New:  "east-2",
											},
										},
									},
								},
							},
							{
								Type: DiffTypeEdited,
								Name: "Strategy",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeEdited,
										Name: "MaxParallel",
										Old:  "1",
										New:  "2",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			// VaultToken is filtered
			Old: &Job{
				ID:         "vault-job",
				VaultToken: "secret",
			},
			New: &Job{
				ID:         "vault-job",
				VaultToken: "new-secret",
			},
			Expected: &JobDiff{
				Type: DiffTypeNone,
				ID:   "vault-job",
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
	ci.Parallel(t)

	cases := []struct {
		TestCase   string
		Old, New   *TaskGroup
		Expected   *TaskGroupDiff
		ExpErr     bool
		Contextual bool
	}{
		{
			TestCase: "Empty",
			Old:      nil,
			New:      nil,
			Expected: &TaskGroupDiff{
				Type: DiffTypeNone,
			},
		},
		{
			TestCase: "Primitive only that has different names",
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
			ExpErr: true,
		},
		{
			TestCase: "Primitive only that is the same",
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
			TestCase: "Primitive only that has diffs",
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
			TestCase: "Map diff",
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
			TestCase: "Constraints edited",
			Old: &TaskGroup{
				Constraints: []*Constraint{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
					},
					{
						LTarget: "bar",
						RTarget: "bar",
						Operand: "bar",
					},
				},
			},
			New: &TaskGroup{
				Constraints: []*Constraint{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
					},
					{
						LTarget: "baz",
						RTarget: "baz",
						Operand: "baz",
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
			TestCase: "Affinities edited",
			Old: &TaskGroup{
				Affinities: []*Affinity{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
						Weight:  20,
					},
					{
						LTarget: "bar",
						RTarget: "bar",
						Operand: "bar",
						Weight:  20,
					},
				},
			},
			New: &TaskGroup{
				Affinities: []*Affinity{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
						Weight:  20,
					},
					{
						LTarget: "baz",
						RTarget: "baz",
						Operand: "baz",
						Weight:  20,
					},
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Affinity",
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
							{
								Type: DiffTypeAdded,
								Name: "Weight",
								Old:  "",
								New:  "20",
							},
						},
					},
					{
						Type: DiffTypeDeleted,
						Name: "Affinity",
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
							{
								Type: DiffTypeDeleted,
								Name: "Weight",
								Old:  "20",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			TestCase: "Consul added",
			Old:      &TaskGroup{},
			New: &TaskGroup{
				Consul: &Consul{
					Namespace: "team1",
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Consul",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Namespace",
								Old:  "",
								New:  "team1",
							},
						},
					},
				},
			},
		},
		{
			TestCase: "Consul deleted",
			Old: &TaskGroup{
				Consul: &Consul{
					Namespace: "team1",
				},
			},
			New: &TaskGroup{},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeDeleted,
						Name: "Consul",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "Namespace",
								Old:  "team1",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			TestCase: "Consul updated",
			Old: &TaskGroup{
				Consul: &Consul{
					Namespace: "team1",
				},
			},
			New: &TaskGroup{
				Consul: &Consul{
					Namespace: "team2",
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Consul",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "Namespace",
								Old:  "team1",
								New:  "team2",
							},
						},
					},
				},
			},
		},
		{
			TestCase: "RestartPolicy added",
			Old:      &TaskGroup{},
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
							{
								Type: DiffTypeAdded,
								Name: "RenderTemplates",
								Old:  "",
								New:  "false",
							},
						},
					},
				},
			},
		},
		{
			TestCase: "RestartPolicy deleted",
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
							{
								Type: DiffTypeDeleted,
								Name: "RenderTemplates",
								Old:  "false",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			TestCase: "RestartPolicy edited",
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
					Attempts:        2,
					Interval:        2 * time.Second,
					Delay:           2 * time.Second,
					Mode:            "delay",
					RenderTemplates: true,
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
							{
								Type: DiffTypeEdited,
								Name: "RenderTemplates",
								Old:  "false",
								New:  "true",
							},
						},
					},
				},
			},
		},
		{
			TestCase:   "RestartPolicy edited with context",
			Contextual: true,
			Old: &TaskGroup{
				RestartPolicy: &RestartPolicy{
					Attempts:        1,
					Interval:        1 * time.Second,
					Delay:           1 * time.Second,
					Mode:            "fail",
					RenderTemplates: false,
				},
			},
			New: &TaskGroup{
				RestartPolicy: &RestartPolicy{
					Attempts:        2,
					Interval:        2 * time.Second,
					Delay:           1 * time.Second,
					Mode:            "fail",
					RenderTemplates: true,
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
							{
								Type: DiffTypeEdited,
								Name: "RenderTemplates",
								Old:  "false",
								New:  "true",
							},
						},
					},
				},
			},
		},
		{
			TestCase: "ReschedulePolicy added",
			Old:      &TaskGroup{},
			New: &TaskGroup{
				ReschedulePolicy: &ReschedulePolicy{
					Attempts:      1,
					Interval:      15 * time.Second,
					Delay:         5 * time.Second,
					MaxDelay:      20 * time.Second,
					DelayFunction: "exponential",
					Unlimited:     false,
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "ReschedulePolicy",
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
								New:  "5000000000",
							},
							{
								Type: DiffTypeAdded,
								Name: "DelayFunction",
								Old:  "",
								New:  "exponential",
							},
							{
								Type: DiffTypeAdded,
								Name: "Interval",
								Old:  "",
								New:  "15000000000",
							},
							{
								Type: DiffTypeAdded,
								Name: "MaxDelay",
								Old:  "",
								New:  "20000000000",
							},
							{
								Type: DiffTypeAdded,
								Name: "Unlimited",
								Old:  "",
								New:  "false",
							},
						},
					},
				},
			},
		},
		{
			TestCase: "ReschedulePolicy deleted",
			Old: &TaskGroup{
				ReschedulePolicy: &ReschedulePolicy{
					Attempts:      1,
					Interval:      15 * time.Second,
					Delay:         5 * time.Second,
					MaxDelay:      20 * time.Second,
					DelayFunction: "exponential",
					Unlimited:     false,
				},
			},
			New: &TaskGroup{},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeDeleted,
						Name: "ReschedulePolicy",
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
								Old:  "5000000000",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "DelayFunction",
								Old:  "exponential",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Interval",
								Old:  "15000000000",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "MaxDelay",
								Old:  "20000000000",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Unlimited",
								Old:  "false",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			TestCase: "ReschedulePolicy edited",
			Old: &TaskGroup{
				ReschedulePolicy: &ReschedulePolicy{
					Attempts:      1,
					Interval:      1 * time.Second,
					DelayFunction: "exponential",
					Delay:         20 * time.Second,
					MaxDelay:      1 * time.Minute,
					Unlimited:     false,
				},
			},
			New: &TaskGroup{
				ReschedulePolicy: &ReschedulePolicy{
					Attempts:      2,
					Interval:      2 * time.Second,
					DelayFunction: "constant",
					Delay:         30 * time.Second,
					MaxDelay:      1 * time.Minute,
					Unlimited:     true,
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "ReschedulePolicy",
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
								Old:  "20000000000",
								New:  "30000000000",
							},
							{
								Type: DiffTypeEdited,
								Name: "DelayFunction",
								Old:  "exponential",
								New:  "constant",
							},
							{
								Type: DiffTypeEdited,
								Name: "Interval",
								Old:  "1000000000",
								New:  "2000000000",
							},
							{
								Type: DiffTypeEdited,
								Name: "Unlimited",
								Old:  "false",
								New:  "true",
							},
						},
					},
				},
			},
		},
		{
			TestCase:   "ReschedulePolicy edited with context",
			Contextual: true,
			Old: &TaskGroup{
				ReschedulePolicy: &ReschedulePolicy{
					Attempts: 1,
					Interval: 1 * time.Second,
				},
			},
			New: &TaskGroup{
				ReschedulePolicy: &ReschedulePolicy{
					Attempts: 1,
					Interval: 2 * time.Second,
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "ReschedulePolicy",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeNone,
								Name: "Attempts",
								Old:  "1",
								New:  "1",
							},
							{
								Type: DiffTypeNone,
								Name: "Delay",
								Old:  "0",
								New:  "0",
							},
							{
								Type: DiffTypeNone,
								Name: "DelayFunction",
								Old:  "",
								New:  "",
							},
							{
								Type: DiffTypeEdited,
								Name: "Interval",
								Old:  "1000000000",
								New:  "2000000000",
							},
							{
								Type: DiffTypeNone,
								Name: "MaxDelay",
								Old:  "0",
								New:  "0",
							},
							{
								Type: DiffTypeNone,
								Name: "Unlimited",
								Old:  "false",
								New:  "false",
							},
						},
					},
				},
			},
		},
		{
			TestCase: "Update strategy deleted",
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
								Name: "AutoPromote",
								Old:  "false",
								New:  "",
							},
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
							{
								Type: DiffTypeDeleted,
								Name: "ProgressDeadline",
								Old:  "0",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			TestCase: "Update strategy added",
			Old:      &TaskGroup{},
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
								Name: "AutoPromote",
								Old:  "",
								New:  "false",
							},
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
							{
								Type: DiffTypeAdded,
								Name: "ProgressDeadline",
								Old:  "",
								New:  "0",
							},
						},
					},
				},
			},
		},
		{
			TestCase: "Update strategy edited",
			Old: &TaskGroup{
				Update: &UpdateStrategy{
					MaxParallel:      5,
					HealthCheck:      "foo",
					MinHealthyTime:   1 * time.Second,
					HealthyDeadline:  30 * time.Second,
					ProgressDeadline: 29 * time.Second,
					AutoRevert:       true,
					AutoPromote:      true,
					Canary:           2,
				},
			},
			New: &TaskGroup{
				Update: &UpdateStrategy{
					MaxParallel:      7,
					HealthCheck:      "bar",
					MinHealthyTime:   2 * time.Second,
					HealthyDeadline:  31 * time.Second,
					ProgressDeadline: 32 * time.Second,
					AutoRevert:       false,
					AutoPromote:      false,
					Canary:           1,
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
								Name: "AutoPromote",
								Old:  "true",
								New:  "false",
							},
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
							{
								Type: DiffTypeEdited,
								Name: "ProgressDeadline",
								Old:  "29000000000",
								New:  "32000000000",
							},
						},
					},
				},
			},
		},
		{
			TestCase:   "Update strategy edited with context",
			Contextual: true,
			Old: &TaskGroup{
				Update: &UpdateStrategy{
					MaxParallel:      5,
					HealthCheck:      "foo",
					MinHealthyTime:   1 * time.Second,
					HealthyDeadline:  30 * time.Second,
					ProgressDeadline: 30 * time.Second,
					AutoRevert:       true,
					AutoPromote:      true,
					Canary:           2,
				},
			},
			New: &TaskGroup{
				Update: &UpdateStrategy{
					MaxParallel:      7,
					HealthCheck:      "foo",
					MinHealthyTime:   1 * time.Second,
					HealthyDeadline:  30 * time.Second,
					ProgressDeadline: 30 * time.Second,
					AutoRevert:       true,
					AutoPromote:      true,
					Canary:           2,
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
								Name: "AutoPromote",
								Old:  "true",
								New:  "true",
							},
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
							{
								Type: DiffTypeNone,
								Name: "ProgressDeadline",
								Old:  "30000000000",
								New:  "30000000000",
							},
						},
					},
				},
			},
		},
		{
			TestCase: "EphemeralDisk added",
			Old:      &TaskGroup{},
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
			TestCase: "EphemeralDisk deleted",
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
			TestCase: "EphemeralDisk edited",
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
			TestCase:   "EphemeralDisk edited with context",
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
			TestCase:   "TaskGroup Services edited",
			Contextual: true,
			Old: &TaskGroup{
				Services: []*Service{
					{
						Name:              "foo",
						Namespace:         "team1",
						TaskName:          "task1",
						EnableTagOverride: false,
						Checks: []*ServiceCheck{
							{
								Name:                   "foo",
								Type:                   "http",
								Command:                "foo",
								Args:                   []string{"foo"},
								Path:                   "foo",
								Protocol:               "http",
								Method:                 "POST",
								Body:                   "{\"key\": \"value\"}",
								Expose:                 true,
								Interval:               1 * time.Second,
								Timeout:                1 * time.Second,
								SuccessBeforePassing:   3,
								FailuresBeforeCritical: 4,
							},
						},
						Connect: &ConsulConnect{
							Native: false,
							SidecarTask: &SidecarTask{
								Name:   "sidecar",
								Driver: "docker",
								Env: map[string]string{
									"FOO": "BAR",
								},
								Config: map[string]interface{}{
									"foo": "baz",
								},
							},
							Gateway: &ConsulGateway{
								Proxy: &ConsulGatewayProxy{
									ConnectTimeout:                  pointer.Of(1 * time.Second),
									EnvoyGatewayBindTaggedAddresses: false,
									EnvoyGatewayBindAddresses: map[string]*ConsulGatewayBindAddress{
										"service1": {
											Address: "10.0.0.1",
											Port:    2001,
										},
									},
									EnvoyDNSDiscoveryType:     "STRICT_DNS",
									EnvoyGatewayNoDefaultBind: false,
									Config: map[string]interface{}{
										"foo": 1,
									},
								},
								Ingress: &ConsulIngressConfigEntry{
									TLS: &ConsulGatewayTLSConfig{
										Enabled: false,
									},
									Listeners: []*ConsulIngressListener{{
										Port:     3001,
										Protocol: "tcp",
										Services: []*ConsulIngressService{{
											Name: "listener1",
										}},
									}},
								},
								Terminating: &ConsulTerminatingConfigEntry{
									Services: []*ConsulLinkedService{{
										Name:     "linked1",
										CAFile:   "ca1.pem",
										CertFile: "cert1.pem",
										KeyFile:  "key1.pem",
										SNI:      "linked1.consul",
									}},
								},
								Mesh: &ConsulMeshConfigEntry{
									// nothing
								},
							},
						},
					},
				},
			},

			New: &TaskGroup{
				Services: []*Service{
					{
						Name:              "foo",
						Namespace:         "team1",
						TaskName:          "task2",
						EnableTagOverride: true,
						Checks: []*ServiceCheck{
							{
								Name:     "foo",
								Type:     "tcp",
								Command:  "bar",
								Path:     "bar",
								Protocol: "tcp",
								Expose:   false,
								Interval: 2 * time.Second,
								Timeout:  2 * time.Second,
								Header: map[string][]string{
									"Foo": {"baz"},
								},
								SuccessBeforePassing:   5,
								FailuresBeforeCritical: 6,
							},
						},
						Connect: &ConsulConnect{
							Native: true,
							SidecarService: &ConsulSidecarService{
								Port: "http",
								Proxy: &ConsulProxy{
									LocalServiceAddress: "127.0.0.1",
									LocalServicePort:    8080,
									Upstreams: []ConsulUpstream{
										{
											DestinationName:      "foo",
											DestinationNamespace: "ns2",
											LocalBindPort:        8000,
											Datacenter:           "dc2",
											LocalBindAddress:     "127.0.0.2",
											MeshGateway: ConsulMeshGateway{
												Mode: "remote",
											},
										},
									},
									Config: map[string]interface{}{
										"foo": "qux",
									},
								},
							},
							Gateway: &ConsulGateway{
								Proxy: &ConsulGatewayProxy{
									ConnectTimeout:                  pointer.Of(2 * time.Second),
									EnvoyGatewayBindTaggedAddresses: true,
									EnvoyGatewayBindAddresses: map[string]*ConsulGatewayBindAddress{
										"service1": {
											Address: "10.0.0.2",
											Port:    2002,
										},
									},
									EnvoyDNSDiscoveryType:     "LOGICAL_DNS",
									EnvoyGatewayNoDefaultBind: true,
									Config: map[string]interface{}{
										"foo": 2,
									},
								},
								Ingress: &ConsulIngressConfigEntry{
									TLS: &ConsulGatewayTLSConfig{
										Enabled: true,
									},
									Listeners: []*ConsulIngressListener{{
										Port:     3002,
										Protocol: "http",
										Services: []*ConsulIngressService{{
											Name:  "listener2",
											Hosts: []string{"127.0.0.1", "127.0.0.1:3002"},
										}},
									}},
								},
								Terminating: &ConsulTerminatingConfigEntry{
									Services: []*ConsulLinkedService{{
										Name:     "linked2",
										CAFile:   "ca2.pem",
										CertFile: "cert2.pem",
										KeyFile:  "key2.pem",
										SNI:      "linked2.consul",
									}},
								},
								Mesh: &ConsulMeshConfigEntry{
									// nothing
								},
							},
						},
					},
				},
			},

			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Service",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeNone,
								Name: "Address",
								Old:  "",
								New:  "",
							},
							{
								Type: DiffTypeNone,
								Name: "AddressMode",
								Old:  "",
								New:  "",
							},
							{
								Type: DiffTypeEdited,
								Name: "EnableTagOverride",
								Old:  "false",
								New:  "true",
							},
							{
								Type: DiffTypeNone,
								Name: "Name",
								Old:  "foo",
								New:  "foo",
							},
							{
								Type: DiffTypeNone,
								Name: "Namespace",
								Old:  "team1",
								New:  "team1",
							},
							{
								Type: DiffTypeNone,
								Name: "OnUpdate",
								Old:  "",
								New:  "",
							},
							{
								Type: DiffTypeNone,
								Name: "PortLabel",
								Old:  "",
								New:  "",
							},
							{
								Type: DiffTypeNone,
								Name: "Provider",
								Old:  "",
								New:  "",
							},
							{
								Type: DiffTypeEdited,
								Name: "TaskName",
								Old:  "task1",
								New:  "task2",
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeEdited,
								Name: "Check",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeNone,
										Name: "AddressMode",
										Old:  "",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Body",
										Old:  "{\"key\": \"value\"}",
										New:  "",
									},
									{
										Type: DiffTypeEdited,
										Name: "Command",
										Old:  "foo",
										New:  "bar",
									},
									{
										Type: DiffTypeEdited,
										Name: "Expose",
										Old:  "true",
										New:  "false",
									},
									{
										Type: DiffTypeEdited,
										Name: "FailuresBeforeCritical",
										Old:  "4",
										New:  "6",
									},
									{
										Type: DiffTypeNone,
										Name: "GRPCService",
										Old:  "",
										New:  "",
									},
									{
										Type: DiffTypeNone,
										Name: "GRPCUseTLS",
										Old:  "false",
										New:  "false",
									},
									{
										Type: DiffTypeNone,
										Name: "InitialStatus",
										Old:  "",
										New:  "",
									},
									{
										Type: DiffTypeEdited,
										Name: "Interval",
										Old:  "1000000000",
										New:  "2000000000",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Method",
										Old:  "POST",
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
										Name: "OnUpdate",
										Old:  "",
										New:  "",
									},
									{
										Type: DiffTypeEdited,
										Name: "Path",
										Old:  "foo",
										New:  "bar",
									},
									{
										Type: DiffTypeNone,
										Name: "PortLabel",
										Old:  "",
										New:  "",
									},
									{
										Type: DiffTypeEdited,
										Name: "Protocol",
										Old:  "http",
										New:  "tcp",
									},
									{
										Type: DiffTypeEdited,
										Name: "SuccessBeforePassing",
										Old:  "3",
										New:  "5",
									},
									{
										Type: DiffTypeNone,
										Name: "TLSServerName",
										Old:  "",
										New:  "",
									},
									{
										Type: DiffTypeNone,
										Name: "TLSSkipVerify",
										Old:  "false",
										New:  "false",
									},
									{
										Type: DiffTypeNone,
										Name: "TaskName",
										Old:  "",
										New:  "",
									},
									{
										Type: DiffTypeEdited,
										Name: "Timeout",
										Old:  "1000000000",
										New:  "2000000000",
									},
									{
										Type: DiffTypeEdited,
										Name: "Type",
										Old:  "http",
										New:  "tcp",
									},
								},
								Objects: []*ObjectDiff{
									{
										Type: DiffTypeAdded,
										Name: "Header",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeAdded,
												Name: "Foo[0]",
												Old:  "",
												New:  "baz",
											},
										},
									},
								},
							},
							{
								Type: DiffTypeEdited,
								Name: "ConsulConnect",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeEdited,
										Name: "Native",
										Old:  "false",
										New:  "true",
									},
								},
								Objects: []*ObjectDiff{

									{
										Type: DiffTypeAdded,
										Name: "SidecarService",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeAdded,
												Name: "DisableDefaultTCPCheck",
												Old:  "",
												New:  "false",
											},
											{
												Type: DiffTypeAdded,
												Name: "Port",
												Old:  "",
												New:  "http",
											},
										},
										Objects: []*ObjectDiff{
											{
												Type: DiffTypeAdded,
												Name: "ConsulProxy",
												Fields: []*FieldDiff{
													{
														Type: DiffTypeAdded,
														Name: "LocalServiceAddress",
														Old:  "",
														New:  "127.0.0.1",
													}, {
														Type: DiffTypeAdded,
														Name: "LocalServicePort",
														Old:  "",
														New:  "8080",
													},
												},
												Objects: []*ObjectDiff{
													{
														Type: DiffTypeAdded,
														Name: "ConsulUpstreams",
														Fields: []*FieldDiff{
															{
																Type: DiffTypeAdded,
																Name: "Datacenter",
																Old:  "",
																New:  "dc2",
															},
															{
																Type: DiffTypeAdded,
																Name: "DestinationName",
																Old:  "",
																New:  "foo",
															},
															{
																Type: DiffTypeAdded,
																Name: "DestinationNamespace",
																Old:  "",
																New:  "ns2",
															},
															{
																Type: DiffTypeAdded,
																Name: "LocalBindAddress",
																Old:  "",
																New:  "127.0.0.2",
															},
															{
																Type: DiffTypeAdded,
																Name: "LocalBindPort",
																Old:  "",
																New:  "8000",
															},
														},
														Objects: []*ObjectDiff{
															{
																Type: DiffTypeAdded,
																Name: "MeshGateway",
																Fields: []*FieldDiff{
																	{
																		Type: DiffTypeAdded,
																		Name: "Mode",
																		Old:  "",
																		New:  "remote",
																	},
																},
															},
														},
													},
													{
														Type: DiffTypeAdded,
														Name: "Config",
														Fields: []*FieldDiff{
															{
																Type: DiffTypeAdded,
																Name: "foo",
																Old:  "",
																New:  "qux",
															},
														},
													},
												},
											},
										},
									},

									{
										Type: DiffTypeDeleted,
										Name: "SidecarTask",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeDeleted,
												Name: "Driver",
												Old:  "docker",
												New:  "",
											},
											{
												Type: DiffTypeDeleted,
												Name: "Env[FOO]",
												Old:  "BAR",
												New:  "",
											},
											{
												Type: DiffTypeDeleted,
												Name: "Name",
												Old:  "sidecar",
												New:  "",
											},
										},
										Objects: []*ObjectDiff{
											{
												Type: DiffTypeDeleted,
												Name: "Config",
												Fields: []*FieldDiff{
													{
														Type: DiffTypeDeleted,
														Name: "foo",
														Old:  "baz",
														New:  "",
													},
												},
											},
										},
									},
									{
										Type: DiffTypeEdited,
										Name: "Gateway",
										Objects: []*ObjectDiff{
											{
												Type: DiffTypeEdited,
												Name: "Proxy",
												Fields: []*FieldDiff{
													{
														Type: DiffTypeEdited,
														Name: "ConnectTimeout",
														Old:  "1s",
														New:  "2s",
													},
													{
														Type: DiffTypeEdited,
														Name: "EnvoyDNSDiscoveryType",
														Old:  "STRICT_DNS",
														New:  "LOGICAL_DNS",
													},
													{
														Type: DiffTypeEdited,
														Name: "EnvoyGatewayBindTaggedAddresses",
														Old:  "false",
														New:  "true",
													},
													{
														Type: DiffTypeEdited,
														Name: "EnvoyGatewayNoDefaultBind",
														Old:  "false",
														New:  "true",
													},
												},
												Objects: []*ObjectDiff{
													{
														Type: DiffTypeEdited,
														Name: "EnvoyGatewayBindAddresses",
														Fields: []*FieldDiff{
															{
																Type: DiffTypeEdited,
																Name: "service1",
																Old:  "10.0.0.1:2001",
																New:  "10.0.0.2:2002",
															},
														},
													},
													{
														Type: DiffTypeEdited,
														Name: "Config",
														Fields: []*FieldDiff{
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
											{
												Type: DiffTypeEdited,
												Name: "Ingress",
												Objects: []*ObjectDiff{
													{
														Type: DiffTypeEdited,
														Name: "TLS",
														Fields: []*FieldDiff{
															{
																Type: DiffTypeEdited,
																Name: "Enabled",
																Old:  "false",
																New:  "true",
															},
															{
																Type: DiffTypeNone,
																Name: "TLSMaxVersion",
																Old:  "",
																New:  "",
															},
															{
																Type: DiffTypeNone,
																Name: "TLSMinVersion",
																Old:  "",
																New:  "",
															},
														},
													},
													{
														Type: DiffTypeAdded,
														Name: "Listener",
														Fields: []*FieldDiff{
															{
																Type: DiffTypeAdded,
																Name: "Port",
																Old:  "",
																New:  "3002",
															},
															{
																Type: DiffTypeAdded,
																Name: "Protocol",
																Old:  "",
																New:  "http",
															},
														},
														Objects: []*ObjectDiff{
															{
																Type: DiffTypeAdded,
																Name: "ConsulIngressService",
																Fields: []*FieldDiff{
																	{
																		Type: DiffTypeAdded,
																		Name: "Name",
																		Old:  "",
																		New:  "listener2",
																	},
																},
																Objects: []*ObjectDiff{
																	{
																		Type: DiffTypeAdded,
																		Name: "Hosts",
																		Fields: []*FieldDiff{
																			{
																				Type: DiffTypeAdded,
																				Name: "Hosts",
																				Old:  "",
																				New:  "127.0.0.1",
																			},
																			{
																				Type: DiffTypeAdded,
																				Name: "Hosts",
																				Old:  "",
																				New:  "127.0.0.1:3002",
																			},
																		},
																	},
																},
															},
														},
													},
													{
														Type: DiffTypeDeleted,
														Name: "Listener",
														Fields: []*FieldDiff{
															{
																Type: DiffTypeDeleted,
																Name: "Port",
																Old:  "3001",
																New:  "",
															},
															{
																Type: DiffTypeDeleted,
																Name: "Protocol",
																Old:  "tcp",
																New:  "",
															},
														},
														Objects: []*ObjectDiff{
															{
																Type: DiffTypeDeleted,
																Name: "ConsulIngressService",
																Fields: []*FieldDiff{
																	{
																		Type: DiffTypeDeleted,
																		Name: "Name",
																		Old:  "listener1",
																		New:  "",
																	},
																},
															},
														},
													},
												},
											},
											{
												Type: DiffTypeEdited,
												Name: "Terminating",
												Objects: []*ObjectDiff{
													{
														Type: DiffTypeAdded,
														Name: "Service",
														Fields: []*FieldDiff{
															{
																Type: DiffTypeAdded,
																Name: "CAFile",
																Old:  "",
																New:  "ca2.pem",
															},
															{
																Type: DiffTypeAdded,
																Name: "CertFile",
																Old:  "",
																New:  "cert2.pem",
															},
															{
																Type: DiffTypeAdded,
																Name: "KeyFile",
																Old:  "",
																New:  "key2.pem",
															},
															{
																Type: DiffTypeAdded,
																Name: "Name",
																Old:  "",
																New:  "linked2",
															},
															{
																Type: DiffTypeAdded,
																Name: "SNI",
																Old:  "",
																New:  "linked2.consul",
															},
														},
													},
													{
														Type: DiffTypeDeleted,
														Name: "Service",
														Fields: []*FieldDiff{
															{
																Type: DiffTypeDeleted,
																Name: "CAFile",
																Old:  "ca1.pem",
																New:  "",
															},
															{
																Type: DiffTypeDeleted,
																Name: "CertFile",
																Old:  "cert1.pem",
																New:  "",
															},
															{
																Type: DiffTypeDeleted,
																Name: "KeyFile",
																Old:  "key1.pem",
																New:  "",
															},
															{
																Type: DiffTypeDeleted,
																Name: "Name",
																Old:  "linked1",
																New:  "",
															},
															{
																Type: DiffTypeDeleted,
																Name: "SNI",
																Old:  "linked1.consul",
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
					},
				},
			},
		},
		{
			TestCase:   "TaskGroup Networks edited",
			Contextual: true,
			Old: &TaskGroup{
				Networks: Networks{
					{
						Device:   "foo",
						CIDR:     "foo",
						IP:       "foo",
						MBits:    100,
						Hostname: "foo",
						ReservedPorts: []Port{
							{
								Label: "foo",
								Value: 80,
							},
						},
					},
				},
			},
			New: &TaskGroup{
				Networks: Networks{
					{
						Device:   "bar",
						CIDR:     "bar",
						IP:       "bar",
						MBits:    200,
						Hostname: "bar",
						DynamicPorts: []Port{
							{
								Label:       "bar",
								To:          8081,
								HostNetwork: "public",
							},
						},
						DNS: &DNSConfig{
							Servers: []string{"1.1.1.1"},
						},
					},
				},
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Network",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Hostname",
								Old:  "",
								New:  "bar",
							},
							{
								Type: DiffTypeAdded,
								Name: "MBits",
								Old:  "",
								New:  "200",
							},
							{
								Type: DiffTypeNone,
								Name: "Mode",
								Old:  "",
								New:  "",
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeAdded,
								Name: "Dynamic Port",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "HostNetwork",
										Old:  "",
										New:  "public",
									},
									{
										Type: DiffTypeAdded,
										Name: "Label",
										Old:  "",
										New:  "bar",
									},
									{
										Type: DiffTypeAdded,
										Name: "To",
										Old:  "",
										New:  "8081",
									},
								},
							},
							{
								Type: DiffTypeAdded,
								Name: "DNS",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Servers",
										Old:  "",
										New:  "1.1.1.1",
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
								Name: "Hostname",
								Old:  "foo",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "MBits",
								Old:  "100",
								New:  "",
							},
							{
								Type: DiffTypeNone,
								Name: "Mode",
								Old:  "",
								New:  "",
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeDeleted,
								Name: "Static Port",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeNone,
										Name: "HostNetwork",
										Old:  "",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Label",
										Old:  "foo",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "To",
										Old:  "0",
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
						},
					},
				},
			},
		},
		{
			TestCase: "Tasks edited",
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
						Name:          "baz",
						ShutdownDelay: 1 * time.Second,
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
						Name:   "bam",
						Driver: "docker",
					},
					{
						Name:          "baz",
						ShutdownDelay: 2 * time.Second,
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
							{
								Type: DiffTypeAdded,
								Name: "ShutdownDelay",
								Old:  "",
								New:  "0",
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
								Name: "ShutdownDelay",
								Old:  "1000000000",
								New:  "2000000000",
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
							{
								Type: DiffTypeDeleted,
								Name: "ShutdownDelay",
								Old:  "0",
								New:  "",
							},
						},
					},
				},
			},
		},
		{
			TestCase: "TaskGroup shutdown_delay edited",
			Old: &TaskGroup{
				ShutdownDelay: pointer.Of(30 * time.Second),
			},
			New: &TaskGroup{
				ShutdownDelay: pointer.Of(5 * time.Second),
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Fields: []*FieldDiff{
					{
						Type: DiffTypeEdited,
						Name: "ShutdownDelay",
						Old:  "30000000000",
						New:  "5000000000",
					},
				},
			},
		},
		{
			TestCase: "TaskGroup shutdown_delay removed",
			Old: &TaskGroup{
				ShutdownDelay: pointer.Of(30 * time.Second),
			},
			New: &TaskGroup{},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Fields: []*FieldDiff{
					{
						Type: DiffTypeDeleted,
						Name: "ShutdownDelay",
						Old:  "30000000000",
						New:  "",
					},
				},
			},
		},
		{
			TestCase: "TaskGroup shutdown_delay added",
			Old:      &TaskGroup{},
			New: &TaskGroup{
				ShutdownDelay: pointer.Of(30 * time.Second),
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Fields: []*FieldDiff{
					{
						Type: DiffTypeAdded,
						Name: "ShutdownDelay",
						Old:  "",
						New:  "30000000000",
					},
				},
			},
		},

		{
			TestCase: "TaskGroup volumes added",
			Old:      &TaskGroup{},
			New: &TaskGroup{
				Volumes: map[string]*VolumeRequest{
					"foo": {
						Name:     "foo",
						Type:     "host",
						Source:   "foo-src",
						ReadOnly: true,
						PerAlloc: true,
					},
				},
			},

			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Volume",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Name",
								Old:  "",
								New:  "foo",
							},
							{
								Type: DiffTypeAdded,
								Name: "PerAlloc",
								Old:  "",
								New:  "true",
							},
							{
								Type: DiffTypeAdded,
								Name: "ReadOnly",
								Old:  "",
								New:  "true",
							},
							{
								Type: DiffTypeAdded,
								Name: "Source",
								Old:  "",
								New:  "foo-src",
							},
							{
								Type: DiffTypeAdded,
								Name: "Type",
								Old:  "",
								New:  "host",
							},
						},
					},
				},
			},
		},

		{
			TestCase: "TaskGroup volumes edited",
			Old: &TaskGroup{
				Volumes: map[string]*VolumeRequest{
					"foo": {
						Name:     "foo",
						Type:     "csi",
						Source:   "foo-src1",
						ReadOnly: false,
						MountOptions: &CSIMountOptions{
							FSType:     "ext4",
							MountFlags: []string{"relatime", "rw"},
						},
					},
					"bar": {
						Name:     "bar",
						Type:     "host",
						Source:   "bar-src",
						ReadOnly: true,
					},
				},
			},
			New: &TaskGroup{
				Volumes: map[string]*VolumeRequest{
					"foo": {
						Name:     "foo",
						Type:     "csi",
						Source:   "foo-src2",
						ReadOnly: true,
						MountOptions: &CSIMountOptions{
							FSType:     "ext4",
							MountFlags: []string{"relatime", "rw", "nosuid"},
						},
					},
					"bar": { // untouched
						Name:     "bar",
						Type:     "host",
						Source:   "bar-src",
						ReadOnly: true,
					},
				},
			},

			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Volume",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "ReadOnly",
								Old:  "false",
								New:  "true",
							},
							{
								Type: DiffTypeEdited,
								Name: "Source",
								Old:  "foo-src1",
								New:  "foo-src2",
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeEdited,
								Name: "MountOptions",
								Objects: []*ObjectDiff{
									{
										Type: DiffTypeAdded,
										Name: "MountFlags",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeAdded,
												Name: "MountFlags",
												Old:  "",
												New:  "nosuid",
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
			TestCase: "MaxClientDisconnect added",
			Old: &TaskGroup{
				Name:                "foo",
				MaxClientDisconnect: nil,
			},
			New: &TaskGroup{
				Name:                "foo",
				MaxClientDisconnect: pointer.Of(20 * time.Second),
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Name: "foo",
				Fields: []*FieldDiff{
					{
						Type: DiffTypeAdded,
						Name: "MaxClientDisconnect",
						Old:  "",
						New:  "20000000000",
					},
				},
			},
		},
		{
			TestCase: "MaxClientDisconnect updated",
			Old: &TaskGroup{
				Name:                "foo",
				MaxClientDisconnect: pointer.Of(10 * time.Second),
			},
			New: &TaskGroup{
				Name:                "foo",
				MaxClientDisconnect: pointer.Of(20 * time.Second),
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Name: "foo",
				Fields: []*FieldDiff{
					{
						Type: DiffTypeEdited,
						Name: "MaxClientDisconnect",
						Old:  "10000000000",
						New:  "20000000000",
					},
				},
			},
		},
		{
			TestCase: "MaxClientDisconnect deleted",
			Old: &TaskGroup{
				Name:                "foo",
				MaxClientDisconnect: pointer.Of(10 * time.Second),
			},
			New: &TaskGroup{
				Name:                "foo",
				MaxClientDisconnect: nil,
			},
			Expected: &TaskGroupDiff{
				Type: DiffTypeEdited,
				Name: "foo",
				Fields: []*FieldDiff{
					{
						Type: DiffTypeDeleted,
						Name: "MaxClientDisconnect",
						Old:  "10000000000",
						New:  "",
					},
				},
			},
		},
	}

	for i, c := range cases {
		require.NotEmpty(t, c.TestCase, "case #%d needs a name", i+1)

		t.Run(c.TestCase, func(t *testing.T) {
			result, err := c.Old.Diff(c.New, c.Contextual)
			switch c.ExpErr {
			case true:
				require.Error(t, err, "case %q expected error", c.TestCase)
			case false:
				require.NoError(t, err, "case %q expected no error", c.TestCase)
				require.Equal(t, c.Expected, result)
			}
		})
	}
}

func TestTaskDiff(t *testing.T) {
	ci.Parallel(t)

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
					},
					{
						LTarget: "bar",
						RTarget: "bar",
						Operand: "bar",
					},
				},
			},
			New: &Task{
				Constraints: []*Constraint{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
					},
					{
						LTarget: "baz",
						RTarget: "baz",
						Operand: "baz",
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
			Name: "Affinities edited",
			Old: &Task{
				Affinities: []*Affinity{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
						Weight:  20,
					},
					{
						LTarget: "bar",
						RTarget: "bar",
						Operand: "bar",
						Weight:  20,
					},
				},
			},
			New: &Task{
				Affinities: []*Affinity{
					{
						LTarget: "foo",
						RTarget: "foo",
						Operand: "foo",
						Weight:  20,
					},
					{
						LTarget: "baz",
						RTarget: "baz",
						Operand: "baz",
						Weight:  20,
					},
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Affinity",
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
							{
								Type: DiffTypeAdded,
								Name: "Weight",
								Old:  "",
								New:  "20",
							},
						},
					},
					{
						Type: DiffTypeDeleted,
						Name: "Affinity",
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
							{
								Type: DiffTypeDeleted,
								Name: "Weight",
								Old:  "20",
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
					Disabled:      true,
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
								Name: "Disabled",
								Old:  "",
								New:  "true",
							},
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
					Disabled:      true,
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
								Name: "Disabled",
								Old:  "true",
								New:  "",
							},
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
					Disabled:      false,
				},
			},
			New: &Task{
				LogConfig: &LogConfig{
					MaxFiles:      2,
					MaxFileSizeMB: 20,
					Disabled:      true,
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
								Name: "Disabled",
								Old:  "false",
								New:  "true",
							},
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
					Disabled:      false,
				},
			},
			New: &Task{
				LogConfig: &LogConfig{
					MaxFiles:      1,
					MaxFileSizeMB: 20,
					Disabled:      true,
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
								Name: "Disabled",
								Old:  "false",
								New:  "true",
							},
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
						GetterHeaders: map[string]string{
							"User": "user1",
						},
						GetterMode:   "dir",
						RelativeDest: "bar",
					},
				},
			},
			New: &Task{
				Artifacts: []*TaskArtifact{
					{
						GetterSource: "foo/bar",
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
						GetterHeaders: map[string]string{
							"User":       "user2",
							"User-Agent": "nomad",
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
						Type: DiffTypeEdited,
						Name: "Artifact",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "GetterSource",
								Old:  "foo",
								New:  "foo/bar",
							},
						},
					},
					{
						Type: DiffTypeAdded,
						Name: "Artifact",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "GetterHeaders[User-Agent]",
								Old:  "",
								New:  "nomad",
							},
							{
								Type: DiffTypeAdded,
								Name: "GetterHeaders[User]",
								Old:  "",
								New:  "user2",
							},
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
								Name: "GetterHeaders[User]",
								Old:  "user1",
								New:  "",
							},
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
				},
			},
			New: &Task{
				Resources: &Resources{
					CPU:      200,
					MemoryMB: 200,
					DiskMB:   200,
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
				},
			},
			New: &Task{
				Resources: &Resources{
					CPU:      200,
					MemoryMB: 100,
					DiskMB:   200,
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
								Type: DiffTypeNone,
								Name: "Cores",
								Old:  "0",
								New:  "0",
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
								Old:  "0",
								New:  "0",
							},
							{
								Type: DiffTypeNone,
								Name: "MemoryMB",
								Old:  "100",
								New:  "100",
							},
							{
								Type: DiffTypeNone,
								Name: "MemoryMaxMB",
								Old:  "0",
								New:  "0",
							},
						},
					},
				},
			},
		},
		{
			Name: "Resources edited memory_max",
			Old: &Task{
				Resources: &Resources{
					CPU:         100,
					MemoryMB:    100,
					MemoryMaxMB: 200,
					DiskMB:      100,
				},
			},
			New: &Task{
				Resources: &Resources{
					CPU:         100,
					MemoryMB:    100,
					MemoryMaxMB: 300,
					DiskMB:      100,
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
								Name: "MemoryMaxMB",
								Old:  "200",
								New:  "300",
							},
						},
					},
				},
			},
		},
		{
			Name:       "Resources edited memory_max with context",
			Contextual: true,
			Old: &Task{
				Resources: &Resources{
					CPU:         100,
					MemoryMB:    100,
					MemoryMaxMB: 200,
					DiskMB:      100,
				},
			},
			New: &Task{
				Resources: &Resources{
					CPU:         100,
					MemoryMB:    100,
					MemoryMaxMB: 300,
					DiskMB:      100,
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
								Type: DiffTypeNone,
								Name: "CPU",
								Old:  "100",
								New:  "100",
							},
							{
								Type: DiffTypeNone,
								Name: "Cores",
								Old:  "0",
								New:  "0",
							},
							{
								Type: DiffTypeNone,
								Name: "DiskMB",
								Old:  "100",
								New:  "100",
							},
							{
								Type: DiffTypeNone,
								Name: "IOPS",
								Old:  "0",
								New:  "0",
							},
							{
								Type: DiffTypeNone,
								Name: "MemoryMB",
								Old:  "100",
								New:  "100",
							},
							{
								Type: DiffTypeEdited,
								Name: "MemoryMaxMB",
								Old:  "200",
								New:  "300",
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
									To:    8080,
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
									To:    8081,
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
												Name: "To",
												Old:  "",
												New:  "0",
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
											{
												Type: DiffTypeAdded,
												Name: "To",
												Old:  "",
												New:  "8081",
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
												Name: "To",
												Old:  "0",
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
											{
												Type: DiffTypeDeleted,
												Name: "To",
												Old:  "8080",
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
			Name: "Device Resources edited",
			Old: &Task{
				Resources: &Resources{
					Devices: []*RequestedDevice{
						{
							Name:  "foo",
							Count: 2,
						},
						{
							Name:  "bar",
							Count: 2,
						},
						{
							Name:  "baz",
							Count: 2,
						},
					},
				},
			},
			New: &Task{
				Resources: &Resources{
					Devices: []*RequestedDevice{
						{
							Name:  "foo",
							Count: 2,
						},
						{
							Name:  "bar",
							Count: 3,
						},
						{
							Name:  "bam",
							Count: 2,
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
								Type: DiffTypeEdited,
								Name: "Device",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeEdited,
										Name: "Count",
										Old:  "2",
										New:  "3",
									},
								},
							},
							{
								Type: DiffTypeAdded,
								Name: "Device",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Count",
										Old:  "",
										New:  "2",
									},
									{
										Type: DiffTypeAdded,
										Name: "Name",
										Old:  "",
										New:  "bam",
									},
								},
							},
							{
								Type: DiffTypeDeleted,
								Name: "Device",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeDeleted,
										Name: "Count",
										Old:  "2",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Name",
										Old:  "baz",
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
			Name:       "Device Resources edited with context",
			Contextual: true,
			Old: &Task{
				Resources: &Resources{
					CPU:      100,
					MemoryMB: 100,
					DiskMB:   100,
					Devices: []*RequestedDevice{
						{
							Name:  "foo",
							Count: 2,
						},
						{
							Name:  "bar",
							Count: 2,
						},
						{
							Name:  "baz",
							Count: 2,
						},
					},
				},
			},
			New: &Task{
				Resources: &Resources{
					CPU:      100,
					MemoryMB: 100,
					DiskMB:   100,
					Devices: []*RequestedDevice{
						{
							Name:  "foo",
							Count: 2,
						},
						{
							Name:  "bar",
							Count: 3,
						},
						{
							Name:  "bam",
							Count: 2,
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
						Fields: []*FieldDiff{
							{
								Type: DiffTypeNone,
								Name: "CPU",
								Old:  "100",
								New:  "100",
							},
							{
								Type: DiffTypeNone,
								Name: "Cores",
								Old:  "0",
								New:  "0",
							},
							{
								Type: DiffTypeNone,
								Name: "DiskMB",
								Old:  "100",
								New:  "100",
							},
							{
								Type: DiffTypeNone,
								Name: "IOPS",
								Old:  "0",
								New:  "0",
							},
							{
								Type: DiffTypeNone,
								Name: "MemoryMB",
								Old:  "100",
								New:  "100",
							},
							{
								Type: DiffTypeNone,
								Name: "MemoryMaxMB",
								Old:  "0",
								New:  "0",
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeEdited,
								Name: "Device",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeEdited,
										Name: "Count",
										Old:  "2",
										New:  "3",
									},
									{
										Type: DiffTypeNone,
										Name: "Name",
										Old:  "bar",
										New:  "bar",
									},
								},
							},
							{
								Type: DiffTypeAdded,
								Name: "Device",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Count",
										Old:  "",
										New:  "2",
									},
									{
										Type: DiffTypeAdded,
										Name: "Name",
										Old:  "",
										New:  "bam",
									},
								},
							},
							{
								Type: DiffTypeDeleted,
								Name: "Device",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeDeleted,
										Name: "Count",
										Old:  "2",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Name",
										Old:  "baz",
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
								Name: "boom.HostNetwork",
								Old:  "",
								New:  "",
							},
							{
								Type: DiffTypeNone,
								Name: "boom.Label",
								Old:  "boom_port",
								New:  "boom_port",
							},
							{
								Type: DiffTypeNone,
								Name: "boom.To",
								Old:  "0",
								New:  "0",
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
								Name: "EnableTagOverride",
								Old:  "",
								New:  "false",
							},
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
								Name: "EnableTagOverride",
								Old:  "false",
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
						Address:     "a.example.com",
						TaskName:    "task1",
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
								Name: "Address",
								Old:  "",
								New:  "a.example.com",
							},
							{
								Type: DiffTypeAdded,
								Name: "AddressMode",
								Old:  "",
								New:  "driver",
							},
							{
								Type: DiffTypeNone,
								Name: "EnableTagOverride",
								Old:  "false",
								New:  "false",
							},
							{
								Type: DiffTypeNone,
								Name: "Name",
								Old:  "foo",
								New:  "foo",
							},
							{
								Type: DiffTypeNone,
								Name: "Namespace",
								Old:  "",
								New:  "",
							},
							{
								Type: DiffTypeNone,
								Name: "OnUpdate",
								Old:  "",
								New:  "",
							},
							{
								Type: DiffTypeEdited,
								Name: "PortLabel",
								Old:  "foo",
								New:  "bar",
							},
							{
								Type: DiffTypeNone,
								Name: "Provider",
							},
							{
								Type: DiffTypeAdded,
								Name: "TaskName",
								Old:  "",
								New:  "task1",
							},
						},
					},
				},
			},
		},
		{
			Name:       "Service EnableTagOverride edited no context",
			Contextual: false,
			Old: &Task{
				Services: []*Service{{
					EnableTagOverride: false,
				}},
			},
			New: &Task{
				Services: []*Service{{
					EnableTagOverride: true,
				}},
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
								Name: "EnableTagOverride",
								Old:  "false",
								New:  "true",
							},
						},
					},
				},
			},
		},
		{
			Name:       "Services tags edited (no checks) with context",
			Contextual: true,
			Old: &Task{
				Services: []*Service{
					{
						Tags:       []string{"foo", "bar"},
						CanaryTags: []string{"foo", "bar"},
					},
				},
			},
			New: &Task{
				Services: []*Service{
					{
						Tags:       []string{"bar", "bam"},
						CanaryTags: []string{"bar", "bam"},
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
								Name: "CanaryTags",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "CanaryTags",
										Old:  "",
										New:  "bam",
									},
									{
										Type: DiffTypeNone,
										Name: "CanaryTags",
										Old:  "bar",
										New:  "bar",
									},
									{
										Type: DiffTypeDeleted,
										Name: "CanaryTags",
										Old:  "foo",
										New:  "",
									},
								},
							},
							{
								Type: DiffTypeEdited,
								Name: "Tags",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Tags",
										Old:  "",
										New:  "bam",
									},
									{
										Type: DiffTypeNone,
										Name: "Tags",
										Old:  "bar",
										New:  "bar",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Tags",
										Old:  "foo",
										New:  "",
									},
								},
							},
						},
						Fields: []*FieldDiff{
							{
								Type: DiffTypeNone,
								Name: "Address",
							},
							{
								Type: DiffTypeNone,
								Name: "AddressMode",
							},
							{
								Type: DiffTypeNone,
								Name: "EnableTagOverride",
								Old:  "false",
								New:  "false",
							},
							{
								Type: DiffTypeNone,
								Name: "Name",
							},
							{
								Type: DiffTypeNone,
								Name: "Namespace",
							},
							{
								Type: DiffTypeNone,
								Name: "OnUpdate",
							},
							{
								Type: DiffTypeNone,
								Name: "PortLabel",
							},
							{
								Type: DiffTypeNone,
								Name: "Provider",
							},
							{
								Type: DiffTypeNone,
								Name: "TaskName",
							},
						},
					},
				},
			},
		},

		{
			Name: "Service with Connect",
			Old: &Task{
				Services: []*Service{
					{
						Name: "foo",
					},
				},
			},
			New: &Task{
				Services: []*Service{
					{
						Name: "foo",
						Connect: &ConsulConnect{
							SidecarService: &ConsulSidecarService{},
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
								Type: DiffTypeAdded,
								Name: "ConsulConnect",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Native",
										Old:  "",
										New:  "false",
									},
								},
								Objects: []*ObjectDiff{
									{
										Type: DiffTypeAdded,
										Name: "SidecarService",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeAdded,
												Name: "DisableDefaultTCPCheck",
												Old:  "",
												New:  "false",
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
			Name: "Service with Connect Native",
			Old: &Task{
				Services: []*Service{
					{
						Name: "foo",
					},
				},
			},
			New: &Task{
				Services: []*Service{
					{
						Name:     "foo",
						TaskName: "task1",
						Connect: &ConsulConnect{
							Native: true,
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
								Type: DiffTypeAdded,
								Name: "TaskName",
								Old:  "",
								New:  "task1",
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeAdded,
								Name: "ConsulConnect",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Native",
										Old:  "",
										New:  "true",
									},
								},
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
								Header: map[string][]string{
									"Foo": {"bar"},
								},
								SuccessBeforePassing:   1,
								FailuresBeforeCritical: 1,
							},
							{
								Name:                   "bar",
								Type:                   "http",
								Command:                "foo",
								Args:                   []string{"foo"},
								Path:                   "foo",
								Protocol:               "http",
								Interval:               1 * time.Second,
								Timeout:                1 * time.Second,
								SuccessBeforePassing:   7,
								FailuresBeforeCritical: 7,
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
								Name:                   "bar",
								Type:                   "http",
								Command:                "foo",
								Args:                   []string{"foo"},
								Path:                   "foo",
								Protocol:               "http",
								Interval:               1 * time.Second,
								Timeout:                1 * time.Second,
								SuccessBeforePassing:   7,
								FailuresBeforeCritical: 7,
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
								Header: map[string][]string{
									"Eggs": {"spam"},
								},
							},
							{
								Name:                   "bam",
								Type:                   "http",
								Command:                "foo",
								Args:                   []string{"foo"},
								Path:                   "foo",
								Protocol:               "http",
								Interval:               1 * time.Second,
								Timeout:                1 * time.Second,
								SuccessBeforePassing:   2,
								FailuresBeforeCritical: 2,
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
								Objects: []*ObjectDiff{
									{
										Type: DiffTypeAdded,
										Name: "Header",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeAdded,
												Name: "Eggs[0]",
												Old:  "",
												New:  "spam",
											},
										},
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
										Name: "Expose",
										Old:  "",
										New:  "false",
									},
									{
										Type: DiffTypeAdded,
										Name: "FailuresBeforeCritical",
										Old:  "",
										New:  "2",
									},
									{
										Type: DiffTypeAdded,
										Name: "GRPCUseTLS",
										Old:  "",
										New:  "false",
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
										Name: "SuccessBeforePassing",
										Old:  "",
										New:  "2",
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
										Name: "Expose",
										Old:  "false",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "FailuresBeforeCritical",
										Old:  "1",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "GRPCUseTLS",
										Old:  "false",
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
										Name: "SuccessBeforePassing",
										Old:  "1",
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
								Objects: []*ObjectDiff{
									{
										Type: DiffTypeDeleted,
										Name: "Header",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeDeleted,
												Name: "Foo[0]",
												Old:  "bar",
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
								Header: map[string][]string{
									"Foo": {"bar"},
								},
								SuccessBeforePassing:   4,
								FailuresBeforeCritical: 5,
								OnUpdate:               "require_healthy",
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
								Method:        "POST",
								Header: map[string][]string{
									"Foo":  {"bar", "baz"},
									"Eggs": {"spam"},
								},
								SuccessBeforePassing: 4,
								OnUpdate:             "ignore_warnings",
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
								Name: "Address",
								Old:  "",
								New:  "",
							},
							{
								Type: DiffTypeNone,
								Name: "AddressMode",
								Old:  "",
								New:  "",
							},
							{
								Type: DiffTypeNone,
								Name: "EnableTagOverride",
								Old:  "false",
								New:  "false",
							},
							{
								Type: DiffTypeNone,
								Name: "Name",
								Old:  "foo",
								New:  "foo",
							},
							{
								Type: DiffTypeNone,
								Name: "Namespace",
								Old:  "",
								New:  "",
							},
							{
								Type: DiffTypeNone,
								Name: "OnUpdate",
							},
							{
								Type: DiffTypeNone,
								Name: "PortLabel",
								Old:  "",
								New:  "",
							},
							{
								Type: DiffTypeNone,
								Name: "Provider",
							},
							{
								Type: DiffTypeNone,
								Name: "TaskName",
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
										Name: "AddressMode",
										Old:  "",
										New:  "",
									},
									{
										Type: DiffTypeNone,
										Name: "Body",
										Old:  "",
										New:  "",
									},
									{
										Type: DiffTypeNone,
										Name: "Command",
										Old:  "foo",
										New:  "foo",
									},
									{
										Type: DiffTypeNone,
										Name: "Expose",
										Old:  "false",
										New:  "false",
									},
									{
										Type: DiffTypeEdited,
										Name: "FailuresBeforeCritical",
										Old:  "5",
										New:  "0",
									},
									{
										Type: DiffTypeNone,
										Name: "GRPCService",
										Old:  "",
										New:  "",
									},
									{
										Type: DiffTypeNone,
										Name: "GRPCUseTLS",
										Old:  "false",
										New:  "false",
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
										Type: DiffTypeAdded,
										Name: "Method",
										Old:  "",
										New:  "POST",
									},
									{
										Type: DiffTypeNone,
										Name: "Name",
										Old:  "foo",
										New:  "foo",
									},
									{
										Type: DiffTypeEdited,
										Name: "OnUpdate",
										Old:  "require_healthy",
										New:  "ignore_warnings",
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
										Name: "SuccessBeforePassing",
										Old:  "4",
										New:  "4",
									},
									{
										Type: DiffTypeNone,
										Name: "TLSServerName",
										Old:  "",
										New:  "",
									},
									{
										Type: DiffTypeNone,
										Name: "TLSSkipVerify",
										Old:  "false",
										New:  "false",
									},
									{
										Type: DiffTypeNone,
										Name: "TaskName",
										Old:  "",
										New:  "",
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
								Objects: []*ObjectDiff{
									{
										Type: DiffTypeEdited,
										Name: "Header",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeAdded,
												Name: "Eggs[0]",
												Old:  "",
												New:  "spam",
											},
											{
												Type: DiffTypeNone,
												Name: "Foo[0]",
												Old:  "bar",
												New:  "bar",
											},
											{
												Type: DiffTypeAdded,
												Name: "Foo[1]",
												Old:  "",
												New:  "baz",
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
			Name: "CheckRestart edited",
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
								CheckRestart: &CheckRestart{
									Limit: 2,
									Grace: 2 * time.Second,
								},
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
								CheckRestart: &CheckRestart{
									Limit: 3,
									Grace: 3 * time.Second,
								},
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
								Name:     "foo",
								Type:     "http",
								Command:  "foo",
								Args:     []string{"foo"},
								Path:     "foo",
								Protocol: "http",
								Interval: 1 * time.Second,
								Timeout:  1 * time.Second,
								CheckRestart: &CheckRestart{
									Limit: 1,
									Grace: 1 * time.Second,
								},
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
								CheckRestart: &CheckRestart{
									Limit: 4,
									Grace: 4 * time.Second,
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
						Name: "Service",
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeEdited,
								Name: "Check",
								Objects: []*ObjectDiff{
									{
										Type: DiffTypeEdited,
										Name: "CheckRestart",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeEdited,
												Name: "Grace",
												Old:  "3000000000",
												New:  "4000000000",
											},
											{
												Type: DiffTypeEdited,
												Name: "Limit",
												Old:  "3",
												New:  "4",
											},
										},
									},
								},
							},
							{
								Type: DiffTypeEdited,
								Name: "Check",
								Objects: []*ObjectDiff{
									{
										Type: DiffTypeAdded,
										Name: "CheckRestart",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeAdded,
												Name: "Grace",
												New:  "1000000000",
											},
											{
												Type: DiffTypeAdded,
												Name: "IgnoreWarnings",
												New:  "false",
											},
											{
												Type: DiffTypeAdded,
												Name: "Limit",
												New:  "1",
											},
										},
									},
								},
							},
							{
								Type: DiffTypeEdited,
								Name: "Check",
								Objects: []*ObjectDiff{
									{
										Type: DiffTypeDeleted,
										Name: "CheckRestart",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeDeleted,
												Name: "Grace",
												Old:  "2000000000",
											},
											{
												Type: DiffTypeDeleted,
												Name: "IgnoreWarnings",
												Old:  "false",
											},
											{
												Type: DiffTypeDeleted,
												Name: "Limit",
												Old:  "2",
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
			Name: "Vault added",
			Old:  &Task{},
			New: &Task{
				Vault: &Vault{
					Role:         "nomad-task",
					Policies:     []string{"foo", "bar"},
					Env:          true,
					DisableFile:  true,
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
								Name: "DisableFile",
								Old:  "",
								New:  "true",
							},
							{
								Type: DiffTypeAdded,
								Name: "Env",
								Old:  "",
								New:  "true",
							},
							{
								Type: DiffTypeAdded,
								Name: "Role",
								Old:  "",
								New:  "nomad-task",
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
					DisableFile:  true,
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
								Name: "DisableFile",
								Old:  "true",
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
					Role:         "nomad-task",
					Namespace:    "ns1",
					Policies:     []string{"foo", "bar"},
					Env:          true,
					DisableFile:  true,
					ChangeMode:   "signal",
					ChangeSignal: "SIGUSR1",
				},
			},
			New: &Task{
				Vault: &Vault{
					Role:         "nomad-task-2",
					Namespace:    "ns2",
					Policies:     []string{"bar", "baz"},
					Env:          false,
					DisableFile:  false,
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
								Name: "DisableFile",
								Old:  "true",
								New:  "false",
							},
							{
								Type: DiffTypeEdited,
								Name: "Env",
								Old:  "true",
								New:  "false",
							},
							{
								Type: DiffTypeEdited,
								Name: "Namespace",
								Old:  "ns1",
								New:  "ns2",
							},
							{
								Type: DiffTypeEdited,
								Name: "Role",
								Old:  "nomad-task",
								New:  "nomad-task-2",
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
					Role:         "nomad-task",
					Namespace:    "ns1",
					Policies:     []string{"foo", "bar"},
					Env:          true,
					DisableFile:  true,
					ChangeMode:   "signal",
					ChangeSignal: "SIGUSR1",
				},
			},
			New: &Task{
				Vault: &Vault{
					Role:         "nomad-task",
					Namespace:    "ns1",
					Policies:     []string{"bar", "baz"},
					Env:          true,
					DisableFile:  true,
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
								Name: "DisableFile",
								Old:  "true",
								New:  "true",
							},
							{
								Type: DiffTypeNone,
								Name: "Env",
								Old:  "true",
								New:  "true",
							},
							{
								Type: DiffTypeNone,
								Name: "Namespace",
								Old:  "ns1",
								New:  "ns1",
							},
							{
								Type: DiffTypeNone,
								Name: "Role",
								Old:  "nomad-task",
								New:  "nomad-task",
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
						ChangeScript: &ChangeScript{
							Command:     "/bin/foo",
							Args:        []string{"-debug"},
							Timeout:     5,
							FailOnError: false,
						},
						Splay: 1,
						Perms: "0644",
						Uid:   pointer.Of(1001),
						Gid:   pointer.Of(21),
						Wait: &WaitConfig{
							Min: pointer.Of(5 * time.Second),
							Max: pointer.Of(5 * time.Second),
						},
						ErrMissingKey: false,
					},
					{
						SourcePath:   "foo2",
						DestPath:     "bar2",
						EmbeddedTmpl: "baz2",
						ChangeMode:   "bam2",
						ChangeSignal: "SIGHUP2",
						ChangeScript: &ChangeScript{
							Command:     "/bin/foo2",
							Args:        []string{"-debugs"},
							Timeout:     6,
							FailOnError: false,
						},
						Splay:   2,
						Perms:   "0666",
						Uid:     pointer.Of(1000),
						Gid:     pointer.Of(20),
						Envvars: true,
					},
				},
			},
			New: &Task{
				Templates: []*Template{
					{
						SourcePath:   "foo",
						DestPath:     "bar",
						EmbeddedTmpl: "baz new",
						ChangeMode:   "bam",
						ChangeSignal: "SIGHUP",
						ChangeScript: &ChangeScript{
							Command:     "/bin/foo",
							Args:        []string{"-debug"},
							Timeout:     5,
							FailOnError: false,
						},
						Splay: 1,
						Perms: "0644",
						Uid:   pointer.Of(1001),
						Gid:   pointer.Of(21),
						Wait: &WaitConfig{
							Min: pointer.Of(5 * time.Second),
							Max: pointer.Of(10 * time.Second),
						},
						ErrMissingKey: true,
					},
					{
						SourcePath:   "foo3",
						DestPath:     "bar3",
						EmbeddedTmpl: "baz3",
						ChangeMode:   "bam3",
						ChangeSignal: "SIGHUP3",
						ChangeScript: &ChangeScript{
							Command:     "/bin/foo3",
							Args:        []string{"-debugss"},
							Timeout:     7,
							FailOnError: false,
						},
						Splay: 3,
						Perms: "0776",
						Uid:   pointer.Of(1002),
						Gid:   pointer.Of(22),
						Wait: &WaitConfig{
							Min: pointer.Of(5 * time.Second),
							Max: pointer.Of(10 * time.Second),
						},
						ErrMissingKey: true,
					},
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Template",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "EmbeddedTmpl",
								Old:  "baz",
								New:  "baz new",
							},
							{
								Type: DiffTypeEdited,
								Name: "ErrMissingKey",
								Old:  "false",
								New:  "true",
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeEdited,
								Name: "Template",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeEdited,
										Name: "Max",
										Old:  "5000000000",
										New:  "10000000000",
									},
								},
							},
						},
					},
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
								Name: "ErrMissingKey",
								Old:  "",
								New:  "true",
							},
							{
								Type: DiffTypeAdded,
								Name: "Gid",
								Old:  "",
								New:  "22",
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
							{
								Type: DiffTypeAdded,
								Name: "Uid",
								Old:  "",
								New:  "1002",
							},
							{
								Type: DiffTypeAdded,
								Name: "VaultGrace",
								Old:  "",
								New:  "0",
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeAdded,
								Name: "Template",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Max",
										Old:  "",
										New:  "10000000000",
									},
									{
										Type: DiffTypeAdded,
										Name: "Min",
										Old:  "",
										New:  "5000000000",
									},
								},
							},
							{
								Type: DiffTypeAdded,
								Name: "ChangeScript",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Command",
										Old:  "",
										New:  "/bin/foo3",
									},
									{
										Type: DiffTypeAdded,
										Name: "FailOnError",
										Old:  "",
										New:  "false",
									},
									{
										Type: DiffTypeAdded,
										Name: "Timeout",
										Old:  "",
										New:  "7",
									},
								},
								Objects: []*ObjectDiff{
									{
										Type: DiffTypeAdded,
										Name: "Args",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeAdded,
												Name: "Args",
												Old:  "",
												New:  "-debugss",
											},
										},
									},
								},
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
								Name: "ErrMissingKey",
								Old:  "false",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Gid",
								Old:  "20",
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
							{
								Type: DiffTypeDeleted,
								Name: "Uid",
								Old:  "1000",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "VaultGrace",
								Old:  "0",
								New:  "",
							},
						},
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeDeleted,
								Name: "ChangeScript",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeDeleted,
										Name: "Command",
										Old:  "/bin/foo2",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "FailOnError",
										Old:  "false",
										New:  "",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Timeout",
										Old:  "6",
										New:  "",
									},
								},
								Objects: []*ObjectDiff{
									{
										Type: DiffTypeDeleted,
										Name: "Args",
										Fields: []*FieldDiff{
											{
												Type: DiffTypeDeleted,
												Name: "Args",
												Old:  "-debugs",
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
		{
			Name: "Identity added",
			Old:  &Task{},
			New: &Task{
				Identity: &WorkloadIdentity{
					Env: true,
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Identity",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Env",
								Old:  "",
								New:  "true",
							},
							{
								Type: DiffTypeAdded,
								Name: "File",
								Old:  "",
								New:  "false",
							},
						},
					},
				},
			},
		},
		{
			Name: "Identity removed",
			Old: &Task{
				Identity: &WorkloadIdentity{
					Env: true,
				},
			},
			New: &Task{},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeDeleted,
						Name: "Identity",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "Env",
								Old:  "true",
							},
							{
								Type: DiffTypeDeleted,
								Name: "File",
								Old:  "false",
							},
						},
					},
				},
			},
		},
		{
			Name: "Identity modified",
			Old: &Task{
				Identity: &WorkloadIdentity{
					Env: true,
				},
			},
			New: &Task{
				Identity: &WorkloadIdentity{
					Env:  true,
					File: true,
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Identity",
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "File",
								Old:  "false",
								New:  "true",
							},
						},
					},
				},
			},
		},
		{
			Name: "Alternate identities added",
			Old: &Task{
				Identities: []*WorkloadIdentity{
					{
						Name:     "consul",
						Audience: []string{"consul.io"},
						Env:      true,
					},
				},
			},
			New: &Task{
				Identities: []*WorkloadIdentity{
					{
						Name:     "consul",
						Audience: []string{"consul.io"},
						Env:      true,
					},
					{
						Name:     "vault",
						Audience: []string{"vault.io"},
						File:     true,
					},
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeAdded,
						Name: "Identity",
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeAdded,
								Name: "Audience",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Audience",
										Old:  "",
										New:  "vault.io",
									},
								},
							},
						},
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Env",
								Old:  "",
								New:  "false",
							},
							{
								Type: DiffTypeAdded,
								Name: "File",
								Old:  "",
								New:  "true",
							},
							{
								Type: DiffTypeAdded,
								Name: "Name",
								Old:  "",
								New:  "vault",
							},
						},
					},
				},
			},
		},
		{
			Name: "Alternate identities edited",
			Old: &Task{
				Identities: []*WorkloadIdentity{
					{
						Name:     "consul",
						Audience: []string{"consul.io"},
						Env:      true,
						File:     false,
					},
					{
						Name:     "vault",
						Audience: []string{"vault.io"},
						File:     true,
					},
				},
			},
			New: &Task{
				Identities: []*WorkloadIdentity{
					{
						Name:     "consul",
						Audience: []string{"consul-prod.io"},
						Env:      false,
						File:     true,
					},
					{
						// Modifying the previous block to be deleted and a new
						// one to be created.
						Name:     "vault-dev",
						Audience: []string{"vault.io"},
						File:     true,
					},
				},
			},
			Expected: &TaskDiff{
				Type: DiffTypeEdited,
				Objects: []*ObjectDiff{
					{
						Type: DiffTypeEdited,
						Name: "Identity",
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeEdited,
								Name: "Audience",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Audience",
										Old:  "",
										New:  "consul-prod.io",
									},
									{
										Type: DiffTypeDeleted,
										Name: "Audience",
										Old:  "consul.io",
										New:  "",
									},
								},
							},
						},
						Fields: []*FieldDiff{
							{
								Type: DiffTypeEdited,
								Name: "Env",
								Old:  "true",
								New:  "false",
							},
							{
								Type: DiffTypeEdited,
								Name: "File",
								Old:  "false",
								New:  "true",
							},
						},
					},
					{
						Type: DiffTypeAdded,
						Name: "Identity",
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeAdded,
								Name: "Audience",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeAdded,
										Name: "Audience",
										Old:  "",
										New:  "vault.io",
									},
								},
							},
						},
						Fields: []*FieldDiff{
							{
								Type: DiffTypeAdded,
								Name: "Env",
								Old:  "",
								New:  "false",
							},
							{
								Type: DiffTypeAdded,
								Name: "File",
								Old:  "",
								New:  "true",
							},
							{
								Type: DiffTypeAdded,
								Name: "Name",
								Old:  "",
								New:  "vault-dev",
							},
						},
					},
					{
						Type: DiffTypeDeleted,
						Name: "Identity",
						Objects: []*ObjectDiff{
							{
								Type: DiffTypeDeleted,
								Name: "Audience",
								Fields: []*FieldDiff{
									{
										Type: DiffTypeDeleted,
										Name: "Audience",
										Old:  "vault.io",
										New:  "",
									},
								},
							},
						},
						Fields: []*FieldDiff{
							{
								Type: DiffTypeDeleted,
								Name: "Env",
								Old:  "false",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "File",
								Old:  "true",
								New:  "",
							},
							{
								Type: DiffTypeDeleted,
								Name: "Name",
								Old:  "vault",
								New:  "",
							},
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			actual, err := c.Old.Diff(c.New, c.Contextual)
			if c.Error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, c.Expected, actual)
			}
		})
	}
}

func TestServicesDiff(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name       string
		Old, New   []*Service
		Expected   []*ObjectDiff
		Contextual bool
	}{
		{
			Name:       "No changes - empty",
			Contextual: true,
			Old:        []*Service{},
			New:        []*Service{},
			Expected:   nil,
		},
		{
			Name:       "No changes",
			Contextual: true,
			Old: []*Service{
				{
					Name:      "webapp",
					PortLabel: "http",
				},
			},
			New: []*Service{
				{
					Name:      "webapp",
					PortLabel: "http",
				},
			},
			Expected: nil,
		},
		{
			Name:       "Detect changes",
			Contextual: true,
			Old: []*Service{
				{
					Name:              "webapp",
					PortLabel:         "http",
					AddressMode:       "host",
					Address:           "a.example.com",
					EnableTagOverride: true,
					Tags:              []string{"prod"},
					CanaryTags:        []string{"canary"},
				},
			},
			New: []*Service{
				{
					Name:              "webapp-2",
					PortLabel:         "https",
					AddressMode:       "alloc",
					Address:           "b.example.com",
					EnableTagOverride: false,
					Tags:              []string{"prod", "dev"},
					CanaryTags:        []string{"qa"},
				},
			},
			Expected: []*ObjectDiff{
				{
					Type: DiffTypeEdited,
					Name: "Service",
					Fields: []*FieldDiff{
						{
							Type: DiffTypeEdited,
							Name: "Address",
							Old:  "a.example.com",
							New:  "b.example.com",
						},
						{
							Type: DiffTypeEdited,
							Name: "AddressMode",
							Old:  "host",
							New:  "alloc",
						},
						{
							Type: DiffTypeEdited,
							Name: "EnableTagOverride",
							Old:  "true",
							New:  "false",
						},
						{
							Type: DiffTypeEdited,
							Name: "Name",
							Old:  "webapp",
							New:  "webapp-2",
						},
						{
							Type: DiffTypeNone,
							Name: "Namespace",
						},
						{
							Type: DiffTypeNone,
							Name: "OnUpdate",
						},
						{
							Type: DiffTypeEdited,
							Name: "PortLabel",
							Old:  "http",
							New:  "https",
						},
						{
							Type: DiffTypeNone,
							Name: "Provider",
						},
						{
							Type: DiffTypeNone,
							Name: "TaskName",
						},
					},
					Objects: []*ObjectDiff{
						{
							Type: DiffTypeEdited,
							Name: "CanaryTags",
							Fields: []*FieldDiff{
								{
									Type: DiffTypeAdded,
									Name: "CanaryTags",
									New:  "qa",
								},
								{
									Type: DiffTypeDeleted,
									Name: "CanaryTags",
									Old:  "canary",
								},
							},
						},
						{
							Type: DiffTypeAdded,
							Name: "Tags",
							Fields: []*FieldDiff{
								{
									Type: DiffTypeAdded,
									Name: "Tags",
									New:  "dev",
								},
								{
									Type: DiffTypeNone,
									Name: "Tags",
									Old:  "prod",
									New:  "prod",
								},
							},
						},
					},
				},
			},
		},
		{
			Name:       "Service added",
			Contextual: true,
			Old:        []*Service{},
			New: []*Service{
				{
					Name:      "webapp",
					PortLabel: "http",
				},
			},
			Expected: []*ObjectDiff{
				{
					Type: DiffTypeAdded,
					Name: "Service",
					Fields: []*FieldDiff{
						{
							Type: DiffTypeNone,
							Name: "Address",
						},
						{
							Type: DiffTypeNone,
							Name: "AddressMode",
						},
						{
							Type: DiffTypeAdded,
							Name: "EnableTagOverride",
							New:  "false",
						},
						{
							Type: DiffTypeAdded,
							Name: "Name",
							New:  "webapp",
						},
						{
							Type: DiffTypeNone,
							Name: "Namespace",
						},
						{
							Type: DiffTypeNone,
							Name: "OnUpdate",
						},
						{
							Type: DiffTypeAdded,
							Name: "PortLabel",
							New:  "http",
						},
						{
							Type: DiffTypeNone,
							Name: "Provider",
						},
						{
							Type: DiffTypeNone,
							Name: "TaskName",
						},
					},
				},
			},
		},
		{
			Name:       "Service added with same name",
			Contextual: true,
			Old: []*Service{
				{
					Name:      "webapp",
					PortLabel: "http",
				},
			},
			New: []*Service{
				{
					Name:      "webapp",
					PortLabel: "https",
				},
				{
					Name:      "webapp",
					PortLabel: "http",
				},
			},
			Expected: []*ObjectDiff{
				{
					Type: DiffTypeAdded,
					Name: "Service",
					Fields: []*FieldDiff{
						{
							Type: DiffTypeNone,
							Name: "Address",
						},
						{
							Type: DiffTypeNone,
							Name: "AddressMode",
						},
						{
							Type: DiffTypeAdded,
							Name: "EnableTagOverride",
							New:  "false",
						},
						{
							Type: DiffTypeAdded,
							Name: "Name",
							New:  "webapp",
						},
						{
							Type: DiffTypeNone,
							Name: "Namespace",
						},
						{
							Type: DiffTypeNone,
							Name: "OnUpdate",
						},
						{
							Type: DiffTypeAdded,
							Name: "PortLabel",
							New:  "https",
						},
						{
							Type: DiffTypeNone,
							Name: "Provider",
						},
						{
							Type: DiffTypeNone,
							Name: "TaskName",
						},
					},
				},
			},
		},
		{
			Name:       "Modify port label of service with same name",
			Contextual: true,
			Old: []*Service{
				{
					Name:      "webapp",
					PortLabel: "http",
				},
				{
					Name:      "webapp",
					PortLabel: "https",
				},
			},
			New: []*Service{
				{
					Name:      "webapp",
					PortLabel: "https-redirect",
				},
				{
					Name:      "webapp",
					PortLabel: "https",
				},
			},
			Expected: []*ObjectDiff{
				{
					Type: DiffTypeEdited,
					Name: "Service",
					Fields: []*FieldDiff{
						{
							Type: DiffTypeNone,
							Name: "Address",
						},
						{
							Type: DiffTypeNone,
							Name: "AddressMode",
						},
						{
							Type: DiffTypeNone,
							Name: "EnableTagOverride",
							Old:  "false",
							New:  "false",
						},
						{
							Type: DiffTypeNone,
							Name: "Name",
							Old:  "webapp",
							New:  "webapp",
						},
						{
							Type: DiffTypeNone,
							Name: "Namespace",
						},
						{
							Type: DiffTypeNone,
							Name: "OnUpdate",
						},
						{
							Type: DiffTypeEdited,
							Name: "PortLabel",
							Old:  "http",
							New:  "https-redirect",
						}, {
							Type: DiffTypeNone,
							Name: "Provider",
						},
						{
							Type: DiffTypeNone,
							Name: "TaskName",
						},
					},
				},
			},
		},
		{
			Name:       "Modify similar services",
			Contextual: true,
			Old: []*Service{
				{
					Name:      "webapp",
					PortLabel: "http",
					Tags:      []string{"prod"},
				},
				{
					Name:      "webapp",
					PortLabel: "http",
					Tags:      []string{"dev"},
				},
			},
			New: []*Service{
				{
					Name:      "webapp",
					PortLabel: "http",
					Tags:      []string{"prod", "qa"},
				},
				{
					Name:      "webapp",
					PortLabel: "http",
					Tags:      []string{"dev"},
				},
			},
			Expected: []*ObjectDiff{
				{
					Type: DiffTypeEdited,
					Name: "Service",
					Fields: []*FieldDiff{
						{
							Type: DiffTypeNone,
							Name: "Address",
						},
						{
							Type: DiffTypeNone,
							Name: "AddressMode",
						},
						{
							Type: DiffTypeNone,
							Name: "EnableTagOverride",
							Old:  "false",
							New:  "false",
						},
						{
							Type: DiffTypeNone,
							Name: "Name",
							Old:  "webapp",
							New:  "webapp",
						},
						{
							Type: DiffTypeNone,
							Name: "Namespace",
						},
						{
							Type: DiffTypeNone,
							Name: "OnUpdate",
						},
						{
							Type: DiffTypeNone,
							Name: "PortLabel",
							Old:  "http",
							New:  "http",
						},
						{
							Type: DiffTypeNone,
							Name: "Provider",
						},
						{
							Type: DiffTypeNone,
							Name: "TaskName",
						},
					},
					Objects: []*ObjectDiff{
						{
							Type: DiffTypeAdded,
							Name: "Tags",
							Fields: []*FieldDiff{
								{
									Type: DiffTypeAdded,
									Name: "Tags",
									New:  "qa",
								},
								{
									Type: DiffTypeNone,
									Name: "Tags",
									Old:  "prod",
									New:  "prod",
								},
							},
						},
					},
				},
			},
		},
		{
			Name:       "Service with different provider",
			Contextual: true,
			Old: []*Service{
				{
					Name:      "webapp",
					Provider:  "nomad",
					PortLabel: "http",
				},
			},
			New: []*Service{
				{
					Name:      "webapp",
					Provider:  "consul",
					PortLabel: "http",
				},
			},
			Expected: []*ObjectDiff{
				{
					Type: DiffTypeEdited,
					Name: "Service",
					Fields: []*FieldDiff{
						{
							Type: DiffTypeNone,
							Name: "Address",
						},
						{
							Type: DiffTypeNone,
							Name: "AddressMode",
						},
						{
							Type: DiffTypeNone,
							Name: "EnableTagOverride",
							Old:  "false",
							New:  "false",
						},
						{
							Type: DiffTypeNone,
							Name: "Name",
							Old:  "webapp",
							New:  "webapp",
						},
						{
							Type: DiffTypeNone,
							Name: "Namespace",
						},
						{
							Type: DiffTypeNone,
							Name: "OnUpdate",
						},
						{
							Type: DiffTypeNone,
							Name: "PortLabel",
							Old:  "http",
							New:  "http",
						},
						{
							Type: DiffTypeEdited,
							Name: "Provider",
							Old:  "nomad",
							New:  "consul",
						},
						{
							Type: DiffTypeNone,
							Name: "TaskName",
						},
					},
				},
			},
		},
		{
			Name:       "Service with different ingress tls",
			Contextual: false,
			Old: []*Service{
				{
					Name:      "webapp",
					Provider:  "consul",
					PortLabel: "http",
					Connect: &ConsulConnect{
						Gateway: &ConsulGateway{
							Ingress: &ConsulIngressConfigEntry{},
						},
					},
				},
			},
			New: []*Service{
				{
					Name:      "webapp",
					Provider:  "consul",
					PortLabel: "http",
					Connect: &ConsulConnect{
						Gateway: &ConsulGateway{
							Ingress: &ConsulIngressConfigEntry{
								TLS: &ConsulGatewayTLSConfig{
									Enabled:       true,
									TLSMinVersion: "TLSv1_2",
									CipherSuites:  []string{"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384", "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"},
								},
							},
						},
					},
				},
			},
			Expected: []*ObjectDiff{
				{
					Type: DiffTypeEdited,
					Name: "Service",
					Objects: []*ObjectDiff{
						{
							Type: DiffTypeEdited,
							Name: "ConsulConnect",
							Objects: []*ObjectDiff{
								{
									Type: DiffTypeEdited,
									Name: "Gateway",
									Objects: []*ObjectDiff{
										{
											Type: DiffTypeEdited,
											Name: "Ingress",
											Objects: []*ObjectDiff{
												{
													Type: DiffTypeAdded,
													Name: "TLS",
													Fields: []*FieldDiff{
														{
															Type: DiffTypeAdded,
															Name: "Enabled",
															New:  "true",
														},
														{
															Type: DiffTypeAdded,
															Name: "TLSMinVersion",
															New:  "TLSv1_2",
														},
													},
													Objects: []*ObjectDiff{
														{
															Type: DiffTypeAdded,
															Name: "CipherSuites",
															Fields: []*FieldDiff{
																{
																	Type: DiffTypeAdded,
																	Name: "CipherSuites",
																	New:  "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
																},
																{
																	Type: DiffTypeAdded,
																	Name: "CipherSuites",
																	New:  "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
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
						},
					},
				},
			},
		}, {
			Name:       "SidecarService with different meta",
			Contextual: false,
			Old: []*Service{
				{
					Name:      "webapp",
					Provider:  "consul",
					PortLabel: "http",
					Connect: &ConsulConnect{
						SidecarService: &ConsulSidecarService{
							Port:  "http",
							Proxy: &ConsulProxy{},
							Meta: map[string]string{
								"foo": "qux",
							},
						},
						Gateway: &ConsulGateway{
							Ingress: &ConsulIngressConfigEntry{},
						},
					},
				},
			},
			New: []*Service{
				{
					Name:      "webapp",
					Provider:  "consul",
					PortLabel: "http",
					Connect: &ConsulConnect{
						SidecarService: &ConsulSidecarService{
							Port:  "http",
							Proxy: &ConsulProxy{},
							Meta: map[string]string{
								"foo":     "var",
								"testKey": "testValue",
							},
						},
						Gateway: &ConsulGateway{
							Ingress: &ConsulIngressConfigEntry{},
						},
					},
				},
			},
			Expected: []*ObjectDiff{
				{
					Type: DiffTypeEdited,
					Name: "Service",
					Objects: []*ObjectDiff{
						{
							Type:   "Edited",
							Name:   "ConsulConnect",
							Fields: nil,
							Objects: []*ObjectDiff{
								{
									Type: "Edited",
									Name: "SidecarService",
									Fields: []*FieldDiff{
										{
											Type:        "Edited",
											Name:        "Meta[foo]",
											Old:         "qux",
											New:         "var",
											Annotations: nil,
										},
										{
											Type:        "Added",
											Name:        "Meta[testKey]",
											Old:         "",
											New:         "testValue",
											Annotations: nil,
										},
									},
									Objects: nil,
								},
							},
						},
					},
				},
			},
		},
		{
			Name:       "Service with different identity name and aud",
			Contextual: false,
			Old: []*Service{
				{
					Name:      "test",
					Provider:  "consul",
					PortLabel: "http",
					Identity: &WorkloadIdentity{
						Name:     "test",
						Audience: []string{"consul.io"},
						File:     true,
					},
				},
			},
			New: []*Service{
				{
					Name:      "test2",
					Provider:  "consul",
					PortLabel: "http",
					Identity: &WorkloadIdentity{
						Name:     "test2",
						Audience: []string{"consul.io", "nomad.dev"},
						File:     false,
					},
				},
			},
			Expected: []*ObjectDiff{
				{
					Type: DiffTypeEdited,
					Name: "Service",
					Fields: []*FieldDiff{
						{
							Type:        DiffTypeEdited,
							Name:        "Name",
							Old:         "test",
							New:         "test2",
							Annotations: nil,
						},
					},
					Objects: []*ObjectDiff{
						{
							Type: DiffTypeEdited,
							Name: "Identity",
							Fields: []*FieldDiff{
								{
									Type:        DiffTypeEdited,
									Name:        "File",
									Old:         "true",
									New:         "false",
									Annotations: nil,
								},
								{
									Type:        DiffTypeEdited,
									Name:        "Name",
									Old:         "test",
									New:         "test2",
									Annotations: nil,
								},
							},
							Objects: []*ObjectDiff{
								{
									Type: DiffTypeAdded,
									Name: "Audience",
									Fields: []*FieldDiff{
										{
											Type: DiffTypeAdded,
											Name: "Audience",
											New:  "nomad.dev",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			actual := serviceDiffs(c.Old, c.New, c.Contextual)
			require.Equal(t, c.Expected, actual)
		})
	}
}
