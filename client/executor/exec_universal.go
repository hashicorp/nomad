// +build !linux

package executor

import (
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/nomad/structs"
)

// UniversalExecutor exists to make the exec driver compile on all operating systems.
type UniversalExecutor struct{}

func NewExecutor() Executor                                                    { return &UniversalExecutor{} }
func (e *UniversalExecutor) Limit(resources *structs.Resources) error          { return nil }
func (e *UniversalExecutor) ConfigureTaskDir(string, *allocdir.AllocDir) error { return nil }
func (e *UniversalExecutor) Start() error                                      { return nil }
func (e *UniversalExecutor) Open(pid string) error                             { return nil }
func (e *UniversalExecutor) Wait() error                                       { return nil }
func (e *UniversalExecutor) ID() (string, error)                               { return "", nil }
func (e *UniversalExecutor) Shutdown() error                                   { return nil }
func (e *UniversalExecutor) ForceStop() error                                  { return nil }
func (e *UniversalExecutor) Command() *cmd                                     { return nil }
