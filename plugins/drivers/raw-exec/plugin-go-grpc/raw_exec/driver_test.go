package raw_exec

import (
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	ndriver "github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/testtask"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

var basicResources = &structs.Resources{
	CPU:      250,
	MemoryMB: 256,
	DiskMB:   20,
}

func init() {
	rand.Seed(49875)
}

func TestMain(m *testing.M) {
	if !testtask.Run() {
		os.Exit(m.Run())
	}
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

func testConfig(t *testing.T) *config.Config {
	conf := config.DefaultConfig()

	// Evaluate the symlinks so that the temp directory resolves correctly on
	// Mac OS.
	d1, err := ioutil.TempDir("", "TestStateDir")
	if err != nil {
		t.Fatal(err)
	}
	d2, err := ioutil.TempDir("", "TestAllocDir")
	if err != nil {
		t.Fatal(err)
	}

	p1, err := filepath.EvalSymlinks(d1)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := filepath.EvalSymlinks(d2)
	if err != nil {
		t.Fatal(err)
	}

	// Give the directories access to everyone
	if err := os.Chmod(p1, 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p2, 0777); err != nil {
		t.Fatal(err)
	}

	conf.StateDir = p1
	conf.AllocDir = p2
	conf.MaxKillTimeout = 10 * time.Second
	conf.Region = "global"
	conf.Node = mock.Node()
	return conf
}

type testContext struct {
	AllocDir   *allocdir.AllocDir
	DriverCtx  *DriverContext
	ExecCtx    *ndriver.ExecContext
	EnvBuilder *env.Builder
}

// setupTaskEnv creates a test env for GetTaskEnv testing. Returns task dir,
// expected env, and actual env.
func setupTaskEnv(t *testing.T, driver string) (*allocdir.TaskDir, map[string]string, map[string]string) {
	task := &structs.Task{
		Name:   "Foo",
		Driver: driver,
		Env: map[string]string{
			"HELLO": "world",
			"lorem": "ipsum",
		},
		Resources: &structs.Resources{
			CPU:      1000,
			MemoryMB: 500,
			Networks: []*structs.NetworkResource{
				{
					IP:            "1.2.3.4",
					ReservedPorts: []structs.Port{{Label: "one", Value: 80}, {Label: "two", Value: 443}},
					DynamicPorts:  []structs.Port{{Label: "admin", Value: 8081}, {Label: "web", Value: 8086}},
				},
			},
		},
		Meta: map[string]string{
			"chocolate":  "cake",
			"strawberry": "icecream",
		},
	}

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Tasks[0] = task
	alloc.Name = "Bar"
	alloc.TaskResources["web"].Networks[0].DynamicPorts[0].Value = 2000
	conf := testConfig(t)
	allocDir := allocdir.NewAllocDir(testlog.Logger(t), filepath.Join(conf.AllocDir, alloc.ID))
	taskDir := allocDir.NewTaskDir(task.Name)
	eb := env.NewBuilder(conf.Node, alloc, task, conf.Region)
	tmpDriver, err := ndriver.NewDriver(driver, ndriver.NewEmptyDriverContext())
	if err != nil {
		t.Fatalf("unable to create driver %q: %v", driver, err)
	}
	SetEnvvars(eb, tmpDriver.FSIsolation(), taskDir, conf)
	exp := map[string]string{
		"NOMAD_CPU_LIMIT":               "1000",
		"NOMAD_MEMORY_LIMIT":            "500",
		"NOMAD_ADDR_one":                "1.2.3.4:80",
		"NOMAD_IP_one":                  "1.2.3.4",
		"NOMAD_PORT_one":                "80",
		"NOMAD_HOST_PORT_one":           "80",
		"NOMAD_ADDR_two":                "1.2.3.4:443",
		"NOMAD_IP_two":                  "1.2.3.4",
		"NOMAD_PORT_two":                "443",
		"NOMAD_HOST_PORT_two":           "443",
		"NOMAD_ADDR_admin":              "1.2.3.4:8081",
		"NOMAD_ADDR_web_admin":          "192.168.0.100:5000",
		"NOMAD_ADDR_web_http":           "192.168.0.100:2000",
		"NOMAD_IP_web_admin":            "192.168.0.100",
		"NOMAD_IP_web_http":             "192.168.0.100",
		"NOMAD_PORT_web_http":           "2000",
		"NOMAD_PORT_web_admin":          "5000",
		"NOMAD_IP_admin":                "1.2.3.4",
		"NOMAD_PORT_admin":              "8081",
		"NOMAD_HOST_PORT_admin":         "8081",
		"NOMAD_ADDR_web":                "1.2.3.4:8086",
		"NOMAD_IP_web":                  "1.2.3.4",
		"NOMAD_PORT_web":                "8086",
		"NOMAD_HOST_PORT_web":           "8086",
		"NOMAD_META_CHOCOLATE":          "cake",
		"NOMAD_META_STRAWBERRY":         "icecream",
		"NOMAD_META_ELB_CHECK_INTERVAL": "30s",
		"NOMAD_META_ELB_CHECK_TYPE":     "http",
		"NOMAD_META_ELB_CHECK_MIN":      "3",
		"NOMAD_META_OWNER":              "armon",
		"NOMAD_META_chocolate":          "cake",
		"NOMAD_META_strawberry":         "icecream",
		"NOMAD_META_elb_check_interval": "30s",
		"NOMAD_META_elb_check_type":     "http",
		"NOMAD_META_elb_check_min":      "3",
		"NOMAD_META_owner":              "armon",
		"HELLO":                         "world",
		"lorem":                         "ipsum",
		"NOMAD_ALLOC_ID":                alloc.ID,
		"NOMAD_ALLOC_INDEX":             "0",
		"NOMAD_ALLOC_NAME":              alloc.Name,
		"NOMAD_TASK_NAME":               task.Name,
		"NOMAD_GROUP_NAME":              alloc.TaskGroup,
		"NOMAD_JOB_NAME":                alloc.Job.Name,
		"NOMAD_DC":                      "dc1",
		"NOMAD_REGION":                  "global",
	}

	act := eb.Build().Map()
	return taskDir, exp, act
}

// testDriverContext sets up an alloc dir, task dir, DriverContext, and ExecContext.
//
// It is up to the caller to call AllocDir.Destroy to cleanup.
func testDriverContexts(t *testing.T, task *structs.Task) *testContext {
	cfg := testConfig(t)
	cfg.Node = mock.Node()
	alloc := mock.Alloc()
	alloc.NodeID = cfg.Node.ID

	allocDir := allocdir.NewAllocDir(testlog.Logger(t), filepath.Join(cfg.AllocDir, alloc.ID))
	if err := allocDir.Build(); err != nil {
		t.Fatalf("AllocDir.Build() failed: %v", err)
	}

	// Build a temp driver so we can call FSIsolation and build the task dir
	tmpdrv, err := ndriver.NewDriver(task.Driver, ndriver.NewEmptyDriverContext())
	if err != nil {
		allocDir.Destroy()
		t.Fatalf("NewDriver(%q, nil) failed: %v", task.Driver, err)
		return nil
	}

	// Build the task dir
	td := allocDir.NewTaskDir(task.Name)
	if err := td.Build(false, config.DefaultChrootEnv, tmpdrv.FSIsolation()); err != nil {
		allocDir.Destroy()
		t.Fatalf("TaskDir.Build(%#v, %q) failed: %v", config.DefaultChrootEnv, tmpdrv.FSIsolation(), err)
		return nil
	}
	eb := env.NewBuilder(cfg.Node, alloc, task, cfg.Region)
	SetEnvvars(eb, tmpdrv.FSIsolation(), td, cfg)
	execCtx := ndriver.NewExecContext(td, eb.Build())

	logger := testlog.Logger(t)
	emitter := func(m string, args ...interface{}) {
		logger.Printf("[EVENT] "+m, args...)
	}
	driverCtx := NewDriverContext(alloc.Job.Name, alloc.TaskGroup, task.Name, alloc.ID, cfg, cfg.Node, logger, emitter)

	return &testContext{allocDir, driverCtx, execCtx, eb}
}
