package driver

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/driver/executor"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/fields"
	shelpers "github.com/hashicorp/nomad/helper/stats"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

var (
	// We store the clients globally to cache the connection to the docker daemon.
	createClients sync.Once

	// client is a docker client with a timeout of 1 minute. This is for doing
	// all operations with the docker daemon besides which are not long running
	// such as creating, killing containers, etc.
	client *docker.Client

	// waitClient is a docker client with no timeouts. This is used for long
	// running operations such as waiting on containers and collect stats
	waitClient *docker.Client

	// The statistics the Docker driver exposes
	DockerMeasuredMemStats = []string{"RSS", "Cache", "Swap", "Max Usage"}
	DockerMeasuredCpuStats = []string{"Throttled Periods", "Throttled Time", "Percent"}

	// recoverableErrTimeouts returns a recoverable error if the error was due
	// to timeouts
	recoverableErrTimeouts = func(err error) *structs.RecoverableError {
		r := false
		if strings.Contains(err.Error(), "Client.Timeout exceeded while awaiting headers") ||
			strings.Contains(err.Error(), "EOF") {
			r = true
		}
		return structs.NewRecoverableError(err, r)
	}
)

const (
	// NoSuchContainerError is returned by the docker daemon if the container
	// does not exist.
	NoSuchContainerError = "No such container"

	// The key populated in Node Attributes to indicate presence of the Docker
	// driver
	dockerDriverAttr = "driver.docker"

	// dockerSELinuxLabelConfigOption is the key for configuring the
	// SELinux label for binds.
	dockerSELinuxLabelConfigOption = "docker.volumes.selinuxlabel"

	// dockerVolumesConfigOption is the key for enabling the use of custom
	// bind volumes to arbitrary host paths.
	dockerVolumesConfigOption  = "docker.volumes.enabled"
	dockerVolumesConfigDefault = true

	// dockerPrivilegedConfigOption is the key for running containers in
	// Docker's privileged mode.
	dockerPrivilegedConfigOption = "docker.privileged.enabled"

	// dockerTimeout is the length of time a request can be outstanding before
	// it is timed out.
	dockerTimeout = 5 * time.Minute
)

type DockerDriver struct {
	DriverContext

	imageID      string
	driverConfig *DockerDriverConfig
}

type DockerDriverAuth struct {
	Username      string `mapstructure:"username"`       // username for the registry
	Password      string `mapstructure:"password"`       // password to access the registry
	Email         string `mapstructure:"email"`          // email address of the user who is allowed to access the registry
	ServerAddress string `mapstructure:"server_address"` // server address of the registry
}

type DockerLoggingOpts struct {
	Type      string              `mapstructure:"type"`
	ConfigRaw []map[string]string `mapstructure:"config"`
	Config    map[string]string   `mapstructure:"-"`
}

type DockerDriverConfig struct {
	ImageName        string              `mapstructure:"image"`              // Container's Image Name
	LoadImages       []string            `mapstructure:"load"`               // LoadImage is array of paths to image archive files
	Command          string              `mapstructure:"command"`            // The Command/Entrypoint to run when the container starts up
	Args             []string            `mapstructure:"args"`               // The arguments to the Command/Entrypoint
	IpcMode          string              `mapstructure:"ipc_mode"`           // The IPC mode of the container - host and none
	NetworkMode      string              `mapstructure:"network_mode"`       // The network mode of the container - host, nat and none
	NetworkAliases   []string            `mapstructure:"network_aliases"`    // The network-scoped alias for the container
	PidMode          string              `mapstructure:"pid_mode"`           // The PID mode of the container - host and none
	UTSMode          string              `mapstructure:"uts_mode"`           // The UTS mode of the container - host and none
	UsernsMode       string              `mapstructure:"userns_mode"`        // The User namespace mode of the container - host and none
	PortMapRaw       []map[string]int    `mapstructure:"port_map"`           //
	PortMap          map[string]int      `mapstructure:"-"`                  // A map of host port labels and the ports exposed on the container
	Privileged       bool                `mapstructure:"privileged"`         // Flag to run the container in privileged mode
	DNSServers       []string            `mapstructure:"dns_servers"`        // DNS Server for containers
	DNSSearchDomains []string            `mapstructure:"dns_search_domains"` // DNS Search domains for containers
	Hostname         string              `mapstructure:"hostname"`           // Hostname for containers
	LabelsRaw        []map[string]string `mapstructure:"labels"`             //
	Labels           map[string]string   `mapstructure:"-"`                  // Labels to set when the container starts up
	Auth             []DockerDriverAuth  `mapstructure:"auth"`               // Authentication credentials for a private Docker registry
	SSL              bool                `mapstructure:"ssl"`                // Flag indicating repository is served via https
	TTY              bool                `mapstructure:"tty"`                // Allocate a Pseudo-TTY
	Interactive      bool                `mapstructure:"interactive"`        // Keep STDIN open even if not attached
	ShmSize          int64               `mapstructure:"shm_size"`           // Size of /dev/shm of the container in bytes
	WorkDir          string              `mapstructure:"work_dir"`           // Working directory inside the container
	Logging          []DockerLoggingOpts `mapstructure:"logging"`            // Logging options for syslog server
	Volumes          []string            `mapstructure:"volumes"`            // Host-Volumes to mount in, syntax: /path/to/host/directory:/destination/path/in/container
	ForcePull        bool                `mapstructure:"force_pull"`         // Always force pull before running image, useful if your tags are mutable
}

// Validate validates a docker driver config
func (c *DockerDriverConfig) Validate() error {
	if c.ImageName == "" {
		return fmt.Errorf("Docker Driver needs an image name")
	}

	c.PortMap = mapMergeStrInt(c.PortMapRaw...)
	c.Labels = mapMergeStrStr(c.LabelsRaw...)
	if len(c.Logging) > 0 {
		c.Logging[0].Config = mapMergeStrStr(c.Logging[0].ConfigRaw...)
	}
	return nil
}

// NewDockerDriverConfig returns a docker driver config by parsing the HCL
// config
func NewDockerDriverConfig(task *structs.Task, env *env.TaskEnvironment) (*DockerDriverConfig, error) {
	var dconf DockerDriverConfig

	// Default to SSL
	dconf.SSL = true

	if err := mapstructure.WeakDecode(task.Config, &dconf); err != nil {
		return nil, err
	}

	// Interpolate everthing that is a string
	dconf.ImageName = env.ReplaceEnv(dconf.ImageName)
	dconf.Command = env.ReplaceEnv(dconf.Command)
	dconf.IpcMode = env.ReplaceEnv(dconf.IpcMode)
	dconf.NetworkMode = env.ReplaceEnv(dconf.NetworkMode)
	dconf.NetworkAliases = env.ParseAndReplace(dconf.NetworkAliases)
	dconf.PidMode = env.ReplaceEnv(dconf.PidMode)
	dconf.UTSMode = env.ReplaceEnv(dconf.UTSMode)
	dconf.Hostname = env.ReplaceEnv(dconf.Hostname)
	dconf.WorkDir = env.ReplaceEnv(dconf.WorkDir)
	dconf.Volumes = env.ParseAndReplace(dconf.Volumes)
	dconf.DNSServers = env.ParseAndReplace(dconf.DNSServers)
	dconf.DNSSearchDomains = env.ParseAndReplace(dconf.DNSSearchDomains)
	dconf.LoadImages = env.ParseAndReplace(dconf.LoadImages)

	for _, m := range dconf.LabelsRaw {
		for k, v := range m {
			delete(m, k)
			m[env.ReplaceEnv(k)] = env.ReplaceEnv(v)
		}
	}

	for i, a := range dconf.Auth {
		dconf.Auth[i].Username = env.ReplaceEnv(a.Username)
		dconf.Auth[i].Password = env.ReplaceEnv(a.Password)
		dconf.Auth[i].Email = env.ReplaceEnv(a.Email)
		dconf.Auth[i].ServerAddress = env.ReplaceEnv(a.ServerAddress)
	}

	for i, l := range dconf.Logging {
		dconf.Logging[i].Type = env.ReplaceEnv(l.Type)
		for _, c := range l.ConfigRaw {
			for k, v := range c {
				delete(c, k)
				c[env.ReplaceEnv(k)] = env.ReplaceEnv(v)
			}
		}
	}

	for _, m := range dconf.PortMapRaw {
		for k, v := range m {
			delete(m, k)
			m[env.ReplaceEnv(k)] = v
		}
	}

	// Remove any http
	if strings.Contains(dconf.ImageName, "https://") {
		dconf.ImageName = strings.Replace(dconf.ImageName, "https://", "", 1)
	}

	if err := dconf.Validate(); err != nil {
		return nil, err
	}
	return &dconf, nil
}

type dockerPID struct {
	Version        string
	ImageID        string
	ContainerID    string
	KillTimeout    time.Duration
	MaxKillTimeout time.Duration
	PluginConfig   *PluginReattachConfig
}

type DockerHandle struct {
	pluginClient      *plugin.Client
	executor          executor.Executor
	client            *docker.Client
	waitClient        *docker.Client
	logger            *log.Logger
	cleanupImage      bool
	imageID           string
	containerID       string
	version           string
	clkSpeed          float64
	killTimeout       time.Duration
	maxKillTimeout    time.Duration
	resourceUsageLock sync.RWMutex
	resourceUsage     *cstructs.TaskResourceUsage
	waitCh            chan *dstructs.WaitResult
	doneCh            chan bool
}

func NewDockerDriver(ctx *DriverContext) Driver {
	return &DockerDriver{DriverContext: *ctx}
}

// Validate is used to validate the driver configuration
func (d *DockerDriver) Validate(config map[string]interface{}) error {
	fd := &fields.FieldData{
		Raw: config,
		Schema: map[string]*fields.FieldSchema{
			"image": &fields.FieldSchema{
				Type:     fields.TypeString,
				Required: true,
			},
			"load": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"command": &fields.FieldSchema{
				Type: fields.TypeString,
			},
			"args": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"ipc_mode": &fields.FieldSchema{
				Type: fields.TypeString,
			},
			"network_mode": &fields.FieldSchema{
				Type: fields.TypeString,
			},
			"network_aliases": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"pid_mode": &fields.FieldSchema{
				Type: fields.TypeString,
			},
			"uts_mode": &fields.FieldSchema{
				Type: fields.TypeString,
			},
			"userns_mode": &fields.FieldSchema{
				Type: fields.TypeString,
			},
			"port_map": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"privileged": &fields.FieldSchema{
				Type: fields.TypeBool,
			},
			"dns_servers": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"dns_search_domains": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"hostname": &fields.FieldSchema{
				Type: fields.TypeString,
			},
			"labels": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"auth": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"ssl": &fields.FieldSchema{
				Type: fields.TypeBool,
			},
			"tty": &fields.FieldSchema{
				Type: fields.TypeBool,
			},
			"interactive": &fields.FieldSchema{
				Type: fields.TypeBool,
			},
			"shm_size": &fields.FieldSchema{
				Type: fields.TypeInt,
			},
			"work_dir": &fields.FieldSchema{
				Type: fields.TypeString,
			},
			"logging": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"volumes": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"force_pull": &fields.FieldSchema{
				Type: fields.TypeBool,
			},
		},
	}

	if err := fd.Validate(); err != nil {
		return err
	}

	return nil
}

func (d *DockerDriver) Abilities() DriverAbilities {
	return DriverAbilities{
		SendSignals: true,
	}
}

func (d *DockerDriver) FSIsolation() cstructs.FSIsolation {
	return cstructs.FSIsolationImage
}

func (d *DockerDriver) Prestart(ctx *ExecContext, task *structs.Task) error {
	driverConfig, err := NewDockerDriverConfig(task, d.taskEnv)
	if err != nil {
		return err
	}

	// Initialize docker API clients
	client, _, err := d.dockerClients()
	if err != nil {
		return fmt.Errorf("Failed to connect to docker daemon: %s", err)
	}

	if err := d.createImage(driverConfig, client, ctx.TaskDir); err != nil {
		return err
	}

	image := driverConfig.ImageName
	// Now that we have the image we can get the image id
	dockerImage, err := client.InspectImage(image)
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: failed getting image id for %s: %s", image, err)
		return fmt.Errorf("Failed to determine image id for `%s`: %s", image, err)
	}
	d.logger.Printf("[DEBUG] driver.docker: identified image %s as %s", image, dockerImage.ID)

	// Set state needed by Start()
	d.imageID = dockerImage.ID
	d.driverConfig = driverConfig
	return nil
}

func (d *DockerDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {

	pluginLogFile := filepath.Join(ctx.TaskDir.Dir, "executor.out")
	executorConfig := &dstructs.ExecutorConfig{
		LogFile:  pluginLogFile,
		LogLevel: d.config.LogLevel,
	}

	exec, pluginClient, err := createExecutor(d.config.LogOutput, d.config, executorConfig)
	if err != nil {
		return nil, err
	}
	executorCtx := &executor.ExecutorContext{
		TaskEnv:        d.taskEnv,
		Task:           task,
		Driver:         "docker",
		AllocID:        ctx.AllocID,
		LogDir:         ctx.TaskDir.LogDir,
		TaskDir:        ctx.TaskDir.Dir,
		PortLowerBound: d.config.ClientMinPort,
		PortUpperBound: d.config.ClientMaxPort,
	}
	if err := exec.SetContext(executorCtx); err != nil {
		pluginClient.Kill()
		return nil, fmt.Errorf("failed to set executor context: %v", err)
	}

	// Only launch syslog server if we're going to use it!
	syslogAddr := ""
	if runtime.GOOS == "darwin" && len(d.driverConfig.Logging) == 0 {
		d.logger.Printf("[DEBUG] driver.docker: disabling syslog driver as Docker for Mac workaround")
	} else if len(d.driverConfig.Logging) == 0 || d.driverConfig.Logging[0].Type == "syslog" {
		ss, err := exec.LaunchSyslogServer()
		if err != nil {
			pluginClient.Kill()
			return nil, fmt.Errorf("failed to start syslog collector: %v", err)
		}
		syslogAddr = ss.Addr
	}

	config, err := d.createContainerConfig(ctx, task, d.driverConfig, syslogAddr)
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: failed to create container configuration for image %s: %s", d.imageID, err)
		pluginClient.Kill()
		return nil, fmt.Errorf("Failed to create container configuration for image %s: %s", d.imageID, err)
	}

	container, rerr := d.createContainer(config)
	if rerr != nil {
		d.logger.Printf("[ERR] driver.docker: failed to create container: %s", rerr)
		pluginClient.Kill()
		rerr.Err = fmt.Sprintf("Failed to create container: %s", rerr.Err)
		return nil, rerr
	}

	d.logger.Printf("[INFO] driver.docker: created container %s", container.ID)

	cleanupImage := d.config.ReadBoolDefault("docker.cleanup.image", true)

	// We don't need to start the container if the container is already running
	// since we don't create containers which are already present on the host
	// and are running
	if !container.State.Running {
		// Start the container
		if err := d.startContainer(container); err != nil {
			d.logger.Printf("[ERR] driver.docker: failed to start container %s: %s", container.ID, err)
			pluginClient.Kill()
			return nil, fmt.Errorf("Failed to start container %s: %s", container.ID, err)
		}
		d.logger.Printf("[INFO] driver.docker: started container %s", container.ID)
	} else {
		d.logger.Printf("[DEBUG] driver.docker: re-attaching to container %s with status %q",
			container.ID, container.State.String())
	}

	// Return a driver handle
	maxKill := d.DriverContext.config.MaxKillTimeout
	h := &DockerHandle{
		client:         client,
		waitClient:     waitClient,
		executor:       exec,
		pluginClient:   pluginClient,
		cleanupImage:   cleanupImage,
		logger:         d.logger,
		imageID:        d.imageID,
		containerID:    container.ID,
		version:        d.config.Version,
		killTimeout:    GetKillTimeout(task.KillTimeout, maxKill),
		maxKillTimeout: maxKill,
		doneCh:         make(chan bool),
		waitCh:         make(chan *dstructs.WaitResult, 1),
	}
	if err := exec.SyncServices(consulContext(d.config, container.ID)); err != nil {
		d.logger.Printf("[ERR] driver.docker: error registering services with consul for task: %q: %v", task.Name, err)
	}
	go h.collectStats()
	go h.run()
	return h, nil
}

// dockerClients creates two *docker.Client, one for long running operations and
// the other for shorter operations. In test / dev mode we can use ENV vars to
// connect to the docker daemon. In production mode we will read docker.endpoint
// from the config file.
func (d *DockerDriver) dockerClients() (*docker.Client, *docker.Client, error) {
	if client != nil && waitClient != nil {
		return client, waitClient, nil
	}

	var err error
	var merr multierror.Error
	createClients.Do(func() {
		if err = shelpers.Init(); err != nil {
			d.logger.Printf("[FATAL] driver.docker: unable to initialize stats: %v", err)
			return
		}

		// Default to using whatever is configured in docker.endpoint. If this is
		// not specified we'll fall back on NewClientFromEnv which reads config from
		// the DOCKER_* environment variables DOCKER_HOST, DOCKER_TLS_VERIFY, and
		// DOCKER_CERT_PATH. This allows us to lock down the config in production
		// but also accept the standard ENV configs for dev and test.
		dockerEndpoint := d.config.Read("docker.endpoint")
		if dockerEndpoint != "" {
			cert := d.config.Read("docker.tls.cert")
			key := d.config.Read("docker.tls.key")
			ca := d.config.Read("docker.tls.ca")

			if cert+key+ca != "" {
				d.logger.Printf("[DEBUG] driver.docker: using TLS client connection to %s", dockerEndpoint)
				client, err = docker.NewTLSClient(dockerEndpoint, cert, key, ca)
				if err != nil {
					merr.Errors = append(merr.Errors, err)
				}
				waitClient, err = docker.NewTLSClient(dockerEndpoint, cert, key, ca)
				if err != nil {
					merr.Errors = append(merr.Errors, err)
				}
			} else {
				d.logger.Printf("[DEBUG] driver.docker: using standard client connection to %s", dockerEndpoint)
				client, err = docker.NewClient(dockerEndpoint)
				if err != nil {
					merr.Errors = append(merr.Errors, err)
				}
				waitClient, err = docker.NewClient(dockerEndpoint)
				if err != nil {
					merr.Errors = append(merr.Errors, err)
				}
			}
			client.SetTimeout(dockerTimeout)
			return
		}

		d.logger.Println("[DEBUG] driver.docker: using client connection initialized from environment")
		client, err = docker.NewClientFromEnv()
		if err != nil {
			merr.Errors = append(merr.Errors, err)
		}
		client.SetTimeout(dockerTimeout)

		waitClient, err = docker.NewClientFromEnv()
		if err != nil {
			merr.Errors = append(merr.Errors, err)
		}
	})
	return client, waitClient, merr.ErrorOrNil()
}

func (d *DockerDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Get the current status so that we can log any debug messages only if the
	// state changes
	_, currentlyEnabled := node.Attributes[dockerDriverAttr]

	// Initialize docker API clients
	client, _, err := d.dockerClients()
	if err != nil {
		delete(node.Attributes, dockerDriverAttr)
		if currentlyEnabled {
			d.logger.Printf("[INFO] driver.docker: failed to initialize client: %s", err)
		}
		return false, nil
	}

	privileged := d.config.ReadBoolDefault(dockerPrivilegedConfigOption, false)
	if privileged {
		node.Attributes[dockerPrivilegedConfigOption] = "1"
	}

	// This is the first operation taken on the client so we'll try to
	// establish a connection to the Docker daemon. If this fails it means
	// Docker isn't available so we'll simply disable the docker driver.
	env, err := client.Version()
	if err != nil {
		if currentlyEnabled {
			d.logger.Printf("[DEBUG] driver.docker: could not connect to docker daemon at %s: %s", client.Endpoint(), err)
		}
		delete(node.Attributes, dockerDriverAttr)
		return false, nil
	}

	node.Attributes[dockerDriverAttr] = "1"
	node.Attributes["driver.docker.version"] = env.Get("Version")

	// Advertise if this node supports Docker volumes
	if d.config.ReadBoolDefault(dockerVolumesConfigOption, dockerVolumesConfigDefault) {
		node.Attributes["driver."+dockerVolumesConfigOption] = "1"
	}

	return true, nil
}

func (d *DockerDriver) containerBinds(driverConfig *DockerDriverConfig, taskDir *allocdir.TaskDir,
	task *structs.Task) ([]string, error) {

	allocDirBind := fmt.Sprintf("%s:%s", taskDir.SharedAllocDir, allocdir.SharedAllocContainerPath)
	taskLocalBind := fmt.Sprintf("%s:%s", taskDir.LocalDir, allocdir.TaskLocalContainerPath)
	secretDirBind := fmt.Sprintf("%s:%s", taskDir.SecretsDir, allocdir.TaskSecretsContainerPath)
	binds := []string{allocDirBind, taskLocalBind, secretDirBind}

	volumesEnabled := d.config.ReadBoolDefault(dockerVolumesConfigOption, dockerVolumesConfigDefault)

	for _, userbind := range driverConfig.Volumes {
		parts := strings.Split(userbind, ":")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid docker volume: %q", userbind)
		}

		// Resolve dotted path segments
		parts[0] = filepath.Clean(parts[0])

		// Absolute paths aren't always supported
		if filepath.IsAbs(parts[0]) {
			if !volumesEnabled {
				// Disallow mounting arbitrary absolute paths
				return nil, fmt.Errorf("%s is false; cannot mount host paths: %+q", dockerVolumesConfigOption, userbind)
			}
			binds = append(binds, userbind)
			continue
		}

		// Relative paths are always allowed as they mount within a container
		// Expand path relative to alloc dir
		parts[0] = filepath.Join(taskDir.Dir, parts[0])
		binds = append(binds, strings.Join(parts, ":"))
	}

	if selinuxLabel := d.config.Read(dockerSELinuxLabelConfigOption); selinuxLabel != "" {
		// Apply SELinux Label to each volume
		for i := range binds {
			binds[i] = fmt.Sprintf("%s:%s", binds[i], selinuxLabel)
		}
	}

	return binds, nil
}

// createContainerConfig initializes a struct needed to call docker.client.CreateContainer()
func (d *DockerDriver) createContainerConfig(ctx *ExecContext, task *structs.Task,
	driverConfig *DockerDriverConfig, syslogAddr string) (docker.CreateContainerOptions, error) {
	var c docker.CreateContainerOptions
	if task.Resources == nil {
		// Guard against missing resources. We should never have been able to
		// schedule a job without specifying this.
		d.logger.Println("[ERR] driver.docker: task.Resources is empty")
		return c, fmt.Errorf("task.Resources is empty")
	}

	binds, err := d.containerBinds(driverConfig, ctx.TaskDir, task)
	if err != nil {
		return c, err
	}

	config := &docker.Config{
		Image:     driverConfig.ImageName,
		Hostname:  driverConfig.Hostname,
		User:      task.User,
		Tty:       driverConfig.TTY,
		OpenStdin: driverConfig.Interactive,
	}

	if driverConfig.WorkDir != "" {
		config.WorkingDir = driverConfig.WorkDir
	}

	memLimit := int64(task.Resources.MemoryMB) * 1024 * 1024

	if len(driverConfig.Logging) == 0 {
		if runtime.GOOS != "darwin" {
			d.logger.Printf("[DEBUG] driver.docker: Setting default logging options to syslog and %s", syslogAddr)
			driverConfig.Logging = []DockerLoggingOpts{
				{Type: "syslog", Config: map[string]string{"syslog-address": syslogAddr}},
			}
		} else {
			d.logger.Printf("[DEBUG] driver.docker: deferring logging to docker on Docker for Mac")
		}
	}

	hostConfig := &docker.HostConfig{
		// Convert MB to bytes. This is an absolute value.
		Memory:     memLimit,
		MemorySwap: memLimit, // MemorySwap is memory + swap.
		// Convert Mhz to shares. This is a relative value.
		CPUShares: int64(task.Resources.CPU),

		// Binds are used to mount a host volume into the container. We mount a
		// local directory for storage and a shared alloc directory that can be
		// used to share data between different tasks in the same task group.
		Binds: binds,
	}

	if len(driverConfig.Logging) != 0 {
		d.logger.Printf("[DEBUG] driver.docker: Using config for logging: %+v", driverConfig.Logging[0])
		hostConfig.LogConfig = docker.LogConfig{
			Type:   driverConfig.Logging[0].Type,
			Config: driverConfig.Logging[0].Config,
		}
	}

	d.logger.Printf("[DEBUG] driver.docker: using %d bytes memory for %s", hostConfig.Memory, task.Name)
	d.logger.Printf("[DEBUG] driver.docker: using %d cpu shares for %s", hostConfig.CPUShares, task.Name)
	d.logger.Printf("[DEBUG] driver.docker: binding directories %#v for %s", hostConfig.Binds, task.Name)

	//  set privileged mode
	hostPrivileged := d.config.ReadBoolDefault(dockerPrivilegedConfigOption, false)
	if driverConfig.Privileged && !hostPrivileged {
		return c, fmt.Errorf(`Docker privileged mode is disabled on this Nomad agent`)
	}
	hostConfig.Privileged = driverConfig.Privileged

	// set SHM size
	if driverConfig.ShmSize != 0 {
		hostConfig.ShmSize = driverConfig.ShmSize
	}

	// set DNS servers
	for _, ip := range driverConfig.DNSServers {
		if net.ParseIP(ip) != nil {
			hostConfig.DNS = append(hostConfig.DNS, ip)
		} else {
			d.logger.Printf("[ERR] driver.docker: invalid ip address for container dns server: %s", ip)
		}
	}

	// set DNS search domains
	for _, domain := range driverConfig.DNSSearchDomains {
		hostConfig.DNSSearch = append(hostConfig.DNSSearch, domain)
	}

	hostConfig.IpcMode = driverConfig.IpcMode
	hostConfig.PidMode = driverConfig.PidMode
	hostConfig.UTSMode = driverConfig.UTSMode
	hostConfig.UsernsMode = driverConfig.UsernsMode

	hostConfig.NetworkMode = driverConfig.NetworkMode
	if hostConfig.NetworkMode == "" {
		// docker default
		d.logger.Printf("[DEBUG] driver.docker: networking mode not specified; defaulting to %s", defaultNetworkMode)
		hostConfig.NetworkMode = defaultNetworkMode
	}

	// Setup port mapping and exposed ports
	if len(task.Resources.Networks) == 0 {
		d.logger.Println("[DEBUG] driver.docker: No network interfaces are available")
		if len(driverConfig.PortMap) > 0 {
			return c, fmt.Errorf("Trying to map ports but no network interface is available")
		}
	} else {
		// TODO add support for more than one network
		network := task.Resources.Networks[0]
		publishedPorts := map[docker.Port][]docker.PortBinding{}
		exposedPorts := map[docker.Port]struct{}{}

		for _, port := range network.ReservedPorts {
			// By default we will map the allocated port 1:1 to the container
			containerPortInt := port.Value

			// If the user has mapped a port using port_map we'll change it here
			if mapped, ok := driverConfig.PortMap[port.Label]; ok {
				containerPortInt = mapped
			}

			hostPortStr := strconv.Itoa(port.Value)
			containerPort := docker.Port(strconv.Itoa(containerPortInt))

			publishedPorts[containerPort+"/tcp"] = getPortBinding(network.IP, hostPortStr)
			publishedPorts[containerPort+"/udp"] = getPortBinding(network.IP, hostPortStr)
			d.logger.Printf("[DEBUG] driver.docker: allocated port %s:%d -> %d (static)", network.IP, port.Value, port.Value)

			exposedPorts[containerPort+"/tcp"] = struct{}{}
			exposedPorts[containerPort+"/udp"] = struct{}{}
			d.logger.Printf("[DEBUG] driver.docker: exposed port %d", port.Value)
		}

		for _, port := range network.DynamicPorts {
			// By default we will map the allocated port 1:1 to the container
			containerPortInt := port.Value

			// If the user has mapped a port using port_map we'll change it here
			if mapped, ok := driverConfig.PortMap[port.Label]; ok {
				containerPortInt = mapped
			}

			hostPortStr := strconv.Itoa(port.Value)
			containerPort := docker.Port(strconv.Itoa(containerPortInt))

			publishedPorts[containerPort+"/tcp"] = getPortBinding(network.IP, hostPortStr)
			publishedPorts[containerPort+"/udp"] = getPortBinding(network.IP, hostPortStr)
			d.logger.Printf("[DEBUG] driver.docker: allocated port %s:%d -> %d (mapped)", network.IP, port.Value, containerPortInt)

			exposedPorts[containerPort+"/tcp"] = struct{}{}
			exposedPorts[containerPort+"/udp"] = struct{}{}
			d.logger.Printf("[DEBUG] driver.docker: exposed port %s", containerPort)
		}

		d.taskEnv.SetPortMap(driverConfig.PortMap)

		hostConfig.PortBindings = publishedPorts
		config.ExposedPorts = exposedPorts
	}

	d.taskEnv.Build()
	parsedArgs := d.taskEnv.ParseAndReplace(driverConfig.Args)

	// If the user specified a custom command to run as their entrypoint, we'll
	// inject it here.
	if driverConfig.Command != "" {
		// Validate command
		if err := validateCommand(driverConfig.Command, "args"); err != nil {
			return c, err
		}

		cmd := []string{driverConfig.Command}
		if len(driverConfig.Args) != 0 {
			cmd = append(cmd, parsedArgs...)
		}
		d.logger.Printf("[DEBUG] driver.docker: setting container startup command to: %s", strings.Join(cmd, " "))
		config.Cmd = cmd
	} else if len(driverConfig.Args) != 0 {
		config.Cmd = parsedArgs
	}

	if len(driverConfig.Labels) > 0 {
		config.Labels = driverConfig.Labels
		d.logger.Printf("[DEBUG] driver.docker: applied labels on the container: %+v", config.Labels)
	}

	config.Env = d.taskEnv.EnvList()

	containerName := fmt.Sprintf("%s-%s", task.Name, ctx.AllocID)
	d.logger.Printf("[DEBUG] driver.docker: setting container name to: %s", containerName)

	var networkingConfig *docker.NetworkingConfig
	if len(driverConfig.NetworkAliases) > 0 {
		networkingConfig = &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{
				hostConfig.NetworkMode: &docker.EndpointConfig{
					Aliases: driverConfig.NetworkAliases,
				},
			},
		}

		d.logger.Printf("[DEBUG] driver.docker: using network_mode %q with network aliases: %v",
			hostConfig.NetworkMode, strings.Join(driverConfig.NetworkAliases, ", "))
	}

	return docker.CreateContainerOptions{
		Name:             containerName,
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
	}, nil
}

var (
	// imageNotFoundMatcher is a regex expression that matches the image not
	// found error Docker returns.
	imageNotFoundMatcher = regexp.MustCompile(`Error: image .+ not found`)
)

// recoverablePullError wraps the error gotten when trying to pull and image if
// the error is recoverable.
func (d *DockerDriver) recoverablePullError(err error, image string) error {
	recoverable := true
	if imageNotFoundMatcher.MatchString(err.Error()) {
		recoverable = false
	}
	return structs.NewRecoverableError(fmt.Errorf("Failed to pull `%s`: %s", image, err), recoverable)
}

func (d *DockerDriver) Periodic() (bool, time.Duration) {
	return true, 15 * time.Second
}

// createImage creates a docker image either by pulling it from a registry or by
// loading it from the file system
func (d *DockerDriver) createImage(driverConfig *DockerDriverConfig, client *docker.Client, taskDir *allocdir.TaskDir) error {
	image := driverConfig.ImageName
	repo, tag := docker.ParseRepositoryTag(image)
	if tag == "" {
		tag = "latest"
	}

	var dockerImage *docker.Image
	var err error
	// We're going to check whether the image is already downloaded. If the tag
	// is "latest", or ForcePull is set, we have to check for a new version every time so we don't
	// bother to check and cache the id here. We'll download first, then cache.
	if driverConfig.ForcePull {
		d.logger.Printf("[DEBUG] driver.docker: force pull image '%s:%s' instead of inspecting local", repo, tag)
	} else if tag != "latest" {
		dockerImage, err = client.InspectImage(image)
	}

	// Download the image
	if dockerImage == nil {
		if len(driverConfig.LoadImages) > 0 {
			return d.loadImage(driverConfig, client, taskDir)
		}

		return d.pullImage(driverConfig, client, repo, tag)
	}
	return err
}

// pullImage creates an image by pulling it from a docker registry
func (d *DockerDriver) pullImage(driverConfig *DockerDriverConfig, client *docker.Client, repo string, tag string) error {
	pullOptions := docker.PullImageOptions{
		Repository: repo,
		Tag:        tag,
	}

	authOptions := docker.AuthConfiguration{}
	if len(driverConfig.Auth) != 0 {
		authOptions = docker.AuthConfiguration{
			Username:      driverConfig.Auth[0].Username,
			Password:      driverConfig.Auth[0].Password,
			Email:         driverConfig.Auth[0].Email,
			ServerAddress: driverConfig.Auth[0].ServerAddress,
		}
	}

	if authConfigFile := d.config.Read("docker.auth.config"); authConfigFile != "" {
		if f, err := os.Open(authConfigFile); err == nil {
			defer f.Close()
			var authConfigurations *docker.AuthConfigurations
			if authConfigurations, err = docker.NewAuthConfigurations(f); err != nil {
				return fmt.Errorf("Failed to create docker auth object: %v", err)
			}

			authConfigurationKey := ""
			if driverConfig.SSL {
				authConfigurationKey += "https://"
			}

			authConfigurationKey += strings.Split(driverConfig.ImageName, "/")[0]
			if authConfiguration, ok := authConfigurations.Configs[authConfigurationKey]; ok {
				authOptions = authConfiguration
			} else {
				d.logger.Printf("[INFO] Failed to find docker auth with key %s", authConfigurationKey)
			}
		} else {
			return fmt.Errorf("Failed to open auth config file: %v, error: %v", authConfigFile, err)
		}
	}

	d.emitEvent("Downloading image %s:%s", repo, tag)
	err := client.PullImage(pullOptions, authOptions)
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: failed pulling container %s:%s: %s", repo, tag, err)
		return d.recoverablePullError(err, driverConfig.ImageName)
	}
	d.logger.Printf("[DEBUG] driver.docker: docker pull %s:%s succeeded", repo, tag)
	return nil
}

// loadImage creates an image by loading it from the file system
func (d *DockerDriver) loadImage(driverConfig *DockerDriverConfig, client *docker.Client, taskDir *allocdir.TaskDir) error {
	var errors multierror.Error
	for _, image := range driverConfig.LoadImages {
		archive := filepath.Join(taskDir.LocalDir, image)
		d.logger.Printf("[DEBUG] driver.docker: loading image from: %v", archive)
		f, err := os.Open(archive)
		if err != nil {
			errors.Errors = append(errors.Errors, fmt.Errorf("unable to open image archive: %v", err))
			continue
		}
		if err := client.LoadImage(docker.LoadImageOptions{InputStream: f}); err != nil {
			errors.Errors = append(errors.Errors, err)
		}
		f.Close()
	}
	return errors.ErrorOrNil()
}

// createContainer creates the container given the passed configuration. It
// attempts to handle any transient Docker errors.
func (d *DockerDriver) createContainer(config docker.CreateContainerOptions) (*docker.Container, *structs.RecoverableError) {
	// Create a container
	attempted := 0
CREATE:
	container, createErr := client.CreateContainer(config)
	if createErr == nil {
		return container, nil
	}

	d.logger.Printf("[DEBUG] driver.docker: failed to create container %q (attempt %d): %v", config.Name, attempted+1, createErr)
	if strings.Contains(strings.ToLower(createErr.Error()), "container already exists") {
		containers, err := client.ListContainers(docker.ListContainersOptions{
			All: true,
		})
		if err != nil {
			d.logger.Printf("[ERR] driver.docker: failed to query list of containers matching name:%s", config.Name)
			return nil, recoverableErrTimeouts(fmt.Errorf("Failed to query list of containers: %s", err))
		}

		// Delete matching containers
		// Adding a / infront of the container name since Docker returns the
		// container names with a / pre-pended to the Nomad generated container names
		containerName := "/" + config.Name
		d.logger.Printf("[DEBUG] driver.docker: searching for container name %q to purge", containerName)
		for _, container := range containers {
			d.logger.Printf("[DEBUG] driver.docker: listed container %+v", container)
			found := false
			for _, name := range container.Names {
				if name == containerName {
					found = true
					break
				}
			}

			if !found {
				continue
			}

			// Inspect the container and if the container isn't dead then return
			// the container
			container, err := client.InspectContainer(container.ID)
			if err != nil {
				return nil, recoverableErrTimeouts(fmt.Errorf("Failed to inspect container %s: %s", container.ID, err))
			}
			if container != nil && (container.State.Running || container.State.FinishedAt.IsZero()) {
				return container, nil
			}

			err = client.RemoveContainer(docker.RemoveContainerOptions{
				ID:    container.ID,
				Force: true,
			})
			if err != nil {
				d.logger.Printf("[ERR] driver.docker: failed to purge container %s", container.ID)
				return nil, recoverableErrTimeouts(fmt.Errorf("Failed to purge container %s: %s", container.ID, err))
			} else if err == nil {
				d.logger.Printf("[INFO] driver.docker: purged container %s", container.ID)
			}
		}

		if attempted < 5 {
			attempted++
			time.Sleep(1 * time.Second)
			goto CREATE
		}
	}

	return nil, recoverableErrTimeouts(createErr)
}

// startContainer starts the passed container. It attempts to handle any
// transient Docker errors.
func (d *DockerDriver) startContainer(c *docker.Container) *structs.RecoverableError {
	// Start a container
	attempted := 0
START:
	startErr := client.StartContainer(c.ID, c.HostConfig)
	if startErr == nil {
		return nil
	}

	d.logger.Printf("[DEBUG] driver.docker: failed to start container %q (attempt %d): %v", c.ID, attempted+1, startErr)

	// If it is a 500 error it is likely we can retry and be successful
	if strings.Contains(startErr.Error(), "API error (500)") {
		if attempted < 5 {
			attempted++
			time.Sleep(1 * time.Second)
			goto START
		}
	}

	return recoverableErrTimeouts(startErr)
}

func (d *DockerDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	cleanupImage := d.config.ReadBoolDefault("docker.cleanup.image", true)

	// Split the handle
	pidBytes := []byte(strings.TrimPrefix(handleID, "DOCKER:"))
	pid := &dockerPID{}
	if err := json.Unmarshal(pidBytes, pid); err != nil {
		return nil, fmt.Errorf("Failed to parse handle '%s': %v", handleID, err)
	}
	d.logger.Printf("[INFO] driver.docker: re-attaching to docker process: %s", pid.ContainerID)
	d.logger.Printf("[DEBUG] driver.docker: re-attached to handle: %s", handleID)
	pluginConfig := &plugin.ClientConfig{
		Reattach: pid.PluginConfig.PluginConfig(),
	}

	client, waitClient, err := d.dockerClients()
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to docker daemon: %s", err)
	}

	// Look for a running container with this ID
	containers, err := client.ListContainers(docker.ListContainersOptions{
		Filters: map[string][]string{
			"id": []string{pid.ContainerID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to query for container %s: %v", pid.ContainerID, err)
	}

	found := false
	for _, container := range containers {
		if container.ID == pid.ContainerID {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("Failed to find container %s", pid.ContainerID)
	}
	exec, pluginClient, err := createExecutorWithConfig(pluginConfig, d.config.LogOutput)
	if err != nil {
		d.logger.Printf("[INFO] driver.docker: couldn't re-attach to the plugin process: %v", err)
		d.logger.Printf("[DEBUG] driver.docker: stopping container %q", pid.ContainerID)
		if e := client.StopContainer(pid.ContainerID, uint(pid.KillTimeout.Seconds())); e != nil {
			d.logger.Printf("[DEBUG] driver.docker: couldn't stop container: %v", e)
		}
		return nil, err
	}

	ver, _ := exec.Version()
	d.logger.Printf("[DEBUG] driver.docker: version of executor: %v", ver.Version)

	// Return a driver handle
	h := &DockerHandle{
		client:         client,
		waitClient:     waitClient,
		executor:       exec,
		pluginClient:   pluginClient,
		cleanupImage:   cleanupImage,
		logger:         d.logger,
		imageID:        pid.ImageID,
		containerID:    pid.ContainerID,
		version:        pid.Version,
		killTimeout:    pid.KillTimeout,
		maxKillTimeout: pid.MaxKillTimeout,
		doneCh:         make(chan bool),
		waitCh:         make(chan *dstructs.WaitResult, 1),
	}
	if err := exec.SyncServices(consulContext(d.config, pid.ContainerID)); err != nil {
		h.logger.Printf("[ERR] driver.docker: error registering services with consul: %v", err)
	}

	go h.collectStats()
	go h.run()
	return h, nil
}

func (h *DockerHandle) ID() string {
	// Return a handle to the PID
	pid := dockerPID{
		Version:        h.version,
		ImageID:        h.imageID,
		ContainerID:    h.containerID,
		KillTimeout:    h.killTimeout,
		MaxKillTimeout: h.maxKillTimeout,
		PluginConfig:   NewPluginReattachConfig(h.pluginClient.ReattachConfig()),
	}
	data, err := json.Marshal(pid)
	if err != nil {
		h.logger.Printf("[ERR] driver.docker: failed to marshal docker PID to JSON: %s", err)
	}
	return fmt.Sprintf("DOCKER:%s", string(data))
}

func (h *DockerHandle) ContainerID() string {
	return h.containerID
}

func (h *DockerHandle) WaitCh() chan *dstructs.WaitResult {
	return h.waitCh
}

func (h *DockerHandle) Update(task *structs.Task) error {
	// Store the updated kill timeout.
	h.killTimeout = GetKillTimeout(task.KillTimeout, h.maxKillTimeout)
	if err := h.executor.UpdateTask(task); err != nil {
		h.logger.Printf("[DEBUG] driver.docker: failed to update log config: %v", err)
	}

	// Update is not possible
	return nil
}

func (h *DockerHandle) Signal(s os.Signal) error {
	// Convert types
	sysSig, ok := s.(syscall.Signal)
	if !ok {
		return fmt.Errorf("Failed to determine signal number")
	}

	dockerSignal := docker.Signal(sysSig)
	opts := docker.KillContainerOptions{
		ID:     h.containerID,
		Signal: dockerSignal,
	}
	return h.client.KillContainer(opts)

}

// Kill is used to terminate the task. This uses `docker stop -t killTimeout`
func (h *DockerHandle) Kill() error {
	// Stop the container
	err := h.client.StopContainer(h.containerID, uint(h.killTimeout.Seconds()))
	if err != nil {
		h.executor.Exit()
		h.pluginClient.Kill()

		// Container has already been removed.
		if strings.Contains(err.Error(), NoSuchContainerError) {
			h.logger.Printf("[DEBUG] driver.docker: attempted to stop non-existent container %s", h.containerID)
			return nil
		}
		h.logger.Printf("[ERR] driver.docker: failed to stop container %s: %v", h.containerID, err)
		return fmt.Errorf("Failed to stop container %s: %s", h.containerID, err)
	}
	h.logger.Printf("[INFO] driver.docker: stopped container %s", h.containerID)
	return nil
}

func (h *DockerHandle) Stats() (*cstructs.TaskResourceUsage, error) {
	h.resourceUsageLock.RLock()
	defer h.resourceUsageLock.RUnlock()
	var err error
	if h.resourceUsage == nil {
		err = fmt.Errorf("stats collection hasn't started yet")
	}
	return h.resourceUsage, err
}

func (h *DockerHandle) run() {
	// Wait for it...
	exitCode, werr := h.waitClient.WaitContainer(h.containerID)
	if werr != nil {
		h.logger.Printf("[ERR] driver.docker: failed to wait for %s; container already terminated", h.containerID)
	}

	if exitCode != 0 {
		werr = fmt.Errorf("Docker container exited with non-zero exit code: %d", exitCode)
	}

	close(h.doneCh)

	// Remove services
	if err := h.executor.DeregisterServices(); err != nil {
		h.logger.Printf("[ERR] driver.docker: error deregistering services: %v", err)
	}

	// Shutdown the syslog collector
	if err := h.executor.Exit(); err != nil {
		h.logger.Printf("[ERR] driver.docker: failed to kill the syslog collector: %v", err)
	}
	h.pluginClient.Kill()

	// Stop the container just incase the docker daemon's wait returned
	// incorrectly
	if err := h.client.StopContainer(h.containerID, 0); err != nil {
		_, noSuchContainer := err.(*docker.NoSuchContainer)
		_, containerNotRunning := err.(*docker.ContainerNotRunning)
		if !containerNotRunning && !noSuchContainer {
			h.logger.Printf("[ERR] driver.docker: error stopping container: %v", err)
		}
	}

	// Remove the container
	if err := h.client.RemoveContainer(docker.RemoveContainerOptions{ID: h.containerID, RemoveVolumes: true, Force: true}); err != nil {
		h.logger.Printf("[ERR] driver.docker: error removing container: %v", err)
	}

	// Cleanup the image
	if h.cleanupImage {
		if err := h.client.RemoveImage(h.imageID); err != nil {
			h.logger.Printf("[DEBUG] driver.docker: error removing image: %v", err)
		}
	}

	// Send the results
	h.waitCh <- dstructs.NewWaitResult(exitCode, 0, werr)
	close(h.waitCh)
}

// collectStats starts collecting resource usage stats of a docker container
func (h *DockerHandle) collectStats() {
	statsCh := make(chan *docker.Stats)
	statsOpts := docker.StatsOptions{ID: h.containerID, Done: h.doneCh, Stats: statsCh, Stream: true}
	go func() {
		//TODO handle Stats error
		if err := h.waitClient.Stats(statsOpts); err != nil {
			h.logger.Printf("[DEBUG] driver.docker: error collecting stats from container %s: %v", h.containerID, err)
		}
	}()
	numCores := runtime.NumCPU()
	for {
		select {
		case s := <-statsCh:
			if s != nil {
				ms := &cstructs.MemoryStats{
					RSS:      s.MemoryStats.Stats.Rss,
					Cache:    s.MemoryStats.Stats.Cache,
					Swap:     s.MemoryStats.Stats.Swap,
					MaxUsage: s.MemoryStats.MaxUsage,
					Measured: DockerMeasuredMemStats,
				}

				cs := &cstructs.CpuStats{
					ThrottledPeriods: s.CPUStats.ThrottlingData.ThrottledPeriods,
					ThrottledTime:    s.CPUStats.ThrottlingData.ThrottledTime,
					Measured:         DockerMeasuredCpuStats,
				}

				// Calculate percentage
				cores := len(s.CPUStats.CPUUsage.PercpuUsage)
				cs.Percent = calculatePercent(
					s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage,
					s.CPUStats.SystemCPUUsage, s.PreCPUStats.SystemCPUUsage, cores)
				cs.SystemMode = calculatePercent(
					s.CPUStats.CPUUsage.UsageInKernelmode, s.PreCPUStats.CPUUsage.UsageInKernelmode,
					s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage, cores)
				cs.UserMode = calculatePercent(
					s.CPUStats.CPUUsage.UsageInUsermode, s.PreCPUStats.CPUUsage.UsageInUsermode,
					s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage, cores)
				cs.TotalTicks = (cs.Percent / 100) * shelpers.TotalTicksAvailable() / float64(numCores)

				h.resourceUsageLock.Lock()
				h.resourceUsage = &cstructs.TaskResourceUsage{
					ResourceUsage: &cstructs.ResourceUsage{
						MemoryStats: ms,
						CpuStats:    cs,
					},
					Timestamp: s.Read.UTC().UnixNano(),
				}
				h.resourceUsageLock.Unlock()
			}
		case <-h.doneCh:
			return
		}
	}
}

func calculatePercent(newSample, oldSample, newTotal, oldTotal uint64, cores int) float64 {
	numerator := newSample - oldSample
	denom := newTotal - oldTotal
	if numerator <= 0 || denom <= 0 {
		return 0.0
	}

	return (float64(numerator) / float64(denom)) * float64(cores) * 100.0
}
