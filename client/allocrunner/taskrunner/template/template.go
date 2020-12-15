package template

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/hashicorp/consul-template/signals"
	envparse "github.com/hashicorp/go-envparse"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
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

	// VaultToken is the Vault token for the task.
	VaultToken string

	// VaultNamespace is the Vault namespace for the task
	VaultNamespace string

	// TaskDir is the task's directory
	TaskDir string

	// EnvBuilder is the environment variable builder for the task.
	EnvBuilder *taskenv.Builder

	// MaxTemplateEventRate is the maximum rate at which we should emit events.
	MaxTemplateEventRate time.Duration

	// retryRate is only used for testing and is used to increase the retry rate
	retryRate time.Duration
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
}

// run is the long lived loop that handles errors and templates being rendered
func (tm *TaskTemplateManager) run() {
	// Runner is nil if there is no templates
	if tm.runner == nil {
		// Unblock the start if there is nothing to do
		close(tm.config.UnblockCh)
		return
	}

	// Start the runner
	go tm.runner.Start()

	// Block till all the templates have been rendered
	tm.handleFirstRender()

	// Detect if there was a shutdown.
	select {
	case <-tm.shutdownCh:
		return
	default:
	}

	// Read environment variables from env templates before we unblock
	envMap, err := loadTemplateEnv(tm.config.Templates, tm.config.TaskDir, tm.config.EnvBuilder.Build())
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
	// A template has been rendered, figure out what to do
	var handling []string
	signals := make(map[string]struct{})
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
		envMap, err := loadTemplateEnv(tm.config.Templates, tm.config.TaskDir, tm.config.EnvBuilder.Build())
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
			case structs.TemplateChangeModeNoop:
				continue
			}

			if tmpl.Splay > splay {
				splay = tmpl.Splay
			}
		}

		handling = append(handling, id)
	}

	if restart || len(signals) != 0 {
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
		} else if len(signals) != 0 {
			var mErr multierror.Error
			for signal := range signals {
				s := tm.signals[signal]
				event := structs.NewTaskEvent(structs.TaskSignaling).SetTaskSignal(s).SetDisplayMessage("Template re-rendered")
				if err := tm.config.Lifecycle.Signal(event, signal); err != nil {
					multierror.Append(&mErr, err)
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
	}

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

	// Make NOMAD_{ALLOC,TASK,SECRETS}_DIR relative paths to avoid treating
	// them as sandbox escapes when using containers.
	if taskEnv.EnvMap[taskenv.AllocDir] == allocdir.SharedAllocContainerPath {
		taskEnv.EnvMap[taskenv.AllocDir] = allocdir.SharedAllocName
	}
	if taskEnv.EnvMap[taskenv.TaskLocalDir] == allocdir.TaskLocalContainerPath {
		taskEnv.EnvMap[taskenv.TaskLocalDir] = allocdir.TaskLocal
	}
	if taskEnv.EnvMap[taskenv.SecretsDir] == allocdir.TaskSecretsContainerPath {
		taskEnv.EnvMap[taskenv.SecretsDir] = allocdir.TaskSecrets
	}

	ctmpls := make(map[*ctconf.TemplateConfig]*structs.Template, len(config.Templates))
	for _, tmpl := range config.Templates {
		var src, dest string
		if tmpl.SourcePath != "" {
			src = taskEnv.ReplaceEnv(tmpl.SourcePath)
			if !filepath.IsAbs(src) {
				src = filepath.Join(config.TaskDir, src)
			} else {
				src = filepath.Clean(src)
			}
			escapes := helper.PathEscapesSandbox(config.TaskDir, src)
			if escapes && sandboxEnabled {
				return nil, sourceEscapesErr
			}
		}

		if tmpl.DestPath != "" {
			dest = taskEnv.ReplaceEnv(tmpl.DestPath)
			// Note: we *always* join here even if we get passed an absolute
			// path so that $NOMAD_SECRETS_DIR and friends can be used and
			// always fall inside the task working directory
			dest = filepath.Join(config.TaskDir, dest)
			escapes := helper.PathEscapesSandbox(config.TaskDir, dest)
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
		ct.FunctionDenylist = config.ClientConfig.TemplateConfig.FunctionDenylist
		if sandboxEnabled {
			ct.SandboxPath = &config.TaskDir
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

	// Force faster retries
	if config.retryRate != 0 {
		rate := config.retryRate
		conf.Consul.Retry.Backoff = &rate
	}

	// Setup the Consul config
	if cc.ConsulConfig != nil {
		conf.Consul.Address = &cc.ConsulConfig.Addr
		conf.Consul.Token = &cc.ConsulConfig.Token
		conf.Consul.Namespace = &cc.ConsulConfig.Namespace

		if cc.ConsulConfig.EnableSSL != nil && *cc.ConsulConfig.EnableSSL {
			verify := cc.ConsulConfig.VerifySSL != nil && *cc.ConsulConfig.VerifySSL
			conf.Consul.SSL = &ctconf.SSLConfig{
				Enabled: helper.BoolToPtr(true),
				Verify:  &verify,
				Cert:    &cc.ConsulConfig.CertFile,
				Key:     &cc.ConsulConfig.KeyFile,
				CaCert:  &cc.ConsulConfig.CAFile,
			}
		}

		if cc.ConsulConfig.Auth != "" {
			parts := strings.SplitN(cc.ConsulConfig.Auth, ":", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("Failed to parse Consul Auth config")
			}

			conf.Consul.Auth = &ctconf.AuthConfig{
				Enabled:  helper.BoolToPtr(true),
				Username: &parts[0],
				Password: &parts[1],
			}
		}
	}

	// Setup the Vault config
	// Always set these to ensure nothing is picked up from the environment
	emptyStr := ""
	conf.Vault.RenewToken = helper.BoolToPtr(false)
	conf.Vault.Token = &emptyStr
	if cc.VaultConfig != nil && cc.VaultConfig.IsEnabled() {
		conf.Vault.Address = &cc.VaultConfig.Addr
		conf.Vault.Token = &config.VaultToken

		// Set the Vault Namespace. Passed in Task config has
		// highest precedence.
		if config.ClientConfig.VaultConfig.Namespace != "" {
			conf.Vault.Namespace = &config.ClientConfig.VaultConfig.Namespace
		}
		if config.VaultNamespace != "" {
			conf.Vault.Namespace = &config.VaultNamespace
		}

		if strings.HasPrefix(cc.VaultConfig.Addr, "https") || cc.VaultConfig.TLSCertFile != "" {
			skipVerify := cc.VaultConfig.TLSSkipVerify != nil && *cc.VaultConfig.TLSSkipVerify
			verify := !skipVerify
			conf.Vault.SSL = &ctconf.SSLConfig{
				Enabled:    helper.BoolToPtr(true),
				Verify:     &verify,
				Cert:       &cc.VaultConfig.TLSCertFile,
				Key:        &cc.VaultConfig.TLSKeyFile,
				CaCert:     &cc.VaultConfig.TLSCaFile,
				CaPath:     &cc.VaultConfig.TLSCaPath,
				ServerName: &cc.VaultConfig.TLSServerName,
			}
		} else {
			conf.Vault.SSL = &ctconf.SSLConfig{
				Enabled:    helper.BoolToPtr(false),
				Verify:     helper.BoolToPtr(false),
				Cert:       &emptyStr,
				Key:        &emptyStr,
				CaCert:     &emptyStr,
				CaPath:     &emptyStr,
				ServerName: &emptyStr,
			}
		}
	}

	conf.Finalize()
	return conf, nil
}

// loadTemplateEnv loads task environment variables from all templates.
func loadTemplateEnv(tmpls []*structs.Template, taskDir string, taskEnv *taskenv.TaskEnv) (map[string]string, error) {
	all := make(map[string]string, 50)
	for _, t := range tmpls {
		if !t.Envvars {
			continue
		}

		dest := filepath.Join(taskDir, taskEnv.ReplaceEnv(t.DestPath))
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
