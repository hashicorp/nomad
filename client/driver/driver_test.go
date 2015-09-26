package driver

import (
	"log"
	"os"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

func testConfig() *config.Config {
	return &config.Config{}
}

func testDriverContext() *DriverContext {
	cfg := testConfig()
	ctx := NewDriverContext(cfg, cfg.Node, testLogger())
	return ctx
}

func contains(l []string, s string) bool {
	for _, item := range l {
		if item == s {
			return true
		}
	}
	return false
}

func TestPopulateEnvironment(t *testing.T) {
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

	env := PopulateEnvironment(ctx, task)

	// Resources
	cpu := "NOMAD_CPU_LIMIT=1000"
	if !contains(env, cpu) {
		t.Errorf("%s is missing from env", cpu)
	}
	memory := "NOMAD_MEMORY_LIMIT=500"
	if !contains(env, memory) {
		t.Errorf("%s is missing from env", memory)
	}

	// Networking
	ip := "NOMAD_IP=1.2.3.4"
	if !contains(env, ip) {
		t.Errorf("%s is missing from env", ip)
	}
	labelport := "NOMAD_PORT_ADMIN=8080"
	if !contains(env, labelport) {
		t.Errorf("%s is missing from env", labelport)
	}
	numberport := "NOMAD_PORT_5000=12345"
	if !contains(env, numberport) {
		t.Errorf("%s is missing from env", numberport)
	}

	// Metas
	chocolate := "NOMAD_META_CHOCOLATE=cake"
	if !contains(env, chocolate) {
		t.Errorf("%s is missing from env", chocolate)
	}
	strawberry := "NOMAD_META_STRAWBERRY=icecream"
	if !contains(env, strawberry) {
		t.Errorf("%s is missing from env", strawberry)
	}

	// Output some debug info to help see what happened.
	if t.Failed() {
		t.Logf("env: %#v", env)
	}
}
