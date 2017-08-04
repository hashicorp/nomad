//+build linux,lxc

package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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
}

// LxcDriverConfig is the configuration of the LXC Container
type LxcDriverConfig struct {
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
	LogLevel             string   `mapstructure:"log_level"`
	Verbosity            string
}

// NewLxcDriver returns a new instance of the LXC driver
func NewLxcDriver(ctx *DriverContext) Driver {
	return &LxcDriver{DriverContext: *ctx}
}

// Validate validates the lxc driver configuration
func (d *LxcDriver) Validate(config map[string]interface{}) error {
	fd := &fields.FieldData{
		Raw: config,
		Schema: map[string]*fields.FieldSchema{
			"template": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: true,
			},
			"distro": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: false,
			},
			"release": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: false,
			},
			"arch": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: false,
			},
			"image_variant": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: false,
			},
			"image_server": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: false,
			},
			"gpg_key_id": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: false,
			},
			"gpg_key_server": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: false,
			},
			"disable_gpg": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: false,
			},
			"flush_cache": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: false,
			},
			"force_cache": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: false,
			},
			"template_args": &fields.FieldSchema{
				Type:     fields.TypeArray,
				Required: false,
			},
			"log_level": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: false,
			},
			"verbosity": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: false,
			},
		},
	}

	if err := fd.Validate(); err != nil {
		return err
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
	return true, nil
}

func (d *LxcDriver) Prestart(*ExecContext, *structs.Task) (*PrestartResponse, error) {
	return nil, nil
}

// Start starts the LXC Driver
func (d *LxcDriver) Start(ctx *ExecContext, task *structs.Task) (*StartResponse, error) {
	var driverConfig LxcDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &driverConfig); err != nil {
		return nil, err
	}
	lxcPath := lxc.DefaultConfigPath()
	if path := d.config.Read("driver.lxc.path"); path != "" {
		lxcPath = path
	}

	containerName := fmt.Sprintf("%s-%s", task.Name, d.DriverContext.allocID)
	c, err := lxc.NewContainer(containerName, lxcPath)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize container: %v", err)
	}

	var verbosity lxc.Verbosity
	switch driverConfig.Verbosity {
	case "verbose":
		verbosity = lxc.Verbose
	case "", "quiet":
		verbosity = lxc.Quiet
	default:
		return nil, fmt.Errorf("lxc driver config 'verbosity' can only be either quiet or verbose")
	}
	c.SetVerbosity(verbosity)

	var logLevel lxc.LogLevel
	switch driverConfig.LogLevel {
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
		return nil, fmt.Errorf("lxc driver config 'log_level' can only be trace, debug, info, warn or error")
	}
	c.SetLogLevel(logLevel)

	logFile := filepath.Join(ctx.TaskDir.LogDir, fmt.Sprintf("%v-lxc.log", task.Name))
	c.SetLogFile(logFile)

	options := lxc.TemplateOptions{
		Template:             driverConfig.Template,
		Distro:               driverConfig.Distro,
		Release:              driverConfig.Release,
		Arch:                 driverConfig.Arch,
		FlushCache:           driverConfig.FlushCache,
		DisableGPGValidation: driverConfig.DisableGPGValidation,
		ExtraArgs:            driverConfig.TemplateArgs,
	}

	if err := c.Create(options); err != nil {
		return nil, fmt.Errorf("unable to create container: %v", err)
	}

	// Set the network type to none
	if err := c.SetConfigItem("lxc.network.type", "none"); err != nil {
		return nil, fmt.Errorf("error setting network type configuration: %v", err)
	}

	// Bind mount the shared alloc dir and task local dir in the container
	mounts := []string{
		fmt.Sprintf("%s local none rw,bind,create=dir", ctx.TaskDir.LocalDir),
		fmt.Sprintf("%s alloc none rw,bind,create=dir", ctx.TaskDir.SharedAllocDir),
		fmt.Sprintf("%s secrets none rw,bind,create=dir", ctx.TaskDir.SecretsDir),
	}
	for _, mnt := range mounts {
		if err := c.SetConfigItem("lxc.mount.entry", mnt); err != nil {
			return nil, fmt.Errorf("error setting bind mount %q error: %v", mnt, err)
		}
	}

	// Start the container
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("unable to start container: %v", err)
	}

	// Set the resource limits
	if err := c.SetMemoryLimit(lxc.ByteSize(task.Resources.MemoryMB) * lxc.MB); err != nil {
		return nil, fmt.Errorf("unable to set memory limits: %v", err)
	}
	if err := c.SetCgroupItem("cpu.shares", strconv.Itoa(task.Resources.CPU)); err != nil {
		return nil, fmt.Errorf("unable to set cpu shares: %v", err)
	}

	h := lxcDriverHandle{
		container:      c,
		initPid:        c.InitPid(),
		lxcPath:        lxcPath,
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

	return &StartResponse{Handle: &h}, nil
}

func (d *LxcDriver) Cleanup(*ExecContext, *CreatedResources) error { return nil }

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
			container = &c
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
	name := h.container.Name()

	h.logger.Printf("[INFO] driver.lxc: shutting down container %q", name)
	if err := h.container.Shutdown(h.killTimeout); err != nil {
		h.logger.Printf("[INFO] driver.lxc: shutting down container %q failed: %v", name, err)
		if err := h.container.Stop(); err != nil {
			h.logger.Printf("[ERR] driver.lxc: error stopping container %q: %v", name, err)
		}
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
				return
			}
			if err := process.Signal(syscall.Signal(0)); err != nil {
				h.waitCh <- &dstructs.WaitResult{}
				return
			}
			timer.Reset(containerMonitorIntv)
		case <-h.doneCh:
			h.waitCh <- &dstructs.WaitResult{}
			return
		}
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
