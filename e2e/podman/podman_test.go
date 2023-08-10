// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package podman

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/shoenig/test/must"
)

const (
	registryService = "registry"
)

func TestPodman(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(1),
	)

	runRegistry(t)

	t.Run("testRedis", testRedis)
	t.Run("testAuthBasic", testAuthBasic)
	t.Run("testAuthFileStatic", testAuthFileStatic)
	t.Run("testAuthHelper", testAuthHelper)
}

func findService(t *testing.T, name string) (string, int) {
	services, _, err := e2eutil.NomadClient(t).Services().Get(name, nil)
	must.NoError(t, err, must.Sprintf("failed to find %q service", name))
	must.Len(t, 1, services, must.Sprintf("expected 1 %q service", name))
	return services[0].Address, services[0].Port
}

func runRegistry(t *testing.T) {
	_, regCleanup := jobs3.Submit(t,
		"./input/registry.hcl",
		jobs3.Timeout(40*time.Second), // pulls an image
	)
	t.Cleanup(regCleanup)

	// lookup registry address
	addr, port := findService(t, registryService)
	address := fmt.Sprintf("%s:%d", addr, port)

	// run the sed job to fixup the auth.json file with correct address
	_, sedCleanup := jobs3.Submit(t,
		"./input/registry-auths.hcl",
		jobs3.Var("registry_address", address),
		jobs3.Var("user", "root"),
		jobs3.Var("helper_dir", "/usr/local/bin"),
		jobs3.Var("auth_dir", "/etc"),
		jobs3.WaitComplete("create-files"),
		jobs3.Timeout(20*time.Second),
	)
	t.Cleanup(sedCleanup)
}

func testRedis(t *testing.T) {
	job, cleanup := jobs3.Submit(t, "./input/redis.hcl")
	t.Cleanup(cleanup)

	logs := job.TaskLogs("cache", "redis")
	must.StrContains(t, logs.Stdout, "oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo")
}

func testAuthBasic(t *testing.T) {
	// find the private registry service
	regAddr, regPort := findService(t, "registry")

	// run the private bash image
	bashJob, bashCleanup := jobs3.Submit(t, "./input/auth_basic.hcl",
		jobs3.Var("registry_address", regAddr),
		jobs3.Var("registry_port", strconv.Itoa(regPort)),
		jobs3.WaitComplete("basic"),
	)
	t.Cleanup(bashCleanup)
	logs := bashJob.TaskLogs("basic", "echo")
	must.StrContains(t, logs.Stdout, "The auth basic test is OK!")
}

func testAuthFileStatic(t *testing.T) {
	// find the private registry service
	regAddr, regPort := findService(t, "registry")

	// run the private _static bash image
	bashJob, bashCleanup := jobs3.Submit(t, "./input/auth_static.hcl",
		jobs3.Var("registry_address", regAddr),
		jobs3.Var("registry_port", strconv.Itoa(regPort)),
		jobs3.WaitComplete("static"),
	)
	t.Cleanup(bashCleanup)
	logs := bashJob.TaskLogs("static", "echo")
	must.StrContains(t, logs.Stdout, "The static auth test is OK!")
}

func testAuthHelper(t *testing.T) {
	// find the private registry service
	regAddr, regPort := findService(t, "registry")

	t.Log("registry", regAddr, regPort)

	// run the private _helper bash image
	bashJob, bashCleanup := jobs3.Submit(t, "./input/auth_helper.hcl",
		jobs3.Var("registry_address", regAddr),
		jobs3.Var("registry_port", strconv.Itoa(regPort)),
		jobs3.WaitComplete("helper"),
	)
	t.Cleanup(bashCleanup)
	logs := bashJob.TaskLogs("helper", "echo")
	must.StrContains(t, logs.Stdout, "The credentials helper auth test is OK!")
}
