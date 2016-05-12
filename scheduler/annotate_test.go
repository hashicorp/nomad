package scheduler

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
)

func TestAnnotateTaskGroup_Updates(t *testing.T) {
	annotations := &structs.PlanAnnotations{
		DesiredTGUpdates: map[string]*structs.DesiredUpdates{
			"foo": &structs.DesiredUpdates{
				Ignore:            1,
				Place:             2,
				Migrate:           3,
				Stop:              4,
				InPlaceUpdate:     5,
				DestructiveUpdate: 6,
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
	tg := &structs.TaskGroupDiff{}
	tgOrig := &structs.TaskGroupDiff{}
	annotateCountChange(tg)
	if !reflect.DeepEqual(tgOrig, tg) {
		t.Fatalf("annotateCountChange(%#v) should not have caused any annotation: %#v", tgOrig, tg)
	}
}

func TestAnnotateCountChange(t *testing.T) {
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
	td := &structs.TaskDiff{}
	tdOrig := &structs.TaskDiff{}
	annotateTask(td)
	if !reflect.DeepEqual(tdOrig, td) {
		t.Fatalf("annotateTask(%#v) should not have caused any annotation: %#v", tdOrig, td)
	}
}

func TestAnnotateTask_Destructive(t *testing.T) {
	cases := []*structs.TaskDiff{
		{
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
		{
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
		{
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
		{
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
		{
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
		{
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
							Name: "IOPS",
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
		{
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
	}

	for i, c := range cases {
		c.Type = structs.DiffTypeEdited
		annotateTask(c)
		if len(c.Annotations) != 1 || c.Annotations[0] != AnnotationForcesDestructiveUpdate {
			t.Fatalf("case %d not properly annotated %#v", i+1, c)
		}
	}
}

func TestAnnotateTask_Inplace(t *testing.T) {
	cases := []*structs.TaskDiff{
		{
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
		{
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
		{
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
	}

	for i, c := range cases {
		c.Type = structs.DiffTypeEdited
		annotateTask(c)
		if len(c.Annotations) != 1 || c.Annotations[0] != AnnotationForcesInplaceUpdate {
			t.Fatalf("case %d not properly annotated %#v", i+1, c)
		}
	}
}
