package spawn

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSpawn_NoCmd(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("TempFile() failed")
	}
	defer os.Remove(f.Name())

	spawn := NewSpawner(f.Name())
	if err := spawn.Spawn(nil); err == nil {
		t.Fatalf("Spawn() with no user command should fail")
	}
}

func TestSpawn_InvalidCmd(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("TempFile() failed")
	}
	defer os.Remove(f.Name())

	spawn := NewSpawner(f.Name())
	spawn.SetCommand(exec.Command("foo"))
	if err := spawn.Spawn(nil); err == nil {
		t.Fatalf("Spawn() with no invalid command should fail")
	}
}

func TestSpawn_SetsLogs(t *testing.T) {
	// TODO: Figure out why this test fails. If the spawn-daemon directly writes
	// to the opened stdout file it works but not the user command. Maybe a
	// flush issue?
	if runtime.GOOS == "windows" {
		t.Skip("Test fails on windows; unknown reason. Skipping")
	}

	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("TempFile() failed")
	}
	defer os.Remove(f.Name())

	spawn := NewSpawner(f.Name())
	exp := "foo"
	spawn.SetCommand(exec.Command("echo", exp))

	// Create file for stdout.
	stdout, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("TempFile() failed")
	}
	defer os.Remove(stdout.Name())
	spawn.SetLogs(&Logs{Stdout: stdout.Name()})

	if err := spawn.Spawn(nil); err != nil {
		t.Fatalf("Spawn() failed: %v", err)
	}

	if code, err := spawn.Wait(); code != 0 && err != nil {
		t.Fatalf("Wait() returned %v, %v; want 0, nil", code, err)
	}

	stdout2, err := os.Open(stdout.Name())
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}

	data, err := ioutil.ReadAll(stdout2)
	if err != nil {
		t.Fatalf("ReadAll() failed: %v", err)
	}

	act := strings.TrimSpace(string(data))
	if act != exp {
		t.Fatalf("Unexpected data written to stdout; got %v; want %v", act, exp)
	}
}

func TestSpawn_Callback(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("TempFile() failed")
	}
	defer os.Remove(f.Name())

	spawn := NewSpawner(f.Name())
	spawn.SetCommand(exec.Command("sleep", "1"))

	called := false
	cbErr := fmt.Errorf("ERROR CB")
	cb := func(_ int) error {
		called = true
		return cbErr
	}

	if err := spawn.Spawn(cb); err == nil {
		t.Fatalf("Spawn(%#v) should have errored; want %v", cb, cbErr)
	}

	if !called {
		t.Fatalf("Spawn(%#v) didn't call callback", cb)
	}
}

func TestSpawn_ParentWaitExited(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("TempFile() failed")
	}
	defer os.Remove(f.Name())

	spawn := NewSpawner(f.Name())
	spawn.SetCommand(exec.Command("echo", "foo"))
	if err := spawn.Spawn(nil); err != nil {
		t.Fatalf("Spawn() failed %v", err)
	}

	time.Sleep(1 * time.Second)

	code, err := spawn.Wait()
	if err != nil {
		t.Fatalf("Wait() failed %v", err)
	}

	if code != 0 {
		t.Fatalf("Wait() returned %v; want 0", code)
	}
}

func TestSpawn_ParentWait(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("TempFile() failed")
	}
	defer os.Remove(f.Name())

	spawn := NewSpawner(f.Name())
	spawn.SetCommand(exec.Command("sleep", "2"))
	if err := spawn.Spawn(nil); err != nil {
		t.Fatalf("Spawn() failed %v", err)
	}

	code, err := spawn.Wait()
	if err != nil {
		t.Fatalf("Wait() failed %v", err)
	}

	if code != 0 {
		t.Fatalf("Wait() returned %v; want 0", code)
	}
}

func TestSpawn_NonParentWaitExited(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("TempFile() failed")
	}
	defer os.Remove(f.Name())

	spawn := NewSpawner(f.Name())
	spawn.SetCommand(exec.Command("echo", "foo"))
	if err := spawn.Spawn(nil); err != nil {
		t.Fatalf("Spawn() failed %v", err)
	}

	time.Sleep(1 * time.Second)

	// Force the wait to assume non-parent.
	spawn.SpawnPpid = 0
	code, err := spawn.Wait()
	if err != nil {
		t.Fatalf("Wait() failed %v", err)
	}

	if code != 0 {
		t.Fatalf("Wait() returned %v; want 0", code)
	}
}

func TestSpawn_NonParentWait(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("TempFile() failed")
	}
	defer os.Remove(f.Name())

	spawn := NewSpawner(f.Name())
	spawn.SetCommand(exec.Command("sleep", "2"))
	if err := spawn.Spawn(nil); err != nil {
		t.Fatalf("Spawn() failed %v", err)
	}

	// Need to wait on the spawner, otherwise it becomes a zombie and the test
	// only finishes after the init process cleans it. This speeds that up.
	go func() {
		time.Sleep(3 * time.Second)
		if _, err := spawn.spawn.Wait(); err != nil {
			t.FailNow()
		}
	}()

	// Force the wait to assume non-parent.
	spawn.SpawnPpid = 0
	code, err := spawn.Wait()
	if err != nil {
		t.Fatalf("Wait() failed %v", err)
	}

	if code != 0 {
		t.Fatalf("Wait() returned %v; want 0", code)
	}
}

func TestSpawn_DeadSpawnDaemon_Parent(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("TempFile() failed")
	}
	defer os.Remove(f.Name())

	var spawnPid int
	cb := func(pid int) error {
		spawnPid = pid
		return nil
	}

	spawn := NewSpawner(f.Name())
	spawn.SetCommand(exec.Command("sleep", "5"))
	if err := spawn.Spawn(cb); err != nil {
		t.Fatalf("Spawn() errored: %v", err)
	}

	proc, err := os.FindProcess(spawnPid)
	if err != nil {
		t.FailNow()
	}

	if err := proc.Kill(); err != nil {
		t.FailNow()
	}

	if _, err := proc.Wait(); err != nil {
		t.FailNow()
	}

	if _, err := spawn.Wait(); err == nil {
		t.Fatalf("Wait() should have failed: %v", err)
	}
}

func TestSpawn_DeadSpawnDaemon_NonParent(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("TempFile() failed")
	}
	defer os.Remove(f.Name())

	var spawnPid int
	cb := func(pid int) error {
		spawnPid = pid
		return nil
	}

	spawn := NewSpawner(f.Name())
	spawn.SetCommand(exec.Command("sleep", "2"))
	if err := spawn.Spawn(cb); err != nil {
		t.Fatalf("Spawn() errored: %v", err)
	}

	proc, err := os.FindProcess(spawnPid)
	if err != nil {
		t.FailNow()
	}

	if err := proc.Kill(); err != nil {
		t.FailNow()
	}

	if _, err := proc.Wait(); err != nil {
		t.FailNow()
	}

	// Force the wait to assume non-parent.
	spawn.SpawnPpid = 0
	if _, err := spawn.Wait(); err == nil {
		t.Fatalf("Wait() should have failed: %v", err)
	}
}
