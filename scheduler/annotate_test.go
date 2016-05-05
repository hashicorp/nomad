package scheduler

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
)

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
		DiffEntry: structs.DiffEntry{
			Type: structs.DiffTypeEdited,
		},
		Name:     "Count",
		OldValue: 1,
		NewValue: 3,
	}
	down := &structs.FieldDiff{
		DiffEntry: structs.DiffEntry{
			Type: structs.DiffTypeEdited,
		},
		Name:     "Count",
		OldValue: 3,
		NewValue: 1,
	}
	tgUp := &structs.TaskGroupDiff{
		PrimitiveStructDiff: structs.PrimitiveStructDiff{
			DiffEntry: structs.DiffEntry{
				Type: structs.DiffTypeEdited,
			},
			PrimitiveFields: map[string]*structs.FieldDiff{
				"Count": up,
			},
		},
	}
	tgDown := &structs.TaskGroupDiff{
		PrimitiveStructDiff: structs.PrimitiveStructDiff{
			DiffEntry: structs.DiffEntry{
				Type: structs.DiffTypeEdited,
			},
			PrimitiveFields: map[string]*structs.FieldDiff{
				"Count": down,
			},
		},
	}

	// Test the up case
	annotateCountChange(tgUp)
	countDiff := tgUp.PrimitiveFields["Count"]
	if len(countDiff.Annotations) != 1 || countDiff.Annotations[0] != AnnotationForcesCreate {
		t.Fatalf("incorrect annotation: %#v", tgUp)
	}

	// Test the down case
	annotateCountChange(tgDown)
	countDiff = tgDown.PrimitiveFields["Count"]
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
			PrimitiveStructDiff: structs.PrimitiveStructDiff{
				DiffEntry: structs.DiffEntry{
					Type: structs.DiffTypeEdited,
				},
				PrimitiveFields: map[string]*structs.FieldDiff{
					"Driver": &structs.FieldDiff{
						Name:     "Driver",
						OldValue: "docker",
						NewValue: "exec",
					},
				},
			},
		},
		{
			PrimitiveStructDiff: structs.PrimitiveStructDiff{
				DiffEntry: structs.DiffEntry{
					Type: structs.DiffTypeEdited,
				},
				PrimitiveFields: map[string]*structs.FieldDiff{
					"User": &structs.FieldDiff{
						Name:     "User",
						OldValue: "",
						NewValue: "specific",
					},
				},
			},
		},
		{
			Env: &structs.StringMapDiff{
				DiffEntry: structs.DiffEntry{
					Type: structs.DiffTypeAdded,
				},
				Added: map[string]string{
					"foo": "bar",
				},
			},
		},
		{
			Meta: &structs.StringMapDiff{
				DiffEntry: structs.DiffEntry{
					Type: structs.DiffTypeEdited,
				},
				Edited: map[string]structs.StringValueDelta{
					"foo": {
						Old: "a",
						New: "b",
					},
				},
			},
		},
		{
			Artifacts: &structs.TaskArtifactsDiff{
				DiffEntry: structs.DiffEntry{
					Type: structs.DiffTypeAdded,
				},
				Added: []*structs.TaskArtifact{
					{
						GetterSource: "foo",
					},
				},
			},
		},
		{
			Resources: &structs.ResourcesDiff{
				PrimitiveStructDiff: structs.PrimitiveStructDiff{
					DiffEntry: structs.DiffEntry{
						Type: structs.DiffTypeEdited,
					},
					PrimitiveFields: map[string]*structs.FieldDiff{
						"CPU": &structs.FieldDiff{
							Name:     "CPU",
							OldValue: 500,
							NewValue: 1000,
						},
					},
				},
			},
		},
		{
			Config: &structs.StringMapDiff{
				DiffEntry: structs.DiffEntry{
					Type: structs.DiffTypeEdited,
				},
				Edited: map[string]structs.StringValueDelta{
					"command": {
						Old: "/bin/date",
						New: "/bin/bash",
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
			Constraints: []*structs.PrimitiveStructDiff{
				{
					DiffEntry: structs.DiffEntry{
						Type: structs.DiffTypeEdited,
					},
					PrimitiveFields: map[string]*structs.FieldDiff{
						"RTarget": &structs.FieldDiff{
							Name:     "RTarget",
							OldValue: "linux",
							NewValue: "windows",
						},
					},
				},
			},
		},
		{
			LogConfig: &structs.PrimitiveStructDiff{
				DiffEntry: structs.DiffEntry{
					Type: structs.DiffTypeEdited,
				},
				PrimitiveFields: map[string]*structs.FieldDiff{
					"MaxFileSizeMB": &structs.FieldDiff{
						Name:     "MaxFileSizeMB",
						OldValue: 100,
						NewValue: 128,
					},
				},
			},
		},
		{
			Services: &structs.ServicesDiff{
				DiffEntry: structs.DiffEntry{
					Type: structs.DiffTypeAdded,
				},
				Added: []*structs.Service{
					{
						Name:      "foo",
						PortLabel: "rpc",
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
