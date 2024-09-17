// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/flatmap"
	"github.com/mitchellh/hashstructure"
)

// DiffableWithID defines an object that has a unique and stable value that can
// be used as an identifier when generating a diff.
type DiffableWithID interface {
	// DiffID returns the value to use to match entities between the old and
	// the new input.
	DiffID() string
}

// DiffType denotes the type of a diff object.
type DiffType string

var (
	DiffTypeNone    DiffType = "None"
	DiffTypeAdded   DiffType = "Added"
	DiffTypeDeleted DiffType = "Deleted"
	DiffTypeEdited  DiffType = "Edited"
)

func (d DiffType) Less(other DiffType) bool {
	// Edited > Added > Deleted > None
	// But we do a reverse sort
	if d == other {
		return false
	}

	if d == DiffTypeEdited {
		return true
	} else if other == DiffTypeEdited {
		return false
	} else if d == DiffTypeAdded {
		return true
	} else if other == DiffTypeAdded {
		return false
	} else if d == DiffTypeDeleted {
		return true
	} else if other == DiffTypeDeleted {
		return false
	}

	return true
}

// JobDiff contains the diff of two jobs.
type JobDiff struct {
	Type       DiffType
	ID         string
	Fields     []*FieldDiff
	Objects    []*ObjectDiff
	TaskGroups []*TaskGroupDiff
}

// Diff returns a diff of two jobs and a potential error if the Jobs are not
// diffable. If contextual diff is enabled, objects within the job will contain
// field information even if unchanged.
func (j *Job) Diff(other *Job, contextual bool) (*JobDiff, error) {
	// See agent.ApiJobToStructJob Update is a default for TaskGroups
	diff := &JobDiff{Type: DiffTypeNone}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string
	filter := []string{"ID", "Status", "StatusDescription", "Version", "Stable", "CreateIndex",
		"ModifyIndex", "JobModifyIndex", "Update", "SubmitTime", "NomadTokenID", "VaultToken"}

	if j == nil && other == nil {
		return diff, nil
	} else if j == nil {
		j = &Job{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(other, filter, true)
		diff.ID = other.ID
	} else if other == nil {
		other = &Job{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(j, filter, true)
		diff.ID = j.ID
	} else {
		if j.ID != other.ID {
			return nil, fmt.Errorf("can not diff jobs with different IDs: %q and %q", j.ID, other.ID)
		}

		oldPrimitiveFlat = flatmap.Flatten(j, filter, true)
		newPrimitiveFlat = flatmap.Flatten(other, filter, true)
		diff.ID = other.ID
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, false)

	// Datacenters diff
	if setDiff := stringSetDiff(j.Datacenters, other.Datacenters, "Datacenters", contextual); setDiff != nil && setDiff.Type != DiffTypeNone {
		diff.Objects = append(diff.Objects, setDiff)
	}

	// Constraints diff
	conDiff := primitiveObjectSetDiff(
		interfaceSlice(j.Constraints),
		interfaceSlice(other.Constraints),
		[]string{"str"},
		"Constraint",
		contextual)
	if conDiff != nil {
		diff.Objects = append(diff.Objects, conDiff...)
	}

	// Affinities diff
	affinitiesDiff := primitiveObjectSetDiff(
		interfaceSlice(j.Affinities),
		interfaceSlice(other.Affinities),
		[]string{"str"},
		"Affinity",
		contextual)
	if affinitiesDiff != nil {
		diff.Objects = append(diff.Objects, affinitiesDiff...)
	}

	// Task groups diff
	tgs, err := taskGroupDiffs(j.TaskGroups, other.TaskGroups, contextual)
	if err != nil {
		return nil, err
	}
	diff.TaskGroups = tgs

	// Periodic diff
	if pDiff := periodicDiff(j.Periodic, other.Periodic, contextual); pDiff != nil {
		diff.Objects = append(diff.Objects, pDiff)
	}

	// ParameterizedJob diff
	if cDiff := parameterizedJobDiff(j.ParameterizedJob, other.ParameterizedJob, contextual); cDiff != nil {
		diff.Objects = append(diff.Objects, cDiff)
	}

	// Multiregion diff
	if mrDiff := multiregionDiff(j.Multiregion, other.Multiregion, contextual); mrDiff != nil {
		diff.Objects = append(diff.Objects, mrDiff)
	}

	// UI diff
	if uiDiff := uiDiff(j.UI, other.UI, contextual); uiDiff != nil {
		diff.Objects = append(diff.Objects, uiDiff)
	}

	// Check to see if there is a diff. We don't use reflect because we are
	// filtering quite a few fields that will change on each diff.
	if diff.Type == DiffTypeNone {
		for _, fd := range diff.Fields {
			if fd.Type != DiffTypeNone {
				diff.Type = DiffTypeEdited
				break
			}
		}
	}

	if diff.Type == DiffTypeNone {
		for _, od := range diff.Objects {
			if od.Type != DiffTypeNone {
				diff.Type = DiffTypeEdited
				break
			}
		}
	}

	if diff.Type == DiffTypeNone {
		for _, tg := range diff.TaskGroups {
			if tg.Type != DiffTypeNone {
				diff.Type = DiffTypeEdited
				break
			}
		}
	}

	return diff, nil
}

func (j *JobDiff) GoString() string {
	out := fmt.Sprintf("Job %q (%s):\n", j.ID, j.Type)

	for _, f := range j.Fields {
		out += fmt.Sprintf("%#v\n", f)
	}

	for _, o := range j.Objects {
		out += fmt.Sprintf("%#v\n", o)
	}

	for _, tg := range j.TaskGroups {
		out += fmt.Sprintf("%#v\n", tg)
	}

	return out
}

// TaskGroupDiff contains the diff of two task groups.
type TaskGroupDiff struct {
	Type    DiffType
	Name    string
	Fields  []*FieldDiff
	Objects []*ObjectDiff
	Tasks   []*TaskDiff
	Updates map[string]uint64
}

// Diff returns a diff of two task groups. If contextual diff is enabled,
// objects' fields will be stored even if no diff occurred as long as one field
// changed.
func (tg *TaskGroup) Diff(other *TaskGroup, contextual bool) (*TaskGroupDiff, error) {
	diff := &TaskGroupDiff{Type: DiffTypeNone}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string
	filter := []string{"Name"}

	if tg == nil && other == nil {
		return diff, nil
	} else if tg == nil {
		tg = &TaskGroup{}
		diff.Type = DiffTypeAdded
		diff.Name = other.Name
		newPrimitiveFlat = flatmap.Flatten(other, filter, true)
	} else if other == nil {
		other = &TaskGroup{}
		diff.Type = DiffTypeDeleted
		diff.Name = tg.Name
		oldPrimitiveFlat = flatmap.Flatten(tg, filter, true)
	} else {
		if !reflect.DeepEqual(tg, other) {
			diff.Type = DiffTypeEdited
		}
		if tg.Name != other.Name {
			return nil, fmt.Errorf("can not diff task groups with different names: %q and %q", tg.Name, other.Name)
		}
		diff.Name = other.Name
		oldPrimitiveFlat = flatmap.Flatten(tg, filter, true)
		newPrimitiveFlat = flatmap.Flatten(other, filter, true)
	}

	// ShutdownDelay diff
	if oldPrimitiveFlat != nil && newPrimitiveFlat != nil {
		if tg.ShutdownDelay == nil {
			oldPrimitiveFlat["ShutdownDelay"] = ""
		} else {
			oldPrimitiveFlat["ShutdownDelay"] = fmt.Sprintf("%d", *tg.ShutdownDelay)
		}
		if other.ShutdownDelay == nil {
			newPrimitiveFlat["ShutdownDelay"] = ""
		} else {
			newPrimitiveFlat["ShutdownDelay"] = fmt.Sprintf("%d", *other.ShutdownDelay)
		}
	}

	// StopAfterClientDisconnect diff
	if oldPrimitiveFlat != nil && newPrimitiveFlat != nil {
		if tg.StopAfterClientDisconnect == nil {
			oldPrimitiveFlat["StopAfterClientDisconnect"] = ""
		} else {
			oldPrimitiveFlat["StopAfterClientDisconnect"] = fmt.Sprintf("%d", *tg.StopAfterClientDisconnect)
		}
		if other.StopAfterClientDisconnect == nil {
			newPrimitiveFlat["StopAfterClientDisconnect"] = ""
		} else {
			newPrimitiveFlat["StopAfterClientDisconnect"] = fmt.Sprintf("%d", *other.StopAfterClientDisconnect)
		}
	}

	// MaxClientDisconnect diff
	if oldPrimitiveFlat != nil && newPrimitiveFlat != nil {
		if tg.MaxClientDisconnect == nil {
			oldPrimitiveFlat["MaxClientDisconnect"] = ""
		} else {
			oldPrimitiveFlat["MaxClientDisconnect"] = fmt.Sprintf("%d", *tg.MaxClientDisconnect)
		}
		if other.MaxClientDisconnect == nil {
			newPrimitiveFlat["MaxClientDisconnect"] = ""
		} else {
			newPrimitiveFlat["MaxClientDisconnect"] = fmt.Sprintf("%d", *other.MaxClientDisconnect)
		}
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, false)

	// Constraints diff
	conDiff := primitiveObjectSetDiff(
		interfaceSlice(tg.Constraints),
		interfaceSlice(other.Constraints),
		[]string{"str"},
		"Constraint",
		contextual)
	if conDiff != nil {
		diff.Objects = append(diff.Objects, conDiff...)
	}

	// Affinities diff
	affinitiesDiff := primitiveObjectSetDiff(
		interfaceSlice(tg.Affinities),
		interfaceSlice(other.Affinities),
		[]string{"str"},
		"Affinity",
		contextual)
	if affinitiesDiff != nil {
		diff.Objects = append(diff.Objects, affinitiesDiff...)
	}

	// Restart policy diff
	rDiff := primitiveObjectDiff(tg.RestartPolicy, other.RestartPolicy, nil, "RestartPolicy", contextual)
	if rDiff != nil {
		diff.Objects = append(diff.Objects, rDiff)
	}

	// Reschedule policy diff
	reschedDiff := primitiveObjectDiff(tg.ReschedulePolicy, other.ReschedulePolicy, nil, "ReschedulePolicy", contextual)
	if reschedDiff != nil {
		diff.Objects = append(diff.Objects, reschedDiff)
	}

	// EphemeralDisk diff
	diskDiff := primitiveObjectDiff(tg.EphemeralDisk, other.EphemeralDisk, nil, "EphemeralDisk", contextual)
	if diskDiff != nil {
		diff.Objects = append(diff.Objects, diskDiff)
	}

	consulDiff := primitiveObjectDiff(tg.Consul, other.Consul, nil, "Consul", contextual)
	if consulDiff != nil {
		diff.Objects = append(diff.Objects, consulDiff)
	}

	// Update diff
	// COMPAT: Remove "Stagger" in 0.7.0.
	if uDiff := primitiveObjectDiff(tg.Update, other.Update, []string{"Stagger"}, "Update", contextual); uDiff != nil {
		diff.Objects = append(diff.Objects, uDiff)
	}

	// Disconnect diff
	if disconnectDiff := disconectStrategyDiffs(tg.Disconnect, other.Disconnect, contextual); disconnectDiff != nil {
		diff.Objects = append(diff.Objects, disconnectDiff)
	}

	// Network Resources diff
	if nDiffs := networkResourceDiffs(tg.Networks, other.Networks, contextual); nDiffs != nil {
		diff.Objects = append(diff.Objects, nDiffs...)
	}

	// Scaling diff
	if scDiff := scalingDiff(tg.Scaling, other.Scaling, contextual); scDiff != nil {
		diff.Objects = append(diff.Objects, scDiff)
	}

	// Services diff
	if sDiffs := serviceDiffs(tg.Services, other.Services, contextual); sDiffs != nil {
		diff.Objects = append(diff.Objects, sDiffs...)
	}

	// Volumes diff
	if vDiffs := volumeDiffs(tg.Volumes, other.Volumes, contextual); vDiffs != nil {
		diff.Objects = append(diff.Objects, vDiffs...)
	}

	// Tasks diff
	tasks, err := taskDiffs(tg.Tasks, other.Tasks, contextual)
	if err != nil {
		return nil, err
	}
	diff.Tasks = tasks

	return diff, nil
}

func (tg *TaskGroupDiff) GoString() string {
	out := fmt.Sprintf("Group %q (%s):\n", tg.Name, tg.Type)

	if len(tg.Updates) != 0 {
		out += "Updates {\n"
		for update, count := range tg.Updates {
			out += fmt.Sprintf("%d %s\n", count, update)
		}
		out += "}\n"
	}

	for _, f := range tg.Fields {
		out += fmt.Sprintf("%#v\n", f)
	}

	for _, o := range tg.Objects {
		out += fmt.Sprintf("%#v\n", o)
	}

	for _, t := range tg.Tasks {
		out += fmt.Sprintf("%#v\n", t)
	}

	return out
}

// TaskGroupDiffs diffs two sets of task groups. If contextual diff is enabled,
// objects' fields will be stored even if no diff occurred as long as one field
// changed.
func taskGroupDiffs(old, new []*TaskGroup, contextual bool) ([]*TaskGroupDiff, error) {
	oldMap := make(map[string]*TaskGroup, len(old))
	newMap := make(map[string]*TaskGroup, len(new))
	for _, o := range old {
		oldMap[o.Name] = o
	}
	for _, n := range new {
		newMap[n.Name] = n
	}

	var diffs []*TaskGroupDiff
	for name, oldGroup := range oldMap {
		// Diff the same, deleted and edited
		diff, err := oldGroup.Diff(newMap[name], contextual)
		if err != nil {
			return nil, err
		}
		diffs = append(diffs, diff)
	}

	for name, newGroup := range newMap {
		// Diff the added
		if old, ok := oldMap[name]; !ok {
			diff, err := old.Diff(newGroup, contextual)
			if err != nil {
				return nil, err
			}
			diffs = append(diffs, diff)
		}
	}

	sort.Sort(TaskGroupDiffs(diffs))
	return diffs, nil
}

// For sorting TaskGroupDiffs
type TaskGroupDiffs []*TaskGroupDiff

func (tg TaskGroupDiffs) Len() int           { return len(tg) }
func (tg TaskGroupDiffs) Swap(i, j int)      { tg[i], tg[j] = tg[j], tg[i] }
func (tg TaskGroupDiffs) Less(i, j int) bool { return tg[i].Name < tg[j].Name }

// TaskDiff contains the diff of two Tasks
type TaskDiff struct {
	Type        DiffType
	Name        string
	Fields      []*FieldDiff
	Objects     []*ObjectDiff
	Annotations []string
}

// Diff returns a diff of two tasks. If contextual diff is enabled, objects
// within the task will contain field information even if unchanged.
func (t *Task) Diff(other *Task, contextual bool) (*TaskDiff, error) {
	diff := &TaskDiff{Type: DiffTypeNone}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string
	filter := []string{"Name", "Config"}

	if t == nil && other == nil {
		return diff, nil
	} else if t == nil {
		t = &Task{}
		diff.Type = DiffTypeAdded
		diff.Name = other.Name
		newPrimitiveFlat = flatmap.Flatten(other, filter, true)
	} else if other == nil {
		other = &Task{}
		diff.Type = DiffTypeDeleted
		diff.Name = t.Name
		oldPrimitiveFlat = flatmap.Flatten(t, filter, true)
	} else {
		if !reflect.DeepEqual(t, other) {
			diff.Type = DiffTypeEdited
		}
		if t.Name != other.Name {
			return nil, fmt.Errorf("can not diff tasks with different names: %q and %q", t.Name, other.Name)
		}
		diff.Name = other.Name
		oldPrimitiveFlat = flatmap.Flatten(t, filter, true)
		newPrimitiveFlat = flatmap.Flatten(other, filter, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, false)

	// Constraints diff
	conDiff := primitiveObjectSetDiff(
		interfaceSlice(t.Constraints),
		interfaceSlice(other.Constraints),
		[]string{"str"},
		"Constraint",
		contextual)
	if conDiff != nil {
		diff.Objects = append(diff.Objects, conDiff...)
	}

	// Affinities diff
	affinitiesDiff := primitiveObjectSetDiff(
		interfaceSlice(t.Affinities),
		interfaceSlice(other.Affinities),
		[]string{"str"},
		"Affinity",
		contextual)
	if affinitiesDiff != nil {
		diff.Objects = append(diff.Objects, affinitiesDiff...)
	}

	// Config diff
	if cDiff := configDiff(t.Config, other.Config, contextual); cDiff != nil {
		diff.Objects = append(diff.Objects, cDiff)
	}

	// Resources diff
	if rDiff := t.Resources.Diff(other.Resources, contextual); rDiff != nil {
		diff.Objects = append(diff.Objects, rDiff)
	}

	// LogConfig diff
	lDiff := primitiveObjectDiff(t.LogConfig, other.LogConfig, nil, "LogConfig", contextual)
	if lDiff != nil {
		diff.Objects = append(diff.Objects, lDiff)
	}

	// Dispatch payload diff
	dDiff := primitiveObjectDiff(t.DispatchPayload, other.DispatchPayload, nil, "DispatchPayload", contextual)
	if dDiff != nil {
		diff.Objects = append(diff.Objects, dDiff)
	}

	// Artifacts diff
	diffs := primitiveObjectSetDiff(
		interfaceSlice(t.Artifacts),
		interfaceSlice(other.Artifacts),
		nil,
		"Artifact",
		contextual)
	if diffs != nil {
		diff.Objects = append(diff.Objects, diffs...)
	}

	// Services diff
	if sDiffs := serviceDiffs(t.Services, other.Services, contextual); sDiffs != nil {
		diff.Objects = append(diff.Objects, sDiffs...)
	}

	// Vault diff
	vDiff := vaultDiff(t.Vault, other.Vault, contextual)
	if vDiff != nil {
		diff.Objects = append(diff.Objects, vDiff)
	}

	// Consul diff
	consulDiff := primitiveObjectDiff(t.Consul, other.Consul, nil, "Consul", contextual)
	if consulDiff != nil {
		diff.Objects = append(diff.Objects, consulDiff)
	}

	// Template diff
	tmplDiffs := templateDiffs(t.Templates, other.Templates, contextual)
	if tmplDiffs != nil {
		diff.Objects = append(diff.Objects, tmplDiffs...)
	}

	// Identity diff
	idDiffs := idDiff(t.Identity, other.Identity, contextual)
	if idDiffs != nil {
		diff.Objects = append(diff.Objects, idDiffs)
	}

	// Alternate identities diff
	if altIDDiffs := idSliceDiffs(t.Identities, other.Identities, contextual); altIDDiffs != nil {
		diff.Objects = append(diff.Objects, altIDDiffs...)
	}

	// Actions diff
	if aDiffs := actionDiffs(t.Actions, other.Actions, contextual); aDiffs != nil {
		diff.Objects = append(diff.Objects, aDiffs...)
	}

	// volume_mount diff
	if vDiffs := volumeMountsDiffs(t.VolumeMounts, other.VolumeMounts, contextual); vDiffs != nil {
		diff.Objects = append(diff.Objects, vDiffs...)
	}

	// Schedule diff
	if sDiff := scheduleDiff(t.Schedule, other.Schedule, contextual); sDiff != nil {
		diff.Objects = append(diff.Objects, sDiff)
	}

	return diff, nil
}

func actionDiff(old, new *Action, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Action"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		old = &Action{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	} else if new == nil {
		new = &Action{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	}

	// Diff the primitive fields
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Diff the Args field using stringSetDiff
	if setDiff := stringSetDiff(old.Args, new.Args, "Args", contextual); setDiff != nil {
		diff.Objects = append(diff.Objects, setDiff)
	}

	return diff
}

// actionDiffs diffs a set of actions. If contextual diff is enabled, unchanged
// fields within objects nested in the actions will be returned.
func actionDiffs(old, new []*Action, contextual bool) []*ObjectDiff {
	var diffs []*ObjectDiff

	for i := 0; i < len(old) && i < len(new); i++ {
		oldAction := old[i]
		newAction := new[i]

		if diff := actionDiff(oldAction, newAction, contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}

	for i := len(new); i < len(old); i++ {
		if diff := actionDiff(old[i], nil, contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}

	for i := len(old); i < len(new); i++ {
		if diff := actionDiff(nil, new[i], contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}

	sort.Sort(ObjectDiffs(diffs))

	return diffs
}

func scheduleDiff(old, new *TaskSchedule, contextual bool) *ObjectDiff {
	if reflect.DeepEqual(old, new) {
		return nil
	}
	if old == nil {
		old = &TaskSchedule{}
	}
	if new == nil {
		new = &TaskSchedule{}
	}
	return primitiveObjectDiff(old.Cron, new.Cron, nil, "Schedule", contextual)
}

func (t *TaskDiff) GoString() string {
	var out string
	if len(t.Annotations) == 0 {
		out = fmt.Sprintf("Task %q (%s):\n", t.Name, t.Type)
	} else {
		out = fmt.Sprintf("Task %q (%s) (%s):\n", t.Name, t.Type, strings.Join(t.Annotations, ","))
	}

	for _, f := range t.Fields {
		out += fmt.Sprintf("%#v\n", f)
	}

	for _, o := range t.Objects {
		out += fmt.Sprintf("%#v\n", o)
	}

	return out
}

// taskDiffs diffs a set of tasks. If contextual diff is enabled, unchanged
// fields within objects nested in the tasks will be returned.
func taskDiffs(old, new []*Task, contextual bool) ([]*TaskDiff, error) {
	oldMap := make(map[string]*Task, len(old))
	newMap := make(map[string]*Task, len(new))
	for _, o := range old {
		oldMap[o.Name] = o
	}
	for _, n := range new {
		newMap[n.Name] = n
	}

	var diffs []*TaskDiff
	for name, oldGroup := range oldMap {
		// Diff the same, deleted and edited
		diff, err := oldGroup.Diff(newMap[name], contextual)
		if err != nil {
			return nil, err
		}
		diffs = append(diffs, diff)
	}

	for name, newGroup := range newMap {
		// Diff the added
		if old, ok := oldMap[name]; !ok {
			diff, err := old.Diff(newGroup, contextual)
			if err != nil {
				return nil, err
			}
			diffs = append(diffs, diff)
		}
	}

	sort.Sort(TaskDiffs(diffs))
	return diffs, nil
}

// For sorting TaskDiffs
type TaskDiffs []*TaskDiff

func (t TaskDiffs) Len() int           { return len(t) }
func (t TaskDiffs) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t TaskDiffs) Less(i, j int) bool { return t[i].Name < t[j].Name }

// scalingDiff returns the diff of two Scaling objects. If contextual diff is enabled, unchanged
// fields within objects nested in the tasks will be returned.
func scalingDiff(old, new *ScalingPolicy, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Scaling"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	filter := []string{"CreateIndex", "ModifyIndex", "ID", "Type", "Target[Job]", "Target[Group]", "Target[Namespace]"}

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		old = &ScalingPolicy{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(new, filter, true)
	} else if new == nil {
		new = &ScalingPolicy{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(old, filter, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(old, filter, true)
		newPrimitiveFlat = flatmap.Flatten(new, filter, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, false)

	// Diff Policy
	if pDiff := policyDiff(old.Policy, new.Policy, contextual); pDiff != nil {
		diff.Objects = append(diff.Objects, pDiff)
	}

	sort.Sort(FieldDiffs(diff.Fields))
	sort.Sort(ObjectDiffs(diff.Objects))

	return diff
}

// policyDiff returns the diff of two Scaling Policy objects. If contextual diff is enabled, unchanged
// fields within objects nested in the tasks will be returned.
func policyDiff(old, new map[string]interface{}, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Policy"}
	if reflect.DeepEqual(old, new) {
		return nil
	} else if len(old) == 0 {
		diff.Type = DiffTypeAdded
	} else if len(new) == 0 {
		diff.Type = DiffTypeDeleted
	} else {
		diff.Type = DiffTypeEdited
	}

	// Diff the primitive fields.
	oldPrimitiveFlat := flatmap.Flatten(old, nil, false)
	newPrimitiveFlat := flatmap.Flatten(new, nil, false)
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, false)

	return diff
}

// serviceDiff returns the diff of two service objects. If contextual diff is
// enabled, all fields will be returned, even if no diff occurred.
func serviceDiff(old, new *Service, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Service"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		old = &Service{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	} else if new == nil {
		new = &Service{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	if setDiff := stringSetDiff(old.CanaryTags, new.CanaryTags, "CanaryTags", contextual); setDiff != nil {
		diff.Objects = append(diff.Objects, setDiff)
	}

	// Tag diffs
	if setDiff := stringSetDiff(old.Tags, new.Tags, "Tags", contextual); setDiff != nil {
		diff.Objects = append(diff.Objects, setDiff)
	}

	// Checks diffs
	if cDiffs := serviceCheckDiffs(old.Checks, new.Checks, contextual); cDiffs != nil {
		diff.Objects = append(diff.Objects, cDiffs...)
	}

	// Consul Connect diffs
	if conDiffs := connectDiffs(old.Connect, new.Connect, contextual); conDiffs != nil {
		diff.Objects = append(diff.Objects, conDiffs)
	}

	// Workload Identity diffs
	if wiDiffs := idDiff(old.Identity, new.Identity, contextual); wiDiffs != nil {
		diff.Objects = append(diff.Objects, wiDiffs)
	}

	return diff
}

// serviceDiffs diffs a set of services. If contextual diff is enabled, unchanged
// fields within objects nested in the tasks will be returned.
func serviceDiffs(old, new []*Service, contextual bool) []*ObjectDiff {
	// Handle trivial case.
	if len(old) == 1 && len(new) == 1 {
		if diff := serviceDiff(old[0], new[0], contextual); diff != nil {
			return []*ObjectDiff{diff}
		}
		return nil
	}

	// For each service we will try to find a corresponding match in the other
	// service list.
	// The following lists store the index of the matching service for each
	// position of the inputs.
	oldMatches := make([]int, len(old))
	newMatches := make([]int, len(new))

	// Initialize all services as unmatched.
	for i := range oldMatches {
		oldMatches[i] = -1
	}
	for i := range newMatches {
		newMatches[i] = -1
	}

	// Find a match in the new services list for each old service and compute
	// their diffs.
	var diffs []*ObjectDiff
	for oldIndex, oldService := range old {
		newIndex := findServiceMatch(oldService, oldIndex, new, newMatches)

		// Old services that don't have a match were deleted.
		if newIndex < 0 {
			diff := serviceDiff(oldService, nil, contextual)
			diffs = append(diffs, diff)
			continue
		}

		// If A matches B then B matches A.
		oldMatches[oldIndex] = newIndex
		newMatches[newIndex] = oldIndex

		newService := new[newIndex]
		if diff := serviceDiff(oldService, newService, contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}

	// New services without match were added.
	for i, m := range newMatches {
		if m == -1 {
			diff := serviceDiff(nil, new[i], contextual)
			diffs = append(diffs, diff)
		}
	}

	sort.Sort(ObjectDiffs(diffs))
	return diffs
}

// findServiceMatch returns the index of the service in the input services list
// that matches the provided input service.
func findServiceMatch(service *Service, serviceIndex int, services []*Service, matches []int) int {
	// minScoreThreshold can be adjusted to generate more (lower value) or
	// fewer (higher value) matches.
	// More matches result in more Edited diffs, while fewer matches generate
	// more Add/Delete diff pairs.
	minScoreThreshold := 2

	highestScore := 0
	indexMatch := -1

	for i, s := range services {
		// Skip service if it's already matched.
		if matches[i] >= 0 {
			continue
		}

		// Finding a perfect match by just looking at the before and after
		// list of services is impossible since they don't have a stable
		// identifier that can be used to uniquely identify them.
		//
		// Users also have an implicit temporal intuition of which services
		// match each other when editing their jobspec file. If they move the
		// 3rd service to the top, they don't expect their job to change.
		//
		// This intuition could be made explicit by requiring a user-defined
		// unique identifier, but this would cause additional work and the
		// new field would not be intuitive for users to understand how to use
		// it.
		//
		// Using a hash value of the service content will cause any changes to
		// create a delete/add diff pair.
		//
		// There are three main candidates for a service ID:
		//   - name, but they are not unique and can be modified.
		//   - label port, but they have the same problems as name.
		//   - service position within the overall list of services, but if the
		//     service block is moved, it will impact all services that come
		//     after it.
		//
		// None of these values are enough on their own, but they are also too
		// strong when considered all together.
		//
		// So we try to score services by their main candidates with a preference
		// towards name + label over service position.
		score := 0
		if i == serviceIndex {
			score += 1
		}

		if service.PortLabel == s.PortLabel {
			score += 2
		}

		if service.Name == s.Name {
			score += 3
		}

		if score > minScoreThreshold && score > highestScore {
			highestScore = score
			indexMatch = i
		}
	}

	return indexMatch
}

// serviceCheckDiff returns the diff of two service check objects. If contextual
// diff is enabled, all fields will be returned, even if no diff occurred.
func serviceCheckDiff(old, new *ServiceCheck, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Check"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		old = &ServiceCheck{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	} else if new == nil {
		new = &ServiceCheck{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Diff Header
	if headerDiff := checkHeaderDiff(old.Header, new.Header, contextual); headerDiff != nil {
		diff.Objects = append(diff.Objects, headerDiff)
	}

	// Diff check_restart
	if crDiff := checkRestartDiff(old.CheckRestart, new.CheckRestart, contextual); crDiff != nil {
		diff.Objects = append(diff.Objects, crDiff)
	}

	return diff
}

// checkHeaderDiff returns the diff of two service check header objects. If
// contextual diff is enabled, all fields will be returned, even if no diff
// occurred.
func checkHeaderDiff(old, new map[string][]string, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Header"}
	var oldFlat, newFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if len(old) == 0 {
		diff.Type = DiffTypeAdded
		newFlat = flatmap.Flatten(new, nil, false)
	} else if len(new) == 0 {
		diff.Type = DiffTypeDeleted
		oldFlat = flatmap.Flatten(old, nil, false)
	} else {
		diff.Type = DiffTypeEdited
		oldFlat = flatmap.Flatten(old, nil, false)
		newFlat = flatmap.Flatten(new, nil, false)
	}

	diff.Fields = fieldDiffs(oldFlat, newFlat, contextual)
	return diff
}

// checkRestartDiff returns the diff of two service check check_restart
// objects. If contextual diff is enabled, all fields will be returned, even if
// no diff occurred.
func checkRestartDiff(old, new *CheckRestart, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "CheckRestart"}
	var oldFlat, newFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		diff.Type = DiffTypeAdded
		newFlat = flatmap.Flatten(new, nil, true)
		diff.Type = DiffTypeAdded
	} else if new == nil {
		diff.Type = DiffTypeDeleted
		oldFlat = flatmap.Flatten(old, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldFlat = flatmap.Flatten(old, nil, true)
		newFlat = flatmap.Flatten(new, nil, true)
	}

	diff.Fields = fieldDiffs(oldFlat, newFlat, contextual)
	return diff
}

// connectDiffs returns the diff of two Consul connect objects. If contextual
// diff is enabled, all fields will be returned, even if no diff occurred.
func connectDiffs(old, new *ConsulConnect, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "ConsulConnect"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		old = &ConsulConnect{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	} else if new == nil {
		new = &ConsulConnect{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Diff the object field SidecarService.
	sidecarSvcDiff := connectSidecarServiceDiff(old.SidecarService, new.SidecarService, contextual)
	if sidecarSvcDiff != nil {
		diff.Objects = append(diff.Objects, sidecarSvcDiff)
	}

	// Diff the object field SidecarTask.
	sidecarTaskDiff := sidecarTaskDiff(old.SidecarTask, new.SidecarTask, contextual)
	if sidecarTaskDiff != nil {
		diff.Objects = append(diff.Objects, sidecarTaskDiff)
	}

	// Diff the object field ConsulGateway.
	gatewayDiff := connectGatewayDiff(old.Gateway, new.Gateway, contextual)
	if gatewayDiff != nil {
		diff.Objects = append(diff.Objects, gatewayDiff)
	}

	return diff
}

func connectGatewayDiff(prev, next *ConsulGateway, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Gateway"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(prev, next) {
		return nil
	} else if prev == nil {
		prev = new(ConsulGateway)
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	} else if next == nil {
		next = new(ConsulGateway)
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Diff the ConsulGatewayProxy fields.
	gatewayProxyDiff := connectGatewayProxyDiff(prev.Proxy, next.Proxy, contextual)
	if gatewayProxyDiff != nil {
		diff.Objects = append(diff.Objects, gatewayProxyDiff)
	}

	// Diff the ingress gateway fields.
	gatewayIngressDiff := connectGatewayIngressDiff(prev.Ingress, next.Ingress, contextual)
	if gatewayIngressDiff != nil {
		diff.Objects = append(diff.Objects, gatewayIngressDiff)
	}

	//  Diff the terminating gateway fields.
	gatewayTerminatingDiff := connectGatewayTerminatingDiff(prev.Terminating, next.Terminating, contextual)
	if gatewayTerminatingDiff != nil {
		diff.Objects = append(diff.Objects, gatewayTerminatingDiff)
	}

	// Diff the mesh gateway fields.
	gatewayMeshDiff := connectGatewayMeshDiff(prev.Mesh, next.Mesh, contextual)
	if gatewayMeshDiff != nil {
		diff.Objects = append(diff.Objects, gatewayMeshDiff)
	}

	return diff
}

func connectGatewayMeshDiff(prev, next *ConsulMeshConfigEntry, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Mesh"}

	if reflect.DeepEqual(prev, next) {
		return nil
	} else if prev == nil {
		// no fields to further diff
		diff.Type = DiffTypeAdded
	} else if next == nil {
		// no fields to further diff
		diff.Type = DiffTypeDeleted
	} else {
		diff.Type = DiffTypeEdited
	}

	// Currently no fields in mesh gateways.

	return diff
}

func connectGatewayIngressDiff(prev, next *ConsulIngressConfigEntry, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Ingress"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(prev, next) {
		return nil
	} else if prev == nil {
		prev = new(ConsulIngressConfigEntry)
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	} else if next == nil {
		next = new(ConsulIngressConfigEntry)
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Diff the ConsulGatewayTLSConfig objects.
	tlsConfigDiff := connectGatewayTLSConfigDiff(prev.TLS, next.TLS, contextual)
	if tlsConfigDiff != nil {
		diff.Objects = append(diff.Objects, tlsConfigDiff)
	}

	// Diff the Listeners lists.
	gatewayIngressListenersDiff := connectGatewayIngressListenersDiff(prev.Listeners, next.Listeners, contextual)
	if gatewayIngressListenersDiff != nil {
		diff.Objects = append(diff.Objects, gatewayIngressListenersDiff...)
	}

	return diff
}

func connectGatewayTerminatingDiff(prev, next *ConsulTerminatingConfigEntry, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Terminating"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(prev, next) {
		return nil
	} else if prev == nil {
		prev = new(ConsulTerminatingConfigEntry)
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	} else if next == nil {
		next = new(ConsulTerminatingConfigEntry)
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Diff the Services lists.
	gatewayLinkedServicesDiff := connectGatewayTerminatingLinkedServicesDiff(prev.Services, next.Services, contextual)
	if gatewayLinkedServicesDiff != nil {
		diff.Objects = append(diff.Objects, gatewayLinkedServicesDiff...)
	}

	return diff
}

// connectGatewayTerminatingLinkedServicesDiff diffs are a set of services keyed
// by service name. These objects contain only fields.
func connectGatewayTerminatingLinkedServicesDiff(prev, next []*ConsulLinkedService, contextual bool) []*ObjectDiff {
	// create maps, diff the maps, key by linked service name

	prevMap := make(map[string]*ConsulLinkedService, len(prev))
	nextMap := make(map[string]*ConsulLinkedService, len(next))

	for _, s := range prev {
		prevMap[s.Name] = s
	}
	for _, s := range next {
		nextMap[s.Name] = s
	}

	var diffs []*ObjectDiff
	for k, prevS := range prevMap {
		// Diff the same, deleted, and edited
		if diff := connectGatewayTerminatingLinkedServiceDiff(prevS, nextMap[k], contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}
	for k, nextS := range nextMap {
		// Diff the added
		if old, ok := prevMap[k]; !ok {
			if diff := connectGatewayTerminatingLinkedServiceDiff(old, nextS, contextual); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}

	sort.Sort(ObjectDiffs(diffs))
	return diffs
}

func connectGatewayTerminatingLinkedServiceDiff(prev, next *ConsulLinkedService, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Service"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(prev, next) {
		return nil
	} else if prev == nil {
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	} else if next == nil {
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// No objects today.

	return diff
}

func connectGatewayTLSConfigDiff(prev, next *ConsulGatewayTLSConfig, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "TLS"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(prev, next) {
		return nil
	} else if prev == nil {
		prev = &ConsulGatewayTLSConfig{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	} else if next == nil {
		next = &ConsulGatewayTLSConfig{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	}

	// CipherSuites diffs
	if setDiff := stringSetDiff(prev.CipherSuites, next.CipherSuites, "CipherSuites", contextual); setDiff != nil {
		diff.Objects = append(diff.Objects, setDiff)
	}

	// Diff the primitive field.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Diff SDS object
	if sdsDiff := primitiveObjectDiff(prev.SDS, next.SDS, nil, "SDS", contextual); sdsDiff != nil {
		diff.Objects = append(diff.Objects, sdsDiff)
	}

	return diff
}

// connectGatewayIngressListenersDiff diffs are a set of listeners keyed by "protocol/port", which is
// a nifty workaround having slices instead of maps. Presumably such a key will be unique, because if
// if is not the config entry is not going to work anyway.
func connectGatewayIngressListenersDiff(prev, next []*ConsulIngressListener, contextual bool) []*ObjectDiff {
	//  create maps, diff the maps, keys are fields, keys are (port+protocol)

	key := func(l *ConsulIngressListener) string {
		return fmt.Sprintf("%s/%d", l.Protocol, l.Port)
	}

	prevMap := make(map[string]*ConsulIngressListener, len(prev))
	nextMap := make(map[string]*ConsulIngressListener, len(next))

	for _, l := range prev {
		prevMap[key(l)] = l
	}
	for _, l := range next {
		nextMap[key(l)] = l
	}

	var diffs []*ObjectDiff
	for k, prevL := range prevMap {
		// Diff the same, deleted, and edited
		if diff := connectGatewayIngressListenerDiff(prevL, nextMap[k], contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}
	for k, nextL := range nextMap {
		// Diff the added
		if old, ok := prevMap[k]; !ok {
			if diff := connectGatewayIngressListenerDiff(old, nextL, contextual); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}

	sort.Sort(ObjectDiffs(diffs))
	return diffs
}

func connectGatewayIngressListenerDiff(prev, next *ConsulIngressListener, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Listener"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(prev, next) {
		return nil
	} else if prev == nil {
		prev = new(ConsulIngressListener)
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	} else if next == nil {
		next = new(ConsulIngressListener)
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Diff the Ingress Service objects.
	if diffs := connectGatewayIngressServicesDiff(prev.Services, next.Services, contextual); diffs != nil {
		diff.Objects = append(diff.Objects, diffs...)
	}

	return diff
}

// connectGatewayIngressServicesDiff diffs are a set of ingress services keyed by their service name, which
// is a workaround for having slices instead of maps. Presumably the service name is a unique key, because if
// no the config entry is not going to make sense anyway.
func connectGatewayIngressServicesDiff(prev, next []*ConsulIngressService, contextual bool) []*ObjectDiff {

	prevMap := make(map[string]*ConsulIngressService, len(prev))
	nextMap := make(map[string]*ConsulIngressService, len(next))

	for _, s := range prev {
		prevMap[s.Name] = s
	}
	for _, s := range next {
		nextMap[s.Name] = s
	}

	var diffs []*ObjectDiff
	for name, oldIS := range prevMap {
		// Diff the same, deleted, and edited
		if diff := connectGatewayIngressServiceDiff(oldIS, nextMap[name], contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}
	for name, newIS := range nextMap {
		// Diff the added
		if old, ok := prevMap[name]; !ok {
			if diff := connectGatewayIngressServiceDiff(old, newIS, contextual); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}

	sort.Sort(ObjectDiffs(diffs))
	return diffs
}

func connectGatewayIngressServiceDiff(prev, next *ConsulIngressService, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "ConsulIngressService"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(prev, next) {
		return nil
	} else if prev == nil {
		prev = new(ConsulIngressService)
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	} else if next == nil {
		next = new(ConsulIngressService)
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	}

	// Diff pointer types.
	if prev != nil {
		if prev.MaxConnections != nil {
			oldPrimitiveFlat["MaxConnections"] = fmt.Sprintf("%v", *prev.MaxConnections)
		}
	}
	if next != nil {
		if next.MaxConnections != nil {
			newPrimitiveFlat["MaxConnections"] = fmt.Sprintf("%v", *next.MaxConnections)
		}
	}
	if prev != nil {
		if prev.MaxPendingRequests != nil {
			oldPrimitiveFlat["MaxPendingRequests"] = fmt.Sprintf("%v", *prev.MaxPendingRequests)
		}
	}
	if next != nil {
		if next.MaxPendingRequests != nil {
			newPrimitiveFlat["MaxPendingRequests"] = fmt.Sprintf("%v", *next.MaxPendingRequests)
		}
	}
	if prev != nil {
		if prev.MaxConcurrentRequests != nil {
			oldPrimitiveFlat["MaxConcurrentRequests"] = fmt.Sprintf("%v", *prev.MaxConcurrentRequests)
		}
	}
	if next != nil {
		if next.MaxConcurrentRequests != nil {
			newPrimitiveFlat["MaxConcurrentRequests"] = fmt.Sprintf("%v", *next.MaxConcurrentRequests)
		}
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Diff the hosts.
	if hDiffs := stringSetDiff(prev.Hosts, next.Hosts, "Hosts", contextual); hDiffs != nil {
		diff.Objects = append(diff.Objects, hDiffs)
	}

	// Diff the ConsulGatewayTLSConfig objects.
	tlsConfigDiff := connectGatewayTLSConfigDiff(prev.TLS, next.TLS, contextual)
	if tlsConfigDiff != nil {
		diff.Objects = append(diff.Objects, tlsConfigDiff)
	}

	// Diff the ConsulHTTPHeaderModifiers objects (RequestHeaders).
	reqModifiersDiff := connectGatewayHTTPHeaderModifiersDiff(prev.RequestHeaders, next.RequestHeaders, "RequestHeaders", contextual)
	if reqModifiersDiff != nil {
		diff.Objects = append(diff.Objects, reqModifiersDiff)
	}

	// Diff the ConsulHTTPHeaderModifiers objects (ResponseHeaders).
	respModifiersDiff := connectGatewayHTTPHeaderModifiersDiff(prev.ResponseHeaders, next.ResponseHeaders, "ResponseHeaders", contextual)
	if respModifiersDiff != nil {
		diff.Objects = append(diff.Objects, respModifiersDiff)
	}

	return diff
}

func connectGatewayHTTPHeaderModifiersDiff(prev, next *ConsulHTTPHeaderModifiers, name string, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: name}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(prev, next) {
		return nil
	} else if prev == nil {
		prev = new(ConsulHTTPHeaderModifiers)
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	} else if next == nil {
		next = new(ConsulHTTPHeaderModifiers)
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Diff the Remove Headers.
	if rDiffs := stringSetDiff(prev.Remove, next.Remove, "Remove", contextual); rDiffs != nil {
		diff.Objects = append(diff.Objects, rDiffs)
	}

	return diff
}

func connectGatewayProxyDiff(prev, next *ConsulGatewayProxy, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Proxy"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(prev, next) {
		return nil
	} else if prev == nil {
		prev = new(ConsulGatewayProxy)
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	} else if next == nil {
		next = new(ConsulGatewayProxy)
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(prev, nil, true)
		newPrimitiveFlat = flatmap.Flatten(next, nil, true)
	}

	// Diff the ConnectTimeout field (dur ptr). (i.e. convert to string for comparison)
	if oldPrimitiveFlat != nil && newPrimitiveFlat != nil {
		if prev.ConnectTimeout == nil {
			oldPrimitiveFlat["ConnectTimeout"] = ""
		} else {
			oldPrimitiveFlat["ConnectTimeout"] = prev.ConnectTimeout.String()
		}
		if next.ConnectTimeout == nil {
			newPrimitiveFlat["ConnectTimeout"] = ""
		} else {
			newPrimitiveFlat["ConnectTimeout"] = next.ConnectTimeout.String()
		}
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Diff the EnvoyGatewayBindAddresses map.
	bindAddrsDiff := connectGatewayProxyEnvoyBindAddrsDiff(prev.EnvoyGatewayBindAddresses, next.EnvoyGatewayBindAddresses, contextual)
	if bindAddrsDiff != nil {
		diff.Objects = append(diff.Objects, bindAddrsDiff)
	}

	// Diff the opaque Config map.
	if cDiff := configDiff(prev.Config, next.Config, contextual); cDiff != nil {
		diff.Objects = append(diff.Objects, cDiff)
	}

	return diff
}

// connectGatewayProxyEnvoyBindAddrsDiff returns the diff of two maps. If contextual
// diff is enabled, all fields will be returned, even if no diff occurred.
func connectGatewayProxyEnvoyBindAddrsDiff(prev, next map[string]*ConsulGatewayBindAddress, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "EnvoyGatewayBindAddresses"}
	if reflect.DeepEqual(prev, next) {
		return nil
	} else if len(prev) == 0 {
		diff.Type = DiffTypeAdded
	} else if len(next) == 0 {
		diff.Type = DiffTypeDeleted
	} else {
		diff.Type = DiffTypeEdited
	}

	// convert to string representation
	prevMap := make(map[string]string, len(prev))
	nextMap := make(map[string]string, len(next))

	for k, v := range prev {
		prevMap[k] = net.JoinHostPort(v.Address, strconv.Itoa(v.Port))
	}

	for k, v := range next {
		nextMap[k] = net.JoinHostPort(v.Address, strconv.Itoa(v.Port))
	}

	oldPrimitiveFlat := flatmap.Flatten(prevMap, nil, false)
	newPrimitiveFlat := flatmap.Flatten(nextMap, nil, false)
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)
	return diff
}

// connectSidecarServiceDiff returns the diff of two ConsulSidecarService objects.
// If contextual diff is enabled, all fields will be returned, even if no diff occurred.
func connectSidecarServiceDiff(old, new *ConsulSidecarService, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "SidecarService"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		old = &ConsulSidecarService{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	} else if new == nil {
		new = &ConsulSidecarService{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	consulProxyDiff := consulProxyDiff(old.Proxy, new.Proxy, contextual)
	if consulProxyDiff != nil {
		diff.Objects = append(diff.Objects, consulProxyDiff)
	}

	return diff
}

// sidecarTaskDiff returns the diff of two Task objects.
// If contextual diff is enabled, all fields will be returned, even if no diff occurred.
func sidecarTaskDiff(old, new *SidecarTask, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "SidecarTask"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		old = &SidecarTask{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	} else if new == nil {
		new = &SidecarTask{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, false)

	// Config diff
	if cDiff := configDiff(old.Config, new.Config, contextual); cDiff != nil {
		diff.Objects = append(diff.Objects, cDiff)
	}

	// Resources diff
	if rDiff := old.Resources.Diff(new.Resources, contextual); rDiff != nil {
		diff.Objects = append(diff.Objects, rDiff)
	}

	// LogConfig diff
	lDiff := primitiveObjectDiff(old.LogConfig, new.LogConfig, nil, "LogConfig", contextual)
	if lDiff != nil {
		diff.Objects = append(diff.Objects, lDiff)
	}

	return diff
}

// consulProxyDiff returns the diff of two ConsulProxy objects.
// If contextual diff is enabled, all fields will be returned, even if no diff occurred.
func consulProxyDiff(old, new *ConsulProxy, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "ConsulProxy"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		old = &ConsulProxy{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	} else if new == nil {
		new = &ConsulProxy{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	}

	// diff the primitive fields
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// diff the consul upstream slices
	if upDiffs := consulProxyUpstreamsDiff(old.Upstreams, new.Upstreams, contextual); upDiffs != nil {
		diff.Objects = append(diff.Objects, upDiffs...)
	}

	if exposeDiff := consulProxyExposeDiff(old.Expose, new.Expose, contextual); exposeDiff != nil {
		diff.Objects = append(diff.Objects, exposeDiff)
	}

	if tproxyDiff := consulTProxyDiff(old.TransparentProxy, new.TransparentProxy, contextual); tproxyDiff != nil {
		diff.Objects = append(diff.Objects, tproxyDiff)
	}

	// diff the config blob
	if cDiff := configDiff(old.Config, new.Config, contextual); cDiff != nil {
		diff.Objects = append(diff.Objects, cDiff)
	}

	return diff
}

// consulProxyUpstreamsDiff diffs a set of connect upstreams. If contextual diff is
// enabled, unchanged fields within objects nested in the tasks will be returned.
func consulProxyUpstreamsDiff(old, new []ConsulUpstream, contextual bool) []*ObjectDiff {
	oldMap := make(map[string]ConsulUpstream, len(old))
	newMap := make(map[string]ConsulUpstream, len(new))

	idx := func(up ConsulUpstream) string {
		return fmt.Sprintf("%s/%s", up.Datacenter, up.DestinationName)
	}

	for _, o := range old {
		oldMap[idx(o)] = o
	}
	for _, n := range new {
		newMap[idx(n)] = n
	}

	var diffs []*ObjectDiff
	for index, oldUpstream := range oldMap {
		// Diff the same, deleted, and edited
		if diff := consulProxyUpstreamDiff(oldUpstream, newMap[index], contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}

	for index, newUpstream := range newMap {
		// diff the added
		if oldUpstream, exists := oldMap[index]; !exists {
			if diff := consulProxyUpstreamDiff(oldUpstream, newUpstream, contextual); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}
	sort.Sort(ObjectDiffs(diffs))
	return diffs
}

func consulProxyUpstreamDiff(prev, next ConsulUpstream, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "ConsulUpstreams"}
	var oldPrimFlat, newPrimFlat map[string]string

	if reflect.DeepEqual(prev, next) {
		return nil
	} else if prev.Equal(new(ConsulUpstream)) {
		prev = ConsulUpstream{}
		diff.Type = DiffTypeAdded
		newPrimFlat = flatmap.Flatten(next, nil, true)
	} else if next.Equal(new(ConsulUpstream)) {
		next = ConsulUpstream{}
		diff.Type = DiffTypeDeleted
		oldPrimFlat = flatmap.Flatten(prev, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimFlat = flatmap.Flatten(prev, nil, true)
		newPrimFlat = flatmap.Flatten(next, nil, true)
	}

	// diff the primitive fields
	diff.Fields = fieldDiffs(oldPrimFlat, newPrimFlat, contextual)

	// diff the mesh gateway primitive object
	if mDiff := primitiveObjectDiff(prev.MeshGateway, next.MeshGateway, nil, "MeshGateway", contextual); mDiff != nil {
		diff.Objects = append(diff.Objects, mDiff)
	}

	return diff
}

func consulProxyExposeDiff(prev, next *ConsulExposeConfig, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Expose"}

	if reflect.DeepEqual(prev, next) {
		return nil
	} else if prev == nil || prev.Equal(&ConsulExposeConfig{}) {
		prev = &ConsulExposeConfig{}
		diff.Type = DiffTypeAdded
	} else if next == nil || next.Equal(&ConsulExposeConfig{}) {
		next = &ConsulExposeConfig{}
		diff.Type = DiffTypeDeleted
	} else {
		diff.Type = DiffTypeEdited
	}

	var prevPaths, nextPaths []any
	if prev != nil {
		prevPaths = interfaceSlice(prev.Paths)
	}
	if next != nil {
		nextPaths = interfaceSlice(next.Paths)
	}

	if pathDiff := primitiveObjectSetDiff(
		prevPaths,
		nextPaths,
		nil, "Paths",
		contextual); pathDiff != nil {
		diff.Objects = append(diff.Objects, pathDiff...)
	}

	return diff
}

func consulTProxyDiff(prev, next *ConsulTransparentProxy, contextual bool) *ObjectDiff {

	diff := &ObjectDiff{Type: DiffTypeNone, Name: "TransparentProxy"}
	var oldPrimFlat, newPrimFlat map[string]string

	if prev.Equal(next) {
		return diff
	} else if prev == nil {
		prev = &ConsulTransparentProxy{}
		diff.Type = DiffTypeAdded
		newPrimFlat = flatmap.Flatten(next, nil, true)
	} else if next == nil {
		next = &ConsulTransparentProxy{}
		diff.Type = DiffTypeDeleted
		oldPrimFlat = flatmap.Flatten(prev, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimFlat = flatmap.Flatten(prev, nil, true)
		newPrimFlat = flatmap.Flatten(next, nil, true)
	}

	// diff the primitive fields
	diff.Fields = fieldDiffs(oldPrimFlat, newPrimFlat, contextual)

	if setDiff := stringSetDiff(prev.ExcludeInboundPorts, next.ExcludeInboundPorts,
		"ExcludeInboundPorts", contextual); setDiff != nil && setDiff.Type != DiffTypeNone {
		diff.Objects = append(diff.Objects, setDiff)
	}

	if setDiff := stringSetDiff(
		helper.ConvertSlice(prev.ExcludeOutboundPorts, func(a uint16) string { return fmt.Sprint(a) }),
		helper.ConvertSlice(next.ExcludeOutboundPorts, func(a uint16) string { return fmt.Sprint(a) }),
		"ExcludeOutboundPorts",
		contextual,
	); setDiff != nil && setDiff.Type != DiffTypeNone {
		diff.Objects = append(diff.Objects, setDiff)
	}

	if setDiff := stringSetDiff(prev.ExcludeOutboundCIDRs, next.ExcludeOutboundCIDRs,
		"ExcludeOutboundCIDRs", contextual); setDiff != nil && setDiff.Type != DiffTypeNone {
		diff.Objects = append(diff.Objects, setDiff)
	}

	if setDiff := stringSetDiff(prev.ExcludeUIDs, next.ExcludeUIDs,
		"ExcludeUIDs", contextual); setDiff != nil && setDiff.Type != DiffTypeNone {
		diff.Objects = append(diff.Objects, setDiff)
	}

	return diff
}

// serviceCheckDiffs diffs a set of service checks. If contextual diff is
// enabled, unchanged fields within objects nested in the tasks will be
// returned.
func serviceCheckDiffs(old, new []*ServiceCheck, contextual bool) []*ObjectDiff {
	oldMap := make(map[string]*ServiceCheck, len(old))
	newMap := make(map[string]*ServiceCheck, len(new))
	for _, o := range old {
		oldMap[o.Name] = o
	}
	for _, n := range new {
		newMap[n.Name] = n
	}

	var diffs []*ObjectDiff
	for name, oldCheck := range oldMap {
		// Diff the same, deleted and edited
		if diff := serviceCheckDiff(oldCheck, newMap[name], contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}

	for name, newCheck := range newMap {
		// Diff the added
		if old, ok := oldMap[name]; !ok {
			if diff := serviceCheckDiff(old, newCheck, contextual); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}

	sort.Sort(ObjectDiffs(diffs))
	return diffs
}

// vaultDiff returns the diff of two vault objects. If contextual diff is
// enabled, all fields will be returned, even if no diff occurred.
func vaultDiff(old, new *Vault, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Vault"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		old = &Vault{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	} else if new == nil {
		new = &Vault{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Policies diffs
	if setDiff := stringSetDiff(old.Policies, new.Policies, "Policies", contextual); setDiff != nil {
		diff.Objects = append(diff.Objects, setDiff)
	}

	return diff
}

// waitConfigDiff returns the diff of two WaitConfig objects. If contextual diff is
// enabled, all fields will be returned, even if no diff occurred.
func waitConfigDiff(old, new *WaitConfig, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Template"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(new, nil, false)
	} else if new == nil {
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(old, nil, false)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(old, nil, false)
		newPrimitiveFlat = flatmap.Flatten(new, nil, false)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	return diff
}

// changeScriptDiff returns the diff of two ChangeScript objects. If contextual
// diff is enabled, all fields will be returned, even if no diff occurred.
func changeScriptDiff(old, new *ChangeScript, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "ChangeScript"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		old = &ChangeScript{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	} else if new == nil {
		new = &ChangeScript{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Args diffs
	if setDiff := stringSetDiff(old.Args, new.Args, "Args", contextual); setDiff != nil {
		diff.Objects = append(diff.Objects, setDiff)
	}

	return diff
}

// templateDiff returns the diff of two Consul Template objects. If contextual diff is
// enabled, all fields will be returned, even if no diff occurred.
func templateDiff(old, new *Template, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Template"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		old = &Template{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	} else if new == nil {
		new = &Template{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	}

	// Add the pointer primitive fields.
	if old != nil {
		if old.Uid != nil {
			oldPrimitiveFlat["Uid"] = fmt.Sprintf("%v", *old.Uid)
		}
		if old.Gid != nil {
			oldPrimitiveFlat["Gid"] = fmt.Sprintf("%v", *old.Gid)
		}
	}
	if new != nil {
		if new.Uid != nil {
			newPrimitiveFlat["Uid"] = fmt.Sprintf("%v", *new.Uid)
		}
		if new.Gid != nil {
			newPrimitiveFlat["Gid"] = fmt.Sprintf("%v", *new.Gid)
		}
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// WaitConfig diffs
	if waitDiffs := waitConfigDiff(old.Wait, new.Wait, contextual); waitDiffs != nil {
		diff.Objects = append(diff.Objects, waitDiffs)
	}

	// ChangeScript diffs
	if changeScriptDiffs := changeScriptDiff(
		old.ChangeScript, new.ChangeScript, contextual,
	); changeScriptDiffs != nil {
		diff.Objects = append(diff.Objects, changeScriptDiffs)
	}

	return diff
}

// templateDiffs returns the diff of two Consul Template slices. If contextual diff is
// enabled, all fields will be returned, even if no diff occurred.
// serviceDiffs diffs a set of services. If contextual diff is enabled, unchanged
// fields within objects nested in the tasks will be returned.
func templateDiffs(old, new []*Template, contextual bool) []*ObjectDiff {
	// Handle trivial case.
	if len(old) == 1 && len(new) == 1 {
		if diff := templateDiff(old[0], new[0], contextual); diff != nil {
			return []*ObjectDiff{diff}
		}
		return nil
	}

	// For each template we will try to find a corresponding match in the other list.
	// The following lists store the index of the matching template for each
	// position of the inputs.
	oldMatches := make([]int, len(old))
	newMatches := make([]int, len(new))

	// Initialize all templates as unmatched.
	for i := range oldMatches {
		oldMatches[i] = -1
	}
	for i := range newMatches {
		newMatches[i] = -1
	}

	// Find a match in the new templates list for each old template and compute
	// their diffs.
	var diffs []*ObjectDiff
	for oldIndex, oldTemplate := range old {
		newIndex := findTemplateMatch(oldTemplate, new, newMatches)

		// Old templates that don't have a match were deleted.
		if newIndex < 0 {
			diff := templateDiff(oldTemplate, nil, contextual)
			diffs = append(diffs, diff)
			continue
		}

		// If A matches B then B matches A.
		oldMatches[oldIndex] = newIndex
		newMatches[newIndex] = oldIndex

		newTemplate := new[newIndex]
		if diff := templateDiff(oldTemplate, newTemplate, contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}

	// New templates without match were added.
	for i, m := range newMatches {
		if m == -1 {
			diff := templateDiff(nil, new[i], contextual)
			diffs = append(diffs, diff)
		}
	}

	sort.Sort(ObjectDiffs(diffs))
	return diffs
}

func findTemplateMatch(template *Template, newTemplates []*Template, newTemplateMatches []int) int {
	indexMatch := -1

	for i, newTemplate := range newTemplates {
		// Skip template if it's already matched.
		if newTemplateMatches[i] >= 0 {
			continue
		}

		if template.DiffID() == newTemplate.DiffID() {
			indexMatch = i
			break
		}
	}

	return indexMatch
}

// parameterizedJobDiff returns the diff of two parameterized job objects. If
// contextual diff is enabled, all fields will be returned, even if no diff
// occurred.
func parameterizedJobDiff(old, new *ParameterizedJobConfig, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "ParameterizedJob"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		old = &ParameterizedJobConfig{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	} else if new == nil {
		new = &ParameterizedJobConfig{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Meta diffs
	if optionalDiff := stringSetDiff(old.MetaOptional, new.MetaOptional, "MetaOptional", contextual); optionalDiff != nil {
		diff.Objects = append(diff.Objects, optionalDiff)
	}

	if requiredDiff := stringSetDiff(old.MetaRequired, new.MetaRequired, "MetaRequired", contextual); requiredDiff != nil {
		diff.Objects = append(diff.Objects, requiredDiff)
	}

	return diff
}

func multiregionDiff(old, new *Multiregion, contextual bool) *ObjectDiff {

	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Multiregion"}

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		old = &Multiregion{}
		old.Canonicalize()
		diff.Type = DiffTypeAdded
	} else if new == nil {
		new = &Multiregion{}
		diff.Type = DiffTypeDeleted
	} else {
		diff.Type = DiffTypeEdited
	}

	// strategy diff
	stratDiff := primitiveObjectDiff(
		old.Strategy,
		new.Strategy,
		[]string{},
		"Strategy",
		contextual)
	if stratDiff != nil {
		diff.Objects = append(diff.Objects, stratDiff)
	}

	oldMap := make(map[string]*MultiregionRegion, len(old.Regions))
	newMap := make(map[string]*MultiregionRegion, len(new.Regions))
	for _, o := range old.Regions {
		oldMap[o.Name] = o
	}
	for _, n := range new.Regions {
		newMap[n.Name] = n
	}

	for name, oldRegion := range oldMap {
		// Diff the same, deleted and edited
		newRegion := newMap[name]
		rdiff := multiregionRegionDiff(oldRegion, newRegion, contextual)
		if rdiff != nil {
			diff.Objects = append(diff.Objects, rdiff)
		}
	}

	for name, newRegion := range newMap {
		// Diff the added
		if oldRegion, ok := oldMap[name]; !ok {
			rdiff := multiregionRegionDiff(oldRegion, newRegion, contextual)
			if rdiff != nil {
				diff.Objects = append(diff.Objects, rdiff)
			}
		}
	}
	sort.Sort(FieldDiffs(diff.Fields))
	sort.Sort(ObjectDiffs(diff.Objects))
	return diff
}

func multiregionRegionDiff(r, other *MultiregionRegion, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Region"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(r, other) {
		return nil
	} else if r == nil {
		r = &MultiregionRegion{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(other, nil, true)
	} else if other == nil {
		other = &MultiregionRegion{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(r, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(r, nil, true)
		newPrimitiveFlat = flatmap.Flatten(other, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Datacenters diff
	setDiff := stringSetDiff(r.Datacenters, other.Datacenters, "Datacenters", contextual)
	if setDiff != nil && setDiff.Type != DiffTypeNone {
		diff.Objects = append(diff.Objects, setDiff)
	}

	sort.Sort(ObjectDiffs(diff.Objects))
	sort.Sort(FieldDiffs(diff.Fields))

	var added, deleted, edited bool
Loop:
	for _, f := range diff.Fields {
		switch f.Type {
		case DiffTypeEdited:
			edited = true
			break Loop
		case DiffTypeDeleted:
			deleted = true
		case DiffTypeAdded:
			added = true
		}
	}

	if edited || added && deleted {
		diff.Type = DiffTypeEdited
	} else if added {
		diff.Type = DiffTypeAdded
	} else if deleted {
		diff.Type = DiffTypeDeleted
	} else {
		return nil
	}

	return diff
}

func uiDiff(old, new *JobUIConfig, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "UI"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		old = &JobUIConfig{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	} else if new == nil {
		new = &JobUIConfig{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
		newPrimitiveFlat = flatmap.Flatten(new, nil, true)
	}

	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	if linkDiffs := linkDiffs(old.Links, new.Links, contextual); len(linkDiffs) > 0 {
		diff.Objects = append(diff.Objects, linkDiffs...)

	}

	// Sort
	sort.Sort(FieldDiffs(diff.Fields))
	sort.Sort(ObjectDiffs(diff.Objects))

	return diff
}

func linkDiffs(old, new []*JobUILink, contextual bool) []*ObjectDiff {
	var diffs []*ObjectDiff

	for i := 0; i < len(old) && i < len(new); i++ {
		if diff := linkDiff(*old[i], *new[i], contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}

	// Deleted links
	for i := len(new); i < len(old); i++ {
		emptyNew := JobUILink{} // Simulate an empty new link
		if diff := linkDiff(*old[i], emptyNew, contextual); diff != nil {
			diff.Type = DiffTypeDeleted // Mark the diff as a deletion
			diffs = append(diffs, diff)
		}
	}

	// New links
	for i := len(old); i < len(new); i++ {
		emptyOld := JobUILink{} // Simulate an empty old link
		if diff := linkDiff(emptyOld, *new[i], contextual); diff != nil {
			diff.Type = DiffTypeAdded // Mark the diff as an addition
			diffs = append(diffs, diff)
		}
	}

	sort.Sort(ObjectDiffs(diffs))
	return diffs
}

func linkDiff(old, new JobUILink, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Link"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string
	if reflect.DeepEqual(old, new) {
		return nil
	}

	diff.Type = DiffTypeEdited
	oldPrimitiveFlat = flatmap.Flatten(old, nil, true)
	newPrimitiveFlat = flatmap.Flatten(new, nil, true)

	// Diff the primitive fields
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	return diff
}

// volumeDiffs returns the diff of a group's volume requests. If contextual
// diff is enabled, all fields will be returned, even if no diff occurred.
func volumeDiffs(oldVR, newVR map[string]*VolumeRequest, contextual bool) []*ObjectDiff {
	if reflect.DeepEqual(oldVR, newVR) {
		return nil
	}

	diffs := []*ObjectDiff{} //Type: DiffTypeNone, Name: "Volumes"}
	seen := map[string]bool{}
	for name, oReq := range oldVR {
		nReq := newVR[name] // might be nil, that's ok
		seen[name] = true
		diff := volumeDiff(oReq, nReq, contextual)
		if diff != nil {
			diffs = append(diffs, diff)
		}
	}
	for name, nReq := range newVR {
		if !seen[name] {
			// we know old is nil at this point, or we'd have hit it before
			diff := volumeDiff(nil, nReq, contextual)
			if diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}
	return diffs
}

// volumeDiff returns the diff between two volume requests. If contextual diff
// is enabled, all fields will be returned, even if no diff occurred.
func volumeDiff(oldVR, newVR *VolumeRequest, contextual bool) *ObjectDiff {
	if reflect.DeepEqual(oldVR, newVR) {
		return nil
	}

	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Volume"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if oldVR == nil {
		oldVR = &VolumeRequest{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(newVR, nil, true)
	} else if newVR == nil {
		newVR = &VolumeRequest{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(oldVR, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(oldVR, nil, true)
		newPrimitiveFlat = flatmap.Flatten(newVR, nil, true)
	}

	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	mOptsDiff := volumeCSIMountOptionsDiff(oldVR.MountOptions, newVR.MountOptions, contextual)
	if mOptsDiff != nil {
		diff.Objects = append(diff.Objects, mOptsDiff)
	}

	return diff
}

// volumeCSIMountOptionsDiff returns the diff between volume mount options. If
// contextual diff is enabled, all fields will be returned, even if no diff
// occurred.
func volumeCSIMountOptionsDiff(oldMO, newMO *CSIMountOptions, contextual bool) *ObjectDiff {
	if reflect.DeepEqual(oldMO, newMO) {
		return nil
	}

	diff := &ObjectDiff{Type: DiffTypeNone, Name: "MountOptions"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if oldMO == nil && newMO != nil {
		oldMO = &CSIMountOptions{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(newMO, nil, true)
	} else if oldMO != nil && newMO == nil {
		newMO = &CSIMountOptions{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(oldMO, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(oldMO, nil, true)
		newPrimitiveFlat = flatmap.Flatten(newMO, nil, true)
	}

	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	setDiff := stringSetDiff(oldMO.MountFlags, newMO.MountFlags, "MountFlags", contextual)
	if setDiff != nil {
		diff.Objects = append(diff.Objects, setDiff)
	}
	return diff
}

func volumeMountsDiffs(oldMounts, newMounts []*VolumeMount, contextual bool) []*ObjectDiff {
	var diffs []*ObjectDiff

	for i := 0; i < len(oldMounts) && i < len(newMounts); i++ {
		oldMount := oldMounts[i]
		newMount := newMounts[i]

		if diff := volumeMountDiff(oldMount, newMount, contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}

	for i := len(newMounts); i < len(oldMounts); i++ {
		if diff := volumeMountDiff(oldMounts[i], nil, contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}

	for i := len(oldMounts); i < len(newMounts); i++ {
		if diff := volumeMountDiff(nil, newMounts[i], contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}

	sort.Sort(ObjectDiffs(diffs))

	return diffs
}

func volumeMountDiff(oldMount, newMount *VolumeMount, contextual bool) *ObjectDiff {
	if reflect.DeepEqual(oldMount, newMount) {
		return nil
	}

	diff := &ObjectDiff{Type: DiffTypeNone, Name: "VolumeMount"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string
	if oldMount == nil && newMount != nil {
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(newMount, nil, true)
	} else if oldMount != nil && newMount == nil {
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(oldMount, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(oldMount, nil, true)
		newPrimitiveFlat = flatmap.Flatten(newMount, nil, true)
	}

	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)
	return diff
}

// Diff returns a diff of two resource objects. If contextual diff is enabled,
// non-changed fields will still be returned.
func (r *Resources) Diff(other *Resources, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Resources"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(r, other) {
		return nil
	} else if r == nil {
		r = &Resources{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(other, nil, true)
	} else if other == nil {
		other = &Resources{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(r, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(r, nil, true)
		newPrimitiveFlat = flatmap.Flatten(other, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Network Resources diff
	if nDiffs := networkResourceDiffs(r.Networks, other.Networks, contextual); nDiffs != nil {
		diff.Objects = append(diff.Objects, nDiffs...)
	}

	// Requested Devices diff
	if nDiffs := requestedDevicesDiffs(r.Devices, other.Devices, contextual); nDiffs != nil {
		diff.Objects = append(diff.Objects, nDiffs...)
	}

	// NUMA resources diff
	if nDiff := r.NUMA.Diff(other.NUMA, contextual); nDiff != nil {
		diff.Objects = append(diff.Objects, nDiff)
	}

	return diff
}

// Diff returns a diff of two network resources. If contextual diff is enabled,
// non-changed fields will still be returned.
func (n *NetworkResource) Diff(other *NetworkResource, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Network"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string
	filter := []string{"_struct", "Device", "CIDR", "IP", "MBits"}

	if reflect.DeepEqual(n, other) {
		return nil
	} else if n == nil {
		n = &NetworkResource{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(other, filter, true)
	} else if other == nil {
		other = &NetworkResource{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(n, filter, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(n, filter, true)
		newPrimitiveFlat = flatmap.Flatten(other, filter, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Port diffs
	resPorts := portDiffs(n.ReservedPorts, other.ReservedPorts, false, contextual)
	dynPorts := portDiffs(n.DynamicPorts, other.DynamicPorts, true, contextual)
	if resPorts != nil {
		diff.Objects = append(diff.Objects, resPorts...)
	}
	if dynPorts != nil {
		diff.Objects = append(diff.Objects, dynPorts...)
	}

	if dnsDiff := n.DNS.Diff(other.DNS, contextual); dnsDiff != nil {
		diff.Objects = append(diff.Objects, dnsDiff)
	}

	if cniDiff := n.CNI.Diff(other.CNI, contextual); cniDiff != nil {
		diff.Objects = append(diff.Objects, cniDiff)
	}

	return diff
}

// Diff returns a diff of two DNSConfig structs
func (d *DNSConfig) Diff(other *DNSConfig, contextual bool) *ObjectDiff {
	if reflect.DeepEqual(d, other) {
		return nil
	}

	flatten := func(conf *DNSConfig) map[string]string {
		m := map[string]string{}
		if len(conf.Servers) > 0 {
			m["Servers"] = strings.Join(conf.Servers, ",")
		}
		if len(conf.Searches) > 0 {
			m["Searches"] = strings.Join(conf.Searches, ",")
		}
		if len(conf.Options) > 0 {
			m["Options"] = strings.Join(conf.Options, ",")
		}
		return m
	}

	diff := &ObjectDiff{Type: DiffTypeNone, Name: "DNS"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string
	if d == nil {
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatten(other)
	} else if other == nil {
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatten(d)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatten(d)
		newPrimitiveFlat = flatten(other)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	return diff
}

// Diff returns a diff of two CNIConfig structs
func (d *CNIConfig) Diff(other *CNIConfig, contextual bool) *ObjectDiff {
	if d == nil {
		d = &CNIConfig{}
	}
	if other == nil {
		other = &CNIConfig{}
	}
	if d.Equal(other) {
		return nil
	}

	return primitiveObjectDiff(d.Args, other.Args, nil, "CNIConfig", contextual)
}

func disconectStrategyDiffs(old, new *DisconnectStrategy, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Disconnect"}
	var oldDisconnectFlat, newDisconnectFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		diff.Type = DiffTypeAdded
		newDisconnectFlat = flatmap.Flatten(new, nil, false)
	} else if new == nil {
		diff.Type = DiffTypeDeleted
		oldDisconnectFlat = flatmap.Flatten(old, nil, false)
	} else {
		diff.Type = DiffTypeEdited
		oldDisconnectFlat = flatmap.Flatten(old, nil, false)
		newDisconnectFlat = flatmap.Flatten(new, nil, false)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldDisconnectFlat, newDisconnectFlat, contextual)

	return diff
}

// networkResourceDiffs diffs a set of NetworkResources. If contextual diff is enabled,
// non-changed fields will still be returned.
func networkResourceDiffs(old, new []*NetworkResource, contextual bool) []*ObjectDiff {
	// This function will not allow Network Resources to have a diffType of DiffTypeEdited
	// as hash keys for old and new would only be equivalent if new and old are equivalent
	// (no changes found between them). Despite this behavior, a hash must be used to find possible
	// differences between new and old since Network Resources are not ordered.
	makeSet := func(objects []*NetworkResource) map[string]*NetworkResource {
		objMap := make(map[string]*NetworkResource, len(objects))
		for _, obj := range objects {
			hash, err := hashstructure.Hash(obj, nil)
			if err != nil {
				panic(err)
			}
			objMap[fmt.Sprintf("%d", hash)] = obj
		}

		return objMap
	}

	oldSet := makeSet(old)
	newSet := makeSet(new)

	var diffs []*ObjectDiff
	for k, oldV := range oldSet {
		if newV, ok := newSet[k]; !ok {
			if diff := oldV.Diff(newV, contextual); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}
	for k, newV := range newSet {
		if oldV, ok := oldSet[k]; !ok {
			if diff := oldV.Diff(newV, contextual); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}

	sort.Sort(ObjectDiffs(diffs))
	return diffs

}

// portDiffs returns the diff of two sets of ports. The dynamic flag marks the
// set of ports as being Dynamic ports versus Static ports. If contextual diff is enabled,
// non-changed fields will still be returned.
func portDiffs(old, new []Port, dynamic bool, contextual bool) []*ObjectDiff {
	makeSet := func(ports []Port) map[string]Port {
		portMap := make(map[string]Port, len(ports))
		for _, port := range ports {
			portMap[port.Label] = port
		}

		return portMap
	}

	oldPorts := makeSet(old)
	newPorts := makeSet(new)

	filter := []string{"_struct"}
	name := "Static Port"
	if dynamic {
		filter = []string{"_struct", "Value", "IgnoreCollision"}
		name = "Dynamic Port"
	}

	var diffs []*ObjectDiff
	for portLabel, oldPort := range oldPorts {
		// Diff the same, deleted and edited
		if newPort, ok := newPorts[portLabel]; ok {
			diff := primitiveObjectDiff(oldPort, newPort, filter, name, contextual)
			if diff != nil {
				diffs = append(diffs, diff)
			}
		} else {
			diff := primitiveObjectDiff(oldPort, nil, filter, name, contextual)
			if diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}
	for label, newPort := range newPorts {
		// Diff the added
		if _, ok := oldPorts[label]; !ok {
			diff := primitiveObjectDiff(nil, newPort, filter, name, contextual)
			if diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}

	sort.Sort(ObjectDiffs(diffs))
	return diffs

}

func (r *NUMA) Diff(other *NUMA, contextual bool) *ObjectDiff {
	if r.Equal(other) {
		return nil
	}

	diff := &ObjectDiff{Type: DiffTypeNone, Name: "NUMA"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if r == nil {
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(other, nil, true)
	} else if other == nil {
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(r, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(r, nil, true)
		newPrimitiveFlat = flatmap.Flatten(other, nil, true)
	}
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	return diff
}

// Diff returns a diff of two requested devices. If contextual diff is enabled,
// non-changed fields will still be returned.
func (r *RequestedDevice) Diff(other *RequestedDevice, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Device"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(r, other) {
		return nil
	} else if r == nil {
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(other, nil, true)
	} else if other == nil {
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(r, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(r, nil, true)
		newPrimitiveFlat = flatmap.Flatten(other, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	return diff
}

// requestedDevicesDiffs diffs a set of RequestedDevices. If contextual diff is enabled,
// non-changed fields will still be returned.
func requestedDevicesDiffs(old, new []*RequestedDevice, contextual bool) []*ObjectDiff {
	makeSet := func(devices []*RequestedDevice) map[string]*RequestedDevice {
		deviceMap := make(map[string]*RequestedDevice, len(devices))
		for _, d := range devices {
			deviceMap[d.Name] = d
		}

		return deviceMap
	}

	oldSet := makeSet(old)
	newSet := makeSet(new)

	var diffs []*ObjectDiff
	for k, oldV := range oldSet {
		newV := newSet[k]
		if diff := oldV.Diff(newV, contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}
	for k, newV := range newSet {
		if oldV, ok := oldSet[k]; !ok {
			if diff := oldV.Diff(newV, contextual); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}

	sort.Sort(ObjectDiffs(diffs))
	return diffs

}

// configDiff returns the diff of two Task Config objects. If contextual diff is
// enabled, all fields will be returned, even if no diff occurred.
func configDiff(old, new map[string]interface{}, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Config"}
	if reflect.DeepEqual(old, new) {
		return nil
	} else if len(old) == 0 {
		diff.Type = DiffTypeAdded
	} else if len(new) == 0 {
		diff.Type = DiffTypeDeleted
	} else {
		diff.Type = DiffTypeEdited
	}

	// Diff the primitive fields.
	oldPrimitiveFlat := flatmap.Flatten(old, nil, false)
	newPrimitiveFlat := flatmap.Flatten(new, nil, false)
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)
	return diff
}

// idSliceDiff returns the diff of two slices of identity objects. If
// contextual diff is enabled, all fields will be returned, even if no diff
// occurred.
func idSliceDiffs(old, new []*WorkloadIdentity, contextual bool) []*ObjectDiff {
	oldMap := make(map[string]*WorkloadIdentity, len(old))
	newMap := make(map[string]*WorkloadIdentity, len(new))

	for _, o := range old {
		oldMap[o.Name] = o
	}
	for _, n := range new {
		newMap[n.Name] = n
	}

	var diffs []*ObjectDiff
	for index, oldID := range oldMap {
		// Diff the same, deleted, and edited
		if diff := idDiff(oldID, newMap[index], contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}

	for index, newID := range newMap {
		// diff the added
		if oldID, exists := oldMap[index]; !exists {
			if diff := idDiff(oldID, newID, contextual); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}
	sort.Sort(ObjectDiffs(diffs))
	return diffs
}

// idDiff returns the diff of two identity objects. If contextual diff is
// enabled, all fields will be returned, even if no diff occurred.
func idDiff(oldWI, newWI *WorkloadIdentity, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Identity"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string

	if reflect.DeepEqual(oldWI, newWI) {
		return nil
	} else if oldWI == nil {
		oldWI = &WorkloadIdentity{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(newWI, nil, true)
	} else if newWI == nil {
		newWI = &WorkloadIdentity{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(oldWI, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(oldWI, nil, true)
		newPrimitiveFlat = flatmap.Flatten(newWI, nil, true)
	}

	audDiff := stringSetDiff(oldWI.Audience, newWI.Audience, "Audience", contextual)
	if audDiff != nil {
		diff.Objects = append(diff.Objects, audDiff)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	return diff
}

// ObjectDiff contains the diff of two generic objects.
type ObjectDiff struct {
	Type    DiffType
	Name    string
	Fields  []*FieldDiff
	Objects []*ObjectDiff
}

func (o *ObjectDiff) GoString() string {
	out := fmt.Sprintf("\n%q (%s) {\n", o.Name, o.Type)
	for _, f := range o.Fields {
		out += fmt.Sprintf("%#v\n", f)
	}
	for _, o := range o.Objects {
		out += fmt.Sprintf("%#v\n", o)
	}
	out += "}"
	return out
}

func (o *ObjectDiff) Less(other *ObjectDiff) bool {
	if reflect.DeepEqual(o, other) {
		return false
	} else if other == nil {
		return false
	} else if o == nil {
		return true
	}

	if o.Name != other.Name {
		return o.Name < other.Name
	}

	if o.Type != other.Type {
		return o.Type.Less(other.Type)
	}

	if lO, lOther := len(o.Fields), len(other.Fields); lO != lOther {
		return lO < lOther
	}

	if lO, lOther := len(o.Objects), len(other.Objects); lO != lOther {
		return lO < lOther
	}

	// Check each field
	sort.Sort(FieldDiffs(o.Fields))
	sort.Sort(FieldDiffs(other.Fields))

	for i, oV := range o.Fields {
		if oV.Less(other.Fields[i]) {
			return true
		}
	}

	// Check each object
	sort.Sort(ObjectDiffs(o.Objects))
	sort.Sort(ObjectDiffs(other.Objects))
	for i, oV := range o.Objects {
		if oV.Less(other.Objects[i]) {
			return true
		}
	}

	return false
}

// For sorting ObjectDiffs
type ObjectDiffs []*ObjectDiff

func (o ObjectDiffs) Len() int           { return len(o) }
func (o ObjectDiffs) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o ObjectDiffs) Less(i, j int) bool { return o[i].Less(o[j]) }

type FieldDiff struct {
	Type        DiffType
	Name        string
	Old, New    string
	Annotations []string
}

// fieldDiff returns a FieldDiff if old and new are different otherwise, it
// returns nil. If contextual diff is enabled, even non-changed fields will be
// returned.
func fieldDiff(old, new, name string, contextual bool) *FieldDiff {
	diff := &FieldDiff{Name: name, Type: DiffTypeNone}
	if old == new {
		if !contextual {
			return nil
		}
		diff.Old, diff.New = old, new
		return diff
	}

	if old == "" {
		diff.Type = DiffTypeAdded
		diff.New = new
	} else if new == "" {
		diff.Type = DiffTypeDeleted
		diff.Old = old
	} else {
		diff.Type = DiffTypeEdited
		diff.Old = old
		diff.New = new
	}
	return diff
}

func (f *FieldDiff) GoString() string {
	out := fmt.Sprintf("%q (%s): %q => %q", f.Name, f.Type, f.Old, f.New)
	if len(f.Annotations) != 0 {
		out += fmt.Sprintf(" (%s)", strings.Join(f.Annotations, ", "))
	}

	return out
}

func (f *FieldDiff) Less(other *FieldDiff) bool {
	if reflect.DeepEqual(f, other) {
		return false
	} else if other == nil {
		return false
	} else if f == nil {
		return true
	}

	if f.Name != other.Name {
		return f.Name < other.Name
	} else if f.Old != other.Old {
		return f.Old < other.Old
	}

	return f.New < other.New
}

// For sorting FieldDiffs
type FieldDiffs []*FieldDiff

func (f FieldDiffs) Len() int           { return len(f) }
func (f FieldDiffs) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f FieldDiffs) Less(i, j int) bool { return f[i].Less(f[j]) }

// fieldDiffs takes a map of field names to their values and returns a set of
// field diffs. If contextual diff is enabled, even non-changed fields will be
// returned.
func fieldDiffs(old, new map[string]string, contextual bool) []*FieldDiff {
	var diffs []*FieldDiff
	visited := make(map[string]struct{})
	for k, oldV := range old {
		visited[k] = struct{}{}
		newV := new[k]
		if diff := fieldDiff(oldV, newV, k, contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}

	for k, newV := range new {
		if _, ok := visited[k]; !ok {
			if diff := fieldDiff("", newV, k, contextual); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}

	sort.Sort(FieldDiffs(diffs))
	return diffs
}

// stringSetDiff diffs two sets of strings with the given name.
func stringSetDiff(old, new []string, name string, contextual bool) *ObjectDiff {
	oldMap := make(map[string]struct{}, len(old))
	newMap := make(map[string]struct{}, len(new))
	for _, o := range old {
		oldMap[o] = struct{}{}
	}
	for _, n := range new {
		newMap[n] = struct{}{}
	}
	if reflect.DeepEqual(oldMap, newMap) && !contextual {
		return nil
	}

	diff := &ObjectDiff{Name: name}
	var added, removed bool
	for k := range oldMap {
		if _, ok := newMap[k]; !ok {
			diff.Fields = append(diff.Fields, fieldDiff(k, "", name, contextual))
			removed = true
		} else if contextual {
			diff.Fields = append(diff.Fields, fieldDiff(k, k, name, contextual))
		}
	}

	for k := range newMap {
		if _, ok := oldMap[k]; !ok {
			diff.Fields = append(diff.Fields, fieldDiff("", k, name, contextual))
			added = true
		}
	}

	sort.Sort(FieldDiffs(diff.Fields))

	// Determine the type
	if added && removed {
		diff.Type = DiffTypeEdited
	} else if added {
		diff.Type = DiffTypeAdded
	} else if removed {
		diff.Type = DiffTypeDeleted
	} else {
		// Diff of an empty set
		if len(diff.Fields) == 0 {
			return nil
		}

		diff.Type = DiffTypeNone
	}

	return diff
}

func periodicDiff(old, new *PeriodicConfig, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Periodic"}
	var oldPeriodicFlat, newPeriodicFlat map[string]string

	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		old = &PeriodicConfig{}
		diff.Type = DiffTypeAdded
		newPeriodicFlat = flatmap.Flatten(new, nil, true)
	} else if new == nil {
		new = &PeriodicConfig{}
		diff.Type = DiffTypeDeleted
		oldPeriodicFlat = flatmap.Flatten(old, nil, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPeriodicFlat = flatmap.Flatten(old, nil, true)
		newPeriodicFlat = flatmap.Flatten(new, nil, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPeriodicFlat, newPeriodicFlat, contextual)

	if setDiff := stringSetDiff(old.Specs, new.Specs, "Specs", contextual); setDiff != nil && setDiff.Type != DiffTypeNone {
		diff.Objects = append(diff.Objects, setDiff)
	}

	sort.Sort(FieldDiffs(diff.Fields))
	return diff
}

// primitiveObjectDiff returns a diff of the passed objects' primitive fields.
// The filter field can be used to exclude fields from the diff. The name is the
// name of the objects. If contextual is set, non-changed fields will also be
// stored in the object diff.
func primitiveObjectDiff(old, new interface{}, filter []string, name string, contextual bool) *ObjectDiff {
	oldPrimitiveFlat := flatmap.Flatten(old, filter, true)
	newPrimitiveFlat := flatmap.Flatten(new, filter, true)
	delete(oldPrimitiveFlat, "")
	delete(newPrimitiveFlat, "")

	diff := &ObjectDiff{Name: name}
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	var added, deleted, edited bool
Loop:
	for _, f := range diff.Fields {
		switch f.Type {
		case DiffTypeEdited:
			edited = true
			break Loop
		case DiffTypeDeleted:
			deleted = true
		case DiffTypeAdded:
			added = true
		}
	}

	if edited || added && deleted {
		diff.Type = DiffTypeEdited
	} else if added {
		diff.Type = DiffTypeAdded
	} else if deleted {
		diff.Type = DiffTypeDeleted
	} else {
		return nil
	}

	return diff
}

// primitiveObjectSetDiff does a set difference of the old and new sets. The
// filter parameter can be used to filter a set of primitive fields in the
// passed structs. The name corresponds to the name of the passed objects. If
// contextual diff is enabled, objects' primitive fields will be returned even if
// no diff exists.
func primitiveObjectSetDiff(old, new []interface{}, filter []string, name string, contextual bool) []*ObjectDiff {
	makeSet := func(objects []interface{}) map[string]interface{} {
		objMap := make(map[string]interface{}, len(objects))
		for _, obj := range objects {
			var key string

			if diffable, ok := obj.(DiffableWithID); ok {
				key = diffable.DiffID()
			}

			if key == "" {
				hash, err := hashstructure.Hash(obj, nil)
				if err != nil {
					panic(err)
				}
				key = fmt.Sprintf("%d", hash)
			}
			objMap[key] = obj
		}

		return objMap
	}

	oldSet := makeSet(old)
	newSet := makeSet(new)

	var diffs []*ObjectDiff
	for k, oldObj := range oldSet {
		newObj := newSet[k]
		diff := primitiveObjectDiff(oldObj, newObj, filter, name, contextual)
		if diff != nil {
			diffs = append(diffs, diff)
		}
	}
	for k, v := range newSet {
		// Added
		if _, ok := oldSet[k]; !ok {
			diffs = append(diffs, primitiveObjectDiff(nil, v, filter, name, contextual))
		}
	}

	sort.Sort(ObjectDiffs(diffs))
	return diffs
}

// interfaceSlice is a helper method that takes a slice of typed elements and
// returns a slice of interface. This method will panic if given a non-slice
// input.
func interfaceSlice(slice interface{}) []interface{} {
	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		panic("InterfaceSlice() given a non-slice type")
	}

	ret := make([]interface{}, s.Len())

	for i := 0; i < s.Len(); i++ {
		ret[i] = s.Index(i).Interface()
	}

	return ret
}
