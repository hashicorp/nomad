// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

package example

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/hashicorp/nomad/version"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// pluginName is the name of the plugin
	pluginName = "nvidia-example"

	// vendor is the vendor providing the devices
	vendor = "nvidia"

	// deviceType is the type of device being returned
	deviceType = device.DeviceTypeGPU

	// notAvailable value is returned to nomad server in case some properties were
	// undetected by nvml driver
	notAvailable = "N/A"

	// Nvidia-container-runtime environment variable names
	NvidiaVisibleDevices = "NVIDIA_VISIBLE_DEVICES"

	// MPS runtime environment variables
	MpsPipeDirectoryKey = "MPS_PIPE_DIRECTORY"
	MpsLogDirectoryKey  = "MPS_LOG_DIRECTORY"
	CustomMpsUserKey    = "MPS_USER"

	DefaultMpsSockFileAddr = "control"

	deviceName1 = "T4.active"
	deviceName2 = "T4.inactive"
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
		Factory: func(ctx context.Context, l hclog.Logger) interface{} { return NewNvidiaDevice(ctx, l) },
	}

	// pluginInfo describes the plugin
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeDevice,
		PluginApiVersions: []string{device.ApiVersion010},
		PluginVersion:     version.Version,
		Name:              pluginName,
	}

	// configSpec is the specification of the plugin's configuration
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"enabled": hclspec.NewDefault(
			hclspec.NewAttr("enabled", "bool", false),
			hclspec.NewLiteral("true"),
		),

		"ignored_gpu_ids": hclspec.NewDefault(
			hclspec.NewAttr("ignored_gpu_ids", "list(string)", false),
			hclspec.NewLiteral("[]"),
		),
		"fingerprint_period": hclspec.NewDefault(
			hclspec.NewAttr("fingerprint_period", "string", false),
			hclspec.NewLiteral("\"1m\""),
		),
		"dir": hclspec.NewDefault(
			hclspec.NewAttr("dir", "string", false),
			hclspec.NewLiteral("\".\""),
		),
		"list_period": hclspec.NewDefault(
			hclspec.NewAttr("list_period", "string", false),
			hclspec.NewLiteral("\"5s\""),
		),
		"unhealthy_perm": hclspec.NewDefault(
			hclspec.NewAttr("unhealthy_perm", "string", false),
			hclspec.NewLiteral("\"-rwxrwxrwx\""),
		),
		//"dir": hclspec.NewAttr("dir", "string", true),
		//"mps_pipe_directory": hclspec.NewDefault(
		//	hclspec.NewAttr("mps_pipe_directory", "string", false),
		//	hclspec.NewLiteral("/tmp/nvidia-mps"),
		//),
		"mps": hclspec.NewBlock("mps", false,
			hclspec.NewObject(map[string]*hclspec.Spec{
				"enabled":  hclspec.NewAttr("enabled", "bool", true),
				"mps_user": hclspec.NewAttr("mps_user", "string", false),
				//hclspec.NewLiteral("unset"),

				"mps_log_directory": hclspec.NewAttr("mps_log_directory", "string", false),
				//hclspec.NewLiteral("/var/log/nvidia-mps"),
				//),
				"mps_sock_addr": hclspec.NewAttr("mps_sock_addr", "string", false),
				//hclspec.NewLiteral("control"),
				//),
				//"device_specific_mps_config": hclspec.NewBlockList("device_specific_mps_config",
				//	hclspec.NewArray(
				//		[]*hclspec.Spec{
				//			hclspec.NewObject(map[string]*hclspec.Spec{
				//				"uuid": hclspec.NewAttr("uuid", "string", true),
				//				"mps_pipe_directory": hclspec.NewDefault(
				//					hclspec.NewAttr("mps_pipe_directory", "string", true),
				//					hclspec.NewLiteral("/tmp/nvidia-mps"),
				//				),
				//				"mps_log_directory": hclspec.NewDefault(
				//					hclspec.NewAttr("mps_log_directory", "string", true),
				//					hclspec.NewLiteral("/tmp/nvidia-mps"),
				//				),
				//			}),
				//		},
				//	)),
			}),
		),
	})
)

// Config contains configuration information for the plugin.
type Config struct {
	Enabled           bool       `codec:"enabled"`
	IgnoredGPUIDs     []string   `codec:"ignored_gpu_ids"`
	FingerprintPeriod string     `codec:"fingerprint_period"`
	MpsConfig         *MpsConfig `codec:"mps"`
	Dir               string     `codec:"dir"`
	ListPeriod        string     `codec:"list_period"`
	UnhealthyPerm     string     `codec:"unhealthy_perm"`
}

type MpsConfig struct {
	MpsUser          string                     `codec:"mps_user"`
	MpsSockFile      string                     `codec:"mps_sock_addr"`
	MpsPipeDirectory string                     `codec:"mps_pipe_directory"`
	MpsLogDirectory  string                     `codec:"mps_log_directory"`
	DeviceMpsConfig  map[string]DeviceMpsConfig `codec:"device_specific_mps_config"`
}
type DeviceMpsConfig struct {
	UUID             string `codec:"uuid"`
	MpsPipeDirectory string `codec:"mps_pipe_directory"`
	MpsLogDirectory  string `codec:"mps_log_directory"`
}

type NvidiaDevice struct {
	// enabled indicates whether the plugin should be enabled
	enabled bool

	// nvmlClient is used to get data from nvidia
	//nvmlClient nvml.NvmlClient

	// initErr holds an error retrieved during
	// nvmlClient initialization
	//initErr error

	// deviceDir is the directory we expose as devices
	deviceDir string

	// unhealthyPerm is the permissions on a file we consider unhealthy
	unhealthyPerm string

	// listPeriod is how often we should list the device directory to detect new
	// devices
	listPeriod time.Duration

	// ignoredGPUIDs is a set of UUIDs that would not be exposed to nomad
	ignoredGPUIDs map[string]struct{}

	// fingerprintPeriod is how often we should call nvml to get list of devices
	//fingerprintPeriod time.Duration

	//MpsConfig holds a pointer to the MPS configuration
	MpsConfig *MpsConfig

	// devices is the set of detected eligible devices
	devices    map[string]device.DeviceSharing
	deviceLock sync.RWMutex

	logger hclog.Logger
}

// NewNvidiaDevice returns a new nvidia device plugin.
func NewNvidiaDevice(_ context.Context, log hclog.Logger) *NvidiaDevice {
	//nvmlClient, err := nvml.NewNvmlClient()
	logger := log.Named(pluginName)
	//if err != nil && err.Error() != nvml.ErrUnavailableLib.Error() {
	//	logger.Error("unable to initialize Nvidia driver", "reason", err)
	//}
	return &NvidiaDevice{
		logger:        logger,
		devices:       make(map[string]device.DeviceSharing),
		ignoredGPUIDs: make(map[string]struct{}),
		//nvmlClient:    nvmlClient,
		//initErr:       err,
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

func checkAndSetDefault(c string, d string) string {
	if config := c; config != "" {
		return c
	}

	return d
}

// SetConfig is used to set the configuration of the plugin.
func (d *NvidiaDevice) SetConfig(cfg *base.Config) error {
	var config Config
	if len(cfg.PluginConfig) != 0 {
		if err := base.MsgPackDecode(cfg.PluginConfig, &config); err != nil {
			return err
		}
	}
	d.enabled = config.Enabled
	// set MPS config values
	if config.MpsConfig != nil {
		// ensure only global or device specific config are set
		if (config.MpsConfig.MpsPipeDirectory != "" || config.MpsConfig.MpsLogDirectory != "") &&
			len(config.MpsConfig.DeviceMpsConfig) != 0 {
			return errors.New("only global mps variables or device_specific_mps_config block may be set ")
		}

		// Initialize MpsConfig if it hasn't been initialized yet
		if d.MpsConfig == nil {
			fmt.Println("THIS SHOULDN'T HAPPEN")
			d.MpsConfig = &MpsConfig{}
		}

		// set straightforward value on device
		d.MpsConfig.MpsUser = checkAndSetDefault(config.MpsConfig.MpsUser, "unset")
		d.MpsConfig.MpsSockFile = checkAndSetDefault(config.MpsConfig.MpsUser, DefaultMpsSockFileAddr)

		// if present set device specific mps config, otherwise set top level config
		if len(config.MpsConfig.DeviceMpsConfig) != 0 {

			// build map of device UUIDs to config
			deviceConfigMap := make(map[string]DeviceMpsConfig, len(config.MpsConfig.DeviceMpsConfig))
			for _, devConfig := range config.MpsConfig.DeviceMpsConfig {
				deviceConfigMap[devConfig.UUID] = DeviceMpsConfig{
					UUID:             devConfig.UUID,
					MpsPipeDirectory: devConfig.MpsPipeDirectory,
					MpsLogDirectory:  devConfig.MpsLogDirectory,
				}
			}
			// set device specific mpsConfig
			d.MpsConfig.DeviceMpsConfig = deviceConfigMap
		} else {
			// set top level mps directories if no device specific config
			// we have defaults so always use config values
			d.MpsConfig.MpsPipeDirectory = config.MpsConfig.MpsPipeDirectory
			d.MpsConfig.MpsLogDirectory = config.MpsConfig.MpsLogDirectory
			if pipe_dir := config.MpsConfig.MpsPipeDirectory; pipe_dir != "" {
				d.MpsConfig.MpsPipeDirectory = pipe_dir
			} else {
				d.MpsConfig.MpsPipeDirectory = checkAndSetDefault(config.MpsConfig.MpsPipeDirectory, "/tmp/nvidia-mps")

				d.MpsConfig.MpsLogDirectory = checkAndSetDefault(config.MpsConfig.MpsLogDirectory, "/var/log/nvidia-mps")
			}
		}
	}
	for _, ignoredGPUId := range config.IgnoredGPUIDs {
		d.ignoredGPUIDs[ignoredGPUId] = struct{}{}
	}

	//period, err := time.ParseDuration(config.FingerprintPeriod)
	//if err != nil {
	//	return fmt.Errorf("failed to parse fingerprint period %q: %v", config.FingerprintPeriod, err)
	//}
	//d.fingerprintPeriod = period
	// Save the device directory and the unhealthy permissions
	d.deviceDir = config.Dir
	d.unhealthyPerm = config.UnhealthyPerm

	// Convert the poll period
	period, err := time.ParseDuration(config.ListPeriod)
	if err != nil {
		return fmt.Errorf("failed to parse list period %q: %v", config.ListPeriod, err)
	}
	d.listPeriod = period

	return nil
}

// Fingerprint streams detected devices. If device changes are detected or the
// devices health changes, messages will be emitted.
func (d *NvidiaDevice) Fingerprint(ctx context.Context) (<-chan *device.FingerprintResponse, error) {
	if !d.enabled {
		return nil, device.ErrPluginDisabled
	}

	outCh := make(chan *device.FingerprintResponse)
	go d.fingerprint(ctx, outCh)
	return outCh, nil
}

// fingerprint is the long running goroutine that detects hardware
func (d *NvidiaDevice) fingerprint(ctx context.Context, devices chan *device.FingerprintResponse) {
	defer close(devices)

	// Create a timer that will fire immediately for the first detection
	ticker := time.NewTimer(0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ticker.Reset(d.listPeriod)
		}

		d.logger.Info("scanning for changes")

		files, err := ioutil.ReadDir(d.deviceDir)
		if err != nil {
			d.logger.Error("failed to list device directory", "error", err)
			devices <- device.NewFingerprintError(err)
			return
		}
		deviceGroups := make([]*device.DeviceGroup, 0)
		shared, inactive := d.diffFiles(files)
		if len(inactive) != 0 {
			deviceGroups = append(deviceGroups, d.getDeviceGroup(inactive, deviceName2))
		}

		if len(shared) != 0 {
			deviceGroups = append(deviceGroups, d.getDeviceGroup(shared, deviceName1))
		}
		d.logger.Info("files to fingerprint", "inactive files", len(inactive), "active files", len(shared))
		devices <- device.NewFingerprint(deviceGroups...)

	}
}
func (d *NvidiaDevice) diffFiles(files []os.FileInfo) ([]*device.Device, []*device.Device) {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()

	// Build an unhealthy message
	unhealthyDesc := fmt.Sprintf("Device has bad permissions %q", d.unhealthyPerm)

	//var changes bool
	fnames := make(map[string]struct{})
	for _, f := range files {
		name := f.Name()
		fnames[name] = struct{}{}
		if f.IsDir() {
			d.logger.Info("skipping directory", "directory", name)
			continue
		}

		// Determine the health
		perms := f.Mode().Perm().String()
		//turn health into sharing status
		healthBool := perms != d.unhealthyPerm
		var healthy device.DeviceSharing
		if healthBool {
			healthy = device.SharingActive
		} else {
			healthy = device.SharingInactive
		}
		d.logger.Info("checking health", "file perm", perms, "unhealthy perms", d.unhealthyPerm, "healthy", healthy)

		// See if we alreay have the device
		oldHealth, ok := d.devices[name]
		if ok && oldHealth == healthy {
			continue
		}

		// Health has changed or we have a new object
		//changes = true
		d.devices[name] = healthy
	}

	for id := range d.devices {
		if _, ok := fnames[id]; !ok {
			delete(d.devices, id)
			//changes = true
		}
	}

	//// Nothing to do
	//if !changes {
	//	return nil, nil
	//}

	// Build the devices
	shared := make([]*device.Device, 0, len(d.devices))
	inactive := make([]*device.Device, 0, len(d.devices))

	for name, healthy := range d.devices {
		var desc string
		if healthy != device.SharingActive {
			desc = unhealthyDesc
			inactive = append(inactive, &device.Device{
				ID:         name,
				Shared:     healthy,
				HealthDesc: desc,
			})
			continue
		}

		shared = append(shared, &device.Device{
			ID:         name,
			Shared:     healthy,
			HealthDesc: desc,
		})
	}

	return shared, inactive
}

// getDeviceGroup is a helper to build the DeviceGroup given a set of devices.
func (d *NvidiaDevice) getDeviceGroup(devices []*device.Device, name string) *device.DeviceGroup {

	var shared string
	for _, v := range devices {
		if shared == "" {
			shared = string(v.Shared)
		}
		d.logger.Info("getdevicegroup", "shared", v.Shared)
	}

	return &device.DeviceGroup{
		Vendor:  vendor,
		Type:    deviceType,
		Name:    name,
		Devices: devices,
		Attributes: map[string]*structs.Attribute{
			"Shared": {
				String: pointer.Of(shared),
			},
		},
	}

}

// Reserve returns information on how to mount the given devices.
func (d *NvidiaDevice) Reserve(deviceIDs []string) (*device.ContainerReservation, error) {
	if len(deviceIDs) == 0 {
		return nil, status.New(codes.InvalidArgument, "no device ids given").Err()
	}

	deviceDir, err := filepath.Abs(d.deviceDir)
	if err != nil {
		return nil, status.Newf(codes.Internal, "failed to load device dir abs path").Err()
	}

	resp := &device.ContainerReservation{}
	containerEnvs := make(map[string]string)
	for _, id := range deviceIDs {
		// Check if the device is known
		if _, ok := d.devices[id]; !ok {
			return nil, status.Newf(codes.InvalidArgument, "unknown device %q", id).Err()
		}
		if d.devices[id] == device.SharingActive {
			containerEnvs[MpsPipeDirectoryKey] = d.MpsConfig.MpsPipeDirectory
			containerEnvs[MpsLogDirectoryKey] = d.MpsConfig.MpsLogDirectory
		}

		// Add a mount
		resp.Mounts = append(resp.Mounts, &device.Mount{
			TaskPath: fmt.Sprintf("/tmp/task-mounts/%s", id),
			HostPath: filepath.Join(deviceDir, id),
			ReadOnly: false,
		})
	}

	return resp, nil
}

// Stats streams statistics for the detected devices.
func (d *NvidiaDevice) Stats(ctx context.Context, interval time.Duration) (<-chan *device.StatsResponse, error) {
	outCh := make(chan *device.StatsResponse)
	go d.stats(ctx, outCh, interval)
	return outCh, nil
}

// stats is the long running goroutine that streams device statistics
func (d *NvidiaDevice) stats(ctx context.Context, stats chan *device.StatsResponse, interval time.Duration) {
	defer close(stats)

	// Create a timer that will fire immediately for the first detection
	ticker := time.NewTimer(0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ticker.Reset(interval)
		}

		deviceStats, err := d.collectStats()
		if err != nil {
			stats <- &device.StatsResponse{
				Error: err,
			}
			return
		}
		if deviceStats == nil {
			continue
		}

		stats <- &device.StatsResponse{
			Groups: []*device.DeviceGroupStats{deviceStats},
		}
	}
}

func (d *NvidiaDevice) collectStats() (*device.DeviceGroupStats, error) {
	d.deviceLock.RLock()
	defer d.deviceLock.RUnlock()
	l := len(d.devices)
	if l == 0 {
		return nil, nil
	}

	now := time.Now()
	group := &device.DeviceGroupStats{
		Vendor:        vendor,
		Type:          deviceType,
		Name:          deviceName1,
		InstanceStats: make(map[string]*device.DeviceStats, l),
	}

	for k := range d.devices {
		p := filepath.Join(d.deviceDir, k)
		f, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("failed to stat %q: %v", p, err)
		}

		s := &device.DeviceStats{
			Summary: &structs.StatValue{
				IntNumeratorVal: pointer.Of(f.Size()),
				Unit:            "bytes",
				Desc:            "Filesize in bytes",
			},
			Stats: &structs.StatObject{
				Attributes: map[string]*structs.StatValue{
					"size": {
						IntNumeratorVal: pointer.Of(f.Size()),
						Unit:            "bytes",
						Desc:            "Filesize in bytes",
					},
					"modify_time": {
						StringVal: pointer.Of(f.ModTime().String()),
						Desc:      "Last modified",
					},
					"mode": {
						StringVal: pointer.Of(f.Mode().String()),
						Desc:      "File mode",
					},
				},
			},
			Timestamp: now,
		}

		group.InstanceStats[k] = s
	}

	return group, nil
}
