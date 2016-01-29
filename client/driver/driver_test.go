package driver

import (
	"fmt"
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
	"github.com/hashicorp/nomad/nomad/structs"
)

var basicResources = &structs.Resources{
	CPU:      250,
	MemoryMB: 256,
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

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

func testConfig() *config.Config {
	conf := &config.Config{}
	conf.StateDir = os.TempDir()
	conf.AllocDir = os.TempDir()
	return conf
}

func testDriverContexts(task *structs.Task) (*DriverContext, *ExecContext) {
	cfg := testConfig()
	allocDir := allocdir.NewAllocDir(filepath.Join(cfg.AllocDir, structs.GenerateUUID()))
	allocDir.Build([]*structs.Task{task})
	execCtx := NewExecContext(allocDir, fmt.Sprintf("alloc-id-%d", int(rand.Int31())))

	taskEnv, err := GetTaskEnv(allocDir, cfg.Node, task)
	if err != nil {
		return nil, nil
	}

	driverCtx := NewDriverContext(task.Name, cfg, cfg.Node, testLogger(), taskEnv)
	return driverCtx, execCtx
}

func TestDriver_KillTimeout(t *testing.T) {
	expected := 1 * time.Second
	task := &structs.Task{Name: "foo", KillTimeout: expected}
	ctx, _ := testDriverContexts(task)
	ctx.config.MaxKillTimeout = 10 * time.Second

	if actual := ctx.KillTimeout(task); expected != actual {
		t.Fatalf("KillTimeout(%v) returned %v; want %v", task, actual, expected)
	}

	expected = 10 * time.Second
	task = &structs.Task{KillTimeout: 11 * time.Second}

	if actual := ctx.KillTimeout(task); expected != actual {
		t.Fatalf("KillTimeout(%v) returned %v; want %v", task, actual, expected)
	}
}

func TestDriver_GetTaskEnv(t *testing.T) {
	t.Parallel()
	task := &structs.Task{
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
					ReservedPorts: []structs.Port{{"one", 80}, {"two", 443}, {"three", 8080}, {"four", 12345}},
					DynamicPorts:  []structs.Port{{"admin", 8081}, {"web", 8086}},
				},
			},
		},
		Meta: map[string]string{
			"chocolate":  "cake",
			"strawberry": "icecream",
		},
	}

	env, err := GetTaskEnv(nil, nil, task)
	if err != nil {
		t.Fatalf("GetTaskEnv() failed: %v", err)
	}
	exp := map[string]string{
		"NOMAD_CPU_LIMIT":       "1000",
		"NOMAD_MEMORY_LIMIT":    "500",
		"NOMAD_ADDR_one":        "1.2.3.4:80",
		"NOMAD_ADDR_two":        "1.2.3.4:443",
		"NOMAD_ADDR_three":      "1.2.3.4:8080",
		"NOMAD_ADDR_four":       "1.2.3.4:12345",
		"NOMAD_ADDR_admin":      "1.2.3.4:8081",
		"NOMAD_ADDR_web":        "1.2.3.4:8086",
		"NOMAD_META_CHOCOLATE":  "cake",
		"NOMAD_META_STRAWBERRY": "icecream",
		"HELLO":                 "world",
		"lorem":                 "ipsum",
	}

	act := env.EnvMap()
	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("GetTaskEnv() returned %#v; want %#v", act, exp)
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
