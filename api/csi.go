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
	qm, err := v.client.query("/v1/volumes?type=csi", &resp, q)
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
	qm, err := v.client.query("/v1/volume/csi/"+id, &resp, q)
	if err != nil {
		return nil, nil, err
	}

	// Cleanup allocation representation for the ui
	resp.allocs()

	return &resp, qm, nil
}

func (v *CSIVolumes) Register(vol *CSIVolume, w *WriteOptions) (*WriteMeta, error) {
	req := CSIVolumeRegisterRequest{
		Volumes: []*CSIVolume{vol},
	}
	meta, err := v.client.write("/v1/volume/csi/"+vol.ID, req, nil, w)
	return meta, err
}

func (v *CSIVolumes) Deregister(id string, w *WriteOptions) error {
	_, err := v.client.delete("/v1/volume/csi/"+id, nil, w)
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

type CSIOptions struct {
	FSType     string
	MountFlags []string
}

// CSIVolume is used for serialization, see also nomad/structs/csi.go
type CSIVolume struct {
	ID             string
	Name           string
	ExternalID     string `hcl:"external_id"`
	Namespace      string
	Topologies     []*CSITopology
	AccessMode     CSIVolumeAccessMode     `hcl:"access_mode"`
	AttachmentMode CSIVolumeAttachmentMode `hcl:"attachment_mode"`
	Options        *CSIOptions

	// Allocations, tracking claim status
	ReadAllocs  map[string]*Allocation
	WriteAllocs map[string]*Allocation

	// Combine structs.{Read,Write}Allocs
	Allocations []*AllocationListStub

	// Schedulable is true if all the denormalized plugin health fields are true
	Schedulable         bool
	PluginID            string `hcl:"plugin_id"`
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

	// ExtraKeysHCL is used by the hcl parser to report unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

// allocs is called after we query the volume (creating this CSIVolume struct) to collapse
// allocations for the UI
func (v *CSIVolume) allocs() {
	for _, a := range v.WriteAllocs {
		v.Allocations = append(v.Allocations, a.Stub())
	}
	for _, a := range v.ReadAllocs {
		v.Allocations = append(v.Allocations, a.Stub())
	}
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
	ID                  string
	Namespace           string
	Name                string
	ExternalID          string
	Topologies          []*CSITopology
	AccessMode          CSIVolumeAccessMode
	AttachmentMode      CSIVolumeAttachmentMode
	Schedulable         bool
	PluginID            string
	Provider            string
	ControllerRequired  bool
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
	ID                 string
	Provider           string
	Version            string
	ControllerRequired bool
	// Map Node.ID to CSIInfo fingerprint results
	Controllers        map[string]*CSIInfo
	Nodes              map[string]*CSIInfo
	Allocations        []*AllocationListStub
	ControllersHealthy int
	NodesHealthy       int
	CreateIndex        uint64
	ModifyIndex        uint64
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
	qm, err := v.client.query("/v1/plugins?type=csi", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	sort.Sort(CSIPluginIndexSort(resp))
	return resp, qm, nil
}

// Info is used to retrieve a single CSI Plugin Job
func (v *CSIPlugins) Info(id string, q *QueryOptions) (*CSIPlugin, *QueryMeta, error) {
	var resp *CSIPlugin
	qm, err := v.client.query("/v1/plugin/csi/"+id, &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}
