package executor

import (
	"encoding/json"
	"fmt"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/opencontainers/runc/libcontainer/system"
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

func processStartTime(pid int) uint64 {
	stat, err := system.Stat(pid)
	if err != nil {
		return 0
	}

	return stat.StartTime
}

func CleanupExecutor(logger hclog.Logger, cleanupHandleData []byte) error {
	var cl cleanupHandle
	err := json.Unmarshal(cleanupHandleData, &cl)
	if err != nil {
		return fmt.Errorf("failed to unmarshal cleanup handle data: %v", err)
	}

	if cfn, ok := executorCleanupFns[cl.ExecutorType]; ok {
		return cfn(logger, &cl)
	}

	return fmt.Errorf("unknown executor type: %v", cl.ExecutorType)
}
