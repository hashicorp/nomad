// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"time"

	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	"github.com/hashicorp/nomad/nomad/structs"
)

// State captures the state of the allocation runner.
type State struct {
	// ClientStatus captures the overall state of the allocation
	ClientStatus string

	// ClientDescription is an optional human readable description of the
	// allocations client state
	ClientDescription string

	// DeploymentStatus captures the status of the deployment
	DeploymentStatus *structs.AllocDeploymentStatus

	// TaskStates is a snapshot of task states.
	TaskStates map[string]*structs.TaskState

	// NetworkStatus captures network details not known until runtime
	NetworkStatus *structs.AllocNetworkStatus
}

// SetDeploymentStatus is a helper for updating the client-controlled
// DeploymentStatus fields: Healthy and Timestamp. The Canary and ModifyIndex
// fields should only be updated by the server.
func (s *State) SetDeploymentStatus(timestamp time.Time, healthy bool) {
	if s.DeploymentStatus == nil {
		s.DeploymentStatus = &structs.AllocDeploymentStatus{}
	}

	s.DeploymentStatus.Healthy = &healthy
	s.DeploymentStatus.Timestamp = timestamp
}

// ClearDeploymentStatus is a helper to clear the client-controlled
// DeploymentStatus fields: Healthy and Timestamp. The Canary and ModifyIndex
// fields should only be updated by the server.
func (s *State) ClearDeploymentStatus() {
	if s.DeploymentStatus == nil {
		return
	}

	s.DeploymentStatus.Healthy = nil
	s.DeploymentStatus.Timestamp = time.Time{}
}

// Copy returns a deep copy of State.
func (s *State) Copy() *State {
	taskStates := make(map[string]*structs.TaskState, len(s.TaskStates))
	for k, v := range s.TaskStates {
		taskStates[k] = v.Copy()
	}
	return &State{
		ClientStatus:      s.ClientStatus,
		ClientDescription: s.ClientDescription,
		DeploymentStatus:  s.DeploymentStatus.Copy(),
		TaskStates:        taskStates,
		NetworkStatus:     s.NetworkStatus.Copy(),
	}
}

// ClientTerminalStatus returns if the client status is terminal and will no longer transition
func (s *State) ClientTerminalStatus() bool {
	switch s.ClientStatus {
	case structs.AllocClientStatusComplete, structs.AllocClientStatusFailed, structs.AllocClientStatusLost:
		return true
	default:
		return false
	}
}

type AllocVolumes struct {
	CSIVolumes map[string]*CSIVolumeStub // volume request name -> CSIVolumeStub
}

// CSIVolumeStub is a stripped-down version of the CSIVolume with just the
// relevant data that we need to persist about the volume.
type CSIVolumeStub struct {
	VolumeID         string
	VolumeExternalID string
	PluginID         string
	ExternalNodeID   string
	MountInfo        *csimanager.MountInfo
}
