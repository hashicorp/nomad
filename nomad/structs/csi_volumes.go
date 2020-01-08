package structs

import (
	"fmt"
	"strings"
	"time"
)

const (
	VolumeTypeCSI = "csi"
)

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

type CSIVolume struct {
	ID             string
	Driver         string
	Namespace      string
	Topologies     []*CSITopology
	AccessMode     CSIVolumeAccessMode
	AttachmentMode CSIVolumeAttachmentMode

	// Allocations, tracking claim status
	ReadAllocs  map[string]*Allocation
	WriteAllocs map[string]*Allocation
	PastAllocs  map[string]*Allocation

	// Healthy is true iff all the denormalized plugin health fields are true, and the
	// volume has not been marked for garbage collection
	Healthy           bool
	VolumeGC          time.Time
	ControllerName    string
	ControllerHealthy bool
	Controller        []*Job
	NodeHealthy       int
	NodeExpected      int
	ResourceExhausted time.Time

	CreatedIndex  uint64
	ModifiedIndex uint64
}

type CSIVolListStub struct {
	ID                string
	Driver            string
	Namespace         string
	Topologies        []*CSITopology
	AccessMode        CSIVolumeAccessMode
	AttachmentMode    CSIVolumeAttachmentMode
	CurrentReaders    int
	CurrentWriters    int
	Healthy           bool
	VolumeGC          time.Time
	ControllerName    string
	ControllerHealthy bool
	NodeHealthy       int
	NodeExpected      int
	CreatedIndex      uint64
	ModifiedIndex     uint64
}

func CreateCSIVolume(controllerName string) *CSIVolume {
	return &CSIVolume{
		ControllerName: controllerName,
		ReadAllocs:     map[string]*Allocation{},
		WriteAllocs:    map[string]*Allocation{},
		PastAllocs:     map[string]*Allocation{},
		Topologies:     []*CSITopology{},
	}
}

func (v *CSIVolume) Stub() *CSIVolListStub {
	stub := CSIVolListStub{
		ID:                v.ID,
		Driver:            v.Driver,
		Namespace:         v.Namespace,
		Topologies:        v.Topologies,
		AccessMode:        v.AccessMode,
		AttachmentMode:    v.AttachmentMode,
		CurrentReaders:    len(v.ReadAllocs),
		CurrentWriters:    len(v.WriteAllocs),
		Healthy:           v.Healthy,
		VolumeGC:          v.VolumeGC,
		ControllerName:    v.ControllerName,
		ControllerHealthy: v.ControllerHealthy,
		NodeHealthy:       v.NodeHealthy,
		NodeExpected:      v.NodeExpected,
		CreatedIndex:      v.CreatedIndex,
		ModifiedIndex:     v.ModifiedIndex,
	}

	return &stub
}

func (v *CSIVolume) CanReadOnly() bool {
	if !v.Healthy {
		return false
	}

	return v.ResourceExhausted == time.Time{}
}

func (v *CSIVolume) CanWrite() bool {
	if !v.Healthy {
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

func (v *CSIVolume) ClaimRead(alloc *Allocation) bool {
	if !v.CanReadOnly() {
		return false
	}
	v.ReadAllocs[alloc.ID] = alloc
	delete(v.WriteAllocs, alloc.ID)
	delete(v.PastAllocs, alloc.ID)
	return true
}

func (v *CSIVolume) ClaimWrite(alloc *Allocation) bool {
	if !v.CanWrite() {
		return false
	}
	v.WriteAllocs[alloc.ID] = alloc
	delete(v.ReadAllocs, alloc.ID)
	delete(v.PastAllocs, alloc.ID)
	return true
}

func (v *CSIVolume) ClaimRelease(alloc *Allocation) bool {
	delete(v.ReadAllocs, alloc.ID)
	delete(v.WriteAllocs, alloc.ID)
	v.PastAllocs[alloc.ID] = alloc
	return true
}

// GCAlloc is called on Allocation gc, by following the alloc's pointer back to the volume
func (v *CSIVolume) GCAlloc(alloc *Allocation) {
	delete(v.ReadAllocs, alloc.ID)
	delete(v.WriteAllocs, alloc.ID)
	delete(v.PastAllocs, alloc.ID)
}

// Equality by value
func (v *CSIVolume) Equal(o *CSIVolume) bool {
	if v == nil || o == nil {
		return v == o
	}

	// Omit the plugin health fields, their values are controlled by plugin jobs
	if v.ID == o.ID &&
		v.Driver == o.Driver &&
		v.Namespace == o.Namespace &&
		v.AccessMode == o.AccessMode &&
		v.AttachmentMode == o.AttachmentMode &&
		v.ControllerName == o.ControllerName {
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
	if v.Driver == "" {
		errs = append(errs, "missing driver")
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

	var ok bool
	for _, t := range v.Topologies {
		if t != nil && len(t.Segments) > 0 {
			ok = true
			break
		}
	}
	if !ok {
		errs = append(errs, "missing topology")
	}

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
	VolumeID   string
	Allocation *Allocation
	Claim      CSIVolumeClaimMode
	WriteRequest
}

type CSIVolumeListRequest struct {
	Driver string
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
