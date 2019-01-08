package java

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"

	"context"
	"time"

	"github.com/hashicorp/hcl2/hcl"
	ctestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/plugins/shared/hclutils"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func javaCompatible(t *testing.T) {
	ctestutil.JavaCompatible(t)

	_, _, _, err := javaVersionInfo()
	if err != nil {
		t.Skipf("java not found; skipping: %v", err)
	}
}

func TestJavaDriver_Fingerprint(t *testing.T) {
	javaCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	d := NewDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	fpCh, err := harness.Fingerprint(context.Background())
	require.NoError(t, err)

	select {
	case fp := <-fpCh:
		require.Equal(t, drivers.HealthStateHealthy, fp.Health)
		detected, _ := fp.Attributes["driver.java"].GetBool()
		require.True(t, detected)
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail(t, "timeout receiving fingerprint")
	}
}

func TestJavaDriver_Jar_Start_Wait(t *testing.T) {
	javaCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := basicTask(t, "demo-app", map[string]interface{}{
		"jar_path":    "demoapp.jar",
		"args":        []string{"1"},
		"jvm_options": []string{"-Xmx64m", "-Xms32m"},
	})

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	copyFile("./test-resources/demoapp.jar", filepath.Join(task.TaskDir().Dir, "demoapp.jar"), t)

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)
	result := <-ch
	require.Nil(result.Err)

	require.Zero(result.ExitCode)

	// Get the stdout of the process and assert that it's not empty
	stdout, err := ioutil.ReadFile(filepath.Join(task.TaskDir().LogDir, "demo-app.stdout.0"))
	require.NoError(err)
	require.Contains(string(stdout), "Hello")

	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestJavaDriver_Jar_Stop_Wait(t *testing.T) {
	javaCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := basicTask(t, "demo-app", map[string]interface{}{
		"jar_path":    "demoapp.jar",
		"args":        []string{"600"},
		"jvm_options": []string{"-Xmx64m", "-Xms32m"},
	})

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	copyFile("./test-resources/demoapp.jar", filepath.Join(task.TaskDir().Dir, "demoapp.jar"), t)

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)

	require.NoError(harness.WaitUntilStarted(task.ID, 1*time.Second))

	go func() {
		time.Sleep(10 * time.Millisecond)
		harness.StopTask(task.ID, 2*time.Second, "SIGINT")
	}()

	select {
	case result := <-ch:
		require.False(result.Successful())
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

func TestJavaDriver_Class_Start_Wait(t *testing.T) {
	javaCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := basicTask(t, "demo-app", map[string]interface{}{
		"class": "Hello",
		"args":  []string{"1"},
	})

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	copyFile("./test-resources/Hello.class", filepath.Join(task.TaskDir().Dir, "Hello.class"), t)

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)
	result := <-ch
	require.Nil(result.Err)

	require.Zero(result.ExitCode)

	// Get the stdout of the process and assert that it's not empty
	stdout, err := ioutil.ReadFile(filepath.Join(task.TaskDir().LogDir, "demo-app.stdout.0"))
	require.NoError(err)
	require.Contains(string(stdout), "Hello")

	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestJavaCmdArgs(t *testing.T) {
	cases := []struct {
		name     string
		cfg      TaskConfig
		expected []string
	}{
		{
			"jar_path_full",
			TaskConfig{
				JvmOpts: []string{"-Xmx512m", "-Xms128m"},
				JarPath: "/jar-path.jar",
				Args:    []string{"hello", "world"},
			},
			[]string{"-Xmx512m", "-Xms128m", "-jar", "/jar-path.jar", "hello", "world"},
		},
		{
			"class_full",
			TaskConfig{
				JvmOpts:   []string{"-Xmx512m", "-Xms128m"},
				Class:     "ClassName",
				ClassPath: "/classpath",
				Args:      []string{"hello", "world"},
			},
			[]string{"-Xmx512m", "-Xms128m", "-cp", "/classpath", "ClassName", "hello", "world"},
		},
		{
			"jar_path_slim",
			TaskConfig{
				JarPath: "/jar-path.jar",
			},
			[]string{"-jar", "/jar-path.jar"},
		},
		{
			"class_slim",
			TaskConfig{
				Class: "ClassName",
			},
			[]string{"ClassName"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			found := javaCmdArgs(c.cfg)
			require.Equal(t, c.expected, found)
		})
	}
}

func basicTask(t *testing.T, name string, taskConfig map[string]interface{}) *drivers.TaskConfig {
	t.Helper()

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: name,
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

	encodeDriverHelper(t, task, taskConfig)
	return task
}

func encodeDriverHelper(t *testing.T, task *drivers.TaskConfig, taskConfig map[string]interface{}) {
	t.Helper()

	evalCtx := &hcl.EvalContext{
		Functions: hclutils.GetStdlibFuncs(),
	}
	spec, diag := hclspec.Convert(taskConfigSpec)
	require.False(t, diag.HasErrors())
	taskConfigCtyVal, diag := hclutils.ParseHclInterface(taskConfig, spec, evalCtx)
	require.Empty(t, diag.Errs())
	err := task.EncodeDriverConfig(taskConfigCtyVal)
	require.Nil(t, err)

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
