package docker

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/consul/lib/freeport"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/hashicorp/nomad/plugins/shared/loader"
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
	// busyboxImageID is the ID stored in busybox.tar
	busyboxImageID = "busybox:1.29.3"

	// busyboxImageID is the ID stored in busybox_glibc.tar
	busyboxGlibcImageID = "busybox:1.29.3-glibc"

	// busyboxImageID is the ID stored in busybox_musl.tar
	busyboxMuslImageID = "busybox:1.29.3-musl"

	// busyboxLongRunningCmd is a busybox command that runs indefinitely, and
	// ideally responds to SIGINT/SIGTERM.  Sadly, busybox:1.29.3 /bin/sleep doesn't.
	busyboxLongRunningCmd = []string{"/bin/nc", "-l", "-p", "3000", "127.0.0.1"}
)

// Returns a task with a reserved and dynamic port. The ports are returned
// respectively.
func dockerTask(t *testing.T) (*drivers.TaskConfig, *TaskConfig, []int) {
	ports := freeport.GetT(t, 2)
	dockerReserved := ports[0]
	dockerDynamic := ports[1]

	cfg := TaskConfig{
		Image:     busyboxImageID,
		LoadImage: "busybox.tar",
		Command:   busyboxLongRunningCmd[0],
		Args:      busyboxLongRunningCmd[1:],
	}
	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "redis-demo",
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
	if !tu.IsTravis() {
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
	if !tu.IsTravis() {
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
	if !tu.IsTravis() {
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
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	taskCfg := TaskConfig{
		Image:     busyboxImageID,
		LoadImage: "busybox.tar",
		Command:   busyboxLongRunningCmd[0],
		Args:      busyboxLongRunningCmd[1:],
	}
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "nc-demo",
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
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	taskCfg := TaskConfig{
		Image:     busyboxImageID,
		LoadImage: "busybox.tar",
		Command:   "/bin/echo",
		Args:      []string{"hello"},
	}
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "nc-demo",
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
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	taskCfg := TaskConfig{
		Image:     busyboxImageID,
		LoadImage: "busybox.tar",
		Command:   "sleep",
		Args:      []string{"9001"},
	}
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "nc-demo",
		Resources: basicResources,
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	client := newTestDockerClient(t)
	imageID, err := d.Impl().(*Driver).loadImage(task, &taskCfg, client)
	require.NoError(t, err)
	require.NotEmpty(t, imageID)

	// Create a container of the same name but don't start it. This mimics
	// the case of dockerd getting restarted and stopping containers while
	// Nomad is watching them.
	opts := docker.CreateContainerOptions{
		Name: strings.Replace(task.ID, "/", "_", -1),
		Config: &docker.Config{
			Image: busyboxImageID,
			Cmd:   []string{"sleep", "9000"},
		},
	}

	if _, err := client.CreateContainer(opts); err != nil {
		t.Fatalf("error creating initial container: %v", err)
	}

	_, _, err = d.StartTask(task)
	require.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))
}

func TestDockerDriver_Start_LoadImage(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	taskCfg := TaskConfig{
		Image:     busyboxImageID,
		LoadImage: "busybox.tar",
		Command:   "/bin/sh",
		Args: []string{
			"-c",
			"echo hello > $NOMAD_TASK_DIR/output",
		},
	}
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

func TestDockerDriver_Start_BadPull_Recoverable(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	taskCfg := TaskConfig{
		Image:   "127.0.0.1:32121/foo", // bad path
		Command: "/bin/echo",
		Args: []string{
			"hello",
		},
	}
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "busybox-demo",
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
	if !tu.IsTravis() {
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
	taskCfg := TaskConfig{
		Image:     busyboxImageID,
		LoadImage: "busybox.tar",
		Command:   "/bin/sh",
		Args: []string{
			"-c",
			fmt.Sprintf(`sleep 1; echo -n %s > $%s/%s`,
				string(exp), taskenv.AllocDir, file),
		},
	}
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
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	taskCfg := TaskConfig{
		Image:     busyboxImageID,
		LoadImage: "busybox.tar",
		Command:   busyboxLongRunningCmd[0],
		Args:      busyboxLongRunningCmd[1:],
	}
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
	require.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	go func(t *testing.T) {
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, d.StopTask(task.ID, time.Second, "SIGINT"))
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
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)
	timeout := 2 * time.Second
	taskCfg := TaskConfig{
		Image:     busyboxImageID,
		LoadImage: "busybox.tar",
		Command:   "/bin/sleep",
		Args: []string{
			"10",
		},
	}
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
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)
	require := require.New(t)

	task1, _, _ := dockerTask(t)
	task2, _, _ := dockerTask(t)
	task3, _, _ := dockerTask(t)
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

		defer d.DestroyTask(task.ID, true)
	}

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
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)
	require := require.New(t)

	task1, cfg1, _ := dockerTask(t)
	cfg1.Image = busyboxImageID
	cfg1.LoadImage = "busybox.tar"
	require.NoError(task1.EncodeConcreteDriverConfig(cfg1))

	task2, cfg2, _ := dockerTask(t)
	cfg2.Image = busyboxMuslImageID
	cfg2.LoadImage = "busybox_musl.tar"
	require.NoError(task2.EncodeConcreteDriverConfig(cfg2))

	task3, cfg3, _ := dockerTask(t)
	cfg3.Image = busyboxGlibcImageID
	cfg3.LoadImage = "busybox_glibc.tar"
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

		defer d.DestroyTask(task.ID, true)

		require.NoError(d.WaitUntilStarted(task.ID, 5*time.Second))
	}

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

func TestDockerDriver_NetworkMode_Host(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)
	expected := "host"

	taskCfg := TaskConfig{
		Image:       busyboxImageID,
		LoadImage:   "busybox.tar",
		Command:     busyboxLongRunningCmd[0],
		Args:        busyboxLongRunningCmd[1:],
		NetworkMode: expected,
	}
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
	require.NoError(t, err)

	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	defer d.DestroyTask(task.ID, true)

	dockerDriver, ok := d.Impl().(*Driver)
	require.True(t, ok)

	handle, ok := dockerDriver.tasks.Get(task.ID)
	require.True(t, ok)

	container, err := client.InspectContainer(handle.containerID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	actual := container.HostConfig.NetworkMode
	require.Equal(t, expected, actual)
}

func TestDockerDriver_NetworkAliases_Bridge(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
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
	taskCfg := TaskConfig{
		Image:          busyboxImageID,
		LoadImage:      "busybox.tar",
		Command:        busyboxLongRunningCmd[0],
		Args:           busyboxLongRunningCmd[1:],
		NetworkMode:    network.Name,
		NetworkAliases: expected,
	}
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

func TestDockerDriver_Sysctl_Ulimit(t *testing.T) {
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

	client, d, handle, cleanup := dockerSetup(t, task)
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
		defer cleanup()
		copyImage(t, task.TaskDir(), "busybox.tar")

		_, _, err := d.StartTask(task)
		require.NotNil(t, err, "Expected non nil error")
		require.Contains(t, err.Error(), tc.err.Error())
	}
}

func TestDockerDriver_Labels(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
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

	require.Equal(t, 2, len(container.Config.Labels))
	for k, v := range cfg.Labels {
		require.Equal(t, v, container.Config.Labels[k])
	}
}

func TestDockerDriver_ForcePull(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
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
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
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

func TestDockerDriver_SecurityOpt(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
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

func TestDockerDriver_CreateContainerConfig(t *testing.T) {
	t.Parallel()

	task, cfg, _ := dockerTask(t)
	opt := map[string]string{"size": "120G"}

	cfg.StorageOpt = opt
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	dh := dockerDriverHarness(t, nil)
	driver := dh.Impl().(*Driver)

	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	require.NoError(t, err)

	require.Equal(t, "org/repo:0.1", c.Config.Image)
	require.EqualValues(t, opt, c.HostConfig.StorageOpt)
}
func TestDockerDriver_Capabilities(t *testing.T) {
	if !tu.IsTravis() {
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
			task, cfg, _ := dockerTask(t)
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

			defer d.DestroyTask(task.ID, true)
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
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
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
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
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
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
	cfg.WorkDir = "/some/path"
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)

	require.Equal(t, cfg.WorkDir, container.Config.WorkingDir)
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
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, _, port := dockerTask(t)
	res := port[0]
	dyn := port[1]

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

	// Verify that the correct ports are FORWARDED
	expectedPortBindings := map[docker.Port][]docker.PortBinding{
		docker.Port(fmt.Sprintf("%d/tcp", res)): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", res)}},
		docker.Port(fmt.Sprintf("%d/udp", res)): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", res)}},
		docker.Port(fmt.Sprintf("%d/tcp", dyn)): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", dyn)}},
		docker.Port(fmt.Sprintf("%d/udp", dyn)): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", dyn)}},
	}

	require.Exactly(t, expectedPortBindings, container.HostConfig.PortBindings)
}

func TestDockerDriver_PortsMapping(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, port := dockerTask(t)
	res := port[0]
	dyn := port[1]
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

	// Verify that the correct ports are EXPOSED
	expectedExposedPorts := map[docker.Port]struct{}{
		docker.Port("8080/tcp"): {},
		docker.Port("8080/udp"): {},
		docker.Port("6379/tcp"): {},
		docker.Port("6379/udp"): {},
	}

	require.Exactly(t, expectedExposedPorts, container.Config.ExposedPorts)

	// Verify that the correct ports are FORWARDED
	expectedPortBindings := map[docker.Port][]docker.PortBinding{
		docker.Port("8080/tcp"): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", res)}},
		docker.Port("8080/udp"): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", res)}},
		docker.Port("6379/tcp"): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", dyn)}},
		docker.Port("6379/udp"): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", dyn)}},
	}
	require.Exactly(t, expectedPortBindings, container.HostConfig.PortBindings)
}

func TestDockerDriver_User(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
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

func TestDockerDriver_CleanupContainer(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
	cfg.Command = "/bin/echo"
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

func TestDockerDriver_Stats(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
	cfg.Command = "/bin/sleep"
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
	containerPath := "/mnt/vol"
	containerFile := filepath.Join(containerPath, randfn)

	taskCfg := &TaskConfig{
		Image:     busyboxImageID,
		LoadImage: "busybox.tar",
		Command:   "touch",
		Args:      []string{containerFile},
		Volumes:   []string{fmt.Sprintf("%s:%s", hostpath, containerPath)},
	}
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "ls",
		Env:       map[string]string{"VOL_PATH": containerPath},
		Resources: basicResources,
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(taskCfg))

	d := dockerDriverHarness(t, cfg)
	cleanup := d.MkAllocDir(task, true)

	copyImage(t, task.TaskDir(), "busybox.tar")

	return task, d, taskCfg, hostfile, cleanup
}

func TestDockerDriver_VolumesDisabled(t *testing.T) {
	if !tu.IsTravis() {
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

		if _, _, err := driver.StartTask(task); err == nil {
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

		if _, _, err := driver.StartTask(task); err == nil {
			require.Fail(t, "Started driver successfully when volume drivers should have been disabled.")
		}
	}

}

func TestDockerDriver_BindMountsHonorVolumesEnabledFlag(t *testing.T) {
	t.Parallel()

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
			name:            "relative local driver",
			requiresVolumes: false,
			volumeDriver:    "local",
			volumes:         []string{"test-path:/tmp/taskpath"},
			expectedVolumes: []string{"/tmp/nomad/alloc-dir/demo/test-path:/tmp/taskpath"},
		},
		{
			name:            "relative outside task-dir default driver",
			requiresVolumes: false,
			volumeDriver:    "",
			volumes:         []string{"../test-path:/tmp/taskpath"},
			expectedVolumes: []string{"/tmp/nomad/alloc-dir/test-path:/tmp/taskpath"},
		},
		{
			name:            "relative outside task-dir local driver",
			requiresVolumes: false,
			volumeDriver:    "local",
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
			name:            "relative outside task-dir local driver",
			requiresVolumes: true,
			volumeDriver:    "local",
			volumes:         []string{"../../test-path:/tmp/taskpath"},
			expectedVolumes: []string{"/tmp/nomad/test-path:/tmp/taskpath"},
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

func TestDockerDriver_VolumesEnabled(t *testing.T) {
	if !tu.IsTravis() {
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
	if !tu.IsTravis() {
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
			task, cfg, _ := dockerTask(t)
			cfg.Command = "/bin/sleep"
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

func TestDockerDriver_MountsSerialization(t *testing.T) {
	t.Parallel()

	allocDir := "/tmp/nomad/alloc-dir"

	cases := []struct {
		name            string
		requiresVolumes bool
		passedMounts    []DockerMount
		expectedMounts  []docker.HostMount
	}{
		{
			name: "basic volume",
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
	t.Parallel()

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

	c, err := driver.createContainerConfig(task, cfg, "org/repo:0.1")
	require.NoError(t, err)

	expectedMounts := []docker.HostMount{
		{
			Type:        "bind",
			Source:      "/tmp/cfg-mount",
			Target:      "/container/tmp/cfg-mount",
			ReadOnly:    false,
			BindOptions: &docker.BindOptions{},
		},
		{
			Type:     "bind",
			Source:   "/tmp/task-mount",
			Target:   "/container/tmp/task-mount",
			ReadOnly: true,
		},
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
func TestDockerDriver_Cleanup(t *testing.T) {
	testutil.DockerCompatible(t)

	// using a small image and an specific point release to avoid accidental conflicts with other tasks
	imageName := "busybox:1.27.1"
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "cleanup_test",
		Resources: basicResources,
	}
	cfg := &TaskConfig{
		Image:   imageName,
		Command: "/bin/sleep",
		Args:    []string{"100"},
	}

	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, driver, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	require.NoError(t, driver.WaitUntilStarted(task.ID, 5*time.Second))
	// Cleanup
	require.NoError(t, driver.DestroyTask(task.ID, true))

	// Ensure image was removed
	tu.WaitForResult(func() (bool, error) {
		if _, err := client.InspectImage(imageName); err == nil {
			return false, fmt.Errorf("image exists but should have been removed. Does another %v container exist?", imageName)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// The image doesn't exist which shouldn't be an error when calling
	// Cleanup, so call it again to make sure.
	require.NoError(t, driver.Impl().(*Driver).cleanupImage(handle))

}

func TestDockerDriver_AuthConfiguration(t *testing.T) {
	if !tu.IsTravis() {
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

func TestDockerDriver_OOMKilled(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	taskCfg := TaskConfig{
		Image:     busyboxImageID,
		LoadImage: "busybox.tar",
		Command:   "/bin/sh",
		Args:      []string{"-c", `/bin/sleep 2 && x=a && while true; do x="$x$x"; done`},
	}
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "oom-killed",
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
	if !tu.IsTravis() {
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

	test_cases := []struct {
		deviceConfig []DockerDevice
		err          error
	}{
		{brokenConfigs[:1], fmt.Errorf("host path must be set in configuration for devices")},
		{brokenConfigs[1:], fmt.Errorf("invalid cgroup permission string: \"rxb\"")},
	}

	for _, tc := range test_cases {
		task, cfg, _ := dockerTask(t)
		cfg.Devices = tc.deviceConfig
		require.NoError(t, task.EncodeConcreteDriverConfig(cfg))
		d := dockerDriverHarness(t, nil)
		cleanup := d.MkAllocDir(task, true)
		copyImage(t, task.TaskDir(), "busybox.tar")
		defer cleanup()

		_, _, err := d.StartTask(task)
		require.Error(t, err)
		require.Contains(t, err.Error(), tc.err.Error())
	}
}

func TestDockerDriver_Device_Success(t *testing.T) {
	if !tu.IsTravis() {
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

	task, cfg, _ := dockerTask(t)
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
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	entrypoint := []string{"/bin/sh", "-c"}
	task, cfg, _ := dockerTask(t)
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
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
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
	if !tu.IsTravis() {
		t.Parallel()
	}

	// setup
	_, cfg, _ := dockerTask(t)
	driver := dockerDriverHarness(t, nil)

	// assert volume error is recoverable
	_, err := driver.Impl().(*Driver).createContainer(fakeDockerClient{}, docker.CreateContainerOptions{Config: &docker.Config{}}, cfg)
	require.True(t, structs.IsRecoverable(err))
}

func TestDockerDriver_AdvertiseIPv6Address(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	expectedPrefix := "2001:db8:1::242:ac11"
	expectedAdvertise := true
	task, cfg, _ := dockerTask(t)
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

func TestDockerDriver_CPUCFSPeriod(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
	cfg.CPUHardLimit = true
	cfg.CPUCFSPeriod = 1000000
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, _, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle.containerID)

	container, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)

	require.Equal(t, cfg.CPUCFSPeriod, container.HostConfig.CPUPeriod)
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

func copyImage(t *testing.T, taskDir *allocdir.TaskDir, image string) {
	dst := filepath.Join(taskDir.LocalDir, image)
	copyFile(filepath.Join("./test-resources/docker", image), dst, t)
}

// copyFile moves an existing file to the destination
func copyFile(src, dst string, t *testing.T) {
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("copying %v -> %v failed: %v", src, dst, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("copying %v -> %v failed: %v", src, dst, err)
	}
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
