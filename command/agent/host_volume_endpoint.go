// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) HostVolumesListRequest(resp http.ResponseWriter, req *http.Request) (any, error) {
	args := structs.HostVolumeListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	query := req.URL.Query()
	args.Prefix = query.Get("prefix")
	args.NodePool = query.Get("node_pool")
	args.NodeID = query.Get("node_id")

	var out structs.HostVolumeListResponse
	if err := s.agent.RPC("HostVolume.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out.Volumes, nil
}

// HostVolumeSpecificRequest dispatches GET and PUT
func (s *HTTPServer) HostVolumeSpecificRequest(resp http.ResponseWriter, req *http.Request) (any, error) {
	// Tokenize the suffix of the path to get the volume id, tolerating a
	// present or missing trailing slash
	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/volume/host/")
	tokens := strings.FieldsFunc(reqSuffix, func(c rune) bool { return c == '/' })

	if len(tokens) == 0 {
		return nil, CodedError(404, resourceNotFoundErr)
	}

	switch req.Method {

	// PUT /v1/volume/host/create
	// POST /v1/volume/host/create
	// PUT /v1/volume/host/register
	// POST /v1/volume/host/register
	case http.MethodPut, http.MethodPost:
		switch tokens[0] {
		case "create", "":
			return s.hostVolumeCreate(resp, req)
		case "register":
			return s.hostVolumeRegister(resp, req)
		default:
			return nil, CodedError(404, resourceNotFoundErr)
		}

	// DELETE /v1/volume/host/:id
	case http.MethodDelete:
		return s.hostVolumeDelete(tokens[0], resp, req)

	// GET /v1/volume/host/:id
	case http.MethodGet:
		return s.hostVolumeGet(tokens[0], resp, req)
	}

	return nil, CodedError(404, resourceNotFoundErr)
}

func (s *HTTPServer) hostVolumeGet(id string, resp http.ResponseWriter, req *http.Request) (any, error) {
	args := structs.HostVolumeGetRequest{
		ID: id,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.HostVolumeGetResponse
	if err := s.agent.RPC("HostVolume.Get", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Volume == nil {
		return nil, CodedError(404, "volume not found")
	}

	return out.Volume, nil
}

func (s *HTTPServer) hostVolumeRegister(resp http.ResponseWriter, req *http.Request) (any, error) {

	args := structs.HostVolumeRegisterRequest{}
	if err := decodeBody(req, &args); err != nil {
		return err, CodedError(400, err.Error())
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.HostVolumeRegisterResponse
	if err := s.agent.RPC("HostVolume.Register", &args, &out); err != nil {
		return nil, err
	}

	setIndex(resp, out.Index)

	return &out, nil
}

func (s *HTTPServer) hostVolumeCreate(resp http.ResponseWriter, req *http.Request) (any, error) {

	args := structs.HostVolumeCreateRequest{}
	if err := decodeBody(req, &args); err != nil {
		return err, CodedError(400, err.Error())
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.HostVolumeCreateResponse
	if err := s.agent.RPC("HostVolume.Create", &args, &out); err != nil {
		return nil, err
	}

	setIndex(resp, out.Index)

	return &out, nil
}

func (s *HTTPServer) hostVolumeDelete(id string, resp http.ResponseWriter, req *http.Request) (any, error) {
	// HTTP API only supports deleting a single ID because of compatibility with
	// the existing HTTP routes for CSI
	args := structs.HostVolumeDeleteRequest{VolumeID: id}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.HostVolumeDeleteResponse
	if err := s.agent.RPC("HostVolume.Delete", &args, &out); err != nil {
		return nil, err
	}

	setIndex(resp, out.Index)

	return nil, nil
}
