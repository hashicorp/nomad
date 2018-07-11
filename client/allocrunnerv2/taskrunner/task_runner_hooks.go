package taskrunner

import (
	"fmt"
	"time"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocrunner/getter"
	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
	"github.com/hashicorp/nomad/client/allocrunnerv2/taskrunner/state"
	cconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/nomad/structs"
)

// initHooks intializes the tasks hooks.
func (tr *TaskRunner) initHooks() {
	hookLogger := tr.logger.Named("task_hook")

	// Create the task directory hook. This is run first to ensure the
	// directoy path exists for other hooks.
	tr.runnerHooks = []interfaces.TaskHook{
		newTaskDirHook(tr, hookLogger),
		newArtifactHook(tr, hookLogger),
	}
}

// prerun is used to run the runners prerun hooks.
func (tr *TaskRunner) prerun() error {
	//XXX is this necessary? maybe we should have a generic cancelletion
	//    method instead of peeking into the alloc
	// Determine if the allocation is terminaland we should avoid running
	// pre-run hooks.
	alloc := tr.Alloc()
	if alloc.TerminalStatus() {
		tr.logger.Trace("skipping pre-run hooks since allocation is terminal")
		return nil
	}

	if tr.logger.IsTrace() {
		start := time.Now()
		tr.logger.Trace("running pre-run hooks", "start", start)
		defer func() {
			end := time.Now()
			tr.logger.Trace("finished pre-run hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	for _, hook := range tr.runnerHooks {
		pre, ok := hook.(interfaces.TaskPrerunHook)
		if !ok {
			tr.logger.Trace("skipping: not a prerun hook", "name", hook.Name())
			continue
		}

		name := pre.Name()
		// Build the request
		req := interfaces.TaskPrerunRequest{
			Task:    tr.Task(),
			TaskDir: tr.taskDir.Dir,
			TaskEnv: tr.envBuilder.Build(),
		}

		origHookState := tr.localState.Hooks[name]
		if origHookState != nil && origHookState.PrerunDone {
			tr.logger.Trace("skipping: prerun hook already done", "name", pre.Name())
			continue
		}

		req.VaultToken = tr.localState.VaultToken

		// Time the prerun hook
		var start time.Time
		if tr.logger.IsTrace() {
			start = time.Now()
			tr.logger.Trace("running pre-run hook", "name", name, "start", start)
		}

		// Run the pre-run hook
		var resp interfaces.TaskPrerunResponse
		if err := pre.Prerun(&req, &resp); err != nil {
			return structs.WrapRecoverable(fmt.Sprintf("pre-run hook %q failed: %v", name, err), err)
		}

		// Store the hook state
		{
			hookState, ok := tr.localState.Hooks[name]
			if !ok {
				hookState = &state.HookState{}
				tr.localState.Hooks[name] = hookState
			}

			if resp.HookData != nil {
				hookState.Data = resp.HookData
				hookState.PrerunDone = resp.Done
			}

			// Persist local state if the hook state has changed
			if !hookState.Equal(origHookState) {
				if err := tr.persistLocalState(); err != nil {
					return err
				}
			}
		}

		// Store the environment variables returned by the hook
		if len(resp.Env) != 0 {
			tr.envBuilder.SetGenericEnv(resp.Env)
		}

		if tr.logger.IsTrace() {
			end := time.Now()
			tr.logger.Trace("finished pre-run hooks", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return nil
}

// postrun is used to run the runners postrun hooks.
func (tr *TaskRunner) postrun() error {
	if tr.logger.IsTrace() {
		start := time.Now()
		tr.logger.Trace("running post-run hooks", "start", start)
		defer func() {
			end := time.Now()
			tr.logger.Trace("finished post-run hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	for _, hook := range tr.runnerHooks {
		post, ok := hook.(interfaces.TaskPostrunHook)
		if !ok {
			continue
		}

		name := post.Name()
		var start time.Time
		if tr.logger.IsTrace() {
			start = time.Now()
			tr.logger.Trace("running post-run hook", "name", name, "start", start)
		}

		// XXX We shouldn't exit on the first one
		if err := post.Postrun(); err != nil {
			return fmt.Errorf("post-run hook %q failed: %v", name, err)
		}

		if tr.logger.IsTrace() {
			end := time.Now()
			tr.logger.Trace("finished post-run hooks", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return nil
}

// shutdown is used to run the shutdown hooks.
func (tr *TaskRunner) shutdown() error {
	if tr.logger.IsTrace() {
		start := time.Now()
		tr.logger.Trace("running poststop hooks", "start", start)
		defer func() {
			end := time.Now()
			tr.logger.Trace("finished poststop hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	for _, hook := range tr.runnerHooks {
		post, ok := hook.(interfaces.TaskDestroyHook)
		if !ok {
			continue
		}

		name := post.Name()
		var start time.Time
		if tr.logger.IsTrace() {
			start = time.Now()
			tr.logger.Trace("running destroy hook", "name", name, "start", start)
		}

		// XXX We shouldn't exit on the first one
		if err := post.Destroy(); err != nil {
			return fmt.Errorf("destroy hook %q failed: %v", name, err)
		}

		if tr.logger.IsTrace() {
			end := time.Now()
			tr.logger.Trace("finished destroy hooks", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return nil
}

type taskDirHook struct {
	runner *TaskRunner
	logger log.Logger
}

func newTaskDirHook(runner *TaskRunner, logger log.Logger) *taskDirHook {
	td := &taskDirHook{
		runner: runner,
	}
	td.logger = logger.Named(td.Name())
	return td
}

func (h *taskDirHook) Name() string {
	return "task_dir"
}

func (h *taskDirHook) Prerun(req *interfaces.TaskPrerunRequest, resp *interfaces.TaskPrerunResponse) error {
	cc := h.runner.clientConfig
	chroot := cconfig.DefaultChrootEnv
	if len(cc.ChrootEnv) > 0 {
		chroot = cc.ChrootEnv
	}

	// Emit the event that we are going to be building the task directory
	h.runner.SetState("", structs.NewTaskEvent(structs.TaskSetup).SetMessage(structs.TaskBuildingTaskDir))

	// Build the task directory structure
	fsi := h.runner.driver.FSIsolation()
	err := h.runner.taskDir.Build(false, chroot, fsi)
	if err != nil {
		return err
	}

	// Update the environment variables based on the built task directory
	driver.SetEnvvars(h.runner.envBuilder, fsi, h.runner.taskDir, h.runner.clientConfig)
	resp.Done = true
	return nil
}

type EventEmitter interface {
	SetState(state string, event *structs.TaskEvent)
}

// artifactHook downloads artifacts for a task.
type artifactHook struct {
	eventEmitter EventEmitter
	logger       log.Logger
}

func newArtifactHook(e EventEmitter, logger log.Logger) *artifactHook {
	h := &artifactHook{
		eventEmitter: e,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*artifactHook) Name() string {
	return "artifacts"
}

func (h *artifactHook) Prerun(req *interfaces.TaskPrerunRequest, resp *interfaces.TaskPrerunResponse) error {
	h.eventEmitter.SetState(structs.TaskStatePending, structs.NewTaskEvent(structs.TaskDownloadingArtifacts))

	for _, artifact := range req.Task.Artifacts {
		if err := getter.GetArtifact(req.TaskEnv, artifact, req.TaskDir); err != nil {
			wrapped := fmt.Errorf("failed to download artifact %q: %v", artifact.GetterSource, err)
			h.logger.Debug(wrapped.Error())
			h.eventEmitter.SetState(structs.TaskStatePending,
				structs.NewTaskEvent(structs.TaskArtifactDownloadFailed).SetDownloadError(wrapped))
			return wrapped
		}
	}

	resp.Done = true
	return nil
}

/*
TR Hooks:

> @schmichael
Task Validate:
Require:  Client config, task definiton
Return: error
Implement: Prestart

> DONE
Task Dir Build:
Requires: Folder structure, driver isolation, client config
Return env, error
Implement: Prestart

> @alex
Vault: Task, RPC to talk to server to derive token, Node SecretID
Return vault token (Call a setter), error, env
Implement: Prestart

> @alex
Consul Template:
Require: Task, alloc directory, way to signal/restart task, updates when vault token changes
Return env, error
Implement: Prestart and Update (for new Vault token) and Destroy

> @schmichael
Consul Service Reg:
Require: Task, interpolation/ENV
Return: error
Implement: Postrun, Update, Prestop

> @alex
Dispatch Payload:
Require: Alloc
Return error
Implement: Prerun

> @schmichael
Artifacts:
Require: Folder structure, task, interpolation/ENV
Return: error
Implement: Prerun and Destroy
*/
