// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package exec2

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/nomad/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/shoenig/test/must"
)

func TestExec2(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(1),
	)

	t.Run("testEnv", testEnv)
	t.Run("testSecretsDir", testSecretsDir)
	t.Run("testCountdash", testCountdash)
	t.Run("testHTTP", testHTTP)
}

func testEnv(t *testing.T) {
	job, cleanup := jobs3.Submit(t,
		"./input/env.hcl",
		jobs3.WaitComplete("group"),
	)
	t.Cleanup(cleanup)

	logs := job.TaskLogs("group", "env")

	// ensure the job id lines up
	expect := fmt.Sprintf("NOMAD_JOB_ID=%s", job.JobID())
	must.StrContains(t, logs.Stdout, expect)

	// ensure dynamic user e.g.
	// USER=nomad-85249
	userRe := regexp.MustCompile(`nomad-\d+`)
	must.RegexMatch(t, userRe, logs.Stdout)
}

func testSecretsDir(t *testing.T) {
	job, cleanup := jobs3.Submit(t,
		"./input/secrets.hcl",
		jobs3.WaitComplete("group"),
	)
	t.Cleanup(cleanup)

	// ensure we can read the workload identity token file
	nomadTokenLogs := job.TaskLogs("group", "nomad-token")
	tokenRe := regexp.MustCompile(`[\w_-]+`)
	must.RegexMatch(t, tokenRe, nomadTokenLogs.Stdout)

	// ensure we can read the written password.txt file
	passwordLogs := job.TaskLogs("group", "password")
	must.StrContains(t, passwordLogs.Stdout, "abc123")
}

func testCountdash(t *testing.T) {
	job, cleanup := jobs3.Submit(t,
		"./input/countdash.hcl",
	)
	t.Cleanup(cleanup)

	apiEnvoyLogs := job.TaskLogs("api", "connect-proxy-count-api")
	must.StrContains(t, apiEnvoyLogs.Stderr, "all clusters initialized. initializing init manager")

	dashEnvoyLogs := job.TaskLogs("dashboard", "connect-proxy-count-dashboard")
	must.StrContains(t, dashEnvoyLogs.Stderr, "all clusters initialized. initializing init manager")

	apiLogs := job.TaskLogs("api", "backend")
	must.StrContains(t, apiLogs.Stdout, "Serving at http://localhost:9001")

	dashLogs := job.TaskLogs("dashboard", "dashboard")
	must.StrContains(t, dashLogs.Stdout, "Using counting service at http://127.0.0.1:8080")
}

func testHTTP(t *testing.T) {
	job, _ := jobs3.Submit(t,
		"./input/http.hcl",
		jobs3.DisableCleanup(),
	)

	job2, _ := jobs3.Submit(t,
		"./input/http_curl.hcl",
		jobs3.DisableCleanup(),
	)

	logs := job.TaskLogs("backend", "http")
	must.StrContains(t, logs.Stderr, `"GET / HTTP/1.1" 200 -`)        // healthcheck
	must.StrContains(t, logs.Stderr, `"GET /hi.html HTTP/1.1" 200 -`) // curl

	logs2 := job2.TaskLogs("client", "curl")
	must.StrContains(t, logs2.Stdout, "<body><p>Hello, friend!</p></body>")
}
