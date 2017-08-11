package driver

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	sockaddr "github.com/hashicorp/go-sockaddr"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/env"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	tu "github.com/hashicorp/nomad/testutil"
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

// Ports used by tests
var (
	docker_reserved = 2000 + int(rand.Int31n(10000))
	docker_dynamic  = 2000 + int(rand.Int31n(10000))
)

// Returns a task with a reserved and dynamic port. The ports are returned
// respectively.
func dockerTask() (*structs.Task, int, int) {
	docker_reserved += 1
	docker_dynamic += 1
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
				&structs.NetworkResource{
					IP:            "127.0.0.1",
					ReservedPorts: []structs.Port{{Label: "main", Value: docker_reserved}},
					DynamicPorts:  []structs.Port{{Label: "REDIS", Value: docker_dynamic}},
				},
			},
		},
	}, docker_reserved, docker_dynamic
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
func dockerSetup(t *testing.T, task *structs.Task) (*docker.Client, DriverHandle, func()) {
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

func dockerSetupWithClient(t *testing.T, task *structs.Task, client *docker.Client) (*docker.Client, DriverHandle, func()) {
	tctx := testDockerDriverContexts(t, task)
	//tctx.DriverCtx.config.Options = map[string]string{"docker.cleanup.image": "false"}
	driver := NewDockerDriver(tctx.DriverCtx)
	copyImage(t, tctx.ExecCtx.TaskDir, "busybox.tar")

	presp, err := driver.Prestart(tctx.ExecCtx, task)
	if err != nil {
		driver.Cleanup(tctx.ExecCtx, presp.CreatedResources)
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

	return client, sresp.Handle, cleanup
}

func newTestDockerClient(t *testing.T) *docker.Client {
	if !testutil.DockerIsConnected(t) {
		t.SkipNow()
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
	apply, err := d.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if apply != testutil.DockerIsConnected(t) {
		t.Fatalf("Fingerprinter should detect when docker is available")
	}
	if node.Attributes["driver.docker"] != "1" {
		t.Log("Docker daemon not available. The remainder of the docker tests will be skipped.")
	}
	t.Logf("Found docker version %s", node.Attributes["driver.docker.version"])
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

	// This seems fragile, so we might need to reconsider this test if it
	// proves flaky
	expectedAddr, err := sockaddr.GetInterfaceIP("docker0")
	if err != nil {
		t.Fatalf("unable to get ip for docker0: %v", err)
	}
	if expectedAddr == "" {
		t.Fatalf("unable to get ip for docker bridge")
	}

	conf := testConfig()
	conf.Node = mock.Node()
	dd := NewDockerDriver(NewDriverContext("", "", conf, conf.Node, testLogger(), nil))
	ok, err := dd.Fingerprint(conf, conf.Node)
	if err != nil {
		t.Fatalf("error fingerprinting docker: %v", err)
	}
	if !ok {
		t.Fatalf("expected Docker to be enabled but false was returned")
	}

	if found := conf.Node.Attributes["driver.docker.bridge_ip"]; found != expectedAddr {
		t.Fatalf("expected bridge ip %q but found: %q", expectedAddr, found)
	}
	t.Logf("docker bridge ip: %q", conf.Node.Attributes["driver.docker.bridge_ip"])
}

func TestDockerDriver_StartOpen_Wait(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.SkipNow()
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

func TestDockerDriver_Start_LoadImage(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.SkipNow()
	}
	task := &structs.Task{
		Name:   "busybox-demo",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "busybox",
			"load":    "busybox.tar",
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
	outputFile := filepath.Join(ctx.ExecCtx.TaskDir.LogDir, "busybox-demo.stdout.0")
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
		t.SkipNow()
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
		t.SkipNow()
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

func TestDockerDriver_StartN(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.SkipNow()
	}

	task1, _, _ := dockerTask()
	task2, _, _ := dockerTask()
	task3, _, _ := dockerTask()
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
		t.SkipNow()
	}

	task1, _, _ := dockerTask()
	task1.Config["image"] = "busybox"
	task1.Config["load"] = "busybox.tar"

	task2, _, _ := dockerTask()
	task2.Config["image"] = "busybox:musl"
	task2.Config["load"] = "busybox_musl.tar"

	task3, _, _ := dockerTask()
	task3.Config["image"] = "busybox:glibc"
	task3.Config["load"] = "busybox_glibc.tar"

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
}

func TestDockerDriver_NetworkMode_Host(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
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

	waitForExist(t, client, handle.(*DockerHandle))

	container, err := client.InspectContainer(handle.(*DockerHandle).ContainerID())
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

	waitForExist(t, client, handle.(*DockerHandle))

	_, err = client.InspectContainer(handle.(*DockerHandle).ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestDockerDriver_Labels(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	task, _, _ := dockerTask()
	task.Config["labels"] = []map[string]string{
		map[string]string{
			"label1": "value1",
			"label2": "value2",
		},
	}

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle.(*DockerHandle))

	container, err := client.InspectContainer(handle.(*DockerHandle).ContainerID())
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
	task, _, _ := dockerTask()
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
	task, _, _ := dockerTask()
	task.Config["force_pull"] = "true"

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle.(*DockerHandle))

	_, err := client.InspectContainer(handle.(*DockerHandle).ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestDockerDriver_SecurityOpt(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	task, _, _ := dockerTask()
	task.Config["security_opt"] = []string{"seccomp=unconfined"}

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle.(*DockerHandle))

	container, err := client.InspectContainer(handle.(*DockerHandle).ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(task.Config["security_opt"], container.HostConfig.SecurityOpt) {
		t.Errorf("Security Opts don't match.\nExpected:\n%s\nGot:\n%s\n", task.Config["security_opt"], container.HostConfig.SecurityOpt)
	}
}

func TestDockerDriver_DNS(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	task, _, _ := dockerTask()
	task.Config["dns_servers"] = []string{"8.8.8.8", "8.8.4.4"}
	task.Config["dns_search_domains"] = []string{"example.com", "example.org", "example.net"}
	task.Config["dns_options"] = []string{"ndots:1"}

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle.(*DockerHandle))

	container, err := client.InspectContainer(handle.(*DockerHandle).ContainerID())
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
	task, _, _ := dockerTask()
	task.Config["mac_address"] = "00:16:3e:00:00:00"

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle.(*DockerHandle))

	container, err := client.InspectContainer(handle.(*DockerHandle).ContainerID())
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
	task, _, _ := dockerTask()
	task.Config["work_dir"] = "/some/path"

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	container, err := client.InspectContainer(handle.(*DockerHandle).ContainerID())
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
	task, res, dyn := dockerTask()

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle.(*DockerHandle))

	container, err := client.InspectContainer(handle.(*DockerHandle).ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that the correct ports are EXPOSED
	expectedExposedPorts := map[docker.Port]struct{}{
		docker.Port(fmt.Sprintf("%d/tcp", res)): struct{}{},
		docker.Port(fmt.Sprintf("%d/udp", res)): struct{}{},
		docker.Port(fmt.Sprintf("%d/tcp", dyn)): struct{}{},
		docker.Port(fmt.Sprintf("%d/udp", dyn)): struct{}{},
	}

	if !reflect.DeepEqual(container.Config.ExposedPorts, expectedExposedPorts) {
		t.Errorf("Exposed ports don't match.\nExpected:\n%s\nGot:\n%s\n", expectedExposedPorts, container.Config.ExposedPorts)
	}

	// Verify that the correct ports are FORWARDED
	expectedPortBindings := map[docker.Port][]docker.PortBinding{
		docker.Port(fmt.Sprintf("%d/tcp", res)): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", res)}},
		docker.Port(fmt.Sprintf("%d/udp", res)): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", res)}},
		docker.Port(fmt.Sprintf("%d/tcp", dyn)): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", dyn)}},
		docker.Port(fmt.Sprintf("%d/udp", dyn)): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", dyn)}},
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
	task, res, dyn := dockerTask()
	task.Config["port_map"] = []map[string]string{
		map[string]string{
			"main":  "8080",
			"REDIS": "6379",
		},
	}

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	waitForExist(t, client, handle.(*DockerHandle))

	container, err := client.InspectContainer(handle.(*DockerHandle).ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that the correct ports are EXPOSED
	expectedExposedPorts := map[docker.Port]struct{}{
		docker.Port("8080/tcp"): struct{}{},
		docker.Port("8080/udp"): struct{}{},
		docker.Port("6379/tcp"): struct{}{},
		docker.Port("6379/udp"): struct{}{},
	}

	if !reflect.DeepEqual(container.Config.ExposedPorts, expectedExposedPorts) {
		t.Errorf("Exposed ports don't match.\nExpected:\n%s\nGot:\n%s\n", expectedExposedPorts, container.Config.ExposedPorts)
	}

	// Verify that the correct ports are FORWARDED
	expectedPortBindings := map[docker.Port][]docker.PortBinding{
		docker.Port("8080/tcp"): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", res)}},
		docker.Port("8080/udp"): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", res)}},
		docker.Port("6379/tcp"): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", dyn)}},
		docker.Port("6379/udp"): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", dyn)}},
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

	if !testutil.DockerIsConnected(t) {
		t.SkipNow()
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
		_, err := client.InspectContainer(handle.(*DockerHandle).containerID)
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

	waitForExist(t, client, handle.(*DockerHandle))

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
		t.SkipNow()
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
	allocDir := allocdir.NewAllocDir(testLogger(), filepath.Join(cfg.AllocDir, structs.GenerateUUID()))
	if err := allocDir.Build(); err != nil {
		t.Fatalf("failed to build alloc dir: %v", err)
	}
	taskDir := allocDir.NewTaskDir(task.Name)
	if err := taskDir.Build(false, nil, cstructs.FSIsolationImage); err != nil {
		allocDir.Destroy()
		t.Fatalf("failed to build task dir: %v", err)
	}

	alloc := mock.Alloc()
	envBuilder := env.NewBuilder(cfg.Node, alloc, task, cfg.Region)
	execCtx := NewExecContext(taskDir, envBuilder.Build())
	cleanup := func() {
		allocDir.Destroy()
		if filepath.IsAbs(hostpath) {
			os.RemoveAll(hostpath)
		}
	}

	logger := testLogger()
	emitter := func(m string, args ...interface{}) {
		logger.Printf("[EVENT] "+m, args...)
	}
	driverCtx := NewDriverContext(task.Name, alloc.ID, cfg, cfg.Node, testLogger(), emitter)
	driver := NewDockerDriver(driverCtx)
	copyImage(t, taskDir, "busybox.tar")

	return task, driver, execCtx, hostfile, cleanup
}

func TestDockerDriver_VolumesDisabled(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	cfg := testConfig()
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
	cfg := testConfig()

	tmpvol, err := ioutil.TempDir("", "nomadtest_docker_volumesenabled")
	if err != nil {
		t.Fatalf("error creating temporary dir: %v", err)
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

// TestDockerDriver_Cleanup ensures Cleanup removes only downloaded images.
func TestDockerDriver_Cleanup(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.SkipNow()
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
