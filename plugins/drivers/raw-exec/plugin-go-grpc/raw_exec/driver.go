package raw_exec

import (
	"log"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// NewDriverContext initializes a new DriverContext with the specified fields.
// This enables other packages to create DriverContexts but keeps the fields
// private to the driver. If we want to change this later we can gorename all of
// the fields in DriverContext.
func NewDriverContext(jobName, taskGroupName, taskName, allocID string,
	config *config.Config, node *structs.Node,
	logger *log.Logger, eventEmitter LogEventFn) *DriverContext {
	return &DriverContext{
		jobName:       jobName,
		taskGroupName: taskGroupName,
		taskName:      taskName,
		allocID:       allocID,
		Config:        config,
		node:          node,
		logger:        logger,
		emitEvent:     eventEmitter,
	}
}

// DriverContext is a means to inject dependencies such as loggers, configs, and
// node attributes into a Driver without having to change the Driver interface
// each time we do it. Used in conjunction with Factory, above.
type DriverContext struct {
	jobName       string
	taskGroupName string
	taskName      string
	allocID       string
	Config        *config.Config
	logger        *log.Logger
	node          *structs.Node

	emitEvent LogEventFn
}
