package logmon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/plugins/base"
	logplugins "github.com/hashicorp/nomad/plugins/logging"
	"github.com/hashicorp/nomad/plugins/logging/driver"
	loglib "github.com/hashicorp/nomad/plugins/logging/lib"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

const pluginName = "logmon"
const fingerprintPeriod = time.Minute

var (
	errDisabledDriver = fmt.Errorf("logmon is disabled")
)

var bin = getBin()

func getBin() string {
	b, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return b
}

var (
	// PluginID is the docker plugin metadata registered in the plugin catalog.
	PluginID = loader.PluginID{
		Name:       "logmon",
		PluginType: base.PluginTypeLogging,
	}

	PluginConfig = &loader.InternalPluginConfig{
		Config:  map[string]interface{}{},
		Factory: func(ctx context.Context, l hclog.Logger) interface{} { return NewLogMon(l) },
	}

	// pluginInfo is the response returned for the PluginInfo RPC
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeLogging,
		PluginApiVersions: []string{logplugins.ApiVersion010},
		PluginVersion:     "0.2.0",
		Name:              "logmon",
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
	// Enabled is set to false to disable the raw_exec driver
	Enabled bool `codec:"enabled"`
}

func NewLogMon(logger hclog.Logger) logplugins.LoggingPlugin {
	ctx := context.TODO()
	return &logmonImpl{
		logger: logger,
		driver: driver.NewDriver(ctx, logger, newCommand),
	}
}

type logmonImpl struct {
	logger hclog.Logger

	// config is the plugin configuration set by the SetConfig RPC
	config     *Config
	configLock sync.RWMutex

	driver *driver.Driver
	lock   sync.Mutex
}

// PluginInfo returns the identity of this plugin. This method is required for
// implementing the base Nomad plugin interface.
func (l *logmonImpl) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

// ConfigSchema returns the HCL configuration schema of this plugin. This method
// is required for implementing the base Nomad plugin interface.
func (l *logmonImpl) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

// SetConfig sets the driver's configuration. This method is required to
// implement the base Nomad plugin interface
func (l *logmonImpl) SetConfig(cfg *base.Config) error {
	var config Config
	if len(cfg.PluginConfig) != 0 {
		if err := base.MsgPackDecode(cfg.PluginConfig, &config); err != nil {
			return err
		}
	}

	l.configLock.Lock()
	defer l.configLock.Unlock()
	l.config = &config
	return nil
}

// Fingerprint returns the driver's capabilities. This method is required to
// implemnt the Nomad logging plugin interface.
func (l *logmonImpl) Fingerprint(ctx context.Context) (<-chan *logplugins.FingerprintResponse, error) {
	fingerprintCh := make(chan *logplugins.FingerprintResponse)
	go l.handleFingerprint(ctx, fingerprintCh)
	return fingerprintCh, nil
}

func (l *logmonImpl) handleFingerprint(ctx context.Context, fingerprintCh chan<- *logplugins.FingerprintResponse) {
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
func (l *logmonImpl) Start(cfg *loglib.LogConfig) error {
	if l.isDisabled() {
		return errDisabledDriver
	}
	return l.driver.Start(cfg)
}

// Stop stops the log shipper task. This method is required to implemnt the
// Nomad logging plugin interface.
func (l *logmonImpl) Stop(cfg *loglib.LogConfig) error {
	if l.isDisabled() {
		return nil
	}
	return l.driver.Stop(cfg)
}

func (l *logmonImpl) isDisabled() bool {
	l.configLock.RLock()
	defer l.configLock.RUnlock()
	if l.config == nil || l.config.Enabled == false {
		return true
	}
	return false
}

func newCommand(h *driver.TaskHandle, config *loglib.LogConfig) (*exec.Cmd, error) {
	args := argsFromConfig(config)
	cmd := exec.Command(bin, args...)
	cmd.Stdout = h.TaskLogWriter
	cmd.Stderr = h.TaskLogWriter
	cmd.Env = []string{}
	cmd.Dir = config.LogDir
	return cmd, nil
}

func argsFromConfig(config *loglib.LogConfig) []string {
	return []string{
		"logshipper",
		"-job-id", config.JobID,
		"-alloc-id", config.AllocID,
		"-group-name", config.GroupName,
		"-task-name", config.TaskName,
		"-log-dir", config.LogDir,
		"-stdout-log-file", config.StdoutLogFile,
		"-stderr-log-file", config.StderrLogFile,
		"-stdout-fifo", config.StdoutFifo,
		"-stderr-fifo", config.StderrFifo,
		"-max-files", strconv.Itoa(config.MaxFiles),
		"-max-file-size", strconv.Itoa(config.MaxFileSizeMB),
	}
}
