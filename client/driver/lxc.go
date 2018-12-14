//+build linux,lxc

package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/client/stats"
	"github.com/hashicorp/nomad/helper/fields"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"

	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cstructs "github.com/hashicorp/nomad/client/structs"
	lxc "gopkg.in/lxc/go-lxc.v2"
)

const (
	// lxcConfigOption is the key for enabling the LXC driver in the
	// Config.Options map.
	lxcConfigOption = "driver.lxc.enable"

	// lxcVolumesConfigOption is the key for enabling the use of
	// custom bind volumes to arbitrary host paths
	lxcVolumesConfigOption  = "lxc.volumes.enabled"
	lxcVolumesConfigDefault = true

	// containerMonitorIntv is the interval at which the driver checks if the
	// container is still alive
	containerMonitorIntv = 2 * time.Second
)

var (
	LXCMeasuredCpuStats = []string{"System Mode", "User Mode", "Percent"}

	LXCMeasuredMemStats = []string{"RSS", "Cache", "Swap", "Max Usage", "Kernel Usage", "Kernel Max Usage"}
)

// Add the lxc driver to the list of builtin drivers
func init() {
	BuiltinDrivers["lxc"] = NewLxcDriver
}

// LxcDriver allows users to run LXC Containers
type LxcDriver struct {
	DriverContext
	fingerprint.StaticFingerprinter
	lxcPath string
}

// LxcCommonDriverConfig is configuration that's common between
// container types - new containers created with Create and run with
// Start; and containers created from a rootfs clone and started using
// StartExecute.
type LxcCommonDriverConfig struct {
	LogLevel   string              `mapstructure:"log_level"`
	Verbosity  string              `mapstructure:"verbosity"`
	UseExecute bool                `mapstructure:"use_execute"`
	Volumes    []map[string]string `mapstructure:"volumes"`
}

// LxcStartDriverConfig is the configuration for containers that will
// be created with Create and run with Start
type LxcStartDriverConfig struct {
	Template             string
	Distro               string
	Release              string
	Arch                 string
	ImageVariant         string   `mapstructure:"image_variant"`
	ImageServer          string   `mapstructure:"image_server"`
	GPGKeyID             string   `mapstructure:"gpg_key_id"`
	GPGKeyServer         string   `mapstructure:"gpg_key_server"`
	DisableGPGValidation bool     `mapstructure:"disable_gpg"`
	FlushCache           bool     `mapstructure:"flush_cache"`
	ForceCache           bool     `mapstructure:"force_cache"`
	TemplateArgs         []string `mapstructure:"template_args"`
	LxcCommonDriverConfig
}

// LxcExecuteDriverConfig is configuration for containers that will be
// created by cloning a rootfs and run using StartExecute
type LxcExecuteDriverConfig struct {
	LxcCommonDriverConfig
	BaseRootFsPath string   `mapstructure:"base_rootfs_path"`
	BaseConfigPath string   `mapstructure:"base_config_path"`
	CmdArgs        []string `mapstructure:"cmd_args"`
}

// NewLxcDriver returns a new instance of the LXC driver
func NewLxcDriver(ctx *DriverContext) Driver {
	d := &LxcDriver{DriverContext: *ctx}
	return d
}

// Validate validates the lxc driver configuration
func (d *LxcDriver) Validate(config map[string]interface{}) error {
	commonFieldSchema := map[string]*fields.FieldSchema{
		"log_level": {
			Type:     fields.TypeString,
			Required: false,
		},
		"verbosity": {
			Type:     fields.TypeString,
			Required: false,
		},
		"use_execute": {
			Type:     fields.TypeBool,
			Required: false,
		},
		"volumes": {
			Type:     fields.TypeArray,
			Required: false,
		},
	}
	fd := &fields.FieldData{
		Raw: config,
		Schema: map[string]*fields.FieldSchema{
			"template": {
				Type:     fields.TypeString,
				Required: true,
			},
			"distro": {
				Type:     fields.TypeString,
				Required: false,
			},
			"release": {
				Type:     fields.TypeString,
				Required: false,
			},
			"arch": {
				Type:     fields.TypeString,
				Required: false,
			},
			"image_variant": {
				Type:     fields.TypeString,
				Required: false,
			},
			"image_server": {
				Type:     fields.TypeString,
				Required: false,
			},
			"gpg_key_id": {
				Type:     fields.TypeString,
				Required: false,
			},
			"gpg_key_server": {
				Type:     fields.TypeString,
				Required: false,
			},
			"disable_gpg": {
				Type:     fields.TypeString,
				Required: false,
			},
			"flush_cache": {
				Type:     fields.TypeString,
				Required: false,
			},
			"force_cache": {
				Type:     fields.TypeString,
				Required: false,
			},
			"template_args": {
				Type:     fields.TypeArray,
				Required: false,
			},
		},
	}
	for k, v := range commonFieldSchema {
		fd.Schema[k] = v
	}

	execFd := &fields.FieldData{
		Raw: config,
		Schema: map[string]*fields.FieldSchema{
			"base_config_path": {
				Type:     fields.TypeString,
				Required: true,
			},
			"base_rootfs_path": {
				Type:     fields.TypeString,
				Required: true,
			},
			"cmd_args": {
				Type:     fields.TypeArray,
				Required: false,
				Default:  []string{},
			},
		},
	}

	for k, v := range commonFieldSchema {
		execFd.Schema[k] = v
	}

	// default is to use lxc-start
	useExecute := false
	execConfig, ok := config["use_execute"]
	if ok {
		useExecute, ok = execConfig.(bool)
		if !ok {
			return fmt.Errorf("invalid value for use_execute config: %v", execConfig)
		}
	}
	var volumes interface{}
	if useExecute {
		if err := execFd.Validate(); err != nil {
			return err
		}
		volumes, ok = execFd.GetOk("volumes")
	} else {
		if err := fd.Validate(); err != nil {
			return err
		}
		volumes, ok = fd.GetOk("volumes")
	}
	if ok {
		return d.validateVolumesConfig(volumes.([]interface{}))
	} else {
		return nil
	}
}

func (d *LxcDriver) validateVolumesConfig(volumes []interface{}) error {
	for _, volSpecI := range volumes {
		volSpec := volSpecI.(map[string]interface{})
		_, sourcePresent := volSpec["source"]
		dest, destPresent := volSpec["dest"]
		if !sourcePresent || !destPresent {
			return fmt.Errorf("invalid volume bind mount entry: empty src or dest %v", volSpec)
		}

		if dest.(string)[0] == '/' {
			return fmt.Errorf("unsupported absolute container mount point: '%s'", dest)
		}
		mode, modePresent := volSpec["mode"]
		if modePresent && mode.(string) != "ro" && mode.(string) != "rw" {
			return fmt.Errorf("invalid mode in bind vol: '%s'", mode.(string))
		}
	}
	return nil
}

func (d *LxcDriver) Abilities() DriverAbilities {
	return DriverAbilities{
		SendSignals: false,
		Exec:        false,
	}
}

func (d *LxcDriver) FSIsolation() cstructs.FSIsolation {
	return cstructs.FSIsolationImage
}

// Fingerprint fingerprints the lxc driver configuration
func (d *LxcDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	enabled := cfg.ReadBoolDefault(lxcConfigOption, true)
	if !enabled && !cfg.DevMode {
		return false, nil
	}
	version := lxc.Version()
	if version == "" {
		return false, nil
	}
	node.Attributes["driver.lxc.version"] = version
	node.Attributes["driver.lxc"] = "1"

	// Advertise if this node supports lxc volumes
	if d.config.ReadBoolDefault(lxcVolumesConfigOption, lxcVolumesConfigDefault) {
		node.Attributes["driver."+lxcVolumesConfigOption] = "1"
	}

	node.Attributes["driver.lxc.execute"] = "true"

	return true, nil
}

func (d *LxcDriver) Prestart(_ *ExecContext, task *structs.Task) (*PrestartResponse, error) {
	return nil, nil
}

func (d *LxcDriver) getContainerName(task *structs.Task) string {
	return fmt.Sprintf("%s-%s", task.Name, d.DriverContext.allocID)
}

// Start starts the LXC Driver
func (d *LxcDriver) Start(ctx *ExecContext, task *structs.Task) (*StartResponse, error) {
	sresp, err, errCleanup := d.startWithCleanup(ctx, task)
	if err != nil {
		if cleanupErr := errCleanup(); cleanupErr != nil {
			d.logger.Printf("[ERR] error occurred while cleaning up from error in Start: %v", cleanupErr)
		}
	}
	return sresp, err
}

func (d *LxcDriver) startWithCleanup(ctx *ExecContext, task *structs.Task) (*StartResponse, error, func() error) {
	noCleanup := func() error { return nil }

	var commonConfig LxcCommonDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &commonConfig); err != nil {
		return nil, err, noCleanup
	}

	d.lxcPath = lxc.DefaultConfigPath()
	if path := d.config.Read("driver.lxc.path"); path != "" {
		d.lxcPath = path
	}

	containerName := d.getContainerName(task)
	c, err := lxc.NewContainer(containerName, d.lxcPath)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize container: %v", err), noCleanup
	}

	var verbosity lxc.Verbosity
	switch commonConfig.Verbosity {
	case "verbose":
		verbosity = lxc.Verbose
	case "", "quiet":
		verbosity = lxc.Quiet
	default:
		return nil, fmt.Errorf("lxc driver config 'verbosity' can only be either quiet or verbose"), noCleanup
	}
	c.SetVerbosity(verbosity)

	var logLevel lxc.LogLevel
	switch commonConfig.LogLevel {
	case "trace":
		logLevel = lxc.TRACE
	case "debug":
		logLevel = lxc.DEBUG
	case "info":
		logLevel = lxc.INFO
	case "warn":
		logLevel = lxc.WARN
	case "", "error":
		logLevel = lxc.ERROR
	default:
		return nil, fmt.Errorf("lxc driver config 'log_level' can only be trace, debug, info, warn or error"), noCleanup
	}
	c.SetLogLevel(logLevel)

	logFile := filepath.Join(ctx.TaskDir.Dir, fmt.Sprintf("%v-lxc.log", task.Name))
	c.SetLogFile(logFile)

	if commonConfig.UseExecute {
		d.logger.Printf("[INFO] Using lxc-execute to start application container")
		return d.executeContainer(ctx, c, task, &commonConfig)
	} else {
		d.logger.Printf("[INFO] Using lxc-start to start system container")
		return d.startContainer(ctx, c, task, &commonConfig)
	}
}

func (d *LxcDriver) startContainer(ctx *ExecContext, c *lxc.Container, task *structs.Task, commonConfig *LxcCommonDriverConfig) (*StartResponse, error, func() error) {
	noCleanup := func() error { return nil }

	var startConfig LxcStartDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &startConfig); err != nil {
		return nil, err, noCleanup
	}

	options := lxc.TemplateOptions{
		Template:             startConfig.Template,
		Distro:               startConfig.Distro,
		Release:              startConfig.Release,
		Arch:                 startConfig.Arch,
		FlushCache:           startConfig.FlushCache,
		DisableGPGValidation: startConfig.DisableGPGValidation,
		ExtraArgs:            startConfig.TemplateArgs,
	}

	if err := c.Create(options); err != nil {
		return nil, fmt.Errorf("unable to create container: %v", err), noCleanup
	}

	if err := d.setCommonContainerConfig(ctx, c, commonConfig); err != nil {
		return nil, err, c.Destroy
	}

	// Start the container
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("unable to start container: %v", err), c.Destroy
	}

	stopAndDestroyCleanup := func() error {
		if err := c.Stop(); err != nil {
			return err
		}
		return c.Destroy()
	}

	if err := setLimitsOnContainer(c, task); err != nil {
		return nil, err, stopAndDestroyCleanup
	}

	h := lxcDriverHandle{
		container:      c,
		initPid:        c.InitPid(),
		lxcPath:        d.lxcPath,
		logger:         d.logger,
		killTimeout:    GetKillTimeout(task.KillTimeout, d.DriverContext.config.MaxKillTimeout),
		maxKillTimeout: d.DriverContext.config.MaxKillTimeout,
		totalCpuStats:  stats.NewCpuStats(),
		userCpuStats:   stats.NewCpuStats(),
		systemCpuStats: stats.NewCpuStats(),
		waitCh:         make(chan *dstructs.WaitResult, 1),
		doneCh:         make(chan bool, 1),
	}

	go h.run()

	return &StartResponse{Handle: &h}, nil, noCleanup
}

func (d *LxcDriver) executeContainer(ctx *ExecContext, c *lxc.Container, task *structs.Task, commonConfig *LxcCommonDriverConfig) (*StartResponse, error, func() error) {
	noCleanup := func() error { return nil }
	var executeConfig LxcExecuteDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &executeConfig); err != nil {
		return nil, err, noCleanup
	}

	containerPath := filepath.Join(d.lxcPath, c.Name())
	containerRootfsPath := filepath.Join(containerPath, "rootfs")
	if err := os.MkdirAll(containerRootfsPath, 0711); err != nil {
		return nil, fmt.Errorf("unable to create container directory at %s", containerPath), noCleanup
	}

	if executeConfig.BaseRootFsPath[:4] != "lvm:" {
		return nil, fmt.Errorf("only LVM is supported as a base to clone from"), noCleanup
	}

	baseLvName := executeConfig.BaseRootFsPath[4:]
	lvCreateCmd := exec.Command("lvcreate", "-kn", "-n", c.Name(), "-s", baseLvName)
	if err := lvCreateCmd.Run(); err != nil {
		return nil, fmt.Errorf("could not create thin pool snapshot with cmd '%v': %v: %s", lvCreateCmd.Args, err, err.(*exec.ExitError).Stderr), noCleanup
	}

	vgName, err := extractVgName(baseLvName)
	if err != nil {
		return nil, fmt.Errorf("Could not parse LVM Volume Group name from '%s'", baseLvName), noCleanup
	}

	removeLVCleanup := func() error {
		lvRemoveCmd := exec.Command("lvremove", "-f", fmt.Sprintf("%s/%s", vgName, c.Name()))
		if err := lvRemoveCmd.Run(); err != nil {
			return fmt.Errorf("could not remove thin pool snapshot with cmd '%v': %v: %s", lvRemoveCmd.Args, err, err.(*exec.ExitError).Stderr)
		}
		return nil
	}

	tr := func(s string) string {
		return strings.Replace(s, "-", "--", -1)
	}

	storageName := fmt.Sprintf("lvm:/dev/mapper/%s-%s", tr(vgName), tr(c.Name()))

	configTemplate := struct {
		RootFSPath    string
		ContainerName string
	}{
		storageName,
		c.Name(),
	}

	initialConfigFilePath := filepath.Join(d.lxcPath, c.Name(), "config.initial")
	finalConfigFilePath := filepath.Join(d.lxcPath, c.Name(), "config")
	initialConfigFile, err := os.Create(initialConfigFilePath)
	if err != nil {
		return nil, fmt.Errorf("unable to create new config file '%s': %v", initialConfigFilePath, err), removeLVCleanup
	}

	if err := os.Chmod(initialConfigFilePath, 0777); err != nil {
		return nil, fmt.Errorf("unable to change permissions on config file"), removeLVCleanup
	}

	tmpl, err := template.ParseFiles(executeConfig.BaseConfigPath)
	if err != nil {
		return nil, fmt.Errorf("unable to parse config template in '%v': %v",
			executeConfig.BaseConfigPath, err), removeLVCleanup
	}

	if err := tmpl.Execute(initialConfigFile, configTemplate); err != nil {
		initialConfigFile.Close()
		return nil, fmt.Errorf("Error executing config file template: %v", err), removeLVCleanup
	}
	initialConfigFile.Close()

	d.logger.Printf("[INFO] %s initial config path is %s, final config path is %s", c.Name(), initialConfigFilePath, finalConfigFilePath)
	if err := c.LoadConfigFile(initialConfigFilePath); err != nil {
		return nil, fmt.Errorf("unable to read config file for container: %v", err), removeLVCleanup
	}

	if err := d.setCommonContainerConfig(ctx, c, commonConfig); err != nil {
		return nil, err, removeLVCleanup
	}

	// Replace any env vars in Cmd with value from Nomad env:
	parsedArgs := ctx.TaskEnv.ParseAndReplace(executeConfig.CmdArgs)
	d.logger.Printf("[INFO] env vars substituted in command \"%#v\" - new command is \"%#v\"", executeConfig.CmdArgs, parsedArgs)

	if err := c.SetConfigItem("lxc.execute.cmd", strings.Join(parsedArgs, " ")); err != nil {
		return nil, fmt.Errorf("unable to set final parsed command to \"%s\"", parsedArgs), removeLVCleanup
	}

	savedConfigFile := filepath.Join(ctx.TaskDir.Dir, fmt.Sprintf("%v-lxc.config", task.Name))
	for _, fp := range []string{finalConfigFilePath, savedConfigFile} {
		if err := c.SaveConfigFile(fp); err != nil {
			return nil, fmt.Errorf("unable to write final config file to \"%s\"", fp), removeLVCleanup
		}
	}

	if err := c.StartExecute(parsedArgs); err != nil {
		return nil, fmt.Errorf("unable to execute with args '%v': %v", parsedArgs, err), removeLVCleanup
	}

	stopAndRemoveLVCleanup := func() error {
		removeLVCleanup()
		if err := c.Stop(); err != nil {
			return err
		}
		return nil
	}

	if err := setLimitsOnContainer(c, task); err != nil {
		return nil, err, stopAndRemoveLVCleanup
	}

	h := lxcDriverHandle{
		container:      c,
		initPid:        c.InitPid(),
		lxcPath:        d.lxcPath,
		logger:         d.logger,
		killTimeout:    GetKillTimeout(task.KillTimeout, d.DriverContext.config.MaxKillTimeout),
		maxKillTimeout: d.DriverContext.config.MaxKillTimeout,
		totalCpuStats:  stats.NewCpuStats(),
		userCpuStats:   stats.NewCpuStats(),
		systemCpuStats: stats.NewCpuStats(),
		waitCh:         make(chan *dstructs.WaitResult, 1),
		doneCh:         make(chan bool, 1),
	}

	go h.run()

	return &StartResponse{Handle: &h}, nil, noCleanup

}

func extractVgName(baseLvName string) (string, error) {
	vgName := ""
	devMapperRE := regexp.MustCompile("/dev/mapper/([a-zA-Z0-9_+.-]*[^-])-{1}[^-]")
	if !strings.HasPrefix(baseLvName, "/") {
		// vgname/lvname
		c := strings.Split(baseLvName, "/")
		if len(c) != 2 {
			return "", fmt.Errorf("unexpected number of components in baseLvName '%s': %d", baseLvName, len(c))
		}
		vgName = c[0]
	} else {
		// /dev/mapper/vg--name-lv--name
		matches := devMapperRE.FindAllStringSubmatch(baseLvName, 1)
		if matches != nil {
			vgName = matches[0][1]
		} else {
			if strings.HasPrefix(baseLvName, "/dev/") {
				// /dev/vg/lv
				vgName = baseLvName[len("/dev/"):strings.LastIndex(baseLvName, "/")]
			}
		}
	}
	if len(vgName) == 0 {
		return "", fmt.Errorf("could not parse volume group name from '%v':, baseLvName")
	}
	return vgName, nil
}

func (d *LxcDriver) setCommonContainerConfig(ctx *ExecContext, c *lxc.Container, commonConfig *LxcCommonDriverConfig) error {
	// Set the network type to none
	if err := c.SetConfigItem("lxc.net.0.type", "none"); err != nil {
		return fmt.Errorf("error setting network type configuration: %v", err)
	}

	// Bind mount the shared alloc dir and task local dir in the container
	mounts := []string{
		fmt.Sprintf("%s local none rw,bind,create=dir", ctx.TaskDir.LocalDir),
		fmt.Sprintf("%s alloc none rw,bind,create=dir", ctx.TaskDir.SharedAllocDir),
		fmt.Sprintf("%s secrets none rw,bind,create=dir", ctx.TaskDir.SecretsDir),
	}
	for _, mnt := range mounts {
		if err := c.SetConfigItem("lxc.mount.entry", mnt); err != nil {
			return fmt.Errorf("error setting bind mount %q error: %v", mnt, err)
		}
	}

	volumesEnabled := d.config.ReadBoolDefault(lxcVolumesConfigOption, lxcVolumesConfigDefault)

	for _, volSpec := range commonConfig.Volumes {
		mode, modePresent := volSpec["mode"]

		if !modePresent {
			mode = "rw"
		}

		srcPath := volSpec["source"]
		if filepath.IsAbs(srcPath) {
			if !volumesEnabled {
				return fmt.Errorf("absolute bind-mount volume in config but '%v' is false", lxcVolumesConfigOption)
			}
		} else {
			// Relative source paths are treated as relative to alloc dir
			srcPath = filepath.Join(ctx.TaskDir.Dir, srcPath)
		}

		createType := "dir"
		srcFileInfo, err := os.Stat(srcPath)
		if err != nil {
			return fmt.Errorf("couldn't Stat src='%s' in mount directive: %v", srcPath, err)
		}

		if srcFileInfo.Mode().IsRegular() {
			createType = "file"
		}

		mounts = append(mounts, fmt.Sprintf("%s %s none %s,bind,create=%s", srcPath, volSpec["dest"], mode, createType))
	}

	for _, mnt := range mounts {
		if err := c.SetConfigItem("lxc.mount.entry", mnt); err != nil {
			return fmt.Errorf("error setting bind mount %q error: %v", mnt, err)
		}
	}

	for _, envVar := range ctx.TaskEnv.List() {
		if err := c.SetConfigItem("lxc.environment", envVar); err != nil {
			return fmt.Errorf("error setting environment variable '%s': %v", envVar, err)
		}
	}

	return nil
}

func setLimitsOnContainer(c *lxc.Container, task *structs.Task) error {
	// Set the resource limits
	if err := c.SetMemoryLimit(lxc.ByteSize(task.Resources.MemoryMB) * lxc.MB); err != nil {
		return fmt.Errorf("unable to set memory limits: %v", err)
	}
	if err := c.SetCgroupItem("cpu.shares", strconv.Itoa(task.Resources.CPU)); err != nil {
		return fmt.Errorf("unable to set cpu shares: %v", err)
	}
	return nil
}

func (d *LxcDriver) Cleanup(ctx *ExecContext, resources *CreatedResources) error {

	return nil
}

// Open creates the driver to monitor an existing LXC container
func (d *LxcDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	pid := &lxcPID{}
	if err := json.Unmarshal([]byte(handleID), pid); err != nil {
		return nil, fmt.Errorf("Failed to parse handle '%s': %v", handleID, err)
	}

	var container *lxc.Container
	containers := lxc.Containers(pid.LxcPath)
	for _, c := range containers {
		if c.Name() == pid.ContainerName {
			container = c
			break
		}
	}

	if container == nil {
		return nil, fmt.Errorf("container %v not found", pid.ContainerName)
	}

	handle := lxcDriverHandle{
		container:      container,
		initPid:        container.InitPid(),
		lxcPath:        pid.LxcPath,
		logger:         d.logger,
		killTimeout:    pid.KillTimeout,
		maxKillTimeout: d.DriverContext.config.MaxKillTimeout,
		totalCpuStats:  stats.NewCpuStats(),
		userCpuStats:   stats.NewCpuStats(),
		systemCpuStats: stats.NewCpuStats(),
		waitCh:         make(chan *dstructs.WaitResult, 1),
		doneCh:         make(chan bool, 1),
	}
	go handle.run()

	return &handle, nil
}

// lxcDriverHandle allows controlling the lifecycle of an lxc container
type lxcDriverHandle struct {
	container *lxc.Container
	initPid   int
	lxcPath   string

	logger *log.Logger

	killTimeout    time.Duration
	maxKillTimeout time.Duration

	totalCpuStats  *stats.CpuStats
	userCpuStats   *stats.CpuStats
	systemCpuStats *stats.CpuStats

	waitCh chan *dstructs.WaitResult
	doneCh chan bool
}

type lxcPID struct {
	ContainerName string
	InitPid       int
	LxcPath       string
	KillTimeout   time.Duration
}

func (h *lxcDriverHandle) ID() string {
	pid := lxcPID{
		ContainerName: h.container.Name(),
		InitPid:       h.initPid,
		LxcPath:       h.lxcPath,
		KillTimeout:   h.killTimeout,
	}
	data, err := json.Marshal(pid)
	if err != nil {
		h.logger.Printf("[ERR] driver.lxc: failed to marshal lxc PID to JSON: %v", err)
	}
	return string(data)
}

func (h *lxcDriverHandle) WaitCh() chan *dstructs.WaitResult {
	return h.waitCh
}

func (h *lxcDriverHandle) Update(task *structs.Task) error {
	h.killTimeout = GetKillTimeout(task.KillTimeout, h.killTimeout)
	return nil
}

func (h *lxcDriverHandle) Exec(ctx context.Context, cmd string, args []string) ([]byte, int, error) {
	return nil, 0, fmt.Errorf("lxc driver cannot execute commands")
}

func (h *lxcDriverHandle) Kill() error {
	if h.container.Running() {
		name := h.container.Name()
		if err := h.container.Stop(); err != nil {
			h.logger.Printf("[WARN] driver.lxc: error stopping container %q: %v", name, err)
			return fmt.Errorf("could not stop container: %v", err)
		}
		h.logger.Printf("[INFO] driver.lxc: stopped running container %q", name)
	}

	close(h.doneCh)
	return nil
}

func (h *lxcDriverHandle) Signal(s os.Signal) error {
	return fmt.Errorf("LXC does not support signals")
}

func (h *lxcDriverHandle) Stats() (*cstructs.TaskResourceUsage, error) {
	cpuStats, err := h.container.CPUStats()
	if err != nil {
		return nil, nil
	}
	total, err := h.container.CPUTime()
	if err != nil {
		return nil, nil
	}

	t := time.Now()

	// Get the cpu stats
	system := cpuStats["system"]
	user := cpuStats["user"]
	cs := &cstructs.CpuStats{
		SystemMode: h.systemCpuStats.Percent(float64(system)),
		UserMode:   h.systemCpuStats.Percent(float64(user)),
		Percent:    h.totalCpuStats.Percent(float64(total)),
		TotalTicks: float64(user + system),
		Measured:   LXCMeasuredCpuStats,
	}

	// Get the Memory Stats
	memData := map[string]uint64{
		"rss":   0,
		"cache": 0,
		"swap":  0,
	}
	rawMemStats := h.container.CgroupItem("memory.stat")
	for _, rawMemStat := range rawMemStats {
		key, val, err := keysToVal(rawMemStat)
		if err != nil {
			h.logger.Printf("[ERR] driver.lxc: error getting stat for line %q", rawMemStat)
			continue
		}
		if _, ok := memData[key]; ok {
			memData[key] = val

		}
	}
	ms := &cstructs.MemoryStats{
		RSS:      memData["rss"],
		Cache:    memData["cache"],
		Swap:     memData["swap"],
		Measured: LXCMeasuredMemStats,
	}

	mu := h.container.CgroupItem("memory.max_usage_in_bytes")
	for _, rawMemMaxUsage := range mu {
		val, err := strconv.ParseUint(rawMemMaxUsage, 10, 64)
		if err != nil {
			h.logger.Printf("[ERR] driver.lxc: unable to get max memory usage: %v", err)
			continue
		}
		ms.MaxUsage = val
	}
	ku := h.container.CgroupItem("memory.kmem.usage_in_bytes")
	for _, rawKernelUsage := range ku {
		val, err := strconv.ParseUint(rawKernelUsage, 10, 64)
		if err != nil {
			h.logger.Printf("[ERR] driver.lxc: unable to get kernel memory usage: %v", err)
			continue
		}
		ms.KernelUsage = val
	}

	mku := h.container.CgroupItem("memory.kmem.max_usage_in_bytes")
	for _, rawMaxKernelUsage := range mku {
		val, err := strconv.ParseUint(rawMaxKernelUsage, 10, 64)
		if err != nil {
			h.logger.Printf("[ERR] driver.lxc: unable to get max kernel memory usage: %v", err)
			continue
		}
		ms.KernelMaxUsage = val
	}

	taskResUsage := cstructs.TaskResourceUsage{
		ResourceUsage: &cstructs.ResourceUsage{
			CpuStats:    cs,
			MemoryStats: ms,
		},
		Timestamp: t.UTC().UnixNano(),
	}

	return &taskResUsage, nil
}

func (h *lxcDriverHandle) run() {
	defer close(h.waitCh)
	timer := time.NewTimer(containerMonitorIntv)
	for {
		select {
		case <-timer.C:
			process, err := os.FindProcess(h.initPid)
			if err != nil {
				h.waitCh <- &dstructs.WaitResult{Err: err}
				goto DESTROY
			}
			if err := process.Signal(syscall.Signal(0)); err != nil {
				h.waitCh <- &dstructs.WaitResult{}
				goto DESTROY
			}
			timer.Reset(containerMonitorIntv)
		case <-h.doneCh:
			h.waitCh <- &dstructs.WaitResult{}
			goto DESTROY
		}
	}

DESTROY:
	name := h.container.Name()
	if err := h.container.Destroy(); err != nil {
		h.logger.Printf("[ERR] driver.lxc: error destroying container %q: %v.", name, err)
	} else {
		h.logger.Printf("[INFO] driver.lxc: successfully destroyed container %q.", name)
	}
}

func keysToVal(line string) (string, uint64, error) {
	tokens := strings.Split(line, " ")
	if len(tokens) != 2 {
		return "", 0, fmt.Errorf("line isn't a k/v pair")
	}
	key := tokens[0]
	val, err := strconv.ParseUint(tokens[1], 10, 64)
	return key, val, err
}
