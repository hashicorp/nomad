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
			ReservedPorts: []structs.Port{{"main", 12345}},
			DynamicPorts:  []structs.Port{{"HTTP", 43330}},
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
	return
}

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

func testConfig() *config.Config {
	conf := config.DefaultConfig()
	conf.StateDir = os.TempDir()
	conf.AllocDir = os.TempDir()
	conf.MaxKillTimeout = 10 * time.Second
	return conf
}

func testDriverContexts(task *structs.Task) (*DriverContext, *ExecContext) {
	cfg := testConfig()
	allocDir := allocdir.NewAllocDir(filepath.Join(cfg.AllocDir, structs.GenerateUUID()), task.Resources.DiskMB)
	allocDir.Build([]*structs.Task{task})
	alloc := mock.Alloc()
	execCtx := NewExecContext(allocDir, alloc.ID)

	taskEnv, err := GetTaskEnv(allocDir, cfg.Node, task, alloc)
	if err != nil {
		return nil, nil
	}

	driverCtx := NewDriverContext(task.Name, cfg, cfg.Node, testLogger(), taskEnv)
	return driverCtx, execCtx
}

func TestDriver_GetTaskEnv(t *testing.T) {
	task := &structs.Task{
		Name: "Foo",
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
					ReservedPorts: []structs.Port{{"one", 80}, {"two", 443}},
					DynamicPorts:  []structs.Port{{"admin", 8081}, {"web", 8086}},
				},
			},
		},
		Meta: map[string]string{
			"chocolate":  "cake",
			"strawberry": "icecream",
		},
	}

	alloc := mock.Alloc()
	alloc.Name = "Bar"
	env, err := GetTaskEnv(nil, nil, task, alloc)
	if err != nil {
		t.Fatalf("GetTaskEnv() failed: %v", err)
	}
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
		"HELLO":                         "world",
		"lorem":                         "ipsum",
		"NOMAD_ALLOC_ID":                alloc.ID,
		"NOMAD_ALLOC_NAME":              alloc.Name,
		"NOMAD_TASK_NAME":               task.Name,
	}

	act := env.EnvMap()
	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("GetTaskEnv() returned %#v; want %#v", act, exp)
	}
}

func TestMapMergeStrInt(t *testing.T) {
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
