package structs

import "github.com/hashicorp/nomad/nomad/structs"

type ClientHostVolumeCreateRequest struct {
	Name                string
	NodeID              string
	Plugin              string
	VolumeCapabilities  []*structs.CSIVolumeCapability
	MountOptions        *structs.CSIMountOptions
	CapacityMin         int64
	CapacityMax         int64
	RequestedTopologies *structs.CSITopologyRequest
}

type ClientHostVolumeCreateResponse struct {
	ID            string
	Path          string
	CapacityBytes int64
	VolumeContext map[string]string
	Topologies    []*structs.CSITopology
}
