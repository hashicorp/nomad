// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package template

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/taskenv"
	clienttestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

// TestTaskTemplateManager_Permissions tests that we set file permissions
// correctly. This test won't compile on Windows
func TestTaskTemplateManager_Permissions(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireRoot(t)

	// Make a template that will render immediately
	content := "hello, world!"
	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: content,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
		Perms:        "777",
		Uid:          pointer.Of(503),
		Gid:          pointer.Of(20),
	}

	harness := newTestHarness(t, []*structs.Template{template}, false, false)
	harness.start(t)
	defer harness.stop()

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	// Check the file is there
	path := filepath.Join(harness.taskDir, file)
	fi, err := os.Stat(path)
	must.NoError(t, err, must.Sprint("Failed to stat file"))
	must.Eq(t, os.ModePerm, fi.Mode())

	sys := fi.Sys()
	uid := pointer.Of(int(sys.(*syscall.Stat_t).Uid))
	gid := pointer.Of(int(sys.(*syscall.Stat_t).Gid))

	must.Eq(t, template.Uid, uid)
	must.Eq(t, template.Gid, gid)
}

// TestTaskTemplateManager_SymlinkEscapeSource verifies that a malicious or
// compromised task cannot use a symlink parent directory to cause reads to
// escape the sandbox
func TestTaskTemplateManager_SymlinkEscapeSource(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireRoot(t)

	// Create a set of "sensitive" files outside the task dir that the task
	// should not be able to read or write to, despite filesystem permissions
	sensitiveDir := t.TempDir()
	sensitiveFile := filepath.Join(sensitiveDir, "sensitive.txt")
	os.WriteFile(sensitiveFile, []byte("very-secret-stuff"), 0755)

	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Name = TestTaskName
	template := &structs.Template{ChangeMode: structs.TemplateChangeModeNoop}

	// Build a new task environment with a valid DestPath
	harness := newTestHarness(t, []*structs.Template{template}, false, false)
	harness.envBuilder = taskenv.NewBuilder(harness.node, a, task, "global")
	harness.envBuilder.SetClientTaskRoot(harness.taskDir)
	os.MkdirAll(filepath.Join(harness.taskDir, "local"), 0755)
	harness.templates[0].DestPath = filepath.Join("local", "dest.tmpl")

	// "Attack" the SourcePath by creating a symlink from the sensitive file to
	// the task dir; this simulates what happens when the client restarts and
	// the task attacks while the client is down, which is the easiest case to
	// reproduce
	must.NoError(t, os.Symlink(sensitiveDir, filepath.Join(harness.taskDir, "local", "pwned")))
	harness.templates[0].SourcePath = filepath.Join("local", "pwned", "sensitive.txt")

	err := harness.startWithErr()
	t.Cleanup(harness.stop)

	errPath := "/" + filepath.Join((filepath.Base(harness.taskDir)),
		harness.templates[0].SourcePath)

	must.EqError(t, err, fmt.Sprintf("failed to read template: exit status 1: failed to open source file %q: open %s: no such file or directory\n", errPath, errPath))
}

// TestTaskTemplateManager_SymlinkEscapeDest verifies that a malicious or
// compromised task cannot use a symlink parent directory to cause writes to
// escape the sandbox
func TestTaskTemplateManager_SymlinkEscapeDest(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireRoot(t)

	// Create a set of "sensitive" files outside the task dir that the task
	// should not be able to read or write to, despite filesystem permissions
	sensitiveDir := t.TempDir()
	sensitiveFile := filepath.Join(sensitiveDir, "sensitive.txt")
	os.WriteFile(sensitiveFile, []byte("very-secret-stuff"), 0755)

	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Name = TestTaskName
	template := &structs.Template{ChangeMode: structs.TemplateChangeModeNoop}

	// Build a task environment with a valid SourcePath
	harness := newTestHarness(t, []*structs.Template{template}, false, false)
	harness.envBuilder = taskenv.NewBuilder(harness.node, a, task, "global")
	harness.envBuilder.SetClientTaskRoot(harness.taskDir)
	os.MkdirAll(filepath.Join(harness.taskDir, "local"), 0755)

	harness.templates[0].SourcePath = filepath.Join("local", "source.tmpl")
	must.NoError(t, os.WriteFile(
		filepath.Join(harness.taskDir, harness.templates[0].SourcePath),
		[]byte("hacked!"), 0755))

	// "Attack" the DestPath by creating a symlink from the sensitive file to
	// the task dir
	must.NoError(t, os.Symlink(sensitiveDir, filepath.Join(harness.taskDir, "local", "pwned")))
	harness.templates[0].DestPath = filepath.Join("local", "pwned", "sensitive.txt")

	err := harness.startWithErr()
	t.Cleanup(harness.stop)
	must.NoError(t, err)

	// This template has never rendered successfully so we'll get a Kill when we
	// wait for the first render
	select {
	case <-harness.mockHooks.KillCh:
	case <-harness.mockHooks.UnblockCh:
		t.Fatalf("task should not have unblocked")
	case <-time.After(time.Duration(testutil.TestMultiplier()) * time.Second):
		t.Fatalf("task kill should have been called")
	}

	// Ensure we haven't written despite the error
	b, err := os.ReadFile(sensitiveFile)
	must.NoError(t, err)
	must.Eq(t, "very-secret-stuff", string(b))
}
