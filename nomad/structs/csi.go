package structs

import (
	"fmt"
	"strings"
	"time"
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
	return &(*o)
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

// VolumeMountOptions implements the Stringer and GoStringer interfaces to prevent
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

	// Allocations, tracking claim status
	ReadAllocs  map[string]*Allocation
	WriteAllocs map[string]*Allocation

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
func NewCSIVolume(pluginID string, index uint64) *CSIVolume {
	out := &CSIVolume{
		ID:          pluginID,
		CreateIndex: index,
		ModifyIndex: index,
	}

	out.newStructs()
	return out
}

func (v *CSIVolume) newStructs() {
	if v.Topologies == nil {
		v.Topologies = []*CSITopology{}
	}

	v.ReadAllocs = map[string]*Allocation{}
	v.WriteAllocs = map[string]*Allocation{}
}

func (v *CSIVolume) RemoteID() string {
	if v.ExternalID != "" {
		return v.ExternalID
	}
	return v.ID
}

func (v *CSIVolume) Stub() *CSIVolListStub {
	stub := CSIVolListStub{
		ID:                 v.ID,
		Namespace:          v.Namespace,
		Name:               v.Name,
		ExternalID:         v.ExternalID,
		Topologies:         v.Topologies,
		AccessMode:         v.AccessMode,
		AttachmentMode:     v.AttachmentMode,
		CurrentReaders:     len(v.ReadAllocs),
		CurrentWriters:     len(v.WriteAllocs),
		Schedulable:        v.Schedulable,
		PluginID:           v.PluginID,
		Provider:           v.Provider,
		ControllersHealthy: v.ControllersHealthy,
		NodesHealthy:       v.NodesHealthy,
		NodesExpected:      v.NodesExpected,
		CreateIndex:        v.CreateIndex,
		ModifyIndex:        v.ModifyIndex,
	}

	return &stub
}

func (v *CSIVolume) CanReadOnly() bool {
	if !v.Schedulable {
		return false
	}

	return v.ResourceExhausted == time.Time{}
}

func (v *CSIVolume) CanWrite() bool {
	if !v.Schedulable {
		return false
	}

	switch v.AccessMode {
	case CSIVolumeAccessModeSingleNodeWriter, CSIVolumeAccessModeMultiNodeSingleWriter:
		return len(v.WriteAllocs) == 0
	case CSIVolumeAccessModeMultiNodeMultiWriter:
		return v.ResourceExhausted == time.Time{}
	default:
		return false
	}
}

// Copy returns a copy of the volume, which shares only the Topologies slice
func (v *CSIVolume) Copy() *CSIVolume {
	copy := *v
	out := &copy
	out.newStructs()

	for k, v := range v.ReadAllocs {
		out.ReadAllocs[k] = v
	}

	for k, v := range v.WriteAllocs {
		out.WriteAllocs[k] = v
	}

	return out
}

// Claim updates the allocations and changes the volume state
func (v *CSIVolume) Claim(claim CSIVolumeClaimMode, alloc *Allocation) bool {
	switch claim {
	case CSIVolumeClaimRead:
		return v.ClaimRead(alloc)
	case CSIVolumeClaimWrite:
		return v.ClaimWrite(alloc)
	case CSIVolumeClaimRelease:
		return v.ClaimRelease(alloc)
	}
	return false
}

// ClaimRead marks an allocation as using a volume read-only
func (v *CSIVolume) ClaimRead(alloc *Allocation) bool {
	if _, ok := v.ReadAllocs[alloc.ID]; ok {
		return true
	}

	if !v.CanReadOnly() {
		return false
	}
	// Allocations are copy on write, so we want to keep the id but don't need the
	// pointer. We'll get it from the db in denormalize.
	v.ReadAllocs[alloc.ID] = nil
	delete(v.WriteAllocs, alloc.ID)
	return true
}

// ClaimWrite marks an allocation as using a volume as a writer
func (v *CSIVolume) ClaimWrite(alloc *Allocation) bool {
	if _, ok := v.WriteAllocs[alloc.ID]; ok {
		return true
	}

	if !v.CanWrite() {
		return false
	}
	// Allocations are copy on write, so we want to keep the id but don't need the
	// pointer. We'll get it from the db in denormalize.
	v.WriteAllocs[alloc.ID] = nil
	delete(v.ReadAllocs, alloc.ID)
	return true
}

// ClaimRelease is called when the allocation has terminated and already stopped using the volume
func (v *CSIVolume) ClaimRelease(alloc *Allocation) bool {
	delete(v.ReadAllocs, alloc.ID)
	delete(v.WriteAllocs, alloc.ID)
	return true
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
	WriteRequest
}

type CSIVolumeDeregisterResponse struct {
	QueryMeta
}

type CSIVolumeClaimMode int

const (
	CSIVolumeClaimRead CSIVolumeClaimMode = iota
	CSIVolumeClaimWrite
	CSIVolumeClaimRelease
)

type CSIVolumeClaimRequest struct {
	VolumeID     string
	AllocationID string
	Claim        CSIVolumeClaimMode
	WriteRequest
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

	// Cache the count of healthy plugins
	ControllersHealthy int
	NodesHealthy       int

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
}

func (p *CSIPlugin) Copy() *CSIPlugin {
	copy := *p
	out := &copy
	out.newStructs()

	for k, v := range p.Controllers {
		out.Controllers[k] = v
	}

	for k, v := range p.Nodes {
		out.Nodes[k] = v
	}

	return out
}

// AddPlugin adds a single plugin running on the node. Called from state.NodeUpdate in a
// transaction
func (p *CSIPlugin) AddPlugin(nodeID string, info *CSIInfo) {
	if info.ControllerInfo != nil {
		p.ControllerRequired = info.RequiresControllerPlugin &&
			info.ControllerInfo.SupportsAttachDetach

		prev, ok := p.Controllers[nodeID]
		if ok && prev.Healthy {
			p.ControllersHealthy -= 1
		}
		p.Controllers[nodeID] = info
		if info.Healthy {
			p.ControllersHealthy += 1
		}
	}

	if info.NodeInfo != nil {
		prev, ok := p.Nodes[nodeID]
		if ok && prev.Healthy {
			p.NodesHealthy -= 1
		}
		p.Nodes[nodeID] = info
		if info.Healthy {
			p.NodesHealthy += 1
		}
	}
}

// DeleteNode removes all plugins from the node. Called from state.DeleteNode in a
// transaction
func (p *CSIPlugin) DeleteNode(nodeID string) {
	prev, ok := p.Controllers[nodeID]
	if ok && prev.Healthy {
		p.ControllersHealthy -= 1
	}
	delete(p.Controllers, nodeID)

	prev, ok = p.Nodes[nodeID]
	if ok && prev.Healthy {
		p.NodesHealthy -= 1
	}
	delete(p.Nodes, nodeID)
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
		ControllersExpected: len(p.Controllers),
		NodesHealthy:        p.NodesHealthy,
		NodesExpected:       len(p.Nodes),
		CreateIndex:         p.CreateIndex,
		ModifyIndex:         p.ModifyIndex,
	}
}

func (p *CSIPlugin) IsEmpty() bool {
	return len(p.Controllers) == 0 && len(p.Nodes) == 0
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
