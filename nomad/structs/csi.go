package structs

import (
	"errors"
	"fmt"
	"strings"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper"
)

// CSISocketName is the filename that Nomad expects plugins to create inside the
// PluginMountDir.
const CSISocketName = "csi.sock"

// CSIIntermediaryDirname is the name of the directory inside the PluginMountDir
// where Nomad will expect plugins to create intermediary mounts for volumes.
const CSIIntermediaryDirname = "volumes"

// VolumeTypeCSI is the type in the volume stanza of a TaskGroup
const VolumeTypeCSI = "csi"

// CSIPluginType is an enum string that encapsulates the valid options for a
// CSIPlugin stanza's Type. These modes will allow the plugin to be used in
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

// TaskCSIPluginConfig contains the data that is required to setup a task as a
// CSI plugin. This will be used by the csi_plugin_supervisor_hook to configure
// mounts for the plugin and initiate the connection to the plugin catalog.
type TaskCSIPluginConfig struct {
	// ID is the identifier of the plugin.
	// Ideally this should be the FQDN of the plugin.
	ID string

	// Type instructs Nomad on how to handle processing a plugin
	Type CSIPluginType

	// MountDir is the destination that nomad should mount in its CSI
	// directory for the plugin. It will then expect a file called CSISocketName
	// to be created by the plugin, and will provide references into
	// "MountDir/CSIIntermediaryDirname/{VolumeName}/{AllocID} for mounts.
	MountDir string
}

func (t *TaskCSIPluginConfig) Copy() *TaskCSIPluginConfig {
	if t == nil {
		return nil
	}

	nt := new(TaskCSIPluginConfig)
	*nt = *t

	return nt
}

// CSIVolumeCapability is the requested attachment and access mode for a
// volume
type CSIVolumeCapability struct {
	AttachmentMode CSIVolumeAttachmentMode
	AccessMode     CSIVolumeAccessMode
}

// CSIVolumeAttachmentMode chooses the type of storage api that will be used to
// interact with the device.
type CSIVolumeAttachmentMode string

const (
	CSIVolumeAttachmentModeUnknown     CSIVolumeAttachmentMode = ""
	CSIVolumeAttachmentModeBlockDevice CSIVolumeAttachmentMode = "block-device"
	CSIVolumeAttachmentModeFilesystem  CSIVolumeAttachmentMode = "file-system"
)

func ValidCSIVolumeAttachmentMode(attachmentMode CSIVolumeAttachmentMode) bool {
	switch attachmentMode {
	case CSIVolumeAttachmentModeBlockDevice, CSIVolumeAttachmentModeFilesystem:
		return true
	default:
		return false
	}
}

// CSIVolumeAccessMode indicates how a volume should be used in a storage topology
// e.g whether the provider should make the volume available concurrently.
type CSIVolumeAccessMode string

const (
	CSIVolumeAccessModeUnknown CSIVolumeAccessMode = ""

	CSIVolumeAccessModeSingleNodeReader CSIVolumeAccessMode = "single-node-reader-only"
	CSIVolumeAccessModeSingleNodeWriter CSIVolumeAccessMode = "single-node-writer"

	CSIVolumeAccessModeMultiNodeReader       CSIVolumeAccessMode = "multi-node-reader-only"
	CSIVolumeAccessModeMultiNodeSingleWriter CSIVolumeAccessMode = "multi-node-single-writer"
	CSIVolumeAccessModeMultiNodeMultiWriter  CSIVolumeAccessMode = "multi-node-multi-writer"
)

// ValidCSIVolumeAccessMode checks to see that the provided access mode is a valid,
// non-empty access mode.
func ValidCSIVolumeAccessMode(accessMode CSIVolumeAccessMode) bool {
	switch accessMode {
	case CSIVolumeAccessModeSingleNodeReader, CSIVolumeAccessModeSingleNodeWriter,
		CSIVolumeAccessModeMultiNodeReader, CSIVolumeAccessModeMultiNodeSingleWriter,
		CSIVolumeAccessModeMultiNodeMultiWriter:
		return true
	default:
		return false
	}
}

// ValidCSIVolumeWriteAccessMode checks for a writable access mode.
func ValidCSIVolumeWriteAccessMode(accessMode CSIVolumeAccessMode) bool {
	switch accessMode {
	case CSIVolumeAccessModeSingleNodeWriter,
		CSIVolumeAccessModeMultiNodeSingleWriter,
		CSIVolumeAccessModeMultiNodeMultiWriter:
		return true
	default:
		return false
	}
}

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
	no.MountFlags = helper.CopySliceString(o.MountFlags)
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

	return helper.CompareSliceSetString(
		o.MountFlags, p.MountFlags)
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

type CSIVolumeClaim struct {
	AllocationID   string
	NodeID         string
	ExternalNodeID string
	Mode           CSIVolumeClaimMode
	AccessMode     CSIVolumeAccessMode
	AttachmentMode CSIVolumeAttachmentMode
	State          CSIVolumeClaimState
}

type CSIVolumeClaimState int

const (
	CSIVolumeClaimStateTaken CSIVolumeClaimState = iota
	CSIVolumeClaimStateNodeDetached
	CSIVolumeClaimStateControllerDetached
	CSIVolumeClaimStateReadyToFree
	CSIVolumeClaimStateUnpublishing
)

// CSIVolume is the full representation of a CSI Volume
type CSIVolume struct {
	// ID is a namespace unique URL safe identifier for the volume
	ID string
	// Name is a display name for the volume, not required to be unique
	Name string
	// ExternalID identifies the volume for the CSI interface, may be URL unsafe
	ExternalID string
	Namespace  string

	// RequestedTopologies are the topologies submitted as options to
	// the storage provider at the time the volume was created. After
	// volumes are created, this field is ignored.
	RequestedTopologies *CSITopologyRequest

	// Topologies are the topologies returned by the storage provider,
	// based on the RequestedTopologies and what the storage provider
	// could support. This value cannot be set by the user.
	Topologies []*CSITopology

	AccessMode     CSIVolumeAccessMode     // *current* access mode
	AttachmentMode CSIVolumeAttachmentMode // *current* attachment mode
	MountOptions   *CSIMountOptions

	Secrets    CSISecrets
	Parameters map[string]string
	Context    map[string]string
	Capacity   int64 // bytes

	// These values are used only on volume creation but we record them
	// so that we can diff the volume later
	RequestedCapacityMin  int64 // bytes
	RequestedCapacityMax  int64 // bytes
	RequestedCapabilities []*CSIVolumeCapability
	CloneID               string
	SnapshotID            string

	// Allocations, tracking claim status
	ReadAllocs  map[string]*Allocation // AllocID -> Allocation
	WriteAllocs map[string]*Allocation // AllocID -> Allocation

	ReadClaims  map[string]*CSIVolumeClaim // AllocID -> claim
	WriteClaims map[string]*CSIVolumeClaim // AllocID -> claim
	PastClaims  map[string]*CSIVolumeClaim // AllocID -> claim

	// Schedulable is true if all the denormalized plugin health fields are true, and the
	// volume has not been marked for garbage collection
	Schedulable         bool
	PluginID            string
	Provider            string
	ProviderVersion     string
	ControllerRequired  bool
	ControllersHealthy  int
	ControllersExpected int
	NodesHealthy        int
	NodesExpected       int
	ResourceExhausted   time.Time

	CreateIndex uint64
	ModifyIndex uint64
}

// CSIVolListStub is partial representation of a CSI Volume for inclusion in lists
type CSIVolListStub struct {
	ID                  string
	Namespace           string
	Name                string
	ExternalID          string
	Topologies          []*CSITopology
	AccessMode          CSIVolumeAccessMode
	AttachmentMode      CSIVolumeAttachmentMode
	CurrentReaders      int
	CurrentWriters      int
	Schedulable         bool
	PluginID            string
	Provider            string
	ControllersHealthy  int
	ControllersExpected int
	NodesHealthy        int
	NodesExpected       int
	CreateIndex         uint64
	ModifyIndex         uint64
}

// NewCSIVolume creates the volume struct. No side-effects
func NewCSIVolume(volumeID string, index uint64) *CSIVolume {
	out := &CSIVolume{
		ID:          volumeID,
		CreateIndex: index,
		ModifyIndex: index,
	}

	out.newStructs()
	return out
}

func (v *CSIVolume) newStructs() {
	v.Topologies = []*CSITopology{}
	v.MountOptions = new(CSIMountOptions)
	v.Secrets = CSISecrets{}
	v.Parameters = map[string]string{}
	v.Context = map[string]string{}

	v.ReadAllocs = map[string]*Allocation{}
	v.WriteAllocs = map[string]*Allocation{}
	v.ReadClaims = map[string]*CSIVolumeClaim{}
	v.WriteClaims = map[string]*CSIVolumeClaim{}
	v.PastClaims = map[string]*CSIVolumeClaim{}
}

func (v *CSIVolume) RemoteID() string {
	if v.ExternalID != "" {
		return v.ExternalID
	}
	return v.ID
}

func (v *CSIVolume) Stub() *CSIVolListStub {
	stub := CSIVolListStub{
		ID:                  v.ID,
		Namespace:           v.Namespace,
		Name:                v.Name,
		ExternalID:          v.ExternalID,
		Topologies:          v.Topologies,
		AccessMode:          v.AccessMode,
		AttachmentMode:      v.AttachmentMode,
		CurrentReaders:      len(v.ReadAllocs),
		CurrentWriters:      len(v.WriteAllocs),
		Schedulable:         v.Schedulable,
		PluginID:            v.PluginID,
		Provider:            v.Provider,
		ControllersHealthy:  v.ControllersHealthy,
		ControllersExpected: v.ControllersExpected,
		NodesHealthy:        v.NodesHealthy,
		NodesExpected:       v.NodesExpected,
		CreateIndex:         v.CreateIndex,
		ModifyIndex:         v.ModifyIndex,
	}

	return &stub
}

// ReadSchedulable determines if the volume is potentially schedulable
// for reads, considering only the volume capabilities and plugin
// health
func (v *CSIVolume) ReadSchedulable() bool {
	if !v.Schedulable {
		return false
	}

	return v.ResourceExhausted == time.Time{}
}

// WriteSchedulable determines if the volume is potentially
// schedulable for writes, considering only volume capabilities and
// plugin health
func (v *CSIVolume) WriteSchedulable() bool {
	if !v.Schedulable {
		return false
	}

	switch v.AccessMode {
	case CSIVolumeAccessModeSingleNodeWriter,
		CSIVolumeAccessModeMultiNodeSingleWriter,
		CSIVolumeAccessModeMultiNodeMultiWriter:
		return v.ResourceExhausted == time.Time{}

	case CSIVolumeAccessModeUnknown:
		// this volume was created but not currently claimed, so we check what
		// it's capable of, not what it's been previously assigned
		for _, cap := range v.RequestedCapabilities {
			switch cap.AccessMode {
			case CSIVolumeAccessModeSingleNodeWriter,
				CSIVolumeAccessModeMultiNodeSingleWriter,
				CSIVolumeAccessModeMultiNodeMultiWriter:
				return v.ResourceExhausted == time.Time{}
			}
		}
	}
	return false
}

// HasFreeReadClaims determines if there are any free read claims available
func (v *CSIVolume) HasFreeReadClaims() bool {
	switch v.AccessMode {
	case CSIVolumeAccessModeSingleNodeReader:
		return len(v.ReadClaims) == 0
	case CSIVolumeAccessModeSingleNodeWriter:
		return len(v.ReadClaims) == 0 && len(v.WriteClaims) == 0
	case CSIVolumeAccessModeUnknown:
		// This volume was created but not yet claimed, so its
		// capabilities have been checked in ReadSchedulable
		return true
	default:
		// For multi-node AccessModes, the CSI spec doesn't allow for
		// setting a max number of readers we track node resource
		// exhaustion through v.ResourceExhausted which is checked in
		// ReadSchedulable
		return true
	}
}

// HasFreeWriteClaims determines if there are any free write claims available
func (v *CSIVolume) HasFreeWriteClaims() bool {
	switch v.AccessMode {
	case CSIVolumeAccessModeSingleNodeWriter, CSIVolumeAccessModeMultiNodeSingleWriter:
		return len(v.WriteClaims) == 0
	case CSIVolumeAccessModeMultiNodeMultiWriter:
		// the CSI spec doesn't allow for setting a max number of writers.
		// we track node resource exhaustion through v.ResourceExhausted
		// which is checked in WriteSchedulable
		return true
	case CSIVolumeAccessModeUnknown:
		// This volume was created but not yet claimed, so its
		// capabilities have been checked in WriteSchedulable
		return true
	default:
		// Reader modes never have free write claims
		return false
	}
}

// InUse tests whether any allocations are actively using the volume
func (v *CSIVolume) InUse() bool {
	return len(v.ReadAllocs) != 0 ||
		len(v.WriteAllocs) != 0
}

// Copy returns a copy of the volume, which shares only the Topologies slice
func (v *CSIVolume) Copy() *CSIVolume {
	out := new(CSIVolume)
	*out = *v
	out.newStructs() // zero-out the non-primitive structs

	for _, t := range v.Topologies {
		out.Topologies = append(out.Topologies, t.Copy())
	}
	if v.MountOptions != nil {
		*out.MountOptions = *v.MountOptions
	}
	for k, v := range v.Secrets {
		out.Secrets[k] = v
	}
	for k, v := range v.Parameters {
		out.Parameters[k] = v
	}
	for k, v := range v.Context {
		out.Context[k] = v
	}

	for k, alloc := range v.ReadAllocs {
		out.ReadAllocs[k] = alloc.Copy()
	}
	for k, alloc := range v.WriteAllocs {
		out.WriteAllocs[k] = alloc.Copy()
	}

	for k, v := range v.ReadClaims {
		claim := *v
		out.ReadClaims[k] = &claim
	}
	for k, v := range v.WriteClaims {
		claim := *v
		out.WriteClaims[k] = &claim
	}
	for k, v := range v.PastClaims {
		claim := *v
		out.PastClaims[k] = &claim
	}

	return out
}

// Claim updates the allocations and changes the volume state
func (v *CSIVolume) Claim(claim *CSIVolumeClaim, alloc *Allocation) error {
	// COMPAT: volumes registered prior to 1.1.0 will be missing caps for the
	// volume on any claim. Correct this when we make the first change to a
	// claim by setting its currently claimed capability as the only requested
	// capability
	if len(v.RequestedCapabilities) == 0 && v.AccessMode != "" && v.AttachmentMode != "" {
		v.RequestedCapabilities = []*CSIVolumeCapability{
			{
				AccessMode:     v.AccessMode,
				AttachmentMode: v.AttachmentMode,
			},
		}
	}
	if v.AttachmentMode != CSIVolumeAttachmentModeUnknown &&
		claim.AttachmentMode != CSIVolumeAttachmentModeUnknown &&
		v.AttachmentMode != claim.AttachmentMode {
		return fmt.Errorf("cannot change attachment mode of claimed volume")
	}

	if claim.State == CSIVolumeClaimStateTaken {
		switch claim.Mode {
		case CSIVolumeClaimRead:
			return v.claimRead(claim, alloc)
		case CSIVolumeClaimWrite:
			return v.claimWrite(claim, alloc)
		}
	}
	// either GC or a Unpublish checkpoint
	return v.claimRelease(claim)
}

// claimRead marks an allocation as using a volume read-only
func (v *CSIVolume) claimRead(claim *CSIVolumeClaim, alloc *Allocation) error {
	if _, ok := v.ReadAllocs[claim.AllocationID]; ok {
		return nil
	}
	if alloc == nil {
		return fmt.Errorf("allocation missing: %s", claim.AllocationID)
	}

	if !v.ReadSchedulable() {
		return ErrCSIVolumeUnschedulable
	}

	if !v.HasFreeReadClaims() {
		return ErrCSIVolumeMaxClaims
	}

	// Allocations are copy on write, so we want to keep the id but don't need the
	// pointer. We'll get it from the db in denormalize.
	v.ReadAllocs[claim.AllocationID] = nil
	delete(v.WriteAllocs, claim.AllocationID)

	v.ReadClaims[claim.AllocationID] = claim
	delete(v.WriteClaims, claim.AllocationID)
	delete(v.PastClaims, claim.AllocationID)

	v.setModesFromClaim(claim)
	return nil
}

// claimWrite marks an allocation as using a volume as a writer
func (v *CSIVolume) claimWrite(claim *CSIVolumeClaim, alloc *Allocation) error {
	if _, ok := v.WriteAllocs[claim.AllocationID]; ok {
		return nil
	}
	if alloc == nil {
		return fmt.Errorf("allocation missing: %s", claim.AllocationID)
	}

	if !v.WriteSchedulable() {
		return ErrCSIVolumeUnschedulable
	}

	if !v.HasFreeWriteClaims() {
		return ErrCSIVolumeMaxClaims
	}

	// Allocations are copy on write, so we want to keep the id but don't need the
	// pointer. We'll get it from the db in denormalize.
	v.WriteAllocs[alloc.ID] = nil
	delete(v.ReadAllocs, alloc.ID)

	v.WriteClaims[alloc.ID] = claim
	delete(v.ReadClaims, alloc.ID)
	delete(v.PastClaims, alloc.ID)

	v.setModesFromClaim(claim)
	return nil
}

// setModesFromClaim sets the volume AttachmentMode and AccessMode based on
// the first claim we make.  Originally the volume AccessMode and
// AttachmentMode were set during registration, but this is incorrect once we
// started creating volumes ourselves. But we still want these values for CLI
// and UI status.
func (v *CSIVolume) setModesFromClaim(claim *CSIVolumeClaim) {
	if v.AttachmentMode == CSIVolumeAttachmentModeUnknown {
		v.AttachmentMode = claim.AttachmentMode
	}
	if v.AccessMode == CSIVolumeAccessModeUnknown {
		v.AccessMode = claim.AccessMode
	}
}

// claimRelease is called when the allocation has terminated and
// already stopped using the volume
func (v *CSIVolume) claimRelease(claim *CSIVolumeClaim) error {
	if claim.State == CSIVolumeClaimStateReadyToFree {
		delete(v.ReadAllocs, claim.AllocationID)
		delete(v.WriteAllocs, claim.AllocationID)
		delete(v.ReadClaims, claim.AllocationID)
		delete(v.WriteClaims, claim.AllocationID)
		delete(v.PastClaims, claim.AllocationID)

		// remove AccessMode/AttachmentMode if this is the last claim
		if len(v.ReadClaims) == 0 && len(v.WriteClaims) == 0 && len(v.PastClaims) == 0 {
			v.AccessMode = CSIVolumeAccessModeUnknown
			v.AttachmentMode = CSIVolumeAttachmentModeUnknown
		}
	} else {
		v.PastClaims[claim.AllocationID] = claim
	}
	return nil
}

// Equal checks equality by value.
func (v *CSIVolume) Equal(o *CSIVolume) bool {
	if v == nil || o == nil {
		return v == o
	}

	// Omit the plugin health fields, their values are controlled by plugin jobs
	if v.ID == o.ID &&
		v.Namespace == o.Namespace &&
		v.AccessMode == o.AccessMode &&
		v.AttachmentMode == o.AttachmentMode &&
		v.PluginID == o.PluginID {
		// Setwise equality of topologies
		var ok bool
		for _, t := range v.Topologies {
			ok = false
			for _, u := range o.Topologies {
				if t.Equal(u) {
					ok = true
					break
				}
			}
			if !ok {
				return false
			}
		}
		return true
	}
	return false
}

// Validate validates the volume struct, returning all validation errors at once
func (v *CSIVolume) Validate() error {
	errs := []string{}

	if v.ID == "" {
		errs = append(errs, "missing volume id")
	}
	if v.PluginID == "" {
		errs = append(errs, "missing plugin id")
	}
	if v.Namespace == "" {
		errs = append(errs, "missing namespace")
	}
	if v.SnapshotID != "" && v.CloneID != "" {
		errs = append(errs, "only one of snapshot_id and clone_id is allowed")
	}
	if len(v.RequestedCapabilities) == 0 {
		errs = append(errs, "must include at least one capability block")
	}
	if v.RequestedTopologies != nil {
		for _, t := range v.RequestedTopologies.Required {
			if t != nil && len(t.Segments) == 0 {
				errs = append(errs, "required topology is missing segments field")
			}
		}
		for _, t := range v.RequestedTopologies.Preferred {
			if t != nil && len(t.Segments) == 0 {
				errs = append(errs, "preferred topology is missing segments field")
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("validation: %s", strings.Join(errs, ", "))
	}
	return nil
}

// Merge updates the mutable fields of a volume with those from
// another volume. CSIVolume has many user-defined fields which are
// immutable once set, and many fields that are not
// user-settable. Merge will return an error if we try to mutate the
// user-defined immutable fields after they're set, but silently
// ignore fields that are controlled by Nomad.
func (v *CSIVolume) Merge(other *CSIVolume) error {
	if other == nil {
		return nil
	}

	var errs *multierror.Error

	if v.Name != other.Name && other.Name != "" {
		errs = multierror.Append(errs, errors.New("volume name cannot be updated"))
	}
	if v.ExternalID != other.ExternalID && other.ExternalID != "" {
		errs = multierror.Append(errs, errors.New(
			"volume external ID cannot be updated"))
	}
	if v.PluginID != other.PluginID {
		errs = multierror.Append(errs, errors.New(
			"volume plugin ID cannot be updated"))
	}
	if v.CloneID != other.CloneID && other.CloneID != "" {
		errs = multierror.Append(errs, errors.New(
			"volume clone ID cannot be updated"))
	}
	if v.SnapshotID != other.SnapshotID && other.SnapshotID != "" {
		errs = multierror.Append(errs, errors.New(
			"volume snapshot ID cannot be updated"))
	}

	// must be compatible with capacity range
	// TODO: when ExpandVolume is implemented we'll need to update
	// this logic https://github.com/hashicorp/nomad/issues/10324
	if v.Capacity != 0 {
		if other.RequestedCapacityMax < v.Capacity ||
			other.RequestedCapacityMin > v.Capacity {
			errs = multierror.Append(errs, errors.New(
				"volume requested capacity update was not compatible with existing capacity"))
		} else {
			v.RequestedCapacityMin = other.RequestedCapacityMin
			v.RequestedCapacityMax = other.RequestedCapacityMax
		}
	}

	// must be compatible with volume_capabilities
	if v.AccessMode != CSIVolumeAccessModeUnknown ||
		v.AttachmentMode != CSIVolumeAttachmentModeUnknown {
		var ok bool
		for _, cap := range other.RequestedCapabilities {
			if cap.AccessMode == v.AccessMode &&
				cap.AttachmentMode == v.AttachmentMode {
				ok = true
				break
			}
		}
		if ok {
			v.RequestedCapabilities = other.RequestedCapabilities
		} else {
			errs = multierror.Append(errs, errors.New(
				"volume requested capabilities update was not compatible with existing capability in use"))
		}
	} else {
		v.RequestedCapabilities = other.RequestedCapabilities
	}

	// topologies are immutable, so topology request changes must be
	// compatible with the existing topology, if any
	if len(v.Topologies) > 0 {
		if !v.RequestedTopologies.Equal(other.RequestedTopologies) {
			errs = multierror.Append(errs, errors.New(
				"volume topology request update was not compatible with existing topology"))
		}
	}

	// MountOptions can be updated so long as the volume isn't in use,
	// but the caller will reject updating an in-use volume
	v.MountOptions = other.MountOptions

	// Secrets can be updated freely
	v.Secrets = other.Secrets

	// must be compatible with parameters set by from CreateVolumeResponse

	if len(other.Parameters) != 0 && !helper.CompareMapStringString(v.Parameters, other.Parameters) {
		errs = multierror.Append(errs, errors.New(
			"volume parameters cannot be updated"))
	}

	// Context is mutable and will be used during controller
	// validation
	v.Context = other.Context
	return errs.ErrorOrNil()
}

// Request and response wrappers
type CSIVolumeRegisterRequest struct {
	Volumes []*CSIVolume
	WriteRequest
}

type CSIVolumeRegisterResponse struct {
	QueryMeta
}

type CSIVolumeDeregisterRequest struct {
	VolumeIDs []string
	Force     bool
	WriteRequest
}

type CSIVolumeDeregisterResponse struct {
	QueryMeta
}

type CSIVolumeCreateRequest struct {
	Volumes []*CSIVolume
	WriteRequest
}

type CSIVolumeCreateResponse struct {
	Volumes []*CSIVolume
	QueryMeta
}

type CSIVolumeDeleteRequest struct {
	VolumeIDs []string
	WriteRequest
}

type CSIVolumeDeleteResponse struct {
	QueryMeta
}

type CSIVolumeClaimMode int

const (
	CSIVolumeClaimRead CSIVolumeClaimMode = iota
	CSIVolumeClaimWrite

	// for GC we don't have a specific claim to set the state on, so instead we
	// create a new claim for GC in order to bump the ModifyIndex and trigger
	// volumewatcher
	CSIVolumeClaimGC
)

type CSIVolumeClaimBatchRequest struct {
	Claims []CSIVolumeClaimRequest
}

type CSIVolumeClaimRequest struct {
	VolumeID       string
	AllocationID   string
	NodeID         string
	ExternalNodeID string
	Claim          CSIVolumeClaimMode
	AccessMode     CSIVolumeAccessMode
	AttachmentMode CSIVolumeAttachmentMode
	State          CSIVolumeClaimState
	WriteRequest
}

func (req *CSIVolumeClaimRequest) ToClaim() *CSIVolumeClaim {
	return &CSIVolumeClaim{
		AllocationID:   req.AllocationID,
		NodeID:         req.NodeID,
		ExternalNodeID: req.ExternalNodeID,
		Mode:           req.Claim,
		AccessMode:     req.AccessMode,
		AttachmentMode: req.AttachmentMode,
		State:          req.State,
	}
}

type CSIVolumeClaimResponse struct {
	// Opaque static publish properties of the volume. SP MAY use this
	// field to ensure subsequent `NodeStageVolume` or `NodePublishVolume`
	// calls calls have contextual information.
	// The contents of this field SHALL be opaque to nomad.
	// The contents of this field SHALL NOT be mutable.
	// The contents of this field SHALL be safe for the nomad to cache.
	// The contents of this field SHOULD NOT contain sensitive
	// information.
	// The contents of this field SHOULD NOT be used for uniquely
	// identifying a volume. The `volume_id` alone SHOULD be sufficient to
	// identify the volume.
	// This field is OPTIONAL and when present MUST be passed to
	// `NodeStageVolume` or `NodePublishVolume` calls on the client
	PublishContext map[string]string

	// Volume contains the expanded CSIVolume for use on the client after a Claim
	// has completed.
	Volume *CSIVolume

	QueryMeta
}

type CSIVolumeListRequest struct {
	PluginID string
	NodeID   string
	QueryOptions
}

type CSIVolumeListResponse struct {
	Volumes []*CSIVolListStub
	QueryMeta
}

// CSIVolumeExternalListRequest is a request to a controller plugin to list
// all the volumes known to the the storage provider. This request is
// paginated by the plugin and accepts the QueryOptions.PerPage and
// QueryOptions.NextToken fields
type CSIVolumeExternalListRequest struct {
	PluginID string
	QueryOptions
}

type CSIVolumeExternalListResponse struct {
	Volumes   []*CSIVolumeExternalStub
	NextToken string
	QueryMeta
}

// CSIVolumeExternalStub is the storage provider's view of a volume, as
// returned from the controller plugin; all IDs are for external resources
type CSIVolumeExternalStub struct {
	ExternalID    string
	CapacityBytes int64
	VolumeContext map[string]string
	CloneID       string
	SnapshotID    string

	PublishedExternalNodeIDs []string
	IsAbnormal               bool
	Status                   string
}

type CSIVolumeGetRequest struct {
	ID string
	QueryOptions
}

type CSIVolumeGetResponse struct {
	Volume *CSIVolume
	QueryMeta
}

type CSIVolumeUnpublishRequest struct {
	VolumeID string
	Claim    *CSIVolumeClaim
	WriteRequest
}

type CSIVolumeUnpublishResponse struct {
	QueryMeta
}

// CSISnapshot is the storage provider's view of a volume snapshot
type CSISnapshot struct {
	// These fields map to those returned by the storage provider plugin
	ID                     string // storage provider's ID
	ExternalSourceVolumeID string // storage provider's ID for volume
	SizeBytes              int64
	CreateTime             int64
	IsReady                bool

	// These fields are controlled by Nomad
	SourceVolumeID string
	PluginID       string

	// These field are only used during snapshot creation and will not be
	// populated when the snapshot is returned
	Name       string
	Secrets    CSISecrets
	Parameters map[string]string
}

type CSISnapshotCreateRequest struct {
	Snapshots []*CSISnapshot
	WriteRequest
}

type CSISnapshotCreateResponse struct {
	Snapshots []*CSISnapshot
	QueryMeta
}

type CSISnapshotDeleteRequest struct {
	Snapshots []*CSISnapshot
	WriteRequest
}

type CSISnapshotDeleteResponse struct {
	QueryMeta
}

// CSISnapshotListRequest is a request to a controller plugin to list all the
// snapshot known to the the storage provider. This request is paginated by
// the plugin and accepts the QueryOptions.PerPage and QueryOptions.NextToken
// fields
type CSISnapshotListRequest struct {
	PluginID string
	Secrets  CSISecrets
	QueryOptions
}

type CSISnapshotListResponse struct {
	Snapshots []*CSISnapshot
	NextToken string
	QueryMeta
}

// CSIPlugin collects fingerprint info context for the plugin for clients
type CSIPlugin struct {
	ID                 string
	Provider           string // the vendor name from CSI GetPluginInfoResponse
	Version            string // the vendor verson from  CSI GetPluginInfoResponse
	ControllerRequired bool

	// Map Node.IDs to fingerprint results, split by type. Monolith type plugins have
	// both sets of fingerprinting results.
	Controllers map[string]*CSIInfo
	Nodes       map[string]*CSIInfo

	// Allocations are populated by denormalize to show running allocations
	Allocations []*AllocListStub

	// Jobs are populated to by job update to support expected counts and the UI
	ControllerJobs JobDescriptions
	NodeJobs       JobDescriptions

	// Cache the count of healthy plugins
	ControllersHealthy  int
	ControllersExpected int
	NodesHealthy        int
	NodesExpected       int

	CreateIndex uint64
	ModifyIndex uint64
}

// NewCSIPlugin creates the plugin struct. No side-effects
func NewCSIPlugin(id string, index uint64) *CSIPlugin {
	out := &CSIPlugin{
		ID:          id,
		CreateIndex: index,
		ModifyIndex: index,
	}

	out.newStructs()
	return out
}

func (p *CSIPlugin) newStructs() {
	p.Controllers = map[string]*CSIInfo{}
	p.Nodes = map[string]*CSIInfo{}
	p.ControllerJobs = make(JobDescriptions)
	p.NodeJobs = make(JobDescriptions)
}

func (p *CSIPlugin) Copy() *CSIPlugin {
	copy := *p
	out := &copy
	out.newStructs()

	for k, v := range p.Controllers {
		out.Controllers[k] = v.Copy()
	}

	for k, v := range p.Nodes {
		out.Nodes[k] = v.Copy()
	}

	for k, v := range p.ControllerJobs {
		out.ControllerJobs[k] = v.Copy()
	}

	for k, v := range p.NodeJobs {
		out.NodeJobs[k] = v.Copy()
	}

	return out
}

type CSIControllerCapability byte

const (
	// CSIControllerSupportsCreateDelete indicates plugin support for
	// CREATE_DELETE_VOLUME
	CSIControllerSupportsCreateDelete CSIControllerCapability = 0

	// CSIControllerSupportsAttachDetach is true when the controller
	// implements the methods required to attach and detach volumes. If this
	// is false Nomad should skip the controller attachment flow.
	CSIControllerSupportsAttachDetach CSIControllerCapability = 1

	// CSIControllerSupportsListVolumes is true when the controller implements
	// the ListVolumes RPC. NOTE: This does not guarantee that attached nodes
	// will be returned unless SupportsListVolumesAttachedNodes is also true.
	CSIControllerSupportsListVolumes CSIControllerCapability = 2

	// CSIControllerSupportsGetCapacity indicates plugin support for
	// GET_CAPACITY
	CSIControllerSupportsGetCapacity CSIControllerCapability = 3

	// CSIControllerSupportsCreateDeleteSnapshot indicates plugin support for
	// CREATE_DELETE_SNAPSHOT
	CSIControllerSupportsCreateDeleteSnapshot CSIControllerCapability = 4

	// CSIControllerSupportsListSnapshots indicates plugin support for
	// LIST_SNAPSHOTS
	CSIControllerSupportsListSnapshots CSIControllerCapability = 5

	// CSIControllerSupportsClone indicates plugin support for CLONE_VOLUME
	CSIControllerSupportsClone CSIControllerCapability = 6

	// CSIControllerSupportsReadOnlyAttach is set to true when the controller
	// returns the ATTACH_READONLY capability.
	CSIControllerSupportsReadOnlyAttach CSIControllerCapability = 7

	// CSIControllerSupportsExpand indicates plugin support for EXPAND_VOLUME
	CSIControllerSupportsExpand CSIControllerCapability = 8

	// CSIControllerSupportsListVolumesAttachedNodes indicates whether the
	// plugin will return attached nodes data when making ListVolume RPCs
	// (plugin support for LIST_VOLUMES_PUBLISHED_NODES)
	CSIControllerSupportsListVolumesAttachedNodes CSIControllerCapability = 9

	// CSIControllerSupportsCondition indicates plugin support for
	// VOLUME_CONDITION
	CSIControllerSupportsCondition CSIControllerCapability = 10

	// CSIControllerSupportsGet indicates plugin support for GET_VOLUME
	CSIControllerSupportsGet CSIControllerCapability = 11
)

type CSINodeCapability byte

const (

	// CSINodeSupportsStageVolume indicates whether the client should
	// Stage/Unstage volumes on this node.
	CSINodeSupportsStageVolume CSINodeCapability = 0

	// CSINodeSupportsStats indicates plugin support for GET_VOLUME_STATS
	CSINodeSupportsStats CSINodeCapability = 1

	// CSINodeSupportsExpand indicates plugin support for EXPAND_VOLUME
	CSINodeSupportsExpand CSINodeCapability = 2

	// CSINodeSupportsCondition indicates plugin support for VOLUME_CONDITION
	CSINodeSupportsCondition CSINodeCapability = 3
)

func (p *CSIPlugin) HasControllerCapability(cap CSIControllerCapability) bool {
	if len(p.Controllers) < 1 {
		return false
	}
	// we're picking the first controller because they should be uniform
	// across the same version of the plugin
	for _, c := range p.Controllers {
		switch cap {
		case CSIControllerSupportsCreateDelete:
			return c.ControllerInfo.SupportsCreateDelete
		case CSIControllerSupportsAttachDetach:
			return c.ControllerInfo.SupportsAttachDetach
		case CSIControllerSupportsListVolumes:
			return c.ControllerInfo.SupportsListVolumes
		case CSIControllerSupportsGetCapacity:
			return c.ControllerInfo.SupportsGetCapacity
		case CSIControllerSupportsCreateDeleteSnapshot:
			return c.ControllerInfo.SupportsCreateDeleteSnapshot
		case CSIControllerSupportsListSnapshots:
			return c.ControllerInfo.SupportsListSnapshots
		case CSIControllerSupportsClone:
			return c.ControllerInfo.SupportsClone
		case CSIControllerSupportsReadOnlyAttach:
			return c.ControllerInfo.SupportsReadOnlyAttach
		case CSIControllerSupportsExpand:
			return c.ControllerInfo.SupportsExpand
		case CSIControllerSupportsListVolumesAttachedNodes:
			return c.ControllerInfo.SupportsListVolumesAttachedNodes
		case CSIControllerSupportsCondition:
			return c.ControllerInfo.SupportsCondition
		case CSIControllerSupportsGet:
			return c.ControllerInfo.SupportsGet
		default:
			return false
		}
	}
	return false
}

func (p *CSIPlugin) HasNodeCapability(cap CSINodeCapability) bool {
	if len(p.Nodes) < 1 {
		return false
	}
	// we're picking the first node because they should be uniform
	// across the same version of the plugin
	for _, c := range p.Nodes {
		switch cap {
		case CSINodeSupportsStageVolume:
			return c.NodeInfo.RequiresNodeStageVolume
		case CSINodeSupportsStats:
			return c.NodeInfo.SupportsStats
		case CSINodeSupportsExpand:
			return c.NodeInfo.SupportsExpand
		case CSINodeSupportsCondition:
			return c.NodeInfo.SupportsCondition
		default:
			return false
		}
	}
	return false
}

// AddPlugin adds a single plugin running on the node. Called from state.NodeUpdate in a
// transaction
func (p *CSIPlugin) AddPlugin(nodeID string, info *CSIInfo) error {
	if info.ControllerInfo != nil {
		p.ControllerRequired = info.RequiresControllerPlugin
		prev, ok := p.Controllers[nodeID]
		if ok {
			if prev == nil {
				return fmt.Errorf("plugin missing controller: %s", nodeID)
			}
			if prev.Healthy {
				p.ControllersHealthy -= 1
			}
		}

		// note: for this to work as expected, only a single
		// controller for a given plugin can be on a given Nomad
		// client, they also conflict on the client so this should be
		// ok
		if prev != nil || info.Healthy {
			p.Controllers[nodeID] = info
		}
		if info.Healthy {
			p.ControllersHealthy += 1
		}
	}

	if info.NodeInfo != nil {
		prev, ok := p.Nodes[nodeID]
		if ok {
			if prev == nil {
				return fmt.Errorf("plugin missing node: %s", nodeID)
			}
			if prev.Healthy {
				p.NodesHealthy -= 1
			}
		}
		if prev != nil || info.Healthy {
			p.Nodes[nodeID] = info
		}
		if info.Healthy {
			p.NodesHealthy += 1
		}
	}

	return nil
}

// DeleteNode removes all plugins from the node. Called from state.DeleteNode in a
// transaction
func (p *CSIPlugin) DeleteNode(nodeID string) error {
	return p.DeleteNodeForType(nodeID, CSIPluginTypeMonolith)
}

// DeleteNodeForType deletes a client node from the list of controllers or node instance of
// a plugin. Called from deleteJobFromPlugin during job deregistration, in a transaction
func (p *CSIPlugin) DeleteNodeForType(nodeID string, pluginType CSIPluginType) error {
	switch pluginType {
	case CSIPluginTypeController:
		if prev, ok := p.Controllers[nodeID]; ok {
			if prev == nil {
				return fmt.Errorf("plugin missing controller: %s", nodeID)
			}
			if prev.Healthy {
				p.ControllersHealthy--
			}
			delete(p.Controllers, nodeID)
		}

	case CSIPluginTypeNode:
		if prev, ok := p.Nodes[nodeID]; ok {
			if prev == nil {
				return fmt.Errorf("plugin missing node: %s", nodeID)
			}
			if prev.Healthy {
				p.NodesHealthy--
			}
			delete(p.Nodes, nodeID)
		}

	case CSIPluginTypeMonolith:
		var result error

		err := p.DeleteNodeForType(nodeID, CSIPluginTypeController)
		if err != nil {
			result = multierror.Append(result, err)
		}

		err = p.DeleteNodeForType(nodeID, CSIPluginTypeNode)
		if err != nil {
			result = multierror.Append(result, err)
		}

		return result
	}

	return nil
}

// DeleteAlloc removes the fingerprint info for the allocation
func (p *CSIPlugin) DeleteAlloc(allocID, nodeID string) error {
	prev, ok := p.Controllers[nodeID]
	if ok {
		if prev == nil {
			return fmt.Errorf("plugin missing controller: %s", nodeID)
		}
		if prev.AllocID == allocID {
			if prev.Healthy {
				p.ControllersHealthy -= 1
			}
			delete(p.Controllers, nodeID)
		}
	}

	prev, ok = p.Nodes[nodeID]
	if ok {
		if prev == nil {
			return fmt.Errorf("plugin missing node: %s", nodeID)
		}
		if prev.AllocID == allocID {
			if prev.Healthy {
				p.NodesHealthy -= 1
			}
			delete(p.Nodes, nodeID)
		}
	}

	return nil
}

// AddJob adds a job to the plugin and increments expected
func (p *CSIPlugin) AddJob(job *Job, summary *JobSummary) {
	p.UpdateExpectedWithJob(job, summary, false)
}

// DeleteJob removes the job from the plugin and decrements expected
func (p *CSIPlugin) DeleteJob(job *Job, summary *JobSummary) {
	p.UpdateExpectedWithJob(job, summary, true)
}

// UpdateExpectedWithJob maintains the expected instance count
// we use the summary to add non-allocation expected counts
func (p *CSIPlugin) UpdateExpectedWithJob(job *Job, summary *JobSummary, terminal bool) {
	var count int

	for _, tg := range job.TaskGroups {
		if job.Type == JobTypeSystem {
			if summary == nil {
				continue
			}

			s, ok := summary.Summary[tg.Name]
			if !ok {
				continue
			}

			count = s.Running + s.Queued + s.Starting
		} else {
			count = tg.Count
		}

		for _, t := range tg.Tasks {
			if t.CSIPluginConfig == nil ||
				t.CSIPluginConfig.ID != p.ID {
				continue
			}

			// Change the correct plugin expected, monolith should change both
			if t.CSIPluginConfig.Type == CSIPluginTypeController ||
				t.CSIPluginConfig.Type == CSIPluginTypeMonolith {
				if terminal {
					p.ControllerJobs.Delete(job)
				} else {
					p.ControllerJobs.Add(job, count)
				}
			}

			if t.CSIPluginConfig.Type == CSIPluginTypeNode ||
				t.CSIPluginConfig.Type == CSIPluginTypeMonolith {
				if terminal {
					p.NodeJobs.Delete(job)
				} else {
					p.NodeJobs.Add(job, count)
				}
			}
		}
	}

	p.ControllersExpected = p.ControllerJobs.Count()
	p.NodesExpected = p.NodeJobs.Count()
}

// JobDescription records Job identification and the count of expected plugin instances
type JobDescription struct {
	Namespace string
	ID        string
	Expected  int
}

// JobNamespacedDescriptions maps Job.ID to JobDescription
type JobNamespacedDescriptions map[string]JobDescription

func (j JobNamespacedDescriptions) Copy() JobNamespacedDescriptions {
	copy := JobNamespacedDescriptions{}
	for k, v := range j {
		copy[k] = v
	}
	return copy
}

// JobDescriptions maps Namespace to a mapping of Job.ID to JobDescription
type JobDescriptions map[string]JobNamespacedDescriptions

// Add the Job to the JobDescriptions, creating maps as necessary
func (j JobDescriptions) Add(job *Job, expected int) {
	if j == nil {
		j = make(JobDescriptions)
	}
	if j[job.Namespace] == nil {
		j[job.Namespace] = make(JobNamespacedDescriptions)
	}
	j[job.Namespace][job.ID] = JobDescription{
		Namespace: job.Namespace,
		ID:        job.ID,
		Expected:  expected,
	}
}

// Count the Expected instances for all JobDescriptions
func (j JobDescriptions) Count() int {
	if j == nil {
		return 0
	}
	count := 0
	for _, jnd := range j {
		for _, jd := range jnd {
			count += jd.Expected
		}
	}
	return count
}

// Delete the Job from the JobDescriptions
func (j JobDescriptions) Delete(job *Job) {
	if j != nil &&
		j[job.Namespace] != nil {
		delete(j[job.Namespace], job.ID)
	}
}

type CSIPluginListStub struct {
	ID                  string
	Provider            string
	ControllerRequired  bool
	ControllersHealthy  int
	ControllersExpected int
	NodesHealthy        int
	NodesExpected       int
	CreateIndex         uint64
	ModifyIndex         uint64
}

func (p *CSIPlugin) Stub() *CSIPluginListStub {
	return &CSIPluginListStub{
		ID:                  p.ID,
		Provider:            p.Provider,
		ControllerRequired:  p.ControllerRequired,
		ControllersHealthy:  p.ControllersHealthy,
		ControllersExpected: p.ControllersExpected,
		NodesHealthy:        p.NodesHealthy,
		NodesExpected:       p.NodesExpected,
		CreateIndex:         p.CreateIndex,
		ModifyIndex:         p.ModifyIndex,
	}
}

func (p *CSIPlugin) IsEmpty() bool {
	return p == nil ||
		len(p.Controllers) == 0 &&
			len(p.Nodes) == 0 &&
			p.ControllerJobs.Count() == 0 &&
			p.NodeJobs.Count() == 0
}

type CSIPluginListRequest struct {
	QueryOptions
}

type CSIPluginListResponse struct {
	Plugins []*CSIPluginListStub
	QueryMeta
}

type CSIPluginGetRequest struct {
	ID string
	QueryOptions
}

type CSIPluginGetResponse struct {
	Plugin *CSIPlugin
	QueryMeta
}

type CSIPluginDeleteRequest struct {
	ID string
	QueryOptions
}

type CSIPluginDeleteResponse struct {
	QueryMeta
}
