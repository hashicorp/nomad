// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/drivers/shared/capabilities"
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
	// COMPAT(1.0) uses inclusive language. whitelist is used for backward compatibility.
	if v, ok := opts["docker.caps.allowlist"]; ok {
		conf["allow_caps"] = strings.Split(v, ",")
	} else if v, ok := opts["docker.caps.whitelist"]; ok {
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
	// PluginID is the docker plugin metadata registered in the plugin catalog.
	PluginID = loader.PluginID{
		Name:       pluginName,
		PluginType: base.PluginTypeDriver,
	}

	// PluginConfig is the docker config factory function registered in the plugin catalog.
	PluginConfig = &loader.InternalPluginConfig{
		Config:  map[string]interface{}{},
		Factory: func(ctx context.Context, l hclog.Logger) interface{} { return NewDockerDriver(ctx, nil, l) },
	}

	// pluginInfo is the response returned for the PluginInfo RPC.
	pluginInfo = &base.PluginInfoResponse{
		Type:              base.PluginTypeDriver,
		PluginApiVersions: []string{drivers.ApiVersion010},
		PluginVersion:     "0.1.0",
		Name:              pluginName,
	}

	danglingContainersBlock = hclspec.NewObject(map[string]*hclspec.Spec{
		"enabled": hclspec.NewDefault(
			hclspec.NewAttr("enabled", "bool", false),
			hclspec.NewLiteral(`true`),
		),
		"period": hclspec.NewDefault(
			hclspec.NewAttr("period", "string", false),
			hclspec.NewLiteral(`"5m"`),
		),
		"creation_grace": hclspec.NewDefault(
			hclspec.NewAttr("creation_grace", "string", false),
			hclspec.NewLiteral(`"5m"`),
		),
		"dry_run": hclspec.NewDefault(
			hclspec.NewAttr("dry_run", "bool", false),
			hclspec.NewLiteral(`false`),
		),
	})

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

		// extra docker labels, globs supported
		"extra_labels": hclspec.NewAttr("extra_labels", "list(string)", false),

		// logging options
		"logging": hclspec.NewDefault(hclspec.NewBlock("logging", false, hclspec.NewObject(map[string]*hclspec.Spec{
			"type":   hclspec.NewAttr("type", "string", false),
			"config": hclspec.NewBlockAttrs("config", "string", false),
		})), hclspec.NewLiteral(`{
			type = "json-file"
			config = {
				max-file = "2"
				max-size = "2m"
			}
		}`)),

		// garbage collection options
		// default needed for both if the gc {...} block is not set and
		// if the default fields are missing
		"gc": hclspec.NewDefault(hclspec.NewBlock("gc", false, hclspec.NewObject(map[string]*hclspec.Spec{
			"image": hclspec.NewDefault(
				hclspec.NewAttr("image", "bool", false),
				hclspec.NewLiteral("true"),
			),
			"image_delay": hclspec.NewDefault(
				hclspec.NewAttr("image_delay", "string", false),
				hclspec.NewLiteral("\"3m\""),
			),
			"container": hclspec.NewDefault(
				hclspec.NewAttr("container", "bool", false),
				hclspec.NewLiteral("true"),
			),
			"dangling_containers": hclspec.NewDefault(
				hclspec.NewBlock("dangling_containers", false, danglingContainersBlock),
				hclspec.NewLiteral(`{
					enabled = true
					period = "5m"
					creation_grace = "5m"
				}`),
			),
		})), hclspec.NewLiteral(`{
			image = true
			image_delay = "3m"
			container = true
			dangling_containers = {
				enabled = true
				period = "5m"
				creation_grace = "5m"
			}
		}`)),

		// docker volume options
		// defaulted needed for both if the volumes {...} block is not set and
		// if the default fields are missing
		"volumes": hclspec.NewDefault(hclspec.NewBlock("volumes", false, hclspec.NewObject(map[string]*hclspec.Spec{
			"enabled":      hclspec.NewAttr("enabled", "bool", false),
			"selinuxlabel": hclspec.NewAttr("selinuxlabel", "string", false),
		})), hclspec.NewLiteral("{ enabled = false }")),
		"allow_privileged": hclspec.NewAttr("allow_privileged", "bool", false),
		"allow_caps": hclspec.NewDefault(
			hclspec.NewAttr("allow_caps", "list(string)", false),
			hclspec.NewLiteral(capabilities.HCLSpecLiteral),
		),
		"nvidia_runtime": hclspec.NewDefault(
			hclspec.NewAttr("nvidia_runtime", "string", false),
			hclspec.NewLiteral(`"nvidia"`),
		),
		// list of docker runtimes allowed to be used
		"allow_runtimes": hclspec.NewDefault(
			hclspec.NewAttr("allow_runtimes", "list(string)", false),
			hclspec.NewLiteral(`["runc", "nvidia"]`),
		),
		// image to use when creating a network namespace parent container
		"infra_image": hclspec.NewDefault(
			hclspec.NewAttr("infra_image", "string", false),
			hclspec.NewLiteral(fmt.Sprintf(
				`"gcr.io/google_containers/pause-%s:3.1"`,
				runtime.GOARCH,
			)),
		),
		// timeout to use when pulling the infra image.
		"infra_image_pull_timeout": hclspec.NewDefault(
			hclspec.NewAttr("infra_image_pull_timeout", "string", false),
			hclspec.NewLiteral(`"5m"`),
		),

		// the duration that the driver will wait for activity from the Docker engine during an image pull
		// before canceling the request
		"pull_activity_timeout": hclspec.NewDefault(
			hclspec.NewAttr("pull_activity_timeout", "string", false),
			hclspec.NewLiteral(`"2m"`),
		),
		"pids_limit": hclspec.NewAttr("pids_limit", "number", false),
		// disable_log_collection indicates whether docker driver should collect logs of docker
		// task containers.  If true, nomad doesn't start docker_logger/logmon processes
		"disable_log_collection": hclspec.NewAttr("disable_log_collection", "bool", false),
	})

	// mountBodySpec is the hcl specification for the `mount` block
	mountBodySpec = hclspec.NewObject(map[string]*hclspec.Spec{
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
	})

	// healthchecksBodySpec is the hcl specification for the `healthchecks` block
	healthchecksBodySpec = hclspec.NewObject(map[string]*hclspec.Spec{
		"disable": hclspec.NewAttr("disable", "bool", false),
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
		"cpuset_cpus":    hclspec.NewAttr("cpuset_cpus", "string", false),
		"cpu_hard_limit": hclspec.NewAttr("cpu_hard_limit", "bool", false),
		"cpu_cfs_period": hclspec.NewDefault(
			hclspec.NewAttr("cpu_cfs_period", "number", false),
			hclspec.NewLiteral(`100000`),
		),
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
		"group_add":          hclspec.NewAttr("group_add", "list(string)", false),
		"healthchecks":       hclspec.NewBlock("healthchecks", false, healthchecksBodySpec),
		"hostname":           hclspec.NewAttr("hostname", "string", false),
		"init":               hclspec.NewAttr("init", "bool", false),
		"interactive":        hclspec.NewAttr("interactive", "bool", false),
		"ipc_mode":           hclspec.NewAttr("ipc_mode", "string", false),
		"ipv4_address":       hclspec.NewAttr("ipv4_address", "string", false),
		"ipv6_address":       hclspec.NewAttr("ipv6_address", "string", false),
		"isolation":          hclspec.NewAttr("isolation", "string", false),
		"labels":             hclspec.NewAttr("labels", "list(map(string))", false),
		"load":               hclspec.NewAttr("load", "string", false),
		"logging": hclspec.NewBlock("logging", false, hclspec.NewObject(map[string]*hclspec.Spec{
			"type":   hclspec.NewAttr("type", "string", false),
			"driver": hclspec.NewAttr("driver", "string", false),
			"config": hclspec.NewAttr("config", "list(map(string))", false),
		})),
		"mac_address":       hclspec.NewAttr("mac_address", "string", false),
		"memory_hard_limit": hclspec.NewAttr("memory_hard_limit", "number", false),
		// mount and mounts are effectively aliases, but `mounts` is meant for pre-1.0
		// assignment syntax `mounts = [{type="..." ..."}]` while
		// `mount` is 1.0 repeated block syntax `mount { type = "..." }`
		"mount":           hclspec.NewBlockList("mount", mountBodySpec),
		"mounts":          hclspec.NewBlockList("mounts", mountBodySpec),
		"network_aliases": hclspec.NewAttr("network_aliases", "list(string)", false),
		"network_mode":    hclspec.NewAttr("network_mode", "string", false),
		"runtime":         hclspec.NewAttr("runtime", "string", false),
		"pids_limit":      hclspec.NewAttr("pids_limit", "number", false),
		"pid_mode":        hclspec.NewAttr("pid_mode", "string", false),
		"ports":           hclspec.NewAttr("ports", "list(string)", false),
		"port_map":        hclspec.NewAttr("port_map", "list(map(number))", false),
		"privileged":      hclspec.NewAttr("privileged", "bool", false),
		"image_pull_timeout": hclspec.NewDefault(
			hclspec.NewAttr("image_pull_timeout", "string", false),
			hclspec.NewLiteral(`"5m"`),
		),
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

	// driverCapabilities represents the RPC response for what features are
	// implemented by the docker task driver
	driverCapabilities = &drivers.Capabilities{
		SendSignals: true,
		Exec:        true,
		FSIsolation: drivers.FSIsolationImage,
		NetIsolationModes: []drivers.NetIsolationMode{
			drivers.NetIsolationModeHost,
			drivers.NetIsolationModeGroup,
			drivers.NetIsolationModeTask,
		},
		MustInitiateNetwork: true,
		MountConfigs:        drivers.MountConfigSupportAll,
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
	CPUSetCPUs        string             `codec:"cpuset_cpus"`
	Devices           []DockerDevice     `codec:"devices"`
	DNSSearchDomains  []string           `codec:"dns_search_domains"`
	DNSOptions        []string           `codec:"dns_options"`
	DNSServers        []string           `codec:"dns_servers"`
	Entrypoint        []string           `codec:"entrypoint"`
	ExtraHosts        []string           `codec:"extra_hosts"`
	ForcePull         bool               `codec:"force_pull"`
	GroupAdd          []string           `codec:"group_add"`
	Healthchecks      DockerHealthchecks `codec:"healthchecks"`
	Hostname          string             `codec:"hostname"`
	Init              bool               `codec:"init"`
	Interactive       bool               `codec:"interactive"`
	IPCMode           string             `codec:"ipc_mode"`
	IPv4Address       string             `codec:"ipv4_address"`
	IPv6Address       string             `codec:"ipv6_address"`
	Isolation         string             `codec:"isolation"`
	Labels            hclutils.MapStrStr `codec:"labels"`
	LoadImage         string             `codec:"load"`
	Logging           DockerLogging      `codec:"logging"`
	MacAddress        string             `codec:"mac_address"`
	MemoryHardLimit   int64              `codec:"memory_hard_limit"`
	Mounts            []DockerMount      `codec:"mount"`
	NetworkAliases    []string           `codec:"network_aliases"`
	NetworkMode       string             `codec:"network_mode"`
	Runtime           string             `codec:"runtime"`
	PidsLimit         int64              `codec:"pids_limit"`
	PidMode           string             `codec:"pid_mode"`
	Ports             []string           `codec:"ports"`
	PortMap           hclutils.MapStrInt `codec:"port_map"`
	Privileged        bool               `codec:"privileged"`
	ImagePullTimeout  string             `codec:"image_pull_timeout"`
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

	// MountsList supports the pre-1.0 mounts array syntax
	MountsList []DockerMount `codec:"mounts"`
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

	// Docker's CLI defaults to HostPath in this case. See #16754
	if dd.PathInContainer == "" {
		dd.PathInContainer = d.HostPath
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
	Driver string             `codec:"driver"`
	Config hclutils.MapStrStr `codec:"config"`
}

type DockerHealthchecks struct {
	Disable bool `codec:"disable"`
}

func (dh *DockerHealthchecks) Disabled() bool {
	return dh == nil || dh.Disable
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
		// for backward compatibility, as type is optional
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

// ContainerGCConfig controls the behavior of the GC reconciler to detects
// dangling nomad containers that aren't tracked due to docker/nomad bugs
type ContainerGCConfig struct {
	// Enabled controls whether container reconciler is enabled
	Enabled bool `codec:"enabled"`

	// DryRun indicates that reconciler should log unexpectedly running containers
	// if found without actually killing them
	DryRun bool `codec:"dry_run"`

	// PeriodStr controls the frequency of scanning containers
	PeriodStr string        `codec:"period"`
	period    time.Duration `codec:"-"`

	// CreationGraceStr is the duration allowed for a newly created container
	// to live without being registered as a running task in nomad.
	// A container is treated as leaked if it lived more than grace duration
	// and haven't been registered in tasks.
	CreationGraceStr string        `codec:"creation_grace"`
	CreationGrace    time.Duration `codec:"-"`
}

type DriverConfig struct {
	Endpoint                      string        `codec:"endpoint"`
	Auth                          AuthConfig    `codec:"auth"`
	TLS                           TLSConfig     `codec:"tls"`
	GC                            GCConfig      `codec:"gc"`
	Volumes                       VolumeConfig  `codec:"volumes"`
	AllowPrivileged               bool          `codec:"allow_privileged"`
	AllowCaps                     []string      `codec:"allow_caps"`
	GPURuntimeName                string        `codec:"nvidia_runtime"`
	InfraImage                    string        `codec:"infra_image"`
	InfraImagePullTimeout         string        `codec:"infra_image_pull_timeout"`
	infraImagePullTimeoutDuration time.Duration `codec:"-"`
	DisableLogCollection          bool          `codec:"disable_log_collection"`
	PullActivityTimeout           string        `codec:"pull_activity_timeout"`
	PidsLimit                     int64         `codec:"pids_limit"`
	pullActivityTimeoutDuration   time.Duration `codec:"-"`
	ExtraLabels                   []string      `codec:"extra_labels"`
	Logging                       LoggingConfig `codec:"logging"`

	AllowRuntimesList []string            `codec:"allow_runtimes"`
	allowRuntimes     map[string]struct{} `codec:"-"`
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

	DanglingContainers ContainerGCConfig `codec:"dangling_containers"`
}

type VolumeConfig struct {
	Enabled      bool   `codec:"enabled"`
	SelinuxLabel string `codec:"selinuxlabel"`
}

type LoggingConfig struct {
	Type   string            `codec:"type"`
	Config map[string]string `codec:"config"`
}

func (d *Driver) PluginInfo() (*base.PluginInfoResponse, error) {
	return pluginInfo, nil
}

func (d *Driver) ConfigSchema() (*hclspec.Spec, error) {
	return configSpec, nil
}

const danglingContainersCreationGraceMinimum = 1 * time.Minute
const pullActivityTimeoutMinimum = 1 * time.Minute

func (d *Driver) SetConfig(c *base.Config) error {
	var config DriverConfig
	if len(c.PluginConfig) != 0 {
		if err := base.MsgPackDecode(c.PluginConfig, &config); err != nil {
			return err
		}
	}

	d.config = &config
	d.config.InfraImage = strings.TrimPrefix(d.config.InfraImage, "https://")

	if len(d.config.GC.ImageDelay) > 0 {
		dur, err := time.ParseDuration(d.config.GC.ImageDelay)
		if err != nil {
			return fmt.Errorf("failed to parse 'image_delay' duration: %v", err)
		}
		d.config.GC.imageDelayDuration = dur
	}

	if len(d.config.GC.DanglingContainers.PeriodStr) > 0 {
		dur, err := time.ParseDuration(d.config.GC.DanglingContainers.PeriodStr)
		if err != nil {
			return fmt.Errorf("failed to parse 'period' duration: %v", err)
		}
		d.config.GC.DanglingContainers.period = dur
	}

	if len(d.config.GC.DanglingContainers.CreationGraceStr) > 0 {
		dur, err := time.ParseDuration(d.config.GC.DanglingContainers.CreationGraceStr)
		if err != nil {
			return fmt.Errorf("failed to parse 'creation_grace' duration: %v", err)
		}
		if dur < danglingContainersCreationGraceMinimum {
			return fmt.Errorf("creation_grace is less than minimum, %v", danglingContainersCreationGraceMinimum)
		}
		d.config.GC.DanglingContainers.CreationGrace = dur
	}

	if len(d.config.PullActivityTimeout) > 0 {
		dur, err := time.ParseDuration(d.config.PullActivityTimeout)
		if err != nil {
			return fmt.Errorf("failed to parse 'pull_activity_timeout' duration: %v", err)
		}
		if dur < pullActivityTimeoutMinimum {
			return fmt.Errorf("pull_activity_timeout is less than minimum, %v", pullActivityTimeoutMinimum)
		}
		d.config.pullActivityTimeoutDuration = dur
	}

	if d.config.InfraImagePullTimeout != "" {
		dur, err := time.ParseDuration(d.config.InfraImagePullTimeout)
		if err != nil {
			return fmt.Errorf("failed to parse 'infra_image_pull_timeout' duration: %v", err)
		}
		d.config.infraImagePullTimeoutDuration = dur
	}

	d.config.allowRuntimes = make(map[string]struct{}, len(d.config.AllowRuntimesList))
	for _, r := range d.config.AllowRuntimesList {
		d.config.allowRuntimes[r] = struct{}{}
	}

	if c.AgentConfig != nil {
		d.clientConfig = c.AgentConfig.Driver
	}

	dockerClient, err := d.getDockerClient()
	if err != nil {
		return fmt.Errorf("failed to get docker client: %v", err)
	}
	coordinatorConfig := &dockerCoordinatorConfig{
		ctx:         d.ctx,
		client:      dockerClient,
		cleanup:     d.config.GC.Image,
		logger:      d.logger,
		removeDelay: d.config.GC.imageDelayDuration,
	}

	d.coordinator = newDockerCoordinator(coordinatorConfig)

	d.danglingReconciler = newReconciler(d)

	go d.recoverPauseContainers(d.ctx)

	return nil
}

func (d *Driver) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

// Capabilities is returned by the Capabilities RPC and indicates what optional
// features this driver supports.
func (d *Driver) Capabilities() (*drivers.Capabilities, error) {
	driverCapabilities.DisableLogCollection = d.config != nil && d.config.DisableLogCollection
	return driverCapabilities, nil
}
