// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"slices"

	"github.com/hashicorp/nomad/plugin-interface/helper"
)

// CSISocketName is the filename that Nomad expects plugins to create inside the
// PluginMountDir.
const CSISocketName = "csi.sock"

// CSIIntermediaryDirname is the name of the directory inside the PluginMountDir
// where Nomad will expect plugins to create intermediary mounts for volumes.
const CSIIntermediaryDirname = "volumes"

// VolumeTypeCSI is the type in the volume block of a TaskGroup
const VolumeTypeCSI = "csi"

// CSIPluginType is an enum string that encapsulates the valid options for a
// CSIPlugin block's Type. These modes will allow the plugin to be used in
// different ways by the client.
type CSIPluginType string

const (
	// CSIPluginTypeNode indicates that Nomad should only use the plugin for
	// performing Node RPCs against the provided plugin.
	CSIPluginTypeNode CSIPluginType = "node"

	// CSIPluginTypeController indicates that Nomad should only use the plugin for
	// performing Controller RPCs against the provided plugin.
	CSIPluginTypeController CSIPluginType = "controller"

	// CSIPluginTypeMonolith indicates that Nomad can use the provided plugin for
	// both controller and node rpcs.
	CSIPluginTypeMonolith CSIPluginType = "monolith"
)

// CSIPluginTypeIsValid validates the given CSIPluginType string and returns
// true only when a correct plugin type is specified.
func CSIPluginTypeIsValid(pt CSIPluginType) bool {
	switch pt {
	case CSIPluginTypeNode, CSIPluginTypeController, CSIPluginTypeMonolith:
		return true
	default:
		return false
	}
}

// CSIVolumeCapability is the requested attachment and access mode for a
// volume
type CSIVolumeCapability struct {
	AttachmentMode VolumeAttachmentMode
	AccessMode     VolumeAccessMode
}

const (
	CSIVolumeAttachmentModeUnknown     VolumeAttachmentMode = ""
	CSIVolumeAttachmentModeBlockDevice VolumeAttachmentMode = "block-device"
	CSIVolumeAttachmentModeFilesystem  VolumeAttachmentMode = "file-system"
)

const (
	CSIVolumeAccessModeUnknown VolumeAccessMode = ""

	CSIVolumeAccessModeSingleNodeReader VolumeAccessMode = "single-node-reader-only"
	CSIVolumeAccessModeSingleNodeWriter VolumeAccessMode = "single-node-writer"

	CSIVolumeAccessModeMultiNodeReader       VolumeAccessMode = "multi-node-reader-only"
	CSIVolumeAccessModeMultiNodeSingleWriter VolumeAccessMode = "multi-node-single-writer"
	CSIVolumeAccessModeMultiNodeMultiWriter  VolumeAccessMode = "multi-node-multi-writer"
)

// CSIMountOptions contain optional additional configuration that can be used
// when specifying that a Volume should be used with VolumeAccessTypeMount.
type CSIMountOptions struct {
	// FSType is an optional field that allows an operator to specify the type
	// of the filesystem.
	FSType string

	// MountFlags contains additional options that may be used when mounting the
	// volume by the plugin. This may contain sensitive data and should not be
	// leaked.
	MountFlags []string
}

func (o *CSIMountOptions) Copy() *CSIMountOptions {
	if o == nil {
		return nil
	}

	no := *o
	no.MountFlags = slices.Clone(o.MountFlags)
	return &no
}

func (o *CSIMountOptions) Merge(p *CSIMountOptions) {
	if p == nil {
		return
	}
	if p.FSType != "" {
		o.FSType = p.FSType
	}
	if p.MountFlags != nil {
		o.MountFlags = p.MountFlags
	}
}

func (o *CSIMountOptions) Equal(p *CSIMountOptions) bool {
	if o == nil && p == nil {
		return true
	}
	if o == nil || p == nil {
		return false
	}
	if o.FSType != p.FSType {
		return false
	}
	return helper.SliceSetEq(o.MountFlags, p.MountFlags)
}

// CSIMountOptions implements the Stringer and GoStringer interfaces to prevent
// accidental leakage of sensitive mount flags via logs.
var _ fmt.Stringer = &CSIMountOptions{}
var _ fmt.GoStringer = &CSIMountOptions{}

func (o *CSIMountOptions) String() string {
	mountFlagsString := "nil"
	if len(o.MountFlags) != 0 {
		mountFlagsString = "[REDACTED]"
	}

	return fmt.Sprintf("csi.CSIOptions(FSType: %s, MountFlags: %s)", o.FSType, mountFlagsString)
}

func (o *CSIMountOptions) GoString() string {
	return o.String()
}

// Sanitize returns a copy of the CSIMountOptions with sensitive data redacted
func (o *CSIMountOptions) Sanitize() *CSIMountOptions {
	redacted := *o
	if len(o.MountFlags) != 0 {
		redacted.MountFlags = []string{"[REDACTED]"}
	}
	return &redacted
}

// CSISecrets contain optional additional configuration that can be used
// when specifying that a Volume should be used with VolumeAccessTypeMount.
type CSISecrets map[string]string

// CSISecrets implements the Stringer and GoStringer interfaces to prevent
// accidental leakage of secrets via logs.
var _ fmt.Stringer = &CSISecrets{}
var _ fmt.GoStringer = &CSISecrets{}

func (s *CSISecrets) String() string {
	redacted := map[string]string{}
	for k := range *s {
		redacted[k] = "[REDACTED]"
	}
	return fmt.Sprintf("csi.CSISecrets(%v)", redacted)
}

func (s *CSISecrets) GoString() string {
	return s.String()
}

// Sanitize returns a copy of the CSISecrets with sensitive data redacted
func (s *CSISecrets) Sanitize() *CSISecrets {
	redacted := CSISecrets{}
	for k := range *s {
		redacted[k] = "[REDACTED]"
	}
	return &redacted
}
