// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestAnnotateTaskGroup_Updates(t *testing.T) {
	ci.Parallel(t)

	annotations := &structs.PlanAnnotations{
		DesiredTGUpdates: map[string]*structs.DesiredUpdates{
			"foo": {
				Ignore:            1,
				Place:             2,
				Migrate:           3,
				Stop:              4,
				InPlaceUpdate:     5,
				DestructiveUpdate: 6,
				Canary:            7,
			},
		},
	}

	tgDiff := &structs.TaskGroupDiff{
		Type: structs.DiffTypeEdited,
		Name: "foo",
	}
	expected := &structs.TaskGroupDiff{
		Type: structs.DiffTypeEdited,
		Name: "foo",
		Updates: map[string]uint64{
			UpdateTypeIgnore:            1,
			UpdateTypeCreate:            2,
			UpdateTypeMigrate:           3,
			UpdateTypeDestroy:           4,
			UpdateTypeInplaceUpdate:     5,
			UpdateTypeDestructiveUpdate: 6,
			UpdateTypeCanary:            7,
		},
	}

	if err := annotateTaskGroup(tgDiff, annotations); err != nil {
		t.Fatalf("annotateTaskGroup(%#v, %#v) failed: %#v", tgDiff, annotations, err)
	}

	if !reflect.DeepEqual(tgDiff, expected) {
		t.Fatalf("got %#v, want %#v", tgDiff, expected)
	}
}

func TestAnnotateCountChange_NonEdited(t *testing.T) {
	ci.Parallel(t)

	tg := &structs.TaskGroupDiff{}
	tgOrig := &structs.TaskGroupDiff{}
	annotateCountChange(tg)
	if !reflect.DeepEqual(tgOrig, tg) {
		t.Fatalf("annotateCountChange(%#v) should not have caused any annotation: %#v", tgOrig, tg)
	}
}

func TestAnnotateCountChange(t *testing.T) {
	ci.Parallel(t)

	up := &structs.FieldDiff{
		Type: structs.DiffTypeEdited,
		Name: "Count",
		Old:  "1",
		New:  "3",
	}
	down := &structs.FieldDiff{
		Type: structs.DiffTypeEdited,
		Name: "Count",
		Old:  "3",
		New:  "1",
	}
	tgUp := &structs.TaskGroupDiff{
		Type:   structs.DiffTypeEdited,
		Fields: []*structs.FieldDiff{up},
	}
	tgDown := &structs.TaskGroupDiff{
		Type:   structs.DiffTypeEdited,
		Fields: []*structs.FieldDiff{down},
	}

	// Test the up case
	if err := annotateCountChange(tgUp); err != nil {
		t.Fatalf("annotateCountChange(%#v) failed: %v", tgUp, err)
	}
	countDiff := tgUp.Fields[0]
	if len(countDiff.Annotations) != 1 || countDiff.Annotations[0] != AnnotationForcesCreate {
		t.Fatalf("incorrect annotation: %#v", tgUp)
	}

	// Test the down case
	if err := annotateCountChange(tgDown); err != nil {
		t.Fatalf("annotateCountChange(%#v) failed: %v", tgDown, err)
	}
	countDiff = tgDown.Fields[0]
	if len(countDiff.Annotations) != 1 || countDiff.Annotations[0] != AnnotationForcesDestroy {
		t.Fatalf("incorrect annotation: %#v", tgDown)
	}
}

func TestAnnotateTask_NonEdited(t *testing.T) {
	ci.Parallel(t)

	tgd := &structs.TaskGroupDiff{Type: structs.DiffTypeNone}
	td := &structs.TaskDiff{Type: structs.DiffTypeNone}
	tdOrig := &structs.TaskDiff{Type: structs.DiffTypeNone}
	annotateTask(td, tgd)
	if !reflect.DeepEqual(tdOrig, td) {
		t.Fatalf("annotateTask(%#v) should not have caused any annotation: %#v", tdOrig, td)
	}
}

func TestAnnotateTask(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Diff    *structs.TaskDiff
		Parent  *structs.TaskGroupDiff
		Desired string
	}{
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeEdited,
				Fields: []*structs.FieldDiff{
					{
						Type: structs.DiffTypeEdited,
						Name: "Driver",
						Old:  "docker",
						New:  "exec",
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeEdited},
			Desired: AnnotationForcesDestructiveUpdate,
		},
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeEdited,
				Fields: []*structs.FieldDiff{
					{
						Type: structs.DiffTypeEdited,
						Name: "User",
						Old:  "alice",
						New:  "bob",
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeEdited},
			Desired: AnnotationForcesDestructiveUpdate,
		},
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeEdited,
				Fields: []*structs.FieldDiff{
					{
						Type: structs.DiffTypeAdded,
						Name: "Env[foo]",
						Old:  "foo",
						New:  "bar",
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeEdited},
			Desired: AnnotationForcesDestructiveUpdate,
		},
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeEdited,
				Fields: []*structs.FieldDiff{
					{
						Type: structs.DiffTypeAdded,
						Name: "Meta[foo]",
						Old:  "foo",
						New:  "bar",
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeEdited},
			Desired: AnnotationForcesDestructiveUpdate,
		},
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeEdited,
				Objects: []*structs.ObjectDiff{
					{
						Type: structs.DiffTypeAdded,
						Name: "Artifact",
						Fields: []*structs.FieldDiff{
							{
								Type: structs.DiffTypeAdded,
								Name: "GetterOptions[bam]",
								Old:  "",
								New:  "baz",
							},
							{
								Type: structs.DiffTypeAdded,
								Name: "GetterSource",
								Old:  "",
								New:  "bam",
							},
							{
								Type: structs.DiffTypeAdded,
								Name: "RelativeDest",
								Old:  "",
								New:  "bam",
							},
						},
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeEdited},
			Desired: AnnotationForcesDestructiveUpdate,
		},
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeEdited,
				Objects: []*structs.ObjectDiff{
					{
						Type: structs.DiffTypeEdited,
						Name: "Resources",
						Fields: []*structs.FieldDiff{
							{
								Type: structs.DiffTypeEdited,
								Name: "CPU",
								Old:  "100",
								New:  "200",
							},
							{
								Type: structs.DiffTypeEdited,
								Name: "DiskMB",
								Old:  "100",
								New:  "200",
							},
							{
								Type: structs.DiffTypeEdited,
								Name: "MemoryMB",
								Old:  "100",
								New:  "200",
							},
						},
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeEdited},
			Desired: AnnotationForcesDestructiveUpdate,
		},
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeEdited,
				Objects: []*structs.ObjectDiff{
					{
						Type: structs.DiffTypeEdited,
						Name: "Config",
						Fields: []*structs.FieldDiff{
							{
								Type: structs.DiffTypeEdited,
								Name: "bam[1]",
								Old:  "b",
								New:  "c",
							},
						},
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeEdited},
			Desired: AnnotationForcesDestructiveUpdate,
		},
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeEdited,
				Objects: []*structs.ObjectDiff{
					{
						Type: structs.DiffTypeAdded,
						Name: "Constraint",
						Fields: []*structs.FieldDiff{
							{
								Type: structs.DiffTypeAdded,
								Name: "LTarget",
								Old:  "",
								New:  "baz",
							},
							{
								Type: structs.DiffTypeAdded,
								Name: "Operand",
								Old:  "",
								New:  "baz",
							},
							{
								Type: structs.DiffTypeAdded,
								Name: "RTarget",
								Old:  "",
								New:  "baz",
							},
						},
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeEdited},
			Desired: AnnotationForcesInplaceUpdate,
		},
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeEdited,
				Objects: []*structs.ObjectDiff{
					{
						Type: structs.DiffTypeAdded,
						Name: "LogConfig",
						Fields: []*structs.FieldDiff{
							{
								Type: structs.DiffTypeAdded,
								Name: "MaxFileSizeMB",
								Old:  "",
								New:  "10",
							},
							{
								Type: structs.DiffTypeAdded,
								Name: "MaxFiles",
								Old:  "",
								New:  "1",
							},
						},
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeEdited},
			Desired: AnnotationForcesInplaceUpdate,
		},
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeEdited,
				Objects: []*structs.ObjectDiff{
					{
						Type: structs.DiffTypeAdded,
						Name: "LogConfig",
						Fields: []*structs.FieldDiff{
							{
								Type: structs.DiffTypeAdded,
								Name: "Disabled",
								Old:  "true",
								New:  "false",
							},
						},
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeEdited},
			Desired: AnnotationForcesDestructiveUpdate,
		},
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeEdited,
				Objects: []*structs.ObjectDiff{
					{
						Type: structs.DiffTypeEdited,
						Name: "Service",
						Fields: []*structs.FieldDiff{
							{
								Type: structs.DiffTypeEdited,
								Name: "PortLabel",
								Old:  "baz",
								New:  "baz2",
							},
						},
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeEdited},
			Desired: AnnotationForcesInplaceUpdate,
		},
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeEdited,
				Fields: []*structs.FieldDiff{
					{
						Type: structs.DiffTypeEdited,
						Name: "KillTimeout",
						Old:  "200",
						New:  "2000000",
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeEdited},
			Desired: AnnotationForcesInplaceUpdate,
		},
		// Task deleted new parent
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeDeleted,
				Fields: []*structs.FieldDiff{
					{
						Type: structs.DiffTypeAdded,
						Name: "Driver",
						Old:  "",
						New:  "exec",
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeAdded},
			Desired: AnnotationForcesDestroy,
		},
		// Task Added new parent
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeAdded,
				Fields: []*structs.FieldDiff{
					{
						Type: structs.DiffTypeAdded,
						Name: "Driver",
						Old:  "",
						New:  "exec",
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeAdded},
			Desired: AnnotationForcesCreate,
		},
		// Task deleted existing parent
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeDeleted,
				Fields: []*structs.FieldDiff{
					{
						Type: structs.DiffTypeAdded,
						Name: "Driver",
						Old:  "",
						New:  "exec",
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeEdited},
			Desired: AnnotationForcesDestructiveUpdate,
		},
		// Task Added existing parent
		{
			Diff: &structs.TaskDiff{
				Type: structs.DiffTypeAdded,
				Fields: []*structs.FieldDiff{
					{
						Type: structs.DiffTypeAdded,
						Name: "Driver",
						Old:  "",
						New:  "exec",
					},
				},
			},
			Parent:  &structs.TaskGroupDiff{Type: structs.DiffTypeEdited},
			Desired: AnnotationForcesDestructiveUpdate,
		},
	}

	for i, c := range cases {
		annotateTask(c.Diff, c.Parent)
		if len(c.Diff.Annotations) != 1 || c.Diff.Annotations[0] != c.Desired {
			t.Fatalf("case %d not properly annotated; got %s, want %s", i+1, c.Diff.Annotations[0], c.Desired)
		}
	}
}
