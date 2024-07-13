// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) GarbageCollectRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodPut {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var args structs.GenericRequest
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var gResp structs.GenericResponse
	if err := s.agent.RPC("System.GarbageCollect", &args, &gResp); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *HTTPServer) ReconcileJobSummaries(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodPut {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var args structs.GenericRequest
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var gResp structs.GenericResponse
	if err := s.agent.RPC("System.ReconcileJobSummaries", &args, &gResp); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *HTTPServer) Portland(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodPut {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	var args structs.PortlandRequest
	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(400, err.Error())
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var r structs.GenericResponse
	if err := s.agent.RPC("System.Portland", &args, &r); err != nil {
		return nil, err
	}
	setIndex(resp, r.Index)
	return r, nil
}
