package spawn

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	switch os.Getenv("TEST_MAIN") {
	case "app":
		appMain()
	default:
		os.Exit(m.Run())
	}
}

func appMain() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "no command provided")
		os.Exit(1)
	}
	switch cmd := os.Args[1]; cmd {
	case "echo":
		fmt.Println(strings.Join(os.Args[2:], " "))
	case "sleep":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "expected 3 args")
			os.Exit(1)
		}
		dur, err := time.ParseDuration(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not parse sleep time: %v", err)
			os.Exit(1)
		}
		time.Sleep(dur)
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", cmd)
		os.Exit(1)
	}
}

func TestSpawn_NoCmd(t *testing.T) {
	tempFile := tempFileName(t)
	defer os.Remove(tempFile)

	spawn := NewSpawner(tempFile)
	if err := spawn.Spawn(nil); err == nil {
		t.Fatalf("Spawn() with no user command should fail")
	}
}

func TestSpawn_InvalidCmd(t *testing.T) {
	tempFile := tempFileName(t)
	defer os.Remove(tempFile)

	spawn := NewSpawner(tempFile)
	spawn.SetCommand(exec.Command("foo")) // non-existent command
	if err := spawn.Spawn(nil); err == nil {
		t.Fatalf("Spawn() with an invalid command should fail")
	}
}

func TestSpawn_SetsLogs(t *testing.T) {
	tempFile := tempFileName(t)
	defer os.Remove(tempFile)

	spawn := NewSpawner(tempFile)
	exp := "foo"
	spawn.SetCommand(testCommand("echo", exp))

	// Create file for stdout.
	stdout := tempFileName(t)
	defer os.Remove(stdout)
	spawn.SetLogs(&Logs{Stdout: stdout})

	if err := spawn.Spawn(nil); err != nil {
		t.Fatalf("Spawn() failed: %v", err)
	}

	if res := spawn.Wait(); res.ExitCode != 0 && res.Err != nil {
		t.Fatalf("Wait() returned %v, %v; want 0, nil", res.ExitCode, res.Err)
	}

	stdout2, err := os.Open(stdout)
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
	tempFile := tempFileName(t)
	defer os.Remove(tempFile)

	spawn := NewSpawner(tempFile)
	spawn.SetCommand(testCommand("sleep", "1s"))

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
	tempFile := tempFileName(t)
	defer os.Remove(tempFile)

	spawn := NewSpawner(tempFile)
	spawn.SetCommand(testCommand("echo", "foo"))
	if err := spawn.Spawn(nil); err != nil {
		t.Fatalf("Spawn() failed %v", err)
	}

	time.Sleep(1 * time.Second)

	if res := spawn.Wait(); res.ExitCode != 0 && res.Err != nil {
		t.Fatalf("Wait() returned %v, %v; want 0, nil", res.ExitCode, res.Err)
	}
}

func TestSpawn_ParentWait(t *testing.T) {
	tempFile := tempFileName(t)
	defer os.Remove(tempFile)

	spawn := NewSpawner(tempFile)
	spawn.SetCommand(testCommand("sleep", "2s"))
	if err := spawn.Spawn(nil); err != nil {
		t.Fatalf("Spawn() failed %v", err)
	}

	if res := spawn.Wait(); res.ExitCode != 0 && res.Err != nil {
		t.Fatalf("Wait() returned %v, %v; want 0, nil", res.ExitCode, res.Err)
	}
}

func TestSpawn_NonParentWaitExited(t *testing.T) {
	tempFile := tempFileName(t)
	defer os.Remove(tempFile)

	spawn := NewSpawner(tempFile)
	spawn.SetCommand(testCommand("echo", "foo"))
	if err := spawn.Spawn(nil); err != nil {
		t.Fatalf("Spawn() failed %v", err)
	}

	time.Sleep(1 * time.Second)

	// Force the wait to assume non-parent.
	spawn.SpawnPpid = 0
	if res := spawn.Wait(); res.ExitCode != 0 && res.Err != nil {
		t.Fatalf("Wait() returned %v, %v; want 0, nil", res.ExitCode, res.Err)
	}
}

func TestSpawn_NonParentWait(t *testing.T) {
	tempFile := tempFileName(t)
	defer os.Remove(tempFile)

	spawn := NewSpawner(tempFile)
	spawn.SetCommand(testCommand("sleep", "2s"))
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
	if res := spawn.Wait(); res.ExitCode != 0 && res.Err != nil {
		t.Fatalf("Wait() returned %v, %v; want 0, nil", res.ExitCode, res.Err)
	}
}

func TestSpawn_DeadSpawnDaemon_Parent(t *testing.T) {
	tempFile := tempFileName(t)
	defer os.Remove(tempFile)

	var spawnPid int
	cb := func(pid int) error {
		spawnPid = pid
		return nil
	}

	spawn := NewSpawner(tempFile)
	spawn.SetCommand(testCommand("sleep", "5s"))
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

	if res := spawn.Wait(); res.Err == nil {
		t.Fatalf("Wait() should have failed: %v", res.Err)
	}
}

func TestSpawn_DeadSpawnDaemon_NonParent(t *testing.T) {
	tempFile := tempFileName(t)
	defer os.Remove(tempFile)

	var spawnPid int
	cb := func(pid int) error {
		spawnPid = pid
		return nil
	}

	spawn := NewSpawner(tempFile)
	spawn.SetCommand(testCommand("sleep", "2s"))
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
	if res := spawn.Wait(); res.Err == nil {
		t.Fatalf("Wait() should have failed: %v", res.Err)
	}
}

func TestSpawn_Valid_TaskRunning(t *testing.T) {
	tempFile := tempFileName(t)
	defer os.Remove(tempFile)

	spawn := NewSpawner(tempFile)
	spawn.SetCommand(testCommand("sleep", "2s"))
	if err := spawn.Spawn(nil); err != nil {
		t.Fatalf("Spawn() failed %v", err)
	}

	if err := spawn.Valid(); err != nil {
		t.Fatalf("Valid() failed: %v", err)
	}

	if res := spawn.Wait(); res.Err != nil {
		t.Fatalf("Wait() failed: %v", res.Err)
	}
}

func TestSpawn_Valid_TaskExit_ExitCode(t *testing.T) {
	tempFile := tempFileName(t)
	defer os.Remove(tempFile)

	spawn := NewSpawner(tempFile)
	spawn.SetCommand(testCommand("echo", "foo"))
	if err := spawn.Spawn(nil); err != nil {
		t.Fatalf("Spawn() failed %v", err)
	}

	if res := spawn.Wait(); res.Err != nil {
		t.Fatalf("Wait() failed: %v", res.Err)
	}

	if err := spawn.Valid(); err != nil {
		t.Fatalf("Valid() failed: %v", err)
	}
}

func TestSpawn_Valid_TaskExit_NoExitCode(t *testing.T) {
	tempFile := tempFileName(t)
	defer os.Remove(tempFile)

	spawn := NewSpawner(tempFile)
	spawn.SetCommand(testCommand("echo", "foo"))
	if err := spawn.Spawn(nil); err != nil {
		t.Fatalf("Spawn() failed %v", err)
	}

	if res := spawn.Wait(); res.Err != nil {
		t.Fatalf("Wait() failed: %v", res.Err)
	}

	// Delete the file so that it can't find the exit code.
	os.Remove(tempFile)

	if err := spawn.Valid(); err == nil {
		t.Fatalf("Valid() should have failed")
	}
}

func tempFileName(t *testing.T) string {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("TempFile() failed")
	}
	defer f.Close()
	return f.Name()
}

func testCommand(args ...string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "TEST_MAIN=app")
	return cmd
}
