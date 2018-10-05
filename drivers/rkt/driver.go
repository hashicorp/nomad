package rkt

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/driver/executor"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/utils"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

const (
	// pluginName is the name of the plugin
	pluginName = "rkt"

	// fingerprintPeriod is the interval at which the driver will send fingerprint responses
	fingerprintPeriod = 30 * time.Second

	// minRktVersion is the earliest supported version of rkt. rkt added support
	// for CPU and memory isolators in 0.14.0. We cannot support an earlier
	// version to maintain an uniform interface across all drivers
	minRktVersion = "1.27.0"

	// rktCmd is the command rkt is installed as.
	rktCmd = "rkt"

	// rktNetworkDeadline is how long to wait for container network to start
	rktNetworkDeadline = 1 * time.Minute
)

var (
	// pluginInfo is the response returned for the PluginInfo RPC
	pluginInfo = &base.PluginInfoResponse{
		Type:             base.PluginTypeDriver,
		PluginApiVersion: "0.0.1",
		PluginVersion:    "0.1.0",
		Name:             pluginName,
	}

	// configSpec is the hcl specification returned by the ConfigSchema RPC
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"volumes_enabled": hclspec.NewDefault(
			hclspec.NewAttr("volumes_enabled", "bool", false),
			hclspec.NewLiteral("true"),
		),
	})

	// taskConfigSpec is the hcl specification for the driver config section of
	// a task within a job. It is returned in the TaskConfigSchema RPC
	taskConfigSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"image":              hclspec.NewAttr("image", "string", true),
		"command":            hclspec.NewAttr("command", "string", true),
		"args":               hclspec.NewAttr("command", "list(string)", false),
		"trust_prefix":       hclspec.NewAttr("trust_prefix", "string", false),
		"dns_servers":        hclspec.NewAttr("dns_servers", "list(string)", false),
		"dns_search_domains": hclspec.NewAttr("dns_search_domains", "list(string)", false),
		"net":                hclspec.NewAttr("net", "list(string)", false),
		"port_map":           hclspec.NewAttr("port_map", "map(string)", false),
		"volumes":            hclspec.NewAttr("volumes", "list(string)", false),
		"insecure_options":   hclspec.NewAttr("insecure_options", "list(string)", false),
		"no_overlay":         hclspec.NewAttr("no_overlay", "bool", false),
		"debug":              hclspec.NewAttr("debug", "bool", false),
		"group":              hclspec.NewAttr("group", "string", false),
	})

	// capabilities is returned by the Capabilities RPC and indicates what
	// optional features this driver supports
	capabilities = &drivers.Capabilities{
		SendSignals: true,
		Exec:        true,
		FSIsolation: drivers.FSIsolationChroot,
	}

	reRktVersion  = regexp.MustCompile(`rkt [vV]ersion[:]? (\d[.\d]+)`)
	reAppcVersion = regexp.MustCompile(`appc [vV]ersion[:]? (\d[.\d]+)`)
)

// Config is the client configuration for the driver
type Config struct {
	// VolumesEnabled allows tasks to bind host paths (volumes) inside their
	// container. Binding relative paths is always allowed and will be resolved
	// relative to the allocation's directory.
	VolumesEnabled bool `codec:"volumes_enabled"`
}

// TaskConfig is the driver configuration of a task within a job
type TaskConfig struct {
	ImageName        string            `codec:"image"`
	Command          string            `codec:"command"`
	Args             []string          `codec:"args"`
	TrustPrefix      string            `codec:"trust_prefix"`
	DNSServers       []string          `codec:"dns_servers"`        // DNS Server for containers
	DNSSearchDomains []string          `codec:"dns_search_domains"` // DNS Search domains for containers
	Net              []string          `codec:"net"`                // Networks for the containers
	PortMap          map[string]string `codec:"port_map"`           // A map of host port and the port name defined in the image manifest file
	Volumes          []string          `codec:"volumes"`            // Host-Volumes to mount in, syntax: /path/to/host/directory:/destination/path/in/container[:readOnly]
	InsecureOptions  []string          `codec:"insecure_options"`   // list of args for --insecure-options

	NoOverlay bool   `codec:"no_overlay"` // disable overlayfs for rkt run
	Debug     bool   `codec:"debug"`      // Enable debug option for rkt command
	Group     string `codec:"group"`      // Group override for the container
}

// Driver is a driver for running images via Rkt
// We attempt to chose sane defaults for now, with more configuration available
// planned in the future
type Driver struct {
	// eventer is used to handle multiplexing of TaskEvents calls such that an
	// event can be broadcast to all callers
	eventer *utils.Eventer

	// config is the driver configuration set by the SetConfig RPC
	config *Config

	// tasks is the in memory datastore mapping taskIDs to execDriverHandles
	tasks *taskStore

	// ctx is the context for the driver. It is passed to other subsystems to
	// coordinate shutdown
	ctx context.Context

	// signalShutdown is called when the driver is shutting down and cancels the
	// ctx passed to any subsystems
	signalShutdown context.CancelFunc

	// logger will log to the plugin output which is usually an 'executor.out'
	// file located in the root of the TaskDir
	logger hclog.Logger
}

func NewDriver(logger hclog.Logger) *Driver {

	return nil
}

func (d *Driver) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

func (d *Driver) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

func (d *Driver) SetConfig(data []byte) error {
	var config Config
	if err := base.MsgPackDecode(data, &config); err != nil {
		return err
	}

	d.config = &config
	return nil
}

func (d *Driver) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

func (d *Driver) Capabilities() (*drivers.Capabilities, error) {
	return capabilities, nil
}

func (d *Driver) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	ch := make(chan *drivers.Fingerprint)
	go d.handleFingerprint(ctx, ch)
	return ch, nil

}
func (d *Driver) handleFingerprint(ctx context.Context, ch chan *drivers.Fingerprint) {
	ticker := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			ticker.Reset(fingerprintPeriod)
			ch <- d.buildFingerprint()
		}
	}
}

func (d *Driver) buildFingerprint() *drivers.Fingerprint {
	fingerprint := &drivers.Fingerprint{
		Attributes:        map[string]string{},
		Health:            drivers.HealthStateHealthy,
		HealthDescription: "healthy",
	}

	// Only enable if we are root when running on non-windows systems.
	if runtime.GOOS != "windows" && syscall.Geteuid() != 0 {
		d.logger.Debug("must run as root user, disabling")
		fingerprint.Health = drivers.HealthStateUndetected
		fingerprint.HealthDescription = "driver must run as root user"
		return fingerprint
	}

	outBytes, err := exec.Command(rktCmd, "version").Output()
	if err != nil {
		fingerprint.Health = drivers.HealthStateUndetected
		fingerprint.HealthDescription = fmt.Sprintf("failed to executor %s version: %v", rktCmd, err)
		return fingerprint
	}
	out := strings.TrimSpace(string(outBytes))

	rktMatches := reRktVersion.FindStringSubmatch(out)
	appcMatches := reAppcVersion.FindStringSubmatch(out)
	if len(rktMatches) != 2 || len(appcMatches) != 2 {
		fingerprint.Health = drivers.HealthStateUndetected
		fingerprint.HealthDescription = "unable to parse rkt version string"
		return fingerprint
	}

	minVersion, _ := version.NewVersion(minRktVersion)
	currentVersion, _ := version.NewVersion(rktMatches[1])
	if currentVersion.LessThan(minVersion) {
		// Do not allow ancient rkt versions
		fingerprint.Health = drivers.HealthStateUndetected
		fingerprint.HealthDescription = fmt.Sprintf("unsuported rkt version %s", currentVersion)
		d.logger.Warn("unsupported rkt version please upgrade to >= "+minVersion.String(),
			"rkt_version", currentVersion)
		return fingerprint
	}

	fingerprint.Attributes["driver.rkt"] = "1"
	fingerprint.Attributes["driver.rkt.version"] = rktMatches[1]
	fingerprint.Attributes["driver.rkt.appc.version"] = appcMatches[1]
	if d.config.VolumesEnabled {
		fingerprint.Attributes["driver.rkt.volumes.enabled"] = "1"
	}

	return fingerprint

}

func (d *Driver) RecoverTask(handle *drivers.TaskHandle) error {
	panic("not implemented")
}

func (d *Driver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, error) {
	if _, ok := d.tasks.Get(cfg.ID); ok {
		return nil, fmt.Errorf("task with ID '%s' already started", cfg.ID)
	}

	var driverConfig TaskConfig
	if err := cfg.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, fmt.Errorf("failed to decode driver config: %v", err)
	}

	handle := drivers.NewTaskHandle(pluginName)
	handle.Config = cfg

	panic("not implemented")

	// ACI image
	img := driverConfig.ImageName

	// Global arguments given to both prepare and run-prepared
	globalArgs := make([]string, 0, 50)

	// Add debug option to rkt command.
	debug := driverConfig.Debug

	// Add the given trust prefix
	trustPrefix := driverConfig.TrustPrefix
	insecure := false
	if trustPrefix != "" {
		var outBuf, errBuf bytes.Buffer
		cmd := exec.Command(rktCmd, "trust", "--skip-fingerprint-review=true", fmt.Sprintf("--prefix=%s", trustPrefix), fmt.Sprintf("--debug=%t", debug))
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("Error running rkt trust: %s\n\nOutput: %s\n\nError: %s",
				err, outBuf.String(), errBuf.String())
		}
		d.logger.Debug("added trust prefix", "trust_prefix", trustPrefix)
	} else {
		// Disable signature verification if the trust command was not run.
		insecure = true
	}

	// if we have a selective insecure_options, prefer them
	// insecure options are rkt's global argument, so we do this before the actual "run"
	if len(driverConfig.InsecureOptions) > 0 {
		globalArgs = append(globalArgs, fmt.Sprintf("--insecure-options=%s", strings.Join(driverConfig.InsecureOptions, ",")))
	} else if insecure {
		globalArgs = append(globalArgs, "--insecure-options=all")
	}

	// debug is rkt's global argument, so add it before the actual "run"
	globalArgs = append(globalArgs, fmt.Sprintf("--debug=%t", debug))

	prepareArgs := make([]string, 0, 50)
	runArgs := make([]string, 0, 50)

	prepareArgs = append(prepareArgs, globalArgs...)
	prepareArgs = append(prepareArgs, "prepare")
	runArgs = append(runArgs, globalArgs...)
	runArgs = append(runArgs, "run-prepared")

	// disable overlayfs
	if driverConfig.NoOverlay {
		prepareArgs = append(prepareArgs, "--no-overlay=true")
	}

	// Convert underscores to dashes in task names for use in volume names #2358
	sanitizedName := strings.Replace(cfg.Name, "_", "-", -1)

	// Mount /alloc
	allocVolName := fmt.Sprintf("%s-%s-alloc", cfg.ID, sanitizedName)
	prepareArgs = append(prepareArgs, fmt.Sprintf("--volume=%s,kind=host,source=%s", allocVolName, cfg.TaskDir().SharedAllocDir))
	prepareArgs = append(prepareArgs, fmt.Sprintf("--mount=volume=%s,target=%s", allocVolName, cfg.Env[env.AllocDir]))

	// Mount /local
	localVolName := fmt.Sprintf("%s-%s-local", cfg.ID, sanitizedName)
	prepareArgs = append(prepareArgs, fmt.Sprintf("--volume=%s,kind=host,source=%s", localVolName, cfg.TaskDir().LocalDir))
	prepareArgs = append(prepareArgs, fmt.Sprintf("--mount=volume=%s,target=%s", localVolName, cfg.Env[env.TaskLocalDir]))

	// Mount /secrets
	secretsVolName := fmt.Sprintf("%s-%s-secrets", cfg.ID, sanitizedName)
	prepareArgs = append(prepareArgs, fmt.Sprintf("--volume=%s,kind=host,source=%s", secretsVolName, cfg.TaskDir().SecretsDir))
	prepareArgs = append(prepareArgs, fmt.Sprintf("--mount=volume=%s,target=%s", secretsVolName, cfg.Env[env.SecretsDir]))

	// Mount arbitrary volumes if enabled
	if len(driverConfig.Volumes) > 0 {
		if !d.config.VolumesEnabled {
			return nil, fmt.Errorf("volumes_enabled is false; cannot use rkt volumes: %+q", driverConfig.Volumes)
		}
		for i, rawvol := range driverConfig.Volumes {
			parts := strings.Split(rawvol, ":")
			readOnly := "false"
			// job spec:
			//   volumes = ["/host/path:/container/path[:readOnly]"]
			// the third parameter is optional, mount is read-write by default
			if len(parts) == 3 {
				if parts[2] == "readOnly" {
					d.logger.Debug("mounting volume as readOnly", "volume", strings.Join(parts[:2], parts[1]))
					readOnly = "true"
				} else {
					d.logger.Warn("unknown volume parameter ignored for mount", "parameter", parts[2], "mount", parts[0])
				}
			} else if len(parts) != 2 {
				return nil, fmt.Errorf("invalid rkt volume: %q", rawvol)
			}
			volName := fmt.Sprintf("%s-%s-%d", cfg.ID, sanitizedName, i)
			prepareArgs = append(prepareArgs, fmt.Sprintf("--volume=%s,kind=host,source=%s,readOnly=%s", volName, parts[0], readOnly))
			prepareArgs = append(prepareArgs, fmt.Sprintf("--mount=volume=%s,target=%s", volName, parts[1]))
		}
	}

	// Inject environment variables
	for k, v := range cfg.Env {
		prepareArgs = append(prepareArgs, fmt.Sprintf("--set-env=%s=%s", k, v))
	}

	// Image is set here, because the commands that follow apply to it
	prepareArgs = append(prepareArgs, img)

	// Check if the user has overridden the exec command.
	if driverConfig.Command != "" {
		prepareArgs = append(prepareArgs, fmt.Sprintf("--exec=%v", driverConfig.Command))
	}

	// Add memory isolator
	prepareArgs = append(prepareArgs, fmt.Sprintf("--memory=%v", int64(cfg.Resources.MemoryLimitBytes)))

	// Add CPU isolator
	prepareArgs = append(prepareArgs, fmt.Sprintf("--cpu-shares=%v", int64(cfg.Resources.CPUShares)))

	// Add DNS servers
	if len(driverConfig.DNSServers) == 1 && (driverConfig.DNSServers[0] == "host" || driverConfig.DNSServers[0] == "none") {
		// Special case single item lists with the special values "host" or "none"
		runArgs = append(runArgs, fmt.Sprintf("--dns=%s", driverConfig.DNSServers[0]))
	} else {
		for _, ip := range driverConfig.DNSServers {
			if err := net.ParseIP(ip); err == nil {
				msg := fmt.Errorf("invalid ip address for container dns server %q", ip)
				d.logger.Debug("error parsing DNS server", "error", msg)
				return nil, msg
			}
			runArgs = append(runArgs, fmt.Sprintf("--dns=%s", ip))
		}
	}

	// set DNS search domains
	for _, domain := range driverConfig.DNSSearchDomains {
		runArgs = append(runArgs, fmt.Sprintf("--dns-search=%s", domain))
	}

	// set network
	network := strings.Join(driverConfig.Net, ",")
	if network != "" {
		runArgs = append(runArgs, fmt.Sprintf("--net=%s", network))
	}

	// Setup port mapping and exposed ports
	/*if len(task.Resources.Networks) == 0 {
		d.logger.Println("[DEBUG] driver.rkt: No network interfaces are available")
		if len(driverConfig.PortMap) > 0 {
			return nil, fmt.Errorf("Trying to map ports but no network interface is available")
		}
	} else if network == "host" {
		// Port mapping is skipped when host networking is used.
		d.logger.Println("[DEBUG] driver.rkt: Ignoring port_map when using --net=host")
	} else {
		// TODO add support for more than one network
		network := task.Resources.Networks[0]
		for _, port := range network.ReservedPorts {
			var containerPort string

			mapped, ok := driverConfig.PortMap[port.Label]
			if !ok {
				// If the user doesn't have a mapped port using port_map, driver stops running container.
				return nil, fmt.Errorf("port_map is not set. When you defined port in the resources, you need to configure port_map.")
			}
			containerPort = mapped

			hostPortStr := strconv.Itoa(port.Value)

			d.logger.Printf("[DEBUG] driver.rkt: exposed port %s", containerPort)
			// Add port option to rkt run arguments. rkt allows multiple port args
			prepareArgs = append(prepareArgs, fmt.Sprintf("--port=%s:%s", containerPort, hostPortStr))
		}

		for _, port := range network.DynamicPorts {
			// By default we will map the allocated port 1:1 to the container
			var containerPort string

			if mapped, ok := driverConfig.PortMap[port.Label]; ok {
				containerPort = mapped
			} else {
				// If the user doesn't have mapped a port using port_map, driver stops running container.
				return nil, fmt.Errorf("port_map is not set. When you defined port in the resources, you need to configure port_map.")
			}

			hostPortStr := strconv.Itoa(port.Value)

			d.logger.Printf("[DEBUG] driver.rkt: exposed port %s", containerPort)
			// Add port option to rkt run arguments. rkt allows multiple port args
			prepareArgs = append(prepareArgs, fmt.Sprintf("--port=%s:%s", containerPort, hostPortStr))
		}

	}*/

	// If a user has been specified for the task, pass it through to the user
	if cfg.User != "" {
		prepareArgs = append(prepareArgs, fmt.Sprintf("--user=%s", cfg.User))
	}

	// There's no task-level parameter for groups so check the driver
	// config for a custom group
	if driverConfig.Group != "" {
		prepareArgs = append(prepareArgs, fmt.Sprintf("--group=%s", driverConfig.Group))
	}

	// Add user passed arguments.
	if len(driverConfig.Args) != 0 {

		// Need to start arguments with "--"
		prepareArgs = append(prepareArgs, "--")

		for _, arg := range driverConfig.Args {
			prepareArgs = append(prepareArgs, fmt.Sprintf("%v", arg))
		}
	}

	pluginLogFile := filepath.Join(cfg.TaskDir().Dir, fmt.Sprintf("%s-executor.out", cfg.Name))
	executorConfig := &dstructs.ExecutorConfig{
		LogFile:  pluginLogFile,
		LogLevel: "debug",
	}

	// TODO: best way to pass port ranges in from client config
	execIntf, pluginClient, err := utils.CreateExecutor(os.Stderr, hclog.Debug, 14000, 14512, executorConfig)
	if err != nil {
		return nil, err
	}

	absPath, err := GetAbsolutePath(rktCmd)
	if err != nil {
		return nil, err
	}

	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(rktCmd, prepareArgs...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	d.logger.Debug("preparing task", "pod", img, "task_name", cfg.Name, "args", prepareArgs)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("Error preparing rkt pod: %s\n\nOutput: %s\n\nError: %s",
			err, outBuf.String(), errBuf.String())
	}
	uuid := strings.TrimSpace(outBuf.String())
	d.logger.Debug("task prepared", "pod", img, "task_name", cfg.Name, "uuid", uuid)
	runArgs = append(runArgs, uuid)

	// The task's environment is set via --set-env flags above, but the rkt
	// command itself needs an evironment with PATH set to find iptables.
	eb := env.NewEmptyBuilder()
	filter := strings.Split(d.config.ReadDefault("env.blacklist", config.DefaultEnvBlacklist), ",")
	rktEnv := eb.SetHostEnvvars(filter).Build()

	// Enable ResourceLimits to place the executor in a parent cgroup of
	// the rkt container. This allows stats collection via the executor to
	// work just like it does for exec.
	execCmd := &executor.ExecCommand{
		Cmd:            absPath,
		Args:           runArgs,
		ResourceLimits: true,
		Resources: &executor.Resources{
			CPU:      int(cfg.Resources.CPUShares),
			MemoryMB: int(cfg.Resources.MemoryLimitBytes),
			//IOPS:     task.Resources.IOPS,
			//DiskMB:   task.Resources.DiskMB,
		},
		Env:        cfg.EnvList(),
		TaskDir:    cfg.TaskDir().Dir,
		StdoutPath: cfg.StdoutPath,
		StderrPath: cfg.StderrPath,
	}
	ps, err := execIntf.Launch(execCmd)
	if err != nil {
		pluginClient.Kill()
		return nil, err
	}

	d.logger.Debug("started task", "aci", img, "uuid", uuid, "task_name", cfg.Name, "args", runArgs)
	h := &rktHandle{
		uuid:           uuid,
		env:            rktEnv,
		taskDir:        ctx.TaskDir,
		pluginClient:   pluginClient,
		executor:       execIntf,
		executorPid:    ps.Pid,
		logger:         d.logger,
		killTimeout:    GetKillTimeout(task.KillTimeout, maxKill),
		maxKillTimeout: maxKill,
		shutdownSignal: task.KillSignal,
		doneCh:         make(chan struct{}),
		waitCh:         make(chan *dstructs.WaitResult, 1),
	}
	go h.run()

	// Do not attempt to retrieve driver network if one won't exist:
	//  - "host" means the container itself has no networking metadata
	//  - "none" means no network is configured
	// https://coreos.com/rkt/docs/latest/networking/overview.html#no-loopback-only-networking
	var driverNetwork *cstructs.DriverNetwork
	if network != "host" && network != "none" {
		d.logger.Printf("[DEBUG] driver.rkt: retrieving network information for pod %q (UUID: %s) for task %q", img, uuid, d.taskName)
		driverNetwork, err = rktGetDriverNetwork(uuid, driverConfig.PortMap, d.logger)
		if err != nil && !pluginClient.Exited() {
			d.logger.Printf("[WARN] driver.rkt: network status retrieval for pod %q (UUID: %s) for task %q failed. Last error: %v", img, uuid, d.taskName, err)

			// If a portmap was given, this turns into a fatal error
			if len(driverConfig.PortMap) != 0 {
				pluginClient.Kill()
				return nil, fmt.Errorf("Trying to map ports but driver could not determine network information")
			}
		}
	}

}

func (d *Driver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	panic("not implemented")
}

func (d *Driver) StopTask(taskID string, timeout time.Duration, signal string) error {
	panic("not implemented")
}

func (d *Driver) DestroyTask(taskID string, force bool) error {
	panic("not implemented")
}

func (d *Driver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	panic("not implemented")
}

func (d *Driver) TaskStats(taskID string) (*drivers.TaskStats, error) {
	panic("not implemented")
}

func (d *Driver) TaskEvents(context.Context) (<-chan *drivers.TaskEvent, error) {
	panic("not implemented")
}

func (d *Driver) SignalTask(taskID string, signal string) error {
	panic("not implemented")
}

func (d *Driver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	panic("not implemented")
}
