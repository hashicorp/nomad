package nvidia

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/device/cmd/nvidia/nvml"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

const (
	// pluginName is the name of the plugin
	pluginName = "nvidia-gpu"

	// vendor is the vendor providing the devices
	vendor = "nvidia"

	// deviceType is the type of device being returned
	deviceType = device.DeviceTypeGPU

	// notAvailable value is returned to nomad server in case some properties were
	// undetected by nvml driver
	notAvailable = "N/A"
)

var (
	// pluginInfo describes the plugin
	pluginInfo = &base.PluginInfoResponse{
		Type:             base.PluginTypeDevice,
		PluginApiVersion: "0.0.1", // XXX This should be an array and should be consts
		PluginVersion:    "0.1.0",
		Name:             pluginName,
	}

	// configSpec is the specification of the plugin's configuration
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"ignored_gpu_ids": hclspec.NewDefault(
			hclspec.NewAttr("ignored_gpu_ids", "list(string)", false),
			hclspec.NewLiteral("[]"),
		),
		"fingerprint_period": hclspec.NewDefault(
			hclspec.NewAttr("fingerprint_period", "string", false),
			hclspec.NewLiteral("\"5s\""),
		),
		"stats_period": hclspec.NewDefault(
			hclspec.NewAttr("stats_period", "string", false),
			hclspec.NewLiteral("\"5s\""),
		),
	})
)

// Config contains configuration information for the plugin.
type Config struct {
	IgnoredGPUIDs     []string `codec:"ignored_gpu_ids"`
	FingerprintPeriod string   `codec:"fingerprint_period"`
	StatsPeriod       string   `codec:"stats_period"`
}

// NvidiaDevice contains all plugin specific data
type NvidiaDevice struct {
	// nvmlClient is used to get data from nvidia
	nvmlClient nvml.NvmlClient

	// nvmlClientInitializationError holds an error retrieved during
	// nvmlClient initialization
	nvmlClientInitializationError error

	// ignoredGPUIDs is a set of UUIDs that would not be exposed to nomad
	ignoredGPUIDs map[string]struct{}

	// fingerprintPeriod is how often we should call nvml to get list of devices
	fingerprintPeriod time.Duration

	// statsPeriod is how often we should collect statistics for fingerprinted
	// devices.
	statsPeriod time.Duration

	// devices is the set of detected eligible devices
	devices    map[string]struct{}
	deviceLock sync.RWMutex

	logger log.Logger
}

// NewNvidiaDevice returns a new nvidia device plugin.
func NewNvidiaDevice(log log.Logger) *NvidiaDevice {
	nvmlClient, nvmlClientInitializationError := nvml.NewNvmlClient()
	logger := log.Named(pluginName)
	if nvmlClientInitializationError != nil {
		logger.Error("unable to initialize Nvidia driver", "error", nvmlClientInitializationError)
	}
	return &NvidiaDevice{
		logger:                        logger,
		devices:                       make(map[string]struct{}),
		ignoredGPUIDs:                 make(map[string]struct{}),
		nvmlClient:                    nvmlClient,
		nvmlClientInitializationError: nvmlClientInitializationError,
	}
}

// PluginInfo returns information describing the plugin.
func (d *NvidiaDevice) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

// ConfigSchema returns the plugins configuration schema.
func (d *NvidiaDevice) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

// SetConfig is used to set the configuration of the plugin.
func (d *NvidiaDevice) SetConfig(data []byte) error {
	var config Config
	if err := base.MsgPackDecode(data, &config); err != nil {
		return err
	}

	for _, ignoredGPUId := range config.IgnoredGPUIDs {
		d.ignoredGPUIDs[ignoredGPUId] = struct{}{}
	}

	period, err := time.ParseDuration(config.FingerprintPeriod)
	if err != nil {
		return fmt.Errorf("failed to parse fingerprint period %q: %v", config.FingerprintPeriod, err)
	}
	d.fingerprintPeriod = period

	// Convert the stats period
	speriod, err := time.ParseDuration(config.StatsPeriod)
	if err != nil {
		return fmt.Errorf("failed to parse stats period %q: %v", config.StatsPeriod, err)
	}
	d.statsPeriod = speriod

	return nil
}

// Fingerprint streams detected devices. If device changes are detected or the
// devices health changes, messages will be emitted.
func (d *NvidiaDevice) Fingerprint(ctx context.Context) (<-chan *device.FingerprintResponse, error) {
	outCh := make(chan *device.FingerprintResponse)
	go d.fingerprint(ctx, outCh)
	return outCh, nil
}

// Reserve returns information on how to mount the given devices.
func (d *NvidiaDevice) Reserve(deviceIDs []string) (*device.ContainerReservation, error) {
	return nil, nil
}

// Stats streams statistics for the detected devices.
func (d *NvidiaDevice) Stats(ctx context.Context) (<-chan *device.StatsResponse, error) {
	outCh := make(chan *device.StatsResponse)
	go d.stats(ctx, outCh)
	return outCh, nil
}
