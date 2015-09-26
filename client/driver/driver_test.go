package driver

import (
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

var basicResources = &structs.Resources{
	CPU:      250,
	MemoryMB: 256,
	Networks: []*structs.NetworkResource{
		&structs.NetworkResource{
			IP:            "1.2.3.4",
			ReservedPorts: []int{12345},
			DynamicPorts:  []string{"HTTP"},
		},
	},
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
	ctx := NewExecContext(allocDir)
	return ctx
}

func TestDriver_TaskEnvironmentVariables(t *testing.T) {
	ctx := &ExecContext{}
	task := &structs.Task{
		Resources: &structs.Resources{
			CPU:      1000,
			MemoryMB: 500,
			Networks: []*structs.NetworkResource{
				&structs.NetworkResource{
					IP:            "1.2.3.4",
					ReservedPorts: []int{80, 443, 8080, 12345},
					DynamicPorts:  []string{"admin", "5000"},
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
		"NOMAD_PORT_admin":      "8080",
		"NOMAD_PORT_5000":       "12345",
		"NOMAD_META_CHOCOLATE":  "cake",
		"NOMAD_META_STRAWBERRY": "icecream",
		"NOMAD_ALLOC_DIR":       "",
	}

	if !reflect.DeepEqual(env, exp) {
		t.Fatalf("TaskEnvironmentVariables(%#v, %#v) returned %#v; want %#v", ctx, task, env, exp)
	}
}

func TestDriver_PopulateEnvironment(t *testing.T) {
	envVars := map[string]string{"foo": "bar", "BAZ": "baM"}
	act := PopulateEnvironment(envVars)
	sort.Strings(act)
	exp := []string{"foo=bar", "BAZ=baM"}
	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("PopulateEnvironment(%v) returned %v; want %v", envVars, act, exp)
	}

	// Output some debug info to help see what happened.
	if t.Failed() {
		t.Logf("env: %#v", env)
	}
}
