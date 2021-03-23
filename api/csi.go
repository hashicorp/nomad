package api

import (
	"fmt"
	"net/url"
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

// ListExternal returns all CSI volumes, as known to the storage provider
func (v *CSIVolumes) ListExternal(pluginID string, q *QueryOptions) (*CSIVolumeListExternalResponse, *QueryMeta, error) {
	var resp *CSIVolumeListExternalResponse

	qp := url.Values{}
	qp.Set("plugin_id", pluginID)
	if q.NextToken != "" {
		qp.Set("next_token", q.NextToken)
	}
	if q.PerPage != 0 {
		qp.Set("per_page", fmt.Sprint(q.PerPage))
	}

	qm, err := v.client.query("/v1/volumes/external?"+qp.Encode(), &resp, q)
	if err != nil {
		return nil, nil, err
	}

	sort.Sort(CSIVolumeExternalStubSort(resp.Volumes))
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

	return &resp, qm, nil
}

func (v *CSIVolumes) Register(vol *CSIVolume, w *WriteOptions) (*WriteMeta, error) {
	req := CSIVolumeRegisterRequest{
		Volumes: []*CSIVolume{vol},
	}
	meta, err := v.client.write("/v1/volume/csi/"+vol.ID, req, nil, w)
	return meta, err
}

func (v *CSIVolumes) Deregister(id string, force bool, w *WriteOptions) error {
	_, err := v.client.delete(fmt.Sprintf("/v1/volume/csi/%v?force=%t", url.PathEscape(id), force), nil, w)
	return err
}

func (v *CSIVolumes) Create(vol *CSIVolume, w *WriteOptions) ([]*CSIVolume, *WriteMeta, error) {
	req := CSIVolumeCreateRequest{
		Volumes: []*CSIVolume{vol},
	}

	resp := &CSIVolumeCreateResponse{}
	meta, err := v.client.write(fmt.Sprintf("/v1/volume/csi/%v/create", vol.ID), req, resp, w)
	return resp.Volumes, meta, err
}

func (v *CSIVolumes) Delete(externalVolID string, w *WriteOptions) error {
	_, err := v.client.delete(fmt.Sprintf("/v1/volume/csi/%v/delete", url.PathEscape(externalVolID)), nil, w)
	return err
}

func (v *CSIVolumes) Detach(volID, nodeID string, w *WriteOptions) error {
	_, err := v.client.delete(fmt.Sprintf("/v1/volume/csi/%v/detach?node=%v", url.PathEscape(volID), nodeID), nil, w)
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

type CSIMountOptions struct {
	FSType       string   `hcl:"fs_type,optional"`
	MountFlags   []string `hcl:"mount_flags,optional"`
	ExtraKeysHCL []string `hcl1:",unusedKeys" json:"-"` // report unexpected keys
}

type CSISecrets map[string]string

// CSIVolume is used for serialization, see also nomad/structs/csi.go
type CSIVolume struct {
	ID             string
	Name           string
	ExternalID     string `mapstructure:"external_id" hcl:"external_id"`
	Namespace      string
	Topologies     []*CSITopology
	AccessMode     CSIVolumeAccessMode     `hcl:"access_mode"`
	AttachmentMode CSIVolumeAttachmentMode `hcl:"attachment_mode"`
	MountOptions   *CSIMountOptions        `hcl:"mount_options"`
	Secrets        CSISecrets              `mapstructure:"secrets" hcl:"secrets"`
	Parameters     map[string]string       `mapstructure:"parameters" hcl:"parameters"`
	Context        map[string]string       `mapstructure:"context" hcl:"context"`
	Capacity       int64                   `hcl:"-"`

	// These fields are used as part of the volume creation request
	RequestedCapacityMin  int64                  `hcl:"capacity_min"`
	RequestedCapacityMax  int64                  `hcl:"capacity_max"`
	RequestedCapabilities []*CSIVolumeCapability `hcl:"capability"`
	CloneID               string                 `mapstructure:"clone_id" hcl:"clone_id"`
	SnapshotID            string                 `mapstructure:"snapshot_id" hcl:"snapshot_id"`

	// ReadAllocs is a map of allocation IDs for tracking reader claim status.
	// The Allocation value will always be nil; clients can populate this data
	// by iterating over the Allocations field.
	ReadAllocs map[string]*Allocation

	// WriteAllocs is a map of allocation IDs for tracking writer claim
	// status. The Allocation value will always be nil; clients can populate
	// this data by iterating over the Allocations field.
	WriteAllocs map[string]*Allocation

	// Allocations is a combined list of readers and writers
	Allocations []*AllocationListStub

	// Schedulable is true if all the denormalized plugin health fields are true
	Schedulable         bool
	PluginID            string `mapstructure:"plugin_id" hcl:"plugin_id"`
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
	ExtraKeysHCL []string `hcl1:",unusedKeys" json:"-"`
}

// CSIVolumeCapability is a requested attachment and access mode for a
// volume
type CSIVolumeCapability struct {
	AccessMode     CSIVolumeAccessMode     `mapstructure:"access_mode" hcl:"access_mode"`
	AttachmentMode CSIVolumeAttachmentMode `mapstructure:"attachment_mode" hcl:"attachment_mode"`
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

type CSIVolumeListExternalResponse struct {
	Volumes   []*CSIVolumeExternalStub
	NextToken string
}

// CSIVolumeExternalStub is the storage provider's view of a volume, as
// returned from the controller plugin; all IDs are for external resources
type CSIVolumeExternalStub struct {
	ExternalID               string
	CapacityBytes            int64
	VolumeContext            map[string]string
	CloneID                  string
	SnapshotID               string
	PublishedExternalNodeIDs []string
	IsAbnormal               bool
	Status                   string
}

// We can't sort external volumes by creation time because we don't get that
// data back from the storage provider. Sort by External ID within this page.
type CSIVolumeExternalStubSort []*CSIVolumeExternalStub

func (v CSIVolumeExternalStubSort) Len() int {
	return len(v)
}

func (v CSIVolumeExternalStubSort) Less(i, j int) bool {
	return v[i].ExternalID > v[j].ExternalID
}

func (v CSIVolumeExternalStubSort) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}

type CSIVolumeCreateRequest struct {
	Volumes []*CSIVolume
	WriteRequest
}

type CSIVolumeCreateResponse struct {
	Volumes []*CSIVolume
	QueryMeta
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
	Controllers         map[string]*CSIInfo
	Nodes               map[string]*CSIInfo
	Allocations         []*AllocationListStub
	ControllersHealthy  int
	ControllersExpected int
	NodesHealthy        int
	NodesExpected       int
	CreateIndex         uint64
	ModifyIndex         uint64
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
