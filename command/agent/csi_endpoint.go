package agent

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) CSIVolumesRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case http.MethodPut, http.MethodPost:
		return s.csiVolumeRegister(resp, req)
	case http.MethodGet:
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}

	// Type filters volume lists to a specific type. When support for non-CSI volumes is
	// introduced, we'll need to dispatch here
	query := req.URL.Query()
	qtype, ok := query["type"]
	if !ok {
		return []*structs.CSIVolListStub{}, nil
	}
	if qtype[0] != "csi" {
		return nil, nil
	}

	args := structs.CSIVolumeListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	args.Prefix = query.Get("prefix")
	args.PluginID = query.Get("plugin_id")
	args.NodeID = query.Get("node_id")

	var out structs.CSIVolumeListResponse
	if err := s.agent.RPC("CSIVolume.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out.Volumes, nil
}

func (s *HTTPServer) CSIExternalVolumesRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.CSIVolumeExternalListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	query := req.URL.Query()
	args.PluginID = query.Get("plugin_id")

	var out structs.CSIVolumeExternalListResponse
	if err := s.agent.RPC("CSIVolume.ListExternal", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out, nil
}

// CSIVolumeSpecificRequest dispatches GET and PUT
func (s *HTTPServer) CSIVolumeSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Tokenize the suffix of the path to get the volume id
	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/volume/csi/")
	tokens := strings.Split(reqSuffix, "/")
	if len(tokens) < 1 {
		return nil, CodedError(404, resourceNotFoundErr)
	}
	id := tokens[0]

	if len(tokens) == 1 {
		switch req.Method {
		case http.MethodGet:
			return s.csiVolumeGet(id, resp, req)
		case http.MethodPut:
			return s.csiVolumeRegister(resp, req)
		case http.MethodDelete:
			return s.csiVolumeDeregister(id, resp, req)
		default:
			return nil, CodedError(405, ErrInvalidMethod)
		}
	}

	if len(tokens) == 2 {
		switch req.Method {
		case http.MethodPut:
			if tokens[1] == "create" {
				return s.csiVolumeCreate(resp, req)
			}
		case http.MethodDelete:
			if tokens[1] == "detach" {
				return s.csiVolumeDetach(id, resp, req)
			}
			if tokens[1] == "delete" {
				return s.csiVolumeDelete(id, resp, req)
			}
		default:
			return nil, CodedError(405, ErrInvalidMethod)
		}
	}

	return nil, CodedError(404, resourceNotFoundErr)
}

func (s *HTTPServer) csiVolumeGet(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.CSIVolumeGetRequest{
		ID: id,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.CSIVolumeGetResponse
	if err := s.agent.RPC("CSIVolume.Get", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Volume == nil {
		return nil, CodedError(404, "volume not found")
	}

	vol := structsCSIVolumeToApi(out.Volume)

	// remove sensitive fields, as our redaction mechanism doesn't
	// help serializing here
	vol.Secrets = nil
	vol.MountOptions = nil

	return vol, nil
}

func (s *HTTPServer) csiVolumeRegister(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case http.MethodPost, http.MethodPut:
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.CSIVolumeRegisterRequest{}
	if err := decodeBody(req, &args); err != nil {
		return err, CodedError(400, err.Error())
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.CSIVolumeRegisterResponse
	if err := s.agent.RPC("CSIVolume.Register", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)

	return nil, nil
}

func (s *HTTPServer) csiVolumeCreate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case http.MethodPost, http.MethodPut:
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.CSIVolumeCreateRequest{}
	if err := decodeBody(req, &args); err != nil {
		return err, CodedError(400, err.Error())
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.CSIVolumeCreateResponse
	if err := s.agent.RPC("CSIVolume.Create", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)

	return out, nil
}

func (s *HTTPServer) csiVolumeDeregister(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodDelete {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	raw := req.URL.Query().Get("force")
	var force bool
	if raw != "" {
		var err error
		force, err = strconv.ParseBool(raw)
		if err != nil {
			return nil, CodedError(400, "invalid force value")
		}
	}

	args := structs.CSIVolumeDeregisterRequest{
		VolumeIDs: []string{id},
		Force:     force,
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.CSIVolumeDeregisterResponse
	if err := s.agent.RPC("CSIVolume.Deregister", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)

	return nil, nil
}

func (s *HTTPServer) csiVolumeDelete(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodDelete {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.CSIVolumeDeleteRequest{
		VolumeIDs: []string{id},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.CSIVolumeDeleteResponse
	if err := s.agent.RPC("CSIVolume.Delete", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)

	return nil, nil
}

func (s *HTTPServer) csiVolumeDetach(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodDelete {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	nodeID := req.URL.Query().Get("node")
	if nodeID == "" {
		return nil, CodedError(400, "detach requires node ID")
	}

	args := structs.CSIVolumeUnpublishRequest{
		VolumeID: id,
		Claim: &structs.CSIVolumeClaim{
			NodeID: nodeID,
			Mode:   structs.CSIVolumeClaimGC,
		},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.CSIVolumeUnpublishResponse
	if err := s.agent.RPC("CSIVolume.Unpublish", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return nil, nil
}

func (s *HTTPServer) CSISnapshotsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case http.MethodPut, http.MethodPost:
		return s.csiSnapshotCreate(resp, req)
	case http.MethodDelete:
		return s.csiSnapshotDelete(resp, req)
	case http.MethodGet:
		return s.csiSnapshotList(resp, req)
	}
	return nil, CodedError(405, ErrInvalidMethod)
}

func (s *HTTPServer) csiSnapshotCreate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	args := structs.CSISnapshotCreateRequest{}
	if err := decodeBody(req, &args); err != nil {
		return err, CodedError(400, err.Error())
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.CSISnapshotCreateResponse
	if err := s.agent.RPC("CSIVolume.CreateSnapshot", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out, nil
}

func (s *HTTPServer) csiSnapshotDelete(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	args := structs.CSISnapshotDeleteRequest{}
	s.parseWriteRequest(req, &args.WriteRequest)

	snap := &structs.CSISnapshot{Secrets: structs.CSISecrets{}}

	query := req.URL.Query()
	snap.PluginID = query.Get("plugin_id")
	snap.ID = query.Get("snapshot_id")
	secrets := query["secret"]
	for _, raw := range secrets {
		secret := strings.Split(raw, "=")
		if len(secret) == 2 {
			snap.Secrets[secret[0]] = secret[1]
		}
	}

	args.Snapshots = []*structs.CSISnapshot{snap}

	var out structs.CSISnapshotDeleteResponse
	if err := s.agent.RPC("CSIVolume.DeleteSnapshot", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return nil, nil
}

func (s *HTTPServer) csiSnapshotList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	args := structs.CSISnapshotListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	query := req.URL.Query()
	args.PluginID = query.Get("plugin_id")
	querySecrets := query["secrets"]

	// Parse comma separated secrets only when provided
	if len(querySecrets) >= 1 {
		secrets := strings.Split(querySecrets[0], ",")
		args.Secrets = make(structs.CSISecrets)
		for _, raw := range secrets {
			secret := strings.Split(raw, "=")
			if len(secret) == 2 {
				args.Secrets[secret[0]] = secret[1]
			}
		}
	}

	var out structs.CSISnapshotListResponse
	if err := s.agent.RPC("CSIVolume.ListSnapshots", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out, nil
}

// CSIPluginsRequest lists CSI plugins
func (s *HTTPServer) CSIPluginsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	// Type filters plugin lists to a specific type. When support for non-CSI plugins is
	// introduced, we'll need to dispatch here
	query := req.URL.Query()
	qtype, ok := query["type"]
	if !ok {
		return []*structs.CSIPluginListStub{}, nil
	}
	if qtype[0] != "csi" {
		return nil, nil
	}

	args := structs.CSIPluginListRequest{}

	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.CSIPluginListResponse
	if err := s.agent.RPC("CSIPlugin.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out.Plugins, nil
}

// CSIPluginSpecificRequest list the job with CSIInfo
func (s *HTTPServer) CSIPluginSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	// Tokenize the suffix of the path to get the plugin id
	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/plugin/csi/")
	tokens := strings.Split(reqSuffix, "/")
	if len(tokens) > 2 || len(tokens) < 1 {
		return nil, CodedError(404, resourceNotFoundErr)
	}
	id := tokens[0]

	args := structs.CSIPluginGetRequest{ID: id}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.CSIPluginGetResponse
	if err := s.agent.RPC("CSIPlugin.Get", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Plugin == nil {
		return nil, CodedError(404, "plugin not found")
	}

	return structsCSIPluginToApi(out.Plugin), nil
}

// structsCSIPluginToApi converts CSIPlugin, setting Expected the count of known plugin
// instances
func structsCSIPluginToApi(plug *structs.CSIPlugin) *api.CSIPlugin {
	if plug == nil {
		return nil
	}
	out := &api.CSIPlugin{
		ID:                  plug.ID,
		Provider:            plug.Provider,
		Version:             plug.Version,
		Allocations:         make([]*api.AllocationListStub, 0, len(plug.Allocations)),
		ControllerRequired:  plug.ControllerRequired,
		ControllersHealthy:  plug.ControllersHealthy,
		ControllersExpected: plug.ControllersExpected,
		Controllers:         make(map[string]*api.CSIInfo, len(plug.Controllers)),
		NodesHealthy:        plug.NodesHealthy,
		NodesExpected:       plug.NodesExpected,
		Nodes:               make(map[string]*api.CSIInfo, len(plug.Nodes)),
		CreateIndex:         plug.CreateIndex,
		ModifyIndex:         plug.ModifyIndex,
	}

	for k, v := range plug.Controllers {
		out.Controllers[k] = structsCSIInfoToApi(v)
	}

	for k, v := range plug.Nodes {
		out.Nodes[k] = structsCSIInfoToApi(v)
	}

	for _, a := range plug.Allocations {
		if a != nil {
			out.Allocations = append(out.Allocations, structsAllocListStubToApi(a))
		}
	}

	return out
}

// structsCSIVolumeToApi converts CSIVolume, creating the allocation array
func structsCSIVolumeToApi(vol *structs.CSIVolume) *api.CSIVolume {
	if vol == nil {
		return nil
	}

	allocCount := len(vol.ReadAllocs) + len(vol.WriteAllocs)

	out := &api.CSIVolume{
		ID:             vol.ID,
		Name:           vol.Name,
		ExternalID:     vol.ExternalID,
		Namespace:      vol.Namespace,
		Topologies:     structsCSITopolgiesToApi(vol.Topologies),
		AccessMode:     structsCSIAccessModeToApi(vol.AccessMode),
		AttachmentMode: structsCSIAttachmentModeToApi(vol.AttachmentMode),
		MountOptions:   structsCSIMountOptionsToApi(vol.MountOptions),
		Secrets:        structsCSISecretsToApi(vol.Secrets),
		Parameters:     vol.Parameters,
		Context:        vol.Context,

		// Allocations is the collapsed list of both read and write allocs
		Allocations: make([]*api.AllocationListStub, 0, allocCount),

		// tracking of volume claims
		ReadAllocs:  map[string]*api.Allocation{},
		WriteAllocs: map[string]*api.Allocation{},

		Schedulable:         vol.Schedulable,
		PluginID:            vol.PluginID,
		Provider:            vol.Provider,
		ProviderVersion:     vol.ProviderVersion,
		ControllerRequired:  vol.ControllerRequired,
		ControllersHealthy:  vol.ControllersHealthy,
		ControllersExpected: vol.ControllersExpected,
		NodesHealthy:        vol.NodesHealthy,
		NodesExpected:       vol.NodesExpected,
		ResourceExhausted:   vol.ResourceExhausted,
		CreateIndex:         vol.CreateIndex,
		ModifyIndex:         vol.ModifyIndex,
	}

	// WriteAllocs and ReadAllocs will only ever contain the Allocation ID,
	// with a null value for the Allocation; these IDs are mapped to
	// allocation stubs in the Allocations field. This indirection is so the
	// API can support both the UI and CLI consumer in a safely backwards
	// compatible way
	for _, a := range vol.WriteAllocs {
		if a != nil {
			alloc := structsAllocListStubToApi(a.Stub(nil))
			if alloc != nil {
				out.WriteAllocs[alloc.ID] = nil
				out.Allocations = append(out.Allocations, alloc)
			}
		}
	}

	for _, a := range vol.ReadAllocs {
		if a != nil {
			alloc := structsAllocListStubToApi(a.Stub(nil))
			if alloc != nil {
				out.ReadAllocs[alloc.ID] = nil
				out.Allocations = append(out.Allocations, alloc)
			}
		}
	}

	return out
}

// structsCSIInfoToApi converts CSIInfo, part of CSIPlugin
func structsCSIInfoToApi(info *structs.CSIInfo) *api.CSIInfo {
	if info == nil {
		return nil
	}
	out := &api.CSIInfo{
		PluginID:                 info.PluginID,
		AllocID:                  info.AllocID,
		Healthy:                  info.Healthy,
		HealthDescription:        info.HealthDescription,
		UpdateTime:               info.UpdateTime,
		RequiresControllerPlugin: info.RequiresControllerPlugin,
		RequiresTopologies:       info.RequiresTopologies,
	}

	if info.ControllerInfo != nil {
		out.ControllerInfo = &api.CSIControllerInfo{
			SupportsReadOnlyAttach:           info.ControllerInfo.SupportsReadOnlyAttach,
			SupportsAttachDetach:             info.ControllerInfo.SupportsAttachDetach,
			SupportsListVolumes:              info.ControllerInfo.SupportsListVolumes,
			SupportsListVolumesAttachedNodes: info.ControllerInfo.SupportsListVolumesAttachedNodes,
		}
	}

	if info.NodeInfo != nil {
		out.NodeInfo = &api.CSINodeInfo{
			ID:                      info.NodeInfo.ID,
			MaxVolumes:              info.NodeInfo.MaxVolumes,
			RequiresNodeStageVolume: info.NodeInfo.RequiresNodeStageVolume,
		}

		if info.NodeInfo.AccessibleTopology != nil {
			out.NodeInfo.AccessibleTopology = &api.CSITopology{}
			out.NodeInfo.AccessibleTopology.Segments = info.NodeInfo.AccessibleTopology.Segments
		}
	}

	return out
}

// structsAllocListStubToApi converts AllocListStub, for CSIPlugin
func structsAllocListStubToApi(alloc *structs.AllocListStub) *api.AllocationListStub {
	if alloc == nil {
		return nil
	}
	out := &api.AllocationListStub{
		ID:                    alloc.ID,
		EvalID:                alloc.EvalID,
		Name:                  alloc.Name,
		Namespace:             alloc.Namespace,
		NodeID:                alloc.NodeID,
		NodeName:              alloc.NodeName,
		JobID:                 alloc.JobID,
		JobType:               alloc.JobType,
		JobVersion:            alloc.JobVersion,
		TaskGroup:             alloc.TaskGroup,
		DesiredStatus:         alloc.DesiredStatus,
		DesiredDescription:    alloc.DesiredDescription,
		ClientStatus:          alloc.ClientStatus,
		ClientDescription:     alloc.ClientDescription,
		TaskStates:            make(map[string]*api.TaskState, len(alloc.TaskStates)),
		FollowupEvalID:        alloc.FollowupEvalID,
		PreemptedAllocations:  alloc.PreemptedAllocations,
		PreemptedByAllocation: alloc.PreemptedByAllocation,
		CreateIndex:           alloc.CreateIndex,
		ModifyIndex:           alloc.ModifyIndex,
		CreateTime:            alloc.CreateTime,
		ModifyTime:            alloc.ModifyTime,
	}

	out.DeploymentStatus = structsAllocDeploymentStatusToApi(alloc.DeploymentStatus)
	out.RescheduleTracker = structsRescheduleTrackerToApi(alloc.RescheduleTracker)

	for k, v := range alloc.TaskStates {
		out.TaskStates[k] = structsTaskStateToApi(v)
	}

	return out
}

// structsAllocDeploymentStatusToApi converts RescheduleTracker, part of AllocListStub
func structsAllocDeploymentStatusToApi(ads *structs.AllocDeploymentStatus) *api.AllocDeploymentStatus {
	if ads == nil {
		return nil
	}
	out := &api.AllocDeploymentStatus{
		Healthy:     ads.Healthy,
		Timestamp:   ads.Timestamp,
		Canary:      ads.Canary,
		ModifyIndex: ads.ModifyIndex,
	}
	return out
}

// structsRescheduleTrackerToApi converts RescheduleTracker, part of AllocListStub
func structsRescheduleTrackerToApi(rt *structs.RescheduleTracker) *api.RescheduleTracker {
	if rt == nil {
		return nil
	}
	out := &api.RescheduleTracker{
		Events: make([]*api.RescheduleEvent, 0, len(rt.Events)),
	}

	for _, e := range rt.Events {
		out.Events = append(out.Events, &api.RescheduleEvent{
			RescheduleTime: e.RescheduleTime,
			PrevAllocID:    e.PrevAllocID,
			PrevNodeID:     e.PrevNodeID,
		})
	}

	return out
}

// structsTaskStateToApi converts TaskState, part of AllocListStub
func structsTaskStateToApi(ts *structs.TaskState) *api.TaskState {
	if ts == nil {
		return nil
	}
	out := &api.TaskState{
		State:       ts.State,
		Failed:      ts.Failed,
		Restarts:    ts.Restarts,
		LastRestart: ts.LastRestart,
		StartedAt:   ts.StartedAt,
		FinishedAt:  ts.FinishedAt,
		Events:      make([]*api.TaskEvent, 0, len(ts.Events)),
	}

	for _, te := range ts.Events {
		out.Events = append(out.Events, structsTaskEventToApi(te))
	}

	return out
}

// structsTaskEventToApi converts TaskEvents, part of AllocListStub
func structsTaskEventToApi(te *structs.TaskEvent) *api.TaskEvent {
	if te == nil {
		return nil
	}
	out := &api.TaskEvent{
		Type:           te.Type,
		Time:           te.Time,
		DisplayMessage: te.DisplayMessage,
		Details:        te.Details,

		// DEPRECATION NOTICE: The following fields are all deprecated. see TaskEvent struct in structs.go for details.
		FailsTask:        te.FailsTask,
		RestartReason:    te.RestartReason,
		SetupError:       te.SetupError,
		DriverError:      te.DriverError,
		DriverMessage:    te.DriverMessage,
		ExitCode:         te.ExitCode,
		Signal:           te.Signal,
		Message:          te.Message,
		KillReason:       te.KillReason,
		KillTimeout:      te.KillTimeout,
		KillError:        te.KillError,
		StartDelay:       te.StartDelay,
		DownloadError:    te.DownloadError,
		ValidationError:  te.ValidationError,
		DiskLimit:        te.DiskLimit,
		FailedSibling:    te.FailedSibling,
		VaultError:       te.VaultError,
		TaskSignalReason: te.TaskSignalReason,
		TaskSignal:       te.TaskSignal,
		GenericSource:    te.GenericSource,
	}

	return out
}

// structsCSITopolgiesToApi converts topologies, part of structsCSIVolumeToApi
func structsCSITopolgiesToApi(tops []*structs.CSITopology) []*api.CSITopology {
	out := make([]*api.CSITopology, 0, len(tops))
	for _, t := range tops {
		out = append(out, &api.CSITopology{
			Segments: t.Segments,
		})
	}

	return out
}

// structsCSIAccessModeToApi converts access mode, part of structsCSIVolumeToApi
func structsCSIAccessModeToApi(mode structs.CSIVolumeAccessMode) api.CSIVolumeAccessMode {
	switch mode {
	case structs.CSIVolumeAccessModeSingleNodeReader:
		return api.CSIVolumeAccessModeSingleNodeReader
	case structs.CSIVolumeAccessModeSingleNodeWriter:
		return api.CSIVolumeAccessModeSingleNodeWriter
	case structs.CSIVolumeAccessModeMultiNodeReader:
		return api.CSIVolumeAccessModeMultiNodeReader
	case structs.CSIVolumeAccessModeMultiNodeSingleWriter:
		return api.CSIVolumeAccessModeMultiNodeSingleWriter
	case structs.CSIVolumeAccessModeMultiNodeMultiWriter:
		return api.CSIVolumeAccessModeMultiNodeMultiWriter
	default:
	}
	return api.CSIVolumeAccessModeUnknown
}

// structsCSIAttachmentModeToApiModeToApi converts attachment mode, part of structsCSIVolumeToApi
func structsCSIAttachmentModeToApi(mode structs.CSIVolumeAttachmentMode) api.CSIVolumeAttachmentMode {
	switch mode {
	case structs.CSIVolumeAttachmentModeBlockDevice:
		return api.CSIVolumeAttachmentModeBlockDevice
	case structs.CSIVolumeAttachmentModeFilesystem:
		return api.CSIVolumeAttachmentModeFilesystem
	default:
	}
	return api.CSIVolumeAttachmentModeUnknown
}

// structsCSIMountOptionsToApi converts mount options, part of structsCSIVolumeToApi
func structsCSIMountOptionsToApi(opts *structs.CSIMountOptions) *api.CSIMountOptions {
	if opts == nil {
		return nil
	}

	return &api.CSIMountOptions{
		FSType:     opts.FSType,
		MountFlags: opts.MountFlags,
	}
}

func structsCSISecretsToApi(secrets structs.CSISecrets) api.CSISecrets {
	out := make(api.CSISecrets, len(secrets))
	for k, v := range secrets {
		out[k] = v
	}
	return out
}
