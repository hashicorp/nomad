// +build linux

package driver

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"

	ctestutil "github.com/hashicorp/nomad/client/testutil"
)

func TestRktVersionRegex(t *testing.T) {
	ctestutil.RktCompatible(t)
	t.Parallel()

	inputRkt := "rkt version 0.8.1"
	inputAppc := "appc version 1.2.0"
	expectedRkt := "0.8.1"
	expectedAppc := "1.2.0"
	rktMatches := reRktVersion.FindStringSubmatch(inputRkt)
	appcMatches := reAppcVersion.FindStringSubmatch(inputAppc)
	if rktMatches[1] != expectedRkt {
		fmt.Printf("Test failed; got %q; want %q\n", rktMatches[1], expectedRkt)
	}
	if appcMatches[1] != expectedAppc {
		fmt.Printf("Test failed; got %q; want %q\n", appcMatches[1], expectedAppc)
	}
}

// The fingerprinter test should always pass, even if rkt is not installed.
func TestRktDriver_Fingerprint(t *testing.T) {
	ctestutil.RktCompatible(t)
	t.Parallel()

	ctx := testDriverContexts(t, &structs.Task{Name: "foo", Driver: "rkt"})
	d := NewRktDriver(ctx.DriverCtx)
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	request := &cstructs.FingerprintRequest{Config: &config.Config{}, Node: node}
	var response cstructs.FingerprintResponse
	err := d.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	attributes := response.Attributes
	if attributes == nil {
		t.Fatalf("expected attributes to not equal nil")
	}
	if attributes["driver.rkt"] != "1" {
		t.Fatalf("Missing Rkt driver")
	}
	if attributes["driver.rkt.version"] == "" {
		t.Fatalf("Missing Rkt driver version")
	}
	if attributes["driver.rkt.appc.version"] == "" {
		t.Fatalf("Missing appc version for the Rkt driver")
	}
}

func TestRktDriver_Start_DNS(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

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

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer resp.Handle.Kill()

	// Attempt to open
	handle2, err := d.Open(ctx.ExecCtx, resp.Handle.ID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle2 == nil {
		t.Fatalf("missing handle")
	}
	handle2.Kill()
}

func TestRktDriver_Start_Wait(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

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

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	handle := resp.Handle.(*rktHandle)
	defer handle.Kill()

	// Update should be a no-op
	if err := handle.Update(task); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Signal should be an error
	if err := resp.Handle.Signal(syscall.SIGTERM); err == nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case res := <-resp.Handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}

	// Make sure pod was removed #3561
	var stderr bytes.Buffer
	cmd := exec.Command(rktCmd, "status", handle.uuid)
	cmd.Stdout = ioutil.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatalf("expected error running 'rkt status %s' on removed container", handle.uuid)
	}
	if out := stderr.String(); !strings.Contains(out, "no matches found") {
		t.Fatalf("expected 'no matches found' but received: %s", out)
	}
}

func TestRktDriver_Start_Wait_Skip_Trust(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

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

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer resp.Handle.Kill()

	// Update should be a no-op
	err = resp.Handle.Update(task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case res := <-resp.Handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}
}

func TestRktDriver_Start_Wait_AllocDir(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	exp := []byte{'w', 'i', 'n'}
	file := "output.txt"
	tmpvol, err := ioutil.TempDir("", "nomadtest_rktdriver_volumes")
	if err != nil {
		t.Fatalf("error creating temporary dir: %v", err)
	}
	defer os.RemoveAll(tmpvol)
	hostpath := filepath.Join(tmpvol, file)

	task := &structs.Task{
		Name:   "rkttest_alpine",
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

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
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

// TestRktDriver_UserGroup asserts tasks may override the user and group of the
// rkt image.
func TestRktDriver_UserGroup(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}
	require := assert.New(t)

	task := &structs.Task{
		Name:   "etcd",
		Driver: "rkt",
		User:   "nobody",
		Config: map[string]interface{}{
			"image": "docker://redis:3.2",
			"group": "nogroup",
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

	tctx := testDriverContexts(t, task)
	defer tctx.AllocDir.Destroy()
	d := NewRktDriver(tctx.DriverCtx)

	_, err := d.Prestart(tctx.ExecCtx, task)
	require.Nil(err)
	resp, err := d.Start(tctx.ExecCtx, task)
	require.Nil(err)
	defer resp.Handle.Kill()

	timeout := time.Duration(testutil.TestMultiplier()*15) * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// WaitUntil we can determine the user/group redis is running as
	expected := []byte("redis-server *:6379         nobody   nogroup\n")
	testutil.WaitForResult(func() (bool, error) {
		raw, code, err := resp.Handle.Exec(ctx, "/bin/bash", []string{"-c", "ps -eo args,user,group | grep ^redis"})
		if err != nil {
			return false, err
		}
		if code != 0 {
			return false, fmt.Errorf("unexpected exit code: %d", code)
		}
		return bytes.Equal(expected, raw), fmt.Errorf("expected %q but found %q", expected, raw)
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	require.Nil(resp.Handle.Kill())
}

func TestRktTrustPrefix(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

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

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err == nil {
		resp.Handle.Kill()
		t.Fatalf("Should've failed")
	}
	msg := "Error running rkt trust"
	if !strings.Contains(err.Error(), msg) {
		t.Fatalf("Expecting '%v' in '%v'", msg, err)
	}
}

func TestRktTaskValidate(t *testing.T) {
	ctestutil.RktCompatible(t)
	t.Parallel()

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

func TestRktDriver_PortMapping(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	task := &structs.Task{
		Name:   "etcd",
		Driver: "rkt",
		Config: map[string]interface{}{
			"image": "docker://redis:3.2",
			"port_map": []map[string]string{
				{
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
				{
					IP:            "127.0.0.1",
					ReservedPorts: []structs.Port{{Label: "main", Value: 8080}},
				},
			},
		},
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRktDriver(ctx.DriverCtx)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer resp.Handle.Kill()
	if resp.Network == nil {
		t.Fatalf("Expected driver to set a DriverNetwork, but it did not!")
	}

	failCh := make(chan error, 1)
	go func() {
		time.Sleep(1 * time.Second)
		if err := resp.Handle.Kill(); err != nil {
			failCh <- err
		}
	}()

	select {
	case err := <-failCh:
		t.Fatalf("failed to kill handle: %v", err)
	case <-resp.Handle.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}
}

// TestRktDriver_PortsMapping_Host asserts that port_map isn't required when
// host networking is used.
func TestRktDriver_PortsMapping_Host(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	task := &structs.Task{
		Name:   "etcd",
		Driver: "rkt",
		Config: map[string]interface{}{
			"image": "docker://redis:latest",
			"net":   []string{"host"},
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
					ReservedPorts: []structs.Port{{Label: "main", Value: 8080}},
				},
			},
		},
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRktDriver(ctx.DriverCtx)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer resp.Handle.Kill()
	if resp.Network != nil {
		t.Fatalf("No network should be returned with --net=host but found: %#v", resp.Network)
	}

	failCh := make(chan error, 1)
	go func() {
		time.Sleep(1 * time.Second)
		if err := resp.Handle.Kill(); err != nil {
			failCh <- err
		}
	}()

	select {
	case err := <-failCh:
		t.Fatalf("failed to kill handle: %v", err)
	case <-resp.Handle.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*15) * time.Second):
		t.Fatalf("timeout")
	}
}

func TestRktDriver_HandlerExec(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	task := &structs.Task{
		Name:   "etcd",
		Driver: "rkt",
		Config: map[string]interface{}{
			"trust_prefix": "coreos.com/etcd",
			"image":        "coreos.com/etcd:v2.0.4",
			"command":      "/etcd",
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

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer resp.Handle.Kill()

	// Exec a command that should work
	testutil.WaitForResult(func() (bool, error) {
		out, code, err := resp.Handle.Exec(context.TODO(), "/etcd", []string{"--version"})
		if err != nil {
			return false, fmt.Errorf("error exec'ing etcd --version: %v", err)
		}
		if code != 0 {
			return false, fmt.Errorf("expected `etcd --version` to succeed but exit code was: %d\n%s", code, string(out))
		}
		if expected := []byte("etcd version "); !bytes.HasPrefix(out, expected) {
			return false, fmt.Errorf("expected output to start with %q but found:\n%q", expected, out)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Exec a command that should fail
	out, code, err := resp.Handle.Exec(context.TODO(), "/etcd", []string{"--kaljdshf"})
	if err != nil {
		t.Fatalf("error exec'ing bad command: %v", err)
	}
	if code == 0 {
		t.Fatalf("expected `stat` to fail but exit code was: %d", code)
	}
	if expected := "flag provided but not defined"; !bytes.Contains(out, []byte(expected)) {
		t.Fatalf("expected output to contain %q but found: %q", expected, out)
	}

	if err := resp.Handle.Kill(); err != nil {
		t.Fatalf("error killing handle: %v", err)
	}
}

func TestRktDriver_Stats(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	task := &structs.Task{
		Name:   "etcd",
		Driver: "rkt",
		Config: map[string]interface{}{
			"trust_prefix": "coreos.com/etcd",
			"image":        "coreos.com/etcd:v2.0.4",
			"command":      "/etcd",
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

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer resp.Handle.Kill()

	testutil.WaitForResult(func() (bool, error) {
		stats, err := resp.Handle.Stats()
		if err != nil {
			return false, err
		}
		if stats == nil || stats.ResourceUsage == nil {
			return false, fmt.Errorf("stats is nil")
		}
		if stats.ResourceUsage.CpuStats.TotalTicks == 0 {
			return false, fmt.Errorf("cpu ticks unset")
		}
		if stats.ResourceUsage.MemoryStats.RSS == 0 {
			return false, fmt.Errorf("rss stats unset")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("error: %v", err)
	})

}

func TestRktDriver_Remove_Error(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	// Removing a nonexistent pod should return an error
	if err := rktRemove("00000000-0000-0000-0000-000000000000"); err == nil {
		t.Fatalf("expected an error")
	}

	if err := rktRemove("zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"); err == nil {
		t.Fatalf("expected an error")
	}
}
