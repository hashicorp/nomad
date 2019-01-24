package nvidia

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/devices/gpu/nvidia/nvml"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
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

const (
	// Nvidia-container-runtime environment variable names
	NvidiaVisibleDevices = "NVIDIA_VISIBLE_DEVICES"
)

var (
	// PluginID is the nvidia plugin metadata registered in the plugin
	// catalog.
	PluginID = loader.PluginID{
		Name:       pluginName,
		PluginType: base.PluginTypeDevice,
	}

	// PluginConfig is the nvidia factory function registered in the
	// plugin catalog.
	PluginConfig = &loader.InternalPluginConfig{
		Factory: func(l log.Logger) interface{} { return NewNvidiaDevice(l) },
	}

	// pluginInfo describes the plugin
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeDevice,
		PluginApiVersions: []string{device.ApiVersion010},
		PluginVersion:     "0.1.0",
		Name:              pluginName,
	}

	// configSpec is the specification of the plugin's configuration
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"ignored_gpu_ids": hclspec.NewDefault(
			hclspec.NewAttr("ignored_gpu_ids", "list(string)", false),
			hclspec.NewLiteral("[]"),
		),
		"fingerprint_period": hclspec.NewDefault(
			hclspec.NewAttr("fingerprint_period", "string", false),
			hclspec.NewLiteral("\"1m\""),
		),
	})
)

// Config contains configuration information for the plugin.
type Config struct {
	IgnoredGPUIDs     []string `codec:"ignored_gpu_ids"`
	FingerprintPeriod string   `codec:"fingerprint_period"`
}

// NvidiaDevice contains all plugin specific data
type NvidiaDevice struct {
	// nvmlClient is used to get data from nvidia
	nvmlClient nvml.NvmlClient

	// initErr holds an error retrieved during
	// nvmlClient initialization
	initErr error

	// ignoredGPUIDs is a set of UUIDs that would not be exposed to nomad
	ignoredGPUIDs map[string]struct{}

	// fingerprintPeriod is how often we should call nvml to get list of devices
	fingerprintPeriod time.Duration

	// devices is the set of detected eligible devices
	devices    map[string]struct{}
	deviceLock sync.RWMutex

	logger log.Logger
}

// NewNvidiaDevice returns a new nvidia device plugin.
func NewNvidiaDevice(log log.Logger) *NvidiaDevice {
	nvmlClient, err := nvml.NewNvmlClient()
	logger := log.Named(pluginName)
	if err != nil && err.Error() != nvml.UnavailableLib.Error() {
		logger.Error("unable to initialize Nvidia driver", "reason", err)
	}
	return &NvidiaDevice{
		logger:        logger,
		devices:       make(map[string]struct{}),
		ignoredGPUIDs: make(map[string]struct{}),
		nvmlClient:    nvmlClient,
		initErr:       err,
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
func (d *NvidiaDevice) SetConfig(cfg *base.Config) error {
	var config Config
	if len(cfg.PluginConfig) != 0 {
		if err := base.MsgPackDecode(cfg.PluginConfig, &config); err != nil {
			return err
		}
	}

	for _, ignoredGPUId := range config.IgnoredGPUIDs {
		d.ignoredGPUIDs[ignoredGPUId] = struct{}{}
	}

	period, err := time.ParseDuration(config.FingerprintPeriod)
	if err != nil {
		return fmt.Errorf("failed to parse fingerprint period %q: %v", config.FingerprintPeriod, err)
	}
	d.fingerprintPeriod = period

	return nil
}

// Fingerprint streams detected devices. If device changes are detected or the
// devices health changes, messages will be emitted.
func (d *NvidiaDevice) Fingerprint(ctx context.Context) (<-chan *device.FingerprintResponse, error) {
	outCh := make(chan *device.FingerprintResponse)
	go d.fingerprint(ctx, outCh)
	return outCh, nil
}

type reservationError struct {
	notExistingIDs []string
}

func (e *reservationError) Error() string {
	return fmt.Sprintf("unknown device IDs: %s", strings.Join(e.notExistingIDs, ","))
}

// Reserve returns information on how to mount given devices.
// Assumption is made that nomad server is responsible for correctness of
// GPU allocations, handling tricky cases such as double-allocation of single GPU
func (d *NvidiaDevice) Reserve(deviceIDs []string) (*device.ContainerReservation, error) {
	if len(deviceIDs) == 0 {
		return &device.ContainerReservation{}, nil
	}
	// Due to the asynchronous nature of NvidiaPlugin, there is a possibility
	// of race condition
	//
	// Timeline:
	// 	1 - fingerprint reports that GPU with id "1" is present
	//  2 - the following events happen at the same time:
	// 		a) server decides to allocate GPU with id "1"
	//      b) fingerprint check reports that GPU with id "1" is no more present
	//
	// The latest and always valid version of fingerprinted ids are stored in
	// d.devices map. To avoid this race condition an error is returned if
	// any of provided deviceIDs is not found in d.devices map
	d.deviceLock.RLock()
	var notExistingIDs []string
	for _, id := range deviceIDs {
		if _, deviceIDExists := d.devices[id]; !deviceIDExists {
			notExistingIDs = append(notExistingIDs, id)
		}
	}
	d.deviceLock.RUnlock()
	if len(notExistingIDs) != 0 {
		return nil, &reservationError{notExistingIDs}
	}

	return &device.ContainerReservation{
		Envs: map[string]string{
			NvidiaVisibleDevices: strings.Join(deviceIDs, ","),
		},
	}, nil
}

// Stats streams statistics for the detected devices.
func (d *NvidiaDevice) Stats(ctx context.Context, interval time.Duration) (<-chan *device.StatsResponse, error) {
	outCh := make(chan *device.StatsResponse)
	go d.stats(ctx, outCh, interval)
	return outCh, nil
}
