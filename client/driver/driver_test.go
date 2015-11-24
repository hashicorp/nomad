package driver

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"

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

func testDriverContext(task string) *DriverContext {
	cfg := testConfig()
	return NewDriverContext(task, cfg, cfg.Node, testLogger())
}

func testDriverExecContext(task *structs.Task, driverCtx *DriverContext) *ExecContext {
	allocDir := allocdir.NewAllocDir(filepath.Join(driverCtx.config.AllocDir, structs.GenerateUUID()))
	allocDir.Build([]*structs.Task{task})
	ctx := NewExecContext(allocDir, fmt.Sprintf("alloc-id-%d", int(rand.Int31())))
	return ctx
}

func TestDriver_TaskEnvironmentVariables(t *testing.T) {
	t.Parallel()
	ctx := &ExecContext{}
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

	env := TaskEnvironmentVariables(ctx, task)
	exp := map[string]string{
		"NOMAD_CPU_LIMIT":       "1000",
		"NOMAD_MEMORY_LIMIT":    "500",
		"NOMAD_IP":              "1.2.3.4",
		"NOMAD_PORT_one":        "80",
		"NOMAD_PORT_two":        "443",
		"NOMAD_PORT_three":      "8080",
		"NOMAD_PORT_four":       "12345",
		"NOMAD_PORT_admin":      "8081",
		"NOMAD_PORT_web":        "8086",
		"NOMAD_META_CHOCOLATE":  "cake",
		"NOMAD_META_STRAWBERRY": "icecream",
		"HELLO":                 "world",
		"lorem":                 "ipsum",
	}

	act := env.Map()
	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("TaskEnvironmentVariables(%#v, %#v) returned %#v; want %#v", ctx, task, act, exp)
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
