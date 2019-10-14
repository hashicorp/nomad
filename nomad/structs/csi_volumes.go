package structs

import (
	"fmt"
	"strings"
	"time"
)

const (
	VolumeTypeCSI = "csi"
)

type CSIVolume struct {
	ID         string
	Driver     string
	Namespace  string
	Topology   *CSITopology
	MaxReaders int
	MaxWriters int

	// Allocations, tracking claim status
	ReadAllocs  map[string]*Allocation
	WriteAllocs map[string]*Allocation
	PastAllocs  map[string]*Allocation

	// Healthy is true iff all the denormalized plugin health fields are true, and the
	// volume has not been marked for garbage collection
	Healthy           bool
	VolumeGC          time.Time
	Controller        *Job
	ControllerHealthy bool
	NodeHealthy       int
	NodeExpected      int

	CreatedIndex  uint64
	ModifiedIndex uint64
}

type CSIVolListStub struct {
	ID                string
	Driver            string
	Namespace         string
	Topology          *CSITopology
	MaxReaders        int
	MaxWriters        int
	CurrentReaders    int
	CurrentWriters    int
	Healthy           bool
	VolumeGC          time.Time
	ControllerID      string
	ControllerHealthy bool
	NodeHealthy       int
	NodeExpected      int
	CreatedIndex      uint64
	ModifiedIndex     uint64
}

func CreateCSIVolume(controller *Job) *CSIVolume {
	return &CSIVolume{
		Controller:  controller,
		ReadAllocs:  map[string]*Allocation{},
		WriteAllocs: map[string]*Allocation{},
		PastAllocs:  map[string]*Allocation{},
		Topology:    &CSITopology{},
	}
}

func (v *CSIVolume) Stub() *CSIVolListStub {
	stub := CSIVolListStub{
		ID:             v.ID,
		Driver:         v.Driver,
		Namespace:      v.Namespace,
		Topology:       v.Topology,
		MaxReaders:     v.MaxReaders,
		MaxWriters:     v.MaxWriters,
		CurrentReaders: len(v.ReadAllocs),
		CurrentWriters: len(v.WriteAllocs),
		Healthy:        v.Healthy,
		VolumeGC:       v.VolumeGC,
		NodeHealthy:    v.NodeHealthy,
		NodeExpected:   v.NodeExpected,
		CreatedIndex:   v.CreatedIndex,
		ModifiedIndex:  v.ModifiedIndex,
	}

	if v.Controller != nil {
		stub.ControllerID = v.Controller.ID
		stub.ControllerHealthy = v.Controller.Status == JobStatusRunning
	}

	return &stub
}

func (v *CSIVolume) CanReadOnly() bool {
	if len(v.ReadAllocs) < v.MaxReaders {
		return true
	}
	return false
}

func (v *CSIVolume) CanWrite() bool {
	if len(v.WriteAllocs) < v.MaxWriters {
		return true
	}
	return false
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
	if o == nil {
		return false
	}

	// Omit the plugin health fields, their values are controlled by plugin jobs
	return v.ID == o.ID &&
		v.Driver == o.Driver &&
		v.Namespace == o.Namespace &&
		v.MaxReaders == o.MaxReaders &&
		v.MaxWriters == o.MaxWriters &&
		v.Controller == o.Controller &&
		v.Topology.Equal(o.Topology)
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
	if v.MaxReaders == 0 && v.MaxWriters == 0 {
		errs = append(errs, "missing access, set max readers and/or max writers")
	}
	if v.Topology == nil || len(v.Topology.Segments) == 0 {
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
