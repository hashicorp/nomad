package taskrunner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	tinterfaces "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
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
	consul       consul.ConsulServiceAPI
	logger       log.Logger
	shutdownWait time.Duration
}

// scriptCheckHook implements a task runner hook for running script
// checks in the context of a task
type scriptCheckHook struct {
	consul       consul.ConsulServiceAPI
	allocID      string
	taskName     string
	logger       log.Logger
	shutdownWait time.Duration // max time to wait for scripts to shutdown
	shutdownCh   chan struct{} // closed when all scripts should shutdown

	// The following fields can be changed by Update()
	driverExec tinterfaces.ScriptExecutor
	taskEnv    *taskenv.TaskEnv

	// These maintain state
	scripts        map[string]*scriptCheck
	runningScripts map[string]*taskletHandle

	// Since Update() may be called concurrently with any other hook all
	// hook methods must be fully serialized
	mu sync.Mutex
}

func newScriptCheckHook(c scriptCheckHookConfig) *scriptCheckHook {
	scriptChecks := make(map[string]*scriptCheck)
	for _, service := range c.task.Services {
		for _, check := range service.Checks {
			if check.Type != structs.ServiceCheckScript {
				continue
			}
			sc := newScriptCheck(&scriptCheckConfig{
				allocID:  c.alloc.ID,
				taskName: c.task.Name,
				check:    check,
				service:  service,
				agent:    c.consul,
			})
			scriptChecks[sc.id] = sc
		}
	}

	h := &scriptCheckHook{
		consul:         c.consul,
		allocID:        c.alloc.ID,
		taskName:       c.task.Name,
		scripts:        scriptChecks,
		runningScripts: make(map[string]*taskletHandle),
		shutdownWait:   defaultShutdownWait,
		shutdownCh:     make(chan struct{}),
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

// PostStart implements interfaces.TaskPoststartHook. It adds the current
// task context (driver and env) to the script checks and starts up the
// scripts.
func (h *scriptCheckHook) Poststart(ctx context.Context, req *interfaces.TaskPoststartRequest, _ *interfaces.TaskPoststartResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if req.DriverExec == nil {
		return fmt.Errorf("driver doesn't support script checks")
	}

	// Store the TaskEnv for interpolating now and when Updating
	h.driverExec = req.DriverExec
	h.taskEnv = req.TaskEnv
	h.scripts = h.getTaskScriptChecks()

	// Handle starting scripts
	for checkID, script := range h.scripts {
		// If it's already running, cancel and replace
		if oldScript, running := h.runningScripts[checkID]; running {
			oldScript.cancel()
		}
		// Start and store the handle
		h.runningScripts[checkID] = script.run()
	}
	return nil
}

// Updated implements interfaces.TaskUpdateHook. It adds the current
// task context (driver and env) to the script checks and replaces any
// that have been changed.
func (h *scriptCheckHook) Update(ctx context.Context, req *interfaces.TaskUpdateRequest, _ *interfaces.TaskUpdateResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Get current script checks with request's driver metadata as it
	// can't change due to Updates
	oldScriptChecks := h.getTaskScriptChecks()

	task := req.Alloc.LookupTask(h.taskName)
	if task == nil {
		return fmt.Errorf("task %q not found in updated alloc", h.taskName)
	}

	// Update service hook fields
	h.taskEnv = req.TaskEnv

	// Create new script checks struct with those new values
	newScriptChecks := h.getTaskScriptChecks()

	// Handle starting scripts
	for checkID, script := range newScriptChecks {
		if _, ok := oldScriptChecks[checkID]; ok {
			// If it's already running, cancel and replace
			if oldScript, running := h.runningScripts[checkID]; running {
				oldScript.cancel()
			}
			// Start and store the handle
			h.runningScripts[checkID] = script.run()
		}
	}

	// Cancel scripts we no longer want
	for checkID := range oldScriptChecks {
		if _, ok := newScriptChecks[checkID]; !ok {
			if oldScript, running := h.runningScripts[checkID]; running {
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

// getTaskScriptChecks returns an interpolated copy of services and checks with
// values from the task's environment.
func (h *scriptCheckHook) getTaskScriptChecks() map[string]*scriptCheck {
	// Guard against not having a valid taskEnv. This can be the case if the
	// PreKilling or Exited hook is run before Poststart.
	if h.taskEnv == nil || h.driverExec == nil {
		return nil
	}
	newChecks := make(map[string]*scriptCheck)
	for _, orig := range h.scripts {
		sc := orig.Copy()
		sc.exec = h.driverExec
		sc.logger = h.logger
		sc.shutdownCh = h.shutdownCh
		sc.callback = newScriptCheckCallback(sc)
		sc.Command = h.taskEnv.ReplaceEnv(orig.Command)
		sc.Args = h.taskEnv.ParseAndReplace(orig.Args)
		newChecks[sc.id] = sc
	}
	return newChecks
}

// heartbeater is the subset of consul agent functionality needed by script
// checks to heartbeat
type heartbeater interface {
	UpdateTTL(id, output, status string) error
}

// scriptCheck runs script checks via a interfaces.ScriptExecutor and updates the
// appropriate check's TTL when the script succeeds.
type scriptCheck struct {
	id          string
	agent       heartbeater
	lastCheckOk bool // true if the last check was ok; otherwise false
	tasklet
}

// scriptCheckConfig is a parameter struct for newScriptCheck
type scriptCheckConfig struct {
	allocID  string
	taskName string
	service  *structs.Service
	check    *structs.ServiceCheck
	agent    heartbeater
}

// newScriptCheck constructs a scriptCheck. we're only going to
// configure the immutable fields of scriptCheck here, with the
// rest being configured during the Poststart hook so that we have
// the rest of the task execution environment
func newScriptCheck(config *scriptCheckConfig) *scriptCheck {
	serviceID := agentconsul.MakeTaskServiceID(
		config.allocID, config.taskName, config.service)
	checkID := agentconsul.MakeCheckID(serviceID, config.check)

	sc := &scriptCheck{
		id:          checkID,
		agent:       config.agent,
		lastCheckOk: true, // start logging on first failure
	}
	// we can't use the promoted fields of tasklet in the struct literal
	sc.allocID = config.allocID
	sc.taskName = config.taskName
	sc.Command = config.check.Command
	sc.Args = config.check.Args
	sc.Interval = config.check.Interval
	sc.Timeout = config.check.Timeout
	return sc
}

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
		err = s.updateTTL(ctx, s.id, outputMsg, state)
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

// updateTTL updates the state to Consul, performing an expontential backoff
// in the case where the check isn't registered in Consul to avoid a race between
// service registration and the first check.
func (s *scriptCheck) updateTTL(ctx context.Context, id, msg, state string) error {
	for attempts := 0; ; attempts++ {
		err := s.agent.UpdateTTL(id, msg, state)
		if err == nil ||
			!strings.Contains(err.Error(), "does not have associated TTL") {
			return err
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
