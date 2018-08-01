package raw_exec

import (
	"io"
	"os"
	"time"
)

// ExecContext is a task's execution context
type ExecContext struct {
	// TaskDir contains information about the task directory structure.
	*TaskDir

	// TaskEnv contains the task's environment variables.
	*TaskEnv

	LogOutput io.Writer

	MaxPort        uint
	MinPort        uint
	MaxKillTimeout time.Duration
	Version        string
}

type TaskEnv struct {
	NodeAttrs map[string]string
	EnvMap    map[string]string
}

type Resources struct {
	CPU      int
	MemoryMB int
	DiskMB   int
	IOPS     int
	*Networks
}

type LogConfig struct {
	MaxFiles      int
	MaxFileSizeMB int
}
type Config struct {
	Command string
	Args    []string
}

type TaskInfo struct {
	*Resources
	*LogConfig
	*Config
	User string
	Name string
}

type TaskDir struct {
	Dir       string
	LogDir    string
	LogLevel  string
	LogOutput *os.File
}

type Port struct {
	Label string
	Value uint32
}

type Networks struct {
	device        string
	cidr          string
	ip            string
	mbits         uint64
	ReservedPorts *Port
	DynamicPorts  *Port
}

type ExecutorConfig struct {
	LogFile  string
	LogLevel string
	MaxPort  uint
	MinPort  uint
}
