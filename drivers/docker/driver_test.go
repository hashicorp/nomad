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
	"strconv"
	"strings"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/consul/lib/freeport"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/loader"
	tu "github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	basicResources = &drivers.Resources{
		NomadResources: &structs.Resources{
			CPU:      250,
			MemoryMB: 256,
			DiskMB:   20,
		},
		LinuxResources: &drivers.LinuxResources{
			CPUShares:        250,
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

// Returns a task with a reserved and dynamic port. The ports are returned
// respectively.
func dockerTask(t *testing.T) (*drivers.TaskConfig, *TaskConfig, []int) {
	ports := freeport.GetT(t, 2)
	dockerReserved := ports[0]
	dockerDynamic := ports[1]

	cfg := TaskConfig{
		Image:     "busybox",
		LoadImage: "busybox.tar",
		Command:   "/bin/nc",
		Args:      []string{"-l", "127.0.0.1", "-p", "0"},
	}
	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "redis-demo",
		Resources: &drivers.Resources{
			NomadResources: &structs.Resources{
				MemoryMB: 256,
				CPU:      512,
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
func dockerSetup(t *testing.T, task *drivers.TaskConfig) (*docker.Client, *drivers.DriverHarness, *taskHandle, func()) {
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
func dockerDriverHarness(t *testing.T, cfg map[string]interface{}) *drivers.DriverHarness {
	logger := testlog.HCLogger(t)
	harness := drivers.NewDriverHarness(t, NewDockerDriver(logger))
	if cfg == nil {
		cfg = map[string]interface{}{
			"image_gc_delay": "1s",
		}
	}
	plugLoader, err := loader.NewPluginLoader(&loader.PluginLoaderConfig{
		Logger:    logger,
		PluginDir: "./plugins",
		InternalPlugins: map[loader.PluginID]*loader.InternalPluginConfig{
			PluginID: &loader.InternalPluginConfig{
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
	driver, ok := instance.Plugin().(*drivers.DriverHarness)
	if !ok {
		t.Fatal("plugin instance is not a driver... wat?")
	}

	return driver
}

func newTestDockerClient(t *testing.T) *docker.Client {
	t.Helper()
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

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

	request := &cstructs.FingerprintRequest{Config: &config.Config{}, Node: node}
	var response cstructs.FingerprintResponse
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("requires Docker")
	}
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

	request := &cstructs.FingerprintRequest{Config: conf, Node: conf.Node}
	var response cstructs.FingerprintResponse

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
	if !testutil.DockerIsConnected(t) {
		t.Skip("requires Docker")
	}
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	taskCfg := TaskConfig{
		Image:     "busybox",
		LoadImage: "busybox.tar",
		Command:   "/bin/nc",
		Args:      []string{"-l", "127.0.0.1", "-p", "0"},
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
		t.Fatalf("wait channel should not have recieved an exit result")
	case <-time.After(time.Duration(tu.TestMultiplier()*1) * time.Second):
	}
}

func TestDockerDriver_Start_WaitFinish(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	taskCfg := TaskConfig{
		Image:     "busybox",
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	taskCfg := TaskConfig{
		Image:     "busybox",
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

	// Create a container of the same name but don't start it. This mimics
	// the case of dockerd getting restarted and stopping containers while
	// Nomad is watching them.
	opts := docker.CreateContainerOptions{
		Name: strings.Replace(task.ID, "/", "_", -1),
		Config: &docker.Config{
			Image: "busybox",
			Cmd:   []string{"sleep", "9000"},
		},
	}

	client := newTestDockerClient(t)
	if _, err := client.CreateContainer(opts); err != nil {
		t.Fatalf("error creating initial container: %v", err)
	}

	_, _, err := d.StartTask(task)
	require.NoError(t, err)

	defer d.DestroyTask(task.ID, true)

	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))
}

func TestDockerDriver_Start_LoadImage(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	taskCfg := TaskConfig{
		Image:     "busybox",
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

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
		Image:     "busybox",
		LoadImage: "busybox.tar",
		Command:   "/bin/sh",
		Args: []string{
			"-c",
			fmt.Sprintf(`sleep 1; echo -n %s > $%s/%s`,
				string(exp), env.AllocDir, file),
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
			require.Fail(t, "ExitResult should be successful: %v", res)
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	taskCfg := TaskConfig{
		Image:     "busybox",
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

	go func() {
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, d.StopTask(task.ID, time.Second, "SIGINT"))
	}()

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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}
	timeout := 2 * time.Second
	taskCfg := TaskConfig{
		Image:     "busybox",
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}
	require := require.New(t)

	task1, cfg1, _ := dockerTask(t)
	cfg1.Image = "busybox"
	cfg1.LoadImage = "busybox.tar"
	require.NoError(task1.EncodeConcreteDriverConfig(cfg1))

	task2, cfg2, _ := dockerTask(t)
	cfg2.Image = "busybox:musl"
	cfg2.LoadImage = "busybox_musl.tar"
	cfg2.Args = []string{"-l", "-p", "0"}
	require.NoError(task2.EncodeConcreteDriverConfig(cfg2))

	task3, cfg3, _ := dockerTask(t)
	cfg3.Image = "busybox:glibc"
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

		d.WaitUntilStarted(task.ID, 5*time.Second)
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}
	expected := "host"

	taskCfg := TaskConfig{
		Image:       "busybox",
		LoadImage:   "busybox.tar",
		Command:     "/bin/nc",
		Args:        []string{"-l", "127.0.0.1", "-p", "0"},
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

	d.WaitUntilStarted(task.ID, 5*time.Second)

	defer d.DestroyTask(task.ID, true)

	dockerDriver, ok := d.Impl().(*Driver)
	require.True(t, ok)

	handle, ok := dockerDriver.tasks.Get(task.ID)
	require.True(t, ok)

	container, err := client.InspectContainer(handle.container.ID)
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}
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
		Image:          "busybox",
		LoadImage:      "busybox.tar",
		Command:        "/bin/nc",
		Args:           []string{"-l", "127.0.0.1", "-p", "0"},
		NetworkMode:    network.Name,
		NetworkAliases: expected,
	}
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "busybox-demo",
		Resources: basicResources,
	}
	require.NoError(task.EncodeConcreteDriverConfig(&taskCfg))

	d := dockerDriverHarness(t, nil)
	cleanup := d.MkAllocDir(task, true)
	defer cleanup()
	copyImage(t, task.TaskDir(), "busybox.tar")

	_, _, err = d.StartTask(task)
	require.NoError(err)

	d.WaitUntilStarted(task.ID, 5*time.Second)

	defer d.DestroyTask(task.ID, true)

	dockerDriver, ok := d.Impl().(*Driver)
	require.True(ok)

	handle, ok := dockerDriver.tasks.Get(task.ID)
	require.True(ok)

	_, err = client.InspectContainer(handle.container.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestDockerDriver_Sysctl_Ulimit(t *testing.T) {
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
	d.WaitUntilStarted(task.ID, 5*time.Second)

	container, err := client.InspectContainer(handle.container.ID)
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
	brokenConfigs := []map[string]string{
		map[string]string{
			"nofile": "",
		},
		map[string]string{
			"nofile": "abc:1234",
		},
		map[string]string{
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, cfg, _ := dockerTask(t)
	cfg.Labels = map[string]string{
		"label1": "value1",
		"label2": "value2",
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.container.ID)
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, cfg, _ := dockerTask(t)
	cfg.ForcePull = true
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	_, err := client.InspectContainer(handle.container.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestDockerDriver_ForcePull_RepoDigest(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, cfg, _ := dockerTask(t)
	cfg.LoadImage = ""
	cfg.Image = "library/busybox@sha256:58ac43b2cc92c687a32c8be6278e50a063579655fe3090125dcb2af0ff9e1a64"
	localDigest := "sha256:8ac48589692a53a9b8c2d1ceaa6b402665aa7fe667ba51ccc03002300856d8c7"
	cfg.ForcePull = true
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.container.ID)
	require.NoError(t, err)
	require.Equal(t, localDigest, container.Image)
}

func TestDockerDriver_SecurityOpt(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, cfg, _ := dockerTask(t)
	cfg.SecurityOpt = []string{"seccomp=unconfined"}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.container.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	require.Exactly(t, cfg.SecurityOpt, container.HostConfig.SecurityOpt)
}

func TestDockerDriver_Capabilities(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}
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

			d.WaitUntilStarted(task.ID, 5*time.Second)

			container, err := client.InspectContainer(handle.container.ID)
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, cfg, _ := dockerTask(t)
	cfg.DNSServers = []string{"8.8.8.8", "8.8.4.4"}
	cfg.DNSSearchDomains = []string{"example.com", "example.org", "example.net"}
	cfg.DNSOptions = []string{"ndots:1"}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.container.ID)
	require.NoError(t, err)

	require.Exactly(t, cfg.DNSServers, container.HostConfig.DNS)
	require.Exactly(t, cfg.DNSSearchDomains, container.HostConfig.DNSSearch)
	require.Exactly(t, cfg.DNSOptions, container.HostConfig.DNSOptions)
}

func TestDockerDriver_MACAddress(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, cfg, _ := dockerTask(t)
	cfg.MacAddress = "00:16:3e:00:00:00"
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.container.ID)
	require.NoError(t, err)

	require.Equal(t, cfg.MacAddress, container.NetworkSettings.MacAddress)
}

func TestDockerWorkDir(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, cfg, _ := dockerTask(t)
	cfg.WorkDir = "/some/path"
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.container.ID)
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, _, port := dockerTask(t)
	res := port[0]
	dyn := port[1]

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	container, err := client.InspectContainer(handle.container.ID)
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

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

	container, err := client.InspectContainer(handle.container.ID)
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

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
		_, err := client.InspectContainer(handle.container.ID)
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, cfg, _ := dockerTask(t)
	cfg.Command = "/bin/sleep"
	cfg.Args = []string{"1000"}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	_, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	go func() {
		time.Sleep(3 * time.Second)
		ru, err := handle.Stats()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if ru.ResourceUsage == nil {
			d.DestroyTask(task.ID, true)
			t.Fatalf("expected resource usage")
		}
		d.DestroyTask(task.ID, true)
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

func setupDockerVolumes(t *testing.T, cfg map[string]interface{}, hostpath string) (*drivers.TaskConfig, *drivers.DriverHarness, *TaskConfig, string, func()) {
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	randfn := fmt.Sprintf("test-%d", rand.Int())
	hostfile := filepath.Join(hostpath, randfn)
	containerPath := "/mnt/vol"
	containerFile := filepath.Join(containerPath, randfn)

	taskCfg := &TaskConfig{
		Image:     "busybox",
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	cfg := map[string]interface{}{
		"volumes_enabled": false,
		"image_gc":        false,
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

func TestDockerDriver_VolumesEnabled(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

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

	d := dockerDriverHarness(t, nil)
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
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

// TestDockerDriver_Cleanup ensures Cleanup removes only downloaded images.
func TestDockerDriver_Cleanup(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	imageName := "hello-world:latest"
	task := &drivers.TaskConfig{
		ID:        uuid.Generate(),
		Name:      "cleanup_test",
		Resources: basicResources,
	}
	cfg := &TaskConfig{
		Image: imageName,
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, driver, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	driver.WaitUntilStarted(task.ID, 5*time.Second)
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

/*
func TestDockerDriver_AuthConfiguration(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

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

	for i, c := range cases {
		act, err := authFromDockerConfig(path)(c.Repo)
		if err != nil {
			t.Fatalf("Test %d failed: %v", i+1, err)
		}

		if !reflect.DeepEqual(act, c.AuthConfig) {
			t.Fatalf("Test %d failed: Unexpected auth config: got %+v; want %+v", i+1, act, c.AuthConfig)
		}
	}
}

func TestDockerDriver_OOMKilled(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task := &structs.Task{
		Name:   "oom-killed",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "busybox",
			"load":    "busybox.tar",
			"command": "sh",
			// Incrementally creates a bigger and bigger variable.
			"args": []string{"-c", "x=a; while true; do eval x='$x$x'; done"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			CPU:      250,
			MemoryMB: 10,
			DiskMB:   20,
			Networks: []*structs.NetworkResource{},
		},
	}

	_, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	select {
	case res := <-handle.WaitCh():
		if res.Successful() {
			t.Fatalf("expected error, but container exited successful")
		}

		if res.Err.Error() != "OOM Killed" {
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	brokenConfigs := []interface{}{
		map[string]interface{}{
			"host_path": "",
		},
		map[string]interface{}{
			"host_path":          "/dev/sda1",
			"cgroup_permissions": "rxb",
		},
	}

	test_cases := []struct {
		deviceConfig interface{}
		err          error
	}{
		{[]interface{}{brokenConfigs[0]}, fmt.Errorf("host path must be set in configuration for devices")},
		{[]interface{}{brokenConfigs[1]}, fmt.Errorf("invalid cgroup permission string: \"rxb\"")},
	}

	for _, tc := range test_cases {
		task, _, _ := dockerTask(t)
		task.Config["devices"] = tc.deviceConfig

		ctx := testDockerDriverContexts(t, task)
		driver := NewDockerDriver(ctx.DriverCtx)
		copyImage(t, ctx.ExecCtx.TaskDir, "busybox.tar")
		defer ctx.Destroy()

		if _, err := driver.Prestart(ctx.ExecCtx, task); err == nil || err.Error() != tc.err.Error() {
			t.Fatalf("error expected in prestart, got %v, expected %v", err, tc.err)
		}
	}
}

func TestDockerDriver_Device_Success(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

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
	config := map[string]interface{}{
		"host_path":      hostPath,
		"container_path": containerPath,
	}

	task, _, _ := dockerTask(t)
	task.Config["devices"] = []interface{}{config}

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle)

	container, err := client.InspectContainer(handle.ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.NotEmpty(t, container.HostConfig.Devices, "Expected one device")
	assert.Equal(t, expectedDevice, container.HostConfig.Devices[0], "Incorrect device ")
}

func TestDockerDriver_Entrypoint(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	entrypoint := []string{"/bin/sh", "-c"}
	task, _, _ := dockerTask(t)
	task.Config["entrypoint"] = entrypoint

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle)

	container, err := client.InspectContainer(handle.ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	require.Len(t, container.Config.Entrypoint, 2, "Expected one entrypoint")
	require.Equal(t, entrypoint, container.Config.Entrypoint, "Incorrect entrypoint ")
}

func TestDockerDriver_Kill(t *testing.T) {
	assert := assert.New(t)
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	// Tasks started with a signal that is not supported should not error
	task := &structs.Task{
		Name:       "nc-demo",
		Driver:     "docker",
		KillSignal: "SIGKILL",
		Config: map[string]interface{}{
			"load":    "busybox.tar",
			"image":   "busybox",
			"command": "/bin/nc",
			"args":    []string{"-l", "127.0.0.1", "-p", "0"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	ctx := testDockerDriverContexts(t, task)
	defer ctx.Destroy()
	d := NewDockerDriver(ctx.DriverCtx)
	copyImage(t, ctx.ExecCtx.TaskDir, "busybox.tar")

	_, err := d.Prestart(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("error in prestart: %v", err)
	}

	resp, err := d.Start(ctx.ExecCtx, task)
	assert.Nil(err)
	assert.NotNil(resp.Handle)

	handle := resp.Handle.(*DockerHandle)
	waitForExist(t, client, handle)
	err = handle.Kill()
	assert.Nil(err)
}

func TestDockerDriver_ReadonlyRootfs(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, _, _ := dockerTask(t)
	task.Config["readonly_rootfs"] = true

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle)

	container, err := client.InspectContainer(handle.ContainerID())
	assert.Nil(t, err, "Error inspecting container: %v", err)

	assert.True(t, container.HostConfig.ReadonlyRootfs, "ReadonlyRootfs option not set")
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
	task, _, _ := dockerTask(t)
	tctx := testDockerDriverContexts(t, task)
	driver := NewDockerDriver(tctx.DriverCtx).(*DockerDriver)
	driver.driverConfig = &DockerDriverConfig{ImageName: "test"}

	// assert volume error is recoverable
	_, err := driver.createContainer(fakeDockerClient{}, docker.CreateContainerOptions{})
	require.True(t, structs.IsRecoverable(err))
}

func TestDockerDriver_AdvertiseIPv6Address(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	expectedPrefix := "2001:db8:1::242:ac11"
	expectedAdvertise := true
	task := &structs.Task{
		Name:   "nc-demo",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":                  "busybox",
			"load":                   "busybox.tar",
			"command":                "/bin/nc",
			"args":                   []string{"-l", "127.0.0.1", "-p", "0"},
			"advertise_ipv6_address": expectedAdvertise,
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
	}

	client := newTestDockerClient(t)

	// Make sure IPv6 is enabled
	net, err := client.NetworkInfo("bridge")
	if err != nil {
		t.Skip("error retrieving bridge network information, skipping")
	}
	if net == nil || !net.EnableIPv6 {
		t.Skip("IPv6 not enabled on bridge network, skipping")
	}

	tctx := testDockerDriverContexts(t, task)
	driver := NewDockerDriver(tctx.DriverCtx)
	copyImage(t, tctx.ExecCtx.TaskDir, "busybox.tar")
	defer tctx.Destroy()

	presp, err := driver.Prestart(tctx.ExecCtx, task)
	defer driver.Cleanup(tctx.ExecCtx, presp.CreatedResources)
	if err != nil {
		t.Fatalf("Error in prestart: %v", err)
	}

	sresp, err := driver.Start(tctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("Error in start: %v", err)
	}

	if sresp.Handle == nil {
		t.Fatalf("handle is nil\nStack\n%s", debug.Stack())
	}

	assert.Equal(t, expectedAdvertise, sresp.Network.AutoAdvertise, "Wrong autoadvertise. Expect: %s, got: %s", expectedAdvertise, sresp.Network.AutoAdvertise)

	if !strings.HasPrefix(sresp.Network.IP, expectedPrefix) {
		t.Fatalf("Got IP address %q want ip address with prefix %q", sresp.Network.IP, expectedPrefix)
	}

	defer sresp.Handle.Kill()
	handle := sresp.Handle.(*DockerHandle)

	waitForExist(t, client, handle)

	container, err := client.InspectContainer(handle.ContainerID())
	if err != nil {
		t.Fatalf("Error inspecting container: %v", err)
	}

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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, _, _ := dockerTask(t)
	task.Config["cpu_hard_limit"] = true
	task.Config["cpu_cfs_period"] = 1000000

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle)

	container, err := client.InspectContainer(handle.ContainerID())
	assert.Nil(t, err, "Error inspecting container: %v", err)
}*/

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
