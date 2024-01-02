// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) NodePoolsRequest(resp http.ResponseWriter, req *http.Request) (any, error) {
	switch req.Method {
	case http.MethodGet:
		return s.nodePoolList(resp, req)
	case http.MethodPut, http.MethodPost:
		return s.nodePoolUpsert(resp, req, "")
	default:
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}
}

func (s *HTTPServer) NodePoolSpecificRequest(resp http.ResponseWriter, req *http.Request) (any, error) {
	path := strings.TrimPrefix(req.URL.Path, "/v1/node/pool/")
	switch {
	case strings.HasSuffix(path, "/nodes"):
		poolName := strings.TrimSuffix(path, "/nodes")
		return s.nodePoolNodesList(resp, req, poolName)
	case strings.HasSuffix(path, "/jobs"):
		poolName := strings.TrimSuffix(path, "/jobs")
		return s.nodePoolJobList(resp, req, poolName)
	default:
		return s.nodePoolCRUD(resp, req, path)
	}
}

func (s *HTTPServer) nodePoolCRUD(resp http.ResponseWriter, req *http.Request, poolName string) (any, error) {
	switch req.Method {
	case http.MethodGet:
		return s.nodePoolQuery(resp, req, poolName)
	case http.MethodPut, http.MethodPost:
		return s.nodePoolUpsert(resp, req, poolName)
	case http.MethodDelete:
		return s.nodePoolDelete(resp, req, poolName)
	default:
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}
}

func (s *HTTPServer) nodePoolList(resp http.ResponseWriter, req *http.Request) (any, error) {
	args := structs.NodePoolListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.NodePoolListResponse
	if err := s.agent.RPC("NodePool.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.NodePools == nil {
		out.NodePools = make([]*structs.NodePool, 0)
	}
	return out.NodePools, nil
}

func (s *HTTPServer) nodePoolQuery(resp http.ResponseWriter, req *http.Request, poolName string) (any, error) {
	args := structs.NodePoolSpecificRequest{
		Name: poolName,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SingleNodePoolResponse
	if err := s.agent.RPC("NodePool.GetNodePool", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.NodePool == nil {
		return nil, CodedError(http.StatusNotFound, "node pool not found")
	}

	return out.NodePool, nil
}

func (s *HTTPServer) nodePoolUpsert(resp http.ResponseWriter, req *http.Request, poolName string) (any, error) {
	var pool structs.NodePool
	if err := decodeBody(req, &pool); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}

	if poolName != "" && pool.Name != poolName {
		return nil, CodedError(http.StatusBadRequest, "Node pool name does not match request path")
	}

	args := structs.NodePoolUpsertRequest{
		NodePools: []*structs.NodePool{&pool},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.GenericResponse
	if err := s.agent.RPC("NodePool.UpsertNodePools", &args, &out); err != nil {
		return nil, err
	}

	setIndex(resp, out.Index)
	return nil, nil
}

func (s *HTTPServer) nodePoolDelete(resp http.ResponseWriter, req *http.Request, poolName string) (any, error) {
	args := structs.NodePoolDeleteRequest{
		Names: []string{poolName},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.GenericResponse
	if err := s.agent.RPC("NodePool.DeleteNodePools", &args, &out); err != nil {
		return nil, err
	}

	setIndex(resp, out.Index)
	return nil, nil
}

func (s *HTTPServer) nodePoolNodesList(resp http.ResponseWriter, req *http.Request, poolName string) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	args := structs.NodePoolNodesRequest{
		Name: poolName,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	// Parse node fields selection.
	fields, err := parseNodeListStubFields(req)
	if err != nil {
		return nil, CodedError(http.StatusBadRequest, fmt.Sprintf("Failed to parse node list fields: %v", err))
	}
	args.Fields = fields

	if args.Prefix != "" {
		// the prefix argument is ambiguous for this endpoint (does it refer to
		// the node pool name or the node IDs like /v1/nodes?) so the RPC
		// handler ignores it
		return nil, CodedError(http.StatusBadRequest, "prefix argument not allowed")
	}

	var out structs.NodePoolNodesResponse
	if err := s.agent.RPC("NodePool.ListNodes", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Nodes == nil {
		out.Nodes = make([]*structs.NodeListStub, 0)
	}
	return out.Nodes, nil
}

func (s *HTTPServer) nodePoolJobList(resp http.ResponseWriter, req *http.Request, poolName string) (any, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	args := structs.NodePoolJobsRequest{
		Name: poolName,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	if args.Prefix != "" {
		// the prefix argument is ambiguous for this endpoint (does it refer to
		// the node pool name or the job names like /v1/jobs?) so the RPC
		// handler ignores it
		return nil, CodedError(http.StatusBadRequest, "prefix argument not allowed")
	}

	// Parse meta query param
	args.Fields = &structs.JobStubFields{}
	jobMeta, err := parseBool(req, "meta")
	if err != nil {
		return nil, err
	}
	if jobMeta != nil {
		args.Fields.Meta = *jobMeta
	}

	var out structs.NodePoolJobsResponse
	if err := s.agent.RPC("NodePool.ListJobs", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Jobs == nil {
		out.Jobs = make([]*structs.JobListStub, 0)
	}
	return out.Jobs, nil
}
