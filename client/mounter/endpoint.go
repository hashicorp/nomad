// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mounter

import (
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/plugins/drivers/fsisolation"
)

type MounterEndpoint struct {
	srv   *MounterServer
	inner *allocdir.DefaultBuilder
}

// MounterEndpoint RPC method names
const (
	MethodPing         = "MounterEndpoint.Ping"
	MethodBuildAlloc   = "MounterEndpoint.BuildAlloc"
	MethodDestroyAlloc = "MounterEndpoint.DestroyAlloc"
	MethodMoveAlloc    = "MounterEndpoint.MoveAlloc"
	MethodBuildTask    = "MounterEndpoint.BuildTask"
	MethodDestroyTask  = "MounterEndpoint.DestroyTask"
)

// MounterResp is a generic RPC response
type MounterResp struct {
	// TODO: nothing to return from any of these?
}

type MounterPingReq struct{}

func (m *MounterEndpoint) Ping(args *MounterPingReq, reply *MounterResp) error {
	m.srv.log.Info("new connection from agent")
	return nil
}

type MounterBuildAllocReq struct {
	AllocDir *allocdir.AllocDir
}

func (m *MounterEndpoint) BuildAlloc(args *MounterBuildAllocReq, reply *MounterResp) error {
	m.srv.log.Info("building allocdir",
		"alloc_dir", args.AllocDir.AllocDir, "alloc_id", args.AllocDir.AllocID)
	err := m.inner.Build(args.AllocDir)
	if err != nil {
		m.srv.log.Error("failed to build allocdir",
			"alloc_dir", args.AllocDir.AllocDir, "alloc_id", args.AllocDir.AllocID, "error", err)
	}

	return err
}

type MounterDestroyAllocReq struct {
	AllocDir *allocdir.AllocDir
}

func (m *MounterEndpoint) DestroyAlloc(args *MounterDestroyAllocReq, reply *MounterResp) error {
	m.srv.log.Info("destroying allocdir",
		"alloc_dir", args.AllocDir.AllocDir, "alloc_id", args.AllocDir.AllocID)
	err := m.inner.Destroy(args.AllocDir)
	if err != nil {
		m.srv.log.Error("failed to allocdir",
			"alloc_dir", args.AllocDir.AllocDir, "alloc_id", args.AllocDir.AllocID, "error", err)
	}
	return err
}

type MounterMoveAllocReq struct {
	OldAllocDir *allocdir.AllocDir
	NewAllocDir *allocdir.AllocDir
	Tasks       []string
}

func (m *MounterEndpoint) MoveAlloc(args *MounterMoveAllocReq, reply *MounterResp) error {
	m.srv.log.Info("moving allocdir",
		"old_alloc_dir", args.OldAllocDir.AllocDir,
		"new_alloc_dir", args.NewAllocDir.AllocDir,
		"alloc_id", args.OldAllocDir.AllocID)
	err := m.inner.Move(args.OldAllocDir, args.NewAllocDir, args.Tasks)
	if err != nil {
		m.srv.log.Error("failed to move allocdir",
			"old_alloc_dir", args.OldAllocDir.AllocDir,
			"new_alloc_dir", args.NewAllocDir.AllocDir,
			"alloc_id", args.OldAllocDir.AllocID,
			"error", err,
		)
	}
	return err
}

type MounterBuildTaskReq struct {
	TaskDir *allocdir.TaskDir
	FSI     fsisolation.Mode
	Chroot  map[string]string
	User    string
}

func (m *MounterEndpoint) BuildTask(args *MounterBuildTaskReq, reply *MounterResp) error {
	m.srv.log.Info("building task dir", "dir", args.TaskDir.Dir)
	err := m.inner.BuildTaskDir(args.TaskDir, args.FSI, args.Chroot, args.User)
	if err != nil {
		m.srv.log.Error("failed build task dir", "dir", args.TaskDir.Dir, "error", err)
	}
	return err
}

type MounterDestroyTaskReq struct {
	TaskDir *allocdir.TaskDir
}

func (m *MounterEndpoint) DestroyTask(args *MounterDestroyTaskReq, reply *MounterResp) error {
	m.srv.log.Info("unmounting task dir", "dir", args.TaskDir.Dir)
	err := m.inner.UnmountTaskDir(args.TaskDir)
	if err != nil {
		m.srv.log.Error("failed to unmount task dir", "dir", args.TaskDir.Dir, "error", err)
	}
	return err
}
