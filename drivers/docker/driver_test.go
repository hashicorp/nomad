package docker

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/devices/gpu/nvidia"
	"github.com/hashicorp/nomad/helper/freeport"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func dockerIsRemote(t *testing.T) bool {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		return false
	}

	// Technically this could be a local tcp socket but for testing purposes
	// we'll just assume that tcp is only used for remote connections.
	if client.Endpoint()[0:3] == "tcp" {
		return true
	}
	return false
}

var (
	// busyboxLongRunningCmd is a busybox command that runs indefinitely, and
	// ideally responds to SIGINT/SIGTERM.  Sadly, busybox:1.29.3 /bin/sleep doesn't.
	busyboxLongRunningCmd = []string{"nc", "-l", "-p", "3000", "127.0.0.1"}
)

// Returns a task with a reserved and dynamic port. The ports are returned
// respectively, and should be reclaimed with freeport.Return at the end of a test.
func dockerTask(t *testing.T) (*drivers.TaskConfig, *TaskConfig, []int) {
	ports := freeport.MustTake(2)
	dockerReserved := ports[0]
	dockerDynamic := ports[1]

	cfg := newTaskConfig("", busyboxLongRunningCmd)
	task := &drivers.TaskConfig{
		ID:      uuid.Generate(),
		Name:    "redis-demo",
		AllocID: uuid.Generate(),
		Env: map[string]string{
			"test": t.Name(),
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

	require.NoError(t, task.EncodeConcreteDriverConfig(&cfg))

	return task, &cfg, ports
}

// dockerSetup does all of the basic setup you need to get a running docker
// process up and running for testing. Use like:
//
//	task := taskTemplate()
//	// do custom task configuration
//	client, handle, cleanup := dockerSetup(t, task)
//	defer cleanup()
//	// do test stuff
//
// If there is a problem during setup this function will abort or skip the test
// and indicate the reason.
func dockerSetup(t *testing.T, task *drivers.TaskConfig) (*docker.Client, *dtestutil.DriverHarness, *taskHandle, func()) {
	client := newTestDockerClient(t)
	driver := dockerDriverHarness(t, nil)
	cleanup := driver.MkAllocDir(task, true)

	copyImage(t, task.TaskDir(), "busybox.tar")
	_, _, err := driver.StartTask(task)
	require.NoError(t, err)

	dockerDriver, ok := driver.Impl().(*Driver)
	require.True(t, ok)
	handle, ok := dockerDriver.tasks.Get(task.ID)
	require.True(t, ok)

	return client, driver, handle, func() {
		driver.DestroyTask(task.ID, true)
		cleanup()
	}
}

// cleanSlate removes the specified docker image, including potentially stopping/removing any
// containers based on that image. This is used to decouple tests that would be coupled
// by using the same container image.
func cleanSlate(client *docker.Client, imageID string) {
	if img, _ := client.InspectImage(imageID); img == nil {
		return
	}
	containers, _ := client.ListContainers(docker.ListContainersOptions{
		All: true,
		Filters: map[string][]string{
			"ancestor": {imageID},
		},
	})
	for _, c := range containers {
		client.RemoveContainer(docker.RemoveContainerOptions{
			Force: true,
			ID:    c.ID,
		})
	}
	client.RemoveImageExtended(imageID, docker.RemoveImageOptions{
		Force: true,
	})
	return
}

// dockerDriverHarness wires up everything needed to launch a task with a docker driver.
// A driver plugin interface and cleanup function is returned
func dockerDriverHarness(t *testing.T, cfg map[string]interface{}) *dtestutil.DriverHarness {
	logger := testlog.HCLogger(t)
	harness := dtestutil.NewDriverHarness(t, NewDockerDriver(logger))
	if cfg == nil {
		cfg = map[string]interface{}{
			"gc": map[string]interface{}{
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
				Factory: func(hclog.Logger) interface{} {
					return harness
				},
			},
		},
	})

	require.NoError(t, err)
	instance, err := plugLoader.Dispense(pluginName, base.PluginTypeDriver, nil, logger)
	require.NoError(t, err)
	driver, ok := instance.Plugin().(*dtestutil.DriverHarness)
	if !ok {
		t.Fatal("plugin instance is not a driver... wat?")
	}

	return driver
}

func newTestDockerClient(t *testing.T) *docker.Client {
	t.Helper()
	testutil.DockerCompatible(t)

	client, err := docker.NewClientFromEnv()
	if err != nil {
		t.Fatalf("Failed to initialize client: %s\nStack\n%s", err, debug.Stack())
	}
	return client
}

/*
// This test should always pass, even if docker daemon is not available
func TestDockerDriver_Fingerprint(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}

	ctx := testDockerDriverContexts(t, &structs.Task{Name: "foo", Driver: "docker", Resources: basicResources})
	//ctx.DriverCtx.config.Options = map[string]string{"docker.cleanup.image": "false"}
	defer ctx.Destroy()
	d := NewDockerDriver(ctx.DriverCtx)
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	request := &fingerprint.FingerprintRequest{Config: &config.Config{}, Node: node}
	var response fingerprint.FingerprintResponse
	err := d.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	attributes := response.Attributes
	if testutil.DockerIsConnected(t) && attributes["driver.docker"] == "" {
		t.Fatalf("Fingerprinter should detect when docker is available")
	}

	if attributes["driver.docker"] != "1" {
		t.Log("Docker daemon not available. The remainder of the docker tests will be skipped.")
	} else {

		// if docker is available, make sure that the response is tagged as
		// applicable
		if !response.Detected {
			t.Fatalf("expected response to be applicable")
		}
	}

	t.Logf("Found docker version %s", attributes["driver.docker.version"])
}

// TestDockerDriver_Fingerprint_Bridge asserts that if Docker is running we set
// the bridge network's IP as a node attribute. See #2785
func TestDockerDriver_Fingerprint_Bridge(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)
	if runtime.GOOS != "linux" {
		t.Skip("expect only on linux")
	}

	// This seems fragile, so we might need to reconsider this test if it
	// proves flaky
	expectedAddr, err := sockaddr.GetInterfaceIP("docker0")
	if err != nil {
		t.Fatalf("unable to get ip for docker0: %v", err)
	}
	if expectedAddr == "" {
		t.Fatalf("unable to get ip for docker bridge")
	}

	conf := testConfig(t)
	conf.Node = mock.Node()
	dd := NewDockerDriver(NewDriverContext("", "", "", "", conf, conf.Node, testlog.Logger(t), nil))

	request := &fingerprint.FingerprintRequest{Config: conf, Node: conf.Node}
	var response fingerprint.FingerprintResponse

	err = dd.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("error fingerprinting docker: %v", err)
	}

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	attributes := response.Attributes
	if attributes == nil {
		t.Fatalf("expected attributes to be set")
	}

	if attributes["driver.docker"] == "" {
		t.Fatalf("expected Docker to be enabled but false was returned")
	}

	if found := attributes["driver.docker.bridge_ip"]; found != expectedAddr {
		t.Fatalf("expected bridge ip %q but found: %q", expectedAddr, found)
	}
	t.Logf("docker bridge ip: %q", attributes["driver.docker.bridge_ip"])
}

func TestDockerDriver_Check_DockerHealthStatus(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)
	if runtime.GOOS != "linux" {
		t.Skip("expect only on linux")
	}

	require := require.New(t)

	expectedAddr, err := sockaddr.GetInterfaceIP("docker0")
	if err != nil {
		t.Fatalf("unable to get ip for docker0: %v", err)
	}
	if expectedAddr == "" {
		t.Fatalf("unable to get ip for docker bridge")
	}

	conf := testConfig(t)
	conf.Node = mock.Node()
	dd := NewDockerDriver(NewDriverContext("", "", "", "", conf, conf.Node, testlog.Logger(t), nil))

	request := &cstructs.HealthCheckRequest{}
	var response cstructs.HealthCheckResponse

	dc, ok := dd.(fingerprint.HealthCheck)
	require.True(ok)
	err = dc.HealthCheck(request, &response)
	require.Nil(err)

	driverInfo := response.Drivers["docker"]
	require.NotNil(driverInfo)
	require.True(driverInfo.Healthy)
}*/

func TestDockerDriver_Start_Wait(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	taskCfg := newTaskConfig("", busyboxLongRunningCmd)
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "nc-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	_, _, err := d.StartTask(task)
	require.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	// Attempt to wait
	waitCh, err := d.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)

	select {
	case <-waitCh:
		t.Fatalf("wait channel should not have received an exit result")
	case <-time.After(time.Duration(tu.TestMultiplier()*1) * time.Second):
	}
}

func TestDockerDriver_Start_WaitFinish(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	taskCfg := newTaskConfig("", []string{"echo", "hello"})
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "nc-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	_, _, err := d.StartTask(task)
	require.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	// Attempt to wait
	waitCh, err := d.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)

	select {
	case res := <-waitCh:
		if !res.Successful() {
			require.Fail(t, "ExitResult should be successful: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		require.Fail(t, "timeout")
	}
}

// TestDockerDriver_Start_StoppedContainer asserts that Nomad will detect a
// stopped task container, remove it, and start a new container.
//
// See https://github.com/hashicorp/nomad/issues/3419
func TestDockerDriver_Start_StoppedContainer(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	taskCfg := newTaskConfig("", []string{"sleep", "9001"})
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "nc-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	client := newTestDockerClient(t)

	var imageID string
	var err error

	if runtime.GOOS != "windows" {
		imageID, err = d.Impl().(*Driver).loadImage(task, &taskCfg, client)
	} else {
		image, lErr := client.InspectImage("dantoml/busybox-windows:08012019")
		err = lErr
		if image != nil {
			imageID = image.ID
		}
	}
	require.NoError(t, err)
	require.NotEmpty(t, imageID)

	// Create a container of the same name but don't start it. This mimics
	// the case of dockerd getting restarted and stopping containers while
	// Nomad is watching them.
	opts := docker.CreateContainerOptions{
		Name: strings.Replace(task.ID, "/", "_", -1),
		Config: &docker.Config{
			Image: taskCfg.Image,
			Cmd:   []string{"sleep", "9000"},
			Env:   []string{fmt.Sprintf("test=%s", t.Name())},
		},
	}

	if _, err := client.CreateContainer(opts); err != nil {
		t.Fatalf("error creating initial container: %v", err)
	}

	_, _, err = d.StartTask(task)
	defer d.DestroyTask(task.ID, true)
	require.NoError(t, err)

	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))
	require.NoError(t, d.DestroyTask(task.ID, true))
}

func TestDockerDriver_Start_LoadImage(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	taskCfg := newTaskConfig("", []string{"sh", "-c", "echo hello > $NOMAD_TASK_DIR/output"})
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "busybox-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	_, _, err := d.StartTask(task)
	require.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	waitCh, err := d.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)
	select {
	case res := <-waitCh:
		if !res.Successful() {
			require.Fail(t, "ExitResult should be successful: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		require.Fail(t, "timeout")
	}

	// Check that data was written to the shared alloc directory.
	outputFile := filepath.Join(task.TaskDir().LocalDir, "output")
	act, err := ioutil.ReadFile(outputFile)
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
	if !tu.IsCI() {
		t.Parallel()
	}
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
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, false)
	defer cleanup()

	_, _, err := d.StartTask(task)
	require.Error(t, err)
	require.Contains(t, err.Error(), "image name required")

	d.DestroyTask(task.ID, true)
}

func TestDockerDriver_Start_BadPull_Recoverable(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	taskCfg := TaskConfig{
		Image:   "127.0.0.1:32121/foo", // bad path
		Command: "echo",
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
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()

	_, _, err := d.StartTask(task)
	require.Error(t, err)

	defer d.DestroyTask(task.ID, true)

	if rerr, ok := err.(*structs.RecoverableError); !ok {
		t.Fatalf("want recoverable error: %+v", err)
	} else if !rerr.IsRecoverable() {
		t.Fatalf("error not recoverable: %+v", err)
	}
}

func TestDockerDriver_Start_Wait_AllocDir(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	// This test requires that the alloc dir be mounted into docker as a volume.
	// Because this cannot happen when docker is run remotely, e.g. when running
	// docker in a VM, we skip this when we detect Docker is being run remotely.
	if !testutil.DockerIsConnected(t) || dockerIsRemote(t) {
		t.Skip("Docker not connected")
	}

	exp := []byte{'w', 'i', 'n'}
	file := "output.txt"

	taskCfg := newTaskConfig("", []string{
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
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	_, _, err := d.StartTask(task)
	require.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	// Attempt to wait
	waitCh, err := d.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)

	select {
	case res := <-waitCh:
		if !res.Successful() {
			require.Fail(t, fmt.Sprintf("ExitResult should be successful: %v", res))
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		require.Fail(t, "timeout")
	}

	// Check that data was written to the shared alloc directory.
	outputFile := filepath.Join(task.TaskDir().SharedAllocDir, file)
	act, err := ioutil.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Couldn't read expected output: %v", err)
	}

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("Command outputted %v; want %v", act, exp)
	}
}

func TestDockerDriver_Start_Kill_Wait(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	taskCfg := newTaskConfig("", busyboxLongRunningCmd)
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "busybox-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	_, _, err := d.StartTask(task)
	require.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	go func(t *testing.T) {
		time.Sleep(100 * time.Millisecond)
		signal := "SIGINT"
		if runtime.GOOS == "windows" {
			signal = "SIGKILL"
		}
		require.NoError(t, d.StopTask(task.ID, time.Second, signal))
	}(t)

	// Attempt to wait
	waitCh, err := d.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)

	select {
	case res := <-waitCh:
		if res.Successful() {
			require.Fail(t, "ExitResult should err: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		require.Fail(t, "timeout")
	}
}

func TestDockerDriver_Start_KillTimeout(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	if runtime.GOOS == "windows" {
		t.Skip("Windows Docker does not support SIGUSR1")
	}

	timeout := 2 * time.Second
	taskCfg := newTaskConfig("", []string{"sleep", "10"})
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "busybox-demo",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	_, _, err := d.StartTask(task)
	require.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	var killSent time.Time
	go func() {
		time.Sleep(100 * time.Millisecond)
		killSent = time.Now()
		require.NoError(t, d.StopTask(task.ID, timeout, "SIGUSR1"))
	}()

	// Attempt to wait
	waitCh, err := d.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)

	var killed time.Time
	select {
	case <-waitCh:
		killed = time.Now()
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		require.Fail(t, "timeout")
	}

	require.True(t, killed.Sub(killSent) > timeout)
}

func TestDockerDriver_StartN(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows Docker does not support SIGINT")
	}
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)
	require := require.New(t)

	task1, _, ports1 := dockerTask(t)
	defer freeport.Return(ports1)

	task2, _, ports2 := dockerTask(t)
	defer freeport.Return(ports2)

	task3, _, ports3 := dockerTask(t)
	defer freeport.Return(ports3)

	taskList := []*drivers.TaskConfig{task1, task2, task3}

	t.Logf("Starting %d tasks", len(taskList))

	d := dockerDriverHarness(t, nil)
	// Let's spin up a bunch of things
	for _, task := range taskList {
		cleanup := d.MkAllocDir(task, true)
		defer cleanup()
		copyImage(t, task.TaskDir(), "busybox.tar")
		_, _, err := d.StartTask(task)
		require.NoError(err)

	}

	defer d.DestroyTask(task3.ID, true)
	defer d.DestroyTask(task2.ID, true)
	defer d.DestroyTask(task1.ID, true)

	t.Log("All tasks are started. Terminating...")
	for _, task := range taskList {
		require.NoError(d.StopTask(task.ID, time.Second, "SIGINT"))

		// Attempt to wait
		waitCh, err := d.WaitTask(context.Background(), task.ID)
		require.NoError(err)

		select {
		case <-waitCh:
		case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
			require.Fail("timeout waiting on task")
		}
	}

	t.Log("Test complete!")
}

func TestDockerDriver_StartNVersions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipped on windows, we don't have image variants available")
	}
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)
	require := require.New(t)

	task1, cfg1, ports1 := dockerTask(t)
	defer freeport.Return(ports1)
	tcfg1 := newTaskConfig("", []string{"echo", "hello"})
	cfg1.Image = tcfg1.Image
	cfg1.LoadImage = tcfg1.LoadImage
	require.NoError(task1.EncodeConcreteDriverConfig(cfg1))

	task2, cfg2, ports2 := dockerTask(t)
	defer freeport.Return(ports2)
	tcfg2 := newTaskConfig("musl", []string{"echo", "hello"})
	cfg2.Image = tcfg2.Image
	cfg2.LoadImage = tcfg2.LoadImage
	require.NoError(task2.EncodeConcreteDriverConfig(cfg2))

	task3, cfg3, ports3 := dockerTask(t)
	defer freeport.Return(ports3)
	tcfg3 := newTaskConfig("glibc", []string{"echo", "hello"})
	cfg3.Image = tcfg3.Image
	cfg3.LoadImage = tcfg3.LoadImage
	require.NoError(task3.EncodeConcreteDriverConfig(cfg3))

	taskList := []*drivers.TaskConfig{task1, task2, task3}

	t.Logf("Starting %d tasks", len(taskList))
	d := dockerDriverHarness(t, nil)

	// Let's spin up a bunch of things
	for _, task := range taskList {
		cleanup := d.MkAllocDir(task, true)
		defer cleanup()
		copyImage(t, task.TaskDir(), "busybox.tar")
		copyImage(t, task.TaskDir(), "busybox_musl.tar")
		copyImage(t, task.TaskDir(), "busybox_glibc.tar")
		_, _, err := d.StartTask(task)
		require.NoError(err)

		require.NoError(d.WaitUntilStarted(task.ID, 5*time.Second))
	}

	defer d.DestroyTask(task3.ID, true)
	defer d.DestroyTask(task2.ID, true)
	defer d.DestroyTask(task1.ID, true)

	t.Log("All tasks are started. Terminating...")
	for _, task := range taskList {
		require.NoError(d.StopTask(task.ID, time.Second, "SIGINT"))

		// Attempt to wait
		waitCh, err := d.WaitTask(context.Background(), task.ID)
		require.NoError(err)

		select {
		case <-waitCh:
		case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
			require.Fail("timeout waiting on task")
		}
	}

	t.Log("Test complete!")
}

func TestDockerDriver_Labels(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)

	cfg.Labels = map[string]string{
		"label1": "value1",
		"label2": "value2",
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.containerID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// expect to see 1 additional standard labels
	require.Equal(t, len(cfg.Labels)+1, len(container.Config.Labels))
	for k, v := range cfg.Labels {
		require.Equal(t, v, container.Config.Labels[k])
	}
}

func TestDockerDriver_ForcePull(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)

	cfg.ForcePull = true
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	_, err := client.InspectContainer(handle.containerID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestDockerDriver_ForcePull_RepoDigest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO: Skipped digest test on Windows")
	}

	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.LoadImage = ""
	cfg.Image = "library/busybox@sha256:58ac43b2cc92c687a32c8be6278e50a063579655fe3090125dcb2af0ff9e1a64"
	localDigest := "sha256:8ac48589692a53a9b8c2d1ceaa6b402665aa7fe667ba51ccc03002300856d8c7"
	cfg.ForcePull = true
	cfg.Command = busyboxLongRunningCmd[0]
	cfg.Args = busyboxLongRunningCmd[1:]
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)
	require.Equal(t, localDigest, container.Image)
}

func TestDockerDriver_SecurityOptUnconfined(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support seccomp")
	}
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.SecurityOpt = []string{"seccomp=unconfined"}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.containerID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	require.Exactly(t, cfg.SecurityOpt, container.HostConfig.SecurityOpt)
}

func TestDockerDriver_SecurityOptFromFile(t *testing.T) {

	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support seccomp")
	}
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.SecurityOpt = []string{"seccomp=./test-resources/docker/seccomp.json"}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)

	require.Contains(t, container.HostConfig.SecurityOpt[0], "reboot")
}

func TestDockerDriver_CreateContainerConfig(t *testing.T) {
	t.Parallel()

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	opt := map[string]string{"size": "120G"}

	cfg.StorageOpt = opt
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)

	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	require.NoError(t, err)

	require.Equal(t, "org/repo:0.1", c.Config.Image)
	require.EqualValues(t, opt, c.HostConfig.StorageOpt)

	// Container name should be /<task_name>-<alloc_id> for backward compat
	containerName := fmt.Sprintf("%s-%s", strings.Replace(task.Name, "/", "_", -1), task.AllocID)
	require.Equal(t, containerName, c.Name)
}

func TestDockerDriver_CreateContainerConfig_User(t *testing.T) {
	t.Parallel()

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	task.User = "random-user-1"

	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)

	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	require.NoError(t, err)

	require.Equal(t, task.User, c.Config.User)
}

func TestDockerDriver_CreateContainerConfig_Labels(t *testing.T) {
	t.Parallel()

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	task.AllocID = uuid.Generate()
	task.JobName = "redis-demo-job"

	cfg.Labels = map[string]string{
		"user_label": "user_value",

		// com.hashicorp.nomad. labels are reserved and
		// cannot be overridden
		"com.hashicorp.nomad.alloc_id": "bad_value",
	}

	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)

	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	require.NoError(t, err)

	expectedLabels := map[string]string{
		// user provided labels
		"user_label": "user_value",
		// default labels
		"com.hashicorp.nomad.alloc_id": task.AllocID,
	}

	require.Equal(t, expectedLabels, c.Config.Labels)
}

func TestDockerDriver_CreateContainerConfig_Logging(t *testing.T) {
	t.Parallel()

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
			task, cfg, ports := dockerTask(t)
			defer freeport.Return(ports)

			cfg.Logging = c.loggingConfig
			require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

			dh := dockerDriverHarness(t, nil)
			driver := dh.Impl().(*Driver)

			cc, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
			require.NoError(t, err)

			require.Equal(t, c.expectedConfig.Type, cc.HostConfig.LogConfig.Type)
			require.Equal(t, c.expectedConfig.Config["max-file"], cc.HostConfig.LogConfig.Config["max-file"])
			require.Equal(t, c.expectedConfig.Config["max-size"], cc.HostConfig.LogConfig.Config["max-size"])
		})
	}
}

func TestDockerDriver_CreateContainerConfigWithRuntimes(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}
	if runtime.GOOS != "linux" {
		t.Skip("nvidia plugin supports only linux")
	}
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
			task, cfg, ports := dockerTask(t)
			defer freeport.Return(ports)

			dh := dockerDriverHarness(t, nil)
			driver := dh.Impl().(*Driver)

			driver.gpuRuntime = testCase.gpuRuntimeSet
			driver.config.GPURuntimeName = testCase.expectedRuntime
			if testCase.nvidiaDevicesProvided {
				task.DeviceEnv[nvidia.NvidiaVisibleDevices] = "GPU_UUID_1"
			}

			c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
			if testCase.expectToReturnError {
				require.NotNil(t, err)
			} else {
				require.NoError(t, err)
				if testCase.nvidiaDevicesProvided {
					require.Equal(t, testCase.expectedRuntime, c.HostConfig.Runtime)
				} else {
					// no nvidia devices provided -> no point to use nvidia runtime
					require.Equal(t, "", c.HostConfig.Runtime)
				}
			}
		})
	}
}

func TestDockerDriver_Capabilities(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)
	if runtime.GOOS == "windows" {
		t.Skip("Capabilities not supported on windows")
	}

	testCases := []struct {
		Name       string
		CapAdd     []string
		CapDrop    []string
		Whitelist  string
		StartError string
	}{
		{
			Name:    "default-whitelist-add-allowed",
			CapAdd:  []string{"fowner", "mknod"},
			CapDrop: []string{"all"},
		},
		{
			Name:       "default-whitelist-add-forbidden",
			CapAdd:     []string{"net_admin"},
			StartError: "net_admin",
		},
		{
			Name:    "default-whitelist-drop-existing",
			CapDrop: []string{"fowner", "mknod"},
		},
		{
			Name:      "restrictive-whitelist-drop-all",
			CapDrop:   []string{"all"},
			Whitelist: "fowner,mknod",
		},
		{
			Name:      "restrictive-whitelist-add-allowed",
			CapAdd:    []string{"fowner", "mknod"},
			CapDrop:   []string{"all"},
			Whitelist: "fowner,mknod",
		},
		{
			Name:       "restrictive-whitelist-add-forbidden",
			CapAdd:     []string{"net_admin", "mknod"},
			CapDrop:    []string{"all"},
			Whitelist:  "fowner,mknod",
			StartError: "net_admin",
		},
		{
			Name:      "permissive-whitelist",
			CapAdd:    []string{"net_admin", "mknod"},
			Whitelist: "all",
		},
		{
			Name:      "permissive-whitelist-add-all",
			CapAdd:    []string{"all"},
			Whitelist: "all",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			client := newTestDockerClient(t)
			task, cfg, ports := dockerTask(t)
			defer freeport.Return(ports)

			if len(tc.CapAdd) > 0 {
				cfg.CapAdd = tc.CapAdd
			}
			if len(tc.CapDrop) > 0 {
				cfg.CapDrop = tc.CapDrop
			}
			require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

			d := dockerDriverHarness(t, nil)
			dockerDriver, ok := d.Impl().(*Driver)
			require.True(t, ok)
			if tc.Whitelist != "" {
				dockerDriver.config.AllowCaps = strings.Split(tc.Whitelist, ",")
			}

			cleanup := d.MkAllocDir(task, true)
			defer cleanup()
			copyImage(t, task.TaskDir(), "busybox.tar")

			_, _, err := d.StartTask(task)
			defer d.DestroyTask(task.ID, true)
			if err == nil && tc.StartError != "" {
				t.Fatalf("Expected error in start: %v", tc.StartError)
			} else if err != nil {
				if tc.StartError == "" {
					require.NoError(t, err)
				} else {
					require.Contains(t, err.Error(), tc.StartError)
				}
				return
			}

			handle, ok := dockerDriver.tasks.Get(task.ID)
			require.True(t, ok)

			require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

			container, err := client.InspectContainer(handle.containerID)
			require.NoError(t, err)

			require.Exactly(t, tc.CapAdd, container.HostConfig.CapAdd)
			require.Exactly(t, tc.CapDrop, container.HostConfig.CapDrop)
		})
	}
}

func TestDockerDriver_DNS(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.DNSServers = []string{"8.8.8.8", "8.8.4.4"}
	cfg.DNSSearchDomains = []string{"example.com", "example.org", "example.net"}
	cfg.DNSOptions = []string{"ndots:1"}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)

	require.Exactly(t, cfg.DNSServers, container.HostConfig.DNS)
	require.Exactly(t, cfg.DNSSearchDomains, container.HostConfig.DNSSearch)
	require.Exactly(t, cfg.DNSOptions, container.HostConfig.DNSOptions)
}

func TestDockerDriver_MACAddress(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)
	if runtime.GOOS == "windows" {
		t.Skip("Windows docker does not support setting MacAddress")
	}

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.MacAddress = "00:16:3e:00:00:00"
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)

	require.Equal(t, cfg.MacAddress, container.NetworkSettings.MacAddress)
}

func TestDockerWorkDir(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.WorkDir = "/some/path"
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)
	require.Equal(t, cfg.WorkDir, filepath.ToSlash(container.Config.WorkingDir))
}

func inSlice(needle string, haystack []string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func TestDockerDriver_PortsNoMap(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, _, ports := dockerTask(t)
	defer freeport.Return(ports)
	res := ports[0]
	dyn := ports[1]

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)

	// Verify that the correct ports are EXPOSED
	expectedExposedPorts := map[docker.Port]struct{}{
		docker.Port(fmt.Sprintf("%d/tcp", res)): {},
		docker.Port(fmt.Sprintf("%d/udp", res)): {},
		docker.Port(fmt.Sprintf("%d/tcp", dyn)): {},
		docker.Port(fmt.Sprintf("%d/udp", dyn)): {},
	}

	require.Exactly(t, expectedExposedPorts, container.Config.ExposedPorts)

	hostIP := "127.0.0.1"
	if runtime.GOOS == "windows" {
		hostIP = ""
	}

	// Verify that the correct ports are FORWARDED
	expectedPortBindings := map[docker.Port][]docker.PortBinding{
		docker.Port(fmt.Sprintf("%d/tcp", res)): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", res)}},
		docker.Port(fmt.Sprintf("%d/udp", res)): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", res)}},
		docker.Port(fmt.Sprintf("%d/tcp", dyn)): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", dyn)}},
		docker.Port(fmt.Sprintf("%d/udp", dyn)): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", dyn)}},
	}

	require.Exactly(t, expectedPortBindings, container.HostConfig.PortBindings)
}

func TestDockerDriver_PortsMapping(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	res := ports[0]
	dyn := ports[1]
	cfg.PortMap = map[string]int{
		"main":  8080,
		"REDIS": 6379,
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)

	// Verify that the port environment variables are set
	require.Contains(t, container.Config.Env, "NOMAD_PORT_main=8080")
	require.Contains(t, container.Config.Env, "NOMAD_PORT_REDIS=6379")

	// Verify that the correct ports are EXPOSED
	expectedExposedPorts := map[docker.Port]struct{}{
		docker.Port("8080/tcp"): {},
		docker.Port("8080/udp"): {},
		docker.Port("6379/tcp"): {},
		docker.Port("6379/udp"): {},
	}

	require.Exactly(t, expectedExposedPorts, container.Config.ExposedPorts)

	hostIP := "127.0.0.1"
	if runtime.GOOS == "windows" {
		hostIP = ""
	}

	// Verify that the correct ports are FORWARDED
	expectedPortBindings := map[docker.Port][]docker.PortBinding{
		docker.Port("8080/tcp"): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", res)}},
		docker.Port("8080/udp"): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", res)}},
		docker.Port("6379/tcp"): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", dyn)}},
		docker.Port("6379/udp"): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", dyn)}},
	}
	require.Exactly(t, expectedPortBindings, container.HostConfig.PortBindings)
}

func TestDockerDriver_CreateContainerConfig_PortsMapping(t *testing.T) {
	t.Parallel()

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	res := ports[0]
	dyn := ports[1]
	cfg.PortMap = map[string]int{
		"main":  8080,
		"REDIS": 6379,
	}
	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)

	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	require.NoError(t, err)

	require.Equal(t, "org/repo:0.1", c.Config.Image)
	require.Contains(t, c.Config.Env, "NOMAD_PORT_main=8080")
	require.Contains(t, c.Config.Env, "NOMAD_PORT_REDIS=6379")

	// Verify that the correct ports are FORWARDED
	hostIP := "127.0.0.1"
	if runtime.GOOS == "windows" {
		hostIP = ""
	}
	expectedPortBindings := map[docker.Port][]docker.PortBinding{
		docker.Port("8080/tcp"): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", res)}},
		docker.Port("8080/udp"): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", res)}},
		docker.Port("6379/tcp"): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", dyn)}},
		docker.Port("6379/udp"): {{HostIP: hostIP, HostPort: fmt.Sprintf("%d", dyn)}},
	}
	require.Exactly(t, expectedPortBindings, c.HostConfig.PortBindings)

}

func TestDockerDriver_CleanupContainer(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.Command = "echo"
	cfg.Args = []string{"hello"}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitCh, err := d.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)

	select {
	case res := <-waitCh:
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}

		err = d.DestroyTask(task.ID, false)
		require.NoError(t, err)

		time.Sleep(3 * time.Second)

		// Ensure that the container isn't present
		_, err := client.InspectContainer(handle.containerID)
		if err == nil {
			t.Fatalf("expected to not get container")
		}

	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}
}

func TestDockerDriver_EnableImageGC(t *testing.T) {
	testutil.DockerCompatible(t)

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.Command = "echo"
	cfg.Args = []string{"hello"}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

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

	copyImage(t, task.TaskDir(), "busybox.tar")
	_, _, err := driver.StartTask(task)
	require.NoError(t, err)

	dockerDriver, ok := driver.Impl().(*Driver)
	require.True(t, ok)
	_, ok = dockerDriver.tasks.Get(task.ID)
	require.True(t, ok)

	waitCh, err := dockerDriver.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)
	select {
	case res := <-waitCh:
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}

	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}

	// we haven't called DestroyTask, image should be present
	_, err = client.InspectImage(cfg.Image)
	require.NoError(t, err)

	err = dockerDriver.DestroyTask(task.ID, false)
	require.NoError(t, err)

	// image_delay is 3s, so image should still be around for a bit
	_, err = client.InspectImage(cfg.Image)
	require.NoError(t, err)

	// Ensure image was removed
	tu.WaitForResult(func() (bool, error) {
		if _, err := client.InspectImage(cfg.Image); err == nil {
			return false, fmt.Errorf("image exists but should have been removed. Does another %v container exist?", cfg.Image)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

func TestDockerDriver_DisableImageGC(t *testing.T) {
	testutil.DockerCompatible(t)

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.Command = "echo"
	cfg.Args = []string{"hello"}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

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

	copyImage(t, task.TaskDir(), "busybox.tar")
	_, _, err := driver.StartTask(task)
	require.NoError(t, err)

	dockerDriver, ok := driver.Impl().(*Driver)
	require.True(t, ok)
	handle, ok := dockerDriver.tasks.Get(task.ID)
	require.True(t, ok)

	waitCh, err := dockerDriver.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)
	select {
	case res := <-waitCh:
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}

	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}

	// we haven't called DestroyTask, image should be present
	_, err = client.InspectImage(handle.containerImage)
	require.NoError(t, err)

	err = dockerDriver.DestroyTask(task.ID, false)
	require.NoError(t, err)

	// image_delay is 1s, wait a little longer
	time.Sleep(3 * time.Second)

	// image should not have been removed or scheduled to be removed
	_, err = client.InspectImage(cfg.Image)
	require.NoError(t, err)
	dockerDriver.coordinator.imageLock.Lock()
	_, ok = dockerDriver.coordinator.deleteFuture[handle.containerImage]
	require.False(t, ok, "image should not be registered for deletion")
	dockerDriver.coordinator.imageLock.Unlock()
}

func TestDockerDriver_MissingContainer_Cleanup(t *testing.T) {
	testutil.DockerCompatible(t)

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.Command = "echo"
	cfg.Args = []string{"hello"}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

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

	copyImage(t, task.TaskDir(), "busybox.tar")
	_, _, err := driver.StartTask(task)
	require.NoError(t, err)

	dockerDriver, ok := driver.Impl().(*Driver)
	require.True(t, ok)
	h, ok := dockerDriver.tasks.Get(task.ID)
	require.True(t, ok)

	waitCh, err := dockerDriver.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)
	select {
	case res := <-waitCh:
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}

	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}

	// remove the container out-of-band
	require.NoError(t, client.RemoveContainer(docker.RemoveContainerOptions{
		ID: h.containerID,
	}))

	require.NoError(t, dockerDriver.DestroyTask(task.ID, false))

	// Ensure image was removed
	tu.WaitForResult(func() (bool, error) {
		if _, err := client.InspectImage(cfg.Image); err == nil {
			return false, fmt.Errorf("image exists but should have been removed. Does another %v container exist?", cfg.Image)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// Ensure that task handle was removed
	_, ok = dockerDriver.tasks.Get(task.ID)
	require.False(t, ok)
}

func TestDockerDriver_Stats(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.Command = "sleep"
	cfg.Args = []string{"1000"}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	_, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	go func() {
		defer d.DestroyTask(task.ID, true)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch, err := handle.Stats(ctx, 1*time.Second)
		assert.NoError(t, err)
		select {
		case ru := <-ch:
			assert.NotNil(t, ru.ResourceUsage)
		case <-time.After(3 * time.Second):
			assert.Fail(t, "stats timeout")
		}
	}()

	waitCh, err := d.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)
	select {
	case res := <-waitCh:
		if res.Successful() {
			t.Fatalf("should err: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*10) * time.Second):
		t.Fatalf("timeout")
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

	taskCfg := newTaskConfig("", []string{"touch", containerFile})
	taskCfg.Volumes = []string{fmt.Sprintf("%s:%s", hostpath, containerPath)}

	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "ls",
		AllocID:   uuid.Generate(),
		Env:       map[string]string{"VOL_PATH": containerPath},
		Resources: basicResources,
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(taskCfg))

	d := dockerDriverHarness(t, cfg)
	cleanup := d.MkAllocDir(task, true)

	copyImage(t, task.TaskDir(), "busybox.tar")

	return task, d, &taskCfg, hostfile, cleanup
}

func TestDockerDriver_VolumesDisabled(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
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
		tmpvol, err := ioutil.TempDir("", "nomadtest_docker_volumesdisabled")
		if err != nil {
			t.Fatalf("error creating temporary dir: %v", err)
		}

		task, driver, _, _, cleanup := setupDockerVolumes(t, cfg, tmpvol)
		defer cleanup()

		_, _, err = driver.StartTask(task)
		defer driver.DestroyTask(task.ID, true)
		if err == nil {
			require.Fail(t, "Started driver successfully when volumes should have been disabled.")
		}
	}

	// Relative paths should still be allowed
	{
		task, driver, _, fn, cleanup := setupDockerVolumes(t, cfg, ".")
		defer cleanup()

		_, _, err := driver.StartTask(task)
		require.NoError(t, err)
		defer driver.DestroyTask(task.ID, true)

		waitCh, err := driver.WaitTask(context.Background(), task.ID)
		require.NoError(t, err)
		select {
		case res := <-waitCh:
			if !res.Successful() {
				t.Fatalf("unexpected err: %v", res)
			}
		case <-time.After(time.Duration(tu.TestMultiplier()*10) * time.Second):
			t.Fatalf("timeout")
		}

		if _, err := ioutil.ReadFile(filepath.Join(task.TaskDir().Dir, fn)); err != nil {
			t.Fatalf("unexpected error reading %s: %v", fn, err)
		}
	}

	// Volume Drivers should be rejected (error)
	{
		task, driver, taskCfg, _, cleanup := setupDockerVolumes(t, cfg, "fake_flocker_vol")
		defer cleanup()

		taskCfg.VolumeDriver = "flocker"
		require.NoError(t, task.EncodeConcreteDriverConfig(taskCfg))

		_, _, err := driver.StartTask(task)
		defer driver.DestroyTask(task.ID, true)
		if err == nil {
			require.Fail(t, "Started driver successfully when volume drivers should have been disabled.")
		}
	}
}

func TestDockerDriver_VolumesEnabled(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	tmpvol, err := ioutil.TempDir("", "nomadtest_docker_volumesenabled")
	require.NoError(t, err)

	// Evaluate symlinks so it works on MacOS
	tmpvol, err = filepath.EvalSymlinks(tmpvol)
	require.NoError(t, err)

	task, driver, _, hostpath, cleanup := setupDockerVolumes(t, nil, tmpvol)
	defer cleanup()

	_, _, err = driver.StartTask(task)
	require.NoError(t, err)
	defer driver.DestroyTask(task.ID, true)

	waitCh, err := driver.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)
	select {
	case res := <-waitCh:
		if !res.Successful() {
			t.Fatalf("unexpected err: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*10) * time.Second):
		t.Fatalf("timeout")
	}

	if _, err := ioutil.ReadFile(hostpath); err != nil {
		t.Fatalf("unexpected error reading %s: %v", hostpath, err)
	}
}

func TestDockerDriver_Mounts(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
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
			// Build the task
			task, cfg, ports := dockerTask(t)
			defer freeport.Return(ports)
			cfg.Command = "sleep"
			cfg.Args = []string{"10000"}
			cfg.Mounts = c.Mounts
			require.NoError(t, task.EncodeConcreteDriverConfig(cfg))
			cleanup := d.MkAllocDir(task, true)
			defer cleanup()

			copyImage(t, task.TaskDir(), "busybox.tar")

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
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	path := "./test-resources/docker/auth.json"
	cases := []struct {
		Repo       string
		AuthConfig *docker.AuthConfiguration
	}{
		{
			Repo:       "lolwhat.com/what:1337",
			AuthConfig: nil,
		},
		{
			Repo: "redis:3.2",
			AuthConfig: &docker.AuthConfiguration{
				Username:      "test",
				Password:      "1234",
				Email:         "",
				ServerAddress: "https://index.docker.io/v1/",
			},
		},
		{
			Repo: "quay.io/redis:3.2",
			AuthConfig: &docker.AuthConfiguration{
				Username:      "test",
				Password:      "5678",
				Email:         "",
				ServerAddress: "quay.io",
			},
		},
		{
			Repo: "other.io/redis:3.2",
			AuthConfig: &docker.AuthConfiguration{
				Username:      "test",
				Password:      "abcd",
				Email:         "",
				ServerAddress: "https://other.io/v1/",
			},
		},
	}

	for _, c := range cases {
		act, err := authFromDockerConfig(path)(c.Repo)
		require.NoError(t, err)
		require.Exactly(t, c.AuthConfig, act)
	}
}

func TestDockerDriver_AuthFromTaskConfig(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}

	cases := []struct {
		Auth       DockerAuth
		AuthConfig *docker.AuthConfiguration
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
				Email:      "foo@bar.com",
				ServerAddr: "www.foobar.com",
			},
			AuthConfig: &docker.AuthConfiguration{
				Username:      "foo",
				Password:      "bar",
				Email:         "foo@bar.com",
				ServerAddress: "www.foobar.com",
			},
			Desc: "All fields set",
		},
		{
			Auth: DockerAuth{
				Username:   "foo",
				Password:   "bar",
				ServerAddr: "www.foobar.com",
			},
			AuthConfig: &docker.AuthConfiguration{
				Username:      "foo",
				Password:      "bar",
				ServerAddress: "www.foobar.com",
			},
			Desc: "Email not set",
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			act, err := authFromTaskConfig(&TaskConfig{Auth: c.Auth})("test")
			require.NoError(t, err)
			require.Exactly(t, c.AuthConfig, act)
		})
	}
}

func TestDockerDriver_OOMKilled(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support OOM Killer")
	}

	taskCfg := newTaskConfig("", []string{"sh", "-c", `sleep 2 && x=a && while true; do x="$x$x"; done`})
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "oom-killed",
		AllocID:   uuid.Generate(),
		Resources: basicResources,
	}
	task.Resources.LinuxResources.MemoryLimitBytes = 10 * 1024 * 1024
	task.Resources.NomadResources.Memory.MemoryMB = 10

	require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	_, _, err := d.StartTask(task)
	require.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	waitCh, err := d.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)
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
	if !tu.IsCI() {
		t.Parallel()
	}
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
		task, cfg, ports := dockerTask(t)
		cfg.Devices = tc.deviceConfig
		require.NoError(t, task.EncodeConcreteDriverConfig(cfg))
		d := dockerDriverHarness(t, nil)
		cleanup := d.MkAllocDir(task, true)
		copyImage(t, task.TaskDir(), "busybox.tar")
		defer cleanup()

		_, _, err := d.StartTask(task)
		require.Error(t, err)
		require.Contains(t, err.Error(), tc.err.Error())
		freeport.Return(ports)
	}
}

func TestDockerDriver_Device_Success(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	if runtime.GOOS != "linux" {
		t.Skip("test device mounts only on linux")
	}

	hostPath := "/dev/random"
	containerPath := "/dev/myrandom"
	perms := "rwm"

	expectedDevice := docker.Device{
		PathOnHost:        hostPath,
		PathInContainer:   containerPath,
		CgroupPermissions: perms,
	}
	config := DockerDevice{
		HostPath:      hostPath,
		ContainerPath: containerPath,
	}

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.Devices = []DockerDevice{config}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, driver, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, driver.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)

	require.NotEmpty(t, container.HostConfig.Devices, "Expected one device")
	require.Equal(t, expectedDevice, container.HostConfig.Devices[0], "Incorrect device ")
}

func TestDockerDriver_Entrypoint(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	entrypoint := []string{"sh", "-c"}
	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.Entrypoint = entrypoint
	cfg.Command = strings.Join(busyboxLongRunningCmd, " ")
	cfg.Args = []string{}

	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, driver, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	require.NoError(t, driver.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)

	require.Len(t, container.Config.Entrypoint, 2, "Expected one entrypoint")
	require.Equal(t, entrypoint, container.Config.Entrypoint, "Incorrect entrypoint ")
}

func TestDockerDriver_ReadonlyRootfs(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	if runtime.GOOS == "windows" {
		t.Skip("Windows Docker does not support root filesystem in read-only mode")
	}

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.ReadonlyRootfs = true
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, driver, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, driver.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)

	require.True(t, container.HostConfig.ReadonlyRootfs, "ReadonlyRootfs option not set")
}

// fakeDockerClient can be used in places that accept an interface for the
// docker client such as createContainer.
type fakeDockerClient struct{}

func (fakeDockerClient) CreateContainer(docker.CreateContainerOptions) (*docker.Container, error) {
	return nil, fmt.Errorf("volume is attached on another node")
}
func (fakeDockerClient) InspectContainer(id string) (*docker.Container, error) {
	panic("not implemented")
}
func (fakeDockerClient) ListContainers(docker.ListContainersOptions) ([]docker.APIContainers, error) {
	panic("not implemented")
}
func (fakeDockerClient) RemoveContainer(opts docker.RemoveContainerOptions) error {
	panic("not implemented")
}

// TestDockerDriver_VolumeError asserts volume related errors when creating a
// container are recoverable.
func TestDockerDriver_VolumeError(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}

	// setup
	_, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	driver := dockerDriverHarness(t, nil)

	// assert volume error is recoverable
	_, err := driver.Impl().(*Driver).createContainer(fakeDockerClient{}, docker.CreateContainerOptions{Config: &docker.Config{}}, cfg.Image)
	require.True(t, structs.IsRecoverable(err))
}

func TestDockerDriver_AdvertiseIPv6Address(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	expectedPrefix := "2001:db8:1::242:ac11"
	expectedAdvertise := true
	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	cfg.AdvertiseIPv6Addr = expectedAdvertise
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client := newTestDockerClient(t)

	// Make sure IPv6 is enabled
	net, err := client.NetworkInfo("bridge")
	if err != nil {
		t.Skip("error retrieving bridge network information, skipping")
	}
	if net == nil || !net.EnableIPv6 {
		t.Skip("IPv6 not enabled on bridge network, skipping")
	}

	driver := dockerDriverHarness(t, nil)
	cleanup := driver.MkAllocDir(task, true)
	copyImage(t, task.TaskDir(), "busybox.tar")
	defer cleanup()

	_, network, err := driver.StartTask(task)
	defer driver.DestroyTask(task.ID, true)
	require.NoError(t, err)

	require.Equal(t, expectedAdvertise, network.AutoAdvertise, "Wrong autoadvertise. Expect: %s, got: %s", expectedAdvertise, network.AutoAdvertise)

	if !strings.HasPrefix(network.IP, expectedPrefix) {
		t.Fatalf("Got IP address %q want ip address with prefix %q", network.IP, expectedPrefix)
	}

	handle, ok := driver.Impl().(*Driver).tasks.Get(task.ID)
	require.True(t, ok)

	require.NoError(t, driver.WaitUntilStarted(task.ID, time.Second))

	container, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)

	if !strings.HasPrefix(container.NetworkSettings.GlobalIPv6Address, expectedPrefix) {
		t.Fatalf("Got GlobalIPv6address %s want GlobalIPv6address with prefix %s", expectedPrefix, container.NetworkSettings.GlobalIPv6Address)
	}
}

func TestParseDockerImage(t *testing.T) {
	tests := []struct {
		Image string
		Repo  string
		Tag   string
	}{
		{"library/hello-world:1.0", "library/hello-world", "1.0"},
		{"library/hello-world", "library/hello-world", "latest"},
		{"library/hello-world:latest", "library/hello-world", "latest"},
		{"library/hello-world@sha256:f5233545e43561214ca4891fd1157e1c3c563316ed8e237750d59bde73361e77", "library/hello-world@sha256:f5233545e43561214ca4891fd1157e1c3c563316ed8e237750d59bde73361e77", ""},
	}
	for _, test := range tests {
		t.Run(test.Image, func(t *testing.T) {
			repo, tag := parseDockerImage(test.Image)
			require.Equal(t, test.Repo, repo)
			require.Equal(t, test.Tag, tag)
		})
	}
}

func TestDockerImageRef(t *testing.T) {
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
			require.Equal(t, test.Image, image)
		})
	}
}

func waitForExist(t *testing.T, client *docker.Client, containerID string) {
	tu.WaitForResult(func() (bool, error) {
		container, err := client.InspectContainer(containerID)
		if err != nil {
			if _, ok := err.(*docker.NoSuchContainer); !ok {
				return false, err
			}
		}

		return container != nil, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

// TestDockerDriver_CreationIdempotent asserts that createContainer and
// and startContainers functions are idempotent, as we have some retry
// logic there without ensureing we delete/destroy containers
func TestDockerDriver_CreationIdempotent(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, ports := dockerTask(t)
	defer freeport.Return(ports)
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client := newTestDockerClient(t)
	driver := dockerDriverHarness(t, nil)
	cleanup := driver.MkAllocDir(task, true)
	defer cleanup()

	copyImage(t, task.TaskDir(), "busybox.tar")

	d, ok := driver.Impl().(*Driver)
	require.True(t, ok)

	_, err := d.createImage(task, cfg, client)
	require.NoError(t, err)

	containerCfg, err := d.createContainerConfig(task, cfg, cfg.Image)
	require.NoError(t, err)

	c, err := d.createContainer(client, containerCfg, cfg.Image)
	require.NoError(t, err)
	defer client.RemoveContainer(docker.RemoveContainerOptions{
		ID:    c.ID,
		Force: true,
	})

	// calling createContainer again creates a new one and remove old one
	c2, err := d.createContainer(client, containerCfg, cfg.Image)
	require.NoError(t, err)
	defer client.RemoveContainer(docker.RemoveContainerOptions{
		ID:    c2.ID,
		Force: true,
	})

	require.NotEqual(t, c.ID, c2.ID)
	// old container was destroyed
	{
		_, err := client.InspectContainer(c.ID)
		require.Error(t, err)
		require.Contains(t, err.Error(), NoSuchContainerError)
	}

	// now start container twice
	require.NoError(t, d.startContainer(c2))
	require.NoError(t, d.startContainer(c2))

	tu.WaitForResult(func() (bool, error) {
		c, err := client.InspectContainer(c2.ID)
		if err != nil {
			return false, fmt.Errorf("failed to get container status: %v", err)
		}

		if !c.State.Running {
			return false, fmt.Errorf("container is not running but %v", c.State)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

// TestDockerDriver_CreateContainerConfig_CPUHardLimit asserts that a default
// CPU quota and period are set when cpu_hard_limit = true.
func TestDockerDriver_CreateContainerConfig_CPUHardLimit(t *testing.T) {
	t.Parallel()

	task, _, ports := dockerTask(t)
	defer freeport.Return(ports)

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)
	schema, _ := driver.TaskConfigSchema()
	spec, _ := hclspecutils.Convert(schema)

	val, _, _ := hclutils.ParseHclInterface(map[string]interface{}{
		"image":          "foo/bar",
		"cpu_hard_limit": true,
	}, spec, nil)

	require.NoError(t, task.EncodeDriverConfig(val))
	cfg := &TaskConfig{}
	require.NoError(t, task.DecodeDriverConfig(cfg))
	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	require.NoError(t, err)

	require.NotZero(t, c.HostConfig.CPUQuota)
	require.NotZero(t, c.HostConfig.CPUPeriod)
}
