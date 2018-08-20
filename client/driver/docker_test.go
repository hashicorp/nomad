package driver

import (
	"fmt"
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
	sockaddr "github.com/hashicorp/go-sockaddr"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/fingerprint"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	tu "github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
func dockerTask(t *testing.T) (*structs.Task, int, int) {
	ports := freeport.GetT(t, 2)
	dockerReserved := ports[0]
	dockerDynamic := ports[1]
	return &structs.Task{
		Name:   "redis-demo",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "busybox",
			"load":    "busybox.tar",
			"command": "/bin/nc",
			"args":    []string{"-l", "127.0.0.1", "-p", "0"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
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
	}, dockerReserved, dockerDynamic
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
func dockerSetup(t *testing.T, task *structs.Task) (*docker.Client, *DockerHandle, func()) {
	client := newTestDockerClient(t)
	return dockerSetupWithClient(t, task, client)
}

func testDockerDriverContexts(t *testing.T, task *structs.Task) *testContext {
	tctx := testDriverContexts(t, task)

	// Drop the delay
	tctx.DriverCtx.config.Options = make(map[string]string)
	tctx.DriverCtx.config.Options[dockerImageRemoveDelayConfigOption] = "1s"

	return tctx
}

func dockerSetupWithClient(t *testing.T, task *structs.Task, client *docker.Client) (*docker.Client, *DockerHandle, func()) {
	t.Helper()
	tctx := testDockerDriverContexts(t, task)
	driver := NewDockerDriver(tctx.DriverCtx)
	copyImage(t, tctx.ExecCtx.TaskDir, "busybox.tar")

	presp, err := driver.Prestart(tctx.ExecCtx, task)
	if err != nil {
		if presp != nil && presp.CreatedResources != nil {
			driver.Cleanup(tctx.ExecCtx, presp.CreatedResources)
		}
		tctx.AllocDir.Destroy()
		t.Fatalf("error in prestart: %v", err)
	}
	// Update the exec ctx with the driver network env vars
	tctx.ExecCtx.TaskEnv = tctx.EnvBuilder.SetDriverNetwork(presp.Network).Build()

	sresp, err := driver.Start(tctx.ExecCtx, task)
	if err != nil {
		driver.Cleanup(tctx.ExecCtx, presp.CreatedResources)
		tctx.AllocDir.Destroy()
		t.Fatalf("Failed to start driver: %s\nStack\n%s", err, debug.Stack())
	}

	if sresp.Handle == nil {
		driver.Cleanup(tctx.ExecCtx, presp.CreatedResources)
		tctx.AllocDir.Destroy()
		t.Fatalf("handle is nil\nStack\n%s", debug.Stack())
	}

	// At runtime this is handled by TaskRunner
	tctx.ExecCtx.TaskEnv = tctx.EnvBuilder.SetDriverNetwork(sresp.Network).Build()

	cleanup := func() {
		driver.Cleanup(tctx.ExecCtx, presp.CreatedResources)
		sresp.Handle.Kill()
		tctx.AllocDir.Destroy()
	}

	return client, sresp.Handle.(*DockerHandle), cleanup
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

// This test should always pass, even if docker daemon is not available
func TestDockerDriver_Fingerprint(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}

	ctx := testDockerDriverContexts(t, &structs.Task{Name: "foo", Driver: "docker", Resources: basicResources})
	//ctx.DriverCtx.config.Options = map[string]string{"docker.cleanup.image": "false"}
	defer ctx.AllocDir.Destroy()
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
}

func TestDockerDriver_StartOpen_Wait(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task := &structs.Task{
		Name:   "nc-demo",
		Driver: "docker",
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
	//ctx.DriverCtx.config.Options = map[string]string{"docker.cleanup.image": "false"}
	defer ctx.AllocDir.Destroy()
	d := NewDockerDriver(ctx.DriverCtx)
	copyImage(t, ctx.ExecCtx.TaskDir, "busybox.tar")

	_, err := d.Prestart(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("error in prestart: %v", err)
	}

	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Handle == nil {
		t.Fatalf("missing handle")
	}
	defer resp.Handle.Kill()

	// Attempt to open
	resp2, err := d.Open(ctx.ExecCtx, resp.Handle.ID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2 == nil {
		t.Fatalf("missing handle")
	}
}

func TestDockerDriver_Start_Wait(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}
	task := &structs.Task{
		Name:   "nc-demo",
		Driver: "docker",
		Config: map[string]interface{}{
			"load":    "busybox.tar",
			"image":   "busybox",
			"command": "/bin/echo",
			"args":    []string{"hello"},
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

	_, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	// Update should be a no-op
	err := handle.Update(task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case res := <-handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
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
	task := &structs.Task{
		Name:   "nc-demo",
		Driver: "docker",
		Config: map[string]interface{}{
			"load":    "busybox.tar",
			"image":   "busybox",
			"command": "sleep",
			"args":    []string{"9000"},
		},
		Resources: &structs.Resources{
			MemoryMB: 100,
			CPU:      100,
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      1,
			MaxFileSizeMB: 10,
		},
	}

	tctx := testDockerDriverContexts(t, task)
	defer tctx.AllocDir.Destroy()

	copyImage(t, tctx.ExecCtx.TaskDir, "busybox.tar")
	client := newTestDockerClient(t)
	driver := NewDockerDriver(tctx.DriverCtx).(*DockerDriver)
	driverConfig := &DockerDriverConfig{ImageName: "busybox", LoadImage: "busybox.tar"}
	if _, err := driver.loadImage(driverConfig, client, tctx.ExecCtx.TaskDir); err != nil {
		t.Fatalf("error loading image: %v", err)
	}

	// Create a container of the same name but don't start it. This mimics
	// the case of dockerd getting restarted and stopping containers while
	// Nomad is watching them.
	opts := docker.CreateContainerOptions{
		Name: fmt.Sprintf("%s-%s", task.Name, tctx.DriverCtx.allocID),
		Config: &docker.Config{
			Image: "busybox",
			Cmd:   []string{"sleep", "9000"},
		},
	}
	if _, err := client.CreateContainer(opts); err != nil {
		t.Fatalf("error creating initial container: %v", err)
	}

	// Now assert that the driver can still start normally
	presp, err := driver.Prestart(tctx.ExecCtx, task)
	if err != nil {
		driver.Cleanup(tctx.ExecCtx, presp.CreatedResources)
		t.Fatalf("error in prestart: %v", err)
	}
	defer driver.Cleanup(tctx.ExecCtx, presp.CreatedResources)

	sresp, err := driver.Start(tctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("failed to start driver: %s", err)
	}
	handle := sresp.Handle.(*DockerHandle)
	waitForExist(t, client, handle)
	handle.Kill()
}

func TestDockerDriver_Start_LoadImage(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}
	task := &structs.Task{
		Name:   "busybox-demo",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "busybox",
			"load":    "busybox.tar",
			"command": "/bin/sh",
			"args": []string{
				"-c",
				"echo hello > $NOMAD_TASK_DIR/output",
			},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
	}

	ctx := testDockerDriverContexts(t, task)
	//ctx.DriverCtx.config.Options = map[string]string{"docker.cleanup.image": "false"}
	defer ctx.AllocDir.Destroy()
	d := NewDockerDriver(ctx.DriverCtx)

	// Copy the image into the task's directory
	copyImage(t, ctx.ExecCtx.TaskDir, "busybox.tar")

	_, err := d.Prestart(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer resp.Handle.Kill()

	select {
	case res := <-resp.Handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}

	// Check that data was written to the shared alloc directory.
	outputFile := filepath.Join(ctx.ExecCtx.TaskDir.LocalDir, "output")
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
	task := &structs.Task{
		Name:   "busybox-demo",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "127.0.1.1:32121/foo", // bad path
			"command": "/bin/echo",
			"args": []string{
				"hello",
			},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
	}

	ctx := testDockerDriverContexts(t, task)
	//ctx.DriverCtx.config.Options = map[string]string{"docker.cleanup.image": "false"}
	defer ctx.AllocDir.Destroy()
	d := NewDockerDriver(ctx.DriverCtx)

	_, err := d.Prestart(ctx.ExecCtx, task)
	if err == nil {
		t.Fatalf("want error in prestart: %v", err)
	}

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
	task := &structs.Task{
		Name:   "nc-demo",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "busybox",
			"load":    "busybox.tar",
			"command": "/bin/sh",
			"args": []string{
				"-c",
				fmt.Sprintf(`sleep 1; echo -n %s > $%s/%s`,
					string(exp), env.AllocDir, file),
			},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
	}

	ctx := testDockerDriverContexts(t, task)
	//ctx.DriverCtx.config.Options = map[string]string{"docker.cleanup.image": "false"}
	defer ctx.AllocDir.Destroy()
	d := NewDockerDriver(ctx.DriverCtx)
	copyImage(t, ctx.ExecCtx.TaskDir, "busybox.tar")

	_, err := d.Prestart(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer resp.Handle.Kill()

	select {
	case res := <-resp.Handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}

	// Check that data was written to the shared alloc directory.
	outputFile := filepath.Join(ctx.AllocDir.SharedDir, file)
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
	task := &structs.Task{
		Name:   "nc-demo",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "busybox",
			"load":    "busybox.tar",
			"command": "/bin/sleep",
			"args":    []string{"10"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	_, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	go func() {
		time.Sleep(100 * time.Millisecond)
		err := handle.Kill()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	select {
	case res := <-handle.WaitCh():
		if res.Successful() {
			t.Fatalf("should err: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*10) * time.Second):
		t.Fatalf("timeout")
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
	task := &structs.Task{
		Name:   "nc-demo",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "busybox",
			"load":    "busybox.tar",
			"command": "/bin/sleep",
			"args":    []string{"10"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources:   basicResources,
		KillTimeout: timeout,
		KillSignal:  "SIGUSR1", // Pick something that doesn't actually kill it
	}

	_, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	// Reduce the timeout for the docker client.
	handle.client.SetTimeout(1 * time.Second)

	// Kill the task
	var killSent, killed time.Time
	go func() {
		killSent = time.Now()
		if err := handle.Kill(); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	select {
	case <-handle.WaitCh():
		killed = time.Now()
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout")
	}

	if killed.Sub(killSent) < timeout {
		t.Fatalf("kill timeout not respected")
	}
}

func TestDockerDriver_StartN(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task1, _, _ := dockerTask(t)
	task2, _, _ := dockerTask(t)
	task3, _, _ := dockerTask(t)
	taskList := []*structs.Task{task1, task2, task3}

	handles := make([]DriverHandle, len(taskList))

	t.Logf("Starting %d tasks", len(taskList))

	// Let's spin up a bunch of things
	for idx, task := range taskList {
		ctx := testDockerDriverContexts(t, task)
		//ctx.DriverCtx.config.Options = map[string]string{"docker.cleanup.image": "false"}
		defer ctx.AllocDir.Destroy()
		d := NewDockerDriver(ctx.DriverCtx)
		copyImage(t, ctx.ExecCtx.TaskDir, "busybox.tar")

		_, err := d.Prestart(ctx.ExecCtx, task)
		if err != nil {
			t.Fatalf("error in prestart #%d: %v", idx+1, err)
		}
		resp, err := d.Start(ctx.ExecCtx, task)
		if err != nil {
			t.Errorf("Failed starting task #%d: %s", idx+1, err)
			continue
		}
		handles[idx] = resp.Handle
	}

	t.Log("All tasks are started. Terminating...")

	for idx, handle := range handles {
		if handle == nil {
			t.Errorf("Bad handle for task #%d", idx+1)
			continue
		}

		err := handle.Kill()
		if err != nil {
			t.Errorf("Failed stopping task #%d: %s", idx+1, err)
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

	task1, _, _ := dockerTask(t)
	task1.Config["image"] = "busybox"
	task1.Config["load"] = "busybox.tar"

	task2, _, _ := dockerTask(t)
	task2.Config["image"] = "busybox:musl"
	task2.Config["load"] = "busybox_musl.tar"
	task2.Config["args"] = []string{"-l", "-p", "0"}

	task3, _, _ := dockerTask(t)
	task3.Config["image"] = "busybox:glibc"
	task3.Config["load"] = "busybox_glibc.tar"

	taskList := []*structs.Task{task1, task2, task3}

	handles := make([]DriverHandle, len(taskList))

	t.Logf("Starting %d tasks", len(taskList))
	client := newTestDockerClient(t)

	// Let's spin up a bunch of things
	for idx, task := range taskList {
		ctx := testDockerDriverContexts(t, task)
		//ctx.DriverCtx.config.Options = map[string]string{"docker.cleanup.image": "false"}
		defer ctx.AllocDir.Destroy()
		d := NewDockerDriver(ctx.DriverCtx)
		copyImage(t, ctx.ExecCtx.TaskDir, "busybox.tar")
		copyImage(t, ctx.ExecCtx.TaskDir, "busybox_musl.tar")
		copyImage(t, ctx.ExecCtx.TaskDir, "busybox_glibc.tar")

		_, err := d.Prestart(ctx.ExecCtx, task)
		if err != nil {
			t.Fatalf("error in prestart #%d: %v", idx+1, err)
		}
		resp, err := d.Start(ctx.ExecCtx, task)
		if err != nil {
			t.Errorf("Failed starting task #%d: %s", idx+1, err)
			continue
		}
		handles[idx] = resp.Handle
		waitForExist(t, client, resp.Handle.(*DockerHandle))
	}

	t.Log("All tasks are started. Terminating...")

	for idx, handle := range handles {
		if handle == nil {
			t.Errorf("Bad handle for task #%d", idx+1)
			continue
		}

		err := handle.Kill()
		if err != nil {
			t.Errorf("Failed stopping task #%d: %s", idx+1, err)
		}
	}

	t.Log("Test complete!")
}

func waitForExist(t *testing.T, client *docker.Client, handle *DockerHandle) {
	handle.logger.Printf("[DEBUG] docker.test: waiting for container %s to exist...", handle.ContainerID())
	tu.WaitForResult(func() (bool, error) {
		container, err := client.InspectContainer(handle.ContainerID())
		if err != nil {
			if _, ok := err.(*docker.NoSuchContainer); !ok {
				return false, err
			}
		}

		return container != nil, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
	handle.logger.Printf("[DEBUG] docker.test: ...container %s exists!", handle.ContainerID())
}

func TestDockerDriver_NetworkMode_Host(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}
	expected := "host"

	task := &structs.Task{
		Name:   "nc-demo",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":        "busybox",
			"load":         "busybox.tar",
			"command":      "/bin/nc",
			"args":         []string{"-l", "127.0.0.1", "-p", "0"},
			"network_mode": expected,
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

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle)

	container, err := client.InspectContainer(handle.ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	actual := container.HostConfig.NetworkMode
	if actual != expected {
		t.Fatalf("Got network mode %q; want %q", expected, actual)
	}
}

func TestDockerDriver_NetworkAliases_Bridge(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	// Because go-dockerclient doesn't provide api for query network aliases, just check that
	// a container can be created with a 'network_aliases' property

	// Create network, network-scoped alias is supported only for containers in user defined networks
	client := newTestDockerClient(t)
	networkOpts := docker.CreateNetworkOptions{Name: "foobar", Driver: "bridge"}
	network, err := client.CreateNetwork(networkOpts)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer client.RemoveNetwork(network.ID)

	expected := []string{"foobar"}
	task := &structs.Task{
		Name:   "nc-demo",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":           "busybox",
			"load":            "busybox.tar",
			"command":         "/bin/nc",
			"args":            []string{"-l", "127.0.0.1", "-p", "0"},
			"network_mode":    network.Name,
			"network_aliases": expected,
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

	client, handle, cleanup := dockerSetupWithClient(t, task, client)
	defer cleanup()

	waitForExist(t, client, handle)

	_, err = client.InspectContainer(handle.ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestDockerDriver_Sysctl_Ulimit(t *testing.T) {
	task, _, _ := dockerTask(t)
	expectedUlimits := map[string]string{
		"nproc":  "4242",
		"nofile": "2048:4096",
	}
	task.Config["sysctl"] = []map[string]string{
		{
			"net.core.somaxconn": "16384",
		},
	}
	task.Config["ulimit"] = []map[string]string{
		expectedUlimits,
	}

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle)

	container, err := client.InspectContainer(handle.ContainerID())
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
	brokenConfigs := []interface{}{
		map[string]interface{}{
			"nofile": "",
		},
		map[string]interface{}{
			"nofile": "abc:1234",
		},
		map[string]interface{}{
			"nofile": "1234:abc",
		},
	}

	test_cases := []struct {
		ulimitConfig interface{}
		err          error
	}{
		{[]interface{}{brokenConfigs[0]}, fmt.Errorf("Malformed ulimit specification nofile: \"\", cannot be empty")},
		{[]interface{}{brokenConfigs[1]}, fmt.Errorf("Malformed soft ulimit nofile: abc:1234")},
		{[]interface{}{brokenConfigs[2]}, fmt.Errorf("Malformed hard ulimit nofile: 1234:abc")},
	}

	for _, tc := range test_cases {
		task, _, _ := dockerTask(t)
		task.Config["ulimit"] = tc.ulimitConfig

		ctx := testDockerDriverContexts(t, task)
		driver := NewDockerDriver(ctx.DriverCtx)
		copyImage(t, ctx.ExecCtx.TaskDir, "busybox.tar")
		defer ctx.AllocDir.Destroy()

		_, err := driver.Prestart(ctx.ExecCtx, task)
		assert.NotNil(t, err, "Expected non nil error")
		assert.Equal(t, err.Error(), tc.err.Error(), "unexpected error in prestart, got %v, expected %v", err, tc.err)
	}
}

func TestDockerDriver_Labels(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, _, _ := dockerTask(t)
	task.Config["labels"] = []map[string]string{
		{
			"label1": "value1",
			"label2": "value2",
		},
	}

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle)

	container, err := client.InspectContainer(handle.ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if want, got := 2, len(container.Config.Labels); want != got {
		t.Errorf("Wrong labels count for docker job. Expect: %d, got: %d", want, got)
	}

	if want, got := "value1", container.Config.Labels["label1"]; want != got {
		t.Errorf("Wrong label value docker job. Expect: %s, got: %s", want, got)
	}
}

func TestDockerDriver_ForcePull_IsInvalidConfig(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, _, _ := dockerTask(t)
	task.Config["force_pull"] = "nothing"

	ctx := testDockerDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	//ctx.DriverCtx.config.Options = map[string]string{"docker.cleanup.image": "false"}
	driver := NewDockerDriver(ctx.DriverCtx)

	if _, err := driver.Prestart(ctx.ExecCtx, task); err == nil {
		t.Fatalf("error expected in prestart")
	}
}

func TestDockerDriver_ForcePull(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, _, _ := dockerTask(t)
	task.Config["force_pull"] = "true"

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle)

	_, err := client.InspectContainer(handle.ContainerID())
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

	task, _, _ := dockerTask(t)
	task.Config["load"] = ""
	task.Config["image"] = "library/busybox@sha256:58ac43b2cc92c687a32c8be6278e50a063579655fe3090125dcb2af0ff9e1a64"
	localDigest := "sha256:8ac48589692a53a9b8c2d1ceaa6b402665aa7fe667ba51ccc03002300856d8c7"
	task.Config["force_pull"] = "true"

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle)

	container, err := client.InspectContainer(handle.ContainerID())
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

	task, _, _ := dockerTask(t)
	task.Config["security_opt"] = []string{"seccomp=unconfined"}

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle)

	container, err := client.InspectContainer(handle.ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(task.Config["security_opt"], container.HostConfig.SecurityOpt) {
		t.Errorf("Security Opts don't match.\nExpected:\n%s\nGot:\n%s\n", task.Config["security_opt"], container.HostConfig.SecurityOpt)
	}
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
			task, _, _ := dockerTask(t)
			if len(tc.CapAdd) > 0 {
				task.Config["cap_add"] = tc.CapAdd
			}
			if len(tc.CapDrop) > 0 {
				task.Config["cap_drop"] = tc.CapDrop
			}

			tctx := testDockerDriverContexts(t, task)
			if tc.Whitelist != "" {
				tctx.DriverCtx.config.Options[dockerCapsWhitelistConfigOption] = tc.Whitelist
			}

			driver := NewDockerDriver(tctx.DriverCtx)
			copyImage(t, tctx.ExecCtx.TaskDir, "busybox.tar")
			defer tctx.AllocDir.Destroy()

			presp, err := driver.Prestart(tctx.ExecCtx, task)
			defer driver.Cleanup(tctx.ExecCtx, presp.CreatedResources)
			if err != nil {
				t.Fatalf("Error in prestart: %v", err)
			}

			sresp, err := driver.Start(tctx.ExecCtx, task)
			if err == nil && tc.StartError != "" {
				t.Fatalf("Expected error in start: %v", tc.StartError)
			} else if err != nil {
				if tc.StartError == "" {
					t.Fatalf("Failed to start driver: %s\nStack\n%s", err, debug.Stack())
				} else if !strings.Contains(err.Error(), tc.StartError) {
					t.Fatalf("Expect error containing \"%s\", got %v", tc.StartError, err)
				}
				return
			}

			if sresp.Handle == nil {
				t.Fatalf("handle is nil\nStack\n%s", debug.Stack())
			}
			defer sresp.Handle.Kill()
			handle := sresp.Handle.(*DockerHandle)

			waitForExist(t, client, handle)

			container, err := client.InspectContainer(handle.ContainerID())
			if err != nil {
				t.Fatalf("Error inspecting container: %v", err)
			}

			if !reflect.DeepEqual(tc.CapAdd, container.HostConfig.CapAdd) {
				t.Errorf("CapAdd doesn't match.\nExpected:\n%s\nGot:\n%s\n", tc.CapAdd, container.HostConfig.CapAdd)
			}

			if !reflect.DeepEqual(tc.CapDrop, container.HostConfig.CapDrop) {
				t.Errorf("CapDrop doesn't match.\nExpected:\n%s\nGot:\n%s\n", tc.CapDrop, container.HostConfig.CapDrop)
			}
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

	task, _, _ := dockerTask(t)
	task.Config["dns_servers"] = []string{"8.8.8.8", "8.8.4.4"}
	task.Config["dns_search_domains"] = []string{"example.com", "example.org", "example.net"}
	task.Config["dns_options"] = []string{"ndots:1"}

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle)

	container, err := client.InspectContainer(handle.ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(task.Config["dns_servers"], container.HostConfig.DNS) {
		t.Errorf("DNS Servers don't match.\nExpected:\n%s\nGot:\n%s\n", task.Config["dns_servers"], container.HostConfig.DNS)
	}

	if !reflect.DeepEqual(task.Config["dns_search_domains"], container.HostConfig.DNSSearch) {
		t.Errorf("DNS Search Domains don't match.\nExpected:\n%s\nGot:\n%s\n", task.Config["dns_search_domains"], container.HostConfig.DNSSearch)
	}

	if !reflect.DeepEqual(task.Config["dns_options"], container.HostConfig.DNSOptions) {
		t.Errorf("DNS Options don't match.\nExpected:\n%s\nGot:\n%s\n", task.Config["dns_options"], container.HostConfig.DNSOptions)
	}
}

func TestDockerDriver_MACAddress(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, _, _ := dockerTask(t)
	task.Config["mac_address"] = "00:16:3e:00:00:00"

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle)

	container, err := client.InspectContainer(handle.ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if container.NetworkSettings.MacAddress != task.Config["mac_address"] {
		t.Errorf("expected mac_address=%q but found %q", task.Config["mac_address"], container.NetworkSettings.MacAddress)
	}
}

func TestDockerWorkDir(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, _, _ := dockerTask(t)
	task.Config["work_dir"] = "/some/path"

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	container, err := client.InspectContainer(handle.ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if want, got := "/some/path", container.Config.WorkingDir; want != got {
		t.Errorf("Wrong working directory for docker job. Expect: %s, got: %s", want, got)
	}
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

	task, res, dyn := dockerTask(t)

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle)

	container, err := client.InspectContainer(handle.ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that the correct ports are EXPOSED
	expectedExposedPorts := map[docker.Port]struct{}{
		docker.Port(fmt.Sprintf("%d/tcp", res)): {},
		docker.Port(fmt.Sprintf("%d/udp", res)): {},
		docker.Port(fmt.Sprintf("%d/tcp", dyn)): {},
		docker.Port(fmt.Sprintf("%d/udp", dyn)): {},
	}

	if !reflect.DeepEqual(container.Config.ExposedPorts, expectedExposedPorts) {
		t.Errorf("Exposed ports don't match.\nExpected:\n%s\nGot:\n%s\n", expectedExposedPorts, container.Config.ExposedPorts)
	}

	// Verify that the correct ports are FORWARDED
	expectedPortBindings := map[docker.Port][]docker.PortBinding{
		docker.Port(fmt.Sprintf("%d/tcp", res)): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", res)}},
		docker.Port(fmt.Sprintf("%d/udp", res)): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", res)}},
		docker.Port(fmt.Sprintf("%d/tcp", dyn)): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", dyn)}},
		docker.Port(fmt.Sprintf("%d/udp", dyn)): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", dyn)}},
	}

	if !reflect.DeepEqual(container.HostConfig.PortBindings, expectedPortBindings) {
		t.Errorf("Forwarded ports don't match.\nExpected:\n%s\nGot:\n%s\n", expectedPortBindings, container.HostConfig.PortBindings)
	}

	expectedEnvironment := map[string]string{
		"NOMAD_ADDR_main":  fmt.Sprintf("127.0.0.1:%d", res),
		"NOMAD_ADDR_REDIS": fmt.Sprintf("127.0.0.1:%d", dyn),
	}

	for key, val := range expectedEnvironment {
		search := fmt.Sprintf("%s=%s", key, val)
		if !inSlice(search, container.Config.Env) {
			t.Errorf("Expected to find %s in container environment: %+v", search, container.Config.Env)
		}
	}
}

func TestDockerDriver_PortsMapping(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, res, dyn := dockerTask(t)
	task.Config["port_map"] = []map[string]string{
		{
			"main":  "8080",
			"REDIS": "6379",
		},
	}

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle)

	container, err := client.InspectContainer(handle.ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that the correct ports are EXPOSED
	expectedExposedPorts := map[docker.Port]struct{}{
		docker.Port("8080/tcp"): {},
		docker.Port("8080/udp"): {},
		docker.Port("6379/tcp"): {},
		docker.Port("6379/udp"): {},
	}

	if !reflect.DeepEqual(container.Config.ExposedPorts, expectedExposedPorts) {
		t.Errorf("Exposed ports don't match.\nExpected:\n%s\nGot:\n%s\n", expectedExposedPorts, container.Config.ExposedPorts)
	}

	// Verify that the correct ports are FORWARDED
	expectedPortBindings := map[docker.Port][]docker.PortBinding{
		docker.Port("8080/tcp"): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", res)}},
		docker.Port("8080/udp"): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", res)}},
		docker.Port("6379/tcp"): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", dyn)}},
		docker.Port("6379/udp"): {{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", dyn)}},
	}

	if !reflect.DeepEqual(container.HostConfig.PortBindings, expectedPortBindings) {
		t.Errorf("Forwarded ports don't match.\nExpected:\n%s\nGot:\n%s\n", expectedPortBindings, container.HostConfig.PortBindings)
	}

	expectedEnvironment := map[string]string{
		"NOMAD_PORT_main":      "8080",
		"NOMAD_PORT_REDIS":     "6379",
		"NOMAD_HOST_PORT_main": strconv.Itoa(res),
	}

	sort.Strings(container.Config.Env)
	for key, val := range expectedEnvironment {
		search := fmt.Sprintf("%s=%s", key, val)
		if !inSlice(search, container.Config.Env) {
			t.Errorf("Expected to find %s in container environment:\n%s\n\n", search, strings.Join(container.Config.Env, "\n"))
		}
	}
}

func TestDockerDriver_User(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task := &structs.Task{
		Name:   "redis-demo",
		User:   "alice",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "busybox",
			"load":    "busybox.tar",
			"command": "/bin/sleep",
			"args":    []string{"10000"},
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

	ctx := testDockerDriverContexts(t, task)
	//ctx.DriverCtx.config.Options = map[string]string{"docker.cleanup.image": "false"}
	driver := NewDockerDriver(ctx.DriverCtx)
	defer ctx.AllocDir.Destroy()
	copyImage(t, ctx.ExecCtx.TaskDir, "busybox.tar")

	_, err := driver.Prestart(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("error in prestart: %v", err)
	}

	// It should fail because the user "alice" does not exist on the given
	// image.
	resp, err := driver.Start(ctx.ExecCtx, task)
	if err == nil {
		resp.Handle.Kill()
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

	task := &structs.Task{
		Name:   "redis-demo",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "busybox",
			"load":    "busybox.tar",
			"command": "/bin/echo",
			"args":    []string{"hello"},
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

	_, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	// Update should be a no-op
	err := handle.Update(task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case res := <-handle.WaitCh():
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
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task := &structs.Task{
		Name:   "sleep",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "busybox",
			"load":    "busybox.tar",
			"command": "/bin/sleep",
			"args":    []string{"100"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	_, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle)

	go func() {
		time.Sleep(3 * time.Second)
		ru, err := handle.Stats()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if ru.ResourceUsage == nil {
			handle.Kill()
			t.Fatalf("expected resource usage")
		}
		err = handle.Kill()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	select {
	case res := <-handle.WaitCh():
		if res.Successful() {
			t.Fatalf("should err: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*10) * time.Second):
		t.Fatalf("timeout")
	}
}

func setupDockerVolumes(t *testing.T, cfg *config.Config, hostpath string) (*structs.Task, Driver, *ExecContext, string, func()) {
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	randfn := fmt.Sprintf("test-%d", rand.Int())
	hostfile := filepath.Join(hostpath, randfn)
	containerPath := "/mnt/vol"
	containerFile := filepath.Join(containerPath, randfn)

	task := &structs.Task{
		Name:   "ls",
		Env:    map[string]string{"VOL_PATH": containerPath},
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "busybox",
			"load":    "busybox.tar",
			"command": "touch",
			"args":    []string{containerFile},
			"volumes": []string{fmt.Sprintf("%s:${VOL_PATH}", hostpath)},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	// Build alloc and task directory structure
	allocDir := allocdir.NewAllocDir(testlog.Logger(t), filepath.Join(cfg.AllocDir, uuid.Generate()))
	if err := allocDir.Build(); err != nil {
		t.Fatalf("failed to build alloc dir: %v", err)
	}
	taskDir := allocDir.NewTaskDir(task.Name)
	if err := taskDir.Build(false, nil, cstructs.FSIsolationImage); err != nil {
		allocDir.Destroy()
		t.Fatalf("failed to build task dir: %v", err)
	}
	copyImage(t, taskDir, "busybox.tar")

	// Setup driver
	alloc := mock.Alloc()
	logger := testlog.Logger(t)
	emitter := func(m string, args ...interface{}) {
		logger.Printf("[EVENT] "+m, args...)
	}
	driverCtx := NewDriverContext(alloc.Job.Name, alloc.TaskGroup, task.Name, alloc.ID, cfg, cfg.Node, testlog.Logger(t), emitter)
	driver := NewDockerDriver(driverCtx)

	// Setup execCtx
	envBuilder := env.NewBuilder(cfg.Node, alloc, task, cfg.Region)
	SetEnvvars(envBuilder, driver.FSIsolation(), taskDir, cfg)
	execCtx := NewExecContext(taskDir, envBuilder.Build())

	// Setup cleanup function
	cleanup := func() {
		allocDir.Destroy()
		if filepath.IsAbs(hostpath) {
			os.RemoveAll(hostpath)
		}
	}
	return task, driver, execCtx, hostfile, cleanup
}

func TestDockerDriver_VolumesDisabled(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	cfg := testConfig(t)
	cfg.Options = map[string]string{
		dockerVolumesConfigOption: "false",
		"docker.cleanup.image":    "false",
	}

	{
		tmpvol, err := ioutil.TempDir("", "nomadtest_docker_volumesdisabled")
		if err != nil {
			t.Fatalf("error creating temporary dir: %v", err)
		}

		task, driver, execCtx, _, cleanup := setupDockerVolumes(t, cfg, tmpvol)
		defer cleanup()

		_, err = driver.Prestart(execCtx, task)
		if err != nil {
			t.Fatalf("error in prestart: %v", err)
		}
		if _, err := driver.Start(execCtx, task); err == nil {
			t.Fatalf("Started driver successfully when volumes should have been disabled.")
		}
	}

	// Relative paths should still be allowed
	{
		task, driver, execCtx, fn, cleanup := setupDockerVolumes(t, cfg, ".")
		defer cleanup()

		_, err := driver.Prestart(execCtx, task)
		if err != nil {
			t.Fatalf("error in prestart: %v", err)
		}
		resp, err := driver.Start(execCtx, task)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		defer resp.Handle.Kill()

		select {
		case res := <-resp.Handle.WaitCh():
			if !res.Successful() {
				t.Fatalf("unexpected err: %v", res)
			}
		case <-time.After(time.Duration(tu.TestMultiplier()*10) * time.Second):
			t.Fatalf("timeout")
		}

		if _, err := ioutil.ReadFile(filepath.Join(execCtx.TaskDir.Dir, fn)); err != nil {
			t.Fatalf("unexpected error reading %s: %v", fn, err)
		}
	}

	// Volume Drivers should be rejected (error)
	{
		task, driver, execCtx, _, cleanup := setupDockerVolumes(t, cfg, "fake_flocker_vol")
		defer cleanup()
		task.Config["volume_driver"] = "flocker"

		if _, err := driver.Prestart(execCtx, task); err != nil {
			t.Fatalf("error in prestart: %v", err)
		}
		if _, err := driver.Start(execCtx, task); err == nil {
			t.Fatalf("Started driver successfully when volume drivers should have been disabled.")
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

	cfg := testConfig(t)

	tmpvol, err := ioutil.TempDir("", "nomadtest_docker_volumesenabled")
	if err != nil {
		t.Fatalf("error creating temporary dir: %v", err)
	}

	// Evaluate symlinks so it works on MacOS
	tmpvol, err = filepath.EvalSymlinks(tmpvol)
	if err != nil {
		t.Fatalf("error evaluating symlinks: %v", err)
	}

	task, driver, execCtx, hostpath, cleanup := setupDockerVolumes(t, cfg, tmpvol)
	defer cleanup()

	_, err = driver.Prestart(execCtx, task)
	if err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	resp, err := driver.Start(execCtx, task)
	if err != nil {
		t.Fatalf("Failed to start docker driver: %v", err)
	}
	defer resp.Handle.Kill()

	select {
	case res := <-resp.Handle.WaitCh():
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

	goodMount := map[string]interface{}{
		"target": "/nomad",
		"volume_options": []interface{}{
			map[string]interface{}{
				"labels": []interface{}{
					map[string]string{"foo": "bar"},
				},
				"driver_config": []interface{}{
					map[string]interface{}{
						"name": "local",
						"options": []interface{}{
							map[string]interface{}{
								"foo": "bar",
							},
						},
					},
				},
			},
		},
		"readonly": true,
		"source":   "test",
	}

	cases := []struct {
		Name   string
		Mounts []interface{}
		Error  string
	}{
		{
			Name:   "good-one",
			Error:  "",
			Mounts: []interface{}{goodMount},
		},
		{
			Name:   "good-many",
			Error:  "",
			Mounts: []interface{}{goodMount, goodMount, goodMount},
		},
		{
			Name:  "multiple volume options",
			Error: "Only one volume_options stanza allowed",
			Mounts: []interface{}{
				map[string]interface{}{
					"target": "/nomad",
					"volume_options": []interface{}{
						map[string]interface{}{
							"driver_config": []interface{}{
								map[string]interface{}{
									"name": "local",
								},
							},
						},
						map[string]interface{}{
							"driver_config": []interface{}{
								map[string]interface{}{
									"name": "local",
								},
							},
						},
					},
				},
			},
		},
		{
			Name:  "multiple driver configs",
			Error: "volume driver config may only be specified once",
			Mounts: []interface{}{
				map[string]interface{}{
					"target": "/nomad",
					"volume_options": []interface{}{
						map[string]interface{}{
							"driver_config": []interface{}{
								map[string]interface{}{
									"name": "local",
								},
								map[string]interface{}{
									"name": "local",
								},
							},
						},
					},
				},
			},
		},
		{
			Name:  "multiple volume labels",
			Error: "labels may only be",
			Mounts: []interface{}{
				map[string]interface{}{
					"target": "/nomad",
					"volume_options": []interface{}{
						map[string]interface{}{
							"labels": []interface{}{
								map[string]string{"foo": "bar"},
								map[string]string{"baz": "bam"},
							},
						},
					},
				},
			},
		},
		{
			Name:  "multiple driver options",
			Error: "driver options may only",
			Mounts: []interface{}{
				map[string]interface{}{
					"target": "/nomad",
					"volume_options": []interface{}{
						map[string]interface{}{
							"driver_config": []interface{}{
								map[string]interface{}{
									"name": "local",
									"options": []interface{}{
										map[string]interface{}{
											"foo": "bar",
										},
										map[string]interface{}{
											"bam": "bar",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	task := &structs.Task{
		Name:   "redis-demo",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "busybox",
			"load":    "busybox.tar",
			"command": "/bin/sleep",
			"args":    []string{"10000"},
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

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			// Build the task
			task.Config["mounts"] = c.Mounts

			ctx := testDockerDriverContexts(t, task)
			driver := NewDockerDriver(ctx.DriverCtx)
			copyImage(t, ctx.ExecCtx.TaskDir, "busybox.tar")
			defer ctx.AllocDir.Destroy()

			_, err := driver.Prestart(ctx.ExecCtx, task)
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
	task := &structs.Task{
		Name:   "cleanup_test",
		Driver: "docker",
		Config: map[string]interface{}{
			"image": imageName,
		},
	}
	tctx := testDockerDriverContexts(t, task)
	defer tctx.AllocDir.Destroy()

	// Run Prestart
	driver := NewDockerDriver(tctx.DriverCtx).(*DockerDriver)
	resp, err := driver.Prestart(tctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	res := resp.CreatedResources
	if len(res.Resources) == 0 || len(res.Resources[dockerImageResKey]) == 0 {
		t.Fatalf("no created resources: %#v", res)
	}

	// Cleanup
	rescopy := res.Copy()
	if err := driver.Cleanup(tctx.ExecCtx, rescopy); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Make sure rescopy is updated
	if len(rescopy.Resources) > 0 {
		t.Errorf("Cleanup should have cleared resource map: %#v", rescopy.Resources)
	}

	// Ensure image was removed
	tu.WaitForResult(func() (bool, error) {
		if _, err := client.InspectImage(driver.driverConfig.ImageName); err == nil {
			return false, fmt.Errorf("image exists but should have been removed. Does another %v container exist?", imageName)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// The image doesn't exist which shouldn't be an error when calling
	// Cleanup, so call it again to make sure.
	if err := driver.Cleanup(tctx.ExecCtx, res.Copy()); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
}

func copyImage(t *testing.T, taskDir *allocdir.TaskDir, image string) {
	dst := filepath.Join(taskDir.LocalDir, image)
	copyFile(filepath.Join("./test-resources/docker", image), dst, t)
}

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
		defer ctx.AllocDir.Destroy()

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
	defer ctx.AllocDir.Destroy()
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
			"image":   "busybox",
			"load":    "busybox.tar",
			"command": "/bin/nc",
			"args":    []string{"-l", "127.0.0.1", "-p", "0"},
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
	defer tctx.AllocDir.Destroy()

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
	assert.Equal(t, int64(1000000), container.HostConfig.CPUPeriod, "cpu_cfs_period option incorrectly set")
}
