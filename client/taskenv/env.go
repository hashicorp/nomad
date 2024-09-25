// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskenv

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/helper"
	hargs "github.com/hashicorp/nomad/helper/args"
	"github.com/hashicorp/nomad/helper/escapingfs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/zclconf/go-cty/cty"
)

// A set of environment variables that are exported by each driver.
const (
	// AllocDir is the environment variable with the path to the alloc directory
	// that is shared across tasks within a task group.
	AllocDir = "NOMAD_ALLOC_DIR"

	// TaskLocalDir is the environment variable with the path to the tasks local
	// directory where it can store data that is persisted to the alloc is
	// removed.
	TaskLocalDir = "NOMAD_TASK_DIR"

	// SecretsDir is the environment variable with the path to the tasks secret
	// directory where it can store sensitive data.
	SecretsDir = "NOMAD_SECRETS_DIR"

	// MemLimit is the environment variable with the tasks memory limit in MBs.
	MemLimit = "NOMAD_MEMORY_LIMIT"

	// MemMaxLimit is the environment variable with the tasks maximum memory limit in MBs.
	MemMaxLimit = "NOMAD_MEMORY_MAX_LIMIT"

	// CpuLimit is the environment variable with the tasks CPU limit in MHz.
	CpuLimit = "NOMAD_CPU_LIMIT"

	// CpuCores is the environment variable for passing the task's reserved cpu cores
	CpuCores = "NOMAD_CPU_CORES"

	// AllocID is the environment variable for passing the allocation ID.
	AllocID = "NOMAD_ALLOC_ID"

	// ShortAllocID is the environment variable for passing the short version
	// of the allocation ID.
	ShortAllocID = "NOMAD_SHORT_ALLOC_ID"

	// AllocName is the environment variable for passing the allocation name.
	AllocName = "NOMAD_ALLOC_NAME"

	// TaskName is the environment variable for passing the task name.
	TaskName = "NOMAD_TASK_NAME"

	// GroupName is the environment variable for passing the task group name.
	GroupName = "NOMAD_GROUP_NAME"

	// JobID is the environment variable for passing the job ID.
	JobID = "NOMAD_JOB_ID"

	// JobName is the environment variable for passing the job name.
	JobName = "NOMAD_JOB_NAME"

	// JobParentID is the environment variable for passing the ID of the parnt of the job
	JobParentID = "NOMAD_JOB_PARENT_ID"

	// AllocIndex is the environment variable for passing the allocation index.
	AllocIndex = "NOMAD_ALLOC_INDEX"

	// Datacenter is the environment variable for passing the datacenter in which the alloc is running.
	Datacenter = "NOMAD_DC"

	// CgroupParent is the environment variable for passing the cgroup parent in which cgroups are made.
	CgroupParent = "NOMAD_PARENT_CGROUP"

	// Namespace is the environment variable for passing the namespace in which the alloc is running.
	Namespace = "NOMAD_NAMESPACE"

	// Region is the environment variable for passing the region in which the alloc is running.
	Region = "NOMAD_REGION"

	// AddrPrefix is the prefix for passing both dynamic and static port
	// allocations to tasks.
	// E.g $NOMAD_ADDR_http=127.0.0.1:80
	//
	// The ip:port are always the host's.
	AddrPrefix = "NOMAD_ADDR_"

	HostAddrPrefix = "NOMAD_HOST_ADDR_"

	// IpPrefix is the prefix for passing the host IP of a port allocation
	// to a task.
	IpPrefix = "NOMAD_IP_"

	HostIpPrefix = "NOMAD_HOST_IP_"

	// PortPrefix is the prefix for passing the port allocation to a task.
	// It will be the task's port if a port map is specified. Task's should
	// bind to this port.
	PortPrefix = "NOMAD_PORT_"

	AllocPortPrefix = "NOMAD_ALLOC_PORT_"

	// HostPortPrefix is the prefix for passing the host port when a port
	// map is specified.
	HostPortPrefix = "NOMAD_HOST_PORT_"

	// MetaPrefix is the prefix for passing task meta data.
	MetaPrefix = "NOMAD_META_"

	// UpstreamPrefix is the prefix for passing upstream IP and ports to the alloc
	UpstreamPrefix = "NOMAD_UPSTREAM_"

	// AllocPrefix is a general purpose alloc prefix. It is currently used as
	// the env var prefix used to export network namespace information
	// including IP, Port, and interface.
	AllocPrefix = "NOMAD_ALLOC_"

	// VaultToken is the environment variable for passing the Vault token
	VaultToken = "VAULT_TOKEN"

	// VaultNamespace is the environment variable for passing the Vault namespace, if applicable
	VaultNamespace = "VAULT_NAMESPACE"

	// WorkloadToken is the environment variable for passing the Nomad Workload Identity token
	WorkloadToken = "NOMAD_TOKEN"
)

// The node values that can be interpreted.
const (
	nodeIdKey     = "node.unique.id"
	nodeDcKey     = "node.datacenter"
	nodeRegionKey = "node.region"
	nodeNameKey   = "node.unique.name"
	nodeClassKey  = "node.class"
	nodePoolKey   = "node.pool"

	// Prefixes used for lookups.
	nodeAttributePrefix = "attr."
	nodeMetaPrefix      = "meta."
)

// TaskEnv is a task's environment as well as node attribute's for
// interpolation.
type TaskEnv struct {
	// NodeAttrs is the map of node attributes for interpolation
	NodeAttrs map[string]string

	// EnvMap is the map of environment variables
	EnvMap map[string]string

	// deviceEnv is the environment variables populated from the device hooks.
	deviceEnv map[string]string

	// envList is a memoized list created by List()
	envList []string

	// EnvMap is the map of environment variables with client-specific
	// task directories
	// See https://github.com/hashicorp/nomad/pull/9671
	EnvMapClient map[string]string

	// clientTaskDir is the absolute path to the task root directory on the host
	// <alloc_dir>/<task>
	clientTaskDir string

	// clientSharedAllocDir is the path to shared alloc directory on the host
	// <alloc_dir>/alloc/
	clientSharedAllocDir string
}

// NewTaskEnv creates a new task environment with the given environment, device
// environment and node attribute maps.
func NewTaskEnv(env, envClient, deviceEnv, node map[string]string, clientTaskDir, clientAllocDir string) *TaskEnv {
	return &TaskEnv{
		NodeAttrs:            node,
		deviceEnv:            deviceEnv,
		EnvMap:               env,
		EnvMapClient:         envClient,
		clientTaskDir:        clientTaskDir,
		clientSharedAllocDir: clientAllocDir,
	}
}

// NewEmptyTaskEnv creates a new empty task environment.
func NewEmptyTaskEnv() *TaskEnv {
	return &TaskEnv{
		NodeAttrs: make(map[string]string),
		deviceEnv: make(map[string]string),
		EnvMap:    make(map[string]string),
	}
}

// List returns the task's environment as a slice of NAME=value pair strings.
func (t *TaskEnv) List() []string {
	if t.envList != nil {
		return t.envList
	}

	env := []string{}
	for k, v := range t.EnvMap {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}

// DeviceEnv returns the task's environment variables set by device hooks.
func (t *TaskEnv) DeviceEnv() map[string]string {
	m := make(map[string]string, len(t.deviceEnv))
	for k, v := range t.deviceEnv {
		m[k] = v
	}

	return m
}

// Map of the task's environment variables.
func (t *TaskEnv) Map() map[string]string {
	m := make(map[string]string, len(t.EnvMap))
	for k, v := range t.EnvMap {
		m[k] = v
	}

	return m
}

// All of the task's environment variables and the node's attributes in a
// single map.
func (t *TaskEnv) All() map[string]string {
	m := make(map[string]string, len(t.EnvMap)+len(t.NodeAttrs))
	for k, v := range t.EnvMap {
		m[k] = v
	}
	for k, v := range t.NodeAttrs {
		m[k] = v
	}

	return m
}

// AllValues is a map of the task's environment variables and the node's
// attributes with cty.Value (String) values. Errors including keys are
// returned in a map by key name.
//
// In the rare case of a fatal error, only an error value is returned. This is
// likely a programming error as user input should not be able to cause a fatal
// error.
func (t *TaskEnv) AllValues() (map[string]cty.Value, map[string]error, error) {
	errs := make(map[string]error)

	// Intermediate map for building up nested go types
	allMap := make(map[string]interface{}, len(t.EnvMap)+len(t.NodeAttrs))

	// Intermediate map for all env vars including those whose keys that
	// cannot be nested (eg foo...bar)
	envMap := make(map[string]cty.Value, len(t.EnvMap))

	// Prepare job-based variables (eg job.meta, job.group.task.env, etc)
	for k, v := range t.EnvMap {
		if err := addNestedKey(allMap, k, v); err != nil {
			errs[k] = err
		}
		envMap[k] = cty.StringVal(v)
	}

	// Prepare node-based variables (eg node.*, attr.*, meta.*)
	for k, v := range t.NodeAttrs {
		if err := addNestedKey(allMap, k, v); err != nil {
			errs[k] = err
		}
	}

	// Add flat envMap as a Map to allMap so users can access any key via
	// HCL2's indexing syntax: ${env["foo...bar"]}
	allMap["env"] = cty.MapVal(envMap)

	// Add meta and attr to node if they exist to properly namespace things
	// a bit.
	nodeMapI, ok := allMap["node"]
	if !ok {
		return nil, nil, fmt.Errorf("missing node variable")
	}
	nodeMap, ok := nodeMapI.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("invalid type for node variable: %T", nodeMapI)
	}
	if attrMap, ok := allMap["attr"]; ok {
		nodeMap["attr"] = attrMap
	}
	if metaMap, ok := allMap["meta"]; ok {
		nodeMap["meta"] = metaMap
	}

	// ctyify the entire tree of strings and maps
	tree, err := ctyify(allMap)
	if err != nil {
		// This should not be possible and is likely a programming
		// error. Invalid user input should be cleaned earlier.
		return nil, nil, err
	}

	return tree, errs, nil
}

// ParseAndReplace takes the user supplied args replaces any instance of an
// environment variable or Nomad variable in the args with the actual value.
func (t *TaskEnv) ParseAndReplace(args []string) []string {
	if args == nil {
		return nil
	}

	replaced := make([]string, len(args))
	for i, arg := range args {
		replaced[i] = hargs.ReplaceEnv(arg, t.EnvMap, t.NodeAttrs)
	}

	return replaced
}

// ReplaceEnv takes an arg and replaces all occurrences of environment variables
// and Nomad variables.  If the variable is found in the passed map it is
// replaced, otherwise the original string is returned.
func (t *TaskEnv) ReplaceEnv(arg string) string {
	return hargs.ReplaceEnv(arg, t.EnvMap, t.NodeAttrs)
}

// replaceEnvClient takes an arg and replaces all occurrences of client-specific
// environment variables and Nomad variables.  If the variable is found in the
// passed map it is replaced, otherwise the original string is returned.
// The difference from ReplaceEnv client is potentially different values for
// the following variables:
// * NOMAD_ALLOC_DIR
// * NOMAD_TASK_DIR
// * NOMAD_SECRETS_DIR
// and anything that was interpolated using them.
//
// See https://github.com/hashicorp/nomad/pull/9671
func (t *TaskEnv) replaceEnvClient(arg string) string {
	return hargs.ReplaceEnv(arg, t.EnvMapClient, t.NodeAttrs)
}

// checkEscape returns true if the absolute path testPath escapes both the
// task directory and shared allocation directory specified in the
// directory path fields of this TaskEnv
func (t *TaskEnv) checkEscape(testPath string) bool {
	for _, p := range []string{t.clientTaskDir, t.clientSharedAllocDir} {
		if p != "" && !escapingfs.PathEscapesSandbox(p, testPath) {
			return false
		}
	}
	return true
}

// ClientPath interpolates the argument as a path, using the
// environment variables with client-relative directories. The
// result is an absolute path on the client filesystem.
//
// If the interpolated result is a relative path, it is made absolute
// If joinEscape, an interpolated path that escapes will be joined with the
// task dir.
// The result is checked to see whether it (still) escapes both the task working
// directory and the shared allocation directory.
func (t *TaskEnv) ClientPath(rawPath string, joinEscape bool) (string, bool) {
	path := t.replaceEnvClient(rawPath)
	if !filepath.IsAbs(path) || (t.checkEscape(path) && joinEscape) {
		path = filepath.Join(t.clientTaskDir, path)
	}
	path = filepath.Clean(path)
	escapes := t.checkEscape(path)
	return path, escapes
}

// Builder is used to build task environment's and is safe for concurrent use.
type Builder struct {
	// envvars are custom set environment variables
	envvars map[string]string

	// templateEnv are env vars set from templates
	templateEnv map[string]string

	// hostEnv are environment variables filtered from the host
	hostEnv map[string]string

	// nodeAttrs are Node attributes and metadata
	nodeAttrs map[string]string

	// taskMeta are the meta attributes on the task
	taskMeta map[string]string

	// allocDir from task's perspective; eg /alloc
	allocDir string

	// localDir from task's perspective; eg /local
	localDir string

	// secretsDir from task's perspective; eg /secrets
	secretsDir string

	// clientSharedAllocDir is the shared alloc dir from the client's perspective; eg, <alloc_dir>/<alloc_id>/alloc
	clientSharedAllocDir string

	// clientTaskRoot is the task working directory from the client's perspective; eg <alloc_dir>/<alloc_id>/<task>
	clientTaskRoot string

	// clientTaskLocalDir is the local dir from the client's perspective; eg <client_task_root>/local
	clientTaskLocalDir string

	// clientTaskSecretsDir is the secrets dir from the client's perspective; eg <client_task_root>/secrets
	clientTaskSecretsDir string

	cpuCores             string
	cpuLimit             int64
	memLimit             int64
	memMaxLimit          int64
	taskName             string
	allocIndex           int
	datacenter           string
	cgroupParent         string
	namespace            string
	region               string
	allocId              string
	allocName            string
	groupName            string
	vaultToken           string
	vaultNamespace       string
	injectVaultToken     bool
	workloadTokenDefault string
	workloadTokens       map[string]string // identity name -> encoded JWT
	jobID                string
	jobName              string
	jobParentID          string

	// otherPorts for tasks in the same alloc
	otherPorts map[string]string

	// driverNetwork is the network defined by the driver (or nil if none
	// was defined).
	driverNetwork *drivers.DriverNetwork

	// network resources from the task; must be lazily turned into env vars
	// because portMaps and advertiseIP can change after builder creation
	// and affect network env vars.
	networks []*structs.NetworkResource

	networkStatus  *structs.AllocNetworkStatus
	allocatedPorts structs.AllocatedPorts

	// hookEnvs are env vars set by hooks and stored by hook name to
	// support adding/removing vars from multiple hooks (eg HookA adds A:1,
	// HookB adds A:2, HookA removes A, A should equal 2)
	hookEnvs map[string]map[string]string

	// hookNames is a slice of hooks in hookEnvs to apply hookEnvs in the
	// order the hooks are run.
	hookNames []string

	// deviceHookName is the device hook name. It is set only if device hooks
	// are set. While a bit round about, this enables us to return device hook
	// environment variables without having to hardcode the name of the hook.
	deviceHookName string

	// upstreams from the group connect enabled services
	upstreams []structs.ConsulUpstream

	mu *sync.RWMutex
}

// NewBuilder creates a new task environment builder.
func NewBuilder(node *structs.Node, alloc *structs.Allocation, task *structs.Task, region string) *Builder {
	b := NewEmptyBuilder()
	b.region = region
	return b.setTask(task).setAlloc(alloc).setNode(node)
}

// NewEmptyBuilder creates a new environment builder.
func NewEmptyBuilder() *Builder {
	return &Builder{
		mu:       &sync.RWMutex{},
		hookEnvs: map[string]map[string]string{},
		envvars:  make(map[string]string),
	}
}

// buildEnv returns the environment variables and device environment
// variables with respect to the task directories passed in the arguments.
func (b *Builder) buildEnv(allocDir, localDir, secretsDir string,
	nodeAttrs map[string]string) (map[string]string, map[string]string) {

	envMap := make(map[string]string)
	var deviceEnvs map[string]string

	// Add the directories
	if allocDir != "" {
		envMap[AllocDir] = allocDir
	}
	if localDir != "" {
		envMap[TaskLocalDir] = localDir
	}
	if secretsDir != "" {
		envMap[SecretsDir] = secretsDir
	}

	// Add the resource limits
	if b.memLimit != 0 {
		envMap[MemLimit] = strconv.FormatInt(b.memLimit, 10)
	}
	if b.memMaxLimit != 0 {
		envMap[MemMaxLimit] = strconv.FormatInt(b.memMaxLimit, 10)
	}
	if b.cpuLimit != 0 {
		envMap[CpuLimit] = strconv.FormatInt(b.cpuLimit, 10)
	}
	if b.cpuCores != "" {
		envMap[CpuCores] = b.cpuCores
	}

	// Add the task metadata
	if b.allocId != "" {
		envMap[AllocID] = b.allocId
		envMap[ShortAllocID] = b.allocId[:8]
	}
	if b.allocName != "" {
		envMap[AllocName] = b.allocName
	}
	if b.groupName != "" {
		envMap[GroupName] = b.groupName
	}
	if b.allocIndex != -1 {
		envMap[AllocIndex] = strconv.Itoa(b.allocIndex)
	}
	if b.taskName != "" {
		envMap[TaskName] = b.taskName
	}
	if b.jobID != "" {
		envMap[JobID] = b.jobID
	}
	if b.jobName != "" {
		envMap[JobName] = b.jobName
	}
	if b.jobParentID != "" {
		envMap[JobParentID] = b.jobParentID
	}
	if b.datacenter != "" {
		envMap[Datacenter] = b.datacenter
	}
	if b.cgroupParent != "" {
		envMap[CgroupParent] = b.cgroupParent
	}
	if b.namespace != "" {
		envMap[Namespace] = b.namespace
	}
	if b.region != "" {
		envMap[Region] = b.region
	}

	// Build the network related env vars
	buildNetworkEnv(envMap, b.networks, b.driverNetwork)

	// Build the addr of the other tasks
	for k, v := range b.otherPorts {
		envMap[k] = v
	}

	// Build the Consul Connect upstream env vars
	buildUpstreamsEnv(envMap, b.upstreams)

	// Build the network namespace information if we have the required detail
	// available.
	if b.networkStatus != nil && b.allocatedPorts != nil {
		addNomadAllocNetwork(envMap, b.allocatedPorts, b.networkStatus)
	}

	// Build the Vault Token
	if b.injectVaultToken && b.vaultToken != "" {
		envMap[VaultToken] = b.vaultToken
	}

	// Build the Vault Namespace
	if b.injectVaultToken && b.vaultNamespace != "" {
		envMap[VaultNamespace] = b.vaultNamespace
	}

	// Build the Nomad Workload Token
	if b.workloadTokenDefault != "" {
		envMap[WorkloadToken] = b.workloadTokenDefault
	}

	for name, token := range b.workloadTokens {
		envMap[WorkloadToken+"_"+name] = token
	}

	// Copy and interpolate task meta
	for k, v := range b.taskMeta {
		envMap[hargs.ReplaceEnv(k, nodeAttrs, envMap)] = hargs.ReplaceEnv(v, nodeAttrs, envMap)
	}

	// Interpolate and add environment variables from the host. Only do this if
	// the variable is not present in the map; we do not want to override task
	// variables in favour of the same variable found within the host OS env
	// vars.
	for k, v := range b.hostEnv {
		if _, ok := envMap[k]; !ok {
			envMap[k] = hargs.ReplaceEnv(v, nodeAttrs, envMap)
		}
	}

	// Copy interpolated task env vars second as they override host env vars
	for k, v := range b.envvars {
		envMap[k] = hargs.ReplaceEnv(v, nodeAttrs, envMap)
	}

	// Copy hook env vars in the order the hooks were run
	for _, h := range b.hookNames {
		for k, v := range b.hookEnvs[h] {
			e := hargs.ReplaceEnv(v, nodeAttrs, envMap)
			envMap[k] = e

			if h == b.deviceHookName {
				if deviceEnvs == nil {
					deviceEnvs = make(map[string]string, len(b.hookEnvs[h]))
				}

				deviceEnvs[k] = e
			}
		}
	}

	// Copy template env vars as they override task env vars
	for k, v := range b.templateEnv {
		envMap[k] = v
	}

	// Clean keys (see #2405)
	prefixesToClean := [...]string{AddrPrefix, IpPrefix, PortPrefix, HostPortPrefix, MetaPrefix}
	cleanedEnv := make(map[string]string, len(envMap))
	for k, v := range envMap {
		cleanedK := k
		for i := range prefixesToClean {
			if strings.HasPrefix(k, prefixesToClean[i]) {
				cleanedK = helper.CleanEnvVar(k, '_')
			}
		}
		cleanedEnv[cleanedK] = v
	}

	return cleanedEnv, deviceEnvs
}

// Build must be called after all the tasks environment values have been set.
func (b *Builder) Build() *TaskEnv {
	nodeAttrs := make(map[string]string)

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.region != "" {
		// Copy region over to node attrs
		nodeAttrs[nodeRegionKey] = b.region
	}
	// Copy node attributes
	for k, v := range b.nodeAttrs {
		nodeAttrs[k] = v
	}

	envMap, deviceEnvs := b.buildEnv(b.allocDir, b.localDir, b.secretsDir, nodeAttrs)
	envMapClient, _ := b.buildEnv(b.clientSharedAllocDir, b.clientTaskLocalDir, b.clientTaskSecretsDir, nodeAttrs)

	return NewTaskEnv(envMap, envMapClient, deviceEnvs, nodeAttrs, b.clientTaskRoot, b.clientSharedAllocDir)
}

// UpdateTask updates the environment based on a new alloc and task.
func (b *Builder) UpdateTask(alloc *structs.Allocation, task *structs.Task) *Builder {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.setTask(task).setAlloc(alloc)
}

// SetHookEnv sets environment variables from a hook. Variables are
// Last-Write-Wins, so if a hook writes a variable that's also written by a
// later hook, the later hooks value always gets used.
func (b *Builder) SetHookEnv(hook string, envs map[string]string) *Builder {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.setHookEnvLocked(hook, envs)
}

// setHookEnvLocked is the implementation of setting hook environment variables
// and should be called with the lock held
func (b *Builder) setHookEnvLocked(hook string, envs map[string]string) *Builder {
	if _, exists := b.hookEnvs[hook]; !exists {
		b.hookNames = append(b.hookNames, hook)
	}
	b.hookEnvs[hook] = envs

	return b
}

// SetDeviceHookEnv sets environment variables from a device hook. Variables are
// Last-Write-Wins, so if a hook writes a variable that's also written by a
// later hook, the later hooks value always gets used.
func (b *Builder) SetDeviceHookEnv(hookName string, envs map[string]string) *Builder {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Store the device hook name
	b.deviceHookName = hookName
	return b.setHookEnvLocked(hookName, envs)
}

// setTask is called from NewBuilder to populate task related environment
// variables.
func (b *Builder) setTask(task *structs.Task) *Builder {
	if task == nil {
		return b
	}
	b.taskName = task.Name
	b.envvars = make(map[string]string, len(task.Env))
	for k, v := range task.Env {
		b.envvars[k] = v
	}

	// COMPAT(0.11): Remove in 0.11
	if task.Resources == nil {
		b.memLimit = 0
		b.memMaxLimit = 0
		b.cpuLimit = 0
	} else {
		b.memLimit = int64(task.Resources.MemoryMB)
		b.memMaxLimit = int64(task.Resources.MemoryMaxMB)
		b.cpuLimit = int64(task.Resources.CPU)
	}
	return b
}

// setAlloc is called from NewBuilder to populate alloc related environment
// variables.
func (b *Builder) setAlloc(alloc *structs.Allocation) *Builder {
	b.allocId = alloc.ID
	b.allocName = alloc.Name
	b.groupName = alloc.TaskGroup
	b.allocIndex = int(alloc.Index())
	b.jobID = alloc.Job.ID
	b.jobName = alloc.Job.Name
	b.jobParentID = alloc.Job.ParentID
	b.namespace = alloc.Namespace

	// Set meta
	combined := alloc.Job.CombinedTaskMeta(alloc.TaskGroup, b.taskName)
	// taskMetaSize is double to total meta keys to account for given and upper
	// cased values
	taskMetaSize := len(combined) * 2

	// if job is parameterized initialize optional meta to empty strings
	if alloc.Job.Dispatched {
		optionalMetaCount := len(alloc.Job.ParameterizedJob.MetaOptional)
		b.taskMeta = make(map[string]string, taskMetaSize+optionalMetaCount*2)

		for _, k := range alloc.Job.ParameterizedJob.MetaOptional {
			b.taskMeta[fmt.Sprintf("%s%s", MetaPrefix, strings.ToUpper(k))] = ""
			b.taskMeta[fmt.Sprintf("%s%s", MetaPrefix, k)] = ""
		}
	} else {
		b.taskMeta = make(map[string]string, taskMetaSize)
	}

	for k, v := range combined {
		b.taskMeta[fmt.Sprintf("%s%s", MetaPrefix, strings.ToUpper(k))] = v
		b.taskMeta[fmt.Sprintf("%s%s", MetaPrefix, k)] = v
	}

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)

	b.otherPorts = make(map[string]string, len(tg.Tasks)*2)

	// Protect against invalid allocs where AllocatedResources isn't set.
	// TestClient_AddAllocError explicitly tests for this condition
	if alloc.AllocatedResources != nil {
		// Populate task resources
		if tr, ok := alloc.AllocatedResources.Tasks[b.taskName]; ok {
			b.cpuLimit = tr.Cpu.CpuShares
			b.cpuCores = idset.From[uint16](tr.Cpu.ReservedCores).String()
			b.memLimit = tr.Memory.MemoryMB
			b.memMaxLimit = tr.Memory.MemoryMaxMB

			// Copy networks to prevent sharing
			b.networks = make([]*structs.NetworkResource, len(tr.Networks))
			for i, n := range tr.Networks {
				b.networks[i] = n.Copy()
			}
		}

		// COMPAT(1.0): remove in 1.0 when AllocatedPorts can be used exclusively
		// Add ports from other tasks
		for taskName, resources := range alloc.AllocatedResources.Tasks {
			// Add ports from other tasks
			if taskName == b.taskName {
				continue
			}

			for _, nw := range resources.Networks {
				for _, p := range nw.ReservedPorts {
					addPort(b.otherPorts, taskName, nw.IP, p.Label, p.Value)
				}
				for _, p := range nw.DynamicPorts {
					addPort(b.otherPorts, taskName, nw.IP, p.Label, p.Value)
				}
			}
		}

		// COMPAT(1.0): remove in 1.0 when AllocatedPorts can be used exclusively
		// Add ports from group networks
		//TODO Expose IPs but possibly only via variable interpolation
		for _, nw := range alloc.AllocatedResources.Shared.Networks {
			for _, p := range nw.ReservedPorts {
				addGroupPort(b.otherPorts, p)
			}
			for _, p := range nw.DynamicPorts {
				addGroupPort(b.otherPorts, p)
			}
		}

		// Add any allocated host ports
		if alloc.AllocatedResources.Shared.Ports != nil {
			b.allocatedPorts = alloc.AllocatedResources.Shared.Ports
			addPorts(b.otherPorts, alloc.AllocatedResources.Shared.Ports)
		}
	}

	var upstreams []structs.ConsulUpstream
	for _, svc := range tg.Services {
		if svc.Connect.HasSidecar() && svc.Connect.SidecarService.HasUpstreams() {
			upstreams = append(upstreams, svc.Connect.SidecarService.Proxy.Upstreams...)
		}
	}
	if len(upstreams) > 0 {
		b.setUpstreamsLocked(upstreams)
	}

	return b
}

// setNode is called from NewBuilder to populate node attributes.
func (b *Builder) setNode(n *structs.Node) *Builder {
	b.nodeAttrs = make(map[string]string, 4+len(n.Attributes)+len(n.Meta))
	b.nodeAttrs[nodeIdKey] = n.ID
	b.nodeAttrs[nodeNameKey] = n.Name
	b.nodeAttrs[nodeClassKey] = n.NodeClass
	b.nodeAttrs[nodeDcKey] = n.Datacenter
	b.nodeAttrs[nodePoolKey] = n.NodePool
	b.datacenter = n.Datacenter
	b.cgroupParent = n.CgroupParent

	// Set up the attributes.
	for k, v := range n.Attributes {
		b.nodeAttrs[fmt.Sprintf("%s%s", nodeAttributePrefix, k)] = v
	}

	// Set up the meta.
	for k, v := range n.Meta {
		b.nodeAttrs[fmt.Sprintf("%s%s", nodeMetaPrefix, k)] = v
	}
	return b
}

func (b *Builder) SetAllocDir(dir string) *Builder {
	b.mu.Lock()
	b.allocDir = dir
	b.mu.Unlock()
	return b
}

func (b *Builder) SetTaskLocalDir(dir string) *Builder {
	b.mu.Lock()
	b.localDir = dir
	b.mu.Unlock()
	return b
}

func (b *Builder) SetClientSharedAllocDir(dir string) *Builder {
	b.mu.Lock()
	b.clientSharedAllocDir = dir
	b.mu.Unlock()
	return b
}

func (b *Builder) SetClientTaskRoot(dir string) *Builder {
	b.mu.Lock()
	b.clientTaskRoot = dir
	b.mu.Unlock()
	return b
}

func (b *Builder) SetClientTaskLocalDir(dir string) *Builder {
	b.mu.Lock()
	b.clientTaskLocalDir = dir
	b.mu.Unlock()
	return b
}

func (b *Builder) SetClientTaskSecretsDir(dir string) *Builder {
	b.mu.Lock()
	b.clientTaskSecretsDir = dir
	b.mu.Unlock()
	return b
}

func (b *Builder) SetSecretsDir(dir string) *Builder {
	b.mu.Lock()
	b.secretsDir = dir
	b.mu.Unlock()
	return b
}

// SetDriverNetwork defined by the driver.
func (b *Builder) SetDriverNetwork(n *drivers.DriverNetwork) *Builder {
	ncopy := n.Copy()
	b.mu.Lock()
	b.driverNetwork = ncopy
	b.mu.Unlock()
	return b
}

// buildNetworkEnv env vars in the given map.
//
//	Auto:   NOMAD_PORT_<label>
//	Host:   NOMAD_IP_<label>, NOMAD_ADDR_<label>, NOMAD_HOST_PORT_<label>
//
// Handled by setAlloc -> otherPorts:
//
//	Task:   NOMAD_TASK_{IP,PORT,ADDR}_<task>_<label> # Always host values
func buildNetworkEnv(envMap map[string]string, nets structs.Networks, driverNet *drivers.DriverNetwork) {
	for _, n := range nets {
		for _, p := range n.ReservedPorts {
			buildPortEnv(envMap, p, n.IP, driverNet)
		}
		for _, p := range n.DynamicPorts {
			buildPortEnv(envMap, p, n.IP, driverNet)
		}
	}
}

func buildPortEnv(envMap map[string]string, p structs.Port, ip string, driverNet *drivers.DriverNetwork) {
	// Host IP, port, and address
	portStr := strconv.Itoa(p.Value)

	var ipFamilyPrefix string
	if strings.Contains(ip, ":") {
		ipFamilyPrefix = "NOMAD_IPv6_"
	} else {
		ipFamilyPrefix = "NOMAD_IPv4_"
	}

	envMap[IpPrefix+p.Label] = ip
	envMap[ipFamilyPrefix+p.Label] = ip
	envMap[HostPortPrefix+p.Label] = portStr
	envMap[AddrPrefix+p.Label] = net.JoinHostPort(ip, portStr)

	// Set Port to task's value if there's a port map
	if driverNet != nil && driverNet.PortMap[p.Label] != 0 {
		envMap[PortPrefix+p.Label] = strconv.Itoa(driverNet.PortMap[p.Label])
	} else {
		// Default to host's
		envMap[PortPrefix+p.Label] = portStr
	}
}

// SetUpstreams defined by connect enabled group services
func (b *Builder) SetUpstreams(upstreams []structs.ConsulUpstream) *Builder {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.setUpstreamsLocked(upstreams)
}

func (b *Builder) setUpstreamsLocked(upstreams []structs.ConsulUpstream) *Builder {
	b.upstreams = upstreams
	return b
}

func (b *Builder) SetNetworkStatus(netStatus *structs.AllocNetworkStatus) *Builder {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.networkStatus = netStatus
	return b
}

// buildUpstreamsEnv builds NOMAD_UPSTREAM_{IP,PORT,ADDR}_{destination} vars
func buildUpstreamsEnv(envMap map[string]string, upstreams []structs.ConsulUpstream) {
	// Proxy sidecars always bind to localhost
	const ip = "127.0.0.1"
	for _, u := range upstreams {
		port := strconv.Itoa(u.LocalBindPort)
		envMap[UpstreamPrefix+"IP_"+u.DestinationName] = ip
		envMap[UpstreamPrefix+"PORT_"+u.DestinationName] = port
		envMap[UpstreamPrefix+"ADDR_"+u.DestinationName] = net.JoinHostPort(ip, port)

		// Also add cleaned version
		cleanName := helper.CleanEnvVar(u.DestinationName, '_')
		envMap[UpstreamPrefix+"ADDR_"+cleanName] = net.JoinHostPort(ip, port)
		envMap[UpstreamPrefix+"IP_"+cleanName] = ip
		envMap[UpstreamPrefix+"PORT_"+cleanName] = port
	}
}

// addNomadAllocNetwork builds NOMAD_ALLOC_{IP,INTERFACE,ADDR}_{port_label}
// vars. NOMAD_ALLOC_PORT_* is handled within addPorts and therefore omitted
// from this function.
func addNomadAllocNetwork(envMap map[string]string, p structs.AllocatedPorts, netStatus *structs.AllocNetworkStatus) {
	for _, allocatedPort := range p {
		portStr := strconv.Itoa(allocatedPort.To)
		envMap[AllocPrefix+"INTERFACE_"+allocatedPort.Label] = netStatus.InterfaceName
		envMap[AllocPrefix+"IP_"+allocatedPort.Label] = netStatus.Address
		envMap[AllocPrefix+"ADDR_"+allocatedPort.Label] = net.JoinHostPort(netStatus.Address, portStr)
	}
}

// SetPortMapEnvs sets the PortMap related environment variables on the map
func SetPortMapEnvs(envs map[string]string, ports map[string]int) map[string]string {
	if envs == nil {
		envs = map[string]string{}
	}

	for portLabel, port := range ports {
		portEnv := helper.CleanEnvVar(PortPrefix+portLabel, '_')
		envs[portEnv] = strconv.Itoa(port)
	}
	return envs
}

// SetHostEnvvars adds the host environment variables to the tasks. The filter
// parameter can be use to filter host environment from entering the tasks.
func (b *Builder) SetHostEnvvars(filter []string) *Builder {
	filterMap := make(map[string]struct{}, len(filter))
	for _, f := range filter {
		filterMap[f] = struct{}{}
	}

	fullHostEnv := os.Environ()
	filteredHostEnv := make(map[string]string, len(fullHostEnv))
	for _, e := range fullHostEnv {
		parts := strings.SplitN(e, "=", 2)
		key, value := parts[0], parts[1]

		// Skip filtered environment variables
		if _, filtered := filterMap[key]; filtered {
			continue
		}

		filteredHostEnv[key] = value
	}

	b.mu.Lock()
	b.hostEnv = filteredHostEnv
	b.mu.Unlock()
	return b
}

func (b *Builder) SetTemplateEnv(m map[string]string) *Builder {
	b.mu.Lock()
	b.templateEnv = m
	b.mu.Unlock()
	return b
}

func (b *Builder) SetVaultToken(token, namespace string, inject bool) *Builder {
	b.mu.Lock()
	b.vaultToken = token
	b.vaultNamespace = namespace
	b.injectVaultToken = inject
	b.mu.Unlock()
	return b
}

func (b *Builder) SetDefaultWorkloadToken(token string) *Builder {
	b.mu.Lock()
	b.workloadTokenDefault = token
	b.mu.Unlock()
	return b
}

func (b *Builder) SetWorkloadToken(name, token string) *Builder {
	b.mu.Lock()
	if b.workloadTokens == nil {
		b.workloadTokens = map[string]string{}
	}
	b.workloadTokens[name] = token
	b.mu.Unlock()
	return b
}

// addPort keys and values for other tasks to an env var map
func addPort(m map[string]string, taskName, ip, portLabel string, port int) {
	key := fmt.Sprintf("%s%s_%s", AddrPrefix, taskName, portLabel)
	m[key] = net.JoinHostPort(ip, strconv.Itoa(port))
	key = fmt.Sprintf("%s%s_%s", IpPrefix, taskName, portLabel)
	m[key] = ip
	key = fmt.Sprintf("%s%s_%s", PortPrefix, taskName, portLabel)
	m[key] = strconv.Itoa(port)
}

// addGroupPort adds a group network port. The To value is used if one is
// specified.
func addGroupPort(m map[string]string, port structs.Port) {
	if port.To > 0 {
		m[PortPrefix+port.Label] = strconv.Itoa(port.To)
	} else {
		m[PortPrefix+port.Label] = strconv.Itoa(port.Value)
	}

	m[HostPortPrefix+port.Label] = strconv.Itoa(port.Value)
}

func addPorts(m map[string]string, ports structs.AllocatedPorts) {
	for _, p := range ports {
		port := strconv.Itoa(p.Value)
		m[AddrPrefix+p.Label] = net.JoinHostPort(p.HostIP, port)
		m[HostAddrPrefix+p.Label] = net.JoinHostPort(p.HostIP, port)
		m[IpPrefix+p.Label] = p.HostIP
		m[HostIpPrefix+p.Label] = p.HostIP
		if p.To > 0 {
			val := strconv.Itoa(p.To)
			m[PortPrefix+p.Label] = val
			m[AllocPortPrefix+p.Label] = val
		} else {
			val := strconv.Itoa(p.Value)
			m[PortPrefix+p.Label] = val
			m[AllocPortPrefix+p.Label] = val
		}

		m[HostPortPrefix+p.Label] = port
	}
}
