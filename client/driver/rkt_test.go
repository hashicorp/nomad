package driver

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"

	ctestutils "github.com/hashicorp/nomad/client/testutil"
)

func TestRktVersionRegex(t *testing.T) {
	if os.Getenv("NOMAD_TEST_RKT") == "" {
		t.Skip("skipping rkt tests")
	}

	input_rkt := "rkt version 0.8.1"
	input_appc := "appc version 1.2.0"
	expected_rkt := "0.8.1"
	expected_appc := "1.2.0"
	rktMatches := reRktVersion.FindStringSubmatch(input_rkt)
	appcMatches := reAppcVersion.FindStringSubmatch(input_appc)
	if rktMatches[1] != expected_rkt {
		fmt.Printf("Test failed; got %q; want %q\n", rktMatches[1], expected_rkt)
	}
	if appcMatches[1] != expected_appc {
		fmt.Printf("Test failed; got %q; want %q\n", appcMatches[1], expected_appc)
	}
}

// The fingerprinter test should always pass, even if rkt is not installed.
func TestRktDriver_Fingerprint(t *testing.T) {
	if os.Getenv("NOMAD_TEST_RKT") == "" {
		t.Skip("skipping rkt tests")
	}

	ctestutils.RktCompatible(t)
	ctx := testDriverContexts(t, &structs.Task{Name: "foo", Driver: "rkt"})
	d := NewRktDriver(ctx.DriverCtx)
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	apply, err := d.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !apply {
		t.Fatalf("should apply")
	}
	if node.Attributes["driver.rkt"] != "1" {
		t.Fatalf("Missing Rkt driver")
	}
	if node.Attributes["driver.rkt.version"] == "" {
		t.Fatalf("Missing Rkt driver version")
	}
	if node.Attributes["driver.rkt.appc.version"] == "" {
		t.Fatalf("Missing appc version for the Rkt driver")
	}
}

func TestRktDriver_Start_DNS(t *testing.T) {
	if os.Getenv("NOMAD_TEST_RKT") == "" {
		t.Skip("skipping rkt tests")
	}

	ctestutils.RktCompatible(t)
	// TODO: use test server to load from a fixture
	task := &structs.Task{
		Name:   "etcd",
		Driver: "rkt",
		Config: map[string]interface{}{
			"trust_prefix":       "coreos.com/etcd",
			"image":              "coreos.com/etcd:v2.0.4",
			"command":            "/etcd",
			"dns_servers":        []string{"8.8.8.8", "8.8.4.4"},
			"dns_search_domains": []string{"example.com", "example.org", "example.net"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			MemoryMB: 128,
			CPU:      100,
		},
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRktDriver(ctx.DriverCtx)

	if err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	handle, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}
	defer handle.Kill()

	// Attempt to open
	handle2, err := d.Open(ctx.ExecCtx, handle.ID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle2 == nil {
		t.Fatalf("missing handle")
	}
}

func TestRktDriver_Start_Wait(t *testing.T) {
	if os.Getenv("NOMAD_TEST_RKT") == "" {
		t.Skip("skipping rkt tests")
	}

	ctestutils.RktCompatible(t)
	task := &structs.Task{
		Name:   "etcd",
		Driver: "rkt",
		Config: map[string]interface{}{
			"trust_prefix": "coreos.com/etcd",
			"image":        "coreos.com/etcd:v2.0.4",
			"command":      "/etcd",
			"args":         []string{"--version"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			MemoryMB: 128,
			CPU:      100,
		},
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRktDriver(ctx.DriverCtx)

	if err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	handle, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}
	defer handle.Kill()

	// Update should be a no-op
	err = handle.Update(task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Signal should be an error
	if err = handle.Signal(syscall.SIGTERM); err == nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case res := <-handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}
}

func TestRktDriver_Start_Wait_Skip_Trust(t *testing.T) {
	if os.Getenv("NOMAD_TEST_RKT") == "" {
		t.Skip("skipping rkt tests")
	}

	ctestutils.RktCompatible(t)
	task := &structs.Task{
		Name:   "etcd",
		Driver: "rkt",
		Config: map[string]interface{}{
			"image":   "coreos.com/etcd:v2.0.4",
			"command": "/etcd",
			"args":    []string{"--version"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			MemoryMB: 128,
			CPU:      100,
		},
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRktDriver(ctx.DriverCtx)

	if err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	handle, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}
	defer handle.Kill()

	// Update should be a no-op
	err = handle.Update(task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case res := <-handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}
}

func TestRktDriver_Start_Wait_AllocDir(t *testing.T) {
	if os.Getenv("NOMAD_TEST_RKT") == "" {
		t.Skip("skipping rkt tests")
	}

	ctestutils.RktCompatible(t)

	exp := []byte{'w', 'i', 'n'}
	file := "output.txt"
	tmpvol, err := ioutil.TempDir("", "nomadtest_rktdriver_volumes")
	if err != nil {
		t.Fatalf("error creating temporary dir: %v", err)
	}
	defer os.RemoveAll(tmpvol)
	hostpath := filepath.Join(tmpvol, file)

	task := &structs.Task{
		Name:   "alpine",
		Driver: "rkt",
		Config: map[string]interface{}{
			"image":   "docker://alpine",
			"command": "/bin/sh",
			"args": []string{
				"-c",
				fmt.Sprintf(`echo -n %s > foo/%s`, string(exp), file),
			},
			"net":     []string{"none"},
			"volumes": []string{fmt.Sprintf("%s:/foo", tmpvol)},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			MemoryMB: 128,
			CPU:      100,
		},
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRktDriver(ctx.DriverCtx)

	if err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	handle, err := d.Start(ctx.ExecCtx, task)
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
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	// Check that data was written to the shared alloc directory.
	act, err := ioutil.ReadFile(hostpath)
	if err != nil {
		t.Fatalf("Couldn't read expected output: %v", err)
	}

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("Command output is %v; expected %v", act, exp)
	}
}

func TestRktDriverUser(t *testing.T) {
	if os.Getenv("NOMAD_TEST_RKT") == "" {
		t.Skip("skipping rkt tests")
	}

	ctestutils.RktCompatible(t)
	task := &structs.Task{
		Name:   "etcd",
		Driver: "rkt",
		User:   "alice",
		Config: map[string]interface{}{
			"trust_prefix": "coreos.com/etcd",
			"image":        "coreos.com/etcd:v2.0.4",
			"command":      "/etcd",
			"args":         []string{"--version"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			MemoryMB: 128,
			CPU:      100,
		},
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRktDriver(ctx.DriverCtx)

	if err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	handle, err := d.Start(ctx.ExecCtx, task)
	if err == nil {
		handle.Kill()
		t.Fatalf("Should've failed")
	}
	msg := "unknown user alice"
	if !strings.Contains(err.Error(), msg) {
		t.Fatalf("Expecting '%v' in '%v'", msg, err)
	}
}

func TestRktTrustPrefix(t *testing.T) {
	if os.Getenv("NOMAD_TEST_RKT") == "" {
		t.Skip("skipping rkt tests")
	}
	ctestutils.RktCompatible(t)
	task := &structs.Task{
		Name:   "etcd",
		Driver: "rkt",
		Config: map[string]interface{}{
			"trust_prefix": "example.com/invalid",
			"image":        "coreos.com/etcd:v2.0.4",
			"command":      "/etcd",
			"args":         []string{"--version"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			MemoryMB: 128,
			CPU:      100,
		},
	}
	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRktDriver(ctx.DriverCtx)

	if err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	handle, err := d.Start(ctx.ExecCtx, task)
	if err == nil {
		handle.Kill()
		t.Fatalf("Should've failed")
	}
	msg := "Error running rkt trust"
	if !strings.Contains(err.Error(), msg) {
		t.Fatalf("Expecting '%v' in '%v'", msg, err)
	}
}

func TestRktTaskValidate(t *testing.T) {
	ctestutils.RktCompatible(t)
	task := &structs.Task{
		Name:   "etcd",
		Driver: "rkt",
		Config: map[string]interface{}{
			"trust_prefix":       "coreos.com/etcd",
			"image":              "coreos.com/etcd:v2.0.4",
			"command":            "/etcd",
			"args":               []string{"--version"},
			"dns_servers":        []string{"8.8.8.8", "8.8.4.4"},
			"dns_search_domains": []string{"example.com", "example.org", "example.net"},
		},
		Resources: basicResources,
	}
	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRktDriver(ctx.DriverCtx)

	if err := d.Validate(task.Config); err != nil {
		t.Fatalf("Validation error in TaskConfig : '%v'", err)
	}
}

// TODO: Port Mapping test should be ran with proper ACI image and test the port access.
func TestRktDriver_PortsMapping(t *testing.T) {
	if os.Getenv("NOMAD_TEST_RKT") == "" {
		t.Skip("skipping rkt tests")
	}

	ctestutils.RktCompatible(t)
	task := &structs.Task{
		Name:   "etcd",
		Driver: "rkt",
		Config: map[string]interface{}{
			"image": "docker://redis:latest",
			"args":  []string{"--version"},
			"port_map": []map[string]string{
				map[string]string{
					"main": "6379-tcp",
				},
			},
			"debug": "true",
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
					ReservedPorts: []structs.Port{{"main", 8080}},
				},
			},
		},
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRktDriver(ctx.DriverCtx)

	if err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	handle, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}

	failCh := make(chan error, 1)
	go func() {
		time.Sleep(1 * time.Second)
		if err := handle.Kill(); err != nil {
			failCh <- err
		}
	}()

	select {
	case err := <-failCh:
		t.Fatalf("failed to kill handle: %v", err)
	case <-handle.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}
}
