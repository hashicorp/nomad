package rotatelogs

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"golang.org/x/sys/unix"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/plugins/base"
	logplugins "github.com/hashicorp/nomad/plugins/logging"
	"github.com/hashicorp/nomad/plugins/logging/driver"
	loglib "github.com/hashicorp/nomad/plugins/logging/lib"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

const pluginName = "rotatelogs"
const fingerprintPeriod = time.Minute

var (
	errDisabledDriver = fmt.Errorf("rotatelogs is disabled")
)

var (
	// PluginID is the docker plugin metadata registered in the plugin catalog.
	PluginID = loader.PluginID{
		Name:       "rotatelogs",
		PluginType: base.PluginTypeLogging,
	}

	PluginConfig = &loader.InternalPluginConfig{
		Config:  map[string]interface{}{},
		Factory: func(ctx context.Context, l hclog.Logger) interface{} { return NewRotateLogs(l) },
	}

	// pluginInfo is the response returned for the PluginInfo RPC
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeLogging,
		PluginApiVersions: []string{logplugins.ApiVersion010},
		PluginVersion:     "0.2.0",
		Name:              "rotatelogs",
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

func NewRotateLogs(logger hclog.Logger) logplugins.LoggingPlugin {
	ctx := context.TODO()
	bin, err := exec.LookPath("rotatelogs")
	if err != nil {
		// TODO: fingerprint this and re-enable if it shows up
		logger.Debug("rotatefiles does not exist on this system: log plugin disabled")
	}

	return &rotateLogsImpl{
		bin:    bin,
		logger: logger,
		driver: driver.NewDriver(ctx, logger, newCommandFactory(bin, logger)),
	}
}

type rotateLogsImpl struct {
	logger hclog.Logger

	bin string

	// config is the plugin configuration set by the SetConfig RPC
	config     *Config
	configLock sync.RWMutex

	driver *driver.Driver
	lock   sync.Mutex
}

// PluginInfo returns the identity of this plugin. This method is required for
// implementing the base Nomad plugin interface.
func (l *rotateLogsImpl) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

// ConfigSchema returns the HCL configuration schema of this plugin. This method
// is required for implementing the base Nomad plugin interface.
func (l *rotateLogsImpl) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

// SetConfig sets the driver's configuration. This method is required to
// implement the base Nomad plugin interface
func (l *rotateLogsImpl) SetConfig(cfg *base.Config) error {
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
func (l *rotateLogsImpl) Fingerprint(ctx context.Context) (<-chan *logplugins.FingerprintResponse, error) {
	fingerprintCh := make(chan *logplugins.FingerprintResponse)
	go l.handleFingerprint(ctx, fingerprintCh)
	return fingerprintCh, nil
}

func (l *rotateLogsImpl) handleFingerprint(ctx context.Context, fingerprintCh chan<- *logplugins.FingerprintResponse) {
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
func (l *rotateLogsImpl) Start(cfg *loglib.LogConfig) error {
	if l.isDisabled() {
		return errDisabledDriver
	}
	return l.driver.Start(cfg)
}

// Stop stops the log shipper task. This method is required to implemnt the
// Nomad logging plugin interface.
func (l *rotateLogsImpl) Stop(cfg *loglib.LogConfig) error {
	if l.isDisabled() {
		return nil
	}
	return l.driver.Stop(cfg)
}

func (l *rotateLogsImpl) isDisabled() bool {
	l.configLock.RLock()
	defer l.configLock.RUnlock()
	if l.config == nil || l.config.Enabled == false || l.bin == "" {
		return true
	}
	return false
}

func newCommandFactory(bin string, logger hclog.Logger) driver.NewCommandFunc {

	return func(h *driver.TaskHandle, config *loglib.LogConfig) (*exec.Cmd, error) {
		args := argsFromConfig(config)
		cmd := exec.Command(bin, args...)

		err := os.MkdirAll(config.LogDir, 0700)
		if err != nil {
			return nil, fmt.Errorf("could not create logdir: %v", err)
		}

		// TODO: we need to collect stdout too! can we configure the task to use
		// the same output based on the log plugin?
		_, err = os.Create(config.StdoutFifo)
		if err != nil {
			return nil, fmt.Errorf("could not open stdout fifo: %v", err)
		}

		// TODO: right now this is exiting immediately and then getting
		// restarted once the task starts. If we use client/lib/fifo which
		// removes the O_NONBLOCK, it just hangs here instead.
		unix.Mkfifo(config.StderrFifo, 0600)
		taskStderr, err := os.OpenFile(config.StderrFifo, unix.O_RDONLY|unix.O_NONBLOCK, fs.ModeNamedPipe)
		if err != nil {
			return nil, fmt.Errorf("could not open stderr fifo: %v", err)
		}
		cmd.Stdin = taskStderr

		// rotatelogs own stdout/stderr should come back up to the logger
		cmd.Stderr = h.TaskLogWriter
		cmd.Stdout = h.TaskLogWriter

		cmd.Env = []string{}
		cmd.Dir = config.LogDir
		return cmd, nil
	}

}

func argsFromConfig(config *loglib.LogConfig) []string {

	if config.MaxFiles < 1 {
		config.MaxFiles = 1
	}
	if config.MaxFileSizeMB < 1 {
		config.MaxFileSizeMB = 1
	}
	maxSize := fmt.Sprintf("%dM", config.MaxFileSizeMB)

	// TODO: we need to be able to provide the fifo as stdin!
	// TODO: we need to be able to provide one stderr/stdout!
	return []string{
		"-f",
		"-n", strconv.Itoa(config.MaxFiles),
		filepath.Join(config.LogDir, config.StderrLogFile+".0"),
		maxSize,
	}
}
