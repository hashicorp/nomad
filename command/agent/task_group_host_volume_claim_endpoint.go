// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) TaskGroupHostVolumeClaimRequest(resp http.ResponseWriter, req *http.Request) (any, error) {
	// Tokenize the suffix of the path to get the volume id, tolerating a
	// present or missing trailing slash
	reqSuffix := strings.TrimPrefix(req.URL.Path, "/v1/volume/claim/")
	tokens := strings.FieldsFunc(reqSuffix, func(c rune) bool { return c == '/' })

	if len(tokens) == 0 {
		return nil, CodedError(404, resourceNotFoundErr)
	}

	switch req.Method {
	// DELETE /v1/volume/claim/:id
	case http.MethodDelete:
		return s.taskGroupHostVolumeClaimDelete(tokens[0], resp, req)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) TaskGroupHostVolumeClaimListRequest(resp http.ResponseWriter, req *http.Request) (any, error) {
	args := structs.TaskGroupVolumeClaimListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.TaskGroupVolumeClaimListResponse
	if err := s.agent.RPC("TaskGroupHostVolumeClaim.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out.Claims, nil
}

func (s *HTTPServer) taskGroupHostVolumeClaimDelete(id string, resp http.ResponseWriter, req *http.Request) (any, error) {
	args := structs.TaskGroupVolumeClaimDeleteRequest{ClaimID: id}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.TaskGroupVolumeClaimDeleteResponse
	if err := s.agent.RPC("TaskGroupHostVolumeClaim.Delete", &args, &out); err != nil {
		return nil, err
	}

	setIndex(resp, out.Index)
	return nil, nil
}
