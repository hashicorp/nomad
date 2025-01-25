// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"

	"github.com/hashicorp/nomad/nomad/structs"
)

// ListVolumesRequest dispatches requests for listing volumes to a specific type.
func (s *HTTPServer) ListVolumesRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	query := req.URL.Query()
	qtype, ok := query["type"]
	if !ok {
		return []*structs.CSIVolListStub{}, nil
	}
	switch qtype[0] {
	case "host":
		return s.HostVolumesListRequest(resp, req)
	case "csi":
		return s.CSIVolumesRequest(resp, req)
	default:
		return nil, CodedError(404, resourceNotFoundErr)
	}
}
