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

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/drivers"
	lxc "gopkg.in/lxc/go-lxc.v2"
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

	containerName := fmt.Sprintf("%s-%s", cfg.Name, uuid.Generate())
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
	// Bind mount the shared alloc dir and task local dir in the container
	mounts := []string{
		fmt.Sprintf("%s local none rw,bind,create=dir", cfg.TaskDir().LocalDir),
		fmt.Sprintf("%s alloc none rw,bind,create=dir", cfg.TaskDir().SharedAllocDir),
		fmt.Sprintf("%s secrets none rw,bind,create=dir", cfg.TaskDir().SecretsDir),
	}

	volumesEnabled := d.config.AllowVolumes

	for _, volDesc := range taskConfig.Volumes {
		// the format was checked in Validate()
		paths := strings.Split(volDesc, ":")

		if filepath.IsAbs(paths[0]) {
			if !volumesEnabled {
				return fmt.Errorf("absolute bind-mount volume in config but volumes are disabled")
			}
		} else {
			// Relative source paths are treated as relative to alloc dir
			paths[0] = filepath.Join(cfg.TaskDir().Dir, paths[0])
		}

		mounts = append(mounts, fmt.Sprintf("%s %s none rw,bind,create=dir", paths[0], paths[1]))
	}

	for _, mnt := range mounts {
		if err := c.SetConfigItem("lxc.mount.entry", mnt); err != nil {
			return fmt.Errorf("error setting bind mount %q error: %v", mnt, err)
		}
	}

	return nil
}

func (d *Driver) setResourceLimits(c *lxc.Container, cfg *drivers.TaskConfig) error {
	if err := c.SetMemoryLimit(lxc.ByteSize(cfg.Resources.NomadResources.MemoryMB) * lxc.MB); err != nil {
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
