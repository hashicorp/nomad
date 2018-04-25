package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/armon/circbuf"
	docker "github.com/fsouza/go-dockerclient"

	"github.com/docker/docker/cli/config/configfile"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/driver/executor"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/fields"
	shelpers "github.com/hashicorp/nomad/helper/stats"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

var (
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

	// dockerCleanupImageConfigOption is the key for whether or not to
	// cleanup images after the task exits.
	dockerCleanupImageConfigOption  = "docker.cleanup.image"
	dockerCleanupImageConfigDefault = true

	// dockerPullTimeoutConfigOption is the key for setting an images pull
	// timeout
	dockerImageRemoveDelayConfigOption  = "docker.cleanup.image.delay"
	dockerImageRemoveDelayConfigDefault = 3 * time.Minute

	// dockerCapsWhitelistConfigOption is the key for setting the list of
	// allowed Linux capabilities
	dockerCapsWhitelistConfigOption  = "docker.caps.whitelist"
	dockerCapsWhitelistConfigDefault = dockerBasicCaps

	// dockerTimeout is the length of time a request can be outstanding before
	// it is timed out.
	dockerTimeout = 5 * time.Minute

	// dockerHealthCheckTimeout is the length of time a request for a health
	// check client can be outstanding before it is timed out.
	dockerHealthCheckTimeout = 1 * time.Minute

	// dockerImageResKey is the CreatedResources key for docker images
	dockerImageResKey = "image"

	// dockerAuthHelperPrefix is the prefix to attach to the credential helper
	// and should be found in the $PATH. Example: ${prefix-}${helper-name}
	dockerAuthHelperPrefix = "docker-credential-"

	// dockerBasicCaps is comma-separated list of Linux capabilities that are
	// allowed by docker by default, as documented in
	// https://docs.docker.com/engine/reference/run/#block-io-bandwidth-blkio-constraint
	dockerBasicCaps = "CHOWN,DAC_OVERRIDE,FSETID,FOWNER,MKNOD,NET_RAW,SETGID," +
		"SETUID,SETFCAP,SETPCAP,NET_BIND_SERVICE,SYS_CHROOT,KILL,AUDIT_WRITE"

	// This is cpu.cfs_period_us: the length of a period.
	// The default values is 100 milliseconds (ms) represented in microseconds (us).
	// Below is the documentation:
	// https://www.kernel.org/doc/Documentation/scheduler/sched-bwc.txt
	// https://docs.docker.com/engine/api/v1.35/#
	defaultCFSPeriodUS = 100000
)

type DockerDriver struct {
	DriverContext

	driverConfig *DockerDriverConfig
	imageID      string

	// A tri-state boolean to know if the fingerprinting has happened and
	// whether it has been successful
	fingerprintSuccess *bool
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

type DockerMount struct {
	Target        string                 `mapstructure:"target"`
	Source        string                 `mapstructure:"source"`
	ReadOnly      bool                   `mapstructure:"readonly"`
	VolumeOptions []*DockerVolumeOptions `mapstructure:"volume_options"`
}

type DockerDevice struct {
	HostPath          string `mapstructure:"host_path"`
	ContainerPath     string `mapstructure:"container_path"`
	CgroupPermissions string `mapstructure:"cgroup_permissions"`
}

type DockerVolumeOptions struct {
	NoCopy       bool                       `mapstructure:"no_copy"`
	Labels       []map[string]string        `mapstructure:"labels"`
	DriverConfig []DockerVolumeDriverConfig `mapstructure:"driver_config"`
}

// VolumeDriverConfig holds a map of volume driver specific options
type DockerVolumeDriverConfig struct {
	Name    string              `mapstructure:"name"`
	Options []map[string]string `mapstructure:"options"`
}

// DockerDriverConfig defines the user specified config block in a jobspec
type DockerDriverConfig struct {
	ImageName            string              `mapstructure:"image"`                  // Container's Image Name
	LoadImage            string              `mapstructure:"load"`                   // LoadImage is a path to an image archive file
	Command              string              `mapstructure:"command"`                // The Command to run when the container starts up
	Args                 []string            `mapstructure:"args"`                   // The arguments to the Command
	Entrypoint           []string            `mapstructure:"entrypoint"`             // Override the containers entrypoint
	IpcMode              string              `mapstructure:"ipc_mode"`               // The IPC mode of the container - host and none
	NetworkMode          string              `mapstructure:"network_mode"`           // The network mode of the container - host, nat and none
	NetworkAliases       []string            `mapstructure:"network_aliases"`        // The network-scoped alias for the container
	IPv4Address          string              `mapstructure:"ipv4_address"`           // The container ipv4 address
	IPv6Address          string              `mapstructure:"ipv6_address"`           // the container ipv6 address
	PidMode              string              `mapstructure:"pid_mode"`               // The PID mode of the container - host and none
	UTSMode              string              `mapstructure:"uts_mode"`               // The UTS mode of the container - host and none
	UsernsMode           string              `mapstructure:"userns_mode"`            // The User namespace mode of the container - host and none
	PortMapRaw           []map[string]string `mapstructure:"port_map"`               //
	PortMap              map[string]int      `mapstructure:"-"`                      // A map of host port labels and the ports exposed on the container
	Privileged           bool                `mapstructure:"privileged"`             // Flag to run the container in privileged mode
	SysctlRaw            []map[string]string `mapstructure:"sysctl"`                 //
	Sysctl               map[string]string   `mapstructure:"-"`                      // The sysctl custom configurations
	UlimitRaw            []map[string]string `mapstructure:"ulimit"`                 //
	Ulimit               []docker.ULimit     `mapstructure:"-"`                      // The ulimit custom configurations
	DNSServers           []string            `mapstructure:"dns_servers"`            // DNS Server for containers
	DNSSearchDomains     []string            `mapstructure:"dns_search_domains"`     // DNS Search domains for containers
	DNSOptions           []string            `mapstructure:"dns_options"`            // DNS Options
	ExtraHosts           []string            `mapstructure:"extra_hosts"`            // Add host to /etc/hosts (host:IP)
	Hostname             string              `mapstructure:"hostname"`               // Hostname for containers
	LabelsRaw            []map[string]string `mapstructure:"labels"`                 //
	Labels               map[string]string   `mapstructure:"-"`                      // Labels to set when the container starts up
	Auth                 []DockerDriverAuth  `mapstructure:"auth"`                   // Authentication credentials for a private Docker registry
	AuthSoftFail         bool                `mapstructure:"auth_soft_fail"`         // Soft-fail if auth creds are provided but fail
	TTY                  bool                `mapstructure:"tty"`                    // Allocate a Pseudo-TTY
	Interactive          bool                `mapstructure:"interactive"`            // Keep STDIN open even if not attached
	ShmSize              int64               `mapstructure:"shm_size"`               // Size of /dev/shm of the container in bytes
	WorkDir              string              `mapstructure:"work_dir"`               // Working directory inside the container
	Logging              []DockerLoggingOpts `mapstructure:"logging"`                // Logging options for syslog server
	Volumes              []string            `mapstructure:"volumes"`                // Host-Volumes to mount in, syntax: /path/to/host/directory:/destination/path/in/container
	Mounts               []DockerMount       `mapstructure:"mounts"`                 // Docker volumes to mount
	VolumeDriver         string              `mapstructure:"volume_driver"`          // Docker volume driver used for the container's volumes
	ForcePull            bool                `mapstructure:"force_pull"`             // Always force pull before running image, useful if your tags are mutable
	MacAddress           string              `mapstructure:"mac_address"`            // Pin mac address to container
	SecurityOpt          []string            `mapstructure:"security_opt"`           // Flags to pass directly to security-opt
	Devices              []DockerDevice      `mapstructure:"devices"`                // To allow mounting USB or other serial control devices
	CapAdd               []string            `mapstructure:"cap_add"`                // Flags to pass directly to cap-add
	CapDrop              []string            `mapstructure:"cap_drop"`               // Flags to pass directly to cap-drop
	ReadonlyRootfs       bool                `mapstructure:"readonly_rootfs"`        // Mount the container’s root filesystem as read only
	AdvertiseIPv6Address bool                `mapstructure:"advertise_ipv6_address"` // Flag to use the GlobalIPv6Address from the container as the detected IP
	CPUHardLimit         bool                `mapstructure:"cpu_hard_limit"`         // Enforce CPU hard limit.
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

// Validate validates a docker driver config
func (c *DockerDriverConfig) Validate() error {
	if c.ImageName == "" {
		return fmt.Errorf("Docker Driver needs an image name")
	}
	if len(c.Devices) > 0 {
		for _, dev := range c.Devices {
			if dev.HostPath == "" {
				return fmt.Errorf("host path must be set in configuration for devices")
			}
			if dev.CgroupPermissions != "" {
				for _, c := range dev.CgroupPermissions {
					ch := string(c)
					if ch != "r" && ch != "w" && ch != "m" {
						return fmt.Errorf("invalid cgroup permission string: %q", dev.CgroupPermissions)
					}
				}
			}
		}
	}
	c.Sysctl = mapMergeStrStr(c.SysctlRaw...)
	c.Labels = mapMergeStrStr(c.LabelsRaw...)
	if len(c.Logging) > 0 {
		c.Logging[0].Config = mapMergeStrStr(c.Logging[0].ConfigRaw...)
	}

	mergedUlimitsRaw := mapMergeStrStr(c.UlimitRaw...)
	ulimit, err := sliceMergeUlimit(mergedUlimitsRaw)
	if err != nil {
		return err
	}
	c.Ulimit = ulimit
	return nil
}

// NewDockerDriverConfig returns a docker driver config by parsing the HCL
// config
func NewDockerDriverConfig(task *structs.Task, env *env.TaskEnv) (*DockerDriverConfig, error) {
	var dconf DockerDriverConfig

	if err := mapstructure.WeakDecode(task.Config, &dconf); err != nil {
		return nil, err
	}

	// Interpolate everything that is a string
	dconf.ImageName = env.ReplaceEnv(dconf.ImageName)
	dconf.Command = env.ReplaceEnv(dconf.Command)
	dconf.Entrypoint = env.ParseAndReplace(dconf.Entrypoint)
	dconf.IpcMode = env.ReplaceEnv(dconf.IpcMode)
	dconf.NetworkMode = env.ReplaceEnv(dconf.NetworkMode)
	dconf.NetworkAliases = env.ParseAndReplace(dconf.NetworkAliases)
	dconf.IPv4Address = env.ReplaceEnv(dconf.IPv4Address)
	dconf.IPv6Address = env.ReplaceEnv(dconf.IPv6Address)
	dconf.PidMode = env.ReplaceEnv(dconf.PidMode)
	dconf.UTSMode = env.ReplaceEnv(dconf.UTSMode)
	dconf.Hostname = env.ReplaceEnv(dconf.Hostname)
	dconf.WorkDir = env.ReplaceEnv(dconf.WorkDir)
	dconf.LoadImage = env.ReplaceEnv(dconf.LoadImage)
	dconf.Volumes = env.ParseAndReplace(dconf.Volumes)
	dconf.VolumeDriver = env.ReplaceEnv(dconf.VolumeDriver)
	dconf.DNSServers = env.ParseAndReplace(dconf.DNSServers)
	dconf.DNSSearchDomains = env.ParseAndReplace(dconf.DNSSearchDomains)
	dconf.DNSOptions = env.ParseAndReplace(dconf.DNSOptions)
	dconf.ExtraHosts = env.ParseAndReplace(dconf.ExtraHosts)
	dconf.MacAddress = env.ReplaceEnv(dconf.MacAddress)
	dconf.SecurityOpt = env.ParseAndReplace(dconf.SecurityOpt)
	dconf.CapAdd = env.ParseAndReplace(dconf.CapAdd)
	dconf.CapDrop = env.ParseAndReplace(dconf.CapDrop)

	for _, m := range dconf.SysctlRaw {
		for k, v := range m {
			delete(m, k)
			m[env.ReplaceEnv(k)] = env.ReplaceEnv(v)
		}
	}

	for _, m := range dconf.UlimitRaw {
		for k, v := range m {
			delete(m, k)
			m[env.ReplaceEnv(k)] = env.ReplaceEnv(v)
		}
	}

	for _, m := range dconf.LabelsRaw {
		for k, v := range m {
			delete(m, k)
			m[env.ReplaceEnv(k)] = env.ReplaceEnv(v)
		}
	}
	dconf.Labels = mapMergeStrStr(dconf.LabelsRaw...)

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

	for i, m := range dconf.Mounts {
		dconf.Mounts[i].Target = env.ReplaceEnv(m.Target)
		dconf.Mounts[i].Source = env.ReplaceEnv(m.Source)

		if len(m.VolumeOptions) > 1 {
			return nil, fmt.Errorf("Only one volume_options stanza allowed")
		}

		if len(m.VolumeOptions) == 1 {
			vo := m.VolumeOptions[0]
			if len(vo.Labels) > 1 {
				return nil, fmt.Errorf("labels may only be specified once in volume_options stanza")
			}

			if len(vo.Labels) == 1 {
				for k, v := range vo.Labels[0] {
					if k != env.ReplaceEnv(k) {
						delete(vo.Labels[0], k)
					}
					vo.Labels[0][env.ReplaceEnv(k)] = env.ReplaceEnv(v)
				}
			}

			if len(vo.DriverConfig) > 1 {
				return nil, fmt.Errorf("volume driver config may only be specified once")
			}
			if len(vo.DriverConfig) == 1 {
				vo.DriverConfig[0].Name = env.ReplaceEnv(vo.DriverConfig[0].Name)
				if len(vo.DriverConfig[0].Options) > 1 {
					return nil, fmt.Errorf("volume driver options may only be specified once")
				}

				if len(vo.DriverConfig[0].Options) == 1 {
					options := vo.DriverConfig[0].Options[0]
					for k, v := range options {
						if k != env.ReplaceEnv(k) {
							delete(options, k)
						}
						options[env.ReplaceEnv(k)] = env.ReplaceEnv(v)
					}
				}
			}
		}
	}

	if len(dconf.Logging) > 0 {
		dconf.Logging[0].Config = mapMergeStrStr(dconf.Logging[0].ConfigRaw...)
	}

	portMap := make(map[string]int)
	for _, m := range dconf.PortMapRaw {
		for k, v := range m {
			ki, vi := env.ReplaceEnv(k), env.ReplaceEnv(v)
			p, err := strconv.Atoi(vi)
			if err != nil {
				return nil, fmt.Errorf("failed to parse port map value %v to %v: %v", ki, vi, err)
			}
			portMap[ki] = p
		}
	}
	dconf.PortMap = portMap

	// Remove any http
	if strings.Contains(dconf.ImageName, "https://") {
		dconf.ImageName = strings.Replace(dconf.ImageName, "https://", "", 1)
	}

	// If devices are configured set default cgroup permissions
	if len(dconf.Devices) > 0 {
		for i, dev := range dconf.Devices {
			if dev.CgroupPermissions == "" {
				dev.CgroupPermissions = "rwm"
			}
			dconf.Devices[i] = dev
		}
	}

	if err := dconf.Validate(); err != nil {
		return nil, err
	}
	return &dconf, nil
}

type dockerPID struct {
	Version        string
	Image          string
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
	Image             string
	ImageID           string
	containerID       string
	version           string
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

func (d *DockerDriver) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	client, _, err := d.dockerClients()
	if err != nil {
		if d.fingerprintSuccess == nil || *d.fingerprintSuccess {
			d.logger.Printf("[INFO] driver.docker: failed to initialize client: %s", err)
		}
		d.fingerprintSuccess = helper.BoolToPtr(false)
		return nil
	}

	// This is the first operation taken on the client so we'll try to
	// establish a connection to the Docker daemon. If this fails it means
	// Docker isn't available so we'll simply disable the docker driver.
	env, err := client.Version()
	if err != nil {
		if d.fingerprintSuccess == nil || *d.fingerprintSuccess {
			d.logger.Printf("[DEBUG] driver.docker: could not connect to docker daemon at %s: %s", client.Endpoint(), err)
		}
		d.fingerprintSuccess = helper.BoolToPtr(false)
		resp.RemoveAttribute(dockerDriverAttr)
		return nil
	}

	resp.AddAttribute(dockerDriverAttr, "1")
	resp.AddAttribute("driver.docker.version", env.Get("Version"))
	resp.Detected = true

	privileged := d.config.ReadBoolDefault(dockerPrivilegedConfigOption, false)
	if privileged {
		resp.AddAttribute(dockerPrivilegedConfigOption, "1")
	}

	// Advertise if this node supports Docker volumes
	if d.config.ReadBoolDefault(dockerVolumesConfigOption, dockerVolumesConfigDefault) {
		resp.AddAttribute("driver."+dockerVolumesConfigOption, "1")
	}

	// Detect bridge IP address - #2785
	if nets, err := client.ListNetworks(); err != nil {
		d.logger.Printf("[WARN] driver.docker: error discovering bridge IP: %v", err)
	} else {
		for _, n := range nets {
			if n.Name != "bridge" {
				continue
			}

			if len(n.IPAM.Config) == 0 {
				d.logger.Printf("[WARN] driver.docker: no IPAM config for bridge network")
				break
			}

			if n.IPAM.Config[0].Gateway != "" {
				resp.AddAttribute("driver.docker.bridge_ip", n.IPAM.Config[0].Gateway)
			} else if d.fingerprintSuccess == nil {
				// Docker 17.09.0-ce dropped the Gateway IP from the bridge network
				// See https://github.com/moby/moby/issues/32648
				d.logger.Printf("[DEBUG] driver.docker: bridge_ip could not be discovered")
			}
			break
		}
	}

	d.fingerprintSuccess = helper.BoolToPtr(true)
	return nil
}

// HealthCheck implements the interface for the HealthCheck interface. This
// performs a health check on the docker driver, asserting whether the docker
// driver is responsive to a `docker ps` command.
func (d *DockerDriver) HealthCheck(req *cstructs.HealthCheckRequest, resp *cstructs.HealthCheckResponse) error {
	dinfo := &structs.DriverInfo{
		UpdateTime: time.Now(),
	}

	healthCheckClient, err := d.dockerHealthCheckClient()
	if err != nil {
		d.logger.Printf("[WARN] driver.docker: failed to retrieve Docker client in the process of a docker health check: %v", err)
		dinfo.HealthDescription = fmt.Sprintf("Failed retrieving Docker client: %v", err)
		resp.AddDriverInfo("docker", dinfo)
		return nil
	}

	_, err = healthCheckClient.ListContainers(docker.ListContainersOptions{All: false})
	if err != nil {
		d.logger.Printf("[WARN] driver.docker: failed to list Docker containers in the process of a Docker health check: %v", err)
		dinfo.HealthDescription = fmt.Sprintf("Failed to list Docker containers: %v", err)
		resp.AddDriverInfo("docker", dinfo)
		return nil
	}

	d.logger.Printf("[TRACE] driver.docker: docker driver is available and is responsive to `docker ps`")
	dinfo.Healthy = true
	dinfo.HealthDescription = "Driver is available and responsive"
	resp.AddDriverInfo("docker", dinfo)
	return nil
}

// GetHealthChecks implements the interface for the HealthCheck interface. This
// sets whether the driver is eligible for periodic health checks and the
// interval at which to do them.
func (d *DockerDriver) GetHealthCheckInterval(req *cstructs.HealthCheckIntervalRequest, resp *cstructs.HealthCheckIntervalResponse) error {
	resp.Eligible = true
	resp.Period = 1 * time.Minute
	return nil
}

// Validate is used to validate the driver configuration
func (d *DockerDriver) Validate(config map[string]interface{}) error {
	fd := &fields.FieldData{
		Raw: config,
		Schema: map[string]*fields.FieldSchema{
			"image": {
				Type:     fields.TypeString,
				Required: true,
			},
			"load": {
				Type: fields.TypeString,
			},
			"command": {
				Type: fields.TypeString,
			},
			"args": {
				Type: fields.TypeArray,
			},
			"entrypoint": {
				Type: fields.TypeArray,
			},
			"ipc_mode": {
				Type: fields.TypeString,
			},
			"network_mode": {
				Type: fields.TypeString,
			},
			"network_aliases": {
				Type: fields.TypeArray,
			},
			"ipv4_address": {
				Type: fields.TypeString,
			},
			"ipv6_address": {
				Type: fields.TypeString,
			},
			"mac_address": {
				Type: fields.TypeString,
			},
			"pid_mode": {
				Type: fields.TypeString,
			},
			"uts_mode": {
				Type: fields.TypeString,
			},
			"userns_mode": {
				Type: fields.TypeString,
			},
			"sysctl": {
				Type: fields.TypeArray,
			},
			"ulimit": {
				Type: fields.TypeArray,
			},
			"port_map": {
				Type: fields.TypeArray,
			},
			"privileged": {
				Type: fields.TypeBool,
			},
			"dns_servers": {
				Type: fields.TypeArray,
			},
			"dns_options": {
				Type: fields.TypeArray,
			},
			"dns_search_domains": {
				Type: fields.TypeArray,
			},
			"extra_hosts": {
				Type: fields.TypeArray,
			},
			"hostname": {
				Type: fields.TypeString,
			},
			"labels": {
				Type: fields.TypeArray,
			},
			"auth": {
				Type: fields.TypeArray,
			},
			"auth_soft_fail": {
				Type: fields.TypeBool,
			},
			// COMPAT: Remove in 0.6.0. SSL is no longer needed
			"ssl": {
				Type: fields.TypeBool,
			},
			"tty": {
				Type: fields.TypeBool,
			},
			"interactive": {
				Type: fields.TypeBool,
			},
			"shm_size": {
				Type: fields.TypeInt,
			},
			"work_dir": {
				Type: fields.TypeString,
			},
			"logging": {
				Type: fields.TypeArray,
			},
			"volumes": {
				Type: fields.TypeArray,
			},
			"volume_driver": {
				Type: fields.TypeString,
			},
			"mounts": {
				Type: fields.TypeArray,
			},
			"force_pull": {
				Type: fields.TypeBool,
			},
			"security_opt": {
				Type: fields.TypeArray,
			},
			"devices": {
				Type: fields.TypeArray,
			},
			"cap_add": {
				Type: fields.TypeArray,
			},
			"cap_drop": {
				Type: fields.TypeArray,
			},
			"readonly_rootfs": {
				Type: fields.TypeBool,
			},
			"advertise_ipv6_address": {
				Type: fields.TypeBool,
			},
			"cpu_hard_limit": {
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
		Exec:        true,
	}
}

func (d *DockerDriver) FSIsolation() cstructs.FSIsolation {
	return cstructs.FSIsolationImage
}

// getDockerCoordinator returns the docker coordinator and the caller ID to use when
// interacting with the coordinator
func (d *DockerDriver) getDockerCoordinator(client *docker.Client) (*dockerCoordinator, string) {
	config := &dockerCoordinatorConfig{
		client:      client,
		cleanup:     d.config.ReadBoolDefault(dockerCleanupImageConfigOption, dockerCleanupImageConfigDefault),
		logger:      d.logger,
		removeDelay: d.config.ReadDurationDefault(dockerImageRemoveDelayConfigOption, dockerImageRemoveDelayConfigDefault),
	}

	return GetDockerCoordinator(config), fmt.Sprintf("%s-%s", d.DriverContext.allocID, d.DriverContext.taskName)
}

func (d *DockerDriver) Prestart(ctx *ExecContext, task *structs.Task) (*PrestartResponse, error) {
	driverConfig, err := NewDockerDriverConfig(task, ctx.TaskEnv)
	if err != nil {
		return nil, err
	}

	// Set state needed by Start
	d.driverConfig = driverConfig

	// Initialize docker API clients
	client, _, err := d.dockerClients()
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to docker daemon: %s", err)
	}

	// Ensure the image is available
	id, err := d.createImage(driverConfig, client, ctx.TaskDir)
	if err != nil {
		return nil, err
	}
	d.imageID = id

	resp := NewPrestartResponse()
	resp.CreatedResources.Add(dockerImageResKey, id)

	// Return the PortMap if it's set
	if len(driverConfig.PortMap) > 0 {
		resp.Network = &cstructs.DriverNetwork{
			PortMap: driverConfig.PortMap,
		}
	}
	return resp, nil
}

func (d *DockerDriver) Start(ctx *ExecContext, task *structs.Task) (*StartResponse, error) {
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
		TaskEnv:        ctx.TaskEnv,
		Task:           task,
		Driver:         "docker",
		LogDir:         ctx.TaskDir.LogDir,
		TaskDir:        ctx.TaskDir.Dir,
		PortLowerBound: d.config.ClientMinPort,
		PortUpperBound: d.config.ClientMaxPort,
	}
	if err := exec.SetContext(executorCtx); err != nil {
		pluginClient.Kill()
		return nil, fmt.Errorf("failed to set executor context: %v", err)
	}

	// The user hasn't specified any logging options so launch our own syslog
	// server if possible.
	syslogAddr := ""
	if len(d.driverConfig.Logging) == 0 {
		if runtime.GOOS == "darwin" {
			d.logger.Printf("[DEBUG] driver.docker: disabling syslog driver as Docker for Mac workaround")
		} else {
			ss, err := exec.LaunchSyslogServer()
			if err != nil {
				pluginClient.Kill()
				return nil, fmt.Errorf("failed to start syslog collector: %v", err)
			}
			syslogAddr = ss.Addr
		}
	}

	config, err := d.createContainerConfig(ctx, task, d.driverConfig, syslogAddr)
	if err != nil {
		d.logger.Printf("[ERR] driver.docker: failed to create container configuration for image %q (%q): %v", d.driverConfig.ImageName, d.imageID, err)
		pluginClient.Kill()
		return nil, fmt.Errorf("Failed to create container configuration for image %q (%q): %v", d.driverConfig.ImageName, d.imageID, err)
	}

	container, err := d.createContainer(client, config)
	if err != nil {
		wrapped := fmt.Sprintf("Failed to create container: %v", err)
		d.logger.Printf("[ERR] driver.docker: %s", wrapped)
		pluginClient.Kill()
		return nil, structs.WrapRecoverable(wrapped, err)
	}

	d.logger.Printf("[INFO] driver.docker: created container %s", container.ID)

	// We don't need to start the container if the container is already running
	// since we don't create containers which are already present on the host
	// and are running
	if !container.State.Running {
		// Start the container
		if err := d.startContainer(container); err != nil {
			d.logger.Printf("[ERR] driver.docker: failed to start container %s: %s", container.ID, err)
			pluginClient.Kill()
			return nil, structs.NewRecoverableError(fmt.Errorf("Failed to start container %s: %s", container.ID, err), structs.IsRecoverable(err))
		}

		// InspectContainer to get all of the container metadata as
		// much of the metadata (eg networking) isn't populated until
		// the container is started
		runningContainer, err := client.InspectContainer(container.ID)
		if err != nil {
			err = fmt.Errorf("failed to inspect started container %s: %s", container.ID, err)
			d.logger.Printf("[ERR] driver.docker: %v", err)
			pluginClient.Kill()
			return nil, structs.NewRecoverableError(err, true)
		}
		container = runningContainer
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
		logger:         d.logger,
		Image:          d.driverConfig.ImageName,
		ImageID:        d.imageID,
		containerID:    container.ID,
		version:        d.config.Version.VersionNumber(),
		killTimeout:    GetKillTimeout(task.KillTimeout, maxKill),
		maxKillTimeout: maxKill,
		doneCh:         make(chan bool),
		waitCh:         make(chan *dstructs.WaitResult, 1),
	}
	go h.collectStats()
	go h.run()

	// Detect container address
	ip, autoUse := d.detectIP(container)

	// Create a response with the driver handle and container network metadata
	resp := &StartResponse{
		Handle: h,
		Network: &cstructs.DriverNetwork{
			PortMap:       d.driverConfig.PortMap,
			IP:            ip,
			AutoAdvertise: autoUse,
		},
	}
	return resp, nil
}

// detectIP of Docker container. Returns the first IP found as well as true if
// the IP should be advertised (bridge network IPs return false). Returns an
// empty string and false if no IP could be found.
func (d *DockerDriver) detectIP(c *docker.Container) (string, bool) {
	if c.NetworkSettings == nil {
		// This should only happen if there's been a coding error (such
		// as not calling InspectContainer after CreateContainer). Code
		// defensively in case the Docker API changes subtly.
		d.logger.Printf("[ERROR] driver.docker: no network settings for container %s", c.ID)
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
		if d.driverConfig.AdvertiseIPv6Address {
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
		d.logger.Printf("[WARN] driver.docker: task %s multiple (%d) Docker networks for container %q but Nomad only supports 1: choosing %q", d.taskName, n, c.ID, ipName)
	}

	return ip, auto
}

func (d *DockerDriver) Cleanup(_ *ExecContext, res *CreatedResources) error {
	retry := false
	var merr multierror.Error
	for key, resources := range res.Resources {
		switch key {
		case dockerImageResKey:
			for _, value := range resources {
				err := d.cleanupImage(value)
				if err != nil {
					if structs.IsRecoverable(err) {
						retry = true
					}
					merr.Errors = append(merr.Errors, err)
					continue
				}

				// Remove cleaned image from resources
				res.Remove(dockerImageResKey, value)
			}
		default:
			d.logger.Printf("[ERR] driver.docker: unknown resource to cleanup: %q", key)
		}
	}
	return structs.NewRecoverableError(merr.ErrorOrNil(), retry)
}

// cleanupImage removes a Docker image. No error is returned if the image
// doesn't exist or is still in use. Requires the global client to already be
// initialized.
func (d *DockerDriver) cleanupImage(imageID string) error {
	if !d.config.ReadBoolDefault(dockerCleanupImageConfigOption, dockerCleanupImageConfigDefault) {
		// Config says not to cleanup
		return nil
	}

	coordinator, callerID := d.getDockerCoordinator(client)
	coordinator.RemoveImage(imageID, callerID)

	return nil
}

// dockerHealthCheckClient creates a single *docker.Client with a timeout of
// one minute, which will be used when performing Docker health checks.
func (d *DockerDriver) dockerHealthCheckClient() (*docker.Client, error) {
	createClientsLock.Lock()
	defer createClientsLock.Unlock()

	if healthCheckClient != nil {
		return healthCheckClient, nil
	}

	var err error
	healthCheckClient, err = d.newDockerClient(dockerHealthCheckTimeout)
	if err != nil {
		return nil, err
	}

	return healthCheckClient, nil
}

// dockerClients creates two *docker.Client, one for long running operations and
// the other for shorter operations. In test / dev mode we can use ENV vars to
// connect to the docker daemon. In production mode we will read docker.endpoint
// from the config file.
func (d *DockerDriver) dockerClients() (*docker.Client, *docker.Client, error) {
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
func (d *DockerDriver) newDockerClient(timeout time.Duration) (*docker.Client, error) {
	var err error
	var merr multierror.Error
	var newClient *docker.Client

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
			newClient, err = docker.NewTLSClient(dockerEndpoint, cert, key, ca)
			if err != nil {
				merr.Errors = append(merr.Errors, err)
			}
		} else {
			d.logger.Printf("[DEBUG] driver.docker: using standard client connection to %s", dockerEndpoint)
			newClient, err = docker.NewClient(dockerEndpoint)
			if err != nil {
				merr.Errors = append(merr.Errors, err)
			}
		}
	} else {
		d.logger.Println("[DEBUG] driver.docker: using client connection initialized from environment")
		newClient, err = docker.NewClientFromEnv()
		if err != nil {
			merr.Errors = append(merr.Errors, err)
		}
	}

	if timeout != 0 {
		newClient.SetTimeout(timeout)
	}
	return newClient, merr.ErrorOrNil()
}

func (d *DockerDriver) containerBinds(driverConfig *DockerDriverConfig, ctx *ExecContext,
	task *structs.Task) ([]string, error) {

	allocDirBind := fmt.Sprintf("%s:%s", ctx.TaskDir.SharedAllocDir, ctx.TaskEnv.EnvMap[env.AllocDir])
	taskLocalBind := fmt.Sprintf("%s:%s", ctx.TaskDir.LocalDir, ctx.TaskEnv.EnvMap[env.TaskLocalDir])
	secretDirBind := fmt.Sprintf("%s:%s", ctx.TaskDir.SecretsDir, ctx.TaskEnv.EnvMap[env.SecretsDir])
	binds := []string{allocDirBind, taskLocalBind, secretDirBind}

	volumesEnabled := d.config.ReadBoolDefault(dockerVolumesConfigOption, dockerVolumesConfigDefault)

	if !volumesEnabled && driverConfig.VolumeDriver != "" {
		return nil, fmt.Errorf("%s is false; cannot use volume driver %q", dockerVolumesConfigOption, driverConfig.VolumeDriver)
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
			if !volumesEnabled {
				// Disallow mounting arbitrary absolute paths
				return nil, fmt.Errorf("%s is false; cannot mount host paths: %+q", dockerVolumesConfigOption, userbind)
			}
			binds = append(binds, userbind)
			continue
		}

		// Relative paths are always allowed as they mount within a container
		// When a VolumeDriver is set, we assume we receive a binding in the format volume-name:container-dest
		// Otherwise, we assume we receive a relative path binding in the format relative/to/task:/also/in/container
		if driverConfig.VolumeDriver == "" {
			// Expand path relative to alloc dir
			parts[0] = filepath.Join(ctx.TaskDir.Dir, parts[0])
		}

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

	binds, err := d.containerBinds(driverConfig, ctx, task)
	if err != nil {
		return c, err
	}

	// create the config block that will later be consumed by go-dockerclient
	config := &docker.Config{
		Image:       d.imageID,
		Entrypoint:  driverConfig.Entrypoint,
		Hostname:    driverConfig.Hostname,
		User:        task.User,
		Tty:         driverConfig.TTY,
		OpenStdin:   driverConfig.Interactive,
		StopTimeout: int(task.KillTimeout.Seconds()),
		StopSignal:  task.KillSignal,
	}

	if driverConfig.WorkDir != "" {
		config.WorkingDir = driverConfig.WorkDir
	}

	memLimit := int64(task.Resources.MemoryMB) * 1024 * 1024

	if len(driverConfig.Logging) == 0 {
		if runtime.GOOS == "darwin" {
			d.logger.Printf("[DEBUG] driver.docker: deferring logging to docker on Docker for Mac")
		} else {
			d.logger.Printf("[DEBUG] driver.docker: Setting default logging options to syslog and %s", syslogAddr)
			driverConfig.Logging = []DockerLoggingOpts{
				{Type: "syslog", Config: map[string]string{"syslog-address": syslogAddr}},
			}
		}
	}

	hostConfig := &docker.HostConfig{
		// Convert MB to bytes. This is an absolute value.
		Memory: memLimit,
		// Convert Mhz to shares. This is a relative value.
		CPUShares: int64(task.Resources.CPU),

		// Binds are used to mount a host volume into the container. We mount a
		// local directory for storage and a shared alloc directory that can be
		// used to share data between different tasks in the same task group.
		Binds: binds,

		VolumeDriver: driverConfig.VolumeDriver,
	}

	// Calculate CPU Quota
	// cfs_quota_us is the time per core, so we must
	// multiply the time by the number of cores available
	// See https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/6/html/resource_management_guide/sec-cpu
	if driverConfig.CPUHardLimit {
		numCores := runtime.NumCPU()
		percentTicks := float64(task.Resources.CPU) / float64(d.node.Resources.CPU)
		hostConfig.CPUQuota = int64(percentTicks*defaultCFSPeriodUS) * int64(numCores)
	}

	// Windows does not support MemorySwap/MemorySwappiness #2193
	if runtime.GOOS == "windows" {
		hostConfig.MemorySwap = 0
		hostConfig.MemorySwappiness = -1
	} else {
		hostConfig.MemorySwap = memLimit // MemorySwap is memory + swap.
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
	if driverConfig.CPUHardLimit {
		d.logger.Printf("[DEBUG] driver.docker: using %dms cpu quota and %dms cpu period for %s", hostConfig.CPUQuota, defaultCFSPeriodUS, task.Name)
	}
	d.logger.Printf("[DEBUG] driver.docker: binding directories %#v for %s", hostConfig.Binds, task.Name)

	//  set privileged mode
	hostPrivileged := d.config.ReadBoolDefault(dockerPrivilegedConfigOption, false)
	if driverConfig.Privileged && !hostPrivileged {
		return c, fmt.Errorf(`Docker privileged mode is disabled on this Nomad agent`)
	}
	hostConfig.Privileged = driverConfig.Privileged

	// set capabilities
	hostCapsWhitelistConfig := d.config.ReadDefault(
		dockerCapsWhitelistConfigOption, dockerCapsWhitelistConfigDefault)
	hostCapsWhitelist := make(map[string]struct{})
	for _, cap := range strings.Split(hostCapsWhitelistConfig, ",") {
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
			d.logger.Printf("[ERR] driver.docker: invalid ip address for container dns server: %s", ip)
		}
	}

	if len(driverConfig.Devices) > 0 {
		var devices []docker.Device
		for _, device := range driverConfig.Devices {
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
		if len(m.VolumeOptions) == 1 {
			vo := m.VolumeOptions[0]
			hm.VolumeOptions = &docker.VolumeOptions{
				NoCopy: vo.NoCopy,
			}

			if len(vo.DriverConfig) == 1 {
				dc := vo.DriverConfig[0]
				hm.VolumeOptions.DriverConfig = docker.VolumeDriverConfig{
					Name: dc.Name,
				}
				if len(dc.Options) == 1 {
					hm.VolumeOptions.DriverConfig.Options = dc.Options[0]
				}
			}
			if len(vo.Labels) == 1 {
				hm.VolumeOptions.Labels = vo.Labels[0]
			}
		}
		hostConfig.Mounts = append(hostConfig.Mounts, hm)
	}

	// set DNS search domains and extra hosts
	hostConfig.DNSSearch = driverConfig.DNSSearchDomains
	hostConfig.DNSOptions = driverConfig.DNSOptions
	hostConfig.ExtraHosts = driverConfig.ExtraHosts

	hostConfig.IpcMode = driverConfig.IpcMode
	hostConfig.PidMode = driverConfig.PidMode
	hostConfig.UTSMode = driverConfig.UTSMode
	hostConfig.UsernsMode = driverConfig.UsernsMode
	hostConfig.SecurityOpt = driverConfig.SecurityOpt
	hostConfig.Sysctls = driverConfig.Sysctl
	hostConfig.Ulimits = driverConfig.Ulimit
	hostConfig.ReadonlyRootfs = driverConfig.ReadonlyRootfs

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

		hostConfig.PortBindings = publishedPorts
		config.ExposedPorts = exposedPorts
	}

	parsedArgs := ctx.TaskEnv.ParseAndReplace(driverConfig.Args)

	// If the user specified a custom command to run, we'll inject it here.
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

	config.Env = ctx.TaskEnv.List()

	containerName := fmt.Sprintf("%s-%s", task.Name, d.DriverContext.allocID)
	d.logger.Printf("[DEBUG] driver.docker: setting container name to: %s", containerName)

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
		d.logger.Printf("[DEBUG] driver.docker: using network_mode %q with network aliases: %v",
			hostConfig.NetworkMode, strings.Join(driverConfig.NetworkAliases, ", "))
	}

	if driverConfig.IPv4Address != "" || driverConfig.IPv6Address != "" {
		networkingConfig.EndpointsConfig[hostConfig.NetworkMode].IPAMConfig = &docker.EndpointIPAMConfig{
			IPv4Address: driverConfig.IPv4Address,
			IPv6Address: driverConfig.IPv6Address,
		}
		d.logger.Printf("[DEBUG] driver.docker: using network_mode %q with ipv4: %q and ipv6: %q",
			hostConfig.NetworkMode, driverConfig.IPv4Address, driverConfig.IPv6Address)
	}

	if driverConfig.MacAddress != "" {
		config.MacAddress = driverConfig.MacAddress
		d.logger.Printf("[DEBUG] driver.docker: using pinned mac address: %q", config.MacAddress)
	}

	return docker.CreateContainerOptions{
		Name:             containerName,
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
	}, nil
}

func (d *DockerDriver) Periodic() (bool, time.Duration) {
	return true, 15 * time.Second
}

// createImage creates a docker image either by pulling it from a registry or by
// loading it from the file system
func (d *DockerDriver) createImage(driverConfig *DockerDriverConfig, client *docker.Client, taskDir *allocdir.TaskDir) (string, error) {
	image := driverConfig.ImageName
	repo, tag := docker.ParseRepositoryTag(image)
	if tag == "" {
		tag = "latest"
	}

	coordinator, callerID := d.getDockerCoordinator(client)

	// We're going to check whether the image is already downloaded. If the tag
	// is "latest", or ForcePull is set, we have to check for a new version every time so we don't
	// bother to check and cache the id here. We'll download first, then cache.
	if driverConfig.ForcePull {
		d.logger.Printf("[DEBUG] driver.docker: force pull image '%s:%s' instead of inspecting local", repo, tag)
	} else if tag != "latest" {
		if dockerImage, _ := client.InspectImage(image); dockerImage != nil {
			// Image exists so just increment its reference count
			coordinator.IncrementImageReference(dockerImage.ID, image, callerID)
			return dockerImage.ID, nil
		}
	}

	// Load the image if specified
	if driverConfig.LoadImage != "" {
		return d.loadImage(driverConfig, client, taskDir)
	}

	// Download the image
	return d.pullImage(driverConfig, client, repo, tag)
}

// pullImage creates an image by pulling it from a docker registry
func (d *DockerDriver) pullImage(driverConfig *DockerDriverConfig, client *docker.Client, repo, tag string) (id string, err error) {
	authOptions, err := d.resolveRegistryAuthentication(driverConfig, repo)
	if err != nil {
		if d.driverConfig.AuthSoftFail {
			d.logger.Printf("[WARN] Failed to find docker auth for repo %q: %v", repo, err)
		} else {
			return "", fmt.Errorf("Failed to find docker auth for repo %q: %v", repo, err)
		}
	}

	if authIsEmpty(authOptions) {
		d.logger.Printf("[DEBUG] driver.docker: did not find docker auth for repo %q", repo)
	}

	d.emitEvent("Downloading image %s:%s", repo, tag)
	coordinator, callerID := d.getDockerCoordinator(client)

	return coordinator.PullImage(driverConfig.ImageName, authOptions, callerID, d.emitEvent)
}

// authBackend encapsulates a function that resolves registry credentials.
type authBackend func(string) (*docker.AuthConfiguration, error)

// resolveRegistryAuthentication attempts to retrieve auth credentials for the
// repo, trying all authentication-backends possible.
func (d *DockerDriver) resolveRegistryAuthentication(driverConfig *DockerDriverConfig, repo string) (*docker.AuthConfiguration, error) {
	return firstValidAuth(repo, []authBackend{
		authFromTaskConfig(driverConfig),
		authFromDockerConfig(d.config.Read("docker.auth.config")),
		authFromHelper(d.config.Read("docker.auth.helper")),
	})
}

// loadImage creates an image by loading it from the file system
func (d *DockerDriver) loadImage(driverConfig *DockerDriverConfig, client *docker.Client,
	taskDir *allocdir.TaskDir) (id string, err error) {

	archive := filepath.Join(taskDir.LocalDir, driverConfig.LoadImage)
	d.logger.Printf("[DEBUG] driver.docker: loading image from: %v", archive)

	f, err := os.Open(archive)
	if err != nil {
		return "", fmt.Errorf("unable to open image archive: %v", err)
	}

	if err := client.LoadImage(docker.LoadImageOptions{InputStream: f}); err != nil {
		return "", err
	}
	f.Close()

	dockerImage, err := client.InspectImage(driverConfig.ImageName)
	if err != nil {
		return "", recoverableErrTimeouts(err)
	}

	coordinator, callerID := d.getDockerCoordinator(client)
	coordinator.IncrementImageReference(dockerImage.ID, driverConfig.ImageName, callerID)
	return dockerImage.ID, nil
}

// createContainer creates the container given the passed configuration. It
// attempts to handle any transient Docker errors.
func (d *DockerDriver) createContainer(client createContainerClient, config docker.CreateContainerOptions) (*docker.Container, error) {
	// Create a container
	attempted := 0
CREATE:
	container, createErr := client.CreateContainer(config)
	if createErr == nil {
		return container, nil
	}

	d.logger.Printf("[DEBUG] driver.docker: failed to create container %q from image %q (ID: %q) (attempt %d): %v",
		config.Name, d.driverConfig.ImageName, d.imageID, attempted+1, createErr)

	// Volume management tools like Portworx may not have detached a volume
	// from a previous node before Nomad started a task replacement task.
	// Treat these errors as recoverable so we retry.
	if strings.Contains(strings.ToLower(createErr.Error()), "volume is attached on another node") {
		return nil, structs.NewRecoverableError(createErr, true)
	}

	// If the container already exists determine whether it's already
	// running or if it's dead and needs to be recreated.
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
		for _, shimContainer := range containers {
			d.logger.Printf("[DEBUG] driver.docker: listed container %+v", shimContainer.Names)
			found := false
			for _, name := range shimContainer.Names {
				if name == containerName {
					d.logger.Printf("[DEBUG] driver.docker: Found container %v: %v", containerName, shimContainer.ID)
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
				return nil, structs.NewRecoverableError(err, true)
			}
			if container != nil && container.State.Running {
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
	} else if strings.Contains(strings.ToLower(createErr.Error()), "no such image") {
		// There is still a very small chance this is possible even with the
		// coordinator so retry.
		return nil, structs.NewRecoverableError(createErr, true)
	}

	return nil, recoverableErrTimeouts(createErr)
}

// startContainer starts the passed container. It attempts to handle any
// transient Docker errors.
func (d *DockerDriver) startContainer(c *docker.Container) error {
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
		return structs.NewRecoverableError(startErr, true)
	}

	return recoverableErrTimeouts(startErr)
}

func (d *DockerDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
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
			"id": {pid.ContainerID},
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

	// Increment the reference count since we successfully attached to this
	// container
	coordinator, callerID := d.getDockerCoordinator(client)
	coordinator.IncrementImageReference(pid.ImageID, pid.Image, callerID)

	// Return a driver handle
	h := &DockerHandle{
		client:         client,
		waitClient:     waitClient,
		executor:       exec,
		pluginClient:   pluginClient,
		logger:         d.logger,
		Image:          pid.Image,
		ImageID:        pid.ImageID,
		containerID:    pid.ContainerID,
		version:        pid.Version,
		killTimeout:    pid.KillTimeout,
		maxKillTimeout: pid.MaxKillTimeout,
		doneCh:         make(chan bool),
		waitCh:         make(chan *dstructs.WaitResult, 1),
	}
	go h.collectStats()
	go h.run()
	return h, nil
}

func (h *DockerHandle) ID() string {
	// Return a handle to the PID
	pid := dockerPID{
		Version:        h.version,
		ContainerID:    h.containerID,
		Image:          h.Image,
		ImageID:        h.ImageID,
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

func (h *DockerHandle) Exec(ctx context.Context, cmd string, args []string) ([]byte, int, error) {
	fullCmd := make([]string, len(args)+1)
	fullCmd[0] = cmd
	copy(fullCmd[1:], args)
	createExecOpts := docker.CreateExecOptions{
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Cmd:          fullCmd,
		Container:    h.containerID,
		Context:      ctx,
	}
	exec, err := h.client.CreateExec(createExecOpts)
	if err != nil {
		return nil, 0, err
	}

	output, _ := circbuf.NewBuffer(int64(dstructs.CheckBufSize))
	startOpts := docker.StartExecOptions{
		Detach:       false,
		Tty:          false,
		OutputStream: output,
		ErrorStream:  output,
		Context:      ctx,
	}
	if err := client.StartExec(exec.ID, startOpts); err != nil {
		return nil, 0, err
	}
	res, err := client.InspectExec(exec.ID)
	if err != nil {
		return output.Bytes(), 0, err
	}
	return output.Bytes(), res.ExitCode, nil
}

func (h *DockerHandle) Signal(s os.Signal) error {
	// Convert types
	sysSig, ok := s.(syscall.Signal)
	if !ok {
		return fmt.Errorf("Failed to determine signal number")
	}

	// TODO When we expose signals we will need a mapping layer that converts
	// MacOS signals to the correct signal number for docker. Or we change the
	// interface to take a signal string and leave it up to driver to map?

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
			h.logger.Printf("[DEBUG] driver.docker: attempted to stop nonexistent container %s", h.containerID)
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

	container, ierr := h.waitClient.InspectContainer(h.containerID)
	if ierr != nil {
		h.logger.Printf("[ERR] driver.docker: failed to inspect container %s: %v", h.containerID, ierr)
	} else if container.State.OOMKilled {
		werr = fmt.Errorf("OOM Killed")
	}

	close(h.doneCh)

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
				cs.Percent = calculatePercent(
					s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage,
					s.CPUStats.SystemCPUUsage, s.PreCPUStats.SystemCPUUsage, numCores)
				cs.SystemMode = calculatePercent(
					s.CPUStats.CPUUsage.UsageInKernelmode, s.PreCPUStats.CPUUsage.UsageInKernelmode,
					s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage, numCores)
				cs.UserMode = calculatePercent(
					s.CPUStats.CPUUsage.UsageInUsermode, s.PreCPUStats.CPUUsage.UsageInUsermode,
					s.CPUStats.CPUUsage.TotalUsage, s.PreCPUStats.CPUUsage.TotalUsage, numCores)
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

// loadDockerConfig loads the docker config at the specified path, returning an
// error if it couldn't be read.
func loadDockerConfig(file string) (*configfile.ConfigFile, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("Failed to open auth config file: %v, error: %v", file, err)
	}
	defer f.Close()

	cfile := new(configfile.ConfigFile)
	if err = cfile.LoadFromReader(f); err != nil {
		return nil, fmt.Errorf("Failed to parse auth config file: %v", err)
	}
	return cfile, nil
}

// parseRepositoryInfo takes a repo and returns the Docker RepositoryInfo. This
// is useful for interacting with a Docker config object.
func parseRepositoryInfo(repo string) (*registry.RepositoryInfo, error) {
	name, err := reference.ParseNamed(repo)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse named repo %q: %v", repo, err)
	}

	repoInfo, err := registry.ParseRepositoryInfo(name)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse repository: %v", err)
	}

	return repoInfo, nil
}

// firstValidAuth tries a list of auth backends, returning first error or AuthConfiguration
func firstValidAuth(repo string, backends []authBackend) (*docker.AuthConfiguration, error) {
	for _, backend := range backends {
		auth, err := backend(repo)
		if auth != nil || err != nil {
			return auth, err
		}
	}
	return nil, nil
}

// authFromTaskConfig generates an authBackend for any auth given in the task-configuration
func authFromTaskConfig(driverConfig *DockerDriverConfig) authBackend {
	return func(string) (*docker.AuthConfiguration, error) {
		if len(driverConfig.Auth) == 0 {
			return nil, nil
		}
		auth := driverConfig.Auth[0]
		return &docker.AuthConfiguration{
			Username:      auth.Username,
			Password:      auth.Password,
			Email:         auth.Email,
			ServerAddress: auth.ServerAddress,
		}, nil
	}
}

// authFromDockerConfig generate an authBackend for a dockercfg-compatible file.
// The authBacken can either be from explicit auth definitions or via credential
// helpers
func authFromDockerConfig(file string) authBackend {
	return func(repo string) (*docker.AuthConfiguration, error) {
		if file == "" {
			return nil, nil
		}
		repoInfo, err := parseRepositoryInfo(repo)
		if err != nil {
			return nil, err
		}

		cfile, err := loadDockerConfig(file)
		if err != nil {
			return nil, err
		}

		return firstValidAuth(repo, []authBackend{
			func(string) (*docker.AuthConfiguration, error) {
				dockerAuthConfig := registry.ResolveAuthConfig(cfile.AuthConfigs, repoInfo.Index)
				auth := &docker.AuthConfiguration{
					Username:      dockerAuthConfig.Username,
					Password:      dockerAuthConfig.Password,
					Email:         dockerAuthConfig.Email,
					ServerAddress: dockerAuthConfig.ServerAddress,
				}
				if authIsEmpty(auth) {
					return nil, nil
				}
				return auth, nil
			},
			authFromHelper(cfile.CredentialHelpers[registry.GetAuthConfigKey(repoInfo.Index)]),
			authFromHelper(cfile.CredentialsStore),
		})
	}
}

// authFromHelper generates an authBackend for a docker-credentials-helper;
// A script taking the requested domain on input, outputting JSON with
// "Username" and "Secret"
func authFromHelper(helperName string) authBackend {
	return func(repo string) (*docker.AuthConfiguration, error) {
		if helperName == "" {
			return nil, nil
		}
		helper := dockerAuthHelperPrefix + helperName
		cmd := exec.Command(helper, "get")

		// Ensure that the HTTPs prefix exists
		if !strings.HasPrefix(repo, "https://") {
			repo = fmt.Sprintf("https://%s", repo)
		}

		cmd.Stdin = strings.NewReader(repo)

		output, err := cmd.Output()
		if err != nil {
			switch err.(type) {
			default:
				return nil, err
			case *exec.ExitError:
				return nil, fmt.Errorf("%s with input %q failed with stderr: %s", helper, repo, output)
			}
		}

		var response map[string]string
		if err := json.Unmarshal(output, &response); err != nil {
			return nil, err
		}

		auth := &docker.AuthConfiguration{
			Username: response["Username"],
			Password: response["Secret"],
		}

		if authIsEmpty(auth) {
			return nil, nil
		}
		return auth, nil
	}
}

// authIsEmpty returns if auth is nil or an empty structure
func authIsEmpty(auth *docker.AuthConfiguration) bool {
	if auth == nil {
		return false
	}
	return auth.Username == "" &&
		auth.Password == "" &&
		auth.Email == "" &&
		auth.ServerAddress == ""
}

// createContainerClient is the subset of Docker Client methods used by the
// createContainer method to ease testing subtle error conditions.
type createContainerClient interface {
	CreateContainer(docker.CreateContainerOptions) (*docker.Container, error)
	InspectContainer(id string) (*docker.Container, error)
	ListContainers(docker.ListContainersOptions) ([]docker.APIContainers, error)
	RemoveContainer(opts docker.RemoveContainerOptions) error
}
