package environment

import (
	"fmt"
	"strconv"
	"strings"
)

// A set of environment variables that are exported by each driver.
const (
	// The path to the alloc directory that is shared across tasks within a task
	// group.
	AllocDir = "NOMAD_ALLOC_DIR"

	// The tasks memory limit in MBs.
	MemLimit = "NOMAD_MEMORY_LIMIT"

	// The tasks limit in MHz.
	CpuLimit = "NOMAD_CPU_LIMIT"

	// The IP address for the task.
	TaskIP = "NOMAD_IP"

	// Prefix for passing both dynamic and static port allocations to
	// tasks.
	// E.g. $NOMAD_PORT_1 or $NOMAD_PORT_http
	PortPrefix = "NOMAD_PORT_"

	// Prefix for passing task meta data.
	MetaPrefix = "NOMAD_META_"
)

type TaskEnvironment map[string]string

func NewTaskEnivornment() TaskEnvironment {
	return make(map[string]string)
}

// Parses a list of strings with NAME=value pairs and returns a TaskEnvironment.
func ParseFromList(envVars []string) (TaskEnvironment, error) {
	t := NewTaskEnivornment()

	for _, pair := range envVars {
		parts := strings.Split(pair, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("Couldn't parse environment variable: %v", pair)
		}

		t[parts[0]] = parts[1]
	}

	return t, nil
}

// Returns a list of strings with NAME=value pairs.
func (t TaskEnvironment) List() []string {
	env := []string{}
	for k, v := range t {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}

func (t TaskEnvironment) Map() map[string]string {
	return t
}

func (t TaskEnvironment) SetAllocDir(dir string) {
	t[AllocDir] = dir
}

func (t TaskEnvironment) SetMemLimit(limit int) {
	t[MemLimit] = strconv.Itoa(limit)
}

func (t TaskEnvironment) SetCpuLimit(limit int) {
	t[CpuLimit] = strconv.Itoa(limit)
}

func (t TaskEnvironment) SetTaskIp(ip string) {
	t[TaskIP] = ip
}

// Takes a map of port labels to their port value.
func (t TaskEnvironment) SetPorts(ports map[string]int) {
	for label, port := range ports {
		t[fmt.Sprintf("%s%s", PortPrefix, label)] = strconv.Itoa(port)
	}
}

// Takes a map of meta values to be passed to the task. The keys are capatilized
// when the environent variable is set.
func (t TaskEnvironment) SetMeta(m map[string]string) {
	for k, v := range m {
		t[fmt.Sprintf("%s%s", MetaPrefix, strings.ToUpper(k))] = v
	}
}

func (t TaskEnvironment) SetEnvvars(m map[string]string) {
	for k, v := range m {
		t[strings.ToUpper(k)] = v
	}
}
