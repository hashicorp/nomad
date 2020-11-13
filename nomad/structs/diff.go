package structs

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/helper/flatmap"
	"github.com/mitchellh/hashstructure"
)

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
		"ModifyIndex", "JobModifyIndex", "Update", "SubmitTime", "NomadTokenID"}

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
	if pDiff := primitiveObjectDiff(j.Periodic, other.Periodic, nil, "Periodic", contextual); pDiff != nil {
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

	// Update diff
	// COMPAT: Remove "Stagger" in 0.7.0.
	if uDiff := primitiveObjectDiff(tg.Update, other.Update, []string{"Stagger"}, "Update", contextual); uDiff != nil {
		diff.Objects = append(diff.Objects, uDiff)
	}

	// Network Resources diff
	if nDiffs := networkResourceDiffs(tg.Networks, other.Networks, contextual); nDiffs != nil {
		diff.Objects = append(diff.Objects, nDiffs...)
	}

	// Services diff
	if sDiffs := serviceDiffs(tg.Services, other.Services, contextual); sDiffs != nil {
		diff.Objects = append(diff.Objects, sDiffs...)
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

	// Template diff
	tmplDiffs := primitiveObjectSetDiff(
		interfaceSlice(t.Templates),
		interfaceSlice(other.Templates),
		nil,
		"Template",
		contextual)
	if tmplDiffs != nil {
		diff.Objects = append(diff.Objects, tmplDiffs...)
	}

	return diff, nil
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

	return diff
}

// serviceDiffs diffs a set of services. If contextual diff is enabled, unchanged
// fields within objects nested in the tasks will be returned.
func serviceDiffs(old, new []*Service, contextual bool) []*ObjectDiff {
	oldMap := make(map[string]*Service, len(old))
	newMap := make(map[string]*Service, len(new))
	for _, o := range old {
		oldMap[o.Name] = o
	}
	for _, n := range new {
		newMap[n.Name] = n
	}

	var diffs []*ObjectDiff
	for name, oldService := range oldMap {
		// Diff the same, deleted and edited
		if diff := serviceDiff(oldService, newMap[name], contextual); diff != nil {
			diffs = append(diffs, diff)
		}
	}

	for name, newService := range newMap {
		// Diff the added
		if old, ok := oldMap[name]; !ok {
			if diff := serviceDiff(old, newService, contextual); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}

	sort.Sort(ObjectDiffs(diffs))
	return diffs
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

	// Diff the ConsulGatewayIngress fields.
	gatewayIngressDiff := connectGatewayIngressDiff(prev.Ingress, next.Ingress, contextual)
	if gatewayIngressDiff != nil {
		diff.Objects = append(diff.Objects, gatewayIngressDiff)
	}

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

func connectGatewayTLSConfigDiff(prev, next *ConsulGatewayTLSConfig, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "TLS"}
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

	// Diff the primitive field.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	return diff
}

// connectGatewayIngressListenersDiff diffs are a set of listeners keyed by "protocol/port", which is
// a nifty workaround having slices instead of maps. Presumably such a key will be unique, because if
// it is not the config entry is not going to work anyway.
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

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Diff the hosts.
	if hDiffs := stringSetDiff(prev.Hosts, next.Hosts, "Hosts", contextual); hDiffs != nil {
		diff.Objects = append(diff.Objects, hDiffs)
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
			oldPrimitiveFlat["ConnectTimeout"] = fmt.Sprintf("%s", *prev.ConnectTimeout)
		}
		if next.ConnectTimeout == nil {
			newPrimitiveFlat["ConnectTimeout"] = ""
		} else {
			newPrimitiveFlat["ConnectTimeout"] = fmt.Sprintf("%s", *next.ConnectTimeout)
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
		prevMap[k] = fmt.Sprintf("%s:%d", v.Address, v.Port)
	}

	for k, v := range next {
		nextMap[k] = fmt.Sprintf("%s:%d", v.Address, v.Port)
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

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	consulUpstreamsDiff := primitiveObjectSetDiff(
		interfaceSlice(old.Upstreams),
		interfaceSlice(new.Upstreams),
		nil, "ConsulUpstreams", contextual)
	if consulUpstreamsDiff != nil {
		diff.Objects = append(diff.Objects, consulUpstreamsDiff...)
	}

	// Config diff
	if cDiff := configDiff(old.Config, new.Config, contextual); cDiff != nil {
		diff.Objects = append(diff.Objects, cDiff)
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

	return diff
}

// Diff returns a diff of two network resources. If contextual diff is enabled,
// non-changed fields will still be returned.
func (r *NetworkResource) Diff(other *NetworkResource, contextual bool) *ObjectDiff {
	diff := &ObjectDiff{Type: DiffTypeNone, Name: "Network"}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string
	filter := []string{"Device", "CIDR", "IP"}

	if reflect.DeepEqual(r, other) {
		return nil
	} else if r == nil {
		r = &NetworkResource{}
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatmap.Flatten(other, filter, true)
	} else if other == nil {
		other = &NetworkResource{}
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatmap.Flatten(r, filter, true)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatmap.Flatten(r, filter, true)
		newPrimitiveFlat = flatmap.Flatten(other, filter, true)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	// Port diffs
	resPorts := portDiffs(r.ReservedPorts, other.ReservedPorts, false, contextual)
	dynPorts := portDiffs(r.DynamicPorts, other.DynamicPorts, true, contextual)
	if resPorts != nil {
		diff.Objects = append(diff.Objects, resPorts...)
	}
	if dynPorts != nil {
		diff.Objects = append(diff.Objects, dynPorts...)
	}

	if dnsDiff := r.DNS.Diff(other.DNS, contextual); dnsDiff != nil {
		diff.Objects = append(diff.Objects, dnsDiff)
	}

	return diff
}

// Diff returns a diff of two DNSConfig structs
func (c *DNSConfig) Diff(other *DNSConfig, contextual bool) *ObjectDiff {
	if reflect.DeepEqual(c, other) {
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
	if c == nil {
		diff.Type = DiffTypeAdded
		newPrimitiveFlat = flatten(other)
	} else if other == nil {
		diff.Type = DiffTypeDeleted
		oldPrimitiveFlat = flatten(c)
	} else {
		diff.Type = DiffTypeEdited
		oldPrimitiveFlat = flatten(c)
		newPrimitiveFlat = flatten(other)
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat, contextual)

	return diff
}

// networkResourceDiffs diffs a set of NetworkResources. If contextual diff is enabled,
// non-changed fields will still be returned.
func networkResourceDiffs(old, new []*NetworkResource, contextual bool) []*ObjectDiff {
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

	var filter []string
	name := "Static Port"
	if dynamic {
		filter = []string{"Value"}
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
	for k, v := range oldSet {
		// Deleted
		if _, ok := newSet[k]; !ok {
			diffs = append(diffs, primitiveObjectDiff(v, nil, filter, name, contextual))
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
