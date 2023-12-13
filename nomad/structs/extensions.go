// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"reflect"
)

var (
	// extendedTypes is a mapping of extended types to their extension function
	// TODO: the duplicates could be simplified by looking up the base type in the case of a pointer type in ConvertExt
	extendedTypes = map[reflect.Type]extendFunc{
		reflect.TypeOf(Node{}):       nodeExt,
		reflect.TypeOf(&Node{}):      nodeExt,
		reflect.TypeOf(CSIVolume{}):  csiVolumeExt,
		reflect.TypeOf(&CSIVolume{}): csiVolumeExt,
	}
)

// nodeExt ensures the node is sanitized and adds the legacy field .Drain back to encoded Node objects
func nodeExt(v interface{}) interface{} {
	node := v.(*Node).Sanitize()
	// transform to a struct with inlined Node fields plus the Drain field
	// - using defined type (not an alias!) EmbeddedNode gives us free conversion to a distinct type
	// - distinct type prevents this encoding extension from being called recursively/infinitely on the embedding
	// - pointers mean the conversion function doesn't have to make a copy during conversion
	type EmbeddedNode Node
	return &struct {
		*EmbeddedNode
		Drain bool
	}{
		EmbeddedNode: (*EmbeddedNode)(node),
		Drain:        node != nil && node.DrainStrategy != nil,
	}
}

func csiVolumeExt(v interface{}) interface{} {
	vol := v.(*CSIVolume)
	type EmbeddedCSIVolume CSIVolume

	allocCount := len(vol.ReadAllocs) + len(vol.WriteAllocs)

	apiVol := &struct {
		*EmbeddedCSIVolume
		Allocations []*AllocListStub
	}{
		EmbeddedCSIVolume: (*EmbeddedCSIVolume)(vol),
		Allocations:       make([]*AllocListStub, 0, allocCount),
	}

	// WriteAllocs and ReadAllocs will only ever contain the Allocation ID,
	// with a null value for the Allocation; these IDs are mapped to
	// allocation stubs in the Allocations field. This indirection is so the
	// API can support both the UI and CLI consumer in a safely backwards
	// compatible way
	for _, a := range vol.ReadAllocs {
		if a != nil {
			apiVol.ReadAllocs[a.ID] = nil
			apiVol.Allocations = append(apiVol.Allocations, a.Stub(nil))
		}
	}
	for _, a := range vol.WriteAllocs {
		if a != nil {
			apiVol.WriteAllocs[a.ID] = nil
			apiVol.Allocations = append(apiVol.Allocations, a.Stub(nil))
		}
	}

	// MountFlags can contain secrets, so we always redact it but want
	// to show the user that we have the value
	if vol.MountOptions != nil && len(vol.MountOptions.MountFlags) > 0 {
		apiVol.MountOptions.MountFlags = []string{"[REDACTED]"}
	}

	// would be better not to have at all but left in and redacted for
	// backwards compatibility with the existing API
	apiVol.Secrets = nil

	return apiVol
}
