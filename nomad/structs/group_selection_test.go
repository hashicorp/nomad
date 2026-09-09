// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func groupSelectionTestJob() *Job {
	job := testJob()
	base := job.TaskGroups[0]
	job.TaskGroups = nil
	for i, name := range []string{"encoder", "orin", "thor"} {
		group := base.Copy()
		group.Name = name
		group.Count = i + 2
		job.TaskGroups = append(job.TaskGroups, group)
	}
	job.GroupSelections = []*TaskGroupSelection{{Name: "runtime", Count: 1, Groups: []string{"encoder", "orin", "thor"}}}
	job.Canonicalize()
	return job
}

func TestJob_GroupSelectionsValidate(t *testing.T) {
	ci.Parallel(t)
	tests := []struct {
		name   string
		change func(*Job)
		want   string
	}{
		{"one of three", func(j *Job) {}, ""},
		{"two of three", func(j *Job) { j.GroupSelections[0].Count = 2 }, ""},
		{"all groups", func(j *Job) { j.GroupSelections[0].Count = 3 }, ""},
		{"disabled selection", func(j *Job) { j.GroupSelections[0].Count = 0 }, ""},
		{"zero count candidate", func(j *Job) { j.TaskGroups[0].Count = 0 }, ""},
		{"independent selections", func(j *Job) {
			j.GroupSelections[0].Groups = []string{"encoder", "orin"}
			j.GroupSelections = append(j.GroupSelections, &TaskGroupSelection{Name: "other", Count: 1, Groups: []string{"thor"}})
		}, ""},
		{"required group", func(j *Job) { j.GroupSelections[0].Groups = []string{"orin", "thor"} }, ""},
		{"nil selection", func(j *Job) { j.GroupSelections = append(j.GroupSelections, nil) }, "must not be null"},
		{"empty name", func(j *Job) { j.GroupSelections[0].Name = "" }, "name must be non-empty"},
		{"null name", func(j *Job) { j.GroupSelections[0].Name = "bad\x00name" }, "null character"},
		{"negative count", func(j *Job) { j.GroupSelections[0].Count = -1 }, "count must be non-negative"},
		{"too many groups", func(j *Job) { j.GroupSelections[0].Count = 4 }, "exceeds its 3"},
		{"not enough positive counts", func(j *Job) { j.GroupSelections[0].Count = 3; j.TaskGroups[0].Count = 0 }, "exceeds its 2"},
		{"empty candidates", func(j *Job) { j.GroupSelections[0].Groups = nil }, "at least one task group"},
		{"empty candidate name", func(j *Job) { j.GroupSelections[0].Groups[0] = "" }, "name must not be empty"},
		{"unknown candidate", func(j *Job) { j.GroupSelections[0].Groups[0] = "missing" }, "unknown task group"},
		{"duplicate candidate", func(j *Job) { j.GroupSelections[0].Groups[0] = "thor" }, "is repeated"},
		{"duplicate selection", func(j *Job) { j.GroupSelections = append(j.GroupSelections, j.GroupSelections[0].Copy()) }, "defined more than once"},
		{"overlapping selections", func(j *Job) {
			j.GroupSelections = append(j.GroupSelections, &TaskGroupSelection{Name: "other", Count: 1, Groups: []string{"thor"}})
		}, "belongs to both"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			job := groupSelectionTestJob()
			tc.change(job)
			err := job.Validate()
			if tc.want == "" {
				must.NoError(t, err)
			} else {
				must.Error(t, err)
				must.StrContains(t, err.Error(), tc.want)
			}
		})
	}
	for _, kind := range []string{JobTypeBatch, JobTypeSystem, JobTypeSysBatch, JobTypeCore} {
		t.Run(kind, func(t *testing.T) {
			job := groupSelectionTestJob()
			job.Type = kind
			err := job.validateGroupSelections()
			must.Error(t, err)
			must.StrContains(t, err.Error(), "only supported for service jobs")
			job.GroupSelections = nil
			must.NoError(t, job.validateGroupSelections())
		})
	}
}

func TestJob_GroupSelectionsCopyLookupAndSpecChanged(t *testing.T) {
	ci.Parallel(t)
	job := groupSelectionTestJob()
	must.Eq(t, job.GroupSelections[0], job.LookupGroupSelection("runtime"))
	must.Eq(t, job.GroupSelections[0], job.TaskGroupSelection("thor"))
	must.Nil(t, job.LookupGroupSelection("missing"))
	must.Nil(t, job.TaskGroupSelection("required"))
	must.Nil(t, (*Job)(nil).LookupGroupSelection("runtime"))
	must.Nil(t, (*Job)(nil).TaskGroupSelection("thor"))
	copy := job.Copy()
	must.True(t, job.GroupSelections[0].Equal(copy.GroupSelections[0]))
	must.False(t, job.SpecChanged(copy))
	copy.GroupSelections[0].Groups[0] = "changed"
	must.Eq(t, "encoder", job.GroupSelections[0].Groups[0])
	must.False(t, job.GroupSelections[0].Equal(copy.GroupSelections[0]))
	must.True(t, job.SpecChanged(copy))
	copy = job.Copy()
	copy.GroupSelections[0].Count = 2
	must.True(t, job.SpecChanged(copy))
	must.Eq(t, 1, job.GroupSelections[0].Count)
	copy = job.Copy()
	copy.GroupSelections[0].Groups[0], copy.GroupSelections[0].Groups[1] = copy.GroupSelections[0].Groups[1], copy.GroupSelections[0].Groups[0]
	must.False(t, job.GroupSelections[0].Equal(copy.GroupSelections[0]))
	must.True(t, job.SpecChanged(copy))
	job.GroupSelections = []*TaskGroupSelection{}
	job.Canonicalize()
	must.Nil(t, job.GroupSelections)
	must.Nil(t, (*TaskGroupSelection)(nil).Copy())
	must.True(t, (*TaskGroupSelection)(nil).Equal(nil))
}

func TestAllocation_GroupSelectionCopy(t *testing.T) {
	ci.Parallel(t)
	alloc := &Allocation{Job: &Job{Type: JobTypeService}, GroupSelection: &AllocationGroupSelection{Name: "runtime", Slot: 0, Cohort: "new"}}
	for _, clone := range []*Allocation{alloc.Copy(), alloc.CopySkipJob()} {
		must.True(t, alloc.GroupSelection.Equal(clone.GroupSelection))
		clone.GroupSelection.Cohort = "changed"
		must.Eq(t, "new", alloc.GroupSelection.Cohort)
		must.False(t, alloc.GroupSelection.Equal(clone.GroupSelection))
	}
	stub := alloc.Stub(nil)
	stub.GroupSelection.Slot = 2
	must.Zero(t, alloc.GroupSelection.Slot)
	must.Nil(t, (*AllocationGroupSelection)(nil).Copy())
	must.True(t, (*AllocationGroupSelection)(nil).Equal(nil))
	must.False(t, alloc.GroupSelection.Equal(nil))
}

func TestDeployment_GroupSelectionCopy(t *testing.T) {
	ci.Parallel(t)
	job := groupSelectionTestJob()
	dep := NewDeployment(job, 50, time.Now().UnixNano())
	must.MapLen(t, 1, dep.GroupSelections)
	selection := dep.GroupSelections["runtime"]
	must.Eq(t, 1, selection.Count)
	must.NotNil(t, selection.Slots)
	must.MapLen(t, 0, selection.Slots)
	selection.Slots[0] = &DeploymentGroupSelectionSlot{TaskGroup: "thor", Cohort: "new", PreviousTaskGroup: "orin", PreviousCohort: "old"}
	clone := dep.Copy()
	must.True(t, selection.Equal(clone.GroupSelections["runtime"]))
	clone.GroupSelections["runtime"].Slots[0].TaskGroup = "encoder"
	must.Eq(t, "thor", selection.Slots[0].TaskGroup)
	must.False(t, selection.Equal(clone.GroupSelections["runtime"]))
	clone.GroupSelections["runtime"].Slots[1] = &DeploymentGroupSelectionSlot{}
	must.MapLen(t, 1, selection.Slots)
	delete(clone.GroupSelections, "runtime")
	must.MapLen(t, 1, dep.GroupSelections)
	must.Nil(t, (*DeploymentGroupSelection)(nil).Copy())
	must.True(t, (*DeploymentGroupSelection)(nil).Equal(nil))
	must.Nil(t, (*DeploymentGroupSelectionSlot)(nil).Copy())
	must.True(t, (*DeploymentGroupSelectionSlot)(nil).Equal(nil))
}

func TestDeployment_GroupSelectionProgressDeadline(t *testing.T) {
	ci.Parallel(t)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		deadlines []time.Duration
		zeroCount int
		count     int
		want      time.Duration
	}{
		{"maximum", []time.Duration{5 * time.Minute, 10 * time.Minute, 2 * time.Minute}, -1, 1, 10 * time.Minute},
		{"disabled candidate", []time.Duration{5 * time.Minute, 10 * time.Minute, 0}, -1, 1, 0},
		{"all disabled", []time.Duration{0, 0, 0}, -1, 1, 0},
		{"ineligible unbounded candidate", []time.Duration{5 * time.Minute, 10 * time.Minute, 0}, 2, 1, 10 * time.Minute},
		{"disabled selection", []time.Duration{5 * time.Minute, 10 * time.Minute, 2 * time.Minute}, -1, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			job := groupSelectionTestJob()
			job.GroupSelections[0].Count = tc.count
			for i, deadline := range tc.deadlines {
				job.TaskGroups[i].Update = &UpdateStrategy{ProgressDeadline: deadline}
			}
			if tc.zeroCount >= 0 {
				job.TaskGroups[tc.zeroCount].Count = 0
			}
			got := NewDeployment(job, 50, now.UnixNano()).GroupSelections["runtime"].RequireProgressBy
			if tc.want == 0 {
				must.True(t, got.IsZero())
			} else {
				must.True(t, now.Add(tc.want).Equal(got))
			}
		})
	}
	job := groupSelectionTestJob()
	job.TaskGroups[0].Update = nil
	must.True(t, NewDeployment(job, 50, now.UnixNano()).GroupSelections["runtime"].RequireProgressBy.IsZero())
	job.GroupSelections = nil
	must.Nil(t, NewDeployment(job, 50, now.UnixNano()).GroupSelections)
}

func TestGroupSelection_BackwardSerialization(t *testing.T) {
	ci.Parallel(t)
	for _, value := range []any{&Job{}, &Allocation{}, &AllocListStub{}, &Deployment{}} {
		data, err := json.Marshal(value)
		must.NoError(t, err)
		must.False(t, strings.Contains(string(data), "GroupSelection"))
		encoded, err := Encode(JobRegisterRequestType, value)
		must.NoError(t, err)
		var fields map[string]any
		must.NoError(t, Decode(encoded[1:], &fields))
		_, hasSelections := fields["GroupSelections"]
		must.False(t, hasSelections)
		_, hasSelection := fields["GroupSelection"]
		must.False(t, hasSelection)
	}
	job := groupSelectionTestJob()
	data, err := Encode(JobRegisterRequestType, job)
	must.NoError(t, err)
	var restored Job
	must.NoError(t, Decode(data[1:], &restored))
	must.True(t, job.GroupSelections[0].Equal(restored.GroupSelections[0]))
	dep := NewDeployment(job, 50, time.Now().UnixNano())
	dep.GroupSelections["runtime"].Slots[0] = &DeploymentGroupSelectionSlot{TaskGroup: "thor", Cohort: "new", PreviousTaskGroup: "orin", PreviousCohort: "old"}
	data, err = Encode(DeploymentStatusUpdateRequestType, dep)
	must.NoError(t, err)
	var restoredDep Deployment
	must.NoError(t, Decode(data[1:], &restoredDep))
	must.True(t, dep.GroupSelections["runtime"].Equal(restoredDep.GroupSelections["runtime"]))
}

func TestJob_GroupSelectionsDiff(t *testing.T) {
	ci.Parallel(t)
	old := groupSelectionTestJob()
	cases := []struct {
		name          string
		change        func(*Job)
		kind          DiffType
		field         string
		before, after string
	}{
		{"count", func(j *Job) { j.GroupSelections[0].Count = 2 }, DiffTypeEdited, "Count", "1", "2"},
		{"reorder", func(j *Job) {
			j.GroupSelections[0].Groups[0], j.GroupSelections[0].Groups[1] = j.GroupSelections[0].Groups[1], j.GroupSelections[0].Groups[0]
		}, DiffTypeEdited, "Groups[0]", "encoder", "orin"},
		{"member removal", func(j *Job) { j.GroupSelections[0].Groups = j.GroupSelections[0].Groups[:2] }, DiffTypeEdited, "Groups[2]", "thor", ""},
		{"selection removal", func(j *Job) { j.GroupSelections = nil }, DiffTypeDeleted, "Name", "runtime", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next := old.Copy()
			tc.change(next)
			for _, contextual := range []bool{false, true} {
				diff, err := old.Diff(next, contextual)
				must.NoError(t, err)
				must.Eq(t, DiffTypeEdited, diff.Type)
				must.Len(t, 1, diff.Objects)
				object := diff.Objects[0]
				must.Eq(t, "GroupSelection", object.Name)
				must.Eq(t, tc.kind, object.Type)
				found := false
				for _, field := range object.Fields {
					if field.Name == tc.field {
						must.Eq(t, tc.before, field.Old)
						must.Eq(t, tc.after, field.New)
						found = true
					}
				}
				must.True(t, found)
			}
		})
	}
	noSelection := old.Copy()
	noSelection.GroupSelections = nil
	added, err := noSelection.Diff(old, false)
	must.NoError(t, err)
	must.Len(t, 1, added.Objects)
	must.Eq(t, DiffTypeAdded, added.Objects[0].Type)
	unchanged, err := old.Diff(old.Copy(), true)
	must.NoError(t, err)
	must.Eq(t, DiffTypeNone, unchanged.Type)
	must.Len(t, 0, unchanged.Objects)
}
