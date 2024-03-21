// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package template

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	ctconf "github.com/hashicorp/consul-template/config"
	"github.com/hashicorp/consul-template/manager"
	"github.com/hashicorp/consul-template/renderer"
	"github.com/hashicorp/consul-template/signals"
	envparse "github.com/hashicorp/go-envparse"
	"github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	trenderer "github.com/hashicorp/nomad/client/allocrunner/taskrunner/template/renderer"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/subproc"
	"github.com/hashicorp/nomad/nomad/structs"
	structsc "github.com/hashicorp/nomad/nomad/structs/config"
)

const (
	// consulTemplateSourceName is the source name when using the TaskHooks.
	consulTemplateSourceName = "Template"

	// missingDepEventLimit is the number of missing dependencies that will be
	// logged before we switch to showing just the number of missing
	// dependencies.
	missingDepEventLimit = 3

	// DefaultMaxTemplateEventRate is the default maximum rate at which a
	// template event should be fired.
	DefaultMaxTemplateEventRate = 3 * time.Second
)

var (
	sourceEscapesErr = errors.New("template source path escapes alloc directory")
	destEscapesErr   = errors.New("template destination path escapes alloc directory")
)

// TaskTemplateManager is used to run a set of templates for a given task
type TaskTemplateManager struct {
	// config holds the template managers configuration
	config *TaskTemplateManagerConfig

	// lookup allows looking up the set of Nomad templates by their consul-template ID
	lookup map[string][]*structs.Template

	// runner is the consul-template runner
	runner *manager.Runner

	// handle is used to execute scripts
	handle     interfaces.ScriptExecutor
	handleLock sync.Mutex

	// signals is a lookup map from the string representation of a signal to its
	// actual signal
	signals map[string]os.Signal

	// shutdownCh is used to signal and started goroutine to shutdown
	shutdownCh chan struct{}

	// shutdown marks whether the manager has been shutdown
	shutdown     bool
	shutdownLock sync.Mutex
}

// TaskTemplateManagerConfig is used to configure an instance of the
// TaskTemplateManager
type TaskTemplateManagerConfig struct {
	// UnblockCh is closed when the template has been rendered
	UnblockCh chan struct{}

	// Lifecycle is used to interact with the task the template manager is being
	// run for
	Lifecycle interfaces.TaskLifecycle

	// Events is used to emit events for the task
	Events interfaces.EventEmitter

	// Templates is the set of templates we are managing
	Templates []*structs.Template

	// ClientConfig is the Nomad Client configuration
	ClientConfig *config.Config

	// ConsulNamespace is the Consul namespace for the task
	ConsulNamespace string

	// ConsulToken is the Consul ACL token fetched by consul_hook using
	// workload identity
	ConsulToken string

	// ConsulConfig is the Consul configuration to use for this template. It may
	// be nil if Nomad has no Consul cofiguration
	ConsulConfig *structsc.ConsulConfig

	// VaultToken is the Vault token for the task.
	VaultToken string

	// VaultConfig is the Vault configuration to use for this template. It may
	// be nil if the task does not use Vault.
	VaultConfig *structsc.VaultConfig

	// VaultNamespace is the Vault namespace for the task
	VaultNamespace string

	// TaskDir is the task's directory
	TaskDir string

	// EnvBuilder is the environment variable builder for the task.
	EnvBuilder *taskenv.Builder

	// MaxTemplateEventRate is the maximum rate at which we should emit events.
	MaxTemplateEventRate time.Duration

	// NomadNamespace is the Nomad namespace for the task
	NomadNamespace string

	// NomadToken is the Nomad token or identity claim for the task
	NomadToken string

	// TaskID is a unique identifier for this task's template manager, for use
	// in downstream platform-specific template runner consumers
	TaskID string

	Logger hclog.Logger
}

// Validate validates the configuration.
func (c *TaskTemplateManagerConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("Nil config passed")
	} else if c.UnblockCh == nil {
		return fmt.Errorf("Invalid unblock channel given")
	} else if c.Lifecycle == nil {
		return fmt.Errorf("Invalid lifecycle hooks given")
	} else if c.Events == nil {
		return fmt.Errorf("Invalid event hook given")
	} else if c.ClientConfig == nil {
		return fmt.Errorf("Invalid client config given")
	} else if c.TaskDir == "" {
		return fmt.Errorf("Invalid task directory given: %q", c.TaskDir)
	} else if c.EnvBuilder == nil {
		return fmt.Errorf("Invalid task environment given")
	} else if c.MaxTemplateEventRate == 0 {
		return fmt.Errorf("Invalid max template event rate given")
	}

	return nil
}

func NewTaskTemplateManager(config *TaskTemplateManagerConfig) (*TaskTemplateManager, error) {
	// Check pre-conditions
	if err := config.Validate(); err != nil {
		return nil, err
	}

	tm := &TaskTemplateManager{
		config:     config,
		shutdownCh: make(chan struct{}),
	}

	// Parse the signals that we need
	for _, tmpl := range config.Templates {
		if tmpl.ChangeSignal == "" {
			continue
		}

		sig, err := signals.Parse(tmpl.ChangeSignal)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse signal %q", tmpl.ChangeSignal)
		}

		if tm.signals == nil {
			tm.signals = make(map[string]os.Signal)
		}

		tm.signals[tmpl.ChangeSignal] = sig
	}

	// the platform sandbox needs to be created before we construct the runner
	// so that reading the template is sandboxed
	err := createPlatformSandbox(config)
	if err != nil {
		return nil, err
	}

	// Build the consul-template runner
	runner, lookup, err := templateRunner(config)
	if err != nil {
		return nil, err
	}
	tm.runner = runner
	tm.lookup = lookup

	go tm.run()
	return tm, nil
}

// Stop is used to stop the consul-template runner
func (tm *TaskTemplateManager) Stop() {
	tm.shutdownLock.Lock()
	defer tm.shutdownLock.Unlock()

	if tm.shutdown {
		return
	}

	close(tm.shutdownCh)
	tm.shutdown = true

	// Stop the consul-template runner
	if tm.runner != nil {
		tm.runner.Stop()
	}

	destroyPlatformSandbox(tm.config)
}

// SetDriverHandle sets the executor
func (tm *TaskTemplateManager) SetDriverHandle(executor interfaces.ScriptExecutor) {
	tm.handleLock.Lock()
	defer tm.handleLock.Unlock()
	tm.handle = executor

}

// run is the long lived loop that handles errors and templates being rendered
func (tm *TaskTemplateManager) run() {
	// Runner is nil if there are no templates
	if tm.runner == nil {
		// Unblock the start if there is nothing to do
		close(tm.config.UnblockCh)
		return
	}

	// Start the runner. We don't defer a call to tm.runner.Stop here so that
	// the runner can keep dynamic secrets alive during the task's
	// kill_timeout. We stop the runner in the Stop hook, which is guaranteed to
	// be called during task kill.
	go tm.runner.Start()

	// Block till all the templates have been rendered or until an error has
	// triggered taskrunner Kill, which closes tm.shutdownCh before we return
	tm.handleFirstRender()

	// Detect if there was a shutdown.
	select {
	case <-tm.shutdownCh:
		return
	default:
	}

	// Read environment variables from env templates before we unblock
	envMap, err := loadTemplateEnv(tm.config.Templates, tm.config.EnvBuilder.Build())
	if err != nil {
		tm.config.Lifecycle.Kill(context.Background(),
			structs.NewTaskEvent(structs.TaskKilling).
				SetFailsTask().
				SetDisplayMessage(fmt.Sprintf("Template failed to read environment variables: %v", err)))
		return
	}
	tm.config.EnvBuilder.SetTemplateEnv(envMap)

	// Unblock the task
	close(tm.config.UnblockCh)

	// If all our templates are change mode no-op, then we can exit here
	if tm.allTemplatesNoop() {
		return
	}

	// handle all subsequent render events.
	tm.handleTemplateRerenders(time.Now())
}

// handleFirstRender blocks till all templates have been rendered
func (tm *TaskTemplateManager) handleFirstRender() {
	// missingDependencies is the set of missing dependencies.
	var missingDependencies map[string]struct{}

	// eventTimer is used to trigger the firing of an event showing the missing
	// dependencies.
	eventTimer := time.NewTimer(tm.config.MaxTemplateEventRate)
	if !eventTimer.Stop() {
		<-eventTimer.C
	}

	// outstandingEvent tracks whether there is an outstanding event that should
	// be fired.
	outstandingEvent := false

	// Wait till all the templates have been rendered
WAIT:
	for {
		select {
		case <-tm.shutdownCh:
			return
		case err, ok := <-tm.runner.ErrCh:
			if !ok {
				continue
			}

			// we don't return here so that we wait for tm.shutdownCh in the
			// next pass thru the loop; this ensures the callers doesn't unblock
			// prematurely
			tm.config.Lifecycle.Kill(context.Background(),
				structs.NewTaskEvent(structs.TaskKilling).
					SetFailsTask().
					SetDisplayMessage(fmt.Sprintf("Template failed: %v", err)))
		case <-tm.runner.TemplateRenderedCh():
			// A template has been rendered, figure out what to do
			events := tm.runner.RenderEvents()

			// Not all templates have been rendered yet
			if len(events) < len(tm.lookup) {
				continue
			}

			dirty := false
			for _, event := range events {
				// This template hasn't been rendered
				if event.LastWouldRender.IsZero() {
					continue WAIT
				}
				if event.WouldRender && event.DidRender {
					dirty = true
				}
			}

			// if there's a driver handle then the task is already running and
			// that changes how we want to behave on first render
			if dirty && tm.config.Lifecycle.IsRunning() {
				handledRenders := make(map[string]time.Time, len(tm.config.Templates))
				tm.onTemplateRendered(handledRenders, time.Time{})
			}

			break WAIT
		case <-tm.runner.RenderEventCh():
			events := tm.runner.RenderEvents()
			joinedSet := make(map[string]struct{})
			for _, event := range events {
				missing := event.MissingDeps
				if missing == nil {
					continue
				}

				for _, dep := range missing.List() {
					joinedSet[dep.String()] = struct{}{}
				}
			}

			// Check to see if the new joined set is the same as the old
			different := len(joinedSet) != len(missingDependencies)
			if !different {
				for k := range joinedSet {
					if _, ok := missingDependencies[k]; !ok {
						different = true
						break
					}
				}
			}

			// Nothing to do
			if !different {
				continue
			}

			// Update the missing set
			missingDependencies = joinedSet

			// Update the event timer channel
			if !outstandingEvent {
				// We got new data so reset
				outstandingEvent = true
				eventTimer.Reset(tm.config.MaxTemplateEventRate)
			}
		case <-eventTimer.C:
			if missingDependencies == nil {
				continue
			}

			// Clear the outstanding event
			outstandingEvent = false

			// Build the missing set
			missingSlice := make([]string, 0, len(missingDependencies))
			for k := range missingDependencies {
				missingSlice = append(missingSlice, k)
			}
			sort.Strings(missingSlice)

			if l := len(missingSlice); l > missingDepEventLimit {
				missingSlice[missingDepEventLimit] = fmt.Sprintf("and %d more", l-missingDepEventLimit)
				missingSlice = missingSlice[:missingDepEventLimit+1]
			}

			missingStr := strings.Join(missingSlice, ", ")
			tm.config.Events.EmitEvent(structs.NewTaskEvent(consulTemplateSourceName).SetDisplayMessage(fmt.Sprintf("Missing: %s", missingStr)))
		}
	}
}

// handleTemplateRerenders is used to handle template render events after they
// have all rendered. It takes action based on which set of templates re-render.
// The passed allRenderedTime is the time at which all templates have rendered.
// This is used to avoid signaling the task for any render event before hand.
func (tm *TaskTemplateManager) handleTemplateRerenders(allRenderedTime time.Time) {
	// A lookup for the last time the template was handled
	handledRenders := make(map[string]time.Time, len(tm.config.Templates))

	for {
		select {
		case <-tm.shutdownCh:
			return
		case err, ok := <-tm.runner.ErrCh:
			if !ok {
				continue
			}

			// we don't return here so that we wait for tm.shutdownCh in the
			// next pass thru the loop; this ensures the callers doesn't unblock
			// prematurely
			tm.config.Lifecycle.Kill(context.Background(),
				structs.NewTaskEvent(structs.TaskKilling).
					SetFailsTask().
					SetDisplayMessage(fmt.Sprintf("Template failed: %v", err)))
		case <-tm.runner.TemplateRenderedCh():
			tm.onTemplateRendered(handledRenders, allRenderedTime)
		}
	}
}

func (tm *TaskTemplateManager) onTemplateRendered(handledRenders map[string]time.Time, allRenderedTime time.Time) {

	var handling []string
	signals := make(map[string]struct{})
	scripts := []*structs.ChangeScript{}
	restart := false
	var splay time.Duration

	events := tm.runner.RenderEvents()
	for id, event := range events {

		// First time through
		if allRenderedTime.After(event.LastDidRender) || allRenderedTime.Equal(event.LastDidRender) {
			handledRenders[id] = allRenderedTime
			continue
		}

		// We have already handled this one
		if htime := handledRenders[id]; htime.After(event.LastDidRender) || htime.Equal(event.LastDidRender) {
			continue
		}

		// Lookup the template and determine what to do
		tmpls, ok := tm.lookup[id]
		if !ok {
			tm.config.Lifecycle.Kill(context.Background(),
				structs.NewTaskEvent(structs.TaskKilling).
					SetFailsTask().
					SetDisplayMessage(fmt.Sprintf("Template runner returned unknown template id %q", id)))
			return
		}

		// Read environment variables from templates
		envMap, err := loadTemplateEnv(tm.config.Templates, tm.config.EnvBuilder.Build())
		if err != nil {
			tm.config.Lifecycle.Kill(context.Background(),
				structs.NewTaskEvent(structs.TaskKilling).
					SetFailsTask().
					SetDisplayMessage(fmt.Sprintf("Template failed to read environment variables: %v", err)))
			return
		}
		tm.config.EnvBuilder.SetTemplateEnv(envMap)

		for _, tmpl := range tmpls {
			switch tmpl.ChangeMode {
			case structs.TemplateChangeModeSignal:
				signals[tmpl.ChangeSignal] = struct{}{}
			case structs.TemplateChangeModeRestart:
				restart = true
			case structs.TemplateChangeModeScript:
				scripts = append(scripts, tmpl.ChangeScript)
			case structs.TemplateChangeModeNoop:
				continue
			}

			if tmpl.Splay > splay {
				splay = tmpl.Splay
			}
		}

		handling = append(handling, id)
	}

	shouldHandle := restart || len(signals) != 0 || len(scripts) != 0
	if !shouldHandle {
		return
	}

	// Apply splay timeout to avoid applying change_mode too frequently.
	if splay != 0 {
		ns := splay.Nanoseconds()
		offset := rand.Int63n(ns)
		t := time.Duration(offset)

		select {
		case <-time.After(t):
		case <-tm.shutdownCh:
			return
		}
	}

	// Update handle time
	for _, id := range handling {
		handledRenders[id] = events[id].LastDidRender
	}

	if restart {
		tm.config.Lifecycle.Restart(context.Background(),
			structs.NewTaskEvent(structs.TaskRestartSignal).
				SetDisplayMessage("Template with change_mode restart re-rendered"), false)
	} else {
		// Handle signals and scripts since the task may have multiple
		// templates with mixed change_mode values.
		tm.handleChangeModeSignal(signals)
		tm.handleChangeModeScript(scripts)
	}
}

func (tm *TaskTemplateManager) handleChangeModeSignal(signals map[string]struct{}) {
	var mErr multierror.Error
	for signal := range signals {
		s := tm.signals[signal]
		event := structs.NewTaskEvent(structs.TaskSignaling).SetTaskSignal(s).SetDisplayMessage("Template re-rendered")
		if err := tm.config.Lifecycle.Signal(event, signal); err != nil {
			_ = multierror.Append(&mErr, err)
		}
	}

	if err := mErr.ErrorOrNil(); err != nil {
		flat := make([]os.Signal, 0, len(signals))
		for signal := range signals {
			flat = append(flat, tm.signals[signal])
		}

		tm.config.Lifecycle.Kill(context.Background(),
			structs.NewTaskEvent(structs.TaskKilling).
				SetFailsTask().
				SetDisplayMessage(fmt.Sprintf("Template failed to send signals %v: %v", flat, err)))
	}
}

func (tm *TaskTemplateManager) handleChangeModeScript(scripts []*structs.ChangeScript) {
	// process script execution concurrently
	var wg sync.WaitGroup
	for _, script := range scripts {
		wg.Add(1)
		go tm.processScript(script, &wg)
	}
	wg.Wait()
}

// handleScriptError is a helper function that produces a TaskKilling event and
// emits a message
func (tm *TaskTemplateManager) handleScriptError(script *structs.ChangeScript, msg string) {
	ev := structs.NewTaskEvent(structs.TaskHookFailed).SetDisplayMessage(msg)
	tm.config.Events.EmitEvent(ev)

	if script.FailOnError {
		tm.config.Lifecycle.Kill(context.Background(),
			structs.NewTaskEvent(structs.TaskKilling).
				SetFailsTask().
				SetDisplayMessage("Template script failed, task is being killed"))
	}
}

// processScript is used for executing change_mode script and handling errors
func (tm *TaskTemplateManager) processScript(script *structs.ChangeScript, wg *sync.WaitGroup) {
	defer wg.Done()

	if tm.handle == nil {
		failureMsg := fmt.Sprintf(
			"Template failed to run script %v with arguments %v because task driver doesn't support the exec operation",
			script.Command,
			script.Args,
		)
		tm.handleScriptError(script, failureMsg)
		return
	}
	_, exitCode, err := tm.handle.Exec(script.Timeout, script.Command, script.Args)
	if err != nil {
		failureMsg := fmt.Sprintf(
			"Template failed to run script %v with arguments %v on change: %v Exit code: %v",
			script.Command,
			script.Args,
			err,
			exitCode,
		)
		tm.handleScriptError(script, failureMsg)
		return
	}
	if exitCode != 0 {
		failureMsg := fmt.Sprintf(
			"Template ran script %v with arguments %v on change but it exited with code code: %v",
			script.Command,
			script.Args,
			exitCode,
		)
		tm.handleScriptError(script, failureMsg)
		return
	}
	tm.config.Events.EmitEvent(structs.NewTaskEvent(structs.TaskHookMessage).
		SetDisplayMessage(
			fmt.Sprintf(
				"Template successfully ran script %v with arguments: %v. Exit code: %v",
				script.Command,
				script.Args,
				exitCode,
			)))
}

// allTemplatesNoop returns whether all the managed templates have change mode noop.
func (tm *TaskTemplateManager) allTemplatesNoop() bool {
	for _, tmpl := range tm.config.Templates {
		if tmpl.ChangeMode != structs.TemplateChangeModeNoop {
			return false
		}
	}

	return true
}

// templateRunner returns a consul-template runner for the given templates and a
// lookup by destination to the template. If no templates are in the config, a
// nil template runner and lookup is returned.
func templateRunner(config *TaskTemplateManagerConfig) (
	*manager.Runner, map[string][]*structs.Template, error) {

	if len(config.Templates) == 0 {
		return nil, nil, nil
	}

	// Parse the templates
	ctmplMapping, err := parseTemplateConfigs(config)
	if err != nil {
		return nil, nil, err
	}

	// Create the runner configuration.
	runnerConfig, err := newRunnerConfig(config, ctmplMapping)
	if err != nil {
		return nil, nil, err
	}

	runner, err := manager.NewRunner(runnerConfig, false)
	if err != nil {
		return nil, nil, err
	}

	// Set Nomad's environment variables.
	// consul-template falls back to the host process environment if a
	// variable isn't explicitly set in the configuration, so we need
	// to mask the environment out to ensure only the task env vars are
	// available.
	runner.Env = maskProcessEnv(config.EnvBuilder.Build().All())

	// Build the lookup
	idMap := runner.TemplateConfigMapping()
	lookup := make(map[string][]*structs.Template, len(idMap))
	for id, ctmpls := range idMap {
		for _, ctmpl := range ctmpls {
			templates := lookup[id]
			templates = append(templates, ctmplMapping[ctmpl])
			lookup[id] = templates
		}
	}

	return runner, lookup, nil
}

// maskProcessEnv masks away any environment variable not found in task env.
// It manipulates the parameter directly and returns it without copying.
func maskProcessEnv(env map[string]string) map[string]string {
	procEnvs := os.Environ()
	for _, e := range procEnvs {
		ekv := strings.SplitN(e, "=", 2)
		if _, ok := env[ekv[0]]; !ok {
			env[ekv[0]] = ""
		}
	}

	return env
}

// parseTemplateConfigs converts the tasks templates in the config into
// consul-templates
func parseTemplateConfigs(config *TaskTemplateManagerConfig) (map[*ctconf.TemplateConfig]*structs.Template, error) {
	sandboxEnabled := !config.ClientConfig.TemplateConfig.DisableSandbox
	taskEnv := config.EnvBuilder.Build()

	ctmpls := make(map[*ctconf.TemplateConfig]*structs.Template, len(config.Templates))
	for _, tmpl := range config.Templates {
		var src, dest string
		if tmpl.SourcePath != "" {
			var escapes bool
			src, escapes = taskEnv.ClientPath(tmpl.SourcePath, false)
			if escapes && sandboxEnabled {
				return nil, sourceEscapesErr
			}
		}

		if tmpl.DestPath != "" {
			var escapes bool
			dest, escapes = taskEnv.ClientPath(tmpl.DestPath, true)
			if escapes && sandboxEnabled {
				return nil, destEscapesErr
			}
		}

		ct := ctconf.DefaultTemplateConfig()
		ct.Source = &src
		ct.Destination = &dest
		ct.Contents = &tmpl.EmbeddedTmpl
		ct.LeftDelim = &tmpl.LeftDelim
		ct.RightDelim = &tmpl.RightDelim
		ct.ErrMissingKey = &tmpl.ErrMissingKey
		ct.FunctionDenylist = config.ClientConfig.TemplateConfig.FunctionDenylist
		if sandboxEnabled {
			ct.SandboxPath = &config.TaskDir
		}

		if tmpl.Wait != nil {
			if err := tmpl.Wait.Validate(); err != nil {
				return nil, err
			}

			ct.Wait = &ctconf.WaitConfig{
				Enabled: pointer.Of(true),
				Min:     tmpl.Wait.Min,
				Max:     tmpl.Wait.Max,
			}
		}

		// Set the permissions
		if tmpl.Perms != "" {
			v, err := strconv.ParseUint(tmpl.Perms, 8, 12)
			if err != nil {
				return nil, fmt.Errorf("Failed to parse %q as octal: %v", tmpl.Perms, err)
			}
			m := os.FileMode(v)
			ct.Perms = &m
		}
		// Set ownership
		if tmpl.Uid != nil && *tmpl.Uid >= 0 {
			ct.Uid = tmpl.Uid
		}
		if tmpl.Gid != nil && *tmpl.Gid >= 0 {
			ct.Gid = tmpl.Gid
		}

		ct.Finalize()

		ctmpls[ct] = tmpl
	}

	return ctmpls, nil
}

// newRunnerConfig returns a consul-template runner configuration, setting the
// Vault and Consul configurations based on the clients configs.
func newRunnerConfig(config *TaskTemplateManagerConfig,
	templateMapping map[*ctconf.TemplateConfig]*structs.Template) (*ctconf.Config, error) {

	cc := config.ClientConfig
	conf := ctconf.DefaultConfig()

	// Gather the consul-template templates
	flat := ctconf.TemplateConfigs(make([]*ctconf.TemplateConfig, 0, len(templateMapping)))
	for ctmpl := range templateMapping {
		local := ctmpl
		flat = append(flat, local)
	}
	conf.Templates = &flat

	// Set the amount of time to do a blocking query for.
	if cc.TemplateConfig.BlockQueryWaitTime != nil {
		conf.BlockQueryWaitTime = cc.TemplateConfig.BlockQueryWaitTime
	}

	// Set the stale-read threshold to allow queries to be served by followers
	// if the last replicated data is within this bound.
	if cc.TemplateConfig.MaxStale != nil {
		conf.MaxStale = cc.TemplateConfig.MaxStale
	}

	// Set the minimum and maximum amount of time to wait for the cluster to reach
	// a consistent state before rendering a template.
	if cc.TemplateConfig.Wait != nil {
		// If somehow the WaitConfig wasn't set correctly upstream, return an error.
		var err error
		err = cc.TemplateConfig.Wait.Validate()
		if err != nil {
			return nil, err
		}
		conf.Wait, err = cc.TemplateConfig.Wait.ToConsulTemplate()
		if err != nil {
			return nil, err
		}
	}

	// Make sure any template specific configuration set by the job author is within
	// the bounds set by the operator.
	if cc.TemplateConfig.WaitBounds != nil {
		// If somehow the WaitBounds weren't set correctly upstream, return an error.
		err := cc.TemplateConfig.WaitBounds.Validate()
		if err != nil {
			return nil, err
		}

		// Check and override with bounds
		for _, tmpl := range *conf.Templates {
			if tmpl.Wait == nil || !*tmpl.Wait.Enabled {
				continue
			}
			if cc.TemplateConfig.WaitBounds.Min != nil {
				if tmpl.Wait.Min != nil && *tmpl.Wait.Min < *cc.TemplateConfig.WaitBounds.Min {
					tmpl.Wait.Min = &*cc.TemplateConfig.WaitBounds.Min
				}
			}
			if cc.TemplateConfig.WaitBounds.Max != nil {
				if tmpl.Wait.Max != nil && *tmpl.Wait.Max > *cc.TemplateConfig.WaitBounds.Max {
					tmpl.Wait.Max = &*cc.TemplateConfig.WaitBounds.Max
				}
			}
		}
	}

	// Set up the Consul config
	if config.ConsulConfig != nil {
		conf.Consul.Address = &config.ConsulConfig.Addr

		// if we're using WI, use the token from consul_hook
		// NOTE: from Nomad 1.9 on, WI will be the only supported way of
		// getting Consul tokens
		if config.ConsulToken != "" {
			conf.Consul.Token = &config.ConsulToken
		} else {
			conf.Consul.Token = &config.ConsulConfig.Token
		}

		// Get the Consul namespace from agent config. This is the lower level
		// of precedence (beyond default).
		if config.ConsulConfig.Namespace != "" {
			conf.Consul.Namespace = &config.ConsulConfig.Namespace
		}

		if config.ConsulConfig.EnableSSL != nil && *config.ConsulConfig.EnableSSL {
			verify := config.ConsulConfig.VerifySSL != nil && *config.ConsulConfig.VerifySSL
			conf.Consul.SSL = &ctconf.SSLConfig{
				Enabled: pointer.Of(true),
				Verify:  &verify,
				Cert:    &config.ConsulConfig.CertFile,
				Key:     &config.ConsulConfig.KeyFile,
				CaCert:  &config.ConsulConfig.CAFile,
			}
		}

		if config.ConsulConfig.Auth != "" {
			parts := strings.SplitN(config.ConsulConfig.Auth, ":", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("Failed to parse Consul Auth config")
			}

			conf.Consul.Auth = &ctconf.AuthConfig{
				Enabled:  pointer.Of(true),
				Username: &parts[0],
				Password: &parts[1],
			}
		}

		// Set the user-specified Consul RetryConfig
		if cc.TemplateConfig.ConsulRetry != nil {
			var err error
			err = cc.TemplateConfig.ConsulRetry.Validate()
			if err != nil {
				return nil, err
			}
			conf.Consul.Retry, err = cc.TemplateConfig.ConsulRetry.ToConsulTemplate()
			if err != nil {
				return nil, err
			}
		}
	}

	// Get the Consul namespace from job/group config. This is the higher level
	// of precedence if set (above agent config).
	if config.ConsulNamespace != "" {
		conf.Consul.Namespace = &config.ConsulNamespace
	}

	// Set up the Vault config
	// Always set these to ensure nothing is picked up from the environment
	emptyStr := ""
	conf.Vault.RenewToken = pointer.Of(false)
	conf.Vault.Token = &emptyStr
	if config.VaultConfig != nil && config.VaultConfig.IsEnabled() {
		conf.Vault.Address = &config.VaultConfig.Addr
		conf.Vault.Token = &config.VaultToken

		// Set the Vault Namespace. Passed in Task config has
		// highest precedence.
		if config.VaultConfig.Namespace != "" {
			conf.Vault.Namespace = &config.VaultConfig.Namespace
		}
		if config.VaultNamespace != "" {
			conf.Vault.Namespace = &config.VaultNamespace
		}

		if strings.HasPrefix(config.VaultConfig.Addr, "https") || config.VaultConfig.TLSCertFile != "" {
			skipVerify := config.VaultConfig.TLSSkipVerify != nil && *config.VaultConfig.TLSSkipVerify
			verify := !skipVerify
			conf.Vault.SSL = &ctconf.SSLConfig{
				Enabled:    pointer.Of(true),
				Verify:     &verify,
				Cert:       &config.VaultConfig.TLSCertFile,
				Key:        &config.VaultConfig.TLSKeyFile,
				CaCert:     &config.VaultConfig.TLSCaFile,
				CaPath:     &config.VaultConfig.TLSCaPath,
				ServerName: &config.VaultConfig.TLSServerName,
			}
		} else {
			conf.Vault.SSL = &ctconf.SSLConfig{
				Enabled:    pointer.Of(false),
				Verify:     pointer.Of(false),
				Cert:       &emptyStr,
				Key:        &emptyStr,
				CaCert:     &emptyStr,
				CaPath:     &emptyStr,
				ServerName: &emptyStr,
			}
		}

		// Set the user-specified Vault RetryConfig
		if cc.TemplateConfig.VaultRetry != nil {
			var err error
			if err = cc.TemplateConfig.VaultRetry.Validate(); err != nil {
				return nil, err
			}
			conf.Vault.Retry, err = cc.TemplateConfig.VaultRetry.ToConsulTemplate()
			if err != nil {
				return nil, err
			}
		}
	}

	// Set up Nomad
	conf.Nomad.Namespace = &config.NomadNamespace
	conf.Nomad.Transport.CustomDialer = cc.TemplateDialer
	conf.Nomad.Token = &config.NomadToken
	if cc.TemplateConfig != nil && cc.TemplateConfig.NomadRetry != nil {
		// Set the user-specified Nomad RetryConfig
		var err error
		if err = cc.TemplateConfig.NomadRetry.Validate(); err != nil {
			return nil, err
		}
		conf.Nomad.Retry, err = cc.TemplateConfig.NomadRetry.ToConsulTemplate()
		if err != nil {
			return nil, err
		}
	}

	sandboxEnabled := isSandboxEnabled(config)
	sandboxDir := filepath.Dir(config.TaskDir) // alloc working directory
	conf.ReaderFunc = ReaderFn(config.TaskID, sandboxDir, sandboxEnabled)
	conf.RendererFunc = RenderFn(config.TaskID, sandboxDir, sandboxEnabled)
	conf.Finalize()
	return conf, nil
}

func isSandboxEnabled(cfg *TaskTemplateManagerConfig) bool {
	if cfg.ClientConfig != nil && cfg.ClientConfig.TemplateConfig != nil && cfg.ClientConfig.TemplateConfig.DisableSandbox {
		return false
	}
	return true
}

type sandboxConfig struct {
	thisBin     string
	sandboxPath string
	destPath    string
	sourcePath  string
	perms       string
	user        string
	group       string
	taskID      string
	contents    []byte
}

func ReaderFn(taskID, taskDir string, sandboxEnabled bool) func(string) ([]byte, error) {
	if !sandboxEnabled {
		return nil
	}
	thisBin := subproc.Self()

	return func(src string) ([]byte, error) {

		sandboxCfg := &sandboxConfig{
			thisBin:     thisBin,
			sandboxPath: taskDir,
			sourcePath:  src,
			taskID:      taskID,
		}

		stdout, stderr, code, err := readTemplateFromSandbox(sandboxCfg)
		if err != nil && code != 0 {
			return nil, fmt.Errorf("%v: %s", err, string(stderr))
		}

		// this will get wrapped in CT log formatter
		fmt.Fprintf(os.Stderr, "[DEBUG] %s", string(stderr))
		return stdout, nil
	}
}

func RenderFn(taskID, taskDir string, sandboxEnabled bool) func(*renderer.RenderInput) (*renderer.RenderResult, error) {
	if !sandboxEnabled {
		return nil
	}
	thisBin := subproc.Self()

	return func(i *renderer.RenderInput) (*renderer.RenderResult, error) {
		wouldRender := false
		didRender := false

		sandboxCfg := &sandboxConfig{
			thisBin:     thisBin,
			sandboxPath: taskDir,
			destPath:    i.Path,
			perms:       strconv.FormatUint(uint64(i.Perms), 8),
			user:        i.User,
			group:       i.Group,
			taskID:      taskID,
			contents:    i.Contents,
		}

		logs, code, err := renderTemplateInSandbox(sandboxCfg)
		if err != nil {
			if len(logs) > 0 {
				log.Printf("[ERROR] %v: %s", err, logs)
			} else {
				log.Printf("[ERROR] %v", err)
			}
			return &renderer.RenderResult{
				DidRender:   false,
				WouldRender: false,
				Contents:    []byte{},
			}, fmt.Errorf("template render subprocess failed: %w", err)
		}
		if code == trenderer.ExitWouldRenderButDidnt {
			didRender = false
			wouldRender = true
		} else {
			didRender = true
			wouldRender = true
		}

		// the subprocess emits logs matching the consul-template runner, but we
		// CT doesn't support hclog, so we just print the whole output here to
		// stderr the same way CT does so the results look seamless
		if len(logs) > 0 {
			log.Printf("[DEBUG] %s", logs)
		}

		result := &renderer.RenderResult{
			DidRender:   didRender,
			WouldRender: wouldRender,
			Contents:    i.Contents,
		}
		return result, nil
	}
}

// loadTemplateEnv loads task environment variables from all templates.
func loadTemplateEnv(tmpls []*structs.Template, taskEnv *taskenv.TaskEnv) (map[string]string, error) {
	all := make(map[string]string, 50)
	for _, t := range tmpls {
		if !t.Envvars {
			continue
		}

		// we checked escape before we rendered the file
		dest, _ := taskEnv.ClientPath(t.DestPath, true)
		f, err := os.Open(dest)
		if err != nil {
			return nil, fmt.Errorf("error opening env template: %v", err)
		}
		defer f.Close()

		// Parse environment fil
		vars, err := envparse.Parse(f)
		if err != nil {
			return nil, fmt.Errorf("error parsing env template %q: %v", dest, err)
		}
		for k, v := range vars {
			all[k] = v
		}
	}
	return all, nil
}
