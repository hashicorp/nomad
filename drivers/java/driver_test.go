package java

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"context"
	"time"

	"github.com/hashicorp/hcl2/hcl"
	ctestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
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

func TestDriver_Fingerprint(t *testing.T) {
	javaCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	d := NewDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)

	fpCh, err := harness.Fingerprint(context.Background())
	require.NoError(t, err)

	select {
	case fp := <-fpCh:
		require.Equal(t, drivers.HealthStateHealthy, fp.Health)
		require.Equal(t, "1", fp.Attributes["driver.java"])
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail(t, "timeout receiving fingerprint")
	}
}

func TestDriver_Jar_Start_Wait(t *testing.T) {
	javaCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)

	task := basicTask(t, "demo-app", map[string]interface{}{
		"jar_path":    "demoapp.jar",
		"args":        []string{"1"},
		"jvm_options": []string{"-Xmx64m", "-Xms32m"},
	})

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	copyFile("./test-resources/java/demoapp.jar", filepath.Join(task.TaskDir().Dir, "demoapp.jar"), t)

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

func TestDriver_Jar_Stop_Wait(t *testing.T) {
	javaCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)

	task := basicTask(t, "demo-app", map[string]interface{}{
		"jar_path":    "demoapp.jar",
		"args":        "20",
		"jvm_options": []string{"-Xmx64m", "-Xms32m"},
	})

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	copyFile("./test-resources/java/demoapp.jar", filepath.Join(task.TaskDir().Dir, "demoapp.jar"), t)

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result := <-ch
		require.Equal(2, result.Signal)
	}()

	require.NoError(harness.WaitUntilStarted(task.ID, 1*time.Second))

	wg.Add(1)
	go func() {
		defer wg.Done()

		time.Sleep(10 * time.Millisecond)
		err := harness.StopTask(task.ID, 2*time.Second, "SIGINT")
		require.NoError(err)
	}()

	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		wg.Wait()
	}()

	select {
	case <-waitCh:
		status, err := harness.InspectTask(task.ID)
		require.NoError(err)
		require.Equal(drivers.TaskStateExited, status.State)
	case <-time.After(5 * time.Second):
		require.Fail("timeout waiting for task to shutdown")
	}

	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestDriver_Class_Start_Wait(t *testing.T) {
	javaCompatible(t)
	if !testutil.IsTravis() {
		t.Parallel()
	}

	require := require.New(t)
	d := NewDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)

	task := basicTask(t, "demo-app", map[string]interface{}{
		"class_path": "${NOMAD_TASK_DIR}",
		"class":      "Hello",
		"args":       []string{"1"},
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
	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: name,
		Resources: &drivers.Resources{
			NomadResources: &structs.Resources{
				MemoryMB: 128,
				CPU:      100,
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
	evalCtx := &hcl.EvalContext{
		Functions: shared.GetStdlibFuncs(),
	}
	spec, diag := hclspec.Convert(taskConfigSpec)
	require.False(t, diag.HasErrors())
	taskConfigCtyVal, diag := shared.ParseHclInterface(taskConfig, spec, evalCtx)
	if diag.HasErrors() {
		fmt.Println("conversion error", diag.Error())
	}
	require.False(t, diag.HasErrors())
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
