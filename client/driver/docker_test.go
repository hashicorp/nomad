package driver

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/environment"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

func testDockerDriverContext(task string) *DriverContext {
	cfg := testConfig()
	cfg.DevMode = true
	return NewDriverContext(task, cfg, cfg.Node, testLogger())
}

// dockerIsConnected checks to see if a docker daemon is available (local or remote)
func dockerIsConnected(t *testing.T) bool {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		return false
	}

	// Creating a client doesn't actually connect, so make sure we do something
	// like call Version() on it.
	env, err := client.Version()
	if err != nil {
		t.Logf("Failed to connect to docker daemon: %s", err)
		return false
	}

	t.Logf("Successfully connected to docker daemon running version %s", env.Get("Version"))
	return true
}

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

func dockerTask() *structs.Task {
	return &structs.Task{
		Name: "redis-demo",
		Config: map[string]interface{}{
			"image": "redis",
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
			Networks: []*structs.NetworkResource{
				&structs.NetworkResource{
					IP:            "127.0.0.1",
					ReservedPorts: []structs.Port{{"main", 11110}},
					DynamicPorts:  []structs.Port{{"REDIS", 43330}},
				},
			},
		},
	}
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
	if !dockerIsConnected(t) {
		t.SkipNow()
	}

	client, err := docker.NewClientFromEnv()
	if err != nil {
		t.Fatalf("Failed to initialize client: %s\nStack\n%s", err, debug.Stack())
	}

	driverCtx := testDockerDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	driver := NewDockerDriver(driverCtx)

	handle, err := driver.Start(ctx, task)
	if err != nil {
		ctx.AllocDir.Destroy()
		t.Fatalf("Failed to start driver: %s\nStack\n%s", err, debug.Stack())
	}
	if handle == nil {
		ctx.AllocDir.Destroy()
		t.Fatalf("handle is nil\nStack\n%s", debug.Stack())
	}

	cleanup := func() {
		handle.Kill()
		ctx.AllocDir.Destroy()
	}

	return client, handle, cleanup
}

func TestDockerDriver_Handle(t *testing.T) {
	h := &DockerHandle{
		imageID:     "imageid",
		containerID: "containerid",
		doneCh:      make(chan struct{}),
		waitCh:      make(chan *cstructs.WaitResult, 1),
	}

	actual := h.ID()
	expected := `DOCKER:{"ImageID":"imageid","ContainerID":"containerid"}`
	if actual != expected {
		t.Errorf("Expected `%s`, found `%s`", expected, actual)
	}
}

// This test should always pass, even if docker daemon is not available
func TestDockerDriver_Fingerprint(t *testing.T) {
	d := NewDockerDriver(testDockerDriverContext(""))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	apply, err := d.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if apply != dockerIsConnected(t) {
		t.Fatalf("Fingerprinter should detect when docker is available")
	}
	if node.Attributes["driver.docker"] != "1" {
		t.Log("Docker daemon not available. The remainder of the docker tests will be skipped.")
	}
	t.Logf("Found docker version %s", node.Attributes["driver.docker.version"])
}

func TestDockerDriver_StartOpen_Wait(t *testing.T) {
	if !dockerIsConnected(t) {
		t.SkipNow()
	}

	task := &structs.Task{
		Name: "redis-demo",
		Config: map[string]interface{}{
			"image": "redis",
		},
		Resources: basicResources,
	}

	driverCtx := testDockerDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()
	d := NewDockerDriver(driverCtx)

	handle, err := d.Start(ctx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}
	defer handle.Kill()

	// Attempt to open
	handle2, err := d.Open(ctx, handle.ID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle2 == nil {
		t.Fatalf("missing handle")
	}
}

func TestDockerDriver_Start_Wait(t *testing.T) {
	task := &structs.Task{
		Name: "redis-demo",
		Config: map[string]interface{}{
			"image":   "redis",
			"command": "redis-server",
			"args":    []string{"-v"},
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
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
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestDockerDriver_Start_Wait_AllocDir(t *testing.T) {
	// This test requires that the alloc dir be mounted into docker as a volume.
	// Because this cannot happen when docker is run remotely, e.g. when running
	// docker in a VM, we skip this when we detect Docker is being run remotely.
	if !dockerIsConnected(t) || dockerIsRemote(t) {
		t.SkipNow()
	}

	exp := []byte{'w', 'i', 'n'}
	file := "output.txt"
	task := &structs.Task{
		Name: "redis-demo",
		Config: map[string]interface{}{
			"image":   "redis",
			"command": "/bin/bash",
			"args": []string{
				"-c",
				fmt.Sprintf(`sleep 1; echo -n %s > $%s/%s`,
					string(exp), environment.AllocDir, file),
			},
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
	}

	driverCtx := testDockerDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()
	d := NewDockerDriver(driverCtx)

	handle, err := d.Start(ctx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}
	defer handle.Kill()

	select {
	case res := <-handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(5 * time.Second):
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
	task := &structs.Task{
		Name: "redis-demo",
		Config: map[string]interface{}{
			"image":   "redis",
			"command": "/bin/sleep",
			"args":    []string{"10"},
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
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestDocker_StartN(t *testing.T) {
	if !dockerIsConnected(t) {
		t.SkipNow()
	}

	task1 := dockerTask()
	task1.Resources.Networks[0].ReservedPorts[0] = structs.Port{Label: "main", Value: 11110}
	task1.Resources.Networks[0].DynamicPorts[0] = structs.Port{Label: "REDIS", Value: 43331}

	task2 := dockerTask()
	task2.Resources.Networks[0].ReservedPorts[0] = structs.Port{Label: "main", Value: 22222}
	task2.Resources.Networks[0].DynamicPorts[0] = structs.Port{Label: "REDIS", Value: 43332}

	task3 := dockerTask()
	task3.Resources.Networks[0].ReservedPorts[0] = structs.Port{Label: "main", Value: 33333}
	task3.Resources.Networks[0].DynamicPorts[0] = structs.Port{Label: "REDIS", Value: 43333}

	taskList := []*structs.Task{task1, task2, task3}

	handles := make([]DriverHandle, len(taskList))

	t.Logf("==> Starting %d tasks", len(taskList))

	// Let's spin up a bunch of things
	var err error
	for idx, task := range taskList {
		driverCtx := testDockerDriverContext(task.Name)
		ctx := testDriverExecContext(task, driverCtx)
		defer ctx.AllocDir.Destroy()
		d := NewDockerDriver(driverCtx)

		handles[idx], err = d.Start(ctx, task)
		if err != nil {
			t.Errorf("Failed starting task #%d: %s", idx+1, err)
		}
	}

	t.Log("==> All tasks are started. Terminating...")

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

	t.Log("==> Test complete!")
}

func TestDocker_StartNVersions(t *testing.T) {
	if !dockerIsConnected(t) {
		t.SkipNow()
	}

	task1 := dockerTask()
	task1.Config["image"] = "redis"
	task1.Resources.Networks[0].ReservedPorts[0] = structs.Port{Label: "main", Value: 11110}
	task1.Resources.Networks[0].DynamicPorts[0] = structs.Port{Label: "REDIS", Value: 43331}

	task2 := dockerTask()
	task2.Config["image"] = "redis:latest"
	task2.Resources.Networks[0].ReservedPorts[0] = structs.Port{Label: "main", Value: 22222}
	task2.Resources.Networks[0].DynamicPorts[0] = structs.Port{Label: "REDIS", Value: 43332}

	task3 := dockerTask()
	task3.Config["image"] = "redis:3.0"
	task3.Resources.Networks[0].ReservedPorts[0] = structs.Port{Label: "main", Value: 33333}
	task3.Resources.Networks[0].DynamicPorts[0] = structs.Port{Label: "REDIS", Value: 43333}

	taskList := []*structs.Task{task1, task2, task3}

	handles := make([]DriverHandle, len(taskList))

	t.Logf("==> Starting %d tasks", len(taskList))

	// Let's spin up a bunch of things
	var err error
	for idx, task := range taskList {
		driverCtx := testDockerDriverContext(task.Name)
		ctx := testDriverExecContext(task, driverCtx)
		defer ctx.AllocDir.Destroy()
		d := NewDockerDriver(driverCtx)

		handles[idx], err = d.Start(ctx, task)
		if err != nil {
			t.Errorf("Failed starting task #%d: %s", idx+1, err)
		}
	}

	t.Log("==> All tasks are started. Terminating...")

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

	t.Log("==> Test complete!")
}

func TestDockerHostNet(t *testing.T) {
	expected := "host"

	task := &structs.Task{
		Name: "redis-demo",
		Config: map[string]interface{}{
			"image":        "redis",
			"network_mode": expected,
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
	}

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	container, err := client.InspectContainer(handle.(*DockerHandle).ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	actual := container.HostConfig.NetworkMode
	if actual != expected {
		t.Errorf("DNS Network mode doesn't match.\nExpected:\n%s\nGot:\n%s\n", expected, actual)
	}
}

func TestDockerLabels(t *testing.T) {
	task := dockerTask()
	task.Config["labels"] = []map[string]string{
		map[string]string{
			"label1": "value1",
			"label2": "value2",
		},
	}

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

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

func TestDockerDNS(t *testing.T) {
	task := dockerTask()
	task.Config["dns_servers"] = []string{"8.8.8.8", "8.8.4.4"}
	task.Config["dns_search_domains"] = []string{"example.com", "example.org", "example.net"}

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	container, err := client.InspectContainer(handle.(*DockerHandle).ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(task.Config["dns_servers"], container.HostConfig.DNS) {
		t.Errorf("DNS Servers don't match.\nExpected:\n%s\nGot:\n%s\n", task.Config["dns_servers"], container.HostConfig.DNS)
	}

	if !reflect.DeepEqual(task.Config["dns_search_domains"], container.HostConfig.DNSSearch) {
		t.Errorf("DNS Servers don't match.\nExpected:\n%s\nGot:\n%s\n", task.Config["dns_search_domains"], container.HostConfig.DNSSearch)
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

func TestDockerPortsNoMap(t *testing.T) {
	task := dockerTask()

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

	container, err := client.InspectContainer(handle.(*DockerHandle).ContainerID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that the correct ports are EXPOSED
	expectedExposedPorts := map[docker.Port]struct{}{
		docker.Port("11110/tcp"): struct{}{},
		docker.Port("11110/udp"): struct{}{},
		docker.Port("43330/tcp"): struct{}{},
		docker.Port("43330/udp"): struct{}{},
		// This one comes from the redis container
		docker.Port("6379/tcp"): struct{}{},
	}

	if !reflect.DeepEqual(container.Config.ExposedPorts, expectedExposedPorts) {
		t.Errorf("Exposed ports don't match.\nExpected:\n%s\nGot:\n%s\n", expectedExposedPorts, container.Config.ExposedPorts)
	}

	// Verify that the correct ports are FORWARDED
	expectedPortBindings := map[docker.Port][]docker.PortBinding{
		docker.Port("11110/tcp"): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: "11110"}},
		docker.Port("11110/udp"): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: "11110"}},
		docker.Port("43330/tcp"): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: "43330"}},
		docker.Port("43330/udp"): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: "43330"}},
	}

	if !reflect.DeepEqual(container.HostConfig.PortBindings, expectedPortBindings) {
		t.Errorf("Forwarded ports don't match.\nExpected:\n%s\nGot:\n%s\n", expectedPortBindings, container.HostConfig.PortBindings)
	}

	expectedEnvironment := map[string]string{
		"NOMAD_PORT_main":  "11110",
		"NOMAD_PORT_REDIS": "43330",
	}

	for key, val := range expectedEnvironment {
		search := fmt.Sprintf("%s=%s", key, val)
		if !inSlice(search, container.Config.Env) {
			t.Errorf("Expected to find %s in container environment: %+v", search, container.Config.Env)
		}
	}
}

func TestDockerPortsMapping(t *testing.T) {
	task := dockerTask()
	task.Config["port_map"] = []map[string]string{
		map[string]string{
			"main":  "8080",
			"REDIS": "6379",
		},
	}

	client, handle, cleanup := dockerSetup(t, task)
	defer cleanup()

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
		docker.Port("8080/tcp"): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: "11110"}},
		docker.Port("8080/udp"): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: "11110"}},
		docker.Port("6379/tcp"): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: "43330"}},
		docker.Port("6379/udp"): []docker.PortBinding{docker.PortBinding{HostIP: "127.0.0.1", HostPort: "43330"}},
	}

	if !reflect.DeepEqual(container.HostConfig.PortBindings, expectedPortBindings) {
		t.Errorf("Forwarded ports don't match.\nExpected:\n%s\nGot:\n%s\n", expectedPortBindings, container.HostConfig.PortBindings)
	}

	expectedEnvironment := map[string]string{
		"NOMAD_PORT_main":  "8080",
		"NOMAD_PORT_REDIS": "6379",
	}

	for key, val := range expectedEnvironment {
		search := fmt.Sprintf("%s=%s", key, val)
		if !inSlice(search, container.Config.Env) {
			t.Errorf("Expected to find %s in container environment: %+v", search, container.Config.Env)
		}
	}
}
