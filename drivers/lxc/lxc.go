//+build linux,lxc

package lxc

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/nomad/plugins/drivers"
	ldevices "github.com/opencontainers/runc/libcontainer/devices"
	"gopkg.in/lxc/go-lxc.v2"
)

var (
	verbosityLevels = map[string]lxc.Verbosity{
		"":        lxc.Quiet,
		"verbose": lxc.Verbose,
		"quiet":   lxc.Quiet,
	}

	logLevels = map[string]lxc.LogLevel{
		"":      lxc.ERROR,
		"debug": lxc.DEBUG,
		"error": lxc.ERROR,
		"info":  lxc.INFO,
		"trace": lxc.TRACE,
		"warn":  lxc.WARN,
	}
)

const (
	// containerMonitorIntv is the interval at which the driver checks if the
	// container is still alive
	containerMonitorIntv = 2 * time.Second
)

func (d *Driver) lxcPath() string {
	lxcPath := d.config.Path
	if lxcPath == "" {
		lxcPath = lxc.DefaultConfigPath()
	}
	return lxcPath

}
func (d *Driver) initializeContainer(cfg *drivers.TaskConfig, taskConfig TaskConfig) (*lxc.Container, error) {
	lxcPath := d.lxcPath()

	containerName := fmt.Sprintf("%s-%s", cfg.Name, cfg.AllocID)
	c, err := lxc.NewContainer(containerName, lxcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize container: %v", err)
	}

	if v, ok := verbosityLevels[taskConfig.Verbosity]; ok {
		c.SetVerbosity(v)
	} else {
		return nil, fmt.Errorf("lxc driver config 'verbosity' can only be either quiet or verbose")
	}

	if v, ok := logLevels[taskConfig.LogLevel]; ok {
		c.SetLogLevel(v)
	} else {
		return nil, fmt.Errorf("lxc driver config 'log_level' can only be trace, debug, info, warn or error")
	}

	logFile := filepath.Join(cfg.TaskDir().Dir, fmt.Sprintf("%v-lxc.log", cfg.Name))
	c.SetLogFile(logFile)

	return c, nil
}

func (d *Driver) configureContainerNetwork(c *lxc.Container) error {
	// Set the network type to none
	if err := c.SetConfigItem(networkTypeConfigKey(), "none"); err != nil {
		return fmt.Errorf("error setting network type configuration: %v", err)
	}
	return nil
}

func networkTypeConfigKey() string {
	if lxc.VersionAtLeast(2, 1, 0) {
		return "lxc.net.0.type"
	}

	// prior to 2.1, network used
	return "lxc.network.type"
}

func (d *Driver) mountVolumes(c *lxc.Container, cfg *drivers.TaskConfig, taskConfig TaskConfig) error {
	mounts, err := d.mountEntries(cfg, taskConfig)
	if err != nil {
		return err
	}

	devCgroupAllows, err := d.devicesCgroupEntries(cfg)
	if err != nil {
		return err
	}

	for _, mnt := range mounts {
		if err := c.SetConfigItem("lxc.mount.entry", mnt); err != nil {
			return fmt.Errorf("error setting bind mount %q error: %v", mnt, err)
		}
	}

	for _, cgroupDev := range devCgroupAllows {
		if err := c.SetConfigItem("lxc.cgroup.devices.allow", cgroupDev); err != nil {
			return fmt.Errorf("error setting cgroup permission %q error: %v", cgroupDev, err)
		}
	}

	return nil
}

// mountEntries compute the mount entries to be set on the container
func (d *Driver) mountEntries(cfg *drivers.TaskConfig, taskConfig TaskConfig) ([]string, error) {
	// Bind mount the shared alloc dir and task local dir in the container
	mounts := []string{
		fmt.Sprintf("%s local none rw,bind,create=dir", cfg.TaskDir().LocalDir),
		fmt.Sprintf("%s alloc none rw,bind,create=dir", cfg.TaskDir().SharedAllocDir),
		fmt.Sprintf("%s secrets none rw,bind,create=dir", cfg.TaskDir().SecretsDir),
	}

	mounts = append(mounts, d.formatTaskMounts(cfg.Mounts)...)
	mounts = append(mounts, d.formatTaskDevices(cfg.Devices)...)

	volumesEnabled := d.config.AllowVolumes

	for _, volDesc := range taskConfig.Volumes {
		// the format was checked in Validate()
		paths := strings.Split(volDesc, ":")

		if filepath.IsAbs(paths[0]) {
			if !volumesEnabled {
				return nil, fmt.Errorf("absolute bind-mount volume in config but volumes are disabled")
			}
		} else {
			// Relative source paths are treated as relative to alloc dir
			paths[0] = filepath.Join(cfg.TaskDir().Dir, paths[0])
		}

		// LXC assumes paths are relative with respect to rootfs
		target := strings.TrimLeft(paths[1], "/")
		mounts = append(mounts, fmt.Sprintf("%s %s none rw,bind,create=dir", paths[0], target))
	}

	return mounts, nil

}

func (d *Driver) devicesCgroupEntries(cfg *drivers.TaskConfig) ([]string, error) {
	entries := make([]string, len(cfg.Devices))

	for i, d := range cfg.Devices {
		hd, err := ldevices.DeviceFromPath(d.HostPath, d.Permissions)
		if err != nil {
			return nil, err
		}

		entries[i] = hd.CgroupString()
	}

	return entries, nil
}

func (d *Driver) formatTaskMounts(mounts []*drivers.MountConfig) []string {
	result := make([]string, len(mounts))

	for i, m := range mounts {
		result[i] = d.formatMount(m.HostPath, m.TaskPath, m.Readonly)
	}

	return result
}

func (d *Driver) formatTaskDevices(devices []*drivers.DeviceConfig) []string {
	result := make([]string, len(devices))

	for i, m := range devices {
		result[i] = d.formatMount(m.HostPath, m.TaskPath,
			!strings.Contains(m.Permissions, "w"))
	}

	return result
}

func (d *Driver) formatMount(hostPath, taskPath string, readOnly bool) string {
	typ := "dir"
	s, err := os.Stat(hostPath)
	if err != nil {
		d.logger.Warn("failed to find mount host path type, defaulting to dir type", "path", hostPath, "error", err)
	} else if !s.IsDir() {
		typ = "file"
	}

	perm := "rw"
	if readOnly {
		perm = "ro"
	}

	// LXC assumes paths are relative with respect to rootfs
	target := strings.TrimLeft(taskPath, "/")
	return fmt.Sprintf("%s %s none %s,bind,create=%s", hostPath, target, perm, typ)
}

func (d *Driver) setResourceLimits(c *lxc.Container, cfg *drivers.TaskConfig) error {
	if err := c.SetMemoryLimit(lxc.ByteSize(cfg.Resources.NomadResources.Memory.MemoryMB) * lxc.MB); err != nil {
		return fmt.Errorf("unable to set memory limits: %v", err)
	}

	if err := c.SetCgroupItem("cpu.shares", strconv.FormatInt(cfg.Resources.LinuxResources.CPUShares, 10)); err != nil {
		return fmt.Errorf("unable to set cpu shares: %v", err)
	}

	return nil
}

func toLXCCreateOptions(taskConfig TaskConfig) lxc.TemplateOptions {
	return lxc.TemplateOptions{
		Template:             taskConfig.Template,
		Distro:               taskConfig.Distro,
		Release:              taskConfig.Release,
		Arch:                 taskConfig.Arch,
		FlushCache:           taskConfig.FlushCache,
		DisableGPGValidation: taskConfig.DisableGPGValidation,
		ExtraArgs:            taskConfig.TemplateArgs,
	}
}

// waitTillStopped blocks and returns true when container stops;
// returns false with an error message if the container processes cannot be identified.
//
// Use this in preference to c.Wait() - lxc Wait() function holds a write lock on the container
// blocking any other operation on container, including looking up container stats
func waitTillStopped(c *lxc.Container) (bool, error) {
	ps, err := os.FindProcess(c.InitPid())
	if err != nil {
		return false, err
	}

	for {
		if err := ps.Signal(syscall.Signal(0)); err != nil {
			return true, nil
		}

		time.Sleep(containerMonitorIntv)
	}
}
