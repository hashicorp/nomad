// +build linux

package rkt

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/hcl2/hcl"
	ctestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/testtask"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	basePlug "github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/plugins/shared/hclutils"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

var _ drivers.DriverPlugin = (*Driver)(nil)

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

// Tests setting driver config options
func TestRktDriver_SetConfig(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	d := NewRktDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	// Enable Volumes
	config := &Config{
		VolumesEnabled: true,
	}

	var data []byte
	require.NoError(basePlug.MsgPackEncode(&data, config))
	bconfig := &basePlug.Config{PluginConfig: data}
	require.NoError(harness.SetConfig(bconfig))
	require.Exactly(config, d.(*Driver).config)

	config.VolumesEnabled = false
	data = []byte{}
	require.NoError(basePlug.MsgPackEncode(&data, config))
	bconfig = &basePlug.Config{PluginConfig: data}
	require.NoError(harness.SetConfig(bconfig))
	require.Exactly(config, d.(*Driver).config)

}

// Verifies using a trust prefix and passing dns servers and search domains
// Also verifies sending sigterm correctly stops the driver instance
func TestRktDriver_Start_Wait_Stop_DNS(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewRktDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:      uuid.Generate(),
		AllocID: uuid.Generate(),
		Name:    "etcd",
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 128,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 100,
				},
			},
			LinuxResources: &drivers.LinuxResources{
				MemoryLimitBytes: 134217728,
				CPUShares:        100,
			},
		},
	}

	taskConfig := map[string]interface{}{
		"trust_prefix":       "coreos.com/etcd",
		"image":              "coreos.com/etcd:v2.0.4",
		"command":            "/etcd",
		"dns_servers":        []string{"8.8.8.8", "8.8.4.4"},
		"dns_search_domains": []string{"example.com", "example.org", "example.net"},
		"net":                []string{"host"},
	}

	encodeDriverHelper(require, task, taskConfig)
	testtask.SetTaskConfigEnv(task)
	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	handle, driverNet, err := harness.StartTask(task)
	require.NoError(err)
	require.Nil(driverNet)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)

	require.NoError(harness.WaitUntilStarted(task.ID, 1*time.Second))

	go func() {
		harness.StopTask(task.ID, 2*time.Second, "SIGTERM")
	}()

	select {
	case result := <-ch:
		require.Equal(int(unix.SIGTERM), result.Signal)
	case <-time.After(10 * time.Second):
		require.Fail("timeout waiting for task to shutdown")
	}

	// Ensure that the task is marked as dead, but account
	// for WaitTask() closing channel before internal state is updated
	testutil.WaitForResult(func() (bool, error) {
		status, err := harness.InspectTask(task.ID)
		if err != nil {
			return false, fmt.Errorf("inspecting task failed: %v", err)
		}
		if status.State != drivers.TaskStateExited {
			return false, fmt.Errorf("task hasn't exited yet; status: %v", status.State)
		}

		return true, nil
	}, func(err error) {
		require.NoError(err)
	})

	require.NoError(harness.DestroyTask(task.ID, true))
}

// Verifies waiting on task to exit cleanly
func TestRktDriver_Start_Wait_Stop(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewRktDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:      uuid.Generate(),
		AllocID: uuid.Generate(),
		Name:    "etcd",
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 128,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 100,
				},
			},
			LinuxResources: &drivers.LinuxResources{
				MemoryLimitBytes: 134217728,
				CPUShares:        100,
			},
		},
	}

	taskConfig := map[string]interface{}{
		"trust_prefix": "coreos.com/etcd",
		"image":        "coreos.com/etcd:v2.0.4",
		"command":      "/etcd",
		"args":         []string{"--version"},
		"net":          []string{"none"},
		"debug":        true,
	}

	encodeDriverHelper(require, task, taskConfig)
	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	// Wait on the task, it should exit since we are only asking for etcd version here
	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)
	result := <-ch
	require.Nil(result.Err)

	require.Zero(result.ExitCode)

	require.NoError(harness.DestroyTask(task.ID, true))

}

// Verifies that skipping trust_prefix works
func TestRktDriver_Start_Wait_Skip_Trust(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewRktDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:      uuid.Generate(),
		AllocID: uuid.Generate(),
		Name:    "etcd",
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 128,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 100,
				},
			},
			LinuxResources: &drivers.LinuxResources{
				MemoryLimitBytes: 134217728,
				CPUShares:        100,
			},
		},
	}

	taskConfig := map[string]interface{}{
		"image":   "coreos.com/etcd:v2.0.4",
		"command": "/etcd",
		"args":    []string{"--version"},
		"net":     []string{"none"},
		"debug":   true,
	}

	encodeDriverHelper(require, task, taskConfig)
	testtask.SetTaskConfigEnv(task)

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	// Wait on the task, it should exit since we are only asking for etcd version here
	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)
	result := <-ch
	require.Nil(result.Err)
	require.Zero(result.ExitCode)

	require.NoError(harness.DestroyTask(task.ID, true))

}

// Verifies that an invalid trust prefix returns expected error
func TestRktDriver_InvalidTrustPrefix(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewRktDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:      uuid.Generate(),
		AllocID: uuid.Generate(),
		Name:    "etcd",
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 128,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 100,
				},
			},
			LinuxResources: &drivers.LinuxResources{
				MemoryLimitBytes: 134217728,
				CPUShares:        100,
			},
		},
	}

	taskConfig := map[string]interface{}{
		"trust_prefix": "example.com/invalid",
		"image":        "coreos.com/etcd:v2.0.4",
		"command":      "/etcd",
		"args":         []string{"--version"},
		"net":          []string{"none"},
		"debug":        true,
	}

	encodeDriverHelper(require, task, taskConfig)
	testtask.SetTaskConfigEnv(task)

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	_, _, err := harness.StartTask(task)
	require.Error(err)
	expectedErr := "Error running rkt trust"
	require.Contains(err.Error(), expectedErr)

}

// Verifies reattaching to a running container
// This test manipulates the harness's internal state map
// to remove the task and then reattaches to it
func TestRktDriver_StartWaitRecoverWaitStop(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewRktDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:      uuid.Generate(),
		AllocID: uuid.Generate(),
		Name:    "etcd",
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 128,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 100,
				},
			},
			LinuxResources: &drivers.LinuxResources{
				MemoryLimitBytes: 134217728,
				CPUShares:        100,
			},
		},
	}

	taskConfig := map[string]interface{}{
		"image":   "coreos.com/etcd:v2.0.4",
		"command": "/etcd",
	}

	encodeDriverHelper(require, task, taskConfig)

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), task.ID)
	require.NoError(err)

	var waitDone bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result := <-ch
		require.Error(result.Err)
		waitDone = true
	}()

	originalStatus, err := d.InspectTask(task.ID)
	require.NoError(err)

	d.(*Driver).tasks.Delete(task.ID)

	wg.Wait()
	require.True(waitDone)
	_, err = d.InspectTask(task.ID)
	require.Equal(drivers.ErrTaskNotFound, err)

	err = d.RecoverTask(handle)
	require.NoError(err)

	status, err := d.InspectTask(task.ID)
	require.NoError(err)
	require.Exactly(originalStatus, status)

	ch, err = harness.WaitTask(context.Background(), task.ID)
	require.NoError(err)

	wg.Add(1)
	waitDone = false
	go func() {
		defer wg.Done()
		result := <-ch
		require.NoError(result.Err)
		require.NotZero(result.ExitCode)
		require.Equal(9, result.Signal)
		waitDone = true
	}()

	time.Sleep(300 * time.Millisecond)
	require.NoError(d.StopTask(task.ID, 0, "SIGKILL"))
	wg.Wait()
	require.NoError(d.DestroyTask(task.ID, false))
	require.True(waitDone)

}

// Verifies mounting a volume from the host machine and writing
// some data to it from inside the container
func TestRktDriver_Start_Wait_Volume(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewRktDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	// enable volumes
	config := &Config{VolumesEnabled: true}

	var data []byte
	require.NoError(basePlug.MsgPackEncode(&data, config))
	bconfig := &basePlug.Config{PluginConfig: data}
	require.NoError(harness.SetConfig(bconfig))

	task := &drivers.TaskConfig{
		ID:      uuid.Generate(),
		AllocID: uuid.Generate(),
		Name:    "rkttest_alpine",
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 128,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 100,
				},
			},
			LinuxResources: &drivers.LinuxResources{
				MemoryLimitBytes: 134217728,
				CPUShares:        100,
			},
		},
	}
	exp := []byte{'w', 'i', 'n'}
	file := "output.txt"
	tmpvol, err := ioutil.TempDir("", "nomadtest_rktdriver_volumes")
	require.NoError(err)
	defer os.RemoveAll(tmpvol)
	hostpath := filepath.Join(tmpvol, file)

	taskConfig := map[string]interface{}{
		"image":   "docker://redis:3.2-alpine",
		"command": "/bin/sh",
		"args": []string{
			"-c",
			fmt.Sprintf("echo -n %s > /foo/%s", string(exp), file),
		},
		"net":     []string{"none"},
		"volumes": []string{fmt.Sprintf("%s:/foo", tmpvol)},
	}

	encodeDriverHelper(require, task, taskConfig)
	testtask.SetTaskConfigEnv(task)

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	_, _, err = harness.StartTask(task)
	require.NoError(err)

	// Task should terminate quickly
	waitCh, err := harness.WaitTask(context.Background(), task.ID)
	require.NoError(err)

	select {
	case res := <-waitCh:
		require.NoError(res.Err)
		require.True(res.Successful(), fmt.Sprintf("exit code %v", res.ExitCode))
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail("WaitTask timeout")
	}

	// Check that data was written to the shared alloc directory.
	act, err := ioutil.ReadFile(hostpath)
	require.NoError(err)
	require.Exactly(exp, act)
	require.NoError(harness.DestroyTask(task.ID, true))
}

// Verifies mounting a task mount from the host machine and writing
// some data to it from inside the container
func TestRktDriver_Start_Wait_TaskMounts(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewRktDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	// mounts through task config should be enabled regardless
	config := &Config{VolumesEnabled: false}

	var data []byte
	require.NoError(basePlug.MsgPackEncode(&data, config))
	bconfig := &basePlug.Config{PluginConfig: data}
	require.NoError(harness.SetConfig(bconfig))

	tmpvol, err := ioutil.TempDir("", "nomadtest_rktdriver_volumes")
	require.NoError(err)
	defer os.RemoveAll(tmpvol)

	task := &drivers.TaskConfig{
		ID:      uuid.Generate(),
		AllocID: uuid.Generate(),
		Name:    "rkttest_alpine",
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 128,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 100,
				},
			},
			LinuxResources: &drivers.LinuxResources{
				MemoryLimitBytes: 134217728,
				CPUShares:        100,
			},
		},
		Mounts: []*drivers.MountConfig{
			{HostPath: tmpvol, TaskPath: "/foo", Readonly: false},
		},
	}
	exp := []byte{'w', 'i', 'n'}
	file := "output.txt"
	hostpath := filepath.Join(tmpvol, file)

	taskConfig := map[string]interface{}{
		"image":   "docker://redis:3.2-alpine",
		"command": "/bin/sh",
		"args": []string{
			"-c",
			fmt.Sprintf("echo -n %s > /foo/%s", string(exp), file),
		},
		"net": []string{"none"},
	}

	encodeDriverHelper(require, task, taskConfig)
	testtask.SetTaskConfigEnv(task)

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	_, _, err = harness.StartTask(task)
	require.NoError(err)

	// Task should terminate quickly
	waitCh, err := harness.WaitTask(context.Background(), task.ID)
	require.NoError(err)

	select {
	case res := <-waitCh:
		require.NoError(res.Err)
		require.True(res.Successful(), fmt.Sprintf("exit code %v", res.ExitCode))
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail("WaitTask timeout")
	}

	// Check that data was written to the shared alloc directory.
	act, err := ioutil.ReadFile(hostpath)
	require.NoError(err)
	require.Exactly(exp, act)
	require.NoError(harness.DestroyTask(task.ID, true))
}

// Verifies port mapping
func TestRktDriver_PortMapping(t *testing.T) {
	ctestutil.RktCompatible(t)

	require := require.New(t)
	d := NewRktDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:      uuid.Generate(),
		AllocID: uuid.Generate(),
		Name:    "redis",
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 128,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 100,
				},
				Networks: []*structs.NetworkResource{
					{
						IP:            "127.0.0.1",
						ReservedPorts: []structs.Port{{Label: "main", Value: 8080}},
					},
				},
			},
			LinuxResources: &drivers.LinuxResources{
				MemoryLimitBytes: 134217728,
				CPUShares:        100,
			},
		},
	}

	taskConfig := map[string]interface{}{
		"image": "docker://redis:3.2-alpine",
		"port_map": map[string]string{
			"main": "6379-tcp",
		},
		"debug": "true",
	}

	encodeDriverHelper(require, task, taskConfig)

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	_, driverNetwork, err := harness.StartTask(task)
	require.NoError(err)
	require.NotNil(driverNetwork)
	require.NoError(harness.DestroyTask(task.ID, true))
}

// This test starts a redis container, setting user and group.
// It verifies that running ps inside the container shows the expected user and group
func TestRktDriver_UserGroup(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewRktDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:      uuid.Generate(),
		AllocID: uuid.Generate(),
		User:    "nobody",
		Name:    "rkttest_alpine",
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 128,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 100,
				},
			},
			LinuxResources: &drivers.LinuxResources{
				MemoryLimitBytes: 134217728,
				CPUShares:        100,
			},
		},
	}

	taskConfig := map[string]interface{}{
		"image":   "docker://redis:3.2-alpine",
		"group":   "nogroup",
		"command": "sleep",
		"args":    []string{"9000"},
		"net":     []string{"none"},
	}

	encodeDriverHelper(require, task, taskConfig)
	testtask.SetTaskConfigEnv(task)

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	_, _, err := harness.StartTask(task)
	require.NoError(err)

	expected := []byte("\nnobody   nogroup  /bin/sleep 9000\n")
	testutil.WaitForResult(func() (bool, error) {
		res, err := d.ExecTask(task.ID, []string{"ps", "-o", "user,group,args"}, time.Second)
		if err != nil {
			return false, fmt.Errorf("failed to exec: %#v", err)
		}
		if !res.ExitResult.Successful() {
			return false, fmt.Errorf("ps failed: %#v %#v", res.ExitResult, res)
		}
		raw := res.Stdout
		return bytes.Contains(raw, expected), fmt.Errorf("expected %q but found:\n%s", expected, raw)
	}, func(err error) {
		require.NoError(err)
	})

	require.NoError(harness.DestroyTask(task.ID, true))
}

//  Verifies executing both correct and incorrect commands inside the container
func TestRktDriver_Exec(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewRktDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:      uuid.Generate(),
		AllocID: uuid.Generate(),
		Name:    "etcd",
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 128,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 100,
				},
			},
			LinuxResources: &drivers.LinuxResources{
				MemoryLimitBytes: 134217728,
				CPUShares:        100,
			},
		},
	}

	taskConfig := map[string]interface{}{
		"trust_prefix": "coreos.com/etcd",
		"image":        "coreos.com/etcd:v2.0.4",
		"net":          []string{"none"},
	}

	encodeDriverHelper(require, task, taskConfig)
	testtask.SetTaskConfigEnv(task)

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	_, _, err := harness.StartTask(task)
	require.NoError(err)

	// Run command that should succeed
	expected := []byte("etcd version")
	testutil.WaitForResult(func() (bool, error) {
		res, err := d.ExecTask(task.ID, []string{"/etcd", "--version"}, time.Second)
		if err != nil {
			return false, fmt.Errorf("failed to exec: %#v", err)
		}
		if !res.ExitResult.Successful() {
			return false, fmt.Errorf("/etcd --version failed: %#v %#v", res.ExitResult, res)
		}
		raw := res.Stdout
		return bytes.Contains(raw, expected), fmt.Errorf("expected %q but found:\n%s", expected, raw)
	}, func(err error) {
		require.NoError(err)
	})

	// Run command that should fail
	expected = []byte("flag provided but not defined")
	testutil.WaitForResult(func() (bool, error) {
		res, err := d.ExecTask(task.ID, []string{"/etcd", "--cgdfgdfg"}, time.Second)
		if err != nil {
			return false, fmt.Errorf("failed to exec: %#v", err)
		}
		if res.ExitResult.Successful() {
			return false, fmt.Errorf("/etcd --cgdfgdfg unexpected succeeded: %#v %#v", res.ExitResult, res)
		}
		raw := res.Stdout
		return bytes.Contains(raw, expected), fmt.Errorf("expected %q but found:\n%s", expected, raw)
	}, func(err error) {
		require.NoError(err)
	})

	require.NoError(harness.DestroyTask(task.ID, true))
}

//  Verifies getting resource usage stats
// TODO(preetha) figure out why stats are zero
func TestRktDriver_Stats(t *testing.T) {
	ctestutil.RktCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewRktDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:      uuid.Generate(),
		AllocID: uuid.Generate(),
		Name:    "etcd",
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 128,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 100,
				},
			},
			LinuxResources: &drivers.LinuxResources{
				MemoryLimitBytes: 134217728,
				CPUShares:        100,
			},
		},
	}

	taskConfig := map[string]interface{}{
		"trust_prefix": "coreos.com/etcd",
		"image":        "coreos.com/etcd:v2.0.4",
		"command":      "/etcd",
		"net":          []string{"none"},
	}

	encodeDriverHelper(require, task, taskConfig)
	testtask.SetTaskConfigEnv(task)

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	// Wait for task to start
	_, err = harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)

	// Wait until task started
	require.NoError(harness.WaitUntilStarted(task.ID, 1*time.Second))

	resourceUsage, err := d.TaskStats(task.ID)
	require.Nil(err)

	//TODO(preetha) why are these zero
	fmt.Printf("pid map %v\n", resourceUsage.Pids)
	fmt.Printf("CPU:%+v Memory:%+v", resourceUsage.ResourceUsage.CpuStats, resourceUsage.ResourceUsage.MemoryStats)

	require.NoError(harness.DestroyTask(task.ID, true))

}

func encodeDriverHelper(require *require.Assertions, task *drivers.TaskConfig, taskConfig map[string]interface{}) {
	evalCtx := &hcl.EvalContext{
		Functions: hclutils.GetStdlibFuncs(),
	}
	spec, diag := hclspec.Convert(taskConfigSpec)
	require.False(diag.HasErrors())
	taskConfigCtyVal, diag := hclutils.ParseHclInterface(taskConfig, spec, evalCtx)
	if diag.HasErrors() {
		fmt.Println("conversion error", diag.Error())
	}
	require.False(diag.HasErrors())
	err := task.EncodeDriverConfig(taskConfigCtyVal)
	require.Nil(err)
}
