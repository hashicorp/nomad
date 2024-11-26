// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import cstructs "github.com/hashicorp/nomad/client/structs"

type HostVolumeStateManager interface {
	PutDynamicHostVolume(*HostVolumeState) error
	GetDynamicHostVolume(string) (*HostVolumeState, error)
	GetDynamicHostVolumes() ([]*HostVolumeState, error)
	DeleteDynamicHostVolume(string) error
}

type HostVolumeState struct {
	ID         string
	CreateReq  *cstructs.ClientHostVolumeCreateRequest
	CreateResp *HostVolumePluginCreateResponse // has Context for re-Create or Delete
}
