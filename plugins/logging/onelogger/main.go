package main

import (
	"context"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/plugins/base"
	logplugins "github.com/hashicorp/nomad/plugins/logging"
	loglib "github.com/hashicorp/nomad/plugins/logging/lib"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		JSONFormat: true,
		Name:       pluginName,
	})
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: base.Handshake,
		Plugins: map[string]plugin.Plugin{
			pluginName: logplugins.NewPlugin(NewOneLogger(logger)),
		},
		GRPCServer: plugin.DefaultGRPCServer,
		Logger:     logger,
	})
}

const pluginName = "onelogger"

func NewOneLogger(logger hclog.Logger) logplugins.LoggingPlugin {
	return &OneLogger{
		logger: logger,
		tasks:  make(map[string]*loglib.TaskLogger),
	}
}

type OneLogger struct {
	logger hclog.Logger

	tasks  map[string]*loglib.TaskLogger
	config *Config

	lock sync.RWMutex
}

const fingerprintPeriod = time.Minute

var (
	// PluginID is the logging plugin metadata registered in the plugin catalog.
	PluginID = loader.PluginID{
		Name:       "onelogger",
		PluginType: base.PluginTypeLogging,
	}

	PluginConfig = &loader.InternalPluginConfig{
		Config:  map[string]interface{}{},
		Factory: func(ctx context.Context, l hclog.Logger) interface{} { return NewOneLogger(l) },
	}

	// pluginInfo is the response returned for the PluginInfo RPC
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeLogging,
		PluginApiVersions: []string{logplugins.ApiVersion010},
		PluginVersion:     "0.0.1",
		Name:              pluginName,
	}

	// configSpec is the hcl specification returned by the ConfigSchema RPC
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"enabled": hclspec.NewDefault(
			hclspec.NewAttr("enabled", "bool", false),
			hclspec.NewLiteral("true"),
		),
	})
)

// Config is the driver configuration set by the SetConfig RPC call
type Config struct {
	// Enabled is set to false to disable the log driver
	Enabled bool `codec:"enabled"`
}

// PluginInfo returns the identity of this plugin. This method is required for
// implementing the base Nomad plugin interface.
func (one *OneLogger) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

// ConfigSchema returns the HCL configuration schema of this plugin. This method
// is required for implementing the base Nomad plugin interface.
func (one *OneLogger) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

// SetConfig sets the driver's configuration. This method is required to
// implement the base Nomad plugin interface
func (one *OneLogger) SetConfig(cfg *base.Config) error {
	var config Config
	if len(cfg.PluginConfig) != 0 {
		if err := base.MsgPackDecode(cfg.PluginConfig, &config); err != nil {
			return err
		}
	}

	one.lock.Lock()
	defer one.lock.Unlock()
	one.config = &config
	return nil
}

// Fingerprint returns the driver's capabilities. This method is required to
// implemnt the Nomad logging plugin interface.
func (one *OneLogger) Fingerprint(ctx context.Context) (<-chan *logplugins.FingerprintResponse, error) {
	fingerprintCh := make(chan *logplugins.FingerprintResponse)
	go one.handleFingerprint(ctx, fingerprintCh)
	return fingerprintCh, nil
}

func (one *OneLogger) handleFingerprint(ctx context.Context, fingerprintCh chan<- *logplugins.FingerprintResponse) {
	defer close(fingerprintCh)
	t, cancel := helper.NewSafeTimer(0)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			fingerprintCh <- &logplugins.FingerprintResponse{}
			t.Reset(fingerprintPeriod)

		}
	}
}

// Start launches the log shipper task and sets up the monitoring for it. This
// method is required to implemnt the Nomad logging plugin interface.
func (one *OneLogger) Start(cfg *loglib.LogConfig) error {

	one.lock.Lock()
	defer one.lock.Unlock()

	id := cfg.ID()

	taskLogger := one.tasks[id]
	var err error

	if taskLogger == nil {
		taskLogger, err = loglib.NewTaskLogger(cfg, one.logger)
		if err != nil {
			one.logger.Error(err.Error())
			return err
		}
		one.tasks[id] = taskLogger
	}

	go one.wait(id, taskLogger)
	return nil
}

func (one *OneLogger) wait(id string, taskLogger *loglib.TaskLogger) {
	taskLogger.Wait()
	one.lock.Lock()
	defer one.lock.Unlock()
	delete(one.tasks, id)
}

// Stop stops the log shipper task. This method is required to implemnt the
// Nomad logging plugin interface.
func (one *OneLogger) Stop(cfg *loglib.LogConfig) error {
	one.lock.Lock()
	defer one.lock.Unlock()

	id := cfg.ID()

	taskLogger := one.tasks[id]
	if taskLogger != nil {
		taskLogger.Close()
	}
	taskLogger.Wait()
	delete(one.tasks, id)
	return nil
}
