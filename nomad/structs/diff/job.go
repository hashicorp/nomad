package diff

import (
	"fmt"
	"reflect"

	"github.com/hashicorp/nomad/nomad/structs"
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

func (d *DiffEntry) SetDiffType(old, new interface{}) {
	if reflect.DeepEqual(old, new) {
		d.Type = DiffTypeNone
	} else if old == nil {
		d.Type = DiffTypeAdded
	} else if new == nil {
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
	Added, Deleted []*structs.TaskGroup
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
	Added, Deleted []*structs.Task
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
}

// ServicesDiff contains the set of Services that were changed.
type ServicesDiff struct {
	DiffEntry
	Added, Deleted []*structs.Service
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
	Added, Deleted []*structs.ServiceCheck
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
	Added, Deleted []*structs.TaskArtifact
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
	Added, Deleted []structs.Port
	Edited         []*PrimitiveStructDiff
}

// PrimitiveStructDiff contains the diff of two structs that only contain
// primitive fields.
type PrimitiveStructDiff struct {
	DiffEntry
	PrimitiveFields []*FieldDiff
}

func (p *PrimitiveStructDiff) DiffFields(old, new interface{}, fields []string) {
	for _, field := range fields {
		oldV := getField(old, field)
		newV := getField(new, field)
		pDiff := NewFieldDiff(field, oldV, newV)
		if pDiff != nil {
			p.Type = DiffTypeEdited
			p.PrimitiveFields = append(p.PrimitiveFields, pDiff)
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
func NewJobDiff(old, new *structs.Job) *JobDiff {
	if old == nil && new == nil || reflect.DeepEqual(old, new) {
		return nil
	}

	// Get the diffs of the primitive fields
	diff := &JobDiff{}
	diff.DiffFields(old, new, jobPrimitiveFields)

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

	diff.SetDiffType(old, new)
	return diff
}

// NewTaskGroupDiff returns the diff between two task groups. If there is no
// difference, nil is returned.
func NewTaskGroupDiff(old, new *structs.TaskGroup) *TaskGroupDiff {
	if old == nil && new == nil || reflect.DeepEqual(old, new) {
		return nil
	}

	// Get the diffs of the primitive fields
	diff := &TaskGroupDiff{}
	diff.DiffFields(old, new, taskGroupPrimitiveFields)

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

	diff.SetDiffType(old, new)
	return diff
}

// NewTaskDiff returns the diff between two tasks. If there is no difference,
// nil is returned.
func NewTaskDiff(old, new *structs.Task) *TaskDiff {
	if old == nil && new == nil || reflect.DeepEqual(old, new) {
		return nil
	}

	// Get the diffs of the primitive fields
	diff := &TaskDiff{}
	diff.DiffFields(old, new, taskPrimitiveFields)

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
	// TODO: Flat map the config and diff it.

	// If there are no changes return nil
	if len(diff.PrimitiveFields)+len(diff.Constraints) == 0 &&
		diff.Artifacts == nil &&
		diff.LogConfig == nil &&
		diff.Services == nil &&
		diff.Env == nil &&
		diff.Meta == nil {
		return nil
	}

	diff.SetDiffType(old, new)
	return diff
}

// NewServiceDiff returns the diff between two services. If there is no
// difference, nil is returned.
func NewServiceDiff(old, new *structs.Service) *ServiceDiff {
	if old == nil && new == nil || reflect.DeepEqual(old, new) {
		return nil
	}

	// Get the diffs of the primitive fields
	diff := &ServiceDiff{}
	diff.DiffFields(old, new, servicePrimitiveFields)

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

	diff.SetDiffType(old, new)
	return diff
}

// NewServiceCheckDiff returns the diff between two service checks. If there is
// no difference, nil is returned.
func NewServiceCheckDiff(old, new *structs.ServiceCheck) *ServiceCheckDiff {
	if old == nil && new == nil || reflect.DeepEqual(old, new) {
		return nil
	}

	// Get the diffs of the primitive fields
	diff := &ServiceCheckDiff{}
	diff.DiffFields(old, new, serviceCheckPrimitiveFields)

	// Get the args diff
	diff.Args = NewStringSetDiff(old.Args, new.Args)

	// If there are no changes return nil
	if len(diff.PrimitiveFields) == 0 &&
		diff.Args == nil {
		return nil
	}

	diff.SetDiffType(old, new)
	return diff
}

// NewTaskArtifactDiff returns the diff between two task artifacts. If there is
// no difference, nil is returned.
func NewTaskArtifactDiff(old, new *structs.TaskArtifact) *TaskArtifactDiff {
	if old == nil && new == nil || reflect.DeepEqual(old, new) {
		return nil
	}

	// Get the diffs of the primitive fields
	diff := &TaskArtifactDiff{}
	diff.DiffFields(old, new, taskArtifactPrimitiveFields)

	// Get the args diff
	diff.GetterOptions = NewStringMapDiff(old.GetterOptions, new.GetterOptions)

	// If there are no changes return nil
	if len(diff.PrimitiveFields) == 0 &&
		diff.GetterOptions == nil {
		return nil
	}

	diff.SetDiffType(old, new)
	return diff
}

// NewResourcesDiff returns the diff between two resources. If there is no
// difference, nil is returned.
func NewResourcesDiff(old, new *structs.Resources) *ResourcesDiff {
	if old == nil && new == nil || reflect.DeepEqual(old, new) {
		return nil
	}

	// Get the diffs of the primitive fields
	diff := &ResourcesDiff{}
	diff.DiffFields(old, new, resourcesPrimitiveFields)

	// Get the network resource diff
	diff.Networks = setDiffNetworkResources(old.Networks, new.Networks)

	// If there are no changes return nil
	if len(diff.PrimitiveFields) == 0 &&
		diff.Networks == nil {
		return nil
	}

	diff.SetDiffType(old, new)
	return diff
}

// NewNetworkResourceDiff returns the diff between two network resources. If
// there is no difference, nil is returned.
func NewNetworkResourceDiff(old, new *structs.NetworkResource) *NetworkResourceDiff {
	if old == nil && new == nil || reflect.DeepEqual(old, new) {
		return nil
	}

	// Get the diffs of the primitive fields
	diff := &NetworkResourceDiff{}
	diff.DiffFields(old, new, networkResourcePrimitiveFields)

	// Get the port diffs
	diff.ReservedPorts = setDiffPorts(old.ReservedPorts, new.ReservedPorts)
	diff.DynamicPorts = setDiffPorts(old.DynamicPorts, new.DynamicPorts)

	// If there are no changes return nil
	if len(diff.PrimitiveFields) == 0 &&
		diff.DynamicPorts == nil &&
		diff.ReservedPorts == nil {
		return nil
	}

	diff.SetDiffType(old, new)
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

	diff.SetDiffType(old, new)
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
	diff := &StringSetDiff{}
	lOld, lNew := len(old), len(new)
	if reflect.DeepEqual(old, new) {
		return nil
	} else if lOld == 0 && lNew > 0 {
		diff.Type = DiffTypeAdded
		return diff
	} else if lNew == 0 && lOld > 0 {
		diff.Type = DiffTypeDeleted
		return diff
	}

	diff.Type = DiffTypeEdited
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
	return diff
}

// NewStringMapDiff returns the diff between two maps of strings. If there is no
// difference, nil is returned.
func NewStringMapDiff(old, new map[string]string) *StringMapDiff {
	diff := &StringMapDiff{}
	lOld, lNew := len(old), len(new)
	if reflect.DeepEqual(old, new) {
		return nil
	} else if lOld == 0 && lNew > 0 {
		diff.Type = DiffTypeAdded
		return diff
	} else if lNew == 0 && lOld > 0 {
		diff.Type = DiffTypeDeleted
		return diff
	}

	diff.Type = DiffTypeEdited
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

	if len(diff.Added)+len(diff.Deleted)+len(diff.Edited) == 0 {
		return nil
	}
	return diff
}

// Set helpers

// setDiffTaskGroups does a set difference of task groups using the task group
// name as a key.
func setDiffTaskGroups(old, new []*structs.TaskGroup) *TaskGroupsDiff {
	diff := &TaskGroupsDiff{}

	oldMap := make(map[string]*structs.TaskGroup)
	newMap := make(map[string]*structs.TaskGroup)
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
func setDiffTasks(old, new []*structs.Task) *TasksDiff {
	diff := &TasksDiff{}

	oldMap := make(map[string]*structs.Task)
	newMap := make(map[string]*structs.Task)
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
func setDiffServices(old, new []*structs.Service) *ServicesDiff {
	diff := &ServicesDiff{}

	oldMap := make(map[string]*structs.Service)
	newMap := make(map[string]*structs.Service)
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
func setDiffServiceChecks(old, new []*structs.ServiceCheck) *ServiceChecksDiff {
	diff := &ServiceChecksDiff{}

	oldMap := make(map[string]*structs.ServiceCheck)
	newMap := make(map[string]*structs.ServiceCheck)
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
func setDiffTaskArtifacts(old, new []*structs.TaskArtifact) *TaskArtifactsDiff {
	diff := &TaskArtifactsDiff{}

	oldMap := make(map[string]*structs.TaskArtifact)
	newMap := make(map[string]*structs.TaskArtifact)
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
func setDiffNetworkResources(old, new []*structs.NetworkResource) *NetworkResourcesDiff {
	diff := &NetworkResourcesDiff{}

	added, del := setDifference(interfaceSlice(old), interfaceSlice(new))
	for _, a := range added {
		nDiff := NewNetworkResourceDiff(nil, a.(*structs.NetworkResource))
		diff.Added = append(diff.Added, nDiff)
	}
	for _, d := range del {
		nDiff := NewNetworkResourceDiff(d.(*structs.NetworkResource), nil)
		diff.Added = append(diff.Deleted, nDiff)
	}

	return diff
}

// setDiffPorts does a set difference of ports using the label as a key.
func setDiffPorts(old, new []structs.Port) *PortsDiff {
	diff := &PortsDiff{}

	oldMap := make(map[string]structs.Port)
	newMap := make(map[string]structs.Port)
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
