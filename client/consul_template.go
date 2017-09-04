package client

import (
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
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// consulTemplateSourceName is the source name when using the TaskHooks.
	consulTemplateSourceName = "Template"

	// hostSrcOption is the Client option that determines whether the template
	// source may be from the host
	hostSrcOption = "template.allow_host_source"

	// missingDepEventLimit is the number of missing dependencies that will be
	// logged before we switch to showing just the number of missing
	// dependencies.
	missingDepEventLimit = 3

	// DefaultMaxTemplateEventRate is the default maximum rate at which a
	// template event should be fired.
	DefaultMaxTemplateEventRate = 3 * time.Second
)

var (
	// testRetryRate is used to speed up tests by setting consul-templates retry
	// rate to something low
	testRetryRate time.Duration = 0
)

// TaskHooks is an interface which provides hooks into the tasks life-cycle
type TaskHooks interface {
	// Restart is used to restart the task
	Restart(source, reason string)

	// Signal is used to signal the task
	Signal(source, reason string, s os.Signal) error

	// UnblockStart is used to unblock the starting of the task. This should be
	// called after prestart work is completed
	UnblockStart(source string)

	// Kill is used to kill the task because of the passed error. If fail is set
	// to true, the task is marked as failed
	Kill(source, reason string, fail bool)

	// EmitEvent is used to emit an event to be stored in the tasks events.
	EmitEvent(source, message string)
}

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
	// Hooks is used to interact with the task the template manager is being run
	// for
	Hooks TaskHooks

	// Templates is the set of templates we are managing
	Templates []*structs.Template

	// ClientConfig is the Nomad Client configuration
	ClientConfig *config.Config

	// VaultToken is the Vault token for the task.
	VaultToken string

	// TaskDir is the task's directory
	TaskDir string

	// EnvBuilder is the environment variable builder for the task.
	EnvBuilder *env.Builder

	// MaxTemplateEventRate is the maximum rate at which we should emit events.
	MaxTemplateEventRate time.Duration

	// retryRate is only used for testing and is used to increase the retry rate
	retryRate time.Duration
}

// Validate validates the configuration.
func (c *TaskTemplateManagerConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("Nil config passed")
	} else if c.Hooks == nil {
		return fmt.Errorf("Invalid task hooks given")
	} else if c.ClientConfig == nil {
		return fmt.Errorf("Invalid client config given")
	} else if c.TaskDir == "" {
		return fmt.Errorf("Invalid task directory given")
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
		tm.config.Hooks.UnblockStart(consulTemplateSourceName)
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
	envMap, err := loadTemplateEnv(tm.config.Templates, tm.config.TaskDir)
	if err != nil {
		tm.config.Hooks.Kill(consulTemplateSourceName, err.Error(), true)
		return
	}
	tm.config.EnvBuilder.SetTemplateEnv(envMap)

	// Unblock the task
	tm.config.Hooks.UnblockStart(consulTemplateSourceName)

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

			tm.config.Hooks.Kill(consulTemplateSourceName, err.Error(), true)
		case <-tm.runner.TemplateRenderedCh():
			// A template has been rendered, figure out what to do
			events := tm.runner.RenderEvents()

			// Not all templates have been rendered yet
			if len(events) < len(tm.lookup) {
				continue
			}

			for _, event := range events {
				// This template hasn't been rendered
				if event.LastWouldRender.IsZero() {
					continue WAIT
				}
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
			tm.config.Hooks.EmitEvent(consulTemplateSourceName, fmt.Sprintf("Missing: %s", missingStr))
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

			tm.config.Hooks.Kill(consulTemplateSourceName, err.Error(), true)
		case <-tm.runner.TemplateRenderedCh():
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
					tm.config.Hooks.Kill(consulTemplateSourceName, fmt.Sprintf("template runner returned unknown template id %q", id), true)
					return
				}

				// Read environment variables from templates
				envMap, err := loadTemplateEnv(tmpls, tm.config.TaskDir)
				if err != nil {
					tm.config.Hooks.Kill(consulTemplateSourceName, err.Error(), true)
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
					tm.config.Hooks.Restart(consulTemplateSourceName, "template with change_mode restart re-rendered")
				} else if len(signals) != 0 {
					var mErr multierror.Error
					for signal := range signals {
						err := tm.config.Hooks.Signal(consulTemplateSourceName, "template re-rendered", tm.signals[signal])
						if err != nil {
							multierror.Append(&mErr, err)
						}
					}

					if err := mErr.ErrorOrNil(); err != nil {
						flat := make([]os.Signal, 0, len(signals))
						for signal := range signals {
							flat = append(flat, tm.signals[signal])
						}
						tm.config.Hooks.Kill(consulTemplateSourceName, fmt.Sprintf("Sending signals %v failed: %v", flat, err), true)
					}
				}
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

	runner, err := manager.NewRunner(runnerConfig, false, false)
	if err != nil {
		return nil, nil, err
	}

	// Set Nomad's environment variables
	runner.Env = config.EnvBuilder.Build().All()

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

// parseTemplateConfigs converts the tasks templates in the config into
// consul-templates
func parseTemplateConfigs(config *TaskTemplateManagerConfig) (map[ctconf.TemplateConfig]*structs.Template, error) {
	allowAbs := config.ClientConfig.ReadBoolDefault(hostSrcOption, true)
	taskEnv := config.EnvBuilder.Build()

	ctmpls := make(map[ctconf.TemplateConfig]*structs.Template, len(config.Templates))
	for _, tmpl := range config.Templates {
		var src, dest string
		if tmpl.SourcePath != "" {
			if filepath.IsAbs(tmpl.SourcePath) {
				if !allowAbs {
					return nil, fmt.Errorf("Specifying absolute template paths disallowed by client config: %q", tmpl.SourcePath)
				}

				src = tmpl.SourcePath
			} else {
				src = filepath.Join(config.TaskDir, taskEnv.ReplaceEnv(tmpl.SourcePath))
			}
		}
		if tmpl.DestPath != "" {
			dest = filepath.Join(config.TaskDir, taskEnv.ReplaceEnv(tmpl.DestPath))
		}

		ct := ctconf.DefaultTemplateConfig()
		ct.Source = &src
		ct.Destination = &dest
		ct.Contents = &tmpl.EmbeddedTmpl
		ct.LeftDelim = &tmpl.LeftDelim
		ct.RightDelim = &tmpl.RightDelim

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

		ctmpls[*ct] = tmpl
	}

	return ctmpls, nil
}

// newRunnerConfig returns a consul-template runner configuration, setting the
// Vault and Consul configurations based on the clients configs.
func newRunnerConfig(config *TaskTemplateManagerConfig,
	templateMapping map[ctconf.TemplateConfig]*structs.Template) (*ctconf.Config, error) {

	cc := config.ClientConfig
	conf := ctconf.DefaultConfig()

	// Gather the consul-template tempates
	flat := ctconf.TemplateConfigs(make([]*ctconf.TemplateConfig, 0, len(templateMapping)))
	for ctmpl := range templateMapping {
		local := ctmpl
		flat = append(flat, &local)
	}
	conf.Templates = &flat

	// Go through the templates and determine the minimum Vault grace
	vaultGrace := time.Duration(-1)
	for _, tmpl := range templateMapping {
		// Initial condition
		if vaultGrace < 0 {
			vaultGrace = tmpl.VaultGrace
		} else if tmpl.VaultGrace < vaultGrace {
			vaultGrace = tmpl.VaultGrace
		}
	}

	// Force faster retries
	if config.retryRate != 0 {
		rate := config.retryRate
		conf.Consul.Retry.Backoff = &rate
	}

	// Setup the Consul config
	if cc.ConsulConfig != nil {
		conf.Consul.Address = &cc.ConsulConfig.Addr
		conf.Consul.Token = &cc.ConsulConfig.Token

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
		conf.Vault.Grace = helper.TimeToPtr(vaultGrace)

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
func loadTemplateEnv(tmpls []*structs.Template, taskDir string) (map[string]string, error) {
	all := make(map[string]string, 50)
	for _, t := range tmpls {
		if !t.Envvars {
			continue
		}
		f, err := os.Open(filepath.Join(taskDir, t.DestPath))
		if err != nil {
			return nil, fmt.Errorf("error opening env template: %v", err)
		}
		defer f.Close()

		// Parse environment fil
		vars, err := envparse.Parse(f)
		if err != nil {
			return nil, fmt.Errorf("error parsing env template %q: %v", t.DestPath, err)
		}
		for k, v := range vars {
			all[k] = v
		}
	}
	return all, nil
}
