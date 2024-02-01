// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package template

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
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
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	if m := fi.Mode(); m != os.ModePerm {
		t.Fatalf("Got mode %v; want %v", m, os.ModePerm)
	}

	sys := fi.Sys()
	uid := pointer.Of(int(sys.(*syscall.Stat_t).Uid))
	gid := pointer.Of(int(sys.(*syscall.Stat_t).Gid))

	must.Eq(t, template.Uid, uid)
	must.Eq(t, template.Gid, gid)
}
