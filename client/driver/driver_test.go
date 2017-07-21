package driver

import (
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/helper/testtask"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

var basicResources = &structs.Resources{
	CPU:      250,
	MemoryMB: 256,
	DiskMB:   20,
	Networks: []*structs.NetworkResource{
		&structs.NetworkResource{
			IP:            "0.0.0.0",
			ReservedPorts: []structs.Port{{Label: "main", Value: 12345}},
			DynamicPorts:  []structs.Port{{Label: "HTTP", Value: 43330}},
		},
	},
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

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

func testConfig() *config.Config {
	conf := config.DefaultConfig()
	conf.StateDir = os.TempDir()
	conf.AllocDir = os.TempDir()
	conf.MaxKillTimeout = 10 * time.Second
	conf.Region = "global"
	conf.Node = mock.Node()
	return conf
}

type testContext struct {
	AllocDir   *allocdir.AllocDir
	DriverCtx  *DriverContext
	ExecCtx    *ExecContext
	EnvBuilder *env.Builder
}

// testDriverContext sets up an alloc dir, task dir, DriverContext, and ExecContext.
//
// It is up to the caller to call AllocDir.Destroy to cleanup.
func testDriverContexts(t *testing.T, task *structs.Task) *testContext {
	cfg := testConfig()
	cfg.Node = mock.Node()
	allocDir := allocdir.NewAllocDir(testLogger(), filepath.Join(cfg.AllocDir, structs.GenerateUUID()))
	if err := allocDir.Build(); err != nil {
		t.Fatalf("AllocDir.Build() failed: %v", err)
	}
	alloc := mock.Alloc()

	// Build a temp driver so we can call FSIsolation and build the task dir
	tmpdrv, err := NewDriver(task.Driver, NewEmptyDriverContext())
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
	execCtx := NewExecContext(td, eb.Build())

	logger := testLogger()
	emitter := func(m string, args ...interface{}) {
		logger.Printf("[EVENT] "+m, args...)
	}
	driverCtx := NewDriverContext(task.Name, alloc.ID, cfg, cfg.Node, logger, emitter)

	return &testContext{allocDir, driverCtx, execCtx, eb}
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
				&structs.NetworkResource{
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
	conf := testConfig()
	allocDir := allocdir.NewAllocDir(testLogger(), filepath.Join(conf.AllocDir, alloc.ID))
	taskDir := allocDir.NewTaskDir(task.Name)
	eb := env.NewBuilder(conf.Node, alloc, task, conf.Region)
	tmpDriver, err := NewDriver(driver, NewEmptyDriverContext())
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
		"NOMAD_ADDR_web_main":           "192.168.0.100:5000",
		"NOMAD_ADDR_web_http":           "192.168.0.100:2000",
		"NOMAD_IP_web_main":             "192.168.0.100",
		"NOMAD_IP_web_http":             "192.168.0.100",
		"NOMAD_PORT_web_http":           "2000",
		"NOMAD_PORT_web_main":           "5000",
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

func TestDriver_GetTaskEnv_None(t *testing.T) {
	t.Parallel()
	taskDir, exp, act := setupTaskEnv(t, "raw_exec")

	// raw_exec should use host alloc dir path
	exp[env.AllocDir] = taskDir.SharedAllocDir
	exp[env.TaskLocalDir] = taskDir.LocalDir
	exp[env.SecretsDir] = taskDir.SecretsDir

	// Since host env vars are included only ensure expected env vars are present
	for expk, expv := range exp {
		v, ok := act[expk]
		if !ok {
			t.Errorf("%q not found in task env", expk)
			continue
		}
		if v != expv {
			t.Errorf("Expected %s=%q but found %q", expk, expv, v)
		}
	}

	// Make sure common host env vars are included.
	for _, envvar := range [...]string{"PATH", "HOME", "USER"} {
		if exp := os.Getenv(envvar); act[envvar] != exp {
			t.Errorf("Expected envvar %s=%q  !=  %q", envvar, exp, act[envvar])
		}
	}
}

func TestDriver_GetTaskEnv_Chroot(t *testing.T) {
	t.Parallel()
	_, exp, act := setupTaskEnv(t, "exec")

	exp[env.AllocDir] = allocdir.SharedAllocContainerPath
	exp[env.TaskLocalDir] = allocdir.TaskLocalContainerPath
	exp[env.SecretsDir] = allocdir.TaskSecretsContainerPath

	// Since host env vars are included only ensure expected env vars are present
	for expk, expv := range exp {
		v, ok := act[expk]
		if !ok {
			t.Errorf("%q not found in task env", expk)
			continue
		}
		if v != expv {
			t.Errorf("Expected %s=%q but found %q", expk, expv, v)
		}
	}

	// Make sure common host env vars are included.
	for _, envvar := range [...]string{"PATH", "HOME", "USER"} {
		if exp := os.Getenv(envvar); act[envvar] != exp {
			t.Errorf("Expected envvar %s=%q  !=  %q", envvar, exp, act[envvar])
		}
	}
}

// TestDriver_TaskEnv_Image ensures host environment variables are not set
// for image based drivers. See #2211
func TestDriver_TaskEnv_Image(t *testing.T) {
	t.Parallel()
	_, exp, act := setupTaskEnv(t, "docker")

	exp[env.AllocDir] = allocdir.SharedAllocContainerPath
	exp[env.TaskLocalDir] = allocdir.TaskLocalContainerPath
	exp[env.SecretsDir] = allocdir.TaskSecretsContainerPath

	// Since host env vars are excluded expected and actual maps should be equal
	for expk, expv := range exp {
		v, ok := act[expk]
		delete(act, expk)
		if !ok {
			t.Errorf("Env var %s missing. Expected %s=%q", expk, expk, expv)
			continue
		}
		if v != expv {
			t.Errorf("Env var %s=%q -- Expected %q", expk, v, expk)
		}
	}
	// Any remaining env vars are unexpected
	for actk, actv := range act {
		t.Errorf("Env var %s=%q is unexpected", actk, actv)
	}
}

func TestMapMergeStrInt(t *testing.T) {
	t.Parallel()
	a := map[string]int{
		"cakes":   5,
		"cookies": 3,
	}

	b := map[string]int{
		"cakes": 3,
		"pies":  2,
	}

	c := mapMergeStrInt(a, b)

	d := map[string]int{
		"cakes":   3,
		"cookies": 3,
		"pies":    2,
	}

	if !reflect.DeepEqual(c, d) {
		t.Errorf("\nExpected\n%+v\nGot\n%+v\n", d, c)
	}
}

func TestMapMergeStrStr(t *testing.T) {
	t.Parallel()
	a := map[string]string{
		"cake":   "chocolate",
		"cookie": "caramel",
	}

	b := map[string]string{
		"cake": "strawberry",
		"pie":  "apple",
	}

	c := mapMergeStrStr(a, b)

	d := map[string]string{
		"cake":   "strawberry",
		"cookie": "caramel",
		"pie":    "apple",
	}

	if !reflect.DeepEqual(c, d) {
		t.Errorf("\nExpected\n%+v\nGot\n%+v\n", d, c)
	}
}

func TestCreatedResources_AddMerge(t *testing.T) {
	t.Parallel()
	res1 := NewCreatedResources()
	res1.Add("k1", "v1")
	res1.Add("k1", "v2")
	res1.Add("k1", "v1")
	res1.Add("k2", "v1")

	expected := map[string][]string{
		"k1": {"v1", "v2"},
		"k2": {"v1"},
	}
	if !reflect.DeepEqual(expected, res1.Resources) {
		t.Fatalf("1.  %#v != expected %#v", res1.Resources, expected)
	}

	// Make sure merging nil works
	var res2 *CreatedResources
	res1.Merge(res2)
	if !reflect.DeepEqual(expected, res1.Resources) {
		t.Fatalf("2.  %#v != expected %#v", res1.Resources, expected)
	}

	// Make sure a normal merge works
	res2 = NewCreatedResources()
	res2.Add("k1", "v3")
	res2.Add("k2", "v1")
	res2.Add("k3", "v3")
	res1.Merge(res2)

	expected = map[string][]string{
		"k1": {"v1", "v2", "v3"},
		"k2": {"v1"},
		"k3": {"v3"},
	}
	if !reflect.DeepEqual(expected, res1.Resources) {
		t.Fatalf("3.  %#v != expected %#v", res1.Resources, expected)
	}
}

func TestCreatedResources_CopyRemove(t *testing.T) {
	t.Parallel()
	res1 := NewCreatedResources()
	res1.Add("k1", "v1")
	res1.Add("k1", "v2")
	res1.Add("k1", "v3")
	res1.Add("k2", "v1")

	// Assert Copy creates a deep copy
	res2 := res1.Copy()

	if !reflect.DeepEqual(res1, res2) {
		t.Fatalf("%#v != %#v", res1, res2)
	}

	// Assert removing v1 from k1 returns true and updates Resources slice
	if removed := res2.Remove("k1", "v1"); !removed {
		t.Fatalf("expected v1 to be removed: %#v", res2)
	}

	if expected := []string{"v2", "v3"}; !reflect.DeepEqual(expected, res2.Resources["k1"]) {
		t.Fatalf("unpexpected list for k1: %#v", res2.Resources["k1"])
	}

	// Assert removing the only value from a key removes the key
	if removed := res2.Remove("k2", "v1"); !removed {
		t.Fatalf("expected v1 to be removed from k2: %#v", res2.Resources)
	}

	if _, found := res2.Resources["k2"]; found {
		t.Fatalf("k2 should have been removed from Resources: %#v", res2.Resources)
	}

	// Make sure res1 wasn't updated
	if reflect.DeepEqual(res1, res2) {
		t.Fatalf("res1 should not equal res2: #%v", res1)
	}
}
