package structs

import (
	"fmt"
	"strings"
	"time"

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

// ValidCSIVolumeAccessMode checks for a writable access mode
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

// CSIMountOptions implements the Stringer and GoStringer interfaces to prevent
// accidental leakage of sensitive mount flags via logs.
var _ fmt.Stringer = &CSIMountOptions{}
var _ fmt.GoStringer = &CSIMountOptions{}

func (v *CSIMountOptions) String() string {
	mountFlagsString := "nil"
	if len(v.MountFlags) != 0 {
		mountFlagsString = "[REDACTED]"
	}

	return fmt.Sprintf("csi.CSIOptions(FSType: %s, MountFlags: %s)", v.FSType, mountFlagsString)
}

func (v *CSIMountOptions) GoString() string {
	return v.String()
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
	ExternalID     string
	Namespace      string
	Topologies     []*CSITopology
	AccessMode     CSIVolumeAccessMode
	AttachmentMode CSIVolumeAttachmentMode
	MountOptions   *CSIMountOptions
	Secrets        CSISecrets
	Parameters     map[string]string
	Context        map[string]string

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

func (v *CSIVolume) ReadSchedulable() bool {
	if !v.Schedulable {
		return false
	}

	return v.ResourceExhausted == time.Time{}
}

// WriteSchedulable determines if the volume is schedulable for writes, considering only
// volume health
func (v *CSIVolume) WriteSchedulable() bool {
	if !v.Schedulable {
		return false
	}

	switch v.AccessMode {
	case CSIVolumeAccessModeSingleNodeWriter, CSIVolumeAccessModeMultiNodeSingleWriter, CSIVolumeAccessModeMultiNodeMultiWriter:
		return v.ResourceExhausted == time.Time{}
	default:
		return false
	}
}

// WriteFreeClaims determines if there are any free write claims available
func (v *CSIVolume) WriteFreeClaims() bool {
	switch v.AccessMode {
	case CSIVolumeAccessModeSingleNodeWriter, CSIVolumeAccessModeMultiNodeSingleWriter:
		return len(v.WriteClaims) == 0
	case CSIVolumeAccessModeMultiNodeMultiWriter:
		// the CSI spec doesn't allow for setting a max number of writers.
		// we track node resource exhaustion through v.ResourceExhausted
		// which is checked in WriteSchedulable
		return true
	default:
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

	if claim.State == CSIVolumeClaimStateTaken {
		switch claim.Mode {
		case CSIVolumeClaimRead:
			return v.ClaimRead(claim, alloc)
		case CSIVolumeClaimWrite:
			return v.ClaimWrite(claim, alloc)
		}
	}
	// either GC or a Unpublish checkpoint
	return v.ClaimRelease(claim)
}

// ClaimRead marks an allocation as using a volume read-only
func (v *CSIVolume) ClaimRead(claim *CSIVolumeClaim, alloc *Allocation) error {
	if _, ok := v.ReadAllocs[claim.AllocationID]; ok {
		return nil
	}
	if alloc == nil {
		return fmt.Errorf("allocation missing: %s", claim.AllocationID)
	}

	if !v.ReadSchedulable() {
		return fmt.Errorf("unschedulable")
	}

	// Allocations are copy on write, so we want to keep the id but don't need the
	// pointer. We'll get it from the db in denormalize.
	v.ReadAllocs[claim.AllocationID] = nil
	delete(v.WriteAllocs, claim.AllocationID)

	v.ReadClaims[claim.AllocationID] = claim
	delete(v.WriteClaims, claim.AllocationID)
	delete(v.PastClaims, claim.AllocationID)

	return nil
}

// ClaimWrite marks an allocation as using a volume as a writer
func (v *CSIVolume) ClaimWrite(claim *CSIVolumeClaim, alloc *Allocation) error {
	if _, ok := v.WriteAllocs[claim.AllocationID]; ok {
		return nil
	}
	if alloc == nil {
		return fmt.Errorf("allocation missing: %s", claim.AllocationID)
	}

	if !v.WriteSchedulable() {
		return fmt.Errorf("unschedulable")
	}

	if !v.WriteFreeClaims() {
		// Check the blocking allocations to see if they belong to this job
		for _, a := range v.WriteAllocs {
			if a != nil && (a.Namespace != alloc.Namespace || a.JobID != alloc.JobID) {
				return fmt.Errorf("volume max claim reached")
			}
		}
	}

	// Allocations are copy on write, so we want to keep the id but don't need the
	// pointer. We'll get it from the db in denormalize.
	v.WriteAllocs[alloc.ID] = nil
	delete(v.ReadAllocs, alloc.ID)

	v.WriteClaims[alloc.ID] = claim
	delete(v.ReadClaims, alloc.ID)
	delete(v.PastClaims, alloc.ID)

	return nil
}

// ClaimRelease is called when the allocation has terminated and
// already stopped using the volume
func (v *CSIVolume) ClaimRelease(claim *CSIVolumeClaim) error {
	if claim.State == CSIVolumeClaimStateReadyToFree {
		delete(v.ReadAllocs, claim.AllocationID)
		delete(v.WriteAllocs, claim.AllocationID)
		delete(v.ReadClaims, claim.AllocationID)
		delete(v.WriteClaims, claim.AllocationID)
		delete(v.PastClaims, claim.AllocationID)
	} else {
		v.PastClaims[claim.AllocationID] = claim
	}
	return nil
}

// Equality by value
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
	if v.AccessMode == "" {
		errs = append(errs, "missing access mode")
	}
	if v.AttachmentMode == "" {
		errs = append(errs, "missing attachment mode")
	}
	if v.AttachmentMode == CSIVolumeAttachmentModeBlockDevice {
		if v.MountOptions != nil {
			if v.MountOptions.FSType != "" {
				errs = append(errs, "mount options not allowed for block-device")
			}
			if v.MountOptions.MountFlags != nil && len(v.MountOptions.MountFlags) != 0 {
				errs = append(errs, "mount options not allowed for block-device")
			}
		}
	}

	// TODO: Volume Topologies are optional - We should check to see if the plugin
	//       the volume is being registered with requires them.
	// var ok bool
	// for _, t := range v.Topologies {
	// 	if t != nil && len(t.Segments) > 0 {
	// 		ok = true
	// 		break
	// 	}
	// }
	// if !ok {
	// 	errs = append(errs, "missing topology")
	// }

	if len(errs) > 0 {
		return fmt.Errorf("validation: %s", strings.Join(errs, ", "))
	}
	return nil
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
	State          CSIVolumeClaimState
	WriteRequest
}

func (req *CSIVolumeClaimRequest) ToClaim() *CSIVolumeClaim {
	return &CSIVolumeClaim{
		AllocationID:   req.AllocationID,
		NodeID:         req.NodeID,
		ExternalNodeID: req.ExternalNodeID,
		Mode:           req.Claim,
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

// AddPlugin adds a single plugin running on the node. Called from state.NodeUpdate in a
// transaction
func (p *CSIPlugin) AddPlugin(nodeID string, info *CSIInfo) error {
	if info.ControllerInfo != nil {
		p.ControllerRequired = info.RequiresControllerPlugin &&
			(info.ControllerInfo.SupportsAttachDetach ||
				info.ControllerInfo.SupportsReadOnlyAttach)

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
		prev, ok := p.Controllers[nodeID]
		if ok {
			if prev == nil {
				return fmt.Errorf("plugin missing controller: %s", nodeID)
			}
			if prev.Healthy {
				p.ControllersHealthy -= 1
			}
		}
		delete(p.Controllers, nodeID)

	case CSIPluginTypeNode:
		prev, ok := p.Nodes[nodeID]
		if ok {
			if prev == nil {
				return fmt.Errorf("plugin missing node: %s", nodeID)
			}
			if prev.Healthy {
				p.NodesHealthy -= 1
			}
		}
		delete(p.Nodes, nodeID)

	case CSIPluginTypeMonolith:
		p.DeleteNodeForType(nodeID, CSIPluginTypeController)
		p.DeleteNodeForType(nodeID, CSIPluginTypeNode)
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
