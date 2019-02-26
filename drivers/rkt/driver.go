package rkt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	appcschema "github.com/appc/spec/schema"
	"github.com/hashicorp/consul-template/signals"
	hclog "github.com/hashicorp/go-hclog"
	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/drivers/shared/executor"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
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

	// networkDeadline is how long to wait for container network
	// information to become available.
	networkDeadline = 1 * time.Minute

	// taskHandleVersion is the version of task handle which this driver sets
	// and understands how to decode driver state
	taskHandleVersion = 1
)

var (
	// PluginID is the rawexec plugin metadata registered in the plugin
	// catalog.
	PluginID = loader.PluginID{
		Name:       pluginName,
		PluginType: base.PluginTypeDriver,
	}

	// PluginConfig is the rawexec factory function registered in the
	// plugin catalog.
	PluginConfig = &loader.InternalPluginConfig{
		Config:  map[string]interface{}{},
		Factory: func(l hclog.Logger) interface{} { return NewRktDriver(l) },
	}
)

// PluginLoader maps pre-0.9 client driver options to post-0.9 plugin options.
func PluginLoader(opts map[string]string) (map[string]interface{}, error) {
	conf := map[string]interface{}{}
	if v, err := strconv.ParseBool(opts["driver.rkt.volumes.enabled"]); err == nil {
		conf["volumes_enabled"] = v
	}
	return conf, nil
}

var (
	// pluginInfo is the response returned for the PluginInfo RPC
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeDriver,
		PluginApiVersions: []string{drivers.ApiVersion010},
		PluginVersion:     "0.1.0",
		Name:              pluginName,
	}

	// configSpec is the hcl specification returned by the ConfigSchema RPC
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"volumes_enabled": hclspec.NewDefault(
			hclspec.NewAttr("volumes_enabled", "bool", false),
			hclspec.NewLiteral("true"),
		),
	})

	// taskConfigSpec is the hcl specification for the driver config section of
	// a taskConfig within a job. It is returned in the TaskConfigSchema RPC
	taskConfigSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"image":              hclspec.NewAttr("image", "string", true),
		"command":            hclspec.NewAttr("command", "string", false),
		"args":               hclspec.NewAttr("args", "list(string)", false),
		"trust_prefix":       hclspec.NewAttr("trust_prefix", "string", false),
		"dns_servers":        hclspec.NewAttr("dns_servers", "list(string)", false),
		"dns_search_domains": hclspec.NewAttr("dns_search_domains", "list(string)", false),
		"net":                hclspec.NewAttr("net", "list(string)", false),
		"port_map":           hclspec.NewAttr("port_map", "list(map(string))", false),
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
		FSIsolation: drivers.FSIsolationImage,
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

// TaskConfig is the driver configuration of a taskConfig within a job
type TaskConfig struct {
	ImageName        string             `codec:"image"`
	Command          string             `codec:"command"`
	Args             []string           `codec:"args"`
	TrustPrefix      string             `codec:"trust_prefix"`
	DNSServers       []string           `codec:"dns_servers"`        // DNS Server for containers
	DNSSearchDomains []string           `codec:"dns_search_domains"` // DNS Search domains for containers
	Net              []string           `codec:"net"`                // Networks for the containers
	PortMap          hclutils.MapStrStr `codec:"port_map"`           // A map of host port and the port name defined in the image manifest file
	Volumes          []string           `codec:"volumes"`            // Host-Volumes to mount in, syntax: /path/to/host/directory:/destination/path/in/container[:readOnly]
	InsecureOptions  []string           `codec:"insecure_options"`   // list of args for --insecure-options

	NoOverlay bool   `codec:"no_overlay"` // disable overlayfs for rkt run
	Debug     bool   `codec:"debug"`      // Enable debug option for rkt command
	Group     string `codec:"group"`      // Group override for the container
}

// TaskState is the state which is encoded in the handle returned in
// StartTask. This information is needed to rebuild the taskConfig state and handler
// during recovery.
type TaskState struct {
	ReattachConfig *pstructs.ReattachConfig
	TaskConfig     *drivers.TaskConfig
	Pid            int
	StartedAt      time.Time
	UUID           string
}

// Driver is a driver for running images via Rkt We attempt to chose sane
// defaults for now, with more configuration available planned in the future.
type Driver struct {
	// eventer is used to handle multiplexing of TaskEvents calls such that an
	// event can be broadcast to all callers
	eventer *eventer.Eventer

	// config is the driver configuration set by the SetConfig RPC
	config *Config

	// nomadConfig is the client config from nomad
	nomadConfig *base.ClientDriverConfig

	// tasks is the in memory datastore mapping taskIDs to rktTaskHandles
	tasks *taskStore

	// ctx is the context for the driver. It is passed to other subsystems to
	// coordinate shutdown
	ctx context.Context

	// signalShutdown is called when the driver is shutting down and cancels the
	// ctx passed to any subsystems
	signalShutdown context.CancelFunc

	// logger will log to the Nomad agent
	logger hclog.Logger

	// A tri-state boolean to know if the fingerprinting has happened and
	// whether it has been successful
	fingerprintSuccess *bool
	fingerprintLock    sync.Mutex
}

func NewRktDriver(logger hclog.Logger) drivers.DriverPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	logger = logger.Named(pluginName)
	return &Driver{
		eventer:        eventer.NewEventer(ctx, logger),
		config:         &Config{},
		tasks:          newTaskStore(),
		ctx:            ctx,
		signalShutdown: cancel,
		logger:         logger,
	}
}

func (d *Driver) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

func (d *Driver) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

func (d *Driver) SetConfig(cfg *base.Config) error {
	var config Config
	if len(cfg.PluginConfig) != 0 {
		if err := base.MsgPackDecode(cfg.PluginConfig, &config); err != nil {
			return err
		}
	}

	d.config = &config
	if cfg.AgentConfig != nil {
		d.nomadConfig = cfg.AgentConfig.Driver
	}
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
	defer close(ch)
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

// setFingerprintSuccess marks the driver as having fingerprinted successfully
func (d *Driver) setFingerprintSuccess() {
	d.fingerprintLock.Lock()
	d.fingerprintSuccess = helper.BoolToPtr(true)
	d.fingerprintLock.Unlock()
}

// setFingerprintFailure marks the driver as having failed fingerprinting
func (d *Driver) setFingerprintFailure() {
	d.fingerprintLock.Lock()
	d.fingerprintSuccess = helper.BoolToPtr(false)
	d.fingerprintLock.Unlock()
}

// fingerprintSuccessful returns true if the driver has
// never fingerprinted or has successfully fingerprinted
func (d *Driver) fingerprintSuccessful() bool {
	d.fingerprintLock.Lock()
	defer d.fingerprintLock.Unlock()
	return d.fingerprintSuccess == nil || *d.fingerprintSuccess
}

func (d *Driver) buildFingerprint() *drivers.Fingerprint {
	fingerprint := &drivers.Fingerprint{
		Attributes:        map[string]*pstructs.Attribute{},
		Health:            drivers.HealthStateHealthy,
		HealthDescription: drivers.DriverHealthy,
	}

	// Only enable if we are root
	if syscall.Geteuid() != 0 {
		if d.fingerprintSuccessful() {
			d.logger.Debug("must run as root user, disabling")
		}
		d.setFingerprintFailure()
		fingerprint.Health = drivers.HealthStateUndetected
		fingerprint.HealthDescription = drivers.DriverRequiresRootMessage
		return fingerprint
	}

	outBytes, err := exec.Command(rktCmd, "version").Output()
	if err != nil {
		fingerprint.Health = drivers.HealthStateUndetected
		fingerprint.HealthDescription = fmt.Sprintf("Failed to execute %s version: %v", rktCmd, err)
		d.setFingerprintFailure()
		return fingerprint
	}
	out := strings.TrimSpace(string(outBytes))

	rktMatches := reRktVersion.FindStringSubmatch(out)
	appcMatches := reAppcVersion.FindStringSubmatch(out)
	if len(rktMatches) != 2 || len(appcMatches) != 2 {
		fingerprint.Health = drivers.HealthStateUndetected
		fingerprint.HealthDescription = "Unable to parse rkt version string"
		d.setFingerprintFailure()
		return fingerprint
	}

	minVersion, _ := version.NewVersion(minRktVersion)
	currentVersion, _ := version.NewVersion(rktMatches[1])
	if currentVersion.LessThan(minVersion) {
		// Do not allow ancient rkt versions
		fingerprint.Health = drivers.HealthStateUndetected
		fingerprint.HealthDescription = fmt.Sprintf("Unsuported rkt version %s", currentVersion)
		if d.fingerprintSuccessful() {
			d.logger.Warn("unsupported rkt version please upgrade to >= "+minVersion.String(),
				"rkt_version", currentVersion)
		}
		d.setFingerprintFailure()
		return fingerprint
	}

	fingerprint.Attributes["driver.rkt"] = pstructs.NewBoolAttribute(true)
	fingerprint.Attributes["driver.rkt.version"] = pstructs.NewStringAttribute(rktMatches[1])
	fingerprint.Attributes["driver.rkt.appc.version"] = pstructs.NewStringAttribute(appcMatches[1])
	if d.config.VolumesEnabled {
		fingerprint.Attributes["driver.rkt.volumes.enabled"] = pstructs.NewBoolAttribute(true)
	}
	d.setFingerprintSuccess()
	return fingerprint

}

func (d *Driver) RecoverTask(handle *drivers.TaskHandle) error {
	if handle == nil {
		return fmt.Errorf("error: handle cannot be nil")
	}

	// COMPAT(0.10): pre 0.9 upgrade path check
	if handle.Version == 0 {
		return d.recoverPre09Task(handle)
	}

	// If already attached to handle there's nothing to recover.
	if _, ok := d.tasks.Get(handle.Config.ID); ok {
		d.logger.Trace("nothing to recover; task already exists",
			"task_id", handle.Config.ID,
			"task_name", handle.Config.Name,
		)
		return nil
	}

	var taskState TaskState
	if err := handle.GetDriverState(&taskState); err != nil {
		d.logger.Error("failed to decode taskConfig state from handle", "error", err, "task_id", handle.Config.ID)
		return fmt.Errorf("failed to decode taskConfig state from handle: %v", err)
	}

	plugRC, err := pstructs.ReattachConfigToGoPlugin(taskState.ReattachConfig)
	if err != nil {
		d.logger.Error("failed to build ReattachConfig from taskConfig state", "error", err, "task_id", handle.Config.ID)
		return fmt.Errorf("failed to build ReattachConfig from taskConfig state: %v", err)
	}

	execImpl, pluginClient, err := executor.ReattachToExecutor(plugRC,
		d.logger.With("task_name", handle.Config.Name, "alloc_id", handle.Config.AllocID))
	if err != nil {
		d.logger.Error("failed to reattach to executor", "error", err, "task_id", handle.Config.ID)
		return fmt.Errorf("failed to reattach to executor: %v", err)
	}

	// The taskConfig's environment is set via --set-env flags in Start, but the rkt
	// command itself needs an environment with PATH set to find iptables.
	// TODO (preetha) need to figure out how to read env.blacklist
	eb := taskenv.NewEmptyBuilder()
	filter := strings.Split(config.DefaultEnvBlacklist, ",")
	rktEnv := eb.SetHostEnvvars(filter).Build()

	h := &taskHandle{
		exec:         execImpl,
		env:          rktEnv,
		pid:          taskState.Pid,
		uuid:         taskState.UUID,
		pluginClient: pluginClient,
		taskConfig:   taskState.TaskConfig,
		procState:    drivers.TaskStateRunning,
		startedAt:    taskState.StartedAt,
		exitResult:   &drivers.ExitResult{},
	}

	d.tasks.Set(taskState.TaskConfig.ID, h)

	go h.run()
	return nil
}

func (d *Driver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	if _, ok := d.tasks.Get(cfg.ID); ok {
		return nil, nil, fmt.Errorf("taskConfig with ID '%s' already started", cfg.ID)
	}

	var driverConfig TaskConfig

	if err := cfg.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to decode driver config: %v", err)
	}

	handle := drivers.NewTaskHandle(taskHandleVersion)
	handle.Config = cfg

	// todo(preetha) - port map in client v1 is a slice of maps that get merged. figure out if the caller will do this
	//driverConfig.PortMap

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
			return nil, nil, fmt.Errorf("Error running rkt trust: %s\n\nOutput: %s\n\nError: %s",
				err, outBuf.String(), errBuf.String())
		}
		d.logger.Debug("added trust prefix", "trust_prefix", trustPrefix, "task_name", cfg.Name)
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

	// Convert underscores to dashes in taskConfig names for use in volume names #2358
	sanitizedName := strings.Replace(cfg.Name, "_", "-", -1)

	// Mount /alloc
	allocVolName := fmt.Sprintf("%s-%s-alloc", cfg.AllocID, sanitizedName)
	prepareArgs = append(prepareArgs, fmt.Sprintf("--volume=%s,kind=host,source=%s", allocVolName, cfg.TaskDir().SharedAllocDir))
	prepareArgs = append(prepareArgs, fmt.Sprintf("--mount=volume=%s,target=%s", allocVolName, "/alloc"))

	// Mount /local
	localVolName := fmt.Sprintf("%s-%s-local", cfg.AllocID, sanitizedName)
	prepareArgs = append(prepareArgs, fmt.Sprintf("--volume=%s,kind=host,source=%s", localVolName, cfg.TaskDir().LocalDir))
	prepareArgs = append(prepareArgs, fmt.Sprintf("--mount=volume=%s,target=%s", localVolName, "/local"))

	// Mount /secrets
	secretsVolName := fmt.Sprintf("%s-%s-secrets", cfg.AllocID, sanitizedName)
	prepareArgs = append(prepareArgs, fmt.Sprintf("--volume=%s,kind=host,source=%s", secretsVolName, cfg.TaskDir().SecretsDir))
	prepareArgs = append(prepareArgs, fmt.Sprintf("--mount=volume=%s,target=%s", secretsVolName, "/secrets"))

	// Mount arbitrary volumes if enabled
	if len(driverConfig.Volumes) > 0 {
		if !d.config.VolumesEnabled {
			return nil, nil, fmt.Errorf("volumes_enabled is false; cannot use rkt volumes: %+q", driverConfig.Volumes)
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
				return nil, nil, fmt.Errorf("invalid rkt volume: %q", rawvol)
			}
			volName := fmt.Sprintf("%s-%s-%d", cfg.AllocID, sanitizedName, i)
			prepareArgs = append(prepareArgs, fmt.Sprintf("--volume=%s,kind=host,source=%s,readOnly=%s", volName, parts[0], readOnly))
			prepareArgs = append(prepareArgs, fmt.Sprintf("--mount=volume=%s,target=%s", volName, parts[1]))
		}
	}

	// Mount task volumes, always do
	for i, vol := range cfg.Mounts {
		volName := fmt.Sprintf("%s-%s-taskmounts-%d", cfg.AllocID, sanitizedName, i)
		prepareArgs = append(prepareArgs, fmt.Sprintf("--volume=%s,kind=host,source=%s,readOnly=%v", volName, vol.HostPath, vol.Readonly))
		prepareArgs = append(prepareArgs, fmt.Sprintf("--mount=volume=%s,target=%s", volName, vol.TaskPath))
	}

	// Mount task devices, always do
	for i, vol := range cfg.Devices {
		volName := fmt.Sprintf("%s-%s-taskdevices-%d", cfg.AllocID, sanitizedName, i)
		readOnly := !strings.Contains(vol.Permissions, "w")
		prepareArgs = append(prepareArgs, fmt.Sprintf("--volume=%s,kind=host,source=%s,readOnly=%v", volName, vol.HostPath, readOnly))
		prepareArgs = append(prepareArgs, fmt.Sprintf("--mount=volume=%s,target=%s", volName, vol.TaskPath))
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
	prepareArgs = append(prepareArgs, fmt.Sprintf("--memory=%v", cfg.Resources.LinuxResources.MemoryLimitBytes))

	// Add CPU isolator
	prepareArgs = append(prepareArgs, fmt.Sprintf("--cpu=%v", cfg.Resources.LinuxResources.CPUShares))

	// Add DNS servers
	if len(driverConfig.DNSServers) == 1 && (driverConfig.DNSServers[0] == "host" || driverConfig.DNSServers[0] == "none") {
		// Special case single item lists with the special values "host" or "none"
		runArgs = append(runArgs, fmt.Sprintf("--dns=%s", driverConfig.DNSServers[0]))
	} else {
		for _, ip := range driverConfig.DNSServers {
			if err := net.ParseIP(ip); err == nil {
				wrappedErr := fmt.Errorf("invalid ip address for container dns server %q", ip)
				d.logger.Debug("error parsing DNS server", "error", wrappedErr)
				return nil, nil, wrappedErr
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
	if len(cfg.Resources.NomadResources.Networks) == 0 {
		d.logger.Debug("no network interfaces are available")
		if len(driverConfig.PortMap) > 0 {
			return nil, nil, fmt.Errorf("Trying to map ports but no network interface is available")
		}
	} else if network == "host" {
		// Port mapping is skipped when host networking is used.
		d.logger.Debug("Ignoring port_map when using --net=host", "task_name", cfg.Name)
	} else {
		network := cfg.Resources.NomadResources.Networks[0]
		for _, port := range network.ReservedPorts {
			var containerPort string

			mapped, ok := driverConfig.PortMap[port.Label]
			if !ok {
				// If the user doesn't have a mapped port using port_map, driver stops running container.
				return nil, nil, fmt.Errorf("port_map is not set. When you defined port in the resources, you need to configure port_map.")
			}
			containerPort = mapped

			hostPortStr := strconv.Itoa(port.Value)

			d.logger.Debug("driver.rkt: exposed port", "containerPort", containerPort)
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
				return nil, nil, fmt.Errorf("port_map is not set. When you defined port in the resources, you need to configure port_map.")
			}

			hostPortStr := strconv.Itoa(port.Value)

			d.logger.Debug("exposed port", "containerPort", containerPort, "task_name", cfg.Name)
			// Add port option to rkt run arguments. rkt allows multiple port args
			prepareArgs = append(prepareArgs, fmt.Sprintf("--port=%s:%s", containerPort, hostPortStr))
		}

	}

	// If a user has been specified for the taskConfig, pass it through to the user
	if cfg.User != "" {
		prepareArgs = append(prepareArgs, fmt.Sprintf("--user=%s", cfg.User))
	}

	// There's no taskConfig-level parameter for groups so check the driver
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
	executorConfig := &executor.ExecutorConfig{
		LogFile:  pluginLogFile,
		LogLevel: "debug",
	}

	execImpl, pluginClient, err := executor.CreateExecutor(
		d.logger.With("task_name", handle.Config.Name, "alloc_id", handle.Config.AllocID),
		d.nomadConfig, executorConfig)
	if err != nil {
		return nil, nil, err
	}

	absPath, err := GetAbsolutePath(rktCmd)
	if err != nil {
		return nil, nil, err
	}

	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(rktCmd, prepareArgs...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	d.logger.Debug("preparing taskConfig", "pod", img, "task_name", cfg.Name, "args", prepareArgs)
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("Error preparing rkt pod: %s\n\nOutput: %s\n\nError: %s",
			err, outBuf.String(), errBuf.String())
	}
	uuid := strings.TrimSpace(outBuf.String())
	d.logger.Debug("taskConfig prepared", "pod", img, "task_name", cfg.Name, "uuid", uuid)
	runArgs = append(runArgs, uuid)

	// The taskConfig's environment is set via --set-env flags above, but the rkt
	// command itself needs an environment with PATH set to find iptables.

	// TODO (preetha) need to figure out how to pass env.blacklist from client config
	eb := taskenv.NewEmptyBuilder()
	filter := strings.Split(config.DefaultEnvBlacklist, ",")
	rktEnv := eb.SetHostEnvvars(filter).Build()

	// Enable ResourceLimits to place the executor in a parent cgroup of
	// the rkt container. This allows stats collection via the executor to
	// work just like it does for exec.
	execCmd := &executor.ExecCommand{
		Cmd:            absPath,
		Args:           runArgs,
		ResourceLimits: true,
		Resources:      cfg.Resources,

		// Use rktEnv, the environment needed for running rkt, not the task env
		Env: rktEnv.List(),

		TaskDir:    cfg.TaskDir().Dir,
		StdoutPath: cfg.StdoutPath,
		StderrPath: cfg.StderrPath,
	}
	ps, err := execImpl.Launch(execCmd)
	if err != nil {
		pluginClient.Kill()
		return nil, nil, err
	}

	d.logger.Debug("started taskConfig", "aci", img, "uuid", uuid, "task_name", cfg.Name, "args", runArgs)
	h := &taskHandle{
		exec:         execImpl,
		env:          rktEnv,
		pid:          ps.Pid,
		uuid:         uuid,
		pluginClient: pluginClient,
		taskConfig:   cfg,
		procState:    drivers.TaskStateRunning,
		startedAt:    time.Now().Round(time.Millisecond),
		logger:       d.logger,
	}

	rktDriverState := TaskState{
		ReattachConfig: pstructs.ReattachConfigFromGoPlugin(pluginClient.ReattachConfig()),
		Pid:            ps.Pid,
		TaskConfig:     cfg,
		StartedAt:      h.startedAt,
		UUID:           uuid,
	}

	if err := handle.SetDriverState(&rktDriverState); err != nil {
		d.logger.Error("failed to start task, error setting driver state", "error", err, "task_name", cfg.Name)
		execImpl.Shutdown("", 0)
		pluginClient.Kill()
		return nil, nil, fmt.Errorf("failed to set driver state: %v", err)
	}

	d.tasks.Set(cfg.ID, h)
	go h.run()

	// Do not attempt to retrieve driver network if one won't exist:
	//  - "host" means the container itself has no networking metadata
	//  - "none" means no network is configured
	// https://coreos.com/rkt/docs/latest/networking/overview.html#no-loopback-only-networking
	var driverNetwork *drivers.DriverNetwork
	if network != "host" && network != "none" {
		d.logger.Debug("retrieving network information for pod", "pod", img, "UUID", uuid, "task_name", cfg.Name)
		driverNetwork, err = rktGetDriverNetwork(uuid, driverConfig.PortMap, d.logger)
		if err != nil && !pluginClient.Exited() {
			d.logger.Warn("network status retrieval for pod failed", "pod", img, "UUID", uuid, "task_name", cfg.Name, "error", err)

			// If a portmap was given, this turns into a fatal error
			if len(driverConfig.PortMap) != 0 {
				pluginClient.Kill()
				return nil, nil, fmt.Errorf("Trying to map ports but driver could not determine network information")
			}
		}
	}

	return handle, driverNetwork, nil

}

func (d *Driver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	ch := make(chan *drivers.ExitResult)
	go d.handleWait(ctx, handle, ch)

	return ch, nil
}

func (d *Driver) StopTask(taskID string, timeout time.Duration, signal string) error {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	if err := handle.exec.Shutdown(signal, timeout); err != nil {
		if handle.pluginClient.Exited() {
			return nil
		}
		return fmt.Errorf("executor Shutdown failed: %v", err)
	}

	return nil
}

func (d *Driver) DestroyTask(taskID string, force bool) error {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	if handle.IsRunning() && !force {
		return fmt.Errorf("cannot destroy running task")
	}

	if !handle.pluginClient.Exited() {
		if handle.IsRunning() {
			if err := handle.exec.Shutdown("", 0); err != nil {
				handle.logger.Error("destroying executor failed", "err", err)
			}
		}

		handle.pluginClient.Kill()
	}

	d.tasks.Delete(taskID)
	return nil
}

func (d *Driver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	return handle.TaskStatus(), nil
}

func (d *Driver) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	return handle.exec.Stats(ctx, interval)
}

func (d *Driver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	return d.eventer.TaskEvents(ctx)
}

func (d *Driver) SignalTask(taskID string, signal string) error {
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	sig := os.Interrupt
	if s, ok := signals.SignalLookup[signal]; ok {
		d.logger.Warn("signal to send to task unknown, using SIGINT", "signal", signal, "task_id", handle.taskConfig.ID, "task_name", handle.taskConfig.Name)
		sig = s
	}
	return handle.exec.Signal(sig)
}

func (d *Driver) ExecTask(taskID string, cmdArgs []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	if len(cmdArgs) == 0 {
		return nil, fmt.Errorf("error cmd must have atleast one value")
	}
	handle, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}
	// enter + UUID + cmd + args...
	cmd := cmdArgs[0]
	args := cmdArgs[1:]
	enterArgs := make([]string, 3+len(args))
	enterArgs[0] = "enter"
	enterArgs[1] = handle.uuid
	enterArgs[2] = handle.env.ReplaceEnv(cmd)
	copy(enterArgs[3:], handle.env.ParseAndReplace(args))
	out, exitCode, err := handle.exec.Exec(time.Now().Add(timeout), rktCmd, enterArgs)
	if err != nil {
		return nil, err
	}

	return &drivers.ExecTaskResult{
		Stdout: out,
		ExitResult: &drivers.ExitResult{
			ExitCode: exitCode,
		},
	}, nil

}

// GetAbsolutePath returns the absolute path of the passed binary by resolving
// it in the path and following symlinks.
func GetAbsolutePath(bin string) (string, error) {
	lp, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path to %q executable: %v", bin, err)
	}

	return filepath.EvalSymlinks(lp)
}

func rktGetDriverNetwork(uuid string, driverConfigPortMap map[string]string, logger hclog.Logger) (*drivers.DriverNetwork, error) {
	deadline := time.Now().Add(networkDeadline)
	var lastErr error
	try := 0

	for time.Now().Before(deadline) {
		try++
		if status, err := rktGetStatus(uuid, logger); err == nil {
			for _, net := range status.Networks {
				if !net.IP.IsGlobalUnicast() {
					continue
				}

				// Get the pod manifest so we can figure out which ports are exposed
				var portmap map[string]int
				manifest, err := rktGetManifest(uuid)
				if err == nil {
					portmap, err = rktManifestMakePortMap(manifest, driverConfigPortMap)
					if err != nil {
						lastErr = fmt.Errorf("could not create manifest-based portmap: %v", err)
						return nil, lastErr
					}
				} else {
					lastErr = fmt.Errorf("could not get pod manifest: %v", err)
					return nil, lastErr
				}

				// This is a successful landing; log if its not the first attempt.
				if try > 1 {
					logger.Debug("retrieved network info for pod", "uuid", uuid, "attempt", try)
				}
				return &drivers.DriverNetwork{
					PortMap: portmap,
					IP:      status.Networks[0].IP.String(),
				}, nil
			}

			if len(status.Networks) == 0 {
				lastErr = fmt.Errorf("no networks found")
			} else {
				lastErr = fmt.Errorf("no good driver networks out of %d returned", len(status.Networks))
			}
		} else {
			lastErr = fmt.Errorf("getting status failed: %v", err)
		}

		waitTime := getJitteredNetworkRetryTime()
		logger.Debug("failed getting network info for pod, sleeping", "uuid", uuid, "attempt", try, "err", lastErr, "wait", waitTime)
		time.Sleep(waitTime)
	}
	return nil, fmt.Errorf("timed out, last error: %v", lastErr)
}

// Given a rkt/appc pod manifest and driver portmap configuration, create
// a driver portmap.
func rktManifestMakePortMap(manifest *appcschema.PodManifest, configPortMap map[string]string) (map[string]int, error) {
	if len(manifest.Apps) == 0 {
		return nil, fmt.Errorf("manifest has no apps")
	}
	if len(manifest.Apps) != 1 {
		return nil, fmt.Errorf("manifest has multiple apps!")
	}
	app := manifest.Apps[0]
	if app.App == nil {
		return nil, fmt.Errorf("specified app has no App object")
	}

	portMap := make(map[string]int)
	for svc, name := range configPortMap {
		for _, port := range app.App.Ports {
			if port.Name.String() == name {
				portMap[svc] = int(port.Port)
			}
		}
	}
	return portMap, nil
}

// Retrieve pod status for the pod with the given UUID.
func rktGetStatus(uuid string, logger hclog.Logger) (*Pod, error) {
	statusArgs := []string{
		"status",
		"--format=json",
		uuid,
	}
	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(rktCmd, statusArgs...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if outBuf.Len() > 0 {
			logger.Debug("status output for UUID", "uuid", uuid, "error", elide(outBuf))
		}
		if errBuf.Len() == 0 {
			return nil, err
		}
		logger.Debug("status error output", "uuid", uuid, "error", elide(errBuf))
		return nil, fmt.Errorf("%s. stderr: %q", err, elide(errBuf))
	}
	var status Pod
	if err := json.Unmarshal(outBuf.Bytes(), &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// Retrieves a pod manifest
func rktGetManifest(uuid string) (*appcschema.PodManifest, error) {
	statusArgs := []string{
		"cat-manifest",
		uuid,
	}
	var outBuf bytes.Buffer
	cmd := exec.Command(rktCmd, statusArgs...)
	cmd.Stdout = &outBuf
	cmd.Stderr = ioutil.Discard
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	var manifest appcschema.PodManifest
	if err := json.Unmarshal(outBuf.Bytes(), &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// Create a time with a 0 to 100ms jitter for rktGetDriverNetwork retries
func getJitteredNetworkRetryTime() time.Duration {
	return time.Duration(900+rand.Intn(100)) * time.Millisecond
}

// Conditionally elide a buffer to an arbitrary length
func elideToLen(inBuf bytes.Buffer, length int) bytes.Buffer {
	if inBuf.Len() > length {
		inBuf.Truncate(length)
		inBuf.WriteString("...")
	}
	return inBuf
}

// Conditionally elide a buffer to an 80 character string
func elide(inBuf bytes.Buffer) string {
	tempBuf := elideToLen(inBuf, 80)
	return tempBuf.String()
}

func (d *Driver) handleWait(ctx context.Context, handle *taskHandle, ch chan *drivers.ExitResult) {
	defer close(ch)
	var result *drivers.ExitResult
	ps, err := handle.exec.Wait(ctx)
	if err != nil {
		result = &drivers.ExitResult{
			Err: fmt.Errorf("executor: error waiting on process: %v", err),
		}
	} else {
		result = &drivers.ExitResult{
			ExitCode: ps.ExitCode,
			Signal:   ps.Signal,
		}
	}

	select {
	case <-ctx.Done():
	case <-d.ctx.Done():
	case ch <- result:
	}
}

func (d *Driver) Shutdown() {
	d.signalShutdown()
}
