package diff

import (
	"reflect"
	"sort"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestNewJobDiff_Same(t *testing.T) {
	job1 := mock.Job()
	job2 := mock.Job()
	job2.ID = job1.ID

	diff := NewJobDiff(job1, job2)
	if diff != nil {
		t.Fatalf("expected nil job diff; got %s", spew.Sdump(diff))
	}
}

func TestNewJobDiff_Constraints(t *testing.T) {
	c1 := &structs.Constraint{LTarget: "foo"}
	c2 := &structs.Constraint{LTarget: "bar"}
	c3 := &structs.Constraint{LTarget: "baz"}

	// Test the added case.
	j1 := &structs.Job{Constraints: []*structs.Constraint{c1, c2}}
	j2 := &structs.Job{Constraints: []*structs.Constraint{c1, c2, c3}}

	diff := NewJobDiff(j1, j2)
	if diff == nil {
		t.Fatalf("expected non-nil job diff")
	}

	if diff.Type != DiffTypeEdited {
		t.Fatalf("got diff type %v; want %v", diff.Type, DiffTypeEdited)
	}

	if len(diff.Constraints) != 1 {
		t.Fatalf("expected one constraint diff; got %v", diff.Constraints)
	}

	cdiff := diff.Constraints[0]
	if cdiff.Type != DiffTypeAdded {
		t.Fatalf("expected constraint to be added: %#v", cdiff)
	}
	if len(cdiff.PrimitiveFields) != 3 {
		t.Fatalf("bad: %#v", cdiff)
	}

	// Test the deleted case.
	j1 = &structs.Job{Constraints: []*structs.Constraint{c1, c2}}
	j2 = &structs.Job{Constraints: []*structs.Constraint{c1}}

	diff = NewJobDiff(j1, j2)
	if diff == nil {
		t.Fatalf("expected non-nil job diff")
	}

	if diff.Type != DiffTypeEdited {
		t.Fatalf("got diff type %v; want %v", diff.Type, DiffTypeEdited)
	}

	if len(diff.Constraints) != 1 {
		t.Fatalf("expected one constraint diff; got %v", diff.Constraints)
	}

	cdiff = diff.Constraints[0]
	if cdiff.Type != DiffTypeDeleted {
		t.Fatalf("expected constraint to be deleted: %#v", cdiff)
	}
	if len(cdiff.PrimitiveFields) != 3 {
		t.Fatalf("bad: %#v", cdiff)
	}
}

func TestNewJobDiff_Datacenters(t *testing.T) {
	j1 := &structs.Job{Datacenters: []string{"a", "b"}}
	j2 := &structs.Job{Datacenters: []string{"b", "c"}}

	diff := NewJobDiff(j1, j2)
	if diff == nil {
		t.Fatalf("expected non-nil job diff")
	}

	if diff.Type != DiffTypeEdited {
		t.Fatalf("got diff type %v; want %v", diff.Type, DiffTypeEdited)
	}

	dd := diff.Datacenters
	if dd == nil {
		t.Fatalf("expected datacenter diff")
	}

	if !reflect.DeepEqual(dd.Added, []string{"c"}) ||
		!reflect.DeepEqual(dd.Deleted, []string{"a"}) {
		t.Fatalf("bad: %#v", dd)
	}

}

func TestNewJobDiff_TaskGroups(t *testing.T) {
	tg1 := &structs.TaskGroup{Name: "foo"}
	tg2 := &structs.TaskGroup{Name: "bar"}
	tg2_2 := &structs.TaskGroup{Name: "bar", Count: 2}
	tg3 := &structs.TaskGroup{Name: "baz"}

	j1 := &structs.Job{TaskGroups: []*structs.TaskGroup{tg1, tg2}}
	j2 := &structs.Job{TaskGroups: []*structs.TaskGroup{tg2_2, tg3}}

	diff := NewJobDiff(j1, j2)
	if diff == nil {
		t.Fatalf("expected non-nil job diff")
	}

	if diff.Type != DiffTypeEdited {
		t.Fatalf("got diff type %v; want %v", diff.Type, DiffTypeEdited)
	}

	tgd := diff.TaskGroups
	if tgd == nil {
		t.Fatalf("expected task group diff")
	}

	if !reflect.DeepEqual(tgd.Added, []*structs.TaskGroup{tg3}) ||
		!reflect.DeepEqual(tgd.Deleted, []*structs.TaskGroup{tg1}) {
		t.Fatalf("bad: %#v", tgd)
	}

	if len(tgd.Edited) != 1 {
		t.Fatalf("expect one edited task group: %#v", tgd)
	}
	if e := tgd.Edited[0]; tgd.Type != DiffTypeEdited && len(e.PrimitiveFields) != 1 {
		t.Fatalf("bad: %#v", e)
	}

}

func TestNewPrimitiveStructDiff(t *testing.T) {
	p1 := structs.Port{Label: "1"}
	p2 := structs.Port{Label: "2"}
	p3 := structs.Port{}

	pdiff := NewPrimitiveStructDiff(nil, nil, portFields)
	if pdiff != nil {
		t.Fatalf("expected no diff: %#v", pdiff)
	}

	pdiff = NewPrimitiveStructDiff(p1, p1, portFields)
	if pdiff != nil {
		t.Fatalf("expected no diff: %#v", pdiff)
	}

	pdiff = NewPrimitiveStructDiff(p1, p2, portFields)
	if pdiff == nil {
		t.Fatalf("expected diff")
	}

	if pdiff.Type != DiffTypeEdited {
		t.Fatalf("unexpected type: got %v; want %v", pdiff.Type, DiffTypeEdited)
	}

	if len(pdiff.PrimitiveFields) != 1 {
		t.Fatalf("unexpected number of field diffs: %#v", pdiff.PrimitiveFields)
	}

	f := pdiff.PrimitiveFields[0]
	if f.Type != DiffTypeEdited {
		t.Fatalf("unexpected type: got %v; want %v", f.Type, DiffTypeEdited)
	}
	if !reflect.DeepEqual(f.OldValue, "1") || !reflect.DeepEqual(f.NewValue, "2") {
		t.Fatalf("bad: %#v", f)
	}

	pdiff = NewPrimitiveStructDiff(p1, p3, portFields)
	if pdiff == nil {
		t.Fatalf("expected diff")
	}

	if pdiff.Type != DiffTypeEdited {
		t.Fatalf("unexpected type: got %v; want %v", pdiff.Type, DiffTypeEdited)
	}

	if len(pdiff.PrimitiveFields) != 1 {
		t.Fatalf("unexpected number of field diffs: %#v", pdiff.PrimitiveFields)
	}

	f = pdiff.PrimitiveFields[0]
	if f.Type != DiffTypeEdited {
		t.Fatalf("unexpected type: got %v; want %v", f.Type, DiffTypeEdited)
	}
	if !reflect.DeepEqual(f.OldValue, "1") || !reflect.DeepEqual(f.NewValue, "") {
		t.Fatalf("bad: %#v", f)
	}
}

func TestSetDiffPrimitiveStructs(t *testing.T) {
	p1 := structs.Port{Label: "1"}
	p2 := structs.Port{Label: "2"}
	p3 := structs.Port{Label: "3"}
	p4 := structs.Port{Label: "4"}
	p5 := structs.Port{Label: "5"}
	p6 := structs.Port{Label: "6"}

	old := []structs.Port{p1, p2, p3, p4}
	new := []structs.Port{p3, p4, p5, p6}

	diffs := setDiffPrimitiveStructs(interfaceSlice(old), interfaceSlice(new), portFields)
	if len(diffs) != 4 {
		t.Fatalf("expected four diffs: %#v", diffs)
	}

	var added, deleted int
	for _, diff := range diffs {
		switch diff.Type {
		case DiffTypeAdded:
			added++
		case DiffTypeDeleted:
			deleted++
		default:
			t.Fatalf("unexpected diff type: %#v", diff.Type)
		}
	}

	if added != 2 && deleted != 2 {
		t.Fatalf("incorrect counts")
	}
}

func TestNewFieldDiff(t *testing.T) {
	cases := []struct {
		NilExpected bool
		Old, New    interface{}
		Expected    DiffType
	}{
		{
			NilExpected: true,
			Old:         1,
			New:         1,
		},
		{
			NilExpected: true,
			Old:         true,
			New:         true,
		},
		{
			NilExpected: true,
			Old:         "foo",
			New:         "foo",
		},
		{
			NilExpected: true,
			Old:         2.23,
			New:         2.23,
		},
		{
			Old:      1,
			New:      4,
			Expected: DiffTypeEdited,
		},
		{
			Old:      true,
			New:      false,
			Expected: DiffTypeEdited,
		},
		{
			Old:      "foo",
			New:      "bar",
			Expected: DiffTypeEdited,
		},
		{
			Old:      2.23,
			New:      12.511,
			Expected: DiffTypeEdited,
		},
		{
			Old:      nil,
			New:      4,
			Expected: DiffTypeAdded,
		},
		{
			Old:      nil,
			New:      true,
			Expected: DiffTypeAdded,
		},
		{
			Old:      nil,
			New:      "bar",
			Expected: DiffTypeAdded,
		},
		{
			Old:      nil,
			New:      12.511,
			Expected: DiffTypeAdded,
		},
		{
			Old:      4,
			New:      nil,
			Expected: DiffTypeDeleted,
		},
		{
			Old:      true,
			New:      nil,
			Expected: DiffTypeDeleted,
		},
		{
			Old:      "bar",
			New:      nil,
			Expected: DiffTypeDeleted,
		},
		{
			Old:      12.511,
			New:      nil,
			Expected: DiffTypeDeleted,
		},
	}

	for i, c := range cases {
		diff := NewFieldDiff("foo", c.Old, c.New)
		if diff == nil {
			if !c.NilExpected {
				t.Fatalf("case %d: diff was nil and unexpected", i)
			}
			continue
		}

		if diff.Type != c.Expected {
			t.Fatalf("case %d: wanted type %v; got %v", diff.Type, c.Expected)
		}
	}
}

func TestStringSetDiff(t *testing.T) {
	values := []string{"1", "2", "3", "4", "5", "6"}

	setDiff := NewStringSetDiff(values[:4], values[2:])
	if setDiff.Type != DiffTypeEdited {
		t.Fatalf("expected edited")
	}

	addedExp := []string{"5", "6"}
	deletedExp := []string{"1", "2"}
	sort.Strings(setDiff.Added)
	sort.Strings(setDiff.Deleted)

	if !reflect.DeepEqual(addedExp, setDiff.Added) ||
		!reflect.DeepEqual(deletedExp, setDiff.Deleted) {
		t.Fatalf("bad: %#v", setDiff)
	}
}

func TestStringMapDiff(t *testing.T) {
	m1 := map[string]string{
		"a": "aval",
		"b": "bval",
	}
	m2 := map[string]string{
		"b": "bval2",
		"c": "cval",
	}
	expected := &StringMapDiff{
		DiffEntry: DiffEntry{
			Type: DiffTypeEdited,
		},
		Added:   map[string]string{"c": "cval"},
		Deleted: map[string]string{"a": "aval"},
		Edited: map[string]StringValueDelta{
			"b": StringValueDelta{Old: "bval",
				DiffEntry: DiffEntry{
					Type: DiffTypeEdited,
				},
				New: "bval2",
			},
		},
	}

	act := NewStringMapDiff(m1, m2)
	if !reflect.DeepEqual(act, expected) {
		t.Fatalf("got %#v; want %#v", act, expected)
	}
}

func TestSetDifference(t *testing.T) {
	old := []interface{}{1, 2}
	new := []interface{}{2, 3}
	added, deleted := setDifference(old, new)

	if len(added) != 1 && len(deleted) != 1 {
		t.Fatalf("bad: %#v %#v", added, deleted)
	}

	a, ok := added[0].(int)
	if !ok || a != 3 {
		t.Fatalf("bad: %v %v", a, ok)
	}

	d, ok := deleted[0].(int)
	if !ok || d != 1 {
		t.Fatalf("bad: %v %v", a, ok)
	}
}

func TestKeyedSetDifference(t *testing.T) {
	oldMap := map[string]interface{}{
		"a": 1,
		"b": 2,
		"c": 3,
	}
	newMap := map[string]interface{}{
		"b": 3,
		"c": 3,
		"d": 4,
	}

	added, deleted, edited, unmodified := keyedSetDifference(oldMap, newMap)

	if v, ok := added["d"]; !ok || v.(int) != 4 {
		t.Fatalf("bad: %#v", added)
	}
	if v, ok := deleted["a"]; !ok || v.(int) != 1 {
		t.Fatalf("bad: %#v", deleted)
	}
	if l := len(edited); l != 1 || edited[0] != "b" {
		t.Fatalf("bad: %#v", edited)
	}
	if l := len(unmodified); l != 1 || unmodified[0] != "c" {
		t.Fatalf("bad: %#v", edited)
	}
}

func TestInterfaceSlice(t *testing.T) {
	j1 := mock.Job()
	j2 := mock.Job()
	jobs := []*structs.Job{j1, j2}

	slice := interfaceSlice(jobs)
	if len(slice) != 2 {
		t.Fatalf("bad: %#v", slice)
	}

	f := slice[0]
	actJob1, ok := f.(*structs.Job)
	if !ok {
		t.Fatalf("unexpected type: %v", actJob1)
	}

	if !reflect.DeepEqual(actJob1, j1) {
		t.Fatalf("got %#v, want %#v", actJob1, j1)
	}
}

func TestGetField(t *testing.T) {
	j := mock.Job()
	exp := "foo"
	j.Type = "foo"

	i := getField(j, "Type")
	act, ok := i.(string)
	if !ok {
		t.Fatalf("expected to get string type back")
	}

	if act != exp {
		t.Fatalf("got %v; want %v", act, exp)
	}
}
