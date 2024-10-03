// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux
// +build !linux

package allocdir

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers/fsisolation"
	"github.com/shoenig/test/must"
)

func TestAllocDir_ReadAt_CaseInsensitiveSecretDir(t *testing.T) {
	ci.Parallel(t)

	// On macOS, os.TempDir returns a symlinked path under /var which
	// is outside of the directories shared into the VM used for Docker.
	// Expand the symlink to get the real path in /private, which is ok.
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	must.NoError(t, err)

	d := NewAllocDir(testlog.HCLogger(t), tmp, tmp, "test")
	must.NoError(t, d.Build())
	defer func() { _ = d.Destroy() }()

	td := d.NewTaskDir(t1Windows)
	must.NoError(t, td.Build(fsisolation.None, nil, "nobody"))

	target := filepath.Join(t1Windows.Name, TaskSecrets, "test_file")

	full := filepath.Join(d.AllocDir, target)
	must.NoError(t, os.WriteFile(full, []byte("hi"), 0o600))

	targetCaseInsensitive := filepath.Join(t1Windows.Name, "sEcReTs", "test_file")

	_, err = d.ReadAt(targetCaseInsensitive, 0)
	must.EqError(t, err, "Reading secret file prohibited: "+targetCaseInsensitive)
}

var (
	t1Windows = &structs.Task{
		Name:   "web",
		Driver: "exec",
		Config: map[string]interface{}{
			"command": "/bin/date",
			"args":    "+%s",
		},
		Resources: &structs.Resources{
			DiskMB: 1,
		},
	}
)
