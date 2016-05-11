package structs

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/helper/flatmap"
	"github.com/mitchellh/hashstructure"
)

// TODO: Support contextual diff

const (
	// AnnotationForcesDestructiveUpdate marks a diff as causing a destructive
	// update.
	AnnotationForcesDestructiveUpdate = "forces create/destroy update"

	// AnnotationForcesInplaceUpdate marks a diff as causing an in-place
	// update.
	AnnotationForcesInplaceUpdate = "forces in-place update"
)

// UpdateTypes denote the type of update to occur against the task group.
const (
	UpdateTypeIgnore            = "ignore"
	UpdateTypeCreate            = "create"
	UpdateTypeDestroy           = "destroy"
	UpdateTypeMigrate           = "migrate"
	UpdateTypeInplaceUpdate     = "in-place update"
	UpdateTypeDestructiveUpdate = "create/destroy update"
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
// diffable.
func (j *Job) Diff(other *Job) (*JobDiff, error) {
	diff := &JobDiff{Type: DiffTypeNone}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string
	filter := []string{"ID", "Status", "StatusDescription", "CreateIndex", "ModifyIndex", "JobModifyIndex"}

	// TODO This logic is too complicated
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
		if !reflect.DeepEqual(j, other) {
			diff.Type = DiffTypeEdited
		}

		if j.ID != other.ID {
			return nil, fmt.Errorf("can not diff jobs with different IDs: %q and %q", j.ID, other.ID)
		}

		oldPrimitiveFlat = flatmap.Flatten(j, filter, true)
		newPrimitiveFlat = flatmap.Flatten(other, filter, true)
		diff.ID = other.ID
	}

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat)

	// Datacenters diff
	if setDiff := stringSetDiff(j.Datacenters, other.Datacenters, "Datacenters"); setDiff != nil {
		diff.Objects = append(diff.Objects, setDiff)
	}

	// Constraints diff
	conDiff := primitiveObjectSetDiff(
		interfaceSlice(j.Constraints),
		interfaceSlice(other.Constraints),
		[]string{"str"},
		"Constraint")
	if conDiff != nil {
		diff.Objects = append(diff.Objects, conDiff...)
	}

	// Task groups diff
	tgs, err := taskGroupDiffs(j.TaskGroups, other.TaskGroups)
	if err != nil {
		return nil, err
	}
	diff.TaskGroups = tgs

	// Update diff
	if uDiff := primitiveObjectDiff(j.Update, other.Update, nil, "Update"); uDiff != nil {
		diff.Objects = append(diff.Objects, uDiff)
	}

	// Periodic diff
	if pDiff := primitiveObjectDiff(j.Periodic, other.Periodic, nil, "Periodic"); pDiff != nil {
		diff.Objects = append(diff.Objects, pDiff)
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
	Updates map[string]int
}

// Diff returns a diff of two task groups.
func (tg *TaskGroup) Diff(other *TaskGroup) (*TaskGroupDiff, error) {
	diff := &TaskGroupDiff{Type: DiffTypeNone}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string
	filter := []string{"Name"}

	// TODO This logic is too complicated
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

	// Diff the primitive fields.
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat)

	// Constraints diff
	conDiff := primitiveObjectSetDiff(
		interfaceSlice(tg.Constraints),
		interfaceSlice(other.Constraints),
		[]string{"str"},
		"Constraint")
	if conDiff != nil {
		diff.Objects = append(diff.Objects, conDiff...)
	}

	// Restart policy diff
	if rDiff := primitiveObjectDiff(tg.RestartPolicy, other.RestartPolicy, nil, "RestartPolicy"); rDiff != nil {
		diff.Objects = append(diff.Objects, rDiff)
	}

	// Tasks diff
	tasks, err := taskDiffs(tg.Tasks, other.Tasks)
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

// TaskGroupDiffs diffs two sets of task groups.
func taskGroupDiffs(old, new []*TaskGroup) ([]*TaskGroupDiff, error) {
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
		diff, err := oldGroup.Diff(newMap[name])
		if err != nil {
			return nil, err
		}
		diffs = append(diffs, diff)
	}

	for name, newGroup := range newMap {
		// Diff the added
		if old, ok := oldMap[name]; !ok {
			diff, err := old.Diff(newGroup)
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

// Diff returns a diff of two tasks.
func (t *Task) Diff(other *Task) (*TaskDiff, error) {
	diff := &TaskDiff{Type: DiffTypeNone}
	var oldPrimitiveFlat, newPrimitiveFlat map[string]string
	filter := []string{"Name", "Config"}

	// TODO This logic is too complicated
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
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat)

	// Constraints diff
	conDiff := primitiveObjectSetDiff(
		interfaceSlice(t.Constraints),
		interfaceSlice(other.Constraints),
		[]string{"str"},
		"Constraint")
	if conDiff != nil {
		diff.Objects = append(diff.Objects, conDiff...)
	}

	// Config diff
	if cDiff := configDiff(t.Config, other.Config); cDiff != nil {
		diff.Objects = append(diff.Objects, cDiff)
	}

	// Resources diff
	if rDiff := t.Resources.Diff(other.Resources); rDiff != nil {
		diff.Objects = append(diff.Objects, rDiff)
	}

	// LogConfig diff
	if lDiff := primitiveObjectDiff(t.LogConfig, other.LogConfig, nil, "LogConfig"); lDiff != nil {
		diff.Objects = append(diff.Objects, lDiff)
	}

	// Artifacts diff
	diffs := primitiveObjectSetDiff(
		interfaceSlice(t.Artifacts),
		interfaceSlice(other.Artifacts),
		nil,
		"Artifact")
	if diffs != nil {
		diff.Objects = append(diff.Objects, diffs...)
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

// taskDiffs diffs a set of tasks.
func taskDiffs(old, new []*Task) ([]*TaskDiff, error) {
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
		diff, err := oldGroup.Diff(newMap[name])
		if err != nil {
			return nil, err
		}
		diffs = append(diffs, diff)
	}

	for name, newGroup := range newMap {
		// Diff the added
		if old, ok := oldMap[name]; !ok {
			diff, err := old.Diff(newGroup)
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

// Diff returns a diff of two resource objects.
func (r *Resources) Diff(other *Resources) *ObjectDiff {
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
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat)

	// Network Resources diff
	if nDiffs := networkResourceDiffs(r.Networks, other.Networks); nDiffs != nil {
		diff.Objects = append(diff.Objects, nDiffs...)
	}

	return diff
}

// Diff returns a diff of two network resources.
func (r *NetworkResource) Diff(other *NetworkResource) *ObjectDiff {
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
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat)

	// Port diffs
	if resPorts := portDiffs(r.ReservedPorts, other.ReservedPorts, false); resPorts != nil {
		diff.Objects = append(diff.Objects, resPorts...)
	}
	if dynPorts := portDiffs(r.DynamicPorts, other.DynamicPorts, true); dynPorts != nil {
		diff.Objects = append(diff.Objects, dynPorts...)
	}

	return diff
}

// networkResourceDiffs diffs a set of NetworkResources.
func networkResourceDiffs(old, new []*NetworkResource) []*ObjectDiff {
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
			if diff := oldV.Diff(newV); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}
	for k, newV := range newSet {
		if oldV, ok := oldSet[k]; !ok {
			if diff := oldV.Diff(newV); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}

	sort.Sort(ObjectDiffs(diffs))
	return diffs

}

// portDiffs returns the diff of two sets of ports. The dynamic flag marks the
// set of ports as being Dynamic ports versus Static ports.
func portDiffs(old, new []Port, dynamic bool) []*ObjectDiff {
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
			if diff := primitiveObjectDiff(oldPort, newPort, filter, name); diff != nil {
				diffs = append(diffs, diff)
			}
		} else {
			if diff := primitiveObjectDiff(oldPort, nil, filter, name); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}
	for label, newPort := range newPorts {
		// Diff the added
		if _, ok := oldPorts[label]; !ok {
			if diff := primitiveObjectDiff(nil, newPort, filter, name); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}

	sort.Sort(ObjectDiffs(diffs))
	return diffs

}

// configDiff returns the diff of two Task Config objects.
func configDiff(old, new map[string]interface{}) *ObjectDiff {
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
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat)
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
	Type     DiffType
	Name     string
	Old, New string
}

// NewFieldDiff returns a FieldDiff if old and new are different otherwise, it
// returns nil.
func NewFieldDiff(old, new, name string) *FieldDiff {
	if old == new {
		return nil
	}

	diff := &FieldDiff{Name: name}
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
	return fmt.Sprintf("%q (%s): %q => %q", f.Name, f.Type, f.Old, f.New)
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
// field diffs.
func fieldDiffs(old, new map[string]string) []*FieldDiff {
	var diffs []*FieldDiff
	visited := make(map[string]struct{})
	for k, oldV := range old {
		visited[k] = struct{}{}
		newV := new[k]
		if diff := NewFieldDiff(oldV, newV, k); diff != nil {
			diffs = append(diffs, diff)
		}
	}

	for k, newV := range new {
		if _, ok := visited[k]; !ok {
			if diff := NewFieldDiff("", newV, k); diff != nil {
				diffs = append(diffs, diff)
			}
		}
	}

	sort.Sort(FieldDiffs(diffs))
	return diffs
}

// stringSetDiff diffs two sets of strings with the given name.
func stringSetDiff(old, new []string, name string) *ObjectDiff {
	oldMap := make(map[string]struct{}, len(old))
	newMap := make(map[string]struct{}, len(new))
	for _, o := range old {
		oldMap[o] = struct{}{}
	}
	for _, n := range new {
		newMap[n] = struct{}{}
	}
	if reflect.DeepEqual(oldMap, newMap) {
		return nil
	}

	diff := &ObjectDiff{Name: name}
	var added, removed bool
	for k := range oldMap {
		if _, ok := newMap[k]; !ok {
			diff.Fields = append(diff.Fields, NewFieldDiff(k, "", name))
			removed = true
		}
	}

	for k := range newMap {
		if _, ok := oldMap[k]; !ok {
			diff.Fields = append(diff.Fields, NewFieldDiff("", k, name))
			added = true
		}
	}

	sort.Sort(FieldDiffs(diff.Fields))

	// Determine the type
	if added && removed {
		diff.Type = DiffTypeEdited
	} else if added {
		diff.Type = DiffTypeAdded
	} else {
		diff.Type = DiffTypeDeleted
	}

	return diff
}

// primitiveObjectDiff returns a diff of the passed objects' primitive fields.
// The filter field can be used to exclude fields from the diff. The name is the
// name of the objects.
func primitiveObjectDiff(old, new interface{}, filter []string, name string) *ObjectDiff {
	oldPrimitiveFlat := flatmap.Flatten(old, filter, true)
	newPrimitiveFlat := flatmap.Flatten(new, filter, true)
	delete(oldPrimitiveFlat, "")
	delete(newPrimitiveFlat, "")

	diff := &ObjectDiff{Name: name}
	diff.Fields = fieldDiffs(oldPrimitiveFlat, newPrimitiveFlat)

	var added, deleted, edited bool
	for _, f := range diff.Fields {
		switch f.Type {
		case DiffTypeEdited:
			edited = true
			break
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
// passed structs. The name corresponds to the name of the passed objects.
func primitiveObjectSetDiff(old, new []interface{}, filter []string, name string) []*ObjectDiff {
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
		if _, ok := newSet[k]; !ok {
			diffs = append(diffs, primitiveObjectDiff(v, nil, filter, name))
		}
	}
	for k, v := range newSet {
		if _, ok := oldSet[k]; !ok {
			diffs = append(diffs, primitiveObjectDiff(nil, v, filter, name))
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
