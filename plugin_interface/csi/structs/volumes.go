// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"

	multierror "github.com/hashicorp/go-multierror"
)

const (
	VolumeTypeHost = "host"

	VolumeMountPropagationPrivate       = "private"
	VolumeMountPropagationHostToTask    = "host-to-task"
	VolumeMountPropagationBidirectional = "bidirectional"

	SELinuxSharedVolume  = "z"
	SELinuxPrivateVolume = "Z"
)

var (
	errVolMountInvalidPropagationMode = fmt.Errorf("volume mount has an invalid propagation mode")
	errVolMountInvalidSELinuxLabel    = fmt.Errorf("volume mount has an invalid SELinux label")
	errVolMountEmptyVol               = fmt.Errorf("volume mount references an empty volume")
)

// ClientHostVolumeConfig is used to configure access to host paths on a Nomad Client
type ClientHostVolumeConfig struct {
	Name     string `hcl:",key"`
	Path     string `hcl:"path"`
	ReadOnly bool   `hcl:"read_only"`
	// ID is set for dynamic host volumes only.
	ID string `hcl:"-"`
}

func (p *ClientHostVolumeConfig) Equal(o *ClientHostVolumeConfig) bool {
	if p == nil && o == nil {
		return true
	}
	if p == nil || o == nil {
		return false
	}
	return *p == *o
}

func (p *ClientHostVolumeConfig) Copy() *ClientHostVolumeConfig {
	if p == nil {
		return nil
	}

	c := new(ClientHostVolumeConfig)
	*c = *p
	return c
}

func CopyMapStringClientHostVolumeConfig(m map[string]*ClientHostVolumeConfig) map[string]*ClientHostVolumeConfig {
	if m == nil {
		return nil
	}

	nm := make(map[string]*ClientHostVolumeConfig, len(m))
	for k, v := range m {
		nm[k] = v.Copy()
	}

	return nm
}

func CopySliceClientHostVolumeConfig(s []*ClientHostVolumeConfig) []*ClientHostVolumeConfig {
	l := len(s)
	if l == 0 {
		return nil
	}

	ns := make([]*ClientHostVolumeConfig, l)
	for idx, cfg := range s {
		ns[idx] = cfg.Copy()
	}

	return ns
}

func HostVolumeSliceMerge(a, b []*ClientHostVolumeConfig) []*ClientHostVolumeConfig {
	n := make([]*ClientHostVolumeConfig, len(a))
	seenKeys := make(map[string]int, len(a))

	for i, config := range a {
		n[i] = config.Copy()
		seenKeys[config.Name] = i
	}

	for _, config := range b {
		if fIndex, ok := seenKeys[config.Name]; ok {
			n[fIndex] = config.Copy()
			continue
		}

		n = append(n, config.Copy())
	}

	return n
}

// VolumeAttachmentMode chooses the type of storage api that will be used to
// interact with the device.
type VolumeAttachmentMode string

// VolumeAccessMode indicates how a volume should be used in a storage topology
// e.g whether the provider should make the volume available concurrently.
type VolumeAccessMode string

// VolumeMount represents the relationship between a destination path in a task
// and the task group volume that should be mounted there.
type VolumeMount struct {
	Volume          string
	Destination     string
	ReadOnly        bool
	PropagationMode string
	SELinuxLabel    string
}

// Hash is a very basic string based implementation of a hasher.
func (v *VolumeMount) Hash() string {
	return fmt.Sprintf("%#+v", v)
}

func (v *VolumeMount) Equal(o *VolumeMount) bool {
	if v == nil || o == nil {
		return v == o
	}
	switch {
	case v.Volume != o.Volume:
		return false
	case v.Destination != o.Destination:
		return false
	case v.ReadOnly != o.ReadOnly:
		return false
	case v.PropagationMode != o.PropagationMode:
		return false
	case v.SELinuxLabel != o.SELinuxLabel:
		return false
	}

	return true
}

func (v *VolumeMount) Copy() *VolumeMount {
	if v == nil {
		return nil
	}

	nv := new(VolumeMount)
	*nv = *v
	return nv
}

func (v *VolumeMount) Validate() error {
	var mErr *multierror.Error

	// Validate the task does not reference undefined volume mounts
	if v.Volume == "" {
		mErr = multierror.Append(mErr, errVolMountEmptyVol)
	}

	if !v.MountPropagationModeIsValid() {
		mErr = multierror.Append(mErr, fmt.Errorf("%w: %q", errVolMountInvalidPropagationMode, v.PropagationMode))
	}

	if !v.SELinuxLabelIsValid() {
		mErr = multierror.Append(mErr, fmt.Errorf("%w: \"%s\"", errVolMountInvalidSELinuxLabel, v.SELinuxLabel))
	}

	return mErr.ErrorOrNil()
}

func (v *VolumeMount) MountPropagationModeIsValid() bool {
	switch v.PropagationMode {
	case "", VolumeMountPropagationPrivate, VolumeMountPropagationHostToTask, VolumeMountPropagationBidirectional:
		return true
	default:
		return false
	}
}

func (v *VolumeMount) SELinuxLabelIsValid() bool {
	switch v.SELinuxLabel {
	case "", SELinuxSharedVolume, SELinuxPrivateVolume:
		return true
	default:
		return false
	}
}

func CopySliceVolumeMount(s []*VolumeMount) []*VolumeMount {
	l := len(s)
	if l == 0 {
		return nil
	}

	c := make([]*VolumeMount, l)
	for i, v := range s {
		c[i] = v.Copy()
	}
	return c
}
