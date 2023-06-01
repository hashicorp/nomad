// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) NodePoolsRequest(resp http.ResponseWriter, req *http.Request) (any, error) {
	switch req.Method {
	case "GET":
		return s.nodePoolList(resp, req)
	case "PUT", "POST":
		return s.nodePoolUpsert(resp, req, "")
	default:
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}
}

func (s *HTTPServer) NodePoolSpecificRequest(resp http.ResponseWriter, req *http.Request) (any, error) {
	path := strings.TrimPrefix(req.URL.Path, "/v1/node/pool/")
	switch {
	default:
		return s.nodePoolCRUD(resp, req, path)
	}
}

func (s *HTTPServer) nodePoolCRUD(resp http.ResponseWriter, req *http.Request, poolName string) (any, error) {
	switch req.Method {
	case "GET":
		return s.nodePoolQuery(resp, req, poolName)
	case "PUT", "POST":
		return s.nodePoolUpsert(resp, req, poolName)
	case "DELETE":
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
