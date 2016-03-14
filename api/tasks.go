package api

import (
	"time"
)

// RestartPolicy defines how the Nomad client restarts
// tasks in a taskgroup when they fail
type RestartPolicy struct {
	Interval time.Duration
	Attempts int
	Delay    time.Duration
	Mode     string
}

// The ServiceCheck data model represents the consul health check that
// Nomad registers for a Task
type ServiceCheck struct {
	Id       string
	Name     string
	Type     string
	Script   string
	Path     string
	Protocol string
	Interval time.Duration
	Timeout  time.Duration
}

// The Service model represents a Consul service defintion
type Service struct {
	Id        string
	Name      string
	Tags      []string
	PortLabel string `mapstructure:"port"`
	Checks    []ServiceCheck
}

// TaskGroup is the unit of scheduling.
type TaskGroup struct {
	Name          string
	Count         int
	Constraints   []*Constraint
	Tasks         []*Task
	RestartPolicy *RestartPolicy
	Meta          map[string]string
}

// NewTaskGroup creates a new TaskGroup.
func NewTaskGroup(name string, count int) *TaskGroup {
	return &TaskGroup{
		Name:  name,
		Count: count,
	}
}

// Constrain is used to add a constraint to a task group.
func (g *TaskGroup) Constrain(c *Constraint) *TaskGroup {
	g.Constraints = append(g.Constraints, c)
	return g
}

// AddMeta is used to add a meta k/v pair to a task group
func (g *TaskGroup) SetMeta(key, val string) *TaskGroup {
	if g.Meta == nil {
		g.Meta = make(map[string]string)
	}
	g.Meta[key] = val
	return g
}

// AddTask is used to add a new task to a task group.
func (g *TaskGroup) AddTask(t *Task) *TaskGroup {
	g.Tasks = append(g.Tasks, t)
	return g
}

// LogConfig provides configuration for log rotation
type LogConfig struct {
	MaxFiles      int
	MaxFileSizeMB int
}

// Task is a single process in a task group.
type Task struct {
	Name        string
	Driver      string
	Config      map[string]interface{}
	Constraints []*Constraint
	Env         map[string]string
	Services    []Service
	Resources   *Resources
	Meta        map[string]string
	KillTimeout time.Duration
	LogConfig   *LogConfig
}

// NewTask creates and initializes a new Task.
func NewTask(name, driver string) *Task {
	return &Task{
		Name:   name,
		Driver: driver,
	}
}

// Configure is used to configure a single k/v pair on
// the task.
func (t *Task) SetConfig(key, val string) *Task {
	if t.Config == nil {
		t.Config = make(map[string]interface{})
	}
	t.Config[key] = val
	return t
}

// SetMeta is used to add metadata k/v pairs to the task.
func (t *Task) SetMeta(key, val string) *Task {
	if t.Meta == nil {
		t.Meta = make(map[string]string)
	}
	t.Meta[key] = val
	return t
}

// Require is used to add resource requirements to a task.
func (t *Task) Require(r *Resources) *Task {
	t.Resources = r
	return t
}

// Constraint adds a new constraints to a single task.
func (t *Task) Constrain(c *Constraint) *Task {
	t.Constraints = append(t.Constraints, c)
	return t
}

// SetLogConfig sets a log config to a task
func (t *Task) SetLogConfig(l *LogConfig) *Task {
	t.LogConfig = l
	return t
}

// TaskState tracks the current state of a task and events that caused state
// transistions.
type TaskState struct {
	State  string
	Events []*TaskEvent
}

const (
	TaskDriverFailure = "Driver Failure"
	TaskReceived      = "Received"
	TaskStarted       = "Started"
	TaskTerminated    = "Terminated"
	TaskKilled        = "Killed"
	TaskRestarting    = "Restarting"
	TaskNotRestarting = "Restarts Exceeded"
)

// TaskEvent is an event that effects the state of a task and contains meta-data
// appropriate to the events type.
type TaskEvent struct {
	Type        string
	Time        int64
	DriverError string
	ExitCode    int
	Signal      int
	Message     string
	KillError   string
	StartDelay  int64
}
