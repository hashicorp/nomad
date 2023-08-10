// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package docker

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	tu "github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerDriver_User(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	task.User = "alice"
	cfg.Command = "/bin/sleep"
	cfg.Args = []string{"10000"}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	_, _, err := d.StartTask(task)
	if err == nil {
		d.DestroyTask(task.ID, true)
		t.Fatalf("Should've failed")
	}

	if !strings.Contains(err.Error(), "alice") {
		t.Fatalf("Expected failure string not found, found %q instead", err.Error())
	}
}

func TestDockerDriver_NetworkAliases_Bridge(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	require := require.New(t)

	// Because go-dockerclient doesn't provide api for query network aliases, just check that
	// a container can be created with a 'network_aliases' property

	// Create network, network-scoped alias is supported only for containers in user defined networks
	client := newTestDockerClient(t)
	networkOpts := docker.CreateNetworkOptions{Name: "foobar", Driver: "bridge"}
	network, err := client.CreateNetwork(networkOpts)
	require.NoError(err)
	defer client.RemoveNetwork(network.ID)

	expected := []string{"foobar"}
	taskCfg := newTaskConfig("", busyboxLongRunningCmd)
	taskCfg.NetworkMode = network.Name
	taskCfg.NetworkAliases = expected
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "busybox",
		Resources: basicResources,
	}
	require.NoError(task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	_, _, err = d.StartTask(task)
	require.NoError(err)
	require.NoError(d.WaitUntilStarted(task.ID, 5*time.Second))

	defer d.DestroyTask(task.ID, true)

	dockerDriver, ok := d.Impl().(*Driver)
	require.True(ok)

	handle, ok := dockerDriver.tasks.Get(task.ID)
	require.True(ok)

	_, err = client.InspectContainer(handle.containerID)
	require.NoError(err)
}

func TestDockerDriver_NetworkMode_Host(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)
	expected := "host"

	taskCfg := newTaskConfig("", busyboxLongRunningCmd)
	taskCfg.NetworkMode = expected

	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "busybox-demo",
		Resources: basicResources,
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	_, _, err := d.StartTask(task)
	must.NoError(t, err)

	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	defer d.DestroyTask(task.ID, true)

	dockerDriver, ok := d.Impl().(*Driver)
	must.True(t, ok)

	handle, ok := dockerDriver.tasks.Get(task.ID)
	must.True(t, ok)

	client := newTestDockerClient(t)

	container, err := client.InspectContainer(handle.containerID)
	must.NoError(t, err)

	actual := container.HostConfig.NetworkMode
	must.Eq(t, expected, actual)
}

func TestDockerDriver_CPUCFSPeriod(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	cfg.CPUHardLimit = true
	cfg.CPUCFSPeriod = 1000000
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, _, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()

	waitForExist(t, client, handle.containerID)

	container, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)

	require.Equal(t, cfg.CPUCFSPeriod, container.HostConfig.CPUPeriod)
}

func TestDockerDriver_Sysctl_Ulimit(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	expectedUlimits := map[string]string{
		"nproc":  "4242",
		"nofile": "2048:4096",
	}
	cfg.Sysctl = map[string]string{
		"net.core.somaxconn": "16384",
	}
	cfg.Ulimit = expectedUlimits
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.containerID)
	assert.Nil(t, err, "unexpected error: %v", err)

	want := "16384"
	got := container.HostConfig.Sysctls["net.core.somaxconn"]
	assert.Equal(t, want, got, "Wrong net.core.somaxconn config for docker job. Expect: %s, got: %s", want, got)

	expectedUlimitLen := 2
	actualUlimitLen := len(container.HostConfig.Ulimits)
	assert.Equal(t, want, got, "Wrong number of ulimit configs for docker job. Expect: %d, got: %d", expectedUlimitLen, actualUlimitLen)

	for _, got := range container.HostConfig.Ulimits {
		if expectedStr, ok := expectedUlimits[got.Name]; !ok {
			t.Errorf("%s config unexpected for docker job.", got.Name)
		} else {
			if !strings.Contains(expectedStr, ":") {
				expectedStr = expectedStr + ":" + expectedStr
			}

			splitted := strings.SplitN(expectedStr, ":", 2)
			soft, _ := strconv.Atoi(splitted[0])
			hard, _ := strconv.Atoi(splitted[1])
			assert.Equal(t, int64(soft), got.Soft, "Wrong soft %s ulimit for docker job. Expect: %d, got: %d", got.Name, soft, got.Soft)
			assert.Equal(t, int64(hard), got.Hard, "Wrong hard %s ulimit for docker job. Expect: %d, got: %d", got.Name, hard, got.Hard)

		}
	}
}

func TestDockerDriver_Sysctl_Ulimit_Errors(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	brokenConfigs := []map[string]string{
		{
			"nofile": "",
		},
		{
			"nofile": "abc:1234",
		},
		{
			"nofile": "1234:abc",
		},
	}

	testCases := []struct {
		ulimitConfig map[string]string
		err          error
	}{
		{brokenConfigs[0], fmt.Errorf("Malformed ulimit specification nofile: \"\", cannot be empty")},
		{brokenConfigs[1], fmt.Errorf("Malformed soft ulimit nofile: abc:1234")},
		{brokenConfigs[2], fmt.Errorf("Malformed hard ulimit nofile: 1234:abc")},
	}

	for _, tc := range testCases {
		task, cfg, _ := dockerTask(t)
		cfg.Ulimit = tc.ulimitConfig
		require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

		d := dockerDriverHarness(t, nil)
		cleanup := d.MkAllocDir(task, true)
		t.Cleanup(cleanup)
		copyImage(t, task.TaskDir(), "busybox.tar")

		_, _, err := d.StartTask(task)
		require.NotNil(t, err, "Expected non nil error")
		require.Contains(t, err.Error(), tc.err.Error())
	}
}

// This test does not run on Windows due to stricter path validation in the
// negative case for non existent mount paths. We should write a similar test
// for windows.
func TestDockerDriver_BindMountsHonorVolumesEnabledFlag(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	allocDir := "/tmp/nomad/alloc-dir"

	cases := []struct {
		name            string
		requiresVolumes bool

		volumeDriver string
		volumes      []string

		expectedVolumes []string
	}{
		{
			name:            "basic plugin",
			requiresVolumes: true,
			volumeDriver:    "nfs",
			volumes:         []string{"test-path:/tmp/taskpath"},
			expectedVolumes: []string{"test-path:/tmp/taskpath"},
		},
		{
			name:            "absolute default driver",
			requiresVolumes: true,
			volumeDriver:    "",
			volumes:         []string{"/abs/test-path:/tmp/taskpath"},
			expectedVolumes: []string{"/abs/test-path:/tmp/taskpath"},
		},
		{
			name:            "absolute local driver",
			requiresVolumes: true,
			volumeDriver:    "local",
			volumes:         []string{"/abs/test-path:/tmp/taskpath"},
			expectedVolumes: []string{"/abs/test-path:/tmp/taskpath"},
		},
		{
			name:            "relative default driver",
			requiresVolumes: false,
			volumeDriver:    "",
			volumes:         []string{"test-path:/tmp/taskpath"},
			expectedVolumes: []string{"/tmp/nomad/alloc-dir/demo/test-path:/tmp/taskpath"},
		},
		{
			name:            "named volume local driver",
			requiresVolumes: true,
			volumeDriver:    "local",
			volumes:         []string{"test-path:/tmp/taskpath"},
			expectedVolumes: []string{"test-path:/tmp/taskpath"},
		},
		{
			name:            "relative outside task-dir default driver",
			requiresVolumes: false,
			volumeDriver:    "",
			volumes:         []string{"../test-path:/tmp/taskpath"},
			expectedVolumes: []string{"/tmp/nomad/alloc-dir/test-path:/tmp/taskpath"},
		},
		{
			name:            "relative outside alloc-dir default driver",
			requiresVolumes: true,
			volumeDriver:    "",
			volumes:         []string{"../../test-path:/tmp/taskpath"},
			expectedVolumes: []string{"/tmp/nomad/test-path:/tmp/taskpath"},
		},
		{
			name:            "clean path local driver",
			requiresVolumes: true,
			volumeDriver:    "local",
			volumes:         []string{"/tmp/nomad/../test-path:/tmp/taskpath"},
			expectedVolumes: []string{"/tmp/test-path:/tmp/taskpath"},
		},
	}

	t.Run("with volumes enabled", func(t *testing.T) {
		dh := dockerDriverHarness(t, nil)
		driver := dh.Impl().(*Driver)
		driver.config.Volumes.Enabled = true

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				task, cfg, _ := dockerTask(t)
				cfg.VolumeDriver = c.volumeDriver
				cfg.Volumes = c.volumes

				task.AllocDir = allocDir
				task.Name = "demo"

				require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

				cc, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
				require.NoError(t, err)

				for _, v := range c.expectedVolumes {
					require.Contains(t, cc.HostConfig.Binds, v)
				}
			})
		}
	})

	t.Run("with volumes disabled", func(t *testing.T) {
		dh := dockerDriverHarness(t, nil)
		driver := dh.Impl().(*Driver)
		driver.config.Volumes.Enabled = false

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				task, cfg, _ := dockerTask(t)
				cfg.VolumeDriver = c.volumeDriver
				cfg.Volumes = c.volumes

				task.AllocDir = allocDir
				task.Name = "demo"

				require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

				cc, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
				if c.requiresVolumes {
					require.Error(t, err, "volumes are not enabled")
				} else {
					require.NoError(t, err)

					for _, v := range c.expectedVolumes {
						require.Contains(t, cc.HostConfig.Binds, v)
					}
				}
			})
		}
	})
}

// This test does not run on windows due to differences in the definition of
// an absolute path, changing path expansion behaviour. A similar test should
// be written for windows.
func TestDockerDriver_MountsSerialization(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	allocDir := "/tmp/nomad/alloc-dir"

	cases := []struct {
		name            string
		requiresVolumes bool
		passedMounts    []DockerMount
		expectedMounts  []docker.HostMount
	}{
		{
			name:            "basic volume",
			requiresVolumes: true,
			passedMounts: []DockerMount{
				{
					Target:   "/nomad",
					ReadOnly: true,
					Source:   "test",
				},
			},
			expectedMounts: []docker.HostMount{
				{
					Type:          "volume",
					Target:        "/nomad",
					Source:        "test",
					ReadOnly:      true,
					VolumeOptions: &docker.VolumeOptions{},
				},
			},
		},
		{
			name: "basic bind",
			passedMounts: []DockerMount{
				{
					Type:   "bind",
					Target: "/nomad",
					Source: "test",
				},
			},
			expectedMounts: []docker.HostMount{
				{
					Type:        "bind",
					Target:      "/nomad",
					Source:      "/tmp/nomad/alloc-dir/demo/test",
					BindOptions: &docker.BindOptions{},
				},
			},
		},
		{
			name:            "basic absolute bind",
			requiresVolumes: true,
			passedMounts: []DockerMount{
				{
					Type:   "bind",
					Target: "/nomad",
					Source: "/tmp/test",
				},
			},
			expectedMounts: []docker.HostMount{
				{
					Type:        "bind",
					Target:      "/nomad",
					Source:      "/tmp/test",
					BindOptions: &docker.BindOptions{},
				},
			},
		},
		{
			name:            "bind relative outside",
			requiresVolumes: true,
			passedMounts: []DockerMount{
				{
					Type:   "bind",
					Target: "/nomad",
					Source: "../../test",
				},
			},
			expectedMounts: []docker.HostMount{
				{
					Type:        "bind",
					Target:      "/nomad",
					Source:      "/tmp/nomad/test",
					BindOptions: &docker.BindOptions{},
				},
			},
		},
		{
			name:            "basic tmpfs",
			requiresVolumes: false,
			passedMounts: []DockerMount{
				{
					Type:   "tmpfs",
					Target: "/nomad",
					TmpfsOptions: DockerTmpfsOptions{
						SizeBytes: 321,
						Mode:      0666,
					},
				},
			},
			expectedMounts: []docker.HostMount{
				{
					Type:   "tmpfs",
					Target: "/nomad",
					TempfsOptions: &docker.TempfsOptions{
						SizeBytes: 321,
						Mode:      0666,
					},
				},
			},
		},
	}

	t.Run("with volumes enabled", func(t *testing.T) {
		dh := dockerDriverHarness(t, nil)
		driver := dh.Impl().(*Driver)
		driver.config.Volumes.Enabled = true

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				task, cfg, _ := dockerTask(t)
				cfg.Mounts = c.passedMounts

				task.AllocDir = allocDir
				task.Name = "demo"

				require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

				cc, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
				require.NoError(t, err)
				require.EqualValues(t, c.expectedMounts, cc.HostConfig.Mounts)
			})
		}
	})

	t.Run("with volumes disabled", func(t *testing.T) {
		dh := dockerDriverHarness(t, nil)
		driver := dh.Impl().(*Driver)
		driver.config.Volumes.Enabled = false

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				task, cfg, _ := dockerTask(t)

				cfg.Mounts = c.passedMounts

				task.AllocDir = allocDir
				task.Name = "demo"

				require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

				cc, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
				if c.requiresVolumes {
					require.Error(t, err, "volumes are not enabled")
				} else {
					require.NoError(t, err)
					require.EqualValues(t, c.expectedMounts, cc.HostConfig.Mounts)
				}
			})
		}
	})
}

// TestDockerDriver_CreateContainerConfig_MountsCombined asserts that
// devices and mounts set by device managers/plugins are honored
// and present in docker.CreateContainerOptions, and that it is appended
// to any devices/mounts a user sets in the task config.
func TestDockerDriver_CreateContainerConfig_MountsCombined(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	task.Devices = []*drivers.DeviceConfig{
		{
			HostPath:    "/dev/fuse",
			TaskPath:    "/container/dev/task-fuse",
			Permissions: "rw",
		},
	}
	task.Mounts = []*drivers.MountConfig{
		{
			HostPath: "/tmp/task-mount",
			TaskPath: "/container/tmp/task-mount",
			Readonly: true,
		},
	}

	cfg.Devices = []DockerDevice{
		{
			HostPath:          "/dev/stdout",
			ContainerPath:     "/container/dev/cfg-stdout",
			CgroupPermissions: "rwm",
		},
	}
	cfg.Mounts = []DockerMount{
		{
			Type:     "bind",
			Source:   "/tmp/cfg-mount",
			Target:   "/container/tmp/cfg-mount",
			ReadOnly: false,
		},
	}

	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)
	driver.config.Volumes.Enabled = true

	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	require.NoError(t, err)
	expectedMounts := []docker.HostMount{
		{
			Type:     "bind",
			Source:   "/tmp/cfg-mount",
			Target:   "/container/tmp/cfg-mount",
			ReadOnly: false,
			BindOptions: &docker.BindOptions{
				Propagation: "",
			},
		},
		{
			Type:     "bind",
			Source:   "/tmp/task-mount",
			Target:   "/container/tmp/task-mount",
			ReadOnly: true,
			BindOptions: &docker.BindOptions{
				Propagation: "rprivate",
			},
		},
	}

	if runtime.GOOS != "linux" {
		expectedMounts[0].BindOptions = &docker.BindOptions{}
		expectedMounts[1].BindOptions = &docker.BindOptions{}
	}

	foundMounts := c.HostConfig.Mounts
	sort.Slice(foundMounts, func(i, j int) bool {
		return foundMounts[i].Target < foundMounts[j].Target
	})
	require.EqualValues(t, expectedMounts, foundMounts)

	expectedDevices := []docker.Device{
		{
			PathOnHost:        "/dev/stdout",
			PathInContainer:   "/container/dev/cfg-stdout",
			CgroupPermissions: "rwm",
		},
		{
			PathOnHost:        "/dev/fuse",
			PathInContainer:   "/container/dev/task-fuse",
			CgroupPermissions: "rw",
		},
	}

	foundDevices := c.HostConfig.Devices
	sort.Slice(foundDevices, func(i, j int) bool {
		return foundDevices[i].PathInContainer < foundDevices[j].PathInContainer
	})
	require.EqualValues(t, expectedDevices, foundDevices)
}

// TestDockerDriver_Cleanup ensures Cleanup removes only downloaded images.
// Doesn't run on windows because it requires an image variant
func TestDockerDriver_Cleanup(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	// using a small image and an specific point release to avoid accidental conflicts with other tasks
	cfg := newTaskConfig("", []string{"sleep", "100"})
	cfg.Image = "busybox:1.29.2"
	cfg.LoadImage = ""
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "cleanup_test",
		Resources: basicResources,
	}

	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, driver, handle, cleanup := dockerSetup(t, task, map[string]interface{}{
		"gc": map[string]interface{}{
			"image":       true,
			"image_delay": "1ms",
		},
	})
	defer cleanup()

	require.NoError(t, driver.WaitUntilStarted(task.ID, 5*time.Second))
	// Cleanup
	require.NoError(t, driver.DestroyTask(task.ID, true))

	// Ensure image was removed
	tu.WaitForResult(func() (bool, error) {
		if _, err := client.InspectImage(cfg.Image); err == nil {
			return false, fmt.Errorf("image exists but should have been removed. Does another %v container exist?", cfg.Image)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// The image doesn't exist which shouldn't be an error when calling
	// Cleanup, so call it again to make sure.
	require.NoError(t, driver.Impl().(*Driver).cleanupImage(handle))
}

// Tests that images prefixed with "https://" are supported
func TestDockerDriver_Start_Image_HTTPS(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	taskCfg := TaskConfig{
		Image:            "https://gcr.io/google_containers/pause:0.8.0",
		ImagePullTimeout: "5m",
	}
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "pause",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	harness := dockerDriverHarness(t, nil)
	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	_, _, err := harness.StartTask(task)
	require.NoError(t, err)

	err = harness.WaitUntilStarted(task.ID, 1*time.Minute)
	require.NoError(t, err)

	harness.DestroyTask(task.ID, true)
}

func newTaskConfig(variant string, command []string) TaskConfig {
	// busyboxImageID is the ID stored in busybox.tar
	busyboxImageID := "busybox:1.29.3"

	image := busyboxImageID
	loadImage := "busybox.tar"
	if variant != "" {
		image = fmt.Sprintf("%s-%s", busyboxImageID, variant)
		loadImage = fmt.Sprintf("busybox_%s.tar", variant)
	}

	return TaskConfig{
		Image:            image,
		ImagePullTimeout: "5m",
		LoadImage:        loadImage,
		Command:          command[0],
		Args:             command[1:],
	}
}

func copyImage(t *testing.T, taskDir *allocdir.TaskDir, image string) {
	dst := filepath.Join(taskDir.LocalDir, image)
	copyFile(filepath.Join("./test-resources/docker", image), dst, t)
}

// copyFile moves an existing file to the destination
func copyFile(src, dst string, t *testing.T) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("copying %v -> %v failed: %v", src, dst, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	require.NoError(t, err, "copying %v -> %v failed: %v", src, dst, err)

	defer func() {
		if err := out.Close(); err != nil {
			t.Fatalf("copying %v -> %v failed: %v", src, dst, err)
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		t.Fatalf("copying %v -> %v failed: %v", src, dst, err)
	}
	if err := out.Sync(); err != nil {
		t.Fatalf("copying %v -> %v failed: %v", src, dst, err)
	}
}

func TestDocker_ExecTaskStreaming(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	taskCfg := newTaskConfig("", []string{"/bin/sleep", "1000"})
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "nc-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	harness := dockerDriverHarness(t, nil)
	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	_, _, err := harness.StartTask(task)
	require.NoError(t, err)

	err = harness.WaitUntilStarted(task.ID, 1*time.Minute)
	require.NoError(t, err)

	defer harness.DestroyTask(task.ID, true)

	dtestutil.ExecTaskStreamingConformanceTests(t, harness, task.ID)

}

// Tests that a given DNSConfig properly configures dns
func Test_dnsConfig(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	cases := []struct {
		name string
		cfg  *drivers.DNSConfig
	}{
		{
			name: "nil",
		},
		{
			name: "basic",
			cfg: &drivers.DNSConfig{
				Servers: []string{"1.1.1.1", "1.0.0.1"},
			},
		},
		{
			name: "full",
			cfg: &drivers.DNSConfig{
				Servers:  []string{"1.1.1.1", "1.0.0.1"},
				Searches: []string{"local.test", "node.consul"},
				Options:  []string{"ndots:2", "edns0"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			harness := dockerDriverHarness(t, nil)

			taskCfg := newTaskConfig("", []string{"/bin/sleep", "1000"})
			task := &drivers.TaskConfig{
				ID:        uuid.Generate(),
				Name:      "nc-demo",
				AllocID:   uuid.Generate(),
				Resources: basicResources,
				DNS:       c.cfg,
			}
			require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

			cleanup := harness.MkAllocDir(task, false)

			_, _, err := harness.StartTask(task)
			require.NoError(t, err)

			err = harness.WaitUntilStarted(task.ID, 1*time.Minute)
			require.NoError(t, err)

			dtestutil.TestTaskDNSConfig(t, harness, task.ID, c.cfg)

			// cleanup immediately before the next test case
			require.NoError(t, harness.DestroyTask(task.ID, true))
			cleanup()
			harness.Kill()
		})
	}
}
