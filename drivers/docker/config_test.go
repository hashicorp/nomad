// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestConfig_ParseHCL(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name string

		input    string
		expected *TaskConfig
	}{
		{
			"basic image",
			`config {
				image = "redis:7"
			}`,
			&TaskConfig{
				Image:            "redis:7",
				Devices:          []DockerDevice{},
				Mounts:           []DockerMount{},
				MountsList:       []DockerMount{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
		},
	}

	parser := hclutils.NewConfigParser(taskConfigSpec)
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			var tc *TaskConfig

			parser.ParseHCL(t, c.input, &tc)

			require.EqualValues(t, c.expected, tc)

		})
	}
}

func TestConfig_ParseJSON(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		input    string
		expected TaskConfig
	}{
		{
			name:  "nil values for blocks are safe",
			input: `{"Config": {"image": "bash:3", "mounts": null}}`,
			expected: TaskConfig{
				Image:            "bash:3",
				Mounts:           []DockerMount{},
				MountsList:       []DockerMount{},
				Devices:          []DockerDevice{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
		},
		{
			name:  "nil values for 'volumes' field are safe",
			input: `{"Config": {"image": "bash:3", "volumes": null}}`,
			expected: TaskConfig{
				Image:            "bash:3",
				Mounts:           []DockerMount{},
				MountsList:       []DockerMount{},
				Devices:          []DockerDevice{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
		},
		{
			name:  "nil values for 'args' field are safe",
			input: `{"Config": {"image": "bash:3", "args": null}}`,
			expected: TaskConfig{
				Image:            "bash:3",
				Mounts:           []DockerMount{},
				MountsList:       []DockerMount{},
				Devices:          []DockerDevice{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
		},
		{
			name:  "nil values for string fields are safe",
			input: `{"Config": {"image": "bash:3", "command": null}}`,
			expected: TaskConfig{
				Image:            "bash:3",
				Mounts:           []DockerMount{},
				MountsList:       []DockerMount{},
				Devices:          []DockerDevice{},
				CPUCFSPeriod:     100000,
				ImagePullTimeout: "5m",
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			var tc TaskConfig
			hclutils.NewConfigParser(taskConfigSpec).ParseJson(t, c.input, &tc)

			require.Equal(t, c.expected, tc)
		})
	}
}

func TestConfig_PortMap_Deserialization(t *testing.T) {
	ci.Parallel(t)

	parser := hclutils.NewConfigParser(taskConfigSpec)

	expectedMap := map[string]int{
		"ssh":   25,
		"http":  80,
		"https": 443,
	}

	t.Run("parsing hcl block case", func(t *testing.T) {
		validHCL := `
config {
  image = "redis"
  port_map {
    ssh   = 25
    http  = 80
    https = 443
  }
}`

		var tc *TaskConfig
		parser.ParseHCL(t, validHCL, &tc)

		require.EqualValues(t, expectedMap, tc.PortMap)
	})

	t.Run("parsing hcl assignment case", func(t *testing.T) {
		validHCL := `
config {
  image = "redis"
  port_map = {
    ssh   = 25
    http  = 80
    https = 443
  }
}`

		var tc *TaskConfig
		parser.ParseHCL(t, validHCL, &tc)

		require.EqualValues(t, expectedMap, tc.PortMap)
	})

	validJsons := []struct {
		name string
		json string
	}{
		{
			"single map in an array",
			`{"Config": {"image": "redis", "port_map": [{"ssh": 25, "http": 80, "https": 443}]}}`,
		},
		{
			"array of single map entries",
			`{"Config": {"image": "redis", "port_map": [{"ssh": 25}, {"http": 80}, {"https": 443}]}}`,
		},
		{
			"array of maps",
			`{"Config": {"image": "redis", "port_map": [{"ssh": 25, "http": 80}, {"https": 443}]}}`,
		},
	}

	for _, c := range validJsons {
		t.Run("json:"+c.name, func(t *testing.T) {
			var tc *TaskConfig
			parser.ParseJson(t, c.json, &tc)

			require.EqualValues(t, expectedMap, tc.PortMap)
		})
	}

}

func TestConfig_ParseAllHCL(t *testing.T) {
	ci.Parallel(t)

	cfgStr := `
config {
  image = "redis:7"
  image_pull_timeout = "15m"
  advertise_ipv6_address = true
  args = ["command_arg1", "command_arg2"]
  auth {
    username = "myusername"
    password = "mypassword"
    email = "myemail@example.com"
    server_address = "https://example.com"
  }

  auth_soft_fail = true
  cap_add = ["CAP_SYS_NICE"]
  cap_drop = ["CAP_SYS_ADMIN", "CAP_SYS_TIME"]
  command = "/bin/bash"
  cpu_hard_limit = true
  cpu_cfs_period = 20
  devices = [
    {"host_path"="/dev/null", "container_path"="/tmp/container-null", cgroup_permissions="rwm"},
    {"host_path"="/dev/random", "container_path"="/tmp/container-random"},
    {"host_path"="/dev/bus/usb"},
  ]
  dns_search_domains = ["sub.example.com", "sub2.example.com"]
  dns_options = ["debug", "attempts:10"]
  dns_servers = ["8.8.8.8", "1.1.1.1"]
  entrypoint = ["/bin/bash", "-c"]
  extra_hosts = ["127.0.0.1  localhost.example.com"]
  force_pull = true
  group_add = ["group1", "group2"]
  healthchecks {
    disable = true
  }
  hostname = "self.example.com"
  interactive = true
  ipc_mode = "host"
  ipv4_address = "10.0.2.1"
  ipv6_address = "2601:184:407f:b37c:d834:412e:1f86:7699"
  labels {
    owner = "hashicorp-nomad"
    key = "val"
  }
  load = "/tmp/image.tar.gz"
  logging {
    driver = "json-file-driver"
    type   = "json-file"
    config {
      "max-file" = "3"
      "max-size" = "10m"
    }
  }
  mac_address = "02:42:ac:11:00:02"
  memory_hard_limit = 512

  mount {
    type = "bind"
    target ="/mount-bind-target"
    source = "/bind-source-mount"
    readonly = true
    bind_options {
      propagation = "rshared"
    }
  }

  mount {
    type = "tmpfs"
    target ="/mount-tmpfs-target"
    readonly = true
    tmpfs_options {
      size = 30000
      mode = 0777
    }
  }

  mounts = [
    {
      type = "bind"
      target = "/bind-target",
      source = "/bind-source"
      readonly = true
      bind_options {
        propagation = "rshared"
      }
    },
    {
      type = "tmpfs"
      target = "/tmpfs-target",
      readonly = true
      tmpfs_options {
        size = 30000
        mode = 0777
      }
    },
    {
      type = "volume"
      target = "/volume-target"
      source = "/volume-source"
      readonly = true
      volume_options {
        no_copy = true
        labels {
          label_key = "label_value"
	}
        driver_config {
          name = "nfs"
          options {
            option_key = "option_value"
          }
        }
      }
    },
  ]
  network_aliases = ["redis"]
  network_mode = "host"
  pids_limit = 2000
	pid_mode = "host"
	ports = ["http", "https"]
  port_map {
    http = 80
    redis = 6379
  }
  privileged = true
  readonly_rootfs = true
  runtime = "runc"
  security_opt = [
    "credentialspec=file://gmsaUser.json"
  ],
  shm_size = 30000
  storage_opt {
    dm.thinpooldev = "dev/mapper/thin-pool"
    dm.use_deferred_deletion = "true"
    dm.use_deferred_removal = "true"

  }
  sysctl {
    net.core.somaxconn = "16384"
  }
  tty = true
  ulimit {
    nproc = "4242"
    nofile = "2048:4096"
  }
  uts_mode = "host"
  userns_mode = "host"
  volumes = [
    "/host-path:/container-path:rw",
  ]
  volume_driver = "host"
  work_dir = "/tmp/workdir"
}`

	expected := &TaskConfig{
		Image:             "redis:7",
		ImagePullTimeout:  "15m",
		AdvertiseIPv6Addr: true,
		Args:              []string{"command_arg1", "command_arg2"},
		Auth: DockerAuth{
			Username:   "myusername",
			Password:   "mypassword",
			Email:      "myemail@example.com",
			ServerAddr: "https://example.com",
		},
		AuthSoftFail: true,
		CapAdd:       []string{"CAP_SYS_NICE"},
		CapDrop:      []string{"CAP_SYS_ADMIN", "CAP_SYS_TIME"},
		Command:      "/bin/bash",
		CPUHardLimit: true,
		CPUCFSPeriod: 20,
		Devices: []DockerDevice{
			{
				HostPath:          "/dev/null",
				ContainerPath:     "/tmp/container-null",
				CgroupPermissions: "rwm",
			},
			{
				HostPath:          "/dev/random",
				ContainerPath:     "/tmp/container-random",
				CgroupPermissions: "",
			},
			{
				HostPath:          "/dev/bus/usb",
				ContainerPath:     "",
				CgroupPermissions: "",
			},
		},
		DNSSearchDomains: []string{"sub.example.com", "sub2.example.com"},
		DNSOptions:       []string{"debug", "attempts:10"},
		DNSServers:       []string{"8.8.8.8", "1.1.1.1"},
		Entrypoint:       []string{"/bin/bash", "-c"},
		ExtraHosts:       []string{"127.0.0.1  localhost.example.com"},
		ForcePull:        true,
		GroupAdd:         []string{"group1", "group2"},
		Healthchecks:     DockerHealthchecks{Disable: true},
		Hostname:         "self.example.com",
		Interactive:      true,
		IPCMode:          "host",
		IPv4Address:      "10.0.2.1",
		IPv6Address:      "2601:184:407f:b37c:d834:412e:1f86:7699",
		Labels: map[string]string{
			"owner": "hashicorp-nomad",
			"key":   "val",
		},
		LoadImage: "/tmp/image.tar.gz",
		Logging: DockerLogging{
			Driver: "json-file-driver",
			Type:   "json-file",
			Config: map[string]string{
				"max-file": "3",
				"max-size": "10m",
			}},
		MacAddress:      "02:42:ac:11:00:02",
		MemoryHardLimit: 512,
		Mounts: []DockerMount{
			{
				Type:     "bind",
				Target:   "/mount-bind-target",
				Source:   "/bind-source-mount",
				ReadOnly: true,
				BindOptions: DockerBindOptions{
					Propagation: "rshared",
				},
			},
			{
				Type:     "tmpfs",
				Target:   "/mount-tmpfs-target",
				Source:   "",
				ReadOnly: true,
				TmpfsOptions: DockerTmpfsOptions{
					SizeBytes: 30000,
					Mode:      511,
				},
			},
		},
		MountsList: []DockerMount{
			{
				Type:     "bind",
				Target:   "/bind-target",
				Source:   "/bind-source",
				ReadOnly: true,
				BindOptions: DockerBindOptions{
					Propagation: "rshared",
				},
			},
			{
				Type:     "tmpfs",
				Target:   "/tmpfs-target",
				Source:   "",
				ReadOnly: true,
				TmpfsOptions: DockerTmpfsOptions{
					SizeBytes: 30000,
					Mode:      511,
				},
			},
			{
				Type:     "volume",
				Target:   "/volume-target",
				Source:   "/volume-source",
				ReadOnly: true,
				VolumeOptions: DockerVolumeOptions{
					NoCopy: true,
					Labels: map[string]string{
						"label_key": "label_value",
					},
					DriverConfig: DockerVolumeDriverConfig{
						Name: "nfs",
						Options: map[string]string{
							"option_key": "option_value",
						},
					},
				},
			},
		},
		NetworkAliases: []string{"redis"},
		NetworkMode:    "host",
		PidsLimit:      2000,
		PidMode:        "host",
		Ports:          []string{"http", "https"},
		PortMap: map[string]int{
			"http":  80,
			"redis": 6379,
		},
		Privileged:     true,
		ReadonlyRootfs: true,
		Runtime:        "runc",
		SecurityOpt: []string{
			"credentialspec=file://gmsaUser.json",
		},
		ShmSize: 30000,
		StorageOpt: map[string]string{
			"dm.thinpooldev":           "dev/mapper/thin-pool",
			"dm.use_deferred_deletion": "true",
			"dm.use_deferred_removal":  "true",
		},
		Sysctl: map[string]string{
			"net.core.somaxconn": "16384",
		},
		TTY: true,
		Ulimit: map[string]string{
			"nofile": "2048:4096",
			"nproc":  "4242",
		},
		UTSMode:    "host",
		UsernsMode: "host",
		Volumes: []string{
			"/host-path:/container-path:rw",
		},
		VolumeDriver: "host",
		WorkDir:      "/tmp/workdir",
	}

	var tc *TaskConfig
	hclutils.NewConfigParser(taskConfigSpec).ParseHCL(t, cfgStr, &tc)

	require.EqualValues(t, expected, tc)
}

// TestConfig_DriverConfig_GC asserts that gc is parsed
// and populated with defaults as expected
func TestConfig_DriverConfig_GC(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		config   string
		expected GCConfig
	}{
		{
			name:   "pure default",
			config: `{}`,
			expected: GCConfig{
				Image: true, ImageDelay: "3m", Container: true,
				DanglingContainers: ContainerGCConfig{
					Enabled: true, PeriodStr: "5m", CreationGraceStr: "5m"},
			},
		},
		{
			name:   "partial gc",
			config: `{ gc { } }`,
			expected: GCConfig{
				Image: true, ImageDelay: "3m", Container: true,
				DanglingContainers: ContainerGCConfig{
					Enabled: true, PeriodStr: "5m", CreationGraceStr: "5m"},
			},
		},
		{
			name:   "partial gc",
			config: `{ gc { dangling_containers { } } }`,
			expected: GCConfig{
				Image: true, ImageDelay: "3m", Container: true,
				DanglingContainers: ContainerGCConfig{
					Enabled: true, PeriodStr: "5m", CreationGraceStr: "5m"},
			},
		},
		{
			name:   "partial image",
			config: `{ gc { image = false } }`,
			expected: GCConfig{
				Image: false, ImageDelay: "3m", Container: true,
				DanglingContainers: ContainerGCConfig{
					Enabled: true, PeriodStr: "5m", CreationGraceStr: "5m"},
			},
		},
		{
			name:   "partial image_delay",
			config: `{ gc { image_delay = "1d"} }`,
			expected: GCConfig{
				Image: true, ImageDelay: "1d", Container: true,
				DanglingContainers: ContainerGCConfig{
					Enabled: true, PeriodStr: "5m", CreationGraceStr: "5m"},
			},
		},
		{
			name:   "partial dangling_containers",
			config: `{ gc { dangling_containers { enabled = false } } }`,
			expected: GCConfig{
				Image: true, ImageDelay: "3m", Container: true,
				DanglingContainers: ContainerGCConfig{
					Enabled: false, PeriodStr: "5m", CreationGraceStr: "5m"},
			},
		},
		{
			name:   "incomplete dangling_containers 2",
			config: `{ gc { dangling_containers { period = "10m" } } }`,
			expected: GCConfig{
				Image: true, ImageDelay: "3m", Container: true,
				DanglingContainers: ContainerGCConfig{
					Enabled: true, PeriodStr: "10m", CreationGraceStr: "5m"},
			},
		},
		{
			name: "full default",
			config: `{ gc {
			image = false
			image_delay = "5m"
			container = false
			dangling_containers {
			     enabled = false
			     dry_run = true
			     period = "10m"
			     creation_grace = "20m"
			}}}`,
			expected: GCConfig{
				Image:      false,
				ImageDelay: "5m",
				Container:  false,
				DanglingContainers: ContainerGCConfig{
					Enabled:          false,
					DryRun:           true,
					PeriodStr:        "10m",
					CreationGraceStr: "20m",
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var tc DriverConfig
			hclutils.NewConfigParser(configSpec).ParseHCL(t, "config "+c.config, &tc)
			require.EqualValues(t, c.expected, tc.GC)

		})
	}
}

func TestConfig_Capabilities(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		config   string
		expected *drivers.Capabilities
	}{
		{
			name:   "pure default",
			config: `{}`,
			expected: &drivers.Capabilities{
				SendSignals:          true,
				Exec:                 true,
				FSIsolation:          "image",
				NetIsolationModes:    []drivers.NetIsolationMode{"host", "group", "task"},
				MustInitiateNetwork:  true,
				MountConfigs:         0,
				RemoteTasks:          false,
				DisableLogCollection: false,
			},
		},
		{
			name:   "disabled",
			config: `{ disable_log_collection = true }`,
			expected: &drivers.Capabilities{
				SendSignals:          true,
				Exec:                 true,
				FSIsolation:          "image",
				NetIsolationModes:    []drivers.NetIsolationMode{"host", "group", "task"},
				MustInitiateNetwork:  true,
				MountConfigs:         0,
				RemoteTasks:          false,
				DisableLogCollection: true,
			},
		},
		{
			name:   "enabled explicitly",
			config: `{ disable_log_collection = false }`,
			expected: &drivers.Capabilities{
				SendSignals:          true,
				Exec:                 true,
				FSIsolation:          "image",
				NetIsolationModes:    []drivers.NetIsolationMode{"host", "group", "task"},
				MustInitiateNetwork:  true,
				MountConfigs:         0,
				RemoteTasks:          false,
				DisableLogCollection: false,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var tc DriverConfig
			hclutils.NewConfigParser(configSpec).ParseHCL(t, "config "+c.config, &tc)

			d := &Driver{config: &tc}
			caps, err := d.Capabilities()
			must.NoError(t, err)
			must.Eq(t, c.expected, caps)
		})
	}
}

func TestConfig_DriverConfig_InfraImagePullTimeout(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		config   string
		expected string
	}{
		{
			name:     "default",
			config:   `{}`,
			expected: "5m",
		},
		{
			name:     "set explicitly",
			config:   `{ infra_image_pull_timeout = "1m" }`,
			expected: "1m",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var tc DriverConfig
			hclutils.NewConfigParser(configSpec).ParseHCL(t, "config "+c.config, &tc)
			require.Equal(t, c.expected, tc.InfraImagePullTimeout)
		})
	}
}

func TestConfig_DriverConfig_PullActivityTimeout(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		config   string
		expected string
	}{
		{
			name:     "default",
			config:   `{}`,
			expected: "2m",
		},
		{
			name:     "set explicitly",
			config:   `{ pull_activity_timeout = "5m" }`,
			expected: "5m",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var tc DriverConfig
			hclutils.NewConfigParser(configSpec).ParseHCL(t, "config "+c.config, &tc)
			require.Equal(t, c.expected, tc.PullActivityTimeout)
		})
	}
}

func TestConfig_DriverConfig_AllowRuntimes(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		config   string
		expected map[string]struct{}
	}{
		{
			name:     "pure default",
			config:   `{}`,
			expected: map[string]struct{}{"runc": {}, "nvidia": {}},
		},
		{
			name:     "custom",
			config:   `{ allow_runtimes = ["runc", "firecracker"]}`,
			expected: map[string]struct{}{"runc": {}, "firecracker": {}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var tc map[string]interface{}
			hclutils.NewConfigParser(configSpec).ParseHCL(t, "config "+c.config, &tc)

			dh := dockerDriverHarness(t, tc)
			d := dh.Impl().(*Driver)
			require.Equal(t, c.expected, d.config.allowRuntimes)
		})
	}
}
