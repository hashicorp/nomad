package example

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/kr/pretty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// pluginName is the name of the plugin
	pluginName = "example-fs-device"

	// vendor is the vendor providing the devices
	vendor = "nomad"

	// deviceType is the type of device being returned
	deviceType = "file"

	// deviceName is the name of the devices being exposed
	deviceName = "mock"
)

var (
	// pluginInfo describes the plugin
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeDevice,
		PluginApiVersions: []string{device.ApiVersion010},
		PluginVersion:     "v0.1.0",
		Name:              pluginName,
	}

	// configSpec is the specification of the plugin's configuration
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
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
	})
)

// Config contains configuration information for the plugin.
type Config struct {
	Dir           string `codec:"dir"`
	ListPeriod    string `codec:"list_period"`
	UnhealthyPerm string `codec:"unhealthy_perm"`
}

// FsDevice is an example device plugin. The device plugin exposes files as
// devices and periodically polls the directory for new files. If a file has a
// given file permission, it is considered unhealthy. This device plugin is
// purely for use as an example.
type FsDevice struct {
	logger log.Logger

	// deviceDir is the directory we expose as devices
	deviceDir string

	// unhealthyPerm is the permissions on a file we consider unhealthy
	unhealthyPerm string

	// listPeriod is how often we should list the device directory to detect new
	// devices
	listPeriod time.Duration

	// devices is the set of detected devices and maps whether they are healthy
	devices    map[string]bool
	deviceLock sync.RWMutex
}

// NewExampleDevice returns a new example device plugin.
func NewExampleDevice(log log.Logger) *FsDevice {
	return &FsDevice{
		logger:  log.Named(pluginName),
		devices: make(map[string]bool),
	}
}

// PluginInfo returns information describing the plugin.
func (d *FsDevice) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

// ConfigSchema returns the plugins configuration schema.
func (d *FsDevice) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

// SetConfig is used to set the configuration of the plugin.
func (d *FsDevice) SetConfig(c *base.Config) error {
	var config Config
	if err := base.MsgPackDecode(c.PluginConfig, &config); err != nil {
		return err
	}

	// Save the device directory and the unhealthy permissions
	d.deviceDir = config.Dir
	d.unhealthyPerm = config.UnhealthyPerm

	// Convert the poll period
	period, err := time.ParseDuration(config.ListPeriod)
	if err != nil {
		return fmt.Errorf("failed to parse list period %q: %v", config.ListPeriod, err)
	}
	d.listPeriod = period

	d.logger.Debug("test debug")
	d.logger.Info("config set", "config", log.Fmt("% #v", pretty.Formatter(config)))
	return nil
}

// Fingerprint streams detected devices. If device changes are detected or the
// devices health changes, messages will be emitted.
func (d *FsDevice) Fingerprint(ctx context.Context) (<-chan *device.FingerprintResponse, error) {
	if d.deviceDir == "" {
		return nil, status.New(codes.Internal, "device directory not set in config").Err()
	}

	outCh := make(chan *device.FingerprintResponse)
	go d.fingerprint(ctx, outCh)
	return outCh, nil
}

// fingerprint is the long running goroutine that detects hardware
func (d *FsDevice) fingerprint(ctx context.Context, devices chan *device.FingerprintResponse) {
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

		d.logger.Trace("scanning for changes")

		files, err := ioutil.ReadDir(d.deviceDir)
		if err != nil {
			d.logger.Error("failed to list device directory", "error", err)
			devices <- device.NewFingerprintError(err)
			return
		}

		detected := d.diffFiles(files)
		if len(detected) == 0 {
			continue
		}

		devices <- device.NewFingerprint(getDeviceGroup(detected))

	}
}

func (d *FsDevice) diffFiles(files []os.FileInfo) []*device.Device {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()

	// Build an unhealthy message
	unhealthyDesc := fmt.Sprintf("Device has bad permissions %q", d.unhealthyPerm)

	var changes bool
	fnames := make(map[string]struct{})
	for _, f := range files {
		name := f.Name()
		fnames[name] = struct{}{}
		if f.IsDir() {
			d.logger.Trace("skipping directory", "directory", name)
			continue
		}

		// Determine the health
		perms := f.Mode().Perm().String()
		healthy := perms != d.unhealthyPerm
		d.logger.Trace("checking health", "file perm", perms, "unhealthy perms", d.unhealthyPerm, "healthy", healthy)

		// See if we alreay have the device
		oldHealth, ok := d.devices[name]
		if ok && oldHealth == healthy {
			continue
		}

		// Health has changed or we have a new object
		changes = true
		d.devices[name] = healthy
	}

	for id := range d.devices {
		if _, ok := fnames[id]; !ok {
			delete(d.devices, id)
			changes = true
		}
	}

	// Nothing to do
	if !changes {
		return nil
	}

	// Build the devices
	detected := make([]*device.Device, 0, len(d.devices))
	for name, healthy := range d.devices {
		var desc string
		if !healthy {
			desc = unhealthyDesc
		}

		detected = append(detected, &device.Device{
			ID:         name,
			Healthy:    healthy,
			HealthDesc: desc,
		})
	}

	return detected
}

// getDeviceGroup is a helper to build the DeviceGroup given a set of devices.
func getDeviceGroup(devices []*device.Device) *device.DeviceGroup {
	return &device.DeviceGroup{
		Vendor:  vendor,
		Type:    deviceType,
		Name:    deviceName,
		Devices: devices,
	}
}

// Reserve returns information on how to mount the given devices.
func (d *FsDevice) Reserve(deviceIDs []string) (*device.ContainerReservation, error) {
	if len(deviceIDs) == 0 {
		return nil, status.New(codes.InvalidArgument, "no device ids given").Err()
	}

	deviceDir, err := filepath.Abs(d.deviceDir)
	if err != nil {
		return nil, status.Newf(codes.Internal, "failed to load device dir abs path").Err()
	}

	resp := &device.ContainerReservation{}

	for _, id := range deviceIDs {
		// Check if the device is known
		if _, ok := d.devices[id]; !ok {
			return nil, status.Newf(codes.InvalidArgument, "unknown device %q", id).Err()
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
func (d *FsDevice) Stats(ctx context.Context, interval time.Duration) (<-chan *device.StatsResponse, error) {
	outCh := make(chan *device.StatsResponse)
	go d.stats(ctx, outCh, interval)
	return outCh, nil
}

// stats is the long running goroutine that streams device statistics
func (d *FsDevice) stats(ctx context.Context, stats chan *device.StatsResponse, interval time.Duration) {
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

func (d *FsDevice) collectStats() (*device.DeviceGroupStats, error) {
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
		Name:          deviceName,
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
