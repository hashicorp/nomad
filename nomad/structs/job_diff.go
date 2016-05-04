package structs

import (
	"fmt"
	"reflect"

	"github.com/hashicorp/nomad/helper/flatmap"
	"github.com/mitchellh/hashstructure"
)

// The below are the set of primitive fields that can be diff'd automatically
// using FieldDiff.
var (
	jobPrimitiveFields = []string{
		"Region",
		"ID",
		"ParentID",
		"Name",
		"Type",
		"Priority",
		"AllAtOnce",
	}

	constraintFields = []string{
		"LTarget",
		"RTarget",
		"Operand",
	}

	updateStrategyFields = []string{
		"Stagger",
		"MaxParallel",
	}

	periodicConfigFields = []string{
		"Enabled",
		"Spec",
		"SpecType",
		"ProhibitOverlap",
	}

	taskGroupPrimitiveFields = []string{
		"Name",
		"Count",
	}

	restartPolicyFields = []string{
		"Attempts",
		"Interval",
		"Delay",
		"Mode",
	}

	taskPrimitiveFields = []string{
		"Name",
		"Driver",
		"User",
		"KillTimeout",
	}

	logConfigFields = []string{
		"MaxFiles",
		"MaxFileSizeMB",
	}

	servicePrimitiveFields = []string{
		"Name",
		"PortLabel",
	}

	serviceCheckPrimitiveFields = []string{
		"Name",
		"Type",
		"Command",
		"Path",
		"Protocol",
		"Interval",
		"Timeout",
	}

	taskArtifactPrimitiveFields = []string{
		"GetterSource",
		"RelativeDest",
	}

	resourcesPrimitiveFields = []string{
		"CPU",
		"MemoryMB",
		"DiskMB",
		"IOPS",
	}

	networkResourcePrimitiveFields = []string{
		"Device",
		"CIDR",
		"IP",
		"MBits",
	}

	portFields = []string{
		"Label",
		"Value",
	}
)

// DiffType is the high-level type of the diff.
type DiffType string

const (
	DiffTypeNone    DiffType = "Equal"
	DiffTypeAdded            = "Added"
	DiffTypeDeleted          = "Deleted"
	DiffTypeEdited           = "Edited"
)

// DiffEntry contains information about a diff.
type DiffEntry struct {
	Type        DiffType
	Annotations []string
}

// SetDiffType sets the diff type. The inputs must be a pointer.
func (d *DiffEntry) SetDiffType(old, new interface{}) {
	if reflect.DeepEqual(old, new) {
		d.Type = DiffTypeNone
		return
	}

	oldV := reflect.ValueOf(old)
	newV := reflect.ValueOf(new)

	if oldV.IsNil() {
		d.Type = DiffTypeAdded
	} else if newV.IsNil() {
		d.Type = DiffTypeDeleted
	} else {
		d.Type = DiffTypeEdited
	}
}

// JobDiff contains the set of changes betwen two Jobs.
type JobDiff struct {
	PrimitiveStructDiff
	Constraints []*PrimitiveStructDiff
	Datacenters *StringSetDiff
	Update      *PrimitiveStructDiff
	Periodic    *PrimitiveStructDiff
	Meta        *StringMapDiff
	TaskGroups  *TaskGroupsDiff
}

// TaskGroupsDiff contains the set of Task Groups that were changed.
type TaskGroupsDiff struct {
	DiffEntry
	Added, Deleted []*TaskGroup
	Edited         []*TaskGroupDiff
}

// TaskGroupsDiff contains the set of changes between two Task Groups.
type TaskGroupDiff struct {
	PrimitiveStructDiff
	Constraints   []*PrimitiveStructDiff
	RestartPolicy *PrimitiveStructDiff
	Meta          *StringMapDiff
	Tasks         *TasksDiff
}

// TasksDiff contains the set of Tasks that were changed.
type TasksDiff struct {
	DiffEntry
	Added, Deleted []*Task
	Edited         []*TaskDiff
}

// TaskDiff contains the changes between two Tasks.
type TaskDiff struct {
	PrimitiveStructDiff
	Constraints []*PrimitiveStructDiff
	LogConfig   *PrimitiveStructDiff
	Env         *StringMapDiff
	Meta        *StringMapDiff
	Services    *ServicesDiff
	Artifacts   *TaskArtifactsDiff
	Resources   *ResourcesDiff
	Config      *StringMapDiff
}

// ServicesDiff contains the set of Services that were changed.
type ServicesDiff struct {
	DiffEntry
	Added, Deleted []*Service
	Edited         []*ServiceDiff
}

// ServiceDiff contains the changes between two Services.
type ServiceDiff struct {
	PrimitiveStructDiff
	Tags   *StringSetDiff
	Checks *ServiceChecksDiff
}

// ServiceChecksDiff contains the set of Service Checks that were changed.
type ServiceChecksDiff struct {
	DiffEntry
	Added, Deleted []*ServiceCheck
	Edited         []*ServiceCheckDiff
}

// ServiceCheckDiff contains the changes between two Service Checks.
type ServiceCheckDiff struct {
	PrimitiveStructDiff
	Args *StringSetDiff
}

// TaskArtifactsDiff contains the set of Task Artifacts that were changed.
type TaskArtifactsDiff struct {
	DiffEntry
	Added, Deleted []*TaskArtifact
	Edited         []*TaskArtifactDiff
}

// TaskArtifactDiff contains the diff between two Task Artifacts.
type TaskArtifactDiff struct {
	PrimitiveStructDiff
	GetterOptions *StringMapDiff
}

// ResourcesDiff contains the diff between two Resources.
type ResourcesDiff struct {
	PrimitiveStructDiff
	Networks *NetworkResourcesDiff
}

// NetworkResourcesDiff contains the set of Network Resources that were changed.
type NetworkResourcesDiff struct {
	DiffEntry
	Added, Deleted []*NetworkResourceDiff
}

// NetworkResourceDiff contains the diff between two Network Resources.
type NetworkResourceDiff struct {
	PrimitiveStructDiff
	ReservedPorts *PortsDiff
	DynamicPorts  *PortsDiff
}

// PortsDiff contains the difference between two sets of Ports.
type PortsDiff struct {
	DiffEntry
	Added, Deleted []Port
	Edited         []*PrimitiveStructDiff
}

// PrimitiveStructDiff contains the diff of two structs that only contain
// primitive fields.
type PrimitiveStructDiff struct {
	DiffEntry
	PrimitiveFields map[string]*FieldDiff
}

// DiffFields performs the diff of the passed fields against the old and new
// object.
func (p *PrimitiveStructDiff) DiffFields(old, new interface{}, fields []string) {
	for _, field := range fields {
		oldV := getField(old, field)
		newV := getField(new, field)
		pDiff := NewFieldDiff(field, oldV, newV)
		if pDiff != nil {
			if p.PrimitiveFields == nil {
				p.PrimitiveFields = make(map[string]*FieldDiff)
			}

			p.PrimitiveFields[field] = pDiff
		}
	}
}

// FieldDiff contains the diff between an old and new version of a field.
type FieldDiff struct {
	DiffEntry
	Name     string
	OldValue interface{}
	NewValue interface{}
}

// StringSetDiff captures the changes that occured between two sets of strings
type StringSetDiff struct {
	DiffEntry
	Added, Deleted []string
}

// StringMapDiff captures the changes that occured between two string maps
type StringMapDiff struct {
	DiffEntry
	Added, Deleted map[string]string
	Edited         map[string]StringValueDelta
}

// StringValueDelta holds the old and new value of a string.
type StringValueDelta struct {
	DiffEntry
	Old, New string
}

// NewJobDiff returns the diff between two jobs. If there is no difference, nil
// is returned.
func NewJobDiff(old, new *Job) *JobDiff {
	diff := &JobDiff{}
	diff.SetDiffType(old, new)
	if diff.Type == DiffTypeNone {
		return nil
	}

	// Get the diffs of the primitive fields
	diff.DiffFields(old, new, jobPrimitiveFields)

	// Protect accessing nil fields, this occurs after diffing the primitives so
	// that we can properly detect Added/Deleted fields.
	if old == nil {
		old = &Job{}
	}
	if new == nil {
		new = &Job{}
	}

	// Get the diff of the datacenters
	diff.Datacenters = NewStringSetDiff(old.Datacenters, new.Datacenters)

	// Get the diff of the constraints.
	diff.Constraints = setDiffPrimitiveStructs(
		interfaceSlice(old.Constraints),
		interfaceSlice(new.Constraints),
		constraintFields)

	// Get the update strategy diff
	diff.Update = NewPrimitiveStructDiff(old.Update, new.Update, updateStrategyFields)

	// Get the update strategy diff
	diff.Periodic = NewPrimitiveStructDiff(old.Periodic, new.Periodic, periodicConfigFields)

	// Get the meta diff
	diff.Meta = NewStringMapDiff(old.Meta, new.Meta)

	// Get the task group diff
	diff.TaskGroups = setDiffTaskGroups(old.TaskGroups, new.TaskGroups)

	// If there are no changes return nil
	if len(diff.PrimitiveFields)+len(diff.Constraints) == 0 &&
		diff.Datacenters == nil &&
		diff.Update == nil &&
		diff.Periodic == nil &&
		diff.Meta == nil &&
		diff.TaskGroups == nil {
		return nil
	}

	return diff
}

// NewTaskGroupDiff returns the diff between two task groups. If there is no
// difference, nil is returned.
func NewTaskGroupDiff(old, new *TaskGroup) *TaskGroupDiff {
	diff := &TaskGroupDiff{}
	diff.SetDiffType(old, new)
	if diff.Type == DiffTypeNone {
		return nil
	}

	// Get the diffs of the primitive fields
	diff.DiffFields(old, new, taskGroupPrimitiveFields)

	// Protect accessing nil fields, this occurs after diffing the primitives so
	// that we can properly detect Added/Deleted fields.
	if old == nil {
		old = &TaskGroup{}
	}
	if new == nil {
		new = &TaskGroup{}
	}

	// Get the diff of the constraints.
	diff.Constraints = setDiffPrimitiveStructs(
		interfaceSlice(old.Constraints),
		interfaceSlice(new.Constraints),
		constraintFields)

	// Get the restart policy diff
	diff.RestartPolicy = NewPrimitiveStructDiff(old.RestartPolicy, new.RestartPolicy, restartPolicyFields)

	// Get the meta diff
	diff.Meta = NewStringMapDiff(old.Meta, new.Meta)

	// Get the task diff
	diff.Tasks = setDiffTasks(old.Tasks, new.Tasks)

	// If there are no changes return nil
	if len(diff.PrimitiveFields)+len(diff.Constraints) == 0 &&
		diff.Tasks == nil &&
		diff.RestartPolicy == nil &&
		diff.Meta == nil {
		return nil
	}

	return diff
}

// NewTaskDiff returns the diff between two tasks. If there is no difference,
// nil is returned.
func NewTaskDiff(old, new *Task) *TaskDiff {
	diff := &TaskDiff{}
	diff.SetDiffType(old, new)
	if diff.Type == DiffTypeNone {
		return nil
	}

	// Get the diffs of the primitive fields
	diff.DiffFields(old, new, taskPrimitiveFields)

	// Protect accessing nil fields, this occurs after diffing the primitives so
	// that we can properly detect Added/Deleted fields.
	if old == nil {
		old = &Task{}
	}
	if new == nil {
		new = &Task{}
	}

	// Get the diff of the constraints.
	diff.Constraints = setDiffPrimitiveStructs(
		interfaceSlice(old.Constraints),
		interfaceSlice(new.Constraints),
		constraintFields)

	// Get the meta and env diff
	diff.Meta = NewStringMapDiff(old.Meta, new.Meta)
	diff.Env = NewStringMapDiff(old.Env, new.Env)

	// Get the log config diff
	diff.LogConfig = NewPrimitiveStructDiff(old.LogConfig, new.LogConfig, logConfigFields)

	// Get the services diff
	diff.Services = setDiffServices(old.Services, new.Services)

	// Get the artifacts diff
	diff.Artifacts = setDiffTaskArtifacts(old.Artifacts, new.Artifacts)

	// Get the resource diff
	diff.Resources = NewResourcesDiff(old.Resources, new.Resources)

	// Get the task config diff
	diff.Config = NewStringMapDiff(flatmap.Flatten(old.Config), flatmap.Flatten(new.Config))

	// If there are no changes return nil
	if len(diff.PrimitiveFields)+len(diff.Constraints) == 0 &&
		diff.Config == nil &&
		diff.Artifacts == nil &&
		diff.LogConfig == nil &&
		diff.Services == nil &&
		diff.Env == nil &&
		diff.Meta == nil {
		return nil
	}

	return diff
}

// NewServiceDiff returns the diff between two services. If there is no
// difference, nil is returned.
func NewServiceDiff(old, new *Service) *ServiceDiff {
	diff := &ServiceDiff{}
	diff.SetDiffType(old, new)
	if diff.Type == DiffTypeNone {
		return nil
	}

	// Get the diffs of the primitive fields
	diff.DiffFields(old, new, servicePrimitiveFields)

	// Protect accessing nil fields, this occurs after diffing the primitives so
	// that we can properly detect Added/Deleted fields.
	if old == nil {
		old = &Service{}
	}
	if new == nil {
		new = &Service{}
	}

	// Get the tags diff
	diff.Tags = NewStringSetDiff(old.Tags, new.Tags)

	// Get the checks diff
	diff.Checks = setDiffServiceChecks(old.Checks, new.Checks)

	// If there are no changes return nil
	if len(diff.PrimitiveFields) == 0 &&
		diff.Checks == nil &&
		diff.Tags == nil {
		return nil
	}

	return diff
}

// NewServiceCheckDiff returns the diff between two service checks. If there is
// no difference, nil is returned.
func NewServiceCheckDiff(old, new *ServiceCheck) *ServiceCheckDiff {
	diff := &ServiceCheckDiff{}
	diff.SetDiffType(old, new)
	if diff.Type == DiffTypeNone {
		return nil
	}

	// Get the diffs of the primitive fields
	diff.DiffFields(old, new, serviceCheckPrimitiveFields)

	// Protect accessing nil fields, this occurs after diffing the primitives so
	// that we can properly detect Added/Deleted fields.
	if old == nil {
		old = &ServiceCheck{}
	}
	if new == nil {
		new = &ServiceCheck{}
	}

	// Get the args diff
	diff.Args = NewStringSetDiff(old.Args, new.Args)

	// If there are no changes return nil
	if len(diff.PrimitiveFields) == 0 &&
		diff.Args == nil {
		return nil
	}

	return diff
}

// NewTaskArtifactDiff returns the diff between two task artifacts. If there is
// no difference, nil is returned.
func NewTaskArtifactDiff(old, new *TaskArtifact) *TaskArtifactDiff {
	diff := &TaskArtifactDiff{}
	diff.SetDiffType(old, new)
	if diff.Type == DiffTypeNone {
		return nil
	}

	// Get the diffs of the primitive fields
	diff.DiffFields(old, new, taskArtifactPrimitiveFields)

	// Protect accessing nil fields, this occurs after diffing the primitives so
	// that we can properly detect Added/Deleted fields.
	if old == nil {
		old = &TaskArtifact{}
	}
	if new == nil {
		new = &TaskArtifact{}
	}

	// Get the args diff
	diff.GetterOptions = NewStringMapDiff(old.GetterOptions, new.GetterOptions)

	// If there are no changes return nil
	if len(diff.PrimitiveFields) == 0 &&
		diff.GetterOptions == nil {
		return nil
	}

	return diff
}

// NewResourcesDiff returns the diff between two resources. If there is no
// difference, nil is returned.
func NewResourcesDiff(old, new *Resources) *ResourcesDiff {
	diff := &ResourcesDiff{}
	diff.SetDiffType(old, new)
	if diff.Type == DiffTypeNone {
		return nil
	}

	// Get the diffs of the primitive fields
	diff.DiffFields(old, new, resourcesPrimitiveFields)

	// Protect accessing nil fields, this occurs after diffing the primitives so
	// that we can properly detect Added/Deleted fields.
	if old == nil {
		old = &Resources{}
	}
	if new == nil {
		new = &Resources{}
	}

	// Get the network resource diff
	diff.Networks = setDiffNetworkResources(old.Networks, new.Networks)

	// If there are no changes return nil
	if len(diff.PrimitiveFields) == 0 &&
		diff.Networks == nil {
		return nil
	}

	return diff
}

// NewNetworkResourceDiff returns the diff between two network resources. If
// there is no difference, nil is returned.
func NewNetworkResourceDiff(old, new *NetworkResource) *NetworkResourceDiff {
	diff := &NetworkResourceDiff{}
	diff.SetDiffType(old, new)
	if diff.Type == DiffTypeNone {
		return nil
	}

	// Get the diffs of the primitive fields
	diff.DiffFields(old, new, networkResourcePrimitiveFields)

	// Protect accessing nil fields, this occurs after diffing the primitives so
	// that we can properly detect Added/Deleted fields.
	if old == nil {
		old = &NetworkResource{}
	}
	if new == nil {
		new = &NetworkResource{}
	}

	// Get the port diffs
	diff.ReservedPorts = setDiffPorts(old.ReservedPorts, new.ReservedPorts)
	diff.DynamicPorts = setDiffPorts(old.DynamicPorts, new.DynamicPorts)

	// If there are no changes return nil
	if len(diff.PrimitiveFields) == 0 &&
		diff.DynamicPorts == nil &&
		diff.ReservedPorts == nil {
		return nil
	}

	return diff
}

// NewPrimitiveStructDiff returns the diff between two structs containing only
// primitive fields. The list of fields to be diffed is passed via the fields
// parameter. If there is no difference, nil is returned.
func NewPrimitiveStructDiff(old, new interface{}, fields []string) *PrimitiveStructDiff {
	if reflect.DeepEqual(old, new) {
		return nil
	}

	// Diff the individual fields
	diff := &PrimitiveStructDiff{}
	diff.DiffFields(old, new, fields)
	if len(diff.PrimitiveFields) == 0 {
		return nil
	}

	var added, deleted bool
	for _, f := range diff.PrimitiveFields {
		switch f.Type {
		case DiffTypeEdited:
			diff.Type = DiffTypeEdited
			return diff
		case DiffTypeAdded:
			added = true
		case DiffTypeDeleted:
			deleted = true
		}
	}

	if added && deleted {
		diff.Type = DiffTypeEdited
	} else if added {
		diff.Type = DiffTypeAdded
	} else {
		diff.Type = DiffTypeDeleted
	}

	return diff
}

// NewFieldDiff returns the diff between two fields. If there is no difference,
// nil is returned.
func NewFieldDiff(name string, old, new interface{}) *FieldDiff {
	diff := &FieldDiff{Name: name}
	if reflect.DeepEqual(old, new) {
		return nil
	} else if old == nil {
		diff.Type = DiffTypeAdded
		diff.NewValue = new
	} else if new == nil {
		diff.Type = DiffTypeDeleted
		diff.OldValue = old
	} else {
		diff.Type = DiffTypeEdited
		diff.OldValue = old
		diff.NewValue = new
	}

	return diff
}

// NewStringSetDiff returns the diff between two sets of strings. If there is no
// difference, nil is returned.
func NewStringSetDiff(old, new []string) *StringSetDiff {
	if reflect.DeepEqual(old, new) {
		return nil
	}

	diff := &StringSetDiff{}
	makeMap := func(inputs []string) map[string]interface{} {
		m := make(map[string]interface{})
		for _, in := range inputs {
			m[in] = struct{}{}
		}
		return m
	}

	added, deleted, _, _ := keyedSetDifference(makeMap(old), makeMap(new))
	for k := range added {
		diff.Added = append(diff.Added, k)
	}
	for k := range deleted {
		diff.Deleted = append(diff.Deleted, k)
	}

	la, ld := len(added), len(deleted)
	if la+ld == 0 {
		return nil
	} else if ld == 0 {
		diff.Type = DiffTypeAdded
	} else if la == 0 {
		diff.Type = DiffTypeDeleted
	} else {
		diff.Type = DiffTypeEdited
	}

	return diff
}

// NewStringMapDiff returns the diff between two maps of strings. If there is no
// difference, nil is returned.
func NewStringMapDiff(old, new map[string]string) *StringMapDiff {
	if reflect.DeepEqual(old, new) {
		return nil
	}

	diff := &StringMapDiff{}
	diff.Added = make(map[string]string)
	diff.Deleted = make(map[string]string)
	diff.Edited = make(map[string]StringValueDelta)

	for k, v := range old {
		if _, ok := new[k]; !ok {
			diff.Deleted[k] = v
		}
	}
	for k, newV := range new {
		oldV, ok := old[k]
		if !ok {
			diff.Added[k] = newV
			continue
		}

		// Key is in both, check if they have been edited.
		if newV != oldV {
			d := StringValueDelta{Old: oldV, New: newV}
			d.Type = DiffTypeEdited
			diff.Edited[k] = d
		}
	}

	la, ld, le := len(diff.Added), len(diff.Deleted), len(diff.Edited)
	if la+ld+le == 0 {
		return nil
	}

	if le != 0 || la > 0 && ld > 0 {
		diff.Type = DiffTypeEdited
	} else if ld == 0 {
		diff.Type = DiffTypeAdded
	} else if la == 0 {
		diff.Type = DiffTypeDeleted
	}
	return diff
}

// Set helpers

// setDiffTaskGroups does a set difference of task groups using the task group
// name as a key.
func setDiffTaskGroups(old, new []*TaskGroup) *TaskGroupsDiff {
	diff := &TaskGroupsDiff{}

	oldMap := make(map[string]*TaskGroup)
	newMap := make(map[string]*TaskGroup)
	for _, tg := range old {
		oldMap[tg.Name] = tg
	}
	for _, tg := range new {
		newMap[tg.Name] = tg
	}

	for k, v := range oldMap {
		if _, ok := newMap[k]; !ok {
			diff.Deleted = append(diff.Deleted, v)
		}
	}
	for k, newV := range newMap {
		oldV, ok := oldMap[k]
		if !ok {
			diff.Added = append(diff.Added, newV)
			continue
		}

		// Key is in both, check if they have been edited.
		if !reflect.DeepEqual(oldV, newV) {
			tgdiff := NewTaskGroupDiff(oldV, newV)
			diff.Edited = append(diff.Edited, tgdiff)
		}
	}

	if len(diff.Added)+len(diff.Deleted)+len(diff.Edited) == 0 {
		return nil
	}
	return diff
}

// setDiffTasks does a set difference of tasks using the task name as a key.
func setDiffTasks(old, new []*Task) *TasksDiff {
	diff := &TasksDiff{}

	oldMap := make(map[string]*Task)
	newMap := make(map[string]*Task)
	for _, task := range old {
		oldMap[task.Name] = task
	}
	for _, task := range new {
		newMap[task.Name] = task
	}

	for k, v := range oldMap {
		if _, ok := newMap[k]; !ok {
			diff.Deleted = append(diff.Deleted, v)
		}
	}
	for k, newV := range newMap {
		oldV, ok := oldMap[k]
		if !ok {
			diff.Added = append(diff.Added, newV)
			continue
		}

		// Key is in both, check if they have been edited.
		if !reflect.DeepEqual(oldV, newV) {
			tdiff := NewTaskDiff(oldV, newV)
			diff.Edited = append(diff.Edited, tdiff)
		}
	}

	if len(diff.Added)+len(diff.Deleted)+len(diff.Edited) == 0 {
		return nil
	}
	return diff
}

// setDiffServices does a set difference of Services using the service name as a
// key.
func setDiffServices(old, new []*Service) *ServicesDiff {
	diff := &ServicesDiff{}

	oldMap := make(map[string]*Service)
	newMap := make(map[string]*Service)
	for _, s := range old {
		oldMap[s.Name] = s
	}
	for _, s := range new {
		newMap[s.Name] = s
	}

	for k, v := range oldMap {
		if _, ok := newMap[k]; !ok {
			diff.Deleted = append(diff.Deleted, v)
		}
	}
	for k, newV := range newMap {
		oldV, ok := oldMap[k]
		if !ok {
			diff.Added = append(diff.Added, newV)
			continue
		}

		// Key is in both, check if they have been edited.
		if !reflect.DeepEqual(oldV, newV) {
			sdiff := NewServiceDiff(oldV, newV)
			diff.Edited = append(diff.Edited, sdiff)
		}
	}

	if len(diff.Added)+len(diff.Deleted)+len(diff.Edited) == 0 {
		return nil
	}
	return diff
}

// setDiffServiceChecks does a set difference of service checks using the check
// name as a key.
func setDiffServiceChecks(old, new []*ServiceCheck) *ServiceChecksDiff {
	diff := &ServiceChecksDiff{}

	oldMap := make(map[string]*ServiceCheck)
	newMap := make(map[string]*ServiceCheck)
	for _, s := range old {
		oldMap[s.Name] = s
	}
	for _, s := range new {
		newMap[s.Name] = s
	}

	for k, v := range oldMap {
		if _, ok := newMap[k]; !ok {
			diff.Deleted = append(diff.Deleted, v)
		}
	}
	for k, newV := range newMap {
		oldV, ok := oldMap[k]
		if !ok {
			diff.Added = append(diff.Added, newV)
			continue
		}

		// Key is in both, check if they have been edited.
		if !reflect.DeepEqual(oldV, newV) {
			sdiff := NewServiceCheckDiff(oldV, newV)
			diff.Edited = append(diff.Edited, sdiff)
		}
	}

	if len(diff.Added)+len(diff.Deleted)+len(diff.Edited) == 0 {
		return nil
	}
	return diff
}

// setDiffTaskArtifacts does a set difference of task artifacts using the geter
// source as a key.
func setDiffTaskArtifacts(old, new []*TaskArtifact) *TaskArtifactsDiff {
	diff := &TaskArtifactsDiff{}

	oldMap := make(map[string]*TaskArtifact)
	newMap := make(map[string]*TaskArtifact)
	for _, ta := range old {
		oldMap[ta.GetterSource] = ta
	}
	for _, ta := range new {
		newMap[ta.GetterSource] = ta
	}

	for k, v := range oldMap {
		if _, ok := newMap[k]; !ok {
			diff.Deleted = append(diff.Deleted, v)
		}
	}
	for k, newV := range newMap {
		oldV, ok := oldMap[k]
		if !ok {
			diff.Added = append(diff.Added, newV)
			continue
		}

		// Key is in both, check if they have been edited.
		if !reflect.DeepEqual(oldV, newV) {
			tdiff := NewTaskArtifactDiff(oldV, newV)
			diff.Edited = append(diff.Edited, tdiff)
		}
	}

	if len(diff.Added)+len(diff.Deleted)+len(diff.Edited) == 0 {
		return nil
	}
	return diff
}

// setDiffNetworkResources does a set difference of network resources.
func setDiffNetworkResources(old, new []*NetworkResource) *NetworkResourcesDiff {
	diff := &NetworkResourcesDiff{}

	added, del := setDifference(interfaceSlice(old), interfaceSlice(new))
	for _, a := range added {
		nDiff := NewNetworkResourceDiff(nil, a.(*NetworkResource))
		diff.Added = append(diff.Added, nDiff)
	}
	for _, d := range del {
		nDiff := NewNetworkResourceDiff(d.(*NetworkResource), nil)
		diff.Added = append(diff.Deleted, nDiff)
	}

	return diff
}

// setDiffPorts does a set difference of ports using the label as a key.
func setDiffPorts(old, new []Port) *PortsDiff {
	diff := &PortsDiff{}

	oldMap := make(map[string]Port)
	newMap := make(map[string]Port)
	for _, p := range old {
		oldMap[p.Label] = p
	}
	for _, p := range new {
		newMap[p.Label] = p
	}

	for k, v := range oldMap {
		if _, ok := newMap[k]; !ok {
			diff.Deleted = append(diff.Deleted, v)
		}
	}
	for k, newV := range newMap {
		oldV, ok := oldMap[k]
		if !ok {
			diff.Added = append(diff.Added, newV)
			continue
		}

		// Key is in both, check if they have been edited.
		if !reflect.DeepEqual(oldV, newV) {
			pdiff := NewPrimitiveStructDiff(oldV, newV, portFields)
			diff.Edited = append(diff.Edited, pdiff)
		}
	}

	if len(diff.Added)+len(diff.Deleted)+len(diff.Edited) == 0 {
		return nil
	}
	return diff
}

// setDiffPrimitiveStructs does a set difference on primitive structs. The
// caller must pass the primitive structs fields.
func setDiffPrimitiveStructs(old, new []interface{}, fields []string) []*PrimitiveStructDiff {
	var diffs []*PrimitiveStructDiff

	added, del := setDifference(old, new)
	for _, a := range added {
		pDiff := NewPrimitiveStructDiff(nil, a, fields)
		diffs = append(diffs, pDiff)
	}
	for _, d := range del {
		pDiff := NewPrimitiveStructDiff(d, nil, fields)
		diffs = append(diffs, pDiff)
	}

	return diffs
}

// Reflective helpers.

// setDifference does a set difference on two sets of interfaces and returns the
// values that were added or deleted when comparing the new to old.
func setDifference(old, new []interface{}) (added, deleted []interface{}) {
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

	addedMap, deletedMap, _, _ := keyedSetDifference(makeSet(old), makeSet(new))
	flatten := func(in map[string]interface{}) []interface{} {
		out := make([]interface{}, 0, len(in))
		for _, v := range in {
			out = append(out, v)
		}
		return out
	}

	return flatten(addedMap), flatten(deletedMap)
}

// keyedSetDifference does a set difference on keyed object and returns the
// objects that have been added, deleted, edited and unmodified when comparing
// the new to old set.
func keyedSetDifference(old, new map[string]interface{}) (
	added, deleted map[string]interface{}, edited, unmodified []string) {

	added = make(map[string]interface{})
	deleted = make(map[string]interface{})

	for k, v := range old {
		if _, ok := new[k]; !ok {
			deleted[k] = v
		}
	}
	for k, newV := range new {
		oldV, ok := old[k]
		if !ok {
			added[k] = newV
			continue
		}

		// Key is in both, check if they have been edited.
		if reflect.DeepEqual(oldV, newV) {
			unmodified = append(unmodified, k)
		} else {
			edited = append(edited, k)
		}
	}

	return added, deleted, edited, unmodified
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

// getField is a helper that returns the passed fields value of the given
// object. This method will panic if the field does not exist in the passed
// object.
func getField(obj interface{}, field string) interface{} {
	if obj == nil {
		return nil
	}

	r := reflect.ValueOf(obj)
	r = reflect.Indirect(r)
	if !r.IsValid() {
		return nil
	}

	f := r.FieldByName(field)
	return f.Interface()
}
