// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package docker

import (
	"context"
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

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	ntestutil "github.com/hashicorp/nomad/testutil"
	tu "github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestDockerDriver_User(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	task.User = "alice"
	cfg.Command = "/bin/sleep"
	cfg.Args = []string{"10000"}
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

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

	ctx := context.Background()

	// Because go-dockerclient doesn't provide api for query network aliases, just check that
	// a container can be created with a 'network_aliases' property

	// Create network, network-scoped alias is supported only for containers in user defined networks
	client := newTestDockerClient(t)
	networkResponse, err := client.NetworkCreate(ctx, "foobar", network.CreateOptions{Driver: "bridge"})
	must.NoError(t, err)
	defer client.NetworkRemove(ctx, networkResponse.ID)

	network, err := client.NetworkInspect(ctx, networkResponse.ID, network.InspectOptions{})
	must.NoError(t, err)

	expected := []string{"foobar"}
	taskCfg := newTaskConfig("", busyboxLongRunningCmd)
	taskCfg.NetworkMode = network.Name
	taskCfg.NetworkAliases = expected
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "busybox",
		Resources: basicResources,
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	_, _, err = d.StartTask(task)
	must.NoError(t, err)
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	defer d.DestroyTask(task.ID, true)

	dockerDriver, ok := d.Impl().(*Driver)
	must.True(t, ok)

	handle, ok := dockerDriver.tasks.Get(task.ID)
	must.True(t, ok)

	_, err = client.ContainerInspect(ctx, handle.containerID)
	must.NoError(t, err)
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
	must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

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

	container, err := client.ContainerInspect(context.Background(), handle.containerID)
	must.NoError(t, err)

	actual := string(container.HostConfig.NetworkMode)
	must.Eq(t, expected, actual)
}

func TestDockerDriver_CPUCFSPeriod(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	cfg.CPUHardLimit = true
	cfg.CPUCFSPeriod = 1000000
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, _, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()

	waitForExist(t, client, handle.containerID)

	container, err := client.ContainerInspect(context.Background(), handle.containerID)
	must.NoError(t, err)

	must.Eq(t, cfg.CPUCFSPeriod, container.HostConfig.CPUPeriod)
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
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID)
	must.NoError(t, err)

	want := "16384"
	got := container.HostConfig.Sysctls["net.core.somaxconn"]
	must.Eq(t, want, got, must.Sprintf(
		"Wrong net.core.somaxconn config for docker job. Expect: %s, got: %s", want, got))

	expectedUlimitLen := 2
	actualUlimitLen := len(container.HostConfig.Ulimits)
	must.Eq(t, want, got, must.Sprintf(
		"Wrong number of ulimit configs for docker job. Expect: %d, got: %d", expectedUlimitLen, actualUlimitLen))

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
			must.Eq(t, int64(soft), got.Soft, must.Sprintf(
				"Wrong soft %s ulimit for docker job. Expect: %d, got: %d", got.Name, soft, got.Soft))
			must.Eq(t, int64(hard), got.Hard, must.Sprintf(
				"Wrong hard %s ulimit for docker job. Expect: %d, got: %d", got.Name, hard, got.Hard))

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
		must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

		d := dockerDriverHarness(t, nil)
		cleanup := d.MkAllocDir(task, true)
		t.Cleanup(cleanup)
		copyImage(t, task.TaskDir(), "busybox.tar")

		_, _, err := d.StartTask(task)
		must.ErrorContains(t, err, tc.err.Error())
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

				must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

				cc, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
				must.NoError(t, err)

				for _, v := range c.expectedVolumes {
					must.SliceContains(t, cc.Host.Binds, v)
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

				must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

				cc, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
				if c.requiresVolumes {
					must.Error(t, err, must.Sprint("volumes are not enabled"))
				} else {
					must.NoError(t, err)

					for _, v := range c.expectedVolumes {
						must.SliceContains(t, cc.Host.Binds, v)
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
		expectedMounts  []mount.Mount
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
			expectedMounts: []mount.Mount{
				{
					Type:          "volume",
					Target:        "/nomad",
					Source:        "test",
					ReadOnly:      true,
					VolumeOptions: &mount.VolumeOptions{DriverConfig: &mount.Driver{}},
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
			expectedMounts: []mount.Mount{
				{
					Type:        "bind",
					Target:      "/nomad",
					Source:      "/tmp/nomad/alloc-dir/demo/test",
					BindOptions: &mount.BindOptions{},
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
			expectedMounts: []mount.Mount{
				{
					Type:        "bind",
					Target:      "/nomad",
					Source:      "/tmp/test",
					BindOptions: &mount.BindOptions{},
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
			expectedMounts: []mount.Mount{
				{
					Type:        "bind",
					Target:      "/nomad",
					Source:      "/tmp/nomad/test",
					BindOptions: &mount.BindOptions{},
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
			expectedMounts: []mount.Mount{
				{
					Type:   "tmpfs",
					Target: "/nomad",
					TmpfsOptions: &mount.TmpfsOptions{
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

				must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

				cc, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
				must.NoError(t, err)
				must.Eq(t, c.expectedMounts, cc.Host.Mounts)
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

				must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

				cc, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
				if c.requiresVolumes {
					must.Error(t, err, must.Sprint("volumes are not enabled"))
				} else {
					must.NoError(t, err)
					must.Eq(t, c.expectedMounts, cc.Host.Mounts)
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

	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)
	driver.config.Volumes.Enabled = true

	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	must.NoError(t, err)
	expectedMounts := []mount.Mount{
		{
			Type:     "bind",
			Source:   "/tmp/cfg-mount",
			Target:   "/container/tmp/cfg-mount",
			ReadOnly: false,
			BindOptions: &mount.BindOptions{
				Propagation: "",
			},
		},
		{
			Type:     "bind",
			Source:   "/tmp/task-mount",
			Target:   "/container/tmp/task-mount",
			ReadOnly: true,
			BindOptions: &mount.BindOptions{
				Propagation: "rprivate",
			},
		},
	}

	if runtime.GOOS != "linux" {
		expectedMounts[0].BindOptions = &mount.BindOptions{}
		expectedMounts[1].BindOptions = &mount.BindOptions{}
	}

	foundMounts := c.Host.Mounts
	sort.Slice(foundMounts, func(i, j int) bool {
		return foundMounts[i].Target < foundMounts[j].Target
	})
	must.Eq(t, expectedMounts, foundMounts)

	expectedDevices := []container.DeviceMapping{
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

	foundDevices := c.Host.Devices
	sort.Slice(foundDevices, func(i, j int) bool {
		return foundDevices[i].PathInContainer < foundDevices[j].PathInContainer
	})
	must.Eq(t, expectedDevices, foundDevices)
}

// TestDockerDriver_Cleanup ensures Cleanup removes only downloaded images.
// Doesn't run on windows because it requires an image variant
func TestDockerDriver_Cleanup(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	// using a small image and an specific point release to avoid accidental conflicts with other tasks
	cfg := newTaskConfig("", []string{"sleep", "100"})
	cfg.Image = ntestutil.TestDockerImage("busybox", "1.29.2")
	cfg.LoadImage = ""
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "cleanup_test",
		Resources: basicResources,
	}

	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, driver, handle, cleanup := dockerSetup(t, task, map[string]interface{}{
		"gc": map[string]interface{}{
			"image":       true,
			"image_delay": "1ms",
		},
	})
	defer cleanup()

	must.NoError(t, driver.WaitUntilStarted(task.ID, 5*time.Second))
	// Cleanup
	must.NoError(t, driver.DestroyTask(task.ID, true))

	// Ensure image was removed
	tu.WaitForResult(func() (bool, error) {
		if _, _, err := client.ImageInspectWithRaw(context.Background(), cfg.Image); err == nil {
			return false, fmt.Errorf("image exists but should have been removed. Does another %v container exist?", cfg.Image)
		}

		return true, nil
	}, func(err error) {
		must.NoError(t, err)
	})

	// The image doesn't exist which shouldn't be an error when calling
	// Cleanup, so call it again to make sure.
	must.NoError(t, driver.Impl().(*Driver).cleanupImage(handle))
}

// Tests that images prefixed with "https://" are supported
func TestDockerDriver_Start_Image_HTTPS(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	taskCfg := TaskConfig{
		Image:            "https://registry.k8s.io/pause:3.3",
		ImagePullTimeout: "5m",
	}
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "pause",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	harness := dockerDriverHarness(t, nil)
	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	_, _, err := harness.StartTask(task)
	must.NoError(t, err)

	err = harness.WaitUntilStarted(task.ID, 1*time.Minute)
	must.NoError(t, err)

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
	must.NoError(t, err, must.Sprintf("copying %v -> %v failed: %v", src, dst, err))

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
	must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	harness := dockerDriverHarness(t, nil)
	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	_, _, err := harness.StartTask(task)
	must.NoError(t, err)

	err = harness.WaitUntilStarted(task.ID, 1*time.Minute)
	must.NoError(t, err)

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
			must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

			cleanup := harness.MkAllocDir(task, false)

			_, _, err := harness.StartTask(task)
			must.NoError(t, err)

			err = harness.WaitUntilStarted(task.ID, 1*time.Minute)
			must.NoError(t, err)

			dtestutil.TestTaskDNSConfig(t, harness, task.ID, c.cfg)

			// cleanup immediately before the next test case
			must.NoError(t, harness.DestroyTask(task.ID, true))
			cleanup()
			harness.Kill()
		})
	}
}
