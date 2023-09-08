// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"fmt"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

func CSIPlugin() *structs.CSIPlugin {
	return &structs.CSIPlugin{
		ID:                 uuid.Generate(),
		Provider:           "com.hashicorp:mock",
		Version:            "0.1",
		ControllerRequired: true,
		Controllers:        map[string]*structs.CSIInfo{},
		Nodes:              map[string]*structs.CSIInfo{},
		Allocations:        []*structs.AllocListStub{},
		ControllersHealthy: 0,
		NodesHealthy:       0,
	}
}

func CSIVolume(plugin *structs.CSIPlugin) *structs.CSIVolume {
	return &structs.CSIVolume{
		ID:                  uuid.Generate(),
		Name:                "test-vol",
		ExternalID:          "vol-01",
		Namespace:           "default",
		Topologies:          []*structs.CSITopology{},
		AccessMode:          structs.CSIVolumeAccessModeUnknown,
		AttachmentMode:      structs.CSIVolumeAttachmentModeUnknown,
		MountOptions:        &structs.CSIMountOptions{},
		Secrets:             structs.CSISecrets{},
		Parameters:          map[string]string{},
		Context:             map[string]string{},
		ReadAllocs:          map[string]*structs.Allocation{},
		WriteAllocs:         map[string]*structs.Allocation{},
		ReadClaims:          map[string]*structs.CSIVolumeClaim{},
		WriteClaims:         map[string]*structs.CSIVolumeClaim{},
		PastClaims:          map[string]*structs.CSIVolumeClaim{},
		PluginID:            plugin.ID,
		Provider:            plugin.Provider,
		ProviderVersion:     plugin.Version,
		ControllerRequired:  plugin.ControllerRequired,
		ControllersHealthy:  plugin.ControllersHealthy,
		ControllersExpected: len(plugin.Controllers),
		NodesHealthy:        plugin.NodesHealthy,
		NodesExpected:       len(plugin.Nodes),
	}
}

func CSIPluginJob(pluginType structs.CSIPluginType, pluginID string) *structs.Job {

	job := new(structs.Job)

	switch pluginType {
	case structs.CSIPluginTypeController:
		job = Job()
		job.ID = fmt.Sprintf("mock-controller-%s", pluginID)
		job.Name = "job-plugin-controller"
		job.TaskGroups[0].Count = 2
	case structs.CSIPluginTypeNode:
		job = SystemJob()
		job.ID = fmt.Sprintf("mock-node-%s", pluginID)
		job.Name = "job-plugin-node"
	case structs.CSIPluginTypeMonolith:
		job = SystemJob()
		job.ID = fmt.Sprintf("mock-monolith-%s", pluginID)
		job.Name = "job-plugin-monolith"
	}

	job.TaskGroups[0].Name = "plugin"
	job.TaskGroups[0].Tasks[0].Name = "plugin"
	job.TaskGroups[0].Tasks[0].Driver = "docker"
	job.TaskGroups[0].Tasks[0].Services = nil
	job.TaskGroups[0].Tasks[0].CSIPluginConfig = &structs.TaskCSIPluginConfig{
		ID:       pluginID,
		Type:     pluginType,
		MountDir: "/csi",
	}
	job.Canonicalize()
	return job
}
