package driver

import (
        "fmt"
        "os"
        "os/exec"
        "testing"

        "github.com/hashicorp/nomad/client/config"
        "github.com/hashicorp/nomad/nomad/structs"

        ctestutils "github.com/hashicorp/nomad/client/testutil"
)

// rktLocated looks to see whether rkt binaries are available on this system
// before we try to run tests. We may need to tweak this for cross-OS support
// but I think this should work on *nix at least.
func rktLocated() bool {
        _, err := exec.Command("rkt", "version").CombinedOutput()
        return err == nil
}

func TestRktDriver_Handle(t *testing.T) {
        h := &rktHandle{
                proc:   &os.Process{Pid: 123},
                name:   "foo",
                doneCh: make(chan struct{}),
                waitCh: make(chan error, 1),
        }

        actual := h.ID()
        expected := `Rkt:{"Pid":123,"Name":"foo"}`
        if actual != expected {
                t.Errorf("Expected `%s`, found `%s`", expected, actual)
        }
}

// The fingerprinter test should always pass, even if rkt is not installed.
func TestRktDriver_Fingerprint(t *testing.T) {
        ctestutils.RktCompatible(t)
        d := NewRktDriver(testDriverContext(""))
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
        if node.Attributes["driver.rkt"] == "" {
                t.Fatalf("Missing Rkt driver")
        }
        if node.Attributes["driver.rkt.version"] == "" {
                t.Fatalf("Missing Rkt driver version")
        }
}

func TestRktDriver_Start(t *testing.T) {
        if !rktLocated() {
                t.Skip("Rkt not found; skipping")
        }

        // TODO: use test server to load from a fixture
        task := &structs.Task{
                Name: "linux",
                Config: map[string]string{
                        "trust_prefix": "coreos.com/etcd",
                        "name":  "coreos.com/etcd:v2.0.4",
                },
        }

        driverCtx := testDriverContext(task.Name)
        ctx := testDriverExecContext(task, driverCtx)
        d := NewRktDriver(driverCtx)

        handle, err := d.Start(ctx, task)
        if err != nil {
                t.Fatalf("err: %v", err)
        }
        if handle == nil {
                t.Fatalf("missing handle")
        }

        // Attempt to open
        handle2, err := d.Open(ctx, handle.ID())
        if err != nil {
                t.Fatalf("err: %v", err)
        }
        if handle2 == nil {
                t.Fatalf("missing handle")
        }

        // Clean up
        if err := handle.Kill(); err != nil {
                fmt.Printf("\nError killing Rkt test: %s", err)
        }
}
