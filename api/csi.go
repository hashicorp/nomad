package api

import (
	"sort"
	"time"
)

// CSIVolumes is used to query the top level csi volumes
type CSIVolumes struct {
	client *Client
}

// CSIVolumes returns a handle on the CSIVolumes endpoint
func (c *Client) CSIVolumes() *CSIVolumes {
	return &CSIVolumes{client: c}
}

// List returns all CSI volumes
func (v *CSIVolumes) List(q *QueryOptions) ([]*CSIVolumeListStub, *QueryMeta, error) {
	var resp []*CSIVolumeListStub
	qm, err := v.client.query("/v1/csi/volumes", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	sort.Sort(CSIVolumeIndexSort(resp))
	return resp, qm, nil
}

// PluginList returns all CSI volumes for the specified plugin id
func (v *CSIVolumes) PluginList(pluginID string) ([]*CSIVolumeListStub, *QueryMeta, error) {
	return v.List(&QueryOptions{Prefix: pluginID})
}

// Info is used to retrieve a single CSIVolume
func (v *CSIVolumes) Info(id string, q *QueryOptions) (*CSIVolume, *QueryMeta, error) {
	var resp CSIVolume
	qm, err := v.client.query("/v1/csi/volume/"+id, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return &resp, qm, nil
}

func (v *CSIVolumes) Register(vol *CSIVolume, w *WriteOptions) (*WriteMeta, error) {
	req := CSIVolumeRegisterRequest{
		Volumes: []*CSIVolume{vol},
	}
	meta, err := v.client.write("/v1/csi/volume/"+vol.ID, req, nil, w)
	return meta, err
}

func (v *CSIVolumes) Deregister(id string, w *WriteOptions) error {
	_, err := v.client.delete("/v1/csi/volume/"+id, nil, w)
	return err
}

// CSIVolumeAttachmentMode duplicated in nomad/structs/csi.go
type CSIVolumeAttachmentMode string

const (
	CSIVolumeAttachmentModeUnknown     CSIVolumeAttachmentMode = ""
	CSIVolumeAttachmentModeBlockDevice CSIVolumeAttachmentMode = "block-device"
	CSIVolumeAttachmentModeFilesystem  CSIVolumeAttachmentMode = "file-system"
)

// CSIVolumeAccessMode duplicated in nomad/structs/csi.go
type CSIVolumeAccessMode string

const (
	CSIVolumeAccessModeUnknown CSIVolumeAccessMode = ""

	CSIVolumeAccessModeSingleNodeReader CSIVolumeAccessMode = "single-node-reader-only"
	CSIVolumeAccessModeSingleNodeWriter CSIVolumeAccessMode = "single-node-writer"

	CSIVolumeAccessModeMultiNodeReader       CSIVolumeAccessMode = "multi-node-reader-only"
	CSIVolumeAccessModeMultiNodeSingleWriter CSIVolumeAccessMode = "multi-node-single-writer"
	CSIVolumeAccessModeMultiNodeMultiWriter  CSIVolumeAccessMode = "multi-node-multi-writer"
)

// CSIVolume is used for serialization, see also nomad/structs/csi.go
type CSIVolume struct {
	ID             string
	Namespace      string
	Topologies     []*CSITopology
	AccessMode     CSIVolumeAccessMode
	AttachmentMode CSIVolumeAttachmentMode

	// Combine structs.{Read,Write,Past}Allocs
	Allocations []*AllocationListStub

	// Healthy is true iff all the denormalized plugin health fields are true, and the
	// volume has not been marked for garbage collection
	Healthy             bool
	VolumeGC            time.Time
	PluginID            string
	ControllersHealthy  int
	ControllersExpected int
	NodesHealthy        int
	NodesExpected       int
	ResourceExhausted   time.Time

	CreateIndex uint64
	ModifyIndex uint64
}

type CSIVolumeIndexSort []*CSIVolumeListStub

func (v CSIVolumeIndexSort) Len() int {
	return len(v)
}

func (v CSIVolumeIndexSort) Less(i, j int) bool {
	return v[i].CreateIndex > v[j].CreateIndex
}

func (v CSIVolumeIndexSort) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}

// CSIVolumeListStub omits allocations. See also nomad/structs/csi.go
type CSIVolumeListStub struct {
	ID             string
	Namespace      string
	Topologies     []*CSITopology
	AccessMode     CSIVolumeAccessMode
	AttachmentMode CSIVolumeAttachmentMode

	// Healthy is true iff all the denormalized plugin health fields are true, and the
	// volume has not been marked for garbage collection
	Healthy             bool
	VolumeGC            time.Time
	PluginID            string
	ControllersHealthy  int
	ControllersExpected int
	NodesHealthy        int
	NodesExpected       int
	ResourceExhausted   time.Time

	CreateIndex uint64
	ModifyIndex uint64
}

type CSIVolumeRegisterRequest struct {
	Volumes []*CSIVolume
	WriteRequest
}

type CSIVolumeDeregisterRequest struct {
	VolumeIDs []string
	WriteRequest
}

// CSI Plugins are jobs with plugin specific data
type CSIPlugins struct {
	client *Client
}

type CSIPlugin struct {
	ID string
	// Map Node.ID to CSIInfo fingerprint results
	Controllers        map[string]*CSIInfo
	Nodes              map[string]*CSIInfo
	ControllersHealthy int
	NodesHealthy       int
	CreateIndex        uint64
	ModifyIndex        uint64
}

type CSIPluginListStub struct {
	ID                  string
	ControllersHealthy  int
	ControllersExpected int
	NodesHealthy        int
	NodesExpected       int
	CreateIndex         uint64
	ModifyIndex         uint64
}

type CSIPluginIndexSort []*CSIPluginListStub

func (v CSIPluginIndexSort) Len() int {
	return len(v)
}

func (v CSIPluginIndexSort) Less(i, j int) bool {
	return v[i].CreateIndex > v[j].CreateIndex
}

func (v CSIPluginIndexSort) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}

// CSIPlugins returns a handle on the CSIPlugins endpoint
func (c *Client) CSIPlugins() *CSIPlugins {
	return &CSIPlugins{client: c}
}

// List returns all CSI plugins
func (v *CSIPlugins) List(q *QueryOptions) ([]*CSIPluginListStub, *QueryMeta, error) {
	var resp []*CSIPluginListStub
	qm, err := v.client.query("/v1/csi/plugins", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	sort.Sort(CSIPluginIndexSort(resp))
	return resp, qm, nil
}

// Info is used to retrieve a single CSI Plugin Job
func (v *CSIPlugins) Info(id string, q *QueryOptions) (*CSIPlugin, *QueryMeta, error) {
	var resp *CSIPlugin
	qm, err := v.client.query("/v1/csi/plugin/"+id, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}
