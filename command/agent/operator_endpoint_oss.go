// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package agent

import (
	"net/http"
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
