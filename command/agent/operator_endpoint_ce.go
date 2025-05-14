// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package agent

import (
	"net/http"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) LicenseRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case http.MethodGet:
		resp.WriteHeader(http.StatusNoContent)
		return nil, nil
	case http.MethodPut:
		return nil, CodedError(501, ErrEntOnly)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

// OperatorUtilizationRequest is used get a utilization reporting bundle.
func (s *HTTPServer) OperatorUtilizationRequest(resp http.ResponseWriter, req *http.Request) (any, error) {
	if req.Method != http.MethodPost {
		return nil, CodedError(405, ErrInvalidMethod)
	}
	return nil, CodedError(501, ErrEntOnly)
}

func autopilotToAPIEntState(_ structs.OperatorHealthReply, _ *api.OperatorHealthReply) interface{} {
	return nil
}
