// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mounter

import (
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/plugins/drivers/fsisolation"
)

// MounterShim allows creating, destroying, and accessing an allocation's
// directory. It implements allocdir.Builder but sends requests to an external
// process.
type MounterShim struct {
	client *MounterClient
}

func NewMounterShim(client *MounterClient) allocdir.Builder {
	return &MounterShim{
		client: client,
	}
}

func (m *MounterShim) Build(d *allocdir.AllocDir) error {
	args := &MounterBuildAllocReq{
		AllocDir: d,
	}
	var resp MounterResp
	return RPC(m.client, MethodBuildAlloc, args, &resp)
}

func (m *MounterShim) Destroy(d *allocdir.AllocDir) error {
	args := &MounterDestroyAllocReq{
		AllocDir: d,
	}
	var resp MounterResp
	return RPC(m.client, MethodDestroyAlloc, args, &resp)
}

func (m *MounterShim) Move(oldDir *allocdir.AllocDir, newDir *allocdir.AllocDir, tasks []string) error {
	args := &MounterMoveAllocReq{
		OldAllocDir: oldDir,
		NewAllocDir: newDir,
		Tasks:       tasks,
	}
	var resp MounterResp
	return RPC(m.client, MethodMoveAlloc, args, &resp)
}

func (m *MounterShim) BuildTaskDir(taskDir *allocdir.TaskDir, fsi fsisolation.Mode, chroot map[string]string, user string) error {
	args := &MounterBuildTaskReq{
		TaskDir: taskDir,
		FSI:     fsi,
		Chroot:  chroot,
		User:    user,
	}
	var resp MounterResp
	return RPC(m.client, MethodBuildTask, args, &resp)
}

func (m *MounterShim) UnmountTaskDir(taskDir *allocdir.TaskDir) error {
	args := &MounterDestroyTaskReq{
		TaskDir: taskDir,
	}
	var resp MounterResp
	return RPC(m.client, MethodDestroyTask, args, &resp)
}
