package docker

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/consul-template/signals"
	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/drivers/docker/docklog"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/plugins/shared/loader"
)

const (
	// NoSuchContainerError is returned by the docker daemon if the container
	// does not exist.
	NoSuchContainerError = "No such container"

	// pluginName is the name of the plugin
	pluginName = "docker"

	// fingerprintPeriod is the interval at which the driver will send fingerprint responses
	fingerprintPeriod = 30 * time.Second

	// dockerTimeout is the length of time a request can be outstanding before
	// it is timed out.
	dockerTimeout = 5 * time.Minute

	// dockerBasicCaps is comma-separated list of Linux capabilities that are
	// allowed by docker by default, as documented in
	// https://docs.docker.com/engine/reference/run/#block-io-bandwidth-blkio-constraint
	dockerBasicCaps = "CHOWN,DAC_OVERRIDE,FSETID,FOWNER,MKNOD,NET_RAW,SETGID," +
		"SETUID,SETFCAP,SETPCAP,NET_BIND_SERVICE,SYS_CHROOT,KILL,AUDIT_WRITE"

	// dockerAuthHelperPrefix is the prefix to attach to the credential helper
	// and should be found in the $PATH. Example: ${prefix-}${helper-name}
	dockerAuthHelperPrefix = "docker-credential-"
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
		Factory: func(l hclog.Logger) interface{} { return NewDockerDriver(l) },
	}
)

func PluginLoader(opts map[string]string) (map[string]interface{}, error) {
	conf := map[string]interface{}{}
	if v, ok := opts["docker.endpoint"]; ok {
		conf["endpoint"] = v
	}
	if v, ok := opts["docker.auth.config"]; ok {
		conf["auth_config"] = v
	}
	if v, ok := opts["docker.auth.helper"]; ok {
		conf["auth_helper"] = v
	}
	if _, ok := opts["docker.tls.cert"]; ok {
		conf["tls"] = map[string]interface{}{
			"cert": opts["docker.tls.cert"],
			"key":  opts["docker.tls.key"],
			"ca":   opts["docker.tls.ca"],
		}
	}
	if v, ok := opts["docker.cleanup.image.delay"]; ok {
		conf["image_gc_delay"] = v
	}
	if v, ok := opts["docker.volumes.selinuxlabel"]; ok {
		conf["volumes_selinuxlabel"] = v
	}
	if v, ok := opts["docker.caps.whitelist"]; ok {
		conf["allow_caps"] = strings.Split(v, ",")
	}
	if v, err := strconv.ParseBool(opts["docker.cleanup.image"]); err == nil {
		conf["image_gc"] = v
	}
	if v, err := strconv.ParseBool(opts["docker.volumes.enabled"]); err == nil {
		conf["volumes_enabled"] = v
	}
	if v, err := strconv.ParseBool(opts["docker.privileged.enabled"]); err == nil {
		conf["allow_privileged"] = v
	}
	if v, err := strconv.ParseBool(opts["docker.cleanup.container"]); err == nil {
		conf["container_gc"] = v
	}
	return conf, nil
}

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
		"endpoint":    hclspec.NewAttr("endpoint", "string", false),
		"auth_config": hclspec.NewAttr("auth_config", "string", false),
		"auth_helper": hclspec.NewAttr("auth_helper", "string", false),
		"tls": hclspec.NewBlock("tls", false, hclspec.NewObject(map[string]*hclspec.Spec{
			"cert": hclspec.NewAttr("cert", "string", false),
			"key":  hclspec.NewAttr("key", "string", false),
			"ca":   hclspec.NewAttr("ca", "string", false),
		})),
		"image_gc": hclspec.NewDefault(
			hclspec.NewAttr("image_gc", "bool", false),
			hclspec.NewLiteral("true"),
		),
		"image_gc_delay": hclspec.NewAttr("image_gc_delay", "string", false),
		"volumes_enabled": hclspec.NewDefault(
			hclspec.NewAttr("volumes_enabled", "bool", false),
			hclspec.NewLiteral("true"),
		),
		"volumes_selinuxlabel": hclspec.NewAttr("volumes_selinuxlabel", "string", false),
		"allow_privileged":     hclspec.NewAttr("allow_privileged", "bool", false),
		"allow_caps": hclspec.NewDefault(
			hclspec.NewAttr("allow_caps", "list(string)", false),
			hclspec.NewLiteral(`["CHOWN","DAC_OVERRIDE","FSETID","FOWNER","MKNOD","NET_RAW","SETGID","SETUID","SETFCAP","SETPCAP","NET_BIND_SERVICE","SYS_CHROOT","KILL","AUDIT_WRITE"]`),
		),
		"container_gc": hclspec.NewDefault(
			hclspec.NewAttr("container_gc", "bool", false),
			hclspec.NewLiteral("true"),
		),
	})

	// taskConfigSpec is the hcl specification for the driver config section of
	// a task within a job. It is returned in the TaskConfigSchema RPC
	taskConfigSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"image":                  hclspec.NewAttr("image", "string", true),
		"advertise_ipv6_address": hclspec.NewAttr("advertise_ipv6_address", "bool", false),
		"args":                   hclspec.NewAttr("args", "list(string)", false),
		"auth": hclspec.NewBlock("auth", false, hclspec.NewObject(map[string]*hclspec.Spec{
			"username":       hclspec.NewAttr("username", "string", false),
			"password":       hclspec.NewAttr("password", "string", false),
			"email":          hclspec.NewAttr("email", "string", false),
			"server_address": hclspec.NewAttr("server_address", "string", false),
		})),
		"auth_soft_fail": hclspec.NewAttr("auth_soft_fail", "bool", false),
		"cap_add":        hclspec.NewAttr("cap_add", "list(string)", false),
		"cap_drop":       hclspec.NewAttr("cap_drop", "list(string)", false),
		"command":        hclspec.NewAttr("command", "string", false),
		"cpu_hard_limit": hclspec.NewAttr("cpu_hard_limit", "bool", false),
		"cpu_cfs_period": hclspec.NewAttr("cpu_cfs_period", "number", false),
		"devices": hclspec.NewBlockSet("devices", hclspec.NewObject(map[string]*hclspec.Spec{
			"host_path":          hclspec.NewAttr("host_path", "string", false),
			"container_path":     hclspec.NewAttr("container_path", "string", false),
			"cgroup_permissions": hclspec.NewAttr("cgroup_permissions", "string", false),
		})),
		"dns_search_domains": hclspec.NewAttr("dns_search_domains", "list(string)", false),
		"dns_options":        hclspec.NewAttr("dns_options", "list(string)", false),
		"dns_servers":        hclspec.NewAttr("dns_servers", "list(string)", false),
		"entrypoint":         hclspec.NewAttr("entrypoint", "list(string)", false),
		"extra_hosts":        hclspec.NewAttr("extra_hosts", "list(string)", false),
		"force_pull":         hclspec.NewAttr("force_pull", "bool", false),
		"hostname":           hclspec.NewAttr("hostname", "string", false),
		"interactive":        hclspec.NewAttr("interactive", "bool", false),
		"ipc_mode":           hclspec.NewAttr("ipc_mode", "string", false),
		"ipv4_address":       hclspec.NewAttr("ipv4_address", "string", false),
		"ipv6_address":       hclspec.NewAttr("ipv6_address", "string", false),
		"labels":             hclspec.NewAttr("labels", "map(string)", false),
		"load":               hclspec.NewAttr("load", "string", false),
		"logging":            hclspec.NewAttr("logging", "map(string)", false),
		"mac_address":        hclspec.NewAttr("mac_address", "map(string)", false),
		"mounts": hclspec.NewBlockSet("mounts", hclspec.NewObject(map[string]*hclspec.Spec{
			"target":   hclspec.NewAttr("target", "string", false),
			"source":   hclspec.NewAttr("source", "string", false),
			"readonly": hclspec.NewAttr("readonly", "bool", false),
			"volume_options": hclspec.NewBlockSet("volume_options", hclspec.NewObject(map[string]*hclspec.Spec{
				"no_copy": hclspec.NewAttr("no_copy", "bool", false),
				"labels":  hclspec.NewAttr("labels", "map(string)", false),
				"driver_config": hclspec.NewBlockSet("driver_config", hclspec.NewObject(map[string]*hclspec.Spec{
					"name":    hclspec.NewAttr("name", "string", false),
					"options": hclspec.NewAttr("name", "map(string)", false),
				})),
			})),
		})),
		"network_aliases": hclspec.NewAttr("network_aliases", "list(string)", false),
		"network_mode":    hclspec.NewAttr("network_mode", "string", false),
		"pids_limit":      hclspec.NewAttr("pids_limit", "number", false),
		"pid_mode":        hclspec.NewAttr("pid_mode", "string", false),
		"port_map":        hclspec.NewAttr("port_map", "map(number)", false),
		"privileged":      hclspec.NewAttr("privileged", "bool", false),
		"readonly_rootfs": hclspec.NewAttr("readonly_rootfs", "bool", false),
		"security_opt":    hclspec.NewAttr("security_opt", "list(string)", false),
		"shm_size":        hclspec.NewAttr("shm_size", "number", false),
		"sysctl":          hclspec.NewAttr("sysctl", "map(string)", false),
		"tty":             hclspec.NewAttr("tty", "bool", false),
		"ulimit":          hclspec.NewAttr("ulimit", "map(string)", false),
		"uts_mode":        hclspec.NewAttr("uts_mode", "string", false),
		"userns_mode":     hclspec.NewAttr("userns_mode", "string", false),
		"volumes":         hclspec.NewAttr("volumes", "list(string)", false),
		"volume_driver":   hclspec.NewAttr("volume_driver", "string", false),
		"work_dir":        hclspec.NewAttr("work_dir", "string", false),
	})

	// capabilities is returned by the Capabilities RPC and indicates what
	// optional features this driver supports
	capabilities = &drivers.Capabilities{
		SendSignals: true,
		Exec:        true,
		FSIsolation: structs.FSIsolationImage,
	}

	// createClientsLock is a lock that protects reading/writing global client
	// variables
	createClientsLock sync.Mutex

	// client is a docker client with a timeout of 5 minutes. This is for doing
	// all operations with the docker daemon besides which are not long running
	// such as creating, killing containers, etc.
	client *docker.Client

	// waitClient is a docker client with no timeouts. This is used for long
	// running operations such as waiting on containers and collect stats
	waitClient *docker.Client

	// healthCheckClient is a docker client with a timeout of 1 minute. This is
	// necessary to have a shorter timeout than other API or fingerprint calls
	healthCheckClient *docker.Client

	// The statistics the Docker driver exposes
	DockerMeasuredMemStats = []string{"RSS", "Cache", "Swap", "Max Usage"}
	DockerMeasuredCpuStats = []string{"Throttled Periods", "Throttled Time", "Percent"}

	// recoverableErrTimeouts returns a recoverable error if the error was due
	// to timeouts
	recoverableErrTimeouts = func(err error) error {
		r := false
		if strings.Contains(err.Error(), "Client.Timeout exceeded while awaiting headers") ||
			strings.Contains(err.Error(), "EOF") {
			r = true
		}
		return nstructs.NewRecoverableError(err, r)
	}
)

type TaskConfig struct {
	Image             string            `codec:"image"`
	AdvertiseIPv6Addr bool              `codec:"advertise_ipv6_address"`
	Args              []string          `codec:"args"`
	Auth              DockerAuth        `codec:"auth"`
	AuthSoftFail      bool              `codec:"auth_soft_fail"`
	CapAdd            []string          `codec:"cap_add"`
	CapDrop           []string          `codec:"cap_drop"`
	Command           string            `codec:"command"`
	CPUCFSPeriod      int64             `codec:"cpu_cfs_period"`
	CPUHardLimit      bool              `codec:"cpu_hard_limit"`
	Devices           []DockerDevice    `codec:"devices"`
	DNSSearchDomains  []string          `codec:"dns_search_domains"`
	DNSOptions        []string          `codec:"dns_options"`
	DNSServers        []string          `codec:"dns_servers"`
	Entrypoint        []string          `codec:"entrypoint"`
	ExtraHosts        []string          `codec:"extra_hosts"`
	ForcePull         bool              `codec:"force_pull"`
	Hostname          string            `codec:"hostname"`
	Interactive       bool              `codec:"interactive"`
	IPCMode           string            `codec:"ipc_mode"`
	IPv4Address       string            `codec:"ipv4_address"`
	IPv6Address       string            `codec:"ipv6_address"`
	Labels            map[string]string `codec:"labels"`
	LoadImage         string            `codec:"load"`
	Logging           DockerLogging     `codec:"logging"`
	MacAddress        string            `codec:"mac_address"`
	Mounts            []DockerMount     `codec:"mounts"`
	NetworkAliases    []string          `codec:"network_aliases"`
	NetworkMode       string            `codec:"network_mode"`
	PidsLimit         int64             `codec:"pids_limit"`
	PidMode           string            `codec:"pid_mode"`
	PortMap           map[string]int    `codec:"port_map"`
	Privileged        bool              `codec:"privileged"`
	ReadonlyRootfs    bool              `codec:"readonly_rootfs"`
	SecurityOpt       []string          `codec:"security_opt"`
	ShmSize           int64             `codec:"shm_size"`
	Sysctl            map[string]string `codec:"sysctl"`
	TTY               bool              `codec:"tty"`
	Ulimit            map[string]string `codec:"ulimit"`
	UTSMode           string            `codec:"uts_mode"`
	UsernsMode        string            `codec:"userns_mode"`
	Volumes           []string          `codec:"volumes"`
	VolumeDriver      string            `codec:"volume_driver"`
	WorkDir           string            `codec:"work_dir"`
}

type DockerAuth struct {
	Username   string `codec:"username"`
	Password   string `codec:"password"`
	Email      string `codec:"email"`
	ServerAddr string `codec:"server_address"`
}

type DockerDevice struct {
	HostPath          string `codec:"host_path"`
	ContainerPath     string `codec:"container_path"`
	CgroupPermissions string `codec:"cgroup_permissions"`
}

type DockerLogging struct {
	Type   string            `codec:"type"`
	Config map[string]string `codec:"config"`
}

type DockerMount struct {
	Target        string              `codec:"target"`
	Source        string              `codec:"source"`
	ReadOnly      bool                `codec:"readonly"`
	VolumeOptions DockerVolumeOptions `codec:"volume_options"`
}

type DockerVolumeOptions struct {
	NoCopy       bool                     `codec:"no_copy"`
	Labels       map[string]string        `codec:"labels"`
	DriverConfig DockerVolumeDriverConfig `codec:"driver_config"`
}

// VolumeDriverConfig holds a map of volume driver specific options
type DockerVolumeDriverConfig struct {
	Name    string            `codec:"name"`
	Options map[string]string `codec:"options"`
}

type DriverConfig struct {
	Endpoint             string        `codec:"endpoint"`
	AuthConfig           string        `codec:"auth_config"`
	AuthHelper           string        `codec:"auth_helper"`
	TLS                  TLSConfig     `codec:"tls"`
	ImageGC              bool          `codec:"image_gc"`
	ImageGCDelay         string        `codec:"image_gc_delay"`
	imageGCDelayDuration time.Duration `codec:"-"`
	VolumesEnabled       bool          `codec:"volumes_enabled"`
	VolumesSelinuxLabel  string        `codec:"volumes_selinuxlabel"`
	AllowPrivileged      bool          `codec:"allow_privileged"`
	AllowCaps            []string      `codec:"allow_caps"`
	ContainerGC          bool          `codec:"container_gc"`
}

type TLSConfig struct {
	Cert string `codec:"cert"`
	Key  string `codec:"key"`
	CA   string `codec:"ca"`
}

type Driver struct {
	// eventer is used to handle multiplexing of TaskEvents calls such that an
	// event can be broadcast to all callers
	eventer *eventer.Eventer

	config       *DriverConfig
	clientConfig *base.ClientDriverConfig
	// ctx is the context for the driver. It is passed to other subsystems to
	// coordinate shutdown
	ctx context.Context

	// signalShutdown is called when the driver is shutting down and cancels the
	// ctx passed to any subsystems
	signalShutdown context.CancelFunc

	// tasks is the in memory datastore mapping taskIDs to taskHandles
	tasks *taskStore

	// logger will log to the plugin output which is usually an 'executor.out'
	// file located in the root of the TaskDir
	logger hclog.Logger
}

// NewDockerDriver returns a docker implementation of a driver plugin
func NewDockerDriver(logger hclog.Logger) drivers.DriverPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	logger = logger.Named(pluginName)
	return &Driver{
		eventer:        eventer.NewEventer(ctx, logger),
		config:         &DriverConfig{},
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

func (d *Driver) SetConfig(data []byte, cfg *base.ClientAgentConfig) error {
	var config DriverConfig
	if err := base.MsgPackDecode(data, &config); err != nil {
		return err
	}

	d.config = &config
	if len(d.config.ImageGCDelay) > 0 {
		dur, err := time.ParseDuration(d.config.ImageGCDelay)
		if err != nil {
			return fmt.Errorf("failed to parse 'image_gc_delay' duration: %v", err)
		}
		d.config.imageGCDelayDuration = dur
	}

	if cfg != nil {
		d.clientConfig = cfg.Driver
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

func (d *Driver) buildFingerprint() *drivers.Fingerprint {
	fp := &drivers.Fingerprint{
		Attributes:        map[string]string{},
		Health:            drivers.HealthStateHealthy,
		HealthDescription: "healthy",
	}
	client, _, err := d.dockerClients()
	if err != nil {
		d.logger.Info("failed to initialize client", "error", err)
		return &drivers.Fingerprint{
			Health:            drivers.HealthStateUndetected,
			HealthDescription: "ready",
		}
	}

	env, err := client.Version()
	if err != nil {
		d.logger.Debug("could not connect to docker daemon", "endpoint", client.Endpoint(), "error", err)
		return &drivers.Fingerprint{
			Health:            drivers.HealthStateUnhealthy,
			HealthDescription: "failed to connect to docker daemon",
		}
	}

	fp.Attributes["driver.docker"] = "1"
	fp.Attributes["driver.docker.version"] = env.Get("Version")
	if d.config.AllowPrivileged {
		fp.Attributes["driver.docker.privileged.enabled"] = "1"
	}

	if d.config.VolumesEnabled {
		fp.Attributes["driver.docker.volumes.enabled"] = "1"
	}

	if nets, err := client.ListNetworks(); err != nil {
		d.logger.Warn("error discovering bridge IP", "error", err)
	} else {
		for _, n := range nets {
			if n.Name != "bridge" {
				continue
			}

			if len(n.IPAM.Config) == 0 {
				d.logger.Warn("no IPAM config for bridge network")
				break
			}

			if n.IPAM.Config[0].Gateway != "" {
				fp.Attributes["driver.docker.bridge_ip"] = n.IPAM.Config[0].Gateway
			} else {
				// Docker 17.09.0-ce dropped the Gateway IP from the bridge network
				// See https://github.com/moby/moby/issues/32648
				d.logger.Debug("bridge_ip could not be discovered")
			}
			break
		}
	}

	return fp
}

func (d *Driver) RecoverTask(*drivers.TaskHandle) error {
	panic("not implemented")
}

func (d *Driver) StartTask(cfg *drivers.TaskConfig) (*drivers.TaskHandle, *structs.DriverNetwork, error) {
	if _, ok := d.tasks.Get(cfg.ID); ok {
		return nil, nil, fmt.Errorf("taskConfig with ID '%s' already started", cfg.ID)
	}

	var driverConfig TaskConfig

	if err := cfg.DecodeDriverConfig(&driverConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to decode driver config: %v", err)
	}

	handle := drivers.NewTaskHandle(pluginName)
	handle.Config = cfg

	// Initialize docker API clients
	client, _, err := d.dockerClients()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to connect to docker daemon: %s", err)
	}

	id, err := d.createImage(cfg, &driverConfig, client)
	if err != nil {
		return nil, nil, err
	}

	containerCfg, err := d.createContainerConfig(cfg, &driverConfig, id)
	if err != nil {
		d.logger.Error("failed to create container configuration", "image_name", driverConfig.Image,
			"image_id", id, "error", err)
		return nil, nil, fmt.Errorf("Failed to create container configuration for image %q (%q): %v", driverConfig.Image, id, err)
	}

	startAttempts := 0
CREATE:
	container, err := d.createContainer(client, containerCfg, &driverConfig)
	if err != nil {
		d.logger.Error("failed to create container", "error", err)
		return nil, nil, nstructs.NewRecoverableError(fmt.Errorf("failed to create container: %v", err), nstructs.IsRecoverable(err))
	}

	d.logger.Info("created container", "container_id", container.ID)

	// We don't need to start the container if the container is already running
	// since we don't create containers which are already present on the host
	// and are running
	if !container.State.Running {
		// Start the container
		if err := d.startContainer(container); err != nil {
			d.logger.Error("failed to start container", "container_id", container.ID, "error", err)
			client.RemoveContainer(docker.RemoveContainerOptions{
				ID:    container.ID,
				Force: true,
			})
			// Some sort of docker race bug, recreating the container usually works
			if strings.Contains(err.Error(), "OCI runtime create failed: container with id exists:") && startAttempts < 5 {
				startAttempts++
				d.logger.Debug("reattempting container create/start sequence", "attempt", startAttempts, "container_id", id)
				goto CREATE
			}
			return nil, nil, nstructs.NewRecoverableError(fmt.Errorf("Failed to start container %s: %s", container.ID, err), nstructs.IsRecoverable(err))
		}

		// InspectContainer to get all of the container metadata as
		// much of the metadata (eg networking) isn't populated until
		// the container is started
		runningContainer, err := client.InspectContainer(container.ID)
		if err != nil {
			msg := "failed to inspect started container"
			d.logger.Error(msg, "error", err)
			return nil, nil, nstructs.NewRecoverableError(fmt.Errorf("%s %s: %s", msg, container.ID, err), true)
		}
		container = runningContainer
		d.logger.Info("started container", "container_id", container.ID)
	} else {
		d.logger.Debug("re-attaching to container", "container_id",
			container.ID, "container_state", container.State.String())
	}

	dlogger, pluginClient, err := docklog.LaunchDockerLogger(d.logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to launch docker logger plugin: %v", err)
	}

	if err := dlogger.Start(&docklog.StartOpts{
		Endpoint:    d.config.Endpoint,
		ContainerID: container.ID,
		Stdout:      cfg.StdoutPath,
		Stderr:      cfg.StderrPath,
		TLSCert:     d.config.TLS.Cert,
		TLSKey:      d.config.TLS.Key,
		TLSCA:       d.config.TLS.CA,
	}); err != nil {
		pluginClient.Kill()
		return nil, nil, fmt.Errorf("failed to launch docker logger process %s: %v", container.ID, err)
	}

	// Detect container address
	ip, autoUse := d.detectIP(container, &driverConfig)

	net := &structs.DriverNetwork{
		PortMap:       driverConfig.PortMap,
		IP:            ip,
		AutoAdvertise: autoUse,
	}

	// Return a driver handle
	h := &taskHandle{
		client:                client,
		waitClient:            waitClient,
		dlogger:               dlogger,
		dloggerPluginClient:   pluginClient,
		logger:                d.logger.With("container_id", container.ID),
		task:                  cfg,
		container:             container,
		doneCh:                make(chan bool),
		waitCh:                make(chan struct{}),
		removeContainerOnExit: d.config.ContainerGC,
		net:                   net,
	}
	d.tasks.Set(cfg.ID, h)
	go h.collectStats()
	go h.run()

	return handle, net, nil
}

// createContainerClient is the subset of Docker Client methods used by the
// createContainer method to ease testing subtle error conditions.
type createContainerClient interface {
	CreateContainer(docker.CreateContainerOptions) (*docker.Container, error)
	InspectContainer(id string) (*docker.Container, error)
	ListContainers(docker.ListContainersOptions) ([]docker.APIContainers, error)
	RemoveContainer(opts docker.RemoveContainerOptions) error
}

// createContainer creates the container given the passed configuration. It
// attempts to handle any transient Docker errors.
func (d *Driver) createContainer(client createContainerClient, config docker.CreateContainerOptions,
	driverConfig *TaskConfig) (*docker.Container, error) {
	// Create a container
	attempted := 0
CREATE:
	container, createErr := client.CreateContainer(config)
	if createErr == nil {
		return container, nil
	}

	d.logger.Debug("failed to create container", "container_name",
		config.Name, "image_name", driverConfig.Image, "image_id", config.Config.Image,
		"attempt", attempted+1, "error", createErr)

	// Volume management tools like Portworx may not have detached a volume
	// from a previous node before Nomad started a task replacement task.
	// Treat these errors as recoverable so we retry.
	if strings.Contains(strings.ToLower(createErr.Error()), "volume is attached on another node") {
		return nil, nstructs.NewRecoverableError(createErr, true)
	}

	// If the container already exists determine whether it's already
	// running or if it's dead and needs to be recreated.
	if strings.Contains(strings.ToLower(createErr.Error()), "container already exists") {
		containers, err := client.ListContainers(docker.ListContainersOptions{
			All: true,
		})
		if err != nil {
			d.logger.Error("failed to query list of containers matching name", "container_name", config.Name)
			return nil, recoverableErrTimeouts(fmt.Errorf("Failed to query list of containers: %s", err))
		}

		// Delete matching containers
		// Adding a / infront of the container name since Docker returns the
		// container names with a / pre-pended to the Nomad generated container names
		containerName := "/" + config.Name
		d.logger.Debug("searching for container to purge", "container_name", containerName)
		for _, shimContainer := range containers {
			d.logger.Debug("listed container", "names", hclog.Fmt("%+v", shimContainer.Names))
			found := false
			for _, name := range shimContainer.Names {
				if name == containerName {
					d.logger.Debug("Found container", "containter_name", containerName, "container_id", shimContainer.ID)
					found = true
					break
				}
			}

			if !found {
				continue
			}

			// Inspect the container and if the container isn't dead then return
			// the container
			container, err := client.InspectContainer(shimContainer.ID)
			if err != nil {
				err = fmt.Errorf("Failed to inspect container %s: %s", shimContainer.ID, err)

				// This error is always recoverable as it could
				// be caused by races between listing
				// containers and this container being removed.
				// See #2802
				return nil, nstructs.NewRecoverableError(err, true)
			}
			if container != nil && container.State.Running {
				return container, nil
			}

			err = client.RemoveContainer(docker.RemoveContainerOptions{
				ID:    container.ID,
				Force: true,
			})
			if err != nil {
				d.logger.Error("failed to purge container", "container_id", container.ID)
				return nil, recoverableErrTimeouts(fmt.Errorf("Failed to purge container %s: %s", container.ID, err))
			} else if err == nil {
				d.logger.Info("purged container", "container_id", container.ID)
			}
		}

		if attempted < 5 {
			attempted++
			time.Sleep(1 * time.Second)
			goto CREATE
		}
	} else if strings.Contains(strings.ToLower(createErr.Error()), "no such image") {
		// There is still a very small chance this is possible even with the
		// coordinator so retry.
		return nil, nstructs.NewRecoverableError(createErr, true)
	}

	return nil, recoverableErrTimeouts(createErr)
}

// startContainer starts the passed container. It attempts to handle any
// transient Docker errors.
func (d *Driver) startContainer(c *docker.Container) error {
	// Start a container
	attempted := 0
START:
	startErr := client.StartContainer(c.ID, c.HostConfig)
	if startErr == nil {
		return nil
	}

	d.logger.Debug("failed to start container", "container_id", c.ID, "attempt", attempted+1, "error", startErr)

	// If it is a 500 error it is likely we can retry and be successful
	if strings.Contains(startErr.Error(), "API error (500)") {
		if attempted < 5 {
			attempted++
			time.Sleep(1 * time.Second)
			goto START
		}
		return nstructs.NewRecoverableError(startErr, true)
	}

	return recoverableErrTimeouts(startErr)
}

// getDockerCoordinator returns the docker coordinator and the caller ID to use when
// interacting with the coordinator
func (d *Driver) getDockerCoordinator(client *docker.Client, task *drivers.TaskConfig) (*dockerCoordinator, string) {
	config := &dockerCoordinatorConfig{
		client:      client,
		cleanup:     d.config.ImageGC,
		logger:      d.logger,
		removeDelay: d.config.imageGCDelayDuration,
	}

	return GetDockerCoordinator(config), fmt.Sprintf("%s-%s", task.ID, task.Name)
}

// createImage creates a docker image either by pulling it from a registry or by
// loading it from the file system
func (d *Driver) createImage(task *drivers.TaskConfig, driverConfig *TaskConfig, client *docker.Client) (string, error) {
	image := driverConfig.Image
	repo, tag := parseDockerImage(image)

	coordinator, callerID := d.getDockerCoordinator(client, task)

	// We're going to check whether the image is already downloaded. If the tag
	// is "latest", or ForcePull is set, we have to check for a new version every time so we don't
	// bother to check and cache the id here. We'll download first, then cache.
	if driverConfig.ForcePull {
		d.logger.Debug("force pulling image instead of inspecting local", "image_ref", dockerImageRef(repo, tag))
	} else if tag != "latest" {
		if dockerImage, _ := client.InspectImage(image); dockerImage != nil {
			// Image exists so just increment its reference count
			coordinator.IncrementImageReference(dockerImage.ID, image, callerID)
			return dockerImage.ID, nil
		}
	}

	// Load the image if specified
	if driverConfig.LoadImage != "" {
		return d.loadImage(task, driverConfig, client)
	}

	// Download the image
	return d.pullImage(task, driverConfig, client, repo, tag)
}

// pullImage creates an image by pulling it from a docker registry
func (d *Driver) pullImage(task *drivers.TaskConfig, driverConfig *TaskConfig, client *docker.Client, repo, tag string) (id string, err error) {
	authOptions, err := d.resolveRegistryAuthentication(driverConfig, repo)
	if err != nil {
		if driverConfig.AuthSoftFail {
			d.logger.Warn("Failed to find docker repo auth", "repo", repo, "error", err)
		} else {
			return "", fmt.Errorf("Failed to find docker auth for repo %q: %v", repo, err)
		}
	}

	if authIsEmpty(authOptions) {
		d.logger.Debug("did not find docker auth for repo", "repo", repo)
	}

	d.eventer.EmitEvent(&drivers.TaskEvent{
		TaskID:    task.ID,
		Timestamp: time.Now(),
		Message:   "Downloading image",
		Annotations: map[string]string{
			"image": dockerImageRef(repo, tag),
		},
	})
	coordinator, callerID := d.getDockerCoordinator(client, task)

	return coordinator.PullImage(driverConfig.Image, authOptions, callerID, d.emitEventFunc(task))
}

func (d *Driver) emitEventFunc(task *drivers.TaskConfig) LogEventFn {
	return func(msg string, annotations map[string]string) {
		d.eventer.EmitEvent(&drivers.TaskEvent{
			TaskID:      task.ID,
			Timestamp:   time.Now(),
			Message:     msg,
			Annotations: annotations,
		})
	}
}

// authBackend encapsulates a function that resolves registry credentials.
type authBackend func(string) (*docker.AuthConfiguration, error)

// resolveRegistryAuthentication attempts to retrieve auth credentials for the
// repo, trying all authentication-backends possible.
func (d *Driver) resolveRegistryAuthentication(driverConfig *TaskConfig, repo string) (*docker.AuthConfiguration, error) {
	return firstValidAuth(repo, []authBackend{
		authFromTaskConfig(driverConfig),
		authFromDockerConfig(d.config.AuthConfig),
		authFromHelper(d.config.AuthHelper),
	})
}

// loadImage creates an image by loading it from the file system
func (d *Driver) loadImage(task *drivers.TaskConfig, driverConfig *TaskConfig, client *docker.Client) (id string, err error) {

	archive := filepath.Join(task.TaskDir().LocalDir, driverConfig.LoadImage)
	d.logger.Debug("loading image from disk", "archive", archive)

	f, err := os.Open(archive)
	if err != nil {
		return "", fmt.Errorf("unable to open image archive: %v", err)
	}

	if err := client.LoadImage(docker.LoadImageOptions{InputStream: f}); err != nil {
		return "", err
	}
	f.Close()

	dockerImage, err := client.InspectImage(driverConfig.Image)
	if err != nil {
		return "", recoverableErrTimeouts(err)
	}

	coordinator, callerID := d.getDockerCoordinator(client, task)
	coordinator.IncrementImageReference(dockerImage.ID, driverConfig.Image, callerID)
	return dockerImage.ID, nil
}

func (d *Driver) containerBinds(task *drivers.TaskConfig, driverConfig *TaskConfig) ([]string, error) {

	allocDirBind := fmt.Sprintf("%s:%s", task.TaskDir().SharedAllocDir, task.Env[env.AllocDir])
	taskLocalBind := fmt.Sprintf("%s:%s", task.TaskDir().LocalDir, task.Env[env.TaskLocalDir])
	secretDirBind := fmt.Sprintf("%s:%s", task.TaskDir().SecretsDir, task.Env[env.SecretsDir])
	binds := []string{allocDirBind, taskLocalBind, secretDirBind}

	if !d.config.VolumesEnabled && driverConfig.VolumeDriver != "" {
		return nil, fmt.Errorf("'volumes_enabled' is false; cannot use volume driver %q", driverConfig.VolumeDriver)
	}

	for _, userbind := range driverConfig.Volumes {
		parts := strings.Split(userbind, ":")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid docker volume: %q", userbind)
		}

		// Resolve dotted path segments
		parts[0] = filepath.Clean(parts[0])

		// Absolute paths aren't always supported
		if filepath.IsAbs(parts[0]) {
			if !d.config.VolumesEnabled {
				// Disallow mounting arbitrary absolute paths
				return nil, fmt.Errorf("'volumes_enabled' is false; cannot mount host paths: %+q", userbind)
			}
			binds = append(binds, userbind)
			continue
		}

		// Relative paths are always allowed as they mount within a container
		// When a VolumeDriver is set, we assume we receive a binding in the format volume-name:container-dest
		// Otherwise, we assume we receive a relative path binding in the format relative/to/task:/also/in/container
		if driverConfig.VolumeDriver == "" {
			// Expand path relative to alloc dir
			parts[0] = filepath.Join(task.TaskDir().Dir, parts[0])
		}

		binds = append(binds, strings.Join(parts, ":"))
	}

	if selinuxLabel := d.config.VolumesSelinuxLabel; selinuxLabel != "" {
		// Apply SELinux Label to each volume
		for i := range binds {
			binds[i] = fmt.Sprintf("%s:%s", binds[i], selinuxLabel)
		}
	}

	return binds, nil
}

func (d *Driver) createContainerConfig(task *drivers.TaskConfig, driverConfig *TaskConfig,
	imageID string) (docker.CreateContainerOptions, error) {

	logger := d.logger.With("task_name", task.Name)
	var c docker.CreateContainerOptions
	if task.Resources == nil {
		// Guard against missing resources. We should never have been able to
		// schedule a job without specifying this.
		logger.Error("task.Resources is empty")
		return c, fmt.Errorf("task.Resources is empty")
	}

	binds, err := d.containerBinds(task, driverConfig)
	if err != nil {
		return c, err
	}
	logger.Trace("binding volumes", "volumes", binds)

	// create the config block that will later be consumed by go-dockerclient
	config := &docker.Config{
		Image:      imageID,
		Entrypoint: driverConfig.Entrypoint,
		Hostname:   driverConfig.Hostname,
		User:       task.User,
		Tty:        driverConfig.TTY,
		OpenStdin:  driverConfig.Interactive,
	}

	if driverConfig.WorkDir != "" {
		config.WorkingDir = driverConfig.WorkDir
	}

	hostConfig := &docker.HostConfig{
		// Convert MB to bytes. This is an absolute value.
		Memory: task.Resources.LinuxResources.MemoryLimitBytes,
		// Convert Mhz to shares. This is a relative value.
		CPUShares: task.Resources.LinuxResources.CPUShares,

		// Binds are used to mount a host volume into the container. We mount a
		// local directory for storage and a shared alloc directory that can be
		// used to share data between different tasks in the same task group.
		Binds: binds,

		VolumeDriver: driverConfig.VolumeDriver,

		PidsLimit: driverConfig.PidsLimit,
	}

	// Calculate CPU Quota
	// cfs_quota_us is the time per core, so we must
	// multiply the time by the number of cores available
	// See https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/6/html/resource_management_guide/sec-cpu
	if driverConfig.CPUHardLimit {
		numCores := runtime.NumCPU()
		percentTicks := float64(task.Resources.NomadResources.CPU) / float64(task.Resources.NomadResources.CPU)
		if driverConfig.CPUCFSPeriod < 0 || driverConfig.CPUCFSPeriod > 1000000 {
			return c, fmt.Errorf("invalid value for cpu_cfs_period")
		}
		if driverConfig.CPUCFSPeriod == 0 {
			driverConfig.CPUCFSPeriod = task.Resources.LinuxResources.CPUPeriod
		}
		hostConfig.CPUPeriod = driverConfig.CPUCFSPeriod
		hostConfig.CPUQuota = int64(percentTicks*float64(driverConfig.CPUCFSPeriod)) * int64(numCores)
	}

	// Windows does not support MemorySwap/MemorySwappiness #2193
	if runtime.GOOS == "windows" {
		hostConfig.MemorySwap = 0
		hostConfig.MemorySwappiness = -1
	} else {
		hostConfig.MemorySwap = task.Resources.LinuxResources.MemoryLimitBytes // MemorySwap is memory + swap.
	}

	hostConfig.LogConfig = docker.LogConfig{
		Type:   driverConfig.Logging.Type,
		Config: driverConfig.Logging.Config,
	}

	logger.Debug("configured resources", "memory", hostConfig.Memory,
		"cpu_shares", hostConfig.CPUShares, "cpu_quota", hostConfig.CPUQuota,
		"cpu_period", hostConfig.CPUPeriod)
	logger.Debug("binding directories", "binds", hclog.Fmt("%#v", hostConfig.Binds))

	//  set privileged mode
	if driverConfig.Privileged && !d.config.AllowPrivileged {
		return c, fmt.Errorf(`Docker privileged mode is disabled on this Nomad agent`)
	}
	hostConfig.Privileged = driverConfig.Privileged

	// set capabilities
	hostCapsWhitelistConfig := d.config.AllowCaps
	hostCapsWhitelist := make(map[string]struct{})
	for _, cap := range hostCapsWhitelistConfig {
		cap = strings.ToLower(strings.TrimSpace(cap))
		hostCapsWhitelist[cap] = struct{}{}
	}

	if _, ok := hostCapsWhitelist["all"]; !ok {
		effectiveCaps, err := tweakCapabilities(
			strings.Split(dockerBasicCaps, ","),
			driverConfig.CapAdd,
			driverConfig.CapDrop,
		)
		if err != nil {
			return c, err
		}
		var missingCaps []string
		for _, cap := range effectiveCaps {
			cap = strings.ToLower(cap)
			if _, ok := hostCapsWhitelist[cap]; !ok {
				missingCaps = append(missingCaps, cap)
			}
		}
		if len(missingCaps) > 0 {
			return c, fmt.Errorf("Docker driver doesn't have the following caps whitelisted on this Nomad agent: %s", missingCaps)
		}
	}

	hostConfig.CapAdd = driverConfig.CapAdd
	hostConfig.CapDrop = driverConfig.CapDrop

	// set SHM size
	if driverConfig.ShmSize != 0 {
		hostConfig.ShmSize = driverConfig.ShmSize
	}

	// set DNS servers
	for _, ip := range driverConfig.DNSServers {
		if net.ParseIP(ip) != nil {
			hostConfig.DNS = append(hostConfig.DNS, ip)
		} else {
			logger.Error("invalid ip address for container dns server", "ip", ip)
		}
	}

	if len(driverConfig.Devices) > 0 {
		var devices []docker.Device
		for _, device := range driverConfig.Devices {
			if device.HostPath == "" {
				return c, fmt.Errorf("host path must be set in configuration for devices")
			}
			if device.CgroupPermissions != "" {
				for _, char := range device.CgroupPermissions {
					ch := string(char)
					if ch != "r" && ch != "w" && ch != "m" {
						return c, fmt.Errorf("invalid cgroup permission string: %q", device.CgroupPermissions)
					}
				}
			} else {
				device.CgroupPermissions = "rwm"
			}
			dev := docker.Device{
				PathOnHost:        device.HostPath,
				PathInContainer:   device.ContainerPath,
				CgroupPermissions: device.CgroupPermissions}
			devices = append(devices, dev)
		}
		hostConfig.Devices = devices
	}

	// Setup mounts
	for _, m := range driverConfig.Mounts {
		hm := docker.HostMount{
			Target:   m.Target,
			Source:   m.Source,
			Type:     "volume", // Only type supported
			ReadOnly: m.ReadOnly,
		}
		vo := m.VolumeOptions
		hm.VolumeOptions = &docker.VolumeOptions{
			NoCopy: vo.NoCopy,
		}

		dc := vo.DriverConfig
		hm.VolumeOptions.DriverConfig = docker.VolumeDriverConfig{
			Name: dc.Name,
		}
		hm.VolumeOptions.DriverConfig.Options = dc.Options
		hm.VolumeOptions.Labels = vo.Labels
		hostConfig.Mounts = append(hostConfig.Mounts, hm)
	}

	// set DNS search domains and extra hosts
	hostConfig.DNSSearch = driverConfig.DNSSearchDomains
	hostConfig.DNSOptions = driverConfig.DNSOptions
	hostConfig.ExtraHosts = driverConfig.ExtraHosts

	hostConfig.IpcMode = driverConfig.IPCMode
	hostConfig.PidMode = driverConfig.PidMode
	hostConfig.UTSMode = driverConfig.UTSMode
	hostConfig.UsernsMode = driverConfig.UsernsMode
	hostConfig.SecurityOpt = driverConfig.SecurityOpt
	hostConfig.Sysctls = driverConfig.Sysctl

	ulimits, err := sliceMergeUlimit(driverConfig.Ulimit)
	if err != nil {
		return c, fmt.Errorf("failed to parse ulimit configuration: %v", err)
	}
	hostConfig.Ulimits = ulimits

	hostConfig.ReadonlyRootfs = driverConfig.ReadonlyRootfs

	hostConfig.NetworkMode = driverConfig.NetworkMode
	if hostConfig.NetworkMode == "" {
		// docker default
		logger.Debug("networking mode not specified; using default", "network_mode", defaultNetworkMode)
		hostConfig.NetworkMode = defaultNetworkMode
	}

	// Setup port mapping and exposed ports
	if len(task.Resources.NomadResources.Networks) == 0 {
		logger.Debug("no network interfaces are available")
		if len(driverConfig.PortMap) > 0 {
			return c, fmt.Errorf("Trying to map ports but no network interface is available")
		}
	} else {
		// TODO add support for more than one network
		network := task.Resources.NomadResources.Networks[0]
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
			logger.Debug("allocated static port", "ip", network.IP, "port", port.Value)

			exposedPorts[containerPort+"/tcp"] = struct{}{}
			exposedPorts[containerPort+"/udp"] = struct{}{}
			logger.Debug("exposed port", "port", port.Value)
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
			logger.Debug("allocated mapped port", "ip", network.IP, "port", port.Value)

			exposedPorts[containerPort+"/tcp"] = struct{}{}
			exposedPorts[containerPort+"/udp"] = struct{}{}
			logger.Debug("exposed port", "port", containerPort)
		}

		hostConfig.PortBindings = publishedPorts
		config.ExposedPorts = exposedPorts
	}

	// If the user specified a custom command to run, we'll inject it here.
	if driverConfig.Command != "" {
		// Validate command
		if err := validateCommand(driverConfig.Command, "args"); err != nil {
			return c, err
		}

		cmd := []string{driverConfig.Command}
		if len(driverConfig.Args) != 0 {
			cmd = append(cmd, driverConfig.Args...)
		}
		logger.Debug("setting container startup command", "command", strings.Join(cmd, " "))
		config.Cmd = cmd
	} else if len(driverConfig.Args) != 0 {
		config.Cmd = driverConfig.Args
	}

	if len(driverConfig.Labels) > 0 {
		config.Labels = driverConfig.Labels
		logger.Debug("applied labels on the container", "labels", config.Labels)
	}

	config.Env = task.EnvList()

	containerName := strings.Replace(task.ID, "/", "_", -1)
	logger.Debug("setting container name", "container_name", containerName)

	var networkingConfig *docker.NetworkingConfig
	if len(driverConfig.NetworkAliases) > 0 || driverConfig.IPv4Address != "" || driverConfig.IPv6Address != "" {
		networkingConfig = &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{
				hostConfig.NetworkMode: {},
			},
		}
	}

	if len(driverConfig.NetworkAliases) > 0 {
		networkingConfig.EndpointsConfig[hostConfig.NetworkMode].Aliases = driverConfig.NetworkAliases
		logger.Debug("setting container network aliases", "network_mode", hostConfig.NetworkMode,
			"network_aliases", strings.Join(driverConfig.NetworkAliases, ", "))
	}

	if driverConfig.IPv4Address != "" || driverConfig.IPv6Address != "" {
		networkingConfig.EndpointsConfig[hostConfig.NetworkMode].IPAMConfig = &docker.EndpointIPAMConfig{
			IPv4Address: driverConfig.IPv4Address,
			IPv6Address: driverConfig.IPv6Address,
		}
		logger.Debug("setting container network configuration", "network_mode", hostConfig.NetworkMode,
			"ipv4_address", driverConfig.IPv4Address, "ipv6_address", driverConfig.IPv6Address)
	}

	if driverConfig.MacAddress != "" {
		config.MacAddress = driverConfig.MacAddress
		logger.Debug("setting container mac address", "mac_address", config.MacAddress)
	}

	return docker.CreateContainerOptions{
		Name:             containerName,
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
	}, nil
}

// detectIP of Docker container. Returns the first IP found as well as true if
// the IP should be advertised (bridge network IPs return false). Returns an
// empty string and false if no IP could be found.
func (d *Driver) detectIP(c *docker.Container, driverConfig *TaskConfig) (string, bool) {
	if c.NetworkSettings == nil {
		// This should only happen if there's been a coding error (such
		// as not calling InspectContainer after CreateContainer). Code
		// defensively in case the Docker API changes subtly.
		d.logger.Error("no network settings for container", "container_id", c.ID)
		return "", false
	}

	ip, ipName := "", ""
	auto := false
	for name, net := range c.NetworkSettings.Networks {
		if net.IPAddress == "" {
			// Ignore networks without an IP address
			continue
		}

		ip = net.IPAddress
		if driverConfig.AdvertiseIPv6Addr {
			ip = net.GlobalIPv6Address
			auto = true
		}
		ipName = name

		// Don't auto-advertise IPs for default networks (bridge on
		// Linux, nat on Windows)
		if name != "bridge" && name != "nat" {
			auto = true
		}

		break
	}

	if n := len(c.NetworkSettings.Networks); n > 1 {
		d.logger.Warn("multiple Docker networks for container found but Nomad only supports 1",
			"total_networks", n,
			"container_id", c.ID,
			"container_network", ipName)
	}

	return ip, auto
}

// validateCommand validates that the command only has a single value and
// returns a user friendly error message telling them to use the passed
// argField.
func validateCommand(command, argField string) error {
	trimmed := strings.TrimSpace(command)
	if len(trimmed) == 0 {
		return fmt.Errorf("command empty: %q", command)
	}

	if len(trimmed) != len(command) {
		return fmt.Errorf("command contains extra white space: %q", command)
	}

	return nil
}

func (d *Driver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}
	ch := make(chan *drivers.ExitResult)
	go d.handleWait(ctx, ch, h)
	return ch, nil
}

func (d *Driver) handleWait(ctx context.Context, ch chan *drivers.ExitResult, h *taskHandle) {
	defer close(ch)
	select {
	case <-h.waitCh:
		ch <- h.exitResult
	case <-ctx.Done():
		ch <- &drivers.ExitResult{
			Err: ctx.Err(),
		}
	}
}

func (d *Driver) StopTask(taskID string, timeout time.Duration, signal string) error {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	if signal == "" {
		signal = "SIGINT"
	}

	sig, err := signals.Parse(signal)
	if err != nil {
		return fmt.Errorf("failed to parse signal: %v", err)
	}

	return h.Kill(timeout, sig)
}

func (d *Driver) DestroyTask(taskID string, force bool) error {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	defer h.dloggerPluginClient.Kill()

	c, err := h.client.InspectContainer(h.container.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect container state: %v", err)
	}
	if c.State.Running && !force {
		return fmt.Errorf("must call StopTask for the given task before Destroy or set force to true")
	}

	if err := h.client.StopContainer(h.container.ID, 0); err != nil {
		h.logger.Warn("failed to stop container during destroy", "error", err)
	}

	if err := h.dlogger.Stop(); err != nil {
		h.logger.Error("failed to stop docker logger process during destroy",
			"error", err, "logger_pid", h.dloggerPluginClient.ReattachConfig().Pid)
	}

	if err := d.cleanupImage(h); err != nil {
		h.logger.Error("failed to cleanup image after destroying container",
			"error", err)
	}

	return nil
}

// cleanupImage removes a Docker image. No error is returned if the image
// doesn't exist or is still in use. Requires the global client to already be
// initialized.
func (d *Driver) cleanupImage(handle *taskHandle) error {
	if !d.config.ImageGC {
		return nil
	}

	coordinator, callerID := d.getDockerCoordinator(client, handle.task)
	coordinator.RemoveImage(handle.container.Image, callerID)

	return nil
}

func (d *Driver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	status := &drivers.TaskStatus{
		ID:          h.task.ID,
		Name:        h.task.Name,
		StartedAt:   h.container.State.StartedAt,
		CompletedAt: h.container.State.FinishedAt,
		DriverAttributes: map[string]string{
			"container_id": h.container.ID,
		},
		NetworkOverride: h.net,
		ExitResult:      h.exitResult,
	}

	status.State = drivers.TaskStateUnknown
	if h.container.State.Running {
		status.State = drivers.TaskStateRunning
	}
	if h.container.State.Dead {
		status.State = drivers.TaskStateExited
	}

	return status, nil
}

func (d *Driver) TaskStats(taskID string) (*structs.TaskResourceUsage, error) {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	return h.Stats()
}

func (d *Driver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	return d.eventer.TaskEvents(ctx)
}

func (d *Driver) SignalTask(taskID string, signal string) error {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return drivers.ErrTaskNotFound
	}

	sig, err := signals.Parse(signal)
	if err != nil {
		return fmt.Errorf("failed to parse signal: %v", err)
	}

	return h.Signal(sig)
}

func (d *Driver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	h, ok := d.tasks.Get(taskID)
	if !ok {
		return nil, drivers.ErrTaskNotFound
	}

	if len(cmd) == 0 {
		return nil, fmt.Errorf("cmd is required, but was empty")
	}

	ctx, _ := context.WithTimeout(context.Background(), timeout)

	return h.Exec(ctx, cmd[0], cmd[1:])
}

// dockerClients creates two *docker.Client, one for long running operations and
// the other for shorter operations. In test / dev mode we can use ENV vars to
// connect to the docker daemon. In production mode we will read docker.endpoint
// from the config file.
func (d *Driver) dockerClients() (*docker.Client, *docker.Client, error) {
	createClientsLock.Lock()
	defer createClientsLock.Unlock()

	if client != nil && waitClient != nil {
		return client, waitClient, nil
	}

	var err error

	// Onlt initialize the client if it hasn't yet been done
	if client == nil {
		client, err = d.newDockerClient(dockerTimeout)
		if err != nil {
			return nil, nil, err
		}
	}

	// Only initialize the waitClient if it hasn't yet been done
	if waitClient == nil {
		waitClient, err = d.newDockerClient(0 * time.Minute)
		if err != nil {
			return nil, nil, err
		}
	}

	return client, waitClient, nil
}

// newDockerClient creates a new *docker.Client with a configurable timeout
func (d *Driver) newDockerClient(timeout time.Duration) (*docker.Client, error) {
	var err error
	var merr multierror.Error
	var newClient *docker.Client

	// Default to using whatever is configured in docker.endpoint. If this is
	// not specified we'll fall back on NewClientFromEnv which reads config from
	// the DOCKER_* environment variables DOCKER_HOST, DOCKER_TLS_VERIFY, and
	// DOCKER_CERT_PATH. This allows us to lock down the config in production
	// but also accept the standard ENV configs for dev and test.
	dockerEndpoint := d.config.Endpoint
	if dockerEndpoint != "" {
		cert := d.config.TLS.Cert
		key := d.config.TLS.Key
		ca := d.config.TLS.CA

		if cert+key+ca != "" {
			d.logger.Debug("using TLS client connection", "endpoint", dockerEndpoint)
			newClient, err = docker.NewTLSClient(dockerEndpoint, cert, key, ca)
			if err != nil {
				merr.Errors = append(merr.Errors, err)
			}
		} else {
			d.logger.Debug("using standard client connection", "endpoint", dockerEndpoint)
			newClient, err = docker.NewClient(dockerEndpoint)
			if err != nil {
				merr.Errors = append(merr.Errors, err)
			}
		}
	} else {
		d.logger.Debug("using client connection initialized from environment")
		newClient, err = docker.NewClientFromEnv()
		if err != nil {
			merr.Errors = append(merr.Errors, err)
		}
	}

	if timeout != 0 && newClient != nil {
		newClient.SetTimeout(timeout)
	}
	return newClient, merr.ErrorOrNil()
}

func sliceMergeUlimit(ulimitsRaw map[string]string) ([]docker.ULimit, error) {
	var ulimits []docker.ULimit

	for name, ulimitRaw := range ulimitsRaw {
		if len(ulimitRaw) == 0 {
			return []docker.ULimit{}, fmt.Errorf("Malformed ulimit specification %v: %q, cannot be empty", name, ulimitRaw)
		}
		// hard limit is optional
		if strings.Contains(ulimitRaw, ":") == false {
			ulimitRaw = ulimitRaw + ":" + ulimitRaw
		}

		splitted := strings.SplitN(ulimitRaw, ":", 2)
		if len(splitted) < 2 {
			return []docker.ULimit{}, fmt.Errorf("Malformed ulimit specification %v: %v", name, ulimitRaw)
		}
		soft, err := strconv.Atoi(splitted[0])
		if err != nil {
			return []docker.ULimit{}, fmt.Errorf("Malformed soft ulimit %v: %v", name, ulimitRaw)
		}
		hard, err := strconv.Atoi(splitted[1])
		if err != nil {
			return []docker.ULimit{}, fmt.Errorf("Malformed hard ulimit %v: %v", name, ulimitRaw)
		}

		ulimit := docker.ULimit{
			Name: name,
			Soft: int64(soft),
			Hard: int64(hard),
		}
		ulimits = append(ulimits, ulimit)
	}
	return ulimits, nil
}
