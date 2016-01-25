package env

import (
	"fmt"
	"strconv"
	"strings"

	hargs "github.com/hashicorp/nomad/helper/args"
	"github.com/hashicorp/nomad/nomad/structs"
)

// A set of environment variables that are exported by each driver.
const (
	// The path to the alloc directory that is shared across tasks within a task
	// group.
	AllocDir = "NOMAD_ALLOC_DIR"

	// The path to the tasks local directory where it can store data that is
	// persisted to the alloc is removed.
	TaskLocalDir = "NOMAD_TASK_DIR"

	// The tasks memory limit in MBs.
	MemLimit = "NOMAD_MEMORY_LIMIT"

	// The tasks limit in MHz.
	CpuLimit = "NOMAD_CPU_LIMIT"

	// The IP address for the task.
	TaskIP = "NOMAD_IP"

	// Prefix for passing both dynamic and static port allocations to
	// tasks.
	// E.g. $NOMAD_IP_1=127.0.0.1:1 or $NOMAD_IP_http=127.0.0.1:80
	AddrPrefix = "NOMAD_ADDR_"

	// Prefix for passing the host port when a portmap is specified.
	HostPortPrefix = "NOMAD_HOST_PORT_"

	// Prefix for passing task meta data.
	MetaPrefix = "NOMAD_META_"
)

// The node values that can be interpreted.
const (
	nodeIdKey    = "node.id"
	nodeDcKey    = "node.datacenter"
	nodeNameKey  = "node.name"
	nodeClassKey = "node.class"

	// Prefixes used for lookups.
	nodeAttributePrefix = "attr."
	nodeMetaPrefix      = "meta."
)

// TaskEnvironment is used to expose information to a task via environment
// variables and provide interpolation of Nomad variables.
type TaskEnvironment struct {
	env      map[string]string
	meta     map[string]string
	allocDir string
	taskDir  string
	cpuLimit int
	memLimit int
	node     *structs.Node
	networks []*structs.NetworkResource
	portMap  map[string]int

	// taskEnv is the variables that will be set in the tasks environment
	taskEnv map[string]string

	// nodeValues is the values that are allowed for interprolation from the
	// node.
	nodeValues map[string]string
}

func NewTaskEnvironment(node *structs.Node) *TaskEnvironment {
	return &TaskEnvironment{node: node}
}

// ParseAndReplace takes the user supplied args replaces any instance of an
// environment variable or nomad variable in the args with the actual value.
func (t *TaskEnvironment) ParseAndReplace(args []string) []string {
	replaced := make([]string, len(args))
	for i, arg := range args {
		replaced[i] = hargs.ReplaceEnv(arg, t.taskEnv, t.nodeValues)
	}

	return replaced
}

// ReplaceEnv takes an arg and replaces all occurences of environment variables
// and nomad variables.  If the variable is found in the passed map it is
// replaced, otherwise the original string is returned.
func (t *TaskEnvironment) ReplaceEnv(arg string) string {
	return hargs.ReplaceEnv(arg, t.taskEnv, t.nodeValues)
}

// Build must be called after all the tasks environment values have been set.
func (t *TaskEnvironment) Build() *TaskEnvironment {
	t.nodeValues = make(map[string]string)
	t.taskEnv = make(map[string]string)

	// Build the task metadata
	for k, v := range t.meta {
		t.taskEnv[fmt.Sprintf("%s%s", MetaPrefix, strings.ToUpper(k))] = v
	}

	// Build the ports
	for _, network := range t.networks {
		for label, value := range network.MapLabelToValues(t.portMap) {
			IPPort := fmt.Sprintf("%s:%d", network.IP, value)
			t.taskEnv[fmt.Sprintf("%s%s", AddrPrefix, label)] = IPPort

			// Pass an explicit port mapping to the environment
			if port, ok := t.portMap[label]; ok {
				t.taskEnv[fmt.Sprintf("%s%s", HostPortPrefix, label)] = strconv.Itoa(port)
			}
		}
	}

	// Build the directories
	if t.allocDir != "" {
		t.taskEnv[AllocDir] = t.allocDir
	}
	if t.taskDir != "" {
		t.taskEnv[TaskLocalDir] = t.taskDir
	}

	// Build the resource limits
	if t.memLimit != 0 {
		t.taskEnv[MemLimit] = strconv.Itoa(t.memLimit)
	}
	if t.cpuLimit != 0 {
		t.taskEnv[CpuLimit] = strconv.Itoa(t.cpuLimit)
	}

	// Build the node
	if t.node != nil {
		// Set up the node values.
		t.nodeValues[nodeIdKey] = t.node.ID
		t.nodeValues[nodeDcKey] = t.node.Datacenter
		t.nodeValues[nodeNameKey] = t.node.Name
		t.nodeValues[nodeClassKey] = t.node.NodeClass

		// Set up the attributes.
		for k, v := range t.node.Attributes {
			t.nodeValues[fmt.Sprintf("%s%s", nodeAttributePrefix, k)] = v
		}

		// Set up the meta.
		for k, v := range t.node.Meta {
			t.nodeValues[fmt.Sprintf("%s%s", nodeMetaPrefix, k)] = v
		}
	}

	// Interpret the environment variables
	interpreted := make(map[string]string, len(t.env))
	for k, v := range t.env {
		interpreted[k] = hargs.ReplaceEnv(v, t.nodeValues, t.taskEnv)
	}

	for k, v := range interpreted {
		t.taskEnv[k] = v
	}

	return t
}

// EnvList returns a list of strings with NAME=value pairs.
func (t *TaskEnvironment) EnvList() []string {
	env := []string{}
	for k, v := range t.taskEnv {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}

// EnvMap returns a copy of the tasks environment variables.
func (t *TaskEnvironment) EnvMap() map[string]string {
	m := make(map[string]string, len(t.taskEnv))
	for k, v := range t.taskEnv {
		m[k] = v
	}

	return m
}

// Builder methods to build the TaskEnvironment
func (t *TaskEnvironment) SetAllocDir(dir string) *TaskEnvironment {
	t.allocDir = dir
	return t
}

func (t *TaskEnvironment) ClearAllocDir() *TaskEnvironment {
	t.allocDir = ""
	return t
}

func (t *TaskEnvironment) SetTaskLocalDir(dir string) *TaskEnvironment {
	t.taskDir = dir
	return t
}

func (t *TaskEnvironment) ClearTaskLocalDir() *TaskEnvironment {
	t.taskDir = ""
	return t
}

func (t *TaskEnvironment) SetMemLimit(limit int) *TaskEnvironment {
	t.memLimit = limit
	return t
}

func (t *TaskEnvironment) ClearMemLimit() *TaskEnvironment {
	t.memLimit = 0
	return t
}

func (t *TaskEnvironment) SetCpuLimit(limit int) *TaskEnvironment {
	t.cpuLimit = limit
	return t
}

func (t *TaskEnvironment) ClearCpuLimit() *TaskEnvironment {
	t.cpuLimit = 0
	return t
}

func (t *TaskEnvironment) SetNetworks(networks []*structs.NetworkResource) *TaskEnvironment {
	t.networks = networks
	return t
}

func (t *TaskEnvironment) clearNetworks() *TaskEnvironment {
	t.networks = nil
	return t
}

func (t *TaskEnvironment) SetPortMap(portMap map[string]int) *TaskEnvironment {
	t.portMap = portMap
	return t
}

func (t *TaskEnvironment) clearPortMap() *TaskEnvironment {
	t.portMap = nil
	return t
}

// Takes a map of meta values to be passed to the task. The keys are capatilized
// when the environent variable is set.
func (t *TaskEnvironment) SetMeta(m map[string]string) *TaskEnvironment {
	t.meta = m
	return t
}

func (t *TaskEnvironment) ClearMeta() *TaskEnvironment {
	t.meta = nil
	return t
}

func (t *TaskEnvironment) SetEnvvars(m map[string]string) *TaskEnvironment {
	t.env = m
	return t
}

// Appends the given environment variables.
func (t *TaskEnvironment) AppendEnvvars(m map[string]string) *TaskEnvironment {
	if t.env == nil {
		t.env = make(map[string]string, len(m))
	}

	for k, v := range m {
		t.env[k] = v
	}
	return t
}

func (t *TaskEnvironment) ClearEnvvars() *TaskEnvironment {
	t.env = nil
	return t
}
