// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mounter

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func TestRPC(t *testing.T) {

	dir := t.TempDir()
	sockPath := filepath.Join(dir, "mounter.sock")

	srv, err := NewMounterServer(testlog.HCLogger(t), sockPath, "tim")
	must.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	t.Cleanup(cancel)

	go srv.Run(ctx)

	client, err := NewMounterClient(ctx, testlog.HCLogger(t), sockPath)
	must.NoError(t, err)

	args := &MounterBuildAllocReq{
		AllocID:   uuid.Generate(),
		AllocDir:  "/home/tim/tmp/allocs",
		SharedDir: "/home/tim/tmp/mount_allocs",
	}
	var resp MounterBuildAllocResp
	err = RPC(client, MethodBuildAlloc, args, &resp)
	must.NoError(t, err)
}
