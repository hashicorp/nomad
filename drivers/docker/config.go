package docker

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

const (
	// NoSuchContainerError is returned by the docker daemon if the container
	// does not exist.
	NoSuchContainerError = "No such container"

	// ContainerNotRunningError is returned by the docker daemon if the container
	// is not running, yet we requested it to stop
	ContainerNotRunningError = "Container not running"

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

func PluginLoader(opts map[string]string) (map[string]interface{}, error) {
	conf := map[string]interface{}{}
	if v, ok := opts["docker.endpoint"]; ok {
		conf["endpoint"] = v
	}

	// dockerd auth
	authConf := map[string]interface{}{}
	if v, ok := opts["docker.auth.config"]; ok {
		authConf["config"] = v
	}
	if v, ok := opts["docker.auth.helper"]; ok {
		authConf["helper"] = v
	}
	conf["auth"] = authConf

	// dockerd tls
	if _, ok := opts["docker.tls.cert"]; ok {
		conf["tls"] = map[string]interface{}{
			"cert": opts["docker.tls.cert"],
			"key":  opts["docker.tls.key"],
			"ca":   opts["docker.tls.ca"],
		}
	}

	// garbage collection
	gcConf := map[string]interface{}{}
	if v, err := strconv.ParseBool(opts["docker.cleanup.image"]); err == nil {
		gcConf["image"] = v
	}
	if v, ok := opts["docker.cleanup.image.delay"]; ok {
		gcConf["image_delay"] = v
	}
	if v, err := strconv.ParseBool(opts["docker.cleanup.container"]); err == nil {
		gcConf["container"] = v
	}
	conf["gc"] = gcConf

	// volume options
	volConf := map[string]interface{}{}
	if v, err := strconv.ParseBool(opts["docker.volumes.enabled"]); err == nil {
		volConf["enabled"] = v
	}
	if v, ok := opts["docker.volumes.selinuxlabel"]; ok {
		volConf["selinuxlabel"] = v
	}
	conf["volumes"] = volConf

	// capabilities
	if v, ok := opts["docker.caps.whitelist"]; ok {
		conf["allow_caps"] = strings.Split(v, ",")
	}

	// privileged containers
	if v, err := strconv.ParseBool(opts["docker.privileged.enabled"]); err == nil {
		conf["allow_privileged"] = v
	}

	// nvidia_runtime
	if v, ok := opts["docker.nvidia_runtime"]; ok {
		conf["nvidia_runtime"] = v
	}

	return conf, nil
}

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

	// pluginInfo is the response returned for the PluginInfo RPC
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeDriver,
		PluginApiVersions: []string{drivers.ApiVersion010},
		PluginVersion:     "0.1.0",
		Name:              pluginName,
	}

	// configSpec is the hcl specification returned by the ConfigSchema RPC
	// and is used to parse the contents of the 'plugin "docker" {...}' block.
	// Example:
	//	plugin "docker" {
	//		config {
	//		endpoint = "unix:///var/run/docker.sock"
	//		auth {
	//			config = "/etc/docker-auth.json"
	//			helper = "docker-credential-aws"
	//		}
	//		tls {
	//			cert = "/etc/nomad/nomad.pub"
	//			key = "/etc/nomad/nomad.pem"
	//			ca = "/etc/nomad/nomad.cert"
	//		}
	//		gc {
	//			image = true
	//			image_delay = "5m"
	//			container = false
	//		}
	//		volumes {
	//			enabled = true
	//			selinuxlabel = "z"
	//		}
	//		allow_privileged = false
	//		allow_caps = ["CHOWN", "NET_RAW" ... ]
	//		nvidia_runtime = "nvidia"
	//		}
	//	}
	configSpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"endpoint": hclspec.NewAttr("endpoint", "string", false),

		// docker daemon auth option for image registry
		"auth": hclspec.NewBlock("auth", false, hclspec.NewObject(map[string]*hclspec.Spec{
			"config": hclspec.NewAttr("config", "string", false),
			"helper": hclspec.NewAttr("helper", "string", false),
		})),

		// client tls options
		"tls": hclspec.NewBlock("tls", false, hclspec.NewObject(map[string]*hclspec.Spec{
			"cert": hclspec.NewAttr("cert", "string", false),
			"key":  hclspec.NewAttr("key", "string", false),
			"ca":   hclspec.NewAttr("ca", "string", false),
		})),

		// garbage collection options
		// default needed for both if the gc {...} block is not set and
		// if the default fields are missing
		"gc": hclspec.NewDefault(hclspec.NewBlock("gc", false, hclspec.NewObject(map[string]*hclspec.Spec{
			"image": hclspec.NewDefault(
				hclspec.NewAttr("image", "bool", false),
				hclspec.NewLiteral("true"),
			),
			"image_delay": hclspec.NewAttr("image_delay", "string", false),
			"container": hclspec.NewDefault(
				hclspec.NewAttr("container", "bool", false),
				hclspec.NewLiteral("true"),
			),
		})), hclspec.NewLiteral(`{
			image = true
			container = true
		}`)),

		// docker volume options
		// defaulted needed for both if the volumes {...} block is not set and
		// if the default fields are missing
		"volumes": hclspec.NewDefault(hclspec.NewBlock("volumes", false, hclspec.NewObject(map[string]*hclspec.Spec{
			"enabled": hclspec.NewDefault(
				hclspec.NewAttr("enabled", "bool", false),
				hclspec.NewLiteral("true"),
			),
			"selinuxlabel": hclspec.NewAttr("selinuxlabel", "string", false),
		})), hclspec.NewLiteral("{ enabled = true }")),
		"allow_privileged": hclspec.NewAttr("allow_privileged", "bool", false),
		"allow_caps": hclspec.NewDefault(
			hclspec.NewAttr("allow_caps", "list(string)", false),
			hclspec.NewLiteral(`["CHOWN","DAC_OVERRIDE","FSETID","FOWNER","MKNOD","NET_RAW","SETGID","SETUID","SETFCAP","SETPCAP","NET_BIND_SERVICE","SYS_CHROOT","KILL","AUDIT_WRITE"]`),
		),
		"nvidia_runtime": hclspec.NewDefault(
			hclspec.NewAttr("nvidia_runtime", "string", false),
			hclspec.NewLiteral(`"nvidia"`),
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
		"devices": hclspec.NewBlockList("devices", hclspec.NewObject(map[string]*hclspec.Spec{
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
		"labels":             hclspec.NewAttr("labels", "list(map(string))", false),
		"load":               hclspec.NewAttr("load", "string", false),
		"logging": hclspec.NewBlock("logging", false, hclspec.NewObject(map[string]*hclspec.Spec{
			"type":   hclspec.NewAttr("type", "string", false),
			"config": hclspec.NewAttr("config", "list(map(string))", false),
		})),
		"mac_address": hclspec.NewAttr("mac_address", "string", false),
		"mounts": hclspec.NewBlockList("mounts", hclspec.NewObject(map[string]*hclspec.Spec{
			"type": hclspec.NewDefault(
				hclspec.NewAttr("type", "string", false),
				hclspec.NewLiteral("\"volume\""),
			),
			"target":   hclspec.NewAttr("target", "string", false),
			"source":   hclspec.NewAttr("source", "string", false),
			"readonly": hclspec.NewAttr("readonly", "bool", false),
			"bind_options": hclspec.NewBlock("bind_options", false, hclspec.NewObject(map[string]*hclspec.Spec{
				"propagation": hclspec.NewAttr("propagation", "string", false),
			})),
			"tmpfs_options": hclspec.NewBlock("tmpfs_options", false, hclspec.NewObject(map[string]*hclspec.Spec{
				"size": hclspec.NewAttr("size", "number", false),
				"mode": hclspec.NewAttr("mode", "number", false),
			})),
			"volume_options": hclspec.NewBlock("volume_options", false, hclspec.NewObject(map[string]*hclspec.Spec{
				"no_copy": hclspec.NewAttr("no_copy", "bool", false),
				"labels":  hclspec.NewAttr("labels", "list(map(string))", false),
				"driver_config": hclspec.NewBlock("driver_config", false, hclspec.NewObject(map[string]*hclspec.Spec{
					"name":    hclspec.NewAttr("name", "string", false),
					"options": hclspec.NewAttr("options", "list(map(string))", false),
				})),
			})),
		})),
		"network_aliases": hclspec.NewAttr("network_aliases", "list(string)", false),
		"network_mode":    hclspec.NewAttr("network_mode", "string", false),
		"pids_limit":      hclspec.NewAttr("pids_limit", "number", false),
		"pid_mode":        hclspec.NewAttr("pid_mode", "string", false),
		"port_map":        hclspec.NewAttr("port_map", "list(map(number))", false),
		"privileged":      hclspec.NewAttr("privileged", "bool", false),
		"readonly_rootfs": hclspec.NewAttr("readonly_rootfs", "bool", false),
		"security_opt":    hclspec.NewAttr("security_opt", "list(string)", false),
		"shm_size":        hclspec.NewAttr("shm_size", "number", false),
		"storage_opt":     hclspec.NewBlockAttrs("storage_opt", "string", false),
		"sysctl":          hclspec.NewAttr("sysctl", "list(map(string))", false),
		"tty":             hclspec.NewAttr("tty", "bool", false),
		"ulimit":          hclspec.NewAttr("ulimit", "list(map(string))", false),
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
		FSIsolation: drivers.FSIsolationImage,
	}
)

type TaskConfig struct {
	Image             string             `codec:"image"`
	AdvertiseIPv6Addr bool               `codec:"advertise_ipv6_address"`
	Args              []string           `codec:"args"`
	Auth              DockerAuth         `codec:"auth"`
	AuthSoftFail      bool               `codec:"auth_soft_fail"`
	CapAdd            []string           `codec:"cap_add"`
	CapDrop           []string           `codec:"cap_drop"`
	Command           string             `codec:"command"`
	CPUCFSPeriod      int64              `codec:"cpu_cfs_period"`
	CPUHardLimit      bool               `codec:"cpu_hard_limit"`
	Devices           []DockerDevice     `codec:"devices"`
	DNSSearchDomains  []string           `codec:"dns_search_domains"`
	DNSOptions        []string           `codec:"dns_options"`
	DNSServers        []string           `codec:"dns_servers"`
	Entrypoint        []string           `codec:"entrypoint"`
	ExtraHosts        []string           `codec:"extra_hosts"`
	ForcePull         bool               `codec:"force_pull"`
	Hostname          string             `codec:"hostname"`
	Interactive       bool               `codec:"interactive"`
	IPCMode           string             `codec:"ipc_mode"`
	IPv4Address       string             `codec:"ipv4_address"`
	IPv6Address       string             `codec:"ipv6_address"`
	Labels            hclutils.MapStrStr `codec:"labels"`
	LoadImage         string             `codec:"load"`
	Logging           DockerLogging      `codec:"logging"`
	MacAddress        string             `codec:"mac_address"`
	Mounts            []DockerMount      `codec:"mounts"`
	NetworkAliases    []string           `codec:"network_aliases"`
	NetworkMode       string             `codec:"network_mode"`
	PidsLimit         int64              `codec:"pids_limit"`
	PidMode           string             `codec:"pid_mode"`
	PortMap           hclutils.MapStrInt `codec:"port_map"`
	Privileged        bool               `codec:"privileged"`
	ReadonlyRootfs    bool               `codec:"readonly_rootfs"`
	SecurityOpt       []string           `codec:"security_opt"`
	ShmSize           int64              `codec:"shm_size"`
	StorageOpt        map[string]string  `codec:"storage_opt"`
	Sysctl            hclutils.MapStrStr `codec:"sysctl"`
	TTY               bool               `codec:"tty"`
	Ulimit            hclutils.MapStrStr `codec:"ulimit"`
	UTSMode           string             `codec:"uts_mode"`
	UsernsMode        string             `codec:"userns_mode"`
	Volumes           []string           `codec:"volumes"`
	VolumeDriver      string             `codec:"volume_driver"`
	WorkDir           string             `codec:"work_dir"`
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

func (d DockerDevice) toDockerDevice() (docker.Device, error) {
	dd := docker.Device{
		PathOnHost:        d.HostPath,
		PathInContainer:   d.ContainerPath,
		CgroupPermissions: d.CgroupPermissions,
	}

	if d.HostPath == "" {
		return dd, fmt.Errorf("host path must be set in configuration for devices")
	}

	if dd.CgroupPermissions == "" {
		dd.CgroupPermissions = "rwm"
	}

	if !validateCgroupPermission(dd.CgroupPermissions) {
		return dd, fmt.Errorf("invalid cgroup permission string: %q", dd.CgroupPermissions)
	}

	return dd, nil
}

type DockerLogging struct {
	Type   string             `codec:"type"`
	Config hclutils.MapStrStr `codec:"config"`
}

type DockerMount struct {
	Type          string              `codec:"type"`
	Target        string              `codec:"target"`
	Source        string              `codec:"source"`
	ReadOnly      bool                `codec:"readonly"`
	BindOptions   DockerBindOptions   `codec:"bind_options"`
	VolumeOptions DockerVolumeOptions `codec:"volume_options"`
	TmpfsOptions  DockerTmpfsOptions  `codec:"tmpfs_options"`
}

func (m DockerMount) toDockerHostMount() (docker.HostMount, error) {
	if m.Type == "" {
		// for backward compatbility, as type is optional
		m.Type = "volume"
	}

	hm := docker.HostMount{
		Target:   m.Target,
		Source:   m.Source,
		Type:     m.Type,
		ReadOnly: m.ReadOnly,
	}

	switch m.Type {
	case "volume":
		vo := m.VolumeOptions
		hm.VolumeOptions = &docker.VolumeOptions{
			NoCopy: vo.NoCopy,
			Labels: vo.Labels,
			DriverConfig: docker.VolumeDriverConfig{
				Name:    vo.DriverConfig.Name,
				Options: vo.DriverConfig.Options,
			},
		}
	case "bind":
		hm.BindOptions = &docker.BindOptions{
			Propagation: m.BindOptions.Propagation,
		}
	case "tmpfs":
		if m.Source != "" {
			return hm, fmt.Errorf(`invalid source, must be "" for tmpfs`)
		}
		hm.TempfsOptions = &docker.TempfsOptions{
			SizeBytes: m.TmpfsOptions.SizeBytes,
			Mode:      m.TmpfsOptions.Mode,
		}
	default:
		return hm, fmt.Errorf(`invalid mount type, must be "bind", "volume", "tmpfs": %q`, m.Type)
	}

	return hm, nil
}

type DockerVolumeOptions struct {
	NoCopy       bool                     `codec:"no_copy"`
	Labels       hclutils.MapStrStr       `codec:"labels"`
	DriverConfig DockerVolumeDriverConfig `codec:"driver_config"`
}

type DockerBindOptions struct {
	Propagation string `codec:"propagation"`
}

type DockerTmpfsOptions struct {
	SizeBytes int64 `codec:"size"`
	Mode      int   `codec:"mode"`
}

// DockerVolumeDriverConfig holds a map of volume driver specific options
type DockerVolumeDriverConfig struct {
	Name    string             `codec:"name"`
	Options hclutils.MapStrStr `codec:"options"`
}

type DriverConfig struct {
	Endpoint        string       `codec:"endpoint"`
	Auth            AuthConfig   `codec:"auth"`
	TLS             TLSConfig    `codec:"tls"`
	GC              GCConfig     `codec:"gc"`
	Volumes         VolumeConfig `codec:"volumes"`
	AllowPrivileged bool         `codec:"allow_privileged"`
	AllowCaps       []string     `codec:"allow_caps"`
	GPURuntimeName  string       `codec:"nvidia_runtime"`
}

type AuthConfig struct {
	Config string `codec:"config"`
	Helper string `codec:"helper"`
}

type TLSConfig struct {
	Cert string `codec:"cert"`
	Key  string `codec:"key"`
	CA   string `codec:"ca"`
}

type GCConfig struct {
	Image              bool          `codec:"image"`
	ImageDelay         string        `codec:"image_delay"`
	imageDelayDuration time.Duration `codec:"-"`
	Container          bool          `codec:"container"`
}

type VolumeConfig struct {
	Enabled      bool   `codec:"enabled"`
	SelinuxLabel string `codec:"selinuxlabel"`
}

func (d *Driver) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

func (d *Driver) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

func (d *Driver) SetConfig(c *base.Config) error {
	var config DriverConfig
	if len(c.PluginConfig) != 0 {
		if err := base.MsgPackDecode(c.PluginConfig, &config); err != nil {
			return err
		}
	}

	d.config = &config
	if len(d.config.GC.ImageDelay) > 0 {
		dur, err := time.ParseDuration(d.config.GC.ImageDelay)
		if err != nil {
			return fmt.Errorf("failed to parse 'image_delay' duration: %v", err)
		}
		d.config.GC.imageDelayDuration = dur
	}

	if c.AgentConfig != nil {
		d.clientConfig = c.AgentConfig.Driver
	}

	dockerClient, _, err := d.dockerClients()
	if err != nil {
		return fmt.Errorf("failed to get docker client: %v", err)
	}
	coordinatorConfig := &dockerCoordinatorConfig{
		client:      dockerClient,
		cleanup:     d.config.GC.Image,
		logger:      d.logger,
		removeDelay: d.config.GC.imageDelayDuration,
	}

	d.coordinator = newDockerCoordinator(coordinatorConfig)

	return nil
}

func (d *Driver) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

func (d *Driver) Capabilities() (*drivers.Capabilities, error) {
	return capabilities, nil
}
