// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"strconv"
	"strings"

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

	return out.Volume, nil
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

	secrets := parseCSISecrets(req)
	args := structs.CSIVolumeDeleteRequest{
		VolumeIDs: []string{id},
		Secrets:   secrets,
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

	secrets := parseCSISecrets(req)
	snap.Secrets = secrets

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
	secrets := parseCSISecrets(req)
	args.Secrets = secrets
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

	return out.Plugin, nil
}

// parseCSISecrets extracts a map of k/v pairs from the CSI secrets
// header. Silently ignores invalid secrets
func parseCSISecrets(req *http.Request) structs.CSISecrets {
	secretsHeader := req.Header.Get("X-Nomad-CSI-Secrets")
	if secretsHeader == "" {
		return nil
	}

	secrets := map[string]string{}
	secretkvs := strings.Split(secretsHeader, ",")
	for _, secretkv := range secretkvs {
		if key, value, found := strings.Cut(secretkv, "="); found {
			secrets[key] = value
		}
	}
	if len(secrets) == 0 {
		return nil
	}
	return structs.CSISecrets(secrets)
}
