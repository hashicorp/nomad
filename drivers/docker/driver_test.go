// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/go-connections/nat"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/pluginutils/hclspecutils"
	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	tu "github.com/hashicorp/nomad/testutil"
	"github.com/moby/moby/api/types/container"
	containerapi "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"
	mclient "github.com/moby/moby/client"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

var (
	basicResources = &drivers.Resources{
		NomadResources: &structs.AllocatedTaskResources{
			Memory: structs.AllocatedMemoryResources{
				MemoryMB: 256,
			},
			Cpu: structs.AllocatedCpuResources{
				CpuShares: 250,
			},
		},
		LinuxResources: &drivers.LinuxResources{
			CPUShares:        512,
			MemoryLimitBytes: 256 * 1024 * 1024,
		},
	}
)

var (
	top = numalib.Scan(numalib.PlatformScanners(false))
)

func dockerIsRemote() bool {
	client, err := mclient.NewClientWithOpts(mclient.FromEnv, mclient.WithAPIVersionNegotiation())
	if err != nil {
		return false
	}

	// Technically this could be a local tcp socket but for testing purposes
	// we'll just assume that tcp is only used for remote connections.
	if client.DaemonHost()[0:3] == "tcp" {
		return true
	}
	return false
}

var (
	// busyboxLongRunningCmd is a busybox command that runs indefinitely, and
	// ideally responds to SIGINT/SIGTERM.  Sadly, busybox:1.29.3 /bin/sleep doesn't.
	busyboxLongRunningCmd = []string{"nc", "-l", "-p", "3000", "127.0.0.1"}
)

// Returns a task with a reserved and dynamic port.
func dockerTask(t *testing.T) (*drivers.TaskConfig, *TaskConfig, []int) {
	ports := ci.PortAllocator.Grab(2)
	dockerReserved := ports[0]
	dockerDynamic := ports[1]

	cfg := newTaskConfig(busyboxLongRunningCmd)
	task := &drivers.TaskConfig{
		ID:      uuid.Generate(),
		Name:    "redis-demo",
		AllocID: uuid.Generate(),
		Env: map[string]string{
			"test":              t.Name(),
			"NOMAD_ALLOC_DIR":   "/alloc",
			"NOMAD_TASK_DIR":    "/local",
			"NOMAD_SECRETS_DIR": "/secrets",
		},
		DeviceEnv: make(map[string]string),
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 256,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 512,
				},
				Networks: []*structs.NetworkResource{
					{
						IP:            "127.0.0.1",
						ReservedPorts: []structs.Port{{Label: "main", Value: dockerReserved}},
						DynamicPorts:  []structs.Port{{Label: "REDIS", Value: dockerDynamic}},
					},
				},
			},
			LinuxResources: &drivers.LinuxResources{
				CPUShares:        512,
				MemoryLimitBytes: 256 * 1024 * 1024,
				PercentTicks:     float64(512) / float64(4096),
			},
		},
	}

	if runtime.GOOS == "windows" {
		task.Env["NOMAD_ALLOC_DIR"] = "c:/alloc"
		task.Env["NOMAD_TASK_DIR"] = "c:/local"
		task.Env["NOMAD_SECRETS_DIR"] = "c:/secrets"
	}

	must.NoError(t, task.EncodeConcreteDriverConfig(&cfg))

	return task, &cfg, ports
}

// dockerSetup does all of the basic setup you need to get a running docker
// process up and running for testing. Use like:
//
//	task := taskTemplate()
//	// do custom task configuration
//	client, handle, cleanup := dockerSetup(t, task, nil)
//	defer cleanup()
//	// do test stuff
//
// If there is a problem during setup this function will abort or skip the test
// and indicate the reason.
func dockerSetup(t *testing.T, task *drivers.TaskConfig, driverCfg map[string]interface{}) (*mclient.Client, *dtestutil.DriverHarness, *taskHandle, func()) {
	client := newTestDockerClient(t)
	driver := dockerDriverHarness(t, driverCfg)
	cleanup := driver.MkAllocDir(task, loggingIsEnabled(&DriverConfig{}, task))

	copyImage(t, task.TaskDir(), fmt.Sprintf("busybox_linux_%s.tar", runtime.GOARCH))
	_, _, err := driver.StartTask(task)
	must.NoError(t, err)

	dockerDriver, ok := driver.Impl().(*Driver)
	must.True(t, ok)
	handle, ok := dockerDriver.tasks.Get(task.ID)
	must.True(t, ok)

	return client, driver, handle, func() {
		driver.DestroyTask(task.ID, true)
		cleanup()
	}
}

// cleanSlate removes the specified docker image, including potentially stopping/removing any
// containers based on that image. This is used to decouple tests that would be coupled
// by using the same container image.
func cleanSlate(client *mclient.Client, imageID string) {
	ctx := context.Background()
	if img, err := client.ImageInspect(ctx, imageID); err != nil || img.ID == "" {
		return
	}
	containers, _ := client.ContainerList(ctx, mclient.ContainerListOptions{
		All:     true,
		Filters: mclient.Filters{}.Add("ancestor", imageID),
	})
	for _, c := range containers.Items {
		_, _ = client.ContainerRemove(ctx, c.ID, mclient.ContainerRemoveOptions{Force: true})
	}
	_, _ = client.ImageRemove(ctx, imageID, mclient.ImageRemoveOptions{
		Force: true,
	})
}

// dockerDriverHarness wires up everything needed to launch a task with a docker driver.
// A driver plugin interface and cleanup function is returned
func dockerDriverHarness(t *testing.T, cfg map[string]interface{}) *dtestutil.DriverHarness {
	logger := testlog.HCLogger(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel() })
	harness := dtestutil.NewDriverHarness(t, NewDockerDriver(ctx, logger))
	if cfg == nil {
		cfg = map[string]interface{}{
			"gc": map[string]interface{}{
				"image":       false,
				"image_delay": "1s",
			},
		}
	}

	plugLoader, err := loader.NewPluginLoader(&loader.PluginLoaderConfig{
		Logger:            logger,
		PluginDir:         "./plugins",
		SupportedVersions: loader.AgentSupportedApiVersions,
		InternalPlugins: map[loader.PluginID]*loader.InternalPluginConfig{
			PluginID: {
				Config: cfg,
				Factory: func(context.Context, hclog.Logger) interface{} {
					return harness
				},
			},
		},
	})

	must.NoError(t, err)
	instance, err := plugLoader.Dispense(pluginName, base.PluginTypeDriver, nil, logger)
	must.NoError(t, err)
	driver, ok := instance.Plugin().(*dtestutil.DriverHarness)
	if !ok {
		t.Fatal("plugin instance is not a driver... wat?")
	}

	return driver
}

func newTestDockerClient(t *testing.T) *mclient.Client {
	t.Helper()
	testutil.DockerCompatible(t)

	client, err := mclient.NewClientWithOpts(mclient.FromEnv, mclient.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to initialize client: %s\nStack\n%s", err, debug.Stack())
	}
	return client
}

// testRemoteDockerImage is used only for tests where we need to pull the Docker
// image from a remote. Most tests should use newTaskConfig instead.
func testRemoteDockerImage(name, tag string) string {
	img := name + ":" + tag
	if tu.IsCI() {
		// use our mirror to avoid rate-limiting in CI
		img = "docker.mirror.hashicorp.services/" + img
	} else {
		// explicitly include docker.io for podman
		img = "docker.io/" + img
	}
	return img
}

// Following tests have been removed from this file.
// [TestDockerDriver_Fingerprint, TestDockerDriver_Fingerprint_Bridge, TestDockerDriver_Check_DockerHealthStatus]
// If you want to checkout/revert those tests, please check commit: 41715b1860778aa80513391bd64abd721d768ab0

func TestDockerDriver_Start_Wait(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	taskCfg := newTaskConfig(busyboxLongRunningCmd)
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "nc-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), taskCfg.LoadImage)

	_, _, err := d.StartTask(task)
	must.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	// Attempt to wait
	waitCh, err := d.WaitTask(context.Background(), task.ID)
	must.NoError(t, err)

	select {
	case <-waitCh:
		t.Fatalf("wait channel should not have received an exit result")
	case <-time.After(time.Duration(tu.TestMultiplier()*1) * time.Second):
	}
}

func TestDockerDriver_Start_WaitFinish(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	taskCfg := newTaskConfig([]string{"echo", "hello"})
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "nc-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), taskCfg.LoadImage)

	_, _, err := d.StartTask(task)
	must.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	// Attempt to wait
	waitCh, err := d.WaitTask(context.Background(), task.ID)
	must.NoError(t, err)

	select {
	case res := <-waitCh:
		if !res.Successful() {
			t.Fatalf("ExitResult should be successful: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatal("timeout")
	}
}

// TestDockerDriver_Start_StoppedContainer asserts that Nomad will detect a
// stopped task container, remove it, and start a new container.
//
// See https://github.com/hashicorp/nomad/issues/3419
func TestDockerDriver_Start_StoppedContainer(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	taskCfg := newTaskConfig([]string{"sleep", "9001"})
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "nc-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), taskCfg.LoadImage)

	client := newTestDockerClient(t)

	var imageID string
	var err error

	if runtime.GOOS != "windows" {
		imageID, _, err = d.Impl().(*Driver).loadImage(task, &taskCfg, client)
	} else {
		image, lErr := client.ImageInspect(context.Background(), taskCfg.Image)
		err = lErr
		if image.ID != "" {
			imageID = image.ID
		}
	}
	must.NoError(t, err)
	must.NotEq(t, imageID, "")

	// Create a container of the same name but don't start it. This mimics
	// the case of dockerd getting restarted and stopping containers while
	// Nomad is watching them.
	containerName := strings.Replace(task.ID, "/", "_", -1)
	opts := &containerapi.Config{
		Cmd:   []string{"sleep", "9000"},
		Env:   []string{fmt.Sprintf("test=%s", t.Name())},
		Image: taskCfg.Image,
	}

	_, err = client.ContainerCreate(context.Background(), mclient.ContainerCreateOptions{
		Name:   containerName,
		Config: opts,
	})
	must.NoError(t, err)

	if _, err := client.ContainerCreate(context.Background(), mclient.ContainerCreateOptions{
		Name:   containerName,
		Config: opts,
	}); err != nil {
		if !errdefs.IsConflict(err) {
			t.Fatalf("error creating initial container: %v", err)
		}
	}

	_, _, err = d.StartTask(task)
	defer d.DestroyTask(task.ID, true)
	must.NoError(t, err)

	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))
	must.NoError(t, d.DestroyTask(task.ID, true))

	_, err = client.ContainerRemove(context.Background(), containerName, mclient.ContainerRemoveOptions{Force: true})
	must.NoError(t, err)
}

// TestDockerDriver_ContainerAlreadyExists asserts that when Nomad tries to
// start a job and the container already exists, it purges it (if it's not in
// the running state), and starts it again (as opposed to trying to
// continuously re-create an already existing container)
func TestDockerDriver_ContainerAlreadyExists(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	ctx := context.Background()

	task, cfg, _ := dockerTask(t)
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client := newTestDockerClient(t)
	driver := dockerDriverHarness(t, nil)
	cleanup := driver.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), cfg.LoadImage)

	d, ok := driver.Impl().(*Driver)
	must.True(t, ok)

	_, _, err := d.createImage(task, cfg, client)
	must.NoError(t, err)

	containerCfg, err := d.createContainerConfig(task, cfg, cfg.Image)
	must.NoError(t, err)

	// create a container
	c, err := d.createContainer(client, containerCfg, cfg.Image)
	must.NoError(t, err)
	defer client.ContainerRemove(ctx, c.Container.ID, mclient.ContainerRemoveOptions{Force: true})

	// now that the container has been created, start the task that uses it, and
	// assert that it doesn't end up in "container already exists" fail loop
	_, _, err = d.StartTask(task)
	must.NoError(t, err)
	d.DestroyTask(task.ID, true)

	// let's try all of the above again, but this time with a created and running
	// container
	c, err = d.createContainer(client, containerCfg, cfg.Image)
	must.NoError(t, err)
	defer client.ContainerRemove(ctx, c.Container.ID, mclient.ContainerRemoveOptions{Force: true})

	must.NoError(t, d.startContainer(*c))
	_, _, err = d.StartTask(task)
	must.NoError(t, err)
	d.DestroyTask(task.ID, true)
}

func TestDockerDriver_Start_LoadImage(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	taskCfg := newTaskConfig([]string{"sh", "-c", "echo hello > $NOMAD_TASK_DIR/output"})
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "busybox-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), taskCfg.LoadImage)

	_, _, err := d.StartTask(task)
	must.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	waitCh, err := d.WaitTask(context.Background(), task.ID)
	must.NoError(t, err)
	select {
	case res := <-waitCh:
		if !res.Successful() {
			t.Fatalf("ExitResult should be successful: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatal("timeout")
	}

	// Check that data was written to the shared alloc directory.
	outputFile := filepath.Join(task.TaskDir().LocalDir, "output")
	act, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Couldn't read expected output: %v", err)
	}

	exp := "hello"
	if strings.TrimSpace(string(act)) != exp {
		t.Fatalf("Command outputted %v; want %v", act, exp)
	}

}

// Tests that starting a task without an image fails
func TestDockerDriver_Start_NoImage(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	taskCfg := TaskConfig{
		Command: "echo",
		Args:    []string{"foo"},
	}
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "echo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, false)
	defer cleanup()

	_, _, err := d.StartTask(task)
	must.Error(t, err)
	must.StrContains(t, err.Error(), "image name required")

	d.DestroyTask(task.ID, true)
}

func TestDockerDriver_Start_BadPull_Recoverable(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	taskCfg := TaskConfig{
		Image:            "127.0.0.1:32121/foo", // bad path
		ImagePullTimeout: "5m",
		Command:          "echo",
		Args: []string{
			"hello",
		},
	}
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "busybox-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()

	_, _, err := d.StartTask(task)
	must.Error(t, err)

	defer d.DestroyTask(task.ID, true)

	if rerr, ok := err.(*structs.RecoverableError); !ok {
		t.Fatalf("want recoverable error: %+v", err)
	} else if !rerr.IsRecoverable() {
		t.Fatalf("error not recoverable: %+v", err)
	}
}

func TestDockerDriver_Start_Wait_AllocDir(t *testing.T) {
	ci.Parallel(t)
	// This test musts that the alloc dir be mounted into docker as a volume.
	// Because this cannot happen when docker is run remotely, e.g. when running
	// docker in a VM, we skip this when we detect Docker is being run remotely.
	if !testutil.DockerIsConnected(t) || dockerIsRemote() {
		t.Skip("Docker not connected")
	}

	exp := []byte{'w', 'i', 'n'}
	file := "output.txt"

	taskCfg := newTaskConfig([]string{
		"sh",
		"-c",
		fmt.Sprintf(`sleep 1; echo -n %s > $%s/%s`,
			string(exp), taskenv.AllocDir, file),
	})
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "busybox-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), taskCfg.LoadImage)

	_, _, err := d.StartTask(task)
	must.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	// Attempt to wait
	waitCh, err := d.WaitTask(context.Background(), task.ID)
	must.NoError(t, err)

	select {
	case res := <-waitCh:
		if !res.Successful() {
			t.Fatalf("ExitResult should be successful: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatal("timeout")
	}

	// Check that data was written to the shared alloc directory.
	outputFile := filepath.Join(task.TaskDir().SharedAllocDir, file)
	act, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Couldn't read expected output: %v", err)
	}

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("Command outputted %v; want %v", act, exp)
	}
}

func TestDockerDriver_Start_Kill_Wait(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	taskCfg := newTaskConfig(busyboxLongRunningCmd)
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "busybox-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), taskCfg.LoadImage)

	_, _, err := d.StartTask(task)
	must.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	go func(t *testing.T) {
		time.Sleep(100 * time.Millisecond)
		signal := "SIGINT"
		if runtime.GOOS == "windows" {
			signal = "SIGKILL"
		}
		must.NoError(t, d.StopTask(task.ID, time.Second, signal))
	}(t)

	// Attempt to wait
	waitCh, err := d.WaitTask(context.Background(), task.ID)
	must.NoError(t, err)

	select {
	case res := <-waitCh:
		if res.Successful() {
			t.Fatalf("ExitResult should err: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatal(t, "timeout")
	}
}

func TestDockerDriver_Start_KillTimeout(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	if runtime.GOOS == "windows" {
		t.Skip("Windows Docker does not support SIGUSR1")
	}

	timeout := 2 * time.Second
	taskCfg := newTaskConfig([]string{"sleep", "10"})
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "busybox-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), taskCfg.LoadImage)

	_, _, err := d.StartTask(task)
	must.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	var killSent time.Time
	go func() {
		time.Sleep(100 * time.Millisecond)
		killSent = time.Now()
		must.NoError(t, d.StopTask(task.ID, timeout, "SIGUSR1"))
	}()

	// Attempt to wait
	waitCh, err := d.WaitTask(context.Background(), task.ID)
	must.NoError(t, err)

	var killed time.Time
	select {
	case <-waitCh:
		killed = time.Now()
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatal(t, "timeout")
	}

	must.True(t, killed.Sub(killSent) > timeout)
}

func TestDockerDriver_StartN(t *testing.T) {
	ci.Parallel(t)
	if runtime.GOOS == "windows" {
		t.Skip("Windows Docker does not support SIGINT")
	}
	testutil.DockerCompatible(t)

	task1, taskCfg, _ := dockerTask(t)
	task2, _, _ := dockerTask(t)
	task3, _, _ := dockerTask(t)

	taskList := []*drivers.TaskConfig{task1, task2, task3}

	t.Logf("Starting %d tasks", len(taskList))

	d := dockerDriverHarness(t, nil)
	// Let's spin up a bunch of things
	for _, task := range taskList {
		cleanup := d.MkAllocDir(task, true)
		defer cleanup()
		copyImage(t, task.TaskDir(), taskCfg.LoadImage)
		_, _, err := d.StartTask(task)
		must.NoError(t, err)

	}

	defer d.DestroyTask(task3.ID, true)
	defer d.DestroyTask(task2.ID, true)
	defer d.DestroyTask(task1.ID, true)

	t.Log("All tasks are started. Terminating...")
	for _, task := range taskList {
		must.NoError(t, d.StopTask(task.ID, time.Second, "SIGINT"))

		// Attempt to wait
		waitCh, err := d.WaitTask(context.Background(), task.ID)
		must.NoError(t, err)

		select {
		case <-waitCh:
		case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
			t.Fatal("timeout waiting on task")
		}
	}

	t.Log("Test complete!")
}

func TestDockerDriver_StartNVersions(t *testing.T) {
	ci.Parallel(t)
	if runtime.GOOS == "windows" || runtime.GOARCH == "arm64" {
		t.Skip("Skipped on windows or arm64, we don't have image variants available")
	}
	testutil.DockerCompatible(t)

	task1, cfg1, _ := dockerTask(t)

	tcfg1 := newTaskConfig([]string{"echo", "hello"})
	cfg1.Image = tcfg1.Image
	cfg1.LoadImage = tcfg1.LoadImage
	must.NoError(t, task1.EncodeConcreteDriverConfig(cfg1))

	task2, cfg2, _ := dockerTask(t)
	cfg2.Image = "busybox:1.29.3"
	cfg2.LoadImage = "busybox_musl.tar"
	must.NoError(t, task2.EncodeConcreteDriverConfig(cfg2))

	task3, cfg3, _ := dockerTask(t)
	cfg3.Image = "busybox:1.29.3"
	cfg3.LoadImage = "busybox_glibc.tar"
	must.NoError(t, task3.EncodeConcreteDriverConfig(cfg3))

	taskList := []*drivers.TaskConfig{task1, task2, task3}

	t.Logf("Starting %d tasks", len(taskList))
	d := dockerDriverHarness(t, nil)

	// Let's spin up a bunch of things
	for _, task := range taskList {
		cleanup := d.MkAllocDir(task, true)
		defer cleanup()
		copyImage(t, task.TaskDir(), "busybox_linux_amd64.tar")
		copyImage(t, task.TaskDir(), "busybox_musl.tar")
		copyImage(t, task.TaskDir(), "busybox_glibc.tar")
		_, _, err := d.StartTask(task)
		must.NoError(t, err)

		must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))
	}

	defer d.DestroyTask(task3.ID, true)
	defer d.DestroyTask(task2.ID, true)
	defer d.DestroyTask(task1.ID, true)

	t.Log("All tasks are started. Terminating...")
	for _, task := range taskList {
		must.NoError(t, d.StopTask(task.ID, time.Second, "SIGINT"))

		// Attempt to wait
		waitCh, err := d.WaitTask(context.Background(), task.ID)
		must.NoError(t, err)

		select {
		case <-waitCh:
		case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
			t.Fatal("timeout waiting on task")
		}
	}

	t.Log("Test complete!")
}

func TestDockerDriver_Labels(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	cfg.Labels = map[string]string{
		"label1": "value1",
		"label2": "value2",
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// expect to see 1 additional standard labels (allocID)
	must.Eq(t, len(cfg.Labels)+1, len(container.Container.Config.Labels))
	for k, v := range cfg.Labels {
		must.Eq(t, v, container.Container.Config.Labels[k])
	}
}

func TestDockerDriver_ExtraLabels(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	dockerClientConfig := make(map[string]interface{})

	dockerClientConfig["extra_labels"] = []string{"task*", "job_name"}
	client, d, handle, cleanup := dockerSetup(t, task, dockerClientConfig)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	expectedLabels := map[string]string{
		"com.hashicorp.nomad.alloc_id":        task.AllocID,
		"com.hashicorp.nomad.task_name":       task.Name,
		"com.hashicorp.nomad.task_group_name": task.TaskGroupName,
		"com.hashicorp.nomad.job_name":        task.JobName,
	}

	// expect to see 4 labels (allocID by default, task_name and task_group_name due to task*, and job_name)
	must.Eq(t, 4, len(container.Container.Config.Labels))
	for k, v := range expectedLabels {
		must.Eq(t, v, container.Container.Config.Labels[k])
	}
}

func TestDockerDriver_LoggingConfiguration(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	dockerClientConfig := make(map[string]interface{})
	loggerConfig := map[string]string{"gelf-address": "udp://1.2.3.4:12201", "tag": "gelf"}

	dockerClientConfig["logging"] = LoggingConfig{
		Type:   "gelf",
		Config: loggerConfig,
	}
	client, d, handle, cleanup := dockerSetup(t, task, dockerClientConfig)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)

	must.Eq(t, "gelf", container.Container.HostConfig.LogConfig.Type)
	must.Eq(t, loggerConfig, container.Container.HostConfig.LogConfig.Config)
}

// TestDockerDriver_LogCollectionDisabled ensures that logmon isn't configured
// when log collection is disable, but out-of-band Docker log shipping still
// works as expected
func TestDockerDriver_LogCollectionDisabled(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
	task.StdoutPath = os.DevNull
	task.StderrPath = os.DevNull

	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	dockerClientConfig := make(map[string]interface{})
	loggerConfig := map[string]string{"gelf-address": "udp://1.2.3.4:12201", "tag": "gelf"}

	dockerClientConfig["logging"] = LoggingConfig{
		Type:   "gelf",
		Config: loggerConfig,
	}
	client, d, handle, cleanup := dockerSetup(t, task, dockerClientConfig)
	t.Cleanup(cleanup)
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))
	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)
	must.Nil(t, handle.dlogger)

	must.Eq(t, "gelf", container.Container.HostConfig.LogConfig.Type)
	must.Eq(t, loggerConfig, container.Container.HostConfig.LogConfig.Config)
}

func TestDockerDriver_HealthchecksDisable(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
	cfg.Healthchecks.Disable = true

	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)

	must.NotNil(t, container.Container.Config.Healthcheck)
	must.Eq(t, []string{"NONE"}, container.Container.Config.Healthcheck.Test)
}

func TestDockerDriver_ForcePull(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	cfg.ForcePull = true
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()

	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	_, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.Nil(t, err)
}

func TestDockerDriver_ForcePull_RepoDigest(t *testing.T) {
	ci.Parallel(t)
	if runtime.GOOS == "windows" {
		t.Skip("TODO: Skipped digest test on Windows")
	}
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	sha := "@sha256:58ac43b2cc92c687a32c8be6278e50a063579655fe3090125dcb2af0ff9e1a64"
	imageName := "library/busybox" + sha
	if tu.IsCI() {
		imageName = "docker.mirror.hashicorp.services/busybox" + sha
	}

	cfg.LoadImage = ""
	cfg.Image = imageName
	cfg.ForcePull = true
	cfg.Command = busyboxLongRunningCmd[0]
	cfg.Args = busyboxLongRunningCmd[1:]
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)

	switch runtime.GOARCH {
	// TODO(jrasell): Renable this test for amd64 once we have investigated why
	// it has suddently changed to a different digest.
	case "amd64":
	// 	must.Eq(t, "sha256:8ac48589692a53a9b8c2d1ceaa6b402665aa7fe667ba51ccc03002300856d8c7", container.Container.Image)
	case "arm64":
		must.Eq(t, "sha256:ba3a78826904c625e65a2eed1f247bbab59898f043490e7113e88907bf7c6b3b", container.Container.Image)
	default:
		t.Fatalf("unsupported test architecture: %s", runtime.GOARCH)
	}
}

func TestDockerDriver_SecurityOptUnconfined(t *testing.T) {
	ci.Parallel(t)
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support seccomp")
	}
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	cfg.SecurityOpt = []string{"seccomp=unconfined"}
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	must.SliceContains(t, container.Container.HostConfig.SecurityOpt, "seccomp=unconfined")
}

func TestDockerDriver_SecurityOptFromFile(t *testing.T) {
	ci.Parallel(t)
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support seccomp")
	}
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	cfg.SecurityOpt = []string{"seccomp=./test-resources/docker/seccomp.json"}
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)

	must.StrContains(t, container.Container.HostConfig.SecurityOpt[0], "reboot")
}

func TestDockerDriver_Runtime(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	cfg.Runtime = "runc"
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)

	must.Eq(t, "runc", container.Container.HostConfig.Runtime)
}

func TestDockerDriver_CreateContainerConfig(t *testing.T) {
	ci.Parallel(t)

	task, cfg, _ := dockerTask(t)

	opt := map[string]string{"size": "120G"}

	cfg.StorageOpt = opt
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)

	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	must.NoError(t, err)

	must.Eq(t, "org/repo:0.1", c.Config.Image)
	must.Eq(t, opt, c.Host.StorageOpt)

	// Container name should be /<task_name>-<alloc_id> for backward compat
	containerName := fmt.Sprintf("%s-%s", strings.Replace(task.Name, "/", "_", -1), task.AllocID)
	must.Eq(t, containerName, c.Name)
}

func TestDockerDriver_CreateContainerConfig_RuntimeConflict(t *testing.T) {
	ci.Parallel(t)

	task, cfg, _ := dockerTask(t)

	task.DeviceEnv["NVIDIA_VISIBLE_DEVICES"] = "GPU_UUID_1"

	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)
	driver.gpuRuntime = true

	// Should error if a runtime was explicitly set that doesn't match gpu runtime
	cfg.Runtime = "nvidia"
	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	must.NoError(t, err)
	must.Eq(t, "nvidia", c.Host.Runtime)

	cfg.Runtime = "custom"
	_, err = driver.createContainerConfig(task, cfg, "org/repo:0.1")
	must.Error(t, err)
	must.StrContains(t, err.Error(), "conflicting runtime requests")
}

func TestDockerDriver_CreateContainerConfig_ChecksAllowRuntimes(t *testing.T) {
	ci.Parallel(t)

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)
	driver.gpuRuntime = true
	driver.config.allowRuntimes = map[string]struct{}{
		"runc":   {},
		"custom": {},
	}

	allowRuntime := []string{
		"", // default always works
		"runc",
		"custom",
	}

	task, cfg, _ := dockerTask(t)

	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	for _, runtime := range allowRuntime {
		t.Run(runtime, func(t *testing.T) {
			cfg.Runtime = runtime
			c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
			must.NoError(t, err)
			must.Eq(t, runtime, c.Host.Runtime)
		})
	}

	t.Run("not allowed: denied", func(t *testing.T) {
		cfg.Runtime = "denied"
		_, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
		must.Error(t, err)
		must.StrContains(t, err.Error(), `runtime "denied" is not allowed`)
	})

}

func TestDockerDriver_CreateContainerConfig_User(t *testing.T) {
	ci.Parallel(t)

	task, cfg, _ := dockerTask(t)

	task.User = "random-user-1"

	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)

	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	must.NoError(t, err)

	must.Eq(t, task.User, c.Config.User)
}

func TestDockerDriver_CreateContainerConfig_Labels(t *testing.T) {
	ci.Parallel(t)

	task, cfg, _ := dockerTask(t)

	task.AllocID = uuid.Generate()
	task.JobName = "redis-demo-job"

	cfg.Labels = map[string]string{
		"user_label": "user_value",

		// com.hashicorp.nomad. labels are reserved and
		// cannot be overridden
		"com.hashicorp.nomad.alloc_id": "bad_value",
	}

	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)

	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	must.NoError(t, err)

	expectedLabels := map[string]string{
		// user provided labels
		"user_label": "user_value",
		// default label
		"com.hashicorp.nomad.alloc_id": task.AllocID,
	}

	must.Eq(t, expectedLabels, c.Config.Labels)
}

func TestDockerDriver_CreateContainerConfig_Logging(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name           string
		loggingConfig  DockerLogging
		expectedConfig DockerLogging
	}{
		{
			"simple type",
			DockerLogging{Type: "fluentd"},
			DockerLogging{
				Type:   "fluentd",
				Config: map[string]string{},
			},
		},
		{
			"simple driver",
			DockerLogging{Driver: "fluentd"},
			DockerLogging{
				Type:   "fluentd",
				Config: map[string]string{},
			},
		},
		{
			"type takes precedence",
			DockerLogging{
				Type:   "json-file",
				Driver: "fluentd",
			},
			DockerLogging{
				Type:   "json-file",
				Config: map[string]string{},
			},
		},
		{
			"user config takes precedence, even if no type provided",
			DockerLogging{
				Type:   "",
				Config: map[string]string{"max-file": "3", "max-size": "10m"},
			},
			DockerLogging{
				Type:   "",
				Config: map[string]string{"max-file": "3", "max-size": "10m"},
			},
		},
		{
			"defaults to json-file w/ log rotation",
			DockerLogging{
				Type: "",
			},
			DockerLogging{
				Type:   "json-file",
				Config: map[string]string{"max-file": "2", "max-size": "2m"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			task, cfg, _ := dockerTask(t)

			cfg.Logging = c.loggingConfig
			must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

			dh := dockerDriverHarness(t, nil)
			driver := dh.Impl().(*Driver)

			cc, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
			must.NoError(t, err)

			must.Eq(t, c.expectedConfig.Type, cc.Host.LogConfig.Type)
			must.Eq(t, c.expectedConfig.Config["max-file"], cc.Host.LogConfig.Config["max-file"])
			must.Eq(t, c.expectedConfig.Config["max-size"], cc.Host.LogConfig.Config["max-size"])
		})
	}
}

func TestDockerDriver_CreateContainerConfig_Mounts(t *testing.T) {
	ci.Parallel(t)
	testutil.RequireLinux(t)

	task, cfg, _ := dockerTask(t)

	cfg.Mounts = []DockerMount{
		{
			Type:   "bind",
			Target: "/map-bind-target",
			Source: "/map-source",
		},
		{
			Type:   "tmpfs",
			Target: "/map-tmpfs-target",
		},
	}
	cfg.MountsList = []DockerMount{
		{
			Type:   "bind",
			Target: "/list-bind-target",
			Source: "/list-source",
		},
		{
			Type:   "tmpfs",
			Target: "/list-tmpfs-target",
		},
	}
	cfg.Volumes = []string{
		"/etc/ssl/certs:/etc/ssl/certs:ro",
		"/var/www:/srv/www",
	}

	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))
	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)
	driver.config.Volumes.Enabled = true
	driver.config.Volumes.SelinuxLabel = "z"

	cc, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	must.NoError(t, err)

	must.Eq(t, []mount.Mount{
		// from mount map
		{
			Type:        "bind",
			Target:      "/map-bind-target",
			Source:      "/map-source",
			BindOptions: &mount.BindOptions{},
		},
		{
			Type:         "tmpfs",
			Target:       "/map-tmpfs-target",
			TmpfsOptions: &mount.TmpfsOptions{},
		},
		// from mount list
		{
			Type:        "bind",
			Target:      "/list-bind-target",
			Source:      "/list-source",
			BindOptions: &mount.BindOptions{},
		},
		{
			Type:         "tmpfs",
			Target:       "/list-tmpfs-target",
			TmpfsOptions: &mount.TmpfsOptions{},
		},
	}, cc.Host.Mounts)

	must.Eq(t, []string{
		"alloc:/alloc:z",
		"redis-demo/local:/local:z",
		"redis-demo/secrets:/secrets:z",
		"/etc/ssl/certs:/etc/ssl/certs:ro,z",
		"/var/www:/srv/www:z",
	}, cc.Host.Binds)
}

func TestDockerDriver_CreateContainerConfig_Mounts_Windows(t *testing.T) {
	ci.Parallel(t)
	testutil.RequireWindows(t)

	task, cfg, _ := dockerTask(t)

	cfg.Mounts = []DockerMount{
		{
			Type:   "bind",
			Target: "/map-bind-target",
			Source: "/map-source",
		},
		{
			Type:   "tmpfs",
			Target: "/map-tmpfs-target",
		},
	}
	cfg.MountsList = []DockerMount{
		{
			Type:   "bind",
			Target: "/list-bind-target",
			Source: "/list-source",
		},
		{
			Type:   "tmpfs",
			Target: "/list-tmpfs-target",
		},
	}
	cfg.Volumes = []string{
		"c:/etc/ssl/certs:c:/etc/ssl/certs",
		"c:/var/www:c:/srv/www",
	}

	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))
	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)
	driver.config.Volumes.Enabled = true

	cc, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	must.NoError(t, err)

	must.Eq(t, []mount.Mount{
		// from mount map
		{
			Type:        "bind",
			Target:      "/map-bind-target",
			Source:      "redis-demo\\map-source",
			BindOptions: &mount.BindOptions{},
		},
		{
			Type:         "tmpfs",
			Target:       "/map-tmpfs-target",
			TmpfsOptions: &mount.TmpfsOptions{},
		},
		// from mount list
		{
			Type:        "bind",
			Target:      "/list-bind-target",
			Source:      "redis-demo\\list-source",
			BindOptions: &mount.BindOptions{},
		},
		{
			Type:         "tmpfs",
			Target:       "/list-tmpfs-target",
			TmpfsOptions: &mount.TmpfsOptions{},
		},
	}, cc.Host.Mounts)

	must.Eq(t, []string{
		`alloc:c:/alloc`,
		`redis-demo\local:c:/local`,
		`redis-demo\secrets:c:/secrets`,
		`c:\etc\ssl\certs:c:/etc/ssl/certs`,
		`c:\var\www:c:/srv/www`,
	}, cc.Host.Binds)
}

func TestDockerDriver_CreateContainerConfigWithRuntimes(t *testing.T) {
	ci.Parallel(t)
	testCases := []struct {
		description           string
		gpuRuntimeSet         bool
		expectToReturnError   bool
		expectedRuntime       string
		nvidiaDevicesProvided bool
	}{
		{
			description:           "gpu devices are provided, docker driver was able to detect nvidia-runtime 1",
			gpuRuntimeSet:         true,
			expectToReturnError:   false,
			expectedRuntime:       "nvidia",
			nvidiaDevicesProvided: true,
		},
		{
			description:           "gpu devices are provided, docker driver was able to detect nvidia-runtime 2",
			gpuRuntimeSet:         true,
			expectToReturnError:   false,
			expectedRuntime:       "nvidia-runtime-modified-name",
			nvidiaDevicesProvided: true,
		},
		{
			description:           "no gpu devices provided - no runtime should be set",
			gpuRuntimeSet:         true,
			expectToReturnError:   false,
			expectedRuntime:       "nvidia",
			nvidiaDevicesProvided: false,
		},
		{
			description:           "no gpuRuntime supported by docker driver",
			gpuRuntimeSet:         false,
			expectToReturnError:   true,
			expectedRuntime:       "nvidia",
			nvidiaDevicesProvided: true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			task, cfg, _ := dockerTask(t)

			dh := dockerDriverHarness(t, map[string]interface{}{
				"allow_runtimes": []string{"runc", "nvidia", "nvidia-runtime-modified-name"},
			})
			driver := dh.Impl().(*Driver)

			driver.gpuRuntime = testCase.gpuRuntimeSet
			driver.config.GPURuntimeName = testCase.expectedRuntime
			if testCase.nvidiaDevicesProvided {
				task.DeviceEnv["NVIDIA_VISIBLE_DEVICES"] = "GPU_UUID_1"
			}

			c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
			if testCase.expectToReturnError {
				must.NotNil(t, err)
			} else {
				must.NoError(t, err)
				if testCase.nvidiaDevicesProvided {
					must.Eq(t, testCase.expectedRuntime, c.Host.Runtime)
				} else {
					// no nvidia devices provided -> no point to use nvidia runtime
					must.Eq(t, "", c.Host.Runtime)
				}
			}
		})
	}
}

func TestDockerDriver_Capabilities(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)
	if runtime.GOOS == "windows" {
		t.Skip("Capabilities not supported on windows")
	}

	testCases := []struct {
		Name       string
		CapAdd     []string
		CapDrop    []string
		Allowlist  string
		StartError string
	}{
		{
			Name:    "default-allowlist-add-allowed",
			CapAdd:  []string{"fowner", "mknod"},
			CapDrop: []string{"all"},
		},
		{
			Name:       "default-allowlist-add-forbidden",
			CapAdd:     []string{"net_admin"},
			StartError: "net_admin",
		},
		{
			Name:    "default-allowlist-drop-existing",
			CapDrop: []string{"fowner", "mknod", "net_raw"},
		},
		{
			Name:      "restrictive-allowlist-drop-all",
			CapDrop:   []string{"all"},
			Allowlist: "fowner,mknod",
		},
		{
			Name:      "restrictive-allowlist-add-allowed",
			CapAdd:    []string{"fowner", "mknod"},
			CapDrop:   []string{"all"},
			Allowlist: "mknod,fowner",
		},
		{
			Name:       "restrictive-allowlist-add-forbidden",
			CapAdd:     []string{"net_admin", "mknod"},
			CapDrop:    []string{"all"},
			Allowlist:  "fowner,mknod",
			StartError: "net_admin",
		},
		{
			Name:      "permissive-allowlist",
			CapAdd:    []string{"mknod", "net_admin"},
			Allowlist: "all",
		},
		{
			Name:      "permissive-allowlist-add-all",
			CapAdd:    []string{"all"},
			Allowlist: "all",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			client := newTestDockerClient(t)
			task, cfg, _ := dockerTask(t)

			if len(tc.CapAdd) > 0 {
				cfg.CapAdd = tc.CapAdd
			}
			if len(tc.CapDrop) > 0 {
				cfg.CapDrop = tc.CapDrop
			}
			must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

			d := dockerDriverHarness(t, nil)
			dockerDriver, ok := d.Impl().(*Driver)
			must.True(t, ok)
			if tc.Allowlist != "" {
				dockerDriver.config.AllowCaps = strings.Split(tc.Allowlist, ",")
			}

			cleanup := d.MkAllocDir(task, true)
			defer cleanup()
			copyImage(t, task.TaskDir(), cfg.LoadImage)

			_, _, err := d.StartTask(task)
			defer d.DestroyTask(task.ID, true)
			if err == nil && tc.StartError != "" {
				t.Fatalf("Expected error in start: %v", tc.StartError)
			} else if err != nil {
				if tc.StartError == "" {
					must.NoError(t, err)
				} else {
					must.StrContains(t, err.Error(), tc.StartError)
				}
				return
			}

			handle, ok := dockerDriver.tasks.Get(task.ID)
			must.True(t, ok)

			must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

			container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
			must.NoError(t, err)

			must.Eq(t, len(tc.CapAdd), len(container.Container.HostConfig.CapAdd))
			must.Eq(t, len(tc.CapDrop), len(container.Container.HostConfig.CapDrop))
		})
	}
}

func TestDockerDriver_DNS(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)
	testutil.ExecCompatible(t)

	cases := []struct {
		name string
		cfg  *drivers.DNSConfig
	}{
		{
			name: "nil DNSConfig",
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
		task, cfg, _ := dockerTask(t)

		task.DNS = c.cfg
		must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

		_, d, _, cleanup := dockerSetup(t, task, nil)
		t.Cleanup(cleanup)

		must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))
		t.Cleanup(func() { _ = d.DestroyTask(task.ID, true) })

		dtestutil.TestTaskDNSConfig(t, d, task.ID, c.cfg)
	}

}

func TestDockerDriver_Init(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support init.")
	}

	task, cfg, _ := dockerTask(t)

	cfg.Init = true
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)

	must.Eq(t, cfg.Init, *container.Container.HostConfig.Init)
}

func TestDockerDriver_CPUSetCPUs(t *testing.T) {
	// The cpuset_cpus config option is ignored starting in Nomad 1.7

	ci.Parallel(t)
	testutil.DockerCompatible(t)
	testutil.CgroupsCompatible(t)

	testCases := []struct {
		Name       string
		CPUSetCPUs string
	}{
		{
			Name:       "Single CPU",
			CPUSetCPUs: "",
		},
		{
			Name:       "Comma separated list of CPUs",
			CPUSetCPUs: "",
		},
		{
			Name:       "Range of CPUs",
			CPUSetCPUs: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			task, cfg, _ := dockerTask(t)

			cfg.CPUSetCPUs = testCase.CPUSetCPUs
			must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

			client, d, handle, cleanup := dockerSetup(t, task, nil)
			defer cleanup()
			must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

			container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
			must.NoError(t, err)

			must.Eq(t, cfg.CPUSetCPUs, container.Container.HostConfig.Resources.CpusetCpus)
		})
	}
}

func TestDockerDriver_MemoryHardLimit(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support MemoryReservation")
	}

	task, cfg, _ := dockerTask(t)

	cfg.MemoryHardLimit = 300
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)

	must.Eq(t, task.Resources.LinuxResources.MemoryLimitBytes, container.Container.HostConfig.MemoryReservation)
	must.Eq(t, cfg.MemoryHardLimit*1024*1024, container.Container.HostConfig.Memory)
}

func TestDockerDriver_MACAddress(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)
	if runtime.GOOS == "windows" {
		t.Skip("Windows docker does not support setting MacAddress")
	}

	task, cfg, _ := dockerTask(t)

	cfg.MacAddress = "00:16:3e:00:00:00"
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)

	found := false
	for _, endpoint := range container.Container.NetworkSettings.Networks {
		if endpoint.MacAddress.String() == cfg.MacAddress {
			found = true
			break
		}
	}
	must.True(t, found)
}

func TestDockerWorkDir(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)

	cfg.WorkDir = "/some/path"
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)
	must.Eq(t, cfg.WorkDir, filepath.ToSlash(container.Container.Config.WorkingDir))
}

func TestDockerDriver_PortsNoMap(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, _, ports := dockerTask(t)
	res := ports[0]
	dyn := ports[1]

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)

	// Verify that the correct ports are EXPOSED
	expectedExposedPorts := map[string]struct{}{
		fmt.Sprintf("%d/tcp", res): {},
		fmt.Sprintf("%d/udp", res): {},
		fmt.Sprintf("%d/tcp", dyn): {},
		fmt.Sprintf("%d/udp", dyn): {},
	}

	actualExposedPorts := make(map[string]struct{}, len(container.Container.Config.ExposedPorts))
	for port := range container.Container.Config.ExposedPorts {
		actualExposedPorts[port.String()] = struct{}{}
	}
	must.Eq(t, expectedExposedPorts, actualExposedPorts)

	hostIP := "127.0.0.1"
	if runtime.GOOS == "windows" {
		hostIP = ""
	}

	// Verify that the correct ports are FORWARDED
	expectedPortBindings := map[string][]nat.PortBinding{
		fmt.Sprintf("%d/tcp", res): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", res)}},
		fmt.Sprintf("%d/udp", res): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", res)}},
		fmt.Sprintf("%d/tcp", dyn): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", dyn)}},
		fmt.Sprintf("%d/udp", dyn): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", dyn)}},
	}

	actualPortBindings := make(map[string][]nat.PortBinding, len(container.Container.HostConfig.PortBindings))
	for port, bindings := range container.Container.HostConfig.PortBindings {
		converted := make([]nat.PortBinding, len(bindings))
		for i, binding := range bindings {
			hostIP := ""
			if binding.HostIP.IsValid() {
				hostIP = binding.HostIP.String()
			}
			converted[i] = nat.PortBinding{
				HostIP:   hostIP,
				HostPort: binding.HostPort,
			}
		}
		actualPortBindings[port.String()] = converted
	}

	must.Eq(t, expectedPortBindings, actualPortBindings)
}

func TestDockerDriver_PortsMapping(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, ports := dockerTask(t)
	res := ports[0]
	dyn := ports[1]
	cfg.PortMap = map[string]int{
		"main":  8080,
		"REDIS": 6379,
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)

	// Verify that the port environment variables are set
	must.SliceContains(t, container.Container.Config.Env, "NOMAD_PORT_main=8080")
	must.SliceContains(t, container.Container.Config.Env, "NOMAD_PORT_REDIS=6379")

	// Verify that the correct ports are EXPOSED
	expectedExposedPorts := map[string]struct{}{
		"8080/tcp": {},
		"8080/udp": {},
		"6379/tcp": {},
		"6379/udp": {},
	}

	actualExposedPorts := make(map[string]struct{}, len(container.Container.Config.ExposedPorts))
	for port := range container.Container.Config.ExposedPorts {
		actualExposedPorts[port.String()] = struct{}{}
	}
	must.Eq(t, expectedExposedPorts, actualExposedPorts)

	hostIP := "127.0.0.1"
	if runtime.GOOS == "windows" {
		hostIP = ""
	}

	// Verify that the correct ports are FORWARDED
	expectedPortBindings := map[string][]nat.PortBinding{
		"8080/tcp": {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", res)}},
		"8080/udp": {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", res)}},
		"6379/tcp": {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", dyn)}},
		"6379/udp": {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", dyn)}},
	}

	actualPortBindings := make(map[string][]nat.PortBinding, len(container.Container.HostConfig.PortBindings))
	for port, bindings := range container.Container.HostConfig.PortBindings {
		converted := make([]nat.PortBinding, len(bindings))
		for i, binding := range bindings {
			hostIP := ""
			if binding.HostIP.IsValid() {
				hostIP = binding.HostIP.String()
			}
			converted[i] = nat.PortBinding{
				HostIP:   hostIP,
				HostPort: binding.HostPort,
			}
		}
		actualPortBindings[port.String()] = converted
	}
	must.Eq(t, expectedPortBindings, actualPortBindings)
}

func TestDockerDriver_CreateContainerConfig_Ports(t *testing.T) {
	ci.Parallel(t)

	task, cfg, ports := dockerTask(t)
	hostIP := "127.0.0.1"
	if runtime.GOOS == "windows" {
		hostIP = ""
	}
	portmappings := structs.AllocatedPorts(make([]structs.AllocatedPortMapping, len(ports)))
	portmappings[0] = structs.AllocatedPortMapping{
		Label:  "main",
		Value:  ports[0],
		HostIP: hostIP,
		To:     8080,
	}
	portmappings[1] = structs.AllocatedPortMapping{
		Label:  "REDIS",
		Value:  ports[1],
		HostIP: hostIP,
		To:     6379,
	}
	task.Resources.Ports = &portmappings
	cfg.Ports = []string{"main", "REDIS"}

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)

	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	must.NoError(t, err)

	must.Eq(t, "org/repo:0.1", c.Config.Image)

	// Verify that the correct ports are FORWARDED
	expectedPortBindings := map[string][]nat.PortBinding{
		"8080/tcp": {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", ports[0])}},
		"8080/udp": {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", ports[0])}},
		"6379/tcp": {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", ports[1])}},
		"6379/udp": {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", ports[1])}},
	}

	actualPortBindings := make(map[string][]nat.PortBinding, len(c.Host.PortBindings))
	for port, bindings := range c.Host.PortBindings {
		converted := make([]nat.PortBinding, len(bindings))
		for i, binding := range bindings {
			hostIP := ""
			if binding.HostIP.IsValid() {
				hostIP = binding.HostIP.String()
			}
			converted[i] = nat.PortBinding{
				HostIP:   hostIP,
				HostPort: binding.HostPort,
			}
		}
		actualPortBindings[port.String()] = converted
	}
	must.Eq(t, expectedPortBindings, actualPortBindings)

}
func TestDockerDriver_CreateContainerConfig_PortsMapping(t *testing.T) {
	ci.Parallel(t)

	task, cfg, ports := dockerTask(t)
	res := ports[0]
	dyn := ports[1]
	cfg.PortMap = map[string]int{
		"main":  8080,
		"REDIS": 6379,
	}
	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)

	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	must.NoError(t, err)

	must.Eq(t, "org/repo:0.1", c.Config.Image)
	must.SliceContains(t, c.Config.Env, "NOMAD_PORT_main=8080")
	must.SliceContains(t, c.Config.Env, "NOMAD_PORT_REDIS=6379")

	// Verify that the correct ports are FORWARDED
	hostIP := "127.0.0.1"
	if runtime.GOOS == "windows" {
		hostIP = ""
	}
	expectedPortBindings := map[string][]nat.PortBinding{
		"8080/tcp": {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", res)}},
		"8080/udp": {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", res)}},
		"6379/tcp": {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", dyn)}},
		"6379/udp": {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", dyn)}},
	}

	actualPortBindings := make(map[string][]nat.PortBinding, len(c.Host.PortBindings))
	for port, bindings := range c.Host.PortBindings {
		converted := make([]nat.PortBinding, len(bindings))
		for i, binding := range bindings {
			hostIP := ""
			if binding.HostIP.IsValid() {
				hostIP = binding.HostIP.String()
			}
			converted[i] = nat.PortBinding{
				HostIP:   hostIP,
				HostPort: binding.HostPort,
			}
		}
		actualPortBindings[port.String()] = converted
	}
	must.Eq(t, expectedPortBindings, actualPortBindings)

}

func TestDockerDriver_CleanupContainer(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
	cfg.Command = "echo"
	cfg.Args = []string{"hello"}
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()

	waitCh, err := d.WaitTask(context.Background(), task.ID)
	must.NoError(t, err)

	select {
	case res := <-waitCh:
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}

		err = d.DestroyTask(task.ID, false)
		must.NoError(t, err)

		time.Sleep(3 * time.Second)

		// Ensure that the container isn't present
		_, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
		if err == nil {
			t.Fatalf("expected to not get container")
		}

	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}
}

func TestDockerDriver_EnableImageGC(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)
	ctx := context.Background()

	task, cfg, _ := dockerTask(t)
	cfg.Command = "echo"
	cfg.Args = []string{"hello"}
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client := newTestDockerClient(t)
	driver := dockerDriverHarness(t, map[string]interface{}{
		"gc": map[string]interface{}{
			"container":   true,
			"image":       true,
			"image_delay": "2s",
		},
	})
	cleanup := driver.MkAllocDir(task, true)
	defer cleanup()

	cleanSlate(client, cfg.Image)

	copyImage(t, task.TaskDir(), cfg.LoadImage)
	_, _, err := driver.StartTask(task)
	must.NoError(t, err)

	dockerDriver, ok := driver.Impl().(*Driver)
	must.True(t, ok)
	_, ok = dockerDriver.tasks.Get(task.ID)
	must.True(t, ok)

	waitCh, err := dockerDriver.WaitTask(ctx, task.ID)
	must.NoError(t, err)
	select {
	case res := <-waitCh:
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}

	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}

	// we haven't called DestroyTask, image should be present
	_, err = client.ImageInspect(ctx, cfg.Image)
	must.NoError(t, err)

	err = dockerDriver.DestroyTask(task.ID, false)
	must.NoError(t, err)

	// image_delay is 3s, so image should still be around for a bit
	_, err = client.ImageInspect(ctx, cfg.Image)
	must.NoError(t, err)

	// Ensure image was removed
	tu.WaitForResult(func() (bool, error) {
		if _, err := client.ImageInspect(ctx, cfg.Image); err == nil {
			return false, fmt.Errorf("image exists but should have been removed. Does another %v container exist?", cfg.Image)
		}

		return true, nil
	}, func(err error) {
		must.NoError(t, err)
	})
}

func TestDockerDriver_DisableImageGC(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	ctx := context.Background()

	task, cfg, _ := dockerTask(t)
	cfg.Command = "echo"
	cfg.Args = []string{"hello"}
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client := newTestDockerClient(t)
	driver := dockerDriverHarness(t, map[string]interface{}{
		"gc": map[string]interface{}{
			"container":   true,
			"image":       false,
			"image_delay": "1s",
		},
	})
	cleanup := driver.MkAllocDir(task, true)
	defer cleanup()

	cleanSlate(client, cfg.Image)

	copyImage(t, task.TaskDir(), cfg.LoadImage)
	_, _, err := driver.StartTask(task)
	must.NoError(t, err)

	dockerDriver, ok := driver.Impl().(*Driver)
	must.True(t, ok)
	handle, ok := dockerDriver.tasks.Get(task.ID)
	must.True(t, ok)

	waitCh, err := dockerDriver.WaitTask(ctx, task.ID)
	must.NoError(t, err)
	select {
	case res := <-waitCh:
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}

	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}

	// we haven't called DestroyTask, image should be present
	_, err = client.ImageInspect(ctx, handle.containerImage)
	must.NoError(t, err)

	err = dockerDriver.DestroyTask(task.ID, false)
	must.NoError(t, err)

	// image_delay is 1s, wait a little longer
	time.Sleep(3 * time.Second)

	// image should not have been removed or scheduled to be removed
	_, err = client.ImageInspect(ctx, cfg.Image)
	must.NoError(t, err)
	dockerDriver.coordinator.imageLock.Lock()
	_, ok = dockerDriver.coordinator.deleteFuture[handle.containerImage]
	must.False(t, ok, must.Sprint("image should not be registered for deletion"))
	dockerDriver.coordinator.imageLock.Unlock()
}

func TestDockerDriver_MissingContainer_Cleanup(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	ctx := context.Background()

	task, cfg, _ := dockerTask(t)

	cfg.Command = "echo"
	cfg.Args = []string{"hello"}
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client := newTestDockerClient(t)
	driver := dockerDriverHarness(t, map[string]interface{}{
		"gc": map[string]interface{}{
			"container":   true,
			"image":       true,
			"image_delay": "0s",
		},
	})
	cleanup := driver.MkAllocDir(task, true)
	defer cleanup()

	cleanSlate(client, cfg.Image)

	copyImage(t, task.TaskDir(), cfg.LoadImage)
	_, _, err := driver.StartTask(task)
	must.NoError(t, err)

	dockerDriver, ok := driver.Impl().(*Driver)
	must.True(t, ok)
	h, ok := dockerDriver.tasks.Get(task.ID)
	must.True(t, ok)

	waitCh, err := dockerDriver.WaitTask(context.Background(), task.ID)
	must.NoError(t, err)
	select {
	case res := <-waitCh:
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}

	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}

	// remove the container out-of-band
	_, err = client.ContainerRemove(ctx, h.containerID, mclient.ContainerRemoveOptions{})
	must.NoError(t, err)

	must.NoError(t, dockerDriver.DestroyTask(task.ID, false))

	// Ensure image was removed
	tu.WaitForResult(func() (bool, error) {
		if _, err := client.ImageInspect(ctx, cfg.Image); err == nil {
			return false, fmt.Errorf("image exists but should have been removed. Does another %v container exist?", cfg.Image)
		}

		return true, nil
	}, func(err error) {
		must.NoError(t, err)
	})

	// Ensure that task handle was removed
	_, ok = dockerDriver.tasks.Get(task.ID)
	must.False(t, ok)
}

func TestDockerDriver_Stats(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	ctx := context.Background()

	task, cfg, _ := dockerTask(t)

	cfg.Command = "sleep"
	cfg.Args = []string{"1000"}
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	_, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	ch, err := handle.Stats(ctx, 1*time.Second, top.Compute())
	must.NoError(t, err)

	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(func() error {
		ru, ok := <-ch
		if !ok {
			return fmt.Errorf("task resource usage channel is closed")
		}
		if ru == nil {
			return fmt.Errorf("task resource usage is nil")
		}
		if ru.ResourceUsage == nil {
			return fmt.Errorf("resourceUsage is nil")
		}
		return nil
	}),
		wait.Timeout(3*time.Second),
		wait.Gap(50*time.Millisecond),
	))

	must.NoError(t, d.DestroyTask(task.ID, true))

	waitCh, err := d.WaitTask(context.Background(), task.ID)
	must.NoError(t, err)
	select {
	case res := <-waitCh:
		if res.Successful() {
			t.Fatalf("should err: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*10) * time.Second):
		t.Fatal("timeout")
	}
}

func setupDockerVolumes(t *testing.T, cfg map[string]interface{}, hostpath string) (*drivers.TaskConfig, *dtestutil.DriverHarness, *TaskConfig, string, func()) {
	testutil.DockerCompatible(t)

	randfn := fmt.Sprintf("test-%d", rand.Int())
	hostfile := filepath.Join(hostpath, randfn)
	var containerPath string
	if runtime.GOOS == "windows" {
		containerPath = "C:\\data"
	} else {
		containerPath = "/mnt/vol"
	}
	containerFile := filepath.Join(containerPath, randfn)

	taskCfg := newTaskConfig([]string{"touch", containerFile})
	taskCfg.Volumes = []string{fmt.Sprintf("%s:%s", hostpath, containerPath)}

	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "ls",
		AllocID:   uuid.Generate(),
		Env:       map[string]string{"VOL_PATH": containerPath},
		Resources: basicResources,
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(taskCfg))

	d := dockerDriverHarness(t, cfg)
	cleanup := d.MkAllocDir(task, true)

	copyImage(t, task.TaskDir(), taskCfg.LoadImage)

	return task, d, &taskCfg, hostfile, cleanup
}

func TestDockerDriver_VolumesDisabled(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	cfg := map[string]interface{}{
		"volumes": map[string]interface{}{
			"enabled": false,
		},
		"gc": map[string]interface{}{
			"image": false,
		},
	}

	{
		tmpvol := t.TempDir()

		task, driver, _, _, cleanup := setupDockerVolumes(t, cfg, tmpvol)
		defer cleanup()

		_, _, err := driver.StartTask(task)
		defer driver.DestroyTask(task.ID, true)
		if err == nil {
			t.Fatal("Started driver successfully when volumes should have been disabled.")
		}
	}

	// Relative paths should still be allowed
	{
		task, driver, _, fn, cleanup := setupDockerVolumes(t, cfg, ".")
		defer cleanup()

		_, _, err := driver.StartTask(task)
		must.NoError(t, err)
		defer driver.DestroyTask(task.ID, true)

		waitCh, err := driver.WaitTask(context.Background(), task.ID)
		must.NoError(t, err)
		select {
		case res := <-waitCh:
			if !res.Successful() {
				t.Fatalf("unexpected err: %v", res)
			}
		case <-time.After(time.Duration(tu.TestMultiplier()*10) * time.Second):
			t.Fatalf("timeout")
		}

		if _, err := os.ReadFile(filepath.Join(task.TaskDir().Dir, fn)); err != nil {
			t.Fatalf("unexpected error reading %s: %v", fn, err)
		}
	}

	// Volume Drivers should be rejected (error)
	{
		task, driver, taskCfg, _, cleanup := setupDockerVolumes(t, cfg, "fake_flocker_vol")
		defer cleanup()

		taskCfg.VolumeDriver = "flocker"
		must.NoError(t, task.EncodeConcreteDriverConfig(taskCfg))

		_, _, err := driver.StartTask(task)
		defer driver.DestroyTask(task.ID, true)
		if err == nil {
			t.Fatal("Started driver successfully when volume drivers should have been disabled.")
		}
	}
}

func TestDockerDriver_VolumesEnabled(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	cfg := map[string]interface{}{
		"volumes": map[string]interface{}{
			"enabled": true,
		},
		"gc": map[string]interface{}{
			"image": false,
		},
	}

	tmpvol := t.TempDir()

	// Evaluate symlinks so it works on MacOS
	tmpvol, err := filepath.EvalSymlinks(tmpvol)
	must.NoError(t, err)

	task, driver, _, hostpath, cleanup := setupDockerVolumes(t, cfg, tmpvol)
	defer cleanup()

	_, _, err = driver.StartTask(task)
	must.NoError(t, err)
	defer driver.DestroyTask(task.ID, true)

	waitCh, err := driver.WaitTask(context.Background(), task.ID)
	must.NoError(t, err)
	select {
	case res := <-waitCh:
		if !res.Successful() {
			t.Fatalf("unexpected err: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*10) * time.Second):
		t.Fatalf("timeout")
	}

	if _, err := os.ReadFile(hostpath); err != nil {
		t.Fatalf("unexpected error reading %s: %v", hostpath, err)
	}
}

func TestDockerDriver_Mounts(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	goodMount := DockerMount{
		Target: "/nomad",
		VolumeOptions: DockerVolumeOptions{
			Labels: map[string]string{"foo": "bar"},
			DriverConfig: DockerVolumeDriverConfig{
				Name: "local",
			},
		},
		ReadOnly: true,
		Source:   "test",
	}

	if runtime.GOOS == "windows" {
		goodMount.Target = "C:\\nomad"
	}

	cases := []struct {
		Name   string
		Mounts []DockerMount
		Error  string
	}{
		{
			Name:   "good-one",
			Error:  "",
			Mounts: []DockerMount{goodMount},
		},
		{
			Name:   "duplicate",
			Error:  "Duplicate mount point",
			Mounts: []DockerMount{goodMount, goodMount, goodMount},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			d := dockerDriverHarness(t, nil)
			driver := d.Impl().(*Driver)
			driver.config.Volumes.Enabled = true

			// Build the task
			task, cfg, _ := dockerTask(t)

			cfg.Command = "sleep"
			cfg.Args = []string{"10000"}
			cfg.Mounts = c.Mounts
			must.NoError(t, task.EncodeConcreteDriverConfig(cfg))
			cleanup := d.MkAllocDir(task, true)
			defer cleanup()

			copyImage(t, task.TaskDir(), cfg.LoadImage)

			_, _, err := d.StartTask(task)
			defer d.DestroyTask(task.ID, true)
			if err == nil && c.Error != "" {
				t.Fatalf("expected error: %v", c.Error)
			} else if err != nil {
				if c.Error == "" {
					t.Fatalf("unexpected error in prestart: %v", err)
				} else if !strings.Contains(err.Error(), c.Error) {
					t.Fatalf("expected error %q; got %v", c.Error, err)
				}
			}
		})
	}
}

func TestDockerDriver_AuthConfiguration(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	path := "./test-resources/docker/auth.json"
	cases := []struct {
		Repo       string
		AuthConfig *registry.AuthConfig
	}{
		{
			Repo:       "lolwhat.com/what:1337",
			AuthConfig: nil,
		},
		{
			Repo: "redis:7",
			AuthConfig: &registry.AuthConfig{
				Auth:          "eyJ1c2VybmFtZSI6InRlc3QiLCJwYXNzd29yZCI6IjEyMzQifQ==",
				Username:      "test",
				Password:      "1234",
				ServerAddress: "https://index.docker.io/v1/",
			},
		},
		{
			Repo: "quay.io/redis:7",
			AuthConfig: &registry.AuthConfig{
				Auth:          "eyJ1c2VybmFtZSI6InRlc3QiLCJwYXNzd29yZCI6IjU2NzgifQ==",
				Username:      "test",
				Password:      "5678",
				ServerAddress: "quay.io",
			},
		},
		{
			Repo: "other.io/redis:7",
			AuthConfig: &registry.AuthConfig{
				Auth:          "eyJ1c2VybmFtZSI6InRlc3QiLCJwYXNzd29yZCI6ImFiY2QifQ==",
				Username:      "test",
				Password:      "abcd",
				ServerAddress: "https://other.io/v1/",
			},
		},
	}

	for _, c := range cases {
		act, err := authFromDockerConfig(path)(c.Repo)
		must.NoError(t, err)
		must.Eq(t, c.AuthConfig, act)
	}
}

func TestDockerDriver_AuthFromTaskConfig(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Auth       DockerAuth
		AuthConfig *registry.AuthConfig
		Desc       string
	}{
		{
			Auth:       DockerAuth{},
			AuthConfig: nil,
			Desc:       "Empty Config",
		},
		{
			Auth: DockerAuth{
				Username:   "foo",
				Password:   "bar",
				ServerAddr: "www.foobar.com",
			},
			AuthConfig: &registry.AuthConfig{
				Auth:          "eyJ1c2VybmFtZSI6ImZvbyIsInBhc3N3b3JkIjoiYmFyIn0=",
				Username:      "foo",
				Password:      "bar",
				ServerAddress: "www.foobar.com",
			},
			Desc: "All fields set",
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			act, err := authFromTaskConfig(&TaskConfig{Auth: c.Auth})("test")
			must.NoError(t, err)
			must.Eq(t, c.AuthConfig, act)
		})
	}
}

func TestDockerDriver_OOMKilled(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	testutil.CgroupsCompatibleV2(t)

	taskCfg := newTaskConfig([]string{"sh", "-c", `sleep 2 && x=a && while true; do x="$x$x"; done`})
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "oom-killed",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	task.Resources.LinuxResources.MemoryLimitBytes = 10 * 1024 * 1024
	task.Resources.NomadResources.Memory.MemoryMB = 10

	must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), taskCfg.LoadImage)

	_, _, err := d.StartTask(task)
	must.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	waitCh, err := d.WaitTask(context.Background(), task.ID)
	must.NoError(t, err)
	select {
	case res := <-waitCh:
		if res.Successful() {
			t.Fatalf("expected error, but container exited successful")
		}

		if !res.OOMKilled {
			t.Fatalf("not killed by OOM killer: %s", res.Err)
		}

		t.Logf("Successfully killed by OOM killer")

	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}
}

func TestDockerDriver_Devices_IsInvalidConfig(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	brokenConfigs := []DockerDevice{
		{
			HostPath: "",
		},
		{
			HostPath:          "/dev/sda1",
			CgroupPermissions: "rxb",
		},
	}

	testCases := []struct {
		deviceConfig []DockerDevice
		err          error
	}{
		{brokenConfigs[:1], fmt.Errorf("host path must be set in configuration for devices")},
		{brokenConfigs[1:], fmt.Errorf("invalid cgroup permission string: \"rxb\"")},
	}

	for _, tc := range testCases {
		task, cfg, _ := dockerTask(t)
		cfg.Devices = tc.deviceConfig
		must.NoError(t, task.EncodeConcreteDriverConfig(cfg))
		d := dockerDriverHarness(t, nil)
		cleanup := d.MkAllocDir(task, true)
		copyImage(t, task.TaskDir(), cfg.LoadImage)
		defer cleanup()

		_, _, err := d.StartTask(task)
		must.Error(t, err)
		must.StrContains(t, err.Error(), tc.err.Error())
	}
}

func TestDockerDriver_Device_Success(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	if runtime.GOOS != "linux" {
		t.Skip("test device mounts only on linux")
	}

	cases := []struct {
		Name     string
		Input    DockerDevice
		Expected container.DeviceMapping
	}{
		{
			Name: "AllSet",
			Input: DockerDevice{
				HostPath:          "/dev/random",
				ContainerPath:     "/dev/hostrandom",
				CgroupPermissions: "rwm",
			},
			Expected: container.DeviceMapping{
				PathOnHost:        "/dev/random",
				PathInContainer:   "/dev/hostrandom",
				CgroupPermissions: "rwm",
			},
		},
		{
			Name: "OnlyHost",
			Input: DockerDevice{
				HostPath: "/dev/random",
			},
			Expected: container.DeviceMapping{
				PathOnHost:        "/dev/random",
				PathInContainer:   "/dev/random",
				CgroupPermissions: "rwm",
			},
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.Name, func(t *testing.T) {
			task, cfg, _ := dockerTask(t)

			cfg.Devices = []DockerDevice{tc.Input}
			must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

			client, driver, handle, cleanup := dockerSetup(t, task, nil)
			defer cleanup()
			must.NoError(t, driver.WaitUntilStarted(task.ID, 5*time.Second))

			container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
			must.NoError(t, err)

			must.SliceNotEmpty(t, container.Container.HostConfig.Devices, must.Sprint("Expected one device"))
			must.Eq(t, tc.Expected, container.Container.HostConfig.Devices[0], must.Sprint("Incorrect device"))
		})
	}
}

func TestDockerDriver_Entrypoint(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	entrypoint := []string{"sh", "-c"}
	task, cfg, _ := dockerTask(t)

	cfg.Entrypoint = entrypoint
	cfg.Command = strings.Join(busyboxLongRunningCmd, " ")
	cfg.Args = []string{}

	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, driver, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()

	must.NoError(t, driver.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)

	must.Len(t, 2, container.Container.Config.Entrypoint, must.Sprint("Expected one entrypoint"))
	must.Eq(t, entrypoint, container.Container.Config.Entrypoint, must.Sprint("Incorrect entrypoint"))
}

func TestDockerDriver_ReadonlyRootfs(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	if runtime.GOOS == "windows" {
		t.Skip("Windows Docker does not support root filesystem in read-only mode")
	}

	task, cfg, _ := dockerTask(t)

	cfg.ReadonlyRootfs = true
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, driver, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, driver.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)

	must.True(t, container.Container.HostConfig.ReadonlyRootfs, must.Sprint("ReadonlyRootfs option not set"))
}

// fakeDockerClient can be used in places that accept an interface for the
// docker client such as createContainer.
type fakeDockerClient struct{}

func (fakeDockerClient) ContainerCreate(context.Context, mclient.ContainerCreateOptions) (mclient.ContainerCreateResult, error) {
	return mclient.ContainerCreateResult{}, fmt.Errorf("duplicate mount point")
}
func (fakeDockerClient) ContainerInspect(context.Context, string, mclient.ContainerInspectOptions) (mclient.ContainerInspectResult, error) {
	panic("not implemented")
}
func (fakeDockerClient) ContainerList(context.Context, mclient.ContainerListOptions) (mclient.ContainerListResult, error) {
	panic("not implemented")
}
func (fakeDockerClient) ContainerRemove(context.Context, string, mclient.ContainerRemoveOptions) (mclient.ContainerRemoveResult, error) {
	panic("not implemented")
}

// TestDockerDriver_VolumeError asserts volume related errors when creating a
// container are recoverable.
func TestDockerDriver_VolumeError(t *testing.T) {
	ci.Parallel(t)

	// setup
	_, cfg, _ := dockerTask(t)

	driver := dockerDriverHarness(t, nil)

	// assert volume error is recoverable
	_, err := driver.Impl().(*Driver).createContainer(fakeDockerClient{}, createContainerOptions{
		Config: &containerapi.Config{}}, cfg.Image)
	must.True(t, structs.IsRecoverable(err))
}

func TestDockerDriver_AdvertiseIPv6Address(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	ctx := context.Background()

	expectedPrefix := "2001:db8:1::242:ac11"
	expectedAdvertise := true
	task, cfg, _ := dockerTask(t)

	cfg.AdvertiseIPv6Addr = expectedAdvertise
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client := newTestDockerClient(t)

	// Make sure IPv6 is enabled
	net, err := client.NetworkInspect(ctx, "bridge", mclient.NetworkInspectOptions{})
	if err != nil {
		t.Skip("error retrieving bridge network information, skipping")
	}
	if !net.Network.EnableIPv6 {
		t.Skip("IPv6 not enabled on bridge network, skipping")
	}

	driver := dockerDriverHarness(t, nil)
	cleanup := driver.MkAllocDir(task, true)
	copyImage(t, task.TaskDir(), cfg.LoadImage)
	defer cleanup()

	_, network, err := driver.StartTask(task)
	defer driver.DestroyTask(task.ID, true)
	must.NoError(t, err)

	must.Eq(t, expectedAdvertise, network.AutoAdvertise,
		must.Sprintf("Wrong autoadvertise. Expect: %v, got: %v", expectedAdvertise, network.AutoAdvertise))

	if !strings.HasPrefix(network.IP, expectedPrefix) {
		t.Fatalf("Got IP address %q want ip address with prefix %q", network.IP, expectedPrefix)
	}

	handle, ok := driver.Impl().(*Driver).tasks.Get(task.ID)
	must.True(t, ok)

	must.NoError(t, driver.WaitUntilStarted(task.ID, time.Second))

	container, err := client.ContainerInspect(ctx, handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)

	found := false
	for _, endpoint := range container.Container.NetworkSettings.Networks {
		if strings.HasPrefix(endpoint.GlobalIPv6Address.String(), expectedPrefix) {
			found = true
			break
		}
	}
	must.True(t, found)
}

func TestDockerImageRef(t *testing.T) {
	ci.Parallel(t)
	tests := []struct {
		Image string
		Repo  string
		Tag   string
	}{
		{"library/hello-world:1.0", "library/hello-world", "1.0"},
		{"library/hello-world:latest", "library/hello-world", "latest"},
		{"library/hello-world@sha256:f5233545e43561214ca4891fd1157e1c3c563316ed8e237750d59bde73361e77", "library/hello-world@sha256:f5233545e43561214ca4891fd1157e1c3c563316ed8e237750d59bde73361e77", ""},
	}
	for _, test := range tests {
		t.Run(test.Image, func(t *testing.T) {
			image := dockerImageRef(test.Repo, test.Tag)
			must.Eq(t, test.Image, image)
		})
	}
}

func waitForExist(t *testing.T, client *client.Client, containerID string) {
	tu.WaitForResult(func() (bool, error) {
		container, err := client.ContainerInspect(context.Background(), containerID, mclient.ContainerInspectOptions{})
		if err != nil {
			if !errdefs.IsNotFound(err) {
				return false, err
			}
		}

		return container.Container.ID != "", nil
	}, func(err error) {
		must.NoError(t, err)
	})
}

// TestDockerDriver_CreationIdempotent asserts that createContainer and
// startContainers functions are idempotent, as we have some retry logic there
// without ensuring we delete/destroy containers
func TestDockerDriver_CreationIdempotent(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	ctx := context.Background()

	task, cfg, _ := dockerTask(t)

	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client := newTestDockerClient(t)
	driver := dockerDriverHarness(t, nil)
	cleanup := driver.MkAllocDir(task, true)
	defer cleanup()

	copyImage(t, task.TaskDir(), cfg.LoadImage)

	d, ok := driver.Impl().(*Driver)
	must.True(t, ok)

	_, _, err := d.createImage(task, cfg, client)
	must.NoError(t, err)

	containerCfg, err := d.createContainerConfig(task, cfg, cfg.Image)
	must.NoError(t, err)

	c, err := d.createContainer(client, containerCfg, cfg.Image)
	must.NoError(t, err)
	defer client.ContainerRemove(ctx, c.Container.ID, mclient.ContainerRemoveOptions{Force: true})

	// calling createContainer again creates a new one and remove old one
	c2, err := d.createContainer(client, containerCfg, cfg.Image)
	must.NoError(t, err)
	defer client.ContainerRemove(ctx, c2.Container.ID, mclient.ContainerRemoveOptions{Force: true})

	must.NotEq(t, c.Container.ID, c2.Container.ID)
	// old container was destroyed
	{
		_, err := client.ContainerInspect(ctx, c.Container.ID, mclient.ContainerInspectOptions{})
		must.Error(t, err)
		must.StrContains(t, err.Error(), NoSuchContainerError)
	}

	// now start container twice
	must.NoError(t, d.startContainer(*c2))
	must.NoError(t, d.startContainer(*c2))

	tu.WaitForResult(func() (bool, error) {
		c, err := client.ContainerInspect(ctx, c2.Container.ID, mclient.ContainerInspectOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get container status: %v", err)
		}

		if !c.Container.State.Running {
			return false, fmt.Errorf("container is not running but %v", c.Container.State)
		}

		return true, nil
	}, func(err error) {
		must.NoError(t, err)
	})
}

// TestDockerDriver_CreateContainerConfig_CPUHardLimit asserts that a default
// CPU quota and period are set when cpu_hard_limit = true.
func TestDockerDriver_CreateContainerConfig_CPUHardLimit(t *testing.T) {
	ci.Parallel(t)

	task, _, _ := dockerTask(t)

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)
	schema, _ := driver.TaskConfigSchema()
	spec, _ := hclspecutils.Convert(schema)

	val, _, _ := hclutils.ParseHclInterface(map[string]interface{}{
		"image":          "foo/bar",
		"cpu_hard_limit": true,
	}, spec, nil)

	must.NoError(t, task.EncodeDriverConfig(val))
	cfg := &TaskConfig{}
	must.NoError(t, task.DecodeDriverConfig(cfg))
	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	must.NoError(t, err)

	must.NonZero(t, c.Host.CPUQuota)
	must.NonZero(t, c.Host.CPUPeriod)
}

func TestDockerDriver_memoryLimits(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name             string
		driverMemoryMB   int64
		taskResources    drivers.MemoryResources
		expectedHard     int64
		expectedReserved int64
	}{
		{
			"plain request",
			0,
			drivers.MemoryResources{MemoryMB: 10},
			10 * 1024 * 1024,
			0,
		},
		{
			"with driver max",
			20,
			drivers.MemoryResources{MemoryMB: 10},
			20 * 1024 * 1024,
			10 * 1024 * 1024,
		},
		{
			"with resources max",
			20,
			drivers.MemoryResources{MemoryMB: 10, MemoryMaxMB: 20},
			20 * 1024 * 1024,
			10 * 1024 * 1024,
		},
		{
			"with driver and resources max: higher driver",
			30,
			drivers.MemoryResources{MemoryMB: 10, MemoryMaxMB: 20},
			30 * 1024 * 1024,
			10 * 1024 * 1024,
		},
		{
			"with driver and resources max: higher resources",
			20,
			drivers.MemoryResources{MemoryMB: 10, MemoryMaxMB: 30},
			30 * 1024 * 1024,
			10 * 1024 * 1024,
		},
		{
			"with reserved-only memory oversubscription",
			20,
			drivers.MemoryResources{MemoryMB: 20, MemoryMaxMB: -1},
			0,
			20 * 1024 * 1024,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hard, reserved := memoryLimits(c.driverMemoryMB, c.taskResources)
			must.Eq(t, c.expectedHard, hard)
			must.Eq(t, c.expectedReserved, reserved)
		})
	}
}

func TestDockerDriver_parseSignal(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		name            string
		runtime         string
		specifiedSignal string
		expectedSignal  string
	}{
		{
			name:            "default",
			runtime:         runtime.GOOS,
			specifiedSignal: "",
			expectedSignal:  "SIGTERM",
		},
		{
			name:            "set",
			runtime:         runtime.GOOS,
			specifiedSignal: "SIGHUP",
			expectedSignal:  "SIGHUP",
		},
		{
			name:            "windows conversion",
			runtime:         "windows",
			specifiedSignal: "SIGINT",
			expectedSignal:  "SIGTERM",
		},
		{
			name:            "not signal",
			runtime:         runtime.GOOS,
			specifiedSignal: "SIGDOESNOTEXIST",
			expectedSignal:  "", // throws error
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := parseSignal(tc.runtime, tc.specifiedSignal)
			if tc.expectedSignal == "" {
				must.Error(t, err, must.Sprint("invalid signal"))
			} else {
				must.NoError(t, err)
				must.Eq(t, s.(syscall.Signal).String(), s.String())
			}
		})
	}
}

// This test asserts that Nomad isn't overriding the STOPSIGNAL in a Dockerfile
func TestDockerDriver_StopSignal(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)
	if runtime.GOOS == "windows" || runtime.GOARCH == "arm64" {
		t.Skip("Skipped on windows or arm64, we don't have image variants available")
	}

	cases := []struct {
		name            string
		imageName       string
		imageLoad       string
		variant         string
		jobKillSignal   string
		expectedSignals []string
	}{
		{
			name:            "stopsignal-only",
			imageName:       "busybox:1.29.3-stopsignal",
			imageLoad:       "busybox_stopsignal.tar",
			jobKillSignal:   "",
			expectedSignals: []string{"19", "9"},
		},
		{
			name:            "stopsignal-killsignal",
			imageName:       "busybox:1.29.3-stopsignal",
			imageLoad:       "busybox_stopsignal.tar",
			jobKillSignal:   "SIGTERM",
			expectedSignals: []string{"15", "19", "9"},
		},
		{
			name:            "killsignal-only",
			variant:         "",
			jobKillSignal:   "SIGTERM",
			expectedSignals: []string{"15", "15", "9"},
		},
		{
			name:            "nosignals-default",
			variant:         "",
			jobKillSignal:   "",
			expectedSignals: []string{"15", "9"},
		},
	}

	for i := range cases {
		c := cases[i]
		t.Run(c.name, func(t *testing.T) {
			taskCfg := newTaskConfig([]string{"sleep", "9901"})
			if c.imageLoad != "" {
				taskCfg.Image = c.imageName
				taskCfg.LoadImage = c.imageLoad
			}

			task := &drivers.TaskConfig{
				ID:        uuid.Generate(),
				Name:      c.name,
				AllocID:   uuid.Generate(),
				Resources: basicResources,
			}
			must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

			d := dockerDriverHarness(t, nil)
			cleanup := d.MkAllocDir(task, true)
			defer cleanup()

			copyImage(t, task.TaskDir(), taskCfg.LoadImage)

			client := newTestDockerClient(t)

			ctx, cancel := context.WithCancel(context.Background())
			eventsResult := client.Events(ctx, mclient.EventsListOptions{})
			defer cancel()

			_, _, err := d.StartTask(task)
			must.NoError(t, err)
			must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

			stopErr := make(chan error, 1)
			go func() {
				err := d.StopTask(task.ID, 1*time.Second, c.jobKillSignal)
				stopErr <- err
			}()

			timeout := time.After(10 * time.Second)
			var receivedSignals []string
		WAIT:
			for {
				select {
				case msg := <-eventsResult.Messages:
					// Only add kill signals
					if msg.Action == "kill" {
						sig := msg.Actor.Attributes["signal"]
						receivedSignals = append(receivedSignals, sig)

						if reflect.DeepEqual(receivedSignals, c.expectedSignals) {
							break WAIT
						}
					}
				case err := <-eventsResult.Err:
					if err != nil && err != context.Canceled {
						must.NoError(t, err)
					}
				case err := <-stopErr:
					must.NoError(t, err, must.Sprint("stop task failed"))
				case <-timeout:
					// timeout waiting for signals
					must.Eq(t, c.expectedSignals, receivedSignals, must.Sprint("timed out waiting for expected signals"))
				}
			}
		})
	}
}

func TestDockerDriver_GroupAdd(t *testing.T) {
	if !tu.IsCI() {
		ci.Parallel(t)
	}
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
	cfg.GroupAdd = []string{"12345", "9999"}
	must.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task, nil)
	defer cleanup()
	must.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.ContainerInspect(context.Background(), handle.containerID, mclient.ContainerInspectOptions{})
	must.NoError(t, err)

	must.Eq(t, cfg.GroupAdd, container.Container.HostConfig.GroupAdd)
}

// TestDockerDriver_CollectStats verifies that the TaskStats API collects stats
// periodically and that these values are non-zero as expected
func TestDockerDriver_CollectStats(t *testing.T) {
	ci.Parallel(t)
	testutil.RequireLinux(t) // stats outputs are different on Windows
	testutil.DockerCompatible(t)

	// we want to generate at least some CPU usage
	args := []string{"/bin/sh", "-c", "cat /dev/urandom | base64 > /dev/null"}
	taskCfg := newTaskConfig(args)
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "nc-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	plugin, ok := d.Impl().(*Driver)
	must.True(t, ok)
	plugin.compute.TotalCompute = 1000
	plugin.compute.NumCores = 1

	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), taskCfg.LoadImage)

	_, _, err := d.StartTask(task)
	must.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	// this test has to run for a while because the minimum stats interval we
	// can get from Docker is 1s
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	recv, err := d.TaskStats(ctx, task.ID, time.Second)
	must.NoError(t, err)

	statsReceived := 0
	tickValues := set.From([]float64{})

DONE:
	for {
		select {
		case stats := <-recv:
			if stats == nil {
				// prevent NPE when channel close races with context close
				continue
			}
			statsReceived++
			ticks := stats.ResourceUsage.CpuStats.TotalTicks
			must.Greater(t, 0, ticks)
			tickValues.Insert(ticks)
			rss := stats.ResourceUsage.MemoryStats.RSS
			must.Greater(t, 0, rss)
			if statsReceived >= 3 {
				cancel() // 3 is plenty
			}
		case <-ctx.Done():
			break DONE
		}
	}

	// CPU stats should be changed with every interval
	must.Len(t, statsReceived, tickValues.Slice())
}
