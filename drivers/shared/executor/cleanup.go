package executor

import (
	"encoding/json"
	"fmt"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/shirou/gopsutil/process"
)

type cleanupHandleFn func(hclog.Logger, *cleanupHandle) error

// cleanupHandle represents state required to recreate the executor handle
type universalData struct {
	CommandConfig *ExecCommand
	Cgroups       resourceContainerContext
}

type libcontainerData struct {
	Root        string
	ContainerId string
}

type cleanupHandle struct {
	Version      string
	ExecutorType string
	Pid          int
	StartTime    uint64

	UniversalData    universalData
	LibcontainerData libcontainerData
}

func (b *cleanupHandle) serialize() []byte {
	bytes, err := json.Marshal(b)
	if err != nil {
		// this type should always serialized
		panic(fmt.Errorf("failed to serialize handle: %v", err))
	}

	return bytes
}

// processStartTime attempts to determine the create time of a given process,
// in milliseconds since epoch.
func processStartTime(pid int) (uint64, error) {
	ps, err := process.NewProcess(int32(pid))
	if err != nil {
		return 0, fmt.Errorf("failed to find process: %v", err)
	}

	st, err := ps.CreateTime()
	if err != nil {
		return 0, fmt.Errorf("failed to find process start time: %v", err)
	}

	if st < 0 {
		return 0, fmt.Errorf("process started before unix epoch, or overflowed int64: %v", st)
	}

	return uint64(st), nil
}

func CleanupExecutor(logger hclog.Logger, cleanupHandleData []byte) error {
	if len(cleanupHandleData) == 0 {
		return fmt.Errorf("empty clean up handler")
	}
	var cl cleanupHandle
	err := json.Unmarshal(cleanupHandleData, &cl)
	if err != nil {
		return fmt.Errorf("failed to unmarshal cleanup handle data: %v", err)
	}

	logger.Info("cleaning up executor and killing children processes", "task_pid", cl.Pid, "task_start_time", cl.StartTime)

	if cfn, ok := executorCleanupFns[cl.ExecutorType]; ok {
		return cfn(logger, &cl)
	}

	return fmt.Errorf("unknown executor type: %v", cl.ExecutorType)
}
