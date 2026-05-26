// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) JobQueueStatus(resp http.ResponseWriter, req *http.Request) (any, error) {
	switch req.Method {
	case http.MethodGet:
		break
	default:
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	var args structs.QueueStatusRequest
	var out structs.QueueStatusResponse

	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	if err := s.agent.RPC("Job.QueueStatus", &args, &out); err != nil {
		return nil, err
	}

	setMeta(resp, &out.QueryMeta)
	return out.Status, nil
}
