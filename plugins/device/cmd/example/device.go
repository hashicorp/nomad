// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

package example

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/plugins/shared/structs"
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

	// fileMode indicates the plugin builds fingerprints based on filepermissions
	fileMode = "file"

	// DynamicMode indicates the plugin builds fingerprints from static config
	dynamicMode = "dynamic"

	staticMode = "static"

	dynamicFileTTL = 120
)

var (

	//disallowedDirs lists filepaths that are not allowed in
	// the `dir` when running in dynamic mode.
	disallowedSegments = []string{"..", "bin", "boot", "dev", "etc", "lib", "sbin", "srv", "proc", "Windows"}

	// pluginInfo describes the plugin
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeDevice,
		PluginApiVersions: []string{device.ApiVersion010},
		PluginVersion:     "v0.1.0",
		Name:              pluginName,
	}

	// configSpec is the specification of the plugin's configuration
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"plugin_mode": hclspec.NewAttr("plugin_mode", "string", false),
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
		"attribute_config": hclspec.NewBlockList("attribute_config", hclspec.NewObject(map[string]*hclspec.Spec{
			"attribute_name":  hclspec.NewAttr("attribute_name", "string", true),
			"attribute_type":  hclspec.NewAttr("attribute_type", "string", true),
			"attribute_unit":  hclspec.NewAttr("unit", "string", false),
			"attribute_value": hclspec.NewAttr("attribute_value", "string", true),
		}),
		),

		"device_config": hclspec.NewBlockList("device_config", hclspec.NewObject(map[string]*hclspec.Spec{
			"id":        hclspec.NewAttr("id", "string", true),
			"unhealthy": hclspec.NewAttr("unhealthy", "bool", false),
		})),
	})
)

// StaticDevice allows users to define devices in configuration
// instead of relying on file discovery.
type StaticDevice struct {
	ID        string `codec:"id"`
	Unhealthy bool   `codec:"unhealthy"`
}

// AttributeConfig defines a list of attributes and values
type AttributeConfig struct {
	AttributeName  string `codec:"attribute_name"`
	AttributeType  string `codec:"attribute_type"`
	AttributeValue string `codec:"attribute_value"`
	AttributeUnit  string `codec:"attribute_unit"`
}

// Config contains configuration information for the plugin.
type Config struct {
	Dir             string             `codec:"dir"`
	ListPeriod      string             `codec:"list_period"`
	PluginMode      string             `codec:"plugin_mode"`
	UnhealthyPerm   string             `codec:"unhealthy_perm"`
	AttributeConfig []*AttributeConfig `codec:"attribute_config"`
	DeviceConfig    []*StaticDevice    `codec:"device_config"`
}

// FileInfo is a struct for holding all the information
// the plugin needs to clean up files after tasks have launched
type FileInfo struct {
	DeviceFiles map[string]time.Time
	FileLock    sync.RWMutex
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

	// pluginMode indicates how devices are modeled. In file mode the plugin reads
	// files from the device directory and reports their health based on file
	// permissions.In static mode the plugin models the devices from the
	// `device_config` configuration object. In dynamic mode the plugin models
	// the devices from `device_config` and creates files during device
	// reservation and attempts to clean them up after 2 minutes in collectDynamicStats.
	pluginMode string

	// defaultAttributes are those defined in configuration. If defined the attributes will be
	// applied to all devices
	defaultAttributes map[string]*structs.Attribute

	// devices is the set of detected devices and maps whether they are healthy
	devices    map[string]bool
	deviceLock sync.RWMutex

	// dynamicFileInfo holds a map of file names dynamically created during device reservation
	// and their creation times. Files older than 90 seconds are deleted during fingerprinting
	dynamicFileInfo FileInfo
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
func (d *FsDevice) configToAttribute(configAttr *AttributeConfig) (*structs.Attribute, error) {
	var attr *structs.Attribute
	if configAttr == nil {
		return nil, fmt.Errorf("failed to parse attribute, cannot be nil: %v", configAttr)
	}
	switch strings.ToLower(configAttr.AttributeType) {
	case "string":
		if configAttr.AttributeValue == "" {
			return nil, fmt.Errorf("failed to parse attribute: %v", configAttr)
		}
		attr = structs.NewStringAttribute(configAttr.AttributeValue)
	case "float":
		fVal, err := strconv.ParseFloat(configAttr.AttributeValue, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse attribute: %s. %w", configAttr.AttributeValue, err)
		}
		attr = structs.NewFloatAttribute(fVal, configAttr.AttributeUnit)
	case "int":
		iVal, err := strconv.ParseInt(configAttr.AttributeValue, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse attribute: %s. %w", configAttr.AttributeValue, err)
		}
		attr = structs.NewIntAttribute(iVal, configAttr.AttributeUnit)
	case "bool":
		bVal, err := strconv.ParseBool(configAttr.AttributeValue)
		if err != nil {
			return nil, fmt.Errorf("failed to parse attribute: %s. %w", configAttr.AttributeValue, err)
		}
		attr = structs.NewBoolAttribute(bVal)
	default:
		return nil, fmt.Errorf("failed to parse attribute: AttributeType must be one of ['string', 'float', 'int', or 'bool'] got: %s.", configAttr.AttributeType)
	}

	return attr, nil
}

// SetConfig is used to set the configuration of the plugin.
func (d *FsDevice) SetConfig(c *base.Config) error {
	var config Config
	if err := base.MsgPackDecode(c.PluginConfig, &config); err != nil {
		return err
	}
	switch config.PluginMode {
	case fileMode:
		d.pluginMode = config.PluginMode
	case staticMode:
		d.pluginMode = config.PluginMode
	case dynamicMode:
		d.pluginMode = config.PluginMode
	default:
		return errors.New("plugin_mode must be set and must be one of `dynamic`, `static` or `file")
	}

	if d.pluginMode != fileMode && len(config.DeviceConfig) == 0 {
		return errors.New("device_config must be provided when plugin_mode set to dynamic` or `static`")
	}
	if d.pluginMode == fileMode && len(config.DeviceConfig) != 0 {
		return errors.New("plugin_mode set to `file`, unexpected device_config provided")
	}

	period, err := time.ParseDuration(config.ListPeriod)
	if err != nil {
		return fmt.Errorf("failed to parse list period %q: %v", config.ListPeriod, err)
	}

	d.deviceDir = config.Dir
	d.unhealthyPerm = config.UnhealthyPerm
	d.listPeriod = period

	if config.AttributeConfig != nil {
		d.defaultAttributes = make(map[string]*structs.Attribute, len(config.AttributeConfig))
		for _, a := range config.AttributeConfig {
			attr, err := d.configToAttribute(a)
			if err != nil {
				return err
			}

			d.defaultAttributes[(strings.ToLower(a.AttributeName))] = attr
		}
	}
	// load device_config devices and set up dynamicDevices, if needed
	if len(config.DeviceConfig) != 0 {
		for _, v := range config.DeviceConfig {
			// device map holds 'healthy' & config holds 'unhealthy', so flip bool
			d.devices[v.ID] = !v.Unhealthy
		}
		if d.pluginMode == dynamicMode {
			// check deviceDir before
			err = sanitizeDir(d.deviceDir)
			if err != nil {
				return err
			}
			d.dynamicFileInfo = FileInfo{
				DeviceFiles: make(map[string]time.Time, len(config.DeviceConfig)),
			}
		}
	}
	return nil
}

// sanitizeDir checks the user provided dir to ensure it is not a relative
// path or a path in a sensitive directory
func sanitizeDir(dirPath string) error {
	// allow default value
	if dirPath == "." {
		return nil
	}
	var segments []string
	segments = strings.Split(dirPath, "/")
	if len(segments) == 1 {
		segments = strings.Split(dirPath, "\\")
	}
	if len(segments) == 1 {
		return errors.New("could not split path")
	}
	for _, segment := range segments {
		if slices.Contains(disallowedSegments, segment) {
			return fmt.Errorf("%s is not allowed in dir when running in dynamic mode", segment)
		}
		// only disallow "." if the segment is less than 2 characters to avoid
		// disallowing hidden directories
		if len(segment) < 32 && strings.Contains(segment, ".") {
			return fmt.Errorf("%s is not allowed in dir when running in dynamic mode", segment)
		}
	}
	return nil
}

// Fingerprint streams detected devices. If device changes are detected or the
// devices health changes, messages will be emitted.
func (d *FsDevice) Fingerprint(ctx context.Context) (<-chan *device.FingerprintResponse, error) {
	outCh := make(chan *device.FingerprintResponse)
	if d.pluginMode != fileMode {
		go d.fingerprint(ctx, outCh)
	} else {
		go d.fingerprintFiles(ctx, outCh)
	}
	return outCh, nil
}

// fingerprint is the long running goroutine that detects hardware
func (d *FsDevice) fingerprintFiles(ctx context.Context, devices chan *device.FingerprintResponse) {
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

		files, err := os.ReadDir(d.deviceDir)
		if err != nil {
			d.logger.Error("failed to list device directory", "error", err)
			devices <- device.NewFingerprintError(err)
			return
		}

		detected := d.diffFiles(files)
		if len(detected) == 0 {
			continue
		}
		group := d.getDeviceGroup(detected)
		devices <- device.NewFingerprint(group)

	}
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

		var dynamicDevices []*device.Device
		for id, healthy := range d.devices {
			unhealthyDesc := "Device was designated unhealthy"
			var desc string
			if !healthy {
				desc = unhealthyDesc
			}
			dynamicDevices = append(dynamicDevices, &device.Device{
				ID:         id,
				Healthy:    healthy,
				HealthDesc: desc,
			})
		}
		group := d.getDeviceGroup(dynamicDevices)
		devices <- device.NewFingerprint(group)

	}

}

func (d *FsDevice) diffFiles(files []os.DirEntry) []*device.Device {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()

	// Build an unhealthy message
	unhealthyDesc := fmt.Sprintf("Device has bad permissions %q", d.unhealthyPerm)

	var changes bool
	fnames := make(map[string]struct{})
	for _, f := range files {
		info, err := f.Info()
		if err != nil {
			d.logger.Error("could not retrieve file info,", "error", err.Error())
		}
		name := info.Name()
		fnames[name] = struct{}{}
		if info.IsDir() {
			d.logger.Trace("skipping directory", "directory", name)
			continue
		}

		// Determine the health
		perms := info.Mode().Perm().String()
		healthy := perms != d.unhealthyPerm
		d.logger.Trace("checking health", "file perm", perms, "unhealthy perms", d.unhealthyPerm, "healthy", healthy)

		// See if we already have the device
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
func (d *FsDevice) getDeviceGroup(devices []*device.Device) *device.DeviceGroup {
	if len(d.defaultAttributes) == 0 {
		return &device.DeviceGroup{
			Vendor:  vendor,
			Type:    deviceType,
			Name:    deviceName,
			Devices: devices,
			Attributes: map[string]*structs.Attribute{
				"cool-attribute": {
					String: new("attribute-wearing-sunglasses"),
				},
			},
		}
	}

	return &device.DeviceGroup{
		Vendor:     vendor,
		Type:       deviceType,
		Name:       deviceName,
		Devices:    devices,
		Attributes: d.defaultAttributes,
	}

}

// Reserve returns information on how to mount the given devices.
func (d *FsDevice) Reserve(deviceIDs []string) (*device.ContainerReservation, error) {
	if len(deviceIDs) == 0 {
		return nil, status.New(codes.InvalidArgument, "no device ids given").Err()
	}

	resp := &device.ContainerReservation{}
	// create device files in dynamic mode only
	if d.pluginMode == dynamicMode {
		d.dynamicFileInfo.FileLock.Lock()
		defer d.dynamicFileInfo.FileLock.Unlock()

		root, err := os.OpenRoot(d.deviceDir)
		if err != nil {
			return nil, err
		}
		defer root.Close()

		for _, v := range deviceIDs {
			if healthy, ok := d.devices[v]; !ok {
				d.logger.Error("requested device unhealthy or not in device list", "device", v, "healthy", healthy, "ok", ok)
				continue
			}

			fileName := filepath.Join(d.deviceDir, v)
			f, err := root.Create(v)
			if err != nil {
				d.logger.Error(fmt.Sprintf("failed to create device file for %s", fileName), "error", err.Error())
				continue
			}

			_, err = f.Stat()
			if err != nil {
				d.logger.Error(fmt.Sprintf("device file may not exist, could not read stats for :%s", fileName), "error", err.Error())
			}
			// Track dynamic file after validating with stats read
			d.dynamicFileInfo.DeviceFiles[v] = time.Now()

			// Add a mount
			resp.Mounts = append(resp.Mounts, &device.Mount{
				TaskPath: fmt.Sprintf("/tmp/task-mounts/%s", v),
				HostPath: filepath.Join(d.deviceDir, v),
				ReadOnly: false,
			})
		}
		return resp, nil
	}

	deviceDir, err := filepath.Abs(d.deviceDir)
	if err != nil {
		return nil, status.Newf(codes.Internal, "failed to load device dir abs path").Err()
	}

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
	var statsFunc func() (*device.DeviceGroupStats, error)

	switch d.pluginMode {
	case fileMode:
		statsFunc = d.collectFileStats
	case staticMode:
		statsFunc = d.collectStaticStats
	case dynamicMode:
		statsFunc = d.collectDynamicStats
	}

	// Create a timer that will fire immediately for the first detection
	ticker := time.NewTimer(0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ticker.Reset(interval)
		}

		deviceStats, err := statsFunc()
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

// collectDynamicStats returns the size, modify_time, and time at which the dynamic
// device file is eligible for deletion
func (d *FsDevice) collectDynamicStats() (*device.DeviceGroupStats, error) {
	l := len(d.devices)
	group := &device.DeviceGroupStats{
		Vendor:        vendor,
		Type:          deviceType,
		Name:          deviceName,
		InstanceStats: make(map[string]*device.DeviceStats, l),
	}
	now := time.Now()
	var (
		failedFiles []string
		errs        multierror.Error
	)
	for name, createTime := range d.dynamicFileInfo.DeviceFiles {
		p := filepath.Join(d.deviceDir, name)
		deletionTime := createTime.Add(time.Second * dynamicFileTTL)

		f, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("failed to stat %q: %v", p, err)
		}

		s := &device.DeviceStats{
			Summary: &structs.StatValue{
				IntNumeratorVal: new(f.Size()),
				Unit:            "bytes",
				Desc:            "Filesize in bytes",
			},
			Stats: &structs.StatObject{
				Attributes: map[string]*structs.StatValue{
					"size": {
						IntNumeratorVal: new(f.Size()),
						Unit:            "bytes",
						Desc:            "Filesize in bytes",
					},
					"modify_time": {
						StringVal: new(f.ModTime().String()),
						Desc:      "Last modified",
					},
					"file_ttl": {
						StringVal: new(deletionTime.GoString()),
						Desc:      "Time after which dynamic file will be removed",
					},
				},
			},
			Timestamp: now,
		}

		group.InstanceStats[name] = s

		now := time.Now()
		if now.After(deletionTime) {
			d.dynamicFileInfo.FileLock.Lock()
			defer d.dynamicFileInfo.FileLock.Unlock()

			root, err := os.OpenRoot(d.deviceDir)
			if err != nil {
				d.logger.Error("deletion failed", "error", err.Error())
				continue
			}

			err = root.Remove(name)
			if err != nil {
				errs = *multierror.Append(&errs, err)
				failedFiles = append(failedFiles, p)
			} else {
				delete(d.dynamicFileInfo.DeviceFiles, name)
				d.logger.Trace("successfully removed device", "device name", p)
			}
		}
	}

	collectedErrs := errs.ErrorOrNil()
	if collectedErrs != nil {
		d.logger.Error("dynamic file deletion failed, manual deletion required", "undeleted files", strings.Join(failedFiles, ", "), "error", collectedErrs.Error())

	}
	return group, collectedErrs
}

// collectStaticStats returns a single stat that lists the number of devices available
func (d *FsDevice) collectStaticStats() (*device.DeviceGroupStats, error) {
	l := len(d.devices)
	return &device.DeviceGroupStats{
		Vendor: vendor,
		Type:   deviceType,
		Name:   deviceName,
		InstanceStats: map[string]*device.DeviceStats{
			"static": {
				Summary: &structs.StatValue{
					IntNumeratorVal: new(int64(l)),
					Unit:            "devices",
					Desc:            "Number of static devices available",
				},
			},
		},
	}, nil

}

// colelctFileStats returns the size, modify_time, and mode for device files
func (d *FsDevice) collectFileStats() (*device.DeviceGroupStats, error) {
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
				IntNumeratorVal: new(f.Size()),
				Unit:            "bytes",
				Desc:            "Filesize in bytes",
			},
			Stats: &structs.StatObject{
				Attributes: map[string]*structs.StatValue{
					"size": {
						IntNumeratorVal: new(f.Size()),
						Unit:            "bytes",
						Desc:            "Filesize in bytes",
					},
					"modify_time": {
						StringVal: new(f.ModTime().String()),
						Desc:      "Last modified",
					},
					"mode": {
						StringVal: new(f.Mode().String()),
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
