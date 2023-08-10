// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	tinterfaces "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/client/taskenv"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/structs"
)

var _ interfaces.TaskPoststartHook = &scriptCheckHook{}
var _ interfaces.TaskUpdateHook = &scriptCheckHook{}
var _ interfaces.TaskStopHook = &scriptCheckHook{}

// default max amount of time to wait for all scripts on shutdown.
const defaultShutdownWait = time.Minute

type scriptCheckHookConfig struct {
	alloc        *structs.Allocation
	task         *structs.Task
	consul       serviceregistration.Handler
	logger       log.Logger
	shutdownWait time.Duration
}

// scriptCheckHook implements a task runner hook for running script
// checks in the context of a task
type scriptCheckHook struct {
	consul          serviceregistration.Handler
	consulNamespace string
	alloc           *structs.Allocation
	task            *structs.Task
	logger          log.Logger
	shutdownWait    time.Duration // max time to wait for scripts to shutdown
	shutdownCh      chan struct{} // closed when all scripts should shutdown

	// The following fields can be changed by Update()
	driverExec tinterfaces.ScriptExecutor
	taskEnv    *taskenv.TaskEnv

	// These maintain state and are populated by Poststart() or Update()
	scripts        map[string]*scriptCheck
	runningScripts map[string]*taskletHandle

	// Since Update() may be called concurrently with any other hook all
	// hook methods must be fully serialized
	mu sync.Mutex
}

// newScriptCheckHook returns a hook without any scriptChecks.
// They will get created only once their task environment is ready
// in Poststart() or Update()
func newScriptCheckHook(c scriptCheckHookConfig) *scriptCheckHook {
	h := &scriptCheckHook{
		consul:          c.consul,
		consulNamespace: c.alloc.Job.LookupTaskGroup(c.alloc.TaskGroup).Consul.GetNamespace(),
		alloc:           c.alloc,
		task:            c.task,
		scripts:         make(map[string]*scriptCheck),
		runningScripts:  make(map[string]*taskletHandle),
		shutdownWait:    defaultShutdownWait,
		shutdownCh:      make(chan struct{}),
	}

	if c.shutdownWait != 0 {
		h.shutdownWait = c.shutdownWait // override for testing
	}
	h.logger = c.logger.Named(h.Name())
	return h
}

func (h *scriptCheckHook) Name() string {
	return "script_checks"
}

// Prestart implements interfaces.TaskPrestartHook. It stores the
// initial structs.Task
func (h *scriptCheckHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, _ *interfaces.TaskPrestartResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.task = req.Task
	return nil
}

// PostStart implements interfaces.TaskPoststartHook. It creates new
// script checks with the current task context (driver and env), and
// starts up the scripts.
func (h *scriptCheckHook) Poststart(ctx context.Context, req *interfaces.TaskPoststartRequest, _ *interfaces.TaskPoststartResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if req.DriverExec == nil {
		h.logger.Debug("driver doesn't support script checks")
		return nil
	}
	h.driverExec = req.DriverExec
	h.taskEnv = req.TaskEnv

	return h.upsertChecks()
}

// Updated implements interfaces.TaskUpdateHook. It creates new
// script checks with the current task context (driver and env and possibly
// new structs.Task), and starts up the scripts.
func (h *scriptCheckHook) Update(ctx context.Context, req *interfaces.TaskUpdateRequest, _ *interfaces.TaskUpdateResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	task := req.Alloc.LookupTask(h.task.Name)
	if task == nil {
		return fmt.Errorf("task %q not found in updated alloc", h.task.Name)
	}
	h.alloc = req.Alloc
	h.task = task
	h.taskEnv = req.TaskEnv

	return h.upsertChecks()
}

func (h *scriptCheckHook) upsertChecks() error {
	// Create new script checks struct with new task context
	oldScriptChecks := h.scripts
	h.scripts = h.newScriptChecks()

	// Run new or replacement scripts
	for id, script := range h.scripts {
		// If it's already running, cancel and replace
		if oldScript, running := h.runningScripts[id]; running {
			oldScript.cancel()
		}
		// Start and store the handle
		h.runningScripts[id] = script.run()
	}

	// Cancel scripts we no longer want
	for id := range oldScriptChecks {
		if _, ok := h.scripts[id]; !ok {
			if oldScript, running := h.runningScripts[id]; running {
				oldScript.cancel()
			}
		}
	}
	return nil
}

// Stop implements interfaces.TaskStopHook and blocks waiting for running
// scripts to finish (or for the shutdownWait timeout to expire).
func (h *scriptCheckHook) Stop(ctx context.Context, req *interfaces.TaskStopRequest, resp *interfaces.TaskStopResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	close(h.shutdownCh)
	deadline := time.After(h.shutdownWait)
	err := fmt.Errorf("timed out waiting for script checks to exit")
	for _, script := range h.runningScripts {
		select {
		case <-script.wait():
		case <-ctx.Done():
			// the caller is passing the background context, so
			// we should never really see this outside of testing
		case <-deadline:
			// at this point the Consul client has been cleaned
			// up so we don't want to hang onto this.
			return err
		}
	}
	return nil
}

func (h *scriptCheckHook) newScriptChecks() map[string]*scriptCheck {
	scriptChecks := make(map[string]*scriptCheck)
	interpolatedTaskServices := taskenv.InterpolateServices(h.taskEnv, h.task.Services)
	for _, service := range interpolatedTaskServices {
		for _, check := range service.Checks {
			if check.Type != structs.ServiceCheckScript {
				continue
			}
			serviceID := serviceregistration.MakeAllocServiceID(
				h.alloc.ID, h.task.Name, service)
			sc := newScriptCheck(&scriptCheckConfig{
				consulNamespace: h.consulNamespace,
				allocID:         h.alloc.ID,
				taskName:        h.task.Name,
				check:           check,
				serviceID:       serviceID,
				ttlUpdater:      h.consul,
				driverExec:      h.driverExec,
				taskEnv:         h.taskEnv,
				logger:          h.logger,
				shutdownCh:      h.shutdownCh,
			})
			if sc != nil {
				scriptChecks[sc.id] = sc
			}
		}
	}

	// Walk back through the task group to see if there are script checks
	// associated with the task. If so, we'll create scriptCheck tasklets
	// for them. The group-level service and any check restart behaviors it
	// needs are entirely encapsulated within the group service hook which
	// watches Consul for status changes.
	//
	// The script check is associated with a group task if the service.task or
	// service.check.task matches the task name. The service.check.task takes
	// precedence.
	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)
	interpolatedGroupServices := taskenv.InterpolateServices(h.taskEnv, tg.Services)
	for _, service := range interpolatedGroupServices {
		for _, check := range service.Checks {
			if check.Type != structs.ServiceCheckScript {
				continue
			}
			if !h.associated(h.task.Name, service.TaskName, check.TaskName) {
				continue
			}
			groupTaskName := "group-" + tg.Name
			serviceID := serviceregistration.MakeAllocServiceID(
				h.alloc.ID, groupTaskName, service)
			sc := newScriptCheck(&scriptCheckConfig{
				consulNamespace: h.consulNamespace,
				allocID:         h.alloc.ID,
				taskName:        groupTaskName,
				check:           check,
				serviceID:       serviceID,
				ttlUpdater:      h.consul,
				driverExec:      h.driverExec,
				taskEnv:         h.taskEnv,
				logger:          h.logger,
				shutdownCh:      h.shutdownCh,
				isGroup:         true,
			})
			if sc != nil {
				scriptChecks[sc.id] = sc
			}
		}
	}
	return scriptChecks
}

// associated returns true if the script check is associated with the task. This
// would be the case if the check.task is the same as task, or if the service.task
// is the same as the task _and_ check.task is not configured (i.e. the check
// inherits the task of the service).
func (*scriptCheckHook) associated(task, serviceTask, checkTask string) bool {
	if checkTask == task {
		return true
	}
	if serviceTask == task && checkTask == "" {
		return true
	}
	return false
}

// TTLUpdater is the subset of consul agent functionality needed by script
// checks to heartbeat
type TTLUpdater interface {
	UpdateTTL(id, namespace, output, status string) error
}

// scriptCheck runs script checks via a interfaces.ScriptExecutor and updates the
// appropriate check's TTL when the script succeeds.
type scriptCheck struct {
	id              string
	consulNamespace string
	ttlUpdater      TTLUpdater
	check           *structs.ServiceCheck
	lastCheckOk     bool // true if the last check was ok; otherwise false
	tasklet
}

// scriptCheckConfig is a parameter struct for newScriptCheck
type scriptCheckConfig struct {
	allocID         string
	taskName        string
	serviceID       string
	consulNamespace string
	check           *structs.ServiceCheck
	ttlUpdater      TTLUpdater
	driverExec      tinterfaces.ScriptExecutor
	taskEnv         *taskenv.TaskEnv
	logger          log.Logger
	shutdownCh      chan struct{}
	isGroup         bool
}

// newScriptCheck constructs a scriptCheck. we're only going to
// configure the immutable fields of scriptCheck here, with the
// rest being configured during the Poststart hook so that we have
// the rest of the task execution environment
func newScriptCheck(config *scriptCheckConfig) *scriptCheck {

	// Guard against not having a valid taskEnv. This can be the case if the
	// PreKilling or Exited hook is run before Poststart.
	if config.taskEnv == nil || config.driverExec == nil {
		return nil
	}

	orig := config.check
	sc := &scriptCheck{
		ttlUpdater:  config.ttlUpdater,
		check:       config.check.Copy(),
		lastCheckOk: true, // start logging on first failure
	}

	// we can't use the promoted fields of tasklet in the struct literal
	sc.Command = config.taskEnv.ReplaceEnv(config.check.Command)
	sc.Args = config.taskEnv.ParseAndReplace(config.check.Args)
	sc.Interval = config.check.Interval
	sc.Timeout = config.check.Timeout
	sc.exec = config.driverExec
	sc.callback = newScriptCheckCallback(sc)
	sc.logger = config.logger
	sc.shutdownCh = config.shutdownCh
	sc.check.Command = sc.Command
	sc.check.Args = sc.Args

	if config.isGroup {
		// group services don't have access to a task environment
		// at creation, so their checks get registered before the
		// check can be interpolated here. if we don't use the
		// original checkID, they can't be updated.
		sc.id = agentconsul.MakeCheckID(config.serviceID, orig)
	} else {
		sc.id = agentconsul.MakeCheckID(config.serviceID, sc.check)
	}
	sc.consulNamespace = config.consulNamespace
	return sc
}

// Copy does a *shallow* copy of script checks.
func (sc *scriptCheck) Copy() *scriptCheck {
	newSc := sc
	return newSc
}

// closes over the script check and returns the taskletCallback for
// when the script check executes.
func newScriptCheckCallback(s *scriptCheck) taskletCallback {

	return func(ctx context.Context, params execResult) {
		output := params.output
		code := params.code
		err := params.err

		state := api.HealthCritical
		switch code {
		case 0:
			state = api.HealthPassing
		case 1:
			state = api.HealthWarning
		}

		var outputMsg string
		if err != nil {
			state = api.HealthCritical
			outputMsg = err.Error()
		} else {
			outputMsg = string(output)
		}

		// heartbeat the check to Consul
		err = s.updateTTL(ctx, outputMsg, state)
		select {
		case <-ctx.Done():
			// check has been removed; don't report errors
			return
		default:
		}

		if err != nil {
			if s.lastCheckOk {
				s.lastCheckOk = false
				s.logger.Warn("updating check failed", "error", err)
			} else {
				s.logger.Debug("updating check still failing", "error", err)
			}

		} else if !s.lastCheckOk {
			// Succeeded for the first time or after failing; log
			s.lastCheckOk = true
			s.logger.Info("updating check succeeded")
		}
	}
}

const (
	updateTTLBackoffBaseline = 1 * time.Second
	updateTTLBackoffLimit    = 3 * time.Second
)

// updateTTL updates the state to Consul, performing an exponential backoff
// in the case where the check isn't registered in Consul to avoid a race between
// service registration and the first check.
func (sc *scriptCheck) updateTTL(ctx context.Context, msg, state string) error {
	for attempts := 0; ; attempts++ {
		err := sc.ttlUpdater.UpdateTTL(sc.id, sc.consulNamespace, msg, state)
		if err == nil {
			return nil
		}

		// Handle the retry case
		backoff := (1 << (2 * uint64(attempts))) * updateTTLBackoffBaseline
		if backoff > updateTTLBackoffLimit {
			return err
		}

		// Wait till retrying
		select {
		case <-ctx.Done():
			return err
		case <-time.After(backoff):
		}
	}
}
