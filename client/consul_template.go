package client

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
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
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// hostSrcOption is the Client option that determines whether the template
	// source may be from the host
	hostSrcOption = "template.allow_host_source"
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
}

// TaskTemplateManager is used to run a set of templates for a given task
type TaskTemplateManager struct {
	// templates is the set of templates we are managing
	templates []*structs.Template

	// lookup allows looking up the set of Nomad templates by their consul-template ID
	lookup map[string][]*structs.Template

	// hooks is used to signal/restart the task as templates are rendered
	hook TaskHooks

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

func NewTaskTemplateManager(hook TaskHooks, tmpls []*structs.Template,
	config *config.Config, vaultToken, taskDir string,
	envBuilder *env.Builder) (*TaskTemplateManager, error) {

	// Check pre-conditions
	if hook == nil {
		return nil, fmt.Errorf("Invalid task hook given")
	} else if config == nil {
		return nil, fmt.Errorf("Invalid config given")
	} else if taskDir == "" {
		return nil, fmt.Errorf("Invalid task directory given")
	} else if envBuilder == nil {
		return nil, fmt.Errorf("Invalid task environment given")
	}

	tm := &TaskTemplateManager{
		templates:  tmpls,
		hook:       hook,
		shutdownCh: make(chan struct{}),
	}

	// Parse the signals that we need
	for _, tmpl := range tmpls {
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
	runner, lookup, err := templateRunner(tmpls, config, vaultToken, taskDir, envBuilder.Build())
	if err != nil {
		return nil, err
	}
	tm.runner = runner
	tm.lookup = lookup

	go tm.run(envBuilder, taskDir)
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
func (tm *TaskTemplateManager) run(envBuilder *env.Builder, taskDir string) {
	// Runner is nil if there is no templates
	if tm.runner == nil {
		// Unblock the start if there is nothing to do
		tm.hook.UnblockStart("consul-template")
		return
	}

	// Start the runner
	go tm.runner.Start()

	// Track when they have all been rendered so we don't signal the task for
	// any render event before hand
	var allRenderedTime time.Time

	// Handle the first rendering
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

			tm.hook.Kill("consul-template", err.Error(), true)
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
		}
	}

	// Read environment variables from env templates
	envMap, err := loadTemplateEnv(tm.templates, taskDir)
	if err != nil {
		tm.hook.Kill("consul-template", err.Error(), true)
		return
	}
	envBuilder.SetTemplateEnv(envMap)

	allRenderedTime = time.Now()
	tm.hook.UnblockStart("consul-template")

	// If all our templates are change mode no-op, then we can exit here
	if tm.allTemplatesNoop() {
		return
	}

	// A lookup for the last time the template was handled
	numTemplates := len(tm.templates)
	handledRenders := make(map[string]time.Time, numTemplates)

	for {
		select {
		case <-tm.shutdownCh:
			return
		case err, ok := <-tm.runner.ErrCh:
			if !ok {
				continue
			}

			tm.hook.Kill("consul-template", err.Error(), true)
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
					tm.hook.Kill("consul-template", fmt.Sprintf("consul-template runner returned unknown template id %q", id), true)
					return
				}

				// Read environment variables from templates
				envMap, err := loadTemplateEnv(tmpls, taskDir)
				if err != nil {
					tm.hook.Kill("consul-template", err.Error(), true)
					return
				}
				envBuilder.SetTemplateEnv(envMap)

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
					tm.hook.Restart("consul-template", "template with change_mode restart re-rendered")
				} else if len(signals) != 0 {
					var mErr multierror.Error
					for signal := range signals {
						err := tm.hook.Signal("consul-template", "template re-rendered", tm.signals[signal])
						if err != nil {
							multierror.Append(&mErr, err)
						}
					}

					if err := mErr.ErrorOrNil(); err != nil {
						flat := make([]os.Signal, 0, len(signals))
						for signal := range signals {
							flat = append(flat, tm.signals[signal])
						}
						tm.hook.Kill("consul-template", fmt.Sprintf("Sending signals %v failed: %v", flat, err), true)
					}
				}
			}
		}
	}
}

// allTemplatesNoop returns whether all the managed templates have change mode noop.
func (tm *TaskTemplateManager) allTemplatesNoop() bool {
	for _, tmpl := range tm.templates {
		if tmpl.ChangeMode != structs.TemplateChangeModeNoop {
			return false
		}
	}

	return true
}

// templateRunner returns a consul-template runner for the given templates and a
// lookup by destination to the template. If no templates are given, a nil
// template runner and lookup is returned.
func templateRunner(tmpls []*structs.Template, config *config.Config,
	vaultToken, taskDir string, taskEnv *env.TaskEnv) (
	*manager.Runner, map[string][]*structs.Template, error) {

	if len(tmpls) == 0 {
		return nil, nil, nil
	}

	runnerConfig, err := runnerConfig(config, vaultToken)
	if err != nil {
		return nil, nil, err
	}

	// Parse the templates
	allowAbs := config.ReadBoolDefault(hostSrcOption, true)
	ctmplMapping, err := parseTemplateConfigs(tmpls, taskDir, taskEnv, allowAbs)
	if err != nil {
		return nil, nil, err
	}

	// Set the config
	flat := ctconf.TemplateConfigs(make([]*ctconf.TemplateConfig, 0, len(ctmplMapping)))
	for ctmpl := range ctmplMapping {
		local := ctmpl
		flat = append(flat, &local)
	}
	runnerConfig.Templates = &flat

	runner, err := manager.NewRunner(runnerConfig, false, false)
	if err != nil {
		return nil, nil, err
	}

	// Set Nomad's environment variables
	runner.Env = taskEnv.All()

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

// parseTemplateConfigs converts the tasks templates into consul-templates
func parseTemplateConfigs(tmpls []*structs.Template, taskDir string,
	taskEnv *env.TaskEnv, allowAbs bool) (map[ctconf.TemplateConfig]*structs.Template, error) {

	ctmpls := make(map[ctconf.TemplateConfig]*structs.Template, len(tmpls))
	for _, tmpl := range tmpls {
		var src, dest string
		if tmpl.SourcePath != "" {
			if filepath.IsAbs(tmpl.SourcePath) {
				if !allowAbs {
					return nil, fmt.Errorf("Specifying absolute template paths disallowed by client config: %q", tmpl.SourcePath)
				}

				src = tmpl.SourcePath
			} else {
				src = filepath.Join(taskDir, taskEnv.ReplaceEnv(tmpl.SourcePath))
			}
		}
		if tmpl.DestPath != "" {
			dest = filepath.Join(taskDir, taskEnv.ReplaceEnv(tmpl.DestPath))
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

// runnerConfig returns a consul-template runner configuration, setting the
// Vault and Consul configurations based on the clients configs.
func runnerConfig(config *config.Config, vaultToken string) (*ctconf.Config, error) {
	conf := ctconf.DefaultConfig()

	t, f := true, false

	// Force faster retries
	if testRetryRate != 0 {
		rate := testRetryRate
		conf.Consul.Retry.Backoff = &rate
	}

	// Setup the Consul config
	if config.ConsulConfig != nil {
		conf.Consul.Address = &config.ConsulConfig.Addr
		conf.Consul.Token = &config.ConsulConfig.Token

		if config.ConsulConfig.EnableSSL != nil && *config.ConsulConfig.EnableSSL {
			verify := config.ConsulConfig.VerifySSL != nil && *config.ConsulConfig.VerifySSL
			conf.Consul.SSL = &ctconf.SSLConfig{
				Enabled: &t,
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
				Enabled:  &t,
				Username: &parts[0],
				Password: &parts[1],
			}
		}
	}

	// Setup the Vault config
	// Always set these to ensure nothing is picked up from the environment
	emptyStr := ""
	conf.Vault.RenewToken = &f
	conf.Vault.Token = &emptyStr
	if config.VaultConfig != nil && config.VaultConfig.IsEnabled() {
		conf.Vault.Address = &config.VaultConfig.Addr
		conf.Vault.Token = &vaultToken

		if strings.HasPrefix(config.VaultConfig.Addr, "https") || config.VaultConfig.TLSCertFile != "" {
			skipVerify := config.VaultConfig.TLSSkipVerify != nil && *config.VaultConfig.TLSSkipVerify
			verify := !skipVerify
			conf.Vault.SSL = &ctconf.SSLConfig{
				Enabled:    &t,
				Verify:     &verify,
				Cert:       &config.VaultConfig.TLSCertFile,
				Key:        &config.VaultConfig.TLSKeyFile,
				CaCert:     &config.VaultConfig.TLSCaFile,
				CaPath:     &config.VaultConfig.TLSCaPath,
				ServerName: &config.VaultConfig.TLSServerName,
			}
		} else {
			conf.Vault.SSL = &ctconf.SSLConfig{
				Enabled:    &f,
				Verify:     &f,
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
