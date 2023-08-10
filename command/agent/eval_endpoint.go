// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

// EvalsRequest is the entry point for /v1/evaluations and is responsible for
// handling both the listing of evaluations, and the bulk deletion of
// evaluations. The latter is a dangerous operation and should use the
// eval delete command to perform this.
func (s *HTTPServer) EvalsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case http.MethodGet:
		return s.evalsListRequest(resp, req)
	case http.MethodDelete:
		return s.evalsDeleteRequest(resp, req)
	default:
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}
}

func (s *HTTPServer) evalsListRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	args := structs.EvalListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	query := req.URL.Query()
	args.FilterEvalStatus = query.Get("status")
	args.FilterJobID = query.Get("job")

	var out structs.EvalListResponse
	if err := s.agent.RPC("Eval.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Evaluations == nil {
		out.Evaluations = make([]*structs.Evaluation, 0)
	}
	return out.Evaluations, nil
}

func (s *HTTPServer) evalsDeleteRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {

	var args structs.EvalDeleteRequest

	if err := decodeBody(req, &args); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	numIDs := len(args.EvalIDs)

	if args.Filter != "" && numIDs > 0 {
		return nil, CodedError(http.StatusBadRequest,
			"evals cannot be deleted by both ID and filter")
	}
	if args.Filter == "" && numIDs == 0 {
		return nil, CodedError(http.StatusBadRequest,
			"evals must be deleted by either ID or filter")
	}

	// If an explicit list of evaluation IDs is sent, ensure its within bounds
	if numIDs > structs.MaxUUIDsPerWriteRequest {
		return nil, CodedError(http.StatusBadRequest, fmt.Sprintf(
			"request includes %v evaluation IDs, must be %v or fewer",
			numIDs, structs.MaxUUIDsPerWriteRequest))
	}

	// Pass the write request to populate all meta fields.
	s.parseWriteRequest(req, &args.WriteRequest)

	var reply structs.EvalDeleteResponse
	if err := s.agent.RPC(structs.EvalDeleteRPCMethod, &args, &reply); err != nil {
		return nil, err
	}

	setIndex(resp, reply.Index)
	return reply, nil
}

func (s *HTTPServer) EvalSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	path := strings.TrimPrefix(req.URL.Path, "/v1/evaluation/")
	switch {
	case strings.HasSuffix(path, "/allocations"):
		evalID := strings.TrimSuffix(path, "/allocations")
		return s.evalAllocations(resp, req, evalID)
	default:
		return s.evalQuery(resp, req, path)
	}
}

func (s *HTTPServer) evalAllocations(resp http.ResponseWriter, req *http.Request, evalID string) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.EvalSpecificRequest{
		EvalID: evalID,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.EvalAllocationsResponse
	if err := s.agent.RPC("Eval.Allocations", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Allocations == nil {
		out.Allocations = make([]*structs.AllocListStub, 0)
	}
	return out.Allocations, nil
}

func (s *HTTPServer) evalQuery(resp http.ResponseWriter, req *http.Request, evalID string) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	args := structs.EvalSpecificRequest{
		EvalID: evalID,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	query := req.URL.Query()
	args.IncludeRelated = query.Get("related") == "true"

	var out structs.SingleEvalResponse
	if err := s.agent.RPC("Eval.GetEval", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Eval == nil {
		return nil, CodedError(404, "eval not found")
	}
	return out.Eval, nil
}

func (s *HTTPServer) EvalsCountRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	args := structs.EvalCountRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.EvalCountResponse
	if err := s.agent.RPC("Eval.Count", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return &out, nil
}
