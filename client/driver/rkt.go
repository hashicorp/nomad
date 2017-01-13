package driver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/executor"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/fields"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

var (
	reRktVersion  = regexp.MustCompile(`rkt [vV]ersion[:]? (\d[.\d]+)`)
	reAppcVersion = regexp.MustCompile(`appc [vV]ersion[:]? (\d[.\d]+)`)
)

const (
	// minRktVersion is the earliest supported version of rkt. rkt added support
	// for CPU and memory isolators in 0.14.0. We cannot support an earlier
	// version to maintain an uniform interface across all drivers
	minRktVersion = "1.0.0"

	// The key populated in the Node Attributes to indicate the presence of the
	// Rkt driver
	rktDriverAttr = "driver.rkt"

	// rktVolumesConfigOption is the key for enabling the use of custom
	// bind volumes.
	rktVolumesConfigOption  = "rkt.volumes.enabled"
	rktVolumesConfigDefault = true

	// rktCmd is the command rkt is installed as.
	rktCmd = "rkt"
)

// RktDriver is a driver for running images via Rkt
// We attempt to chose sane defaults for now, with more configuration available
// planned in the future
type RktDriver struct {
	DriverContext
}

type RktDriverConfig struct {
	ImageName        string              `mapstructure:"image"`
	Command          string              `mapstructure:"command"`
	Args             []string            `mapstructure:"args"`
	TrustPrefix      string              `mapstructure:"trust_prefix"`
	DNSServers       []string            `mapstructure:"dns_servers"`        // DNS Server for containers
	DNSSearchDomains []string            `mapstructure:"dns_search_domains"` // DNS Search domains for containers
	Net              []string            `mapstructure:"net"`                // Networks for the containers
	PortMapRaw       []map[string]string `mapstructure:"port_map"`           //
	PortMap          map[string]string   `mapstructure:"-"`                  // A map of host port and the port name defined in the image manifest file
	Volumes          []string            `mapstructure:"volumes"`            // Host-Volumes to mount in, syntax: /path/to/host/directory:/destination/path/in/container

	Debug bool `mapstructure:"debug"` // Enable debug option for rkt command
}

// rktHandle is returned from Start/Open as a handle to the PID
type rktHandle struct {
	pluginClient   *plugin.Client
	executorPid    int
	executor       executor.Executor
	logger         *log.Logger
	killTimeout    time.Duration
	maxKillTimeout time.Duration
	waitCh         chan *dstructs.WaitResult
	doneCh         chan struct{}
}

// rktPID is a struct to map the pid running the process to the vm image on
// disk
type rktPID struct {
	PluginConfig   *PluginReattachConfig
	ExecutorPid    int
	KillTimeout    time.Duration
	MaxKillTimeout time.Duration
}

// NewRktDriver is used to create a new exec driver
func NewRktDriver(ctx *DriverContext) Driver {
	return &RktDriver{DriverContext: *ctx}
}

func (d *RktDriver) FSIsolation() cstructs.FSIsolation {
	return cstructs.FSIsolationImage
}

// Validate is used to validate the driver configuration
func (d *RktDriver) Validate(config map[string]interface{}) error {
	fd := &fields.FieldData{
		Raw: config,
		Schema: map[string]*fields.FieldSchema{
			"image": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: true,
			},
			"command": &fields.FieldSchema{
				Type: fields.TypeString,
			},
			"args": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"trust_prefix": &fields.FieldSchema{
				Type: fields.TypeString,
			},
			"dns_servers": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"dns_search_domains": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"net": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"port_map": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"debug": &fields.FieldSchema{
				Type: fields.TypeBool,
			},
			"volumes": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
		},
	}

	if err := fd.Validate(); err != nil {
		return err
	}

	return nil
}

func (d *RktDriver) Abilities() DriverAbilities {
	return DriverAbilities{
		SendSignals: false,
	}
}

func (d *RktDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Get the current status so that we can log any debug messages only if the
	// state changes
	_, currentlyEnabled := node.Attributes[rktDriverAttr]

	// Only enable if we are root when running on non-windows systems.
	if runtime.GOOS != "windows" && syscall.Geteuid() != 0 {
		if currentlyEnabled {
			d.logger.Printf("[DEBUG] driver.rkt: must run as root user, disabling")
		}
		delete(node.Attributes, rktDriverAttr)
		return false, nil
	}

	outBytes, err := exec.Command(rktCmd, "version").Output()
	if err != nil {
		delete(node.Attributes, rktDriverAttr)
		return false, nil
	}
	out := strings.TrimSpace(string(outBytes))

	rktMatches := reRktVersion.FindStringSubmatch(out)
	appcMatches := reAppcVersion.FindStringSubmatch(out)
	if len(rktMatches) != 2 || len(appcMatches) != 2 {
		delete(node.Attributes, rktDriverAttr)
		return false, fmt.Errorf("Unable to parse Rkt version string: %#v", rktMatches)
	}

	node.Attributes[rktDriverAttr] = "1"
	node.Attributes["driver.rkt.version"] = rktMatches[1]
	node.Attributes["driver.rkt.appc.version"] = appcMatches[1]

	minVersion, _ := version.NewVersion(minRktVersion)
	currentVersion, _ := version.NewVersion(node.Attributes["driver.rkt.version"])
	if currentVersion.LessThan(minVersion) {
		// Do not allow ancient rkt versions
		d.logger.Printf("[WARN] driver.rkt: please upgrade rkt to a version >= %s", minVersion)
		node.Attributes[rktDriverAttr] = "0"
	}

	// Advertise if this node supports rkt volumes
	if d.config.ReadBoolDefault(rktVolumesConfigOption, rktVolumesConfigDefault) {
		node.Attributes["driver."+rktVolumesConfigOption] = "1"
	}
	return true, nil
}

func (d *RktDriver) Periodic() (bool, time.Duration) {
	return true, 15 * time.Second
}

func (d *RktDriver) Prestart(ctx *ExecContext, task *structs.Task) error {
	return nil
}

// Run an existing Rkt image.
func (d *RktDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
	var driverConfig RktDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &driverConfig); err != nil {
		return nil, err
	}

	driverConfig.PortMap = mapMergeStrStr(driverConfig.PortMapRaw...)

	// ACI image
	img := driverConfig.ImageName

	// Build the command.
	var cmdArgs []string

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
		d.logger.Printf("[DEBUG] driver.rkt: added trust prefix: %q", trustPrefix)
	} else {
		// Disble signature verification if the trust command was not run.
		insecure = true
	}
	cmdArgs = append(cmdArgs, "run")

	// Mount /alloc
	allocVolName := fmt.Sprintf("%s-%s-alloc", ctx.AllocID, task.Name)
	cmdArgs = append(cmdArgs, fmt.Sprintf("--volume=%s,kind=host,source=%s", allocVolName, ctx.TaskDir.SharedAllocDir))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--mount=volume=%s,target=%s", allocVolName, allocdir.SharedAllocContainerPath))

	// Mount /local
	localVolName := fmt.Sprintf("%s-%s-local", ctx.AllocID, task.Name)
	cmdArgs = append(cmdArgs, fmt.Sprintf("--volume=%s,kind=host,source=%s", localVolName, ctx.TaskDir.LocalDir))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--mount=volume=%s,target=%s", localVolName, allocdir.TaskLocalContainerPath))

	// Mount /secrets
	secretsVolName := fmt.Sprintf("%s-%s-secrets", ctx.AllocID, task.Name)
	cmdArgs = append(cmdArgs, fmt.Sprintf("--volume=%s,kind=host,source=%s", secretsVolName, ctx.TaskDir.SecretsDir))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--mount=volume=%s,target=%s", secretsVolName, allocdir.TaskSecretsContainerPath))

	// Mount arbitrary volumes if enabled
	if len(driverConfig.Volumes) > 0 {
		if enabled := d.config.ReadBoolDefault(rktVolumesConfigOption, rktVolumesConfigDefault); !enabled {
			return nil, fmt.Errorf("%s is false; cannot use rkt volumes: %+q", rktVolumesConfigOption, driverConfig.Volumes)
		}

		for i, rawvol := range driverConfig.Volumes {
			parts := strings.Split(rawvol, ":")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid rkt volume: %q", rawvol)
			}
			volName := fmt.Sprintf("%s-%s-%d", ctx.AllocID, task.Name, i)
			cmdArgs = append(cmdArgs, fmt.Sprintf("--volume=%s,kind=host,source=%s", volName, parts[0]))
			cmdArgs = append(cmdArgs, fmt.Sprintf("--mount=volume=%s,target=%s", volName, parts[1]))
		}
	}

	cmdArgs = append(cmdArgs, img)
	if insecure {
		cmdArgs = append(cmdArgs, "--insecure-options=all")
	}
	cmdArgs = append(cmdArgs, fmt.Sprintf("--debug=%t", debug))

	// Inject environment variables
	for k, v := range d.taskEnv.EnvMap() {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--set-env=%v=%q", k, v))
	}

	// Check if the user has overridden the exec command.
	if driverConfig.Command != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--exec=%v", driverConfig.Command))
	}

	// Add memory isolator
	cmdArgs = append(cmdArgs, fmt.Sprintf("--memory=%vM", int64(task.Resources.MemoryMB)))

	// Add CPU isolator
	cmdArgs = append(cmdArgs, fmt.Sprintf("--cpu=%vm", int64(task.Resources.CPU)))

	// Add DNS servers
	if len(driverConfig.DNSServers) == 1 && (driverConfig.DNSServers[0] == "host" || driverConfig.DNSServers[0] == "none") {
		// Special case single item lists with the special values "host" or "none"
		cmdArgs = append(cmdArgs, fmt.Sprintf("--dns=%s", driverConfig.DNSServers[0]))
	} else {
		for _, ip := range driverConfig.DNSServers {
			if err := net.ParseIP(ip); err == nil {
				msg := fmt.Errorf("invalid ip address for container dns server %q", ip)
				d.logger.Printf("[DEBUG] driver.rkt: %v", msg)
				return nil, msg
			} else {
				cmdArgs = append(cmdArgs, fmt.Sprintf("--dns=%s", ip))
			}
		}
	}

	// set DNS search domains
	for _, domain := range driverConfig.DNSSearchDomains {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--dns-search=%s", domain))
	}

	// set network
	network := strings.Join(driverConfig.Net, ",")
	if network != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--net=%s", network))
	}

	// Setup port mapping and exposed ports
	if len(task.Resources.Networks) == 0 {
		d.logger.Println("[DEBUG] driver.rkt: No network interfaces are available")
		if len(driverConfig.PortMap) > 0 {
			return nil, fmt.Errorf("Trying to map ports but no network interface is available")
		}
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
			cmdArgs = append(cmdArgs, fmt.Sprintf("--port=%s:%s", containerPort, hostPortStr))
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
			cmdArgs = append(cmdArgs, fmt.Sprintf("--port=%s:%s", containerPort, hostPortStr))
		}

	}

	// Add user passed arguments.
	if len(driverConfig.Args) != 0 {
		parsed := d.taskEnv.ParseAndReplace(driverConfig.Args)

		// Need to start arguments with "--"
		if len(parsed) > 0 {
			cmdArgs = append(cmdArgs, "--")
		}

		for _, arg := range parsed {
			cmdArgs = append(cmdArgs, fmt.Sprintf("%v", arg))
		}
	}

	// Set the host environment variables.
	filter := strings.Split(d.config.ReadDefault("env.blacklist", config.DefaultEnvBlacklist), ",")
	d.taskEnv.AppendHostEnvvars(filter)

	pluginLogFile := filepath.Join(ctx.TaskDir.Dir, fmt.Sprintf("%s-executor.out", task.Name))
	executorConfig := &dstructs.ExecutorConfig{
		LogFile:  pluginLogFile,
		LogLevel: d.config.LogLevel,
	}

	execIntf, pluginClient, err := createExecutor(d.config.LogOutput, d.config, executorConfig)
	if err != nil {
		return nil, err
	}
	executorCtx := &executor.ExecutorContext{
		TaskEnv: d.taskEnv,
		Driver:  "rkt",
		AllocID: ctx.AllocID,
		Task:    task,
		TaskDir: ctx.TaskDir.Dir,
		LogDir:  ctx.TaskDir.LogDir,
	}
	if err := execIntf.SetContext(executorCtx); err != nil {
		pluginClient.Kill()
		return nil, fmt.Errorf("failed to set executor context: %v", err)
	}

	absPath, err := GetAbsolutePath(rktCmd)
	if err != nil {
		return nil, err
	}

	execCmd := &executor.ExecCommand{
		Cmd:  absPath,
		Args: cmdArgs,
		User: task.User,
	}
	ps, err := execIntf.LaunchCmd(execCmd)
	if err != nil {
		pluginClient.Kill()
		return nil, err
	}

	d.logger.Printf("[DEBUG] driver.rkt: started ACI %q with: %v", img, cmdArgs)
	maxKill := d.DriverContext.config.MaxKillTimeout
	h := &rktHandle{
		pluginClient:   pluginClient,
		executor:       execIntf,
		executorPid:    ps.Pid,
		logger:         d.logger,
		killTimeout:    GetKillTimeout(task.KillTimeout, maxKill),
		maxKillTimeout: maxKill,
		doneCh:         make(chan struct{}),
		waitCh:         make(chan *dstructs.WaitResult, 1),
	}
	if err := h.executor.SyncServices(consulContext(d.config, "")); err != nil {
		h.logger.Printf("[ERR] driver.rkt: error registering services for task: %q: %v", task.Name, err)
	}
	go h.run()
	return h, nil
}

func (d *RktDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	// Parse the handle
	pidBytes := []byte(strings.TrimPrefix(handleID, "Rkt:"))
	id := &rktPID{}
	if err := json.Unmarshal(pidBytes, id); err != nil {
		return nil, fmt.Errorf("failed to parse Rkt handle '%s': %v", handleID, err)
	}

	pluginConfig := &plugin.ClientConfig{
		Reattach: id.PluginConfig.PluginConfig(),
	}
	exec, pluginClient, err := createExecutorWithConfig(pluginConfig, d.config.LogOutput)
	if err != nil {
		d.logger.Println("[ERROR] driver.rkt: error connecting to plugin so destroying plugin pid and user pid")
		if e := destroyPlugin(id.PluginConfig.Pid, id.ExecutorPid); e != nil {
			d.logger.Printf("[ERROR] driver.rkt: error destroying plugin and executor pid: %v", e)
		}
		return nil, fmt.Errorf("error connecting to plugin: %v", err)
	}

	ver, _ := exec.Version()
	d.logger.Printf("[DEBUG] driver.rkt: version of executor: %v", ver.Version)
	// Return a driver handle
	h := &rktHandle{
		pluginClient:   pluginClient,
		executorPid:    id.ExecutorPid,
		executor:       exec,
		logger:         d.logger,
		killTimeout:    id.KillTimeout,
		maxKillTimeout: id.MaxKillTimeout,
		doneCh:         make(chan struct{}),
		waitCh:         make(chan *dstructs.WaitResult, 1),
	}
	if err := h.executor.SyncServices(consulContext(d.config, "")); err != nil {
		h.logger.Printf("[ERR] driver.rkt: error registering services: %v", err)
	}
	go h.run()
	return h, nil
}

func (h *rktHandle) ID() string {
	// Return a handle to the PID
	pid := &rktPID{
		PluginConfig:   NewPluginReattachConfig(h.pluginClient.ReattachConfig()),
		KillTimeout:    h.killTimeout,
		MaxKillTimeout: h.maxKillTimeout,
		ExecutorPid:    h.executorPid,
	}
	data, err := json.Marshal(pid)
	if err != nil {
		h.logger.Printf("[ERR] driver.rkt: failed to marshal rkt PID to JSON: %s", err)
	}
	return fmt.Sprintf("Rkt:%s", string(data))
}

func (h *rktHandle) WaitCh() chan *dstructs.WaitResult {
	return h.waitCh
}

func (h *rktHandle) Update(task *structs.Task) error {
	// Store the updated kill timeout.
	h.killTimeout = GetKillTimeout(task.KillTimeout, h.maxKillTimeout)
	h.executor.UpdateTask(task)

	// Update is not possible
	return nil
}

func (h *rktHandle) Signal(s os.Signal) error {
	return fmt.Errorf("Rkt does not support signals")
}

// Kill is used to terminate the task. We send an Interrupt
// and then provide a 5 second grace period before doing a Kill.
func (h *rktHandle) Kill() error {
	h.executor.ShutDown()
	select {
	case <-h.doneCh:
		return nil
	case <-time.After(h.killTimeout):
		return h.executor.Exit()
	}
}

func (h *rktHandle) Stats() (*cstructs.TaskResourceUsage, error) {
	return nil, DriverStatsNotImplemented
}

func (h *rktHandle) run() {
	ps, werr := h.executor.Wait()
	close(h.doneCh)
	if ps.ExitCode == 0 && werr != nil {
		if e := killProcess(h.executorPid); e != nil {
			h.logger.Printf("[ERROR] driver.rkt: error killing user process: %v", e)
		}
	}
	// Remove services
	if err := h.executor.DeregisterServices(); err != nil {
		h.logger.Printf("[ERR] driver.rkt: failed to deregister services: %v", err)
	}

	// Exit the executor
	if err := h.executor.Exit(); err != nil {
		h.logger.Printf("[ERR] driver.rkt: error killing executor: %v", err)
	}
	h.pluginClient.Kill()

	// Send the results
	h.waitCh <- dstructs.NewWaitResult(ps.ExitCode, 0, werr)
	close(h.waitCh)
}
