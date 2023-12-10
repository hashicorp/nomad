// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) DeploymentsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	args := structs.DeploymentListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.DeploymentListResponse
	if err := s.agent.RPC("Deployment.List", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Deployments == nil {
		out.Deployments = make([]*structs.Deployment, 0)
	}
	return out.Deployments, nil
}

func (s *HTTPServer) DeploymentSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	path := strings.TrimPrefix(req.URL.Path, "/v1/deployment/")
	switch {
	case strings.HasPrefix(path, "allocations/"):
		deploymentID := strings.TrimPrefix(path, "allocations/")
		return s.deploymentAllocations(resp, req, deploymentID)
	case strings.HasPrefix(path, "fail/"):
		deploymentID := strings.TrimPrefix(path, "fail/")
		return s.deploymentFail(resp, req, deploymentID)
	case strings.HasPrefix(path, "pause/"):
		deploymentID := strings.TrimPrefix(path, "pause/")
		return s.deploymentPause(resp, req, deploymentID)
	case strings.HasPrefix(path, "promote/"):
		deploymentID := strings.TrimPrefix(path, "promote/")
		return s.deploymentPromote(resp, req, deploymentID)
	case strings.HasPrefix(path, "allocation-health/"):
		deploymentID := strings.TrimPrefix(path, "allocation-health/")
		return s.deploymentSetAllocHealth(resp, req, deploymentID)
	case strings.HasPrefix(path, "unblock/"):
		deploymentID := strings.TrimPrefix(path, "unblock/")
		return s.deploymentUnblock(resp, req, deploymentID)
	default:
		return s.deploymentQuery(resp, req, path)
	}
}

// TODO test and api
func (s *HTTPServer) deploymentFail(resp http.ResponseWriter, req *http.Request, deploymentID string) (interface{}, error) {
	if req.Method != http.MethodPut && req.Method != http.MethodPost {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}
	args := structs.DeploymentFailRequest{
		DeploymentID: deploymentID,
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.DeploymentUpdateResponse
	if err := s.agent.RPC("Deployment.Fail", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) deploymentPause(resp http.ResponseWriter, req *http.Request, deploymentID string) (interface{}, error) {
	if req.Method != http.MethodPut && req.Method != http.MethodPost {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	var pauseRequest structs.DeploymentPauseRequest
	if err := decodeBody(req, &pauseRequest); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}
	if pauseRequest.DeploymentID == "" {
		return nil, CodedError(http.StatusBadRequest, "DeploymentID must be specified")
	}
	if pauseRequest.DeploymentID != deploymentID {
		return nil, CodedError(http.StatusBadRequest, "Deployment ID does not match")
	}
	s.parseWriteRequest(req, &pauseRequest.WriteRequest)

	var out structs.DeploymentUpdateResponse
	if err := s.agent.RPC("Deployment.Pause", &pauseRequest, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) deploymentPromote(resp http.ResponseWriter, req *http.Request, deploymentID string) (interface{}, error) {
	if req.Method != http.MethodPut && req.Method != http.MethodPost {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	var promoteRequest structs.DeploymentPromoteRequest
	if err := decodeBody(req, &promoteRequest); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}
	if promoteRequest.DeploymentID == "" {
		return nil, CodedError(http.StatusBadRequest, "DeploymentID must be specified")
	}
	if promoteRequest.DeploymentID != deploymentID {
		return nil, CodedError(http.StatusBadRequest, "Deployment ID does not match")
	}
	s.parseWriteRequest(req, &promoteRequest.WriteRequest)

	var out structs.DeploymentUpdateResponse
	if err := s.agent.RPC("Deployment.Promote", &promoteRequest, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) deploymentUnblock(resp http.ResponseWriter, req *http.Request, deploymentID string) (interface{}, error) {
	if req.Method != http.MethodPut && req.Method != http.MethodPost {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	var unblockRequest structs.DeploymentUnblockRequest
	if err := decodeBody(req, &unblockRequest); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}
	if unblockRequest.DeploymentID == "" {
		return nil, CodedError(http.StatusBadRequest, "DeploymentID must be specified")
	}
	if unblockRequest.DeploymentID != deploymentID {
		return nil, CodedError(http.StatusBadRequest, "Deployment ID does not match")
	}
	s.parseWriteRequest(req, &unblockRequest.WriteRequest)

	var out structs.DeploymentUpdateResponse
	if err := s.agent.RPC("Deployment.Unblock", &unblockRequest, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) deploymentSetAllocHealth(resp http.ResponseWriter, req *http.Request, deploymentID string) (interface{}, error) {
	if req.Method != http.MethodPut && req.Method != http.MethodPost {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	var healthRequest structs.DeploymentAllocHealthRequest
	if err := decodeBody(req, &healthRequest); err != nil {
		return nil, CodedError(http.StatusBadRequest, err.Error())
	}
	if healthRequest.DeploymentID == "" {
		return nil, CodedError(http.StatusBadRequest, "DeploymentID must be specified")
	}
	if healthRequest.DeploymentID != deploymentID {
		return nil, CodedError(http.StatusBadRequest, "Deployment ID does not match")
	}
	s.parseWriteRequest(req, &healthRequest.WriteRequest)

	var out structs.DeploymentUpdateResponse
	if err := s.agent.RPC("Deployment.SetAllocHealth", &healthRequest, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return out, nil
}

func (s *HTTPServer) deploymentAllocations(resp http.ResponseWriter, req *http.Request, deploymentID string) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	args := structs.DeploymentSpecificRequest{
		DeploymentID: deploymentID,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.AllocListResponse
	if err := s.agent.RPC("Deployment.Allocations", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Allocations == nil {
		out.Allocations = make([]*structs.AllocListStub, 0)
	}
	for _, alloc := range out.Allocations {
		alloc.SetEventDisplayMessages()
	}
	return out.Allocations, nil
}

func (s *HTTPServer) deploymentQuery(resp http.ResponseWriter, req *http.Request, deploymentID string) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	args := structs.DeploymentSpecificRequest{
		DeploymentID: deploymentID,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.SingleDeploymentResponse
	if err := s.agent.RPC("Deployment.GetDeployment", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	if out.Deployment == nil {
		return nil, CodedError(http.StatusNotFound, "deployment not found")
	}
	return out.Deployment, nil
}
