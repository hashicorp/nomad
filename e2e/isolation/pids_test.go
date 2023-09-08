// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package isolation

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/shoenig/test/must"
)

func TestPIDs(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(1),
	)

	// exec driver
	t.Run("testExecNamespacePID", testExecNamespacePID)
	t.Run("testExecHostPID", testExecHostPID)
	t.Run("testExecNamespaceAllocExec", testExecNamespaceAllocExec)

	// java driver
	t.Run("testJavaNamespacePID", testJavaNamespacePID)
	t.Run("testJavaHostPID", testJavaHostPID)
	t.Run("testJavaNamespaceAllocExec", testJavaNamespaceAllocExec)

	// raw_exec driver
	t.Run("testRawExecNoNamespacePID", testRawExecNoNamespacePID)
}

var (
	pidRe = regexp.MustCompile(`my pid is (\d+)`)
)

func testExecNamespacePID(t *testing.T) {
	job, cleanup := jobs3.Submit(t,
		"./input/exec.hcl",
		jobs3.WaitComplete("group"),
	)
	t.Cleanup(cleanup)

	logs := job.TaskLogs("group", "bash")
	must.StrContains(t, logs.Stdout, "my pid is 1")
}

func testExecHostPID(t *testing.T) {
	job, cleanup := jobs3.Submit(t,
		"./input/exec_host.hcl",
		jobs3.WaitComplete("group"),
	)
	t.Cleanup(cleanup)

	logs := job.TaskLogs("group", "bash")
	subs := pidRe.FindStringSubmatch(logs.Stdout)
	must.SliceLen(t, 2, subs)
	must.NotEq(t, "1", subs[1], must.Sprint("expected any pid other than 1"))
}

func testExecNamespaceAllocExec(t *testing.T) {
	job, cleanup := jobs3.Submit(t, "./input/alloc_exec.hcl")
	t.Cleanup(cleanup)

	logs := job.Exec("group", "sleep", []string{"ps", "ax"})
	lines := strings.Split(strings.TrimSpace(logs.Stdout), "\n")

	// header, sleep, ps (and nothing else)
	must.SliceLen(t, 3, lines, must.Sprintf("expected 3 lines of output, got %s", lines))
}

func testJavaNamespacePID(t *testing.T) {
	job, cleanup := jobs3.Submit(t,
		"./input/java.hcl",
		jobs3.WaitComplete("group"),
	)
	t.Cleanup(cleanup)

	logs := job.TaskLogs("group", "java")
	must.StrContains(t, logs.Stdout, "my pid is 1")
}

func testJavaHostPID(t *testing.T) {
	job, cleanup := jobs3.Submit(t,
		"./input/java_host.hcl",
		jobs3.WaitComplete("group"),
	)
	t.Cleanup(cleanup)

	logs := job.TaskLogs("group", "java")
	subs := pidRe.FindStringSubmatch(logs.Stdout)
	must.SliceLen(t, 2, subs)
	must.NotEq(t, "1", subs[1], must.Sprint("expected any pid other than 1"))
}

func testJavaNamespaceAllocExec(t *testing.T) {
	job, cleanup := jobs3.Submit(t, "./input/alloc_exec_java.hcl")
	t.Cleanup(cleanup)

	logs := job.Exec("group", "sleep", []string{"ps", "ax"})
	lines := strings.Split(strings.TrimSpace(logs.Stdout), "\n")

	// header, java, ps (and nothing else)
	must.SliceLen(t, 3, lines, must.Sprintf("expected 3 lines of output, got %s", lines))
}

func testRawExecNoNamespacePID(t *testing.T) {
	job, cleanup := jobs3.Submit(t, "./input/raw_exec.hcl")
	t.Cleanup(cleanup)

	logs := job.TaskLogs("group", "bash")
	subs := pidRe.FindStringSubmatch(logs.Stdout)
	must.SliceLen(t, 2, subs)
	must.NotEq(t, "1", subs[1], must.Sprint("expected any pid other than 1"))
}
