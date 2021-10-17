package csimanager

import (
	"context"
	"strings"

	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/nomad/structs"
)

type MountInfo struct {
	Source   string
	IsDevice bool
}

type UsageOptions struct {
	ReadOnly       bool
	AttachmentMode structs.CSIVolumeAttachmentMode
	AccessMode     structs.CSIVolumeAccessMode
	MountOptions   *structs.CSIMountOptions
}

// ToFS is used by a VolumeManager to construct the path to where a volume
// should be staged/published. It should always return a string that is easy
// enough to manage as a filesystem path segment (e.g avoid starting the string
// with a special character).
func (u *UsageOptions) ToFS() string {
	var sb strings.Builder

	if u.ReadOnly {
		sb.WriteString("ro-")
	} else {
		sb.WriteString("rw-")
	}

	sb.WriteString(string(u.AttachmentMode))
	sb.WriteString("-")
	sb.WriteString(string(u.AccessMode))

	return sb.String()
}

type VolumeMounter interface {
	MountVolume(ctx context.Context, vol *structs.CSIVolume, alloc *structs.Allocation, usageOpts *UsageOptions, publishContext map[string]string) (*MountInfo, error)
	UnmountVolume(ctx context.Context, volID, remoteID, allocID string, usageOpts *UsageOptions) error
}

type Manager interface {
	// PluginManager returns a PluginManager for use by the node fingerprinter.
	PluginManager() pluginmanager.PluginManager

	// MounterForPlugin returns a VolumeMounter for the plugin ID associated
	// with the volume.	Returns an error if this plugin isn't registered.
	MounterForPlugin(ctx context.Context, pluginID string) (VolumeMounter, error)

	// Shutdown shuts down the Manager and unmounts any locally attached volumes.
	Shutdown()
}
